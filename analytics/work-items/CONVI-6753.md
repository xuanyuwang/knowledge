# CONVI-6753: Zero-weight criteria show N/A in Performance Insights

**Record state:** Fix recorded as implemented with PR pending in May; current publication/repair status not rechecked.
**Primary domain:** analytics
**Primary subdomain:** qa-score
**Official ticket:** [CONVI-6753](https://linear.app/cresta/issue/CONVI-6753)
**Last substantive source update:** 2026-05-14
**Consolidated:** 2026-10-25

## Objective and Impact

Restore Home Care Delivered's valid criterion display after zero-weight rows reached ClickHouse with a zero denominator. Role: investigated the write-path regression and documented a compatibility fix and repair requirement.

## Source Context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`; historical comparison repo: `historic-analytics`.
- Historical worktree: `/Users/xuanyu.wang/repos/go-servers-convi-6753`; branch: `convi-6753-fix-weight-zero-float` (registered in `workspace/repos.yaml`, not revalidated here).
- Recorded fix location: `shared/clickhouse/conversations/scorecard_score.go`; helper: `BuildScoreRowsFromDirectorScores`.
- [Original investigation](../sessions/2026-10-25/convi-6753-legacy-evidence/README.md), [May 13 log](../sessions/2026-10-25/convi-6753-legacy-evidence/log/2026-05-13.md).

## Findings and Decisions

The historical record attributes the regression to the direct-from-director write path bypassing the old historic PostgreSQL projection, which had supplied a `1e-13` compatibility weight. Direct writes retained `FloatWeight=0`; criterion aggregation `SUM(percentage * float_weight) / SUM(float_weight)` then had no nonzero denominator. A criterion configured to contribute zero weight to the total is not necessarily an inapplicable criterion.

The recorded compatibility fix substitutes `1e-13` when the derived percentage weight is zero and names `TestBuildScoreRowsFromDirectorScores_WeightZeroUsesMinFloat`. It is a proposal/implementation record for that write path, not a new general recommendation to assign epsilon weights to all excluded or N/A scores. The archived explanation of the old PostgreSQL default was not re-proved during migration.

## Validation and Next Actions

- The saved README says “fix implemented, PR pending” and calls for a Performance Config Backfill from April 28 onward after merge. It provides no completed rollout or customer-backfill result.
- An earlier April 28–May 7 backfill had not corrected the issue; rewriting with the unchanged writer is not a fix.
- If reopened, inspect current writer behavior, criterion versus overall aggregation, referenced tests, PR history, and customer repair state before selecting another action.
- Durable semantics: [QA Score](../subdomains/qa-score/README.md); presentation: [Performance Insights](../subdomains/performance-insights/README.md).

## Timeline

- 2026-05-13 — Investigation, regression timeline, compatibility fix, and worktree registration recorded.
- 2026-05-14 — Last substantive Git update to the original project.
- 2026-07-15 — Legacy README gained domain navigation.
- 2026-10-25 — Evidence consolidated; see [migration session](../sessions/2026-10-25/codex-five-legacy-project-consolidation.md).
