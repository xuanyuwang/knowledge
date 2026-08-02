# US East ClickHouse Distributed backlog incident

## Executive summary

From July 29 through August 2, 2026, `us-east-1-prod` accumulated a region-wide ClickHouse Distributed-table spool backlog. The first visible symptom was a sudden increase in the scorecard PG-to-CH missing-rate monitor. It later surfaced as missing Performance Insights data for Home Care Delivered and Guitar Center, score/scorecard version mismatches, successful backfills that did not become queryable, and eventually stalled conversation indexing for most tenants.

The root cause was throughput starvation in ClickHouse's asynchronous Distributed sender:

- inserts to `<tenant>.<table>_d` were acknowledged after being persisted as local spool files;
- background delivery was not batched, producing millions of small files and one network send per file;
- `BackgroundDistributedSchedulePoolTask` was saturated at 16/16 on every active node while serving 3,328 Distributed tables;
- incoming files accumulated faster than the sender could drain them, with per-table backoff reaching up to 30 seconds.

Nothing appeared failed: replicas and Keeper were healthy after the earlier instability, inserts returned success, and queue rows reported `is_blocked=0` and `error_count=0`. The queue was starved, not broken. No data was lost; it remained on source-node disks until delivered.

The incident was resolved by enabling batched Distributed sends and split-on-failure, increasing the sender pool from 16 to 48, and rolling the Conversations ClickHouse nodes. The queue fell from approximately 6.36 million files / 107.6 GB to 600,000 files by 01:17 EDT on August 2, and to 224 files / 2.5 MB by 11:59 EDT. Guitar Center and HCD's `conversation_d`, `score_d`, and `scorecard_d` queues were fully drained.

## Timeline

### July 25–28: infrastructure instability establishes the conditions

The `us-east-1-prod` Conversations ClickHouse cluster experienced pod churn and Keeper session expiration during a kernel/overlayfs incident. This was relevant context and contributed to delayed delivery, but it did not fully explain the multi-day accumulation ultimately found.

### July 29: missing-rate monitor is the first strong symptom

At 10:01 UTC, the scorecard sync monitor reported:

- 376,676 / 3,245,817 missing (11.60%);
- 44 affected `us-east-1` profiles.

The next run rose to 434,921 / 3,272,355 (13.29%).

Acorns provided the first exact proof of asynchronous delivery lag:

- PostgreSQL created 2,183 eligible scorecards at 09:40–09:41 UTC.
- Successful `INSERT INTO scorecard_d` queries accepted exactly 2,183 rows at 09:42.
- At 10:01, shard-local reads found only 55 rows, reproducing the monitor alert.
- The remaining parts reached local tables between 10:57 and 11:16.

This ruled out producer loss and established that a successful Distributed insert did not mean the data was queryable.

Initial hypotheses included:

- Temporal repair heartbeat failures;
- a July 28 apiserver rollout regression;
- AutoQA permanently skipping ClickHouse;
- permanent replica divergence;
- ClickHouse/Keeper instability.

The first four were rejected. Keeper instability explained acute delays, but the investigation had not yet quantified the accumulating Distributed spool or sender capacity.

### July 31–August 1: the regional gap becomes severe

The August 1 06:10 EDT monitor, covering the prior 24 hours, reported:

- 2,184,453 / 2,883,271 scorecards missing (75.76%);
- 49 affected `us-east-1` profiles;
- Guitar Center: 49,630 / 49,652 missing (99.96%);
- HCD: 11,900 / 11,954 missing (99.55%).

This was a regional projection failure, not a two-customer product bug.

### August 1 morning: customer-visible incident

HCD reported missing QM data for Intake and Recur in Performance Insights, affecting agent performance and month-end bonus reporting. Guitar Center then reported the same symptom.

Observed behavior:

- conversations and annotations existed in Closed Conversations;
- PostgreSQL scorecards and real scores continued to be written;
- Performance Insights was empty or returned N/A;
- scorecard, annotation, and reindex jobs reported success but did not restore visibility;
- HCD partially recovered after a long delay, while Guitar Center remained incomplete.

The incident was tracked in [INSI-4251](https://linear.app/cresta/issue/INSI-4251/missing-data-in-performance-insights-for-731).

### August 1 afternoon: hypotheses narrowed

Investigators considered:

- a Performance Insights query/UI bug;
- a recent score-to-ClickHouse mapping regression;
- CustomerSnapshot conversion errors introduced by PR 30530;
- stale all-N/A score versions;
- a stuck distributed DDL task, `query-0000012747`;
- delayed or partial delivery between `score_d` and `scorecard_d`.

Code review ruled out PR 30530 and score mapping changes. CustomerSnapshot errors concerned conversation events and were unrelated.

The stuck DDL task was real: an inactive shard prevented an `ALTER TABLE ... ON CLUSTER DELETE` from completing. Removing `/clickhouse/conversations/task_queue/ddl/query-0000012747` restored DDL and replica health, but the customer Distributed queues continued to grow. This was a contributing obstruction for re-score cleanup, not the root cause of the region-wide ingestion stall.

### August 1 evening: smoke backfill proves ACK is not delivery

An exact July 31 monitor comparison for Guitar Center found:

- 49,686 eligible scorecards;
- 49,509 missing (99.64%);
- 93 timestamp-stale;
- 84 fully synced.

A five-minute smoke repair targeted 124 missing conversation scorecards through the same `ReindexScorecardsWorkflow` used by the monitor:

- Temporal completed successfully in 5.85 seconds.
- Immediately afterward, 0 / 124 were visible in local ClickHouse tables.
- Hours later, all 124 remained missing while queue file counts increased.

This demonstrated that the application and Temporal paths were functioning through the Distributed-table acceptance boundary. Additional backfills only added spool files and could not restore visibility while delivery was starved.

The independent queues also explained partial projections. For Guitar Center template `019ee2b2-9b0d-75e7-acae-e730c3fa6f43`, the newest `score_d` version was frequently ahead of `scorecard_d`, and never behind it:

- July 29: 559 missing scorecard companions; 460 score versions ahead.
- July 30: 163 missing companions; 2,137 score versions ahead.
- July 31: 1,560 missing companions; 77 score versions ahead.
- August 1: all 213 score IDs lacked scorecard companions.

### August 2 00:08 EDT: backlog identified as root cause

The regional queue was measured at:

- 6.36 million files;
- 107.6 GB compressed;
- 61 customer databases;
- approximately 17 KB per file on average.

Largest queues included NCLH, United East, Marriott, Alaska Air, RCG, Rentokil, Lending Club, and Guitar Center. Guitar Center alone held 51,301 files / 2.77 GB across all Distributed tables.

Further investigation established the capacity mechanism:

- 3,328 Distributed tables competed for the sender pool;
- `BackgroundDistributedSchedulePoolTask` was 16/16 saturated on every active node;
- unbatched delivery performed one network round trip per small file;
- per-table scheduling backoff could grow to `distributed_background_insert_max_sleep_time_ms=30000`;
- arrival exceeded drain, producing an unbounded spool.

The suppressed `BgDistSchPool` “Temporarily pause scheduling of tasks” log had previously been treated as benign when queues were only 5–9 files per node. At incident scale, it was the missing early-warning signal.

### August 2 00:23–01:17 EDT: fixes and recovery

[flux-deployments PR 312915](https://github.com/cresta/flux-deployments/pull/312915), merged at 00:27 EDT, enabled:

- `distributed_background_insert_batch = 1`;
- `distributed_background_insert_split_batch_on_failure = 1`.

Batching coalesces many spool files into one send. Split-on-failure prevents one rejected or oversized batch from wedging a table's queue.

Initial drain throughput improved from approximately 2 files/second to 62 files/second but remained slow.

[flux-deployments PR 312923](https://github.com/cresta/flux-deployments/pull/312923), merged at 00:46 EDT, increased:

- `background_distributed_schedule_pool_size`: 16 → 48.

This server-level setting caused a rolling restart of the nine Conversations nodes. The restart plus expanded pool activated effective batched draining.

Recovery milestones:

- 01:09 EDT: Guitar Center data became visible.
- 01:11 EDT: other affected customers caught up.
- 01:17 EDT: queue reduced to approximately 600,000 files; incident resolved.
- 07:58 EDT: PagerDuty formally marked resolved, citing a ClickHouse backlog drained after restart.
- 11:59 EDT: only 224 files / 2.5 MB remained region-wide; Guitar Center and HCD had zero queued `conversation_d`, `score_d`, or `scorecard_d` files.

All nine Conversations nodes currently report pool size 48. The batch and split-on-failure settings are active.

## Root cause

The primary root cause was a Distributed-table small-file and sender-capacity mismatch:

1. High-volume application writes generated one local spool file per asynchronous Distributed insert.
2. Background batching was disabled.
3. A 16-thread scheduling pool was shared across 3,328 Distributed tables.
4. Millions of small files created high per-file scheduling, filesystem, and network overhead.
5. Ingress exceeded delivery throughput, so queues grew while all API calls and health checks remained successful.

Earlier Keeper/pod instability likely accelerated accumulation, and the stuck DDL task blocked a specific re-score cleanup path, but neither explains the regional, error-free, multi-table backlog as completely as sender-pool saturation and unbatched delivery.

## Why the symptoms were misleading

- **Successful backfill:** meant the Distributed table persisted the spool files, not that shard-local tables received them.
- **Healthy replicas:** replication begins after Distributed delivery; it cannot report data still upstream in the spool.
- **No queue errors:** the queue was throughput-starved rather than blocked.
- **Partial PI data:** `score_d`, `scorecard_d`, conversations, and annotations have independent queues and can arrive at different times.
- **Customer-specific timing:** independent source-node/table/shard queues drained at different rates, so HCD recovered before Guitar Center and other larger tenants stalled earlier.

## Rejected or secondary hypotheses

- **Score mapping regression / PR 30530:** ruled out by code history; related errors were for CustomerSnapshot events.
- **Temporal heartbeat timeout:** could affect repair workflows but not the original accepted-yet-invisible writes.
- **Performance Insights-only bug:** contradicted by direct PG/CH gaps across conversations and scorecard tables.
- **Permanent data loss:** contradicted by persisted queue files and recovery after drainage.
- **Stuck DDL task as sole root cause:** removing it restored DDL health but did not stop queue growth.
- **EBS IOPS change PR 311481:** did not alter `us-east-1-prod` Conversations volumes and was not the direct cause.

## Follow-up actions

1. Alert on regional and per-tenant `system.distribution_queue` file count, bytes, and oldest-file age.
2. Alert on `BackgroundDistributedSchedulePoolTask` utilization and replace the suppressed `BgDistSchPool` signal with a rate-limited actionable metric.
3. Keep batching and split-on-failure enabled; evaluate applying these defaults to other regions after measuring their queues.
4. Capacity-plan `background_distributed_schedule_pool_size` against the number of Distributed tables and observed file arrival rate.
5. Change application/Temporal success semantics or observability so “accepted into Distributed spool” is distinguishable from “queryable on shard-local tables.”
6. Do not launch broad backfills while the original writes remain queued; backfills amplify the incident.
7. Enhance the scorecard sync monitor to identify candidate IDs still present in the Distributed spool or apply a queue-aware grace period.
8. Add an incident runbook: inspect `system.distribution_queue`, sender-pool saturation, and shard-local `part_log` before treating PG/CH gaps as loss.

## Sources

- [Incident Slack channel](https://cresta.enterprise.slack.com/archives/C0BMCK2301Y)
- [INSI-4251](https://linear.app/cresta/issue/INSI-4251/missing-data-in-performance-insights-for-731)
- [PR 312915: batch Distributed sends](https://github.com/cresta/flux-deployments/pull/312915)
- [PR 312923: sender pool 16 → 48](https://github.com/cresta/flux-deployments/pull/312923)
- `sessions/2026-07-30/codex-us-east-1-missing-spike.md`
- `sessions/2026-08-01/codex-performance-insights-missing-data.md`
