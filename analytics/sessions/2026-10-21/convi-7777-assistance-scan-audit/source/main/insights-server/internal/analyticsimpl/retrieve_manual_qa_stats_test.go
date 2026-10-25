package analyticsimpl

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"

	"github.com/cresta/go-servers/shared/db/converter"
	"github.com/cresta/go-servers/shared/testing/cmputil"
)

type RetrieveManualQAStatsTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveManualQAStatsTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
	// Creates the agent users.
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("agent_id_1"),
		Username:    "agent_user_name_1",
		DisplayName: converter.ConvertToNullString("agent name 1"),
		FullName:    converter.ConvertToNullString("agent name 1"),
		UserRoles:   []int32{int32(authpb.AuthProto_AGENT)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("agent_id_2"),
		Username:    "agent_user_name_2",
		DisplayName: converter.ConvertToNullString("agent name 2"),
		FullName:    converter.ConvertToNullString("agent name 2"),
		UserRoles:   []int32{int32(authpb.AuthProto_AGENT)},
	}).Error)

	// Creates the QA analyst users.
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_1"),
		Username:    "analyst_user_name_1",
		DisplayName: converter.ConvertToNullString("analyst name 1"),
		FullName:    converter.ConvertToNullString("analyst name 1"),
		UserRoles:   []int32{int32(authpb.AuthProto_QA_SPECIALIST)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_2"),
		Username:    "analyst_user_name_2",
		DisplayName: converter.ConvertToNullString("analyst name 2"),
		FullName:    converter.ConvertToNullString("analyst name 2"),
		UserRoles:   []int32{int32(authpb.AuthProto_QA_SPECIALIST)},
	}).Error)

	// Create the calibrator user.
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("calibrator_id"),
		Username:    "calibrator_user_name",
		DisplayName: converter.ConvertToNullString("calibrator name"),
		FullName:    converter.ConvertToNullString("calibrator name"),
		UserRoles:   []int32{int32(authpb.AuthProto_ADMIN)},
	}).Error)

	// Creates the conversations.
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_1"),
		PlatformID:        "testconversationid1a-platformid",
		PlatformAccountID: "testconversationid1a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_1"),
		CreatedAt:         time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_2"),
		PlatformID:        "testconversationid2a-platformid",
		PlatformAccountID: "testconversationid2a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_1"),
		CreatedAt:         time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_3"),
		PlatformID:        "testconversationid3a-platformid",
		PlatformAccountID: "testconversationid3a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_2"),
		CreatedAt:         time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_4"),
		PlatformID:        "testconversationid4a-platformid",
		PlatformAccountID: "testconversationid4a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_2"),
		CreatedAt:         time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_5"),
		PlatformID:        "testconversationid5a-platformid",
		PlatformAccountID: "testconversationid5a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_2"),
		CreatedAt:         time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_6"),
		PlatformID:        "testconversationid6a-platformid",
		PlatformAccountID: "testconversationid6a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_1"),
		CreatedAt:         time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC),
	}).Error)

	// Creates the scorecard templates
	qaTaskConfig := &qadbpb.QATaskConfig{
		TaskCompleteConfig: &qadbpb.TaskCompleteConfig{
			EvaluatedConversationCountPerAgent: 5,
		},
		EvaluationPeriodConfig: &qadbpb.EvaluationPeriodConfig{
			StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
		},
		QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
			QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
				{
					QaAnalystUserId:           "analyst_id_1",
					AssignedConversationCount: 10,
				},
				{
					QaAnalystUserId:           "analyst_id_2",
					AssignedConversationCount: 5,
				},
			},
		},
	}
	templateAudience := &coachingpb.Audience{
		Users: []string{"customers/customer_id/users/agent_id_1", "customers/customer_id/users/agent_id_2"},
	}
	qaTaskConfigJSON, err := protojson.Marshal(qaTaskConfig)
	s.Require().NoError(err)
	qaTaskConfigOverride := &qadbpb.QATaskConfig{
		TaskCompleteConfig: &qadbpb.TaskCompleteConfig{
			EvaluatedConversationCountPerAgent: 3,
			OverriddenAgents: []*qadbpb.TaskCompleteConfig_OverriddenAgentsConfig{
				{
					User:                               userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
					EvaluatedConversationCountPerAgent: 2,
				},
				{
					Group:                              userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_1"}.String(),
					EvaluatedConversationCountPerAgent: 3,
				},
			},
		},
		EvaluationPeriodConfig: &qadbpb.EvaluationPeriodConfig{
			StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
			OverrideTimeRange: &qadbpb.EvaluationPeriodConfig_TimeRange{
				StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2023, 8, 14, 23, 59, 59, 0, time.UTC)),
			},
		},
		QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
			QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
				{
					QaAnalystUserId:           "analyst_id_1",
					AssignedConversationCount: 10,
				},
				{
					QaAnalystUserId:           "analyst_id_2",
					AssignedConversationCount: 5,
				},
			},
		},
		ConversationSuggestionConfig: &qadbpb.ConversationSuggestionConfig{},
	}
	qaTaskConfigOverrideJSON, err := protojson.Marshal(qaTaskConfigOverride)
	s.Require().NoError(err)
	audienceJSON, err := protojson.Marshal(templateAudience)
	s.Require().NoError(err)
	// Create a simple template with two numeric criteria for consistency score calculation
	simpleTemplate := `{
		"version": 2,
		"items": [
			{
				"type": "numeric-radios",
				"weight": 1,
				"identifier": "criterion_id_1",
				"displayName": "Criterion 1",
				"settings": {
					"range": {
						"max": 5,
						"min": 0
					}
				}
			},
			{
				"type": "numeric-radios",
				"weight": 1,
				"identifier": "criterion_id_2",
				"displayName": "Criterion 2",
				"settings": {
					"range": {
						"max": 5,
						"min": 0
					}
				}
			}
		],
		"displayCommentField": true
	}`
	simpleTemplateOverride := `{
		"version": 2,
		"items": [
			{
				"type": "numeric-radios",
				"weight": 1,
				"identifier": "criterion_id_1_override",
				"displayName": "Criterion 1 Override",
				"settings": {
					"range": {
						"max": 5,
						"min": 0
					}
				}
			},
			{
				"type": "numeric-radios",
				"weight": 1,
				"identifier": "criterion_id_2_override",
				"displayName": "Criterion 2 Override",
				"settings": {
					"range": {
						"max": 5,
						"min": 0
					}
				}
			}
		],
		"displayCommentField": true
	}`
	s.Require().NoError(s.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id",
		Revision:     "r1",
		UsecaseIds:   []string{"usecase_id"},
		Title:        "title1",
		Template:     simpleTemplate,
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(qaTaskConfigJSON)),
		Audience:     sql.NullString{String: string(audienceJSON), Valid: true},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id_override",
		Revision:     "r1",
		UsecaseIds:   []string{"usecase_id"},
		Title:        "title1",
		Template:     simpleTemplateOverride,
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(qaTaskConfigOverrideJSON)),
		Audience:     sql.NullString{String: string(audienceJSON), Valid: true},
	}).Error)

	// Creates the tasks.
	s.Require().NoError(s.DB().Create(&dbmodel.Tasks{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		TaskID:            "task_id_1",
		TaskDisplayName:   sql.NullString{String: "Task 1", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CreatorUserID:     "creator_id",
		LastUpdaterUserID: "creator_id",
		TaskType:          coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus:        coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		UsecaseID:         sql.NullString{String: "usecase_id", Valid: true},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{Message: &qadbpb.DirectorTaskAudienceConfig{
			UserIds: []string{"agent_id_1", "agent_id_2"},
			QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
				QaAnalystQuotas: qaTaskConfig.QaAnalystQuotaConfig.QaAnalystQuotas,
			},
		}},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
			QmTaskCompleteConfig: qaTaskConfig.TaskCompleteConfig,
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: qaTaskConfig.EvaluationPeriodConfig,
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          "scorecard_template_id",
				ConversationSuggestionConfig: qaTaskConfig.ConversationSuggestionConfig,
			},
		}},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Tasks{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		TaskID:            "task_id_2",
		TaskDisplayName:   sql.NullString{String: "Task 2", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CreatorUserID:     "creator_id",
		LastUpdaterUserID: "creator_id",
		TaskType:          coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus:        coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		UsecaseID:         sql.NullString{String: "usecase_id", Valid: true},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{Message: &qadbpb.DirectorTaskAudienceConfig{
			UserIds: []string{"agent_id_1", "agent_id_2"},
			QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
				QaAnalystQuotas: qaTaskConfigOverride.QaAnalystQuotaConfig.QaAnalystQuotas,
			},
		}},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
			QmTaskCompleteConfig: qaTaskConfigOverride.TaskCompleteConfig,
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: qaTaskConfigOverride.EvaluationPeriodConfig,
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          "scorecard_template_id_override",
				ConversationSuggestionConfig: qaTaskConfigOverride.ConversationSuggestionConfig,
			},
		}},
	}).Error)

	// Creates the scorecards.
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_1",
		ConversationID:   "convo_id_1",
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(100.0),
		Comment:          converter.ConvertToNullString("good job"),
		AutoFailed:       converter.ConvertToNullBool(false),
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS)),
		ScorecardType:    converter.ConvertToNullInt32(int32(coachingpb.ScorecardType_SCORECARD_TYPE_UNSPECIFIED)),
		TaskIds:          []string{"task_id_1"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_2",
		ConversationID:   "convo_id_2",
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(50.0),
		AutoFailed:       converter.ConvertToNullBool(false),
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS)),
		TaskIds:          []string{"task_id_1"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_3",
		ConversationID:   "convo_id_3",
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(0.0),
		Comment:          converter.ConvertToNullString("bad job"),
		AutoFailed:       converter.ConvertToNullBool(true),
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS)),
		TaskIds:          []string{"task_id_1"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_4",
		ConversationID:   "convo_id_4",
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(60.0),
		Comment:          converter.ConvertToNullString("ok job"),
		AutoFailed:       converter.ConvertToNullBool(false),
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_QM_TASK_HOME)),
		TaskIds:          []string{"task_id_1"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_5",
		ConversationID:   "convo_id_5",
		MessageIds:       []string{"message_id_1", "message_id_2"},
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(70.0),
		AutoFailed:       converter.ConvertToNullBool(false),
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_QM_TASK_HOME)),
		TaskIds:          []string{"task_id_1"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_1_override",
		ConversationID:   "convo_id_1",
		TemplateID:       "scorecard_template_id_override",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(100.0),
		Comment:          converter.ConvertToNullString("good job"),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"task_id_2"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_2_override",
		ConversationID:   "convo_id_2",
		TemplateID:       "scorecard_template_id_override",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(50.0),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"task_id_2"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_3_override",
		ConversationID:   "convo_id_3",
		TemplateID:       "scorecard_template_id_override",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(0.0),
		Comment:          converter.ConvertToNullString("bad job"),
		AutoFailed:       converter.ConvertToNullBool(true),
		TaskIds:          []string{"task_id_2"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_4_override",
		ConversationID:   "convo_id_4",
		TemplateID:       "scorecard_template_id_override",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(60.0),
		Comment:          converter.ConvertToNullString("ok job"),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"task_id_2"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_5_override",
		ConversationID:   "convo_id_5",
		TemplateID:       "scorecard_template_id_override",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(70.0),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"task_id_2"},
	}).Error)

	// Creates the calibration scorecards.
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		ResourceID:            "calibrated_scorecard_id_1",
		ConversationID:        "convo_id_1",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "r1",
		AgentUserID:           "agent_id_1",
		SubmitterUserID:       converter.ConvertToNullString("calibrator_id"),
		SubmittedAt:           converter.ConvertToNullTime(time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:        converter.ConvertToNullBool(true),
		Score:                 converter.ConvertToNullFloat64(50.0),
		AutoFailed:            converter.ConvertToNullBool(false),
		CalibratedScorecardID: converter.ConvertToNullString("scorecard_id_1"),
		// CalibrationConsistencyScore is now calculated dynamically, not read from DB
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_QM_TASK_HOME)),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:              s.customerID,
		Profile:               s.profileID,
		ResourceID:            "calibrated_scorecard_id_4",
		ConversationID:        "convo_id_4",
		TemplateID:            "scorecard_template_id",
		TemplateRevision:      "r1",
		AgentUserID:           "agent_id_2",
		SubmitterUserID:       converter.ConvertToNullString("calibrator_id"),
		SubmittedAt:           converter.ConvertToNullTime(time.Date(2023, 10, 4, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:        converter.ConvertToNullBool(true),
		Score:                 converter.ConvertToNullFloat64(80.0),
		AutoFailed:            converter.ConvertToNullBool(false),
		CalibratedScorecardID: converter.ConvertToNullString("scorecard_id_4"),
		// CalibrationConsistencyScore is now calculated dynamically, not read from DB
		SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_QM_TASK_HOME)),
	}).Error)

	// Creates the scorecard scores.
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_1",
		ScorecardID:         "scorecard_id_1",
		CriterionIdentifier: "criterion_id_1",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_2",
		ScorecardID:         "scorecard_id_1",
		CriterionIdentifier: "criterion_id_2",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_3",
		ScorecardID:         "scorecard_id_2",
		CriterionIdentifier: "criterion_id_1",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)
	// Creates scores for scorecard_id_4 (reference for calibrated_scorecard_id_4)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_4_1",
		ScorecardID:         "scorecard_id_4",
		CriterionIdentifier: "criterion_id_1",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(2.0),
		NumericValue:        converter.ConvertToNullFloat64(2.0),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_4_2",
		ScorecardID:         "scorecard_id_4",
		CriterionIdentifier: "criterion_id_2",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(3.0),
		NumericValue:        converter.ConvertToNullFloat64(3.0),
	}).Error)

	// Creates scores for calibration scorecards (matching reference scorecards for consistency calculation)
	// Scores for calibrated_scorecard_id_1 - matching scorecard_id_1 (100% consistency)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "calibrated_score_id_1_1",
		ScorecardID:         "calibrated_scorecard_id_1",
		CriterionIdentifier: "criterion_id_1",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5), // Matches scorecard_id_1
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "calibrated_score_id_1_2",
		ScorecardID:         "calibrated_scorecard_id_1",
		CriterionIdentifier: "criterion_id_2",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5), // Matches scorecard_id_1
	}).Error)
	// Scores for calibrated_scorecard_id_4 - matching scorecard_id_4 (100% consistency)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "calibrated_score_id_4_1",
		ScorecardID:         "calibrated_scorecard_id_4",
		CriterionIdentifier: "criterion_id_1",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(2.0),
		NumericValue:        converter.ConvertToNullFloat64(2.0), // Matches scorecard_id_4
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "calibrated_score_id_4_2",
		ScorecardID:         "calibrated_scorecard_id_4",
		CriterionIdentifier: "criterion_id_2",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(3.0),
		NumericValue:        converter.ConvertToNullFloat64(3.0), // Matches scorecard_id_4
	}).Error)

	// Creates scores for override template scorecards
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_1_override",
		ScorecardID:         "scorecard_id_1_override",
		CriterionIdentifier: "criterion_id_1_override",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_2_override",
		ScorecardID:         "scorecard_id_1_override",
		CriterionIdentifier: "criterion_id_2_override",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_3_override",
		ScorecardID:         "scorecard_id_2_override",
		CriterionIdentifier: "criterion_id_1_override",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)

	// Template with both regular and outcome (metadata trigger) criteria for testing overwritten count exclusion.
	outcomeTemplateID := "scorecard_template_id_outcome"
	simpleTemplateWithOutcome := `{
		"version": 2,
		"items": [
			{
				"type": "numeric-radios",
				"weight": 1,
				"identifier": "criterion_regular",
				"displayName": "Regular Criterion",
				"settings": {
					"range": {"max": 5, "min": 0}
				}
			},
			{
				"type": "numeric-radios",
				"weight": 1,
				"identifier": "criterion_outcome",
				"displayName": "Outcome Criterion",
				"auto_qa": {
					"triggers": [{"type": "metadata", "resource_name": "customers/customer_id/profiles/profile_id/moments/moment_outcome"}]
				},
				"settings": {
					"range": {"max": 5, "min": 0}
				}
			}
		],
		"displayCommentField": true
	}`
	s.Require().NoError(s.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   outcomeTemplateID,
		Revision:     "r1",
		UsecaseIds:   []string{"usecase_id"},
		Title:        "Outcome Template",
		Template:     simpleTemplateWithOutcome,
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(qaTaskConfigJSON)),
		Audience:     sql.NullString{String: string(audienceJSON), Valid: true},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Tasks{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		TaskID:            "task_id_outcome",
		TaskDisplayName:   sql.NullString{String: "Outcome Task", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CreatorUserID:     "creator_id",
		LastUpdaterUserID: "creator_id",
		TaskType:          coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus:        coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		UsecaseID:         sql.NullString{String: "usecase_id", Valid: true},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{Message: &qadbpb.DirectorTaskAudienceConfig{
			UserIds: []string{"agent_id_1", "agent_id_2"},
			QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
				QaAnalystQuotas: qaTaskConfig.QaAnalystQuotaConfig.QaAnalystQuotas,
			},
		}},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
			QmTaskCompleteConfig: qaTaskConfig.TaskCompleteConfig,
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: qaTaskConfig.EvaluationPeriodConfig,
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          outcomeTemplateID,
				ConversationSuggestionConfig: qaTaskConfig.ConversationSuggestionConfig,
			},
		}},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_outcome",
		ConversationID:   "convo_id_1",
		TemplateID:       outcomeTemplateID,
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 5, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(80.0),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"task_id_outcome"},
	}).Error)
	// Overwritten on regular criterion (ai=1, num=0.5) and outcome criterion (ai=2, num=1).
	// Only the regular criterion overwrite should be counted.
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_outcome_regular",
		ScorecardID:         "scorecard_id_outcome",
		CriterionIdentifier: "criterion_regular",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(1.0),
		NumericValue:        converter.ConvertToNullFloat64(0.5),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_id_outcome_metadata",
		ScorecardID:         "scorecard_id_outcome",
		CriterionIdentifier: "criterion_outcome",
		AiScored:            converter.ConvertToNullBool(true),
		AiValue:             converter.ConvertToNullFloat64(2.0),
		NumericValue:        converter.ConvertToNullFloat64(1.0),
	}).Error)

	// Create a no-task template
	noTaskTemplateID := "scorecard_template_id_no_task"
	noTaskTemplateRevision := "r1"
	noTaskConfig := &qadbpb.QATaskConfig{
		TaskCompleteConfig: &qadbpb.TaskCompleteConfig{
			EvaluatedConversationCountPerAgent: 2,
		},
		EvaluationPeriodConfig: &qadbpb.EvaluationPeriodConfig{
			StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
		},
	}
	noTaskConfigJSON, err := protojson.Marshal(noTaskConfig)
	s.Require().NoError(err)
	s.Require().NoError(s.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   noTaskTemplateID,
		Revision:     noTaskTemplateRevision,
		UsecaseIds:   []string{"usecase_id"},
		Title:        "No Task Template",
		Template:     "{}",
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(noTaskConfigJSON)),
		Audience:     sql.NullString{String: string(audienceJSON), Valid: true},
	}).Error)

	// Add scorecards for the no-task template
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_no_task_1",
		ConversationID:   "convo_id_1",
		TemplateID:       noTaskTemplateID,
		TemplateRevision: noTaskTemplateRevision,
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(90.0),
		Comment:          converter.ConvertToNullString("no task comment 1"),
		AutoFailed:       converter.ConvertToNullBool(false),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_no_task_2",
		ConversationID:   "convo_id_2",
		TemplateID:       noTaskTemplateID,
		TemplateRevision: noTaskTemplateRevision,
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(80.0),
		Comment:          converter.ConvertToNullString("no task comment 2"),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_no_task_3",
		ConversationID:   "convo_id_3",
		TemplateID:       noTaskTemplateID,
		TemplateRevision: noTaskTemplateRevision,
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(80.0),
		Comment:          converter.ConvertToNullString("no task comment 3"),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{shared.NoTask},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_no_task_4",
		ConversationID:   "convo_id_4",
		TemplateID:       noTaskTemplateID,
		TemplateRevision: noTaskTemplateRevision,
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(80.0),
		Comment:          converter.ConvertToNullString("no task comment 4"),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"scorecard_template_id_no_task"},
	}).Error)

	s.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func TestRetrieveManualQAStatsPublishedConversations(t *testing.T) {
	r := RetrieveManualQAStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	t.Cleanup(func() { r.TearDownSuite() })

	// Mark scorecard_id_1 as published
	r.Require().NoError(r.DB().Model(&dbmodel.Scorecards{}).
		Where("resource_id = ?", "scorecard_id_1").
		Update("published_at", time.Date(2023, 8, 2, 10, 0, 0, 0, time.UTC)).Error)

	r.mockUserClient.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		Return(&userpb.ListUsersResponse{}, nil).AnyTimes()

	// When
	resp, err := r.Client.RetrieveManualQAStats(r.ctx, &analyticspb.RetrieveManualQAStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
		},
		FilterByAttribute: &analyticspb.Attribute{
			ScorecardTemplateNames: []string{
				coachingpb.ScorecardTemplateName{
					CustomerID:                  "customer_id",
					ProfileID:                   "profile_id",
					ScorecardTemplateID:         "scorecard_template_id",
					ScorecardTemplateRevisionID: "r1",
				}.String(),
			},
			TaskNames: []string{
				coachingpb.DirectorTaskName{
					CustomerID:     "customer_id",
					ProfileID:      "profile_id",
					DirectorTaskID: "task_id_1",
				}.String(),
			},
		},
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
				},
			},
		},
	})

	// Then
	if err != nil {
		t.Fatalf("RetrieveManualQAStats failed: %v", err)
	}
	if len(resp.ManualQaStats) == 0 {
		t.Fatal("Expected non-empty ManualQaStats")
	}

	expectedScorecardName := "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1"
	totalPublished := 0
	found := false
	for _, stat := range resp.ManualQaStats {
		for _, conv := range stat.PublishedConversations {
			totalPublished++
			if conv.ScorecardName == expectedScorecardName {
				found = true
			}
		}
	}
	if totalPublished != 1 {
		t.Fatalf("Expected 1 published conversation, got %d", totalPublished)
	}
	if !found {
		t.Fatalf("Expected published conversation with ScorecardName %s, not found", expectedScorecardName)
	}
}

func TestRetrieveManualQAStats(t *testing.T) {
	r := RetrieveManualQAStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc string
		// esReqPath              string
		// esRespPath             string
		req                     *analyticspb.RetrieveManualQAStatsRequest
		want                    *analyticspb.RetrieveManualQAStatsResponse
		listUsersReq            *userpb.ListUsersRequest
		listUsersResp           *userpb.ListUsersResponse
		listUsersFromGroupsReq  *userpb.ListUsersRequest
		listUsersFromGroupsResp *userpb.ListUsersResponse
		wantErr                 error
	}{
		{
			desc: "TestTimeRangeStats",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 4,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 1,
					},
					{
						EvaluatedConversationCount: 5,
					},
				},
			},
		},
		{
			desc: "TestSinglePeriodCompletionRateStats",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 4,
						CompletionRate:             0.4,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 1,
						CompletionRate:             0.5,
					},
					{
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 5,
						CompletionRate:             0.5,
					},
				},
			},
		},
		{
			desc: "TestMultiplePeriodsCompletionRateStats",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
					},
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
									EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 5,
						CompletionRate:             0.5,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
									EndTimestamp:   timestamppb.New(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 0,
						CompletionRate:             0,
					},
					{
						Attribute:                  &analyticspb.Attribute{},
						AssignedConversationCount:  20,
						EvaluatedConversationCount: 5,
						CompletionRate:             0.25,
					},
				},
			},
		},
		{
			desc: "TestFilterByTaskNames",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 4,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 1,
					},
					{
						EvaluatedConversationCount: 5,
					},
				},
			},
		},
		{
			desc: "TestQAAnalystStats",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						CompletionRate:             0.2,
						EvaluatedConversationCount: 2,
						AssignedConversationCount:  10,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
						},
						AvgPerfScore:               60.,
						CompletionRate:             0.6,
						EvaluatedConversationCount: 3,
						AssignedConversationCount:  5,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_5",
									CreateTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_5",
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_2",
								},
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 70.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "TestOutcomeCriteriaExcludedFromOverwrittenCount",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_EVALUATION_PERIOD,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_outcome",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_outcome",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AssignedConversationCount:  10,
						AvgPerfScore:               80.,
						AvgConsistencyScore:        -1,
						CompletionRate:             0.1,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_outcome",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 80.0,
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_outcome",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 80.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
						},
						AssignedConversationCount: 5,
						AvgPerfScore:              -1,
						AvgConsistencyScore:       -1,
					},
				},
			},
		},
		{
			desc: "TestQAAnalystStats_AgentFilter",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
					Users: []*userpb.User{
						{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
							DisplayName: "agent name 1",
						},
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AvgPerfScore:               100.,
						CompletionRate:             -1,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
						},
						AvgPerfScore:               50.,
						CompletionRate:             -1,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						AvgConsistencyScore: -1.,
					},
				},
			},
		},
		{
			desc: "TestQAAnalystStatsGroupByTime",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
						},
						AvgPerfScore:               60.,
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_5",
									CreateTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_2",
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_5",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 70.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               50.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               55.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore:        60.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               70.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_5",
									CreateTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_2",
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_5",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 70.0,
							},
						},
					},
				},
			},
		},
		{
			desc: "TestAgentStatsGroupByTime",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AvgPerfScore:               75.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
						},
						AvgPerfScore:               (60. + 70.) / 3.0,
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 60.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_5",
									CreateTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_2",
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_5",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 70.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               75.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               30.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 60.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               70.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_5",
									CreateTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_5/messages/message_id_2",
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_5",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 70.0,
							},
						},
					},
				},
			},
		},
		{
			desc: "TestQAAnalystStats_QAAnalystFilter",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						CompletionRate:             0.2,
						EvaluatedConversationCount: 2,
						AssignedConversationCount:  10,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "TestTimeRangeStats_SubmissionSourceFilter",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 3,
					},
					{
						EvaluatedConversationCount: 3,
					},
				},
			},
		},
		{
			desc: "TestTimeRangeStats_MultipleSubmissionSourceFilter",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_QM_TASK_HOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 4,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 1,
					},
					{
						EvaluatedConversationCount: 5,
					},
				},
			},
		},
		{
			desc: "TestSinglePeriodCompletionRateStats_SubmissionSourceFilter",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 3,
						CompletionRate:             0.3,
					},
					{
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 3,
						CompletionRate:             0.3,
					},
				},
			},
		},
		{
			desc: "TestQAAnalystStats_SubmissionSourceFilter",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						CompletionRate:             0.2,
						EvaluatedConversationCount: 2,
						AssignedConversationCount:  10,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
						AvgConsistencyScore: 100., // Calculated dynamically; 100% when all scores match
						CalibratedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore:        100.0,
								ConsistencyScore: 100.0, // Calculated dynamically; 100% when all scores match
								Calibrator: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "calibrator_id"}.String(),
									DisplayName: "calibrator name",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
						},
						AvgPerfScore:               50.,
						CompletionRate:             0.2,
						EvaluatedConversationCount: 1,
						AssignedConversationCount:  5,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
						AvgConsistencyScore: -1.,
					},
				},
			},
		},
		{
			desc: "TestAgentStatsGroupByTime_SubmissionSourceFilter",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
					ScorecardSubmissionSources: []analyticspb.Attribute_ScorecardSubmissionSource{
						analyticspb.Attribute_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AvgPerfScore:               75.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
						},
						AvgPerfScore:               0.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               75.,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               0.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
					},
				},
			},
		},
		{
			desc: "TestQAAnalystStats_WithOverrideScorecards",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 29, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_override",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_2",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_QA_ANALYST,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						AvgConsistencyScore:        -1,
						CompletionRate:             0.2,
						EvaluatedConversationCount: 2,
						AssignedConversationCount:  10,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
									CreateTime: timestamppb.New(time.Date(2023, 8, 3, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_3_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 0.0,
							},
						},
						OverwrittenCriteriaCount: 2,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
									CreateTime: timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
								PerfScore: 100.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
							},
						},
						AvgPerfScore:               60.,
						AvgConsistencyScore:        -1,
						CompletionRate:             0.6,
						EvaluatedConversationCount: 3,
						AssignedConversationCount:  5,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 60.0,
							},
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_5",
									CreateTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_5_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 70.0,
							},
						},
						CommentedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_4",
									CreateTime: timestamppb.New(time.Date(2023, 8, 4, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_4_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 60.0,
							},
						},
						OverwrittenCriteriaCount: 1,
						CriteriaOverwrittenConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2_override",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
				},
			},
		},
		{
			desc: "TestMultiplePeriodsCompletionRateStats_WithOverrideScorecards",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			listUsersFromGroupsReq: &userpb.ListUsersRequest{
				Parent:                 "customers/customer_id",
				ProfileId:              "profile_id",
				IncludeUserMemberships: true,
				OrderField:             "user_id",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					IncludeIndirectGroupMemberships: true,
					State:                           userpb.User_ACTIVE,
					Groups:                          []string{userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_1"}.String()},
					Roles:                           []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
				},
			},
			listUsersFromGroupsResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
					},
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_override",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_2",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
									EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 5,
						CompletionRate:             1.0,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
									EndTimestamp:   timestamppb.New(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 0,
						CompletionRate:             0,
					},
					{
						Attribute:                  &analyticspb.Attribute{},
						AssignedConversationCount:  10,
						EvaluatedConversationCount: 5,
						CompletionRate:             0.5,
					},
				},
			},
		},
		{
			desc: "TestSinglePeriodCompletionRateStats_WithOverrideScorecards",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			listUsersFromGroupsReq: &userpb.ListUsersRequest{
				Parent:                 "customers/customer_id",
				ProfileId:              "profile_id",
				IncludeUserMemberships: true,
				OrderField:             "user_id",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					IncludeIndirectGroupMemberships: true,
					Groups:                          []string{userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_1"}.String()},
					State:                           userpb.User_ACTIVE,
					Roles:                           []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
				},
			},
			listUsersFromGroupsResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_override",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_2",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 4,
						CompletionRate:             0.8,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 1,
						CompletionRate:             1,
					},
					{
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 5,
						CompletionRate:             1,
					},
				},
			},
		},
		{
			desc: "TestNoTaskTemplateStats",
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_no_task",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					ScorecardTemplateNamesWithNoTask: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_no_task",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						EvaluatedConversationCount: 3,
					},
					{
						EvaluatedConversationCount: 3,
					},
				},
			},
		},
		{
			desc: "TestAgentStatsGroupByTime_ScorecardReviewersFilter",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 2, 11, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 3, 13, 0, 0, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
					ScorecardReviewerAudience: &analyticspb.UserAudience{
						Users: []string{"customers/customer_id/users/analyst_id_2"},
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               50.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
				},
			},
		},
		{
			desc: "TestAgentStatsGroupByTime_ScorecardReviewerGroupFilterWithoutAgentRole",
			listUsersReq: &userpb.ListUsersRequest{
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
				OrderField:             "user_id",
			},
			listUsersResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			listUsersFromGroupsReq: &userpb.ListUsersRequest{
				Parent:    "customers/customer_id",
				ProfileId: "profile_id",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					Groups:                          []string{"customers/customer_id/groups/reviewer_team"},
					IncludeIndirectGroupMemberships: true,
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					UserTypes: []enums.UserType{
						enums.UserType_PROD_USER,
						enums.UserType_GUEST_USER,
						enums.UserType_AI_AGENT_USER,
					},
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			},
			listUsersFromGroupsResp: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name:        "customers/customer_id/users/analyst_id_2",
						DisplayName: "analyst name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAStatsRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 2, 11, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 3, 13, 0, 0, 0, time.UTC)),
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_1",
						}.String(),
					},
					ScorecardReviewerAudience: &analyticspb.UserAudience{
						Groups: []string{"customers/customer_id/groups/reviewer_team"},
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveManualQAStatsResponse{
				ManualQaStats: []*analyticspb.ManualQAStats{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AvgPerfScore:               50.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_2"}.String(),
									DisplayName: "agent name 2",
								},
							},
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
						AvgPerfScore:               50.,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name:       "customers/customer_id/profiles/profile_id/conversations/convo_id_2",
									CreateTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_2",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
									DisplayName: "analyst name 2",
								},
								PerfScore: 50.0,
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			if tc.listUsersReq != nil && tc.listUsersResp != nil {
				r.mockUserClient.EXPECT().ListUsers(gomock.Any(), tc.listUsersReq).Times(1).Return(tc.listUsersResp, nil)
			}
			if tc.listUsersFromGroupsReq != nil && tc.listUsersFromGroupsResp != nil {
				r.mockUserClient.EXPECT().ListUsers(gomock.Any(), tc.listUsersFromGroupsReq).Times(1).Return(tc.listUsersFromGroupsResp, nil)
			}
			resp, err := r.Client.RetrieveManualQAStats(r.ctx, tc.req)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v but got nil", tc.wantErr)
				}
				if diff := cmp.Diff(tc.wantErr.Error(), err.Error()); diff != "" {
					t.Fatalf("Got error with diff (-want +got):\n %v", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("RetrieveManualQAStats return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}

func TestRetrieveManualQAStatsConversationCoverage(t *testing.T) {
	r := RetrieveManualQAStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	t.Cleanup(func() { r.TearDownSuite() })

	r.Require().NoError(r.DB().Create(&dbmodel.Tasks{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		TaskID:            "task_id_conversation_coverage",
		TaskDisplayName:   sql.NullString{String: "Task conversation coverage", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CreatorUserID:     "creator_id",
		LastUpdaterUserID: "creator_id",
		TaskType:          coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_QM,
		TaskStatus:        coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		UsecaseID:         sql.NullString{String: "usecase_id", Valid: true},
		TaskAudienceConfig: qadbpb.DirectorTaskAudienceConfigSQL{Message: &qadbpb.DirectorTaskAudienceConfig{
			QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
				QaAnalystQuotas: []*qadbpb.QAAnalystQuotaConfig_QAAnalystQuota{
					{QaAnalystUserId: "analyst_id_1", AssignedConversationCount: 10},
				},
			},
		}},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
			QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
				EvaluatedConversationCountPerAgent: 5,
				CompletionType:                     qadbpb.TaskCompleteConfig_COMPLETION_TYPE_CONVERSATION_COVERAGE_PERCENTAGE,
				ConversationCoveragePercentage:     100,
			},
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: &qadbpb.EvaluationPeriodConfig{
				StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
				EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
			},
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          "scorecard_template_id",
				ConversationSuggestionConfig: &qadbpb.ConversationSuggestionConfig{},
			},
		}},
	}).Error)
	for _, scorecard := range []struct {
		resourceID     string
		conversationID string
		agentUserID    string
		submitterID    string
		submittedAt    time.Time
	}{
		{"scorecard_id_coverage_1", "convo_id_1", "agent_id_1", "analyst_id_1", time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)},
		{"scorecard_id_coverage_2", "convo_id_5", "agent_id_2", "analyst_id_2", time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)},
	} {
		r.Require().NoError(r.DB().Create(&dbmodel.Scorecards{
			Customer:         r.customerID,
			Profile:          r.profileID,
			ResourceID:       scorecard.resourceID,
			ConversationID:   scorecard.conversationID,
			TemplateID:       "scorecard_template_id",
			TemplateRevision: "r1",
			AgentUserID:      scorecard.agentUserID,
			SubmitterUserID:  converter.ConvertToNullString(scorecard.submitterID),
			SubmittedAt:      converter.ConvertToNullTime(scorecard.submittedAt),
			ManuallyScored:   converter.ConvertToNullBool(true),
			Score:            converter.ConvertToNullFloat64(100.0),
			AutoFailed:       converter.ConvertToNullBool(false),
			SubmissionSource: converter.ConvertToNullInt32(int32(coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS)),
			TaskIds:          []string{"task_id_conversation_coverage"},
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
		OrderField:             "user_id",
	}
	taskAgentsResp := &userpb.ListUsersResponse{
		Users: []*userpb.User{
			{Name: "customers/customer_id/users/agent_id_1", DisplayName: "agent name 1"},
			{Name: "customers/customer_id/users/agent_id_2", DisplayName: "agent name 2"},
		},
	}
	coverageTaskFilter := &analyticspb.Attribute{
		ScorecardTemplateNames: []string{
			coachingpb.ScorecardTemplateName{
				CustomerID:                  "customer_id",
				ProfileID:                   "profile_id",
				ScorecardTemplateID:         "scorecard_template_id",
				ScorecardTemplateRevisionID: "r1",
			}.String(),
		},
		TaskNames: []string{
			coachingpb.DirectorTaskName{
				CustomerID:     "customer_id",
				ProfileID:      "profile_id",
				DirectorTaskID: "task_id_conversation_coverage",
			}.String(),
		},
	}
	groupByTimeRange := []*analyticspb.AttributeStructure{
		{
			AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
				AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
			},
		},
	}

	r.Run("single period uses the eligible conversation count as the quota", func() {
		// Given
		r.mockUserClient.EXPECT().ListUsers(gomock.Any(), taskAgentsReq).Times(1).Return(taskAgentsResp, nil)
		r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile("ConversationCoverage_es_response.json"))
		r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("ConversationCoverage_Aug2023_es_request.json"), cmputil.IgnoreJSONFields("fields"))

		// When
		resp, err := r.Client.RetrieveManualQAStats(r.ctx, &analyticspb.RetrieveManualQAStatsRequest{
			Parent: r.profileResourceName,
			FilterByEvaluationPeriods: []*analyticspb.TimeRange{
				{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
			},
			Frequency:         analyticspb.Frequency_WEEKLY,
			FilterByAttribute: coverageTaskFilter,
			GroupByAttributes: groupByTimeRange,
		})

		// Then
		r.Require().NoError(err)
		want := &analyticspb.RetrieveManualQAStatsResponse{
			ManualQaStats: []*analyticspb.ManualQAStats{
				{
					Attribute: &analyticspb.Attribute{
						TimeRanges: []*analyticspb.TimeRange{
							{StartTimestamp: timestamppb.New(time.Date(2023, 7, 30, 0, 0, 0, 0, time.UTC))},
						},
					},
					AssignedConversationCount:  8,
					EvaluatedConversationCount: 1,
					CompletionRate:             0.125,
				},
				{
					Attribute: &analyticspb.Attribute{
						TimeRanges: []*analyticspb.TimeRange{
							{StartTimestamp: timestamppb.New(time.Date(2023, 8, 6, 0, 0, 0, 0, time.UTC))},
						},
					},
					AssignedConversationCount:  8,
					EvaluatedConversationCount: 1,
					CompletionRate:             0.25,
				},
				{
					AssignedConversationCount:  8,
					EvaluatedConversationCount: 2,
					CompletionRate:             0.25,
				},
			},
		}
		r.Require().Empty(cmp.Diff(want, resp, protocmp.Transform()))
	})

	r.Run("multiple periods use each period's eligible conversation count as the quota", func() {
		// Given
		r.mockUserClient.EXPECT().ListUsers(gomock.Any(), taskAgentsReq).Times(1).Return(taskAgentsResp, nil)
		// The two periods are counted at the same time, and the fake ES server answers in arrival order. So both
		// periods get the same response. The request files still check the time range of each period.
		r.mockESServer.Resps = append(r.mockESServer.Resps,
			r.getFile("ConversationCoverage_es_response.json"),
			r.getFile("ConversationCoverage_es_response.json"))
		r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("ConversationCoverage_Aug2023_es_request.json"), cmputil.IgnoreJSONFields("fields"))
		r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("ConversationCoverage_Sep2023_es_request.json"), cmputil.IgnoreJSONFields("fields"))

		// When
		resp, err := r.Client.RetrieveManualQAStats(r.ctx, &analyticspb.RetrieveManualQAStatsRequest{
			Parent: r.profileResourceName,
			FilterByEvaluationPeriods: []*analyticspb.TimeRange{
				{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
				},
				{
					StartTimestamp: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC)),
				},
			},
			Frequency:         analyticspb.Frequency_WEEKLY,
			FilterByAttribute: coverageTaskFilter,
			GroupByAttributes: groupByTimeRange,
		})

		// Then
		r.Require().NoError(err)
		want := &analyticspb.RetrieveManualQAStatsResponse{
			ManualQaStats: []*analyticspb.ManualQAStats{
				{
					Attribute: &analyticspb.Attribute{
						TimeRanges: []*analyticspb.TimeRange{
							{
								StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
								EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 0, time.UTC)),
							},
						},
					},
					AssignedConversationCount:  8,
					EvaluatedConversationCount: 2,
					CompletionRate:             0.25,
				},
				{
					Attribute: &analyticspb.Attribute{
						TimeRanges: []*analyticspb.TimeRange{
							{
								StartTimestamp: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
								EndTimestamp:   timestamppb.New(time.Date(2023, 9, 30, 23, 59, 59, 0, time.UTC)),
							},
						},
					},
					AssignedConversationCount:  8,
					EvaluatedConversationCount: 0,
					CompletionRate:             0,
				},
				{
					Attribute:                  &analyticspb.Attribute{},
					AssignedConversationCount:  16,
					EvaluatedConversationCount: 2,
					CompletionRate:             0.125,
				},
			},
		}
		r.Require().Empty(cmp.Diff(want, resp, protocmp.Transform()))
	})
}
