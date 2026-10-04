# ParseUserFilterForAnalytics refactor analysis — 2026-09-29

**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Branch/worktree:** `xw/convi-7715-dynamic-group-filter` at `/Users/xuanyu.wang/repos/go-servers-convi-7715`  
**Source state:** commit `899da6f56e`; no refactor edits made.

## Current responsibilities

1. Step 0 loads the canonical eligible users with `listAllUsers` (agent-only, active state, profile, direct/indirect membership). It now also resolves selected group types, flattens dynamic-group members into requested users, handles selected-but-empty, and prefilters team-only group selections. These are eligibility and request-selection responsibilities mixed together.
2. Step 1 reads customer ACL config, builds ACL options, obtains managed users/groups, expands authorized TEAM groups for limited access, unions selected users plus groups for disabled/root access, and handles no-access early return. Access policy and audience materialization are mixed.
3. Step 2 intersects the ACL result with canonical eligible users and derives `shouldUseAllAgents`. For root/disabled group-only requests with no explicit users, the group restriction was already applied in Step 0; Step 2 does not perform that intersection itself.
4. Step 3 fetches group metadata again when needed, builds TEAM membership mappings/group aggregation, converts users, derives `UsersFromGroups`, sorts, and builds the result. Selection and authorization should be complete before this presentation/query-output phase.

## Proposed behavior-preserving boundaries

Keep `ParseUserFilterForAnalytics` as orchestration while extracting focused stages:

1. `loadEligibleUsers`: wrapper around existing `listAllUsers` with the ground-truth contract.
2. `prepareRequestedAudience`: resolve name-only group metadata; output explicit users plus dynamic members, selected TEAM groups, an explicit `wasSelected` flag, and candidate eligible users. Apply the existing TEAM-only group prefilter here, before ACL evaluation. Empty normalized selection is not an unfiltered request.
3. `authorizeAudience`: evaluate ACL state and return authorized user/group identities plus an explicit access mode (disabled, root, limited). Initially wrap existing `applyResourceACL` without changing its internals.
4. `selectEligibleAudience`: complete user/group union and ACL intersection against the candidate eligible users, then decide query-all from `!wasSelected` plus ACL mode. Preserve current group-only and mixed-selection behavior exactly before simplifying the asymmetric group expansion.
5. `buildAnalyticsFilterResult`: build TEAM mappings and aggregation metadata from the final eligible user map, then convert/sort/result fields. Leave its `shared.ListGroups` behavior intact in the first refactor.

The intended distinction is between the selected audience, the canonical eligible population, the authorized audience, and response/query metadata. A typed intermediate state would make `wasSelected` and ACL mode explicit and avoid relying on empty slices to mean both no filter and no audience. This is a refactor proposal, not an approved semantic change.

## Invariants to keep in focused tests

- No filter with ACL disabled/root can set `ShouldQueryAllUsers`; a selected audience that resolves empty cannot.
- Explicit users, dynamic members, and selected TEAM members use union semantics, then remain bounded by ACL and the canonical eligible population.
- Limited ACL may return managed users, TEAM groups, and the caller identity; group-only and mixed selections retain their current behavior.
- Direct-membership, agent-only, inactive-user, profile, root/default TEAM mapping, child-team expansion, output sorting, and `ApplyUserFilterFromResult` behavior remain stable.

## Scope note

This concerns the existing analytics parser. A wider shared user-filter parser design exists under CONVI-6719; its population/role semantics are not interchangeable with this mechanical refactor without a separate decision and tests.

## Follow-up: `GroupsToAggregate` contract

`GroupsToAggregate` is a set of real TEAM groups derived from the final eligible users' memberships, not the raw selected group filter and not an SQL audience restriction. `buildUserGroupMappings` fetches profile-scoped groups (including child teams when a team is selected), uses the TEAM names to constrain aggregation groups when appropriate, then scans final users' `LiteUser.Memberships`. It skips dynamic-group memberships; for non-agent grouping it also skips root/default and may skip indirect memberships. It builds unique `Group` objects from membership metadata and sorts them by resource name before returning.

In `RetrieveQAScoreStats`, group-by-GROUP queries first retrieve per-agent scores, then `convertRowsPerUserToPerGroupQAScoreStatsResponse` uses `GroupsToAggregate` as the allowed group keys and `UserNameToGroupNamesMap` to place each agent's score into matching TEAM buckets. The same list is reused for group metadata when group-by-AGENT returns agent rows with group memberships. A dynamic group selected as a filter determines the agent audience, but the resulting group rows remain actual TEAM groups. This dual use explains why the field's name does not perfectly describe its contract; a refactor should name it by output role and keep it separate from `FinalGroups` and requested groups.

## Follow-up: boundary with unified analytics/coaching parser

The proposed shared `ParseResult` in `shared/user-filter/types.go` already separates `FinalUsers`, `AllGroups` (TEAM groups represented in final users' memberships), `FinalGroups` (selected subset), and direct/all user-to-TEAM membership maps. That supports a shared parser returning the teams of final eligible users. The analytics-specific choice of TEAM buckets for group rows or team labels on agent rows should remain caller-side post-processing. Coaching callers can ignore the optional membership outputs.

The existing analytics `GroupsToAggregate` name is inaccurate because it also supplies TEAM metadata for agent rows. A behavior-preserving local rename could use `ResponseTeams`; a unified result should prefer `AllTeams` or `TeamsOfFinalUsers` over `AllGroups` if the set excludes DYNAMIC_GROUP. Keep the selected TEAM subset separately named. A selected dynamic group must select member users without constraining output to a dynamic-group bucket; the unified plan's `FinalGroups` rule needs to state whether "no groups selected" means "no TEAM groups selected" in that case. Also specify whether root/default and direct/indirect TEAM memberships are included in each output instead of inheriting `hasAgentAsGroupByKey` behavior.

On 2026-09-29, the user chose to leave the existing analytics code unchanged and document two derived values for the unified parser: `teamsForAgentInfo` for direct TEAM metadata on agent rows, and `teamsForAggregation` for TEAM buckets used to calculate grouped statistics. Updated `user-filter-consolidation/user-filter-behavioral-standard.md` and `implementation-plan.md`; no refactor code changes were made. The separate CONVI-7715 dynamic-group fix was published as [PR #33026](https://github.com/cresta/go-servers/pull/33026).
