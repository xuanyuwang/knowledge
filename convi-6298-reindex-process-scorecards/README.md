# CONVI-6298: Reindex Process Scorecards into ClickHouse

**Created**: 2026-02-24
**Updated**: 2026-02-24
**Linear**: https://linear.app/cresta/issue/CONVI-6298
**Status**: In progress — switching to scorecard-centric workflow approach

## Overview

Process scorecards (`SCORECARD_TEMPLATE_TYPE_PROCESS`) are not reindexed by the existing `batch-reindex-conversations` cron job because that flow is conversation-centric: it queries `app.chats` and calls `BatchIndexConversations`, which reads scorecards by `conversation_id` and effectively skips process scorecards.

This project creates a new scorecard-centric reindex workflow (`JOB_TYPE_REINDEX_SCORECARDS`) that writes scorecard data directly to ClickHouse, starting with process scorecards.

## Architecture (current direction)

```
                    REINDEX_MODE env var
                          |
              +-----------+-----------+
              |           |           |
         conversation   process      all
              |           |           |
   JOB_TYPE_REINDEX_   JOB_TYPE_REINDEX_
   CONVERSATIONS       SCORECARDS
   (existing, unchanged)    (new)
              |                |
   BatchIndexConversations   ReindexScorecards Workflow
   (ES + CH)                      |
                            Branch on scorecard_types
                                  |
                          (PROCESS for now)
                                  |
                           Query PG
                      (director.scorecards
                       + template_revisions
                       + historic_scores)
                                  |
                            Write CH
                       (WriteScores + WriteScorecards
                        isProcessScorecard=true)
```

## Approach History

### Approach 1: Separate `JOB_TYPE_REINDEX_PROCESS_SCORECARDS` (abandoned)
- Dedicated job type + workflow + handler for process scorecards only
- Rejected: too narrow, doesn't generalize

### Approach 2: Extend `JOB_TYPE_REINDEX_CONVERSATIONS` (abandoned)
- Add `reindex_scorecard_types` field to `ReindexConversationsPayload`
- Branch existing workflow to run conversation and/or process scorecard activities
- Rejected: `reindexconversations` is conversation-centric; process scorecards are not about conversations

### Approach 3: New `JOB_TYPE_REINDEX_SCORECARDS` (current) ← TODO
- New scorecard-centric workflow with `repeated ScorecardTemplateType` in payload
- Clean separation: conversations workflow stays conversation-centric
- Extensible: could later add direct conversation scorecard → CH path

## Changes Needed (Approach 3)

| # | Repo | File | Type | Description |
|---|------|------|------|-------------|
| 1 | cresta-proto | `job.proto` | Modify | Add `JOB_TYPE_REINDEX_SCORECARDS` enum value |
| 2 | cresta-proto | `job_payload.proto` | Modify | Add `ReindexScorecardsPayload` message + oneof field |
| 3 | cresta-proto | `reindex_scorecards.proto` | **New** | Workflow input proto |
| 4 | go-servers | `temporal/ingestion/reindexscorecards/` | **New** | Package: const, workflow, activity |
| 5 | go-servers | `jobhandler/reindex_scorecards_handler.go` | **New** | Job handler |
| 6 | go-servers | `jobhandler/registry.go` | Modify | Register handler |
| 7 | go-servers | `temporal/registration/registration.go` | Modify | Register workflow + activity |
| 8 | go-servers | `batch-reindex-conversations/task.go` | Modify | Create new job type for process/all mode |

### Revert (Approach 2 code on branch)
- Remove `reindex_scorecard_types` field from `ReindexConversationsPayload`
- Revert all changes to `reindexconversations/` package (workflow.go, activity.go, const.go)
- Delete `reindexconversations/process_scorecard_activity.go`
- Revert registration.go changes

## Key Design Decisions

1. **Reuse `ScorecardTemplateType` enum** as a repeated field — extensible if new scorecard types are added
2. **Batch size 50** for process scorecards (conservative PG query load)
3. **ClickHouseClient injected** into activity struct (same pattern as `retention` activities)
4. **`REINDEX_MODE` env var** on cron: `conversation` (default, existing behavior), `process`, `all`

## PRs

- cresta-proto PR #7919 — needs update to Approach 3
- go-servers PR #25916 — needs update to Approach 3

## Log History

| Date | Summary |
|------|---------|
| 2026-02-24 | Explored 3 approaches: separate job type → extend reindexconversations → new scorecard-centric workflow. Settled on Approach 3. PRs currently have Approach 2 code; need to update next session. |
