# Med Mutual Active Days and later behavior guides

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** active-days
**Substantive source dates:** 2026-03-23 to 2026-07-22
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [med-mutual-active-days-and-behavior-guides](med-mutual-active-days-and-behavior-guides.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/clickhouse-schema`; `/Users/xuanyu.wang/repos/cresta-proto`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The March investigation separated conversation activity from online-event evidence. On March 16 the saved query counted 603 agents with conversations, 70 with heartbeats and 47 with Agent Assist labels. This supports an online-evidence coverage gap; the original claim that every missing agent never used Desktop is stronger than those counts alone establish. Heartbeat absence can also reflect collection or client differences. The 47/70 ratio is a cohort comparison, not a measured per-conversation labeling failure rate.

The July 22 guide supersedes the March conversation-level description: agent messages qualify when sent during an online interval and belong to the message day; conversations without agent messages use assigned-agent online status at conversation start/end and belong to the conversation start day. Neither contract establishes actual feature use.

## Recorded outcome and limits

Recorded March diagnosis and July behavior-guide rewrite; external document paste still listed as follow-up. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

If reopened, verify the affected client/event collection and exact agent/conversation overlap before recommending adoption changes. Revalidate the current labeler and confirm whether the July guide was published. Continue the Med Mutual ticket as CONVI-6423 if ticket work resumes; this topic record also owns the later guide evidence.

## Evidence and timeline

- 2026-03-23 to 2026-07-22 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/active-days-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/active-days-legacy-evidence/README.md); [heartbeat-analysis.md](../sessions/2026-10-25/active-days-legacy-evidence/heartbeat-analysis.md); [log/2026-07-22.md](../sessions/2026-10-25/active-days-legacy-evidence/log/2026-07-22.md); [deliverables/agent-leaderboard-active-days-behavior-guide.md](../sessions/2026-10-25/active-days-legacy-evidence/deliverables/agent-leaderboard-active-days-behavior-guide.md).
- Durable reference home: [active-days](../subdomains/active-days/README.md).
