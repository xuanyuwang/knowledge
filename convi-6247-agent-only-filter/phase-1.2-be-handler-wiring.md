# Phase 1.2: BE Handler Wiring — `filter_to_agents_only`

**Created:** 2026-03-13

## Problem

All 16 analytics API handlers hardcode `listAgentOnly` instead of reading it from `req.FilterToAgentsOnly`. The FE "Agents only" filter toggle has no effect on backend behavior.

| Current behavior | APIs | Default |
|-----------------|------|---------|
| Hardcoded `true` | AgentStats, ConversationStats, SuggestionStats, SummarizationStats, SmartComposeStats, NoteTakingStats, GuidedWorkflowStats, KnowledgeBaseStats, HintStats, KnowledgeAssistStats, QAScoreStats | agents only |
| Hardcoded `false` (old pattern) | AssistanceStats (legacy), CoachingSessionStats, CommentStats, ScorecardStats | include managers |
| Dynamic (done) | LiveAssistStats | `req.GetFilterToAgentsOnly()` |

## Solution

Read `filter_to_agents_only` from the request using `req.GetFilterToAgentsOnly()`. Proto3 `bool` defaults to `false` when unset. The FE filter state defaults to `true` and always sends the field, so the effective default for new FE clients is `true`.

No proto change needed — keep `bool filter_to_agents_only` as-is.

### Handler pattern

```go
// Replace hardcoded true:
listAgentOnly := req.GetFilterToAgentsOnly()
```

### Behavior matrix

| Client | Field value | `GetFilterToAgentsOnly()` | Result |
|--------|-----------|--------------------------|--------|
| New FE, filter "Yes" (default) | `true` | `true` | agents only |
| New FE, filter "No" | `false` | `false` | include managers |
| Old FE (no field) | unset | `false` | include managers (behavior change, acceptable) |

## Changes made

### 11 Go handlers updated

Directory: `go-servers/insights-server/internal/analyticsimpl/`

| File | Old | New |
|------|-----|-----|
| `retrieve_agent_stats.go:48` | `listAgentOnly := true` | `listAgentOnly := req.GetFilterToAgentsOnly()` |
| `retrieve_conversation_stats.go:43` | `listAgentOnly := true` | same |
| `retrieve_suggestion_stats.go:47` | `listAgentOnly := true` | same |
| `retrieve_summarization_stats.go:40` | `listAgentOnly := true` | same |
| `retrieve_smart_compose_stats.go:57` | `listAgentOnly := true` | same |
| `retrieve_note_taking_stats.go:47` | `listAgentOnly := true` | same |
| `retrieve_guided_workflow_stats.go:47` | `listAgentOnly := true` | same |
| `retrieve_knowledge_base_stats.go:47` | `listAgentOnly := true` | same |
| `retrieve_knowledge_assist_stats.go:41` | `listAgentOnly := true` | same |
| `retrieve_hint_stats.go:51` | `listAgentOnly := true` | same |
| `retrieve_qa_score_stats.go:75` | `listAgentOnly := true` | same |

### Unchanged

| File | Reason |
|------|--------|
| `retrieve_live_assist_stats.go` | Already uses `req.GetFilterToAgentsOnly()` |
| `retrieve_assistance_stats.go` | Legacy API, uses old 3-step pattern with `ListUsersMappedToGroups(..., false)` |
| `retrieve_coaching_session_stats.go` | Manager-only API, uses `ListUsersMappedToGroups(..., false)` |
| `retrieve_comment_stats.go` | Manager-only API |
| `retrieve_scorecard_stats.go` | Manager-only API |
