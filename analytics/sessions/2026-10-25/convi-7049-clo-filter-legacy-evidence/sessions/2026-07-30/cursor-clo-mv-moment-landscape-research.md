# CLO MV Moment Landscape Research

**Date:** 2026-07-30
**Source repos:** `/Users/xuanyu.wang/repos/go-servers-convi-7383`, `/Users/xuanyu.wang/repos/clickhouse-schema`
**Related ticket:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via)

## Question

Should the CLO target table reserve room for merging existing MVs or future moment types?

## Method

- Traced `RetrieveQAScoreStats` ClickHouse table routing in go-servers CONVI-7383 branch.
- Inventoried moment-related MVs in clickhouse-schema `init_db.up.sql`.
- Compared column shapes and access patterns across metadata, conversation, value-count, and proposed CLO MVs.

## Findings

- `RetrieveQAScoreStats` uses up to two moment MVs plus raw fallback: `moment_annotation_mv_by_metadata_d`, `moment_annotation_mv_by_conversation_outcome_d`, and optionally `moment_annotation_d`.
- Routing is per filter group via separate CTEs; metadata and CLO never share one merged moment table.
- Schema has three existing moment MVs with distinct purposes; none is a generic multi-type table.
- Merging metadata or conversation MVs into CLO storage would create a sparse wide table with no query-path benefit.

## Conclusion

Keep CLO storage narrow and type-14-only. Do not add `moment_type` or payload columns for future types. New filterable moment families should get their own `moment_annotation_mv_by_*` table.

## Deliverable

`deliverables/clo-mv-moment-mv-landscape-and-schema-scope.md`
