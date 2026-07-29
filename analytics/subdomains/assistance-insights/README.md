# Assistance Insights

## Purpose

Own Assistance Insights frontend/backend semantics, filters, metrics, hint engagement, grouping, and displayed-data lineage.

Assistance Insights is distinct from Performance Insights:

- Performance Insights primarily reports scorecard and QA criterion results through `RetrieveQAScoreStats`.
- Assistance Insights reports usage and engagement with agent-assistance features through several analytics APIs.
- Similar behavior or policy names do not make the resulting percentages directly comparable.

## Primary APIs

- `RetrieveHintStats` — Behavioral, Reminder, KB, Guided Workflow, and Checklist hint counts and engagement.
- `RetrieveAssistanceStats` and split feature-specific assistance APIs — aggregate assistance usage.
- `RetrieveKnowledgeAssistStats` — knowledge assistance metrics.
- `RetrieveConversationStats` — conversation populations and assisted-conversation rates.
- `RetrieveAdherences` — Silence/Hold hint adaptation path.

Canonical API reference:

- [RetrieveHintStats](../shared-analytics-platform/retrieve-hint-stats.md)

## Current Semantics

- Hint engagement is generally rendered as `totalHintFollowedCount / totalHintSentCount`.
- Reminder hints surface sent volume rather than followed rate.
- Assistance filters use conversation start time and can include user/team/group, duration, metadata/moment-group, use-case, and agent-only constraints.
- Agent/policy and group/policy leaderboards intentionally omit frequency to request aggregate rows.
- Group rows are built from per-agent data and can overlap when users belong to multiple selected groups.
- Frontend display paths are inconsistent about clamping ratios to 100%; clamping must not substitute for correct API counting semantics.

## Architecture and Source Map

**Director**

- Page: `packages/director-app/src/features/insights/assistance/AssistanceInsights.tsx`
- Container: `packages/director-app/src/components/insights/assistance/assistance-insights-container/AssistanceInsightsContainer.tsx`
- Statistics tabs: `packages/director-app/src/components/insights/assistance/assistance-insights-statistics/`
- Hint hooks: `packages/director-app/src/components/insights/hooks/useHintStats.ts` and `useGetHintStatsByHintType.tsx`
- Shared request builder: `packages/director-app/src/components/insights/hooks/util-hooks/useInsightsRequestParams.ts`

**Backend**

- Analytics implementation: `go-servers/insights-server/internal/analyticsimpl/`
- Hint stats handler: `retrieve_hint_stats.go`
- Hint stats ClickHouse path: `retrieve_hint_stats_clickhouse.go`

**Data**

- `moment_annotation_d`
- `action_annotation_d`
- `conversation_event_d`
- additional API-specific conversation and assistance tables

## Known Risks

- Behavioral `RetrieveHintStats` currently mixes sent-action and followed-moment grains, allowing engagement above 100%.
- Assistance and Performance metrics can share a display name while measuring different populations.
- Response-wide totals, grouped result totals, and time-bucket entries must not be interchanged.
- Group membership overlap can make totals non-additive across teams/groups.
- Different hint types use different source tables and followed-event definitions.

## Active Case

- [Heartland Behavior Hints adherence mismatch](../../work-items/heartland-behavior-hints-adherence-mismatch.md) — source data is valid; the current behavioral hint query divides positive moments by sent actions and can return rates above 100%.

## Open Questions

- Confirm the product definition of “followed” for every hint type.
- Establish API-level invariants for percentage-producing metrics.
- Catalog all split-assistance APIs and the feature flags selecting old versus split paths.
- Document exact conversation-population differences among Assistance cards, charts, tables, and drawers.
