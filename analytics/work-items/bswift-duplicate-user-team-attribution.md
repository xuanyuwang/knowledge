# Bswift duplicate-user team attribution

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** insights-user-filter
**Substantive source dates:** 2026-03-17 to 2026-04-26
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [bswift-duplicate-user-team-attribution](bswift-duplicate-user-team-attribution.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/director`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

Wrong team attribution came from conversation identity, not merely a team-bucketing bug. Historical conversations were routed to duplicate GUEST_USER IDs whose memberships differed from PROD_USER IDs. April 26 notes record migrating 1,752 Briana and 418 Tennisha PG rows, deleting stale CH keys and backfilling; their post-repair CH guest counts were zero. PG and CH totals were not identical and should not be presented as a full population reconciliation.

Brenda was the reverse case: active guest identity had the correct Diann Ochwat team and conversations, while the production identity was inactive, in default, and had no conversations. A blanket guest-to-production migration would move good data to the wrong account. The retained SQL is an execution record, not a current repair command.

## Recorded outcome and limits

Briana/Tennisha repairs recorded complete; Brenda account decision unresolved in historical notes. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Before any follow-up, verify routing, active identity, team memberships and downstream keys for each person independently. Coordinate any Brenda identity cleanup; do not assume user type determines the authoritative account.

## Evidence and timeline

- 2026-03-17 to 2026-04-26 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/bswift-wrong-team-mapping-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/bswift-wrong-team-mapping-legacy-evidence/README.md); [log/2026-04-26.md](../sessions/2026-10-25/bswift-wrong-team-mapping-legacy-evidence/log/2026-04-26.md).
- Durable reference home: [insights-user-filter](../subdomains/insights-user-filter/README.md).
