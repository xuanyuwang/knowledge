package analyticsimpl

import (
	"context"
	"slices"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveHintStats(
	ctx context.Context, req *analyticspb.RetrieveHintStatsRequest,
) (resp *analyticspb.RetrieveHintStatsResponse, err error) {
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
	hintTypeFilter := analyticspb.Attribute_HINT_TYPE_UNSPECIFIED
	if req.FilterByAttribute != nil {
		hintTypeFilter = req.FilterByAttribute.HintType
	}
	// log request for debugging purpose
	if hintTypeFilter == analyticspb.Attribute_GW_HINT {
		a.logger.Infof(ctx, "RetrieveHintStatsRequest request for GW_HINT:\n%s", req)
	}

	var (
		users                             []*userpb.User
		groupsToAggregate                 []*userpb.Group
		userNameToGroupNamesMap           map[string][]string
		hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
		hasPolicyAsGroupByKey             = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY)
		includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
	)

	// New implementation using ParseUserFilterForAnalytics
	listAgentOnly := req.GetFilterToAgentsOnly() // Hint stats track agent activities
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
		return &analyticspb.RetrieveHintStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveHintStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveHintStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveHintStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveHintStatsInternalForGroupsInSingleQuery(ctx, req, hasAgentAsGroupByKey, hasPolicyAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.retrieveHintStatsInternal(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveHintStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveHintStatsRequest,
	hasAgentAsGroupByKey, hasPolicyAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveHintStatsResponse, error) {
	// Retrieve per-agent hint stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveHintStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	if len(req.FilterByAttribute.Groups) > 0 {
		// Only add user filters when group filter is not empty to avoid user list being too long.
		perUserReqForAll.FilterByAttribute.Users = append(perUserReqForAll.FilterByAttribute.Users, users...)
	}
	perUserResp, err := a.retrieveHintStatsInternal(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		// Only append group info onto attribute.
		return convertRowsPerUserToPerUserPerGroupHintStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	if hasPolicyAsGroupByKey {
		// Convert per-agent/per-policy to per-group/per-policy stats.
		return convertRowsPerUserPerPolicyToPerGroupPerPolicyHintStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
	}
	// Convert per-agent to per-group stats.
	return convertRowsPerUserToPerGroupHintStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func (a AnalyticsServiceImpl) retrieveHintStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveHintStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveHintStatsResponse, error) {
	return a.readHintStatsFromClickhouse(ctx, req, users)
}

// If `indicesToRemove` is not empty, it will remove the hint results at the given indices.
func removeHintResults(response *analyticspb.RetrieveHintStatsResponse, indicesToRemove []int) {
	if response == nil || len(indicesToRemove) == 0 {
		return
	}
	hintResults := []*analyticspb.HintStatsResult{}
	totalConvoCount := int32(0)
	totalHintSentCount := int32(0)
	totalHintFollowedCount := int32(0)
	totalAgentFrequencyCount := float32(0)
	for i, hintResult := range response.HintStatsResults {
		if slices.Contains(indicesToRemove, i) {
			continue
		}
		hintResults = append(hintResults, hintResult)
		totalConvoCount += hintResult.TotalConversationCount
		totalHintSentCount += hintResult.TotalHintSentCount
		totalHintFollowedCount += hintResult.TotalHintFollowedCount
		totalAgentFrequencyCount += safeDivideFloat(float32(hintResult.TotalHintSentCount), hintResult.AverageHintSentCountPerFrequencyPerUser)
	}
	response.HintStatsResults = hintResults
	response.TotalConversationCount = totalConvoCount
	response.TotalHintSentCount = totalHintSentCount
	response.TotalHintFollowedCount = totalHintFollowedCount
	response.AverageHintSentCountPerConversation = safeDivideFloat(totalHintSentCount, totalConvoCount)
	response.AverageHintFollowedCountPerConversation = safeDivideFloat(totalHintFollowedCount, totalConvoCount)
	response.AverageHintSentCountPerFrequencyPerUser = safeDivideFloat(float32(totalHintSentCount), totalAgentFrequencyCount)
	response.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(float32(totalHintFollowedCount), totalAgentFrequencyCount)
}

func convertRowsPerUserToPerUserPerGroupHintStatsResponse(
	perUserResp *analyticspb.RetrieveHintStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveHintStatsResponse, error) {
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
	hintStatsResultsPerUserPerGroup := []*analyticspb.HintStatsResult{}
	for _, hintStatsResult := range perUserResp.HintStatsResults {
		if len(hintStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := hintStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					hintStatsResult.Attribute.Groups = append(hintStatsResult.Attribute.Groups, group)
				}
			}
			hintStatsResultsPerUserPerGroup = append(hintStatsResultsPerUserPerGroup, hintStatsResult)
		}
	}
	return &analyticspb.RetrieveHintStatsResponse{
		HintStatsResults:                            sortResults(hintStatsResultsPerUserPerGroup),
		TotalConversationCount:                      perUserResp.TotalConversationCount,
		TotalHintSentCount:                          perUserResp.TotalHintSentCount,
		TotalHintFollowedCount:                      perUserResp.TotalHintFollowedCount,
		AverageHintSentCountPerConversation:         perUserResp.AverageHintSentCountPerConversation,
		AverageHintSentCountPerFrequencyPerUser:     perUserResp.AverageHintSentCountPerFrequencyPerUser,
		AverageHintFollowedCountPerConversation:     perUserResp.AverageHintFollowedCountPerConversation,
		AverageHintFollowedCountPerFrequencyPerUser: perUserResp.AverageHintFollowedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserPerPolicyToPerGroupPerPolicyHintStatsResponse(
	perUserResp *analyticspb.RetrieveHintStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveHintStatsResponse, error) {
	// Initialize the per-group and per-policy hint stats results.
	groupNameToGroup := map[string]*userpb.Group{}
	for _, group := range groups {
		groupNameToGroup[group.Name] = &userpb.Group{
			Name:        group.Name,
			DisplayName: group.DisplayName,
			Type:        group.Type,
		}
	}

	// Append per-user stats to per-group stats.
	hintStatsResultsByGroupAndPolicy := make(map[string]*analyticspb.HintStatsResult)
	for _, hintStatsResults := range perUserResp.HintStatsResults {
		if len(hintStatsResults.Attribute.Users) == 0 || len(hintStatsResults.Attribute.AttributePolicies) == 0 {
			continue
		}
		userName := hintStatsResults.Attribute.Users[0].GetName()
		policyName := ""
		if len(hintStatsResults.Attribute.AttributePolicies) > 0 {
			policyName = hintStatsResults.Attribute.AttributePolicies[0].GetName()
		}
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := hintStatsResultsByGroupAndPolicy[groupName+policyName]; !exists {
					group, groupExists := groupNameToGroup[groupName]
					if !groupExists {
						continue
					}
					hintStatsResultsByGroupAndPolicy[groupName+policyName] = &analyticspb.HintStatsResult{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{group},
							// Keep the same policies and actions from per-user results.
							AttributePolicies: hintStatsResults.Attribute.AttributePolicies,
							Actions:           hintStatsResults.Attribute.Actions,
						},
					}
				}
				hintStatsResultsByGroupAndPolicy[groupName+policyName].TotalHintSentCount += hintStatsResults.TotalHintSentCount
				hintStatsResultsByGroupAndPolicy[groupName+policyName].TotalHintFollowedCount += hintStatsResults.TotalHintFollowedCount
				hintStatsResultsByGroupAndPolicy[groupName+policyName].TotalConversationCount += hintStatsResults.TotalConversationCount
				hintStatsResultsByGroupAndPolicy[groupName+policyName].HintStats = append(hintStatsResultsByGroupAndPolicy[groupName+policyName].HintStats, hintStatsResults.HintStats...)
			}
		}
	}

	// Assemble per-group stats.
	hintStatsResultsForAllGroups := []*analyticspb.HintStatsResult{}
	for _, hintStatsResults := range hintStatsResultsByGroupAndPolicy {
		if hintStatsResults.TotalConversationCount == 0 {
			continue
		}
		hintStatsResults.AverageHintSentCountPerConversation = safeDivideFloat(hintStatsResults.TotalHintSentCount, hintStatsResults.TotalConversationCount)
		hintStatsResults.AverageHintFollowedCountPerConversation = safeDivideFloat(hintStatsResults.TotalHintFollowedCount, hintStatsResults.TotalConversationCount)
		userFrequencyCount := len(hintStatsResults.HintStats)
		if userFrequencyCount > 0 {
			hintStatsResults.AverageHintSentCountPerFrequencyPerUser = safeDivideFloat(hintStatsResults.TotalHintSentCount, int32(userFrequencyCount))
			hintStatsResults.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(hintStatsResults.TotalHintFollowedCount, int32(userFrequencyCount))
			hintStatsResults.HintStats = convertPerUserToPerGroupHintStats(hintStatsResults.HintStats)
		}
		hintStatsResultsForAllGroups = append(hintStatsResultsForAllGroups, hintStatsResults)
	}

	return &analyticspb.RetrieveHintStatsResponse{
		HintStatsResults:                            sortResults(hintStatsResultsForAllGroups),
		TotalConversationCount:                      perUserResp.TotalConversationCount,
		TotalHintSentCount:                          perUserResp.TotalHintSentCount,
		TotalHintFollowedCount:                      perUserResp.TotalHintFollowedCount,
		AverageHintSentCountPerConversation:         perUserResp.AverageHintSentCountPerConversation,
		AverageHintSentCountPerFrequencyPerUser:     perUserResp.AverageHintSentCountPerFrequencyPerUser,
		AverageHintFollowedCountPerConversation:     perUserResp.AverageHintFollowedCountPerConversation,
		AverageHintFollowedCountPerFrequencyPerUser: perUserResp.AverageHintFollowedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupHintStatsResponse(
	perUserResp *analyticspb.RetrieveHintStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveHintStatsResponse, error) {
	// Initialize the per-group hint stats results.
	hintStatsResultsByGroup := make(map[string]*analyticspb.HintStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		hintStatsResultsByGroup[group.GetName()] = &analyticspb.HintStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, hintStatsResults := range perUserResp.HintStatsResults {
		if len(hintStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := hintStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := hintStatsResultsByGroup[groupName]; exists {
					hintStatsResultsByGroup[groupName].TotalHintSentCount += hintStatsResults.TotalHintSentCount
					hintStatsResultsByGroup[groupName].TotalHintFollowedCount += hintStatsResults.TotalHintFollowedCount
					hintStatsResultsByGroup[groupName].TotalConversationCount += hintStatsResults.TotalConversationCount
					hintStatsResultsByGroup[groupName].HintStats = append(hintStatsResultsByGroup[groupName].HintStats, hintStatsResults.HintStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	hintStatsResultsForAllGroups := []*analyticspb.HintStatsResult{}
	for _, hintStatsResults := range hintStatsResultsByGroup {
		if hintStatsResults.TotalConversationCount == 0 {
			continue
		}
		hintStatsResults.AverageHintSentCountPerConversation = safeDivideFloat(hintStatsResults.TotalHintSentCount, hintStatsResults.TotalConversationCount)
		hintStatsResults.AverageHintFollowedCountPerConversation = safeDivideFloat(hintStatsResults.TotalHintFollowedCount, hintStatsResults.TotalConversationCount)
		userFrequencyCount := len(hintStatsResults.HintStats)
		if userFrequencyCount > 0 {
			hintStatsResults.AverageHintSentCountPerFrequencyPerUser = safeDivideFloat(hintStatsResults.TotalHintSentCount, int32(userFrequencyCount))
			hintStatsResults.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(hintStatsResults.TotalHintFollowedCount, int32(userFrequencyCount))
			hintStatsResults.HintStats = convertPerUserToPerGroupHintStats(hintStatsResults.HintStats)
		}
		hintStatsResultsForAllGroups = append(hintStatsResultsForAllGroups, hintStatsResults)
	}

	return &analyticspb.RetrieveHintStatsResponse{
		HintStatsResults:                            sortResults(hintStatsResultsForAllGroups),
		TotalConversationCount:                      perUserResp.TotalConversationCount,
		TotalHintSentCount:                          perUserResp.TotalHintSentCount,
		TotalHintFollowedCount:                      perUserResp.TotalHintFollowedCount,
		AverageHintSentCountPerConversation:         perUserResp.AverageHintSentCountPerConversation,
		AverageHintSentCountPerFrequencyPerUser:     perUserResp.AverageHintSentCountPerFrequencyPerUser,
		AverageHintFollowedCountPerConversation:     perUserResp.AverageHintFollowedCountPerConversation,
		AverageHintFollowedCountPerFrequencyPerUser: perUserResp.AverageHintFollowedCountPerFrequencyPerUser,
	}, nil
}

func convertPerUserToPerGroupHintStats(hintStatsPerUser []*analyticspb.HintStats) []*analyticspb.HintStats {
	// Initial and append per-frequency stats per group.
	hintStatsByTimestamp := make(map[int64]*analyticspb.HintStats)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, hintStats := range hintStatsPerUser {
		key := hintStats.TimeRange.StartTimestamp.Seconds
		if _, exists := hintStatsByTimestamp[key]; !exists {
			hintStatsByTimestamp[key] = &analyticspb.HintStats{
				TimeRange: hintStats.TimeRange,
			}
		}
		hintStatsByTimestamp[key].TotalHintSentCount += hintStats.TotalHintSentCount
		hintStatsByTimestamp[key].TotalHintFollowedCount += hintStats.TotalHintFollowedCount
		hintStatsByTimestamp[key].TotalConversationCount += hintStats.TotalConversationCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-frequency stats.
	hintStatsPerGroup := []*analyticspb.HintStats{}
	for key, hintStats := range hintStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		hintStatsPerGroup = append(hintStatsPerGroup, &analyticspb.HintStats{
			TimeRange:                                   hintStats.TimeRange,
			TotalHintSentCount:                          hintStats.TotalHintSentCount,
			TotalHintFollowedCount:                      hintStats.TotalHintFollowedCount,
			TotalConversationCount:                      hintStats.TotalConversationCount,
			AverageHintSentCountPerConversation:         safeDivideFloat(hintStats.TotalHintSentCount, hintStats.TotalConversationCount),
			AverageHintSentCountPerFrequencyPerUser:     safeDivideFloat(hintStats.TotalHintSentCount, userFrequencyCount),
			AverageHintFollowedCountPerConversation:     safeDivideFloat(hintStats.TotalHintFollowedCount, hintStats.TotalConversationCount),
			AverageHintFollowedCountPerFrequencyPerUser: safeDivideFloat(hintStats.TotalHintFollowedCount, userFrequencyCount),
		})
	}
	sort.Slice(hintStatsPerGroup, func(i, j int) bool {
		return hintStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < hintStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return hintStatsPerGroup
}
