package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	sessionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/session"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbsessionpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/session"
	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"

	"github.com/cresta/go-servers/shared/db/converter"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type RetrieveQMTaskStatsTestSuite struct {
	AnalyticsV2TestSuite
	analyticsService *AnalyticsServiceImpl
}

func (s *RetrieveQMTaskStatsTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()

	// Create users needed for INNER JOINs in queryScorecardDetails
	s.Require().NoError(s.DB().Create(&model.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("agent_id_1"),
		Username:    "agent_user_1",
		DisplayName: converter.ConvertToNullString("Agent 1"),
		FullName:    converter.ConvertToNullString("Agent 1"),
	}).Error)
	s.Require().NoError(s.DB().Create(&model.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("agent_id_2"),
		Username:    "agent_user_2",
		DisplayName: converter.ConvertToNullString("Agent 2"),
		FullName:    converter.ConvertToNullString("Agent 2"),
	}).Error)
	s.Require().NoError(s.DB().Create(&model.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_1"),
		Username:    "analyst_user_1",
		DisplayName: converter.ConvertToNullString("Analyst 1"),
		FullName:    converter.ConvertToNullString("Analyst 1"),
	}).Error)
	s.Require().NoError(s.DB().Create(&model.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_2"),
		Username:    "analyst_user_2",
		DisplayName: converter.ConvertToNullString("Analyst 2"),
		FullName:    converter.ConvertToNullString("Analyst 2"),
	}).Error)
	s.Require().NoError(s.DB().Create(&model.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_3"),
		Username:    "analyst_user_3",
		DisplayName: converter.ConvertToNullString("Analyst 3"),
		FullName:    converter.ConvertToNullString("Analyst 3"),
	}).Error)

	// Initialize the analytics service with only the required dependencies
	s.analyticsService = &AnalyticsServiceImpl{
		appsDB:             s.DBTestSuite.AppsDBRegistry(),
		logger:             log.GetLogger(),
		userClientRegistry: clientregistry.NewTestRegistry[userpb.UserServiceClient](s.mockUserClient),
	}

	// Create scorecard template
	s.Require().NoError(s.DB().Create(&model.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id",
		Revision:     "1",
		Title:        "Test Template",
		Template:     `{"criteria":[{"type":"numeric-radios","weight":1,"settings":{"range":{"max":5,"min":1}},"identifier":"c1","displayName":"Criterion"}]}`,
		UsecaseIds:   []string{"usecase_id"},
		QaTaskConfig: sql.NullString{},
		Audience:     sql.NullString{},
	}).Error)

	// Create scorecard template
	s.Require().NoError(s.DB().Create(&model.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id_2",
		Revision:     "1",
		Title:        "Test Template",
		Template:     `{"criteria":[{"type":"numeric-radios","weight":1,"settings":{"range":{"max":5,"min":1}},"identifier":"c1","displayName":"Criterion"}]}`,
		UsecaseIds:   []string{"usecase_id_2"},
		QaTaskConfig: sql.NullString{},
		Audience:     sql.NullString{},
	}).Error)

	// Create a template without any task setup
	s.Require().NoError(s.DB().Create(&model.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "no_task_template_id",
		Revision:     "1",
		Title:        "No Task Template",
		Template:     `{"criteria":[{"type":"numeric-radios","weight":1,"settings":{"range":{"max":5,"min":1}},"identifier":"c1","displayName":"Criterion"}]}`,
		UsecaseIds:   []string{"usecase_id"},
		QaTaskConfig: sql.NullString{},
		Audience:     sql.NullString{},
	}).Error)

	// Create an inactive template
	s.Require().NoError(s.DB().Create(&model.ScorecardTemplates{
		Customer:      s.customerID,
		Profile:       s.profileID,
		ResourceID:    "inactive_template_id",
		Revision:      "1",
		Title:         "Inactive Template",
		Template:      `{"criteria":[{"type":"numeric-radios","weight":1,"settings":{"range":{"max":5,"min":1}},"identifier":"c1","displayName":"Criterion"}]}`,
		UsecaseIds:    []string{"usecase_id"},
		QaTaskConfig:  sql.NullString{},
		Audience:      sql.NullString{},
		DeactivatedAt: converter.ConvertToNullTime(time.Now()),
		Status:        coachingpb.ScorecardTemplateStatus_SCORECARD_TEMPLATE_STATUS_INACTIVE,
	}).Error)

	// Create QM tasks
	s.Require().NoError(s.DB().Create(&model.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{String: "usecase_id", Valid: true},
		TaskID:     "qm_task_1",
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{
			Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
					ScorecardTemplateId: "scorecard_template_id",
				},
			},
		},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{
			Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 10,
				},
			},
		},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{
			Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{
							QaAnalystUserId:           "analyst_id_1",
							AssignedConversationCount: 10,
						},
					},
				},
			},
		},
	}).Error)

	// Create an archived task
	s.Require().NoError(s.DB().Create(&model.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{String: "usecase_id", Valid: true},
		TaskID:     "qm_task_archived",
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ARCHIVED,
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{
			Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
					ScorecardTemplateId: "scorecard_template_id",
				},
			},
		},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{
			Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 5,
				},
			},
		},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{
			Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{
							QaAnalystUserId:           "analyst_id_1",
							AssignedConversationCount: 5,
						},
					},
				},
			},
		},
	}).Error)

	// Create an archived task with status changed time
	s.Require().NoError(s.DB().Create(&model.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{String: "usecase_id_2", Valid: true},
		TaskID:     "qm_task_archived_2",
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ARCHIVED,
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{
			Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
					ScorecardTemplateId: "scorecard_template_id_2",
				},
			},
		},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{
			Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 5,
				},
			},
		},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{
			Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{
							QaAnalystUserId:           "analyst_id_3",
							AssignedConversationCount: 5,
						},
					},
				},
			},
		},
		LastStatusChangedAt: sql.NullTime{Time: time.Date(2022, 10, 15, 0, 0, 0, 0, time.UTC), Valid: true},
	}).Error)

	// Create a deleted task
	s.Require().NoError(s.DB().Create(&model.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{String: "usecase_id", Valid: true},
		TaskID:     "qm_task_2",
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_DELETED,
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{
			Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
					ScorecardTemplateId: "scorecard_template_id_2",
				},
			},
		},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{
			Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 10,
				},
			},
		},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{
			Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{
							QaAnalystUserId:           "analyst_id_1",
							AssignedConversationCount: 10,
						},
					},
				},
			},
		},
	}).Error)

	// Create a task for the inactive template
	s.Require().NoError(s.DB().Create(&model.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{String: "usecase_id", Valid: true},
		TaskID:     "qm_task_3",
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{
			Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
					ScorecardTemplateId: "inactive_template_id",
				},
			},
		},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{
			Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 10,
				},
			},
		},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{
			Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{
							QaAnalystUserId:           "analyst_id_1",
							AssignedConversationCount: 10,
						},
					},
				},
			},
		},
	}).Error)

	// Create scorecards
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_1",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_1"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 5, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 6, 12, 0, 0, 0, time.UTC), Valid: true},
		PublishedAt:           sql.NullTime{Time: time.Date(2023, 8, 6, 14, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.85, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		Comment:               sql.NullString{String: "needs improvement", Valid: true},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Score with overwritten criteria for scorecard_1 (task path)
	s.Require().NoError(s.DB().Create(&model.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_task_1",
		ScorecardID:         "scorecard_1",
		CriterionIdentifier: "c1",
		NumericValue:        sql.NullFloat64{Float64: 4, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 2, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}).Error)

	// Calibration scorecard for scorecard_1: analyst_id_2 calibrates analyst_id_1's work
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "calibration_scorecard_1",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_1"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_2", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 5, 14, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.85, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{String: "scorecard_1", Valid: true},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Score for calibration scorecard — same criterion and value as scorecard_1 → 100% consistency
	s.Require().NoError(s.DB().Create(&model.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_calibration_1",
		ScorecardID:         "calibration_scorecard_1",
		CriterionIdentifier: "c1",
		NumericValue:        sql.NullFloat64{Float64: 4, Valid: true},
	}).Error)

	// Scorecard for archived task
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_archived_1",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_archived"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 12, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 13, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.88, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_2",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_1"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 11, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.92, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Create a scorecard without any task for the template has a task setup
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_3",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "1",
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 11, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.92, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Create a scorecard for the template has no task setup (with comment for export stats)
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_4",
		TemplateID:            "no_task_template_id",
		TemplateRevision:      "1",
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 15, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 16, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.95, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		Comment:               sql.NullString{String: "good job", Valid: true},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Create a score with overwritten criteria for scorecard_4 (no-task template)
	s.Require().NoError(s.DB().Create(&model.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_notask_1",
		ScorecardID:         "scorecard_4",
		CriterionIdentifier: "c1",
		NumericValue:        sql.NullFloat64{Float64: 3, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 5, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}).Error)

	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_5",
		TemplateID:            "no_task_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{shared.NoTask},
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 15, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 16, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.95, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_6",
		TemplateID:            "no_task_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{},
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 15, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 16, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.95, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Create a scorecard for the inactive template
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_7",
		TemplateID:            "inactive_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_2"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2023, 8, 20, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2023, 8, 21, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.98, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id", Valid: true},
		ResourceID:            "scorecard_8",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "1",
		TaskIds:               []string{},
		SubmitterUserID:       sql.NullString{String: "analyst_id_2", Valid: true},
		AgentUserID:           "agent_id_1",
		SubmittedAt:           sql.NullTime{Time: time.Date(2024, 8, 15, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2024, 8, 16, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.99, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Create scorecards for the archived task with last status changed time
	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id_2", Valid: true},
		ResourceID:            "scorecard_9",
		TemplateID:            "scorecard_template_id_2",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_archived_2"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_3", Valid: true},
		AgentUserID:           "agent_id_2",
		SubmittedAt:           sql.NullTime{Time: time.Date(2022, 10, 20, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2022, 10, 21, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.98, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		UsecaseID:             sql.NullString{String: "usecase_id_2", Valid: true},
		ResourceID:            "scorecard_10",
		TemplateID:            "scorecard_template_id_2",
		TemplateRevision:      "1",
		TaskIds:               []string{"qm_task_archived_2"},
		SubmitterUserID:       sql.NullString{String: "analyst_id_3", Valid: true},
		AgentUserID:           "agent_id_2",
		SubmittedAt:           sql.NullTime{Time: time.Date(2022, 11, 5, 12, 0, 0, 0, time.UTC), Valid: true},
		AcknowledgedAt:        sql.NullTime{Time: time.Date(2022, 11, 6, 12, 0, 0, 0, time.UTC), Valid: true},
		Score:                 sql.NullFloat64{Float64: 0.5, Valid: true},
		ManuallyScored:        sql.NullBool{Bool: true, Valid: true},
		CalibratedScorecardID: sql.NullString{},
		SubmissionSource:      sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)

	// Create session logs for skip suggested conversation tracking
	templateResourceName := coachingpb.ScorecardTemplateName{
		CustomerID:                  s.customerID,
		ProfileID:                   s.profileID,
		ScorecardTemplateID:         "scorecard_template_id",
		ScorecardTemplateRevisionID: "1",
	}.String()
	userResourceName := userpb.UserName{CustomerID: s.customerID, UserID: "analyst_id_1"}.String()
	for i, ts := range []time.Time{
		time.Date(2023, 8, 3, 10, 0, 0, 0, time.UTC),
		time.Date(2023, 8, 7, 14, 0, 0, 0, time.UTC),
	} {
		s.Require().NoError(s.DB().Create(&model.SessionLogs{
			CustomerID:   s.customerID,
			ProfileID:    s.profileID,
			SessionID:    fmt.Sprintf("session_skip_%d", i),
			SessionLogID: fmt.Sprintf("log_skip_%d", i),
			Type:         sessionpb.SessionLog_TYPE_DIRECTOR_BUTTON_CLICK,
			Payload: dbsessionpb.SessionLog_LogPayloadSQL{Message: &dbsessionpb.SessionLog_LogPayload{
				Payload: &dbsessionpb.SessionLog_LogPayload_DirectorButtonClickPayload{
					DirectorButtonClickPayload: &sessionpb.DirectorButtonClickPayload{
						DirectorButton:                sessionpb.DirectorButtonClickPayload_DIRECTOR_BUTTON_QM_FOCUS_VIEW_SKIP,
						ScorecardTemplateResourceName: templateResourceName,
						UserResourceName:              userResourceName,
					},
				},
			}},
			CreateTime: ts,
		}).Error)
	}

	// Skip by analyst_id_2 with no scorecard on qm_task_1 in August — must not be counted in export stats.
	analyst2ResourceName := userpb.UserName{CustomerID: s.customerID, UserID: "analyst_id_2"}.String()
	s.Require().NoError(s.DB().Create(&model.SessionLogs{
		CustomerID:   s.customerID,
		ProfileID:    s.profileID,
		SessionID:    "session_skip_no_scorecard",
		SessionLogID: "log_skip_no_scorecard",
		Type:         sessionpb.SessionLog_TYPE_DIRECTOR_BUTTON_CLICK,
		Payload: dbsessionpb.SessionLog_LogPayloadSQL{Message: &dbsessionpb.SessionLog_LogPayload{
			Payload: &dbsessionpb.SessionLog_LogPayload_DirectorButtonClickPayload{
				DirectorButtonClickPayload: &sessionpb.DirectorButtonClickPayload{
					DirectorButton:                sessionpb.DirectorButtonClickPayload_DIRECTOR_BUTTON_QM_FOCUS_VIEW_SKIP,
					ScorecardTemplateResourceName: templateResourceName,
					UserResourceName:              analyst2ResourceName,
				},
			},
		}},
		CreateTime: time.Date(2023, 8, 8, 10, 0, 0, 0, time.UTC),
	}).Error)

	// Create a session log skip for no_task_template_id
	noTaskTemplateResourceName := coachingpb.ScorecardTemplateName{
		CustomerID:                  s.customerID,
		ProfileID:                   s.profileID,
		ScorecardTemplateID:         "no_task_template_id",
		ScorecardTemplateRevisionID: "1",
	}.String()
	s.Require().NoError(s.DB().Create(&model.SessionLogs{
		CustomerID:   s.customerID,
		ProfileID:    s.profileID,
		SessionID:    "session_skip_notask",
		SessionLogID: "log_skip_notask",
		Type:         sessionpb.SessionLog_TYPE_DIRECTOR_BUTTON_CLICK,
		Payload: dbsessionpb.SessionLog_LogPayloadSQL{Message: &dbsessionpb.SessionLog_LogPayload{
			Payload: &dbsessionpb.SessionLog_LogPayload_DirectorButtonClickPayload{
				DirectorButtonClickPayload: &sessionpb.DirectorButtonClickPayload{
					DirectorButton:                sessionpb.DirectorButtonClickPayload_DIRECTOR_BUTTON_QM_FOCUS_VIEW_SKIP,
					ScorecardTemplateResourceName: noTaskTemplateResourceName,
					UserResourceName:              userResourceName,
				},
			},
		}},
		CreateTime: time.Date(2023, 9, 5, 10, 0, 0, 0, time.UTC),
	}).Error)

	// --- Test data for TestEvaluationPeriodProgressStatusDerivedFromCompletionRate ---
	// Use a deterministic start anchored to start-of-today minus 15 days, so both
	// SetupSuite and the test function derive identical values independently.
	// Production code calls time.Now() internally, so the period must contain "now"
	// and progress must be well above CompletionRate (1%) for BEHIND_SCHEDULE.
	now := time.Now().UTC()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	currentPeriodStart := startOfToday.AddDate(0, 0, -15)
	currentPeriodMid := currentPeriodStart.AddDate(0, 0, 2)

	s.Require().NoError(s.DB().Create(&model.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "current_period_template_id",
		Revision:   "1",
		Title:      "Current Period Template",
		Template:   `{"criteria":[{"type":"numeric-radios","weight":1,"settings":{"range":{"max":5,"min":1}},"identifier":"c1","displayName":"Criterion"}]}`,
		UsecaseIds: []string{"usecase_id_progress"},
		Status:     coachingpb.ScorecardTemplateStatus_SCORECARD_TEMPLATE_STATUS_ACTIVE,
	}).Error)

	s.Require().NoError(s.DB().Create(&model.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{String: "usecase_id_progress", Valid: true},
		TaskID:     "qm_task_current_period",
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{
			Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
					ScorecardTemplateId: "current_period_template_id",
				},
			},
		},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(currentPeriodStart),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{
			Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 100,
				},
			},
		},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{
			Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{
							QaAnalystUserId:           "analyst_id_1",
							AssignedConversationCount: 100,
						},
					},
				},
			},
		},
	}).Error)

	s.Require().NoError(s.DB().Create(&model.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		UsecaseID:        sql.NullString{String: "usecase_id_progress", Valid: true},
		ResourceID:       "scorecard_current_period",
		TemplateID:       "current_period_template_id",
		TemplateRevision: "1",
		TaskIds:          []string{"qm_task_current_period"},
		SubmitterUserID:  sql.NullString{String: "analyst_id_1", Valid: true},
		AgentUserID:      "agent_id_1",
		SubmittedAt:      sql.NullTime{Time: currentPeriodMid, Valid: true},
		AcknowledgedAt:   sql.NullTime{Time: currentPeriodMid.Add(time.Hour), Valid: true},
		Score:            sql.NullFloat64{Float64: 0.80, Valid: true},
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmissionSource: sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
	}).Error)
}

func (s *RetrieveQMTaskStatsTestSuite) TestRetrieveQMTaskStats() {
	// Calculate expected periods based on task configuration
	task := &model.Tasks{
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{
			Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			},
		},
	}
	qaTaskConfig := &qadbpb.QATaskConfig{
		EvaluationPeriodConfig: task.TaskScheduleConfig.Message.GetQmTaskScheduleConfig(),
	}
	expectedPeriods, err := qa.ListEvaluationPeriods([]*qadbpb.QATaskConfig{qaTaskConfig}, nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(expectedPeriods)

	tests := []struct {
		name          string
		request       *analyticspb.RetrieveDirectorTaskStatsRequest
		expectedError error
	}{
		{
			name: "success",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			// Pairs with the "scoring activity included" case below to lock in the
			// include_export_stats gate documented on
			// RetrieveDirectorTaskStatsRequest.include_export_stats.
			name: "scoring activity excluded by default",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
				// IncludeExportStats left unset (false) — the page-load shape.
			},
		},
		{
			name: "scoring activity included when requested",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
				IncludeExportStats: true,
			},
		},
		{
			name: "export stats included for no-task template",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNamesWithNoTask: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "no_task_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
				IncludeExportStats: true,
			},
		},
		{
			name: "success archived only",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ARCHIVED),
				},
			},
		},
		{
			name: "success active and archived",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ARCHIVED),
				},
			},
		},
		{
			name: "success filter with template including no task",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
					ScorecardTemplateNamesWithNoTask: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "success filter with only template",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "success filter with only no task template",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					ScorecardTemplateNamesWithNoTask: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "filter by task name, agent users and submission sources",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
					Users: []*userpb.User{
						{Name: "customers/" + s.customerID + "/users/agent_id_1"},
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "success with partial stats has local timezone earlier than period timezone",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 5, 4, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 11, 2, 3, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "America/New_York",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "success with partial periods only for local timezone earlier than period timezone",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 5, 4, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 9, 15, 3, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "America/New_York",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "success with partial stats has local timezone later than period timezone",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 4, 15, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 11, 2, 14, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "Asia/Tokyo",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "success with partial periods only for local timezone later than period timezone",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 4, 15, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 9, 14, 14, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "Asia/Tokyo",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "invalid group by",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
			expectedError: status.Errorf(codes.InvalidArgument, "invalid group_by_attributes: [%v]", analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK),
		},
		{
			name: "group by time range only",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "group by time range weekly",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "group by time range filter with template including no task",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
					ScorecardTemplateNamesWithNoTask: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "group by time range filter with only no task template",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					ScorecardTemplateNamesWithNoTask: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "group by time range filter with only template",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "usecase with no tasks or templates",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "empty_usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "usecase with no tasks or templates - group by time range only",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "empty_usecase_id",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "group by time range only, filter by task name, agent users and submission sources",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_1",
					}.String()},
					Users: []*userpb.User{
						{Name: "customers/" + s.customerID + "/users/agent_id_1"},
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "template without task setup",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "no_task_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "group by time range only, template without task setup",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "no_task_template_id",
						ScorecardTemplateRevisionID: "*",
					}.String()},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
		{
			name: "filter by scorecard reviewer",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id",
						ScorecardTemplateRevisionID: "1",
					}.String()},
					ScorecardReviewerAudience: &analyticspb.UserAudience{
						Users: []string{"customers/customer_id/users/analyst_id_2"},
					},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
			},
		},
		{
			name: "test for archived task with last status changed time",
			request: &analyticspb.RetrieveDirectorTaskStatsRequest{
				Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 12, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{customerpb.UsecaseName{
						CustomerID: s.customerID,
						ProfileID:  s.profileID,
						UsecaseID:  "usecase_id_2",
					}.String()},
					ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
						CustomerID:                  s.customerID,
						ProfileID:                   s.profileID,
						ScorecardTemplateID:         "scorecard_template_id_2",
						ScorecardTemplateRevisionID: "*",
					}.String()},
					TaskNames: []string{coachingpb.DirectorTaskName{
						CustomerID:     s.customerID,
						ProfileID:      s.profileID,
						DirectorTaskID: "qm_task_archived_2",
					}.String()},
					Users: []*userpb.User{
						{Name: "customers/" + s.customerID + "/users/agent_id_2"},
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Frequency: analyticspb.Frequency_EVALUATION_PERIOD,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UTC",
				},
				TaskStatus: []int32{
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ARCHIVED),
					int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
				},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			resp, err := s.analyticsService.RetrieveQMTaskStats(context.Background(), tt.request)
			if tt.expectedError != nil {
				s.Error(err)
				s.Equal(tt.expectedError.Error(), err.Error())
				return
			}
			s.NoError(err)

			if tt.name == "success" {
				// Verify we have stats for both templates (with and without task)
				s.Equal(5, len(resp.DirectorTaskStats), "Expected 5 director task stats but got %v", len(resp.DirectorTaskStats))

				// Find the stats for each template
				for _, stat := range resp.DirectorTaskStats {
					if len(stat.Attribute.TaskNames) > 0 {
						// Verify stats for template with task
						s.NotNil(stat, "Expected to find stats for template with task")
						s.Len(stat.Attribute.ScorecardTemplateNames, 1)
						s.Len(stat.Attribute.TaskNames, 1)
						s.Len(stat.Attribute.TimeRanges, 1)

						// Check if this is the period with task (August 2023)
						startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
						if startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)) {
							s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
							s.Equal(int32(10), stat.QmTaskStats.AssignedScorecardCount)
							s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
							s.InDelta(float64(0.885), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
							s.InDelta(float64(0.2), float64(stat.QmTaskStats.CompletionRate), 0.001)
						} else {
							// For other periods, only assigned count is set
							s.Equal(int32(10), stat.QmTaskStats.AssignedScorecardCount)
							s.Equal(float32(-1), stat.QmTaskStats.AvgPerfScore)
						}
					} else {
						// Verify stats for template without task
						s.NotNil(stat, "Expected to find stats for template without task")
						s.Len(stat.Attribute.ScorecardTemplateNames, 1)
						s.Len(stat.Attribute.TaskNames, 0)
						s.Len(stat.Attribute.TimeRanges, 1)

						// Check if this is the no_task_template_id
						if strings.Contains(stat.Attribute.ScorecardTemplateNames[0], "no_task_template_id") {
							s.Equal(int32(3), stat.QmTaskStats.EvalueatedScrecardCount)
							s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
							s.Equal(int32(3), stat.QmTaskStats.AcknowledgedScorecardCount)
							s.InDelta(float64(0.95), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
							s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
						} else {
							// For the regular template without task
							s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
							s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
							s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
							s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
							s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
						}
					}
				}
				return
			}

			if tt.name == "scoring activity excluded by default" {
				// Without IncludeExportStats, every QMTaskStats must come back
				// with the export-only fields cleared, regardless of the
				// underlying scorecard data. The proto contract on
				// RetrieveDirectorTaskStatsRequest.include_export_stats
				// promises clients these fields are omitted from the response.
				s.NotEmpty(resp.DirectorTaskStats)
				for _, stat := range resp.DirectorTaskStats {
					s.Equal(int32(0), stat.QmTaskStats.OverwrittenCriteriaCount,
						"OverwrittenCriteriaCount must be zero when IncludeExportStats=false")
					s.Equal(int32(0), stat.QmTaskStats.CommentedScorecardCount,
						"CommentedScorecardCount must be zero when IncludeExportStats=false")
					s.Equal(int32(0), stat.QmTaskStats.CalibratedScorecardCount,
						"CalibratedScorecardCount must be zero when IncludeExportStats=false")
					s.Equal(float32(-1.0), stat.QmTaskStats.AvgConsistencyScore,
						"AvgConsistencyScore must default to -1 when IncludeExportStats=false")
					s.Equal(int32(0), stat.QmTaskStats.SkipSuggestedConversationCount,
						"SkipSuggestedConversationCount must be zero when IncludeExportStats=false")
					s.Equal(analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_UNSPECIFIED,
						stat.QmTaskStats.EvaluationPeriodProgressStatus,
						"EvaluationPeriodProgressStatus must be UNSPECIFIED when IncludeExportStats=false")
				}
				return
			}

			if tt.name == "scoring activity included when requested" {
				s.NotEmpty(resp.DirectorTaskStats)
				for _, stat := range resp.DirectorTaskStats {
					if len(stat.Attribute.TaskNames) == 1 && strings.Contains(stat.Attribute.TaskNames[0], "qm_task_1") {
						startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
						if startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)) {
							// scorecard_1 has a comment
							s.Equal(int32(1), stat.QmTaskStats.CommentedScorecardCount,
								"CommentedScorecardCount: scorecard_1 has a comment")
							// scorecard_1 has one overwritten criterion (score_task_1)
							s.Equal(int32(1), stat.QmTaskStats.OverwrittenCriteriaCount,
								"OverwrittenCriteriaCount: scorecard_1 has one overwritten criterion")
							// Two analyst_id_1 skips count; analyst_id_2 skip without a scorecard is excluded
							s.Equal(int32(2), stat.QmTaskStats.SkipSuggestedConversationCount,
								"SkipSuggestedConversationCount must reflect session log seed data")
							// scorecard_1 is calibrated by calibration_scorecard_1
							s.Equal(int32(1), stat.QmTaskStats.CalibratedScorecardCount,
								"CalibratedScorecardCount: scorecard_1 has a calibration scorecard")
							// Both scorecards gave criterion c1 the same value (4) → 100% consistency
							s.InDelta(100.0, float64(stat.QmTaskStats.AvgConsistencyScore), 0.01,
								"AvgConsistencyScore: identical scores on c1 → 100%% consistency")
						}
					}
				}
				return
			}

			if tt.name == "export stats included for no-task template" {
				s.NotEmpty(resp.DirectorTaskStats)
				for _, stat := range resp.DirectorTaskStats {
					// No-task stats have no TaskNames
					if len(stat.Attribute.TaskNames) == 0 {
						s.Equal(int32(1), stat.QmTaskStats.CommentedScorecardCount,
							"CommentedScorecardCount: scorecard_4 has a comment")
						s.Equal(int32(1), stat.QmTaskStats.OverwrittenCriteriaCount,
							"OverwrittenCriteriaCount: scorecard_4 has one overwritten criterion")
						s.Equal(int32(1), stat.QmTaskStats.SkipSuggestedConversationCount,
							"SkipSuggestedConversationCount: one skip event for no_task_template_id")
						s.Equal(int32(0), stat.QmTaskStats.CalibratedScorecardCount,
							"CalibratedScorecardCount: no calibration data for no-task template")
						s.Equal(float32(-1.0), stat.QmTaskStats.AvgConsistencyScore,
							"AvgConsistencyScore: no calibrated scorecards → -1")
					}
				}
				return
			}

			if tt.name == "success archived only" {
				s.NotEmpty(resp.DirectorTaskStats)
				foundArchived := false
				for _, stat := range resp.DirectorTaskStats {
					if len(stat.Attribute.TaskNames) == 1 && strings.Contains(stat.Attribute.TaskNames[0], "qm_task_archived") {
						foundArchived = true
						// August period includes one evaluated and acknowledged scorecard from archived task
						startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
						if startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)) {
							s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
							s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
							s.InDelta(float64(0.88), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						}
					}
				}
				s.True(foundArchived, "Expected archived task stats present")
				return
			}

			if tt.name == "success active and archived" {
				s.NotEmpty(resp.DirectorTaskStats)
				foundActive := false
				foundArchived := false
				for _, stat := range resp.DirectorTaskStats {
					if len(stat.Attribute.TaskNames) == 1 {
						if strings.Contains(stat.Attribute.TaskNames[0], "qm_task_1") {
							foundActive = true
						}
						if strings.Contains(stat.Attribute.TaskNames[0], "qm_task_archived") {
							foundArchived = true
						}
					}
				}
				s.True(foundActive, "Expected active task stats present")
				s.True(foundArchived, "Expected archived task stats present")
				return
			}

			if tt.name == "success with partial stats has local timezone earlier than period timezone" {
				fmt.Println("resp here", resp)
				s.Equal(4, len(resp.DirectorTaskStats), "Expected 4 director task stats but got %v", len(resp.DirectorTaskStats))
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 8, 5, 4, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 8, 31, 3, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.Equal(int32(-1), stat.QmTaskStats.AssignedScorecardCount)
						s.InDelta(float64(0.885), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 10, 31, 23, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 11, 1, 4, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 11, 2, 3, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					default:
						s.Fail("Unexpected start time", startTime)
					}
				}
				return
			}

			if tt.name == "success with partial stats has local timezone later than period timezone" {
				fmt.Println("resp here", resp)
				s.Equal(4, len(resp.DirectorTaskStats), "Expected 4 director task stats but got %v", len(resp.DirectorTaskStats))
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 8, 4, 15, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 8, 31, 14, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.Equal(int32(-1), stat.QmTaskStats.AssignedScorecardCount)
						s.InDelta(float64(0.885), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 10, 31, 23, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 11, 1, 15, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 11, 2, 14, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					default:
						s.Fail("Unexpected start time", startTime)
					}
				}
				return
			}

			if tt.name == "group by time range only" {
				// Verify we have stats for each day in August
				s.Equal(3, len(resp.DirectorTaskStats), "Expected 3 days of stats but got %v", len(resp.DirectorTaskStats))

				// Find the days with our test data
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 8, 5, 0, 0, 0, 0, time.UTC):
						// Verify Aug 5 stats
						s.NotNil(stat, "Expected to find stats for Aug 5")
						s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.85), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					case time.Date(2023, 8, 10, 0, 0, 0, 0, time.UTC):
						// Verify Aug 10 stats
						s.NotNil(stat, "Expected to find stats for Aug 10")
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					case time.Date(2023, 8, 15, 0, 0, 0, 0, time.UTC):
						// Verify Aug 15 stats (template without task)
						s.NotNil(stat, "Expected to find stats for Aug 15")
						s.Equal(int32(3), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(3), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.95), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					default:
						s.Fail("Unexpected start time", startTime)
					}
				}
				// Verify total stats
				s.NotNil(resp.TotalDirectorTaskStats)
				s.NotNil(resp.TotalDirectorTaskStats.QmTaskStats)
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(6), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(6), totalStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.9233333468437195), float64(totalStats.AvgPerfScore), 0.001)
				s.InDelta(float64(-1), float64(totalStats.CompletionRate), 0.001)
				return
			}

			if tt.name == "group by time range weekly" {
				s.Equal(3, len(resp.DirectorTaskStats), "Expected 3 weeks of stats but got %v", len(resp.DirectorTaskStats))

				// Find the weeks with our test data
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC): // First week of August (starts on Sunday)
						// Verify first week stats (Aug 5)
						s.NotNil(stat, "Expected to find stats for first week of August")
						s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.85), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					case time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC): // Second week of August
						// Verify second week stats (Aug 10)
						s.NotNil(stat, "Expected to find stats for second week of August")
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					case time.Date(2023, 8, 13, 0, 0, 0, 0, time.UTC): // Third week of August
						// Verify third week stats (Aug 15)
						s.NotNil(stat, "Expected to find stats for third week of August")
						s.Equal(int32(3), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(3), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.95), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					default:
						s.Fail("Unexpected start time", startTime)
					}
				}
				// Verify total stats
				s.NotNil(resp.TotalDirectorTaskStats)
				s.NotNil(resp.TotalDirectorTaskStats.QmTaskStats)
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(6), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(6), totalStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.9233333468437195), float64(totalStats.AvgPerfScore), 0.001)
				s.InDelta(float64(-1), float64(totalStats.CompletionRate), 0.001)
				return
			}

			if tt.name == "group by time range only, filter by task name, agent users and submission sources" {
				// Verify we have stats for each day in August
				s.Equal(2, len(resp.DirectorTaskStats), "Expected 2 days of stats but got %v", len(resp.DirectorTaskStats))

				// Find the days with our test data
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					if startTime.Equal(time.Date(2023, 8, 5, 0, 0, 0, 0, time.UTC)) {
						// Verify Aug 5 stats
						s.NotNil(stat, "Expected to find stats for Aug 5")
						s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.85), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					} else if startTime.Equal(time.Date(2023, 8, 10, 0, 0, 0, 0, time.UTC)) {
						// Verify Aug 10 stats
						s.NotNil(stat, "Expected to find stats for Aug 10")
						s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					}
				}
				// Verify total stats
				s.NotNil(resp.TotalDirectorTaskStats)
				s.NotNil(resp.TotalDirectorTaskStats.QmTaskStats)
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(2), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(2), totalStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.885), float64(totalStats.AvgPerfScore), 0.001)
				s.InDelta(float64(-1), float64(totalStats.CompletionRate), 0.001)
				return
			}

			if tt.name == "template without task setup" {
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 director task stats but got %v", len(resp.DirectorTaskStats))

				// Verify the stats for the template without task setup
				stat := resp.DirectorTaskStats[0]
				s.NotNil(stat)
				s.NotNil(stat.Attribute)
				s.Len(stat.Attribute.ScorecardTemplateNames, 1)
				s.Len(stat.Attribute.TaskNames, 0)
				s.Len(stat.Attribute.TimeRanges, 1)

				// Verify stats
				s.Equal(int32(3), stat.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
				s.Equal(int32(3), stat.QmTaskStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.95), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
				s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
				return
			}

			if tt.name == "success filter with template including no task" {
				// Verify we have stats for both templates (with and without task)
				s.Equal(4, len(resp.DirectorTaskStats), "Expected 4 director task stats but got %v", len(resp.DirectorTaskStats))

				// Find the stats for each template
				for _, stat := range resp.DirectorTaskStats {
					if len(stat.Attribute.TaskNames) > 0 {
						// Verify stats for template with task
						s.NotNil(stat, "Expected to find stats for template with task")
						s.Len(stat.Attribute.ScorecardTemplateNames, 1)
						s.Len(stat.Attribute.TaskNames, 1)
						s.Len(stat.Attribute.TimeRanges, 1)

						// Check if this is the period with task (August 2023)
						startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
						if startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)) {
							s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
							s.Equal(int32(10), stat.QmTaskStats.AssignedScorecardCount)
							s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
							s.InDelta(float64(0.885), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
							s.InDelta(float64(0.2), float64(stat.QmTaskStats.CompletionRate), 0.001)
						} else {
							// For other periods, only assigned count is set
							s.Equal(int32(10), stat.QmTaskStats.AssignedScorecardCount)
							s.Equal(float32(-1), stat.QmTaskStats.AvgPerfScore)
						}
					} else {
						// Verify stats for template without task
						s.NotNil(stat, "Expected to find stats for template without task")
						s.Len(stat.Attribute.ScorecardTemplateNames, 1)
						s.Len(stat.Attribute.TaskNames, 0)
						s.Len(stat.Attribute.TimeRanges, 1)

						// For the regular template without task
						s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
					}
				}
				return
			}

			if tt.name == "group by time range filter with template including no task" {
				s.Equal(2, len(resp.DirectorTaskStats), "Expected 2 weeks of stats but got %v", len(resp.DirectorTaskStats))
				// Find the weeks with our test data
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC): // First week of August (starts on Sunday)
						// Verify first week stats (Aug 5)
						s.NotNil(stat, "Expected to find stats for first week of August")
						s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.85), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					case time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC): // Second week of August
						// Verify second week stats for scorecard with and without task (Aug 10)
						s.NotNil(stat, "Expected to find stats for second week of August")
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
					default:
						s.Fail("Unexpected start time", startTime)
					}
				}
				// Verify total stats
				s.NotNil(resp.TotalDirectorTaskStats)
				s.NotNil(resp.TotalDirectorTaskStats.QmTaskStats)
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(3), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(3), totalStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.897), float64(totalStats.AvgPerfScore), 0.001)
				s.InDelta(float64(-1), float64(totalStats.CompletionRate), 0.001)
				return
			}

			if tt.name == "success filter with only no task template" {
				// Verify we have stats for each week
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 week of stats but got %v", len(resp.DirectorTaskStats))

				// Verify the stats for the week with our test data
				stat := resp.DirectorTaskStats[0]
				s.NotNil(stat, "Expected to find stats for the week")
				s.Len(stat.Attribute.ScorecardTemplateNames, 1)
				s.Len(stat.Attribute.TaskNames, 0)
				s.Len(stat.Attribute.TimeRanges, 1)

				// Verify the time range is request start time and end time
				startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
				endTime := stat.Attribute.TimeRanges[0].EndTimestamp.AsTime()
				s.True(startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)), "Expected starting Aug 1")
				s.True(endTime.Equal(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)), "Expected ending Oct 31")

				// Verify stats
				s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
				s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
				s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
				return
			}

			if tt.name == "group by time range filter with only no task template" {
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 week of stats but got %v", len(resp.DirectorTaskStats))

				stat := resp.DirectorTaskStats[0]
				// Verify first week stats (Aug 5)
				startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
				s.True(startTime.Equal(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)), "Expected starting Aug ")
				// Verify stats
				s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
				s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
				s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
				// Verify total stats
				s.NotNil(resp.TotalDirectorTaskStats)
				s.NotNil(resp.TotalDirectorTaskStats.QmTaskStats)
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(1), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(1), totalStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.92), float64(totalStats.AvgPerfScore), 0.001)
				s.InDelta(float64(-1), float64(totalStats.CompletionRate), 0.001)
				return
			}

			if tt.name == "success filter with only template" {
				// Verify we have stats for each week
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 week of stats but got %v", len(resp.DirectorTaskStats))

				// Verify the stats for the week with our test data
				stat := resp.DirectorTaskStats[0]
				s.NotNil(stat, "Expected to find stats for the week")
				s.Len(stat.Attribute.ScorecardTemplateNames, 1)
				s.Len(stat.Attribute.TaskNames, 0)
				s.Len(stat.Attribute.TimeRanges, 1)

				// Verify the time range is request start time and end time
				startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
				endTime := stat.Attribute.TimeRanges[0].EndTimestamp.AsTime()
				s.True(startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)), "Expected starting Aug 1")
				s.True(endTime.Equal(time.Date(2023, 10, 31, 23, 59, 59, 999999999, time.UTC)), "Expected ending Oct 31")

				// Verify stats
				s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
				s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
				s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
				return
			}

			if tt.name == "group by time range filter with only template" {
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 week of stats but got %v", len(resp.DirectorTaskStats))

				stat := resp.DirectorTaskStats[0]
				// Verify first week stats (Aug 5)
				startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
				s.True(startTime.Equal(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)), "Expected starting Aug ")
				// Verify stats
				s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(0), stat.QmTaskStats.AssignedScorecardCount)
				s.Equal(int32(1), stat.QmTaskStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.92), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
				s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
				// Verify total stats
				s.NotNil(resp.TotalDirectorTaskStats)
				s.NotNil(resp.TotalDirectorTaskStats.QmTaskStats)
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(1), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(1), totalStats.AcknowledgedScorecardCount)
				s.InDelta(float64(0.92), float64(totalStats.AvgPerfScore), 0.001)
				s.InDelta(float64(-1), float64(totalStats.CompletionRate), 0.001)
				return
			}

			if tt.name == "usecase with no tasks or templates" {
				s.Empty(resp.DirectorTaskStats, "Expected empty director task stats for empty usecase but got %v", resp.DirectorTaskStats)
				s.Nil(resp.TotalDirectorTaskStats)
				return
			}

			if tt.name == "usecase with no tasks or templates - group by time range only" {
				totalStats := resp.TotalDirectorTaskStats.QmTaskStats
				s.Equal(int32(0), totalStats.EvalueatedScrecardCount)
				s.Equal(int32(0), totalStats.AssignedScorecardCount)
				s.Equal(int32(0), totalStats.AcknowledgedScorecardCount)
				s.Equal(float32(-1), totalStats.AvgPerfScore)
				s.Equal(float32(-1), totalStats.CompletionRate)

				s.Empty(resp.DirectorTaskStats, "Expected empty director task stats for empty usecase but got %v", resp.DirectorTaskStats)

				return
			}

			if tt.name == "success with partial periods only for local timezone earlier than period timezone" {
				s.Equal(2, len(resp.DirectorTaskStats), "Expected 2 director task stats but got %v", len(resp.DirectorTaskStats))
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 8, 5, 4, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 8, 31, 3, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.Equal(int32(-1), stat.QmTaskStats.AssignedScorecardCount)
						s.InDelta(float64(0.885), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 8, 31, 4, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 9, 15, 3, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(int32(0), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.Equal(int32(-1), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(float32(-1), stat.QmTaskStats.AvgPerfScore)
						s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					default:
						s.Fail("Unexpected start time", startTime)
					}
				}
				return
			}

			if tt.name == "success with partial periods only for local timezone later than period timezone" {
				s.Equal(2, len(resp.DirectorTaskStats), "Expected 2 director task stats but got %v", len(resp.DirectorTaskStats))
				for _, stat := range resp.DirectorTaskStats {
					startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
					switch startTime {
					case time.Date(2023, 8, 4, 15, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 8, 31, 14, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(int32(2), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(2), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.Equal(int32(-1), stat.QmTaskStats.AssignedScorecardCount)
						s.InDelta(float64(0.885), float64(stat.QmTaskStats.AvgPerfScore), 0.001)
						s.InDelta(float64(-1), float64(stat.QmTaskStats.CompletionRate), 0.001)
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					case time.Date(2023, 8, 31, 15, 0, 0, 0, time.UTC):
						s.Equal(time.Date(2023, 9, 14, 14, 59, 59, 0, time.UTC), stat.Attribute.TimeRanges[0].EndTimestamp.AsTime())
						s.Equal(int32(0), stat.QmTaskStats.EvalueatedScrecardCount)
						s.Equal(int32(0), stat.QmTaskStats.AcknowledgedScorecardCount)
						s.Equal(int32(-1), stat.QmTaskStats.AssignedScorecardCount)
						s.Equal(float32(-1), stat.QmTaskStats.AvgPerfScore)
						s.Equal(float32(-1), stat.QmTaskStats.CompletionRate)
						s.Equal(analyticspb.QMTaskStats_PARTIAL_PERIOD, stat.QmTaskStats.StatsPeriodType)
					default:
						s.Fail("Unexpected start time", startTime)
					}

				}
				return
			}

			if tt.name == "filter by scorecard reviewer" {
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 director task stats but got %v", len(resp.DirectorTaskStats))

				directorTask := resp.DirectorTaskStats[0]
				s.Equal(int32(1), directorTask.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(1), directorTask.QmTaskStats.AcknowledgedScorecardCount)
				s.Equal(float32(0.99), directorTask.QmTaskStats.AvgPerfScore)
			}

			if tt.name == "test for archived task with last status changed time" {
				s.Equal(1, len(resp.DirectorTaskStats), "Expected 1 director task stats but got %v", len(resp.DirectorTaskStats))
				directorTask := resp.DirectorTaskStats[0]
				s.Equal(int32(1), directorTask.QmTaskStats.EvalueatedScrecardCount)
				s.Equal(int32(1), directorTask.QmTaskStats.AcknowledgedScorecardCount)
				s.Equal(float32(0.98), directorTask.QmTaskStats.AvgPerfScore)
			}
		})
	}
}

func TestRetrieveQMTaskStats(t *testing.T) {
	suite.Run(t, &RetrieveQMTaskStatsTestSuite{
		AnalyticsV2TestSuite: newTestSuite(t),
	})
}

func (s *RetrieveQMTaskStatsTestSuite) TestGetClosestLocalTime() {
	testCases := []struct {
		name           string
		localTime      time.Time
		pivotTime      time.Time
		expectedBefore time.Time
		expectedAfter  time.Time
	}{
		{
			name:           "local timezone is earlier than pivot timezone",
			localTime:      time.Date(2023, 8, 1, 4, 0, 0, 0, time.UTC),
			pivotTime:      time.Date(2023, 8, 5, 7, 0, 0, 0, time.UTC),
			expectedBefore: time.Date(2023, 8, 5, 4, 0, 0, 0, time.UTC),
			expectedAfter:  time.Date(2023, 8, 6, 4, 0, 0, 0, time.UTC),
		},
		{
			name:           "local timezone is later than pivot timezone",
			localTime:      time.Date(2023, 8, 1, 8, 0, 0, 0, time.UTC),
			pivotTime:      time.Date(2023, 8, 5, 5, 0, 0, 0, time.UTC),
			expectedBefore: time.Date(2023, 8, 4, 8, 0, 0, 0, time.UTC),
			expectedAfter:  time.Date(2023, 8, 5, 8, 0, 0, 0, time.UTC),
		},
		{
			name:           "local timezone is same as pivot timezone",
			localTime:      time.Date(2023, 8, 1, 8, 0, 0, 0, time.UTC),
			pivotTime:      time.Date(2023, 8, 5, 8, 0, 0, 0, time.UTC),
			expectedBefore: time.Date(2023, 8, 5, 8, 0, 0, 0, time.UTC),
			expectedAfter:  time.Date(2023, 8, 5, 8, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Test getClosestLocalTimeBeforePivot
			actualBefore := getClosestLocalTimeBeforePivot(tc.localTime, tc.pivotTime)
			s.Equal(tc.expectedBefore, actualBefore, "getClosestLocalTimeBeforePivot failed")

			// Test getClosestLocalTimeAfterPivot
			actualAfter := getClosestLocalTimeAfterPivot(tc.localTime, tc.pivotTime)
			s.Equal(tc.expectedAfter, actualAfter, "getClosestLocalTimeAfterPivot failed")
		})
	}
}

func (s *RetrieveQMTaskStatsTestSuite) TestComputeEvaluationPeriodStatusUsesCorrectCompletionRate() {
	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC)
	midpoint := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC) // ~50% through the period

	testCases := []struct {
		name           string
		completionRate float32
		expectedStatus analyticspb.EvaluationPeriodProgressStatus
	}{
		{
			name:           "high completion rate is on track",
			completionRate: 0.8,
			expectedStatus: analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_ON_TRACK,
		},
		{
			name:           "low completion rate is behind schedule",
			completionRate: 0.1,
			expectedStatus: analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_BEHIND_SCHEDULE,
		},
		{
			name:           "completion rate matching progress is on track",
			completionRate: 0.5,
			expectedStatus: analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_ON_TRACK,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			status := computeEvaluationPeriodStatus(midpoint, tc.completionRate, periodStart, periodEnd)
			s.Equal(tc.expectedStatus, status,
				"completionRate=%.2f at midpoint should be %v", tc.completionRate, tc.expectedStatus)
		})
	}
}

func (s *RetrieveQMTaskStatsTestSuite) TestEvaluationPeriodProgressStatusDerivedFromCompletionRate() {
	now := time.Now().UTC()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	currentPeriodStart := startOfToday.AddDate(0, 0, -15)

	resp, err := s.analyticsService.RetrieveQMTaskStats(context.Background(), &analyticspb.RetrieveDirectorTaskStatsRequest{
		Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(currentPeriodStart),
			EndTimestamp:   timestamppb.New(currentPeriodStart.AddDate(0, 2, 0)),
		},
		FilterByAttribute: &analyticspb.Attribute{
			UsecaseNames: []string{customerpb.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  "usecase_id_progress",
			}.String()},
		},
		GroupByAttributes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
			analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
		},
		Frequency:          analyticspb.Frequency_EVALUATION_PERIOD,
		IncludeExportStats: true,
		Metadata: &analyticspb.Metadata{
			TimeZoneId: "UTC",
		},
		TaskStatus: []int32{
			int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
		},
	})
	s.NoError(err)
	s.NotEmpty(resp.DirectorTaskStats)

	found := false
	for _, stat := range resp.DirectorTaskStats {
		if len(stat.Attribute.TaskNames) == 1 && strings.Contains(stat.Attribute.TaskNames[0], "qm_task_current_period") {
			found = true
			// 1 evaluated out of 100 assigned → CompletionRate = 0.01
			s.Equal(int32(1), stat.QmTaskStats.EvalueatedScrecardCount)
			s.Equal(int32(100), stat.QmTaskStats.AssignedScorecardCount)
			s.InDelta(0.01, float64(stat.QmTaskStats.CompletionRate), 0.001,
				"CompletionRate should be 1/100 = 0.01")

			// Period started 15 days ago (well into a 1-month period),
			// so progressPercentage is well above CompletionRate (1%) → BEHIND_SCHEDULE.
			s.Equal(
				analyticspb.EvaluationPeriodProgressStatus_EVALUATION_PERIOD_PROGRESS_STATUS_BEHIND_SCHEDULE,
				stat.QmTaskStats.EvaluationPeriodProgressStatus,
				"With 1%% completion halfway through the period, status must be BEHIND_SCHEDULE",
			)
		}
	}
	s.True(found, "Expected to find stats for qm_task_current_period")
}

func (s *RetrieveQMTaskStatsTestSuite) TestPublishedScorecardCount() {
	// When
	resp, err := s.analyticsService.RetrieveQMTaskStats(context.Background(), &analyticspb.RetrieveDirectorTaskStatsRequest{
		Parent: "customers/" + s.customerID + "/profiles/" + s.profileID,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
		},
		FilterByAttribute: &analyticspb.Attribute{
			UsecaseNames: []string{customerpb.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  "usecase_id",
			}.String()},
			ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
				CustomerID:                  s.customerID,
				ProfileID:                   s.profileID,
				ScorecardTemplateID:         "scorecard_template_id",
				ScorecardTemplateRevisionID: "*",
			}.String()},
			TaskNames: []string{coachingpb.DirectorTaskName{
				CustomerID:     s.customerID,
				ProfileID:      s.profileID,
				DirectorTaskID: "qm_task_1",
			}.String()},
		},
		GroupByAttributes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
			analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
		},
		TaskStatus: []int32{
			int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE),
		},
	})

	// Then
	s.Require().NoError(err)
	s.Require().NotEmpty(resp.DirectorTaskStats)

	found := false
	for _, stat := range resp.DirectorTaskStats {
		if len(stat.Attribute.TaskNames) == 1 && strings.Contains(stat.Attribute.TaskNames[0], "qm_task_1") {
			startTime := stat.Attribute.TimeRanges[0].StartTimestamp.AsTime()
			if startTime.Equal(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)) {
				found = true
				s.Equal(int32(1), stat.QmTaskStats.PublishedScorecardCount,
					"Only scorecard_1 has published_at set; scorecard_2 does not")
			}
		}
	}
	s.True(found, "Expected to find stats for qm_task_1 in August 2023")
}

func TestRetrieveQMTaskStatsConversationCoverage(t *testing.T) {
	r := RetrieveQMTaskStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	t.Cleanup(func() { r.TearDownSuite() })

	audienceJSON, err := protojson.Marshal(&coachingpb.Audience{
		Users: []string{"customers/customer_id/users/agent_id_1", "customers/customer_id/users/agent_id_2"},
	})
	r.Require().NoError(err)
	r.Require().NoError(r.DB().Create(&model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "coverage_template_id",
		Revision:   "1",
		Title:      "All Eligible Template",
		Template:   `{"criteria":[{"type":"numeric-radios","weight":1,"settings":{"range":{"max":5,"min":1}},"identifier":"c1","displayName":"Criterion"}]}`,
		UsecaseIds: []string{"usecase_id"},
		Audience:   sql.NullString{String: string(audienceJSON), Valid: true},
	}).Error)
	for _, task := range []struct {
		taskID         string
		completionType qadbpb.TaskCompleteConfig_CompletionType
		analystQuota   int32
	}{
		{"qm_task_coverage", qadbpb.TaskCompleteConfig_COMPLETION_TYPE_CONVERSATION_COVERAGE_PERCENTAGE, 10},
		{"qm_task_fixed_count", qadbpb.TaskCompleteConfig_COMPLETION_TYPE_UNSPECIFIED, 4},
	} {
		r.Require().NoError(r.DB().Create(&model.Tasks{
			CustomerID: r.customerID,
			ProfileID:  r.profileID,
			UsecaseID:  sql.NullString{String: "usecase_id", Valid: true},
			TaskID:     task.taskID,
			TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
			TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
			TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
				QmTaskContentConfig: &qadbpb.QMTaskContentConfig{ScorecardTemplateId: "coverage_template_id"},
			}},
			TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
				QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
					StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
				},
			}},
			TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
				QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
					EvaluatedConversationCountPerAgent: 2,
					CompletionType:                     task.completionType,
					ConversationCoveragePercentage:     100,
				},
			}},
			TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{Message: &qadbpb.DirectorTaskAudienceConfig{
				QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
					QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
						{QaAnalystUserId: "analyst_id_1", AssignedConversationCount: task.analystQuota},
					},
				},
			}},
		}).Error)
	}
	for _, scorecard := range []struct {
		resourceID  string
		taskID      string
		agentUserID string
		submittedAt time.Time
	}{
		{"coverage_scorecard_1", "qm_task_coverage", "agent_id_1", time.Date(2023, 8, 5, 12, 0, 0, 0, time.UTC)},
		{"coverage_scorecard_2", "qm_task_coverage", "agent_id_2", time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)},
		{"fixed_count_scorecard_1", "qm_task_fixed_count", "agent_id_1", time.Date(2023, 8, 12, 12, 0, 0, 0, time.UTC)},
	} {
		r.Require().NoError(r.DB().Create(&model.Scorecards{
			Customer:         r.customerID,
			Profile:          r.profileID,
			UsecaseID:        sql.NullString{String: "usecase_id", Valid: true},
			ResourceID:       scorecard.resourceID,
			TemplateID:       "coverage_template_id",
			TemplateRevision: "1",
			TaskIds:          []string{scorecard.taskID},
			SubmitterUserID:  sql.NullString{String: "analyst_id_1", Valid: true},
			AgentUserID:      scorecard.agentUserID,
			SubmittedAt:      sql.NullTime{Time: scorecard.submittedAt, Valid: true},
			Score:            sql.NullFloat64{Float64: 1, Valid: true},
			ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
			SubmissionSource: sql.NullInt32{Int32: int32(qadbpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS), Valid: true},
		}).Error)
	}

	r.mockConfigClient.EXPECT().GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().Return(&config.ProfileConfig{}, nil)
	r.mockConfigClient.EXPECT().Get(gomock.Any()).AnyTimes().Return(&config.CustomerConfig{}, nil)

	taskAgentsReq := &userpb.ListUsersRequest{
		Parent:    "customers/customer_id",
		ProfileId: "profile_id",
		Filter: &userpb.ListUsersRequest_ListUsersFilter{
			State:   userpb.User_ACTIVE,
			Roles:   []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
			UserIds: []string{"agent_id_1", "agent_id_2"},
			UserTypes: []enums.UserType{
				enums.UserType_PROD_USER,
				enums.UserType_GUEST_USER,
				enums.UserType_AI_AGENT_USER,
			},
			CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
		},
		IncludeUserMemberships: true,
		PageSize:               shared.DefaultListUsersPageSize,
		OrderField:             "user_id",
	}
	taskAgentsResp := &userpb.ListUsersResponse{
		Users: []*userpb.User{
			{Name: "customers/customer_id/users/agent_id_1", DisplayName: "agent name 1"},
			{Name: "customers/customer_id/users/agent_id_2", DisplayName: "agent name 2"},
		},
	}
	retrieveTaskStats := func(start, end time.Time) map[string]*analyticspb.QMTaskStats {
		resp, err := r.Client.RetrieveDirectorTaskStats(r.ctx, &analyticspb.RetrieveDirectorTaskStatsRequest{
			Parent:           "customers/" + r.customerID + "/profiles/" + r.profileID,
			DirectorTaskType: int32(coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM),
			FilterByTimeRange: &analyticspb.TimeRange{
				StartTimestamp: timestamppb.New(start),
				EndTimestamp:   timestamppb.New(end),
			},
			FilterByAttribute: &analyticspb.Attribute{
				UsecaseNames: []string{customerpb.UsecaseName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					UsecaseID:  "usecase_id",
				}.String()},
				ScorecardTemplateNames: []string{coachingpb.ScorecardTemplateName{
					CustomerID:                  r.customerID,
					ProfileID:                   r.profileID,
					ScorecardTemplateID:         "coverage_template_id",
					ScorecardTemplateRevisionID: "*",
				}.String()},
				TaskNames: []string{
					coachingpb.DirectorTaskName{CustomerID: r.customerID, ProfileID: r.profileID, DirectorTaskID: "qm_task_coverage"}.String(),
					coachingpb.DirectorTaskName{CustomerID: r.customerID, ProfileID: r.profileID, DirectorTaskID: "qm_task_fixed_count"}.String(),
				},
			},
			GroupByAttributes: []analyticspb.AttributeType{
				analyticspb.AttributeType_ATTRIBUTE_TYPE_DIRECTOR_TASK,
				analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
			},
			TaskStatus: []int32{int32(coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE)},
		})
		r.Require().NoError(err)
		statsByTask := make(map[string]*analyticspb.QMTaskStats)
		for _, stat := range resp.DirectorTaskStats {
			r.Require().Len(stat.Attribute.TaskNames, 1)
			taskName, err := coachingpb.ParseDirectorTaskName(stat.Attribute.TaskNames[0])
			r.Require().NoError(err)
			statsByTask[taskName.DirectorTaskID] = stat.QmTaskStats
		}
		r.Require().Len(statsByTask, 2)
		return statsByTask
	}

	r.Run("period with eligible conversations uses their count as the quota", func() {
		// Given
		r.mockUserClient.EXPECT().ListUsers(gomock.Any(), taskAgentsReq).Times(1).Return(taskAgentsResp, nil)
		r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile("ConversationCoverage_es_response.json"))
		r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("ConversationCoverage_Aug2023_es_request.json"), cmputil.IgnoreJSONFields("fields"))

		// When
		statsByTask := retrieveTaskStats(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC))

		// Then
		coverage := statsByTask["qm_task_coverage"]
		r.Require().Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, coverage.StatsPeriodType)
		r.Require().Equal(int32(8), coverage.AssignedScorecardCount)
		r.Require().Equal(int32(2), coverage.EvalueatedScrecardCount)
		r.Require().InDelta(0.25, coverage.CompletionRate, 1e-6)
		fixedCount := statsByTask["qm_task_fixed_count"]
		r.Require().Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, fixedCount.StatsPeriodType)
		r.Require().Equal(int32(4), fixedCount.AssignedScorecardCount)
		r.Require().Equal(int32(1), fixedCount.EvalueatedScrecardCount)
		r.Require().InDelta(0.25, fixedCount.CompletionRate, 1e-6)
	})

	r.Run("period without eligible conversations has no completion rate", func() {
		// Given
		r.mockUserClient.EXPECT().ListUsers(gomock.Any(), taskAgentsReq).Times(1).Return(taskAgentsResp, nil)
		r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile("ConversationCoverage_empty_es_response.json"))
		r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("ConversationCoverage_Sep2023_es_request.json"), cmputil.IgnoreJSONFields("fields"))

		// When
		statsByTask := retrieveTaskStats(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 9, 30, 23, 59, 59, 999999999, time.UTC))

		// Then
		coverage := statsByTask["qm_task_coverage"]
		r.Require().Equal(analyticspb.QMTaskStats_REGULAR_PERIOD, coverage.StatsPeriodType)
		r.Require().Equal(int32(0), coverage.AssignedScorecardCount)
		r.Require().Equal(int32(0), coverage.EvalueatedScrecardCount)
		r.Require().InDelta(-1, coverage.CompletionRate, 1e-6)
		fixedCount := statsByTask["qm_task_fixed_count"]
		r.Require().Equal(int32(4), fixedCount.AssignedScorecardCount)
		r.Require().Equal(int32(0), fixedCount.EvalueatedScrecardCount)
		r.Require().InDelta(0, fixedCount.CompletionRate, 1e-6)
	})
}
