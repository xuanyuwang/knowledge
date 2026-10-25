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
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/db/converter"
	"github.com/cresta/go-servers/shared/testing/cmputil"
)

type SuggestManualQAConversationsTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *SuggestManualQAConversationsTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
}

func (s *SuggestManualQAConversationsTestSuite) createFixtures(suggestMessages bool) {
	s.CleanDB()

	// Creates the agent users.
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("agent_id_1"),
		Username:    "agent_user_name_1",
		DisplayName: converter.ConvertToNullString("agent name 1"),
		FullName:    converter.ConvertToNullString("Agent 1"),
		UserRoles:   []int32{int32(authpb.AuthProto_AGENT)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("agent_id_2"),
		Username:    "agent_user_name_2",
		DisplayName: converter.ConvertToNullString("agent name 2"),
		FullName:    converter.ConvertToNullString("Agent 2"),
		UserRoles:   []int32{int32(authpb.AuthProto_AGENT)},
	}).Error)

	// Creates the QA analyst users.
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_1"),
		Username:    "analyst_user_name_1",
		DisplayName: converter.ConvertToNullString("analyst name 1"),
		FullName:    converter.ConvertToNullString("Analyst 1"),
		UserRoles:   []int32{int32(authpb.AuthProto_QA_SPECIALIST)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID:  converter.ConvertToNullString(s.customerID),
		UserID:      converter.ConvertToNullString("analyst_id_2"),
		Username:    "analyst_user_name_2",
		DisplayName: converter.ConvertToNullString("analyst name 2"),
		FullName:    converter.ConvertToNullString("Analyst 2"),
		UserRoles:   []int32{int32(authpb.AuthProto_QA_SPECIALIST)},
	}).Error)

	// Creates the conversations.
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_1"),
		ID:                1,
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
		ID:                2,
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
		ID:                3,
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
		ID:                4,
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
		ID:                5,
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
		ID:                6,
		PlatformID:        "testconversationid6a-platformid",
		PlatformAccountID: "testconversationid6a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_1"),
		CreatedAt:         time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC),
		EndedAt:           converter.ConvertToNullTime(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_7"),
		ID:                7,
		PlatformID:        "testconversationid7a-platformid",
		PlatformAccountID: "testconversationid7a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_1"),
		CreatedAt:         time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC),
		EndedAt:           converter.ConvertToNullTime(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID:        converter.ConvertToNullString(s.customerID),
		ProfileID:         converter.ConvertToNullString(s.profileID),
		UsecaseID:         converter.ConvertEmptyToInvalidNullString("usecase_id"),
		ConversationID:    converter.ConvertToNullString("convo_id_8"),
		ID:                8,
		PlatformID:        "testconversationid8a-platformid",
		PlatformAccountID: "testconversationid8a-platformaccountid",
		AgentUserID:       converter.ConvertToNullString("agent_id_2"),
		CreatedAt:         time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC),
		EndedAt:           converter.ConvertToNullTime(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
	}).Error)

	// Creates the messages.
	s.Require().NoError(s.DB().Create(&dbmodel.SpeakerRoleTypes{
		ID: "agent",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Messages{
		CustomerID:            converter.ConvertToNullString(s.customerID),
		ProfileID:             converter.ConvertToNullString(s.profileID),
		ConversationID:        converter.ConvertToNullString("convo_id_7"),
		ConversationMessageID: converter.ConvertToNullString("message_id_1"),
		AgentUserID:           converter.ConvertToNullString("agent_id_1"),
		ChatID:                8,
		PlatformSequenceID:    0,
		SpeakerRole:           "agent",
		Text:                  "hello",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Messages{
		CustomerID:            converter.ConvertToNullString(s.customerID),
		ProfileID:             converter.ConvertToNullString(s.profileID),
		ConversationID:        converter.ConvertToNullString("convo_id_8"),
		ConversationMessageID: converter.ConvertToNullString("message_id_2"),
		AgentUserID:           converter.ConvertToNullString("agent_id_2"),
		ChatID:                9,
		PlatformSequenceID:    0,
		SpeakerRole:           "agent",
		Text:                  "world",
	}).Error)

	// Creates the scorecard templates
	qaTaskConfig := &qadbpb.QATaskConfig{
		TaskCompleteConfig: &qadbpb.TaskCompleteConfig{
			EvaluatedConversationCountPerAgent: 5,
		},
		EvaluationPeriodConfig: &qadbpb.EvaluationPeriodConfig{
			StartTime:          timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EvaluationDuration: durationpb.New(40 * 365.25 * 24 * time.Hour), // 40 years
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
					Group:                              "group_id_1",
					EvaluatedConversationCountPerAgent: 1,
				},
			},
		},
		EvaluationPeriodConfig: &qadbpb.EvaluationPeriodConfig{
			StartTime:          timestamppb.New(time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)),
			EvaluationDuration: durationpb.New(40 * 365.25 * 24 * time.Hour), // 40 years
			OverrideTimeRange: &qadbpb.EvaluationPeriodConfig_TimeRange{
				StartTimestamp: timestamppb.New(time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2023, 7, 15, 0, 0, 0, 0, time.UTC)),
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
	qaScoreConfig := &qadbpb.QAScoreConfig{
		ScoringSubject: qadbpb.ScorecardScoringSubject_SCORECARD_SCORING_SUBJECT_MESSAGE,
	}
	qaScoreConfigJSON, err := protojson.Marshal(qaScoreConfig)
	s.Require().NoError(err)
	qaTaskConfigOverrideJSON, err := protojson.Marshal(qaTaskConfigOverride)
	s.Require().NoError(err)
	audienceJSON, err := protojson.Marshal(templateAudience)
	s.Require().NoError(err)
	scorecardTemplate := &dbmodel.ScorecardTemplates{
		Customer:     s.customerID,
		Profile:      s.profileID,
		ResourceID:   "scorecard_template_id",
		Revision:     "r1",
		UsecaseIds:   []string{"usecase_id"},
		Title:        "title1",
		Template:     "{}",
		QaTaskConfig: converter.ConvertEmptyToInvalidNullString(string(qaTaskConfigJSON)),
		Audience:     sql.NullString{String: string(audienceJSON), Valid: true},
	}
	if suggestMessages {
		scorecardTemplate.QaScoreConfig = converter.ConvertEmptyToInvalidNullString(string(qaScoreConfigJSON))
	}
	s.Require().NoError(s.DB().Create(scorecardTemplate).Error)
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
		TaskIds:          []string{"task_id_1"},
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
		TaskIds:          []string{"task_id_1"},
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
		TaskIds:          []string{"task_id_1"},
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
		TaskIds:          []string{"task_id_1"},
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
		TaskIds:          []string{"task_id_1"},
	}).Error)

	// set disable shuffle flag to true for consistent output
	DisableManualQAConversationsShuffle = true

	s.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func TestSuggestManualQAConversations(t *testing.T) {
	r := SuggestManualQAConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                    string
		esReqPath               string
		esRespPath              string
		req                     *analyticspb.SuggestManualQAConversationsRequest
		want                    *analyticspb.SuggestManualQAConversationsResponse
		listUsersReq            *userpb.ListUsersRequest
		listUsersResp           *userpb.ListUsersResponse
		listUsersFromGroupsReq  *userpb.ListUsersRequest
		listUsersFromGroupsResp *userpb.ListUsersResponse
		suggestMessages         bool
	}{
		{
			desc: "TestSuggestManualQAConversations",
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

			esReqPath:  "SuggestManualQAConversations_RetrieveClosedConversations_es_request.json",
			esRespPath: "SuggestManualQAConversations_RetrieveClosedConversations_es_response.json",
			req: &analyticspb.SuggestManualQAConversationsRequest{
				Parent: r.profileResourceName,
				ScorecardTemplateName: coachingpb.ScorecardTemplateName{
					CustomerID:                  "customer_id",
					ProfileID:                   "profile_id",
					ScorecardTemplateID:         "scorecard_template_id",
					ScorecardTemplateRevisionID: "r1",
				}.String(),
				DirectorTaskName: coachingpb.DirectorTaskName{
					CustomerID:     "customer_id",
					ProfileID:      "profile_id",
					DirectorTaskID: "task_id_1",
				}.String(),
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				SuggestedConversationsCount: 2,
			},
			want: &analyticspb.SuggestManualQAConversationsResponse{
				SuggestedConversations: []*analyticspb.ManualQAConversationInfo{
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_7",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid7a-platformid",
								PlatformAccountId:      "testconversationid7a-platformaccountid",
							},
							LegacyChatId:      7,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
					},
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_8",
							Agent: "customers/customer_id/users/agent_id_2",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 2",
							},
							AgentUserName: "agent_user_name_2",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid8a-platformid",
								PlatformAccountId:      "testconversationid8a-platformaccountid",
							},
							LegacyChatId:      8,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_2",
						},
					},
				},
			},
		},
		{
			desc: "TestSuggestManualQAConversations_PrioritizedAgents",
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
			esReqPath:  "SuggestManualQAConversations_PrioritizedAgents_RetrieveClosedConversations_es_request.json",
			esRespPath: "SuggestManualQAConversations_PrioritizedAgents_RetrieveClosedConversations_es_response.json",
			req: &analyticspb.SuggestManualQAConversationsRequest{
				Parent: r.profileResourceName,
				ScorecardTemplateName: coachingpb.ScorecardTemplateName{
					CustomerID:                  "customer_id",
					ProfileID:                   "profile_id",
					ScorecardTemplateID:         "scorecard_template_id",
					ScorecardTemplateRevisionID: "r1",
				}.String(),
				DirectorTaskName: coachingpb.DirectorTaskName{
					CustomerID:     "customer_id",
					ProfileID:      "profile_id",
					DirectorTaskID: "task_id_1",
				}.String(),
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				PrioritizedAgents: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
				},
				SuggestedConversationsCount: 2,
			},
			want: &analyticspb.SuggestManualQAConversationsResponse{
				SuggestedConversations: []*analyticspb.ManualQAConversationInfo{
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_7",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid7a-platformid",
								PlatformAccountId:      "testconversationid7a-platformaccountid",
							},
							LegacyChatId:      7,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
					},
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_6",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid6a-platformid",
								PlatformAccountId:      "testconversationid6a-platformaccountid",
							},
							LegacyChatId:      6,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
					},
				},
			},
		},
		{
			desc: "TestSuggestManualQAConversations_PrioritizedAgents_Override",
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
					Groups:                          []string{"group_id_1"},
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
			esReqPath:  "SuggestManualQAConversations_PrioritizedAgents_Override_RetrieveClosedConversations_es_request.json",
			esRespPath: "SuggestManualQAConversations_PrioritizedAgents_RetrieveClosedConversations_es_response.json",
			req: &analyticspb.SuggestManualQAConversationsRequest{
				Parent: r.profileResourceName,
				ScorecardTemplateName: coachingpb.ScorecardTemplateName{
					CustomerID:                  "customer_id",
					ProfileID:                   "profile_id",
					ScorecardTemplateID:         "scorecard_template_id_override",
					ScorecardTemplateRevisionID: "r1",
				}.String(),
				DirectorTaskName: coachingpb.DirectorTaskName{
					CustomerID:     "customer_id",
					ProfileID:      "profile_id",
					DirectorTaskID: "task_id_2",
				}.String(),
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				PrioritizedAgents: []*userpb.User{
					{
						Name:        "customers/customer_id/users/agent_id_1",
						DisplayName: "agent name 1",
					},
				},
				SuggestedConversationsCount: 2,
			},
			want: &analyticspb.SuggestManualQAConversationsResponse{
				SuggestedConversations: []*analyticspb.ManualQAConversationInfo{
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_7",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid7a-platformid",
								PlatformAccountId:      "testconversationid7a-platformaccountid",
							},
							LegacyChatId:      7,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
					},
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_6",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid6a-platformid",
								PlatformAccountId:      "testconversationid6a-platformaccountid",
							},
							LegacyChatId:      6,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
					},
				},
			},
		},
		{
			desc: "TestSuggestManualQAConversations_WithDirectorTask",
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
			esReqPath:  "SuggestManualQAConversations_RetrieveClosedConversations_es_request.json",
			esRespPath: "SuggestManualQAConversations_RetrieveClosedConversations_es_response.json",
			req: &analyticspb.SuggestManualQAConversationsRequest{
				Parent: r.profileResourceName,
				ScorecardTemplateName: coachingpb.ScorecardTemplateName{
					CustomerID:                  "customer_id",
					ProfileID:                   "profile_id",
					ScorecardTemplateID:         "scorecard_template_id",
					ScorecardTemplateRevisionID: "r1",
				}.String(),
				DirectorTaskName: coachingpb.DirectorTaskName{
					CustomerID:     "customer_id",
					ProfileID:      "profile_id",
					DirectorTaskID: "task_id_1",
				}.String(),
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				SuggestedConversationsCount: 2,
			},
			want: &analyticspb.SuggestManualQAConversationsResponse{
				SuggestedConversations: []*analyticspb.ManualQAConversationInfo{
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_7",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid7a-platformid",
								PlatformAccountId:      "testconversationid7a-platformaccountid",
							},
							LegacyChatId:      7,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
					},
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_8",
							Agent: "customers/customer_id/users/agent_id_2",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 2",
							},
							AgentUserName: "agent_user_name_2",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid8a-platformid",
								PlatformAccountId:      "testconversationid8a-platformaccountid",
							},
							LegacyChatId:      8,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_2",
						},
					},
				},
			},
		},
		{
			desc:            "TestSuggestManualQAConversations_SuggestMessages",
			suggestMessages: true,
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

			esReqPath:  "SuggestManualQAConversations_RetrieveClosedConversations_es_request.json",
			esRespPath: "SuggestManualQAConversations_RetrieveClosedConversations_es_response.json",
			req: &analyticspb.SuggestManualQAConversationsRequest{
				Parent: r.profileResourceName,
				ScorecardTemplateName: coachingpb.ScorecardTemplateName{
					CustomerID:                  "customer_id",
					ProfileID:                   "profile_id",
					ScorecardTemplateID:         "scorecard_template_id",
					ScorecardTemplateRevisionID: "r1",
				}.String(),
				DirectorTaskName: coachingpb.DirectorTaskName{
					CustomerID:     "customer_id",
					ProfileID:      "profile_id",
					DirectorTaskID: "task_id_1",
				}.String(),
				QaAnalyst: &userpb.User{
					Name: "customers/customer_id/users/analyst_id_1",
				},
				SuggestedConversationsCount: 2,
			},
			want: &analyticspb.SuggestManualQAConversationsResponse{
				SuggestedConversations: []*analyticspb.ManualQAConversationInfo{
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_7",
							Agent: "customers/customer_id/users/agent_id_1",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 1",
							},
							AgentUserName: "agent_user_name_1",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid7a-platformid",
								PlatformAccountId:      "testconversationid7a-platformaccountid",
							},
							LegacyChatId:      7,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_1",
						},
						MessageNames: []string{"customers/customer_id/profiles/profile_id/conversations/convo_id_7/messages/message_id_1"},
					},
					{
						Conversation: &conversation.Conversation{
							Name:  "customers/customer_id/profiles/profile_id/conversations/convo_id_8",
							Agent: "customers/customer_id/users/agent_id_2",
							AgentInfo: &conversation.ConversationMessage_SpeakerInfo{
								DisplayName: "Agent 2",
							},
							AgentUserName: "agent_user_name_2",
							PlatformInfo: &conversation.Conversation_PlatformInfo{
								PlatformConversationId: "testconversationid8a-platformid",
								PlatformAccountId:      "testconversationid8a-platformaccountid",
							},
							LegacyChatId:      8,
							CreateTime:        timestamppb.New(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
							Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
							State:             conversation.Conversation_CLOSED,
							MultiLanguageMode: conversation.Conversation_NONE,
						},
						Agent: &userpb.User{
							Name: "customers/customer_id/users/agent_id_2",
						},
						MessageNames: []string{"customers/customer_id/profiles/profile_id/conversations/convo_id_8/messages/message_id_2"},
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			r.createFixtures(tc.suggestMessages)
			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(2).
				Return(&config.ProfileConfig{}, nil)

			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			if tc.listUsersReq != nil && tc.listUsersResp != nil {
				r.mockUserClient.EXPECT().ListUsers(gomock.Any(), tc.listUsersReq).Times(1).Return(tc.listUsersResp, nil)
			}
			if tc.listUsersFromGroupsReq != nil && tc.listUsersFromGroupsResp != nil {
				r.mockUserClient.EXPECT().ListUsers(gomock.Any(), tc.listUsersFromGroupsReq).Times(1).Return(tc.listUsersFromGroupsResp, nil)
			}
			resp, err := r.Client.SuggestManualQAConversations(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform(),
					protocmp.IgnoreFields(&conversation.Conversation{}, "start_time", "end_time", "platform_info", "raise_hand_state", "audio_persistence_state", "extended_info", "audio_retention_info")); diff != "" {
					t.Fatalf("SuggestManualQAConversations return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}

// TestSuggestManualQAConversations_InProgressDraftLock verifies that agents with active
// in-progress manual scorecards (updated within the lock window) are deprioritized in
// TaskHome suggestions, preventing double-assignment to a second reviewer.
func TestSuggestManualQAConversations_InProgressDraftLock(t *testing.T) {
	r := SuggestManualQAConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	// Given
	r.createFixtures(false)
	// Standard fixtures give agent_id_1: 2 submitted / quota 5 (rate=0.4)
	// and agent_id_2: 3 submitted / quota 5 (rate=0.6), so agent_id_1 is normally
	// suggested first. Adding 2 in-progress drafts for agent_id_1 raises its effective
	// rate to (2+2)/5=0.8 > 0.6, so agent_id_2 should be suggested first.
	for i, convoID := range []string{"convo_id_6", "convo_id_7"} {
		r.Require().NoError(r.DB().Create(&dbmodel.Scorecards{
			Customer:         r.customerID,
			Profile:          r.profileID,
			ResourceID:       fmt.Sprintf("draft_scorecard_%d", i),
			ConversationID:   convoID,
			TemplateID:       "scorecard_template_id",
			TemplateRevision: "r1",
			AgentUserID:      "agent_id_1",
			ManuallyScored:   converter.ConvertToNullBool(true),
		}).Error)
	}

	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(2).
		Return(&config.ProfileConfig{}, nil)
	r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile("SuggestManualQAConversations_RetrieveClosedConversations_es_response.json"))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("SuggestManualQAConversations_RetrieveClosedConversations_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	r.mockUserClient.EXPECT().ListUsers(gomock.Any(), &userpb.ListUsersRequest{
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
	}).Times(1).Return(&userpb.ListUsersResponse{
		Users: []*userpb.User{
			{Name: "customers/customer_id/users/agent_id_1", DisplayName: "agent name 1"},
			{Name: "customers/customer_id/users/agent_id_2", DisplayName: "agent name 2"},
		},
	}, nil)

	// When
	resp, err := r.Client.SuggestManualQAConversations(r.ctx, &analyticspb.SuggestManualQAConversationsRequest{
		Parent: r.profileResourceName,
		ScorecardTemplateName: coachingpb.ScorecardTemplateName{
			CustomerID:                  "customer_id",
			ProfileID:                   "profile_id",
			ScorecardTemplateID:         "scorecard_template_id",
			ScorecardTemplateRevisionID: "r1",
		}.String(),
		DirectorTaskName: coachingpb.DirectorTaskName{
			CustomerID:     "customer_id",
			ProfileID:      "profile_id",
			DirectorTaskID: "task_id_1",
		}.String(),
		QaAnalyst:                   &userpb.User{Name: "customers/customer_id/users/analyst_id_1"},
		SuggestedConversationsCount: 2,
	})

	// Then
	if err != nil {
		t.Fatalf("Got code %v in error: %v", err, status.Code(err))
	}
	want := &analyticspb.SuggestManualQAConversationsResponse{
		SuggestedConversations: []*analyticspb.ManualQAConversationInfo{
			{
				Conversation: &conversation.Conversation{
					Name:              "customers/customer_id/profiles/profile_id/conversations/convo_id_8",
					Agent:             "customers/customer_id/users/agent_id_2",
					AgentInfo:         &conversation.ConversationMessage_SpeakerInfo{DisplayName: "Agent 2"},
					AgentUserName:     "agent_user_name_2",
					LegacyChatId:      8,
					CreateTime:        timestamppb.New(time.Date(2023, 9, 10, 12, 0, 0, 0, time.UTC)),
					Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
					State:             conversation.Conversation_CLOSED,
					MultiLanguageMode: conversation.Conversation_NONE,
				},
				Agent: &userpb.User{Name: "customers/customer_id/users/agent_id_2"},
			},
			{
				Conversation: &conversation.Conversation{
					Name:              "customers/customer_id/profiles/profile_id/conversations/convo_id_7",
					Agent:             "customers/customer_id/users/agent_id_1",
					AgentInfo:         &conversation.ConversationMessage_SpeakerInfo{DisplayName: "Agent 1"},
					AgentUserName:     "agent_user_name_1",
					LegacyChatId:      7,
					CreateTime:        timestamppb.New(time.Date(2023, 9, 11, 12, 0, 0, 0, time.UTC)),
					Usecase:           "customers/customer_id/profiles/profile_id/usecases/usecase_id",
					State:             conversation.Conversation_CLOSED,
					MultiLanguageMode: conversation.Conversation_NONE,
				},
				Agent: &userpb.User{Name: "customers/customer_id/users/agent_id_1"},
			},
		},
	}
	if diff := cmp.Diff(want, resp,
		protocmp.Transform(),
		protocmp.IgnoreFields(&conversation.Conversation{}, "start_time", "end_time", "platform_info", "raise_hand_state", "audio_persistence_state", "extended_info", "audio_retention_info")); diff != "" {
		t.Fatalf("unexpected response diff (-want +got):\n%v", diff)
	}
}
