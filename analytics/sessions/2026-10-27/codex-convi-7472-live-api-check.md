# CONVI-7472 live browser/API check — October 27, 2026

Source repo context: `/Users/xuanyu.wang/repos/director`, main checkout, branch main (no source changes or source re-verification). Browser: user's existing Chrome tab at https://golden-cc.cresta.com/director/qm/report/appeals . Read-only production reporting; normal browser authentication used, no credentials extracted or saved.

## Full July: confirmed

Observed production Network response for `https://api.streaming.us-west-2-prod.cresta.com/v1/customers/golden-cc/profiles/us-west-2/appealStats:retrieve`, HTTP 200.

Request: start `2026-07-01T04:00:00.000Z`, end `2026-08-01T03:59:59.999Z`; MONTHLY; groupByAttributes `[QA_ATTRIBUTE_TYPE_TIME_RANGE]`; timezone America/Toronto; use case voice-care; Pet Scorecard - Voice template `019dcf98-a7b6-746f-807d-869fd8be7e22@74071806`; empty users/teams/groups and reviewer users/groups.

The July `appealStats` bucket starts `2026-07-01T04:00:00Z` and returns:

| Field | Value |
| --- | --- |
| appealedConversationCount | 10 |
| resolvedConversationCount | 10 |
| adjustedConversationCount | 3 |
| appealedCriteriaCount | 12 |
| resolvedCriteriaCount | 12 |
| adjustedCriteriaCount | 3 |

UI chart displays 10/10/10 (Appealed/Resolved/Adjusted). Table contains 10 rows, three with 1/1 adjusted and seven with 0 adjusted. Criteria overview is 25% adjusted / 12 resolved, consistent with 3/12. This proves the observed chart inflation occurs after a correct distinct adjusted count reaches the browser. Exact source mapping remains the ticket's reported diagnosis, not independently rechecked this session.

## Partial July: supporting check

Changed only date/frequency to July 1–22 Daily through visible report controls. Reloaded with DevTools Network open and read the time-series response. `totalAppealStats` returns 7 appealed, 7 resolved, 1 adjusted conversation; 9 appealed/resolved criteria and 1 adjusted criterion. UI overview/table agree (11% = 1/9). Observed individual July 18/21/22 daily buckets with resolved=1, adjusted=0 despite chart bars both being 1.

The 6-resolved/1-adjusted ticket screenshot is a historical snapshot, not an expected invariant for all of July or today's reconstructed range. Today's full-month count is 10/3; July 1–22 is 7/1. Why historical resolved count was six was not investigated. No need to resolve that history to demonstrate the chart mapping defect.

## Outcome and follow-up

First follow-up completed: live time-series API does not inflate adjusted to resolved in the reproduced full-July case. Fix/test frontend mapping next; revalidate full July as 10 resolved / 3 adjusted, with current data, after shipping. No evidence here supports a data backfill for this chart defect. No code, ticket, Zendesk, permission, or customer record modifications made. Restored original July 1–31 Monthly view.

Evidence: [partial-period API screenshot](convi-7472-july22-api.png) and [restored full-July report](convi-7472-july-full-report.png). Screenshots contain report data only, not request headers or authentication material.
