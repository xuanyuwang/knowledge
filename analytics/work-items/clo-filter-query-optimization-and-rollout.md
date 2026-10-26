# CLO filter semantics, query optimization and rollout

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** insights-user-filter
**Substantive source dates:** 2026-06-17 to 2026-08-23
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [clo-filter-query-optimization-and-rollout](clo-filter-query-optimization-and-rollout.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/clickhouse-schema`; `/Users/xuanyu.wang/repos/config`; `/Users/xuanyu.wang/repos/flux-deployments`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

CLO is Cresta-modeled CONVERSATION_OUTCOME data, distinct from customer-uploaded metadata outcomes. The frontend picker and backend support grew into typed storage/MV optimization and separate rollout work. The raw query path parsed JSON from broad moment_annotation_d and could drag metadata groups off their narrow optimized view; mixed-filter routing therefore needed attention beyond adding a picker.

July measurement recorded an 85.83-second six-month raw CLO query versus 66.67 seconds for the closest no-CLO request, with materially higher reads. August's 37.5–40.5-second browser refresh after optimization is directional evidence only: filters, cache, window and measurement layer differ, so it is not a controlled speedup or p95. The storage design uses a narrow typed target table, trigger MV without POPULATE and a distributed read table plus backfill. Physical-replica bytes and equivalent allocated cost are not incremental billing.

[CONVI-7383](CONVI-7383.md) owns the historical schema/backfill rollout; [CONVI-7543](CONVI-7543.md) owns feature GA. Source history and successful sync workflows did not prove durable effective config: August 23 fleet replay succeeded, but ConfigService/Admin read-back remained pending. Comcast/Schwab exclusions and schema exceptions must stay explicit.

## Recorded outcome and limits

Historical backend support, eligible backfills and config repair/replay recorded; effective GA read-back still open. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Follow the two canonical rollout tickets. Verify effective flags and query routing independently, preserve exclusions, and benchmark identical request populations before publishing performance claims.

## Evidence and timeline

- 2026-06-17 to 2026-08-23 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-7049-clo-filter-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-7049-clo-filter-legacy-evidence/README.md); [work-items/CONVI-7383.md](../sessions/2026-10-25/convi-7049-clo-filter-legacy-evidence/work-items/CONVI-7383.md); [work-items/CONVI-7543.md](../sessions/2026-10-25/convi-7049-clo-filter-legacy-evidence/work-items/CONVI-7543.md); [sessions/2026-08-23/codex-remote-master-verification.md](../sessions/2026-10-25/convi-7049-clo-filter-legacy-evidence/sessions/2026-08-23/codex-remote-master-verification.md).
- Durable reference home: [insights-user-filter](../subdomains/insights-user-filter/README.md).
