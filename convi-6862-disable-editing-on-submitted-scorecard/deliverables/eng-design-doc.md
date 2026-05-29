# CONVI-6862 Engineering Design Doc

**Authors:** xuanyu.wang@cresta.ai
**Status:** Implemented / Documented
**Last reviewed / updated:** 2026-05-29
**Related ticket:** `CONVI-6862`

## Goal

Make submitted scorecards read-only for the in-scope surfaces while allowing explicit post-submit exceptions through template configuration.

## Scope

### In scope

- normal Closed Conversations scorecards
- normal process scorecards
- backend enforcement for submitted-lock behavior
- frontend configuration of submitted-scorecard editors in the template builder
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

## Product Semantics

Current product semantics:

- first submit remains allowed for an unsubmitted scorecard
- once an in-scope scorecard is submitted, the listed post-submit operations are locked
- lock exceptions are configured per template through `submitted_scorecard_editors` / `submittedScorecardEditors`
- reset is part of the submitted lock scope for this iteration

Historical note:

- the 2026-05-22 audience-style permitted-user pivot is now superseded by the merged implementation contract
- keep that earlier direction only as historical context, not as the active schema/design target

## Design Overview

The system exposes one submitted-lock concept for in-scope scorecards:

1. Unsubmitted in-scope scorecards continue to use existing edit and submit flows.
2. The first submit is still allowed.
3. After submit, the scorecard becomes read-only for the locked operations.
4. Backend enforces the lock as the source of truth.
5. Frontend exposes template-level submitted-editor configuration and reflects the backend lock on supported scorecard surfaces.

## Backend Design

Backend is the source of truth for submitted-lock enforcement.

### Enforcement policy

For in-scope scorecards:

- if the scorecard is not submitted, existing behavior remains
- if the scorecard is submitted, block:
  - criteria value updates
  - criterion comments
  - general notes edits
  - reset scorecard
- do not block the first submit action
- do not expand this iteration to appeals or calibration scorecards

### Permission contract

Template permission configuration uses:

- `submitted_scorecard_editors` / `submittedScorecardEditors`
- shape:
  - `users`
  - `teams`
  - `groups`

Runtime semantics:

- empty or unset submitted editors fall back to existing edit permission
- configured submitted editors are evaluated dynamically at runtime
- team and group membership is resolved on the backend

## Frontend Design

Frontend mirrors the backend lock and exposes submitted-editor configuration in the template builder.

### Template builder behavior

- the submitted-editor control lives in `TemplateBuilderAdvanced`
- FE uses `UserTeamGroupSelect`
- FE supports users, teams, and groups
- FE hydrates and saves all three buckets
- FE does not filter the picker by `permissions.scorecardGraders`
- changing `Who can use this scorecard` does not clear submitted editors
- empty-state text is `All users`

### UX expectations

For submitted in-scope scorecards:

- criteria inputs are read-only
- criterion comments are read-only
- general notes are read-only
- reset is disabled or hidden

For unsubmitted in-scope scorecards:

- editing remains available
- first submit remains available

## Schema Direction

The active implementation contract is the merged `submitted_scorecard_editors` / `submittedScorecardEditors` field with a `users + teams + groups` shape.

The earlier audience-style direction is now historical context only. Do not use it as the active design target unless the project is explicitly re-opened for a schema redesign.

## Validation

This design remains correct only if all of the following stay true:

- active docs say only normal Closed Conversations and normal process scorecards are in scope
- active docs say appeal and calibration scorecards are out of scope
- active docs include `ResetScorecard` in the submitted lock scope
- active docs keep first submit allowed
- active docs describe the merged `submitted_scorecard_editors` / `submittedScorecardEditors` contract as the source of truth
- active docs describe FE as supporting users, teams, and groups in `TemplateBuilderAdvanced`
