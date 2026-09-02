# CONVI-7533 deletion validation

## Context

- Ticket: [CONVI-7533](https://linear.app/cresta/issue/CONVI-7533/request-to-delete-outlier-scorecards-to-fix-monthly-view)
- Parent investigation: `CONVI-7254`
- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- Branch: `convi-7254-monthly-qa-score`
- ClickHouse: `home_care_delivered_us_east_1` on `us-east-1-prod` (`clickhouse-conversations.us-east-1-prod.internal.cresta.ai`)
- App DB: `hcd` / `director.scorecards`
- Auth DB: `home-care-delivered` / `auth.users`
- Template: `01995dc7-348c-7288-9d51-23bc68b45e86`
- Linear comment replaced: `a982024d-18bf-44d4-becc-9e7f629ebab6`

## Objective

Replace the existing Linear proposal with the concrete results of:

1. identifying the exact three May and one February weight-1 scorecards; and
2. running the production monthly aggregation formula with those IDs excluded.

## Credential Clearance

- ClickHouse: config `origin/master:configv3/prod/home-care-delivered/us-east-1/config.yaml`, cluster `us-east-1-prod`, host `clickhouse-conversations.us-east-1-prod.internal.cresta.ai`, database `home_care_delivered_us_east_1`. Used `us-east-1-prod_dev` via the ClickHouse skill.
- App DB: same config path, cluster `us-east-1-prod`, database `hcd` (cresta-cli rejects `home_care_delivered_us_east_1` as a PG name). Used `us-east-1-prod_k8s_ro` through `cresta-cli connstring --read-only`.
- Auth DB: `origin/master:configv3/prod/home-care-delivered/config.yaml`, cluster `auth-us-east-1-prod`, database `home-care-delivered`.
- No other credential is cleared or used. No production mutation.

## Query Shape

Reconstructed `RetrieveQAScoreStats` from `qaScoreStatsClickhouseQuery` / golden SQL:

- latest scorecard version CTE from `scorecard_d`;
- `score_d` distinct projection with `percentage_value >= 0` and `not_applicable <> true`;
- voicemail exclusion via `conversation_d.is_voice_mail <> 1`;
- template + criterion filters;
- current America/Toronto offset `toIntervalHour(-4)` (August DST), so May = `[2026-05-01 04:00:00, 2026-06-01 04:00:00)` and February = `[2026-02-01 04:00:00, 2026-03-01 04:00:00)`;
- formula `SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) / SUM(float_weight) FILTER (WHERE percentage_value >= 0)`.

Pass/fail for this aggregation is `percentage_value > 0` vs `= 0`. On May revision `2ab0f092`, passing rows have `percentage_value=1` and `numeric_value=0`.

## Identified Records

### May phone number (`019e7502-0e12-756b-a7ca-c81276df1781`)

| scorecard_id | conversation_id | revision | weight | percentage | agent | owner | scorecard_time UTC | created Toronto | status |
|---|---|---|---|---|---|---|---|---|---|
| `019e7502-3fd5-7528-94c6-e45c0b7e7448` | `019e7501-5e30-7412-9455-c60eac1223c1` | `33e46102` | 1 | 0 | `f45327f2d43224e3` | Melissa Crossin | 2026-05-29 18:31:24.477688 | 2026-05-29 14:32:22 | auto, unsubmitted |
| `019e7503-2411-7e04-883f-6bea8930d41c` | `019e7500-f082-7260-9964-4cb8066759f5` | `33e46102` | 1 | 0 | `141e633c084159dd` | Rabiah McCaskey | 2026-05-29 18:30:56.406262 | 2026-05-29 14:33:20 | auto, unsubmitted |
| `019e7504-dc03-78a7-b70b-d0e751b4e725` | `019e7503-b582-71d1-a1a1-01f141e601f3` | `33e46102` | 1 | 0 | `e25af18fccea7249` | Sheena Paris | 2026-05-29 18:33:57.905038 | 2026-05-29 14:35:13 | auto, unsubmitted |

Remaining May population: 1,303 passing + 3,444 failing `1e-13` rows on `2ab0f092`. `uniqExact(scorecard_id) = row_count` in every group. No other weight-1 rows.

### February how-did-you-hear (`0199c056-ce00-751a-ab4b-ecc11e4d7412`)

| scorecard_id | conversation_id | revision | weight | percentage | agent | owner | scorecard_time UTC | created Toronto | status |
|---|---|---|---|---|---|---|---|---|---|
| `019c2e30-ba72-7ed4-a202-b3ea651eaeec` | `019c2e2a-07e3-7b30-b434-93ebb6686910` | `8dd94019` | 1 | 1 | `cab06ed52e4e6304` | Latrese Proctor; created/updated by Kathy Barretta (`b3f02a4053b144c5`) | 2026-02-05 14:17:12.677092 | 2026-02-05 09:24:31, updated 11:57:09 | manually scored draft, unsubmitted |

Remaining February population: 93 passing `90dfe0ba` + 333 failing `90dfe0ba` + 1 failing `1ea5882b`, all `1e-13`. No duplicates. No other weight-1 rows.

## Aggregation Results

| scenario | criterion | weighted_percentage_sum | weight_sum | score | scorecards | passing | failing | distinct weights |
|---|---|---|---|---|---|---|---|---|
| baseline | May phone | `1.303e-10` | `3.000000000474696` | `4.343333332646079e-11` | 4750 | 1303 | 3447 | 2 |
| exclude_four | May phone | `1.303e-10` | `4.747e-10` | `0.274489151042764` | 4747 | 1303 | 3444 | 1 (`1e-13`) |
| baseline | Feb hear | `1.0000000000092997` | `1.0000000000427` | `0.9999999999665996` | 428 | 94 | 334 | 2 |
| exclude_four | Feb hear | `9.3e-12` | `4.27e-11` | `0.2177985948477751` | 427 | 93 | 334 | 1 (`1e-13`) |

Remaining `weight_sum > 0`; scores are defined. Identical remaining weights make the production formula equal `1303/4747` and `93/427`.

## Collateral Impact of Whole-Scorecard Deletion

February card also carries weight-1 rows for other criteria. Excluding it changes February monthly production scores:

| criterion | name | baseline score | exclusion score |
|---|---|---|---|
| `01995dd2-0da6-7647-8393-7d3031e9d152` | Asking for the address | 17/17 = 100% | 16/16 = 100% |
| `019a3572-4ce6-77a8-8596-047485ea134b` | Asking Incontinence type | 40.00% | 50.00% |
| `019a3572-fcd5-75cb-8972-3d38af2c0205` | Asking about mobility issues | 83.33% | 100% |
| `019a7957-728c-71de-ab0d-f649c251787e` | Ask about rash | 75.00% | 100% |
| `019a9226-17b0-75f9-95c4-0a79b0d9f1d6` | Asking about UTI or Fall | 75.00% | 100% |
| `019ae9fc-5a4a-77dc-b0f1-528b2db875be` | Avoid Reading the Address | 8/8 = 100% | 7/7 = 100% |

May collateral: Rabiah’s card also has `019dbb67-7060-726d-84a1-251226b41e1d` (“Asking for the physician”), all weight-1: `2661/2774 = 95.926%` → `2660/2773 = 95.925%`. The other two May cards only contain the phone-number criterion among applicable scores.

## Safety Conclusion

- Deletion of the four IDs fixes the two reported views with defined remaining aggregates.
- May deletion is safe for unrelated criteria.
- February deletion is not a no-op: it inflates four other mixed-weight February criteria toward 100%.
- Recommended default: delete the three May auto IDs; require explicit CS/customer acceptance before deleting the February draft.

## Linear Update

Replaced comment `a982024d` on CONVI-7533 with the completed results (not a proposal). No production delete was executed.
