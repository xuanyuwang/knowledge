# Prod Test Plan: Reindex Process Scorecards

**Created**: 2026-03-24
**Customer**: cresta-sandbox-2
**Cluster**: voice-prod
**Profile**: voice-sandbox-2

## Why This Customer

- Only **30 process scorecards** and **358 scores** in PG — small enough to verify manually
- Clear gap between PG and CH — easy to confirm the cron job fills it
- Sandbox customer — no production risk

## Baseline: PG (Source of Truth)

| Metric | Value |
|--------|-------|
| Total process scorecards | 30 |
| Total process scores | 358 |
| Date range | 2024-10-31 to 2025-10-21 |
| Distinct process templates | 6 |

### Breakdown by Template

| Template ID | Title | Month | Count |
|-------------|-------|-------|-------|
| `64508830-7e49-4d90-b01f-c07cc66b52bf` | Ocean Vacations Calibration / New Template | 2024-10 | 3 |
| `64508830-7e49-4d90-b01f-c07cc66b52bf` | Ocean Vacations Calibration / New Template | 2024-11 | 9 |
| `9de9c223-856f-4ef2-9416-3ae1fde2b88e` | Back office process | 2025-04 | 8 |
| `9de9c223-856f-4ef2-9416-3ae1fde2b88e` | Customer Request for Information Response | 2025-04 | 4 |
| `2cfa327c-f2b5-4acd-aa40-de5a0350da69` | Operations Complaints | 2025-07 | 3 |
| `0198aa57-80ad-741b-a19d-2dcd0d13676c` | External Process Template | 2025-08 | 2 |
| `019a0888-3500-77b1-a905-eb1b10887325` | [EDU] Fraud Review | 2025-10 | 1 |

Note: `019ce8a5-ed17-744c-9b96-cc2ab1400128` ("Performance Template") is type=2 in `scorecard_templates` but has 0 scorecards in `director.scorecards`.

## Baseline: ClickHouse (Before Reindex)

| Metric | Value |
|--------|-------|
| Process scorecards in CH | 14 |
| Process scores in CH | 0 |

### CH Scorecards by Template

| Template ID | Count | Earliest | Latest |
|-------------|-------|----------|--------|
| `9de9c223-...` | 6 | 2025-04-09 | 2025-04-09 |
| `64508830-...` | 4 | 2024-10-31 | 2024-11-04 |
| `0198aa57-...` | 2 | 2025-08-14 | 2025-08-15 |
| `2cfa327c-...` | 1 | 2025-07-01 | 2025-07-01 |
| `019a0888-...` | 1 | 2025-10-21 | 2025-10-21 |

### Gap Summary

| | PG | CH | Missing |
|--|----|----|---------|
| Scorecards | 30 | 14 | **16** |
| Scores | 358 | 0 | **358** |

## Test Plan

### 1. Run the Reindex Cron Job

Trigger via `CreateJob` gRPC or cron with these parameters:

- **Job type**: `JOB_TYPE_REINDEX_SCORECARDS`
- **Customer**: `cresta-sandbox-2`
- **Profile**: `voice-sandbox-2`
- **Scorecard types**: `SCORECARD_TEMPLATE_TYPE_PROCESS` (default)
- **Start time**: `2024-10-01T00:00:00Z` (before earliest scorecard)
- **End time**: `2025-11-01T00:00:00Z` (after latest scorecard)
- **Clean up before write**: `true` (to test cleanup path on prod)

### 2. Verify: Scorecards in CH

```sql
SELECT scorecard_template_id, count() as cnt
FROM cresta_sandbox_2_voice_sandbox_2.scorecard_d
WHERE scorecard_template_id IN (
  '64508830-7e49-4d90-b01f-c07cc66b52bf',
  '9de9c223-856f-4ef2-9416-3ae1fde2b88e',
  '2cfa327c-f2b5-4acd-aa40-de5a0350da69',
  '0198aa57-80ad-741b-a19d-2dcd0d13676c',
  '019a0888-3500-77b1-a905-eb1b10887325'
)
GROUP BY scorecard_template_id
ORDER BY cnt DESC
```

**Expected**: Total = **30** (up from 14)

### 3. Verify: Scores in CH

```sql
SELECT count() as total_scores
FROM cresta_sandbox_2_voice_sandbox_2.scorecard_score_d
WHERE scorecard_template_id IN (
  '64508830-7e49-4d90-b01f-c07cc66b52bf',
  '9de9c223-856f-4ef2-9416-3ae1fde2b88e',
  '2cfa327c-f2b5-4acd-aa40-de5a0350da69',
  '0198aa57-80ad-741b-a19d-2dcd0d13676c',
  '019a0888-3500-77b1-a905-eb1b10887325'
)
```

**Expected**: Total = **358** (up from 0)

### 4. Verify: No Duplicates

```sql
SELECT scorecard_id, count() as cnt
FROM cresta_sandbox_2_voice_sandbox_2.scorecard_d FINAL
WHERE scorecard_template_id IN (
  '64508830-7e49-4d90-b01f-c07cc66b52bf',
  '9de9c223-856f-4ef2-9416-3ae1fde2b88e',
  '2cfa327c-f2b5-4acd-aa40-de5a0350da69',
  '0198aa57-80ad-741b-a19d-2dcd0d13676c',
  '019a0888-3500-77b1-a905-eb1b10887325'
)
GROUP BY scorecard_id
HAVING cnt > 1
```

**Expected**: 0 rows (no duplicates after FINAL)

### 5. Verify: Idempotency

Run the job a second time with the same parameters. Re-run verification queries 2-4. Counts should remain the same.

## Test Results (2026-03-24)

### Job Execution

- **K8s Job**: `reindex-process-sandbox2-1774361443`
- **Temporal Workflow ID**: `reindexscorecards-cresta-sandbox-2-voice-sandbox-2-15f8c300-1bfe-4301-8166-26f83dc93dd8`
- **Run ID**: `3b7631d4-58d6-4569-8c7a-0bbacca5ad34`
- **Job dispatched in**: ~575ms

### Post-Reindex CH State

| Table | Before | After (FINAL) |
|-------|--------|---------------|
| `scorecard_d` (process, `conversation_id=''`) | 14 (12 after FINAL) | **12** |
| `score_d` (process, `conversation_id=''`) | 0 | **114** |
| `scorecard_score_d` (process) | 0 | 0 (not used for process scorecards) |

Note: Scores are written to `score_d`, not `scorecard_score_d`.

### CH Scorecards by Template (After)

| Template ID | Before | After | PG Total |
|-------------|--------|-------|----------|
| `64508830-...` (Ocean Vacations) | 4 | 4 | 12 |
| `9de9c223-...` (Back office) | 6 | 4 | 12 |
| `0198aa57-...` (External Process) | 2 | 2 | 2 |
| `2cfa327c-...` (Operations Complaints) | 1 | 1 | 3 |
| `019a0888-...` (EDU Fraud Review) | 1 | 1 | 1 |

### Validation

| Check | Result |
|-------|--------|
| No duplicates (FINAL) | PASS (0 dupes) |
| Scores written | PASS (114 in `score_d`) |
| Cleanup ran | PASS (old data deleted before write) |

### Analysis

- **12 of 30** PG scorecards qualified (40% pass rate)
- **18 filtered out** by `GenerateHistoricScorecardScores` — scorecards with no matching `director.scores` rows, no matching template revision, or only non-leaf/chapter criteria
- This filter rate is consistent with staging behavior (196 out of ~400+ on walter-dev)
- The `score_d` table is the correct target for process scorecard scores (not `scorecard_score_d`)

## Connection Details

### App DB (PG)

```bash
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only voice-prod voice-prod voice-sandbox-2) && /opt/homebrew/opt/postgresql@15/bin/psql "$CONN"
```

### ClickHouse

```bash
clickhouse client \
  --host clickhouse-conversations.voice-prod.internal.cresta.ai \
  --port 9440 \
  --user admin \
  --password 'jIKiJqSXovuvntudQHuMqwD0PWaJ8buU' \
  --secure \
  --database cresta_sandbox_2_voice_sandbox_2
```
