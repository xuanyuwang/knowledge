# Mixed-Revision QA Score Semantics

**Pattern family name:** mixed-revision QA score semantics

**Primary subdomain:** [qa-score](../subdomains/qa-score/README.md)

**Validated:** 2026-07-27

## Definition

A class of analytics defects where Performance Insights or Leaderboard surfaces display QA metrics that are **technically consistent with stored score rows**, but **misleading relative to user intent**, because template revisions changed how a criterion contributes meaning (weight, option-to-score mapping, or N/A behavior) while analytics APIs continue to aggregate, filter, or present **revision-derived stored values** without making that historical semantics explicit.

This is an **analytics interpretation and aggregation** problem, not a PostgreSQL ↔ ClickHouse projection failure.

## Core Invariant

Each score row carries percentage, weight, and inclusion semantics derived from the **template revision referenced by that scorecard**. A later template revision does not rewrite historical rows. Analytics must therefore either:

1. aggregate in a revision-aware way,
2. filter with semantics that match the surfaced product question, or
3. make the revision/context explicit in the UI.

When none of these hold, users interpret displayed Yes/No labels or current template intent against numbers computed from older revision semantics.

## Manifestations

| Case | Ticket / report | Surface | Mechanism | Symptom |
|---|---|---|---|---|
| Cross-revision weight domination | [CONVI-7254](../work-items/CONVI-7254.md) | Performance Insights monthly QA | `RetrieveQAScoreStats` sums `percentage × weight` across rows from different revisions; brief weight-1 revisions dominate semantic-weight-zero (`1e-13`) rows | Criterion shows ~0% or 100% while drill-down looks contradictory |
| Revision-derived N/A exclusion | [CONVI-7238](../work-items/CONVI-7238.md) | Manager Leaderboard | `includeNaScored: false` excludes scorecards whose revision-derived aggregate is N/A (`score < 0` in ClickHouse) | Submitted scorecards disappear from evaluated count |
| Pinned-revision option-score inversion | [CONVI-7378](../work-items/CONVI-7378.md) | Closed Conversations + Performance Insights | Scorecard pinned to revision with inverted option mapping (Yes = 0 points); later revision fixes mapping; manual override changes label but not revision semantics | All Yes in section still shows 83.3%; PI shows 0% for criterion after override to Yes |

## Shared Diagnostic Ladder

1. Confirm PG and CH criterion/scorecard fields align. If not, stop and route to `scorecard-data-sync`.
2. Identify the **scorecard's pinned template revision** and the **latest template revision** shown in UI URLs.
3. Compare criterion **weight**, **option-to-score mapping**, and **N/A / exclusion** settings across revisions.
4. Inspect stored row fields: `numeric_value`, `ai_value`, percentage, weight, aggregate `score`.
5. Trace the analytics API path (`RetrieveQAScoreStats`, leaderboard filters, popover queries) and reproduce the exact aggregation/filter SQL.
6. Classify whether the bug is aggregation semantics, inclusion filter semantics, or presentation against current template intent.

## Non-Cases

- Missing or stale ClickHouse `scorecard_d` / `score_d` rows → `scorecard-data-sync`
- Scorecard should not exist, or chapter aggregate not recomputed on update → `scorecard-workflows` / API bug investigation
- Template owner wants historical scorecards rescored after a config fix → `scorecard-template` / rescoring workflow

## Related Artifacts

- [CONVI-7378 SCAN investigation](convi-7378-scan-consent-qa-score-investigation.md) — PM/manager-ready report with initiative options
- [HCD mixed-revision QA score investigation](hcd-mixed-revision-qa-score-investigation.md) — detailed CONVI-7254 evidence
- [QA Score subdomain](../subdomains/qa-score/README.md) — field semantics and invariants

## Initiative Framing (for PM / leadership)

The three known cases (CONVI-7378, CONVI-7254, CONVI-7238) justify a single product initiative: **trustworthy QA analytics across template revisions**. Users interpret Yes/No labels and current template intent, but stored rows and aggregation logic encode historical revision semantics.

Suggested initiative pillars:

1. **Define the contract** — When should analytics use pinned-revision semantics vs current template semantics vs equal per-scorecard observation weight?
2. **Fix aggregation** — Address cross-revision weight domination in `RetrieveQAScoreStats` (CONVI-7254) and inclusion filters in Leaderboard (CONVI-7238).
3. **Improve legibility** — Surface revision context or rescoring status when displayed percentages may disagree with current template meaning (CONVI-7378).
4. **Operational playbooks** — Template-owner workflow when a scoring configuration error is corrected after scorecards already exist.

See [CONVI-7378 investigation](convi-7378-scan-consent-qa-score-investigation.md) for initiative options A–D and recommended customer communication.

## Open Product Questions

- Should criterion-grouped QA stats treat each scorecard observation equally, or use configured template weights even when those weights changed across revisions?
- Should leaderboard counts include submitted scorecards whose revision-derived aggregate is N/A?
- Should analytics surfaces indicate when displayed criterion percentages reflect a superseded revision's option mapping?
