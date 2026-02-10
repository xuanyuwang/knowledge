# CONVI-6192: Agent Stats "N/A" After Excluding Ingestion Pipeline

## Problem Summary

After enabling `exclude_ingestion_pipeline_from_agent_stats` for alaska-air, agent
`e1da0d058fd089c3` shows "N/A" on days where they only had source 8 (INGESTION_PIPELINE)
conversations. Expected: "0" (logged in, but no qualifying agent assist).

## Data

**Before flag** (`conversation_source in (0,8)`):
| Date       | total_agent_count | active_agent_count |
|------------|------------------|--------------------|
| 2026-02-06 | 1                | 1                  |
| 2026-02-05 | 1                | 1                  |
| 2026-02-02 | 1                | 0                  |
| 2026-01-30 | 1                | 1                  |
| 2026-01-29 | 1                | 1                  |
| 2026-01-26 | 1                | 1                  |

**After flag** (`conversation_source in (0)`):
| Date       | total_agent_count | active_agent_count |
|------------|------------------|--------------------|
| 2026-02-06 | 1                | 1                  |
| 2026-02-05 | 1                | 1                  |
| 2026-02-02 | 1                | 0                  |
| 2026-01-30 | 1                | 1                  |

Missing: 2026-01-26 and 2026-01-29 (had **only** source 8 conversations).

## Root Cause

The query has two CTEs joined with `LEFT JOIN`:

```
convs (conversation_d)  ──LEFT JOIN──>  convs_with_aa (conversation_with_labels_d)
```

**`convs` CTE** — queries `conversation_d` with `conversation_source in (0)` filter.
On dates where the agent only had source 8 conversations, this CTE returns **no rows**.
Since `convs` is the LEFT (driving) side of the join, **the entire row disappears**.

**`convs_with_aa` CTE** — queries `conversation_with_labels_d` with **no source filter**
(the table doesn't have a `conversation_source` column). Despite the SQL comment
claiming "convs_with_aa only have conversations from source 0", the table actually
contains source 8 conversations too. So `convs_with_aa` returns `active_agent_count=1`
for 2026-01-26 (from source 8 agent assist).

### Why the row disappears

```
convs (source 0 only):         No row for 2026-01-26
convs_with_aa (no source filter): Has row for 2026-01-26 (active=1)
LEFT JOIN:                       No row from left → entire row dropped
```

## Why RIGHT JOIN Does NOT Fix This

The user suggested changing `LEFT JOIN` to `RIGHT JOIN`. Here's why it fails:

### Case 1: 2026-01-26 (source 8 only, after flag)

```
convs:          NO row (no source 0 conversations)
convs_with_aa:  HAS row (active=1 from source 8 AA)
RIGHT JOIN:     Row preserved from convs_with_aa side
                total_agent_count = NULL → scanned as 0 in Go
                active_agent_count = 1
```

Since `useCrestaConversationWithAACount=true` for this customer:
`agentCount = activeAgentCount = 1` → **Display: "1"** (WRONG, expected "0")

The source 8 conversations with agent assist are still counted because
`conversation_with_labels_d` has no `conversation_source` column to filter on.

### Case 2: 2026-02-02 (source 0, no AA) — REGRESSION

```
convs:          HAS row (total=1, source 0 conversations exist)
convs_with_aa:  NO row (no conversations with has_agent_assistance=1)
RIGHT JOIN:     No row from right → ENTIRE ROW DROPPED
```

**Display: "N/A"** instead of the current correct "0". This breaks existing behavior.

### Summary: RIGHT JOIN Trade-offs

| Date       | Current (LEFT) | RIGHT JOIN | Expected |
|------------|---------------|------------|----------|
| 2026-01-26 | N/A           | 1          | 0        |
| 2026-02-02 | 0             | N/A        | 0        |

RIGHT JOIN fixes neither case correctly and introduces a regression for 2026-02-02.

FULL OUTER JOIN has the same problem for 2026-01-26 (displays "1" not "0") because
`convs_with_aa` still includes source 8 agent assist data.

## Correct Fix

The fundamental issue is that **both CTEs need to exclude source 8**, but
`conversation_with_labels_d` has no `conversation_source` column. We need:

1. **Login detection** (row existence) from ALL sources including source 8
2. **Agent assist count** excluding source 8

### Approach: Three CTEs

Add a third CTE that includes all sources for login detection, and add a
subquery to `convs_with_aa` to exclude source 8:

```sql
WITH
-- Login detection: always includes source 8 for row existence
login_convs AS (
    SELECT
        DATE_TRUNC('day', conversation_start_time + ...) AS truncated_time,
        agent_user_id
    FROM conversation_d
    WHERE conversation_source IN (0, 8)  -- always include source 8
      AND is_dev_user = 0
      AND agent_user_id <> ''
      AND ... -- time/user filters
    GROUP BY truncated_time, agent_user_id
),
-- Filtered total count (respects the flag)
convs AS (
    SELECT
        truncated_time, agent_user_id,
        COUNT(DISTINCT agent_user_id) AS total_agent_count
    FROM conversation_d
    WHERE conversation_source IN (0)  -- flag-filtered
      AND is_dev_user = 0
      AND agent_user_id <> ''
      AND ... -- time/user filters
    GROUP BY truncated_time, agent_user_id
),
-- Agent assist count, excluding source 8 via semi-join
convs_with_aa AS (
    SELECT
        truncated_time, agent_user_id,
        COUNT(DISTINCT agent_user_id) AS active_agent_count
    FROM conversation_with_labels_d
    WHERE agent_user_id <> ''
      AND has_agent_assistance = 1
      AND conversation_id IN (
          SELECT conversation_id FROM conversation_d
          WHERE conversation_source IN (0)
      )
      AND ... -- time/user filters
    GROUP BY truncated_time, agent_user_id
)
SELECT
    login_convs.truncated_time,
    login_convs.agent_user_id,
    COALESCE(convs.total_agent_count, 0) AS total_agent_count,
    COALESCE(convs_with_aa.active_agent_count, 0) AS active_agent_count
FROM login_convs
LEFT JOIN convs USING (truncated_time, agent_user_id)
LEFT JOIN convs_with_aa USING (truncated_time, agent_user_id)
```

### Expected results for 2026-01-26 after flag:

```
login_convs:  HAS row (source 8 conversations exist)
convs:        NO row (no source 0 conversations)
convs_with_aa: NO row (source 8 AA filtered out by semi-join)
Result:       total=0, active=0
agentCount:   0 (useCrestaConversationWithAACount=true)
Display:      "0" ✅
```

### Expected results for 2026-02-02:

```
login_convs:  HAS row (source 0 conversations exist)
convs:        HAS row (total=1)
convs_with_aa: NO row (no AA)
Result:       total=1, active=0
agentCount:   0 (useCrestaConversationWithAACount=true)
Display:      "0" ✅
```

### Simplification

The three-CTE approach is only needed when `includeIngestionPipeline=false`.
When `includeIngestionPipeline=true`, the current two-CTE query works fine.

So the fix is conditional:
- `includeIngestionPipeline=true`: Use current query (no change)
- `includeIngestionPipeline=false`: Use three-CTE query with semi-join filter

## Code Changes Required

File: `insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go`

In function `agentConversationWithLabelCountClickhouseQuery()`:
1. When `includeIngestionPipeline=false`:
   - Add `login_convs` CTE (source 0,8, always)
   - Keep `convs` with source (0) for filtered count
   - Add `conversation_id IN (SELECT ... WHERE conversation_source IN (0))` to `convs_with_aa`
   - Change final SELECT to use `login_convs` as the driving table
   - Use `COALESCE` for nullable joined columns
2. When `includeIngestionPipeline=true`: No change

Also update the Go row scanning to handle the COALESCE'd values (they should already
work since COALESCE returns 0 instead of NULL).

## Also Fix: Stale SQL Comment

The comment `-- convs_with_aa only have conversations from source 0` is incorrect.
`conversation_with_labels_d` contains conversations from all sources including source 8.
This should be updated to reflect reality.
