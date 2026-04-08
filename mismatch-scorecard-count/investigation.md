# Investigation: Mismatched Scorecard Count (Brinks)

**Created**: 2026-04-06
**Updated**: 2026-04-06
**Customer**: brinks / care-voice
**Cluster**: voice-prod

## Problem Statement

On the QM Report page, the same filter set returns different scorecard counts:
- **RetrieveDirectorTaskStats** (PG): **197** evaluated scorecards
- **RetrieveQAScoreStats** (CH): **25** scorecards

## DB Connections

```bash
# PostgreSQL
open $(cresta-cli connstring -i --read-only voice-prod voice-prod brinks-care-voice)

# ClickHouse (brinks_care_voice db)
clickhouse://admin:jIKiJqSXovuvntudQHuMqwD0PWaJ8buU@clickhouse-conversations.voice-prod.internal.cresta.ai:9440
```

## Request Differences

| Field | DirectorTaskStats (PG) | QAScoreStats (CH) |
|-------|----------------------|-------------------|
| Template | `...@e6b75151` (with revision) | `...dbc027ea-adc3-4184-9f86-610543c79414` (no revision) |
| **criterionIdentifiers** | **none** | **`1b372658-0599-4e45-a1f3-d1fb57f4857f`** |
| **includeNaScored** | n/a | **false** |
| taskStatus | [2, 4] (submitted+acknowledged) | scorecardStatuses: [] |
| Time range | 2026-03-01T04:00:00 to 2026-04-01T03:59:59.999 | same |

## Root Cause: TWO Separate Issues

### Issue 1: Data Sync Gap (PG: 197, CH: 81)

**116 scorecards exist in PG `historic.scorecard_scores` but are missing from CH `score_d`.**

#### Temporal Pattern

| Date Range | PG | CH | Missing |
|------------|-----|-----|---------|
| Mar 2-29 | 132 | 16 | **116** |
| Mar 30-31 + Apr 1 | 65 | 65 | **0** |

Before March 30: ~88% of scorecards missing from CH.
After March 30: **0% missing** -- 100% sync.

#### Key Observation

The 16 pre-March-30 scorecards that ARE in CH were all **re-updated** between March 30 - April 1 (`updated_at` much later than `submitted_at`). This means:
- The original CH write at create/submit time **failed silently**
- A later update triggered a new CH write, which succeeded

**Conclusion**: The ClickHouse write path was broken for this customer before ~March 30. Something was fixed/deployed around that date. Scorecards that were re-edited after the fix got synced; the rest remain missing.

#### Daily Breakdown

| Date | PG | CH | Missing |
|------|-----|-----|---------|
| 03-02 | 2 | 0 | 2 |
| 03-03 | 9 | 2 | 7 |
| 03-04 | 9 | 2 | 7 |
| 03-05 | 5 | 1 | 4 |
| 03-06 | 6 | 0 | 6 |
| 03-07 | 2 | 0 | 2 |
| 03-09 | 3 | 0 | 3 |
| 03-10 | 2 | 0 | 2 |
| 03-11 | 7 | 1 | 6 |
| 03-12 | 5 | 0 | 5 |
| 03-13 | 7 | 1 | 6 |
| 03-16 | 1 | 0 | 1 |
| 03-17 | 8 | 0 | 8 |
| 03-18 | 6 | 1 | 5 |
| 03-19 | 4 | 0 | 4 |
| 03-20 | 15 | 2 | 13 |
| 03-23 | 2 | 1 | 1 |
| 03-24 | 8 | 0 | 8 |
| 03-25 | 8 | 0 | 8 |
| 03-26 | 18 | 3 | 15 |
| 03-27 | 5 | 2 | 3 |
| **03-30** | **23** | **23** | **0** |
| **03-31** | **31** | **31** | **0** |
| **04-01** | **11** | **11** | **0** |

### Issue 2: Criterion Filter (CH: 81 -> 25)

The QAScoreStats request includes `criterionIdentifiers: ["1b372658-0599-4e45-a1f3-d1fb57f4857f"]` and `includeNaScored: false`. This criterion only exists on a subset of scorecards.

| Source | Filter | Count |
|--------|--------|-------|
| PG `historic.scorecard_scores` | template + March | 197 |
| PG `historic.scorecard_scores` | + criterion `1b372658` | 56 |
| CH `score_d` | template + March | 81 |
| CH `score_d` | + criterion `1b372658` | 25 |
| CH `score_d` | + criterion + not_applicable=false | 25 |

The criterion filter is **by design** -- RetrieveQAScoreStats is supposed to filter by criterion. The real issue is that if all 197 were in CH, the criterion-filtered count would be ~56 (matching PG), not 25.

## Impact Summary

| Comparison | Expected | Actual | Root Cause |
|-----------|----------|--------|------------|
| PG vs CH (total) | 197 = 197 | 197 vs 81 | **Data sync gap** (116 missing before Mar 30) |
| CH total vs CH criterion | 81 > 25 | 81 vs 25 | **By design** (criterion filter) |
| PG criterion vs CH criterion | 56 = 56 | 56 vs 25 | 31 missing due to data sync gap |

## Next Steps

1. **Investigate what changed around March 29-30** -- check deployments, config changes for voice-prod
2. **Backfill the 116 missing scorecards** to CH from PG historic data
3. **Verify the criterion filter behavior is correct** -- confirm that the QM Report page is intentionally using a criterion filter in RetrieveQAScoreStats (this reduces count from 197 to ~56 even with perfect sync)

## Sample Missing Scorecards

```
019caf37-19e0-766e-9466-fe2f43d09799  (Mar 02)
019cafa8-8eaf-72de-8b53-fdff7e0f42bd  (Mar 02)
019cb109-ce35-7308-9bad-c832a716cf99  (Mar 03)
019cb112-8374-700f-b254-5c643aad00b7  (Mar 03)
019cb11a-e359-7776-8518-a3694cfd7d0f  (Mar 03)
```

Full list: 116 IDs available via diff of `/tmp/pg_scorecard_ids.txt` and `/tmp/ch_scorecard_ids.txt`.
