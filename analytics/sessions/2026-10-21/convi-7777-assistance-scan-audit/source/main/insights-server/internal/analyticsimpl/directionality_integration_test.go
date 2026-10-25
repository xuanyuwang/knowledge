package analyticsimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protojson"

	dbcommonpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/common"
	dbmomentpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/moment"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared/descriptors"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type DirectionalityCHTestSuite struct {
	AnalyticsTestSuite
	dates *TimePeriodData
}

func (r *DirectionalityCHTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.server.enableDirectionality = true
	r.dates = r.SetupTimePeriodData()
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

type agentScoreData struct {
	agentUserID     string
	percentageValue float64
}

// seedScoringData inserts ScorecardRow and ScoreRow records into real ClickHouse.
// Each agent gets one scorecard with one criterion score.
func (r *DirectionalityCHTestSuite) seedScoringData(templateID, criterionID string, agents []agentScoreData) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	now := time.Now()
	scorecardTime := time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC)

	// Insert scorecards
	scorecardBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO scorecard (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScorecardRow{})))
	r.Require().NoError(err)

	type scorecardMeta struct {
		scorecardID    string
		conversationID string
	}
	metas := make([]scorecardMeta, len(agents))

	for i, a := range agents {
		scID := uuid.NewString()
		convID := uuid.NewString()
		metas[i] = scorecardMeta{scorecardID: scID, conversationID: convID}

		r.Require().NoError(scorecardBatch.AppendStruct(&clickhouseconv.ScorecardRow{
			AgentUserID:             a.agentUserID,
			ScorecardID:             scID,
			ScorecardTemplateID:     templateID,
			ScorecardTime:           scorecardTime,
			ScorecardLastUpdateTime: scorecardTime,
			ConversationID:          convID,
			CustomerID:              r.customerID,
			ProfileID:               r.profileID,
			Score:                   a.percentageValue * 100,
			UpdateTime:              now,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scorecardBatch))

	// Insert scores
	scoreBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO score (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScoreRow{})))
	r.Require().NoError(err)

	for i, a := range agents {
		r.Require().NoError(scoreBatch.AppendStruct(&clickhouseconv.ScoreRow{
			AgentUserID:             a.agentUserID,
			ScorecardID:             metas[i].scorecardID,
			ScorecardTemplateID:     templateID,
			ScorecardTime:           scorecardTime,
			ScorecardLastUpdateTime: scorecardTime,
			ConversationID:          metas[i].conversationID,
			CriterionID:             criterionID,
			PercentageValue:         a.percentageValue,
			FloatWeight:             1.0,
			CustomerID:              r.customerID,
			ProfileID:               r.profileID,
			ScoreID:                 uuid.NewString(),
			UpdateTime:              now,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scoreBatch))
}

// TestDirectionality_GroupByAgent_QuintileCorrectness verifies the full pipeline:
// DB records → directionality resolution → CH query with ranking CASE expression
// → quintile assignment using CH-computed ranking scores.
func (r *DirectionalityCHTestSuite) TestDirectionality_GroupByAgent_QuintileCorrectness() {
	templateResourceID := "dir-ch-quintile"
	momentID := "dir-ch-quintile-moment"
	criterionID := "aht-criterion"
	momentResourceName := fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, momentID)

	templateJSON := buildDirectionalityTemplateJSON(r.T(), criterionID, momentResourceName)
	momentPayload := buildMomentPayloadJSON(r.T(), dbcommonpb.OutcomeType_TIME)

	cleanup := r.CreateModels(
		&dbmodel.ScorecardTemplates{
			Customer:   r.customerID,
			Profile:    r.profileID,
			ResourceID: templateResourceID,
			Revision:   "r1",
			Template:   templateJSON,
			Title:      "Dir CH Quintile",
			CreatedAt:  time.Now(),
		},
		&dbmodel.MomentTemplates{
			CustomerID:       r.customerID,
			ProfileID:        r.profileID,
			MomentTemplateID: momentID,
			DisplayName:      "AHT",
			Payload:          momentPayload,
		},
	)
	defer cleanup()

	r.seedScoringData(templateResourceID, criterionID, []agentScoreData{
		{agentUserID: "q_agent1", percentageValue: 0.10},
		{agentUserID: "q_agent2", percentageValue: 0.20},
		{agentUserID: "q_agent3", percentageValue: 0.30},
		{agentUserID: "q_agent4", percentageValue: 0.40},
		{agentUserID: "q_agent5", percentageValue: 0.50},
	})

	resp, err := r.server.RetrieveQAScoreStats(r.ctx, &analyticspb.RetrieveQAScoreStatsRequest{
		Parent:            r.profileResourceName,
		FilterByTimeRange: &r.dates.janPeriod,
		GroupByAttributeTypes: []analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
		},
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{
				descriptors.ScorecardTemplateName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					TemplateID: templateResourceID,
				}.String(),
			},
		},
		Metadata: &r.dates.metadataLA,
	})

	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)
	r.Require().Len(resp.QaScoreResult.Scores, 5)

	// agent with lowest AHT (0.10) → ranking = -0.10 (highest) → Q1
	// agent with highest AHT (0.50) → ranking = -0.50 (lowest) → Q5
	wantQuintiles := map[string]analyticspb.QuintileRank{
		"customers/customer_id/users/q_agent1": analyticspb.QuintileRank_QUINTILE_RANK_1,
		"customers/customer_id/users/q_agent2": analyticspb.QuintileRank_QUINTILE_RANK_2,
		"customers/customer_id/users/q_agent3": analyticspb.QuintileRank_QUINTILE_RANK_3,
		"customers/customer_id/users/q_agent4": analyticspb.QuintileRank_QUINTILE_RANK_4,
		"customers/customer_id/users/q_agent5": analyticspb.QuintileRank_QUINTILE_RANK_5,
	}
	for _, s := range resp.QaScoreResult.Scores {
		userName := s.GroupedBy.User.Name
		wantRank, ok := wantQuintiles[userName]
		r.True(ok, "unexpected user %s", userName)
		r.Equal(wantRank, s.GroupedBy.QuintileRank,
			"agent %s (display=%.2f) should be %v", userName, s.Score, wantRank)
		r.True(s.Score >= 0, "display score should not be negative: %s = %.2f", userName, s.Score)
	}
}

// TestDirectionality_GroupByAgentTier_TierCorrectness verifies that tier
// partitioning uses CH-computed ranking scores when lower-is-better criteria exist.
func (r *DirectionalityCHTestSuite) TestDirectionality_GroupByAgentTier_TierCorrectness() {
	templateResourceID := "dir-ch-tier"
	momentID := "dir-ch-tier-moment"
	criterionID := "aht-criterion-tier"
	momentResourceName := fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, momentID)

	templateJSON := buildDirectionalityTemplateJSON(r.T(), criterionID, momentResourceName)
	momentPayload := buildMomentPayloadJSON(r.T(), dbcommonpb.OutcomeType_TIME)

	cleanup := r.CreateModels(
		&dbmodel.ScorecardTemplates{
			Customer:   r.customerID,
			Profile:    r.profileID,
			ResourceID: templateResourceID,
			Revision:   "r1",
			Template:   templateJSON,
			Title:      "Dir CH Tier",
			CreatedAt:  time.Now(),
		},
		&dbmodel.MomentTemplates{
			CustomerID:       r.customerID,
			ProfileID:        r.profileID,
			MomentTemplateID: momentID,
			DisplayName:      "AHT Tier",
			Payload:          momentPayload,
		},
	)
	defer cleanup()

	r.seedScoringData(templateResourceID, criterionID, []agentScoreData{
		{agentUserID: "t_agent1", percentageValue: 0.10},
		{agentUserID: "t_agent2", percentageValue: 0.20},
		{agentUserID: "t_agent3", percentageValue: 0.30},
		{agentUserID: "t_agent4", percentageValue: 0.40},
		{agentUserID: "t_agent5", percentageValue: 0.50},
		{agentUserID: "t_agent6", percentageValue: 0.60},
		{agentUserID: "t_agent7", percentageValue: 0.70},
		{agentUserID: "t_agent8", percentageValue: 0.80},
	})

	resp, err := r.server.RetrieveQAScoreStats(r.ctx, &analyticspb.RetrieveQAScoreStatsRequest{
		Parent:            r.profileResourceName,
		FilterByTimeRange: &r.dates.janPeriod,
		GroupByAttributeTypes: []analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER,
		},
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{
				descriptors.ScorecardTemplateName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					TemplateID: templateResourceID,
				}.String(),
			},
		},
		Metadata: &r.dates.metadataLA,
	})

	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)
	r.Require().Len(resp.QaScoreResult.Scores, 3, "expected 3 tiers")

	tierScores := make(map[analyticspb.AgentTier]float32)
	for _, s := range resp.QaScoreResult.Scores {
		tierScores[s.GroupedBy.AgentTier] = s.Score
	}

	// TOP tier agents have the lowest AHT → lowest display score.
	r.Less(tierScores[analyticspb.AgentTier_TOP_AGENTS], tierScores[analyticspb.AgentTier_BOTTOM_AGENTS],
		"TOP tier (%.2f) should have lower AHT than BOTTOM tier (%.2f)",
		tierScores[analyticspb.AgentTier_TOP_AGENTS], tierScores[analyticspb.AgentTier_BOTTOM_AGENTS])
}

func buildDirectionalityTemplateJSON(t testing.TB, criterionID, momentResourceName string) string {
	t.Helper()
	tmpl := map[string]any{
		"version": 1,
		"criteria": []any{
			map[string]any{
				"type":        "labeled-radios",
				"weight":      1,
				"identifier":  criterionID,
				"displayName": criterionID,
				"auto_qa": map[string]any{
					"triggers": []map[string]any{
						{
							"type":          "metadata",
							"resource_name": momentResourceName,
						},
					},
				},
				"settings": map[string]any{
					"options": []any{
						map[string]any{"label": "Yes", "value": 1},
						map[string]any{"label": "No", "value": 0},
					},
				},
			},
		},
	}
	data, err := json.Marshal(tmpl)
	require.NoError(t, err)
	return string(data)
}

func buildMomentPayloadJSON(t testing.TB, outcomeType dbcommonpb.OutcomeType) string {
	t.Helper()
	momentData := &dbmomentpb.MomentData{
		Moment: &dbmomentpb.MomentData_ConversationOutcomeMomentData{
			ConversationOutcomeMomentData: &dbmomentpb.ConversationOutcomeMomentData{
				Type: outcomeType,
			},
		},
	}
	data, err := protojson.Marshal(momentData)
	require.NoError(t, err)
	return string(data)
}

func TestDirectionalityCH(t *testing.T) {
	suite.Run(t, &DirectionalityCHTestSuite{})
}
