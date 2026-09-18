# CONVI-7708: Assistance Insights value-or-missing metadata parity

**Status:** backlog
**Primary domain:** analytics
**Primary subdomain:** assistance-insights
**Official ticket:** [CONVI-7708](https://linear.app/cresta/issue/CONVI-7708/support-value-or-missing-metadata-filters-across-assistance-insights)
**Last updated:** 2026-09-17

## Objective and Impact

- **Objective:** Support `selected latest metadata value OR metadata annotation absent` across the nine Assistance Insights RPCs that already implement value-only and missing-only metadata filtering.
- **Impact:** Director constructs this combined shape from its shared metadata filter state. The affected APIs currently return `INVALID_ARGUMENT`, which can break cards, leaderboards, and comparison-window requests rather than returning the Elasticsearch-compatible population.
- **Role:** audited and ticketed

## Scope

**In scope**

- `RetrieveAssistanceStats`
- `RetrieveAgentStats`
- `RetrieveHintStats`
- `RetrieveKnowledgeAssistStats`
- `RetrieveSuggestionStats`
- `RetrieveKnowledgeBaseStats`
- `RetrieveNoteTakingStats`
- `RetrieveGuidedWorkflowStats`
- `RetrieveSummarizationStats`

**Non-goals**

- `RetrieveConversationStats`, handled by CONVI-7706 and go-servers PR #32440.
- `RetrieveSmartComposeStats`, which currently ignores ordinary metadata filters and therefore needs complete metadata-filter support rather than overlap alone.
- Shared-parser callers without a confirmed Director metadata-filter contract. They should retain explicit rejection until their contract and consumers are established.

## Current Understanding

Director places the same metadata template in both `moments` and `excluded_moments` and includes the selected values in `metadata_value_attributes`. Elasticsearch represents this as an OR. The nine scoped ClickHouse APIs already support both component predicates independently through `generateMetadataFilters`, but the shared parser rejects their combination because its separate include and exclude lists cannot retain the OR relationship.

This is needed for contract parity: the shared Director filter can reach these APIs, Elasticsearch already supports the shape, and returning an error for the supported UI composition is user-visible breakage. The explicit error remains preferable to silently wrong analytics until every query builder correctly represents the OR.

## Validation Expectations

- Value-only, missing-only, and combined populations are correct.
- Multiple selected values retain latest-value semantics, with `allowMatchingStaleMetadataValues` honored.
- Each overlap group is AND-ed with other metadata groups and unrelated exclusions.
- Current and comparison-window calls behave identically.
- Legacy and split Assistance Insights aggregation paths are contract-equivalent.
- Query-level tests cover the APIs' distinct tables and CTE placements.

## Next Actions

1. Design a shared representation that retains value-or-missing groups without weakening rejection for unsupported callers.
2. Implement and validate the nine scoped RPCs.
3. Track Smart Compose's broader metadata-filter correctness problem separately.

## Timeline

- 2026-09-17 — Completed the shared-caller audit and created Medium-priority CONVI-7708, related to CONVI-7706 and CONVI-7402.
