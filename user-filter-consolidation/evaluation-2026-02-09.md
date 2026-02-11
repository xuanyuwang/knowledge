# User Filter Consolidation - Re-evaluation

**Date**: 2026-02-09
**Purpose**: Re-evaluate the project against the current codebase state

---

## Executive Summary

The original project goal was to **unify `ParseUserFilterForAnalytics` into `Parse`** by lifting the analytics-specific logic into the shared `user-filter` package. This has **not been done**. Instead, the codebase evolved via a different strategy: **gradually migrating individual APIs to use `ParseUserFilterForAnalytics`**, gated by a feature flag. The two functions remain separate, and the migration is roughly 40% complete.

**Recommendation**: The original unification plan remains sound in principle but may not be the highest priority. The more pressing work is completing the migration of the remaining ~17 APIs to `ParseUserFilterForAnalytics` and removing the feature flag. The unification into `shared/user-filter` can be a follow-up.

---

## Current State of the Codebase

### Two Separate Implementations Still Exist

| Function | Location | Callers |
|----------|----------|---------|
| `Parse` | `shared/user-filter/user_filter.go:315` | 3 coaching callers in apiserver |
| `ParseUserFilterForAnalytics` | `insights-server/internal/analyticsimpl/common_user_filter.go:131` | 12 insights-server APIs (migrated) |
| Old pattern (ApplyResourceACL + ListUsersMappedToGroups + MoveFiltersToUserFilter) | `insights-server/internal/shared/common.go` | ~17 insights-server APIs (not migrated) |

### Migration Progress

**Migrated to `ParseUserFilterForAnalytics` (12 APIs):**
1. `retrieve_agent_stats.go`
2. `retrieve_conversation_stats.go`
3. `retrieve_guided_workflow_stats.go`
4. `retrieve_hint_stats.go`
5. `retrieve_knowledge_assist_stats.go`
6. `retrieve_knowledge_base_stats.go`
7. `retrieve_live_assist_stats.go`
8. `retrieve_note_taking_stats.go`
9. `retrieve_qa_score_stats.go`
10. `retrieve_smart_compose_stats.go`
11. `retrieve_suggestion_stats.go`
12. `retrieve_summarization_stats.go`

**Still on old pattern (~17 APIs):**
1. `retrieve_appeal_stats.go`
2. `retrieve_assistance_stats.go`
3. `retrieve_chat_bot_conversation_stats.go`
4. `retrieve_chat_bot_session_stats.go`
5. `retrieve_coaching_efficiency_stats.go`
6. `retrieve_coaching_session_stats.go`
7. `retrieve_comment_stats.go`
8. `retrieve_conversation_message_stats.go`
9. `retrieve_conversation_outcome_stats.go`
10. `retrieve_director_task_stats.go`
11. `retrieve_group_calibration_stats.go`
12. `retrieve_manager_stats.go`
13. `retrieve_manual_qa_stats.go`
14. `retrieve_qm_task_stats.go`
15. `retrieve_scorecard_criteria_stats.go`
16. `retrieve_scorecard_stats.go`
17. `retrieve_system_stats.go`

### Feature Flag

`ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` is still active in `analyticsimpl.go:161-165`, gating whether the new or old path is used. This means even the 12 "migrated" APIs can fall back to the old pattern if the flag is off.

---

## Evaluation of the Original Plan

### What the Plan Proposed (4 Phases)

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1: Prepare shared/user-filter | Add `ParseOptions`, new client container, extend `FilteredUsersAndGroups` | **Not started** |
| Phase 2: Implement unified logic | Port `listAllUsers`, `applyResourceACL`, `buildUserGroupMappings` into shared package | **Not started** |
| Phase 3: Migrate callers | Update all `retrieve_*_stats.go` and coaching callers | **Partially done** (12/29 insights APIs migrated to `ParseUserFilterForAnalytics`, coaching untouched) |
| Phase 4: Cleanup | Remove `ParseUserFilterForAnalytics`, remove old `Parse` | **Not started** |

### What Actually Happened Instead

The team took a **pragmatic incremental approach**: migrate individual APIs from the old 3-step pattern to `ParseUserFilterForAnalytics`, one by one, behind a feature flag. This avoids the riskier "big bang" refactor that the original plan proposed.

Key commits since June 2025:
- `a1d466f6` - CONVI-6010: Migrated `RetrieveSummarizationStats`
- `fb2272ef` - CONVI-5173: Added `ListAgentOnly` support
- `44b5e129` - CONVI-5173: Fixed group filter bug in `ParseUserFilterForAnalytics`
- `69ecdcf5` - CONVI-6175: Added `ShouldQueryAllUsers` flag for ClickHouse query size limit
- `7387fa7b` - Extracted duplicated user filter logic into helper function

---

## Analysis: Is the Original Plan Still Valid?

### Arguments FOR the unification (merging into shared/user-filter)

1. **Single source of truth** - Having one `Parse` function with options is cleaner than two parallel implementations
2. **Coaching callers benefit** - The 3 coaching callers would gain analytics-grade filtering (profile scoping, ground truth, deactivated user handling)
3. **Easier maintenance** - Bug fixes in one place instead of two
4. **Removes 700+ lines** - `common_user_filter.go` could be deleted

### Arguments AGAINST (or for deferring)

1. **Risk** - The unification touches shared infrastructure used by both apiserver and insights-server. A bug affects everything.
2. **The incremental approach is working** - APIs are being migrated one by one with targeted JIRA tickets, and bugs are being found and fixed along the way (e.g., CONVI-5173 group filter fix)
3. **Coaching callers don't need it** - The 3 coaching callers only use `UserNames` from the result. They work fine with the current `Parse`.
4. **Unused parameters problem is contained** - The 3 unused params in `ParseUserFilterForAnalytics` are annoying but harmless. They can be cleaned up independently.
5. **Feature flag still active** - The new pattern hasn't been fully validated in production for all APIs yet. Unifying before that is premature.

### Verdict

**Defer the unification. Prioritize completing the API migration first.**

The original plan's analysis (LiteUser mapping, unused params, cache analysis) is still accurate and valuable. But the execution strategy should be:

1. **Finish migrating the remaining ~17 APIs** to `ParseUserFilterForAnalytics`
2. **Remove the feature flag** once all APIs are migrated and stable
3. **Clean up unused parameters** (the 3 identified: `enableListUsersCache`, `listUsersCache`, `shouldMoveFiltersToUserFilter`)
4. **Then consider** the unification into `shared/user-filter` as a separate project

---

## What Changed Since the Analysis Was Written

### Analysis Documents Still Accurate?

| Document | Accuracy | Notes |
|----------|----------|-------|
| `comparison-parse-functions.md` | **Mostly accurate** | Core behavioral differences still hold. `ParseUserFilterForAnalytics` signature unchanged (still 17 params). New additions: `ShouldQueryAllUsers` flag, `ApplyUserFilterFromResult` helper. |
| `analysis-unused-params.md` | **Still accurate** | The 3 unused params (`enableListUsersCache`, `listUsersCache`, `shouldMoveFiltersToUserFilter`) are still present and unused in `common_user_filter.go:131-148`. |
| `analysis-cache-usage.md` | **Still accurate** | Cache params still declared but not used in `ParseUserFilterForAnalytics`. |
| `README.md` (plan) | **Structurally valid, execution diverged** | The proposed data structures and migration phases are sound but the team chose a different execution path. |

### New Developments Not in Original Analysis

1. **`ShouldQueryAllUsers` flag** - Added to `ParseUserFilterResult` to optimize ClickHouse queries when no user filter is specified. Avoids query size limits.
2. **`ApplyUserFilterFromResult` helper** - Extracted as a helper to reduce duplication across migrated APIs.
3. **`expandGroupsToUsers` with UNION semantics** - Fixed a bug where ACL group expansion used intersection instead of union.
4. **More APIs exist** - Several APIs not in the original analysis exist (appeal, assistance, chatbot, director task, etc.) that also need migration.

---

## Recommended Next Steps

### Immediate (complete the migration)
1. Migrate the remaining ~17 APIs to `ParseUserFilterForAnalytics`
2. Each migration should be a separate PR with its own JIRA ticket (following CONVI-60xx pattern)
3. Priority order: scorecard APIs first (most customer-facing), then others

### Short-term (stabilize)
4. Remove `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` feature flag once all APIs are migrated and stable
5. Remove old helper functions from `shared/common.go` (`ListUsersMappedToGroups`, `MoveFiltersToUserFilter`)
6. Clean up the 3 unused parameters in `ParseUserFilterForAnalytics`

### Long-term (optional unification)
7. Consider moving `ParseUserFilterForAnalytics` logic into `shared/user-filter` as a new `ParseV2` or enhanced `Parse` with options
8. Migrate coaching callers to the new API
9. Delete `common_user_filter.go`

---

## Updated Project Status

| Metric | Value |
|--------|-------|
| APIs migrated to new pattern | 12 / ~29 (41%) |
| Feature flag removed | No |
| Unused params cleaned up | No |
| Old shared helpers removed | No |
| Unification into shared package | Not started |
| Coaching callers updated | Not started |
