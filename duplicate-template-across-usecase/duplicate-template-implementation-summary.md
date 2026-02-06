# Duplicate Scorecard Template to Another Use Case - Implementation Summary

**Linear Ticket:** CONVI-6116
**Date:** 2026-02-06 (Updated)

## Overview

This implementation adds "Duplicate to another use case" functionality for Performance Config (Scorecard) templates, mirroring the existing functionality for Opera Rules/Policies.

## Implementation Approach

Following the investigation doc, we used the **Frontend-Only Approach (Option B)**, which:
- Reuses the existing `CreateScorecardTemplate` API
- No backend changes required
- Implements a two-step modal flow (warning → use case selector)

## Files Created

### 1. `DuplicateScorecardTemplateModal.tsx`
**Path:** `/director/packages/director-app/src/features/admin/coaching/scorecard-templates/`

A new React component implementing the two-step modal:
- **Step 1 (Warning):** Displays warning about Opera integration being unlinked and audience reset
- **Step 2 (Select Use Case):** Use case selector using ListSelect component
- Filters use cases: same profile, excludes current UC, excludes CARE_EFFICIENCY
- On Create: navigates to template builder with `copyFrom` and `targetUsecase` query params
- Uses `TablerIcon` for channel icons with `getIconForConversationChannel` helper

## Files Modified

### 1. `ScorecardTemplateThreeDotMenu.tsx`
**Changes:**
- Added `onDuplicateToAnotherUsecase` prop
- Added new menu item "Duplicate to another use case" (hidden for archived templates)

### 2. `useScorecardTemplatesColumns.tsx`
**Changes:**
- Added `onDuplicateToAnotherUsecase` to hook options interface
- Passed callback to `ScorecardTemplateThreeDotMenu` component

### 3. `ScorecardTemplates.tsx`
**Changes:**
- Imported `DuplicateScorecardTemplateModal`
- Added state for modal visibility (`duplicateToUsecaseTemplate`)
- Added handlers: `handleDuplicateToAnotherUsecase`, `handleCloseDuplicateModal`
- Passed callback to `useScorecardTemplatesColumns`
- Rendered the modal

### 4. `consts.ts` (template-builder)
**Changes:**
- Added `TARGET_USECASE_PARAM = 'targetUsecase'` constant
- Added `DEFAULT_OUTCOME_CRITERION` constant with explicit undefined for optional properties

### 5. `ScorecardTemplateBuilder.tsx`
**Changes:**
- Added `targetUsecase` query parameter handling
- When `targetUsecase` is present with `copyFrom`:
  - Clears `audience` (will default to all agents)
  - Sets `usecaseNames` to target use case
- Passes `targetUsecase` to `TemplateBuilderForm`

### 6. `TemplateBuilderForm.tsx`
**Changes:**
- Added `targetUsecase` prop
- Passes `targetUsecase` to `TemplateBuilderFormConfigurationStep`
- Derives `isCrossUseCaseDuplicate` from `targetUsecase` for backfill prompt logic

### 7. `TemplateBuilderFormConfigurationStep.tsx`
**Changes:**
- Added `targetUsecase` prop
- **Smart outcome reset logic:**
  - Uses `useOutcomeMetadata` with target use case filter
  - Uses `useWatch` to observe template items
  - Calls `resetUnavailableOutcome` helper to reset only unavailable outcomes
  - Preserves outcomes that exist in both source and target use cases

### 8. `useOutcomeMetadata.ts` (director-api)
**Changes:**
- Added optional `usecases` parameter to `UseOutcomeMetadataOptions` interface
- Passes `usecases` filter to `useAllMoments` hooks

## User Flow

1. User navigates to Admin > Performance Config
2. Clicks 3-dot menu on a template row
3. Selects "Duplicate to another use case"
4. Modal appears with warning about what will/won't be copied
5. User clicks "Understood & Continue"
6. Use case selector appears
7. User selects target use case and clicks "Create"
8. User is redirected to template builder with copied template
9. The duplicated template has:
   - Title shows " Copy" suffix
   - Audience cleared (defaults to all agents)
   - Use case set to selected target
   - **Outcome criteria that exist in target use case remain configured**
   - **Outcome criteria that don't exist in target use case are reset to "New Conversation Outcome"**
10. User reviews/adjusts the template and saves

## What Gets Copied

| Item | Copied? |
|------|---------|
| Template structure (criteria, chapters) | Yes |
| Template title (+ " Copy" suffix) | Yes |
| Template type (Conversation/Process) | Yes |
| Permissions (role-based) | Yes |
| Scoring configuration | Yes |

## What Does NOT Get Copied

| Item | Behavior |
|------|----------|
| Audience | Defaults to all agents |
| QA task configuration | Cleared |
| Auto-QA triggers (Opera integration) | Cleared |
| Outcome metadata references | **Smart reset**: Only outcomes unavailable in target use case are reset |

## Smart Outcome Reset Logic

When duplicating across use cases, the system:
1. Fetches available outcome metadata for the target use case
2. For each outcome criterion in the template:
   - Checks if the metadata `resource_name` exists in target use case
   - If **available**: keeps the outcome configuration intact
   - If **unavailable**: resets to empty state with generic "New Conversation Outcome" name
3. Uses early return pattern for readability in `resetUnavailableOutcome` helper

## Constraints

- Same profile only (cannot duplicate across profiles)
- Single use case at a time
- Cannot duplicate archived templates
- Same template type preserved

## Code Quality Improvements

- Uses `TablerIcon` instead of `FeatherIcon` for icon wrapping
- Extracts `getIconForConversationChannel` helper function
- Uses `useWatch` instead of `form.getValues()` for reactive form updates
- Uses early return pattern for better readability
- Derives `isCrossUseCaseDuplicate` from `targetUsecase` (no redundant prop)
