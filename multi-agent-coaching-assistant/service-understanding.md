# Cresta-Assistant Service — Understanding Document

**Created:** 2026-03-02
**Purpose:** Document the existing coaching assistant service architecture to inform the multi-agent extension

## Architecture Overview

```
Director Frontend → cresta-assistant (Python gRPC) → Backend Services (apiserver, insights-server, bot-server)
                                                   → LLM Proxy (streaming inference)
```

The service has **two separate workflows**:
1. **Intent Creation** — Translates intent-assistant requests to bot-server VirtualAgentService (existing, not relevant to this project)
2. **Coaching Summary** — Orchestrates data from multiple backends + LLM to generate coaching insights (our focus)

## Three-Layer Backend Design

```
┌─────────────────────────────────────────────────┐
│ gRPC Handler (cresta_assistant_service.py)       │  Routes RPC, validates auth
├─────────────────────────────────────────────────┤
│ Orchestrator (coaching_assistant.py, 1025 lines) │  12 backend wrappers, action routing,
│                                                   │  LLM streaming, conversation filtering
├─────────────────────────────────────────────────┤
│ Data Fetcher (coaching_data_fetcher.py, 1307 ln) │  Pure data shaping, callable injection,
│                                                   │  behavior resolution, evidence marking
├─────────────────────────────────────────────────┤
│ Prompt Builder (coaching_prompt_builder.py)       │  Per-action prompt templates
│ Conv. Filter  (coaching_conversation_filter.py)   │  LLM-based coachable conv selection
└─────────────────────────────────────────────────┘
```

Key design: **Dependency injection via callables.** The orchestrator wraps 12 gRPC calls as plain functions and passes them to the data fetcher via a `CoachingBackendClients` dataclass. The fetcher never imports gRPC directly.

## Request/Response Contract

### Input (AssistCoachingSummaryRequest)

```
profile              string            "customers/{id}/profiles/{id}"
chat_history         repeated Turn     Previous conversation turns
current_turn         Turn              Current user message
  └─ user_content
       ├─ text       string            User's question
       ├─ action     enum              COACHING_RECOMMENDATION | SUMMARIZE_TRENDS | ...
       └─ plan_summary_input
            ├─ scorecard_template_name  string
            ├─ criterion_id             string
            └─ message_names            repeated string
coaching_context     Context
  ├─ agent_name      string            "customers/{id}/users/{id}" (SINGLE agent)
  ├─ usecase_name    string            "sales" | "support"
  └─ plan_summary_context
       └─ coaching_plan_name  string
```

### Output (AssistCoachingSummaryResponse, streamed)

```
progress             ProgressUpdate     Loading state
assistant_content    Content
  ├─ message         {text, type}       LLM-generated text
  └─ action_result   oneof:
       ├─ plan_summary_result           Examples + message_names
       └─ agent_coaching_criteria_trends_result   CriterionTrend list
```

## The 12 Backend API Calls

The orchestrator wraps these as callables for the data fetcher:

| # | Method | Backend Service | Returns |
|---|--------|----------------|---------|
| 1 | `retrieve_coaching_progresses` | CoachingService | CriterionTrend (score, delta, target) |
| 2 | `retrieve_qa_conversations` | AnalyticsService | CoachingExample list (failed/passed) |
| 3 | `calculate_auto_scoring_evidence` | AutoQaService | evidence map: conv→criterion→messages |
| 4 | `list_conversations_library_clips` | ConversationsLibraryService | Curated positive clips |
| 5 | `retrieve_top_performer_examples` | AnalyticsService + QA | Top agent's perfect conversations |
| 6 | `retrieve_conversation_messages` | ConversationService | Full transcript with speakers |
| 7 | `list_coaching_plans` | CoachingService | Active plans with focus criteria |
| 8 | `suggest_coaching_opportunities` | CoachingService | Suggested coaching areas |
| 9 | `list_targets` | CoachingService | Org-level score targets |
| 10 | `list_current_scorecard_templates` | CoachingService | Template JSON |
| 11 | `retrieve_behaviors` | BehaviorService | Behavior definitions |
| 12 | `batch_get_moments` | MomentService | Moment definitions (intent/keyword/composite) |

## Action Type Workflows

### COACHING_RECOMMENDATION (most complex, LLM call)

```
1. Fetch voicemail moment name (for exclusion)
2. fetch_action_data_with_clients() →
   a. Retrieve criterion trends (scores, deltas)
   b. Retrieve failed QA conversations (score=0)
   c. List library clips OR top performer examples (positive)
   d. Calculate auto-scoring evidence for all conversations
   e. Resolve behavior description (scorecard → behaviors → moments)
   f. Fetch full transcripts for failed + good conversations
   g. Annotate transcripts with <<< EVIDENCE markers
3. If too many transcripts → LLM conversation filter (select most coachable)
4. Build prompt via coaching_prompt_builder
5. Stream LLM response (gpt-5.2 default)
6. Yield structured response
```

### SUMMARIZE_TRENDS (no LLM, structured data only)

```
1. Fetch ALL criteria from:
   - Active coaching plans (focus criteria)
   - Coaching opportunities (suggested)
   - Org-level targets
2. Deduplicate criteria
3. Match with progress data (scores, deltas)
4. Sort by priority (gap from target)
5. Return CriterionTrend list (no LLM needed)
```

### SHOW_EXAMPLES / EXPLAIN_EXAMPLES (no LLM)

```
1. Fetch failed QA conversations
2. Fetch library clips / top performer examples
3. Return structured CoachingExample lists
```

### CUSTOMER_EXPERIENCE_IMPACT / CHAT (LLM call, minimal data)

```
1. Fetch behavior context
2. Build prompt
3. Stream LLM response
```

## Key Data Structures

### CoachingActionData (returned by data fetcher)

```python
@dataclass
class CoachingActionData:
    positive_examples: list[CoachingExample]       # Library or top performer examples
    negative_examples: list[CoachingExample]        # Failed QA examples
    criterion_trends: list[CriterionTrend]          # Score trends
    message_names: list[str]                        # Evidence message names
    behavior_description: str                       # From moments chain
    failed_conversation_transcripts: list[dict]     # Formatted transcripts w/ evidence
    good_conversation_transcripts: list[dict]       # Formatted transcripts w/ evidence
    score_context: dict                             # {current, start, target, trajectory}
```

### CoachingBackendClients (callable injection)

```python
@dataclass
class CoachingBackendClients:
    retrieve_coaching_progresses: Callable | None
    retrieve_qa_conversations: Callable | None
    calculate_auto_scoring_evidence: Callable | None
    list_conversations_library_clips: Callable | None
    retrieve_top_performer_examples: Callable | None
    retrieve_conversation_messages: Callable | None
    list_coaching_plans: Callable | None
    suggest_coaching_opportunities: Callable | None
    list_targets: Callable | None
    list_current_scorecard_templates: Callable | None
    retrieve_behaviors: Callable | None
    batch_get_moments: Callable | None
```

## Behavior Description Resolution Chain

```
ScoreCardTemplate → find criterion item → auto_qa triggers → behavior resource names
  → BehaviorService.RetrieveBehaviors → moment references
  → MomentService.BatchGetMoments → extract moment definition
  → Formatted JSON (intent/keyword/composite patterns)
```

## Evidence System

- `CalculateConversationAutoScoringEvidence` returns which messages triggered scoring
- Messages are annotated with `<<< EVIDENCE` markers in transcript
- Message window: ±50 messages around evidence for context
- Used by LLM to understand exactly where scoring failures occurred

## QA Filter Construction

```python
QAFilter:
  time_range: Last N days (default 7)
  users: [specific agent]              ← currently single agent
  usecase_names: [use case filter]
  criterion_identifiers: [criterion]
  score_ranges: [e.g., 0.0-0.0 for failed]
  moment_groups: [exclude voicemail]
```

## Multi-Agent Extension Points

### What needs to change

1. **Proto**: `coaching_context.agent_name` → accept list of agents or team/group identifier
2. **Proto**: New action types for team-level insights
3. **Orchestrator**: Loop or batch data fetching across agents
4. **Data Fetcher**: Aggregate data across agents (trends, examples, transcripts)
5. **Prompt Builder**: Team-level prompt templates (comparisons, team patterns)
6. **QA Filter**: `users` field already accepts lists — can pass multiple agents

### What's already friendly to multi-agent

- Backend APIs mostly support batch operations (e.g., `RetrieveQAConversations` accepts agent list)
- Callable injection pattern means adding new data sources is straightforward
- Prompt templates are isolated per action — team actions can be added independently
- Conversation filter works on transcript lists — can handle cross-agent pools
- The `CoachingBackendClients` dataclass can be extended without breaking existing code

## Environment & Running Locally

```bash
# Requires VPN for staging
export API_SERVER_GRPC_ADDR="grpc-cresta-api.voice-staging.internal.cresta.ai:443"
export API_SERVER_GRPC_SECURE="true"
export INSIGHTS_SERVER_GRPC_ADDR="grpc-cresta-api.voice-staging.internal.cresta.ai:443"
export INSIGHTS_SERVER_GRPC_SECURE="true"
export BOT_SERVER_GRPC_ADDR="grpc-cresta-api.voice-staging.internal.cresta.ai:443"
export BOT_SERVER_GRPC_SECURE="true"
export LOCAL_GRPC_AUTH_ARGS="voice-staging cresta"

make start_service   # localhost:8081
grpcui -plaintext localhost:8081  # Interactive testing
```

## File Reference

| File | Lines | Purpose |
|------|-------|---------|
| `coaching_assistant.py` | 1,025 | Orchestrator: 12 backend wrappers, action routing, LLM streaming |
| `coaching_data_fetcher.py` | 1,307 | Data fetching, shaping, behavior resolution, evidence marking |
| `proto_mapper.py` | 477 | Proto mapping (for intent creation, not coaching) |
| `cresta_assistant_service.py` | 239 | gRPC servicer routing |
| `coaching_conversation_filter.py` | 190 | LLM-based conversation selection |
| `coaching_prompt_builder.py` | 155 | Per-action prompt templates |
| `clients/__init__.py` | 68 | gRPC channel creation + auth |
