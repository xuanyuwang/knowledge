# CLO MV Schema Scope: Moment MV Landscape and Extensibility

**Date:** 2026-07-30
**Related tickets:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via), [CONVI-7049](https://linear.app/cresta/issue/CONVI-7049/support-clo-filter-in-performance-insights)
**Question:** Should the CLO target table reserve room for merging existing MVs or future moment types?

## Short answer

**No.** Keep the CLO storage table narrow and specific to `moment_type = 14`.

- Do **not** design it as a merged table for existing metadata or conversation MVs.
- Do **not** add generic `moment_type` or payload columns “for future types.”
- If another moment family later needs PI-style filtering, follow the existing pattern: a separate `moment_annotation_mv_by_<family>` projection MV with typed columns for that family.

The typed `outcome_*` columns are already the right amount of forward compatibility within the conversation-outcome family.

---

## 1. MVs used by `RetrieveQAScoreStats`

Source: `go-servers-convi-7383/insights-server/internal/analyticsimpl/`

### Always-read tables (non-moment)

| Table | Purpose |
|---|---|
| `score_d` or `scorecard_d` | Criterion / scorecard scores |
| `scorecard_d` | Scorecard metadata joins |
| `conversation_d` | Optional duration / end-time filters or grouping |

### Moment-annotation tables (only when `FilterByAttribute.MomentGroups` is non-empty)

| Table | When used (CONVI-7383 path) |
|---|---|
| `moment_annotation_mv_by_metadata_d` | Metadata include/exclude filters (`Moment_CONVERSATION_METADATA`, `moment_type = 19`) |
| `moment_annotation_mv_by_conversation_outcome_d` | CLO include filters when profile flag `use_conversation_outcome_moment_annotation_materialized_view` is on |
| `moment_annotation_d` | CLO include filters when the flag is off (raw table + JSON extraction) |

Routing is **per filter group**, not one shared moment table:

```go
// common_clickhouse.go — three sources, selected independently per moment group
momentAnnotationFilterSourceRaw
momentAnnotationFilterSourceMetadataMV          → moment_annotation_mv_by_metadata_d
momentAnnotationFilterSourceConversationOutcomeMV → moment_annotation_mv_by_conversation_outcome_d
```

In `retrieve_qa_score_stats_clickhouse.go`, each moment filter group gets its own `t1/t2/moment_annotation_filter_N` CTE pair joined against **that group's chosen table**. Metadata and CLO can coexist in one request on different tables.

### Moment types handled in this RPC

| Proto moment type | ClickHouse `moment_type` | MV / table |
|---|---|---|
| `CONVERSATION_METADATA` | 19 | `moment_annotation_mv_by_metadata_d` |
| `CONVERSATION_OUTCOME` (CLO) | 14 | `moment_annotation_mv_by_conversation_outcome_d` or `moment_annotation_d` |

Other moment types (e.g. voice mail) are **not** filtered through moment-annotation MVs in this handler. Voice mail exclusion uses `conversation_d.is_voice_mail`.

### Tables **not** used by `RetrieveQAScoreStats`

| Table | Used elsewhere for |
|---|---|
| `moment_annotation_mv_by_conversation` / `_d` | Conversation-keyed annotation lookups (IDs, not typed filter values) |
| `metadata_moment_value_count_mv_d` | `RetrieveMetadataValues` — distinct metadata value discovery / counts |

---

## 2. Current moment-related MVs in ClickHouse schema

Canonical definitions: `clickhouse-schema/conversations/migrations/20230824160348_init_db.up.sql`

| MV | `moment_type` filter | Engine | Shape | Purpose |
|---|---|---|---|---|
| `moment_annotation_mv_by_conversation` | **None** (all types) | `ReplicatedReplacingMergeTree` | Inline MV | Per-annotation conversation join keys (`conversation_id`, `message_id`, `moment_annotation_id`, times) |
| `moment_annotation_mv_by_metadata` | **19** | `ReplicatedReplacingMergeTree` | Inline MV | Row-level metadata filter projection (`metadata_*_value` columns) |
| `metadata_moment_value_count_mv` | **19** | `AggregatingMergeTree` | Inline MV (INSI-4097 migrated prod to `TO` storage) | Aggregated distinct metadata value counts (`uniqState`) |

All three are **purpose-specific**, not a generic polymorphic moment table.

Naming pattern for row-level filter MVs:

```text
moment_annotation_mv_by_<access_dimension>
  → moment_annotation_mv_by_metadata
  → moment_annotation_mv_by_conversation_outcome   (proposed)
```

---

## 3. Column comparison: metadata MV vs proposed CLO storage

| Column family | Metadata MV (`type 19`) | CLO storage (`type 14`) |
|---|---|---|
| Keys | `conversation_id`, `conversation_start_time`, `moment_template_id` | Same |
| Typed values | `metadata_string_value`, `metadata_number_value`, `metadata_bool_value` | `outcome_string_value`, `outcome_number_value`, `outcome_bool_value`, `outcome_value_type` |
| Version | `update_time` | `update_time` (TBD: confirm vs `create_time`) |
| Type discriminator | Implicit (whole MV is type 19) | `outcome_value_type` + implicit type-14 filter |
| Payload / JSON | Not stored | Not stored (by design — avoids query-time JSON) |

The CLO table already mirrors the metadata MV’s “typed oneof columns” pattern within its own family. That is sufficient extensibility for boolean/string/number outcomes without a shared multi-type table.

---

## 4. Should we merge existing MVs into the CLO target?

### Metadata MV (`moment_annotation_mv_by_metadata`)

**No.**

- Different `moment_type` (19 vs 14).
- Different value column names and query builders.
- `RetrieveQAScoreStats` already joins metadata and CLO through **separate CTEs on separate tables**.
- A merged table would be wide, sparse, and worse on sort/partition pruning. Most rows would have NULL outcome or NULL metadata columns.

### Conversation MV (`moment_annotation_mv_by_conversation`)

**No.**

- Different access pattern: identity/join keys for all moment types, not typed filter values.
- No value columns for PI filtering.
- Not referenced by `RetrieveQAScoreStats` moment-filter path.

### Value-count MV (`metadata_moment_value_count`)

**No.**

- Aggregated `AggregatingMergeTree` with `uniqState`, not row-level `ReplacingMergeTree`.
- Used by `RetrieveMetadataValues`, not QA score stats filtering.
- Merging row-level and aggregate storage would break both access patterns.

---

## 5. Should we reserve room for future moment types?

### Do not add to CLO target table

| Tempting “future-proof” column | Why skip it |
|---|---|
| `moment_type` | Redundant — entire table is ingested with `WHERE moment_type = 14` |
| `moment_annotation_payload` | Defeats the performance goal (typed columns avoid JSON at query time) |
| Nullable metadata + outcome columns in one table | Creates a sparse union table; conflicts with per-family MV architecture |
| Generic `value_string` / `value_number` / `value_bool` shared across types | Would require query-layer reinterpretation by type and blur semantics already split in go-servers |

### Expected pattern for a new filterable moment family

If Performance Insights later filters another moment family the same way:

1. Add `moment_annotation_mv_by_<new_family>` (or `TO` storage + trigger, per INSI-4097).
2. Filter at ingest: `WHERE moment_type = <N>`.
3. Project typed columns for that family’s payload shape.
4. Add a new `momentAnnotationFilterSource*` and table constant in go-servers.
5. Route that moment group to its own table in per-group CTEs.

That matches how metadata (19) and CLO (14) are handled today. One MV per family, not one mega-table.

### What *is* worth keeping flexible in CLO DDL

Within the conversation-outcome family only:

- Confirm `update_time` vs `create_time` for ReplacingMergeTree version semantics.
- Keep the string/number/bool + `outcome_value_type` pattern (already covers oneof outcomes).
- Prefer `ALTER TABLE ... ADD COLUMN` later if a new outcome encoding appears, rather than pre-provisioning unused columns now.

---

## 6. Recommendation for CLO target table design

Adopt the **INSI-4097 `TO target_table` pattern** with a **narrow, CLO-only schema**:

```text
moment_annotation_by_conversation_outcome          -- storage (type 14 only)
moment_annotation_mv_by_conversation_outcome         -- trigger TO storage
moment_annotation_mv_by_conversation_outcome_d       -- distributed over storage
```

Design principles:

1. **Scope:** Only conversation-outcome (`moment_type = 14`) rows.
2. **Columns:** Keys + typed `outcome_*` + `update_time`; no `moment_type`, no payload JSON.
3. **No merge:** Do not combine with metadata MV, conversation MV, or value-count MV.
4. **Future types:** New families get new MVs; do not extend this table.
5. **Query contract:** go-servers already expects dedicated outcome column names on a dedicated distributed table.

---

## 7. References

| Source | Relevance |
|---|---|
| `go-servers-convi-7383/insights-server/internal/analyticsimpl/common_clickhouse.go` | Per-group MV routing, column constants |
| `go-servers-convi-7383/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go` | Per-filter CTE generation |
| `go-servers-convi-7383/shared/clickhouse/testing/schemas/ch_conv_schema.sql` | Draft CLO MV columns |
| `clickhouse-schema/.../20230824160348_init_db.up.sql` | Existing moment MV definitions |
| `convi-7049-clo-filter/deliverables/clo-mv-to-target-table-proposal.md` | Recommended CLO rollout shape |
| `convi-7049-clo-filter/sessions/2026-07-23/codex-query-structure-performance.md` | Why metadata and CLO need independent table routing |
