# CONVI-7143 Message Query Performance Check

## Context

User asked whether `findAgentMessages` always queries messages by conversation IDs, and whether dropping the message time range would significantly degrade performance.

Source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree: main at `/Users/xuanyu.wang/repos/go-servers`

## Code Evidence

Current checked-out code in `/Users/xuanyu.wang/repos/go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go`:

- `Task.Start` selects conversations for a batch, then calls `findAgentMessages(ctx, ..., convs, timeRange.Start, timeRange.End, db)`.
- `findConversations` batches conversations by `chats.ended_at >= startTime AND chats.ended_at < endTime`.
- `findAgentMessages` maps the selected conversations to `convIDs` and always includes `Where("conversation_id IN ?", convIDs)`.
- The same query also currently filters `Where("platform_timestamp >= ? AND platform_timestamp < ?", batchStart, batchEnd)`.

Current test evidence in `/Users/xuanyu.wang/repos/go-servers/cron/sync-users/internal/conversation-agent-assistance/task_test.go` includes `Ignores messages outside batch window`, so the current workspace still encodes the timestamp-bound behavior.

## Schema Evidence

Checked-in app schema for `app.messages`:

- `/Users/xuanyu.wang/repos/go-servers/apiserver/sql-schema/app/app-schema.sql` defines `conversation_id`, `customer_id`, `profile_id`, `speaker_role`, and `platform_timestamp` columns.
- The table has a uniqueness constraint on `(customer_id, profile_id, conversation_id, conversation_message_id)`.
- The explicit message indexes found in the checked-in app schema are:
  - `CREATE INDEX ix_messages_conversation_id ON app.messages USING btree(conversation_id);`
  - `CREATE INDEX ix_messages_conversation_message_id ON app.messages USING btree(conversation_message_id);`
- Search did not find a checked-in `app.messages(platform_timestamp)` index or a composite `app.messages(customer_id, profile_id, conversation_id, platform_timestamp)` index.

## Performance Read

From code/schema only, dropping the message timestamp range should not turn this into a full-table scan, because the query remains scoped by `conversation_id IN ?` and `conversation_id` has a btree index.

The performance impact should mainly be the extra messages read per selected conversation, not all messages for the customer/profile/time range. For normal closed voice conversations this is likely bounded by messages-per-conversation and therefore not a large regression. The current timestamp predicate may not materially help index selection because there is no checked-in timestamp index on `app.messages`.

Residual risk:

- Very large batches can produce a large `IN` list, and removing the timestamp condition means all agent messages for each selected conversation are returned.
- Long-running or pathological conversations could add more rows than the current batch-window slice.
- Exact production impact should be confirmed with `EXPLAIN` against the app DB read path if available.

Performance-preserving correctness option:

- Best correctness fix is to scope by selected conversation IDs and remove the timestamp predicate.
- If we want to keep a broad time guard, derive it from selected conversations, e.g. `[min(convs.StartTime), max(convs.EndTime))` with an optional small buffer, rather than using batch start derived from the previous labeled conversation end watermark.

## Origin Main Confirmation

Fetched `origin/main` on 2026-07-02. Latest remote commit:

- `f1bcb8163835e14867b1159a79290463758c5afa` / `Resolve external RAG credential overrides from user info (#29564)`

On latest `origin/main`, `findAgentMessages` has already been changed:

- `Task.Start` calls `findAgentMessages(ctx, logger, customerID, profileID, convs, db)` with no batch start/end arguments.
- `findAgentMessages` comment says messages are scoped only by `conversation_id` so multi-day conversations are handled correctly, and that the timestamp filter previously dropped messages sent before batch start.
- The query filters by `customer_id`, `profile_id`, `speaker_role = 'agent'`, and `conversation_id IN ?`; it does not filter by `platform_timestamp`.
- Conversation IDs are chunked with `maxConvIDsPerQuery = 10000` to stay below the Postgres extended-protocol parameter limit.
- The corresponding test now has `Finds agent messages sent before conversation end (multi-day conversation)` instead of the old outside-batch-window exclusion test.

Relevant remote history:

- `eed6ced8b2 fix(conv-agent-assistance): scope findAgentMessages by conversation_id only (#29058)`
- `1650f0e6e5 fix(conversation-agent-assistance): chunk conversation IDs to avoid Postgres 65535-parameter limit (#29084)`

Conclusion: latest `origin/main` confirms the proposed correctness fix and includes a performance/operational guard for very large `IN` lists.

## Rerun Implication

If production is stale and the rerun uses a binary containing the `origin/main` behavior above, rerunning `cron-label-conversations` over a range containing the target conversation's `ended_at` should repair this missing label automatically:

- `findConversations` will select the conversation by `ended_at` in the target range.
- `findAgentMessages` will load all agent messages for that selected conversation by `conversation_id`, including messages before the batch start.
- `findOverlappedAgentOnlineEventsFromCH` still queries online events over the target batch window with a ±3h buffer.
- `labelMessages` will match the previously missed agent message against the online event interval.
- `BatchWriteConversationWithLabels` writes via `INSERT INTO conversation_with_labels_d`; it does not require deleting/replacing prior rows.
- `RetrieveConversationStats` uses grouped/max label evidence, so an appended true message label should make the conversation count as powered by Agent Assist.

Caveats:

- Rerunning stale production code would reproduce the miss.
- The target range must include the conversation end time, not only the message time.
- The underlying app DB message and ClickHouse online event rows must still be available.
- Because the write path appends, reruns can add duplicates; this should not inflate the powered-conversation count because the read query groups by conversation/agent and uses max label evidence, but it can add extra raw rows.

## Backfill Execution

Executed the planned one-off Kubernetes Job on 2026-07-02:

- Context: `voice-prod_dev`
- Namespace: `cresta-cron`
- Job: `backfill-labels-hilton-convi-7143-1783017928`
- Image inherited from CronJob: `242659714806.dkr.ecr.us-west-2.amazonaws.com/cresta/go-servers/cron-sync-users:main-20260625_153442z-6fd67cef`
- Env overrides:
  - `ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=true`
  - `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START=2026-06-16T22:00:00Z`
  - `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_END_AT_RANGE_END=2026-06-16T22:31:00Z`
  - `FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=hilton`

Kubernetes result:

- Job completed successfully: `Complete`, `succeeded=1`.
- Pod `backfill-labels-hilton-convi-7143-1783017928-244wd` completed with exit code `0`.
- Logs included `[ConversationHasAgentAssistBackfill] started!` and `[ConversationHasAgentAssistBackfill] ended!`.

ClickHouse verification:

Target conversation now has a true label row:

| Conversation ID | Agent | has_aa | label_rows | last_update_time | message_id | message_time |
|---|---|---:|---:|---|---|---|
| `019ed272-c005-7c27-b060-54d7ff273d5e` | `1beab1a5dad3497d` | true | 1 | `2026-07-02 18:46:04.508516` | `019ed272-c05d-7a30-be31-f7253ad3bf51` | `2026-06-16 22:00:06.733000` |

Aggregate query for the Assistance Insights slice now returns:

| metric | conversations | aht_sec |
|---|---:|---:|
| total | 10 | 294 |
| aa_on | 10 | 294 |

Browser verification:

- Opened the authenticated Hilton Director Assistance Insights page in Chrome.
- Visible filters were `Gaylynn Bryant`, `Jun 16 | Daily`, `All minutes`, profile/usecase `HGV Club`.
- `Convos powered by agent assist` showed `100.0%`, `Total conversations: 10`, and `Conversations powered by Agent Assist: 10`.

Post-backfill check of the other 9 previously powered conversations:

- The 9 pre-existing true label rows are still present.
- Original write times:
  - `019ed201-fdf3-7d68-9e37-5c42e93b3fd0`: `2026-06-25 18:59:56.645511`
  - `019ed224-7e32-783d-94d7-81bc56c7f50a`: `2026-06-25 18:59:56.645511`
  - `019ed227-53b9-753d-88be-b326100565a6`: `2026-06-16 21:00:24.013332`
  - `019ed260-186e-77fb-a115-e03cde37ec51`: `2026-06-16 22:00:20.650814`
  - `019ed261-25a0-7bf4-b617-e59e27588a91`: `2026-06-16 22:00:20.650973`
  - `019ed262-cc0d-7709-8dd8-c68c8cf094d4`: `2026-06-16 22:00:20.651473`
  - `019ed274-7bd6-7b81-b020-abeef968e648`: `2026-06-16 22:31:09.749976`
  - `019ed282-2bcd-76c3-a170-77fcbbba37e5`: `2026-06-16 22:31:09.751498`
  - `019ed291-8965-7da0-96ee-4b9b03c6e941`: `2026-06-25 18:59:56.645511`
- The boundary backfill appended duplicate true rows for `019ed274-7bd6-7b81-b020-abeef968e648` and `019ed282-2bcd-76c3-a170-77fcbbba37e5` at `2026-07-02 18:46:04.508516`, because they are also in the rerun `[22:00, 22:31)` ended-at window.
