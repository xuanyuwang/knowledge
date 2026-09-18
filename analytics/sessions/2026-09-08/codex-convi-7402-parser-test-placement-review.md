# CONVI-7402 parser test placement review

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402`
- Branch: `convi-7402-bswift-users-getting-qa-score-stats-errors`
- Current head: `a9cf40e6c6`.
- User requested verification before relocating parser cases around common_clickhouse_test.go:388-392 to an RPC test file.

## Verdict

Skip this finding. The cited cases belong to `TestParseClickhouseFilter_RejectsValueOrMissingMetadataFilter`, testing shared `parseClickhouseFilter` directly. Current callers include agent, conversation, assistance, hint, knowledge-base, scorecard and other stats RPCs. No single matching RPC owns this behavior. Existing parser coverage in common_clickhouse_test.go predates this PR (verified at main ancestor 98583180ee).

The broader insights-server guide describes action_<rpc_name> test naming, but this package uses retrieve_<rpc_name> files, as its package guide and filesystem show. RPC test placement guidance does not establish that shared parser unit tests should move to an arbitrary endpoint. The moment constructor, table inputs and feature-flag cleanup are local to this test; no shared fixture needs relocation to base_test.go.

No source/test changes. All 17 table cases pass in the focused parser run (analyticsimpl 1.559s), using offline/local Go settings and unsetting TEST_DATABASE_URL. Source worktree remains clean at a9cf40e6c6. No network credentials used.
