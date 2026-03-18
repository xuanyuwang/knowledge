# Wrong User-to-Team Mapping: Duplicate GUEST_USER Accounts

**Created:** 2026-03-17
**Updated:** 2026-03-18
**Status:** Root cause identified, scope assessed

## Problem

For customer bswift, agent "Briana Joyner" (user_id `e41f4a45cbde99a6`) is displayed under team "default" on the Performance page, but actually belongs to team "Nathaniel King" (group_id `01992ac1-2405-77f7-bae6-84114a04211e`).

## Root Cause

There are **two user accounts** for "Briana Joyner" in bswift:

| Field | PROD User | GUEST User (duplicate) |
|-------|-----------|------------------------|
| user_id | `e41f4a45cbde99a6` | `516b107b2e23b60f` |
| username | `joynerb@bswift.com` | `joynerb1@bswift.com` |
| type | 1 (PROD_USER) | 2 (**GUEST_USER**) |
| active_state | 1 (ACTIVE) | 2 (INACTIVE) |
| profile_status | ACTIVE | INACTIVE |
| user_roles | {1} (AGENT) | {1} (AGENT) |
| direct TEAM group | Nathaniel King | **default** |
| conversations in app.chats | **0** | **1752** |

The old GUEST_USER account passes ALL filters in `ListUsersForAnalytics` and is included in `groundTruthUsers`:

1. **Type filter** (`auth.users.type != DEV_USER`): `GUEST_USER (2) != DEV_USER (3)` — **passes**
2. **AgentOnly filter**: `user_roles = {1}` (AGENT only) — **passes**
3. **IncludeInactiveUsers**: When `excludeDeactivatedUsers = false` (default), `profile_status = INACTIVE` is included — **passes**
4. **Profile filter**: Has `user_profile_configs` entry for `us-east-1` — **passes**

Since the old user's only TEAM group membership is the "default" group, `buildUserGroupMappings` maps it to team "default". The ClickHouse scorecard data also references this old user_id, so its QA scores appear under the "default" team.

## Conversation Data: GUEST Users Are the Active Ones

Cross-checking `app.chats` (source of truth for conversations) reveals that **all conversations are being created under the GUEST_USER ids**, not the PROD_USER ids:

| Agent | PROD user_id | PROD convos | GUEST user_id | GUEST convos | Latest convo |
|-------|-------------|-------------|---------------|--------------|--------------|
| Briana Joyner | `e41f4a45cbde99a6` | **0** | `516b107b2e23b60f` | **1,752** | 2026-03-17 |
| Brenda O'Neal | `e2696310309a1dbc` | **0** | `5689b8439d3e985d` | **575** | 2026-03-18 |
| Tennisha Cox | `56aa04f69c744cb9` | **6** | `f22a48d63d182d43` | **418** | 2026-03-17 |

The platform integration is still routing conversations to the old GUEST accounts. This means:
- **ClickHouse data cleanup alone won't fix it** — new conversations continue under old user_ids
- The GUEST accounts are the "real" working accounts from the platform's perspective

### Briana Joyner's team_group_id in app.chats

| team_group_id | Count | Period |
|---------------|-------|--------|
| _(empty)_ | 1,321 | 2025-09 to 2026-03 |
| `01992ac1-2405-77cc` (Trace Campbell) | 431 | 2025-10 to 2026-01 |

The `team_group_id` is mostly empty (1,321 convos) because `getUserTeamGroupID()` returns empty when the user's only TEAM group is "default" (which is excluded from the logic).

## Why the PROD User Shows Correctly (When It Shows)

The PROD user `e41f4a45cbde99a6` has a direct membership in "Nathaniel King" and is correctly mapped. But since it has **zero conversations**, it only appears on pages that show agents regardless of data (e.g., agent list), not on data-driven pages like Performance.

On the Performance page you see:
- "Briana Joyner" (GUEST, `516b107b2e23b60f`) → **default** (has all the conversation data)
- "Briana Joyner" (PROD, `e41f4a45cbde99a6`) → not visible (no conversation data)

## User Type Enum Values

```
0: USER_TYPE_UNSPECIFIED
1: PROD_USER
2: GUEST_USER   ← NOT filtered by ListUsersForAnalytics
3: DEV_USER     ← Only this type is filtered out
4: BOT_USER
5: AI_AGENT_USER
```

File: `auth/sql-schema/gen/go/protos/user/user.pb.go:136-141`

## Code Path

```
RetrieveQAScoreStats
  → ParseUserFilterForAnalytics
    → listAllUsers (IncludeInactiveUsers=true)
      → ListUsersForAnalytics (type != DEV_USER, profile_status IN (ACTIVE, INACTIVE))
        → Returns BOTH users (active PROD + inactive GUEST)
    → buildUserGroupMappings
      → Maps e41f4a45cbde99a6 → Nathaniel King
      → Maps 516b107b2e23b60f → default (its only TEAM group)
  → ClickHouse query includes both user_ids
  → Only 516b107b2e23b60f has data → appears under "default"
```

## Key Files

- `ListUsersForAnalytics`: `auth/internal/service/nonpublic/user/action_list_users_for_analytics.go:73-77`
- `UserType enum`: `auth/sql-schema/gen/go/protos/user/user.pb.go:136-141`
- `listAllUsers`: `insights-server/internal/analyticsimpl/common_user_filter.go:451-489`
- `buildUserGroupMappings`: `insights-server/internal/analyticsimpl/common_user_filter.go:363-447`
- `appendGroupMemberships`: `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go:275-321`
- `getUserTeamGroupID`: `apiserver/internal/conversation/action_update_conversation_agent.go:62-98`

## Scope of Impact

Checked the us-east-1 auth DB — **only 3 affected users, all in bswift**:

| Agent | PROD state | GUEST state | GUEST group |
|-------|-----------|-------------|-------------|
| Briana Joyner | ACTIVE | INACTIVE | default |
| Tennisha Cox | ACTIVE | INACTIVE | default |
| Brenda O'Neal | INACTIVE | **ACTIVE** | Diann Ochwat |

Note: Brenda O'Neal's PROD_USER is inactive while her GUEST_USER is active.

## Potential Fixes

### Option 1: Fix GUEST_USER group memberships (Short-term, bswift-only)
Update the 3 GUEST_USER accounts to have the correct team group memberships (matching their PROD counterparts). This immediately fixes the display issue.

**Pros:** Quick fix, no code changes
**Cons:** Doesn't prevent future occurrences; only fixes bswift

### Option 2: Fix platform integration routing (Root cause)
Investigate why the platform integration routes conversations to GUEST_USER ids instead of PROD_USER ids. Update the routing so new conversations use the correct user_id.

**Pros:** Fixes the actual root cause
**Cons:** Requires platform team involvement; historical data still references old ids

### Option 3: Filter GUEST_USER in ListUsersForAnalytics (Systemic code fix)
Change the type filter from `type != DEV_USER` to `type NOT IN (DEV_USER, GUEST_USER, BOT_USER, AI_AGENT_USER)`.

**Pros:** Prevents non-production users from appearing in analytics systemically
**Cons:** Would HIDE the GUEST_USER data (which is the actual conversation data); may break analytics for these agents entirely until platform routing is fixed

### Option 4: Migrate conversation data to PROD_USER ids
Update `app.chats.agent_user_id` and ClickHouse data to point to the PROD_USER ids.

**Pros:** Clean long-term fix
**Cons:** Large data migration; risky; needs platform routing fix first (Option 2) to prevent new data under old ids

### Recommended Approach
1. **Immediate**: Option 1 (fix GUEST group memberships) to resolve the display issue
2. **Next**: Option 2 (fix platform routing) to stop creating data under wrong user_ids
3. **Later**: Option 4 (migrate data) if needed for clean analytics
