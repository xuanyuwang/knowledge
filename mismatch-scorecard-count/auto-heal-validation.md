# Auto-Heal Missing Scorecards: Validation Checklist

**Created**: 2026-04-09
**Updated**: 2026-04-09 (validated)

## Idea

Run the sync monitor hourly. When missing scorecards are detected, trigger a targeted backfill for just those scorecards (not the entire time range). Verify after backfill. Repeat until fixed.

## Key Findings from Exploration

- **`reindexscorecards` workflow** (`temporal/ingestion/reindexscorecards/`) exists but is **hardcoded for process scorecards only** (type=PROCESS filter at `activity.go:315`)
- Proto (`ReindexScorecardsPayload`) has a `scorecard_types` field but the workflow ignores it
- `batch-reindex-conversations` cron creates **both** `reindexconversations` + `reindexscorecards` workflows by default (`REINDEX_MODE="all"`)
- Triggered via `InternalJobServiceClient.CreateJob()` with `JOB_TYPE_REINDEX_SCORECARDS`
- CH writes use the same shared path (`BuildScoreRows`, `BuildScorecardRows`) with an `isProcessScorecard` flag

## Validation Results

Results from code exploration and the Brinks backfill test case.

## What to Validate

### 1. Can `reindexscorecards` be extended for conversation scorecards?

**VALIDATED: Yes, feasible. Recommend new workflow over extending existing.**

The workflow currently only handles process scorecards. For conversation scorecards it would additionally need:
- **Conversation context**: `StartedAt`, `EndedAt`, `Metadata.Message.EffectiveLanguage` from `app.chats`
- **Shard routing**: Process scorecards use deterministic `int(referenceTime.Unix()) % shardCount`; conversation scorecards query CH `scorecard_d_mv_by_conversation` to find shard
- **`isProcessScorecard=false`**: The CH write path supports it, but the workflow currently hardcodes `true`

**Recommendation**: Create a **new workflow** rather than extending `reindexscorecards`. Reasons:
- Process and conversation scorecards have fundamentally different fetch logic (no conversation join vs conversation join)
- Different shard routing (deterministic vs CH query)
- The new workflow can accept scorecard IDs directly (not time range)

**Reference code in `reindexconversations`** — conversation scorecard call chain:
```
BuildClickHouseDataForBatchWrite() (conversation.go:850)
  → readScorecards()         (conversation.go:1877)  — PG: director.scorecards by conv_id
  → readScorecardScores()    (conversation.go:1785)  — PG: director.scores by scorecard_id
  → BuildScoreRows(..., isProcessScorecard=false)  (conversation.go:2977)
  → BuildScorecardRows(..., isProcessScorecard=false) (conversation.go:2881)
```
Lots of irrelevant code to prune: conversation indexing, message writes, annotation handling, agent assist data.

### 2. Can we backfill by scorecard IDs instead of time range?

**VALIDATED: Yes. The CH write path supports it with minimal conversation data.**

The write functions need these inputs per scorecard:

| Input | Source | Required for |
|-------|--------|-------------|
| `dbmodel.Scorecards` (full object) | PG `director.scorecards` by ID | `BuildScorecardRows` |
| `[]*historicmodel.ScorecardScores` | PG `director.scores` by scorecard_id | `BuildScoreRows` |
| `conversation.StartedAt` | PG `app.chats` by conversation_id | scorecard_time, shard routing |
| `conversation.EndedAt` | PG `app.chats` by conversation_id | duration calculation |
| `conversation.Metadata.EffectiveLanguage` | PG `app.chats` by conversation_id | language_code field |
| `agentUser.IsDevUser` | PG users table by agent_user_id | is_dev_user flag |

**No full conversation object needed.** The `conversationsByID` map only reads 3 fields. Can be constructed minimally:
```go
conversationMap[scorecardConvID] = &dbmodel.Chats{
    StartedAt: startedAt,
    EndedAt:   sql.NullTime{Time: endedAt, Valid: true},
    Metadata:  convomodel.ConversationMetadataSQL{Message: &convomodel.ConversationMetadata{EffectiveLanguage: lang}},
}
```

**For scorecards with empty `conversation_id`** (34 of 35 Brinks missing): These should be treated as process-like scorecards (`isProcessScorecard=true` or similar handling). No conversation lookup needed — just scorecard + scores from PG.

### 2a. Cost of finding missing scorecard IDs

Before we can backfill by ID, the monitor must identify which IDs are missing. The current sync monitor approach:
1. PG: fetch all `resource_id` from `director.scorecards` in time range → returns all IDs to the cron pod
2. CH: `SELECT COUNT(DISTINCT scorecard_id) FROM scorecard_d WHERE scorecard_id IN (?)` → returns a single count

To get the **exact missing IDs**, we'd need to change step 2 to return the set of existing IDs, then diff locally. Or:
- PG returns all IDs (already done)
- CH returns existing IDs: `SELECT DISTINCT scorecard_id FROM scorecard_d WHERE scorecard_id IN (?)`
- Diff in memory on the cron pod

**Validate**:
- Memory cost of holding all IDs in the cron pod for a 48h window across all customers. Rough estimate: largest customers have ~10K scorecards/day × 2 days = 20K UUIDs × 36 bytes ≈ 720KB per customer — should be fine.
- CH query cost of returning full ID lists vs just counts. The `IN` clause already sends all PG IDs to CH; returning matching IDs instead of a count adds network transfer but minimal query cost.

### 3. How to pass scorecard IDs from monitor to backfill?

**DECIDED: Direct Temporal workflow input from cron task.**

The sync monitor cron task detects missing IDs, then calls `temporalClient.ExecuteWorkflow()` directly with the ID list as workflow input. No InternalJobService, no API wrapper, no shared DB table.

| Approach | Pros | Cons | Verdict |
|----------|------|------|---------|
| **Temporal signal/child workflow** | Native data passing, audit trail | Monitor must be a Temporal workflow too | Overkill |
| **Direct from cron task** | Simplest, no extra infra, IDs passed as workflow input | Loses InternalJobService job tracking | **Chosen** |
| **Shared DB table** | Decoupled, persistent | Extra schema, cleanup needed | Overkill |
| **Cron env vars** | Simple | Size-limited, no persistence, log loss | Not viable |

Temporal workflow input supports up to 4MB payload (~100K UUIDs). For a 48h window, even the largest customers produce ~20K scorecards — well within limits.

**Precedent**: `sync-users/coach_builder_update_dynamic_audience_task.go:98` uses this exact pattern — cron task directly calls `temporalClient.ExecuteWorkflow()`.

### 4. Triggering a Temporal workflow

**DECIDED: Cron task triggers workflow directly via `temporalClient.ExecuteWorkflow()`.**

No InternalJobService needed. The cron task already has access to the Temporal client (injected via DI). Pattern is proven in the codebase:

| Pattern | Used by | InternalJobService? |
|---------|---------|---------------------|
| Cron → direct `ExecuteWorkflow()` | `coach_builder_update_dynamic_audience_task.go` | No |
| API handler → direct `ExecuteWorkflow()` | `CloseConversation`, `RunTestCases`, 10+ others | No |
| Cron → `InternalJobService.CreateJob()` → handler → workflow | `batch-reindex-conversations` | Yes |

The InternalJobService pattern adds: API dependency, ADMIN auth, job handler registration, job tracking. For an automated self-healing loop, these are overhead without value — the Temporal workflow itself provides audit trail and retry semantics.

### 5. Monitor cost at hourly frequency

The sync monitor queries:
1. PG: `SELECT resource_id FROM director.scorecards WHERE submitted_at >= ? AND submitted_at < ?` (returns all IDs)
2. CH: `SELECT COUNT(DISTINCT scorecard_id) FROM scorecard_d WHERE scorecard_id IN (?) AND scorecard_submit_time >= ?` (checks existence)

For hourly runs with a 24h lookback:
- PG query: light (indexed on `submitted_at`)
- CH query: `IN` clause with all IDs from PG — could be 100s-1000s of IDs per customer

**Validate**: What's the CH query cost with large `IN` clauses across all customers per cluster? Current monthly run takes seconds per customer — hourly should be fine, but confirm with production metrics.

### 6. What time range should the hourly monitor check?

**Decision: Rolling 48h window.**

Rationale: 24h is too tight — a scorecard submitted at 11pm might not be detected as missing until the next run. 48h gives a full extra day of buffer. Cost difference is minimal (2x IDs per customer, still well under memory/query limits).

**Remaining question**: How quickly after scorecard submission should the CH write succeed? If writes fail silently (like the Brinks case), when is the earliest we can reliably detect the gap? Check if there's a known CH write delay (likely seconds, but confirm).

### 7. Retry and escalation

If a scorecard can't be backfilled after N attempts:
- Should the system escalate (Slack alert)?
- What's the retry interval?
- Should it give up after a max number of retries?

**Validate**: Are there cases where a scorecard physically cannot be written to CH? (e.g., conversation deleted, shard unreachable, data corruption)

### 8. Existing Brinks gap: 35 remaining scorecards

**VALIDATED: Root cause identified.**

| Submit Date | Missing | Root Cause |
|-------------|---------|------------|
| Mar 13 | 24 | **Empty `conversation_id`** — standalone QA scorecards (external system, likely Verint) |
| Mar 23 | 1 | **Conv date mismatch** — conversation from Mar 13, submitted Mar 23 |
| Mar 27 | 10 | **Empty `conversation_id`** — standalone QA scorecards |

**34/35 have empty `conversation_id`** (not NULL, empty string). These are scorecards from an external QA system (process IDs like "VI36:...", "708666532", "CS Number: 761209129"). The `reindexconversations` orphan scorecard batch processed most days but **skipped Mar 13 and Mar 27** for unknown reasons.

**1/35 has a conversation from Mar 13 but was submitted Mar 23** — the Mar 23 backfill couldn't find it (conversation outside time range).

**This proves ID-based backfill is needed**: time-range backfill fundamentally cannot fix these because:
- Empty-conv-id scorecards depend on the orphan batch, which is unreliable
- Cross-date scorecards (submitted date != conversation date) fall through the cracks
- Both cases are trivially fixed by looking up the scorecard by ID directly

## Architecture (validated)

```
[Hourly Cron: scorecard-sync-monitor]
    │
    ├─ For each customer/profile (existing per-task loop):
    │
    │   Phase 1: Count comparison (cheap)
    │   1. PG: SELECT COUNT(*) FROM director.scorecards WHERE submitted_at in [now-48h, now]
    │   2. CH: SELECT COUNT(DISTINCT scorecard_id) FROM scorecard_d WHERE scorecard_submit_time in [now-48h, now]
    │   3. Emit metrics: total_submitted, ch_count, missing_count, missing_rate
    │   4. If counts match → done (skip expensive ID lookup)
    │
    │   Phase 2: Find missing IDs (only when counts mismatch)
    │   5. PG: SELECT resource_id FROM director.scorecards WHERE ...
    │   6. CH: SELECT DISTINCT scorecard_id FROM scorecard_d WHERE scorecard_id IN (?)
    │   7. Diff in memory → missing IDs
    │
    │   Phase 3: Triage missing scorecards
    │   8. PG: SELECT resource_id, conversation_id FROM director.scorecards WHERE resource_id IN (missing)
    │   9. Categorize:
    │       - conversation scorecards (has conversation_id) → isProcessScorecard=false
    │       - standalone scorecards (empty conversation_id) → isProcessScorecard=true
    │
    │   Phase 4: Trigger backfill (batched)
    │   10. Split missing IDs into batches (e.g., 500 per workflow)
    │   11. For each batch:
    │       temporalClient.ExecuteWorkflow("backfill-scorecards-by-id", {
    │           scorecardIDs, customerID, profileID
    │       })
    │           │
    │           ├─ PG: fetch scorecards by ID (director.scorecards)
    │           ├─ PG: fetch scores by scorecard_id (director.scores)
    │           ├─ PG: fetch conversation context (app.chats) — skip if empty conv_id
    │           ├─ PG: fetch agent user (users table) — for IsDevUser
    │           ├─ Build: BuildScoreRows(..., isProcessScorecard=emptyConvID)
    │           ├─ Build: BuildScorecardRows(...)
    │           └─ CH: BatchWriteScores + BatchWriteScorecards
    │
    └─ Alert on persistent failures (missing after N backfill attempts)
```

**Key design points**:
- **Count-first optimization**: Compare counts before fetching IDs. Most customers will match (0 missing), so the expensive ID lookup is skipped. This makes hourly runs cheap.
- **Triage**: Categorize missing scorecards by type (conversation vs standalone) before backfill. Different write paths for each.
- **Batching**: Split large ID lists into batches (e.g., 500 per workflow) to avoid Temporal payload limits and allow parallel processing. Normal case is <10 missing, but edge cases (like Brinks 436) need batching.
- **Metrics**: Emit gauges to Groundcover/Datadog for dashboarding (see item 9 below).
- Cron task triggers workflow directly (`temporalClient.ExecuteWorkflow`), no InternalJobService
- Rolling 48h monitor window

### 9. Metrics and dashboarding

**VALIDATED: Feasible via `shared-go/framework/stats`.**

The sync monitor currently emits **no metrics** — only logs and Slack. Adding metrics is straightforward:

**Library**: `github.com/cresta/shared-go/framework/stats`
**Backend**: Groundcover (Prometheus endpoint at `ds.groundcover.com/datasources/prometheus`) + Datadog
**Pattern**: Inject `stats.Client`, call `metric.WithTags(tags...).Gauge(ctx, value)`
**Precedent**: `knowledge-base-data-report` task emits 12+ metrics with the same pattern

**Proposed metrics**:

| Metric | Type | Description |
|--------|------|-------------|
| `cresta.scorecard_sync_monitor.total_submitted` | Gauge | PG scorecard count in 48h window |
| `cresta.scorecard_sync_monitor.ch_count` | Gauge | CH scorecard count in 48h window |
| `cresta.scorecard_sync_monitor.missing_count` | Gauge | Number of missing scorecards |
| `cresta.scorecard_sync_monitor.missing_rate_percent` | Gauge | Missing rate as percentage |
| `cresta.scorecard_sync_monitor.backfill_triggered` | Counter | Number of backfill workflows triggered |

**Tags**: `customer_id`, `profile_id`, `cluster`, `status` (OK/WARNING/CRITICAL)

**Dashboard queries** (Groundcover/Grafana):
```promql
# Missing rate over time per customer
cresta_scorecard_sync_monitor_missing_rate_percent{cluster="voice-prod"}

# Alert: any customer above 5%
cresta_scorecard_sync_monitor_missing_rate_percent > 5

# Total missing across cluster
sum(cresta_scorecard_sync_monitor_missing_count) by (cluster)
```

**Implementation**: Add `stats.Client` to factory, define metrics in `metrics.go`, emit in `Run()` after each check. See `knowledge-base-data-report/metrics.go` for reference pattern.

## Validated Summary

| Item | Status | Result |
|------|--------|--------|
| 1. Extend reindexscorecards? | **Validated** | New workflow recommended over extending |
| 2. ID-based backfill feasible? | **Validated** | Yes — minimal conversation data needed (3 fields) |
| 2a. Cost of finding missing IDs | **Validated** | ~720KB/customer for 48h window, acceptable. Count-first optimization skips ID lookup when counts match |
| 3. Pass IDs monitor→backfill | **Decided** | Direct Temporal workflow input from cron task |
| 4. Triggering workflow | **Decided** | Cron task calls `temporalClient.ExecuteWorkflow()` directly |
| 5. Monitor cost hourly | **Validated** | Count comparison is cheap; ID lookup only on mismatch. Seconds per customer |
| 6. Time window | **Decided** | Rolling 48h |
| 7. Retry/escalation | Open | Need to identify unfixable cases |
| 8. Brinks 35 remaining | **Validated** | 34 empty conv_id + 1 cross-date; proves ID-based approach |
| 9. Metrics/dashboarding | **Validated** | `shared-go/framework/stats` → Groundcover/Datadog. No metrics emitted today; easy to add |

## Next Steps

1. ~~Investigate the 35 remaining Brinks scorecards~~ — Done: 34 empty conv_id, 1 cross-date
2. ~~Read CH write path to confirm ID-based backfill feasible~~ — Done: confirmed, minimal conv data needed
3. ~~Decide: extend reindexscorecards vs new workflow~~ — New workflow recommended
4. ~~Decide triggering mechanism~~ — Cron task direct `ExecuteWorkflow()`
5. Design the new workflow proto and activity (accepts scorecard IDs, handles both conv-linked and standalone)
6. Add metrics to sync monitor (`stats.Client` + Groundcover dashboard)
7. Implement count-first optimization + triage + batching in monitor
8. Prototype with the 35 Brinks scorecards as test case
9. Wire up monitor → backfill → verify loop
