package analyticsimpl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	authmock "github.com/cresta/go-servers/shared/auth/mocks"
	"github.com/cresta/go-servers/shared/scoring"
	userfilter "github.com/cresta/go-servers/shared/user-filter"
	userfiltermock "github.com/cresta/go-servers/shared/user-filter/mocks"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"

	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
)

func TestRetrieveAppealStats(t *testing.T) {
	suite.Run(t, &RetrieveAppealStatsTestSuite{})
}

type RetrieveAppealStatsTestSuite struct {
	AnalyticsTestSuite
	AgentUserID       string
	QAAnalystUserID   string
	ManagerUserID     string
	Profile           *shared.CustomerProfile
	TemplateA         *dbmodel.ScorecardTemplates
	ScorecardA        *dbmodel.Scorecards
	Scores            []*dbmodel.Scores
	MultiSelectScores []*dbmodel.Scores

	userServiceClient *usermock.UserServiceClient
	aclProvider       *authmock.MockResourceACLHelperProvider

	statsComparer        cmp.Option
	userComparer         cmp.Option
	qaAttributesComparer cmp.Option
}

func (s *RetrieveAppealStatsTestSuite) CleanDB() {
	db := s.DB()
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS director CASCADE").Error)
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS historic CASCADE").Error)
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS app CASCADE").Error)

	s.LoadAppSchema(s.T())
	s.LoadDirectorSchema(s.T())
}

func (s *RetrieveAppealStatsTestSuite) InitializeFixtures() {
	s.customerID = "test_customer"
	s.profileID = "test_profile"
	s.usecaseID = "test_usecase"
	s.AgentUserID = "test_agent_user_id"
	s.QAAnalystUserID = "test_qa_analyst_user_id"
	s.ManagerUserID = "test_manager_id"
	s.Profile = &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.TemplateA = &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "01 The original current template",
		CreatedAt:  time.Date(2021, 11, 24, 12, 0, 0, 0, time.UTC),
		UsecaseIds: []string{s.usecaseID},
		Template: `{
			"version": 2,
			"items": [
			{
				"displayName": "Sentence",
				"type": "sentence",
				"weight": 1,
				"identifier": "criterion1"
			},
			{
				"displayName": "Numeric Radios",
				"type": "numeric-radios",
				"weight": 1,
				"settings": {"range": {"max": 5, "min": 1}},
				"identifier": "criterion2"
			},
			{
				"displayName": "Multi Select",
				"type": "dropdown-numeric-values",
				"weight": 1,
				"identifier": "multi-select",
				"settings": {
					"enableMultiSelect":true, 
					"showNA": true,
					"options":[
    			{
    			   "label":"Yes",
    			   "value":0
    			},
    			{
    			   "label":"No",
    			   "value":1
    			},
    			{
    			   "label":"Yesser",
    			   "value":2
    			},
    			{
    			   "label":"The Yessest",
    			   "value":3
    			}], 
					"scores":[
    			{
    			   "score":1,
    			   "value":0
    			},
    			{
    			   "score":0,
    			   "value":1
    			},
    			{
    			   "score":2,
    			   "value":2
    			},
    			{
    			   "score":2,
    			   "value":3
    			}]
				}
			}]
		}`,
	}

	s.ScorecardA = &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2021, 9, 23, 12, 2, 2, 0, time.UTC),
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Date(2021, 9, 23, 12, 12, 2, 0, time.UTC)},
		SubmitterUserID:  sql.NullString{Valid: true, String: s.QAAnalystUserID},
		TemplateID:       s.TemplateA.ResourceID,
		TemplateRevision: s.TemplateA.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: s.ManagerUserID},
		Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
		AgentUserID:      s.AgentUserID,
		ConversationID:   uuid.NewString(),
		Comment:          sql.NullString{Valid: true, String: "Great conversation!"},
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
	}

	s.Scores = []*dbmodel.Scores{
		{
			Customer:            s.ScorecardA.Customer,
			Profile:             s.ScorecardA.Profile,
			ResourceID:          "score_1_1",
			ScorecardID:         s.ScorecardA.ResourceID,
			CriterionIdentifier: "criterion1",
			Comment:             sql.NullString{Valid: true, String: "Text Value"},
			CommentAccessRoles:  []int32{int32(authpb.AuthProto_MANAGER)},
			TextValue:           sql.NullString{Valid: true, String: "good"},
		},
		{
			Customer:            s.ScorecardA.Customer,
			Profile:             s.ScorecardA.Profile,
			ResourceID:          "score_1_2",
			ScorecardID:         s.ScorecardA.ResourceID,
			CriterionIdentifier: "criterion2",
			Comment:             sql.NullString{Valid: true, String: "Number Value"},
			CommentAccessRoles:  []int32{},
			NumericValue:        sql.NullFloat64{Float64: 10.0, Valid: true},
			NotApplicable:       sql.NullBool{Bool: false, Valid: false},
		},
	}

	s.MultiSelectScores = []*dbmodel.Scores{
		{
			Customer:            s.ScorecardA.Customer,
			Profile:             s.ScorecardA.Profile,
			ResourceID:          "score_1_3",
			ScorecardID:         s.ScorecardA.ResourceID,
			CriterionIdentifier: "multi-select",
			NumericValue:        sql.NullFloat64{Float64: 2.0, Valid: true},
		},
		{
			Customer:            s.ScorecardA.Customer,
			Profile:             s.ScorecardA.Profile,
			ResourceID:          "score_1_4",
			ScorecardID:         s.ScorecardA.ResourceID,
			CriterionIdentifier: "multi-select",
			NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
		},
	}
}

func (s *RetrieveAppealStatsTestSuite) InitializeServer() {
	s.server = &AnalyticsServiceImpl{appsDB: s.DBTestSuite.AppsDBRegistry(), appEnv: "test", logger: log.GetLogger()}
	s.userServiceClient = usermock.NewUserServiceClient(s.T())
	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](s.userServiceClient)

	s.aclProvider = authmock.NewMockResourceACLHelperProvider(gomock.NewController(s.T()))
	s.aclProvider.EXPECT().Get().AnyTimes()
	s.server.resourceACLHelperProvider = s.aclProvider
}

func (s *RetrieveAppealStatsTestSuite) SetupSuite() {
	s.SetupTestDatabase(s.T())

	s.statsComparer = cmp.Comparer(func(a, b map[CriterionConvKey]*AppealInfo) bool {
		return cmp.Equal(a, b,
			cmpopts.SortMaps(func(a, b CriterionConvKey) bool {
				aKey := fmt.Sprintf("%s-%s-%s", a.CriterionID, a.ConversationID, a.ScorecardTemplateID)
				bKey := fmt.Sprintf("%s-%s-%s", b.CriterionID, b.ConversationID, b.ScorecardTemplateID)
				return aKey < bKey
			}),
			cmp.Comparer(func(a, b *AppealInfo) bool {
				return a.IsAppealed == b.IsAppealed && a.IsResolved == b.IsResolved && a.IsAdjusted == b.IsAdjusted
			}),
			cmpopts.IgnoreUnexported(AppealInfo{}),
		)
	})

	s.userComparer = cmp.Comparer(func(a, b *userpb.User) bool {
		return (a == nil && b == nil) || (a.Name == b.Name)
	})
	timeStampComparer := cmp.Comparer(func(a, b *timestamppb.Timestamp) bool {
		return cmp.Equal(a, b,
			cmpopts.IgnoreUnexported(timestamppb.Timestamp{}),
		)
	})

	qaAttributeComparer := cmp.Comparer(func(a, b *analytics.QAAttribute) bool {
		equalUsers := cmp.Equal(a.Users, b.Users, cmpopts.EquateEmpty(), s.userComparer)
		equalFrequency := cmp.Equal(a.TimeRange, b.TimeRange,
			cmpopts.IgnoreUnexported(analytics.TimeRange{}),
			timeStampComparer,
		)
		return equalUsers && equalFrequency
	})
	qaAttributeToString := func(a *analytics.QAAttribute) string {
		key := ""
		for _, u := range a.Users {
			key += u.Name
		}
		if a.TimeRange != nil {
			if a.TimeRange.StartTimestamp != nil {
				key += a.TimeRange.StartTimestamp.String()
			}
			if a.TimeRange.EndTimestamp != nil {
				key += a.TimeRange.EndTimestamp.String()
			}
		}
		for _, c := range a.CriterionIdentifiers {
			key += c
		}
		return key
	}
	s.qaAttributesComparer = cmp.Comparer(func(a, b []*analytics.QAAttribute) bool {
		return cmp.Equal(a, b,
			cmpopts.EquateEmpty(),
			cmpopts.SortSlices(func(i, j *analytics.QAAttribute) bool {
				return qaAttributeToString(i) < qaAttributeToString(j)
			}),
			qaAttributeComparer,
		)
	})
}

func (s *RetrieveAppealStatsTestSuite) SetupTest() {
	s.CleanDB()
	s.InitializeFixtures()
	s.InitializeServer()
}

func (s *RetrieveAppealStatsTestSuite) TearDownTest() {
	s.userServiceClient.AssertExpectations(s.T())
}

func (s *RetrieveAppealStatsTestSuite) SetupSubTest() {
	s.CleanDB()
	s.InitializeFixtures()
	s.InitializeServer()
}

func (s *RetrieveAppealStatsTestSuite) TearDownSubTest() {
	s.userServiceClient.AssertExpectations(s.T())
}

func (s *RetrieveAppealStatsTestSuite) TestInvalidRequest() {
	baseReq := &analytics.RetrieveAppealStatsRequest{
		Parent: customer.ProfileName{
			CustomerID: s.customerID,
			ProfileID:  s.profileID,
		}.String(),
		Frequency: analytics.Frequency_WEEKLY,
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{customer.UsecaseName{
				CustomerID: s.customerID,
				ProfileID:  s.profileID,
				UsecaseID:  s.usecaseID,
			}.String()},
			ScorecardTemplates: []string{
				coaching.ScorecardTemplateName{
					CustomerID:                  s.customerID,
					ProfileID:                   s.profileID,
					ScorecardTemplateID:         s.TemplateA.ResourceID,
					ScorecardTemplateRevisionID: s.TemplateA.Revision,
				}.String(),
			},
		},
		GroupByAttributes: []analytics.QAAttributeType{
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
		},
	}
	expectedTimeRangeError := "a time range with start and end time is required"
	s.Run("time range is missing", func() {
		req := (proto.Clone(baseReq)).(*analytics.RetrieveAppealStatsRequest)
		req.FilterByTimeRange = nil
		_, err := s.server.RetrieveAppealStats(context.TODO(), req)
		s.Assert().ErrorContains(err, expectedTimeRangeError)
		s.Require().Error(err)
	})
	s.Run("time range start is missing", func() {
		req := (proto.Clone(baseReq)).(*analytics.RetrieveAppealStatsRequest)
		req.FilterByTimeRange.StartTimestamp = nil
		_, err := s.server.RetrieveAppealStats(context.TODO(), req)
		s.Assert().ErrorContains(err, expectedTimeRangeError)
		s.Require().Error(err)
	})
	s.Run("time range end is missing", func() {
		req := (proto.Clone(baseReq)).(*analytics.RetrieveAppealStatsRequest)
		req.FilterByTimeRange.EndTimestamp = nil
		_, err := s.server.RetrieveAppealStats(context.TODO(), req)
		s.Assert().ErrorContains(err, expectedTimeRangeError)
		s.Require().Error(err)
	})

	s.Run("use case is missing", func() {
		req := (proto.Clone(baseReq)).(*analytics.RetrieveAppealStatsRequest)
		req.FilterByAttribute.UsecaseNames = nil
		expectedErrMsg := "there should be one usecase name"
		_, err := s.server.RetrieveAppealStats(context.TODO(), req)
		s.Assert().ErrorContains(err, expectedErrMsg)
		s.Require().Error(err)
	})
	s.Run("scorecard template is missing", func() {
		req := (proto.Clone(baseReq)).(*analytics.RetrieveAppealStatsRequest)
		templateB := &dbmodel.ScorecardTemplates{
			Customer:   s.customerID,
			Profile:    s.profileID,
			ResourceID: uuid.NewString(),
			CreatedAt:  time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
			Revision:   uuid.NewString(),
		}
		req.FilterByAttribute.ScorecardTemplates = []string{
			coaching.ScorecardTemplateName{
				CustomerID:                  s.customerID,
				ProfileID:                   s.profileID,
				ScorecardTemplateID:         s.TemplateA.ResourceID,
				ScorecardTemplateRevisionID: s.TemplateA.Revision,
			}.String(),
			coaching.ScorecardTemplateName{
				CustomerID:                  s.customerID,
				ProfileID:                   s.profileID,
				ScorecardTemplateID:         templateB.ResourceID,
				ScorecardTemplateRevisionID: templateB.Revision,
			}.String(),
		}
		expectedErrMsg := "there should be one or none scorecard template name"
		_, err := s.server.RetrieveAppealStats(context.TODO(), req)
		s.Assert().ErrorContains(err, expectedErrMsg)
		s.Require().Error(err)
	})
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealScorecards() {
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, _, _, _, err = s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)

	conditions := RetrieveAppealStatsConditions{
		ScorecardTemplateID: s.TemplateA.ResourceID,
		AppealRequestSubmitAtTimeRange: TimeRange{
			Start: s.ScorecardA.SubmittedAt.Time.Add(-time.Hour),
			End:   s.ScorecardA.SubmittedAt.Time.Add(time.Hour),
		},
		FilteredUsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				userpb.UserName{
					CustomerID: s.customerID,
					UserID:     s.ScorecardA.AgentUserID,
				}.String(),
			},
		},
	}
	frequency := analytics.Frequency_WEEKLY
	appeals, err := retrieveAppealScorecards(db, s.Profile, &frequency, &analytics.Metadata{}, &conditions)
	s.Require().NoError(err)
	s.Len(appeals, 1)
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealWorkflows() {
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, appealScorecard, _, _, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)

	appealWorkflows, err := retrieveAppealWorkflows(db, s.Profile, map[string]*ScorecardWithFrequency{
		appealScorecard.ResourceID: {Scorecards: *appealScorecard, Frequency: sql.NullTime{}},
	})
	s.Require().NoError(err)
	s.Len(appealWorkflows, 1)
	s.Assert().Equal(appealWorkflows[0].AppealRequest.ResourceID, appealScorecard.ResourceID)
}

// CONVI-7485: an appeal can be filed against an unsubmitted auto-scored original, so an unsubmitted
// original or replica must not drop the workflow from appeal reporting. Only the appeal request
// itself has to be submitted.
func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealWorkflowsWithUnsubmittedOriginal() {
	db := s.DB()
	s.Require().NoError(db.Create(s.TemplateA).Error)
	s.Require().NoError(db.Create(s.ScorecardA).Error)
	s.Require().NoError(db.Create(s.Scores).Error)
	replicaScorecard, appealScorecard, _, _, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, false)
	s.Require().NoError(err)

	// The original and its replica are unsubmitted; the appeal request is not.
	s.Require().NoError(db.Model(&dbmodel.Scorecards{}).
		Where("resource_id IN ?", []string{s.ScorecardA.ResourceID, replicaScorecard.ResourceID}).
		Update("submitted_at", nil).Error)

	appealWorkflows, err := retrieveAppealWorkflows(db, s.Profile, map[string]*ScorecardWithFrequency{
		appealScorecard.ResourceID: {Scorecards: *appealScorecard, Frequency: sql.NullTime{}},
	})
	s.Require().NoError(err)
	s.Require().Len(appealWorkflows, 1, "an unsubmitted original and replica should not drop the workflow")
	s.Assert().Equal(appealScorecard.ResourceID, appealWorkflows[0].AppealRequest.ResourceID)

	// An unsubmitted appeal request is still excluded.
	s.Require().NoError(db.Model(&dbmodel.Scorecards{}).
		Where("resource_id = ?", appealScorecard.ResourceID).
		Update("submitted_at", nil).Error)

	appealWorkflows, err = retrieveAppealWorkflows(db, s.Profile, map[string]*ScorecardWithFrequency{
		appealScorecard.ResourceID: {Scorecards: *appealScorecard, Frequency: sql.NullTime{}},
	})
	s.Require().NoError(err)
	s.Assert().Empty(appealWorkflows, "an unsubmitted appeal request should be excluded")
}

func (s *RetrieveAppealStatsTestSuite) TestGroupedStats() {
	teamplateA := "templateA"
	convA := "convA"
	agentA := "agentA"
	agentAName := userpb.UserName{CustomerID: s.customerID, UserID: agentA}
	agentAUser := &userpb.User{Name: agentAName.String()}
	agentB := "agentB"
	agentBName := userpb.UserName{CustomerID: s.customerID, UserID: agentB}
	agentBUser := &userpb.User{Name: agentBName.String()}
	qaAnalystA := "qaAnalystA"
	qaAnalystAName := userpb.UserName{CustomerID: s.customerID, UserID: qaAnalystA}
	qaAnalystAUser := &userpb.User{Name: qaAnalystAName.String()}
	qaAnalystB := "qaAnalystB"
	qaAnalystBName := userpb.UserName{CustomerID: s.customerID, UserID: qaAnalystB}
	qaAnalystBUser := &userpb.User{Name: qaAnalystBName.String()}
	frequencyA := fn.Must(time.Parse(time.DateOnly, "2025-01-13"))
	frequencyB := fn.Must(time.Parse(time.DateOnly, "2025-01-06"))
	convCreateTime := fn.Must(time.Parse(time.DateOnly, "2025-01-14"))
	criterionKeyA := func() CriterionConvKey {
		return CriterionConvKey{
			ScorecardTemplateID: teamplateA,
			CriterionID:         "criterionA",
			ConversationID:      convA,
		}
	}
	criterionKeyB := func() CriterionConvKey {
		return CriterionConvKey{
			ScorecardTemplateID: teamplateA,
			CriterionID:         "criterionB",
			ConversationID:      convA,
		}
	}
	criterionAppealInfos := map[CriterionConvKey]*AppealInfo{
		criterionKeyA(): {
			IsAppealed:     true,
			IsResolved:     true,
			IsAdjusted:     true,
			CriterionID:    "criterionA",
			ConversationID: convA,
			AgentID:        agentA,
			QAAnalystID:    qaAnalystA,
			ConvCreateTime: convCreateTime,
			Frequency:      sql.NullTime{Valid: true, Time: frequencyA},
		},
		criterionKeyB(): {
			IsAppealed:     false,
			IsResolved:     false,
			IsAdjusted:     false,
			CriterionID:    "criterionB",
			ConversationID: convA,
			AgentID:        agentA,
			QAAnalystID:    qaAnalystA,
			ConvCreateTime: convCreateTime,
			Frequency:      sql.NullTime{Valid: true, Time: frequencyA},
		},
	}

	userIDToUser := map[string]*userpb.User{
		agentA:     agentAUser,
		agentB:     agentBUser,
		qaAnalystA: qaAnalystAUser,
		qaAnalystB: qaAnalystBUser,
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}

	s.Run("group by None", func() {
		// When
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByNone,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		s.Assert().Empty(groups)
	})

	s.Run("group by agent", func() {
		criterionAppealInfos[criterionKeyB()].AgentID = agentB

		// When
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByAgent,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		s.Len(groups, 2)
		expectedAttributes := []*analytics.QAAttribute{
			{
				Users: []*userpb.User{agentAUser},
			},
			{
				Users: []*userpb.User{agentBUser},
			},
		}
		s.Assert().Empty(
			cmp.Diff(
				expectedAttributes,
				[]*analytics.QAAttribute{groups[0].Attribute, groups[1].Attribute},
				s.qaAttributesComparer,
			),
		)
	})

	s.Run("group by qa analyst", func() {
		criterionAppealInfos[criterionKeyA()].QAAnalystID = qaAnalystA
		criterionAppealInfos[criterionKeyB()].QAAnalystID = qaAnalystB

		// When
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByQAAnlyst,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		s.Len(groups, 2)
		expectedAttributes := []*analytics.QAAttribute{
			{
				Users: []*userpb.User{qaAnalystAUser},
			},
			{
				Users: []*userpb.User{qaAnalystBUser},
			},
		}
		s.Assert().Empty(
			cmp.Diff(
				expectedAttributes,
				[]*analytics.QAAttribute{groups[0].Attribute, groups[1].Attribute},
				s.qaAttributesComparer,
			),
		)
	})

	s.Run("group by agent and frequency", func() {
		criterionAppealInfos[criterionKeyA()].AgentID = agentA
		criterionAppealInfos[criterionKeyA()].Frequency = sql.NullTime{Valid: true, Time: frequencyA}
		criterionAppealInfos[criterionKeyB()].AgentID = agentA
		criterionAppealInfos[criterionKeyB()].Frequency = sql.NullTime{Valid: true, Time: frequencyB}

		// When
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByAgentAndFrequency,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		s.Len(groups, 2)
		expectedAttributes := []*analytics.QAAttribute{
			{
				Users: []*userpb.User{agentAUser},
				TimeRange: &analytics.TimeRange{
					StartTimestamp: timestamppb.New(frequencyA),
				},
			},
			{
				Users: []*userpb.User{agentAUser},
				TimeRange: &analytics.TimeRange{
					StartTimestamp: timestamppb.New(frequencyB),
				},
			},
		}
		s.Assert().Empty(
			cmp.Diff(
				expectedAttributes,
				[]*analytics.QAAttribute{groups[0].Attribute, groups[1].Attribute},
				s.qaAttributesComparer,
			),
		)
	})

	s.Run("group by qa analyst and frequency", func() {
		criterionAppealInfos[criterionKeyA()].QAAnalystID = qaAnalystA
		criterionAppealInfos[criterionKeyA()].Frequency = sql.NullTime{Valid: true, Time: frequencyA}
		criterionAppealInfos[criterionKeyB()].QAAnalystID = qaAnalystA
		criterionAppealInfos[criterionKeyB()].Frequency = sql.NullTime{Valid: true, Time: frequencyB}

		// When
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByQAAnlystAndFrequency,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		s.Len(groups, 2)
		expectedAttributes := []*analytics.QAAttribute{
			{
				Users: []*userpb.User{qaAnalystAUser},
				TimeRange: &analytics.TimeRange{
					StartTimestamp: timestamppb.New(frequencyA),
				},
			},
			{
				Users: []*userpb.User{qaAnalystAUser},
				TimeRange: &analytics.TimeRange{
					StartTimestamp: timestamppb.New(frequencyB),
				},
			},
		}
		s.Assert().Empty(
			cmp.Diff(
				expectedAttributes,
				[]*analytics.QAAttribute{groups[0].Attribute, groups[1].Attribute},
				s.qaAttributesComparer,
			),
		)
	})

	s.Run("group by frequency", func() {
		criterionAppealInfos[criterionKeyB()].Frequency = sql.NullTime{Valid: true, Time: frequencyB}

		// When
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByFrequency,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		s.Len(groups, 2)
		expectedAttributes := []*analytics.QAAttribute{
			{
				TimeRange: &analytics.TimeRange{
					StartTimestamp: timestamppb.New(frequencyA),
				},
			},
			{
				TimeRange: &analytics.TimeRange{
					StartTimestamp: timestamppb.New(frequencyB),
				},
			},
		}
		s.Assert().Empty(
			cmp.Diff(
				expectedAttributes,
				[]*analytics.QAAttribute{groups[0].Attribute, groups[1].Attribute},
				s.qaAttributesComparer,
			),
		)
	})

	s.Run("group by agent and criterion", func() {
		criterionAppealInfos[criterionKeyA()].AgentID = agentA
		criterionAppealInfos[criterionKeyB()].AgentID = agentA
		groups, total := aggregateGroupAppealStats(
			criterionAppealInfos,
			RetrieveAppealStatsGroupByAgentAndCriterion,
			userIDToUser,
			profile,
		)
		s.Assert().NotEmpty(total)
		expectedAttributes := []*analytics.QAAttribute{
			{
				Users:                []*userpb.User{agentAUser},
				CriterionIdentifiers: []string{criterionKeyA().CriterionID},
			},
			{
				Users:                []*userpb.User{agentAUser},
				CriterionIdentifiers: []string{criterionKeyB().CriterionID},
			},
		}
		s.Assert().Empty(
			cmp.Diff(
				expectedAttributes,
				[]*analytics.QAAttribute{groups[0].Attribute, groups[1].Attribute},
				s.qaAttributesComparer,
			),
		)
	})
}

func (s *RetrieveAppealStatsTestSuite) TestAnalyzeCriterionAppealInfo() {
	textCriterionKey := func() CriterionConvKey {
		return CriterionConvKey{
			ScorecardTemplateID: s.TemplateA.ResourceID,
			CriterionID:         s.Scores[0].CriterionIdentifier,
			ConversationID:      s.ScorecardA.ConversationID,
			ScorecardID:         s.ScorecardA.ResourceID,
		}
	}
	numericCriterionKey := func() CriterionConvKey {
		return CriterionConvKey{
			ScorecardTemplateID: s.TemplateA.ResourceID,
			CriterionID:         s.Scores[1].CriterionIdentifier,
			ConversationID:      s.ScorecardA.ConversationID,
			ScorecardID:         s.ScorecardA.ResourceID,
		}
	}

	s.Run("not appealed, not resolved, not adjusted", func() {
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)
		err = db.Create(s.ScorecardA).Error
		s.Require().NoError(err)
		err = db.Create(s.Scores).Error
		s.Require().NoError(err)
		replica, appeal, _, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
		s.Require().NoError(err)

		appealWorkflow := scoring.AppealWorkflow{
			Original:        s.ScorecardA,
			OriginalReplica: replica,
			AppealRequest:   appeal,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appeal.ResourceID: {Scorecards: *appeal, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appeal.ConversationID: {
					CreatedAt: time.Time{},
				},
			},
			map[string]bool{
				textCriterionKey().CriterionID:    true,
				numericCriterionKey().CriterionID: true,
			},
		)

		// Verify
		expectedStats := map[CriterionConvKey]*AppealInfo{
			textCriterionKey(): {
				IsAppealed: false,
				IsResolved: false,
				IsAdjusted: false,
			},
			numericCriterionKey(): {
				IsAppealed: false,
				IsResolved: false,
				IsAdjusted: false,
			},
		}
		s.Assert().Truef(
			cmp.Equal(stats, expectedStats, s.statsComparer),
			"expected stats: %v\n, actual stats: %v\n",
			expectedStats, stats,
		)
	})

	s.Run("appealed, not resolve, not adjusted", func() {
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)
		err = db.Create(s.ScorecardA).Error
		s.Require().NoError(err)
		err = db.Create(s.Scores).Error
		s.Require().NoError(err)
		replica, appeal, resolve, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, false)
		s.Require().NoError(err)
		// set appeal scores
		appealScores := scorecardIDToScores[appeal.ResourceID]
		appealScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
		appealScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}

		appealWorkflow := scoring.AppealWorkflow{
			Original:        s.ScorecardA,
			OriginalReplica: replica,
			AppealRequest:   appeal,
			AppealResolve:   resolve,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appeal.ResourceID: {Scorecards: *appeal, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appeal.ConversationID: {
					CreatedAt: time.Time{},
				},
			},
			map[string]bool{
				textCriterionKey().CriterionID:    true,
				numericCriterionKey().CriterionID: true,
			},
		)

		// Verify
		expectedStats := map[CriterionConvKey]*AppealInfo{
			textCriterionKey(): {
				IsAppealed: true,
				IsResolved: false,
				IsAdjusted: false,
			},
			numericCriterionKey(): {
				IsAppealed: true,
				IsResolved: false,
				IsAdjusted: false,
			},
		}
		s.Assert().Truef(
			cmp.Equal(stats, expectedStats, s.statsComparer),
			"expected stats: %v\n, actual stats: %v\n",
			expectedStats, stats,
		)
	})

	s.Run("appealed, resolve, not adjusted", func() {
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)
		err = db.Create(s.ScorecardA).Error
		s.Require().NoError(err)
		err = db.Create(s.Scores).Error
		s.Require().NoError(err)
		replica, appeal, resolve, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
		s.Require().NoError(err)
		// set appeal scores
		appealScores := scorecardIDToScores[appeal.ResourceID]
		appealScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
		appealScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}

		appealWorkflow := scoring.AppealWorkflow{
			Original:        s.ScorecardA,
			OriginalReplica: replica,
			AppealRequest:   appeal,
			AppealResolve:   resolve,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appeal.ResourceID: {Scorecards: *appeal, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appeal.ConversationID: {
					CreatedAt: time.Time{},
				},
			},
			map[string]bool{
				textCriterionKey().CriterionID:    true,
				numericCriterionKey().CriterionID: true,
			},
		)

		// Verify
		expectedStats := map[CriterionConvKey]*AppealInfo{
			textCriterionKey(): {
				IsAppealed: true,
				IsResolved: true,
				IsAdjusted: false,
			},
			numericCriterionKey(): {
				IsAppealed: true,
				IsResolved: true,
				IsAdjusted: false,
			},
		}
		s.Assert().Truef(
			cmp.Equal(stats, expectedStats, s.statsComparer),
			"expected stats: %v\n, actual stats: %v\n",
			expectedStats, stats,
		)
	})

	s.Run("appealed, resolve, adjusted", func() {
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)
		err = db.Create(s.ScorecardA).Error
		s.Require().NoError(err)
		err = db.Create(s.Scores).Error
		s.Require().NoError(err)
		replica, appeal, resolve, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
		s.Require().NoError(err)
		// set appeal scores
		appealScores := scorecardIDToScores[appeal.ResourceID]
		appealScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
		appealScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}
		s.Require().NoError(db.Save(appealScores).Error)
		// set resolve scores
		resolveScores := scorecardIDToScores[resolve.ResourceID]
		resolveScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
		resolveScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}

		appealWorkflow := scoring.AppealWorkflow{
			Original:        s.ScorecardA,
			OriginalReplica: replica,
			AppealRequest:   appeal,
			AppealResolve:   resolve,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appeal.ResourceID: {Scorecards: *appeal, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appeal.ConversationID: {
					CreatedAt: time.Time{},
				},
			},
			map[string]bool{
				textCriterionKey().CriterionID:    true,
				numericCriterionKey().CriterionID: true,
			},
		)

		// Verify
		expectedStats := map[CriterionConvKey]*AppealInfo{
			textCriterionKey(): {
				IsAppealed: true,
				IsResolved: true,
				IsAdjusted: true,
			},
			numericCriterionKey(): {
				IsAppealed: true,
				IsResolved: true,
				IsAdjusted: true,
			},
		}
		s.Assert().Truef(
			cmp.Equal(stats, expectedStats, s.statsComparer),
			"expected stats: %v\n, actual stats: %v\n",
			expectedStats, stats,
		)
	})

	s.Run("template with branches - different criteria in original, appeal, and resolve", func() {
		db := s.DB()

		// Create a template with branches
		templateWithBranches := &dbmodel.ScorecardTemplates{
			Customer:   s.customerID,
			Profile:    s.profileID,
			ResourceID: uuid.NewString(),
			Revision:   "r1",
			Title:      "Template with Branches",
			CreatedAt:  time.Date(2021, 11, 24, 12, 0, 0, 0, time.UTC),
			UsecaseIds: []string{s.usecaseID},
			Template: `{
				"version": 2,
				"items": [
					{
						"displayName": "Main Question",
						"type": "labeled-radios",
						"weight": 2,
						"settings": {
							"options": [
								{"label": "Yes", "value": 1},
								{"label": "No", "value": 0}
							]
						},
						"identifier": "main-question",
						"branches": [
							{
								"identifier": "yes-branch",
								"condition": {
									"numeric_values": [1]
								},
								"children": [
									{
										"displayName": "Follow-up Question for Yes",
										"type": "numeric-radios",
										"weight": 1,
										"settings": {
											"range": {"max": 5, "min": 1}
										},
										"identifier": "follow-up-yes"
									},
									{
										"displayName": "Additional Details",
										"type": "sentence",
										"weight": 0,
										"identifier": "details-yes"
									}
								]
							},
							{
								"identifier": "no-branch",
								"condition": {
									"numeric_values": [0]
								},
								"children": [
									{
										"displayName": "Reason for No",
										"type": "labeled-radios",
										"weight": 1,
										"settings": {
											"options": [
												{"label": "Technical Issue", "value": 2},
												{"label": "Process Issue", "value": 1},
												{"label": "Other", "value": 0}
											]
										},
										"identifier": "reason-no"
									},
									{
										"displayName": "Follow-up Question for No",
										"type": "labeled-radios",
										"weight": 1,
										"settings": {
											"options": [
												{"label": "Yes", "value": 1},
												{"label": "No", "value": 0}
											]
										},
										"identifier": "follow-up-no",
										"branches": [
											{
												"identifier": "follow-up-no-yes-branch",
												"condition": {
													"numeric_values": [1]
												},
												"children": [
													{
														"displayName": "Nested Follow-up",
														"type": "numeric-radios",
														"weight": 1,
														"settings": {
															"range": {"max": 3, "min": 1}
														},
														"identifier": "nested-follow-up"
													}
												]
											}
										]
									}
								]
							}
						]
					},
					{
						"displayName": "Independent Question",
						"type": "numeric-radios",
						"weight": 1,
						"settings": {
							"range": {"max": 5, "min": 1}
						},
						"identifier": "independent-question"
					}
				]
			}`,
		}
		err := db.Create(templateWithBranches).Error
		s.Require().NoError(err)

		// Create scorecard with branches
		scorecardWithBranches := &dbmodel.Scorecards{
			Customer:         s.customerID,
			Profile:          s.profileID,
			ResourceID:       uuid.NewString(),
			CreatedAt:        time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2021, 9, 23, 12, 2, 2, 0, time.UTC),
			SubmittedAt:      sql.NullTime{Valid: true, Time: time.Date(2021, 9, 23, 12, 12, 2, 0, time.UTC)},
			SubmitterUserID:  sql.NullString{Valid: true, String: s.QAAnalystUserID},
			TemplateID:       templateWithBranches.ResourceID,
			TemplateRevision: templateWithBranches.Revision,
			CreatorUserID:    sql.NullString{Valid: true, String: s.ManagerUserID},
			Score:            sql.NullFloat64{Float64: 75.0, Valid: true},
			AgentUserID:      s.AgentUserID,
			ConversationID:   uuid.NewString(),
			Comment:          sql.NullString{Valid: true, String: "Branched conversation!"},
			UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		}
		err = db.Create(scorecardWithBranches).Error
		s.Require().NoError(err)

		// Original scores - main question = "Yes", so follow-up-yes and details-yes should be visible
		originalScores := []*dbmodel.Scores{
			{
				Customer:            scorecardWithBranches.Customer,
				Profile:             scorecardWithBranches.Profile,
				ResourceID:          "original_main_question",
				ScorecardID:         scorecardWithBranches.ResourceID,
				CriterionIdentifier: "main-question",
				TextValue:           sql.NullString{Valid: true, String: "Yes"},
				NumericValue:        sql.NullFloat64{Float64: 1.0, Valid: true},
			},
			{
				Customer:            scorecardWithBranches.Customer,
				Profile:             scorecardWithBranches.Profile,
				ResourceID:          "original_follow_up_yes",
				ScorecardID:         scorecardWithBranches.ResourceID,
				CriterionIdentifier: "follow-up-yes",
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
			{
				Customer:            scorecardWithBranches.Customer,
				Profile:             scorecardWithBranches.Profile,
				ResourceID:          "original_details_yes",
				ScorecardID:         scorecardWithBranches.ResourceID,
				CriterionIdentifier: "details-yes",
				TextValue:           sql.NullString{Valid: true, String: "Good service provided"},
			},
			{
				Customer:            scorecardWithBranches.Customer,
				Profile:             scorecardWithBranches.Profile,
				ResourceID:          "original_independent",
				ScorecardID:         scorecardWithBranches.ResourceID,
				CriterionIdentifier: "independent-question",
				NumericValue:        sql.NullFloat64{Float64: 4.0, Valid: true},
			},
		}
		err = db.Create(originalScores).Error
		s.Require().NoError(err)

		// Setup appeal workflow
		replica, appeal, resolve, scorecardIDToScores, err := s.SetupScorecard(db, templateWithBranches, scorecardWithBranches, originalScores, true, true)
		s.Require().NoError(err)

		// Appeal scores - main question changed to "No", so reason-no and follow-up-no should be visible
		// Also appeal the independent question
		appealScores := scorecardIDToScores[appeal.ResourceID]
		// Filter out scores for criteria that should not be visible due to branch change
		filteredAppealScores := []*dbmodel.Scores{}
		for _, score := range appealScores {
			switch score.CriterionIdentifier {
			case "main-question":
				// Change main question from "Yes" to "No"
				score.TextValue = sql.NullString{Valid: true, String: "No"}
				score.NumericValue = sql.NullFloat64{Float64: 0.0, Valid: true}
				filteredAppealScores = append(filteredAppealScores, score)
			case "follow-up-yes":
				// This criterion should not be in appeal since main-question is now "No"
				// Skip this score entirely
				continue
			case "details-yes":
				// This criterion should not be in appeal since main-question is now "No"
				// Skip this score entirely
				continue
			case "independent-question":
				// Appeal the independent question
				score.NumericValue = sql.NullFloat64{Float64: 2.0, Valid: true}
				filteredAppealScores = append(filteredAppealScores, score)
			default:
				filteredAppealScores = append(filteredAppealScores, score)
			}
		}
		// Update the scorecardIDToScores map with filtered scores
		scorecardIDToScores[appeal.ResourceID] = filteredAppealScores
		s.Require().NoError(db.Save(filteredAppealScores).Error)

		// Create new scores for criteria that should appear in appeal due to branch change
		newAppealScores := []*dbmodel.Scores{
			{
				Customer:            appeal.Customer,
				Profile:             appeal.Profile,
				ResourceID:          "appeal_reason_no",
				ScorecardID:         appeal.ResourceID,
				CriterionIdentifier: "reason-no",
				TextValue:           sql.NullString{Valid: true, String: "Technical Issue"},
				NumericValue:        sql.NullFloat64{Float64: 2.0, Valid: true},
			},
			{
				Customer:            appeal.Customer,
				Profile:             appeal.Profile,
				ResourceID:          "appeal_follow_up_no",
				ScorecardID:         appeal.ResourceID,
				CriterionIdentifier: "follow-up-no",
				TextValue:           sql.NullString{Valid: true, String: "Yes"},
				NumericValue:        sql.NullFloat64{Float64: 1.0, Valid: true},
			},
			{
				Customer:            appeal.Customer,
				Profile:             appeal.Profile,
				ResourceID:          "appeal_nested_follow_up",
				ScorecardID:         appeal.ResourceID,
				CriterionIdentifier: "nested-follow-up",
				NumericValue:        sql.NullFloat64{Float64: 2.0, Valid: true},
			},
		}
		err = db.Create(newAppealScores).Error
		s.Require().NoError(err)
		scorecardIDToScores[appeal.ResourceID] = append(scorecardIDToScores[appeal.ResourceID], newAppealScores...)

		// Resolve scores - main question stays "No", but some values are adjusted
		resolveScores := scorecardIDToScores[resolve.ResourceID]
		// Filter out scores for criteria that should not be visible due to branch change
		filteredResolveScores := []*dbmodel.Scores{}
		for _, score := range resolveScores {
			switch score.CriterionIdentifier {
			case "main-question":
				// Keep the appealed value
				score.TextValue = sql.NullString{Valid: true, String: "No"}
				score.NumericValue = sql.NullFloat64{Float64: 0.0, Valid: true}
				filteredResolveScores = append(filteredResolveScores, score)
			case "follow-up-yes":
				// This criterion should not be in resolve since main-question is "No"
				// Skip this score entirely
				continue
			case "details-yes":
				// This criterion should not be in resolve since main-question is "No"
				// Skip this score entirely
				continue
			case "independent-question":
				// Partially adjust the independent question
				score.NumericValue = sql.NullFloat64{Float64: 3.0, Valid: true}
				filteredResolveScores = append(filteredResolveScores, score)
			default:
				filteredResolveScores = append(filteredResolveScores, score)
			}
		}
		// Update the scorecardIDToScores map with filtered scores
		scorecardIDToScores[resolve.ResourceID] = filteredResolveScores
		s.Require().NoError(db.Save(filteredResolveScores).Error)

		// Create new scores for criteria that should appear in resolve due to branch change
		newResolveScores := []*dbmodel.Scores{
			{
				Customer:            resolve.Customer,
				Profile:             resolve.Profile,
				ResourceID:          "resolve_reason_no",
				ScorecardID:         resolve.ResourceID,
				CriterionIdentifier: "reason-no",
				TextValue:           sql.NullString{Valid: true, String: "Process Issue"},
				NumericValue:        sql.NullFloat64{Float64: 1.0, Valid: true},
			},
			{
				Customer:            resolve.Customer,
				Profile:             resolve.Profile,
				ResourceID:          "resolve_follow_up_no",
				ScorecardID:         resolve.ResourceID,
				CriterionIdentifier: "follow-up-no",
				TextValue:           sql.NullString{Valid: true, String: "Yes"},
				NumericValue:        sql.NullFloat64{Float64: 0.0, Valid: true},
			},
		}
		err = db.Create(newResolveScores).Error
		s.Require().NoError(err)
		scorecardIDToScores[resolve.ResourceID] = append(scorecardIDToScores[resolve.ResourceID], newResolveScores...)

		appealWorkflow := scoring.AppealWorkflow{
			Original:        scorecardWithBranches,
			OriginalReplica: replica,
			AppealRequest:   appeal,
			AppealResolve:   resolve,
		}

		// Create criteria IDs map for all possible criteria
		criteriaIDs := map[string]bool{
			"main-question":        true,
			"follow-up-yes":        true,
			"details-yes":          true,
			"reason-no":            true,
			"follow-up-no":         true,
			"nested-follow-up":     true,
			"independent-question": true,
			"resolution-note":      true,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appeal.ResourceID: {Scorecards: *appeal, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appeal.ConversationID: {
					CreatedAt: time.Time{},
				},
			},
			criteriaIDs,
		)

		// Define criterion keys for easier reference
		mainQuestionKey := CriterionConvKey{
			ScorecardTemplateID: templateWithBranches.ResourceID,
			CriterionID:         "main-question",
			ConversationID:      scorecardWithBranches.ConversationID,
			ScorecardID:         scorecardWithBranches.ResourceID,
		}

		reasonNoKey := CriterionConvKey{
			ScorecardTemplateID: templateWithBranches.ResourceID,
			CriterionID:         "reason-no",
			ConversationID:      scorecardWithBranches.ConversationID,
			ScorecardID:         scorecardWithBranches.ResourceID,
		}
		followUpNoKey := CriterionConvKey{
			ScorecardTemplateID: templateWithBranches.ResourceID,
			CriterionID:         "follow-up-no",
			ConversationID:      scorecardWithBranches.ConversationID,
			ScorecardID:         scorecardWithBranches.ResourceID,
		}
		nestedFollowUpKey := CriterionConvKey{
			ScorecardTemplateID: templateWithBranches.ResourceID,
			CriterionID:         "nested-follow-up",
			ConversationID:      scorecardWithBranches.ConversationID,
			ScorecardID:         scorecardWithBranches.ResourceID,
		}
		independentQuestionKey := CriterionConvKey{
			ScorecardTemplateID: templateWithBranches.ResourceID,
			CriterionID:         "independent-question",
			ConversationID:      scorecardWithBranches.ConversationID,
			ScorecardID:         scorecardWithBranches.ResourceID,
		}

		// Verify expected stats
		expectedStats := map[CriterionConvKey]*AppealInfo{
			mainQuestionKey: {
				IsAppealed: true, // Appealed from "Yes" to "No"
				IsResolved: true, // Has resolve score
				IsAdjusted: true, // Adjusted from "Yes" to "No"
			},
			// Note: followUpYesKey and detailsYesKey are no longer visible in the appeal scorecard
			// because they should not be visible due to branch change (main-question changed from "Yes" to "No")
			reasonNoKey: {
				IsAppealed: true, // Appealed (new criterion in appeal)
				IsResolved: true, // Has resolve score
				IsAdjusted: true, // Adjusted and does not exist in original scorecard
			},
			followUpNoKey: {
				IsAppealed: true, // Appealed (new criterion in appeal)
				IsResolved: true, // Has resolve score
				IsAdjusted: true, // Adjusted and does not exist in original scorecard
			},
			nestedFollowUpKey: {
				IsAppealed: true,  // Appealed (new criterion in appeal)
				IsResolved: false, // Removed from resolve scorecard
				IsAdjusted: false, // Not adjusted
			},
			independentQuestionKey: {
				IsAppealed: true, // Appealed (new criterion in appeal)
				IsResolved: true, // Has resolve score
				IsAdjusted: true, // Adjusted and does not exist in original scorecard
			},
		}

		s.Assert().Truef(
			cmp.Equal(stats, expectedStats, s.statsComparer),
			"expected stats: %v\n, actual stats: %v\n",
			expectedStats, stats,
		)
	})

	s.Run("per-message criteria - only specific messages appealed", func() {
		// Given: A scorecard with per-message criteria where:
		//   - msg1 has original score of 3
		//   - msg2 has original score of 4
		// And an appeal workflow where:
		//   - msg1 appeal score changed to 5
		//   - msg2 appeal score stayed at 4
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)

		scorecardA := &dbmodel.Scorecards{
			Customer:         s.customerID,
			Profile:          s.profileID,
			ResourceID:       uuid.NewString(),
			CreatedAt:        time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2021, 9, 23, 12, 2, 2, 0, time.UTC),
			SubmittedAt:      sql.NullTime{Valid: true, Time: time.Date(2021, 9, 23, 12, 12, 2, 0, time.UTC)},
			SubmitterUserID:  sql.NullString{Valid: true, String: s.QAAnalystUserID},
			TemplateID:       s.TemplateA.ResourceID,
			TemplateRevision: s.TemplateA.Revision,
			CreatorUserID:    sql.NullString{Valid: true, String: s.ManagerUserID},
			Score:            sql.NullFloat64{Float64: 51.0, Valid: true},
			AgentUserID:      s.AgentUserID,
			ConversationID:   uuid.NewString(),
			UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		}
		err = db.Create(scorecardA).Error
		s.Require().NoError(err)

		// Create per-message scores with MessageID populated
		perMessageCriterionID := "per-message-criterion"
		msg1ID := "msg-001"
		msg2ID := "msg-002"

		originalScores := []*dbmodel.Scores{
			{
				Customer:            scorecardA.Customer,
				Profile:             scorecardA.Profile,
				ResourceID:          "original_msg1",
				ScorecardID:         scorecardA.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
			{
				Customer:            scorecardA.Customer,
				Profile:             scorecardA.Profile,
				ResourceID:          "original_msg2",
				ScorecardID:         scorecardA.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID},
				NumericValue:        sql.NullFloat64{Float64: 4.0, Valid: true},
			},
		}
		err = db.Create(originalScores).Error
		s.Require().NoError(err)

		// Create replica and appeal scorecards
		replicaCard := &dbmodel.Scorecards{
			Customer:             scorecardA.Customer,
			Profile:              scorecardA.Profile,
			ResourceID:           uuid.NewString(),
			TemplateID:           scorecardA.TemplateID,
			TemplateRevision:     scorecardA.TemplateRevision,
			ConversationID:       scorecardA.ConversationID,
			AgentUserID:          scorecardA.AgentUserID,
			SubmittedAt:          scorecardA.SubmittedAt,
			SubmitterUserID:      scorecardA.SubmitterUserID,
			ScorecardType:        sql.NullInt32{Valid: true, Int32: int32(coaching.ScorecardType_SCORECARD_TYPE_APPEAL_ORIGINAL_REPLICA)},
			ReferenceScorecardID: sql.NullString{Valid: true, String: scorecardA.ResourceID},
		}
		err = db.Create(replicaCard).Error
		s.Require().NoError(err)

		replicaScores := []*dbmodel.Scores{
			{
				Customer:            replicaCard.Customer,
				Profile:             replicaCard.Profile,
				ResourceID:          "replica_msg1",
				ScorecardID:         replicaCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
			{
				Customer:            replicaCard.Customer,
				Profile:             replicaCard.Profile,
				ResourceID:          "replica_msg2",
				ScorecardID:         replicaCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID},
				NumericValue:        sql.NullFloat64{Float64: 4.0, Valid: true},
			},
		}
		err = db.Create(replicaScores).Error
		s.Require().NoError(err)

		appealCard := &dbmodel.Scorecards{
			Customer:             scorecardA.Customer,
			Profile:              scorecardA.Profile,
			ResourceID:           uuid.NewString(),
			TemplateID:           scorecardA.TemplateID,
			TemplateRevision:     scorecardA.TemplateRevision,
			ConversationID:       scorecardA.ConversationID,
			AgentUserID:          scorecardA.AgentUserID,
			SubmittedAt:          scorecardA.SubmittedAt,
			SubmitterUserID:      scorecardA.SubmitterUserID,
			ScorecardType:        sql.NullInt32{Valid: true, Int32: int32(coaching.ScorecardType_SCORECARD_TYPE_APPEAL_REQUEST)},
			ReferenceScorecardID: sql.NullString{Valid: true, String: replicaCard.ResourceID},
		}
		err = db.Create(appealCard).Error
		s.Require().NoError(err)

		// Appeal scores: msg1 changed to 5, msg2 stays at 4
		appealScores := []*dbmodel.Scores{
			{
				Customer:            appealCard.Customer,
				Profile:             appealCard.Profile,
				ResourceID:          "appeal_msg1",
				ScorecardID:         appealCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 5.0, Valid: true}, // Changed from 3 to 5
			},
			{
				Customer:            appealCard.Customer,
				Profile:             appealCard.Profile,
				ResourceID:          "appeal_msg2",
				ScorecardID:         appealCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID},
				NumericValue:        sql.NullFloat64{Float64: 4.0, Valid: true}, // Same as original
			},
		}
		err = db.Create(appealScores).Error
		s.Require().NoError(err)

		scorecardIDToScores := map[string][]*dbmodel.Scores{
			scorecardA.ResourceID:  originalScores,
			replicaCard.ResourceID: replicaScores,
			appealCard.ResourceID:  appealScores,
		}

		appealWorkflow := scoring.AppealWorkflow{
			Original:        scorecardA,
			OriginalReplica: replicaCard,
			AppealRequest:   appealCard,
		}

		// When: analyzeCriterionAppealInfo is called
		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appealCard.ResourceID: {Scorecards: *appealCard, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appealCard.ConversationID: {CreatedAt: time.Time{}},
			},
			map[string]bool{perMessageCriterionID: true},
		)

		// Then: msg1 should be marked as appealed, msg2 should NOT be appealed
		s.Assert().Len(stats, 2, "Should have 2 AppealInfo entries, one per message")

		// Find the entries by MessageID
		var msg1Stats, msg2Stats *AppealInfo
		for key, info := range stats {
			if key.MessageID == msg1ID {
				msg1Stats = info
			} else if key.MessageID == msg2ID {
				msg2Stats = info
			}
		}

		s.Require().NotNil(msg1Stats, "Should have stats for msg1")
		s.Require().NotNil(msg2Stats, "Should have stats for msg2")

		// msg1 should be appealed (value changed from 3 to 5)
		s.Assert().True(msg1Stats.IsAppealed, "msg1 should be appealed")
		s.Assert().Equal(msg1ID, msg1Stats.MessageID, "msg1 MessageID should be set")

		// msg2 should NOT be appealed (value stayed at 4)
		s.Assert().False(msg2Stats.IsAppealed, "msg2 should NOT be appealed")
		s.Assert().Equal(msg2ID, msg2Stats.MessageID, "msg2 MessageID should be set")
	})

	s.Run("per-message criteria - same value different message is appealed", func() {
		// Test that same values for different messages are NOT incorrectly matched
		// Original scores: msg1=3
		// Appeal scores: msg2=3 (same value, different message)
		// Expected: msg1 is appealed (removed), msg2 is appealed (added)
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)

		scorecardA := &dbmodel.Scorecards{
			Customer:         s.customerID,
			Profile:          s.profileID,
			ResourceID:       uuid.NewString(),
			CreatedAt:        time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2021, 9, 23, 12, 2, 2, 0, time.UTC),
			SubmittedAt:      sql.NullTime{Valid: true, Time: time.Date(2021, 9, 23, 12, 12, 2, 0, time.UTC)},
			SubmitterUserID:  sql.NullString{Valid: true, String: s.QAAnalystUserID},
			TemplateID:       s.TemplateA.ResourceID,
			TemplateRevision: s.TemplateA.Revision,
			AgentUserID:      s.AgentUserID,
			ConversationID:   uuid.NewString(),
			UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		}
		err = db.Create(scorecardA).Error
		s.Require().NoError(err)

		perMessageCriterionID := "per-message-criterion"
		msg1ID := "msg-001"
		msg2ID := "msg-002"

		originalScores := []*dbmodel.Scores{
			{
				Customer:            scorecardA.Customer,
				Profile:             scorecardA.Profile,
				ResourceID:          "original_msg1",
				ScorecardID:         scorecardA.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
		}
		err = db.Create(originalScores).Error
		s.Require().NoError(err)

		replicaCard := &dbmodel.Scorecards{
			Customer:             scorecardA.Customer,
			Profile:              scorecardA.Profile,
			ResourceID:           uuid.NewString(),
			TemplateID:           scorecardA.TemplateID,
			TemplateRevision:     scorecardA.TemplateRevision,
			ConversationID:       scorecardA.ConversationID,
			AgentUserID:          scorecardA.AgentUserID,
			SubmittedAt:          scorecardA.SubmittedAt,
			SubmitterUserID:      scorecardA.SubmitterUserID,
			ScorecardType:        sql.NullInt32{Valid: true, Int32: int32(coaching.ScorecardType_SCORECARD_TYPE_APPEAL_ORIGINAL_REPLICA)},
			ReferenceScorecardID: sql.NullString{Valid: true, String: scorecardA.ResourceID},
		}
		err = db.Create(replicaCard).Error
		s.Require().NoError(err)

		replicaScores := []*dbmodel.Scores{
			{
				Customer:            replicaCard.Customer,
				Profile:             replicaCard.Profile,
				ResourceID:          "replica_msg1",
				ScorecardID:         replicaCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
		}
		err = db.Create(replicaScores).Error
		s.Require().NoError(err)

		appealCard := &dbmodel.Scorecards{
			Customer:             scorecardA.Customer,
			Profile:              scorecardA.Profile,
			ResourceID:           uuid.NewString(),
			TemplateID:           scorecardA.TemplateID,
			TemplateRevision:     scorecardA.TemplateRevision,
			ConversationID:       scorecardA.ConversationID,
			AgentUserID:          scorecardA.AgentUserID,
			SubmittedAt:          scorecardA.SubmittedAt,
			SubmitterUserID:      scorecardA.SubmitterUserID,
			ScorecardType:        sql.NullInt32{Valid: true, Int32: int32(coaching.ScorecardType_SCORECARD_TYPE_APPEAL_REQUEST)},
			ReferenceScorecardID: sql.NullString{Valid: true, String: replicaCard.ResourceID},
		}
		err = db.Create(appealCard).Error
		s.Require().NoError(err)

		// Appeal scores: different message, same value
		appealScores := []*dbmodel.Scores{
			{
				Customer:            appealCard.Customer,
				Profile:             appealCard.Profile,
				ResourceID:          "appeal_msg2",
				ScorecardID:         appealCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID}, // Different message
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},  // Same value
			},
		}
		err = db.Create(appealScores).Error
		s.Require().NoError(err)

		scorecardIDToScores := map[string][]*dbmodel.Scores{
			scorecardA.ResourceID:  originalScores,
			replicaCard.ResourceID: replicaScores,
			appealCard.ResourceID:  appealScores,
		}

		appealWorkflow := scoring.AppealWorkflow{
			Original:        scorecardA,
			OriginalReplica: replicaCard,
			AppealRequest:   appealCard,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appealCard.ResourceID: {Scorecards: *appealCard, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appealCard.ConversationID: {CreatedAt: time.Time{}},
			},
			map[string]bool{perMessageCriterionID: true},
		)

		// Verify: should have 2 entries - one for msg1 (removed), one for msg2 (added)
		s.Assert().Len(stats, 2, "Should have 2 AppealInfo entries (msg1 removed, msg2 added)")

		var msg1Stats, msg2Stats *AppealInfo
		for key, info := range stats {
			if key.MessageID == msg1ID {
				msg1Stats = info
			} else if key.MessageID == msg2ID {
				msg2Stats = info
			}
		}

		s.Require().NotNil(msg1Stats, "Should have stats for msg1")
		s.Require().NotNil(msg2Stats, "Should have stats for msg2")

		// msg1 should be appealed (was in original, removed in appeal)
		s.Assert().True(msg1Stats.IsAppealed, "msg1 should be appealed (removed)")

		// msg2 should be appealed (wasn't in original, added in appeal)
		s.Assert().True(msg2Stats.IsAppealed, "msg2 should be appealed (added)")
	})

	s.Run("mixed per-message and non-per-message criteria", func() {
		// Test that both per-message and regular criteria work together
		db := s.DB()
		err := db.Create(s.TemplateA).Error
		s.Require().NoError(err)

		scorecardA := &dbmodel.Scorecards{
			Customer:         s.customerID,
			Profile:          s.profileID,
			ResourceID:       uuid.NewString(),
			CreatedAt:        time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2021, 9, 23, 12, 2, 2, 0, time.UTC),
			SubmittedAt:      sql.NullTime{Valid: true, Time: time.Date(2021, 9, 23, 12, 12, 2, 0, time.UTC)},
			SubmitterUserID:  sql.NullString{Valid: true, String: s.QAAnalystUserID},
			TemplateID:       s.TemplateA.ResourceID,
			TemplateRevision: s.TemplateA.Revision,
			AgentUserID:      s.AgentUserID,
			ConversationID:   uuid.NewString(),
			UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
		}
		err = db.Create(scorecardA).Error
		s.Require().NoError(err)

		regularCriterionID := "regular-criterion"
		perMessageCriterionID := "per-message-criterion"
		msg1ID := "msg-001"
		msg2ID := "msg-002"

		originalScores := []*dbmodel.Scores{
			// Regular criterion (no MessageID)
			{
				Customer:            scorecardA.Customer,
				Profile:             scorecardA.Profile,
				ResourceID:          "original_regular",
				ScorecardID:         scorecardA.ResourceID,
				CriterionIdentifier: regularCriterionID,
				NumericValue:        sql.NullFloat64{Float64: 5.0, Valid: true},
			},
			// Per-message criterion msg1
			{
				Customer:            scorecardA.Customer,
				Profile:             scorecardA.Profile,
				ResourceID:          "original_msg1",
				ScorecardID:         scorecardA.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
			// Per-message criterion msg2
			{
				Customer:            scorecardA.Customer,
				Profile:             scorecardA.Profile,
				ResourceID:          "original_msg2",
				ScorecardID:         scorecardA.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID},
				NumericValue:        sql.NullFloat64{Float64: 4.0, Valid: true},
			},
		}
		err = db.Create(originalScores).Error
		s.Require().NoError(err)

		replicaCard := &dbmodel.Scorecards{
			Customer:             scorecardA.Customer,
			Profile:              scorecardA.Profile,
			ResourceID:           uuid.NewString(),
			TemplateID:           scorecardA.TemplateID,
			TemplateRevision:     scorecardA.TemplateRevision,
			ConversationID:       scorecardA.ConversationID,
			AgentUserID:          scorecardA.AgentUserID,
			SubmittedAt:          scorecardA.SubmittedAt,
			SubmitterUserID:      scorecardA.SubmitterUserID,
			ScorecardType:        sql.NullInt32{Valid: true, Int32: int32(coaching.ScorecardType_SCORECARD_TYPE_APPEAL_ORIGINAL_REPLICA)},
			ReferenceScorecardID: sql.NullString{Valid: true, String: scorecardA.ResourceID},
		}
		err = db.Create(replicaCard).Error
		s.Require().NoError(err)

		replicaScores := []*dbmodel.Scores{
			{
				Customer:            replicaCard.Customer,
				Profile:             replicaCard.Profile,
				ResourceID:          "replica_regular",
				ScorecardID:         replicaCard.ResourceID,
				CriterionIdentifier: regularCriterionID,
				NumericValue:        sql.NullFloat64{Float64: 5.0, Valid: true},
			},
			{
				Customer:            replicaCard.Customer,
				Profile:             replicaCard.Profile,
				ResourceID:          "replica_msg1",
				ScorecardID:         replicaCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true},
			},
			{
				Customer:            replicaCard.Customer,
				Profile:             replicaCard.Profile,
				ResourceID:          "replica_msg2",
				ScorecardID:         replicaCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID},
				NumericValue:        sql.NullFloat64{Float64: 4.0, Valid: true},
			},
		}
		err = db.Create(replicaScores).Error
		s.Require().NoError(err)

		appealCard := &dbmodel.Scorecards{
			Customer:             scorecardA.Customer,
			Profile:              scorecardA.Profile,
			ResourceID:           uuid.NewString(),
			TemplateID:           scorecardA.TemplateID,
			TemplateRevision:     scorecardA.TemplateRevision,
			ConversationID:       scorecardA.ConversationID,
			AgentUserID:          scorecardA.AgentUserID,
			SubmittedAt:          scorecardA.SubmittedAt,
			SubmitterUserID:      scorecardA.SubmitterUserID,
			ScorecardType:        sql.NullInt32{Valid: true, Int32: int32(coaching.ScorecardType_SCORECARD_TYPE_APPEAL_REQUEST)},
			ReferenceScorecardID: sql.NullString{Valid: true, String: replicaCard.ResourceID},
		}
		err = db.Create(appealCard).Error
		s.Require().NoError(err)

		// Appeal scores:
		// - regular: 5->7 (appealed)
		// - msg1: 3->3 (NOT appealed)
		// - msg2: 4->6 (appealed)
		appealScores := []*dbmodel.Scores{
			{
				Customer:            appealCard.Customer,
				Profile:             appealCard.Profile,
				ResourceID:          "appeal_regular",
				ScorecardID:         appealCard.ResourceID,
				CriterionIdentifier: regularCriterionID,
				NumericValue:        sql.NullFloat64{Float64: 7.0, Valid: true}, // Changed
			},
			{
				Customer:            appealCard.Customer,
				Profile:             appealCard.Profile,
				ResourceID:          "appeal_msg1",
				ScorecardID:         appealCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg1ID},
				NumericValue:        sql.NullFloat64{Float64: 3.0, Valid: true}, // Same
			},
			{
				Customer:            appealCard.Customer,
				Profile:             appealCard.Profile,
				ResourceID:          "appeal_msg2",
				ScorecardID:         appealCard.ResourceID,
				CriterionIdentifier: perMessageCriterionID,
				MessageID:           sql.NullString{Valid: true, String: msg2ID},
				NumericValue:        sql.NullFloat64{Float64: 6.0, Valid: true}, // Changed
			},
		}
		err = db.Create(appealScores).Error
		s.Require().NoError(err)

		scorecardIDToScores := map[string][]*dbmodel.Scores{
			scorecardA.ResourceID:  originalScores,
			replicaCard.ResourceID: replicaScores,
			appealCard.ResourceID:  appealScores,
		}

		appealWorkflow := scoring.AppealWorkflow{
			Original:        scorecardA,
			OriginalReplica: replicaCard,
			AppealRequest:   appealCard,
		}

		stats := analyzeCriterionAppealInfo(
			[]*scoring.AppealWorkflow{&appealWorkflow},
			scorecardIDToScores,
			map[string]*ScorecardWithFrequency{
				appealCard.ResourceID: {Scorecards: *appealCard, Frequency: sql.NullTime{Valid: true}},
			},
			map[string]*dbmodel.Chats{
				appealCard.ConversationID: {CreatedAt: time.Time{}},
			},
			map[string]bool{
				regularCriterionID:    true,
				perMessageCriterionID: true,
			},
		)

		// Verify: should have 3 entries:
		// - 1 for regular criterion (empty MessageID)
		// - 2 for per-message criterion (one per message)
		s.Assert().Len(stats, 3, "Should have 3 AppealInfo entries")

		var regularStats, msg1Stats, msg2Stats *AppealInfo
		for key, info := range stats {
			switch {
			case key.CriterionID == regularCriterionID:
				regularStats = info
				s.Assert().Equal("", key.MessageID, "Regular criterion should have empty MessageID in key")
			case key.CriterionID == perMessageCriterionID && key.MessageID == msg1ID:
				msg1Stats = info
			case key.CriterionID == perMessageCriterionID && key.MessageID == msg2ID:
				msg2Stats = info

			}
		}

		s.Require().NotNil(regularStats, "Should have stats for regular criterion")
		s.Require().NotNil(msg1Stats, "Should have stats for msg1")
		s.Require().NotNil(msg2Stats, "Should have stats for msg2")

		// Regular criterion should be appealed (5->7)
		s.Assert().True(regularStats.IsAppealed, "Regular criterion should be appealed")
		s.Assert().Equal("", regularStats.MessageID, "Regular criterion should have empty MessageID")

		// msg1 should NOT be appealed (3->3)
		s.Assert().False(msg1Stats.IsAppealed, "msg1 should NOT be appealed")
		s.Assert().Equal(msg1ID, msg1Stats.MessageID, "msg1 should have correct MessageID")

		// msg2 should be appealed (4->6)
		s.Assert().True(msg2Stats.IsAppealed, "msg2 should be appealed")
		s.Assert().Equal(msg2ID, msg2Stats.MessageID, "msg2 should have correct MessageID")
	})
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealStatsInternalGroupByQAAnalystAndcriterion() {
	// Given
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	qaAnalystID := "qa_analyst_a"
	s.ScorecardA.SubmitterUserID = sql.NullString{Valid: true, String: qaAnalystID}
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, appealCard, resolveCard, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)
	// update scores
	appealScores := scorecardIDToScores[appealCard.ResourceID]
	appealScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
	appealScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}
	s.Require().NoError(db.Save(appealScores).Error)
	resolveScores := scorecardIDToScores[resolveCard.ResourceID]
	resolveScores[0].TextValue = sql.NullString{Valid: true, String: "worst"}
	resolveScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 12.0}
	s.Require().NoError(db.Save(resolveScores).Error)

	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: s.ScorecardA.ConversationID},
	}
	err = db.Create(conv).Error
	s.Require().NoError(err)

	userFilterParser := userfiltermock.NewMockUserFilterParser(s.T())
	agentName := userpb.UserName{
		CustomerID: s.customerID,
		UserID:     s.AgentUserID,
	}
	userFilterParser.EXPECT().
		Parse(mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentName.String()},
		}, nil)

	request := &analytics.RetrieveAppealStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(s.ScorecardA.CreatedAt.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(s.ScorecardA.CreatedAt.Add(time.Hour)),
		},
		Frequency: analytics.Frequency_WEEKLY,
		Metadata:  &analytics.Metadata{},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", s.customerID, s.profileID, s.usecaseID)},
			ScorecardTemplates: []string{
				coaching.ScorecardTemplateName{
					CustomerID:                  s.customerID,
					ProfileID:                   s.profileID,
					ScorecardTemplateID:         s.TemplateA.ResourceID,
					ScorecardTemplateRevisionID: s.TemplateA.Revision,
				}.String(),
			},
		},
		GroupByAttributes: []analytics.QAAttributeType{
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_QA_ANALYST,
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.userServiceClient.EXPECT().
		ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{{
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, s.AgentUserID),
			}, {
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, qaAnalystID),
			}},
		}, nil)

		// When
	resp, err := retrieveAppealStatsInternal(
		log.GetLogger(),
		db,
		profile,
		s.mockConfigClient,
		s.mockACLHelper,
		s.userServiceClient,
		userFilterParser,
		RetrieveAppealStatsRequestInternal{
			Context:                    s.ctx,
			RetrieveAppealStatsRequest: request,
		},
	)

	// Verify
	s.Require().NoError(err)
	s.Assert().EqualValues(resp.TotalAppealStats.ResolvedCriteriaCount, 2)
	s.Assert().EqualValues(resp.AppealStats[0].ResolvedCriteriaCount, 1)
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealStatsInternal() {
	// Given
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	qaAnalystID := "qa_analyst_a"
	s.ScorecardA.SubmitterUserID = sql.NullString{Valid: true, String: qaAnalystID}
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, _, _, _, err = s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)
	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: s.ScorecardA.ConversationID},
	}
	err = db.Create(conv).Error
	s.Require().NoError(err)

	userFilterParser := userfiltermock.NewMockUserFilterParser(s.T())
	agentName := userpb.UserName{
		CustomerID: s.customerID,
		UserID:     s.AgentUserID,
	}
	userFilterParser.EXPECT().
		Parse(mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentName.String()},
		}, nil)

	request := &analytics.RetrieveAppealStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(time.Hour)),
		},
		Frequency: analytics.Frequency_WEEKLY,
		Metadata:  &analytics.Metadata{},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", s.customerID, s.profileID, s.usecaseID)},
			ScorecardTemplates: []string{
				coaching.ScorecardTemplateName{
					CustomerID:                  s.customerID,
					ProfileID:                   s.profileID,
					ScorecardTemplateID:         s.TemplateA.ResourceID,
					ScorecardTemplateRevisionID: s.TemplateA.Revision,
				}.String(),
			},
		},
		GroupByAttributes: []analytics.QAAttributeType{analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT},
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.userServiceClient.EXPECT().
		ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{{
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, s.AgentUserID),
			}, {
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, qaAnalystID),
			}},
		}, nil)

		// When
	resp, err := retrieveAppealStatsInternal(
		log.GetLogger(),
		db,
		profile,
		s.mockConfigClient,
		s.mockACLHelper,
		s.userServiceClient,
		userFilterParser,
		RetrieveAppealStatsRequestInternal{
			Context:                    s.ctx,
			RetrieveAppealStatsRequest: request,
		},
	)

	// Verify
	s.Require().NoError(err)
	s.Assert().EqualValues(resp.TotalAppealStats.ResolvedCriteriaCount, 0)
	s.Assert().EqualValues(resp.AppealStats[0].ResolvedCriteriaCount, 0)
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealStatsInternal_ConvoFilter() {
	// Given
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	qaAnalystID := "qa_analyst_id"
	s.ScorecardA.SubmitterUserID = sql.NullString{Valid: true, String: qaAnalystID}
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, _, _, _, err = s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)
	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: s.ScorecardA.ConversationID},
	}
	err = db.Create(conv).Error
	s.Require().NoError(err)

	userFilterParser := userfiltermock.NewMockUserFilterParser(s.T())

	request := &analytics.RetrieveAppealStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(time.Hour)),
		},
		Frequency: analytics.Frequency_WEEKLY,
		Metadata:  &analytics.Metadata{},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", s.customerID, s.profileID, s.usecaseID)},
			ScorecardTemplates: []string{
				coaching.ScorecardTemplateName{
					CustomerID:                  s.customerID,
					ProfileID:                   s.profileID,
					ScorecardTemplateID:         s.TemplateA.ResourceID,
					ScorecardTemplateRevisionID: s.TemplateA.Revision,
				}.String(),
			},
			ConversationNames: []string{conversationpb.ConversationName{
				CustomerID:     s.ScorecardA.Customer,
				ProfileID:      s.ScorecardA.Profile,
				ConversationID: s.ScorecardA.ConversationID,
			}.String()},
		},
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.userServiceClient.EXPECT().
		ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{{
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, s.AgentUserID),
			}, {
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, qaAnalystID),
			}},
		}, nil)

		// When
	resp, err := retrieveAppealStatsInternal(
		log.GetLogger(),
		db,
		profile,
		s.mockConfigClient,
		s.mockACLHelper,
		s.userServiceClient,
		userFilterParser,
		RetrieveAppealStatsRequestInternal{
			Context:                    s.ctx,
			RetrieveAppealStatsRequest: request,
		},
	)

	// Verify
	s.Require().NoError(err)
	s.Assert().EqualValues(resp.TotalAppealStats.ResolvedCriteriaCount, 0)
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealStatsInternalGroupByQAAnalystAndcriterion_UnresolvedMultiSelectRequest() {
	// Given
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	qaAnalystID := "qa_analyst_a"
	s.ScorecardA.SubmitterUserID = sql.NullString{Valid: true, String: qaAnalystID}
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	err = db.Create(s.MultiSelectScores).Error
	s.Require().NoError(err)
	_, appealCard, _, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, append(s.Scores, s.MultiSelectScores...), true, false)
	s.Require().NoError(err)
	// update scores
	appealScores := scorecardIDToScores[appealCard.ResourceID]
	appealScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
	appealScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}
	appealScores[2].NumericValue = sql.NullFloat64{Valid: true, Float64: 2.0}
	appealScores[3].NumericValue = sql.NullFloat64{Valid: true, Float64: 0.0}
	s.Require().NoError(db.Save(appealScores).Error)
	// resolveScores := scorecardIDToScores[resolveCard.ResourceID]
	// resolveScores[0].TextValue = sql.NullString{Valid: true, String: "worst"}
	// resolveScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 12.0}
	// s.Require().NoError(db.Save(resolveScores).Error)

	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: s.ScorecardA.ConversationID},
	}
	err = db.Create(conv).Error
	s.Require().NoError(err)

	userFilterParser := userfiltermock.NewMockUserFilterParser(s.T())
	agentName := userpb.UserName{
		CustomerID: s.customerID,
		UserID:     s.AgentUserID,
	}
	userFilterParser.EXPECT().
		Parse(mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentName.String()},
		}, nil)

	request := &analytics.RetrieveAppealStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(s.ScorecardA.CreatedAt.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(s.ScorecardA.CreatedAt.Add(time.Hour)),
		},
		Frequency: analytics.Frequency_WEEKLY,
		Metadata:  &analytics.Metadata{},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", s.customerID, s.profileID, s.usecaseID)},
			ScorecardTemplates: []string{
				coaching.ScorecardTemplateName{
					CustomerID:                  s.customerID,
					ProfileID:                   s.profileID,
					ScorecardTemplateID:         s.TemplateA.ResourceID,
					ScorecardTemplateRevisionID: s.TemplateA.Revision,
				}.String(),
			},
		},
		GroupByAttributes: []analytics.QAAttributeType{
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_QA_ANALYST,
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.userServiceClient.EXPECT().
		ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{{
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, s.AgentUserID),
			}, {
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, qaAnalystID),
			}},
		}, nil)

		// When
	resp, err := retrieveAppealStatsInternal(
		log.GetLogger(),
		db,
		profile,
		s.mockConfigClient,
		s.mockACLHelper,
		s.userServiceClient,
		userFilterParser,
		RetrieveAppealStatsRequestInternal{
			Context:                    s.ctx,
			RetrieveAppealStatsRequest: request,
		},
	)

	// Verify
	s.Require().NoError(err)
	s.Assert().EqualValues(0, resp.TotalAppealStats.ResolvedCriteriaCount)
	s.Assert().EqualValues(3, resp.TotalAppealStats.AppealedCriteriaCount)
	s.Assert().EqualValues(0, resp.AppealStats[0].ResolvedCriteriaCount)
}

func (s *RetrieveAppealStatsTestSuite) TestGetFilteredWorkflowsByOriginalReviewers() {
	// Given
	db := s.DB()
	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	qaAnalystID := "reviewer_a"
	s.ScorecardA.SubmitterUserID = sql.NullString{Valid: true, String: qaAnalystID}
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, appealCard, resolveCard, scorecardIDToScores, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)

	appealScores := scorecardIDToScores[appealCard.ResourceID]
	appealScores[0].TextValue = sql.NullString{Valid: true, String: "bad"}
	appealScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 11.0}
	s.Require().NoError(db.Save(appealScores).Error)
	resolveScores := scorecardIDToScores[resolveCard.ResourceID]
	resolveScores[0].TextValue = sql.NullString{Valid: true, String: "worst"}
	resolveScores[1].NumericValue = sql.NullFloat64{Valid: true, Float64: 12.0}
	s.Require().NoError(db.Save(resolveScores).Error)

	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: s.ScorecardA.ConversationID},
	}
	err = db.Create(conv).Error
	s.Require().NoError(err)

	agentName := userpb.UserName{
		CustomerID: s.customerID,
		UserID:     s.AgentUserID,
	}
	userFilterParser := userfiltermock.NewMockUserFilterParser(s.T())

	userFilterParser.EXPECT().
		Parse(mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentName.String()},
		}, nil)

	request := &analytics.RetrieveAppealStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(time.Hour)),
		},
		Frequency: analytics.Frequency_WEEKLY,
		Metadata:  &analytics.Metadata{},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", s.customerID, s.profileID, s.usecaseID)},
			ScorecardTemplates: []string{
				coaching.ScorecardTemplateName{
					CustomerID:                  s.customerID,
					ProfileID:                   s.profileID,
					ScorecardTemplateID:         s.TemplateA.ResourceID,
					ScorecardTemplateRevisionID: s.TemplateA.Revision,
				}.String(),
			},
			ScorecardReviewerAudience: &analytics.UserAudience{
				Users: []string{fmt.Sprintf("customers/%s/users/%s", s.customerID, "reviewer_b")},
			},
		},
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.userServiceClient.EXPECT().
		ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{{
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, s.AgentUserID),
			}, {
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, qaAnalystID),
			}},
		}, nil)

	// When
	resp, err := retrieveAppealStatsInternal(
		log.GetLogger(),
		db,
		profile,
		s.mockConfigClient,
		s.mockACLHelper,
		s.userServiceClient,
		userFilterParser,
		RetrieveAppealStatsRequestInternal{
			Context:                    s.ctx,
			RetrieveAppealStatsRequest: request,
		},
	)

	// Verify
	s.Require().NoError(err)
	s.Assert().EqualValues(resp.TotalAppealStats.ResolvedCriteriaCount, 0)
}

// set up appeal scorecard from the given original scorecard and scores
func (s *RetrieveAppealStatsTestSuite) SetupScorecard(
	db *gorm.DB, template *dbmodel.ScorecardTemplates,
	card *dbmodel.Scorecards, scores []*dbmodel.Scores,
	createAppealRequest, createAppealResolve bool,
) (
	replicaCard, appealCard, resolveCard *dbmodel.Scorecards,
	scorecardIDToScores map[string][]*dbmodel.Scores,
	err error,
) {
	scorecardIDToScores = make(map[string][]*dbmodel.Scores)
	scorecardIDToScores[card.ResourceID] = scores

	createCardAndScores := func(db *gorm.DB, cardType coaching.ScorecardType, reference string) (*dbmodel.Scorecards, error) {
		var err error
		newCard := *card
		newCard.ResourceID = uuid.NewString() + "-" + card.ResourceID
		newCard.ScorecardType = sql.NullInt32{
			Valid: true,
			Int32: int32(cardType),
		}
		newCard.ReferenceScorecardID = sql.NullString{
			Valid:  true,
			String: reference,
		}
		newScores := []*dbmodel.Scores{}
		for _, score := range scores {
			newScore := *score
			newScore.ResourceID = uuid.NewString() + "-" + score.ResourceID
			newScore.ScorecardID = newCard.ResourceID
			newScores = append(newScores, &newScore)
		}
		err = errors.Join(err, db.Create(&newCard).Error)
		err = errors.Join(err, db.Create(&newScores).Error)
		scorecardIDToScores[newCard.ResourceID] = newScores
		return &newCard, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var err error
		if createAppealRequest {
			var appealErr error
			replicaCard, appealErr = createCardAndScores(
				tx, coaching.ScorecardType_SCORECARD_TYPE_APPEAL_ORIGINAL_REPLICA, card.ResourceID)
			err = errors.Join(err, appealErr)
			appealCard, appealErr = createCardAndScores(
				tx, coaching.ScorecardType_SCORECARD_TYPE_APPEAL_REQUEST, replicaCard.ResourceID)
			err = errors.Join(err, appealErr)
		}

		if createAppealResolve {
			var resolveErr error
			resolveCard, resolveErr = createCardAndScores(
				tx, coaching.ScorecardType_SCORECARD_TYPE_APPEAL_RESOLVE, appealCard.ResourceID)
			err = errors.Join(err, resolveErr)
		}

		return err
	})
	return replicaCard, appealCard, resolveCard, scorecardIDToScores, err
}

func (s *RetrieveAppealStatsTestSuite) TestRetrieveAppealStatsInternal_NoTemplateID() {
	// Given - Create multiple templates and scorecards to test that all are returned when no template ID is specified
	db := s.DB()

	// Create a second template
	templateB := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: uuid.NewString(),
		Revision:   "r1",
		Title:      "02 Another template",
		CreatedAt:  time.Date(2021, 11, 24, 12, 0, 0, 0, time.UTC),
		UsecaseIds: []string{s.usecaseID},
		Template: `{
			"version": 2,
			"items": [
			{
				"displayName": "Another Criterion",
				"type": "sentence",
				"weight": 1,
				"identifier": "criterion3"
			}]
		}`,
	}

	err := db.Create(s.TemplateA).Error
	s.Require().NoError(err)
	err = db.Create(templateB).Error
	s.Require().NoError(err)

	// Create scorecard for template A
	qaAnalystID := "qa_analyst_a"
	s.ScorecardA.SubmitterUserID = sql.NullString{Valid: true, String: qaAnalystID}
	err = db.Create(s.ScorecardA).Error
	s.Require().NoError(err)
	err = db.Create(s.Scores).Error
	s.Require().NoError(err)
	_, appealScorecardA, _, scorecardIDToScoresA, err := s.SetupScorecard(db, s.TemplateA, s.ScorecardA, s.Scores, true, true)
	s.Require().NoError(err)

	// Update appeal scores for template A to create actual appeal scenarios
	appealScoresA := scorecardIDToScoresA[appealScorecardA.ResourceID]
	appealScoresA[0].TextValue = sql.NullString{Valid: true, String: appealScoresA[0].TextValue.String + " (appealed)"}
	appealScoresA[0].NumericValue = sql.NullFloat64{Valid: true, Float64: appealScoresA[0].NumericValue.Float64 + 1.0}
	s.Require().NoError(db.Save(appealScoresA).Error)

	// Create scorecard for template B
	scorecardB := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       uuid.NewString(),
		CreatedAt:        time.Date(2021, 9, 23, 12, 2, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2021, 9, 23, 12, 2, 2, 0, time.UTC),
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Date(2021, 9, 23, 12, 12, 2, 0, time.UTC)},
		SubmitterUserID:  sql.NullString{Valid: true, String: s.QAAnalystUserID},
		TemplateID:       templateB.ResourceID,
		TemplateRevision: templateB.Revision,
		CreatorUserID:    sql.NullString{Valid: true, String: s.ManagerUserID},
		Score:            sql.NullFloat64{Float64: 75.0, Valid: true},
		AgentUserID:      s.AgentUserID,
		ConversationID:   uuid.NewString(),
		Comment:          sql.NullString{Valid: true, String: "Another conversation!"},
		UsecaseID:        sql.NullString{Valid: true, String: s.usecaseID},
	}

	scoresB := []*dbmodel.Scores{
		{
			Customer:            scorecardB.Customer,
			Profile:             scorecardB.Profile,
			ResourceID:          "score_2_1",
			ScorecardID:         scorecardB.ResourceID,
			CriterionIdentifier: "criterion3",
			Comment:             sql.NullString{Valid: true, String: "Another Value"},
			CommentAccessRoles:  []int32{int32(authpb.AuthProto_MANAGER)},
			TextValue:           sql.NullString{Valid: true, String: "excellent"},
		},
	}

	err = db.Create(scorecardB).Error
	s.Require().NoError(err)
	err = db.Create(scoresB).Error
	s.Require().NoError(err)
	_, appealScorecardB, _, scorecardIDToScoresB, err := s.SetupScorecard(db, templateB, scorecardB, scoresB, true, true)
	s.Require().NoError(err)

	// Update appeal scores to create actual appeal scenarios
	appealScoresB := scorecardIDToScoresB[appealScorecardB.ResourceID]
	appealScoresB[0].TextValue = sql.NullString{Valid: true, String: appealScoresB[0].TextValue.String + " (appealed)"}
	appealScoresB[0].NumericValue = sql.NullFloat64{Valid: true, Float64: appealScoresB[0].NumericValue.Float64 + 1.0}
	s.Require().NoError(db.Save(appealScoresB).Error)

	// Create conversations for both scorecards
	convA := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: s.ScorecardA.ConversationID},
	}
	convB := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		CreatedAt:      fn.Must(time.Parse(time.DateOnly, "2024-01-05")),
		ConversationID: sql.NullString{Valid: true, String: scorecardB.ConversationID},
	}
	err = db.Create(convA).Error
	s.Require().NoError(err)
	err = db.Create(convB).Error
	s.Require().NoError(err)

	userFilterParser := userfiltermock.NewMockUserFilterParser(s.T())
	agentName := userpb.UserName{
		CustomerID: s.customerID,
		UserID:     s.AgentUserID,
	}
	userFilterParser.EXPECT().
		Parse(mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentName.String()},
		}, nil)

	request := &analytics.RetrieveAppealStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(s.ScorecardA.SubmittedAt.Time.Add(time.Hour)),
		},
		Frequency: analytics.Frequency_WEEKLY,
		Metadata:  &analytics.Metadata{},
		FilterByAttribute: &analytics.QAAttribute{
			UsecaseNames: []string{fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", s.customerID, s.profileID, s.usecaseID)},
			// No ScorecardTemplates specified - should return all
		},
		GroupByAttributes: []analytics.QAAttributeType{analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT},
	}

	profile := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	s.userServiceClient.EXPECT().
		ListUsers(mock.Anything, mock.Anything).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{{
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, s.AgentUserID),
			}, {
				Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, qaAnalystID),
			}},
		}, nil)

	// When
	resp, err := retrieveAppealStatsInternal(
		log.GetLogger(),
		db,
		profile,
		s.mockConfigClient,
		s.mockACLHelper,
		s.userServiceClient,
		userFilterParser,
		RetrieveAppealStatsRequestInternal{
			Context:                    s.ctx,
			RetrieveAppealStatsRequest: request,
		},
	)

	// Verify
	s.Require().NoError(err)
	// Should have results from both templates
	s.Len(resp.AppealStats, 1)                                          // Should have 1 group (1 agent)
	s.Assert().Equal(int(resp.AppealStats[0].AppealedCriteriaCount), 2) // Both templates should be appealed
	s.Assert().Equal(int(resp.AppealStats[0].ResolvedCriteriaCount), 2) // Both should be resolved
}
