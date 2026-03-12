# Staff Projects

## Project 1: User Filter Consolidation (Low-cost, High-leverage)

## Why this is the “lowest cost” Staff project in this repo

This project already contains:
- A concrete unification goal and migration plan (`user-filter-consolidation/README.md`)
- Behavioral standards and analysis documents (`user-filter-consolidation/user-filter-behavioral-standard.md`, etc.)
- Evidence of ongoing/partial migration and real bugs fixed (log + evaluation docs)

That means the cost is mostly **finishing + standardizing + rolling out**, not starting from zero.

## Staff-level framing

### Problem
Multiple implementations of user-filter parsing across services lead to:
- Divergent semantics (union/intersection, empty-filter meaning, role filtering)
- Security/correctness risk via ACL + group expansion bugs
- Slow iteration and repeated refactoring across endpoints

### North-star outcome
One canonical, well-tested, well-observed user-filter implementation with explicitly documented semantics that becomes the default across services.

### Success metrics (fill in as we go)
- Correctness: reduce “filter semantics” bugs to near-zero for migrated endpoints
- Adoption: % of endpoints using the canonical path
- Safety: zero unauthorized-data incidents attributable to filter logic
- Dev velocity: reduce per-endpoint migration time via templates/helpers
- Performance: no regression (or measurable improvement) in p95 latency for key APIs

## What “moving from senior → staff” looks like on this project

- Write a semantic standard that other teams can cite and implement against (not just “my code”).
- Provide a migration path that is safe by default (feature flags, staged rollout, backout).
- Align multiple teams/services on a single definition of “correct”.
- Make adoption easy: helper APIs, examples, and tests that reduce cognitive load.
- Publish post-launch results and institutionalize learnings.

## Concrete work items (staff-level deliverables)

1) **Define invariants** (source-of-truth semantics)
- Union/intersection rules when users + groups are provided
- Role-filter behavior during group expansion
- Empty filter semantics under different ACL contexts

2) **Create a canonical package contract**
- Stable API surface (inputs/outputs/options)
- Backward-compat plan and deprecation milestones

3) **Harden correctness**
- Table-driven tests for edge cases + regression suite
- Property-style checks for invariants (where applicable)

4) **Rollout discipline**
- Feature-flag strategy + staged rollout (by endpoint / by customer / by traffic slice)
- Observability: metrics, logs, dashboards, and alert thresholds
- Backout plan and clear ownership during rollout

5) **Adoption engine**
- Migration guide and “golden example” PR
- Small refactor templates for common call sites
- Office hours / async Q&A doc for adopters

## 30/60/90 day plan (pragmatic)

- 30 days: lock semantics + tests + canonical API contract; migrate 1–2 high-traffic endpoints safely
- 60 days: migrate the majority of call sites; publish dashboards + rollout playbook
- 90 days: deprecate legacy paths; write post-launch evaluation and finalize standards

---

## Project 2: ClickHouse External Tables for Reference Data Filtering

**Design review:** [`large-user-id-clickhouse/design-review.md`](../large-user-id-clickhouse/design-review.md)
**Full project docs:** [`large-user-id-clickhouse/`](../large-user-id-clickhouse/)

### Why this is a Staff-level project

This project demonstrates multiple Staff dimensions simultaneously:

1. **Problem framing & strategy** — Identified that the immediate bug (user ID list exceeding ClickHouse max query size) was a symptom of a systemic gap: no general-purpose mechanism for passing reference data into ClickHouse queries. Reframed from "fix the user filter" to "build a reusable pattern for any external data."

2. **Options analysis & principled decision-making** — Evaluated 10 candidate solutions across complexity, generalizability, performance, and blast radius. Wrote a structured comparison ([`solutions-comparison.md`](../large-user-id-clickhouse/solutions-comparison.md)) and chose `ext` external tables with clear rationale, not just the first thing that worked.

3. **Architecture for leverage** — Designed the solution as two generic helper functions (`buildExtTable`, `attachExtTablesToContext`) that work for any data type, not just user IDs. Future use cases (scorecard IDs, conversation IDs) can reuse the same pattern with zero new infrastructure.

4. **Execution via minimal blast radius** — Zero changes to the shared ClickHouse query layer. Feature-flagged rollout with instant rollback. All 17 caller files follow a 3-line adoption pattern. No schema changes, no infra coordination.

5. **Rollout discipline** — Staged rollout plan (flag disabled → staging validation → production → flag cleanup), explicit testing matrix, and backout plan documented before any production change.

6. **Clear communication** — Produced a design review doc that frames the problem for a broad audience, includes benchmarks (ext tables 3.3x faster at 10K users), and makes the decision transparent.

### Staff artifacts produced

| Artifact | Location |
|----------|----------|
| Problem statement + systemic framing | `design-review.md` → Goal & Background |
| Options analysis (10 solutions) | `solutions-comparison.md` |
| Tradeoff-based decision record | `design-review.md` → "Why ext tables over alternatives" |
| Performance benchmarks | `design-review.md` → Performance section |
| Rollout plan + backout plan | `design-review.md` → Release Plans |
| Testing plan (staging + prod) | `testing-plan.md` |
| Implementation plan | `implementation-plan.md` |

### Mapping to Staff gaps (from `senior-to-staff.md`)

| Gap | How this project addresses it |
|-----|-------------------------------|
| Scope & ownership | Owned end-to-end: from bug discovery → systemic reframing → design → implementation → rollout plan |
| Problem framing & strategy | Reframed a point fix into a general-purpose mechanism; produced structured options + decision |
| Architecture & correctness | Designed for evolution (generic helpers, not user-ID-specific); dual path is temporary by design |
| Execution via leverage | Pattern is reusable for any reference data; 17 callers adopt with 3-line change |
| Influence without authority | Design review shared with team; decision rationale documented for alignment |
| Operational excellence | Feature flag, staged rollout, testing matrix, backout plan |
