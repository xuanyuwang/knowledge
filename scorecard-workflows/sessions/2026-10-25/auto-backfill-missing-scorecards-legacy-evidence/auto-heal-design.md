
Authors: xuanyu.wang@cresta.ai

Status: Draft

Last reviewed / updated: Apr 17, 2026

## Goal

Automatically detect and fix missing scorecards in ClickHouse (CH). Today, scorecards occasionally fail to sync from PostgreSQL (PG) to CH due to silent write failures, resulting in data gaps visible on the QM Report page (e.g., Brinks had 26% missing scorecards in March 2026).

- **Detect** missing scorecards hourly for all scorecards, plus submitted and unsubmitted subsets
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
- **Submitted scorecard**: A scorecard with `submitted_at IS NOT NULL`
- **Unsubmitted scorecard**: A scorecard with `submitted_at IS NULL`
- **Conversation scorecard**: A scorecard with a non-empty `conversation_id`
- **Process scorecard**: A scorecard with an empty `conversation_id`
- **Sync monitor**: Existing cron task that detects PG→CH sync gaps

## Overview

```
[Hourly Cron: scorecard-sync-monitor (enhanced)]
    │
    ├─ For each customer/profile (existing per-task loop):
    │
    │   Phase 1: Inventory + rate calculation (cheap, every run)
    │   ├─ PG: fetch recent scorecard inventory in [now-48h, now]
    │   │   ├─ submitted slice: submitted_at in window
    │   │   └─ unsubmitted slice: submitted_at IS NULL AND created_at in window
    │   ├─ CH: fetch existing scorecard_ids for the same PG inventory
    │   ├─ Derive missing sets and rates for:
    │   │   ├─ all scorecards
    │   │   ├─ submitted scorecards
    │   │   └─ unsubmitted scorecards
    │   ├─ Emit metrics for all 3 views
    │   └─ If all missing sets are empty → done
    │
    │   Phase 2: Triage missing scorecard resource names
    │   ├─ Build one combined missing set for healing
    │   ├─ Split by conversation_id:
    │   │   ├─ conversation_id != "" → conversation_scorecard_resource_names
    │   │   └─ conversation_id == "" → process_scorecard_resource_names
    │   └─ Build resource names: customers/{cid}/profiles/{pid}/scorecards/{sid}
    │
    │   Phase 3: Dispatch (per customer/profile)
    │   └─ ExecuteWorkflow("reindexscorecards", {
    │         process_scorecard_resource_names,
    │         conversation_scorecard_resource_names,
    │       }) for each batch
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

#### Phase 1: Inventory + Missing-Rate Calculation

The monitor now needs three missing-rate views on every run:
- **All scorecards** in the rolling window
- **Submitted scorecards** in the rolling window
- **Unsubmitted scorecards** in the rolling window

Every scorecard is exactly one of:
- **Submitted** or **unsubmitted**
- **Conversation** or **process**

To do that reliably, fetch one recent PG inventory, then derive the slices in memory:

- **Submitted slice**: `submitted_at IS NOT NULL AND submitted_at >= ? AND submitted_at < ?`
- **Unsubmitted slice**: `submitted_at IS NULL AND created_at >= ? AND created_at < ?`
- **All slice**: union of the two sets above

```sql
-- PG: recent scorecard inventory for all 3 views
SELECT resource_id, conversation_id, created_at, submitted_at
FROM director.scorecards
WHERE customer = ? AND profile = ?
  AND calibrated_scorecard_id IS NULL
  AND (scorecard_type IS NULL OR scorecard_type = 0)
  AND (
    (submitted_at IS NOT NULL AND submitted_at >= ? AND submitted_at < ?)
    OR
    (submitted_at IS NULL AND created_at >= ? AND created_at < ?)
  )
```

Build three ID sets in memory from the PG rows:
- `all_scorecard_ids`
- `submitted_scorecard_ids`
- `unsubmitted_scorecard_ids`

Then fetch the CH rows for the same PG inventory:

```sql
-- CH: fetch existing IDs once, scoped to the PG inventory
SELECT DISTINCT scorecard_id
FROM scorecard_d
WHERE scorecard_id IN (?)
  AND _row_exists = 1
```

With the PG inventory set and CH existing-ID set, derive:
- `missing_all = all_scorecard_ids - existing_ids`
- `missing_submitted = submitted_scorecard_ids - existing_ids`
- `missing_unsubmitted = unsubmitted_scorecard_ids - existing_ids`

Emit all 3 totals, missing counts, and missing rates on every run. If all three missing sets are empty, no backfill is triggered.

Why use `created_at` for unsubmitted scorecards: unsubmitted scorecards do not have `submitted_at`, so `created_at` is the stable PG-side timestamp for the "recent but still unsubmitted" slice. This keeps the monitor focused on newly-created unsubmitted scorecards instead of dragging long-lived ones into every hourly run forever.

#### Phase 2: Triage

The PG inventory already includes `conversation_id`, so triage can happen without another PG query.

Build scorecard resource names for missing IDs:
`customers/{customerID}/profiles/{profileID}/scorecards/{scorecardID}`

Memory cost: ~720KB per large customer for a 48h window (20K UUIDs × 36 bytes).

For healing, we do **not** split submitted and unsubmitted into separate backfill queues. The submitted/unsubmitted distinction is only for metric calculation and alerting. Once we have the combined missing set, classify only by scorecard type:
- **Conversation scorecards**: `conversation_id != ""`
- **Process scorecards**: `conversation_id == ""`

#### Phase 3: Dispatch

Dispatch a single combined missing set per customer/profile. Split into batches of 500 total resource names if needed.

```go
for _, batch := range chunkPair(missingConversationNames, missingProcessNames, 500) {
    temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
        ID:        fmt.Sprintf("reindexscorecards-%s-%s-%s", customerID, profileID, batchID),
        TaskQueue: "reindex_scorecards",
        Namespace: "ingestion",
    }, ReindexScorecardsWorkflow, &ReindexScorecardsInput{
        ProcessScorecardResourceNames:      batch.process,
        ConversationScorecardResourceNames: batch.conversation,
    })
}
```

Precedent: `sync-users/coach_builder_update_dynamic_audience_task.go:98` uses the same pattern — cron task directly calls `temporalClient.ExecuteWorkflow()`.

### Workflow: `reindexscorecards` (Extended)

Extend the existing workflow at `temporal/ingestion/reindexscorecards/`. Currently accepts only a time range and processes `type=PROCESS` scorecards. Add two new input fields for targeted resource-name-based backfill, and a new activity for conversation scorecards.

No workflow input change is needed for submitted vs unsubmitted handling. That distinction exists only in monitor-side metrics.

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
| `cresta.scorecard_sync_monitor.total_all` | Gauge | customer_id, profile_id, cluster | PG count for all scorecards in the rolling window |
| `cresta.scorecard_sync_monitor.total_submitted` | Gauge | customer_id, profile_id, cluster | PG count for submitted scorecards in the rolling window |
| `cresta.scorecard_sync_monitor.total_unsubmitted` | Gauge | customer_id, profile_id, cluster | PG count for unsubmitted scorecards in the rolling window |
| `cresta.scorecard_sync_monitor.ch_count_all` | Gauge | customer_id, profile_id, cluster | CH count for all scorecards in the rolling window |
| `cresta.scorecard_sync_monitor.ch_count_submitted` | Gauge | customer_id, profile_id, cluster | CH count for submitted scorecards in the rolling window |
| `cresta.scorecard_sync_monitor.ch_count_unsubmitted` | Gauge | customer_id, profile_id, cluster | CH count for unsubmitted scorecards in the rolling window |
| `cresta.scorecard_sync_monitor.missing_count_all` | Gauge | customer_id, profile_id, cluster | Missing count for all scorecards |
| `cresta.scorecard_sync_monitor.missing_count_submitted` | Gauge | customer_id, profile_id, cluster | Missing count for submitted scorecards |
| `cresta.scorecard_sync_monitor.missing_count_unsubmitted` | Gauge | customer_id, profile_id, cluster | Missing count for unsubmitted scorecards |
| `cresta.scorecard_sync_monitor.missing_rate_percent_all` | Gauge | customer_id, profile_id, cluster | Missing rate for all scorecards |
| `cresta.scorecard_sync_monitor.missing_rate_percent_submitted` | Gauge | customer_id, profile_id, cluster | Missing rate for submitted scorecards |
| `cresta.scorecard_sync_monitor.missing_rate_percent_unsubmitted` | Gauge | customer_id, profile_id, cluster | Missing rate for unsubmitted scorecards |
| `cresta.scorecard_sync_monitor.backfill_triggered` | Counter | customer_id, profile_id, cluster | Backfill workflows triggered |

Reference pattern: `knowledge-base-data-report/metrics.go` emits 12+ metrics with same approach.

#### Dashboards

Groundcover dashboard with:
- Missing rate over time per customer for all/submitted/unsubmitted (3 series)
- Alert: any customer above 5% missing rate on submitted scorecards
- Total missing across cluster (aggregate)
- Backfill trigger frequency

#### Alerting

- Slack alert (existing) on submitted missing rate > 1% (WARNING) or > 10% (CRITICAL)
- Non-paging dashboard alert on unsubmitted missing rate > 5%
- Groundcover alert on persistent missing (same scorecard missing for >3 consecutive runs)

### SLO

- **Target (submitted)**: <1% missing rate for submitted scorecards within 48h of submission
- **Target (unsubmitted)**: <5% missing rate for unsubmitted scorecards created within the last 48h
- **Target (overall)**: <1% missing rate for the combined all-scorecard rolling window
- **Dependency SLOs**: PG (99.99%), CH (99.9%), Temporal (99.9%)
- **Degraded behavior**: If CH is down, monitor detects mismatch but backfill fails → Temporal retries. If PG is down, monitor fails → next hourly run retries.

### Testing Plans

1. **Unit tests**: Mock PG/CH queries, verify submitted/unsubmitted slice building, missing-rate calculation for all 3 views, triage categorization, and batching logic
2. **Integration test**: Use embedded PG + CH to verify end-to-end for both submitted and unsubmitted scorecards: insert scorecards in PG, verify monitor detects missing, verify backfill writes to CH
3. **Manual validation**: Use the 35 remaining Brinks scorecards as test case — if ID-based backfill recovers all 35, the approach is validated
4. **Draft validation**: Create an unsubmitted scorecard, confirm it shows up in the unsubmitted slice, gets healed, and disappears from the missing set without waiting for submission
5. **Load test**: Simulate a customer with 10K missing scorecards (Brinks-scale) to verify batching and Temporal workflow throughput

### Technical Debts

- The sync monitor currently has no `stats.Client` injection — needs to be added to factory and task struct
- The `reindexscorecards` workflow's proto has an unused `scorecard_types` field — now being leveraged with the new resource name fields
- The CH `IN` clause for large ID lists may hit query size limits for extreme cases (>50K IDs). Batching at 500 resource names per workflow mitigates this
- Unsubmitted monitoring depends on `created_at`, so long-lived drafts will age out of the rolling window unless we add a separate stale-draft monitor later

### Cost Estimate

- **Hourly cron**: Negligible — count comparisons take <1s per customer. ~280 customers across all clusters × 24 runs/day = ~6,700 count queries/day (PG + CH).
- **Backfill workflows**: Only triggered when missing scorecards detected. Normal state is 0 missing. Worst case (Brinks-like incident): ~1 workflow per 500 missing scorecards, each running for <1 minute.
- **Net new infra cost**: ~$0 (uses existing cron pod, Temporal cluster, PG, CH).

### Concrete Implementation Plan

#### Repo 1: `knowledge`

- Update this design doc with submitted + unsubmitted monitoring semantics
- Capture implementation sequencing and repo ownership
- Keep validation notes in `mismatch-scorecard-count/auto-heal-validation.md` aligned with the new scope if we learn anything while implementing

#### Repo 2: `cresta-proto`

- Update `cresta/v1/job/job_payload.proto`
  - Add `process_scorecard_resource_names`
  - Add `conversation_scorecard_resource_names`
- Regenerate the Go bindings consumed by `go-servers`
- Verify `ReindexScorecardsPayload` remains backward-compatible for existing time-range callers

#### Repo 3: `go-servers`

- `cron/task-runner/tasks/scorecard-sync-monitor/`
  - Refactor the task to build one recent PG inventory
  - Derive all/submitted/unsubmitted sets and missing rates
  - Emit the expanded stats set
  - Build one combined missing set for healing
  - Split the combined missing set into conversation scorecards and process scorecards for dispatch
- `cron/task-runner/tasks/scorecard-sync-monitor/result_collector.go`
  - Update the cluster summary and Slack formatting so submitted gaps are emphasized first
- `cron/task-runner/tasks/scorecard-sync-monitor/factory.go`
  - Inject `stats.Client` and any Temporal client dependency needed for direct workflow dispatch
- `temporal/ingestion/reindexscorecards/`
  - Extend the workflow for targeted resource-name mode
  - Add the conversation-scorecard activity
  - Extend the process-scorecard activity for resource-name lookup
  - Keep time-range mode working for existing callers
- `apiserver/internal/internaljob/jobhandler/reindex_scorecards_handler.go`
  - Verify the public job path still works after the payload change
- Tests
  - Update `cron/task-runner/testing/scorecard_sync_monitor_test.go`
  - Add workflow/activity coverage for submitted and unsubmitted targeted backfill

### Release Plans & Timelines

#### Day 1: Proto + targeted workflow plumbing

- Add `process_scorecard_resource_names` and `conversation_scorecard_resource_names` to `ReindexScorecardsPayload` proto
- Add resource-name code path in existing `ReindexProcessScorecardsActivity`
- Validate with Brinks 34 standalone scorecards

#### Day 2: Monitor refactor + submitted/unsubmitted metrics

- Refactor the monitor to build a recent scorecard inventory
- Compute missing rates for all/submitted/unsubmitted
- Add `stats.Client` + metrics to sync monitor
- Build one combined healing set from the missing IDs
- Validate with one submitted and one unsubmitted synthetic gap

#### Day 3: Conversation activity + rollout

- New conversation-scorecard activity (fetch scorecard/scores/conversation, write to CH)
- Reuses existing `BuildScoreRows`/`BuildScorecardRows`/`BatchWrite*` — mostly wiring
- Batching (500 resource names per workflow)
- Deploy hourly on voice-prod (Brinks), verify, roll out to all clusters
- Groundcover dashboard

**Total estimate**: ~3 days

## Design Review Notes
