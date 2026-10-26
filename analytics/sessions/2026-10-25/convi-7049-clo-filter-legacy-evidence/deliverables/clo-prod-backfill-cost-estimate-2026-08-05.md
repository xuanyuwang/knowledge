# Prod CLO Phase B backfill cost estimate (2026-08-05)
**Baseline throughput:** voice-staging `cresta_walter_dev` Phase B apply [31023390119](https://github.com/cresta/clickhouse-schema/actions/runs/31023390119) — ~482k rows / ~17.3s ≈ **28k rows/sec** (clo write). Alt scan rate ~54k raw/sec.
**Inventory:** `count()` of `moment_type=14` on `moment_annotation_d` via Secrets Manager admin (not FINAL). Linear: CONVI-7423.

## 2026-08-07 update — us-west-2 and us-east-1

Live refresh after completing eu-west-2, ap-southeast-2, chat, and voice:

- Actual outcome: west apply completed in ~18 minutes and east in ~12 minutes, both without timeout. Full post-validation completed with no database behind.
- Completed prod applies processed 115.2M raw rows in 632.9 seconds of config-runner time, an effective weighted rate of about **182k raw rows/sec**.
- `us-west-2-prod`: **244,348,864** raw rows, 87/173 physical DBs non-empty, zero non-Finished DDL. Expected apply-runner time is **~22 minutes** at 182k/s or **~35 minutes** at the slower chat-prod rate (~115k/s). Budget **30–45 minutes** for the apply workflow.
- `us-east-1-prod`: **435,480,504** raw rows, 68/96 physical DBs non-empty, zero non-Finished DDL. Expected apply-runner time is **~40 minutes** at 182k/s or **~63 minutes** at ~115k/s. Budget **45–75 minutes** for the apply workflow.
- If run sequentially, budget **~75–120 minutes** for both apply workflows, excluding dry runs and post-validation. Separate cluster runs are preferred.
- The old 28k/s staging baseline implies ~2.42h west and ~4.32h east, but completed production runs were about 4–8x faster; retain this only as a severe-slowdown contingency.
- Largest DBs: `united_east_us_east_1` 199.0M (~18m at 182k/s, ~29m at 115k/s); `cvs_us_west_2` 140.8M (~13m / ~20m). Both fit under the runner's 3,600s query limit at observed production rates, but monthly chunks remain safer for retry granularity and unexpected load.
- Phase A drift does not affect row-time totals because all gaps are zero-row. Five normal-schema DBs need Phase A: west `enova_us_west_2`, `goodtime_sbx_us_west_2`, `goodtime_us_west_2`; east `bill_us_east_1`, `comcast_us_east_1`.
- Seven west DBs lack `moment_annotation_payload` and are approved skips: `angi_us_west_2`, `hsbc_us_west_2`, `indeed_mg_sbx_us_west_2`, `mpa_sbx_us_west_2`, `mpa_us_west_2`, `nyman_turkish_us_west_2`, `rcg_us_west_2`. All have zero m14 rows; indeed has partial storage only.

### Timeout gates

- Reusable GHA operation job: `timeout-minutes: 90`, applied per cluster matrix job.
- ClickHouse client setting: `max_execution_time=3600` seconds per database query.
- Driver `send_receive_timeout=3900` seconds.
- Distributed DDL wait: `distributed_ddl_task_timeout=900` seconds.
- West needs roughly 47k raw rows/s to finish within an 86-minute runner budget after setup; east needs roughly 84k/s. The 115k/s conservative production calibration fits both, with less margin on east.
- CVS needs ~39k/s and United East ~55k/s to fit each 60-minute query limit.
- `main.py` catches per-DB exceptions, logs `Failed DBs`, continues, and does not exit non-zero. A workflow may therefore be green despite a timed-out database. Always inspect final logs for `Failed DBs` and `Failed to run queries`.
- `max-parallel: 1` serializes matrix clusters in the current reusable workflow. Run west and east as separate targeted workflows; do not use `all-prod`, which also includes completed and intentionally skipped clusters.

## Fleet

| Metric | Value |
|---|---|
| DBs | 355 (192 with m14>0) |
| Total m14 raw | 793,341,281 |
| Est. @ 28k/s | ~7.87 h serial-equivalent |
| Est. @ ~54k raw/s | ~4.1 h |
| Fleet wall (old assumption) | Parallel estimate is obsolete; current workflow uses `max-parallel: 1` |

## By cluster

| Cluster | m14 raw | DBs w/ data | Est. @ 28k/s |
|---|---:|---:|---:|
| us-east-1-prod | 436,893,406 | 68/96 | ~4.33 h |
| us-west-2-prod | 241,753,061 | 82/159 | ~2.4 h |
| voice-prod | 91,232,901 | 24/54 | ~0.91 h |
| chat-prod | 18,322,080 | 11/31 | ~0.18 h |
| ap-southeast-2-prod | 4,040,505 | 2/6 | ~0.04 h |
| eu-west-2-prod | 1,099,328 | 5/9 | ~0.01 h |

## Top 15 DBs

| Cluster | Database | m14 raw | Est. @ 28k/s |
|---|---|---:|---:|
| us-east-1-prod | `united_east_us_east_1` | 198,392,400 | ~118.1 min |
| us-west-2-prod | `cvs_us_west_2` | 140,221,452 | ~83.5 min |
| us-east-1-prod | `marriott_us_east_1` | 47,681,432 | ~28.4 min |
| us-east-1-prod | `rcg_us_east_1` | 34,965,207 | ~20.8 min |
| voice-prod | `cresta_sandbox_2_voice_sandbox_2` | 31,066,973 | ~18.5 min |
| us-east-1-prod | `alaska_air_us_east_1` | 29,306,612 | ~17.4 min |
| voice-prod | `hilton_voice` | 22,980,327 | ~13.7 min |
| us-west-2-prod | `airbnb_us_west_2` | 18,555,379 | ~11.0 min |
| us-east-1-prod | `nrg_us_east_1` | 16,008,224 | ~9.5 min |
| us-east-1-prod | `rentokil_us_east_1` | 14,628,123 | ~8.7 min |
| us-west-2-prod | `viking_us_west_2` | 12,623,913 | ~7.5 min |
| us-east-1-prod | `spirit_us_east_1` | 9,482,445 | ~5.6 min |
| voice-prod | `brinks_care_voice` | 8,102,062 | ~4.8 min |
| us-east-1-prod | `wyndham_us_east_1` | 7,762,748 | ~4.6 min |
| chat-prod | `united_airlines_united_airlines_care_chat` | 7,583,725 | ~4.5 min |

## Gaps / caveats

- ca-central-1-prod unreachable (209); schwab secret denied; comcast no local AWS profile; some us-west-2 counts failed (max queries).
- Raw counts; FINAL may be ~0.5× (staging). Prod load may slow throughput.
- Chunk United East / CVS monthly. Validate one large pilot before fleet.
