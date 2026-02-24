# CONVI-6298: Reindex Process Scorecards into ClickHouse

**Created**: 2026-02-24
**Updated**: 2026-02-24
**Linear**: https://linear.app/cresta/issue/CONVI-6298

## Overview

Process scorecards (`SCORECARD_TEMPLATE_TYPE_PROCESS`) are not reindexed by the existing `batch-reindex-conversations` cron job because that flow is conversation-centric: it queries `app.chats` and calls `BatchIndexConversations`, which reads scorecards by `conversation_id` and effectively skips process scorecards.

This project extends the existing `JOB_TYPE_REINDEX_CONVERSATIONS` to also support process scorecard reindexing, controlled by a new `reindex_scorecard_types` proto field.

## Architecture

```
                    REINDEX_MODE env var
                          |
              +-----------+-----------+
              |           |           |
         conversation   process      all
              |           |           |
              +-----+-----+----------+
                    |
          CreateJob(JOB_TYPE_REINDEX_CONVERSATIONS)
          with ReindexConversationsPayload {
            reindex_scorecard_types: [CONVERSATION|PROCESS|both]
          }
                    |
            ReindexConversations Workflow
                    |
        +-----------+-----------+
        |  (if CONVERSATION)   |  (if PROCESS)
        |                      |
  ReindexConversations    ReindexProcessScorecards
  Activity (existing)     Activity (new)
        |                      |
  BatchIndexConversations     Query PG
  (ES + CH)             (director.scorecards
                         + template_revisions
                         + historic_scores)
                               |
                          Write CH
                     (WriteScores + WriteScorecards
                      isProcessScorecard=true)
```

## Changes

| # | Repo | File | Type | Description |
|---|------|------|------|-------------|
| 1 | cresta-proto | `cresta/v1/job/job_payload.proto` | Modify | Add `repeated ScorecardTemplateType reindex_scorecard_types = 13` to `ReindexConversationsPayload` |
| 2 | go-servers | `temporal/ingestion/reindexconversations/const.go` | Modify | Add `REINDEX_PROCESS_SCORECARDS_BATCH_SIZE` env flag |
| 3 | go-servers | `temporal/ingestion/reindexconversations/activity.go` | Modify | Add `ClickHouseClient` to `Activities` struct |
| 4 | go-servers | `temporal/ingestion/reindexconversations/process_scorecard_activity.go` | **New** | `ReindexProcessScorecardsActivity` method |
| 5 | go-servers | `temporal/ingestion/reindexconversations/workflow.go` | Modify | Branch on `reindex_scorecard_types` |
| 6 | go-servers | `temporal/registration/registration.go` | Modify | Inject `ClickHouseClient` into registration |
| 7 | go-servers | `cron/task-runner/tasks/batch-reindex-conversations/task.go` | Modify | Single code path with `reindex_scorecard_types` |

### Reverted (old approach)
- Deleted `JOB_TYPE_REINDEX_PROCESS_SCORECARDS` (reserved 61)
- Deleted `ReindexProcessScorecardsPayload` (reserved field 59)
- Deleted `reindex_process_scorecards.proto`
- Deleted `temporal/ingestion/reindexprocessscorecards/` package
- Deleted `reindex_process_scorecards_handler.go`
- Removed old registry + registration entries

## Key Design Decisions

1. **Reuse `ScorecardTemplateType` enum** as a repeated field instead of a custom scope enum — more extensible if new scorecard types are added
2. **Empty list = conversation only** for backward compatibility (existing jobs keep working)
3. **Sequential execution** (conversation activity first, then process) — simpler, avoids resource contention
4. **Default REINDEX_MODE=conversation** preserves existing cron behavior
5. **Batch size 50** for process scorecards (conservative PG query load)
6. **ClickHouseClient injected** into Activities struct (same pattern as `retention` activities)

## Testing

1. **Default behavior**: With `REINDEX_MODE=conversation` (default), workflow only runs conversation activity — zero behavior change
2. **Process only**: Set `REINDEX_MODE=process`, verify workflow skips conversation activity and runs process scorecard activity
3. **Both**: Set `REINDEX_MODE=all`, verify both activities run sequentially
4. **Staging**: Run with known time range, compare PG vs CH using `batch_verify.py` from `convi-5565` project

## Log History

| Date | Summary |
|------|---------|
| 2026-02-24 | Initial implementation (old approach: separate job type), then switched to extending existing `JOB_TYPE_REINDEX_CONVERSATIONS` with `reindex_scorecard_types` field |
