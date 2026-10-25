package analyticsimpl

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbqapb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedauth "github.com/cresta/go-servers/shared/auth"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	sharedqa "github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"
	testingauth "github.com/cresta/go-servers/testing/shared/auth"
)

func TestRetrieveGroupCalibrationStats(t *testing.T) {
	suite.Run(t, &RetrieveGroupCalibrationStatsTestSuite{})
}

type RetrieveGroupCalibrationStatsTestSuite struct {
	AnalyticsTestSuite

	authEntityProvider sharedauth.AuthEntityProvider

	templateSturct     scoring.ScorecardTemplateStructure
	templateSturctJSON string

	// Criterion constants
	numericRadioCriterionID   string
	labelRadioCriterionID     string
	sentenceCriterionID       string
	dropdownNumeriCriterionID string

	numericRadioCriterionDisplayName    string
	labelRadioCriterionDisplayName      string
	sentenceCriterionDisplayName        string
	dropdownNumericCriterionDisplayName string

	criterionNumericValue1 float64
	criterionNumericValue2 float64
	criterionNumericValue3 float64

	scoreResourcePrefix string

	taskAID string
	taskBID string

	// Time constants
	taskCreatedTime time.Time
	taskDueDate     time.Time
}

func (s *RetrieveGroupCalibrationStatsTestSuite) CleanDB() {
	db := s.DB()
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS director CASCADE").Error)
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS historic CASCADE").Error)
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS app CASCADE").Error)

	s.LoadAppSchema(s.T())
	s.LoadDirectorSchema(s.T())
}

func (s *RetrieveGroupCalibrationStatsTestSuite) SetupSuite() {
	s.SetupTestDatabase(s.T())
	s.customerID = "test-customer-id"
	s.profileID = "test-profile-id"
	s.usecaseID = "test-usecase-id"

	s.taskAID = "taskA-" + uuid.NewString()
	s.taskBID = "taskB-" + uuid.NewString()

	// Initialize constants
	s.numericRadioCriterionID = "numeric-radios-criterion"
	s.labelRadioCriterionID = "labeled-radios-criterion"
	s.sentenceCriterionID = "sentence-criterion"
	s.dropdownNumeriCriterionID = "dropdown-numeric-criterion"

	s.numericRadioCriterionDisplayName = s.numericRadioCriterionID + "-displayname"
	s.labelRadioCriterionDisplayName = s.labelRadioCriterionID + "-displayname"
	s.sentenceCriterionDisplayName = s.sentenceCriterionID + "-displayname"
	s.dropdownNumericCriterionDisplayName = s.dropdownNumeriCriterionID + "-displayname"

	s.criterionNumericValue1 = 10.0
	s.criterionNumericValue2 = 3.0
	s.criterionNumericValue3 = 5.0

	s.scoreResourcePrefix = "score-"

	s.taskCreatedTime = time.Date(2021, 11, 24, 12, 0, 0, 0, time.UTC)
	s.taskDueDate = time.Date(2021, 11, 30, 12, 0, 0, 0, time.UTC)

	s.templateSturct = &scoring.ScorecardTemplateStructureV2{
		Version: 2,
		Items: []scoring.ScorecardTemplateStructureNode{
			scoring.SentenceCriterion{
				BaseCriterion: scoring.BaseCriterion{
					Identifier:  s.sentenceCriterionID,
					DisplayName: s.sentenceCriterionDisplayName,
					Weight:      1,
					Type:        scoring.SentenceTemplateType,
				},
			},
			scoring.NumericRadiosCriterion{
				BaseCriterion: scoring.BaseCriterion{
					Identifier:  s.numericRadioCriterionID,
					DisplayName: s.numericRadioCriterionDisplayName,
					Weight:      1,
					Type:        scoring.NumericRadiosCriterionTemplateType,
				},
				Settings: &scoring.NumericRadiosCriterionSettings{
					CriterionWithValueSettings: scoring.CriterionWithValueSettings{
						ExcludeFromQAScores: &[]bool{false}[0],
					},
				},
			},
			scoring.LabeledRadiosCriterion{
				BaseCriterion: scoring.BaseCriterion{
					Identifier:  s.labelRadioCriterionID,
					DisplayName: s.labelRadioCriterionDisplayName,
					Weight:      1,
					Type:        scoring.LabeledRadiosCriterionTemplateType,
				},
				LabeledCriterion: scoring.LabeledCriterion{
					Settings: &scoring.LabeledCriterionTypeOptionSettings{
						CriterionWithValueSettings: scoring.CriterionWithValueSettings{
							ExcludeFromQAScores: &[]bool{false}[0],
						},
					},
				},
			},
			scoring.DropdownNumericCriterion{
				BaseCriterion: scoring.BaseCriterion{
					Identifier:  s.dropdownNumeriCriterionID,
					DisplayName: s.dropdownNumericCriterionDisplayName,
					Weight:      1,
					Type:        scoring.DropdownNumericCriterionTemplateType,
				},
				LabeledCriterion: scoring.LabeledCriterion{
					Settings: &scoring.LabeledCriterionTypeOptionSettings{
						CriterionWithValueSettings: scoring.CriterionWithValueSettings{
							ExcludeFromQAScores: &[]bool{false}[0],
						},
					},
				},
			},
		},
	}
	templateSturctJSON, err := json.Marshal(s.templateSturct)
	s.Require().NoError(err)
	s.templateSturctJSON = string(templateSturctJSON)

	controllerE := gomock.NewController(s.T())
	s.authEntityProvider = authmocks.NewMockAuthEntityProvider(controllerE)

	// Default auth entity expectation for all tests
	s.authEntityProvider.(*authmocks.MockAuthEntityProvider).EXPECT().GetAuthEntity(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, customerID string) (*sharedauth.PrincipalEntity, error) {
			return sharedauth.GetPrincipalEntityFromAuthContext(ctx)
		})
}

func (s *RetrieveGroupCalibrationStatsTestSuite) SetupTest() {
	s.CleanDB()
}

func (s *RetrieveGroupCalibrationStatsTestSuite) SetupSubTest() {
	s.CleanDB()
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestRetrieveGroupCalibrationData() {
	// Given
	template := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "01 The original current template",
		CreatedAt:  s.taskCreatedTime,
		UsecaseIds: []string{s.usecaseID},
		Template:   s.templateSturctJSON,
	}

	qaAdminUserID := uuid.NewString()
	qaAnalystSubmitterUserID := uuid.NewString()
	qaAnalystSubmitterUserID2 := uuid.NewString()
	taskA := &dbmodel.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{Valid: true, String: s.usecaseID},
		TaskID:     uuid.NewString(),
		CreatedAt:  s.taskCreatedTime,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION,
		TaskScheduleConfig: dbqapb.DirectorTaskScheduleConfigSQL{
			Message: &dbqapb.DirectorTaskScheduleConfig{
				GroupCalibrationScheduleConfig: &dbqapb.GroupCalibrationTaskScheduleConfig{
					DueDate: timestamppb.New(s.taskDueDate),
				},
			},
		},
		TaskContentConfig: dbqapb.DirectorTaskContentConfigSQL{
			Message: &dbqapb.DirectorTaskContentConfig{
				GroupCalibrationContentConfig: &dbqapb.GroupCalibrationTaskContentConfig{
					Tasks: []*dbqapb.GroupCalibrationTaskContentConfig_GroupCalibrationTask{
						{ScorecardTemplateId: template.ResourceID, ScorecardTemplateRevisionId: template.Revision},
					},
				},
			},
		},
		TaskAudienceConfig: dbqapb.DirectorTaskAudienceConfigSQL{
			Message: &dbqapb.DirectorTaskAudienceConfig{
				UserIds: []string{qaAnalystSubmitterUserID},
			},
		},
	}
	taskB := &dbmodel.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{Valid: true, String: s.usecaseID},
		TaskID:     uuid.NewString(),
		CreatedAt:  s.taskCreatedTime,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION,
		TaskAudienceConfig: dbqapb.DirectorTaskAudienceConfigSQL{
			Message: &dbqapb.DirectorTaskAudienceConfig{
				UserIds: []string{qaAnalystSubmitterUserID2},
			},
		},
		TaskScheduleConfig: dbqapb.DirectorTaskScheduleConfigSQL{
			Message: &dbqapb.DirectorTaskScheduleConfig{
				GroupCalibrationScheduleConfig: &dbqapb.GroupCalibrationTaskScheduleConfig{
					DueDate: timestamppb.New(s.taskDueDate),
				},
			},
		},
		TaskContentConfig: dbqapb.DirectorTaskContentConfigSQL{
			Message: &dbqapb.DirectorTaskContentConfig{
				GroupCalibrationContentConfig: &dbqapb.GroupCalibrationTaskContentConfig{
					Tasks: []*dbqapb.GroupCalibrationTaskContentConfig_GroupCalibrationTask{
						{ScorecardTemplateId: template.ResourceID, ScorecardTemplateRevisionId: template.Revision},
					},
				},
			},
		},
	}

	agentUserID := uuid.NewString()
	taskAAnswerKeyScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        taskA.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        taskA.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: taskA.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAdminUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAdminUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{taskA.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY), Valid: true},
	}
	taskAAnswerKeyScores := map[string]*dbmodel.Scores{
		s.sentenceCriterionID: {
			Customer:            taskAAnswerKeyScorecard.Customer,
			Profile:             taskAAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		s.numericRadioCriterionID: {
			Customer:            taskAAnswerKeyScorecard.Customer,
			Profile:             taskAAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.labelRadioCriterionID: {
			Customer:            taskAAnswerKeyScorecard.Customer,
			Profile:             taskAAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.dropdownNumeriCriterionID: {
			Customer:            taskAAnswerKeyScorecard.Customer,
			Profile:             taskAAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	taskAResponseScorecardA := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        taskA.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        taskA.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: taskA.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystSubmitterUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystSubmitterUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{taskA.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}
	taskAResponseScoresA := map[string]*dbmodel.Scores{
		s.sentenceCriterionID: {
			Customer:            taskAResponseScorecardA.Customer,
			Profile:             taskAResponseScorecardA.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAResponseScorecardA.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		s.numericRadioCriterionID: { // same as answer key
			Customer:            taskAResponseScorecardA.Customer,
			Profile:             taskAResponseScorecardA.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAResponseScorecardA.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.labelRadioCriterionID: { // diff from answer key
			Customer:            taskAResponseScorecardA.Customer,
			Profile:             taskAResponseScorecardA.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAResponseScorecardA.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		// dropdown criterion is not present in the response scorecard
	}

	taskAResponseScorecardB := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        taskA.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        taskA.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: taskA.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystSubmitterUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystSubmitterUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{taskA.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}
	taskAResponseScoresB := map[string]*dbmodel.Scores{
		s.sentenceCriterionID: {
			Customer:            taskAResponseScorecardB.Customer,
			Profile:             taskAResponseScorecardB.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAResponseScorecardB.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		s.numericRadioCriterionID: { // same as answer key
			Customer:            taskAResponseScorecardB.Customer,
			Profile:             taskAResponseScorecardB.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAResponseScorecardB.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.dropdownNumeriCriterionID: { // diff from answer key
			Customer:            taskAResponseScorecardB.Customer,
			Profile:             taskAResponseScorecardB.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskAResponseScorecardB.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		// label radio criterion is not present in the response scorecard
	}

	taskBAnswerKeyScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        taskB.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        taskB.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: taskB.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAdminUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAdminUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{taskB.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY), Valid: true},
	}
	taskBAnswerKeyScores := map[string]*dbmodel.Scores{
		s.sentenceCriterionID: {
			Customer:            taskBAnswerKeyScorecard.Customer,
			Profile:             taskBAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		s.numericRadioCriterionID: {
			Customer:            taskBAnswerKeyScorecard.Customer,
			Profile:             taskBAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.labelRadioCriterionID: {
			Customer:            taskBAnswerKeyScorecard.Customer,
			Profile:             taskBAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.dropdownNumeriCriterionID: {
			Customer:            taskBAnswerKeyScorecard.Customer,
			Profile:             taskBAnswerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBAnswerKeyScorecard.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}
	taskBResponseScorecardA := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        taskB.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        taskB.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: taskB.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystSubmitterUserID2},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystSubmitterUserID2},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{taskB.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}
	taskBResponseScoresA := map[string]*dbmodel.Scores{
		s.sentenceCriterionID: {
			Customer:            taskBResponseScorecardA.Customer,
			Profile:             taskBResponseScorecardA.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBResponseScorecardA.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		s.numericRadioCriterionID: { // same as answer key
			Customer:            taskBResponseScorecardA.Customer,
			Profile:             taskBResponseScorecardA.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBResponseScorecardA.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		s.labelRadioCriterionID: { // diff from answer key
			Customer:            taskBResponseScorecardA.Customer,
			Profile:             taskBResponseScorecardA.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         taskBResponseScorecardA.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		// dropdown criterion is not present in the response scorecard
	}
	db := s.DB()
	setupData := func() {
		s.Require().NoError(db.Create(template).Error)
		s.Require().NoError(db.Create(taskA).Error)
		s.Require().NoError(db.Create(taskB).Error)

		s.Require().NoError(db.Create(taskAAnswerKeyScorecard).Error)
		s.Require().NoError(db.Create(fn.MapValueToSlice(taskAAnswerKeyScores)).Error)

		s.Require().NoError(db.Create(taskAResponseScorecardA).Error)
		s.Require().NoError(db.Create(fn.MapValueToSlice(taskAResponseScoresA)).Error)

		s.Require().NoError(db.Create(taskAResponseScorecardB).Error)
		s.Require().NoError(db.Create(fn.MapValueToSlice(taskAResponseScoresB)).Error)

		s.Require().NoError(db.Create(taskBAnswerKeyScorecard).Error)
		s.Require().NoError(db.Create(fn.MapValueToSlice(taskBAnswerKeyScores)).Error)
		s.Require().NoError(db.Create(taskBResponseScorecardA).Error)
		s.Require().NoError(db.Create(fn.MapValueToSlice(taskBResponseScoresA)).Error)
	}

	verify := func(tasks map[string]*GroupCalibrationTask) (
		taskSet set.Set[string],
		scorecardSet set.Set[string],
		sentenceSet set.Set[string],
		numericRadioSet set.Set[string],
		labelRadioSet set.Set[string],
		dropdownNumericSet set.Set[string],
	) {
		taskSet = set.NewFromSlice([]string{})
		scorecardSet = set.NewFromSlice([]string{})
		sentenceSet = set.NewFromSlice([]string{})
		numericRadioSet = set.NewFromSlice([]string{})
		labelRadioSet = set.NewFromSlice([]string{})
		dropdownNumericSet = set.NewFromSlice([]string{})

		for taskID, taskInfo := range tasks {
			taskSet.Add(taskID)
			scorecards := taskInfo.ResponseScorecards
			if scorecards == nil {
				continue
			}
			for _, scorecard := range scorecards {
				scorecardSet.Add(scorecard.ResponseScorecardID)
				for _, score := range scorecard.ResponseScores {
					switch score.CriterionIdentifier {
					case s.sentenceCriterionID:
						sentenceSet.Add(scorecard.ResponseScorecardID)
					case s.numericRadioCriterionID:
						numericRadioSet.Add(scorecard.ResponseScorecardID)
					case s.labelRadioCriterionID:
						labelRadioSet.Add(scorecard.ResponseScorecardID)
					case s.dropdownNumeriCriterionID:
						dropdownNumericSet.Add(scorecard.ResponseScorecardID)
					}
				}
			}
		}
		return taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet
	}

	qaAnalystUserID := uuid.NewString()
	authedUser := &dbmodel.Users{
		CustomerID: sql.NullString{Valid: true, String: s.customerID},
		UserID:     sql.NullString{Valid: true, String: qaAnalystUserID},
		UserRoles:  pq.Int32Array{int32(authpb.AuthProto_SUPER_ADMIN)},
	}
	ctx, _ := testingauth.GetIncomeCxtWithAuth(authedUser, "")
	mockUserServiceClient := usermock.NewUserServiceClient(s.T())

	s.Run("tasks with no response scorecard", func() {
		// Given
		setupData()
		taskC := *taskA
		taskC.TaskID = "taskC"
		taskCAnswerKeyScorecard := *taskAAnswerKeyScorecard
		taskCAnswerKeyScorecard.ResourceID = uuid.NewString()
		taskCAnswerKeyScorecard.TaskIds = []string{taskC.TaskID}
		taskCAnswerKeyScores := map[string]*dbmodel.Scores{}
		for criterionID, score := range taskAAnswerKeyScores {
			newScore := *score
			newScore.ResourceID = s.scoreResourcePrefix + uuid.NewString()
			newScore.ScorecardID = taskCAnswerKeyScorecard.ResourceID
			taskCAnswerKeyScores[criterionID] = &newScore
		}
		s.Require().NoError(db.Create(&taskC).Error)
		s.Require().NoError(db.Create(&taskCAnswerKeyScorecard).Error)
		s.Require().NoError(db.Create(fn.MapValueToSlice(taskCAnswerKeyScores)).Error)
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseIDs: []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 3)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(3, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("No filter", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseIDs: []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 2)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(2, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter task IDs", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				DirectorTaskIDs: []string{taskA.TaskID},
				CustomerID:      s.customerID,
				ProfileID:       s.profileID,
				UsecaseIDs:      []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 1)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(1, taskSet.Len())
		s.Require().Equal(2, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(2, numericRadioSet.Len())
		s.Assert().Equal(1, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter template IDs", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				ScorecardTemplateIDs: []string{template.ResourceID},
				CustomerID:           s.customerID,
				ProfileID:            s.profileID,
				UsecaseIDs:           []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 2)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(2, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter qa analyst user IDs", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				QAAnalystIDs: []string{qaAnalystSubmitterUserID},
				CustomerID:   s.customerID,
				ProfileID:    s.profileID,
				UsecaseIDs:   []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Len(taskToResponseScorecards, 1)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Assert().Equal(1, taskSet.Len())
		s.Assert().Equal(2, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(2, numericRadioSet.Len())
		s.Assert().Equal(1, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter by time range: overlap task's first half of schedule", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				TaskOverlapTimeRange: &TimeRange{
					Start: taskA.CreatedAt.AddDate(0, 0, -1),
					End:   taskA.TaskScheduleConfig.Message.GroupCalibrationScheduleConfig.DueDate.AsTime().AddDate(0, 0, -1),
				},
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseIDs: []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 2)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(2, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter by time range: overlap task's second half of schedule", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				TaskOverlapTimeRange: &TimeRange{
					Start: taskA.CreatedAt.AddDate(0, 0, 2),
					End:   taskA.TaskScheduleConfig.Message.GroupCalibrationScheduleConfig.DueDate.AsTime().AddDate(0, 0, 2),
				},
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseIDs: []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 2)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(2, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter by time range: inside task's schedule", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				TaskOverlapTimeRange: &TimeRange{
					Start: taskA.CreatedAt.AddDate(0, 0, 1),
					End:   taskA.TaskScheduleConfig.Message.GroupCalibrationScheduleConfig.DueDate.AsTime().AddDate(0, 0, -1),
				},
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseIDs: []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 2)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(2, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})

	s.Run("filter by time range: include task's schedule", func() {
		setupData()
		// When
		taskToResponseScorecards, err := retrieveGroupCalibrationTasks(
			ctx,
			db,
			&shared.CustomerProfile{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
			},
			mockUserServiceClient,
			analytics.Frequency_WEEKLY,
			&analytics.Metadata{
				TimeZoneId: "America/New_York",
			},
			&GroupCalibrationStatsConditions{
				TaskOverlapTimeRange: &TimeRange{
					Start: taskA.CreatedAt.AddDate(0, 0, -1),
					End:   taskA.TaskScheduleConfig.Message.GroupCalibrationScheduleConfig.DueDate.AsTime().AddDate(0, 0, 1),
				},
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseIDs: []string{s.usecaseID},
			},
		)
		// Then
		s.Require().NoError(err)
		s.Require().Len(taskToResponseScorecards, 2)
		taskSet, scorecardSet, sentenceSet, numericRadioSet, labelRadioSet, dropdownNumericSet := verify(taskToResponseScorecards)
		s.Require().Equal(2, taskSet.Len())
		s.Require().Equal(3, scorecardSet.Len())
		// each scorecard has 4 answer key scores
		s.Assert().Equal(0, sentenceSet.Len())
		s.Assert().Equal(3, numericRadioSet.Len())
		s.Assert().Equal(2, labelRadioSet.Len())
		s.Assert().Equal(1, dropdownNumericSet.Len())
	})
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestAggregateGroupCalibrationStatsNoGroupBy() {
	// Setup test data
	scorecardAID := uuid.NewString()
	scorecardBID := uuid.NewString()
	scorecardCID := uuid.NewString()
	scoreA := 50.0
	scoreB := 100.0
	scoreC := 70.0
	userAID := "userA" + uuid.NewString()
	userBID := "userB" + uuid.NewString()
	userCID := "userC" + uuid.NewString()
	tasks := map[string]*GroupCalibrationTask{
		s.taskAID: {
			TaskID:                    s.taskAID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userBID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userAID, userBID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardAID,
				ConsistencyScore:    scoreA,
				QAAnalystID:         userAID,
			}, {
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardBID,
				ConsistencyScore:    scoreB,
				QAAnalystID:         userBID,
			}},
		},
		s.taskBID: {
			TaskID:                    s.taskBID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userBID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userCID}),
			DueDate:                   time.Now().AddDate(0, 0, -1),
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskBID,
				ResponseScorecardID: scorecardCID,
				ConsistencyScore:    scoreC,
				QAAnalystID:         userCID,
			}},
		},
	}

	profileName := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}

	// Call the function with GroupCalibrationstatsGroupByNone
	groupStats, totalStats := aggregateGroupCalibrationStats(
		tasks,
		GroupCalibrationstatsGroupByNone,
		map[string]*userpb.User{}, // Empty map as specified
		profileName,
		false, // exclude ongoing tasks from consistency score
	)

	// Verify results
	s.Require().Len(groupStats, 1, "Expected 1 group stat for GroupByNone")
	s.Assert().Equal(int32(2), groupStats[0].DirectorTaskCount, "Expected 3 director tasks")
	assigned, evaluated, pending := s.sumOfGroupScorecardCount(groupStats...)
	s.Assert().Equal(assigned, totalStats.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(evaluated, totalStats.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(pending, totalStats.GroupCalibrationTaskStats.PendingScorecardCount)

	// Check consistency score calculation: 2 equal out of 3 total = 66.67%
	s.Require().InDelta(
		float32(scoreA+scoreB+scoreC)/3,
		groupStats[0].GroupCalibrationTaskStats.AvgConsistencyScore,
		0.01,
		"Expected average consistency score of 66.67%",
	)

	// Verify scorecard count
	s.Require().Equal(int32(3), groupStats[0].GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 2 evaluated scorecards")

	// Verify the scorecards in the result
	s.Require().Len(groupStats[0].GroupCalibrationTaskStats.EvaluatedScorecards, 3, "Expected 2 scorecards")

	// Check scorecard consistency scores
	foundScorecard1 := false
	foundScorecard2 := false
	foundScorecard3 := false
	for _, scorecard := range groupStats[0].GroupCalibrationTaskStats.EvaluatedScorecards {
		scorecardName1 := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: scorecardAID}.String()
		scorecardName2 := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: scorecardBID}.String()
		scorecardName3 := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: scorecardCID}.String()
		switch scorecard.ScorecardName {
		case scorecardName1:
			s.Require().Equal(float32(50.0), scorecard.ConsistencyScore, "Expected scorecard1 consistency score to be 50.0")
			foundScorecard1 = true
		case scorecardName2:
			s.Require().Equal(float32(100.0), scorecard.ConsistencyScore, "Expected scorecard2 consistency score to be 100.0")
			foundScorecard2 = true
		case scorecardName3:
			s.Require().Equal(float32(70.0), scorecard.ConsistencyScore, "Expected scorecard3 consistency score to be 70.0")
			foundScorecard3 = true
		}
	}
	s.Require().True(foundScorecard1, "Expected to find scorecard1")
	s.Require().True(foundScorecard2, "Expected to find scorecard2")
	s.Require().True(foundScorecard3, "Expected to find scorecard3")
	// Also verify total stats
	s.Require().Equal(int32(2), totalStats.DirectorTaskCount, "Expected 2 director tasks")

	// Check consistency score calculation: 2 equal out of 3 total = 66.67%
	s.Require().InDelta(
		float32(scoreA+scoreB+scoreC)/3,
		totalStats.GroupCalibrationTaskStats.AvgConsistencyScore,
		0.01,
		"Expected average consistency score of 66.67%",
	)

	// Verify scorecard count
	s.Require().Equal(int32(3), totalStats.GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 2 evaluated scorecards")

	// Verify the scorecards in the result
	s.Require().Len(totalStats.GroupCalibrationTaskStats.EvaluatedScorecards, 3, "Expected 2 scorecards")
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestAggregateGroupCalibrationStatsGroupByCriterion() {
	// Setup test data
	scorecardAID := uuid.NewString()
	scorecardBID := uuid.NewString()
	scorecardCID := uuid.NewString()
	scoreA := 50.0
	scoreB := 100.0
	scoreC := 70.0
	userAID := "userA" + uuid.NewString()
	userBID := "userB" + uuid.NewString()
	userCID := "userC" + uuid.NewString()
	template, _ := scoring.ParseScorecardTemplateStructure(s.templateSturctJSON)

	// Create criterion keys
	numericRadioCriterionKey := sharedqa.CriterionKey{CriterionID: s.numericRadioCriterionID}
	labelRadioCriterionKey := sharedqa.CriterionKey{CriterionID: s.labelRadioCriterionID}
	dropdownNumericCriterionKey := sharedqa.CriterionKey{CriterionID: s.dropdownNumeriCriterionID}

	tasks := map[string]*GroupCalibrationTask{
		s.taskAID: {
			TaskID:                    s.taskAID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userBID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userAID, userBID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			Template:                  template,
			AnswerKeyScores: map[sharedqa.CriterionKey]*GroupCalibrationScore{
				numericRadioCriterionKey: {
					CriterionIdentifier:  s.numericRadioCriterionID,
					NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
					CriterionDisplayName: s.numericRadioCriterionDisplayName,
				},
				dropdownNumericCriterionKey: {
					CriterionIdentifier:  s.dropdownNumeriCriterionID,
					NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
					CriterionDisplayName: s.dropdownNumericCriterionDisplayName,
				},
			},
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardAID,
				ConsistencyScore:    scoreA,
				QAAnalystID:         userAID,
				ResponseScores: map[sharedqa.CriterionKey]*GroupCalibrationScore{
					numericRadioCriterionKey: {
						CriterionIdentifier:  s.numericRadioCriterionID,
						NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
						CriterionDisplayName: s.numericRadioCriterionDisplayName,
					},
					labelRadioCriterionKey: {
						CriterionIdentifier:  s.labelRadioCriterionID,
						NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
						CriterionDisplayName: s.labelRadioCriterionDisplayName,
					},
					dropdownNumericCriterionKey: {
						CriterionIdentifier:  s.dropdownNumeriCriterionID,
						NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue3 + 1, Valid: true},
						CriterionDisplayName: s.dropdownNumericCriterionDisplayName,
					},
				},
			}, {
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardBID,
				ConsistencyScore:    scoreB,
				QAAnalystID:         userBID,
				ResponseScores: map[sharedqa.CriterionKey]*GroupCalibrationScore{
					numericRadioCriterionKey: {
						CriterionIdentifier:  s.numericRadioCriterionID,
						NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
						CriterionDisplayName: s.numericRadioCriterionDisplayName,
					},
					labelRadioCriterionKey: {
						CriterionIdentifier:  s.labelRadioCriterionID,
						NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
						CriterionDisplayName: s.labelRadioCriterionDisplayName,
					},
				},
			}},
		},
		s.taskBID: {
			TaskID:                    s.taskBID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userCID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			Template:                  template,
			AnswerKeyScores: map[sharedqa.CriterionKey]*GroupCalibrationScore{
				numericRadioCriterionKey: {
					CriterionIdentifier:  s.numericRadioCriterionID,
					NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
					CriterionDisplayName: s.numericRadioCriterionDisplayName,
				},
				dropdownNumericCriterionKey: {
					CriterionIdentifier:  s.dropdownNumeriCriterionID,
					NumericValue:         sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
					CriterionDisplayName: s.dropdownNumericCriterionDisplayName,
				},
			},
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskBID,
				ResponseScorecardID: scorecardCID,
				ConsistencyScore:    scoreC,
				QAAnalystID:         userCID,
				ResponseScores: map[sharedqa.CriterionKey]*GroupCalibrationScore{
					numericRadioCriterionKey: {
						CriterionIdentifier: s.numericRadioCriterionID,
						NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
					},
				},
			}},
		},
	}

	profileName := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}

	// Call the function with GroupCalibrationStatsGroupByNone
	groupStats, totalStats := aggregateGroupCalibrationCriteriaStats(
		tasks,
		map[string]*userpb.User{
			// Just for mocking
			userAID: {Name: userAID},
			userBID: {Name: userBID},
			userCID: {Name: userCID},
		}, // Empty map as specified
		profileName,
	)

	// Verify results
	s.Require().Len(groupStats, 3, "Expected 3 group stat for GroupByCriterion")
	var numericGroup, labelGroup, dropdownGroup *analytics.DirectorTaskStats
	for _, group := range groupStats {
		criterionID := group.Attribute.CriterionIdentifiers[0]
		switch criterionID {
		case s.numericRadioCriterionID:
			numericGroup = group
		case s.labelRadioCriterionID:
			labelGroup = group
		case s.dropdownNumeriCriterionID:
			dropdownGroup = group
		}
	}
	s.Assert().Equal(int32(0), numericGroup.DirectorTaskCount, "this field is ignored for GroupByCriterion")
	s.Assert().Equal(int32(0), labelGroup.DirectorTaskCount, "this field is ignored for GroupByCriterion")
	s.Assert().Equal(int32(0), dropdownGroup.DirectorTaskCount, "this field is ignored for GroupByCriterion")

	// Verify numeric radio criterion stats
	s.Len(numericGroup.GroupCalibrationTaskStats.EvaluatedCriteria, 3, "Should have 3 numeric criteria evaluations")

	// Verify labeled radio criterion stats
	s.Len(labelGroup.GroupCalibrationTaskStats.EvaluatedCriteria, 3, "Should have 3 label criteria evaluations")

	// Verify dropdown numeric criterion stats
	s.Len(dropdownGroup.GroupCalibrationTaskStats.EvaluatedCriteria, 3, "Should have 3 dropdown criteria evaluations")

	// Verify average consistency scores
	// These values depend on the comparison algorithm in calculateGroupCaliScorecardConsistencyScores
	// For this mock test, we simply verify the averages are calculated properly
	s.Assert().InDelta(numericGroup.GroupCalibrationTaskStats.AvgConsistencyScore, float32(66.66), 0.01,
		"Numeric criterion has two same value and one different value")
	s.Assert().InDelta(labelGroup.GroupCalibrationTaskStats.AvgConsistencyScore, float32(33.33), 0.01,
		"Label criterion has one same empty and two non-empty")
	s.Assert().InDelta(dropdownGroup.GroupCalibrationTaskStats.AvgConsistencyScore, float32(0), 0.01,
		"Dropdown criterion has one different value and two empty")

	// Also verify total stats
	s.Assert().InDelta(totalStats.GroupCalibrationTaskStats.AvgConsistencyScore, float32(33.33), 0.01,
		"Total stats should have non-zero consistency score")
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestAggregateGroupCalibrationStatsGroupByQAAnalyst() {
	// Setup test data
	scorecardAID := uuid.NewString()
	scorecardBID := uuid.NewString()
	scorecardCID := uuid.NewString()
	scoreA := 50.0
	scoreB := 100.0
	scoreC := 70.0
	userAID := "userA" + uuid.NewString()
	userBID := "userB" + uuid.NewString()
	userCID := "userC" + uuid.NewString()
	tasks := map[string]*GroupCalibrationTask{
		s.taskAID: {
			TaskID:                    s.taskAID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userBID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userAID, userBID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardAID,
				ConsistencyScore:    scoreA,
				QAAnalystID:         userAID,
			}, {
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardBID,
				ConsistencyScore:    scoreB,
				QAAnalystID:         userBID,
			}},
		},
		s.taskBID: {
			TaskID:                    s.taskBID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userCID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskBID,
				ResponseScorecardID: scorecardCID,
				ConsistencyScore:    scoreC,
				QAAnalystID:         userCID,
			}},
		},
	}

	profileName := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}

	// Call the function with GroupCalibrationstatsGroupByNone
	groupStats, totalStats := aggregateGroupCalibrationStats(
		tasks,
		GroupCalibrationstatsGroupByQAAnalyst,
		map[string]*userpb.User{
			// Just for mocking
			userAID: {Name: userAID},
			userBID: {Name: userBID},
			userCID: {Name: userCID},
		}, // Empty map as specified
		profileName,
		false,
	)

	// Verify results
	s.Require().Len(groupStats, 3, "Expected 3 group stat for GroupByNone")
	var userAGroup, userBGroup, userCGroup *analytics.DirectorTaskStats
	for _, group := range groupStats {
		userID := group.Attribute.Users[0].Name
		switch userID {
		case userAID:
			userAGroup = group
		case userBID:
			userBGroup = group
		case userCID:
			userCGroup = group
		}
	}
	s.Assert().Equal(int32(2), userAGroup.DirectorTaskCount, "Expected 2 director tasks")
	s.Assert().Equal(int32(1), userBGroup.DirectorTaskCount, "Expected 2 director tasks")
	s.Assert().Equal(int32(2), userCGroup.DirectorTaskCount, "Expected 2 director tasks")

	s.Assert().Equal(int32(2), userAGroup.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(int32(1), userAGroup.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(int32(1), userAGroup.GroupCalibrationTaskStats.PendingScorecardCount)
	s.Assert().InDelta(scoreA, userAGroup.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01)

	s.Assert().Equal(int32(1), userBGroup.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(int32(1), userBGroup.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(int32(0), userBGroup.GroupCalibrationTaskStats.PendingScorecardCount)
	s.Assert().InDelta(scoreB, userBGroup.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01)

	s.Assert().Equal(int32(2), userCGroup.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(int32(1), userCGroup.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(int32(1), userCGroup.GroupCalibrationTaskStats.PendingScorecardCount)
	s.Assert().InDelta(scoreC, userCGroup.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01)

	assigned, evaluated, pending := s.sumOfGroupScorecardCount(userAGroup, userBGroup, userCGroup)
	s.Assert().Equal(assigned, totalStats.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(evaluated, totalStats.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(pending, totalStats.GroupCalibrationTaskStats.PendingScorecardCount)

	// Also verify total stats
	s.Assert().Equal(int32(2), totalStats.DirectorTaskCount, "Expected 2 director tasks")

	// Check consistency score calculation: 2 equal out of 3 total = 66.67%
	s.Assert().InDelta(
		float32(scoreA+scoreB+scoreC)/3,
		totalStats.GroupCalibrationTaskStats.AvgConsistencyScore,
		0.01,
		"Expected average consistency score of 66.67%",
	)
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestAggregateGroupCalibrationStatsGroupByTask() {
	// Setup test data
	scorecardAID := uuid.NewString()
	scorecardBID := uuid.NewString()
	scorecardCID := uuid.NewString()
	scoreA := 50.0
	scoreB := 100.0
	scoreC := 70.0
	userAID := "userA" + uuid.NewString()
	userBID := "userB" + uuid.NewString()
	userCID := "userC" + uuid.NewString()
	tasks := map[string]*GroupCalibrationTask{
		s.taskAID: {
			TaskID:                    s.taskAID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userBID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userAID, userBID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardAID,
				ConsistencyScore:    scoreA,
				QAAnalystID:         userAID,
			}, {
				TaskID:              s.taskAID,
				ResponseScorecardID: scorecardBID,
				ConsistencyScore:    scoreB,
				QAAnalystID:         userBID,
			}},
		},
		s.taskBID: {
			TaskID:                    s.taskBID,
			AssigneeUserIDSet:         set.NewFromSlice([]string{userAID, userCID}),
			CompleteAssigneeUserIDSet: set.NewFromSlice([]string{userCID}),
			DueDate:                   time.Now().AddDate(0, 0, 1),
			ResponseScorecards: []*GroupCalibrationScorecard{{
				TaskID:              s.taskBID,
				ResponseScorecardID: scorecardCID,
				ConsistencyScore:    scoreC,
				QAAnalystID:         userCID,
			}},
		},
	}

	profileName := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}

	// Call the function with GroupCalibrationstatsGroupByNone
	groupStats, totalStats := aggregateGroupCalibrationStats(
		tasks,
		GroupCalibrationstatsGroupByDirectorTask,
		map[string]*userpb.User{
			// Just for mocking
			userAID: {Name: userAID},
			userBID: {Name: userBID},
			userCID: {Name: userCID},
		}, // Empty map as specified
		profileName,
		false,
	)

	// Verify results
	s.Require().Len(groupStats, 2, "Expected 3 group stat for GroupByNone")
	var taskAGroup, taskBGroup *analytics.DirectorTaskStats
	for _, group := range groupStats {
		taskName, err := coachingpb.ParseDirectorTaskName(group.Attribute.TaskNames[0])
		s.Require().NoError(err)
		switch taskName.DirectorTaskID {
		case s.taskAID:
			taskAGroup = group
		case s.taskBID:
			taskBGroup = group
		}
	}
	s.Assert().Equal(int32(1), taskAGroup.DirectorTaskCount, "Expected 2 director tasks")
	s.Assert().Equal(int32(1), taskBGroup.DirectorTaskCount, "Expected 2 director tasks")

	s.Assert().Equal(int32(3), taskAGroup.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(int32(2), taskAGroup.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(int32(1), taskAGroup.GroupCalibrationTaskStats.PendingScorecardCount)
	s.Assert().InDelta((scoreA+scoreB)/2, taskAGroup.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01)

	s.Assert().Equal(int32(2), taskBGroup.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(int32(1), taskBGroup.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(int32(1), taskBGroup.GroupCalibrationTaskStats.PendingScorecardCount)
	s.Assert().InDelta(scoreC, taskBGroup.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01)

	assigned, evaluated, pending := s.sumOfGroupScorecardCount(taskAGroup, taskBGroup)
	s.Assert().Equal(assigned, totalStats.GroupCalibrationTaskStats.AssignedScorecardCount)
	s.Assert().Equal(evaluated, totalStats.GroupCalibrationTaskStats.EvaluatedScorecardCount)
	s.Assert().Equal(pending, totalStats.GroupCalibrationTaskStats.PendingScorecardCount)

	// Also verify total stats
	s.Assert().Equal(int32(2), totalStats.DirectorTaskCount, "Expected 2 director tasks")

	// Check consistency score calculation: 2 equal out of 3 total = 66.67%
	s.Assert().InDelta(
		float32(scoreA+scoreB+scoreC)/3,
		totalStats.GroupCalibrationTaskStats.AvgConsistencyScore,
		0.01,
		"Expected average consistency score of 66.67%",
	)
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestRetrieveGroupCalibrationStatsNoGroupBy() {
	// Given
	template := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "01 The original current template",
		CreatedAt:  s.taskCreatedTime,
		UsecaseIds: []string{s.usecaseID},
		Template:   s.templateSturctJSON,
	}

	qaAdminUserID := uuid.NewString()
	qaAnalystUserID := uuid.NewString()
	task := &dbmodel.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{Valid: true, String: s.usecaseID},
		TaskID:     uuid.NewString(),
		CreatedAt:  s.taskCreatedTime,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION,
		TaskScheduleConfig: dbqapb.DirectorTaskScheduleConfigSQL{
			Message: &dbqapb.DirectorTaskScheduleConfig{
				GroupCalibrationScheduleConfig: &dbqapb.GroupCalibrationTaskScheduleConfig{
					DueDate: timestamppb.New(s.taskDueDate),
				},
			},
		},
		TaskAudienceConfig: dbqapb.DirectorTaskAudienceConfigSQL{
			Message: &dbqapb.DirectorTaskAudienceConfig{
				UserIds: []string{qaAnalystUserID},
			},
		},
		TaskContentConfig: dbqapb.DirectorTaskContentConfigSQL{
			Message: &dbqapb.DirectorTaskContentConfig{
				GroupCalibrationContentConfig: &dbqapb.GroupCalibrationTaskContentConfig{
					Tasks: []*dbqapb.GroupCalibrationTaskContentConfig_GroupCalibrationTask{
						{ScorecardTemplateId: template.ResourceID, ScorecardTemplateRevisionId: template.Revision},
					},
				},
			},
		},
	}

	agentUserID := uuid.NewString()
	answerKeyScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAdminUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAdminUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY), Valid: true},
	}

	// Create two response scorecards to test aggregation
	responseScorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	responseScorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	answerKeyScores := []*dbmodel.Scores{
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// Create two matching sets of response scores - one for each scorecard
	responseScores1 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// For the second scorecard, make one criterion different to test inconsistency
	responseScores2 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			// This value is different from answer key
			NumericValue:  sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable: sql.NullBool{Bool: false, Valid: false},
		},
	}

	db := s.DB()
	s.Require().NoError(db.Create(template).Error)
	s.Require().NoError(db.Create(task).Error)
	s.Require().NoError(db.Create(answerKeyScorecard).Error)
	s.Require().NoError(db.Create(responseScorecard1).Error)
	s.Require().NoError(db.Create(responseScorecard2).Error)
	s.Require().NoError(db.Create(answerKeyScores).Error)
	s.Require().NoError(db.Create(responseScores1).Error)
	s.Require().NoError(db.Create(responseScores2).Error)

	// Create a mock user service client
	mockUserServiceClient := usermock.NewUserServiceClient(s.T())
	mockUserServiceClient.EXPECT().ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard1.SubmitterUserID.String}.String()},
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard2.SubmitterUserID.String}.String()},
			},
		}, nil).Once()

	// Create a mock context with authentication
	authedUser := &dbmodel.Users{
		CustomerID: sql.NullString{Valid: true, String: s.customerID},
		UserID:     sql.NullString{Valid: true, String: qaAnalystUserID},
		UserRoles:  pq.Int32Array{int32(authpb.AuthProto_SUPER_ADMIN)},
	}
	ctx, _ := testingauth.GetIncomeCxtWithAuth(authedUser, "")

	// Create request with GroupCalibrationstatsGroupByNone (no groupby attributes)
	req := &analytics.RetrieveDirectorTaskStatsRequest{
		Metadata: &analytics.Metadata{
			TimeZoneId: "America/New_York",
		},
		FilterByAttribute: &analytics.Attribute{
			UsecaseNames: []string{customerpb.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  s.usecaseID,
			}.String()},
		},
	}

	// When
	resp, err := retrieveGroupCalibrationStats(ctx, db, &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}, mockUserServiceClient, req, s.authEntityProvider)

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify the response has the expected group stats
	s.Require().Len(resp.DirectorTaskStats, 1, "Expected one group stat for GroupByNone")
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.DirectorTaskCount, "Expected 1 director task")

	// Verify that we now have 2 scorecards in the response
	s.Require().Equal(int32(2), resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 2 evaluated scorecards")
	s.Require().Len(resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards, 2, "Expected 2 scorecards in results")

	// First scorecard should have 100% consistency
	// Second scorecard should have less than 100% consistency (due to the one different criterion)
	// Calculate the expected average
	scorecard1Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard1.ResourceID}.String()
	scorecard2Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard2.ResourceID}.String()

	// Find scorecard1 and scorecard2 in the results
	var scorecard1Found, scorecard2Found bool
	var scorecard1Score, scorecard2Score float32

	for _, scorecard := range resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards {
		if scorecard.ScorecardName == scorecard1Name {
			scorecard1Found = true
			scorecard1Score = scorecard.ConsistencyScore
			s.Require().InDelta(float32(100.0), scorecard.ConsistencyScore, 0.01, "Expected first scorecard to have 100% consistency")
		} else if scorecard.ScorecardName == scorecard2Name {
			scorecard2Found = true
			scorecard2Score = scorecard.ConsistencyScore
			// Expect less than 100% since one criterion is different
			s.Require().InDelta(float32(33.333), scorecard.ConsistencyScore, 0.01, "Expected second scorecard to have less than 100% consistency")
		}
	}

	s.Require().True(scorecard1Found, "First scorecard should be in the results")
	s.Require().True(scorecard2Found, "Second scorecard should be in the results")

	// Verify the average consistency score is the average of both scorecards
	expectedAvgConsistency := (scorecard1Score + scorecard2Score) / 2
	s.Require().InDelta(expectedAvgConsistency, resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01,
		"Expected average consistency score to be average of both scorecards")
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestRetrieveGroupCalibrationStatsExcludeOngoingTasks() {
	// Given
	template := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "01 The original current template",
		CreatedAt:  s.taskCreatedTime,
		UsecaseIds: []string{s.usecaseID},
		Template:   s.templateSturctJSON,
	}

	qaAdminUserID := uuid.NewString()
	qaAnalystUserID := uuid.NewString()
	qaAnalystUserName := userpb.UserName{CustomerID: s.customerID, UserID: qaAnalystUserID}
	task := &dbmodel.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{Valid: true, String: s.usecaseID},
		TaskID:     uuid.NewString(),
		CreatedAt:  s.taskCreatedTime,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION,
		TaskScheduleConfig: dbqapb.DirectorTaskScheduleConfigSQL{
			Message: &dbqapb.DirectorTaskScheduleConfig{
				GroupCalibrationScheduleConfig: &dbqapb.GroupCalibrationTaskScheduleConfig{
					DueDate: timestamppb.New(time.Now().AddDate(0, 0, 1)), // set the due date to be 1 day from now (ongoing task)
				},
			},
		},
		TaskAudienceConfig: dbqapb.DirectorTaskAudienceConfigSQL{
			Message: &dbqapb.DirectorTaskAudienceConfig{
				UserIds: []string{qaAnalystUserID},
			},
		},
		TaskContentConfig: dbqapb.DirectorTaskContentConfigSQL{
			Message: &dbqapb.DirectorTaskContentConfig{
				GroupCalibrationContentConfig: &dbqapb.GroupCalibrationTaskContentConfig{
					Tasks: []*dbqapb.GroupCalibrationTaskContentConfig_GroupCalibrationTask{
						{ScorecardTemplateId: template.ResourceID, ScorecardTemplateRevisionId: template.Revision},
					},
				},
			},
		},
	}

	agentUserID := uuid.NewString()
	answerKeyScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAdminUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAdminUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY), Valid: true},
	}

	// Create two response scorecards to test aggregation
	responseScorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	responseScorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	answerKeyScores := []*dbmodel.Scores{
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// Create two matching sets of response scores - one for each scorecard
	responseScores1 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// For the second scorecard, make one criterion different to test inconsistency
	responseScores2 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			// This value is different from answer key
			NumericValue:  sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable: sql.NullBool{Bool: false, Valid: false},
		},
	}

	db := s.DB()
	s.Require().NoError(db.Create(template).Error)
	s.Require().NoError(db.Create(task).Error)
	s.Require().NoError(db.Create(answerKeyScorecard).Error)
	s.Require().NoError(db.Create(responseScorecard1).Error)
	s.Require().NoError(db.Create(responseScorecard2).Error)
	s.Require().NoError(db.Create(answerKeyScores).Error)
	s.Require().NoError(db.Create(responseScores1).Error)
	s.Require().NoError(db.Create(responseScores2).Error)

	// Create a mock user service client
	mockUserServiceClient := usermock.NewUserServiceClient(s.T())
	mockUserServiceClient.EXPECT().ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard1.SubmitterUserID.String}.String()},
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard2.SubmitterUserID.String}.String()},
			},
		}, nil).Once()

	// Create a mock context with authentication
	authedUser := &dbmodel.Users{
		CustomerID: sql.NullString{Valid: true, String: s.customerID},
		UserID:     sql.NullString{Valid: true, String: qaAnalystUserID},
		UserRoles:  pq.Int32Array{int32(authpb.AuthProto_SUPER_ADMIN)},
	}
	ctx, _ := testingauth.GetIncomeCxtWithAuth(authedUser, "")

	// Create request with GroupCalibrationstatsGroupByNone (no groupby attributes)
	req := &analytics.RetrieveDirectorTaskStatsRequest{
		Metadata: &analytics.Metadata{
			TimeZoneId: "America/New_York",
		},
		FilterByAttribute: &analytics.Attribute{
			UsecaseNames: []string{customerpb.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  s.usecaseID,
			}.String()},
		},
		Assignees: []string{qaAnalystUserName.String()},
	}

	// When
	resp, err := retrieveGroupCalibrationStats(ctx, db, &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}, mockUserServiceClient, req, s.authEntityProvider)

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify the response has the expected group stats
	s.Require().Len(resp.DirectorTaskStats, 1, "Expected one group stat for GroupByNone")
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.DirectorTaskCount, "Expected 1 director task")

	// Verify that we now have 2 scorecards in the response
	s.Require().Equal(int32(2), resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 2 evaluated scorecards")
	s.Require().Len(resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards, 2, "Expected 2 scorecards in results")

	// First scorecard should have 100% consistency
	// Second scorecard should have less than 100% consistency (due to the one different criterion)
	// Calculate the expected average
	scorecard1Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard1.ResourceID}.String()
	scorecard2Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard2.ResourceID}.String()

	// Find scorecard1 and scorecard2 in the results
	var scorecard1Found, scorecard2Found bool

	for _, scorecard := range resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards {
		if scorecard.ScorecardName == scorecard1Name {
			scorecard1Found = true
			s.Assert().InDelta(float32(100.0), scorecard.ConsistencyScore, 0.01, "Expected first scorecard to have 100% consistency")
		} else if scorecard.ScorecardName == scorecard2Name {
			scorecard2Found = true
			// Expect less than 100% since one criterion is different
			s.Assert().InDelta(float32(33.333), scorecard.ConsistencyScore, 0.01, "Expected second scorecard to have less than 100% consistency")
		}
	}

	s.Assert().True(scorecard1Found, "First scorecard should be in the results")
	s.Assert().True(scorecard2Found, "Second scorecard should be in the results")

	expectedAvgConsistency := ConsistencyScoreNotAvailable
	s.Assert().InDelta(expectedAvgConsistency, resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01,
		"Expected average consistency score to be average of both scorecards")
}

func (s *RetrieveGroupCalibrationStatsTestSuite) sumOfGroupScorecardCount(groupStats ...*analytics.DirectorTaskStats) (assigned, evaluated, pending int32) {
	assigned = int32(0)
	evaluated = int32(0)
	pending = int32(0)

	for _, groupStat := range groupStats {
		assigned += groupStat.GroupCalibrationTaskStats.AssignedScorecardCount
		evaluated += groupStat.GroupCalibrationTaskStats.EvaluatedScorecardCount
		pending += groupStat.GroupCalibrationTaskStats.PendingScorecardCount
	}
	return assigned, evaluated, pending
}

func (s *RetrieveGroupCalibrationStatsTestSuite) TestReturnNegativeConsistencyScoreWhenNoValidScorecards() {
	// Given
	template := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "01 The original current template",
		CreatedAt:  s.taskCreatedTime,
		UsecaseIds: []string{s.usecaseID},
		Template:   s.templateSturctJSON,
	}

	qaAdminUserID := uuid.NewString()
	qaAnalystUserID := uuid.NewString()
	qaAnalystUserName := userpb.UserName{CustomerID: s.customerID, UserID: qaAnalystUserID}
	task := &dbmodel.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{Valid: true, String: s.usecaseID},
		TaskID:     uuid.NewString(),
		CreatedAt:  time.Now().AddDate(0, 0, -7),
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION,
		TaskScheduleConfig: dbqapb.DirectorTaskScheduleConfigSQL{
			Message: &dbqapb.DirectorTaskScheduleConfig{
				GroupCalibrationScheduleConfig: &dbqapb.GroupCalibrationTaskScheduleConfig{
					DueDate: timestamppb.New(time.Now().AddDate(0, 0, 1)),
				},
			},
		},
		TaskAudienceConfig: dbqapb.DirectorTaskAudienceConfigSQL{
			Message: &dbqapb.DirectorTaskAudienceConfig{
				UserIds: []string{qaAnalystUserID},
			},
		},
		TaskContentConfig: dbqapb.DirectorTaskContentConfigSQL{
			Message: &dbqapb.DirectorTaskContentConfig{
				GroupCalibrationContentConfig: &dbqapb.GroupCalibrationTaskContentConfig{
					Tasks: []*dbqapb.GroupCalibrationTaskContentConfig_GroupCalibrationTask{
						{ScorecardTemplateId: template.ResourceID, ScorecardTemplateRevisionId: template.Revision},
					},
				},
			},
		},
	}

	agentUserID := uuid.NewString()
	answerKeyScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAdminUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAdminUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY), Valid: true},
	}

	// Create two response scorecards to test aggregation
	responseScorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAnalystUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAnalystUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	answerKeyScores := []*dbmodel.Scores{
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// Create two matching sets of response scores - one for each scorecard
	responseScores1 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	db := s.DB()
	s.Require().NoError(db.Create(template).Error)
	s.Require().NoError(db.Create(task).Error)
	s.Require().NoError(db.Create(answerKeyScorecard).Error)
	s.Require().NoError(db.Create(responseScorecard1).Error)
	s.Require().NoError(db.Create(answerKeyScores).Error)
	s.Require().NoError(db.Create(responseScores1).Error)

	// Create a mock user service client
	mockUserServiceClient := usermock.NewUserServiceClient(s.T())
	mockUserServiceClient.EXPECT().ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard1.SubmitterUserID.String}.String()},
			},
		}, nil).Once()

	// Create a mock context with authentication
	authedUser := &dbmodel.Users{
		CustomerID: sql.NullString{Valid: true, String: s.customerID},
		UserID:     sql.NullString{Valid: true, String: qaAnalystUserID},
		UserRoles:  pq.Int32Array{int32(authpb.AuthProto_SUPER_ADMIN)},
	}
	ctx, _ := testingauth.GetIncomeCxtWithAuth(authedUser, "")

	// Create request with GroupCalibrationstatsGroupByNone (no groupby attributes)
	req := &analytics.RetrieveDirectorTaskStatsRequest{
		Metadata: &analytics.Metadata{
			TimeZoneId: "America/New_York",
		},
		FilterByAttribute: &analytics.Attribute{
			UsecaseNames: []string{customerpb.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  s.usecaseID,
			}.String()},
		},
		Assignees: []string{qaAnalystUserName.String()},
	}

	// When
	resp, err := retrieveGroupCalibrationStats(ctx, db, &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}, mockUserServiceClient, req, s.authEntityProvider)

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify the response has the expected group stats
	s.Require().Len(resp.DirectorTaskStats, 1, "Expected one group stat for GroupByNone")
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.DirectorTaskCount, "Expected 1 director task")

	// Verify that we now have 2 scorecards in the response
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 1 evaluated scorecard")
	s.Require().Len(resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards, 1, "Expected 1 scorecard in results")

	// First scorecard should have 100% consistency
	// Second scorecard should have less than 100% consistency (due to the one different criterion)
	// Calculate the expected average
	scorecard1Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard1.ResourceID}.String()

	// Find scorecard1 and scorecard2 in the results
	var scorecard1Found bool

	for _, scorecard := range resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards {
		if scorecard.ScorecardName == scorecard1Name {
			scorecard1Found = true
			s.Require().InDelta(float32(100.0), scorecard.ConsistencyScore, 0.01, "Expected first scorecard to have 100% consistency")
		}
	}

	s.Require().True(scorecard1Found, "First scorecard should be in the results")

	// Verify the average consistency score is the average of both scorecards
	expectedAvgConsistency := ConsistencyScoreNotAvailable
	s.Require().InDelta(expectedAvgConsistency, resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01,
		"Expected average consistency score to be average of both scorecards")
}

// TestTaskAudienceIsUpdated tests that when the task audience is updated, the scorecards are excluded from the results
func (s *RetrieveGroupCalibrationStatsTestSuite) TestTaskAudienceIsUpdated() {
	// Given
	template := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "01 The original current template",
		CreatedAt:  s.taskCreatedTime,
		UsecaseIds: []string{s.usecaseID},
		Template:   s.templateSturctJSON,
	}

	qaAdminUserID := uuid.NewString()
	submitterUserID := uuid.NewString()
	submitterUserID2 := uuid.NewString()
	task := &dbmodel.Tasks{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
		UsecaseID:  sql.NullString{Valid: true, String: s.usecaseID},
		TaskID:     uuid.NewString(),
		CreatedAt:  s.taskCreatedTime,
		TaskStatus: coachingpb.DirectorTaskStatus_DIRECTOR_TASK_STATUS_ACTIVE,
		TaskType:   coachingpb.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION,
		TaskScheduleConfig: dbqapb.DirectorTaskScheduleConfigSQL{
			Message: &dbqapb.DirectorTaskScheduleConfig{
				GroupCalibrationScheduleConfig: &dbqapb.GroupCalibrationTaskScheduleConfig{
					DueDate: timestamppb.New(s.taskDueDate),
				},
			},
		},
		TaskAudienceConfig: dbqapb.DirectorTaskAudienceConfigSQL{
			Message: &dbqapb.DirectorTaskAudienceConfig{
				UserIds: []string{submitterUserID, submitterUserID2},
			},
		},
		TaskContentConfig: dbqapb.DirectorTaskContentConfigSQL{
			Message: &dbqapb.DirectorTaskContentConfig{
				GroupCalibrationContentConfig: &dbqapb.GroupCalibrationTaskContentConfig{
					Tasks: []*dbqapb.GroupCalibrationTaskContentConfig_GroupCalibrationTask{
						{ScorecardTemplateId: template.ResourceID, ScorecardTemplateRevisionId: template.Revision},
					},
				},
			},
		},
	}

	agentUserID := uuid.NewString()
	answerKeyScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: qaAdminUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: qaAdminUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_ANSWER_KEY), Valid: true},
	}

	// Create two response scorecards to test aggregation
	responseScorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: submitterUserID},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: submitterUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	// Create a second response scorecard with a different submitter
	responseScorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		UpdatedAt:        task.CreatedAt.AddDate(0, 0, 1),
		SubmittedAt:      sql.NullTime{Valid: true, Time: task.CreatedAt.AddDate(0, 0, 1)},
		SubmitterUserID:  sql.NullString{Valid: true, String: submitterUserID2},
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: submitterUserID2},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      agentUserID,
		ConversationID:   uuid.NewString(),
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		TaskIds:          []string{task.TaskID},
		ScorecardType:    sql.NullInt32{Int32: int32(coachingpb.ScorecardType_SCORECARD_TYPE_GROUP_CALIBRATION_RESPONSE), Valid: true},
	}

	answerKeyScores := []*dbmodel.Scores{
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            answerKeyScorecard.Customer,
			Profile:             answerKeyScorecard.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         answerKeyScorecard.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// Create two matching sets of response scores - one for each scorecard
	responseScores1 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue2, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard1.Customer,
			Profile:             responseScorecard1.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard1.ResourceID,
			CriterionIdentifier: s.dropdownNumeriCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue3, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	// For the second scorecard, make one criterion different to test inconsistency
	responseScores2 := []*dbmodel.Scores{
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.sentenceCriterionID,
		},
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.labelRadioCriterionID,
			NumericValue:        sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
		{
			Customer:            responseScorecard2.Customer,
			Profile:             responseScorecard2.Profile,
			ResourceID:          s.scoreResourcePrefix + uuid.NewString(),
			ScorecardID:         responseScorecard2.ResourceID,
			CriterionIdentifier: s.numericRadioCriterionID,
			// This value is different from answer key
			NumericValue:  sql.NullFloat64{Float64: s.criterionNumericValue1, Valid: true},
			NotApplicable: sql.NullBool{Bool: false, Valid: false},
		},
	}

	db := s.DB()
	s.Require().NoError(db.Create(template).Error)
	s.Require().NoError(db.Create(task).Error)
	s.Require().NoError(db.Create(answerKeyScorecard).Error)
	s.Require().NoError(db.Create(responseScorecard1).Error)
	s.Require().NoError(db.Create(responseScorecard2).Error)
	s.Require().NoError(db.Create(answerKeyScores).Error)
	s.Require().NoError(db.Create(responseScores1).Error)
	s.Require().NoError(db.Create(responseScores2).Error)

	// Create a mock user service client
	mockUserServiceClient := usermock.NewUserServiceClient(s.T())
	mockUserServiceClient.EXPECT().ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard1.SubmitterUserID.String}.String()},
				{Name: userpb.UserName{CustomerID: s.customerID, UserID: responseScorecard2.SubmitterUserID.String}.String()},
			},
		}, nil).Twice()

	// Create a mock context with authentication
	authedUser := &dbmodel.Users{
		CustomerID: sql.NullString{Valid: true, String: s.customerID},
		UserID:     sql.NullString{Valid: true, String: submitterUserID},
		UserRoles:  pq.Int32Array{int32(authpb.AuthProto_SUPER_ADMIN)},
	}
	ctx, _ := testingauth.GetIncomeCxtWithAuth(authedUser, "")

	// Create request with GroupCalibrationstatsGroupByNone (no groupby attributes)
	req := &analytics.RetrieveDirectorTaskStatsRequest{
		Metadata: &analytics.Metadata{
			TimeZoneId: "America/New_York",
		},
		FilterByAttribute: &analytics.Attribute{
			UsecaseNames: []string{customerpb.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  s.usecaseID,
			}.String()},
		},
	}

	// When
	resp, err := retrieveGroupCalibrationStats(ctx, db, &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}, mockUserServiceClient, req, s.authEntityProvider)

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify the response has the expected group stats
	s.Require().Len(resp.DirectorTaskStats, 1, "Expected one group stat for GroupByNone")
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.DirectorTaskCount, "Expected 1 director task")

	// Verify that we now have 2 scorecards in the response
	s.Require().Equal(int32(2), resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 2 evaluated scorecards")
	s.Require().Len(resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards, 2, "Expected 2 scorecards in results")

	// First scorecard should have 100% consistency
	scorecard1Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard1.ResourceID}.String()
	scorecard2Name := coachingpb.ScorecardName{CustomerID: s.customerID, ProfileID: s.profileID, ScorecardID: responseScorecard2.ResourceID}.String()

	// Find scorecard1 and scorecard2 in the results
	var scorecard1Found, scorecard2Found bool

	for _, scorecard := range resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards {
		if scorecard.ScorecardName == scorecard1Name {
			scorecard1Found = true
			s.Require().InDelta(float32(100.0), scorecard.ConsistencyScore, 0.01, "Expected first scorecard to have 100% consistency")
		} else if scorecard.ScorecardName == scorecard2Name {
			scorecard2Found = true
		}
	}

	s.Require().True(scorecard1Found, "First scorecard should be in the results")
	s.Require().True(scorecard2Found, "Second scorecard should not be in the results")

	// Update the task audience to only include submitter 1
	task.TaskAudienceConfig.Message.UserIds = []string{submitterUserID}
	s.Require().NoError(db.Save(task).Error)

	resp, err = retrieveGroupCalibrationStats(ctx, db, &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}, mockUserServiceClient, req, s.authEntityProvider)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify the response has the expected group stats
	s.Require().Len(resp.DirectorTaskStats, 1, "Expected one group stat for GroupByNone")
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.DirectorTaskCount, "Expected 1 director task")

	// Verify that we now have 1 scorecards in the response: submitter 2 should be excluded
	s.Require().Equal(int32(1), resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecardCount, "Expected 1 evaluated scorecards")
	s.Require().Len(resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards, 1, "Expected 1 scorecards in results")

	var scorecard1Score float32
	scorecard1Found = false
	scorecard2Found = false

	for _, scorecard := range resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.EvaluatedScorecards {
		if scorecard.ScorecardName == scorecard1Name {
			scorecard1Found = true
			scorecard1Score = scorecard.ConsistencyScore
			s.Require().InDelta(float32(100.0), scorecard.ConsistencyScore, 0.01, "Expected first scorecard to have 100% consistency")
		} else if scorecard.ScorecardName == scorecard2Name {
			scorecard2Found = true
		}
	}

	s.Require().True(scorecard1Found, "First scorecard should be in the results")
	s.Require().False(scorecard2Found, "Second scorecard should not be in the results")

	// Verify the average consistency score is the average of both scorecards
	expectedAvgConsistency := scorecard1Score
	s.Require().InDelta(expectedAvgConsistency, resp.TotalDirectorTaskStats.GroupCalibrationTaskStats.AvgConsistencyScore, 0.01,
		"Expected average consistency score to be average of both scorecards")
}
