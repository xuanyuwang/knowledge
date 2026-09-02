# CONVI-7049 - CLO Filter in Performance Insights

> Migrated navigation: [Analytics / Insights User Filter](../analytics/subdomains/insights-user-filter/README.md). This folder remains detailed historical evidence.

**Created:** 2026-06-17
**Status:** CONVI-7383 staging and approved prod Phase B backfills are complete. NCLH's optimized path produced a directional 2.12–2.29× browser improvement. Production GA config [#152031](https://github.com/cresta/config/pull/152031), staging GA [#152040](https://github.com/cresta/config/pull/152040), and repair [#152115](https://github.com/cresta/config/pull/152115) merged. On 2026-08-23, the complete 354-customer eligible production state was replayed with persistence enabled; [workflow run 32655059116](https://github.com/cresta/config/actions/runs/32655059116) succeeded in every production region. Effective ConfigService/Admin read-back is still pending. The 117 Comcast/Schwab production maps remain intentional exclusions.
**Ticket:** [CONVI-7049](https://linear.app/cresta/issue/CONVI-7049/support-clo-filter-in-performance-insights)

## Objective

Support CLO filtering in Performance Insights.

The related parent ticket is [CONVI-7043](https://linear.app/cresta/issue/CONVI-7043/custom-clo-as-performance-insights-filter): HGV needs Custom CLO as a Performance Insights filter, and the CSV export milestone landed first. The CSV export PR is `cresta/go-servers#28769`, linked to CONVI-7048.

## Terminology

For this ticket, CLO means **Cresta Modeled Outcome**, based on `cresta/go-servers#28769`.

In code, CLO is represented as first-class `CONVERSATION_OUTCOME` moment data, not customer-uploaded outcome metadata. Customer-uploaded outcomes can also exist as `CONVERSATION_METADATA` with `DETAILED_TYPE_METADATA_OUTCOME`, but that is a separate path already represented in Performance filters as customer-provided outcome metadata.

## Current Finding

Performance Insights already supports customer-provided outcome metadata filters:

- `CUSTOMER_OUTCOME`
- `CUSTOMER_OUTCOME_BIN`

Performance Insights does not currently wire Cresta-modeled outcome filters:

- `CONVERSATION_OUTCOME_MOMENT`
- `OUTCOME_NUMERIC_BIN`

Shared Insights and Closed Conversations already have reusable frontend filter hooks for those Cresta-modeled outcome filters. The backend Elasticsearch analytics path also already understands `OutcomeValueAttributes` for `CONVERSATION_OUTCOME` moment groups.

## PR 28769 Details

`cresta/go-servers#28769` added CLO columns to Performance Insights scorecard CSV export. It stores/fetches CLO as:

- Definition/display metadata: `app.moment_templates`, filtered by active `MOMENT_TYPE_CONVERSATION_OUTCOME`.
- Per-conversation values: `app.moment_annotations`, filtered by `MOMENT_TYPE_CONVERSATION_OUTCOME`.
- Join key for display/value pairing: `taxonomy`.
- Export value source: `payload.conversationOutcome.{booleanValue|stringValue|numberValue}`.
- CSV header shape: `CLO - {moment_templates.display_name}`.

The export code builds:

- `cloTaxonomyToDisplayName: map[taxonomy]displayName`
- `conversationToCLOMap: map[conversationID]map[taxonomy]value`

It only fetches CLO when exporting conversation-backed scorecards and when `ExcludeConversationMetadata` is false.

## Likely Implementation Shape

Most of the work should be in Director:

- Add Cresta outcome filter menu sections/options to Performance Insights.
- Import/register `CRESTA_OUTCOME_STATE_ACCESSORS`, `useOutcomeLevelSelect`, and `useExpandingOutcomeNumericBinLevelSelect` in `performance-filters/utils.ts`.
- Fetch `CONVERSATION_OUTCOME` moments in `usePerformanceFilters.tsx`, similar to existing shared Insights filter V2 or Closed Conversations.
- Add level-select hook args for `CONVERSATION_OUTCOME_MOMENT` and `OUTCOME_NUMERIC_BIN`.
- Include outcome filter state in Performance local storage if persistence is required.
- Ensure process scorecards keep hiding/clearing conversation-only filters.

Backend may only need tests, unless Performance request construction drops these moment groups before calling analytics APIs.

## Query Performance Follow-up

The backend support merged in `cresta/go-servers#29175`, but its CLO query path is structurally more expensive than the existing metadata-moment path:

- Metadata-only filters continue to use the narrow, monthly-partitioned `moment_annotation_mv_by_metadata_d` view with typed value columns. Their generated SQL did not change.
- CLO filters use the broad `moment_annotation_d` table and parse outcome values from `moment_annotation_payload` with ClickHouse JSON functions.
- The source-table choice is request-wide. If a request contains any CLO group, existing metadata include/exclude filters in that same request are also moved back to `moment_annotation_d`.

This creates the clearest regression risk for mixed CLO + metadata filters. It is a code-structure finding, not yet a measured production latency result. The preferred short-term change is per-moment-group table routing so metadata CTEs stay on the optimized view; the long-term option is a typed, partitioned conversation-outcome materialized view.

Detailed evidence and a runtime validation matrix are in `sessions/2026-07-23/codex-query-structure-performance.md`.

An executable plan is available at `deliverables/clo-filter-performance-test-plan.md`. It defines the A–J benchmark matrix, direct-SQL and UI tracks, selectivity/cardinality controls, query-log collection, safe staging/production gates, component-decomposition tests, recording templates, and decision criteria.

Preliminary production measurement on 2026-07-28 found that the supplied six-month NCLH boolean CLO request completed in 85.83 s versus 66.67 s for the closest same-filter no-CLO query, while increasing reads by approximately 10.1× rows and 7.5× bytes. The CLO annotation component alone took 17.83 s and read 245.4 GB. A paired count showed that the JSON payload increased bytes by 3.02× and shard user CPU by 2.29×. This justifies a typed CLO MV prototype in staging, but not yet a production schema decision or stable p95 claim.

After the NCLH CLO MV flag was enabled, a directional production browser check on 2026-08-11 observed a 180-day daily conversion-only refresh clearing all visible loading regions in 37.5-40.5 s. Compared with the 85.83 s historical raw-table CLO query, this suggests 52.8-56.3% lower latency (2.12-2.29x faster) and remains well below the 120 s frontend timeout. The filters, execution window, cache state, and measurement layer were not identical, so this is rough evidence rather than a controlled before/after or p95 claim. Details are in `sessions/2026-08-11/codex-browser-mv-performance.md`.

Detailed evidence and limitations are in `deliverables/clo-filter-performance-results-2026-07-28.md`.

ClickHouse MV rollout investigation (general MV background, Cresta `POPULATE` pattern vs recommended chunked 180-day backfill, TTL recommendation) is in `deliverables/clo-mv-clickhouse-creation-investigation.md`.

The recommended ClickHouse rollout proposal is in `deliverables/clo-mv-to-target-table-proposal.md`. It follows the production INSI-4097 `TO target_table` pattern (`metadata_moment_value_count`): separate storage table, trigger MV without `POPULATE`, distributed table over storage, and chunked 180-day backfill for selected large customers. Schema-scope research in `deliverables/clo-mv-moment-mv-landscape-and-schema-scope.md` concludes the CLO target table should stay narrow (type 14 only) and not merge with metadata or other moment MVs. An earlier inline-MV draft remains in `deliverables/clo-mv-without-populate-proposal.md`.

## Prod Storage Outcome

The completed full-history prod backfill consumes 64.18 GiB across physical replicas. The original one-year plan would consume approximately 53.11 GiB; older history adds 11.08 GiB (20.9%). These bytes use already-provisioned gp3 PVCs, so the immediate marginal EBS bill is zero. The equivalent allocated value is about $5.15/month for full history and $0.89/month for the older-history increment. Details: `deliverables/clo-prod-storage-cost-2026-08-08.md`.

## Key Files

- Frontend Performance filter setup: `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx`
- Frontend Performance filter registry: `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/utils.ts`
- Shared outcome filter conversion: `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/useMomentGroupFilterFromFilterState.ts`
- Backend ES outcome filtering: `/Users/xuanyu.wang/repos/go-servers-convi-7080/insights-server/internal/analyticsimpl/elasticsearch/request.go`
- DB tables: `app.moment_templates`, `app.moment_annotations`

## Related Artifacts

- `investigation.md`
- `data-flow.md`
- `project.yaml`
- `deliverables/clo-filter-performance-test-plan.md`
- `deliverables/clo-filter-performance-results-2026-07-28.md`
- `deliverables/clo-mv-to-target-table-proposal.md`
- `deliverables/clo-mv-moment-mv-landscape-and-schema-scope.md`
- `deliverables/clo-mv-without-populate-proposal.md`
