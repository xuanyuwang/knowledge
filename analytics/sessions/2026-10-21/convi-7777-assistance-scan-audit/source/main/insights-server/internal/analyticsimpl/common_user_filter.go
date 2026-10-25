package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"strings"

	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/client/grpc/registry"
	"github.com/cresta/shared-go/framework/log"
)

// ParseUserFilterResult contains the results of ParseUserFilterForAnalytics.
// This struct is designed to be compatible with shared/user-filter.FilteredUsersAndGroups
// for easier migration in the future.
type ParseUserFilterResult struct {
	// UserNameToGroupNamesMap maps user names to their group names.
	// Compatible with FilteredUsersAndGroups.UserNameToDirectGroupNames or UserNameToAllGroupNames.
	UserNameToGroupNamesMap map[string][]string

	// GroupsToAggregate are groups to aggregate in the response (when grouping by GROUP attribute).
	GroupsToAggregate []*userpb.Group

	// UsersFromGroups are users fetched from groups, used for response construction.
	UsersFromGroups []*userpb.User

	// FinalUsers are the final filtered users for query filtering via req.FilterByAttribute.Users.
	// These are sorted by Name.
	FinalUsers []*userpb.User

	// FinalGroups are the final filtered groups.
	// Note: These are expanded to users before the query, not used directly in query filtering.
	FinalGroups []*userpb.Group

	// ShouldQueryAllUsers indicates that the query should include all users matching the
	// ground truth criteria (role/status filters) without explicit user ID filtering
	// in the WHERE clause.
	//
	// This flag is set to true in two cases:
	//   Case 1: ACL disabled + empty user filter + empty group filter
	//     - No access control enforcement
	//     - Empty filters means "query all users"
	//   Case 2: ACL enabled + root access + empty user filter + empty group filter
	//     - Caller has unrestricted access
	//     - Empty filters means "query all users"
	//
	// This flag is NEVER true for:
	//   Case 3: ACL enabled + limited access
	//     - isRootAccess=false, so shouldUseAllAgents condition is never satisfied
	//     - FinalUsers contains only their managed users
	//     - WHERE clause MUST filter to those specific users
	//   Case 4: Any case with a user filter or group filter (even with ACL disabled or root access)
	//     - User/group filter means we're querying a specific subset of users
	//     - WHERE clause MUST filter to those specific users
	//
	// When ShouldQueryAllUsers is true:
	//   - FinalUsers contains all matching users (for response construction/metadata enrichment)
	//   - Callers should NOT add user filter to WHERE clause (to avoid query size limit)
	//   - Query will be filtered by time range, profile_id, etc.
	//
	// When ShouldQueryAllUsers is false:
	//   - FinalUsers should be used in WHERE clause
	//   - If FinalUsers is empty, caller should return empty response (no access)
	ShouldQueryAllUsers bool
}

func userAudienceToUsersAndGroups(audience *analyticspb.UserAudience) ([]*userpb.User, []*userpb.Group) {
	if audience == nil {
		return nil, nil
	}
	users := make([]*userpb.User, 0, len(audience.GetUsers()))
	for _, userName := range audience.GetUsers() {
		users = append(users, &userpb.User{Name: userName})
	}
	groups := make([]*userpb.Group, 0, len(audience.GetGroups()))
	for _, groupName := range audience.GetGroups() {
		groups = append(groups, &userpb.Group{Name: groupName})
	}
	return users, groups
}

func (a AnalyticsServiceImpl) resolveScorecardReviewerAudience(
	ctx context.Context,
	parent *shared.CustomerProfile,
	qaAttribute *analyticspb.QAAttribute,
	includeDirectGroupMembershipsOnly bool,
) (*ParseUserFilterResult, bool, error) {
	audience := qaAttribute.GetScorecardReviewerAudience()
	if audience == nil {
		return nil, false, nil
	}

	reqUsers, reqGroups := userAudienceToUsersAndGroups(audience)
	result, err := ParseUserFilterForAnalytics(
		ctx,
		a.logger,
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		parent.CustomerID,
		parent.ProfileID,
		reqUsers,
		reqGroups,
		false, // hasAgentAsGroupByKey
		includeDirectGroupMembershipsOnly,
		false, // listAgentOnly
		false, // includePeerUserStats
		false, // excludeDeactivatedUsers
	)
	if err != nil {
		return nil, false, err
	}

	filterUsers := result.FinalUsers
	filterGroups := result.FinalGroups
	shouldEarlyReturn := ApplyUserFilterFromResult(result, &filterUsers, &filterGroups, a.enableExtTableForUserFilter)
	audience.Users = make([]string, 0, len(filterUsers))
	for _, user := range filterUsers {
		audience.Users = append(audience.Users, user.GetName())
	}
	audience.Groups = nil
	return result, shouldEarlyReturn, nil
}

// ParseUserFilterForAnalytics extracts common user/group filtering logic for Analytics APIs.
//
// This function performs the following steps:
//  0. Fetch ALL users as ground truth (agents if listAgentOnly=true, all users if false),
//     then resolve selected group types and convert dynamic groups to member users
//  1. Apply ACL filtering to get managed users/groups and ACL state flags
//  2. Filter ACL users to ground truth subset (early return if no ACL users + limited access)
//  3. Build user-to-group mappings from ground truth users
//
// Parameters:
//   - listAgentOnly: If true, ground truth only includes agents; if false, includes all users
//   - excludeDeactivatedUsers: If true, ground truth excludes deactivated users
//   - includeDirectGroupMembershipsOnly: If true, only includes direct group memberships
//   - hasAgentAsGroupByKey: If true, includes per-agent group mappings in response
//   - includePeerUserStats: If true, includes peer users in ACL filtering
//
// Flag Combinations and Behavior:
//
// 1. ACL State (determined by applyResourceACL):
//   - ACL Disabled (isACLEnabled=false): Passes through input filters unchanged
//   - ACL Enabled + Root Access (isACLEnabled=true, isRootAccess=true): Passes through input filters
//   - ACL Enabled + Limited Access (isACLEnabled=true, isRootAccess=false): Returns managed users/groups only
//
// 2. Ground Truth Filtering (Step 0):
//   - listAgentOnly=true + excludeDeactivatedUsers=false: All agents (active + deactivated)
//   - listAgentOnly=true + excludeDeactivatedUsers=true: Active agents only
//   - listAgentOnly=false + excludeDeactivatedUsers=false: All users (active + deactivated)
//   - listAgentOnly=false + excludeDeactivatedUsers=true: Active users only
//
// 3. Step 2 Behavior (after ACL + ground truth intersection):
//   - Empty reqUsers + ACL disabled: Returns all ground truth users
//   - Empty reqUsers + Root access: Returns all ground truth users
//   - Empty reqUsers + Limited access with no managed users: Early return with empty results
//   - Non-empty reqUsers: Returns intersection of reqUsers, ACL users, and ground truth
//
// 4. Step 3 Behavior (buildUserGroupMappings):
//   - Builds user-to-group mappings from ground truth users
//   - Extracts groups for aggregation when grouping by GROUP attribute
//   - Result contains only users and groups that exist in ground truth
//
// Returns:
//   - *ParseUserFilterResult: A struct containing:
//   - UserNameToGroupNamesMap: Mapping from user names to their group names
//   - GroupsToAggregate: Groups to aggregate in the response
//   - UsersFromGroups: Users fetched from groups (for response construction)
//   - FinalUsers: Final filtered users (for query filtering via req.FilterByAttribute.Users), sorted by Name
//   - FinalGroups: Final filtered groups (expanded to users before query, not used directly in query)
//   - error: Any error encountered during processing
//
// Important notes:
//   - Ground truth serves as the source of truth for all filtering operations
//   - All user lists are intersected with ground truth to ensure consistent filtering
//   - Results are sorted by Name for deterministic ordering
//   - Early return when ACL is enabled + limited access + no managed users (see Step 1)
//   - Empty finalUsers from ParseUserFilterForAnalytics ALWAYS means "no users to query" because:
//     1. ACL disabled + empty request → Returns ALL agents/users from ground truth (not empty)
//     2. Root access + empty request → Returns ALL agents/users from ground truth (not empty)
//     3. Limited access + no permissions → Early return with empty results
//     4. Limited access + has permissions → Returns specific managed users (could be empty if none match ground truth)
func ParseUserFilterForAnalytics(
	ctx context.Context,
	logger log.Logger,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	configClient config.Client,
	aclHelper auth.ResourceACLHelper,
	customerID, profileID string,
	reqUsers []*userpb.User,
	reqGroups []*userpb.Group,
	hasAgentAsGroupByKey bool,
	includeDirectGroupMembershipsOnly bool,
	listAgentOnly bool,
	includePeerUserStats bool,
	excludeDeactivatedUsers bool,
) (*ParseUserFilterResult, error) {
	hadAudienceFilter := len(reqUsers) > 0 || len(reqGroups) > 0

	// Step 0: Fetch ALL agent-only users as ground truth (if listAgentOnly=true).
	// This serves as the source of truth for:
	//   1) Agent-only filtering (intersect all results with this)
	//   2) Metadata enrichment (Username, FullName)
	//   3) Deactivated user filtering (via excludeDeactivatedUsers parameter)
	groundTruthUsers, err := listAllUsers(
		ctx,
		internalUserClientRegistry,
		customerID,
		profileID,
		includeDirectGroupMembershipsOnly,
		excludeDeactivatedUsers,
		listAgentOnly,
	)
	if err != nil {
		return nil, err
	}
	logger.Infof(ctx, "[ParseUserFilterForAnalytics] after listAllUsers: groundTruthUsers length=%d", len(groundTruthUsers))

	// Dynamic groups select their member users. Resolve the name-only request groups
	// before applying the ACL, which grants group access through TEAM hierarchy.
	if len(reqGroups) > 0 {
		teamGroups, dynamicGroups, err := resolveSelectedGroups(ctx, userClientRegistry, internalUserClientRegistry, customerID, profileID, reqGroups)
		if err != nil {
			return nil, err
		}
		if len(dynamicGroups) > 0 {
			members := filterUsersByGroups(groundTruthUsers, dynamicGroups, includeDirectGroupMembershipsOnly)
			reqUsers = unionUsers(reqUsers, convertLiteUsersToUsers(members, customerID))
		}
		reqGroups = teamGroups
	}

	// An explicitly selected group with no eligible members is an empty audience,
	// not the unfiltered request represented by empty users and groups.
	if hadAudienceFilter && len(reqUsers) == 0 && len(reqGroups) == 0 {
		return emptyParseUserFilterResult(), nil
	}

	// Filter groundTruthUsers by reqGroups if specified, but ONLY when there's no user filter.
	// When reqUsers is also present, UNION semantics apply instead (see CONVI-6284).
	if len(reqGroups) > 0 && len(reqUsers) == 0 {
		groundTruthUsers = filterUsersByGroups(groundTruthUsers, reqGroups, includeDirectGroupMembershipsOnly)
	}

	// Step 1: Get resource ACL based on the caller identity.
	// Returns isACLEnabled and isRootAccess flags to distinguish:
	//   - ACL disabled OR root access: empty finalUsers means "all users"
	//   - ACL enabled + limited access: empty finalUsers means "no managed users"
	// Note: When ACL is enabled with limited access and no user filtering is applied,
	// the ACL helper always returns the user itself (even if they have no managed users),
	// meaning finalUsers may be non-empty containing only the caller.
	// NEW: applyResourceACL now expands groups to users and UNIONs them with aclUserIDs,
	// fixing the bug where users and groups were intersected instead of unioned.
	finalUsers, finalGroups, isACLEnabled, isRootAccess, err := applyResourceACL(
		ctx,
		configClient,
		customerID,
		profileID,
		aclHelper,
		reqUsers,
		reqGroups,
		includePeerUserStats,
		internalUserClientRegistry,
		includeDirectGroupMembershipsOnly,
		listAgentOnly,
		excludeDeactivatedUsers,
	)
	if err != nil {
		return nil, err
	}
	logger.Infof(ctx, "[ParseUserFilterForAnalytics] after applyResourceACL: finalUsers length=%d finalGroups length=%d isACLEnabled=%v isRootAccess=%v", len(finalUsers), len(finalGroups), isACLEnabled, isRootAccess)

	// CONVI-6284: When both users and groups are selected, UNION them
	// (selected users ∪ group members). For limited access (Case 3),
	// applyResourceACL already handles this internally.
	if (!isACLEnabled || isRootAccess) && len(finalUsers) > 0 && len(finalGroups) > 0 {
		groupMemberUsers := filterUsersByGroups(groundTruthUsers, finalGroups, includeDirectGroupMembershipsOnly)
		finalUsers = unionUsers(finalUsers, convertLiteUsersToUsers(groupMemberUsers, customerID))
	}

	// If ACL is enabled and not root access, and there are no ACL users, early return
	if isACLEnabled && !isRootAccess && len(finalUsers) == 0 {
		logger.Infof(ctx, "[ParseUserFilterForAnalytics] early return: ACL enabled, not root, no ACL users")
		return emptyParseUserFilterResult(), nil
	}
	// Step 2: Intersect ACL filtered users with ground truth users.
	// Special cases where empty finalUsers means "all agents":
	//   1) ACL disabled + no user filter + no group filter
	//   2) ACL enabled + root access + no user filter + no group filter
	// Otherwise: Filter finalUsers with ground truth
	//
	// IMPORTANT: We also check for empty finalGroups because when a group filter is specified,
	// we should NOT query all users - we should only query users in the specified groups.
	// This prevents the ShouldQueryAllUsers flag from being set to true when there's a group filter.
	shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) || // ACL disabled + no user/group filter
		(isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0) // ACL enabled + root access + no user/group filter
	if !shouldUseAllAgents && len(finalUsers) > 0 {
		// Filter finalUsers to only include agents from ground truth.
		// Note: We only filter when finalUsers is non-empty. When finalUsers is empty but
		// finalGroups is non-empty (group filter without user filter), the group filtering
		// is handled later in buildUserGroupMappings.
		groundTruthUsers = updateGroundTruthUsers(finalUsers, groundTruthUsers)
	}
	logger.Infof(ctx, "[ParseUserFilterForAnalytics] shouldUseAllAgents=%v groundTruthUsers length=%d", shouldUseAllAgents, len(groundTruthUsers))

	// Step 3: Build user-to-group mappings from groundTruthUsers.
	// This is used for:
	//   - Response construction (usersFromGroups parameter in retrieveAgentStatsInternal)
	//   - Group aggregation (when grouping by GROUP attribute)
	userNameToGroupNamesMap, groupsToAggregate, err := buildUserGroupMappings(
		ctx,
		userClientRegistry,
		internalUserClientRegistry,
		customerID,
		profileID,
		finalGroups,
		groundTruthUsers,
		hasAgentAsGroupByKey,
		includeDirectGroupMembershipsOnly,
	)
	if err != nil {
		return nil, err
	}

	// Convert groundTruthUsers to []*userpb.User for final result
	finalUsers = convertLiteUsersToUsers(groundTruthUsers, customerID)

	// usersFromGroups are users that belong to finalGroups (for response construction).
	// Reuse the already-converted finalUsers instead of converting a second time:
	// both derive from groundTruthUsers, which is keyed by the same resource name
	// that convertLiteUsersToUsers writes to User.Name.
	usersFromGroupNames := set.New[string]()
	for userName, groupNames := range userNameToGroupNamesMap {
		if len(groupNames) == 0 {
			continue
		}
		if _, exists := groundTruthUsers[userName]; !exists {
			continue
		}
		usersFromGroupNames.Add(userName)
	}
	usersFromGroups := make([]*userpb.User, 0, usersFromGroupNames.Len())
	for _, user := range finalUsers {
		if usersFromGroupNames.Has(user.Name) {
			usersFromGroups = append(usersFromGroups, user)
		}
	}

	// TODO: investigate whether these sorts are still needed. They add O(n log n)
	// work over the full profile on every request; if no downstream consumer relies
	// on finalUsers/usersFromGroups/groupsToAggregate being sorted by Name, remove them.
	slices.SortFunc(finalUsers, func(a, b *userpb.User) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(usersFromGroups, func(a, b *userpb.User) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(groupsToAggregate, func(a, b *userpb.Group) int {
		return strings.Compare(a.Name, b.Name)
	})

	logger.Infof(ctx, "[ParseUserFilterForAnalytics] returning: FinalUsers length=%d ShouldQueryAllUsers=%v", len(finalUsers), shouldUseAllAgents)
	return &ParseUserFilterResult{
		UserNameToGroupNamesMap: userNameToGroupNamesMap,
		GroupsToAggregate:       groupsToAggregate,
		UsersFromGroups:         usersFromGroups,
		FinalUsers:              finalUsers,
		FinalGroups:             finalGroups,
		ShouldQueryAllUsers:     shouldUseAllAgents,
	}, nil
}

func emptyParseUserFilterResult() *ParseUserFilterResult {
	return &ParseUserFilterResult{
		UserNameToGroupNamesMap: map[string][]string{},
		GroupsToAggregate:       []*userpb.Group{},
		UsersFromGroups:         []*userpb.User{},
		FinalUsers:              []*userpb.User{},
		FinalGroups:             []*userpb.Group{},
	}
}

// resolveSelectedGroups uses UserService metadata because analytics requests
// carry group names without their types. Groups absent from the current profile
// are ignored as empty selections, never treated as unrestricted access.
func resolveSelectedGroups(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	reqGroups []*userpb.Group,
) ([]*userpb.Group, []*userpb.Group, error) {
	for _, group := range reqGroups {
		name, err := userpb.ParseGroupName(group.GetName())
		if err != nil {
			return nil, nil, err
		}
		if name.CustomerID != customerID {
			return nil, nil, fmt.Errorf("group %q belongs to a different customer", group.GetName())
		}
	}

	resolvedGroups, err := shared.ListGroups(ctx, userClientRegistry, internalUserClientRegistry, customerID, profileID, reqGroups, true)
	if err != nil {
		return nil, nil, err
	}
	groupsByName := make(map[string]*userpb.Group, len(resolvedGroups))
	for _, group := range resolvedGroups {
		groupsByName[group.GetName()] = group
	}

	var teamGroups, dynamicGroups []*userpb.Group
	for _, requested := range reqGroups {
		group, ok := groupsByName[requested.GetName()]
		if !ok {
			continue
		}
		switch group.GetType() {
		case userpb.Group_TEAM:
			teamGroups = append(teamGroups, group)
		case userpb.Group_DYNAMIC_GROUP:
			dynamicGroups = append(dynamicGroups, group)
		default:
			return nil, nil, fmt.Errorf("unsupported group type %v for %q", group.GetType(), group.GetName())
		}
	}
	return teamGroups, dynamicGroups, nil
}

// convertLiteUsersToUsers converts map[string]*internaluserpb.LiteUser to []*userpb.User
func convertLiteUsersToUsers(liteUsers map[string]*internaluserpb.LiteUser, customerID string) []*userpb.User {
	result := make([]*userpb.User, 0, len(liteUsers))
	for _, liteUser := range liteUsers {
		result = append(result, &userpb.User{
			Name:     buildUserResourceName(customerID, liteUser.UserId),
			Username: liteUser.Username,
			FullName: liteUser.FullName,
		})
	}
	return result
}

// updateGroundTruthUsers updates agent ground truth by intersecting with the users parameter.
func updateGroundTruthUsers(users []*userpb.User, groundTruthUsers map[string]*internaluserpb.LiteUser) map[string]*internaluserpb.LiteUser {
	result := make(map[string]*internaluserpb.LiteUser, len(users))
	for _, user := range users {
		if agentUser, exists := groundTruthUsers[user.Name]; exists {
			result[user.Name] = agentUser // Use enriched version from ground truth
		}
	}
	return result
}

// filterUsersByGroups filters groundTruthUsers to only include users who are members of the specified groups.
// It uses the Memberships field in LiteUser to check group membership.
// When includeDirectGroupMembershipsOnly is true, only direct memberships are considered.
func filterUsersByGroups(
	groundTruthUsers map[string]*internaluserpb.LiteUser,
	groups []*userpb.Group,
	includeDirectGroupMembershipsOnly bool,
) map[string]*internaluserpb.LiteUser {
	if len(groups) == 0 {
		return groundTruthUsers
	}

	// Build a set of group IDs to filter by
	groupIDSet := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		groupName, err := userpb.ParseGroupName(group.Name)
		if err != nil {
			continue
		}
		groupIDSet[groupName.GroupID] = struct{}{}
	}

	// Filter users by group membership
	result := make(map[string]*internaluserpb.LiteUser)
	for userName, user := range groundTruthUsers {
		for _, membership := range user.Memberships {
			// Skip indirect memberships if only direct memberships are requested
			if includeDirectGroupMembershipsOnly && !membership.IsDirectMember {
				continue
			}
			if _, exists := groupIDSet[membership.Group.GroupId]; exists {
				result[userName] = user
				break
			}
		}
	}
	return result
}

// buildUserGroupMappings builds user-to-group mappings from groundTruthUsers.
// This function uses the group membership information already present in groundTruthUsers.
//
// Parameters:
//   - finalGroups: Groups to filter by. If empty, fetches all groups for the profile.
//   - groundTruthUsers: Users with their group memberships already populated.
//
// Returns:
//   - userNameToGroupNamesMap: mapping from user name to group names
//   - groupsToAggregate: groups that should be included in aggregation
func buildUserGroupMappings(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	finalGroups []*userpb.Group,
	groundTruthUsers map[string]*internaluserpb.LiteUser,
	hasAgentAsGroupByKey bool,
	includeDirectGroupMembershipsOnly bool,
) (map[string][]string, []*userpb.Group, error) {
	var (
		usersToGroups    = make(map[string][]string)
		groupNameToGroup = make(map[string]*userpb.Group)
	)

	// Use shared.ListGroups to fetch groups with child team expansion.
	// When a parent team is specified, ListGroups includes IncludeGroupMemberships
	// which returns child teams as GROUP-type members, expanding the group list
	// to include both the parent and its sub-teams. This is needed so that sub-teams
	// appear in groupsToAggregate for team leaderboard views.
	//
	// We always need to list groups and filter on customer profiles of the listed groups.
	//
	// As of 10/08/2024, we noticed a group in holidayinn-transfers-voice owning other holidayinn
	// profiles' agents. In this case, we cannot safely assume the empty group filter means all
	// groups. Instead it should be the groups only belonging to the specific customer profile
	// from Director page.
	groups, err := shared.ListGroups(ctx, userClientRegistry, internalUserClientRegistry, customerID, profileID, finalGroups, false)
	if err != nil {
		return nil, nil, err
	}

	// Build groupNames only from TEAM type groups.
	// This is important because shared.ListGroups returns both TEAM and DYNAMIC_GROUP types,
	// but the membership loop below only processes TEAM memberships.
	// If we include dynamic group names here, the slices.Contains check below would fail
	// for all team memberships when filtering by dynamic groups, causing GroupsToAggregate
	// to be empty.
	groupNames := []string{}
	for _, group := range groups {
		if group.Type == userpb.Group_TEAM {
			groupNames = append(groupNames, group.Name)
		}
	}

	// Build mappings from groundTruthUsers' memberships
	for userName, user := range groundTruthUsers {
		for _, membership := range user.Memberships {
			groupID := membership.Group.GroupId
			if !hasAgentAsGroupByKey && groupID == consts.RootGroupID {
				// Skip the root group for team leaderboard
				continue
			}
			if !hasAgentAsGroupByKey && groupID == consts.DefaultGroupID {
				// Skip the default group for team leaderboard
				continue
			}
			if (hasAgentAsGroupByKey || includeDirectGroupMembershipsOnly) && !membership.IsDirectMember {
				// Skip the indirect group for agent leaderboard
				continue
			}
			if membership.Group.GroupType != internaluserpb.LiteGroup_TEAM {
				// Skip when a group is not team, i.e. when a group is virtual group
				continue
			}

			groupName := buildGroupResourceName(customerID, groupID)
			usersToGroups[userName] = append(usersToGroups[userName], groupName)

			// If we have a group key by agent, we should return the team group for the agent.
			// Otherwise, when we have a non-empty team filter, we should only return a subset of
			// groups from user's group memberships; otherwise, it might return some
			// unnecessary groups.
			if hasAgentAsGroupByKey || len(groupNames) == 0 || slices.Contains(groupNames, groupName) {
				groupNameToGroup[groupName] = &userpb.Group{
					Name:        groupName,
					DisplayName: membership.Group.DisplayName,
					Type:        userpb.Group_TEAM,
					IsRoot:      false,
				}
			}
		}
	}

	// Sort group names alphabetically so agents with multiple team memberships always
	// get the same team label regardless of API response ordering.
	for userName := range usersToGroups {
		slices.Sort(usersToGroups[userName])
	}

	return usersToGroups, fn.MapValueToSlice(groupNameToGroup), nil
}

// listAllUsers lists all users (or agents only if listAgentOnly=true).
// Returns LiteUser objects which include group membership information.
func listAllUsers(
	ctx context.Context,
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	includeDirectGroupMembershipsOnly bool,
	excludeDeactivatedUsers bool,
	listAgentOnly bool,
) (map[string]*internaluserpb.LiteUser, error) {
	internalUserClient, err := internalUserClientRegistry.CreateClient(customerID, profileID)
	if err != nil {
		return nil, err
	}
	var nextPageOffset int32
	allAgentUsers := make(map[string]*internaluserpb.LiteUser)
	for {
		listUsersForAnalyticsReq := &internaluserpb.ListUsersForAnalyticsRequest{
			Parent:                          shared.Customer{CustomerID: customerID}.String(),
			ProfileIds:                      []string{profileID},
			AgentOnly:                       listAgentOnly,
			IncludeIndirectGroupMemberships: !includeDirectGroupMembershipsOnly,
			IncludeInactiveUsers:            !excludeDeactivatedUsers,
			PageOffset:                      nextPageOffset,
		}
		listUsers, err := internalUserClient.ListUsersForAnalytics(ctx, listUsersForAnalyticsReq)
		if err != nil {
			return nil, err
		}
		for _, user := range listUsers.Users {
			userName := buildUserResourceName(customerID, user.UserId)
			allAgentUsers[userName] = user
		}
		if listUsers.NextPageOffset == 0 {
			break
		}
		nextPageOffset = listUsers.NextPageOffset
	}

	return allAgentUsers, nil
}

// expandGroupsToUsers expands groups to their member users.
// This is used to implement the UNION semantic for users and groups filters.
// Note: ListUsersForAnalytics already filters by listAgentOnly and excludeDeactivatedUsers,
// so the returned users naturally match the ground truth criteria. No additional filtering needed.
func expandGroupsToUsers(
	ctx context.Context,
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID string,
	profileID string,
	groups []*userpb.Group,
	includeDirectGroupMembershipsOnly bool,
	listAgentOnly bool,
	excludeDeactivatedUsers bool,
) ([]*userpb.User, error) {
	if len(groups) == 0 {
		return []*userpb.User{}, nil
	}

	// Extract group IDs from groups
	groupIDs := []string{}
	for _, group := range groups {
		groupName, err := userpb.ParseGroupName(group.Name)
		if err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupName.GroupID)
	}

	// Call ListUsersForAnalytics with group filter, similar to listAllUsers
	// This already respects listAgentOnly and excludeDeactivatedUsers flags
	internalUserClient, err := internalUserClientRegistry.CreateClient(customerID, profileID)
	if err != nil {
		return nil, err
	}

	var nextPageOffset int32
	allUsers := make(map[string]*userpb.User)
	for {
		listUsersForAnalyticsReq := &internaluserpb.ListUsersForAnalyticsRequest{
			Parent:                          shared.Customer{CustomerID: customerID}.String(),
			ProfileIds:                      []string{profileID},
			GroupIds:                        groupIDs, // Filter by groups
			AgentOnly:                       listAgentOnly,
			IncludeIndirectGroupMemberships: !includeDirectGroupMembershipsOnly,
			IncludeInactiveUsers:            !excludeDeactivatedUsers,
			PageOffset:                      nextPageOffset,
		}
		listUsers, err := internalUserClient.ListUsersForAnalytics(ctx, listUsersForAnalyticsReq)
		if err != nil {
			return nil, err
		}
		for _, user := range listUsers.Users {
			userName := buildUserResourceName(customerID, user.UserId)
			allUsers[userName] = &userpb.User{
				Name:     userName,
				Username: user.Username,
				FullName: user.FullName,
			}
		}
		if listUsers.NextPageOffset == 0 {
			break
		}
		nextPageOffset = listUsers.NextPageOffset
	}

	// Convert map to slice
	result := []*userpb.User{}
	for _, user := range allUsers {
		result = append(result, user)
	}

	return result, nil
}

// ApplyUserFilterFromResult applies user filtering from ParseUserFilterResult to the request's filter attributes.
// It updates the users and groups fields based on the result and returns true if the caller should
// early return (when there are no users to query due to limited access with no managed users).
//
// When useExtTable is true, FinalUsers are always passed through (even when ShouldQueryAllUsers=true)
// because ext tables handle large user lists efficiently via the binary protocol, eliminating the
// query size concern that ShouldQueryAllUsers was designed to work around.
//
// This function works with both analyticspb.Attribute and analyticspb.QAAttribute by accepting
// pointers to the Users and Groups fields directly.
//
// Usage pattern:
//
//	if shouldEarlyReturn := ApplyUserFilterFromResult(result, &req.FilterByAttribute.Users, &req.FilterByAttribute.Groups, a.enableExtTableForUserFilter); shouldEarlyReturn {
//	    return &analyticspb.RetrieveXXXStatsResponse{}, nil
//	}
func ApplyUserFilterFromResult(result *ParseUserFilterResult, users *[]*userpb.User, groups *[]*userpb.Group, useExtTable bool) bool {
	// Handle user filtering for the WHERE clause:
	// - useExtTable=true: Always use FinalUsers via ext table (no query size limit concern)
	// - ShouldQueryAllUsers=true && !useExtTable: Don't add user filter (avoids query size limit)
	//   This happens only for Case 1 (ACL disabled) or Case 2 (root access) with empty filters.
	// - ShouldQueryAllUsers=false: Use FinalUsers in WHERE clause
	//   This happens for Case 3 (limited access) or Case 4 (user/group filter) where we MUST filter.
	if result.ShouldQueryAllUsers && !useExtTable {
		*users = []*userpb.User{}
	} else {
		*users = result.FinalUsers
	}
	*groups = result.FinalGroups

	// Early return if no users to query (e.g., limited access with no managed users)
	// Note: Only check this when NOT querying all users, since ShouldQueryAllUsers=true
	// means we want all users (FinalUsers is populated for metadata enrichment only).
	return !result.ShouldQueryAllUsers && len(result.FinalUsers) == 0
}

// unionUsers merges two user slices, deduplicating by user Name.
func unionUsers(users1 []*userpb.User, users2 []*userpb.User) []*userpb.User {
	userMap := map[string]*userpb.User{}

	for _, user := range users1 {
		userMap[user.Name] = user
	}

	for _, user := range users2 {
		userMap[user.Name] = user
	}

	result := []*userpb.User{}
	for _, user := range userMap {
		result = append(result, user)
	}

	return result
}

// applyResourceACL updates the users/groups filters to respect the resource ACL on the authenticated user.
//
// Return values (users, groups, isACLEnabled, isRootAccess, error):
//
// Case 1: ACL is DISABLED (EnableAdvancedDataAccessControl=false)
//   - Returns: (input users, input groups, isACLEnabled=false, isRootAccess=false, nil)
//   - Behavior: Passes through input filters unchanged
//   - Interpretation: No ACL enforcement, empty input means "access all entities"
//
// Case 2: ACL is ENABLED + Root Access (IsRootAccess=true)
//   - Returns: (input users, input groups, isACLEnabled=true, isRootAccess=true, nil)
//   - Behavior: Passes through input filters unchanged
//   - Interpretation: Root access, empty input means "access all entities"
//
// Case 3: ACL is ENABLED + Limited Access (IsRootAccess=false)
//   - Returns: (managed users, managed groups, isACLEnabled=true, isRootAccess=false, nil)
//   - Behavior: Returns only users/groups the caller has permission to access
//   - Interpretation: Limited access, empty result means "no managed entities"
//
// The combination of flags distinguishes these critical cases:
//   - (isACLEnabled=false, *): No ACL, treat empty as "all access"
//   - (isACLEnabled=true, isRootAccess=true): Root access, treat empty as "all access"
//   - (isACLEnabled=true, isRootAccess=false): Limited access, treat empty as "no access"
func applyResourceACL(
	ctx context.Context,
	configClient config.Client,
	customerID string,
	profileID string,
	resourceACLHelper auth.ResourceACLHelper,
	filteredUsers []*userpb.User,
	filteredGroups []*userpb.Group,
	includePeerUserStats bool,
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	includeDirectGroupMembershipsOnly bool,
	listAgentOnly bool,
	excludeDeactivatedUsers bool,
) ([]*userpb.User, []*userpb.Group, bool, bool, error) {
	isACLEnabled := false
	isRootAccess := false
	customerConfig, err := configClient.Get(customerID)
	if err != nil {
		return nil, nil, isACLEnabled, isRootAccess, err
	}
	if customerConfig.GetEnableAdvancedDataAccessControl() {
		isACLEnabled = true
		aclOptions, err := shared.BuildResourceACLOptions(filteredUsers, filteredGroups, includePeerUserStats)
		if err != nil {
			return nil, nil, isACLEnabled, isRootAccess, err
		}
		resourceACL, err := resourceACLHelper.GetResourceACL(ctx, customerID, aclOptions...)
		if err != nil {
			return nil, nil, isACLEnabled, isRootAccess, err
		}
		isRootAccess = resourceACL.IsRootAccess
		if !resourceACL.IsRootAccess {
			// Get ACL-filtered users
			aclUserIDs := resourceACL.GetAllUserIDs()
			filteredUsers = shared.ConvertUserIDsToUsers(customerID, aclUserIDs)

			// Get ACL-filtered groups
			aclGroupIDs := resourceACL.GetAllGroupIDs()
			filteredGroups = shared.ConvertGroupIDsToGroups(customerID, aclGroupIDs)

			// NEW: Expand groups to users and UNION with filteredUsers
			//
			// Only expand when aclGroupIDs is non-empty:
			//   Case A: Caller has access to groups → expand and union
			//   Case B-1: ACL disabled/root access + empty reqGroups → aclGroupIDs=[] (no filtering)
			//   Case B-2: Limited access + empty reqGroups → aclGroupIDs=[] (groups not requested)
			//   Case B-3: Limited access + no group access → aclGroupIDs=[] (no access)
			//
			// In all Case B scenarios, empty aclGroupIDs means nothing to expand.
			// The final result is: filteredUsers = (aclUserIDs ∪ users-from-aclGroupIDs)
			if len(filteredGroups) > 0 {
				usersFromGroups, err := expandGroupsToUsers(
					ctx,
					internalUserClientRegistry,
					customerID,
					profileID,
					filteredGroups,
					includeDirectGroupMembershipsOnly,
					listAgentOnly,
					excludeDeactivatedUsers,
				)
				if err != nil {
					return nil, nil, isACLEnabled, isRootAccess, err
				}

				// UNION: Merge users from groups with aclUserIDs
				// This fixes the bug where users and groups were intersected instead of unioned
				filteredUsers = unionUsers(filteredUsers, usersFromGroups)
			}
		}
		// If IsRootAccess=true, original filteredUsers and filteredGroups are returned unchanged
	}
	return filteredUsers, filteredGroups, isACLEnabled, isRootAccess, nil
}
