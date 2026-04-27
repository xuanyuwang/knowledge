# bswift Wrong User-to-Team Mapping (Duplicate GUEST_USER)

**Created:** 2026-04-26
**Updated:** 2026-04-26

## Overview

For customer bswift, certain agents appear under the wrong team (e.g., "default") on the Performance page. Root cause: duplicate GUEST_USER accounts exist alongside PROD_USER accounts. Conversations are routed to the GUEST_USER ids, which have incorrect team group memberships.

Originally investigated in `insights-user-filter/wrong-team-mapping-investigation.md` (2026-03-17). Other teams made partial fixes (~Mar 18) to route new conversations to PROD users, but historical data was not migrated and Brenda O'Neal was not fixed at all.

## Affected Agents

| Agent | PROD user_id | GUEST user_id | Status |
|-------|-------------|---------------|--------|
| Briana Joyner | `e41f4a45cbde99a6` | `516b107b2e23b60f` | **Fixed** |
| Tennisha Cox | `56aa04f69c744cb9` | `f22a48d63d182d43` | **Fixed** |
| Brenda O'Neal | `e2696310309a1dbc` | `5689b8439d3e985d` | **Not fixed** |

## Fix Summary (2026-04-26)

### Briana Joyner & Tennisha Cox — Fixed

**PostgreSQL (`app.chats`):**
- Updated `agent_user_id` and `team_group_id` for historical conversations (see `fix-historical-conversations.sql`)
- Briana: 1,752 rows migrated → PROD total now 2,999
- Tennisha: 418 rows migrated → PROD total now 1,049
- Verified: 0 conversations remain under GUEST user IDs

**ClickHouse (`conversation_d`):**
- Deleted stale GUEST rows, then ran backfill jobs to reindex from PG
- Tennisha: backfill 2026-03-03 → 2026-03-18 (single job)
- Briana: backfill 2025-09-13 → 2026-03-18 (12 jobs, ~2-week windows)
- Final state:

| Agent | GUEST | PROD | Earliest | Latest |
|-------|-------|------|----------|--------|
| Briana Joyner | 0 | 1,483 | 2026-01-02 | 2026-04-24 |
| Tennisha Cox | 0 | 818 | 2026-01-06 | 2026-04-24 |

Note: PG and CH counts differ because `conversation_d` is populated by a separate pipeline.

### Brenda O'Neal — Different Situation

Unlike Briana and Tennisha, Brenda's case is reversed — the GUEST user is the active, correctly-configured account:

| Field | PROD (`e2696310309a1dbc`) | GUEST (`5689b8439d3e985d`) |
|-------|--------------------------|---------------------------|
| Username | `onealb33@bswift.com` | `o'nealb33@bswift.com` |
| Type | 1 (PROD_USER) | 2 (GUEST_USER) |
| Active state | **2 (INACTIVE)** | **1 (ACTIVE)** |
| Team | **default** | **Diann Ochwat** (correct) |
| Conversations (PG) | 0 | 1,185 |
| Conversations (CH) | 0 | 1,613 |

The GUEST user has the correct team assignment and all the conversation data. The PROD user is deactivated and only in the default team. No routing fix is needed — the GUEST account is effectively the "real" account.

Options:
1. **Leave as-is** — GUEST user is working correctly; conversations and team are right
2. **Activate PROD, assign correct team, migrate data** — cleaner but requires more work and coordination

## Status

Briana & Tennisha: **Complete**
Brenda: **Needs decision** — GUEST account is the active/correct one, opposite of the other two agents

## Log History

| Date | Summary |
|------|---------|
| 2026-03-17 | Original investigation — root cause identified (see `insights-user-filter/wrong-team-mapping-investigation.md`) |
| 2026-04-26 | Fixed Briana & Tennisha in PG and CH. Brenda still unfixed — needs platform team. |
