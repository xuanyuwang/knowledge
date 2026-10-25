package analyticsimpl

import (
	"context"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveCustomerSnapshotStats(
	ctx context.Context, req *analyticspb.RetrieveCustomerSnapshotStatsRequest,
) (resp *analyticspb.RetrieveCustomerSnapshotStatsResponse, err error) {
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)
	err = shared.ValidateGroupByAttributes(req.GroupByAttributeTypes, req.Frequency)
	if err != nil {
		return nil, err
	}

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	_, err = postgres.ParseArgs(&req.Frequency, req.Metadata)
	if err != nil {
		return nil, err
	}

	var (
		users                             []*userpb.User
		groupsToAggregate                 []*userpb.Group
		userNameToGroupNamesMap           map[string][]string
		hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
		includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
	)

	listAgentOnly := req.GetFilterToAgentsOnly()
	excludeDeactivatedUsers := req.FilterByAttribute != nil && req.FilterByAttribute.GetExcludeDeactivatedUsers()

	result, err := ParseUserFilterForAnalytics(
		ctx,
		a.logger,
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		parent.CustomerID,
		parent.ProfileID,
		req.FilterByAttribute.GetUsers(),
		req.FilterByAttribute.GetGroups(),
		hasAgentAsGroupByKey,
		includeDirectGroupMembershipsOnly,
		listAgentOnly,
		req.GetIncludePeerUserStats(),
		excludeDeactivatedUsers,
	)
	if err != nil {
		return nil, err
	}

	userNameToGroupNamesMap = result.UserNameToGroupNamesMap
	groupsToAggregate = result.GroupsToAggregate
	users = result.FinalUsers

	if !shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) &&
		!shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		users = []*userpb.User{}
		groupsToAggregate = []*userpb.Group{}
		userNameToGroupNamesMap = map[string][]string{}
	}

	if shouldEarlyReturn := ApplyUserFilterFromResult(result, &req.FilterByAttribute.Users, &req.FilterByAttribute.Groups, a.enableExtTableForUserFilter); shouldEarlyReturn {
		return &analyticspb.RetrieveCustomerSnapshotStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveCustomerSnapshotStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveCustomerSnapshotStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveCustomerSnapshotStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveCustomerSnapshotStatsForGroups(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readCustomerSnapshotStatsFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveCustomerSnapshotStatsForGroups(
	ctx context.Context,
	req *analyticspb.RetrieveCustomerSnapshotStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveCustomerSnapshotStatsResponse, error) {
	// Retrieve per-agent customer snapshot stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveCustomerSnapshotStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.readCustomerSnapshotStatsFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupCustomerSnapshotStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupCustomerSnapshotStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupCustomerSnapshotStatsResponse(
	perUserResp *analyticspb.RetrieveCustomerSnapshotStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveCustomerSnapshotStatsResponse, error) {
	userNameToUserMap := make(map[string]*userpb.User)
	for _, user := range users {
		userNameToUserMap[user.Name] = user
	}
	groupNameToGroupMap := make(map[string]*userpb.Group)
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	resultsPerUserPerGroup := []*analyticspb.CustomerSnapshotStatsResult{}
	for _, result := range perUserResp.CustomerSnapshotStatsResults {
		if len(result.Attribute.Users) == 0 {
			continue
		}
		userName := result.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					result.Attribute.Groups = append(result.Attribute.Groups, group)
				}
			}
			resultsPerUserPerGroup = append(resultsPerUserPerGroup, result)
		}
	}
	return &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults:          sortResults(resultsPerUserPerGroup),
		TotalDisplayTime:                      perUserResp.TotalDisplayTime,
		AverageDisplayTime:                    perUserResp.AverageDisplayTime,
		AverageDisplayTimePerFrequencyPerUser: perUserResp.AverageDisplayTimePerFrequencyPerUser,
		TotalConversationsWithSnapshot:        perUserResp.TotalConversationsWithSnapshot,
	}, nil
}

func convertRowsPerUserToPerGroupCustomerSnapshotStatsResponse(
	perUserResp *analyticspb.RetrieveCustomerSnapshotStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveCustomerSnapshotStatsResponse, error) {
	resultsByGroup := make(map[string]*analyticspb.CustomerSnapshotStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		resultsByGroup[group.GetName()] = &analyticspb.CustomerSnapshotStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	for _, perUserResult := range perUserResp.CustomerSnapshotStatsResults {
		if len(perUserResult.Attribute.Users) == 0 {
			continue
		}
		userName := perUserResult.Attribute.Users[0].GetName()
		groupNames, exists := userNameToGroupNamesMap[userName]
		if !exists {
			continue
		}
		for _, groupName := range groupNames {
			groupResult, exists := resultsByGroup[groupName]
			if !exists {
				continue
			}
			groupResult.TotalDisplayTime += perUserResult.TotalDisplayTime
			groupResult.TotalConversationsWithSnapshot += perUserResult.TotalConversationsWithSnapshot
			groupResult.CustomerSnapshotStats = append(groupResult.CustomerSnapshotStats, perUserResult.CustomerSnapshotStats...)
		}
	}

	resultsForAllGroups := []*analyticspb.CustomerSnapshotStatsResult{}
	for _, groupResult := range resultsByGroup {
		if len(groupResult.CustomerSnapshotStats) == 0 {
			continue
		}
		groupResult.AverageDisplayTime = safeDivideFloat64(groupResult.TotalDisplayTime, int64(groupResult.TotalConversationsWithSnapshot))
		// Per-group per-frequency-per-user: total display time over the
		// user x bucket combinations in this group (the per-user bucket
		// entries accumulated above, before they are merged per timestamp).
		groupResult.AverageDisplayTimePerFrequencyPerUser = safeDivideFloat64(groupResult.TotalDisplayTime, int64(len(groupResult.CustomerSnapshotStats)))
		groupResult.CustomerSnapshotStats = convertPerUserToPerGroupCustomerSnapshotStats(groupResult.CustomerSnapshotStats)
		resultsForAllGroups = append(resultsForAllGroups, groupResult)
	}

	return &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults:          sortResults(resultsForAllGroups),
		TotalDisplayTime:                      perUserResp.TotalDisplayTime,
		AverageDisplayTime:                    perUserResp.AverageDisplayTime,
		AverageDisplayTimePerFrequencyPerUser: perUserResp.AverageDisplayTimePerFrequencyPerUser,
		TotalConversationsWithSnapshot:        perUserResp.TotalConversationsWithSnapshot,
	}, nil
}

func convertPerUserToPerGroupCustomerSnapshotStats(perUserStats []*analyticspb.CustomerSnapshotStats) []*analyticspb.CustomerSnapshotStats {
	statsByTimestamp := make(map[int64]*analyticspb.CustomerSnapshotStats)
	userCountByTimestamp := make(map[int64]int32)
	for _, stat := range perUserStats {
		key := stat.TimeRange.StartTimestamp.Seconds
		if _, exists := statsByTimestamp[key]; !exists {
			statsByTimestamp[key] = &analyticspb.CustomerSnapshotStats{
				TimeRange: stat.TimeRange,
			}
		}
		statsByTimestamp[key].TotalDisplayTime += stat.TotalDisplayTime
		statsByTimestamp[key].TotalConversationsWithSnapshot += stat.TotalConversationsWithSnapshot
		userCountByTimestamp[key]++
	}

	statsPerGroup := []*analyticspb.CustomerSnapshotStats{}
	for key, stat := range statsByTimestamp {
		statsPerGroup = append(statsPerGroup, &analyticspb.CustomerSnapshotStats{
			TimeRange:                             stat.TimeRange,
			TotalDisplayTime:                      stat.TotalDisplayTime,
			AverageDisplayTime:                    safeDivideFloat64(stat.TotalDisplayTime, int64(stat.TotalConversationsWithSnapshot)),
			AverageDisplayTimePerFrequencyPerUser: safeDivideFloat64(stat.TotalDisplayTime, int64(userCountByTimestamp[key])),
			TotalConversationsWithSnapshot:        stat.TotalConversationsWithSnapshot,
		})
	}
	sort.Slice(statsPerGroup, func(i, j int) bool {
		return statsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < statsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return statsPerGroup
}
