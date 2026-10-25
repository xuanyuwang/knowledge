package analyticsimpl

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	ev "github.com/cresta/go-servers/shared/clickhouse/events"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// RetrieveCustomerSnapshotStatsIntegrationTestSuite drives the real ClickHouse
// container to verify the session-pairing + islands-and-gaps merge logic that
// the unit tests cannot reach (the mock returns pre-aggregated rows, so the
// session_intervals/ordered/islands/merged CTEs never run there).
//
// It does NOT call UseMockedClickhouseConnections, so the server resolves the
// real events ClickHouse connection. Both conversation_d and conversation_event_d
// are seeded on the events connection because the metric query runs there and
// reads both tables from the events database.
type RetrieveCustomerSnapshotStatsIntegrationTestSuite struct {
	AnalyticsTestSuite
	janFirst time.Time
}

func (r *RetrieveCustomerSnapshotStatsIntegrationTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	// The base suite already mocks configClient.Get() (ACL disabled) AnyTimes;
	// RetrieveCustomerSnapshotStats does not call GetProfileConfig, so no further
	// config mocking is needed.
	r.janFirst = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
}

// seedConversation inserts one conversation_d row on the events connection.
func (r *RetrieveCustomerSnapshotStatsIntegrationTestSuite) seedConversation(agentUserID, conversationID string, startTime time.Time) {
	ctx := context.Background()
	conn := r.GetEventsConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	r.Require().NoError(err)
	r.Require().NoError(batch.AppendStruct(&clickhouseshared.ConversationRow{
		AgentUserID:           agentUserID,
		ConversationID:        conversationID,
		ConversationStartTime: startTime,
		ConversationEndTime:   startTime.Add(30 * time.Minute),
		CustomerID:            r.customerID,
		ProfileID:             r.profileID,
		CreateTime:            time.Now(),
		UpdateTime:            time.Now(),
	}))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, batch))
}

// snapshotEvent builds a CUSTOMER_SNAPSHOT_PRESENTED (27) or CLOSED (28) event
// payload and inserts it into conversation_event_d on the events connection.
func (r *RetrieveCustomerSnapshotStatsIntegrationTestSuite) snapshotEvent(
	agentUserID, conversationID string, conversationStart, createTime time.Time, eventType int32, sessionID, contentHash string,
) {
	ctx := context.Background()
	conn := r.GetEventsConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO conversation_event_d (%s)", clickhouseshared.ExtractCHFields(ev.ConversationEventRow{})))
	r.Require().NoError(err)
	payloadField := "customerSnapshotClosed"
	if eventType == 27 {
		payloadField = "customerSnapshotPresented"
	}
	r.Require().NoError(batch.AppendStruct(&ev.ConversationEventRow{
		AgentUserID:           agentUserID,
		ConversationID:        conversationID,
		ConversationStartTime: conversationStart,
		CreateTime:            createTime,
		CustomerID:            r.customerID,
		EventID:               uuid.NewString(),
		EventName:             "customer_snapshot",
		EventPayload: fmt.Sprintf(
			`{%q: {%q: %q, %q: %q}}`,
			payloadField, "sessionId", sessionID, "contentHash", contentHash),
		EventType: eventType,
		ProfileID: r.profileID,
	}))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, batch))
}

// TestRetrieveCustomerSnapshotStats_PairingAndMerge verifies the ClickHouse
// display-time metric end to end:
//
//	conversation 1 (agent): two overlapping snapshot sessions + one inverted
//	  session that must be dropped.
//	    A: presented @100s, closed @300s -> [100,300]
//	    B: presented @200s, closed @400s -> [200,400] (overlaps A)
//	    C: presented @500s, closed @450s -> dropped (closed before presented)
//	  islands-and-gaps merges A+B into one interval [100,400] = 300s, so the
//	  summed display time is 300 (NOT 400 — overlap is not double-counted).
//
//	conversation 2 (same agent): one simple session.
//	    D: presented @600s, closed @650s -> [600,650] = 50s
//
// Per the final aggregation (grouped by day + agent): total display time =
// 300 + 50 = 350s across 2 conversations, 1 distinct agent.
func (r *RetrieveCustomerSnapshotStatsIntegrationTestSuite) TestRetrieveCustomerSnapshotStats_PairingAndMerge() {
	// Given
	// Use agent "1" (known to the base suite's mock user service) so the explicit
	// user filter survives ParseUserFilterForAnalytics, which validates requested
	// users against the user service. Isolation from other tests' data holds
	// because no other test seeds CUSTOMER_SNAPSHOT (event_type 27/28) events.
	agentUserID := "1"
	conv1 := "cs_conv1_" + uuid.NewString()
	conv2 := "cs_conv2_" + uuid.NewString()
	convStart := r.janFirst

	r.seedConversation(agentUserID, conv1, convStart)
	r.seedConversation(agentUserID, conv2, convStart)

	// Conversation 1: overlapping sessions A and B, plus inverted session C.
	r.snapshotEvent(agentUserID, conv1, convStart, convStart.Add(100*time.Second), 27 /*PRESENTED*/, "A", "ch_A")
	r.snapshotEvent(agentUserID, conv1, convStart, convStart.Add(300*time.Second), 28 /*CLOSED*/, "A", "ch_A")
	r.snapshotEvent(agentUserID, conv1, convStart, convStart.Add(200*time.Second), 27 /*PRESENTED*/, "B", "ch_B")
	r.snapshotEvent(agentUserID, conv1, convStart, convStart.Add(400*time.Second), 28 /*CLOSED*/, "B", "ch_B")
	r.snapshotEvent(agentUserID, conv1, convStart, convStart.Add(500*time.Second), 27 /*PRESENTED*/, "C", "ch_C")
	r.snapshotEvent(agentUserID, conv1, convStart, convStart.Add(450*time.Second), 28 /*CLOSED*/, "C", "ch_C")
	// Conversation 2: one simple session D.
	r.snapshotEvent(agentUserID, conv2, convStart, convStart.Add(600*time.Second), 27 /*PRESENTED*/, "D", "ch_D")
	r.snapshotEvent(agentUserID, conv2, convStart, convStart.Add(650*time.Second), 28 /*CLOSED*/, "D", "ch_D")

	// When
	resp, err := r.server.RetrieveCustomerSnapshotStats(r.ctx, &analyticspb.RetrieveCustomerSnapshotStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(r.janFirst),
			EndTimestamp:   timestamppb.New(r.janFirst.Add(24 * time.Hour)),
		},
		// Filter to the seeded agent so other tests' data in the shared
		// per-customer/profile ClickHouse DB cannot affect the assertion.
		FilterByAttribute: &analyticspb.Attribute{
			Users: []*userpb.User{
				{Name: fmt.Sprintf("customers/%s/users/%s", r.customerID, agentUserID)},
			},
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Frequency: analyticspb.Frequency_DAILY,
	})

	// Then
	r.Require().NoError(err)
	r.Require().NotNil(resp)
	// One agent result, one day bucket. The overlap merge yields 300s (not
	// 400s) for conversation 1; conversation 2 contributes 50s. The inverted
	// session C is dropped, so it does not inflate the conversation count.
	expected := &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults: []*analyticspb.CustomerSnapshotStatsResult{
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: timestamppb.New(r.janFirst)},
						TotalDisplayTime:                      350,
						AverageDisplayTime:                    350.0 / 2,
						AverageDisplayTimePerFrequencyPerUser: 350.0 / 1,
						TotalConversationsWithSnapshot:        2,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{Name: fmt.Sprintf("customers/%s/users/%s", r.customerID, agentUserID), Username: "agent_a", FullName: "agent a"},
					},
				},
				TotalDisplayTime:                      350,
				AverageDisplayTime:                    350.0 / 2,
				AverageDisplayTimePerFrequencyPerUser: 350.0 / 1,
				TotalConversationsWithSnapshot:        2,
			},
		},
		TotalDisplayTime:                      350,
		AverageDisplayTime:                    350.0 / 2,
		AverageDisplayTimePerFrequencyPerUser: 350.0 / 1,
		TotalConversationsWithSnapshot:        2,
	}
	diff := cmp.Diff(
		expected, resp,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveCustomerSnapshotStatsResponse{}, "customer_snapshot_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func TestRetrieveCustomerSnapshotStatsIntegration(t *testing.T) {
	suite.Run(t, &RetrieveCustomerSnapshotStatsIntegrationTestSuite{})
}
