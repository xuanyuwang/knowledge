## Summary

Implement `RetrieveConversationAssistanceUsageStats`:
1. first, we query the all eligible conversations and use `(convo_id, agent id, convo start time)` as key.
2. Reuse the shared query of each assistance feature to get individual assistance usage keyed by `convo_id, agent id)`
3. union all assistance feature usage per `(convo id, agent id)` to calculate if there is any assistance used for the `(convo id, agent id)` group
4. optionally, break the result in different time buckets per FE request to show daily trend

Overall counts deduplicate conversation IDs; Agent rows count distinct conversations per agent; Team rows sum member-agent counts, including shared conversations and overlapping membership. DAILY/WEEKLY buckets use population conversation start time and existing timezone/week semantics. Explicit agent/activity restrictions remain applied even when the shared unrestricted-user optimization would omit them. Population reads force external user tables for these restrictions, even when the shared external-table flag is off, preventing large resolved user directories from exceeding the SQL query-size limit.

The API uses two production files: the action file contains its existing handler/population code plus validation and aggregation; `_clickhouse.go` contains evidence queries and database reads.

Depends on #34044 for the final Summary/GenAI helpers. Existing feature predicates and feature RPCs remain unchanged. Conversation details explicitly return `Unimplemented`; Director is outside this PR. Feature sources receive a deduplicated eligible-agent filter; each family reduces to conversation/agent pairs before an ordinary join with the external eligible population. This replaces GLOBAL IN cohort membership. Seven queries still upload the cohort to the initiator, with concurrency limited to three and no timestamp partition bounds. Selected-agent historical pairs can increase transfer before intersection; production latency, network volume and memory remain unmeasured.

### Counting example

Eligible conversation/agent pairs are `shared/Alice`, `shared/Bob` and `unused/Alice`. Only Alice uses assistance in `shared`. Repeated events and multiple qualifying features still count that pair once.

| Result | Total conversations | With assistance use | Reason |
| --- | --- | --- | --- |
| Overall | 2 | 1 | Distinct conversation IDs |
| Alice | 2 | 1 | Distinct conversations for Alice |
| Bob | 1 | 0 | Alice’s use gives Bob no credit |
| Team containing Alice + Bob | 3 | 1 | Sum of agent counts; `shared` counts twice in the denominator |
| Another Team containing Alice | 2 | 1 | Membership credited independently |

Aggregation tests cover this scenario, including overlapping Team membership and daily buckets. Optional time buckets use the population’s conversation start time and the same counting rules.

## Test plan

- [x] Full API Bazel suite with fresh isolated ClickHouse containers: `bazel test //insights-server/internal/analyticsimpl:action_retrieve_conversation_assistance_usage_stats_test`.
- [x] Focused race checks: `go test -race ./insights-server/internal/analyticsimpl -run '^TestConversationAssistanceUsage($|Validation)' -count=1`.
- [x] Real ClickHouse coverage for all ten V1 categories, exclusions, duplicate/overlapping evidence, wrong-agent evidence, stale copied timestamps, user/ACL restrictions, message-bearing eligibility and bot policy.
- [x] Complete response comparisons for Agent/Team attribution, overlapping memberships, daily/weekly buckets and empty results; required query/scan/iteration failures discard the response.
- [x] No-GLOBAL guard (red before fix), plus real ClickHouse coverage for multiple eligible agents, duplicate population membership, excluded agents and outside conversations.
- [x] Disposable two-node/two-shard ClickHouse 23.8.16.40 check: all seven generated queries run with `distributed_product_mode=deny`; exact pair matching and Summary generation/copy across shards pass. Plans place the population join above remote reduction; captured remote SQL has no population-table reference.
- [x] Agents-only and active-only requests with 8,001 eligible users and the shared external-table flag off: regression fails before the fix and passes after. Small and empty restricted audiences retain their expected results.
- [x] Gazelle, gofmt and `git diff --check`.
- [ ] Staging integration and representative production-scale latency/read-volume/memory benchmarks.

## AI knowledge files

- [x] Updated `insights-server/internal/analyticsimpl/CLAUDE.md` for the complete Phase I counts pipeline and deferred details.
- No domain documentation under `docs/` changed.
