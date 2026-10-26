# CONVI-6842: Define PI volume independently of Closed Conversations

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** conversation-volume
**Official ticket:** [CONVI-6842](https://linear.app/cresta/issue/CONVI-6842)
**Substantive source dates:** 2026-05-14 to 2026-07-06
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-6842](CONVI-6842.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/go-servers`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The original inconsistency was inside PI: no-template volume came from RetrieveConversationStats, while selected-template volume came from QA stats. A selected template could therefore increase the displayed count. Director #20153 changed no-template volume to RetrieveQAScoreStats with includeNaScored=true, matching the selected-template population.

The July PM decision deliberately permits cross-page differences. PI volume counts scorecarded conversations including all-N/A; Performance progression counts those with a non-N/A score; Closed Conversations requires non-empty message/transcript rows. The saved illustrative counts are 639, 592 and 611 respectively. A separate broad production comparison found 1,031 QA conversations without message rows; these two examples have different populations and must not be mixed.

## Recorded outcome and limits

PR #20153 recorded merged July 6; historical staging evidence and PM decision retained. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Reuse the three named population definitions in customer explanations. For future parity reports, freeze exact filters, template/status/N/A rules and message requirements before treating differing totals as a regression.

## Evidence and timeline

- 2026-05-14 to 2026-07-06 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-6842-holiday-inn-pi-vs-closed-conversations-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-6842-holiday-inn-pi-vs-closed-conversations-legacy-evidence/README.md); [log/2026-07-06.md](../sessions/2026-10-25/convi-6842-holiday-inn-pi-vs-closed-conversations-legacy-evidence/log/2026-07-06.md).
- Durable reference home: [conversation-volume](../subdomains/conversation-volume/README.md).
