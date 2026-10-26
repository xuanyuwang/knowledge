# PR #25733 – Review comment verification

**Created:** 2026-02-17  
**PR:** https://github.com/cresta/go-servers/pull/25733

## Status

**All review comments addressed.** See details below.

---

## What this PR does (from knowledge base)

- **File:** `insights-server/internal/analyticsimpl/common_user_filter.go`
- **Change:** In `buildUserGroupMappings` (~375–398), replace `userfilter.FetchGroups` with `shared.ListGroups` so child teams are expanded (same behavior as the old path).
- **Other:** Remove unused `userfilter` import; tests use `setupListGroupsMock` with `ListGroups` call pattern (PageSize=200, no ProfileId).

---

## Cursor Bugbot comment – verification

**Comment:** "Dynamic group filters drop team aggregates" (inline on `common_user_filter.go` ~L377). Bugbot suggested filtering `groupNames` to only include `userpb.Group_TEAM` because `shared.ListGroups` can return both TEAM and DYNAMIC_GROUP; including dynamic groups could make the `slices.Contains` gate empty `GroupsToAggregate` when filtering by dynamic groups.

**Verification (2026-02-17):**

| Check | Result |
|-------|--------|
| Code change (TEAM filter applied)? | **Yes** — commit `59999495a5` (cherry-picked from bugbot autofix `c92bb28308`) filters `groupNames` to only `userpb.Group_TEAM` type. |
| Customer profile comment restored? | **Yes** — commit `1efbf2ee26` restores the holidayinn-transfers-voice cross-profile comment that was dropped during the `FetchGroups` → `shared.ListGroups` refactor. |

**Conclusion:** The Bugbot concern is valid and has been fixed. `shared.ListGroups` requests both `Group_TEAM` and `Group_DYNAMIC_GROUP` types (line 712 in `shared/common.go`). Without the TEAM filter on `groupNames`, selecting a dynamic group would make `slices.Contains` fail for all team memberships, leaving `GroupsToAggregate` empty. The fix ensures only TEAM group names populate `groupNames`, so the `len(groupNames) == 0` fallback correctly triggers when only dynamic groups are filtered.

---

## Commit summary

| Commit | Description |
|--------|-------------|
| `1063cba337` | Initial fix: replace `FetchGroups` with `shared.ListGroups` for child team expansion |
| `959afdb124` | Update B_GH_4 test to assert child expansion (parent + 3 sub-teams in `GroupsToAggregate`) |
| `59999495a5` | Fix bugbot issue: filter `groupNames` to TEAM type only (cherry-picked from bugbot autofix) |
| `1efbf2ee26` | Restore customer profile filtering comment from original code |

## Review comment checklist

| Comment (summary) | Source | Addressed? | How |
|-------------------|--------|------------|-----|
| Dynamic group filters drop team aggregates | cursor[bot] (Bugbot) | Yes | Commit `59999495a5` — filter `groupNames` by `Group_TEAM` |
| No actionable comments | coderabbitai[bot] | N/A | Approved |
| Customer profile filtering comment dropped | Self-review | Yes | Commit `1efbf2ee26` — restored holidayinn cross-profile comment |
