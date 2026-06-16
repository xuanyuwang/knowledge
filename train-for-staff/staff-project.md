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

## Key insights (from blog writeup)

- **Behavioral standard was the highest-leverage artifact** — turned every PR review from a debate about "correct behavior" into a check against the spec. Also served as onboarding material.
- **Treated migration as a product** — dashboard tracking 29 APIs total, which implementation each used, divergence status, migration progress (12/29 → 29/29).
- **Incremental migration beat clean rewrite** — each endpoint migrated one at a time, verified against behavioral standard. Lower risk, delivered value sooner.
- **Feature interactions are the real risk** — `ShouldQueryAllUsers` + `exclude_deactivated` + external tables interacted in ways unit tests couldn't catch. Shadow mode (run both paths, compare) was the most reliable validation.

## 30/60/90 day plan (pragmatic)

- 30 days: lock semantics + tests + canonical API contract; migrate 1–2 high-traffic endpoints safely
- 60 days: migrate the majority of call sites; publish dashboards + rollout playbook
- 90 days: deprecate legacy paths; write post-launch evaluation and finalize standards

### Staff artifacts produced

| Artifact | Location |
|----------|----------|
| Behavioral standard (implementation-agnostic spec) | `user-filter-consolidation/user-filter-behavioral-standard.md` |
| Migration tracking (29 APIs) | `user-filter-consolidation/README.md` |
| Blog writeup (staff perspective) | [`blog/2026-02-28-staff-perspective-user-filter-consolidation.md`](../blog/2026-02-28-staff-perspective-user-filter-consolidation.md) |

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
| Blog writeup (staff perspective) | [`blog/2026-03-13-ext-tables-clickhouse-reference-data.md`](../blog/2026-03-13-ext-tables-clickhouse-reference-data.md) |

### Key insights (from blog writeup)

- **Solved for the class, not the instance** — reframed "user IDs exceed 1MB" into "reference data from app DB needs to reach analytics DB without embedding in SQL text." External tables solve any reference data need (user IDs, group IDs, conversation IDs).
- **"Always ext" beat threshold branching** — traded ~17ms overhead for <50 users for dramatically simpler code (one path, no threshold tuning). Production p50 has 200+ users, so tradeoff is overwhelmingly positive.
- **Shadow mode was highest-leverage testing** — ran both code paths on 10,000+ staging queries, compared results, found 0 mismatches. Caught the `ShouldQueryAllUsers` interaction bug before any customer was affected.
- **4-phase rollout** — dev/staging → shadow mode → production canary (one customer, one week) → global rollout.

### Mapping to Staff gaps (from `senior-to-staff.md`)

| Gap | How this project addresses it |
|-----|-------------------------------|
| Scope & ownership | Owned end-to-end: from bug discovery → systemic reframing → design → implementation → rollout plan |
| Problem framing & strategy | Reframed a point fix into a general-purpose mechanism; produced structured options + decision |
| Architecture & correctness | Designed for evolution (generic helpers, not user-ID-specific); dual path is temporary by design |
| Execution via leverage | Pattern is reusable for any reference data; 17 callers adopt with 3-line change |
| Influence without authority | Design review shared with team; decision rationale documented for alignment |
| Operational excellence | Feature flag, staged rollout, testing matrix, backout plan |

---

## Project 3: Scorecard PG↔ClickHouse Sync Fix (CONVI-5565)

**Investigation doc:** [`convi-5565-scorecard-ch-pg-sync/investigation.md`](../convi-5565-scorecard-ch-pg-sync/investigation.md)
**Notion design doc:** [Scorecard Async Database Updates](https://www.notion.so/cresta/Scorecard-Async-Database-Updates-2974a587b06180f595c3c14492a96104)
**Full project docs:** [`convi-5565-scorecard-ch-pg-sync/`](../convi-5565-scorecard-ch-pg-sync/)

### Why this is a Staff-level project

This project demonstrates a key Staff pattern: **investigating a systemic problem, aligning with leadership on the approach, pushing back with evidence when the proposed fix fails, and delivering a proven solution with a reusable pattern.**

#### The narrative arc

1. **Investigated and framed the problem** — Scorecard data was inconsistent between PostgreSQL (source of truth) and ClickHouse (analytics). Async work from UpdateScorecard and SubmitScorecard APIs could finish out of order, causing ClickHouse to keep stale data. Multiple customers affected (Spirit, SnapFinance). Wrote a detailed investigation doc on Notion and called for a design review.

2. **CTO proposed timestamp-based fix → caused a P2** — During review, CTO proposed using PostgreSQL's `updated_at` for ClickHouse versioning. Implemented it (PR #23999), but it caused a new P2 incident because the root cause wasn't timestamps — it was that async work read stale data from closures. Fix was reverted (PR #24095).

3. **Pushed back with evidence, not opinion** — Instead of just saying "the timestamp approach is wrong," built custom load testing tools (`test_async_order`) and verification tools (`verify_sync`) to demonstrate:
   - The real root cause: async work captured stale closure data instead of re-reading from DB
   - A second, independent bug discovered during testing: GORM `Save()` overwrites unrelated columns (CONVI-6076)
   - Quantified failure rates at different timing thresholds (10ms → 80% pass, 100ms → 100% pass)

4. **Designed and shipped a multi-layered fix** — Three targeted fixes, each addressing a distinct root cause:
   - **Fix 2 (PR #24103):** Atomic transactions — move historic writes inside transaction, async work re-reads fresh data from DB after commit
   - **Fix 3:** GORM `Omit` — prevent `UpdateScorecard` from overwriting `submitted_at`/`submitter_user_id`
   - **Feature flag** (`COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE`) for safe staged rollout

5. **Verified in production at scale** — Production verification on Spirit across 9,155 submitted scorecards over 39 days: **0 score mismatches, 0 submitter mismatches** for all scorecards present in both PG and CH.

### Generalizable pattern: PG→ClickHouse sync

The core solution — **atomic transaction + async re-read from DB + feature-flagged rollout** — is a reusable pattern for any resource that needs cross-database consistency between PostgreSQL and ClickHouse:

| Pattern element | What it solves | Reusable for |
|----------------|---------------|-------------|
| Move writes inside transaction | Prevents async work from reading uncommitted data | Any PG→CH sync with async updates |
| Async work re-reads from DB | Eliminates stale closure data | Any async pipeline that reads state |
| GORM `Omit` for partial updates | Prevents unrelated column overwrites in concurrent APIs | Any GORM model with concurrent writers |
| Feature flag + staged rollout | Safe production rollout with instant rollback | Any behavioral change to data sync |

### Staff artifacts produced

| Artifact | Location |
|----------|----------|
| Problem investigation + root cause analysis | `investigation.md` |
| Architecture doc shared for design review | [Notion: Scorecard Async Database Updates](https://www.notion.so/cresta/Scorecard-Async-Database-Updates-2974a587b06180f595c3c14492a96104) |
| Load testing tool (quantified failure rates) | `tools/test_async_order/main.go` |
| Verification tool (PG vs CH comparison) | `tools/verify_sync/main.go` |
| Production verification results (9,155 scorecards) | `README.md` → Production Verification |
| Blog writeup (staff perspective) | [`blog/2026-03-13-debugging-dual-database-sync.md`](../blog/2026-03-13-debugging-dual-database-sync.md) |

### Key insights (from blog writeup)

- **Investigation beats intuition** — first fix failed because it was based on "the timestamp is wrong" rather than tracing exact data flow ("closure captures stale state"). Precise read/write sequence tracing led to the correct fix.
- **Multiple bugs produce identical symptoms** — async ordering bug and PG lost-update bug both caused "missing submission data." Stopping after one fix would have left intermittent failures. High-concurrency load testing separated the two.
- **`time.Now()` as version column is a design smell** — write order determines truth, not data order. A late-writing goroutine with stale data "wins."
- **Verify assumptions against actual code** — "92% from automated scoring" was wrong because the enum value meant something different, and the automated code path doesn't even use the affected APIs.
- **Quantify the acceptable residual** — after fixes, 0.87% of records had stale analytics data from rapid UI interactions. Documented as bounded and acceptable rather than leaving "sometimes wrong" as the state.

### Mapping to Staff gaps (from `senior-to-staff.md`)

| Gap | How this project addresses it |
|-----|-------------------------------|
| Scope & ownership | Owned end-to-end across multiple systems (PG, ClickHouse, coaching APIs) and multiple customer escalations (Spirit, SnapFinance) |
| Problem framing & strategy | Wrote investigation doc, called for design review, reframed from "data mismatch" to "async work reads stale data" — a systemic pattern, not a one-off bug |
| Architecture & correctness | Multi-layered fix targeting distinct root causes; pattern generalizable to other PG→CH sync resources |
| Execution via leverage | Custom tools (load tester, verifier) reusable for future sync validation; atomic-transaction pattern applicable beyond scorecards |
| Influence without authority | Presented investigation to CTO, accepted the proposed approach, then pushed back with quantified evidence when it caused a P2 — influence through proof, not hierarchy |
| Operational excellence | Feature-flagged rollout, load testing with timing analysis, production verification across 9,155 scorecards over 39 days |

---

## Project 4: Scorecard/Template Domain Stewardship

**Working reference project:** [`scorecard-template/README.md`](../scorecard-template/README.md)  
**Domain skeleton:** [`scorecard-template/deliverables/scorecard-template-domain-skeleton.md`](../scorecard-template/deliverables/scorecard-template-domain-skeleton.md)

### Why this is a Staff-level growth project

This project is about moving from “engineer who handles scorecard/template tickets” to “technical partner for the coaching scorecard/template domain.”

The scorecard/template area is difficult not only because of code complexity, but because core business rules are scattered across tickets, people, UI behavior, proto definitions, database models, and historical decisions. Bugs and feature work repeatedly revisit the same concepts under different names and representations.

That makes this a strong Staff track: the leverage is not a single implementation, but a clearer domain model, a reusable reference, and small improvements that reduce ambiguity for the whole team.

### Personal growth framing

The growth goal is:

- become a domain-minded technical partner for coaching, not only an executor of scorecard/template tickets
- understand the business rules and historical tradeoffs deeply enough to translate between product intent and technical implementation
- recognize recurring patterns across scattered bugs and features, then turn them into durable artifacts and small systemic improvements

This is the shift from local execution to domain stewardship.

### Staff-level framing

#### Problem

Scorecard/template work has two layers:

- the local issue: a bug, feature, or migration task
- the repeated domain pattern: the same business rules, lifecycle transitions, and sharp edges that keep reappearing

Today the repeated layer is fragmented. That fragmentation causes:

- repeated investigation for similar work
- inconsistent mental models across engineering, product, and design
- hidden correctness risk when business rules are implicit in code or tribal knowledge
- fewer opportunities to identify small, high-leverage domain improvements

#### North-star outcome

Create a living working reference for scorecard/template in coaching service, and use it to identify incremental improvements that make the domain easier to reason about, safer to change, and easier for others to work in.

### What “moving from senior → staff” looks like on this project

- Build a domain map that others can use without replaying prior investigations.
- Make business rules explicit and attach them to lifecycle stages instead of leaving them scattered in tickets and code.
- Turn repeated ticket pain into reusable artifacts: glossary, rule catalog, concept map, investigation log, and improvement proposals.
- Propose small paradigm shifts that fit team constraints, such as explicit invariants, shared validation points, or clearer lifecycle modeling.
- Improve the quality of cross-functional discussion by translating between product behavior and implementation details.

### Concrete work items (staff-level deliverables)

1. **Create the domain skeleton**
- Define the first-pass core concepts: scorecard, template, criterion, option, score, assignment, version, evaluation context
- Capture the relationship map and main lifecycle
- Mark unknowns explicitly instead of blocking on completeness

2. **Build the working reference**
- Create a durable reference covering glossary, lifecycle, rule buckets, surrounding systems, and known sharp edges
- Keep it live by updating it from real ticket investigations

3. **Capture repeated patterns**
- For each ticket, record the local issue, lifecycle stage, rule discovered, and whether the pain points to a doc gap, model gap, API gap, test gap, or ownership gap

4. **Propose small systemic improvements**
- Use recurring patterns to suggest low-cost improvements such as invariant tests, shared validation, better naming, clearer state transitions, or improved observability

5. **Socialize the model**
- Use the working reference to align engineering, PM, and design on the actual business rules embodied by scorecard/template behavior

### Success metrics (fill in as we go)

- Understanding: I can explain the domain’s main concepts, relationships, and lifecycle clearly without redoing prior investigation
- Reuse: future scorecard/template work starts from the reference instead of from scratch
- Pattern recognition: repeated bugs/features are grouped into recognizable categories
- Leverage: at least one or two small domain-level improvements are adopted because the repeated pattern is now visible
- Influence: product/engineering discussions get more concrete because the rules and tradeoffs are articulated clearly

### Staff artifacts produced

| Artifact | Location |
|----------|----------|
| Working-reference project home | `scorecard-template/README.md` |
| Domain skeleton | `scorecard-template/deliverables/scorecard-template-domain-skeleton.md` |
| Working-reference project brief | `scorecard-template/deliverables/scorecard-template-working-reference-project.md` |
| Canonical system reference | `scorecard-template/deliverables/scorecard-template-system-reference.md` |
| Blog writeup | [`blog/2026-06-15-from-scorecard-apis-to-business-workflows.md`](../blog/2026-06-15-from-scorecard-apis-to-business-workflows.md) |
| Blog draft | `train-for-staff/deliverables/from-scorecard-apis-to-business-workflows.md` |

### Key insight

- The leverage in this domain is not “know more facts.” It is “organize repeated domain knowledge so ticket work produces a better model of the system over time.”

### Mapping to Staff gaps (from `senior-to-staff.md`)

| Gap | How this project addresses it |
|-----|-------------------------------|
| Scope & ownership | Moves from local ticket delivery to owning the health and clarity of a business-rule-heavy problem space |
| Problem framing & strategy | Reframes scattered bugs/features as symptoms of fragmented domain knowledge and proposes a low-cost strategy: living reference + incremental improvements |
| Architecture & correctness | Makes lifecycle, invariants, and business rules explicit so correctness can be reasoned about across code and product behavior |
| Execution via leverage | Produces artifacts that help future ticket work, teammate onboarding, and AI-assisted investigation |
| Influence without authority | Improves cross-functional alignment by giving PM, design, and engineering a shared domain map and rule vocabulary |
| Operational excellence | Surfaces risky sharp edges and recurring failure modes before they show up again as production issues |

---

## Project 5: Schwab Leaderboard Metric Semantics and API Strategy

**Project docs:** [`convi-6968-schwab-leaderboard-launch/`](../convi-6968-schwab-leaderboard-launch/)
**API decision table:** [`convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md`](../convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md)
**Backend plan:** [`convi-6968-schwab-leaderboard-launch/deliverables/BE plan.md`](../convi-6968-schwab-leaderboard-launch/deliverables/BE%20plan.md)

### Why this is a Staff-level project

The visible feature request was a leaderboard update: add submitted-scorecard counts and a template breakdown drawer. The staff-level work was recognizing that the hard part was not UI wiring. The hard part was metric semantics.

Several existing APIs looked close enough to reuse, but they answered different questions:

- agent-scoped filters apply to `agent_user_id`
- manager completed-scorecard metrics are submitter-attributed
- `scorecard_time` is conversation/process interaction time, not submission time
- `scorecardReviewerAudience` exists in proto but is not applied in the traced `RetrieveQAConversations` ClickHouse path
- template grouping exists in detail rows but not in the aggregate APIs that power the leaderboard

The project value was separating current behavior, MVP behavior, and desired future semantics before locking the frontend into an accidental API contract.

### Staff-level framing

#### Problem

Leaderboard metric work crosses product language, frontend UX, backend API shape, ClickHouse data modeling, and access-filter semantics. Without an explicit semantic contract, "scorecards completed by manager" can silently mean created by, submitted by, reviewed by, or associated with agents under that manager.

#### North-star outcome

Leaderboard scorecard metrics should have explicit attribution and time-basis semantics, and the UI should consume normalized data providers so implementation details can change without changing the user-facing contract.

### Concrete staff moves

1. **Built an API decision table**
- Compared `RetrieveQAScoreStats`, `RetrieveQAConversations`, `RetrieveScorecardStats`, and `ListScorecards` by entity grain, filter parity, template support, attribution axis, and time basis.

2. **Split MVP from target architecture**
- Recommended `ListScorecards` as the MVP manager drawer source because it can filter by submitter and template today.
- Recommended a normalized drawer data-provider abstraction so the UI can later switch to `RetrieveQAConversations`.

3. **Defined the backend semantic direction**
- Kept `QAAttribute.users/groups` as agent filters.
- Used `scorecard_reviewer_audience` as submitter filters.
- Proposed `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` for explicit submitter grouping.
- Planned migration of `RetrieveQAConversations` to the canonical analytics user-filter parser.

4. **Protected metric correctness**
- Traced `scorecard_time` to conversation start/process interaction time and separated it from `scorecard_submit_time`.
- Avoided reusing APIs whose default time basis or attribution axis would produce a plausible but incorrect metric.

### Staff artifacts produced

| Artifact | Location |
|----------|----------|
| Project home | `convi-6968-schwab-leaderboard-launch/README.md` |
| API decision table | `convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md` |
| Backend migration plan | `convi-6968-schwab-leaderboard-launch/deliverables/BE plan.md` |
| FE implementation analysis | `convi-6968-schwab-leaderboard-launch/deliverables/fe-engineering-work.md` |
| Blog candidate | `train-for-staff/deliverables/blog-and-resume-candidates.md` |

### Key insight

- Metric features need semantic contracts before endpoint reuse. The cheapest wrong implementation is often the one that compiles against an existing API but changes the subject, time basis, or attribution axis of the metric.

### Mapping to Staff gaps (from `senior-to-staff.md`)

| Gap | How this project addresses it |
|-----|-------------------------------|
| Scope & ownership | Connects product metric language, FE behavior, backend APIs, ClickHouse fields, and future migration path |
| Problem framing & strategy | Reframes a UI request into a semantic API strategy problem |
| Architecture & correctness | Defines explicit agent vs submitter filter axes and protects submit-time semantics |
| Execution via leverage | Uses a provider abstraction so the drawer UI can survive backend migration |
| Influence without authority | Gives PM/FE/BE a decision table that makes tradeoffs inspectable |
| Operational excellence | Avoids plausible-but-wrong metrics by tracing field provenance and API filter behavior |
