# CONVI-7379 Implementation — 2026-08-12

> **Superseded on 2026-08-13.** This document records the first implementation, which introduced revision-qualified writes. PR review and caller tracing exposed backward-compatibility problems with mixed formats and, especially, revision-independent targets. The accepted design now leaves writes as `templateID/criterionID` and fixes label recovery on reads only. See [`decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md`](../../decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md).

## Summary

Implemented the fix for Pure coaching plan 1:1 sessions showing raw focus-criteria IDs instead of criterion names. Both BE and FE changes are committed and pushed.

## Root Cause

Focus criteria are stored as `{template_id}/{criterion_id}` in DB. The BE read path never populated `criterionDisplayName` (even though the proto has the field). The FE resolved display names only from **current** scorecard templates. When criteria were removed in later template revisions, the FE couldn't find them → raw IDs displayed.

## Solution (Three Parts)

### Part 1: BE Write Path

Store revision ID alongside template ID in DB:
- New format: `{template_id}@{revision_id}/{criterion_id}`
- Empty/wildcard `ScorecardTemplateRevisionID` falls back to legacy format `{template_id}/{criterion_id}`
- File: `go-servers/apiserver/internal/coaching/transformers.go`

### Part 2: BE Read Path

Batch-resolve `criterionDisplayName` from historical template revisions:
- `parseFocusCriteriaID(raw string)` — parses both `{tid}@{rev}/{cid}` (new) and `{tid}/{cid}` (legacy)
- `resolveCriterionDisplayNames` — split resolution:
  - **Explicit refs**: query exact `(resource_id, revision)` pairs from DB
  - **Legacy refs**: query all revisions `ORDER BY created_at DESC`, build `tid@*/cid` keys (first-write-wins = latest revision)
- `criterionRefToDisplayName` — parses template JSON via `scoring.ParseScorecardTemplateStructure`, indexes criteria by `templateID@revisionID/criterionID → displayName`
- Targets table stores `scorecard_template_id/criterion_or_chapter_id` without revision — target lookup key strips revision
- `convertCoachingPlanToPB` and `convertCoachingSessionToPB` now accept `displayNames map[string]string` and populate `CriterionDisplayName`
- Endpoints updated: `GetCoachingPlan`, `ListCoachingSessions`, `ListCoachingPlans`

### Part 3: FE Simplification

Since BE now always populates `criterionDisplayName`, removed all redundant FE-side resolution logic:

**`getCriteriaInfoForTooltip`** — simplified logic:
```typescript
const criterionTemplate = criterionById[criterionId];
const isDeactivated = !criterionTemplate && !!criterion.criterionDisplayName;
const displayName = criterion.criterionDisplayName || criterionId;
const templateName = templateDisplayNameMap.get(baseTemplatePath) || unknownLabel;
```

**`CriterionTooltipContext`** simplified from `{ focusCriteriaOptions, outcomeGoalsOptions }` to `{ templateDisplayNameMap }`.

**`buildFocusCriteriaOptions` / `buildOutcomeGoalsOptions`** — use `criteria.criterionDisplayName` instead of `criterionById[id]?.displayName`.

**`FocusCriteriaSelector.tsx`** — removed `useGetCriteriaIdToDisplayName` hook entirely. Filter condition uses `!!value.criterionDisplayName` directly.

**Deactivated criteria** — displayed with `[Deactivated]` tag and muted styling (strikethrough) in `FocusCriteriaPills`, `OutcomeGoalPills`, and `CriterionTooltipContent`.

## Key Design Decisions

1. **BE-side resolution**: BE has DB access to historical revisions; FE shouldn't know about revision semantics
2. **Batch queries**: Single DB call collects all needed template revisions, not N+1 per criterion
3. **Backward compatible**: Legacy format without `@{revision}` auto-discovers all template revisions (latest revision wins via `ORDER BY created_at DESC` + first-write-wins)
4. **FE simplification**: Once BE always populates `criterionDisplayName`, FE removes all redundant lookup logic — no option-matching, no label-splitting

## Files Changed

### BE (go-servers, PR #31048)
- `transformers.go` — core read/write logic + batch resolver (`parseFocusCriteriaID`, `resolveCriterionDisplayNames`, `criterionRefToDisplayName`, `convertFocusCriteriaToDBFormat`, `convertCoachingPlanFocusCriteriaToDBFormat`)
- `action_get_coaching_plan.go` — pass displayNames to converter
- `action_list_coaching_sessions.go` — batch-resolve display names for all sessions
- `action_list_coaching_plans.go` — pass displayNames to converter
- `action_create_coaching_session.go` — nil pass (no DB in scope)
- `action_update_coaching_session.go` — nil pass (no DB in scope)
- `action_acknowledge_coaching_session.go` — nil pass (tx not in scope outside closure)
- Test files: `action_create_coaching_session_test.go`, `action_update_coaching_session_test.go`, `base_test.go`

### FE (director, branch `convi-7280-add-warning-for-opera-rules-deactivated-in-module-view`, commit `d2ff182d6b`)
- `utils.ts` — simplified `getCriteriaInfoForTooltip`, `buildFocusCriteriaOptions`, `buildOutcomeGoalsOptions`
- `types.ts` — simplified `CriterionTooltipContext`, added `isDeactivated` to `CriterionTooltipInfo`
- `useSessionCriteriaOptions.ts` — returns `templateDisplayNameMap`
- `CoachingSessionCard.tsx` — updated context construction
- `FocusCriteriaSelector.tsx` — removed `useGetCriteriaIdToDisplayName`
- `FocusCriteriaPills.tsx` — deactivated styling
- `OutcomeGoalPills.tsx` — deactivated styling
- `CriterionTooltipContent.tsx` — deactivated note in tooltip
- `CoachingSessionCard.module.css` — deactivated pill styles
- Test files: `utils.test.ts`, `useSessionCriteriaOptions.test.ts`

## Test Results

- **BE**: All unit tests pass (including updated test expectations for new DB format with `@revisionID`)
- **FE**: All 21 unit tests pass across 4 test files; lint, format, and i18n hooks clean

## Errors & Fixes During Implementation

1. **gofmt CI failure**: `transformers.go` field alignment — CI auto-fixed
2. **Test nil pointer dereference**: Target lookup key mismatch (targets use `tid/cid` without revision) — fixed by stripping revision from key
3. **Test format mismatch**: Tests expected old `templateId/criterionId` format — updated to `Template1Rev3Name.ScorecardTemplateRevisionID`
4. **`resolveCriterionDisplayNames` logic**: ASC ordering + `!exists` gave oldest revision — fixed to DESC ordering
5. **Build error in `action_get_coaching_plan.go`**: `dbmodel` not imported — used `parseFocusCriteriaRefs` directly
6. **Build error in `action_acknowledge_coaching_session.go`**: `tx` not in scope — reverted to pass `nil`
7. **FE `buildOutcomeGoalsOptions` filter**: included active non-outcome criteria — fixed to `!!criterionDisplayName && !template`
8. **FE test `isDeactivated`**: tests had empty `criterionById` for active criteria — added criterion entries to context
9. **PR review: `"*"` sentinel in ScorecardTemplateRevisionID**: Legacy criteria were resolved from historical revisions but the converter still sent `"*"` as the `ScorecardTemplateRevisionID` — added `resolvedRevisions` map alongside display names so converters populate the real revision ID
10. **PR review: DB errors silently swallowed**: `resolveCriterionDisplayNames` swallowed DB query errors (returned nil) — changed return type to `(map, map, error)` so callers can annotate and propagate failures
