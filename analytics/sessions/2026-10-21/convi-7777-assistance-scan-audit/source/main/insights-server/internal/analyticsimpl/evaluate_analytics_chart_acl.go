package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// applyACLToChartDataConfig applies ACL filtering to chart data config similar to RetrieveClosedConversations.
// It mimics the complex ACL logic from retrieve_closed_conversations.go, handling various scenarios:
// - a) empty user filter, empty group filter => aclUsers non-empty, aclGroups empty
// - b) non-empty user filter, empty group filter => intersection with ACL users
// - c) empty user filter, non-empty group filter => expand groups to users and apply ACL
// - d) non-empty user filter, non-empty group filter => complex intersection logic
//
// noAccessibleFilterUsers is true when the user/group filters resolve to no user within the ACL; the
// caller must return an empty response rather than evaluate the chart without those filters.
func (a AnalyticsServiceImpl) ApplyACLToChartDataConfig(
	ctx context.Context,
	customerID string,
	profileID string,
	chartDataConfig *dashboardpb.ChartDataConfig,
) (noAccessibleFilterUsers bool, err error) {
	// Extract users and groups from filters and groupBy
	originalFilterGroups, originalFilterUsers, originalGroupByGroups, originalGroupByUsers := extractUserNamesAndGroupNamesFromChartDataConfig(chartDataConfig)

	// Apply ACL logic separately for filter users/groups and groupBy users/groups
	aclFilteredFilterUsers, aclFilteredGroupByUsers, noAccessibleFilterUsers, err := a.applyACLToFilterAndGroupByUsersIndependently(
		ctx, customerID, profileID,
		originalFilterUsers, originalFilterGroups,
		originalGroupByUsers, originalGroupByGroups)
	if err != nil {
		return false, err
	}
	if noAccessibleFilterUsers {
		return true, nil
	}

	// Apply the ACL-filtered users back to the chartDataConfig
	// Now we use the independently filtered users for each category
	err = a.applyACLFilteredUsersToChartConfig(ctx, customerID, profileID, chartDataConfig,
		aclFilteredFilterUsers, aclFilteredGroupByUsers, originalGroupByGroups)
	if err != nil {
		return false, err
	}

	return false, nil
}

// applyACLToFilterAndGroupByUsersIndependently applies ACL filtering to filter users/groups and groupBy users/groups independently
func (a AnalyticsServiceImpl) applyACLToFilterAndGroupByUsersIndependently(
	ctx context.Context,
	customerID string,
	profileID string,
	originalFilterUsers []*userpb.User,
	originalFilterGroups []*userpb.Group,
	originalGroupByUsers []*userpb.User,
	originalGroupByGroups []*userpb.Group,
) (aclFilteredFilterUsers, aclFilteredGroupByUsers []*userpb.User, noAccessibleFilterUsers bool, err error) {
	hasUserFilters := len(originalFilterUsers) > 0 || len(originalFilterGroups) > 0
	// Check if filter and groupBy have the same users/groups to optimize ACL calls
	if a.sameUserGroupSets(originalFilterUsers, originalFilterGroups, originalGroupByUsers, originalGroupByGroups) {
		// If they're the same, apply ACL once and use the result for both
		aclFilteredUsers, noAccessibleUsers, err := a.applyACLToUserGroupSet(ctx, customerID, profileID, originalFilterUsers, originalFilterGroups)
		if err != nil {
			return nil, nil, false, err
		}
		return aclFilteredUsers, aclFilteredUsers, hasUserFilters && noAccessibleUsers, nil
	}

	// Apply ACL to filter users/groups (only if there are users/groups to process)
	if hasUserFilters {
		aclFilteredFilterUsers, noAccessibleFilterUsers, err = a.applyACLToUserGroupSet(ctx, customerID, profileID, originalFilterUsers, originalFilterGroups)
		if err != nil {
			return nil, nil, false, err
		}
		if noAccessibleFilterUsers {
			return nil, nil, true, nil
		}
	}

	// Apply ACL to groupBy users/groups (only if there are users/groups to process)
	if len(originalGroupByUsers) > 0 || len(originalGroupByGroups) > 0 {
		aclFilteredGroupByUsers, _, err = a.applyACLToUserGroupSet(ctx, customerID, profileID, originalGroupByUsers, originalGroupByGroups)
		if err != nil {
			return nil, nil, false, err
		}
	}

	return aclFilteredFilterUsers, aclFilteredGroupByUsers, false, nil
}

// sameUserGroupSets checks if two sets of users and groups are identical
func (a AnalyticsServiceImpl) sameUserGroupSets(
	users1 []*userpb.User, groups1 []*userpb.Group,
	users2 []*userpb.User, groups2 []*userpb.Group,
) bool {
	// Check if user sets are the same
	if len(users1) != len(users2) {
		return false
	}
	userMap1 := make(map[string]bool)
	for _, user := range users1 {
		userMap1[user.Name] = true
	}
	for _, user := range users2 {
		if !userMap1[user.Name] {
			return false
		}
	}

	// Check if group sets are the same
	if len(groups1) != len(groups2) {
		return false
	}
	groupMap1 := make(map[string]bool)
	for _, group := range groups1 {
		groupMap1[group.Name] = true
	}
	for _, group := range groups2 {
		if !groupMap1[group.Name] {
			return false
		}
	}

	return true
}

// applyACLToUserGroupSet applies ACL filtering to a specific set of users and groups
func (a AnalyticsServiceImpl) applyACLToUserGroupSet(
	ctx context.Context,
	customerID string,
	profileID string,
	users []*userpb.User,
	groups []*userpb.Group,
) (aclFilteredUsers []*userpb.User, noAccessibleUsers bool, err error) {
	tempAttribute := &analyticspb.Attribute{
		Users:  users,
		Groups: groups,
	}
	shouldReturnEmptyResponse, err := shared.ApplyResourceACLToAttributeFilter(
		ctx,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		customerID,
		profileID,
		tempAttribute,
		true,
		users,
		true, // returnEmptyResponseWhenNoUsersAfterMove
	)
	if err != nil {
		return nil, false, err
	}
	if shouldReturnEmptyResponse {
		return []*userpb.User{}, true, nil
	}
	return tempAttribute.Users, false, nil
}

// applyACLFilteredUsersToChartConfig applies the ACL-filtered users back to the chart data config
// by intersecting them separately with filter users/groups and groupBy users/groups
func (a AnalyticsServiceImpl) applyACLFilteredUsersToChartConfig(
	ctx context.Context,
	customerID string,
	profileID string,
	chartDataConfig *dashboardpb.ChartDataConfig,
	aclFilteredFilterUsers []*userpb.User,
	aclFilteredGroupByUsers []*userpb.User,
	originalGroupByGroups []*userpb.Group,
) error {
	// We now have ACL-filtered users for both filters and groupBys independently
	// No need for complex intersection logic since it's already been done
	filteredFilterUsers := aclFilteredFilterUsers
	filteredGroupByUsers := aclFilteredGroupByUsers

	// For groups, we need to filter them based on ACL-filtered users
	var filteredGroupByGroups []*userpb.Group
	if len(originalGroupByGroups) > 0 && len(aclFilteredGroupByUsers) > 0 {
		var err error
		filteredGroupByGroups, err = a.filterGroupsByACLUsers(ctx, customerID, profileID, originalGroupByGroups, aclFilteredGroupByUsers)
		if err != nil {
			return err
		}
	}

	// Apply the filtered users and groups back to the chart configuration
	err := a.updateChartConfigWithFilteredUsersAndGroups(chartDataConfig, filteredFilterUsers, filteredGroupByUsers, filteredGroupByGroups)
	if err != nil {
		return err
	}

	return nil
}

// filterGroupsByACLUsers filters groups to exclude those containing users not in ACL
func (a AnalyticsServiceImpl) filterGroupsByACLUsers(
	ctx context.Context,
	customerID string,
	profileID string,
	groups []*userpb.Group,
	aclFilteredUsers []*userpb.User,
) ([]*userpb.Group, error) {
	if len(groups) == 0 || len(aclFilteredUsers) == 0 {
		return []*userpb.Group{}, nil
	}

	// Create a set of ACL-allowed user names for efficient lookup
	aclUserSet := make(map[string]struct{})
	for _, user := range aclFilteredUsers {
		aclUserSet[user.Name] = struct{}{}
	}

	var filteredGroups []*userpb.Group

	for _, group := range groups {
		// Expand this group to its member users
		tempAttribute := &analyticspb.Attribute{
			Groups: []*userpb.Group{group},
		}

		expandedAttribute, err := shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			customerID,
			profileID,
			tempAttribute,
			false, // includeDirectGroupMembershipsOnly
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}

		// Check if all users in this group are within ACL
		filterUsersInACL := true
		for _, user := range expandedAttribute.Users {
			if _, exists := aclUserSet[user.Name]; !exists {
				filterUsersInACL = false
				break
			}
		}

		// Only include the group if all its users are within ACL
		if filterUsersInACL {
			filteredGroups = append(filteredGroups, group)
		}
	}

	return filteredGroups, nil
}

// updateChartConfigWithFilteredUsersAndGroups updates the chart data config with the ACL-filtered users and groups
func (a AnalyticsServiceImpl) updateChartConfigWithFilteredUsersAndGroups(
	chartDataConfig *dashboardpb.ChartDataConfig,
	filteredFilterUsers []*userpb.User,
	filteredGroupByUsers []*userpb.User,
	filteredGroupByGroups []*userpb.Group,
) error {
	// Update filters with ACL-filtered users
	if len(filteredFilterUsers) > 0 {
		// Replace existing user/group filters with ACL-filtered user filters
		newFilters := []*dashboardpb.ChartDataFilter{}

		// Keep non-user/group filters
		for _, filter := range chartDataConfig.GetFilters() {
			if !a.isUserOrGroupFilter(filter) {
				newFilters = append(newFilters, filter)
			}
		}

		// Add ACL-filtered user filter
		userFilter := a.createUserFilter(filteredFilterUsers)
		if userFilter != nil {
			newFilters = append(newFilters, userFilter)
		}

		chartDataConfig.Filters = newFilters
	} else {
		// If no users remain after ACL filtering, remove all user/group filters
		newFilters := []*dashboardpb.ChartDataFilter{}
		for _, filter := range chartDataConfig.GetFilters() {
			if !a.isUserOrGroupFilter(filter) {
				newFilters = append(newFilters, filter)
			}
		}
		chartDataConfig.Filters = newFilters
	}

	// Update groupBy with ACL-filtered users and groups
	if len(filteredGroupByUsers) > 0 || len(filteredGroupByGroups) > 0 {
		// Replace existing user/group groupBy with ACL-filtered user/group groupBy while preserving order
		newGroupBys := []*dashboardpb.ChartGroupBy{}
		userGroupByAdded := false
		groupGroupByAdded := false

		// Process groupBys in original order to preserve position
		for _, groupBy := range chartDataConfig.GetGroupBy() {
			isUserGroupBy, isGroupGroupBy := a.isUserGroupBy(groupBy), a.isGroupGroupBy(groupBy)
			if !isUserGroupBy && !isGroupGroupBy {
				// Keep non-user/group groupBys in their original position
				newGroupBys = append(newGroupBys, groupBy)
			} else {
				// Replace user/group groupBy with ACL-filtered versions in original position
				// Add ACL-filtered user groupBy first (if any and not already added)
				if isUserGroupBy && len(filteredGroupByUsers) > 0 && !userGroupByAdded {
					userGroupBy := a.createGroupByUserWithConfig(filteredGroupByUsers, groupBy.Config)
					if userGroupBy != nil {
						newGroupBys = append(newGroupBys, userGroupBy)
						userGroupByAdded = true
					}
				}
				// Add ACL-filtered group groupBy second (if any and not already added)
				if isGroupGroupBy && len(filteredGroupByGroups) > 0 && !groupGroupByAdded {
					groupGroupBy := a.createGroupByGroupWithConfig(filteredGroupByGroups, groupBy.Config)
					if groupGroupBy != nil {
						newGroupBys = append(newGroupBys, groupGroupBy)
						groupGroupByAdded = true
					}
				}
			}
		}
		chartDataConfig.GroupBy = newGroupBys
	} else {
		// If no users or groups remain after ACL filtering, remove all user/group groupBys
		newGroupBys := []*dashboardpb.ChartGroupBy{}
		for _, groupBy := range chartDataConfig.GetGroupBy() {
			if !a.isUserGroupBy(groupBy) && !a.isGroupGroupBy(groupBy) {
				newGroupBys = append(newGroupBys, groupBy)
			}
		}
		chartDataConfig.GroupBy = newGroupBys
	}

	return nil
}

// Helper functions to identify and create user/group filters and groupBys

func (a AnalyticsServiceImpl) isUserOrGroupFilter(filter *dashboardpb.ChartDataFilter) bool {
	if filter.GetValueSet() == nil || filter.GetValueSet().GetSingleValues() == nil {
		return false
	}

	for _, value := range filter.GetValueSet().GetSingleValues().GetAttributeValues() {
		if value.GetUserNameValue() != "" || value.GetGroupNameValue() != "" {
			return true
		}
	}
	return false
}

func (a AnalyticsServiceImpl) isUserGroupBy(groupBy *dashboardpb.ChartGroupBy) bool {
	if groupBy.GetByProvidedValue() == nil || groupBy.GetByProvidedValue().GetValueSet() == nil ||
		groupBy.GetByProvidedValue().GetValueSet().GetSingleValues() == nil {
		return false
	}

	for _, value := range groupBy.GetByProvidedValue().GetValueSet().GetSingleValues().GetAttributeValues() {
		if value.GetUserNameValue() != "" {
			return true
		}
	}
	return false
}

func (a AnalyticsServiceImpl) isGroupGroupBy(groupBy *dashboardpb.ChartGroupBy) bool {
	if groupBy.GetByProvidedValue() == nil || groupBy.GetByProvidedValue().GetValueSet() == nil ||
		groupBy.GetByProvidedValue().GetValueSet().GetSingleValues() == nil {
		return false
	}

	for _, value := range groupBy.GetByProvidedValue().GetValueSet().GetSingleValues().GetAttributeValues() {
		if value.GetGroupNameValue() != "" {
			return true
		}
	}
	return false
}

func (a AnalyticsServiceImpl) createUserFilter(users []*userpb.User) *dashboardpb.ChartDataFilter {
	if len(users) == 0 {
		return nil
	}

	attributeValues := make([]*dashboardpb.ChartAttributeValue, 0, len(users))
	for _, user := range users {
		attributeValues = append(attributeValues, &dashboardpb.ChartAttributeValue{
			Value: &dashboardpb.ChartAttributeValue_UserNameValue{
				UserNameValue: user.Name,
			},
		})
	}

	return &dashboardpb.ChartDataFilter{
		Operator: dashboardpb.ChartDataFilter_IN,
		ValueSet: &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: attributeValues,
				},
			},
		},
	}
}

func (a AnalyticsServiceImpl) createGroupByUserWithConfig(users []*userpb.User, config *dashboardpb.ChartGroupBy_Config) *dashboardpb.ChartGroupBy {
	if len(users) == 0 {
		return nil
	}

	attributeValues := make([]*dashboardpb.ChartAttributeValue, 0, len(users))
	for _, user := range users {
		attributeValues = append(attributeValues, &dashboardpb.ChartAttributeValue{
			Value: &dashboardpb.ChartAttributeValue_UserNameValue{
				UserNameValue: user.Name,
			},
		})
	}

	return &dashboardpb.ChartGroupBy{
		Config: config,
		GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: attributeValues,
						},
					},
				},
			},
		},
	}
}

func (a AnalyticsServiceImpl) createGroupByGroupWithConfig(groups []*userpb.Group, config *dashboardpb.ChartGroupBy_Config) *dashboardpb.ChartGroupBy {
	if len(groups) == 0 {
		return nil
	}

	attributeValues := make([]*dashboardpb.ChartAttributeValue, 0, len(groups))
	for _, group := range groups {
		attributeValues = append(attributeValues, &dashboardpb.ChartAttributeValue{
			Value: &dashboardpb.ChartAttributeValue_GroupNameValue{
				GroupNameValue: group.Name,
			},
		})
	}

	return &dashboardpb.ChartGroupBy{
		Config: config,
		GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: attributeValues,
						},
					},
				},
			},
		},
	}
}
