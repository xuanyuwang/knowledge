# Scorecard Auto-Heal Implementation and Validation Summary

**Updated**: 2026-06-22  
**Scope**: Summarize the implemented `scorecard-sync-monitor` auto-heal changes and how they were validated.

## Summary

We expanded `scorecard-sync-monitor` from a presence-only detector into a safer auto-heal monitor that can:

- Detect missing ClickHouse `scorecard_d` rows.
- Detect stale existing ClickHouse rows by comparing key scorecard metadata.
- Treat both missing and stale scorecards as reindex candidates.
- Dispatch targeted `JOB_TYPE_REINDEX_SCORECARDS` workflows only when rollout gates allow it.
- Split large accepted candidate sets into smaller reindex workflows.
- Skip automatic backfill when candidate volume is above a configurable manual-backfill threshold.
- Report job creation, Temporal workflow URLs, skip reasons, and clearer Slack summary wording.

The implementation keeps auto-heal disabled by default and relies on explicit allowlisting plus safety thresholds before creating workflows.

## Main Code Changes

### Missing and Stale Detection

The monitor now reads PG scorecard inventory with:

- `resource_id`
- `conversation_id`
- `created_at`
- `submitted_at`
- `updated_at`

For ClickHouse, it reads latest metadata from `scorecard_d` using:

```sql
argMax(scorecard_submit_time, update_time) AS scorecard_submit_time,
argMax(scorecard_last_update_time, update_time) AS scorecard_last_update_time
```

`update_time` is the ClickHouse row version column for `ReplacingMergeTree`; using `argMax(..., update_time)` gives the latest physical row without requiring an expensive `FINAL` query.

The monitor compares:

| PG field | ClickHouse field |
|---|---|
| `director.scorecards.submitted_at` | `scorecard_d.scorecard_submit_time` |
| `director.scorecards.updated_at` | `scorecard_d.scorecard_last_update_time` |

Timestamp comparison normalizes both sides to UTC at microsecond precision. For unsubmitted scorecards, expected submit time is Unix epoch, matching the ClickHouse row builder default.

Classification:

- `missing`: PG has a scorecard, but ClickHouse has no latest row.
- `stale`: ClickHouse has a row, but submit time or update time differs.
- `synced`: ClickHouse has a row and both timestamps match.

V1 intentionally does not compare user fields or `score_d` content.

### Reindex Candidate Handling

Missing and stale scorecards both become reindex candidates. Candidate source tracks whether the scorecard is missing or stale, and stale reason counts distinguish:

- submit-time mismatch
- update-time mismatch

Candidates are split into:

- process scorecards: empty `conversation_id`
- conversation scorecards: non-empty `conversation_id`

Submitted candidates remain prioritized before unsubmitted candidates.

### Auto-Heal Rollout Gates

Auto-heal remains protected by:

- `SCORECARD_SYNC_MONITOR_ENABLE_AUTO_HEAL`, default `false`
- `SCORECARD_SYNC_MONITOR_AUTO_HEAL_CUSTOMER_PROFILE_ALLOWLIST`
- drill-down guards so filtered investigations do not accidentally dispatch backfills
- `SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS`, default `1000`
- `SCORECARD_SYNC_MONITOR_MAX_SCORECARDS_PER_REINDEX_JOB`, default `5000`

`SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS` controls whether auto-heal is allowed for one customer/profile run. If candidates exceed the threshold, the monitor logs and reports an observe-only result instead of creating workflows.

`SCORECARD_SYNC_MONITOR_MAX_SCORECARDS_PER_REINDEX_JOB` controls payload chunking after auto-heal is allowed. This prevents oversized gRPC/Temporal payloads by splitting large accepted candidate sets into smaller workflows.

### Slack and Log Reporting

Slack/log output now includes:

- missing and stale counts
- stale reason counts
- created reindex job names
- Temporal workflow URLs
- chunk information for split jobs
- over-threshold skip messages with manual override guidance
- clearer footer wording: `Profiles` instead of `Tasks`, and `Scorecards` instead of `Total`

Slack startup was also hardened so the scorecard monitor summary Slack client does not conflict with generic cron error notification setup.

## Important Behavior

When candidate count is above the automatic threshold, the monitor does not create a workflow. Example:

```text
[scorecard-sync-monitor] cresta/walter-dev | backfill | skipped auto-heal candidates=1122 threshold=1000
```

Operators can manually rerun with a higher threshold, for example:

```bash
SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS=5000
```

This keeps routine auto-heal bounded while still allowing explicit manual cleanup.

## Validation

### Unit and Integration-Style Coverage

The code changes were validated with focused tests covering:

- matching submitted/update timestamps classify as synced
- submitted PG scorecard with CH submit time at Unix epoch classifies as stale
- unsubmitted PG scorecard with CH submit time at Unix epoch classifies as synced
- unsubmitted PG scorecard with non-default CH submit time classifies as stale
- CH update time older or newer than PG `updated_at` classifies as stale
- UTC/microsecond normalization avoids false positives
- absent CH row classifies as missing
- missing and stale scorecards both become reindex candidates
- submitted candidates are ordered before unsubmitted candidates
- auto-heal disabled logs dry-run and does not call `CreateJob`
- auto-heal enabled dispatches candidates through the reindex path
- allowlist and threshold gates prevent dispatch when expected
- over-threshold Slack formatting includes candidate count, threshold, and override env var
- created-job Slack output still includes workflow links

Primary package test command:

```bash
go test ./cron/task-runner/tasks/scorecard-sync-monitor
```

### Flux and Cron Startup Validation

Flux rendering validated the cron configuration changes across staging and production release paths.

The immediate rollout mitigation set `CRON_ERROR_CHANNEL` empty for `cron-scorecard-sync-monitor` so the scorecard summary Slack path could run without conflicting with generic cron error notification startup.

The task-runner code was then hardened so Slack module installation is safe and missing Slack token paths do not prevent task startup.

### Voice-Staging Manual Validation

Validation used:

- cluster: `voice-staging`
- namespace: `cresta-cron`
- CronJob: `cron-scorecard-sync-monitor`
- image: `main-20260622_203131z-aa39df67`
- customer/profile: `cresta/walter-dev`
- ClickHouse DB: `cresta_walter_dev`

Because `scorecard_d` is a Distributed table, test deletes were applied to the underlying `scorecard` table with `ALTER TABLE ... ON CLUSTER conversations DELETE ...`.

#### Test 1: Under-Threshold Auto-Heal

Range:

```text
2026-06-14T05:00:00Z to 2026-06-14T05:15:00Z
```

Result:

- ClickHouse rows were deleted for the target create-time range.
- Monitor found `748` reindex candidates.
- Threshold was `1000`.
- One reindex workflow was created.
- Log included a Temporal workflow URL.
- No `skipped auto-heal` line appeared.
- The targeted ClickHouse create-time range was restored to `740` rows.

#### Test 2: Over-Threshold Skip

Range:

```text
2026-06-14T06:00:00Z to 2026-06-14T07:00:00Z
```

Result:

- ClickHouse rows were deleted for the target create-time range.
- Monitor found `1122` reindex candidates.
- Threshold was `1000`.
- Monitor logged:

```text
[scorecard-sync-monitor] cresta/walter-dev | backfill | skipped auto-heal candidates=1122 threshold=1000
```

- No reindex workflow was created.

#### Test 3: Manual Override Backfill

Same range as Test 2, with:

```text
SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS=5000
```

Result:

- Monitor found the same `1122` candidates.
- One reindex workflow was created.
- Log included a Temporal workflow URL.
- The targeted ClickHouse create-time range was restored to `1110` rows.

### Final Verification Finding

After restoring the two injected create-time ranges, a dry-run over the broader validation window still reported:

```text
cresta/walter-dev | clickhouse_all | total=6399 missing=71 missing_rate=1.11%
cresta/walter-dev | clickhouse_unsubmitted | total=6399 missing=71 missing_rate=1.11%
```

A bounded cleanup run created a 71-scorecard reindex workflow, but follow-up dry-runs still reported the same 71 missing candidates. Since the injected ranges had already been restored, these 71 appear to be a separate residual set, likely pre-existing or not fixable by the current reindex path. They should be investigated separately from the threshold and workflow-dispatch validation.

## Current Rollout State

The merged monitor supports safe staged rollout:

1. Observe-only runs with auto-heal disabled.
2. Allowlisted auto-heal for selected customer/profile pairs.
3. Automatic skip above `SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS`.
4. Manual override by rerunning with a higher threshold.
5. Workflow payload protection through `SCORECARD_SYNC_MONITOR_MAX_SCORECARDS_PER_REINDEX_JOB`.

The voice-staging validation confirmed the key rollout behaviors:

- under-threshold auto-heal creates a workflow
- over-threshold auto-heal skips workflow creation
- manual threshold override creates the workflow
- targeted ClickHouse ranges can be restored by the generated workflows
- residual unsubmitted gaps may exist independently of the injected test gaps
