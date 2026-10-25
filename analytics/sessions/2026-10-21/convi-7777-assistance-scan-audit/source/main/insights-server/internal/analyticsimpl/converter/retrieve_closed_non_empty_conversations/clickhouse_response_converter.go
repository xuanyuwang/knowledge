package retrieveclosednonemptyconversations

import (
	"strconv"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

type ConversationMessageCountRow struct {
	ConversationID        string    `ch:"conversation_id"`
	ConversationStartTime time.Time `ch:"conversation_start_time"`
	ConversationEndTime   time.Time `ch:"conversation_end_time"`
	MessageCount          uint64    `ch:"message_count"`
}

func ConvertClickhouseResponseToRetrieveClosedNonEmptyConversationsResponse(conversations []ConversationMessageCountRow, parent *shared.CustomerProfile, pageSize, offset int) (*analyticspb.RetrieveClosedNonEmptyConversationsResponse, error) {
	conversationsResponse := make([]*convopb.Conversation, 0, len(conversations))
	// Populate extended info for each conversation
	for _, conversation := range conversations {
		conversationsResponse = append(conversationsResponse, &convopb.Conversation{
			Name: convopb.ConversationName{
				CustomerID:     parent.CustomerID,
				ProfileID:      parent.ProfileID,
				ConversationID: conversation.ConversationID,
			}.String(),
			ExtendedInfo: &convopb.Conversation_ExtendedInfo{
				MessageCount: int32(conversation.MessageCount),
			},
			StartTime: shared.ConvertTimeToAPITimestamp(conversation.ConversationStartTime),
			EndTime:   shared.ConvertTimeToAPITimestamp(conversation.ConversationEndTime),
		})
	}

	// Return response
	return &analyticspb.RetrieveClosedNonEmptyConversationsResponse{
		Conversations:          conversationsResponse,
		TotalConversationCount: int64(len(conversationsResponse)),
		NextPageToken:          generateNextPageToken(conversationsResponse, pageSize, offset),
	}, nil
}

func generateNextPageToken(convos []*convopb.Conversation, pageSize int, offset int) string {
	nextPageToken := ""
	if len(convos) > 0 && len(convos) == pageSize {
		nextPageToken = strconv.Itoa(offset + pageSize)
	}
	return nextPageToken
}
