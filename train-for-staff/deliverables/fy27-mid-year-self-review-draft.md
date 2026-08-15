# FY27 Mid-Year Self-Review Draft

**Review period:** February 1 - July 31, 2026

**Audience:** HiBob mid-year self-reflection

**Voice:** Senior engineer demonstrating Staff-level scope, leverage, and judgment

## 1. What were your 3 most meaningful contributions in H1, and what impact did they have on the team or business?

### 1) I created a scalable, reusable path for large analytics filters

I reframed a concrete failure - large user-ID filters exceeding ClickHouse query limits - as a broader infrastructure gap: we had no general mechanism for passing reference data into analytics queries without embedding it in SQL text.

- I evaluated ten solution options and chose ClickHouse external tables based on correctness, operational simplicity, performance, and reuse beyond user IDs.
- I designed the implementation as generic helpers in the filter-building layer, so 17 caller files could adopt the same three-line pattern without changing the shared query layer or duplicating table construction.
- Benchmarks showed nearly flat external-table performance as the population grew; at 10,000 users the new path was 3.3x faster than a large `IN` clause.
- I shipped it behind a feature flag, addressed a Live Assist regression found during rollout, and completed global production enablement.
- In parallel, I wrote an implementation-independent user-filter behavioral standard and 62 behavioral tests. This turned recurring debates about union/intersection, ACL, group expansion, and deactivated-user semantics into explicit contracts that future migrations can validate against.

**Impact:** We removed a scaling ceiling for large customers, improved query performance for large populations, and created a reusable pattern for future reference-data filters. More importantly, the solution addressed the class of problem rather than only the first endpoint that exposed it.

### 2) I built a scorecard data reliability system spanning detection, auto-healing, write correctness, and production repair

I moved scorecard PostgreSQL-to-ClickHouse consistency from reactive, customer-by-customer repair toward a closed-loop reliability model: establish fleet-wide health, distinguish failure modes, trigger the narrowest safe repair, and verify convergence.

- I ran the sync monitor across all seven production clusters and made the results operationally usable through cluster-level summaries and batched Slack reporting. This exposed previously hidden scope, including 1,601 of 57,800 missing scorecards in `us-east-1-prod` and 450 of 17,147 in `voice-prod` during the measured March window.
- I evolved `scorecard-sync-monitor` from a count-only detector into an auto-healing control loop. It inventories PostgreSQL once; distinguishes missing, stale, and synchronized rows across all/submitted/unsubmitted views; classifies conversation and process scorecards; and dispatches targeted ID-based reindex jobs instead of relying on broad time-range backfills that miss edge cases.
- I designed rollout safety into the system: auto-heal is off by default, constrained by customer/profile allowlists and candidate thresholds, guarded against filtered investigation runs, and protected by workflow chunking. Staging validation proved that an under-threshold set dispatched repair, an over-threshold set of 1,122 candidates safely skipped, and an explicit manual override could recover the targeted range.
- I complemented recovery with prevention. For the dual-write race, I traced stale asynchronous state and unsafe full-struct ORM saves as independent causes, then used load tests and verification tooling to redirect a failed timestamp-based approach. The final design used atomic transactions, PostgreSQL re-reads, guarded partial updates, and a feature-flagged rollout. Production verification covered 2,996 records present in both stores with zero score and submitter mismatches; I separately documented the 0.87% rapid Save-to-Submit residual.
- I also led large-scale historical repair, including an eight-cluster appeal cleanup that removed approximately 70.5 million scorecards and 650.5 million scores and produced clean backfills for 95/95 customer targets. I converted the operational lessons into reusable monitoring, failure-taxonomy, diagnosis, and repair playbooks rather than leaving them in incident history.

**Impact:** This work established the foundation for scorecard data to detect and repair drift safely instead of waiting for customers to find it. It shortened the path from symptom to diagnosis, created a controlled alternative to repeated manual backfills, and made scorecard reliability an observable system with explicit invariants, failure classes, rollout controls, and reusable recovery paths.

### 3) I grew from ticket execution toward scorecard and analytics domain stewardship

Across H1, I repeatedly saw customer issues that looked local but were manifestations of missing semantic contracts across Director, APIs, PostgreSQL, ClickHouse, templates, and historical revisions. I began treating the domain itself as an engineering product.

- I built a living scorecard/template reference covering the concept model, template and scorecard lifecycles, business rules, API/data paths, option/N/A semantics, and recurring ticket patterns.
- I used that model to drive cross-layer work such as submitted-scorecard editor permissions: clarifying product scope, separating proactive permission evaluation from authoritative write enforcement, including reset behavior, and aligning frontend and backend rollout semantics.
- I created API decision tables and data-flow evidence for Leaderboard and Performance Insights discrepancies where similarly named metrics actually used different subjects, filters, template rules, or time bases.
- In July, I connected multiple HCD, United, and SCAN reports into a reusable mixed-revision QA semantics pattern instead of treating each as an isolated data-sync incident. I also isolated a Leaderboard count mismatch to frontend voicemail-filter drift and drove the parity fix across Agent and Manager paths.
- I converted investigation output into durable work items, domain references, decision records, and operational guides so the next engineer can begin from the current model rather than replaying production queries and historical tickets.

**Impact:** The immediate impact was faster and more accurate diagnosis of customer-facing issues. The broader impact is leverage: clearer ownership, fewer semantic regressions, more productive product/engineering discussions, and a growing paved road for scorecard and analytics work beyond my own code contributions.

## 2. Select the 3 Operating Principles you feel you have most embodied in H1

- **Jump In**
- **Help our Customers Win**
- **Patient in Strategy, Impatient in Execution**

## 3. How have you demonstrated the Operating Principles you selected above?

### Jump In

I deliberately stepped outside the scorecard API work I already knew and took ownership in unfamiliar domains when the team needed progress. In user filtering, I learned the interactions among ACL scope, group expansion, roles, deactivated users, and analytics query construction, then turned that understanding into a behavioral standard, tests, and a scalable external-table path. For scored N/A, I worked across template configuration, frontend form state, AutoQA option mapping, persistence formats, and backend score formulas to understand the full lifecycle and fix correctness issues rather than treating each symptom in isolation. I also developed deeper ClickHouse expertise—from query transport and performance tradeoffs to `ReplacingMergeTree` row semantics and PostgreSQL-to-ClickHouse synchronization—and applied it to external tables, production diagnosis, and scorecard auto-heal. In each case, I built the necessary domain model, validated it against real behavior, and carried the work through implementation instead of waiting for the problem to return to my familiar area.

### Help our Customers Win

I treated customer-reported discrepancies as trust problems, not just tickets to close. For scorecard analytics, that meant tracing exact records across PostgreSQL and ClickHouse, proving whether a symptom came from storage, aggregation, template history, or frontend request construction, and completing production verification after the code shipped. I then went a step further: the fleet-wide sync monitor surfaced silent gaps before each customer had to report them, and the auto-heal path turned those findings into bounded, targeted repair. The Brinks investigation illustrates the progression: a test backfill recovered 84/84 scorecards, broader execution recovered 401/436, and the remaining edge cases directly informed the ID-based repair design. The eight-cluster appeal cleanup and 95/95 clean backfill similarly went beyond the code change to restore customer-visible data. This aligns with the principle's emphasis on understanding the customer's long-term success and delivering the result they need, not merely delivering software.

### Patient in Strategy, Impatient in Execution

I balance deliberate architecture with urgency once the direction is clear. For external tables, I evaluated alternatives, defined a reusable contract, and designed a staged rollout before changing production behavior; once aligned, I carried the change through all callers, feature-flag rollout, regression response, and global enablement. I applied the same model to scorecard sync: first measure the seven-cluster problem and learn from targeted Brinks repairs, then replace time-range recovery with an ID-based missing/stale detection and repair loop. Once the architecture was clear, I implemented explicit allowlists, thresholds, workflow chunking, observe-only operation, and focused validation so the system could advance without taking unbounded production risk. During production cleanup, I likewise shifted to customer-volume-based windows and parallel weekend execution when full-range workflows timed out. This is the operating model I want to keep strengthening: patient about the system we are building, impatient about removing the next concrete blocker.

## 4. What are 2 areas where you want to grow your impact in H2? How can your manager help you to grow in these areas?

### 1) Turn my investigation experience into reusable team capability

In H1, I developed substantial experience diagnosing scorecard, analytics, filtering, and data-consistency problems, but too much of that judgment still lives with me and is reused mainly through my own execution. In H2, I want to convert the recurring methods into assets other engineers can apply directly: investigation playbooks, decision trees, runnable verification tools, worked examples, and reusable AI-assisted skills where appropriate. I want these resources to capture not only system facts, but also how to choose the right data source, isolate semantic differences, classify failure modes, validate a repair, and avoid repeating known dead ends. Success would mean another engineer can independently diagnose and resolve a similar issue faster and more safely using these assets, without needing me as the starting point or critical path.

**How my manager can help:** Help me identify the investigation patterns with the highest team-wide value and protect time to turn them into maintained, discoverable workflows rather than one-off notes. Create opportunities for other engineers to use these resources on real work, and give me feedback on adoption and effectiveness so I can improve them based on observed use rather than assuming documentation alone creates leverage.

### 2) Connect technical strategy more directly to customer and business outcomes

I am strong at diagnosing correctness and architecture problems, but I want to get better at defining the outcome before implementation: which customer behavior, reliability metric, adoption signal, latency measure, or support burden should change, and how we will know the work succeeded. I also want to communicate the strategy more consistently in concise executive terms so product, customer success, and engineering can align earlier.

**How my manager can help:** Bring me into roadmap and customer-impact discussions earlier, particularly for scorecard, analytics, and data-reliability work. Help me establish access to the right product/operational metrics and create opportunities to present recommendations to broader stakeholders. A regular calibration on Staff-level expectations - especially problem selection, cross-functional influence, and measurable outcome closure - would help me focus on the highest-leverage gaps.

## Internal Evidence and Calibration Notes

- Operating Principles source: [Cresta Operating Principles](https://docs.superhuman.com/d/_dOdd8RvXUvm/_supg5MWW).
- External tables evidence: `large-user-id-clickhouse/design-review.md`, `large-user-id-clickhouse/log/2026-03-11.md`.
- User-filter semantics evidence: `user-filter-consolidation/user-filter-behavioral-standard.md`, `user-filter-consolidation/phase1-review.md`.
- Scorecard consistency evidence: `convi-5565-scorecard-ch-pg-sync/README.md`, `convi-5565-scorecard-ch-pg-sync/investigation.md`.
- Scorecard monitor/auto-heal evidence: `weekly-summary/weekly-summary-2026-04-06-to-12.md`, `auto-backfill-missing-scorecards/auto-heal-design.md`, `auto-backfill-missing-scorecards/implementation-and-validation-summary.md`, `auto-backfill-missing-scorecards/rollout-plan.md`.
- Scorecard reliability operating model: `scorecard-data-sync/deliverables/monitoring-and-diagnosis.md`, `scorecard-data-sync/deliverables/repair-and-backfill-playbook.md`, `scorecard-data-sync/log/2026-07-22.md`.
- Production cleanup/backfill evidence: `weekly-summary/weekly-summary-2026-02-21-to-23.md`, `weekly-summary/weekly-summary-2026-03-02-to-08.md`, `weekly-summary/weekly-summary-2026-03-23-to-28.md`.
- Domain stewardship evidence: `train-for-staff/staff-project.md`, `scorecard-template/`, `analytics/`, `scorecard-workflows/`.

### Claims deliberately kept bounded

- User-filter migration was not described as complete; the durable contribution claimed here is the behavioral standard/test foundation and the separately shipped external-table path.
- Scorecard production consistency claims use the 2,996 records present in both PostgreSQL and ClickHouse, not all 9,155 submitted PostgreSQL scorecards in the reviewed period.
- The 0.87% rapid Save-to-Submit edge is stated as a documented residual, not as full elimination of every projection race.
- Auto-heal is described as an implemented, safety-gated system with staging validation; the draft does not claim unrestricted production rollout or use post-H1 August recovery results as H1 impact.
