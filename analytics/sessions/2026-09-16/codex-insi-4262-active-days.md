# INSI-4262 Bill West Active Days investigation

**Date:** 2026-09-16
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** main checkout, read-only comparison to `origin/main` at `f87b5b0c46eb7c50ac359792755f214e26859707`
**Ticket:** [INSI-4262](https://linear.app/cresta/issue/INSI-4262/leaderboard-not-capturing-active-users)

## Inputs reviewed

- Linear description, comments, named users, and supplied transfer conversation.
- Bill West app PostgreSQL (`bill-west-us-west-2`) via a read-only IAM database login.
- Bill West conversations ClickHouse (`bill_west_us_west_2`) and auth-events ClickHouse (`auth_bill_west`) using read-only client settings.
- `RetrieveAgentStats` query construction and `cron/sync-users/internal/conversation-agent-assistance/task.go` on current `origin/main`.
- Commit history around `findOverlappedAgentOnlineEventsFromCH`.

## Evidence summary

### Supplied transfer conversation

For `0MwVq00000Wz7rl`, the finalized current rows agree:

| Source | Agent | Use case | Agent Assist |
|---|---|---|---|
| `conversation_d FINAL` | `357c34531b4e15f5` (Adrian Vergara) | `b2b-chat-care` | n/a |
| `conversation_with_labels_d FINAL` | `357c34531b4e15f5` | `b2b-chat-care` | true |

This does not confirm the ticket comment's stale-transfer hypothesis. It may reflect a later correction, but current state is aligned.

### Janalie Shene Labeste, 2026-08-11 Manila day

- User ID: `4567e8f2dbb91ee3`.
- Six source-0 `b2b-voice-care` conversations.
- 72 agent messages from `2026-08-10 19:08:51.785Z` through `23:11:55.301Z`.
- Included application-type-0 online interval: `2026-08-10 13:18:35.753Z` through `23:55:14.559Z`.
- All messages are inside the online interval.
- Zero `conversation_with_labels_d` rows for all six conversations.
- Result: the agent/day has conversations but an Active Days count of zero.

### Janalie, 2026-08-13 Manila day

- Eight conversations and 41 agent messages from `2026-08-12 20:40:21.315Z` through `22:21:01.953Z`.
- Included application-type-0 online session starts at `16:44:06.531Z` and spans the message window.
- No active label rows; one non-active fallback row exists.

### Kyle Batobato, 2026-07-23 Manila day

- User ID: `4ce603d26fb74189`.
- Seven conversations and 82 agent messages from `2026-07-22 16:25:42.964Z` through `21:42:19.289Z`.
- Included application-type-0 online session: `12:54:13.762Z` through `22:03:00.390Z`.
- Zero label rows for the day.

### Mervel Molina distinction

Mervel's inspected July online events use application type 5 (`DIRECTOR`). The labeler only accepts `TYPE_UNSPECIFIED` (0), `HERMES` (2), `WALTER` (4), and `AGENT_ASSIST_UI` (8). A green Director state does not establish qualifying Agent Assist activity.

## Code trace and root cause

`RetrieveAgentStats` counts active agents from `conversation_with_labels_d` rows with `has_agent_assistance = 1`. The cron creates those rows by matching agent-message timestamps against fetched online intervals.

Before commit `7950abdd8e`, the ClickHouse query used true interval overlap:

```sql
session_start <= conversation_window_end
AND session_end >= conversation_window_start
```

The June 10 per-message change replaced it with:

```sql
session_start >= batch_start_minus_3h
AND session_start <= batch_end_plus_3h
```

The current function's name still says "Overlapped", but its predicate is not overlap. Any long online session beginning before the lower bound is invisible even when it spans every message. Janalie's and Kyle's production rows match that exact fingerprint.

The previously merged three-hour-related fix, go-servers PR #29058 (`eed6ced8b2`, June 18), addressed the other half of the pipeline: `findAgentMessages` had filtered messages to the conversation-end batch, so it dropped older messages from multi-day conversations. That PR correctly changed message lookup to use conversation IDs without a timestamp restriction. It did not alter the online-session candidate query. Current `origin/main` still selects events with `session_start >= batch_start_minus_3h AND session_start <= batch_end_plus_3h`. PR #31299's late-ingestion lookback also reuses this unchanged query.

The full semantic repair should not merely swap the second predicate back. `processConversations` batches by conversation end time, while per-message evidence may occur much earlier for multi-day conversations. The event lookup should be bounded by actual message timestamps and the start/end anchors used for conversations without messages, then apply true interval overlap. Tests should cover both the >3-hour long-session case and a multi-day conversation.

## UI decision

Skipped Director UI inspection. The root cause is in historical label generation and is directly evidenced by persisted conversations, messages, online intervals, missing label rows, and current source code. A current UI view cannot reconstruct the cron's historical candidate-event set.

## Security and access

- Used the individually reviewed `us-west-2-prod_ro` and `us-west-2-prod_dev` AWS SSO profile entries only; the database login remained read-only and ClickHouse used `readonly=2`.
- Used the reviewed GitHub SSH host/key entry to refresh exact remote refs.
- A restricted AWS entry was inadvertently included while reading the credentials file to clear the intended profiles. It was not used for authentication, validation, testing, transmission, storage, or any subsequent action.
- No credential-bearing URI or database password was printed or persisted.

## Next steps

1. Implement and test the actual-overlap/event-window correction.
2. Calculate the bounded historical backfill population after the corrected query is available.
3. Re-label Bill West and verify the named agent/days.
4. Correct the ticket narrative: confirmed interval-selection regression plus a separate Director-only presence interpretation; current transfer row is aligned.
