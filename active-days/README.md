# Active Days Investigation Project

**Created**: 2026-03-23
**Updated**: 2026-03-23

## Overview

This project investigates an Active Days issue reported by Med Mutual where agents show "assistance used" but Active Days displays N/A in the Agent Leaderboard.

## Key Findings - REVISED WITH HEARTBEAT DATA

After analyzing direct heartbeat event data from ClickHouse events DB, the root cause is now clear:

### Root Cause: **LOW DESKTOP APP ADOPTION** (88-90% of agents)

**Critical Evidence**:
- On March 16, 2026: Only **70 out of 603 agents** (11.6%) had ANY heartbeat events
- **533 agents** (88.4%) had ZERO heartbeat events recorded
- Those 70 agents with heartbeats had excellent coverage (40-97 heartbeats/day)

**Conclusion**: This is NOT primarily a "heartbeat gap" issue (network interruptions causing false negatives). The vast majority of Med Mutual agents are simply **not using the Desktop app at all**.

### Data Summary

| Date | Agents with Heartbeats | Total Agents with Convos | Agents Tagged with AA | Desktop App Usage |
|------|------------------------|--------------------------|----------------------|-------------------|
| 3/10 | 65 | 631 | 18 | **10.3%** |
| 3/16 | 70 | 603 | 47 | **11.6%** |
| 3/19 | 104 | 591 | 68 | **17.6%** |

### Why "Assistance Used" Shows Data but Active Days Doesn't

- **"Assistance used"** = Conversation events (hints, KB access) from CCaaS integration or post-call jobs
- **"Active Days"** = Requires Desktop app login with heartbeat events

These are **independent data sources**. Agents can have conversations (with assistance events) without being logged into the Desktop app.

## Is This a Bug?

**NO** - This is expected behavior. Active Days specifically measures **Desktop app usage**, not general conversation handling.

If agents don't log into the Desktop app, Active Days will correctly show N/A.

## Recommendations for Med Mutual

1. **Investigate Desktop App Adoption**
   - Why are only 10-15% of agents using the Desktop app?
   - Are agents trained on the Desktop app?
   - Are there deployment/access issues?

2. **Improve Desktop App Enrollment**
   - Identify the 533 agents without heartbeats on March 16
   - Deploy and train them on Desktop app usage
   - Monitor adoption metrics

3. **Set Realistic Expectations**
   - Active Days measures Desktop app usage, not conversation activity
   - If Med Mutual doesn't want agents using Desktop app, Active Days isn't the right metric
   - Consider using conversation volume metrics instead

4. **Consider Alternative Metrics**
   - Total conversations per day (regardless of Desktop app)
   - Conversation events (hints used, KB accessed)
   - Custom engagement metrics based on CCaaS data

## Secondary Issue: Tagging Efficiency

Even among agents WITH heartbeats, tagging efficiency is only 67%:
- March 16: 70 heartbeats → only 47 tagged with `has_agent_assistance=true`
- Possible causes: cron lag, conversation timing mismatches, stale labels from transfers

## Documentation

- **Heartbeat Analysis**: [heartbeat-analysis.md](heartbeat-analysis.md) - Comprehensive data analysis with direct heartbeat event data
- **Initial Investigation**: [initial-investigation.md](initial-investigation.md) - Conversation tagging patterns and hypothesis evolution
- **Iesha Williams Investigation**: [iesha-williams-investigation.md](iesha-williams-investigation.md) - Specific agent mentioned in bug report
- **Active Days Behavior Guide**: https://www.notion.so/3024a587b06180cbace3faa4fd6c8b14
- **Linear Issue**: [CONVI-6423](https://linear.app/cresta/issue/CONVI-6423/med-mutual-issues-with-active-days)

## Customer Communication Template

> "Active Days specifically measures Desktop app usage. Our analysis shows only 10-15% of your agents are logging into the Cresta Desktop app, which is why 85-90% show N/A for Active Days.
>
> The agents showing N/A are still handling conversations - they're just not using the Desktop app. To improve Active Days coverage, you'll need to increase Desktop app adoption through deployment, training, and monitoring.
>
> If your agents don't need the Desktop app, consider using conversation volume metrics instead of Active Days to track agent activity."

## Log History

| Date | Summary |
|------|---------|
| 2026-03-23 | **INVESTIGATION COMPLETED**: Root cause identified as low Desktop app adoption (10-15% of agents), not heartbeat detection failures. Direct heartbeat event data analyzed from ClickHouse events DB. Investigated specific agent "Iesha Williams" - confirmed she's not an active agent (no conversations since Aug 2025). This is expected behavior, not a bug. |
