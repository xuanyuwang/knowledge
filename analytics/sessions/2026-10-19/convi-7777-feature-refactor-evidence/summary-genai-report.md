# Summary and GenAI statistics: consolidated real ClickHouse suites

The existing RPC suites now contain all 49 frozen reader characterization scenarios, 32 seeded full-RPC scenarios and 6 validation cases. The original implementation and refactor passed the identical 95 test nodes (87 leaf scenarios). Reader fixture bodies and assertions match the frozen versions after suite receiver renaming. There are no separately runnable regression or integration test files.

Tests commit: `5c6b94f02361a78a5c27fa3242f7ebf76328d6b8`. Refactor commit: `00fd23587abc3f229e562d77d9b798006de77693`. Existing PRs remain [tests #34043](https://github.com/cresta/go-servers/pull/34043) and [refactor #34044](https://github.com/cresta/go-servers/pull/34044). Root owns publishing and metadata updates. Both assigned worktrees are clean.

## Test organization and evidence

`retrieve_summarization_stats_test.go` and `retrieve_knowledge_assist_stats_test.go` own their DBTestSuite/ClickhouseContainerTestSuite lifecycle, unique profile fixtures, and exact complete-protobuf assertions. ClickHouse results are seeded and queried. External user/config/ACL collaborators use exact request mocks built in suite hooks. The two existing Bazel targets are Gazelle-generated standalone targets with large/long settings; the two obsolete regression targets and eight unused SQL snapshots were removed.

The original merged suites passed in 471.971 seconds; the identical frozen suites passed after extraction in 473.421 seconds. These durations are execution evidence, not benchmarks. Logs: `/tmp/convi7777-summary-genai-merged-before.jsonl` and `/tmp/convi7777-summary-genai-merged-after.jsonl`. Manifest: `/tmp/convi7777-summary-genai-merged-frozen.json`. The JSON report records exact commands, SHA256 hashes, all passed nodes and the removed-case coverage map.

## Behavior preserved

Summary generation requires a valid summary payload. Used copies require event16 and payload datatype1/2, joined to valid generation by conversation ID, start time and agent ID. Distinct conversation grain, unmatched generation, copy mismatches and conversation-derived denominators remain pinned. Summary still counts source7/dev-user evidence; GenAI excludes it.

GenAI counts retrieval-question action IDs, distinct viewed article sessions and nonempty used-reply moment sessions. Messages anchor groups and user denominators. Duplicate evidence, cross-conversation sessions, sent/viewed-only evidence, message-only groups, annotation-only groups, source/dev gates and empty sessions retain their baseline behavior.

Both readers retain the metadata exclusion issue: unmatched excluded metadata fills a default key under the existing LEFT JOIN, so the IS NULL guard drops otherwise eligible rows. Reader cases and a combined include/exclude full-RPC fixture preserve the empty response. GenAI averages retain different bucket/result/response denominators. The original time-only two-day characterization keeps averages0.5/1 per bucket,1 per result and2 per response.

The full-RPC cases cover enriched agent metadata, selected users, inline/external filters, selected TEAM resolution, group and agent+group conversion, Honolulu daily/weekly boundaries, no-frequency agent grouping, time-only grouping, restricted/denied ACLs, latest metadata inclusion and combined user/usecase/duration filters. Removed SQL cases are mapped individually in the JSON. The former Summary canned conversion fixture claimed four distinct generated conversations within one counted conversation; feasible seeded rows now test exact conversion output instead of treating that impossible row shape as a business contract.

## Extraction boundary and verification

Summary's helper owns valid generation/copy CTEs and their existing three-key LEFT JOIN; the RPC retains denominator construction, grouping, distinct counts and response conversion. GenAI's helper owns raw eligible message/action/used-moment selectors; the RPC retains all aggregation and response conversion. All production Go bytes except `common_knowledge_assist_usage_clickhouse.go` exactly match the previously reviewed refactor `6c9cb487c8f55b0156a63e58645d4588b85488e3`. The one-file difference uses explicit canonical column aliases to restore original combined metadata behavior; root independently reviewed this minimal parity fix. The tests branch changes no production source.

The expanded frozen suites exposed a GenAI extraction regression: combined metadata inclusion/exclusion passed original production but failed the child with missing outer grouping columns. The helper used a qualified wildcard under metadata joins; explicit table.field AS field projections restore canonical names without changing filters, joins, args, grouping or raw row multiplicity. The exact previously failing RPC passed after the fix, followed by the full unchanged suite. The failed log is `/tmp/convi7777-summary-genai-merged-after-failed.jsonl`; focused fix evidence is `/tmp/convi7777-summary-genai-merged-fix-focused.jsonl`. This refactor regression is recorded separately from the preserved preexisting metric quirks.

Gofmt, Gazelle target generation and staged whitespace checks passed. Dedicated Bazel targets were not independently run by this worker; focused Go runs compiled the entire package and executed both suites. Source and scenario-body equivalence logs are in the JSON report. Early diagnostic runs corrected a nonexistent fixture enum and expected error text on the original implementation; one baseline startup was intentionally interrupted for coordinated Docker memory usage. No expectations were changed after the final passing baseline was frozen.

## Limits

Fixtures use local ClickHouse23.8 and PostgreSQL; they do not prove production deployment or performance. RPC collaborator mocks verify local user-resolution and ACL integration, not live authorization services. Summary zero-group requests and the same GenAI session across multiple time buckets have no dedicated fixture. Source/dev mixed-table combinations are not exhaustive. No AWS, Okta, Azure or SSH credentials were read or selected. Local ClickHouse uses repository fixture `defaultTestUser` with an empty password; DBTestSuite manages PostgreSQL.
