# Codex Session: scorecard-sync-monitor timeout after CONVI-7158

**Date:** 2026-06-30  
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Branch context:** inspected `origin/main` at `ccc5cc5dd9d11839459dbe6efd3e26f963601236`  
**Related deployment repo:** `/Users/xuanyu.wang/repos/flux-deployments`, `origin/master`

## Prompt

Investigate why `cron-scorecard-sync-monitor` became too slow after the recent merged PR and caused large prod clusters to hit the 30-minute `activeDeadlineSeconds`.

Affected run: 2026-06-30 10:00Z daily CronJob. Failed at 10:30Z in `us-east-1-prod`, `us-west-2-prod`, and `voice-prod`.

## Code Findings

Recent relevant commit:

- `ef07ac421a6fd1a37cfa739bb648a2d5b55bd675` / PR `#29273`: `CONVI-7158 Exclude unbackfillable unsubmitted scorecards`

Current `checkClickhouseMissing` PG inventory query in `cron/task-runner/tasks/scorecard-sync-monitor/task.go`:

```sql
FROM director.scorecards AS sc
WHERE sc.customer = ? AND sc.profile = ?
  AND sc.calibrated_scorecard_id IS NULL
  AND (sc.scorecard_type IS NULL OR sc.scorecard_type = 0)
  AND (
    (sc.submitted_at IS NOT NULL AND sc.submitted_at >= ? AND sc.submitted_at < ?)
    OR (
      sc.submitted_at IS NULL
      AND sc.created_at >= ? AND sc.created_at < ?
      AND EXISTS (
        SELECT 1
        FROM director.scores s
        WHERE s.customer = sc.customer
          AND s.profile = sc.profile
          AND s.scorecard_id = sc.resource_id
      )
    )
  )
```

The same commit added `countExcludedUnsubmittedWithoutScores`, which separately runs:

```sql
SELECT count(*)
FROM director.scorecards AS sc
WHERE sc.customer = ? AND sc.profile = ?
  AND sc.calibrated_scorecard_id IS NULL
  AND (sc.scorecard_type IS NULL OR sc.scorecard_type = 0)
  AND sc.submitted_at IS NULL
  AND sc.created_at >= ? AND sc.created_at < ?
  AND NOT EXISTS (
    SELECT 1
    FROM director.scores s
    WHERE s.customer = sc.customer
      AND s.profile = sc.profile
      AND s.scorecard_id = sc.resource_id
  )
```

This means the new behavior pays for the unsubmitted shell population twice: once for the inventory `EXISTS`, and once for the visibility `NOT EXISTS` count.

## Index Findings

Live `oportun` database on `us-west-2-prod` matched schema:

- `director.scorecards` primary key: `(customer, profile, resource_id)`
- `ix_scorecard_cp_submit`: `(customer, profile, submitted_at)`
- `ix_scorecard_cp_update`: `(customer, profile, updated_at)`
- `ix_scorecard_cp_agent_created_submitted`: `(customer, profile, agent_user_id, submitted_at, created_at)`
- no `(customer, profile, created_at)` index
- no partial `submitted_at IS NULL` + `created_at` index
- `director.scores` primary key: `(customer, profile, resource_id)`
- `ix_scores_cp_scorecard_criterion_non_unique`: `(customer, profile, scorecard_id, criterion_identifier)`

## Live EXPLAIN Evidence

Profile tested: `oportun/us-west-2` in `us-west-2-prod`, read-only IAM connection, window `2026-06-29T10:00:00Z` to `2026-06-30T10:00:00Z`.

Current inventory query:

- `Bitmap Heap Scan on director.scorecards`
- `BitmapOr`
- submitted branch uses `ix_scorecard_cp_submit`
- unsubmitted branch uses `ix_scorecard_cp_agent_created_submitted` with very high estimated cost: `26340158.50`
- correlated subplan does `Index Only Scan using ix_scores_cp_scorecard_criterion_non_unique`
- total plan cost around `26340163`

Current excluded-count query:

- `Aggregate`
- `Nested Loop Anti Join`
- `Gather`
- `Parallel Seq Scan on director.scorecards sc`
- anti-join probe uses `ix_scores_cp_scorecard_criterion_non_unique`
- total plan cost around `24706746`
- bounded `EXPLAIN (ANALYZE, BUFFERS, TIMING OFF)` with `statement_timeout='45s'` canceled due to statement timeout.

Submitted-only split query:

- clean `Index Scan using ix_scorecard_cp_submit`
- total cost around `3.61`

Naive score-driven rewrite:

```sql
SELECT DISTINCT sc.resource_id, ...
FROM director.scores s
JOIN director.scorecards sc
  ON sc.customer = s.customer
 AND sc.profile = s.profile
 AND sc.resource_id = s.scorecard_id
WHERE s.customer = 'oportun'
  AND s.profile = 'us-west-2'
  AND sc.submitted_at IS NULL
  AND sc.created_at >= ...
```

Planner reordered this back to `Parallel Seq Scan on director.scorecards`, so a naive `FROM scores JOIN scorecards` code rewrite does not avoid the bad path.

Materialized scored-ID CTE:

- forced score-driven shape, but planned `Parallel Seq Scan on director.scores`
- estimated matching score rows in billions for `oportun/us-west-2`
- worse than current scorecards path because `director.scores` has no time column.

## GroundCover Corroboration

Narrow log/event queries around 2026-06-30 10:00Z-10:30Z:

- Events show `SuccessfulDelete` at `10:30:00Z` and `SawCompletedJob ... condition: Failed` at `10:30:01Z` for `us-east-1-prod`, `us-west-2-prod`, and `voice-prod`.
- `us-west-2-prod` pod: `cron-scorecard-sync-monitor-29713560-k62xx`.
- `oportun/us-west-2` logged `START` at `10:00:58.349992804Z` and no later monitor log before deletion.
- `cvs/us-west-2` logged `START` at `10:03:43.012108306Z`, then `clickhouse_all total=239898 missing=0` and `END` at `10:29:28.314283245Z`.
- No `CLUSTER SUMMARY` for `us-west-2-prod` before deletion, consistent with unfinished profile tasks preventing the collector from firing.
- Because `cvs/us-west-2` reached ClickHouse stats and `END`, ClickHouse metadata lookup can be large but is not the best-supported root cause. `oportun/us-west-2` stalling before any stats line aligns with the PG inventory/count path.

## Answers To Prompt Questions

1. Exact PG query: current inventory is the single scorecards query with submitted range OR unsubmitted created range plus correlated `EXISTS` against `director.scores`; an additional excluded-count query uses the same unsubmitted shell range with `NOT EXISTS`.
2. Indexed columns: submitted branch is efficient via `(customer, profile, submitted_at)`. Unsubmitted branch is not: there is no direct `(customer, profile, created_at)` or partial unsubmitted-created index. The `scores` existence check can use `(customer, profile, scorecard_id, criterion_identifier)`, but it is executed after selecting/scanning many scorecard shells.
3. Indexes: see Index Findings.
4. EXPLAIN for `oportun/us-west-2`: yes. The excluded-count query plans a parallel seq scan over `director.scorecards`; the inventory query has a huge-cost unsubmitted bitmap index scan and correlated scores subplan. `EXPLAIN ANALYZE` for count timed out at 45s.
5. Rewrite from `director.scores`: naive rewrite is not enough because PostgreSQL reorders it back to scorecards. A materialized scored-ID CTE scans too much of `director.scores` because scores has no time dimension. A score-driven path would need a bounded scored-ID source, such as a persisted/materialized scored-scorecard-id table keyed by customer/profile and maybe scorecard_id, or an additional time-capable source.
6. Split submitted and unsubmitted inventory: yes. Submitted should remain its own `submitted_at` indexed query. Unsubmitted should not be hidden inside an OR with submitted.
7. Add index: useful but not the whole fix. A partial index such as `(customer, profile, created_at) WHERE submitted_at IS NULL AND calibrated_scorecard_id IS NULL AND (scorecard_type IS NULL OR scorecard_type = 0)` would prevent full/poor scans for unsubmitted shells, but still iterates all shell rows and probes `scores`, and the extra excluded-count query doubles the work. Prefer first removing the extra count and splitting queries; then add the partial index if unsubmitted shell scans remain too expensive.
8. ClickHouse contribution: GroundCover shows large `cvs/us-west-2` reached ClickHouse stats and `END`. The strongest evidence points to PG inventory/count as the bottleneck, especially `oportun/us-west-2` starting at 10:00:58 with no stats before deletion and live PG count timing out.

## Minimal Fix Proposal

Fastest safe code change:

1. Split inventory into submitted and unsubmitted queries.
2. Keep submitted query exactly index-driven by `submitted_at`.
3. For unsubmitted, keep the `EXISTS` behavior for correctness but remove `countExcludedUnsubmittedWithoutScores` from the critical path. If the excluded metric is still wanted, make it optional/env-gated or approximate/offline.
4. Add query timing logs around PG inventory, excluded count if retained, ClickHouse metadata, and reindex dispatch to make future bottlenecks visible.

Schema follow-up:

- Add a concurrent partial index for unsubmitted inventory if the split/removal does not bring runtime under budget:

```sql
CREATE INDEX CONCURRENTLY ix_scorecards_cp_unsubmitted_created
ON director.scorecards (customer, profile, created_at)
WHERE submitted_at IS NULL
  AND calibrated_scorecard_id IS NULL
  AND (scorecard_type IS NULL OR scorecard_type = 0);
```

This index should be validated against write overhead and migration safety before rollout.

Longer-term:

- Avoid using `director.scorecards` shell rows as the driver for scored unsubmitted inventory. Create/derive a bounded scored-scorecard inventory keyed by customer/profile/scorecard_id, or add a scorecard-level marker that indicates score rows exist and is maintained at write time. Then unsubmitted inventory can be filtered without probing `scores` per shell.

## Test Plan

- Unit/integration tests:
  - submitted scorecards still included by submitted time;
  - unsubmitted scorecard with at least one `director.scores` row included by created time;
  - unsubmitted scorecard without scores excluded;
  - stale/missing classification unchanged;
  - excluded count removed/gated does not affect Slack summary totals.
- SQL validation:
  - `EXPLAIN` submitted split query on `oportun/us-west-2` uses `ix_scorecard_cp_submit`;
  - `EXPLAIN` unsubmitted query no longer has OR with submitted branch;
  - if adding index, validate it is used for unsubmitted range and that the query does not seq scan `scorecards`.
- Prod validation:
  - run one manual `SCORECARD_SYNC_MONITOR_CUSTOMER_IDS=oportun` job on `us-west-2-prod` with auto-heal disabled or normal safety threshold;
  - confirm PG inventory timing logs are under target and cluster summary emits;
  - then run full `us-west-2-prod` manually before waiting for the next 10:00Z CronJob.

## Follow-up: index cost discussion

On the same read-only `oportun` database, catalog stats showed:

- `director.scorecards`: estimated `606,283,712` rows, `145 GB` heap, `330 GB` indexes, `476 GB` total.
- `director.scores`: estimated `7,810,510,848` rows, `1761 GB` heap, `2223 GB` indexes, `3985 GB` total.
- Existing `scorecards` index sizes:
  - `(customer, profile, conversation_id, template_id, template_revision)`: `114 GB`
  - `(customer, profile, agent_user_id, submitted_at, created_at)`: `80 GB`
  - primary key `(customer, profile, resource_id)`: `73 GB`
  - `(customer, profile, updated_at)`: `39 GB`
  - `(customer, profile, submitted_at)`: `7053 MB`
- `submitted_at` null fraction is `0.99946666`, so a partial index with only `WHERE submitted_at IS NULL` would still cover almost the whole table for `oportun`.
- `created_at` correlation is high (`0.98648167`), which means a created-at range index could be planner-friendly, but the index would still be large if it covers nearly all unsubmitted rows.

Implication: an index can materially improve the unsubmitted range lookup, but the cheapest useful index would need to stay narrow. A broad `(customer, profile, created_at) WHERE submitted_at IS NULL` index is likely tens of GB on `oportun`; a partial predicate that also includes non-calibrated standard scorecards may not reduce much for this profile because `calibrated_scorecard_id` and `scorecard_type` are also effectively null in stats.
