# CONVI-7941: Performance Insights overwritten downloads

**Status:** fix merged — Director PR #24681; local browser verification passed against RCG; production rollout not verified
**Primary domain:** analytics
**Primary subdomain:** performance-insights
**Official ticket:** [CONVI-7941](https://linear.app/cresta/issue/CONVI-7941/download-from-pi-on-of-overwrittens-not-wokring)
**Last updated:** 2026-11-06

## Objective and impact

Investigate missing or incorrect downloads when selecting Number of overwritten in Performance Insights. Diagnose using an existing Walter Dev browser session and read-only database queries.

## Current understanding

Director [PR #24681](https://github.com/cresta/director/pull/24681) merged on November 6 at 15:29:32 UTC, commit `f158f92549d76f84ad6e13aaba68e2205e2f3b78`. The shipped change awaits both progression CSV requests before ZIP generation and handles failures in one chain. Four controlled component regressions and two local Chrome exports against RCG passed, including a download after reload. The separate silent-error hook change was reverted and is excluded. Production deployment has not been verified.

**2026-11-05 RCG:** Reproduced an intermittent incomplete progression ZIP using the database-backed template TEST The Royal Way - Sales & Service [AQM], RCI CO Inbound Service, Last 30 days / Weekly. The first export contained only the by-agent CSV; an unchanged repeat contained both expected CSVs. Source starts both requests independently and generates the ZIP as soon as the by-agent request resolves, without awaiting the summary. Detailed overwritten download (including nonempty comment) and both leaderboard exports succeeded with count 1. This is a concrete defect on the requested surface; original reporter intent is still unverified. See [RCG reproduction](../sessions/2026-11-05/codex-convi-7941-rcg-reproduction.md).


The original ticket has no customer, template, date range, example file, screenshot, or comments. A representative Walter Dev case succeeds: template **06.10.25**, Last 30 days / Weekly, all audiences, exclude voicemail Yes, criterion **posit**, week **October 4–10, 2026**. The page and popover show **1** overwritten conversation; the detailed CSV has one matching row. All three table-level exports also download successfully with matching counts.

A separate confirmed frontend defect hides export failures: `useExportPerformanceInsightCriteria` clears notifications and calls the `getErrorHandler` factory without invoking its returned handler. This can make a failed export appear to silently stop. An expired-token error occurred during this investigation, but that does not establish the cause of the original ticket.

## Historical source context

- Primary source repo for this investigation: `/Users/xuanyu.wang/repos/director`, main checkout `34ad89e76d0050eaaf9ec6e8cf1b5148933ca84e`; no source edits.
- Implementation worktree: `/Users/xuanyu.wang/repos/director-convi-7941-export-errors`, branch `xw/convi-7941-export-errors`, based on `344235d59316f03440436e05335e56c3c86abf8a`. Local change only; no commit, push, PR, or deployment.
- Fetched current origin/main: `344235d593`; the detailed export hook is identical to the checked-out version. The relevant progression changes since the local checkout are styling only.
- Served Director asset path observed in console: `/director/s/main-8d3de48/`. Do not equate this deployed build with origin/main.
- Read-only backend reference: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl`.
- Current config: `origin/master:configv3/staging/cresta/walter-dev/config.yaml`.
- ClickHouse: `clickhouse-conversations.voice-staging.internal.cresta.ai`, database `cresta_walter_dev`, current score data in `score_d_read`.
- PostgreSQL: AWS account/cluster `voice-staging`, database `walter-dev`, `director.scorecard_templates`; connected through `cresta-cli connstring -i --read-only`.
- Explicit AWS profile used: `voice-staging_dev`, after reading its config entry and the associated SSO-session entry. No credentials were printed or saved.

## Verified results

| Surface | Result |
| --- | --- |
| Progression posit cell → Download scorecard | One-row detailed CSV, matching the popover count; repeated successfully after page refresh |
| Performance progression table download | ZIP contains both summary and by-agent CSVs; each has two rows, one for each overwritten criterion, with count 1 |
| Leaderboard by criteria table download | One agent, total overwritten 1, Mathias criterion 1, posit 1 |
| Leaderboard by criterion/time, posit | One agent, total 1, October 4 bucket 1 |

Detailed CSV includes Original Auto Score, Overwritten Score, and posit comment. Auto score `N/A` matches stored `ai_value=-1`; overwritten score is `Yes`; the comment is empty. This sample cannot validate export of a nonempty comment.

The table-level exports contain aggregates. The cell export contains individual conversation records; these are distinct existing capabilities.

## Confirmed progression ZIP root cause

The callback at the source revision matching RCG's build identifier (`6d5b707`) starts summary and by-agent requests independently and generates the ZIP in only the by-agent continuation. Real JSZip enumerates entries synchronously at generation start. A controlled harness executing the actual source callback confirms both response orders, including summary arriving before save but after generation (still missing), plus both request rejection cases. The inner catch covers ZIP generation only; summary rejection produces a partial archive, by-agent rejection produces no download, and neither request rejection reaches the export error handler. This path applies across progression metrics. History traces the split to April 15, 2024, Director PR #7414. [Root-cause evidence and runnable harness](../sessions/2026-11-05/codex-convi-7941-rcg-reproduction.md).

## Action items

1. Obtain the reported customer/template/date range and failing file or missing/wrong-column example before attributing the original report.
2. Per user request on November 5, reverted the separate detail-download silent-error change and removed its assistant-created test. That hook is identical to HEAD; its known error-handler defect remains outside this change.
3. Implemented and merged the progression ZIP repair in PR #24681: start both requests concurrently with `Promise.all`, add both CSVs after both requests succeed, then generate/save the archive. One outer catch covers request, formatting, ZIP generation, and saving failures. Four component export regressions control both response orders and either request failure; real JSZip archives are reopened to check both filenames. Merged as `f158f92549d76f84ad6e13aaba68e2205e2f3b78`; production deployment not verified.

## Evidence

- [Implementation session](../sessions/2026-11-02/codex-convi-7941-export-errors.md)
- [Detailed session](../sessions/2026-11-02/codex-convi-7941-overwritten-download.md)
- [Screenshot](../sessions/2026-11-02/convi-7941-overwritten-tables.jpg)
- Downloaded files are preserved under the session's `convi-7941-downloads/` directory.

## Timeline

- 2026-11-02 — Read ticket and related historical issues, found real overwrite templates through PostgreSQL/ClickHouse, tested detailed and all three table-level exports in Chrome, inspected CSV contents, and documented silent error handling. No source/data mutations or external comments.

- 2026-11-02 — User authorized a worktree implementation for a confident fix. Implemented the confirmed silent-error callback defect with two rejection regression cases; original customer download failure remains unconfirmed.

- 2026-11-05 — Found current RCG overwritten template/scorecard via read-only databases and reproduced intermittent missing summary CSV in progression ZIP (one incomplete, one complete archive). Detailed and both leaderboard CSVs match count 1. [Evidence](../sessions/2026-11-05/codex-convi-7941-rcg-reproduction.md).

- 2026-11-05 — Confirmed packaging race and uncaught request failures using actual callback from RCG build-source revision, real JSZip, and five controlled completion/failure scenarios. ZIP fix not implemented.

- 2026-11-05 — User requested reverting the silent-error fix and replacing it with await-both ZIP repair. Original detail hook restored; new progression regression reproduced early ZIP generation when by-agent resolves first, then all four cases passed with Promise.all. Final checks recorded in the session note.

- 2026-11-05 — User requested a draft PR. Committed as `0d6a4637368fbd112a001844d569140613a8d272`, pushed `xw/convi-7941-export-errors`, and created [draft PR #24681](https://github.com/cresta/director/pull/24681). Four export regressions passed again; all commit hooks passed. GitHub readback confirms draft state and only progression handler plus export test. Worktree clean; no deployment.

- 2026-11-06 — Investigated failing PR #24681 E2E job. Confirmed unrelated legacy-only transcript trace mock versus active shared tracing configuration from Playwright network artifact. No source changes; [diagnosis](../sessions/2026-11-06/codex-convi-7941-e2e-failure.md).

- 2026-11-06 — Final local Chrome test passed at commit `0d6a463736`: two progression downloads, including after reload, each contain both CSVs and the correct populated overwrite row. Production baseline still reproduced missing summary. [Final test evidence](../sessions/2026-11-06/codex-convi-7941-final-browser-test.md). No deployment.

- 2026-11-06 — GitHub confirms PR #24681 merged at 15:29:32 UTC as `f158f92549d76f84ad6e13aaba68e2205e2f3b78`. Recorded local verification and merged status; related knowledge documents prepared for commit/push at user request. Deployment remains unverified.
