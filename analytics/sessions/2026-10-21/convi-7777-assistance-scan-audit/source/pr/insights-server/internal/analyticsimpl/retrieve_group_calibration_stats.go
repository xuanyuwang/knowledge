package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	log "github.com/cresta/shared-go/framework/log"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedauth "github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/db/condition"
	sharedqa "github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"
	shareduser "github.com/cresta/go-servers/shared/user"
)

const (
	ConsistencyScoreNotAvailable = float32(-1.0)
)

// GroupCalibrationAggGroup contains the aggregated data of an aggregation group
// which will be used to generate the final group calibration stats
type GroupCalibrationAggGroup struct {
	DirectorTaskSet        set.Set[string]
	DirectorAssigneeIDSet  set.Set[string]
	Scorecards             map[string]*analytics.EvaluatedScorecard // Used when aggregate on scorecard
	Criteria               []*analytics.EvaluatedCriterion          // Used when aggregate on criterion
	SumOfConsistencyScore  float64
	CountOfItems           int // Number of items in the group. When aggregate on scorecard, this is the number of scorecards. Otherwise, this is the number of criterion
	AssignedScorecardCount int
	PendingScorecardCount  int // Pending: number of assignees that havne't finished the task before due date
}

func (g *GroupCalibrationAggGroup) APIDirectorTaskStats(attribute *analytics.Attribute) *analytics.DirectorTaskStats {
	evaluatedScorecards := fn.MapValueToSlice(g.Scorecards)
	// Use negative value to indicate that the consistency score is not available since the count of items is 0
	avgConsistencyScore := ConsistencyScoreNotAvailable
	if g.CountOfItems > 0 {
		avgConsistencyScore = float32(g.SumOfConsistencyScore / float64(g.CountOfItems))
	}
	stats := &analytics.DirectorTaskStats{
		Attribute:         attribute,
		DirectorTaskCount: int32(g.DirectorTaskSet.Len()),
		GroupCalibrationTaskStats: &analytics.GroupCalibrationTaskStats{
			AvgConsistencyScore:     avgConsistencyScore,
			AssignedScorecardCount:  int32(g.AssignedScorecardCount),
			EvaluatedScorecardCount: int32(len(g.Scorecards)),
			PendingScorecardCount:   int32(g.PendingScorecardCount),
			EvaluatedScorecards:     evaluatedScorecards,
			EvaluatedCriteria:       g.Criteria,
		},
	}
	return stats
}

func NewEmptyGroupCalibrationAggGroup() *GroupCalibrationAggGroup {
	return &GroupCalibrationAggGroup{
		DirectorTaskSet:        set.New[string](),
		DirectorAssigneeIDSet:  set.New[string](),
		Scorecards:             map[string]*analytics.EvaluatedScorecard{},
		SumOfConsistencyScore:  0,
		AssignedScorecardCount: 0,
		PendingScorecardCount:  0,
	}
}

//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
type GroupCalibrationScore struct {
	// group by keys
	TaskID       string                `gorm:"column:task_id"`
	QAAnalystID  string                `gorm:"column:qa_analyst_id"`
	CriterionKey sharedqa.CriterionKey `gorm:"-"`
	Frequency    time.Time             `gorm:"column:frequency"`
	// scorecard info
	ResponseScorecardID string `gorm:"column:scorecard_id"`
	ScorecardType       int32  `gorm:"column:scorecard_type"`
	// criterion info
	CriterionDisplayName string                             `gorm:"-"`
	CriterionIdentifier  string                             `gorm:"column:criterion_identifier"`
	Criterion            scoring.ScorecardTemplateCriterion `gorm:"-"`
	// score info
	NumericValue  sql.NullFloat64 `gorm:"column:numeric_value"`
	NotApplicable sql.NullBool    `gorm:"column:not_applicable"`
}

func (c *GroupCalibrationScore) DBScore() *dbmodel.Scores {
	return &dbmodel.Scores{
		CriterionIdentifier: c.CriterionIdentifier,
		NotApplicable:       c.NotApplicable,
		NumericValue:        c.NumericValue,
	}
}

func (c *GroupCalibrationScore) CriterionValue() *analytics.EvaluatedCriterion_EvaluatedCriterionValue {
	numericValue := float64(-1)
	if c.NumericValue.Valid {
		numericValue = c.NumericValue.Float64
	}
	return &analytics.EvaluatedCriterion_EvaluatedCriterionValue{
		NumericValue:  float32(numericValue),
		NotApplicable: scoring.IsNotApplicable(c.DBScore()),
		Empty:         scoring.IsEmptyScore(c.DBScore()),
	}
}

type GroupCalibrationScorecard struct {
	TaskID              string
	QAAnalystID         string
	Frequency           time.Time
	ResponseScorecardID string
	ResponseScores      map[sharedqa.CriterionKey]*GroupCalibrationScore
	ConsistencyScore    float64 // scorecard level consistency score
}

type GroupCalibrationTask struct {
	TaskID                    string
	StartDate                 time.Time // The time when a task is active. It's the CreatedAt time of a task when a task's status is active
	DueDate                   time.Time
	AssigneeUserIDSet         set.Set[string]
	CompleteAssigneeUserIDSet set.Set[string]
	ResponseScorecards        []*GroupCalibrationScorecard
	AnswerKeyScores           map[sharedqa.CriterionKey]*GroupCalibrationScore
	TemplateKey               sharedqa.ScorecardTemplateKey
	Template                  scoring.ScorecardTemplateStructure
}

type GroupCalibrationstatsGroupByType int32

const (
	GroupCalibrationstatsGroupByNone GroupCalibrationstatsGroupByType = iota
	GroupCalibrationstatsGroupByQAAnalyst
	GroupCalibrationstatsGroupByCriterion
	GroupCalibrationstatsGroupByDirectorTask
	GroupCalibrationstatsGroupByFrequency
	GroupCalibrationstatsGroupByQAAnalystAndFrequency
)

type GroupCalibrationStatsConditions struct {
	TaskOverlapTimeRange                  *TimeRange
	ScorecardTemplateIDs                  []string
	DirectorTaskIDs                       []string
	QAAnalystIDs                          []string
	UsecaseIDs                            []string
	CustomerID                            string
	ProfileID                             string
	ExcludeEarlyTasksFromConsistencyScore bool
}

func (c *GroupCalibrationStatsConditions) String() string {
	return fmt.Sprintf("TaskOverlapTimeRange: %v, ScorecardTemplateIDs: %v, DirectorTaskIDs: %v, QAAnalystIDs: %v, UsecaseIDs: %v, CustomerID: %v, ProfileID: %v, ExcludeEarlyTasksFromConsistencyScore: %v",
		c.TaskOverlapTimeRange, c.ScorecardTemplateIDs, c.DirectorTaskIDs, c.QAAnalystIDs, c.UsecaseIDs, c.CustomerID, c.ProfileID, c.ExcludeEarlyTasksFromConsistencyScore)
}

type GroupCalibrationStatsGroupByKey struct {
	QAAnalystID    string
	DirectorTaskID string
	CriterionKey   sharedqa.CriterionKey
	Frequency      sql.NullTime
}

func (k GroupCalibrationStatsGroupByKey) IsEmpty() bool {
	return k.QAAnalystID == "" && k.DirectorTaskID == "" && k.CriterionKey == sharedqa.CriterionKey{} && !k.Frequency.Valid
}

func (k GroupCalibrationStatsGroupByKey) ToAttribute(userIDToUser map[string]*userpb.User, profileName *shared.CustomerProfile) *analytics.Attribute {
	attribute := &analytics.Attribute{}
	if attribute.Users == nil {
		attribute.Users = []*userpb.User{}
	}
	if k.QAAnalystID != "" {
		attribute.Users = append(attribute.Users, userIDToUser[k.QAAnalystID])
	}
	if (k.CriterionKey != sharedqa.CriterionKey{}) {
		attribute.CriterionIdentifiers = []string{k.CriterionKey.CriterionID}
	}
	if k.DirectorTaskID != "" {
		attribute.TaskNames = append(attribute.TaskNames, coachingpb.DirectorTaskName{
			CustomerID:     profileName.CustomerID,
			ProfileID:      profileName.ProfileID,
			DirectorTaskID: k.DirectorTaskID,
		}.String())
	}
	if k.Frequency.Valid {
		if attribute.TimeRanges == nil {
			attribute.TimeRanges = []*analytics.TimeRange{}
		}
		attribute.TimeRanges = append(attribute.TimeRanges, &analytics.TimeRange{
			StartTimestamp: timestamppb.New(k.Frequency.Time),
		})
	}
	return attribute
}

type GroupCalibrationStatsGroupByKeyGetter func(*GroupCalibrationScorecard) GroupCalibrationStatsGroupByKey

func GroupCalibrationStatsGroupByTypeToGetter(groupBy GroupCalibrationstatsGroupByType) GroupCalibrationStatsGroupByKeyGetter {
	switch groupBy {
	case GroupCalibrationstatsGroupByNone:
		return func(info *GroupCalibrationScorecard) GroupCalibrationStatsGroupByKey {
			return GroupCalibrationStatsGroupByKey{}
		}
	case GroupCalibrationstatsGroupByQAAnalyst:
		return func(info *GroupCalibrationScorecard) GroupCalibrationStatsGroupByKey {
			return GroupCalibrationStatsGroupByKey{
				QAAnalystID: info.QAAnalystID,
			}
		}
	case GroupCalibrationstatsGroupByDirectorTask:
		return func(info *GroupCalibrationScorecard) GroupCalibrationStatsGroupByKey {
			return GroupCalibrationStatsGroupByKey{
				DirectorTaskID: info.TaskID,
			}
		}
	case GroupCalibrationstatsGroupByFrequency:
		return func(info *GroupCalibrationScorecard) GroupCalibrationStatsGroupByKey {
			return GroupCalibrationStatsGroupByKey{
				Frequency: sql.NullTime{
					Time:  info.Frequency,
					Valid: true,
				},
			}
		}
	case GroupCalibrationstatsGroupByQAAnalystAndFrequency:
		return func(info *GroupCalibrationScorecard) GroupCalibrationStatsGroupByKey {
			return GroupCalibrationStatsGroupByKey{
				QAAnalystID: info.QAAnalystID,
				Frequency: sql.NullTime{
					Time:  info.Frequency,
					Valid: true,
				},
			}
		}
	default:
		return nil
	}
}

func retrieveGroupCalibrationStats(
	ctx context.Context,
	db *gorm.DB,
	profile *shared.CustomerProfile,
	userClient userpb.UserServiceClient,
	req *analytics.RetrieveDirectorTaskStatsRequest,
	authEntityProvider sharedauth.AuthEntityProvider,
) (*analytics.RetrieveDirectorTaskStatsResponse, error) {
	logger := log.GetLogger()
	conditions, err := buildGroupCalibrationStatsConditions(ctx, userClient, profile, req, authEntityProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to build group-by conditions: %w", err)
	}
	logger.Infof(ctx, "GC stats conditions: %v", conditions.String())

	groupBy, err := ParseGroupCalibrationStatsGroupByType(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse group-by: %w", err)
	}

	// Retrieve tasks from database
	if req.Frequency == analytics.Frequency_FREQUENCY_UNSPECIFIED {
		if groupBy == GroupCalibrationstatsGroupByFrequency || groupBy == GroupCalibrationstatsGroupByQAAnalystAndFrequency {
			return nil, fmt.Errorf("frequency is required when group-by is frequency or qa-analyst-and-frequency")
		}
		// Default to monthly if frequency is not specified. The generated data will not be used anyway.
		req.Frequency = analytics.Frequency_MONTHLY
	}
	tasks, err := retrieveGroupCalibrationTasks(ctx, db, profile, userClient, req.Frequency, req.GetMetadata(), conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve group calibration: %w", err)
	}
	logger.Infof(ctx, "GC stats tasks IDs: %v", fn.MapKeyToSlice(tasks))
	if len(tasks) == 0 {
		return &analytics.RetrieveDirectorTaskStatsResponse{
			DirectorTaskStats:      []*analytics.DirectorTaskStats{},
			TotalDirectorTaskStats: &analytics.DirectorTaskStats{},
		}, nil
	}

	// calculate consistency scores
	err = calculateGroupCaliScorecardConsistencyScores(tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate scorecard consistency scores: %w", err)
	}

	// Get user details
	userIDToUser, err := getUserDetailsForTasks(ctx, profile, tasks, userClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get user details for tasks: %w", err)
	}

	// Group and aggregate stats
	var groupStats []*analytics.DirectorTaskStats
	var totalStats *analytics.DirectorTaskStats
	if groupBy == GroupCalibrationstatsGroupByCriterion {
		groupStats, totalStats = aggregateGroupCalibrationCriteriaStats(
			tasks,
			userIDToUser,
			profile,
		)
	} else {
		groupStats, totalStats = aggregateGroupCalibrationStats(
			tasks,
			groupBy,
			userIDToUser,
			profile,
			conditions.ExcludeEarlyTasksFromConsistencyScore,
		)
	}

	return &analytics.RetrieveDirectorTaskStatsResponse{
		DirectorTaskStats:      groupStats,
		TotalDirectorTaskStats: totalStats,
	}, nil
}

func aggregateGroupCalibrationCriteriaStats(
	tasks map[string]*GroupCalibrationTask,
	userIDToUser map[string]*userpb.User,
	profileName *shared.CustomerProfile,
) ([]*analytics.DirectorTaskStats, *analytics.DirectorTaskStats) {
	totalStats := NewEmptyGroupCalibrationAggGroup()

	// store all response scores for answer key criteria
	evaluatedCriteria := map[GroupCalibrationStatsGroupByKey]*GroupCalibrationAggGroup{}
	for _, taskInfo := range tasks {
		_, allCriteria := taskInfo.Template.GetChaptersAndCriteria()
		// there is a new evaluated criterion per each scorecard per each scoreable criterion
		for _, criterion := range allCriteria {
			if !scoring.IsCriterionScorableForConsistency(criterion) {
				continue
			}
			criterionKey := sharedqa.CriterionKey{
				ScorecardTemplateKey: taskInfo.TemplateKey,
				CriterionID:          criterion.GetIdentifier(),
			}
			groupKey := GroupCalibrationStatsGroupByKey{CriterionKey: criterionKey}
			if _, ok := evaluatedCriteria[groupKey]; !ok {
				evaluatedCriteria[groupKey] = &GroupCalibrationAggGroup{
					Criteria:              []*analytics.EvaluatedCriterion{},
					SumOfConsistencyScore: 0,
					CountOfItems:          0,
					DirectorTaskSet:       set.NewFromSlice([]string{}), // avoid nil pointer
				}
			}
			answerKeyScore := taskInfo.AnswerKeyScores[criterionKey]
			for _, scorecard := range taskInfo.ResponseScorecards {
				evaluatedCriterion := &analytics.EvaluatedCriterion{
					CriterionDisplayName: criterion.GetDisplayName(),
					SubmitterUser:        userIDToUser[scorecard.QAAnalystID],
					ScorecardName: coachingpb.ScorecardName{
						CustomerID:  profileName.CustomerID,
						ProfileID:   profileName.ProfileID,
						ScorecardID: scorecard.ResponseScorecardID,
					}.String(),
				}
				var answerKeyDBScore, responseDBScore *dbmodel.Scores
				if answerKeyScore != nil {
					evaluatedCriterion.AnswerKeyCriterionValue = answerKeyScore.CriterionValue()
					answerKeyDBScore = answerKeyScore.DBScore()
				} else {
					evaluatedCriterion.AnswerKeyCriterionValue = &analytics.EvaluatedCriterion_EvaluatedCriterionValue{
						Empty: true,
					}
				}
				responseScore := scorecard.ResponseScores[criterionKey]
				if responseScore != nil {
					evaluatedCriterion.ResponseCriterionValue = responseScore.CriterionValue()
					responseDBScore = responseScore.DBScore()
				} else {
					evaluatedCriterion.ResponseCriterionValue = &analytics.EvaluatedCriterion_EvaluatedCriterionValue{
						Empty: true,
					}
				}
				isConsistent := scoring.ScoreEquals(answerKeyDBScore, responseDBScore)
				evaluatedCriterion.Consistent = isConsistent
				if isConsistent {
					evaluatedCriteria[groupKey].SumOfConsistencyScore += 100
					totalStats.SumOfConsistencyScore += 100
				}
				evaluatedCriteria[groupKey].CountOfItems += 1
				totalStats.CountOfItems += 1
				evaluatedCriteria[groupKey].Criteria = append(evaluatedCriteria[groupKey].Criteria, evaluatedCriterion)
			}
		}
	}

	// convert to API stats
	APIGroupStats := []*analytics.DirectorTaskStats{}
	for groupKey, stats := range evaluatedCriteria {
		attribute := groupKey.ToAttribute(userIDToUser, profileName)
		APIGroupStats = append(APIGroupStats, stats.APIDirectorTaskStats(attribute))
	}
	APITotalStats := totalStats.APIDirectorTaskStats(nil /* attribute */)
	return APIGroupStats, APITotalStats
}

func calculateGroupCaliScorecardConsistencyScores(tasks map[string]*GroupCalibrationTask) error {
	for _, task := range tasks {
		responseScorecards := task.ResponseScorecards
		if len(responseScorecards) == 0 {
			continue
		}
		if task.AnswerKeyScores == nil {
			return fmt.Errorf("failed to get answer key scores for task %s", task.TaskID)
		}
		answerKeyScores := []*dbmodel.Scores{}
		for _, c := range task.AnswerKeyScores {
			answerKeyScore := c.DBScore()
			if answerKeyScore != nil {
				answerKeyScores = append(answerKeyScores, answerKeyScore)
			}
		}
		for _, scorecard := range responseScorecards {
			templateStruct := task.Template
			responseScores := []*dbmodel.Scores{}
			for _, c := range scorecard.ResponseScores {
				responseScore := c.DBScore()
				if responseScore != nil {
					responseScores = append(responseScores, responseScore)
				}
			}
			scorecardConsistencyScore, err := scoring.CalculateConsistencyScore(responseScores, answerKeyScores, templateStruct, true /*rootOnly*/)
			if err != nil {
				return fmt.Errorf("failed to calculate consistency score for scorecard %s: %w", scorecard.ResponseScorecardID, err)
			}
			scorecard.ConsistencyScore = scorecardConsistencyScore
		}
	}
	return nil
}

func findCommonElements(slice1, slice2 []string) []string {
	// Create a map for O(1) lookup
	elementMap := make(map[string]bool)
	for _, elem := range slice1 {
		elementMap[elem] = true
	}

	// Find common elements
	var common []string
	for _, elem := range slice2 {
		if elementMap[elem] {
			common = append(common, elem)
		}
	}
	return common
}

func findTasks(
	ctx context.Context,
	db *gorm.DB,
	profile *shared.CustomerProfile,
	userClient userpb.UserServiceClient,
	conditions *GroupCalibrationStatsConditions,
) (map[string]*GroupCalibrationTask, map[string][]string, error) {
	taskCondBuilder := condition.Builder().
		AndWhere("tasks.task_status = @task_status", sql.Named("task_status", coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE)).
		AndWhere("tasks.task_type = @task_type", sql.Named("task_type", coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION)).
		AndWhere("tasks.usecase_id IN @usecase_ids", sql.Named("usecase_ids", conditions.UsecaseIDs)).
		AndWhere("tasks.customer_id = @customer_id", sql.Named("customer_id", conditions.CustomerID)).
		AndWhere("tasks.profile_id = @profile_id", sql.Named("profile_id", conditions.ProfileID))

	if conditions.TaskOverlapTimeRange != nil {
		// Find tasks whose [created_at, due_date] overlaps with the time range
		taskCondBuilder = taskCondBuilder.AndWhere(
			"(NOT (tasks.created_at > @time_range_end OR (tasks.task_schedule_config#>>'{groupCalibrationScheduleConfig,dueDate}')::timestamp < @time_range_start))",
			sql.Named("time_range_end", conditions.TaskOverlapTimeRange.End),
			sql.Named("time_range_start", conditions.TaskOverlapTimeRange.Start),
		)
	}
	if len(conditions.DirectorTaskIDs) > 0 {
		taskCondBuilder = taskCondBuilder.AndWhere("tasks.task_id IN @director_task_ids", sql.Named("director_task_ids", conditions.DirectorTaskIDs))
	}
	if len(conditions.ScorecardTemplateIDs) > 0 {
		taskCondBuilder = taskCondBuilder.AndWhere("tasks.scorecard_template_id IN @scorecard_template_ids", sql.Named("scorecard_template_ids", conditions.ScorecardTemplateIDs))
	}

	//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
	type task struct {
		dbmodel.Tasks
		TemplateID       string `gorm:"column:scorecard_template_id"`
		TemplateRevision string `gorm:"column:template_revision"`
		TemplateJSON     string `gorm:"column:template"`
	}
	var filteredDBTasks []*task
	query := func(tx *gorm.DB) *gorm.DB {
		return tx.Raw(fmt.Sprintf(`
		WITH tasks AS (
			SELECT
				tasks.*,
				content_config->>'scorecardTemplateId' AS scorecard_template_id,
				content_config->>'scorecardTemplateRevisionId' AS template_revision
			FROM
				director.tasks,
				LATERAL jsonb_array_elements(
					(tasks.task_content_config->'groupCalibrationContentConfig'->'tasks')::jsonb
				) AS content_config
		)
		SELECT
            tasks.*,
            st.template
        FROM tasks
        JOIN director.scorecard_templates AS st ON tasks.scorecard_template_id = st.resource_id AND tasks.template_revision = st.revision
		WHERE %s`, taskCondBuilder.QueryString()),
			taskCondBuilder.QueryParams()...,
		).Find(&filteredDBTasks)
	}
	logger := log.GetLogger()
	logger.Infof(ctx, "GC stats tasks query: %s", db.ToSQL(query))
	err := query(db).Error
	if err != nil {
		return nil, nil, err
	}

	dbTaskIDToAudienceUserIDs := map[string][]string{}

	for _, dbTask := range filteredDBTasks {
		taskAudienceUserIDs, err := sharedqa.ExtractUserIDsFromDBTaskAudienceConfig(ctx, userClient, &customerpb.CustomerOrProfileName{
			CustomerID: profile.CustomerID,
			ProfileID:  profile.ProfileID,
		}, &dbmodel.Tasks{TaskAudienceConfig: dbTask.TaskAudienceConfig, TaskType: dbTask.TaskType})
		if err != nil {
			return nil, nil, err
		}

		dbTaskIDToAudienceUserIDs[dbTask.TaskID] = taskAudienceUserIDs
	}

	if len(conditions.QAAnalystIDs) > 0 {
		filteredDBTasks = fn.Filter(filteredDBTasks, func(dbTask *task) bool {
			taskAudienceUserIDsSet := set.NewFromSlice(dbTaskIDToAudienceUserIDs[dbTask.TaskID])
			return taskAudienceUserIDsSet.HasAny(conditions.QAAnalystIDs...)
		})
	}

	if len(filteredDBTasks) == 0 {
		return map[string]*GroupCalibrationTask{}, dbTaskIDToAudienceUserIDs, nil
	}
	tasks := map[string]*GroupCalibrationTask{}
	parsedTemplates := map[sharedqa.ScorecardTemplateKey]scoring.ScorecardTemplateStructure{}
	for _, dbTask := range filteredDBTasks {
		templateKey := sharedqa.ScorecardTemplateKey{
			TemplateID:       dbTask.TemplateID,
			TemplateRevision: dbTask.TemplateRevision,
		}
		if _, ok := parsedTemplates[templateKey]; !ok {
			parsedTemplate, err := scoring.ParseScorecardTemplateStructure(dbTask.TemplateJSON)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse scorecard template: %w", err)
			}
			parsedTemplates[templateKey] = parsedTemplate
		}

		// Find common elements between task audience IDs and assignee IDs
		var assigneeIDs []string
		if len(conditions.QAAnalystIDs) > 0 {
			assigneeIDs = findCommonElements(dbTaskIDToAudienceUserIDs[dbTask.TaskID], conditions.QAAnalystIDs)
		} else {
			assigneeIDs = dbTaskIDToAudienceUserIDs[dbTask.TaskID]
		}

		tasks[dbTask.TaskID] = &GroupCalibrationTask{
			TaskID:                    dbTask.TaskID,
			StartDate:                 dbTask.CreatedAt,
			DueDate:                   dbTask.TaskScheduleConfig.Message.GetGroupCalibrationScheduleConfig().GetDueDate().AsTime(),
			AssigneeUserIDSet:         set.NewFromSlice(assigneeIDs),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{}),
			TemplateKey: sharedqa.ScorecardTemplateKey{
				TemplateID:       dbTask.TemplateID,
				TemplateRevision: dbTask.TemplateRevision,
			},
			Template: parsedTemplates[templateKey],
		}
	}
	return tasks, dbTaskIDToAudienceUserIDs, nil
}

func findScores(
	ctx context.Context,
	db *gorm.DB,
	customerID string,
	profileID string,
	taskIDs []string,
	frequencyArgs []any,
	scorecardType coachingpb.ScorecardType,
	assigneeIDs []string,
	dbTaskIDToAudienceUserIDs map[string][]string,
) ([]*GroupCalibrationScore, error) {
	scoreCondBuilder := condition.
		NewBuilder("cards.scorecard_type = @scorecard_type", sql.Named("scorecard_type", int32(scorecardType))).
		AndWhere("cards.customer = @customer_id", sql.Named("customer_id", customerID)).
		AndWhere("cards.profile = @profile_id", sql.Named("profile_id", profileID)).
		AndWhere("tasks.task_id IN @task_ids", sql.Named("task_ids", taskIDs)).
		AndWhere("cards.submitted_at IS NOT NULL")
	if len(assigneeIDs) > 0 {
		scoreCondBuilder = scoreCondBuilder.AndWhere("cards.submitter_user_id IN @assignee_ids", sql.Named("assignee_ids", assigneeIDs))
	}

	responseArgs := []any{}
	responseArgs = append(responseArgs, frequencyArgs...)
	responseArgs = append(responseArgs, scoreCondBuilder.QueryParams()...)

	baseQuery := `
			SELECT
				tasks.task_id,
				cards.resource_id AS scorecard_id,
				cards.template_id,
				cards.template_revision,
				templates.template,
				cards.scorecard_type AS scorecard_type,
				cards.submitter_user_id AS qa_analyst_id,
				scores.criterion_identifier,
				scores.numeric_value,
				scores.not_applicable,
				DATE_TRUNC(
					@frequency,
					cards.submitted_at AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias
				) AT TIME ZONE @time_zone - INTERVAL '1 day' * @first_day_of_week_bias AS frequency
			FROM
				director.tasks AS tasks
			JOIN
				director.scorecards AS cards
			ON
				ARRAY[tasks.task_id] <@ cards.task_ids AND
				tasks.customer_id = cards.customer AND
				tasks.profile_id = cards.profile
			JOIN
				director.scores AS scores
			ON
				cards.resource_id = scores.scorecard_id AND
				cards.customer = scores.customer AND
				cards.profile = scores.profile
			JOIN
				director.scorecard_templates AS templates
			ON
				cards.template_id = templates.resource_id AND
				cards.template_revision = templates.revision AND
				cards.customer = templates.customer AND
				cards.profile = templates.profile
			WHERE %s
		ORDER BY task_id, scorecard_id, template_id, template_revision, criterion_identifier`

	var responseScores []*GroupCalibrationScore
	query := func(tx *gorm.DB) *gorm.DB {
		return tx.Raw(fmt.Sprintf(baseQuery, scoreCondBuilder.QueryString()), responseArgs...)
	}
	log.GetLogger().Infof(ctx, "GC stats scores query: %s", db.ToSQL(query))
	err := query(db).Find(&responseScores).Error
	if err != nil {
		return nil, err
	}

	if scorecardType == coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE {
		responseScores = fn.Filter(responseScores, func(responseScore *GroupCalibrationScore) bool {
			taskAudienceUserIDsSet := set.NewFromSlice(dbTaskIDToAudienceUserIDs[responseScore.TaskID])
			return taskAudienceUserIDsSet.Has(responseScore.QAAnalystID)
		})
	}

	return responseScores, nil
}

// retrieveGroupCalibrationTasks retrieves the group calibration data from the database
// and grouped by task so that every task, even those without response scorecards,
// will have an entry in the result
func retrieveGroupCalibrationTasks(
	ctx context.Context,
	db *gorm.DB,
	profile *shared.CustomerProfile,
	userClient userpb.UserServiceClient,
	frequency analytics.Frequency,
	metadata *analytics.Metadata,
	conditions *GroupCalibrationStatsConditions,
) (map[string]*GroupCalibrationTask, error) {
	frequencyArgs, err := postgres.ParseArgs(&frequency, metadata)
	if err != nil {
		return nil, err
	}

	tasks, dbTaskIDToAudienceUserIDs, err := findTasks(ctx, db, profile, userClient, conditions)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return map[string]*GroupCalibrationTask{}, nil
	}
	taskIDs := fn.MapKeyToSlice(tasks)

	responseScores, err := findScores(
		ctx,
		db,
		conditions.CustomerID,
		conditions.ProfileID,
		taskIDs,
		frequencyArgs,
		coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE,
		conditions.QAAnalystIDs,
		dbTaskIDToAudienceUserIDs,
	)
	if err != nil {
		return nil, err
	}
	answerKeyScores, err := findScores(
		ctx,
		db,
		conditions.CustomerID,
		conditions.ProfileID,
		taskIDs,
		frequencyArgs,
		coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY,
		[]string{},
		dbTaskIDToAudienceUserIDs,
	)
	if err != nil {
		return nil, err
	}

	buildGroupCalibrationScore := func(records []*GroupCalibrationScore) error {
		for _, record := range records {
			task, ok := tasks[record.TaskID]
			if !ok {
				return fmt.Errorf("failed to find task for criterion %s of scorecard %s", record.CriterionIdentifier, record.ResponseScorecardID)
			}
			chapters, criteria := task.Template.GetChaptersAndCriteria()
			criterion, ok := criteria[record.CriterionIdentifier]
			if !ok {
				// a chapter also has a score, so we skip it
				if _, isChapter := chapters[record.CriterionIdentifier]; isChapter {
					continue
				}
				return fmt.Errorf("failed to get criterion: %s", record.CriterionIdentifier)
			}

			record.CriterionKey = sharedqa.CriterionKey{
				ScorecardTemplateKey: task.TemplateKey,
				CriterionID:          record.CriterionIdentifier,
			}
			record.CriterionDisplayName = criterion.GetDisplayName()
			record.Criterion = criterion
		}
		return nil
	}

	// fill in the fields for response scores
	err = buildGroupCalibrationScore(responseScores)
	if err != nil {
		return nil, err
	}
	responseScores = filterComparableScores(responseScores)

	// fill in the fields for answer key scores
	err = buildGroupCalibrationScore(answerKeyScores)
	if err != nil {
		return nil, err
	}
	answerKeyScores = filterComparableScores(answerKeyScores)

	responseScorecardIDToTaskID := map[string]string{}
	responseScorecardIDToScores := map[string]map[sharedqa.CriterionKey]*GroupCalibrationScore{}
	// group response scores by scorecard ID
	for _, score := range responseScores {
		responseScorecardIDToTaskID[score.ResponseScorecardID] = score.TaskID
		if _, ok := responseScorecardIDToScores[score.ResponseScorecardID]; !ok {
			responseScorecardIDToScores[score.ResponseScorecardID] = map[sharedqa.CriterionKey]*GroupCalibrationScore{}
		}
		responseScorecardIDToScores[score.ResponseScorecardID][score.CriterionKey] = score
	}

	// group answer key scores by task ID
	for _, score := range answerKeyScores {
		if _, ok := tasks[score.TaskID]; !ok {
			return nil, fmt.Errorf("failed to get task for answer key score %s", score.TaskID)
		}
		if tasks[score.TaskID].AnswerKeyScores == nil {
			tasks[score.TaskID].AnswerKeyScores = map[sharedqa.CriterionKey]*GroupCalibrationScore{}
		}
		tasks[score.TaskID].AnswerKeyScores[score.CriterionKey] = score
	}

	// in case there are tasks that have no response scorecards, initialize them with empty slices for each task
	taskIDToResponseScorecards := map[string][]*GroupCalibrationScorecard{}
	for _, taskID := range taskIDs {
		taskIDToResponseScorecards[taskID] = []*GroupCalibrationScorecard{}
	}
	// Add response scorecards to tasks
	for scorecardID := range responseScorecardIDToTaskID {
		taskID := responseScorecardIDToTaskID[scorecardID]
		answerKeyScores := tasks[taskID].AnswerKeyScores
		if answerKeyScores == nil {
			return nil, fmt.Errorf("failed to get answer key scores for task %s", taskID)
		}
		responseScores, ok := responseScorecardIDToScores[scorecardID]
		if !ok {
			return nil, fmt.Errorf("failed to get response scores for scorecard %s", scorecardID)
		}
		randomScore := fn.MapValueToSlice(responseScores)[0]
		qaAnalystID := randomScore.QAAnalystID
		frequency := randomScore.Frequency
		scorecard := &GroupCalibrationScorecard{
			TaskID:              taskID,
			QAAnalystID:         qaAnalystID,
			ResponseScorecardID: scorecardID,
			Frequency:           frequency,
			ResponseScores:      responseScores,
		}
		if _, ok := taskIDToResponseScorecards[taskID]; !ok {
			taskIDToResponseScorecards[taskID] = []*GroupCalibrationScorecard{}
		}
		taskIDToResponseScorecards[taskID] = append(taskIDToResponseScorecards[taskID], scorecard)
		if _, ok := tasks[taskID]; !ok {
			return nil, fmt.Errorf("failed to get task for response scorecard %s", scorecardID)
		}
		tasks[taskID].ResponseScorecards = append(tasks[taskID].ResponseScorecards, scorecard)
		tasks[taskID].CompleteAssigneeUserIDSet.Add(scorecard.QAAnalystID)
	}
	return tasks, nil
}

// filterComparableScores filters out scores that are not comparable
// Currently, only numeric radios, labeled radios, and dropdown numeric values are comparable
func filterComparableScores(scores []*GroupCalibrationScore) []*GroupCalibrationScore {
	comparableScores := []*GroupCalibrationScore{}
	for _, score := range scores {
		if score.Criterion == nil {
			// skip chapters
			continue
		}
		if scoring.IsCriterionScorableForConsistency(score.Criterion) {
			comparableScores = append(comparableScores, score)
		}
	}
	return comparableScores
}

func getUserDetailsForTasks(
	ctx context.Context,
	profile *shared.CustomerProfile,
	tasks map[string]*GroupCalibrationTask,
	userServiceClient userpb.UserServiceClient,
) (map[string]*userpb.User, error) {
	// Collect all assignee user IDs from the tasks
	userIDs := set.New[string]()
	for _, task := range tasks {
		userIDs.AddAll(task.AssigneeUserIDSet.Slice()...)
	}

	if userIDs.Len() == 0 {
		return map[string]*userpb.User{}, nil
	}

	// Query user service for user details
	users := []*userpb.User{}
	err := shareduser.CollectUsersPaginated(
		ctx,
		userServiceClient,
		profile.CustomerID,
		profile.ProfileID,
		&userpb.ListUsersRequest_ListUsersFilter{
			UserIds: userIDs.Slice(),
		},
		1000,                        // page size
		"user_id",                   // order field
		userpb.ListUsersRequest_ASC, // sort order
		true,                        // include user memberships
		func(u []*userpb.User) error {
			users = append(users, u...)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	// Build map from user ID to user
	userIDToUser := make(map[string]*userpb.User, len(users))
	for _, user := range users {
		userName, err := userpb.ParseUserName(user.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user name %s: %w", user.Name, err)
		}
		userIDToUser[userName.UserID] = user
	}

	return userIDToUser, nil
}

func aggregateGroupCalibrationStats(
	tasks map[string]*GroupCalibrationTask,
	groupBy GroupCalibrationstatsGroupByType,
	userIDToUser map[string]*userpb.User,
	profileName *shared.CustomerProfile,
	excludeEarlyTasksFromConsistencyScore bool,
) ([]*analytics.DirectorTaskStats, *analytics.DirectorTaskStats) {
	groupKeyToStats := map[GroupCalibrationStatsGroupByKey]*GroupCalibrationAggGroup{}
	totalStats := NewEmptyGroupCalibrationAggGroup()

	// iterate over tasks' response scorecards to aggregate scorecard level data
	groupKeyToGetter := GroupCalibrationStatsGroupByTypeToGetter(groupBy)
	for _, taskInfo := range tasks {
		scorecards := taskInfo.ResponseScorecards
		taskIsEarly := time.Now().Before(taskInfo.DueDate)
		for _, scorecard := range scorecards {
			groupKey := groupKeyToGetter(scorecard)
			if _, ok := groupKeyToStats[groupKey]; !ok {
				groupKeyToStats[groupKey] = NewEmptyGroupCalibrationAggGroup()
			}

			evaluatedScorecard := &analytics.EvaluatedScorecard{
				ScorecardName:    coachingpb.ScorecardName{CustomerID: profileName.CustomerID, ProfileID: profileName.ProfileID, ScorecardID: scorecard.ResponseScorecardID}.String(),
				SubmitterUser:    userIDToUser[scorecard.QAAnalystID],
				ConsistencyScore: float32(scorecard.ConsistencyScore),
			}

			if _, ok := groupKeyToStats[groupKey].Scorecards[scorecard.ResponseScorecardID]; !ok {
				groupKeyToStats[groupKey].Scorecards[scorecard.ResponseScorecardID] = evaluatedScorecard
				// Only include response scorecards into consistency score in any of the following cases:
				// 1. The task's due date is in the past
				// 2. The request does not ask to exclude ongoing tasks from consistency score
				if !taskIsEarly || !excludeEarlyTasksFromConsistencyScore {
					groupKeyToStats[groupKey].SumOfConsistencyScore += scorecard.ConsistencyScore
					groupKeyToStats[groupKey].CountOfItems += 1
				}
			}

			if _, ok := totalStats.Scorecards[scorecard.ResponseScorecardID]; !ok {
				totalStats.Scorecards[scorecard.ResponseScorecardID] = evaluatedScorecard
				if !taskIsEarly || !excludeEarlyTasksFromConsistencyScore {
					totalStats.SumOfConsistencyScore += scorecard.ConsistencyScore
					totalStats.CountOfItems += 1
				}
			}
		}
	}

	// aggregate assigned task count
	for _, task := range tasks {
		taskIsEarly := time.Now().Before(task.DueDate)
		switch groupBy {
		case GroupCalibrationstatsGroupByNone:
			groupKey := GroupCalibrationStatsGroupByKey{}
			if _, ok := groupKeyToStats[groupKey]; !ok {
				groupKeyToStats[groupKey] = NewEmptyGroupCalibrationAggGroup()
			}
			groupStats := groupKeyToStats[groupKey]
			groupStats.DirectorTaskSet.Add(task.TaskID)
			totalStats.DirectorTaskSet.Add(task.TaskID)
			groupStats.AssignedScorecardCount += task.AssigneeUserIDSet.Len()
			totalStats.AssignedScorecardCount += task.AssigneeUserIDSet.Len()
			if taskIsEarly {
				groupStats.PendingScorecardCount += (task.AssigneeUserIDSet.Len() - task.CompleteAssigneeUserIDSet.Len())
				totalStats.PendingScorecardCount += (task.AssigneeUserIDSet.Len() - task.CompleteAssigneeUserIDSet.Len())
			}
		case GroupCalibrationstatsGroupByQAAnalyst:
			task.AssigneeUserIDSet.ForEach(func(assigneeID string) {
				groupKey := GroupCalibrationStatsGroupByKey{
					QAAnalystID: assigneeID,
				}
				if _, ok := groupKeyToStats[groupKey]; !ok {
					groupKeyToStats[groupKey] = NewEmptyGroupCalibrationAggGroup()
				}
				groupStats := groupKeyToStats[groupKey]
				groupStats.DirectorTaskSet.Add(task.TaskID)
				totalStats.DirectorTaskSet.Add(task.TaskID)
				groupStats.AssignedScorecardCount += 1
				totalStats.AssignedScorecardCount += 1
				if !task.CompleteAssigneeUserIDSet.Has(assigneeID) && taskIsEarly {
					groupStats.PendingScorecardCount += 1
					totalStats.PendingScorecardCount += 1
				}
			})
		case GroupCalibrationstatsGroupByDirectorTask:
			groupKey := GroupCalibrationStatsGroupByKey{
				DirectorTaskID: task.TaskID,
			}
			if _, ok := groupKeyToStats[groupKey]; !ok {
				groupKeyToStats[groupKey] = NewEmptyGroupCalibrationAggGroup()
			}
			groupStats := groupKeyToStats[groupKey]
			groupStats.DirectorTaskSet.Add(task.TaskID)
			totalStats.DirectorTaskSet.Add(task.TaskID)
			groupStats.AssignedScorecardCount += task.AssigneeUserIDSet.Len()
			totalStats.AssignedScorecardCount += task.AssigneeUserIDSet.Len()
			if taskIsEarly {
				groupStats.PendingScorecardCount += (task.AssigneeUserIDSet.Len() - task.CompleteAssigneeUserIDSet.Len())
				totalStats.PendingScorecardCount += (task.AssigneeUserIDSet.Len() - task.CompleteAssigneeUserIDSet.Len())
			}
			groupStats.DirectorAssigneeIDSet.AddAll(task.AssigneeUserIDSet.Slice()...)
		}
	}

	APIGroupStats := []*analytics.DirectorTaskStats{}
	for groupKey, stats := range groupKeyToStats {
		attribute := groupKey.ToAttribute(userIDToUser, profileName)
		if stats.DirectorAssigneeIDSet.Len() > 0 {
			attribute.Users = fn.Map(stats.DirectorAssigneeIDSet.Slice(), func(userID string) *userpb.User {
				return userIDToUser[userID]
			})
		}

		apiStats := stats.APIDirectorTaskStats(attribute)
		APIGroupStats = append(APIGroupStats, apiStats)
	}
	APITotalStats := totalStats.APIDirectorTaskStats(nil /* attribute */)

	return APIGroupStats, APITotalStats
}

func ParseGroupCalibrationStatsGroupByType(req *analytics.RetrieveDirectorTaskStatsRequest) (GroupCalibrationstatsGroupByType, error) {
	reqGroupByAttributes := req.GetGroupByAttributes()
	switch {
	case len(reqGroupByAttributes) == 0:
		return GroupCalibrationstatsGroupByNone, nil
	case shared.OnlyContainsAttributeTypes(
		reqGroupByAttributes,
		analytics.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
	):
		return GroupCalibrationstatsGroupByQAAnalyst, nil
	case shared.OnlyContainsAttributeTypes(
		reqGroupByAttributes,
		analytics.AttributeType_ATTRIBUTE_TYPE_CRITERION,
	):
		return GroupCalibrationstatsGroupByCriterion, nil
	case shared.OnlyContainsAttributeTypes(
		reqGroupByAttributes,
		analytics.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
	):
		return GroupCalibrationstatsGroupByDirectorTask, nil
	case shared.OnlyContainsAttributeTypes(
		reqGroupByAttributes,
		analytics.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return GroupCalibrationstatsGroupByFrequency, nil
	case shared.OnlyContainsAttributeTypes(
		reqGroupByAttributes,
		analytics.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
		analytics.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return GroupCalibrationstatsGroupByQAAnalystAndFrequency, nil
	default:
		return 0, fmt.Errorf("the given group-by is not supported")
	}
}

func buildGroupCalibrationStatsConditions(
	ctx context.Context,
	userClient userpb.UserServiceClient,
	profile *shared.CustomerProfile,
	req *analytics.RetrieveDirectorTaskStatsRequest,
	authEntityProvider sharedauth.AuthEntityProvider,
) (*GroupCalibrationStatsConditions, error) {
	attribute := req.GetFilterByAttribute()

	scorecardTemplateIDs := []string{}
	directorTaskIDs := []string{}
	usecaseIDs := []string{}
	if attribute != nil {
		for _, templateName := range attribute.GetScorecardTemplateNames() {
			name, err := coachingpb.ParseScorecardTemplateName(templateName)
			if err != nil {
				return nil, err
			}
			scorecardTemplateIDs = append(scorecardTemplateIDs, name.ScorecardTemplateID)
		}
		for _, taskName := range attribute.GetTaskNames() {
			name, err := coachingpb.ParseDirectorTaskName(taskName)
			if err != nil {
				return nil, err
			}
			directorTaskIDs = append(directorTaskIDs, name.DirectorTaskID)
		}
		for _, usecaseName := range attribute.GetUsecaseNames() {
			name, err := customerpb.ParseUsecaseName(usecaseName)
			if err != nil {
				return nil, err
			}
			usecaseIDs = append(usecaseIDs, name.UsecaseID)
		}
	}

	uniqueQaAnalystIDs := set.New[string]()
	authedEntity, err := authEntityProvider.GetAuthEntity(ctx, profile.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get principal entity from auth context due to %w", err)
	}
	// Simple permission check:
	// If the requester is a super admin, they can see all requested qa analysts
	// Otherwise, they can only see their own qa analysts
	if authedEntity.HasAnyRole(authpb.AuthProto_SUPER_ADMIN, authpb.AuthProto_QA_ADMIN, authpb.AuthProto_ADMIN) {
		for _, userName := range req.GetAssignees() {
			name, err := userpb.ParseUserName(userName)
			if err != nil {
				return nil, err
			}
			uniqueQaAnalystIDs.Add(name.UserID)
		}

		scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
			ctx,
			userClient,
			&customerpb.CustomerOrProfileName{
				CustomerID: profile.CustomerID,
				ProfileID:  profile.ProfileID,
			},
			attribute.GetScorecardReviewerAudience().GetUsers(),
			attribute.GetScorecardReviewerAudience().GetGroups(),
			nil, /* no role filter */
		)
		if err != nil {
			return nil, err
		}

		if len(scorecardReviewersMap) > 0 {
			if len(uniqueQaAnalystIDs.Slice()) > 0 {
				// If there are both assignees and reviewers specified, take the intersection
				intersectedIDs := set.New[string]()
				uniqueQaAnalystIDs.ForEach(func(id string) {
					if scorecardReviewersMap[id] != nil {
						intersectedIDs.Add(id)
					}
				})
				uniqueQaAnalystIDs = intersectedIDs
			} else {
				// If there are only reviewers specified, take specified reviewers
				uniqueQaAnalystIDs.AddAll(fn.MapKeyToSlice(scorecardReviewersMap)...)
			}
		}
	} else if authedEntity.HasAnyRole(authpb.AuthProto_QA_SPECIALIST, authpb.AuthProto_MANAGER, authpb.AuthProto_MANAGER_2ND) {
		authedUserID, err := authedEntity.GetAuthedUserID()
		if err != nil {
			return nil, err
		}
		uniqueQaAnalystIDs.Add(authedUserID)
	}

	conditions := &GroupCalibrationStatsConditions{
		ScorecardTemplateIDs:                  scorecardTemplateIDs,
		DirectorTaskIDs:                       directorTaskIDs,
		QAAnalystIDs:                          uniqueQaAnalystIDs.Slice(),
		UsecaseIDs:                            usecaseIDs,
		CustomerID:                            profile.CustomerID,
		ProfileID:                             profile.ProfileID,
		ExcludeEarlyTasksFromConsistencyScore: len(req.GetAssignees()) > 0, // TODO: make this a explicit request field
	}
	if req.GetFilterByTimeRange() != nil {
		conditions.TaskOverlapTimeRange = &TimeRange{
			Start: req.GetFilterByTimeRange().GetStartTimestamp().AsTime(),
			End:   req.GetFilterByTimeRange().GetEndTimestamp().AsTime(),
		}
	}
	return conditions, nil
}
