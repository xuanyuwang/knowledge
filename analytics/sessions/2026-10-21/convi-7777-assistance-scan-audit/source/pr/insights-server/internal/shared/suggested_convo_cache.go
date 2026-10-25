package shared

import (
	"strings"
	"time"

	"github.com/cresta/shared-go/container/cache"
	"github.com/cresta/shared-go/util/envflag"
)

var (
	SuggestedConvoCacheTimeout        = envflag.Duration("SUGGESTED_CONVO_CACHE_TIMEOUT", 120*time.Minute, "The cache timeout duration for suggested conversations")
	SuggestedConvoCacheCleanupTimeout = envflag.Duration("SUGGESTED_CONVO_CACHE_CLEANUP_TIMEOUT", 120*time.Minute, "The cache cleanup timeout duration for suggested conversations.")
)

type SuggestedConvoCache struct {
	ConversationSuggested cache.Cache[string, string]
}

func (c *SuggestedConvoCache) SetConversationSuggested(scorecardTemplateID, conversationID, messageID, agentUserID, reviewerUserID string) {
	cacheKey := getSuggestedConvoCacheKey(scorecardTemplateID, conversationID, messageID, agentUserID)
	c.ConversationSuggested.Set(cacheKey, reviewerUserID)
}

func (c *SuggestedConvoCache) GetConversationSuggested(scorecardTemplateID, conversationID, messageID, agentUserID string) (string, bool) {
	cacheKey := getSuggestedConvoCacheKey(scorecardTemplateID, conversationID, messageID, agentUserID)
	reviewerUserID, exists := c.ConversationSuggested.Get(cacheKey)
	return reviewerUserID, exists
}

func getSuggestedConvoCacheKey(scorecardTemplateID, conversationID, messageID, agentUserID string) string {
	return strings.Join([]string{scorecardTemplateID, conversationID, messageID, agentUserID}, ":")
}
