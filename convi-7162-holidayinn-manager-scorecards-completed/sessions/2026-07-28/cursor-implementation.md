# CONVI-7162 implementation session (2026-07-28)

## Worktrees

| Repo | Path | Branch |
|------|------|--------|
| cresta-proto | `/Users/xuanyu.wang/repos/cresta-proto-convi-7162` | `convi-7162-scorecard-time-basis` |
| go-servers | `/Users/xuanyu.wang/repos/go-servers-convi-7162` | `convi-7162-holiday-inn-club-vacations-manager-leaderboard-scorecards` |
| director | `/Users/xuanyu.wang/repos/director-convi-7162` | `convi-7162-manager-scorecards-submit-time` |

## Proto

- Added `ScorecardTimeBasis` enum to `qa_stats.proto`.
- Added `scorecard_time_basis` to `RetrieveQAScoreStatsRequest` (13) and `RetrieveQAConversationsRequest` (11).
- PR: https://github.com/cresta/cresta-proto/pull/9402
- Local bazel-generated Go used via `go.mod` replace for go-servers (not committed to proto PR; CI regenerates on merge).

## go-servers

- `WithTimeRangeColumn` + `qaScorecardTimeRangeOptions` in `common_clickhouse.go`.
- QA score stats + conversations ClickHouse paths use `scorecard_submit_time` when `SUBMIT_TIME`.
- Conversation_d date range skipped under `SUBMIT_TIME`; moment filters keep conversation time.
- Reject `SUBMIT_TIME` + `CONVERSATION_ENDED_AT`.
- Tests + SQL goldens added; focused suites pass.
- `go.mod` has local replace to cresta-proto worktree until published version is available.

## director

- Local `ScorecardTimeBasis` enum in director-api types (until `@cresta/web-client` bump).
- Manager Leaderboard stats + drawer set `SCORECARD_TIME_BASIS_SUBMIT_TIME` only.
- Agent/Team/Performance paths unchanged (field unspecified).

## Remaining

1. Land/fix proto PR CI (Lint API failed once; buf gen still pending).
2. After proto publish: bump go-servers proto dep (drop replace), open go-servers PR.
3. Bump `@cresta/web-client` in director; optionally replace local enum with web-client export; open director PR.
4. Verify Holiday Inn Manager submit-day counts; Agent unchanged.
