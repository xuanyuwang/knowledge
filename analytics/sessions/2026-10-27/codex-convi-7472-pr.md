# CONVI-7472 PR creation — October 27, 2026

Source repo: `/Users/xuanyu.wang/repos/director`.
Worktree: `/Users/xuanyu.wang/repos/director-convi-7472`.
Branch: `xw/convi-7472-adjusted-appeals`.
Base: `76f6b847418f7f287e9b598279348c1a7394c6e1`.

User authorized committing/pushing and creating a PR for the fix. Followed repository director-commit-and-pr skill: draft PR, concise description, remainder of docs/pull_request_template.md preserved exactly. No existing PR found for this branch.

Fresh verification before commit: all QM report tests passed (11 files / 59 tests); full director-app TypeScript passed; changed-file Biome and diff whitespace checks passed. No product changes since reviewed construction report. Commit contains only the component one-line fix and adjacent regression test.

GitHub operations use gh self-authentication and Git HTTPS through gh's credential helper; no SSH key or named AWS/Okta/Azure credential selected or accessed. Local Yarn commands use a non-secret placeholder solely for config interpolation; no dependency install needed.

## Outcome

- Commit `3620322dbc0584cb9ae35af630f965fb509b0a25`: `[CONVI-7472] Correct adjusted appeals chart series`.
- All commit hooks passed: i18n extraction, lint, i18next lint, format and commit-message check.
- Pushed branch using HTTPS and gh credential helper; local worktree clean.
- Created draft [Director PR #24302](https://github.com/cresta/director/pull/24302), base main; GitHub head SHA and exactly two intended files verified with gh pr view.
- Attached PR to this chat. Template preserved outside the description; QA/proof fields remain for completion before marking ready. CI, review, merge, deployment and GCC post-deployment validation remain.
- Construction report updated with the committed head/PR; source fingerprints unchanged. No Linear/Zendesk messages sent.
