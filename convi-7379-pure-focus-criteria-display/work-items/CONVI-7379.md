# CONVI-7379: Pure coaching sessions show raw focus-criterion IDs

**Status:** backend and frontend PRs open
**Primary domain:** coaching workflow
**Primary subdomain:** none
**Official ticket:** [CONVI-7379](https://linear.app/cresta/issue/CONVI-7379/pure-coaching-plan-11-sessions-shows-raw-focus-criteria-ids-instead-of)
**Last updated:** 2026-08-28

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
- **PRs/commits:** [go-servers #31048](https://github.com/cresta/go-servers/pull/31048), [Director #21757](https://github.com/cresta/director/pull/21757), BE commit `6ebc7a750f`, Director commits `cff1d057b3`, `6abb06d6b6`, `1de23b5612`, `21a4365c15`, and `ad5123c519`

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
- Manual FE testing found and `1de23b5612` fixes a trends regression: removed criteria with historical backend names are again hidden from the coaching-plan trends component, while session-note deactivated labels remain unchanged. Two focused regression tests and scoped ESLint pass.
- Review follow-up `21a4365c15` prevents stale selector option IDs from becoming identifier-less criteria, while preserving clone semantics and ordering for valid criteria. The original PR thread has been answered with validation evidence.
- CI follow-up `ad5123c519` commits the required Biome formatting and makes active trend-card names rollout-safe by preferring backend enrichment while falling back to current-template metadata. This fixes the `Criterion N/A` E2E regression without showing removed criteria.
- Product review selected `(deactivated)` suffix text instead of strikethrough for removed criteria retained in historical session notes. The frontend applies the localized suffix to collapsed focus/outcome pills, overflow tooltip titles, and the session criterion selector while preserving muted styling and the explanatory tooltip.

## Next Actions

1. Review PR #31048 at the replacement commit `6ebc7a750f`.
2. Review Director PR #21757 and add the required Before/After preview/proof artifacts after deployment with the backend dependency.
3. Reply to outstanding PR review threads, if needed, to point reviewers to the replacement read-only design.
4. Decide how to respond to the three new automated BE review threads, including the intentional strict malformed-ID policy.
5. Rerun the failed required CI jobs; current failures are runner disconnects and disk exhaustion rather than identified product-code failures.
6. Confirm whether a fresh human approval is required after the latest commits/merge from `main`.
7. Recheck the trends component in the deployed PR environment after commit `1de23b5612`.
8. Verify the `(deactivated)` suffix in the deployed historical-session preview; current-plan trends must continue hiding removed criteria.

## Timeline

- 2026-07-28 — Confirmed Pure stale criteria and current-template-only FE lookup. Evidence: `sessions/2026-07-28/cursor-convi-7379-investigation.md`.
- 2026-08-12 — Implemented the first revision-qualified persistence design. Evidence: `sessions/2026-08-12/claude-convi-7379-implementation.md`.
- 2026-08-13 — Review exposed mixed-format and target compatibility problems; selected and implemented read-path-only label recovery. Evidence: `decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md`, `sessions/2026-08-13/codex-pr-31048-review-findings.md`.
- 2026-08-13 — Replaced the BE branch's 14-commit revision-aware history with one read-path-only commit and force-pushed with an exact lease. Commit: `6ebc7a750f`.
- 2026-08-13 — Replaced PR #31048's obsolete revision-aware description with the final read-path-only contract, compatibility rationale, implementation scope, and validation plan.
- 2026-08-14 — Added the plan-history fallback, consolidated the FE work into one commit, passed 17 focused tests and scoped ESLint, opened Director PR #21757, then corrected its unrelated #21645 base by rebasing the unchanged patch as `cff1d057b3` directly onto `main`.
- 2026-08-14 — Validated automated PR feedback, fixed selector value loss and outcome-option misclassification, added regression coverage, and pushed `6abb06d6b6`; 18 focused tests and scoped validation pass.
- 2026-08-17 — Evaluated the two unresolved BE review threads and implemented their cleanup locally. Focused backend tests pass and CodeRabbit raised 0 issues on the uncommitted diff.
- 2026-08-17 — Committed the two BE review cleanups as `f3a4fc7396`; Gazelle completed successfully without additional generated changes. The commit remains local and unpushed.
- 2026-08-17 — Selected strict malformed-ID error exposure instead of fail-open skipping. Added local tests for the error contract and rejected empty criterion IDs on writes; focused tests pass and CodeRabbit raised 0 issues.
- 2026-08-17 — Committed strict malformed-ID handling as `9bf8c91b28`, pushed both BE follow-up commits to PR #31048, and replied to both unresolved human threads. PR head is `9bf8c91b28`.
- 2026-08-17 — PR head advanced to merge commit `c0ee0003e9`. The two human threads are resolved; three automated threads are newly open. Required CI failed because three self-hosted runners disconnected and another exhausted disk while linking an unrelated test.
- 2026-08-18 — Manual FE testing found removed criteria visible in coaching-plan trends. Restored current-template membership as the visibility gate, retained the ALO exception, added regression tests, and pushed Director commit `1de23b5612`.
- 2026-08-18 — Validated and implemented the stale option-ID review suggestion, pushed `21a4365c15`, and replied to the original thread; 14 focused tests pass and CodeRabbit raised 0 issues.
- 2026-08-18 — Diagnosed lint and coaching-plan E2E failures, committed the pending Biome change, restored an active-criterion label fallback for frontend-first rollout, and pushed `ad5123c519`; replacement CI is running.
- 2026-08-28 — Read the product-review Slack context and replaced historical-session strikethrough/`[Deactivated]` presentation with the localized `(deactivated)` suffix. Focused tests, changed-file lint, TypeScript, formatting, and diff checks pass locally.
