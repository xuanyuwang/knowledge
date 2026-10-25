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
func (a AnalyticsServiceImpl) RetrieveKnowledgeBaseStats(
	ctx context.Context, req *analyticspb.RetrieveKnowledgeBaseStatsRequest,
) (resp *analyticspb.RetrieveKnowledgeBaseStatsResponse, err error) {
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
	listAgentOnly := req.GetFilterToAgentsOnly() // Knowledge base stats track agent activities
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
		false, // includePeerUserStats - not used for knowledge base stats
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
		return &analyticspb.RetrieveKnowledgeBaseStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveKnowledgeBaseStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveKnowledgeBaseStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveKnowledgeBaseStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveKnowledgeBaseStatsForGroups(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readKnowledgeBaseStatsFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveKnowledgeBaseStatsForGroups(
	ctx context.Context,
	req *analyticspb.RetrieveKnowledgeBaseStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveKnowledgeBaseStatsResponse, error) {
	// Retrieve per-agent KnowledgeBase stats for ALL users from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveKnowledgeBaseStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.readKnowledgeBaseStatsFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupKnowledgeBaseStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupsKnowledgeBaseStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupKnowledgeBaseStatsResponse(
	perUserResp *analyticspb.RetrieveKnowledgeBaseStatsResponse,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveKnowledgeBaseStatsResponse, error) {
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
	knowledgeBaseStatsResultsPerUserPerGroup := []*analyticspb.KnowledgeBaseStatsResult{}
	for _, knowledgeBaseStatsResult := range perUserResp.KnowledgeBaseStatsResults {
		if len(knowledgeBaseStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := knowledgeBaseStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					knowledgeBaseStatsResult.Attribute.Groups = append(knowledgeBaseStatsResult.Attribute.Groups, group)
				}
			}
			knowledgeBaseStatsResultsPerUserPerGroup = append(knowledgeBaseStatsResultsPerUserPerGroup, knowledgeBaseStatsResult)
		}
	}
	return &analyticspb.RetrieveKnowledgeBaseStatsResponse{
		KnowledgeBaseStatsResults:               sortResults(knowledgeBaseStatsResultsPerUserPerGroup),
		TotalConversationCount:                  perUserResp.TotalConversationCount,
		TotalKbSearchCount:                      perUserResp.TotalKbSearchCount,
		AverageKbSearchCountPerConversation:     perUserResp.AverageKbSearchCountPerConversation,
		AverageKbSearchCountPerFrequencyPerUser: perUserResp.AverageKbSearchCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupsKnowledgeBaseStatsResponse(
	perUserResp *analyticspb.RetrieveKnowledgeBaseStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveKnowledgeBaseStatsResponse, error) {
	// Initialize the per-group KnowledgeBase stats result.
	knowledgeBaseStatsResultsByGroup := make(map[string]*analyticspb.KnowledgeBaseStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		knowledgeBaseStatsResultsByGroup[group.Name] = &analyticspb.KnowledgeBaseStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, knowledgeBaseStatsResults := range perUserResp.KnowledgeBaseStatsResults {
		if len(knowledgeBaseStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := knowledgeBaseStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := knowledgeBaseStatsResultsByGroup[groupName]; exists {
					knowledgeBaseStatsResultsByGroup[groupName].TotalKbSearchCount += knowledgeBaseStatsResults.TotalKbSearchCount
					knowledgeBaseStatsResultsByGroup[groupName].TotalConversationCount += knowledgeBaseStatsResults.TotalConversationCount
					knowledgeBaseStatsResultsByGroup[groupName].KbSearchStats = append(knowledgeBaseStatsResultsByGroup[groupName].KbSearchStats, knowledgeBaseStatsResults.KbSearchStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	knowledgeBaseStatsResultsForAllGroups := []*analyticspb.KnowledgeBaseStatsResult{}
	for _, knowledgeBaseStatsResults := range knowledgeBaseStatsResultsByGroup {
		if knowledgeBaseStatsResults.TotalConversationCount == 0 || len(knowledgeBaseStatsResults.KbSearchStats) == 0 {
			continue
		}
		knowledgeBaseStatsResults.AverageKbSearchCountPerConversation = safeDivideFloat(knowledgeBaseStatsResults.TotalKbSearchCount, knowledgeBaseStatsResults.TotalConversationCount)
		userFrequencyCount := len(knowledgeBaseStatsResults.KbSearchStats)
		knowledgeBaseStatsResults.AverageKbSearchCountPerFrequencyPerUser = safeDivideFloat(knowledgeBaseStatsResults.TotalKbSearchCount, int32(userFrequencyCount))
		knowledgeBaseStatsResults.KbSearchStats = convertKbSearchStatsPerUserToPerGroup(knowledgeBaseStatsResults.KbSearchStats)
		knowledgeBaseStatsResultsForAllGroups = append(knowledgeBaseStatsResultsForAllGroups, knowledgeBaseStatsResults)
	}

	return &analyticspb.RetrieveKnowledgeBaseStatsResponse{
		KnowledgeBaseStatsResults:               sortResults(knowledgeBaseStatsResultsForAllGroups),
		TotalConversationCount:                  perUserResp.TotalConversationCount,
		TotalKbSearchCount:                      perUserResp.TotalKbSearchCount,
		AverageKbSearchCountPerConversation:     perUserResp.AverageKbSearchCountPerConversation,
		AverageKbSearchCountPerFrequencyPerUser: perUserResp.AverageKbSearchCountPerFrequencyPerUser,
	}, nil
}

func convertKbSearchStatsPerUserToPerGroup(knowledgeBaseStatsPerUser []*analyticspb.KBSearchStats) []*analyticspb.KBSearchStats {
	// Initial and append per-timestamp stats per group.
	knowledgeBaseStatsByTimestamp := make(map[int64]*analyticspb.KBSearchStats)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, knowledgeBaseStats := range knowledgeBaseStatsPerUser {
		key := knowledgeBaseStats.TimeRange.StartTimestamp.Seconds
		if _, exists := knowledgeBaseStatsByTimestamp[key]; !exists {
			knowledgeBaseStatsByTimestamp[key] = &analyticspb.KBSearchStats{
				TimeRange: knowledgeBaseStats.TimeRange,
			}
		}

		knowledgeBaseStatsByTimestamp[key].TotalKbSearchCount += knowledgeBaseStats.TotalKbSearchCount
		knowledgeBaseStatsByTimestamp[key].TotalConversationCount += knowledgeBaseStats.TotalConversationCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-timestamp stats.
	knowledgeBaseStatsPerGroup := []*analyticspb.KBSearchStats{}
	for key, knowledgeBaseStats := range knowledgeBaseStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		knowledgeBaseStatsPerGroup = append(knowledgeBaseStatsPerGroup, &analyticspb.KBSearchStats{
			TimeRange:                               knowledgeBaseStats.TimeRange,
			TotalKbSearchCount:                      knowledgeBaseStats.TotalKbSearchCount,
			TotalConversationCount:                  knowledgeBaseStats.TotalConversationCount,
			AverageKbSearchCountPerConversation:     safeDivideFloat(knowledgeBaseStats.TotalKbSearchCount, knowledgeBaseStats.TotalConversationCount),
			AverageKbSearchCountPerFrequencyPerUser: safeDivideFloat(knowledgeBaseStats.TotalKbSearchCount, userFrequencyCount),
		})
	}
	sort.Slice(knowledgeBaseStatsPerGroup, func(i, j int) bool {
		return knowledgeBaseStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < knowledgeBaseStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return knowledgeBaseStatsPerGroup
}
