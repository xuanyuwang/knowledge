# Duplicate Scorecard Template to Another Use Case - Implementation Summary

**Linear Ticket:** CONVI-6116
**Date:** 2026-02-03

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

### 2. `DuplicateScorecardTemplateModal.module.css`
**Path:** `/director/packages/director-app/src/features/admin/coaching/scorecard-templates/`

CSS module for the modal styling.

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

### 5. `ScorecardTemplateBuilder.tsx`
**Changes:**
- Added `targetUsecase` query parameter handling
- When `targetUsecase` is present with `copyFrom`:
  - Clears `audience` (will default to all agents)
  - Sets `usecaseNames` to target use case
  - Clears outcome references (auto_qa for metadata type criteria)
- Added `clearOutcomeReferences()` helper function

## User Flow

1. User navigates to Admin > Performance Config
2. Clicks 3-dot menu on a template row
3. Selects "Duplicate to another use case"
4. Modal appears with warning about what will/won't be copied
5. User clicks "Understood & Continue"
6. Use case selector appears
7. User selects target use case and clicks "Create"
8. User is redirected to template builder with copied template
9. Title shows " Copy" suffix
10. User edits and saves as new template

## What Gets Copied

| Item | Copied? |
|------|---------|
| Template structure (criteria, chapters) | Yes |
| Template title (+ " Copy" suffix) | Yes |
| Template type (Conversation/Process) | Yes |
| Permissions (role-based) | Yes |
| Scoring configuration | Yes |

## What Does NOT Get Copied

| Item | Cleared |
|------|---------|
| Audience | Defaults to all agents |
| QA task configuration | Cleared |
| Auto-QA triggers (Opera integration) | Cleared |
| Outcome metadata references | Cleared |

## Constraints

- Same profile only (cannot duplicate across profiles)
- Single use case at a time
- Cannot duplicate archived templates
- Same template type preserved

## Testing Notes

The implementation follows the same patterns as `DuplicatePolicyModal.tsx`. TypeScript shows some implicit `any` type warnings which are consistent with the existing codebase patterns.
