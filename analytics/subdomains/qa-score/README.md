# QA Score

## Purpose

Own QA score calculation and display semantics from template options and score rows through analytics aggregation and frontend presentation.

## Semantics and Invariants

- Option identity, option index, raw/numeric value, mapped score, percentage score, and display label are distinct fields.
- N/A or null means no applicable score; it must not be collapsed into numeric zero.
- A zero-weight or excluded criterion can carry a raw value while contributing no percentage score.
- Chapter and criterion aggregates have different denominators and must remain distinguishable.
- Score rows retain percentage and weight values derived from their referenced template revision; a later template revision does not rewrite historical rows.
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

## Legacy Sources and Cases

- `convi-6753-weight-zero-pi-na/`
- `convi-6808-greenix-pi-scores/`
- `qa-score-popover-fix/`
- `convi-6672-achieve-behavior-na/`
- `nascore/`
- [Mixed-revision QA score semantics](../../deliverables/mixed-revision-qa-score-semantics.md)
- [CONVI-7378 SCAN investigation](../../deliverables/convi-7378-scan-consent-qa-score-investigation.md)
- [CONVI-7254 HCD mixed-revision investigation](../../deliverables/hcd-mixed-revision-qa-score-investigation.md)

## Open Questions

- Publish a field-by-field truth table for criterion, chapter, and overall score aggregation.
- Catalog every frontend fallback for null, N/A, zero, and missing rows.
