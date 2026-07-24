# CONVI-7254 monthly QA score investigation

## Context

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- Branch: `convi-7254-monthly-qa-score`
- Ticket: [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254/home-care-delivered-performance-insights-monthly-view-shows-0percent)
- Customer database: `home_care_delivered_us_east_1` on `us-east-1-prod`
- Template: `01995dc7-348c-7288-9d51-23bc68b45e86`
- Criterion: `019e7502-0e12-756b-a7ca-c81276df1781`

## Hypotheses

1. Mixed `float_weight` representations across source rows or template revisions cause a few large weights to dominate the monthly denominator.
2. Monthly time truncation or timezone handling places unexpected rows into the May bucket.
3. The excluded-voicemail metadata query path duplicates or filters score rows differently by frequency.
4. API conversion, caching, or floating-point narrowing changes an otherwise correct ClickHouse aggregate.

## Runtime Evidence

- The reported API row has `score=4.3433333e-11`, `weightSum=3`, and 4,750 scorecards. This implies a weighted numerator near `1.303e-10`.
- A production query over latest scorecard versions found:
  - revision `2ab0f092`: 1,303 passing semantic weight-zero rows represented as `float_weight=1e-13`;
  - revision `2ab0f092`: 3,444 failing semantic weight-zero rows represented as `float_weight=1e-13`;
  - revision `33e46102`: three failing rows at `float_weight=1`.
- The source math therefore produces `(1303 * 1e-13) / (3 + 4747 * 1e-13) = 4.343333...e-11`, matching the API response.
- Daily aggregation is non-zero on most dates but effectively zero on 2026-05-29, where all three unit-weight rows occur. This weakens a pure monthly truncation hypothesis.
- The prior CONVI-6753 fix (`15822775f3`) preserves semantic weight zero while representing it as the tiny non-zero value `1e-13`, inherited from historic PostgreSQL behavior, so divisions remain defined. It does not normalize rows when another template revision gives the same criterion a non-zero weight.
- Reconstructed the exact production query from `clickhouse_RetrieveQAScoreStats_GroupByAgentTime_request.sql`, `clickhouse_RetrieveQAScoreStats_GroupByAgentCriterion_request.sql`, and `clickhouse_RetrieveQAScoreStats_GroupByAgent_EnableLastScorecardFilter_request.sql`, retaining the conversation voicemail filter, latest-scorecard CTE, distinct score projection, criterion/time grouping, and Toronto offset.
- Exact monthly result: weighted numerator `1.3029999999999998e-10`, weight sum `3.0000000004746927`, score `4.343333332646083e-11`, 4,750 conversations, and 4,750 scorecards. This matches the API response.
- Exact monthly unweighted average: `0.27431578947368424`.
- Exact daily query matches the unweighted score on every day except May 29. On May 29, weighted score is `1.5666666666553878e-12` while unweighted score is `0.2146118721461187`, because the three failing unit-weight rows occur in that bucket.

## Hypothesis Evaluation

- Hypothesis 1, mixed weights across revisions: confirmed.
- Hypothesis 2, monthly timezone truncation: rejected; the bucket boundaries and row counts are correct.
- Hypothesis 3, voicemail filtering or join cardinality: rejected; the exact query shape still returns 4,750 unique conversations and scorecards and matches the API.
- Hypothesis 4, response conversion, cache, or float narrowing: rejected; raw ClickHouse output already contains the near-zero score.
- Temporary source instrumentation was removed after exact-query verification. No product fix has been applied.

## Next Steps

1. Confirm the intended criterion-grouped semantic: likely average applicable criterion percentages independently of template weight while preserving weights for overall QA score.
2. Add a regression test covering one criterion across revisions with zero and non-zero configured weights.
3. Implement the narrow query change and verify monthly and daily results.

## Report

- Created the standalone Cursor canvas `CONVI-7254-calculation-pipeline.canvas.tsx`.
- Promoted the investigation into the durable report [`deliverables/hcd-mixed-revision-qa-score-investigation.md`](../../deliverables/hcd-mixed-revision-qa-score-investigation.md).
- The report first demonstrates the failure with a 101-row mock example, then walks the 4,750 HCD production rows through source filtering, time/criterion grouping, weighted aggregation, API conversion, and UI rounding.
- It contrasts the exact API result (`4.3433e-11`, displayed as 0%) with the equal-observation criterion result (`1,303 / 4,750 = 27.43%`) and identifies the weighted aggregation stage as the point where the number becomes incorrect.

## Revision and Formula Semantics

- Score-row generation resolves the template structure by the scorecard's own `(template_id, template_revision)`, not by the latest template revision. The resulting percentage value, semantic criterion weight, and template revision are persisted on the ClickHouse score row.
- Semantic weight zero is physically stored as `1e-13` so downstream division remains defined.
- `RetrieveQAScoreStats` deduplicates to the latest update of each scorecard, which is distinct from choosing the latest template revision.
- The normal query filters by `scorecard_template_id`, then groups criterion results by `(time bucket, criterion_id, scorecard_template_id)` without `scorecard_template_revision`.
- Current criterion result formula is `SUM(percentage_value * float_weight) / SUM(float_weight)` across all qualifying scorecard rows, so scorecards referencing different template revisions are merged and retain the weights computed from their respective referenced revisions.

## Criterion Weight Timeline

Production PostgreSQL `director.scorecard_templates` revision records for template `01995dc7-348c-7288-9d51-23bc68b45e86`, queried on 2026-07-24 and converted to America/Toronto time:

- Before `33e46102`, the target criterion does not appear in this template's revision records.
- `33e46102`, created 2026-05-29 14:32:09.577725: criterion introduced with configured weight 1. Three failing May score rows reference this revision.
- `2ab0f092`, created 2026-05-29 14:35:53.994973: configured weight changed to 0 after 3 minutes 44 seconds. The 4,747 other May score rows reference this revision and store the semantic zero as `1e-13`.
- `0a6d5240` and `006f60ab`, created 2026-06-08: weight remained 0.
- `d41ff713` and `52644909`, created 2026-06-30: weight remained 0.
- `d0aa3ef7`, created 2026-07-01 07:37:43.388133: configured weight restored to 1.
- Six revisions created 2026-07-24 between 11:40 and 11:44 retained weight 1.

The resulting history is `1 → 0 → 1`. The anomalous May data came from the brief initial weight-1 revision, not from the July restoration. Later template revisions do not rewrite historical score rows because each score row retains the values derived from the scorecard's referenced revision. The canvas report now includes this timeline.
