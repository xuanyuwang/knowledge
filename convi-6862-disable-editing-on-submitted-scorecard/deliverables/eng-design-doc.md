# CONVI-6862 Engineering Design Doc

**Authors:** xuanyu.wang@cresta.ai  
**Status:** Draft  
**Last reviewed / updated:** 2026-05-22  
**Related ticket:** `CONVI-6862`

## Goal

Ship a clean v1 that makes submitted scorecards read-only for the current in-scope surfaces, while aligning the future contract around permitted users instead of permitted roles.

## Scope

### In scope

- normal Closed Conversations scorecards
- normal process scorecards
- backend enforcement for submitted-lock behavior
- frontend read-only behavior for the in-scope surfaces
- locking these post-submit operations:
  - criteria value editing
  - criterion commenting
  - general notes editing
  - reset scorecard

### Out of scope

- appeal request scorecards
- appeal resolve scorecards
- group calibration answer key scorecards
- group calibration response scorecards
- redesigning broader appeal or calibration workflows
- finalizing every product detail of empty/default permitted-user semantics

## Product Semantics

The 2026-05-22 thread supersedes the earlier 2026-05-19 framing.

Current product semantics:

- first submit remains allowed for an unsubmitted scorecard
- once an in-scope scorecard is submitted, the listed post-submit operations are locked
- lock exceptions should be modeled as permitted users, not permitted roles
- reset is part of the submitted lock scope for this iteration

Historical note:

- the discarded backend branch explored a role-based `submitted_scorecard_editors` approach
- that branch is useful only as historical exploration and should not drive the current contract

## Design Overview

The system should expose one coherent submitted-lock concept for the in-scope scorecards.

Behavioral model:

1. Unsubmitted in-scope scorecards continue using current edit and submit flows.
2. The first submit is still allowed.
3. After submit, the scorecard becomes read-only for the locked operations.
4. Backend enforces the lock as the source of truth.
5. Frontend reflects the same lock by rendering the scorecard read-only and suppressing the blocked actions.

This is intentionally narrower than the earlier "disable everything after submit" draft. Appeals, calibration flows, and unrelated operations are not part of this iteration.

## Backend Design

Backend is the source of truth for submitted-lock enforcement.

### Enforcement policy

For the in-scope scorecards:

- if the scorecard is not submitted, existing behavior remains
- if the scorecard is submitted, block:
  - criteria value updates
  - criterion comments
  - general notes edits
  - reset scorecard
- do not block the first submit action
- do not expand this iteration to appeals or calibration scorecards

### Enforcement shape

The lock logic should be centralized enough that FE and BE share the same semantics, but the action layer should still choose the operation being attempted.

Recommended operation categories for the current design:

- `update_content`
- `submit`
- `reset`

V1 policy:

- `update_content` is denied after submit for the in-scope scorecards
- `reset` is denied after submit for the in-scope scorecards
- `submit` remains a first-submit transition and is not itself part of the disabled set

### Why backend action layers still matter

The lock should not be described as a single blanket DAO rule because the operation category matters:

- `submit` must remain allowed before submission
- `reset` is now included explicitly, not accidentally
- appeal and calibration paths remain out of scope for this round

That makes action-layer selection of operation type important even if shared helpers carry the core policy.

## Frontend Design

Frontend should mirror the backend lock on the in-scope surfaces.

### UX expectations

For submitted in-scope scorecards:

- criteria inputs are read-only
- criterion comments are read-only
- general notes are read-only
- reset is disabled or hidden
- the user should not be led into autosave or manual update flows that will fail at the backend

For unsubmitted in-scope scorecards:

- editing remains available
- first submit remains available

### Surface expectations

The design explicitly covers:

- normal Closed Conversations scorecard surfaces
- normal process scorecard surfaces

The design explicitly excludes:

- appeal request surfaces
- appeal resolve surfaces
- group calibration surfaces

## Proto and Schema Direction

The preferred product-facing contract is audience-style permitted users, not role-based `submitted_scorecard_editors`.

### Canonical future contract

Follow the existing pattern already present in `cresta/v1/coaching/scorecard_template.proto`:

- an input/config field shaped like `Audience`
- a resolved output-only field shaped like `ResolvedTemplateAudience`

Recommended direction:

- add a template-level field for configured permitted users, modeled like `Audience`
- add a resolved output-only field for those permitted users, modeled like `ResolvedTemplateAudience`
- keep this as the main path for future FE/BE behavior and template modeling

This matches the current product requirement more closely than a role list.

### Deprecated compatibility contract

If `submitted_scorecard_editors` already exists in downstream work or generated schema, keep it only as deprecated compatibility support.

Design stance:

- do not treat `submitted_scorecard_editors` as the preferred end-state schema
- do not center new design or API documentation on role-based override semantics
- only retain it to avoid churn where branches or generated artifacts already introduced it

## Migration and Compatibility Notes

There are two distinct concepts that should not be conflated:

- canonical future contract: user-based audience-style permitted-user configuration
- deprecated compatibility contract: role-based `submitted_scorecard_editors`

Documentation, API planning, and implementation sequencing should keep those separate so downstream teams do not mistake compatibility baggage for product direction.

## Delivery Plan

### Backend

- restart the current backend branch from `origin/main`
- re-implement lock enforcement from the narrowed 2026-05-22 scope
- treat reset as part of the lock scope
- keep submit as a first-submit action
- leave appeals and calibration out of this round

### Frontend

- render submitted in-scope scorecards read-only
- disable the locked operations on the supported surfaces
- avoid continuing stale autosave/update flows once the scorecard is submitted
- keep appeals and calibration untouched for now

### Proto

- pivot planning to audience-style permitted-user fields
- treat any role-based field as deprecated compatibility only

## Risks and Open Questions

- The exact field names for the new permitted-user and resolved-permitted-user proto fields still need API review.
- Empty/default permitted-user semantics may still need product confirmation.
- FE and BE must align on the same definition of "normal" scorecards for the in-scope surfaces.
- If downstream branches already generated role-based schema, compatibility handling must avoid implying that role-based modeling remains the preferred requirement.

## Validation

This design is correct only if all of the following stay true:

- active docs say only normal Closed Conversations and normal process scorecards are in scope
- active docs say appeal and calibration scorecards are out of scope
- active docs include `ResetScorecard` in the submitted lock scope
- active docs keep first submit allowed
- active docs describe permitted users as the product direction
- active docs describe audience/resolved-audience modeling as the preferred proto direction
- active docs treat `submitted_scorecard_editors` as deprecated compatibility only
