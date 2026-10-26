# Assistance Page: Broken Tabs Investigation

**Created:** 2026-04-14

## Problem

On the Assistance page, `filterToAgentsOnly: true` works for most tabs but fails for:
1. **Silence/Hold hints** tab (uses `RetrieveAdherences` API)
2. **Summary** tab (uses `RetrieveAssistanceStats` API -- the legacy one)

Other tabs (Hints, Conversations, Suggestions, Smart Compose, Knowledge Base, Guided Workflows, Note Taking, Summarization, Knowledge Assist, Live Assist) all correctly filter out non-agent users.

## Root Cause

Both broken handlers use the **old 3-step pattern** (`ApplyResourceACL` + `ListUsersMappedToGroups` + `MoveFiltersToUserFilter`) instead of the **new unified** `ParseUserFilterForAnalytics` + `ApplyUserFilterFromResult` pattern. The old pattern hardcodes `listAgentOnly: false` and cannot read the request's `filter_to_agents_only` field.

## Detailed Findings

### 1. `RetrieveAdherences` (Silence/Hold hints tab)

**File:** `go-servers/insights-server/internal/analyticsimpl/retrieve_adherences.go`

**Two problems:**

#### Problem A: Proto missing `filter_to_agents_only` field entirely

**File:** `cresta-proto/cresta/v1/analytics/analytics_service.proto`, lines 1648-1692

`RetrieveAdherencesRequest` has fields 1-7 only. There is **no `filter_to_agents_only` field at all**. The FE cannot even send this field. Even if the handler code were updated, there's no proto field to read from.

Compare with `RetrieveHintStatsRequest` (line 1846: `bool filter_to_agents_only = 11`) which has the field.

#### Problem B: Handler uses old pattern with hardcoded `false`

The handler at line 34 calls `shared.ApplyResourceACL()` directly (the old exported function, not the new `applyResourceACL` internal to `common_user_filter.go`). Then at line 51, it calls `shared.ListUsersMappedToGroups(...)` with hardcoded `false` for `listAgentOnly`:

```go
// Line 34: Old ACL pattern
req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(...)

// Line 51: Hardcoded false
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(
    ...,
    false, // listAgentOnly  <-- HARDCODED
)
```

And in `retrieveAdherencesInternal` at line 83, it calls `shared.MoveFiltersToUserFilter()` with hardcoded `false`:

```go
filterByAttribute, err := shared.MoveFiltersToUserFilter(
    ...,
    false, // listAgentOnly  <-- HARDCODED
)
```

It does NOT call `ParseUserFilterForAnalytics` or `ApplyUserFilterFromResult` at all.

### 2. `RetrieveAssistanceStats` (Summary tab -- legacy API)

**File:** `go-servers/insights-server/internal/analyticsimpl/retrieve_assistance_stats.go`

**Proto has the field** (line 1938: `bool filter_to_agents_only = 9`) -- so the FE can send it.

**But the handler ignores it.** At line 80, it calls `shared.ApplyResourceACL()` (old pattern), then at line 94 calls `shared.ListUsersMappedToGroups(...)` with hardcoded `false`:

```go
// Line 80: Old ACL pattern
req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(...)

// Line 94: Hardcoded false
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(
    ...,
    false, // listAgentOnly  <-- HARDCODED
)
```

And in `retrieveAssistanceStatsInternal` at line 154, `shared.MoveFiltersToUserFilter()` is also called with hardcoded `false`.

This was intentionally left unchanged in Phase 1.2 (BE PR #26301) because it was documented as "Legacy API, uses old 3-step pattern" -- see [phase-1.2-be-handler-wiring.md](phase-1.2-be-handler-wiring.md).

## Working Handler Reference (for comparison)

**File:** `go-servers/insights-server/internal/analyticsimpl/retrieve_hint_stats.go`

The working pattern (lines 50-94):
1. Line 51: `listAgentOnly := req.GetFilterToAgentsOnly()` -- reads from request
2. Lines 57-75: Calls `ParseUserFilterForAnalytics(...)` with `listAgentOnly` parameter
3. Line 92: Calls `ApplyUserFilterFromResult(result, ..., a.enableExtTableForUserFilter)` -- correctly applies ext table logic

## Fix Required

### For `RetrieveAdherences`:
1. **Proto change:** Add `bool filter_to_agents_only = 8` to `RetrieveAdherencesRequest` in `cresta-proto`
2. **Handler migration:** Replace the old 3-step pattern with `ParseUserFilterForAnalytics` + `ApplyUserFilterFromResult`, matching the `RetrieveHintStats` pattern

### For `RetrieveAssistanceStats`:
1. **No proto change needed** -- field already exists at position 9
2. **Handler migration:** Replace the old 3-step pattern with `ParseUserFilterForAnalytics` + `ApplyUserFilterFromResult`
3. **Note:** This is a legacy API with `DeprecateRetrieveAssistanceStats` feature flag. Consider whether migration is worth it or if FE should stop calling it.

## Handler Pattern Comparison

| Aspect | Working (RetrieveHintStats) | Broken (RetrieveAdherences) | Broken (RetrieveAssistanceStats) |
|--------|---------------------------|---------------------------|--------------------------------|
| Proto field | `filter_to_agents_only = 11` | **MISSING** | `filter_to_agents_only = 9` |
| Reads from req | `req.GetFilterToAgentsOnly()` | N/A (no field) | **No** (hardcoded `false`) |
| User filter function | `ParseUserFilterForAnalytics` | `ListUsersMappedToGroups` | `ListUsersMappedToGroups` |
| Applies result | `ApplyUserFilterFromResult` | `MoveFiltersToUserFilter` | `MoveFiltersToUserFilter` |
| Uses ext table | Yes (`a.enableExtTableForUserFilter`) | No | No |
| ACL function | Internal `applyResourceACL` (via ParseUserFilterForAnalytics) | `shared.ApplyResourceACL` (old) | `shared.ApplyResourceACL` (old) |
