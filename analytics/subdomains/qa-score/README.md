# QA Score

## Purpose

Own QA score calculation and display semantics from template options and score rows through analytics aggregation and frontend presentation.

## Semantics and Invariants

- Option identity, option index, raw/numeric value, mapped score, percentage score, and display label are distinct fields.
- N/A or null means no applicable score; it must not be collapsed into numeric zero.
- A zero-weight or excluded criterion can carry a raw value while contributing no percentage score.
- Chapter and criterion aggregates have different denominators and must remain distinguishable.
- Score rows retain percentage and weight values derived from their referenced template revision; a later template revision does not rewrite historical rows.
- PI criterion queries match scorecard ID plus the parent's source update timestamp to avoid mixing scorecard versions. Unequal replication progress can temporarily exclude an affected card; ID-only matching can revive stale scores, including scores overwritten with N/A. See the [source trace and timing example](../../sessions/2026-11-03/codex-scorecard-version-join.md).
- Criterion-level aggregation across revisions must not assume configured contribution weights are comparable observation weights.
- Option-to-score mappings can change across revisions; a Yes/No label shown in UI is not sufficient to infer pass/fail percentage without the pinned revision's mapping.
- Popovers and drill-downs may filter the contributing rows differently from the parent aggregate; their contract must be recorded explicitly.
- Agent and Manager Leaderboard scorecard columns and template drawers use QA APIs and must preserve the same scorecard-membership filters; see the [Leaderboard invariant](../leaderboard/README.md#current-semantics).

## Architecture and Source Map

- **Frontend:** Performance Insights score charts, criteria tables, and popovers
- **APIs:** primarily `RetrieveQAScoreStats`
- **Backend:** analytics-service QA aggregation and shared scoring helpers
- **Storage:** template revisions plus PostgreSQL/ClickHouse scorecard score representations

## Operational Knowledge

- For N/A symptoms, inspect `not_applicable`, raw/numeric value, percentage value, and weight together.
- Confirm the historical template revision and option mapping before treating a displayed label or score as incorrect.

## Consolidated Historical Cases

- [CONVI-6753](../../work-items/CONVI-6753.md): a zero configured contribution weight is distinct from an inapplicable criterion. The May investigation linked criterion N/A to a zero denominator after direct ClickHouse writes bypassed a historic compatibility weight. Its `1e-13` substitution and backfill requirement are historical implementation notes, not a general rule for excluded criteria or proof of deployment. Verify the write path and criterion/overall denominators together when revisiting the case.

## Legacy Sources and Cases

- [CONVI-6753](../../work-items/CONVI-6753.md)
- [CONVI-6808](../../work-items/CONVI-6808.md)
- [CONVI-5805](../../work-items/CONVI-5805.md)
- [achieve-excluded-criteria-na-investigation](../../../scorecard-workflows/work-items/achieve-excluded-criteria-na-investigation.md)
- [na-score-representation-and-fallback](../../../scorecard-workflows/work-items/na-score-representation-and-fallback.md)
- [Mixed-revision QA score semantics](../../deliverables/mixed-revision-qa-score-semantics.md)
- [CONVI-7378 SCAN investigation](../../deliverables/convi-7378-scan-consent-qa-score-investigation.md)
- [CONVI-7254 HCD mixed-revision investigation](../../deliverables/hcd-mixed-revision-qa-score-investigation.md)

## Open Questions

- Publish a field-by-field truth table for criterion, chapter, and overall score aggregation.
- Catalog every frontend fallback for null, N/A, zero, and missing rows.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
