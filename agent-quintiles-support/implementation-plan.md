# Agent Quintiles – Concrete Implementation Plan

**Created:** 2026-02-17  
**Updated:** 2026-02-23

## Quintile definition — TRUE PERCENTILE-BASED (revised)

~~Previous approach (score bands): Fixed ranges 80+→Q1, 60–79→Q2, etc. **DEPRECATED** — see below.~~

### Why score bands don't work

QA scores are **absolute** (weighted average of normalized criterion scores, 0–1). They are NOT percentiles. This means:
- If all agents score 85%+, they'd ALL be Q1 under score bands — no differentiation
- Score distributions are often skewed (many agents cluster in 70–90% range)
- The feature becomes useless for customers with consistently high-scoring agents

### New approach: percentile-based quintile (true quintile)

Rank all agents by score (descending), divide into 5 approximately equal groups:
- **Q1** = top 20% of agents (highest scores)
- **Q2** = next 20%
- **Q3** = middle 20%
- **Q4** = next 20%
- **Q5** = bottom 20% of agents (lowest scores)

**Algorithm:** For N agents sorted descending by score, distribute evenly: each quintile gets `floor(N/5)` agents, with the first `N % 5` quintiles getting one extra. This ensures consecutive quintile filling (no gaps) and approximately equal group sizes.

**Scope:** Quintile is only applied when the response is grouped by agents (`QA_ATTRIBUTE_TYPE_AGENT`). All per-agent scores in the response are ranked together as one flat group.

**Edge cases:**
- N < 5 agents: first N quintiles get 1 agent each, remaining quintiles are empty.
  - Example: 3 agents with scores [90, 70, 50] → Q1=90, Q2=70, Q3=50, Q4=empty, Q5=empty.
  - Example: 1 agent with score [85] → Q1=85, Q2–Q5=empty.
- N = 0: nothing to do
- Ties: agents with the same score at a quintile boundary are moved to the higher (better) quintile. This means the higher quintile grows and the lower quintile shrinks (possibly to 0 members). Implemented via `AssignRankGroups` in `shared/utils/rank.go`.
  - Example: 10 agents, scores [90, 80, 80, 70, 60, 50, 40, 30, 20, 10]. Naive split puts boundary between index 1 and 2, but both have score 80. Tie adjustment moves all 80s into Q1. Result: Q1=[90,80,80], Q2=[70], Q3=[60,50], Q4=[40,30], Q5=[20,10].
  - Example: 5 agents, all score 75. Every boundary is a tie, so all cascade into Q1. Result: Q1=[75,75,75,75,75], Q2–Q5=empty.
  - Example: 10 agents, scores [90, 90, 80, 80, 70, 70, 60, 60, 50, 50]. Tied pairs fall entirely within groups — no tie spans a boundary. Result: Q1=[90,90], Q2=[80,80], Q3=[70,70], Q4=[60,60], Q5=[50,50] (same as no-tie case).

**Why not reuse `PartitionUsingVolumeAndMetric`?**

The existing utility (used by agent tiers) doesn't fit quintile requirements:
1. **Volume-weighted cutoffs**: Splits by cumulative conversation/scorecard volume at percentage cutoffs (bottom 25% of volume, not bottom 25% of agents). Quintiles need equal-count partitioning — each group gets ~N/5 agents regardless of how many conversations each agent has.
2. **Guarantees non-empty partitions**: Uses `ensureNonEmptySliceIndices` to force every partition to have ≥1 element. Quintiles should allow empty groups when N < 5.
3. **No tie awareness**: Doesn't detect ties at boundaries. `AssignRankGroups` detects when elements on both sides of a boundary share the same value and moves the boundary so the entire tie block stays in the higher-ranked group.
4. **Returns slices vs. index map**: Returns `[][]T` (mutates and reslices the input). `AssignRankGroups` returns `map[int]int` (index → group), which works better for stamping a rank onto existing proto objects.

**Floating-point tie detection:** Uses exact `==` equality. Floating-point rounding may cause near-identical values to be treated as distinct. In practice this is not an issue because QA scores come from the same ClickHouse computation pipeline and tied agents have identical float32 representations.

---

## Phase 1: Backend (go-servers + cresta-proto)

### 1.1 Proto change (cresta-proto)

**File:** `cresta-proto/cresta/v1/analytics/qa_stats.proto`

- Add `enum QuintileRank` with values `QUINTILE_RANK_UNSPECIFIED = 0`, `QUINTILE_RANK_1 = 1` through `QUINTILE_RANK_5 = 5`.
- In `message QAScoreGroupBy`, add:
  - `QuintileRank quintile_rank = 7 [(google.api.field_behavior) = OUTPUT_ONLY];`
- Regenerate Go (and any other languages) for the analytics package.

**Enum values** (unchanged — enum represents rank, not score band):
| Value | Number | Meaning |
|-------|--------|---------|
| QUINTILE_RANK_UNSPECIFIED | 0 | Not computed |
| QUINTILE_RANK_1 | 1 | Top 20% (best) |
| QUINTILE_RANK_2 | 2 | 60th–80th percentile |
| QUINTILE_RANK_3 | 3 | 40th–60th percentile |
| QUINTILE_RANK_4 | 4 | 20th–40th percentile |
| QUINTILE_RANK_5 | 5 | Bottom 20% (lowest) |

### 1.2 Backend: percentile-based quintile assignment (go-servers) ✅

**File:** `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

**`setQuintileRankForPerAgentScores`:**
1. Collect all per-agent scores (where `GroupedBy.User != nil`)
2. Sort descending by score (stable sort for deterministic ordering)
3. Call `utils.AssignRankGroups(n, 5, valueFn)` — distributes into 5 quintiles with tie-aware boundaries
4. Map group number (0–4) → `QuintileRank` enum (Q1–Q5)

**`AssignRankGroups`** (generic utility in `shared/utils/rank.go`): distributes N pre-sorted elements into K groups. Base sizes are `floor(N/K)` with first `N%K` groups getting one extra. Tie adjustment: when elements on both sides of a boundary share the same value, the boundary moves forward so the entire tie block stays in the higher-ranked group. Groups can become empty if ties cascade.

`ScoreToQuintileRank` (fixed score bands) removed — quintile is computed from rank position, not individual score value.

### 1.3 Call sites (unchanged)

- **retrieve_qa_score_stats.go:** `setQuintileRankForPerAgentScores(result)` in the `QA_ATTRIBUTE_TYPE_AGENT` path
- **retrieve_qa_score_stats_clickhouse.go:** `setQuintileRankForPerAgentScores(resp)` in `convertCHResponseToQaScoreStatsResponse`

### 1.4 Tests (go-servers) ✅

- `PercentileBased_10Agents`: 10 agents → top 2 Q1, next 2 Q2, etc.
- `SmallGroup_3Agents`: 3 agents → Q1, Q2, Q3 (no gaps)
- `UnevenDistribution_7Agents`: 7 agents → Q1/Q2 get 2, Q3–Q5 get 1
- `NonAgentRowsUntouched`: criterion-only rows stay UNSPECIFIED
- `AllAgentsSameScore`: tied agents all go to Q1 (tie-aware)
- `SingleAgent`: 1 agent → Q1
- `NilSafety`: nil/empty inputs don't panic
- `TestConvertCHResponseSetsQuintileRank`: ClickHouse path percentile-based
- `TestConvertCHResponseNoQuintileForNonAgentRows`: criterion rows stay UNSPECIFIED
- `TestAggregateTopAgentsResponse_NoQuintileRankLeakage`: tier path doesn't leak quintile

### 1.5 BE checklist

| Step | Task | Status |
|------|------|--------|
| 1.1 | Proto: `QuintileRank` enum + `quintile_rank` field | ✅ Merged (#7874) |
| 1.2 | Percentile-based `setQuintileRankForPerAgentScores` | ✅ Done |
| 1.3 | Call sites in Postgres + ClickHouse paths | ✅ Already wired |
| 1.4 | Tests for percentile-based logic | ✅ Done (10 tests) |

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
