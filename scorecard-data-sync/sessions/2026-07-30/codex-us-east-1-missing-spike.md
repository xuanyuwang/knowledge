# US East scorecard missing-rate spike

## Incident

The 2026-07-29 10:01 UTC scorecard sync monitor reported 376,676/3,245,817 rows missing (11.60%) across 44 `us-east-1` profiles. The next run remained elevated at 434,921/3,272,355 (13.29%).

## Root cause

The alert was primarily a ClickHouse Distributed-table delivery-lag incident, not loss at the PostgreSQL-to-ClickHouse producer and not a Temporal reindex heartbeat failure.

For Acorns:

- PostgreSQL created 2,183 eligible unsubmitted scorecards at 09:40-09:41 UTC.
- ClickHouse `query_log` shows successful `INSERT INTO scorecard_d` requests at 09:42 UTC totaling exactly 2,183 rows.
- At 10:01:07 UTC, the monitor's distributed read reached the three local shards and returned only 14 + 17 + 24 = 55 rows. This exactly explains the Slack result: 2,183 missing out of 2,238.
- `part_log` shows the queued rows reaching local `scorecard` tables only from 10:57 through 11:16 UTC.
- The rows now exist on all nine replicas; the sampled row retained `update_time=09:42:48`, proving it was produced before the monitor but delivered after it.

`system.distribution_queue` still showed queued files for `scorecard_d` and related Distributed tables. `system.replicas.last_queue_update_exception` showed `KEEPER_EXCEPTION: Session expired` broadly. This coincided with the region's ClickHouse kernel/overlayfs incident and ClickHouse/Keeper pod churn.

## Rejected hypotheses

- **Temporal repair heartbeat timeouts:** these can make monitor-created repairs fail, but cannot explain the initial misses. The missing rows were already accepted by ClickHouse before the monitor.
- **Apiserver rollout regression:** the July 28 image rollout was a tempting time correlation, but relevant scorecard/ClickHouse indexing code and configuration did not change.
- **AutoQA permanently skipping ClickHouse:** the AutoQA path does skip a direct ClickHouse reindex, but the exact rows were present in successful ClickHouse distributed insert queries.
- **Permanent replica divergence:** all replicas currently have identical Acorns batch counts. The issue was delayed Distributed-table fan-out, not lasting row loss.

## Operational implications

- Do not launch large manual backfills solely from this alert until checking `system.query_log`, `system.part_log`, and `system.distribution_queue`; queued original writes may still arrive and make the backfill redundant.
- Monitor Distributed queue age/file count and Keeper session failures alongside scorecard missing rate.
- Consider making the monitor distinguish rows absent from local tables but still queued in `scorecard_d`, or add a grace period based on Distributed queue age.
