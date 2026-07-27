# CONVI-7254 February QA score investigation

## Context

- Source repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/go-servers`
- Backend worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- Backend branch: `convi-7254-monthly-qa-score`
- Ticket: [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254/home-care-delivered-performance-insights-monthly-view-shows-0percent)
- Customer database: `home_care_delivered_us_east_1` on `us-east-1-prod`
- Template: `01995dc7-348c-7288-9d51-23bc68b45e86`
- Criterion: `0199c056-ce00-751a-ab4b-ecc11e4d7412`
- Period: February 2026 in America/Toronto

## Reported Symptom

The February Performance Insights cell for “Asking ‘How did you hear about HCD?’” shows 100%, but the per-agent popover contains many scores below 100%, including Stacy Brown at 33%.

The supplied per-agent response contains 428 scorecards. Most agent groups have `weightSum = scorecard_count × 1e-13`, but Latrese Proctor has score 1, eight scorecards, and `weightSum = 1`.

## Frontend Trace

- The main Performance Progression request groups by criterion and time range.
- The response is grouped by criterion and converted directly into date cells; each cell uses the backend `QAScore.score`.
- Opening a cell builds a request scoped to the cell's date range and criterion and groups by agent only. The supplied `AGENT + CRITERION` capture is not the exact live-popover shape; it is nevertheless equivalent for this investigation because the request filters to one criterion. The Performance Progression CSV export is the nearby flow that groups by agent, criterion, and time range.
- The popover headline displays `qaScoreResult.averageQaScore`.
- Each agent row displays the corresponding returned `QAScore.score`.
- The frontend sorts and formats the values but does not aggregate agent percentages into the headline.

Relevant source paths:

- `director/packages/director-app/src/features/insights/performance/performance-progression/PerformanceProgression.tsx`
- `director/packages/director-app/src/features/insights/performance/performance-progression/utils.ts`
- `director/packages/director-app/src/features/insights/performance/performance-progression/useColumnsForPerformanceProgression.tsx`
- `director/packages/director-app/src/components/insights/qa-insights/cell/QAICell.tsx`
- `director/packages/director-app/src/components/insights/qa-insights/getValueTypeConfig.tsx`
- `director/packages/director-api/src/services/cresta-api/insights/transformersQAI.ts`

## Exact ClickHouse Reproduction

Reconstructed the generated `RetrieveQAScoreStats` query with:

- February Toronto boundaries: `[2026-02-01 04:00:00Z, 2026-03-01 04:00:00Z)`;
- Intake use case;
- template `01995dc7-348c-7288-9d51-23bc68b45e86`;
- criterion `0199c056-ce00-751a-ab4b-ecc11e4d7412`;
- voicemail exclusion;
- latest scorecard-version deduplication;
- agent and criterion grouping.

The query reproduces the supplied response:

- 428 scorecards;
- total weighted numerator `1.0000000000093001`;
- total weight denominator `1.0000000000426996`;
- `averageQaScore = 0.9999999999666005`, narrowed/serialized as 1 and displayed as 100%.

The source rows are:

- revision `8dd94019`, weight 1, passing: one scorecard;
- revision `1ea5882b`, semantic weight zero stored as `1e-13`, failing: one scorecard;
- revision `90dfe0ba`, semantic weight zero stored as `1e-13`, passing: 93 scorecards;
- revision `90dfe0ba`, semantic weight zero stored as `1e-13`, failing: 333 scorecards.

An equal-observation criterion result is therefore:

```text
(1 + 93) passing / 428 applicable = 21.96%
```

## Why the Popover Looks Contradictory

Latrese Proctor owns the only weight-1 scorecard:

- one passing weight-1 scorecard from revision `8dd94019`;
- seven failing semantic-weight-zero scorecards from revision `90dfe0ba`;
- returned weighted score approximately 100%;
- equal-observation score `1 / 8 = 12.5%`.

Stacy Brown has three semantic-weight-zero scorecards from the same revision, one passing and two failing. Their identical stored weights cancel within her group, so her returned score is correctly 33.33%.

When computing `averageQaScore`, the backend does not average the visible agent percentages. It sums each ClickHouse row's weighted numerator and denominator. Latrese's unit-weight passing row therefore has roughly ten trillion times the influence of each semantic-weight-zero row, causing both the cell and popover headline to show 100%.

## Backend Aggregation

ClickHouse computes each group as:

```text
weighted_percentage_sum = SUM(percentage_value × float_weight)
weight_sum = SUM(float_weight)
group score = weighted_percentage_sum / weight_sum
```

Response conversion then computes:

```text
averageQaScore =
  SUM(group weighted_percentage_sum) /
  SUM(group weight_sum)
```

This reconstructs the source-level weighted result; it is not a simple or scorecard-count-weighted average of agent scores.

## Template Weight Timeline

- The criterion carried weight 1 from its introduction in October 2025 through revision `bb802304`, created 2026-02-13 16:14 Toronto.
- Revision `1ea5882b`, created 2026-02-13 16:26 Toronto, changed the criterion to semantic weight zero.
- Subsequent revisions through July retained semantic weight zero.
- Historical score rows keep values from their referenced revisions, so the old `8dd94019` unit-weight row remains comparable numerically but not semantically with newer zero-weight rows.

## Conclusion

The February symptom is the same cross-revision criterion-weight defect as the May symptom, with the direction reversed:

- May: a few weight-1 failures force the result toward 0%.
- February: one weight-1 pass forces the result toward 100%.

The frontend request and rendering are consistent with the API contract. The data is internally consistent. The incorrect user-facing result is produced by applying template contribution weights to a criterion's own historical observations across revisions.
