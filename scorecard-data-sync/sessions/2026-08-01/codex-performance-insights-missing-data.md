# Performance Insights missing scorecard incident

## Context

- Incident channel: `#p1-2026-08-01-p0-missing-data-in-performance-insights-for-7-31`
- Linear: [INSI-4251](https://linear.app/cresta/issue/INSI-4251/missing-data-in-performance-insights-for-731)
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree context: `main` checkout, read-only investigation
- Reported customers: Guitar Center and Home Care Delivered (HCD), both `us-east-1`

## Customer impact

Performance Insights did not show QM scores that were visible on scorecards in Closed Conversations. The data is used for agent performance and month-end incentive/bonus reporting. Backfill jobs reported success but their results either surfaced only after a long delay or did not surface.

## Findings

This is not only a UI problem and it has not fully recovered.

- PostgreSQL continues to receive scorecards and real score values.
- ClickHouse is missing fresh `scorecard_d` rows and, for some Guitar Center rows already present, serves an older all-N/A score version.
- The suspected July 29 `go-servers` PR 30530 did not change score-to-ClickHouse mapping. Its `failed to convert API event to Clickhouse` errors concern new CustomerSnapshot conversation events and are unrelated.
- ClickHouse Distributed queues still contain customer data. On the queried node:
  - Guitar Center: 3 active `scorecard_d` queue paths (13.1 MB) and 3 active `score_d` queue paths (35.6 MB).
  - HCD: 2 active `scorecard_d` queue paths and 3 active `score_d` queue paths (32.6 MB).
- The ClickHouse shared DDL queue contains a score deletion task from 2026-07-30 (`query-0000012747`) with no completion status on all nine hosts. Incident investigation found shard 0 inactive and repeated `Too many watches` errors. Re-score cleanup uses `ALTER TABLE ... ON CLUSTER DELETE`; the stuck DDL prevents cleanup/reinsert completion and cluster instability also delays new Distributed inserts.

This is related to the July 29 missing-rate incident through the same ClickHouse/Keeper/Distributed-delivery failure domain, but the current incident has an additional concrete blocked-DDL symptom. It is therefore unsafe to describe all current gaps as a benign delay that has already resolved.

## Current gap measurements

Measured at approximately 2026-08-01 19:22 UTC. Counts use the scorecard sync monitor's eligibility and time semantics. ClickHouse counts use the same submitted/create-time windows and `_row_exists = 1`.

### Original monitor cohort

Range: `2026-07-31T10:00:07Z` to `2026-08-01T10:00:07Z`.

- Guitar Center: PostgreSQL 49,652 eligible (23 submitted, 49,629 scored unsubmitted); ClickHouse 0 currently visible; gap 49,652 (100%).
- HCD: PostgreSQL 11,954 eligible (63 submitted, 11,891 scored unsubmitted); ClickHouse 4,913 visible; gap 7,041 (58.90%).

The 06:10 EDT monitor had reported Guitar Center 49,630/49,652 missing and HCD 11,900/11,954 missing. HCD has partially caught up and increased from 4,641 to 4,913 visible rows during roughly eight minutes of this investigation, so its queue is actively draining, but the cohort is still materially incomplete. Guitar Center currently has no live rows from that cohort under the monitor's time semantics.

### Newer cohort

Range: `2026-08-01T10:00:07Z` to measurement time.

- Guitar Center: PostgreSQL 24,542 eligible (46 submitted, 24,496 scored unsubmitted); ClickHouse 0 visible; gap 24,542 (100%).
- HCD: PostgreSQL 1,924 eligible (all scored unsubmitted); ClickHouse 648 visible; gap 1,276 (66.32%).

HCD's customer-visible templates may look recovered through July 31 after targeted backfills, but the overall scorecard projection is not caught up and today's ingestion remains incomplete.

## Recommended next steps

1. Treat ClickHouse cluster health as the blocker; do not keep launching broad backfills while shard/DDL execution is unhealthy.
2. Have the ClickHouse owner restore the inactive shard/Keeper health and resolve or remove the stuck distributed DDL task using the normal operational procedure.
3. Verify the distributed queues drain and rerun the same two cohort comparisons.
4. After infrastructure recovery, reindex the narrow affected templates/date ranges with scorecard cleanup, then verify both row presence and that latest Guitar Center score rows are no longer all N/A.
5. Expand impact assessment from customer reports to the 49 `us-east-1` profiles flagged by the 06:10 EDT monitor (75.76% aggregate missing), prioritizing business-critical profiles.

## 19:50 UTC recovery verification

An operator removed `/clickhouse/conversations/task_queue/ddl/query-0000012747` from ZooKeeper.

- Step 1 is complete for the observed blocker:
  - `query-0000012747` is absent from `system.distributed_ddl_queue` on all replicas.
  - The distributed DDL queue currently has no remaining rows.
  - All nine `score` and `scorecard` replicas for both customers report active replicas `3/3`, writable sessions, and no session expiry. One HCD `scorecard` replica retained a replication queue size of 1 with at most one second delay.
- Step 2 is not complete:
  - Guitar Center still has 18 active queue paths for each of `score_d` and `scorecard_d`, totaling approximately 269 MB and 87 MB.
  - HCD still has 18 active `score_d` queue paths (approximately 161 MB) and 17 active `scorecard_d` queue paths (approximately 62 MB).
  - A 45-second resample showed only small byte movement and no queue-path reduction.
  - HCD data is arriving: the original cohort rose from 4,913 to 7,344 visible rows, leaving 4,610/11,954 missing (38.56%); the newer cohort rose from 648 to 1,006, leaving 918/1,924 missing (47.71%).
  - Guitar Center remains at zero visible rows for both measured cohorts.

Conclusion: DDL/shard health is restored, and HCD is catching up, but the customer Distributed queues have not drained. Guitar Center remains the critical holdout.

## 20:40 UTC gap update

- Guitar Center:
  - Original cohort: 0/49,652 visible; gap 49,652 (100%).
  - Since 10:00 UTC: 0/26,116 visible; gap 26,116 (100%).
  - Queued `score_d` + `scorecard_d` data increased to approximately 373 MB across 18 active queue paths per table.
- HCD:
  - Original cohort: 8,094/11,954 visible; gap 3,860 (32.29%).
  - Since 10:00 UTC: 1,114/1,924 visible; gap 810 (42.10%).
  - Queued `score_d` + `scorecard_d` data remains approximately 242 MB across 18 active queue paths per table.

HCD continues to make progress, but producers are also adding queued data and its queues are not empty. Guitar Center has made no visible progress and its backlog is growing despite healthy replicas and an empty distributed DDL queue.

At 20:42 UTC, Guitar Center's backlog contained 9,827 `scorecard_d` data files (97.8 MB) and 8,570 `score_d` data files (275.4 MB), for 18,397 files and approximately 373.2 MB total. All 36 queue paths reported `is_blocked = 0`, `error_count = 0`, no broken files, and no last exception. There is therefore no newly observed hard failure after the DDL repair; recovery is constrained by the large async delivery backlog and new writes arriving at least as fast as delivery.

## 20:57 UTC targeted repair smoke test

Code verification confirmed that untargeted time-range mode invokes only `ReindexProcessScorecardsActivity`. Conversation scorecards must be supplied through `conversation_scorecard_resource_names`; the scorecard sync monitor performs this classification and sends those names to the same `ReindexScorecardsWorkflow`.

After refreshing AWS SSO with the `K8sContributorAccess` role, a monitor job was run for Guitar Center:

- The first range (`10:00:07`–`10:05:07` UTC) contained zero eligible scorecards and correctly created no workflow.
- A populated five-minute range (`13:20:00`–`13:25:00` UTC) contained 124 eligible scorecards, all missing.
- The monitor classified all 124 as conversation scorecards and created one `JOB_TYPE_REINDEX_SCORECARDS` job:
  - Job: `customers/guitar-center/profiles/us-east-1/jobs/4458cb25-0b78-4673-b850-88b6b8470bb2`
  - Workflow: `reindexscorecards-guitar-center-us-east-1-5d174d38-3894-4230-a06f-7a883c559abc`
  - Run: `b3437eb0-e482-4e44-924f-c516bbb4b810`
- Temporal completed successfully in 5.85 seconds.
- Immediate ClickHouse verification still showed 0/124 rows visible. The writes joined the existing Distributed backlog; queued file counts increased to 9,883 for `scorecard_d` and 8,627 for `score_d`.
- A repeat check at 21:00 UTC still showed 0/124 visible. Queue totals had increased again to 9,893 `scorecard_d` files and 8,637 `score_d` files.

The workflow path is functioning, but backfilling cannot restore PI visibility until Distributed delivery resumes. A larger backfill would currently add load without producing visible recovery.

## 22:13 UTC queue check

Guitar Center's queue had not drained; it grew since the 21:00 UTC check:

- `score_d`: 8,876 files / 278.0 MB, up from 8,637 / 276.0 MB.
- `scorecard_d`: 10,132 files / 99.3 MB, up from 9,893 / 98.1 MB.
- No queue paths reported blocked status or errors.

Because the backlog increased rather than drained, the data-gap follow-up was intentionally not run.

## 22:25 UTC gap update

- Original cohort remains unchanged at 0/49,652 visible (100% missing).
- Since 10:00 UTC, PostgreSQL now has 28,122 eligible scorecards while ClickHouse has 0 visible. The gap increased from 26,116 to 28,122 (+2,006).
- The five-minute smoke-backfill cohort remains 0/124 visible.

The data gap is worsening because PostgreSQL production continues while no Guitar Center rows are delivered from the Distributed queue.

## 22:40 UTC exact July 31 comparison

The prior ad hoc ClickHouse time-field queries were not an exact ID comparison and understated the rows currently present. A dry-run of the production scorecard sync monitor was used for the full Guitar Center Pacific-local July 31 day (`2026-07-31T07:00:00Z`–`2026-08-01T07:00:00Z`), joining PostgreSQL candidate IDs against ClickHouse:

- Total eligible: 49,686.
- Missing: 49,509 (99.64%).
- Present in ClickHouse: 177.
- Timestamp-stale: 93 (0.19% of total).
- Fully synced: 84 (0.17% of total).
- Submitted: 23 total; 8 missing and 15 stale.
- Unsubmitted: 49,663 total; 49,501 missing, 78 stale, and 84 synced.
- Repair candidates: 49,602, all conversation scorecards.

Rows whose `scorecard_last_update_time` is August 1 can legitimately belong to the July 31 cohort. Their existence does not contradict the gap: only 177 of the 49,686 July 31 scorecards are present at all, and 93 of those are stale.

## Score-to-scorecard version mismatch

For template `019ee2b2-9b0d-75e7-acae-e730c3fa6f43`, `score_d` is materially ahead of `scorecard_d`. Comparing the maximum score version to the maximum scorecard version per scorecard ID (with `_row_exists = 1`) produced:

- July 29: 1,922 score IDs; 559 missing scorecard rows, 460 with a newer score version than scorecard, 903 equal.
- July 30: 3,176 score IDs; 163 missing scorecard rows, 2,137 with a newer score version than scorecard, 876 equal.
- July 31: 1,757 score IDs; 1,560 missing scorecard rows, 77 with a newer score version than scorecard, 120 equal.
- August 1: 213 score IDs; all 213 missing scorecard rows.

There were zero cases where the newest score version was older than the newest scorecard version. This directional skew supports partial asynchronous delivery: `score_d` and `scorecard_d` use independent Distributed queues, and score delivery has advanced farther than scorecard delivery. At measurement time, `score_d` still had 8,977 queued files (279 MB) and `scorecard_d` had 10,233 files (100 MB), with no explicit blocked paths or errors.

The original diagnostic's `version_matches` metric is less strict: it counts a scorecard ID if any historical score row matches the latest scorecard version, even when a newer score version also exists. Comparing max-to-max is the appropriate way to identify latest projection skew.

## 00:34 UTC August 2 queue and smoke check

- `score_d`: 9,356 files / 298.5 MB, up from 8,876 / 278.0 MB at 22:13 UTC.
- `scorecard_d`: 10,612 files / 105.3 MB, up from 10,132 / 99.3 MB.
- No queue paths reported blocked status or errors.
- The production monitor's exact ID comparison still reports all 124 smoke-backfill scorecards missing (100%).

The queue continues to grow, and none of the smoke cohort has reached the local ClickHouse tables.

## 01:14 UTC August 2 regional ClickHouse status

Groundcover was not connected in the active tool session, so the equivalent underlying Kubernetes and ClickHouse system signals were checked directly.

- All nine `conversations` ClickHouse pods are Running with zero restarts since their restart approximately six hours earlier.
- All five Keeper pods are Running with zero restarts.
- Across `system.replicas`: zero read-only replicas, zero expired sessions, no queue-update exceptions, and at most one second of replication delay.
- Six replica observations report 2/3 active replicas only for the unrelated `cocacola_us_east_1.user_outcome_field_value` table.
- No Keeper session-expiration or “Too many watches” errors appeared in the last 15 minutes.
- Recent conversation-cluster errors were primarily broken pipes and client-cancelled queries, concentrated on the `*-2-0` pods. Those pods use 118–124 GiB against 360 GiB limits and approximately 2.5–2.7 CPU cores against 36 requested, so they are not near resource limits.
- The regional Distributed backlog remains severe: 6,357,084 files / 107.6 GB across 61 customer databases, with zero blocked paths and maximum error count 1.
- Guitar Center continues growing slowly:
  - `score_d`: 9,380 files / 298.7 MB.
  - `scorecard_d`: 10,636 files / 105.4 MB.
- A request-log ClickHouse pod is crash-looping, but it is a separate ClickHouse installation from `conversations`.

Assessment: the conversation cluster's pods, Keeper sessions, and replication are currently stable, but asynchronous Distributed delivery is operationally unhealthy due to the enormous regional backlog. The cluster is not resource-saturated according to current pod CPU/memory usage, and the system queue exposes no current hard error explaining why delivery throughput remains below ingress.

## Resolution on August 2

The regional backlog was confirmed as the root cause. ClickHouse's `BackgroundDistributedSchedulePoolTask` was saturated at 16/16 on every active node while serving 3,328 Distributed tables. With background batching disabled, each small spool file required an individual send; approximately 6.36 million files / 107.6 GB accumulated across 61 databases without reporting blocked queues or insert failures.

Two production changes resolved the incident:

- [PR 312915](https://github.com/cresta/flux-deployments/pull/312915) enabled `distributed_background_insert_batch=1` and `distributed_background_insert_split_batch_on_failure=1`.
- [PR 312923](https://github.com/cresta/flux-deployments/pull/312923) increased `background_distributed_schedule_pool_size` from 16 to 48 and rolled the nine Conversations nodes.

After batching and restart, drain throughput rose from approximately 2 to 62 files/second, then accelerated. Guitar Center data appeared at 01:09 EDT, other customers caught up by 01:11, and the queue fell to approximately 600,000 files by 01:17. PagerDuty marked the incident resolved.

At 11:59 EDT:

- the region had only 224 files / 2.5 MB remaining;
- Guitar Center and HCD had zero queued `conversation_d`, `score_d`, and `scorecard_d` files;
- batch and split-on-failure settings were active;
- all nine nodes reported sender pool size 48.

The complete synthesis is in `deliverables/2026-08-us-east-distributed-backlog-incident.md`.
