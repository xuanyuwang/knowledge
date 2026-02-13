# RetrieveCoachingEfficiencyStats API - Complete Behavior Documentation

**File:** `insights-server/internal/analyticsimpl/retrieve_coaching_efficiency_stats.go`
**Date:** 2026-01-07

## Table of Contents

1. [Overview](#overview)
2. [Configuration Parameters](#configuration-parameters)
3. [Request Requirements](#request-requirements)
4. [Data Flow](#data-flow)
5. [Coaching Session Query Logic](#coaching-session-query-logic)
6. [QA Score Collection](#qa-score-collection)
7. [Efficiency Calculation](#efficiency-calculation)
8. [Grouping and Aggregation](#grouping-and-aggregation)
9. [Criterion Category Filtering](#criterion-category-filtering)
10. [Response Structure](#response-structure)
11. [Edge Cases and Special Behaviors](#edge-cases-and-special-behaviors)

---

## Overview

The **RetrieveCoachingEfficiencyStats API** calculates how effective coaching sessions are by comparing agent QA scores **before** and **after** coaching sessions. It measures the improvement in agent performance for specific criteria that were coached on.

**Core Concept:**
- **Before Score**: Average QA score for a criterion in the 7 days *before* the coaching session
- **After Score**: Average QA score for the same criterion in the 7 days *after* the coaching session
- **Efficiency**: `(After Score - Before Score) × Number of Conversations`

**Key Point:** This API returns **both**:
1. Session counts (`totalNumberOfSessions`)
2. Coaching efficiency scores (`coachingEfficiency`)

---

## Configuration Parameters

These environment variables control API behavior:

```go
// Line 38-42
NumberCoachingEfficiencyConcurrentQuery  = 15  // Concurrent QA score queries
NumberDaysCoachingEfficiencyQAScoreQuery = 7   // Days before/after session to query
ThresholdForAutoQACriterion              = 5   // Min conversations for auto criteria
ThresholdForManualQACriterion            = 1   // Min conversations for manual criteria
EnabledRtrieveCoachingEfficiencyStatsLog = false // Enable debug logging
```

### What These Mean

| Parameter | Default | Purpose |
|-----------|---------|---------|
| `NUMBER_DAYS_COACHING_EFFICIENCY_QA_SCORE_QUERY` | 7 | Time window size for before/after QA scores |
| `ANALYTICS_SERVICE_AUTO_CRITERION_CONV_COUNT_THRESHOLD` | 5 | Minimum conversations needed after coaching for auto criteria |
| `ANALYTICS_SERVICE_MANUAL_CRITERION_CONV_COUNT_THRESHOLD` | 1 | Minimum conversations needed after coaching for manual criteria |
| `NUMBER_COACHING_EFFICIENCY_CONCURRENT_QUERY` | 15 | Max parallel QA score queries per request |

---

## Request Requirements

### Mandatory Fields

```protobuf
message RetrieveCoachingEfficiencyStatsRequest {
  string parent = 1;                        // REQUIRED: "customers/{id}/profiles/{id}"
  Attribute filterByAttribute = 2;          // REQUIRED: Must include usecaseNames
  TimeRange filterByTimeRange = 3;          // REQUIRED if no coaching session names
  repeated AttributeType groupByAttributeTypes = 4; // REQUIRED
  Frequency frequency = 5;                  // Optional
  Metadata metadata = 6;                    // Optional
}
```

### Validation Rules (Lines 193-200)

1. **Time Range**: Required UNLESS specific coaching session names are provided
2. **Usecase**: MUST specify at least one `usecaseNames` in `filterByAttribute`
3. **Group By**: MUST specify at least one `groupByAttributeTypes`

**Example Valid Request:**

```json
{
  "parent": "customers/hilton/profiles/voice",
  "filterByAttribute": {
    "usecaseNames": ["customers/hilton/profiles/voice/usecases/voice-hgv-marketing"],
    "users": [{"name": "customers/hilton/users/32b969442598f06"}]
  },
  "filterByTimeRange": {
    "startTimestamp": "2026-01-01T05:00:00Z",
    "endTimestamp": "2026-01-08T04:59:59Z"
  },
  "groupByAttributeTypes": ["ATTRIBUTE_TYPE_COACHING_SESSION_SUBMITTER"]
}
```

---

## Data Flow

### High-Level Process (Lines 203-257)

```
1. Validate Request
   ↓
2. Parse Filters (users, groups, criteria, usecase)
   ↓
3. Query Coaching Sessions from Database
   ↓
4. For Each Session Date (in parallel):
   ├─→ Query "Before" QA Scores (7 days before session)
   └─→ Query "After" QA Scores (7 days after session)
   ↓
5. Calculate Efficiency for Each Session
   ↓
6. Group Results by Requested Attributes
   ↓
7. Return Aggregated Statistics
```

---

## Coaching Session Query Logic

### Database Query (Lines 259-430)

The API queries `director.coaching_sessions` with these filters:

#### Always Applied (Lines 279-287)

```sql
WHERE
  cs.customer = ? AND cs.profile = ?           -- Line 280
  AND cs.manager_submitted_at IS NOT NULL      -- Line 283 (CRITICAL)
  AND cs.deleted_at IS NULL                    -- Line 286 (CRITICAL)
  AND cs.usecase_id IN (?)                     -- Line 289
```

**Key Behavior:**
- ✅ **Includes**: Submitted, non-deleted sessions
- ❌ **Excludes**: Draft sessions, soft-deleted sessions

#### Optional Filters

```sql
-- Time range filter (if provided)
AND session_at >= ? AND session_at <= ?        -- Line 293

-- Agent filter (if users specified)
AND cs.agent_user_id IN (?)                    -- Line 302

-- Scorecard template filter
AND scorecard_template_id IN (?)               -- Line 308

-- Criterion filter
AND criterion_id IN (?)                        -- Line 313

-- Specific sessions filter
AND cs.resource_id IN (?)                      -- Line 318
```

### Session-Criterion Expansion (Lines 326-349)

The API **explodes** each coaching session into multiple rows (one per criterion):

```sql
LEFT JOIN LATERAL UNNEST(cs.focus_criteria_ids)
  AS focus_criteria_template_criterion_id ON true
```

**Example:**
- Session `abc123` has `focus_criteria_ids = ['template1/criterion1', 'template1/criterion2']`
- Results in **2 rows**:
  - Row 1: `session_id=abc123, criterion_id=criterion1`
  - Row 2: `session_id=abc123, criterion_id=criterion2`

---

## QA Score Collection

### Time Windows (Lines 758-773)

For each coaching session, the API queries QA scores in **two time windows**:

#### Before Session Window
```
End Date:   1 day before coaching session (end of day)
Start Date: 8 days before coaching session (7 days total)

Example:
  Coaching Session: Jan 6, 2026 @ 21:00
  Before Window:    Dec 30, 2025 23:59:59 - Jan 5, 2026 23:59:59
```

#### After Session Window
```
Start Date: 1 day before coaching session (end of day)
End Date:   6 days after coaching session (7 days total)

Example:
  Coaching Session: Jan 6, 2026 @ 21:00
  After Window:     Jan 5, 2026 23:59:59 - Jan 12, 2026 23:59:59
```

### QA Score Query (Lines 775-808)

The API calls `RetrieveQAScoreStats` with:

```protobuf
RetrieveQAScoreStatsRequest {
  parent: "customers/{id}/profiles/{id}"
  filterByTimeRange: {time_window_above}
  filterByAttribute: {
    users: [agents from sessions]
    criterionIdentifiers: [criteria from sessions]
    usecaseNames: [from request]
    scorecardTemplates: [from request]
  }
  groupByAttributeTypes: [
    QA_ATTRIBUTE_TYPE_AGENT,
    QA_ATTRIBUTE_TYPE_CRITERION
  ]
  respectTemplateQaScoreConfig: true
}
```

**Result:** Average QA score and conversation count for each agent-criterion pair.

### Score Assignment (Lines 809-839)

```go
for _, session := range coachingSessions {
    score := scoreMap[agentCriterion{
        AgentUserID: session.AgentUserID,
        CriterionID: session.CriterionID,
    }]

    if isBeforeScore {
        session.BeforeSessionScore = score.Score
        session.BeforeSessionConvCount = score.TotalConversationCount
    } else {
        session.AfterSessionScore = score.Score
        session.AfterSessionConvCount = score.TotalConversationCount
    }
}
```

**Important:** If no QA scores found, fields remain at **default value 0**.

---

## Efficiency Calculation

### Per-Session Efficiency (Lines 672-678, 977-981)

A session's efficiency is calculated **ONLY IF**:

```go
if c.BeforeSessionConvCount > 0 &&
   c.AfterSessionConvCount >= c.GetConvCountThreshold() {

    efficiency = (AfterSessionScore - BeforeSessionScore) × AfterSessionConvCount
}
```

**Conditions:**
1. ✅ **MUST** have at least 1 conversation in "before" window
2. ✅ **MUST** have at least N conversations in "after" window:
   - **Auto criteria**: ≥5 conversations
   - **Manual criteria**: ≥1 conversation

**Example Calculation:**

```
Session ID: abc123
Agent: John
Criterion: "Call Opening"

Before Score: 75.0 (from 10 conversations)
After Score: 85.0 (from 12 conversations)

Efficiency = (85.0 - 75.0) × 12 = 10.0 × 12 = 120.0
```

### Session Age Filter (Lines 920-975)

Sessions that are **too recent** are skipped entirely:

```go
func isSessionAtOldEnough(t time.Time) bool {
    sevenDaysAgo := time.Now().AddDate(0, 0, -7)
    return !t.After(sevenDaysAgo)
}
```

**Behavior:**
- Sessions **less than 7 days old** → Skip QA score collection
- Reason: Not enough time has passed to collect "after" scores

**Example:**
- Today: Jan 7, 2026
- Session: Jan 6, 2026 @ 21:00
- Age: 1 day
- Result: ❌ Skipped, no efficiency calculated

### Aggregated Efficiency (Lines 660-696)

For grouped results, efficiencies are aggregated:

```go
groupEfficiencyData := make(map[groupKey]*efficiencyData)

for _, session := range sessions {
    if session.BeforeSessionConvCount > 0 &&
       session.AfterSessionConvCount >= threshold {

        efficiency := computeSessionEfficiency(session)

        for _, groupKey := range groupKeys {
            groupEfficiencyData[groupKey].efficiency += efficiency
            groupEfficiencyData[groupKey].convCount += session.AfterSessionConvCount
            groupEfficiencyData[groupKey].sessionIDs[session.ID] = true
        }
    }
}

// Final average efficiency
for groupKey, data := range groupEfficiencyData {
    data.efficiency = data.efficiency / float64(data.convCount)
}
```

**Formula:**
```
Aggregated Efficiency = Total Efficiency / Total Conversations
```

---

## Grouping and Aggregation

### Supported Group By Options (Lines 229-257)

| AttributeType | Description | Use Case |
|---------------|-------------|----------|
| `ATTRIBUTE_TYPE_COACHING_SESSION` | Group by individual sessions | Session details view |
| `ATTRIBUTE_TYPE_COACHING_SESSION_SUBMITTER` | Group by coach/manager | Coach leaderboard |
| `ATTRIBUTE_TYPE_AGENT` | Group by agent being coached | Agent performance |
| `ATTRIBUTE_TYPE_GROUP` | Group by team/group | Team coaching stats |
| `ATTRIBUTE_TYPE_CRITERION` | Group by coaching criterion | Criterion effectiveness |
| `ATTRIBUTE_TYPE_TIME_RANGE` | Group by time period | Trend analysis |

**Multiple Grouping:** Can combine attributes (e.g., session + criterion, coach + time range).

---

## Criterion Category Filtering

### Category Determination (Lines 114-122)

Each criterion in a scorecard template is categorized:

```go
func getCriterionCategory(criterion ScorecardTemplateCriterion) CriterionCategory {
    isOutcome := criterion.IsOutcomeCriterion()
    isEvaluable := !criterion.IsExcludeFromQAScores()

    if isOutcome && !isEvaluable {
        return CRITERION_CATEGORY_OUTCOME
    }
    return CRITERION_CATEGORY_PERFORMANCE
}
```

**Categories:**
- **PERFORMANCE**: Evaluable criteria (can be scored)
- **OUTCOME**: Non-evaluable outcome criteria

### Category Filtering Logic (Lines 412-423)

When `criterionCategories` filter is specified:

```go
if len(filter.CriterionCategories) > 0 {
    criterionCategory := criterionToCategory[key]

    // 1. Skip if criterion not in scorecard template
    if !exists {
        continue
    }

    // 2. Skip if category doesn't match filter
    if !matchesCriterionCategoryFilter(criterionCategory, filter.CriterionCategories) {
        continue
    }

    // 3. Skip non-evaluable PERFORMANCE criteria
    if !criteria.IsEvaluable && criterionCategory == CRITERION_CATEGORY_PERFORMANCE {
        continue
    }
}
```

**Filtering Rules:**

| Criterion Type | Filter Request | Included? |
|----------------|----------------|-----------|
| Evaluable PERFORMANCE | `[PERFORMANCE]` | ✅ Yes |
| Non-evaluable PERFORMANCE | `[PERFORMANCE]` | ❌ No |
| Non-evaluable OUTCOME | `[OUTCOME]` | ✅ Yes |
| Evaluable OUTCOME | `[OUTCOME]` | ❌ No (doesn't exist) |
| Any criterion | `[]` (empty) | ✅ Yes |

**Impact on Sessions:**
- If a session's **all** focus criteria are filtered out → Session excluded from results
- If a session has **mixed** criteria → Only matching criteria included

---

## Response Structure

### CoachingEfficiencyStats (Lines 698-713)

```protobuf
message CoachingEfficiencyStats {
  bool isEfficiencyScoreInvalid = 1;    // true if convCount == 0
  float coachingEfficiency = 2;         // Average efficiency score
  int32 totalNumberOfSessions = 3;      // Count of sessions in this group
  google.protobuf.Timestamp lastSessionDate = 4;  // Most recent session date
  string lastSessionName = 5;           // Name of most recent session
  Attribute attribute = 6;              // Grouping attributes
}
```

### Key Fields Explained

#### `isEfficiencyScoreInvalid` (Line 703)

```go
IsEfficiencyScoreInvalid: data.convCount == 0
```

**Set to `true` when:**
- No sessions met the efficiency calculation conditions
- Sessions too recent (< 7 days old)
- Not enough conversations after coaching
- No conversations before coaching

**Set to `false` when:**
- At least one session has valid before/after scores meeting thresholds

#### `totalNumberOfSessions` (Line 705)

```go
TotalNumberOfSessions: int32(len(data.sessionIDs))
```

**Always** counts ALL sessions in the group, regardless of:
- ✅ Whether efficiency is valid
- ✅ Whether conversations exist
- ✅ Session age

**Critical Distinction:**
- `totalNumberOfSessions`: Count of coaching sessions (always populated)
- `coachingEfficiency`: Effectiveness score (may be invalid)

---

## Edge Cases and Special Behaviors

### 1. Recent Sessions (< 7 Days Old)

**Scenario:**
- Coaching session occurred **3 days ago**

**Behavior:**
- Session **excluded** from QA score queries (Line 921-923)
- Efficiency calculation **skipped**
- `isEfficiencyScoreInvalid = true`
- `totalNumberOfSessions` **still incremented**

**Why:** Not enough time to collect "after" scores (needs 7 days).

### 2. Sessions with Zero Conversations

**Scenario A: No conversations before coaching**
```
BeforeSessionConvCount = 0
AfterSessionConvCount = 10
```

**Behavior:**
- Efficiency calculation **skipped** (Line 672)
- `convCount = 0`
- `isEfficiencyScoreInvalid = true`
- `totalNumberOfSessions` still counts the session

**Scenario B: Insufficient conversations after coaching**
```
BeforeSessionConvCount = 5
AfterSessionConvCount = 3 (for auto criterion, needs 5)
```

**Behavior:**
- Same as Scenario A

### 3. Soft-Deleted Sessions

**Scenario:**
- Session has `deleted_at IS NOT NULL`

**Behavior:**
- Session **completely excluded** from query (Line 286)
- Does **NOT** appear in any results
- Does **NOT** count toward `totalNumberOfSessions`

### 4. Draft Sessions (Not Submitted)

**Scenario:**
- Session has `manager_submitted_at IS NULL`

**Behavior:**
- Session **completely excluded** from query (Line 283)
- Same effect as soft-deleted sessions

### 5. Sessions Without Focus Criteria

**Scenario:**
- Session has `focus_criteria_ids = []` or `NULL`

**Behavior:**
- Session queried from database (Lines 324-365)
- **Falls back to coaching plan's targets** (Lines 348-349)
- If coaching plan has targets: Session treated normally with criteria from targets
- If no coaching plan or no targets: Skipped during QA score collection (Line 924)
- `totalNumberOfSessions` **still incremented** (even if skipped)

**Fallback Logic (Lines 348-349):**
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

**What This Does:**
1. Tries to use focus criteria first
2. If focus criteria is empty/NULL, uses coaching plan target's criteria
3. If both are unavailable, `criterion_id` will be empty → session skipped (Line 924-926)

**Example:**
```
Session abc123:
  - focus_criteria_ids = []
  - coaching_plan_id = "plan_xyz"

Coaching Plan plan_xyz has targets:
  - template_A/empathy
  - template_A/call_opening

Result:
  ✅ Session uses target criteria (empathy, call_opening)
  ✅ Efficiency calculated for both criteria
  ✅ Session included in results
```

### 6. Multiple Criteria Per Session

**Scenario:**
- Session has `focus_criteria_ids = ['criterion1', 'criterion2']`

**Behavior:**
- Session **split** into 2 separate efficiency calculations
- Each criterion gets its own before/after QA scores
- If grouped by session: efficiencies are **summed**
- If grouped by criterion: shown separately

**Example:**
```
Session abc123:
  - Criterion1: efficiency = 50 (from 10 conversations)
  - Criterion2: efficiency = 30 (from 8 conversations)

Group by Session:
  abc123: efficiency = 80 / 18 = 4.44, totalNumberOfSessions = 1

Group by Criterion:
  Criterion1: efficiency = 50 / 10 = 5.0, totalNumberOfSessions = 1
  Criterion2: efficiency = 30 / 8 = 3.75, totalNumberOfSessions = 1
```

### 7. Category Filtering Removes All Criteria

**Scenario:**
- Session has `focus_criteria_ids = ['outcome_criterion1']`
- Request specifies `criterionCategories = [PERFORMANCE]`

**Behavior:**
- Session **completely excluded** from results
- Does **NOT** count toward `totalNumberOfSessions`
- Reason: All criteria filtered out (Lines 412-423)

### 8. Empty Result Set

**Scenario:**
- No sessions match filters

**Behavior:**
- Returns empty response (Line 218)
- `stats = []`

### 9. Manager Submitter vs Creator

**Code (Line 347):**
```sql
COALESCE(cs.manager_submitter_user_id, cs.creator_user_id) AS manager_user_id
```

**Behavior:**
- Uses `manager_submitter_user_id` if available
- Falls back to `creator_user_id` if not
- This determines who gets credited when grouping by submitter

---

## Performance Considerations

### Concurrent Queries (Lines 933-952)

The API parallelizes QA score queries by session date:

```go
guard := make(chan struct{}, *NumberCoachingEfficiencyConcurrentQuery) // Max 15

for date, sessions := range sessionsGroupByDate {
    go func() {
        guard <- struct{}{}        // Acquire slot

        // Query before scores
        retrieveQAScoreStatsForSameDayAgentCriterion(...)

        // Query after scores
        retrieveQAScoreStatsForSameDayAgentCriterion(...)

        <-guard                   // Release slot
    }()
}
```

**Optimization:**
- Up to **15 parallel QA score queries** (configurable)
- Each date gets 2 queries (before + after)
- For 10 session dates: 20 total queries, max 15 parallel

---

## Common Misunderstandings

### ❌ "This API only returns sessions with valid efficiency"

**Wrong.** The API returns **ALL matching sessions** with `totalNumberOfSessions`, but marks efficiency as invalid when conditions aren't met.

### ❌ "isEfficiencyScoreInvalid means the session is bad"

**Wrong.** It means we **can't calculate efficiency yet** (usually due to timing), not that the coaching was ineffective.

### ❌ "Sessions without focus criteria are always excluded"

**Wrong.** Sessions without focus criteria **fall back to coaching plan targets**. They're only excluded if the coaching plan has no targets or no associated coaching plan.

### ❌ "Category filtering affects session counts"

**Partially correct.** If **ALL** criteria in a session are filtered out, the session is excluded. But if **some** criteria match, the session is included.

### ❌ "Sessions older than 7 days have invalid efficiency"

**Wrong.** Sessions **LESS than 7 days old** have invalid efficiency. Older sessions are better (more time to collect data).

---

## Summary Table

| Condition | Included in Query? | Counts Toward Sessions? | Has Efficiency Score? |
|-----------|-------------------|------------------------|----------------------|
| Submitted, not deleted, recent (< 7 days) | ✅ Yes | ✅ Yes | ❌ No (too recent) |
| Submitted, not deleted, old enough | ✅ Yes | ✅ Yes | ✅ Maybe (if has QA scores) |
| Draft (not submitted) | ❌ No | ❌ No | ❌ No |
| Soft-deleted | ❌ No | ❌ No | ❌ No |
| No focus criteria, has plan targets | ✅ Yes | ✅ Yes | ✅ Maybe (uses targets) |
| No focus criteria, no plan/targets | ✅ Yes | ✅ Yes | ❌ No |
| Has QA scores but < threshold convs | ✅ Yes | ✅ Yes | ❌ No |
| Has QA scores, ≥ threshold convs | ✅ Yes | ✅ Yes | ✅ Yes |
| All criteria filtered by category | ❌ No | ❌ No | ❌ No |

---

## Code References

| Behavior | File | Lines |
|----------|------|-------|
| Configuration parameters | `retrieve_coaching_efficiency_stats.go` | 37-43 |
| Request validation | `retrieve_coaching_efficiency_stats.go` | 193-200 |
| Session query filters | `retrieve_coaching_efficiency_stats.go` | 279-320 |
| Session-criterion expansion | `retrieve_coaching_efficiency_stats.go` | 324-359 |
| Category filtering | `retrieve_coaching_efficiency_stats.go` | 412-423 |
| Time window calculation | `retrieve_coaching_efficiency_stats.go` | 758-773 |
| Efficiency calculation condition | `retrieve_coaching_efficiency_stats.go` | 672 |
| Efficiency formula | `retrieve_coaching_efficiency_stats.go` | 978 |
| Invalid flag logic | `retrieve_coaching_efficiency_stats.go` | 703 |
| Session age check | `retrieve_coaching_efficiency_stats.go` | 972-975 |

---

## Related APIs

- **RetrieveCoachingSessionStats**: Simpler API that just counts sessions (no efficiency)
- **RetrieveQAScoreStats**: Called internally to get before/after QA scores
- **ListCoachingSessions**: Returns detailed session data

---

**Document Version:** 1.0
**Author:** Investigation of Hilton coaching discrepancy
**Last Updated:** 2026-01-07
