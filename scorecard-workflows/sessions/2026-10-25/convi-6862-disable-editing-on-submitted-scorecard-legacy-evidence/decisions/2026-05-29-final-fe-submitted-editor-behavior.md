# Decision Record - Final FE Submitted-Editor Behavior

**Date:** 2026-05-29  
**Status:** Accepted

## Context

The earlier FE plan narrowed the submitted-editor selector to users only and tied it to `permissions.scorecardGraders`.

The current frontend implementation no longer follows that narrower plan:

- backend contract accepts `users + teams + groups`
- frontend now round-trips all three buckets
- the submitted-editor control lives in `TemplateBuilderAdvanced`

## Decision

For the current frontend implementation:

- the submitted-editor selector supports users, teams, and groups
- the selector does not depend on `permissions.scorecardGraders`
- changing `Who can use this scorecard` does not clear the submitted-editor selection
- the empty-state wording is `All users`
- placement in `TemplateBuilderAdvanced` is intentional

## Reasoning

- FE should reflect the merged backend contract rather than artificially hiding teams/groups.
- Coupling the selector to `scorecardGraders` created unnecessary UX constraints and stale-selection behavior.
- `TemplateBuilderAdvanced` is the correct long-term home for a more specialized permission control.
- `All users` more accurately reflects the current empty-state semantics than the earlier default-permission wording.

## Consequences

- FE docs must stop describing the selector as user-only or grader-filtered.
- Local validation should explicitly cover users, teams, and groups, plus save/reload round-trip behavior.
- The 2026-05-27 FE log entry remains useful as history, but it is no longer the active FE behavior.
