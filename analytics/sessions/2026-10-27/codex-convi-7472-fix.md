# CONVI-7472 frontend fix — October 27, 2026

Source repo: `/Users/xuanyu.wang/repos/director`.
Worktree: `/Users/xuanyu.wang/repos/director-convi-7472`.
Branch: `xw/convi-7472-adjusted-appeals`.
Base: freshly fetched origin/main `76f6b847418f7f287e9b598279348c1a7394c6e1`.

User authorized the previously proposed frontend mapping fix and regression test in a new worktree. Native creation failed because the workspace root Git repo does not contain the Director ref; used Director's own git worktree command. The source still computes adjustedData but passes resolvedData to adjustedSeries. Plan: regression test through real API hook and component chart-options output, then one-line mapping fix; no API/schema or other report changes.

Dependency install initially failed with GitHub Packages 403 using gh's self-authenticated token. The immutable install succeeded using the existing npm registry credential without printing or persisting it. No lockfile change. No AWS, Okta, Azure, or SSH credential was selected or accessed.


## Implementation and verification

Confirmed the defect on freshly fetched main. `QAReportAppealTrendsCard.tsx` computes adjustedData from adjustedConversationCount, but passes resolvedData into the Adjusted series. Changed that one reference to adjustedData.

Added adjacent `QAReportAppealTrendsCard.test.tsx`, exercising the real component/API hook with a spy on retrieveAppealStats and capturing chart options at HighchartsWithSpinner. Three cases keep Appealed and Resolved at 10 and vary Adjusted between 3, 0, and omitted (expected fallback 0). The corrected regression fixture failed all three cases before the production fix (received 10), then passed after it. An earlier fixture attempt had mismatched local/UTC range instants; fixed the fixture before recording the useful red run.

- All QM report tests: 11 files / 59 tests passed, including the three new cases.
- Full director-app TypeScript (`yarn tsc`): passed.
- ESLint for both changed files with `--max-warnings=0`: passed.
- Biome check for both changed files: passed.
- `git diff --check`: passed.
- Node 24.11.1 and repository Yarn 4.18.1 used. Existing SDK sourcemap warnings occurred during tests.

Source remains local and uncommitted: one production line plus the adjacent test. No push, PR, deployment, or external message. Post-deployment GCC validation remains required. Original Director checkout was untouched. Credentials used during setup: gh self-authentication (registry request rejected) and existing npm GitHub Packages credential (successful install); no credential values recorded. No AWS, Okta, Azure, or SSH credentials accessed.

## Review

Read-only reviewer found no actionable correctness issues. Applied the optional fixture improvement: adjustedCriteriaCount is now 7, deliberately different from adjustedConversationCount, so the tests distinguish the two API fields. Full QM report suite and final focused tests rerun after review. A first edit command used a repo-relative path from the package directory and failed without changing files; corrected the path before final verification.
