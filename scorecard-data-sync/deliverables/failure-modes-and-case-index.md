# Failure Modes and Historical Case Index

## Taxonomy

### Omission

The expected projection attempt never produces a current CH row.

Typical causes include async failure/cancellation without durable replay, skipped intermediate writes, selection gaps, and absent recovery paths for a scorecard type.

### Staleness

The CH row exists but an older source snapshot wins because workers finish out of order, stale state is read before a later commit, or CH versioning follows delivery time rather than source truth.

### Partial projection

The scorecard row exists, but some derived fields or emitted score rows are missing/stale. First rule out expected transformation differences such as non-emitted chapter aggregate rows.

### Coverage gap

Prevention or repair exists but cannot select all valid entities, for example conversation-driven recovery for process scorecards, the wrong time axis, or empty-foreign-key assumptions.

### Cleanup/delete mismatch

The desired projection stops including an entity, but insert-only reindex leaves old CH rows behind. This requires explicit cleanup followed by re-projection.

### Observability/performance failure

The monitor is too coarse, too expensive, or unable to distinguish missing from stale/partial state.

## Case Index

| Case | Class | Durable lesson | Evidence |
|---|---|---|---|
| CONVI-5565 / CONVI-6076 | Staleness plus separate PG lost update | Atomic PG writes and fresh source reads reduced failures, but CH write-time ordering still allowed a narrow rapid update→submit race | `convi-5565-scorecard-ch-pg-sync/` |
| Missing `historic.scorecard_scores` | Omission | Async historic writes could fail or be skipped; synchronous transactional historic writes prevent orphaned director state | `historic-scorecard-missing/investigation.md` |
| Process-scorecard reindex (CONVI-6298) | Coverage gap | Conversation reindex cannot recover entities without conversation identity; scorecard-centric reindex is a separate first-class path | `convi-6298-reindex-process-scorecards/` |
| Pack Rat stale submit metadata | Staleness/partial projection | ID-existence monitoring missed an existing stale row; targeted scorecard reindex repaired submit metadata and emitted score rows | `pg-ch-scorecard-sync-investigation/sessions/2026-06-08/codex-pack-rat-scorecard-sync.md` |
| Appeal cleanup/backfill (CONVI-6209/6227) | Cleanup/delete mismatch | Changing filters does not remove legacy CH rows; delete from local tables on cluster, then re-backfill | `backfill-scorecards/README.md` |
| Sync monitor timeout / CONVI-7186 | Observability/performance | Correlated score-existence checks over huge unsubmitted shell populations made detection itself fail; filter missing unsubmitted IDs after CH comparison | `scorecard-data-sync/work-items/CONVI-7186.md` |

## Explicit Non-Cases

- `convi-6841-process-scorecard-update-race` is a read-after-create failure caused by PG replica lag before `UpdateScorecard`; no CH divergence is required to trigger it. Primary domain: `scorecard-workflows`.
- `hilton-coaching-discrepancy` covers coaching session/efficiency and user-filter semantics. It is not a scorecard PG/CH projection case.

Keeping these boundaries prevents every database-adjacent issue from being mislabeled as data sync.
