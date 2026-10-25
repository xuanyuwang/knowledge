package analyticsimpl

import (
	"context"
	"database/sql"
	"slices"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	qapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/qa"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/framework/stats"
	"github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	AnalystQuotaManager "github.com/cresta/go-servers/shared/analyst-quota-manager"
	coachingconverter "github.com/cresta/go-servers/shared/converters/coaching"
	"github.com/cresta/go-servers/shared/user"

	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	AgentQuotaManager "github.com/cresta/go-servers/shared/agent-quota-manager"
	"github.com/cresta/go-servers/shared/db/condition"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"
	"github.com/cresta/go-servers/shared/utils"
)

var badTemplateRevisionMetric = stats.NewMetric("insights.bad_template_revision")

const (
	// DATE_TRUNC(@frequency, submitted_at AT TIME ZONE @time_zone) # in @time_zone without timezone
	// DATE_TRUNC(@frequency, submitted_at AT TIME ZONE @time_zone) AT TIME ZONE # in @time_zone with timezone
	// Refer to https://www.postgresql.org/docs/13/functions-datetime.html "AT TIME ZONE" Section.
	formattedSubmitTimeQuery string = "DATE_TRUNC(" +
		"@frequency, " +
		"scorecards.submitted_at AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias) AT TIME ZONE @time_zone " +
		"- INTERVAL '1 day' * @first_day_of_week_bias AS formatted_submit_time, "
	formattedSessionCreateTimeQuery string = "DATE_TRUNC(" +
		"@frequency, " +
		"create_time AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias) AT TIME ZONE @time_zone " +
		"- INTERVAL '1 day' * @first_day_of_week_bias AS formatted_create_time, "
)

//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
type rowScorecardDetails struct {
	FormattedSubmitTime                  string
	SubmitTime                           string
	QaUserID                             string
	QaDisplayName                        string
	ScorecardID                          string
	ConversationID                       string
	MessageIds                           pq.StringArray `gorm:"column:message_ids;type:varchar[]"`
	ConversationCreatedAt                string
	AgentUserID                          string
	AgentDisplayName                     string
	Score                                float32
	Comment                              int
	NumCriteriaOverwritten               int
	Calibrated                           bool
	ConsistencyScore                     float32
	CalibratorUserID                     string
	CalibratorDisplayName                string
	CalibrationScorecardID               string
	CalibrationScorecardSubmittedAt      sql.NullTime
	CalibrationScorecardType             sql.NullInt32
	CalibrationScorecardReferenceID      sql.NullString
	CalibrationScorecardTemplateID       string
	CalibrationScorecardTemplateRevision string
	AcknowledgedAt                       string
	PublishedAt                          sql.NullTime
	ProcessInteractionAt                 string
	ProcessID                            string
}

type formattedSubmitTimeUserIDKey struct {
	FormattedSubmitTime string
	userID              string
}

type formattedSubmitTimeCount struct {
	FormattedSubmitTime string
	Count               int32
}

type rowSessionLogDetails struct {
	FormattedCreateTime           string
	SessionID                     string
	UserResourceName              string
	ScorecardTemplateResourceName string
	UserID                        string
}

// nolint:gosec
func (a AnalyticsServiceImpl) RetrieveManualQAStats(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	a.logger.Infof(ctx, "RetrieveManualQAStats RPC starts, req = %v", req)
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}

	// Validates the parent.
	parent, err := customerpb.ParseCustomerOrProfileName(req.Parent)
	if err != nil {
		return nil, err
	}

	// Get resource ACL based on the caller identity.
	req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(ctx, a.configClient, parent.CustomerID, a.resourceACLHelperProvider.Get(), req.FilterByAttribute.Users, req.FilterByAttribute.Groups, false)
	if err != nil {
		return nil, err
	}

	if req.FilterByAttribute != nil && (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
		includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
		filterByAttribute, err := shared.MoveFiltersToUserFilter(
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
		if len(filterByAttribute.Users) == 0 {
			return &analyticspb.RetrieveManualQAStatsResponse{}, nil
		}
		req.FilterByAttribute = filterByAttribute
	}

	err = validateRetrieveManualQAStatsRequest(req)
	if err != nil {
		return nil, err
	}

	switch {
	case len(req.GroupByAttributes) == 1 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
		if req.QaAnalyst != nil {
			if req.Frequency == analyticspb.Frequency_EVALUATION_PERIOD {
				return a.queryQAAnalystStatsGroupByPeriods(ctx, req, parent, true)
			}
			return a.queryQAAnalystStatsGroupByTime(ctx, req, parent)
		}
		if len(req.FilterByEvaluationPeriods) > 1 || req.Frequency == analyticspb.Frequency_EVALUATION_PERIOD {
			req.Frequency = analyticspb.Frequency_EVALUATION_PERIOD
			return a.queryCountStatsGroupByPeriods(ctx, req, parent)
		}
		return a.queryCountStatsGroupByTime(ctx, req, parent)
	case len(req.GroupByAttributes) == 1 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST:
		if len(req.FilterByEvaluationPeriods) > 0 {
			return a.queryQAAnalystStatsGroupByPeriods(ctx, req, parent, false)
		}
		req.Frequency = analyticspb.Frequency_EVALUATION_PERIOD
		return a.queryQAAnalystStatsGroupByTime(ctx, req, parent)
	case len(req.GroupByAttributes) == 2 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST &&
		req.GroupByAttributes[1].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
		if req.Frequency == analyticspb.Frequency_EVALUATION_PERIOD {
			return a.queryQAAnalystStatsGroupByPeriods(ctx, req, parent, true)
		}
		return a.queryQAAnalystStatsGroupByTime(ctx, req, parent)
	case len(req.GroupByAttributes) == 2 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT &&
		req.GroupByAttributes[1].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
		if req.Frequency == analyticspb.Frequency_EVALUATION_PERIOD {
			return a.queryAgentStatsGroupByPeriods(ctx, req, parent)
		}
		return a.queryAgentStatsGroupByTime(ctx, req, parent)

	default:
		return nil, status.Error(codes.InvalidArgument, "the group_by_attributes pattern is not supported")
	}
}

func (a AnalyticsServiceImpl) queryCountStatsGroupByTime(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest, parent *customerpb.CustomerOrProfileName,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}
	filterTemplateNoTask := filterTemplateNoTask(scorecardTemplateName, req.FilterByAttribute)

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}

	startTime, endTime := computeOverallTimeRange(req.FilterByEvaluationPeriods, req.FilterByTimeRange)

	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	filteredSubmissionSources := fn.Map(
		req.FilterByAttribute.GetScorecardSubmissionSources(),
		func(source analyticspb.Attribute_ScorecardSubmissionSource) int32 { return int32(source) },
	)

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}
	filteredSubmitterIDs := fn.MapKeyToSlice(scorecardReviewersMap)

	db := a.appsDB.DB(ctx)

	var period *qa.EvaluationPeriod
	quotaManager := &AgentQuotaManager.AgentQuotaManager{}
	filteredAgentUserIDs := fn.MapKeyToSlice(filteredAgentUsers)
	totalAssignedConversationCount := int32(0)
	if len(req.FilterByEvaluationPeriods) == 1 {
		// Query the template audiences.
		templates, err := qa.ListScorecardTemplates(
			db,
			scorecardTemplateName.CustomerID,
			scorecardTemplateName.ProfileID,
			scorecardTemplateName.ScorecardTemplateID,
			req.FilterByAttribute.UsecaseNames)
		if err != nil {
			return nil, err
		}

		resolvedTaskUsers, err := ResolveTemplateTaskUsers(ctx, userClient, parent, db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, taskID, templates[0])
		if err != nil {
			return nil, err
		}

		if len(filteredAgentUsers) > 0 {
			filteredAgentUserIDs = []string{}
			for audienceUserID := range resolvedTaskUsers {
				if _, exist := filteredAgentUsers[audienceUserID]; exist {
					filteredAgentUserIDs = append(filteredAgentUserIDs, audienceUserID)
				}
			}
		}

		// Get the evaluation period config.
		periods, err := getDBEvaluationPeriods(db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, req.FilterByEvaluationPeriods, taskID)
		if err != nil {
			return nil, err
		}
		period = periods[0]

		var userIDs []string
		if len(filteredAgentUserIDs) > 0 {
			userIDs = filteredAgentUserIDs
		} else {
			userIDs = fn.MapKeyToSlice[string, *userpb.User](resolvedTaskUsers)
		}

		if isConversationCoveragePeriod(period) {
			coverageCounts, err := a.countCoverageConversationsForPeriods(ctx, parent, templates[0], []*qa.EvaluationPeriod{period}, resolvedTaskUsers, userIDs)
			if err != nil {
				return nil, err
			}
			totalAssignedConversationCount = coverageCounts[0]
		} else {
			quotaManager, err = AgentQuotaManager.NewAgentQuotaManager(ctx, parent, userClient, []*qadbpb.TaskCompleteConfig{period.QATaskConfig.TaskCompleteConfig})
			if err != nil {
				return nil, err
			}
			totalAssignedConversationCount, err = quotaManager.GetTotalQuota(userIDs, period.QATaskConfig.GetTaskCompleteConfig())
			if err != nil {
				return nil, err
			}
		}
	}

	manualQaStats, totalManualQaStats, err := queryCountTimeRangeStats(
		db, parent, scorecardTemplateName.ScorecardTemplateID, usecaseIDs, startTime, endTime, period, totalAssignedConversationCount, &req.Frequency, req.Metadata, filteredAgentUserIDs, filteredSubmissionSources, filterTemplateNoTask, taskName, filteredSubmitterIDs)
	if err != nil {
		return nil, err
	}
	manualQaStats = append(manualQaStats, totalManualQaStats)

	return &analyticspb.RetrieveManualQAStatsResponse{
		ManualQaStats: manualQaStats,
	}, nil
}

func (a AnalyticsServiceImpl) queryCountStatsGroupByPeriods(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest, parent *customerpb.CustomerOrProfileName,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}

	db := a.appsDB.DB(ctx)
	templates, err := qa.ListScorecardTemplates(
		db,
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
		scorecardTemplateName.ScorecardTemplateID,
		req.FilterByAttribute.UsecaseNames)
	if err != nil {
		return nil, err
	}
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	resolvedTaskUsers, err := ResolveTemplateTaskUsers(ctx, userClient, parent, db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, taskID, templates[0])
	if err != nil {
		return nil, err
	}

	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}
	filteredAgentUserIDs := []string{}
	if len(filteredAgentUsers) > 0 {
		for audienceUserID := range resolvedTaskUsers {
			if _, exist := filteredAgentUsers[audienceUserID]; exist {
				filteredAgentUserIDs = append(filteredAgentUserIDs, audienceUserID)
			}
		}
	}

	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	periods, err := getDBEvaluationPeriods(db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, req.FilterByEvaluationPeriods, taskID)
	if err != nil {
		return nil, err
	}

	filteredSubmissionSources := fn.Map(
		req.FilterByAttribute.GetScorecardSubmissionSources(),
		func(source analyticspb.Attribute_ScorecardSubmissionSource) int32 { return int32(source) },
	)

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}
	filteredSubmitterIDs := fn.MapKeyToSlice(scorecardReviewersMap)

	manualQaStats := make([]*analyticspb.ManualQAStats, len(periods)+1)
	overallManualQaStats := &analyticspb.ManualQAStats{
		Attribute: &analyticspb.Attribute{},
	}

	taskCompleteConfigs := fn.Map(periods, func(period *qa.EvaluationPeriod) *qadbpb.TaskCompleteConfig {
		return period.QATaskConfig.TaskCompleteConfig
	})
	quotaManager, err := AgentQuotaManager.NewAgentQuotaManager(ctx, parent, userClient, taskCompleteConfigs)
	if err != nil {
		return nil, err
	}

	var userIDs []string
	if len(filteredAgentUserIDs) > 0 {
		userIDs = filteredAgentUserIDs
	} else {
		userIDs = fn.MapKeyToSlice(resolvedTaskUsers)
	}

	var coverageCounts []int32
	if slices.ContainsFunc(periods, isConversationCoveragePeriod) {
		coverageCounts, err = a.countCoverageConversationsForPeriods(ctx, parent, templates[0], periods, resolvedTaskUsers, userIDs)
		if err != nil {
			return nil, err
		}
	}

	for i, period := range periods {
		startTime := period.ScorecardTemplateEvaluationPeriod.StartTime.AsTime()
		endTime := period.ScorecardTemplateEvaluationPeriod.EndTime.AsTime()

		var totalAssignedConversationCount int32
		if isConversationCoveragePeriod(period) {
			totalAssignedConversationCount = coverageCounts[i]
		} else {
			totalAssignedConversationCount, err = quotaManager.GetTotalQuota(userIDs, period.QATaskConfig.TaskCompleteConfig)
			if err != nil {
				return nil, err
			}
		}

		_, periodManualQaStats, err := queryCountTimeRangeStats(
			db, parent, scorecardTemplateName.ScorecardTemplateID, usecaseIDs, startTime, endTime, period, totalAssignedConversationCount, &req.Frequency, req.Metadata, filteredAgentUserIDs, filteredSubmissionSources, false /* filterTemplateNoTask */, taskName, filteredSubmitterIDs)
		if err != nil {
			return nil, err
		}
		periodManualQaStats.Attribute = &analyticspb.Attribute{
			TimeRanges: []*analyticspb.TimeRange{
				{
					StartTimestamp: period.ScorecardTemplateEvaluationPeriod.StartTime,
					EndTimestamp:   period.ScorecardTemplateEvaluationPeriod.EndTime,
				},
			},
		}
		manualQaStats[i] = periodManualQaStats
		overallManualQaStats.EvaluatedConversationCount += periodManualQaStats.EvaluatedConversationCount
		overallManualQaStats.AssignedConversationCount += periodManualQaStats.AssignedConversationCount
		if len(periods) == 1 {
			overallManualQaStats.EvaluationPeriodProgressStatus = periodManualQaStats.EvaluationPeriodProgressStatus
		}
	}
	overallManualQaStats.CompletionRate = safeDivideFloat(overallManualQaStats.EvaluatedConversationCount, overallManualQaStats.AssignedConversationCount)
	manualQaStats[len(manualQaStats)-1] = overallManualQaStats

	return &analyticspb.RetrieveManualQAStatsResponse{
		ManualQaStats: manualQaStats,
	}, nil
}

func (a AnalyticsServiceImpl) queryQAAnalystStatsGroupByTime(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest, parent *customerpb.CustomerOrProfileName,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}

	db := a.appsDB.DB(ctx)
	scorecardTemplate, err := getScorecardTemplate(db, scorecardTemplateName)
	if err != nil {
		return nil, err
	}
	isProcessTemplate := isProcessTemplate(scorecardTemplate)
	filterTemplateNoTask := filterTemplateNoTask(scorecardTemplateName, req.FilterByAttribute)

	startTime, endTime := computeOverallTimeRange(req.FilterByEvaluationPeriods, req.FilterByTimeRange)

	filteredQaUserID, err := getFilteredQaUserID(req)
	if err != nil {
		return nil, err
	}
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}
	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	filteredSubmissionSources := fn.Map(
		req.FilterByAttribute.GetScorecardSubmissionSources(),
		func(source analyticspb.Attribute_ScorecardSubmissionSource) int32 { return int32(source) },
	)

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}
	filteredSubmitterIDs := fn.MapKeyToSlice(scorecardReviewersMap)

	var periods []*qa.EvaluationPeriod
	if len(req.FilterByEvaluationPeriods) > 0 {
		periods, err = getDBEvaluationPeriods(db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, req.FilterByEvaluationPeriods, taskID)
		if err != nil {
			return nil, err
		}
	}

	manualQaStats, err := queryQaUserTimeRangeStats(
		ctx, a.logger, userClient, db, parent, scorecardTemplateName.ScorecardTemplateID, usecaseIDs, startTime, endTime, periods,
		&req.Frequency, req.Metadata, filteredQaUserID, filteredAgentUsers, filteredSubmissionSources, isProcessTemplate, filterTemplateNoTask, taskName, filteredSubmitterIDs)
	if err != nil {
		return nil, err
	}
	return &analyticspb.RetrieveManualQAStatsResponse{ManualQaStats: manualQaStats}, nil
}

func (a AnalyticsServiceImpl) queryQAAnalystStatsGroupByPeriods(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest, parent *customerpb.CustomerOrProfileName, addPerPeriodStats bool,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}

	filteredQaUserID, err := getFilteredQaUserID(req)
	if err != nil {
		return nil, err
	}

	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	filteredSubmissionSources := fn.Map(
		req.FilterByAttribute.GetScorecardSubmissionSources(),
		func(source analyticspb.Attribute_ScorecardSubmissionSource) int32 { return int32(source) },
	)

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}
	filteredSubmitterIDs := fn.MapKeyToSlice(scorecardReviewersMap)

	db := a.appsDB.DB(ctx)
	periods, err := getDBEvaluationPeriods(db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, req.FilterByEvaluationPeriods, taskID)
	if err != nil {
		return nil, err
	}

	// Computes the stats in each period independently.
	// TODO(tinglin): Optimize it by running them in paralle.
	manualQaStats := []*analyticspb.ManualQAStats{}
	qaUserNameMap := make(map[string]*analyticspb.ManualQAStats)
	frequency := analyticspb.Frequency_EVALUATION_PERIOD
	for _, period := range periods {
		startTime := period.ScorecardTemplateEvaluationPeriod.StartTime.AsTime()
		endTime := period.ScorecardTemplateEvaluationPeriod.EndTime.AsTime()

		periodManualQaStats, err := queryQaUserTimeRangeStats(
			ctx, a.logger, userClient, db, parent, scorecardTemplateName.ScorecardTemplateID, usecaseIDs, startTime, endTime,
			[]*qa.EvaluationPeriod{period}, &frequency, req.Metadata, filteredQaUserID, filteredAgentUsers, filteredSubmissionSources, false /* isProcessTemplate */, false /* filterTemplateNoTask */, taskName, filteredSubmitterIDs)
		if err != nil {
			return nil, err
		}
		for _, stats := range periodManualQaStats {
			stats.Attribute.TimeRanges = []*analyticspb.TimeRange{
				{
					StartTimestamp: period.ScorecardTemplateEvaluationPeriod.StartTime,
					EndTimestamp:   period.ScorecardTemplateEvaluationPeriod.EndTime,
				},
			}
			qaUserName := stats.Attribute.Users[0].Name
			if _, exist := qaUserNameMap[qaUserName]; !exist {
				qaUserNameMap[qaUserName] = &analyticspb.ManualQAStats{
					Attribute: &analyticspb.Attribute{
						Users: stats.Attribute.Users,
					},
				}
			}
			aggregateQaManualQaStats(stats, qaUserNameMap[qaUserName])
			if len(periods) == 1 {
				qaUserNameMap[qaUserName].EvaluationPeriodProgressStatus = stats.EvaluationPeriodProgressStatus
			}
		}
		if addPerPeriodStats {
			manualQaStats = append(manualQaStats, periodManualQaStats...)
		}
	}
	manualQaStats = append(manualQaStats, sortUserStats(qaUserNameMap)...)

	return &analyticspb.RetrieveManualQAStatsResponse{ManualQaStats: manualQaStats}, nil
}

func (a AnalyticsServiceImpl) queryAgentStatsGroupByTime(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest, parent *customerpb.CustomerOrProfileName,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}

	startTime, endTime := computeOverallTimeRange(req.FilterByEvaluationPeriods, req.FilterByTimeRange)

	filteredQaUserID, err := getFilteredQaUserID(req)
	if err != nil {
		return nil, err
	}

	// Query the template audiences.
	db := a.appsDB.DB(ctx)
	scorecardTemplate, err := getScorecardTemplate(db, scorecardTemplateName)
	if err != nil {
		return nil, err
	}
	isProcessTemplate := isProcessTemplate(scorecardTemplate)
	filterTemplateNoTask := filterTemplateNoTask(scorecardTemplateName, req.FilterByAttribute)

	templates, err := qa.ListScorecardTemplates(
		db,
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
		scorecardTemplateName.ScorecardTemplateID,
		req.FilterByAttribute.UsecaseNames)
	if err != nil {
		return nil, err
	}
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	resolvedTaskUsers, err := ResolveTemplateTaskUsers(ctx, userClient, parent, db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, taskID, templates[0])
	if err != nil {
		return nil, err
	}
	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	filteredSubmissionSources := fn.Map(
		req.FilterByAttribute.GetScorecardSubmissionSources(),
		func(source analyticspb.Attribute_ScorecardSubmissionSource) int32 { return int32(source) },
	)

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}
	filteredSubmitterIDs := fn.MapKeyToSlice(scorecardReviewersMap)

	manualQaStats, err := queryAgentUserTimeRangeStats(
		db, parent, scorecardTemplateName.ScorecardTemplateID, usecaseIDs, startTime, endTime, &req.Frequency, req.Metadata, filteredQaUserID, resolvedTaskUsers, filteredAgentUsers, filteredSubmissionSources, isProcessTemplate, filterTemplateNoTask, taskName, filteredSubmitterIDs)
	if err != nil {
		return nil, err
	}

	return &analyticspb.RetrieveManualQAStatsResponse{ManualQaStats: manualQaStats}, nil
}

func (a AnalyticsServiceImpl) queryAgentStatsGroupByPeriods(
	ctx context.Context, req *analyticspb.RetrieveManualQAStatsRequest, parent *customerpb.CustomerOrProfileName,
) (*analyticspb.RetrieveManualQAStatsResponse, error) {
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}

	filteredQaUserID, err := getFilteredQaUserID(req)
	if err != nil {
		return nil, err
	}

	// Query the template audiences.
	db := a.appsDB.DB(ctx)
	templates, err := qa.ListScorecardTemplates(
		db,
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
		scorecardTemplateName.ScorecardTemplateID,
		req.FilterByAttribute.UsecaseNames)
	if err != nil {
		return nil, err
	}
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	resolvedTaskUsers, err := ResolveTemplateTaskUsers(ctx, userClient, parent, db, scorecardTemplateName, req.FilterByAttribute.UsecaseNames, taskID, templates[0])
	if err != nil {
		return nil, err
	}
	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	filteredSubmissionSources := fn.Map(
		req.FilterByAttribute.GetScorecardSubmissionSources(),
		func(source analyticspb.Attribute_ScorecardSubmissionSource) int32 { return int32(source) },
	)

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}
	filteredSubmitterIDs := fn.MapKeyToSlice(scorecardReviewersMap)

	// Computes the stats in each period independently.
	manualQaStats := []*analyticspb.ManualQAStats{}
	agentUserNameMap := make(map[string]*analyticspb.ManualQAStats)
	frequency := analyticspb.Frequency_EVALUATION_PERIOD
	for _, period := range req.FilterByEvaluationPeriods {
		startTime := period.StartTimestamp.AsTime()
		endTime := period.EndTimestamp.AsTime()
		periodManualQaStats, err := queryAgentUserTimeRangeStats(
			db, parent, scorecardTemplateName.ScorecardTemplateID, usecaseIDs, startTime, endTime, &frequency, req.Metadata, filteredQaUserID, resolvedTaskUsers, filteredAgentUsers, filteredSubmissionSources, false /* isProcessTemplate */, false /* filterTemplateNoTask */, taskName, filteredSubmitterIDs)
		if err != nil {
			return nil, err
		}
		for _, stats := range periodManualQaStats {
			stats.Attribute.TimeRanges = []*analyticspb.TimeRange{period}
			agentName := stats.Attribute.Users[0].Name
			if _, exist := agentUserNameMap[agentName]; !exist {
				agentUserNameMap[agentName] = &analyticspb.ManualQAStats{
					Attribute: &analyticspb.Attribute{
						Users: stats.Attribute.Users,
					},
				}
			}
			aggregateAgentManualQaStats(stats, agentUserNameMap[agentName])
		}
		manualQaStats = append(manualQaStats, periodManualQaStats...)
	}
	manualQaStats = append(manualQaStats, sortUserStats(agentUserNameMap)...)

	return &analyticspb.RetrieveManualQAStatsResponse{ManualQaStats: manualQaStats}, nil
}

func validateRetrieveManualQAStatsRequest(req *analyticspb.RetrieveManualQAStatsRequest) error {
	if len(req.FilterByEvaluationPeriods) == 0 && req.FilterByTimeRange == nil {
		return status.Errorf(codes.InvalidArgument, "either filter_by_evaluation_periods or filter_by_time_range is required.")
	}
	if len(req.FilterByAttribute.ScorecardTemplateNames) != 1 {
		return status.Errorf(codes.InvalidArgument, "single scorecard template name in the filter_by_attribute is required.")
	}
	if len(req.GroupByAttributes) == 0 {
		return status.Errorf(codes.InvalidArgument, "group_by_attributes is required.")
	}
	if len(req.FilterByAttribute.TaskNames) > 1 {
		return status.Errorf(codes.InvalidArgument, "only one or none task name in the filter_by_attribute is supported")
	}
	return nil
}

func computeOverallTimeRange(periods []*analyticspb.TimeRange, timeRange *analyticspb.TimeRange) (time.Time, time.Time) {
	if len(periods) == 0 {
		return timeRange.StartTimestamp.AsTime(), timeRange.EndTimestamp.AsTime()
	}

	var minStartTime, maxEndTime time.Time
	for i, period := range periods {
		startTime := period.StartTimestamp.AsTime()
		endTime := period.EndTimestamp.AsTime()
		if i == 0 {
			minStartTime = startTime
			maxEndTime = endTime
		} else {
			if startTime.Before(minStartTime) {
				minStartTime = startTime
			}
			if endTime.After(maxEndTime) {
				maxEndTime = endTime
			}
		}
	}
	return minStartTime, maxEndTime
}

func getFilteredQaUserID(req *analyticspb.RetrieveManualQAStatsRequest) (string, error) {
	filteredQaUserID := ""
	if req.GetQaAnalyst() != nil {
		qaUserName, err := userpb.ParseUserName(req.GetQaAnalyst().Name)
		if err != nil {
			return "", err
		}
		filteredQaUserID = qaUserName.UserID
	}
	return filteredQaUserID, nil
}

func (a AnalyticsServiceImpl) getFilteredAgentUsers(
	ctx context.Context,
	userClient userpb.UserServiceClient,
	parent *customerpb.CustomerOrProfileName,
	attribute *analyticspb.Attribute,
) (map[string]*userpb.User, error) {
	var userNames []string
	var groupNames []string

	if attribute.GetUsers() != nil {
		userNames = fn.Map(attribute.GetUsers(), func(user *userpb.User) string {
			return user.GetName()
		})
	}

	if attribute.GetGroups() != nil {
		groupNames = fn.Map(attribute.GetGroups(), func(group *userpb.Group) string {
			return group.GetName()
		})
	}

	return getUsersFromUserAndGroupNames(
		ctx,
		userClient,
		parent,
		userNames,
		groupNames,
		[]authpb.AuthProto_Role{authpb.AuthProto_AGENT},
	)
}

// getUsersFromUserAndGroupNames resolves user resource names and expands group names to user IDs.
//
// The optional roles filter only applies when expanding group members. Explicit user names are
// always preserved so existing user-selection semantics stay unchanged.
func getUsersFromUserAndGroupNames(
	ctx context.Context,
	userClient userpb.UserServiceClient,
	parent *customerpb.CustomerOrProfileName,
	userNames []string,
	groupNames []string,
	roles []authpb.AuthProto_Role,
) (map[string]*userpb.User, error) {
	filteredUsers := make(map[string]*userpb.User)

	if len(groupNames) > 0 {
		filter := &userpb.ListUsersRequest_ListUsersFilter{
			Groups:                          groupNames,
			IncludeIndirectGroupMemberships: true,
			CustomerProfiles:                []string{parent.String()},
			UserTypes:                       []enumspb.UserType{enumspb.UserType_PROD_USER, enumspb.UserType_GUEST_USER, enumspb.UserType_AI_AGENT_USER},
		}
		if len(roles) > 0 {
			filter.Roles = roles
		}
		err := user.QueryListUsers(ctx, userClient, parent, filter, &filteredUsers)
		if err != nil {
			return nil, err
		}
	}

	if len(userNames) > 0 {
		for _, userName := range userNames {
			parsedUserName, err := userpb.ParseUserName(userName)
			if err != nil {
				return nil, err
			}
			filteredUsers[parsedUserName.UserID] = &userpb.User{
				Name: userName,
			}
		}
	}

	return filteredUsers, nil
}

func computeEvaluationPeriodStatus(
	currentTime time.Time, completeRate float32, periodStartTime, periodEndTime time.Time,
) analyticspb.EvaluationPeriodProgressStatus {
	var progressPercentage float32
	if !currentTime.Before(periodEndTime) {
		progressPercentage = 1.0
	} else {
		totalDuration := periodEndTime.Sub(periodStartTime)
		elapsedDuration := currentTime.Sub(periodStartTime)
		progressPercentage = float32(elapsedDuration) / float32(totalDuration)
	}
	if completeRate >= progressPercentage {
		return analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_ON_TRACK
	}
	return analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_BEHIND_SCHEDULE
}

func isInTimeRange(t, startTime, endTime time.Time) bool {
	return !startTime.After(t) && endTime.After(t)
}

func FindEvaluationPeriod(timeRange *analyticspb.TimeRange, periods []*qa.EvaluationPeriod) *qa.EvaluationPeriod {
	for _, period := range periods {
		timeRangeStart := timeRange.StartTimestamp.AsTime()
		timeRangeEnd := timeRange.EndTimestamp.AsTime()
		periodStart := period.ScorecardTemplateEvaluationPeriod.StartTime.AsTime()
		periodEnd := period.ScorecardTemplateEvaluationPeriod.EndTime.AsTime()

		// Check for overlap
		if !timeRangeStart.Before(periodEnd) || !timeRangeEnd.After(periodStart) {
			continue
		}

		// Calculate overlapping time range
		overlapStart := periodStart
		if timeRangeStart.After(periodStart) {
			overlapStart = timeRangeStart
		}
		overlapEnd := periodEnd
		if timeRangeEnd.Before(periodEnd) {
			overlapEnd = timeRangeEnd
		}

		// Calculate durations
		overlapDuration := overlapEnd.Sub(overlapStart)
		periodDuration := periodEnd.Sub(periodStart)

		// Check if overlap is at least 80% of the period duration
		if float64(overlapDuration)/float64(periodDuration) > 0.8 {
			return period
		}
	}
	return nil
}

func getDBEvaluationPeriods(
	db *gorm.DB,
	scorecardTemplateName *coachingpb.ScorecardTemplateName,
	usecases []string,
	filterByEvaluationPeriods []*analyticspb.TimeRange,
	taskID string,
) ([]*qa.EvaluationPeriod, error) {
	var periods []*qa.EvaluationPeriod
	var err error
	periods, err = qa.ListEvaluationPeriodsForTask(db, scorecardTemplateName.CustomerID, scorecardTemplateName.ProfileID, usecases, taskID)
	if err != nil {
		return nil, err
	}
	if len(periods) == 0 && len(filterByEvaluationPeriods) > 0 {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid filter_by_evaluation_period because there is no qa task for the template: %s", scorecardTemplateName.String())
	}
	periodsWithinRange := make([]*qa.EvaluationPeriod, len(filterByEvaluationPeriods))
	for i, evaluationPeriod := range filterByEvaluationPeriods {
		period := FindEvaluationPeriod(evaluationPeriod, periods)
		if period == nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to find the corresponding config for the invalid evaluation period: %v", evaluationPeriod)
		}
		periodsWithinRange[i] = period
	}
	return periodsWithinRange, nil
}

func queryCountTimeRangeStats(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	templateID string,
	usecaseIDs []string,
	startTime, endTime time.Time,
	period *qa.EvaluationPeriod,
	totalAssignedConversationCount int32,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredAgentUserIDs []string,
	filteredSubmissionSources []int32,
	filterTemplateNoTask bool,
	taskName *coachingpb.DirectorTaskName,
	filteredSubmitterIDs []string,
) ([]*analyticspb.ManualQAStats, *analyticspb.ManualQAStats, error) {
	timeCounts, err := queryScorecardCount(db, parent, startTime, endTime, templateID, usecaseIDs, frequency, metadata, filteredAgentUserIDs, filteredSubmissionSources, filterTemplateNoTask, taskName, filteredSubmitterIDs)
	if err != nil {
		return nil, nil, err
	}

	manualQaStats := make([]*analyticspb.ManualQAStats, len(timeCounts))
	cumulatedEvaluatedConversationCount := int32(0)

	for i, timeCount := range timeCounts {
		cumulatedEvaluatedConversationCount += timeCount.Count
		manualQaStats[i] = &analyticspb.ManualQAStats{
			EvaluatedConversationCount: timeCount.Count,
			AssignedConversationCount:  totalAssignedConversationCount,
			CompletionRate:             safeDivideFloat(cumulatedEvaluatedConversationCount, totalAssignedConversationCount),
		}
		if timeCount.FormattedSubmitTime != "" {
			formattedSubmitAPITime, err := shared.ConvertStringToAPITimestamp(timeCount.FormattedSubmitTime)
			if err != nil {
				return nil, nil, err
			}
			manualQaStats[i].Attribute = &analyticspb.Attribute{
				TimeRanges: []*analyticspb.TimeRange{
					{
						StartTimestamp: formattedSubmitAPITime,
					},
				},
			}
		}
	}
	completeRate := safeDivideFloat(cumulatedEvaluatedConversationCount, totalAssignedConversationCount)
	overallManualQaStats := &analyticspb.ManualQAStats{
		EvaluatedConversationCount: cumulatedEvaluatedConversationCount,
		AssignedConversationCount:  totalAssignedConversationCount,
		CompletionRate:             completeRate,
	}
	if period != nil {
		if isInTimeRange(time.Now(), startTime, endTime) {
			overallManualQaStats.EvaluationPeriodProgressStatus = computeEvaluationPeriodStatus(
				time.Now(), completeRate, startTime, endTime,
			)
		}
	}
	return manualQaStats, overallManualQaStats, nil
}

func queryQaUserTimeRangeStats(
	ctx context.Context,
	logger log.Logger,
	userClient userpb.UserServiceClient,
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	templateID string,
	usecaseIDs []string,
	startTime, endTime time.Time,
	periods []*qa.EvaluationPeriod,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredQaUserID string,
	filteredAgentUsers map[string]*userpb.User,
	filteredScorecardSubmissionSources []int32,
	isProcessTemplate bool,
	filterTemplateNoTask bool,
	taskName *coachingpb.DirectorTaskName,
	filteredSubmitterIDs []string,
) ([]*analyticspb.ManualQAStats, error) {
	errorChan := make(chan error, 1)
	scorecardCriteriaOverwrittenMap := make(map[string]rowScorecardDetails)
	if !isProcessTemplate {
		go utils.AvoidPanic(ctx, func() {
			scorecardCriteriaOverwritten, err := queryScorecardDetailsCriteriaOverwrittenCount(
				ctx, logger, db, parent, startTime, endTime, templateID, usecaseIDs, filteredQaUserID, filteredAgentUsers, filteredScorecardSubmissionSources, filterTemplateNoTask, taskName, filteredSubmitterIDs)
			if err != nil {
				errorChan <- err
				return
			}
			for _, scorecardDetails := range scorecardCriteriaOverwritten {
				scorecardCriteriaOverwrittenMap[scorecardDetails.ScorecardID] = scorecardDetails
			}
			errorChan <- nil
		})()
	}

	scorecardDetailsRows, err := queryScorecardDetails(
		db, parent, startTime, endTime, templateID, usecaseIDs, frequency, metadata, filteredQaUserID, filteredAgentUsers, filteredScorecardSubmissionSources, isProcessTemplate, filterTemplateNoTask, taskName, filteredSubmitterIDs)
	if err != nil {
		return nil, err
	}

	// Calculate consistency scores for calibration scorecards
	err = calculateConsistencyScoresForScorecardDetails(db, parent, &scorecardDetailsRows)
	if err != nil {
		return nil, err
	}

	if !isProcessTemplate {
		err = <-errorChan
		if err != nil {
			return nil, err
		}
	}

	sessionLogDetailsRows := []rowSessionLogDetails{}
	if !isProcessTemplate {
		sessionLogDetailsRows, err = querySessionLogDetails(db, parent, startTime, endTime, templateID, frequency, metadata, filteredQaUserID, filteredAgentUsers)
		if err != nil {
			return nil, err
		}
	}

	// Aggregates the stats.
	manualQaStats := []*analyticspb.ManualQAStats{}
	qaUserIDMap, err := aggregateQaUserStats(parent, scorecardDetailsRows, scorecardCriteriaOverwrittenMap, sessionLogDetailsRows)
	if err != nil {
		return nil, err
	}

	if len(periods) > 0 {
		analystIDFullNameMap := make(map[string]string)
		for _, period := range periods {
			// If the given time range is an evaluation period, computes the complete rate stats for the QA analysts.
			qaAnalystQuotas := coachingconverter.DBToAPIQAAnalystQuota(period.QATaskConfig.GetQaAnalystQuotaConfig().GetQaAnalystQuotas())

			analystQuotaManager, err := AnalystQuotaManager.NewAnalystQuotaManager(ctx, parent, userClient, []*qapb.QAAnalystQuotaConfig{{
				QaAnalystQuotas: qaAnalystQuotas,
			}}, nil)
			if err != nil {
				return nil, err
			}

			analystQuotas := analystQuotaManager.GetFilteredUserQuotas(nil)

			for _, analystID := range fn.MapKeyToSlice(analystQuotas) {
				if analystID == "" {
					continue
				}

				if filteredQaUserID != "" && analystID != filteredQaUserID {
					continue
				}

				if _, exist := qaUserIDMap[analystID]; !exist {
					if _, exist := analystIDFullNameMap[analystID]; !exist {
						analystFullName := ""
						err := db.Select("full_name").Model(&dbmodel.Users{}).Where("customer_id = ? AND user_id = ?", parent.CustomerID, analystID).Scan(&analystFullName).Error
						if err != nil {
							return nil, err
						}
						analystIDFullNameMap[analystID] = analystFullName
					}
					qaUserIDMap[analystID] = &analyticspb.ManualQAStats{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: analystID}.String(),
									DisplayName: analystIDFullNameMap[analystID],
								},
							},
						},
					}
				}

				if *frequency == analyticspb.Frequency_EVALUATION_PERIOD && len(filteredAgentUsers) == 0 {
					qaUserIDMap[analystID].AssignedConversationCount += analystQuotas[analystID]
				}
			}
		}
		if *frequency == analyticspb.Frequency_EVALUATION_PERIOD {
			for _, qaUserStats := range qaUserIDMap {
				if qaUserStats.AssignedConversationCount > 0 {
					qaUserStats.CompletionRate = safeDivideFloat(qaUserStats.EvaluatedConversationCount, qaUserStats.AssignedConversationCount)
					if isInTimeRange(time.Now(), startTime, endTime) {
						qaUserStats.EvaluationPeriodProgressStatus = computeEvaluationPeriodStatus(
							time.Now(), qaUserStats.CompletionRate, startTime, endTime,
						)
					}
				} else {
					qaUserStats.CompletionRate = -1.
				}
			}
		}
	} else {
		for _, qaUserStats := range qaUserIDMap {
			qaUserStats.CompletionRate = -1.
		}
	}
	manualQaStats = append(manualQaStats, sortUserStats(qaUserIDMap)...)

	if *frequency != analyticspb.Frequency_EVALUATION_PERIOD {
		formattedSubmitTimeQaUserIDMap, err := aggregateQaUserFormattedSubmitTimeStats(parent, scorecardDetailsRows, scorecardCriteriaOverwrittenMap, sessionLogDetailsRows)
		if err != nil {
			return nil, err
		}
		manualQaStats = append(manualQaStats, sortUserTimeStats(formattedSubmitTimeQaUserIDMap)...)
	}

	return manualQaStats, nil
}

func queryScorecardCount(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	startTime, endTime time.Time,
	templateID string,
	usecaseIDs []string,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredAgentUserIDs []string,
	filteredSubmissionSources []int32,
	filterTemplateNoTask bool,
	taskName *coachingpb.DirectorTaskName,
	filteredSubmitterIDs []string,
) ([]formattedSubmitTimeCount, error) {
	query := db.Model(&dbmodel.Scorecards{}).
		Where("manually_scored = TRUE").
		Where("customer = ?", parent.CustomerID).
		Where("profile = ?", parent.ProfileID).
		Where("template_id = ?", templateID).
		Where("calibrated_scorecard_id IS NULL").
		Where("scorecard_type IS NULL OR scorecard_type = 0")

	query = query.Where("scorecards.submitted_at >= @start_ts_sec AND scorecards.submitted_at <= @end_ts_sec",
		sql.Named("start_ts_sec", startTime),
		sql.Named("end_ts_sec", endTime))

	if len(usecaseIDs) > 0 {
		query = query.Where("scorecards.usecase_id IN ?", usecaseIDs)
	}

	if filterTemplateNoTask {
		query = query.Where("array_length(scorecards.task_ids, 1) IS NULL OR ? = ANY(scorecards.task_ids)", shared.NoTask)
	} else if taskName != nil {
		query = query.Where("? = ANY(scorecards.task_ids)", taskName.DirectorTaskID)
	}

	if len(filteredAgentUserIDs) > 0 {
		query = query.Where("scorecards.agent_user_id IN ?", filteredAgentUserIDs)
	}

	if len(filteredSubmissionSources) > 0 {
		query = query.Where("scorecards.submission_source in ?", filteredSubmissionSources)
	}

	if len(filteredSubmitterIDs) > 0 {
		query = query.Where("scorecards.submitter_user_id in ?", filteredSubmitterIDs)
	}

	if *frequency != analyticspb.Frequency_EVALUATION_PERIOD {
		args, err := postgres.ParseArgs(frequency, metadata)
		if err != nil {
			return nil, err
		}
		query = query.Select(formattedSubmitTimeQuery+"COUNT(*) as count", args...).
			Group("formatted_submit_time").
			Order("formatted_submit_time")

		var timeCounts []formattedSubmitTimeCount
		err = query.Scan(&timeCounts).Error
		if err != nil {
			return nil, err
		}
		return timeCounts, nil
	}

	var scorecardCount int64
	err := query.Select("COUNT(*) as count").Count(&scorecardCount).Error
	if err != nil {
		return nil, err
	}
	return []formattedSubmitTimeCount{
		{
			Count: int32(scorecardCount),
		},
	}, nil
}

func queryScorecardDetails(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	startTime, endTime time.Time,
	templateID string,
	usecaseIDs []string,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredQaUserID string,
	filteredAgentUsers map[string]*userpb.User,
	filteredScorecardSubmissionSources []int32,
	isProcessTemplate bool,
	filterTemplateNoTask bool,
	taskName *coachingpb.DirectorTaskName,
	filteredSubmitterIDs []string,
) ([]rowScorecardDetails, error) {
	scorecardDetailsQuery := `scorecards.submitted_at as submit_time,
		su.user_id as qa_user_id,
		CASE WHEN su.full_name IS NULL OR su.full_name = '' THEN su.display_name ELSE su.full_name END as qa_display_name,
		scorecards.resource_id as scorecard_id,
		scorecards.conversation_id as conversation_id,
		scorecards.message_ids as message_ids,
		au.user_id as agent_user_id,
		CASE WHEN au.full_name IS NULL OR au.full_name = '' THEN au.display_name ELSE au.full_name END as agent_display_name,
		COALESCE(scorecards.score, -1.0) as score,
		CASE WHEN scorecards.comment IS NULL THEN 0 ELSE 1 END AS comment,
		csc.resource_id IS NOT NULL as calibrated,
		-1.0 as consistency_score,
		COALESCE(cu.user_id, '') as calibrator_user_id,
		csc.resource_id as calibration_scorecard_id,
		csc.submitted_at as calibration_scorecard_submitted_at,
		csc.scorecard_type as calibration_scorecard_type,
		csc.reference_scorecard_id as calibration_scorecard_reference_id,
		csc.template_id as calibration_scorecard_template_id,
		csc.template_revision as calibration_scorecard_template_revision,
		COALESCE(cu.full_name, '') as calibrator_display_name,
		scorecards.process_interaction_at as process_interaction_at,
		scorecards.process_id as process_id,
		scorecards.acknowledged_at as acknowledged_at,
		scorecards.published_at as published_at`
	if !isProcessTemplate {
		scorecardDetailsQuery += ", c.created_at as conversation_created_at"
	}

	query := db.Model(&dbmodel.Scorecards{}).
		Joins("LEFT JOIN director.scorecards csc ON scorecards.customer=csc.customer AND scorecards.profile=csc.profile AND scorecards.resource_id=csc.calibrated_scorecard_id AND csc.submitted_at IS NOT NULL").
		Joins("LEFT JOIN app.users cu ON csc.customer=cu.customer_id AND csc.submitter_user_id=cu.user_id")
	if !isProcessTemplate {
		query = query.Joins("LEFT JOIN app.chats c ON scorecards.conversation_id = c.conversation_id")
	}
	query = query.Joins("JOIN app.users su ON su.customer_id = scorecards.customer AND scorecards.submitter_user_id = su.user_id").
		Joins("JOIN app.users au ON au.customer_id = scorecards.customer AND scorecards.agent_user_id = au.user_id").
		Where("scorecards.manually_scored = TRUE").
		Where("scorecards.customer = ?", parent.CustomerID).
		Where("scorecards.profile = ?", parent.ProfileID).
		Where("scorecards.template_id = ?", templateID).
		Where("scorecards.calibrated_scorecard_id IS NULL").
		Where("scorecards.scorecard_type IS NULL OR scorecards.scorecard_type = 0")

	query = query.Where("scorecards.submitted_at >= @start_ts_sec AND scorecards.submitted_at <= @end_ts_sec",
		sql.Named("start_ts_sec", startTime),
		sql.Named("end_ts_sec", endTime))

	if len(usecaseIDs) > 0 {
		query = query.Where("scorecards.usecase_id IN ?", usecaseIDs)
	}

	if filterTemplateNoTask {
		query = query.Where("array_length(scorecards.task_ids, 1) IS NULL OR ? = ANY(scorecards.task_ids)", shared.NoTask)
	} else if taskName != nil {
		query = query.Where("? = ANY(scorecards.task_ids)", taskName.DirectorTaskID)
	}

	if len(filteredAgentUsers) > 0 {
		query = query.Where("scorecards.agent_user_id IN ?", fn.MapKeyToSlice(filteredAgentUsers))
	}

	if len(filteredScorecardSubmissionSources) > 0 {
		query = query.Where("scorecards.submission_source in ?", filteredScorecardSubmissionSources)
	}

	if len(filteredSubmitterIDs) > 0 {
		query = query.Where("scorecards.submitter_user_id in ?", filteredSubmitterIDs)
	}

	if *frequency != analyticspb.Frequency_EVALUATION_PERIOD {
		args, err := postgres.ParseArgs(frequency, metadata)
		if err != nil {
			return nil, err
		}
		query = query.Select(formattedSubmitTimeQuery+scorecardDetailsQuery, args...)
	} else {
		query = query.Select(scorecardDetailsQuery)
	}

	if filteredQaUserID != "" {
		query = query.Where("scorecards.submitter_user_id = ?", filteredQaUserID)
	}

	var scorecardDetailsRows []rowScorecardDetails
	err := query.Scan(&scorecardDetailsRows).Error
	if err != nil {
		return nil, err
	}
	return scorecardDetailsRows, nil
}

func queryScorecardDetailsCriteriaOverwrittenCount(
	ctx context.Context,
	logger log.Logger,
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	startTime, endTime time.Time,
	templateID string,
	usecaseIDs []string,
	filteredQaUserID string,
	filteredAgentUsers map[string]*userpb.User,
	filteredScorecardSubmissionSources []int32,
	filterTemplateNoTask bool,
	taskName *coachingpb.DirectorTaskName,
	filteredSubmitterIDs []string,
) ([]rowScorecardDetails, error) {
	// Get criteria from all template revisions for this template ID.
	templates, err := qa.ListScorecardTemplates(db, parent.CustomerID, parent.ProfileID, templateID, nil /* usecases */)
	if err != nil {
		return nil, err
	}
	scorableCriterionSet := set.New[string]()
	for _, template := range templates {
		templateStructure, err := scoring.ParseScorecardTemplateStructure(template.Template)
		if err != nil {
			logger.Warnf(ctx, "Bad template revision: resource_id=%s revision=%s title=%s customer=%s profile=%s err=%v",
				template.ResourceID, template.Revision, template.Title, template.Customer, template.Profile, err)
			badTemplateRevisionMetric.Incr(ctx)
			return nil, err
		}
		for _, criterion := range templateStructure.GetCriteriaSlice(false /* rootOnly */) {
			if criterion.IsOutcomeCriterion() {
				continue
			}
			scorableCriterionSet.Add(criterion.GetIdentifier())
		}
	}
	scorableCriterion := scorableCriterionSet.Slice()
	if len(scorableCriterion) == 0 {
		return []rowScorecardDetails{}, nil
	}

	// Query the criteria scores that have been overwritten.
	query := db.
		Model(&dbmodel.Scorecards{}).
		Select("scorecards.resource_id as scorecard_id, COUNT(DISTINCT criterion_identifier) AS num_criteria_overwritten").
		Joins("JOIN director.scores s ON s.customer = scorecards.customer and s.profile = scorecards.profile and s.scorecard_id = scorecards.resource_id").
		Where("manually_scored = TRUE").
		Where("scorecards.customer = ?", parent.CustomerID).
		Where("scorecards.profile = ?", parent.ProfileID).
		Where("scorecards.template_id = ?", templateID).
		Where("scorecards.calibrated_scorecard_id IS NULL").
		Where("scorecards.scorecard_type IS NULL OR scorecards.scorecard_type = 0").
		Where("s.ai_scored = TRUE").
		Where("s.numeric_value IS DISTINCT FROM s.ai_value").
		Where("s.criterion_identifier IN ?", scorableCriterion).
		Group("scorecards.resource_id")

	query = query.Where("scorecards.submitted_at >= @start_ts_sec AND scorecards.submitted_at <= @end_ts_sec",
		sql.Named("start_ts_sec", startTime),
		sql.Named("end_ts_sec", endTime))

	if len(usecaseIDs) > 0 {
		query = query.Where("scorecards.usecase_id IN ?", usecaseIDs)
	}

	if filterTemplateNoTask {
		query = query.Where("array_length(scorecards.task_ids, 1) IS NULL OR ? = ANY(scorecards.task_ids)", shared.NoTask)
	} else if taskName != nil {
		query = query.Where("? = ANY(scorecards.task_ids)", taskName.DirectorTaskID)
	}

	if filteredQaUserID != "" {
		query = query.Where("scorecards.submitter_user_id = ?", filteredQaUserID)
	}

	if len(filteredAgentUsers) > 0 {
		query = query.Where("scorecards.agent_user_id IN ?", fn.MapKeyToSlice(filteredAgentUsers))
	}

	if len(filteredScorecardSubmissionSources) > 0 {
		query = query.Where("scorecards.submission_source in ?", filteredScorecardSubmissionSources)
	}

	if len(filteredSubmitterIDs) > 0 {
		query = query.Where("scorecards.submitter_user_id in ?", filteredSubmitterIDs)
	}

	var scorecardDetailsRows []rowScorecardDetails
	err = query.Scan(&scorecardDetailsRows).Error
	if err != nil {
		return nil, err
	}
	return scorecardDetailsRows, nil
}

func querySessionLogDetails(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	startTime, endTime time.Time,
	templateID string,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredQaUserID string,
	filteredAgentUsers map[string]*userpb.User,
) ([]rowSessionLogDetails, error) {
	sessionLogDetailsQuery := `
		session_id,
		payload->'directorButtonClickPayload'->>'userResourceName' as user_resource_name,
		payload->'directorButtonClickPayload'->>'scorecardTemplateResourceName' as scorecard_template_resource_name`

	query := db.Model(&dbmodel.SessionLogs{}).
		Where("type = 6").
		Where("payload->'directorButtonClickPayload'->>'directorButton' = '1'").
		Where("create_time >= @start_ts_sec AND create_time < @end_ts_sec",
			sql.Named("start_ts_sec", startTime),
			sql.Named("end_ts_sec", endTime)).
		Where("customer_id = ?", parent.CustomerID).
		Where("profile_id = ?", parent.ProfileID)

	if len(filteredAgentUsers) > 0 {
		query = query.Where("payload->'directorButtonClickPayload'->>'agentResourceName' IN ?", fn.MapKeyToSlice(filteredAgentUsers))
	}

	if filteredQaUserID != "" {
		filterdQaUserName := userpb.UserName{CustomerID: parent.CustomerID, UserID: filteredQaUserID}.String()
		query = query.Where("payload->'directorButtonClickPayload'->>'userResourceName' = ?", filterdQaUserName)
	}

	if *frequency != analyticspb.Frequency_EVALUATION_PERIOD {
		args, err := postgres.ParseArgs(frequency, metadata)
		if err != nil {
			return nil, err
		}
		query = query.Select(formattedSessionCreateTimeQuery+sessionLogDetailsQuery, args...)
	} else {
		query = query.Select(sessionLogDetailsQuery)
	}

	var sessionLogDetailsRows []rowSessionLogDetails
	err := query.Scan(&sessionLogDetailsRows).Error
	if err != nil {
		return nil, err
	}

	filteredSessionLogDetailsRows := []rowSessionLogDetails{}
	for _, sessionLog := range sessionLogDetailsRows {
		if sessionLog.ScorecardTemplateResourceName == "" || sessionLog.UserResourceName == "" {
			continue
		}

		scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(sessionLog.ScorecardTemplateResourceName)
		if err != nil {
			return nil, err
		}
		if scorecardTemplateName.ScorecardTemplateID != templateID {
			continue
		}
		userName, err := userpb.ParseUserName(sessionLog.UserResourceName)
		if err != nil {
			return nil, err
		}
		sessionLog.UserID = userName.UserID

		filteredSessionLogDetailsRows = append(filteredSessionLogDetailsRows, sessionLog)
	}

	return filteredSessionLogDetailsRows, nil
}

func aggregateQaUserStats(
	parent *customerpb.CustomerOrProfileName,
	scorecardDetailsRows []rowScorecardDetails,
	scorecardCriteriaOverwrittenMap map[string]rowScorecardDetails,
	sessionLogDetailsRows []rowSessionLogDetails,
) (map[string]*analyticspb.ManualQAStats, error) {
	qaUserIDMap := make(map[string]*analyticspb.ManualQAStats)
	for _, scorecardDetails := range scorecardDetailsRows {
		if _, exist := qaUserIDMap[scorecardDetails.QaUserID]; !exist {
			qaUserIDMap[scorecardDetails.QaUserID] = &analyticspb.ManualQAStats{
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
							DisplayName: scorecardDetails.QaDisplayName,
						},
					},
				},
			}
		}
		err := updateQaStats(parent, scorecardDetails, scorecardCriteriaOverwrittenMap, qaUserIDMap[scorecardDetails.QaUserID])
		if err != nil {
			return nil, err
		}
	}
	for _, sessionLog := range sessionLogDetailsRows {
		if qaUserAttribute, exist := qaUserIDMap[sessionLog.UserID]; exist {
			qaUserAttribute.SkipSuggestedConversationCount += 1
		}
	}
	return qaUserIDMap, nil
}

func aggregateQaUserFormattedSubmitTimeStats(
	parent *customerpb.CustomerOrProfileName,
	scorecardDetailsRows []rowScorecardDetails,
	scorecardCriteriaOverwrittenMap map[string]rowScorecardDetails,
	sessionLogDetailsRows []rowSessionLogDetails,
) (map[formattedSubmitTimeUserIDKey]*analyticspb.ManualQAStats, error) {
	formattedSubmitTimeQaUserIDMap := make(map[formattedSubmitTimeUserIDKey]*analyticspb.ManualQAStats)
	for _, scorecardDetails := range scorecardDetailsRows {
		key := formattedSubmitTimeUserIDKey{FormattedSubmitTime: scorecardDetails.FormattedSubmitTime, userID: scorecardDetails.QaUserID}
		if _, exist := formattedSubmitTimeQaUserIDMap[key]; !exist {
			formatttedSubmitAPITime, err := shared.ConvertStringToAPITimestamp(scorecardDetails.FormattedSubmitTime)
			if err != nil {
				return nil, err
			}
			formattedSubmitTimeQaUserIDMap[key] = &analyticspb.ManualQAStats{
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
							DisplayName: scorecardDetails.QaDisplayName,
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: formatttedSubmitAPITime,
						},
					},
				},
			}
		}

		err := updateQaStats(parent, scorecardDetails, scorecardCriteriaOverwrittenMap, formattedSubmitTimeQaUserIDMap[key])
		if err != nil {
			return nil, err
		}
	}
	for _, sessionLog := range sessionLogDetailsRows {
		key := formattedSubmitTimeUserIDKey{FormattedSubmitTime: sessionLog.FormattedCreateTime, userID: sessionLog.UserID}
		if qaUserAttribute, exist := formattedSubmitTimeQaUserIDMap[key]; exist {
			qaUserAttribute.SkipSuggestedConversationCount += 1
		}
	}

	return formattedSubmitTimeQaUserIDMap, nil
}

func updateQaStats(
	parent *customerpb.CustomerOrProfileName,
	scorecardDetails rowScorecardDetails,
	scorecardCriteriaOverwrittenMap map[string]rowScorecardDetails,
	stats *analyticspb.ManualQAStats,
) error {
	conversationInfo := analyticspb.ManualQAConversationInfo{
		ScorecardName: coachingpb.ScorecardName{
			CustomerID:  parent.CustomerID,
			ProfileID:   parent.ProfileID,
			ScorecardID: scorecardDetails.ScorecardID,
		}.String(),
		Agent: &userpb.User{
			Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.AgentUserID}.String(),
			DisplayName: scorecardDetails.AgentDisplayName,
		},
		QaAnalyst: &userpb.User{
			Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
			DisplayName: scorecardDetails.QaDisplayName,
		},
		PerfScore: scorecardDetails.Score,
	}
	if scorecardDetails.ConversationCreatedAt != "" {
		convoCreatedAt, err := shared.ConvertStringToAPITimestamp(scorecardDetails.ConversationCreatedAt)
		if err != nil {
			return err
		}
		conversationInfo.Conversation = &conversationpb.Conversation{
			Name: conversationpb.ConversationName{
				CustomerID:     parent.CustomerID,
				ProfileID:      parent.ProfileID,
				ConversationID: scorecardDetails.ConversationID,
			}.String(),
			CreateTime: convoCreatedAt,
		}
	}
	if len(scorecardDetails.MessageIds) > 0 {
		for _, messageID := range scorecardDetails.MessageIds {
			conversationInfo.MessageNames = append(conversationInfo.MessageNames, conversationpb.ConversationMessageName{
				CustomerID:     parent.CustomerID,
				ProfileID:      parent.ProfileID,
				ConversationID: scorecardDetails.ConversationID,
				MessageID:      messageID,
			}.String())
		}
	}
	if scorecardDetails.ProcessID != "" {
		conversationInfo.ProcessId = scorecardDetails.ProcessID
	}
	if scorecardDetails.ProcessInteractionAt != "" {
		processInteractionTime, err := shared.ConvertStringToAPITimestamp(scorecardDetails.ProcessInteractionAt)
		if err != nil {
			return err
		}
		conversationInfo.ProcessInteractionTime = processInteractionTime
	}
	if scorecardDetails.Calibrated {
		conversationInfo.ConsistencyScore = scorecardDetails.ConsistencyScore
		conversationInfo.Calibrator = &userpb.User{
			Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.CalibratorUserID}.String(),
			DisplayName: scorecardDetails.CalibratorDisplayName,
		}
		stats.CalibratedConversations = append(stats.CalibratedConversations, &conversationInfo)
		stats.AvgConsistencyScore = aggregateAvgConsistencyScore(stats.CalibratedConversations)
	}
	stats.EvaluatedConversations = append(stats.EvaluatedConversations, &conversationInfo)
	stats.EvaluatedConversationCount += 1
	stats.AvgPerfScore = aggregateAvgPerfScore(stats.EvaluatedConversations)

	if scorecardDetails.Comment > 0 {
		stats.CommentedConversations = append(stats.CommentedConversations, &conversationInfo)
	}
	if _, exist := scorecardCriteriaOverwrittenMap[scorecardDetails.ScorecardID]; exist {
		stats.CriteriaOverwrittenConversations = append(stats.CriteriaOverwrittenConversations, &conversationInfo)
		stats.OverwrittenCriteriaCount += int32(scorecardCriteriaOverwrittenMap[scorecardDetails.ScorecardID].NumCriteriaOverwritten)
	}
	if scorecardDetails.AcknowledgedAt != "" {
		stats.AcknowledgedConversations = append(stats.AcknowledgedConversations, &conversationInfo)
	}
	if scorecardDetails.PublishedAt.Valid {
		stats.PublishedConversations = append(stats.PublishedConversations, &conversationInfo)
	}
	return nil
}

func aggregateAvgPerfScore(conversations []*analyticspb.ManualQAConversationInfo) float32 {
	perfScoreSum := float32(0.)
	perfScoreCount := 0
	for _, conversation := range conversations {
		if conversation.PerfScore >= 0 {
			perfScoreSum += conversation.PerfScore
			perfScoreCount += 1
		}
	}
	if perfScoreCount > 0 {
		return perfScoreSum / float32(perfScoreCount)
	}
	return -1.0
}

func aggregateAvgConsistencyScore(conversations []*analyticspb.ManualQAConversationInfo) float32 {
	consistencyScoreSum := float32(0.)
	consistencyScoreCount := 0
	for _, conversation := range conversations {
		if conversation.ConsistencyScore >= 0 {
			consistencyScoreSum += conversation.ConsistencyScore
			consistencyScoreCount += 1
		}
	}
	if consistencyScoreCount > 0 {
		return consistencyScoreSum / float32(consistencyScoreCount)
	}
	return -1.0
}

func aggregateQaManualQaStats(from *analyticspb.ManualQAStats, to *analyticspb.ManualQAStats) {
	to.EvaluatedConversations = append(to.EvaluatedConversations, from.EvaluatedConversations...)
	to.CommentedConversations = append(to.CommentedConversations, from.CommentedConversations...)
	to.AcknowledgedConversations = append(to.AcknowledgedConversations, from.AcknowledgedConversations...)
	to.PublishedConversations = append(to.PublishedConversations, from.PublishedConversations...)
	to.CriteriaOverwrittenConversations = append(to.CriteriaOverwrittenConversations, from.CriteriaOverwrittenConversations...)
	to.OverwrittenCriteriaCount += from.OverwrittenCriteriaCount
	to.AssignedConversationCount += from.AssignedConversationCount
	to.EvaluatedConversationCount += from.EvaluatedConversationCount
	to.AvgPerfScore = aggregateAvgPerfScore(to.EvaluatedConversations)
	if to.CompletionRate >= 0. && from.CompletionRate >= 0 {
		to.CompletionRate = safeDivideFloat(to.EvaluatedConversationCount, to.AssignedConversationCount)
	} else {
		to.CompletionRate = -1.
	}
	to.CalibratedConversations = append(to.CalibratedConversations, from.CalibratedConversations...)
	to.AvgConsistencyScore = aggregateAvgConsistencyScore(to.CalibratedConversations)
	to.SkipSuggestedConversationCount += from.SkipSuggestedConversationCount
}

func queryAgentUserTimeRangeStats(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	templateID string,
	usecaseIDs []string,
	startTime, endTime time.Time,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredQaUserID string,
	templateAudienceUsers map[string]*userpb.User,
	filteredAgentUsers map[string]*userpb.User,
	filteredSubmissionSources []int32,
	isProcessTemplate bool,
	filterTemplateNoTask bool,
	taskName *coachingpb.DirectorTaskName,
	filteredSubmitterIDs []string,
) ([]*analyticspb.ManualQAStats, error) {
	scorecardDetailsRows, err := queryScorecardDetails(
		db, parent, startTime, endTime, templateID, usecaseIDs, frequency, metadata, filteredQaUserID, filteredAgentUsers, filteredSubmissionSources, isProcessTemplate, filterTemplateNoTask, taskName, filteredSubmitterIDs)
	if err != nil {
		return nil, err
	}

	// Aggregates the stats.
	manualQaStats := []*analyticspb.ManualQAStats{}
	agentUserIDMap, err := aggregateAgentUserStats(parent, templateAudienceUsers, scorecardDetailsRows, filteredAgentUsers)
	if err != nil {
		return nil, err
	}
	manualQaStats = append(manualQaStats, sortUserStats(agentUserIDMap)...)

	if *frequency != analyticspb.Frequency_EVALUATION_PERIOD {
		formattedSubmitTimeAgentUserIDMap, err := aggregateAgentUserFormattedSubmitTimeStats(parent, templateAudienceUsers, scorecardDetailsRows)
		if err != nil {
			return nil, err
		}
		manualQaStats = append(manualQaStats, sortUserTimeStats(formattedSubmitTimeAgentUserIDMap)...)
	}

	return manualQaStats, nil
}

func aggregateAgentUserStats(
	parent *customerpb.CustomerOrProfileName,
	templateAudienceUsers map[string]*userpb.User,
	scorecardDetailsRows []rowScorecardDetails,
	filteredAgentUsers map[string]*userpb.User,
) (map[string]*analyticspb.ManualQAStats, error) {
	agentUserIDMap := make(map[string]*analyticspb.ManualQAStats)
	for _, scorecardDetails := range scorecardDetailsRows {
		if _, exist := templateAudienceUsers[scorecardDetails.AgentUserID]; !exist {
			continue
		}
		if _, exist := agentUserIDMap[scorecardDetails.AgentUserID]; !exist {
			agentUserIDMap[scorecardDetails.AgentUserID] = &analyticspb.ManualQAStats{
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{templateAudienceUsers[scorecardDetails.AgentUserID]},
				},
			}
		}
		stats := agentUserIDMap[scorecardDetails.AgentUserID]
		conversationInfo := analyticspb.ManualQAConversationInfo{
			ScorecardName: coachingpb.ScorecardName{
				CustomerID:  parent.CustomerID,
				ProfileID:   parent.ProfileID,
				ScorecardID: scorecardDetails.ScorecardID,
			}.String(),
			Agent: &userpb.User{
				Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.AgentUserID}.String(),
				DisplayName: scorecardDetails.AgentDisplayName,
			},
			QaAnalyst: &userpb.User{
				Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
				DisplayName: scorecardDetails.QaDisplayName,
			},
			PerfScore: scorecardDetails.Score,
		}
		if scorecardDetails.ConversationCreatedAt != "" {
			convoCreatedAt, err := shared.ConvertStringToAPITimestamp(scorecardDetails.ConversationCreatedAt)
			if err != nil {
				return nil, err
			}
			conversationInfo.Conversation = &conversationpb.Conversation{
				Name: conversationpb.ConversationName{
					CustomerID:     parent.CustomerID,
					ProfileID:      parent.ProfileID,
					ConversationID: scorecardDetails.ConversationID,
				}.String(),
				CreateTime: convoCreatedAt,
			}
		}
		if len(scorecardDetails.MessageIds) > 0 {
			for _, messageID := range scorecardDetails.MessageIds {
				conversationInfo.MessageNames = append(conversationInfo.MessageNames, conversationpb.ConversationMessageName{
					CustomerID:     parent.CustomerID,
					ProfileID:      parent.ProfileID,
					ConversationID: scorecardDetails.ConversationID,
					MessageID:      messageID,
				}.String())
			}
		}
		if scorecardDetails.ProcessID != "" {
			conversationInfo.ProcessId = scorecardDetails.ProcessID
		}
		if scorecardDetails.ProcessInteractionAt != "" {
			processInteractionTime, err := shared.ConvertStringToAPITimestamp(scorecardDetails.ProcessInteractionAt)
			if err != nil {
				return nil, err
			}
			conversationInfo.ProcessInteractionTime = processInteractionTime
		}
		stats.EvaluatedConversations = append(stats.EvaluatedConversations, &conversationInfo)
		stats.EvaluatedConversationCount += 1
		stats.AvgPerfScore = aggregateAvgPerfScore(stats.EvaluatedConversations)
		if scorecardDetails.AcknowledgedAt != "" {
			stats.AcknowledgedConversations = append(stats.AcknowledgedConversations, &conversationInfo)
		}
		if scorecardDetails.PublishedAt.Valid {
			stats.PublishedConversations = append(stats.PublishedConversations, &conversationInfo)
		}
	}

	// Makes sure to list all the audience users.
	for agnetUserID, agentUser := range templateAudienceUsers {
		if len(filteredAgentUsers) > 0 && filteredAgentUsers[agnetUserID] == nil {
			continue
		}
		if _, exist := agentUserIDMap[agnetUserID]; !exist {
			agentUserIDMap[agnetUserID] = &analyticspb.ManualQAStats{
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{agentUser},
				},
			}
		}
	}

	return agentUserIDMap, nil
}

func aggregateAgentUserFormattedSubmitTimeStats(
	parent *customerpb.CustomerOrProfileName,
	templateAudienceUsers map[string]*userpb.User,
	scorecardDetailsRows []rowScorecardDetails,
) (map[formattedSubmitTimeUserIDKey]*analyticspb.ManualQAStats, error) {
	formattedSubmitTimeAgentUserIDMap := make(map[formattedSubmitTimeUserIDKey]*analyticspb.ManualQAStats)
	for _, scorecardDetails := range scorecardDetailsRows {
		if _, exist := templateAudienceUsers[scorecardDetails.AgentUserID]; !exist {
			continue
		}

		// The sub time range stats.
		key := formattedSubmitTimeUserIDKey{FormattedSubmitTime: scorecardDetails.FormattedSubmitTime, userID: scorecardDetails.AgentUserID}
		if _, exist := formattedSubmitTimeAgentUserIDMap[key]; !exist {
			formattedSubmitAPITime, err := shared.ConvertStringToAPITimestamp(scorecardDetails.FormattedSubmitTime)
			if err != nil {
				return nil, err
			}
			formattedSubmitTimeAgentUserIDMap[key] = &analyticspb.ManualQAStats{
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{templateAudienceUsers[scorecardDetails.AgentUserID]},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: formattedSubmitAPITime,
						},
					},
				},
			}
		}

		stats := formattedSubmitTimeAgentUserIDMap[key]
		conversationInfo := analyticspb.ManualQAConversationInfo{
			ScorecardName: coachingpb.ScorecardName{
				CustomerID:  parent.CustomerID,
				ProfileID:   parent.ProfileID,
				ScorecardID: scorecardDetails.ScorecardID,
			}.String(),
			Agent: &userpb.User{
				Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.AgentUserID}.String(),
				DisplayName: scorecardDetails.AgentDisplayName,
			},
			QaAnalyst: &userpb.User{
				Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
				DisplayName: scorecardDetails.QaDisplayName,
			},
			PerfScore: scorecardDetails.Score,
		}
		if scorecardDetails.ConversationCreatedAt != "" {
			convoCreatedAt, err := shared.ConvertStringToAPITimestamp(scorecardDetails.ConversationCreatedAt)
			if err != nil {
				return nil, err
			}
			conversationInfo.Conversation = &conversationpb.Conversation{
				Name: conversationpb.ConversationName{
					CustomerID:     parent.CustomerID,
					ProfileID:      parent.ProfileID,
					ConversationID: scorecardDetails.ConversationID,
				}.String(),
				CreateTime: convoCreatedAt,
			}
		}
		if len(scorecardDetails.MessageIds) > 0 {
			for _, messageID := range scorecardDetails.MessageIds {
				conversationInfo.MessageNames = append(conversationInfo.MessageNames, conversationpb.ConversationMessageName{
					CustomerID:     parent.CustomerID,
					ProfileID:      parent.ProfileID,
					ConversationID: scorecardDetails.ConversationID,
					MessageID:      messageID,
				}.String())
			}
		}
		if scorecardDetails.ProcessID != "" {
			conversationInfo.ProcessId = scorecardDetails.ProcessID
		}
		if scorecardDetails.ProcessInteractionAt != "" {
			processInteractionTime, err := shared.ConvertStringToAPITimestamp(scorecardDetails.ProcessInteractionAt)
			if err != nil {
				return nil, err
			}
			conversationInfo.ProcessInteractionTime = processInteractionTime
		}
		stats.EvaluatedConversations = append(stats.EvaluatedConversations, &conversationInfo)
		stats.EvaluatedConversationCount += 1
		stats.AvgPerfScore = aggregateAvgPerfScore(stats.EvaluatedConversations)
		if scorecardDetails.AcknowledgedAt != "" {
			stats.AcknowledgedConversations = append(stats.AcknowledgedConversations, &conversationInfo)
		}
		if scorecardDetails.PublishedAt.Valid {
			stats.PublishedConversations = append(stats.PublishedConversations, &conversationInfo)
		}
	}
	return formattedSubmitTimeAgentUserIDMap, nil
}

func aggregateAgentManualQaStats(from *analyticspb.ManualQAStats, to *analyticspb.ManualQAStats) {
	to.EvaluatedConversations = append(to.EvaluatedConversations, from.EvaluatedConversations...)
	to.AcknowledgedConversations = append(to.AcknowledgedConversations, from.AcknowledgedConversations...)
	to.PublishedConversations = append(to.PublishedConversations, from.PublishedConversations...)
	to.AvgPerfScore = aggregateAvgPerfScore(to.EvaluatedConversations)
	to.EvaluatedConversationCount += from.EvaluatedConversationCount
}

func sortUserStats(userIDMap map[string]*analyticspb.ManualQAStats) []*analyticspb.ManualQAStats {
	manualQaStats := make([]*analyticspb.ManualQAStats, 0, len(userIDMap))
	userIDArray := fn.MapKeyToSlice(userIDMap)
	sort.Slice(userIDArray, func(i, j int) bool {
		return userIDArray[i] < userIDArray[j]
	})
	for _, userID := range userIDArray {
		stats := userIDMap[userID]
		sortManualQAStats(stats)
		manualQaStats = append(manualQaStats, stats)
	}
	return manualQaStats
}

func sortUserTimeStats(formattedSubmitTimeUserIDMap map[formattedSubmitTimeUserIDKey]*analyticspb.ManualQAStats) []*analyticspb.ManualQAStats {
	manualQaStats := make([]*analyticspb.ManualQAStats, 0, len(formattedSubmitTimeUserIDMap))
	formattedSubmitTimeUserIDArray := fn.MapKeyToSlice(formattedSubmitTimeUserIDMap)
	sort.Slice(formattedSubmitTimeUserIDArray, func(i, j int) bool {
		if formattedSubmitTimeUserIDArray[i].userID != formattedSubmitTimeUserIDArray[j].userID {
			return formattedSubmitTimeUserIDArray[i].userID < formattedSubmitTimeUserIDArray[j].userID
		}
		return formattedSubmitTimeUserIDArray[i].FormattedSubmitTime < formattedSubmitTimeUserIDArray[j].FormattedSubmitTime
	})
	for _, key := range formattedSubmitTimeUserIDArray {
		stats := formattedSubmitTimeUserIDMap[key]
		sortManualQAStats(stats)
		manualQaStats = append(manualQaStats, stats)
	}
	return manualQaStats
}

func sortManualQAStats(stats *analyticspb.ManualQAStats) {
	sort.Slice(stats.EvaluatedConversations, func(i, j int) bool {
		return stats.EvaluatedConversations[i].ScorecardName < stats.EvaluatedConversations[j].ScorecardName
	})
	sort.Slice(stats.CommentedConversations, func(i, j int) bool {
		return stats.CommentedConversations[i].ScorecardName < stats.CommentedConversations[j].ScorecardName
	})
	sort.Slice(stats.AcknowledgedConversations, func(i, j int) bool {
		return stats.AcknowledgedConversations[i].ScorecardName < stats.AcknowledgedConversations[j].ScorecardName
	})
	sort.Slice(stats.PublishedConversations, func(i, j int) bool {
		return stats.PublishedConversations[i].ScorecardName < stats.PublishedConversations[j].ScorecardName
	})
	sort.Slice(stats.CriteriaOverwrittenConversations, func(i, j int) bool {
		return stats.CriteriaOverwrittenConversations[i].ScorecardName < stats.CriteriaOverwrittenConversations[j].ScorecardName
	})
}

func getScorecardTemplate(db *gorm.DB, scorecardTemplateName *coachingpb.ScorecardTemplateName) (*dbmodel.ScorecardTemplates, error) {
	var scorecardTemplates *dbmodel.ScorecardTemplates
	conditions := condition.New(
		"customer = ? AND profile = ? AND resource_id = ?",
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
		scorecardTemplateName.ScorecardTemplateID,
	)
	if err := db.First(&scorecardTemplates, conditions...).Error; err != nil {
		return nil, err
	}
	if scorecardTemplates == nil {
		return nil, status.Errorf(codes.NotFound, "not found the scorecard template: %v", scorecardTemplateName.ScorecardTemplateID)
	}
	return scorecardTemplates, nil
}

func isProcessTemplate(scorecardTemplate *dbmodel.ScorecardTemplates) bool {
	return scorecardTemplate.Type == coachingpb.ScorecardTemplateType_SCORECARD_TEMPLATE_TYPE_PROCESS
}

func filterTemplateNoTask(scorecardTemplateName *coachingpb.ScorecardTemplateName, filterByAttribute *analyticspb.Attribute) bool {
	if len(filterByAttribute.ScorecardTemplateNamesWithNoTask) > 0 {
		return slices.Contains(filterByAttribute.ScorecardTemplateNamesWithNoTask, scorecardTemplateName.String())
	}
	return false
}

// Resolves users for a task by fetching its config for a given template.
// Falls back to the template's audience if config or audience is unset.
func ResolveTemplateTaskUsers(ctx context.Context, userClient userpb.UserServiceClient,
	parent *customerpb.CustomerOrProfileName, db *gorm.DB, scorecardTemplateName *coachingpb.ScorecardTemplateName, usecases []string, taskID string, template *dbmodel.ScorecardTemplates,
) (map[string]*userpb.User, error) {
	qaTaskConfigs, err := qa.ListScorecardTemplateQATaskConfigs(
		db,
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
		scorecardTemplateName.ScorecardTemplateID,
		usecases,
		taskID)
	if err != nil {
		return nil, err
	}

	var taskConfig *qadbpb.QATaskConfig
	if len(qaTaskConfigs) != 0 {
		taskConfig = qaTaskConfigs[0]
	}

	return qa.ResolveTaskUsersFromConfig(ctx, userClient, parent, template, taskConfig)
}

func calculateConsistencyScoresForScorecardDetails(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	scorecardDetailsRows *[]rowScorecardDetails,
) error {
	// Build calibration scorecards from rows
	calibrationScorecards := make([]*dbmodel.Scorecards, 0)
	calibrationScorecardIDToRowIndex := make(map[string]int)

	for i, row := range *scorecardDetailsRows {
		if row.Calibrated && row.CalibrationScorecardID != "" {
			// Build scorecard from row data
			scorecard := &dbmodel.Scorecards{
				Customer:         parent.CustomerID,
				Profile:          parent.ProfileID,
				ResourceID:       row.CalibrationScorecardID,
				SubmittedAt:      row.CalibrationScorecardSubmittedAt,
				ScorecardType:    row.CalibrationScorecardType,
				TemplateID:       row.CalibrationScorecardTemplateID,
				TemplateRevision: row.CalibrationScorecardTemplateRevision,
			}

			// Set ReferenceScorecardID for group calibration responses
			if row.CalibrationScorecardReferenceID.Valid {
				scorecard.ReferenceScorecardID = row.CalibrationScorecardReferenceID
			}

			// Set CalibratedScorecardID (points to original scorecard)
			scorecard.CalibratedScorecardID = sql.NullString{
				Valid:  true,
				String: row.ScorecardID,
			}

			calibrationScorecards = append(calibrationScorecards, scorecard)
			calibrationScorecardIDToRowIndex[row.CalibrationScorecardID] = i
		}
	}

	if len(calibrationScorecards) == 0 {
		return nil
	}

	// Get templates for calibration scorecards using the built scorecards
	scorecardToTemplate, err := qa.GetTemplatesForScorecards(db, parent.CustomerID, parent.ProfileID, calibrationScorecards)
	if err != nil {
		return err
	}

	// Calculate consistency scores
	profileName := &customerpb.ProfileName{
		CustomerID: parent.CustomerID,
		ProfileID:  parent.ProfileID,
	}
	consistencyScores, err := scoring.AddConsistencyScore(db, profileName, calibrationScorecards, scorecardToTemplate, nil)
	if err != nil {
		return err
	}

	// Update consistency scores in rows
	for calibrationScorecardID, consistencyScore := range consistencyScores {
		if rowIndex, ok := calibrationScorecardIDToRowIndex[calibrationScorecardID]; ok {
			(*scorecardDetailsRows)[rowIndex].ConsistencyScore = float32(consistencyScore.ConsistentScoreRootCriteria)
		}
	}

	return nil
}
