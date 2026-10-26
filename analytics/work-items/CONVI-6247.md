# CONVI-6247: Agents-only filtering across Insights

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** insights-user-filter
**Official ticket:** [CONVI-6247](https://linear.app/cresta/issue/CONVI-6247)
**Substantive source dates:** 2025-02-17 to 2026-04-20
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-6247](CONVI-6247.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/cresta-proto`; `/Users/xuanyu.wang/repos/config`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The feature adds a page-level agents-only choice and request-level filterToAgentsOnly. The March 23 product default became false when the control is enabled, while flag-off compatibility must preserve older caller behavior. Manager context and each statistics request require explicit handling; adding the menu is insufficient.

March notes record the main FE and 11-handler BE wiring merged, but April found Assistance children silently dropped filterOptions even though hooks already supported the field. A 20-file follow-up #17931 still missed SummaryUsedLeaderboardByType and GenAIAnswersLeaderboardByType; #18132 fixed those on April 20. The bottom-up trace checked 14 RPC paths: 13 stats requests supported the field, while the conversation-message example drawer used another request contract. The March all-PRs-merged summary therefore does not establish April completeness or rollout.

## Recorded outcome and limits

Core PRs historically merged; April follow-up PR #18132 created. Staging E2E and customer flag activation not established for the final state. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Verify final follow-up merge/deployment, flag-on/off defaults and actual request payloads across Performance, Leaderboard and Assistance. Follow RPC call sites back to props rather than relying only on parent propagation.

## Evidence and timeline

- 2025-02-17 to 2026-04-20 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-6247-agent-only-filter-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-6247-agent-only-filter-legacy-evidence/README.md); [log/2026-04-20.md](../sessions/2026-10-25/convi-6247-agent-only-filter-legacy-evidence/log/2026-04-20.md).
- Durable reference home: [insights-user-filter](../subdomains/insights-user-filter/README.md).
