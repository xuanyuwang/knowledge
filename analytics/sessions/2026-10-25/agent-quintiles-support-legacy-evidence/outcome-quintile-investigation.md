# Outcome Quintile Investigation — How Outcomes Participate in QA Score & Quintile

**Created:** 2026-03-09

## Question

For the leaderboard-by-criteria table on the Performance page, how are outcomes (AHT, Conversion, CSAT) involved in the overall agent quintile calculation? Outcomes have fundamentally different calculation methods unlike criteria — are they normalized to 0-1 percentages?

## Key Findings

### 1. Outcomes and QA Criteria Are in the Same Scoring Pipeline

Unlike `RetrieveConversationOutcomeStats` (which stores raw values in `user_outcome_field_value_d`), scorecard outcomes are stored **in the same ClickHouse tables** as criteria (`score_d`, `scorecard_score_d`). They go through the same `percentage_value` / `float_weight` aggregation path.

### 2. `percentage_value` Is NOT Always a Percentage

Verified against template `019b2dde-96c1-76aa-b8a5-6d0738eeaeb2` ("Product Template", revision `4c8f8695`) in `voice-sandbox-2`:

| Item | Type | `percentage_value` range | `numeric_value` range | Normalized? |
|------|------|-------------------------|----------------------|-------------|
| **AHT** | numeric-radios | 59 – 2998 | 59 – 2998 | **No** — raw seconds stored as-is |
| **Conversion** | dropdown-numeric-values | 0 – 1 | 0 – 1 | Yes — binary score from template mapping |
| **CSAT (Cresta Prediction)** | dropdown-numeric-values | 0 – 1 | 1 – 2 | Yes — `scores` mapping converts dropdown value to 0/1 |
| **Thank You Greeting** (criteria) | labeled-radios (Yes/No) | 0 – 1 | 0 – 1 | Yes — binary |

For **dropdown-numeric-values** items, the `scores` config in the template maps raw values to normalized scores. Example for CSAT:
- "too short to evaluate" (value 0) -> score 0
- "low CSAT" (value 1) -> score 0
- "high CSAT" (value 2) -> score 1

The **score** (not the raw value) becomes `percentage_value`. So CSAT's `percentage_value` is 0 or 1.

For **numeric-radios** items (like AHT), `percentage_value` gets the raw numeric value with **no normalization** — hence AHT shows 59–2998 (seconds).

### 3. `excludeFromQAScores` Is the Gate

The `excludeFromQAScores` flag per template item controls whether it participates in QA score aggregation:

**Template items:**

| Item | `excludeFromQAScores` | Participates in QA score? |
|------|----------------------|--------------------------|
| AHT | `true` | **No** — filtered out |
| Conversion | `true` | **No** — filtered out |
| CSAT (Cresta Prediction) | `false` | **Yes** — 0/1 score enters weighted average |
| Thank You Greeting | null (default false) | **Yes** |
| Offer to Assist | null (default false) | **Yes** |
| Show Empathy | null (default false) | **Yes** |
| Ensure Complete Resolution | null (default false) | **Yes** |
| Thank You Closing | null (default false) | **Yes** |

### 4. BE Filtering Mechanism

**File:** `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

`getScoreableCriteria()` (lines 700-720):
1. Parses the scorecard template JSON
2. Iterates all items via `GetCriteriaSlice(false)`
3. Collects only those where `IsExcludeFromQAScores() == false`
4. Returns list of scoreable criterion IDs

This list is injected into `req.FilterByAttribute.CriterionIdentifiers` (line 211), becoming a `WHERE criterion_id IN (...)` filter on the ClickHouse query. Items with `excludeFromQAScores: true` are excluded **at query time**.

```go
// Line 195-212
if *EnableQAScoreScoreableCriteriaOnly &&
    req.ScoreResource != analyticspb.RetrieveQAScoreStatsRequest_QA_SCORE_RESOURCE_SCORECARD {
    criteriaIDFilters := req.GetFilterByAttribute().GetCriterionIdentifiers()
    groupByFlag := postgres.CreateEnumFlag(req.GroupByAttributeTypes)
    if len(criteriaIDFilters) == 0 && !postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION) {
        scoreableCriteria, err := a.getScoreableCriteria(ctx, ...)
        req.FilterByAttribute.CriterionIdentifiers = scoreableCriteria
    }
}
```

The ClickHouse aggregation then runs only on included criteria:
```sql
SUM(percentage_value * float_weight) / SUM(float_weight)
```

### 5. Quintile Calculation Flow (for this template)

For a `groupBy: [AGENT]` request filtered to this template:

1. `getScoreableCriteria()` returns 6 IDs: CSAT + 5 servicing criteria (AHT and Conversion excluded)
2. ClickHouse query: `WHERE criterion_id IN (6 IDs) GROUP BY agent_user_id`
3. Per-agent score = weighted average of CSAT (0/1) and 5 Yes/No criteria (0/1), all weight=1
4. `setQuintileRankForPerAgentScores()` ranks agents by this score, assigns Q1-Q5

### 6. Risk: Misconfigured Templates

If someone accidentally sets `excludeFromQAScores: false` on AHT:
- Raw seconds (59–2998) would enter the weighted average alongside 0–1 criteria scores
- AHT would completely dominate the result (e.g., agent score becomes ~500 instead of ~0.8)
- Quintile ranking would essentially rank agents by AHT alone

**The system relies on correct template configuration, not on enforcing normalization.** There is no validation that `percentage_value` is actually in the 0–1 range for items with `excludeFromQAScores: false`.

### 7. Edge Case: Explicit Request for an Excluded Outcome

The `getScoreableCriteria()` filtering **only runs when `criterionIdentifiers` is empty**:

```go
// Line 199
if len(criteriaIDFilters) == 0 && !postgres.HasEnumValue(groupByFlag, QA_ATTRIBUTE_TYPE_CRITERION) {
    scoreableCriteria, err := a.getScoreableCriteria(ctx, ...)
    req.FilterByAttribute.CriterionIdentifiers = scoreableCriteria
}
```

If the request explicitly includes a criterion ID (even one with `excludeFromQAScores: true`), the BE **skips the scoreable criteria check** and queries CH directly with that ID.

**Example:** FE sends `criterionIdentifiers: ["019b2dde-ad0a-710a-9e12-c1d2148b84ed"]` (AHT):
1. `len(criteriaIDFilters) > 0` -> skip `getScoreableCriteria()`
2. CH query: `WHERE criterion_id = 'AHT-id' GROUP BY agent_user_id`
3. Aggregation uses AHT's raw seconds (59-2998) as `percentage_value`
4. Per-agent scores are in the hundreds/thousands (not 0-1)
5. `setQuintileRankForPerAgentScores()` still assigns Q1-Q5 — the ranking is valid as a relative ordering (percentile-based), but the absolute scores are nonsensical as "QA scores"

**Second bypass:** If the request groups by `QA_ATTRIBUTE_TYPE_CRITERION`, filtering is also skipped. This is the case for the leaderboard-by-criteria table's `[AGENT, CRITERION]` request — but that table uses the overall quintile from a separate `[AGENT]`-only request, so it doesn't affect displayed quintiles.

**In practice this isn't a problem** because:
- The FE never explicitly requests excluded outcomes for QA score purposes
- The leaderboard-per-criteria dropdown only shows scoreable criteria
- The overall quintile request sends empty `criterionIdentifiers`, which triggers the filtering

## Summary

| Aspect | Answer |
|--------|--------|
| Are outcomes stored as percentages? | **Not always** — depends on item type. AHT stores raw seconds. Dropdown items use score mapping. |
| Do outcomes participate in quintile? | **Only if `excludeFromQAScores: false`** — the flag gates inclusion at query time |
| How are included outcomes normalized? | Via the `scores` mapping in the template config (e.g., CSAT dropdown -> 0/1 score) |
| What if normalization is wrong? | No safety net — raw values would corrupt the weighted average and quintile |

## Data Source

- Template: PG `director.scorecard_template_revisions` (template_id `019b2dde-96c1-76aa-b8a5-6d0738eeaeb2`, revision `4c8f8695`)
- Score data: ClickHouse `cresta_sandbox_2_voice_sandbox_2.score_d`
- BE code: `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`
