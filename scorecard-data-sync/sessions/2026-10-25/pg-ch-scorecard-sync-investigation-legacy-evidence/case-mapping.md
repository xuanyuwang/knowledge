# Case Mapping

## Purpose

Map prior scorecard sync and discrepancy work into a common taxonomy.

## Source Projects

- [backfill-scorecards](../../../work-items/CONVI-6209.md)
- [convi-5565-scorecard-ch-pg-sync](../../../work-items/CONVI-5565.md)
- [hilton-coaching-discrepancy](../../../../analytics/work-items/hilton-coaching-report-leaderboard-parity.md)
- [scorecard auto-heal](../../../../scorecard-workflows/sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/auto-heal-design.md)

## Mapping Template

For each prior case, capture:

- Symptom
- Detection mechanism
- Root-cause class
- Prevention gap
- Detection gap
- Repair gap
- Residual risk after the fix

## Initial Hypothesis

The prior work likely spans at least three distinct classes:

1. **Stale overwrite races**
   Newer PostgreSQL state exists, but an older asynchronous projection wins in ClickHouse.
2. **Silent missing writes**
   PostgreSQL is correct, but ClickHouse never receives a durable projection.
3. **Recovery coverage gaps**
   Backfill exists, but its selection logic does not cover all valid scorecards.
