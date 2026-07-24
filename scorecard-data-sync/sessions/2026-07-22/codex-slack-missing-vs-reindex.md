# Slack Missing Count vs Reindex Workflow Investigation

## Context

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree: `main` at `/Users/xuanyu.wang/repos/go-servers`
- Slack event: https://crestalabs.slack.com/archives/C0ABYUBEYSG/p1784715802563159
- Temporal workflow: `reindexscorecards-snapfinance-us-west-2-f5d61cb3-4151-4736-9e31-5964c6ce1960`
- Monitor range: `2026-07-21T10:00:06.897906002Z` to `2026-07-22T10:00:06.897906002Z`

## Symptom

Slack reported `snapfinance/us-west-2 — 0/144092 (0.00% missing)` while also linking one reindex workflow. The workflow input contained three conversation scorecard resource names.

## Findings

- The Slack fraction renders only `SyncStats.MissingCount`, `TotalCount`, and `MissingRate`.
- The monitor separately classifies an existing ClickHouse row as stale when either:
  - PG `submitted_at` differs from CH `scorecard_submit_time`; or
  - PG `updated_at` differs from CH `scorecard_last_update_time`.
- Both missing and stale rows are appended to the same in-memory `reindexCandidateBatch`.
- The reindex workflow payload is built directly from that batch, while the Slack line appends only the number of created jobs and their links. It does not render stale count or total candidate count.
- Therefore, the observed run had zero missing CH rows and three existing-but-stale conversation scorecards. The workflow input is authoritative for the three dispatched scorecards; the Slack `0/144092` is correct only for row absence.
- This is a reporting/labeling ambiguity, not evidence that the workflow received a different candidate set.

The exact mismatch reason per scorecard cannot be recovered from the workflow input because the payload carries resource names, not classification reasons. The cron log line `reindex_candidates` would show aggregate `stale_submit_time` and `stale_update_time` counts. Production Kubernetes log access was unavailable because the local AWS SSO session had expired. Direct DB access is not needed for the code-level conclusion, but historical logs or a pre-repair DB snapshot would be needed to identify each scorecard's exact timestamp mismatch.

## Additional Evidence

- The same Slack run included other profiles with zero missing rows and created workflows, consistent with stale-only dispatches.
- Commit `fca948c5fb` intentionally added stale timestamp detection and included stale rows in reindex batches.
- Package tests passed:

```text
go test ./cron/task-runner/tasks/scorecard-sync-monitor
ok github.com/cresta/go-servers/cron/task-runner/tasks/scorecard-sync-monitor
```

## Recommended Reporting Change

Render missing, stale, and total reindex candidate counts together when a job is created, for example:

```text
0/144092 missing; 3 stale; created 1 reindex job for 3 candidates
```
