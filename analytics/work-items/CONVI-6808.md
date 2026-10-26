# CONVI-6808: Greenix behavior-trigger criteria showing N/A

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** qa-score
**Official ticket:** [CONVI-6808](https://linear.app/cresta/issue/CONVI-6808)
**Substantive source dates:** 2026-05-09 to 2026-05-13
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-6808](CONVI-6808.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/python-ai-services`; `/Users/xuanyu.wang/repos/director`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

The SD criteria use behavior triggers. Scoring indexes annotations by behavior_id, while raw detection annotations can carry only moment_template_id. For Christian Nelson, the saved comparison found 3 conversations accessible by behavior versus 151 by moment template; Explain ROR also had distinct behavior IDs sharing a template. Missing behavior evidence maps to NOT_APPLICABLE when no configured N/A score exists.

The deeper explanation distinguishes raw moment detection from policy-engine adherence evaluation: raw detections lacking behavior_id are not themselves malformed adherence rows. Global behavior annotation counts were thousands, so this evidence does not prove a global pipeline failure. Low coverage for this agent and behavior selection/routing remain the useful questions. The float_weight=0 hypothesis and historic analytics write-path PRs were ruled out. May 9 API responses had valid latest-week data; the prior screenshot's all-N/A week was transient, with conversation-table lag only a hypothesis.

## Recorded outcome and limits

Historical diagnosis with open annotation/routing and product questions; no recorded deployed fix. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

Trace policy evaluation and behavior selection for matched conversations, including shared-template collisions. Decide whether absent behavior evidence should remain N/A or score zero; do not conflate that product rule with pipeline correctness.

## Evidence and timeline

- 2026-05-09 to 2026-05-13 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-6808-greenix-pi-scores-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-6808-greenix-pi-scores-legacy-evidence/README.md); [deliverables/root-cause-elaboration.md](../sessions/2026-10-25/convi-6808-greenix-pi-scores-legacy-evidence/deliverables/root-cause-elaboration.md).
- Durable reference home: [qa-score](../subdomains/qa-score/README.md).
