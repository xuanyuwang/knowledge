# Session: RCG Director latency ticket

- Date: 2026-09-11
- Tool: Codex
- Primary domain: Analytics
- Primary subdomains: Performance Insights, Conversation Volume, Shared Analytics Platform
- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree context: main checkout, read-only investigation; no product-code changes
- Requested action: create a Linear ticket for the severe RCG latency issue

## Sources reviewed

- Slack incident: https://crestalabs.slack.com/archives/D06B2LD34RX/p1789060903484299
- Related `#convo-intelligence` thread opened from the incident discussion: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1788971647720819
- HAR: `/Users/xuanyu.wang/Downloads/rcg.cresta.com(latency).har`
- Screen recording: https://drive.google.com/drive/folders/1IBcn3fbC8mcWMCi0pSQ5uhUZYmCeSreQ?usp=sharing
- Prior bounded investigation: `sessions/2026-09-09/claude-rcg-dtq-pi-visibility.md`

## Ticket evidence

- Andra reported that an hour-long RCG training could not get through most material because Director pages took 30+ seconds to load; every leader on the call reported the problem.
- Trudee reproduced roughly 45 seconds for Opera rules and at least 30 seconds for Opera Analyzer, showing that the incident affected multiple Director pages.
- Later in the thread, a 21-agent Assistance Insights view took 45+ seconds. Everyone on a subsequent call was unable to load data.
- The recording shows intermittent latency that is not proportional to data volume. The conversation-volume widget remains loading after other Performance Insights tables complete and was identified as a critical blocker.
- The HAR contains four parallel `qaScoreStats:retrieve` requests for a seven-day Performance Insights view. All returned HTTP 200, but took 47.380-50.212 seconds. In each request, virtually all elapsed time was server wait (47.378-50.210 seconds), while receive time was 0.396-1.041 ms.
- The HAR requests cover current/prior time-range, agent-tier, and agent groupings. Response bodies range from 944 bytes to 167,157 bytes, so the captured latency is not explained by browser transfer time or the separate oversized template-list response.

## Scope boundary

This ticket tracks RCG Director analytics latency only. It does not track the separate `ListCurrentScorecardTemplates` oversized-response error. The same-title template/data-visibility inconsistency discussed in the recording is also not used as a latency root-cause claim.

## Supported framing

- Confirmed: severe, intermittent customer impact; multiple Director analytics surfaces; cold-load behavior; 47-50 second server waits on successful `RetrieveQAScoreStats` calls.
- Plausible but unproven: RCG insert/backfill load and cache opt-out may contribute. The Slack thread notes a recent RCG reindex/backfill and that RCG had previously been removed from the analytics cache, but neither is established as the root cause.
- Prior observability showed long-tail `RetrieveQAScoreStats` latency and ClickHouse executions, but did not prove an RCG traffic burst or an exact ClickHouse fraction. Fast-versus-slow query comparison remains necessary.

## Linear result

- Created [CONVI-7681](https://linear.app/cresta/issue/CONVI-7681/rcg-director-analytics-takes-30-50-seconds-and-blocks-live-training): `[RCG] Director analytics takes 30-50+ seconds and blocks live training`.
- Priority: Urgent. Status: Todo. Assignee: Xuanyu Wang.
- Labels: `Customer Issue`, `Support`, `Royal Caribbean Group`, `oncall-backlog`.
- Added the Slack incident and screen-recording folder as issue links. The raw HAR was not attached because it contains complete browser request metadata; sanitized measurements are captured in the description.
- Verified the saved issue through Linear after creation.
- Revised the saved description after user feedback so it contains only latency-related impact, evidence, hypotheses, investigation steps, and exit criteria; removed all references to the other issues.
