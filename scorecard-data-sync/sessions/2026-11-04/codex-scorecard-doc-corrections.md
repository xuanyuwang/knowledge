# Incorporate reviewed scorecard race document

Date: 2026-11-04 (America/Toronto).

## Request and source

Xuanyu said the investigation is sufficient, the proposed fixes need the corrections in his comments, and the redesign matches his thinking. He requested incorporation into local documents, commit, and push.

Primary home: `scorecard-data-sync`; workflow-only context is linked from `scorecard-workflows`. Source repos resolve through `workspace/repos.yaml` to `go-servers`, `director` and `clickhouse-schema`; this task edits knowledge only, directly on its main checkout.

Retrieved through Glean `read_document`: Bo Hua Tang's [Race condition & sync delay related issues](https://docs.google.com/document/d/1Fpsrlai5vEa1fAEZXIDym9g-6GziynAnCnKepASXNHQ/edit?tab=t.0#heading=h.da82wwzdbdzk). Returned indexed update time: 2026-11-03T22:23:36Z. The rich document contains the document body and eight comments, all authored by Xuanyu Wang. The synthesized body is preserved in the [corrected recommendations](../../deliverables/scorecard-races-corrected-recommendations.md).

## Comment record

Text below is transcribed from returned comment text, with HTML entities decoded and `<br>` rendered as line breaks. IDs and timestamps retain source identifiers. The response did not provide highlighted ranges, reply threads, or resolved-state metadata, so issue associations below are based on comment meaning. We cannot assert that these eight indexed comments are every comment in the live Google Docs interface.

### Field masks — issue 1

ID: `doco.KTti1KNixLNEuaCEBGeGjQ`; created: 2026-11-03T16:34:31Z.

> This is a low-hanging fruit that can bring huge benefit. Kurt did that before by introducing field masks

Applied: prefer intended-field patches/field masks, while distinguishing same-field conflict and derived-total consistency.

### Transaction isolation — issue 2

ID: `doco._PjL5PTgf3jtJUvcWHvFKQ`; created: 2026-11-03T16:33:20Z.

> Starting a transaction does not stop other transactions from updating those rows with the default Postgres lock (read committed).
>
> It means the data on DB can be updated between step 1 & 2. So there is no guarantee that the merging can keep the data up-to-date.
>
> We may need to stronger transaction lock to prevent other transactions to update the rows until the current transaction is finished

Applied: require a shared locking or optimistic concurrency rule covering read/merge/recompute/persist; default READ COMMITTED alone is insufficient.

### Primary load — issue 3

ID: `doco.APslHIWqUSg1hQk3XDIC6Q`; created: 2026-11-03T16:38:43Z.

> I'm a bit concerned about we read all data from write replica for performance reason. I don't want to abuse the write replica so that the read-write replica doesn't provide any value more.
> If the write replica is OK with the pressure then I'm fine with this fix.

Applied: make primary-read expansion conditional on capacity evidence and coordination; do not claim the load assessment is done.

### Queue flush — issue 5

ID: `doco.pIl63Jt0Zn1wgWlCJGqbAg`; created: 2026-11-03T17:04:20Z.

> it means we need to dump the waiting autosave into the request queue (assume there is one) when submit is clicked?

Applied: pending debounce enters the save queue at Submit click, then Submit follows successful save.

### Timestamp has two meanings — issue 6

ID: `doco.AnRUv0yBgtDK2nKR3eBOLQ`; created: 2026-11-03T17:14:23Z.

> The problem of this approach is that `updated_time` will serve dual-purpose: the CH insertion time, and scorecard version. Therefore, the two purposes can conflict and cause troubles. For example, there is one backfill job use it as insertion time to clean up all stale records.

Applied: reject substituting PG `updated_at` for CH `update_time`; link the pre-existing source-backed cleanup explanation. The comment spells the field `updated_time`; the synthesis uses the implementation name `update_time` used in the document and existing evidence.

### Dedicated version question — issue 6 / redesign

ID: `doco.IiAtdbBiRUOpYF-t9NM4Og`; created: 2026-11-03T17:16:41Z.

> Will a separate scorecard version help?

Applied: dedicated version is the preferred redesign, with coherent payload, all-writer and migration requirements; not a delivery or visibility guarantee by itself.

### Daily refresh limitations — issue 6

ID: `doco.u_smcmiNEKqRv71UAEP1uQ`; created: 2026-11-03T17:15:51Z.

> The current solution is having a daily running job to refresh scorecards on CH. It's not a perfect solution and has limitations. Ideally, there should be a column dedicated to denote scorecard version

Applied: keep auto-heal as imperfect repair; do not promise all mismatches heal within a day.

### Redesign endorsement — part 2

ID: `doco.muVKnf2YS5W1n4yZHC-uAw`; created: 2026-11-03T18:32:37Z.

> Agree with this proposal (except of using PR's updated_at as version).

Applied: preserve the comment verbatim; “PR's” is interpreted in context as PG's `updated_at`, matching the proposal under discussion. The preferred design uses an explicit version. The live user request independently confirms that the redesign matches his thinking.

## Integration and evidence limits

- Added one corrected reference covering all seven issues, preferred optimistic-version redesign, the empty sync-redesign heading and optional audit table.
- Updated CONVI-7784's local current state without changing official Linear workflow. The prior “Linear Backlog” label is historical, not freshly verified.
- Updated the prior ordering work item and options document to distinguish the endorsed integer-version direction from the older timestamp-token option and separate outbox proposal.
- Linked from both domain READMEs and updated sync-domain project metadata.
- Preserved source-reported Done status and PR links for process queues / submit flush. PR merge status, deployment, flags, monitor schedule and primary load were not reverified; no new source/production investigation was needed for this documentation task.
- Existing retention/completeness, cleanup, delivery and deletion constraints remain explicit. The Google Doc's PG→CH/ES sync-redesign heading is empty; no particular delivery architecture is inferred from it.
- Used Glean's configured connector authentication and Git's configured authentication through the programs themselves. No AWS, Okta, Azure or SSH credential/profile was explicitly selected, read, exported or passed to another tool.
- The checkout had unrelated modified and untracked files before this task; stage only this task's explicit file list.

## Validation and handoff

Reviewed the staged diff and validated local Markdown targets across the ten task files, seven issue sections and eight unique source comment IDs. `git diff --check` and `git diff --cached --check` passed; the staged path set exactly matched the ten intended documentation files. This is documentation-only; product tests and production actions are not part of validation. Fetched origin before committing: main was one existing local commit ahead, with no remote-only commits. Commit and push to `knowledge` main as requested; confirm remote/local commit identity after push.
