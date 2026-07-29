# Session: ClickHouse conversations MV patterns (read-only)

Date: 2026-07-28
Repo: `/Users/xuanyu.wang/repos/clickhouse-schema`
Scope: Materialized view creation, POPULATE, TTL, backfill, deployment safety for conversations cluster.

## Verdicts (short)

1. **Canonical DDL** lives in `conversations/migrations/20230824160348_init_db.up.sql`. New customer DBs apply this via `create_database_and_tables_if_absent` → `golang-migrate`. Existing DBs get ad-hoc SQL via `run_queries_in_existing_database` reading `queries.sql` (currently ALTER only; MV recreate history archived under `history_queries/`).
2. **POPULATE is always used** on CREATE MATERIALIZED VIEW (init_db + all history query files). No `WITHOUT POPULATE` / separate `INSERT SELECT` MV backfill scripts in this repo. Production-load warnings exist (off-peak, one MV at a time, timeouts) but not a dedicated “POPULATE is expensive” doc sentence beyond that.
3. **Historical fill** for MVs = `POPULATE AS SELECT ... FROM <source>` at CREATE time. Source-table historical data comes from indexing / batch-processor reindex (`historic-analytics`), not from MV-specific backfill jobs.
4. **TTL**: only `delete_conversation_queue` (2 WEEK) in conversations; `RequestLog` (2 WEEK) in request_log. **No TTL on moment_annotation family or any MV.**
5. **Deployment**: dry-run, after 6pm PST, all 7 k8s clusters, bump `clickhouseschema` in insights-batch-processor.
6. **Recreate ordering**: DROP distributed `_d` first, then local MV with `SYNC`, then CREATE MV (POPULATE), then CREATE distributed. Documented in `queries_add_partitions_to_materialized_views.sql` (INSI-1073 / #104).
7. **180-day**: no pattern found in clickhouse-schema or historic-analytics (no TTL 180, no date-filtered INSERT SELECT for MVs).

## Key paths

- `/Users/xuanyu.wang/repos/clickhouse-schema/conversations/migrations/20230824160348_init_db.up.sql`
- `/Users/xuanyu.wang/repos/clickhouse-schema/conversations/scripts/run_queries_in_existing_database/`
- `/Users/xuanyu.wang/repos/clickhouse-schema/conversations/scripts/run_queries_in_existing_database/history_queries/queries_add_partitions_to_materialized_views.sql`
- `/Users/xuanyu.wang/repos/clickhouse-schema/README.md`
