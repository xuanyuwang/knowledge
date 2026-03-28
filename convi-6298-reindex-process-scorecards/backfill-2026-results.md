# Phase 1 Backfill Results: 2026 Data

**Created**: 2026-03-28
**Time range**: 2026-01-01 to 2026-03-29

## Summary

| Cluster | Total Workflows | Completed | Failed | Running (stuck) | Notes |
|---------|----------------|-----------|--------|-----------------|-------|
| voice-prod | 49 | 48 | 1 | 0 | 1 failed: brinks-care (duplicate criterion) |
| chat-prod | 28 | 28 | 0 | 0 | All good |
| us-west-2-prod | 97 | 97 | 0 | 0 | All good |
| us-east-1-prod | 90 | 90 | 0 | 0 | 10 initially stuck (heartbeat timeout), resolved by re-running one by one |
| ap-southeast-2-prod | 4 | 4 | 0 | 0 | All good |
| eu-west-2-prod | 6 | 6 | 0 | 0 | All good |
| ca-central-1-prod | 2 | 2 | 0 | 0 | All good |
| schwab-prod | 0 | 0 | 0 | 0 | Old image (Feb 25) — REINDEX_MODE=process not supported |

**Overall**: 276 workflows dispatched, 275 completed, 1 failed (brinks-care data issue), schwab-prod skipped (old image)

## Issue 1: brinks-care duplicate criterion (voice-prod)

**Workflow**: `reindexscorecards-brinks-care-voice-4fb69e9f-aeb0-476a-82ef-61912a1377a4`
**Status**: FAILED
**Error**: `failed to generate historic scorecard scores: rpc error: code = InvalidArgument desc = duplicate score for the same criterion 'dddd652f-db46-415a-bbbd-6999b40472fc' given`
**Scorecard**: `019c6c38-65f8-769a-b328-cff8dcfdedbc`

**Root cause**: Pre-existing data quality issue — scorecard has duplicate scores for the same criterion. `GenerateHistoricScorecardScores` rejects this as invalid input.

**Impact**: Only brinks-care process scorecards on voice-prod are not backfilled. Other customers on voice-prod completed successfully.

**Fix options**:
1. Fix the bad data in PG (deduplicate the scores for criterion `dddd652f-db46-415a-bbbd-6999b40472fc` in scorecard `019c6c38-65f8-769a-b328-cff8dcfdedbc`)
2. Make `GenerateHistoricScorecardScores` skip (not fail on) scorecards with duplicate criteria
3. Accept the gap — this is a single scorecard with bad data

## Issue 2: us-east-1-prod heartbeat timeouts (10 workflows)

**Status**: Running (stuck, ~24+ min with TIMEOUT_TYPE_HEARTBEAT)

**Stuck workflows**:
| Workflow ID | Customer |
|-------------|----------|
| `reindexscorecards-lending-club-us-east-1-...` | lending-club |
| `reindexscorecards-cng-us-east-1-...` | cng |
| `reindexscorecards-spirit-us-east-1-...` | spirit |
| `reindexscorecards-sunbit-us-east-1-...` | sunbit |
| `reindexscorecards-rentokil-us-east-1-...` | rentokil |
| `reindexscorecards-fubotv-us-east-1-...` | fubotv |
| `reindexscorecards-marriott-us-east-1-...` | marriott |
| `reindexscorecards-rcg-us-east-1-...` | rcg |
| `reindexscorecards-united-east-us-east-1-...` | united-east |
| `reindexscorecards-alaska-air-us-east-1-...` | alaska-air |

**Root cause**: Running all 90 workflows concurrently overwhelmed the activity workers. When re-run one by one, all 10 completed successfully.

**Resolution**: Terminated stuck workflows, re-dispatched one by one with `RUN_ONLY_FOR_CUSTOMER_IDS`. Results:

| Customer | Duration |
|----------|----------|
| lending-club | 543s |
| cng | 43s |
| spirit | 21s |
| sunbit | 11s |
| rentokil | 31s |
| fubotv | 94s |
| marriott | 11s |
| rcg | 11s |
| united-east | 21s |
| alaska-air | 10s |

## Issue 3: schwab-prod old image

**Image**: `cron-task-runner:main-20260225_210959z-9419ac08` (Feb 25)
**Expected**: `cron-task-runner:main-20260325_233344z-1028c87d` (Mar 25) or newer

The CONVI-6298 code (REINDEX_MODE=process support) is not deployed to schwab-prod. The cron job ran but only dispatched conversation reindex jobs (existing behavior), ignoring the `REINDEX_MODE=process` env var.

**Fix**: Deploy latest cron-task-runner image to schwab-prod, then re-trigger.

---

## Phase 2: Pre-2026 Backfill (2020-01-01 to 2026-01-01)

**Triggered**: 2026-03-28 ~14:27 UTC

| Cluster | Total | Completed | Failed | Notes |
|---------|-------|-----------|--------|-------|
| voice-prod | 49 | 48 | 1 | brinks-care failed (same duplicate criterion issue) |
| chat-prod | 28 | 28 | 0 | All good |
| us-west-2-prod | 97 | 97 | 0 | All good |
| us-east-1-prod | 90 | 90 | 0 | All completed cleanly, no retries |
| ap-southeast-2-prod | 4 | 4 | 0 | All good |
| eu-west-2-prod | 6 | 6 | 0 | All good |
| ca-central-1-prod | 2 | 2 | 0 | All good |
| schwab-prod | 0 | 0 | 0 | Old image — no scorecards workflows dispatched |

**Overall**: 276 workflows, 275 completed, 1 failed (brinks-care, terminated)

No heartbeat timeout issues this time — us-east-1 workflows all completed without activity retries.

### brinks-care (voice-prod) — both phases

Phase 1 scorecard: `019c6c38-65f8-769a-b328-cff8dcfdedbc`, criterion `dddd652f-db46-415a-bbbd-6999b40472fc`
Phase 2 scorecard: `01997887-5fda-704a-9780-9253d724769a`, criterion `1b65061e-5af4-4514-b55e-7ebfdf7aacad`

Both fail with `duplicate score for the same criterion` — pre-existing data quality issue in PG.
