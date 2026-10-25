package analyticsimpl

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// ConversationStatsConversationVersionCHTestSuite runs RetrieveConversationStats
// against a real ClickHouse container, for a conversation whose rows carry
// different copies of its end time: rows written during the call carry the
// placeholder end (end = start), and rows written after the close the real end.
//
// The 25-hour conversation's placeholder end (2021-01-05 21:30 HST) and real end
// (2021-01-06 22:30 HST) fall on different days of the daily buckets.
type ConversationStatsConversationVersionCHTestSuite struct {
	AnalyticsTestSuite
	dates *TimePeriodData
}

func (r *ConversationStatsConversationVersionCHTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.dates = r.SetupTimePeriodData()
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

var (
	versionedConversationStart   = time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	versionedConversationRealEnd = time.Date(2021, 1, 7, 8, 30, 0, 0, time.UTC)
	// realEndDayBucket is the start of the HST day holding the real end.
	realEndDayBucket = time.Date(2021, 1, 6, 10, 0, 0, 0, time.UTC)
)

// seedVersionedConversation writes one conversation with a placeholder-end and a
// real-end row in conversation_d and message_d, with handle times 1800 and 3600.
// With momentTemplateID set it adds a metadata moment with the value "versions";
// with agentAssisted, a placeholder-end and a real-end conversation_with_labels
// row, both assisted.
func (r *ConversationStatsConversationVersionCHTestSuite) seedVersionedConversation(agentUserID, momentTemplateID string, agentAssisted bool) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	conversationID := uuid.NewString()
	start, realEnd := versionedConversationStart, versionedConversationRealEnd

	// One insert per version, so that ReplacingMergeTree does not collapse the two
	// conversation_d rows, which share a sort key, on insert. A background merge can
	// still collapse them to the real-end row, which leaves the expected result as
	// it is.
	for _, end := range []time.Time{start, realEnd} {
		conversationBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
			fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
		r.Require().NoError(err)
		r.Require().NoError(conversationBatch.AppendStruct(&clickhouseshared.ConversationRow{
			AgentUserID:           agentUserID,
			ConversationID:        conversationID,
			ConversationStartTime: start,
			ConversationEndTime:   end,
			CustomerID:            r.customerID,
			ProfileID:             r.profileID,
			CreateTime:            end,
			UpdateTime:            end,
		}))
		r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, conversationBatch))
	}

	messageBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO message (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.MessageRow{})))
	r.Require().NoError(err)
	for _, message := range []struct {
		endTime        time.Time
		handleTimeSecs int32
	}{
		{endTime: start, handleTimeSecs: 1800},
		{endTime: realEnd, handleTimeSecs: 3600},
	} {
		r.Require().NoError(messageBatch.AppendStruct(&clickhouseconv.MessageRow{
			AgentUserID:           agentUserID,
			ConversationID:        conversationID,
			ConversationStartTime: start,
			ConversationEndTime:   message.endTime,
			CustomerID:            r.customerID,
			ProfileID:             r.profileID,
			HandleTimeSecs:        message.handleTimeSecs,
			MessageID:             uuid.NewString(),
			CreateTime:            message.endTime,
			UpdateTime:            message.endTime,
		}))
	}
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, messageBatch))

	if momentTemplateID != "" {
		momentBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
			fmt.Sprintf("INSERT INTO moment_annotation (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.MomentAnnotationRow{})))
		r.Require().NoError(err)
		r.Require().NoError(momentBatch.AppendStruct(&clickhouseconv.MomentAnnotationRow{
			AgentUserID:           agentUserID,
			ConversationID:        conversationID,
			ConversationStartTime: start,
			ConversationEndTime:   realEnd,
			CustomerID:            r.customerID,
			ProfileID:             r.profileID,
			MomentAnnotationID:    uuid.NewString(),
			MomentTemplateID:      momentTemplateID,
			MomentType:            int32(momentpb.Moment_CONVERSATION_METADATA),
			MetadataStringValue:   "versions",
			CreateTime:            realEnd,
			UpdateTime:            realEnd,
		}))
		r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, momentBatch))
	}

	if agentAssisted {
		labelBatch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
			fmt.Sprintf("INSERT INTO conversation_with_labels (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.ConversationWithLabelsRow{})))
		r.Require().NoError(err)
		for _, end := range []time.Time{start, realEnd} {
			r.Require().NoError(labelBatch.AppendStruct(&clickhouseconv.ConversationWithLabelsRow{
				CustomerID:            r.customerID,
				ProfileID:             r.profileID,
				AgentUserID:           agentUserID,
				ConversationID:        conversationID,
				ConversationStartTime: start,
				ConversationEndTime:   end,
				WithAgentAssistance:   true,
				UpdateTime:            end,
				CreateTime:            end,
				MessageID:             uuid.NewString(),
				MessageAgentUserID:    agentUserID,
				MessageTime:           end,
			}))
		}
		r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, labelBatch))
	}
}

// request asks for daily stats over January by end time for one agent, filtered
// on the "versions" metadata moment when momentTemplateID is set.
func (r *ConversationStatsConversationVersionCHTestSuite) request(agentUserID, momentTemplateID string) *analyticspb.RetrieveConversationStatsRequest {
	req := &analyticspb.RetrieveConversationStatsRequest{
		Parent:                     r.profileResourceName,
		FilterByTimeRange:          &r.dates.janPeriod,
		ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
		Frequency:                  analyticspb.Frequency_DAILY,
		GroupByAttributeTypes:      []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE},
		Metadata:                   &r.dates.metadataLA,
		FilterByAttribute: &analyticspb.Attribute{
			Users: []*userpb.User{{Name: userpb.UserName{CustomerID: r.customerID, UserID: agentUserID}.String()}},
		},
		AllowMatchingStaleMetadataValues: true,
	}
	if momentTemplateID != "" {
		req.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{{
			Moments: []*momentpb.Moment{{
				Name: fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, momentTemplateID),
				Type: momentpb.Moment_CONVERSATION_METADATA,
			}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "versions"}},
			}},
		}}
	}
	return req
}

// assertCountedOnceInRealEndBucket checks that the conversation counts once, in
// the real end's day, with the handle time of the real-end message row. Counted
// once per end time, it would show count 2 across two days.
func (r *ConversationStatsConversationVersionCHTestSuite) assertCountedOnceInRealEndBucket(resp *analyticspb.RetrieveConversationStatsResponse) {
	r.Equal(int32(1), resp.GetTotalConversationCount())
	r.Equal(int64(3600), resp.GetAverageHandleTime().GetSeconds())
	r.Require().Len(resp.GetConversationStatsResults(), 1)
	r.Require().Len(resp.GetConversationStatsResults()[0].GetConversationStats(), 1)
	got := resp.GetConversationStatsResults()[0].GetConversationStats()[0]
	r.Equal(realEndDayBucket, got.GetTimeRange().GetStartTimestamp().AsTime())
}

// The moment-filter variant read each message_d row's own end time for the
// bucket. The time bucket now comes from conversation_d's newest version.
func (r *ConversationStatsConversationVersionCHTestSuite) TestEndTimeRange_MomentFilter_BucketsConversationOnceByNewestEndTime() {
	// Given
	suffix := uuid.NewString()[:8]
	agentUserID := "agent-versions-" + suffix
	momentTemplateID := "moment-versions-" + suffix
	r.seedVersionedConversation(agentUserID, momentTemplateID, false /* agentAssisted */)

	// When
	resp, err := r.server.readConversationStatsFromClickhouse(r.ctx, r.request(agentUserID, momentTemplateID),
		nil /* users */, false /* includeBotConversations */, true /* allowMatchingStaleMetadataValues */)

	// Then
	r.Require().NoError(err)
	r.assertCountedOnceInRealEndBucket(resp)
}

// The agent-assist-on filter kept one conversation_with_labels row per distinct
// end time. It now keeps the newest row per agent and conversation, in both query
// variants.
func (r *ConversationStatsConversationVersionCHTestSuite) TestEndTimeRange_AgentAssistOn_BucketsConversationOnceByNewestEndTime() {
	for _, withMomentFilter := range []bool{false, true} {
		r.Run(fmt.Sprintf("withMomentFilter=%t", withMomentFilter), func() {
			// Given
			suffix := uuid.NewString()[:8]
			agentUserID := "agent-assisted-" + suffix
			momentTemplateID := ""
			if withMomentFilter {
				momentTemplateID = "moment-assisted-" + suffix
			}
			r.seedVersionedConversation(agentUserID, momentTemplateID, true /* agentAssisted */)
			req := r.request(agentUserID, momentTemplateID)
			req.FilterByAttribute.AgentAssistFilters = []*analyticspb.Attribute_AgentAssistFilter{{
				AgentAssistUsed: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON,
			}}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.ctx, req,
				nil /* users */, false /* includeBotConversations */, true /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.assertCountedOnceInRealEndBucket(resp)
		})
	}
}

func TestConversationStatsConversationVersionCH(t *testing.T) {
	suite.Run(t, &ConversationStatsConversationVersionCHTestSuite{})
}
