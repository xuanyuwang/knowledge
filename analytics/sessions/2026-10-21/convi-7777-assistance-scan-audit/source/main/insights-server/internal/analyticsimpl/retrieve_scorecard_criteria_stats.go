package analyticsimpl

import (
	"context"
	"slices"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"gorm.io/gorm"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/fnutils"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"
)

type criteriaScoreStats struct {
	CriterionIdentifier string
	TotalAutoScoreCount int64
	OverwrittenCount    int64
}

func (a AnalyticsServiceImpl) RetrieveScorecardCriteriaStats(ctx context.Context, req *analyticspb.RetrieveScorecardCriteriaStatsRequest) (*analyticspb.RetrieveScorecardCriteriaStatsResponse, error) {
	db := a.appsDB.DB(ctx)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}

	shared.CountFilter(ctx, req.FilterByAttribute)

	req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(
		ctx,
		a.configClient,
		parent.CustomerID,
		a.resourceACLHelperProvider.Get(),
		req.FilterByAttribute.Users,
		req.FilterByAttribute.Groups,
		false,
	)
	if err != nil {
		return nil, err
	}

	if len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers {
		includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
		req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			req.FilterByAttribute,
			includeDirectGroupMembershipsOnly,
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		if len(req.FilterByAttribute.Users) == 0 {
			return &analyticspb.RetrieveScorecardCriteriaStatsResponse{}, nil
		}
	}

	scorecardTemplateIDs, err := qa.ExtractScorecardTemplateIDs(req.FilterByAttribute.ScorecardTemplateNames)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid scorecard template names: %v", err)
	}

	behaviorToCriterionMap, err := buildBehaviorToCriterionMappings(db, parent, req.FilterByAttribute.UsecaseNames, req.FilterByAttribute.Behaviors, scorecardTemplateIDs)
	if err != nil {
		return nil, err
	}
	criterionIDs := fnutils.Flatten(fn.MapValueToSlice(behaviorToCriterionMap))

	if len(criterionIDs) == 0 && len(req.FilterByAttribute.Behaviors) != 0 {
		return nil, nil
	}

	scoreStats, err := queryAutoScoresWithCriteria(db, parent, req.FilterByAttribute, scorecardTemplateIDs, req.FilterByTimeRange, criterionIDs)
	if err != nil {
		return nil, err
	}

	shouldGroupByBehavior := slices.Contains(req.GroupByAttributes, analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR)

	var overallAutoScoreCount, overallOverwriteCount int64
	for _, row := range scoreStats {
		overallAutoScoreCount += row.TotalAutoScoreCount
		overallOverwriteCount += row.OverwrittenCount
	}

	response, err := buildResponse(parent, behaviorToCriterionMap, scoreStats, overallAutoScoreCount, overallOverwriteCount, shouldGroupByBehavior)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// queryAutoScoresWithCriteria returns per-criterion total auto scores and
// overwritten count within submitted scorecards.
// When criterionIdentifiers is empty (e.g. no behaviors specified and no templates),
// the query returns stats for all criteria in submitted scorecards.
func queryAutoScoresWithCriteria(db *gorm.DB, parent *shared.CustomerProfile, filter *analyticspb.Attribute, scorecardTemplateIDs []string, timeRange *analyticspb.TimeRange, criterionIdentifiers []string) ([]criteriaScoreStats, error) {
	scSub := db.Table("director.scorecards AS sc").
		Select("sc.resource_id").
		Where("sc.customer = ?", parent.CustomerID).
		Where("sc.profile = ?", parent.ProfileID).
		Where("sc.submitted_at IS NOT NULL").
		Where("sc.calibrated_scorecard_id IS NULL").
		Where("COALESCE(sc.scorecard_type, 0) = 0")

	if timeRange != nil {
		scSub = scSub.Joins("JOIN app.chats c ON c.conversation_id = sc.conversation_id")
		if timeRange.StartTimestamp != nil {
			scSub = scSub.Where("c.started_at >= ?", timeRange.StartTimestamp.AsTime())
		}
		if timeRange.EndTimestamp != nil {
			scSub = scSub.Where("c.started_at <= ?", timeRange.EndTimestamp.AsTime())
		}
	}

	if filter != nil {
		if len(filter.Users) > 0 {
			if userIDs, err := shared.ConvertUsersToUserIDs(filter.Users); err == nil && len(userIDs) > 0 {
				scSub = scSub.Where("sc.agent_user_id IN (?)", userIDs)
			}
		}
		if len(filter.UsecaseNames) > 0 {
			usecaseIDs := make([]string, 0, len(filter.UsecaseNames))
			for _, usecaseName := range filter.UsecaseNames {
				parsedUsecase, err := customerpb.ParseUsecaseName(usecaseName)
				if err != nil {
					continue
				}
				usecaseIDs = append(usecaseIDs, parsedUsecase.GetUsecaseID())
			}
			if len(usecaseIDs) > 0 {
				scSub = scSub.Where("sc.usecase_id IN (?)", usecaseIDs)
			}
		}
	}

	if len(scorecardTemplateIDs) > 0 {
		scSub = scSub.Where("sc.template_id IN (?)", scorecardTemplateIDs)
	}
	query := db.Table("(?) AS sc", scSub).
		Select("s.criterion_identifier AS criterion_identifier, COUNT(*) AS total_auto_score_count, SUM(CASE WHEN s.ai_value IS DISTINCT FROM s.numeric_value THEN 1 ELSE 0 END) AS overwritten_count").
		Joins("JOIN director.scores s ON s.customer = ? AND s.profile = ? AND s.scorecard_id = sc.resource_id", parent.CustomerID, parent.ProfileID).
		Where("s.ai_scored = TRUE")
	if len(criterionIdentifiers) > 0 {
		query = query.Where("s.criterion_identifier IN (?)", criterionIdentifiers)
	}
	query = query.Group("s.criterion_identifier")

	var results []criteriaScoreStats
	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func buildBehaviorToCriterionMappings(db *gorm.DB, parent *shared.CustomerProfile, usecaseNames []string, requestedBehaviors []*behaviorpb.Behavior, scorecardTemplateIDs []string) (map[string][]string, error) {
	profileName := &customerpb.ProfileName{
		CustomerID: parent.CustomerID,
		ProfileID:  parent.ProfileID,
	}

	requestedBehaviorIDs, err := fn.MapE(requestedBehaviors, func(behavior *behaviorpb.Behavior) (string, error) {
		parsedBehaviorName, err := behaviorpb.ParseBehaviorName(behavior.GetName())
		if err != nil {
			return "", err
		}
		return parsedBehaviorName.GetBehaviorID(), nil
	})
	if err != nil {
		return nil, err
	}

	requestedBehaviorIDsSet := set.NewFromSlice(requestedBehaviorIDs)

	scorecardTemplateIDToTemplateMap, err := qa.ListCurrentScorecardTemplateIDs(
		db, profileName, usecaseNames, scorecardTemplateIDs, false /* filterActive */)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list scorecard template IDs: %v", err)
	}

	behaviorToCriterionMap := make(map[string][]string)

	for _, template := range scorecardTemplateIDToTemplateMap {
		templateStructure, err := scoring.ParseScorecardTemplateStructure(template.Template)
		if err != nil {
			continue
		}

		criteria := templateStructure.GetCriteriaSlice(false /*rootOnly*/)
		for _, criterion := range criteria {
			autoQA := criterion.GetAutoQA()
			if autoQA == nil || autoQA.Triggers == nil {
				continue
			}

			criterionID := criterion.GetIdentifier()

			for _, trigger := range *autoQA.Triggers {
				if trigger.Type == scoring.TriggerTypeBehavior {
					behaviorName, err := behaviorpb.ParseBehaviorName(trigger.ResourceName)
					if err != nil {
						return nil, status.Errorf(codes.Internal, "failed to parse behavior name: %v", err)
					}

					behaviorID := behaviorName.GetBehaviorID()

					if requestedBehaviorIDsSet.Len() == 0 || requestedBehaviorIDsSet.Has(behaviorID) {
						if behaviorToCriterionMap[behaviorID] == nil {
							behaviorToCriterionMap[behaviorID] = make([]string, 0)
						}
						behaviorToCriterionMap[behaviorID] = append(behaviorToCriterionMap[behaviorID], criterionID)
					}
				}
			}
		}
	}

	return behaviorToCriterionMap, nil
}

func buildResponse(parent *shared.CustomerProfile, behaviorToCriterionMap map[string][]string, scoreStats []criteriaScoreStats, overallAutoScoreCount, overallOverwriteCount int64, shouldGroupByBehavior bool) (*analyticspb.RetrieveScorecardCriteriaStatsResponse, error) {
	response := &analyticspb.RetrieveScorecardCriteriaStatsResponse{
		OverallCriteriaStats: &analyticspb.ScorecardCriteriaStats{
			CriteriaManualOverwriteCount: overallOverwriteCount,
		},
	}

	if overallAutoScoreCount > 0 {
		response.OverallCriteriaStats.CriteriaManualOverwritePercent = safeDivideFloat64(overallOverwriteCount, overallAutoScoreCount) * 100.0
	}

	if shouldGroupByBehavior {
		behaviorToCriteriaStats := make(map[string][]criteriaScoreStats)
		for behaviorID, criterionIDs := range behaviorToCriterionMap {
			var stats []criteriaScoreStats
			for _, row := range scoreStats {
				if len(criterionIDs) > 0 && slices.Contains(criterionIDs, row.CriterionIdentifier) {
					stats = append(stats, row)
				}
			}
			if len(stats) > 0 {
				behaviorToCriteriaStats[behaviorID] = stats
			}
		}

		criteriaStats := make([]*analyticspb.ScorecardCriteriaStats, 0, len(behaviorToCriteriaStats))
		for behaviorID, stats := range behaviorToCriteriaStats {
			var totalAutoScoreCount, overwriteCount int64
			for _, row := range stats {
				totalAutoScoreCount += row.TotalAutoScoreCount
				overwriteCount += row.OverwrittenCount
			}
			entry := &analyticspb.ScorecardCriteriaStats{
				Attribute: &analyticspb.Attribute{
					Behaviors: []*behaviorpb.Behavior{{
						Name: behaviorpb.BehaviorName{
							CustomerID: parent.CustomerID,
							ProfileID:  parent.ProfileID,
							BehaviorID: behaviorID,
						}.String(),
					}},
				},
				CriteriaManualOverwriteCount: overwriteCount,
			}
			if totalAutoScoreCount > 0 {
				entry.CriteriaManualOverwritePercent = safeDivideFloat64(overwriteCount, totalAutoScoreCount) * 100.0
			}
			criteriaStats = append(criteriaStats, entry)
		}
		response.CriteriaStats = criteriaStats
	}

	return response, nil
}
