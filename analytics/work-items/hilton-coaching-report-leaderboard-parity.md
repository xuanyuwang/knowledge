# Hilton Coaching Report and Manager Leaderboard parity

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** leaderboard
**Substantive source dates:** 2026-01-07 to 2026-02-12
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [hilton-coaching-report-leaderboard-parity](hilton-coaching-report-leaderboard-parity.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/cresta-proto`; `/Users/xuanyu.wang/repos/clickhouse-schema`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

This multi-ticket history belongs nearest Leaderboard because the question compares Manager session counts with Coaching Report. It is not a complete home for coaching workflow lifecycle semantics. APIs diverged on soft-deleted sessions and inactive agents; frontend code also discarded valid session counts whenever efficiency was invalid. Session existence/count validity and efficiency validity are separate contracts.

CONVI-5921 records backend deletion filtering (#25032) and FE fixes (#15934): count sessions even when efficiency is unavailable, use consistent total/day populations, and trust backend efficiency despite empty focus criteria. CONVI-5968 added explicit inclusion for sessions without focus criteria (#7681/#25066/#15953). CONVI-6097 remained implementation work for excludeDeactivatedUsers on RetrieveCoachingSessionStats in the last February note. CONVI-6242 stale-label evidence is already owned by [its canonical work item](CONVI-6242.md); historical speculation about schema changes must not replace that later repair record.

## Recorded outcome and limits

Earlier fixes historically reported released; deactivated-user parity still in progress February 12. No current ticket state asserted. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

If parity work resumes, align deleted-state, active-state, focus-criteria, time and metric validity rules across the two APIs. Continue CONVI-6242 only in its existing item and recheck CONVI-6097 implementation before proposing another fix.

## Evidence and timeline

- 2026-01-07 to 2026-02-12 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/hilton-coaching-discrepancy-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/hilton-coaching-discrepancy-legacy-evidence/README.md); [log/2026-02-12.md](../sessions/2026-10-25/hilton-coaching-discrepancy-legacy-evidence/log/2026-02-12.md).
- Durable reference home: [leaderboard](../subdomains/leaderboard/README.md).
