# Session Note - 2026-05-19 - Codex - Requirements and Design

**Started:** 2026-05-19 10:00 America/Toronto  
**Tool:** Codex  
**Project:** `convi-6862-disable-editing-on-submitted-scorecard`  
**Goal:** Pressure-test the ticket requirements, capture missing clarifications, and start the first engineering design draft.

## Source Context

- **Primary repo:** `director`
- **Repo path:** `/Users/xuanyu.wang/repos/director`
- **Worktree path:** `/Users/xuanyu.wang/repos/director`
- **Branch:** `main`
- **Ticket / PR:** `CONVI-6862`

## Inputs Reviewed

- Linear ticket `CONVI-6862`
- `knowledge/scorecard-template/README.md`
- `knowledge/scorecard-template/deliverables/scorecard-template-system-reference.md`
- `knowledge/scorecard-template/template-structure.md`
- `knowledge/scorecard-template/fe-template-builder.md`
- `knowledge/convi-6709-reversed-scorecard/README.md`
- `knowledge/export-appeal-comments/research.md`
- `director/packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx`
- `director/packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts`
- `director/packages/director-app/src/hooks/coaching/useGetScorecardTemplatePermissions.ts`
- `director/packages/director-app/src/features/admin/coaching/template-builder/steps/access/template-builder-advanced/TemplateBuilderAdvanced.tsx`
- `go-servers/apiserver/internal/coaching/util.go`
- `go-servers/apiserver/internal/coaching/action_update_scorecard.go`
- `go-servers/apiserver/internal/coaching/action_update_scorecard_test.go`
- `cresta-proto/cresta/v1/coaching/scorecard_template.proto`
- `cresta-proto/cresta/v1/coaching/scorecard.proto`
- Coda template link `https://coda.io/d/_dgqgUSgMKAr/Cresta-Eng-Design-Doc-Template_surZrf4s`

## Actions Summary

- Mapped the current scorecard template permission model and scorecard edit flow.
- Checked frontend read-only behavior for scorecards.
- Checked backend permission enforcement and submitted-scorecard update behavior.
- Checked scorecard type coverage for appeal and calibration flows.
- Attempted to fetch the Coda engineering design doc template.

## Findings

- Current product behavior allows updates to already-submitted normal scorecards.
- The normal scorecard edit path uses template `scorecardGraders`; appeal request uses `scorecardAppealers`; there is no dedicated post-submit edit role.
- A separate post-submit permission is the cleaner model because it avoids broadening existing `scorecardGraders`.
- The frontend advanced access UI contains `scorecardPublisherRoles`, but the coaching proto permission message currently only exposes editors, viewers, graders, and appealers.
- The referenced Coda template could not be read directly because the URL redirected to the Coda sign-in page in this environment.
- Product direction was clarified to a v1 hard lock: disable everything after submit first, but preserve design space for future finer-grained controls.

## Decisions Made

- Recommend a separate template permission for post-submit editing rather than reusing `scorecardGraders`.
- Start the engineering design doc locally under `deliverables/` until the Coda template contents are available.
- Shape the draft around a hard-lock v1 with future extension seams instead of designing the first version around granular exceptions.

## Follow-ups

- Decide whether to add the future post-submit permission schema in v1 or defer it.
- Identify any must-preserve workflows that need to survive the hard lock.
- Translate the local draft into the final Coda structure once the template is accessible.

## Links

- Linear: `https://linear.app/cresta/issue/CONVI-6862/disable-editing-on-submitted-scorecard`
- Coda template: `https://coda.io/d/_dgqgUSgMKAr/Cresta-Eng-Design-Doc-Template_surZrf4s`
