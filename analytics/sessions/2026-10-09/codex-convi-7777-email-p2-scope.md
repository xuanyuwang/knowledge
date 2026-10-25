# CONVI-7777 — email conversation linkage and P2 scope

**Date:** 2026-10-09
**Source repo / branch:** `/Users/xuanyu.wang/repos/go-servers`, `origin/main` at `48d308822e90e17d2ca190987220601601ee9d97` (read-only source snapshot). Related Director and cresta-proto source references are in the [PM inventory](../../deliverables/convi-7777-pm-feature-inventory.md).

## PM input and conclusion

EA asked whether email Smart Compose and Suggestions can be tied to conversations and said she is happy to move all email use cases to P2. The linkage premise needs correction: partial/full email Suggestions have used conversation events and are already counted by `RetrieveSuggestionStats`. Email Smart Compose and inserted search-result events also have conversation-level contracts, but current Assistance Insights stats do not query them. Adding those two to a future conversation-level metric needs a new projection/query path and an agreed qualifying-use rule. Exclude **all email use cases** from the proposed v1 feature union per EA's prioritization preference; retain the source evidence under P2.

## Evidence and limits

- [Suggestions query](https://github.com/cresta/go-servers/blob/48d308822e90e17d2ca190987220601601ee9d97/insights-server/internal/analyticsimpl/retrieve_suggestion_stats_clickhouse.go#L55-L96): `conversation_event_d` rows with `conversation_id`; email used event DB types 25 and 27 are counted separately from chat types 4/5/10. Viewed-only email events are not the used predicate.
- [Chat Smart Compose query](https://github.com/cresta/go-servers/blob/48d308822e90e17d2ca190987220601601ee9d97/insights-server/internal/analyticsimpl/retrieve_smart_compose_stats_clickhouse.go#L39-L99): current metric reads legacy analytics events and uses an agent/hour association, not an exact conversation key. Newer conversation-linked events could support a different implementation.
- [Earlier email source audit](codex-convi-7777-email-and-silence-hold.md): email Smart Compose and search insertion contracts and ingestion exist; neither is currently counted by an Insights stats query, and the conversations-cluster projection path needs additional work.
- No production rows, historical event coverage, filtering parity, or deployed behavior were checked here. The distinction is source-level technical linkage versus the currently displayed metric.

## Reply status

Draft only. Nothing was posted to EA or Linear. The [PM inventory](../../deliverables/convi-7777-pm-feature-inventory.md), [work item](../../work-items/CONVI-7777.md), API audit, domain README, and daily log were aligned with the P2 preference.
