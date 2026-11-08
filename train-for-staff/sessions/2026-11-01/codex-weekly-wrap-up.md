# Weekly wrap-up: October 26–November 1, 2026

- **Date / timezone:** 2026-11-01, America/Toronto
- **Primary project:** train-for-staff (weekly engineering evidence synthesis)
- **Source repo:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch / worktree:** `main`, main checkout
- **Role:** Synthesized dated records and reconciled current GitHub and Linear status.
- **Output:** [Weekly summary](../../../weekly-summary/weekly-summary-2026-10-26-to-2026-11-01.md)

## Coverage and method

Reviewed all eight domain daily logs in the week: Analytics October 26–30, Training Simulator October 26–27, and Scorecard Workflows October 26. Associated sessions and canonical work items supplied implementation, investigation, and validation evidence. November 1 read-only GitHub and Linear checks updated stale local status. No dated product-work logs were found for October 31 or November 1 before this capture.

Applied the repository weekly-summary and daily-capture skills. Used the previous week's summary to identify carryover, not as proof of current delivery. Kept the established train-for-staff capture location without adding career or promotion objectives. Preserved all pre-existing working-tree changes.

## Current status and evidence

| Work | November 1 status | Evidence and limits |
| --- | --- | --- |
| CONVI-7810: CSV exports | Linear Done; contract, backend, frontend, and smoke-test PRs merged | [Proto #10413](https://github.com/cresta/cresta-proto/pull/10413), [backend #34647](https://github.com/cresta/go-servers/pull/34647), [frontend #24328](https://github.com/cresta/director/pull/24328), [smoke coverage #24411](https://github.com/cresta/director/pull/24411). Production enablement and large-export performance not established. |
| CONVI-7472: GCC appeals | Linear Done; correction merged | [Director #24302](https://github.com/cresta/director/pull/24302). Deployed customer build not checked in this wrap-up. |
| CONVI-7879: Assistance tracing | Linear Done; backend merged, drawer closed unmerged | [Backend #34417](https://github.com/cresta/go-servers/pull/34417), [drawer #24150](https://github.com/cresta/director/pull/24150). Done does not establish frontend delivery. Closure reason not established. |
| CONVI-7852: AdventHealth team label | Linear In Progress; local changes, no PR | [Canonical work item](../../../analytics/work-items/CONVI-7852.md) and [implementation session](../../../analytics/sessions/2026-10-30/codex-convi-7852-agent-team-fix.md). Local worktree status refreshed read-only. |
| CONVI-7931: Oportun counts | Linear In Progress; diagnosis only | [Canonical work item](../../../analytics/work-items/CONVI-7931.md) and [investigation](../../../analytics/sessions/2026-10-29/codex-convi-7931-oportun-process-count.md). No repair performed. |
| CONVI-7874: Dynamic groups | Linear QA Review; collaborator's PR merged | [Go-servers #34439](https://github.com/cresta/go-servers/pull/34439), [review session](../../../analytics/sessions/2026-10-26/codex-convi-7874-semantic-review.md), [report](../../../analytics/deliverables/convi-7874-semantic-review.html). Final head differs from reviewed head. |
| ZD-24606: Snapfinance attribution | Read-only investigation recorded | [Canonical work item](../../../scorecard-workflows/work-items/ZD-24606.md) and [session](../../../scorecard-workflows/sessions/2026-10-26/codex-snapfinance-qm-task-attribution.md). No independent archive audit history available. |

## Findings retained for continuation

### Training Simulator CSV exports

The [local work item](../../../training-simulator/work-items/CONVI-7810.md) reflects an earlier implementation state. Current PR evidence supersedes that state for this summary. The design now extends the existing `ListTrainingSimulatorTaskRuns` API with creation-time filtering; Director paginates existing APIs and joins historical definitions to build CSV. The previous week's proposal for new export RPCs/backend generation is no longer the next action.

Proto #10413 merged October 27, backend #34647 October 28, and frontend #24328 and smoke coverage #24411 October 29. The frontend PR reports 53 focused app tests, app/testkit typechecks, and three export smoke tests passing locally and in CI with flag overrides. Its Walter Dev / voice-staging validation reports fresh exports of 65 attempts per export, including eight quizzes, and 23,930 comparisons against read-only app/auth DB data. Labels, revisions, results, scores, timestamps, memberships, and audiences matched; conversation start/end were checked against live API values. No missing or duplicate attempts were found within represented resource sets.

Exports preserve retakes and historical criteria/questions, distinguish missing definitions from N/A/pending, enforce stored session audiences for session exports, and retain matching attempts for lesson/module exports. They include formula protection, a UTF-8 BOM, and scores rounded to at most two decimals. The dedicated flag is default-off in the recorded implementation; flag-definition PR status and production targeting were not independently refreshed here.

Do not generalize the focused smoke result to all CI: #24411's current rollup retains QA metadata and E2E failures. Large-export performance and production rollout remain unverified.

### GCC adjusted appeals

[Recorded fix evidence](../../../analytics/sessions/2026-10-27/codex-convi-7472-fix.md) shows the chart used resolved rather than adjusted data. Full July live API results were 10 resolved / 3 adjusted; July 1–22 results were 7 / 1. The older 6 / 1 observation is not the current full-month expectation. Three regressions failed before the fix and passed afterward; the record includes 59 QM-report tests and typechecks. PR #24302 merged October 28 at 03:08 UTC, October 27 Toronto time. Merged and Done are verified; deployment and fixed-build customer validation are not.

### AdventHealth team labels

Backend direct-team IDs are sorted, and Director formerly displayed the first without role priority. The local fix chooses a direct Agent-role team with lowest-ID tie-breaking and retains the old fallback. It changes Agent-table and CSV labels only, using targeted paginated user enrichment for multi-team users. The implementation session records 11 regressions, 27 Leaderboard tests, and TypeScript/lint/Biome/knip checks passing; those checks were not rerun during this synthesis.

Read-only worktree inspection confirmed `/Users/xuanyu.wang/repos/director-convi-7852`, branch `xw/convi-7852-agent-team`, HEAD `59ec62ac94`, with the modified page and four untracked implementation/test files. No commit or PR exists for this local fix. The 33 single-Agent-team potential mismatches are candidates, not a measured count of affected visible rows; 25 multi-Agent cases are separate. Authenticated deployed AdventHealth validation is still pending.

### Oportun process-scorecard counts

July UTC PostgreSQL and ClickHouse contain the same 338 parent IDs. Twelve ClickHouse parent source timestamps are stale relative to criteria; the strict ID/version join yields 326 in both SQL and live daily Performance Insights. A weekly request spanning June 28–August 2 explains 335. Whole-card averaging (99.6107%) versus criterion weighting (99.7071%) explains the score difference. None of these reproduces or explains the original 275 / 63-card discrepancy.

The existing Director fix is present in the served release. No data mutation occurred. Continue by tracing stale writes/sync monitoring and preparing a targeted 12-card reindex, then checking parent/criterion version parity and count 338. Preserve the strict version join because it protects all-N/A overwrite semantics. Obtain the original request context separately.

### Dynamic-group review and assistance tracing

The CONVI-7874 review examined `db800e7d2f64` and reproduced inactive users reintroduced by a root dynamic merge and valid split-field dynamic groups discarded by early empty-Team checks. Four failing probes and three positive controls/shared tests supported the findings. Terry Wang's PR merged October 27 at final head `7651b74e26`; this synthesis did not recheck the findings at that head. Do not describe either finding as still present or resolved without that check. No external review comments or product changes were made by the recorded review.

CONVI-7879 backend #34417 merged October 26 at `72ced1c78b`. Drawer #24150 closed without merge October 28 at `031c1b4558`. The ticket moved to Done October 29 despite still describing both surfaces. Only backend merge is established.

### Snapfinance task attribution

The lookup found four matching Coll Loan Voice tasks: one active created July 7, 2025, and three archived created October 9, 2025, with status changes October 15, 2026. No matching October 2026 creation was found. Latest updater timestamps align with archive saves but are not independent audit evidence of the archive actor. If attribution work continues, obtain the specific alleged original/replacement task IDs.

## Access and changes

Used self-authenticating `gh` and the Linear connector for read-only status checks. No explicit AWS, Okta, Azure, or SSH credential/profile was selected or read in this wrap-up. Historical database checks above belong to the linked evidence, not new database access during synthesis.

Created only the weekly summary, this session note, and the [daily log](../../log/2026-11-01.md). No product code, external ticket, message, publication, commit, or push was made.

## Validation

Passed exact Progress / Problem / Plan section validation, resolution of all 15 local Markdown links, and whitespace checks on all three new files. Used both scoped `git diff --check` and `git diff --no-index --check /dev/null` so untracked files were covered.
