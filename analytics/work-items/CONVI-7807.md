# CONVI-7807: Oportun FUP scorecard absent for affected Performance Insights users

**Status:** Triage in Linear
**Primary domain:** `analytics`
**Primary subdomain:** `performance-insights`
**Official ticket:** [CONVI-7807](https://linear.app/cresta/issue/CONVI-7807)
**Last updated:** 2026-10-06

## Objective and Impact

Investigate why Oportun Support reports zero scorecards for the conversation-linked `PAS SPL + HtD Scorecard FUP` template in Performance Insights, while QM Report and Closed Conversations contain examples. A production engineering session with the same visible filters is populated, so the affected-session cause is unresolved.

## Source Context

- **Source repo:** `/Users/xuanyu.wang/repos/director` (`main` checkout; no code changes in this investigation).
- **Related case:** [CONVI-7667](https://linear.app/cresta/issue/CONVI-7667), the original standalone/process template.
- **Reported template:** `8821d006-5ce6-49b7-9a7f-060b119426d5@ce4baaa1`. The production UI control selected by name did not expose its resource ID.

## Current Understanding

Support's [October 2 comment](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11#comment-21e6b1cd) first reported that the FUP template was blank for June 1–September 30, 2026 while QM Report contained scorecards. The [October 6 comment](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11#comment-76282b5a) repeated that symptom and added conversation-linked examples. In production Director on October 6, an engineering session selected PAS - Follow Up (voice), that template, the same date range, Daily grouping, All minutes, and Exclude voicemail Yes. It rendered a 60% Performance Score, 2,684 scorecards in the All criteria row, and 2,767 eligible conversations. The plotted data ends on September 6 despite the selected end date of September 30; whether later scored FUP rows are expected under these filters needs checking.

The process-template empty-conversation-ID mechanism from CONVI-7667 does not directly explain this conversation-linked template. No failing customer request/response has been captured, and the root cause remains unconfirmed.

## Next Actions

1. Obtain the affected user's exact `qaScoreStats:retrieve` request and response, including template resource ID, use case, date target, filters, and user scope; compare with a populated control. Requested this in replies to both Support follow-ups on October 6.
2. Verify the supplied conversation-linked examples against `scorecard_d FINAL`, `score_d FINAL`, and their joined `conversation_d` rows and time fields.
3. Determine whether September 7–30 should contain scored FUP results under the selected filters.
4. Fix only the confirmed cause, then validate in the affected session.

## Timeline

- 2026-10-06 — Created the distinct FUP investigation ticket after reproducing a populated production control and checking for a dedicated existing issue; linked both October 2 and October 6 Support comments. Evidence: `sessions/2026-10-06/codex-oportun-pi-followups.md`.
- 2026-10-06 — Replied to both Support comments: stated that the blank views did not reproduce in the engineering session, identified the separate ticket, and requested affected-session request/response data.
