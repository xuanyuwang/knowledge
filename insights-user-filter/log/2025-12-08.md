# Daily Engineering Notes – 2025-12-08

## 1. Fixes (Bugs / Issues Resolved)
### Problem:
Well it's a very long story.

At beginning, we want to support a feature, which is only show data of agent-only users on a page.

Agent-only user: a user whose role is only agent, no other roles.

I looked at the FE code, and fonud it's a big work: the page uses ~20 APIs of a service to get data.

Naturally, I don't want to fix 20 APIs. So I was trying to filter the data at FE.

But a closer look told me that's not easy. Because those APIs return statistic data directly. So there is no way for me to update the results to agent-only users -- I don't know what data to filter out.

------

I then went to BE. Lucikly, there is already a function that does the work by the user service team. They create a new function, applied to all APIs.

The function has a parameter: listAgentOnly. Currently, its value is true for only one API.

So looks like it's an easy work: turn listAgentOnly on for all other APIs! Since all APIs are using the same pattern to parse user filters, and there is one API that has been running for months to prove the correctness.

------

Of course, things can't be that smooth.

After consulting user service team and turned flags on, we got a new error: those non-agent-only users are displayed as `unknown` on FE page.

My instinction told me it's just some data mismatching.

And my instinction is right - partially.

It turned out that we have two sets of tools to determine which users' data to fetch.

And they return different sets of users!

The two sets are both used, in different places, for different purposes!

  1. req.FilterByAttribute.Users (ACL users) - used for query filtering
  2. users parameter (from ListUsersMappedToGroups) - used for constructing the response

That's terrible.

------

The context of our codebase is that the user filtering code has always been messy. The requirements were brought up one-by-one over years. So we never had a chance to systemtically review the code design.

I made a one-pager last year and built a common user filter parser, which was working very well in the past year.

I was thinking of using it in this case, but that would be a drastic code change.

I investigated for a long time to understand the code in the Insights service for all of its APIs.

I found that they all share similar pattern. So potentially we can finally use a common function for all of them.

------

Working on refactoring existing code is very painful.

There are so many aspects to consider when filtering users.

There is an ACL (adcanced data access control) system, which can be configed to on/off.

The user filtering is controlled by the folloing flags:
- listAgentOnly
- excludeDeactivatedUsers
- shouldMoveFiltersToUserFilter: MoveFiltersToUserFilter is a legacy code function to call because we don't want to replace it for now, even though it's messy inside
- includeDirectGroupMembershipsOnly: If true, only includes direct group memberships
- hasAgentAsGroupByKey: If true, includes per-agent group mappings in response
- includePeerUserStats: If true, includes peer users in ACL filtering
- if ACL is enabled
- when ACL is enabled, we need to consider if the requester has root access

There are so many combinations. The following is the signature and comment:
```
// ParseUserFilterForAnalytics extracts common user/group filtering logic for Analytics APIs.
//
// This function performs the following steps:
// 0. Fetch ALL users as ground truth (agents if listAgentOnly=true, all users if false)
// 1. Apply ACL filtering to get managed users/groups and ACL state flags
// 2. Filter ACL users to ground truth subset (early return if no ACL users + limited access)
// 3. List users from groups and filter to ground truth subset
// 4. Optionally apply additional filters (group expansion, deactivated user filtering)
//
// Parameters:
//   - listAgentOnly: If true, ground truth only includes agents; if false, includes all users
//   - excludeDeactivatedUsers: If true, ground truth excludes deactivated users
//   - shouldMoveFiltersToUserFilter: If true, executes Step 4 to expand groups and filter deactivated users
//   - includeDirectGroupMembershipsOnly: If true, only includes direct group memberships
//   - hasAgentAsGroupByKey: If true, includes per-agent group mappings in response
//   - includePeerUserStats: If true, includes peer users in ACL filtering
//
// Flag Combinations and Behavior:
//
// 1. ACL State (determined by applyResourceACL):
//    - ACL Disabled (isACLEnabled=false): Passes through input filters unchanged
//    - ACL Enabled + Root Access (isACLEnabled=true, isRootAccess=true): Passes through input filters
//    - ACL Enabled + Limited Access (isACLEnabled=true, isRootAccess=false): Returns managed users/groups only
//
// 2. Ground Truth Filtering (Step 0):
//    - listAgentOnly=true + excludeDeactivatedUsers=false: All agents (active + deactivated)
//    - listAgentOnly=true + excludeDeactivatedUsers=true: Active agents only
//    - listAgentOnly=false + excludeDeactivatedUsers=false: All users (active + deactivated)
//    - listAgentOnly=false + excludeDeactivatedUsers=true: Active users only
//
// 3. Step 2 Behavior (after ACL + ground truth intersection):
//    - Empty reqUsers + ACL disabled: Returns all ground truth users
//    - Empty reqUsers + Root access: Returns all ground truth users
//    - Empty reqUsers + Limited access with no managed users: Early return with empty results
//    - Non-empty reqUsers: Returns intersection of reqUsers, ACL users, and ground truth
//
// 4. Step 3 Behavior (ListUsersMappedToGroups):
//    - Always calls with IncludeInactiveUsers=true (returns all users from groups)
//    - Result is intersected with ground truth (respects listAgentOnly and excludeDeactivatedUsers)
//    - Empty groups: Returns empty usersFromGroups but still builds group mappings
//
// 5. Step 4 Behavior (MoveFiltersToUserFilter, if shouldMoveFiltersToUserFilter=true):
//    - Case a) Empty aclUsers + empty groups + excludeDeactivatedUsers=true:
//        Calls ParseFiltersToUsers with activeUserFilter=true, returns active users only
//    - Case a) Empty aclUsers + non-empty groups:
//        Expands groups and returns all users from those groups
//    - Case b1) Non-empty aclUsers + non-empty groups:
//        Appends group users to aclUsers (union operation)
//    - Case b2) Non-empty aclUsers + excludeDeactivatedUsers=true:
//        Filters aclUsers to keep only active users
//    - All results are intersected with ground truth before returning
//
// Return values:
//   - userNameToGroupNamesMap: Mapping from user names to their group names
//   - groupsToAggregate: Groups to aggregate in the response
//   - usersFromGroups: Users fetched from groups (for response construction)
//   - aclUsers: Final filtered users (for query filtering via req.FilterByAttribute.Users), sorted by UserID
//   - aclGroups: ACL-filtered groups (for query filtering via req.FilterByAttribute.Groups)
//
// Important notes:
//   - Ground truth serves as the source of truth for all filtering operations
//   - All user lists are intersected with ground truth to ensure consistent filtering
//   - Results are sorted by Name for deterministic ordering
//   - Early return at line 144-146 when ACL is enabled + limited access + no managed users
func ParseUserFilterForAnalytics(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	configClient config.Client,
	aclHelper auth.ResourceACLHelper,
	customerID, profileID string,
	reqUsers []*userpb.User,
	reqGroups []*userpb.Group,
	hasAgentAsGroupByKey bool,
	includeDirectGroupMembershipsOnly bool,
	enableListUsersCache bool,
	listUsersCache shared.ListUsersCache,
	listAgentOnly bool,
	includePeerUserStats bool,
	shouldMoveFiltersToUserFilter bool,
	excludeDeactivatedUsers bool,
) (
	map[string][]string, // userNameToGroupNamesMap
	[]*userpb.Group, // groupsToAggregate
	[]*userpb.User, // usersFromGroups
	[]*userpb.User, // aclUsers
	[]*userpb.Group, // aclGroups
	error, // err
)
```

It's even more painful to test it throughly. The testing for this function takes 1200+ lines of code.

We also need to make sure the code is backward compatible.

### Symptoms:
The data of Insights service APIs includes non-agent-only users
### Root Cause:
We have two sets of tools to filter users. One of it doesn't support agent-only filter.
### How I Diagnosed It:
Use Claude Code
### Final Fix:
Have a common function ParseUserFilterForAnalytics for all those APIs.
### Preventative Ideas:
Through tests

## 2. Learnings (New Knowledge)
### What I learned:
### Context:
### Why it's important:
### Example:
### When to apply:

## 3. Surprises (Unexpected Behavior)
### What surprised me:
### Expected vs actual behavior:
### Why it happened:
### Takeaway:

## 4. Explanations I Gave
### Who I explained to (team / code review / slack):
### Topic:
### Summary of explanation:
### Key concepts clarified:
### Possible blog angle:

## 5. Confusing Things (First Confusion → Later Clarity)
### What was confusing:
### Why it was confusing:
### How I figured it out:
### Clean explanation (my future-self will thank me):
### Mental model:

## 6. Things I Googled Multiple Times
### Search topic:
### Why I kept forgetting:
### Clean “final answer”:
### Snippet / Command / Example:

## 7. Code Patterns I Used Today
### Pattern name:
### Situation:
### Code example:
### When this pattern works best:
### Pitfalls:

## 8. Design Decisions / Tradeoffs
### Problem being solved:
### Options considered:
### Decision made:
### Tradeoffs:
### Why this matters at a system level:
### Future considerations:

---

## Screenshots
(Drag & paste images here)

## Raw Snippets / Logs
\`\`\`
Paste raw logs, stack traces, or snippets here
\`\`\`

## Blog Potential
### Short post ideas:
### Deep-dive post ideas:
