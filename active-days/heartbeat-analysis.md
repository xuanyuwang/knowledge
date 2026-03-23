# Heartbeat Analysis - Med Mutual Active Days

**Investigation Date**: 2026-03-23
**Data Sources**: 
- ClickHouse events DB: `auth_mm_ohio.user_event_d` (heartbeat data)
- ClickHouse conversations DB: `mm_ohio_us_east_1.conversation_with_labels_d` (tagging data)

## Key Findings: Low Desktop App Adoption, Not Heartbeat Gaps

### Comparison Table: Heartbeats vs Active Days Tagging

| Date | Agents with Heartbeats | Total Agents with Convos | Agents Tagged with AA | Heartbeat Coverage | AA Tagging Rate |
|------|------------------------|--------------------------|----------------------|-------------------|-----------------|
| 3/10 | 65 | 631 | 18 | **10.3%** | 2.9% |
| 3/11 | 61 | 607 | 25 | **10.0%** | 4.1% |
| 3/12 | 62 | 600 | 39 | **10.3%** | 6.5% |
| 3/13 | 57 | 557 | 37 | **10.2%** | 6.6% |
| 3/16 | 70 | 603 | 47 | **11.6%** | 7.8% |
| 3/17 | 96 | 597 | 59 | **16.1%** | 9.9% |
| 3/18 | 91 | 596 | 62 | **15.3%** | 10.4% |
| 3/19 | 104 | 591 | 68 | **17.6%** | 11.5% |
| 3/20 | 95 | 556 | 64 | **17.1%** | 11.5% |

### Critical Discovery

**Only 10-18% of agents with conversations have ANY heartbeat events.**

This means:
- On March 16: **70 out of 603 agents** (11.6%) had heartbeat events
- On March 16: **533 agents** (88.4%) had ZERO heartbeat events recorded
- Those 70 agents with heartbeats had GOOD coverage (40-97 heartbeats throughout the day)

### Root Cause Identified: LOW DESKTOP APP ADOPTION

This is **NOT** primarily a "heartbeat gap" issue (false negatives from network interruptions).

**The real issue**: **88-90% of Med Mutual agents are NOT using the Desktop app at all.**

They are likely:
1. Taking calls through their CCaaS platform without Cresta Desktop app logged in
2. Using web-based interfaces that don't emit heartbeat events
3. Simply not enrolled in the Desktop app program

### Secondary Issue: Tagging Inefficiency

Even among agents WITH heartbeats, not all are being tagged properly:
- March 16: 70 agents with heartbeats → only 47 tagged with `has_agent_assistance=true`
- This represents a **67% tagging efficiency** (47/70)
- Possible causes:
  - Conversation timing doesn't overlap with heartbeat windows
  - Cron job lag (30-minute delay)
  - Transferred/reassigned conversations (pre-backfill issue)

### Improving Trend After March 16

After the issue was filed on March 16, the data shows improvement:
- Heartbeat coverage: 11.6% (3/16) → 17.6% (3/19)
- AA tagging rate: 7.8% (3/16) → 11.5% (3/19)

This suggests:
- Possible deployment/fix around March 16-17
- OR increased Desktop app adoption efforts
- OR customer started monitoring and encouraging logins

## Heartbeat Quality Analysis

For agents who DO have heartbeats, the coverage is excellent:
- Top 20 agents: 36-97 heartbeats per day
- Time spans: Full work day (11am-11pm, some 24-hour coverage)
- Frequency: ~15-20 minute intervals (typical heartbeat cadence)

**Conclusion**: Heartbeat detection works properly for agents using the Desktop app. The issue is that most agents aren't using it.

## Why "Assistance Used" Shows Data but Active Days Doesn't

Customer reports seeing "assistance used" but no Active Days:

- **"Assistance used"** likely comes from:
  - Conversation events (hints triggered, KB articles accessed)
  - CCaaS integration events
  - Post-call jobs importing conversations
  
- **"Active Days"** requires:
  - Desktop app login
  - Heartbeat events in `user_event_d`
  - Successful overlap detection by `cron-label-conversations`

**These are independent data sources**, which explains the disconnect.

## Revised Conclusion

### Is This a Bug?

**NO - This is expected behavior, but the root cause is different than initially thought.**

Original hypothesis: Heartbeat gaps (false negatives) due to network interruptions ❌
**Actual cause**: Low Desktop app adoption - 88-90% of agents never log in ✅

### What Med Mutual Needs to Do

1. **Investigate Desktop App Adoption**
   - Why are only 10-15% of agents using the Desktop app?
   - Are agents trained on the Desktop app?
   - Are there deployment/access issues preventing logins?

2. **Improve Desktop App Enrollment**
   - Identify the 533 agents (on March 16) who handle conversations but don't have heartbeats
   - Deploy Desktop app to these agents
   - Train them on login procedures
   - Monitor adoption rate

3. **Set Realistic Expectations**
   - If agents don't use Desktop app → Active Days will show N/A
   - This is working as designed
   - Active Days specifically measures **Desktop app usage**, not general conversation handling

4. **Consider Alternative Metrics**
   - If Med Mutual wants to track "days with conversations" regardless of Desktop app:
     - Use conversation volume metrics instead of Active Days
     - Or exclude Active Days from reporting
     - Or define custom "engagement" metrics based on conversation events

### Tagging Efficiency Improvement

For the 67% tagging efficiency issue (70 heartbeats → 47 tagged):
- Check if Med Mutual was backfilled after CONVI-6242 fix (transferred conversations)
- Monitor cron job lag
- Investigate conversation timing patterns

## Customer Communication

When explaining to Med Mutual:

**Bad framing**: "This is a bug with heartbeat detection"
**Good framing**: "Active Days specifically measures Desktop app usage. Your data shows only 10-15% of agents are logging into the Desktop app, which is why 85-90% show N/A for Active Days. The agents showing N/A are still handling conversations - they're just not using the Desktop app."

**Recommended action**: Focus on Desktop app adoption and training, not system fixes.

---

**Investigator**: Xuanyu Wang (with Claude Code)
**Data Period**: March 10-22, 2026
**Evidence Level**: High - Direct heartbeat event data analyzed
