# CONVI-7785: Exclude Training Simulator-only Opera rules from Performance Configuration

**Status:** validating
**Primary domain:** `scorecard-workflows`
**Primary subdomain:** `template-authoring-and-versioning`
**Official ticket:** [CONVI-7785](https://linear.app/cresta/issue/CONVI-7785/exclude-training-simulator-only-opera-rules-from-performance)
**Last updated:** 2026-10-02

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
- **PRs/commits:** none yet.

## Current Understanding

The Performance Configuration conversation template builder's AutoQA picker used `RetrieveBehaviors`, whose behavior representation has no policy source type. It already joined `ListPolicies` data to behaviors for role visibility and policy links, but did not exclude behaviors owned by `SOURCE_TYPE_TRAINING_SIMULATOR` policies. Director source changes now exclude those behaviors in `getTriggerOptions`. This is a frontend-only picker change.

## Findings and Decisions

- `useAutoQATriggers` loads all behavior statuses; its normal policy preset included active and inactive Opera policies but not archived ones. The template picker now requests archived policies too, so the admin option that shows archived behaviors cannot leak archived Simulator-only rules.
- The picker waits for both policy and behavior results before offering options. Other `useAutoQATriggers` consumers retain their existing query and loading behavior.
- Legacy behaviors with no matching policy remain available, preserving the current choice set where ownership cannot be established.

## Blockers and Dependencies

- No code blocker. Live preview or customer data was not used to verify deployed behavior.

## Validation and Rollout

- Focused `getTriggerOptions.test.ts`: passed, covering normal, Simulator-only, archived Simulator-only, and legacy behavior options.
- Director app `yarn tsc`: passed.
- Targeted Biome and ESLint: passed. `git diff --check`: passed.
- No commit, push, or PR yet.

## Next Actions

1. Review the Director diff, then commit/push and open a PR if desired.
2. Verify the template picker against a profile with a Training Simulator-only Opera rule in preview or staging.

## Timeline

- 2026-10-02 — Traced the template creation picker and implemented frontend source filtering. Evidence: `sessions/2026-10-02/codex-convi-7785-template-picker.md`.
- 2026-10-02 — Moved the branch and its uncommitted changes into a dedicated worktree; the main Director checkout is clean.
