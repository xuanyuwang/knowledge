package analyticsimpl

import (
	"context"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveConversationStats(
	ctx context.Context, req *analyticspb.RetrieveConversationStatsRequest,
) (resp *analyticspb.RetrieveConversationStatsResponse, err error) {
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
	listAgentOnly := req.GetFilterToAgentsOnly() // Conversation stats track agent activities
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
		return &analyticspb.RetrieveConversationStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveConversationStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveConversationStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveConversationStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveConversationStatsInternalForGroupsInSingleQuery(ctx, req, parent, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.retrieveConversationStatsInternal(ctx, req, parent, users)
}

func (a AnalyticsServiceImpl) retrieveConversationStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveConversationStatsRequest,
	parent *shared.CustomerProfile,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveConversationStatsResponse, error) {
	// Retrieve per-agent stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveConversationStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.retrieveConversationStatsInternal(ctx, perUserReqForAll, parent, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupConversationStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupConversationStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func (a AnalyticsServiceImpl) retrieveConversationStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveConversationStatsRequest,
	parent *shared.CustomerProfile,
	users []*userpb.User,
) (*analyticspb.RetrieveConversationStatsResponse, error) {
	_, err := postgres.ParseArgs(&req.Frequency, req.Metadata)
	if err != nil {
		return nil, err
	}

	// Whether to include CONVERSATION_SOURCE_BOT as part of stats (default as False).
	includeBotConversations, err := shared.IncludeConversationSourceBot(ctx, a.configClient, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	// Fetch conversation stats from ClickHouse.
	return a.readConversationStatsFromClickhouse(ctx, req, users, includeBotConversations, req.AllowMatchingStaleMetadataValues)
}

func convertRowsPerUserToPerUserPerGroupConversationStatsResponse(
	perUserResp *analyticspb.RetrieveConversationStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveConversationStatsResponse, error) {
	// Build the userName to user and groupName to group maps.
	groupNameToGroupMap := make(map[string]*userpb.Group)
	for _, group := range groups {
		groupNameToGroupMap[group.Name] = group
	}

	// Append the groups into the attribute of per-user resp.
	conversationStatsResultsPerUserPerGroup := []*analyticspb.ConversationStatsResult{}
	for _, conversationStatsResult := range perUserResp.ConversationStatsResults {
		if len(conversationStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := conversationStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					conversationStatsResult.Attribute.Groups = append(conversationStatsResult.Attribute.Groups, group)
				}
			}
			conversationStatsResultsPerUserPerGroup = append(conversationStatsResultsPerUserPerGroup, conversationStatsResult)
		}
	}
	return &analyticspb.RetrieveConversationStatsResponse{
		ConversationStatsResults:                    sortResults(conversationStatsResultsPerUserPerGroup),
		TotalConversationCount:                      perUserResp.TotalConversationCount,
		AverageConversationCountPerFrequency:        perUserResp.AverageConversationCountPerFrequency,
		AverageConversationCountPerFrequencyPerUser: perUserResp.AverageConversationCountPerFrequencyPerUser,
		AverageHandleTime:                           perUserResp.AverageHandleTime,
	}, nil
}

func convertRowsPerUserToPerGroupConversationStatsResponse(
	perUserResp *analyticspb.RetrieveConversationStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveConversationStatsResponse, error) {
	// Initialize the per-group conversation stats results.
	conversationStatsResultsByGroup := make(map[string]*analyticspb.ConversationStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		conversationStatsResultsByGroup[group.GetName()] = &analyticspb.ConversationStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, conversationStatsResults := range perUserResp.ConversationStatsResults {
		if len(conversationStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := conversationStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := conversationStatsResultsByGroup[groupName]; exists {
					conversationStatsResultsByGroup[groupName].ConversationStats = append(conversationStatsResultsByGroup[groupName].ConversationStats, conversationStatsResults.ConversationStats...)
					conversationStatsResultsByGroup[groupName].TotalConversationCount += conversationStatsResults.TotalConversationCount
				}
			}
		}
	}

	// Assemble per-group stats.
	conversationStatsResultsForAllGroups := []*analyticspb.ConversationStatsResult{}
	for _, conversationStatsResults := range conversationStatsResultsByGroup {
		if len(conversationStatsResults.ConversationStats) == 0 || conversationStatsResults.TotalConversationCount == 0 {
			continue
		}
		conversationStatsPerGroup, totalHandleTimeSeconds, totalFrequencyUserCount := convertPerUserToPerGroupConversationStats(conversationStatsResults.ConversationStats)
		conversationStatsResults.ConversationStats = conversationStatsPerGroup
		conversationStatsResults.AverageConversationCountPerFrequency = safeDivideFloat(conversationStatsResults.TotalConversationCount, int32(len(conversationStatsResults.ConversationStats)))
		conversationStatsResults.AverageConversationCountPerFrequencyPerUser = safeDivideFloat(conversationStatsResults.TotalConversationCount, totalFrequencyUserCount)
		conversationStatsResults.AverageHandleTime = &durationpb.Duration{
			Seconds: int64(safeDivideFloat(totalHandleTimeSeconds, conversationStatsResults.TotalConversationCount)),
		}
		conversationStatsResultsForAllGroups = append(conversationStatsResultsForAllGroups, conversationStatsResults)
	}

	return &analyticspb.RetrieveConversationStatsResponse{
		ConversationStatsResults:                    sortResults(conversationStatsResultsForAllGroups),
		TotalConversationCount:                      perUserResp.TotalConversationCount,
		AverageHandleTime:                           perUserResp.AverageHandleTime,
		AverageConversationCountPerFrequencyPerUser: perUserResp.AverageConversationCountPerFrequencyPerUser,
		AverageConversationCountPerFrequency:        perUserResp.AverageConversationCountPerFrequency,
	}, nil
}

func convertPerUserToPerGroupConversationStats(conversationStatsPerUser []*analyticspb.ConversationStats) ([]*analyticspb.ConversationStats, int32, int32) {
	// Initial and append per-frequency stats per group.
	conversationStatsByTimestamp := make(map[int64]*analyticspb.ConversationStats)
	handleTimeByTimestamp := make(map[int64]int32)
	userFreqCountByTimestamp := make(map[int64]int32)
	totalHandleTimeSeconds := int32(0)
	totalFrequencyUserCount := int32(0)
	for _, conversationStats := range conversationStatsPerUser {
		key := conversationStats.TimeRange.StartTimestamp.Seconds
		if _, exists := conversationStatsByTimestamp[key]; !exists {
			conversationStatsByTimestamp[key] = &analyticspb.ConversationStats{
				TimeRange: conversationStats.TimeRange,
			}
			handleTimeByTimestamp[key] = 0
			userFreqCountByTimestamp[key] = 0
		}
		conversationStatsByTimestamp[key].TotalConversationCount += conversationStats.TotalConversationCount
		totalHandleTimeSecondsPerUser := conversationStats.TotalConversationCount * int32(conversationStats.AverageHandleTime.Seconds)
		handleTimeByTimestamp[key] += totalHandleTimeSecondsPerUser
		userFreqCountByTimestamp[key]++
		totalHandleTimeSeconds += totalHandleTimeSecondsPerUser
		totalFrequencyUserCount++
	}

	// Assemble per-frequency stats.
	conversationStatsPerGroup := []*analyticspb.ConversationStats{}
	for key, conversationStats := range conversationStatsByTimestamp {
		conversationStatsPerGroup = append(conversationStatsPerGroup, &analyticspb.ConversationStats{
			TimeRange:              conversationStats.TimeRange,
			TotalConversationCount: conversationStats.TotalConversationCount,
			AverageConversationCountPerFrequencyPerUser: safeDivideFloat(conversationStats.TotalConversationCount, userFreqCountByTimestamp[key]),
			AverageHandleTime: &durationpb.Duration{
				Seconds: int64(safeDivideFloat(handleTimeByTimestamp[key], conversationStats.TotalConversationCount)),
			},
		})
	}
	sort.Slice(conversationStatsPerGroup, func(i, j int) bool {
		return conversationStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < conversationStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return conversationStatsPerGroup, totalHandleTimeSeconds, totalFrequencyUserCount
}
