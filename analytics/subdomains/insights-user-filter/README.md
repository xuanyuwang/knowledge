# Insights User Filter

## Purpose

Own how analytics requests resolve users, teams, groups, roles, hierarchy, access scope, and active state.

## Semantics and Invariants

- Filtering chooses the eligible population; grouping chooses how that population is broken down. They are independent operations.
- Empty selection, root selection, explicit users, teams/groups, ACL scope, deactivated users, agent-only mode, and virtual groups require distinct semantics.
- Hierarchy expansion must define whether descendants and direct members are included and at what point ACL restrictions apply.
- Responses can contain aggregate summary fields even when `groupBy` requests a breakdown.
- Large ID sets may be passed through ClickHouse external tables; scaling strategy must not change membership semantics.

## Architecture and Source Map

- **Frontend:** shared Insights user filter and page-specific filter adapters
- **APIs:** the Analytics API cluster documented in `insights-user-filter/analytics-apis-list.md`
- **Backend:** auth/ACL resolution, user hierarchy expansion, analytics request construction
- **Storage:** user/team/group metadata plus ClickHouse external ID tables

## Operational Knowledge

- Compare the selected UI model, expanded IDs, request filter, `groupBy`, ACL scope, and returned identities separately.
- Treat virtual-group behavior as security-sensitive; visibility must be proven, not inferred from UI availability.
- `ListUsersForAnalytics.AgentOnly` and public `ListUsers.Roles=[AGENT]` are different contracts: the former excludes users with any non-agent role, while the latter matches users having the requested role.
- Role-filtered group expansion can intentionally preserve explicitly selected users. CONVI-6930 uses this for agent audiences, while reviewer audiences intentionally omit the role filter to include non-agent reviewers.
- Current `ListUsersForAnalytics` supports user-ID narrowing, but its proto `user_types` field is not applied by the go-servers auth implementation after CONVI-7343 was reverted.

## Active Work

- [CONVI-6719](../../work-items/CONVI-6719.md) — revalidated Phase 3 before implementation. The unified parser is not implemented; its design must explicitly preserve both analytics `AgentOnly` semantics and coaching/role-based `ListUsers` semantics.

## Legacy Sources and Cases

- `insights-user-filter/`
- `user-filter-consolidation/`
- `convi-7049-clo-filter/`
- `convi-6260-team-leaderboard/`
- `convi-6247-agent-only-filter/`
- `large-user-id-clickhouse/`
- `bswift-wrong-team-mapping/`
- `virtual-group-filter/` (security review required; do not copy credentials)

## Open Questions

- Produce a canonical matrix for empty/root/explicit/virtual selections across every Analytics API.
- Define deactivated-user and historical-membership behavior.
- Decide how the unified parser selects or validates its analytics versus coaching population contract; do not silently combine `ListAgentOnly` with `Roles`/`GroupRoles`/`UserTypes`/`State`.
