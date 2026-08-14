# us-west-2 scorecard missing-rate spike

## Context

- Date: 2026-08-04
- Source repo: `go-servers`
- Branch/worktree: `main` at `/Users/xuanyu.wang/repos/go-servers`
- Region: `us-west-2-prod`
- Monitor range: `2026-08-03T10:00:09Z` to `2026-08-04T10:00:09Z`

## Reported symptom

The scorecard sync monitor compared 2,076,359 eligible PostgreSQL scorecards and reported 411,246 missing from ClickHouse (19.81%) across 43 profiles. Several customers were completely missing, while large tenants such as Airbnb, Frontdoor, Oportun, CVS, and Renuity had substantial partial gaps.

## Live ClickHouse evidence

Read-only checks through Oportun's `us-west-2` Conversations ClickHouse cluster found the same immediate failure mode as the August 1 `us-east-1` incident:

- regional `system.distribution_queue`: 1,478,685 files / 21.14 GiB across 66 databases on the first check;
- queue composition included 181,515 `score_d` files and 165,747 `scorecard_d` files;
- the largest customer queues overlapped the monitor outliers:
  - CVS: 250,594 files total;
  - Oportun: 172,970;
  - Snap Finance: 157,225;
  - Cardworks: 106,079;
  - Frontdoor: 102,397;
  - Airbnb: 74,928;
  - Renuity: 64,848;
- queue paths were not blocked and almost all had zero errors;
- replicas were healthy: zero readonly replicas, zero expired sessions, zero replica delay, and zero queue exceptions;
- all six nodes carrying Distributed spool files reached `BackgroundDistributedSchedulePoolTask=48/48` at 20:26 UTC. Three nodes with no spool files were idle.

The queue was not recovering decisively:

- 20:26:01 UTC: 1,476,798 files / 21.17 GiB;
- 20:29:15 UTC: 1,473,624 files / 21.30 GiB.

File count fell by only 3,174 over 194 seconds while compressed bytes increased by approximately 130 MiB. This means delivery was occurring, but new/larger writes were still arriving and the sender remained effectively saturated.

## Comparison with us-east-1

The immediate mechanism is the same: Distributed inserts are accepted into local spool files, but background delivery to shard-local tables cannot keep up. The monitor then sees PostgreSQL rows that are not yet queryable in ClickHouse. Healthy replicas and successful reindex workflows do not disprove this because replication starts only after Distributed delivery.

The exact underlying configuration state differs:

- East was diagnosed with batching disabled and a 16-thread sender pool.
- West currently reports `distributed_background_insert_batch=1`, split-on-failure enabled, and pool size 48 on all nine nodes.
- Conversations pods were rolled between 18:09 and 18:46 UTC, after the 10:00 UTC monitor run. A settings-profile DDL at 17:37 failed because the XML-backed profile is read-only, followed by successful config reloads. Current settings therefore prove the remediation is active now, but do not establish what was active during the alert window.

The strongest current conclusion is: same regional Distributed-backlog failure mode; the remaining proximate cause is either backlog accumulated before today's remediation rollout or sender capacity still being insufficient for West's sustained write rate.

## Operational implication

Do not raise the monitor's auto-heal threshold or launch broad manual backfills while this queue remains large and saturated. Reindex jobs add more spool files and cannot make data queryable faster than the Distributed sender. First verify a sustained decline in files, bytes, and oldest queued work; then rerun the monitor and repair only residual IDs.

## Recovery check at 22:22 UTC

The regional queue subsequently drained:

- files: 1,473,624 at 20:29 UTC to 210 at 22:22 UTC;
- compressed bytes: 21.30 GiB to 1.90 MiB;
- blocked paths and queue errors: zero;
- sender utilization: no longer saturated, with the busiest spool-bearing nodes at 36/48 tasks.

This is a reduction of more than 99.98% and constitutes effective queue recovery. The next safe action is to rerun the scorecard sync monitor without broad auto-healing, measure the residual PG/CH gap after delivery, and backfill only IDs that remain missing.

## Sources

- User-provided scorecard sync summary for the 24-hour window ending 2026-08-04 10:00 UTC.
- Live `system.distribution_queue`, `system.metrics`, `system.settings`, `system.server_settings`, `system.replicas`, `system.distributed_ddl_queue`, and `system.errors` queries at 20:22–20:29 UTC.
- Kubernetes Conversations and Keeper pod status from `us-west-2-prod`.
- `deliverables/2026-08-us-east-distributed-backlog-incident.md`.
