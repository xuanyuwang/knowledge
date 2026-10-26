# CONVI-5805: Match criterion populations across QA cells and popovers

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** qa-score
**Official ticket:** [CONVI-5805](https://linear.app/cresta/issue/CONVI-5805)
**Substantive source dates:** 2025-12-16; overview created 2026-02-07
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-5805](CONVI-5805.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/go-servers`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

A clicked non-scorable criterion intersected with a scorable-only set; the empty intersection fallback broadened the request to all scorable criteria. Relative performance should exclude criteria marked excludeFromQAScores, while exception/overwrite modes may include those criteria but still require score-capable types. The saved solution selected one criterion filter at PerformanceProgression and passed it through table, chapter, totals, top-agent and export requests.

The detailed log also found an All criteria mismatch: enableAutofailScoring could clear criterionIdentifiers and reactivate backend default scorable filtering. Reusing the explicitly selected IDs preserved the intended exception/overwrite population. A fix to QAICell alone therefore did not establish whole-table consistency.

## Recorded outcome and limits

Development/fix narrative and historical branch retained; no PR, merge or current deployment proof recorded. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

For a recurrence, compare actual criterion ID sets for cell, popover, chapter, All criteria and CSV in all three modes; include an empty intersection and excluded-from-QA criterion. Do not widen an explicit empty selection to all criteria.

## Evidence and timeline

- 2025-12-16; overview created 2026-02-07 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/qa-score-popover-fix-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/qa-score-popover-fix-legacy-evidence/README.md); [log/2025-12-16.md](../sessions/2026-10-25/qa-score-popover-fix-legacy-evidence/log/2025-12-16.md).
- Durable reference home: [qa-score](../subdomains/qa-score/README.md).
