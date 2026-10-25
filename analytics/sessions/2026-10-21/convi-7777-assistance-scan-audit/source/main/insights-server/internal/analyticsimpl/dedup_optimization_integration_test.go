package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared/descriptors"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type DedupOptimizationCHTestSuite struct {
	AnalyticsTestSuite
	dates *TimePeriodData
}

func (r *DedupOptimizationCHTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.dates = r.SetupTimePeriodData()
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

type scoreSeed struct {
	agentUserID     string
	scorecardID     string
	conversationID  string
	criterionID     string
	percentageValue float64
	floatWeight     float64
	// notApplicable marks the criterion score as N/A. N/A rows store
	// percentageValue=-1 and not_applicable=true, and are excluded from stats
	// unless the request opts into IncludeNaScored.
	notApplicable bool
	// Scorecard-level fields for dedup/status filtering
	scorecardLastUpdateTime time.Time
	scorecardSubmitTime     time.Time
	manuallyScored          bool
}

func (r *DedupOptimizationCHTestSuite) seedData(templateID string, seeds []scoreSeed) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	now := time.Now()
	scorecardTime := time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC)

	// Collect unique scorecards
	type scKey struct{ id string }
	seenScorecards := map[scKey]bool{}

	// Insert scorecards (deduplicated by scorecardID + lastUpdateTime)
	scorecardBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO scorecard (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScorecardRow{})))
	r.Require().NoError(err)

	for _, s := range seeds {
		key := scKey{id: s.scorecardID + s.scorecardLastUpdateTime.String()}
		if seenScorecards[key] {
			continue
		}
		seenScorecards[key] = true

		r.Require().NoError(scorecardBatch.AppendStruct(&clickhouseconv.ScorecardRow{
			AgentUserID:             s.agentUserID,
			ScorecardID:             s.scorecardID,
			ScorecardTemplateID:     templateID,
			ScorecardTime:           scorecardTime,
			ScorecardLastUpdateTime: s.scorecardLastUpdateTime,
			ScorecardSubmitTime:     s.scorecardSubmitTime,
			ManuallyScored:          s.manuallyScored,
			ConversationID:          s.conversationID,
			CustomerID:              r.customerID,
			ProfileID:               r.profileID,
			UpdateTime:              now,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scorecardBatch))

	// Insert scores
	scoreBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO score (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScoreRow{})))
	r.Require().NoError(err)

	for _, s := range seeds {
		r.Require().NoError(scoreBatch.AppendStruct(&clickhouseconv.ScoreRow{
			AgentUserID:             s.agentUserID,
			ScorecardID:             s.scorecardID,
			ScorecardTemplateID:     templateID,
			ScorecardTime:           scorecardTime,
			ScorecardLastUpdateTime: s.scorecardLastUpdateTime,
			ScorecardSubmitTime:     s.scorecardSubmitTime,
			ManuallyScored:          s.manuallyScored,
			ConversationID:          s.conversationID,
			CriterionID:             s.criterionID,
			PercentageValue:         s.percentageValue,
			NotApplicable:           s.notApplicable,
			FloatWeight:             s.floatWeight,
			CustomerID:              r.customerID,
			ProfileID:               r.profileID,
			ScoreID:                 uuid.NewString(),
			UpdateTime:              now,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scoreBatch))
}

// seedConversationChat inserts the Postgres chats row that
// convertCHResponseToQaConversationsResponse joins ClickHouse results against. A
// conversation returned by the CH query but missing from this table is dropped as
// "not in DB", so without this seed the RetrieveQAConversations regression
// assertion would pass vacuously even if the all-N/A dedup were broken.
func (r *DedupOptimizationCHTestSuite) seedConversationChat(conversationID string) {
	r.Require().NoError(r.DB().Create(&dbmodel.Chats{
		CustomerID:     sql.NullString{Valid: true, String: r.customerID},
		ProfileID:      sql.NullString{Valid: true, String: r.profileID},
		ConversationID: sql.NullString{Valid: true, String: conversationID},
		StartedAt:      time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC),
	}).Error)
}

func (r *DedupOptimizationCHTestSuite) buildRequest(templateID string, opts ...func(*analyticspb.RetrieveQAScoreStatsRequest)) *analyticspb.RetrieveQAScoreStatsRequest {
	req := &analyticspb.RetrieveQAScoreStatsRequest{
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
					TemplateID: templateID,
				}.String(),
			},
		},
		Metadata: &r.dates.metadataLA,
	}
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// TestDedupOptimization_GroupByAgent_BasicScoring verifies the optimized query
// (without scorecard_d scan) returns correct per-agent scores from real ClickHouse.
func (r *DedupOptimizationCHTestSuite) TestDedupOptimization_GroupByAgent_BasicScoring() {
	// Given
	templateID := "dedup-basic-" + uuid.NewString()[:8]
	updateTime := time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 13, 0, 0, 0, time.UTC)

	r.seedData(templateID, []scoreSeed{
		{agentUserID: "dedup_agent1", scorecardID: uuid.NewString(), conversationID: uuid.NewString(), criterionID: "c1", percentageValue: 0.80, floatWeight: 1.0, scorecardLastUpdateTime: updateTime, scorecardSubmitTime: submitTime},
		{agentUserID: "dedup_agent1", scorecardID: uuid.NewString(), conversationID: uuid.NewString(), criterionID: "c1", percentageValue: 0.60, floatWeight: 1.0, scorecardLastUpdateTime: updateTime, scorecardSubmitTime: submitTime},
		{agentUserID: "dedup_agent2", scorecardID: uuid.NewString(), conversationID: uuid.NewString(), criterionID: "c1", percentageValue: 0.90, floatWeight: 1.0, scorecardLastUpdateTime: updateTime, scorecardSubmitTime: submitTime},
	})

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.buildRequest(templateID))

	// Then
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)
	r.Require().Len(resp.QaScoreResult.Scores, 2, "expected 2 agents")

	agentScores := map[string]float32{}
	agentConvCounts := map[string]int32{}
	for _, s := range resp.QaScoreResult.Scores {
		agentScores[s.GroupedBy.User.Name] = s.Score
		agentConvCounts[s.GroupedBy.User.Name] = s.TotalConversationCount
	}

	agent1Name := fmt.Sprintf("customers/%s/users/dedup_agent1", r.customerID)
	agent2Name := fmt.Sprintf("customers/%s/users/dedup_agent2", r.customerID)

	r.InDelta(0.70, agentScores[agent1Name], 0.01, "agent1 avg score should be (0.80+0.60)/2 = 0.70")
	r.InDelta(0.90, agentScores[agent2Name], 0.01, "agent2 score should be 0.90")
	r.Equal(int32(2), agentConvCounts[agent1Name], "agent1 should have 2 conversations")
	r.Equal(int32(1), agentConvCounts[agent2Name], "agent2 should have 1 conversation")
	r.Equal(int32(3), resp.QaScoreResult.TotalConversationCount)
}

// TestDedupOptimization_ScorecardDedup verifies that only the latest version of
// a scorecard is used, which is the core correctness property the optimization must preserve.
func (r *DedupOptimizationCHTestSuite) TestDedupOptimization_ScorecardDedup() {
	// Given
	templateID := "dedup-version-" + uuid.NewString()[:8]
	scorecardID := uuid.NewString()
	conversationID := uuid.NewString()
	olderVersion := time.Date(2021, 1, 5, 10, 0, 0, 0, time.UTC)
	newerVersion := time.Date(2021, 1, 5, 14, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 15, 0, 0, 0, time.UTC)

	r.seedData(templateID, []scoreSeed{
		// Older version: score = 0.30
		{agentUserID: "dedup_v_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: 0.30, floatWeight: 1.0, scorecardLastUpdateTime: olderVersion, scorecardSubmitTime: submitTime},
		// Newer version: score = 0.90
		{agentUserID: "dedup_v_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: 0.90, floatWeight: 1.0, scorecardLastUpdateTime: newerVersion, scorecardSubmitTime: submitTime},
	})

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.buildRequest(templateID))

	// Then
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)

	agentName := fmt.Sprintf("customers/%s/users/dedup_v_agent", r.customerID)
	var agentScore float32
	for _, s := range resp.QaScoreResult.Scores {
		if s.GroupedBy.User.Name == agentName {
			agentScore = s.Score
		}
	}
	r.InDelta(0.90, agentScore, 0.01, "should use newer version score (0.90), not older (0.30)")
	r.Equal(int32(1), resp.QaScoreResult.TotalScorecardCount, "two versions of same scorecard should count as 1")
}

// TestDedupOptimization_LatestVersionAllNA reproduces the PR #27274 regression
// reported by AAA Life: an original "No" score that a reviewer later overwrites
// with all-N/A keeps showing the stale "No" as a false 0% exception.
//
// The dedup that picks the latest scorecard version must be computed over rows
// that are NOT N/A-filtered. With the default IncludeNaScored=false, the
// scorecard_score CTE is filtered to `percentage_value >= 0 AND not_applicable
// != true`, which drops the newer all-N/A version entirely. If the dedup's
// max(scorecard_last_update_time) is taken from that filtered CTE, it never sees
// the newer version and falls back to the older "No" row. The correct behavior
// is that the latest (all-N/A) version wins the dedup and, since every criterion
// is N/A and N/A is excluded, the scorecard contributes nothing — no 0% score.
func (r *DedupOptimizationCHTestSuite) TestDedupOptimization_LatestVersionAllNA() {
	// Given
	templateID := "dedup-na-" + uuid.NewString()[:8]
	scorecardID := uuid.NewString()
	conversationID := uuid.NewString()
	olderVersion := time.Date(2021, 1, 5, 10, 0, 0, 0, time.UTC)
	newerVersion := time.Date(2021, 1, 5, 14, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 15, 0, 0, 0, time.UTC)

	r.seedData(templateID, []scoreSeed{
		// Older version: scored "No" (percentage_value = 0).
		{agentUserID: "dedup_na_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: 0.0, floatWeight: 1.0, scorecardLastUpdateTime: olderVersion, scorecardSubmitTime: submitTime, manuallyScored: true},
		// Newer version: reviewer overwrote the criterion to N/A.
		{agentUserID: "dedup_na_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: -1.0, notApplicable: true, floatWeight: 1.0, scorecardLastUpdateTime: newerVersion, scorecardSubmitTime: submitTime, manuallyScored: true},
	})

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.buildRequest(templateID))

	// Then
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)

	agentName := fmt.Sprintf("customers/%s/users/dedup_na_agent", r.customerID)
	for _, s := range resp.QaScoreResult.Scores {
		r.NotEqual(agentName, s.GroupedBy.User.Name,
			"latest version is all-N/A and N/A is excluded, so no 0%% score should surface for the agent")
	}
	r.Equal(int32(0), resp.QaScoreResult.TotalScorecardCount,
		"the only scorecard's latest version is all-N/A, so it must not count")
	r.Equal(int32(0), resp.QaScoreResult.TotalConversationCount,
		"no scored conversations should remain once the all-N/A overwrite wins the dedup")
}

// TestDedupOptimization_LatestVersionAllNA_QAConversations is the
// RetrieveQAConversations counterpart to TestDedupOptimization_LatestVersionAllNA.
// RetrieveQAConversations is the RPC that renders the per-conversation 0% exception
// AAA Life reported, so the all-N/A dedup fix must hold here too: once the latest
// scorecard version is all-N/A, the default (IncludeNaScored=false) path must surface
// no conversation row for that scorecard.
func (r *DedupOptimizationCHTestSuite) TestDedupOptimization_LatestVersionAllNA_QAConversations() {
	// Given
	templateID := "dedup-na-conv-" + uuid.NewString()[:8]
	scorecardID := uuid.NewString()
	conversationID := uuid.NewString()
	olderVersion := time.Date(2021, 1, 5, 10, 0, 0, 0, time.UTC)
	newerVersion := time.Date(2021, 1, 5, 14, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 15, 0, 0, 0, time.UTC)

	r.seedData(templateID, []scoreSeed{
		// Older version: scored "No" (percentage_value = 0).
		{agentUserID: "dedup_na_conv_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: 0.0, floatWeight: 1.0, scorecardLastUpdateTime: olderVersion, scorecardSubmitTime: submitTime, manuallyScored: true},
		// Newer version: reviewer overwrote the criterion to N/A.
		{agentUserID: "dedup_na_conv_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: -1.0, notApplicable: true, floatWeight: 1.0, scorecardLastUpdateTime: newerVersion, scorecardSubmitTime: submitTime, manuallyScored: true},
	})
	// The conversation must exist in Postgres or the CH result is dropped before the
	// assertion can observe the regression.
	r.seedConversationChat(conversationID)

	// When
	resp, err := r.server.RetrieveQAConversations(r.ctx, &analyticspb.RetrieveQAConversationsRequest{
		Parent:            r.profileResourceName,
		FilterByTimeRange: &r.dates.janPeriod,
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{
				descriptors.ScorecardTemplateName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					TemplateID: templateID,
				}.String(),
			},
			// Set explicitly so RetrieveQAConversations does not call getScoreableCriteria.
			CriterionIdentifiers: []string{"c1"},
		},
		Metadata: &r.dates.metadataLA,
	})

	// Then
	r.Require().NoError(err)
	r.Empty(resp.ConversationInfos,
		"latest version is all-N/A and N/A is excluded, so no conversation exception should surface")
}

// TestDedupOptimization_LatestVersionAllNA_IncludeNaScored is the opt-in counterpart:
// when the caller sets IncludeNaScored=true, the all-N/A latest version is no longer
// filtered out, so the scorecard surfaces and is counted. The dedup still picks the
// latest version from scorecard_d, so this asserts the IncludeNaScored path stays at
// one scorecard while the default path stays at zero (TestDedupOptimization_LatestVersionAllNA).
func (r *DedupOptimizationCHTestSuite) TestDedupOptimization_LatestVersionAllNA_IncludeNaScored() {
	// Given
	templateID := "dedup-na-incl-" + uuid.NewString()[:8]
	scorecardID := uuid.NewString()
	conversationID := uuid.NewString()
	olderVersion := time.Date(2021, 1, 5, 10, 0, 0, 0, time.UTC)
	newerVersion := time.Date(2021, 1, 5, 14, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 15, 0, 0, 0, time.UTC)

	r.seedData(templateID, []scoreSeed{
		// Older version: scored "No" (percentage_value = 0).
		{agentUserID: "dedup_na_incl_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: 0.0, floatWeight: 1.0, scorecardLastUpdateTime: olderVersion, scorecardSubmitTime: submitTime, manuallyScored: true},
		// Newer version: reviewer overwrote the criterion to N/A.
		{agentUserID: "dedup_na_incl_agent", scorecardID: scorecardID, conversationID: conversationID, criterionID: "c1", percentageValue: -1.0, notApplicable: true, floatWeight: 1.0, scorecardLastUpdateTime: newerVersion, scorecardSubmitTime: submitTime, manuallyScored: true},
	})

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.buildRequest(templateID, func(req *analyticspb.RetrieveQAScoreStatsRequest) {
		req.FilterByAttribute.IncludeNaScored = true
	}))

	// Then
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)
	r.Equal(int32(1), resp.QaScoreResult.TotalScorecardCount,
		"with IncludeNaScored=true the all-N/A latest version is counted as one scorecard")
}

// TestDedupOptimization_ScorecardStatusFilter verifies that scorecard status
// filtering (MANUALLY_SUBMITTED, DRAFT, AUTO) works correctly when derived
// from score_d instead of scorecard_d.
func (r *DedupOptimizationCHTestSuite) TestDedupOptimization_ScorecardStatusFilter() {
	// Given
	templateID := "dedup-status-" + uuid.NewString()[:8]
	updateTime := time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 13, 0, 0, 0, time.UTC)
	zeroTime := time.Time{}

	r.seedData(templateID, []scoreSeed{
		// Submitted scorecard (scorecard_submit_time != 0)
		{agentUserID: "dedup_status_submitted", scorecardID: uuid.NewString(), conversationID: uuid.NewString(), criterionID: "c1", percentageValue: 0.80, floatWeight: 1.0, scorecardLastUpdateTime: updateTime, scorecardSubmitTime: submitTime, manuallyScored: true},
		// Draft scorecard (scorecard_submit_time = 0, manually_scored = true)
		{agentUserID: "dedup_status_draft", scorecardID: uuid.NewString(), conversationID: uuid.NewString(), criterionID: "c1", percentageValue: 0.50, floatWeight: 1.0, scorecardLastUpdateTime: updateTime, scorecardSubmitTime: zeroTime, manuallyScored: true},
		// Auto scorecard (scorecard_submit_time = 0, manually_scored = false)
		{agentUserID: "dedup_status_auto", scorecardID: uuid.NewString(), conversationID: uuid.NewString(), criterionID: "c1", percentageValue: 0.60, floatWeight: 1.0, scorecardLastUpdateTime: updateTime, scorecardSubmitTime: zeroTime, manuallyScored: false},
	})

	// When — filter to only MANUALLY_SUBMITTED
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.buildRequest(templateID, func(req *analyticspb.RetrieveQAScoreStatsRequest) {
		req.FilterByAttribute.ScorecardStatuses = []analyticspb.QAAttribute_ScorecardStatus{
			analyticspb.QAAttribute_MANUALLY_SUBMITTED,
		}
	}))

	// Then
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)

	agentNames := map[string]bool{}
	for _, s := range resp.QaScoreResult.Scores {
		agentNames[s.GroupedBy.User.Name] = true
	}

	submittedName := fmt.Sprintf("customers/%s/users/dedup_status_submitted", r.customerID)
	draftName := fmt.Sprintf("customers/%s/users/dedup_status_draft", r.customerID)
	autoName := fmt.Sprintf("customers/%s/users/dedup_status_auto", r.customerID)

	r.True(agentNames[submittedName], "submitted scorecard should be included")
	r.False(agentNames[draftName], "draft scorecard should be excluded")
	r.False(agentNames[autoName], "auto scorecard should be excluded")
	r.Equal(int32(1), resp.QaScoreResult.TotalScorecardCount, "only 1 submitted scorecard")
}

func TestDedupOptimizationCH(t *testing.T) {
	suite.Run(t, &DedupOptimizationCHTestSuite{})
}
