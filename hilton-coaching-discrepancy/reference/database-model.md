# Database Models and Relationships in RetrieveCoachingEfficiencyStats API

**Date:** 2026-01-09
**Schema:** `director` (PostgreSQL)
**Purpose:** Understand how coaching efficiency data flows through database tables

---

## Table of Contents

1. [Entity Relationship Overview](#entity-relationship-overview)
2. [Core Tables](#core-tables)
3. [Relationship Details](#relationship-details)
4. [Data Flow in API](#data-flow-in-api)
5. [Join Strategy](#join-strategy)
6. [Special Cases](#special-cases)

---

## Entity Relationship Overview

```
┌────────────────────┐
│  coaching_plans    │ (1 plan has N targets, N sessions)
│  ─────────────────│
│  PK: resource_id   │
│  agent_user_id     │────┐
│  focus_criteria[]  │    │ (defines which criteria to coach)
│  usecase_id        │    │
└────────────────────┘    │
          │               │
          │ 1:N           │
          ▼               │
┌────────────────────┐    │
│  coaching_sessions │    │ (1 session for a plan, can have multiple criteria)
│  ─────────────────│    │
│  PK: resource_id   │    │
│  coaching_plan_id  │────┘ FK (optional)
│  agent_user_id     │
│  focus_criteria[]  │────┐ (specific criteria for this session)
│  manager_submitter │    │
│  session_at        │    │
│  deleted_at        │    │
└────────────────────┘    │
                          │
          ┌───────────────┘
          │ (exploded via UNNEST)
          │
          ▼
┌────────────────────┐         ┌───────────────────────┐
│  targets           │         │  scorecard_templates  │
│  ─────────────────│         │  ────────────────────│
│  PK: target_id     │         │  PK: resource_id      │
│  coaching_plan_id  │─ ─ ┐    │  template (jsonb)     │
│  scorecard_temp_id │────┼───▶│    └─ criteria[]      │
│  criterion_id      │    │    │  qa_score_config      │
│  target (score)    │    │    └───────────────────────┘
└────────────────────┘    │              │
                          │              │
                          │              │ (defines structure)
                          │              │
                          │              ▼
┌────────────────────┐    │    ┌───────────────────────┐
│  scorecards        │    │    │  scores               │
│  ─────────────────│    │    │  ────────────────────│
│  PK: resource_id   │    │    │  PK: resource_id      │
│  conversation_id   │    │    │  scorecard_id         │────┐
│  agent_user_id     │    │    │  criterion_identifier │    │
│  template_id       │────┘    │  numeric_value        │    │
│  submitted_at      │         │  ai_scored            │    │
│  score (overall)   │◀────────│  (one per criterion)  │    │
└────────────────────┘         └───────────────────────┘    │
        │                                                    │
        │ 1:N                                                │
        └────────────────────────────────────────────────────┘
        (1 scorecard has N criterion scores)


Legend:
─────▶ : Foreign Key / Direct Reference
─ ─ ─▶ : Logical Reference (not enforced FK)
1:N    : One-to-Many Relationship
[]     : Array Field
```

---

## Core Tables

### 1. `coaching_sessions` - The Main Entity

**Purpose:** Represents a single coaching session between a coach and an agent.

**Key Fields:**

| Field | Type | Description | Nullable | API Usage |
|-------|------|-------------|----------|-----------|
| `resource_id` | varchar | PK: Session ID (ULID format) | NO | Used for grouping results |
| `agent_user_id` | varchar | Agent being coached | NO | **Critical filter** |
| `manager_submitter_user_id` | varchar | Coach who submitted | YES | Used for "group by coach" |
| `coaching_plan_id` | varchar | FK to coaching_plans | YES | Used for criterion fallback |
| `focus_criteria_ids` | text[] | Array of "template_id/criterion_id" | YES | **Exploded via UNNEST** |
| `session_at` | timestamptz | When coaching occurred | YES | **Used for before/after windows** |
| `manager_submitted_at` | timestamptz | When coach submitted | YES | **MUST NOT BE NULL** (filter) |
| `deleted_at` | timestamptz | Soft delete timestamp | YES | **MUST BE NULL** (filter) |
| `usecase_id` | varchar | Which usecase this belongs to | YES | Required filter |

**Relationships:**
- **→ coaching_plans** (N:1): Via `coaching_plan_id`
- **→ users** (implied): Via `agent_user_id`, `manager_submitter_user_id`

**Critical Filters Applied:**
```sql
WHERE cs.manager_submitted_at IS NOT NULL  -- Only submitted sessions
  AND cs.deleted_at IS NULL                -- Exclude soft-deleted
  AND cs.usecase_id IN (?)                 -- Filter by usecase
  AND cs.session_at >= ? AND cs.session_at <= ?  -- Time range
```

---

### 2. `coaching_plans` - Coaching Goals

**Purpose:** Defines the overall coaching plan for an agent, including target criteria and behaviors.

**Key Fields:**

| Field | Type | Description | Nullable |
|-------|------|-------------|----------|
| `resource_id` | varchar | PK: Plan ID | NO |
| `agent_user_id` | varchar | Agent being coached | NO |
| `creator_user_id` | varchar | Who created the plan | NO |
| `focus_criteria_ids` | text[] | Array of "template_id/criterion_id" | YES |
| `focus_behaviors` | text[] | Behaviors to improve | YES |
| `start_date` | date | Plan start | YES |
| `end_date` | date | Plan end | YES |
| `is_active` | boolean | Plan status | YES |
| `usecase_id` | varchar | Which usecase | YES |

**Relationships:**
- **← coaching_sessions** (1:N): One plan can have multiple sessions
- **← targets** (1:N): One plan defines multiple target scores

**Example Data:**
```
Plan ID: 0199ca28-db8e-728f-a39e-d677ea6b2361
Agent: 3d0a777968e6158f
focus_criteria_ids: ['c76cbabf-ef55-446e-b08a-fb3a5060168b/f4533ce1']
```

---

### 3. `targets` - Target QA Scores

**Purpose:** Defines target QA scores for specific criteria within a coaching plan.

**Key Fields:**

| Field | Type | Description | Nullable |
|-------|------|-------------|----------|
| `target_id` | varchar | PK: Target ID | NO |
| `coaching_plan_id` | varchar | FK to coaching_plans | YES |
| `scorecard_template_id` | varchar | Which template | NO |
| `criterion_or_chapter_id` | varchar | Which criterion | NO |
| `target` | float8 | Target score to achieve | YES |
| `qa_score` | float8 | Current/actual score | YES |
| `usecase_id` | varchar | Which usecase | YES |

**Relationships:**
- **→ coaching_plans** (N:1): Via `coaching_plan_id`
- **→ scorecard_templates** (N:1): Via `scorecard_template_id`

**Role in API:**
- **Fallback criterion resolver** when `coaching_sessions.focus_criteria_ids = []`
- Provides the COALESCE logic:
  ```sql
  COALESCE(
      NULLIF(split_part(cs.focus_criteria_ids[i], '/', 2), ''),
      t.criterion_or_chapter_id  ← Falls back to this
  )
  ```

---

### 4. `scorecard_templates` - Criterion Definitions

**Purpose:** Defines the structure and configuration of scorecards used for QA evaluation.

**Key Fields:**

| Field | Type | Description | Nullable |
|-------|------|-------------|----------|
| `resource_id` | varchar | PK: Template ID | NO |
| `revision` | varchar | Version number | NO |
| `title` | varchar | Template name | YES |
| `template` | jsonb | **Full template structure** | YES |
| `qa_score_config` | jsonb | How to calculate QA scores | YES |
| `autoqa_triggers` | text[] | Auto QA configuration | YES |
| `usecase_ids` | text[] | Which usecases can use this | YES |
| `status` | int2 | Active/inactive status | YES |

**template JSONB Structure:**
```json
{
  "criteria": [
    {
      "identifier": "f4533ce1",
      "name": "Call Quality",
      "description": "Agent handled call professionally",
      "auto_qa": {
        "triggers": ["keyword1", "keyword2"]
      },
      "exclude_from_qa_scores": false,
      "is_outcome_criterion": false,
      "weight": 1.0
    }
  ],
  "chapters": [...]
}
```

**Relationships:**
- **← scorecards** (1:N): One template used by many scorecards
- **← targets** (1:N): One template has many target criteria

**API Usage:**
- Determines if criterion is **auto** vs **manual** (affects conversation count threshold)
- Determines if criterion is **evaluable** (`exclude_from_qa_scores` flag)
- Categorizes criteria as **PERFORMANCE** vs **OUTCOME**

---

### 5. `scorecards` - QA Evaluation Instances

**Purpose:** Represents a single QA evaluation of a conversation using a scorecard template.

**Key Fields:**

| Field | Type | Description | Nullable |
|-------|------|-------------|----------|
| `resource_id` | varchar | PK: Scorecard ID | NO |
| `conversation_id` | varchar | Which conversation was scored | NO |
| `agent_user_id` | varchar | Agent being evaluated | NO |
| `template_id` | varchar | FK to scorecard_templates | NO |
| `template_revision` | varchar | Version used | NO |
| `creator_user_id` | varchar | Who created the scorecard | NO |
| `submitter_user_id` | varchar | Who submitted it | YES |
| `submitted_at` | timestamptz | **When scorecard was submitted** | YES |
| `score` | float8 | **Overall scorecard score** | YES |
| `ai_scored_at` | timestamptz | When AI scored it | YES |
| `manually_scored` | boolean | Manual vs AI scoring | YES |
| `usecase_id` | varchar | Which usecase | YES |

**Relationships:**
- **→ scorecard_templates** (N:1): Via `template_id`
- **← scores** (1:N): One scorecard has multiple criterion scores
- **→ conversations** (implied): Via `conversation_id`

**API Usage:**
- Used by `RetrieveQAScoreStats` (called internally by efficiency API)
- Filtered by `submitted_at` to match before/after time windows
- Filtered by `agent_user_id` to match coaching session agent

---

### 6. `scores` - Individual Criterion Scores

**Purpose:** Stores the score for each individual criterion within a scorecard.

**Key Fields:**

| Field | Type | Description | Nullable |
|-------|------|-------------|----------|
| `resource_id` | varchar | PK: Score ID | NO |
| `scorecard_id` | varchar | FK to scorecards | NO |
| `criterion_identifier` | varchar | **Which criterion** (matches template) | NO |
| `numeric_value` | float8 | **Human-scored value** | YES |
| `ai_value` | float8 | AI-suggested value | YES |
| `text_value` | varchar | Text response (for text criteria) | YES |
| `not_applicable` | boolean | Was N/A selected | YES |
| `ai_scored` | boolean | Was this AI-scored | YES |
| `auto_failed` | boolean | Auto-fail detected | YES |
| `usecase_id` | varchar | Which usecase | YES |

**Relationships:**
- **→ scorecards** (N:1): Via `scorecard_id`

**API Usage:**
- Provides criterion-level scores for efficiency calculation
- Aggregated across conversations for before/after QA scores
- The actual value used in efficiency formula

---

## Relationship Details

### Join Path: Sessions → QA Scores

The API performs these joins to get from coaching sessions to QA scores:

```sql
-- Step 1: Get coaching sessions with criteria
coaching_sessions cs
LEFT JOIN LATERAL UNNEST(cs.focus_criteria_ids)
  AS focus_criteria_template_criterion_id ON true

-- Step 2: Join to coaching plan for fallback
LEFT JOIN coaching_plans cp ON
  cs.coaching_plan_id = cp.resource_id AND
  cs.customer = cp.customer AND
  cs.profile = cp.profile

-- Step 3: Join to targets for criterion resolution
LEFT JOIN targets t ON
  t.coaching_plan_id = cp.resource_id AND
  t.customer = cp.customer AND
  t.profile = cp.profile

-- Result: One row per (session, criterion) pair
-- Used to determine which QA scores to fetch
```

Then separately (via internal API call to `RetrieveQAScoreStats`):

```sql
-- Step 4: Get QA scores for the agent/criterion
scorecards sc
INNER JOIN scores s ON
  s.scorecard_id = sc.resource_id
WHERE
  sc.agent_user_id = ?
  AND sc.template_id = ?
  AND s.criterion_identifier = ?
  AND sc.submitted_at >= ? AND sc.submitted_at <= ?

-- Aggregated to get average score and conversation count
```

---

### Cardinality Examples

#### Example 1: Session with 2 Focus Criteria

```
coaching_session:
  resource_id: abc123
  focus_criteria_ids: ['template1/criterion1', 'template1/criterion2']

After UNNEST and JOINs:
  Row 1: session=abc123, template=template1, criterion=criterion1
  Row 2: session=abc123, template=template1, criterion=criterion2

Efficiency calculation:
  Each row gets its own before/after QA scores
  Each row contributes separately to aggregation
```

#### Example 2: Session with Empty Focus Criteria (Hilton Case)

```
coaching_session:
  resource_id: 019aeb27...
  focus_criteria_ids: []  ← Empty!
  coaching_plan_id: 0199ca28...

coaching_plan:
  resource_id: 0199ca28...
  focus_criteria_ids: ['c76cbabf.../f4533ce1']

targets:
  coaching_plan_id: 0199ca28...
  scorecard_template_id: c76cbabf...
  criterion_id: f4533ce1

After UNNEST (produces 0 rows from empty array):
  UNNEST([]) → (no rows)

But LEFT JOIN to targets still exists:
  Row 1: session=019aeb27..., template=c76cbabf..., criterion=f4533ce1
         ↑ From targets.criterion_or_chapter_id via COALESCE fallback

Efficiency calculation:
  Uses criterion f4533ce1 from the coaching plan targets
  Session still contributes to coach's overall efficiency
```

---

## Data Flow in API

### Phase 1: Query Coaching Sessions

```
INPUT: API Request
  - parent: "customers/hilton/profiles/voice"
  - filterByTimeRange: Dec 1-6, 2025
  - filterByAttribute:
      usecaseNames: ["...voice-hgv-marketing"]
      users: [coach or agents]
  - groupByAttributeTypes: ["COACHING_SESSION_SUBMITTER"]

QUERY: director.coaching_sessions
  JOIN director.coaching_plans
  JOIN director.targets

OUTPUT: List<coachingSessionCriterion>
  [
    {
      agent_user_id: "49e2249fad1a2d54",
      session_id: "019ae12d...",
      session_at: "2025-12-02 22:27:00",
      template_id: "c76cbabf...",
      criterion_id: "f4533ce1",
      submitter_id: "32b969442598f06"
    },
    ...
  ]
```

### Phase 2: Fetch QA Scores

For each unique `(agent, criterion, session_date)`:

```
CALL: RetrieveQAScoreStats API

BEFORE WINDOW:
  - Start: session_date - 8 days
  - End: session_date - 1 day

AFTER WINDOW:
  - Start: session_date - 1 day
  - End: session_date + 6 days

QUERY (internally):
  SELECT
    AVG(s.numeric_value) as score,
    COUNT(DISTINCT sc.conversation_id) as conv_count
  FROM director.scorecards sc
  JOIN director.scores s ON s.scorecard_id = sc.resource_id
  WHERE
    sc.agent_user_id = ?
    AND sc.template_id = ?
    AND s.criterion_identifier = ?
    AND sc.submitted_at >= ? AND sc.submitted_at <= ?
    AND sc.submitted_at IS NOT NULL
  GROUP BY sc.agent_user_id, s.criterion_identifier

OUTPUT:
  {
    before_score: 0.80,
    before_conv_count: 15,
    after_score: 0.60,
    after_conv_count: 10
  }
```

### Phase 3: Calculate Efficiency

```
For each session-criterion:
  if before_conv_count > 0 AND
     after_conv_count >= threshold:

    efficiency = (after_score - before_score) × after_conv_count

    Example:
      efficiency = (0.60 - 0.80) × 10 = -2.0
      percentage = -2.0 / 10 = -20%
```

### Phase 4: Aggregate by Grouping

```
Group by: coach (manager_submitter_user_id)

For coach "32b969442598f06":
  Session 1: efficiency = +0.48, convs = 10
  Session 2: efficiency = -2.02, convs = 10
  Session 3: efficiency = +0.31, convs = 10
  Session 4: efficiency = +0.15, convs = 10

  Total efficiency = (+0.48 - 2.02 + 0.31 + 0.15) = -1.08
  Total convs = 40

  Avg efficiency = -1.08 / 40 = -0.027 = -2.7%

RESPONSE:
  {
    coachingEfficiencyStats: [
      {
        attribute: { users: ["coach_32b969442598f06"] },
        coachingEfficiency: -0.027,
        totalNumberOfSessions: 4,
        isEfficiencyScoreInvalid: false
      }
    ]
  }
```

---

## Join Strategy

### Why LEFT JOIN?

The API uses **LEFT JOIN** for all joins to ensure:

1. **Sessions without plans still appear** (rare, but possible)
2. **Sessions without targets still counted** (for session count)
3. **No data loss** even if foreign keys are NULL

### UNNEST with LEFT JOIN LATERAL

```sql
LEFT JOIN LATERAL UNNEST(cs.focus_criteria_ids)
  AS focus_criteria_template_criterion_id ON true
```

**Behavior:**
- If `focus_criteria_ids = ['a', 'b']` → Produces 2 rows
- If `focus_criteria_ids = []` → Produces **0 rows** from UNNEST
- But LEFT JOIN keeps the main session row, allowing targets fallback

**This is why empty `focus_criteria_ids` still works!**

### COALESCE for Criterion Resolution

```sql
COALESCE(
    NULLIF(split_part(focus_criteria_template_criterion_id, '/', 1), ''),
    t.scorecard_template_id
) AS scorecard_template_id,

COALESCE(
    NULLIF(split_part(focus_criteria_template_criterion_id, '/', 2), ''),
    t.criterion_or_chapter_id
) AS criterion_id
```

**Logic:**
1. Try to parse `focus_criteria_ids` array element: `"template_id/criterion_id"`
2. If empty string after parsing → Use `targets` table values
3. This provides **automatic fallback** to coaching plan targets

---

## Special Cases

### Case 1: Empty focus_criteria_ids (Hilton Issue)

**Data:**
```sql
coaching_sessions:
  focus_criteria_ids: []

coaching_plans:
  focus_criteria_ids: ['c76cbabf.../f4533ce1']

targets:
  criterion_or_chapter_id: 'f4533ce1'
```

**Query Result:**
- UNNEST([]) produces 0 rows
- But LEFT JOIN to targets keeps session row
- COALESCE fills in criterion from targets
- **Result:** Session uses criterion `f4533ce1` from coaching plan

**Efficiency Calculation:**
- ✅ Session is included
- ✅ QA scores fetched for criterion f4533ce1
- ✅ Efficiency calculated normally
- ✅ Contributes to coach's overall efficiency

---

### Case 2: Multiple Criteria per Session

**Data:**
```sql
coaching_sessions:
  resource_id: 'abc123'
  focus_criteria_ids: ['template1/c1', 'template1/c2', 'template1/c3']
```

**Query Result:**
- UNNEST produces **3 rows**
- Each criterion gets separate before/after QA scores
- Each criterion contributes to efficiency independently

**Efficiency Calculation:**
```
Criterion c1: (after1 - before1) × convs1 = efficiency1
Criterion c2: (after2 - before2) × convs2 = efficiency2
Criterion c3: (after3 - before3) × convs3 = efficiency3

Session efficiency = (efficiency1 + efficiency2 + efficiency3) / (convs1 + convs2 + convs3)
```

**Grouped by Session:**
- All 3 criteria rolled into single session efficiency
- `totalNumberOfSessions = 1` (not 3)

**Grouped by Criterion:**
- 3 separate results, each with `totalNumberOfSessions = 1`

---

### Case 3: Soft-Deleted Sessions

**Data:**
```sql
coaching_sessions:
  resource_id: 'xyz789'
  deleted_at: '2025-12-10 10:00:00'  ← Not NULL
```

**Query Result:**
- Filtered out by `WHERE cs.deleted_at IS NULL`
- **Completely excluded** from results
- Does **NOT** count toward `totalNumberOfSessions`
- Does **NOT** affect any efficiency calculations

---

### Case 4: Draft Sessions (Not Submitted)

**Data:**
```sql
coaching_sessions:
  resource_id: 'draft123'
  manager_submitted_at: NULL  ← Not submitted yet
```

**Query Result:**
- Filtered out by `WHERE cs.manager_submitted_at IS NOT NULL`
- **Completely excluded** from results
- Same behavior as soft-deleted sessions

---

### Case 5: Sessions Too Recent

**Data:**
```sql
coaching_sessions:
  resource_id: 'recent456'
  session_at: '2026-01-07 12:00:00'  ← 2 days ago

Today: 2026-01-09
```

**Query Result:**
- Session **IS included** in initial query
- But **skipped during QA score collection**:
  ```go
  if !isSessionAtOldEnough(c.CoachingSessionAt) {
      continue  // Need 7+ days for after scores
  }
  ```
- Result:
  - `before_score = 0`, `before_conv_count = 0`
  - `after_score = 0`, `after_conv_count = 0`
- Efficiency calculation **skipped** (conv_count == 0 condition)
- But session **still counted** in `totalNumberOfSessions`
- `isEfficiencyScoreInvalid = true`

---

## Performance Considerations

### Indexes Required

For optimal performance, these indexes are critical:

```sql
-- coaching_sessions queries
CREATE INDEX idx_coaching_sessions_lookup ON director.coaching_sessions
  (customer, profile, usecase_id, agent_user_id, session_at)
  WHERE manager_submitted_at IS NOT NULL AND deleted_at IS NULL;

-- scorecards queries (used by RetrieveQAScoreStats)
CREATE INDEX idx_scorecards_qa_lookup ON director.scorecards
  (agent_user_id, template_id, submitted_at)
  WHERE submitted_at IS NOT NULL;

-- scores queries
CREATE INDEX idx_scores_criterion_lookup ON director.scores
  (scorecard_id, criterion_identifier);

-- coaching_plans
CREATE INDEX idx_coaching_plans_lookup ON director.coaching_plans
  (customer, profile, resource_id);

-- targets
CREATE INDEX idx_targets_plan_lookup ON director.targets
  (customer, profile, coaching_plan_id);
```

### Query Optimization

1. **Session query** scans ~10-100 sessions (depends on time range)
2. **QA score queries** parallelized (up to 15 concurrent, configurable)
3. **Each QA query** scans ~10-1000 scorecards per agent/criterion/window
4. **Total QA queries** = (unique session dates) × 2 (before + after)

**Example:**
- 4 sessions on 3 different dates
- 3 dates × 2 windows = **6 QA score API calls**
- With parallelization: ~0.5-2 seconds total

---

## Summary

### Key Relationships

1. **coaching_sessions** is the anchor table (filtered by submission status, deletion, time range)
2. **coaching_plans** provides fallback criteria when session has empty `focus_criteria_ids`
3. **targets** links plans to specific scorecard criteria
4. **scorecard_templates** defines criterion properties (auto vs manual, evaluable, category)
5. **scorecards** + **scores** provide the actual QA data for before/after comparison

### Critical Design Choices

1. **UNNEST + LEFT JOIN** allows handling empty `focus_criteria_ids` arrays
2. **COALESCE fallback** ensures criterion resolution even without explicit session criteria
3. **Two-phase approach** (sessions first, then QA scores) optimizes parallel queries
4. **Soft deletes** (`deleted_at`) preserve historical data while excluding from calculations
5. **Submission gates** (`manager_submitted_at`) ensure only completed sessions are analyzed

### Common Pitfalls

1. ❌ Assuming empty `focus_criteria_ids` means no efficiency calculation
2. ❌ Forgetting that `totalNumberOfSessions` includes sessions with invalid efficiency
3. ❌ Not accounting for the 7-day minimum age requirement for sessions
4. ❌ Treating multi-criteria sessions as multiple sessions in UI
5. ❌ Filtering by `focus_criteria_ids` array length before using API

---

**Document Version:** 1.0
**Database Schema Version:** As of 2026-01-09
**Related Documents:**
- `RETRIEVE_COACHING_EFFICIENCY_STATS_API.md` - API behavior documentation
- `FINAL_ANALYSIS.md` - Hilton coaching discrepancy analysis
