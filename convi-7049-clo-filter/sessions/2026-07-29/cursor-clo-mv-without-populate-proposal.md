# CLO MV Without `POPULATE` Proposal

**Date:** 2026-07-29
**Source repo:** `/Users/xuanyu.wang/repos/clickhouse-schema`
**Branch/worktree context:** Not inspected or modified; proposal derived from the existing ClickHouse creation investigation
**Related ticket:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via)

## Objective

Create a brief proposal that can be shared with ClickHouse-experienced coworkers to review creation and backfill of the CLO materialized view without `POPULATE`.

## Inputs

- `deliverables/clo-mv-clickhouse-creation-investigation.md`
- Existing project state in `project.yaml` and `README.md`
- ClickHouse materialized-view and backfilling guidance linked by the investigation

## Outcome

Created and refined `deliverables/clo-mv-without-populate-proposal.md` with:

- the existing inline `moment_annotation_mv_by_metadata` pattern as the primary schema precedent;
- no `POPULATE`;
- chunked 180-day `INSERT INTO ... SELECT` backfill;
- rollout limited initially to selected large customers;
- `TO target_table` as an optional lifecycle-management alternative rather than a backfill requirement;
- explicit warning that `ReplacingMergeTree` deduplication is eventual;
- focused review questions for shard execution, overlap handling, version semantics, validation, and chunk sizing.

## Important refinement

Manual backfill does not require the `TO target_table` form. An inline MV can receive `INSERT INTO ... SELECT` directly; workload control comes from bounded, sequential chunks rather than from the storage topology. The proposal does not assume that overlapping live ingestion and backfill is immediately safe. Rows may remain duplicated until merges, and ordinary queries may observe them.

## Follow-up

Consult ClickHouse reviewers, incorporate their decisions into the exact DDL and production runbook, then validate the approach in staging before enabling the Insights read-path flag.
