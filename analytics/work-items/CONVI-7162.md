# CONVI-7162: Restore Manager scorecard submission-day counts

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** leaderboard
**Official ticket:** [CONVI-7162](https://linear.app/cresta/issue/CONVI-7162)
**Substantive source dates:** 2026-07-02 to 2026-08-24
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-7162](CONVI-7162.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/cresta-proto`; `/Users/xuanyu.wang/repos/config`; `/Users/xuanyu.wang/repos/flux-deployments`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The CONVI-6968 QA migration changed Manager completed-scorecard reporting from submit time to conversation time. July investigation found the same 20 submitted scorecards in PG and CH: submit-day grouping returned two each weekday, while conversation-time grouping reproduced the customer distribution. This was a read-contract issue, not missing projection rows.

On July 30 the product decision restored prior submit-time behavior with explicit ScorecardTimeBasis support and Manager-only opt-in. Historical August 19 official-route testing returned two for Cliff Hawker on August 17 in both aggregate and breakdown. The August 24 follow-up clarifies that historical ranges recompute at query time without backfill, but individual breakdown rows may still display conversation timestamps; correct membership does not imply the row's date label changed.

## Recorded outcome and limits

Historical notes report deployment, global eligible enablement and exact customer verification. Current route/flag status not rechecked. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Preserve submit-time filtering and grouping in both count and drawer. Explain the row timestamp separately if the customer asks; verify current release and effective flag before diagnosing a new mismatch.

## Evidence and timeline

- 2026-07-02 to 2026-08-24 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-7162-holidayinn-manager-scorecards-completed-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-7162-holidayinn-manager-scorecards-completed-legacy-evidence/README.md); [decisions/2026-07-30-manager-leaderboard-submit-time.md](../sessions/2026-10-25/convi-7162-holidayinn-manager-scorecards-completed-legacy-evidence/decisions/2026-07-30-manager-leaderboard-submit-time.md); [log/2026-08-19.md](../sessions/2026-10-25/convi-7162-holidayinn-manager-scorecards-completed-legacy-evidence/log/2026-08-19.md); [log/2026-08-24.md](../sessions/2026-10-25/convi-7162-holidayinn-manager-scorecards-completed-legacy-evidence/log/2026-08-24.md).
- Durable reference home: [leaderboard](../subdomains/leaderboard/README.md).
