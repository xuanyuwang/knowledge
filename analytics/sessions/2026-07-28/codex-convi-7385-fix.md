# CONVI-7385 Frontend Fix

## Context

- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7385`
- Branch: `convi-7385-qa-scorecard-api-count-mismatch-between-column-and-drawer`
- Base: `origin/main` at `c026adba8b`
- Commit: `506de9db6f`
- Draft PR: [cresta/director#21214](https://github.com/cresta/director/pull/21214)
- Plan: `deliverables/convi-7385-column-drawer-count-parity-plan.md`

## Implementation

- Agent drawer now preserves the page `voicemailMoment` by no longer overriding it after spreading page filters.
- Manager drawer explicitly passes through `filtersState.voicemailMoment`.
- Manager submitted-only scorecard-resource and submitter-audience semantics remain unchanged.
- No new tests were added, per implementation direction.

## Filter-Parity Audit

- Agent drawer spreads the complete QA filter state and only intentionally narrows `usersTeamsGroups` to the selected agent. Removing the voicemail override restores the dropped shared filter without changing duration, N/A, scorecard, usecase/category, or deactivated-user fields.
- Manager drawer intentionally reconstructs a narrower state for submitted scorecards and submitter attribution. It already preserved duration and N/A fields; this change adds voicemail while retaining `MANUALLY_SUBMITTED`, scorecard resource, empty subject selection, and reviewer-audience narrowing.
- Latest `origin/main` exposed an additional same-class issue not captured by the original plan: Manager column request construction also omitted `voicemailMoment`. Updating only the drawer would have introduced reverse mismatch, so `ManagerLeaderboardPage` now passes voicemail to the Manager column QA requests as well.
- Updated the shared Leaderboard QA filter-state type from `CommonInsightsFiltersState` to `PerformanceFiltersState` so `voicemailMoment` is part of the compile-time contract instead of surviving only through object spread.
- `listAgentOnly` remains a pre-existing API asymmetry: `RetrieveQAScoreStats` supports `filterToAgentsOnly`, while `RetrieveQAConversations` has no equivalent request field. It does not affect this selected-agent voicemail repro and is left as a non-blocking follow-up rather than expanding CONVI-7385.

## Validation

- `git diff --check`: passed.
- Biome formatting/check on all changed TypeScript files: passed using the installed main-checkout binary and configuration.
- CodeRabbit CLI review returned zero findings; the initially drafted test files were subsequently removed per implementation direction.
- After GitHub Packages authentication was refreshed, `yarn install --immutable`, `yarn lint:precommit`, and `yarn tsc` passed.
- Required commit hooks passed without bypasses, including i18n extraction, lint, i18next lint, formatting, and dependency checks.
- Commit `506de9db6f` was pushed and draft PR [#21214](https://github.com/cresta/director/pull/21214) was opened.
- Manual CNG browser/network validation remains pending.

## Next Steps

1. Validate the CNG Agnes Long repro and Manager network payload.
2. Add preview/proof and QA-selection details to the draft PR.
3. Resolve review/CI feedback and complete the ticket.
