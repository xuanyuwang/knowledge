# Coaching Session Generation

**Created:** 2026-03-02
**Updated:** 2026-03-03
**Status:** Endpoint implemented in `coaching-ai-summary` app

## Overview

`POST /api/create-session` endpoint in the `coaching-ai-summary` vibe-coded Flask app. Takes AI analysis text + agent context, uses OpenAI structured output to generate coaching session meeting notes, then creates the session via gRPC `CreateCoachingSession`.

## How It Works

```
Request (analysis_text, agent, focus_criteria)
  → OpenAI gpt-4o structured output → MeetingNotes (Pydantic)
  → Render to HTML
  → Build CoachingSession proto (meeting_notes, focus_criteria, session_time)
  → gRPC CreateCoachingSession
  → Return session resource name + HTML
```

### Request Body

```json
{
  "agent_user_name": "customers/cresta/users/{id}",
  "creator_user_name": "customers/cresta/users/{id}",
  "coaching_plan_name": "customers/cresta/profiles/{id}/coachingPlans/{id}",
  "usecase_name": "",
  "analysis_text": "AI-generated coaching analysis...",
  "focus_criteria": [
    {"scorecard_template_name": "...", "criterion_id": "empathy", "criterion_display_name": "Empathy"}
  ]
}
```

### Response

```json
{
  "success": true,
  "coaching_session_name": "customers/cresta/profiles/{id}/coachingSessions/{id}",
  "meeting_notes": "<h3>Overview</h3><p>...</p>..."
}
```

### MeetingNotes Schema (Pydantic → OpenAI structured output)

| Field | Type | Description |
|-------|------|-------------|
| `overview` | `str` | One-paragraph session overview |
| `discussion_points` | `list[str]` | Key topics to discuss |
| `action_items` | `list[str]` | Concrete next steps for the agent |
| `positive_recognition` | `list[str]` | What the agent is doing well |

## Key Files

| File | Repo | Description |
|------|------|-------------|
| `app/coaching-ai-summary/app.py` | `chat-ai` | Endpoint, LLM helpers, HTML renderer |
| `app/coaching-ai-summary/requirements.txt` | `chat-ai` | Dependencies (openai, pydantic, python-dotenv) |

## Proto References

| Proto | Fields Used |
|-------|-------------|
| `CoachingSession` | `agent_user_name`, `creator_user_name`, `coaching_plan_name`, `usecase_name`, `meeting_notes`, `session_time`, `manager_submitter_user_name`, `focus_criteria` |
| `FocusCriteriaInfo` | `scorecard_template_name`, `criterion_id`, `criterion_display_name` |
| `CreateCoachingSessionRequest` | `parent`, `coaching_session` |

## Related Projects

- [multi-agent-coaching-assistant](../multi-agent-coaching-assistant/) — parent project containing the full vibe-coded app

## Log History

| Date | Summary |
|------|---------|
| 2026-03-02 | Implemented endpoint: OpenAI structured output + gRPC CreateCoachingSession; deployed to voice-staging |
| 2026-03-03 | Added 4 session presets to UI: critical (2 criteria), high, medium, critical multi-criteria |
