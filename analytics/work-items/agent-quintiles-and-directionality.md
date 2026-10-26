# Agent quintiles and score directionality

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** quintiles
**Substantive source dates:** 2026-02-17 to 2026-04-16
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [agent-quintiles-and-directionality](agent-quintiles-and-directionality.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/cresta-proto`; `/Users/xuanyu.wang/repos/config`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The design changed from fixed score bands to relative percentile groups on February 19. Q1 is best, Q5 worst; ranking depends on the selected agents, time range, templates and criterion. The earlier agent-tier document's absolute-band quintile description is superseded. Existing three-tier partitioning was volume weighted, while quintiles partition the ranked population and later use tie-aware boundaries; equal-size buckets are an intent rather than a guarantee when ties remain together.

The per-page reference distinguishes overall ranking from a selected-criterion rank. Coaching Hub and Coaching Plan can agree only with compatible seven-day/default populations. Explicit criterion IDs bypass default scorable-criterion selection, so raw outcome values must not silently be treated as normalized QA scores. CONVI-6219 added lower-is-better directionality work for time/churn; April review notes identify #26616 and supersede treating the March PR table as the final implementation list.

## Recorded outcome and limits

Historical FE/BE PR sequence and partial review states; no present merge or deployment state inferred. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Recheck final ranking, tie handling, lower-is-better wiring, flag and matching request populations before comparing pages. Related tickets include CONVI-6219 and CONVI-6389; this multi-ticket topic preserves the combined history.

## Evidence and timeline

- 2026-02-17 to 2026-04-16 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/agent-quintiles-support-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/agent-quintiles-support-legacy-evidence/README.md); [agent-tier-logic.md](../sessions/2026-10-25/agent-quintiles-support-legacy-evidence/agent-tier-logic.md); [quintile-rank-behaviour-reference.md](../sessions/2026-10-25/agent-quintiles-support-legacy-evidence/quintile-rank-behaviour-reference.md); [log/2026-04-09.md](../sessions/2026-10-25/agent-quintiles-support-legacy-evidence/log/2026-04-09.md); [log/2026-04-16.md](../sessions/2026-10-25/agent-quintiles-support-legacy-evidence/log/2026-04-16.md).
- Durable reference home: [quintiles](../subdomains/quintiles/README.md).
