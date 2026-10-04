# RCG CEL Retreat score mismatch investigation

- Date: 2026-09-28
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source branch/worktree context: main checkout, read-only; no code edits
- Customer/profile: RCG / `us-east-1`, production
- Report: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1790617067502889

Used the customer app DB and ClickHouse connection skills with the selected production config `origin/master:configv3/prod/rcg/us-east-1/config.yaml`. Queried `director.scorecards` by conversation/template, then `director.scores` by customer/profile/scorecard ID to use the available composite index. Queried `scorecard_d` and `score_d` by scorecard ID; grouped score versions with `argMax(..., update_time)`.

PG scorecard `01a091f4-8df3-72f7-b976-d5b75a99abad` has 50%, while chapter score rows are 100/0/100/100. CH `scorecard_d` has 50% and CH `score_d` latest weighted percentages are 1/0/1/1 at weight 25 each. Thus CH agrees with the inconsistent PG source, and 75% is the calculated total. PG `submitted_at` precedes `updated_at` by 752 microseconds. The September 11 mutation preceded PR #32447's September 24 merge. This supports, but does not prove, the submit/autosave race; a stale/partial update remains possible. No write or repair was performed.

After the user changed a criterion to N/A and back to No, PG parent became 75% at 2026-09-28 18:36:10.697674 UTC; the four chapter totals remained 100/0/100/100. CH parent has a newer 75 version projected at 18:36:13.059760 UTC, and all latest weighted score rows carry `scorecard_score=75`. Both `scorecard_d FINAL` and `scorecard_d_read FINAL` return 75; `score_d_read FINAL` returns four rows with parent score 75. The user reports the scorecard view shows 75 but a report/chart still shows 50. Exact report URL and filters are pending, so its serving path remains unverified.

Credential handling: manually inspected the selected AWS `us-east-1-prod_dev` and `us-east-1-prod_ro` profile entries plus their `cresta` SSO session entry before use. The DB connection helper used its read-only connection path; the ClickHouse helper used `us-east-1-prod_dev`. No credentials were printed or stored.

See `../../work-items/rcg-cel-retreat-score-mismatch.md` for findings and next actions.
