# PostgreSQL ↔ ClickHouse Scorecard Sync Investigation

**Created:** 2026-04-20
**Status:** Active

## Goal

Create a structured investigation and reusable Codex skill for scorecard sync issues between PostgreSQL (system of record) and ClickHouse (analytics store), starting from general distributed data-sync theory and narrowing toward concrete production failure modes and repair workflows.

## Why This Project Exists

Several prior efforts touched parts of the same problem from different angles:

- [backfill-scorecards](../backfill-scorecards/README.md)
- [convi-5565-scorecard-ch-pg-sync](../convi-5565-scorecard-ch-pg-sync/README.md)
- [hilton-coaching-discrepancy](../hilton-coaching-discrepancy/README.md)
- [mismatch-scorecard-count](../mismatch-scorecard-count/auto-heal-design.md)

Those projects are useful source material, but they start from concrete incidents, fixes, or recovery workflows. This project starts one layer higher: what does it take, in theory, to keep two databases in sync when one is transactional and one is analytical?

The point is to build a reusable mental model first, then encode it as an operational skill that can handle repeated sync tickets without rediscovering the same query ladder.

## Current Artifacts

- `deliverables/pg-ch-data-sync-investigator/SKILL.md` - reusable Codex skill for PG to CH sync incidents
- `sessions/2026-06-08/codex-pack-rat-scorecard-sync.md` - Pack Rat live incident investigation and repair notes
- `log/2026-06-08.md` - concise daily movement

## Working Approach

This is the right direction, with one guardrail: stay broad only long enough to build a useful taxonomy.

The investigation will move in four phases:

1. **General sync theory**
   Define common sync models, consistency tradeoffs, ordering guarantees, idempotency requirements, replay semantics, and reconciliation patterns.
2. **Constrained architecture**
   Add the real shape of the problem: PostgreSQL as source of truth, ClickHouse as derived analytics store, asynchronous writes, mutable scorecards, late updates, missing conversations, and backfills.
3. **Case mapping**
   Revisit prior incidents and map each one to the taxonomy rather than treating each as an isolated mystery.
4. **Concrete solutions**
   Produce candidate prevention, detection, and recovery strategies with clear tradeoffs.

## Core Questions

### Phase 1: Theory

- What are the standard patterns for keeping OLTP and OLAP systems aligned?
- What guarantees matter most: at-least-once delivery, ordering, deduplication, monotonic updates, convergence, auditability?
- What are the canonical failure modes when syncing mutable entities across stores?

### Phase 2: Real Constraints

- What is the actual scorecard lifecycle in PostgreSQL?
- Which scorecard mutations must appear in ClickHouse, and which can be derived or ignored?
- Where do ordering races exist today?
- What makes process scorecards and conversation scorecards different?
- Which edge cases break time-range backfills?

### Phase 3: Prior Cases

- Which incidents were caused by stale overwrites versus missing writes versus incomplete backfill coverage?
- Which fixes improved prevention, and which only improved repair?
- Which gaps remain systematic rather than incidental?

### Phase 4: Concrete Design

- What should the steady-state write path be?
- What should the detection path be?
- What should the repair path be?
- What invariants should be monitored continuously?

## Initial Investigation Frame

The starting assumption for this project:

- PostgreSQL is the source of truth for scorecard state.
- ClickHouse should be treated as a derived projection that must eventually converge to PostgreSQL.
- A useful design must cover three separate concerns:
  - **Prevention:** reduce or eliminate divergence at write time
  - **Detection:** identify divergence quickly and precisely
  - **Repair:** restore convergence safely and cheaply

That separation matters because previous work spans all three, but not always explicitly.

## Latest Case: Pack Rat Scorecard Submit Sync

On 2026-06-08, investigated Pack Rat platform conversation id `12112918`, internal conversation id `019e7e88-d362-72a5-be86-51c5ddd865bb`.

Key result:

- Conversation and message rows were synced between PG and CH.
- One scorecard, `019e7e99-e3ce-7f3e-9b03-4e4bccc61532`, had stale CH submit metadata.
- Targeted `JOB_TYPE_REINDEX_SCORECARDS` repaired `scorecard_d.scorecard_submit_time` and propagated the submit time to emitted `score_d` rows.
- Raw PG `director.scores` had six additional chapter aggregate rows not emitted by the CH reindex builder; the production builder validates chapter scores but intentionally skips emitting them as `score_d` rows.

## Expected Outputs

- A sync-failure taxonomy for PostgreSQL → ClickHouse scorecards
- A map from prior incidents to that taxonomy
- A list of invariants that define “in sync”
- A decision framework for prevention vs detection vs repair
- A shortlist of concrete next-step investigations or implementation proposals

## Notes For This Project

- Start broad, but always ask: does this concept help explain a real scorecard sync failure?
- Prefer reusable concepts over incident-specific storytelling in the early phase.
- Do not assume all mismatches share the same root cause.
- Separate “missing in ClickHouse”, “stale in ClickHouse”, and “not recoverable by time-range backfill” as distinct classes unless evidence proves otherwise.

## Next Suggested Docs

- `sync-theory.md` — canonical two-database sync models and failure taxonomy
- `scorecard-specific-constraints.md` — real-system constraints for scorecards
- `case-mapping.md` — prior projects mapped into the taxonomy
- `solution-space.md` — prevention, detection, and repair options
