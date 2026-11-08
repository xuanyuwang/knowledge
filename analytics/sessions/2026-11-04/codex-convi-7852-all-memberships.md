# CONVI-7852 — return all supplied user memberships

Source repo: `/Users/xuanyu.wang/repos/go-servers`.
Branch/worktree: `xw/convi-7852-analytics-membership-roles`, `/Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles`.
Starting analytics PR #35094 head: `1d0fc06feb61453362eedf4756e73619fec7b3c8`; clean worktree, published head matched.

User approved replacing row-specific direct TEAM selection with conversion of all memberships supplied for each eligible user. Frontend owns matching row groups and selecting the displayed team. Search found no reusable public internal-user-membership to public GroupMembership converter: auth's existing helpers convert database models and are internal. Convert inline in withConversationMemberships and remove conversationUserMemberships. Preserve input order, group type/display name, directness and roles; do not invent indirect/custom-role resolution. Keep existing response decoration timing and grouping condition, query/cache identity, population and metrics unchanged.

Update existing regression to assert all supplied memberships including indirect/direct entries for the same group, an off-row team, dynamic groups, display names from membership metadata, and cloned response/role slices. Run it before implementation to confirm the existing filtering fails the new expectation; then validate parser and focused RPC/cache/Team tests. Full conversation suite remains limited by known local ClickHouse memory, so do not retry unrelated full-suite failures.

## Outcome

Removed conversationUserMemberships and inlined fn.Map conversion inside withConversationMemberships. LiteGroup/public Group type values are documented consistent in the proto; use the enum cast. Public IsIndirect is the inverse of internal IsDirectMember. Display names come from membership metadata. All input memberships and ordering are preserved, including direct/indirect duplicates, with role slices cloned. No producer or frontend source changed.

TestAllUserGroupMemberships failed before the production edit with the expected missing-membership/order/display-name mismatch. After implementation all five focused methods passed in 8.5s (verbose log confirmed exact method names): TestAllUserGroupMemberships, TestMissingMetadataClearsStaleRoles, TestRPCMembershipRoles, TestCachedRPCMembershipRoles, TestRPCTeamAggregationKeepsBothTeams. An initial combined test filter did not select parser tests; corrected with a separate unfiltered full common_user_filter_test run, which passed in 0.2s and logged real parser execution. Gazelle produced no BUILD changes; gofmt and git diff --check passed. Full conversation suite not rerun due to the previously documented local ClickHouse memory limitation.

Commit 86259f78ab pushed as a fast-forward to existing PR #35094 with an explicit lease against the starting head. PR description updated to the all-membership contract. Git/gh used existing authentication; no AWS, Okta, Azure or SSH credentials explicitly selected. No customer/production access. Post-push CI not monitored. Frontend still needs inline-metadata integration and team selection; no customer fix claim.
