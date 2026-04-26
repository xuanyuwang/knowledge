# CONVI-6247: Agent-Only Manager Inclusion Filter in Performance

**Created:** 2025-02-17
**Updated:** 2026-03-23 (default value behavior updated to match "Exclude deactivated users")

## Overview

Add a **page-wide filter option** (like **Scorecard Status**) so users can choose whether to include **agent-only** users or **agent + manager** users in metrics. This drives a new **request-level field** on analytics APIs; the FE passes it to the backend based on the filter and on context (e.g. Agent tab vs Manager tab).

- **`listAgentOnly=true`** when the filter or context is “agent-only” (e.g., Agent tab in Leaderboard, or user selected “Agents only” in Performance).
- **`listAgentOnly=false`** when including managers (e.g., Manager tab, or user selected “Include managers” in the page-wide filter).

## Page-wide filter (FE)

Add a new **page-wide filter** control, similar to **Scorecard Status**, on:

- **Performance**
- **Leaderboard** (where it applies to agent-related views)
- **Agent Assist** (where relevant)

The filter state is used when calling analytics APIs: pass the new request field according to the selected option (and tab context where applicable).

### Filter Behavior

**Pattern:** Same as "Exclude deactivated users"
- **Default value:** `false` (include both agents and managers)
- **When toggled to `true`:** Filter appears on filter bar; APIs receive `filter_to_agents_only: true` (agents only)
- **When toggled back to `false` (default):** Filter disappears from filter bar; APIs receive `filter_to_agents_only: false` (include managers)

This makes the filter opt-in: users see agent + manager data by default, and the filter chip only appears when explicitly excluding managers.

## Scope

| Layer | Work |
|-------|------|
| **BE (Analytics APIs)** | Add new request field (e.g. `list_agent_only` or `filter_to_agents_only`) to APIs that use `ParseUserFilterForAnalytics`; pass it through to `ParseUserFilterForAnalytics(..., listAgentOnly)`. |
| **FE** | (1) Add **page-wide filter option** (like Scorecard Status) on Performance, Leaderboard, Agent Assist. (2) When calling these APIs, pass the new field from filter state + context (e.g. `true` for Agent tab / “Agents only”, `false` for Manager tab / “Include managers”). |

Reference: [insights-user-filter](../insights-user-filter/) for API list, `ParseUserFilterForAnalytics` usage, and [apis-by-leaderboard-tab.md](../insights-user-filter/apis-by-leaderboard-tab.md) for Agent vs Manager tab mapping.

## Key References

- **Linear:** [CONVI-6247](https://linear.app/cresta/issue/CONVI-6247/add-this-filter-for-agent-only-manager-inclusion-filter-in-performance)
- **Context:** [insights-user-filter](../insights-user-filter/) — consolidated API refactoring, `listAgentOnly` semantics, which APIs use `ParseUserFilterForAnalytics`
- **API categorization:** [insights-user-filter/apis-by-leaderboard-tab.md](../insights-user-filter/apis-by-leaderboard-tab.md) — Agent vs Manager vs multi-tab APIs

## Implementation plan

Structured phases and task list: **[implementation-plan.md](implementation-plan.md)** (BE → types → Performance filter → Leaderboard filter → API pass-through → i18n → Agent Assist). Reference: [page-wide-filter-investigation.md](page-wide-filter-investigation.md) for file paths and Scorecard Status pattern. Summary:

- **Pattern:** Same as Scorecard Status: add `FilterKey`, state field (`listAgentOnly`), persistence, filter component (e.g. `BooleanFilter`), and wire into `useInsightsRequestParams` / QA request params. Manager tab: disable filter and use “include managers”.
- **Key paths:** Performance → `usePerformanceFilters`; Leaderboard → `useLeaderboardsFilters`; both use `OrderedFilterBar` and `useUnifiedFiltersStore`. `filterToAgentsOnly` already exists in `FilterByAttributesOptions` and `useInsightsRequestParams`; replace hardcoded `true` on Agent/Team leaderboard with `filtersState.listAgentOnly`.

## Status

**All PRs merged.** FE: [#16777](https://github.com/cresta/director/pull/16777) (types + Performance filter), [#17314](https://github.com/cresta/director/pull/17314) (Leaderboard filter + API wiring), [#17356](https://github.com/cresta/director/pull/17356) (backward compat defaults — deployed to prod), [#17394](https://github.com/cresta/director/pull/17394) (Agent Assist filter UI). BE: [#26301](https://github.com/cresta/go-servers/pull/26301) (handler wiring). Ready to test on staging with `enableAgentOnlyFilter` flag. Next: staging E2E test → prod deployment of #17394 → enable flag for customers.

## Log History

| Date       | Summary |
|------------|---------|
| 2025-02-17 | Project created; scope: page-wide filter + request field + FE pass-through. Investigation in page-wide-filter-investigation.md; implementation plan in implementation-plan.md (phases, tasks, file ref). |
| 2026-02-18 | Phase 2 complete: added `FilterKey.LIST_AGENT_ONLY`, `listAgentOnly` to CommonInsightsFiltersState, LocalStorage types, and Leaderboard filter state. PR: [director #16777](https://github.com/cresta/director/pull/16777). |
| 2026-02-19 | Phase 3 complete: Performance page "Agents only" BooleanFilter + useListAgentOnlyLevelSelect hook. Combined Phase 2+3 into single PR #16777. |
| 2026-02-20 | Added i18n: en-US locale keys, converted Performance menu options to `getFilterSelectionMenuOptions()` with `getI18n()`, BooleanFilter labels via `useTranslation`. |
| 2026-03-10 | Rebased PR #16777 on main: resolved conflicts with `scoreResource` (leaderboard) and `tCommon` (performance i18n) additions from main. All review comments addressed. |
| 2026-03-12 | **PR #16777 merged.** Phase 4 (Leaderboard filter) + Phase 5.1–5.3 (API wiring: replaced hardcoded `filterToAgentsOnly: true` with `filtersState.listAgentOnly` on Agent/Team tabs) in PR [#17314](https://github.com/cresta/director/pull/17314). Phase 6 (Agent Assist) deferred — separate filter state type, 11+ call sites. |
| 2026-03-13 | PR #17314 merged. i18n consolidation, review fixes, filter bar chip removal. Phase 1.2: BE handler wiring — 11 Go handlers updated to read `req.GetFilterToAgentsOnly()`. FE default fix (`listAgentOnly: true` when flag off). PRs: FE [#17356](https://github.com/cresta/director/pull/17356), BE [#26301](https://github.com/cresta/go-servers/pull/26301) (blocked by FE). |
| 2026-03-16 | PR #17356 merged. Resolved merge conflicts, reverted stale i18n changes, added Agent Assist backward compat (`?? true` in `useInsightsRequestParams`). Replied to BE PR CodeRabbit review (13 internal callers verified safe). Phase 6 implemented: Agent Assist filter UI in PR [#17394](https://github.com/cresta/director/pull/17394). |
| 2026-03-18 | Investigated missing `filter_to_agents_only` on `RetrieveQAScoreStats` on prod — root cause: PR #16777 sets `listAgentOnly: undefined` when flag off, fix in PR #17356 confirmed on staging. Reviewed PR #17394 against all earlier PR feedback (7 issues checked, all clean). Fixed import ordering lint failure, pushed. |
| 2026-03-19 | PR #17394 merged (review fix: simplified `hiddenFilters` to consts). BE PR #26301 merged. FE #17356 deployed to prod. All PRs merged — ready for staging E2E test with `enableAgentOnlyFilter` flag. |
| 2026-03-23 | Updated filter default value behavior to match "Exclude deactivated users" pattern: default `false`, only appears on filter bar when toggled to `true`, disappears when toggled back to `false`. Previous implementation had default `true` (from Phase 3.1/4.1). |
| 2026-04-13 | Discovered Assistance page filter gap: child components never received `filterToAgentsOnly` (gap from original PR #17394, not a regression). Fixed by threading `filterOptions` through 20 files. See [assistance-filter-gap.md](assistance-filter-gap.md). |
| 2026-04-20 | Full scan found 2 more missed components (`SummaryUsedLeaderboardByType`, `GenAIAnswersLeaderboardByType`). Fix PR [#18132](https://github.com/cresta/director/pull/18132). |
