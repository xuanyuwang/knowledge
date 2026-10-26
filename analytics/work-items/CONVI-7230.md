# CONVI-7230: Keep customer-provided outcome filters available

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** performance-insights
**Official ticket:** [CONVI-7230](https://linear.app/cresta/issue/CONVI-7230)
**Substantive source dates:** 2026-07-09
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-7230](CONVI-7230.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/config`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

Package Paid is customer-provided CONVERSATION_METADATA with DETAILED_TYPE_METADATA_OUTCOME. enableCLOFilters was incorrectly gating CUSTOMER_OUTCOME and CUSTOMER_OUTCOME_BIN alongside modeled outcomes, hiding the picker and clearing those filter states when the flag was off. The two-file fix keeps customer metadata outcomes available while retaining the modeled-outcome gate.

Follow-up reproduction narrowed the claim: a cross-page saved-filter warning may be legitimate because Performance does not support every filter from Closed Conversations. The final change promises manual picker availability, not removal of every saved-filter warning or a frozen-filter behavior fix. Those speculative follow-ups were discarded.

## Recorded outcome and limits

Draft Director #20606 committed July 9; saved typecheck, focused lint and diff check passed. Merge/deploy not established. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

If resumed, check the final PR/deployment and test customer-provided versus modeled filters with enableCLOFilters off/on. Keep saved-filter compatibility warnings separate from picker availability.

## Evidence and timeline

- 2026-07-09 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-7230-performance-insights-outcome-filters-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-7230-performance-insights-outcome-filters-legacy-evidence/README.md); [sessions/2026-07-09/codex-package-paid-filter.md](../sessions/2026-10-25/convi-7230-performance-insights-outcome-filters-legacy-evidence/sessions/2026-07-09/codex-package-paid-filter.md).
- Durable reference home: [performance-insights](../subdomains/performance-insights/README.md).
