# CLO MV `TO target_table` Proposal

**Date:** 2026-07-30
**Source repo:** `/Users/xuanyu.wang/repos/clickhouse-schema`
**Branch/worktree context:** Read-only inspection of `main` and GitHub commit `fdc0b755` for INSI-4097 runbook
**Related ticket:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via)

## Objective

After ClickHouse expert feedback pointing to the INSI-4097 production pattern, create a new deliverable focused on the `TO target_table` approach for the CLO MV rollout.

## Inputs reviewed

- `deliverables/clo-mv-without-populate-proposal.md`
- `deliverables/clo-mv-clickhouse-creation-investigation.md`
- GitHub: `queries_fix_metadata_moment_value_count_replicated.sql` at `fdc0b755`
- Local: `init_db.up.sql` metadata MV and `metadata_moment_value_count` inline MV definitions
- `go-servers-convi-7383/shared/clickhouse/testing/schemas/ch_conv_schema.sql` CLO MV draft columns

## INSI-4097 pattern extracted

1. Drop distributed, then old inline MV.
2. Create replicated **storage table** (`metadata_moment_value_count`).
3. Create trigger MV with `TO storage` and no `POPULATE`.
4. Recreate distributed over **storage**, keeping `_mv_d` read name.
5. Backfill with `INSERT INTO storage SELECT ... FROM moment_annotation_d`.

Key lifecycle benefit: trigger can be dropped/recreated without losing storage data.

## Outcome

Created `deliverables/clo-mv-to-target-table-proposal.md` recommending:

- `moment_annotation_by_conversation_outcome` storage table
- `moment_annotation_mv_by_conversation_outcome` trigger with `TO`
- `moment_annotation_mv_by_conversation_outcome_d` distributed over storage
- 180-day chunked backfill for selected large customers
- explicit note that ReplacingMergeTree overlap handling differs from INSI's uniqState idempotency

## Follow-up

- Align `clickhouse-schema` DDL and go-servers distributed local table name with storage table naming.
- Prototype in staging and validate shard-safe chunked backfill before prod rollout to selected large customers.
