package retrieveclosednonemptyconversations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

func TestConvertClickhouseResponseToRetrieveClosedNonEmptyConversationsResponse(t *testing.T) {
	t.Run("EmptyRows", func(t *testing.T) {
		rows := []ConversationMessageCountRow{}

		result, err := ConvertClickhouseResponseToRetrieveClosedNonEmptyConversationsResponse(rows, &shared.CustomerProfile{
			CustomerID: "test-customer",
			ProfileID:  "test-profile",
		}, 10, 0)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Conversations)
		assert.Equal(t, int64(0), result.TotalConversationCount)
		assert.Empty(t, result.NextPageToken)
	})

	t.Run("SingleRow", func(t *testing.T) {
		startTime := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
		endTime := time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)

		rows := []ConversationMessageCountRow{
			{
				ConversationID:        "conv-123",
				ConversationStartTime: startTime,
				ConversationEndTime:   endTime,
				MessageCount:          1,
			},
		}

		result, err := ConvertClickhouseResponseToRetrieveClosedNonEmptyConversationsResponse(rows, &shared.CustomerProfile{
			CustomerID: "test-customer",
			ProfileID:  "test-profile",
		}, 10, 0)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Conversations, 1)
		assert.Equal(t, int64(1), result.TotalConversationCount)
		assert.Empty(t, result.NextPageToken)

		conv := result.Conversations[0]

		// Check conversation name
		expectedName := convopb.ConversationName{
			CustomerID:     "test-customer",
			ProfileID:      "test-profile",
			ConversationID: "conv-123",
		}.String()
		assert.Equal(t, expectedName, conv.Name)

		// Check timestamps
		assert.Equal(t, startTime, conv.StartTime.AsTime())
		assert.Equal(t, endTime, conv.EndTime.AsTime())

		// Check extended info
		assert.NotNil(t, conv.ExtendedInfo)
		assert.Equal(t, int32(1), conv.ExtendedInfo.MessageCount)
	})

	t.Run("MultipleRows", func(t *testing.T) {
		startTime1 := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
		endTime1 := time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)
		startTime2 := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		endTime2 := time.Date(2023, 1, 1, 13, 0, 0, 0, time.UTC)

		rows := []ConversationMessageCountRow{
			{
				ConversationID:        "conv-123",
				ConversationStartTime: startTime1,
				ConversationEndTime:   endTime1,
				MessageCount:          1,
			},
			{
				ConversationID:        "conv-456",
				ConversationStartTime: startTime2,
				ConversationEndTime:   endTime2,
				MessageCount:          2,
			},
		}

		result, err := ConvertClickhouseResponseToRetrieveClosedNonEmptyConversationsResponse(rows, &shared.CustomerProfile{
			CustomerID: "test-customer",
			ProfileID:  "test-profile",
		}, 10, 0)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Conversations, 2)
		assert.Equal(t, int64(2), result.TotalConversationCount)
		assert.Empty(t, result.NextPageToken)

		// Check that both conversations are properly converted
		convNames := []string{result.Conversations[0].Name, result.Conversations[1].Name}
		expectedName1 := convopb.ConversationName{
			CustomerID:     "test-customer",
			ProfileID:      "test-profile",
			ConversationID: "conv-123",
		}.String()
		expectedName2 := convopb.ConversationName{
			CustomerID:     "test-customer",
			ProfileID:      "test-profile",
			ConversationID: "conv-456",
		}.String()
		assert.Contains(t, convNames, expectedName1)
		assert.Contains(t, convNames, expectedName2)
	})
}
