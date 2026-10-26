# Coaching Plan Page - Quintile Rank Badge Implementation Plan

**Created:** 2026-02-24

## Design

A badge displayed above the "Create new plan" button in the coaching plan header.

```
------------------
Overall agent rank

[Trophy icon] Xth quintile
------------------
```

### Layout (from Figma)

- Width: 147px, Height: 67px
- Padding: `Spacing/BASE` (top/bottom), `Spacing/MD` (left/right)
- Gap: 4px (between label and rank row)
- Border-radius: `Radius/md`
- Background: darkened header color via `color-mix(in srgb, var(--header-color) 80%, black)`

### Content

- **Line 1**: "Overall agent rank" — secondary text, small font
- **Line 2**: `[QuintileRankIcon] Xth quintile` — trophy icon + ordinal rank text

## Data Source

### Problem: quintile rank requires AGENT-only grouping

The backend `setQuintileRankForPerAgentScores` computes quintile rank across all agents in the response. However, the existing page calls group by AGENT+CRITERION (or other combinations), which produces per-agent-per-criterion scores — not a single per-agent overall score for proper quintile ranking.

**Quintile rank is relative to the response population, which is fine.** We don't need org-wide ranking. We just need a separate call grouped by AGENT only so the backend computes one quintile rank per agent based on their overall score.

### Pages that need a separate AGENT-only call

| Page | PR | Existing grouping | Fix |
|------|----|-------------------|-----|
| Coaching Hub | #16887 | AGENT+CRITERION | Add AGENT-only call, same filters |
| Coaching Plan | (new) | N/A (no existing call) | Add AGENT-only call |

Leaderboard (#16884) and Performance (#16886) already have or can use AGENT-only calls — verify on a case-by-case basis.

### API: `RetrieveQAScoreStats`

The API **requires** `scorecard_templates` (proto `REQUIRED` field). Cannot be called without templates.

### Correct approach: AGENT-only grouping call

Make a separate `RetrieveQAScoreStats` call grouped by AGENT only (not AGENT+CRITERION), using the same user/template filters as the page:

1. Use `useCurrentScorecardTemplates()` to get all active template names
2. Call `RetrieveQAScoreStats` with:
   - `filterByAttribute.users`: same as existing page filters (keep same population)
   - `filterByAttribute.scorecardTemplates`: all template names from step 1
   - `groupByAttributeTypes`: `[QA_ATTRIBUTE_TYPE_AGENT]` (agent-only → quintile rank populated)
   - `filterByTimeRange`: last 7 days
3. Find the specific agent in the response: `scores.find(s => s.groupedBy?.user?.name === agent.name)`
4. Extract `quintileRank` from that score's `groupedBy.quintileRank`

### Impact on Coaching Hub (#16887)

The current Coaching Hub implementation extracts quintile from `criteriaInfo[*].qAScoreResult.groupedBy.quintileRank`. This data comes from an AGENT+CRITERION call — the quintile rank is computed per-agent-per-criterion, not per-agent overall.

**Fix needed**: Add a separate AGENT-only call to the Coaching Hub page, with the same user filters but `groupByAttributeTypes: [QA_ATTRIBUTE_TYPE_AGENT]` only. Then extract each agent's quintile rank from that response instead.

### Skip conditions
- Skip when `enableQuintileRank` feature flag is off
- Skip when no templates available

## Files to Change

### 1. `AgentCoachingPlanHeader.tsx`
**Path**: `features/coaching-workflow/agent-coaching/agent-coaching-plan/agent-coaching-plan-header/AgentCoachingPlanHeader.tsx`

Changes:
- Add props: `quintileRank?: QuintileRank`, `enableQuintileRank?: boolean`
- Set CSS variable `--header-color` on the header div (alongside existing inline `background`)
- Render `QuintileRankBadge` component above "Create new plan" button in `.coachingPlanHeader__actions`
- Only render when `enableQuintileRank && quintileRank` is truthy

### 2. `AgentCoachingPlanHeader.module.css`
**Path**: same directory

Changes:
- Add `.quintileRankBadge` class:
  ```css
  .quintileRankBadge {
    width: 147px;
    height: 67px;
    padding: var(--cresta-spacing-base) var(--cresta-spacing-md);
    gap: 4px;
    border-radius: var(--radius-md);
    background-color: color-mix(in srgb, var(--header-color) 80%, black);
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  ```
- Add `.quintileRankBadge__label` for "Overall agent rank" text
- Add `.quintileRankBadge__rank` for the trophy + rank row

### 3. `AgentCoachingPlan.tsx`
**Path**: `features/coaching-workflow/agent-coaching/agent-coaching-plan/AgentCoachingPlan.tsx`

Changes:
- Add `useFeatureFlag('enableQuintileRank')` call
- Add `useQAScoreStats` call with:
  - All template names from existing `useCurrentScorecardTemplates()`
  - Agent name from `agent` prop
  - Group by `QA_ATTRIBUTE_TYPE_AGENT`
  - Last 7 days time range
  - Skip when flag off or no data
- Extract `quintileRank` from response
- Pass `quintileRank` and `enableQuintileRank` as props to `AgentCoachingPlanHeader`

### 4. Utility: `getOrdinalSuffix`
Either:
- Reuse from `AgentDetailsCell.tsx` (extract to shared util), OR
- Inline in the badge component (simpler, only 2 usage sites)

## Design token mapping

| Figma token | CSS variable (likely) |
|-------------|----------------------|
| `Spacing/BASE` | `var(--cresta-spacing-base)` or `var(--cresta-spacing-xs)` |
| `Spacing/MD` | `var(--cresta-spacing-md)` |
| `Radius/md` | `var(--radius-md)` or `var(--cresta-radius-md)` |

Need to verify exact token names in codebase.

## Badge visibility rules

- Show only when `enableQuintileRank` flag is on
- Show only for Q1/Q2/Q3 (has trophy icon) — or show for all ranks with just text for Q4/Q5?
  - **Decision needed**: Should the badge show for Q4/Q5 without icon, or hide entirely?
  - Recommendation: Show for all ranks (Q1-Q5) since the text "Xth quintile" is informative regardless. Only the trophy icon is Q1-Q3.
- Show regardless of `isAgentOnly` (agents should see their own rank)
- Show regardless of plan active status

## Per-page AGENT-only call pattern

Each page makes its own AGENT-only call using the same user/template filters it already uses, just changing the grouping:

| Parameter | Value |
|-----------|-------|
| `users` | Same as existing page filters (keep same population) |
| `scorecardTemplates` | Same as existing page filters |
| `groupByAttributeTypes` | `[QA_ATTRIBUTE_TYPE_AGENT]` |
| `filterByTimeRange` | Same as existing page filters |

The quintile rank is relative to the page's agent population, which is the correct behavior.

## Implementation order

1. Add QA score stats call in `AgentCoachingPlan.tsx` (data layer)
2. Add badge CSS in `AgentCoachingPlanHeader.module.css`
3. Add badge rendering in `AgentCoachingPlanHeader.tsx`
4. Verify design token names match codebase
5. Test with feature flag on/off
