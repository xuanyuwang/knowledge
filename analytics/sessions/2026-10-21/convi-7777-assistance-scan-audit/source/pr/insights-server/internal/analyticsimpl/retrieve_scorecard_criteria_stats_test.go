package analyticsimpl

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
)

type RetrieveScorecardCriteriaStatsTestSuite struct {
	AnalyticsTestSuite
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) SetupSuite() {
	s.AnalyticsTestSuite.SetupSuite()
	// Avoid ACL narrowing filters in tests
	s.mockConfigClient.EXPECT().Get(gomock.Any()).AnyTimes().Return(&config.CustomerConfig{}, nil)
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) AfterTest(suiteName, testName string) {
	db := s.DB()
	db.Delete(&dbmodel.ScorecardTemplates{}, "1=1")
	db.Delete(&dbmodel.Scorecards{}, "1=1")
	db.Delete(&dbmodel.Scores{}, "1=1")
	db.Delete(&dbmodel.Chats{}, "1=1")
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestBasicCount() {
	templateJSON := fmt.Sprintf(`{
		"version": 2,
		"items": [
			{
				"type": "labeled-radios",
				"identifier": "greeting",
				"displayName": "Greeting Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			},
			{
				"type": "labeled-radios",
				"identifier": "empathy",
				"displayName": "Empathy Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/empathy"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`, s.customerID, s.profileID, s.customerID, s.profileID)

	tmpl := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "tmpl1",
		Revision:   "r1",
		Title:      "Template 1",
		Template:   templateJSON,
	}

	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv1"},
	}

	scorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc1",
		ConversationID:   "conv1",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}

	overwrittenScore1 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score1",
		ScorecardID:         scorecard.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.80, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.80, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenScore2 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score2",
		ScorecardID:         scorecard.ResourceID,
		CriterionIdentifier: "empathy",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.60, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	s.CreateModels(tmpl, conv, scorecard, overwrittenScore1, overwrittenScore2)

	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
	})

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(int64(1), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestGroupByBehavior() {
	currentTime := time.Now()

	templateJSON := fmt.Sprintf(`{
		"version": 2,
		"items": [
			{
				"type": "labeled-radios",
				"identifier": "greeting",
				"displayName": "Greeting Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			},
			{
				"type": "labeled-radios",
				"identifier": "empathy",
				"displayName": "Empathy Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/empathy"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			},
			{
				"type": "labeled-radios",
				"identifier": "ignoredbehavior",
				"displayName": "Ignored Behavior",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/ignoredbehavior"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`, s.customerID, s.profileID, s.customerID, s.profileID, s.customerID, s.profileID)

	template := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "tmpl2",
		Revision:   "r1",
		Title:      "Behavior Template",
		Template:   templateJSON,
	}

	conv1 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_beh1"},
		StartedAt:      currentTime,
	}
	conv2 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_beh2"},
		StartedAt:      currentTime,
	}

	convOutOfTimeRange := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_beh3"},
		StartedAt:      currentTime.Add(-time.Hour * 24),
	}

	scorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_behavior1",
		ConversationID:   conv1.ConversationID.String,
		AgentUserID:      "agent1",
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: currentTime},
	}
	scorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_behavior2",
		ConversationID:   conv2.ConversationID.String,
		AgentUserID:      "agent2",
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: currentTime},
	}
	scorecard3 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_behavior3",
		ConversationID:   convOutOfTimeRange.ConversationID.String,
		AgentUserID:      "agent3",
		TemplateID:       template.ResourceID,
		TemplateRevision: template.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: currentTime.Add(-time.Hour * 24)},
	}

	overwrittenGreetingScore1 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_greeting1",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.90, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.7, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	invalidatedGreetingScore2 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_greeting2",
		ScorecardID:         scorecard2.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.7, Valid: false},
		NotApplicable:       sql.NullBool{Bool: true, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.7, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenEmpathyScore := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_empathy1",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "empathy",
		NumericValue:        sql.NullFloat64{Float64: 0.85, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.5, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	nonOverwrittenEmpathyScore := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_empathy2",
		ScorecardID:         scorecard2.ResourceID,
		CriterionIdentifier: "empathy",
		NumericValue:        sql.NullFloat64{Float64: 0.5, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.5, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenIgnoredBehaviorScore := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_ignoredbehavior1",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "ignoredbehavior",
		NumericValue:        sql.NullFloat64{Float64: 1, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.5, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenEmpathyScoreOutOfTimeRange := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_empathy3",
		ScorecardID:         scorecard3.ResourceID,
		CriterionIdentifier: "empathy",
		NumericValue:        sql.NullFloat64{Float64: 1, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.5, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	s.CreateModels(
		template,
		conv1, conv2, convOutOfTimeRange,
		scorecard1, scorecard2, scorecard3,
		overwrittenGreetingScore1, invalidatedGreetingScore2, nonOverwrittenEmpathyScore, overwrittenEmpathyScore, overwrittenIgnoredBehaviorScore, overwrittenEmpathyScoreOutOfTimeRange)

	behaviorGreeting := &behaviorpb.Behavior{
		Name:        fmt.Sprintf("customers/%s/profiles/%s/behaviors/greeting", s.customerID, s.profileID),
		DisplayName: "Greeting",
	}
	behaviorEmpathy := &behaviorpb.Behavior{
		Name:        fmt.Sprintf("customers/%s/profiles/%s/behaviors/empathy", s.customerID, s.profileID),
		DisplayName: "Empathy",
	}

	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
		FilterByAttribute: &analyticspb.Attribute{
			Behaviors:              []*behaviorpb.Behavior{behaviorGreeting, behaviorEmpathy},
			ScorecardTemplateNames: []string{fmt.Sprintf("customers/%s/profiles/%s/scorecardTemplates/%s@%s", s.customerID, s.profileID, template.ResourceID, template.Revision)},
		},
		GroupByAttributes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR},
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(currentTime.Add(-time.Hour)),
			EndTimestamp:   timestamppb.New(currentTime.Add(time.Hour)),
		},
	})

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(int64(3), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())

	s.Require().Len(resp.CriteriaStats, 2, "Expected 2 behavior stats: 1 for greeting and 1 for empathy")

	var greetingStats, empathyStats *analyticspb.ScorecardCriteriaStats
	for _, stat := range resp.CriteriaStats {
		if len(stat.Attribute.Behaviors) > 0 {
			switch stat.Attribute.Behaviors[0].Name {
			case behaviorGreeting.Name:
				greetingStats = stat
			case behaviorEmpathy.Name:
				empathyStats = stat
			}
		}
	}

	s.Require().NotNil(greetingStats, "Greeting behavior stats should exist")
	s.Equal(int64(2), greetingStats.CriteriaManualOverwriteCount)

	s.Require().NotNil(empathyStats, "Empathy behavior stats should exist")
	s.Equal(int64(1), empathyStats.CriteriaManualOverwriteCount)
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestOnlySubmittedScorecardsAreCounted() {
	// Given
	templateJSON := fmt.Sprintf(`{
		"version": 2,
		"items": [
			{
				"type": "labeled-radios",
				"identifier": "greeting",
				"displayName": "Greeting Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`, s.customerID, s.profileID)

	tmpl := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "tmpl_submitted",
		Revision:   "r1",
		Title:      "Template",
		Template:   templateJSON,
	}

	conv := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_submitted"},
	}

	// Submitted scorecard with overwrite
	submittedScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_submitted",
		ConversationID:   "conv_submitted",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}

	// Non-submitted scorecard with overwrite
	nonSubmittedScorecard := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_not_submitted",
		ConversationID:   "conv_submitted",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: false},
	}

	overwrittenScoreSubmitted := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_submitted",
		ScorecardID:         submittedScorecard.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.90, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	overwrittenScoreNotSubmitted := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_not_submitted",
		ScorecardID:         nonSubmittedScorecard.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.90, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	s.CreateModels(tmpl, conv, submittedScorecard, nonSubmittedScorecard, overwrittenScoreSubmitted, overwrittenScoreNotSubmitted)

	// When
	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
	})

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(int64(1), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())
	s.InDelta(100.0, resp.GetOverallCriteriaStats().GetCriteriaManualOverwritePercent(), 0.01)
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestOverallPercentageCalculation() {
	// Given
	templateJSON := fmt.Sprintf(`{
		"version": 2,
		"items": [
			{
				"type": "labeled-radios",
				"identifier": "greeting",
				"displayName": "Greeting Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`, s.customerID, s.profileID)

	tmpl := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "tmpl",
		Revision:   "r1",
		Title:      "Template",
		Template:   templateJSON,
	}

	conv1 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv1_overwritten"},
	}
	conv2 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv2_overwritten"},
	}
	conv3 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv3_non_overwritten"},
	}
	conv4 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv4_non_overwritten"},
	}
	conv5 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv5_non_overwritten"},
	}

	scorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc1",
		ConversationID:   "conv1_overwritten",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc2",
		ConversationID:   "conv2_overwritten",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard3 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc3",
		ConversationID:   "conv3_non_overwritten",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard4 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc4",
		ConversationID:   "conv4_non_overwritten",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard5 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc5",
		ConversationID:   "conv5_non_overwritten",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}

	overwrittenScore1 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score1",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.90, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenScore2 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score2",
		ScorecardID:         scorecard2.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.80, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.60, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	nonOverwrittenScore3 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score3",
		ScorecardID:         scorecard3.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	nonOverwrittenScore4 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score4",
		ScorecardID:         scorecard4.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	nonOverwrittenScore5 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score5",
		ScorecardID:         scorecard5.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	s.CreateModels(
		tmpl,
		conv1, conv2, conv3, conv4, conv5,
		scorecard1, scorecard2, scorecard3, scorecard4, scorecard5,
		overwrittenScore1, overwrittenScore2,
		nonOverwrittenScore3, nonOverwrittenScore4, nonOverwrittenScore5,
	)

	// When
	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
	})

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(int64(2), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())
	s.InDelta(40.0, resp.GetOverallCriteriaStats().GetCriteriaManualOverwritePercent(), 0.01)
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestBehaviorPercentageCalculation() {
	// Given
	templateJSON := fmt.Sprintf(`{
		"version": 2,
		"items": [
			{
				"type": "labeled-radios",
				"identifier": "greeting",
				"displayName": "Greeting Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			},
			{
				"type": "labeled-radios",
				"identifier": "greeting_tone",
				"displayName": "Greeting Tone",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`, s.customerID, s.profileID, s.customerID, s.profileID)

	tmpl := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "tmpl_behavior",
		Revision:   "r1",
		Title:      "Template",
		Template:   templateJSON,
	}

	conv1 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_beh1"},
	}
	conv2 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_beh2"},
	}
	conv3 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_beh3"},
	}

	scorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_beh1",
		ConversationID:   "conv_beh1",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_beh2",
		ConversationID:   "conv_beh2",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard3 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_beh3",
		ConversationID:   "conv_beh3",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}

	overwrittenGreetingScore1 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_beh_greeting1",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.90, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenGreetingScore2 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_beh_greeting2",
		ScorecardID:         scorecard2.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.85, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.65, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	overwrittenGreetingToneScore := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_beh_tone1",
		ScorecardID:         scorecard2.ResourceID,
		CriterionIdentifier: "greeting_tone",
		NumericValue:        sql.NullFloat64{Float64: 0.80, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.60, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	greetingToneScore1 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_beh1_tone",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "greeting_tone",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	greetingScore3 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_beh3_greeting",
		ScorecardID:         scorecard3.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	greetingToneScore3 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_beh3_tone",
		ScorecardID:         scorecard3.ResourceID,
		CriterionIdentifier: "greeting_tone",
		NumericValue:        sql.NullFloat64{Float64: 0.70, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	s.CreateModels(
		tmpl,
		conv1, conv2, conv3,
		scorecard1, scorecard2, scorecard3,
		overwrittenGreetingScore1, overwrittenGreetingScore2, overwrittenGreetingToneScore,
		greetingToneScore1, greetingScore3, greetingToneScore3,
	)

	behaviorGreeting := &behaviorpb.Behavior{
		Name:        fmt.Sprintf("customers/%s/profiles/%s/behaviors/greeting", s.customerID, s.profileID),
		DisplayName: "Greeting",
	}

	// When
	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
		FilterByAttribute: &analyticspb.Attribute{
			Behaviors:              []*behaviorpb.Behavior{behaviorGreeting},
			ScorecardTemplateNames: []string{fmt.Sprintf("customers/%s/profiles/%s/scorecardTemplates/%s@%s", s.customerID, s.profileID, tmpl.ResourceID, tmpl.Revision)},
		},
		GroupByAttributes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR},
	})

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().Len(resp.CriteriaStats, 1)

	s.Equal(int64(3), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())
	s.InDelta(50.0, resp.GetOverallCriteriaStats().GetCriteriaManualOverwritePercent(), 0.01)

	greetingStats := resp.CriteriaStats[0]
	s.Require().NotNil(greetingStats)
	s.Equal(int64(3), greetingStats.CriteriaManualOverwriteCount)
	s.InDelta(50.0, greetingStats.CriteriaManualOverwritePercent, 0.01)
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestPercentageWithNoScorecards() {
	// Given - no scorecards created

	// When
	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
	})

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(int64(0), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())
	s.Equal(float64(0), resp.GetOverallCriteriaStats().GetCriteriaManualOverwritePercent())
}

func (s *RetrieveScorecardCriteriaStatsTestSuite) TestPercentageWithAllOverwritten() {
	// Given
	templateJSON := fmt.Sprintf(`{
		"version": 2,
		"items": [
			{
				"type": "labeled-radios",
				"identifier": "greeting",
				"displayName": "Greeting Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/greeting"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			},
			{
				"type": "labeled-radios",
				"identifier": "empathy",
				"displayName": "Empathy Quality",
				"weight": 1,
				"settings": {
					"options": [
						{"label": "Good", "value": 1},
						{"label": "Poor", "value": 0}
					]
				},
				"auto_qa": {
					"triggers": [
						{
							"type": "behavior",
							"resource_name": "customers/%s/profiles/%s/behaviors/empathy"
						}
					],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`, s.customerID, s.profileID, s.customerID, s.profileID)

	tmpl := &dbmodel.ScorecardTemplates{
		Customer:   s.customerID,
		Profile:    s.profileID,
		ResourceID: "tmpl_all_overwritten",
		Revision:   "r1",
		Title:      "Template",
		Template:   templateJSON,
	}

	conv1 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_all1"},
	}
	conv2 := &dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: s.customerID},
		ProfileID:      sql.NullString{Valid: true, String: s.profileID},
		ConversationID: sql.NullString{Valid: true, String: "conv_all2"},
	}

	scorecard1 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_all1",
		ConversationID:   "conv_all1",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}
	scorecard2 := &dbmodel.Scorecards{
		Customer:         s.customerID,
		Profile:          s.profileID,
		ResourceID:       "sc_all2",
		ConversationID:   "conv_all2",
		AgentUserID:      "agent1",
		TemplateID:       tmpl.ResourceID,
		TemplateRevision: tmpl.Revision,
		ManuallyScored:   sql.NullBool{Bool: true, Valid: true},
		SubmittedAt:      sql.NullTime{Valid: true, Time: time.Now()},
	}

	overwrittenScore1 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_all1",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.90, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.70, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenScore2 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_all2",
		ScorecardID:         scorecard2.ResourceID,
		CriterionIdentifier: "greeting",
		NumericValue:        sql.NullFloat64{Float64: 0.80, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.60, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}
	overwrittenScore3 := &dbmodel.Scores{
		Customer:            s.customerID,
		Profile:             s.profileID,
		ResourceID:          "score_all3",
		ScorecardID:         scorecard1.ResourceID,
		CriterionIdentifier: "empathy",
		NumericValue:        sql.NullFloat64{Float64: 0.85, Valid: true},
		AiValue:             sql.NullFloat64{Float64: 0.75, Valid: true},
		AiScored:            sql.NullBool{Bool: true, Valid: true},
	}

	s.CreateModels(tmpl, conv1, conv2, scorecard1, scorecard2, overwrittenScore1, overwrittenScore2, overwrittenScore3)

	// When
	resp, err := s.server.RetrieveScorecardCriteriaStats(s.ctx, &analyticspb.RetrieveScorecardCriteriaStatsRequest{
		Parent: s.profileResourceName,
	})

	// Then
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(int64(3), resp.GetOverallCriteriaStats().GetCriteriaManualOverwriteCount())
	s.InDelta(100.0, resp.GetOverallCriteriaStats().GetCriteriaManualOverwritePercent(), 0.01)
}

func TestRetrieveScorecardCriteriaStats(t *testing.T) {
	suite.Run(t, &RetrieveScorecardCriteriaStatsTestSuite{})
}
