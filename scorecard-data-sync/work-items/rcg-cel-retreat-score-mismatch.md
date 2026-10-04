# RCG CEL Retreat score mismatch

## Objective and status

Investigate why RCG conversation `01a076df-fc80-7271-a256-8ab71f874a4c`, template `019d4e5d-b0a7-74ca-add3-3c051fba74bc@d8ccf52a`, displays 50% although four equally weighted criteria imply 75%.

Initial read-only production investigation on 2026-09-28 found an inconsistent PostgreSQL parent score that ClickHouse mirrored. Later the same day, a user changed one criterion to N/A and back to No. The source scorecard and latest ClickHouse versions became 75%, but the user reported a report/chart still showing 50%. The original incident is consistent with the earlier submit/autosave race, but the exact request interleaving is not established from these rows. The remaining report discrepancy needs the exact report URL and query path. No agent-initiated repair or backfill was run.

## Evidence

- [Slack report](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1790617067502889): Customer Xperience Center DTQ, 75% expected, 50% displayed. Reply confirms all four criteria have weight 25.
- Production RCG app DB, `director.scorecards`: scorecard `01a091f4-8df3-72f7-b976-d5b75a99abad`, use case `cel-retreat`, `score=50`, `manually_scored=true`, `created_at=2026-09-11 19:31:59.025002+00`, `submitted_at=19:32:19.673849+00`, `updated_at=19:32:19.674601+00`. Submit/update gap: 752 microseconds.
- `director.scores`: four chapter rows have `numeric_value` 100, 0, 100, 100. Nine leaf score rows were also present. Thus the weighted chapter total is `(100+0+100+100)/4 = 75`.
- Production RCG ClickHouse `scorecard_d`: same scorecard/template revision and `score=50`, with the same source update and submit times; projection `update_time=2026-09-11 19:32:22.063544`.
- ClickHouse `score_d`: the four weight-25 criteria have latest `percentage_value` 1, 0, 1, 1. Each has two projection versions with identical values, the latest at `2026-09-14 05:23:51.440276`. Other score rows have zero effective weight. The four current weighted rows calculate to 75%.
- [go-servers PR #32447](https://github.com/cresta/go-servers/pull/32447), which preserves the stored score during SubmitScorecard, merged on 2026-09-24 at 18:04:23 UTC, after this scorecard's 2026-09-11 mutation. The previous RCG case documents the mechanism and its limits in [rcg-casino-dtq-score-mismatch.md](rcg-casino-dtq-score-mismatch.md).
- After the user's N/A → No edit, `director.scorecards.score=75` and `updated_at=2026-09-28 18:36:10.697674+00`; the four PG chapter totals remain 100/0/100/100.
- ClickHouse `scorecard_d` has a new 75 row with source update time `2026-09-28 18:36:10.697674` and projection time `18:36:13.059760`; the older 50 row remains in raw history. `scorecard_d FINAL` and `scorecard_d_read FINAL` each return 75. The four latest weight-25 `score_d` rows and `score_d_read FINAL` rows carry `scorecard_score=75`.

## Assessment

The parent/child mismatch originates in PostgreSQL and is copied to ClickHouse. The near-coincident submit/update timestamps and pre-fix date make the prior submit/autosave overwrite a strong candidate. A partial or stale `UpdateScorecard` can also compute the parent from an incoming subset while retained child rows represent a fuller state. Neither table records request order or score-row update time, so the specific writer race is unproven.

## Next actions

1. Obtain the exact report/chart URL and filters, then trace the serving API and query/cache path for its reported 50%. Compare its refresh time and result against the latest `scorecard_d_read FINAL`/`score_d_read FINAL` values. Avoid assuming raw old CH rows are the report's cause without its query.
2. If incident-level attribution is needed, inspect retained September 11 request traces/audit data for the scorecard. Do not infer exact request order from timestamps alone.
