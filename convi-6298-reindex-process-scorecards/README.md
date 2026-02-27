# CONVI-6298: Reindex Process Scorecards into ClickHouse

**Created**: 2026-02-24
**Updated**: 2026-02-27
**Linear**: https://linear.app/cresta/issue/CONVI-6298
**Status**: PRs ready — proto merged, go-servers awaiting review

## Overview

Process scorecards (`SCORECARD_TEMPLATE_TYPE_PROCESS`) are not reindexed by the existing `batch-reindex-conversations` cron job because that flow is conversation-centric: it queries `app.chats` and calls `BatchIndexConversations`, which reads scorecards by `conversation_id` and effectively skips process scorecards.

This project creates a new scorecard-centric reindex workflow (`JOB_TYPE_REINDEX_SCORECARDS`) that writes scorecard data directly to ClickHouse, starting with process scorecards.

## Architecture

```
                    REINDEX_MODE env var (default: all)
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
                          (PROCESS by default)
                                  |
                           Query PG
                      (director.scorecards
                       + director.scores
                       + template_revisions)
                                  |
                      GenerateHistoricScorecardScores
                      (pure computation, no DB write)
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

### Approach 3: New `JOB_TYPE_REINDEX_SCORECARDS` (implemented)
- New scorecard-centric workflow with `repeated ScorecardTemplateType` in payload
- Clean separation: conversations workflow stays conversation-centric
- Extensible: could later add direct conversation scorecard → CH path

## Implementation

### Proto Changes (cresta-proto) — PR #7919 (merged)

| File | Change |
|------|--------|
| `cresta/v1/job/job.proto` | Added `JOB_TYPE_REINDEX_SCORECARDS = 61` |
| `cresta/v1/job/job_payload.proto` | Added `ReindexScorecardsPayload` message + oneof field 59 |
| `cresta/nonpublic/temporal/ingestion/reindex_scorecards.proto` | New workflow input proto |

### Go-servers Changes — PR #25916 (open)

| File | Type | Description |
|------|------|-------------|
| `temporal/ingestion/reindexscorecards/const.go` | New | Task queue name + env flags |
| `temporal/ingestion/reindexscorecards/workflow.go` | New | Temporal workflow (same pattern as reindexconversations) |
| `temporal/ingestion/reindexscorecards/activity.go` | New | `ReindexScorecardsActivity` — reconstructs scores from director data, writes to CH |
| `temporal/ingestion/reindexscorecards/BUILD.bazel` | New | Bazel build file |
| `jobhandler/reindex_scorecards_handler.go` | New | Job handler for `JOB_TYPE_REINDEX_SCORECARDS` |
| `jobhandler/registry.go` | Modified | Added handler entry |
| `jobhandler/BUILD.bazel` | Modified | Added handler source + dep |
| `temporal/registration/registration.go` | Modified | Added registration function + task queue |
| `temporal/registration/BUILD.bazel` | Modified | Added dep |
| `batch-reindex-conversations/task.go` | Modified | Added `REINDEX_MODE` env var + process scorecard dispatch |

## Key Design Decisions

1. **Reconstruct from director data**: Scores are built from `director.scores` + `scorecard_template_revisions` using `GenerateHistoricScorecardScores()`, avoiding a read from the `historic.scorecard_scores` table. The historic table is a denormalized projection of director data and can be rebuilt deterministically.
2. **No heartbeat resume**: Heartbeat is used for progress visibility and liveness detection only. On retry, the activity reprocesses from scratch — writes are idempotent (CH `ReplacingMergeTree` deduplicates). OFFSET-based pagination can't reliably resume when underlying data changes.
3. **Reuse `ScorecardTemplateType` enum** as a repeated field — extensible if new scorecard types are added
4. **Default to PROCESS only** when `scorecard_types` is empty — this is the gap being filled
5. **Default `REINDEX_MODE=all`** on cron — runs both conversation and process scorecard reindex jobs
6. **Batch size 50** for process scorecards (conservative PG query load)
7. **ClickHouseClient injected** into activity struct (same pattern as `retention` activities)

## PRs

- cresta-proto PR #7919 — **merged**
- go-servers PR #25916 — **open**, awaiting review

## Remaining Work

- [x] Proto changes (cresta-proto PR merged)
- [x] Go-servers implementation
- [x] Bazel build verification (all packages compile)
- [ ] Code review approval
- [ ] Staging test with known time range
- [ ] Verify CH data matches PG using `batch_verify.py`

## Log History

| Date | Summary |
|------|---------|
| 2026-02-24 | Explored 3 approaches: separate job type → extend reindexconversations → new scorecard-centric workflow. Settled on Approach 3. |
| 2026-02-26 | Reverted Approach 2, implemented Approach 3: proto changes + full go-servers implementation. Fixed CI failures, addressed CodeRabbit review comments. |
| 2026-02-27 | Refactored activity to reconstruct scores from director data (skip historic schema). Removed heartbeat resume (OFFSET pagination unreliable), kept heartbeat for progress visibility. Rebased onto main with updated cresta-proto dep. All packages build. |
