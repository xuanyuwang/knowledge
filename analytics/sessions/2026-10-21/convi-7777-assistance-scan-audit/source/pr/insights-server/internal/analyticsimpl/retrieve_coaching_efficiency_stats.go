package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	grpcRegistry "github.com/cresta/shared-go/framework/client/grpc/registry"
	"github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	userfilter "github.com/cresta/go-servers/shared/user-filter"
	"github.com/cresta/go-servers/shared/utils"
)

var (
	NumberCoachingEfficiencyConcurrentQuery  = envflag.Int("NUMBER_COACHING_EFFICIENCY_CONCURRENT_QUERY", 15, "the number of the concurrent queries for coaching efficiency")
	NumberDaysCoachingEfficiencyQAScoreQuery = envflag.Int("NUMBER_DAYS_COACHING_EFFICIENCY_QA_SCORE_QUERY", 7, "the number of days to query the qa scores for coaching efficiency")
	ThresholdForAutoQACriterion              = envflag.Int("ANALYTICS_SERVICE_AUTO_CRITERION_CONV_COUNT_THRESHOLD", 5, "the conv count threshold to include a auto criterion into efficiency calculation")
	ThresholdForManualQACriterion            = envflag.Int("ANALYTICS_SERVICE_MANUAL_CRITERION_CONV_COUNT_THRESHOLD", 1, "the conv count threshold to include a manual criterion into efficiency calculation")
	EnabledRtrieveCoachingEfficiencyStatsLog = envflag.Bool("ANALYTICS_SERVICE_ENABLE_RETRIEVE_COACHING_EFFICIENCY_STATS_LOG", false, "enable debug log for RetrieveCoachingEfficiencyStats")
)

const formattedSessionTimeQuery string = "DATE_TRUNC(" +
	"@frequency, " +
	"session_at AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias) AT TIME ZONE @time_zone " +
	"- INTERVAL '1 day' * @first_day_of_week_bias AS frequency, "

type qaScoreStatsGetter interface {
	RetrieveQAScoreStats(ctx context.Context, in *analyticspb.RetrieveQAScoreStatsRequest) (*analyticspb.RetrieveQAScoreStatsResponse, error)
}

type userOutcomeStatsGetter interface {
	RetrieveUserOutcomeStats(ctx context.Context, in *analyticspb.RetrieveUserOutcomeStatsRequest) (*analyticspb.RetrieveUserOutcomeStatsResponse, error)
}

type efficiencyData struct {
	efficiency       float64
	efficiencyWeight int32
	sessionIDs       map[string]bool
	latestSessionAt  time.Time
	lastSessionID    string
}

func (d *efficiencyData) String() string {
	return fmt.Sprintf("efficiency: %v, efficiencyWeight: %v, latestSessionAt: %v, lastSessionID: %v, sessionIDs: %v",
		d.efficiency, d.efficiencyWeight, d.latestSessionAt, d.lastSessionID, d.sessionIDs)
}

type criterionKey struct {
	ScorecardTemplateID string
	CriterionID         string
	AgentUserID         string
}

type sessionGroupKey struct {
	AgentUserID                string
	CoachingSessionID          string
	Frequency                  sql.NullTime
	CoachingSessionSubmitterID string
	GroupID                    string
	CriterionID                string
}

// Necessary cocahing session data used in RetrieveCoachingEfficiencyStats
//
//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
type coachingSessionCriterion struct {
	AgentUserID                string         `gorm:"column:user_id"`
	CoachingSessionID          string         `gorm:"column:coaching_session_id"`
	GroupSessionID             sql.NullString `gorm:"column:group_session_id"`
	CoachingSessionAt          time.Time      `gorm:"column:coaching_session_at"`
	ScorecardTemplateID        string         `gorm:"column:scorecard_template_id"`
	CriterionID                string         `gorm:"column:criterion_id"`
	CoachingSessionSubmitterID string         `gorm:"column:manager_user_id"`
	BeforeSessionScore         float64
	BeforeSessionConvCount     int32
	AfterSessionScore          float64
	AfterSessionConvCount      int32
	IsAutoCriterion            bool
	IsEvaluable                bool // if this criteria should be included the efficiency calculation
	Frequency                  sql.NullTime

	IsUserOutcome              bool
	OutcomeFieldDefinitionName string
	OutcomeAggregationMode     analyticspb.AggregationMode
}

func (c *coachingSessionCriterion) String() string {
	return fmt.Sprintf("agent user ID: %v, session ID: %v, session at: %v, template ID: %v, criterion ID: %v, submitter ID: %v, before score: %v, before conv count: %v, after score: %v, after conv count: %v, is auto: %v",
		c.AgentUserID, c.CoachingSessionID, c.CoachingSessionAt, c.ScorecardTemplateID, c.CriterionID,
		c.CoachingSessionSubmitterID, c.BeforeSessionScore, c.BeforeSessionConvCount,
		c.AfterSessionScore, c.AfterSessionConvCount, c.IsAutoCriterion)
}

// dedupBySession is the default session-count key: every coaching session counts on its own.
func dedupBySession(c *coachingSessionCriterion) string {
	return c.CoachingSessionID
}

// coachingSessionTypeCondition returns a GORM scope filtering coaching sessions by 1:1 vs
// group coaching, or nil for ALL/UNSPECIFIED (no filtering). groupSessionIDColumn is the
// qualified group_session_id column ("cs.group_session_id" for the efficiency query,
// "group_session_id" for session stats). Group = non-empty group_session_id; 1:1 = NULL or
// empty -- matching the dedup semantics (Valid && != "").
func coachingSessionTypeCondition(
	filter analyticspb.CoachingSessionFilterType, groupSessionIDColumn string,
) func(*gorm.DB) *gorm.DB {
	switch filter {
	case analyticspb.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_ONE_ON_ONE:
		return func(d *gorm.DB) *gorm.DB {
			return d.Where("COALESCE(" + groupSessionIDColumn + ", '') = ''")
		}
	case analyticspb.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_GROUP:
		return func(d *gorm.DB) *gorm.DB {
			return d.Where("COALESCE(" + groupSessionIDColumn + ", '') != ''")
		}
	default:
		return nil // ALL / UNSPECIFIED
	}
}

// dedupByGroupSession collapses a group (bulk) coaching event to a single count while still
// counting ungrouped 1:1 sessions individually. Used by the Overview total and Session Coach
// surfaces ("sessions held" model). The prefixes prevent a group id from colliding with a
// resource id.
func dedupByGroupSession(c *coachingSessionCriterion) string {
	if c.GroupSessionID.Valid && c.GroupSessionID.String != "" {
		return "group:" + c.GroupSessionID.String
	}
	return "session:" + c.CoachingSessionID
}

func (c *coachingSessionCriterion) GetConvCountThreshold() int32 {
	if c.IsAutoCriterion {
		return int32(*ThresholdForAutoQACriterion)
	} else {
		return int32(*ThresholdForManualQACriterion)
	}
}

func matchesCriterionCategoryFilter(
	criterionCategory analyticspb.Attribute_CriterionCategory,
	requestedCategories []analyticspb.Attribute_CriterionCategory,
) bool {
	if len(requestedCategories) == 0 {
		return true
	}

	for _, requestedCategory := range requestedCategories {
		if requestedCategory == analyticspb.Attribute_CRITERION_CATEGORY_UNSPECIFIED {
			return true
		}
		if criterionCategory == requestedCategory {
			return true
		}
	}
	return false
}

// All filters used in retrieveCoachingEfficiencyStatsFilter
type retrieveCoachingEfficiencyStatsFilter struct {
	UsersAndGroups                      *userfilter.FilteredUsersAndGroups
	ScorecardTemplateNames              []string
	ScorecardTemplateIDs                []string
	CriterionIDs                        []string
	CoachingSessionIDs                  []string
	UsecaseNames                        []string
	UsecaseIDs                          []string
	CriterionCategories                 []analyticspb.Attribute_CriterionCategory
	IsIncludeDirectGroupMembershipsOnly bool
}

func (a AnalyticsServiceImpl) RetrieveCoachingEfficiencyStats(
	ctx context.Context, req *analyticspb.RetrieveCoachingEfficiencyStatsRequest,
) (*analyticspb.RetrieveCoachingEfficiencyStatsResponse, error) {
	RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "request: %v", req)

	err := validateRetrieveCoachingEfficiencyStatsRequest(req)
	if err != nil {
		return nil, err
	}

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", req.Parent)
	}

	// pasrse request filters
	filter, err := parseRetrieveCoachingEfficiencyStatsFilter(
		ctx, a.logger, parent, a.configClient, a.resourceACLHelperProvider.Get(), a.userClientRegistry,
		req.FilterByAttribute, &userfilter.UserFilterParserImpl{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse FilterByAttribute: %v", err)
	}
	RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "filter: %+v", filter.UsersAndGroups.String())
	if len(filter.UsersAndGroups.UserNames) == 0 {
		return &analyticspb.RetrieveCoachingEfficiencyStatsResponse{}, err
	}

	return a.retrieveCoachingEfficiencyStatsInternal(ctx, a, req, filter, parent)
}

func RetrieveCoachingEfficiencyStatsInfof(ctx context.Context, logger log.Logger, msg string, args ...interface{}) {
	if *EnabledRtrieveCoachingEfficiencyStatsLog {
		msg = fmt.Sprintf("RetrieveCoachingEfficiencyStats: %s", msg)
		logger.Infof(ctx, msg, args...)
	}
}

func validateRetrieveCoachingEfficiencyStatsRequest(req *analyticspb.RetrieveCoachingEfficiencyStatsRequest) error {
	if req.FilterByTimeRange == nil && len(req.GetFilterByAttribute().GetCoachingSessionNames()) == 0 {
		return status.Error(codes.InvalidArgument, "a time range is required when no coaching session names are provided")
	}
	if len(req.GetFilterByAttribute().GetUsecaseNames()) == 0 {
		return status.Error(codes.InvalidArgument, "usecase is required")
	}
	return nil
}

func (a AnalyticsServiceImpl) retrieveCoachingEfficiencyStatsInternal(
	ctx context.Context,
	qaScoreGetter qaScoreStatsGetter,
	req *analyticspb.RetrieveCoachingEfficiencyStatsRequest,
	filter *retrieveCoachingEfficiencyStatsFilter,
	parent *shared.CustomerProfile,
) (*analyticspb.RetrieveCoachingEfficiencyStatsResponse, error) {
	hasGroupbyTimeRange := shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE)
	hasGroupByCriterion := shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_CRITERION)
	sessions, err := a.readCoachingSessionCriterion(ctx, parent, filter, req.FilterByTimeRange, &req.Frequency, req.Metadata, hasGroupbyTimeRange, req.CoachingSessionFilter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query coaching sessions: %v", err)
	}

	if len(sessions) == 0 {
		return &analyticspb.RetrieveCoachingEfficiencyStatsResponse{}, nil
	}

	err = fillBeforeAndAfterQAScores(ctx, a.logger, parent, qaScoreGetter, &a, filter, sessions)
	if err != nil {
		return nil, fmt.Errorf("failed to query before and after qa scores: %w", err)
	}
	for _, session := range sessions {
		RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "parent: %v, session: %v", session.String())
	}

	switch {
	case shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION):
		response, err := a.getCoachingEfficiencyGroupBySessions(ctx, parent, sessions, hasGroupbyTimeRange, hasGroupByCriterion)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed when calculating coaching efficiency with grouping by sessions: %v", err)
		}
		return response, nil
	case shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION_SUBMITTER):
		response, err := a.getCoachingEfficiencyGroupByCoaches(ctx, parent, sessions, hasGroupbyTimeRange)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed when calculating coaching efficiency with grouping by coaches: %v", err)
		}
		return response, nil
	case shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP):
		response, err := a.getCoachingEfficiencyGroupByGroups(ctx, a.logger, parent, sessions, filter, hasGroupbyTimeRange)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed when calculating coaching efficiency with grouping by groups: %v", err)
		}
		return response, nil
	case shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT):
		response, err := a.getCoachingEfficiencyGroupByAgents(ctx, parent, sessions, hasGroupbyTimeRange)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed when calculating coaching efficiency with grouping by agents: %v", err)
		}
		return response, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid group by %v", req.GroupByAttributeTypes)
	}
}

func (a AnalyticsServiceImpl) readCoachingSessionCriterion(
	ctx context.Context,
	parent *shared.CustomerProfile,
	filter *retrieveCoachingEfficiencyStatsFilter,
	timeRange *analyticspb.TimeRange,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	hasGroupbyTimeRange bool,
	coachingSessionFilter analyticspb.CoachingSessionFilterType,
) ([]*coachingSessionCriterion, error) {
	var frequencyArgs []interface{}
	var err error
	if hasGroupbyTimeRange {
		frequencyArgs, err = postgres.ParseArgs(frequency, metadata)
		if err != nil {
			return nil, err
		}
	}
	db := a.appsDB.DB(ctx)

	conditions := make([]func(*gorm.DB) *gorm.DB, 0)
	conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
		return db.Where("cs.customer = ? AND cs.profile = ?", parent.CustomerID, parent.ProfileID)
	})
	conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
		return d.Where("cs.manager_submitted_at IS NOT NULL")
	})
	conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
		return d.Where("cs.deleted_at IS NULL")
	})
	conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
		return d.Where("cs.usecase_id IN (?)", filter.UsecaseIDs)
	})
	if timeRange != nil {
		conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
			return d.Where("session_at >= ? AND session_at <= ?", timeRange.StartTimestamp.AsTime(), timeRange.EndTimestamp.AsTime())
		})
	}
	if len(filter.UsersAndGroups.UserNames) > 0 {
		userIDs, err := fn.MapE(filter.UsersAndGroups.UserNames, userfilter.ConvertUserNameToID)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Where("cs.agent_user_id IN (?)", userIDs)
		})
	}

	if len(filter.ScorecardTemplateIDs) > 0 {
		conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
			// When template filter is set, only include sessions with focus_criteria_ids matching the template
			return d.Where(
				"split_part(focus_criteria_template_criterion_id, '/', 1) IN (?)",
				filter.ScorecardTemplateIDs,
			)
		})
	}
	if len(filter.CriterionIDs) > 0 {
		conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
			// When criterion filter is set, only include sessions with focus_criteria_ids matching the criterion
			return d.Where(
				"split_part(focus_criteria_template_criterion_id, '/', 2) IN (?)",
				filter.CriterionIDs,
			)
		})
	}
	if len(filter.CoachingSessionIDs) > 0 {
		conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
			return d.Where("cs.resource_id IN (?)", filter.CoachingSessionIDs)
		})
	}

	// Auto-compute whether to include sessions without focus criteria:
	// Include them when NO template/criterion/session filters are present
	includeSessionsWithoutFocusCriteria := len(filter.ScorecardTemplateIDs) == 0 &&
		len(filter.CriterionIDs) == 0 &&
		len(filter.CoachingSessionIDs) == 0

	// When filters are present, exclude sessions without focus criteria
	if !includeSessionsWithoutFocusCriteria {
		conditions = append(conditions, func(d *gorm.DB) *gorm.DB {
			return d.Where("focus_criteria_template_criterion_id IS NOT NULL")
		})
	}

	// Filter by coaching session type (1:1 vs group), no-op for ALL/UNSPECIFIED.
	if sessionTypeCondition := coachingSessionTypeCondition(coachingSessionFilter, "cs.group_session_id"); sessionTypeCondition != nil {
		conditions = append(conditions, sessionTypeCondition)
	}

	// get sessions
	var sessionCriteria []*coachingSessionCriterion
	querySessionCriteria := func(db *gorm.DB) *gorm.DB {
		baseQuery := db.Table("director.coaching_sessions AS cs").
			Joins(`
				LEFT JOIN LATERAL UNNEST(cs.focus_criteria_ids) AS focus_criteria_template_criterion_id ON true
			`).
			Scopes(conditions...)

		baseSelect := `
			cs.resource_id AS coaching_session_id,
			cs.group_session_id AS group_session_id,
			cs.agent_user_id AS user_id,
			cs.session_at AS coaching_session_at,
			COALESCE(cs.manager_submitter_user_id, cs.creator_user_id) AS manager_user_id,
			split_part(focus_criteria_template_criterion_id, '/', 1) AS scorecard_template_id,
			split_part(focus_criteria_template_criterion_id, '/', 2) AS criterion_id`

		var query *gorm.DB
		if hasGroupbyTimeRange {
			query = baseQuery.Select(formattedSessionTimeQuery+baseSelect, frequencyArgs...)
		} else {
			query = baseQuery.Select(baseSelect)
		}
		return query.Order("coaching_session_id, criterion_id, user_id, manager_submitter_user_id").
			Find(&sessionCriteria)
	}
	err = querySessionCriteria(db).Error
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return querySessionCriteria(tx) })
	RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "query of finding criteria: %s", sql)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error when query session criteria: %v", err)
	}

	templateIDs := set.NewFromSlice(fn.Map(sessionCriteria, func(c *coachingSessionCriterion) string {
		return c.ScorecardTemplateID
	}))
	scorecardTemplateIDToTemplateMap, err := qa.ListCurrentScorecardTemplateIDs(db,
		&customerpb.ProfileName{
			CustomerID: parent.CustomerID,
			ProfileID:  parent.ProfileID,
		}, filter.UsecaseNames, templateIDs.Slice(), false)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error when query criteria: %v", err)
	}
	criterionToAutoQA := map[criterionKey]bool{}
	criterionToEvaluate := map[criterionKey]bool{}
	criterionToCategory := map[criterionKey]analyticspb.Attribute_CriterionCategory{}
	criterionToUserOutcome := map[criterionKey]scoring.UserOutcomeCriterionInfo{}
	for _, template := range scorecardTemplateIDToTemplateMap {
		templateStructure, err := scoring.ParseScorecardTemplateStructure(template.Template)
		if err != nil {
			return nil, err
		}
		for _, criterion := range templateStructure.GetCriteriaSlice(false /*rootOnly*/) {
			key := criterionKey{
				ScorecardTemplateID: template.ResourceID,
				CriterionID:         criterion.GetIdentifier(),
			}
			criterionToAutoQA[key] = criterion.GetAutoQA() != nil && len(*criterion.GetAutoQA().Triggers) > 0
			criterionToEvaluate[key] = !criterion.IsExcludeFromQAScores()
			criterionToCategory[key] = scoring.CriterionToCriterionCategory(criterion)
		}
		// ALO criteria carry the outcome field + aggregation used to fetch before/after values
		for criterionID, info := range scoring.ExtractCriterionUserOutcomeInfo(template.Template) {
			criterionToUserOutcome[criterionKey{ScorecardTemplateID: template.ResourceID, CriterionID: criterionID}] = info
		}
	}
	filteredSessions := make([]*coachingSessionCriterion, 0, len(sessionCriteria))
	for _, criteria := range sessionCriteria {
		key := criterionKey{
			ScorecardTemplateID: criteria.ScorecardTemplateID,
			CriterionID:         criteria.CriterionID,
		}
		isAutoCriterion, exist := criterionToAutoQA[key]
		if exist {
			criteria.IsAutoCriterion = isAutoCriterion
		}

		evaluable, exist := criterionToEvaluate[key]
		if exist && evaluable {
			criteria.IsEvaluable = true
		}

		if userOutcomeInfo, exist := criterionToUserOutcome[key]; exist {
			criteria.IsUserOutcome = true
			criteria.OutcomeFieldDefinitionName = userOutcomeInfo.FieldDefinitionName
			criteria.OutcomeAggregationMode = userOutcomeInfo.AggregationType.AnalyticsMode()
		}

		if len(filter.CriterionCategories) > 0 {
			// Only apply category filter to sessions with focus criteria
			// Sessions without focus criteria should be included in session count but not in effectiveness calculation
			if criteria.CriterionID != "" && criteria.ScorecardTemplateID != "" {
				criterionCategory, exists := criterionToCategory[key]
				if !exists {
					continue
				}
				if !matchesCriterionCategoryFilter(criterionCategory, filter.CriterionCategories) {
					continue
				}
				if !criteria.IsEvaluable && criterionCategory == analyticspb.Attribute_CRITERION_CATEGORY_PERFORMANCE {
					continue
				}
			}
			// Sessions with empty criterion IDs pass through to be counted in total sessions
		}

		filteredSessions = append(filteredSessions, criteria)
	}

	RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "read %d coaching sessions from db: %+v", len(sessionCriteria), sessionCriteria)
	return filteredSessions, nil
}

func (a AnalyticsServiceImpl) getCoachingEfficiencyGroupByGroups(
	ctx context.Context,
	logger log.Logger,
	parent *shared.CustomerProfile,
	sessions []*coachingSessionCriterion,
	filter *retrieveCoachingEfficiencyStatsFilter,
	hasGroupbyTimeRange bool,
) (*analyticspb.RetrieveCoachingEfficiencyStatsResponse, error) {
	var userGroupNames map[string][]string
	if filter.IsIncludeDirectGroupMembershipsOnly {
		userGroupNames = filter.UsersAndGroups.UserNameToDirectGroupNames
	} else {
		userGroupNames = filter.UsersAndGroups.UserNameToAllGroupNames
	}
	// convert user->groupNames to user->groupIDs
	userGroupIDs := map[string][]string{}
	for userName, groupNames := range userGroupNames {
		userGroupIDs[userName] = []string{}
		for _, groupName := range groupNames {
			groupID, err := userpb.ParseGroupName(groupName)
			if err != nil {
				return nil, fmt.Errorf("can't parse group name during group coaching efficiency by group: %v", err)
			}
			userGroupIDs[userName] = append(userGroupIDs[userName], groupID.GroupID)
		}
	}
	if *EnabledRtrieveCoachingEfficiencyStatsLog {
		logger.Infof(ctx, "RetrieveCoachingEfficiencyStats users by groups: %+v", userGroupIDs)
	}

	totalEfficiencyStats := a.computeTotalCoachingEfficiencyStats(ctx, parent, sessions)
	groupEfficiencyData := a.computeGroupCoachingEfficiencyStats(ctx, sessions, func(c *coachingSessionCriterion) []sessionGroupKey {
		userName := userpb.UserName{
			CustomerID: parent.CustomerID,
			UserID:     c.AgentUserID,
		}

		var groupKeys []sessionGroupKey
		for _, groupID := range userGroupIDs[userName.String()] {
			if hasGroupbyTimeRange {
				groupKeys = append(groupKeys, sessionGroupKey{
					GroupID: groupID,
					Frequency: sql.NullTime{
						Time:  c.Frequency.Time,
						Valid: true,
					},
				})
			} else {
				groupKeys = append(groupKeys, sessionGroupKey{
					GroupID: groupID,
				})
			}
		}
		return groupKeys
	}, dedupBySession)

	response := &analyticspb.RetrieveCoachingEfficiencyStatsResponse{
		TotalCoachingEfficiency: totalEfficiencyStats,
		CoachingEfficiencyStats: make([]*analyticspb.CoachingEfficiencyStats, 0, len(groupEfficiencyData)),
	}

	for key, efficiencyData := range groupEfficiencyData {
		groupID := key.GroupID

		efficiencyStats := convertEfficiencyDataToStats(parent, efficiencyData)
		efficiencyStats.Attribute = &analyticspb.Attribute{
			Groups: []*userpb.Group{{Name: userpb.GroupName{CustomerID: parent.CustomerID, GroupID: groupID}.String()}},
		}
		if key.Frequency.Valid {
			efficiencyStats.Attribute.TimeRanges = []*analyticspb.TimeRange{{
				StartTimestamp: timestamppb.New(key.Frequency.Time),
			}}
		}
		response.CoachingEfficiencyStats = append(response.CoachingEfficiencyStats, efficiencyStats)
	}

	return response, nil
}

func (a AnalyticsServiceImpl) getCoachingEfficiencyGroupByAgents(
	ctx context.Context,
	parent *shared.CustomerProfile,
	sessions []*coachingSessionCriterion,
	hasGroupbyTimeRange bool,
) (*analyticspb.RetrieveCoachingEfficiencyStatsResponse, error) {
	totalEfficiencyStats := a.computeTotalCoachingEfficiencyStats(ctx, parent, sessions)
	agentEfficiencyData := a.computeGroupCoachingEfficiencyStats(ctx, sessions, func(c *coachingSessionCriterion) []sessionGroupKey {
		if hasGroupbyTimeRange {
			return []sessionGroupKey{
				{AgentUserID: c.AgentUserID, Frequency: sql.NullTime{Time: c.Frequency.Time, Valid: true}},
			}
		}
		return []sessionGroupKey{
			{AgentUserID: c.AgentUserID},
		}
	}, dedupBySession)

	response := &analyticspb.RetrieveCoachingEfficiencyStatsResponse{
		TotalCoachingEfficiency: totalEfficiencyStats,
		CoachingEfficiencyStats: make([]*analyticspb.CoachingEfficiencyStats, 0, len(agentEfficiencyData)),
	}

	for key, efficiencyData := range agentEfficiencyData {
		efficiencyStats := convertEfficiencyDataToStats(parent, efficiencyData)
		efficiencyStats.Attribute = &analyticspb.Attribute{
			Users: []*userpb.User{{Name: userpb.UserName{CustomerID: parent.CustomerID, UserID: key.AgentUserID}.String()}},
		}
		if key.Frequency.Valid {
			efficiencyStats.Attribute.TimeRanges = []*analyticspb.TimeRange{{
				StartTimestamp: timestamppb.New(key.Frequency.Time),
			}}
		}
		response.CoachingEfficiencyStats = append(response.CoachingEfficiencyStats, efficiencyStats)
	}

	return response, nil
}

func (a AnalyticsServiceImpl) getCoachingEfficiencyGroupByCoaches(
	ctx context.Context,
	parent *shared.CustomerProfile,
	sessions []*coachingSessionCriterion,
	hasGroupbyTimeRange bool,
) (*analyticspb.RetrieveCoachingEfficiencyStatsResponse, error) {
	totalEfficiencyStats := a.computeTotalCoachingEfficiencyStats(ctx, parent, sessions)
	coachEfficiencyData := a.computeGroupCoachingEfficiencyStats(ctx, sessions, func(c *coachingSessionCriterion) []sessionGroupKey {
		if hasGroupbyTimeRange {
			return []sessionGroupKey{
				{CoachingSessionSubmitterID: c.CoachingSessionSubmitterID, Frequency: sql.NullTime{Time: c.Frequency.Time, Valid: true}},
			}
		}
		return []sessionGroupKey{
			{CoachingSessionSubmitterID: c.CoachingSessionSubmitterID},
		}
	}, dedupByGroupSession)

	response := &analyticspb.RetrieveCoachingEfficiencyStatsResponse{
		TotalCoachingEfficiency: totalEfficiencyStats,
		CoachingEfficiencyStats: make([]*analyticspb.CoachingEfficiencyStats, 0, len(coachEfficiencyData)),
	}

	for key, efficiencyData := range coachEfficiencyData {
		efficiencyStats := convertEfficiencyDataToStats(parent, efficiencyData)
		efficiencyStats.Attribute = &analyticspb.Attribute{
			Users: []*userpb.User{{
				Name: userpb.UserName{
					CustomerID: parent.CustomerID,
					UserID:     key.CoachingSessionSubmitterID,
				}.String(),
			}},
		}
		if key.Frequency.Valid {
			efficiencyStats.Attribute.TimeRanges = []*analyticspb.TimeRange{{
				StartTimestamp: timestamppb.New(key.Frequency.Time),
			}}
		}
		response.CoachingEfficiencyStats = append(response.CoachingEfficiencyStats, efficiencyStats)
	}
	return response, nil
}

func (a AnalyticsServiceImpl) getCoachingEfficiencyGroupBySessions(
	ctx context.Context,
	parent *shared.CustomerProfile,
	sessions []*coachingSessionCriterion,
	hasGroupbyTimeRange bool,
	hasGroupByCriterion bool,
) (*analyticspb.RetrieveCoachingEfficiencyStatsResponse, error) {
	totalEfficiencyStats := a.computeTotalCoachingEfficiencyStats(ctx, parent, sessions)
	sessionEfficiencyData := a.computeGroupCoachingEfficiencyStats(ctx, sessions, func(c *coachingSessionCriterion) []sessionGroupKey {
		groupKey := sessionGroupKey{
			CoachingSessionID: c.CoachingSessionID,
		}
		if hasGroupbyTimeRange {
			groupKey.Frequency = sql.NullTime{Time: c.Frequency.Time, Valid: true}
		}
		return []sessionGroupKey{groupKey}
	}, dedupBySession)
	if hasGroupByCriterion {
		sessionCriterionEfficiencyData := a.computeGroupCoachingEfficiencyStats(ctx, sessions, func(c *coachingSessionCriterion) []sessionGroupKey {
			return []sessionGroupKey{
				{CoachingSessionID: c.CoachingSessionID, CriterionID: c.CriterionID},
			}
		}, dedupBySession)
		for key, efficiencyData := range sessionCriterionEfficiencyData {
			sessionEfficiencyData[key] = efficiencyData
		}
	}

	response := &analyticspb.RetrieveCoachingEfficiencyStatsResponse{
		TotalCoachingEfficiency: totalEfficiencyStats,
		CoachingEfficiencyStats: make([]*analyticspb.CoachingEfficiencyStats, 0, len(sessionEfficiencyData)),
	}

	for key, efficiencyData := range sessionEfficiencyData {
		efficiencyStats := convertEfficiencyDataToStats(parent, efficiencyData)
		efficiencyStats.Attribute = &analyticspb.Attribute{
			CoachingSessionNames: []string{coachingpb.CoachingSessionName{
				CustomerID:        parent.CustomerID,
				ProfileID:         parent.ProfileID,
				CoachingSessionID: key.CoachingSessionID,
			}.String()},
		}

		if key.Frequency.Valid {
			efficiencyStats.Attribute.TimeRanges = []*analyticspb.TimeRange{{
				StartTimestamp: timestamppb.New(key.Frequency.Time),
			}}
		}
		if key.CriterionID != "" {
			efficiencyStats.Attribute.CriterionIdentifiers = []string{key.CriterionID}
		}
		response.CoachingEfficiencyStats = append(response.CoachingEfficiencyStats, efficiencyStats)
	}
	return response, nil
}

func (a AnalyticsServiceImpl) computeTotalCoachingEfficiencyStats(
	ctx context.Context,
	parent *shared.CustomerProfile,
	sessions []*coachingSessionCriterion,
) *analyticspb.CoachingEfficiencyStats {
	totalEfficiencyData := a.computeGroupCoachingEfficiencyStats(ctx, sessions, func(c *coachingSessionCriterion) []sessionGroupKey {
		return make([]sessionGroupKey, 1)
	}, dedupByGroupSession)
	return convertEfficiencyDataToStats(parent, totalEfficiencyData[sessionGroupKey{}])
}

func (a AnalyticsServiceImpl) computeGroupCoachingEfficiencyStats(
	ctx context.Context,
	sessions []*coachingSessionCriterion,
	groupBy func(c *coachingSessionCriterion) []sessionGroupKey,
	dedupKey func(c *coachingSessionCriterion) string,
) map[sessionGroupKey]*efficiencyData {
	groupEfficiencyData := make(map[sessionGroupKey]*efficiencyData)
	for _, c := range sessions {
		groupKeys := groupBy(c)

		for _, groupKey := range groupKeys {
			if _, exist := groupEfficiencyData[groupKey]; !exist {
				groupEfficiencyData[groupKey] = &efficiencyData{
					sessionIDs: map[string]bool{},
				}
			}
		}

		switch {
		case c.IsUserOutcome:
			if c.BeforeSessionConvCount > 0 && c.AfterSessionConvCount > 0 {
				delta := a.computeSessionEfficiency(ctx, c)
				for _, groupKey := range groupKeys {
					groupEfficiencyData[groupKey].efficiency += delta
					groupEfficiencyData[groupKey].efficiencyWeight++
				}
			}
		case c.BeforeSessionConvCount > 0 && c.AfterSessionConvCount >= c.GetConvCountThreshold():
			efficiency := a.computeSessionEfficiency(ctx, c)
			for _, groupKey := range groupKeys {
				groupEfficiencyData[groupKey].efficiency += efficiency
				groupEfficiencyData[groupKey].efficiencyWeight += c.AfterSessionConvCount
			}
		}

		for _, groupKey := range groupKeys {
			data := groupEfficiencyData[groupKey]
			data.sessionIDs[dedupKey(c)] = true
			if data.latestSessionAt.Before(c.CoachingSessionAt) {
				data.latestSessionAt = c.CoachingSessionAt
				data.lastSessionID = c.CoachingSessionID
			}
		}
	}

	for groupKey, data := range groupEfficiencyData {
		data.efficiency = safeDivideFloat64(data.efficiency, float64(data.efficiencyWeight))
		RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "group %v data: %v", groupKey, data.String())
	}

	return groupEfficiencyData
}

func convertEfficiencyDataToStats(parent *shared.CustomerProfile, data *efficiencyData) *analyticspb.CoachingEfficiencyStats {
	if data == nil {
		return nil
	}
	return &analyticspb.CoachingEfficiencyStats{
		IsEfficiencyScoreInvalid: data.efficiencyWeight == 0,
		CoachingEfficiency:       float32(data.efficiency),
		TotalNumberOfSessions:    int32(len(data.sessionIDs)),
		LastSessionDate:          timestamppb.New(data.latestSessionAt),
		LastSessionName: coachingpb.CoachingSessionName{
			CustomerID:        parent.CustomerID,
			ProfileID:         parent.ProfileID,
			CoachingSessionID: data.lastSessionID,
		}.String(),
	}
}

func retrieveQAScoreStatsForSameDayAgentCriterion(
	ctx context.Context,
	logger log.Logger,
	analyticsService qaScoreStatsGetter,
	parent *shared.CustomerProfile,
	coachingSessions []*coachingSessionCriterion,
	filter *retrieveCoachingEfficiencyStatsFilter,
	sessionDate time.Time,
	isBeforeScore bool,
) error {
	agentUserIDsMap := map[string]bool{}
	criterionIDsMap := map[string]bool{}
	for _, c := range coachingSessions {
		agentUserIDsMap[c.AgentUserID] = true
		criterionIDsMap[c.CriterionID] = true
	}
	agentUserIDs := fn.MapKeyToSlice(agentUserIDsMap)
	criterionIDs := fn.MapKeyToSlice(criterionIDsMap)
	sort.Strings(agentUserIDs)
	sort.Strings(criterionIDs)
	users := fn.Map(agentUserIDs, func(s string) *userpb.User {
		return &userpb.User{
			Name: userpb.UserName{
				CustomerID: parent.CustomerID,
				UserID:     s,
			}.String(),
		}
	})

	// Key by (scorecard_template_id, criterion_id): criterion ids aren't unique across
	// templates (CONVI-7014), and the QA score response is grouped the same way.
	type agentCriterion struct {
		AgentUserID         string
		ScorecardTemplateID string
		CriterionID         string
	}

	coachingSessionsMap := map[agentCriterion]*coachingSessionCriterion{}
	for _, c := range coachingSessions {
		key := agentCriterion{
			AgentUserID:         c.AgentUserID,
			ScorecardTemplateID: c.ScorecardTemplateID,
			CriterionID:         c.CriterionID,
		}
		coachingSessionsMap[key] = c
	}

	timeRange := beforeAfterTimeRange(sessionDate, isBeforeScore)

	request := &analyticspb.RetrieveQAScoreStatsRequest{
		Parent:            parent.String(),
		FilterByTimeRange: timeRange,
		FilterByAttribute: &analyticspb.QAAttribute{
			Users:                users,
			CriterionIdentifiers: criterionIDs,
			UsecaseNames:         filter.UsecaseNames,
			ScorecardTemplates:   filter.ScorecardTemplateNames,
		},
		GroupByAttributeTypes: []analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
		RespectTemplateQaScoreConfig: true,
	}
	RetrieveCoachingEfficiencyStatsInfof(ctx, logger, "parent: %v, qa score request timerange: %v, %v", parent.String(), timeRange.StartTimestamp.AsTime(), timeRange.EndTimestamp.AsTime())

	response, err := analyticsService.RetrieveQAScoreStats(ctx, request)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to query qa score -- request: %v; error: %v", request, err)
	}

	scoreMap := map[agentCriterion]*analyticspb.QAScore{}
	for _, score := range response.QaScoreResult.Scores {
		agentUserName, err := userpb.ParseUserName(score.GroupedBy.User.Name)
		if err != nil {
			return err
		}
		key := agentCriterion{
			AgentUserID:         agentUserName.UserID,
			ScorecardTemplateID: score.GroupedBy.ScorecardTemplateId,
			CriterionID:         score.GroupedBy.CriterionId,
		}
		scoreMap[key] = score
	}
	for _, session := range coachingSessions {
		score, exist := scoreMap[agentCriterion{
			AgentUserID:         session.AgentUserID,
			ScorecardTemplateID: session.ScorecardTemplateID,
			CriterionID:         session.CriterionID,
		}]
		if exist {
			if isBeforeScore {
				session.BeforeSessionScore = float64(score.Score)
				session.BeforeSessionConvCount = score.TotalConversationCount
				RetrieveCoachingEfficiencyStatsInfof(
					ctx, logger,
					"parent: %v, agent criterion: %v, before score: %v, count of conv: %v",
					parent.String(),
					session.String(),
					score.Score,
					score.TotalConversationCount,
				)
			} else {
				session.AfterSessionScore = float64(score.Score)
				session.AfterSessionConvCount = score.TotalConversationCount
				RetrieveCoachingEfficiencyStatsInfof(
					ctx, logger,
					"parent: %v, agent criterion: %v, after score: %v, count of conv: %v",
					parent.String(),
					session.String(),
					score.Score,
					score.TotalConversationCount,
				)
			}
		}
	}
	return nil
}

// userOutcomeAgentField keys an ALO value by (agent, outcome field definition).
type userOutcomeAgentField struct {
	AgentUserID         string
	FieldDefinitionName string
}

func retrieveUserOutcomeStatsForSameDayAgentCriterion(
	ctx context.Context,
	logger log.Logger,
	analyticsService userOutcomeStatsGetter,
	parent *shared.CustomerProfile,
	coachingSessions []*coachingSessionCriterion,
	sessionDate time.Time,
	isBeforeScore bool,
) error {
	sessionsByAggregationMode := map[analyticspb.AggregationMode][]*coachingSessionCriterion{}
	for _, c := range coachingSessions {
		sessionsByAggregationMode[c.OutcomeAggregationMode] = append(sessionsByAggregationMode[c.OutcomeAggregationMode], c)
	}

	timeRange := beforeAfterTimeRange(sessionDate, isBeforeScore)
	for aggregationMode, sessions := range sessionsByAggregationMode {
		if err := fillUserOutcomeWindow(ctx, logger, analyticsService, parent, sessions, timeRange, aggregationMode, isBeforeScore); err != nil {
			return err
		}
	}
	return nil
}

func fillUserOutcomeWindow(
	ctx context.Context,
	logger log.Logger,
	analyticsService userOutcomeStatsGetter,
	parent *shared.CustomerProfile,
	coachingSessions []*coachingSessionCriterion,
	timeRange *analyticspb.TimeRange,
	aggregationMode analyticspb.AggregationMode,
	isBeforeScore bool,
) error {
	agentUserIDsMap := map[string]bool{}
	fieldDefinitionNamesMap := map[string]bool{}
	for _, c := range coachingSessions {
		agentUserIDsMap[c.AgentUserID] = true
		fieldDefinitionNamesMap[c.OutcomeFieldDefinitionName] = true
	}
	agentUserIDs := fn.MapKeyToSlice(agentUserIDsMap)
	fieldDefinitionNames := fn.MapKeyToSlice(fieldDefinitionNamesMap)
	sort.Strings(agentUserIDs)
	sort.Strings(fieldDefinitionNames)
	userNames := fn.Map(agentUserIDs, func(s string) string {
		return userpb.UserName{CustomerID: parent.CustomerID, UserID: s}.String()
	})

	request := &analyticspb.RetrieveUserOutcomeStatsRequest{
		Parent: parent.String(),
		Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
			TimeRange:                   timeRange,
			Users:                       userNames,
			OutcomeFieldDefinitionNames: fieldDefinitionNames,
		},
		GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
			AggregationMode: aggregationMode,
			AttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
			},
		},
	}
	RetrieveCoachingEfficiencyStatsInfof(ctx, logger, "parent: %v, user outcome request timerange: %v, %v", parent.String(), timeRange.StartTimestamp.AsTime(), timeRange.EndTimestamp.AsTime())

	response, err := analyticsService.RetrieveUserOutcomeStats(ctx, request)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to query user outcome stats -- request: %v; error: %v", request, err)
	}

	valueMap := map[userOutcomeAgentField]*analyticspb.UserOutcomeStats{}
	for _, stat := range response.GetUserOutcomeStats() {
		agentUserName, err := userpb.ParseUserName(stat.GetGroupBy().GetUser())
		if err != nil {
			return err
		}
		valueMap[userOutcomeAgentField{
			AgentUserID:         agentUserName.UserID,
			FieldDefinitionName: stat.GetGroupBy().GetOutcomeFieldDefinitionName(),
		}] = stat
	}

	for _, session := range coachingSessions {
		stat, exist := valueMap[userOutcomeAgentField{
			AgentUserID:         session.AgentUserID,
			FieldDefinitionName: session.OutcomeFieldDefinitionName,
		}]
		if !exist {
			continue
		}
		if isBeforeScore {
			session.BeforeSessionScore = stat.GetValue()
			session.BeforeSessionConvCount = stat.GetRecordCount()
		} else {
			session.AfterSessionScore = stat.GetValue()
			session.AfterSessionConvCount = stat.GetRecordCount()
		}
	}
	return nil
}

func parseRetrieveCoachingEfficiencyStatsFilter(
	ctx context.Context,
	logger log.Logger,
	profile *shared.CustomerProfile,
	configClient config.Client,
	resourceACLHelper auth.ResourceACLHelper,
	userClientRegistry grpcRegistry.Registry[userpb.UserServiceClient],
	attribute *analyticspb.Attribute,
	userFilterParser userfilter.UserFilterParser,
) (*retrieveCoachingEfficiencyStatsFilter, error) {
	includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(attribute.GetGroupMembershipFilter())
	filteredUsersAndGroups, err := parseAgentAndGroupFilter(
		ctx, logger, profile, configClient, resourceACLHelper,
		userClientRegistry, attribute, userFilterParser, includeDirectGroupMembershipsOnly,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent & group filter: %w", err)
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsersAndGroups:                      filteredUsersAndGroups,
		IsIncludeDirectGroupMembershipsOnly: includeDirectGroupMembershipsOnly,
		CriterionIDs:                        attribute.GetCriterionIdentifiers(),
		CriterionCategories:                 attribute.GetCriterionCategories(),
	}

	filter.CoachingSessionIDs, err = fn.MapE(attribute.CoachingSessionNames, func(n string) (string, error) {
		sessionName, err := coachingpb.ParseCoachingSessionName(n)
		if err != nil {
			return "", status.Errorf(codes.InvalidArgument, "invalid coaching session names: %v", n)
		}
		return sessionName.CoachingSessionID, nil
	})
	if err != nil {
		return nil, err
	}

	if len(attribute.ScorecardTemplateNames) > 0 {
		scorecardTemplateIDs, err := fn.MapE(attribute.ScorecardTemplateNames, func(t string) (string, error) {
			name, err := coachingpb.ParseScorecardTemplateName(t)
			if err != nil {
				return "", status.Errorf(codes.InvalidArgument, "invalid scorecard template name: %v", t)
			}
			return name.ScorecardTemplateID, nil
		})
		if err != nil {
			return nil, err
		}
		filter.ScorecardTemplateIDs = scorecardTemplateIDs
		filter.ScorecardTemplateNames = attribute.ScorecardTemplateNames
	}

	if len(attribute.UsecaseNames) > 0 {
		filter.UsecaseNames = attribute.UsecaseNames
		filter.UsecaseIDs, err = fn.MapE(filter.UsecaseNames, func(u string) (string, error) {
			usecaseName, err := customerpb.ParseUsecaseName(u)
			if err != nil {
				return "", status.Errorf(codes.InvalidArgument, "invalid usecase name: %v", u)
			}
			return usecaseName.UsecaseID, nil
		})
		if err != nil {
			return nil, err
		}
	}
	return filter, nil
}

func fillBeforeAndAfterQAScores(
	ctx context.Context,
	logger log.Logger,
	parent *shared.CustomerProfile,
	qaScoreGetter qaScoreStatsGetter,
	userOutcomeGetter userOutcomeStatsGetter,
	reqFilter *retrieveCoachingEfficiencyStatsFilter,
	coachingSessions []*coachingSessionCriterion,
) error {
	sessionsGroupByDate := map[time.Time][]*coachingSessionCriterion{}
	for _, c := range coachingSessions {
		if !isSessionAtOldEnough(c.CoachingSessionAt) {
			continue
		}
		if c.CriterionID == "" {
			continue
		}
		date := getStartOfDay(c.CoachingSessionAt)
		if _, exist := sessionsGroupByDate[date]; !exist {
			sessionsGroupByDate[date] = []*coachingSessionCriterion{}
		}
		sessionsGroupByDate[date] = append(sessionsGroupByDate[date], c)
	}
	errorChan := make(chan error, len(sessionsGroupByDate))
	guard := make(chan struct{}, *NumberCoachingEfficiencyConcurrentQuery)
	for date, sessions := range sessionsGroupByDate {
		qaSessions := fn.Filter(sessions, func(c *coachingSessionCriterion) bool { return !c.IsUserOutcome })
		userOutcomeSessions := fn.Filter(sessions, func(c *coachingSessionCriterion) bool { return c.IsUserOutcome })
		go utils.AvoidPanic(ctx, func() {
			guard <- struct{}{}
			defer func() { <-guard }()

			if len(qaSessions) > 0 {
				if err := retrieveQAScoreStatsForSameDayAgentCriterion(
					ctx, logger, qaScoreGetter, parent, qaSessions, reqFilter, date, true /* isBeforeScore */); err != nil {
					errorChan <- err
					return
				}
				if err := retrieveQAScoreStatsForSameDayAgentCriterion(
					ctx, logger, qaScoreGetter, parent, qaSessions, reqFilter, date, false /* isBeforeScore */); err != nil {
					errorChan <- err
					return
				}
			}

			if len(userOutcomeSessions) > 0 {
				if err := retrieveUserOutcomeStatsForSameDayAgentCriterion(
					ctx, logger, userOutcomeGetter, parent, userOutcomeSessions, date, true /* isBeforeScore */); err != nil {
					errorChan <- err
					return
				}
				if err := retrieveUserOutcomeStatsForSameDayAgentCriterion(
					ctx, logger, userOutcomeGetter, parent, userOutcomeSessions, date, false /* isBeforeScore */); err != nil {
					errorChan <- err
					return
				}
			}
			errorChan <- nil
		})()
	}

	for i := 0; i < len(sessionsGroupByDate); i++ {
		err := <-errorChan
		if err != nil {
			return err
		}
	}

	return nil
}

func getStartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func getEndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999000000, t.Location())
}

func isSessionAtOldEnough(t time.Time) bool {
	sevenDaysAgo := time.Now().AddDate(0, 0, -*NumberDaysCoachingEfficiencyQAScoreQuery)
	return !t.After(sevenDaysAgo)
}

// beforeAfterTimeRange returns the ±N-day window around a session date (N = NumberDaysCoachingEfficiencyQAScoreQuery).
func beforeAfterTimeRange(sessionDate time.Time, isBeforeScore bool) *analyticspb.TimeRange {
	if isBeforeScore {
		endDate := getEndOfDay(sessionDate.AddDate(0, 0, -1))
		startDate := endDate.AddDate(0, 0, -*NumberDaysCoachingEfficiencyQAScoreQuery)
		return &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(startDate),
			EndTimestamp:   timestamppb.New(endDate),
		}
	}
	startDate := getEndOfDay(sessionDate.AddDate(0, 0, -1))
	endDate := startDate.AddDate(0, 0, *NumberDaysCoachingEfficiencyQAScoreQuery)
	return &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(startDate),
		EndTimestamp:   timestamppb.New(endDate),
	}
}

func (a AnalyticsServiceImpl) computeSessionEfficiency(ctx context.Context, s *coachingSessionCriterion) float64 {
	if s.IsUserOutcome {
		efficiency := s.AfterSessionScore - s.BeforeSessionScore
		RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "ALO efficiency for session %v: %v - %v = %v", s.String(), s.AfterSessionScore, s.BeforeSessionScore, efficiency)
		return efficiency
	}
	efficiency := (s.AfterSessionScore - s.BeforeSessionScore) * float64(s.AfterSessionConvCount)
	RetrieveCoachingEfficiencyStatsInfof(ctx, a.logger, "efficiency for session %v: (%v - %v) * %v = %v", s.String(), s.AfterSessionScore, s.BeforeSessionScore, float64(s.AfterSessionConvCount), efficiency)
	return efficiency
}

func parseAgentAndGroupFilter(
	ctx context.Context,
	logger log.Logger,
	profile *shared.CustomerProfile,
	configClient config.Client,
	resourceACLHelper auth.ResourceACLHelper,
	userClientRegistry grpcRegistry.Registry[userpb.UserServiceClient],
	attribute *analyticspb.Attribute,
	userFilterParser userfilter.UserFilterParser,
	includeDirectGroupMembershipsOnly bool,
) (*userfilter.FilteredUsersAndGroups, error) {
	userServiceClient, err := userClientRegistry.CreateClient(profile.CustomerID, profile.ProfileID)
	if err != nil {
		return nil, err
	}
	groupNames := fn.Map(attribute.Groups, func(g *userpb.Group) string {
		return g.Name
	})
	groupsByGroupType := map[userpb.Group_Type][]string{}
	if len(groupNames) > 0 {
		groupsByGroupType, err = userfilter.GroupsByGroupType(
			ctx, profile.CustomerID, profile.ProfileID, groupNames, userServiceClient,
		)
	}
	if err != nil {
		return nil, err
	}
	userNames := fn.Map(attribute.Users, func(u *userpb.User) string {
		return u.Name
	})
	userFilterConditions := &userfilter.UserFilterConditions{
		SelectedUserNames:         userNames,
		SelectedVirtualGroupNames: groupsByGroupType[userpb.Group_DYNAMIC_GROUP],
		SelectedTeamGroupNames:    groupsByGroupType[userpb.Group_TEAM],
		UserTypes: []enums.UserType{
			enums.UserType_PROD_USER,
			enums.UserType_GUEST_USER,
		},
		Roles: []authpb.AuthProto_Role{
			authpb.AuthProto_AGENT,
		},
		DirectTeamOnly: includeDirectGroupMembershipsOnly,
	}
	if attribute.ExcludeDeactivatedUsers {
		userFilterConditions.State = userpb.User_ACTIVE
	}
	if attribute.IncludeDevUsers {
		userFilterConditions.UserTypes = append(userFilterConditions.UserTypes, enums.UserType_DEV_USER)
	}
	filteredUsersAndGroups, err := userFilterParser.Parse(
		ctx, profile.CustomerID, userFilterConditions,
		userServiceClient, configClient, resourceACLHelper, logger,
	)
	if err != nil {
		return nil, err
	}
	return filteredUsersAndGroups, nil
}
