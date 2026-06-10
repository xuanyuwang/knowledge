# Codex Session: Pack Rat Scorecard Sync Incident

**Date:** 2026-06-08  
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Knowledge project:** `/Users/xuanyu.wang/repos/knowledge/pg-ch-scorecard-sync-investigation`  
**Customer/profile:** `pack-rat/us-east-1`  
**Platform conversation id:** `12112918`  
**Internal conversation id:** `019e7e88-d362-72a5-be86-51c5ddd865bb`

## Summary

The ticket looked like a broad conversation data-sync issue, but the concrete divergence was limited to one submitted scorecard on an otherwise synced conversation.

Postgres and ClickHouse matched for:

- conversation row: 1 in each store
- message rows: 162 in each store
- scorecard rows: 17 scorecards present in both stores

The affected scorecard was:

- scorecard id: `019e7e99-e3ce-7f3e-9b03-4e4bccc61532`
- template id: `03c5bd63-0647-4844-9ce0-ae97346a10cc`
- revision: `9b2b3a73`
- score: `87`

## Divergence

Postgres source of truth:

- `director.scorecards.submitted_at = 2026-06-01 20:01:10.868867+00`
- `director.scorecards.updated_at = 2026-06-01 20:01:10.869503+00`
- `director.scorecards.manually_scored = true`
- `director.scores` count for the scorecard: 50

ClickHouse before repair:

- `scorecard_d.scorecard_submit_time = 1970-01-01 00:00:00`
- `scorecard_d.scorecard_last_update_time = 2026-06-01 20:01:03.053811`
- `score_d` count for the scorecard: 44
- all 44 CH score rows had zero `scorecard_submit_time`

PG rows not emitted to CH `score_d`:

- `019a9ce1-832c-7516-94ee-70a2e4cbd72b`
- `4968a28f-8b0a-4542-8999-9cfbe4d23d8c`
- `537a912c-3583-4200-93b5-aefd783bcdf5`
- `653a4bbb-2712-4586-b942-f63b16300b4f`
- `67f74b5d-bb55-43bc-b0c4-d06ed73a26f0`
- `e9369e4e-3272-4af5-94f4-e3cad9caf315`

These are top-level chapter aggregate rows in the template: `Intro`, `Education`, `Customer Service`, `Sold Orders`, `Recap`, and `Close`. The CH writer path in `BuildScoreRowsFromDirectorScores` validates chapter scores but intentionally does not emit them as `score_d` rows, so the correct post-repair comparison is PG criteria excluding chapter rows vs CH `score_d`.

## Likely Failure Mode

This fits the known scorecard async-write class:

1. Scorecard exists in PG and initial CH rows are written.
2. A later manual submit updates PG.
3. CH projection either misses the submit-time write or receives a stale projection.
4. CH remains converged enough for scorecard identity and score, but stale for submit status on `scorecard_d` and emitted `score_d` rows.

Relevant code paths:

- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/coaching/action_submit_scorecard.go`
- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/coaching/action_create_scorecard.go`
- `/Users/xuanyu.wang/repos/go-servers/shared/clickhouse/conversations/conversation.go`
- `/Users/xuanyu.wang/repos/go-servers/temporal/ingestion/reindexscorecards/activity.go`

## Repair

Created a targeted `JOB_TYPE_REINDEX_SCORECARDS` job for the one conversation scorecard:

- job: `customers/pack-rat/profiles/us-east-1/jobs/75308d52-2d4f-4783-9095-a185131cfaae`
- Temporal workflow id: `reindexscorecards-pack-rat-us-east-1-0cc02074-961a-4a5d-ae43-a656caa6d7fa`
- run id: `605a218d-4e84-4430-abc4-332cddf8a70e`

Payload shape:

```json
{
  "parent": "customers/pack-rat/profiles/us-east-1",
  "job": {
    "type": "JOB_TYPE_REINDEX_SCORECARDS",
    "payload": {
      "reindexScorecardsPayload": {
        "conversationScorecardResourceNames": [
          "customers/pack-rat/profiles/us-east-1/scorecards/019e7e99-e3ce-7f3e-9b03-4e4bccc61532"
        ]
      }
    }
  }
}
```

Why not use `cron/task-runner/tasks/scorecard-sync-monitor` for this repair:

- The monitor checks existence only: PG scorecard IDs vs `SELECT DISTINCT scorecard_id FROM scorecard_d`.
- This scorecard already existed in `scorecard_d`, so the monitor would report missing count 0 for the exact Pack Rat/template/usecase window.
- The monitor's backfill trigger is useful when a scorecard is absent from CH; this incident needed reindexing an existing-but-stale CH row.

## Post-Repair Validation

After the targeted reindex:

- `scorecard_d.scorecard_submit_time = 2026-06-01 20:01:10.868867`
- `scorecard_d.scorecard_last_update_time = 2026-06-01 20:01:10.869503`
- `scorecard_d.submitter_user_id = c59af400af1b5303`
- `score_d FINAL` emitted rows: 44
- `score_d FINAL` rows with zero submit timestamp: 0
- `score_d FINAL` max update time: `2026-06-08 15:08:44.271818`

## Repeatable Query Pattern

Start from platform id, resolve to internal conversation id:

```sql
select id, customer_id, profile_id, platform_chat_id, platform_id, conversation_id,
       usecase_id, agent_user_id, started_at, ended_at, created_at
from app.chats
where platform_chat_id = '<platform_conversation_id>'
   or conversation_id = '<conversation_id>';
```

Then compare Postgres and ClickHouse at each layer:

- `app.chats` vs `conversation_d`
- `app.messages` vs `message_d`
- `director.scorecards` vs `scorecard_d`
- `director.scores` vs `score_d`

Use `FINAL` or explicit latest-row logic when validating a repaired ClickHouse row.
