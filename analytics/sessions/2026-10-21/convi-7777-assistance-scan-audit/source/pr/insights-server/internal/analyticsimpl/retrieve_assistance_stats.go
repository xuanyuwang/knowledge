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
func (a AnalyticsServiceImpl) RetrieveAssistanceStats(
	ctx context.Context, req *analyticspb.RetrieveAssistanceStatsRequest,
) (resp *analyticspb.RetrieveAssistanceStatsResponse, err error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	profile, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	// As of 12/17/2024, most customers don't need this (legacy) API. To make
	// FE code easier, we determine to skip API logic in BE.
	if profile.GetFeatures() != nil &&
		profile.GetFeatures().GetInsights() != nil &&
		profile.GetFeatures().GetInsights().DeprecateRetrieveAssistanceStats {
		return &analyticspb.RetrieveAssistanceStatsResponse{}, nil
	}

	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)
	if err := shared.ValidateGroupByAttributes(req.GroupByAttributeTypes, req.Frequency); err != nil {
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
		return &analyticspb.RetrieveAssistanceStatsResponse{}, nil
	}
	if result.ShouldQueryAllUsers && !a.enableExtTableForUserFilter {
		req.FilterByAttribute.Users = nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveAssistanceStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveAssistanceStatsResponse
	found, storeCache, cacheErr := a.getInsightsStatsFromCache(ctx, "RetrieveAssistanceStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && cacheErr == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		// Retrieve per-agent stats for ALL users fetched from the given groups,
		// then aggregate up to per-group (or per-user-per-group).
		perUserReq := proto.Clone(req).(*analyticspb.RetrieveAssistanceStatsRequest)
		perUserReq.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
		perUserResp, err := a.readAssistanceStatsFromClickhouse(ctx, perUserReq, users)
		if err != nil {
			return nil, err
		}
		if hasAgentAsGroupByKey {
			return convertRowsPerUserToPerUserPerGroupAssistanceStatsResponse(perUserResp, groupsToAggregate, userNameToGroupNamesMap), nil
		}
		return convertRowsPerUserToPerGroupAssistanceStatsResponse(perUserResp, groupsToAggregate, userNameToGroupNamesMap), nil
	}
	return a.readAssistanceStatsFromClickhouse(ctx, req, users)
}

// convertRowsPerUserToPerUserPerGroupAssistanceStatsResponse attaches the
// caller-supplied groups to each per-user assistance stats result. The
// per-user response only carries hint stats (the only metric still backed by
// ClickHouse), so totals are taken from the per-user response unchanged.
func convertRowsPerUserToPerUserPerGroupAssistanceStatsResponse(
	perUserResp *analyticspb.RetrieveAssistanceStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) *analyticspb.RetrieveAssistanceStatsResponse {
	groupNameToGroupMap := make(map[string]*userpb.Group, len(groups))
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	results := []*analyticspb.AssistanceStatsResult{}
	for _, r := range perUserResp.AssistanceStatsResults {
		if r.Attribute == nil || len(r.Attribute.Users) == 0 {
			continue
		}
		userName := r.Attribute.Users[0].Name
		groupNames, exists := userNameToGroupNamesMap[userName]
		if !exists {
			continue
		}
		for _, groupName := range groupNames {
			if group, ok := groupNameToGroupMap[groupName]; ok {
				group.Members = []*userpb.GroupMembership{}
				r.Attribute.Groups = append(r.Attribute.Groups, group)
			}
		}
		results = append(results, r)
	}
	return &analyticspb.RetrieveAssistanceStatsResponse{
		AssistanceStatsResults:                      sortResults(results),
		TotalHintFollowedCount:                      perUserResp.TotalHintFollowedCount,
		AverageHintFollowedCountPerConversation:     perUserResp.AverageHintFollowedCountPerConversation,
		AverageHintFollowedCountPerFrequencyPerUser: perUserResp.AverageHintFollowedCountPerFrequencyPerUser,
	}
}

// convertRowsPerUserToPerGroupAssistanceStatsResponse aggregates per-user hint
// stats into per-group hint stats. Only hint metrics are populated since the
// ClickHouse path only computes hint stats.
func convertRowsPerUserToPerGroupAssistanceStatsResponse(
	perUserResp *analyticspb.RetrieveAssistanceStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) *analyticspb.RetrieveAssistanceStatsResponse {
	resultsByGroup := make(map[string]*analyticspb.AssistanceStatsResult, len(groups))
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		resultsByGroup[group.GetName()] = &analyticspb.AssistanceStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	for _, r := range perUserResp.AssistanceStatsResults {
		if r.Attribute == nil || len(r.Attribute.Users) == 0 {
			continue
		}
		userName := r.Attribute.Users[0].GetName()
		groupNames, exists := userNameToGroupNamesMap[userName]
		if !exists {
			continue
		}
		for _, groupName := range groupNames {
			if perGroup, ok := resultsByGroup[groupName]; ok {
				perGroup.HintStats = append(perGroup.HintStats, r.HintStats...)
				perGroup.TotalHintFollowedCount += r.TotalHintFollowedCount
			}
		}
	}

	allGroupResults := []*analyticspb.AssistanceStatsResult{}
	for _, r := range resultsByGroup {
		if len(r.HintStats) == 0 || r.TotalHintFollowedCount == 0 {
			continue
		}
		hintStats, totalConversationCount, totalFrequencyUserCount := convertPerUserToPerGroupHintFollowedStats(r.HintStats)
		r.HintStats = hintStats
		r.AverageHintFollowedCountPerConversation = safeDivideFloat(r.TotalHintFollowedCount, totalConversationCount)
		r.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(r.TotalHintFollowedCount, totalFrequencyUserCount)
		allGroupResults = append(allGroupResults, r)
	}

	return &analyticspb.RetrieveAssistanceStatsResponse{
		AssistanceStatsResults:                      sortResults(allGroupResults),
		TotalHintFollowedCount:                      perUserResp.TotalHintFollowedCount,
		AverageHintFollowedCountPerConversation:     perUserResp.AverageHintFollowedCountPerConversation,
		AverageHintFollowedCountPerFrequencyPerUser: perUserResp.AverageHintFollowedCountPerFrequencyPerUser,
	}
}

// convertPerUserToPerGroupHintFollowedStats merges per-user hint stats into a
// single per-timestamp series, returning the merged series along with the
// total conversation count and total user-frequency count derived from the
// per-user averages.
func convertPerUserToPerGroupHintFollowedStats(hintStatsPerUser []*analyticspb.HintStats) ([]*analyticspb.HintStats, int32, int32) {
	statsByTimestamp := make(map[int64]*analyticspb.HintStats)
	conversationCountByTimestamp := make(map[int64]int32)
	frequencyUserCountByTimestamp := make(map[int64]int32)
	totalConversationCount := int32(0)
	totalFrequencyUserCount := int32(0)
	for _, stats := range hintStatsPerUser {
		key := stats.TimeRange.StartTimestamp.Seconds
		if _, exists := statsByTimestamp[key]; !exists {
			statsByTimestamp[key] = &analyticspb.HintStats{TimeRange: stats.TimeRange}
		}
		statsByTimestamp[key].TotalHintFollowedCount += stats.TotalHintFollowedCount
		conversationCount := int32(safeDivideFloat(float32(stats.TotalHintFollowedCount), stats.AverageHintFollowedCountPerConversation))
		conversationCountByTimestamp[key] += conversationCount
		totalConversationCount += conversationCount
		frequencyUserCountByTimestamp[key]++
		totalFrequencyUserCount++
	}

	statsPerGroup := make([]*analyticspb.HintStats, 0, len(statsByTimestamp))
	for key, stats := range statsByTimestamp {
		statsPerGroup = append(statsPerGroup, &analyticspb.HintStats{
			TimeRange:                                   stats.TimeRange,
			TotalHintFollowedCount:                      stats.TotalHintFollowedCount,
			AverageHintFollowedCountPerConversation:     safeDivideFloat(stats.TotalHintFollowedCount, conversationCountByTimestamp[key]),
			AverageHintFollowedCountPerFrequencyPerUser: safeDivideFloat(stats.TotalHintFollowedCount, frequencyUserCountByTimestamp[key]),
		})
	}
	sort.Slice(statsPerGroup, func(i, j int) bool {
		return statsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < statsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return statsPerGroup, totalConversationCount, totalFrequencyUserCount
}
