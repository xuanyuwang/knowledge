# Auto-Heal Cron Rollout Plan

**Created**: 2026-06-07  
**Ticket**: CONVI-6869 - Piloting auto-heal cron job  
**Scope**: Safely enable the enhanced `scorecard-sync-monitor` cron on staging and production.

Related investigation: `stale-scorecard-consistency-investigation.md` covers the stale existing-row gap, the `argMax(..., update_time)` rationale, and Slack startup rollout findings.

## Goal

Roll out scorecard auto-heal in phases so detection, metrics, alerting, and backfill dispatch can be validated independently. The monitor should never jump directly from a suspended/manual cron to unrestricted production backfills.

The target design is:

1. Detect missing scorecards from PG vs ClickHouse for a rolling window.
2. Emit all/submitted/unsubmitted metrics.
3. Build missing scorecard resource names.
4. Split missing scorecards into process vs conversation inputs.
5. Dispatch targeted `reindexscorecards` backfills only after rollout controls are enabled.

## Safety Controls Required Before Enabling Scheduled Runs

Before unsuspending staging or production cron, add explicit rollout controls:

| Control | Required default | Purpose |
|---|---:|---|
| `SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL` | `false` | Allows observe-only scheduled runs without creating reindex jobs. |
| `SCORECARD_SYNC_MONITOR_MAX_BACKFILL_SCORECARDS_PER_RUN` | small non-zero only when auto-heal is enabled | Prevents one run from launching a large recovery wave. |
| Auto-heal customer/profile allowlist | empty | Lets detection run broadly while healing is limited to canaries. |
| Drill-down/manual guard | auto-heal disabled unless explicitly allowed | Prevents historical or filtered manual investigations from creating accidental backfills. |

Detection, metrics, and Slack summaries should continue to run when auto-heal is disabled.

## Rollout Sequence

### 1. Add rollout controls

Implementation checklist:

- Add `SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL=false` by default.
- Add a max-heal cap per cron run.
- Add an auto-heal allowlist independent from `SCORECARD_SYNC_MONITOR_CUSTOMER_IDS`.
- Log when missing scorecards are found but auto-heal is disabled.
- Emit backfill-triggered as `0` for dry-run/observe-only runs.
- Keep filtered drill-down runs observe-only unless an explicit override is set.

Exit criteria:

- A scheduled run can detect missing scorecards without calling `CreateJob`.
- Unit tests cover enabled vs disabled auto-heal behavior.
- A local/manual run clearly logs the dry-run decision and missing counts.

### 2. Deploy code with auto-heal disabled

Deploy the monitor/workflow/proto changes with the auto-heal gate set to `false`.

Validation:

- Cron remains suspended until the image/config is confirmed.
- Manual staging run succeeds with auto-heal disabled.
- Metrics appear for:
  - `total_all`
  - `total_submitted`
  - `total_unsubmitted`
  - `ch_count_all`
  - `ch_count_submitted`
  - `ch_count_unsubmitted`
  - `missing_count_all`
  - `missing_count_submitted`
  - `missing_count_unsubmitted`
  - `missing_rate_percent_all`
  - `missing_rate_percent_submitted`
  - `missing_rate_percent_unsubmitted`
  - `backfill_triggered`
- Slack summary emphasizes submitted gaps first.

### 3. Staging dry-run validation

Unsuspend staging cron while keeping `SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL=false`.

Recommended staging config:

```yaml
schedule: "0 * * * *"
suspend: false
SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL: "false"
```

Run for at least 24 hours.

Validate:

- No `CreateJob` / reindex job is created by the cron.
- PG and ClickHouse query load stays acceptable.
- Logs show `clickhouse_all`, `clickhouse_submitted`, and `clickhouse_unsubmitted`.
- Metrics and dashboards show stable series for all three views.
- Slack output is not noisy.
- Missing sets are stable and explainable.
- Manual trigger still works with custom time ranges and customer filters.

Exit criteria:

- 24 hours of successful scheduled staging runs.
- No unexpected backfill jobs.
- No query/load regression.
- Dashboard and Slack output are usable by an oncall engineer.

### 4. Staging constrained auto-heal

Enable auto-heal only for one low-risk staging customer/profile.

Recommended initial config:

```yaml
SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL: "true"
SCORECARD_SYNC_MONITOR_AUTO_HEAL_CUSTOMER_PROFILE_ALLOWLIST: "<customer>/<profile>"
SCORECARD_SYNC_MONITOR_MAX_BACKFILL_SCORECARDS_PER_RUN: "10"
```

Validation:

- Create or identify one missing submitted scorecard and one missing unsubmitted scorecard.
- Confirm the monitor detects both.
- Confirm process vs conversation classification is correct.
- Confirm `reindexscorecards` writes ClickHouse rows.
- Confirm the next monitor run shows missing count drop.
- Confirm repeated healing is idempotent.

Exit criteria:

- Both submitted and unsubmitted staging gaps are repaired.
- Workflow failures are visible in logs/Temporal.
- No repeated loop against permanently unhealable IDs.

### 5. Production observe-only

Deploy production with auto-heal disabled.

Recommended initial config:

```yaml
SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL: "false"
```

Rollout:

1. Unsuspend one production cluster in observe-only mode.
2. Run daily or every few hours first if there is concern about query load.
3. Move to hourly only after metrics and logs look healthy.
4. Repeat by cluster.

Validate for 24-48 hours:

- Submitted missing rate by customer/profile.
- Unsubmitted missing rate by customer/profile.
- Total missing by cluster.
- Would-have-triggered backfill volume.
- PG/CH query latency and error rate.
- Slack volume.

Exit criteria:

- Production detection is trustworthy.
- Backfill volume estimates are bounded.
- Oncall/dashboard views are ready before any production writes.

### 6. Production canary auto-heal

Enable auto-heal for one known customer/profile only.

Recommended initial config:

```yaml
SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL: "true"
SCORECARD_SYNC_MONITOR_AUTO_HEAL_CUSTOMER_PROFILE_ALLOWLIST: "<customer>/<profile>"
SCORECARD_SYNC_MONITOR_MAX_BACKFILL_SCORECARDS_PER_RUN: "10"
```

Monitor:

- `backfill_triggered`
- Temporal workflow success/failure
- ClickHouse write errors
- Missing submitted rate over the next 1-3 monitor runs
- Same IDs remaining missing after repeated attempts

Stop expansion if:

- The same IDs remain missing after repeated heals.
- Workflow failure rate is non-trivial.
- Query or write load is higher than expected.
- Slack/dashboard output is ambiguous.

### 7. Gradual production expansion

Expand only after the canary repairs real gaps and the next monitor run confirms the drop.

Suggested expansion:

1. One customer/profile.
2. All profiles for one low-volume customer.
3. One production cluster.
4. Remaining production clusters.

Suggested cap ramp:

```text
10 -> 50 -> 100 -> 500
```

Keep the customer/profile allowlist until the cap and persistent-failure behavior are proven.

### 8. Rollback

Primary rollback should be config-only:

```yaml
SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL: "false"
```

Secondary rollback:

- Suspend `cron-scorecard-sync-monitor`.
- Terminate active `reindexscorecards` workflows only if they show bad writes or runaway retries.
- Roll back code only if detection itself is broken.

## Current Verification: Can We Start From Step 3?

**Verdict: No, not safely from the checked-in local state as of 2026-06-07.**

Reasons:

1. The staging HelmRelease is still suspended and scheduled daily, not hourly:
   - `flux-deployments/apps/cron-task-runner/releases/01-staging/helmrelease-cron-scorecard-sync-monitor.yaml`
   - Current values: `schedule: 0 8 * * *`, `suspend: true`
2. The monitor currently triggers backfill whenever `missingBatch != nil`.
   - `go-servers/cron/task-runner/tasks/scorecard-sync-monitor/task.go`
   - `Run()` calls `triggerBackfill()` directly after missing scorecards are detected.
3. No `SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL`, dry-run, or max-backfill env flag exists in:
   - `go-servers/cron/task-runner/tasks/scorecard-sync-monitor/factory.go`
   - `go-servers/cron/task-runner/tasks/scorecard-sync-monitor/README.md`
4. Because of that, unsuspending staging would not be a dry-run. It would create reindex jobs for any detected missing scorecards.

Minimum work before Step 3:

1. Add and test the auto-heal gate with default `false`.
2. Add a max backfill cap and allowlist.
3. Deploy those controls to staging.
4. Configure staging cron with `suspend: false`, hourly schedule, and `SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL=false`.

After that, Step 3 can begin.
