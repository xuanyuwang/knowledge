# Codex Session: scorecard-sync-monitor missing-empty filter implementation

**Date:** 2026-07-08
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7186-monitor-query`
**Branch:** `convi-7186-monitor-filter-missing-empty`
**Base:** `origin/main` at `927e992f49ef18b28a8f678247daf04398b844d0`
**Ticket:** `CONVI-7186`

## Goal

Implement the query workaround discussed after the 2026-06-30 `scorecard-sync-monitor` timeout investigation:

1. Fetch time-range PG scorecard inventory without filtering unsubmitted rows through `EXISTS (director.scores)`.
2. Query ClickHouse metadata for that inventory.
3. Only for missing unsubmitted scorecard IDs, query `director.scores` to determine which missing unsubmitted scorecards have score rows.
4. Exclude missing unsubmitted scorecards without score rows from totals and reindex candidates.

This keeps the expensive score-existence check proportional to missing unsubmitted rows rather than all unsubmitted scorecard shells in the time range.

## Code Changes

Changed:

- `/Users/xuanyu.wang/repos/go-servers-convi-7186-monitor-query/cron/task-runner/tasks/scorecard-sync-monitor/task.go`
- `/Users/xuanyu.wang/repos/go-servers-convi-7186-monitor-query/cron/task-runner/tasks/scorecard-sync-monitor/task_test.go`
- `/Users/xuanyu.wang/repos/go-servers-convi-7186-monitor-query/cron/task-runner/tasks/scorecard-sync-monitor/README.md`

Main behavior:

- Removed the up-front correlated `EXISTS` from the unsubmitted branch of the PG inventory query.
- Removed `countExcludedUnsubmittedWithoutScores`, which scanned the unsubmitted shell population a second time.
- Added `findScorecardIDsWithScores`, which runs:

```sql
SELECT DISTINCT scorecard_id
FROM director.scores
WHERE customer = ?
  AND profile = ?
  AND scorecard_id IN ?
```

only for missing unsubmitted IDs.

The helper batches the `scorecard_id IN ?` checks in groups of `5000` IDs (`pgScorecardIDsWithScoresBatchSize`) so an unexpectedly large missing set does not produce one enormous SQL statement.

- Added stats/log fields:
  - `FilteredMissingUnsubmittedWithoutScores`
  - `UnsubmittedWithoutScoresExistenceCheckCount`
- Updated the inventory log line to:

```text
filtered_missing_unsubmitted_without_scores=N checked_missing_unsubmitted=M
```

## Accounting Semantics

The monitor now excludes no-score unsubmitted scorecards only when they are missing in ClickHouse.

This is an intentional performance tradeoff based on current projection behavior: no-score conversation scorecards are expected not to have ClickHouse rows, so they surface as missing and are filtered after the ClickHouse lookup. If a no-score unsubmitted scorecard somehow has a ClickHouse metadata row, it remains in the denominator.

Adjusted totals:

- submitted total remains all submitted PG inventory in the time range;
- unsubmitted total starts as all unsubmitted PG inventory in the time range, then subtracts missing unsubmitted scorecards with no score rows;
- all total is `submittedCount + adjustedUnsubmittedCount`, not raw inventory length.

## Validation

Ran:

```bash
go test ./cron/task-runner/tasks/scorecard-sync-monitor
```

Result:

```text
ok  	github.com/cresta/go-servers/cron/task-runner/tasks/scorecard-sync-monitor	1.248s
```

## Notes

The focused integration test already had:

- missing unsubmitted scorecard with a `director.scores` row;
- stale unsubmitted scorecard with a `director.scores` row and ClickHouse row;
- missing unsubmitted scorecard without a `director.scores` row.

The test now verifies that:

- adjusted totals remain the same as the old semantic totals;
- `FilteredMissingUnsubmittedWithoutScores = 1`;
- `UnsubmittedWithoutScoresExistenceCheckCount = 2`, proving the PG score-existence check is limited to missing unsubmitted scorecards rather than all unsubmitted inventory.
