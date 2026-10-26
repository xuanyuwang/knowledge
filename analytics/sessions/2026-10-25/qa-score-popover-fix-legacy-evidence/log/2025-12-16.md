# Daily Engineering Notes – 2025-12-16

## 1. Fixes (Bugs / Issues Resolved)
# Development Journey: QA Score Popover Bug Fix

## Branch: convi-5805-discrepancy-between-the-number-of-overwritten-shown-in-perf

This document chronicles the investigation, attempts, discoveries, and final solution for fixing the QA score popover bug.

---

## Phase 1: Initial Investigation (Discovery)

### Starting Point
**User Report**: "There's a discrepancy between the number of overwritten shown in Performance page cells vs. the popover."

**Initial Hypothesis**: The bug was a simple data mismatch issue.

### First Investigation Steps

1. **Located the Performance Page Components**
   - Found `Performance.tsx` renders `PerformanceProgression.tsx`
   - Identified `QAICell.tsx` as the cell component that shows popovers
   - Discovered `useColumnsForPerformanceProgression.tsx` creates column definitions

2. **Traced Data Flow**
   ```
   PerformanceProgression
     → creates rows with criterionIds
     → useColumnsForPerformanceProgression
       → wraps cells in QAICell
         → QAICell receives criterionIds prop
           → uses criterionIds to query agent scores for popover
   ```

3. **Key Discovery**: Each `QAICell` has a `criterionIds` prop that should identify which criterion(ia) to query

### Initial Problem Identified

When clicking a cell, the popover showed data for ALL criteria instead of just the clicked criterion. But interestingly:
- ✅ Some criteria worked correctly
- ❌ Others showed all criteria

**Critical Question**: Why only some criteria?

---

## Phase 2: Pattern Recognition (The "Why Non-Scorable?" Moment)

### Breakthrough Discovery
**User Observation**: "The bug doesn't exist for some criteria but exists for others. For example, if a criterion's `excludeFromQAScores` is true, then cells on that row have this bug."

This was the key insight! The bug only affected **non-scorable criteria** (those with `excludeFromQAScores: true`).

### Deep Dive Into Filtering Logic

1. **Examined QAICell query parameters**:
   ```typescript
   const requestParams = useQAScoreStatsRequestParams(modifiedFilterState, GROUP_BY, {
     enableAutofailScoring: title === getAllCriteriaName(),
     excludeNonScorable: !outcomeConfig?.isOutcome,  // ← Always true!
     additionalFilterByAttribute: additionalFilters,
   });
   ```

2. **Followed the `excludeNonScorable` flag**:
   - `excludeNonScorable: true` → triggers special filtering
   - Goes through `useQAFilterByAttribute` → `getQAFilterByAttribute`
   - Calls `getScorecardTemplateItemFilter` with `isScorable` filter

3. **Found the Bug** in `getQAFilterByAttribute.ts`:
   ```typescript
   if (filterState.scorecardTemplateItems?.length) {
     const intersectionCriteria = intersection(
       filterState.scorecardTemplateItems,
       scorableCriterionIdentifiers
     );
     if (!intersectionCriteria.length) {
       return scorableCriterionIdentifiers;  // ← BUG! Returns ALL
     }
     return intersectionCriteria;
   }
   ```

### Why The Bug Happened

For non-scorable criteria:
1. `scorecardTemplateItems = [nonScorableCriterionId]` (from cell)
2. System intersects with `scorableCriterionIdentifiers` (all scorable from template)
3. Intersection is empty (non-scorable ID not in scorable list)
4. Fallback returns ALL scorable criteria
5. Popover shows wrong data

For scorable criteria:
1. `scorecardTemplateItems = [scorableCriterionId]`
2. Intersection = `[scorableCriterionId]` ✅
3. Correct result

---

## Phase 3: Understanding Context (The "Display Mode" Revelation)

### User Clarification
**User**: "The table has three modes: Performance score, Number of exceptions, Number of overwritten. We skip non-scoreable criteria when showing Performance score. But when showing other two types, we don't care if a criteria is scoreable or not."

This changed everything! The issue wasn't just about non-scorable criteria - it was about **when** they should be included.

### New Understanding

**Three Display Modes with Different Rules**:

1. **Performance Score (RELATIVE)**
   - Include ONLY scorable criteria
   - Non-scorable don't contribute to scores
   - Current behavior: ✅ Correct (excludes non-scorable)

2. **Number of Exceptions (EXCEPTIONS)**
   - Include ALL criteria (scorable + non-scorable)
   - Non-scorable CAN have exceptions
   - Current behavior: ❌ Wrong (excludes non-scorable)

3. **Number of Overwritten (OVERWRITTEN)**
   - Include ALL criteria (scorable + non-scorable)
   - Non-scorable CAN be overwritten
   - Current behavior: ❌ Wrong (excludes non-scorable)

### The Real Bug
`QAICell` was using `excludeNonScorable: !outcomeConfig?.isOutcome`, which means:
- **Always** exclude non-scorable (except for outcomes)
- **Should be**: Only exclude in RELATIVE mode

---

## Phase 4: First Solution Attempt (The Quick Fix)

### Approach
Update `QAICell.tsx` to check display mode:

```typescript
excludeNonScorable: !outcomeConfig?.isOutcome && valueType === RELATIVE
```

### Implementation
1. Added import: `import { RELATIVE } from '../../../gridtable'`
2. Updated the condition
3. Tested locally

### Result
✅ Partially worked! Individual cells now showed correct data.
❌ But still had issues with:
- Chapter rows
- "All criteria" row
- CSV export
- Top agents column

**Lesson Learned**: The fix needed to be more comprehensive. Can't just fix one component when the filtering logic is scattered.

---

## Phase 5: Architectural Problem Recognition

### Discovery
**User**: "Since the display mode (HeatMapValueType) is global to a table, we should set up criterion filter at the top level (PerformanceProgression.tsx)."

This was a crucial architectural insight!

### Problems Identified

1. **Scattered Filtering Logic**:
   - QAICell had its own logic
   - Chapter queries used hardcoded `isScorable`
   - Top agents used boolean flag
   - CSV export had no filtering
   - Different components made different assumptions

2. **No Single Source of Truth**:
   - Each component decided independently what to filter
   - Led to inconsistencies and bugs

3. **Boolean Flags vs. Functions**:
   - Using `excludeNonScorable: boolean` was inflexible
   - Couldn't express "filter by type but not by setting"

### New Strategy
**Create a global criterion filter** at `PerformanceProgression` level:
1. One place defines filtering logic
2. Based on display mode
3. Applied consistently to ALL queries
4. Use filter functions instead of boolean flags

---

## Phase 6: Comprehensive Refactoring

### Step 6a: Global Filter Creation

**In PerformanceProgression.tsx**:
```typescript
const criterionFilter = useMemo(
  () => (heatmapConfigurationValueType === RELATIVE ? isScorable : isScorableCriterion),
  [heatmapConfigurationValueType]
);
```

**Why `isScorableCriterion` for EXCEPTIONS/OVERWRITTEN?**
- Still need to filter out non-scorable TYPES (like "sentence" types)
- But DON'T filter out criteria with `excludeFromQAScores: true`
- `isScorable` checks both; `isScorableCriterion` checks only type

### Step 6b: Apply to All Queries

Applied `criterionFilter` to:
1. Main table data query
2. "All criteria" row query
3. Chapter row queries
4. Top agents queries (3 separate calls)
5. CSV export query

### Step 6c: Refactor to Filter Functions

**Challenge**: The existing code used boolean flags (`excludeNonScorable`, `filterOnScorableCriteria`)

**Solution**: Refactor to accept filter functions
- `useQAScoreStatsRequestParams` accepts `criterionFilter` function
- `useQAFilterByAttribute` accepts `criterionFilter` function
- `getQAFilterByAttribute` accepts `criterionFilter` function
- `getScorecardTemplateItemFilter` accepts filter function parameter

**Benefits**:
- Type-safe (function signatures)
- Flexible (any filter logic)
- Clear intent (what's being filtered and why)

### Step 6d: Remove Buggy Fallback

**In getScorecardTemplateItemFilter**:
```typescript
// REMOVED THIS:
if (!intersectionCriteria.length) {
  return scorableCriterionIdentifiers;  // Buggy fallback
}

// NOW JUST RETURNS:
return intersection(filterState.scorecardTemplateItems, filteredCriterionIdentifiers);
```

This respects the requested criterion IDs even when intersection is empty.

---

## Phase 7: The "All Criteria" Row Mystery

### New Issue Discovered
**User**: "Now all rows match between displayed cell and popover, except the 'All criteria' row."

### Investigation
1. Checked the network requests - criterion IDs were correct ✅
2. But the displayed number (13) didn't match popover (16)
3. **Hypothesis**: The displayed data comes from `qaScoreStatsWholeTemplate`

### Root Cause Found
```typescript
const requestParamsWholeTemplate = useQAScoreStatsRequestParams(filtersState, GROUP_BY_TIME_RANGE, {
  enableAutofailScoring: true,  // ← This clears criterionIdentifiers!
  criterionFilter,
  additionalFilterByAttribute: additionalFilters,
});
```

**The Problem**:
1. `enableAutofailScoring: true` clears `criterionIdentifiers` to `[]`
2. Backend receives empty array
3. Backend auto-excludes non-scorable criteria
4. Returns 13 items (only scorable)
5. But we need 16 items (scorable + non-scorable in EXCEPTIONS mode)

### First Attempt: Conditional Clearing
**Idea**: Don't clear `criterionIdentifiers` if we have a `criterionFilter`

**Problem**: This changes shared logic that might affect other flows

### Better Solution: Explicit Criterion IDs
**User Insight**: "We have a full list of criterion IDs in `requestParams.filterByAttribute.criterionIdentifiers`. It's using our custom criterionFilter. It can serve as ground truth."

**Implementation**:
```typescript
const filtersStateWithExplicitCriteria = useMemo(
  () => ({
    ...filtersState,
    scorecardTemplateItems: requestParams.filterByAttribute?.criterionIdentifiers ?? [],
  }),
  [filtersState, requestParams.filterByAttribute?.criterionIdentifiers]
);

const requestParamsWholeTemplate = useQAScoreStatsRequestParams(
  filtersStateWithExplicitCriteria,  // Use explicit IDs
  GROUP_BY_TIME_RANGE,
  {
    enableAutofailScoring: true,
    additionalFilterByAttribute: additionalFilters,
  }
);
```

**Why This Works**:
- Reuses already-filtered criterion IDs as ground truth
- When `enableAutofailScoring` checks `scorecardTemplateItems?.length`, it finds IDs
- Doesn't clear them
- Backend gets explicit list, doesn't auto-filter
- Returns correct count

### Also Fixed Display Logic
```typescript
const criteriaRows = rows
  .filter((r) => !!r.templateItemCriterion)
  .filter((r) => criterionFilter(r.templateItemCriterion!));
```

This ensures the `criterionIds` in the row match what's queried.

---

## Phase 8: Edge Cases and Refinements

### Issue: TypeScript Errors
**Problem**: `requestParams.filterByAttribute` might be undefined

**Fix**: Add optional chaining and nullish coalescing:
```typescript
scorecardTemplateItems: requestParams.filterByAttribute?.criterionIdentifiers ?? []
```

**Linter**: Auto-fixed this during commit

### Issue: Linting Error
**Problem**: "`!criterionFilter` is always falsy"

**Why**: `criterionFilter` is always either `isScorable` or `isScorableCriterion`, never undefined

**Fix**: Remove the unnecessary check:
```typescript
// Before
.filter((r) => !criterionFilter || criterionFilter(r.templateItemCriterion!))

// After
.filter((r) => criterionFilter(r.templateItemCriterion!))
```

### Issue: Import Organization
**Linter**: Auto-organized imports during commit

---

## Phase 9: Testing and Validation

### Linting
```bash
yarn lint:precommit
```
**Result**: ✅ All checks pass

### Manual Testing Needed
- Performance Score mode (RELATIVE)
  - Individual cells
  - Chapter rows
  - "All criteria" row
- Number of Exceptions mode (EXCEPTIONS)
  - Individual cells (scorable and non-scorable)
  - Chapter rows
  - "All criteria" row
- Number of Overwritten mode (OVERWRITTEN)
  - Individual cells (scorable and non-scorable)
  - Chapter rows
  - "All criteria" row

---

## Phase 10: Final Fix - Remove enableAutofailScoring

### User Insight
**User**: "we need to remove line 134, right? Otherwise, the filterByAttribute.criterionIdentifiers is going to cleared"

This referred to the `enableAutofailScoring: true` parameter in the requestParamsWholeTemplate call.

### The Problem
In PerformanceProgression.tsx, the "All criteria" query was using:
```typescript
const requestParamsWholeTemplate = useQAScoreStatsRequestParams(
  filtersStateWithExplicitCriteria,
  GROUP_BY_TIME_RANGE,
  {
    enableAutofailScoring: true,  // ← This would clear criterionIdentifiers!
    additionalFilterByAttribute: additionalFilters,
  }
);
```

Even though we were passing explicit criterion IDs via `filtersStateWithExplicitCriteria`, the `enableAutofailScoring` flag could potentially interfere with the filtering logic.

### The Solution
Since we're already passing explicit criterion IDs through `filtersStateWithExplicitCriteria`, we don't need `enableAutofailScoring: true` at all. The explicit IDs serve as our source of truth.

**Fix**: Removed the `enableAutofailScoring` parameter entirely:
```typescript
const requestParamsWholeTemplate = useQAScoreStatsRequestParams(
  filtersStateWithExplicitCriteria,
  GROUP_BY_TIME_RANGE,
  {
    additionalFilterByAttribute: additionalFilters,
  }
);
```

### Why This Works
1. We're already filtering criteria via the global `criterionFilter`
2. The filtered IDs are captured in `requestParams.filterByAttribute.criterionIdentifiers`
3. We pass these explicit IDs via `filtersStateWithExplicitCriteria`
4. No need for `enableAutofailScoring` logic that might clear or modify them
5. Backend receives exactly the criteria we want, nothing more, nothing less

---

## Phase 11: Commit and Documentation

### Commit Created (Amended)
```
[db5c073f2c] [CONVI-5805] Fix QA score popover bug: respect display mode for criterion filtering
```

### Files Changed: 7
1. QAICell.tsx - Display mode check
2. PerformanceProgression.tsx - Global filter
3. useQAScoreStatsRequestParams.ts - Filter function support
4. useQAFilterByAttribute.ts - Filter function support
5. getQAFilterByAttribute.ts - Filter function, remove fallback
6. useGetQAScoreChapterStats.ts - Accept filter from parent
7. useTopAgentsQAScoreStats.ts - Accept filter from parent

### Documentation Created
1. **PR_DESCRIPTION.md** - For GitHub PR with testing instructions
2. **FINAL_COMPREHENSIVE_DOCUMENTATION.md** - Complete technical docs
3. **DEVELOPMENT_JOURNEY.md** - This document

---

## Key Learnings

### 1. Pattern Recognition Matters
The breakthrough came from recognizing that only non-scorable criteria were affected. This focused the investigation on filtering logic.

### 2. User Context Is Critical
Understanding the three display modes completely changed the solution approach. What seemed like a bug was actually a feature requirement.

### 3. Architecture Over Quick Fixes
The first fix (QAICell only) worked partially but left other components broken. The global filter architecture fixed everything consistently.

### 4. Type Safety Prevents Bugs
Moving from boolean flags to filter functions made the code:
- More type-safe
- More flexible
- Easier to understand
- Harder to misuse

### 5. Single Source of Truth
Having one place define filtering logic (global `criterionFilter`) eliminated inconsistencies and made the system predictable.

### 6. Backend Behavior Matters
Understanding that the backend auto-filters when `criterionIdentifiers: []` was crucial for fixing the "All criteria" row.

### 7. Reuse When Possible
Using `requestParams.filterByAttribute.criterionIdentifiers` as ground truth was elegant - it reused already-filtered data instead of duplicating logic.

---

## Timeline Summary

1. **Investigation Start**: Identified data flow and components
2. **Pattern Discovery**: Found bug only affects non-scorable criteria
3. **Context Understanding**: Learned about three display modes
4. **First Fix**: Updated QAICell (partial solution)
5. **Architecture Recognition**: Realized need for global filter
6. **Comprehensive Refactor**: Implemented global filter + function refactor
7. **"All Criteria" Debug**: Fixed backend auto-filtering issue
8. **Polish**: Resolved TypeScript/linting issues
9. **Testing & Commit**: Validated and committed changes
10. **Final Fix**: Removed enableAutofailScoring to prevent criterion ID clearing
11. **Documentation**: Created comprehensive docs

**Total Development Time**: ~1 session (iterative problem-solving)

---

## Remaining Work

### Testing Required
- Smoke test in all three display modes
- Verify chapter rows work correctly
- Verify CSV export has correct data
- Verify "All criteria" row shows correct count
- Verify no regressions in other pages

### Potential Future Improvements

1. **Type Definitions**: Consider creating explicit types for filter functions:
   ```typescript
   type CriterionFilter = (criterion: ScorecardCriterionTemplate) => boolean;
   ```

2. **Testing**: Add unit tests for filter functions and integration tests for display mode behavior

3. **Documentation**: Add inline comments explaining the display mode filtering logic for future developers

---

## Conclusion

This bug fix journey demonstrates the importance of:
- Systematic investigation
- Pattern recognition
- Understanding user context
- Architectural thinking
- Comprehensive solutions over quick patches

The final solution not only fixes the bug but improves the architecture, making the system more maintainable and less prone to similar issues in the future.

**Status**: ✅ Implementation Complete, Ready for QA Testing

---

**Document Author**: Claude Code
**Date**: 2024-12-15
**Branch**: convi-5805-discrepancy-between-the-number-of-overwritten-shown-in-perf
**Commit**: db5c073f2c (amended)

### Preventative Ideas:

I feel like this is a classic "null pointer" issue (not sure). Basically, we need to distinguish default value, and no-value.

We failed to do so, that's why we use all criteria instead of empty criteria. It makes the investigation harder, because the sympotom is misleading

## 2. Learnings (New Knowledge)
### What I learned:
### Context:
### Why it's important:
### Example:
### When to apply:

## 3. Surprises (Unexpected Behavior)
### What surprised me:
### Expected vs actual behavior:
### Why it happened:
### Takeaway:

## 4. Explanations I Gave
### Who I explained to (team / code review / slack):
### Topic:
### Summary of explanation:
### Key concepts clarified:
### Possible blog angle:

## 5. Confusing Things (First Confusion → Later Clarity)
### What was confusing:
### Why it was confusing:
### How I figured it out:
### Clean explanation (my future-self will thank me):
### Mental model:

## 6. Things I Googled Multiple Times
### Search topic:
### Why I kept forgetting:
### Clean “final answer”:
### Snippet / Command / Example:

## 7. Code Patterns I Used Today
### Pattern name:
### Situation:
### Code example:
### When this pattern works best:
### Pitfalls:

## 8. Design Decisions / Tradeoffs
### Problem being solved:
### Options considered:
### Decision made:
### Tradeoffs:
### Why this matters at a system level:
### Future considerations:

---

## Screenshots
(Drag & paste images here)

## Raw Snippets / Logs
\`\`\`
Paste raw logs, stack traces, or snippets here
\`\`\`

## Blog Potential
### Short post ideas:
### Deep-dive post ideas:
