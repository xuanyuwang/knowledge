# CONVI-6298: Reindex Process Scorecards into ClickHouse

**Author**: Xuanyu Wang
**Date**: 2026-02-24
**Linear**: https://linear.app/cresta/issue/CONVI-6298

## Problem

The existing `batch-reindex-conversations` cron job is conversation-centric: it queries `app.chats`, calls `BatchIndexConversations`, and reads scorecards by `conversation_id`. This means **process scorecards** (`SCORECARD_TEMPLATE_TYPE_PROCESS`) are never reindexed into ClickHouse because they aren't tied to a conversation row in `app.chats`.

When process scorecard data in ClickHouse becomes stale or missing, there is no mechanism to backfill it.

## Solution

Add a parallel reindex path that queries process scorecards directly from PostgreSQL and writes them to ClickHouse using the existing `WriteScores`/`WriteScorecards` functions (which already support `isProcessScorecard=true`).

```
batch-reindex-conversations cron
          |
    REINDEX_MODE env var (default: "conversation")
          |
    ┌─────┼─────────┐
    │     │         │
 convo  process    all
    │     │         │
 existing  NEW      both
  flow    flow
           │
    ┌──────┴──────┐
    │             │
  Query PG     Write CH
  director.     WriteScores()
  scorecards    WriteScorecards()
  + template    isProcessScorecard=true
  revisions
  + historic
  scores
```

## Changes

### cresta-proto

- `job.proto` — new enum `JOB_TYPE_REINDEX_PROCESS_SCORECARDS = 58`
- `job_payload.proto` — new `ReindexProcessScorecardsPayload` message with `start_time`, `end_time`, `clean_up_before_write`, `reindex_batch_size`
- `reindex_process_scorecards.proto` — new workflow input proto

### go-servers

- **New package** `temporal/ingestion/reindexprocessscorecards/` — Temporal workflow + activity
- **New file** `jobhandler/reindex_process_scorecards_handler.go` — job handler (ADMIN/SUPER_ADMIN)
- **Modified** `registry.go` — register new handler
- **Modified** `registration.go` — register workflow + activity with ClickHouseClient DI
- **Modified** `batch-reindex-conversations/task.go` — `REINDEX_MODE` env var

## Activity Logic

1. Count process scorecards in `[start_time, end_time)` via JOIN on `director.scorecards` + `director.scorecard_template_revisions` (where `type = 2`)
2. Fetch in batches (default 50, configurable via env flag or job payload)
3. For each batch, load `director.historic_scorecard_scores` by scorecard IDs
4. Optionally delete existing CH data if `clean_up_before_write = true` (off by default — CH `ReplacingMergeTree` auto-deduplicates by `update_time`)
5. Write scores + scorecard rows to CH via existing functions
6. Heartbeat after each batch for fault-tolerant resumption

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `REINDEX_MODE` | `conversation` | `conversation`, `process`, or `all` |
| `REINDEX_START_TIME` | 24h ago | RFC3339 start time |
| `REINDEX_END_TIME` | now | RFC3339 end time |
| `REINDEX_PROCESS_SCORECARDS_BATCH_SIZE` | 50 | PG query batch size |
| `REINDEX_PROCESS_SCORECARDS_MAX_ATTEMPTS` | 6 | Temporal retry attempts |
| `REINDEX_PROCESS_SCORECARDS_HEARTBEAT_TIMEOUT` | 5m | Activity heartbeat timeout |

## Rollout Plan

1. **Merge proto changes** to cresta-proto, wait for codegen
2. **Merge go-servers changes** — default `REINDEX_MODE=conversation` means zero behavior change
3. **Test on staging** — set `REINDEX_MODE=process` with a known time range, verify CH data with `batch_verify.py`
4. **Enable in prod** — switch cron to `REINDEX_MODE=all` to run both conversation and process scorecard reindex

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| PG query load from JOIN on `director.scorecards` + `scorecard_template_revisions` | Conservative batch size (50), configurable via env var or job payload |
| Proto field number conflicts with in-flight PRs | Verified enum 58 and oneof 57 are available on HEAD |
| CH client availability | Injected via DI (same pattern as `retention` activities) |
| Backward compatibility | Default mode preserves existing behavior; no changes unless `REINDEX_MODE` is set |
