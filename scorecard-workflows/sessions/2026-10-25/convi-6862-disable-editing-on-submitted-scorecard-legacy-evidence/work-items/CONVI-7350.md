# CONVI-7350: Use the latest template revision for permission evaluation

**Status:** validating  
**Primary domain:** `convi-6862-disable-editing-on-submitted-scorecard`  
**Primary subdomain:** none  
**Official ticket:** [CONVI-7350](https://linear.app/cresta/issue/CONVI-7350/refer-the-latest-revision-of-template-for-permission-evaluation)  
**Last updated:** 2026-07-22

## Objective and impact

- **Objective:** Make submitted-scorecard permission previews and backend write enforcement use the latest scorecard-template permissions.
- **Customer/system impact:** Prevent Director from presenting an editable submitted scorecard that apiserver later rejects with a 403 because the two paths authorized against different template revisions.
- **Role:** diagnosed, designed, and implemented.

## Scope

**In scope**

- Change `hasSubmittedScorecardEditPermission` to load the latest template revision internally.
- Remove its `scorecardTemplate` argument.
- Update the `UpdateScorecard` and `ResetScorecard` call sites.
- Add regression coverage for permission changes between a scorecard's pinned revision and the latest revision.

**Non-goals**

- Re-pin scorecards to the latest template revision.
- Change scorecard content or scoring semantics to use the latest revision.
- Make one gRPC endpoint call `EvaluateScorecardsPermissions`.
- Change the configuration-service behavior of `disableEditingOnSubmittedScorecards`.
- Address Director's separate fail-open behavior when the permission query errors.

## Source context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/director`
- **Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7350`
- **Branch:** `convi-7350-refer-the-latest-revision-of-template-for-permission`
- **PR/commits:** [go-servers #30336](https://github.com/cresta/go-servers/pull/30336), commit `3d580bd3b9`
- **Related prior PR:** [go-servers #29774](https://github.com/cresta/go-servers/pull/29774)
- **Related ticket:** [CONVI-7206](https://linear.app/cresta/issue/CONVI-7206)

## Current understanding

Implemented. Submitted-editor authorization for `UpdateScorecard` and `ResetScorecard` now loads the latest template revision through `hasSubmittedScorecardEditPermission`, matching `EvaluateScorecardsPermissions`. Pinned template revisions remain in use for scoring and scorecard content.

## Findings and decisions

- Root cause was latest-versus-pinned template revision selection across proactive evaluation and write enforcement.
- Chosen boundary: load latest revision inside `hasSubmittedScorecardEditPermission` rather than calling the Evaluate RPC from write handlers.
- Authorization fails closed if the latest template cannot be loaded.

## Validation and rollout

- Added update/reset allow and deny regression tests for scorecards pinned to `r1` with different `r2` submitted editors.
- Passed focused Bazel filters for the new tests and the full `TestUpdateScorecard`, `TestResetScorecard`, and `TestEvaluateScorecardsPermissions` suites.
- Remaining: open/land the CONVI-7350 PR and verify on Walter-dev with the reported scorecard.

## Next actions

1. Land [go-servers #30336](https://github.com/cresta/go-servers/pull/30336).
2. Manually verify the Walter-dev scorecard that previously showed `allowed: true` then update 403.
3. Optionally track Director fail-open on permission-query errors separately.

## Timeline

- 2026-07-22 — Confirmed that `EvaluateScorecardsPermissions` returned `allowed: true` for the affected requester and scorecard.
- 2026-07-22 — Traced the 403 to latest-versus-pinned template revision selection.
- 2026-07-22 — Agreed on refactoring `hasSubmittedScorecardEditPermission` to load the latest revision internally.
- 2026-07-22 — Populated [CONVI-7350](https://linear.app/cresta/issue/CONVI-7350/refer-the-latest-revision-of-template-for-permission-evaluation) with the root cause, implementation outline, acceptance criteria, and tests.
- 2026-07-22 — Implemented the helper refactor, call-site updates, and revision-mismatch regressions in `go-servers-convi-7350`.
