# Agent Stats "Active Days" Fix — FULL OUTER JOIN

**Created**: 2026-02-09
**Updated**: 2026-02-09
**PR**: https://github.com/cresta/go-servers/pull/25613
**Notion Doc**: https://www.notion.so/cresta/3024a587b06180cbace3faa4fd6c8b14
**Related Project**: [convi-6192-conversation-source-config](../convi-6192-conversation-source-config/)

## Overview

After enabling `exclude_ingestion_pipeline_from_agent_stats` for Alaska Air, agents who only had source 8 (INGESTION_PIPELINE) conversations on certain days showed **"N/A"** instead of **"0"** in the Agent Leaderboard's "Active Days" metric.

## Root Cause

The `RetrieveAgentStats` ClickHouse query uses two CTEs joined together:

- `convs` — from `conversation_d`, filtered by `conversation_source in (0)` (after flag)
- `convs_with_aa` — from `conversation_with_labels_d`, no source filter (table lacks the column)

With LEFT JOIN, when `convs` has no rows for a given day (agent only had source 8 conversations), the entire row disappears — causing "N/A".

## Fix

Three changes in `retrieve_agent_stats_clickhouse.go`:

1. **SQL: LEFT JOIN → FULL OUTER JOIN** — preserves rows from both CTEs
2. **SQL: COALESCE** — handles NULLs from the outer join (`COALESCE(convs.total_agent_count, 0)`)
3. **Go: Guard condition** — `if useCrestaConversationWithAACount && row.totalAgentCount > 0` prevents source-8-only rows (where `total=0, active=1`) from incorrectly showing as "1"

### Why simpler approaches don't work

| Approach | Problem |
|----------|---------|
| RIGHT JOIN | Drops rows where agent has conversations but no AA (regression for "0" days) |
| Three-CTE with semi-join | Correct but complex; FULL OUTER JOIN + guard achieves the same result |
| Filter `convs_with_aa` by source | `conversation_with_labels_d` has no `conversation_source` column |

## Files Changed

| File | Change |
|------|--------|
| `retrieve_agent_stats_clickhouse.go` | FULL OUTER JOIN + COALESCE in SQL; `totalAgentCount > 0` guard in Go |
| `retrieve_agent_stats_test.go` | New test case for source-8-only agent; if-else → switch (lint fix) |
| 5 testdata SQL files | Updated expected SQL to match FULL OUTER JOIN + COALESCE |

## Production Verification (Alaska Air)

Tested agents `76515806a32dcf91` and `a86442ed872e344b`:

- Agent `a86442ed872e344b` on 2026-01-26: `total=0, active=1` in FULL OUTER JOIN (was missing entirely with LEFT JOIN)
- The Go guard correctly maps this to display "0" instead of "1"

## CSM Questions Investigated (2026-02-09)

Two questions from the CSM team about today's data:

1. **Agent `a86442ed872e344b` shows 0 despite having chats**: 25 source-0 conversations exist, but only 2 rows in `conversation_with_labels_d` (both `has_agent_assistance=false`). Root cause: `cron-label-conversations` runs every 30 minutes — data hadn't been processed yet.

2. **Agent `76515806a32dcf91` shows 0 with no chats today**: Actually has 28 conversations with AA=true. Query correctly returns `total=1, active=1`. CSM may have had a timezone or UI caching issue.

## Key Learnings

- `conversation_with_labels_d` contains conversations from ALL sources (0, 7, 8) despite the SQL comment claiming "only source 0"
- `cron-label-conversations` runs every 30 minutes, causing up to 30-min delay for AA tagging
- Agent Assist tagging is based on login heartbeat overlap, not direct feature usage — leading to false positives (browser extension logins) and false negatives (heartbeat gaps, cron delay)

## Notion Documentation

Created a non-engineer-facing guide: [Agent Leaderboard – 'Active Days' Behavior Guide](https://www.notion.so/cresta/3024a587b06180cbace3faa4fd6c8b14)

Covers:
- Where Active Days appears (widget vs table)
- Display values with UI tooltips (N/A, 0, 1)
- What is Agent Assist (login detection logic)
- Known limitations (false positives, false negatives, cron delay)
- Scenario breakdowns with the `exclude_ingestion_pipeline_from_agent_stats` flag
- Real-world Alaska Air example

## Log History

| Date | Summary |
|------|---------|
| 2026-02-09 | Full investigation, fix, PR, Notion doc, production verification, CSM investigation |
