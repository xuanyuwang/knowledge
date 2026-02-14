# User Filter Consolidation — Detailed Implementation Plan

**Created**: 2026-02-09
**Updated**: 2026-02-13

---

## Scope

Consolidate user filter logic from 3 locations into a single shared package (`shared/user-filter/`):
1. `shared/user-filter/user_filter.go` — coaching path (3 callers)
2. `insights-server/internal/analyticsimpl/common_user_filter.go` — new analytics path (12 callers)
3. Old inline pattern in `insights-server/` — `ApplyResourceACL` + `ListUsersMappedToGroups` + `MoveFiltersToUserFilter` (15 callers)

**Total callers: 30** (12 new analytics + 15 old analytics + 3 coaching)

---

## Caller Inventory

### Already on `ParseUserFilterForAnalytics` (12)

| File | Function | Pattern |
|------|----------|---------|
| `retrieve_agent_stats.go` | `RetrieveAgentStats` | Agent leaderboard |
| `retrieve_conversation_stats.go` | `RetrieveConversationStats` | Agent leaderboard |
| `retrieve_guided_workflow_stats.go` | `RetrieveGuidedWorkflowStats` | Agent leaderboard |
| `retrieve_hint_stats.go` | `RetrieveHintStats` | Agent leaderboard |
| `retrieve_knowledge_assist_stats.go` | `RetrieveKnowledgeAssistStats` | Agent leaderboard |
| `retrieve_knowledge_base_stats.go` | `RetrieveKnowledgeBaseStats` | Team leaderboard |
| `retrieve_live_assist_stats.go` | `RetrieveLiveAssistStats` | Agent leaderboard |
| `retrieve_note_taking_stats.go` | `RetrieveNoteTakingStats` | Time-range only |
| `retrieve_qa_score_stats.go` | `RetrieveQAScoreStats` | Agent leaderboard |
| `retrieve_smart_compose_stats.go` | `RetrieveSmartComposeStats` | Agent leaderboard |
| `retrieve_suggestion_stats.go` | `RetrieveSuggestionStats` | Agent leaderboard |
| `retrieve_summarization_stats.go` | `RetrieveSummarizationStats` | Agent leaderboard |

### Still on old pattern (15)

| File | Function | Pattern |
|------|----------|---------|
| `retrieve_adherences.go` | `RetrieveAdherences` | Agent leaderboard |
| `retrieve_annotated_time_series.go` | `RetrieveAnnotatedTimeSeries` | Other |
| `retrieve_assistance_stats.go` | `RetrieveAssistanceStats` | Time-range only |
| `retrieve_closed_conversations.go` | `RetrieveClosedConversations` | Time-range only |
| `retrieve_coaching_session_stats.go` | `RetrieveCoachingSessionStats` | Agent leaderboard |
| `retrieve_comment_stats.go` | `RetrieveCommentStats` | Time-range only |
| `retrieve_conversation_message_stats.go` | `RetrieveConversationMessageStats` | Agent leaderboard |
| `retrieve_conversation_messages.go` | `RetrieveConversationMessages` | Time-range only |
| `retrieve_conversation_outcome_stats.go` | `RetrieveConversationOutcomeStats` | Agent leaderboard |
| `retrieve_manager_stats.go` | `RetrieveManagerStats` | Team leaderboard |
| `retrieve_manual_qa_progress.go` | `RetrieveManualQAProgress` | Agent leaderboard |
| `retrieve_manual_qa_stats.go` | `RetrieveManualQAStats` | Agent leaderboard |
| `retrieve_partial_conversations.go` | `RetrievePartialConversations` | Time-range only |
| `retrieve_scorecard_criteria_stats.go` | `RetrieveScorecardCriteriaStats` | Agent leaderboard |
| `retrieve_scorecard_stats.go` | `RetrieveScorecardStats` | Agent leaderboard |

### Coaching callers (3)

| File | Function | Pattern |
|------|----------|---------|
| `apiserver/.../action_list_coaching_plans.go` | `ListCoachingPlans` | Coaching |
| `apiserver/.../action_retrieve_coaching_overviews.go` | `RetrieveCoachingOverviews` | Coaching |
| `apiserver/.../action_retrieve_coaching_progresses.go` | `RetrieveCoachingProgresses` | Coaching |

---

## PR Sequence

### Phase 1: Behavioral Test Suite

**Goal**: Build a comprehensive test suite that encodes all behaviors from the behavioral standard. These tests serve as the regression guard for all subsequent changes.

Tests are written against the **existing** `ParseUserFilterForAnalytics` function first (to verify they pass today), then later retargeted at the new unified implementation.

#### PR 1.1: Core behavioral tests — ACL + base population

**Files**: `insights-server/internal/analyticsimpl/common_user_filter_test.go`

Add test cases for behaviors not currently covered:
- B-ACL-2: Limited access narrows selections to managed set (intersection, not replacement)
- B-ACL-4/5: UNION of managed users and managed groups (already exists as `UnionOfUsersAndGroupsFromACL`, verify it's thorough)
- B-BP-1: Base population fetched first — add test where group expansion returns a non-agent user, verify it's excluded
- B-BP-3: All results within base population — add test where ACL returns users outside base population
- B-BP-4: Metadata from base population — add test where ACL user has different FullName than base population

**Size**: ~200-300 lines of tests

#### PR 1.2: Selection filtering behavioral tests

**Files**: `insights-server/internal/analyticsimpl/common_user_filter_test.go`

Add test cases:
- B-SF-1: Empty selection = all accessible users (both ACL states)
- B-SF-2: Selection restricts results (both users and groups)
- B-SF-3: Combined user + group selections use UNION — **this will FAIL for ParseUserFilterForAnalytics** (known Divergence 5: it uses INTERSECTION). Document the failure as expected.
- B-SF-4: Non-existent entries silently dropped
- B-SF-5: Invalid resource names return error
- B-SF-6: Selection filtering works in all ACL states

**Size**: ~200-300 lines of tests

#### PR 1.3: Group mechanics + deactivated user tests

**Files**: `insights-server/internal/analyticsimpl/common_user_filter_test.go`

Add test cases:
- B-GS-1: Groups expand to member users
- B-GS-2: Group types handled separately (TEAM vs DYNAMIC)
- B-GS-3: Direct membership controls group→user expansion (add two-direction test)
- B-GS-4: Unparseable group names silently skipped
- B-GH-4: Child teams must appear in GroupsToAggregate — **this will document the current bug in `ParseUserFilterForAnalytics`** (known Divergence 10: `FetchGroups` does not expand child groups). Test asserts the current (broken) behavior where only the parent group appears in `GroupsToAggregate`.
- B-DU-1/B-DU-2: Deactivated users excluded/included (complex scenarios: deactivated + ACL + groups)
- B-GM-1: Both direct and indirect memberships tracked in output maps
- B-GM-3: Only TEAM groups in mappings

**Size**: ~200-300 lines of tests

#### PR 1.4: Query optimization + edge case tests

**Files**: `insights-server/internal/analyticsimpl/common_user_filter_test.go`

Add test cases:
- B-QO-1 through B-QO-4: ShouldQueryAllUsers in all states (some already exist, add missing combos)
- B-SO-1/B-SO-2: Sorting guarantees (resource name, deterministic)
- B-PS-1/B-PS-2: Profile scoping
- B-PG-2/B-PG-3: Pagination for large sets
- Combination tests: ACL limited + group selection + deactivated + direct-only

**Size**: ~200-300 lines of tests

---

### Phase 2: New Types and Interface in Shared Package

**Goal**: Define the new public API in `shared/user-filter/` without implementing it yet. This lets us validate the API design and get team review before writing logic.

#### PR 2.1: New types and interface

**Files**: NEW `shared/user-filter/types.go`, UPDATE `shared/user-filter/BUILD.bazel`

```go
// types.go

// ParseOptions contains per-call parameters for user filter parsing.
type ParseOptions struct {
    CustomerID              string
    ProfileID               string
    SelectedUsers           []string   // User resource names
    SelectedGroups          []string   // Group resource names (both TEAM and DYNAMIC)
    ExcludeDeactivatedUsers bool
    DirectMembershipsOnly   bool
    ListAgentOnly           bool
    IncludePeerUserStats    bool
    Roles                   []authpb.AuthProto_Role
    UserTypes               []enums.UserType
    GroupRoles              []authpb.AuthProto_Role
    State                   userpb.User_State
}

// ParseResult contains the output of user filter parsing.
type ParseResult struct {
    FinalUsers         map[string]LiteUser        // Key = user resource name
    FinalGroups        map[string]LiteGroup       // Key = group resource name
    UserToDirectGroups map[string][]LiteGroup     // Key = user resource name → direct TEAM memberships
    UserToAllGroups    map[string][]LiteGroup     // Key = user resource name → all TEAM memberships
    GroupToDirectMembers map[string][]LiteUser    // Key = group resource name → direct members
    GroupToAllMembers    map[string][]LiteUser    // Key = group resource name → all members
    AllGroups          map[string]LiteGroup       // All TEAM groups encountered
    ShouldQueryAllUsers bool
}

// LiteUser is a lightweight user representation with metadata.
type LiteUser struct {
    ResourceName string
    Username     string
    FullName     string
    UserID       string
}

// LiteGroup is a lightweight group representation with metadata.
type LiteGroup struct {
    ResourceName string
    DisplayName  string
    GroupType    string // "TEAM" or "DYNAMIC"
    GroupID      string
}

// Parser handles user filter operations. Dependencies are injected
// via constructor; per-call parameters go in ParseOptions.
type Parser struct {
    internalUserClient internaluserpb.InternalUserServiceClient
    userClient         userpb.UserServiceClient
    configClient       config.Client
    aclHelper          auth.ResourceACLHelper
    logger             log.Logger
}

func NewParser(
    internalUserClient internaluserpb.InternalUserServiceClient,
    userClient         userpb.UserServiceClient,
    configClient       config.Client,
    aclHelper          auth.ResourceACLHelper,
    logger             log.Logger,
) *Parser
```

**Size**: ~100-150 lines

#### PR 2.2: Post-processing utilities

**Files**: NEW `shared/user-filter/utilities.go`, UPDATE `shared/user-filter/BUILD.bazel`

```go
// utilities.go

func StripRootAndDefaultGroups(userToGroups map[string][]LiteGroup) map[string][]LiteGroup
func UserIDs(users map[string]LiteUser) ([]string, error)
func GroupNamesToStrings(userToGroups map[string][]LiteGroup) map[string][]string
func ApplyToQuery(result *ParseResult, users *[]*userpb.User, groups *[]*userpb.Group) (shouldEarlyReturn bool)
```

These are pure functions — implement them immediately with unit tests.

**Size**: ~150-200 lines (implementation + tests)

---

### Phase 3: Unified Implementation

**Goal**: Implement the new `Parse` method. All behavioral tests from Phase 1 should pass (after retargeting).

#### PR 3.1: Core parse implementation — base population + ACL

**Files**: NEW `shared/user-filter/parse.go`

Implement:
- `(p *Parser) Parse(ctx context.Context, opts ParseOptions) (*ParseResult, error)`
- Internal: `listBasePopulation()` — calls `ListUsersForAnalytics` with pagination
- Internal: `applyACL()` — calls ACL helper, determines state, applies intersection
- Metrics: `shared.user_filter.parse.request.count`, `.error.count`, `.duration_ms`
- Logging: entry, base population count, ACL before/after, exit

Move/adapt from `common_user_filter.go`:
- `listAllUsers()` → `listBasePopulation()`
- `applyResourceACL()` → `applyACL()`
- `expandGroupsToUsers()` (for ACL group expansion)
- `updateGroundTruthUsers()` → intersection logic

**Size**: ~300-400 lines (may need to be this large since it's the core logic)

#### PR 3.2: Selection filtering + group expansion

**Files**: `shared/user-filter/parse.go`

Implement:
- Internal: `applySelections()` — UNION of selected users + expanded group members, intersect with base population
- Internal: `expandGroupsToUsers()` — expand TEAM and DYNAMIC groups separately
- Internal: `filterUsersByGroups()` — match users to groups, respect DirectMembershipsOnly
- UNION semantics (not INTERSECTION — this is the Divergence 5 fix)

Move/adapt from `common_user_filter.go`:
- `filterUsersByGroups()`
- `GroupsByGroupType()` (already in shared package)

**Size**: ~200-300 lines

#### PR 3.3: Group membership tracking + output construction

**Files**: `shared/user-filter/parse.go`

Implement:
- Internal: `buildMembershipMaps()` — build UserToDirectGroups, UserToAllGroups, GroupToDirectMembers, GroupToAllMembers, AllGroups
- Internal: `buildResult()` — assemble ParseResult, sort by resource name
- `ShouldQueryAllUsers` computation

Move/adapt from `common_user_filter.go`:
- `buildUserGroupMappings()` — split into two maps (direct + all), remove hasAgentAsGroupByKey coupling

**Critical fix**: Child group expansion (Divergence 10 / B-GH-4). The current `buildUserGroupMappings` uses `FetchGroups` which only returns explicitly-requested groups — child teams are not discovered. The unified implementation must expand child groups from parent group memberships (matching the old path's `ListGroups` behavior). Without this, team leaderboards for parent teams only show one row. See CONVI-6260.

**Size**: ~200-300 lines

#### PR 3.4: Retarget behavioral tests to new implementation

**Files**: `shared/user-filter/parse_test.go` (NEW)

Create a new test suite that:
- Uses the same test scenarios from Phase 1
- Targets `Parser.Parse()` instead of `ParseUserFilterForAnalytics`
- All tests should pass (including B-SF-3 UNION test that previously failed, and B-GH-4 child group expansion test that previously documented the bug)

**Size**: ~500-800 lines (comprehensive test suite)

---

### Phase 4: Feature Flag + Shadow Mode Infrastructure

#### PR 4.1: Add proto config flag

**Files**:
- `cresta-proto/.../insights.proto` — add field 21: `bool enable_unified_user_filter = 21;`
- `insights-server/internal/analyticsimpl/analyticsimpl.go` — read config flag

**Size**: ~20-30 lines

#### PR 4.2: Shadow mode comparison helper

**Files**: NEW `shared/user-filter/compare.go`

```go
// CompareResults compares old ParseUserFilterResult with new ParseResult.
// Logs differences. Returns true if results match.
func CompareResults(
    ctx context.Context,
    logger log.Logger,
    oldResult *ParseUserFilterResult,
    newResult *ParseResult,
    knownDivergences []string, // e.g., "sort_order", "union_vs_intersection"
) bool
```

Comparison logic:
- Compare FinalUsers sets (by resource name)
- Compare user-to-groups mappings
- Compare ShouldQueryAllUsers
- Log structured diff on mismatch
- Exclude known divergences from mismatch count

**Size**: ~150-200 lines (implementation + tests)

---

### Phase 5: Caller Migration

**Goal**: Migrate each caller from old path to new `Parser.Parse()`, gated by feature flag.

Each PR follows this template:

```go
// Before:
result, err := ParseUserFilterForAnalytics(ctx, ...)
// or: ApplyResourceACL + ListUsersMappedToGroups + MoveFiltersToUserFilter

// After:
if useUnifiedUserFilter(configClient, customerID) {
    result, err := parser.Parse(ctx, ParseOptions{...})
    groupMap := result.UserToDirectGroups  // or StripRootAndDefaultGroups(...)
    groups := result.AllGroups             // or result.FinalGroups
    if shouldReturn := ApplyToQuery(result, &req.Users, &req.Groups); shouldReturn {
        return &Response{}, nil
    }
    // optional: shadow mode comparison with old result
} else {
    // existing code unchanged
}
```

#### Migration order

Migrate by caller pattern (simplest first, most complex last):

**Wave 1: Time-range only callers (simplest — no group maps needed)**

| PR | Files | Notes |
|----|-------|-------|
| 5.1 | `retrieve_note_taking_stats.go` | Already on ParseUserFilterForAnalytics. **Shadow mode enabled** — representative for time-range pattern. |
| 5.2 | `retrieve_assistance_stats.go`, `retrieve_closed_conversations.go`, `retrieve_comment_stats.go`, `retrieve_conversation_messages.go`, `retrieve_partial_conversations.go` | Old pattern. Batch — all identical time-range pattern. |

**Size**: 5.1 ~80-100 lines; 5.2 ~200-300 lines (5 files, identical changes)

**Wave 2: Agent leaderboard callers (most common pattern)**

| PR | Files | Notes |
|----|-------|-------|
| 5.3 | `retrieve_agent_stats.go` | Already on ParseUserFilterForAnalytics. **Flagship + shadow mode** — validate agent leaderboard pattern first. |
| 5.4 | `retrieve_conversation_stats.go`, `retrieve_qa_score_stats.go`, `retrieve_smart_compose_stats.go`, `retrieve_suggestion_stats.go`, `retrieve_summarization_stats.go` | Batch — already on ParseUserFilterForAnalytics, identical agent leaderboard pattern. |
| 5.5 | `retrieve_guided_workflow_stats.go`, `retrieve_hint_stats.go`, `retrieve_knowledge_assist_stats.go`, `retrieve_live_assist_stats.go` | Batch — already on ParseUserFilterForAnalytics, identical agent leaderboard pattern. |
| 5.6 | `retrieve_conversation_message_stats.go`, `retrieve_conversation_outcome_stats.go`, `retrieve_coaching_session_stats.go` | Batch — old pattern, agent leaderboard. |
| 5.7 | `retrieve_adherences.go`, `retrieve_manual_qa_progress.go`, `retrieve_manual_qa_stats.go` | Batch — old pattern, agent leaderboard. |
| 5.8 | `retrieve_scorecard_criteria_stats.go`, `retrieve_scorecard_stats.go` | Batch — old pattern, agent leaderboard. |

**Size**: 5.3 ~100-150 lines; batches ~200-400 lines each

**Wave 3: Team leaderboard callers**

| PR | Files | Notes |
|----|-------|-------|
| 5.9 | `retrieve_knowledge_base_stats.go` | Already on ParseUserFilterForAnalytics. **Shadow mode enabled** — representative for team leaderboard pattern. |
| 5.10 | `retrieve_manager_stats.go` | Old pattern. |

**Size per PR**: ~100-150 lines each

**Wave 4: Special callers**

| PR | Files | Notes |
|----|-------|-------|
| 5.11 | `retrieve_annotated_time_series.go` | Old pattern, unique aggregation logic. Needs individual review. |

**Size**: ~100-200 lines

**Wave 5: Coaching callers**

| PR | Files | Notes |
|----|-------|-------|
| 5.12 | Add `InternalUserServiceClient` to coaching `ServiceImpl` + module injection | Infrastructure prep — no behavior change. |
| 5.13 | `action_list_coaching_plans.go`, `action_retrieve_coaching_overviews.go`, `action_retrieve_coaching_progresses.go` | Batch — all 3 only use `UserNames` → `Keys(result.FinalUsers)`. **Shadow mode on one**. |

**Size**: 5.12 ~50-80 lines; 5.13 ~150-200 lines

**Coaching migration details**: All 3 callers only use `parsedFilter.UserNames` from the result. The new path uses `Keys(result.FinalUsers)` which returns the same resource name strings. Coaching callers already have access to `profileID` (extracted from `profileName.ProfileID`) but don't pass it to the current `Parse()` — the new `ParseOptions` includes `ProfileID`. Adding `InternalUserServiceClient` to `ServiceImpl` is a prerequisite (PR 5.12) since the current coaching module only has `UserServiceClient`.

---

### Phase 6: Cleanup

After all callers are migrated and the feature flag has been stable for 2+ sprints:

#### PR 6.1: Remove `ParseUserFilterForAnalytics` and helpers

**Files**: `insights-server/internal/analyticsimpl/common_user_filter.go`

Delete:
- `ParseUserFilterForAnalytics()`
- `ParseUserFilterResult` struct
- `ApplyUserFilterFromResult()`
- `applyResourceACL()`, `listAllUsers()`, `expandGroupsToUsers()`
- `buildUserGroupMappings()`, `filterUsersByGroups()`
- `updateGroundTruthUsers()`, `unionUsers()`, `convertLiteUsersToUsers()`

**Size**: Large deletion (~500+ lines removed)

#### PR 6.2: Remove old inline pattern helpers

**Files**: `insights-server/internal/shared/common.go`

Delete:
- `ListUsersMappedToGroups()`
- `MoveFiltersToUserFilter()`
- Related helpers

**Size**: Large deletion

#### PR 6.3: Remove old `Parse` and `UserFilterParser` interface

**Files**: `shared/user-filter/user_filter.go`

Replace old interface and implementation with the new `Parser` struct. Update `module.go` to provide `*Parser` instead of `UserFilterParser`.

**Size**: ~200 lines changed

#### PR 6.4: Remove feature flag and shadow mode

**Files**:
- `cresta-proto/.../insights.proto` — deprecate field 21 (or leave as no-op)
- `shared/user-filter/compare.go` — delete
- Each caller file — remove flag check, keep only new path

**Size**: ~300-400 lines across many files (bulk simplification)

#### PR 6.5: Remove `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` env var

**Files**: `insights-server/internal/analyticsimpl/analyticsimpl.go`

Remove the old env var and all references.

**Size**: ~30-50 lines

---

## Rollout Strategy

```
Phase 1 (tests)
    ↓
Phase 2 (types) → Phase 3 (implementation) → Phase 4 (flag + shadow)
    ↓
Phase 5 Wave 1: time-range callers (2 PRs)
    ↓ enable flag for 1 test customer, validate via shadow mode on retrieve_note_taking_stats
Phase 5 Wave 2: agent leaderboard (6 PRs)
    ↓ enable flag for 5 more customers, validate via shadow mode on retrieve_agent_stats
Phase 5 Wave 3: team leaderboard (2 PRs)
    ↓ enable flag for all customers, validate via shadow mode on retrieve_knowledge_base_stats
Phase 5 Wave 4: special callers (1 PR)
Phase 5 Wave 5: coaching callers (2 PRs)
    ↓ validate via shadow mode on one coaching caller
    ↓ stable for 2+ sprints
Phase 6: cleanup (5 PRs)
```

**Shadow mode callers** (1 per pattern — runs both old and new, compares results):
- `retrieve_note_taking_stats.go` — time-range pattern
- `retrieve_agent_stats.go` — agent leaderboard pattern
- `retrieve_knowledge_base_stats.go` — team leaderboard pattern
- 1 coaching caller — coaching pattern

**Flag rollout within each wave**:
1. Merge caller PR (flag off by default)
2. Enable flag for 1 low-traffic customer
3. Monitor metrics + shadow mode logs for 1-2 days
4. Enable flag for all customers
5. Monitor for 1 week
6. Move to next wave

---

## Key Files Reference

| File | Role |
|------|------|
| `shared/user-filter/user_filter.go` | Current shared implementation (coaching) |
| `shared/user-filter/types.go` | NEW — types and interface |
| `shared/user-filter/parse.go` | NEW — unified implementation |
| `shared/user-filter/utilities.go` | NEW — post-processing utilities |
| `shared/user-filter/compare.go` | NEW — shadow mode comparison (temporary) |
| `shared/user-filter/module.go` | DI setup (update) |
| `shared/user-filter/BUILD.bazel` | Build deps (update) |
| `insights-server/internal/analyticsimpl/common_user_filter.go` | Current analytics implementation (source for migration) |
| `insights-server/internal/analyticsimpl/common_user_filter_test.go` | Current tests (extend in Phase 1) |
| `insights-server/internal/shared/common.go` | Old pattern helpers (delete in Phase 6) |
| `cresta-proto/.../insights.proto` | Feature flag proto definition |

---

## PR Count Summary

| Phase | PRs | Type |
|-------|-----|------|
| Phase 1: Behavioral tests | 4 | Test only |
| Phase 2: Types + utilities | 2 | Types + pure functions |
| Phase 3: Implementation | 4 | Behind flag |
| Phase 4: Flag + shadow | 2 | Infrastructure |
| Phase 5: Caller migration | 13 | Per-caller or batched |
| Phase 6: Cleanup | 5 | Deletion |
| **Total** | **~30** | |

---

## Resolved Decisions

1. **Shadow mode scope**: Enable for 1 representative caller per pattern — `retrieve_note_taking_stats.go` (time-range), `retrieve_agent_stats.go` (agent leaderboard), `retrieve_knowledge_base_stats.go` (team leaderboard), 1 coaching caller.
2. **Coaching callers**: Migrate in the same consolidation. Coaching callers only use `UserNames` (→ `Keys(result.FinalUsers)`). Requires adding `InternalUserServiceClient` to coaching `ServiceImpl` as a prerequisite PR. Coaching callers already have `profileID` but don't pass it to current `Parse()` — the new `ParseOptions` includes it.
3. **Batching**: Identical-pattern callers are batched into single PRs. Wave 2 reduced from 18 PRs to 6. Wave 1 from 6 to 2. Wave 5 from 3 to 2. Total Phase 5 reduced from 30 to 13 PRs.
