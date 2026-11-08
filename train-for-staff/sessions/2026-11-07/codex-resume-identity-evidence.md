# Resume Reconstruction: Identity and Project Evidence (Initial Pass)

**Date:** 2026-11-07

**Primary domain:** `train-for-staff`

**Primary subdomain:** none

**Source repo:** `/Users/xuanyuwang/repos/knowledge`

**Branch/worktree:** `main`, `/Users/xuanyuwang/repos/knowledge`

**Source repo registry:** `workspace/repos.yaml`

**Scope:** Validate the proposed technical-owner identity and begin a selective evidence inventory. No resume wording or final project selection was produced in the initial pass.

## Inputs reviewed

- User's proposed identity and cross-database sync story in this conversation.
- Career operating brief, career-positioning work item, recurring-strength synthesis, seniority calibration, Workstream 6 claims ledger, FY27 self-review, legacy resume snippets, and project-story documents.
- Canonical analytics, scorecard-workflow, and scorecard-data-sync references; selected historical scorecard missing-data investigation; existing external-table and Leaderboard records.
- `workflow/ai-operating-model.md`, `workflow/domain-centered-knowledge-model.md`, `train-for-staff/project.yaml`, and `train-for-staff/README.md`.

## Initial identity assessment

**Interpretation:** The user's “Technical Owner” hypothesis is substantially supported, with an important scope calibration. The best-supported positioning is a senior backend/product-infrastructure engineer who takes technical ownership of ambiguous product and data-correctness problems within a bounded domain: clarifies the actual behavior that matters, investigates across service and storage boundaries, makes evidence-backed tradeoffs, and carries work through rollout, production verification, and operational follow-up.

This is distinct from task execution because several records show reframing the reported symptom into a more general system problem, testing competing explanations, producing decision artifacts or reusable mechanisms, and verifying the production outcome. The evidence most strongly supports hands-on ownership and technical judgment. It supports influence through evidence in specific decisions, but does not yet establish organization-wide technical leadership, routine PM/CTO alignment, or measurable team-wide adoption of the artifacts.

The existing career calibration is consistent: established Senior Engineer with repeated Staff-like ownership in bounded technical domains. The user's “Technical Owner” wording describes a contribution mode; it should not by itself imply formal authority or broad cross-team program ownership.

## Evidence-backed strengths

1. **Problem reframing and product awareness — high confidence.** Large ClickHouse user filters were reframed as a missing reference-data transport capability. Analytics discrepancies are repeatedly decomposed into subject, eligibility, time, revision, and aggregation semantics. Sources: `analytics/work-items/large-user-filter-external-tables.md`; `analytics/sessions/2026-10-25/large-user-id-clickhouse-legacy-evidence/design-review.md`; `train-for-staff/deliverables/workstream-2-recurring-strengths.md`.
2. **Cross-system investigation — high confidence.** The scorecard consistency work spans API mutation paths, PostgreSQL transactions/ORM behavior, asynchronous work, and ClickHouse projection. The scorecard domain also records other distinct failure classes, including historic-schema omissions and regional ClickHouse delivery backlogs. Sources: `scorecard-data-sync/sessions/2026-10-25/convi-5565-scorecard-ch-pg-sync-legacy-evidence/`; `historic-scorecard-missing/investigation.md`; `scorecard-data-sync/README.md`.
3. **Technical judgment — high confidence for technical tradeoffs.** External-table design compared ten alternatives and chose a general path; scorecard monitoring/repair uses bounded candidate thresholds, allowlists, chunking, and targeted repair; Leaderboard work separated MVP data access from a more explicit target contract. Sources: external-table design review; `scorecard-workflows/sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/`; `analytics/sessions/2026-10-25/convi-6968-schwab-leaderboard-launch-legacy-evidence/deliverables/api-decision-table.md`.
4. **Production ownership and validation — high confidence.** Examples include feature-flagged rollout and global enablement for external tables, production comparison for scorecard consistency, and fleet monitoring/guarded repair with staging validation. Long-term incident-rate reduction is not established across all examples.
5. **Reusable contracts and durable reasoning — high confidence for creation, medium for leverage.** Behavioral standards, API decision tables, failure taxonomies, verification tools, and repair playbooks are documented. Adoption by other engineers and measurable time saved remain gaps.

## Candidate project inventory (initial, unranked)

### Scorecard PG-to-ClickHouse consistency and race investigation

- **Problem/context:** Customer-facing analytics could show stale or incomplete scorecard state across PostgreSQL and ClickHouse. Multiple causes and attempted fixes appear in the record.
- **Personal contribution documented:** Investigated API/write paths; created concurrency and cross-store verification tools; tested a first timestamp-based approach; drove a corrected direction with atomic writes, DB re-reads, guarded partial updates, and feature-flagged rollout. Current canonical material corrects earlier root-cause explanations: do not present the stale-closure account as the settled explanation for every incident.
- **Technical challenge/judgment:** Similar symptoms came from different write and projection conditions; PostgreSQL source correctness, ClickHouse projection freshness, and ClickHouse cleanup/indexing time are separate concerns.
- **Outcome:** 0 score or submitter mismatches among 2,996 comparable production records over 39 days; a separately documented 0.87% rapid Save-to-Submit residual. This is not proof of perfect consistency.
- **Collaboration/unknowns:** Existing legacy portfolio says findings went to CTO/design review, but current canonical evidence should be checked before claiming the exact alignment process. PM role and customer-facing business outcome are not well evidenced.
- **Identity signal:** Complex investigation, evidence-led direction, end-to-end verification, reliability.
- **Evidence:** `train-for-staff/deliverables/phone-screen-scorecard-pg-clickhouse.md`; `scorecard-data-sync/sessions/2026-10-25/convi-5565-scorecard-ch-pg-sync-legacy-evidence/README.md`; `scorecard-data-sync/deliverables/ui-pg-ch-ordering-options.md`.

### Fleet-wide scorecard sync monitoring and safety-gated repair

- **Problem/context:** Customer-by-customer repair and broad time-window backfills could not establish fleet health or reliably target all missing/stale records.
- **Personal contribution documented:** Ran monitoring across seven production clusters; expanded detection and classification; designed targeted ID-based repair and safety controls (off-by-default, allowlists, thresholds, filtered-run guard, workflow chunking); validated dispatch, skip, and manual override behavior in staging.
- **Technical challenge/judgment:** Distinguishing missing, stale, valid lag, and intentionally different projections while preventing an investigation filter or large candidate set from causing an unsafe bulk repair.
- **Outcome:** Operational detection/repair mechanism and staging evidence; no established measured reduction in customer incidents or support burden yet. Historical investigation records show one batch failure's rate falling from ~51% to ~0.1% after a Jan. 17 fix, but that metric is tied to a specific Spirit batch window and must not be generalized to the fleet system.
- **Collaboration/unknowns:** Ownership of implementation versus operational collaborators, production adoption of auto-heal, and approval/alignment process need confirmation.
- **Identity signal:** Reliability ownership, operational judgment, long-term system thinking.
- **Evidence:** `train-for-staff/deliverables/fy27-mid-year-self-review-draft.md`; `scorecard-workflows/sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/auto-heal-design.md`; `.../implementation-and-validation-summary.md`; `historic-scorecard-missing/investigation.md`.

### ClickHouse external tables for large analytics filters

- **Problem/context:** Large user-ID filters exceeded ClickHouse query-size limits; the deeper issue was lack of a general mechanism for reference data in analytics queries.
- **Personal contribution documented:** Compared ten options, proposed generic external-table helpers rather than a user-ID-only patch, chose a consistent path, and carried feature-flagged rollout through a discovered Live Assist regression to global enablement.
- **Technical challenge/judgment:** Tradeoffs across correctness, query performance, implementation and operational complexity, small-list overhead, and future reuse.
- **Outcome:** Pattern covers 17 caller files; at 10,000 users benchmarked 3.3x faster than a large `IN` clause. This is a benchmark and implementation scope, not proof of production latency improvement or adoption by 17 teams.
- **Collaboration/unknowns:** Decision reviewers and product/customer participation not identified in the reviewed evidence; actual follow-on reuse and time saved are unknown.
- **Identity signal:** Problem discovery, architecture, measured tradeoffs, product-minded scalability, rollout ownership.
- **Evidence:** `analytics/sessions/2026-10-25/large-user-id-clickhouse-legacy-evidence/design-review.md`; `analytics/work-items/large-user-filter-external-tables.md`; Workstream 6 claims ledger.

### User-filter semantic standardization

- **Problem/context:** Several APIs implemented similarly named user filters with divergent ACL, role, group-expansion, active-state, and union/intersection behavior.
- **Personal contribution documented:** Compared behavior, authored an implementation-independent standard, documented divergences, added 62 behavioral tests, and fixed a production union/intersection defect.
- **Technical challenge/judgment:** Preserve caller-specific behavior while making hidden semantics explicit enough to test and migrate safely.
- **Outcome:** A standard/test foundation and a concrete production bug fix. Full migration was not complete; do not repeat the older 29/29 claim.
- **Collaboration/unknowns:** Who reviewed/adopted the standard and whether it changed other engineers' implementation choices are not established.
- **Identity signal:** Product semantics, ambiguity reduction, cross-API systems thinking.
- **Evidence:** `user-filter-consolidation/README.md`; `user-filter-consolidation/user-filter-behavioral-standard.md`; Workstream 6 claims ledger.

### Schwab Leaderboard metric and API strategy

- **Problem/context:** Submitted-scorecard counts and template drill-downs looked like a UI addition, while candidate APIs differed in subject, grain, filters, template support, and time basis.
- **Personal contribution documented:** Compared four API paths, separated agent from submitter and event-time semantics, proposed an MVP provider boundary and a target backend contract, and revised direction as backend capabilities evolved.
- **Technical challenge/judgment:** Avoid a plausible but semantically incorrect metric and balance delivery path against target architecture.
- **Outcome:** Decision-ready analysis; later backend/proto work added explicit submitter semantics. Adoption and business outcome need confirmation.
- **Collaboration/unknowns:** Which PM/engineering stakeholders approved the MVP, what shipped for the customer, and resulting usage/outcome are not clear from the reviewed notes.
- **Identity signal:** Product thinking, cross-layer technical direction, API tradeoffs.
- **Evidence:** `analytics/work-items/CONVI-6968.md`; `analytics/sessions/2026-10-25/convi-6968-schwab-leaderboard-launch-legacy-evidence/deliverables/api-decision-table.md`.

### Group Calibration and scorecard product workflows (candidate needs narrowing)

- **Problem/context:** Group Calibration spans answer keys, participant responses, permissions, completion semantics, reporting, and CSV export; source docs show recurring parity and metric-definition issues.
- **Personal contribution documented:** Career records describe backend architecture/delivery and domain modeling; canonical domain material captures ownership boundaries and identifies UI/API/data semantics.
- **Technical challenge/judgment:** Distinguish answer-key from participant response ownership and define eligible responses/criteria; diagnose export gaps at the actual presentation layer before adding redundant backend fields.
- **Outcome:** A substantial shipped product workflow is reported in the 2025 annual evidence, but adoption and quantified customer/product outcomes were not recovered in this initial pass.
- **Collaboration/unknowns:** Personal architecture and implementation share, PM/design partnership, launch responsibilities, and usage are unknown.
- **Identity signal:** Product ownership across workflow, API, data, permissions, and reporting; potentially adds breadth beyond analytics reliability.
- **Evidence:** `scorecard-workflows/subdomains/group-calibration/README.md`; `2025-annual-review/`; `train-for-staff/deliverables/workstream-6-resume-and-interview-projection.md`.

### Historical historic-schema scorecard omission (keep distinct pending confirmation)

- **Problem/context:** Submitted scorecards existed in Director/PostgreSQL but were absent from `historic.scorecard_scores` and therefore ClickHouse; sampled customers had 29–52% missing in January, with daily samples exceeding 90% after Jan. 21.
- **Personal contribution documented:** The investigation traces a conditional asynchronous historic-write omission and identifies synchronous transactional historic writes as the prevention direction; the log captures an initial race hypothesis and monitoring intent.
- **Technical challenge/judgment:** Submission could skip a historic write based on an assumption that create-time asynchronous work had already written it; transactional writes prevent orphaned source records. The broader investigation also distinguishes this from separate batch failures.
- **Outcome:** The artifact reports new scorecards at 0% missing for verified customers and calls the issue resolved, but this initial pass did not validate exact rollout attribution or connect the user's 90%-to-1% story to this incident. It does not itself establish proactive monitoring before customer reports.
- **Collaboration/unknowns:** Who found the customer pattern, led the code change, aligned the design, and delivered monitoring/alerts; exact pre/post denominators and period; whether this is the user's reference example.
- **Identity signal:** Potentially the clearest customer-problem-discovery to correctness fix story, if personal contribution and relation to the user's reference are confirmed.
- **Evidence:** `historic-scorecard-missing/investigation.md`; `historic-scorecard-missing/log/2025-11-25.md`; `scorecard-data-sync/deliverables/failure-modes-and-case-index.md`.

## Preliminary synthesis and boundaries

- “Technical Owner” is a good working role label if it means hands-on responsibility for technical outcomes within a product/initiative, not unilateral decision authority.
- The most representative identity is not simply “data correctness.” External tables and Leaderboard work show product infrastructure and metric semantics; Group Calibration may show workflow/product breadth; fleet monitoring adds operational reliability.
- The strongest evidence for full lifecycle ownership clusters in external tables and scorecard data reliability. The strongest evidence for explicit alternatives/tradeoffs is external tables and scorecard repair controls.
- Product/customer impact is often technically visible but rarely tied to revenue, adoption, retention, or support burden. Stakeholder and PM/CTO alignment is less documented than investigation and implementation.
- Do not consolidate all scorecard-sync cases into one project: historic-schema omission, asynchronous PG/CH consistency, and regional ClickHouse delivery backlogs have distinct causes and remedies.

## Next step

The user clarified that the 90%-to-approximately-1% story was only an example and its details are not accurate; do not map it to a historical project or use those metrics. A Markdown resume draft was subsequently created from existing evidence at `train-for-staff/deliverables/resume-draft.md`. The user also clarified that Group Calibration was a large feature and that they delivered many other end-to-end workflows not captured in this repository. Do not treat Group Calibration as a minor bug or assume the current repo inventory is complete. Next, inventory those missing workflows with the user and compare them against the current project examples; select representative work rather than listing every feature.
