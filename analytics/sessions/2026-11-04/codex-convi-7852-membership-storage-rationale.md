# CONVI-7852 — explain analytics membership storage

Source repo: `/Users/xuanyu.wang/repos/go-servers`.
Branch/worktree: `xw/convi-7852-analytics-membership-roles`, `/Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles`.
Reviewed PR #35094 head `c30fca8d40a4c60d967dbdb40611b9d55c74cbc0`, stacked on auth. User asked why membership retention is Agent-conditional and separate from FinalUsers. Read-only explanation; no source or PR changes.

## Confirmed source path

- `common_user_filter.go:345`: membership map is populated only when grouping includes AGENT. This flag means grouping dimension, not Agent-role eligibility. Metadata already arrives on LiteUsers independently; guard merely avoids retaining an extra map for other callers. Only current consumer is conversation stats Agent+Group response enrichment (`retrieve_conversation_stats.go:85`). There is no data correctness requirement to condition map construction on AGENT.
- `common_user_filter.go:705`: ApplyUserFilterFromResult assigns FinalUsers to request FilterByAttribute.Users except all-users/no-external-table mode, where request users are empty.
- `retrieve_conversation_stats.go:93`: clones the mutated request for stats cache; sorting only orders user/group messages, without stripping metadata.
- `shared/requestcache/request_cache_helper.go`: hashes request via protobuf.HashProto, persists full request JSON, compares full parsed proto via proto.Equal on hit. HashProto deterministically marshals the complete message (`shared/protobuf/protos.go:87`). Thus adding memberships to FinalUsers also adds them to relevant cache keys and persisted cache request payloads. Role edits could miss the metric cache even though eligible users and SQL metrics are unchanged. Existing names/display names already participate; this PR avoids adding membership state to that coupling.
- SQL user filtering reads User.Name and extracts user IDs (`common_clickhouse.go:417`); membership fields do not themselves change the predicate. Comment about entering filters must not imply memberships change SQL selection.
- Conversation response builder (`retrieve_conversation_stats_clickhouse.go:192`) copies only FullName/Username from the supplied users, so filling FinalUsers alone would not automatically expose memberships in response rows. Current response enrichment runs after metric cache storage and on hits, using parsed user metadata (subject to auth's cache freshness). Stats cache default TTL is four hours (`api_cache.go:57`).

## Assessment

Separate map is an implementation choice to preserve existing shared request/cache behavior; not a fundamental requirement. It is possible to always populate memberships on FinalUsers, but first separate rich user metadata from filter/cache identity (e.g., canonicalize filter users to names only) and explicitly handle response propagation and cache-hit enrichment. Such normalization would affect shared parser callers and require targeted equivalence/cache coverage. No broader change approved in this discussion. Agent-conditional map retention could independently be removed without changing cache keys; retain response enrichment policy separately if only Agent+Group is intended.
