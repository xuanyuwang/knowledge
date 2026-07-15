# Shared Analytics Platform

## Purpose

Own the contracts shared by Performance Insights and Leaderboard: analytics-service APIs, request/grouping semantics, FE request construction, source tables, attribution, and cross-page operational behavior.

## Shared Contracts

- Analytics responses can contain pre-aggregated summary fields alongside grouped `resultGroups`; `groupBy` controls breakdown shape and does not necessarily remove response-level aggregates.
- Non-QA APIs use `AttributeStructure`; QA APIs use `QAAttributeType`. Similar-looking grouping values are distinct proto systems.
- Performance Insights is primarily QA/`RetrieveQAScoreStats`; Leaderboard combines agent, conversation, assistance, QA, coaching, comment, and scorecard statistics.
- Filters must be traced end-to-end: FE state/default/persistence → request field → backend user/data filtering → grouping/aggregation → display.
- ClickHouse is the main analytical source, but exact tables and eligibility vary by API; Elasticsearch and Postgres still back some surfaces.

## Architecture and Source Map

- **Frontend:** Director Insights filter hooks, request-param builders, API hooks, and page components
- **API contract:** `cresta-proto/cresta/v1/analytics/analytics_service.proto`
- **Backend:** `go-servers/insights-server/internal/analyticsimpl/`
- **Shared filter logic:** `go-servers/shared/user-filter/` and analytics common-filter helpers
- **Data:** `conversation_d`, `message_d`, `conversation_with_labels_d`, `scorecard_d`, `score_d`, plus API-specific sources

## Legacy Sources

- `agent-stats-analytics-behaviors/`
- `insights-user-filter/analytics-apis-list.md`
- `insights-user-filter/fe-group-by-usage-patterns.md`
- `user-filter-consolidation/`

## Open Questions

- Which API contracts and source tables are current versus legacy split-API paths?
- Which shared time, identity, and denominator semantics need explicit invariant tests?
- Which cross-page differences are intentional product semantics versus drift?
