# Task Recommendation Exploration — What Data Supports What Tasks

**Created:** 2026-03-02
**Purpose:** Analyze available backend data to determine what coaching task recommendations are feasible

## Input

The frontend will pass an **explicit list of agent_names** (e.g., the agents visible on the coaching hub). The service iterates over these agents, fetches data for each, aggregates, and generates task recommendations.

## Available Data Sources (Per Agent)

### 1. Coaching Progresses (`RetrieveCoachingProgresses`)

**What it returns per agent:**
- List of criteria with: `current_score`, `start_score`, `score_delta`, `target_score`
- Already supports `agent_user_names` as a list (can batch)

**Task signals:**

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| Declining performance | `score_delta < 0` | "Coach {agent} on {criterion} — declined {delta}% this week" |
| Below target | `current_score < target_score` | "{agent} is at {score}% on {criterion} (target: {target}%)" |
| Significant improvement | `score_delta > threshold` (e.g., >10%) | "Recognize {agent}'s {delta}% improvement on {criterion}" |
| Stagnant below target | `score_delta ≈ 0` AND `current_score < target` | "{agent} isn't improving on {criterion} — consider changing approach" |
| Hit/exceeded target | `current_score >= target_score` | "{agent} hit target on {criterion} — consider raising goal" |

### 2. Coaching Opportunities (`SuggestCoachingOpportunities`)

**What it returns per agent:**
- Focus criteria with `coaching_reason` (system-generated explanation)
- Already supports `agent_user_names` list

**Task signals:**

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| New opportunity | Opportunity exists without active plan | "System suggests coaching {agent} on {criterion}: {reason}" |

### 3. Coaching Plans (`ListCoachingPlans`)

**What it returns per agent:**
- Active plans with focus criteria
- Supports `agent_user_names` list

**Task signals:**

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| No active plan | Agent has no coaching plan | "Create a coaching plan for {agent}" |
| Plan not working | Active plan exists but criterion declining | "Review {agent}'s plan on {criterion} — not improving" |
| Plan complete | Active plan and criterion hit target | "Close out {agent}'s plan on {criterion} — target reached" |

### 4. Failed QA Conversations (`RetrieveQAConversations`, score=0)

**What it returns per agent:**
- List of conversations where agent scored 0 on a criterion
- Includes conversation name, message name, reason

**Task signals:**

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| High failure count | >N failures on a criterion in time window | "Review {agent}'s {count} failed conversations on {criterion}" |
| Recurring failures | Failures persisting week over week | "{agent} continues failing on {criterion} — needs intervention" |

### 5. Org-Level Targets (`ListTargets`)

**What it returns (org-wide, not per-agent):**
- Target scores per criterion
- Shared across all agents

**Task signals:**

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| Agent below org target | `current_score < org_target` | "{agent} below org target on {criterion}" |

### 6. QA Score Stats (`RetrieveQAScoreStats`, grouped by agent)

**What it returns:**
- Score aggregation per agent, per criterion
- Can compare across agents

**Task signals:**

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| Worst performer | Agent has lowest score among team | "Priority: {agent} is lowest on {criterion} ({score}%)" |
| Outlier | One agent significantly below team average | "{agent} is an outlier on {criterion} — {delta}% below team avg" |

## Cross-Agent Signals (Team-Level)

These emerge when we compare data across multiple agents:

| Signal | Condition | Potential Task |
|--------|-----------|----------------|
| Team-wide weakness | >50% of agents below target on same criterion | "Team issue: {N}/{total} agents below target on {criterion}" |
| Best practice sharing | Top agent identified | "Pair {worst_agent} with {best_agent} for {criterion} mentoring" |
| Uneven coaching load | Some agents have plans, others don't | "Coaching coverage gap: {N} agents have no active plan" |

## Recommended Task Categories

Based on the data analysis, these task categories are well-supported:

### Priority 1 — Strong data support
1. **Coach on declining criterion** (from coaching progresses: score_delta < 0)
2. **Review failed conversations** (from QA conversations: high failure count)
3. **Create coaching plan** (from coaching plans: no active plan)

### Priority 2 — Good data support
4. **Follow up on coaching opportunity** (from coaching opportunities: system-suggested)
5. **Review ineffective plan** (coaching plans + progresses: plan exists but declining)
6. **Recognize improvement** (coaching progresses: positive delta)

### Priority 3 — Cross-agent, needs aggregation
7. **Address team-wide weakness** (multiple agents below target on same criterion)
8. **Priority ranking** (sort agents by urgency across team)

## Implementation Approach

### Data Fetching Strategy

For each agent in the list, we need:
- `RetrieveCoachingProgresses` — **can batch** (accepts agent_user_names list)
- `ListCoachingPlans` — **can batch** (accepts agent_user_names list)
- `SuggestCoachingOpportunities` — **can batch** (accepts agent_user_names list)
- `ListTargets` — **single call** (org-level, shared)
- `RetrieveQAConversations` — **per agent+criterion** (need to iterate)

Efficient approach:
1. Single call to `ListTargets` (org-level)
2. Single call to `RetrieveCoachingProgresses` with all agent names
3. Single call to `ListCoachingPlans` with all agent names
4. Parallel calls to `SuggestCoachingOpportunities` per agent (or batch if supported)
5. Selective `RetrieveQAConversations` calls only for flagged agents/criteria

### Task Generation

Option A: **Rule-based** — Apply threshold rules to generate tasks (fast, deterministic)
Option B: **LLM-based** — Feed aggregated data to LLM and let it prioritize (flexible, more natural language)
Option C: **Hybrid** — Rule-based generation + LLM for prioritization and natural language formatting

### Output Format

Each task recommendation could include:
```
{
  agent_name: "customers/{id}/users/{id}",
  agent_display_name: "Alice Smith",
  task_type: "COACH_ON_CRITERION" | "REVIEW_CONVERSATIONS" | "CREATE_PLAN" | ...,
  criterion_id: "empathy",
  criterion_display_name: "Empathy",
  priority: "HIGH" | "MEDIUM" | "LOW",
  summary: "Coach Alice on Empathy — declined 15% this week, now at 62% (target: 80%)",
  supporting_data: {
    current_score: 0.62,
    target_score: 0.80,
    score_delta: -0.15,
    failed_conversation_count: 3,
  }
}
```

## Open Questions

1. **How many agents typically?** 5-10? 50? This affects whether we can batch or need to parallelize.
2. **Time window?** Current default is 7 days. Same for multi-agent?
3. **Should tasks be LLM-generated or rule-based?** LLM gives more natural language but adds latency.
4. **Should we rank tasks globally (across all agents) or group by agent?**
5. **What's the frontend expectation?** A list of cards? A streamed summary?
