# CONVI-7674: RCG process scorecards show no data in Performance Insights

**Status:** In Progress  
**Priority:** High  
**Primary domain:** `analytics`  
**Primary subdomain:** `performance-insights`  
**Official ticket:** [CONVI-7674](https://linear.app/cresta/issue/CONVI-7674/rcg-process-scorecards-show-no-data-in-performance-insights-after)  
**Last updated:** 2026-09-17

## Objective and Impact

- Restore process-scorecard data in Performance Insights for RCG and other customers using the default close-date filter state.
- RCG users repeatedly receive zero/no-data for `Customer Xperience Center - Social Care DTQ - V1` while QM Report contains scores; the customer escalated again on 2026-09-14.

## Current Understanding

The attached HAR contains 20 successful `RetrieveQAScoreStats` responses with `totalScorecardCount: 0`. Every request uses the correct data-bearing template and use case, has no user/group restriction, and sends `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT`.

Director PR #22057 / CONVI-7586 made conversation-ended time the default. Process-template filter normalization does not clear this target, so the backend adds an inner conversation join. Process scorecards have empty conversation IDs, and the join removes every row. RCG ClickHouse reproduction reduces 51 valid August scorecards to zero.

The same root cause is fixed by Director PR #22772. On 2026-09-15, the PR was retargeted from CONVI-7667 to this canonical RCG issue, renamed `[CONVI-7674] Fix process scorecard no-data in Performance Insights`, and merged as `f740a0fa253a6c304516502ac933387f09a1eb2c`; CONVI-7667 remains related investigation context.

The 2026-09-14 review confirmed that PR #22772 is the correct minimal frontend fix: it clears the incompatible conversation-ended target only for process templates and covers the count chart's internally reconstructed primary, unfiltered, and delta requests. It preserves conversation-template behavior. A backend guard would be useful defense in depth against other callers sending the same invalid combination.

The release `release_director_2026-09-17-9c7412f` does not contain the fix. Its branch was cut from `main` at the Monday 2026-09-14 23:20 UTC cutoff; PR #22772 merged Tuesday at 18:59 UTC, about 19 hours 39 minutes too late for that train. The release commit is not descended from the PR merge commit, and direct inspection of the release tree shows both pre-fix code paths. The branch's two later commits are explicit hotfixes, not a resync from `main`. RCG needs a later release containing PR #22772 before deployed validation.

The cross-user behavior is now explained and preview-validated. Andra's failing HAR includes `CONVERSATION_ENDED_AT`; Xuanyu's current RCG production cache for the same data-bearing template omits `dateRangeTarget`. Cached state replaces rather than merges the newer default, and the non-email RCG page does not expose the hidden date-target control. Andra confirmed that PR override `22772_merge-e392f52` loads the data. The issue is deterministic per browser-local persisted state, not random intermittency.

The paired HAR comparison now confirms this at the network boundary. Andra sends the ended-at field in 20/20 calls and receives zero in all 20. Xuanyu omits it in 19/21 calls; all 18 successful omitted-field calls are populated, while Xuanyu's only two ended-at calls return zero. Both captures use the same template, use case, windows, and empty audience/status filters. This rules out the Cresta-account identity and visible filters as the differentiator in the captured requests.

## Validation Needed

1. Deploy a Director release containing merged PR #22772; the 2026-09-17 `9c7412f` release does not contain it.
2. Preview validation complete: Andra re-ran her affected RCG state with override `22772_merge-e392f52` and confirmed that data loads.
3. After deployment, explicitly validate both fresh/incognito and persisted filter states.
4. Confirm the fix across representative process templates and customers.
5. Compare the exact template resource ID rather than the display label: the first live production/local comparison used different templates (`Customer Xperience Center...` versus `QM Coaching Customer Xperience Center...`) and therefore did not test the PR behavior.

## Timeline

- 2026-09-10 — Ticket created after HAR confirmed the close-date request path and zero-result join behavior.
- 2026-09-14 — Reused the existing exact ticket rather than creating a duplicate; added renewed escalation, ownership, customer/on-call labels, CONVI-7667/PR #22772 relationship, and moved it to In Progress.
- 2026-09-14 — Independently rechecked all 20 HAR request/response pairs and PR #22772's current diff; confirmed it is the correct minimal fix for RCG, with deployed replay still required.
- 2026-09-14 — Live Chrome comparison found production data (46 scorecards) and local zero, but the tabs selected different similarly named templates; an exact resource-ID comparison is required before drawing a regression conclusion.
- 2026-09-14 — Confirmed the cross-user split: Xuanyu's production cache omits `dateRangeTarget`, while Andra's failing HAR sends conversation-ended time. Andra validated the PR override and reported that data loads.
- 2026-09-15 — Paired HAR comparison confirmed the hidden-field split: 20/20 Andra requests send ended-at and return zero; 18/18 successful Xuanyu requests that omit it return data, and Xuanyu's two ended-at requests return zero.
- 2026-09-15 — Retargeted Director PR #22772's title, Linear link, failure evidence, environment, and QA steps from CONVI-7667/Oportun to CONVI-7674/RCG while preserving the latest videos, validation, QA selection, and review controls.
- 2026-09-17 — Confirmed `release_director_2026-09-17-9c7412f` excludes PR #22772 by both commit ancestry and direct release-tree inspection. The weekly branch was cut Monday at 23:20 UTC; the PR merged Tuesday at 18:59 UTC, while only two explicit Wednesday hotfixes entered the already-cut train.
