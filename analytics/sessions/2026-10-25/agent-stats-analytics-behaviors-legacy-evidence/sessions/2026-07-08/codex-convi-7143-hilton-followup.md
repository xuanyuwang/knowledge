# CONVI-7143 Hilton Follow-Up: Linda Nesmith Jun 29 / Jul 6

**Date:** 2026-07-08
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/go-servers`
**Ticket/comment:** Linear CONVI-7143 `comment-ba6c34af`

## Prompt

Investigate the follow-up customer example where Hilton Assistance Insights still shows a powered-by-Agent-Assist discrepancy after the Jun 16 targeted backfill:

- Agent: Linda Nesmith
- Jun 29 example: Assistance Insights shows `8 total / 7 powered`; Closed Conversations looks real-time; Source=batch does not reveal the missing call.
- Customer also reported a similar Jul 6 recurrence.

## Prior Context

The original CONVI-7143 Jun 16 Gaylynn Bryant miss was a stale-label problem. Existing label rows were written before PR #29058, where stale code queried agent messages with both `conversation_id` and batch `platform_timestamp` bounds even though conversations were selected by end time. A one-off backfill on the current cron image repaired the target conversation and the UI moved to `10 total / 10 powered`.

## Data Findings

Linda Nesmith active agent maps to:

- `agent_user_id = 6beb20c585ac437f`
- HGV Club `usecase_id = voice-club-care`

Follow-up PG verification used the read-only `voice-prod` / `hilton-voice` connection from:

```bash
cresta-cli connstring -i --read-only voice-prod voice-prod hilton-voice
```

Online-session source-table verification used the read-only `chat-prod` / `auth-prod` / `hilton` connection from:

```bash
cresta-cli connstring -i --read-only chat-prod auth-prod hilton
```

The relevant auth source table is `auth.user_online_activities`, with columns:

- `customer_id`
- `user_id`
- `start_at`
- `application_type`
- `end_at`
- `is_online`
- `navigation_path`

For Jun 29 local day (`2026-06-29 04:00:00Z` to `2026-06-30 04:00:00Z`), the unpowered conversation found in the matching slice was:

- `conversation_id = 019f1486-73be-7800-ad45-aafcb80bd34d`
- `conversation_start_time = 2026-06-29 17:56:23.644165`
- `conversation_end_time = 2026-06-29 17:57:26.000000`
- `conversation_source = 0`
- `is_voice_mail = true`
- only agent message at `2026-06-29 17:56:33.785000`
- `conversation_with_labels_d` rows: `0`

Relevant online Agent Assist intervals for Jun 29:

- `2026-06-29 17:44:21.762` to `2026-06-29 17:55:34.641`
- `2026-06-29 17:56:52.896` to `2026-06-29 18:31:11.875`

The only agent message at `17:56:33.785` falls in the gap between intervals, so no label row is expected under the current labeler semantics.

For Jul 6 local day (`2026-07-06 04:00:00Z` to `2026-07-07 04:00:00Z`), the unpowered conversation found in the matching slice was:

- `conversation_id = 019f3858-bec2-7740-902b-a625c8250b37`
- `conversation_start_time = 2026-07-06 16:52:47.973654`
- `conversation_end_time = 2026-07-06 16:53:55.000000`
- `conversation_source = 0`
- `is_voice_mail = true`
- agent messages at `2026-07-06 16:53:06.993000`
- `conversation_with_labels_d` rows: `0`

Relevant online Agent Assist intervals for Jul 6:

- `2026-07-06 16:37:20.864` to `2026-07-06 16:45:07.316`
- `2026-07-06 16:45:10.888` to `2026-07-06 16:52:21.086`
- `2026-07-06 16:54:12.630` to `2026-07-06 17:13:19.221`
- `2026-07-06 17:13:36.525` to `2026-07-06 17:32:32.180`

The agent message at `16:53:06.993` falls in the gap between intervals, so it also receives no label row under the current semantics.

Auth PG returned `message_inside_interval = false` for every nearby online interval on both dates.

The surrounding successful label rows for Jun 29 and Jul 6 were written after PR #29058:

- Jun 29 examples: update times around `2026-06-29 16:31:01`, `17:30:53`, `18:01:41`, `18:30:50`, `19:01:36`
- Jul 6 examples: update times around `2026-07-06 14:30:42`, `17:00:51`, `18:30:54`, `19:00:56`

Therefore these later examples are not stale-code fallout from the Jun 18 fix. They are online-event coverage gaps relative to message timestamps.

PG evidence confirms the missing conversations themselves are real-time source and have the same message timestamps:

| reported_day | conversation_id | usecase_id | agent_user_id | source | started_at | ended_at | agent_message_count | first_agent_message_at |
|---|---|---|---|---:|---|---|---:|---|
| 2026-06-29 | `019f1486-73be-7800-ad45-aafcb80bd34d` | `voice-club-care` | `6beb20c585ac437f` | 0 | `2026-06-29 17:56:23.644165+00` | `2026-06-29 17:57:26+00` | 1 | `2026-06-29 17:56:33.785+00` |
| 2026-07-06 | `019f3858-bec2-7740-902b-a625c8250b37` | `voice-club-care` | `6beb20c585ac437f` | 0 | `2026-07-06 16:52:47.973654+00` | `2026-07-06 16:53:55+00` | 1 | `2026-07-06 16:53:06.993+00` |

PG source distribution for Linda/HGV Club on the two local days:

| local_day | conversation_source | conversations |
|---|---:|---:|
| 2026-06-29 | 0 | 10 |
| 2026-07-06 | 0 | 19 |

## Code Evidence

- `RetrieveConversationStats` with Agent Assist ON builds a subquery from `conversation_with_labels_d` and requires `max(has_agent_assistance) = true`.
  - `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_conversation_stats_clickhouse.go`
- The labeler fetches conversations by closed/end time, fetches agent messages, then calls `labelMessages`.
  - `/Users/xuanyu.wang/repos/go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go`
- `labelMessages` only returns a label when a message timestamp overlaps an online event for the same agent:
  - `!msg.Timestamp.Before(event.StartTime) && !msg.Timestamp.After(event.EndTime)`
- `writeMessagesWithLabel` writes `has_agent_assistance = true`; unassisted new-style conversations simply have no label rows.

## Interpretation

The follow-up is a different class from the original Jun 16 miss:

- Jun 16: valid AA message existed, but stale pre-#29058 cron code missed it; backfill fixed the row.
- Jun 29 / Jul 6: the agent messages fall outside recorded AA online windows, so the labeler has no evidence to mark the conversations powered.

Closed Conversations `Source` is not a valid way to identify conversations that are not powered by Agent Assist. Source reflects the conversation ingestion/source dimension, not Agent Assist online-session evidence. In both later Linda examples, the missing conversations are real-time source but unpowered by the AA metric because there is no overlapping online event at message time.

## Follow-Ups

- If product expects "real-time conversation" to imply "powered by Agent Assist", the metric definition must change; the current implementation intentionally uses message-overlap-with-AA-online-session evidence.
- The support workflow should not tell customers to use Closed Conversations Source=batch to find non-powered AA conversations.
- A possible product improvement is a drilldown/filter for Agent Assist ON/OFF backed by `conversation_with_labels_d`, but that is separate from data repair.
