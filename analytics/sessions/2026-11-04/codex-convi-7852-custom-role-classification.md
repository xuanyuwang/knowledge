# CONVI-7852 — custom roles and team Agent/Manager classification

Source repo: `/Users/xuanyu.wang/repos/go-servers`.
Branch/worktree: `xw/convi-7852-auth-membership-roles`, `/Users/xuanyu.wang/repos/go-servers-convi-7852-auth-roles`.
Read-only investigation requested by user; no source edits or tests run.

## Documentation

Glean search and full document read: Sam Segers, Custom roles and permissions as an extension (updated October 10, 2025), https://docs.google.com/document/d/1h4UiAd7depNbBjVRkae2QtSXQd10qcAe5o_C6aKYGp0. Double Writing explicitly covers users/memberships: custom role inheriting Agent writes back proto Agent, default Manager writes back Manager, generic Base Role writes no proto role. All custom roles inherit one Cresta-defined role. Permissions can differ from those of the mapped proto role. This is a design doc; corroborate implemented behavior from source.

Full published help page read: https://docs.cresta.com/documentation/admin-and-config/org-management/custom-roles-in-org-management (updated March 14, 2026). Creation requires choosing a base role and supports overriding permissions. Names or permissions alone do not identify Agent/Manager classification.

## Source evidence

- shared/auth/customroles/utils.go:8 exported CustomRoleToProtoRole maps role ID/inheritedFrom role ID to the Cresta-defined proto role; unknown/unmapped returns ROLE_UNSPECIFIED. Expects the parent role ID; it does not fetch definitions.
- auth/internal/service/user/util.go:1230 syncProtoRolesAndCustomRolesForDoubleWrite prioritizes custom roles if supplied and calls syncProtoRolesFromCustomRoles. At 1257 the latter fetches customer role definitions, maps SourceRoleID through CrestaRolesByID, excludes generic Base Role, and replaces membership.Roles. No feature-permission inference.
- auth/internal/service/user/action_add_group_memberships.go:187 invokes synchronization before validation. At 80/81 it persists resolved membership.Roles to role_id and custom role resource IDs to role_ids in the same membership write.
- auth/shared/constant/crestarole/role_definitions.go:33 includes Agent, Manager, Manager2nd and other exact enum mappings. Manager+ is a chosen set of roles, not numerical enum ordering or arbitrary permission equivalence.
- auth/internal/service/user/util_test.go:33 existing test covers customer Manager inheritance and generic Base Role producing no proto role; inspected only, not run.
- auth/shared/userimport/roles_lookup.go NewRolesLookup uses the public mapper and Role.InheritedFrom for custom-role classification.

## Implication

Yes, custom-role assignments in a specific team can establish Agent/Manager/Manager2nd classification through their base role. Existing analytics role_id projection already includes the proto-role representation when normal auth write synchronization has run. Returning raw custom role IDs or resolving custom roles in analytics is unnecessary for this classification under that condition. Earlier broad wording that custom roles were deferred applies to raw custom-role metadata/permission resolution, not to excluding all custom-role users. Generic Base Role assignments have no inferred Agent/Manager identity even if their permissions resemble one. Global user roles do not prove a role in a particular team. Do not claim every customer row is synchronized: no customer DB data inspected. Indirect team-role propagation remains outside this check.

Used Glean and local source only; no AWS, Okta, Azure or SSH credentials selected, no production access or external messages.
