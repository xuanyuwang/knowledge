# CONVI-7777 column label and definition tooltip

- Date: 2026-10-22 (America/Toronto).
- Source repo: /Users/xuanyu.wang/repos/director.
- Worktree: /Users/xuanyu.wang/repos/director-convi-7777-assistance-usage.
- Branch: xw/convi-7777-assistance-usage; starting HEAD 18bd08da904af8624dc69e7d8509d04564801365.
- Request: Update existing PR https://github.com/cresta/director/pull/24020 to shorten Conversations to Convos and explain the metric in a tooltip.
- Evidence: Glean read of CONVI-7777 and current protobuf conversation_assistance_usage_stats.proto in cresta-proto-convi-7777-assistance-usage. Ten qualifying categories; once per agent/conversation; Team sums member agent counts. Reminder, Silence/Hold, Smart Compose and email assistance excluded.
- Approach: SharedTooltip in each table header, headerDisplayName metadata to retain column selector label, consistent shorter CSV heading. Use source translations and generated en-US extraction. No new UI chrome tests, per Director AGENTS.md.
- Validation: focused column/CSV tests 5/5 passed, Director app TypeScript build passed, ESLint and Biome passed for four changed source/test files, full app Babel i18n extraction compiled 7,321 files successfully, git diff --check passed. Yarn workspace script could not resolve rimraf; ran the same checked-in Babel extraction using local binaries under Node 24.11.1. No browser visual validation.
- Authentication: gh/git self-authentication only; no credentials or explicit AWS/Okta/Azure/SSH credential selections were read or supplied.
- Publication: commit a9b7d6f3fe pushed to existing PR #24020. All commit hooks passed (i18n extraction, lint, i18next lint, format). Remote CI not yet verified.
