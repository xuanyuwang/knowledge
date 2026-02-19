# Agent Quintiles – Concrete Implementation Plan

**Created:** 2026-02-17  
**Updated:** 2026-02-19

## Quintile definition (score bands, 0–100 scale)

| Quintile | Rank | Score range (inclusive) | Note |
|----------|------|--------------------------|------|
| 1 (top)  | 1    | 80 and above             | Best performers |
| 2        | 2    | 60 – 79                  | |
| 3        | 3    | 40 – 59                  | |
| 4        | 4    | 20 – 39                  | |
| 5 (bottom) | 5  | 19 and below             | Lowest performers |

**Backend note:** The API stores `score` as **0–1** (see proto: "number between 0 and 1"). When computing quintile in BE, use the same bands on a 0–1 scale: 0.8, 0.6, 0.4, 0.2 (e.g. score >= 0.8 → quintile 1; 0.6 <= score < 0.8 → 2; etc.).

---

## Phase 1: Backend (go-servers + cresta-proto)

### 1.1 Proto change (cresta-proto)

**File:** `cresta-proto/cresta/v1/analytics/qa_stats.proto`

- Add `enum QuintileRank` with values `QUINTILE_RANK_UNSPECIFIED = 0`, `QUINTILE_RANK_1 = 1` through `QUINTILE_RANK_5 = 5`.
- In `message QAScoreGroupBy`, add:
  - `QuintileRank quintile_rank = 7 [(google.api.field_behavior) = OUTPUT_ONLY];`
- Regenerate Go (and any other languages) for the analytics package.

**Enum values:**
| Value | Number | Score range |
|-------|--------|-------------|
| QUINTILE_RANK_UNSPECIFIED | 0 | Not computed |
| QUINTILE_RANK_1 | 1 | 80+ (best) |
| QUINTILE_RANK_2 | 2 | 60–79 |
| QUINTILE_RANK_3 | 3 | 40–59 |
| QUINTILE_RANK_4 | 4 | 20–39 |
| QUINTILE_RANK_5 | 5 | 0–19 (lowest) |

### 1.2 Backend: quintile from score (go-servers)

**Logic:** Pure function: given a score (float32, 0–1), return `QuintileRank` enum value.

- **File (new or in existing util):** e.g. `insights-server/internal/analyticsimpl/quintile.go` or next to `retrieve_qa_score_stats.go`.

**Function signature and logic:**

```go
// ScoreToQuintileRank maps a QA score (0-1) to QuintileRank enum per product bands:
// QUINTILE_RANK_1: 80+ (0.8+), QUINTILE_RANK_2: 60-79 (0.6-0.8),
// QUINTILE_RANK_3: 40-59 (0.4-0.6), QUINTILE_RANK_4: 20-39 (0.2-0.4),
// QUINTILE_RANK_5: 0-19 (<0.2).
func ScoreToQuintileRank(score float32) analyticspb.QuintileRank {
    switch {
    case score >= 0.8:  return analyticspb.QuintileRank_QUINTILE_RANK_1
    case score >= 0.6:  return analyticspb.QuintileRank_QUINTILE_RANK_2
    case score >= 0.4:  return analyticspb.QuintileRank_QUINTILE_RANK_3
    case score >= 0.2:  return analyticspb.QuintileRank_QUINTILE_RANK_4
    default:            return analyticspb.QuintileRank_QUINTILE_RANK_5
    }
}
```

- Handle NaN/negative by treating as bottom band (QUINTILE_RANK_5) if desired, or document that callers only pass valid 0–1 scores.
- Add unit tests for boundaries: 0, 0.19, 0.2, 0.39, 0.4, 0.59, 0.6, 0.79, 0.8, 1.0.

### 1.3 Populate quintile_rank when returning per-agent scores

**Where:** Any code path that builds a `RetrieveQAScoreStatsResponse` with **per-agent** scores (group by AGENT). Each `QAScore` in the response has `GroupedBy` and `Score`; after the score is final, set `GroupedBy.QuintileRank = ScoreToQuintileRank(score.Score)`.

**Files to touch:**

1. **`insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`**
   - After building per-user (per-agent) scores and before returning:
     - When appending to the response scores (e.g. in the path that returns `perUserResp` or equivalent), for each score that has `GroupedBy.User` set, set `GroupedBy.QuintileRank = ScoreToQuintileRank(score.Score)`.
   - Likely locations (to be confirmed by grep):
     - Where `perUserResp` is built and returned (e.g. after `retrieveQAScoreStatsPerUser...`).
     - In `appendGroupMemberships` if it copies scores (then set quintile on each score’s GroupedBy).
     - In `convertRowsPerUserToPerGroupQAScoreStatsResponse` we are converting to per-group; per-agent response is the one we need to tag. So the main place is: **before** any aggregation that drops per-agent granularity, iterate over `perUserResp.QaScoreResult.Scores` and set `score.GroupedBy.QuintileRank = ScoreToQuintileRank(score.Score)`.
   - Ensure we only set quintile when `GroupedBy.User != nil` (per-agent row).

2. **`insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`**
   - If the ClickHouse path also returns per-agent scores (scores with `GroupedBy.User`), after building each score set `GroupedBy.QuintileRank = ScoreToQuintileRank(score.Score)`.
   - Search for where `QAScore` or `QAScoreGroupBy` is filled and add the same assignment.

**Edge cases:**

- **NA / no score:** If an agent has no score or a special “NA” sentinel, either leave `quintile_rank` unset (0) or set to 0 and let FE treat 0 as “N/A”. Prefer 0 = not set for clarity.
- **Grouped by time/criterion:** When the same agent appears in multiple rows (e.g. per time range or per criterion), each row has its own score; compute quintile per row from that row’s score. No cross-row aggregation.

**Concrete insertion points:**
- **retrieve_qa_score_stats.go:** Add `func setQuintileRankForPerAgentScores(response *analyticspb.RetrieveQAScoreStatsResponse)` that sets `score.GroupedBy.QuintileRank = ScoreToQuintileRank(score.Score)` for each score with `GroupedBy.User != nil`. In `retrieveQAScoreStatsInternal`, in the block `if postgres.HasEnumValue(groupByFlag, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT)`, call `setQuintileRankForPerAgentScores(result)` then `return appendGroupMemberships(result, ...)`.
- **retrieve_qa_score_stats_clickhouse.go:** In `convertCHResponseToQaScoreStatsResponse`, inside the loop over `rows`, after computing `groupedBy` and `scoreVal`, if `groupedBy.User != nil` set `groupedBy.QuintileRank = ScoreToQuintileRank(scoreVal)` before appending the score to `scores`.

### 1.4 Tests (go-servers)

- **Unit tests for `ScoreToQuintileRank`:** All boundaries above; and one or two negative/NaN if we define behavior.
- **Integration / table-driven test:** In `retrieve_qa_score_stats_test.go` (or equivalent), add a case that requests per-agent stats and asserts that returned scores have `quintile_rank` set and that it matches the band for the score (e.g. score 0.85 → quintile 1, 0.5 → 3).

### 1.5 BE checklist (summary)

| Step | Task | Owner |
|------|------|--------|
| 1.1 | Add `QuintileRank` enum + `quintile_rank` field to `QAScoreGroupBy` in cresta-proto; regenerate | BE |
| 1.2 | Implement `ScoreToQuintileRank(score float32) analyticspb.QuintileRank` + unit tests | BE |
| 1.3 | In retrieve_qa_score_stats.go, set `GroupedBy.QuintileRank` for every per-agent score before return | BE |
| 1.4 | Same for retrieve_qa_score_stats_clickhouse.go if it returns per-agent scores | BE |
| 1.5 | Add test(s) that per-agent response has correct quintile_rank for given scores | BE |

---

## Phase 2: Frontend (director)

Full requirements in `requirements.md`. Detailed investigation in `fe-investigation.md`.

### 2.0 Type layer ✅
- `apiTypes.ts`: Added `QuintileRank` import + `quintileRank?: QuintileRank` to `QAScoreGroupBy`
- `transformersQAI.ts`: Mapped `quintileRank: groupedBy?.quintileRank`
- `LeaderboardRow` in `types.ts`: Added `quintileRank?: QuintileRank`
- `useVisibleColumnsForLeaderboards.tsx`: Added `'quintileRank'` to `alwaysVisible`

### 2.1 Shared QuintileRankIcon component (not started)
- New component: gold/silver/bronze icon for Q1/Q2/Q3; no icon for Q4/Q5
- Optional tooltip prop (for Coaching Hub: "Xth quintile based on last 7 days")
- Location: `components/insights/shared/QuintileRankIcon.tsx` or `components/qa/shared/QuintileRankIcon.tsx`

### 2.2 Agent Leaderboard (partially done — needs corrections)

**Column:**
- ✅ Row assignment: `row.quintileRank = groupResult.groupedBy?.quintileRank`
- ⚠️ **Position wrong**: Currently after Performance group → move to **after Live Assist group** (before Outcome Metrics)
- ⚠️ **Display wrong**: Currently "Q1"–"Q5" → change to plain number **1–5**

**Icon on name:**
- Not started. Add `QuintileRankIcon` inline in the Name column cell and in the tooltip.

### 2.3 Agent Leaderboard per metric (not started)
- Add `quintileRank?: QuintileRank` to `LeaderboardByMetricTableData` in `types.ts`
- Thread `quintileRank` from QA score data into rows via `useLeaderboardByMetricData`
- Add `QuintileRankIcon` inline in the Name column cell (`AgentLeaderboardByMetric.tsx`)

### 2.4 Performance → Leaderboard by criteria — 2nd table (not started)

**Component:** `LeaderboardByScorecardTemplateItem.tsx`

- Add `quintileRank?: QuintileRank` to `LeaderboardByScorecardTemplateItemRow` (in local `types.ts`)
- Extract `quintileRank` from `groupedBy` in `createAllRows()` (`useLeaderboardByScorecardTemplateData.tsx`)
- Add "Quintile Rank" column inside sticky group, after "Average Performance" (~120px). Update `stickyHeadersWidth`.
- Add `QuintileRankIcon` inline in the Name column cell + tooltip

### 2.5 Performance → Leaderboard per criteria — 3rd table (not started)

**Component:** `LeaderboardPerCriterion.tsx` + `useLeaderboardPerCriterionColumns.tsx`

- Add `quintileRank?: QuintileRank` to `LeaderboardPerCriterionRow`
- Extract `quintileRank` from `groupedBy` when building rows (~lines 177–204)
- Add "Quintile Rank" column as last `static: true` column (after Average). Uses `GridTable` (not `DirectorTable`).
- Add `QuintileRankIcon` inline in the Name column cell + tooltip

### 2.6 Coaching Hub → Recent Coaching Activities (not started)

**Component:** `RecentCoachingActivities.tsx` → `AgentDetailsCell.tsx`

- Thread agent-level quintile into `AgentCoachingOverviewWithCriteriaInfo` or pass separately. The component already fetches QA stats into `qaStatsMap` — can extract per-agent quintile from there.
- Add `QuintileRankIcon` in `AgentDetailsCell.tsx` next to agent name
- Tooltip: "Xth quintile based on last 7 days"
- **Note:** Tooltip implies quintile is always based on last 7 days regardless of date filter — verify with product.

### 2.7 Coaching Plan page
- TBD per requirements

### 2.8 Feature flag (not started — can be done in parallel with other FE work)

Guard all quintile UI behind a feature flag. When off, no quintile column or icon appears anywhere.

**Step 1: Add flag in config repo** (`~/repos/config`)

**File:** `src/CustomerConfig.ts` — add to the `featureFlags` object (alphabetically sorted):

```typescript
/**
 * Enables quintile rank column and icons on agent names in Performance, Leaderboard, and Coaching Hub pages.
 * @see Director, Insights
 */
readonly enableQuintileRank?: boolean;
```

Then regenerate schemas:
```bash
yarn gen:all
```

Or use the helper script:
```bash
yarn add-director-flag "enableQuintileRank" "Enables quintile rank column and icons on agent names in Performance, Leaderboard, and Coaching Hub pages." "Director, Insights"
```

**Files changed in config repo:**
1. `src/CustomerConfig.ts` — add flag definition (manual)
2. `json-schema/CustomerPublicConfigYaml.json` — auto-generated
3. `json-schema/V3FrontendConfig.json` — auto-generated
4. `json-schema/V3FrontendConfigYaml.json` — auto-generated
5. `json-schema/HermesCustomerConfigYaml.json` — auto-generated
6. `json-schema/WalterCustomerConfigYaml.json` — auto-generated

**Step 2: Regenerate types in director**

After the config PR lands and the JSON schema is published to CDN:
```bash
cd packages/director-app && yarn generate:feature-flags-types
```

This updates `packages/director-app/src/types/frontendFeatureFlags.ts` with the new `enableQuintileRank` in the `SchemaFeatureFlag` union type.

**Alternative (for immediate development):** Add to `localFeatureFlags.ts` as a `LocalFeatureFlag` first, then migrate to schema-based after config PR lands.

**Step 3: Use the flag in director**

In each component that renders quintile UI, read the flag and gate rendering:

```typescript
const enableQuintileRank = useFeatureFlag('enableQuintileRank');

// Gate column visibility
if (enableQuintileRank && visibleColumns.has('quintileRank')) {
  // ... add quintile column
}

// Gate icon rendering
{enableQuintileRank && <QuintileRankIcon quintileRank={row.quintileRank} />}
```

**Components to gate:**
- `AgentLeaderboard.tsx` — column + icon on name
- `AgentLeaderboardByMetric.tsx` — icon on name
- `LeaderboardByScorecardTemplateItem.tsx` — column + icon on name
- `LeaderboardPerCriterion.tsx` / `useLeaderboardPerCriterionColumns.tsx` — column + icon on name
- `AgentDetailsCell.tsx` (Coaching Hub) — icon on name
- `useVisibleColumnsForLeaderboards.tsx` — conditionally add `'quintileRank'` to `alwaysVisible`

---

## Worktrees

- **Proto:** Worktree already created: `../cresta-proto-quintiles` (branch `feature/agent-quintiles-proto`). Work in that directory for Phase 1.1.
- **BE:** `git worktree add ../go-servers-quintiles -b feature/agent-quintiles` from go-servers root.
- **FE:** `git worktree add ../director-quintiles -b feature/agent-quintiles` from director root.

---

## References

- Proto: `cresta-proto/cresta/v1/analytics/qa_stats.proto` – `QAScore.score` (0–1), `QAScoreGroupBy`.
- BE: `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`, `retrieve_qa_score_stats_clickhouse.go`.
- Investigation: `agent-quintiles-support/investigation.md`, `README.md`.
