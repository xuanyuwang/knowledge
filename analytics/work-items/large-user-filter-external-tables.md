# Large user-filter lists and external-table regressions

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** insights-user-filter
**Substantive source dates:** 2026-03-09 to 2026-03-24
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [large-user-filter-external-tables](large-user-filter-external-tables.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/flux-deployments`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

Large group/ACL/active-user expansions made inline IN lists exceed query size. ShouldQueryAllUsers reduced text for unrestricted populations but could not safely replace role-restricted membership. External tables moved user IDs out of query text. March history records #26178/#26250 and staged/global flag changes, including delayed release propagation; desired config alone was not proof every stage received it.

CONVI-6476 exposed a downstream SQL-builder assumption: Live Assist duplicated a positional user argument for agent OR manager predicates. External-table predicates have no placeholder, so appending nil shifted later use-case arguments and returned empty rows. #26519 guarded duplication when the argument was non-nil. This is a query/argument-shape compatibility lesson, not evidence external tables inherently lose data.

A separate staging QA empty response was correct: the selected criterion had zero score rows and the template had no rows in the requested window. That control case prevents interpreting every empty response during rollout as an external-table regression.

## Recorded outcome and limits

Historical external-table merge/rollout and Live Assist fix recorded. Current client behavior and deployment not verified. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Before another transport/query rewrite, compare eligible sets and validate placeholders/args for predicates reused under multiple user axes. Confirm requested data exists independently; use CONVI-6476 if that specific regression is reopened.

## Evidence and timeline

- 2026-03-09 to 2026-03-24 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/large-user-id-clickhouse-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/large-user-id-clickhouse-legacy-evidence/README.md); [convi-6476-live-assist-broken.md](../sessions/2026-10-25/large-user-id-clickhouse-legacy-evidence/convi-6476-live-assist-broken.md); [validation-qa-score-stats-empty.md](../sessions/2026-10-25/large-user-id-clickhouse-legacy-evidence/validation-qa-score-stats-empty.md).
- Durable reference home: [insights-user-filter](../subdomains/insights-user-filter/README.md).
