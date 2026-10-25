package analyticsimpl

import (
	"context"
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

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsdbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/analytics"
	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/db/converter"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/go-servers/shared/testing/mock"
)

type RetrieveManualQAProgressTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveManualQAProgressTestSuite) SetupSuite() {
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
		ConversationSuggestionConfig: &qadbpb.ConversationSuggestionConfig{},
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
					EvaluatedConversationCountPerAgent: 1,
				},
				{
					Group:                              userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_2"}.String(),
					EvaluatedConversationCountPerAgent: 4,
				},
			},
		},
		EvaluationPeriodConfig: &qadbpb.EvaluationPeriodConfig{
			StartTime:                timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EvaluationPeriodDuration: qadbpb.EvaluationPeriodConfig_EVALUATION_PERIOD_DURATION_ONE_MONTH,
			OverrideTimeRange: &qadbpb.EvaluationPeriodConfig_TimeRange{
				StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2023, 8, 14, 23, 59, 59, 999999999, time.UTC)),
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

	qaTaskConfigOverrideTemplateAudience := &qadbpb.QATaskConfig{
		TaskCompleteConfig: &qadbpb.TaskCompleteConfig{
			EvaluatedConversationCountPerAgent: 5,
			Audience: &qadbpb.TaskAudience{
				Users: []string{userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String()},
			},
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
		ConversationSuggestionConfig: &qadbpb.ConversationSuggestionConfig{},
	}
	qaTaskConfigOverrideTemplateAudienceJSON, err := protojson.Marshal(qaTaskConfigOverrideTemplateAudience)
	s.Require().NoError(err)

	audienceJSON, err := protojson.Marshal(templateAudience)
	s.Require().NoError(err)
	s.Require().NoError(s.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id",
		Revision:     "r1",
		UsecaseIds:   []string{"usecase_id"},
		Title:        "title1",
		Template:     "{}",
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
		Template:     "{}",
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(qaTaskConfigOverrideJSON)),
		Audience:     sql.NullString{String: string(audienceJSON), Valid: true},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id_explicit_task_template_audience",
		Revision:     "r1",
		UsecaseIds:   []string{"usecase_id"},
		Title:        "title1",
		Template:     "{}",
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(qaTaskConfigOverrideTemplateAudienceJSON)),
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

	s.Require().NoError(s.DB().Create(&dbmodel.Tasks{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		TaskID:            "task_id_explicit_task_template_audience",
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
				QaAnalystQuotas: qaTaskConfigOverrideTemplateAudience.QaAnalystQuotaConfig.QaAnalystQuotas,
			},
		}},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
			QmTaskCompleteConfig: qaTaskConfigOverrideTemplateAudience.TaskCompleteConfig,
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: qaTaskConfigOverrideTemplateAudience.EvaluationPeriodConfig,
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          "scorecard_template_id_explicit_task_template_audience",
				ConversationSuggestionConfig: qaTaskConfigOverrideTemplateAudience.ConversationSuggestionConfig,
			},
		}},
	}).Error)

	conversationEndedAtSuggestionConfig := &qadbpb.ConversationSuggestionConfig{
		TargetFieldForTimeRange: analyticsdbpb.TargetFieldForTimeRange_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
	}
	s.Require().NoError(s.DB().Create(&dbmodel.Tasks{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		TaskID:            "task_id_conversation_ended_at",
		TaskDisplayName:   sql.NullString{String: "Task conversation ended at", Valid: true},
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
			QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
				EvaluatedConversationCountPerAgent: 5,
			},
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: qaTaskConfig.EvaluationPeriodConfig,
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          "scorecard_template_id",
				ConversationSuggestionConfig: conversationEndedAtSuggestionConfig,
			},
		}},
	}).Error)

	s.Require().NoError(s.DB().Create(&dbmodel.Tasks{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
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
			UserIds: []string{"agent_id_1", "agent_id_2"},
			QaAnalystQuotaConfig: &qadbpb.QAAnalystQuotaConfig{
				QaAnalystQuotas: qaTaskConfig.QaAnalystQuotaConfig.QaAnalystQuotas,
			},
		}},
		TaskCompleteConfig: qadbpb.DirectorTaskCompleteConfigSQL{Message: &qadbpb.DirectorTaskCompleteConfig{
			QmTaskCompleteConfig: &qadbpb.TaskCompleteConfig{
				EvaluatedConversationCountPerAgent: 5,
				CompletionType:                     qadbpb.TaskCompleteConfig_COMPLETION_TYPE_CONVERSATION_COVERAGE_PERCENTAGE,
				ConversationCoveragePercentage:     7,
			},
		}},
		TaskScheduleConfig: qadbpb.DirectorTaskScheduleConfigSQL{Message: &qadbpb.DirectorTaskScheduleConfig{
			QmTaskScheduleConfig: qaTaskConfig.EvaluationPeriodConfig,
		}},
		TaskContentConfig: qadbpb.DirectorTaskContentConfigSQL{Message: &qadbpb.DirectorTaskContentConfig{
			QmTaskContentConfig: &qadbpb.QMTaskContentConfig{
				ScorecardTemplateId:          "scorecard_template_id",
				ConversationSuggestionConfig: conversationEndedAtSuggestionConfig,
			},
		}},
	}).Error)

	// Creates the scorecards.
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_1",
		ConversationID:   "convo_id_1",
		MessageIds:       []string{"message_id_1", "message_id_2"},
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(100.0),
		Comment:          converter.ConvertToNullString("good job"),
		AutoFailed:       converter.ConvertToNullBool(false),
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
		TaskIds:          []string{"task_id_1"},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_5",
		ConversationID:   "convo_id_5",
		TemplateID:       "scorecard_template_id",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_2",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_2"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(70.0),
		AutoFailed:       converter.ConvertToNullBool(false),
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
	s.Require().NoError(s.DB().Create(&dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "scorecard_id_1_explicit_task_template_audience",
		ConversationID:   "convo_id_1",
		TemplateID:       "scorecard_template_id_explicit_task_template_audience",
		TemplateRevision: "r1",
		AgentUserID:      "agent_id_1",
		SubmitterUserID:  converter.ConvertToNullString("analyst_id_1"),
		SubmittedAt:      converter.ConvertToNullTime(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
		ManuallyScored:   converter.ConvertToNullBool(true),
		Score:            converter.ConvertToNullFloat64(100.0),
		Comment:          converter.ConvertToNullString("good job"),
		AutoFailed:       converter.ConvertToNullBool(false),
		TaskIds:          []string{"task_id_explicit_task_template_audience"},
	}).Error)
}

func TestRetrieveManualQAProgress(t *testing.T) {
	r := RetrieveManualQAProgressTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                    string
		esReqPath               string
		esRespPath              string
		req                     *analyticspb.RetrieveManualQAProgressRequest
		want                    *analyticspb.RetrieveManualQAProgressResponse
		wantErr                 error
		listUsersReq            *userpb.ListUsersRequest
		listUsersResp           *userpb.ListUsersResponse
		listUsersFromGroupsReq  []*userpb.ListUsersRequest
		listUsersFromGroupsResp map[string][]*userpb.User
	}{
		{
			desc: "TestRetrieveManualQAProgress_HandledConversationsCount_WithTimeRange",
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
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
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
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: false,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_2",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime: timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
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
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime: timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_HandledConversationsCount",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_es_request.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCount_es_response.json",
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
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
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
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_2",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 100,
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
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 200,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_HandledConversationsCount_ConversationEndedAt",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_ConversationEndedAt_es_request.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCount_es_response.json",
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
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
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
							DirectorTaskID: "task_id_conversation_ended_at",
						}.String(),
					},
				},
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount: 5,
						HandledConversationsCount: 100,
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
						AssignedConversationCount: 5,
						HandledConversationsCount: 200,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_ConversationCoveragePercentage",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_ConversationEndedAt_es_request.json",
			esRespPath: "RetrieveManualQAProgress_ConversationCoverage_es_response.json",
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
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
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
							DirectorTaskID: "task_id_conversation_coverage",
						}.String(),
					},
				},
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount: 4,
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
						AssignedConversationCount: 11,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_FilterAgent",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_FilteredAgents_es_request.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCount_FilteredAgents_es_response.json",
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
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
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
					Users: []*userpb.User{
						{
							Name:        "customers/customer_id/users/agent_id_1",
							DisplayName: "agent name 1",
						},
					},
				},
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_2",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 100,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_WithOverrideAgents",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_es_request_with_override.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCount_es_response.json",
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
			listUsersFromGroupsReq: []*userpb.ListUsersRequest{
				{
					Parent:   "customers/customer_id",
					PageSize: 6000,
					Filter: &userpb.ListUsersRequest_ListUsersFilter{
						State:                           userpb.User_ACTIVE,
						Groups:                          []string{"group_id_1"},
						IncludeIndirectGroupMemberships: true,
						CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
						Roles:                           []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
					},
				},
				{
					Parent:   "customers/customer_id",
					PageSize: 6000,
					Filter: &userpb.ListUsersRequest_ListUsersFilter{
						State:                           userpb.User_ACTIVE,
						Groups:                          []string{"group_id_2"},
						IncludeIndirectGroupMemberships: true,
						CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
						Roles:                           []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
					},
				},
			},
			listUsersFromGroupsResp: map[string][]*userpb.User{
				userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_1"}.String(): {
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
				userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_2"}.String(): {},
			},
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
				},
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
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount:  2,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 100,
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
						AssignedConversationCount:  1,
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 200,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_WithOverrideAgentsInSameGroup",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_es_request_with_override.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCount_es_response.json",
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
			listUsersFromGroupsReq: []*userpb.ListUsersRequest{
				{
					Parent:   "customers/customer_id",
					PageSize: 6000,
					Filter: &userpb.ListUsersRequest_ListUsersFilter{
						CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
						IncludeIndirectGroupMemberships: true,
						Groups:                          []string{userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_1"}.String()},
					},
				},
				{
					Parent:   "customers/customer_id",
					PageSize: 6000,
					Filter: &userpb.ListUsersRequest_ListUsersFilter{
						CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
						IncludeIndirectGroupMemberships: true,
						Groups:                          []string{userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_2"}.String()},
					},
				},
			},
			listUsersFromGroupsResp: map[string][]*userpb.User{
				userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_1"}.String(): {
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
				userpb.GroupName{CustomerID: "customer_id", GroupID: "group_id_2"}.String(): {
					{
						Name:        "customers/customer_id/users/agent_id_2",
						DisplayName: "agent name 2",
					},
				},
			},
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
				},
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
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount:  2,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 100,
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
						AssignedConversationCount:  4,
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 200,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_WithTaskID",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCount_es_request.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCount_es_response.json",
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
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
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
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 2,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
								},
								MessageNames: []string{
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_1",
									"customers/customer_id/profiles/profile_id/conversations/convo_id_1/messages/message_id_2",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 2, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 100,
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
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 3,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_3",
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
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_2"}.String(),
							DisplayName: "analyst name 2",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 10, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 200,
					},
				},
			},
		},
		{
			desc:       "TestRetrieveManualQAProgress_TaskAudienceOverridesTemplateAudience",
			esReqPath:  "RetrieveManualQAProgress_HandledConversationsCountExplicitTaskAudience_es_request.json",
			esRespPath: "RetrieveManualQAProgress_HandledConversationsCountExplicitTaskAudience_es_response.json",
			listUsersReq: &userpb.ListUsersRequest{
				Parent:    "customers/customer_id",
				ProfileId: "profile_id",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					State:   userpb.User_ACTIVE,
					Roles:   []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
					UserIds: []string{"agent_id_1"},
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
				},
			},
			req: &analyticspb.RetrieveManualQAProgressRequest{
				Parent: r.profileResourceName,
				FilterByEvaluationPeriods: []*analyticspb.TimeRange{
					{
						StartTimestamp: timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2023, 8, 31, 23, 59, 59, 999999999, time.UTC)),
					},
				},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						coachingpb.ScorecardTemplateName{
							CustomerID:                  "customer_id",
							ProfileID:                   "profile_id",
							ScorecardTemplateID:         "scorecard_template_id_explicit_task_template_audience",
							ScorecardTemplateRevisionID: "r1",
						}.String(),
					},
					TaskNames: []string{
						coachingpb.DirectorTaskName{
							CustomerID:     "customer_id",
							ProfileID:      "profile_id",
							DirectorTaskID: "task_id_explicit_task_template_audience",
						}.String(),
					},
				},
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				ReturnHandledConversationsCount: true,
			},
			want: &analyticspb.RetrieveManualQAProgressResponse{
				ManualQaProgresses: []*analyticspb.ManualQAProgress{
					{
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{
								{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
							},
						},
						AssignedConversationCount:  5,
						EvaluatedConversationCount: 1,
						EvaluatedConversations: []*analyticspb.ManualQAConversationInfo{
							{
								Conversation: &conversationpb.Conversation{
									Name: "customers/customer_id/profiles/profile_id/conversations/convo_id_1",
								},
								ScorecardName: "customers/customer_id/profiles/profile_id/scorecards/scorecard_id_1_explicit_task_template_audience",
								Agent: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "agent_id_1"}.String(),
									DisplayName: "agent name 1",
								},
								QaAnalyst: &userpb.User{
									Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
									DisplayName: "analyst name 1",
								},
							},
						},
						LastQaAnalyst: &userpb.User{
							Name:        userpb.UserName{CustomerID: "customer_id", UserID: "analyst_id_1"}.String(),
							DisplayName: "analyst name 1",
						},
						LastEvaluatedTime:         timestamppb.New(time.Date(2023, 8, 1, 12, 0, 0, 0, time.UTC)),
						HandledConversationsCount: 100,
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(&config.CustomerConfig{}, nil)
			if tc.esReqPath != "" {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
				r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			}
			if tc.listUsersReq != nil && tc.listUsersResp != nil {
				r.mockUserClient.EXPECT().ListUsers(gomock.Any(), tc.listUsersReq).Times(1).Return(tc.listUsersResp, nil)
			}

			if len(tc.listUsersFromGroupsReq) > 0 {
				r.mockUserClient.EXPECT().ListUsers(gomock.Any(), mock.ProtoEq(&userpb.ListUsersRequest{
					Parent:                 "customers/customer_id",
					ProfileId:              "profile_id",
					IncludeUserMemberships: true,
					OrderField:             "user_id",
					Filter: &userpb.ListUsersRequest_ListUsersFilter{
						CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
						IncludeIndirectGroupMemberships: true,
						State:                           userpb.User_ACTIVE,
						Roles:                           []authpb.AuthProto_Role{authpb.AuthProto_AGENT},
					},
				}, protocmp.IgnoreFields(&userpb.ListUsersRequest_ListUsersFilter{}, "groups"))).Times(len(tc.listUsersFromGroupsReq)).DoAndReturn(func(ctx context.Context, req *userpb.ListUsersRequest, opts ...grpc.CallOption) (*userpb.ListUsersResponse, error) {
					r.Require().Len(req.GetFilter().GetGroups(), 1, "request should be exactly per group")

					return &userpb.ListUsersResponse{
						Users: tc.listUsersFromGroupsResp[req.GetFilter().GetGroups()[0]],
					}, nil
				})
			}

			resp, err := r.Client.RetrieveManualQAProgress(r.ctx, tc.req)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v, got nil", tc.wantErr)
				}
				if diff := cmp.Diff(tc.wantErr.Error(), err.Error()); diff != "" {
					t.Fatalf("RetrieveManualQAProgress return unexpected error with diff (-want +got):\n %v", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("RetrieveManualQAProgress return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}
