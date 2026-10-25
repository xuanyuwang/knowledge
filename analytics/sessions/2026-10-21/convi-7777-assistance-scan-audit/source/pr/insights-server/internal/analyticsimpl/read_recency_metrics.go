package analyticsimpl

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/shared-go/framework/stats"
)

// Read-recency instrumentation for the Elasticsearch conversations hot-window sizing work
// (see the "Autonomous Shard-Size Control" design doc).
//
// These metrics answer a single question: how far back in time do conversation reads
// actually reach? That distribution is needed to choose the ES conversations hot-window
// retention size from data instead of a guess. It is not otherwise observable — the
// app-level "Sent ES query" log is gated off in prod (LogQuery flag), and groundcover
// obfuscates the _search request body, so the conversation_started_at lower bound cannot
// be read from logs or traces.
//
// The metrics are PII-free by construction: only a lookback duration (in days) and a
// bounded-vs-all-time counter are emitted, tagged by customer_id, profile_id, and
// read_type. No query text, filter values, conversation content, or user identifiers are
// recorded. Customer/profile tags are attached EXPLICITLY here (this service does not
// inject them via ctx/tags middleware) — they are required so the hot window can be sized
// per customer (whales such as cvs may read far deeper than small customers; a fleet-wide
// blended distribution would be useless for that decision).
var (
	// conversationReadLookbackDaysMetric is the distribution of (now - lower_bound) in days
	// for conversation reads that carry a time bound.
	conversationReadLookbackDaysMetric = stats.NewMetric("insights.conversation_read_lookback_days")
	// conversationReadRangeMetric counts reads by whether they are time-bounded or all-time
	// (all-time == a full-index scan with no lower bound).
	conversationReadRangeMetric = stats.NewMetric("insights.conversation_read_range_total")
)

// recordConversationReadRecency emits how far back a conversation read reaches, tagged by
// customer/profile so the ES hot window can be sized per customer.
//
// from is the lower bound of the read's time window (conversation_started_at.gte). A nil
// from means the read is unbounded / all-time — a full-index scan — which is recorded on
// the all-time counter only (there is no meaningful lookback value). readType identifies
// the calling read path (e.g. the RPC) so paths can be compared.
func recordConversationReadRecency(ctx context.Context, from *timestamppb.Timestamp, customerID, profileID, readType string) {
	tags := []string{"customer_id:" + customerID, "profile_id:" + profileID, "read_type:" + readType}
	if from == nil {
		conversationReadRangeMetric.Incr(ctx, append(tags, "range:all_time")...)
		return
	}
	lookbackDays := time.Since(from.AsTime()).Hours() / 24
	// Guard against clock skew / future-dated bounds so the distribution stays clean.
	if lookbackDays < 0 {
		lookbackDays = 0
	}
	conversationReadLookbackDaysMetric.Distribution(ctx, lookbackDays, tags...)
	conversationReadRangeMetric.Incr(ctx, append(tags, "range:bounded")...)
}
