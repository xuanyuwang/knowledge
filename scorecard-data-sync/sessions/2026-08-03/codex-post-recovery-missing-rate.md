# Post-recovery scorecard missing-rate verification

## Context

- **Source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Branch/worktree:** `main` checkout, read-only operational verification
- **Cluster:** `us-east-1-prod`
- **Monitor Job:** `scorecard-sync-monitor-jul28-1785763803`
- **Range:** `2026-07-28T00:00:00Z` through `2026-08-03T13:30:03Z`
- **Profiles checked:** 91
- **Auto-healing:** disabled

The first pod could not obtain an AWS CNI address and remained in `ContainerCreating`. It was deleted, and the Job controller scheduled a replacement pod that completed successfully at `2026-08-03T13:38:39Z`. The Job's failed count of one represents the recycled pending pod, not a failed monitor task. The completed run reported `errors=0`.

## Regional result

- Eligible scorecards: 16,858,725
- Missing: 56,676 (0.3362%)
- Stale: 1,073 (0.0064%)
- Profiles with at least one missing row: 19
- Profiles with zero missing rows: 72
- Critical profiles: 1
- Warning profiles by overall missing rate: 0

The regional aggregate is now far below the July 29–August 1 incident rates. Almost all remaining missing rows belong to one customer, CNG.

## Reported customers

### Guitar Center

- Total: 246,129
- Missing: 0 (0.00%)
- Stale: 4 (rounded overall stale rate 0.00%)
- Submitted: 182 total, 0 missing, 4 stale
- Unsubmitted: 245,947 total, 0 missing, 0 stale

### Home Care Delivered

- Total: 57,360
- Missing: 0 (0.00%)
- Stale: 10 (0.02%)
- Submitted: 318 total, 0 missing, 10 stale
- Unsubmitted: 57,042 total, 0 missing, 0 stale

Both customer gaps from the incident are fully delivered. Only a small submitted-scorecard timestamp-staleness tail remains.

## Profiles with nonzero missing rows

| Profile | Status | Total | Missing | Missing rate | Stale |
| --- | --- | ---: | ---: | ---: | ---: |
| cng/us-east-1 | CRITICAL | 514,722 | 54,819 | 10.65% | 27 |
| ecpi-uni/us-east-1 | OK | 106,281 | 934 | 0.88% | 0 |
| pack-rat/us-east-1 | OK | 95,548 | 88 | 0.09% | 0 |
| mason/us-east-1 | OK | 42,740 | 22 | 0.05% | 0 |
| comerica-east/us-east-1 | OK | 52,601 | 16 | 0.03% | 0 |
| rcg/us-east-1 | OK | 771,953 | 120 | 0.02% | 290 |
| lending-club/us-east-1 | OK | 240,679 | 42 | 0.02% | 37 |
| tailorcare/us-east-1 | OK | 19,666 | 4 | 0.02% | 0 |
| ralphlauren/us-east-1 | OK | 15,231 | 3 | 0.02% | 0 |
| hancock-whitney/us-east-1 | OK | 12,411 | 2 | 0.02% | 0 |
| united-east/us-east-1 | OK | 7,137,201 | 432 | 0.01% | 163 |
| nrg/us-east-1 | OK | 945,735 | 99 | 0.01% | 82 |
| sunbit/us-east-1 | OK | 682,011 | 37 | 0.01% | 46 |
| alaska-air/us-east-1 | OK | 645,794 | 33 | 0.01% | 46 |
| mm-ohio/us-east-1 | OK | 66,753 | 8 | 0.01% | 0 |
| alert360/us-east-1 | OK | 79,916 | 7 | 0.01% | 34 |
| altice/us-east-1 | OK | 65,643 | 7 | 0.01% | 1 |
| dsi-distributing/us-east-1 | OK | 92,068 | 2 | 0.00% | 0 |
| collegeboard/us-east-1 | OK | 23,070 | 1 | 0.00% | 27 |

## CNG outlier

CNG accounts for 54,819 / 56,676 regional missing rows (96.72%). Its overall gap is entirely in the unsubmitted cohort:

- Submitted: 26 total, 0 missing, 8 stale
- Unsubmitted: 514,696 total, 54,819 missing (10.65%), 19 stale
- Reindex candidates: 54,846, all conversation scorecards

Direct ClickHouse verification after the monitor showed zero queued CNG `score_d` or `scorecard_d` files. The regional Distributed queue was also effectively empty at 243 files / 2.16 MB across 61 databases.

Therefore CNG's residual gap is not explained by a currently undrained Distributed queue. It requires a separate cohort-level investigation before deciding whether to backfill.

## Conclusion

The Guitar Center and HCD incident cohorts have recovered. Regionally, the backlog fix is holding: 72/91 profiles have zero missing scorecards and the aggregate missing rate is 0.3362%. The remaining regional number is dominated by a separate CNG gap rather than broad Distributed delivery lag.

## Auto-heal execution

At user request, a second monitor run created backfill jobs for all current missing and stale candidates:

- Kubernetes job: `scorecard-sync-autoheal-jul28-1785764959`
- Range: `2026-07-28T00:00:00Z` through `2026-08-03T13:49:19Z`
- Auto-heal allowlist: all profiles
- Per-profile safety ceiling: 200,000 candidates
- Chunk size: 5,000
- Monitor tasks/errors: 91 / 0
- Reindex jobs created: 47 across 37 profiles
- Unique candidates dispatched: 57,773
  - 56,700 missing
  - 1,073 additional timestamp-stale candidates
- CNG: 54,844 candidates split into 11 jobs
- Guitar Center: one job for 4 stale rows; no missing rows
- HCD: one job for 10 stale rows; no missing rows

All 47 `ReindexScorecards` Temporal workflows completed successfully, including all 11 CNG chunks.

## Post-backfill verification

A dry monitor job, `scorecard-sync-verify-jul28-1785765529`, reran the identical time window after the workflows completed:

- Regional: 26 / 16,908,176 missing (0.0002%) and 3 stale.
- CNG: 0 / 515,002 missing and 0 stale.
- Guitar Center: 0 / 246,575 missing and 0 stale.
- HCD: 0 / 57,714 missing and 0 stale.
- 90 / 91 profiles have zero missing scorecards.
- The only residual gap is Alaska Air: 26 / 646,688 missing overall, all in the submitted conversation-scorecard cohort (26 / 590 submitted, 4.41%).

The Alaska Air auto-heal workflow completed successfully and repaired the seven missing unsubmitted rows plus all 46 stale rows, but the 26 submitted conversation scorecards remained absent. Re-running the same automatic repair would repeat an already successful workflow without evidence it can repair this cohort; these rows need targeted diagnosis rather than another blind retry.

The complete backfill action and verification results were copied to [CONVI-7414](https://linear.app/cresta/issue/CONVI-7414/backfill-missing-scorecards).
