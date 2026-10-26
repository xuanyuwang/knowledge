# Iesha Williams - Specific Agent Investigation

**Agent User ID**: `8fdf767a811dc558`
**Investigation Date**: 2026-03-23
**Context**: Customer mentioned "Iesha Williams" as an example agent showing "assistance used but no active days"

## Findings

### Heartbeat History (All Time)
- **Total heartbeats**: 39 (from Jan 28 - March 23, 2026)
- **March 12**: 1 heartbeat at 12:29 PM
- **March 16** (issue filed): **0 heartbeats** ❌
- **March 17**: 3 heartbeats (4:41 PM - 7:23 PM)
- **March 19**: 3 heartbeats (6:21 PM - 7:46 PM)

### Conversation History
- **Total conversations ever**: 1
- **Last conversation**: August 12, 2025
- **Conversations in March 2026**: **0** ❌

### Conversation Events (March 10-22, 2026)
- **Total events**: 0
- **No hints, KB access, or other assistance events**

## Analysis

### Is Iesha Williams in the "zero heartbeat" category?

**On March 16 specifically: YES** - She had zero heartbeats on the day the issue was filed.

**Overall pattern: NO** - She's not an active agent at all. She's not handling conversations, so she wouldn't show "assistance used" either.

### Why doesn't she match the bug description?

The bug report states: **"agents in director have assistance used but no active days"**

Iesha Williams has:
- ❌ NO conversations in March 2026
- ❌ NO conversation events (no "assistance used")
- ❌ NO Active Days (correctly showing N/A because she's not active)

**Conclusion**: Iesha Williams was likely mentioned as a general example by the customer, but she's not actually experiencing the reported issue. She's simply not an active agent during this period.

## The Real Issue

The actual problem affects the **533 agents on March 16** who:
- ✅ ARE handling conversations (conversation_d has records)
- ✅ MAY have assistance events (hints, KB access)
- ❌ Have ZERO heartbeat events
- ❌ Show N/A for Active Days (correctly, because no Desktop app login)

These 533 agents are the ones who would appear to have "assistance used but no active days" if:
1. They're getting conversation events through CCaaS integration or post-call jobs
2. They're NOT logging into the Desktop app

## Iesha Williams' Activity Pattern

Based on the data:
- **Very low engagement**: Only 39 heartbeats across 2 months
- **Not an active agent**: Last conversation was 7 months ago (August 2025)
- **Sporadic Desktop app usage**: 1-3 logins per day on only 3 days in March
- **No current workload**: Not handling any conversations in March 2026

She appears to be:
- A former agent no longer taking calls
- Or an agent on extended leave
- Or a test/training account with minimal usage

## Recommendation

**Don't use Iesha Williams as the investigation target.** Instead, focus on the 533 agents with:
- Active conversation volume (50+ conversations/day)
- Zero heartbeat events
- Potential conversation events from CCaaS

Example agents to investigate instead (from March 16 data):
- `c6dec8345341b253`: 101 conversations, 0 with AA
- `b15d4358e1732f9`: 83 conversations, 0 with AA
- `e61e5821793c6188`: 83 conversations, 0 with AA

These are the agents experiencing the Desktop app adoption issue.

---

**Investigator**: Xuanyu Wang (with Claude Code)
**Data Sources**:
- ClickHouse events DB: `auth_mm_ohio.user_event_d`
- ClickHouse conversations DB: `mm_ohio_us_east_1.conversation_d`
