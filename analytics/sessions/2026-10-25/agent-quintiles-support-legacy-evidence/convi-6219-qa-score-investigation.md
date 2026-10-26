# CONVI-6219: QA Score Directionality Investigation

## Problem

`RetrieveQAScoreStats` has two functions that assume higher score = better:

1. **`aggregateTopAgentsResponse()`** (line 249) — partitions agents into TOP/AVG/BOTTOM tiers. Groups by (time_range, team, group, criterion_id), so each partition group has the same criterion. Same issue as user outcome stats.

2. **`setQuintileRankForPerAgentScores()`** (line 558) — assigns quintile ranks Q1-Q5. Sorts descending by score (higher = Q1). Ranks ALL agent scores together regardless of criterion.

For TIME/CHURN criteria (e.g., AHT), lower scores mean better performance, so:
- Top agents should be those with lowest scores
- Q1 should be assigned to agents with lowest scores

## How Criteria Map to Outcome Types

The mapping chain is:
1. **Scorecard template** → parse with `scoring.ParseScorecardTemplateStructure(template.Template)`
2. **Criterion** → find metadata triggers with `criterion.GetAutoQA().Triggers` where `trigger.Type == TriggerTypeMetadata`
3. **Moment resource name** → `trigger.ResourceName` from the metadata trigger
4. **Moment template** → query `director.moment_templates` by moment ID
5. **Outcome type** → from `MomentData.ConversationOutcomeMomentData.Type` or `MomentData.ConversationMetadataMomentData.OutcomeType`
6. **Is lower better?** → `OutcomeType_TIME` or `OutcomeType_CHURN`

This is already implemented in the coaching service:
- `extractCriterionOutcomeMomentResourceNames()` (apiserver, line 640)
- `getOutcomeTypesFromMomentTemplates()` (apiserver, line 671)
- `isLowerBetterOutcomeType()` (apiserver, line 708)

## What Insights-Server Already Has

The insights-server already:
- Has access to `a.appsDB` (PostgreSQL with moment templates)
- Fetches scorecard templates via `qa.ListCurrentScorecardTemplateIDs()` in `getScoreableCriteria()` (line 691)
- Parses templates via `scoring.ParseScorecardTemplateStructure()` (line 710)
- Has `scorecard_templates` in the request's `FilterByAttribute`

## Proposed Fix

### Step 1: Create shared helper to build criterion → lower-is-better mapping

Reuse the coaching service pattern but in insights-server. Create a helper that:
1. Takes scorecard template IDs and DB connection
2. Parses templates → extracts criterion → moment resource name mapping
3. Queries moment templates → extracts outcome types
4. Returns `set.Set[string]` of criterion IDs where lower is better

This helper should live in a shared location (or in the insights analyticsimpl package) since the coaching code is in apiserver.

### Step 2: Fix `aggregateTopAgentsResponse`

Similar to the user outcome stats fix:
- Pass the lower-is-better criterion set
- For each group (which has a single criterion_id), check if it's lower-is-better
- Negate the score metric if so

### Step 3: Fix `setQuintileRankForPerAgentScores`

This is trickier because it ranks all agent scores together:
- If scores span multiple criteria with different directionalities, they can't be meaningfully ranked together
- Most likely the frontend calls this with a single criterion filter, so all scores have the same directionality
- Check the criterion from the response scores, look up directionality, and flip the sort if lower-is-better

### Step 4: Get the data

In `retrieveQAScoreStatsInternal()` (line 184), before calling the ranking functions:
1. Get scorecard template IDs from the request
2. Use the helper to build the lower-is-better criterion set
3. Pass it to `aggregateTopAgentsResponse()` and `setQuintileRankForPerAgentScores()`

## Complexity Assessment

This is significantly more complex than the user outcome stats fix because:
- The outcome type is not directly on the QA score data — it requires a multi-step lookup (scorecard template → criterion → moment → outcome type)
- The helper functions (`extractCriterionOutcomeMomentResourceNames`, `getOutcomeTypesFromMomentTemplates`) currently live in apiserver and use internal DB models — they need to be either moved to shared or reimplemented
- Additional DB queries needed at request time (moment templates lookup)

## Key Files

- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` — functions to fix
- `apiserver/internal/coaching/action_retrieve_coaching_progresses.go` — reference implementation (lines 638-716)
- `shared/scoring/` — scorecard template parsing (already shared)
- `shared/qa/` — scorecard template listing (already shared)
