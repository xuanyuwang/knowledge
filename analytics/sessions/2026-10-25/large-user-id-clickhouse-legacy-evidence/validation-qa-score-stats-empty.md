# Validation: RetrieveQAScoreStats Empty Response for cresta/walter-dev

**Created:** 2026-03-13
**Environment:** voice-staging
**Result: CONFIRMED — empty response is correct**

## Request

```json
{
  "parent": "customers/cresta/profiles/walter-dev",
  "metadata": {"timeZoneId": "America/Toronto"},
  "frequency": "WEEKLY",
  "filterByTimeRange": {
    "startTimestamp": "2026-02-08T05:00:00.000Z",
    "endTimestamp": "2026-03-14T03:59:59.999Z"
  },
  "groupByAttributeTypes": ["QA_ATTRIBUTE_TYPE_AGENT"],
  "filterByAttribute": {
    "users": [],
    "groups": [],
    "scorecardTemplates": ["customers/cresta/profiles/walter-dev/scorecardTemplates/0196d606-34f9-7683-9ced-bb84529f380e"],
    "includeNaScored": false,
    "criterionIdentifiers": ["019c2ba5-b130-7571-b6f9-0800b696c972"],
    "usecaseNames": ["customers/cresta/profiles/walter-dev/usecases/walter-dev"],
    "excludeDeactivatedUsers": false,
    "scorecardStatuses": []
  },
  "filterToAgentsOnly": false
}
```

## Expected Response

```json
{
  "qaScoreResult": {
    "scores": [],
    "averageQaScore": 0,
    "totalConversationCount": 0,
    "totalScorecardCount": 0
  }
}
```

## Validation: Why Empty is Correct

### Two independent reasons lead to empty results:

### Reason 1: Criterion `019c2ba5-b130-7571-b6f9-0800b696c972` has ZERO rows in ClickHouse

The request filters on `criterionIdentifiers: ["019c2ba5-b130-7571-b6f9-0800b696c972"]`. This criterion **does not exist at all** in the `score_d` table:

```
CH query: SELECT count(*) FROM score_d WHERE criterion_id = '019c2ba5-b130-7571-b6f9-0800b696c972'
Result:   0
```

The only criterion that exists for this template is `0196d606-39f2-779d-a56e-cee059ac52f1` (1 row).

**Why?** From PG, the template's JSON shows criterion `019c2ba5-b130-7571-b6f9-0800b696c972` ("Sebastian - modeled metadata outcome") has:
- `"excludeFromQAScores": true`
- `"type": "dropdown-numeric-values"` with empty scores/options arrays

This criterion has never been scored in ClickHouse — no scorecard has ever recorded a score for it.

### Reason 2: Template has ZERO data in the requested time range

Even ignoring the criterion filter, the template `0196d606-34f9-7683-9ced-bb84529f380e` has **no data at all** in the time range `2026-02-08 to 2026-03-14`:

```
CH query: SELECT count(*) FROM score_d
          WHERE scorecard_template_id = '0196d606-34f9-7683-9ced-bb84529f380e'
            AND scorecard_time >= '2026-02-08 05:00:00'
            AND scorecard_time < '2026-03-14 04:00:00'
            AND usecase_id = 'walter-dev'
Result:   0
```

The template's only data is 1 row from `2026-01-20 15:29:33`, which is outside the requested time range.

### Complete template data in ClickHouse

| scorecard_time | usecase_id | agent_user_id | criterion_id | percentage_value | scorecard_id |
|---|---|---|---|---|---|
| 2026-01-20 15:29:33 | walter-dev | 9e151f0863143402 | 0196d606-39f2-779d-a56e-cee059ac52f1 | 1 | 019c58b1-96ba-729f-8774-96bd20466f2a |

Only 1 record total, for a different criterion, and outside the time range.

## Code Path Analysis

From `retrieve_qa_score_stats_clickhouse.go:527-669`:

1. **Table selection**: No `scoreResource` in request → defaults to `scoreTable` (criterion-level `score_d`)
2. **`parseCommonConditionsForQAAttribute`** (line 540): Builds WHERE on `scorecard_template_id IN (...)` and `usecase_id IN (...)`
3. **`parseScoreConditionsForQAAttribute`** (line 548): Adds `criterion_id IN ('019c2ba5-b130-7571-b6f9-0800b696c972')` — line 688-693 of `common_clickhouse.go`
4. **`includeNaScored: false`**: Adds `percentage_value >= 0 AND not_applicable != true`
5. **Empty `users`**: No user WHERE clause added
6. **Empty `scorecardStatuses`**: No status filter
7. **`groupByAttributeTypes: [AGENT]`**: Adds `GROUP BY agent_user_id`
8. **No moment groups**: Uses `qaScoreStatsClickhouseQuery` (simple path, no moment joins)

The generated query effectively becomes:
```sql
WITH
  scorecard AS (SELECT * FROM scorecard_d WHERE scorecard_time >= ... AND scorecard_time < ... AND usecase_id IN ('walter-dev') AND scorecard_template_id IN ('0196d606-34f9-7683-9ced-bb84529f380e')),
  ...
  scorecard_score AS (SELECT ... FROM score_d WHERE scorecard_time >= ... AND scorecard_time < ... AND usecase_id IN ('walter-dev') AND scorecard_template_id IN ('0196d606-34f9-7683-9ced-bb84529f380e') AND criterion_id IN ('019c2ba5-b130-7571-b6f9-0800b696c972') AND percentage_value >= 0 AND not_applicable != true)
SELECT agent_user_id, SUM(...), ... FROM scorecard_score JOIN filtered_scorecard ... GROUP BY agent_user_id
```

Both `scorecard` CTE and `scorecard_score` CTE return 0 rows → JOIN produces 0 rows → empty response.

## PG Context

The scorecard template exists in PG with 13 revisions:

| Field | Value |
|---|---|
| resource_id | 0196d606-34f9-7683-9ced-bb84529f380e |
| title | "Krystal Test Template" |
| status | 1 (active) |
| usecase_ids | {walter-dev} |
| latest revision created_at | 2026-03-09 14:51:52 |

The criterion `019c2ba5-b130-7571-b6f9-0800b696c972` is in the template JSON as "Sebastian - modeled metadata outcome" with `excludeFromQAScores: true`.

## Conclusion

The empty response is **correct** for two independent reasons:
1. The filtered criterion (`019c2ba5-b130-7571-b6f9-0800b696c972`) has never been scored in ClickHouse — 0 rows exist
2. The template has no data at all in the requested time range (Feb 8 – Mar 14) — the only data is from Jan 20
