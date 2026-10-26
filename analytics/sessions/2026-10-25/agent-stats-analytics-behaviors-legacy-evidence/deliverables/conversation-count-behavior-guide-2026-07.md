# Conversation Count Behavior Guide

Last reviewed: 2026-07-06

## Purpose

This guide explains what each "conversation count" means across Insights surfaces. These counts use different definitions and data sources by design.

**Decision:** PM agreement on [CONVI-6842](https://linear.app/cresta/issue/CONVI-6842).

**Fix:** [director#20153](https://github.com/cresta/director/pull/20153) (merged 2026-07-06) — no-template Performance Insights Conversation volume now uses `RetrieveQAScoreStats` with `includeNaScored: true`, matching selected-template behavior.

**Coda source:** [What does Conversation Count mean across Insights pages?](https://coda.io/d/_doM9F-e3jUe/What-does-Conversation-Count-mean-across-Insights-pages_suvFVqLd)

## Summary Table

| Surface | API | Data source | Definition | Holiday Inn Apr 15 example |
|---|---|---|---|---|
| PI Conversation volume (no template) | `RetrieveQAScoreStats` | `score_d` | Distinct conversations with a scorecard, including all-N/A scorecards | 639 |
| PI Conversation volume (selected template) | `RetrieveQAScoreStats` | `score_d` | Distinct conversations with the scorecard template assigned, including all-N/A scorecards | 639 (with equivalent audience scope) |
| PI Performance progression `# of convo` | `RetrieveQAScoreStats` | `score_d` | Scorecarded conversations with at least one non-N/A score | 592 |
| Closed Conversations | `RetrieveConversationStats` | `message_d` + `conversation_d` | Scorecarded conversations with non-empty message/transcript rows | 611 |
| Customer Insights conversation count | Elasticsearch query | `message` documents | Non-empty conversation count | Varies by ES/CH sync |

PI and Closed Conversations are **intentionally allowed to differ**.

## Surface Definitions

### a) Conversation volume for ${scorecard template}

- Reads from `score_d` in ClickHouse via `RetrieveQAScoreStats`.
- Counts distinct conversations with the scorecard template assigned.
- Includes conversations with all N/A scores.

### b) Conversation volume for No template

- Reads from `score_d` in ClickHouse via `RetrieveQAScoreStats` (after director#20153).
- Counts distinct conversations with a scorecard assigned.
- Includes conversations with all N/A scores.
- Matches case (a) when template audience scope is equivalent.

**Director implementation:** `ConversationCountChart` uses `useQAScoreStats` for both template and no-template paths. `PerformanceConversations` passes `filtersWithNAValues` so `includeNaScored: true`.

### c) `# of Convo` column in Performance progression

- Reads from `score_d` via `RetrieveQAScoreStats`.
- Counts conversations with at least one non-N/A score in scorecards.
- Stricter than Conversation volume because all-N/A scorecarded conversations are excluded.

### d) Conversation count in Customer Insights

- Reads from `message` documents in Elasticsearch.
- Non-empty conversation count (excludes conversations without messages).

### e) Closed Conversations

- Reads from `message_d` joined with `conversation_d` via `RetrieveConversationStats`.
- Scorecarded conversations with non-empty message/transcript rows.
- Stricter than PI Conversation volume by design.

## FAQs

### Why do different scorecard templates show different Conversation volume counts?

Different scorecard templates can have different audience scopes.

### Why did no-template and selected-template PI volume differ before the fix?

Before director#20153, no-template volume used `RetrieveConversationStats` (`message_d`) while selected-template volume used `RetrieveQAScoreStats` (`score_d`). After the fix, both paths are scorecard-backed when audience scope is equivalent.

### Why does Conversation volume differ from Performance progression `# of convo`?

Conversation volume includes all-N/A scorecards. The progression column excludes conversations where every score is N/A.

### Why does Closed Conversations differ from PI Conversation volume?

PI volume is scorecard-centric and includes N/A scorecards. Closed Conversations additionally requires non-empty message/transcript rows. Scorecarded conversations without message rows can appear in PI volume but not in Closed Conversations.

### Why can Customer Insights and Closed Conversations differ slightly?

Both target non-empty conversations, but Customer Insights reads Elasticsearch while Closed Conversations reads ClickHouse. Small deltas can occur from dedup or deletion timing differences.
