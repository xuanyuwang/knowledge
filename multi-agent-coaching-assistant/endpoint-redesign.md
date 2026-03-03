# Endpoint Redesign: Vibe-Code App Pattern

**Created:** 2026-03-02
**Updated:** 2026-03-02
**Status:** Implemented

## Key Insight

The `coaching-plan-summary` app in `chat-ai` (PR #8990) does **not** call `CrestaAssistantService` from `python-ai-services`. Instead, it:

1. **Calls Cresta backend gRPC services directly** (CoachingService, AnalyticsService, etc.) from a Flask app
2. **Aggregates data** and calls OpenAI for LLM summarization
3. **Deploys as a standalone Flask app** in the `vibe-coded` namespace on `voice-staging`

This means our team coaching identifier should follow the **same pattern** — a vibe-coded Flask app that calls Cresta gRPC backends directly and does its own scoring/ranking.

## Architecture Comparison

### Current (bottle in python-ai-services)
```
team_coaching_server.py (bottle)
  → TeamCoachingIdentifier (holds gRPC stubs from env vars)
    → CoachingService gRPC
  → Returns JSON
```
**Problems:**
- Lives in `python-ai-services` repo (heavy, requires proto builds)
- Uses env-var auth (LOCAL_GRPC_AUTH_ARGS) — not compatible with vibe-code auth
- Not deployable via vibe-code workflow

### Target (Flask vibe-coded app in chat-ai) — IMPLEMENTED
```
app/coaching-ai-summary/
├── app.py                      # Flask app with REST endpoints
├── context.py                  # Copied from vibe-coded template (token auth)
├── scorer.py                   # Scoring logic + AgentRawData (pure Python)
├── models.py                   # Dataclasses
├── cache.py                    # JSON debug cache
├── values.yaml                 # K8s config
├── build_and_push.sh
├── requirements.txt
├── requirements-greyparrot.txt
└── .gitignore
```

## What Moves vs. What Stays

### Moves to chat-ai (vibe-coded app)

| Module | Reason |
|--------|--------|
| `team_coaching_models.py` → `models.py` | Pure Python, no changes |
| `team_coaching_scorer.py` → `scorer.py` | Pure Python, no changes needed |
| `team_coaching_cache.py` → `cache.py` | Pure Python, no changes |
| REST endpoint logic | Rewritten for Flask + context.py auth |
| gRPC calls | Rewritten using context.py pattern (`ctx.grpc_channel()`, `ctx.grpc_metadata`) |

### Stays in python-ai-services (or removed)

| Module | Reason |
|--------|--------|
| `team_coaching_identifier.py` | gRPC wrapper methods move into `app.py` |
| `team_coaching_data_fetcher.py` | Protocol-based DI not needed — direct gRPC calls in app.py |
| `team_coaching_server.py` (bottle) | Replaced by Flask app |
| `test_team_coaching_staging.py` | Replaced by vibe-code local dev flow |

### Tests stay in python-ai-services

The scorer and model tests are pure Python — they can stay in `python-ai-services/cresta-assistant/tests/` and also be copied to the chat-ai app. The integration tests need reworking for the new gRPC call pattern.

## REST API Design

### Authentication

Following the vibe-code pattern, auth comes from the admin console:

```
http://localhost:5000?access_token=<TOKEN>&cluster_id=voice-staging&customer_id=cresta&profile_id=walter-dev
```

The `context.py` module handles token → cookie persistence and gRPC metadata creation.

### Endpoints

#### `POST /api/identify` — Identify agents needing coaching

**Request body:**
```json
{
  "usecase_name": "",
  "agent_names": [
    "customers/cresta/users/231dd765a58558ae",
    "customers/cresta/users/c3f71720f83244cb"
  ]
}
```

Note: `profile` and `customer_id` come from the context (cookies), not the request body.

**Response:**
```json
{
  "summary": "5 agents assessed, 2 critical, 1 high, 2 low",
  "total_agents": 5,
  "assessments": [
    {
      "agent_name": "customers/cresta/users/...",
      "priority": "critical",
      "priority_score": 12.5,
      "has_active_plan": true,
      "reasons": ["Empathy: score 50% is 40% below target 90%"],
      "criterion_assessments": [...]
    }
  ],
  "cache_file": "fetch_20260302_195526.json"
}
```

#### `GET /api/users` — List agents for selection

Reuses the same pattern as coaching-plan-summary.

#### `GET /api/coaching-progress` — Get raw progress data for one agent

For drill-down after identifying which agents need coaching.

#### `GET /api/coaching-plan` — Get coaching plan for one agent

For drill-down.

#### `GET /` — UI

Start with a simple form: select agents, click "Identify", see ranked results. Can use the coaching-plan-summary UI as a starting point.

## gRPC Call Pattern (in app.py)

Instead of the `TeamCoachingIdentifier` class with its env-var based stubs, each gRPC call follows the context.py pattern:

```python
@app.route('/api/identify', methods=['POST'])
def identify():
    ctx = context.get_context()
    body = request.get_json()
    agent_names = body.get('agent_names', [])
    usecase_name = body.get('usecase_name', '')

    profile_parent = str(ProfileName(
        customer_id=ctx.customer_id,
        profile_id=ctx.profile_id,
    ))

    stub = coaching_service_pb2_grpc.CoachingServiceStub(ctx.grpc_channel())

    # 1. Fetch coaching progresses
    trends_by_agent = _fetch_coaching_progresses(stub, ctx, profile_parent, agent_names, usecase_name)

    # 2. Fetch coaching plans
    plans_by_agent = _fetch_coaching_plans(stub, ctx, profile_parent, agent_names, usecase_name)

    # 3. Fetch coaching opportunities
    opps_by_agent = _fetch_coaching_opportunities(stub, ctx, profile_parent, agent_names, usecase_name)

    # 4. Fetch org targets
    org_targets = _fetch_targets(stub, ctx, profile_parent)

    # 5. Score and rank
    result = scorer.score_team(agent_data, org_targets)

    return jsonify(result.to_dict())
```

Each `_fetch_*` function is a plain function (not a method on a class) that takes the stub and context:

```python
def _fetch_coaching_progresses(stub, ctx, profile_parent, agent_names, usecase_name):
    request = coaching_service_pb2.RetrieveCoachingProgressesRequest(
        parent=profile_parent,
        page_size=100,
    )
    request.agent_user_names.extend(agent_names)
    response = stub.RetrieveCoachingProgresses(request, metadata=ctx.grpc_metadata)
    # ... group by agent
```

## Deployment

### Bootstrap the app — DONE

```bash
cd ~/repos/chat-ai
# copy_template.sh requires gsed — done manually with macOS sed
mkdir -p app/coaching-ai-summary
cp app/vibe-coded/build_and_push.tmpl.sh app/coaching-ai-summary/build_and_push.sh
cp app/vibe-coded/context.tmpl.py app/coaching-ai-summary/context.py
cp app/vibe-coded/values.tmpl.yaml app/coaching-ai-summary/values.yaml
sed -i '' 's/sample-app/coaching-ai-summary/g' app/coaching-ai-summary/values.yaml
sed -i '' 's/sample-app/coaching-ai-summary/g' app/coaching-ai-summary/build_and_push.sh
```

### Local development — TESTED

```bash
cd app/coaching-ai-summary
~/.pyenv/versions/3.13.12/bin/python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python app.py  # runs on port 5001 (port 5000 used by AirPlay)

# Test with cookies (context.py redirects GET requests, so use direct cookies)
TOKEN=$(cresta-cli cresta-token --bearer voice-staging cresta | sed 's/^Bearer //')
curl -s -b "access_token=${TOKEN};cluster_id=voice-staging;customer_id=cresta;profile_id=walter-dev" http://localhost:5001/api/users
curl -s -X POST -b "access_token=${TOKEN};cluster_id=voice-staging;customer_id=cresta;profile_id=walter-dev" \
  -H "Content-Type: application/json" \
  -d '{"agent_names": ["customers/cresta/users/231dd765a58558ae"]}' \
  http://localhost:5001/api/identify
```

### Deploy to staging

```bash
app/coaching-ai-summary/build_and_push.sh
# → Available at: https://coaching-ai-summary.voice-staging.internal.cresta.ai
```

## Implementation Notes

### Import differences from python-ai-services

- `CustomerName` → `from cresta.v1.customer.customer_resourcename_pb2 import CustomerName` (not `crestapy.resource_name`)
- `ProfileName` → `from cresta.v1.customer.profile_resourcename_pb2 import ProfileName`
- `AgentRawData` moved into `scorer.py` with plain list fields instead of proto types

### Python version

Requires Python 3.10+ (`kw_only=True` in context.py dataclass). Using pyenv 3.13.12.

### Staging test results

- `/health` → protos_available: true
- `/api/users` → 5,560 users
- `/api/identify` (5 agents) → all LOW (sentinel scores correctly filtered)
- Cache files: `fetch_*.json` (41KB), `result_*.json` (8KB)
