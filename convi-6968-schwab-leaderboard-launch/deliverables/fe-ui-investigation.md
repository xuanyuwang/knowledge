# FE UI Investigation

## 2026-06-08 Backend Update

Backend support for the Manager QA API migration is now merged:

- `RetrieveQAScoreStats` supports scorecard submitter grouping with `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`.
- `RetrieveQAConversations` supports scorecard submitter filtering through `QAAttribute.scorecard_reviewer_audience`.
- `QAAttribute.users/groups` remain agent filters; `scorecard_reviewer_audience` is the submitter filter.
- Manager FE no longer needs to wait for submitter filtering support. The remaining work is wiring generated client types and switching the Manager aggregate/drawer providers to the QA APIs.
- The latest project decision accepts QA API time-range semantics for the new Manager path, so exact old submit-time parity is not a blocker.

## Scope

This document covers the frontend UI work for adding scorecard-count drill-downs to the Insights Leaderboard page for:

- Agent tab, root `data-testid="agent-leaderboard"`
- Manager tab, root `data-testid="manager-leaderboard"`

The backend API decisions are captured in `deliverables/api-decision-table.md`. This document focuses on where to add the table cells, how to open the drawer, and which existing drawer patterns should be reused.

## Current Table Structure

| Tab | Main component | Table test id | Current table pattern | Scorecard column state |
|-----|----------------|---------------|-----------------------|------------------------|
| Agent | `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboard.tsx` | `agent-leaderboard-table` | TanStack `columnHelper` columns inside a `columns` `useMemo`; rows are assembled from multiple stats hooks in a `rows` `useMemo`. | No visible submitted-scorecard row key or column today. `METRIC.SCORECARD_COMPLETED` exists in selectable Agent metrics, but `useVisibleColumnsForLeaderboards` does not map it to an Agent row field. |
| Manager | `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/manager-leaderboard/ManagerLeaderboard.tsx` | `manager-leaderboard-table` | TanStack `columnHelper` columns inside grouped column `useMemo`s. | Existing `Scorecards completed` column already uses `numOfScorecardsCompleted`. This should become the clickable drill-down cell instead of adding a duplicate column. |

## Agent Tab Column

| Question | Finding |
|----------|---------|
| Where should the value come from? | Add a separate `RetrieveQAScoreStats` query for this column only, grouped by agent and hardcoded to `scorecardStatuses = [MANUALLY_SUBMITTED]`. Do not reuse the existing Performance score query because its default empty status filter includes all matching statuses. |
| Which field provides the count? | `QAScore.totalScorecardCount` from `/Users/xuanyu.wang/repos/director/packages/director-api/src/services/cresta-api/insights/apiTypes.ts`. |
| Required row type change | Add something like `numOfSubmittedScorecards?: number` to `LeaderboardRow` in `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/types.ts`. |
| Required row population | Merge the submitted-only `RetrieveQAScoreStats` result into rows by `groupedBy.user.name` and set `row.numOfSubmittedScorecards = groupResult.totalScorecardCount || 0`. |
| Required visibility change | Update `useVisibleColumnsForLeaderboards` so `METRIC.SCORECARD_COMPLETED` maps to `numOfSubmittedScorecards`. |
| Required column change | Add a new Agent table accessor for `numOfSubmittedScorecards` with title `Number of submitted scorecards`. A dedicated `Scorecards` group is clearer because the cell opens a scorecard drawer, not a score metric. |
| CSV impact | Add `numOfSubmittedScorecards` to `getLeaderboardRowCSVHeader` if the new visible column should export. |

Recommended cell behavior:

- Render formatted number text for `0`, `undefined`, or disabled state.
- Render a button/link-styled cell for positive counts.
- On click, open the scorecard-template breakdown drawer with the selected agent row.
- Keep the row count sourced from a submitted-only `RetrieveQAScoreStats` query to preserve QA filter consistency while matching the column title.

## Manager Tab Column

| Question | Finding |
|----------|---------|
| Where is the value populated? | `ManagerLeaderboard.tsx`, loop over `scorecardStats.data?.resultsGroupedByAttributeTypes`. |
| Which API is used? | `useScorecardStats` in `ManagerLeaderboardPage.tsx`, which calls the existing `RetrieveScorecardStats` path. |
| Which field is used? | `row.numOfScorecardsCompleted = groupResult.averageScorecardCompletedPerUser`. The code notes that because the result is grouped by manager, average per user equals the manager's scorecard count. |
| Where is the column defined? | `coachingColumnGroup` in `ManagerLeaderboard.tsx`; header is `Scorecards completed`, cell is currently `formatNumberToFixed(cell.getValue())`. |
| Recommended UI change | Reuse this existing column and make only the positive-count cell clickable. Do not add a second Manager scorecard column. |

Recommended cell behavior:

- Preserve the current `Scorecards completed` header and number formatting.
- Render the cell as clickable only when the formatted value is positive.
- On click, open the same scorecard-template breakdown drawer in Manager mode.
- Keep the table count sourced from `RetrieveScorecardStats` to match the current Manager metric semantics.

## Drawer Opening Pattern

| Existing page | Pattern | Fit for this feature |
|---------------|---------|----------------------|
| Performance page | Uses `QAICell`, `useQAConversationExamplesDrawer`, and `QAConversationExamplesDrawer`. Opens a conversation/examples drawer after a score popover interaction. | Useful for request/link utilities, but not a good direct UI reuse. It assumes a selected scorecard template and renders conversation examples, while this feature needs all template groups. |
| Coaching Hub | Uses parent-owned drawer state via `useAgentRecentActivitiesDrawerProps`; opens `AgentRecentCoachingActivitiesDrawer` with Mantine `Drawer` and `Accordion`. | Strong match for interaction shape: table row click opens a right-side drawer with header plus collapsible sections. |
| Coaching Report | Uses parent-owned state via `useTeamOverviewDrawerProps`; opens `TeamOverviewDrawer` with `FullDrawer`, `FullDrawer.Body`, and Mantine `Accordion`. | Strong match for modern drawer structure. Prefer this over raw Mantine `Drawer` for new leaderboard work unless local design constraints require Mantine `Drawer`. |
| QA group calibration leaderboard | Uses `FullDrawer`, fetches scorecard details with `useListAllScorecards`, and shows scorecard rows. | Useful reference for scorecard detail fetching and scorecard row rendering patterns. |

Recommended opening architecture:

| Layer | Responsibility |
|-------|----------------|
| `AgentLeaderboardPage.tsx` / `ManagerLeaderboardPage.tsx` | Own selected row and drawer open/close state because the pages already have the active leaderboard filters. |
| `AgentLeaderboard.tsx` / `ManagerLeaderboard.tsx` | Receive `onScorecardCountClick(row)` prop and call it from the clickable scorecard cell. |
| New drawer component | Receive `{ opened, onClose, variant, userResourceName, userDisplayName, filtersState, aggregateCount }`, fetch details only when open, group by template, and render collapsible template sections. Use data-provider hooks so Agent and Manager API choices are hidden from the drawer shell. |

This is more explicit than making the table own the drawer because the drawer needs the active filters from the page-level state.

## Drawer Data Fetching

| Tab | Table aggregate API | Drawer API | FE grouping key | Notes |
|-----|---------------------|------------|-----------------|-------|
| Agent | Separate `RetrieveQAScoreStats` query with `scorecardStatuses = [MANUALLY_SUBMITTED]` | `RetrieveQAConversations` with `scorecardStatuses = [MANUALLY_SUBMITTED]` | `criteriaInfo[].scorecardTemplateId` or template resource derived from scorecard details | Fetch only for the selected agent when the drawer opens. Use the leaderboard filters plus `usersTeamsGroups.userNames = [agent.resourceName]`. Include `filterToAgentsOnly` once backend support is available. Resolve section titles from the page's existing current-template list. |
| Manager | `RetrieveQAScoreStats` with `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` when grouping by submitter | `RetrieveQAConversations` with selected manager in `scorecard_reviewer_audience` | Template ID/name from QA conversation rows | Fetch only for the selected manager when the drawer opens. Put manager/submitter selection in `scorecard_reviewer_audience`, not normal `users/groups`. Hide the API response shape behind a normalized data-provider hook. |

Existing hooks to reuse:

| Hook | File | Use |
|------|------|-----|
| `useRetrieveAllQAConversations` | `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/useRetrieveAllQAConversations.ts` | Agent drawer, fetch all pages for the selected agent. |
| `useListAllScorecards` | `/Users/xuanyu.wang/repos/director/packages/director-app/src/hooks/coaching/useListAllScorecards.ts` | Historical Manager fallback if old submit-time semantics are needed. Also useful if Agent drawer needs scorecard metadata after `RetrieveQAConversations`. |
| `useGetScorecardTemplatesFilteredByPermissions` / `ListCurrentScorecardTemplates` | `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/useScorecardFilters.tsx` and `/Users/xuanyu.wang/repos/director/packages/director-app/src/hooks/coaching/useCurrentScorecardTemplates.ts` | Agent drawer template-title lookup. The leaderboard filter stack already loads current templates; reuse that response instead of introducing a new template-list query for normal cases. |
| `ConversationLink` | `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/conversation-link/ConversationLink.tsx` | Render row links to scorecards/conversations inside each template section. |

Implementation caution:

- `useRetrieveQAConversationsRequestParams` expects `PerformanceFiltersState`, while leaderboard filters are `LeaderboardsFiltersState`. It is close but not identical.
- Prefer adding a leaderboard-specific request builder or a small shared pure utility rather than casting leaderboard filters to performance filters.
- Performance's existing `QAConversationExamplesDrawer` has `skip = !open || !requestParams.filterByAttribute?.scorecardTemplates?.length`; that would incorrectly block the all-template drawer when no template filter is selected.
- Agent submitted-scorecard paths must hardcode `scorecardStatuses = [MANUALLY_SUBMITTED]` even when the visible page filter is empty.
- Manager drawer should use a normalized data-provider return shape so the UI does not depend directly on the `RetrieveQAConversations` response shape.

## Drawer UI Design

Recommended component location:

`/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/`

Recommended files:

| File | Purpose |
|------|---------|
| `ScorecardTemplateBreakdownDrawer.tsx` | Shared drawer UI for Agent and Manager modes. |
| `useScorecardTemplateBreakdownDrawerProps.ts` | Small parent-owned state hook, following Coaching Hub / Coaching Report patterns. |
| `useAgentScorecardTemplateBreakdown.ts` | Agent drawer request and grouping from submitted-only `RetrieveQAConversations`. |
| `useManagerScorecardTemplateBreakdown.ts` | Manager drawer normalized data provider using `RetrieveQAConversations` with selected manager in `scorecard_reviewer_audience`. Keep the normalized contract so a fallback provider remains easy if needed. |
| `ScorecardTemplateBreakdownDrawer.module.css` | Drawer/header/list styling. |

Recommended UI structure:

- `FullDrawer opened={opened} onClose={onClose} position="right" size="lg"`
- `FullDrawer.Header`: user avatar/name, aggregate count, optional date range subtitle, close button.
- `FullDrawer.Body`: loading/error/empty states, then `Accordion multiple defaultValue={allTemplateKeys}`.
- Each `Accordion.Item`: template title and count in the control.
- Each panel: list scorecards belonging to that template. For conversation-backed scorecards, render the shared `ConversationLink` component so the row title links to the closed conversation with the scorecard/template selected.
- Deeplink query params: include `scorecardTemplateResourceName=<template resource name>` and `conversationDefaultTab=scorecard`. Do not include `criterion` for this drawer because the desired behavior is opening the target scorecard without focusing a criterion.
- Shared link behavior: `ConversationLink` should build scorecard-tab query params when `scorecardTemplateResourceName` exists even if `criterionId` is absent. Existing template-plus-criterion links still focus the criterion.
- Timestamp source: follow the Performance QA examples drawer pattern by letting `ConversationLink` render the timestamp. Agent rows pass `conversation.endTime || conversation.startTime`, so closed-conversation end time is preferred and conversation start time is the fallback. Manager rows from `RetrieveQAConversations` should use the same conversation-backed timestamp pattern. Do not use score as the row metadata for these linked scorecard rows.
- Link text source: Agent and Manager QA conversation rows use `conversation.platformInfo.platformConversationId || conversation.name`, so the displayed link text is a short conversation id when available.

Template section header should display:

| Field | Source |
|-------|--------|
| Template title | Manager and Agent: lookup from the already-loaded `ListCurrentScorecardTemplates` response used by `useGetScorecardTemplatesFilteredByPermissions`. Key by parsed template resource ID from `criteriaInfo[].scorecardTemplateId` or a versioned template resource name when available. If no current-template match exists, fall back to displaying the template ID/resource name, or fetch scorecard metadata only if historical/deactivated template titles are required. |
| Count | Number of distinct scorecards in that template group. |

## Concrete Implementation Plan

### Agent Tab

1. Add `numOfSubmittedScorecards?: number` to `LeaderboardRow`.
2. Add a separate `RetrieveQAScoreStats` call in `AgentLeaderboardPage.tsx` for the new column, grouped by agent and hardcoded to `scorecardStatuses = [MANUALLY_SUBMITTED]`.
3. Map `METRIC.SCORECARD_COMPLETED` to `numOfSubmittedScorecards` in `useVisibleColumnsForLeaderboards`.
4. Merge the submitted-only QAScore result into Agent rows by `groupedBy.user.name`.
5. Add a `Scorecards` column group or a standalone accessor column in `AgentLeaderboard.tsx` with header `Number of submitted scorecards`.
6. Add `onScorecardCountClick?: (row: AgentLeaderboardRow) => void` to `AgentLeaderboardSharedProps`.
7. Render the positive count as a clickable cell that calls the callback.
8. Render the shared drawer from `AgentLeaderboardPage.tsx` with selected agent and current `filtersState`.
9. Drawer fetches submitted-only `RetrieveQAConversations` for the selected agent and groups distinct scorecards by template.

### Manager Tab

1. Add `onScorecardCompletedClick?: (row: ManagerLeaderboardRow) => void` to `ManagerLeaderBoardSharedProps`.
2. Change the existing `numOfScorecardsCompleted` cell in `coachingColumnGroup` to a clickable cell for positive counts.
3. Render the shared drawer from `ManagerLeaderboardPage.tsx` with selected manager and current `filtersState`.
4. Replace or supplement the existing Manager aggregate provider with `RetrieveQAScoreStats`, using manager selections in `scorecard_reviewer_audience` and `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` when grouping by submitter.
5. Drawer uses `useManagerScorecardTemplateBreakdown` as a normalized data-provider hook.
6. Provider fetches `RetrieveQAConversations` with the selected manager in `scorecard_reviewer_audience` and submitted scorecard status as needed.
7. Group returned scorecard rows by template ID/name and display template title/count sections.

## Test Plan

| Area | Test |
|------|------|
| Agent submitted query params | New Agent aggregate query includes `scorecardStatuses = [MANUALLY_SUBMITTED]` regardless of empty visible page status filter. |
| Agent row mapping | Given a submitted-only QAScore with `groupedBy.user.name` and `totalScorecardCount`, row has `numOfSubmittedScorecards`. |
| Agent visible column | When `METRIC.SCORECARD_COMPLETED` is visible, Agent table includes `Number of submitted scorecards`. |
| Agent cell interaction | Positive Agent count calls `onScorecardCountClick` with the row; zero/undefined does not open drawer. |
| Manager cell interaction | Existing `Scorecards completed` positive cell calls `onScorecardCompletedClick`; formatting remains unchanged for non-clickable cells. |
| Manager provider abstraction | Drawer consumes normalized template groups and is not coupled to raw `RetrieveQAConversations` response shape. |
| Drawer grouping | Given mixed scorecards/templates, drawer renders one accordion section per template with correct title and count. |
| Drawer fetch skip | Drawer does not fetch when closed or no selected user exists. |

## FE Workload Estimate

| Work item | Estimate | Notes |
|-----------|----------|-------|
| Table row/column wiring | 0.75-1.25 days | Includes leaderboard types, visible columns, Agent table, Manager table, and a separate Agent submitted-only aggregate query. |
| Drawer shell and state plumbing | 1 day | New shared drawer, parent-owned state in both pages, callbacks through table props. |
| Agent drawer data hook and grouping | 1-1.25 days | Request builder needs care because leaderboard filters are not exactly performance filters. Template title resolution should reuse the current-template list already loaded by the page/filter stack. |
| Manager drawer data hook and grouping | 0.75-1.25 days | Use `RetrieveQAConversations` with `scorecard_reviewer_audience` and wrap it in a normalized provider. |
| Tests and polish | 1-1.5 days | Component tests for columns/clicks plus hook/grouping tests. |
| Total | 4.5-6.5 days | Assumes backend support for Agent `filterToAgentsOnly` on `RetrieveQAConversations` is available or gated. |

## Open Questions / Risks

| Topic | Risk |
|-------|------|
| Agent all-template fetch size | Fetching all QA conversations for one agent is on-demand, but high-volume agents may still produce large result sets. Consider page size limits or lazy section loading if needed. |
| Historical/deactivated Agent templates | The normal title path should use the current-template list already loaded through `ListCurrentScorecardTemplates`. If `RetrieveQAConversations` returns scorecards for templates absent from that list, display the template ID/resource name as fallback, or add a targeted metadata lookup if product requires exact historical titles. |
| Manager parity | New Manager QA API path intentionally accepts QA API time-range semantics instead of exact old `RetrieveScorecardStats` submit-time behavior. Validate sample customer data against the product definition, not the old API's exact row set. |
| Manager provider fallback | Keep the drawer UI on a normalized provider contract so a temporary `ListScorecards` fallback remains localized if needed. |
| Filter parity | Manager filters disable template/status/duration/deactivated filters in the current UI, so the drawer should not invent those filters. Agent drawer should mirror current Agent filters as closely as `RetrieveQAConversations` supports. |
