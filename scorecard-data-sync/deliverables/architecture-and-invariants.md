# Scorecard PG-to-ClickHouse Architecture and Invariants

## Contract

PostgreSQL owns authoritative scorecard business state. ClickHouse is a derived analytics projection and must eventually converge to the correct PostgreSQL-derived representation.

The projection is not a row-for-row copy:

- `director.scorecards` is the authoritative scorecard instance.
- `director.scores` is the authoritative criterion/chapter response state.
- `historic.scorecard_scores` is an intermediate denormalized projection used by some write paths, not an independent source of truth.
- ClickHouse `scorecard`/`scorecard_d` stores scorecard-level analytics rows.
- ClickHouse `score`/`score_d` stores emitted criterion-level rows.
- Template revisions and conversation/process context contribute derived fields and sharding inputs.

Therefore, a valid comparison must account for transformation rules. In particular, the ClickHouse writer validates template chapter scores but intentionally does not emit chapter aggregate rows to `score_d`; raw `director.scores` counts can legitimately be larger.

## Primary Projection Paths

| Path | Driver | Coverage | Important behavior |
|---|---|---|---|
| Coaching API real-time write | Create/update/submit/reset scorecard | Conversation and process scorecards | PG transaction commits first; ClickHouse work may run asynchronously and must re-read committed state |
| Conversation reindex | Conversation/time range via `BatchIndexConversations` | Conversation scorecards only | Conversation-centric selection excludes process scorecards |
| Scorecard reindex | `JOB_TYPE_REINDEX_SCORECARDS` | Targeted conversation or process scorecard IDs; scorecard-centric ranges where supported | Reconstructs from director data and uses production projection code |
| AutoQM/autoscoring | Conversation scoring workflow | Conversation scorecards | Separate synchronous/worker path; do not assume it shares manual update/submit races |
| Sync monitor/auto-heal | PG inventory compared with CH scorecard IDs | Missing standard conversation scorecards in the monitor's configured window | Detects absence, not stale fields on an existing CH row |

## Scorecard-Type Boundary

Conversation scorecards and process scorecards require different recovery selection and sharding:

- Conversation scorecards are discoverable from conversation-based paths and shard using conversation context/time.
- Process scorecards can have an empty `conversation_id`, use `process_interaction_at` for deterministic shard selection, and require scorecard-centric reindex.

Before CONVI-6298, a failed process-scorecard real-time write had no general reindex path. `JOB_TYPE_REINDEX_SCORECARDS` closed that coverage gap.

## Correctness Dimensions

Do not use a single count as proof of synchronization.

### Existence

- Every in-scope PG scorecard expected in analytics has a current CH `scorecard_d` row.
- Every in-scope emitted criterion has a current CH `score_d` row.

### Field correctness

- Score, submit/publish state, submitter, manual/AI flags, template identity/revision, agent/use case, and relevant timestamps match their canonical PG-derived values.

### Version correctness

- The winning ClickHouse version represents the latest valid source state, not merely the worker that wrote last.
- Delivery/worker time is not a reliable business version when an earlier snapshot can be written later.

### Projection correctness

- The expected criterion set is based on emitted criteria, excluding chapter aggregate rows and any intentionally filtered scorecard types.
- Delete/filter semantics are explicit: reindex inserts do not automatically remove rows that are no longer eligible.

### Freshness

- Temporary lag is acceptable only within a known window and only if convergence is detectable and repairable.
- A scorecard that exists but has stale submission fields is incorrect even when aggregate existence counts match.

## Ordering and Idempotency Rules

- PG mutations that must be atomic belong in the same transaction.
- Async projection code must read committed source state instead of relying on captured mutable objects.
- Repeated projection of the same state must be safe.
- ReplacingMergeTree behavior is only correct when its version column orders source truth correctly.
- Replay and repair must not allow an older source snapshot to dominate newer state.

## Boundary with `scorecard-workflows`

Use `scorecard-workflows` for whether a scorecard should be created, editable, submitted, published, appealed, or calibrated. Use this domain for whether an in-scope scorecard's analytical projection exists and matches PostgreSQL.

Examples:

- A newly created process scorecard is not visible to an immediate PG replica read: workflow/PG consistency.
- A committed process scorecard is absent from CH and cannot be conversation-reindexed: data sync.
- An appeal scorecard is intentionally excluded from analytics but stale CH rows remain: workflow eligibility defines the desired set; data sync owns cleanup/reprojection mechanics.
