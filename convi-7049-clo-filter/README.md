# CONVI-7049 - CLO Filter in Performance Insights

**Created:** 2026-06-17
**Status:** Initial investigation complete
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
