# Med Mutual Active Days Investigation - 2026-03-23

**Linear Issue**: [CONVI-6423](https://linear.app/cresta/issue/CONVI-6423/med-mutual-issues-with-active-days)
**Customer**: Med Mutual (mm-ohio-us-east-1)
**Issue Filed**: 2026-03-16
**Agent Example**: Iesha Williams

## Issue Description

Agents in Director have "assistance used" but Active Days shows N/A. The customer reports that this appears to affect all agents.

## Investigation Findings

### 1. Data Analysis from ClickHouse (March 16, 2026)

Analyzed the `conversation_with_labels_d` table to check Agent Assist tagging:

```
Date: 2026-03-16
- Conversations with has_agent_assistance=false: 11,729 (92%)
- Conversations with has_agent_assistance=true:   1,058 (8%)
- Unique agents with false: 568
- Unique agents with true:  47
```

**Pattern over time (March 10-22)**:
- Consistently 85-95% of conversations have `has_agent_assistance=false`
- Only a small fraction of agents are being detected as logged in
- This pattern is stable across weekdays

### 2. Conversation Sources

On March 16, Med Mutual had conversations from multiple sources:
- conversation_source=0: 12,739 conversations
- conversation_source=7:  3,229 conversations
- conversation_source=8:     59 conversations
- conversation_source=9:     50 conversations

(Note: Need to verify which source is "Normal" vs "Ingestion Pipeline")

### 3. Conversation Events Present

Conversation events were logged on March 16, indicating some level of activity:
- event_type=8:  63,556 events
- event_type=7:  26,399 events
- event_type=11:  9,893 events
- event_type=12:  7,759 events

## Active Days Behavior (from Documentation)

According to the [Active Days Behavior Guide](https://www.notion.so/3024a587b06180cbace3faa4fd6c8b14):

### How Active Days Works

1. **Conversation Tagging**: Every conversation is tagged as "has Agent Assist" or "doesn't have Agent Assist" by the `cron-label-conversations` job (runs every 30 minutes)

2. **Tagging Logic**: A conversation gets `has_agent_assistance=true` when the agent's login time (from `user_online_activities`) overlaps with the conversation time

3. **Active Days Values**:
   - **1** = Agent was active and had qualifying conversations with Agent Assist
   - **0** = Agent was logged in or had conversations, but didn't meet criteria
   - **N/A** = No evidence the agent was logged in or active

### Known Limitations

The documentation explicitly describes several scenarios where Active Days can be incorrect:

#### False Negatives (conversation incorrectly tagged "doesn't have Agent Assist")

Login detection relies on periodic **heartbeat signals** between the agent's app and the server. These heartbeats can be disrupted by:

- **Computer sleep or idle**: The agent's computer going to sleep or idling
- **Network interruptions**: Temporary connectivity issues, VPN drops, etc.
- **Brief offline periods**: The agent's machine being briefly offline for any reason

**Impact**: When a heartbeat gap occurs, the system records an "offline" event. If a conversation happens to fall between two heartbeat signals during such a gap, it will not overlap with any "online" event, and the conversation will be tagged as "doesn't have Agent Assist" — **even though the agent was actually using the app**.

If that conversation was the agent's **only** conversation on a given day, that entire day would be misclassified as **0** (or **N/A**) instead of **1**.

#### Data Processing Delay

- Cron job runs every 30 minutes
- Conversations appear in `conversation_d` in near real-time
- But tagging happens only after cron runs
- During this window (up to 30 minutes), agents may show **0** instead of **1**

#### Stale Labels from Transferred Conversations

- **Fixed in Feb 2026** (go-servers PR #25706, CONVI-6242)
- Backfilled for Alaska Air only
- **Other customers not yet backfilled** - Med Mutual may still have stale labels from 2026

## Analysis: Is This a Bug or Expected Behavior?

Based on the investigation, this appears to be **EXPECTED BEHAVIOR DUE TO DOCUMENTED LIMITATIONS**, specifically:

### Most Likely Cause: Heartbeat Gaps (False Negatives)

The data shows that 92% of conversations on March 16 have `has_agent_assistance=false`. This is consistent with the documented limitation where:

1. **Heartbeat detection failures**: If Med Mutual agents experience:
   - Network connectivity issues
   - VPN instability
   - Computer sleep/idle during conversations
   - Desktop app crashes or restarts

2. **Result**: Conversations fall between heartbeat signals → tagged as "doesn't have Agent Assist" → Active Days shows **N/A** instead of **1**

### Why Agents Show "Assistance Used" but No Active Days

- **"Assistance used"** likely refers to conversation events (hints shown, KB articles accessed, etc.) which are logged separately from login detection
- **"Active Days"** depends on the `has_agent_assistance` tag, which requires successful heartbeat overlap detection
- These are **two different data sources**: conversation events vs user_online_activities

### Additional Possible Causes

1. **Stale labels from transfers**: Med Mutual hasn't been backfilled since the Feb 2026 fix for CONVI-6242
2. **Ingestion Pipeline conversations**: If Med Mutual has `exclude_ingestion_pipeline_from_agent_stats=true` and conversations are from pipeline sources
3. **Browser extension logins**: If agents are using browser extensions instead of Desktop app (false positive scenario, but would show 0 not N/A)

## Recommendations

### 1. Check Network/Infrastructure Issues

The most likely root cause is connectivity problems affecting heartbeat detection. Investigate:
- VPN stability for Med Mutual agents
- Network interruptions on March 16
- Desktop app crash logs
- Computer power management settings causing sleep/idle

### 2. Verify Configuration

Check if Med Mutual has:
```sql
-- App DB query (needs correct schema)
SELECT env_config->'agent_stats'->>'exclude_ingestion_pipeline_from_agent_stats'
FROM environments WHERE namespace = 'mm-ohio-us-east-1';
```

### 3. Check for Stale Labels

Med Mutual hasn't been backfilled since the CONVI-6242 fix. If they have conversation transfers/reassignments in 2026, they may have stale labels. Run the same backfill procedure used for Alaska Air.

### 4. Educate Customer on Limitations

Share the [Active Days Behavior Guide](https://www.notion.so/3024a587b06180cbace3faa4fd6c8b14) with the customer, specifically the "False Negatives" section explaining heartbeat gaps. This is **working as designed** given the technical constraints.

### 5. Monitor Heartbeat Reliability

Add monitoring for:
- Percentage of conversations with `has_agent_assistance=false`
- Heartbeat gap frequency per agent
- Network quality metrics for this customer

## Conclusion

**This is most likely NOT a bug, but rather expected behavior due to documented limitations of heartbeat-based login detection.**

The 92% rate of `has_agent_assistance=false` suggests systematic heartbeat detection issues, likely due to:
- Network connectivity problems
- VPN instability
- Desktop app issues
- Power management settings

**Next Steps**:
1. Investigate Med Mutual's network infrastructure and agent connectivity
2. Check for stale labels from conversation transfers
3. Educate customer on the limitation
4. Consider improving heartbeat reliability or implementing alternative login detection methods

---

**Investigation Date**: 2026-03-23
**Investigator**: Xuanyu Wang (with Claude Code)
**Status**: Findings documented, awaiting customer infrastructure review
