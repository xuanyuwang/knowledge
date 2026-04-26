# Case Mapping

## Purpose

Map prior scorecard sync and discrepancy work into a common taxonomy.

## Source Projects

- [backfill-scorecards](../backfill-scorecards/README.md)
- [convi-5565-scorecard-ch-pg-sync](../convi-5565-scorecard-ch-pg-sync/README.md)
- [hilton-coaching-discrepancy](../hilton-coaching-discrepancy/README.md)
- [mismatch-scorecard-count](../mismatch-scorecard-count/auto-heal-design.md)

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
