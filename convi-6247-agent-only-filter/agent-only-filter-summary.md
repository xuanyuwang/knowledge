# "Agents Only" Filter — Behaviour & Engineering Reference

**Created:** 2026-03-30
**Project:** CONVI-6247
**Status:** All code merged. Rollout controlled by frontend feature flag `enableAgentOnlyFilter`.

---

## Part 1: Behaviour Reference (for CS / non-engineers)

### What does the "Agents only" filter do?

The "Agents only" filter controls whether analytics metrics include **only agents** or **both agents and managers** in the data.

- **Filter ON (Yes):** Metrics only include agent users — managers are excluded.
- **Filter OFF (No / default):** Metrics include all users — both agents and managers.

### Where does the filter appear?

The filter is available on three Insights pages:

| Page | Location | Notes |
|------|----------|-------|
| **Performance** | Inside the "+Filters" dropdown | Available on all tabs |
| **Leaderboard** | Inside the "+Filters" dropdown | Available on **Agent** and **Team** tabs only. **Disabled on Manager tab** (Manager tab always includes managers). |
| **Agent Assist** | Inside the "+Filters" dropdown | Available on all tabs |

The filter does **not** appear as a standalone chip on the filter bar. Users must open the "+Filters" dropdown to find it.

### Filter bar chip behaviour

The filter follows the same pattern as "Exclude deactivated users":

- When set to **"Yes"** (agents only) → a chip appears on the filter bar showing the active filter
- When set to **"No"** (default, include all users) → no chip is shown on the filter bar

### Default behaviour

| Scenario | What happens |
|----------|-------------|
| Feature flag **disabled** (most customers currently) | Filter is hidden. Metrics include **agents only** (same as legacy behaviour). No user action needed. |
| Feature flag **enabled**, filter not touched | Default is **agents only** (`true`). Same as legacy behaviour. |
| Feature flag **enabled**, user sets filter to "No" | Metrics include **agents and managers**. A chip does NOT appear (this is the default-off state). |
| Feature flag **enabled**, user sets filter to "Yes" | Metrics include **agents only**. A "Yes" chip appears on the filter bar. |

> **Key point:** When the feature flag is off, the system behaves identically to the legacy behaviour — only agent data is shown. Enabling the feature flag gives users the *option* to include managers, but does not change anything until a user explicitly toggles the filter.

### Which metrics are affected?

All analytics APIs on the Performance, Leaderboard, and Agent Assist pages respect this filter. This includes:

- QA Scores (scorecards, per-criterion breakdowns)
- Conversation stats (count, handle time)
- Agent stats (active days, etc.)
- All assistance metrics (hints, suggestions, smart compose, summarization, note-taking, guided workflow, knowledge assist, knowledge base)
- Live assist stats

**Not affected** (always include managers):
- Coaching session stats
- Comment stats
- Scorecard stats (manager-authored)
- Manager stats

### Common scenarios and expected behaviour

| Scenario | Expected? | Explanation |
|----------|-----------|-------------|
| A manager's name does not appear in the Leaderboard Agent tab | Yes | Default is agents-only. Managers are excluded unless the user sets "Agents only" to "No". |
| After enabling the flag, metrics numbers change | Yes | If a user toggles "Agents only" to "No", manager activity will be included, which may change totals and averages. |
| The filter is not visible on a customer's instance | Yes | The feature flag `enableAgentOnlyFilter` must be enabled for that customer. Contact engineering to enable it. |
| Manager tab on Leaderboard does not show the filter | Yes | The filter is intentionally disabled on the Manager tab — it always includes managers. |
| Filter setting persists after page reload | Yes | The filter value is saved in the browser's local storage per user. |
| Different users see different filter states | Yes | Each user's filter selection is stored locally in their browser. |

---

## Part 2: Engineering Details

### Architecture overview

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend (Director)                                        │
│                                                             │
│  Feature flag: enableAgentOnlyFilter                        │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │ Performance  │  │ Leaderboard  │  │  Agent Assist     │  │
│  │ Filters Hook │  │ Filters Hook │  │  Filters Hook     │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬──────────┘  │
│         │                 │                    │             │
│         └────────┬────────┴────────────────────┘             │
│                  ▼                                           │
│  useInsightsRequestParams / useQAScoreStatsRequestParams    │
│         filterToAgentsOnly: boolean                         │
│                  │                                           │
└──────────────────┼───────────────────────────────────────────┘
                   │ gRPC request
                   ▼
┌──────────────────────────────────────────────────────────────┐
│  Backend (go-servers / insights-server)                      │
│                                                              │
│  Handler reads: req.GetFilterToAgentsOnly()                  │
│         │                                                    │
│         ▼                                                    │
│  ParseUserFilterForAnalytics(... listAgentOnly bool)         │
│         │                                                    │
│         ▼                                                    │
│  listAllUsers(AgentOnly: listAgentOnly)                      │
│  → ListUsersForAnalytics RPC to user service                 │
│         │                                                    │
│         ▼                                                    │
│  Base population (ground truth) established                  │
│  All subsequent filters (ACL, groups, selections)            │
│  are intersected with this base population                   │
│         │                                                    │
│         ▼                                                    │
│  Final user IDs → ClickHouse WHERE user_id IN (...)          │
└──────────────────────────────────────────────────────────────┘
```

### Data flow

1. **Frontend** reads filter state `listAgentOnly` (boolean) from the page's filter hook.
2. The value is passed as `filterToAgentsOnly` in the gRPC request to analytics APIs.
3. **Backend handler** reads `req.GetFilterToAgentsOnly()` and passes it to `ParseUserFilterForAnalytics()`.
4. `ParseUserFilterForAnalytics` calls `listAllUsers()` with `AgentOnly: listAgentOnly`:
   - `true` → `ListUsersForAnalytics` returns only users with agent role
   - `false` → `ListUsersForAnalytics` returns all users (agents + managers + others)
5. This set of users becomes the **base population (ground truth)** — an immutable ceiling.
6. All subsequent filtering (ACL permissions, group expansion, user selections, deactivated user exclusion) produces subsets of this base population via intersection.
7. The final user ID list is passed to ClickHouse queries as `WHERE user_id IN (...)`.

### Proto field

- **Field:** `filter_to_agents_only` (bool)
- **Location:** Added to all `Retrieve*StatsRequest` messages in `cresta-proto` PR #7872
- **Behaviour when unset:** Defaults to `false` (protobuf zero value) — includes all users

### Backend handlers (go-servers)

**File path pattern:** `insights-server/internal/analyticsimpl/retrieve_*.go`

**11 handlers read from request** (use `req.GetFilterToAgentsOnly()`):

| Handler | File | Line |
|---------|------|------|
| RetrieveAgentStats | `retrieve_agent_stats.go` | 48 |
| RetrieveConversationStats | `retrieve_conversation_stats.go` | 43 |
| RetrieveSuggestionStats | `retrieve_suggestion_stats.go` | 47 |
| RetrieveSummarizationStats | `retrieve_summarization_stats.go` | 40 |
| RetrieveSmartComposeStats | `retrieve_smart_compose_stats.go` | 57 |
| RetrieveNoteTakingStats | `retrieve_note_taking_stats.go` | 47 |
| RetrieveGuidedWorkflowStats | `retrieve_guided_workflow_stats.go` | 47 |
| RetrieveKnowledgeBaseStats | `retrieve_knowledge_base_stats.go` | 47 |
| RetrieveKnowledgeAssistStats | `retrieve_knowledge_assist_stats.go` | 41 |
| RetrieveHintStats | `retrieve_hint_stats.go` | 51 |
| RetrieveQAScoreStats | `retrieve_qa_score_stats.go` | 75 |

**1 handler already used request value:** `retrieve_live_assist_stats.go`

**5 handlers unchanged** (hardcoded `false` — always include managers):
- `retrieve_coaching_session_stats.go` — manager-only metric
- `retrieve_comment_stats.go` — manager-only metric
- `retrieve_scorecard_stats.go` — manager-only metric
- `retrieve_manager_stats.go` — manager-only metric
- `retrieve_assistance_stats.go` — legacy API, not wired

**Core user filter logic:** `insights-server/internal/analyticsimpl/common_user_filter.go`
- `ParseUserFilterForAnalytics()` — lines 73-293
- `listAllUsers()` — lines 455-495
- `ApplyUserFilterFromResult()` — lines 587-605

### Frontend code (director)

**Feature flag:** `enableAgentOnlyFilter` in `src/types/frontendFeatureFlags.ts` (line 138-141)

**Filter hooks** (each follows same pattern — initial state, localStorage persistence, feature flag guard):

| Page | File | Default |
|------|------|---------|
| Performance | `src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx` | `true` |
| Leaderboard | `src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx` | `true` |
| Agent Assist | `src/components/insights/hooks/useAssistanceFilters.tsx` | `true` |

**Feature flag guard behaviour** (same in all three hooks):
```typescript
if (!enableAgentOnlyFilter) {
  hiddenFilters.push(FilterKey.LIST_AGENT_ONLY);  // hide from UI
  return { ...state, listAgentOnly: true };         // force agents-only
}
```

**API wiring:**
- `useInsightsRequestParams.ts` (line 44): `filterToAgentsOnly: filterOptions?.filterToAgentsOnly ?? true`
- `useQAScoreStatsRequestParams.ts` (line 39): `filterToAgentsOnly: filtersState.listAgentOnly`
- `AssistanceInsightsContainer.tsx` (lines 75, 77): passes `filterToAgentsOnly: listAgentOnly` to request options

**Filter UI component:** `src/components/filters/list-agent-only-level-select/useListAgentOnlyLevelSelect.ts`
- Uses `useSingleBooleanLevelSelect` with label "Agents only" and option "Yes"

**Filter configuration in each page:**
- `FilterKey.LIST_AGENT_ONLY = 'list_agent_only'` in `src/types/filters/FilterKey.ts`
- Menu options, state accessors, and hook mappings registered in each page's filter utils

### Feature flag and release status

| Flag | Type | Status |
|------|------|--------|
| `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` | Backend env var (insights-server) | **Removed** — fully rolled out and stable as of 2026-03-12. No longer needed. |
| `enableAgentOnlyFilter` | Frontend feature flag (config database) | **Per-customer.** Managed through cresta-config service. Not in flux-deployments. Must be enabled per customer to show the filter UI. |

### Merged PRs

| PR | Repo | Description |
|----|------|-------------|
| cresta-proto #7872 | cresta-proto | Added `filter_to_agents_only` field to analytics request protos |
| go-servers #26301 | go-servers | Backend handler wiring — 11 handlers read from request |
| director #16777 | director | FE types + Performance page filter UI |
| director #17314 | director | Leaderboard filter + API wiring for all pages |
| director #17356 | director | Backward-compatible defaults (deployed to prod) |
| director #17394 | director | Agent Assist filter UI |

### Planned change (not yet on origin/main)

A design decision was made on 2026-03-23 to change the default from `true` to `false`, matching the "Exclude deactivated users" pattern (opt-in rather than default-on). This change has **not yet landed on origin/main** in the director repo. When implemented:
- Default will be `false` (include agents + managers)
- Users explicitly toggle to `true` to filter to agents only
- When feature flag is disabled, behaviour would force `true` (agents-only, same as legacy)
