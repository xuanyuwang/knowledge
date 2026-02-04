# Investigation: Scorecard Missing from Historic Schema

## DB connections

rentokil: `cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod rentokil-us-east-1`
spirit: `cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod spirit-us-east-1`
guitar-center: `cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod guitar-center-us-east-1`
nclh: `cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod nclh-us-east-1`

Clickhouse: `clickhouse://admin:ItVIZdiPT8XQmD5Yox16ROpdNcjJYEEx@clickhouse-conversations.us-east-1-prod.internal.cresta.ai:9440?statusColor=F8F8F8&env=production&name=conv-us-east-1-prod&tLSMode=2&usePrivateKey=false&safeModeLevel=0&advancedSafeModeLevel=0&driverVersion=0&lazyload=false`
All customers are on the same clickhouse cluster.

## Problem Statement

Scorecards exist in `director.scorecards` with valid scores in `director.scores`, but are completely missing from `historic.scorecard_scores` (and consequently from ClickHouse).

### Scale of Impact

**Cluster**: us-east-1-prod
**Time Range**: January 2026

| Customer | Profile | Total Submitted | Missing | % Missing |
|----------|---------|-----------------|---------|-----------|
| **rentokil** | us-east-1 | 3,637 | 1,165 | **32.0%** |
| **spirit** | us-east-1 | 6,395 | 1,868 | **29.2%** |
| **guitar-center** | us-east-1 | 699 | 285 | **40.8%** |
| **nclh** | us-east-1 | 104 | 54 | **51.9%** |

**Total Impact (4 customers sampled)**: ~3,370+ missing scorecards in January alone.

### First Missing Scorecard by Customer

| Customer | First Missing Created | Notes |
|----------|----------------------|-------|
| **rentokil** | Dec 31, 2025 15:30 | First occurrence |
| **spirit** | Jan 8, 2026 22:37 | |
| **guitar-center** | Jan 10, 2026 07:39 | (1 old one-off from Jan 2025) |
| **nclh** | Jan 11, 2026 17:15 | (2 old one-offs from Jul 2025) |

### Monthly Breakdown (All 4 Customers Show Same Pattern)

| Month | rentokil | spirit | guitar-center | nclh |
|-------|----------|--------|---------------|------|
| Jul-Nov 2025 | 0% | 0% | 0% | 0% |
| Dec 2025 | 0.2% (7) | 0% | 0% | 0% |
| **Jan 2026** | **32%** | **29%** | **41%** | **52%** |

### Missing Scorecards by Date (Last 2 Weeks)

| Date | Total | Missing | % Missing |
|------|-------|---------|-----------|
| 2026-01-28 | 26 | 23 | 88% |
| 2026-01-27 | 81 | 77 | 95% |
| 2026-01-26 | 102 | 98 | 96% |
| 2026-01-25 | 24 | 23 | 96% |
| 2026-01-24 | 25 | 22 | 88% |
| 2026-01-23 | 95 | 92 | 97% |
| 2026-01-22 | 119 | 116 | 97% |
| 2026-01-21 | 159 | 141 | 89% |
| 2026-01-20 | 216 | 104 | 48% |
| 2026-01-19 | 62 | 30 | 48% |
| 2026-01-18 | 34 | 13 | 38% |
| 2026-01-17 | 49 | 22 | 45% |
| 2026-01-16 | 128 | 38 | 30% |
| 2026-01-15 | 129 | 26 | 20% |

**Key Observation**: Missing rate jumped from ~20-30% to **90%+** around January 21, 2026.

### Historical Timeline (6 Months)

| Month | Total | Missing | % Missing |
|-------|-------|---------|-----------|
| Aug 2025 | 6 | 0 | 0.0% |
| Sep 2025 | 67 | 0 | 0.0% |
| Oct 2025 | 173 | 0 | 0.0% |
| Nov 2025 | 3,367 | 0 | 0.0% |
| Dec 2025 | 3,984 | 7 | 0.2% |
| **Jan 2026** | **3,637** | **1,165** | **32.0%** |

### Detailed Timeline (Dec-Jan Transition)

| Date | Total | Missing | Notes |
|------|-------|---------|-------|
| Dec 15-30 | ~1,200 | 0 | All healthy |
| **Dec 31** | 79 | **7** | **First occurrence** |
| Jan 1 | 32 | 1 | |
| Jan 2-14 | ~2,000 | ~300 | 10-33% missing |
| **Jan 21+** | ~500 | ~450 | **90%+ missing** |

### First Missing Scorecards (Dec 31)

The 7 scorecards that first went missing were **created** Dec 31 but **submitted** Jan 14-28:

| Created | Submitted |
|---------|-----------|
| Dec 31 15:30 | Jan 23 |
| Dec 31 15:34 | Jan 27 |
| Dec 31 15:48 | Jan 14 |
| Dec 31 17:46 | Jan 28 |
| Dec 31 19:02 | Jan 14 |
| Dec 31 21:04 | Jan 27 |
| Dec 31 22:00 | Jan 21 |

**Key Insight**: The historic write likely happens at **submit time**, not create time. The issue correlates with submission date, not creation date.

## Unsubmitted Scorecards Analysis (All 4 Customers)

### Months with Issues

| Customer | Months with Issues |
|----------|-------------------|
| **rentokil** | Sep 2025 (0.3%), **Jan 2026 (1.1%)** |
| **spirit** | Jul-Oct 2025 (~0.2-0.3%), **Nov 2025 (4.7%)**, **Dec 2025 (3.7%)**, **Jan 2026 (12%)** |
| **guitar-center** | None |
| **nclh** | None |

### January 2026 Summary

| Customer | Total | Missing | % Missing |
|----------|-------|---------|-----------|
| **rentokil** | 2,555,891 | 27,373 | 1.1% |
| **spirit** | 2,644,078 | 317,561 | **12.0%** |
| **guitar-center** | 693,771 | 3 | 0.0% |
| **nclh** | 674,707 | 0 | 0.0% |

### rentokil: One-Time Batch Job Failure

All missing unsubmitted scorecards (27,344) were created in **ONE hour**:

| Hour (UTC) | Total | Missing |
|------------|-------|---------|
| Jan 23 02:00 | 520 | 0 |
| **Jan 23 03:00** | **27,580** | **27,344 (99%)** |
| Jan 23 04:00 | 99 | 0 |

**Affected Templates**:
- "Sonya's Test - Process Template CSAT" - 22,977 scorecards
- "RTX Customer Care Scorecard 2026" - 4,369 scorecards

### spirit: Chronic Batch Job Issue (Worsening Over Time)

Spirit has had **ongoing issues since at least July 2025**, progressively worsening:

| Month | Total | Missing | % Missing |
|-------|-------|---------|-----------|
| Jul 2025 | 74,097 | 208 | 0.3% |
| Aug 2025 | 934,526 | 568 | 0.1% |
| Sep 2025 | 1,028,204 | 3,002 | 0.3% |
| Oct 2025 | 1,582,092 | 3,018 | 0.2% |
| **Nov 2025** | 1,336,711 | **62,516** | **4.7%** |
| **Dec 2025** | 1,378,896 | **50,777** | **3.7%** |
| **Jan 2026** | 2,644,078 | **317,561** | **12.0%** |

#### January 2026: 14:00 UTC Batch Job Spike (FIXED Jan 17)

The daily batch job at **14:00 UTC** was failing ~50% in early January:

| Date | Total (14:00 UTC) | Missing | % |
|------|-------------------|---------|---|
| Jan 10 | 60,669 | 32,432 | **53.5%** |
| Jan 11 | 41,416 | 21,660 | **52.3%** |
| Jan 13 | 59,862 | 30,092 | **50.3%** |
| Jan 14 | 60,483 | 30,910 | **51.1%** |
| Jan 15 | 60,527 | 30,749 | **50.8%** |
| Jan 16 | 63,469 | 33,469 | **52.7%** |
| **Jan 17** | 60,787 | 54 | **0.1%** (FIXED) |
| Jan 18+ | ~50k/day | ~70/day | ~0.1% |

**Key Insight**: The 14:00 UTC batch job was fixed on Jan 17, but Spirit has had a **chronic underlying issue** since mid-2025 that has been progressively worsening.

### guitar-center & nclh: No Significant Issues

Both customers show essentially **0% missing** for unsubmitted scorecards.

### Key Differences by Customer

| Customer | Pattern | Status |
|----------|---------|--------|
| **rentokil** | One-time batch failure (Jan 23 03:00) | Single incident |
| **spirit** | Chronic issue since Jul 2025, worsening over time; 14:00 UTC batch job spike fixed Jan 17 | **Chronic issue** |
| **guitar-center** | No issues | Healthy |
| **nclh** | No issues | Healthy |

### Example Affected Scorecard

**Scorecard ID**: `019bcaf4-4b65-7838-b07d-20640fe7628e`
- Created: 2026-01-17
- Submitted: 2026-01-23
- Conversation exists: Yes (ended 2026-01-16)
- Scores in director: 35
- Records in historic: **0**

### Verification Queries

```sql
-- Connection:
open $(cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod rentokil-us-east-1)

-- Scorecard exists in director schema (source of truth)
SELECT * FROM director.scorecards
WHERE customer = 'rentokil' AND profile = 'us-east-1'
AND resource_id = '019bcaf4-4b65-7838-b07d-20640fe7628e';
-- Returns: 1 row

-- Scores exist in director schema
SELECT * FROM director.scores
WHERE customer = 'rentokil' AND profile = 'us-east-1'
AND scorecard_id = '019bcaf4-4b65-7838-b07d-20640fe7628e';
-- Returns: rows with valid scores

-- MISSING from historic schema
SELECT * FROM historic.scorecard_scores
WHERE customer_id = 'rentokil' AND profile_id = 'us-east-1'
AND scorecard_id = '019bcaf4-4b65-7838-b07d-20640fe7628e';
-- Returns: 0 rows
```

## Feature Flag Status

### `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE`

| Environment | Status | Historic Write Behavior |
|-------------|--------|-------------------------|
| **us-east-1-prod** | **ENABLED (Jan 29, 2026)** | Synchronous in transaction |
| us-west-2-staging | Enabled | Synchronous in transaction |
| voice-staging | Enabled | Synchronous in transaction |
| chat-staging | Enabled | Synchronous in transaction |

**Definition**: `apiserver/internal/coaching/constants.go:23`
**Production Enablement**: Commit `2f406fb6fc7` on Jan 29, 2026 15:38:38 (PR #253866)

## Root Cause Analysis

### Two Code Paths Based on Feature Flag

#### 1. With Flag DISABLED (Production - `us-east-1-prod`)

**CreateScorecard path**: `action_create_scorecard.go:563-627` (`createDefaultScorecard`)
```go
func (s *ServiceImpl) createDefaultScorecard(...) {
    // Step 1: Create scorecard in director schema
    err = scoring.CreateScorecardAndScoresInDB(db, newScorecard, dbScores, ...)

    // Step 2: Historic write happens ASYNCHRONOUSLY
    asyncWork := s.asyncScorecardWork(...)  // line 598
    extrawork.RunWithTimeout(ctx, ..., asyncWork, ...)  // line 607
}
```

**Key Issue**: Historic write happens in `asyncScorecardWork()` which runs asynchronously and can fail silently.

#### 2. With Flag ENABLED (Staging)

**CreateScorecard path**: `action_create_scorecard.go:631-711` (`createDefaultScorecardAtomic`)
```go
func (s *ServiceImpl) createDefaultScorecardAtomic(...) {
    err = transaction.RunTransaction(ctx, db, func(tx *gorm.DB) error {
        // Step 1: Create scorecard in director schema
        err := scoring.CreateScorecardAndScoresInDB(tx, ...)

        // Step 2: Historic write happens SYNCHRONOUSLY in same transaction
        _, err = scorecardutils.WriteHistoricScorecardScores(tx, ...)  // line 668
        return nil
    })
}
```

**Key Benefit**: If historic write fails, the entire transaction rolls back - no orphaned records.

### Async Work Logic (`asyncScorecardWork` at line 877)

```go
func (s *ServiceImpl) asyncScorecardWork(...) func(context.Context) error {
    return func(ctx context.Context) (err error) {
        // Skip if calibration scorecard
        if isCalibrationScorecardInDB(scorecard) {
            return nil  // Early return - no historic write!
        }
        // Skip if not original scorecard (e.g., appeal, group calibration response)
        if !isOriginalScorecardInDB(scorecard) {
            return nil  // Early return - no historic write!
        }

        // ... (fetch conversation, determine voicemail status)

        // The actual historic write
        historicScorecardScores, err = scoring.UpdateHistoricScorecardScores(
            db, dbScores, scorecard, ..., templateStructure, ...)
        if err != nil {
            s.logger.Errorf(asyncCtx, "failed to update historic.scorecard_scores: %v", err)
            return err  // Error is logged but may be lost
        }

        // ClickHouse write
        scoreRows, err := scoring.UpdateScoresInClickHouse(...)
    }
}
```

### Possible Reasons for Missing Historic Record

| Condition | Code Location | Explanation |
|-----------|--------------|-------------|
| Calibration scorecard | Line 886-888 | `CalibratedScorecardID.Valid = true` → skip |
| Non-original scorecard type | Line 889-891 | Appeal, group calibration response → skip |
| Async work timeout | `extrawork.ContextWithNewTimeout` | Default timeout exceeded |
| DB error during historic write | Line 952-957 | Error logged but work continues |
| Conversation not found | Line 907-913 | Async work fails early |
| Context cancelled | Various | Request cancelled before async completes |
| Race condition | Various | Multiple concurrent operations |

## Key Files

| File | Purpose |
|------|---------|
| `apiserver/internal/coaching/constants.go:23` | Feature flag definition |
| `apiserver/internal/coaching/action_create_scorecard.go:563` | `createDefaultScorecard` - legacy async path |
| `apiserver/internal/coaching/action_create_scorecard.go:631` | `createDefaultScorecardAtomic` - fixed sync path |
| `apiserver/internal/coaching/action_create_scorecard.go:877` | `asyncScorecardWork` - async historic write logic |
| `apiserver/internal/coaching/action_submit_scorecard.go:1074` | `AsyncExtraWorkForSubmitScorecard` |
| `shared/scoring/scorecard_scores_dao.go:348` | `UpdateHistoricScorecardScores` |

## Investigation Steps

### 1. Check Scorecard Properties

```sql
-- Check if it's a calibration scorecard (would be skipped)
SELECT calibrated_scorecard_id, scorecard_type
FROM director.scorecards
WHERE customer = 'rentokil' AND profile = 'us-east-1'
AND resource_id = '019bcaf4-4b65-7838-b07d-20640fe7628e';
```

- If `calibrated_scorecard_id` is NOT NULL → calibration scorecard, skipped by design
- If `scorecard_type` is NOT 0/NULL (UNSPECIFIED) → non-original, skipped by design

### 2. Check Conversation Exists

```sql
-- Get conversation_id from scorecard
SELECT conversation_id FROM director.scorecards
WHERE customer = 'rentokil' AND profile = 'us-east-1'
AND resource_id = '019bcaf4-4b65-7838-b07d-20640fe7628e';

-- Check if conversation exists
SELECT * FROM app.chats
WHERE customer_id = 'rentokil' AND profile_id = 'us-east-1'
AND conversation_id = '<conversation_id_from_above>';
```

### 3. Check for Errors in Logs

Search for errors around the time the scorecard was created:
- `"failed to update the historic.scorecard_scores"`
- `"failed to find conversation"`
- `"failed to determine whether the conversation is voice mail"`

## Deployment Timeline Analysis

### Apiserver Prod Releases (flux-deployments)

| Date | Image Tag | Commit | Notes |
|------|-----------|--------|-------|
| Dec 17, 2025 | main-20251217_201515z-49d58a85 | 49d58a85 | Last release before issue |
| **Jan 8, 2026** | main-20260108_153010z-c86fe8e9 | c86fe8e9 | First release after 3-week gap |
| Jan 14, 2026 | main-20260114_003446z-5ab66507 | 5ab66507 | Includes fix commit |

### Key Commit: CONVI-5565 Fix (a8f67df325)

**Commit**: `a8f67df3251ed760e9a818caf59cb323587a73ba`
**Author**: Xuanyu Wang
**Date**: Jan 8, 2026 21:48 EST
**Title**: "fix historic schema race condition by moving writes to synchronous execution (Part 2)"

This commit was designed to fix exactly this issue by:
- Moving historic schema writes from async to synchronous execution
- Gating the fix behind `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE` feature flag

**CRITICAL**: The fix was deployed on Jan 14, but the **feature flag is NOT enabled in production**!

### Correlation with Missing Scorecards

| Customer | First Missing | Release Timeline |
|----------|---------------|------------------|
| rentokil | Dec 31, 2025 | After Dec 17 release |
| spirit | Jan 8, 2026 | Same day as Jan 8 release |
| guitar-center | Jan 10, 2026 | After Jan 8 release |
| nclh | Jan 11, 2026 | After Jan 8 release |

## Hypothesis Analysis

### Hypotheses Ruled Out

| Hypothesis | Evidence | Status |
|------------|----------|--------|
| **Calibration scorecard** | Verified `calibrated_scorecard_id IS NULL` for affected scorecards | ❌ RULED OUT |
| **Non-original scorecard type** | Verified `scorecard_type IS NULL` (UNSPECIFIED) for affected scorecards | ❌ RULED OUT |
| **Conversation not found** | Verified conversation exists for example scorecard (rentokil) | ❌ RULED OUT |
| **Customer-specific issue** | All 4 customers show same pattern for submitted scorecards (0% → 30-50% in Jan 2026) | ❌ RULED OUT |
| **Single deployment caused it** | Spirit's unsubmitted scorecard issue dates back to **Jul 2025**, long before recent deployments | ❌ RULED OUT |

### Hypotheses Still Possible

| Hypothesis | Evidence | Status |
|------------|----------|--------|
| **Async work race conditions** | Fix (CONVI-5565) specifically addresses race conditions; feature flag NOT enabled in prod | ✅ **LIKELY** |
| **Async work timeout/cancellation** | Async path is inherently unreliable; errors logged but may be lost | ✅ POSSIBLE |
| **Jan 8 deployment exacerbated existing race conditions** | Submitted scorecards issue correlates with Jan 8 release; 3-week code accumulation | ✅ POSSIBLE |

### How the Race Condition Causes Missing Historic Records

When the feature flag is **DISABLED** (current production):

1. **CreateScorecard/UpdateScorecard** writes to `director.scorecards` synchronously, then starts **async work** to write to `historic.scorecard_scores`

2. **SubmitScorecard** calls `AsyncExtraWorkForSubmitScorecard` which **conditionally skips** the historic write

#### The Skip Condition in `AsyncExtraWorkForSubmitScorecard`

```go
// action_submit_scorecard.go:483-490
func shouldSkipUpdateHistoricScoresWhenSubmitScorecard(scorecard *dbmodel.Scorecards) bool {
    validScorecardTypes := set.NewFromSlice([]coachingpb.ScorecardType{
        coachingpb.ScorecardType_SCORECARD_TYPE_UNSPECIFIED,
    })
    return !scorecard.SubmittedAt.Valid && validScorecardTypes.Has(apiScorecardType)
}

// action_submit_scorecard.go:1118-1125
// In AsyncExtraWorkForSubmitScorecard:
if !skipUpdateHistoricScores {
    _, err = scorecardutils.WriteHistoricScorecardScores(db, conversation, scorecard, scores, templateStructure)
    // ...
}
```

**`skipUpdateHistoricScores = true`** when BOTH conditions are met:
1. `!scorecard.SubmittedAt.Valid` - scorecard is NOT yet submitted
2. `ScorecardType` is `SCORECARD_TYPE_UNSPECIFIED` (normal scorecard)

#### When Historic Write is Skipped vs Written

| Scorecard Type | First Submit? | Skip Historic Write? | Reason |
|----------------|---------------|---------------------|--------|
| **UNSPECIFIED** (normal) | Yes | **YES - SKIP** | Assumes Create already wrote it |
| UNSPECIFIED | No (re-submit) | No | Write it |
| APPEAL_REQUEST | Any | No | Always write |
| APPEAL_RESOLVE | Any | No | Always write |
| GROUP_CALIBRATION_RESPONSE | Any | No | Always write |

#### The Race Condition Flow

```
CreateScorecard                         SubmitScorecard (first time)
     │                                        │
     ▼                                        ▼
Write to director ✓                     Write to director ✓
     │                                        │
     ▼                                        ▼
Start async work ───────────────►      shouldSkipUpdateHistoricScores()
(write to historic)                    → !SubmittedAt.Valid = true
     │                                 → Type = UNSPECIFIED
     │                                        │
     │                                        ▼
     │                            skipUpdateHistoricScores = TRUE
     │                                        │
     │                                        ▼
     │                            Historic write SKIPPED
     │                            (assumes Create already did it)
     │
     ▼
May still be pending/failed ─────────► Historic data MISSING!
```

#### Why Submitted Scorecards Have Higher Missing Rates

- **Unsubmitted scorecards**: Only depend on CreateScorecard's async work
- **Submitted scorecards**: Depend on CreateScorecard's async work completing **before** SubmitScorecard runs

For normal (UNSPECIFIED) scorecards being submitted for the **first time**:
- `skipUpdateHistoricScores = true`
- `AsyncExtraWorkForSubmitScorecard` **skips** the historic write
- **Assumes** CreateScorecard's async work already wrote to historic

If CreateScorecard's async work:
- Is still pending when SubmitScorecard runs
- Failed silently (timeout, error, context cancelled)
- Never ran at all

Then SubmitScorecard **skips** the historic write, resulting in **permanently missing** historic data.

### Root Cause Conclusion

The **root cause is the async historic write path** which has inherent race conditions that cause silent failures:

1. **The issue has existed for months** - Spirit's unsubmitted scorecards have been failing since Jul 2025
2. **The issue got significantly worse in January 2026** - possibly due to:
   - Increased load/volume
   - Code changes in Jan 8 release making race conditions more likely
   - Both factors combined
3. **The fix exists but isn't enabled** - CONVI-5565 (commit `a8f67df325`) was deployed Jan 14, but `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE` feature flag is NOT enabled in production

**The async historic write path has inherent race conditions that cause silent failures. The fix exists (CONVI-5565) but is not enabled in production.**

## Recommendations

### Immediate Actions

1. **Investigate the timeline** - Check deployments and logs around:
   - Late Dec 2025 / Early Jan 2026 (first occurrences)
   - Jan 20-21, 2026 (major spike to 90%+)
2. **Backfill missing records** - ~3,300+ scorecards across 3 customers sampled; likely many more across all customers
3. **Audit all customers in us-east-1-prod** - Pattern confirmed across rentokil, spirit, guitar-center

### Long-term Fix

1. **Enable `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE=true` in production**
   - This ensures historic writes happen atomically with director writes
   - If historic write fails, director write also rolls back
   - No orphaned records
   - Already enabled in staging, needs production rollout

2. **Add monitoring/alerting for historic write failures**
   - Currently errors are logged but may be missed
   - Add metrics for historic write success/failure
   - Alert when missing rate exceeds threshold

3. **Backfill tool for missing historic records**
   - Create a tool to identify and backfill scorecards missing from historic schema
   - Run across all affected customers

## Investigation Queries

### Database Connections

```bash
# rentokil
AWS_REGION=us-east-1 CONN=$(cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod rentokil-us-east-1) && psql "$CONN"

# spirit
AWS_REGION=us-east-1 CONN=$(cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod spirit-us-east-1) && psql "$CONN"

# guitar-center
AWS_REGION=us-east-1 CONN=$(cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod guitar-center-us-east-1) && psql "$CONN"

# nclh
AWS_REGION=us-east-1 CONN=$(cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod nclh-us-east-1) && psql "$CONN"
```

### Query 1: Monthly Breakdown of Missing Scorecards (Last 6 Months)

```sql
-- Replace 'CUSTOMER' and 'PROFILE' with actual values
SELECT
  DATE_TRUNC('month', d.created_at)::date as month,
  COUNT(*) as total_submitted,
  COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) as missing,
  ROUND(100.0 * COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) / COUNT(*), 1) as pct_missing
FROM director.scorecards d
LEFT JOIN (SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
           WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE') h
  ON d.resource_id = h.scorecard_id
WHERE d.customer = 'CUSTOMER' AND d.profile = 'PROFILE'
  AND d.submitted_at IS NOT NULL
  AND d.calibrated_scorecard_id IS NULL
  AND (d.scorecard_type IS NULL OR d.scorecard_type = 0)
  AND d.created_at > NOW() - INTERVAL '6 months'
GROUP BY DATE_TRUNC('month', d.created_at)
ORDER BY month DESC;
```

### Query 2: Daily Breakdown of Missing Scorecards

```sql
-- Replace 'CUSTOMER' and 'PROFILE' with actual values
SELECT
  DATE(d.created_at) as date,
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) as missing
FROM director.scorecards d
LEFT JOIN (SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
           WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE') h
  ON d.resource_id = h.scorecard_id
WHERE d.customer = 'CUSTOMER' AND d.profile = 'PROFILE'
  AND d.submitted_at IS NOT NULL
  AND d.calibrated_scorecard_id IS NULL
  AND (d.scorecard_type IS NULL OR d.scorecard_type = 0)
  AND d.created_at BETWEEN '2025-12-15' AND '2026-01-15'
GROUP BY DATE(d.created_at)
ORDER BY date;
```

### Query 3: Find First Missing Scorecard

```sql
-- Replace 'CUSTOMER' and 'PROFILE' with actual values
SELECT MIN(created_at) as first_missing_created
FROM director.scorecards
WHERE customer = 'CUSTOMER' AND profile = 'PROFILE'
  AND submitted_at IS NOT NULL
  AND calibrated_scorecard_id IS NULL
  AND (scorecard_type IS NULL OR scorecard_type = 0)
  AND resource_id NOT IN (
    SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
    WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE'
  );
```

### Query 4: List First N Missing Scorecards with Details

```sql
-- Replace 'CUSTOMER' and 'PROFILE' with actual values
SELECT resource_id, created_at, submitted_at
FROM director.scorecards
WHERE customer = 'CUSTOMER' AND profile = 'PROFILE'
  AND submitted_at IS NOT NULL
  AND calibrated_scorecard_id IS NULL
  AND (scorecard_type IS NULL OR scorecard_type = 0)
  AND resource_id NOT IN (
    SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
    WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE'
  )
ORDER BY created_at
LIMIT 10;
```

### Query 5: Check Specific Scorecard Details

```sql
-- Check scorecard properties
SELECT resource_id, calibrated_scorecard_id, scorecard_type, conversation_id,
       submitted_at, created_at, updated_at
FROM director.scorecards
WHERE customer = 'CUSTOMER' AND profile = 'PROFILE'
AND resource_id = 'SCORECARD_ID';

-- Check if scores exist
SELECT COUNT(*) as score_count
FROM director.scores
WHERE customer = 'CUSTOMER' AND profile = 'PROFILE'
AND scorecard_id = 'SCORECARD_ID';

-- Check if historic records exist
SELECT COUNT(*) as historic_count
FROM historic.scorecard_scores
WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE'
AND scorecard_id = 'SCORECARD_ID';

-- Check if conversation exists
SELECT conversation_id, started_at, ended_at
FROM app.chats
WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE'
AND conversation_id = 'CONVERSATION_ID';
```

### Query 6: Count Total Missing in Last Month

```sql
-- Replace 'CUSTOMER' and 'PROFILE' with actual values
SELECT
  COUNT(*) as total_submitted,
  COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) as missing_from_historic
FROM director.scorecards d
LEFT JOIN (SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
           WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE') h
  ON d.resource_id = h.scorecard_id
WHERE d.customer = 'CUSTOMER' AND d.profile = 'PROFILE'
  AND d.submitted_at IS NOT NULL
  AND d.calibrated_scorecard_id IS NULL
  AND (d.scorecard_type IS NULL OR d.scorecard_type = 0)
  AND d.created_at > NOW() - INTERVAL '1 month';
```

### Query 7: Monthly Breakdown for Unsubmitted Scorecards

```sql
-- Replace 'CUSTOMER' and 'PROFILE' with actual values
SELECT
  DATE_TRUNC('month', d.created_at)::date as month,
  COUNT(*) as total_unsubmitted,
  COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) as missing,
  ROUND(100.0 * COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) / COUNT(*), 1) as pct_missing
FROM director.scorecards d
LEFT JOIN (SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
           WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE') h
  ON d.resource_id = h.scorecard_id
WHERE d.customer = 'CUSTOMER' AND d.profile = 'PROFILE'
  AND d.submitted_at IS NULL
  AND d.calibrated_scorecard_id IS NULL
  AND (d.scorecard_type IS NULL OR d.scorecard_type = 0)
  AND d.created_at > NOW() - INTERVAL '6 months'
GROUP BY DATE_TRUNC('month', d.created_at)
ORDER BY month DESC;
```

### Query 8: Hourly Breakdown for Specific Date

```sql
-- Useful for identifying batch job failures
SELECT
  DATE_TRUNC('hour', d.created_at) as hour,
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) as missing
FROM director.scorecards d
LEFT JOIN (SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
           WHERE customer_id = 'CUSTOMER' AND profile_id = 'PROFILE') h
  ON d.resource_id = h.scorecard_id
WHERE d.customer = 'CUSTOMER' AND d.profile = 'PROFILE'
  AND d.submitted_at IS NULL
  AND d.calibrated_scorecard_id IS NULL
  AND (d.scorecard_type IS NULL OR d.scorecard_type = 0)
  AND d.created_at BETWEEN '2026-01-23 00:00:00' AND '2026-01-24 00:00:00'
GROUP BY DATE_TRUNC('hour', d.created_at)
ORDER BY hour;
```

### Query 9: Template Distribution for Missing Scorecards

```sql
SELECT template_id, COUNT(*) as cnt
FROM director.scorecards
WHERE customer = 'CUSTOMER' AND profile = 'PROFILE'
  AND submitted_at IS NULL
  AND created_at BETWEEN 'START_TIME' AND 'END_TIME'
GROUP BY template_id
ORDER BY cnt DESC
LIMIT 10;
```

## Staging Environment Check

### Feature Flag Enablement Date

**Commit:** `1c85e95842c` - "CONVI-5565: Enable COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE in staging (#251184)"
**Date:** January 18, 2026 13:27

The feature flag was enabled in all staging environments (chat-staging, us-west-2-staging, voice-staging) on this date.

### Customer: cox-sales (chat-staging)

**Result:** No scorecards exist for this customer in staging.

### Customer: cresta/walter-dev (voice-staging)

**Connection:**
```bash
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only voice-staging voice-staging walter-dev) && psql "$CONN"
```

**Result:** 14M+ scorecards exist (customer=cresta, profile=walter-dev)

| Month | Submitted Total | Missing | % Missing |
|-------|-----------------|---------|-----------|
| Jan 2026 | 21 | 2 | 9.5% |
| Dec 2025 | 11 | 0 | 0.0% |
| Nov 2025 | 19 | 1 | 5.3% |
| Oct 2025 | 1 | 0 | 0.0% |

**Note:** The missing rate for submitted scorecards (5-10%) is significantly better than production (30-50%), suggesting the sync path helps. However, queries for more granular before/after analysis timed out due to the large data volume (14M+ scorecards).

**Unsubmitted scorecards** show 86-90% missing, but this is expected as:
1. Most were created before the flag was enabled (Jan 18, 2026)
2. The large volume (~14M) may be from automated testing that doesn't go through normal code paths

## Git Commands for Release Investigation

```bash
# Check apiserver prod releases in flux-deployments
cd /path/to/flux-deployments
git log --format="%h %ad %s" --date=format:"%Y-%m-%d %H:%M" --since="2025-12-01" --until="2026-01-15" -- "**/apiserver*" "**/cresta-api*" | grep -E "prod-main|prod-early"

# Check image version in a specific release commit
git show <COMMIT>:apps/apiserver/releases/03-prod-main/helmrelease-apiserver.yaml | grep "tag:"

# Check scorecard-related changes between two go-servers commits
cd /path/to/go-servers
git log --oneline <OLD_COMMIT>..<NEW_COMMIT> -- "apiserver/internal/coaching/*" "shared/scoring/*" "*scorecard*" "*historic*"
```

---

## Production Fix Verification (February 3, 2026)

### Feature Flag Enablement

**Commit**: `2f406fb6fc7`
**Date**: January 29, 2026 at 15:38:38
**PR**: #253866 - "Enable COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE for apiserver"

The feature flag was enabled across all release stages (head, staging, prod-early, prod-main).

### Backfill Jobs

Backfill jobs were run for all affected customers to fix historical missing records. Data before Feb 1 reflects backfilled state.

### Verification Results (Feb 1+ Only)

**Note**: Only data from Feb 1 onwards is reliable for measuring feature flag impact, as earlier data was affected by backfill jobs.

Using `compare_scorecard_sync.go` script to verify all 4 customers:

#### Rentokil

| Date | Total | Missing | % Missing |
|------|-------|---------|-----------|
| 2026-02-01 | 76 | 0 | 0.0% |
| 2026-02-02 | 250 | 0 | 0.0% |
| 2026-02-03 | 139 | 0 | 0.0% |
| **TOTAL** | **465** | **0** | **0.0%** |

ClickHouse: ✓ 468/468 (100%) found

#### Spirit

| Date | Total | Missing | % Missing |
|------|-------|---------|-----------|
| 2026-02-01 | 93 | 0 | 0.0% |
| 2026-02-02 | 232 | 0 | 0.0% |
| 2026-02-03 | 196 | 0 | 0.0% |
| **TOTAL** | **521** | **0** | **0.0%** |

ClickHouse: ✓ 524/524 (100%) found

#### NCLH

| Date | Total | Missing | % Missing |
|------|-------|---------|-----------|
| 2026-02-02 | 3 | 0 | 0.0% |
| 2026-02-03 | 2 | 0 | 0.0% |
| **TOTAL** | **5** | **0** | **0.0%** |

ClickHouse: ✓ 5/5 (100%) found

#### Guitar-Center

| Date | Total | Missing | % Missing |
|------|-------|---------|-----------|
| 2026-02-01 | 2 | 0 | 0.0% |
| 2026-02-02 | 6 | 0 | 0.0% |
| 2026-02-03 | 13 | 0 | 0.0% |
| **TOTAL** | **21** | **0** | **0.0%** |

ClickHouse: ✓ 21/21 (100%) found

### Summary Comparison

| Customer | Before Fix (Jan 2026) | After Fix (Feb 1-3) | ClickHouse (Full Verification) |
|----------|----------------------|---------------------|--------------------------------|
| rentokil | 32.0% missing | **0.0%** | ✓ 468/468 (100%) |
| spirit | 29.2% missing | **0.0%** | ✓ 524/524 (100%) |
| nclh | 51.9% missing | **0.0%** | ✓ 5/5 (100%) |
| guitar-center | 40.8% missing | **0.0%** | ✓ 21/21 (100%) |
| **TOTAL** | | | **✓ 1,018/1,018 (100%)** |

### Conclusion

**The fix is working.** After enabling `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE` in production on Jan 29, 2026:

1. **All new scorecards are properly written to historic schema** - 0% missing rate for all verified customers
2. **All scorecards are syncing to ClickHouse** - Sample verification confirms data flows from Postgres to ClickHouse
3. **Historical data was backfilled** - Missing records from Jan 2026 have been recovered
4. **Issue is resolved** - The race condition between async historic writes and scorecard submission is eliminated by the synchronous write path

### Verification Script

A Go script `compare_scorecard_sync.go` is available in this directory for future verification:

```bash
# Usage
go run compare_scorecard_sync.go -customer <customer> -profile <profile> -start <date> -end <date>

# Example
go run compare_scorecard_sync.go -customer rentokil -profile us-east-1 -start 2026-02-01 -end 2026-02-03
```
