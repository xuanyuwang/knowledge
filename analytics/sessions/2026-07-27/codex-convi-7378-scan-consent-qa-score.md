# CONVI-7378 SCAN Consent to Call investigation

**Ticket:** [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating)
**Domain:** analytics (`qa-score`)
**Pattern:** mixed-revision QA score semantics — pinned-revision option-score inversion

## Source

- Slack: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785114593796649
- Linear: https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating

## Summary

Investigation confirmed PG/CH alignment, then reclassified the issue under analytics. Consent to Call in template revision `a88e3c94` has inverted option-to-score mapping (Yes = 0%). The scorecard is pinned to that revision; PI 0% and section 83.3% are consistent with stored revision semantics, not sync failure.

## Key evidence

- Scorecard `019f8c65-1276-71f6-bc84-8304492da8d9`, revision `a88e3c94`, overall score 95.8.
- Consent: `numeric_value=0` (Yes), `ai_value=1` (original auto No).
- Section aggregate: 83.3% = (5×100% + 1×0%) / 6 under pinned revision.
- CH `scan_health_us_west_2.score_d` / `scorecard_d` match Postgres.

## Classification

Not `scorecard-data-sync`. Same pattern family as CONVI-7238 and CONVI-7254.

## Deliverables

- Work item: `work-items/CONVI-7378.md`
- PM/manager report: `deliverables/convi-7378-scan-consent-qa-score-investigation.md`
