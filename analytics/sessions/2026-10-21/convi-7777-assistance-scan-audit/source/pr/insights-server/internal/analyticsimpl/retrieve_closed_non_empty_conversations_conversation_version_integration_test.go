package analyticsimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/util/jwt"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	sharedauth "github.com/cresta/go-servers/shared/auth"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// ClosedNonEmptyConversationsVersionCHTestSuite runs RetrieveClosedNonEmptyConversations
// against a real ClickHouse container, for conversations with more than one
// conversation_d version. ReplacingMergeTree merges versions that share a sort key
// in the background, at any time, so the versions here differ in agent_user_id or
// in the start hour, the two leading sort key columns. Such versions never merge.
type ClosedNonEmptyConversationsVersionCHTestSuite struct {
	AnalyticsTestSuite
}

func (r *ClosedNonEmptyConversationsVersionCHTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	// The requester is the user in the access token claim of the request context.
	authEntityProvider := authmocks.NewMockAuthEntityProvider(gomock.NewController(r.T()))
	authEntityProvider.EXPECT().GetAuthEntity(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, _ string) (*sharedauth.PrincipalEntity, error) {
			return sharedauth.GetPrincipalEntityFromAuthContext(ctx)
		})
	r.server.authEntityProvider = authEntityProvider
}

func (r *ClosedNonEmptyConversationsVersionCHTestSuite) requesterContext(requesterUserID string) context.Context {
	claim := jwt.CrestaAccessTokenClaim{
		User: jwt.ClaimUser{Name: userpb.UserName{CustomerID: r.customerID, UserID: requesterUserID}.String()},
	}
	claimJSON, err := json.Marshal(claim)
	r.Require().NoError(err)
	return metadata.NewIncomingContext(r.ctx, metadata.New(map[string]string{"cresta-access-token-claim": string(claimJSON)}))
}

func (r *ClosedNonEmptyConversationsVersionCHTestSuite) insertConversationVersion(row clickhouseshared.ConversationRow) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	r.Require().NoError(err)
	row.CustomerID = r.customerID
	row.ProfileID = r.profileID
	row.CreateTime = row.UpdateTime
	r.Require().NoError(batch.AppendStruct(&row))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, batch))
}

func (r *ClosedNonEmptyConversationsVersionCHTestSuite) insertMessage(conversationID, agentUserID string, startTime, endTime time.Time) {
	ctx := context.Background()
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO message (%s)", clickhouseshared.ExtractCHFields(clickhouseconv.MessageRow{})))
	r.Require().NoError(err)
	r.Require().NoError(batch.AppendStruct(&clickhouseconv.MessageRow{
		AgentUserID:           agentUserID,
		ConversationID:        conversationID,
		ConversationStartTime: startTime,
		ConversationEndTime:   endTime,
		CustomerID:            r.customerID,
		ProfileID:             r.profileID,
		MessageID:             uuid.NewString(),
		CreateTime:            endTime,
		UpdateTime:            endTime,
	}))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, batch))
}

// endedOnJan2 asks for the closed conversations that ended on 2023-01-02 UTC.
func (r *ClosedNonEmptyConversationsVersionCHTestSuite) endedOnJan2() *analyticspb.RetrieveClosedNonEmptyConversationsRequest {
	return &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
		Parent: r.profileResourceName,
		EndTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2023, 1, 2, 23, 59, 59, 0, time.UTC)),
		},
	}
}

func (r *ClosedNonEmptyConversationsVersionCHTestSuite) conversationName(conversationID string) string {
	return fmt.Sprintf("%s/conversations/%s", r.profileResourceName, conversationID)
}

// The requester's conversations are those whose newest version has the requester
// as agent and ends in the range. Filtering the raw rows would return the
// conversation reassigned away from the requester and the one whose end moved past
// the range, through their older versions.
func (r *ClosedNonEmptyConversationsVersionCHTestSuite) TestEndTimeRange_FiltersOnNewestVersion() {
	// Given
	suffix := uuid.NewString()[:8]
	requesterUserID := "requester-" + suffix
	otherAgentUserID := "other-agent-" + suffix
	startTime := time.Date(2023, 1, 2, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 1, 2, 11, 0, 0, 0, time.UTC)
	firstWrite := time.Date(2023, 1, 2, 11, 0, 0, 0, time.UTC)

	keptConversationID := uuid.NewString()
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: requesterUserID, ConversationID: keptConversationID,
		ConversationStartTime: startTime, ConversationEndTime: endTime, UpdateTime: firstWrite,
	})
	r.insertMessage(keptConversationID, requesterUserID, startTime, endTime)

	// Starts two hours earlier than the others, so that it sorts after them.
	reassignedToRequesterID := uuid.NewString()
	reassignedToRequesterStart := startTime.Add(-2 * time.Hour)
	reassignedToRequesterEnd := endTime.Add(-2 * time.Hour)
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: otherAgentUserID, ConversationID: reassignedToRequesterID,
		ConversationStartTime: reassignedToRequesterStart, ConversationEndTime: reassignedToRequesterEnd, UpdateTime: firstWrite,
	})
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: requesterUserID, ConversationID: reassignedToRequesterID,
		ConversationStartTime: reassignedToRequesterStart, ConversationEndTime: reassignedToRequesterEnd, UpdateTime: firstWrite.Add(time.Hour),
	})
	r.insertMessage(reassignedToRequesterID, requesterUserID, reassignedToRequesterStart, reassignedToRequesterEnd)

	reassignedAwayID := uuid.NewString()
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: requesterUserID, ConversationID: reassignedAwayID,
		ConversationStartTime: startTime, ConversationEndTime: endTime, UpdateTime: firstWrite,
	})
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: otherAgentUserID, ConversationID: reassignedAwayID,
		ConversationStartTime: startTime, ConversationEndTime: endTime, UpdateTime: firstWrite.Add(time.Hour),
	})
	r.insertMessage(reassignedAwayID, otherAgentUserID, startTime, endTime)

	// The newer version starts an hour earlier and ends the next day, after the range.
	endMovedConversationID := uuid.NewString()
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: requesterUserID, ConversationID: endMovedConversationID,
		ConversationStartTime: startTime, ConversationEndTime: endTime, UpdateTime: firstWrite,
	})
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: requesterUserID, ConversationID: endMovedConversationID,
		ConversationStartTime: startTime.Add(-time.Hour), ConversationEndTime: endTime.Add(24 * time.Hour), UpdateTime: firstWrite.Add(24 * time.Hour),
	})
	r.insertMessage(endMovedConversationID, requesterUserID, startTime, endTime)

	// When
	resp, err := r.server.RetrieveClosedNonEmptyConversations(r.requesterContext(requesterUserID), r.endedOnJan2())

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.GetConversations(), 2)
	r.Equal(r.conversationName(keptConversationID), resp.GetConversations()[0].GetName())
	r.Equal(endTime, resp.GetConversations()[0].GetEndTime().AsTime())
	r.Equal(r.conversationName(reassignedToRequesterID), resp.GetConversations()[1].GetName())
}

// An end-time range puts no lower bound on the start time, so a conversation that
// started long before the range is still returned.
func (r *ClosedNonEmptyConversationsVersionCHTestSuite) TestEndTimeRange_ReturnsLongConversation() {
	// Given
	requesterUserID := "requester-" + uuid.NewString()[:8]
	startTime := time.Date(2022, 11, 15, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 1, 2, 11, 0, 0, 0, time.UTC)
	conversationID := uuid.NewString()
	r.insertConversationVersion(clickhouseshared.ConversationRow{
		AgentUserID: requesterUserID, ConversationID: conversationID,
		ConversationStartTime: startTime, ConversationEndTime: endTime, UpdateTime: endTime,
	})
	r.insertMessage(conversationID, requesterUserID, startTime, endTime)

	// When
	resp, err := r.server.RetrieveClosedNonEmptyConversations(r.requesterContext(requesterUserID), r.endedOnJan2())

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.GetConversations(), 1)
	r.Equal(r.conversationName(conversationID), resp.GetConversations()[0].GetName())
}

func TestClosedNonEmptyConversationsVersionCH(t *testing.T) {
	suite.Run(t, &ClosedNonEmptyConversationsVersionCHTestSuite{})
}
