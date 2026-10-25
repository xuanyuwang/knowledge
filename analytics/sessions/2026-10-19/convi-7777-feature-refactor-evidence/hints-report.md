# CONVI-7777 Hints extraction — consolidated verification

Status: verified. Tests commit `3dab1f940dd30546b802b449915645225703566d`; refactor commit `77a9aec2bdaba22ff3e362b6c9964415b69d06f7`. Existing PRs remain [tests #34037](https://github.com/cresta/go-servers/pull/34037) and [refactor #34038](https://github.com/cresta/go-servers/pull/34038).

The existing `RetrieveHintStatsTestSuite` now seeds real ClickHouse and owns every frozen reader regression body. There is one runnable test, one generated large/long Bazel target, and no Hints SQL goldens or separate integration test file. The tests-only branch retains the original production implementation. The child branch contains the same two production Go files as the previously verified extraction, byte for byte.

## Verification

- Original production: 82 named test nodes plus one package node (83 total), zero failures, 389.661 seconds.
- Extracted production: the same 82 named test nodes plus one package node (83 total), zero failures, 390.892 seconds.
- Consolidated test SHA-256: `c2b7032c62b61b0ee987ad41a0c443f8b1771130314dd6759c5987d0d7deb519`; identical in both branches.
- All 16 prior regression-test method bodies are unchanged after receiver rename. Their per-method hashes are retained in `hints-report.frozen-body-equivalence.json`.
- Generated BUILD metadata and `git diff --check` pass in both branches. The standalone Bazel target was not executed by this agent.
- The supplemental seven-node raw-relation execution evidence remains applicable because the production source bytes are unchanged; it was not rerun during consolidation.

Both final runs use:

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '^TestRetrieveHintStats$' -count=1 -timeout 20m -json
```

The first calibration was interrupted by a confirmed Docker OOM and exit 137 while other suites ran concurrently. Final evidence comes from the successful reruns; the failed log and minimal Docker event evidence are retained.

## Full RPC coverage and replacement mapping

The 32 new full-RPC matrix cases execute both delivery modes and assert complete responses: selected/all users, selected groups, inline/external user filters, user/group resolution and enrichment, policy filtering/display, duration range boundaries, behavior filtering, group/agent/policy combinations, bot inclusion, metadata inclusion/exclusion, daily bounds, and Sunday/Monday weekly Honolulu buckets. Three validation cases preserve missing-time-range and unsupported-grouping failures. Dependency calls use exact protobuf matchers.

| Previous mocked case | Seeded replacement |
| --- | --- |
| `TestRetrieveHintStats/TestCases/GroupByAgentAndGroup` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_GroupByAgentAndGroup` |
| `TestRetrieveHintStats/TestCases/GroupByGroup` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_GroupByGroup` |
| `TestRetrieveHintStats/TestCases/GroupAndFilterByBehavior` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_GroupAndFilterByBehavior` |
| `TestRetrieveHintStats/TestCases/GroupByAgentPolicy_NotGroupByTime` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_GroupByAgentPolicy_NotGroupByTime` |
| `TestRetrieveHintStats/TestCases/GroupByGroupPolicy_NotGroupByTime` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_GroupByGroupPolicy_NotGroupByTime` |
| `TestRetrieveHintStats/TestCases/GroupByAgentGroupPolicy_NotGroupByTime` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_GroupByAgentGroupPolicy_NotGroupByTime` |
| `TestRetrieveHintStats/TestCases/WithoutFilterByTimeRange_Throw` | `TestRetrieveHintStats/TestRPCRequestValidation/WithoutFilterByTimeRange_Throw` |
| `TestRetrieveHintStats/TestCases/TooManyGroupByAttrs_Throw` | `TestRetrieveHintStats/TestRPCRequestValidation/TooManyGroupByAttrs_Throw` |
| `TestRetrieveHintStats/TestCases/TooManyGroupByAttrsWithTime_Throw` | `TestRetrieveHintStats/TestRPCRequestValidation/TooManyGroupByAttrsWithTime_Throw` |
| `TestRetrieveHintStats/TestCases/IncludeBotConversations` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_IncludeBotConversations` |
| `TestRetrieveHintStats/TestCases/FilterByMetadataMomentGroups_IncludeAndExcludeMoments` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actionfalse_FilterByMetadataMomentGroups_IncludeAndExcludeMoments` |
| `TestRetrieveHintStatsUseActionAnnotation/TestCases/GroupByAgentAndGroup` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actiontrue_GroupByAgentAndGroup` |
| `TestRetrieveHintStatsUseActionAnnotation/TestCases/GroupByGroup` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actiontrue_GroupByGroup` |
| `TestRetrieveHintStatsUseActionAnnotation/TestCases/GroupAndFilterByBehavior` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actiontrue_GroupAndFilterByBehavior` |
| `TestRetrieveHintStatsUseActionAnnotation/TestCases/GroupByAgentGroupPolicy_NotGroupByTime` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actiontrue_GroupByAgentGroupPolicy_NotGroupByTime` |
| `TestRetrieveHintStatsUseActionAnnotation/TestCases/IncludeBotConversations` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actiontrue_IncludeBotConversations` |
| `TestRetrieveHintStatsUseActionAnnotation/TestCases/FilterByMetadataMomentGroups_IncludeAndExcludeMoments` | `TestRetrieveHintStats/TestRPCUserGroupPolicyAndTimeCoverage/Actiontrue_FilterByMetadataMomentGroups_IncludeAndExcludeMoments` |
| `TestHintStatsClickhouseQueryCountsBehavioralFollowsByHintAction` | `TestRetrieveHintStats/TestBehavioralDeliveryModesAndDuplicateAnnotations` |

## Preserved semantics

- **Mixed sent populations use MAX**: Sent/conversation/user denominators are independent branch MAX values, not summed or union-distinct populations.
- **Moment-based sent and followed can lack delivered actions**: Default sent includes it; both followed paths count it. Action mode excludes it only from sent.
- **Knowledge follows accept empty and unlinked action IDs**: COUNT DISTINCT includes the empty string and IDs with no matching sent action.
- **Metadata exclusion depends on nullable joins**: Unmatched IDs are empty strings; both exclusion-only and combined filters return empty. Nullable joins yield intended filtered cohorts.
- **Action mode loses behavior-filtered populations**: Action table has no behavior column, so the existing false filter removes sent groups.
- **Agent attribution can separate sent from followed**: Action-mode recipient group has sent1/followed0; detector-only followed group is dropped by the sent-anchored LEFT JOIN.
- **Conversation totals retain grouping multiplicity**: Each agent group counts the conversation once and the response sums to2.
- **Group/policy no-frequency per-user averages remain zero**: Group-level per-user averages remain zero because grouped conversion derives its denominator from time buckets, which no-frequency reads omit. Top-level rates retain the original values. Both delivery modes are characterized by full-RPC assertions.

The extraction retains four reusable raw relations. Legacy aggregation, grouping, argument ordering, reader/converter behavior, policy processing and rollout controls remain owned by the existing RPC. No production deployment, production data census, benchmark or query-plan claim is made.

No AWS, Okta, Azure or SSH credentials were read, selected or used. This agent did not push or edit the PRs; root owns publication.
