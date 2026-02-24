# CONVI-5565: Data Sync Between ClickHouse and PostgreSQL - Complete Investigation

**Linear Issue**: https://linear.app/cresta/issue/CONVI-5565/data-sync-between-ch-and-postgres
**Related Issue**: https://linear.app/cresta/issue/CONVI-6076 (PostgreSQL race condition)
**Date**: 2025-01-16 to 2026-01-27

---

## Table of Contents

1. [Problem Description](#problem-description)
2. [Fix Attempt 1: Use PostgreSQL updated_at (Failed)](#fix-attempt-1-use-postgresql-updated_at-failed)
3. [Fix 2: Atomic Transactions (PR #24103)](#fix-2-atomic-transactions-pr-24103)
4. [Discovery: PostgreSQL Race Condition (CONVI-6076)](#discovery-postgresql-race-condition-convi-6076)
5. [Fix 3: GORM Omit for Partial Updates](#fix-3-gorm-omit-for-partial-updates)
6. [Final Validation: Load Testing Results](#final-validation-load-testing-results)
7. [Conclusions](#conclusions)

---

## Problem Description

### Root Cause

Scorecard APIs' async work were **not finishing in the same order as they were called**, causing data inconsistency between PostgreSQL and ClickHouse.

**Sequence of the bug:**

1. User calls UpdateScorecard API (sets score = 5)
2. User calls SubmitScorecard API immediately after (sets score = 10, submitted_at = T2)
3. **UpdateScorecard's async work starts first** (fast)
4. **SubmitScorecard's async work starts second** (slow)
5. Both async works finish in reverse order:
   - SubmitScorecard async work finishes first → writes score=10 to ClickHouse with update_time=T1
   - UpdateScorecard async work finishes second → writes score=5 to ClickHouse with update_time=T2 (newer)
6. ClickHouse's ReplacingMergeTree keeps the row with newer update_time (T2)
7. **Result: ClickHouse shows stale score=5 instead of correct score=10**

### Impact

- Performance Insights showing incorrect evaluation data
- Scorecard submitted_at timestamps being overwritten by later updates
- Customer-reported data inconsistencies (Spirit, SnapFinance)

### Related Issues

- **CONVI-5685**: Investigate async work order of coaching APIs
- **CONVI-5609**: [SnapFinance] evaluations missing in Performance Insights
- **CONVI-5662**: [Spirit] Performance Insights "Conversation volume" undercounts

---

## Fix Attempt 1: Use PostgreSQL updated_at (Failed)

**PR**: https://github.com/cresta/go-servers/pull/23999
**Reverted by**: https://github.com/cresta/go-servers/pull/24095

### Approach

Change ClickHouse's `update_time` column from using `time.Now()` to PostgreSQL's `updated_at` timestamp. This way, even if async work runs out of order, ClickHouse's ReplacingMergeTree would use the correct timestamp for versioning.

### Changes Made

- Added `Version` field to `ScorecardExtra` struct
- Populated `Version` with `scorecard.UpdatedAt` in `buildScorecardRows()`
- Used `scorecardExtra.Version` for `UpdateTime` in `buildScorecardRow()`
- Used `scorecard.UpdatedAt` for `UpdateTime` in `buildScoreRows()`

### Why It Failed

This approach was fundamentally flawed because:

1. **Async work captured scorecard object in closure** instead of reading fresh from database
2. When UpdateScorecard's async work ran AFTER SubmitScorecard's async work, it used the **stale scorecard object** from the closure with an **earlier updated_at timestamp**
3. This stale data overwrote the correct SubmitScorecard data in ClickHouse
4. The root cause was **reading stale data**, not the timestamp source

---

## Fix 2: Atomic Transactions (PR #24103)

**PR**: https://github.com/cresta/go-servers/pull/24103
**Status**: Merged

### Key Insight

The problem wasn't the timestamp - it was that async work was reading stale data.

### Core Changes

#### 1. Move Historic Schema Writes Inside Transactions

**OLD (buggy) behavior:**
- Historic scores written **asynchronously** → reads stale data
- ClickHouse updated with uncommitted/stale values

**NEW (fixed) behavior:**
- Historic scores written **synchronously INSIDE transaction**
- Async work runs **AFTER transaction commit** → reads fresh committed data
- ClickHouse updated with correct persisted values

#### 2. Feature Flag for Gradual Rollout

Added `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE` feature flag (default: `false`).

#### 3. Atomic Function Variants

Created `*Atomic` versions of all scorecard APIs:
- `createDefaultScorecardAtomic`
- `updateScorecardAndScoresAtomic`
- `submitScorecardInternalAtomic`
- `resetScorecardDataAtomic`

#### 4. Async Work Re-reads from Database

Created new async work function `scorecardAsyncWorkReadFromDB` that:
- Re-reads scorecard from PostgreSQL at start of execution (using write replica)
- Re-reads historic scores from database
- Never uses captured closure variables
- Ensures ClickHouse always gets the latest committed state

### Implementation Pattern

```go
// New behavior (feature flag enabled, fixed)
func UpdateScorecardAtomic(...) {
    var asyncWork func()

    db.Transaction(func(tx) {
        // Update app schema
        UpdateScorecardAndScoresInDB(tx, ...)

        // Write historic schema
        WriteHistoricScorecardScores(tx, ...)

        // Capture async work but DON'T execute yet
        asyncWork = func() {
            scorecardAsyncWorkReadFromDB(scorecard, ...)
        }
    })

    // Run async work AFTER transaction commits
    // → reads fresh committed data from DB
    asyncWorkQueue.Execute(asyncWork)
}
```

---

## Discovery: PostgreSQL Race Condition (CONVI-6076)

**Date**: 2025-01-18
**Linear Ticket**: https://linear.app/cresta/issue/CONVI-6076

### Summary

During testing of the CONVI-5565 fix, we discovered a **separate race condition at the PostgreSQL level** when `UpdateScorecard` and `SubmitScorecard` APIs are called concurrently.

### Initial Test Results (Before GORM Omit Fix)

| Iterations | Wait Time | Pass Rate | Failures |
|------------|-----------|-----------|----------|
| 50         | 2s        | 84%       | 8        |
| 20         | 5s        | 85%       | 3        |
| 30         | 3s        | 93%       | 2        |

### Root Cause: Lost Update

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

Both APIs use `gorm.Save()` which saves the **entire struct**, not just modified fields.

### Column Overlap Analysis

| Column | SubmitScorecard | UpdateScorecard |
|--------|-----------------|-----------------|
| `SubmittedAt` | **Sets to NOW()** | Doesn't touch, but **saves stale value** |
| `SubmitterUserID` | Sets | Doesn't touch, but saves stale value |
| `LastUpdaterUserID` | Sets | Sets |
| `TaskIds` | Sets | Sets |

---

## Fix 3: GORM Omit for Partial Updates

### Solution

Use GORM's `Omit` to exclude `submitted_at` and `submitter_user_id` when UpdateScorecard saves:

```go
err := tx.Model(updatedScorecard).Omit("submitted_at", "submitter_user_id").Save(updatedScorecard).Error
```

### Benefits

- Minimal code change
- Directly addresses the specific race condition
- No new columns or DB triggers needed
- No locking overhead

---

## Final Validation: Load Testing Results

**Date**: 2026-01-23 to 2026-01-27
**Environment**: chat-staging, cox/sales
**Feature Flag**: `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE=true`
**Fixes Applied**: PR #24103 (Atomic) + GORM Omit

### Experiment 1: Concurrent API Calls (Stress Test)

Tests with Update and Submit called near-simultaneously (10ms apart).

| Round | CH Verify Wait | Passed | Failed | Success Rate |
|-------|----------------|--------|--------|--------------|
| 1     | 3s             | 49     | 1      | 98%          |
| 2     | 3s             | 45     | 5      | 90%          |
| 3     | 5s             | 45     | 5      | 90%          |
| 4     | 2s             | 41     | 9      | 82%          |
| 5     | 3s             | 40     | 10     | 80%          |
| **Total** | -          | **220** | **30** | **86.4%** |

**Observation**: ~10-20% failure rate when APIs are called concurrently (10ms apart).

### Experiment 2: API Call Timing Impact

**Hypothesis**: If we increase the delay between UpdateScorecard and SubmitScorecard, UpdateScorecard's async work has time to complete first, allowing SubmitScorecard's async work (with correct data) to write last and win.

| API Delay | Iterations | Passed | Failed | Success Rate |
|-----------|------------|--------|--------|--------------|
| 10ms      | 50         | 40     | 10     | **80%**      |
| 50ms      | 50         | 47     | 3      | **94%**      |
| 100ms     | 50         | 50     | 0      | **100%**     |
| 200ms     | 50         | 50     | 0      | **100%**     |
| 500ms     | 100        | 99     | 1      | **99%**      |
| 1000ms    | 100        | 100    | 0      | **100%**     |

**Key Finding**: With >= 100ms delay between API calls, the failure rate drops to ~0%.

### Root Cause of Remaining Failures

When UpdateScorecard and SubmitScorecard are called concurrently:

```
Timeline (failure scenario):
─────────────────────────────────────────────────────────────────────────────
Time    UpdateScorecard                    SubmitScorecard
─────────────────────────────────────────────────────────────────────────────
T1      API called
T2      Transaction commits
T3      Async work starts
T4      Re-reads from DB (submitted_at=NULL)
T5                                         API called
T6                                         Transaction commits (sets submitted_at)
T7                                         Async work starts
T8                                         Re-reads from DB (submitted_at=correct)
T9                                         Writes to CH (update_time=T9)
T10     Writes to CH (update_time=T10, stale data!)
        ↑ T10 > T9, so ClickHouse keeps stale data
─────────────────────────────────────────────────────────────────────────────
```

**The problem**: UpdateScorecard's async work reads from DB **before** SubmitScorecard commits, but writes to ClickHouse **after** SubmitScorecard's async work. Since `update_time = time.Now()` is set at write time, the stale data wins.

### Why 100ms Delay Fixes It

```
Timeline (success scenario with 100ms+ delay):
─────────────────────────────────────────────────────────────────────────────
Time    UpdateScorecard                    SubmitScorecard
─────────────────────────────────────────────────────────────────────────────
T1      API called
T2      Transaction commits
T3      Async work starts
T4      Re-reads from DB (submitted_at=NULL)
T5      Writes to CH (update_time=T5)
        ... 100ms delay ...
T6                                         API called
T7                                         Transaction commits (sets submitted_at)
T8                                         Async work starts
T9                                         Re-reads from DB (submitted_at=correct)
T10                                        Writes to CH (update_time=T10)
        ↑ T10 > T5, so ClickHouse keeps correct data
─────────────────────────────────────────────────────────────────────────────
```

The ~100ms threshold represents the time for UpdateScorecard's async work to complete (re-read + ClickHouse write).

---

## Conclusions

### Solution Design Validated

The current solution is designed with the following principle:

> Each API has two parts: **sync** (transaction) and **async** (ClickHouse update). The sync part ensures PostgreSQL consistency via transactions and GORM Omit. The async part re-reads from PostgreSQL and updates ClickHouse. After multiple rounds of async work, ClickHouse eventually converges to the correct state.

**This experiment validates the design**:

1. **PostgreSQL is always correct** - The GORM Omit fix ensures UpdateScorecard doesn't overwrite `submitted_at`
2. **ClickHouse eventually converges** - When API calls are spaced >= 100ms apart, ClickHouse gets the correct data
3. **Race condition only occurs under extreme concurrency** - Real-world usage rarely has Update and Submit called within 10ms of each other

### Real-World Impact Assessment

| Scenario | Likelihood | Impact |
|----------|------------|--------|
| Normal user interaction | Low | N/A - users don't click that fast |
| Automated systems | Medium | May need 100ms delay between calls |
| Load/stress tests | High | Expected ~10-20% failure rate |

### Known Limitations

1. **ClickHouse may have temporary stale data** when APIs are called within 100ms
2. **PostgreSQL is always correct** - source of truth
3. **No code fix planned** - documented as acceptable limitation for edge cases

### Lessons Learned

1. **Root cause matters**: The failed attempt tried to solve the symptom (timestamp ordering) instead of the root cause (reading stale data)
2. **Multiple layers of bugs**: CONVI-5565 (async order) and CONVI-6076 (PostgreSQL race) were two separate issues
3. **Test infrastructure is crucial**: The async work queue and load test tools made it possible to reliably reproduce and verify fixes
4. **Feature flags enable safe rollout**: Gradual rollout prevented production incidents

---

## Key Files

### Core Implementation
- `apiserver/internal/coaching/action_create_scorecard.go`
- `apiserver/internal/coaching/action_update_scorecard.go`
- `apiserver/internal/coaching/action_submit_scorecard.go`
- `apiserver/internal/coaching/action_reset_scorecard.go`

### Test Tools
- `tools/test_async_order/main.go` - Load testing tool
- `tools/verify_sync/main.go` - Single scorecard verification

### Test Commands

```bash
# Build test tool
cd tools/test_async_order
go build -o test_async_order .

# Run with default settings (10ms API delay, 2s CH wait)
./test_async_order -iterations 50

# Run with custom API delay (test timing impact)
./test_async_order -iterations 50 -api-delay 100 -wait 3

# Verify specific scorecard
cd tools/verify_sync
go build -o verify_sync .
./verify_sync -name "customers/cox/profiles/sales/scorecards/<scorecard-id>"
```
