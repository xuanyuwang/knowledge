# Workstream 6: Resume and Interview Projection

**Status:** First targeted projection
**Date:** 2026-09-03
**Target:** Senior Product Infrastructure / data-intensive backend roles

## Purpose

Project the working career hypothesis into a selective resume and a balanced interview portfolio without inflating level, scope, adoption, or business impact.

The tailored resume is available as [`resume-product-infrastructure.typ`](resume-product-infrastructure.typ) and a visually verified one-page [`resume-product-infrastructure.pdf`](resume-product-infrastructure.pdf). The existing `resume.typ` remains the canonical general resume and was not overwritten.

## Positioning used

> Senior backend and product-infrastructure engineer who owns ambiguous data-correctness and business-semantic problems across system boundaries, turns them into explicit reusable contracts, and carries solutions through safe production validation.

The intended thirty-second memory is:

> He makes complex product and data systems trustworthy, and he builds the mechanisms that keep teams from solving the same correctness problem repeatedly.

## Resume strategy

### What leads

1. Reusable product infrastructure: ClickHouse external tables.
2. Deep cross-system correctness: scorecard PostgreSQL-to-ClickHouse consistency.
3. Reliability as a system: fleet monitoring and bounded repair.
4. Explicit reusable semantics: user-filter behavioral standard and tests.
5. Product breadth: Group Calibration backend ownership.

This ordering makes Candidate 2 from Workstream 4 legible while using Candidate 1's strongest proof.

### What changed from the general resume

- Replaced the technology-and-tenure-heavy summary with a value-and-problem statement.
- Removed the unsupported phrase “improving system reliability across teams.”
- Bounded the user-filter claim to the verified standard, behavioral tests, and production semantics fix instead of claiming the incomplete unification as a full migration.
- Used the documented 17 caller files and 3.3× result at 10,000 users for external tables rather than mixing counts and benchmark sizes from different artifacts.
- Added the fleet-wide scorecard monitoring and safety-gated repair system as evidence of preventive infrastructure and operational design.
- Removed the vague “multiple high-impact backend features” bullet, which named activity without enough outcome evidence.
- Preserved earlier-company bullets as inherited claims from the existing resume; they were not independently revalidated in this pass.

## Claims ledger for the tailored Cresta section

| Claim | Evidence | Calibration |
|---|---|---|
| Ten alternatives evaluated for external tables | `large-user-id-clickhouse/solutions-comparison.md`, `design-review.md` | Strong |
| Generic path adopted across 17 caller files | `train-for-staff/staff-project.md`, FY27 self-review | Strong; describes implementation scope, not adoption by 17 teams |
| 3.3× faster at 10,000 users | `large-user-id-clickhouse/design-review.md` | Strong; benchmark, not production latency |
| Zero score and submitter mismatches across 2,996 comparable records | `convi-5565-scorecard-ch-pg-sync/README.md` | Strong; excludes records absent from one store and does not claim zero submission-time residual |
| Seven-cluster scorecard monitoring | FY27 self-review and linked monitoring evidence | Strong for monitor execution |
| Safety-gated targeted repair | Auto-heal design and staging validation linked from FY27 self-review | Strong for implemented controls; do not claim unrestricted production auto-heal |
| User-filter standard and 62 behavioral tests | FY27 self-review and user-filter artifacts | Strong |
| Production union/intersection defect fixed | `user-filter-consolidation/README.md`, CONVI-6284 | Strong |
| Group Calibration backend architecture and delivery | 2025 annual-review evidence | Moderate–strong; recover product adoption or outcome metrics for a stronger external claim |

## Interview portfolio

The portfolio deliberately spans deep diagnosis, infrastructure design, operational systems, semantic standardization, product judgment, and feature ownership.

### Story 1: Scorecard cross-store consistency

**Readiness:** Interview-ready

**Primary signal:** Deep diagnosis, technical judgment, evidence-led influence, production validation

**Canonical narrative:** [`phone-screen-scorecard-pg-clickhouse.md`](phone-screen-scorecard-pg-clickhouse.md)

**Hook**

Two stores disagreed because asynchronous writes could preserve stale state even though each local code path looked reasonable. The first timestamp-based fix caused a P2 and had to be reverted.

**Personal contribution**

- Reconstructed the write ordering across APIs, transactions, GORM, closures, and ClickHouse replacement semantics.
- Built load and cross-store verification tools.
- Proved stale closure state and a separate lost-update race.
- Designed atomic writes, post-commit re-reads, guarded partial updates, and feature-flagged rollout.

**Outcome**

Zero score or submitter mismatches across 2,996 comparable production records over 39 days, with the remaining submission-time edge documented separately.

**Best questions**

- Tell me about the hardest production problem you diagnosed.
- Tell me about a failed approach or disagreement.
- How do you validate distributed-system correctness?

**Boundary**

Do not claim perfect cross-store consistency or use all 9,155 records as the comparison denominator.

### Story 2: ClickHouse external tables

**Readiness:** Strong brief; prepare a full spoken narrative

**Primary signal:** Problem reframing, options analysis, reusable infrastructure, rollout discipline

**Hook**

A user-filter query exceeded ClickHouse's query-size limit, but the durable problem was the absence of a general mechanism for passing application reference data into analytics queries.

**Personal contribution**

- Compared ten approaches across correctness, complexity, performance, and operational cost.
- Selected external tables and designed generic helpers rather than a user-ID-specific patch.
- Chose one consistent path instead of threshold branching.
- Planned feature flags, comparison testing, staged rollout, and backout.
- Responded to rollout-discovered edge cases, including `ShouldQueryAllUsers` and a nil-argument regression.

**Outcome**

The path covered 17 caller files and benchmarked 3.3× faster than a large `IN` clause at 10,000 users, followed by global enablement.

**Best questions**

- Tell me about an architectural tradeoff.
- Tell me about something reusable you built.
- How do you roll out a risky infrastructure change?

**Boundary**

Describe benchmarks as benchmarks. Do not infer organization-wide adoption or engineering time saved.

### Story 3: Scorecard monitoring and bounded repair

**Readiness:** Strong evidence, but the narrative needs a tighter single arc

**Primary signal:** Reliability strategy, operational scale, safety mechanisms, long-term ownership

**Hook**

Customer-by-customer repair could not establish whether scorecard projection was healthy across the fleet, and broad time-range backfills were both incomplete and operationally risky.

**Personal contribution**

- Ran monitoring across seven production clusters and exposed missing-scorecard scope.
- Evolved count-only detection toward missing/stale/synchronized classification.
- Designed targeted ID-based repair rather than broad time-range recovery.
- Added allowlists, thresholds, filtered-run guards, chunking, and manual override.
- Converted operational findings into failure taxonomies and repair playbooks.

**Outcome**

Created an observable, safety-gated path from detection to targeted repair, with staging scenarios verifying dispatch, threshold skip, and override behavior.

**Best questions**

- Tell me about improving reliability systematically.
- How do you make dangerous operations safe?
- Tell me about ownership beyond one incident.

**Boundary**

Do not claim unrestricted production auto-heal or a measured incident-rate reduction without additional evidence.

### Story 4: User-filter semantic standardization

**Readiness:** Medium; use for semantics and standards, not completed migration

**Primary signal:** Making implicit contracts explicit, finding silent divergence, creating migration guardrails

**Hook**

Three user-filter implementations answered the same-looking request differently across ACL, selection, group expansion, roles, and active-state behavior.

**Personal contribution**

- Compared implementations and caller behavior rather than assuming one was canonical.
- Wrote an implementation-independent behavioral standard.
- Identified silent semantic divergences, including user-plus-group intersection versus union.
- Added 62 behavioral tests and fixed the production union/intersection defect.
- Exposed future source-sensitive active-state requirements rather than forcing them into one coarse flag.

**Outcome**

Created a verified contract and test foundation for future consolidation while fixing a concrete production semantic bug.

**Best questions**

- Tell me about standardizing inconsistent systems.
- How do you migrate behavior safely?
- Tell me about a bug caused by ambiguous requirements.

**Boundary**

The canonical project record says the full unification was not complete. Do not use the older `12/29 → 29/29` completion claim without newer canonical evidence.

### Story 5: Leaderboard metric semantics and API strategy

**Readiness:** Medium; recover shipped outcome and stakeholder decision

**Primary signal:** Product judgment, cross-layer systems thinking, MVP versus target architecture

**Hook**

A leaderboard drawer looked like a frontend feature, but existing APIs disagreed on who was counted, when the event occurred, which templates were grouped, and whether the data was aggregate or detail grain.

**Personal contribution**

- Compared four APIs by entity grain, filter parity, template support, attribution axis, and time basis.
- Distinguished agent, submitter, and reviewer semantics.
- Separated an MVP provider path from the desired explicit backend contract.
- Updated the recommendation when backend capabilities changed.

**Outcome**

Produced a decision-ready semantic and API path that avoided a plausible-but-wrong metric contract.

**Best questions**

- Tell me about working through ambiguous product requirements.
- Tell me about balancing MVP delivery and architecture.
- How do you work across frontend and backend boundaries?

**Boundary**

Do not claim cross-team adoption or business outcome until the shipped path and stakeholder feedback are recovered.

### Story 6: Group Calibration backend ownership

**Readiness:** Medium; older story needs refreshed metrics and collaborator boundaries

**Primary signal:** Greenfield feature ownership, product workflow modeling, breadth beyond correctness investigations

**Hook**

Group Calibration required a new end-to-end QM workflow for creating exercises, assigning evaluators, collecting responses, measuring consistency, reporting results, and sending notifications.

**Personal contribution**

- Led backend implementation across data models, task APIs, response flows, scoring, analytics, and notifications.
- Worked across the lifecycle rather than delivering a single endpoint.
- Continued later domain work around reporting and export semantics.

**Outcome**

Enabled the backend workflow used by QA teams to run calibration exercises and compare evaluator consistency.

**Best questions**

- Tell me about owning a large feature.
- How do you model a new product workflow?
- Tell me about coordinating across multiple backend capabilities.

**Boundary**

Recover launch scope, usage, customer outcome, collaborators, and exact personal decision ownership before using this as a primary Staff-scope story.

## Story selection by interview signal

| Interview signal | Primary story | Complement |
|---|---|---|
| Technical depth | Scorecard consistency | External tables |
| Architecture and tradeoffs | External tables | Leaderboard semantics |
| Reliability and operations | Monitoring and repair | Scorecard consistency |
| Ambiguity and product judgment | Leaderboard semantics | User filters |
| Reusable leverage | External tables | User filters |
| End-to-end feature ownership | Group Calibration | Scorecard consistency |
| Failure or disagreement | Scorecard consistency | External-table rollout edge cases |
| Cross-functional collaboration | Leaderboard or Group Calibration | Evidence recovery required |
| Mentoring or delegation | None ready | Evidence recovery required |

## Missing stories and evidence

The portfolio is not yet balanced for every Senior or Staff interview. Highest-value recovery:

1. A specific mentoring, delegation, or execution-through-others story.
2. A named cross-functional disagreement or alignment outcome.
3. Measured adoption or engineering time saved by a reusable mechanism.
4. A customer or business metric changed by the technical work.
5. A strong shipped story outside analytics and scorecard correctness.
6. Refreshed Group Calibration launch, adoption, and collaborator evidence.

## Tailoring rules

### Product-infrastructure role

- Lead with external tables, scorecard consistency, and monitor/repair.
- Use user filters to show standards and migration thinking.
- Keep product-semantic stories as evidence of application context.

### Reliability-heavy backend role

- Lead with scorecard consistency and monitor/repair.
- Describe external tables as preventive infrastructure and safe rollout.
- Avoid suggesting SRE or availability depth not present in the portfolio.

### Product-backend role

- Lead with Group Calibration or Leaderboard, followed by scorecard consistency.
- Emphasize cross-layer semantics and end-to-end workflow ownership.
- Recover stakeholder and shipped-outcome evidence before making influence central.

## Next actions

1. Rehearse Stories 1 and 2 as two-minute phone-screen answers and ten-minute deep dives.
2. Turn Story 3 into one bounded narrative instead of a list of monitoring, repair, and cleanup activities.
3. Recover the missing evidence for Stories 5 and 6.
4. Build a genuine mentoring or delegation story before claiming broader Staff readiness.
5. Tailor only after applying the Workstream 5 opportunity scorecard to a real role.

## Sources

- [Workstream 4 career hypotheses](workstream-4-career-hypotheses.md)
- [Workstream 5 next-role fit](workstream-5-next-role-fit.md)
- [Workstream 3 seniority calibration](workstream-3-seniority-calibration.md)
- [Resume snippets](../resume-snippets.md)
- [Staff project portfolio](../staff-project.md)
- Canonical project evidence linked in the claims ledger and story briefs

## Maintenance rule

Update the projection for a real opportunity, not for generic polish. Any new resume claim must have a source, a bounded denominator where relevant, and clear personal contribution.
