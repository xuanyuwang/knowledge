# CONVI-7049 - CLO Filter in Performance Insights

> Migrated navigation: [Analytics / Insights User Filter](../analytics/subdomains/insights-user-filter/README.md). This folder remains detailed historical evidence.

**Created:** 2026-06-17
**Status:** Preliminary production evidence collected; CLO MV Insights flag proto merged ([cresta-proto#9400](https://github.com/cresta/cresta-proto/pull/9400)); waiting on config schema sync; ClickHouse MV create/backfill/TTL investigation written for [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via)
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

Detailed evidence and limitations are in `deliverables/clo-filter-performance-results-2026-07-28.md`.

ClickHouse MV rollout investigation (general MV background, Cresta `POPULATE` pattern vs recommended chunked 180-day backfill, TTL recommendation) is in `deliverables/clo-mv-clickhouse-creation-investigation.md`.

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
