# CONVI-7852 — retain parsed membership metadata unconditionally

Source repo: `/Users/xuanyu.wang/repos/go-servers`.
Branch/worktree: `xw/convi-7852-analytics-membership-roles`, `/Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles`.
Starting head: `c30fca8d40a4c60d967dbdb40611b9d55c74cbc0`, PR #35094 open, stacked on auth. Worktree clean; remote matches local.

User approved retaining membership metadata unconditionally and agreed memberships should not participate in stats cache keys, with some role freshness delay acceptable. Implement the already-discussed option: build `UserNameToMemberships` for all eligible parsed users regardless of grouping; keep it separate from filter/cache users. No change to current Agent+Group response decoration policy or cache behavior. Add non-Agent grouping coverage and verify ACL filtering and unchanged FinalUsers/request-cache identity. Use test-first failure then full parser target and focused conversation role/cache/team tests. Update and push the existing analytics PR.

## Implementation and test-first evidence

Extended existing ACL/metadata regression into `WithAgentGrouping` and `WithoutAgentGrouping` cases. Both verify only the allowed user's memberships survive and FinalUsers remain the original membership-free user messages. Test-first run failed exactly the new non-Agent case because the membership map was nil; Agent case passed. Removed only the metadata-map construction guard and clarified its comment. Existing group mapping and response-decoration conditions remain as before.

Full common_user_filter_test passed in 1.5s, including both grouping cases. Gazelle passed without additional BUILD changes; gofmt/diff checks pass. Focused conversation role/cache/Team tests running.

## Validation and publication

Full parser target passed in 1.5s. All five focused conversation methods passed in 8.3s, with verbose names confirming candidate-team filtering, missing metadata, fresh role responses, cached role responses, and unchanged Team aggregation. Prior full conversation-suite local ClickHouse memory limit remains documented; no full rerun for this two-file parser-only update. Gazelle and diff checks passed.

Committed/pushed `f05c8789a133d01cb7ee67b1382eee679124f8b0` to existing analytics PR #35094. Push was fast-forward with an explicit lease against old remote `c30fca8d40a4c60d967dbdb40611b9d55c74cbc0`. PR still stacked on auth; description now states unconditional parsed metadata and exclusion from cache identity, with new regression evidence. Source worktree clean. No auth or Director edits. Post-push CI not monitored.

No change to stats cache key construction or role freshness; existing post-cache response enrichment remains. Git/gh used existing authentication, no AWS/Okta/Azure/SSH credential explicitly selected. No production/customer operations or external messages.
