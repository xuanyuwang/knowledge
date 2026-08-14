# CONVI-7387 RetrieveHintStats fix

**Date:** 2026-08-04
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7387`
**Branch:** `convi-7387-discrepancy-in-user-adherence-data-between-insights-tools`
**Issue:** https://linear.app/cresta/issue/CONVI-7387/discrepancy-in-user-adherence-data-between-insights-tools
**PR:** https://github.com/cresta/go-servers/pull/30782

## Implementation

- Changed behavioral `hint_followed_count` from distinct `moment_annotation_id` to distinct `adherence_action_annotation_id`.
- Required a non-empty action annotation ID so the numerator uses the same hint-action identity as the denominator.
- Left KB and Guided Workflow follow counting unchanged.
- Updated all `RetrieveHintStats` golden SQL fixtures.
- Added a focused query regression test covering both default and action-annotation-based sent-query implementations.

## Validation

- `bazel test //insights-server/internal/analyticsimpl:retrieve_hint_stats_test` — passed.
- `git diff --check` — passed.
- Commit: `190ef15783`.

## Expected effect

For the reproduced Heartland week, the behavioral hint numerator changes from 215 positive moment annotations to 106 distinct followed hint actions, while the denominator remains 134 sent hint actions. The resulting hint follow rate is 79.1% instead of 160.4%.
