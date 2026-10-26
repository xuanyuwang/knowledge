# RCG submitted scorecard edit-permission investigation

**Date:** 2026-08-25  
**Scope:** Read-only investigation across go-servers, director, config, cresta-proto  
**Note:** `/Users/xuanyu.wang/repos/go-servers-convi-7206` is absent locally; CONVI-7206/7350 are merged into `/Users/xuanyu.wang/repos/go-servers`. Closest related checkout: `go-servers-ga-submitted-scorecard-lock` (lacks FF gating vs main).

## Contract (critical)

`submitted_scorecard_editors` / `submittedScorecardEditors` is a **UserTeamGroup** (`users` + `teams` + `groups`), **not** a role list. Role-based grading (`scorecard_graders`, which defaults to include `QA_SPECIALIST`) is a **separate** field and is **replaced** (not OR’d) once a non-empty submitted-editors allowlist is configured.

## Decision path

1. FE gate: `disableEditingOnSubmittedScorecards` + submitted scorecard → `EvaluateScorecardsPermissions(MODIFY_SUBMITTED_LOCKED_SCORECARD)`.
2. BE evaluator expands allowlist via user-filter parser; membership = exact `requester.Name` ∈ `parsedFilter.UserNames`.
3. If restriction applies: return allowlist membership only (deny role bypass, including QA_ADMIN / QA_SPECIALIST).
4. If no restriction: fall back to `HasScorecardEditPermission` (role defaults / `scorecard_graders`).

## Canonical production target

- Submitted scorecard: `customers/rcg/profiles/us-east-1/scorecards/019f386e-a934-7271-93ae-838feb4468be`
- Conversation: `019f28b8-53d3-77b3-8ba0-d611106e39cb`
- Pinned template: `customers/rcg/profiles/us-east-1/scorecardTemplates/019f1b3c-ee76-724c-9dd5-020f2f41955c@28a6df1c`

RCG app and auth databases were queried through the approved read-only connectors.

- Template revisions are `dfd5fa4f` (active), pinned `28a6df1c` (active), `8e1102c3` (archived), and latest `b457423f` (inactive/deactivated Aug 21).
- No revision of template resource `019f1b3c-ee76-724c-9dd5-020f2f41955c` has configured submitted editors.
- Multiple later, similarly named template resources exist; several active replacements have 31 submitted editors. `EvaluateScorecardsPermissions` does not follow those resources by title—it loads the latest revision of the scorecard's stored template ID.
- Quality Assurance 40 (`829fd45d23e3d691`) and Quality Assurance 44 (`83f68adf7584b433`) each have one active, non-migrated identity enabled for `us-east-1`.
- QA 40 has custom QA Admin and QA Specialist roles; QA 44 has custom QA Specialist.

## Live permission API verification

Production `EvaluateScorecardsPermissions` was called twice for the canonical scorecard and `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`:

- Quality Assurance 40: `allowed: true`
- Quality Assurance 44: `allowed: true`

Because this template resource has no submitted-editor allowlist, the evaluator falls back to normal role-based edit permission. Both named users pass. The reported inability to edit this scorecard is therefore not reproduced by the backend submitted-scorecard permission API; investigate a separate frontend lock or collect the users' own failing network response.
