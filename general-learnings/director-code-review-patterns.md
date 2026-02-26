# Director Code Review Patterns

**Created**: 2026-02-25
**Source**: Human reviewer feedback from agent-quintiles PRs (#16883, #16911, #16884, #16886, #16887, #16905)

Patterns and conventions that reviewers enforce in the director repo. Follow these to avoid common review feedback.

---

## 1. Never Suppress Lint Rules -- Fix the Root Cause

**Reviewer**: t-vince (called out in #16886, #16887)
**Files**: `useLeaderboardByScorecardTemplateData.tsx`, `useLeaderboardPerCriterionColumns.tsx`, `AgentDetailsCell.tsx`

**Issue**: `eslint-disable-next-line import/no-internal-modules` was added to import `QuintileRankIcon` from a deep internal path.

**Preferred pattern**: Do not disable lint rules with inline comments. If a component needs to be shared across packages, move it to the proper shared library (`@cresta/director-components`). All custom icons live in `director-components`.

**Quote**: "Please don't just disable the rules. It looks like it's an Icon, most (if not all) of the shared icons are in director-components. Please move the icon there & update the references" -- t-vince

---

## 2. Use Design System Color Tokens, Not Raw Hex Values

**Reviewer**: t-vince (#16883)
**File**: `QuintileRankIcon.module.css`

**Issue**: Raw hex color values like `#ff9900` were used for the icon colors.

**Preferred pattern**: All designs should use colors from the design system. If new colors are needed, they should be added to the color system. Ping the design lead (Patrick Soutar) for new color requirements.

**Quote**: "Normally all designs should use the new design colors. If there are new colors required, they can be added to the color system." -- t-vince

---

## 3. Use `useTranslation` Hook, Not `getI18n().getFixedT()`

**Reviewer**: Hintful (#16886)
**Files**: `useLeaderboardByScorecardTemplateData.tsx`, `useLeaderboardPerCriterionColumns.tsx`

**Issue**: Used `getI18n().getFixedT(null, 'director-app-insights', 'qa.quintile-rank')` for translations.

**Preferred pattern**: In React hooks and components, use the `useTranslation` hook directly:
```tsx
const { t } = useTranslation('director-app-insights', { keyPrefix: 'qa.quintile-rank' });
```

---

## 4. i18n Ordinal Pattern with `_I18N_EXTRACT_` Macros

**Reviewers**: t-vince, gl3nn (#16887)
**File**: `AgentDetailsCell.tsx`

**Issue**: Used a custom `getOrdinalSuffix()` function to build strings like "1st quintile based on last 7 days". This is not translatable.

**Preferred pattern**: Use i18next's ordinal pluralization:
```tsx
t('quintile', { count: agentQuintileRank, ordinal: true, defaultValue: '{{count}}st quintile' })
```

With explicit extract macros for all ordinal variants (placed *after* the main usage in JSX):
```tsx
{_I18N_EXTRACT_ && t('quintile_ordinal_two', '{{count}}nd quintile')}
{_I18N_EXTRACT_ && t('quintile_ordinal_few', '{{count}}rd quintile')}
{_I18N_EXTRACT_ && t('quintile_ordinal_other', '{{count}}th quintile')}
```

**Key rules**:
- Extract macros must appear **after** the main `t()` call, not before
- Do **not** pass `count`/`ordinal` parameters in the extract macros -- just the key and default value
- The `_ordinal_one` case is already covered by the main `t()` call's `defaultValue`, so a separate extract macro for it is redundant
- Run `yarn i18n:full-extract` and verify the resulting locale file looks correct

**Quote**: "You can't have these in the code before the main occurrence." -- gl3nn

---

## 5. Mark Translation-Exempt Terms Properly

**Reviewer**: t-vince (#16883)
**File**: `director-app-insights.json` (locales)

**Issue**: "Quintile Rank" was added as a translatable string, but it may be a domain term that should be excluded from translation.

**Preferred pattern**: Some domain-specific terms should be added to the i18n ignore index. Coordinate with the i18n lead (gl3nn) regarding the ignore index.

---

## 6. Use Consistent Empty-Value Placeholders (`--` Double Hyphen)

**Reviewer**: Hintful (#16884, #16886)
**Files**: `AgentLeaderboard.tsx`, `useLeaderboardByScorecardTemplateData.tsx`, `useLeaderboardPerCriterionColumns.tsx`

**Issue**: Used en-dash `'--'` as placeholder for unspecified values, inconsistent with the codebase.

**Preferred pattern**: Use `'--'` (double ASCII hyphen) for empty/unspecified cell values across all tables for consistency.

---

## 7. Extract Magic Numbers as Named Constants

**Reviewer**: Hintful (#16884)
**File**: `AgentLeaderboard.tsx`

**Issue**: Inline `fixedWidth: 80` in column meta.

**Preferred pattern**: Define constants outside the component at module level:
```tsx
const QUINTILE_RANK_COLUMN_WIDTH = 80;
// ...
meta: { fixedWidth: QUINTILE_RANK_COLUMN_WIDTH },
```

---

## 8. Choose Clear, Accurate Variable Names

**Reviewer**: Hintful (#16886)
**File**: `LeaderboardPerCriterion.tsx`

**Issue**: A variable named `GROUP_BY_AGENT_ONLY` was misleading about its purpose.

**Preferred pattern**: Name variables to accurately reflect their semantic purpose. The fix was renaming to `QUINTILE_GROUP_BY`.

---

## 9. Consider Memoization for `.find()` in Render Paths

**Reviewer**: t-vince (#16887)
**File**: `AgentDetailsCell.tsx`

**Issue**: `overview.criteriaInfo.find(...)` was called inline without memoization, raising concern about performance with large lists.

**Preferred pattern**: When iterating over potentially large arrays (like `criteriaInfo`) inside cell renderers, consider whether the computation is already memoized or needs to be wrapped in `useMemo`.

---

## 10. Extract Reusable UI + Logic into Self-Contained Components

**Reviewer**: flatplate (#16905)
**File**: `AgentCoachingPlanHeader.tsx`, `AgentCoachingPlan.tsx`

**Issue**: Quintile rank data-fetching, feature flag checking, and badge rendering were spread across parent (`AgentCoachingPlan`) and child (`AgentCoachingPlanHeader`) components, making the parent bloated.

**Preferred pattern**: When a feature is self-contained (its own data fetching, feature flag, rendering), extract it into a dedicated component. The fix was creating `QuintileRankBadge` that owns everything internally.

**Quote**: "This feels like it can be its own component. We can even pack the quintile related logic inside that component, would make things a bit cleaner imo" -- flatplate

---

## 11. Show Loading States for Async Data

**Reviewer**: flatplate (#16905)
**File**: `AgentCoachingPlan.tsx`

**Issue**: The `useQAScoreStats` hook's loading state was not destructured or displayed while quintile data was being fetched.

**Preferred pattern**: Always destructure and use the `isLoading` state from data hooks. Show a loading indicator while data is being fetched, rather than silently showing nothing.

**Quote**: "I find it strange that we don't use the loading state here. Can we show loading indicator while this is loading" -- flatplate

---

## 12. Do Not Override the PR Description Template

**Reviewer**: t-vince (#16886, #16887 -- review-level feedback)

**Issue**: The PR description template was replaced/overridden (likely by AI-generated text), removing the standard sections that GitHub Actions relies on for checks.

**Preferred pattern**:
- Keep the PR description template intact -- it is used by GH Actions for automated checks
- Add custom content in the "description" section, do not replace the entire template
- Always include a screenshot proving you tested the change
- Do not blindly use AI to generate test steps in the PR -- there is already a GH Action that generates test steps for QA after merge

**Quote**: "Don't just change the description, you can add things in the 'description' part, but the description template is used in the GH actions to perform some checks. E.g. a part of it is a request for a screenshot to show that you've actually tested your code." -- t-vince

---

## 13. Include Visual Screenshots in PR Descriptions

**Reviewer**: tinglinliu (#16886)

**Issue**: No visual screenshots were included in the PR description for a UI change.

**Preferred pattern**: For any frontend/UI PR, include screenshots in the PR description showing the rendered result to prove the change was tested.

---

## Summary of Reviewers and Their Focus Areas

| Reviewer | Focus Areas |
|----------|-------------|
| **t-vince** | Lint rule enforcement, shared component architecture, design tokens, i18n, PR template discipline |
| **Hintful** | Consistency (placeholders, naming), code hygiene (named constants), using proper React hooks for i18n |
| **gl3nn** | i18n implementation details -- ordinal patterns, `_I18N_EXTRACT_` macro placement and syntax |
| **flatplate** | Component architecture (encapsulation, SRP), loading state handling |
| **tinglinliu** | PR documentation quality (screenshots) |
