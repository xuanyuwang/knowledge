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
func (a AnalyticsServiceImpl) RetrieveNoteTakingStats(
	ctx context.Context, req *analyticspb.RetrieveNoteTakingStatsRequest,
) (resp *analyticspb.RetrieveNoteTakingStatsResponse, err error) {
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

	// New implementation using ParseUserFilterForAnalytics
	listAgentOnly := req.GetFilterToAgentsOnly() // Note-taking stats track agent activities
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

	// Extract values from result
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
		return &analyticspb.RetrieveNoteTakingStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveNoteTakingStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveNoteTakingStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveNoteTakingStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveNoteTakingStatsForGroups(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readNoteTakingStatsFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveNoteTakingStatsForGroups(
	ctx context.Context,
	req *analyticspb.RetrieveNoteTakingStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveNoteTakingStatsResponse, error) {
	// Retrieve per-agent NoteTaking stats for ALL users from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveNoteTakingStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.readNoteTakingStatsFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupNoteTakingStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupsNoteTakingStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupNoteTakingStatsResponse(
	perUserResp *analyticspb.RetrieveNoteTakingStatsResponse,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveNoteTakingStatsResponse, error) {
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
	noteTakingStatsResultsPerUserPerGroup := []*analyticspb.NoteTakingStatsResult{}
	for _, noteTakingStatsResult := range perUserResp.NoteTakingStatsResults {
		if len(noteTakingStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := noteTakingStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					noteTakingStatsResult.Attribute.Groups = append(noteTakingStatsResult.Attribute.Groups, group)
				}
			}
			noteTakingStatsResultsPerUserPerGroup = append(noteTakingStatsResultsPerUserPerGroup, noteTakingStatsResult)
		}
	}
	return &analyticspb.RetrieveNoteTakingStatsResponse{
		NoteTakingStatsResults:                        sortResults(noteTakingStatsResultsPerUserPerGroup),
		TotalConversationCount:                        perUserResp.TotalConversationCount,
		TotalNoteTakingUsedCount:                      perUserResp.TotalNoteTakingUsedCount,
		AverageNoteTakingUsedCountPerConversation:     perUserResp.AverageNoteTakingUsedCountPerConversation,
		AverageNoteTakingUsedCountPerFrequencyPerUser: perUserResp.AverageNoteTakingUsedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupsNoteTakingStatsResponse(
	perUserResp *analyticspb.RetrieveNoteTakingStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveNoteTakingStatsResponse, error) {
	// Initialize the per-group NoteTaking stats result.
	noteTakingStatsResultsByGroup := make(map[string]*analyticspb.NoteTakingStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		noteTakingStatsResultsByGroup[group.Name] = &analyticspb.NoteTakingStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, noteTakingStatsResults := range perUserResp.NoteTakingStatsResults {
		if len(noteTakingStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := noteTakingStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := noteTakingStatsResultsByGroup[groupName]; exists {
					noteTakingStatsResultsByGroup[groupName].TotalNoteTakingUsedCount += noteTakingStatsResults.TotalNoteTakingUsedCount
					noteTakingStatsResultsByGroup[groupName].TotalConversationCount += noteTakingStatsResults.TotalConversationCount
					noteTakingStatsResultsByGroup[groupName].NoteTakingStats = append(noteTakingStatsResultsByGroup[groupName].NoteTakingStats, noteTakingStatsResults.NoteTakingStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	noteTakingStatsResultsForAllGroups := []*analyticspb.NoteTakingStatsResult{}
	for _, noteTakingStatsResults := range noteTakingStatsResultsByGroup {
		if noteTakingStatsResults.TotalConversationCount == 0 || len(noteTakingStatsResults.NoteTakingStats) == 0 {
			continue
		}
		noteTakingStatsResults.AverageNoteTakingUsedCountPerConversation = safeDivideFloat(noteTakingStatsResults.TotalNoteTakingUsedCount, noteTakingStatsResults.TotalConversationCount)
		userFrequencyCount := len(noteTakingStatsResults.NoteTakingStats)
		noteTakingStatsResults.AverageNoteTakingUsedCountPerFrequencyPerUser = safeDivideFloat(noteTakingStatsResults.TotalNoteTakingUsedCount, int32(userFrequencyCount))
		noteTakingStatsResults.NoteTakingStats = convertNoteTakingStatsPerUserToPerGroup(noteTakingStatsResults.NoteTakingStats)
		noteTakingStatsResultsForAllGroups = append(noteTakingStatsResultsForAllGroups, noteTakingStatsResults)
	}

	return &analyticspb.RetrieveNoteTakingStatsResponse{
		NoteTakingStatsResults:                        sortResults(noteTakingStatsResultsForAllGroups),
		TotalConversationCount:                        perUserResp.TotalConversationCount,
		TotalNoteTakingUsedCount:                      perUserResp.TotalNoteTakingUsedCount,
		AverageNoteTakingUsedCountPerConversation:     perUserResp.AverageNoteTakingUsedCountPerConversation,
		AverageNoteTakingUsedCountPerFrequencyPerUser: perUserResp.AverageNoteTakingUsedCountPerFrequencyPerUser,
	}, nil
}

func convertNoteTakingStatsPerUserToPerGroup(noteTakingStatsPerUser []*analyticspb.NoteTakingStats) []*analyticspb.NoteTakingStats {
	// Initial and append per-timestamp stats per group.
	noteTakingStatsByTimestamp := make(map[int64]*analyticspb.NoteTakingStats)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, noteTakingStats := range noteTakingStatsPerUser {
		key := noteTakingStats.TimeRange.StartTimestamp.Seconds
		if _, exists := noteTakingStatsByTimestamp[key]; !exists {
			noteTakingStatsByTimestamp[key] = &analyticspb.NoteTakingStats{
				TimeRange: noteTakingStats.TimeRange,
			}
		}

		noteTakingStatsByTimestamp[key].TotalNoteTakingUsedCount += noteTakingStats.TotalNoteTakingUsedCount
		noteTakingStatsByTimestamp[key].TotalConversationCount += noteTakingStats.TotalConversationCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-timestamp stats.
	noteTakingStatsPerGroup := []*analyticspb.NoteTakingStats{}
	for key, noteTakingStats := range noteTakingStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		noteTakingStatsPerGroup = append(noteTakingStatsPerGroup, &analyticspb.NoteTakingStats{
			TimeRange:                                 noteTakingStats.TimeRange,
			TotalNoteTakingUsedCount:                  noteTakingStats.TotalNoteTakingUsedCount,
			TotalConversationCount:                    noteTakingStats.TotalConversationCount,
			AverageNoteTakingUsedCountPerConversation: safeDivideFloat(noteTakingStats.TotalNoteTakingUsedCount, noteTakingStats.TotalConversationCount),
			AverageNoteTakingUsedCountPerUser:         safeDivideFloat(noteTakingStats.TotalNoteTakingUsedCount, userFrequencyCount),
		})
	}
	sort.Slice(noteTakingStatsPerGroup, func(i, j int) bool {
		return noteTakingStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < noteTakingStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return noteTakingStatsPerGroup
}
