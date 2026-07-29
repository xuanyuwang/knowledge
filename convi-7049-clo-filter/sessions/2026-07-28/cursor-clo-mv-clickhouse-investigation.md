# Session: ClickHouse CLO MV creation / backfill / TTL investigation

Date: 2026-07-28
Source repo: `/Users/xuanyu.wang/repos/clickhouse-schema`
Related: CONVI-7383, CONVI-7049; cresta-proto PR #9400 merged (config sync in flight)

## Inputs

- User requirements: 180-day backfill; clarify POPULATE vs separate backfill job; production load safety; whether TTL is needed; include general MV background.
- Repo patterns under `clickhouse-schema` (init_db, run_queries history, README).
- ClickHouse official backfilling / CREATE VIEW docs.

## Findings

- Cresta pattern: local MV + Distributed `_d`, always `POPULATE`, monthly partitions on modern MVs; no separate INSERT SELECT backfill scripts; no TTL on moment_annotation family.
- ClickHouse docs discourage POPULATE under live ingest (miss window + heavy scan).
- Prefer for CLO: create without POPULATE → chunked 180d INSERT SELECT → validate → enable Insights flag.
- TTL optional; default no TTL; partitions + type filter first.

## Artifact

`deliverables/clo-mv-clickhouse-creation-investigation.md`
