# CONVI-7402: Bswift QA score stats errors on Team + (no value)

**Ticket:** [CONVI-7402](https://linear.app/cresta/issue/CONVI-7402/bswift-users-getting-qa-score-stats-errors)
**Zendesk:** [#22883](https://crestasupport.zendesk.com/agent/tickets/22883)
**Slack:** [thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785449772055899)
**Customer:** Bswift (`bswift` / `us-east-1`)
**Template:** Automated Heartbeat Quality Scorecard (`0199abb9-fd82-7696-9302-54a27b5e33b6`)
**Status:** PR #31335 merged; QA paths fixed; regression tracked in [CONVI-7706](https://linear.app/cresta/issue/CONVI-7706/regressionbswift-retrieveconversationstats-rejects-team-value-no-value)
**Last validated:** 2026-09-17

## Verdict

HTTP 400 on `RetrieveQAScoreStats` (and parity gap on QA conversations) is fixed by porting Elasticsearch overlap OR semantics into ClickHouse QA.

When a metadata `MomentGroup` includes the same moment template id in both `moments` (concrete Team values) and `excluded_moments` (`(no value)`), the backend now treats it as:

- `(Team ∈ selected values)` OR `(Team moment template is absent (no annotation for that template id))`

Independent moment groups and unrelated exclusions continue to be AND-ed exactly as before.

Secondary: bswift scorecard PG→CH sync gap can still cause empty/stale PI data after the 400 is fixed — track separately from this validation error.

## Production follow-up (2026-09-17)

The new bswift HAR confirms the QA fix works but exposes the remaining shared-parser gap. All four errors are now `RetrieveConversationStats` 400s with `value-or-missing metadata filters are not supported for this ClickHouse endpoint`; the request contains Team `POD 1` plus `(no value)` and a valid independent voicemail exclusion. Director sends this filter to the Average Handle Time metric for both current and comparison windows. The QA score and conversation-volume surfaces render, while AHT shows `--`.

This error comes from the defensive `validateNoMetadataMomentOverlap` added in PR #31335 to prevent shared non-QA callers from silently producing an impossible include-AND-exclude predicate. It occurs before ClickHouse execution and is unrelated to scorecard sync. The narrow follow-up is to implement OR semantics in the conversation-stats moment query builder while retaining explicit rejection for other unimplemented shared callers. Full evidence: [2026-09-17 HAR follow-up](../sessions/2026-09-17/codex-convi-7402-conversation-stats-har.md).

Created high-priority regression [CONVI-7706](https://linear.app/cresta/issue/CONVI-7706/regressionbswift-retrieveconversationstats-rejects-team-value-no-value), related to CONVI-7402, with the HAR evidence, request-to-source root cause, proposed endpoint-specific fix, and validation matrix.

## PR review follow-up (2026-09-08)

PR [#31335](https://github.com/cresta/go-servers/pull/31335) has three unresolved human review threads and is awaiting code-owner approval. The must-fix finding is a submit-time asymmetry: scorecards can be filtered by submit time while the new metadata overlap CTEs remain filtered by conversation time, which can over-include rows by misclassifying an out-of-window annotation as missing. The low-risk alias concern in the QA-conversations builder should also be addressed. Broader Elasticsearch parity and the still-unsatisfiable overlap behavior in shared `parseClickhouseFilter` should be explicit follow-ups unless product scope requires them in this PR. Full triage and sequence: `sessions/2026-09-08/codex-convi-7402-pr-review-actions.md`.

All requested fixes and cleanup are isolated in seven commits from `33340674eb` through `1b2a95bd9a`. The submit-time correctness bug and alias concern are fixed; unsupported broader QA shapes are explicit; shared non-QA overlap no longer silently yields zero rows; parser/CTE maintainability notes are addressed. Combined focused Go validation passed. The commits are pushed to PR [#31335](https://github.com/cresta/go-servers/pull/31335); CI and code-owner re-review remain.

## Adversarial review fixes (2026-09-08)

All four findings in the [runtime-backed review](../deliverables/convi-7402-pr-31335-adversarial-review.md) are fixed in separate local commits: `5970423a86` (QA conversations submit time), `cd3990ac3c` (all shared overlapping IDs), `a3efa37d07` (numeric endpoints), and `1036223149` (invalid metadata values). `fe0109acd5` separately fixes the whitespace-sensitive outcome assertion.

Each correctness regression reproduced the original failure before its fix. The combined parser/query/QA-suite/ES-parser run passes. Real local ClickHouse executions confirm correct submit-time membership, numeric boundaries, latest values and duplicate behavior. The cached repository Gazelle check and diff checks pass. Full evidence and commands are in the [implementation session](../sessions/2026-09-08/codex-convi-7402-review-fixes.md).

The source worktree is clean at `fe0109acd5`, atop the prior clean rebase onto main `98583180ee`. The subsequent review verified that remote PR head now also equals `fe0109acd5`. No external write was made by the review. CI/code-owner review and production scan cost/MV freshness validation remain.

## Second review (2026-09-08)

[Focused re-review](../sessions/2026-09-08/codex-convi-7402-second-review.md) found one remaining P2: the shared overlap validator skips untyped moment entries. With the same template ID on both sides but one omitted type, it silently returns value-only or missing-only filters. Equivalent ES requests retain the OR. Two reproductions fail; the 49 fully typed ID-set cases, two-group/null-setting runtime matrix, and all four numeric-boundary comparisons pass. The follow-up fix is now pushed as `8810713e2a`: group-level metadata classification catches all overlapping IDs even when a type is omitted. Four new regressions reproduced the failure before the fix; combined parser/query/QA/ES tests, Gazelle, formatting, and commit checks pass. Remote head matches local HEAD and the source worktree is clean. [Fix and push evidence](../sessions/2026-09-08/codex-convi-7402-partial-type-fix.md). CI/code-owner review and production cost/freshness validation remain.

## Submit-time scan restriction (2026-09-08)

Verified the source-scan finding against `8810713e2a` and fixed it in `a9cf40e6c6`. SUBMIT_TIME moment source CTEs now restrict rows to distinct candidate conversation IDs from `scorecard_score_per_conversation` using GLOBAL IN. Other time targets retain existing behavior. All six source-CTE regressions failed before the fix and pass afterward; combined Go suites and real local ClickHouse checks pass, including older conversations, out-of-window submissions and empty candidate sets. The commit is pushed to PR #31335; source worktree is clean. [Evidence and validation](../sessions/2026-09-08/codex-convi-7402-submit-time-scan.md). Production physical scan cost/distributed performance and CI/code-owner review remain.

## Implemented fix

- `parseMomentConditionsForQAAttribute` (in `insights-server/internal/analyticsimpl/common_clickhouse.go`) no longer blanket-rejects include+exclude overlap for the same metadata moment template id. Instead it returns an `overlapMomentFilters` pair (valueFilter + missingFilter) representing the overlap as OR semantics. This behavior remains gated by `CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS`.
- `qaScoreStatsClickhouseQueryWithMetadataView` and `qaConversationsClickhouseQuery` (in the corresponding ClickHouse query builder files) generate the overlap as `LEFT JOIN` + `WHERE (valueExists OR missingBranchIsTrue)`, while leaving excluded-only moment groups as their prior exclusion behavior.

Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402` (branch `convi-7402-bswift-users-getting-qa-score-stats-errors`)

## Validation

- Added parser unit test: `TestParseMomentConditionsForQAAttribute_MetadataMomentOverlap`.
- Added query-level SQL golden regressions + unit SQL coverage:
  - `testdata/clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql`
  - `testdata/clickhouse_RetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql`
- Ran `gofmt` on touched files and `bazel run //:gazelle`.
- Note: embedded-postgres-based full suite could not be executed here due to an `initdb` failure (`could not create shared memory segment: No space left on device`); this does not affect compilation or the SQL-builder/unit coverage.

## Key pointers

- Frontend: `director/.../useMomentGroupFilterFromFilterState.ts`
- Backend parsing: `go-servers/.../common_clickhouse.go` (`parseMomentConditionsForQAAttribute`)
- Backend ClickHouse query builders:
  - `retrieve_qa_score_stats_clickhouse.go`
  - `retrieve_qa_conversations_clickhouse.go`
- ES reference: `elasticsearch/request.go` `convertConvoMomentGroupToConvoFilters`
- Rollout of PI combined behavior: director [INSI-2621](https://github.com/cresta/director/pull/17329)

## Parser test placement review (2026-09-08)

Skipped the requested relocation of common_clickhouse_test.go:388-392 after verifying these are shared-parser cases spanning multiple RPCs, with test-local fixtures. All 17 focused cases pass; no source changes. [Rationale](../sessions/2026-09-08/codex-convi-7402-parser-test-placement-review.md).
