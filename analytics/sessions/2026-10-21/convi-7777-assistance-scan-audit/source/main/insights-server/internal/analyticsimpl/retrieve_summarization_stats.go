package analyticsimpl

import (
	"context"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"google.golang.org/protobuf/proto"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveSummarizationStats(
	ctx context.Context, req *analyticspb.RetrieveSummarizationStatsRequest,
) (resp *analyticspb.RetrieveSummarizationStatsResponse, err error) {
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

	// New implementation using ParseUserFilterForAnalytics
	listAgentOnly := req.GetFilterToAgentsOnly() // Summarization stats track agent activities
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
		return &analyticspb.RetrieveSummarizationStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveSummarizationStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveSummarizationStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveSummarizationStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveSummarizationStatsInternalForGroupsInSingleQuery(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.retrieveSummarizationStatsInternal(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveSummarizationStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveSummarizationStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSummarizationStatsResponse, error) {
	// Retrieve per-agent stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveSummarizationStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.retrieveSummarizationStatsInternal(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupSummarizationStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupSummarizationStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func (a AnalyticsServiceImpl) retrieveSummarizationStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveSummarizationStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveSummarizationStatsResponse, error) {

	return a.readSummarizationStatsFromClickhouse(ctx, req, users)
}

func convertRowsPerUserToPerUserPerGroupSummarizationStatsResponse(
	perUserResp *analyticspb.RetrieveSummarizationStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSummarizationStatsResponse, error) {
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
	summarizationStatsResultsPerUserPerGroup := []*analyticspb.SummarizationStatsResult{}
	for _, summarizationStatsResult := range perUserResp.SummarizationStatsResults {
		if len(summarizationStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := summarizationStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					summarizationStatsResult.Attribute.Groups = append(summarizationStatsResult.Attribute.Groups, group)
				}
			}
			summarizationStatsResultsPerUserPerGroup = append(summarizationStatsResultsPerUserPerGroup, summarizationStatsResult)
		}
	}
	return &analyticspb.RetrieveSummarizationStatsResponse{
		SummarizationStatsResults:                             sortResults(summarizationStatsResultsPerUserPerGroup),
		TotalSummarizationUsedCount:                           perUserResp.TotalSummarizationUsedCount,
		TotalSummarizationGeneratedCount:                      perUserResp.TotalSummarizationGeneratedCount,
		TotalConversationCount:                                perUserResp.TotalConversationCount,
		AverageSummarizationUsedRatio:                         perUserResp.AverageSummarizationUsedRatio,
		AverageSummarizationUsedCountPerConversation:          perUserResp.AverageSummarizationUsedCountPerConversation,
		AverageSummarizationUsedCountPerFrequencyPerUser:      perUserResp.AverageSummarizationUsedCountPerFrequencyPerUser,
		AverageSummarizationGeneratedCountPerFrequencyPerUser: perUserResp.AverageSummarizationGeneratedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupSummarizationStatsResponse(
	perUserResp *analyticspb.RetrieveSummarizationStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveSummarizationStatsResponse, error) {
	// Initialize the per-group summarization stats results.
	summarizationStatsResultsByGroup := make(map[string]*analyticspb.SummarizationStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		summarizationStatsResultsByGroup[group.GetName()] = &analyticspb.SummarizationStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, summarizationStatsResults := range perUserResp.SummarizationStatsResults {
		if len(summarizationStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := summarizationStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := summarizationStatsResultsByGroup[groupName]; exists {
					summarizationStatsResultsByGroup[groupName].SummarizationStats = append(summarizationStatsResultsByGroup[groupName].SummarizationStats, summarizationStatsResults.SummarizationStats...)

					summarizationStatsResultsByGroup[groupName].TotalSummarizationUsedCount += summarizationStatsResults.TotalSummarizationUsedCount
					summarizationStatsResultsByGroup[groupName].TotalSummarizationGeneratedCount += summarizationStatsResults.TotalSummarizationGeneratedCount
					summarizationStatsResultsByGroup[groupName].TotalConversationCount += summarizationStatsResults.TotalConversationCount
				}
			}
		}
	}

	// Assemble per-group stats.
	summarizationStatsResultsForAllGroups := []*analyticspb.SummarizationStatsResult{}
	for _, summarizationStatsResults := range summarizationStatsResultsByGroup {
		if len(summarizationStatsResults.SummarizationStats) == 0 || summarizationStatsResults.TotalSummarizationUsedCount == 0 {
			continue
		}
		summarizationStatsPerGroup, totalFrequencyUserCount := convertPerUserToPerGroupSummarizationStats(summarizationStatsResults.SummarizationStats)
		summarizationStatsResults.SummarizationStats = summarizationStatsPerGroup
		summarizationStatsResults.AverageSummarizationUsedRatio = min(safeDivideFloat(summarizationStatsResults.TotalSummarizationUsedCount, summarizationStatsResults.TotalSummarizationGeneratedCount), 1) * 100
		summarizationStatsResults.AverageSummarizationUsedCountPerConversation = safeDivideFloat(summarizationStatsResults.TotalSummarizationUsedCount, summarizationStatsResults.TotalConversationCount)
		summarizationStatsResults.AverageSummarizationUsedCountPerFrequencyPerUser = safeDivideFloat(summarizationStatsResults.TotalSummarizationUsedCount, totalFrequencyUserCount)
		summarizationStatsResults.AverageSummarizationGeneratedCountPerFrequencyPerUser = safeDivideFloat(summarizationStatsResults.TotalSummarizationGeneratedCount, totalFrequencyUserCount)
		summarizationStatsResultsForAllGroups = append(summarizationStatsResultsForAllGroups, summarizationStatsResults)
	}

	return &analyticspb.RetrieveSummarizationStatsResponse{
		SummarizationStatsResults:                             sortResults(summarizationStatsResultsForAllGroups),
		TotalSummarizationUsedCount:                           perUserResp.TotalSummarizationUsedCount,
		TotalSummarizationGeneratedCount:                      perUserResp.TotalSummarizationGeneratedCount,
		TotalConversationCount:                                perUserResp.TotalConversationCount,
		AverageSummarizationUsedRatio:                         perUserResp.AverageSummarizationUsedRatio,
		AverageSummarizationUsedCountPerConversation:          perUserResp.AverageSummarizationUsedCountPerConversation,
		AverageSummarizationUsedCountPerFrequencyPerUser:      perUserResp.AverageSummarizationUsedCountPerFrequencyPerUser,
		AverageSummarizationGeneratedCountPerFrequencyPerUser: perUserResp.AverageSummarizationGeneratedCountPerFrequencyPerUser,
	}, nil
}

func convertPerUserToPerGroupSummarizationStats(autoSummarizationStatsPerUser []*analyticspb.SummarizationStats) ([]*analyticspb.SummarizationStats, int32) {
	// Initial and append per-timestamp stats per group.
	statsByTimestamp := make(map[int64]*analyticspb.SummarizationStats)
	frequencyUserCountByTimestamp := make(map[int64]int32)
	totalFrequencyUserCount := int32(0)
	for _, stats := range autoSummarizationStatsPerUser {
		key := stats.TimeRange.StartTimestamp.Seconds
		_, exists := statsByTimestamp[key]
		if !exists {
			statsByTimestamp[key] = &analyticspb.SummarizationStats{
				TimeRange: stats.TimeRange,
			}
			frequencyUserCountByTimestamp[key] = 0
		}

		statsByTimestamp[key].TotalSummarizationUsedCount += stats.TotalSummarizationUsedCount
		statsByTimestamp[key].TotalSummarizationGeneratedCount += stats.TotalSummarizationGeneratedCount
		statsByTimestamp[key].TotalConversationCount += stats.TotalConversationCount
		frequencyUserCountByTimestamp[key]++
		totalFrequencyUserCount++
	}

	// Assemble per-timestamp stats.
	statsPerGroup := []*analyticspb.SummarizationStats{}
	for key, stats := range statsByTimestamp {
		statsPerGroup = append(statsPerGroup, &analyticspb.SummarizationStats{
			TimeRange:                                    stats.TimeRange,
			TotalSummarizationUsedCount:                  stats.TotalSummarizationUsedCount,
			TotalSummarizationGeneratedCount:             stats.TotalSummarizationGeneratedCount,
			TotalConversationCount:                       stats.TotalConversationCount,
			AverageSummarizationUsedRatio:                min(safeDivideFloat(stats.TotalSummarizationUsedCount, stats.TotalSummarizationGeneratedCount), 1) * 100,
			AverageSummarizationUsedCountPerConversation: safeDivideFloat(stats.TotalSummarizationUsedCount, stats.TotalConversationCount),
			AverageSummarizationUsedCountPerUser:         safeDivideFloat(stats.TotalSummarizationUsedCount, frequencyUserCountByTimestamp[key]),
			AverageSummarizationGeneratedCountPerUser:    safeDivideFloat(stats.TotalSummarizationGeneratedCount, frequencyUserCountByTimestamp[key]),
		})
	}
	sort.Slice(statsPerGroup, func(i, j int) bool {
		return statsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < statsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return statsPerGroup, totalFrequencyUserCount
}
