# Agent Leaderboard - Active Days Behavior Guide

Last reviewed: 2026-06-29

## Purpose

This guide explains how Agent Leaderboard decides whether a per-agent, per-day Active Days cell shows `1`, `0`, or `N/A`.

The implementation now depends on two data sets:

- `conversation_d`: the source of closed conversations, agent assignment, use case, source type, and the denominator for valid agent activity.
- `conversation_with_labels_d`: the source of Agent Assist evidence written by `cron-label-conversations`.

## Where Active Days Appears

Active Days appears in two places on Performance Insights:

| Location | Meaning | Values |
|---|---|---|
| Statistics widget | Average number of active days per agent across the selected time range. | Numeric aggregate |
| Agent Leaderboard table | Per-agent, per-day active value. | `1`, `0`, or `N/A` |

The rest of this guide focuses on the Leaderboard table cell. The widget is derived from those per-day values.

## Display Values

| Cell value | Meaning | Included in averages? |
|---|---|---|
| `1` | The agent had at least one qualifying conversation and at least one qualifying Agent Assist evidence row for that day. | Yes |
| `0` | The agent had relevant evidence, but did not satisfy the active-day criteria for the selected configuration. Examples: valid conversations with no Agent Assist evidence, or only ingestion-pipeline evidence when ingestion is excluded. | Yes |
| `N/A` | The query has no row for that agent/day. The system has no evidence that should count or suppress the day. | No |

The distinction between `0` and `N/A` is intentional. `0` lowers averages; `N/A` is excluded from averages.

## Conversation Sources

The relevant source values are:

| Source | Meaning |
|---|---|
| `0` | Normal conversation source. |
| `8` | Ingestion Pipeline conversation source. |
| `7` | Bot conversation source, included only when the request/config includes bot conversations. |

By default, Active Days can include normal and ingestion-pipeline conversations. The `exclude_ingestion_pipeline_from_agent_stats` config removes source `8` from valid activity counting, but source `8` can still leave Agent Assist evidence in `conversation_with_labels_d`.

## Current Agent Assist Labeling Behavior

`cron-label-conversations` no longer decides Agent Assist by checking whether a whole conversation span overlaps a login span.

Current behavior:

1. The cron processes closed conversations whose `ended_at` is in the batch range.
2. It fetches conversations from Normal and Ingestion Pipeline sources and excludes dev users.
3. It fetches `speaker_role = 'agent'` messages for those conversation IDs.
4. Message lookup is scoped by `conversation_id` only. It intentionally does not filter by message timestamp, because the batch window is based on `conversation_end_time`; multi-day conversations can contain valid agent messages before the batch start.
5. Conversation IDs are queried in 10,000-ID chunks to avoid Postgres' 65,535-parameter limit.
6. It fetches online events whose event `startTime` is within the batch window plus a 3-hour buffer.
7. For each agent message, it writes one `conversation_with_labels_d` row if the message timestamp falls within an online event for that agent.

For message-level rows:

- `message_id` is the stable conversation message ID.
- `message_agent_user_id` is the agent ID from the message row.
- `message_time` is the message `platform_timestamp`.
- `has_agent_assistance` is always `true`.

## Conversations With No Agent Messages

The cron also handles conversations that have no agent messages, including conversations with only visitor messages.

For those conversations, it writes one fallback row with `message_id = ''`. The row uses conversation-level matching:

- If the assigned agent was online at `conversation_start_time` or `conversation_end_time`, `has_agent_assistance = true`.
- Otherwise, `has_agent_assistance = false`.

These fallback rows keep the ClickHouse watermark moving and keep no-agent-message conversations visible to downstream analytics.

## Read Path Behavior

### Leaderboard Active Days

`RetrieveAgentStats` builds two ClickHouse CTEs:

- `convs`: counts distinct agents from `conversation_d`, respecting source filters such as `exclude_ingestion_pipeline_from_agent_stats`.
- `convs_with_aa`: counts distinct agents from `conversation_with_labels_d` where `has_agent_assistance = 1`.

The two CTEs are joined with `FULL OUTER JOIN`, and the returned counts are `COALESCE`d to zero. This preserves days where only one side has data.

For new message-level rows, the Active Days time filter and day bucket use `message_time`. For legacy or fallback rows where `message_id = ''`, the query falls back to `conversation_start_time`.

When the customer is configured to use Agent Assist conversation count, the displayed active count uses `active_agent_count` only if `total_agent_count > 0`. This guard prevents source-8-only Agent Assist evidence from showing `1` when ingestion-pipeline conversations are excluded.

### Conversation Stats Agent Assist Filter

Conversation Stats answers a different question: "Did this conversation have Agent Assist?"

For that API, the time range scopes conversations by conversation time, not message time:

- Agent Assist ON: aggregate `max(has_agent_assistance)` across all label rows for the conversation.
- Agent Assist OFF: start from `conversation_d` and use `GLOBAL LEFT ANTI JOIN` to exclude conversations that have any true label row.

This is backward compatible with both old conversation-level rows and new message-level rows.

## Exclude Ingestion Pipeline Behavior

Flag: `exclude_ingestion_pipeline_from_agent_stats`

When the flag is OFF:

- Normal and Ingestion Pipeline conversations both count as valid activity.
- A day can show `1` if the agent has qualifying Agent Assist evidence.

When the flag is ON:

- Only Normal conversations count in `total_agent_count`.
- Ingestion Pipeline label rows can still appear in `conversation_with_labels_d`.
- If an agent only has ingestion-pipeline Agent Assist evidence and no valid Normal conversations, the guard keeps the day at `0`, not `1`.

This behavior is deliberate: the system has evidence related to the agent/day, but the excluded source must not inflate Active Days.

## Current Limitations

Agent Assist evidence is still based on login/online-event overlap, not direct feature usage.

Known limitations:

- False positives are possible when online events imply the agent was logged into a Cresta app even if the agent was not actively using Agent Assist for a specific conversation.
- False negatives are possible when heartbeats are missing or delayed, such as sleep, idle, VPN, network interruption, or short gaps between online events.
- Current-day data can lag because labels are written by cron rather than synchronously with conversation writes.
- Legacy rows with `message_id = ''` can coexist with message-level rows during transition/backfill windows; read paths account for this.

## Historical Fixes

Relevant merged PRs:

| PR | Repository | Merged | Behavior impact |
|---|---|---|---|
| #25706 | `go-servers` | 2026-02-18 | Processes closed conversations by `ended_at` to avoid stale labels after transfer/reassignment. |
| #8844 | `cresta-proto` | 2026-06-09 | Adds message fields to `ConversationWithLabels`. |
| #213 | `clickhouse-schema` | 2026-06-10 | Adds `message_id`, `message_agent_user_id`, `message_time`; extends sort key with `message_id`. |
| #28531 | `go-servers` | 2026-06-10 | Switches write path to per-message Agent Assist labeling. |
| #28706 | `go-servers` | 2026-06-12 | Updates read paths for per-message rows and mixed old/new rows. |
| #29030 | `go-servers` | 2026-06-18 | Adds fallback rows for conversations with no agent messages. |
| #29058 | `go-servers` | 2026-06-18 | Removes message timestamp filter from message lookup; fixes multi-day conversations. |
| #29084 | `go-servers` | 2026-06-19 | Chunks message lookup by conversation IDs to avoid Postgres parameter-limit failures. |

## Practical Debugging Checklist

When a cell looks wrong, check:

1. Does `conversation_d` have closed conversations for the agent/day/usecase/source filters?
2. Is `exclude_ingestion_pipeline_from_agent_stats` enabled?
3. Does `conversation_with_labels_d` have `has_agent_assistance = 1` rows?
4. Are the label rows message-level (`message_id != ''`) or fallback/legacy (`message_id = ''`)?
5. For message-level rows, does `message_time` fall in the requested day?
6. For fallback/legacy rows, does `conversation_start_time` fall in the requested day?
7. Was the cron/backfill run after the conversation closed?
8. Are there only ingestion-pipeline rows when the flag excludes ingestion?

## Summary

- `1`: qualifying conversation plus qualifying Agent Assist evidence for the day.
- `0`: evidence exists, but the active-day criteria are not met for the selected configuration.
- `N/A`: no row/evidence for the agent/day, so the day is excluded from averages.
- New Agent Assist labels are message-granular.
- Conversations with no agent messages get fallback conversation-level rows.
- Leaderboard active-day attribution uses `message_time` for new rows and `conversation_start_time` for fallback/legacy rows.
