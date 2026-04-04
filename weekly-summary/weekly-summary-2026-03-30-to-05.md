# Weekly Summary - Week of 2026-03-30

**Created:** 2026-04-03

## Progresses

### Scorecard Template (NAScore / N/A scoring)

- Shipped `enableNAScore` feature flag via config repo (cresta/config#142788, merged).
- Documented FE template builder investigation in `fe-template-builder.md`.
- Converged on final **D+C** design: N/A is a real option in options/scores with an `isNA` flag (single source of truth; no separate `naScore` field); grader submits `{ notApplicable: true, numericValue }` for scored N/A.
- Confirmed analytics compatibility with existing ClickHouse patterns (row + aggregation filters); no analytics pipeline changes required.
- Mapped grader submission path (`CriterionInputDisplay` → `getPartialScoreForNumericValue` / sentinel handling).
- Reverted uncommitted director FE changes; implementation blocked on design review.
- Updated Coda design doc with full D+C content and resolved open questions.

### CONVI-6247 Agent-Only Filter

- Authored `agent-only-filter-summary.md` (behaviour reference + engineering details) and published to Coda.
- Verified go-servers: all handlers use `GetFilterToAgentsOnly()`; `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` removed on main.
- Verified director: default `listAgentOnly` still `true` on origin/main (planned switch to `false` not yet landed).

## Problems

### Technical Issues

- NAScore: local FE experiments reverted; need design sign-off before re-applying director changes.

### Blockers

- None recorded as hard blockers; primary dependency is design review for D+C before FE/BE implementation.

### Learnings from Failures

- Design iterated Hybrid B+D → D+C to remove dual-store (`naScore` vs AutoQA options) and to support i18n via `isNA` rather than label matching.

## Plan

### Next Week Priorities

1. Close design review on N/A-as-option (`isNA`) and align director + BE on implementation order.
2. Implement D+C in director (gated by `enableNAScore`) and corresponding BE scoring guards (`ComputeScores`, `ComputeCriterionPercentageScore`).
3. Follow through on agent-only filter doc adoption and any remaining default/opt-in alignment with product.

### Follow-ups Required

- Land director changes once NAScore design is approved.
- Track director default for `listAgentOnly` vs. documented plan (`false`).

### Pending Reviews/Decisions

- Formal design approval for D+C NAScore before merging FE.
