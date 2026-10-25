package analyticsimpl

import (
	"context"

	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/util/errgroup"

	"github.com/cresta/go-servers/insights-server/internal/dashboarding"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

const maxConcurrentGroupMemberLookups = 10

// scopeOuterGroupByToUserFilters drops the outer Top/Bottom-N team and user values that share no
// member with every IN user/team filter. The outer groupby is a filters agg with one bucket per
// provided value, so without scoping, out-of-filter values return 0 and pad the Top-N. For example,
// a Cork team filter over a list of every manager team shows the Cork managers, then managers from
// other sites at 0.
//
// outerGroupMemberIDs holds the member user IDs of every resolved outer team, keyed by group name,
// for reuse when building the groupby aggregation.
func (a AnalyticsServiceImpl) scopeOuterGroupByToUserFilters(
	ctx context.Context,
	parent *shared.CustomerProfile,
	chartDataConfig *dashboardpb.ChartDataConfig,
) (outerGroupMemberIDs map[string][]string, noValuesLeft bool, err error) {
	groupBys := chartDataConfig.GetGroupBy()
	if !needManualSortFirstGroupby(groupBys) {
		return nil, false, nil
	}
	outerProvidedValue := groupBys[0].GetByProvidedValue()
	outerValueSet := outerProvidedValue.GetValueSet()
	if len(dashboarding.MapGroupsValueSetToNames(outerValueSet)) == 0 && len(dashboarding.MapUsersValueSetToNames(outerValueSet)) == 0 {
		return nil, false, nil
	}
	filterUserIDSets, err := a.userIDSetsFromUserFilters(ctx, parent, chartDataConfig.GetFilters())
	if err != nil {
		return nil, false, err
	}
	if len(filterUserIDSets) == 0 {
		return nil, false, nil
	}

	outerGroupNames := dashboarding.MapGroupsValueSetToNames(outerValueSet)
	outerMemberIDLists, err := a.groupMemberUserIDsConcurrently(ctx, parent, fn.Map(outerGroupNames, func(groupName string) []string { return []string{groupName} }))
	if err != nil {
		return nil, false, err
	}
	outerGroupMemberIDs = map[string][]string{}
	for i, groupName := range outerGroupNames {
		outerGroupMemberIDs[groupName] = outerMemberIDLists[i]
	}

	keptValues := []*dashboardpb.ChartAttributeValue{}
	for _, value := range outerValueSet.GetSingleValues().GetAttributeValues() {
		var memberIDs []string
		switch {
		case value.GetGroupNameValue() != "":
			memberIDs = outerGroupMemberIDs[value.GetGroupNameValue()]
		case value.GetUserNameValue() != "":
			userName, err := userpb.ParseUserName(value.GetUserNameValue())
			if err != nil {
				return nil, false, err
			}
			memberIDs = []string{userName.UserID}
		default:
			keptValues = append(keptValues, value)
			continue
		}
		if !overlapsEverySet(memberIDs, filterUserIDSets) {
			continue
		}
		keptValues = append(keptValues, value)
	}

	outerProvidedValue.ValueSet = &dashboardpb.ChartValueSet{
		Values: &dashboardpb.ChartValueSet_SingleValues{
			SingleValues: &dashboardpb.ChartValueSet_Single{
				AttributeValues: keptValues,
			},
		},
	}
	return outerGroupMemberIDs, len(keptValues) == 0, nil
}

// userIDSetsFromUserFilters returns one user ID set per IN filter holding user or team values.
// Filters are ANDed and the values of one filter are ORed, matching the ES query they produce.
func (a AnalyticsServiceImpl) userIDSetsFromUserFilters(
	ctx context.Context,
	parent *shared.CustomerProfile,
	filters []*dashboardpb.ChartDataFilter,
) ([]set.Set[string], error) {
	var userIDSets []set.Set[string]
	var filterGroupNameLists [][]string
	for _, filter := range filters {
		if filter.GetOperator() != dashboardpb.ChartDataFilter_IN {
			continue
		}
		groupNames := dashboarding.MapGroupsValueSetToNames(filter.GetValueSet())
		userNames := dashboarding.MapUsersValueSetToNames(filter.GetValueSet())
		if len(groupNames) == 0 && len(userNames) == 0 {
			continue
		}

		userIDs := set.New[string]()
		for _, userNameValue := range userNames {
			userName, err := userpb.ParseUserName(userNameValue)
			if err != nil {
				return nil, err
			}
			userIDs.Add(userName.UserID)
		}
		userIDSets = append(userIDSets, userIDs)
		filterGroupNameLists = append(filterGroupNameLists, groupNames)
	}

	// Filters without groups resolve to no member, so no call is made for them.
	filterMemberIDLists, err := a.groupMemberUserIDsConcurrently(ctx, parent, filterGroupNameLists)
	if err != nil {
		return nil, err
	}
	for i, memberIDs := range filterMemberIDLists {
		userIDSets[i].AddAll(memberIDs...)
	}
	return userIDSets, nil
}

// groupMemberUserIDsConcurrently returns the members of each entry of groupNameLists, in order, with
// one user service call per non-empty entry.
func (a AnalyticsServiceImpl) groupMemberUserIDsConcurrently(
	ctx context.Context,
	parent *shared.CustomerProfile,
	groupNameLists [][]string,
) ([][]string, error) {
	memberIDLists := make([][]string, len(groupNameLists))
	eg, egCtx := errgroup.WithContext(ctx, errgroup.WithLimit(maxConcurrentGroupMemberLookups))
	for i, groupNames := range groupNameLists {
		if len(groupNames) == 0 {
			continue
		}
		eg.Go(func() error {
			memberIDs, err := a.groupMemberUserIDs(egCtx, parent.CustomerID, parent.ProfileID, groupNames)
			if err != nil {
				return err
			}
			memberIDLists[i] = memberIDs
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return memberIDLists, nil
}

func overlapsEverySet(userIDs []string, userIDSets []set.Set[string]) bool {
	for _, userIDSet := range userIDSets {
		if !userIDSet.HasAny(userIDs...) {
			return false
		}
	}
	return true
}
