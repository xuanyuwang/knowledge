# PR #32440 semantic correctness review — 2026-09-17

No actionable correctness findings against `e7704f66378c40466620eacd4535cfede19e942b`. The insights from [PR #31335's review](convi-7402-pr-31335-adversarial-review.md) transfer well as review checks; its four corrected defects did not reproduce in this PR's supported conversation-stats path.

Reviewed [PR #32440](https://github.com/cresta/go-servers/pull/32440) against its exact parent/base `6e2648ebeddfd56c8484ea2766d6e5c01b4f0dce`. GitHub head/base were verified at the beginning and end. No rebase, implementation fixes, commits, pushes, review comments, or external mutations were performed.

## How the earlier findings apply

| Earlier lesson | Current implementation and evidence | Result |
|---|---|---|
| Preserve value-or-missing as an OR before lowering filters | `parseConversationStatsValueOrMissingFilters` extracts supported same-ID singleton groups; separate groups are AND-ed by the query builder. Remaining filters go through the unchanged shared parser. Multiple overlap groups and overlap plus ordinary inclusion/exclusion execute correctly. | Pass |
| Reject unsupported overlap shapes before discarding structure | Nine additional parser cases cover larger include/exclude lists, omitted types on either side, invalid names, missing values and unset value oneofs. Unsupported overlaps still return `InvalidArgument`. | Pass; support remains deliberately narrower than ES |
| Align annotation filters with the endpoint's time axis | The reader passes its effective start/end column into preprocessing and the ordinary parser. Both selected-value and existence scans use that column. Fixtures include conversations starting before the window and ending inside it, plus conversations ending outside it. | Pass; scorecard SUBMIT_TIME is not an axis of this RPC |
| Honor numeric endpoints and reject invalid metadata | Shared conversion already includes the #31335 boundary/oneof fixes. Runtime singleton `[5,5]` and exclusive/inclusive `(5,10]` cases pass; malformed values are rejected. | Pass |
| Execute SQL rather than trusting snapshots | 96 complete generated queries, tested with independent fixture membership/aggregate expectations, produced 176 passing ClickHouse executions. | Pass within local-test limits |
| Audit shared-helper callers | `parseClickhouseFilter` still has 16 production callers; only conversation stats preprocesses away supported overlaps. QA passes the same MV-selection expression into the renamed helper that it previously calculated internally. | No unintended enabling of other RPCs found |
| Bound source scans | All three raw annotation scans per latest-value overlap retain conversation-time predicates. The earlier unbounded submit-time scan pattern is absent. | Logical bounds confirmed; physical scan cost not measured |

## Contract and execution coverage

For metadata template A, selected values are OR-ed; missing means no annotation for A, not a stored empty string, false, or zero. Groups are AND-ed. Default mode tests the latest `create_time`; stale-enabled mode accepts any matching annotation. The new selected-value and existence CTEs each deduplicate conversation IDs before joining the aggregate, preventing annotation fanout.

The runtime matrix crosses 12 scenarios with start/end filtering, latest/stale selection, and ungrouped versus agent/day grouping:

- String X, empty string, multiple selected strings X/Y.
- Boolean true and false; numeric zero.
- Inclusive numeric singleton and exclusive-lower/inclusive-upper interval.
- Two independent value-or-missing groups.
- Value-or-missing plus ordinary include-only and missing-only groups.
- Ordinary include-only and missing-only regression cases.

All fixtures also exclude voicemail. Data contains missing annotations, disallowed values, both directions of value changes, duplicate annotations/messages, independent group matches/mismatches, and time-boundary membership. Assertions check conversation counts, distinct users, handle-time sums, and grouping keys against an independent Python oracle. Generator checks verify positional placeholder counts and that preprocessing leaves the input proto unchanged.

All 96 queries run with `join_use_nulls=0`; 80 also run with `join_use_nulls=1`. The 16 ordinary missing-only / mixed-group variants are not claimed to support NULL joins: their unchanged independent exclusion uses `= ''`. This pre-existing limitation was also identified in #31335. The new overlap's `ifNull` expressions work in both modes.

One beneficial adjacent change: the extracted latest-value helper now strips boolean metadata predicates when determining the maximum timestamp, as well as string/number predicates. This makes the latest-value selection consistent for boolean metadata. It affects ordinary conversation-stats includes too; no other RPC uses this helper.

## Tests and CI

The following focused selection passed (7.673 seconds):

```sh
env -u TEST_DATABASE_URL GOPROXY=off GONOPROXY=none GOTOOLCHAIN=local \
  go test ./insights-server/internal/analyticsimpl \
  -run 'TestRetrieveConversationStats|TestParseClickhouseFilter_RejectsValueOrMissingMetadataFilter|TestParseMomentConditionsForQAAttribute|TestRetrieveQAScoreStatsClickhouseQuery_FilterByMetadataMomentGroups_OverlapValuePlusNoValue' -count=1
```

Temporary `TestReview32440ExportSQL` and `TestReview32440RejectShapes` passed (1.428 seconds), as did the existing Elasticsearch overlap contract test (0.935 seconds). The initial temporary generator compile used the wrong frequency enum spelling; correcting the harness to `Frequency_DAILY` resolved it without a product change. ClickHouse **26.8.2.7** executed **176/176** successful checks. Temporary source tests are retained as evidence and removed from the PR worktree.

Required GitHub CI is **not green**. Run `35262160139` has four failed ES-backed targets across three Bazel shards: conversation messages, partial conversations, closed conversations, and conversation message stats. Their logs contain response-body EOF errors. `TestRetrieveConversationMessages/BasicRequest` reproduces the same EOF locally on both head and exact base. Base verification uses a Go overlay restoring all three PR-changed files to their parent versions and replacing the temporary review file with an empty package. Thus this reproduced failure predates the PR; the other failing targets have not each been independently base-tested, and this is not a claim that every CI failure is explained.

## Remaining limits and recommendations

- The committed new test primarily checks SQL substrings and only one overlap plus voicemail. It does not permanently protect numeric boundaries, multiple groups, stale transitions, input preservation, or real database results. Preserve a compact regression matrix from this review in a follow-up; temporary review coverage is not CI coverage.
- Local Memory tables validate SQL name resolution and logic, not distributed joins, replication, ReplacingMergeTree merges, production query settings, or metadata ingestion freshness/deletion semantics. The local version is newer than the repository's previously documented test-container version. No live ES differential or production query was run.
- Runtime SQL was generated by the production parser/builders, mirroring the reader's wiring. Existing endpoint suite tests passed, but the temporary overlap matrix does not itself issue gRPC requests or exercise ACL/cache/config boundaries.
- Equal latest timestamps with conflicting values, raw-vs-MV historical consistency, and production scan cost remain unmeasured existing concerns. Raw storage and `create_time` preserve conversation stats' current contract; this review does not claim parity with every QA storage lifecycle.
- Staging validation after deployment remains necessary for the current and comparison-window Bswift AHT requests. Address the existing CI blocker before merge.

## Evidence and credentials

[Reproduction bundle](../sessions/2026-09-17/pr-32440-review-evidence/README.md) contains generated queries, fixtures, oracle, results, temporary Go tests, and bounded baseline failure excerpts. [Session record](../sessions/2026-09-17/codex-pr-32440-semantic-review.md) ties the review to the canonical work item.

Credentials used: existing GitHub CLI HTTPS authentication for read-only PR/check/log reads; repository embedded-PostgreSQL test account `cresta` for local suites. Local ClickHouse runs in-process without credentials. Go dependency network access was disabled for test runs. No AWS, Okta, Azure, SSH, or production database credentials were read or used.
