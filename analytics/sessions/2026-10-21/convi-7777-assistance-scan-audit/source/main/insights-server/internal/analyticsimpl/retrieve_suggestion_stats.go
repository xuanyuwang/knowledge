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
func (a AnalyticsServiceImpl) RetrieveSuggestionStats(
	ctx context.Context, req *analyticspb.RetrieveSuggestionStatsRequest,
) (resp *analyticspb.RetrieveSuggestionStatsResponse, err error) {
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
	listAgentOnly := req.GetFilterToAgentsOnly() // Suggestion stats track agent activities
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
		return &analyticspb.RetrieveSuggestionStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveSuggestionStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveSuggestionStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveSuggestionStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveSuggestionStatsForGroups(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readSuggestionStatsFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveSuggestionStatsForGroups(
	ctx context.Context,
	req *analyticspb.RetrieveSuggestionStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSuggestionStatsResponse, error) {
	// Retrieve per-agent suggestion stats for ALL users from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveSuggestionStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.readSuggestionStatsFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupSuggestionStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupSuggestionStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupSuggestionStatsResponse(
	perUserResp *analyticspb.RetrieveSuggestionStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSuggestionStatsResponse, error) {
	// Build the groupName to group maps.
	groupNameToGroupMap := make(map[string]*userpb.Group)
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	// Append the groups into the attribute of per-user resp.
	suggestionStatsResultsPerUserPerGroup := []*analyticspb.SuggestionStatsResult{}
	for _, suggestionStatsResult := range perUserResp.SuggestionStatsResults {
		if len(suggestionStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := suggestionStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					suggestionStatsResult.Attribute.Groups = append(suggestionStatsResult.Attribute.Groups, group)
				}
			}
			suggestionStatsResultsPerUserPerGroup = append(suggestionStatsResultsPerUserPerGroup, suggestionStatsResult)
		}
	}
	return &analyticspb.RetrieveSuggestionStatsResponse{
		SuggestionStatsResults:                        sortResults(suggestionStatsResultsPerUserPerGroup),
		TotalConversationCount:                        perUserResp.TotalConversationCount,
		TotalSuggestionUsedCount:                      perUserResp.TotalSuggestionUsedCount,
		AverageSuggestionUsedCountPerConversation:     perUserResp.AverageSuggestionUsedCountPerConversation,
		AverageSuggestionUsedCountPerFrequencyPerUser: perUserResp.AverageSuggestionUsedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupSuggestionStatsResponse(
	perUserResp *analyticspb.RetrieveSuggestionStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSuggestionStatsResponse, error) {
	// Initialize the per-group suggestion stats result.
	suggestionStatsResultsByGroup := make(map[string]*analyticspb.SuggestionStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		suggestionStatsResultsByGroup[group.Name] = &analyticspb.SuggestionStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, suggestionStatsResults := range perUserResp.SuggestionStatsResults {
		if len(suggestionStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := suggestionStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := suggestionStatsResultsByGroup[groupName]; exists {
					suggestionStatsResultsByGroup[groupName].TotalSuggestionUsedCount += suggestionStatsResults.TotalSuggestionUsedCount
					suggestionStatsResultsByGroup[groupName].TotalConversationCount += suggestionStatsResults.TotalConversationCount
					suggestionStatsResultsByGroup[groupName].SuggestionStats = append(suggestionStatsResultsByGroup[groupName].SuggestionStats, suggestionStatsResults.SuggestionStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	suggestionStatsResultsForAllGroups := []*analyticspb.SuggestionStatsResult{}
	for _, suggestionStatsResults := range suggestionStatsResultsByGroup {
		if suggestionStatsResults.TotalConversationCount == 0 || len(suggestionStatsResults.SuggestionStats) == 0 {
			continue
		}
		suggestionStatsResults.AverageSuggestionUsedCountPerConversation = safeDivideFloat(suggestionStatsResults.TotalSuggestionUsedCount, suggestionStatsResults.TotalConversationCount)
		userFrequencyCount := len(suggestionStatsResults.SuggestionStats)
		suggestionStatsResults.AverageSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(suggestionStatsResults.TotalSuggestionUsedCount, int32(userFrequencyCount))
		suggestionStatsResults.SuggestionStats = convertSuggestionStatsPerUserToPerGroup(suggestionStatsResults.SuggestionStats)
		suggestionStatsResultsForAllGroups = append(suggestionStatsResultsForAllGroups, suggestionStatsResults)
	}

	return &analyticspb.RetrieveSuggestionStatsResponse{
		SuggestionStatsResults:                        sortResults(suggestionStatsResultsForAllGroups),
		TotalConversationCount:                        perUserResp.TotalConversationCount,
		TotalSuggestionUsedCount:                      perUserResp.TotalSuggestionUsedCount,
		AverageSuggestionUsedCountPerConversation:     perUserResp.AverageSuggestionUsedCountPerConversation,
		AverageSuggestionUsedCountPerFrequencyPerUser: perUserResp.AverageSuggestionUsedCountPerFrequencyPerUser,
	}, nil
}

func convertSuggestionStatsPerUserToPerGroup(suggestionStatsPerUser []*analyticspb.SuggestionStats) []*analyticspb.SuggestionStats {
	// Initial and append per-timestamp stats per group.
	suggestionStatsByTimestamp := make(map[int64]*analyticspb.SuggestionStats)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, suggestionStats := range suggestionStatsPerUser {
		key := suggestionStats.TimeRange.StartTimestamp.Seconds
		if _, exists := suggestionStatsByTimestamp[key]; !exists {
			suggestionStatsByTimestamp[key] = &analyticspb.SuggestionStats{
				TimeRange: suggestionStats.TimeRange,
			}
		}

		suggestionStatsByTimestamp[key].TotalSuggestionUsedCount += suggestionStats.TotalSuggestionUsedCount
		suggestionStatsByTimestamp[key].TotalConversationCount += suggestionStats.TotalConversationCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-timestamp stats.
	suggestionStatsPerGroup := []*analyticspb.SuggestionStats{}
	for key, suggestionStats := range suggestionStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		suggestionStatsPerGroup = append(suggestionStatsPerGroup, &analyticspb.SuggestionStats{
			TimeRange:                                     suggestionStats.TimeRange,
			TotalSuggestionUsedCount:                      suggestionStats.TotalSuggestionUsedCount,
			TotalConversationCount:                        suggestionStats.TotalConversationCount,
			AverageSuggestionUsedCountPerConversation:     safeDivideFloat(suggestionStats.TotalSuggestionUsedCount, suggestionStats.TotalConversationCount),
			AverageSuggestionUsedCountPerFrequencyPerUser: safeDivideFloat(suggestionStats.TotalSuggestionUsedCount, userFrequencyCount),
		})
	}
	sort.Slice(suggestionStatsPerGroup, func(i, j int) bool {
		return suggestionStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < suggestionStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return suggestionStatsPerGroup
}
