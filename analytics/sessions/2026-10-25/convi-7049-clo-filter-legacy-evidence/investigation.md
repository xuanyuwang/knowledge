# CLO Filter Investigation

**Date:** 2026-06-17

## Ticket Context

CONVI-7049 says: "Currently CLO is not filterable in Performance Insights. we should support it."

CONVI-7043 says HGV needs Custom CLO as a filter in Performance Insights and as metadata in Opera if possible. It also states the staged customer expectation:

- By 2026-06-19: CLO appears as a field when downloading scorecards.
- In July: filter by CLO value in Performance Insights.

The scorecard CSV export work landed in `cresta/go-servers#28769`. That PR says CLO data is stored in `moment_annotations` with `type=CONVERSATION_OUTCOME`, fetched in two batches:

- `moment_templates` for display names.
- `moment_annotations` for per-conversation values.

## Product Meaning

For this ticket, CLO means **Cresta Modeled Outcome**, based on `cresta/go-servers#28769`.

This matters because there are two outcome-like data paths in the codebase:

- Cresta-modeled outcomes: `CONVERSATION_OUTCOME` moments and `conversation_outcome` annotation payloads. This is CLO for CONVI-7049.
- Customer-provided outcomes: `CONVERSATION_METADATA` moments with `DETAILED_TYPE_METADATA_OUTCOME`. Performance Insights already has customer-provided outcome metadata filters.

## DB Storage

Primary Postgres tables:

- `app.moment_templates`
- `app.moment_annotations`

Schema references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/app/app-schema.sql:1828`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/app/app-schema.sql:3650`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/gen/model/moment_annotations.go:56`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/gen/model/moment_templates.go:62`

`moment_annotations` stores:

- `customer_id`
- `profile_id`
- `conversation_id`
- optional `message_id`
- `moment_annotation_id`
- optional `moment_template_id`
- `type`
- optional `taxonomy`
- `payload jsonb`

The DB enum value is:

- `MOMENT_TYPE_CONVERSATION_OUTCOME = 14`
- Defined in `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/protos/moment/moment.proto:41`

The DB annotation payload field is:

- `MomentAnnotationPayload.conversation_outcome = 14`
- Defined in `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/protos/moment/moment_annotation.proto:29`

The value payload is one-of:

- `number_value`
- `string_value`
- `boolean_value`

Defined in `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/sql-schema/protos/moment/moment_annotation.proto:568`.

### CLO Definition Rows

CLO definition/display data is stored in `app.moment_templates`.

The export PR fetches templates with:

```sql
customer_id = ?
AND profile_id = ?
AND taxonomy_state = Moment_ACTIVE
AND "type" = MOMENT_TYPE_CONVERSATION_OUTCOME
```

If usecase IDs are supplied, it adds overlap filtering against `usecase_ids`, plus `consts.AllUsecase`:

```sql
?::text[] && ARRAY[usecase_ids]::text[]
```

Only `display_name` and `taxonomy` are selected. The code stores them as:

```go
cloTaxonomyToDisplayName[taxonomy] = displayName
```

Code reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:1163`

### CLO Annotation Rows

CLO per-conversation values are stored in `app.moment_annotations`.

The export PR fetches annotations by reusing the scorecard export query as an `EXISTS` subquery:

```sql
scorecards.profile = moment_annotations.profile_id
AND scorecards.customer = moment_annotations.customer_id
AND scorecards.conversation_id = moment_annotations.conversation_id
```

Then it adds:

```sql
customer_id = ?
AND profile_id = ?
AND "type" = MOMENT_TYPE_CONVERSATION_OUTCOME
AND taxonomy IS NOT NULL
```

Only `conversation_id`, `taxonomy`, and `payload` are selected. The code then:

1. Skips empty taxonomy.
2. Skips annotation taxonomy not present in `cloTaxonomyToDisplayName`.
3. Parses `payload` as `dbmomentpb.MomentAnnotationPayload`.
4. Reads `payload.GetConversationOutcome()`.
5. Converts number/string/boolean values into strings.
6. Stores `conversationToCLOMap[conversation_id][taxonomy] = value`.

Code reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:1192`

### Payload Examples From PR Tests

Boolean:

```json
{"conversationOutcome":{"booleanValue":true}}
```

String:

```json
{"conversationOutcome":{"stringValue":"resolved"}}
```

Number:

```json
{"conversationOutcome":{"numberValue":42}}
```

Test references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards_test.go:1765`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards_test.go:1815`

## CSV Export Path

The related CSV export code is useful because it shows the exact DB fetch and shaping:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:886`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:1154`

`getLinkedCLOForExport`:

1. Fetches active `moment_templates` where `type = MOMENT_TYPE_CONVERSATION_OUTCOME`.
2. Applies usecase filtering with `usecase_ids` plus `AllUsecase`.
3. Fetches matching `moment_annotations` where `type = MOMENT_TYPE_CONVERSATION_OUTCOME` and `taxonomy IS NOT NULL`.
4. Parses `payload` into `MomentAnnotationPayload`.
5. Reads `payload.GetConversationOutcome()` and formats number/string/boolean values.
6. Builds `conversationToCLOMap[conversation_id][taxonomy] = value`.

The PR extends `linkedScorecardData` with:

```go
cloTaxonomyToDisplayName map[string]string
conversationToCLOMap     map[string]map[string]string
```

Code reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:65`

It adds CLO fetch into `getLinkedExportData` only when:

- the export does not include a process scorecard template (`!hasProcessTemplate`)
- `ExcludeConversationMetadata` is false

Code reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:886`

Header and row behavior:

- It scans all `conversationToCLOMap` values to find taxonomies with display names.
- It dedupes and sorts taxonomies with `fnutils.DedupeAndSort`.
- Headers are emitted as `CLO - {displayName}` after regular `METADATA - ...` columns.
- Each row writes `conversationToCLOMap[scorecard.ConversationID][taxonomy]`, or empty string if missing.

Code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:499`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:678`

Parallel export behavior:

- `mergeLinkedScorecardData` merges both `cloTaxonomyToDisplayName` and nested `conversationToCLOMap`.
- `newLinkedScorecardData` initializes both maps.

Code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:1463`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards.go:1476`

This is export-specific SQL. Performance Insights should not reuse this SQL directly for chart filtering; it should use the existing analytics filter pipeline.

### Export Edge Cases Captured by Tests

The PR added four CLO export tests:

- Boolean CLO export: `CLO - Conversion Rate` column with value `true`.
- Multiple value types: boolean `false`, string `resolved`, and number `42`.
- `ExcludeConversationMetadata = true`: no CLO column is emitted.
- Unknown annotation taxonomy: annotation is silently skipped when no active matching CLO template exists.

It also updated test cleanup to delete `app.moment_annotations`.

References:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards_test.go:1765`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards_test.go:1815`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards_test.go:1926`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/action_export_scorecards_test.go:1976`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/internal/coaching/base_test.go:376`

## Analytics / Elasticsearch Path

CLO annotations are indexed into analytics Elasticsearch conversation docs as nested `moment_annotations`.

The converter sets explicit mapping fields for numeric outcome values:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/apiserver/shared/elasticsearch/converter.go:512`
- Numeric outcome values are copied to `payload_with_explicit_mapping.outcome_number_value`.

The analytics request builder recognizes conversation outcome moment groups:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/insights-server/internal/analyticsimpl/elasticsearch/common.go:134`
- `/Users/xuanyu.wang/repos/go-servers-convi-7080/insights-server/internal/analyticsimpl/elasticsearch/request.go:4710`

For outcome filters, the backend expects `OutcomeValueAttributes`:

- `/Users/xuanyu.wang/repos/go-servers-convi-7080/insights-server/internal/analyticsimpl/elasticsearch/request.go:3653`

`convertConversationOutcomeValueToFilter` builds a nested filter over `moment_annotations`:

- Always filters `moment_annotations.moment_template_id` to the outcome moment ID.
- Adds `payload.conversationOutcome.booleanValue` terms for boolean values.
- Adds `payload.conversationOutcome.stringValue` terms for string values.
- Adds `payload.conversationOutcome.numberValue` terms or explicit numeric mapping terms for numeric values.
- Supports numeric bins through `OutcomeValueAttribute.numeric_bin`.

This means backend filter primitives already exist for Cresta-modeled outcome moments.

## Frontend State and Conversion

Shared filter state already contains Cresta outcome filters:

- `conversationOutcomeToOutcomeValues`
- `outcomeNumericBins`

State accessors:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/filters/types.ts:35`

Shared conversion to analytics `MomentGroupFilter`:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/useMomentGroupFilterFromFilterState.ts:36`

`addOutcomeValueToMomentGroupFilter` converts:

- `outcomeNumericBins` to `outcomeValueAttributes` with numeric bins.
- `conversationOutcomeToOutcomeValues` to `outcomeValueAttributes` with exact outcome values.

Shared level-select hooks:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/filters/outcome-conversation-filters/useOutcomeLevelSelect.ts:27`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/filters/outcome-conversation-filters/useExpandingOutcomeNumericBinLevelSelect.ts:30`

Closed Conversations and shared Insights Filter V2 already register these:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/conversations/hooks/useConversationFilters.tsx:349`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/conversations/hooks/useConversationFilters.tsx:381`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/insights-filters/filterV2Utils.ts:148`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/insights-filters/useInsightsFiltersV2.tsx:500`

## Current Performance Insights Gap

Performance filters initialize the Cresta outcome maps:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx:117`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx:187`

But Performance filter registry only wires customer-provided outcome metadata:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/utils.ts:7`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/utils.ts:81`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/utils.ts:88`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/utils.ts:99`

Specifically, `performance-filters/utils.ts` includes:

- `CUSTOMER_OUTCOME_STATE_ACCESSORS`
- `CUSTOMER_OUTCOME`
- `CUSTOMER_OUTCOME_BIN`

It does not include:

- `CRESTA_OUTCOME_STATE_ACCESSORS`
- `CONVERSATION_OUTCOME_MOMENT`
- `OUTCOME_NUMERIC_BIN`

`usePerformanceFilters.tsx` fetches customer-provided outcome metadata moments:

- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx:343`

It does not fetch `CONVERSATION_OUTCOME` moments for the Cresta-modeled outcome filters. Shared Insights Filter V2 already does this via `useOutcomeCategories`.

## Proposed Implementation Path

1. Confirm product scope:
   - CONVI-7049 should expose Cresta-modeled outcomes, i.e. `CONVERSATION_OUTCOME_MOMENT` and `OUTCOME_NUMERIC_BIN`.
   - Keep customer-provided outcome metadata as the existing `CUSTOMER_OUTCOME` and `CUSTOMER_OUTCOME_BIN` filters.

2. Add Performance filter menu support:
   - Add an "Outcome (Cresta-modeled)" section to `useFilterSelectionSections`.
   - Add `CONVERSATION_OUTCOME_MOMENT` and `OUTCOME_NUMERIC_BIN` to `FILTERS_SELECTION_EXPANDING_LEVEL_SELECTS`.
   - Spread `CRESTA_OUTCOME_STATE_ACCESSORS` into `FILTERS_SELECTION_STATE_ACCESSORS`.
   - Register `useOutcomeLevelSelect` and `useExpandingOutcomeNumericBinLevelSelect` in `FILTERS_SELECTION_HOOKS`.

3. Fetch Cresta outcome moments in `usePerformanceFilters.tsx`:
   - Reuse `useOutcomeCategories` or `useOutcomeMoments` patterns.
   - Use active `CONVERSATION_OUTCOME` moments applicable to Director filters.

4. Add `filtersSelectionHooksArgs`:
   - `CONVERSATION_OUTCOME_MOMENT`: `conversationOutcomeMoments`, `loading`, `pageVariant`.
   - `OUTCOME_NUMERIC_BIN`: `conversationOutcomeMoments` filtered to moments with `payload.numericBins`.

5. Add local storage support if filter persistence is expected:
   - `filterStateToLocalState` currently ignores `conversationOutcomeToOutcomeValues` and `outcomeNumericBins`.
   - `localStateToFilterState` resets both maps.
   - There is shared local storage transform logic in `/Users/xuanyu.wang/repos/director/packages/director-app/src/hooks/local-storage-filter/utils.ts:281`, but numeric outcome bins may need explicit persistence support depending on product expectations.

6. Keep process scorecard behavior:
   - `modifyFiltersState` already clears `conversationOutcomeToOutcomeValues` and `outcomeNumericBins` when selected scorecard template type is process.
   - Hidden filters should also include the two new Cresta outcome filter keys for process scorecards.

7. Verify request propagation:
   - Confirm Performance request code calls `useMomentGroupFilterFromFilterState` or equivalent with the modified filter state.
   - Add/adjust tests around generated analytics request containing `outcome_value_attributes`.

## Open Questions

- Should Performance Insights show both "Outcome (Customer provided)" and "Outcome (Cresta-modeled)" sections?
- Should selected CLO filters persist in Performance filter local storage?
- Should numeric CLOs be exposed via bins only, exact numeric values, or both?
- Is this feature behind an existing or new frontend feature flag?
