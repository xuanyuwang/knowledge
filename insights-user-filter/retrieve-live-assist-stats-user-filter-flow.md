# RetrieveLiveAssistStats: User Filter Flow to Clickhouse Query

## Complete Data Flow

### Step-by-Step Flow

```
Request
  └─ req.FilterByAttribute.Users = [user1, user2, user3]
         │
         ▼
Step 1: ApplyResourceACL (line 34)
  └─ Filters users based on caller's ACL permissions
  └─ req.FilterByAttribute.Users = [user1, user2] (ACL filtered)
         │
         ▼
Step 2: ListUsersMappedToGroups (line 48-59) [Optional - only if grouping]
  └─ Fetches users from groups for response construction
  └─ Doesn't modify req.FilterByAttribute.Users
  └─ Returns: users = [user3, user4] (from groups)
         │
         ▼
Step 3: MoveFiltersToUserFilter (line 108-116) [Optional - only if groups/deactivation]
  └─ Expands groups to users, filters deactivated
  └─ req.FilterByAttribute.Users = [user1, user2, user5] (updated)
         │
         ▼
Step 4: readLiveAssistStatsFromClickhouse (line 126)
  └─ req.FilterByAttribute.Users passed to parseClickhouseFilter
         │
         ▼
Step 5: parseClickhouseFilter (line 100)
  └─ Calls buildUsersConditionAndArgs(attribute.Users, targetTables)
         │
         ▼
Step 6: buildUsersConditionAndArgs (common_clickhouse.go:142-164)
  └─ Extracts user IDs from User objects
  └─ Creates: agentUserIDs = ["user1", "user2", "user5"]
  └─ Builds SQL condition: "agent_user_id IN (?, ?, ?)"
  └─ Returns: conditionsAndArgs[actionAnnotationTable] =
      [{condition: "agent_user_id IN (?, ?, ?)", args: ["user1", "user2", "user5"]}]
         │
         ▼
Step 7: liveAssistStatsClickHouseQuery (retrieve_live_assist_stats_clickhouse.go:17-86)
  └─ ⭐ SPECIAL HANDLING (lines 20-27):
      For each condition containing "agent_user_id":
        - Duplicate the condition
        - Replace "agent_user_id" with "manager_user_id"
        - Join with OR

      BEFORE: agent_user_id IN (?, ?, ?)
      AFTER:  (agent_user_id IN (?, ?, ?)) OR (manager_user_id IN (?, ?, ?))

      Args are duplicated too: ["user1", "user2", "user5", "user1", "user2", "user5"]
         │
         ▼
Step 8: Final Clickhouse Query
  └─ WHERE clause in raised_hands_and_whispers CTE (line 50):
      AND (agent_user_id IN ('user1', 'user2', 'user5'))
          OR (manager_user_id IN ('user1', 'user2', 'user5'))
```

---

## Code Analysis

### 1. User ID Extraction (buildUsersConditionAndArgs)

**File**: `common_clickhouse.go:142-164`

```go
func buildUsersConditionAndArgs(
    users []*userpb.User,
    targetTables []ClickhouseTable,
) (map[ClickhouseTable][]conditionAndArg, error) {
    // Extract user IDs from User objects
    agentUserIDs := []string{}
    for _, user := range users {
        userName, err := userpb.ParseUserName(user.Name)
        if err != nil {
            return nil, status.Error(codes.InvalidArgument, err.Error())
        }
        agentUserIDs = append(agentUserIDs, userName.UserID)
    }

    // Build IN clause condition
    conditionsAndArgs := map[ClickhouseTable][]conditionAndArg{}
    if len(agentUserIDs) > 0 {
        for _, t := range targetTables {
            colName := tableSpecificColumnName(t, agentUserIDColumn)
            con, arg := columnIn(colName, agentUserIDs)
            conditionsAndArgs[t] = append(conditionsAndArgs[t],
                conditionAndArg{condition: con, arg: arg})
        }
    }
    return conditionsAndArgs, nil
}
```

**What it does**:
1. Takes `[]*userpb.User` (e.g., `[{Name: "customers/123/users/user1"}, ...]`)
2. Parses each user name to extract user ID
3. Creates SQL condition: `agent_user_id IN (?, ?, ?)`
4. Returns condition + args for each target table

**Example**:
```
Input: users = [{Name: "customers/123/users/alice"}, {Name: "customers/123/users/bob"}]
Output: {
  actionAnnotationTable: [
    {
      condition: "agent_user_id IN (?, ?)",
      arg: ["alice", "bob"]
    }
  ]
}
```

---

### 2. Special Handling for Live Assist (liveAssistStatsClickHouseQuery)

**File**: `retrieve_live_assist_stats_clickhouse.go:17-86`

**The Critical Lines (20-27)**:
```go
aaConditions := conditionsAndArgs[actionAnnotationTable]
for i := 0; i < len(aaConditions); i++ {
    c := aaConditions[i]
    if strings.Contains(c.condition, agentUserIDColumn) {
        // Duplicate condition for manager_user_id
        cm := strings.ReplaceAll(c.condition, agentUserIDColumn, "manager_user_id")
        // Join with OR
        c.condition = strings.Join([]string{c.condition, cm}, " OR ")
        // Duplicate args
        c.args = append(c.args, c.arg)
        aaConditions[i] = c
    }
}
```

**What it does**:
1. Finds conditions that filter on `agent_user_id`
2. Creates a copy of the condition with `manager_user_id` instead
3. Combines them with OR
4. Duplicates the args (user IDs appear twice in the query)

**Example Transformation**:
```
BEFORE:
  condition: "agent_user_id IN (?, ?)"
  args: ["alice", "bob"]

AFTER:
  condition: "(agent_user_id IN (?, ?)) OR (manager_user_id IN (?, ?))"
  args: ["alice", "bob", "alice", "bob"]
```

---

### 3. Final Query Structure

**The Query Template** (lines 33-78):
```sql
WITH raised_hands_and_whispers AS (
    SELECT
        conversation_id,
        agent_user_id,
        manager_user_id,
        has_raised_hand,
        has_whisper
    FROM action_annotation_d
    WHERE
        conversation_source IN (0, 8)
        AND action_type IN (7, 12)
        AND is_dev_user = 0
        AND agent_user_id <> ''
        AND %s  -- ← User filter inserted here
    GROUP BY 1,2,3,4
),
agent_live_assist_stats AS (
    SELECT
        agent_user_id,
        COUNT(DISTINCT conversation_id) FILTER (WHERE has_whisper = 1) AS whispered_to_agent_count,
        0 AS whisper_by_manager_count
    FROM raised_hands_and_whispers
    GROUP BY agent_user_id
),
manager_live_assist_stats AS (
    SELECT
        manager_user_id AS agent_user_id,  -- ← Aliased for UNION
        0 AS whispered_to_agent_count,
        COUNT(DISTINCT conversation_id) FILTER (WHERE has_whisper = 1) AS whisper_by_manager_count
    FROM raised_hands_and_whispers
    GROUP BY manager_user_id
)
SELECT * FROM agent_live_assist_stats
UNION ALL
SELECT * FROM manager_live_assist_stats
```

**With Actual User Filter**:
```sql
-- User filter (from Step 7):
AND ((agent_user_id IN ('alice', 'bob')) OR (manager_user_id IN ('alice', 'bob')))
```

---

## Why This Design?

### The Dual-Purpose Query

The query serves **two different use cases** with a **single WHERE clause**:

| Use Case | Filtered By | Metric Calculated |
|----------|-------------|------------------|
| **Agent Leaderboard** | agent_user_id IN (alice, bob) | whispered_to_agent_count |
| **Manager Leaderboard** | manager_user_id IN (alice, bob) | whisper_by_manager_count |

By using `OR` in the WHERE clause:
- Records where alice/bob are **agents** (agent_user_id matches) → counted in agent stats
- Records where alice/bob are **managers** (manager_user_id matches) → counted in manager stats

### The UNION ALL Pattern

Two CTEs produce separate result sets:
1. `agent_live_assist_stats`: Groups by agent_user_id, calculates agent metrics
2. `manager_live_assist_stats`: Groups by manager_user_id (aliased as agent_user_id), calculates manager metrics

Both are returned in the response, and frontends pick what they need:
- Agent Leaderboard uses `whispered_to_agent_count`
- Manager Leaderboard uses `whisper_by_manager_count`

---

## The Problem with Current Implementation

### Scenario: Agent Leaderboard Request

**Request**:
```
FilterByAttribute.Users = [alice, bob, charlie]
```

**Where**:
- `alice` has roles: `[AGENT]` (pure agent)
- `bob` has roles: `[AGENT, MANAGER]` (agent+manager)
- `charlie` has roles: `[AGENT]` (pure agent)

**Current Flow** (listAgentOnly = false):

```
Step 1: ApplyResourceACL
  → Users: [alice, bob, charlie] (all allowed)

Step 2: ListUsersMappedToGroups (listAgentOnly = false)
  → Returns ALL users with agent role (includes bob)

Step 3: MoveFiltersToUserFilter (listAgentOnly = false)
  → Users: [alice, bob, charlie] (unchanged)

Step 4-7: Clickhouse query
  → WHERE (agent_user_id IN ('alice', 'bob', 'charlie')) OR (manager_user_id IN ('alice', 'bob', 'charlie'))
```

**Result on Agent Leaderboard**:
- ✅ alice appears (pure agent)
- ❌ bob appears (agent+manager - should be excluded!)
- ✅ charlie appears (pure agent)

### What Should Happen (listAgentOnly = true):

```
Step 1: ApplyResourceACL
  → Users: [alice, bob, charlie] (all allowed by ACL)

Step 2: ParseUserFilterForAnalytics (listAgentOnly = true)
  → Ground truth: Only users with AGENT role (no other roles)
  → Intersects: [alice, charlie] (bob excluded because he has MANAGER role too)

Step 3-6: Clickhouse query
  → WHERE (agent_user_id IN ('alice', 'charlie')) OR (manager_user_id IN ('alice', 'charlie'))
```

**Result on Agent Leaderboard**:
- ✅ alice appears (pure agent)
- ✅ bob excluded (agent+manager - correctly filtered!)
- ✅ charlie appears (pure agent)

---

## Summary

### User Filter Flow

```
req.FilterByAttribute.Users
  ↓ (ACL filtering)
req.FilterByAttribute.Users (ACL-filtered)
  ↓ (Group expansion / deactivation filtering)
req.FilterByAttribute.Users (final list)
  ↓ (buildUsersConditionAndArgs)
agentUserIDs = ["user1", "user2", ...]
  ↓ (SQL condition generation)
condition: "agent_user_id IN (?, ?, ...)"
  ↓ (liveAssistStatsClickHouseQuery - SPECIAL!)
condition: "(agent_user_id IN (...)) OR (manager_user_id IN (...))"
  ↓ (Final query)
WHERE ... AND ((agent_user_id IN (...)) OR (manager_user_id IN (...)))
```

### The Critical Transformation

**Line 20-27 in `retrieve_live_assist_stats_clickhouse.go`**:
```go
if strings.Contains(c.condition, agentUserIDColumn) {
    cm := strings.ReplaceAll(c.condition, agentUserIDColumn, "manager_user_id")
    c.condition = strings.Join([]string{c.condition, cm}, " OR ")
    c.args = append(c.args, c.arg)
}
```

This is **unique to RetrieveLiveAssistStats** and is why the API can serve both agent and manager use cases with a single query.

### The Fix Needed

The user list that goes into the query must be filtered differently based on the use case:
- **Agent/Team Leaderboards**: Use `listAgentOnly = true` → Filter to pure agents
- **Manager Leaderboard**: Use `listAgentOnly = false` → Include users with manager role

This requires adding a request parameter `filter_to_agents_only` to distinguish these use cases.
