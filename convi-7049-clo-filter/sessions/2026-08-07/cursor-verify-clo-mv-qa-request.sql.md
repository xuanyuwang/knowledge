# Session: verify CLO MV vs raw SQL for walter-dev QA request

**Date:** 2026-08-07
**Tool:** Cursor
**Cluster:** voice-staging / `cresta_walter_dev`

## Request

`RetrieveQAScoreStats` for `customers/cresta/profiles/walter-dev`:

- Window: `[2026-08-01 04:00:00, 2026-08-08 03:59:59)` UTC (America/Toronto day bounds)
- Usecase: `walter-dev`
- CLO moment: `7252ec08-0620-4c83-93aa-6ccc84b6750b`
- Outcome strings: `resolved` OR `unresolved`
- Group by: DAILY TIME_RANGE only
- `includeNaScored: false`, score resource: criteria/`score_d`

## Composed path differences

| | Flag off (raw) | Flag on (MV) |
|---|---|---|
| Table | `moment_annotation_d` | `moment_annotation_by_conversation_outcome_d` |
| Latest column | `create_time` | `update_time` |
| Type filter | `moment_type = 14` | omitted (storage is type-14 only) |
| Value predicate | `JSONHas` + `JSONExtractString(...)=` | `outcome_value_type='string' AND outcome_string_value=` |

Shared score body: `scorecard_d` / `score_d` with `usecase_id IN ('walter-dev')`, `percentage_value >= 0`, `not_applicable <> true`, Toronto `toIntervalHour(-4)` day trunc.

SQL artifacts: `/tmp/clo-mv-verify/{full_raw,full_mv,filter_diff,full_compare}.sql`

## Results

Filter conversation sets (`resolved` OR `unresolved`):

- raw=1957, mv(update_time)=1957, raw_only=0, mv_only=0
- raw vs mv(create_time) also identical 1957/0/0

Full QA score-stats aggregates:

- rows=6/6, sum_convs=85/85, sum_scorecards=2055/2055, sum_weight=2445/2445
- abs weighted diff=0, mismatched_days=0
- Per-day rows bit-identical (float epsilon only)

## Verdict

For this request on staging `cresta`/`walter-dev`, flag on and flag off produce matching correctness.
