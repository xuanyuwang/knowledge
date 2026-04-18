# Weekly Summary - Week of 2026-04-12

**Created:** 2026-04-17

## Progresses

### nascore (N/A Score Support)
- **AutoQA bug fixes**: Fixed N/A label display in AutoQA dropdowns (showing "N/A (no score)" instead of "N/A (2)") and fixed score auto-population bug when disabling AutoQA on new criteria
- **Solution B implementation**: Implemented N/A score mapping by passing full criteria map to `MapToScores`, added `isNumericValueNAOption` helper, updated all outcome types (DETECTED, NOT_DETECTED, NOT_APPLICABLE). All tests pass
- **PR review & CI fixes**: Addressed review feedback — removed type casts, inline styles, dead code, extracted callbacks. Merged main (114 commits behind) to fix CI yarn install failures
- **Testing plan**: Created comprehensive testing plan covering 9 sections with all N/A score scenarios
- **Research consolidation**: Moved 20 research documents from `director/.tmp/` to `nascore/research/`

### export-appeal-comments (New Project)
- **Investigation complete**: Identified how criterion comment columns are built in CSV export (`criterionScores[0].Comment.String`). Found `Score.comment` is overloaded — grader comment on regular scorecards, approval reason on appeal resolve scorecards
- **Implementation complete**: Built appeal resolve comment lookup using simple 3-JOIN forward traversal (more efficient than recursive CTE). Modified 5 sites in `action_export_scorecards.go`, added `appealResolveCommentMap` field and `findAppealResolveScorecardIDs()` function

### convi-6247-agent-only-filter
- **Bug identified**: `filterToAgentsOnly: true` with empty groups does not filter out non-agent users because `ShouldQueryAllUsers` short-circuit bypasses agent-only filtering at ClickHouse query level
- **Assistance page fix**: Committed fix threading `filterOptions` through 20 files for Silence/Hold hints and Summary tabs that never migrated to `ParseUserFilterForAnalytics` pattern

### mismatch-scorecard-count
- Committed auto-heal design doc for automated detection and backfill of missing scorecards in ClickHouse (Temporal workflow `backfill-scorecards-by-id`, count-first optimization, 3 phases ~4 SWE-weeks)

### agent-quintiles-support
- Committed deferred PR #26616 review work: refactored `extractLowerIsBetterFieldDefs`, declined `set.Set[string]` for `FieldDefinitionNames`

### scorecard-template
- Updated N/A option design doc with finding #5 about AutoQA dropdown stale values on criterion recreate (cosmetic, pre-existing)

## Problems

### Technical Issues
- **CI failures on nascore PR**: Main branch was 114 commits behind, causing yarn install failures. Resolved by merging main
- **Agent-only filter gap**: `ShouldQueryAllUsers` optimization bypasses agent-only filtering — fundamental design tension between "query all users for performance" and "filter to agents only". Needs ClickHouse-level fix
- **Assistance page never migrated**: Two handlers for Silence/Hold hints and Summary tabs were missed when `ParseUserFilterForAnalytics` pattern was introduced. Fixed by threading `filterOptions` through 20 files

### Learnings
- Simple 3-JOIN forward traversal is more efficient than recursive CTE for original → resolve scorecard lookup chains
- AutoQA uses index (not value) as lookup key for score options — important for N/A score mapping

## Plan

### Next Week Priorities
1. **nascore**: Get N/A score PR through final review and merged
2. **export-appeal-comments**: Submit PR for appeal resolve comment export
3. **convi-6247-agent-only-filter**: Fix the empty-groups + agent-only filter bypass at ClickHouse query level

### Follow-ups Required
- nascore: Verify all 9 testing plan scenarios pass in staging
- convi-6247: Investigate `ShouldQueryAllUsers` interaction with agent-only filter for a proper fix

### Pending Reviews/Decisions
- nascore PR awaiting final review after addressing all feedback
- mismatch-scorecard-count auto-heal design: needs team review before implementation begins
