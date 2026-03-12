# Testing Plan: ext Table for Large User ID Lists

**Created:** 2026-03-11
**Updated:** 2026-03-12
**PR #1:** #26178 (merged — ext table infrastructure + threading through all callers)
**PR #2:** TBD (fix — always use FinalUsers via ext table when flag is on)
**Feature Flag:** `ENABLE_EXT_TABLE_FOR_USER_FILTER` (default: `false`)

## What Changed

When the flag is enabled, two things happen:

### 1. Ext table replaces IN clause for user filtering
All ClickHouse analytics queries replace:
```sql
WHERE agent_user_id IN ('id1', 'id2', ..., 'idN')
```
with:
```sql
WHERE agent_user_id IN (SELECT user_id FROM agent_filter)
```
where `agent_filter` is a ClickHouse external data table sent via the binary protocol.

### 2. All queries filter by resolved users (no more unfiltered queries)
Previously, when `ShouldQueryAllUsers=true` (root access + empty filters), the user WHERE clause was skipped entirely to avoid query size limits. This meant ClickHouse returned rows for ALL users in the table, including unknown/orphaned user IDs.

With the flag on, `ApplyUserFilterFromResult` now always passes `FinalUsers` from `ParseUserFilterForAnalytics` via the ext table. This ensures:
- Queries are filtered precisely to the resolved user set
- No more "unknown user" results from unfiltered CH queries
- The `ShouldQueryAllUsers` optimization is superseded by ext tables (ext tables have no query size limit)

## Affected APIs

All 17 Insights analytics RPCs that go through `parseClickhouseFilter`, plus 2 QA paths:

| # | RPC | File | Notes |
|---|-----|------|-------|
| 1 | RetrieveConversationStats | `retrieve_conversation_stats_clickhouse.go` | Also has direct `buildUsersConditionAndArgs` call for group branch |
| 2 | RetrieveAgentStats | `retrieve_agent_stats_clickhouse.go` | |
| 3 | RetrieveManagerStats | `retrieve_manager_stats_clickhouse.go` | |
| 4 | RetrieveScorecardStats | `retrieve_scorecard_stats_clickhouse.go` | Column rename: `agent_user_id` -> `creator_user_id` via `strings.ReplaceAll` |
| 5 | RetrieveLiveAssistStats | `retrieve_live_assist_stats_clickhouse.go` | Column rename: `agent_user_id` -> `manager_user_id` via `strings.ReplaceAll` |
| 6 | RetrieveSuggestionStats | `retrieve_suggestion_stats_clickhouse.go` | |
| 7 | RetrieveSummarizationStats | `retrieve_summarization_stats_clickhouse.go` | |
| 8 | RetrieveSmartComposeStats | `retrieve_smart_compose_stats_clickhouse.go` | |
| 9 | RetrieveAssistanceStats | `retrieve_assistance_stats_clickhouse.go` | |
| 10 | RetrieveHintStats | `retrieve_hint_stats_clickhouse.go` | |
| 11 | RetrieveKnowledgeAssistStats | `retrieve_knowledge_assist_stats_clickhouse.go` | |
| 12 | RetrieveKnowledgeBaseStats | `retrieve_knowledge_base_stats_clickhouse.go` | |
| 13 | RetrieveNoteTakingStats | `retrieve_note_taking_stats_clickhouse.go` | |
| 14 | RetrieveGuidedWorkflowStats | `retrieve_guided_workflow_stats_clickhouse.go` | |
| 15 | RetrieveMetadataValues | `retrieve_metadata_values_clickhouse.go` | |
| 16 | RetrieveAdherences | `retrieve_adherences_clickhouse.go` | |
| 17 | RetrieveClosedNonEmptyConversations | `retrieve_closed_non_empty_conversations_clickhouse.go` | |
| 18 | RetrieveQAScoreStats | `retrieve_qa_score_stats_clickhouse.go` | Uses `parseCommonConditionsForQAAttribute` + `parseScoreConditionsForQAAttribute` |
| 19 | RetrieveQAConversations | `retrieve_qa_conversations_clickhouse.go` | Same QA paths |

## Testing Phases

### Phase 1: Staging Validation (flag ON)

**Goal:** Verify all APIs return identical results with ext tables vs IN clauses.

#### Setup
1. Set `ENABLE_EXT_TABLE_FOR_USER_FILTER=true` in staging insights-server config
2. Deploy to staging

#### Test Cases

Run each test case via the Insights UI (Performance, Leaderboard, Coaching Hub, QA pages) and compare against current production (flag off).

| # | Test Case | What It Exercises | How to Verify |
|---|-----------|-------------------|---------------|
| 1 | **No user filter** (root user, no filter applied) | `ShouldQueryAllUsers=true` path — ext table IS created with all FinalUsers (PR #26250 change). Previously this skipped user filtering entirely. | Results should NOT include unknown/orphaned users. May differ from production (flag off) which returns unfiltered data. |
| 2 | **Single agent selected** | Small ext table (1 row) | Agent stats match production exactly |
| 3 | **Group filter (small team, ~10 agents)** | Group expansion -> ext table with ~10 IDs | Group stats match production |
| 4 | **Group filter (large team, 500+ agents)** | The main use case — large ext table | Stats match production. Query succeeds (would have failed with IN clause if > max_query_size) |
| 5 | **Exclude deactivated users** | Resolves to all active users (potentially thousands) | Stats match production |
| 6 | **Scorecard stats** | Tests the `strings.ReplaceAll` column rename path — ext table column is `user_id` to avoid collision | Scorecard page loads correctly, numbers match |
| 7 | **Live assist stats** | Same column rename concern (`manager_user_id`) | Live assist page loads, numbers match |
| 8 | **QA Score / QA Conversations** | Tests `parseCommonConditionsForQAAttribute` path with `tableSpecificColumnName` | QA pages load, no unknown users in results |
| 9 | **Manager view (limited access)** | ACL resolves managed agents -> ext table | Manager sees correct agent subset |
| 10 | **Agent + Group filter combined** | Both filters set — tests `FinalUsers` union behavior | Results match production (known edge case: top-level totals may include unbucketed users — pre-existing) |
| 11 | **Smart Compose stats** | Only API that merges ClickHouse + Postgres results. Both paths now receive FinalUsers. | Page loads correctly, merged results are consistent |

#### Customers to Test With

Pick customers with varying user counts to cover different scales:

| Scale | Example | Why |
|-------|---------|-----|
| Small (~50 agents) | Any small customer in staging | Baseline correctness |
| Medium (~500 agents) | Mid-size customer | Typical use case |
| Large (2000+ agents) | Large customer (e.g., Hilton, Alaska Air if available in staging) | The scenario that originally triggered the max_query_size error |

### Phase 2: Production Rollout

#### Step 1: Enable for one cluster
- Set `ENABLE_EXT_TABLE_FOR_USER_FILTER=true` on a single production cluster
- Monitor for 24-48 hours:
  - Error rates in insights-server logs (look for ext table errors)
  - Query latency p50/p95/p99 (should improve for large user counts, slight regression for very small)
  - ClickHouse query log: verify ext table queries appear correctly

#### Step 2: Enable globally
- Roll out to all production clusters
- Monitor for 1-2 weeks

#### Step 3: Remove flag
- After stable production run, remove `ENABLE_EXT_TABLE_FOR_USER_FILTER` flag
- Remove old `buildUsersConditionAndArgs` IN-clause code path
- Clean up `useExtTable` parameter from all function signatures

### Phase 3: Edge Cases to Watch

| Edge Case | Risk | What to Check |
|-----------|------|---------------|
| `ShouldQueryAllUsers=true` + ext table on | **Behavioral change** — previously no user WHERE clause, now filters by FinalUsers via ext table | Results may exclude orphaned/unknown user data that was previously included. This is intentional — the old behavior was a side effect of the query size optimization. |
| Smart Compose (ClickHouse + Postgres merge) | Low — Postgres already received FinalUsers in `ShouldQueryAllUsers=false` cases | Both data sources now filter by same user set. Verify merged results are consistent. |
| ClickHouse version mismatch | Very Low — ext tables are a long-standing CH feature | Query doesn't error on staging CH version |
| Concurrent ext tables on same connection | Low — each query gets its own context | No cross-query contamination |
| Scorecard `strings.ReplaceAll` on `user_id` | Low — `user_id` is not a substring of any other column name in the query | Scorecard queries execute correctly |

## Success Criteria

1. All 11 staging test cases pass — results should not include unknown/orphaned users
2. No new errors in insights-server logs after enabling the flag
3. Query latency does not regress for any API (and improves for large user counts)
4. Test case 1 (no user filter) may show **fewer results** than production (flag off) — this is expected because orphaned user data is now filtered out
5. The flag can be safely removed after 2 weeks of stable production
