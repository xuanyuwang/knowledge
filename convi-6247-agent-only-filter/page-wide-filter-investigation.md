# Page-wide filter: how to add it

**Created:** 2025-02-17  
**Updated:** 2025-02-17 — concrete implementation plan from codebase investigation

## Goal

Add a new page-wide filter option (like **Scorecard Status**) for “Agents only” vs “Include managers” on Performance, Leaderboard, and Agent Assist. The filter value drives the `filterToAgentsOnly` / `listAgentOnly` request parameter for analytics APIs.

---

## How Scorecard Status is implemented (reference pattern)

### Performance page

- **Filter bar:** `OrderedFilterBar` in `Performance.tsx` (line ~149). It receives `filters={filters.components}`, `filtersOrder`, and `filtersSelectionProps` from `usePerformanceFilters()`.
- **Hook:** `usePerformanceFilters` in `packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx`.
- **State:** `PerformanceFiltersState` in `components/insights/types.ts` extends `CommonInsightsFiltersState`, which includes `scorecardStatus?: QAAttributeScorecardStatus[]`. Persisted via `useUnifiedFiltersStore` with key `'performance-page-v2'`; serialization in `filterStateToLocalState` / `localStateToFilterState` (same file) includes `scorecardStatus`.
- **Filter key:** `FilterKey.SCORECARD_STATUS = 'scorecard_status'` in `types/filters/FilterKey.ts`.
- **UI component:** `QAScorecardStatusLevelFilter` from `../../../filters`, backed by `useQAScorecardStatusLevelSelect()` (title: “Scorecard Status”, options: Submitted / Draft / Auto-scored only). Registered in `usePerformanceFilters` in the `components` map (line ~358) and in:
  - `FILTER_SELECTION_MENU_OPTIONS` (utils.ts) — so it appears in the “add filter” menu with label “Scorecard status”.
  - `FILTERS_SELECTION_STATE_ACCESSORS` — `[FilterKey.SCORECARD_STATUS]: 'scorecardStatus'`.
  - `FILTERS_SELECTION_HOOKS` — `[FilterKey.SCORECARD_STATUS]: useQAScorecardStatusLevelSelect`.
- **Featured filters:** Scorecard Status is **not** in `FEATURED_FILTERS` for Performance; it’s only in the selection menu. So it’s an optional filter users can add.
- **Flow to API:** `scorecardStatus` from `filters.state` is included in `PerformanceFiltersState` → passed to `useFilterByAttribute` / `useQAFilterByAttribute` via the filter state used to build `filterByAttribute` for analytics requests.

### Leaderboard page

- **Filter bar:** Same `OrderedFilterBar` in `Leaderboard.tsx` (line ~114). Filters come from `useLeaderboardsFilters(activeTab)`.
- **Hook:** `useLeaderboardsFilters` in `packages/director-app/src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx`.
- **State:** `LeaderboardsFiltersState` includes `scorecardStatus`; persisted with key `'leaderboard-page'`; `LocalStorageLeaderboardsFiltersState` includes `scorecardStatus`.
- **Scorecard Status:** Same `FilterKey.SCORECARD_STATUS`, same `QAScorecardStatusLevelFilter` and `useQAScorecardStatusLevelSelect`. In `getFilterMenuOptions()` (line ~149) the label is from i18n `leaderboards.filters.scorecard-status`. For **Manager** tab, Scorecard Status is **disabled** via `scorecardFilterProps` (line ~224): `[FilterKey.SCORECARD_STATUS]: { isDisabled: true }`.
- **API:** Leaderboard passes `filterToAgentsOnly: true` **hardcoded** in options for Agent/Team tabs (e.g. `AgentLeaderboardPage.tsx`: `QA_STATS_INCLUDE_PEER_USER_STATS_OPTIONS = { includePeerUserStats: true, filterToAgentsOnly: true }`). Manager tab does not use that option (or uses false implicitly).

### Shared pieces

- **Filter bar component:** `OrderedFilterBar` from `components/filters` (path: `../../../components` from insights). It renders a list of filter components and a “filter selection” menu; order and visibility come from `filtersSelection.filtersOrder` and the menu options.
- **Persistence:** `useUnifiedFiltersStore` (from `@/hooks`) with a page-specific `filterStateKey`; state is serialized to local storage via `filterStateToLocalState` / `localStateToFilterState`.
- **Request pipeline:** `useInsightsRequestParams` (util-hooks) accepts `filterOptions?.filterToAgentsOnly` and passes it into `useFilterByAttribute` → backend. QA path: `useQAScoreStatsRequestParams` builds params; it does **not** currently pass `filterToAgentsOnly` (that lives only in `useInsightsRequestParams`). So for RetrieveQAScoreStats we need to add support in the QA request params if the backend adds the field.

---

## Concrete implementation plan

### 1. Add new filter key and state field

- **FilterKey** (`types/filters/FilterKey.ts`): Add e.g. `LIST_AGENT_ONLY = 'list_agent_only'` or `AGENTS_ONLY = 'agents_only'`.
- **Performance state:** In `CommonInsightsFiltersState` or `PerformanceFiltersState` (and `LocalStoragePerformanceInsightsFiltersState`), add `listAgentOnly?: boolean` (default `true` = “Agents only”).
- **Leaderboard state:** In `LeaderboardsFiltersState` and `LocalStorageLeaderboardsFiltersState`, add the same `listAgentOnly?: boolean`.

### 2. Filter UI component

- **Option A (simplest):** Reuse `BooleanFilter` (like “Exclude deactivated users”): label “Agents only”, Yes = agents only, No = include managers. No new hook needed.
- **Option B:** Add a small hook similar to `useQAScorecardStatusLevelSelect` that returns two options (“Agents only” / “Include managers”) and use the existing level-select filter pattern. Gives consistent UX with Scorecard Status.
- Recommendation: **Option A** for speed; can switch to Option B later for copy/UX.

### 3. Performance page — usePerformanceFilters

- **Initial state:** In `getInitialFiltersState` (usePerformanceFilters.tsx), add `listAgentOnly: true` (and in `localStateToFilterState` / `filterStateToLocalState` so it persists).
- **Components:** Add a new entry in the `components` map, e.g. `[FilterKey.LIST_AGENT_ONLY]: (options) => <BooleanFilter ... value={filters.listAgentOnly} onChange={updateFilters('listAgentOnly')} label="Agents only" ... />`.
- **Filter selection:** In `utils.ts` (performance-filters): add to `FILTER_SELECTION_MENU_OPTIONS`, `FILTERS_SELECTION_STATE_ACCESSORS` (`[FilterKey.LIST_AGENT_ONLY]: 'listAgentOnly'`), and `FILTERS_SELECTION_HOOKS`. For a simple boolean, you can use a minimal level-select hook (two options) or keep it only as a featured filter without a level-select (if the UI is just a toggle in the bar).
- **Featured vs menu:** Decide whether to put it in `FEATURED_FILTERS` (always visible) or only in the “add filter” menu like Scorecard Status.
- **modifyFiltersState:** No need to clear `listAgentOnly` for process templates (unlike scorecardStatus); include it in the returned state as-is.

### 4. Leaderboard page — useLeaderboardsFilters

- **Initial state:** In `INITIAL_LEADERBOARDS_FILTERS_STATE` and in `localStateToFilterState` / `filterStateToLocalState`, add `listAgentOnly: true`.
- **Components:** Add `[FilterKey.LIST_AGENT_ONLY]: ...` same as Performance. For **Manager** tab, pass `isDisabled: true` (like Scorecard Status) so the filter is disabled or hidden on Manager tab.
- **Filter menu:** Add to `getFilterMenuOptions()` and `FILTER_STATE_ACCESSORS`; add a hook for the filter selection if using level-select, or use BooleanFilter only.
- **Tab behavior:** On Agent/Team tabs, use `filters.listAgentOnly` when building request options. On Manager tab, always pass `filterToAgentsOnly: false` (or omit) regardless of the stored value.

### 5. Pass filter value into API calls

- **useInsightsRequestParams** already accepts `filterOptions?.filterToAgentsOnly`. Every call site that currently passes hardcoded `filterToAgentsOnly: true` should instead pass `filterToAgentsOnly: filtersState.listAgentOnly ?? true` (or from the relevant filter state).
  - **AgentLeaderboardPage:** Replace `QA_STATS_INCLUDE_PEER_USER_STATS_OPTIONS` and similar with options that use `filtersState.listAgentOnly`.
  - **TeamLeaderboardPage:** Same.
  - **ManagerLeaderboardPage:** Keep `filterToAgentsOnly: false` (or omit); do not use the filter value for manager metrics.
- **Performance:** All components that use `filters.state` and then call `useQAScoreStatsRequestParams` or `useInsightsRequestParams` need to pass `filterToAgentsOnly: filters.state.listAgentOnly ?? true`. That implies:
  - **useQAScoreStatsRequestParams** (and possibly `getQAScoreStatsRequestParams`): Extend the options type to include `filterToAgentsOnly` and pass it through to the request params if the backend RetrieveQAScoreStats request supports it.
  - **useQAFilterByAttribute** / **getQAFilterByAttribute**: Only if the backend expects filter_to_agents_only on the filter object; otherwise it’s a separate request field (see backend proto).
- **Backend:** Ensure analytics request protos (and RetrieveQAScoreStats if applicable) have a field for “list agent only” / “filter to agents only”; then the FE just sets that field from `filtersState.listAgentOnly`.

### 6. Agent Assist

- Agent Assist (if it uses the same insights/analytics APIs and has a filter bar) should be audited for call sites of `useInsightsRequestParams` or QA stats; add the same filter state and pass-through where the result is used for agent-related views. Location in codebase to confirm: search for “Agent Assist” or assistance insights container and its filter state source.

### 7. Backend (for completeness)

- Add request field (e.g. `list_agent_only` or `filter_to_agents_only`) to the analytics APIs that use `ParseUserFilterForAnalytics` (see insights-user-filter project). Pass that field into `ParseUserFilterForAnalytics(..., listAgentOnly)`. Default `false` for backward compatibility.

---

## File checklist

| Area | File(s) | Change |
|------|--------|--------|
| Filter key | `src/types/filters/FilterKey.ts` | Add `LIST_AGENT_ONLY` (or `AGENTS_ONLY`) |
| State types | `src/components/insights/types.ts` | Add `listAgentOnly?: boolean` to CommonInsightsFiltersState, LocalStorage types |
| Performance filters | `src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx` | State init, persistence, components map, modifyFiltersState |
| Performance filter utils | `src/components/insights/hooks/performance-filters/utils.ts` | FILTER_SELECTION_MENU_OPTIONS, STATE_ACCESSORS, HOOKS (and optionally FEATURED_FILTERS) |
| Leaderboard filters | `src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx` | State init, persistence, components, getFilterMenuOptions, FILTER_STATE_ACCESSORS, disable on Manager tab |
| Insights request params | `src/components/insights/hooks/util-hooks/useInsightsRequestParams.ts` | Already has filterToAgentsOnly; call sites pass from state |
| QA request params | `src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts` | Add filterToAgentsOnly to options and to request if proto has it |
| QA filter by attribute | `src/components/insights/hooks/util-hooks/useQAFilterByAttribute.ts` / getQAFilterByAttribute | Only if backend expects it on filter; else pass as top-level request field |
| Leaderboard Agent/Team | `src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx`, `team-leaderboard/TeamLeaderboardPage.tsx` | Pass `filtersState.listAgentOnly` in options instead of hardcoded true |
| Performance call sites | All components using useQAScoreStatsRequestParams / useInsightsRequestParams with filters.state | Pass `filters.state.listAgentOnly` in options |
| i18n | `locales/.../director-app-insights.json` (and others) | Label for “Agents only” / “Include managers” if needed |

---

## Summary

- **Pattern:** Same as Scorecard Status: add a key to `FilterKey`, a field to the page filter state and persistence, a component in the filter bar (BooleanFilter or small level-select), and wire the value into every analytics/QA request that supports it.
- **Difference from Scorecard Status:** This is a single boolean (or two-option select); no need for a complex level-select unless you want the same UX as Scorecard Status. Manager tab: hide or disable the filter and always use “include managers” for manager metrics.
- **Already in place:** `FilterByAttributesOptions.filterToAgentsOnly` and `useInsightsRequestParams(filterOptions?.filterToAgentsOnly)`; Leaderboard currently hardcodes `filterToAgentsOnly: true` for Agent/Team. So the main work is adding the state + UI and replacing the hardcoded value with the filter state.

## Related

- CONVI-6247
- [README.md](README.md) — project overview and scope
- insights-user-filter: API list and `ParseUserFilterForAnalytics` usage
