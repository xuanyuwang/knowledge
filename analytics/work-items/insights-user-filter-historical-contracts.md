# Historical Analytics user-filter contracts

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** insights-user-filter
**Substantive source dates:** 2025-11-27 to 2026-03-17
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [insights-user-filter-historical-contracts](insights-user-filter-historical-contracts.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/cresta-proto`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The legacy investigation separates query population from response metadata and team bucketing. FinalUsers controls the eligible user set; UsersFromGroups is a subset for memberships, so using only the latter for user-only response enrichment loses names. Team mapping is backend metadata plus post-query aggregation, not a team label embedded in every CH query.

Empty authorized populations must not turn into unrestricted queries. Conversely, skipping an explicit user predicate for a root empty filter is only equivalent if role/active-state constraints permit it; ClickHouse does not know agent roles. A March note flags a distinct user-plus-group union issue: totals may include an explicit user with no selected-group bucket. That was a question needing reachability/product verification, not a proven new security defect. Later [CONVI-6719](CONVI-6719.md) is the canonical continuation for unified-parser design and supersedes mechanical reuse of the old plan.

## Recorded outcome and limits

Historical API inventory, consolidation designs and edge-case evidence; not a current implementation inventory. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Use CONVI-6719 for continuing parser work, [CONVI-6260](CONVI-6260.md) for the existing child-team ticket, and the Bswift topic for identity repair. Prove ACL, explicit-user/group union, role and membership equivalence before optimizing calls.

## Evidence and timeline

- 2025-11-27 to 2026-03-17 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/insights-user-filter-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/insights-user-filter-legacy-evidence/README.md); [consolidated-investigation-analysis.md](../sessions/2026-10-25/insights-user-filter-legacy-evidence/consolidated-investigation-analysis.md); [metadata-enrichment-fix-analysis.md](../sessions/2026-10-25/insights-user-filter-legacy-evidence/metadata-enrichment-fix-analysis.md); [empty-filter-non-agent-data-bug.md](../sessions/2026-10-25/insights-user-filter-legacy-evidence/empty-filter-non-agent-data-bug.md); [group-branch-finalusers-mismatch.md](../sessions/2026-10-25/insights-user-filter-legacy-evidence/group-branch-finalusers-mismatch.md); [log/2026-03-17.md](../sessions/2026-10-25/insights-user-filter-legacy-evidence/log/2026-03-17.md).
- Durable reference home: [insights-user-filter](../subdomains/insights-user-filter/README.md).
