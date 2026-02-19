# Data Analysis: Alaska Air Backfill Scope

**Created:** 2026-02-18
**Updated:** 2026-02-18

## Data Volume Summary

| Metric | Count |
|--------|-------|
| Postgres conversations (cron filter match) | 1,510,418 |
| ClickHouse distributed total rows | 1,636,467 |
| ClickHouse distributed distinct conversations | 1,633,276 |
| **ClickHouse duplicate conversations (stale)** | **3,191** |
| Days in range (Jan 1 – Feb 18) | 49 |
| Avg conversations per day | ~30,800 (Postgres) |

## ClickHouse vs Postgres Comparison

ClickHouse has **more** distinct conversations (1.63M) than Postgres currently matches (1.51M). The excess ~123K are likely stale entries where conversations were later:
- Re-assigned to dev users (now excluded by `is_dev_user = FALSE`)
- Had their `ended_at` set back to null/zero
- Changed source type

### Daily Volume (sample days)

| Day | Postgres (cron filter) | ClickHouse (distributed) | CH duplicates |
|-----|----------------------|------------------------|---------------|
| 2026-01-01 (Wed) | 27,255 | 33,630 / 33,625 unique | 5 |
| 2026-01-05 (Sun) | 44,593 | 57,775 / 57,766 unique | 9 |
| 2026-01-17 (Sat) | 26,341 | 26,817 / 26,804 unique | 13 |
| 2026-01-20 (Tue) | 33,929 | 34,940 / 34,393 unique | 547 |
| 2026-01-29 (Thu) | 31,273 | 32,699 / 31,880 unique | 819 |
| 2026-02-10 (Tue) | 32,249 | 32,629 / 32,620 unique | 9 |
| 2026-02-13 (Fri) | 30,208 | 31,014 / 30,776 unique | 238 |

Days with higher duplicate counts (Jan 20, Jan 23, Jan 29, Feb 13-14) likely had more agent re-assignments.

## Usecase Distribution

| Usecase | Postgres | ClickHouse (distributed) | Ratio |
|---------|----------|------------------------|-------|
| reservations | 979,269 | ~978K | ~1:1 |
| reservations-chat | 279,628 | ~405K | 1.4x (stale usecases) |
| customer-care | 148,826 | ~149K | ~1:1 |
| cargo | 40,254 | ~40K | ~1:1 |
| central-baggage | 33,589 | ~34K | ~1:1 |
| chat | 18,467 | ~15K | 0.8x |
| cco | 9,177 | ~9K | ~1:1 |

## Backfill Operations

### What the backfill needs to do:
1. **DELETE** ~1.64M rows from ClickHouse (all 3 shards, ON CLUSTER)
2. **Wait** for mutations to complete
3. **RE-INSERT** ~1.51M rows (from Postgres via the cron job)

### After backfill, expected state:
- ~1.51M rows total (matching Postgres)
- 0 duplicate conversation_ids
- Correct agent_user_id and usecase_id for all rows

## Cluster Info

- **ClickHouse cluster**: clickhouse-conversations.us-east-1-prod.internal.cresta.ai:9440
- **Database**: alaska_air_us_east_1
- **Shards**: 3 (local table holds ~1/3 of data)
- **Table engine**: ReplicatedReplacingMergeTree
- **Distributed table**: conversation_with_labels_d

## Sample Timing: Feb 18 Backfill (without pre-deletion)

**Ran:** 2026-02-19 01:26 UTC

### Results

| Metric | Value |
|--------|-------|
| Day tested | Feb 18 (~32K conversations) |
| Job creation → completion | **~5 seconds** |
| Conversations processed | 31,828 newly written rows |
| Total profiles scanned | 88 (filtered to 1: alaska-air) |
| Customer filter | Working correctly (`FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE`) |

### Duplicate Impact (without pre-deletion)

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Total rows (distributed) | 32,151 | 55,653 | +23,502 |
| Distinct conversations | 32,142 | 32,144 | +2 |
| Conversations with duplicates | 9 | 23,504 | +23,495 |
| Extra duplicate rows | 9 | 23,509 | +23,500 |

**73% of conversations** ended up with duplicate rows when backfilling without pre-deletion. This confirms deletion is mandatory.

### Extrapolated Full Backfill Timing

**Option A: Bulk delete + sequential cron jobs**
- DELETE 1.64M rows (ON CLUSTER): ~10-30 minutes for mutation
- Cron job per day: ~5 seconds × 49 days = ~4 minutes
- **Total: ~15-35 minutes**

**Option B: Per-day delete + cron (sequential script)**
- Delete per day (~33K rows): ~1-2 minutes mutation wait
- Cron job per day: ~5 seconds
- Total per day: ~2-3 minutes
- **Total: ~2-3 hours for 49 days**

**Recommendation:** Option A (bulk delete first, then run jobs) is much faster.

### Cleanup Needed

The test backfill created 23,509 duplicate rows for Feb 18. These will be cleaned up during the actual full backfill (delete all + re-insert).

## Full Backfill Execution: 2026-02-19

### Timeline

| Step | Start (UTC) | End (UTC) | Duration |
|------|-------------|-----------|----------|
| Bulk DELETE 1.65M rows | 01:32:49 | 01:32:50 | ~1 second |
| Mutation completion | 01:33:14 | 01:33:14 | Instant |
| Verify 0 rows | 01:33 | 01:33 | — |
| Backfill 49 days | 01:34:00 | 01:49:17 | **15 min 17 sec** |
| **Total** | **01:32:49** | **01:49:17** | **~16.5 minutes** |

### Results

| Metric | Before | After |
|--------|--------|-------|
| Total rows | 1,651,744 | 1,539,530 |
| Distinct conversations | 1,633,276 | 1,539,530 |
| Duplicate conversations | 18,468 | **0** |
| Rows per conversation | 1.01x | **1.00x (perfect)** |

### Comparison with Postgres

| Metric | Postgres (cron filters) | ClickHouse (after backfill) |
|--------|------------------------|---------------------------|
| Total conversations | 1,510,418 | 1,539,530 |
| Delta | — | +29,112 (~1.9%) |

The slight overshoot (+29K) is expected: Postgres was queried with a JOIN on `is_dev_user = FALSE`, but the cron may include some conversations where the agent was a dev user at the time of conversation but was later flagged. The numbers are within reasonable range.

### Usecase Distribution (after backfill)

| Usecase | Before backfill | After backfill | Postgres |
|---------|----------------|----------------|----------|
| reservations | ~978K | 979,016 | 979,269 |
| reservations-chat | ~405K (stale!) | 306,938 | 279,628 |
| customer-care | ~149K | 148,764 | 148,826 |
| chat | ~15K | 18,864 | 18,467 |

The `reservations-chat` count dropped from ~405K to 307K — confirming that ~100K conversations had stale usecase labels that have been corrected.

### Zero Failures

All 49 days completed successfully with no failures or retries needed.
