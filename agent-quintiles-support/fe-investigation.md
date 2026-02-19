# FE Investigation: Quintile Rank in Director

**Created:** 2026-02-18
**Updated:** 2026-02-18

## Overview

The BE now returns `quintile_rank` (enum `QuintileRank`, values 0–5) on `QAScoreGroupBy` for per-agent `RetrieveQAScoreStats` responses. The FE needs to:

1. Pick up the new proto type
2. Thread it through the transformer and internal types
3. Display it on Performance, Leaderboard, and Coaching Hub pages

## Current State

- `@cresta/web-client@2.0.534` includes `QuintileRank` enum and `quintileRank` field on `QAScoreGroupBy` (verified).
- **No references** to `quintileRank` or `QuintileRank` exist anywhere in the director codebase yet.

## Data Flow

```
@cresta/web-client (proto generated types)
  → transformersQAI.ts: transformQAScoreGroupBy()
    → apiTypes.ts: QAScoreGroupBy interface
      → hooks (useQAScoreStats, etc.)
        → components (AgentLeaderboard, LeaderboardByScorecardTemplateItem, etc.)
```

## Files to Change

### 1. Type Layer (director-api package)

**`packages/director-api/src/services/cresta-api/insights/apiTypes.ts:675-686`**

Current `QAScoreGroupBy`:
```typescript
export interface QAScoreGroupBy {
  intervalStart?: Dayjs;
  team?: Team;
  directGroupMembership?: GroupMembership;
  group?: Group;
  user?: User;
  criterionId?: string;
  agentTier?: AgentTier;
  // ADD: quintileRank?: QuintileRank;
}
```

**`packages/director-api/src/services/cresta-api/insights/transformersQAI.ts:35-50`**

`transformQAScoreGroupBy` — add `quintileRank: groupedBy?.quintileRank` to the return object (follows the `agentTier: groupedBy?.agentTier` pattern on line 48).

### 2. Leaderboard Row Types

**`packages/director-app/src/features/insights/leaderboard/types.ts`**

Add to `LeaderboardRow` (line 52):
```typescript
quintileRank?: QuintileRank;
```

This propagates to `AgentLeaderboardRow` and `TeamLeaderboardRow` via inheritance.

### 3. Agent Leaderboard Row Building

**`packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboard.tsx:231-240`**

Current loop that sets `row.adherence`:
```typescript
for (const groupResult of score.data?.qaScoreResult.scores || []) {
  if (!groupResult.groupedBy) { continue; }
  const row = getCorrectRowFromLeaderboardQAGroupBy(groupResult.groupedBy, agentIdToData);
  if (!row) { continue; }
  row.adherence = groupResult.score || 0;
  // ADD: row.quintileRank = groupResult.groupedBy?.quintileRank;
}
```

### 4. Column Definitions

**`packages/director-app/src/features/insights/leaderboard/utils.tsx`**

Add a quintile rank column accessor, following the pattern of the `adherence` column. Needs a cell renderer — either plain text (Q1–Q5) or a colored badge component.

### 5. Performance Page Tables

**LeaderboardByScorecardTemplateItem** (`packages/director-app/src/components/insights/qa-insights/leaderboard-by-scorecard-template-item/`)
- Hook: `useLeaderboardByScorecardTemplateData` — builds rows from `qaScoreResult.scores`
- Columns: `useColumnsFromScorecardTemplate`

**LeaderboardPerCriterion** (`packages/director-app/src/components/insights/qa-insights/leaderboard-per-criterion/`)
- Row building in component (~line 170–250)
- Columns: `useLeaderboardPerCriterionColumns`

**PerformanceProgression** (`packages/director-app/src/components/insights/qa-insights/performance-progression/`)
- Groups by CRITERION + TIME_RANGE (not AGENT), so quintile rank won't appear here unless the grouping changes.

### 6. Coaching Hub

**`packages/director-app/src/features/coaching-workflow/coaching-hub/CoachingHub.tsx`**

Does not directly display per-agent QA scores in the main view. Quintile rank integration here would require threading it into agent overview/details drawers. Likely a follow-up.

## Display Component

**Reference:** `QAScoreBadge` at `packages/director-app/src/components/qa/shared/QAScoreBadge.tsx` — simple Paper wrapper with colored text. Can model a `QuintileRankBadge` similarly.

**Options:**
- Text badge: "Q1"–"Q5" with 5-color palette
- Colored dot/circle icon
- Use existing `agentTier` display pattern if one exists

## Dependency Check

Verified: `@cresta/web-client@2.0.534` includes `QuintileRank` enum and `quintileRank` on `QAScoreGroupBy`. No dep bump needed.

## Summary of Changes

| File | Change |
|------|--------|
| `apiTypes.ts` | Add `quintileRank?: QuintileRank` to `QAScoreGroupBy` |
| `transformersQAI.ts` | Map `quintileRank` in `transformQAScoreGroupBy` |
| `types.ts` (leaderboard) | Add `quintileRank` to `LeaderboardRow` |
| `AgentLeaderboard.tsx` | Assign `row.quintileRank` in QA score loop |
| `utils.tsx` (leaderboard) | Add quintile rank column definition |
| `useColumnsFromScorecardTemplate.tsx` | Add quintile column to Performance leaderboard |
| `useLeaderboardPerCriterionColumns.tsx` | Add quintile column to per-criterion leaderboard |
| New component | `QuintileRankBadge` or similar display component |
