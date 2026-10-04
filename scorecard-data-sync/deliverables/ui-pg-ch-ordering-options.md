# Preserving scorecard action order from UI through PostgreSQL to ClickHouse

Date: 2026-09-19 (America/Toronto). Status: investigated design options; no implementation or rollout selected.

For a focused explanation of the timestamp conflict, start with [Why assigning PG updated_at to CH update_time breaks reindex cleanup](why-pg-updated-at-breaks-ch-cleanup.md). It introduces the assumptions first, then walks through ordering and deletion examples.

## Recommendation

Start with a **save-success barrier in the existing frontend queue, transactionally correct backend mutations, and coherent projection reads**. This requires no database schema change or historical backfill and addresses concrete races. It is a meaningful mitigation, not a complete delivery or ordering guarantee.

For a complete eventual-convergence contract, prefer **frontend ordering + server-owned aggregate revisions + a PostgreSQL transactional outbox + a revision-aware ClickHouse projection with explicit deletion semantics**. PostgreSQL and the existing worker/Temporal infrastructure can implement this without introducing Kafka. Source revisions solve stale replacement; the outbox solves lost delivery. Neither replaces the other.

Do not revive the old change that assigns PG `updated_at` to CH `update_time`. That column also powers reindex cleanup. The failure is an overloaded contract, not evidence that source versions are inherently unsuitable.

Two corrections to earlier knowledge are material:

- The historical explanation that an older PG timestamp overwrites a newer PG timestamp merely because its payload was captured is not a valid explanation of max-version replacement for the same key. The cleanup conflict is demonstrable in the original commit and current source.
- The main frontend form already serializes enqueued saves through `scope: { id: 'saveScorecard' }`. It is inaccurate to describe it as debounce-only. Debounce completion, successful-save dependency, other hooks, other tabs, and non-UI writers remain separate concerns.

## 1. What happened to the original timestamp proposal?

[PR #23999](https://github.com/cresta/go-servers/pull/23999), merged 2025-11-05, replaced `time.Now()` with `scorecard.UpdatedAt` in both scorecard and score builders. [PR #24095](https://github.com/cresta/go-servers/pull/24095), merged 2025-11-08, reverted those assignments. The final merged implementation used `UpdatedAt` directly; the intermediate `ScorecardExtra.Version` field described in the PR body was removed before merge.

### The exact Temporal-to-cleanup path

1. `ReindexConversations` enables `clean_up_clickhouse_scorecard_scores` for a run.
2. `getCleanUpClickhouseCutOffTime` chooses the job creation time by default; a valid explicitly earlier cutoff is allowed.
3. `conversation_reindexer.go` asks `BatchIndexConversations` to rebuild the `SCORECARD` and `SCORE` tables, bypassing their ordinary existing-ID dedup when cleanup is requested. It supplies the cutoff.
4. `BatchIndexConversations` enqueues a table/conversation/cutoff cleanup request when the cron deletion feature is enabled.
5. The offline `delete-conversations-in-clickhouse` task processes that queue and calls `DeleteConversationsFromClickhouseWithCutOffTime`.
6. The scorecard and score branches build a scoped deletion predicate containing **`update_time < cutoff`**.

The source comment explicitly says CH `update_time` means when the data is inserted. More precisely, builders stamp it with application `time.Now()` before the write; it is not a database acknowledgement or proof of query visibility. The replacement engine also uses this same column to choose the winning version.

Example:

| Event | Time |
|---|---|
| Scorecard last modified in PG | Monday 09:00 |
| Reindex job created; cleanup cutoff | Friday 10:00 |
| Correct unchanged PG row rebuilt and inserted into CH | Friday 10:05 |
| Offline cleanup executes | Friday 10:10 |

With the original meaning, `update_time=Friday 10:05`, so the rebuilt row survives. With the proposed meaning, `update_time=Monday 09:00`, so **the fresh row qualifies for deletion**. If every selected scorecard predates the cutoff, all those selected scorecard/score rows can qualify. The blast radius is the queued tables, conversations, and time restrictions; this is not an unconditional whole-cluster deletion. It depends on cleanup being enabled and running after the relevant inserts are visible. If a delayed insert arrives after the mutation, it can survive instead, producing timing-dependent results.

This is the **conversation reindex plus offline cleanup** path, not automatically every `reindex_scorecards` run. The newer scorecard-centric workflow has its own optional `CleanUpBeforeWrite` behavior; its normal insert-only mode must not be conflated with the cutoff sweep.

### Evidence strength and historical correction

- **Confirmed:** original change and revert, the cleanup contract in the exact original commit, the same mechanism in refreshed backend main, and an isolated ClickHouse mutation reproducing deletion of freshly indexed rows using old source timestamps.
- **Not independently established here:** which production workflow run/tenant triggered the November incident, its actual flags, exact affected IDs, or whether cleanup was the sole reason for the revert. The fetched revert body only says it reverts #23999; the original ticket comments link a missing-scorecards incident channel but do not contain the full postmortem. This report establishes why the proposal is unsafe, without inventing the missing incident evidence.
- **Correction:** the legacy `convi-5565-scorecard-ch-pg-sync/investigation.md` says stale captured data with an *earlier* timestamp overwrote the newer timestamp. That does not follow for the same replacement key when both versions use the intended source timestamp. Stale or internally inconsistent payloads remain real problems, but they need their own mechanism.

### Other problems a direct timestamp substitution leaves open

1. **Mixed version domains during migration.** A row previously stamped with a recent indexing time can outrank a correct reindex stamped with an old PG time. Fixing cleanup alone does not make repairs win. A controlled new table/cutover or carefully bounded reconstruction is needed; flipping writers is not enough.
2. **Timestamps are not automatically revisions.** The current field is a GORM-managed `time.Time`, not a proven database commit sequence. Equal microsecond values, different app clocks, timestamps assigned before lock waits, and writers that do not bump the parent require auditing. PostgreSQL `now()` is transaction-start time, and a free-standing sequence is allocation order, not generally commit order. A source-order token must be generated under the aggregate's serialization rule.
3. **The projection has multiple inputs.** Parent scorecard, criterion rows, template interpretation, conversation attributes, user flags, and derived fields can change independently. Reprojecting changed context at the same parent `updated_at` creates different payloads with equal versions unless the invalidation/version contract covers those inputs.
4. **Deletes and key moves are different from updates.** Rows removed from PG do not emit a newer replacement automatically. Mutable `agent_user_id` and `scorecard_time` participate in existing CH sort/shard placement; a higher version at a different key or shard does not remove the old row.
5. **Ordering does not deliver missing work.** A perfect version field cannot recover work lost between PG commit and asynchronous scheduling, nor make Distributed inserts instantly queryable.

## 2. Define the guarantee before choosing machinery

The practical target is **per-scorecard causal action order and eventual correct analytical state**, not a global clock order across all users.

- **UI intent:** edit A then edit B then Submit in one editor means the state submitted contains B. If autosaves coalesce, intermediate keystrokes need not all be persisted. Submit, reset, publish, and appeal transitions must remain barriers and must not be coalesced away.
- **PG acceptance:** an acknowledged mutation commits a coherent aggregate: parent totals/auto-fail and the final applicable criterion state agree. Another client's stale snapshot cannot silently overwrite a newer accepted revision.
- **CH safety:** delivery or retry of revision N cannot displace N+1 for the same logical entity. A deletion remains deleted despite a late pre-delete write.
- **CH liveness:** every accepted relevant change remains recoverably scheduled until its projection converges, under the explicit assumption that storage and workers eventually recover.
- **Read contract:** PG save success and CH visibility are different acknowledgements. Immediate read-your-write analytics needs a visibility barrier or PG-backed response/overlay; ordinary eventual projection cannot promise it.

Two independent tabs have no objective shared click order. Use a server-assigned accepted order and optimistic concurrency: the later stale client refreshes/rebases or gets an explicit conflict. Client wall-clock timestamps are not a solution to that ambiguity.

If the requirement is to preserve **every accepted intermediate action as history**, add an immutable event/audit retention contract. A latest-state CH projection, coalescing queue, or outbox that discards delivered events does not provide that history.

## 3. What the current implementation already does, and what it misses

| Boundary | Refreshed source behavior | Remaining issue |
|---|---|---|
| Main scorecard form | 300 ms debounce feeds `useSaveScorecardMutation`; shared TanStack scope serializes queued mutation functions | A not-yet-fired debounce is outside the queue. Submit directly calls SubmitScorecard for an existing draft and assumes autosave persisted the form |
| Other FE paths | Separate `useUpdateScorecard` and `useSubmitScorecard` hooks have no matching scope | Queue coverage is not universal; separate tabs have separate clients/caches |
| PG mutations | Update reads parent and prepares scores before entering the transaction; parent/child persistence is transactional | Atomic writes do not validate a stale prepared snapshot; aggregate calculation can disagree with criteria retained by merge semantics |
| Submit mitigation | PR #32447 preserves stored score and auto-fail and reloads, targeting one overwrite path | PR is **open/unmerged** at inspection; it does not solve general concurrent update/update ordering |
| Async projection | Re-reads parent from primary after commit, later reads children, then writes CH | Reads are not one aggregate snapshot. A concurrent mutation can occur between them or between read and insertion |
| Delivery | `ProductionAsyncWorkQueue` invokes `extrawork.RunWithTimeout` | This call is not a PG-transactional durable delivery record; there is a commit-to-scheduling failure window |
| CH storage | `ReplacingMergeTree(update_time)` in raw tables and lookup MVs; `update_time=time.Now()` | A delayed old snapshot can get a newer replacement version |
| CH reads | Some QA queries already select `max(scorecard_last_update_time)` and join matching parent/children | Read-time source ordering cannot recover a correct version already discarded by a merge using indexing time |
| Repair | Missing/stale monitor and targeted reindex exist | Useful fallback; timestamp/count checks alone do not prove full content consistency or deletion coverage |

The query-library check confirms both that the current scope serializes already-enqueued operations and that it starts subsequent mutations after an earlier mutation fails. Therefore, queue order alone is not a successful-save dependency. The actual UI disables/loading-gates some actions during pending work, but that does not cover an edit still inside the debounce window. Browser-level reproduction of the full form remains a follow-up.

## 4. Candidate solutions and tradeoffs

Effort labels are relative engineering/rollout estimates, not measured staffing quotes. Existing tooling means PostgreSQL, current services, Temporal, and ClickHouse. Options are composable; a CH-only option cannot establish UI order.

| Option | Scope and outcome | PG schema / CH schema | Historical work | Tools | Cost / complexity |
|---|---|---|---|---|---|
| **A. Harden existing FE queue + narrow BE fixes** | Save-success barrier; final-state PG calculation; coherent projection reads. Strong local improvements, incomplete end-to-end guarantee | None / none | No broad backfill; repair already-corrupt IDs separately | Existing | Low–medium / low–medium |
| **B. A + shared per-entity serialization** | Order participating mutations/projectors through PG locking or one keyed Temporal owner. Partial unless every writer, retry, delete and failover participates | None necessarily / none | None necessary for prevention | Existing PG/Temporal | Medium / medium–high; runtime contention |
| **C. A + hardened existing `updated_at` as CAS/version** | Avoid a new PG revision column by turning its existing timestamp into a strict logical token; CH uses source ordering separately from indexing | No new PG column, API/writer changes / CH engine or table changes | CH rebuild/cutover or explicit lazy-baseline semantics; targeted PG repairs | Existing | Medium–high / high contract-audit cost |
| **D. A + integer aggregate revision + versioned CH deletes** | Robust accepted-write and sink-order safety, but async delivery can still be lost | Revision (plus delete generation/state as needed) / version and deletion representation, stable identity | Baseline/rebuild or phased lazy initialization; full guarantee only for migrated scope | Existing | Medium–high / medium–high |
| **E. D + transactional outbox — preferred complete eventual design** | Adds recoverable delivery to revision safety; covers accepted changes and deletes when all writers participate | D + outbox / D | Initial snapshot/checkpoint for existing corpus; can phase by tenant/time range | PG poller; existing workers/Temporal optional | High initial / medium ongoing |
| **F. D + durable Temporal command owner before PG commit** | Temporal accepts ordered commands durably, then drives idempotent PG mutation and projection; avoids PG-to-Temporal start gap by changing entry point | Revision + persisted command dedup / D | Projection baseline; drain old entry points | Existing Temporal/PG/CH | High / high API and workflow lifecycle change |
| **G. D's FE/PG contract + WAL CDC projection** | Durable committed source stream; potentially broad coverage of DB writers and deletes; reconstruct aggregate changes before CH | Usually API revision/dedup still needed; logical replication setup / versioned projection | Consistent initial snapshot + WAL handoff and retained checkpoints | New CDC connector/stream processor; Kafka optional | High–very high / high operational complexity |

### A. Best first increment with no schema migration or backfill

**Frontend:** retain existing serialization. Route every mutation of a logical scorecard through a shared coordinator. A per-scorecard scope avoids unrelated cards blocking each other, but preserve a stable temporary identity through creation and all hooks. Coalesce only unsent autosaves into the latest complete form state. On Submit: cancel the old debounce, enqueue/save the latest form state, wait for that save to succeed, then submit; keep this as one ordered operation so later autosaves cannot interpose. Block Submit on save failure. Read current acknowledged state at dispatch, and prevent older responses/refetches from resetting newer unsaved edits. Reset, publish, appeal resolution, navigation and unmount need explicit handling too. Cancelling a browser request does not undo a server commit.

Simply calling `debounce.flush()` and immediately submitting is insufficient with a void-returning mutate interface and independent error handling. Provide an awaitable operation or implement the save-then-submit chain within the same queued mutation. Do not hold the shared mutation slot while awaiting another mutation queued behind itself; that would deadlock.

**Backend:** preserve narrowly owned fields in lifecycle mutations (the #32447 approach is one instance), lock/read the parent inside the transaction, apply edits to the current criterion state, and recompute totals from the final *applicable* criteria, respecting branches, N/A, weights, chapters, and auto-fail. Do not blindly sum all retained inactive rows. All aggregate writers need the same locking rule for this protection to hold. Without CAS, a stale full client snapshot can still overwrite newer values after acquiring the lock.

**Projector:** read parent and children under one primary-backed repeatable-read snapshot, or a locking/atomic query contract that actually guarantees consistency. Merely wrapping multiple SELECTs in PostgreSQL's default READ COMMITTED transaction is insufficient. A second version check before insert narrows races but is not a fence: an update can occur after the check.

**Outcome:** fixes specific local ordering, submit/autosave, and mixed-read cases. Does not guarantee multi-client stale-write rejection, durable delivery, sink monotonicity, or deletion safety. Existing incorrect data remains incorrect until repaired.

### B. Serialize using existing PG/Temporal primitives

For PG, serialize at the scorecard parent before reading/calculating. For projections, a shared advisory lock or a single Temporal owner keyed by tenant/profile/scorecard can coalesce work and read the latest committed aggregate. Real-time writes, AutoQM, scorecard/conversation reindex, reset/delete, appeals, scripts and repair paths must all honor the owner/fence.

This can be valuable without new tables, but a process-local mutex cannot coordinate pods. A lock around CH network I/O increases PG connection usage and couples write latency to CH availability. A lease can expire while its previous owner still writes; Temporal activity retries/timeouts can overlap in-flight external side effects. A sink-validated version/fencing token is needed for strict safety. Current wall clocks, backfill bypasses and the PG-commit-to-workflow-start gap remain concerns. Running only one worker is not a durable correctness proof.

### C. Reuse the existing timestamp, with explicit new semantics

Use exact PG `updated_at` as an opaque `expected_update_time` token. Under the parent lock, check equality and assign a strictly greater value, for example `GREATEST(clock_timestamp(), previous_updated_at + interval '1 microsecond')`; suppress ORM rewrites of that value. Every parent/child/deletion-relevant writer must follow this rule and transport full precision. This is a logical revision encoded as time, not reliance on time accurately reflecting causal order. Assess effects on UI date displays, exports and monitor windows if logical timestamps drift ahead.

In CH, keep indexing metadata separate. Existing `scorecard_last_update_time` could provide the source token without a new source-version column, but changing the replacement engine/version contract still requires a supported table migration and all associated readers/MVs. A query-only `argMax(source_time)` patch cannot undo destructive engine merges.

This saves one PG column, not the writer audit, stale-client protocol, cleanup redesign, cross-input invalidation, delivery guarantee, or migration work. It is a legitimate constrained option if schema constraints dominate, but an explicit integer revision is easier to reason about.

### D. Explicit aggregate revisions and deletion semantics

Each accepted mutation validates `expected_revision=N`, changes the whole aggregate atomically, and advances it to N+1 while holding the parent lock. Return the new token. Use the same rule for submission, reset, appeals, AutoQM and repairs; create needs uniqueness and idempotency. A blind increment after persisting stale data would merely version the wrong state.

The FE coordinator uses the revision returned by its own preceding successful save. For external edits, reject/rebase explicitly; do not silently retry a stale full snapshot with a newly fetched token. An operation/request ID with durable dedup state handles ambiguous retries after commit. Revision CAS alone detects a stale retry but cannot tell the client whether its earlier request succeeded.

Project immutable coherent snapshots with their actual revisions. Never label an old payload with a newly fetched revision. Every relevant input change must invalidate/bump the projection contract: either include dependency changes in the aggregate revision discipline or introduce an explicitly ordered projection revision with recorded input versions. A code/schema rebuild needs its own controlled generation/cutover; simply letting a newer schema number outrank all source revisions can promote stale data.

CH must select the greatest valid version and keep a stable logical replacement key and shard. Removed score rows need versioned tombstones or complete aggregate-snapshot semantics; replacing an ID after reset needs an identity/generation rule. Retain tombstones until every allowed replay/old writer is fenced out. Ordinary physical deletion followed by a late insert can resurrect data. Existing mutable sort/shard fields must be decoupled from identity or old locations explicitly invalidated.

**Cleanup cannot remain an inference that old business data was not reindexed.** Prefer explicit obsolete-key/tombstone handling or a completed rebuild generation. Even two columns are insufficient if a same-revision rewrite can merge away the freshly indexed copy before an `indexed_at` sweep: the cleanup contract must survive merges, failed rebuilds and concurrent writes. Stop/drain incompatible old cutoff jobs before cutover.

### E. Transactional outbox using the current stack

Within the mutation's PG transaction, persist the revisioned aggregate and an outbox item. A crash then leaves either neither or both. A poller claims committed items, retries with bounded backoff, and projects them; it can call an existing Temporal activity/workflow for durable orchestration. There is no requirement for a new message broker.

Two valid payload contracts have different costs:

- **Immutable revision snapshot:** payload is bound to the committed revision, including deleted keys/tombstones and needed input provenance. More storage and payload construction, easiest replay semantics.
- **Durable invalidation:** event records a dirty aggregate and source revision; worker reconstructs a coherent *current* snapshot and labels it with the revision actually read. It may skip intermediate states, so it is unsuitable as an action-history log. It must not clear a newer dirty revision when acknowledging older work. Hard deletion needs durable delete state, because there may no longer be a parent to reread.

Projection may succeed and the acknowledgement fail; replay must be harmless. Do not require exactly-once transport. Keep a delivery acknowledgement distinct from an analytical-visibility watermark: Distributed enqueue success is not query convergence. Retain sufficient replay/checkpoint history, monitor queue age and failed items, and reconcile the complete expected score/scorecard set.

**Guarantee:** for migrated writers/entities, accepted causal order and eventual latest-state/deletion convergence despite retries and crash windows, assuming recovery and faithful projection. It does not by itself provide simultaneous atomic reads across separate CH tables or immediate global visibility.

### F. Temporal as the durable command entry point

Durably accept a command into one entity workflow **before** the PG side effect; the workflow validates revision, runs an idempotent PG transaction keyed by command ID, and projects the result. Continue-As-New, workflow identity/reuse, owner generation, result reporting, and retry fencing are part of the design. Wait for PG commit before returning a “saved” response; a “command accepted” response alone has different UX semantics.

This can remove the need for a separate outbox for participating commands, but starting/signaling Temporal after an ordinary PG commit does not. Direct PG writers still need integration or another capture path. It adds workflow operations to the primary UI mutation path and is a larger contract change than an outbox poller.

### G. WAL CDC

Use a connector such as Debezium or an evaluated managed equivalent to capture committed transactions and deletes, preserving commit position plus intra-transaction order where needed. Raw table CDC does not preserve original UI click order and does not automatically produce a coherent scorecard aggregate. Parent and child records must be grouped/reconstructed at transaction boundaries; joining arbitrary “latest” rows in the projector can recreate mixed-version bugs.

Multi-table derived fields, external inputs, snapshot/WAL handoff, replication-slot retention, lag, failover/LSN epochs, partitioning, replay, deletions and schema evolution need explicit contracts. A sortable WAL position is not a substitute for source API stale-write protection. Prefer this when multiple domains need a maintained CDC platform, not solely to avoid adding one outbox table.

## 5. Things that do not establish the guarantee

- Increasing debounce, adding fixed sleeps, or hiding an older response reduces symptoms but does not order accepted writes.
- Making CH calls synchronous after PG commit still leaves two-store partial failure and ambiguous retry; it adds CH availability to UI latency.
- Read-primary, post-commit reread, or check-before-write alone leaves a read-to-write race.
- `max(now(), pg_updated_at)`, HLC wall timestamps generated at worker execution, or retry-time restamping can still reward old payloads with new versions.
- Changing only analytics queries cannot restore versions discarded by the raw table engine. `FINAL` also cannot remove rows under different full sorting keys/shards.
- An outbox without source-version protection can durably deliver the wrong winner; revisions without durable capture can safely order an incomplete stream.
- A larger auto-heal threshold improves repair coverage/capacity, not causal order or proof of content correctness.

## 6. Rollout and proof obligations

### Increment 1: A, bounded and inexpensive

Implement the FE success barrier across covered paths; test create, rapid edits, save failures, submit, reset, and navigation. Apply transaction/aggregate fixes and the scoped SubmitScorecard mitigation after normal review. Use coherent projector snapshots. Do not alter CH `update_time` semantics. No global historical backfill is required; track affected-ID repair separately. Measure saved-versus-submitted revision/state mismatches rather than claiming broad percentage improvement without a baseline.

### Increment 2: D + E for one bounded tenant/entity cohort

1. Inventory every writer and cleanup consumer, including existing Temporal histories/activities, old binaries, cron jobs, direct scripts and retries.
2. Add revision/request-id and outbox contracts, initially in shadow mode; validate parent/criterion invariants and snapshot identity.
3. Build a separate revision-aware CH projection, including lookup tables, derived fields, delete handling and stable routing. Plan a table rebuild rather than assume an engine-version change is a trivial metadata ALTER.
4. Bootstrap from a consistent PG snapshot with a durable change checkpoint; replay concurrent changes/deletes. Revision defaults or lazy initialization can avoid an eager PG rewrite, but do not establish correctness for untouched historical CH data.
5. Shadow-compare exact aggregate IDs, revision, criterion keys, payload fingerprints, submit/publish state and tombstones. Validate CH readers select the correct versions before and after merges.
6. Fence/drain old writers and queued cutoff cleanup; cut over readers by tenant/cohort. Retain a rollback/replay route that respects the new version domain. Do not mix old wall-time values and new logical revisions in the same replacement comparison.

No strict requirement for one all-customer backfill on day one: migrate a bounded cohort and state that guarantee's scope. A full-corpus guarantee eventually requires baselining/reconciling that corpus. “No backfill” cannot also mean “all historical corruption repaired.”

### If reads must never see mixed parent/child revisions

Per-row replacement plus an outbox gives eventual convergence, not a transaction across `scorecard` and `score`. Matching revision joins can temporarily yield missing rows while one half arrives. Options include a single CH aggregate row containing its criterion array, or retained revision snapshots plus a completion/publication contract that readers verify across all required data. A marker written last is not alone sufficient when tables/shards/replicas become visible in different orders. Keeping the previous complete revision available requires retaining it until the new one is publishable; a merge engine that already discarded it defeats that fallback. This stronger read guarantee adds schema, query and operational cost.

### Acceptance matrix

| Schedule | Required behavior |
|---|---|
| Edit B then Submit before 300 ms expires | Submit includes B and waits for successful persistence |
| Save fails, Submit queued | No submission of an older state; pending intent retained for recovery |
| Two tabs submit the same expected revision | One succeeds; the other explicitly resolves conflict |
| PG commit then process crash before scheduling | Outbox/command record still drives projection |
| CH write succeeds then worker crashes | Duplicate retry has no state regression |
| Old projection arrives after new projection | New source revision wins before/after merge |
| Parent update commits between parent/child reads | Project coherent revision or retry; never label a mixed snapshot as valid |
| Delete/reset then delayed pre-delete write | No resurrection, including after replay/tombstone retention boundaries |
| Historical unchanged row reindexed with cleanup | Correct row survives; obsolete keys removed explicitly |
| Removed criterion, changed agent/time/shard | No surviving old-key row in reader results |
| Context/template projection changes at same PG parent time | Explicit invalidation/revision or rebuild generation handles it |
| Distributed delivery/replica lag, parent/child partial write | Visibility remains pending or old complete snapshot served under chosen read contract |
| Old/new writer overlap during rollout | Old writers cannot outrank, resurrect or delete the new projection |

## 7. Validation performed and limits

Actual isolated ClickHouse **26.8.2.7** execution passed nine checks: wall-time stale overwrite; loss of recoverability after `OPTIMIZE FINAL`; source-version ordering; all rebuilt old-source rows matching cleanup; indexing-clock separation; actual cleanup deletion; mixed-version-domain migration failure; changed sorting-key duplication; newer tombstone suppression of a late insert. These use synthetic UInt64 clocks for deterministic ordering and actual MergeTree engines/mutation semantics. They are not tests of deployed versions, Replicated/Distributed delivery, production flags or tenant state.

The installed TanStack Query implementation passed three deterministic schedules: shared-scope serialization, Submit preceding a not-yet-enqueued debounce callback, and queue continuation after save failure. This checks library behavior and the source-derived gap; it is not a browser reproduction of the complete React form.

Reproduction files and output:

- [ClickHouse schedules](../sessions/2026-09-19/ordering-counterexamples.sql)
- [ClickHouse results](../sessions/2026-09-19/ordering-check-results.txt)
- [TanStack schedules](../sessions/2026-09-19/mutation-scope-check.mjs)
- [Session evidence and limits](../sessions/2026-09-19/codex-ui-pg-ch-ordering-options.md)

## Source map

Backend main was verified against GitHub at `ab941601e66af653b19a7cf91777b793bdedb2c7`; frontend fetched main `74c41e4585efd3440d32e74f416189cfe700602e`; schema fetched main `36b195838e9329751332af6488c0c3d4574b6ab3`. Existing checkouts were not switched. These are source snapshots, not deployment claims.

| Evidence | Pinned source |
|---|---|
| Original merged cleanup contract | [reindexer at #23999](https://github.com/cresta/go-servers/blob/d04ef2d3faeaadc0bb36760c1a75017675169b37/temporal/ingestion/reindexconversations/conversation_reindexer.go#L323), [cutoff deletes at same commit](https://github.com/cresta/go-servers/blob/d04ef2d3faeaadc0bb36760c1a75017675169b37/shared/clickhouse/conversations/conversation.go#L1586) |
| Current default cutoff | [activity.go](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/temporal/ingestion/reindexconversations/activity.go#L183) |
| Current cleanup handoff | [reindexer](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/temporal/ingestion/reindexconversations/conversation_reindexer.go#L308), [enqueue](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/apiserver/internal/internalconversation/action_batch_index_conversations.go#L813) |
| Offline delete and predicate | [cron task](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/cron/sync-users/internal/delete-conversations-in-clickhouse/delete_latest_conversations_in_queue.go#L130), [scorecard/score predicates](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/shared/clickhouse/conversations/conversation.go#L1661) |
| Current FE queue and submit branch | [shared scope](https://github.com/cresta/director/blob/74c41e4585efd3440d32e74f416189cfe700602e/packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts#L87), [direct submit](https://github.com/cresta/director/blob/74c41e4585efd3440d32e74f416189cfe700602e/packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts#L195), [debounce](https://github.com/cresta/director/blob/74c41e4585efd3440d32e74f416189cfe700602e/packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx#L509) |
| Mutation preparation before transaction | [UpdateScorecard](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/apiserver/internal/coaching/action_update_scorecard.go#L142) |
| Parent save and retained children | [persistence](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/shared/scoring/scorecard_scores_dao.go#L60), [merge semantics](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/shared/scoring/scorecard_scores_dao.go#L178) |
| Projection reads and async dispatch | [parent/child reads](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/apiserver/internal/coaching/action_create_scorecard.go#L682), [production queue](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/apiserver/internal/coaching/async_work_queue.go#L30) |
| Wall-time CH row version | [score rows](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/shared/clickhouse/conversations/scorecard_score.go#L326), [scorecard row](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/shared/clickhouse/conversations/conversation.go#L3065) |
| CH engines, keys, routing and MVs | [schema](https://github.com/cresta/clickhouse-schema/blob/36b195838e9329751332af6488c0c3d4574b6ab3/conversations/migrations/20230824160348_init_db.up.sql#L425) |
| Existing source-time read selection | [QA query builder](https://github.com/cresta/go-servers/blob/ab941601e66af653b19a7cf91777b793bdedb2c7/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L76) |
| Historical official issue | [CONVI-5565](https://linear.app/cresta/issue/CONVI-5565/data-sync-between-ch-and-postgres); issue and all returned comments inspected read-only |

Related domain context: [legacy theory archive](../../pg-ch-scorecard-sync-investigation/README.md), [architecture](architecture-and-invariants.md), [RCG PG inconsistency](../work-items/rcg-casino-dtq-score-mismatch.md), [Distributed-delivery incident](2026-08-us-east-distributed-backlog-incident.md).
