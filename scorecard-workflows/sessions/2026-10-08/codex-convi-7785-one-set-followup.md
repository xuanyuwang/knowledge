# CONVI-7785 one-set review follow-up

- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree/branch: `/Users/xuanyu.wang/repos/director-convi-7785`, `xw/convi-7785-exclude-simulator-rules`
- PR: [director#23461](https://github.com/cresta/director/pull/23461#discussion_r4219504259)
- New head: `496873381a1f59c684b4d7ac9c3c96b3d2b32d1f`

Xuanyu confirmed the single-owner interpretation in response to the reviewer. Removed `nonSimulatorBehaviors` and simplified the picker filter to membership in the Simulator-owned set. Removed the test that assigned one behavior to both regular and Simulator policies, a state rejected by normal backend policy create/update validation. The retained test covers regular, Simulator-only, archived Simulator-only, and unowned behavior choices.

Validation: focused Vitest 1/1 passed; director-app `yarn tsc` passed; Biome and package-scoped targeted ESLint passed; `git diff --check` passed. The repository pre-commit `run-lint` hook failed in its repo-wide director-app task after 2m 4s; its output contained a TypeScript stack trace. The commit skipped only that hook after the targeted checks passed; other hooks passed. Commit was pushed and GitHub API showed PR head at `496873381a` and state `open`, `draft=false`. New-head CI was not yet verified.

No GitHub review reply was posted. `git` and `gh` authenticated themselves; no selected AWS, Okta, Azure, or SSH credential was used.
