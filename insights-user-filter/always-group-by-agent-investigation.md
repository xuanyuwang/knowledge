# Investigation: Always GROUP BY agent_user_id in Analytics APIs

**Created:** 2026-02-26
**Updated:** 2026-02-26

## Context

To support post-query agent-only filtering (excluding managers from results), we need all 13 Analytics APIs to always query at the `agent_user_id` level. Currently, `agent_user_id` is only in `GROUP BY` when the FE requests `group_by_agent` — otherwise the query returns a single aggregated row across all agents.

The goal: always include `agent_user_id` in GROUP BY, then re-aggregate in Go after filtering out non-agent users.

## Key Pattern

All APIs share this structure:
1. `parseClickhouseGroupBy()` returns `groupByKeys` based on the request's `GroupByAttributeTypes` and `Frequency`
2. `optionalGroupBy` is built: if `len(groupByKeys) > 0`, it becomes `GROUP BY <keys>`, otherwise it's `""` (no GROUP BY)
3. Results are scanned dynamically based on `groupByKeys` — if `agentUserIDColumn` is not in `groupByKeys`, the scan doesn't read an `agentUserID` value
4. The `convertCHResponseTo*` function groups results by `groupByAttribute` (which uses the `agentUserID` field from the row struct)

## Summary Table

| # | API | Category | Aggregation Without GROUP BY | Complexity | What Needs to Change |
|---|-----|----------|-------------------------------|------------|---------------------|
| 1 | RetrieveAgentStats | B | COUNT(DISTINCT agent_user_id) across all agents (total/active agent count) | Medium | Force agent_user_id in GROUP BY; re-aggregate COUNT(DISTINCT) in Go; handle FULL OUTER JOIN merging |
| 2 | RetrieveConversationStats | B | COUNT(DISTINCT conversation_id), COUNT(DISTINCT agent_user_id), SUM(aht_sec) | Medium | Force agent_user_id; re-aggregate convo_count, user_count, aht_sec; recalculate avg AHT |
| 3 | RetrieveHintStats | B | Counts of hints sent/followed, convo_count, user_count across multiple CTEs (UNION ALL + LEFT JOIN) | High | Force agent_user_id in all sub-CTEs (hint_sent behavioral, hint_sent non-behavioral, hint_followed behavioral, hint_followed GW/KB); re-aggregate with MAX/SUM in Go; complex multi-CTE query |
| 4 | RetrieveLiveAssistStats | C | N/A -- always requires GROUP BY (hardcoded `GROUP BY %s` in inner CTEs) | High | Unique: uses manager_user_id swapped for agent_user_id in manager stats CTE; UNION ALL of agent_stats + manager_stats; forcing agent_user_id already happens but the manager CTE uses `manager_user_id AS agent_user_id` |
| 5 | RetrieveSuggestionStats | B | COUNT(DISTINCT event_id), COUNT(DISTINCT conversation_id), COUNT(DISTINCT agent_user_id) | Low | Force agent_user_id; re-aggregate counts in Go; simple single-table query |
| 6 | RetrieveSummarizationStats | B | Generated/used summarization counts, convo_count, user_count via multiple CTEs + LEFT JOIN | Medium | Force agent_user_id in conversation_stats, summarization_stats CTEs; re-aggregate SUM/MAX in Go |
| 7 | RetrieveSmartComposeStats | B | Smart compose used count, convo_count, user_count via analytics_event_d + conversation_d | Medium | Force agent_user_id in conversation_stats and smart_compose_stats CTEs; re-aggregate SUM/MAX in Go |
| 8 | RetrieveNoteTakingStats | B | Note-taking used count, convo_count, user_count via JOIN conversations + conversation_events | Medium | Force agent_user_id; re-aggregate in Go |
| 9 | RetrieveGuidedWorkflowStats | B | GW click count, convo_count, user_count via JOIN conversations + gw_conversation_events | Medium | Force agent_user_id; re-aggregate in Go |
| 10 | RetrieveKnowledgeBaseStats | B | KB search count, convo_count, user_count from conversation_event_d | Low | Force agent_user_id; re-aggregate in Go; simple single-table query |
| 11 | RetrieveKnowledgeAssistStats | A | **Always requires GROUP BY** (returns error if `len(groupByKeys) == 0`) | None | Already validates groupByKeys != 0; but may not always include agent_user_id (could be just truncated_time). Need to force agent_user_id into groupByKeys. |
| 12 | RetrieveQAScoreStats | B | Weighted score sums, conversation_count, scorecard_count via scorecard tables | Medium-High | Force agent_user_id; complex scorecard query with multiple CTEs; re-aggregate weighted_percentage_sum / weight_sum in Go; handle per-criterion grouping interaction |
| 13 | RetrieveAssistanceStats | B | convo_count, user_count, hint_followed_count via message_d + moment_annotation_d + conversation_event_d | Medium | Force agent_user_id in all sub-CTEs; re-aggregate MAX/SUM in Go |

## Category Definitions

- **Category A** (no change needed): Already always GROUP BY agent_user_id
- **Category B** (needs change): Optional GROUP BY agent_user_id; would need to force it and add Go re-aggregation
- **Category C** (complex/special): Unique query structure that makes the change non-trivial

## Detailed Analysis Per API

### 1. RetrieveAgentStats (Category B, Medium)

**File:** `retrieve_agent_stats_clickhouse.go`

**Query structure:** Two CTEs (`convs` and `convs_with_aa`) both using `optionalGroupBy`, joined with `FULL OUTER JOIN USING (groupByKeys)`.

**Without agent_user_id GROUP BY:** Returns a single row with `COUNT(DISTINCT agent_user_id)` for total agents and active agents.

**With agent_user_id:** Returns one row per agent per time period.

**Re-aggregation needed:**
- `totalAgentCount` / `activeAgentCount` become per-agent (0 or 1), need to SUM across agents after filtering
- The `FULL OUTER JOIN` logic complicates things -- both CTEs must include agent_user_id in GROUP BY and JOIN keys

**Go changes:** After filtering, re-sum the agent counts. The existing `convertAgentStatsFromConvWithLabelsToAgentStatsResponse` already handles per-agent rows, so mainly need to handle the case where caller did NOT request group_by_agent but we still have per-agent rows.

### 2. RetrieveConversationStats (Category B, Medium)

**File:** `retrieve_conversation_stats_clickhouse.go`

**Query structure:** Two CTEs (`conv_with_aa` for conversation IDs, `conv_with_aht` for per-conversation AHT from message_d), joined, then aggregated with `optionalGroupBy`.

**Without agent_user_id GROUP BY:** Returns single row: total convo_count, total user_count, total aht_sec.

**Re-aggregation needed:**
- SUM convo_count across agents (but beware: same conversation can have multiple agents, so `COUNT(DISTINCT conversation_id)` per agent then SUM would over-count)
- **Correctness concern:** When grouping by agent_user_id, a conversation can appear under multiple agents. Summing per-agent conversation counts will be > actual distinct conversation count. Need `COUNT(DISTINCT conversation_id)` across all agents in Go.
- `aht_sec` is SUM of per-conversation AHT -- can SUM across agents
- `user_count` is `COUNT(DISTINCT agent_user_id)` -- just count remaining agents after filter

**This is a significant correctness concern for conversation-level metrics.**

### 3. RetrieveHintStats (Category B, High)

**File:** `retrieve_hint_stats_clickhouse.go`

**Query structure:** Very complex. Multiple sub-CTEs:
- `hint_sent` CTE: UNION ALL of behavioral hints (from moment_annotation_d) + non-behavioral hints (from action_annotation_d), each with `optionalGroupBy`
- `combined_hint_sent`: MAX aggregation of hint_sent
- `hint_followed` CTE: UNION ALL of behavioral hint followed + GW/KB hint followed
- Final: LEFT JOIN combined_hint_sent with hint_followed

**Additional supported GROUP BY keys:** `behaviorIDColumn`, `policyIDColumn` (beyond agent_user_id + truncated_time).

**Re-aggregation needed:** All sub-CTEs use `optionalGroupBy`. Would need to force agent_user_id in all 4+ sub-queries, then re-aggregate MAX/SUM in Go.

### 4. RetrieveLiveAssistStats (Category C, High)

**File:** `retrieve_live_assist_stats_clickhouse.go`

**Query structure:** UNIQUE -- does NOT use `optionalGroupBy` for the inner CTEs. The inner `raised_hands_and_whispers` CTE always groups by `conversation_id, agent_user_id, manager_user_id`. The outer CTEs (`agent_live_assist_stats` and `manager_live_assist_stats`) always use `GROUP BY %s` (using `groupBy = concatKeys(groupByKeys)`).

**Special complication:** The manager stats CTE swaps `manager_user_id` into the `agent_user_id` column position:
```go
aaGroupByQueriesForManagerStats := strings.ReplaceAll(aaGroupByQueries, agentUserIDColumn, fmt.Sprintf("%s AS %s", "manager_user_id", agentUserIDColumn))
```

Then does `UNION ALL` of agent stats + manager stats. So the same "agent_user_id" column holds either an actual agent or a manager depending on the CTE.

**This makes "always GROUP BY agent_user_id" inherently tricky** -- the column is already used for two different purposes. If `groupByKeys` is empty, the inner CTEs `GROUP BY %s` would be `GROUP BY ""` which is invalid.

**Status:** If the FE never calls LiveAssistStats without group_by_agent (likely), this may not need change. But if it does, the UNION ALL + dual-purpose agent_user_id column makes this the hardest API to adapt.

### 5. RetrieveSuggestionStats (Category B, Low)

**File:** `retrieve_suggestion_stats_clickhouse.go`

**Query structure:** Simple single-table query on `conversation_event_d`. Just SELECT with `optionalGroupBy`.

**Re-aggregation needed:**
- SUM total_suggestions_count across agents
- COUNT(DISTINCT conversation_id) concern (same as ConversationStats -- conversations shared by agents)
- Re-count user_count after filtering

### 6. RetrieveSummarizationStats (Category B, Medium)

**File:** `retrieve_summarization_stats_clickhouse.go`

**Query structure:** Multiple CTEs:
- `conversation_stats`: convo_count + user_count from conversation_d with `optionalGroupBy`
- `convo_with_valid_summarization`: DISTINCT conversation_id, agent_user_id from action_annotation_d
- `convo_with_used_summarization`: DISTINCT conversation_id, agent_user_id from conversation_event_d
- `summarization_stats`: JOIN of valid + used, with `optionalGroupBy`
- Final: LEFT JOIN conversation_stats + summarization_stats

**Re-aggregation needed:** Force agent_user_id in CTEs. Re-aggregate SUM/MAX in Go.

### 7. RetrieveSmartComposeStats (Category B, Medium)

**File:** `retrieve_smart_compose_stats_clickhouse.go`

**Query structure:** Multiple CTEs:
- `conversation_stats`: from conversation_d with `optionalGroupBy`
- `agent_hourly_conversation`: DISTINCT per agent (already groups by agent_user_id)
- `agent_hourly_smart_compose_stats`: already groups by agent_user_id
- `smart_compose_stats`: JOIN with `optionalGroupBy`
- Final: LEFT JOIN with `optionalGroupBy`

**Note:** Inner CTEs already have agent_user_id naturally. The outer aggregation uses `optionalGroupBy`.

### 8. RetrieveNoteTakingStats (Category B, Medium)

**File:** `retrieve_note_taking_stats_clickhouse.go`

**Query structure:** Two CTEs (conversations + conversation_events) JOINed, then aggregated with `optionalGroupBy`.

### 9. RetrieveGuidedWorkflowStats (Category B, Medium)

**File:** `retrieve_guided_workflow_stats_clickhouse.go`

**Query structure:** Same pattern as NoteTaking. Two CTEs JOINed, then `optionalGroupBy`.

**Additional GROUP BY key:** `guidedWorkflowNameColumn`.

### 10. RetrieveKnowledgeBaseStats (Category B, Low)

**File:** `retrieve_knowledge_base_stats_clickhouse.go`

**Query structure:** Simple single-table query on `conversation_event_d` with `optionalGroupBy`. Very similar to SuggestionStats.

### 11. RetrieveKnowledgeAssistStats (Category A*, None/Low)

**File:** `retrieve_knowledge_assist_stats_clickhouse.go`

**Query structure:** Three CTEs (messages, action_annotations, moment_annotations) joined via LEFT JOIN on group-by keys. Always uses `GROUP BY %s` (never optional -- requires `len(groupByKeys) != 0`).

**Special:** Returns error if `len(groupByKeys) == 0`. However, could be called with only `truncated_time` (no agent_user_id). Need to ensure agent_user_id is always injected.

**The LEFT JOIN between CTEs uses dynamically-built join conditions based on groupByKeys** (`leftJoinActionAnnotations`, `leftJoinMomentAnnotations`). Adding agent_user_id means adding it to these join conditions too.

### 12. RetrieveQAScoreStats (Category B, Medium-High)

**File:** `retrieve_qa_score_stats_clickhouse.go`

**Query structure:** Complex scorecard query:
- `conversation` CTE (optional)
- `scorecard` + `scorecard_last_update` + `filtered_scorecard` CTEs
- `scorecard_score` CTE
- Final: JOIN scorecard_score + filtered_scorecard with `optionalGroupBy`
- With moments: adds `scorecard_score_per_conversation` intermediate CTE

**Additional GROUP BY key:** `criterionIDColumn`.

**Re-aggregation needed:**
- `weighted_percentage_sum` and `weight_sum` can be SUMmed across agents
- `total_conversation_count` and `total_scorecard_count` need COUNT(DISTINCT) -- cannot just SUM per-agent counts
- **Interaction with per-criterion quintile:** When grouping by [AGENT, CRITERION], quintile ranks are per-criterion. Adding always-agent grouping interacts with the quintile logic.

### 13. RetrieveAssistanceStats (Category B, Medium)

**File:** `retrieve_assistance_stats_clickhouse.go`

**Query structure:** Three CTEs:
- `conversation_count`: from message_d with `optionalGroupBy`
- `hint_followed`: UNION ALL of moment_annotation_d + conversation_event_d, each with `optionalGroupBy`
- Final: LEFT JOIN with `optionalGroupBy`

## Cross-Cutting Concerns

### 1. COUNT(DISTINCT) Re-aggregation Problem

When forcing `GROUP BY agent_user_id`, metrics like `COUNT(DISTINCT conversation_id)` become per-agent. A conversation can have multiple agents, so summing per-agent conversation counts over-counts. **We need to track distinct conversation_ids in Go** (e.g., using a set), or accept the over-count as "agent-conversation pairs" rather than "distinct conversations."

**Affected APIs:** ConversationStats, HintStats, SuggestionStats, SummarizationStats, SmartComposeStats, NoteTakingStats, GuidedWorkflowStats, KnowledgeBaseStats, QAScoreStats, AssistanceStats

**Options:**
- A) Accept over-counting for totals (simple but slightly inaccurate)
- B) Track distinct conversation IDs in Go using a set (accurate but more memory)
- C) Run a separate query for global totals without agent GROUP BY (accurate but extra query)

### 2. user_count Re-aggregation

When grouping by agent_user_id, `COUNT(DISTINCT agent_user_id)` per row is always 1. The global `user_count` is just the count of distinct agent rows after filtering. This is straightforward.

### 3. Weighted Average Re-aggregation (QAScoreStats)

For QA scores, `weighted_percentage_sum / weight_sum` gives the average. These can be correctly re-aggregated by summing numerator and denominator separately. This works correctly.

### 4. MAX vs SUM in Multi-CTE Queries

Several APIs use `MAX(cte.count)` in the final SELECT when joining CTEs. With per-agent grouping, these become per-agent values and re-aggregation in Go should use SUM (not MAX) for totals.

### 5. Performance Impact

Always grouping by agent_user_id means more rows returned from ClickHouse. For a customer with 1000 agents and 30 days, instead of 30 rows (one per day) you get 30,000 rows. This is likely acceptable but worth noting.

## Effort Estimation

| Category | APIs | Effort Per API | Total |
|----------|------|----------------|-------|
| Low (single-table, simple aggregation) | SuggestionStats, KnowledgeBaseStats | 0.5 day | 1 day |
| Medium (multi-CTE, standard re-agg) | AgentStats, ConversationStats, SummarizationStats, SmartComposeStats, NoteTakingStats, GuidedWorkflowStats, AssistanceStats, KnowledgeAssistStats | 1 day | 8 days |
| Medium-High (scorecard complexity) | QAScoreStats | 1.5 days | 1.5 days |
| High (multi-CTE, complex join patterns) | HintStats | 1.5 days | 1.5 days |
| High (special UNION ALL + dual-purpose column) | LiveAssistStats | 2 days | 2 days |
| **Total** | **13 APIs** | | **~14 days** |

Plus:
- Shared infrastructure for re-aggregation helper functions: 1-2 days
- Testing (unit + integration): 3-4 days
- **Grand total: ~19-20 engineering days (4 weeks)**

## Alternative: Intercept at ParseUserFilterForAnalytics Level

Instead of modifying all 13 query builders, we could:
1. In `ParseUserFilterForAnalytics`, always inject `agent_user_id` into the `users` filter (WHERE clause, not GROUP BY)
2. This filters at the ClickHouse level without needing GROUP BY changes

**Pros:** Much simpler, no re-aggregation needed
**Cons:** Only works if we have the list of agent user IDs upfront (which we do from `listAgentOnly`)

**This is likely the better approach** -- filter in WHERE clause rather than GROUP BY + re-aggregate. See `convi-6247-agent-only-filter` project for this approach.

## Recommendation

**Do NOT pursue the "always GROUP BY agent_user_id" approach.** The WHERE-clause filtering approach (already being implemented in CONVI-6247) is:
- ~13x less work (add `users` filter to requests vs. rewrite 13 query builders + Go re-aggregation)
- No risk of introducing aggregation bugs
- No performance impact from increased result set sizes
- Semantically cleaner (filtering vs. query-then-discard)

The "always GROUP BY" approach would only be needed if we required **computed per-agent metrics that are not available from the WHERE-clause approach** (e.g., quintile ranking, which is separately handled).
