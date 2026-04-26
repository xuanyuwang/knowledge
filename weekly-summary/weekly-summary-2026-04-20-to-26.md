# Weekly Summary - Week of 2026-04-20

**Created:** 2026-04-26

## Progresses

### convi-6247-agent-only-filter
- Completed full bottom-up API call trace across all 14 Assistance page API calls to verify `filterToAgentsOnly` coverage
- Found and fixed 2 missed components (`SummaryUsedLeaderboardByType`, `GenAIAnswersLeaderboardByType`) that silently dropped `filterOptions`
- Created follow-up PR [#18132](https://github.com/cresta/director/pull/18132)
- All 13 stats APIs now confirmed passing `filterToAgentsOnly`; only exception is `retrieveConversationMessages` (drill-down drawer, doesn't support the field)

### nascore (N/A Score Support)
- Fixed save-time validation error for # of Occurrences criteria with scored N/A — root cause was length mismatch between `auto_qa.options` and `settings.scores` when N/A is scored
- Addressed PR review: extracted `SettingsScore` type to module scope
- Completed comprehensive lifecycle analysis of options/scores/auto_qa data flow across all stages (DB → Load → Form → Edit → Save → DB)
- Identified 8 pain points including latent bug (`not_applicable` not remapped on legacy load) and fragile save fallback
- Cross-validated with Codex-produced refactor design; reconciled both into unified direction with canonical invariants and two-level module split
- Created `options-scores-lifecycle.md` reference document and incorporated 3 rounds of review feedback (P1: misleading invariant table, P1: over-broad save abstraction, P2: missing mutation entrypoints)

### user-filter-consolidation
- Reviewed and addressed PR #26451 (Phase 2) feedback from tinglinliu
- Added `NewParseResult()` constructor to prevent nil-map panics
- Improved `WithCoachingDefaults` godoc to warn about ordering sensitivity with `WithIncludeDevUsers()`

### mismatch-scorecard-count
- Expanded auto-heal monitoring design from single submitted-scorecard check to three views (all, submitted, unsubmitted)
- Refactored monitor flow: single PG inventory, in-memory set derivation, targeted backfills by conversation vs. process scorecards
- Added concrete implementation sequencing across repos and updated metrics/alerting/test plans

## Problems

### Technical Issues
- Top-down code scanning missed components where TypeScript props interfaces silently dropped `filterOptions` — resolved by switching to bottom-up API call tracing
- N/A score validation broke silently because `auto_qa.options` and `settings.options` have different N/A semantics (separate field vs. inline entry)

### Learnings from Failures
- **Top-down vs. bottom-up verification**: Tracing props from parent → child misses cases where intermediate TypeScript interfaces drop fields silently. Bottom-up (find all API calls → trace backward to components) is more reliable for verifying end-to-end coverage
- **Value semantics drift**: The nascore options/scores lifecycle has `option.value` meaning different things at different stages (score value in form state, index in DB). Both Codex and manual analysis independently identified this as the core fragility

## Plan

### Next Week Priorities
1. Land nascore PR with validation fix and type cleanup
2. Begin nascore options/scores refactor based on lifecycle analysis — start with `CriterionOptionsManager` pure functions
3. Follow up on convi-6247 PR #18132 merge
4. Continue user-filter-consolidation Phase 2 PR review cycle

### Follow-ups Required
- Verify convi-6247 follow-up PR #18132 gets reviewed and merged
- Fix latent `not_applicable` remap bug in legacy load path (nascore)
- Address save fallback risk where `option.value` returns index instead of score

### Pending Reviews/Decisions
- user-filter-consolidation PR #26451 — awaiting re-review after addressing comments
- mismatch-scorecard-count auto-heal design — needs stakeholder alignment before implementation
