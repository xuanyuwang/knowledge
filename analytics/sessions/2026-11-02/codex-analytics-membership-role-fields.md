# Adding per-team roles to ListUsersForAnalytics

Date: 2026-11-02 (America/Toronto).
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`, main, HEAD `35c364ff69`. Related contract: sibling cresta-proto checkout. Read-only source investigation; no product edits.

## Contract

`/Users/xuanyu.wang/repos/cresta-proto/cresta/nonpublic/user/user.proto:22` contains UserGroupMembership with only group=1 and is_direct_member=2. A proto update is necessary. Proposed additive fields are repeated cresta.v1.auth.AuthProto.Role roles=3 and repeated string custom_roles=4, plus import cresta/v1/auth/auth.proto. Keep these on each membership rather than on LiteUser or LiteGroup. No change to request fields or the outer response wrapper is required. Document direct assignment semantics explicitly; indirect rows do not represent resolved inherited roles. Reuse the public GroupMembership field types and custom role resource-name format.

The proto package's existing BUILD.bazel already depends on the auth proto package, although generated files and Gazelle still need regeneration/checking during implementation. Release/generated Go package must be consumed by go-servers; its current go.mod references v2.32.92-0.20261027104727-8785b40b77fc. No generated file edits were made. Relevant proto and handler files match the previously fetched origin/main; no new remote fetch was performed today.

## Auth data and conversion

`auth/sql-schema/auth/schema.sql:501-525` defines user_group_memberships_all. Direct rows expose role_id (old-system numeric strings) and role_ids (new custom-role-system IDs). Indirect rows explicitly return empty arrays. The generated ORM already provides RoleID and RoleIds as pq.StringArray, so no SQL migration or ORM regeneration is required for this proposal.

`auth/internal/service/nonpublic/user/action_list_users_for_analytics.go:59-65` omits both arrays from its smart-select projection. Add RoleID and RoleIds using matching gorm column tags and pq.StringArray types; extend the compile-time dbmodel.UserGroupMembershipsAll check at lines 75-80. The existing batched membership query at lines 210-225 then fetches the arrays for the same selected users and memberships. No additional user/group query or join is needed.

Populate each response membership at lines 248-256. Convert RoleID decimal strings to AuthProto.Role, matching the existing public conversion at auth/shared/model/user.go:583-592: invalid int32 strings are skipped; numeric enum values are preserved. The existing converter functions are unexported in other packages, so they cannot simply be called from this package; choose a local conversion helper or deliberately share the existing conversion during implementation. No cross-service API calls are necessary. Convert RoleIds to typed role resource names with shared/auth.RoleIDsToStringRoleNames at shared/auth/role.go:20. Public User membership conversion at auth/shared/model/user.go:248-263 is the comparison implementation.

Populate roles from each membership, never auth.users.user_roles. Preserve direct/indirect rows separately, all existing tenant/profile/managed-group restrictions, user eligibility, pagination and ordering. Direct-only role metadata meets the agreed Agent display requirement; effective inherited-role reporting requires a different implementation.

## Cache and validation

Auth caches JSON of the generated LiteUser structures (nonpublic/user/cache.go). New fields serialize automatically once populated, but old entries decode with empty roles. Version the logical cache key before hashing, or deliberately accept/document the existing five-minute stale fallback. Mixed old/new producer responses must not be interpreted as authoritative absence of roles. No new request flag is necessary for this proposal.

Proposed checks: additive proto build/lint/breaking compatibility and code generation; real-DB membership role separation for Agent versus Manager Secondary; custom-role resource names; indirect empty arrays and coexisting direct/indirect rows; malformed/unknown numeric conversion; unchanged identities, filters, tenant/profile isolation and pagination; cold/warm JSON cache role preservation and old-entry behavior. These tests were not executed because this turn changed no product code.

This investigation addresses the internal RPC only. Passing roles through insights to Director remains separate from populating ListUsersForAnalytics. The [broader recommendation](../../deliverables/list-users-for-analytics-roles-recommendation.md) records those later response/cache changes.

No credentials, external messages, production queries, commits, pushes or PRs. Existing Director worktree untouched. Used API-design and data-access skills; durable closure follows the daily-capture workflow already read in this chat.
