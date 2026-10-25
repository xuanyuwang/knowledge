# CONVI-7777 — Dedicated API and split-query discussion

- Date: 2026-10-13 (America/Toronto).
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`.
- Source context: `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, branch `xw/convi-7777-assistance-usage`; read-only source inspection.
- Inputs: construction HTML/Markdown, API semantics audit, work item, cohort-superset session, current backend and proto worktrees.

## Verified current construction

The first pass extends `RetrieveConversationStats` with `include_assistance_usage`. The actual evidence implementation is `insights-server/internal/analyticsimpl/retrieve_conversation_assistance_usage_clickhouse.go:21`. It builds three union branches over moments, use events, and summary-use events, plus valid summary annotations. Evidence scans are bounded by conversation start time; the final existing cohort join applies request eligibility. The SQL deduplicates conversation/agent keys and the existing Agent/Team aggregation carries the assisted count. The current stats response exposes aggregates, not the base conversation IDs. Runtime, dependency integration, and latency remain unverified.

## User proposal and recommendation for discussion

The user proposes a dedicated metric API: obtain base population keys, query feature-use keys with smaller queries, union those keys, intersect with the base population, and calculate the metric server-side. This is semantically viable and does not require a single large SQL statement.

Recommended API boundary for discussion: one dedicated public metric RPC, reusing a shared internal cohort-selection helper from ConversationStats, with internal feature-evidence query helpers. Reusing eligibility does not require calling the aggregate RPC over the network or creating one public API per feature. Preserve ACL, source/bot rules, message-bearing eligibility, metadata filters, time buckets, and existing Team attribution. Use `(conversation_id, agent_user_id)` within the customer/profile scope; conversation IDs alone lose transfer attribution. Feature IDs can annotate breakdowns, but must not multiply the overall numerator.

Suggested execution: resolve the cohort once, run evidence queries with bounded concurrency, deduplicate and intersect incrementally in Go, and return denominator/assisted counts by agent/team/time bucket. Feature queries retain their use predicates while the shared base defines eligibility. Restrict evidence to eligible keys when supported efficiently, using an external table or database-side join rather than a huge literal IN list. This transport choice requires profiling. Repeated queries and independent reads introduce load and freshness considerations; multiple reads do not imply a common database snapshot.

Smaller statements are easier to instrument and tune but are not proven faster. Several features share `conversation_event_d`; separate feature queries may repeat scans that the current combined event branch performs once. Compare the present query, per-feature queries, and queries grouped by physical source. Measure representative ranges/cohort sizes, metadata cases, p50/p95 latency, read rows/bytes, ClickHouse and Go peak memory, transferred key volume, and concurrent-request load. No benchmark or production query ran in this session.

For an exact metric, a required query failure must fail the request or explicitly mark it incomplete; missing evidence cannot silently become zero. Cancellation/deadlines should cover all workers. Large cohorts may need streaming/batching or database-side intersection instead of an unbounded in-memory set.

## State and follow-up

### Subsequent clarification and source check

Latest clarification supersedes the initial Team deduplication proposal below: the user chooses existing Team behavior (sum agent counts, including shared conversations within one team) and conversation-start-time selection/buckets. Overall/no-group counting remains under clarification.

Verified no-Agent/no-Group requests take the direct query path rather than the Team converter (e.g. `retrieve_suggestion_stats.go:76-98`, `retrieve_summarization_stats.go:69-92`). Suggestions, Notes, KB search, Guided Workflow, Summary, and ConversationStats use `COUNT(DISTINCT conversation_id)` at the SQL grouping scope. With only frequency, that means distinct IDs per time bucket; it is not a sum of agent counts. Hints performs branch-level distinct conversation counts then combines sent branches with MAX (`retrieve_hint_stats_clickhouse.go:310-319,355-365`), so its denominator is not a general union of all hint conversations. Knowledge Assist/GenAI has session/action metrics, not an equivalent conversation denominator. Overall totals from Agent/Team-grouped requests must not be treated as the direct ungrouped metric.

Verified the seven V1 feature queries call both `parseClickhouseFilter` and `parseClickhouseGroupBy` without time overrides. `getConversationTimeRangeColumn` defaults to conversation start (`common_clickhouse.go:2054`); frequency buckets use that selector (`common_clickhouse.go:1246-1249`). Their target tables retain the physical conversation_start_time column, including events-cluster conversation events. ConversationStats also defaults to start, with an explicit end-time option. This is not universal across all analytics APIs: user/analytics-event tables map the default logical column to event_time, and scorecard tables map it to scorecard/formatted time (`common_clickhouse.go:1881-1930`).

The user confirms the column belongs beside "Convos powered by AA %" on Agent and Team leaderboards; the existing `assistance-used-chart` also needs a time-bucket trend. Agent rows retain separate participation. The intended Team and overall metrics deduplicate conversation IDs. In the cross-team transfer example, Alice on Team A uses assistance and Bob on Team B does not: only Team A gets numerator credit, both teams get denominator credit, and the overall metric counts one assisted conversation.

Current feature APIs do not establish the proposed Team distinct-conversation rule. Hints (`retrieve_hint_stats.go:325-326`), Notes (`retrieve_note_taking_stats.go:191-192`), and Suggestions (`retrieve_suggestion_stats.go:186-189`) sum per-agent usage and conversation counts into groups through `userNameToGroupNamesMap`. Summary, Knowledge Assist, KB search, and Guided Workflow follow the same per-agent-to-group structure. Their aggregation credits the using agent's groups rather than propagating use to every team that handled that conversation; physical row attribution still depends on each query. Two agents on one team can contribute the same conversation twice because group conversion has already lost the IDs. Ungrouped query paths can use SQL `COUNT(DISTINCT conversation_id)`, so totals should not be assumed to equal summed agent/team rows. The new API must retain raw keys until deduplication at the requested aggregation scope. Existing feature APIs remain unchanged.

This is an architectural discussion, not an approved replacement design. No product code, proto, frontend, tests, external systems, or Git history changed. Confirm the API/query direction before refactoring the first pass; retain existing V1/P2 scope and unresolved predicate decisions. No AWS, Okta, Azure, or SSH credentials/profiles were selected or used.
