# go-servers QA ClickHouse time-range investigation (CONVI-7162)

Date: 2026-07-28
Repo: `/Users/xuanyu.wang/repos/go-servers` (main @ `85d639ecaf`, behind origin by ~462)

## Branch / worktree state

- **No worktree** at `/Users/xuanyu.wang/repos/go-servers-convi-7162` (documented Jul 10 path is gone).
- Local branch `convi-7162-holiday-inn-club-vacations-manager-leaderboard-scorecards` exists but has **zero unique commits** (reflog: created from `origin/main` at `1904aee21e`; no remote `*7162*` heads).
- Jul 10 request-shape gate implementation is **not present** on main or on that branch tip. No PR for CONVI-7162 found. Tests named `WithSubmittedScorecardReviewerUsesSubmitTime` / helpers like `timeColumnOverride` are absent.

## Current time-range behavior (main)

- Default QA path: `getConversationTimeRangeColumn` → `conversation_start_time`, remapped via `tableColumnNameMapping` to `scorecard_time` for `scoreTable` / `scorecardTable`.
- `ConversationTimeRangeField=ENDED_AT` → `conversation_end_time` (needs conversation join; score tables map end column to `ColumnNotExist`).
- No submit-time time-range option on QA APIs. `scorecard_submit_time` is only used for status filters (`MANUALLY_SUBMITTED` etc.) and is projected for grouping potential, but filter/group still use `scorecard_time` by default.
- Old `RetrieveScorecardStats` still rewrites `scorecard_time` → `scorecard_submit_time` (and agent → submitter).

## Files for ScorecardTimeBasis opt-in

Proto (cresta-proto) + go-servers analyticsimpl helpers/call sites + SQL fixtures/tests; optional director to set field. See investigation handoff for full list.
