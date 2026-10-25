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
func (a AnalyticsServiceImpl) RetrieveGuidedWorkflowStats(
	ctx context.Context, req *analyticspb.RetrieveGuidedWorkflowStatsRequest,
) (resp *analyticspb.RetrieveGuidedWorkflowStatsResponse, err error) {
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
	listAgentOnly := req.GetFilterToAgentsOnly() // Guided workflow stats track agent activities
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
		return &analyticspb.RetrieveGuidedWorkflowStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveGuidedWorkflowStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveGuidedWorkflowStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveGuidedWorkflowStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveGuidedWorkflowStatsForGroups(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readGuidedWorkflowStatsFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveGuidedWorkflowStatsForGroups(
	ctx context.Context,
	req *analyticspb.RetrieveGuidedWorkflowStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveGuidedWorkflowStatsResponse, error) {
	// Retrieve per-agent GuidedWorkflow stats for ALL users from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveGuidedWorkflowStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.readGuidedWorkflowStatsFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupGuidedWorkflowStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupsGuidedWorkflowStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupGuidedWorkflowStatsResponse(
	perUserResp *analyticspb.RetrieveGuidedWorkflowStatsResponse,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveGuidedWorkflowStatsResponse, error) {
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
	guidedWorkflowStatsResultsPerUserPerGroup := []*analyticspb.GuidedWorkflowStatsResult{}
	for _, guidedWorkflowStatsResult := range perUserResp.GuidedWorkflowStatsResults {
		if len(guidedWorkflowStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := guidedWorkflowStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					guidedWorkflowStatsResult.Attribute.Groups = append(guidedWorkflowStatsResult.Attribute.Groups, group)
				}
			}
			guidedWorkflowStatsResultsPerUserPerGroup = append(guidedWorkflowStatsResultsPerUserPerGroup, guidedWorkflowStatsResult)
		}
	}
	return &analyticspb.RetrieveGuidedWorkflowStatsResponse{
		GuidedWorkflowStatsResults:                         sortResults(guidedWorkflowStatsResultsPerUserPerGroup),
		TotalConversationCount:                             perUserResp.TotalConversationCount,
		TotalGuidedWorkflowClickCount:                      perUserResp.TotalGuidedWorkflowClickCount,
		AverageGuidedWorkflowClickCountPerConversation:     perUserResp.AverageGuidedWorkflowClickCountPerConversation,
		AverageGuidedWorkflowClickCountPerFrequencyPerUser: perUserResp.AverageGuidedWorkflowClickCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupsGuidedWorkflowStatsResponse(
	perUserResp *analyticspb.RetrieveGuidedWorkflowStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveGuidedWorkflowStatsResponse, error) {
	// Initialize the per-group GuidedWorkflow stats result.
	guidedWorkflowStatsResultsByGroup := make(map[string]*analyticspb.GuidedWorkflowStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		guidedWorkflowStatsResultsByGroup[group.Name] = &analyticspb.GuidedWorkflowStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, guidedWorkflowStatsResults := range perUserResp.GuidedWorkflowStatsResults {
		if len(guidedWorkflowStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := guidedWorkflowStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := guidedWorkflowStatsResultsByGroup[groupName]; exists {
					guidedWorkflowStatsResultsByGroup[groupName].TotalGuidedWorkflowClickCount += guidedWorkflowStatsResults.TotalGuidedWorkflowClickCount
					guidedWorkflowStatsResultsByGroup[groupName].TotalConversationCount += guidedWorkflowStatsResults.TotalConversationCount
					guidedWorkflowStatsResultsByGroup[groupName].GuidedWorkflowStats = append(guidedWorkflowStatsResultsByGroup[groupName].GuidedWorkflowStats, guidedWorkflowStatsResults.GuidedWorkflowStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	guidedWorkflowStatsResultsForAllGroups := []*analyticspb.GuidedWorkflowStatsResult{}
	for _, guidedWorkflowStatsResults := range guidedWorkflowStatsResultsByGroup {
		if guidedWorkflowStatsResults.TotalConversationCount == 0 || len(guidedWorkflowStatsResults.GuidedWorkflowStats) == 0 {
			continue
		}
		guidedWorkflowStatsResults.AverageGuidedWorkflowClickCountPerConversation = safeDivideFloat(guidedWorkflowStatsResults.TotalGuidedWorkflowClickCount, guidedWorkflowStatsResults.TotalConversationCount)
		userFrequencyCount := len(guidedWorkflowStatsResults.GuidedWorkflowStats)
		guidedWorkflowStatsResults.AverageGuidedWorkflowClickCountPerFrequencyPerUser = safeDivideFloat(guidedWorkflowStatsResults.TotalGuidedWorkflowClickCount, int32(userFrequencyCount))
		guidedWorkflowStatsResults.GuidedWorkflowStats = convertGuidedWorkflowStatsPerUserToPerGroup(guidedWorkflowStatsResults.GuidedWorkflowStats)
		guidedWorkflowStatsResultsForAllGroups = append(guidedWorkflowStatsResultsForAllGroups, guidedWorkflowStatsResults)
	}

	return &analyticspb.RetrieveGuidedWorkflowStatsResponse{
		GuidedWorkflowStatsResults:                         sortResults(guidedWorkflowStatsResultsForAllGroups),
		TotalConversationCount:                             perUserResp.TotalConversationCount,
		TotalGuidedWorkflowClickCount:                      perUserResp.TotalGuidedWorkflowClickCount,
		AverageGuidedWorkflowClickCountPerConversation:     perUserResp.AverageGuidedWorkflowClickCountPerConversation,
		AverageGuidedWorkflowClickCountPerFrequencyPerUser: perUserResp.AverageGuidedWorkflowClickCountPerFrequencyPerUser,
	}, nil
}

func convertGuidedWorkflowStatsPerUserToPerGroup(guidedWorkflowStatsPerUser []*analyticspb.GuidedWorkflowStat) []*analyticspb.GuidedWorkflowStat {
	// Initial and append per-timestamp stats per group.
	guidedWorkflowStatsByTimestamp := make(map[int64]*analyticspb.GuidedWorkflowStat)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, guidedWorkflowStats := range guidedWorkflowStatsPerUser {
		key := guidedWorkflowStats.TimeRange.StartTimestamp.Seconds
		if _, exists := guidedWorkflowStatsByTimestamp[key]; !exists {
			guidedWorkflowStatsByTimestamp[key] = &analyticspb.GuidedWorkflowStat{
				TimeRange: guidedWorkflowStats.TimeRange,
			}
		}

		guidedWorkflowStatsByTimestamp[key].TotalGuidedWorkflowClickCount += guidedWorkflowStats.TotalGuidedWorkflowClickCount
		guidedWorkflowStatsByTimestamp[key].TotalConversationCount += guidedWorkflowStats.TotalConversationCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-timestamp stats.
	guidedWorkflowStatsPerGroup := []*analyticspb.GuidedWorkflowStat{}
	for key, guidedWorkflowStats := range guidedWorkflowStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		guidedWorkflowStatsPerGroup = append(guidedWorkflowStatsPerGroup, &analyticspb.GuidedWorkflowStat{
			TimeRange:                     guidedWorkflowStats.TimeRange,
			TotalGuidedWorkflowClickCount: guidedWorkflowStats.TotalGuidedWorkflowClickCount,
			TotalConversationCount:        guidedWorkflowStats.TotalConversationCount,
			AverageGuidedWorkflowClickCountPerConversation:     safeDivideFloat(guidedWorkflowStats.TotalGuidedWorkflowClickCount, guidedWorkflowStats.TotalConversationCount),
			AverageGuidedWorkflowClickCountPerFrequencyPerUser: safeDivideFloat(guidedWorkflowStats.TotalGuidedWorkflowClickCount, userFrequencyCount),
		})
	}
	sort.Slice(guidedWorkflowStatsPerGroup, func(i, j int) bool {
		return guidedWorkflowStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < guidedWorkflowStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return guidedWorkflowStatsPerGroup
}
