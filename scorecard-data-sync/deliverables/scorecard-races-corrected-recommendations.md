# Scorecard races and sync delays: corrected recommendations

Updated: 2026-11-04 (America/Toronto). Investigation accepted; recommendations corrected from Xuanyu Wang's comments; redesign direction endorsed. This is documentation of the reviewed direction, not an implementation or deployment report.

Source: Bo Hua Tang, [Race condition & sync delay related issues](https://docs.google.com/document/d/1Fpsrlai5vEa1fAEZXIDym9g-6GziynAnCnKepASXNHQ/edit?tab=t.0#heading=h.da82wwzdbdzk), indexed modification time 2026-11-03 22:23:36 UTC. Glean returned the body and eight comments by Xuanyu dated November 3. [Comment record and retrieval limits](../sessions/2026-11-04/codex-scorecard-doc-corrections.md).

The document's investigation is sufficient for the next design discussion. Its proposed fixes must be read with the comments incorporated. The preferred larger redesign is a shared mutation function with explicit scorecard versions, optimistic conflict detection, caller-specific conflict handling, and a separate version in ClickHouse. PG `updated_at` and CH projection time must not substitute for that version.

## Part 1: issues and corrected actions

### 1. Two writers overwrite unrelated fields

Two writers read `{score: 10, comment: comment_1}`. A saves `{score: 20, comment: comment_1}`; B later saves its stale `{score: 10, comment: comment_2}`. A's score change is lost because B resends a field it did not intend to change.

**Action:** send and persist only intended fields, using field masks. Xuanyu identifies this as an inexpensive improvement with a prior field-mask precedent from Kurt. Treat that precedent as review context; its exact implementation was not located in this documentation task.

The backend must honor the mask through persistence. A frontend-only partial payload is insufficient if backend code still saves a stale full row. For score edits, merge the intended changes with stored score data and calculate parent/chapter totals from the resulting applicable criterion set, with the concurrency protection described in issue 2. Field masks reduce unrelated-field overwrites; they do not resolve two writers changing the same field.

### 2. Stored parent total differs from persisted score rows

The source example uses four equally weighted criteria. B stores Q1=1, Q2=0, Q3=1, Q4=1 and a 75% total. An older partial A then calculates 66.7% from Q1–Q3 while persistence retains B's Q4 row. The final criteria imply 75%, but the parent says 66.7%.

**Corrected action:** read, merge, calculate with `ComputeScores`, and persist the final applicable criterion state and parent/chapter totals under an actual concurrency rule. Wrapping those steps in a default PostgreSQL READ COMMITTED transaction provides atomic writes, but does not stop another writer changing rows after the read.

One implementation option is to lock the scorecard parent before loading the aggregate and hold the lock through calculation and commit. Every relevant writer must follow the same parent-lock rule; locking the parent alone does not prevent an independent child writer bypassing that rule. The preferred redesign below instead adds an expected version and an atomic version check, with the parent and children changed in the same transaction. A conflicting transaction must roll back all its writes. These are implementation requirements derived from the comment, not proof that a particular locking solution is already implemented.

Preserve active-branch and N/A rules: retained inactive scores must not enter totals merely because they remain stored.

### 3. A stale PostgreSQL replica feeds a write or reindex

The source cites [go-servers #32734](https://github.com/cresta/go-servers/pull/32734) for primary reads during scorecard-triggered search rebuilds and [#32447](https://github.com/cresta/go-servers/pull/32447) for submit reads. It describes a remaining close-workflow sequence: `TriggerAutoQa` writes PG while normally skipping its own reindex, then `ReindexConversation` reads a replica unless `CLOSE_CONVERSATION_REINDEX_USE_DATABASE_WRITE_REPLICA` is enabled.

**Conditional action:** assess the primary's load before enabling that flag or expanding primary reads in AutoQA reindexing. Xuanyu accepts primary reads if the primary can sustain the pressure; the correction does not authorize routing all reads to it. Confirm the interaction with the Insights team and [#33786 / INSI-4695](https://github.com/cresta/go-servers/pull/33786), whose comment says to turn it on before disabling dedup.

The source also proposes `UseDatabaseWriteReplica: true` for the AutoQA trigger's own reindex. Evaluate both paths together, including when that reindex is skipped. Primary reads address replica lag; they do not prevent an older snapshot from arriving at CH/ES after a newer one. Runtime flag values and capacity were not checked here.

### 4. Process scorecard autosaves arrive out of order

The source describes an unqueued process editor: autosave A changes 0→1 and autosave B changes 1→2; B reaches the backend first, then A restores 1.

**Action:** serialize process saves, waiting for the preceding save to finish. The source marks this done and links Director [#24480](https://github.com/cresta/director/pull/24480), [#24481](https://github.com/cresta/director/pull/24481), and [#24483](https://github.com/cresta/director/pull/24483). Their current merge and deployment status was not independently checked.

A local editor queue does not coordinate other browsers, backend writers, or replicas. Preserve save failures as failures; finishing a failed save does not authorize submitting older state.

### 5. A pending debounced autosave is queued after Submit

An edit happens at 0 ms, Submit is queued at 100 ms, and the edit's debounce fires at 300 ms. Submit can precede the visible edit even though already-enqueued operations share a queue.

**Corrected action:** flush the waiting autosave into the request queue when Submit is clicked, then enqueue Submit after it. Submission must depend on successful persistence of that visible state. Merely having a queue does not include work still waiting in the debounce timer.

The source marks the flush change done in Director [#24482](https://github.com/cresta/director/pull/24482) and says #32447 mitigates the related backend overwrite. Neither statement establishes safety across every editor or writer; current deployment was not checked. Xuanyu's comment specifically confirms that the waiting autosave needs to enter the queue at click time.

### 6. CH and ES projections can regress to older source state

The source describes overlapping copies from scorecard async work, scorecard-triggered reindex, close-workflow reindex, and repair/reindex jobs. Copy A can read 50%, pause, then build/write after copy B has read and written 75%.

For CH, the source says `ReplacingMergeTree(update_time)` can prefer A because its build-time stamp is newer. For ES, a full conversation `index` write without a version can let the later-arriving older payload win. ES's record includes more than scorecards.

**Rejected fix:** do not replace CH `update_time` with PG `updated_at`. Xuanyu's correction is that this overloads one field with source-order and insertion-time meanings. Cleanup uses it to remove old projections; a freshly rebuilt row carrying an old PG timestamp can be deleted. PG timestamps also need not advance for every relevant write and may depend on application clocks. See [the existing cleanup explanation](why-pg-updated-at-breaks-ch-cleanup.md) for the concrete source-backed counterexample and historical revert.

**Preferred direction:** use a dedicated scorecard version, transported with the coherent payload it describes, and use a separate CH version field for replacement. Preserve projection/indexing-time metadata and redesign cleanup explicitly. Adding a column alone is insufficient: engine migration, retained/deleted keys, routing, same-version rebuilds, and compatible readers still need a design.

The document describes a daily 10:00 UTC CH monitor with auto-heal that compares source timestamps, and proposes an analogous ES checker as a fallback. Repair reduces some drift; it is not a guarantee that every content mismatch is detected or repaired within a day. Xuanyu explicitly calls the daily CH refresh imperfect. Current schedule, configuration, and coverage were not reverified.

ES external versioning needs a coherent version for the entire conversation document. A scorecard version alone cannot order independent conversation inputs. An ES checker is a proposed eventual-repair measure; a full ES version contract remains open.

### 7. A conversation temporarily disappears from the PI drawer

The source describes parent `scorecard_d` and criterion `score_d` projections becoming visible at different times. A query joins the newest parent to scores with the same source update time; when the parent arrives first, the matching score rows are absent and the conversation disappears.

**Proposed read mitigation:** serve the latest complete version visible in both parent and criteria while the next version arrives. Reducing sync delay is useful but cannot eliminate independent table/replica visibility delays.

**Feasibility requirement carried forward from our existing analysis:** this is an option to validate, not a guaranteed small query fix. The previous complete version must remain available after merges, and completeness must mean the expected applicable criterion set, not merely one matching child row. Zero-score/all-N/A states, resets/deletes and eligibility changes need explicit treatment; fallback must not resurrect obsolete state. A completion marker by itself does not guarantee simultaneous visibility across shards/replicas. See [the existing read-contract analysis](ui-pg-ch-ordering-options.md#if-reads-must-never-see-mixed-parentchild-revisions).

## Part 2: preferred redesign

Xuanyu endorses the redesign, excluding the use of PG `updated_at` as a version. The document proposes:

1. **One shared scorecard mutation function.** All mutation paths use the same concurrency contract.
2. **A dedicated PG version.** Each update specifies the version it is based on. The acceptance check and increment are atomic, conceptually `UPDATE scorecards SET ..., version = version + 1 WHERE resource_id = :id AND version = :expected_version`.
3. **An explicit conflict result.** Of two writers expecting version 100, at most one accepts that version. A zero-row update returns a dedicated conflict code with current data. The API's exact code and response shape remain to be designed.
4. **Caller-specific recovery.** Compare intended fields with current state, drop obsolete work, rebase/resend an appropriate patch, or show the refreshed data and let the person decide. Do not blindly attach a new version to the old full snapshot.
5. **Coverage across all writers.** The source lists conversation/process autosave and submit, appeal request/resolution, reset, publish, acknowledge, backfill-publish cron, use-case changes, user migration, AI backfill, task-ID backfill, and similar maintenance paths. Incrementing versions only in the UI update endpoint leaves bypasses.
6. **A separate CH source version.** Use the greatest valid version to resolve replacement independently of projection time. The source expects new tables, data migration and removal of old tables because changing the replacement version is not a simple in-place column switch. The complete plan must include readers, materialized views, cleanup and old-writer overlap.

The accepted unit is the scorecard aggregate: parent, applicable criteria, chapter/overall totals and lifecycle changes must agree at one accepted version. Shared mutation logic must bind version validation and all those writes to one transaction. The exact implementation can use locks internally, but a transaction alone and a blind version increment after stale writes are insufficient.

A version detects stale edits and orders delivered snapshots. It does not by itself guarantee delivery after PG commit, coherent source reads, deletion handling, audit history, or atomic CH parent/child visibility. Those remain separate requirements.

### Sync redesign is still unspecified in this Google Doc

The heading “Redesign data sync from PG to CH/ES” has no body in the retrieved document. Do not attribute an outbox, CDC, or Temporal delivery selection to this source or to Xuanyu's agreement with optimistic concurrency.

Our [September options analysis](ui-pg-ch-ordering-options.md) separately recommends a transactional outbox for recoverable delivery and explicit deletion semantics. Retain that as prior technical analysis to evaluate alongside the preferred version design, not as a newly approved implementation choice.

## Part 3: optional audit history

The source proposes an audit table recording timestamp, actor, scorecard ID, and old/new changed values to explain score changes and answer customer questions. This remains a nice-to-have proposal. Latest-state CH projection and an outbox that discards delivered entries are not substitutes for retained audit history. Retention, access and exact audit coverage are not specified by the source.

## What to carry into implementation planning

| Area | Current direction | Still to establish |
|---|---|---|
| Intended-field writes | Apply field masks through backend persistence | Exact endpoint/writer coverage and prior precedent |
| Aggregate totals | Calculate from final applicable rows under concurrency protection | Lock/CAS implementation and deterministic competing-writer cases |
| Primary reads | Conditional on capacity and freshness needs | Load evidence, exact flag rollout and Insights coordination |
| Editor queues | Source marks process queues and submit flush done | Current PR/deployment state and failure behavior |
| Source and CH order | Prefer explicit versions and caller conflict handling | API/DB/table migration, all writers, cleanup and deletes |
| ES | Full-document versioning remains open; checker proposed | Version coverage and repair detection contract |
| PI visibility | Consider previous complete version | Retention/completeness and reset/N/A behavior |
| Reliable delivery | Separate design work | Decide whether prior outbox proposal fits |
| Audit | Optional history table | Retention and actor/change coverage |

No incident's exact request order is established by this synthesis. Existing RCG evidence remains evidence of parent/criterion mismatch, not proof of a specific race schedule. The next step is to turn the corrected direction into concrete implementation work without repeating the completed broad investigation.
