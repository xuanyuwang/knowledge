# CONVI-7715 Oportun group-filter investigation — 2026-09-24

**Source repo:** `/Users/xuanyu.wang/repos/go-servers` at `main` (`6e9999f0ad`); read-only. Related Director checkout `/Users/xuanyu.wang/repos/director` at `convi-7746-training-simulator-fe-archive-lesson-warning-dialog-ungate` (`ffe7933f7a`); read-only. Knowledge written in its main checkout.

Fetched both `origin/main` refs on 2026-09-24 (`go-servers` `72eba766a8`, Director `065c7b3cf4`). The source files cited below have no diff between the inspected checkouts and the respective refreshed `origin/main` refs.

## Inputs

- Linear CONVI-7715 (Todo, assigned Xuanyu Wang): Liliana sees 0/N/A with virtual group `Coll test group AB test 1to59` for Collections AA model, This month/Daily; she sees data by teams and Support sees data with the same group. Ticket includes Zendesk 23788 but no request capture or conversation IDs. No issue comments.
- Linear CONVI-7347: older Ryan Helwig Collections no-data report; no group-filter reproduction or causal fix recorded in the issue. No issue comments.
- `config` latest `origin/master` on 2026-09-24: `configv3/prod/oportun/config.yaml` has `customerId=oportun`, profile `us-west-2`, `authConfig.enableAdvancedDataAccessControl=true`; auth DB on AWS `chat-prod/auth-prod/oportun`.

## Current source trace

1. Director `packages/director-app/src/components/insights/hooks/util-hooks/getQAFilterByAttribute.ts` maps `usersTeamsGroups.groupNames` into `QAAttribute.groups`, with teams also mapped into that field.
2. `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go:89-117` passes request users/groups into `ParseUserFilterForAnalytics`; `:139-143` returns an empty result when its final user set is empty.
3. `common_user_filter.go:227-230` prefilters ground-truth users to selected group memberships for a group-only request. `:242-255` then calls `applyResourceACL`; `:269-279` returns empty when ACL users are absent; `:280-301` intersects nonempty ACL users with the prefiltered ground truth.
4. `insights-server/internal/shared/common.go:930-945` adds `IncludeManagedGroups` and `WithFilteredGroupIDs` for a group-only request, with **no** `IncludeManagedUsers`.
5. `shared/auth/acl_helper.go:228-240, 275-290` derives manageable groups from TEAM memberships/child TEAM groups. `:466-489` filters that list to the requested group ID. A dynamic group is absent. `GetAllUserIDs` still includes the caller. `common_user_filter.go:757-795` expands only returned ACL groups, so no virtual members are added.
6. Since Liliana is not a virtual-group member, the remaining caller user is removed by the parser's ground-truth intersection; `ApplyUserFilterFromResult` early-returns. Root-access Support bypasses the scoped ACL; selected TEAM groups can pass the ACL and expand their users.

Existing tests cover a dynamic-group filter with ACL disabled (`common_user_filter_test.go` B_GS_2). They do not cover this dynamic-group plus limited TEAM-manager ACL combination.

## Read-only Oportun auth DB checks

Used `connect-customer-auth-db` skill with `--read-only` via `cresta-cli`; individually inspected local `chat-prod_ro` AWS SSO config before use. Credential used: cleared `chat-prod_ro` SSO profile. No credential value was printed or stored. SQL only read `auth.groups`, `auth.users`, `auth.user_group_memberships_all`, and `auth.user_profile_configs`.

- Selected group ID `019fca63-ffe8-778b-9d03-f760d1771e72`; `group_type=2` (`Group_DYNAMIC_GROUP` in proto), active, profile `us-west-2`.
- 53 distinct direct members; all 53 active globally and in profile `us-west-2` on 2026-09-24.
- Liliana ID `339b3bfd62e13211`. She has 18 direct TEAM memberships with `custom_manager` or `custom_manager_2nd`; across these teams, all 53 selected virtual-group members have a TEAM membership. Liliana is not in the selected virtual group.
- Sergio ID `113fc4322f761641`. He has one direct TEAM membership with `custom_manager` or `custom_manager_2nd`, and none of the 53 virtual-group members currently overlap it. He is not in the virtual group. His separate expected audience needs verification.

The current data strongly confirms the source-level failure mechanism for Liliana, but the precise 2026-09-11 request and deployed binary were not captured. This is a current-state diagnosis, not a historical onset claim.

## Proposed implementation boundary

For a selected dynamic group, expand to candidate member IDs, compute the caller's managed-user ACL independently of that group's ID, and intersect. Keep team selection's descendant semantics and explicit-user/group union behavior. Verify all analytics parser consumers before changing shared code; add one positive and one denied-member test.

No product code, Linear comments/status, or production rows changed.
