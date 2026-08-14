# CONVI-7383 Phase A blocker recheck

**Date:** 2026-08-06
**Source repo:** `/Users/xuanyu.wang/repos/clickhouse-schema`
**Branch/ref checked:** `origin/main` at `69d6a05d05023fea18db9c70bee50352c12d2507`

## Scope

Read-only readiness check for the unfinished Phase A DDL rollout. No schema workflow was triggered and Phase B was not started.

## Results

- `us-west-2-staging` infrastructure is now healthy:
  - `chi-conversations-conversations-0-0-0` is `2/2 Running`, with `0` restarts and age `20h`.
  - Direct ClickHouse `SELECT 1` succeeded.
  - `clusterAllReplicas('conversations', system.distributed_ddl_queue)` returned zero rows whose status was not `Finished`.
- `ca-central-1-prod` remains blocked:
  - TLS connection to `clickhouse-conversations.ca-central-1-prod.internal.cresta.ai:9440` timed out after approximately 30 seconds.
  - This matches the earlier Code 209 connection timeout, so no schema retry should be started yet.
- `origin/main` still contains the Phase B backfill `INSERT` in `queries.sql` at commit `69d6a05`.
  - Before applying Phase A to `us-west-2-staging`, restore the archived Phase A steps 1-5 to `queries.sql`, commit and push to `main`, then trigger the main-ref GHA.
  - Keep `use_conversation_outcome_moment_annotation_materialized_view` off.

## Conclusion

The staging infrastructure blocker is cleared, but the Phase A apply is not immediately runnable until `queries.sql` on `main` is switched back from Phase B to Phase A. ca-central remains infrastructure-blocked.
