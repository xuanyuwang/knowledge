# Active Days missing rows versus stale labels

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** active-days
**Substantive source dates:** 2026-02-09 to 2026-02-11
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [active-days-join-and-stale-label-diagnostics](active-days-join-and-stale-label-diagnostics.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/clickhouse-schema`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

Two different failures produced different cells. Source-8-only days disappeared with a LEFT JOIN after ingestion exclusion; FULL OUTER JOIN plus zero coalescing preserved evidence, and the totalAgentCount > 0 guard prevented excluded-source evidence from becoming an active 1. PR #25613 records that repair.

The later Corinne Harmon example had 17 final chat conversations but label rows still pointed at earlier agents and reservations-chat. Creation-time, forward-only labeling had run before reassignment. Changing join shape cannot repair that attribution. The February note proposed lookback/deletion and ended-time alternatives; later canonical CONVI-6242 evidence records the chosen ended-time work and customer backfill. Mutable replacement keys mean FINAL alone cannot remove rows whose keys changed. Earlier assertions that a schema change has zero performance cost are unmeasured proposals.

## Recorded outcome and limits

Join fix and production-data diagnosis recorded; stale-label ticket already has a canonical record. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Use [CONVI-6242](CONVI-6242.md) for continuing stale-label repair and [CONVI-6192](CONVI-6192.md) for source-exclusion history. Revalidate keys, time attribution and valid-source denominator before any new repair.

## Evidence and timeline

- 2026-02-09 to 2026-02-11 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/agent-stats-active-days-fix-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/agent-stats-active-days-fix-legacy-evidence/README.md); [log/2026-02-11.md](../sessions/2026-10-25/agent-stats-active-days-fix-legacy-evidence/log/2026-02-11.md).
- Durable reference home: [active-days](../subdomains/active-days/README.md).
