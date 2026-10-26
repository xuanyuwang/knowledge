# Leaderboard Scorecard Breakdown FE Review Guide

This document explains the FE work in the branch around the leaderboard scorecard count drawer. It is organized around the three review areas:

- Shared scorecard template breakdown drawer
- Agent leaderboard tab
- Manager leaderboard tab

The main feature is: scorecard count cells become clickable and open a drawer that groups the relevant scorecards by scorecard template.

## Top-Level Entry Points

`packages/director-app/src/features/insights/leaderboard/Leaderboard.tsx`

- Owns active tab from the route.
- Calls `useLeaderboardsFilters(activeTab)`.
- Builds `qaScoreFilterState` by copying `filters.state` and adding `frequency: 'DAILY'`.
- Passes both filter forms down:
  - `filtersState`: general leaderboard filters.
  - `qaScoreFiltersState`: QA-score-compatible filter state used by QA stats and agent drawer details.

`packages/director-app/src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx`

- Stores filters through `useUnifiedFiltersStore` with key `leaderboard-page`.
- Initial defaults include date range, user/team/group selection, duration buckets, scorecard template fields, scorecard status, autofail score resource, exclude deactivated, and agents-only.
- Applies URL preselected user/team/group filters via `useUserTeamGroupFromUrl`.
- Manager tab disables several filters in the UI:
  - Scorecard template
  - Scorecard template items
  - Scorecard status
  - Duration buckets
  - Exclude deactivated users
  - Agents-only
- Because disabled filters can still exist in persisted state, always verify which fields are actually copied into each tab's API request object.

## Shared Drawer

Files:

- `packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/ScorecardTemplateBreakdownDrawer.tsx`
- `packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useScorecardTemplateBreakdownDrawerState.ts`
- `packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/types.ts`
- `packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/utils.ts`
- `packages/director-app/src/components/insights/conversation-link/ConversationLink.tsx`

### Shared Types

`ScorecardTemplateBreakdownUser`

- `displayName`: visible row name.
- `resourceName`: user resource name used for filtering.
- `username`: optional fallback for agent header display.

`ScorecardTemplateBreakdownGroup`

- `templateId`: group identity used as `Accordion.Item` key/value.
- `templateResourceName`: scorecard template resource name used for deep-link query params.
- `templateTitle`: visible accordion label.
- `scorecards`: list of `ScorecardTemplateBreakdownScorecard`.

`ScorecardTemplateBreakdownScorecard`

- Agent path may populate `conversationInfo` from `QAConversationInfo`.
- Manager path now also populates `conversationInfo` from `QAConversationInfo`.
- The legacy `scorecard` field and `groupScorecardsByScorecardTemplate` helper still exist for scorecard-list-shaped data, but the Manager drawer no longer uses `ListScorecards`.
- Shared fields include scorecard id, score, optional conversation name, optional platform/process id, and optional timestamp.

### Drawer State

`useScorecardTemplateBreakdownDrawerState`

- State is only `selectedUser`.
- `opened` is derived as `!!selectedUser`.
- `open(user)` sets selected user.
- `close()` clears selected user.

There is no separate boolean state, so the selected user is the source of truth for whether the drawer should be open.

### Drawer UI State

`ScorecardTemplateBreakdownDrawer`

- Receives `opened`, `onClose`, `user`, `groups`, `isLoading`, and `isError`.
- Header:
  - Avatar and visible name use `user?.displayName || user?.username || ''`.
  - Subtitle is static translated text.
- Body states:
  - `isLoading`: loading spinner.
  - `isError`: translated error text.
  - `groups.length === 0`: empty component.
  - Otherwise: summary plus accordion.
- Accordion:
  - Mantine uncontrolled `Accordion`.
  - `multiple`.
  - `defaultValue={[]}` means all groups start collapsed.
  - No React state is kept for expanded groups.

### Displayed Drawer Data Sources

| UI element | Source field | Notes |
| --- | --- | --- |
| Header avatar/name | `user.displayName`, fallback `user.username` | Parent tab passes this from the clicked row. |
| Summary scorecard count | `groups.reduce(...group.scorecards.length...)` | Derived after hook transformation. |
| Summary template count | `groups.length` | Derived after grouping. |
| Accordion title | `group.templateTitle` | Built by utility lookup from template metadata, fallback template id. |
| Accordion scorecard count | `group.scorecards.length` | Count within the group. |
| Scorecard/conversation link | `ConversationLink` | Uses either scorecard-focus route or closed-conversation route. |
| Link visible id | `processId`, `scorecard.processId`, or `platformConversationId` depending link type | `ConversationLink` truncates the visible value and prepends `#`. |
| Link timestamp | conversation `endTime/startTime`, scorecard `timestamp`, or scorecard `processInteractionTime` | Formatted in `ConversationLink`. |
| Scorecard submitter | `scorecard.submitter` | Only shown for `type: 'scorecard'` when scorecard data exists. |

### Link Behavior

`ConversationLink`

- `type: 'scorecard'`:
  - Route: `ABS_ROUTES.QA.TASK_HOME.PROCESS_FOCUS_VIEW`.
  - Query includes `scorecardName`.
  - Optional query includes selected criterion and selected scorecard template.
- `type: 'conversation'`:
  - Route: `ABS_ROUTES.CONVERSATIONS.CLOSED`.
  - Path includes `conversationName`.
  - If `scorecardTemplateResourceName` exists, query opens the scorecard tab and selects that template.

The drawer passes a scorecard template resource name when it can resolve one, so conversation links can open directly to the template in the scorecard side panel.

### Shared Transform Utilities

`buildScorecardTemplateTitleLookup(templates)`

- Indexes template title by parsed scorecard template id from `template.name`.
- Value is `template.title || template.name`.

`buildScorecardTemplateResourceNameLookup(templates)`

- Uses the same parsed scorecard template id key.
- Value is the full `template.name`.

`groupQAConversationsByScorecardTemplate(...)`

- Used by Agent and Manager drawers.
- Input: `QAConversationInfo[]`.
- Dedupes by `conversationInfo.scorecardId`.
- Template id first comes from top-level `conversationInfo.scorecardTemplateId`, then falls back to the first `criteriaInfo` entry with `scorecardTemplateId`, then `unknown-template`.
- Score comes from `conversationInfo.totalPercentage`.
- Keeps the full `conversationInfo` so the drawer can read conversation metadata.

`groupScorecardsByScorecardTemplate(...)`

- Legacy helper for `Scorecard[]` data. It is not used by the current Manager drawer after the QA Conversations migration.
- Input: `Scorecard[]` from `ListScorecards`.
- Template identity starts as `scorecard.templateId || scorecard.templateName || unknown-template`.
- Scorecard id is parsed from `scorecard.name`; fallback is raw `scorecard.name`.
- Timestamp is `scorecard.submittedAt || scorecard.updatedAt`.
- Platform conversation id is `scorecard.processId`, or parsed conversation id/name from `scorecard.conversationName`.

## Agent Tab

Files:

- `packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx`
- `packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboard.tsx`
- `packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useAgentScorecardTemplateBreakdown.ts`

### Agent Page Filter Inputs

`AgentLeaderboardPage` receives:

- `filtersState` from `useLeaderboardsFilters`.
- `qaScoreFiltersState` from `Leaderboard.tsx`.
- `preSelectionOfFiltersCompleted`.
- `outcomeMetadataByName`.

General insights requests use `filtersState` reduced into an `InsightsAttribute`:

- `usersTeamsGroups`
- `conversationDurationBuckets`
- `excludeDeactivatedUsers`

QA score requests use `qaScoreFiltersState`, which preserves QA-specific filters:

- submit date range
- frequency
- user/team/group selection
- scorecard template
- scorecard template items
- scorecard status
- include NA scored
- score ranges
- score resource/autofail setting
- exclude deactivated
- agents-only
- date range target

### Agent Page Fetches

`AgentLeaderboardPage`

| Hook | Request builder | Purpose |
| --- | --- | --- |
| `useAgentStats` | `useInsightsRequestParams` | Active days. |
| `useConversationStats` | `useInsightsRequestParams` | Conversation volume and handle time. |
| `useConversationStats` with `agentAssistUsedOnly` | `useInsightsRequestParams` | Conversations powered by Agent Assist ratio. |
| `useHintStats` | `useInsightsRequestParams` | Hint stats. |
| `useAssistanceStats` or `useAssistanceStatsWithSplitAPIs` | `useInsightsRequestParams` | Assistance metrics. |
| `useLiveAssistStats` | `useInsightsRequestParams` | Hands raised / whispers received. |
| `useKnowledgeAssistStats` | `useInsightsRequestParams` | GenAI answer metrics. |
| `useGetHintStatsByHintType` | filter values + day range | Hint engagement by hint type. |
| `useGetQAStats` | `useQAScoreStatsRequestParams` | Performance score, quintile rank, and new submitted scorecard count. |
| `useOutcomeStatsData` | `filtersState` + outcome criteria | Outcome metric columns. |

The new agent scorecard count comes from `useGetQAStats`, not from the drawer request.

### Agent Row Transformation

`AgentLeaderboard`

- Builds rows in a `useMemo`.
- Uses `agentIdToData: Map<string, AgentLeaderboardRow>` keyed by user resource name.
- `getCorrectRowFromLeaderboard(...)` reads `groupedByAttributes.user`.
- `getCorrectRowFromLeaderboardQAGroupBy(...)` reads QA `groupedBy.user`.
- Dev users and entries missing resource name/full name are skipped.

Displayed scorecard-count field:

```ts
row.numOfSubmittedScorecards = groupResult.totalScorecardCount || 0;
```

This is assigned while iterating `score.data?.qaScoreResult.scores`.

Review note: if a user appears only in QA score results and not in conversation stats, the row is created with an empty `name`/`username` because the QA loop does not currently assign `fullName` or `username` onto the row. Existing row population usually comes from conversation stats first.

### Agent Count Column UI

The new column is added when `visibleColumns.has('numOfSubmittedScorecards')`.

`visibleColumns` includes this field when:

- `useVisibleColumnsForLeaderboards` sees `visibleMetrics.includes(METRIC.SCORECARD_COMPLETED)`.

Cell behavior:

- Formats value with `formatLeaderboardNumberToFixed(value, 0, false)`.
- If value is missing or `<= 0`, renders plain formatted text.
- If value is positive, renders an `UnstyledButton`.
- Button opens the drawer with:
  - `displayName: row.name`
  - `resourceName: row.resourceName`
  - `username: row.username`

### Agent Drawer Fetch

`useAgentScorecardTemplateBreakdown`

Input:

- `filtersState`: `qaScoreFiltersState` from page.
- `selectedAgent`: row user selected by the count button.
- `opened`: drawer open state.

It creates `agentFiltersState`:

- Spreads all original QA filters.
- Sets `submitDateRangeInternal` to `submitDateRange`.
- Sets `voicemailMoment` to `undefined`.
- Replaces `usersTeamsGroups` with only the selected agent:
  - `userNames: [selectedAgent.resourceName]`
  - empty `teamNames`
  - empty `groupNames`

Request:

- `useRetrieveQAConversationsRequestParams(agentFiltersState, 1000, { enableAutofailScoring: true })`
- `useRetrieveAllQAConversations(requestParams, !opened || !selectedAgent)`
- `useGetScorecardTemplatesFilteredByPermissions(true, undefined, undefined, !opened)`

Important filter behavior:

- Keeps QA filters such as scorecard template, criteria, status, include NA, score range, score resource, date range target.
- Overrides audience to the clicked agent.
- Fetches all pages recursively once enabled.
- Template metadata is permission-filtered before being used for display names/deep links.

### Agent Drawer Transform and Display

`groupQAConversationsByScorecardTemplate`

- Groups by `QAConversationInfo.criteriaInfo[].scorecardTemplateId`.
- Dedupe by `scorecardId`.
- Each row keeps `conversationInfo`.

The shared drawer then chooses:

- `conversationInfo.conversation` if present:
  - Renders `ConversationLink` as `type: 'conversation'`.
  - Timestamp is conversation `endTime || startTime`.
  - Platform id is `conversation.platformInfo.platformConversationId || scorecard.platformConversationId || conversationName`.
- If no conversation:
  - Renders `ConversationLink` as `type: 'scorecard'`.
  - Scorecard name is built from `scorecardId`.
  - Process id is `scorecard.platformConversationId`.

## Manager Tab

Files:

- `packages/director-app/src/features/insights/leaderboard/manager-leaderboard/ManagerLeaderboardPage.tsx`
- `packages/director-app/src/features/insights/leaderboard/manager-leaderboard/ManagerLeaderboard.tsx`
- `packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useManagerScorecardTemplateBreakdown.ts`
- `packages/director-app/src/features/insights/leaderboard/leaderboard-by-metric/manager-leaderboard-by-metric/hooks/useLeaderboardByMetricDataForManagers.tsx`

### Manager Page Filter Inputs

`ManagerLeaderboardPage` receives both `filtersState` and `qaScoreFiltersState`.

Manager still uses general insights request params for non-scorecard metrics. Scorecard counts now use QA score stats and start from `qaScoreFiltersState`, with Manager-specific overrides.

The Manager tab filter UI disables scorecard filters and duration buckets. In the page logic, only the following fields are used to build manager stats requests:

- Selected team names from `filtersState.usersTeamsGroups.teamNames`.
- Submit date range through `useDayRangeFromFilterState(filtersState.submitDateRange)`.
- `filtersState.conversationDurationBuckets` is copied into `filterValues`, but the UI disables that filter on Manager.

For Manager scorecard QA requests, the page builds `managerScorecardFiltersState`:

- Spreads `qaScoreFiltersState`.
- Forces `submitDateRange` to `filtersState.submitDateRange`.
- Clears `usersTeamsGroups` to `EMPTY_USER_TEAM_GROUP_SELECTION`.
- Copies `conversationDurationBuckets`.
- Forces `scorecardStatus: [QAAttributeScorecardStatus.MANUALLY_SUBMITTED]`.
- Forces `scoreResource: QA_SCORE_RESOURCE_SCORECARD`.
- Forces `listAgentOnly: false`.

The manager user names are not placed into `QAAttribute.users`. They are passed through `additionalFilterByAttribute.scorecardReviewerAudience.users`, so `QAAttribute.users/groups` remain reserved for agent filters.

### Manager Audience Derivation

Manager rows are not based directly on selected users.

Flow:

1. Build `filterByChildTeams` from selected teams:
   - roles: `MANAGER`, `MANAGER_2ND`, `QA_SPECIALIST`, `QA_ADMIN`, `ADMIN`
   - `groupNames: filtersState.usersTeamsGroups.teamNames`
   - `includeIndirectGroupMemberships: true`
2. `useFilteredUsers(filterByChildTeams, { pageSize: 5000 })`
3. Remove dev users.
4. Build `filterByManagerUsers`:
   - `userNames`: unique manager/resource names
   - empty teams/groups
5. All manager stats calls skip when `filterByManagerUsers.userNames.length === 0`.

### Manager Page Fetches

`ManagerLeaderboardPage`

| Hook | Request builder | Purpose |
| --- | --- | --- |
| `useAllGroups` | direct API hook | Needed to map manager user resource name to team resource name. |
| `useFilteredUsers` | selected teams + manager roles | Expands selected teams into manager users. |
| `useManagerStats` | `managerStatsRequestParamsForAgentStats` | Manager page view metrics. |
| `useQAScoreStats` | `useQAScoreStatsRequestParams` grouped by submitter | Scorecards evaluated aggregate count. |
| `useQAScoreStats` | `useQAScoreStatsRequestParams` grouped by submitter and time range | Scorecards evaluated by-metric/daily count. |
| `useCoachingSessionStats` | `managerStatsRequestParams` | Coaching sessions submitted. |
| `useCommentingStats` | `managerStatsRequestParams` | Comments placed. |
| `useLiveAssistStats` | `managerStatsRequestParams` | Whispers given by manager. |

`managerStatsRequestParamsForAgentStats`

- Uses only manager user audience.
- Comment says active days/page stats are only affected by user/team.

`managerStatsRequestParams`

- Uses manager user audience.
- Includes `conversationDurationBuckets` from `filterValues`, though Manager UI disables the filter.

`managerScorecardStatsRequestParams`

- Uses QA attribute group-by `[QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER]`.
- Uses `scoreResource: QA_SCORE_RESOURCE_SCORECARD`.
- Uses `scorecardStatus: MANUALLY_SUBMITTED`.
- Places manager resource names in `additionalFilterByAttribute.scorecardReviewerAudience.users`.
- Keeps `filterByAttribute.users` and `filterByAttribute.groups` empty.

`managerScorecardStatsByTimeRangeRequestParams`

- Same filters as the aggregate request.
- Uses QA attribute group-by `[QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER, QA_ATTRIBUTE_TYPE_TIME_RANGE]`.
- Feeds the Manager by-metric table.

### Manager Row Transformation

`ManagerLeaderboard`

- Builds rows in a `useMemo`.
- Uses `managerNameToRow: Map<string, ManagerLeaderboardRow>` keyed by manager user resource name.
- `getCorrectRowFromLeaderboard(...)` reads `groupResult.groupedByAttributes.user`.
- It looks up `teamResourceName` from `teamsByManager`.
- Entries missing display name or manager resource name are skipped.

Displayed fields and sources:

| Row field | Source |
| --- | --- |
| `manager` | `groupedByAttributes.user.fullName` |
| `resourceName` | `groupedByAttributes.user.name` |
| `teamResourceName` | `teamsByManager.get(managerResourceName) || ''` |
| `managerPageViews` | `ManagerStatsGroupResult.totalDirectorPagesVisitCount` |
| `managerLiveChatPageViews` | `ManagerStatsGroupResult.totalDirectorLiveChatPagesVisitCount` |
| `managerClosedChatPageViews` | `ManagerStatsGroupResult.totalDirectorClosedChatPagesVisitCount` |
| `numOfScorecardsCompleted` | `QAScoreStatsResponse.qaScoreResult.scores[].totalScorecardCount` |
| `numOfCoachingSessionsSubmitted` | `CoachingSessionStatsGroupResult.averageCoachingSessionsSubmittedPerUser` |
| `numOfCommentsPlaced` | `CommentStatsGroupResult.totalCommentsPlacedCount` |
| `whispersGivenByManager` | `LiveAssistStatsGroupResult.totalWhisperByManagerCount` |

The scorecard column label was renamed from "Scorecards completed" to "Scorecards evaluated", but the underlying field remains `numOfScorecardsCompleted`.

The scorecard stats loop reads `groupResult.groupedBy?.user`. If a manager only appears in QA score stats and not in manager page stats, the row is created from that QA user and the team is looked up through `teamsByManager`.

### Manager Count Column UI

The existing manager scorecard column is in the Coaching column group.

Cell behavior:

- Formats value with `formatLeaderboardNumberToFixed(value)`.
- If value is missing or `<= 0`, renders plain formatted text.
- If value is positive, renders an `UnstyledButton`.
- Button opens the drawer with:
  - `displayName: row.manager`
  - `resourceName: row.resourceName`

### Manager Drawer Fetch

`useManagerScorecardTemplateBreakdown`

Input:

- `filtersState` from Manager page.
- `selectedManager` from clicked row.
- `opened` from drawer state.

It builds a QA Conversations request:

- Starts from `filtersState`.
- Sets `submitDateRangeInternal` to `filtersState.submitDateRange`.
- Sets `frequency: 'DAILY'`.
- Clears `voicemailMoment`.
- Clears `usersTeamsGroups` to `EMPTY_USER_TEAM_GROUP_SELECTION`.
- Forces `scorecardStatus: [QAAttributeScorecardStatus.MANUALLY_SUBMITTED]`.
- Forces `scoreResource: QA_SCORE_RESOURCE_SCORECARD`.
- Forces `listAgentOnly: false`.
- Adds `additionalFilterByAttribute.scorecardReviewerAudience.users = [selectedManager.resourceName]`.

Then:

- Calls `useRetrieveQAConversationsRequestParams(managerFiltersState, 1000, { enableAutofailScoring: true, additionalFilterByAttribute })`.
- Calls `useRetrieveAllQAConversations(requestParams, !opened || !selectedManager)` to fetch all pages.
- Calls `useGetScorecardTemplatesFilteredByPermissions(true, undefined, undefined, !opened)` for template titles and resource names.

Important filter behavior:

- This is the QA Conversations path, not the coaching `ListScorecards` path.
- It filters selected manager through `scorecardReviewerAudience`, matching the QA score stats submitter semantics.
- It uses QA scorecard time-range semantics through the shared QA request builder.
- It preserves the Manager date range and forces manual submitted scorecards.

### Manager Drawer Transform and Display

`groupQAConversationsByScorecardTemplate`

- Groups returned QA conversations by scorecard template.
- Dedupes by `conversationInfo.scorecardId`.
- Uses permission-filtered template metadata for template titles and resource names.
- Prefers top-level `QAConversationInfo.scorecardTemplateId`; this is required for QA Conversations responses that omit `criteriaInfo`.
- Each drawer scorecard row receives:
  - `conversationInfo`
  - `scorecardId`
  - `score: conversationInfo.totalPercentage`

The shared drawer then renders Manager rows the same way as Agent QA rows:

- `type: 'conversation'` if `conversationInfo.conversation` exists.
- `type: 'scorecard'` if the QA conversation row has no conversation payload.

## Review Hotspots

Use this as a targeted checklist while reviewing:

1. Count/detail consistency
   - Agent count source is QA score stats; agent detail source is QA conversations.
   - Manager count source is QA score stats; manager detail source is QA conversations.
   - Verify Manager aggregate, by-metric, and drawer all use submitter filtering through `scorecardReviewerAudience`.

2. Agent selected-user override
   - Drawer details deliberately replace the audience filter with only the selected agent.
   - Other QA filters are preserved.

3. Template identity normalization
   - QA Conversations may carry template id at top level or per criterion.
   - Grouping now prefers top-level `scorecardTemplateId`, then criterion-level ids, then `unknown-template`.

4. Permission-filtered template labels
   - Agent and Manager drawer template metadata is filtered through scorecard template permissions.

5. Drawer open-state source of truth
   - `selectedUser` is the open state.
   - Closing clears selected user.
   - Data hooks are disabled while closed.

6. Link behavior
   - Conversation links open closed conversation route and optionally select the scorecard template panel.
   - Scorecard-only links open process focus view with `scorecardName`.

7. Loading behavior
   - Parent table loading is independent from drawer loading.
   - Drawer shows spinner until its own detail query and template query settle.

8. CSV behavior
   - Agent CSV can include `numOfSubmittedScorecards` through `getLeaderboardRowCSVHeader`.
   - Manager CSV still uses `numOfScorecardsCompleted`; only labels changed to "Scorecards evaluated".

9. QA time semantics
   - Manager scorecard counts now use QA API time-range semantics.
   - Do not expect parity with old `RetrieveScorecardStats` submit-time behavior for scorecards submitted in range but whose scorecard/conversation time is outside the selected range.

## Short Data Flow Diagrams

### Agent Count and Drawer

```text
Leaderboard
  useLeaderboardsFilters(activeTab)
  qaScoreFilterState = filters.state + frequency DAILY
    |
    v
AgentLeaderboardPage
  useGetQAStats(qaScoreFilterState, groupBy agent)
    |
    v
AgentLeaderboard
  row.numOfSubmittedScorecards = score.totalScorecardCount
  click positive count -> selectedUser
    |
    v
useAgentScorecardTemplateBreakdown
  qaScoreFilterState + selected agent audience
  retrieve all QA conversations
  fetch visible templates
  group by scorecard template
    |
    v
ScorecardTemplateBreakdownDrawer
```

### Manager Count and Drawer

```text
Leaderboard
  useLeaderboardsFilters(activeTab)
    |
    v
ManagerLeaderboardPage
  selected teams -> useFilteredUsers -> manager userNames
  useQAScoreStats(group by SCORECARD_SUBMITTER)
    filterByAttribute.scorecardReviewerAudience.users = manager userNames
    |
    v
ManagerLeaderboard
  row.numOfScorecardsCompleted = totalScorecardCount
  click positive count -> selectedUser
    |
    v
useManagerScorecardTemplateBreakdown
  retrieve all QA conversations
  scorecardReviewerAudience.users = selected manager
  fetch visible templates
  group QA conversations by scorecard template
    |
    v
ScorecardTemplateBreakdownDrawer
```
