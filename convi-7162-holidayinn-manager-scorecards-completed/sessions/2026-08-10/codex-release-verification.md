# CONVI-7162 Holiday Inn production release verification

**Date:** 2026-08-10 (America/Toronto)
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** post-merge production verification; no source changes
**Ticket:** [CONVI-7162](https://linear.app/cresta/issue/CONVI-7162/holiday-inn-club-vacations-manager-leaderboard-scorecards-completed)

## Objective

Verify that the merged Manager Leaderboard submit-time fix is present in `holidayinn-transfers-voice` behind `filterByScorecardSubmitTime` and that the flag-enabled UI matches ClickHouse.

## Ticket expectation

For Cliff Hawker and Ride Along Template during 2026-06-15 through 2026-06-21, Manager `Scorecards completed` must be attributed by `scorecard_submit_time`. The expected weekday distribution is 2, 2, 2, 2, 2 (10 total).

## Initial flag-off UI verification

Surface: `https://holidayinn-transfers-voice.cresta.com/director-beta/insights/leaderboard/managers`

Filters/state observed:

- use case: `transfers-voice`
- view: Manager
- date range: Jun 15–Jun 21, 2026
- daily metric: Scorecards completed

Cliff Hawker row:

| Day ET | UI |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 3 |
| 2026-06-17 | 1 |
| 2026-06-18 | 2 |
| 2026-06-19 | N/A (0) |
| 2026-06-20 | 2 |
| 2026-06-21 | N/A (0) |
| Total | 10 |

This is the expected flag-off, pre-fix conversation-time redistribution. The initial conclusion omitted the guarded URL override.

## Flag-enabled production verification

Surface:

`https://holidayinn-transfers-voice.cresta.com/director-beta/insights/leaderboard/managers?filterByScorecardSubmitTime=true`

The flag-enabled UI was checked for Last 7 days (2026-08-04 through 2026-08-10 ET), Manager view, Scorecards completed. This current window cleanly distinguishes submit time from conversation time.

| Day ET | Flag-enabled UI |
|---|---:|
| 2026-08-04 | 2 |
| 2026-08-05 | 2 |
| 2026-08-06 | 2 |
| 2026-08-07 | N/A (0) |
| 2026-08-08 | N/A (0) |
| 2026-08-09 | N/A (0) |
| 2026-08-10 | 2 |
| Total | 8 |

Fresh ClickHouse results for the same submitter and window, with all templates:

| Basis | Daily counts | Total |
|---|---|---:|
| `scorecard_submit_time` | Aug 4: 2; Aug 5: 2; Aug 6: 2; Aug 10: 2 | 8 |
| `scorecard_time` | Aug 5: 2; Aug 6: 2; Aug 10: 2 | 6 |

The flag-enabled UI matches `scorecard_submit_time` exactly and differs from `scorecard_time`.

## Official Director release verification

Surface:

`https://holidayinn-transfers-voice.cresta.com/director/insights/leaderboard/managers?filterByScorecardSubmitTime=true`

The same Last 7 days window and Manager `Scorecards completed` metric were checked on the official non-beta route.

| Day ET | Official flag-query UI |
|---|---:|
| 2026-08-04 | N/A (0) |
| 2026-08-05 | 2 |
| 2026-08-06 | 2 |
| 2026-08-07 | N/A (0) |
| 2026-08-08 | N/A (0) |
| 2026-08-09 | N/A (0) |
| 2026-08-10 | 2 |
| Total | 6 |

The query parameter remained present in the URL, but the official route matched ClickHouse `scorecard_time` (6) rather than `scorecard_submit_time` (8). Therefore the official Director release does not yet honor the guarded fix, while `director-beta` does.

## ClickHouse verification

Production database: `holidayinn_transfers_voice.scorecard_d FINAL` on `voice-prod`.

Filters:

- `submitter_user_id = '9d654376ad4f1cdc'` (Cliff Hawker)
- `scorecard_template_id = 'f00391f9-c9f8-4bd4-885a-3bcad260817c'` (Ride Along Template)
- America/New_York window 2026-06-15 00:00 through 2026-06-22 00:00

Results:

| Basis | Daily counts |
|---|---|
| `scorecard_submit_time` | Jun 15–19: 2, 2, 2, 2, 2 |
| `scorecard_time` | Jun 15: 2; Jun 16: 3; Jun 17: 1; Jun 18: 2; Jun 20: 2 |

The initial flag-off UI matches `scorecard_time` exactly and does not match `scorecard_submit_time`.

## Conclusion

The fix is working on `director-beta` behind `filterByScorecardSubmitTime`, but it is **not yet working on the official `/director/` release**. On identical Last 7 days data, beta flag-on matched submit time (8) while official with the same query flag matched conversation time (6). The official Director release must be updated and retested before CONVI-7162 can be considered released to the customer-facing channel.

## Credential hygiene

- Used AWS SSO profile `voice-prod_dev` after reading its own config entry and confirming no restricted marking.
- Used secret `clickhouse/voice-prod/users/admin` only after checking its metadata/tags and finding no restricted marking.
- The secret value was passed directly to the ClickHouse client and was not printed or stored.
- `voice-prod_ro` was tried first, but it lacked `secretsmanager:GetSecretValue`; no ClickHouse query ran under that profile.
