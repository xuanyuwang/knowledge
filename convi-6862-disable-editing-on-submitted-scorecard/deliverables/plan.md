# CONVI-6862 Plan: Refresh Scope, Proto Direction, and BE Restart

## Summary

This plan reflects the 2026-05-22 Linear thread and replaces the earlier 2026-05-19 role-based framing.

The work now has three linked goals:

1. Align project knowledge and ticket language with the narrowed product scope.
2. Pivot proto planning from role-based `submitted_scorecard_editors` to audience-style permitted users.
3. Restart backend implementation from a clean branch at `origin/main`.

## Requirement Snapshot

### In scope for this iteration

- normal scorecards in Closed Conversations
- normal process scorecards

### Out of scope for this iteration

- appeal request
- appeal resolve
- group calibration answer key
- group calibration response

### Operations locked after submit

- criteria value editing
- criterion commenting
- general notes editing
- reset scorecard

### Operations still allowed

- first submit for an unsubmitted scorecard remains allowed

## Product Direction

The exception model is now permitted users, not permitted roles.

The product-facing behavior should be described as:

- submitted scorecards become read-only on the in-scope surfaces
- the backend is the source of truth for lock enforcement
- a configured permitted-user list can override the submitted lock
- empty or default semantics for that permitted-user list still need exact product/API confirmation, but the schema direction should be user-based rather than role-based

## Proto Direction

The active design should not center on:

- `repeated Role submitted_scorecard_editors`

Instead, the plan should follow the existing `audience` / `resolved_audience` pattern already used in `cresta/v1/coaching/scorecard_template.proto`.

### Canonical future contract

Add template-level permitted-user configuration shaped similarly to `Audience`, plus an output-only resolved field shaped similarly to `ResolvedTemplateAudience`.

Recommended design stance:

- add a template-level field for configured permitted users, modeled like `Audience`
- add a resolved output-only field for the expanded display payload, modeled like `ResolvedTemplateAudience`
- keep naming and exact field placement aligned with existing scorecard-template conventions

### Deprecated compatibility contract

If downstream branches or generated schema already contain `submitted_scorecard_editors`, keep it only as deprecated compatibility baggage rather than the preferred product-facing model.

Historical note:

- the discarded backend branch explored role-based `submitted_scorecard_editors`
- that exploration is now historical context, not the current recommended end state

## Shared BE and FE Plan

### Backend

Backend remains the source of truth.

Implementation expectations:

- enforce the submitted lock on the in-scope scorecard types
- block in-scope post-submit mutations for criteria edits, criterion comments, general notes edits, and reset
- allow first submit when the scorecard is not yet submitted
- keep appeals and calibration flows out of this implementation round
- use the permitted-user model as the forward-looking contract for override evaluation

### Frontend

Frontend should render the in-scope submitted scorecards read-only and stop invoking the locked mutations.

Implementation expectations:

- Closed Conversations normal scorecards become read-only after submit
- normal process scorecards become read-only after submit
- criteria editing, criterion comments, general notes editing, and reset controls are disabled or hidden when locked
- submit remains available only until the first successful submit
- appeals and calibration flows should not be changed in this round beyond avoiding accidental regressions

## Backend Restart Plan

Repository:

- `/Users/xuanyu.wang/repos/go-servers-convi-6862`

Branch to keep:

- `convi-6862-disable-editing-on-submitted-scorecard`

Current disposable state before reset:

- local HEAD: `5db8685484`

Reset target:

- `origin/main`
- expected target commit from the planning snapshot: `484a15e26d`

Reset steps:

1. `git fetch origin`
2. confirm the local branch is still `convi-6862-disable-editing-on-submitted-scorecard`
3. `git reset --hard origin/main`

Expected post-reset state:

- same local path
- same branch name
- clean working tree
- local branch no longer matches `origin/convi-6862-disable-editing-on-submitted-scorecard`
- any future push will require a history rewrite such as `--force-with-lease`

## Validation Checklist

Knowledge:

- `deliverables/plan.md` reflects the 2026-05-22 thread
- `deliverables/eng-design-doc.md`, `README.md`, and `project.yaml` match this scope
- no active doc presents role-based `submitted_scorecard_editors` as the preferred product contract
- no active doc says `ResetScorecard` is out of scope
- no active doc says all scorecard types or all post-submit operations are in scope

Linear:

- issue description matches the refined requirements
- a new top-level BE/FE execution-plan comment is added
- the ticket points engineers to this plan as the canonical detailed reference

Backend:

- `git status --short --branch` shows the feature branch clean
- `git rev-parse HEAD` matches `origin/main`
- `git log --oneline --decorate --max-count=5` shows the branch starting from `origin/main`
