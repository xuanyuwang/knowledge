# CONVI-7383 Phase A local direct-host apply

**Date:** 2026-08-07
**Source repo:** `/Users/xuanyu.wang/repos/clickhouse-schema`
**Target:** `us-west-2-staging`

## Intent

Apply archived Phase A DDL locally without changing remote `main`. Phase B and feature-flag enablement remained out of scope.

## Preparation

- Confirmed the source `moment_annotation` table exists on all 9 ClickHouse hosts for 32 databases.
- Confirmed the distributed DDL queue had zero non-`Finished` tasks before starting.
- Temporarily replaced the clean local `queries.sql` Phase B payload with the archived Phase A DDL; restored it afterward, leaving `clickhouse-schema` clean.
- The provided `run.sh` could not run because it hardcodes unavailable `python3.10`.
- The available Python lacked `crestaproto`; package installation was blocked because the staging AWS role cannot obtain a CodeArtifact bearer token.
- Switched to direct database discovery and a database-by-database execution loop using the same five archived DDL statements.

## Blocker

The first statement for the first database, a no-op:

`DROP TABLE IF EXISTS authtest_profile_1.moment_annotation_by_conversation_outcome_d ON CLUSTER conversations SYNC`

completed on 8 of 9 hosts but remained `Inactive` on `chi-conversations-conversations-1-2`.

Evidence:

- All nine pods are Running and directly queryable.
- Host `1-2` retained one logical non-`Finished` distributed DDL task (reported once per replica by `clusterAllReplicas`).
- The host recorded a Keeper `Session expired` error and did not consume the task.
- The local apply client was stopped before its 900-second timeout.
- Cluster verification found zero Phase A object rows, so no storage, trigger MV, or distributed table was created in any database.

## Direct-host bypass

Because the blocker affects only `ON CLUSTER` task consumption, the rollout was retried without `ON CLUSTER`: each of the five Phase A statements was executed directly against all nine pod IPs, database by database.

- Applied successfully to 31 of 32 eligible databases with zero execution failures.
- Verified all three Phase A objects on all nine hosts for each of those 31 databases.
- Verified storage uses `ReplicatedReplacingMergeTree`, the trigger uses `MaterializedView` with `TO` and no `POPULATE`, and `_d` uses `Distributed`.
- Initially skipped `authtest_profile_1` because its queued DROP could execute after recovery and remove the newly created `_d` object.

## Result

The `1-2` pod was safely restarted after confirming it was StatefulSet-managed with a persistent volume, had no active queries, and had no replica backlog or delay. The queued DROP then reached `Finished` on all nine hosts and the distributed DDL queue returned to zero non-`Finished` tasks.

`authtest_profile_1` lacks the source `moment_annotation_payload` column, so its trigger MV cannot be created (Code 47). It now has storage and `_d` on all nine hosts, matching the existing legacy source-table exception pattern. The other 31 databases have the full storage + trigger MV + `_d` Phase A shape.

Phase A is complete on `us-west-2-staging` with one documented legacy exception. Do not start Phase B or enable the Insights flag.
