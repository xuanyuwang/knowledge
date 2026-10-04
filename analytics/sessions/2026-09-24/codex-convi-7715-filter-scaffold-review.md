# CONVI-7715 dynamic-group normalization scaffold review — 2026-09-24

**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`; reviewed worktree `/Users/xuanyu.wang/repos/go-servers-convi-7281`, branch `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`, HEAD `99d196e76f`. Review only; left the uncommitted pseudocode and product source unchanged.

## Proposal assessed

After `listAllUsers`, split requested groups into TEAM and DYNAMIC_GROUP. Convert dynamic-group members found in `groundTruthUsers` into requested users, then pass only teams plus those users to the existing ACL. This is the right semantic boundary: virtual groups select user identities; teams retain hierarchy/managed-group ACL semantics. `listAllUsers` already carries group memberships under the requested profile, agent-only, active-user, and direct/indirect options.

## Blocking findings

1. The proposed `g.Type == Group_DYNAMIC_GROUP` branch will not run for the Director Performance Insights request. Director `getQAFilterByAttribute.ts:44-70` serializes `{name}` for both teams and groups. In current parser tests, `toGroups` also constructs name-only `userpb.Group` values. Resolve group metadata by requested name from the UserService (for example `shared.ListGroups` with `onlyFilteredGroups=true`), validate each requested group belongs to the profile, and split using returned types. Do not trust optional client type as the authority.
2. A selected dynamic group whose eligible membership resolves to zero produces `reqUsers=[]` and, after removing the dynamic group, `reqGroups=[]`. `shouldUseAllAgents` interprets that as an unfiltered selection for ACL-disabled/root callers, returning the entire ground truth; limited callers get their entire managed population. Preserve `hadSelection` and return empty when all selected audiences resolve empty. Stale/unknown group names must not silently become unfiltered.

## Implementation details and regression coverage

- `groundTruthUsers` contains `LiteUser`, while `reqUsers` expects `*userpb.User`; use existing `convertLiteUsersToUsers` and ID/name deduplication. The scaffold's `dedup(reqUsers)` call would need its return value assigned. These are expected pseudocode gaps, not design objections.
- Reuse `filterUsersByGroups(groundTruthUsers, dynamicGroups, includeDirectGroupMembershipsOnly)` to preserve direct/indirect membership semantics. Its current invalid-name behavior silently skips malformed group IDs; the metadata-resolution step should validate names before expansion.
- Preserve `(explicit users ∪ dynamic-group members ∪ selected TEAM members) ∩ accessible users`. With nonempty resolved users plus teams, existing `BuildResourceACLOptions` includes both managed users and managed groups and `applyResourceACL` unions their results. Existing tests cover mixed user+group union but not dynamic-group normalization under limited ACL.
- Add cases for name-only dynamic-group input; limited manager with authorized and unauthorized virtual members; empty/ineligible virtual group under root, ACL-disabled, and limited callers; dynamic plus team; dynamic plus explicit user; stale/unknown group; direct membership; profile/agent-only filtering.
- The parser has many analytics callers, including `RetrieveQAScoreStats`, so review the shared behavior before implementation.

No tests ran because the diff is intentionally non-compilable pseudocode. `git diff --check` on the worktree found no whitespace errors.
