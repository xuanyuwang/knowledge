# CONVI-7785 feature flag decision

- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree/branch: `/Users/xuanyu.wang/repos/director-convi-7785`, `xw/convi-7785-exclude-simulator-rules`
- PR: [director#23461](https://github.com/cresta/director/pull/23461)

The user first requested guarding the picker change with `enableTrainingSimulator`, then reconsidered because Simulator-only policies must always be excluded from conversation templates. Source inspection showed `PolicySourceTypeSelect` uses the flag to show or hide the Opera authoring control, while `getTriggerOptions` filters policies already returned to the conversation-template picker. A policy's source type remains relevant to conversation scoring even if the Simulator UI flag is off. Therefore the filter and archived-policy ownership lookup remain unconditional.

An uncommitted flag-gating change was made in three Director files and passed the focused test, typecheck, Biome, and targeted ESLint. After the user's clarification, those exact edits were restored. The ticket worktree is clean at PR head `3a852b97ab`; no commit or PR update was made for the abandoned gate.

No selected AWS, Okta, Azure, or SSH credential was used. `gh` and `git` authenticated themselves.
