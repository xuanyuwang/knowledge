# CONVI-6247 Implementation Plan

**Created:** 2025-02-17

Structured implementation plan for the agent-only / “Include managers” page-wide filter. Detail and file paths are in [page-wide-filter-investigation.md](page-wide-filter-investigation.md).

---

## Success criteria

- Users can choose “Agents only” vs “Include managers” via a page-wide filter on Performance and Leaderboard (and Agent Assist if in scope).
- Filter value is persisted (e.g. in local storage with other page filters).
- All analytics API calls that support the new field receive it from filter state (no hardcoded `true` for agent views).
- On Leaderboard Manager tab, the filter is disabled/hidden and manager metrics always include managers.

---

## Phase 1: Backend — request field and wiring

**Goal:** Analytics APIs accept an optional request field and pass it to `ParseUserFilterForAnalytics`.

**Canonical API list (Leaderboard):** [insights-user-filter/apis-by-leaderboard-tab.md](../insights-user-filter/apis-by-leaderboard-tab.md). **Performance and Agent Assist** APIs are listed in [analytics-apis-performance-and-assistance.md](analytics-apis-performance-and-assistance.md) in this project. All have `filter_to_agents_only` in proto.

| Category | API | Request message | Proto has field |
|----------|-----|-----------------|-----------------|
| 1 – Agent only | RetrieveAgentStats | RetrieveAgentStatsRequest | ✅ |
| 2 – Agent + Team | RetrieveConversationStats | RetrieveConversationStatsRequest | ✅ |
| 2 | RetrieveAssistanceStats (legacy) | RetrieveAssistanceStatsRequest | ✅ |
| 2 | RetrieveSuggestionStats | RetrieveSuggestionStatsRequest | ✅ |
| 2 | RetrieveSummarizationStats | RetrieveSummarizationStatsRequest | ✅ |
| 2 | RetrieveSmartComposeStats | RetrieveSmartComposeStatsRequest | ✅ |
| 2 | RetrieveNoteTakingStats | RetrieveNoteTakingStatsRequest | ✅ |
| 2 | RetrieveGuidedWorkflowStats | RetrieveGuidedWorkflowStatsRequest | ✅ |
| 2 | RetrieveKnowledgeBaseStats | RetrieveKnowledgeBaseStatsRequest | ✅ |
| 2 | RetrieveHintStats | RetrieveHintStatsRequest | ✅ |
| 2 | RetrieveKnowledgeAssistStats | RetrieveKnowledgeAssistStatsRequest | ✅ |
| 3 – Manager only | RetrieveCoachingSessionStats | RetrieveCoachingSessionStatsRequest | ✅ |
| 3 | RetrieveCommentStats | RetrieveCommentStatsRequest | ✅ |
| 3 | RetrieveScorecardStats | RetrieveScorecardStatsRequest | ✅ |
| 4 – Multi-tab | RetrieveLiveAssistStats | RetrieveLiveAssistStatsRequest | ✅ |
| 5 – Performance / by-metric | RetrieveQAScoreStats | RetrieveQAScoreStatsRequest | ✅ |

| # | Task | Owner | Notes |
|---|------|--------|--------|
| 1.1 | Add optional field to analytics request protos | BE | ✅ **Done (2025-02-17).** All APIs in apis-by-leaderboard-tab.md (above) have `filter_to_agents_only` in `cresta-proto/cresta/v1/analytics/analytics_service.proto`. PR: [cresta-proto #7872](https://github.com/cresta/cresta-proto/pull/7872). |
| 1.2 | Wire request field into each API handler | BE | Read the new field and pass to `ParseUserFilterForAnalytics(..., listAgentOnly: req.FilterToAgentsOnly)` (or equivalent). Keep default `false` when unset for backward compatibility. |
| 1.3 | Verify RetrieveQAScoreStats request has the field | BE | ✅ RetrieveQAScoreStatsRequest has `filter_to_agents_only = 12`. |

**Deliverable:** Backend accepts and uses the new field; existing clients unchanged when field is omitted.

---

## Phase 2: Frontend — types and filter key

**Goal:** New filter exists in types and filter key enum; no UI yet.

| # | Task | File(s) | Notes |
|---|------|---------|--------|
| 2.1 | Add `FilterKey` for the new filter | `director-app/src/types/filters/FilterKey.ts` | e.g. `LIST_AGENT_ONLY = 'list_agent_only'` or `AGENTS_ONLY = 'agents_only'`. |
| 2.2 | Add `listAgentOnly` to Performance filter state | `director-app/src/components/insights/types.ts` | Add `listAgentOnly?: boolean` to `CommonInsightsFiltersState` (or `PerformanceFiltersState`) and to `LocalStoragePerformanceInsightsFiltersState`. Default semantic: `true` = agents only. |
| 2.3 | Add `listAgentOnly` to Leaderboard filter state | Same types file + `useLeaderboardsFilters.tsx` | Add to `LeaderboardsFiltersState` and `LocalStorageLeaderboardsFiltersState` (and to `DefaultLeaderboardsFiltersState` / `INITIAL_LEADERBOARDS_FILTERS_STATE` in the hook). |

**Deliverable:** Types and FilterKey in place; no runtime behavior change yet.

---

## Phase 3: Frontend — Performance page filter

**Goal:** “Agents only” filter appears and persists on Performance; value is in state.

| # | Task | File(s) | Notes |
|---|------|---------|--------|
| 3.1 | Initial state and persistence | `director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx` | In `getInitialFiltersState` add `listAgentOnly: true`. In `filterStateToLocalState` and `localStateToFilterState` add `listAgentOnly`. |
| 3.2 | Register filter component | Same file | In the `components` map add `[FilterKey.LIST_AGENT_ONLY]: (options) => <BooleanFilter value={filters.listAgentOnly} onChange={updateFilters('listAgentOnly')} label="Agents only" ... />` (see “Exclude deactivated users” for props). Include in dependency array. |
| 3.3 | Filter selection menu and accessors | `director-app/src/components/insights/hooks/performance-filters/utils.ts` | Add to `FILTER_SELECTION_MENU_OPTIONS` (label e.g. “Agents only”), `FILTERS_SELECTION_STATE_ACCESSORS` (`[FilterKey.LIST_AGENT_ONLY]: 'listAgentOnly'`), and `FILTERS_SELECTION_HOOKS` (use a minimal two-option hook or a simple boolean hook if available). Optionally add to `FEATURED_FILTERS` if it should be visible by default. |

**Deliverable:** Performance page shows the new filter; toggling updates and persists `listAgentOnly`.

---

## Phase 4: Frontend — Leaderboard page filter

**Goal:** Same filter on Leaderboard; disabled on Manager tab.

| # | Task | File(s) | Notes |
|---|------|---------|--------|
| 4.1 | Initial state and persistence | `director-app/src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx` | In `INITIAL_LEADERBOARDS_FILTERS_STATE`, `localStateToFilterState`, and `filterStateToLocalState` add `listAgentOnly: true`. |
| 4.2 | Register filter component; disable on Manager | Same file | Add `[FilterKey.LIST_AGENT_ONLY]: ...` to `components` (same BooleanFilter pattern as Performance). When `activeTab === LeaderboardTabs.MANAGERS`, pass `isDisabled: true` for this filter (e.g. via a props map or conditional, similar to `scorecardFilterProps` for SCORECARD_STATUS). |
| 4.3 | Filter menu and accessors | Same file | Add to `getFilterMenuOptions()` and to `FILTER_STATE_ACCESSORS`; add hook to `FILTERS_SELECTION_HOOKS` if using level-select in the menu. |

**Deliverable:** Leaderboard shows the filter on Agent/Team tabs; on Manager tab it is disabled. Value persists.

---

## Phase 5: Frontend — pass filter value into API calls

**Goal:** All relevant analytics requests use `filterToAgentsOnly` / `list_agent_only` from filter state instead of hardcoded values.

| # | Task | File(s) | Notes |
|---|------|---------|--------|
| 5.1 | Leaderboard — Agent tab | `director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx` | Replace hardcoded `filterToAgentsOnly: true` in `QA_STATS_INCLUDE_PEER_USER_STATS_OPTIONS`, `INSIGHTS_REQUEST_PARAMS_OPTIONS_FOR_CONVOS_WITH_AA`, and any other options with `filterToAgentsOnly: filtersState.listAgentOnly ?? true`. |
| 5.2 | Leaderboard — Team tab | `director-app/src/features/insights/leaderboard/team-leaderboard/TeamLeaderboardPage.tsx` | Same: pass `filterToAgentsOnly: filtersState.listAgentOnly ?? true` in options for insights/QA requests. |
| 5.3 | Leaderboard — Manager tab | Ensure Manager tab never sends agent-only | Manager metrics should use `filterToAgentsOnly: false` or omit; do not use `filtersState.listAgentOnly` for Manager tab. |
| 5.4 | QA request params support | `director-app/src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts` (and `getQAScoreStatsRequestParams` if used) | Add `filterToAgentsOnly?: boolean` to options type; pass through to the request object if the backend RetrieveQAScoreStats proto has the field. |
| 5.5 | Performance — pass state into QA and insights params | All Performance components that call `useQAScoreStatsRequestParams` or `useInsightsRequestParams` | Pass `filterToAgentsOnly: filters.state.listAgentOnly ?? true` in the options. Affected areas: PerformanceProgression, LeaderboardPerCriterion, ConversationCountChart, StatsGraphContainer, ScoreLineChart*, QAICell, LeaderboardByScorecardTemplateItem, useStatsData, useTopAgentsQAScoreStats, useGetQAScoreChapterStats, etc. Prefer a single place (e.g. options derived from `filters.state` in the parent) to avoid touching every child. |

**Deliverable:** Every analytics/QA call that should respect the filter uses the value from filter state; Manager path does not.

---

## Phase 6: Agent Assist (optional / TBD)

| # | Task | Notes |
|---|------|--------|
| 6.1 | Audit Agent Assist for analytics/insights calls | Search for `useInsightsRequestParams`, `useGetQAStats`, or similar in Agent Assist or assistance-insights code. |
| 6.2 | Add filter state and UI if Agent Assist has its own filter bar | If it uses a shared filter store or a different one, add `listAgentOnly` and a control there too. |
| 6.3 | Pass filter value in Agent Assist API calls | Where results are agent-related, pass `filterToAgentsOnly` from the relevant filter state. |

**Deliverable:** Agent Assist behavior documented; implemented if in scope.

---

## Phase 7: i18n and polish

| # | Task | File(s) | Notes |
|---|------|---------|--------|
| 7.1 | Copy for filter label and options | `director-app/locales/.../director-app.json` or `director-app-insights.json` | e.g. “Agents only”, “Include managers” (if not using BooleanFilter Yes/No). Add for leaderboards if needed (e.g. `leaderboards.filters.agents-only`). |
| 7.2 | Tooltip or help text (optional) | Same | Short explanation of what “Agents only” vs “Include managers” means. |

---

## Testing and acceptance

- [ ] **Performance:** Toggle “Agents only” on/off; verify persisted after reload; verify QA and other analytics requests send the correct value (e.g. via network tab or logging).
- [ ] **Leaderboard Agent tab:** Same; compare metrics with filter on vs off (expect different counts when managers exist).
- [ ] **Leaderboard Team tab:** Same as Agent.
- [ ] **Leaderboard Manager tab:** Filter is disabled or hidden; manager metrics unchanged by the new filter.
- [ ] **Backend:** Old clients (no field) still work; new clients with field get correct filtering.

---

## File reference (quick lookup)

| Area | File (director-app) |
|------|---------------------|
| Filter key | `src/types/filters/FilterKey.ts` |
| State types | `src/components/insights/types.ts` |
| Performance filters | `src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx` |
| Performance filter utils | `src/components/insights/hooks/performance-filters/utils.ts` |
| Leaderboard filters | `src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx` |
| Insights request params | `src/components/insights/hooks/util-hooks/useInsightsRequestParams.ts` |
| QA request params | `src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts` |
| Agent leaderboard page | `src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx` |
| Team leaderboard page | `src/features/insights/leaderboard/team-leaderboard/TeamLeaderboardPage.tsx` |
| Backend / proto | cresta-proto + analytics service (go-servers); see insights-user-filter for API list |

---

## Dependencies and order

1. **Phase 1 (BE)** can be done first so FE has a field to send; or FE can add the field and send a constant until BE is ready.
2. **Phases 2 → 3 → 4** are sequential (types/key, then Performance UI, then Leaderboard UI).
3. **Phase 5** can start once 2 is done (pass-through can use state from 3/4 as they land).
4. **Phase 6** is independent; **Phase 7** can be done with 3/4.

Suggested order: **1 (BE)** → **2 (types)** → **3 (Performance filter)** → **5.4–5.5 (Performance API pass-through)** → **4 (Leaderboard filter)** → **5.1–5.3 (Leaderboard API pass-through)** → **7 (i18n)** → **6 (Agent Assist)**.
