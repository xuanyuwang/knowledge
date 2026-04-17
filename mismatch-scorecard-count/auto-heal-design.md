
Authors: xuanyu.wang@cresta.ai

Status: Draft

Last reviewed / updated: Apr 10, 2026

## Goal

Automatically detect and fix missing scorecards in ClickHouse (CH). Today, scorecards occasionally fail to sync from PostgreSQL (PG) to CH due to silent write failures, resulting in data gaps visible on the QM Report page (e.g., Brinks had 26% missing scorecards in March 2026).

- **Detect** missing scorecards hourly via a lightweight count comparison
- **Fix** them automatically via a targeted ID-based backfill workflow
- **Monitor** sync health via Groundcover/Datadog dashboards

## Non-goals

- Fixing the root cause of why CH writes fail silently (separate investigation)
- Backfilling historic schema (`historic.scorecard_scores`) — this design is CH-only
- Replacing the existing `reindexconversations` workflow — this is a complementary self-healing layer
- Multi-customer batch workflows — each workflow is scoped to a single customer/profile

## Background

### The Problem

Scorecards are created in PG (`director.scorecards` + `director.scores`) and asynchronously written to CH (`scorecard_d`, `score_d`) for analytics. The CH write can fail silently — no error is surfaced, and the scorecard simply doesn't appear in CH. This causes:

- **QM Report page**: Mismatched counts between PG-backed APIs (`RetrieveDirectorTaskStats`) and CH-backed APIs (`RetrieveQAScoreStats`)
- **Insights dashboards**: Missing data for filtered views (by criterion, by agent, etc.)

### Brinks Case Study (March 2026)

- **436 of 1,676 scorecards (26%) were missing** from CH for March 2026
- CH write path was broken before ~March 30; silently fixed after
- Time-range backfill (`reindexconversations`) recovered 401/436 (92%)
- **35 remained unfixed** because:
  - 34 had empty `conversation_id` (standalone QA scorecards from external system)
  - 1 had a conversation date different from its submit date
- These edge cases prove that time-range backfill is fundamentally insufficient

### Existing Tools

| Tool | What it does | Limitation |
|------|-------------|------------|
| `scorecard-sync-monitor` cron | Compares PG vs CH counts, sends Slack alerts | Detection only, no fix. Returns count, not IDs |
| `reindexconversations` workflow | Re-indexes all conversations in a time range → writes scorecards to CH | Expensive (scans all conversations), misses empty-conv-id and cross-date scorecards |
| `reindexscorecards` workflow | Re-indexes process scorecards by time range | Hardcoded to process scorecards only (`type=PROCESS`) |

### Glossary

- **CH**: ClickHouse — columnar analytics database
- **PG**: PostgreSQL — transactional database (source of truth for scorecards)
- **Scorecard**: A QA evaluation form with scores for each criterion
- **Process scorecard**: A scorecard not linked to a conversation (empty `conversation_id`)
- **Conversation scorecard**: A scorecard linked to a specific conversation
- **Sync monitor**: Existing cron task that detects PG→CH sync gaps

## Overview

```
[Hourly Cron: scorecard-sync-monitor (enhanced)]
    │
    ├─ For each customer/profile (existing per-task loop):
    │
    │   Phase 1: Count comparison (cheap, every run)
    │   ├─ PG: COUNT(*) submitted scorecards in [now-48h, now]
    │   ├─ CH: COUNT(DISTINCT scorecard_id) in same range
    │   ├─ Emit metrics (total, missing_count, missing_rate)
    │   └─ If counts match → done
    │
    │   Phase 2: Find missing scorecard resource names (only on mismatch)
    │   ├─ PG: fetch resource_id + conversation_id for submitted scorecards
    │   ├─ CH: fetch existing scorecard_ids
    │   ├─ Diff in memory → missing scorecard resource names
    │   └─ Build resource names: customers/{cid}/profiles/{pid}/scorecards/{sid}
    │
    │   Phase 3: Triage + dispatch (per customer/profile)
    │   ├─ Categorize by conversation_id (already fetched in Phase 2):
    │   │   ├─ conversation_id != "" → conversation_scorecard_resource_names
    │   │   └─ conversation_id == "" → process_scorecard_resource_names
    │   └─ ExecuteWorkflow("reindexscorecards", {
    │         process_scorecard_resource_names,
    │         conversation_scorecard_resource_names,
    │       })
    │
    └─ Emit metrics, alert on persistent failures

[Temporal Workflow: reindexscorecards]  ← EXTENDED (existing)
    │  Accepts: existing time-range fields + two new resource name lists
    │
    ├─ If process_scorecard_resource_names non-empty:
    │   └─ Activity: ReindexProcessScorecardsActivity (existing, extended)
    │       ├─ PG: fetch scorecards by resource name (director.scorecards)
    │       ├─ PG: fetch scores by scorecard_id (director.scores)
    │       ├─ BuildScoreRows(..., isProcessScorecard=true)
    │       ├─ BuildScorecardRows(...)
    │       └─ CH: BatchWriteScores + BatchWriteScorecards
    │
    ├─ If conversation_scorecard_resource_names non-empty:
    │   └─ Activity: ReindexConversationScorecardsActivity (NEW)
    │       ├─ PG: fetch scorecards by resource name (director.scorecards)
    │       ├─ PG: fetch scores by scorecard_id (director.scores)
    │       ├─ PG: fetch conversation context (app.chats) — StartedAt, EndedAt, Language
    │       ├─ PG: fetch agent user (users table) — for IsDevUser flag
    │       ├─ BuildScoreRows(..., isProcessScorecard=false)
    │       ├─ BuildScorecardRows(...)
    │       └─ CH: BatchWriteScores + BatchWriteScorecards
    │
    └─ Returns: count written, count failed, failed resource names
```

## Detailed Design

### Enhanced Sync Monitor (Cron Task)

Modify the existing `scorecard-sync-monitor` cron task in `cron/task-runner/tasks/scorecard-sync-monitor/`.

#### Phase 1: Count Comparison

Current monitor already queries PG and CH. Change from returning only a count to a two-phase approach:

```sql
-- PG (unchanged from current monitor)
SELECT COUNT(*)
FROM director.scorecards
WHERE customer = ? AND profile = ?
  AND submitted_at IS NOT NULL          -- only count submitted (completed) scorecards
  AND calibrated_scorecard_id IS NULL   -- exclude calibration copies (duplicates of originals)
  AND (scorecard_type IS NULL OR scorecard_type = 0)  -- exclude auto-QA scorecards (type=1)
  AND submitted_at >= ? AND submitted_at < ?

-- CH (unchanged from current monitor)
SELECT COUNT(DISTINCT scorecard_id) FROM scorecard_d
WHERE scorecard_id IN (?)              -- scope to PG IDs to avoid counting stale/deleted rows
  AND scorecard_submit_time >= ? AND scorecard_submit_time < ?
```

If PG count == CH count → skip to metrics, no ID lookup needed. This keeps hourly cost minimal — most customers will have 0 missing.

#### Phase 2: Find Missing Scorecard Resource Names

Only executes when Phase 1 detects a mismatch.

```sql
-- PG: fetch all IDs + conversation_id (for triage in same query)
SELECT resource_id, conversation_id
FROM director.scorecards WHERE ... (same filters)

-- CH: fetch existing IDs
SELECT DISTINCT scorecard_id FROM scorecard_d
WHERE scorecard_id IN (?)
  AND _row_exists = 1                  -- respect soft deletes (ReplacingMergeTree flag)
```

Diff in memory on the cron pod. Build scorecard resource names for missing IDs:
`customers/{customerID}/profiles/{profileID}/scorecards/{scorecardID}`

Memory cost: ~720KB per large customer for a 48h window (20K UUIDs × 36 bytes).

#### Phase 3: Triage + Dispatch

Categorize missing scorecards by `conversation_id` (already fetched in Phase 2):
- **Conversation scorecards**: `conversation_id != ""` → `conversation_scorecard_resource_names`
- **Process/standalone scorecards**: `conversation_id == ""` → `process_scorecard_resource_names`

Dispatch a single `reindexscorecards` workflow per customer/profile with both lists. Split into batches of 500 total resource names if needed.

```go
if len(convScorecardNames) > 0 || len(processScorecardNames) > 0 {
    for _, batch := range chunkPair(convScorecardNames, processScorecardNames, 500) {
        temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
            ID:        fmt.Sprintf("reindexscorecards-%s-%s-%s", customerID, profileID, batchID),
            TaskQueue: "reindex_scorecards",
            Namespace: "ingestion",
        }, ReindexScorecardsWorkflow, &ReindexScorecardsInput{
            ProcessScorecardResourceNames:      batch.process,
            ConversationScorecardResourceNames:  batch.conversation,
        })
    }
}
```

Precedent: `sync-users/coach_builder_update_dynamic_audience_task.go:98` uses the same pattern — cron task directly calls `temporalClient.ExecuteWorkflow()`.

### Workflow: `reindexscorecards` (Extended)

Extend the existing workflow at `temporal/ingestion/reindexscorecards/`. Currently accepts only a time range and processes `type=PROCESS` scorecards. Add two new input fields for targeted resource-name-based backfill, and a new activity for conversation scorecards.

#### Extended Input

```protobuf
message ReindexScorecardsPayload {
  // Existing fields (time-range mode)
  google.protobuf.Timestamp start_time = 1;
  google.protobuf.Timestamp end_time = 2;
  repeated ScorecardTemplateType scorecard_types = 3;
  // ...

  // New fields (resource-name mode, pre-triaged by monitor)
  repeated string process_scorecard_resource_names = 10;
  repeated string conversation_scorecard_resource_names = 11;
}
```

Customer/profile is parsed from the resource names. All names in a single workflow invocation belong to the same customer/profile (enforced by the monitor's per-task dispatch).

#### Workflow Logic

```
if len(process_scorecard_resource_names) > 0:
    execute ReindexProcessScorecardsActivity(process_scorecard_resource_names)

if len(conversation_scorecard_resource_names) > 0:
    execute ReindexConversationScorecardsActivity(conversation_scorecard_resource_names)
```

When neither new field is set, the workflow falls back to the existing time-range mode (backwards compatible).

#### Activity 1: `ReindexProcessScorecardsActivity` (Existing, extended)

Extend the existing activity with a new code path for resource-name-based lookup:

1. **Parse** customer_id, profile_id, scorecard_id from resource names
2. **Fetch scorecards** from `director.scorecards` by `resource_id IN (?)` (instead of time-range query)
3. **Fetch scores** from `director.scores` by `scorecard_id IN (?)`
4. **Build CH rows**: `BuildScoreRows(..., isProcessScorecard=true)`, `BuildScorecardRows(...)`
5. **Write to CH**: `BatchWriteScores` + `BatchWriteScorecards`

**Shard routing**: deterministic `int(referenceTime.Unix()) % shardCount` (existing process scorecard path).

#### Activity 2: `ReindexConversationScorecardsActivity` (New)

New activity for conversation-linked scorecards. Needs additional conversation context:

1. **Parse** customer_id, profile_id, scorecard_id from resource names
2. **Fetch scorecards** from `director.scorecards` by `resource_id IN (?)`
3. **Fetch scores** from `director.scores` by `scorecard_id IN (?)`
4. **Fetch conversation context** from `app.chats` by `conversation_id IN (?)`
   - Only needs: `StartedAt`, `EndedAt`, `Metadata.Message.EffectiveLanguage`
   - Build minimal `conversationsByID` map
5. **Fetch agent users** from users table by `agent_user_id` (for `IsDevUser` flag)
6. **Build CH rows**: `BuildScoreRows(..., isProcessScorecard=false)`, `BuildScorecardRows(...)`
7. **Write to CH**: `BatchWriteScores` + `BatchWriteScorecards`

The CH write path is shared with `reindexconversations` (`shared/clickhouse/conversations/conversation.go`). No new write logic needed.

**Shard routing**: Conversation scorecards use shard lookup via `scorecard_d_mv_by_conversation` in CH (existing path in `scorecard_score.go`).

### API

No new external APIs. Workflows are triggered internally by the cron task.

Proto changes:
- **Extended**: `ReindexScorecardsPayload` — add `repeated string process_scorecard_resource_names` and `repeated string conversation_scorecard_resource_names` fields

### Storage

No new storage. Reads from existing PG tables (`director.scorecards`, `director.scores`, `app.chats`, users). Writes to existing CH tables (`scorecard`, `score`).

### Security & Privacy

- No new data types captured
- No new storage locations — reads PG, writes CH (same as existing reindex workflows)
- No new external services
- Access controlled by Temporal namespace (`ingestion`) and cron task IAM role
- No PII handling changes

### Monitoring

#### Metrics (New)

Emit via `shared-go/framework/stats` → Groundcover/Datadog.

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `cresta.scorecard_sync_monitor.total_submitted` | Gauge | customer_id, profile_id, cluster | PG count in 48h window |
| `cresta.scorecard_sync_monitor.ch_count` | Gauge | customer_id, profile_id, cluster | CH count in 48h window |
| `cresta.scorecard_sync_monitor.missing_count` | Gauge | customer_id, profile_id, cluster | Number missing |
| `cresta.scorecard_sync_monitor.missing_rate_percent` | Gauge | customer_id, profile_id, cluster | Missing rate |
| `cresta.scorecard_sync_monitor.backfill_triggered` | Counter | customer_id, profile_id, cluster | Backfill workflows triggered |

Reference pattern: `knowledge-base-data-report/metrics.go` emits 12+ metrics with same approach.

#### Dashboards

Groundcover dashboard with:
- Missing rate over time per customer (line chart)
- Alert: any customer above 5% missing rate
- Total missing across cluster (aggregate)
- Backfill trigger frequency

#### Alerting

- Slack alert (existing) on missing rate > 1% (WARNING) or > 10% (CRITICAL)
- Groundcover alert on persistent missing (same scorecard missing for >3 consecutive runs)

### SLO

- **Target**: <1% missing rate for all customers within 48h of scorecard submission
- **Dependency SLOs**: PG (99.99%), CH (99.9%), Temporal (99.9%)
- **Degraded behavior**: If CH is down, monitor detects mismatch but backfill fails → Temporal retries. If PG is down, monitor fails → next hourly run retries.

### Testing Plans

1. **Unit tests**: Mock PG/CH queries, verify count comparison, ID diffing, triage categorization, batching logic
2. **Integration test**: Use embedded PG + CH to verify end-to-end: insert scorecard in PG, verify monitor detects missing, verify backfill writes to CH
3. **Manual validation**: Use the 35 remaining Brinks scorecards as test case — if ID-based backfill recovers all 35, the approach is validated
4. **Load test**: Simulate a customer with 10K missing scorecards (Brinks-scale) to verify batching and Temporal workflow throughput

### Technical Debts

- The sync monitor currently has no `stats.Client` injection — needs to be added to factory and task struct
- The `reindexscorecards` workflow's proto has an unused `scorecard_types` field — now being leveraged with the new resource name fields
- The CH `IN` clause for large ID lists may hit query size limits for extreme cases (>50K IDs). Batching at 500 resource names per workflow mitigates this

### Cost Estimate

- **Hourly cron**: Negligible — count comparisons take <1s per customer. ~280 customers across all clusters × 24 runs/day = ~6,700 count queries/day (PG + CH).
- **Backfill workflows**: Only triggered when missing scorecards detected. Normal state is 0 missing. Worst case (Brinks-like incident): ~1 workflow per 500 missing scorecards, each running for <1 minute.
- **Net new infra cost**: ~$0 (uses existing cron pod, Temporal cluster, PG, CH).

### Release Plans & Timelines

#### Day 1: Extend `reindexscorecards` workflow + process scorecard activity

- Add `process_scorecard_resource_names` and `conversation_scorecard_resource_names` to `ReindexScorecardsPayload` proto
- Add resource-name code path in existing `ReindexProcessScorecardsActivity`
- Validate with Brinks 34 standalone scorecards

#### Day 2: New `ReindexConversationScorecardsActivity` + monitor enhancement

- New activity (fetch scorecard/scores/conversation, write to CH)
- Reuses existing `BuildScoreRows`/`BuildScorecardRows`/`BatchWrite*` — mostly wiring
- Add `stats.Client` + metrics to sync monitor (count-first optimization, emit gauges)
- Validate with Brinks 1 cross-date scorecard

#### Day 3: Wire up auto-heal loop + deploy

- Add triage + dispatch logic to monitor (Phase 2-3 in architecture)
- Batching (500 resource names per workflow)
- Deploy hourly on voice-prod (Brinks), verify, roll out to all clusters
- Groundcover dashboard

**Total estimate**: ~3 days

## Design Review Notes

