# Agent Stats and conversation-count contracts

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** shared-analytics-platform
**Substantive source dates:** 2026-06-29 to 2026-07-08
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [agent-stats-and-conversation-count-contracts](agent-stats-and-conversation-count-contracts.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/cresta-proto`; `/Users/xuanyu.wang/repos/clickhouse-schema`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The June guide records the transition to message-granular labels. Batches are selected by conversation end, but messages must be fetched by conversation ID without the batch timestamp restriction: valid messages can precede a multi-day conversation's final batch. PR #29058 removed that restriction; #29084 chunked IDs to avoid the PostgreSQL parameter limit. Empty-message fallback rows retain conversation-anchor semantics, and reads distinguish message time from conversation start time.

CONVI-7143's target Hilton slice was historically repaired July 2 and verified as 10 total / 10 powered in CH and UI. July 8 Linda Nesmith misses instead fell in short online-session gaps; they were not proved stale pre-fix data. Closed Conversations source=batch does not identify the complement of powered-by-Agent-Assist. Separately, CONVI-6842 established scorecard-centric PI volume, including all-N/A scorecards, while Closed Conversations remains transcript/message constrained.

## Recorded outcome and limits

Historical target repair verified; behavior documentation refreshed. This does not establish all customers or later dates are repaired. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Use [CONVI-6842](CONVI-6842.md) for the volume decision. For another powered-count miss, trace the exact conversation, agent-message timestamps, online intervals and labeler version before choosing backfill versus semantics work.

## Evidence and timeline

- 2026-06-29 to 2026-07-08 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/agent-stats-analytics-behaviors-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/agent-stats-analytics-behaviors-legacy-evidence/README.md); [deliverables/active-days-behavior-guide-2026-06.md](../sessions/2026-10-25/agent-stats-analytics-behaviors-legacy-evidence/deliverables/active-days-behavior-guide-2026-06.md).
- Durable reference home: [shared-analytics-platform](../subdomains/shared-analytics-platform/README.md).
