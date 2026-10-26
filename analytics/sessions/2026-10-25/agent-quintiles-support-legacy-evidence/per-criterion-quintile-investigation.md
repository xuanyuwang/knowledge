# Per-Criterion Quintile Ranking — BE Investigation

**Created:** 2026-02-26

## Context

Currently, `setQuintileRankForPerAgentScores` assigns quintile ranks by ranking **all per-agent rows together as one flat pool**, regardless of what other groupBy dimensions are present. When the request groups by `[AGENT, CRITERION]`, the response contains rows like:

```
Agent1 + Criterion1 → score 0.9  → quintile ranked against ALL rows
Agent1 + Criterion2 → score 0.7
Agent2 + Criterion1 → score 0.8
Agent2 + Criterion2 → score 0.6
```

We need: **quintile rank per agent per criterion** — i.e., rank agents *within each criterion separately*.

---

## 1. Response Structure Confirmed: Flat Array with Composite GroupBy

**Confirmed** from `example_response.json` (request grouped by `[AGENT, CRITERION]`):

- `qaScoreResult.scores` is a **flat array** of `QAScore` objects.
- Each element's `groupedBy` contains **both** `user` (agent) and `criterionId` populated simultaneously.
- The `quintileRank` field exists in `groupedBy` (currently set, but ranked across ALL rows).

Example entries from real response:
```json
{
  "groupedBy": {
    "user": { "name": "customers/cresta/users/6e8716e64f09d350" },
    "criterionId": "754e4b6f-015e-402e-af11-c470e5554853",
    "quintileRank": "QUINTILE_RANK_1"   // ranked across all agent rows
  },
  "score": 1,
  "totalScorecardCount": 2
}
```

**Key finding:** The `QAScoreGroupBy` format is always a flat struct with all dimensions populated — no nesting. This means for `[AGENT, CRITERION]`, every row has both `user` and `criterionId`.

## 2. Criterion Filtering — Already Supported

Criterion filtering IS supported in the request:

```protobuf
// qa_stats.proto line 76
repeated string criterion_identifiers = 6 [(google.api.field_behavior) = OPTIONAL];
```

**Backend implementation** (`common_clickhouse.go:615-621`):
```go
if criterionsFilter := qaAttribute.CriterionIdentifiers; len(criterionsFilter) > 0 {
    con, arg := columnIn(criterionIDColumn, criterionsFilter)
    conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
}
```

This filters at the ClickHouse query level — only rows matching the specified criteria are returned. So the API already supports asking for specific criteria.

## 3. Current `setQuintileRankForPerAgentScores` — What Needs to Change

**Current code** (`retrieve_qa_score_stats.go:621-654`):

```go
func setQuintileRankForPerAgentScores(response *analyticspb.RetrieveQAScoreStatsResponse) {
    // Collects ALL rows where GroupedBy.User != nil
    var agentScores []*analyticspb.QAScore
    for _, s := range response.QaScoreResult.Scores {
        if s.GetGroupedBy().GetUser() != nil {
            agentScores = append(agentScores, s)
        }
    }
    // Sorts ALL of them together, assigns quintile across the entire pool
    sort.SliceStable(agentScores, func(i, j int) bool {
        return agentScores[i].Score > agentScores[j].Score
    })
    groupMap := utils.AssignRankGroups(n, numQuintiles, func(i int) float32 {
        return agentScores[i].Score
    })
    // Stamps quintile on each row
}
```

**Problem:** When grouped by `[AGENT, CRITERION]`, this pools Agent1+Crit1, Agent1+Crit2, Agent2+Crit1, Agent2+Crit2 all together. An agent scoring 0.9 on Criterion1 competes against another agent scoring 0.6 on Criterion2 — a meaningless comparison.

**What we need:** Group scores by `criterionId` first, then rank agents within each criterion group separately.

## 4. Proposed Solution

### Option A: Modify `setQuintileRankForPerAgentScores` to Be Criterion-Aware

When the response contains rows with `criterionId` set, partition them by criterion first:

```go
func setQuintileRankForPerAgentScores(response *analyticspb.RetrieveQAScoreStatsResponse) {
    if response == nil || response.QaScoreResult == nil {
        return
    }

    // Partition per-agent scores by criterionId.
    // Key "" means no criterion grouping (current behavior).
    byCriterion := map[string][]*analyticspb.QAScore{}
    for _, s := range response.QaScoreResult.Scores {
        if s.GetGroupedBy().GetUser() == nil {
            continue
        }
        key := s.GetGroupedBy().GetCriterionId()
        byCriterion[key] = append(byCriterion[key], s)
    }

    // Assign quintile ranks within each criterion group.
    for _, scores := range byCriterion {
        assignQuintileRanks(scores)
    }
}

func assignQuintileRanks(agentScores []*analyticspb.QAScore) {
    n := len(agentScores)
    if n == 0 {
        return
    }
    sort.SliceStable(agentScores, func(i, j int) bool {
        return agentScores[i].Score > agentScores[j].Score
    })
    numQuintiles := len(quintileRanks)
    groupMap := utils.AssignRankGroups(n, numQuintiles, func(i int) float32 {
        return agentScores[i].Score
    })
    for i, s := range agentScores {
        group := groupMap[i]
        if group < len(quintileRanks) {
            s.GroupedBy.QuintileRank = quintileRanks[group]
        }
    }
}
```

**Why this works:**
- When request groups by `[AGENT]` only → all rows have `criterionId = ""` → single group → **same behavior as today** (backward compatible).
- When request groups by `[AGENT, CRITERION]` → rows are partitioned by criterion → agents ranked within each criterion → **correct per-criterion quintile**.
- No proto changes needed — `quintileRank` field already exists in `QAScoreGroupBy`.
- No new API fields needed — the behavior is automatically determined by whether the response has `criterionId` set.

### Option B: New Function + Explicit Grouping Key (More Flexible)

A more general approach — parameterize the "sub-group key" so it could work with any additional dimension (time range, etc.):

```go
func setQuintileRankForPerAgentScoresGroupedBy(
    response *analyticspb.RetrieveQAScoreStatsResponse,
    subGroupKey func(s *analyticspb.QAScore) string,
) {
    // ... same logic but groups by subGroupKey(s) instead of criterionId
}
```

Called as:
```go
setQuintileRankForPerAgentScoresGroupedBy(result, func(s *analyticspb.QAScore) string {
    return s.GetGroupedBy().GetCriterionId()
})
```

**Verdict:** Option A is simpler and sufficient for the current requirement. Option B is over-engineering unless we know we'll need other sub-group dimensions.

## 5. Call Site — No Changes Needed

The call site at `retrieve_qa_score_stats.go:305-307` does not need to change:
```go
if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT) {
    setQuintileRankForPerAgentScores(result)  // function internally handles criterion grouping
    return appendGroupMemberships(result, users, groups, userNameToGroupNamesMap), nil
}
```

The ClickHouse path call site in `retrieve_qa_score_stats_clickhouse.go` similarly needs no change — same function is called.

## 6. Tests to Add

| Test | Description |
|------|-------------|
| `AgentCriterion_PerCriterionQuintile` | 5 agents × 2 criteria → quintile rank computed separately per criterion |
| `AgentCriterion_DifferentRankPerCriterion` | Agent is Q1 on criterion A but Q3 on criterion B |
| `AgentOnly_BackwardCompatible` | No criterion grouping → same behavior as before |
| `AgentCriterion_SingleCriterion` | Only one criterion → same as agent-only ranking |
| `AgentCriterion_TiesWithinCriterion` | Tied scores within a criterion group |

## 7. Summary

| Aspect | Current | Proposed |
|--------|---------|----------|
| Quintile pool | All per-agent rows together | Per criterion (when criterion grouped) |
| Backward compatible | N/A | Yes — no criterion = same behavior |
| Proto change | None needed | None needed |
| API field change | None needed | None needed |
| Code change | Modify `setQuintileRankForPerAgentScores` | ~20 lines changed |
| Call site change | None | None |
| Risk | Low — additive logic, existing behavior preserved for non-criterion requests |

## 8. Frontend Impact — No FE or BE Changes Needed

### Key finding: FE already achieves per-criterion quintile via filtering

The only table with a criterion dropdown is **LeaderboardPerCriterion** (3rd table on Performance page). When a criterion is selected, the FE sends:

```
groupByAttributeTypes: ["QA_ATTRIBUTE_TYPE_AGENT"]
filterByAttribute.criterionIdentifiers: ["selected-criterion-id"]
```

The criterion filter restricts the backend data to only that criterion **before** aggregation and quintile ranking. So agents are ranked based solely on their scores for the selected criterion — which IS per-criterion ranking. When the user picks a different criterion, a new request fires with the new filter, and agents get re-ranked.

**This means the current BE code (ranking all per-agent rows as one pool) already produces correct per-criterion quintiles**, because the pool only contains data for one criterion due to the filter.

The BE change (partitioning by `criterionId` in `setQuintileRankForPerAgentScores`) would only matter if the FE sent `groupBy: [AGENT, CRITERION]` **without** a criterion filter and expected independent quintiles per criterion in a single response. Currently no FE component does this.

### BE PR status

[go-servers #25968](https://github.com/cresta/go-servers/pull/25968) — Per-criterion quintile ranking. This is a correctness improvement for `[AGENT, CRITERION]` grouped requests, but **not required** by any current FE use case. Can be merged as a defensive fix or closed.

### Pages/tables — no changes needed anywhere:

| Page | Table | Has criterion dropdown? | Per-criterion quintile? |
|------|-------|------------------------|------------------------|
| Performance | Leaderboard per criteria (3rd table) | Yes | Already works — FE filters by criterion, BE ranks within filtered data |
| Performance | Leaderboard by criteria (2nd table) | No (criteria are columns) | N/A — overall agent quintile (unchanged) |
| Leaderboard | Agent Leaderboard | No | N/A — overall agent quintile (unchanged) |
| Leaderboard | Agent Leaderboard per metric | No | N/A — overall (unchanged) |
| Coaching Hub | Recent Coaching Activities | No | N/A — overall (unchanged) |
| Coaching Plan | Header badge | No | N/A — overall (unchanged) |

### Note on the `[AGENT, CRITERION]` request from LeaderboardByScorecardTemplateItem

The 2nd table on Performance sends `groupBy: [AGENT, CRITERION]` with empty `criterionIdentifiers` to fetch all criteria as columns. This request does hit the old code path where all agent rows are ranked in one pool. However, the quintile shown on that table is the **overall agent quintile** (from a separate `[AGENT]`-only request), not from the `[AGENT, CRITERION]` response. So the BE change doesn't affect this table's behavior either.
