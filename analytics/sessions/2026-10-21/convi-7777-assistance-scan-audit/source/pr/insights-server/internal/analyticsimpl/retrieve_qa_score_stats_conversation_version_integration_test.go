package analyticsimpl

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// scoredAgentUserID is the agent on every scorecard and score row the suite seeds.
const scoredAgentUserID = "agent_versions"

// QaScoreStatsConversationVersionCHTestSuite runs RetrieveQAScoreStats against a
// real ClickHouse container, with conversations that have more than one
// conversation_d version: for example, the first write carries the placeholder end
// (end = start) and a later write the real end.
//
// ReplacingMergeTree merges versions that share a sort key in the background, at
// any time, so versions of one conversation here differ in agent_user_id or in the
// start hour, the two leading sort key columns. Such versions never merge.
type QaScoreStatsConversationVersionCHTestSuite struct {
	AnalyticsTestSuite
	dates *TimePeriodData
}

func (r *QaScoreStatsConversationVersionCHTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.dates = r.SetupTimePeriodData()
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

func (r *QaScoreStatsConversationVersionCHTestSuite) TearDownSubTest() {
	r.server.enableExtTableForUserFilter = false
}

// conversationVersion is one conversation_d row. Rows sharing a conversation ID
// are versions of the same conversation; the one with the latest updateTime is
// the newest.
type conversationVersion struct {
	agentUserID string
	startTime   time.Time
	endTime     time.Time
	updateTime  time.Time
}

// scoredConversation is a conversation with its conversation_d versions and one
// scorecard for scoredAgentUserID, scored for one criterion. The scorecard time is
// the first version's start time.
type scoredConversation struct {
	conversationID  string
	versions        []conversationVersion
	percentageValue float64
}

func (r *QaScoreStatsConversationVersionCHTestSuite) seed(templateID, criterionID string, conversations []scoredConversation) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)

	conversationBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	r.Require().NoError(err)
	scorecardBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO scorecard (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScorecardRow{})))
	r.Require().NoError(err)
	scoreBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO score (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ScoreRow{})))
	r.Require().NoError(err)

	for _, conversation := range conversations {
		for _, version := range conversation.versions {
			r.Require().NoError(conversationBatch.AppendStruct(&clickhouseshared.ConversationRow{
				AgentUserID:              version.agentUserID,
				ConversationID:           conversation.conversationID,
				ConversationStartTime:    version.startTime,
				ConversationEndTime:      version.endTime,
				ConversationDurationSecs: int32(version.endTime.Sub(version.startTime).Seconds()),
				CustomerID:               r.customerID,
				ProfileID:                r.profileID,
				CreateTime:               version.updateTime,
				UpdateTime:               version.updateTime,
			}))
		}
		scorecardID := uuid.NewString()
		scorecardTime := conversation.versions[0].startTime
		submitTime := scorecardTime.Add(2 * time.Hour)
		r.Require().NoError(scorecardBatch.AppendStruct(&clickhouseconv.ScorecardRow{
			AgentUserID:             scoredAgentUserID,
			ScorecardID:             scorecardID,
			ScorecardTemplateID:     templateID,
			ScorecardTime:           scorecardTime,
			ScorecardLastUpdateTime: scorecardTime,
			ScorecardSubmitTime:     submitTime,
			ManuallyScored:          true,
			ConversationID:          conversation.conversationID,
			CustomerID:              r.customerID,
			ProfileID:               r.profileID,
			UpdateTime:              submitTime,
		}))
		r.Require().NoError(scoreBatch.AppendStruct(&clickhouseconv.ScoreRow{
			AgentUserID:             scoredAgentUserID,
			ScorecardID:             scorecardID,
			ScorecardTemplateID:     templateID,
			ScorecardTime:           scorecardTime,
			ScorecardLastUpdateTime: scorecardTime,
			ScorecardSubmitTime:     submitTime,
			ManuallyScored:          true,
			ConversationID:          conversation.conversationID,
			CriterionID:             criterionID,
			PercentageValue:         conversation.percentageValue,
			FloatWeight:             1.0,
			CustomerID:              r.customerID,
			ProfileID:               r.profileID,
			ScoreID:                 uuid.NewString(),
			UpdateTime:              submitTime,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, conversationBatch))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scorecardBatch))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, scoreBatch))
}

func (r *QaScoreStatsConversationVersionCHTestSuite) request(
	templateID string,
	timeRangeField analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
) *analyticspb.RetrieveQAScoreStatsRequest {
	return &analyticspb.RetrieveQAScoreStatsRequest{
		Parent:                     r.profileResourceName,
		FilterByTimeRange:          &r.dates.janPeriod,
		ConversationTimeRangeField: timeRangeField,
		GroupByAttributeTypes: []analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{coachingpb.ScorecardTemplateName{
				CustomerID:                  r.customerID,
				ProfileID:                   r.profileID,
				ScorecardTemplateID:         templateID,
				ScorecardTemplateRevisionID: "*",
			}.String()},
		},
		Metadata: &r.dates.metadataLA,
	}
}

// A conversation whose placeholder end and real end both fall in the range is
// counted once. Counted once per end time, the 1.0 conversation would weigh
// twice and the score would read (1.0 + 1.0 + 0.0) / 3 = 0.667.
func (r *QaScoreStatsConversationVersionCHTestSuite) TestEndTimeRange_CountsConversationWithTwoEndTimesOnce() {
	// Given
	suffix := uuid.NewString()[:8]
	templateID := "versions-twice-" + suffix
	twoEndsStart := time.Date(2021, 1, 5, 12, 0, 0, 0, time.UTC)
	oneEndStart := time.Date(2021, 1, 6, 12, 0, 0, 0, time.UTC)
	r.seed(templateID, "crit-versions-twice-"+suffix, []scoredConversation{
		{
			conversationID: uuid.NewString(),
			versions: []conversationVersion{
				{agentUserID: "agent_before_reassignment", startTime: twoEndsStart, endTime: twoEndsStart, updateTime: twoEndsStart},
				{agentUserID: scoredAgentUserID, startTime: twoEndsStart, endTime: twoEndsStart.Add(30 * time.Minute), updateTime: twoEndsStart.Add(time.Hour)},
			},
			percentageValue: 1.0,
		},
		{
			conversationID: uuid.NewString(),
			versions: []conversationVersion{
				{agentUserID: scoredAgentUserID, startTime: oneEndStart, endTime: oneEndStart.Add(30 * time.Minute), updateTime: oneEndStart.Add(time.Hour)},
			},
			percentageValue: 0.0,
		},
	})

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.request(templateID, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT))

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.GetQaScoreResult().GetScores(), 1)
	got := resp.GetQaScoreResult().GetScores()[0]
	r.InDelta(0.5, got.GetScore(), AcceptedDelta)
	r.Equal(int32(2), got.GetTotalConversationCount())
	r.Equal(int32(2), got.GetTotalScorecardCount())
}

// A conversation whose older version ended in the range but whose newest version
// ends after it, as a reopened chat that closed again does, is left out.
func (r *QaScoreStatsConversationVersionCHTestSuite) TestEndTimeRange_FiltersOnNewestEndTime() {
	// Given
	suffix := uuid.NewString()[:8]
	templateID := "versions-newest-" + suffix
	reopenedStart := time.Date(2021, 1, 30, 20, 0, 0, 0, time.UTC)
	newestEnd := time.Date(2021, 2, 1, 9, 0, 0, 0, time.UTC)
	inRangeStart := time.Date(2021, 1, 10, 12, 0, 0, 0, time.UTC)
	r.seed(templateID, "crit-versions-newest-"+suffix, []scoredConversation{
		{
			conversationID: uuid.NewString(),
			versions: []conversationVersion{
				{agentUserID: "agent_before_reassignment", startTime: reopenedStart, endTime: reopenedStart.Add(time.Hour), updateTime: reopenedStart.Add(time.Hour)},
				{agentUserID: scoredAgentUserID, startTime: reopenedStart, endTime: newestEnd, updateTime: newestEnd},
			},
			percentageValue: 1.0,
		},
		{
			conversationID: uuid.NewString(),
			versions: []conversationVersion{
				{agentUserID: scoredAgentUserID, startTime: inRangeStart, endTime: inRangeStart.Add(30 * time.Minute), updateTime: inRangeStart.Add(time.Hour)},
			},
			percentageValue: 0.25,
		},
	})

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, r.request(templateID, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT))

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.GetQaScoreResult().GetScores(), 1)
	got := resp.GetQaScoreResult().GetScores()[0]
	r.InDelta(0.25, got.GetScore(), AcceptedDelta)
	r.Equal(int32(1), got.GetTotalConversationCount())
	r.Equal(int32(1), got.GetTotalScorecardCount())
}

// A conversation whose start time was corrected from inside the range
// (2021-01-30 20:00) to after it (2021-01-31 02:00) is left out. A start-time
// range reads conversation_d only with a conversation filter, here a duration
// bucket. The correction stays within the scan's 24h slack, so the scan reads the
// newest version; a scan bounded to the range itself would read only the older
// version and keep the conversation, for a score of (1.0 + 0.25) / 2 = 0.625.
func (r *QaScoreStatsConversationVersionCHTestSuite) TestStartTimeRange_FiltersOnNewestStartTime() {
	// Given
	suffix := uuid.NewString()[:8]
	templateID := "versions-start-" + suffix
	originalStart := time.Date(2021, 1, 30, 20, 0, 0, 0, time.UTC)
	correctedStart := time.Date(2021, 1, 31, 2, 0, 0, 0, time.UTC)
	inRangeStart := time.Date(2021, 1, 10, 12, 0, 0, 0, time.UTC)
	r.seed(templateID, "crit-versions-start-"+suffix, []scoredConversation{
		{
			conversationID: uuid.NewString(),
			versions: []conversationVersion{
				{agentUserID: scoredAgentUserID, startTime: originalStart, endTime: originalStart.Add(30 * time.Minute), updateTime: originalStart.Add(time.Hour)},
				{agentUserID: scoredAgentUserID, startTime: correctedStart, endTime: correctedStart.Add(30 * time.Minute), updateTime: correctedStart.Add(time.Hour)},
			},
			percentageValue: 1.0,
		},
		{
			conversationID: uuid.NewString(),
			versions: []conversationVersion{
				{agentUserID: scoredAgentUserID, startTime: inRangeStart, endTime: inRangeStart.Add(30 * time.Minute), updateTime: inRangeStart.Add(time.Hour)},
			},
			percentageValue: 0.25,
		},
	})

	req := r.request(templateID, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_STARTED_AT)
	// Bucket 8 is [20m, 40m) in hardcodedConversationDurationBuckets.
	req.FilterByAttribute.ConversationDurationBuckets = []*analyticspb.ConversationDurationBucket{{Id: "8"}}

	// When
	resp, err := r.server.RetrieveQAScoreStats(r.ctx, req)

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.GetQaScoreResult().GetScores(), 1)
	got := resp.GetQaScoreResult().GetScores()[0]
	r.InDelta(0.25, got.GetScore(), AcceptedDelta)
	r.Equal(int32(1), got.GetTotalConversationCount())
}

// An agent filter matches any version of the conversation, so the scored agent's
// scorecard stays in their filtered view after the conversation is reassigned to
// another agent. The score and scorecard rows carry the scored agent.
func (r *QaScoreStatsConversationVersionCHTestSuite) TestAgentFilter_MatchesConversationReassignedAway() {
	for _, useExtTable := range []bool{false, true} {
		r.Run(fmt.Sprintf("useExtTable=%t", useExtTable), func() {
			// Given
			r.server.enableExtTableForUserFilter = useExtTable
			suffix := uuid.NewString()[:8]
			templateID := "versions-agent-" + suffix
			start := time.Date(2021, 1, 12, 12, 0, 0, 0, time.UTC)
			r.seed(templateID, "crit-versions-agent-"+suffix, []scoredConversation{
				{
					conversationID: uuid.NewString(),
					versions: []conversationVersion{
						{agentUserID: scoredAgentUserID, startTime: start, endTime: start.Add(30 * time.Minute), updateTime: start.Add(30 * time.Minute)},
						{agentUserID: "agent_reassigned_to", startTime: start, endTime: start.Add(40 * time.Minute), updateTime: start.Add(time.Hour)},
					},
					percentageValue: 1.0,
				},
			})
			req := r.request(templateID, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT)
			req.FilterByAttribute.Users = []*userpb.User{{
				Name: userpb.UserName{CustomerID: r.customerID, UserID: scoredAgentUserID}.String(),
			}}

			// When
			result, err := r.server.readQaScoreStatsFromClickhouse(r.ctx, req, nil /* users */, nil /* lowerIsBetterCriteria */)

			// Then
			r.Require().NoError(err)
			r.Require().Len(result.response.GetQaScoreResult().GetScores(), 1)
			got := result.response.GetQaScoreResult().GetScores()[0]
			r.InDelta(1.0, got.GetScore(), AcceptedDelta)
			r.Equal(int32(1), got.GetTotalConversationCount())
		})
	}
}

func TestQaScoreStatsConversationVersionCH(t *testing.T) {
	suite.Run(t, &QaScoreStatsConversationVersionCHTestSuite{})
}
