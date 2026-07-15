# CONVI-7230 Package Paid Filter Session

## Input

Linear issue CONVI-7230 reports that Holiday Inn Transfer no longer sees `Package Paid` in the Performance Insights filter picker. Saved filter sets warn that some filters are not applicable on the page. The same customer-provided outcome remains available in Closed Conversations under `Outcome (Customer provided)` and appears active with `Filters` enabled in field configuration.

## Working Context

- Primary repo: `/Users/xuanyu.wang/repos/director`
- Active worktree: `/Users/xuanyu.wang/repos/director-convi-7230`
- Branch: `convi-7230-holiday-inn-transfer-performance-insights-regression-outcome`

## Initial Notes

- The main Director checkout is on another active branch, so a dedicated worktree was created from `origin/main`.
- Initial literal searches for `Package Paid`, `Outcome (Customer provided)`, and `Performance Insights` in Director did not find direct matches, suggesting the behavior is controlled by generic filter metadata.
- The issue comment points to a recent CLO filter feature-flag change as a likely regression source.

## Findings

- Performance Insights splits metadata fields into regular metadata and customer-provided outcome metadata via `groupConvMetadataByOutcomeAndRepresentation`; customer outcomes are `CONVERSATION_METADATA` moments with `DETAILED_TYPE_METADATA_OUTCOME`.
- `enableCLOFilters` was gating all outcome filter picker sections and saved-filter hook args in `usePerformanceFilters`, including `CUSTOMER_OUTCOME` and `CUSTOMER_OUTCOME_BIN`.
- When `enableCLOFilters` was false for the customer, Performance Insights hid customer-provided outcomes and cleared their filter state, which caused saved filter sets containing `Package Paid` to be reported as not applicable.
- Closed Conversations still surfaced the field because this regression was in the Performance Insights filter hook.

## Change

- Always expose and preserve Performance Insights customer-provided outcome filters.
- Keep only Cresta-modeled conversation outcome filters (`CONVERSATION_OUTCOME_MOMENT`, `OUTCOME_NUMERIC_BIN`) behind `enableCLOFilters`.

## Validation

- `yarn install --immutable` in `/Users/xuanyu.wang/repos/director-convi-7230` completed with existing peer dependency warnings.
- `yarn workspace @cresta/director-app tsc` passed.
- `yarn workspace @cresta/director-app eslint src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx src/components/insights/hooks/performance-filters/utils.ts --max-warnings=0` passed.
- `git diff --check` passed.

## Follow-up After User Reproduction

- User reproduced the missing-filter warning while applying saved filter set `1.2 Closed Convo Weekly NQ Audits` on Performance Insights.
- Browser evidence on the target page showed `Stage Name` active as regular metadata and `Package Paid` active as customer outcome metadata.
- Local page state showed `metadataMomentToSelectedMetadataValue` for `Stage Name` and `customerOutcomeMomentToSelectedMetadataValue` for `Package Paid`.
- Corrected conclusion: the warning is not the primary bug for CONVI-7230; cross-page saved filter sets can legitimately warn when they contain filters unsupported by Performance Insights.
- The ticket fix should stay focused on manual filter picker availability: `Package Paid` must appear under `Outcome (Customer provided)` and be addable in Performance Insights when configured for Director Filters.
- Dropped the uncommitted follow-up warning/state-roundtrip changes and updated the draft PR text to avoid claiming the warning itself is fixed.

## Follow-up Refined Manual Editability Issue

- Superseded: the later frozen-filter hypothesis was a misunderstanding and is not the CONVI-7230 scope.
- Final alignment returns to the original issue: `enableCLOFilters` was applied too broadly in Performance Insights and should not guard customer-provided outcomes.
- Removed the uncommitted metadata-select follow-up change; source branch is clean with only the committed two-file picker/feature-flag fix.
