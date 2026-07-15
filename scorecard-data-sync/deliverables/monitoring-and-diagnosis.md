# Monitoring and Diagnosis

## Monitoring Contract

A useful monitor must answer more than whether aggregate counts match:

1. Which PG scorecard IDs are expected in CH?
2. Which are missing?
3. Which existing CH rows have stale critical fields?
4. Which expected emitted criteria are missing?
5. Which entities are intentionally excluded?
6. Can the monitor finish inside its operational deadline on the largest profiles?

The current scorecard sync monitor primarily detects missing scorecard IDs. It does not prove field-level correctness for rows already present.

## Inventory Semantics

- Submitted inventory is selected by `submitted_at` in the monitoring window.
- Unsubmitted inventory is selected by `created_at` and analytical eligibility.
- Empty unsubmitted scorecard shells without `director.scores` are not expected CH entities.
- Calibration and non-standard scorecard types must follow explicit inclusion rules.

CONVI-7186 moved the expensive score-row existence check after the CH lookup: only missing unsubmitted IDs are checked in `director.scores`, in bounded batches. This preserves the intended denominator/reindex set while avoiding a correlated probe across the whole unsubmitted shell population.

## Diagnosis Ladder

### 1. Identify the entity and expected scope

- Resolve customer/profile, scorecard ID, template/revision, scorecard type, conversation/process identity, and relevant time axis.
- Confirm the scorecard is intended for the analytical projection.

### 2. Check authoritative PG state

- `director.scorecards`: existence, score, state timestamps, identities, type, and update time.
- `director.scores`: expected emitted criteria and values.
- Template revision: distinguish criteria from chapter aggregates.

### 3. Check CH scorecard projection

- Query `scorecard_d` using `FINAL` or explicit latest-version selection.
- Compare critical fields, not only existence.

### 4. Check CH score rows

- Compare emitted criterion IDs and critical values in `score_d FINAL`.
- Exclude expected non-emitted chapter aggregate rows before classifying a count difference.

### 5. Classify before repairing

- missing scorecard;
- stale scorecard fields;
- missing/stale criteria;
- expected exclusion;
- lookup/materialized-view-only issue;
- monitor coverage or performance issue.

### 6. Select the narrowest repair

Use `deliverables/repair-and-backfill-playbook.md`. Do not use a broad time-range backfill for a single stale existing row.

## Monitor Performance Lessons

The 2026-06-30 timeout investigation showed:

- submitted-range lookup can use the `(customer, profile, submitted_at)` index efficiently;
- unsubmitted inventory had no comparably selective created-time path on very large profiles;
- an additional `NOT EXISTS` count scanned the unsubmitted shell population a second time;
- naive score-driven SQL could be reordered back into the same bad scorecard scan;
- a broad partial index covering nearly all unsubmitted rows could be extremely large.

Operational guidance:

- separate submitted and unsubmitted accounting conceptually and in timing logs;
- avoid observability-only counts on the critical path when they duplicate expensive work;
- bound ID lists and score-row checks;
- log phase timings so PG inventory, CH lookup, and repair dispatch are distinguishable;
- validate query plans on the largest customer profiles, not only staging-sized data.

## Known Detection Gaps

- Existing-but-stale CH scorecards can pass ID comparison.
- Aggregate counts can match while individual fields are wrong.
- Criterion count comparison can false-positive on chapter rows.
- Time-range selection can miss entities when the wrong creation/submission/process timestamp is used.
- A monitor that exceeds its deadline is itself an observability failure and cannot safely auto-heal.
