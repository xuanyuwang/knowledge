# CONVI-6862 - Disable Editing on Submitted Scorecard

**Created:** 2026-05-19  
**Updated:** 2026-05-22

## Overview

This project captures the refreshed requirements, design direction, and implementation reset plan for CONVI-6862.

The active requirement set comes from the 2026-05-22 Linear thread, which narrowed scope and replaced the older role-based framing with a permitted-user direction.

## Current Objective

Align the knowledge base and ticket with the narrowed scope, pivot proto planning to audience-style permitted users, and restart the backend implementation from `origin/main`.

## Current Scope

In scope:

- normal Closed Conversations scorecards
- normal process scorecards
- post-submit lock for criteria edits, criterion comments, general notes editing, and reset

Out of scope:

- appeal request
- appeal resolve
- group calibration answer key
- group calibration response

Submit remains a first-submit action for unsubmitted scorecards.

## Key Findings

- Submitted scorecards are not currently immutable for the in-scope normal scorecard flows.
- The 2026-05-22 product clarification changed the exception model from permitted roles to permitted users.
- `ResetScorecard` is now explicitly inside the submitted-lock scope for this iteration.
- The recommended proto direction is to follow the existing `audience` / `resolved_audience` pattern in `cresta/v1/coaching/scorecard_template.proto`.
- The discarded backend branch explored role-based `submitted_scorecard_editors`; that is now historical context, not the preferred end-state contract.

## Status

Active

## Source Context

- **Primary repo:** `director`
- **Repo path:** `/Users/xuanyu.wang/repos/director`
- **Backend worktree to restart:** `/Users/xuanyu.wang/repos/go-servers-convi-6862`
- **Current backend branch name:** `convi-6862-disable-editing-on-submitted-scorecard`

Investigation and implementation touch:

- `director`
- `go-servers`
- `cresta-proto`

## Log History

| Date | Summary |
|------|---------|
| 2026-05-19 | Created the project and drafted the initial role-based hard-lock design. |
| 2026-05-22 | Refreshed scope from the Linear thread, pivoted proto planning to audience-style permitted users, and prepared the backend branch reset. |

## Related Artifacts

- `project.yaml`
- `log/2026-05-19.md`
- `log/2026-05-22.md`
- `sessions/2026-05-19/codex-requirements-and-design.md`
- `decisions/2026-05-19-separate-post-submit-permission.md`
- `decisions/2026-05-22-permitted-users-audience-pivot.md`
- `deliverables/plan.md`
- `deliverables/eng-design-doc.md`
