package analyticsimpl

import (
	"context"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveSmartComposeStats(
	ctx context.Context, req *analyticspb.RetrieveSmartComposeStatsRequest,
) (resp *analyticspb.RetrieveSmartComposeStatsResponse, err error) {
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

	var (
		users                             []*userpb.User
		groupsToAggregate                 []*userpb.Group
		userNameToGroupNamesMap           map[string][]string
		hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
		includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
	)

	listAgentOnly := req.GetFilterToAgentsOnly() // SmartCompose stats track agent activities
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
		return &analyticspb.RetrieveSmartComposeStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveSmartComposeStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveSmartComposeStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveSmartComposeStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveSmartComposeStatsForGroups(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readSmartComposeStatsFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveSmartComposeStatsForGroups(
	ctx context.Context,
	req *analyticspb.RetrieveSmartComposeStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSmartComposeStatsResponse, error) {
	// Retrieve per-agent SmartCompose stats for ALL users from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveSmartComposeStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.readSmartComposeStatsFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupSmartComposeStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupsSmartComposeStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupSmartComposeStatsResponse(
	perUserResp *analyticspb.RetrieveSmartComposeStatsResponse,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSmartComposeStatsResponse, error) {
	// Build the userName to user and groupName to group maps.
	userNameToUserMap := make(map[string]*userpb.User)
	for _, user := range users {
		userNameToUserMap[user.Name] = user
	}
	groupNameToGroupMap := make(map[string]*userpb.Group)
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	// Append the groups into the attribute of per-user resp.
	smartComposeStatsResultsPerUserPerGroup := []*analyticspb.SmartComposeStatsResult{}
	for _, smartComposeStatsResult := range perUserResp.SmartComposeStatsResults {
		if len(smartComposeStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := smartComposeStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					smartComposeStatsResult.Attribute.Groups = append(smartComposeStatsResult.Attribute.Groups, group)
				}
			}
			smartComposeStatsResultsPerUserPerGroup = append(smartComposeStatsResultsPerUserPerGroup, smartComposeStatsResult)
		}
	}
	return &analyticspb.RetrieveSmartComposeStatsResponse{
		SmartComposeStatsResults:                        sortResults(smartComposeStatsResultsPerUserPerGroup),
		TotalConversationCount:                          perUserResp.TotalConversationCount,
		TotalSmartComposeUsedCount:                      perUserResp.TotalSmartComposeUsedCount,
		AverageSmartComposeUsedCountPerConversation:     perUserResp.AverageSmartComposeUsedCountPerConversation,
		AverageSmartComposeUsedCountPerFrequencyPerUser: perUserResp.AverageSmartComposeUsedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupsSmartComposeStatsResponse(
	perUserResp *analyticspb.RetrieveSmartComposeStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSmartComposeStatsResponse, error) {
	// Initialize the per-group SmartCompose stats result.
	smartComposeStatsResultsByGroup := make(map[string]*analyticspb.SmartComposeStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		smartComposeStatsResultsByGroup[group.Name] = &analyticspb.SmartComposeStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, smartComposeStatsResults := range perUserResp.SmartComposeStatsResults {
		if len(smartComposeStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := smartComposeStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := smartComposeStatsResultsByGroup[groupName]; exists {
					smartComposeStatsResultsByGroup[groupName].TotalSmartComposeUsedCount += smartComposeStatsResults.TotalSmartComposeUsedCount
					smartComposeStatsResultsByGroup[groupName].TotalConversationCount += smartComposeStatsResults.TotalConversationCount
					smartComposeStatsResultsByGroup[groupName].SmartComposeStats = append(smartComposeStatsResultsByGroup[groupName].SmartComposeStats, smartComposeStatsResults.SmartComposeStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	smartComposeStatsResultsForAllGroups := []*analyticspb.SmartComposeStatsResult{}
	for _, smartComposeStatsResults := range smartComposeStatsResultsByGroup {
		if smartComposeStatsResults.TotalConversationCount == 0 || len(smartComposeStatsResults.SmartComposeStats) == 0 {
			continue
		}
		smartComposeStatsResults.AverageSmartComposeUsedCountPerConversation = safeDivideFloat(smartComposeStatsResults.TotalSmartComposeUsedCount, smartComposeStatsResults.TotalConversationCount)
		userFrequencyCount := len(smartComposeStatsResults.SmartComposeStats)
		smartComposeStatsResults.AverageSmartComposeUsedCountPerFrequencyPerUser = safeDivideFloat(smartComposeStatsResults.TotalSmartComposeUsedCount, int32(userFrequencyCount))
		smartComposeStatsResults.SmartComposeStats = convertSmartComposeStatsPerUserToPerGroup(smartComposeStatsResults.SmartComposeStats)
		smartComposeStatsResultsForAllGroups = append(smartComposeStatsResultsForAllGroups, smartComposeStatsResults)
	}

	return &analyticspb.RetrieveSmartComposeStatsResponse{
		SmartComposeStatsResults:                        sortResults(smartComposeStatsResultsForAllGroups),
		TotalConversationCount:                          perUserResp.TotalConversationCount,
		TotalSmartComposeUsedCount:                      perUserResp.TotalSmartComposeUsedCount,
		AverageSmartComposeUsedCountPerConversation:     perUserResp.AverageSmartComposeUsedCountPerConversation,
		AverageSmartComposeUsedCountPerFrequencyPerUser: perUserResp.AverageSmartComposeUsedCountPerFrequencyPerUser,
	}, nil
}

func convertSmartComposeStatsPerUserToPerGroup(smartComposeStatsPerUser []*analyticspb.SmartComposeStats) []*analyticspb.SmartComposeStats {
	// Initial and append per-timestamp stats per group.
	smartComposeStatsByTimestamp := make(map[int64]*analyticspb.SmartComposeStats)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, smartComposeStats := range smartComposeStatsPerUser {
		key := smartComposeStats.TimeRange.StartTimestamp.Seconds
		if _, exists := smartComposeStatsByTimestamp[key]; !exists {
			smartComposeStatsByTimestamp[key] = &analyticspb.SmartComposeStats{
				TimeRange: smartComposeStats.TimeRange,
			}
		}

		smartComposeStatsByTimestamp[key].TotalSmartComposeUsedCount += smartComposeStats.TotalSmartComposeUsedCount
		smartComposeStatsByTimestamp[key].TotalConversationCount += smartComposeStats.TotalConversationCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-timestamp stats.
	smartComposeStatsPerGroup := []*analyticspb.SmartComposeStats{}
	for key, smartComposeStats := range smartComposeStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		smartComposeStatsPerGroup = append(smartComposeStatsPerGroup, &analyticspb.SmartComposeStats{
			TimeRange:                                       smartComposeStats.TimeRange,
			TotalSmartComposeUsedCount:                      smartComposeStats.TotalSmartComposeUsedCount,
			TotalConversationCount:                          smartComposeStats.TotalConversationCount,
			AverageSmartComposeUsedCountPerConversation:     safeDivideFloat(smartComposeStats.TotalSmartComposeUsedCount, smartComposeStats.TotalConversationCount),
			AverageSmartComposeUsedCountPerFrequencyPerUser: safeDivideFloat(smartComposeStats.TotalSmartComposeUsedCount, userFrequencyCount),
		})
	}
	sort.Slice(smartComposeStatsPerGroup, func(i, j int) bool {
		return smartComposeStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < smartComposeStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return smartComposeStatsPerGroup
}
