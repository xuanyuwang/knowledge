# CONVI-7379: Pure coaching sessions show raw focus-criterion IDs

**Status:** backend and frontend PRs open
**Primary domain:** coaching workflow
**Primary subdomain:** none
**Official ticket:** [CONVI-7379](https://linear.app/cresta/issue/CONVI-7379/pure-coaching-plan-11-sessions-shows-raw-focus-criteria-ids-instead-of)
**Last updated:** 2026-08-14

## Objective and Impact

- **Objective:** Show human-readable names for historical/deactivated focus criteria in Pure coaching-plan 1:1 sessions.
- **Customer/system impact:** Pure has active coaching plans referencing criteria removed from the current Security template; current-template-only FE lookup renders raw identifiers.
- **Role:** diagnosed, designed, implemented, reviewed

## Scope

**In scope**

- Populate `criterionDisplayName` on relevant coaching-plan and coaching-session reads.
- Recover labels from the newest historical template revision containing the criterion.
- Update FE plan/session surfaces to consume backend-provided names.
- Preserve compatibility with revision-independent targets and existing stored focus criteria.

**Non-goals**

- Establish the exact historical template revision used when a plan/session was created.
- Change focus-criterion persistence to `template@revision/criterion`.
- Migrate existing coaching rows or targets.
- Define revision-specific coaching-plan filtering semantics.

## Source Context

- **Repos:** `go-servers`, `director`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7379`, `/Users/xuanyu.wang/repos/director-convi-7379`
- **Branches:** `convi-7379-focus-criteria-display-names-be`, `xw/convi-7379-focus-criteria-display-names-fe`
- **PRs/commits:** [go-servers #31048](https://github.com/cresta/go-servers/pull/31048), [Director #21757](https://github.com/cresta/director/pull/21757), BE commit `6ebc7a750f`, Director commits `cff1d057b3` and `6abb06d6b6`

## Current Understanding

Focus criteria and targets share revision-independent logical identity `(templateID, criterionID)`. The minimal compatible fix is to leave writes unchanged and recover `criterionDisplayName` on reads from the newest template revision containing that criterion. Responses retain `T@*` because the label lookup revision is not historical provenance.

## Findings and Decisions

- Pure's stale criterion IDs exist in historical Security template revisions, so read-time recovery fixes the reported display failure.
- The initial revision-qualified write design introduced mixed `T/C` and `T@R/C` rows.
- `director.targets` has no template revision, and target-based ListCoachingPlans callers are conceptually wildcard/revision-independent.
- Supporting mixed formats would require SQL normalization, a migration policy, overview changes, and coordinated FE filter semantics.
- Decision: fix the read path only. See [decision record](../decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md).
- FE may drop criterion-name lookup maps only on surfaces receiving enriched coaching plan/session criteria; other ID-only APIs still need current-template lookup.

## Blockers and Dependencies

- Assumes criterion IDs are not reused for different meanings within the same template.
- Mutation responses must remain visually consistent through enrichment or refetch/merge behavior.

## Validation and Rollout

- Backend regression coverage verifies newest-containing-revision selection, removed criteria, wildcard response names, and unchanged write format.
- Affected backend endpoint suites passed.
- FE option/tooltip unit tests passed (18 tests); changed-file ESLint, formatting, and `git diff --check` passed.
- The BE branch was rewritten and force-pushed as one read-path-only commit (`6ebc7a750f`). FE follow-up remains in its dedicated worktree.
- The FE branch was consolidated onto current `main` and opened as Director PR #21757. Review fixes in `6abb06d6b6` preserve the full criterion value through selector changes and exclude missing-current-template criteria from outcome options.

## Next Actions

1. Review PR #31048 at the replacement commit `6ebc7a750f`.
2. Review Director PR #21757 and add the required Before/After preview/proof artifacts after deployment with the backend dependency.
3. Reply to outstanding PR review threads, if needed, to point reviewers to the replacement read-only design.

## Timeline

- 2026-07-28 — Confirmed Pure stale criteria and current-template-only FE lookup. Evidence: `sessions/2026-07-28/cursor-convi-7379-investigation.md`.
- 2026-08-12 — Implemented the first revision-qualified persistence design. Evidence: `sessions/2026-08-12/claude-convi-7379-implementation.md`.
- 2026-08-13 — Review exposed mixed-format and target compatibility problems; selected and implemented read-path-only label recovery. Evidence: `decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md`, `sessions/2026-08-13/codex-pr-31048-review-findings.md`.
- 2026-08-13 — Replaced the BE branch's 14-commit revision-aware history with one read-path-only commit and force-pushed with an exact lease. Commit: `6ebc7a750f`.
- 2026-08-13 — Replaced PR #31048's obsolete revision-aware description with the final read-path-only contract, compatibility rationale, implementation scope, and validation plan.
- 2026-08-14 — Added the plan-history fallback, consolidated the FE work into one commit, passed 17 focused tests and scoped ESLint, opened Director PR #21757, then corrected its unrelated #21645 base by rebasing the unchanged patch as `cff1d057b3` directly onto `main`.
- 2026-08-14 — Validated automated PR feedback, fixed selector value loss and outcome-option misclassification, added regression coverage, and pushed `6abb06d6b6`; 18 focused tests and scoped validation pass.
