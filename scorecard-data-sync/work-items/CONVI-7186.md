# CONVI-7186: Bound missing-unsubmitted score checks

**Status:** complete
**Primary domain:** `scorecard-data-sync`
**Official ticket:** CONVI-7186
**Last updated:** 2026-07-15

## Objective and Impact

- **Objective:** Restore scorecard-sync-monitor completion on large profiles without counting empty unsubmitted scorecard shells as missing analytical scorecards.
- **Impact:** The previous correlated score-existence queries caused large production clusters to hit the monitor's 30-minute deadline, eliminating both detection and auto-heal coverage.
- **Role:** Diagnosed, designed, implemented, validated, and followed deployment evidence.

## Source Context

- **Repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7186-monitor-query`
- **Branch:** `convi-7186-monitor-filter-missing-empty`
- **PR:** https://github.com/cresta/go-servers/pull/29786

## Current Understanding

The monitor now compares the broad time-range PG inventory to CH first. Only missing unsubmitted IDs are checked for `director.scores`, in batches of 5000. Missing unsubmitted shells without score rows are removed from the denominator, missing count, and reindex candidates after the CH lookup.

This keeps score-existence work proportional to missing unsubmitted entities instead of every unsubmitted shell and removes the second global excluded-count query.

## Findings and Decisions

- Submitted inventory remained index-efficient; the unsubmitted shell population caused the expensive path.
- A naive score-driven join was not sufficient because the planner could reorder it back to the scorecard scan.
- A broad partial unsubmitted index could be very large on high-volume profiles.
- Post-CH filtering was selected as the focused workaround because empty unsubmitted scorecards are expected to be absent from CH.

## Validation and Rollout

- Focused package test passed: `go test ./cron/task-runner/tasks/scorecard-sync-monitor`.
- PR checks passed and the change merged.
- Main-branch Cron Task Runner image build/push was verified after merge.
- Production monitor runtime/result validation remains the operational follow-up if not already captured elsewhere.

## Next Actions

1. Confirm large-cluster monitor completion and phase timing in production.
2. Track any existing-but-stale field mismatch separately because the monitor remains ID-existence based.

## Timeline

- 2026-06-30 — Live query-plan and GroundCover evidence isolated PG inventory/count as the deadline bottleneck.
- 2026-07-08 — Implemented post-CH missing-unsubmitted score checks and validated focused tests.
- 2026-07-12 — Verified merged main-branch Cron Task Runner image publication.
- 2026-07-15 — Migrated current state into the canonical Scorecard Data Sync domain.
