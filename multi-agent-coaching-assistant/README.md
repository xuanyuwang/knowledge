# Multi-Agent Coaching Assistant

**Created:** 2026-03-02
**Updated:** 2026-03-06
**Status:** Hackathon demo ready — mock data for 8 agents, all 5 action types, deployed to voice-staging (revision 23)

## Overview

Extend the `cresta-assistant` coaching summary service (in `python-ai-services/cresta-assistant/`) to support **multi-agent scenarios**. The service currently serves a single agent at a time — a manager selects one agent, and the AI assistant generates coaching insights for that individual. The goal is to upgrade it so it can serve a team of agents and generate **team-level** coaching outputs (e.g., team actions, cross-agent comparisons, team trends).

## Current State (Single-Agent)

The existing service has a well-structured three-layer architecture:

| Layer | File | Role |
|-------|------|------|
| gRPC Handler | `cresta_assistant_service.py` | Routes RPC, delegates to orchestrator |
| Orchestrator | `coaching_assistant.py` | Coordinates: fetch data → filter → prompt → LLM stream |
| Data Fetcher | `coaching_data_fetcher.py` | Pure data-shaping, no gRPC imports, testable |
| Prompt Builder | `coaching_prompt_builder.py` | Action-specific prompt templates |
| Conversation Filter | `coaching_conversation_filter.py` | LLM-based coachable conversation selection |

### Current Input

```
AssistCoachingSummaryRequest {
  profile: "customers/{id}/profiles/{id}"
  coaching_context: {
    agent_name: "customers/{id}/users/{id}"   ← single agent
    usecase_name: "sales"
  }
  current_turn: {
    action: COACHING_RECOMMENDATION | SUMMARIZE_TRENDS | ...
    plan_summary_input: { scorecard_template_name, criterion_id }
  }
}
```

### Current Action Types

| Action | LLM? | Output |
|--------|-------|--------|
| `COACHING_RECOMMENDATION` | Yes | Analysis + coaching advice (<300 words) |
| `CUSTOMER_EXPERIENCE_IMPACT` | Yes | Customer impact explanation |
| `CHAT` | Yes | Free-form Q&A |
| `SUMMARIZE_TRENDS` | No | Structured `CriterionTrend` data |
| `SHOW_EXAMPLES` | No | Positive/negative conversation examples |
| `EXPLAIN_EXAMPLES` | No | Examples with reasons |

## Goal (Multi-Agent)

- Accept multiple agents (or a team/group) as input
- Add new action types for team-level insights (team trends, cross-agent comparison, team coaching actions)
- Aggregate data across agents for team-level views
- Keep backward compatibility with single-agent flows

## Key Investigation Areas

See [service-understanding.md](./service-understanding.md) for detailed analysis.

## Repositories

| Repo | Path | Purpose |
|------|------|---------|
| `python-ai-services` | `cresta-assistant/services/team_coaching_*.py` | Prototype scoring logic + unit tests (reference) |
| `chat-ai` | `app/coaching-ai-summary/` | Vibe-coded Flask app (hackathon deliverable) |

## Vibe-Coded App: `coaching-ai-summary`

Deployed as a standalone Flask app in `chat-ai` following the vibe-code pattern (same as `coaching-plan-summary` PR #8990).

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/identify` | Identify agents needing coaching, ranked by priority |
| `POST` | `/api/recommend` | Generate AI recommendations from identify results |
| `POST` | `/api/recommend-direct` | Combined identify + recommend in one call |
| `POST` | `/api/create-session` | Generate AI meeting notes and create a coaching session via gRPC |
| `GET` | `/api/users` | List agents for selection |
| `GET` | `/health` | Health check |

### Mock Data (Hackathon Demo)

When agent names contain `2026hackathon-`, the app returns pre-canned responses without gRPC/LLM calls. See `mock_data.py`.

| Mock ID | Display Name | Priority |
|---------|-------------|----------|
| 2026hackathon-1 | Alice Johnson | critical |
| 2026hackathon-2 | Bob Martinez | critical |
| 2026hackathon-3 | Carol Chen | critical |
| 2026hackathon-4 | Dave Thompson | high |
| 2026hackathon-5 | Eve Williams | high |
| 2026hackathon-6 | Frank Garcia | medium |
| 2026hackathon-7 | Grace Kim | medium |
| 2026hackathon-8 | Hank Patel | low |

Demo environment: customer `cresta-sandbox-2`, profile `voice-sandbox-2`.

### Local Development

```bash
cd ~/repos/chat-ai/app/coaching-ai-summary
~/.pyenv/versions/3.13.12/bin/python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python app.py  # runs on port 5001

# Test with auth
TOKEN=$(cresta-cli cresta-token --bearer voice-staging cresta | sed 's/^Bearer //')
curl -s -b "access_token=${TOKEN};cluster_id=voice-staging;customer_id=cresta;profile_id=walter-dev" http://localhost:5001/api/users
```

### Deploy to Staging

```bash
app/coaching-ai-summary/build_and_push.sh
# → https://coaching-ai-summary.voice-staging.internal.cresta.ai
```

## Log History

| Date | Summary |
|------|---------|
| 2026-03-02 | Project created; explored codebase; implemented prototype in python-ai-services (15/15 tests); ran staging validation; created vibe-coded Flask app in chat-ai; tested against voice-staging; added `POST /api/create-session` with OpenAI structured output + gRPC `CreateCoachingSession` |
| 2026-03-03 | Scoring engine, criterion display name resolution, CORS support, context switcher |
| 2026-03-04 | Criterion display name fix, `/api/recommend-direct` endpoint, mock data for hackathon demo |
| 2026-03-05 | Mock data fixes: focus_criteria derivation, full resource name support, all 5 action types, demo env targeting |
