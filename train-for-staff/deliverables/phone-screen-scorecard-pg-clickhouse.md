# Phone Interview Project Story: Scorecard PostgreSQL → ClickHouse Consistency

## What the interviewer should remember

I took an intermittent, multi-customer analytics correctness problem that crossed API transactions, asynchronous workers, PostgreSQL, an ORM, and ClickHouse; proved that two independent races produced the same symptom; and drove a layered fix, safe rollout, and production verification.

This is stronger than “I fixed a race condition.” The senior/staff signal is the combination of end-to-end problem ownership, precise distributed-systems reasoning, evidence-based influence, reusable tooling, rollout discipline, and an honest boundary around the remaining limitation.

## 30-second version

At Cresta, PostgreSQL was the source of truth for coaching scorecards, while ClickHouse powered customer-facing analytics. Multiple customers reported that submitted scorecards sometimes appeared stale or unsubmitted in analytics. I traced the problem across two API paths and found two independent races: asynchronous ClickHouse writers could finish out of order and let stale data win, and GORM full-struct saves could overwrite submission fields in PostgreSQL. I built load and cross-database verification tools, designed a layered fix using atomic transactions, post-commit re-reads, and partial updates, and rolled it out behind a feature flag. Over a 39-day production window, the comparable set of 2,996 scorecards had zero score or submitter mismatches.

## Two-minute version

### Situation and value

Cresta stores authoritative coaching scorecards in PostgreSQL and projects them into ClickHouse for Performance Insights. Spirit and SnapFinance reported analytics inconsistencies: the application could show the correct submitted scorecard while the analytical copy had stale scores or looked unsubmitted. That damages trust in the product because managers use those analytics to evaluate coaching and quality performance.

### Why it was technically difficult

The system did not perform a simple synchronous dual write. `UpdateScorecard` and `SubmitScorecard` committed to PostgreSQL and then launched asynchronous ClickHouse work. ClickHouse used a `ReplacingMergeTree` whose winner was selected by a write-time version. An update worker could read before submission committed, finish after the submit worker, and therefore make an older business state look newer.

The observable symptom also had a second cause. During stress testing, I found that both APIs used GORM `Save()`, which persisted an entire previously read struct. A late update save could restore stale `submitted_at` and `submitter_user_id` values in PostgreSQL. Two races at different storage layers produced nearly identical symptoms.

### What I did

I wrote the investigation and brought it to design review. We first tried source timestamps for ClickHouse versioning, but the change addressed ordering rather than the stale captured state and caused a P2 regression, so we reverted it. I then built a configurable load tester and a PG-versus-ClickHouse verifier to replace intuition with reproducible evidence.

The final change had three layers:

1. Keep related PostgreSQL and historic-schema writes in one transaction.
2. Start asynchronous projection only after commit and re-read committed state instead of using closure-captured objects.
3. Make update APIs omit submission-owned columns, preventing ORM lost updates.

I put the new path behind a feature flag, validated it under concurrency, rolled it out gradually, and checked production data before deleting the legacy path.

### Result

For Spirit, I inspected 9,155 PostgreSQL submitted scorecards over 39 days. Of those, 2,996 existed in both stores and were valid for evaluating this fix; they had zero score mismatches and zero submitter mismatches. A narrow residual remained: 26 of the 2,996, or 0.87%, had stale ClickHouse submission state when a user saved and submitted within roughly 100 ms. PostgreSQL remained correct. We documented that limitation rather than presenting eventual consistency as perfect, and after global validation removed the flag and 740 lines of legacy code.

The other 6,159 PostgreSQL scorecards absent from ClickHouse belonged to a separate conversation-ingestion coverage gap. I explicitly separated that issue from this fix instead of using it to overstate or understate the result.

## Technical deep dive

### Failure sequence

```text
UpdateScorecard                         SubmitScorecard
PG commit
async worker reads unsubmitted state
                                        PG commit with submission state
                                        async worker reads submitted state
                                        writes correct row to ClickHouse
late update worker writes stale row
ClickHouse keeps the later write-time version
```

The important distinction is between **source-state order** and **delivery order**. ClickHouse was choosing by delivery time, so a late writer carrying an early snapshot could win.

### Why the first fix failed

Changing the ClickHouse version from `time.Now()` to PostgreSQL `updated_at` sounded reasonable, but the worker still carried stale closure data. A better version field cannot repair a worker that does not read the intended source state. The lesson was to trace exactly when each value was read, committed, and projected.

### Why the final fix worked—and its boundary

Atomic writes and partial updates protected PostgreSQL correctness. Post-commit re-reads greatly reduced stale projections because workers used committed source data. They did not create total ordering across workers: an update worker can still read just before a submit commit and write after the submit worker. Fully eliminating that window would require a stronger ordering mechanism, such as a source-monotonic version, serialized per-entity projection, or a durable outbox/change stream. The implemented solution was a scoped reliability improvement within the existing architecture, not a claim of strict consistency.

## How to explain my personal value

Use actions and evidence rather than saying “I am beyond senior”:

- **I owned the problem across boundaries.** I followed the state through product APIs, transactions, ORM behavior, asynchronous execution, ClickHouse merge semantics, and production analytics.
- **I improved the problem definition.** I separated an async projection race, a PostgreSQL lost update, and an unrelated ingestion coverage gap instead of treating every mismatch as one bug.
- **I influenced with evidence.** When an initially reviewed approach failed, I helped revert it and used reproducible timing experiments to establish the actual failure sequence.
- **I made correctness testable.** The load tester quantified the concurrency window; the verifier compared authoritative and projected state field by field.
- **I managed production risk.** The work used a feature flag, staged rollout, explicit production verification, and legacy cleanup only after confidence was established.
- **I left leverage behind.** The investigation method and “transaction + fresh source read + partial update” pattern apply to other asynchronous projections, not only scorecards.

If asked about level, say:

> I think this demonstrates strong senior engineering because I independently resolved a difficult cross-system correctness problem and made the rollout measurable and safe. The staff-like part is that I reframed the issue into reusable invariants and tooling, influenced a leadership-level design decision through evidence, and owned the result beyond the code change. I would not claim one project alone proves a title; I use it as concrete evidence of operating at that scope.

## Likely follow-up questions

### What was the customer or business value?

The issue affected the trustworthiness of Performance Insights. A scorecard could be correct in the transactional product but wrong in the analytics managers used for quality and coaching decisions. The fix protected source-of-truth submission fields, eliminated observed score and submitter mismatches in the comparable production set, and converted an intermittent complaint into a measurable residual with a known trigger.

### What did you personally do versus the team?

I led the technical investigation, traced the data flow, implemented or drove the layered correction, built the load and verification tools, analyzed the timing experiments, and performed the production comparison. The approach was reviewed with engineering leadership, and deployment used the team's feature-flag and rollout mechanisms.

Do not claim sole ownership of decisions or work that the evidence only describes as reviewed or shared.

### Why was this more than a normal bug fix?

The symptom crossed two databases and concealed two independent races. Resolving it required defining which store was authoritative, understanding ClickHouse version-selection semantics, reproducing nondeterministic execution, protecting concurrent PostgreSQL writes, choosing an acceptable consistency boundary, and proving the outcome in production.

### Why accept a 0.87% residual?

The residual affected only ClickHouse submission metadata under a narrow rapid Save → Submit window; PostgreSQL remained correct. Eliminating it completely would have required a more invasive ordering or delivery redesign. We quantified the frequency and impact and treated it as a conscious product/reliability tradeoff. If the business later required strict projection consistency, the next design step would be a source-monotonic version or durable ordered projection.

### What would you do differently?

I would build the cross-store verifier and an exact read/commit/write timeline before proposing a versioning change. I would also define the invariant up front: the ClickHouse winner must represent the latest valid PostgreSQL state, not merely the worker that wrote last.

### How did you know the fix worked?

I used three levels of evidence: deterministic code-path reasoning, configurable concurrency tests, and a 39-day production comparison. The production denominator must be stated precisely: 9,155 PG submitted scorecards were inspected, 2,996 existed in both PG and ClickHouse, and those 2,996 were the comparable population for score and submitter correctness.

## Resume bullet

- Diagnosed multi-customer scorecard inconsistencies across PostgreSQL and ClickHouse, uncovering async stale-write and ORM lost-update races; designed atomic, post-commit re-read and partial-update fixes, built concurrency/cross-store verification tooling, and led a feature-flagged rollout that produced zero score or submitter mismatches across 2,996 comparable production scorecards over 39 days.

## Claims ledger

| Claim | Evidence | Interview wording |
|---|---|---|
| Multiple customers were affected | CONVI-5565 investigation names Spirit and SnapFinance | “Multiple customers reported analytics inconsistencies.” |
| Two independent races | CONVI-5565 async ordering; CONVI-6076 GORM lost update | “I found two causes with the same visible symptom.” |
| Reproducible timing evidence | 10 ms: 80%; 50 ms: 94%; 100 ms: 100% in the recorded experiment | Quote as test results, not a universal system guarantee. |
| Production scope | 9,155 PG submitted; 2,996 present in both stores over 39 days | Always distinguish inspected from comparable populations. |
| Correctness result | 0 score and 0 submitter mismatches in the 2,996 comparable rows | Do not say all 9,155 were consistent across stores. |
| Residual | 26/2,996 (0.87%) stale CH submission state in the follow-up analysis | Say “bounded residual,” not “perfect consistency.” |
| Cleanup | PR #26256 removed the flag and 740 legacy lines | Use as evidence that rollout was completed and simplified. |

## Source artifacts

- [`../../convi-5565-scorecard-ch-pg-sync/README.md`](../../convi-5565-scorecard-ch-pg-sync/README.md)
- [`../../convi-5565-scorecard-ch-pg-sync/investigation.md`](../../convi-5565-scorecard-ch-pg-sync/investigation.md)
- [`../../convi-5565-scorecard-ch-pg-sync/log/2026-03-11.md`](../../convi-5565-scorecard-ch-pg-sync/log/2026-03-11.md)
- [`../../convi-5565-scorecard-ch-pg-sync/log/2026-03-12.md`](../../convi-5565-scorecard-ch-pg-sync/log/2026-03-12.md)
- [`../../scorecard-data-sync/deliverables/architecture-and-invariants.md`](../../scorecard-data-sync/deliverables/architecture-and-invariants.md)
- [`../../blog/2026-03-13-debugging-dual-database-sync.md`](../../blog/2026-03-13-debugging-dual-database-sync.md)
