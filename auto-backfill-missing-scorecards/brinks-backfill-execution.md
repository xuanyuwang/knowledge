# Brinks Scorecard Backfill Execution

**Created**: 2026-04-09
**Updated**: 2026-04-09
**Customer**: brinks / care-voice
**Cluster**: voice-prod

## Problem

Brinks has **436 scorecards missing from ClickHouse** out of 1,676 submitted in March 2026 (**26.0% missing rate**). The CH write path was broken before ~March 30; after March 30 the sync is healthy (0% missing).

Source: `cron-scorecard-sync-monitor` run on 2026-04-07 + manual verification on 2026-04-09.

## Daily Breakdown (March 2026)

Queries use `director.scorecards` (PG) vs `scorecard_d FINAL` (CH), matching the sync monitor approach.

| Day | PG | CH | Missing | Rate |
|-----|----|----|---------|------|
| 03-01 | 37 | 37 | 0 | 0.0% |
| 03-02 | 20 | 17 | **3** | 15.0% |
| 03-03 | 79 | 42 | **37** | 46.8% |
| 03-04 | 56 | 30 | **26** | 46.4% |
| 03-05 | 44 | 23 | **21** | 47.7% |
| 03-06 | 31 | 11 | **20** | 64.5% |
| 03-07 | 8 | 5 | **3** | 37.5% |
| 03-09 | 74 | 52 | **22** | 29.7% |
| 03-10 | 87 | 40 | **47** | 54.0% |
| 03-11 | 89 | 67 | **22** | 24.7% |
| 03-12 | 60 | 36 | **24** | 40.0% |
| 03-13 | 62 | 38 | **24** | 38.7% |
| 03-14 | 22 | 22 | 0 | 0.0% |
| 03-16 | 44 | 36 | **8** | 18.2% |
| 03-17 | 60 | 30 | **30** | 50.0% |
| 03-18 | 60 | 45 | **15** | 25.0% |
| 03-19 | 52 | 33 | **19** | 36.5% |
| 03-20 | 41 | 23 | **18** | 43.9% |
| 03-21 | 15 | 15 | 0 | 0.0% |
| 03-23 | 45 | 43 | **2** | 4.4% |
| 03-24 | 125 | 103 | **22** | 17.6% |
| 03-25 | 105 | 78 | **27** | 25.7% |
| 03-26 | 110 | 76 | **34** | 30.9% |
| 03-27 | 62 | 52 | **10** | 16.1% |
| 03-28 | 11 | 9 | **2** | 18.2% |
| **03-30** | **63** | **63** | **0** | **0.0%** |
| **03-31** | **214** | **214** | **0** | **0.0%** |

**Summary**: Mar 1-29 total: 1,399 PG / 963 CH / **436 missing (31.2%)**. Mar 30+: 0 missing.

## Test Run: March 3-5 (Completed)

Ran a 3-day backfill to validate the approach before the full run.

| Metric | Value |
|--------|-------|
| Range | 2026-03-03 to 2026-03-06 |
| Conversations | 58,049 |
| Missing before | 84 |
| Missing after | **0** |
| Recovery | **84/84 (100%)** |
| Backfill command | `./backfill.sh voice-prod "brinks" 2026-03-03 2026-03-06` |
| Temporal workflow | `reindexconversations-brinks-care-voice-27430522-f2d3-4a71-993b-955ad30a8446` |
| Duration | ~50 min (est. based on ~1.1K conversations/min throughput) |

Scorecard data was fully recovered before the workflow finished processing all conversations (~34% through).

## Execution Plan: Full March Backfill

### Approach: Per-Day Parallel

Brinks has ~19K conversations/day. A single job for 29 days (~550K conversations) would take ~8 hours. Running per-day jobs in parallel is safe for this volume and reduces wall-clock time to ~20-30 minutes per job.

**Days to backfill**: March 2-13, 16-20, 23-28 (22 days with missing data)
**Skip**: March 1, 14, 21 (0 missing), March 3-5 (already backfilled in test run), weekends with no data (Mar 8, 15, 22, 29)

### Commands

```bash
cd /Users/xuanyu.wang/repos/knowledge/backfill-scorecards

# Run all missing days in parallel (22 jobs, ~19K conversations each)
# Days with existing 0% missing are safe to re-run (reindex only inserts)
for day in 02 06 07 09 10 11 12 13 16 17 18 19 20 23 24 25 26 27 28; do
  START="2026-03-${day}"
  # Compute next day
  END=$(date -j -v+1d -f "%Y-%m-%d" "$START" "+%Y-%m-%d")
  echo "Launching backfill: $START -> $END"
  ./backfill.sh voice-prod "brinks" "$START" "$END" &
  sleep 2  # stagger kubectl calls slightly
done
wait
echo "All backfill jobs submitted"
```

### Monitoring

```bash
# Port-forward Temporal
kubectl --context=voice-prod_dev -n temporal port-forward svc/temporal-frontend-headless 7233:7233

# List all running workflows
temporal workflow list --namespace ingestion --address localhost:7233 \
  --query 'WorkflowId STARTS_WITH "reindexconversations-brinks"'

# Check specific workflow heartbeat
temporal workflow describe --namespace ingestion --address localhost:7233 \
  --workflow-id "<workflow-id>"
```

### Verification

After all workflows complete, re-run the sync comparison:

```bash
# PG daily counts
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only voice-prod voice-prod brinks-care-voice) \
  && psql "$CONN" -c "
SELECT DATE(submitted_at) as day, COUNT(*) as pg_count
FROM director.scorecards
WHERE customer = 'brinks' AND profile = 'care-voice'
  AND submitted_at IS NOT NULL
  AND calibrated_scorecard_id IS NULL
  AND (scorecard_type IS NULL OR scorecard_type = 0)
  AND submitted_at >= '2026-03-01' AND submitted_at < '2026-03-30'
GROUP BY DATE(submitted_at)
ORDER BY day;"

# CH daily counts
clickhouse-client ... -q "
SELECT toDate(scorecard_submit_time) as day, COUNT(DISTINCT scorecard_id) as ch_count
FROM brinks_care_voice.scorecard_d FINAL
WHERE scorecard_submit_time >= '2026-03-01' AND scorecard_submit_time < '2026-03-30'
  AND _row_exists = 1
GROUP BY day ORDER BY day;"

# Expected: all days PG == CH, 0 missing
```

## DB Connections

```bash
# PostgreSQL
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only voice-prod voice-prod brinks-care-voice)
psql "$CONN"

# ClickHouse (brinks_care_voice db)
clickhouse-client \
  -h clickhouse-conversations.voice-prod.internal.cresta.ai \
  --port 9440 -u admin --password 'jIKiJqSXovuvntudQHuMqwD0PWaJ8buU' --secure
```

## Post-Backfill Verification

All 19 per-day Temporal workflows completed successfully. Re-ran sync comparison:

| Day | PG | CH Before | CH After | Recovered | Still Missing |
|-----|----|-----------|-----------|-----------|----|
| 03-02 | 20 | 17 | **20** | 3 | 0 |
| 03-03 | 79 | 42 | **79** | 37 | 0 |
| 03-04 | 56 | 30 | **56** | 26 | 0 |
| 03-05 | 44 | 23 | **44** | 21 | 0 |
| 03-06 | 31 | 11 | **31** | 20 | 0 |
| 03-07 | 8 | 5 | **8** | 3 | 0 |
| 03-09 | 74 | 52 | **74** | 22 | 0 |
| 03-10 | 87 | 40 | **87** | 47 | 0 |
| 03-11 | 89 | 67 | **89** | 22 | 0 |
| 03-12 | 60 | 36 | **60** | 24 | 0 |
| 03-13 | 62 | 38 | **38** | 0 | **24** |
| 03-16 | 44 | 36 | **44** | 8 | 0 |
| 03-17 | 60 | 30 | **60** | 30 | 0 |
| 03-18 | 60 | 45 | **60** | 15 | 0 |
| 03-19 | 52 | 33 | **52** | 19 | 0 |
| 03-20 | 41 | 23 | **41** | 18 | 0 |
| 03-23 | 45 | 43 | **44** | 1 | **1** |
| 03-24 | 125 | 103 | **125** | 22 | 0 |
| 03-25 | 105 | 78 | **105** | 27 | 0 |
| 03-26 | 110 | 76 | **111** | 35 | 0* |
| 03-27 | 62 | 52 | **52** | 0 | **10** |

**Total recovered**: 401/436 (92.0%). **Still missing**: 35 (Mar 13: 24, Mar 23: 1, Mar 27: 10).

*Mar 26 shows 111 in CH vs 110 in PG — likely a timing difference or late submission.

### Root Cause of 35 Still Missing

34 of 35 missing scorecards have **empty `conversation_id`** (empty string). They are standalone QA scorecards from an external system (process IDs like "VI36:...", "708666532", "CS Number: 761209129").

`reindexconversations` has two sync mechanisms:
1. **Conversation-based sync**: Finds conversations in date range, re-syncs linked scorecards. Worked for all days.
2. **Orphan scorecard batch**: Syncs no-conv-id scorecards. This step processed most days (timestamps `20:25:09-20:25:37` UTC, sequential) but **skipped Mar 13 and Mar 27** for unknown reasons.

The 1 remaining Mar 23 scorecard HAS a conversation_id, but its conversation is from Mar 13. The Mar 23 backfill couldn't find it (wrong conv date), and the Mar 13 backfill didn't sync it.

**Fix**: ID-based backfill targeting these 35 specific scorecard IDs. Missing IDs saved to `/tmp/brinks_still_missing.txt`.

## Execution Log

| Date | Action | Result |
|------|--------|--------|
| 2026-04-09 | Step 1: Confirmed missing rate | 436/1,676 (26.0%) for March |
| 2026-04-09 | Step 2: Test backfill Mar 3-5 | 84/84 recovered (100%) |
| 2026-04-09 | Step 3: Full backfill Mar 2-28 | 401/436 recovered (92%), 35 remaining |
| 2026-04-09 | Step 4: Post-backfill verification | 35 missing: 34 no-conv-id + 1 conv-date-mismatch |
| | Step 5: ID-based backfill for remaining 35 | Pending |
