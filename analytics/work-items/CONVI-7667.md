# CONVI-7667: Oportun process scorecards missing from Performance Insights

**Status:** in review
**Primary domain:** `analytics`
**Primary subdomain:** `performance-insights`
**Official ticket:** [CONVI-7667](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11)
**Last updated:** 2026-09-13

## Objective and Impact

- **Objective:** Explain and fix why Oportun process template `QA-NOA-BO-OPS-ZEUS APP VERIFICATION V1.1` has data in QM Report but renders zero volume and no data in Performance Insights.
- **Customer/system impact:** Oportun Backoffice Processes; one reported template, with likely blast radius to process templates whose PI filter state carries the new default conversation-close date target.
- **Role:** diagnosed and implemented

## Scope

**In scope**

- Screenshot/date interpretation, PI request construction, PostgreSQL source-of-truth counts, ClickHouse projection, exact zero-result reproduction, and a narrow Director fix with regression coverage.

**Non-goals**

- Backfills, customer-data mutation, or backend changes.

## Source Context

- **Repos:** `director`, `go-servers`, `config`, `knowledge`
- **Implementation worktree:** `/Users/xuanyu.wang/repos/director-convi-7667`
- **Implementation branch:** `convi-7667-process-scorecard-date-target`
- **Relevant commit:** Director `bd4074c5f4` / PR #22057 (`CONVI-7586`)
- **Fix commit:** Director `961a44a922`
- **Draft PR:** [cresta/director#22772](https://github.com/cresta/director/pull/22772)

## Current Understanding

This is a **presentation/request-construction issue**, not a PostgreSQL-to-ClickHouse sync gap. Director commit `bd4074c5f4` changed the Performance Insights default `dateRangeTarget` from unspecified to `CONVERSATION_ENDED_AT`. The process-template normalization clears conversation-only filters but does not clear `dateRangeTarget`. `useQAScoreStatsRequestParams` therefore sends the close-time target for process scorecards. The analytics backend implements that target by inner-joining scorecard rows to `conversation_d`; process scorecards have empty conversation IDs, so all are removed.

For the exact template/revision (`94e6b75d-b1eb-4e41-a477-1d070d96fa9c` / `c0139eb5`), July 2026 has 338 submitted/scored rows in PostgreSQL and 338 distinct ClickHouse scorecards. Valid ClickHouse criterion rows reproduce an aggregate score of 99.7207%. Reproducing the conversation-ended-at join returns exactly 0 scorecards and 0 score rows, matching the UI.

## Findings and Decisions

- Screenshot dates are not the same year: the populated screenshot is July 2025 (`Tue 07/01`), while the zero-data screenshots are July 2026 (`Wed 07/01`).
- PostgreSQL counts for the exact template revision: July 2025 process interaction = 353; July 2026 = 338. Both windows have the same submission counts, and all July 2026 rows are submitted and scored.
- ClickHouse `scorecard_d FINAL`: 353 distinct scorecards for July 2025 and 338 for July 2026.
- ClickHouse `score_d FINAL`: all 353/338 scorecards have valid scorable criterion rows; weighted scores are 99.3404% and 99.7207%, respectively.
- Rows are in use case `backoffice-processes` and are not dev-user rows.
- The old screenshot's 352 is one below today's 353-row source-of-truth snapshot; the latest July-2025 row was created/submitted on July 31, so the historical screenshot timing can explain the one-row difference. It does not affect the regression diagnosis.
- The gRPC black-box helper was not used for `RetrieveQAScoreStats` because that method is absent from its reviewed read-only allowlist.
- The working HAR identifies production Director build `58a0ca6dcb` (deployed 2026-09-10). That build still defaults `dateRangeTarget` to `CONVERSATION_ENDED_AT` and does not clear it for process templates; current `origin/main` at `4073474177` also contains no relevant change.
- A parallel unmerged branch, `origin/sdey/insi-4748-date-target-default` at `8c12fa4296` (2026-09-11), addresses the broader non-email default/cached-state problem and threads the selected target through `ConversationCountChart`. It is not contained in the production build or current main and has no PR found by GitHub search.

## Blockers and Dependencies

- Add before/after preview videos for the declared `smoke-functional` QA request, then complete CI/review, merge, and deployment of draft Director PR #22772.

## Validation and Rollout

- Implemented behavior: process-template filter normalization clears `dateRangeTarget`, so generated QA requests omit the conversation-only time target and retain `scorecard_time = process_interaction_at` semantics.
- Regression coverage asserts that process scorecards clear `CONVERSATION_ENDED_AT` while conversation scorecards preserve it.
- Local validation passed: focused Vitest suite (4 tests), Director-app TypeScript build, and commit-hook lint/i18next/format checks.
- A live Oportun UI check on 2026-09-13 did not reproduce the zero-data state with the exact template: July 2025 showed 99% / 353 scorecards and July 2026 showed 100% / 335 scorecards. The working-session HAR contains 12 populated QA-score requests omitting `conversationTimeRangeField` and two empty auxiliary count-chart requests carrying `CONVERSATION_ENDED_AT`. This proves the field is incompatible with process rows but does not prove the originally failing UI used it for its visible widgets; a failing-session HAR is still required for end-to-end attribution.
- Revalidate July 2026 after a fix: volume 338 and aggregate approximately 99.72%, subject to live-data changes.

## Next Actions

1. Complete CI and review for draft Director PR #22772.
2. Revalidate the deployed PI request/response for the reported template: volume 338 and aggregate approximately 99.72%, subject to live-data changes.

## Timeline

- 2026-09-10 — Root cause confirmed through Linear screenshot arithmetic, source history, indexed PostgreSQL counts, ClickHouse aggregation, and exact zero-result reproduction. Evidence: `sessions/2026-09-10/codex-convi-7667-oportun-process-pi.md`, `log/2026-09-10.md`.
- 2026-09-13 — Posted the investigation report to Linear with the root cause, PG/CH evidence, explicit no-backfill conclusion, and proposed Director fix/test plan. Linear comment ID: `df73e6fb-740a-4a29-a46c-308bf595b265`. Evidence: `log/2026-09-13.md`.
- 2026-09-13 — Implemented and validated the process-template normalization fix in commit `961a44a922`; opened draft Director PR [#22772](https://github.com/cresta/director/pull/22772). Evidence: `sessions/2026-09-13/codex-convi-7667-fix.md`, `log/2026-09-13.md`.
- 2026-09-13 — Checked the exact template in the live Oportun Performance Insights UI and analyzed the working-session HAR after Support's added scope comment. Both years were populated. The HAR showed a mixed request set: visible populated requests omitted the conversation target, while two auxiliary requests included it and returned zero. Updated draft PR #22772 to normalize those reconstructed count requests, but retained the need for a failing-session HAR. Evidence: `sessions/2026-09-13/codex-convi-7667-live-reproduction.md`, `log/2026-09-13.md`.
