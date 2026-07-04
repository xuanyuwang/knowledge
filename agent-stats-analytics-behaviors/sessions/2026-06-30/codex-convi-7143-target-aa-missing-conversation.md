# Codex Session: CONVI-7143 Target Missing Agent Assist Conversation

**Date:** 2026-06-30
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/go-servers`
**Ticket:** CONVI-7143

## Objective

Identify the conversation in the Hilton HGV Club 2026-06-16 agent/day slice for agent user ID `1beab1a5dad3497d` that is not marked as powered by Agent Assist, then check whether all conversations in the slice appear correctly marked.

## Current Plan

1. Resolve the exact customer/profile/usecase and query window used by Assistance Insights.
2. Query `conversation_d` for the denominator conversations for the agent/day/usecase/source filters.
3. Query `conversation_with_labels_d` for Agent Assist evidence for those conversations and compare against the aggregate count.
4. Inspect label rows for the missing conversation: message-level vs fallback rows, `has_agent_assistance`, `message_id`, `message_agent_user_id`, `message_time`, and create/update times.
5. Determine whether each conversation is marked correctly, or whether there is stale/missing label data, time-window mismatch, source mismatch, or UI/filter mismatch.

## Notes

- Local config snippets suggest HGV Club maps to `customer_id=hilton`, `profile_id=voice`, `usecase_id=voice-club-care`.

## Investigation Findings

### Director data source confirmation

Confirmed from Director source that the Assistance Insights "Convos powered by agent assist" chart does **not** use `RetrieveAssistanceStats` for the total/powered conversation counts when `enableConversationWithAgentAssistStats` is enabled.

Relevant Director flow:

1. `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/assistance/AssistanceInsights.tsx`
   - Renders `AssistanceInsightsContainer`.
   - Reads feature flag `enableConversationWithAgentAssistStats`.
2. `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/assistance/assistance-insights-container/AssistanceInsightsContainer.tsx`
   - Builds `requestParams` for total conversations.
   - Builds `requestParamsWithAA` with `{ agentAssistUsedOnly: true }`.
   - Calls `useConversationStats(requestParams)` and `useConversationStats(requestParamsWithAA, !enableConversationWithAgentAssistStats)`.
   - Passes both responses to `AssistedConversationRateChart`.
3. `/Users/xuanyu.wang/repos/director/packages/director-app/src/hooks/insights/useFilterByAttributes.ts`
   - Converts `agentAssistUsedOnly: true` into:
     `agentAssistFilters: [{ agentAssistUsed: AGENT_ASSIST_STATE_TYPE_ON }]`.
   - Automatically injects the current usecase into `usecaseNames` when the current usecase is not a usecase superset.
4. `/Users/xuanyu.wang/repos/director/packages/director-api/src/services/cresta-api/insights/insightsApi.ts`
   - `useConversationStats` calls `CrestaAPI.insights.retrieveConversationStats`.
   - `retrieveConversationStats` calls `AnalyticsService.RetrieveConversationStats`.
   - Request payload includes `parent`, `filterByTimeRange`, `metadata`, `filterByAttribute`, `frequency`, `groupByAttributeTypes`, `filterToAgentsOnly`, and optional `conversationTimeRangeField`.
5. `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/analytics/analytics_service.proto`
   - REST mapping is `POST /v1/{parent=customers/*/profiles/*}/conversationStats:retrieve`.

Implication for CONVI-7143:

- The chart's total conversation count is one `RetrieveConversationStats` request without `agentAssistFilters`.
- The powered-by-Agent-Assist count is a second `RetrieveConversationStats` request with `filter_by_attribute.agent_assist_filters = ON`.
- `RetrieveAssistanceStats` is still used elsewhere on Assistance Insights for assistance usage feature metrics, but not for the powered-by-Agent-Assist conversation-count chart.
- The next database reconciliation must reproduce this exact `RetrieveConversationStats` request shape first, including Director's local-day time range conversion, frequency, current usecase injection, selected agent filter, duration buckets, deactivated-user setting, and `filterToAgentsOnly`.

### Live browser check

Used the current Chrome tab on `https://hilton-voice.cresta.com/director/insights/assistance`.

Visible page state:

- Page: Assistance Insights
- Profile/usecase UI: HGV Club
- Filter chips: `Gaylynn Bryant`, `Jun 16 | Daily`, `All minutes`
- Chart title: `Convos powered by agent assist`
- Displayed chart metric: `90.0%`
- Legend/counts:
  - `Total conversations: 10`
  - `Conversations powered by Agent Assist: 9`
- Assistance Used leaderboard row for Gaylynn Bryant also shows `# of convos (per day) = 10.0` and `Convos powered by AA % = 90.0`.

Conclusion from live browser state: the current page no longer reproduces the ticket's `7 total / 6 powered` screenshot. It now matches the earlier ClickHouse reconstruction of `10 / 9` for the broad Jun 16 Daily slice.

Limitation: the browser control API available in this session exposed DOM/page text and console logs, but not completed network request bodies. The exact API path/request shape is confirmed from Director/proto source; the live browser confirmed the active UI state and counts, not a captured payload body.

### Local dev TanStack confirmation

Used Chrome against the local dev page `http://localhost:3100/director/insights/assistance` because the in-app browser did not have the authenticated HGV session and redirected to Microsoft sign-in.

TanStack React Query Devtools on the local page confirmed the chart uses the expected pair of `useRetrieveConversationStats` queries for the current Jun 16 HGV Club/Gaylynn Bryant slice:

- Total conversations query:
  - Query key prefix: `["useRetrieveConversationStats", {...}]`
  - `dateRange`: `["2026-06-16T04:00:00.000Z", "2026-06-17T03:59:59.999Z"]`
  - `filterByAttribute.users`: `[{"name":"customers/hilton/users/1beab1a5dad3497d"}]`
  - `filterByAttribute.usecaseNames`: `["customers/hilton/profiles/voice/usecases/voice-club-care"]`
  - `filterByAttribute.agentAssistFilters`: `null`
  - `conversationDurationBuckets`: `[]`
  - `excludeDeactivatedUsers`: `false`
  - `groups`: `[]`
  - `filterToAgentsOnly`: `false`
  - `frequency`: `DAILY`
  - `groupByAttributeTypes`: `[]`
  - `metadata.startDayOfWeek`: `SUNDAY`
- Powered-by-Agent-Assist query:
  - Same query shape as the total query.
  - Adds `filterByAttribute.agentAssistFilters: [{"agentAssistUsed":"AGENT_ASSIST_STATE_TYPE_ON"}]`.

The local dev UI using those cached queries displayed:

- `Total conversations: 10`
- `Conversations powered by Agent Assist: 9`
- `Convos powered by agent assist: 90.0%`

The TanStack details panel exposed the query keys and selected query metadata, but it did not show scalar values in the collapsed Data Explorer text available to browser automation. The rendered chart/legend supplied the 10/9 values while the selected TanStack query details supplied the exact total and AA query keys.

### In-app browser refresh attempt

After the user opened `https://hilton-voice.cresta.com/director/insights/assistance` in the in-app browser with the correct filters, attempted to claim and refresh that tab to capture fresh runtime/network evidence.

Observations:

- The in-app browser tab metadata initially showed `Assistance Insights | Cresta Director` at `https://hilton-voice.cresta.com/director/insights/assistance`.
- Reload produced new console logs from the prod app bundle and Hilton Voice websocket connection:
  - `cresta websocket status connecting, wss://api-hilton-voice.cresta.com/ws/v1/clientSubscription/streamMessages`
  - `cresta websocket status connected, wss://api-hilton-voice.cresta.com/ws/v1/clientSubscription/streamMessages`
  - app bundle URL: `https://hilton-voice.cresta.com/director/s/release_2026-06-24-1cc5266/assets/index-AnlAjGPd.js`
- The browser automation page context repeatedly failed after reload with `Cannot find context with specified id`, and screenshot/DOM reads reported navigation/context errors.
- A later tab-state read became inconsistent: the tab list reported `https://hilton-voice.cresta.com/director/insights/performance`, while the selected handle evaluated as an internal admin URL. Because of that inconsistency, no additional in-app navigation was performed.

Conclusion: the in-app browser refresh confirmed the prod app bundle/websocket reloaded, but it did not expose reliable completed request URLs or request bodies. The exact endpoint and request shape remain best confirmed by the local dev TanStack query keys plus Director/proto source evidence.

### In-app browser live monitoring

After the user reset the in-app browser page manually, monitored the tab without refreshing or navigating.

The in-app browser settled on the expected prod Assistance Insights page:

- URL: `https://hilton-voice.cresta.com/director/insights/assistance`
- Title: `Assistance Insights | Cresta Director`
- Filter chips:
  - `Gaylynn Bryant`
  - `Jun 16 | Daily`
  - `All minutes`
- Chart: `Convos powered by agent assist`
- Displayed values:
  - `90.0%`
  - `Total conversations: 10`
  - `Conversations powered by Agent Assist: 9`
- Leaderboard row:
  - `Gaylynn Bryant`
  - `Team Shenaya Hayman`
  - `# of convos (per day): 10.0`
  - `Active Days: 1`
  - `Convos powered by AA %: 90.0`

Runtime resource timing from the in-app browser page scope was still unavailable (`resources: null`), so this confirms the active prod UI/filter/count state but still does not capture request payloads.

### ClickHouse access and scope

- Conversation ClickHouse host used: `clickhouse-conversations.voice-prod.internal.cresta.ai:8443`.
- Events ClickHouse host used: `clickhouse-events.voice-prod.internal.cresta.ai:8443`.
- Conversation DB: `hilton_voice`.
- Auth/events DB for online activity: `auth_hilton`.
- Target agent user ID: `1beab1a5dad3497d`.
- Target slice: `customer_id='hilton'`, `profile_id='voice'`, `usecase_id='voice-club-care'`, source `IN (0, 8)`, non-dev users, Jun 16, 2026.

### Denominator reconstruction

Using the `message_d`-based Assistance Stats/Conversation Stats style denominator with `conversation_start_time >= 2026-06-16 00:00:00` and `< 2026-06-17 00:00:00` produced 10 distinct message-backed conversations for the agent/day/usecase. The same slice had 9 distinct conversations with `has_agent_assistance=true` in `conversation_with_labels_d`.

Re-ran the DB reconciliation after confirming the exact TanStack query shape. With the exact local-day UTC bounds from Director (`2026-06-16 04:00:00` to `< 2026-06-17 04:00:00`), agent `1beab1a5dad3497d`, HGV Club usecase, source `IN (0, 7, 8)`, non-dev users, and no voicemail exclusion, the generated-query-shaped ClickHouse reconstruction returned:

| Metric | Count | AHT seconds |
|---|---:|---:|
| total `RetrieveConversationStats` | 10 | 294 |
| AA-on `RetrieveConversationStats` | 9 | 284 |

This exactly matches the current Assistance Insights UI (`10 total / 9 powered`).

Using `conversation_d` directly produced 12 distinct conversations. Two conversations had no `message_d` rows:

- `019ed25e-d5ff-7acb-ba43-c99dbe464df7`: no `message_d` rows, but has a fallback/legacy true label row with `message_id=''`.
- `019ed262-086e-71a3-9e8e-bf13cf7151ba`: no `message_d` rows and no label rows.

The 7/6 count reported in Linear is consistent with the later message-backed subset:

- `019ed260-186e-77fb-a115-e03cde37ec51` - AA true
- `019ed261-25a0-7bf4-b617-e59e27588a91` - AA true
- `019ed262-cc0d-7709-8dd8-c68c8cf094d4` - AA true
- `019ed272-c005-7c27-b060-54d7ff273d5e` - no AA label rows
- `019ed274-7bd6-7b81-b020-abeef968e648` - AA true
- `019ed282-2bcd-76c3-a170-77fcbbba37e5` - AA true
- `019ed291-8965-7da0-96ee-4b9b03c6e941` - AA true

Need follow-up to confirm the exact UI request/time window that narrows the backend denominator to those 7, because the full UTC Jun 16 message-backed slice returns 10/9.

### Missing powered-by-Agent-Assist conversation

The conversation not marked as powered by Agent Assist is:

`019ed272-c005-7c27-b060-54d7ff273d5e`

Observed facts:

- `conversation_start_time`: `2026-06-16 21:59:56.199489`
- `conversation_end_time`: `2026-06-16 22:00:24.000000`
- `conversation_source`: `0`
- `is_voice_mail`: `true`
- `usecase_id`: `voice-club-care`
- `message_d` has an agent message:
  - `message_id`: `019ed272-c05d-7a30-be31-f7253ad3bf51`
  - `speaker_role`: `agent`
  - `agent_user_id`: `1beab1a5dad3497d`
  - `platform_time`: `2026-06-16 22:00:06.733000`
  - `handle_time_secs`: `10`
- `conversation_with_labels_d` has no rows for this conversation, even under other agents.
- `auth_hilton.user_event_d` has an online interval for the same agent from `2026-06-16 21:58:40.936` to `2026-06-16 22:22:49.512`, application type `0`, `isOnline=1`.

Conclusion: based on the labeler rule, the agent message was within an Agent Assist online interval and the conversation appears incorrectly unmarked. It should have a true message-level label row for message `019ed272-c05d-7a30-be31-f7253ad3bf51`.

Exact current denominator row comparison for the 10 UI conversations:

| Conversation ID | Start | End | Voicemail | Agent message rows | AHT seconds | AA label? |
|---|---|---|---:|---:|---:|---:|
| `019ed201-fdf3-7d68-9e37-5c42e93b3fd0` | 19:56:46 | 19:57:32 | false | 6 | 24 | yes |
| `019ed224-7e32-783d-94d7-81bc56c7f50a` | 20:34:27 | 20:35:04 | false | 2 | 20 | yes |
| `019ed227-53b9-753d-88be-b326100565a6` | 20:37:33 | 20:38:36 | false | 5 | 43 | yes |
| `019ed260-186e-77fb-a115-e03cde37ec51` | 21:39:33 | 21:40:03 | true | 3 | 6 | yes |
| `019ed261-25a0-7bf4-b617-e59e27588a91` | 21:40:42 | 21:41:09 | true | 2 | 9 | yes |
| `019ed262-cc0d-7709-8dd8-c68c8cf094d4` | 21:42:30 | 21:44:42 | false | 27 | 126 | yes |
| `019ed272-c005-7c27-b060-54d7ff273d5e` | 21:59:56 | 22:00:24 | true | 4 | 10 | no |
| `019ed274-7bd6-7b81-b020-abeef968e648` | 22:01:49 | 22:02:39 | true | 3 | 30 | yes |
| `019ed282-2bcd-76c3-a170-77fcbbba37e5` | 22:16:46 | 22:17:24 | true | 3 | 16 | yes |
| `019ed291-8965-7da0-96ee-4b9b03c6e941` | 22:33:33 | 22:33:47 | true | 6 | 10 | yes |

All 9 powered conversations have true label evidence. Some have duplicate true rows with the same message ID because of later updates/backfills, but the `RetrieveConversationStats` AA-on query uses `max(has_agent_assistance)` grouped by conversation, so duplicates do not inflate the count.

There is also a true fallback label row for `019ed25e-d5ff-7acb-ba43-c99dbe464df7`, but that conversation has no `message_d` row and therefore does not enter the current chart denominator, which joins `conversation_d`/labels to `message_d`.

### Likely root cause

`/Users/xuanyu.wang/repos/go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go` still has `findAgentMessages` filtering app DB messages by `platform_timestamp >= batchStart AND platform_timestamp < batchEnd`, even after conversations are selected by conversation end time.

The surrounding label updates suggest a batch boundary near `2026-06-16 22:00:20`:

- Conversations ending before that point have label `update_time` around `2026-06-16 22:00:20`.
- Later conversations have label `update_time` around `2026-06-16 22:31:09`.
- The missing conversation ended at `22:00:24`, so it likely fell into the later batch.
- Its agent message occurred at `22:00:06`, before the likely later batch start, so `findAgentMessages` would not load the message even though the conversation was in the batch and the message overlapped an online interval.

This matches the known fragile pattern from earlier Agent Assist labeling work: batching by conversation end time while filtering messages by batch message time can drop valid messages for short calls whose message time precedes the batch start.

Relevant current code references:

- `Task.Start` splits the task into time ranges, fetches conversations, then calls `findAgentMessages` with the same time range.
- `findConversations` selects conversations with `chats.ended_at >= startTime AND chats.ended_at < endTime`.
- `findAgentMessages` selects `app.messages` with `conversation_id IN (...)` but still also filters `platform_timestamp >= batchStart AND platform_timestamp < batchEnd`.

For the missing conversation, the likely boundary is:

- Prior labels written at `2026-06-16 22:00:20` cover conversations ending up to `21:44:42`.
- Later labels written at `2026-06-16 22:31:09` cover conversations ending from `22:02:39` to `22:17:24`.
- Missing conversation ended at `22:00:24`, so it likely belonged to the later conversation-end batch.
- Its valid agent message was at `22:00:06`, before the later batch start, so `findAgentMessages` would not attach it to the conversation even though the online event interval covered it.

#### Cron schedule and batch-boundary evidence

Checked `/Users/xuanyu.wang/repos/flux-deployments` for the deployed cron schedule:

- `releases/voice-prod/cresta-cron/kustomization.yaml` includes `../../../apps/cron-sync-users/releases/03-prod-main` and applies `patch-helmrelease-cron-label-conversations.yaml`.
- `apps/cron-sync-users/releases/03-prod-main/helmrelease-cron-label-conversations.yaml` sets:
  - `suspendCron: false`
  - `schedule: 0,30 * * * *`
  - `ENABLE_CONVERSATION_HAS_AGENT_ASSIST=true`
  - `ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=true`
- The voice-prod patch only sets `enabled: true` and does not override the schedule or Agent Assist labeler envs.
- `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_BATCH_SIZE` is not set in flux for this release, so the code default is `24h`; this matters only when the watermark-to-now range exceeds the default batch size.

The code derives each normal run's start/end as:

- Start: `SELECT max(conversation_end_time) AS start_time FROM conversation_with_labels_d`
- End: `time.Now()`
- Then split if longer than batch size.

Because the start watermark is global for the profile DB, not per agent/usecase, the relevant boundary is the maximum labeled conversation end time across all Hilton Voice labels.

ClickHouse evidence around the two cron runs:

| Run write window | Label rows | Max labeled `conversation_end_time` |
|---|---:|---|
| `2026-06-16 22:00:20.650` to `22:00:20.654` | 1163 | `2026-06-16 22:00:12` |
| `2026-06-16 22:31:09.749` to `22:31:09.753` | 1046 | `2026-06-16 22:30:23` |

The 22:00 run therefore advanced the global watermark to `22:00:12`. The next scheduled run at `22:30` would process conversations with `ended_at >= 22:00:12` and `< now`. The missing conversation ended at `22:00:24`, so it belongs to that later run, but its only agent message timestamp was `22:00:06.733`, before the batch start. That exactly matches the `findAgentMessages` `platform_timestamp >= batchStart` exclusion.

### Correctness check for the other conversations

For the later 7-conversation subset matching the reported 7/6 shape, the other 6 conversations all have true rows in `conversation_with_labels_d`, and their labeled message times fall within online intervals for `1beab1a5dad3497d`. The only incorrect/missing marking found in that subset is `019ed272-c005-7c27-b060-54d7ff273d5e`.

For the full current 10-conversation UI denominator, the same holds: the other 9 conversations have true label rows, and their label message times fall within Agent Assist online intervals for `1beab1a5dad3497d`. The only incorrectly unmarked message-backed conversation found is `019ed272-c005-7c27-b060-54d7ff273d5e`.
