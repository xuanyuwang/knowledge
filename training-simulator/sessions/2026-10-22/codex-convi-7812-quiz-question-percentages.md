# CONVI-7812 quiz question percentages

User requested a new PR linked to the same ticket to close the quiz-question display gap.

Design: show correctResultCount / answeredResultCount using the existing percentage formatter; preserve the no-answer placeholder, authored question order, template-based row visibility, and backend data. Reuse the existing Pass rate heading for both kinds. Shared module view covers Modules drawer and Lessons module drilldown.

Source: /Users/xuanyu.wang/repos/director-convi-7812, new branch xw/convi-7812-quiz-question-pass-rates from origin/main 6c25ce0789. Existing linked worktree was clean and reused.

Validation in progress: refresh installed dependencies to current lockfile; then baseline, failing percentage expectations, minimal implementation, focused suites, lint/type checking, commit hooks.

Credentials: used existing GPR_TOKEN for GitHub Packages after unused placeholder and gh token could not fetch current packages. Used gh/git existing authentication. No AWS, Okta, Azure, or specific SSH credential selected or read.

Validation results: baseline 25 tests passed; updated percentage expectations produced 4 expected failures against original fraction formatting; implementation passes 27 tests across moduleStatsView and lessonStatsView. Director app TypeScript, changed-file ESLint and Biome passed. No code or lockfile changes were needed for dependency setup. Vitest runs serially because simultaneous workers share Node local-storage SQLite files and initially produced a database-locked error. Browser proof/videos remain pending; opening a draft PR according to director-commit-and-pr instructions.

Committed f740a2386f, all commit hooks passed, pushed branch, opened draft [director#24097](https://github.com/cresta/director/pull/24097) linked to CONVI-7812 in title and description. Worktree clean after commit.
