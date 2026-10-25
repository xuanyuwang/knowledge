# Oportun Performance Insights follow-ups — 2026-10-06

**Primary domain:** analytics / performance-insights
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree context:** `main` in the primary checkout; read-only source context, no code edits

## Inputs

- The user's first two URLs point to the same [October 6 CONVI-7667 comment](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11#comment-76282b5a); the additional URL points to the [October 2 comment](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11#comment-21e6b1cd). Both report the FUP template blank in Performance Insights despite QM Report scorecards; October 6 adds conversation-linked examples.
- Linear CONVI-7667 is Done. Its October 2 comment says the original App Verification template looked fine then; the October 6 comment says the customer still sees it blank. That difference is a change in the reported state, not evidence of a distinct new issue.
- The FUP scorecard is conversation-linked according to Support's examples. Its reported template resource is `8821d006-5ce6-49b7-9a7f-060b119426d5@ce4baaa1`.
- Linear issue search found the FUP report in CONVI-7667 comments but no dedicated FUP Performance Insights ticket.

## Production Director control

Used the user's already open `https://oportun.cresta.com/director/insights/performance/conversations` Chrome tab. This is a single engineering session; customer visibility and hidden filter state were not captured.

| Use case | Template | Visible filters | Result |
| --- | --- | --- | --- |
| Backoffice Processes (voice) | QA-NOA-BO-OPS-ZEUS APP VERIFICATION V1.1 | July 1–31, 2025; Daily; no target audience | 99% Performance Score; 353 scorecards; populated chart/table |
| PAS - Follow Up (voice) | PAS SPL + HtD Scorecard FUP | June 1–September 30, 2026; Daily; All minutes; Exclude voicemail Yes; no target audience | 60% Performance Score; 2,684 scorecards in All criteria row; 2,767 eligible conversations; populated chart/table |

The FUP chart's visible x-axis data ended September 6, 2026 even though the selected range ended September 30. This could reflect no later matching rows or another time/filter issue; the control alone cannot decide.

## Ticket decision

- No second ticket for the original process template: CONVI-7667 already tracks it, and this session did not reproduce the renewed blank report. A failing-session request is needed if Support still sees it.
- Created [CONVI-7807](https://linear.app/cresta/issue/CONVI-7807) for the distinct FUP report in Triage, Oportun project, Convo Intelligence team, with Support and Customer Issue labels. Added links to both dated Support comments. The ticket states the customer report and the populated control separately and leaves root cause open; the two comments do not warrant separate FUP tickets.
- Posted [a reply to the October 2 comment](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11#comment-ffea2455) and [a reply to the October 6 comment](https://linear.app/cresta/issue/CONVI-7667/oportun-process-scorecard-qa-noa-bo-ops-zeus-app-verification-v11#comment-86db6328). Both say the blank result could not be reproduced in the engineering session and request `qaScoreStats:retrieve` data from the affected customer session; the later reply covers both templates and their separate tickets.
- No credential files or restricted credentials were read or used; authentication was through the user's already open browser session.
