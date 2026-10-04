# CONVI-7715 worktree and filter-semantics follow-up — 2026-09-24

**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`; new isolated worktree `/Users/xuanyu.wang/repos/go-servers-convi-7715`, branch `xw/convi-7715-dynamic-group-filter`, created from refreshed `origin/main` `72eba766a8`. No product code edited in this follow-up.

User directed all further CONVI-7715 source work to the new worktree. The earlier pseudocode was reviewed in a different worktree before this direction; it was not copied into the new checkout.

Clarified the review feedback:

- Director can know which UI item is a virtual group while still sending only a group resource name. The backend must resolve the authoritative group type before applying the proposed split.
- A selected virtual group can produce zero eligible `reqUsers` if it has no members, or if its members are excluded from `listAllUsers` by the profile, agent-only, active-user, or direct-membership options. A missing/stale group is a separate resolution failure to handle explicitly. If explicit users were also selected, they remain; the empty-result guard applies only when all selected audiences yield no users or teams.
- With a virtual-only selection resolving to zero users, removing it from `reqGroups` would leave both normalized request lists empty. Existing parser semantics interpret that as no filter. Preserve selection intent and return an empty audience in that case.

## Revised parser pseudocode fitted to existing steps (design only)

```go
hadAudienceFilter := len(reqUsers) > 0 || len(reqGroups) > 0 // before Step 0

// Existing Step 0.
groundTruthUsers := listAllUsers(...)

// At the end of Step 0, normalize only when groups were requested. Resolve
// exact group metadata by name/profile from UserService; Director sends names.
if len(reqGroups) > 0 {
    resolvedGroups := resolveExactRequestedGroups(reqGroups, customerID, profileID)
    teamGroups, dynamicGroups := splitByResolvedType(resolvedGroups)
    if len(dynamicGroups) > 0 {
        dynamicMembers := filterUsersByGroups(groundTruthUsers, dynamicGroups, includeDirectGroupMembershipsOnly)
        reqUsers = unionUsers(reqUsers, convertLiteUsersToUsers(dynamicMembers, customerID))
    }
    reqGroups = teamGroups
}

if hadAudienceFilter && len(reqUsers) == 0 && len(reqGroups) == 0 {
    return emptyParseUserFilterResult() // selected audience has no eligible users
}

// Existing group-only prefilter now sees TEAM groups only.
if len(reqGroups) > 0 && len(reqUsers) == 0 {
    groundTruthUsers = filterUsersByGroups(groundTruthUsers, reqGroups, includeDirectGroupMembershipsOnly)
}

// Existing Step 1. Dynamic members arrive as users; teams remain groups.
finalUsers, finalGroups, isACLEnabled, isRootAccess := applyResourceACL(..., reqUsers, reqGroups, ...)
if (!isACLEnabled || isRootAccess) && len(finalUsers) > 0 && len(finalGroups) > 0 {
    finalUsers = unionUsers(finalUsers, convertLiteUsersToUsers(
        filterUsersByGroups(groundTruthUsers, finalGroups, includeDirectGroupMembershipsOnly), customerID))
}
if isACLEnabled && !isRootAccess && len(finalUsers) == 0 {
    return emptyParseUserFilterResult()
}

// Existing Step 2 is unchanged. The selected-but-empty case returned above;
// empty finalUsers/finalGroups here retains the original no-filter meaning.
shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)
if !shouldUseAllAgents && len(finalUsers) > 0 {
    groundTruthUsers = updateGroundTruthUsers(finalUsers, groundTruthUsers)
}
// Existing Step 3: buildUserGroupMappings and result construction unchanged.
```

This retains the existing distinction between no selected audience and a selected audience with zero eligible users without changing `shouldUseAllAgents`. A missing/stale group should be handled as a resolution error or explicit empty selection, never as no filter. No source file was edited for this design.
