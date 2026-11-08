# CONVI-7852 existing Leaderboard membership reuse

Date: 2026-11-01 (America/Toronto).
Primary source repo: `/Users/xuanyu.wang/repos/director`.
Worktree: `/Users/xuanyu.wang/repos/director-convi-7852`, branch `xw/convi-7852-agent-team`.
Request: inspect whether existing Leaderboard fetches include per-team roles that can replace or reduce the new lookup. Source-only review; product code unchanged.

## Findings

- Leaderboard uses `useLeaderboardsFilters` → `UserTeamGroupFilter` → `useUserTeamGroup`.
- Main filter user lookup (`useUserTeamGroup.ts:318`) does not request includeUserMemberships; `useFilteredUsers.ts:76` passes false. User-level roles do not identify roles in each team.
- Filter TEAM lookup (`useUserTeamGroup.ts:353`) requests includeGroupMemberships=true. Public ListGroups loads direct user memberships and converts per-membership roles into Group.members: `go-servers/auth/internal/service/user/action_list_groups.go:312-359`, `util.go:225-245`. Director preserves members and their roles in user/transformers.ts:316-337,397-419. These can be inverted into user-to-team-role metadata for the loaded teams.
- However the filter lists active teams and the backend omits inactive users; requests are searched and paginated (only one page through useFilteredGroups, or up to 5000 for hierarchy). Its local fetchedData is not passed to AgentLeaderboardPage. A partial set of team memberships cannot safely decide the lowest-ID Agent team or absence of Agent teams.
- ALO table and by-metric paths use `useLeaderboardAloAudienceResolution` → `useAudienceResolution.ts:41`, which calls useUsers with includeUserMemberships=true. Full user metadata is available inside that hook, but the wrapper returns only name, username, and team string. Users requested come only from ALO outcome-stat audiences; ALO queries can be skipped, and the lookup does not request fetchAllPages.
- Current authenticated user context covers only the signed-in user, not all leaderboard rows.
- useUsers query key includes 'infinite', full filter and options (`useUsers.ts:42`), while useFilteredUsers uses a different query shape. Existing ALO query and new lookup have different IDs/options. There is no automatic per-user merging or deduplication across these list queries.

## Conclusion

Reusable role-bearing data already exists conditionally: ALO public users and the filter's team members. The new lookup need not always fetch metadata already loaded, but neither existing source guarantees complete information for all multi-team analytics users. A follow-up could share metadata at page level and request only unresolved users, with candidate-team completeness checks and pagination. This review does not authorize or implement that refactor. No live browser/network verification or tests were run because product code was not changed. No credentials used.
