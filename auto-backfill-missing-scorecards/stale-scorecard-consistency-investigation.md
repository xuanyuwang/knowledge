# Stale Scorecard Consistency Investigation

**Created**: 2026-06-15  
**Tickets**: CONVI-7030, CONVI-6869  
**Related PRs**: go-servers#28608, go-servers#28610, go-servers#28694, flux-deployments#290691  
**Scope**: Explain why the monitor needs metadata consistency checks, why the ClickHouse read uses `argMax(..., update_time)`, and what rollout mitigations were added.

## Summary

The original `scorecard-sync-monitor` had a presence-only gap. It could detect:

- PG has scorecard ID `X`.
- ClickHouse `scorecard_d` has no row for `X`.

It could not detect:

- PG has scorecard ID `X`.
- ClickHouse `scorecard_d` also has row `X`.
- The ClickHouse row is stale, for example `scorecard_submit_time = 1970-01-01` while PG `director.scorecards.submitted_at` is populated.

The Pack Rat incident was in the second category. A presence-only query treated the scorecard as synced because a row existed, but analytics still saw stale metadata.

## V1 Consistency Check

V1 compares only the two timestamp fields that map directly from PG scorecard metadata to ClickHouse scorecard metadata:

| PG field | ClickHouse field | Purpose |
|---|---|---|
| `director.scorecards.submitted_at` | `scorecard_d.scorecard_submit_time` | Submitted vs unsubmitted state. `NULL` maps to Unix epoch. |
| `director.scorecards.updated_at` | `scorecard_d.scorecard_last_update_time` | Last scorecard metadata update. |

The monitor classifies each PG inventory row as:

- `missing`: no latest ClickHouse `scorecard_d` row exists.
- `stale`: a ClickHouse row exists, but either submit time or scorecard update time differs after UTC/microsecond normalization.
- `synced`: a ClickHouse row exists and both timestamps match.

Both `missing` and `stale` become reindex candidates. Submitted candidates are kept before unsubmitted candidates so downstream chunking preserves that priority.

Out of scope for V1:

- `submitter_user_id`
- `last_updator_user_id`
- `score_d` row count or content validation

## Why Use `argMax(..., update_time)`

The ClickHouse query in go-servers#28608 reads one metadata row per scorecard:

```sql
SELECT
  scorecard_id,
  argMax(scorecard_submit_time, update_time) AS scorecard_submit_time,
  argMax(scorecard_last_update_time, update_time) AS scorecard_last_update_time
FROM scorecard_d
WHERE scorecard_id IN (SELECT scorecard_id FROM scorecard_filter)
  AND _row_exists = 1
GROUP BY scorecard_id
```

`update_time` is not the business field being validated. It is the ClickHouse row version / ingestion timestamp used by `scorecard_d`'s `ReplicatedReplacingMergeTree(..., update_time)` engine. The business fields being validated are still:

- `scorecard_submit_time`
- `scorecard_last_update_time`

The reason to aggregate by `update_time` is that a ReplacingMergeTree table can expose multiple physical row versions before background merges complete. Querying the table without `FINAL` does not guarantee that only the latest physical row is visible. `argMax(value, update_time)` gives the monitor the metadata from the newest physical row per `scorecard_id` without forcing a `FINAL` query.

This matches the table's replacement semantics: when duplicate primary-key rows eventually merge, the row with the newest `update_time` wins. The monitor needs the same effective latest-row view during normal reads.

## Why Not Compare Against `scorecard_last_update_time`

`scorecard_last_update_time` is one of the values under validation. If the query selected the row by max `scorecard_last_update_time`, a stale row could mask the exact mismatch we are trying to detect.

Example:

1. PG says `updated_at = 2026-06-01T10:00:00Z`.
2. ClickHouse has multiple versions for the same scorecard ID.
3. We need the latest inserted ClickHouse row, then compare its `scorecard_last_update_time` against PG.

Using `update_time` answers "which ClickHouse physical row is latest?" Using `scorecard_last_update_time` would answer "which row claims the newest business scorecard update?" Those are different questions.

## Why Not Rely On ReplacingMergeTree Alone

ReplacingMergeTree deduplication is eventually applied during background merges. Until then, non-`FINAL` reads can still see older physical versions.

The monitor should avoid `FINAL` because it can be expensive across a broad scorecard ID inventory. The external-table filter keeps the query bounded to PG inventory IDs, and `argMax(..., update_time) GROUP BY scorecard_id` gives the latest-row metadata needed for classification.

## Rollout and Startup Findings

During rollout, scheduled monitor jobs were created and completed at the Kubernetes CronJob level, but no scorecard summary appeared in `#scorecard-sync-monitor`. GroundCover logs showed task-runner startup failed before running the task when both Slack paths were active:

- generic cron error notifier via `CRON_ERROR_CHANNEL`
- scorecard monitor summary Slack client

The immediate Flux mitigation is flux-deployments#290691:

- keep `CRON_ERROR_CHANNEL` empty for `cron-scorecard-sync-monitor`
- apply the targeted override across head, staging, prod early/main, and Schwab releases

The code-side fix is go-servers#28694:

- install the shared Slack module once in task-runner
- remove task-specific duplicate Slack module installation
- make missing `SLACK_TOKEN` resolve to a no-op Slack client so startup does not fail before task execution

## Review Comment Response

For the review question "why compare against the update_time?":

`argMax(..., update_time)` is used only to select the latest ClickHouse physical row for each `scorecard_id`. `update_time` is the ReplacingMergeTree version column, not the field being compared to PG. After selecting that latest row, the monitor compares PG `submitted_at` to CH `scorecard_submit_time` and PG `updated_at` to CH `scorecard_last_update_time`.

This avoids using `FINAL` while still matching ReplacingMergeTree's latest-version semantics.

## Validation Notes

Code validation in go-servers#28608 covers:

- matching submitted/update timestamps are synced
- submitted PG scorecard with default CH submit time is stale
- unsubmitted PG scorecard with default CH submit time is synced
- unsubmitted PG scorecard with non-default CH submit time is stale
- update-time mismatches are stale
- missing and stale candidates both enter the reindex batch
- submitted candidates are ordered before unsubmitted candidates

Flux validation for flux-deployments#290691 rendered `CRON_ERROR_CHANNEL: value: ""` for:

- `us-west-2-staging`
- `voice-staging`
- `us-east-1-prod`
- `us-west-2-prod`
- `voice-prod`
- `chat-prod`
- `comcast-prod`
- `eu-west-2-prod`
- `ca-central-1-prod`
- `ap-southeast-2-prod`
- `schwab-prod/prod`
- `schwab-prod/sandbox`
