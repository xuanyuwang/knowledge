# Daily Engineering Notes – 2026-01-19

Today I want to talk about a problem that is common and rare at the same time: race condition of touching the same record from different APIs.

The reasons that it's interesting is that:
1. I discovered it by a load testing script: simply call different APIs quickly (create -> update -> submit) and verify the results. It's very simple but powerful
2. It's an issue I concerned before but in a different perspective: what if many coaches are touching the same scorecard? Right now, triggering the bug is simpler than I thought: simply update and then submit.

The following is a full doc.

# PostgreSQL Race Condition Finding

## Date
2025-01-18

## Summary
During testing of the CONVI-5565 fix for async ClickHouse write ordering, we discovered a **separate race condition at the PostgreSQL level** when `UpdateScorecard` and `SubmitScorecard` APIs are called concurrently.

## Test Setup
- Environment: chat-staging
- Feature flag `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE` enabled
- Test script rapidly calls: Create → Update → Submit (concurrent) → Verify

## Observed Behavior

### Test Results
- 50 iterations with 2s wait: **84% pass rate** (8 failures)
- 20 iterations with 5s wait: **85% pass rate** (3 failures)
- 30 iterations with 3s wait: **93% pass rate** (2 failures)

Failure rate is consistent (~7-15%) regardless of wait time, indicating this is NOT a timing issue with async work.

### Failed Scorecard Example
**Scorecard ID:** `019bd2e8-5c71-70e9-9a89-acb3efd08e34`

**SubmitScorecard API Response:**
- Returned SUCCESS
- Response included non-null `SubmitTime`

**PostgreSQL State (after both APIs completed):**
```
Submitted At: NULL (not submitted)
```

**ClickHouse State:**
```
Submit Time: 1970-01-01 (default/empty)
```

## Root Cause Analysis

This is a classic **"lost update" race condition**:

```
Timeline:
─────────────────────────────────────────────────────────────────
Time    UpdateScorecard                  SubmitScorecard
─────────────────────────────────────────────────────────────────
T1      Read scorecard (submitted_at=NULL)
T2                                       Read scorecard (submitted_at=NULL)
T3                                       Set submitted_at = NOW()
T4                                       Save to DB (committed)
T5      Modify scores
T6      Save to DB (committed)
        ↑ Overwrites submitted_at back to NULL!
─────────────────────────────────────────────────────────────────
```

The `UpdateScorecard` API reads the scorecard BEFORE `SubmitScorecard` commits its changes. When `UpdateScorecard` saves, it overwrites the `submitted_at` field with the stale value (NULL).

Both APIs use `gorm.Save()` which saves the **entire struct**, not just modified fields.

## Column Analysis

### SubmitScorecard modifies:
- `LastUpdaterUserID` → submitter
- `SubmittedAt` → `NOW()`
- `SubmitterUserID` → submitter
- `TaskIds` → from request

### UpdateScorecard modifies:
- `TaskIds`
- `MessageIds`
- `CreatorUserID` (only if not already set)
- `ManuallyScored` → `true`
- `Comment`
- `SubmissionSource` (only if not submitted)
- `LastUpdaterUserID`
- For non-conversation templates: `ProcessInteractionAt`, `ProcessID`, `AgentUserID`

### Overlap Analysis

| Column | SubmitScorecard | UpdateScorecard |
|--------|-----------------|-----------------|
| `SubmittedAt` | **Sets to NOW()** | Doesn't touch, but **saves stale value** |
| `SubmitterUserID` | Sets | Doesn't touch, but saves stale value |
| `LastUpdaterUserID` | Sets | Sets |
| `TaskIds` | Sets | Sets |

**Key finding**: `SubmittedAt` and `SubmitterUserID` are ONLY set by SubmitScorecard, but UpdateScorecard overwrites them with stale values because `gorm.Save()` writes all columns.

## Impact

1. **Data inconsistency**: Scorecard appears submitted in API response but is not actually submitted in database
2. **ClickHouse sync**: Since PostgreSQL has NULL submitted_at, ClickHouse also has empty submit time
3. **User experience**: Users may see scorecard as "submitted" momentarily, then it reverts to "draft"

## Difference from CONVI-5565

| Issue | CONVI-5565 (Fixed) | CONVI-6076 (New) |
|-------|-------------------|---------------------|
| Layer | Async ClickHouse writes | PostgreSQL transactions |
| Cause | Async work execution order | Concurrent API calls overwriting |
| Fix | Synchronous CH writes in transaction | GORM field exclusion |

**Linear Ticket**: [CONVI-6076](https://linear.app/cresta/issue/CONVI-6076)

## Reproduction

Use the test script at `/tools/test_async_order/`:

```bash
# Build
go build -o .tmp/CONVI-5565-test/test_async_order ./tools/test_async_order/

# Run (failed scorecards are preserved for investigation)
.tmp/CONVI-5565-test/test_async_order -iterations 50 -wait 3
```

## Recommended Fix: GORM Field Exclusion (Option 6)

Since `SubmittedAt` and `SubmitterUserID` should **never** be modified by UpdateScorecard, use GORM's `Omit` to exclude these fields when saving:

```go
// In UpdateScorecardAndScoresInDB or a new variant for UpdateScorecard
err := tx.Model(updatedScorecard).Omit("submitted_at", "submitter_user_id").Save(updatedScorecard).Error
```

Or use `Select` to explicitly specify which fields to update:
```go
err := tx.Model(updatedScorecard).Select(
    "task_ids", "message_ids", "creator_user_id", "manually_scored",
    "comment", "submission_source", "last_updater_user_id",
    "process_interaction_at", "process_id", "agent_user_id", "score", "updated_at",
).Updates(updatedScorecard).Error
```

**Benefits**:
- Minimal code change
- Directly addresses the specific race condition
- No new columns or DB triggers needed
- No locking overhead

**Considerations**:
- Need to maintain the field list if new fields are added
- May need separate save paths for UpdateScorecard vs SubmitScorecard

## Other Options Considered (Not Recommended)

### Option 1: Optimistic Locking
Add a version column to scorecards table and check version on update.
**Rejected**: Requires new column, schema migration.

### Option 2: Row-Level Locking
Use `SELECT ... FOR UPDATE` when reading scorecard.
**Rejected**: Risk of deadlocks, latency spikes, easy to overuse.

### Option 3: Partial Updates
Change UpdateScorecard to only update specific fields.
**Rejected**: Overlap concern with TaskIds column.

## Files Referenced

- Test script: `/Users/xuanyu.wang/repos/go-servers/tools/test_async_order/main.go`
- Verify tool: `/Users/xuanyu.wang/repos/go-servers/tools/verify_sync/main.go`
- Failed scorecards preserved in chat-staging for investigation

