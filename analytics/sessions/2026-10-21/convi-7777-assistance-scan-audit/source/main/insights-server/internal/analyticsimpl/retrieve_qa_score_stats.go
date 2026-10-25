package analyticsimpl

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/util/errgroup"

	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"
	"github.com/cresta/shared-go/container/set"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/utils"
)

// defaultMaxConcurrentQaScoreStatsQueries bounds how many per-template-group ClickHouse queries
// RetrieveQAScoreStats issues at once. It is the fallback when the
// MAX_CONCURRENT_QA_SCORE_STATS_QUERIES env var is unset or invalid; see AnalyticsServiceImpl.Init.
const defaultMaxConcurrentQaScoreStatsQueries = 8

var agentTiers = []analyticspb.AgentTier{
	analyticspb.AgentTier_BOTTOM_AGENTS,
	analyticspb.AgentTier_AVERAGE_AGENTS,
	analyticspb.AgentTier_TOP_AGENTS,
}

var quintileRanks = []analyticspb.QuintileRank{
	analyticspb.QuintileRank_QUINTILE_RANK_1,
	analyticspb.QuintileRank_QUINTILE_RANK_2,
	analyticspb.QuintileRank_QUINTILE_RANK_3,
	analyticspb.QuintileRank_QUINTILE_RANK_4,
	analyticspb.QuintileRank_QUINTILE_RANK_5,
}

var allowedAgentOnlyScorecardStatuses = []analyticspb.QAAttribute_ScorecardStatus{
	analyticspb.QAAttribute_AUTO,
	analyticspb.QAAttribute_PUBLISHED,
}

func (a AnalyticsServiceImpl) RetrieveQAScoreStats(
	ctx context.Context,
	req *analyticspb.RetrieveQAScoreStatsRequest,
) (resp *analyticspb.RetrieveQAScoreStatsResponse, err error) {
	a.logger.Infof(ctx, "value of DEFAULT_GRPC_GATEWAY_TIMEOUT is %q", os.Getenv("DEFAULT_GRPC_GATEWAY_TIMEOUT"))
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	groupByFlag := postgres.CreateEnumFlag(req.GroupByAttributeTypes)
	if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER) && postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT) {
		return nil, status.Errorf(codes.InvalidArgument, "Not supporting `agent tier` with `agent` group attribute: %s.", req.GroupByAttributeTypes)
	}
	if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER) && postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER) {
		return nil, status.Errorf(codes.InvalidArgument, "Not supporting `agent tier` with `scorecard submitter` group attribute: %s.", req.GroupByAttributeTypes)
	}
	if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT) && postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER) {
		return nil, status.Errorf(codes.InvalidArgument, "Not supporting `agent` with `scorecard submitter` group attribute: %s.", req.GroupByAttributeTypes)
	}
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.QAAttribute{}
	}

	var (
		users                             []*userpb.User
		groupsToAggregate                 []*userpb.Group
		userNameToGroupNamesMap           map[string][]string
		hasAgentAsGroupByKey              = postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT)
		hasScorecardSubmitterAsGroupByKey = postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER)
		includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
	)

	// New implementation using ParseUserFilterForAnalytics
	listAgentOnly := req.GetFilterToAgentsOnly() // QA score stats track agent performance
	excludeDeactivatedUsers := req.FilterByAttribute != nil && req.FilterByAttribute.GetExcludeDeactivatedUsers()

	// Always pass the group filter to ParseUserFilterForAnalytics for user filtering.
	//
	// Historical context: The old code only passed groups when grouping by GROUP only (not AGENT).
	// The reasoning was: when grouping by AGENT with both user and group filters, the result should
	// include stats for both the explicitly filtered agents AND all agents in the filtered groups.
	// If groups were passed to ListUsersMappedToGroups, it would only return users from those groups,
	// because the old code only have the ability to filter by groups, not by agents, therefore,
	// potentially missing explicitly filtered agents who aren't in those groups.
	//
	// However, ParseUserFilterForAnalytics handles this differently - it has the ability to filter by agents,
	// and groups at the same time, therefore, it doesn't need to pass the group filter to ListUsersMappedToGroups.
	// The hasAgentAsGroupByKey flag controls the group mapping logic inside the ParseUserFilterForAnalytics function.
	result, err := ParseUserFilterForAnalytics(
		ctx,
		a.logger,
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		parent.CustomerID,
		parent.ProfileID,
		req.FilterByAttribute.GetUsers(),
		req.FilterByAttribute.GetGroups(),
		hasAgentAsGroupByKey,
		includeDirectGroupMembershipsOnly,
		listAgentOnly,
		req.GetIncludePeerUserStats(),
		excludeDeactivatedUsers,
	)
	if err != nil {
		return nil, err
	}

	// Extract values from result
	userNameToGroupNamesMap = result.UserNameToGroupNamesMap
	groupsToAggregate = result.GroupsToAggregate
	users = result.FinalUsers

	if !postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT) &&
		!postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_GROUP) &&
		!hasScorecardSubmitterAsGroupByKey {
		users = []*userpb.User{}
		groupsToAggregate = []*userpb.Group{}
		userNameToGroupNamesMap = map[string][]string{}
	}

	if shouldEarlyReturn := ApplyUserFilterFromResult(result, &req.FilterByAttribute.Users, &req.FilterByAttribute.Groups, a.enableExtTableForUserFilter); shouldEarlyReturn {
		return &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{},
		}, nil
	}
	submitterResult, shouldEarlyReturn, err := a.resolveScorecardReviewerAudience(ctx, parent, req.FilterByAttribute, includeDirectGroupMembershipsOnly)
	if err != nil {
		return nil, err
	} else if shouldEarlyReturn {
		return &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{},
		}, nil
	}
	if hasScorecardSubmitterAsGroupByKey {
		users = []*userpb.User{}
		if submitterResult != nil {
			users = submitterResult.FinalUsers
		}
	}

	if err := a.applyScorecardStatusFilter(ctx, req, parent); err != nil {
		return nil, err
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveQAScoreStatsRequest)
	sortQAAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveQAScoreStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveQAScoreStats", insightsQAScoreStatsCachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_GROUP) {
		hasTimeRangeAsGroupKey := postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE)
		hasCriterionAsGroupKey := postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION)
		numExtraGroupings := 0
		if hasTimeRangeAsGroupKey {
			numExtraGroupings++
		}
		if hasCriterionAsGroupKey {
			numExtraGroupings++
		}
		if numExtraGroupings+1 != len(req.GroupByAttributeTypes) {
			return nil, status.Errorf(codes.InvalidArgument, "when grouping by `group`, only `time_range` and `criterion` are supported: %s.", req.GroupByAttributeTypes)
		}
		return a.retrieveQAScoreStatsInternalForGroupsInSingleQuery(
			ctx, req, parent, users, groupsToAggregate, userNameToGroupNamesMap, hasTimeRangeAsGroupKey, hasCriterionAsGroupKey)
	}
	return a.retrieveQAScoreStatsInternal(ctx, req, parent, users, groupsToAggregate, userNameToGroupNamesMap)
}

func (a AnalyticsServiceImpl) retrieveQAScoreStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveQAScoreStatsRequest,
	parent *shared.CustomerProfile,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
	hasTimeRangeAsGroupKey bool,
	hasCriterionAsGroupKey bool,
) (*analyticspb.RetrieveQAScoreStatsResponse, error) {
	// Retrieve per-agent QA score stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveQAScoreStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT}
	if hasTimeRangeAsGroupKey {
		perUserReqForAll.GroupByAttributeTypes = append(perUserReqForAll.GroupByAttributeTypes, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE)
	}
	if hasCriterionAsGroupKey {
		perUserReqForAll.GroupByAttributeTypes = append(perUserReqForAll.GroupByAttributeTypes, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION)
	}
	if len(req.FilterByAttribute.Groups) > 0 {
		// Only add user filters when group filter is not empty to avoid user list being too long.
		perUserReqForAll.FilterByAttribute.Users = append(perUserReqForAll.FilterByAttribute.Users, users...)
	}
	perUserResp, err := a.retrieveQAScoreStatsInternal(ctx, perUserReqForAll, parent, users, groups, userNameToGroupNamesMap)
	if err != nil {
		return nil, err
	}
	return convertRowsPerUserToPerGroupQAScoreStatsResponse(perUserResp, groups, userNameToGroupNamesMap, hasTimeRangeAsGroupKey, hasCriterionAsGroupKey)
}

func (a AnalyticsServiceImpl) retrieveQAScoreStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveQAScoreStatsRequest,
	parent *shared.CustomerProfile,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveQAScoreStatsResponse, error) {
	var err error
	if req.ScoreResource != analyticspb.RetrieveQAScoreStatsRequest_QA_SCORE_RESOURCE_SCORECARD {
		criteriaIDFilters := req.GetFilterByAttribute().GetCriterionIdentifiers()
		groupByFlag := postgres.CreateEnumFlag(req.GroupByAttributeTypes)
		if len(criteriaIDFilters) == 0 && !postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION) {
			scoreableCriteria, err := a.getScoreableCriteria(
				ctx,
				&customerpb.ProfileName{
					CustomerID: parent.CustomerID,
					ProfileID:  parent.ProfileID,
				},
				req.GetFilterByAttribute().GetScorecardTemplates(),
				req.GetFilterByAttribute().GetUsecaseNames())
			if err != nil {
				return nil, err
			}
			req.FilterByAttribute.CriterionIdentifiers = scoreableCriteria
		}
	}

	groupByFlag := postgres.CreateEnumFlag(req.GroupByAttributeTypes)

	// Resolve lower-is-better criteria before querying ClickHouse so that
	// the criterion IDs can be passed as an ext table for the ranking column.
	needsDirectionality := a.enableDirectionality &&
		(postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER) ||
			postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT))
	var lowerIsBetterCriteria set.Set[string]
	if needsDirectionality {
		profileName := parent.ToPBProfileName()
		scorecardTemplateIDs, sErr := qa.ExtractScorecardTemplateIDs(req.GetFilterByAttribute().GetScorecardTemplates())
		if sErr != nil {
			a.logger.Errorf(ctx, "failed to extract scorecard template IDs for directionality: %v", sErr)
		} else {
			templates, tErr := qa.ListCurrentScorecardTemplateIDs(
				a.appsDB.DB(ctx), profileName, req.GetFilterByAttribute().GetUsecaseNames(), scorecardTemplateIDs, false)
			if tErr != nil {
				a.logger.Errorf(ctx, "failed to list scorecard templates for directionality: %v", tErr)
			} else {
				var lErr error
				lowerIsBetterCriteria, lErr = scoring.BuildLowerIsBetterCriterionSet(a.appsDB.DB(ctx), profileName, templates)
				if lErr != nil {
					a.logger.Errorf(ctx, "failed to build lower-is-better criteria set: %v", lErr)
				}
			}
		}
	}

	// Convert set to slice for passing to CH query (ext table).
	var lowerIsBetterCriteriaSlice []string
	if lowerIsBetterCriteria != nil && lowerIsBetterCriteria.Len() > 0 {
		lowerIsBetterCriteriaSlice = lowerIsBetterCriteria.Slice()
	}

	var result *analyticspb.RetrieveQAScoreStatsResponse
	var rankingScores map[string]float64
	if req.RespectTemplateQaScoreConfig {
		if !postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION) {
			return nil, status.Errorf(codes.InvalidArgument, "need to have group by criterion with respectTemplateQaScoreConfig is enabled.")
		}
		if len(req.GetFilterByAttribute().GetConversationDurationBuckets()) > 0 {
			return nil, status.Errorf(codes.InvalidArgument, "should not have conversation duration bucket filter set when RespectTemplateQaScoreConfig is enabled.")
		}

		var chResult *qaScoreStatsResult
		chResult, err = a.readQaScoreStatsRespectingTemplateQaScoreConfig(ctx, users, req, lowerIsBetterCriteriaSlice)
		if err != nil {
			return nil, err
		}
		result = chResult.response
		rankingScores = chResult.rankingScores
	} else {
		// Fetch QA score stats from ClickHouse.
		chResult, chErr := a.readQaScoreStatsFromClickhouse(ctx, req, users, lowerIsBetterCriteriaSlice)
		if chErr != nil {
			return nil, chErr
		}
		result = chResult.response
		rankingScores = chResult.rankingScores
	}

	if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER) {
		return aggregateTopAgentsResponse(result, rankingScores)
	}
	if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT) {
		setQuintileRankForPerAgentScores(result, rankingScores)
		if req.GetDeduplicateUsers() {
			return deduplicateUsersInResponse(result, users, groups, userNameToGroupNamesMap), nil
		}
		return appendGroupMemberships(result, users, groups, userNameToGroupNamesMap), nil
	}
	return result, nil
}

func aggregateTopAgentsResponse(response *analyticspb.RetrieveQAScoreStatsResponse, rankingScores map[string]float64) (*analyticspb.RetrieveQAScoreStatsResponse, error) {
	scores := response.QaScoreResult.Scores
	scoreMap := utils.GroupBy(scores, qaScoreGroupedByToStringKey)

	aggregatedScores := []*analyticspb.QAScore{}
	conversationVolumeMetric := func(qaScore *analyticspb.QAScore) int32 {
		return qaScore.TotalScorecardCount
	}
	for _, scoreSlice := range scoreMap {
		scoreMetric := func(qaScore *analyticspb.QAScore) float32 {
			if rankingScores != nil {
				if rs, ok := rankingScores[rankingScoreKey(qaScore)]; ok {
					return float32(rs)
				}
			}
			return qaScore.Score
		}
		agentParitions := utils.PartitionUsingVolumeAndMetric(scoreSlice, scoreMetric, conversationVolumeMetric, []float32{bottomAgentPercentage, bottomAgentPercentage + AvgAgentPercentage})
		if len(agentParitions) != 3 {
			return nil, fmt.Errorf("expected 3 partitions, got %d", len(agentParitions))
		}
		groupedBy := scoreSlice[0].GroupedBy
		for i, partition := range agentParitions {
			aggregatedScores = append(aggregatedScores, createTieredScoreObject(partition, groupedBy, agentTiers[i]))
		}
	}

	response.QaScoreResult.Scores = aggregatedScores
	return response, nil
}

func appendGroupMemberships(
	perUserResp *analyticspb.RetrieveQAScoreStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) *analyticspb.RetrieveQAScoreStatsResponse {
	// Build the userName to user and groupName to group maps.
	userNameToUserMap := make(map[string]*userpb.User)
	for _, user := range users {
		userNameToUserMap[user.Name] = user
	}
	groupNameToGroupMap := make(map[string]*userpb.Group)
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	// Append the groups into the groupby user of per-user resp.
	scoresPerUserWithGroupMemberships := []*analyticspb.QAScore{}
	for _, score := range perUserResp.QaScoreResult.Scores {
		if score.GroupedBy.User == nil {
			continue
		}
		userName := score.GroupedBy.User.Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					score.GroupedBy.User.GroupMemberships = append(
						score.GroupedBy.User.GroupMemberships,
						&userpb.GroupMembership{
							Group:            group.Name,
							GroupDisplayName: group.DisplayName,
							GroupType:        group.GetType(),
						})
				}
			}
		}
		scoresPerUserWithGroupMemberships = append(scoresPerUserWithGroupMemberships, score)
	}

	return &analyticspb.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analyticspb.QAScoreResult{
			Scores:                 scoresPerUserWithGroupMemberships,
			AverageQaScore:         perUserResp.QaScoreResult.AverageQaScore,
			TotalConversationCount: perUserResp.QaScoreResult.TotalConversationCount,
			TotalScorecardCount:    perUserResp.QaScoreResult.TotalScorecardCount,
		},
	}
}

// deduplicateUsersInResponse moves per-row user identity into a single
// QAScoreResult.Users map keyed by resource name. Each per-row
// QAScoreGroupBy.User is stripped to carry only the Name field as a lookup
// key. This eliminates the O(rows × agent-identity-size) payload bloat that
// occurs when the same agent appears across many (criterion, day) rows.
func deduplicateUsersInResponse(
	perUserResp *analyticspb.RetrieveQAScoreStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) *analyticspb.RetrieveQAScoreStatsResponse {
	groupNameToGroupMap := make(map[string]*userpb.Group, len(groups))
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	usersMap := make(map[string]*userpb.User)
	dedupedScores := make([]*analyticspb.QAScore, 0, len(perUserResp.GetQaScoreResult().GetScores()))
	for _, score := range perUserResp.GetQaScoreResult().GetScores() {
		if score.GetGroupedBy().GetUser() == nil {
			continue
		}
		userName := score.GroupedBy.User.Name

		if _, exists := usersMap[userName]; !exists {
			entry := &userpb.User{
				Name:        score.GroupedBy.User.Name,
				FullName:    score.GroupedBy.User.FullName,
				DisplayName: score.GroupedBy.User.DisplayName,
				Username:    score.GroupedBy.User.Username,
			}
			if groupNames, ok := userNameToGroupNamesMap[userName]; ok {
				for _, groupName := range groupNames {
					if group, ok := groupNameToGroupMap[groupName]; ok {
						entry.GroupMemberships = append(entry.GroupMemberships, &userpb.GroupMembership{
							Group:            group.Name,
							GroupDisplayName: group.DisplayName,
							GroupType:        group.GetType(),
						})
					}
				}
			}
			usersMap[userName] = entry
		}

		score.GroupedBy.User = &userpb.User{Name: userName}
		dedupedScores = append(dedupedScores, score)
	}

	return &analyticspb.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analyticspb.QAScoreResult{
			Scores:                 dedupedScores,
			AverageQaScore:         perUserResp.GetQaScoreResult().GetAverageQaScore(),
			TotalConversationCount: perUserResp.GetQaScoreResult().GetTotalConversationCount(),
			TotalScorecardCount:    perUserResp.GetQaScoreResult().GetTotalScorecardCount(),
			Users:                  usersMap,
		},
	}
}

func createTieredScoreObject(scores []*analyticspb.QAScore, groupedBy *analyticspb.QAScoreGroupBy, agentTier analyticspb.AgentTier) *analyticspb.QAScore {
	groupedByCopy := proto.Clone(groupedBy).(*analyticspb.QAScoreGroupBy)
	groupedByCopy.AgentTier = agentTier
	groupedByCopy.User = nil
	groupedByCopy.QuintileRank = analyticspb.QuintileRank_QUINTILE_RANK_UNSPECIFIED
	if len(scores) == 0 {
		return &analyticspb.QAScore{
			GroupedBy: groupedByCopy,
		}
	}
	aggregatedScores := aggregateScores(scores)
	aggregatedScores.GroupedBy = groupedByCopy
	return aggregatedScores
}

// Creates a composite map key from the groupedBy object of the given score
// Ignores the user field since we want to aggregate the values for the agents.
func qaScoreGroupedByToStringKey(qaScore *analyticspb.QAScore) string {
	groupedBy := qaScore.GroupedBy
	return strings.Join([]string{
		groupedBy.GetTimeRange().String(),
		groupedBy.GetTeam().GetName(),
		groupedBy.GetGroup().GetName(),
		groupedBy.GetScorecardTemplateId(),
		groupedBy.GetCriterionId(),
	}, "-")
}

// criterionMapKey builds the composite key identifying a criterion within a score's groupedBy.
// Criterion ids aren't unique across templates (CONVI-7014), so a criterion is keyed by
// (scorecard_template_id, criterion_id).
func criterionMapKey(groupedBy *analyticspb.QAScoreGroupBy) string {
	return groupedBy.GetScorecardTemplateId() + "|" + groupedBy.GetCriterionId()
}

func rankingScoreKey(qaScore *analyticspb.QAScore) string {
	gb := qaScore.GetGroupedBy()
	userName := gb.GetUser().GetName()
	criterionKey := criterionMapKey(gb)
	timeRange := ""
	if tr := gb.GetTimeRange(); tr != nil {
		timeRange = tr.GetStartTimestamp().String()
	}
	return userName + "|" + criterionKey + "|" + timeRange
}

func aggregateScores(scores []*analyticspb.QAScore) *analyticspb.QAScore {
	if len(scores) == 0 {
		return &analyticspb.QAScore{}
	}
	var totalWeightSum, totalWeightedPercentage float32
	var totalConversationCount int32
	var totalScorecardCount int32
	for _, score := range scores {
		totalWeightSum += score.WeightSum
		totalWeightedPercentage += score.Score * score.WeightSum
		totalConversationCount += score.TotalConversationCount
		totalScorecardCount += score.TotalScorecardCount
	}
	groupBy := proto.Clone(scores[0].GroupedBy)
	return &analyticspb.QAScore{
		GroupedBy:              groupBy.(*analyticspb.QAScoreGroupBy),
		Score:                  safeDivideFloat(totalWeightedPercentage, totalWeightSum),
		TotalConversationCount: totalConversationCount,
		TotalScorecardCount:    totalScorecardCount,
		WeightSum:              totalWeightSum,
	}
}

func convertRowsPerUserToPerGroupQAScoreStatsResponse(
	perUserResp *analyticspb.RetrieveQAScoreStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
	hasTimeRangeAsGroupKey bool,
	hasCriterionAsGroupKey bool,
) (*analyticspb.RetrieveQAScoreStatsResponse, error) {
	groupMap := make(map[string]*userpb.Group)
	for _, group := range groups {
		groupMap[group.GetName()] = group
	}

	// Initialize the per-group QA score results.
	qaScoreResultByGroup := make(map[string]*analyticspb.QAScoreResult)
	for groupName := range groupMap {
		qaScoreResultByGroup[groupName] = &analyticspb.QAScoreResult{
			Scores: []*analyticspb.QAScore{},
		}
	}

	// Append per-user stats to per-group stats.
	for _, score := range perUserResp.QaScoreResult.Scores {
		if score.GroupedBy.User == nil || len(score.GroupedBy.User.Name) == 0 {
			return nil, status.Errorf(codes.FailedPrecondition, "empty user attribute.")
		}
		if groupNames, exists := userNameToGroupNamesMap[score.GroupedBy.User.Name]; exists {
			for _, groupName := range groupNames {
				if _, exists := qaScoreResultByGroup[groupName]; exists {
					qaScoreResultByGroup[groupName].Scores = append(qaScoreResultByGroup[groupName].Scores, score)
				}
			}
		}
	}

	// Assemble per-group stats.
	qaScorePerGroup := []*analyticspb.QAScore{}
	for groupName, qaScoreResult := range qaScoreResultByGroup {
		if len(qaScoreResult.Scores) == 0 {
			continue
		}
		switch {
		case !hasTimeRangeAsGroupKey && !hasCriterionAsGroupKey:
			qaScore := convertPerUserScoresToPerGroupScore(qaScoreResult.Scores, groupMap[groupName])
			qaScorePerGroup = append(qaScorePerGroup, qaScore)
		case hasTimeRangeAsGroupKey && !hasCriterionAsGroupKey:
			qaScores := convertPerUserPerTimeRangeScoresToPerGroupPerTimeRangeScores(qaScoreResult.Scores, groupMap[groupName])
			qaScorePerGroup = append(qaScorePerGroup, qaScores...)
		case !hasTimeRangeAsGroupKey && hasCriterionAsGroupKey:
			qaScores := convertPerUserPerCriterionScoresToPerGroupPerCriterionScores(qaScoreResult.Scores, groupMap[groupName])
			qaScorePerGroup = append(qaScorePerGroup, qaScores...)
		case hasTimeRangeAsGroupKey && hasCriterionAsGroupKey:
			qaScores := convertPerUserPerTimeRangePerCriterionScoresToPerGroupPerTimeRangePerCriterionScores(
				qaScoreResult.Scores, groupMap[groupName])
			qaScorePerGroup = append(qaScorePerGroup, qaScores...)
		}
	}

	return &analyticspb.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analyticspb.QAScoreResult{
			Scores:                 qaScorePerGroup,
			AverageQaScore:         perUserResp.QaScoreResult.AverageQaScore,
			TotalConversationCount: perUserResp.QaScoreResult.TotalConversationCount,
			TotalScorecardCount:    perUserResp.QaScoreResult.TotalScorecardCount,
		},
	}, nil
}

func convertPerUserScoresToPerGroupScore(qaScoresPerUser []*analyticspb.QAScore, group *userpb.Group) *analyticspb.QAScore {
	// Initial and append per-frequency stats per group.
	totalWeights := float32(0)
	totalWeightedScores := float32(0)
	totalConversationCount := int32(0)
	totalScorecardCount := int32(0)
	for _, qaScore := range qaScoresPerUser {
		totalWeights += qaScore.WeightSum
		totalWeightedScores += qaScore.WeightSum * qaScore.Score
		totalConversationCount += qaScore.TotalConversationCount
		totalScorecardCount += qaScore.TotalScorecardCount
	}
	return &analyticspb.QAScore{
		GroupedBy:              &analyticspb.QAScoreGroupBy{Group: group},
		Score:                  safeDivideFloat(totalWeightedScores, totalWeights),
		WeightSum:              totalWeights,
		TotalConversationCount: totalConversationCount,
		TotalScorecardCount:    totalScorecardCount,
	}
}

func convertPerUserPerTimeRangeScoresToPerGroupPerTimeRangeScores(qaScoresPerUserPerTimeRange []*analyticspb.QAScore, group *userpb.Group) []*analyticspb.QAScore {
	qaScoresPerTimeRange := make(map[int64]*analyticspb.QAScore)
	for _, qaScore := range qaScoresPerUserPerTimeRange {
		seconds := qaScore.GroupedBy.TimeRange.StartTimestamp.Seconds
		if _, exists := qaScoresPerTimeRange[seconds]; !exists {
			group.Members = []*userpb.GroupMembership{}
			qaScoresPerTimeRange[seconds] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					Group:     group,
					TimeRange: qaScore.GroupedBy.TimeRange,
				},
			}
		}
		qaScoresPerTimeRange[seconds].Score += qaScore.WeightSum * qaScore.Score
		qaScoresPerTimeRange[seconds].WeightSum += qaScore.WeightSum
		qaScoresPerTimeRange[seconds].TotalConversationCount += qaScore.TotalConversationCount
		qaScoresPerTimeRange[seconds].TotalScorecardCount += qaScore.TotalScorecardCount
	}

	qaScores := []*analyticspb.QAScore{}
	for _, qaScore := range qaScoresPerTimeRange {
		qaScore.Score = safeDivideFloat(qaScore.Score, qaScore.WeightSum)
		qaScores = append(qaScores, qaScore)
	}
	return qaScores
}

func convertPerUserPerCriterionScoresToPerGroupPerCriterionScores(qaScoresPerUserPerCriterion []*analyticspb.QAScore, group *userpb.Group) []*analyticspb.QAScore {
	qaScoresPerCriterion := make(map[string]*analyticspb.QAScore)
	for _, qaScore := range qaScoresPerUserPerCriterion {
		scorecardTemplateID := qaScore.GroupedBy.ScorecardTemplateId
		criterionID := qaScore.GroupedBy.CriterionId
		mapKey := criterionMapKey(qaScore.GroupedBy)
		if _, exists := qaScoresPerCriterion[mapKey]; !exists {
			qaScoresPerCriterion[mapKey] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					Group:               group,
					CriterionId:         criterionID,
					ScorecardTemplateId: scorecardTemplateID,
				},
			}
		}
		qaScoresPerCriterion[mapKey].Score += qaScore.WeightSum * qaScore.Score
		qaScoresPerCriterion[mapKey].WeightSum += qaScore.WeightSum
		qaScoresPerCriterion[mapKey].TotalConversationCount += qaScore.TotalConversationCount
		qaScoresPerCriterion[mapKey].TotalScorecardCount += qaScore.TotalScorecardCount
	}

	qaScores := []*analyticspb.QAScore{}
	for _, qaScore := range qaScoresPerCriterion {
		qaScore.Score = safeDivideFloat(qaScore.Score, qaScore.WeightSum)
		qaScores = append(qaScores, qaScore)
	}
	return qaScores
}

func convertPerUserPerTimeRangePerCriterionScoresToPerGroupPerTimeRangePerCriterionScores(
	qaScoresPerUserPerTimeRangePerCriterion []*analyticspb.QAScore, group *userpb.Group,
) []*analyticspb.QAScore {
	qaScoresPerTimeRangePerCriterion := make(map[int64]map[string]*analyticspb.QAScore)
	for _, qaScore := range qaScoresPerUserPerTimeRangePerCriterion {
		seconds := qaScore.GroupedBy.TimeRange.StartTimestamp.Seconds
		scorecardTemplateID := qaScore.GroupedBy.ScorecardTemplateId
		criterionID := qaScore.GroupedBy.CriterionId
		mapKey := criterionMapKey(qaScore.GroupedBy)
		if _, exists := qaScoresPerTimeRangePerCriterion[seconds]; !exists {
			qaScoresPerTimeRangePerCriterion[seconds] = make(map[string]*analyticspb.QAScore)
		}
		if _, exists := qaScoresPerTimeRangePerCriterion[seconds][mapKey]; !exists {
			group.Members = []*userpb.GroupMembership{}
			qaScoresPerTimeRangePerCriterion[seconds][mapKey] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					Group:               group,
					TimeRange:           qaScore.GroupedBy.TimeRange,
					CriterionId:         criterionID,
					ScorecardTemplateId: scorecardTemplateID,
				},
			}
		}
		qaScoresPerTimeRangePerCriterion[seconds][mapKey].Score += qaScore.WeightSum * qaScore.Score
		qaScoresPerTimeRangePerCriterion[seconds][mapKey].WeightSum += qaScore.WeightSum
		qaScoresPerTimeRangePerCriterion[seconds][mapKey].TotalConversationCount += qaScore.TotalConversationCount
		qaScoresPerTimeRangePerCriterion[seconds][mapKey].TotalScorecardCount += qaScore.TotalScorecardCount
	}

	qaScores := []*analyticspb.QAScore{}
	for _, qaScoresPerTimeRange := range qaScoresPerTimeRangePerCriterion {
		for _, qaScore := range qaScoresPerTimeRange {
			qaScore.Score = safeDivideFloat(qaScore.Score, qaScore.WeightSum)
			qaScores = append(qaScores, qaScore)
		}
	}
	return qaScores
}

// setQuintileRankForPerAgentScores assigns a percentile-based quintile rank
// (Q1 = top 20%, Q5 = bottom 20%) to all per-agent scores in the response.
// All per-agent rows are ranked together as one group. Non-agent rows are
// left untouched. Agents with equal scores that straddle a quintile boundary
// are moved to the higher (better) quintile.
//
// rankingScores is an optional map of agentUserID -> ranking score that accounts
// for lower-is-better criteria (via negation in the CH query). When provided,
// agents are sorted by ranking score instead of display score. This correctly
// handles mixed-directionality templates since the negation happens per-criterion
// before aggregation in ClickHouse.
//
// When ties cascade across multiple boundaries, some quintile groups may end
// up empty; agents are still assigned to the appropriate quintile.
func setQuintileRankForPerAgentScores(response *analyticspb.RetrieveQAScoreStatsResponse, rankingScores map[string]float64) {
	if response == nil || response.QaScoreResult == nil {
		return
	}

	// Collect all per-agent scores.
	var agentScores []*analyticspb.QAScore
	for _, s := range response.QaScoreResult.Scores {
		if s.GetGroupedBy().GetUser() != nil {
			agentScores = append(agentScores, s)
		}
	}

	n := len(agentScores)
	if n == 0 {
		return
	}

	// getRankingScore returns the ranking score for an agent if available,
	// otherwise falls back to the display score.
	getRankingScore := func(s *analyticspb.QAScore) float64 {
		if rankingScores != nil {
			if rs, ok := rankingScores[rankingScoreKey(s)]; ok {
				return rs
			}
		}
		return float64(s.Score)
	}

	// Sort descending by ranking score (higher = better).
	// When rankingScores is populated, lower-is-better criteria have already been
	// negated in the CH query, so descending sort is always correct.
	sort.SliceStable(agentScores, func(i, j int) bool {
		return getRankingScore(agentScores[i]) > getRankingScore(agentScores[j])
	})

	numQuintiles := len(quintileRanks)
	groupMap := utils.AssignRankGroups(n, numQuintiles, func(i int) float32 {
		return float32(getRankingScore(agentScores[i]))
	})
	for i, s := range agentScores {
		group := groupMap[i]
		if group < len(quintileRanks) {
			s.GroupedBy.QuintileRank = quintileRanks[group]
		}
	}
}

func (a AnalyticsServiceImpl) readQaScoreStatsRespectingTemplateQaScoreConfig(
	ctx context.Context,
	users []*userpb.User,
	req *analyticspb.RetrieveQAScoreStatsRequest,
	lowerIsBetterCriteria []string,
) (*qaScoreStatsResult, error) {
	profileName, err := customerpb.ParseProfileName(req.GetParent())
	if err != nil {
		return nil, err
	}

	scorecardTemplateIDs, err := qa.ExtractScorecardTemplateIDs(req.GetFilterByAttribute().GetScorecardTemplates())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "the scorecard template names are invalid: %v", err)
	}

	scorecardTemplateIDToTemplateMap, err := qa.ListCurrentScorecardTemplateIDs(
		a.appsDB.DB(ctx), profileName, req.GetFilterByAttribute().GetUsecaseNames(), scorecardTemplateIDs, false /* filterActive */)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list the scorecard template IDs: %v", err)
	}

	if len(req.GetFilterByAttribute().GetCriterionIdentifiers()) > 0 {
		scorecardTemplateIDToTemplateMap, err = qa.FilterScorecardTemplateByCriteria(
			scorecardTemplateIDToTemplateMap, req.GetFilterByAttribute().GetCriterionIdentifiers())
		if err != nil {
			return nil, err
		}
	}

	// Returns early if there is no scorecard templates found.
	if len(scorecardTemplateIDToTemplateMap) == 0 {
		a.logger.Warnf(ctx, "there is no scorecard template found: %v", req)
		return &qaScoreStatsResult{
			response: &analyticspb.RetrieveQAScoreStatsResponse{
				QaScoreResult: &analyticspb.QAScoreResult{},
			},
		}, nil
	}

	// Group templates by their conversation duration-bucket filter so each group is served by a single
	// `scorecard_template_id IN (...)` query. That filter is the only part of QaScoreConfig affecting
	// which rows match, so templates with identical filters are safe to query together; templates with
	// no filter (the common case, e.g. an empty config) collapse into one query, removing the
	// per-template serial fan-out that timed out for customers with many templates.
	//
	// Merged templates may share a criterion id (CONVI-7014), so results below are keyed by
	// (scorecard_template_id, criterion_id), not criterion_id alone, to keep them separate (CONVI-7068).
	templateGroups, err := groupTemplatesByConversationDurationFilter(scorecardTemplateIDToTemplateMap)
	if err != nil {
		return nil, err
	}

	perGroupReqs := make([]*analyticspb.RetrieveQAScoreStatsRequest, 0, len(templateGroups))
	for _, templateGroup := range templateGroups {
		scorecardTemplateNames, _ := fn.MapE(templateGroup.templateIDs, func(templateID string) (string, error) {
			return coachingpb.ScorecardTemplateName{
				CustomerID:                  profileName.CustomerID,
				ProfileID:                   profileName.ProfileID,
				ScorecardTemplateID:         templateID,
				ScorecardTemplateRevisionID: "*",
			}.String(), nil
		})

		newReq := proto.Clone(req).(*analyticspb.RetrieveQAScoreStatsRequest)
		if req.FilterByAttribute == nil {
			newReq.FilterByAttribute = &analyticspb.QAAttribute{}
		}
		newReq.FilterByAttribute.ScorecardTemplates = scorecardTemplateNames
		newReq.FilterByAttribute.ConversationDurationBuckets = templateGroup.durationBuckets
		perGroupReqs = append(perGroupReqs, newReq)
	}

	// Issue the per-group ClickHouse queries in parallel with bounded concurrency. Grouping above
	// already collapses templates that share a duration filter into a single query, but customers can
	// still have several distinct filter signatures. Each group's query is independent, so this turns
	// the total latency from the sum of all groups' latencies into roughly the latency of the slowest
	// group. Results are written to per-index slots to avoid shared-state mutation, then merged
	// sequentially below.
	perGroupResults := make([]*qaScoreStatsResult, len(perGroupReqs))
	eg, egCtx := errgroup.WithContext(ctx, errgroup.WithLimit(a.maxConcurrentQaScoreStatsQueries))
	for i, groupReq := range perGroupReqs {
		i, groupReq := i, groupReq
		eg.Go(func() error {
			chResult, err := a.readQaScoreStatsFromClickhouse(egCtx, groupReq, users, lowerIsBetterCriteria)
			if err != nil {
				return err
			}
			perGroupResults[i] = chResult
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	qaScores := []*analyticspb.QAScore{}
	var mergedRankingScores map[string]float64
	for _, chResult := range perGroupResults {
		qaScores = append(qaScores, chResult.response.GetQaScoreResult().GetScores()...)
		for k, v := range chResult.rankingScores {
			if mergedRankingScores == nil {
				mergedRankingScores = make(map[string]float64)
			}
			mergedRankingScores[k] = v
		}
	}

	return &qaScoreStatsResult{
		response: &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{
				Scores: qaScores,
			},
		},
		rankingScores: mergedRankingScores,
	}, nil
}

// qaScoreStatsTemplateGroup is a set of scorecard templates that share an identical conversation
// duration-bucket filter and can therefore be served by a single ClickHouse query.
type qaScoreStatsTemplateGroup struct {
	templateIDs     []string
	durationBuckets []*analyticspb.ConversationDurationBucket
}

// groupTemplatesByConversationDurationFilter groups scorecard templates by their conversation
// duration-bucket filter so that templates sharing the same filter are queried together. A template's
// QaScoreConfig only changes which conversations its query matches via this duration filter, so the
// mere presence of a (possibly empty) QaScoreConfig is not enough to require a separate query. In
// practice most templates carry no duration filter (an empty QaScoreConfig, or none at all) and
// therefore collapse into a single group. The returned groups, and the template IDs within each
// group, are sorted deterministically so that query construction is stable across calls.
func groupTemplatesByConversationDurationFilter(
	scorecardTemplateIDToTemplateMap map[string]*model.ScorecardTemplates,
) ([]qaScoreStatsTemplateGroup, error) {
	// Iterate templates in a deterministic order so groups and their members are stable.
	templateIDs := make([]string, 0, len(scorecardTemplateIDToTemplateMap))
	for templateID := range scorecardTemplateIDToTemplateMap {
		templateIDs = append(templateIDs, templateID)
	}
	sort.Strings(templateIDs)

	groupBySignature := make(map[string]*qaScoreStatsTemplateGroup)
	signatures := make([]string, 0)
	for _, templateID := range templateIDs {
		template := scorecardTemplateIDToTemplateMap[templateID]

		var durationBuckets []*analyticspb.ConversationDurationBucket
		if template.QaScoreConfig.Valid {
			protoQaScoreConfig, err := qa.ConvertDBToServiceQAScoreConfig(template)
			if err != nil {
				return nil, err
			}
			durationBuckets = protoQaScoreConfig.GetConversationFilters().GetConversationDurationBuckets()
		}

		signature := conversationDurationBucketsSignature(durationBuckets)
		group, exist := groupBySignature[signature]
		if !exist {
			group = &qaScoreStatsTemplateGroup{durationBuckets: durationBuckets}
			groupBySignature[signature] = group
			signatures = append(signatures, signature)
		}
		group.templateIDs = append(group.templateIDs, templateID)
	}

	sort.Strings(signatures)
	groups := make([]qaScoreStatsTemplateGroup, 0, len(signatures))
	for _, signature := range signatures {
		groups = append(groups, *groupBySignature[signature])
	}
	return groups, nil
}

// conversationDurationBucketsSignature returns a stable key identifying a conversation
// duration-bucket filter. The duration filter depends only on the bucket IDs (the actual bounds are
// looked up from a reference table downstream), so two filters with the same set of bucket IDs are
// equivalent. An empty result means "no duration filter".
func conversationDurationBucketsSignature(buckets []*analyticspb.ConversationDurationBucket) string {
	if len(buckets) == 0 {
		return ""
	}
	bucketIDs := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		bucketIDs = append(bucketIDs, bucket.GetId())
	}
	sort.Strings(bucketIDs)
	bucketIDs = slices.Compact(bucketIDs)
	return strings.Join(bucketIDs, ",")
}

func (a AnalyticsServiceImpl) getScoreableCriteria(
	ctx context.Context,
	profileName *customerpb.ProfileName,
	scorecardTemplateNames []string,
	usecaseNames []string,
) ([]string, error) {
	scorecardTemplateIDs, err := qa.ExtractScorecardTemplateIDs(scorecardTemplateNames)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "the scorecard template names are invalid: %v", err)
	}

	scorecardTemplateIDToTemplateMap, err := qa.ListCurrentScorecardTemplateIDs(
		a.appsDB.DB(ctx), profileName, usecaseNames, scorecardTemplateIDs, false /* filterActive */)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list the scorecard template IDs: %v", err)
	}

	scoreableCriteria := []string{}
	for _, template := range scorecardTemplateIDToTemplateMap {
		templateStructure, err := scoring.ParseScorecardTemplateStructure(template.Template)
		if err != nil {
			return nil, err
		}
		for _, criterion := range templateStructure.GetCriteriaSlice(false /*rootOnly*/) {
			if !criterion.IsExcludeFromQAScores() {
				scoreableCriteria = append(scoreableCriteria, criterion.GetIdentifier())
			}
		}
	}
	return scoreableCriteria, nil
}

func (a AnalyticsServiceImpl) applyScorecardStatusFilter(ctx context.Context, req *analyticspb.RetrieveQAScoreStatsRequest, parent *shared.CustomerProfile) error {
	if a.featureFlagClient == nil {
		return nil
	}
	enableScorecardPublish, err := a.featureFlagClient.BoolValue(ctx, "enableScorecardPublish", false)
	if err != nil {
		a.logger.Warnf(ctx, "feature flag enableScorecardPublish fallback used: %v", err)
	}

	if !enableScorecardPublish {
		return nil
	}

	authEntity, err := a.authEntityProvider.GetAuthEntity(ctx, parent.CustomerID)
	if err != nil {
		a.logger.Errorf(ctx, "Failed to get principal entity from auth token: %v", err)
		return status.Errorf(codes.Unauthenticated, "invalid user auth: %v", err)
	}

	if !authEntity.IsAgentOnly() {
		return nil
	}

	scorecardFilterStatuses := req.GetFilterByAttribute().GetScorecardStatuses()
	if len(scorecardFilterStatuses) == 0 {
		req.FilterByAttribute.ScorecardStatuses = allowedAgentOnlyScorecardStatuses
		return nil
	}

	for _, scorecardStatus := range scorecardFilterStatuses {
		if !slices.Contains(allowedAgentOnlyScorecardStatuses, scorecardStatus) {
			return status.Errorf(codes.PermissionDenied, "scorecard status %v is not allowed for agent-only users", scorecardStatus)
		}
	}

	return nil
}
