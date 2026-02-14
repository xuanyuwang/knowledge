# CONVI-6260: Team Leaderboard Not Breaking Out Sub-Teams

**Created:** 2026-02-13
**Updated:** 2026-02-13

## Overview

Hilton reports that when filtering Performance & Assistance Insights by a parent team and selecting "Leaderboard by criteria: Team", only the parent team appears as one row instead of breaking out sub-teams.

**Root Cause:** `buildUserGroupMappings` in the new `ParseUserFilterForAnalytics` path uses `FetchGroups` which does not expand child groups. The old path's `ListGroups` did this expansion. When the feature flag `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` was enabled (all non-Schwab clusters), the 12 migrated APIs lost sub-team expansion in team leaderboards.

**Key File:** `insights-server/internal/analyticsimpl/common_user_filter.go:375-398` (`buildUserGroupMappings`)

**Fix:** Replaced `userfilter.FetchGroups` with `shared.ListGroups` which expands child teams via `IncludeGroupMemberships`.

**PR:** https://github.com/cresta/go-servers/pull/25733

## Log History

| Date | Summary |
|------|---------|
| 2026-02-13 | Investigation, root cause identified, fix implemented, PR #25733 created |
