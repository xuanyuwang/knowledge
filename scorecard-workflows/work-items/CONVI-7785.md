# CONVI-7785: Exclude Training Simulator-only Opera rules from Performance Configuration

**Status:** Ready for review; codeowner review pending
**Primary domain:** `scorecard-workflows`
**Primary subdomain:** `template-authoring-and-versioning`
**Official ticket:** [CONVI-7785](https://linear.app/cresta/issue/CONVI-7785/exclude-training-simulator-only-opera-rules-from-performance)
**Last updated:** 2026-10-07

## Objective and Impact

- **Objective:** Keep Training Simulator-only Opera behaviors out of the AutoQA trigger choices when authoring a conversation scorecard template in Performance Configuration.
- **Customer/system impact:** Prevents a rule that runs only in Training Simulator from being selected for ordinary conversation scoring.
- **Role:** diagnosed and implemented.

## Scope

**In scope**

- Director template builder trigger options and their policy lookup.
- Preserve existing non-Simulator and unowned behavior choices.

**Non-goals**

- Opera rule authoring, Training Simulator module criteria, and backend runtime policy selection.

## Source Context

- **Repo:** `/Users/xuanyu.wang/repos/director`
- **Worktree:** `/Users/xuanyu.wang/repos/director-convi-7785`
- **Branch:** `xw/convi-7785-exclude-simulator-rules`
- **PRs/commits:** [director#23461](https://github.com/cresta/director/pull/23461), commits `d9de9e1902`, `3a852b97ab`, and current head `496873381a`.

## Current Understanding

The Performance Configuration conversation template builder's AutoQA picker used `RetrieveBehaviors`, whose behavior representation has no policy source type. It already joined `ListPolicies` data to behaviors for role visibility and policy links, but did not exclude behaviors owned by `SOURCE_TYPE_TRAINING_SIMULATOR` policies. Director source changes now exclude those behaviors in `getTriggerOptions`. This is a frontend-only picker change.

## Findings and Decisions

- `useAutoQATriggers` loads all behavior statuses; its normal policy preset included active and inactive Opera policies but not archived ones. The template picker now requests archived policies too, so the admin option that shows archived behaviors cannot leak archived Simulator-only rules.
- The picker waits for both policy and behavior results before offering options. Other `useAutoQATriggers` consumers retain their existing query and loading behavior.
- Legacy behaviors with no matching policy remain available, preserving the current choice set where ownership cannot be established.
- An earlier review prompted commit `3a852b97ab` to preserve a behavior shared by Simulator and regular policies. Normal backend writes reject that shared-ownership state, so commit `496873381a` simplified the picker back to one Simulator-owned set and removed its impossible-state test.
- `enableTrainingSimulator` gates the Simulator authoring UI, while the template picker must reject existing Simulator-only policy behaviors regardless of that flag. The filter remains unconditional after the 2026-10-08 discussion.
- Review of the two-set shared-ownership guard: current policy create/update validation rejects behavior IDs used by another policy in the same customer/profile, including archived policies. The reviewer is right that one Simulator-owned set is sufficient for normal API data; the callback `return` alone does not establish exclusivity. This is an API invariant, not a database uniqueness constraint. Evidence: `sessions/2026-10-08/codex-convi-7785-shared-ownership-review.md`.

## Blockers and Dependencies

- The one-set follow-up is pushed. New-head CI and codeowner review are pending verification.

## Validation and Rollout

- Focused `getTriggerOptions.test.ts`: passed, covering normal, Simulator-only, archived Simulator-only, and legacy behavior options.
- Director app `yarn tsc`: passed.
- Targeted Biome and ESLint: passed. `git diff --check`: passed.
- Commit `d9de9e1902` pushed; draft PR #23461 is open and GitHub reports it mergeable.
- On Walter Dev, an active rule scoped to **Only Training Simulator** supplied `CONVI-7785 Simulator-only QA behavior`. Baseline offered the behavior among 176 choices; PR preview `23461_merge-d9de9e1` omitted it among 175 choices and retained other choices. The browser comparison used the deployed PR SHA `d9de9e1902`.
- The PR body now includes Before and After inline video attachments and `smoke-functional` QA fields. The PR is ready for review. QA metadata, build, tests, lint, typecheck, E2E, and CodeRabbit approval passed on `3a852b97ab`; codeowner check remains pending.
- The uploaded After clip shows a local branch build connected to Walter Dev but ends before the empty search-result state is visible. The PR caption distinguishes that clip from the independently verified deployed-preview comparison. A clearer replacement could not be recorded while the Mac was locked.
- The shared-ownership regression test passes (2 focused tests). Package TypeScript, targeted ESLint, Biome, and `git diff --check` pass for the follow-up. Details: `sessions/2026-10-07/codex-convi-7785-review-fix.md`.
- The repository pre-commit hook's full-app `run-lint` remained running after several minutes and was stopped. The commit skipped only that hook; targeted ESLint on all four ticket files passed. Other commit hooks passed.
- On `496873381a`, the remaining focused test passed (1/1), director-app TypeScript, Biome, package-scoped ESLint, and `git diff --check` passed. The repo-wide `run-lint` hook failed in director-app after 2m 4s; the commit skipped only that hook. See `sessions/2026-10-08/codex-convi-7785-one-set-followup.md`.

## Next Actions

1. Verify new-head CI and obtain codeowner approval before merge.

## Timeline

- 2026-10-02 — Traced the template creation picker and implemented frontend source filtering. Evidence: `sessions/2026-10-02/codex-convi-7785-template-picker.md`.
- 2026-10-02 — Moved the branch and its uncommitted changes into a dedicated worktree; the main Director checkout is clean.
- 2026-10-04 — Revalidated the worktree, committed and pushed the four-file change, and opened draft Director PR #23461. Evidence: `sessions/2026-10-04/codex-convi-7785-pr.md`.
- 2026-10-07 — Verified baseline and deployed PR preview behavior on Walter Dev; QA recordings and PR metadata remain pending. Evidence: `sessions/2026-10-07/codex-convi-7785-review-evidence.md`.
- 2026-10-07 — Video links and QA metadata were added, PR marked ready, and the shared-ownership review fix pushed as `3a852b97ab`. Evidence: `sessions/2026-10-07/codex-convi-7785-review-fix.md`.
- 2026-10-08 — Considered `enableTrainingSimulator` gating and kept the picker filter unconditional; reverted the uncommitted gate. Evidence: `sessions/2026-10-08/codex-convi-7785-flag-decision.md`.
- 2026-10-08 — Simplified the picker to one Simulator-owned set and pushed `496873381a` in response to shared-ownership review. Evidence: `sessions/2026-10-08/codex-convi-7785-one-set-followup.md`.
