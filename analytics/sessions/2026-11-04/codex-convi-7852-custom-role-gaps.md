# CONVI-7852 — custom-role propagation gaps

Primary source repo: `/Users/xuanyu.wang/repos/go-servers`.
Branch/worktree: `xw/convi-7852-auth-membership-roles`, `/Users/xuanyu.wang/repos/go-servers-convi-7852-auth-roles` (HEAD and live PR #35092 `5944941d5b`). Additional read-only context: analytics branch/worktree `/Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles`, PR #35094 `86259f78ab`; Director prototype `/Users/xuanyu.wang/repos/director-convi-7852`, HEAD `59ec62ac94` plus pre-existing uncommitted prototype. No source changes, branch updates, tests, customer DB access, or external messages.

Question: identify gaps to include custom roles in protobuf, ListUsersForAnalytics, and FE team-level Agent/Manager/Manager2nd classification. Earlier docs/source findings: [custom-role classification](codex-convi-7852-custom-role-classification.md). Knowledge operating model and workspace registry read; canonical analytics project already exists.

## 1. Protobuf

Read actual go-servers pinned dependency source at `/Users/xuanyu.wang/go/pkg/mod/github.com/cresta/cresta-proto/v2@v2.34.6-0.20261104130341-34bced4abda2`, avoiding stale standalone proto checkout.

`cresta/nonpublic/user/user.proto` UserGroupMembership has group=1, is_direct_member=2, roles=3 and no custom role field. Proposed additive `repeated string custom_roles = 4`, containing full role resource names (`customers/{customer}/roles/{role}`), with direct-assignment semantics and empty for indirect memberships. No global LiteUser roles needed for a per-team classification. Public `cresta/v1/user/group.proto` GroupMembership already has custom_roles=13. Existing Auth Role.inherited_from identifies the Cresta-defined base role. No additional public analytics response field, role-definition field, or public FE generated field needed. Internal proto generation/release and go-servers dependency update required.

## 2. Producer and analytics pass-through

`auth/internal/service/nonpublic/user/action_list_users_for_analytics.go:67` smart projection selects role_id only. Add RoleIds mapped to role_ids and corresponding compile-time dbmodel projection check. Existing auth.user_group_memberships_all view (schema.sql:505+) already exposes role_ids for direct rows, empty arrays for indirect rows; no DB migration/additional role-table query needed to copy assignments. Populate internal CustomRoles from m.RoleIds using exported `shared/auth.RoleIDsToStringRoleNames` (shared/auth/role.go:20), rather than duplicating the private public-model converter. Existing profile, user and group query predicates remain authoritative; returning all supplied memberships is not all groups across every profile.

Cache serializes LiteUser protobufs; extend regression to include custom role names in direct rows, no inherited role labels on indirect rows, and Redis round-trip/cache hits. Current cache TTL is five minutes. New field can fill after normal expiration, consistent with accepted metadata delay; optional logical key bump from v2 avoids roleless cached payloads immediately after rollout. Membership metadata remains separate from FinalUsers and stats-cache identity.

Necessary extra step: `insights-server/internal/analyticsimpl/common_user_filter.go:433` inline conversion currently only copies Roles. Copy internal CustomRoles to existing public GroupMembership.CustomRoles, cloning the slice. Map retains raw membership protobufs already, so no separate parser data structure needed. Existing decoration is only Agent+Group conversation results, after metrics cache storage; extending to other RPCs/groupings would be additional work, not implied by copying the field.

## 3. FE existing support and missing integration

- director-api User.GroupMembership already has customRoles?: string[] (types/models/user.ts:138+), AuthRole has inheritedFrom, and transformApiGroupMembershipToModel preserves customRoles (services/cresta-api/user/transformers.ts:316+).
- Analytics transformAttributes -> transformUsers -> transformApiUserToModel -> transformApiGroupMembershipToModel preserves nested memberships, so no analytics transformer feature required for this field.
- Public exported `getProtoRoleForCustomRole` (director-app/src/utils/userAuthUtils.ts:218) maps a role's inheritedFrom (or builtin role's own name) to its proto role, unknown/base -> ROLE_UNSPECIFIED.
- Private `getDistinctProtoRolesFromCustomRoleNames` in the same file performs collection resolution through roleDefinitionMap; consider exposing/reusing it for membership classification instead of duplicating it.
- Exported hasReportingManagerRole (same file:260) establishes existing precedence: customRoles present => resolve those; absent => fallback to legacy roles. It only checks MANAGER; explicitly excludes MANAGER_2ND from the reporting-manager rule. Do not repurpose this helper as a generic Manager+ classification.
- Existing prototype preferAgentTeams (leaderboard/agent-leaderboard/agentTeams.ts) still checks membership.roles includes AGENT and looks up separately fetched users. useAgentLeaderboardTeams still calls useUsers; inline stats-user metadata consumption/removal of redundant fetch remains unfinished.

### Definition coverage

CurrentUserContext.tsx:56+ requests only definitions assigned to the authenticated user by default; that map cannot classify arbitrary leaderboard users' custom roles. useRoleDefinitionMap supports a broader request via onlyAssignedToAuthenticatedUser=false and includePermissions=false; the server's ListRoles accepts common roles and GENERAL_BASIC_ACCESS and applies only the requested assigned-user filter. No role permission payload is necessary: name/inheritedFrom suffice.

Coverage caution verified in hook implementation: useRoleDefinitionMap requests ACTIVE definitions with pageSize 5000, sets getNextPageParam, but does not call fetchNextPage, and its query function uses options.pageToken rather than context.pageParam. The wrapper useCancellableInfiniteQuery only delegates to useInfiniteQuery and does not fetch all pages. Thus it does not guarantee all definitions above the first page; inactive assignments are also unresolved by its active-only filter. No evidence of >5000 roles or inactive assignments in a customer; these are implementation limits, not observed customer defects. Missing/loading/failed definitions should produce unknown classification and preserve current displayed-group fallback; do not infer manager status from absence of Agent classification.

### Classification and display

For a candidate group, find its user's membership with matching group resource name, TEAM type, and direct flag. Resolve every custom role through role definitions and the existing mapper. Check AGENT, MANAGER, and MANAGER_2ND independently; multiple roles may coexist. If customRoles absent/empty, use legacy roles for older producers/assignments. Generic custom_base cannot be classified from permissions or display name. Treat custom-role base mapping as distinct from indirect team-membership inheritance; neither resolves role propagation through team ancestry here.

Display rule remains direct Agent team among row groups, lowest team resource ID for ties, existing first-team fallback otherwise. Manager and Manager2nd are exact roles, not enum-number comparisons or interchangeable reporting identities.

## Suggested implementation sequence and validation

1. Add internal proto custom_roles=4 with explicit semantics; release generated dependency.
2. Update auth smart projection/conversion and cache tests. Seed differing role_id and role_ids, direct/indirect duplicate group rows, and custom-only assignments; verify copied names rather than assuming double-write completeness.
3. Update analytics inline conversion with cloned CustomRoles; verify warm/fresh responses, all-membership preservation, stats-filter/cache-key independence and unchanged Team totals.
4. Integrate FE inline memberships and existing role resolver with complete role-definition coverage. Cover custom Agent/Manager/Manager2nd, builtin custom roles, generic Base, conflicting custom/proto arrays, loading/missing definitions, multiple roles, indirect memberships, off-row candidates, stable ordering and fallback. Replace redundant user fetch.

This investigation does not establish that custom IDs must be added to deliver the existing Agent-label fix: mapped proto roles already support normal double-written custom assignments. Raw custom-role propagation is useful if FE should resolve assignments explicitly and avoid relying solely on stored proto mapping. No implementation authorization inferred from the investigation request.

Authentication: existing gh self-authentication only; no AWS, Okta, Azure or SSH credentials selected.
