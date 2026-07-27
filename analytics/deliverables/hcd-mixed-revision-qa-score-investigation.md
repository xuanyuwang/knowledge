# HCD mixed-revision QA score investigation

**Ticket:** [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254/home-care-delivered-performance-insights-monthly-view-shows-0percent)  
**Customer:** Home Care Delivered  
**Surface:** Performance Insights  
**Template:** Intake Live QM (`01995dc7-348c-7288-9d51-23bc68b45e86`)  
**Criteria:** Asking for phone number (`019e7502-0e12-756b-a7ca-c81276df1781`); Asking "How did you hear about HCD?" (`0199c056-ce00-751a-ab4b-ecc11e4d7412`)
**Validated:** 2026-07-27

## Executive Summary

Two apparently opposite monthly discrepancies have the same cause. The May result for "Asking for phone number" displays 0%, while the February result for "How did you hear about HCD" displays 100%, because each aggregation combines score rows created from template revisions in which the same criterion had radically different influence.

In May, three fully influential failures overwhelm 4,747 semantic-weight-zero evaluations. In February, one fully influential pass overwhelms 427 semantic-weight-zero evaluations. The weight-1 rows determine the displayed result regardless of the much larger underlying population.

This is a backend aggregation-semantics defect in `RetrieveQAScoreStats`. The exact generated ClickHouse query reproduces the API response. The discrepancy is not caused by frontend rounding, caching, timezone boundaries, voicemail filtering, or response conversion.

## Non-Engineering Explanation

Most evaluations were created when this question had no impact on the overall score, so they carry almost no voting power. Three evaluations came from a short-lived template version where the question had full impact. When the monthly report combines them, those three evaluations effectively overrule the other 4,747. Because all three influential evaluations failed, the result appears as 0%.

The issue is unequal influence, not simply that three failures were added to the month.

## May Production Data: 0% Instead of 27.43%

The May monthly bucket contains 4,750 applicable scorecards:

- 1,303 passing rows from revision `2ab0f092`, configured with semantic weight zero and physically represented as `1e-13`;
- 3,444 failing rows from revision `2ab0f092`, also represented as `1e-13`;
- three failing rows from revision `33e46102`, configured with weight 1.

An equal-observation criterion result would be:

```text
1,303 passing / 4,750 applicable = 27.43%
```

The current weighted result is:

```text
weighted numerator = 1,303 × 1e-13
weight denominator = 4,747 × 1e-13 + 3 × 1
result = 4.3433e-11
displayed percentage = 0%
```

## February Production Data: 100% Instead of 21.96%

The February bucket for "How did you hear about HCD" contains 428 applicable scorecards:

- one passing row from revision `8dd94019` with weight 1;
- one failing row from revision `1ea5882b` with semantic weight zero, stored as `1e-13`;
- 93 passing and 333 failing rows from revision `90dfe0ba`, also stored as `1e-13`.

An equal-observation criterion result is:

```text
94 passing / 428 applicable = 21.96%
```

The current weighted result is:

```text
weighted numerator = 1 + 93 × 1e-13
weight denominator = 1 + 427 × 1e-13
result = 0.9999999999666005
displayed percentage = 100%
```

The per-agent popover is not contradictory once the weights are inspected. Stacy Brown's three rows all have the same tiny stored weight, so one pass and two failures produce 33.33%. Latrese Proctor owns the unit-weight pass; it overwhelms her seven semantic-weight-zero failures, so her eight-scorecard result is displayed as 100% even though an equal-observation result is 12.5%.

## How the Calculation Reaches the Wrong Result

1. Each scorecard resolves the template revision it references.
2. Its criterion percentage and configured criterion weight are derived from that revision.
3. A semantic zero weight is stored as `1e-13` so downstream arithmetic remains defined.
4. `RetrieveQAScoreStats` filters and deduplicates score rows correctly.
5. Criterion results are grouped by time bucket, criterion ID, and template ID. Template revision is not a grouping key.
6. The aggregation directly combines percentage and weight values from every qualifying revision.
7. The few weight-1 rows dominate the denominator formed mostly from `1e-13` rows.
8. ClickHouse returns a result dominated by the unit-weight rows, which the UI formats as either 0% or 100%.

The error occurs at criterion-level aggregation. Template contribution weight is meaningful when combining different criteria into an overall QA score, but using it to determine the influence of observations within one criterion makes historical revisions incomparable.

## Weight Revision Timeline

### Asking for phone number

The history is `1 → 0 → 1`, rather than a single zero-to-one update:

- Before revision `33e46102`, the criterion was absent from the template revision records.
- **2026-05-29 14:32:09 Toronto:** revision `33e46102` introduced the criterion with weight 1. Three failing May score rows reference it.
- **2026-05-29 14:35:53 Toronto:** revision `2ab0f092` changed the weight to semantic zero, only 3 minutes 44 seconds later.
- **2026-06-08:** revisions `0a6d5240` and `006f60ab` retained weight zero.
- **2026-06-30:** revisions `d41ff713` and `52644909` retained weight zero.
- **2026-07-01 07:37:43 Toronto:** revision `d0aa3ef7` restored weight 1.
- **2026-07-24:** six subsequent revisions retained weight 1.

Restoring the latest template to weight 1 does not repair historical rows. Each score row permanently retains values derived from its referenced template revision.

### How did you hear about HCD

- The criterion had weight 1 from October 2025 through revision `bb802304`, created 2026-02-13 16:14 Toronto.
- Revision `1ea5882b`, created 2026-02-13 16:26 Toronto, changed the criterion to semantic weight zero.
- Every subsequent revision through July retained semantic weight zero.
- The February source still contains one passing scorecard referencing the older unit-weight revision `8dd94019`; later or reprocessed rows use zero-weight revisions.

## Why Monthly and Daily Views Differ

Most daily buckets contain only one weight regime, so all rows in the bucket have comparable influence. May 29 contains both regimes and is also effectively zero. The monthly bucket combines May 29 with all other May dates, exposing the incompatibility across a much larger population.

This confirms that monthly truncation itself is functioning correctly; the larger bucket merely makes mixed-revision weighting visible.

## Validation Evidence

- Queried latest scorecard versions in HCD production ClickHouse.
- Reconstructed the exact `RetrieveQAScoreStats` SQL shape from unit-test golden queries.
- Preserved the production voicemail filter, latest-scorecard CTE, distinct score projection, criterion/time grouping, and Toronto timezone boundaries.
- Reproduced the May API result exactly:
  - weighted numerator: `1.3029999999999998e-10`;
  - weight sum: `3.0000000004746927`;
  - score: `4.343333332646083e-11`;
  - conversations and scorecards: `4,750`.
- Verified an unweighted monthly criterion average of `0.27431578947368424`.
- Verified an unweighted May 29 criterion average of `0.2146118721461187`.
- Reproduced the February per-agent response:
  - weighted numerator: `1.0000000000093001`;
  - weight sum: `1.0000000000426996`;
  - score: `0.9999999999666005`, displayed as 100%;
  - scorecards: `428`;
  - equal-observation result: `94 / 428 = 21.96%`.
- Traced the Director cell and popover request/rendering paths and verified that no frontend agent-score averaging produces the headline.
- Queried production PostgreSQL `director.scorecard_templates` to recover both criteria's revision histories.

## Hypotheses Evaluated

- **Confirmed:** mixed criterion weights across referenced template revisions dominate criterion-level aggregation.
- **Rejected:** monthly timezone truncation or incorrect bucket boundaries.
- **Rejected:** voicemail filtering or join-cardinality duplication.
- **Rejected:** frontend formatting, response conversion, caching, or floating-point narrowing.

## Open Decision

Confirm the intended criterion-grouped contract before implementation. The likely semantic is that every applicable scorecard contributes one observation to its criterion result, independently of the criterion's template contribution weight, while configured weights remain applicable when combining different criteria into chapter or overall QA scores.

After agreement:

1. add a regression test covering one criterion across zero- and non-zero-weight revisions;
2. implement the narrow aggregation change;
3. rerun the exact monthly and daily production queries;
4. verify that overall QA score weighting remains unchanged.

## Evidence Links

- Canonical work item: [`../work-items/CONVI-7254.md`](../work-items/CONVI-7254.md)
- May session evidence: [`../sessions/2026-07-24/codex-convi-7254-monthly-qa-score.md`](../sessions/2026-07-24/codex-convi-7254-monthly-qa-score.md)
- February session evidence: [`../sessions/2026-07-27/codex-convi-7254-february-qa-score.md`](../sessions/2026-07-27/codex-convi-7254-february-qa-score.md)
- QA score domain semantics: [`../subdomains/qa-score/README.md`](../subdomains/qa-score/README.md)
