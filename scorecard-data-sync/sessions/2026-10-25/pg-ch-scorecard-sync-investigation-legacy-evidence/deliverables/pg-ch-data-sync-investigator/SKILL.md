---
name: pg-ch-data-sync-investigator
description: Investigate and repair Cresta PostgreSQL to ClickHouse data-sync issues for conversations, messages, scorecards, and scores. Use when a ticket reports analytics, QA, Performance Insights, export, or Director data mismatches between Postgres source-of-truth tables and ClickHouse derived tables.
---

# PG to ClickHouse Data Sync Investigator

Use this skill for production sync incidents where a platform conversation id, internal conversation id, scorecard id, customer/profile, or analytics mismatch is provided.

## Ground Rules

- Treat Postgres as source of truth and ClickHouse as a derived projection.
- Use read-only Postgres credentials for investigation.
- Do not write directly to ClickHouse for scorecard repairs. Prefer the existing targeted reindex job so row construction stays in production code.
- Keep credentials out of notes, commits, and final reports. Store commands with placeholders only.
- Use `FINAL` or explicit latest-row grouping when validating repaired ClickHouse rows.

## ID Resolution

Tickets often provide a platform conversation id, not the internal conversation id used by ClickHouse.

```sql
select id, customer_id, profile_id, platform_chat_id, platform_id, conversation_id,
       usecase_id, agent_user_id, started_at, ended_at, created_at
from app.chats
where platform_chat_id = '<platform_conversation_id>'
   or platform_id = '<platform_conversation_id>'
   or conversation_id = '<internal_conversation_id>';
```

Use the returned `conversation_id` for ClickHouse queries.

## Investigation Ladder

Work from coarse to narrow. Stop broadening when the mismatch is isolated.

1. Conversation row:
   - PG: `app.chats`
   - CH: `conversation_d`
2. Message rows:
   - PG: `app.messages`
   - CH: `message_d`
3. Scorecard rows:
   - PG: `director.scorecards`
   - CH: `scorecard_d`
4. Criterion score rows:
   - PG: `director.scores`
   - CH: `score_d`
5. Lookup materialized views:
   - `conversation_d_mv_by_conversation`
   - `message_d_mv_by_conversation`
   - `scorecard_d_mv_by_conversation`
   - `score_d_mv_by_conversation`

## Core Postgres Checks

```sql
select count(*) as pg_message_count,
       min(created_at) as first_msg,
       max(created_at) as last_msg,
       min(platform_timestamp) as first_platform_ts,
       max(platform_timestamp) as last_platform_ts,
       min(start_time_ms) as min_start_ms,
       max(end_time_ms) as max_end_ms
from app.messages
where conversation_id = '<conversation_id>';
```

```sql
select s.resource_id, s.template_id, s.template_revision, s.scorecard_type,
       s.score, s.created_at, s.updated_at, s.submitted_at,
       s.ai_scored_at, s.manually_scored, s.auto_failed,
       count(sc.resource_id) as score_count
from director.scorecards s
left join director.scores sc
  on sc.customer = s.customer
 and sc.profile = s.profile
 and sc.scorecard_id = s.resource_id
where s.conversation_id = '<conversation_id>'
group by s.customer, s.profile, s.resource_id
order by s.resource_id;
```

## Core ClickHouse Checks

```sql
select count() as rows,
       min(conversation_start_time),
       max(conversation_end_time),
       any(platform_chat_id),
       any(platform_id),
       any(usecase_id),
       any(agent_user_id)
from conversation_d
where conversation_id = '<conversation_id>';
```

```sql
select count() as rows,
       min(create_time),
       max(create_time),
       min(platform_time),
       max(platform_time),
       min(start_time_ms),
       max(end_time_ms)
from message_d
where conversation_id = '<conversation_id>';
```

```sql
select scorecard_id, scorecard_template_id, scorecard_template_revision,
       score, scorecard_create_time, scorecard_last_update_time,
       scorecard_submit_time, ai_score_time, manually_scored,
       auto_failed, update_time
from scorecard_d
where conversation_id = '<conversation_id>'
order by scorecard_id;
```

```sql
select scorecard_id,
       count() as ch_scores,
       uniqExact(criterion_id) as ch_criteria,
       min(scorecard_submit_time),
       max(scorecard_submit_time),
       max(update_time)
from score_d
where scorecard_id = '<scorecard_id>'
group by scorecard_id;
```

## Classify The Mismatch

- **Missing conversation:** PG `app.chats` exists, CH `conversation_d` missing.
- **Missing messages:** PG and CH conversation rows exist, message counts differ.
- **Missing scorecard:** PG scorecard exists, CH `scorecard_d` missing.
- **Stale scorecard:** scorecard exists in CH but `scorecard_submit_time`, `scorecard_last_update_time`, `score`, publish fields, or manual flags lag PG.
- **Missing criteria:** scorecard exists in CH but `director.scores` count or criterion ids differ from `score_d`.
- **Lookup-only issue:** base CH table is correct but `_mv_by_conversation` count is missing or stale.
- **Expected zero:** PG source data is absent, scorecard is calibration-only, deleted, filtered by scorecard type, or outside intended analytics scope.

Before calling a `director.scores` vs `score_d` count difference a bug, account for template chapter rows. The ClickHouse writer validates chapter scores but does not emit chapter rows as `score_d` records.

## Repair Decision

Use targeted `JOB_TYPE_REINDEX_SCORECARDS` when the mismatch is in `scorecard_d` or `score_d` and the PG source row is correct.

`cron/task-runner/tasks/scorecard-sync-monitor` is useful for broad detection and repair of missing scorecard IDs. It compares PG inventory against `SELECT DISTINCT scorecard_id FROM scorecard_d` and triggers a reindex only for IDs absent from CH. It does not detect stale fields for scorecards that already exist in CH, such as stale `scorecard_submit_time` or stale `scorecard_last_update_time`.

For conversation scorecards, set:

```json
{
  "parent": "customers/<customer>/profiles/<profile>",
  "job": {
    "type": "JOB_TYPE_REINDEX_SCORECARDS",
    "payload": {
      "reindexScorecardsPayload": {
        "conversationScorecardResourceNames": [
          "customers/<customer>/profiles/<profile>/scorecards/<scorecard_id>"
        ]
      }
    }
  }
}
```

For process scorecards, use `processScorecardResourceNames` instead.

Use broad time-window reindex only when many rows are affected and the scope is understood. Use cleanup only when stale rows have different primary keys, such as criteria removed from a template or scorecards deleted from PG.

## Verification

After a repair job finishes or has had time to write:

```sql
select scorecard_id, score, scorecard_last_update_time,
       scorecard_submit_time, submitter_user_id,
       manually_scored, update_time
from scorecard_d FINAL
where scorecard_id = '<scorecard_id>';
```

```sql
select count() as ch_scores,
       countIf(scorecard_submit_time = toDateTime64('1970-01-01 00:00:00', 6)) as zero_submit_rows,
       min(scorecard_submit_time),
       max(scorecard_submit_time),
       uniqExact(criterion_id),
       max(update_time)
from score_d FINAL
where scorecard_id = '<scorecard_id>';
```

Then compare criterion ids:

```bash
comm -3 \
  <(psql "$PG_CONN" -At -c "select criterion_identifier from director.scores where scorecard_id = '<scorecard_id>' order by 1") \
  <(clickhouse client ... --query "select criterion_id from score_d FINAL where scorecard_id = '<scorecard_id>' order by 1")
```

No output means the criterion id sets match.

## Report Template

Include:

- customer/profile and environment
- platform id and internal conversation id
- exact mismatch class
- PG source-of-truth counts/timestamps
- CH derived counts/timestamps before repair
- repair job id and workflow id, if created
- CH validation after repair
- suspected failure mode and whether it is known or new
