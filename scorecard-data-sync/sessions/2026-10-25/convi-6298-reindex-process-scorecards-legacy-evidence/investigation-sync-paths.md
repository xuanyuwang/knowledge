# How Process Scorecards Sync from PostgreSQL to ClickHouse

**Created**: 2026-02-24

## All Callers of WriteScores / WriteScorecards

There are **4 code paths** that write scorecard data to ClickHouse:

| # | Caller | File | `isProcessScorecard` | Handles process? |
|---|--------|------|---------------------|-----------------|
| 1 | CreateScorecard async work | `coaching/action_create_scorecard.go:844` | `!isConvoTemplate` (dynamic) | **YES** |
| 2 | BatchIndexConversations | `conversation.go:833` via `buildScoreRows` | `false` (hardcoded) | **NO** |
| 3 | AutoQA autoscoring | `autoqa/action_trigger_conversation_autoscoring.go:269` | `false` (hardcoded) | **NO** |
| 4 | ReindexProcessScorecards | `reindexprocessscorecards/activity.go:122` | `true` (hardcoded) | **YES** |

## Path 1: Real-Time Sync (CreateScorecard RPC)

This is the **only existing production path** that writes process scorecards to CH.

```
CreateScorecard RPC
  │
  ├─ Synchronous (in PG transaction):
  │   ├─ Write scorecard → director.scorecards
  │   ├─ Write scores → director.scores
  │   └─ Write historic scores → director.historic_scorecard_scores
  │
  └─ Asynchronous (goroutine, after commit):
      ├─ Read historic_scorecard_scores from write replica
      ├─ WriteScores(ctx, db, chClient, scorecard, !isConvoTemplate, ...)
      └─ WriteScorecards(ctx, db, chClient, scorecard, !isConvoTemplate, ...)
```

**Key code** (`action_create_scorecard.go:844-870`):
```go
scoreRows, err := clickhouse.WriteScores(
    asyncCtx, db, s.clickHouseClient,
    scorecard,
    !isConvoTemplate,           // ← true for process scorecards
    historicScorecardScores,
    convoStartAt, languageCode,
)
// ...
clickhouse.WriteScorecards(asyncCtx, db, s.clickHouseClient,
    scorecard, !isConvoTemplate, scoreRows)
```

The `!isConvoTemplate` flag is dynamic — when the template is a process template, this correctly passes `true` to the CH write functions.

**Two variants** of async work exist:
- `scorecardAsyncWorkReadFromDB` (new, when `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE=true`) — reads fresh data from PG write replica
- `asyncScorecardWork` (legacy, default) — may have race condition with historic score writes

## Path 2: BatchIndexConversations (Reindex)

This path **explicitly excludes process scorecards**.

**Filter** (`conversation.go:1887`):
```go
result := db.Where(
    "(scorecard_type IS NULL OR scorecard_type = ?)",
    int32(coachingpb.ScorecardType_SCORECARD_TYPE_UNSPECIFIED),
).Find(&scorecards, &dbmodel.Scorecards{
    Customer:       conversationName.CustomerID,
    Profile:        conversationName.ProfileID,
    ConversationID: conversationName.ConversationID,
})
```

This query only returns scorecards where `scorecard_type` is NULL or 0 (UNSPECIFIED), filtering out process scorecards (`scorecard_type = 2`).

Additionally, `buildScoreRows` is called with `false` hardcoded:
```go
scoreRows := buildScoreRows(..., false /* isProcessScorecard */)
```

## Path 3: AutoQA Autoscoring

Hardcodes `false` — only handles conversation scorecards:
```go
clickhouse.WriteScores(ctx, db, s.clickHouseClient, dbScorecard,
    false, /* isProcessScorecard */ ...)
```

## Path 4: ReindexProcessScorecards (NEW — CONVI-6298)

Our new temporal workflow — the only reindex path for process scorecards.

## Gap Analysis

| Scenario | Process scorecards written to CH? |
|----------|--------------------------------|
| Process scorecard created via CreateScorecard RPC | **YES** — via async work (Path 1) |
| Conversation reindexed via BatchIndexConversations | **NO** — filtered out by `scorecard_type` check |
| Conversation reindexed via `batch-reindex-conversations` cron | **NO** — uses BatchIndexConversations |
| AutoQA scores a conversation | **NO** — hardcoded `false` |
| Process scorecard CH data becomes stale/missing | **NO FIX** until CONVI-6298 |

## Why Process Scorecards Get Out of Sync

1. **Async write failure**: The CreateScorecard async goroutine can fail silently (error logged but not retried). If CH write fails, the scorecard exists in PG but not CH.
2. **No reindex path existed**: Before CONVI-6298, there was no way to backfill process scorecards into CH.
3. **CH data loss/corruption**: If CH data is lost, BatchIndexConversations cannot recover process scorecards.

## Sharding Differences

| Type | Shard key | Calculation |
|------|-----------|-------------|
| Conversation scorecard | `conversation_start_time` | Lookup shard via CH query on conversation_id |
| Process scorecard | `ProcessInteractionAt` | `time.Unix() % shard_count` (deterministic) |

This is why `DeleteScoresOnShard` and `DeleteScorecardOnShard` have the `isConvoTemplate` parameter — they use different shard resolution strategies.
