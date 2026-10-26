# Investigation: Scorecard Count Mismatch Between Performance and QM Report Pages

**Created**: 2026-04-10
**Updated**: 2026-04-10

## TL;DR — Why the Numbers Don't Match

The **Performance page** and **QM Report page** count scorecards differently because they answer different questions:

- **Performance page**: "How many scorecards exist for **conversations that happened** in this time window?"
- **QM Report page**: "How many scorecards were **submitted** in this time window?"

When a reviewer scores a conversation days after it happened, the scorecard shows up in **different time windows** on each page.

### Example

A conversation happens on **March 25**. The reviewer submits the scorecard on **April 2**.

| Page | Filter: April 1–7 | Shows this scorecard? | Why |
|------|--------------------|-----------------------|-----|
| Performance | Yes → No | **No** | It looks at when the conversation happened (March 25 — outside the window) |
| QM Report | Yes → Yes | **Yes** | It looks at when the scorecard was submitted (April 2 — inside the window) |

### What This Means in Practice

- If reviewers score conversations promptly (same day), the two pages will show **similar numbers**
- If there's a delay between conversation and scoring, the numbers will **diverge**
- Neither page is "wrong" — they measure different things
- This is separate from the data sync gap issue (missing scorecards in ClickHouse)

---

## Problem Statement

Even with the same filters (template, time range), the **Performance page** and **QM Report page** show different submitted scorecard counts. This is a separate issue from the CH sync gap investigated earlier — this is about the two APIs fundamentally querying differently.

| Page | API | Count Field |
|------|-----|-------------|
| Performance | `RetrieveQAScoreStats` | `totalScorecardCount` |
| QM Report | `RetrieveDirectorTaskStats` (type=QM) | `evaluatedScorecardCount` |

## Root Cause: Different Timestamp Columns

**This is the primary source of mismatch.**

| Aspect | RetrieveQAScoreStats (Performance) | RetrieveDirectorTaskStats (QM Report) |
|--------|-------------------------------------|---------------------------------------|
| **Data source** | **ClickHouse** (`scorecard_d` / `score_d`) | **PostgreSQL** (`director.scorecards`) |
| **Time range applied to** | **`scorecard_time`** = conversation start time* | **`submitted_at`** = scorecard submission time |
| **Time range operator** | `>= start AND < end` | `>= start AND <= end` |
| **Status filter** | `scorecard_submit_time != 0` (for MANUALLY_SUBMITTED) | `task_status IN [2, 4]` (ACTIVE, ARCHIVED director tasks) |
| **Count logic** | `COUNT(DISTINCT scorecard_id)` across CH | `len(scorecardRows)` from PG query |

*For conversation-based scorecards, `scorecard_time` = conversation start time. For process/standalone scorecards, `scorecard_time` = scorecard create time. (Source: `go-servers/apiserver/sql-schema/protos/dataplatform/coaching/scorecard.proto:45-48`)

### Example Scenario

A scorecard for a conversation that **started March 25** but was **submitted April 2**:

| Page | Time range April 1–7 | Included? | Why |
|------|----------------------|-----------|-----|
| Performance | `scorecard_time >= Apr 1` | **NO** | scorecard_time = Mar 25 (conversation start) |
| QM Report | `submitted_at >= Apr 1` | **YES** | submitted_at = Apr 2 |

Conversely, a scorecard for a conversation that **started April 3** but was **submitted April 10**:

| Page | Time range April 1–7 | Included? | Why |
|------|----------------------|-----------|-----|
| Performance | `scorecard_time >= Apr 1 AND < Apr 8` | **YES** | scorecard_time = Apr 3 |
| QM Report | `submitted_at >= Apr 1 AND <= Apr 7` | **NO** | submitted_at = Apr 10 |

## Additional Differences

### 1. Template Name Format
- **Performance**: `scorecardTemplates/019d7445-...` (template ID only)
- **QM Report**: `scorecardTemplates/019d7445-...@ec634c92` (template ID **+ revision**)

This means the QM Report page is revision-aware. If a template was revised, the two pages could be querying different sets of scorecards.

### 2. Query Path
- **Performance**: Queries CH directly — all scorecards matching template + time range + status
- **QM Report**: Goes through **director tasks** first (`task_status IN [2, 4]`), then finds scorecards linked to those tasks. A scorecard not linked to any active/archived director task would be excluded.

### 3. Time Range Boundary
- Performance: `>= start AND < end` (exclusive end)
- QM Report: `>= start AND <= end` (inclusive end)

The example requests show this in the endTimestamp values — both use `T03:59:59.999Z` pattern, but the operators differ.

### 4. Data Freshness
- CH has a sync pipeline from PG, so there's inherent lag
- Additionally, the Brinks sync gap (116 missing scorecards before Mar 30) compounds this

## Code References

### RetrieveQAScoreStats (Performance)
- **Proto**: `cresta-proto/cresta/v1/analytics/analytics_service.proto:3156-3220`
- **Handler**: `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go:49-160`
- **CH query builder**: `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go:136-186`
- **Time range parsing**: `go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go:476-495`
- **Column mapping**: `go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go:1312-1362`
  - `scoreTable` / `scorecardTable` → `scorecard_time`
  - `scorecardScoreTable` → `conversation_formatted_time`
- **Status filter**: `go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go:564-608`
  - `MANUALLY_SUBMITTED` → `scorecard_submit_time != 0`
  - `DRAFT` → `scorecard_submit_time = 0 AND manually_scored = true`
  - `AUTO` → `scorecard_submit_time = 0 AND manually_scored = false`

### RetrieveDirectorTaskStats (QM Report)
- **Proto**: `cresta-proto/cresta/v1/analytics/analytics_service.proto:25-62`
- **Handler**: `go-servers/insights-server/internal/analyticsimpl/retrieve_director_task_stats.go:14-36`
- **QM stats**: `go-servers/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go`
  - Time range: line 308 → `WHERE submitted_at >= ? AND submitted_at <= ?`
  - Task status: line 265-266 → `WHERE task_status IN ?`
  - Count: line 753 → `len(scorecardRows)`

### scorecard_time Definition
- **Proto**: `go-servers/apiserver/sql-schema/protos/dataplatform/coaching/scorecard.proto:45-48`
- **Sync logic**: `go-servers/shared/clickhouse/conversations/conversation.go:3011-3019`
  - Conversation-based: `scorecardTime = conversation.StartedAt`
  - Process/standalone: `scorecardTime = scorecard.CreatedAt`

## Summary

The mismatch is **by design** — the two APIs serve different purposes:
- **Performance page** answers: "For conversations in this time window, what are the QA scores?"
- **QM Report page** answers: "For scorecards submitted in this time window, what is the task completion?"

When scorecards are submitted promptly (same day as conversation), the counts will be close. When there's a significant delay between conversation and submission, the counts diverge.

## Potential Resolution Approaches

1. **Accept as expected behavior** — document for users that the two pages measure different things
2. **Align the timestamp** — make one page configurable to use either `scorecard_time` or `submitted_at`
3. **Show both counts** — display "by conversation date" and "by submission date" on the pages
