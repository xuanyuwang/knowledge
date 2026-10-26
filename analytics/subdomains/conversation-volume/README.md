# Conversation Volume

## Purpose

Own the surface-specific definitions behind labels such as conversations, completed scorecards, and progression volume.

## Canonical Definitions

- Performance Insights volume is derived from `RetrieveQAScoreStats` / `score_d` and can include all-N/A scorecards when requested.
- Performance Insights progression count requires at least one non-N/A score.
- Closed Conversations uses `RetrieveConversationStats` and requires a non-empty transcript.
- Customer Insights derives volume from Elasticsearch messages.
- Leaderboard volume depends on the API and tab; it must be documented per metric rather than inferred from another surface.
- Performance Insights and Closed Conversations intentionally answer different questions, so their counts are not expected to match exactly.

## Architecture and Source Map

- **Frontend:** Performance Insights, Closed Conversations, Customer Insights, and Leaderboard
- **APIs:** `RetrieveQAScoreStats`, `RetrieveConversationStats`, and tab-specific Leaderboard endpoints
- **Backend/data:** scorecard analytics rows, conversation/transcript data, and Elasticsearch messages

## Operational Knowledge

- First identify the product surface and label; then compare eligibility, score requirement, transcript requirement, time attribution, and filters.
- Do not use one surface as the correctness oracle for another until their predicates have been normalized.

## Legacy Sources and Cases

- `analytics/sessions/2026-10-25/agent-stats-analytics-behaviors-legacy-evidence/deliverables/conversation-count-behavior-guide-2026-07.md`
- [CONVI-6842](../../work-items/CONVI-6842.md)
- [CONVI-7162](../../work-items/CONVI-7162.md)

## Open Questions

- Build a tested predicate matrix for every visible conversation/count label.
- Document time-zone and time-attribution behavior per API.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
