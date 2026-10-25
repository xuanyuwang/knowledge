package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// QaScoreStatsRespectConfigCHTestSuite exercises RetrieveQAScoreStats with
// RespectTemplateQaScoreConfig=true against a real ClickHouse container. That path
// (readQaScoreStatsRespectingTemplateQaScoreConfig) groups scorecard templates by their conversation
// duration filter, runs one ClickHouse query per group in parallel, and merges the results. Templates
// with no duration filter — including NULL or empty ("{}") QaScoreConfig — collapse into a single
// combined `scorecard_template_id IN (...)` query; templates sharing a duration filter form their own
// groups. The tests seed a mix and assert both the group count and each group's scores.
type QaScoreStatsRespectConfigCHTestSuite struct {
	AnalyticsTestSuite
	dates *TimePeriodData
}

func (r *QaScoreStatsRespectConfigCHTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.dates = r.SetupTimePeriodData()
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

// respectConfigSeed is one scored conversation's worth of data: a single scorecard scored for one
// criterion (persisted to ClickHouse) attributed to a scorecard template. The template itself is
// persisted to Postgres separately (see seedTemplateWithEmptyQaScoreConfig /
// seedTemplateWithoutQaScoreConfig / seedTemplateWithDurationFilter) so that the template's
// QaScoreConfig can vary between tests.
type respectConfigSeed struct {
	templateID      string
	criterionID     string
	agentUserID     string
	conversationID  string
	percentageValue float64
	// conversationDurationSecs is the conversation's duration in ClickHouse. It matters only for
	// templates whose QaScoreConfig carries a conversation duration-bucket filter, where the per-group
	// query restricts rows to this column's value falling within the bucket bounds.
	conversationDurationSecs int32
}

// seedTemplate persists a scorecard template to Postgres with the given QaScoreConfig.
func (r *QaScoreStatsRespectConfigCHTestSuite) seedTemplate(templateID string, qaScoreConfig sql.NullString) {
	template := &dbmodel.ScorecardTemplates{
		Customer:      r.customerID,
		Profile:       r.profileID,
		ResourceID:    templateID,
		Revision:      "latest",
		CreatedAt:     time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Title:         "template " + templateID,
		Template:      "{}",
		QaScoreConfig: qaScoreConfig,
	}
	r.Require().NoError(r.DB().Create(template).Error)
}

// seedTemplateWithEmptyQaScoreConfig persists a template whose QaScoreConfig is non-null but
// content-empty ("{}"): the column is .Valid yet carries no duration filter, so it joins the no-filter group.
func (r *QaScoreStatsRespectConfigCHTestSuite) seedTemplateWithEmptyQaScoreConfig(templateID string) {
	r.seedTemplate(templateID, sql.NullString{String: "{}", Valid: true})
}

// seedTemplateWithoutQaScoreConfig persists a template whose QaScoreConfig column is NULL. Like an
// empty config it carries no duration filter, so it joins the same no-filter group as empty-config templates.
func (r *QaScoreStatsRespectConfigCHTestSuite) seedTemplateWithoutQaScoreConfig(templateID string) {
	r.seedTemplate(templateID, sql.NullString{})
}

// seedTemplateWithDurationFilter persists a template whose QaScoreConfig carries a duration-bucket
// filter for the given bucket ID (see hardcodedConversationDurationBuckets). Templates sharing a bucket
// ID share a group and one combined query.
func (r *QaScoreStatsRespectConfigCHTestSuite) seedTemplateWithDurationFilter(templateID, durationBucketID string) {
	config := fmt.Sprintf(`{"conversationFilters": {"conversationDurationBuckets": [{"id": %q}]}}`, durationBucketID)
	r.seedTemplate(templateID, sql.NullString{String: config, Valid: true})
}

// seedScores persists scorecard + score rows to ClickHouse for the given seeds. Each seed contributes
// one scorecard (one conversation) and one criterion score.
func (r *QaScoreStatsRespectConfigCHTestSuite) seedScores(seeds []respectConfigSeed) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	now := time.Now()
	scorecardTime := time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC)
	submitTime := time.Date(2021, 1, 5, 13, 0, 0, 0, time.UTC)

	scorecardBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO scorecard (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScorecardRow{})))
	r.Require().NoError(err)
	scoreBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO score (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScoreRow{})))
	r.Require().NoError(err)
	// The duration-bucket filter is applied against the conversation table (joined on conversation_id),
	// so seed a matching conversation row per seed carrying the same conversation_duration_secs.
	conversationBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	r.Require().NoError(err)

	for _, seed := range seeds {
		scorecardID := uuid.NewString()
		r.Require().NoError(conversationBatch.AppendStruct(&clickhouseshared.ConversationRow{
			AgentUserID:              seed.agentUserID,
			ConversationID:           seed.conversationID,
			ConversationDurationSecs: seed.conversationDurationSecs,
			ConversationStartTime:    scorecardTime,
			ConversationEndTime:      submitTime,
			CustomerID:               r.customerID,
			ProfileID:                r.profileID,
			CreateTime:               now,
			UpdateTime:               now,
		}))
		r.Require().NoError(scorecardBatch.AppendStruct(&clickhouseconv.ScorecardRow{
			AgentUserID:              seed.agentUserID,
			ScorecardID:              scorecardID,
			ScorecardTemplateID:      seed.templateID,
			ScorecardTime:            scorecardTime,
			ScorecardLastUpdateTime:  scorecardTime,
			ScorecardSubmitTime:      submitTime,
			ManuallyScored:           true,
			ConversationID:           seed.conversationID,
			ConversationDurationSecs: seed.conversationDurationSecs,
			CustomerID:               r.customerID,
			ProfileID:                r.profileID,
			UpdateTime:               now,
		}))
		r.Require().NoError(scoreBatch.AppendStruct(&clickhouseconv.ScoreRow{
			AgentUserID:              seed.agentUserID,
			ScorecardID:              scorecardID,
			ScorecardTemplateID:      seed.templateID,
			ScorecardTime:            scorecardTime,
			ScorecardLastUpdateTime:  scorecardTime,
			ScorecardSubmitTime:      submitTime,
			ManuallyScored:           true,
			ConversationID:           seed.conversationID,
			ConversationDurationSecs: seed.conversationDurationSecs,
			CriterionID:              seed.criterionID,
			PercentageValue:          seed.percentageValue,
			FloatWeight:              1.0,
			CustomerID:               r.customerID,
			ProfileID:                r.profileID,
			ScoreID:                  uuid.NewString(),
			UpdateTime:               now,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scorecardBatch))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scoreBatch))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, conversationBatch))
}

// TestRespectConfig_GroupsByDurationFilterAndMergesResults seeds eight templates that collapse into
// exactly three duration-filter query groups internally:
//   - 2 NULL-config + 2 empty-config templates (no duration filter) — one shared group of four;
//   - 2 sharing duration bucket "2" (60–120s) — one group;
//   - 2 sharing duration bucket "6" (300–600s) — one group.
//
// Grouping bounds the number of ClickHouse queries (the latency optimization), but a criterion is
// identified by (scorecard_template_id, criterion_id), so the merged result keeps one row per template —
// eight rows — even where four templates in the no-filter group share a criterion id (CONVI-7068). Each
// row carries its group's seeded score on its single conversation, which also confirms each template's
// duration filter is honored: the conversations fall inside the referenced bucket bounds.
func (r *QaScoreStatsRespectConfigCHTestSuite) TestRespectConfig_GroupsByDurationFilterAndMergesResults() {
	// Given
	suffix := uuid.NewString()[:8]
	noFilterCriterion := "crit-nofilter-" + suffix
	bucket2Criterion := "crit-bucket2-" + suffix
	bucket6Criterion := "crit-bucket6-" + suffix
	const (
		noFilterScore = 0.40
		bucket2Score  = 0.60
		bucket6Score  = 0.80
		bucket2Sec    = 90  // within bucket "2" [60, 120)
		bucket6Sec    = 400 // within bucket "6" [300, 600)
	)

	// A criterion is identified by (scorecard_template_id, criterion_id), so each seeded template yields
	// its own row even when templates share a criterion id (CONVI-7068). Record the per-template
	// expectation as we seed so the assertions stay single-sourced.
	type templateExpectation struct {
		criterionID string
		score       float64
	}
	wantByTemplate := map[string]templateExpectation{}

	var seeds []respectConfigSeed
	var allTemplateIDs []string
	// No-filter group: 2 NULL-config + 2 empty-config templates, all sharing one criterion id; these four
	// collapse into one duration-filter query group but stay four distinct (template, criterion) rows.
	for i := 0; i < 2; i++ {
		nullTemplateID := fmt.Sprintf("nofilter-null-%d-%s", i, suffix)
		emptyTemplateID := fmt.Sprintf("nofilter-empty-%d-%s", i, suffix)
		r.seedTemplateWithoutQaScoreConfig(nullTemplateID)
		r.seedTemplateWithEmptyQaScoreConfig(emptyTemplateID)
		allTemplateIDs = append(allTemplateIDs, nullTemplateID, emptyTemplateID)
		wantByTemplate[nullTemplateID] = templateExpectation{criterionID: noFilterCriterion, score: noFilterScore}
		wantByTemplate[emptyTemplateID] = templateExpectation{criterionID: noFilterCriterion, score: noFilterScore}
		seeds = append(seeds,
			respectConfigSeed{templateID: nullTemplateID, criterionID: noFilterCriterion, agentUserID: "agent_nf", conversationID: uuid.NewString(), percentageValue: noFilterScore},
			respectConfigSeed{templateID: emptyTemplateID, criterionID: noFilterCriterion, agentUserID: "agent_nf", conversationID: uuid.NewString(), percentageValue: noFilterScore},
		)
	}
	// Two meaningful groups, each shared by two templates carrying the same duration bucket.
	for i := 0; i < 2; i++ {
		bucket2TemplateID := fmt.Sprintf("bucket2-%d-%s", i, suffix)
		bucket6TemplateID := fmt.Sprintf("bucket6-%d-%s", i, suffix)
		r.seedTemplateWithDurationFilter(bucket2TemplateID, "2")
		r.seedTemplateWithDurationFilter(bucket6TemplateID, "6")
		allTemplateIDs = append(allTemplateIDs, bucket2TemplateID, bucket6TemplateID)
		wantByTemplate[bucket2TemplateID] = templateExpectation{criterionID: bucket2Criterion, score: bucket2Score}
		wantByTemplate[bucket6TemplateID] = templateExpectation{criterionID: bucket6Criterion, score: bucket6Score}
		seeds = append(seeds,
			respectConfigSeed{templateID: bucket2TemplateID, criterionID: bucket2Criterion, agentUserID: "agent_b2", conversationID: uuid.NewString(), percentageValue: bucket2Score, conversationDurationSecs: bucket2Sec},
			respectConfigSeed{templateID: bucket6TemplateID, criterionID: bucket6Criterion, agentUserID: "agent_b6", conversationID: uuid.NewString(), percentageValue: bucket6Score, conversationDurationSecs: bucket6Sec},
		)
	}
	r.seedScores(seeds)

	scorecardTemplateNames := make([]string, 0, len(allTemplateIDs))
	for _, templateID := range allTemplateIDs {
		scorecardTemplateNames = append(scorecardTemplateNames, coachingpb.ScorecardTemplateName{
			CustomerID:                  r.customerID,
			ProfileID:                   r.profileID,
			ScorecardTemplateID:         templateID,
			ScorecardTemplateRevisionID: "*",
		}.String())
	}

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, &analyticspb.RetrieveQAScoreStatsRequest{
		Parent:            r.profileResourceName,
		FilterByTimeRange: &r.dates.janPeriod,
		GroupByAttributeTypes: []analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: scorecardTemplateNames,
		},
		RespectTemplateQaScoreConfig: true,
		Metadata:                     &r.dates.metadataLA,
	})

	// Then — one row per template, keyed by (scorecard_template_id, criterion_id). The eight templates
	// collapse into three duration-filter query groups internally, but each template keeps its own row
	// (CONVI-7068) even when it shares a criterion id with siblings in the same group. Each row carries
	// its group's seeded score and counts its single conversation, proving every template's duration
	// filter was applied to — and only to — its own conversation.
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)
	r.Require().Len(resp.QaScoreResult.Scores, len(wantByTemplate), "each template must yield its own (scorecard_template_id, criterion_id) row")

	gotScoreByTemplate := map[string]*analyticspb.QAScore{}
	for _, score := range resp.QaScoreResult.Scores {
		r.NotEmpty(score.GroupedBy.CriterionId, "criterion grouping must populate CriterionId")
		r.NotEmpty(score.GroupedBy.ScorecardTemplateId, "criterion grouping must populate ScorecardTemplateId")
		gotScoreByTemplate[score.GroupedBy.ScorecardTemplateId] = score
	}

	for templateID, want := range wantByTemplate {
		got, ok := gotScoreByTemplate[templateID]
		r.Require().True(ok, "template %s should appear in the merged result", templateID)
		r.Equal(want.criterionID, got.GroupedBy.CriterionId, "template %s should carry its seeded criterion id", templateID)
		r.InDelta(want.score, got.Score, AcceptedDelta, "template %s should carry its group's seeded score", templateID)
		r.Equal(int32(1), got.TotalConversationCount, "template %s should aggregate exactly its single conversation", templateID)
	}
}

// TestRespectConfig_SharedCriterionAcrossMergedTemplatesKeepsRowPerTemplate covers two templates that
// share the same criterion id (the CONVI-7014 bug) and score it on the same conversation. Both land in
// the no-filter group, so the merge serves them with one combined query. Criterion ids are not unique
// across scorecard templates, so a criterion is identified by (scorecard_template_id, criterion_id):
// the query groups by both, yielding one row per template even though they share a criterion id and a
// conversation. This is the CONVI-7068 fix — the old path grouped by criterion id alone, collapsing the
// two distinct criteria into a single row.
func (r *QaScoreStatsRespectConfigCHTestSuite) TestRespectConfig_SharedCriterionAcrossMergedTemplatesKeepsRowPerTemplate() {
	// Given
	suffix := uuid.NewString()[:8]
	sharedCriterion := "crit-shared-" + suffix
	sharedConversationID := uuid.NewString()
	const sharedScore = 0.50

	emptyTemplateID := "shared-empty-" + suffix
	nullTemplateID := "shared-null-" + suffix
	r.seedTemplateWithEmptyQaScoreConfig(emptyTemplateID)
	r.seedTemplateWithoutQaScoreConfig(nullTemplateID)

	r.seedScores([]respectConfigSeed{
		{templateID: emptyTemplateID, criterionID: sharedCriterion, agentUserID: "agent_shared", conversationID: sharedConversationID, percentageValue: sharedScore},
		{templateID: nullTemplateID, criterionID: sharedCriterion, agentUserID: "agent_shared", conversationID: sharedConversationID, percentageValue: sharedScore},
	})

	scorecardTemplateNames := make([]string, 0, 2)
	for _, templateID := range []string{emptyTemplateID, nullTemplateID} {
		scorecardTemplateNames = append(scorecardTemplateNames, coachingpb.ScorecardTemplateName{
			CustomerID:                  r.customerID,
			ProfileID:                   r.profileID,
			ScorecardTemplateID:         templateID,
			ScorecardTemplateRevisionID: "*",
		}.String())
	}

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, &analyticspb.RetrieveQAScoreStatsRequest{
		Parent:            r.profileResourceName,
		FilterByTimeRange: &r.dates.janPeriod,
		GroupByAttributeTypes: []analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: scorecardTemplateNames,
		},
		RespectTemplateQaScoreConfig: true,
		Metadata:                     &r.dates.metadataLA,
	})

	// Then — one row per template, each identified by (scorecard_template_id, criterion_id) even though
	// the two templates share a criterion id and the same conversation.
	r.Require().NoError(err)
	r.Require().NotNil(resp.QaScoreResult)
	r.Require().Len(resp.QaScoreResult.Scores, 2, "two templates sharing a criterion id must yield one row per template, keyed by (scorecard_template_id, criterion_id)")

	scoresByTemplate := make(map[string]*analyticspb.QAScore, 2)
	for _, score := range resp.QaScoreResult.Scores {
		r.Equal(sharedCriterion, score.GroupedBy.CriterionId)
		scoresByTemplate[score.GroupedBy.ScorecardTemplateId] = score
	}

	for _, templateID := range []string{emptyTemplateID, nullTemplateID} {
		got, ok := scoresByTemplate[templateID]
		r.Require().True(ok, "expected a row for template %q", templateID)
		r.Equal(int32(1), got.TotalConversationCount, "each template's row counts its single scored conversation once")
		r.Equal(int32(1), got.TotalScorecardCount, "each template contributes exactly one scorecard")
		r.InDelta(sharedScore, got.Score, AcceptedDelta, "score should reflect the shared criterion's seeded value")
	}
}

func TestQaScoreStatsRespectConfigCH(t *testing.T) {
	suite.Run(t, &QaScoreStatsRespectConfigCHTestSuite{})
}
