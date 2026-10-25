package analyticsimpl

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	qapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/qa"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/log"
	"github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	AnalystQuotaManager "github.com/cresta/go-servers/shared/analyst-quota-manager"
	coachingconverter "github.com/cresta/go-servers/shared/converters/coaching"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"

	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/utils"
)

type QMTaskStats struct {
	TemplateID                 string
	TaskID                     string
	EvaluatedScorecardCount    int32
	AssignedScorecardCount     int32
	AcknowledgedScorecardCount int32
	PublishedScorecardCount    int32
	AvgPerfScore               float32
	CompletionRate             float32
	ScoreSum                   float32
	ScoreCount                 int32
	StartTime                  time.Time
	EndTime                    time.Time
	PeriodType                 analyticspb.QMTaskStats_StatsPeriodType
	// Export-only fields — populated by computeExportStats when IncludeExportStats is true.
	// Defaults when disabled: 0 (counts), -1 (AvgConsistencyScore), UNSPECIFIED (EvaluationPeriodProgressStatus).
	OverwrittenCriteriaCount       int32                                      // default: 0
	CommentedScorecardCount        int32                                      // default: 0
	AvgConsistencyScore            float32                                    // default: -1 (N/A)
	CalibratedScorecardCount       int32                                      // default: 0
	SkipSuggestedConversationCount int32                                      // default: 0
	EvaluationPeriodProgressStatus analyticspb.EvaluationPeriodProgressStatus // default: UNSPECIFIED
}

type timeRangeStats struct {
	FormattedSubmitTime string
	EvaluatedCount      int32
	AcknowledgedCount   int32
	PublishedCount      int32
	ScoreSum            float32
	ScoreCount          int32
}

//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
type ScorecardRow struct {
	TemplateID     string
	TaskIDs        pq.StringArray `gorm:"column:task_ids;type:text[]"`
	SubmittedAt    time.Time
	AcknowledgedAt *time.Time
	PublishedAt    *time.Time
	Score          float32
	AssignedCount  int32
}

type QMTaskStatsFilterParams struct {
	RequestedUsecaseIDs       []string
	TemplateTasks             map[string]*TemplateTasksConfig
	FilteredAgentUsers        map[string]*userpb.User
	FilteredSubmissionSources []int32
	StartTime                 time.Time
	EndTime                   time.Time
	Frequency                 *analyticspb.Frequency
	Metadata                  *analyticspb.Metadata
	FilteredSubmitterIDs      []string
	IncludeExportStats        bool
}

type TemplateTasksConfig struct {
	TemplateID          string
	Tasks               []*dbmodel.Tasks
	ShouldIncludeNoTask bool
	IsProcessTemplate   bool
}

const (
	NumberQMTaskScorecardFetchConcurrentQuery = 10
	Day                                       = time.Hour * 24
)

func (a *AnalyticsServiceImpl) RetrieveQMTaskStats(ctx context.Context, req *analyticspb.RetrieveDirectorTaskStatsRequest) (*analyticspb.RetrieveDirectorTaskStatsResponse, error) {
	a.logger.Infof(ctx, "RetrieveQMTaskStats RPC starts, req = %v", req)

	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}

	err := validateRetrieveQMTaskStatsRequest(req)
	if err != nil {
		return nil, err
	}

	// Get the parent info
	parent, err := customerpb.ParseCustomerOrProfileName(req.Parent)
	if err != nil {
		return nil, err
	}
	requestedUsecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			requestedUsecaseIDs = append(requestedUsecaseIDs, usecaseName.GetUsecaseID())
		}
	}
	requestedTaskIDs := make([]string, 0, len(req.FilterByAttribute.TaskNames))
	for _, taskName := range req.FilterByAttribute.TaskNames {
		taskName, err := coachingpb.ParseDirectorTaskName(taskName)
		if err == nil {
			requestedTaskIDs = append(requestedTaskIDs, taskName.DirectorTaskID)
		}
	}
	requestedTemplateIDs := make([]string, 0, len(req.FilterByAttribute.ScorecardTemplateNames))
	for _, templateName := range req.FilterByAttribute.ScorecardTemplateNames {
		templateName, err := coachingpb.ParseScorecardTemplateName(templateName)
		if err == nil {
			requestedTemplateIDs = append(requestedTemplateIDs, templateName.ScorecardTemplateID)
		}
	}
	requestedTemplatesIDsWithNoTask := make([]string, 0, len(req.FilterByAttribute.ScorecardTemplateNamesWithNoTask))
	for _, templateName := range req.FilterByAttribute.ScorecardTemplateNamesWithNoTask {
		templateName, err := coachingpb.ParseScorecardTemplateName(templateName)
		if err == nil {
			requestedTemplatesIDsWithNoTask = append(requestedTemplatesIDsWithNoTask, templateName.ScorecardTemplateID)
		}
	}
	requestedTaskStatus := make([]coachingpb.DirectorTaskStatus, 0, len(req.TaskStatus))
	for _, status := range req.TaskStatus {
		requestedTaskStatus = append(requestedTaskStatus, coachingpb.DirectorTaskStatus(status))
	}
	fetchAllTemplateIDs := len(requestedTemplateIDs) == 0 && len(requestedTemplatesIDsWithNoTask) == 0
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	filteredAgentUsers, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
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

	startTime := req.FilterByTimeRange.StartTimestamp.AsTime()
	endTime := req.FilterByTimeRange.EndTimestamp.AsTime()

	db := a.appsDB.DB(ctx)

	// Map templates to tasks
	allTemplateIDs := make([]string, 0, len(requestedTemplateIDs)+len(requestedTemplatesIDsWithNoTask))
	allTemplateIDs = append(allTemplateIDs, requestedTemplateIDs...)
	allTemplateIDs = append(allTemplateIDs, requestedTemplatesIDsWithNoTask...)
	templates, err := qa.ListCurrentScorecardTemplateIDs(db, &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}, req.FilterByAttribute.UsecaseNames, allTemplateIDs, true)
	if err != nil {
		return nil, err
	}
	templateIDs := make([]string, 0, len(templates))
	for _, template := range templates {
		templateIDs = append(templateIDs, template.ResourceID)
	}
	tasks, err := getQMTasks(db, parent, requestedTaskIDs, requestedUsecaseIDs, requestedTaskStatus, templateIDs, fetchAllTemplateIDs)
	if err != nil {
		return nil, err
	}
	templateTasks := make(map[string]*TemplateTasksConfig)

	if len(templateIDs) > 0 {
		// If we are fetching all template IDs, we need to add stats for templates without tasks
		if fetchAllTemplateIDs {
			for _, templateID := range templateIDs {
				templateTasks[templateID] = &TemplateTasksConfig{
					TemplateID:          templateID,
					Tasks:               []*dbmodel.Tasks{},
					ShouldIncludeNoTask: true,
					IsProcessTemplate:   isProcessTemplate(templates[templateID]),
				}
			}
		} else {
			for _, templateID := range templateIDs {
				templateTasks[templateID] = &TemplateTasksConfig{
					TemplateID:          templateID,
					Tasks:               []*dbmodel.Tasks{},
					ShouldIncludeNoTask: false,
					IsProcessTemplate:   isProcessTemplate(templates[templateID]),
				}
			}
		}
	}

	if len(requestedTemplatesIDsWithNoTask) > 0 {
		for _, templateID := range requestedTemplatesIDsWithNoTask {
			if _, exists := templateTasks[templateID]; exists {
				templateTasks[templateID].ShouldIncludeNoTask = true
			}
		}
	}

	for _, task := range tasks {
		// Store tasks for each template
		if task.TaskContentConfig.Message != nil {
			if qmConfig := task.TaskContentConfig.Message.GetQmTaskContentConfig(); qmConfig != nil {
				// Only append if the template ID exists in the map
				if _, exists := templateTasks[qmConfig.ScorecardTemplateId]; exists {
					templateTasks[qmConfig.ScorecardTemplateId].Tasks = append(templateTasks[qmConfig.ScorecardTemplateId].Tasks, task)
				}
			}
		}
	}

	qmTaskStatsFilterParams := &QMTaskStatsFilterParams{
		RequestedUsecaseIDs:       requestedUsecaseIDs,
		TemplateTasks:             templateTasks,
		FilteredAgentUsers:        filteredAgentUsers,
		FilteredSubmissionSources: filteredSubmissionSources,
		StartTime:                 startTime,
		EndTime:                   endTime,
		Frequency:                 &req.Frequency,
		Metadata:                  req.Metadata,
		FilteredSubmitterIDs:      filteredSubmitterIDs,
		IncludeExportStats:        req.IncludeExportStats,
	}

	switch {
	case len(req.GroupByAttributes) == 2 &&
		req.GroupByAttributes[0] == analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK &&
		req.GroupByAttributes[1] == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
		return a.groupByTaskAndPeriod(ctx, db, parent, qmTaskStatsFilterParams)
	case len(req.GroupByAttributes) == 1 &&
		req.GroupByAttributes[0] == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
		return a.groupByTimeRangeOnly(ctx, db, parent, qmTaskStatsFilterParams)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid group_by_attributes: %v", req.GroupByAttributes)
	}
}

func getQMTasks(db *gorm.DB, parent *customerpb.CustomerOrProfileName, requestTaskIDs []string, requestUsecaseIDs []string, requestTaskStatus []coachingpb.DirectorTaskStatus, templateIDs []string, fetchAllTemplateIDs bool) ([]*dbmodel.Tasks, error) {
	var tasks []*dbmodel.Tasks
	if len(requestTaskIDs) == 0 && !fetchAllTemplateIDs {
		return tasks, nil
	}
	query := db.Where("customer_id = ? AND profile_id = ?", parent.CustomerID, parent.ProfileID).
		Where("task_type = ?", coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM)

	if len(requestTaskStatus) > 0 {
		query = query.Where("task_status IN ?", requestTaskStatus)
	}

	if len(requestUsecaseIDs) > 0 {
		query = query.Where("usecase_id IN ?", requestUsecaseIDs)
	}

	if len(templateIDs) > 0 {
		query = query.Where("task_content_config->'qmTaskContentConfig'->>'scorecardTemplateId' IN ?", templateIDs)
	}

	if len(requestTaskIDs) > 0 {
		query = query.Where("task_id IN ?", requestTaskIDs)
	}

	err := query.Find(&tasks).Error
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func fetchScorecardsForPeriod(
	ctx context.Context,
	logger log.Logger,
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	taskID string,
	templateID string,
	startTime time.Time,
	endTime time.Time,
	filteredAgentUsers map[string]*userpb.User,
	filteredSubmissionSources []int32,
	filteredSubmitterIDs []string,
) ([]ScorecardRow, error) {
	var scorecardRows []ScorecardRow
	query := db.Model(&dbmodel.Scorecards{}).
		Where("customer = ? AND profile = ?", parent.CustomerID, parent.ProfileID).
		Where("template_id = ?", templateID).
		Where("calibrated_scorecard_id IS NULL").
		Where("scorecard_type IS NULL OR scorecard_type = 0").
		Select("template_id, task_ids, submitted_at, acknowledged_at, published_at, COALESCE(score, -1.0) as score")

	query = query.Where("submitted_at >= ? AND submitted_at <= ?", startTime, endTime)

	// If taskID is empty, fetch scorecards where task_ids is NULL or empty, or '__no__task__'
	if taskID == "" {
		query = query.Where("array_length(task_ids, 1) IS NULL OR ? = ANY(task_ids)", shared.NoTask)
	} else {
		query = query.Where("? = ANY(task_ids)", taskID)
	}

	if len(filteredAgentUsers) > 0 {
		query = query.Where("scorecards.agent_user_id IN ?", fn.MapKeyToSlice(filteredAgentUsers))
	}

	if len(filteredSubmissionSources) > 0 {
		query = query.Where("scorecards.submission_source in ?", filteredSubmissionSources)
	}

	if len(filteredSubmitterIDs) > 0 {
		query = query.Where("scorecards.submitter_user_id in ?", filteredSubmitterIDs)
	}

	err := query.Scan(&scorecardRows).Error
	if err != nil {
		logger.Errorf(ctx, "Error fetching scorecards for task %s, period %v-%v: %v", taskID, startTime, endTime, err)
		return nil, err
	}

	return scorecardRows, nil
}

func fetchScorecardsCountGroupedByFrequency(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	templateTasks map[string]*TemplateTasksConfig,
	filteredAgentUsers map[string]*userpb.User,
	filteredSubmissionSources []int32,
	startTime time.Time,
	endTime time.Time,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filteredSubmitterIDs []string,
) ([]timeRangeStats, error) {
	var timeRangeStats []timeRangeStats
	if len(templateTasks) == 0 {
		return timeRangeStats, nil
	}
	args, err := postgres.ParseArgs(frequency, metadata)
	if err != nil {
		return nil, err
	}
	query := db.Model(&dbmodel.Scorecards{}).
		Select(`DATE_TRUNC(@frequency,scorecards.submitted_at AT TIME ZONE @time_zone 
			+ INTERVAL '1 day' * @first_day_of_week_bias) AT TIME ZONE @time_zone 
			- INTERVAL '1 day' * @first_day_of_week_bias AS formatted_submit_time,
			COUNT(*) as evaluated_count,
			COUNT(CASE WHEN acknowledged_at IS NOT NULL THEN 1 END) as acknowledged_count,
			COUNT(CASE WHEN published_at IS NOT NULL THEN 1 END) as published_count,
			SUM(CASE WHEN score >= 0 THEN score END) as score_sum,
			COUNT(CASE WHEN score >= 0 THEN 1 END) as score_count`, args...).
		Where("submitted_at >= ? AND submitted_at <= ?", startTime, endTime).
		Where("customer = ?", parent.CustomerID).
		Where("profile = ?", parent.ProfileID).
		Where("calibrated_scorecard_id IS NULL").
		Where("scorecard_type IS NULL OR scorecard_type = 0").
		Where("manually_scored = TRUE")

	var orConditions []string
	var orArgs []interface{}
	for templateID, templateTasks := range templateTasks {
		taskIDs := make([]string, 0)
		for _, task := range templateTasks.Tasks {
			taskIDs = append(taskIDs, task.TaskID)
		}
		if len(taskIDs) == 0 || templateTasks.ShouldIncludeNoTask {
			orConditions = append(orConditions, "(template_id = ? AND (array_length(task_ids, 1) IS NULL OR ? = ANY(task_ids)))")
			orArgs = append(orArgs, templateID, shared.NoTask)
		}
		if len(taskIDs) > 0 {
			orConditions = append(orConditions, "(template_id = ? AND task_ids && ?)")
			orArgs = append(orArgs, templateID, pq.Array(taskIDs))
		}
	}
	if len(orConditions) > 0 {
		query = query.Where(strings.Join(orConditions, " OR "), orArgs...)
	}

	if len(filteredAgentUsers) > 0 {
		query = query.Where("agent_user_id IN ?", fn.MapKeyToSlice(filteredAgentUsers))
	}

	if len(filteredSubmissionSources) > 0 {
		query = query.Where("submission_source in ?", filteredSubmissionSources)
	}

	if len(filteredSubmitterIDs) > 0 {
		query = query.Where("submitter_user_id in ?", filteredSubmitterIDs)
	}

	// Add time grouping parameters
	query = query.Group("formatted_submit_time").
		Order("formatted_submit_time")

	err = query.Scan(&timeRangeStats).Error
	if err != nil {
		return nil, err
	}

	return timeRangeStats, nil
}

func fetchScorecardsCountForTimeRange(
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	templateID string,
	taskIDs []string,
	usecaseIDs []string,
	filteredAgentUsers map[string]*userpb.User,
	filteredSubmissionSources []int32,
	startTime time.Time,
	endTime time.Time,
	filteredSubmitterIDs []string,
) (timeRangeStats, error) {
	var timeRangeStats timeRangeStats
	query := db.Model(&dbmodel.Scorecards{}).
		Select(`
			COUNT(*) as evaluated_count,
			COUNT(CASE WHEN acknowledged_at IS NOT NULL THEN 1 END) as acknowledged_count,
			COUNT(CASE WHEN published_at IS NOT NULL THEN 1 END) as published_count,
			SUM(CASE WHEN score >= 0 THEN score END) as score_sum,
			COUNT(CASE WHEN score >= 0 THEN 1 END) as score_count`).
		Where("submitted_at >= ? AND submitted_at <= ?", startTime, endTime).
		Where("customer = ?", parent.CustomerID).
		Where("profile = ?", parent.ProfileID).
		Where("template_id = ?", templateID).
		Where("calibrated_scorecard_id IS NULL").
		Where("scorecard_type IS NULL OR scorecard_type = 0")

	if len(taskIDs) > 0 {
		query = query.Where("task_ids && ?", pq.Array(taskIDs))
	} else {
		query = query.Where("array_length(task_ids, 1) IS NULL OR ? = ANY(task_ids)", shared.NoTask)
	}

	if len(usecaseIDs) > 0 {
		query = query.Where("usecase_id IN ?", usecaseIDs)
	}

	if len(filteredAgentUsers) > 0 {
		query = query.Where("agent_user_id IN ?", fn.MapKeyToSlice(filteredAgentUsers))
	}

	if len(filteredSubmissionSources) > 0 {
		query = query.Where("submission_source in ?", filteredSubmissionSources)
	}

	if len(filteredSubmitterIDs) > 0 {
		query = query.Where("submitter_user_id in ?", filteredSubmitterIDs)
	}

	err := query.Scan(&timeRangeStats).Error
	if err != nil {
		return timeRangeStats, err
	}

	return timeRangeStats, nil
}

func (a *AnalyticsServiceImpl) groupByTaskAndPeriod(ctx context.Context, db *gorm.DB, parent *customerpb.CustomerOrProfileName, qmTaskStatsFilterParams *QMTaskStatsFilterParams) (*analyticspb.RetrieveDirectorTaskStatsResponse, error) {
	taskAssignedCounts := make(map[string]int32)

	taskPeriods := make(map[string][]*qa.EvaluationPeriod)
	// Agents are resolved before any worker starts, so a failure here cannot leave workers blocked on the channels.
	coverageTaskTemplates := make(map[string]*dbmodel.ScorecardTemplates)
	coverageTaskUsers := make(map[string]map[string]*userpb.User)
	for templateID, templateTasks := range qmTaskStatsFilterParams.TemplateTasks {
		for _, task := range templateTasks.Tasks {
			if task.TaskScheduleConfig.Message != nil {
				var periods []*qa.EvaluationPeriod
				var err error
				periods, err = qa.ListEvaluationPeriodsForTask(db, parent.CustomerID, parent.ProfileID, qmTaskStatsFilterParams.RequestedUsecaseIDs, task.TaskID)
				if err != nil {
					return nil, err
				}
				taskPeriods[task.TaskID] = periods

				// Matches the fixed-count quota below, which is only computed without an agent filter.
				if len(qmTaskStatsFilterParams.FilteredAgentUsers) == 0 && slices.ContainsFunc(periods, isConversationCoveragePeriod) {
					template, taskUsers, err := a.resolveTaskAgents(ctx, db, parent, templateID, periods[0].QATaskConfig)
					if err != nil {
						return nil, err
					}
					coverageTaskTemplates[task.TaskID] = template
					coverageTaskUsers[task.TaskID] = taskUsers
				}
			}
		}
	}

	resultChan := make(chan QMTaskStats, NumberQMTaskScorecardFetchConcurrentQuery)
	errorChan := make(chan error, NumberQMTaskScorecardFetchConcurrentQuery)

	// guard channel to limit the number of concurrent workers
	guard := make(chan struct{}, NumberQMTaskScorecardFetchConcurrentQuery)
	var wg sync.WaitGroup

	// Fetch and process scorecards for each task and period in parallel
	for templateID, templateTasks := range qmTaskStatsFilterParams.TemplateTasks {
		if len(templateTasks.Tasks) == 0 || templateTasks.ShouldIncludeNoTask {
			// Handle templates without tasks
			qmTaskStats := &QMTaskStats{
				TemplateID:                 templateID,
				TaskID:                     "", // No task ID for templates without tasks
				EvaluatedScorecardCount:    0,
				AssignedScorecardCount:     0, // No assigned count for templates without tasks
				AcknowledgedScorecardCount: 0,
				PublishedScorecardCount:    0,
				AvgPerfScore:               0,
				CompletionRate:             -1, // No completion rate for templates without tasks
				StartTime:                  qmTaskStatsFilterParams.StartTime,
				EndTime:                    qmTaskStatsFilterParams.EndTime,
			}

			wg.Add(1)
			go utils.AvoidPanic(ctx, func() {
				defer wg.Done()
				guard <- struct{}{}
				defer func() { <-guard }()
				// Query scorecards count for time range
				timeRangeStats, err := fetchScorecardsCountForTimeRange(db, parent, templateID, nil, qmTaskStatsFilterParams.RequestedUsecaseIDs, qmTaskStatsFilterParams.FilteredAgentUsers, qmTaskStatsFilterParams.FilteredSubmissionSources, qmTaskStatsFilterParams.StartTime, qmTaskStatsFilterParams.EndTime, qmTaskStatsFilterParams.FilteredSubmitterIDs)
				if err != nil {
					errorChan <- err
					return
				}
				qmTaskStats.EvaluatedScorecardCount = timeRangeStats.EvaluatedCount
				qmTaskStats.AcknowledgedScorecardCount = timeRangeStats.AcknowledgedCount
				qmTaskStats.PublishedScorecardCount = timeRangeStats.PublishedCount
				qmTaskStats.ScoreSum = timeRangeStats.ScoreSum
				qmTaskStats.ScoreCount = timeRangeStats.ScoreCount
				qmTaskStats.AvgPerfScore = calculateAvgPerfScore(timeRangeStats.ScoreSum, timeRangeStats.ScoreCount)
				qmTaskStats.AvgConsistencyScore = -1.0 // -1 signals N/A; overwritten by computeExportStats when enabled

				if qmTaskStatsFilterParams.IncludeExportStats {
					err = computeExportStats(ctx, a.logger, db, parent, qmTaskStats, qmTaskStatsFilterParams.FilteredAgentUsers, qmTaskStatsFilterParams.FilteredSubmissionSources, qmTaskStatsFilterParams.FilteredSubmitterIDs, qmTaskStatsFilterParams.RequestedUsecaseIDs, templateTasks.IsProcessTemplate, qmTaskStatsFilterParams.Metadata)
					if err != nil {
						errorChan <- err
						return
					}
				}

				resultChan <- *qmTaskStats
			})()
		}
		for _, task := range templateTasks.Tasks {
			// Calculate assigned counts
			if len(qmTaskStatsFilterParams.FilteredAgentUsers) == 0 && task.TaskAudienceConfig.Message != nil && task.TaskAudienceConfig.Message.QaAnalystQuotaConfig != nil {
				qaAnalystQuotas := coachingconverter.DBToAPIQAAnalystQuota(task.TaskAudienceConfig.Message.QaAnalystQuotaConfig.GetQaAnalystQuotas())

				analystQuotaManager, err := AnalystQuotaManager.NewAnalystQuotaManager(ctx, parent, a.authUserServiceClient, []*qapb.QAAnalystQuotaConfig{{
					QaAnalystQuotas: qaAnalystQuotas,
				}}, nil)
				if err != nil {
					return nil, err
				}

				taskAssignedCounts[task.TaskID] = analystQuotaManager.GetTotalQuota(nil)
			}

			// Fetch scorecards for each evaluation period per task in parallel
			scheduleConfig := task.TaskScheduleConfig.Message.GetQmTaskScheduleConfig()
			if scheduleConfig != nil {
				periods := taskPeriods[task.TaskID]
				template := coverageTaskTemplates[task.TaskID]
				taskUsers, countCoverage := coverageTaskUsers[task.TaskID]
				previousPeriodEndTime := time.Time{}
				for _, period := range periods {
					periodType := analyticspb.QMTaskStats_REGULAR_PERIOD
					periodStartTime := period.ScorecardTemplateEvaluationPeriod.StartTime.AsTime()
					periodEndTime := period.ScorecardTemplateEvaluationPeriod.EndTime.AsTime()
					// Skip if the period does not overlap with the time range
					if periodEndTime.Before(qmTaskStatsFilterParams.StartTime) || periodStartTime.After(qmTaskStatsFilterParams.EndTime) {
						continue
					}
					// Mark partial period if the period starts before the selected start time
					if periodStartTime.Before(qmTaskStatsFilterParams.StartTime) {
						periodType = analyticspb.QMTaskStats_PARTIAL_PERIOD
						// Set the period start time to the selected start time
						periodStartTime = qmTaskStatsFilterParams.StartTime.Truncate(time.Second)
						// Adjust the period end time to the closest local time before the period end date
						periodEndTime = getClosestLocalTimeBeforePivot(qmTaskStatsFilterParams.EndTime, periodEndTime)
					}
					// Mark partial period if the period ends after the selected end time
					if periodEndTime.After(qmTaskStatsFilterParams.EndTime) {
						location, err := time.LoadLocation(qmTaskStatsFilterParams.Metadata.TimeZoneId)
						if err != nil {
							return nil, err
						}
						// mark it as partial and set the end time to the selected end time if it is not an ongoing period
						currentLocalTime := time.Now().In(location)
						year, month, day := currentLocalTime.Date()
						startOfDay := time.Date(year, month, day, 0, 0, 0, 0, currentLocalTime.Location())
						if qmTaskStatsFilterParams.EndTime.In(location).Before(startOfDay) {
							periodType = analyticspb.QMTaskStats_PARTIAL_PERIOD
							// Set the period end time to the selected end time
							periodEndTime = qmTaskStatsFilterParams.EndTime.Truncate(time.Second)
							if !previousPeriodEndTime.IsZero() {
								previousPeriodEndTime = previousPeriodEndTime.Add(1 * time.Second)
								// Adjust the period start time to the closest local time after the previous period end date
								periodStartTime = getClosestLocalTimeAfterPivot(qmTaskStatsFilterParams.StartTime, previousPeriodEndTime)
							}
							// Adjust the period start time to the closest local time after the period start date if the previous period end time is not set
							periodStartTime = getClosestLocalTimeAfterPivot(qmTaskStatsFilterParams.StartTime, periodStartTime)
						} else {
							periodType = analyticspb.QMTaskStats_ONGOING_PERIOD
						}
					}

					// Skip if the gap is less than a day
					if periodEndTime.Sub(periodStartTime).Abs() < Day-1*time.Second {
						continue
					}

					// Initialize stats for this period
					qmTaskStats := &QMTaskStats{
						TemplateID:                 templateID,
						TaskID:                     task.TaskID,
						EvaluatedScorecardCount:    0,
						AssignedScorecardCount:     taskAssignedCounts[task.TaskID],
						AcknowledgedScorecardCount: 0,
						PublishedScorecardCount:    0,
						AvgPerfScore:               0,
						CompletionRate:             -1,
						StartTime:                  periodStartTime,
						EndTime:                    periodEndTime,
						PeriodType:                 periodType,
					}

					previousPeriodEndTime = periodEndTime

					// If the period is partial, set the assigned scorecard count to -1 (represent N/A)
					if periodType == analyticspb.QMTaskStats_PARTIAL_PERIOD {
						qmTaskStats.AssignedScorecardCount = -1
					}

					countPeriodCoverage := countCoverage && isConversationCoveragePeriod(period) && periodType != analyticspb.QMTaskStats_PARTIAL_PERIOD

					// Start goroutine to fetch scorecards
					wg.Add(1)
					go utils.AvoidPanic(ctx, func() {
						defer wg.Done()
						guard <- struct{}{}
						defer func() { <-guard }()
						if countPeriodCoverage {
							coverageCounts, err := a.countCoverageConversationsForPeriods(ctx, parent, template, []*qa.EvaluationPeriod{period}, taskUsers, fn.MapKeyToSlice(taskUsers))
							if err != nil {
								errorChan <- err
								return
							}
							qmTaskStats.AssignedScorecardCount = coverageCounts[0]
						}
						err := processScorecards(
							ctx,
							a.logger,
							db,
							parent,
							qmTaskStats,
							qmTaskStatsFilterParams.FilteredAgentUsers,
							qmTaskStatsFilterParams.FilteredSubmissionSources,
							resultChan,
							qmTaskStatsFilterParams.FilteredSubmitterIDs,
							qmTaskStatsFilterParams.RequestedUsecaseIDs,
							qmTaskStatsFilterParams.IncludeExportStats,
							templateTasks.IsProcessTemplate,
							qmTaskStatsFilterParams.Metadata,
						)
						if err != nil {
							a.logger.Errorf(ctx, "Failed to process scorecards: %v", err)
							errorChan <- err
							return
						}
					})()
				}
			}
		}
	}

	go utils.AvoidPanic(ctx, func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	})()

	directorTaskStats := []*analyticspb.DirectorTaskStats{}
	var errorOccurred error

	// start consuming the channels so the workers won't get blocked
	for resultChan != nil || errorChan != nil {
		// the select will never enter a case with a nil channel
		select {
		case err, ok := <-errorChan:
			// means the channel has been emptied and closed
			if !ok {
				errorChan = nil
				continue
			}
			// save the first error, but still keep consuming the channel
			if err != nil && errorOccurred == nil {
				errorOccurred = err
			}

		case qmTaskStats, ok := <-resultChan:
			// empty and closed
			if !ok {
				resultChan = nil
				continue
			}
			// if an error has Occurred, skip building the result
			if errorOccurred != nil {
				continue
			}
			directorTaskStat := analyticspb.DirectorTaskStats{
				Attribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:          parent.CustomerID,
						ProfileID:           parent.ProfileID,
						ScorecardTemplateID: qmTaskStats.TemplateID,
					}.String()},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(qmTaskStats.StartTime),
							EndTimestamp:   timestamppb.New(qmTaskStats.EndTime),
						},
					},
				},
				QmTaskStats: &analyticspb.QMTaskStats{
					EvalueatedScrecardCount:        qmTaskStats.EvaluatedScorecardCount,
					AssignedScorecardCount:         qmTaskStats.AssignedScorecardCount,
					AcknowledgedScorecardCount:     qmTaskStats.AcknowledgedScorecardCount,
					PublishedScorecardCount:        qmTaskStats.PublishedScorecardCount,
					AvgPerfScore:                   qmTaskStats.AvgPerfScore,
					CompletionRate:                 qmTaskStats.CompletionRate,
					StatsPeriodType:                qmTaskStats.PeriodType,
					OverwrittenCriteriaCount:       qmTaskStats.OverwrittenCriteriaCount,
					CommentedScorecardCount:        qmTaskStats.CommentedScorecardCount,
					AvgConsistencyScore:            qmTaskStats.AvgConsistencyScore,
					CalibratedScorecardCount:       qmTaskStats.CalibratedScorecardCount,
					SkipSuggestedConversationCount: qmTaskStats.SkipSuggestedConversationCount,
					EvaluationPeriodProgressStatus: qmTaskStats.EvaluationPeriodProgressStatus,
				},
			}
			if qmTaskStats.TaskID != "" {
				directorTaskStat.Attribute.TaskNames = []string{coachingpb.DirectorTaskName{
					CustomerID:     parent.CustomerID,
					ProfileID:      parent.ProfileID,
					DirectorTaskID: qmTaskStats.TaskID,
				}.String()}
			}
			directorTaskStats = append(directorTaskStats, &directorTaskStat)
		}
	}

	if errorOccurred != nil {
		return nil, errorOccurred
	}

	return &analyticspb.RetrieveDirectorTaskStatsResponse{
		DirectorTaskStats: directorTaskStats,
	}, nil
}

func processScorecards(ctx context.Context, logger log.Logger, db *gorm.DB, parent *customerpb.CustomerOrProfileName, qmTaskStats *QMTaskStats, filteredAgentUsers map[string]*userpb.User, filteredSubmissionSources []int32, resultChan chan<- QMTaskStats, filteredSubmitterIDs []string, usecaseIDs []string, includeExportStats bool, isProcessTemplate bool, metadata *analyticspb.Metadata) error {
	scorecardRows, err := fetchScorecardsForPeriod(
		ctx,
		logger,
		db,
		parent,
		qmTaskStats.TaskID,
		qmTaskStats.TemplateID,
		qmTaskStats.StartTime,
		qmTaskStats.EndTime,
		filteredAgentUsers,
		filteredSubmissionSources,
		filteredSubmitterIDs,
	)
	if err != nil {
		return err
	}
	scoreSum := float32(0.0)
	scoreCount := int32(0)
	for _, row := range scorecardRows {
		if row.Score >= 0 {
			scoreSum += row.Score
			scoreCount += 1
		}
		if row.AcknowledgedAt != nil {
			qmTaskStats.AcknowledgedScorecardCount += 1
		}
		if row.PublishedAt != nil {
			qmTaskStats.PublishedScorecardCount += 1
		}
	}
	qmTaskStats.ScoreSum = scoreSum
	qmTaskStats.ScoreCount = scoreCount
	qmTaskStats.AvgPerfScore = calculateAvgPerfScore(scoreSum, scoreCount)
	qmTaskStats.EvaluatedScorecardCount = int32(len(scorecardRows))

	// Calculate completion rate based on analyst quota only when there are no filtered agents
	if qmTaskStats.AssignedScorecardCount > 0 {
		qmTaskStats.CompletionRate = safeDivideFloat(qmTaskStats.EvaluatedScorecardCount, qmTaskStats.AssignedScorecardCount)
	} else {
		qmTaskStats.CompletionRate = -1.0
	}

	qmTaskStats.AvgConsistencyScore = -1.0 // -1 signals N/A; overwritten by computeExportStats when enabled

	// Compute export-only metrics only if requested. Must be after CompletionRate
	// is set, since EvaluationPeriodProgressStatus depends on it.
	if includeExportStats {
		err = computeExportStats(ctx, logger, db, parent, qmTaskStats, filteredAgentUsers, filteredSubmissionSources, filteredSubmitterIDs, usecaseIDs, isProcessTemplate, metadata)
		if err != nil {
			return err
		}
	}

	resultChan <- *qmTaskStats
	return nil
}

func computeExportStats(ctx context.Context, logger log.Logger, db *gorm.DB, parent *customerpb.CustomerOrProfileName, qmTaskStats *QMTaskStats, filteredAgentUsers map[string]*userpb.User, filteredSubmissionSources []int32, filteredSubmitterIDs []string, usecaseIDs []string, isProcessTemplate bool, metadata *analyticspb.Metadata) error {
	taskName := (*coachingpb.DirectorTaskName)(nil)
	filterTemplateNoTask := false
	if qmTaskStats.TaskID != "" {
		taskName = &coachingpb.DirectorTaskName{
			CustomerID:     parent.CustomerID,
			ProfileID:      parent.ProfileID,
			DirectorTaskID: qmTaskStats.TaskID,
		}
	} else {
		filterTemplateNoTask = true
	}

	frequency := analyticspb.Frequency_EVALUATION_PERIOD
	scorecardDetailsRows, err := queryScorecardDetails(
		db,
		parent,
		qmTaskStats.StartTime,
		qmTaskStats.EndTime,
		qmTaskStats.TemplateID,
		usecaseIDs,
		&frequency,
		metadata,
		"", // filteredQaUserID
		filteredAgentUsers,
		filteredSubmissionSources,
		isProcessTemplate,
		filterTemplateNoTask,
		taskName,
		filteredSubmitterIDs,
	)
	if err != nil {
		logger.Errorf(ctx, "Failed to query scorecard details: %v", err)
		return err
	}

	err = calculateConsistencyScoresForScorecardDetails(db, parent, &scorecardDetailsRows)
	if err != nil {
		logger.Errorf(ctx, "Failed to calculate consistency scores: %v", err)
		return err
	}

	scorecardCriteriaOverwritten, err := queryScorecardDetailsCriteriaOverwrittenCount(
		ctx,
		logger,
		db,
		parent,
		qmTaskStats.StartTime,
		qmTaskStats.EndTime,
		qmTaskStats.TemplateID,
		usecaseIDs,
		"", // filteredQaUserID
		filteredAgentUsers,
		filteredSubmissionSources,
		filterTemplateNoTask,
		taskName,
		filteredSubmitterIDs,
	)
	if err != nil {
		logger.Errorf(ctx, "Failed to query criteria overwritten count: %v", err)
		return err
	}

	scorecardCriteriaOverwrittenMap := make(map[string]int)
	for _, row := range scorecardCriteriaOverwritten {
		scorecardCriteriaOverwrittenMap[row.ScorecardID] = row.NumCriteriaOverwritten
	}

	commentedScorecardCount := int32(0)
	overwrittenCriteriaCount := int32(0)
	calibratedScorecardCount := int32(0)
	consistencyScoreSum := float32(0.0)
	consistencyScoreCount := int32(0)

	for _, row := range scorecardDetailsRows {
		if row.Comment > 0 {
			commentedScorecardCount += 1
		}
		if numOverwritten, exists := scorecardCriteriaOverwrittenMap[row.ScorecardID]; exists {
			overwrittenCriteriaCount += int32(numOverwritten)
		}
		if row.Calibrated {
			calibratedScorecardCount += 1
			if row.ConsistencyScore >= 0 {
				consistencyScoreSum += row.ConsistencyScore
				consistencyScoreCount += 1
			}
		}
	}

	qmTaskStats.CommentedScorecardCount = commentedScorecardCount
	qmTaskStats.OverwrittenCriteriaCount = overwrittenCriteriaCount
	qmTaskStats.CalibratedScorecardCount = calibratedScorecardCount
	if consistencyScoreCount > 0 {
		qmTaskStats.AvgConsistencyScore = consistencyScoreSum / float32(consistencyScoreCount)
	} else {
		qmTaskStats.AvgConsistencyScore = -1.0
	}

	sessionLogRows, err := querySessionLogDetails(db, parent, qmTaskStats.StartTime, qmTaskStats.EndTime, qmTaskStats.TemplateID, &frequency, nil, "", filteredAgentUsers)
	if err != nil {
		logger.Errorf(ctx, "Failed to query session log details: %v", err)
		return err
	}
	// Match manual QA: only count skips for QA users who submitted a scorecard in this period.
	qaUserIDsWithScorecards := set.New[string]()
	for _, row := range scorecardDetailsRows {
		qaUserIDsWithScorecards.Add(row.QaUserID)
	}
	var skipSuggestedConversationCount int32
	for _, sessionLog := range sessionLogRows {
		if qaUserIDsWithScorecards.Has(sessionLog.UserID) {
			skipSuggestedConversationCount++
		}
	}
	qmTaskStats.SkipSuggestedConversationCount = skipSuggestedConversationCount

	if isInTimeRange(time.Now(), qmTaskStats.StartTime, qmTaskStats.EndTime) && qmTaskStats.CompletionRate >= 0 {
		qmTaskStats.EvaluationPeriodProgressStatus = computeEvaluationPeriodStatus(
			time.Now(), qmTaskStats.CompletionRate, qmTaskStats.StartTime, qmTaskStats.EndTime,
		)
	}

	return nil
}

func validateRetrieveQMTaskStatsRequest(req *analyticspb.RetrieveDirectorTaskStatsRequest) error {
	if req.FilterByTimeRange == nil {
		return status.Errorf(codes.InvalidArgument, "filter_by_time_range is required.")
	}
	if len(req.GroupByAttributes) == 0 {
		return status.Errorf(codes.InvalidArgument, "group_by_attributes is required.")
	}
	return nil
}

func calculateAvgPerfScore(scoreSum float32, scoreCount int32) float32 {
	if scoreCount > 0 {
		return scoreSum / float32(scoreCount)
	}
	return -1.0
}

func (a *AnalyticsServiceImpl) groupByTimeRangeOnly(
	ctx context.Context,
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	qmTaskStatsFilterParams *QMTaskStatsFilterParams,
) (*analyticspb.RetrieveDirectorTaskStatsResponse, error) {
	// Calculate total assigned count across all tasks
	totalAssignedCount := int32(0)
	for _, templateTasks := range qmTaskStatsFilterParams.TemplateTasks {
		for _, task := range templateTasks.Tasks {
			if len(qmTaskStatsFilterParams.FilteredAgentUsers) == 0 && task.TaskAudienceConfig.Message != nil && task.TaskAudienceConfig.Message.QaAnalystQuotaConfig != nil {
				for _, quota := range task.TaskAudienceConfig.Message.QaAnalystQuotaConfig.QaAnalystQuotas {
					totalAssignedCount += quota.AssignedConversationCount
				}
			}
		}
	}

	// Query scorecards grouped by time rangekIDs, qmTaskStats
	timeRangeStats, err := fetchScorecardsCountGroupedByFrequency(db, parent, qmTaskStatsFilterParams.TemplateTasks, qmTaskStatsFilterParams.FilteredAgentUsers, qmTaskStatsFilterParams.FilteredSubmissionSources, qmTaskStatsFilterParams.StartTime, qmTaskStatsFilterParams.EndTime, qmTaskStatsFilterParams.Frequency, qmTaskStatsFilterParams.Metadata, qmTaskStatsFilterParams.FilteredSubmitterIDs)
	if err != nil {
		return nil, err
	}

	// Build response
	directorTaskStats := make([]*analyticspb.DirectorTaskStats, len(timeRangeStats))
	totalQMTaskStats := &analyticspb.QMTaskStats{
		AssignedScorecardCount: 0,  // No assigned scorecards for time range only
		CompletionRate:         -1, // No completion rate for time range only
	}
	totalScoreSum := float32(0.0)
	totalScoreCount := int32(0)

	for i, stats := range timeRangeStats {
		parsedTime, err := time.Parse(time.RFC3339, stats.FormattedSubmitTime)
		if err != nil {
			return nil, err
		}
		formattedSubmitTime := timestamppb.New(parsedTime)

		directorTaskStats[i] = &analyticspb.DirectorTaskStats{
			Attribute: &analyticspb.Attribute{
				TimeRanges: []*analyticspb.TimeRange{
					{
						StartTimestamp: formattedSubmitTime,
					},
				},
			},
			QmTaskStats: &analyticspb.QMTaskStats{
				EvalueatedScrecardCount:    stats.EvaluatedCount,
				AssignedScorecardCount:     0, // No assigned scorecards for time range only
				AcknowledgedScorecardCount: stats.AcknowledgedCount,
				PublishedScorecardCount:    stats.PublishedCount,
				AvgPerfScore:               calculateAvgPerfScore(stats.ScoreSum, stats.ScoreCount),
				CompletionRate:             -1, // No completion rate for time range only
			},
		}

		// Update total stats
		totalQMTaskStats.EvalueatedScrecardCount += stats.EvaluatedCount
		totalQMTaskStats.AcknowledgedScorecardCount += stats.AcknowledgedCount
		totalQMTaskStats.PublishedScorecardCount += stats.PublishedCount
		totalScoreSum += stats.ScoreSum
		totalScoreCount += stats.ScoreCount
	}

	return &analyticspb.RetrieveDirectorTaskStatsResponse{
		DirectorTaskStats: directorTaskStats,
		TotalDirectorTaskStats: &analyticspb.DirectorTaskStats{
			QmTaskStats: &analyticspb.QMTaskStats{
				EvalueatedScrecardCount:    totalQMTaskStats.EvalueatedScrecardCount,
				AssignedScorecardCount:     totalQMTaskStats.AssignedScorecardCount,
				AcknowledgedScorecardCount: totalQMTaskStats.AcknowledgedScorecardCount,
				PublishedScorecardCount:    totalQMTaskStats.PublishedScorecardCount,
				AvgPerfScore:               calculateAvgPerfScore(totalScoreSum, totalScoreCount),
				CompletionRate:             totalQMTaskStats.CompletionRate,
			},
		},
	}, nil
}

// Get the local time closest to the pivot time that is before the pivot time
func getClosestLocalTimeBeforePivot(localTime time.Time, pivotTime time.Time) time.Time {
	closestLocalTime := time.Date(
		pivotTime.Year(),
		pivotTime.Month(),
		pivotTime.Day(),
		localTime.Hour(),
		localTime.Minute(),
		localTime.Second(),
		0,
		time.UTC,
	)
	if closestLocalTime.After(pivotTime) {
		closestLocalTime = closestLocalTime.AddDate(0, 0, -1)
	}
	return closestLocalTime
}

// Get the local time closest to the pivot time that is after the pivot time
func getClosestLocalTimeAfterPivot(localTime time.Time, pivotTime time.Time) time.Time {
	closestLocalTime := time.Date(
		pivotTime.Year(),
		pivotTime.Month(),
		pivotTime.Day(),
		localTime.Hour(),
		localTime.Minute(),
		localTime.Second(),
		0,
		time.UTC,
	)
	if closestLocalTime.Before(pivotTime) {
		closestLocalTime = closestLocalTime.AddDate(0, 0, 1)
	}
	return closestLocalTime
}
