# FE Investigation: Quintile Rank in Director

**Created:** 2026-02-18
**Updated:** 2026-02-19

## Overview

The BE now returns `quintile_rank` (enum `QuintileRank`, values 0–5) on `QAScoreGroupBy` for per-agent `RetrieveQAScoreStats` responses. The FE needs to:

1. Pick up the new proto type
2. Thread it through the transformer and internal types
3. Display it on Performance, Leaderboard, and Coaching Hub pages

## Current State

- `@cresta/web-client@2.0.534` includes `QuintileRank` enum and `quintileRank` field on `QAScoreGroupBy` (verified).
- **Partial implementation done** in `director-quintiles`: type layer + Agent Leaderboard column (but needs corrections per requirements).

## Data Flow

```
@cresta/web-client (proto generated types)
  → transformersQAI.ts: transformQAScoreGroupBy()
    → apiTypes.ts: QAScoreGroupBy interface
      → hooks (useQAScoreStats, etc.)
        → components (AgentLeaderboard, LeaderboardByScorecardTemplateItem, etc.)
```

## Requirements Summary (from requirements.md)

### Display elements

1. **Quintile Rank column**: Shows number 1–5 (NOT "Q1"–"Q5"). Appears on:
   - Performance → Leaderboard by criteria (2nd table) → Agent tab: after "Average Performance", sticky
   - Performance → Leaderboard per criteria (3rd table) → Agent tab: last sticky column
   - Leaderboard → Agent Leaderboard: after "Live Assist" column group

2. **Quintile icons on agent names**: Gold (Q1), Silver (Q2), Bronze (Q3). No icon for Q4/Q5. Appears on:
   - Performance → Leaderboard by criteria → Agent tab: next to agent name, also in name tooltip
   - Performance → Leaderboard per criteria → Agent tab: same
   - Leaderboard → Agent Leaderboard: next to agent name
   - Leaderboard → Agent Leaderboard per metric: next to agent name
   - Coaching Hub → Recent Coaching Activities: next to agent name, with tooltip "Xth quintile based on last 7 days"

### Coaching Plan page
- TBD

---

## Gap Analysis: Current Implementation vs Requirements

### Done (in director-quintiles)
- [x] `apiTypes.ts`: `quintileRank?: QuintileRank` on `QAScoreGroupBy`
- [x] `transformersQAI.ts`: passthrough `quintileRank: groupedBy?.quintileRank`
- [x] `types.ts`: `quintileRank?: QuintileRank` on `LeaderboardRow`
- [x] `useVisibleColumnsForLeaderboards.tsx`: `'quintileRank'` in `alwaysVisible`
- [x] `AgentLeaderboard.tsx`: `row.quintileRank` assignment + column definition

### Needs correction
- [ ] **Agent Leaderboard column position**: Currently after Performance group → should be after Live Assist group
- [ ] **Column cell display**: Currently shows "Q1"–"Q5" → should show plain number 1–5

### Not yet implemented
- [ ] **Shared `QuintileRankIcon` component**: Gold/silver/bronze icon for Q1/Q2/Q3; no icon for Q4/Q5
- [ ] **Agent Leaderboard name column**: Add icon next to name
- [ ] **Agent Leaderboard per metric name column**: Add icon next to name (row type `LeaderboardByMetricTableData` needs `quintileRank`)
- [ ] **Performance → Leaderboard by criteria (2nd table)**: Add quintile column + icon on name (row type `LeaderboardByScorecardTemplateItemRow` needs `quintileRank`)
- [ ] **Performance → Leaderboard per criteria (3rd table)**: Add quintile column + icon on name (row type `LeaderboardPerCriterionRow` needs `quintileRank`)
- [ ] **Coaching Hub → Recent Coaching Activities**: Add icon on agent name with tooltip (row type `AgentCoachingOverviewWithCriteriaInfo` needs agent-level quintile)

---

## Detailed FE Investigation by Page

### 1. Performance Page

**File:** `director-app/src/components/insights/qa-insights/performance/Performance.tsx`

Renders three tables:
- `PerformanceProgression` – groups by CRITERION + TIME_RANGE, not AGENT → no quintile here
- `LeaderboardByScorecardTemplateItem` – **needs quintile** (2nd table)
- `LeaderboardPerCriterion` – **needs quintile** (3rd table)

#### 1a. Leaderboard by criteria (2nd table)

**Component:** `packages/director-app/src/components/insights/qa-insights/leaderboard-by-scorecard-template-item/LeaderboardByScorecardTemplateItem.tsx`

**Data hook:** `useLeaderboardByScorecardTemplateData.tsx`

**Row type:** `LeaderboardByScorecardTemplateItemRow` (in local `types.ts` ~lines 46–62)
```typescript
export interface LeaderboardByScorecardTemplateItemRow {
  icon?: string;
  username?: string;
  userDisplayName?: string;
  teamDisplayName?: string;
  teamResourceName?: string;
  userResourceName?: string;
  userObject?: User;
  aggregatedRowData?: Partial<Cell>;
  columns: ScorecardItemIdentifierToCell;
}
```
→ **Need to add:** `quintileRank?: QuintileRank`

**Column structure (Agent tab):**
- Sticky group (~line 152–155): Icon (60px) + Name (150px) + Team (150px) + Total (80px) + Average (120px) = 560px total
  - `meta: { sticky: true, fixedWidth: ... }`
- The "Average Performance" column IS inside the sticky group
- After sticky group: dynamic criterion columns from `useColumnsFromScorecardTemplate`

**Agent name rendering (~lines 165–192):**
- Accessor: `'userDisplayName'`, header: `'Name'`
- Renders as `<Link>` to agent overview with tooltip "Show agent overview for {name} ({username})"
- No existing icon slot on name
- **Need to add:** quintile icon inline with name, and in the tooltip

**Where to insert "Quintile Rank" column:**
- Per requirements: right after "Average Performance" column, also sticky
- Average Performance is the last column in the sticky group
- **Approach:** Add Quintile Rank column inside the sticky group, after Average Performance, or as a new sticky column right after the group. Update `stickyHeadersWidth` accordingly.

**Data source for quintile:**
- Rows are built from `qaScoreResult.scores` via `createAllRows()`
- Each score has `groupedBy.quintileRank` (after BE change)
- Need to extract `quintileRank` from the score and store it on the row in `createAllRows()`

#### 1b. Leaderboard per criteria (3rd table)

**Component:** `packages/director-app/src/components/insights/qa-insights/leaderboard-per-criterion/LeaderboardPerCriterion.tsx`

**Column hook:** `useLeaderboardPerCriterionColumns.tsx`

**Row type:** `LeaderboardPerCriterionRow` (in `useLeaderboardPerCriterionColumns.tsx` ~lines 41–59)
```typescript
export type LeaderboardPerCriterionRow = {
  resourceName: string;
  name: string;
  team: string;
  username?: string;
  teamResourceName?: string;
  columns: Record<DateString, Cell | undefined>;
  agentObj?: User;
  teamObj?: Team;
  groupObj?: Group;
  aggregatedData?: Partial<Cell>;
  criterionTemplate?: ScorecardCriterionTemplateBaseWithValue;
};
```
→ **Need to add:** `quintileRank?: QuintileRank`

**Agent tab columns (lines 117–241):**
- Avatar (50px, static) — `ProfileAvatarWithLoginStatus`
- Name (188px, static) — link with tooltip, `useTooltip: true`
- Team (128px, static)
- Count (128px, static)
- Average (128px, static)
- Then dynamic date columns

**Note:** This table uses `GridTable` (not `DirectorTable`). Columns have `static: true` property. Sticky behavior may differ from `DirectorTable`.

**Where to insert "Quintile Rank" column:**
- Per requirements: "last sticky column"
- Currently no explicit sticky meta; columns use `static: true`
- **Approach:** Add Quintile Rank after Average, with `static: true`

**Agent name rendering:**
- Same pattern as above: `ProfileAvatarWithLoginStatus` + link
- **Need to add:** quintile icon inline

**Data source for quintile:**
- Rows built from `qaScoreStats.data.qaScoreResult.scores` (~lines 177–204)
- Each score has `groupedBy.quintileRank`
- Need to extract and store on row

### 2. Leaderboard Page

#### 2a. Agent Leaderboard

**Component:** `packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboard.tsx`

**Column order (lines 473–540):**
1. Headerless sticky group (Icon + Name + Team) = 360px
2. Conversation Volume group
3. Performance group (adherence)
4. **Quintile Rank column** ← currently here (WRONG)
5. AHT group
6. Assistance group
7. Engagement group
8. **Live Assist group** ← should be BEFORE quintile
9. Outcome Metrics group

**Required position:** After Live Assist (step 8), before Outcome Metrics (step 9).

**Agent name column (lines 427–459):**
- Sticky, 150px
- Renders `<Link>` with `<SharedTooltip>` ("Show agent overview for {{name}}")
- **Need to add:** quintile icon inline, also show icon in tooltip

**Cell display correction:**
- Currently: `Q${QuintileRankNumber[value]}` → "Q1"–"Q5"
- Required: plain number `1`–`5`
- Also needs to use `QuintileRankNumber[value]` but without the "Q" prefix

#### 2b. Agent Leaderboard per metric

**Component:** `packages/director-app/src/features/insights/leaderboard/leaderboard-by-metric/agent-leaderboard-by-metric/AgentLeaderboardByMetric.tsx`

**Row type:** `LeaderboardByMetricTableData` (in `types.ts` lines 266–283)
```typescript
export interface LeaderboardByMetricTableData {
  resourceName: string;
  name: string;
  team: string;
  username?: string;
  teamResourceName?: string;
  metricAggregated?: number | undefined;
  columns: Record<DateString, number | undefined>;
}
```
→ **Need to add:** `quintileRank?: QuintileRank`

**Agent name rendering (lines 130–162):**
- Avatar (50px, static) — `ProfileAvatarWithLoginStatus`
- Name (188px, static) — link with tooltip
- **Need to add:** quintile icon inline

**Data source for quintile:**
- Uses `useLeaderboardByMetricData` hook
- Need to thread `quintileRank` from the QA score data into `LeaderboardByMetricTableData` rows

### 3. Coaching Hub

#### 3a. Recent Coaching Activities

**Container:** `packages/director-app/src/features/coaching-workflow/coaching-hub/recent-coaching-activities/RecentCoachingActivities.tsx`

**Table:** `packages/director-app/src/components/coaching-hub/tables/recent-activities/RecentActivitiesTable.tsx`

**Agent name cell:** `packages/director-app/src/components/coaching-hub/tables/shared-cells/AgentDetailsCell.tsx`
- Shows `ProfileAvatarWithLoginStatus` + agent name as clickable button
- Has `SharedMenu` with hover interactions for actions
- Cell receives full `AgentCoachingOverviewWithCriteriaInfo` row

**Row type:** `AgentCoachingOverviewWithCriteriaInfo` (in `types/coaching/agentOverview.ts`)
```typescript
export interface AgentCoachingOverviewWithCriteriaInfo extends AgentCoachingOverview {
  criteriaInfo: (CriterionInfoWithQAScore | CriterionInfoWithTargetProgress)[];
  unfilteredCriteriaInfo?: (CriterionInfoWithQAScore | CriterionInfoWithTargetProgress)[];
  coachingOpportunities?: CriteriaOpportunity[];
}
```

**Key issue:** QA scores are per-criterion in `criteriaInfo`, not at agent level. Need agent-level quintile. Options:
1. Add `quintileRank` to `AgentCoachingOverviewWithCriteriaInfo` and populate from a separate QA stats call
2. The `RecentCoachingActivities` component already fetches QA stats (lines 256–279) into `qaStatsMap` — can extract per-agent quintile from there

**Tooltip requirement:** "Xth quintile based on last 7 days" — implies the quintile shown in Coaching Hub is always based on the last 7 days, regardless of the date filter.

**Where to add icon:**
- In `AgentDetailsCell.tsx`, next to agent name (after the name text, before/after the button)
- Wrap icon in `SharedTooltip` with "Xth quintile based on last 7 days"

---

## Shared Component: QuintileRankIcon

**Purpose:** Small icon displayed next to agent names. Three colors only:
- **Gold** → Q1 (80+)
- **Silver** → Q2 (60–79)
- **Bronze** → Q3 (40–59)
- **No icon** → Q4, Q5

**Props:**
```typescript
interface QuintileRankIconProps {
  quintileRank?: QuintileRank;
  tooltip?: string; // Optional custom tooltip (for Coaching Hub "Xth quintile based on last 7 days")
}
```

**Existing patterns to follow:**
- `QAScoreBadge` (`components/qa/shared/QAScoreBadge.tsx`) — Paper wrapper with colored text
- `BadgeCell` (`components/coaching-hub/tables/shared-cells/BadgeCell.tsx`) — Badge with `bgColor`
- `ProfileAvatarWithLoginStatus` — small inline component next to names

**Suggested location:** `packages/director-app/src/components/insights/shared/QuintileRankIcon.tsx` or `packages/director-app/src/components/qa/shared/QuintileRankIcon.tsx`

---

## Feature Flag

All quintile UI must be gated behind a feature flag `enableQuintileRank`.

### How feature flags work

1. **Config repo** (`~/repos/config`): Flag defined in `src/CustomerConfig.ts` under `featureFlags` object (alphabetically sorted). Run `yarn gen:all` to regenerate JSON schemas. Helper: `yarn add-director-flag "flagName" "description" "services"`.
2. **Director**: Flag consumed via `useFeatureFlag('enableQuintileRank')` hook (returns boolean). Hook reads from: localStorage override → URL query param → customer config (from API).
3. **Type safety**: Flag name must exist in `SchemaFeatureFlag` (auto-generated from JSON schema) or `LocalFeatureFlag` (manual) union types in `packages/director-app/src/types/`.

### Where to gate

The flag should wrap:
- `useVisibleColumnsForLeaderboards.tsx` — conditionally include `'quintileRank'` in `alwaysVisible`
- All quintile column definitions (Agent LB, Performance tables)
- All `QuintileRankIcon` renderings in name columns
- Coaching Hub icon rendering

### Development workflow

For immediate development, add `enableQuintileRank` to `localFeatureFlags.ts` as a `LocalFeatureFlag`. After the config repo PR lands and schema is published, regenerate types (`yarn generate:feature-flags-types`) and remove from `localFeatureFlags.ts`.

### Key files

| File | Purpose |
|------|---------|
| `~/repos/config/src/CustomerConfig.ts` | Define the flag |
| `director-app/src/hooks/useFeatureFlag.ts` | Hook definition |
| `director-app/src/types/frontendFeatureFlags.ts` | Auto-generated `SchemaFeatureFlag` union type |
| `director-app/src/types/localFeatureFlags.ts` | Manual `LocalFeatureFlag` union type (for dev) |
| `director-app/scripts/generate-feature-flags-types.mjs` | Regeneration script |

---

## Summary of All Changes Needed

### Already done (needs corrections)
| File | Current State | Correction Needed |
|------|--------------|-------------------|
| `apiTypes.ts` | ✅ `quintileRank` field | — |
| `transformersQAI.ts` | ✅ passthrough | — |
| `LeaderboardRow` in `types.ts` | ✅ field added | — |
| `useVisibleColumnsForLeaderboards.tsx` | ✅ in `alwaysVisible` | — |
| `AgentLeaderboard.tsx` row assignment | ✅ `row.quintileRank` | — |
| `AgentLeaderboard.tsx` column position | After Performance | Move to after Live Assist |
| `AgentLeaderboard.tsx` cell display | "Q1"–"Q5" | Plain number 1–5 |

### New work needed

| Area | File(s) | Change |
|------|---------|--------|
| Shared icon | New `QuintileRankIcon.tsx` | Gold/silver/bronze icon for Q1/Q2/Q3 |
| Agent LB name | `AgentLeaderboard.tsx` name column | Add icon inline + in tooltip |
| Agent LB by metric | `AgentLeaderboardByMetric.tsx` | Add `quintileRank` to row type + icon on name |
| LB by metric types | `types.ts` → `LeaderboardByMetricTableData` | Add `quintileRank` field |
| Perf: by criteria | `LeaderboardByScorecardTemplateItem.tsx` | Add `quintileRank` to row type, add column (sticky, after Avg Perf), add icon on name |
| Perf: per criteria | `LeaderboardPerCriterion.tsx` + `useLeaderboardPerCriterionColumns.tsx` | Add `quintileRank` to row type, add column (last static), add icon on name |
| Coaching Hub | `AgentDetailsCell.tsx` + `RecentCoachingActivities.tsx` | Thread quintile to row, add icon with tooltip |
| Feature flag (config) | `config/src/CustomerConfig.ts` | Add `enableQuintileRank` flag + `yarn gen:all` |
| Feature flag (director) | `localFeatureFlags.ts` or regenerate `frontendFeatureFlags.ts` | Add flag type for dev or regenerate after config lands |
| Gate all UI | All components above | Wrap quintile column + icon rendering with `useFeatureFlag('enableQuintileRank')` |
