package elasticsearch

import (
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestParseConvoFilter_ConversationChannels verifies that conversation channels
// supplied on ConversationFilterParams are translated into a conversation-level
// terms filter on the conversation_channel field.
func TestParseConvoFilter_ConversationChannels(t *testing.T) {
	channels := []int32{int32(1), int32(2)} // CHAT, VOICE
	params := &ConversationFilterParams{
		ConversationChannels: channels,
	}

	filters, _, err := parseConvoFilter(params, nil, false)
	assert.NoError(t, err)

	var found bool
	for _, f := range filters {
		if f.Terms != nil && f.Terms.ConversationChannels != nil {
			assert.Equal(t, channels, *f.Terms.ConversationChannels)
			found = true
		}
	}
	assert.True(t, found, "expected a conversation_channel terms filter in the parsed convo filters")
}

// TestParseConvoFilter_NoConversationChannels verifies that no channel filter is
// emitted when ConversationChannels is empty (i.e. "all channels").
func TestParseConvoFilter_NoConversationChannels(t *testing.T) {
	params := &ConversationFilterParams{}

	filters, _, err := parseConvoFilter(params, nil, false)
	assert.NoError(t, err)

	for _, f := range filters {
		if f.Terms != nil {
			assert.Nil(t, f.Terms.ConversationChannels, "did not expect a conversation_channel filter when no channels are set")
		}
	}
}

func TestParseConvoFilter_RejectsUnsupportedAIAgentFeedback(t *testing.T) {
	// Given
	params := &ConversationFilterParams{
		AIAgentFeedback: analyticspb.Attribute_AIAgentFeedback(999),
	}

	// When
	filters, excludeFilters, err := parseConvoFilter(params, nil, false)

	// Then
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, filters)
	require.Nil(t, excludeFilters)
}
