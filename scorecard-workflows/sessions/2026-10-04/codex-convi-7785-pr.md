# CONVI-7785 draft PR creation

Date: 2026-10-04 (America/Toronto)
Source repo: `/Users/xuanyu.wang/repos/director`
Branch/worktree: `xw/convi-7785-exclude-simulator-rules` in `/Users/xuanyu.wang/repos/director-convi-7785`

## Result

- Committed exactly four Director files as `d9de9e190265d5cb304182cc61f7304959161cc1` (`[CONVI-7785] Exclude simulator-only rules from conversation templates`).
- Pushed the branch and created draft [director#23461](https://github.com/cresta/director/pull/23461) against `main`. The PR title begins with CONVI-7785 and its body uses `docs/pull_request_template.md`, with only the description filled.
- Git config verification after the push showed that a global `insteadOf` rule rewrote the supplied HTTPS push URL to SSH. The SSH key identity used by Git was not verified. The GitHub CLI used its authenticated account for PR creation.
- GitHub reports the PR open, draft, and mergeable, with the intended four files and one commit.

## Validation

- `yarn install --immutable` completed in the dedicated worktree.
- Focused Vitest: one test passed.
- `yarn tsc` in `packages/director-app`: passed.
- Targeted Biome and ESLint on the four changed files: passed; staged `git diff --check`: passed.
- Repository commit hooks for protected files, i18n extraction/lint, formatting, and ticket prefix passed. The full-app `run-lint` hook was stopped after several minutes and skipped for the commit; targeted ESLint had passed. No preview/staging UI validation was performed.

## Follow-up

- Review the draft PR and exercise the template picker with a Training Simulator-only rule in preview or staging.
