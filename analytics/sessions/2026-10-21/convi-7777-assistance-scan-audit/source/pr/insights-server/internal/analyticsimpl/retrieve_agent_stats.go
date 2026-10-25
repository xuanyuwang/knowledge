package analyticsimpl

import (
	"context"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveAgentStats(
	ctx context.Context, req *analyticspb.RetrieveAgentStatsRequest,
) (resp *analyticspb.RetrieveAgentStatsResponse, err error) {
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)
	var (
		users                   []*userpb.User
		groupsToAggregate       []*userpb.Group
		userNameToGroupNamesMap map[string][]string
	)
	err = shared.ValidateGroupByAttributes(req.GroupByAttributeTypes, req.Frequency)
	if err != nil {
		return nil, err
	}

	parent, err := shared.ParseCustomer(req.Parent)
	if err != nil {
		return nil, err
	}
	if len(req.GetProfileId()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "`profile_id` is required.")
	}

	// process user filters to get the users and groups
	hasAgentAsGroupByKey := shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
	includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())

	// New implementation using ParseUserFilterForAnalytics
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
		req.GetProfileId(),
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
		return &analyticspb.RetrieveAgentStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveAgentStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveAgentStatsResponse
	profileName := &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: req.GetProfileId()}
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveAgentStats", insightsStatsAPICachingCustomerIDs, false, profileName, cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveAgentStatsInternalForGroupsInSingleQuery(
			ctx,
			req,
			parent,
			req.ProfileId,
			hasAgentAsGroupByKey,
			users,
			groupsToAggregate,
			userNameToGroupNamesMap,
		)
	}
	return a.retrieveAgentStatsInternal(
		ctx,
		req,
		parent,
		req.GetProfileId(),
		users,
	)
}

func (a AnalyticsServiceImpl) retrieveAgentStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveAgentStatsRequest,
	parent *shared.Customer, profileID string,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
	// Retrieve per-agent stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveAgentStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.retrieveAgentStatsInternal(ctx, perUserReqForAll, parent, profileID, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupAgentStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupAgentStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

// This method returns true if either env flag enableAgentCountFromCrestaConversation or
// config flag use_cresta_conversation_count is true. The following implementation avoid unnecessary
// API call when env flag is true.
func (a AnalyticsServiceImpl) useCrestaConversationCount(customerID, profileID string) (bool, error) {
	if a.enableAgentCountFromCrestaConversation {
		return true, nil
	}
	// Get the customer (short) name based on customer ID and profile ID.
	profile, err := a.configClient.GetProfileConfig(customerID, profileID)
	if err != nil {
		return false, err
	}
	return profile.GetFeatures() != nil &&
		profile.GetFeatures().GetInsights() != nil &&
		profile.GetFeatures().GetInsights().UseCrestaConversationCount, nil
}

func (a AnalyticsServiceImpl) retrieveAgentStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveAgentStatsRequest,
	parent *shared.Customer, profileID string,
	users []*userpb.User,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
	var err error

	useCrestaConvoCount, err := a.useCrestaConversationCount(parent.CustomerID, profileID)
	if err != nil {
		return nil, err
	}
	includeBotConversations, err := shared.IncludeConversationSourceBot(ctx, a.configClient, parent.CustomerID, profileID)
	if err != nil {
		return nil, err
	}
	includeBotConversations = includeBotConversations || req.FilterByAttribute.IncludeCrestaBot

	includeIngestionPipeline, err := shared.IncludeIngestionPipelineInAgentStats(ctx, a.configClient, parent.CustomerID, profileID)
	if err != nil {
		return nil, err
	}

	return a.readAgentStatsFromClickhouseConvWithLabel(
		ctx, req, profileID, useCrestaConvoCount, users, includeBotConversations, includeIngestionPipeline,
	)
}

func convertRowsPerUserToPerUserPerGroupAgentStatsResponse(
	perUserResp *analyticspb.RetrieveAgentStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
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
	agentStatsResultsPerUserPerGroup := []*analyticspb.AgentStatsResult{}
	for _, agentStatsResult := range perUserResp.AgentStatsResults {
		if len(agentStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := agentStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					agentStatsResult.Attribute.Groups = append(agentStatsResult.Attribute.Groups, group)
				}
			}
			agentStatsResultsPerUserPerGroup = append(agentStatsResultsPerUserPerGroup, agentStatsResult)
		}
	}
	return &analyticspb.RetrieveAgentStatsResponse{
		AgentStatsResults:                   sortResults(agentStatsResultsPerUserPerGroup),
		TotalAgentLoginCount:                perUserResp.TotalAgentLoginCount,
		AverageActiveAgentCountPerFrequency: perUserResp.AverageActiveAgentCountPerFrequency,
		AverageActiveDayCountPerUser:        perUserResp.AverageActiveDayCountPerUser,
		ActiveAgentCount:                    perUserResp.ActiveAgentCount,
	}, nil
}

func convertRowsPerUserToPerGroupAgentStatsResponse(
	perUserResp *analyticspb.RetrieveAgentStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
	// Initialize the per-group agent stats results.
	agentStatsResultsByGroup := make(map[string]*analyticspb.AgentStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		agentStatsResultsByGroup[group.GetName()] = &analyticspb.AgentStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, agentStatsResults := range perUserResp.AgentStatsResults {
		if len(agentStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := agentStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := agentStatsResultsByGroup[groupName]; exists {
					agentStatsResultsByGroup[groupName].AgentStats = append(agentStatsResultsByGroup[groupName].AgentStats, agentStatsResults.AgentStats...)
					agentStatsResultsByGroup[groupName].TotalAgentLoginCount += agentStatsResults.TotalAgentLoginCount
					agentStatsResultsByGroup[groupName].ActiveAgentCount += agentStatsResults.ActiveAgentCount
				}
			}
		}
	}

	// Assemble per-group stats.
	agentStatsResultsForAllGroups := []*analyticspb.AgentStatsResult{}
	for _, agentStatsResults := range agentStatsResultsByGroup {
		// We do need to aggregate the case when TotalAgentLoginCount == 0.
		// For agent stats, the N/A and 0 does make a difference
		// - N/A: no conversation for this agent.
		// - 0: has conversation but agent doesn't log in.
		if len(agentStatsResults.AgentStats) == 0 {
			continue
		}
		agentStatsResults.AgentStats = convertPerUserToPerGroupAgentStats(agentStatsResults.AgentStats)
		agentStatsResults.AverageActiveDayCountPerUser = safeDivideFloat(agentStatsResults.TotalAgentLoginCount, agentStatsResults.ActiveAgentCount)
		agentStatsResults.AverageActiveAgentCountPerFrequency = safeDivideFloat(agentStatsResults.TotalAgentLoginCount, int32(countNonEmptyFrequence(agentStatsResults.AgentStats)))
		agentStatsResultsForAllGroups = append(agentStatsResultsForAllGroups, agentStatsResults)
	}

	return &analyticspb.RetrieveAgentStatsResponse{
		AgentStatsResults:                   sortResults(agentStatsResultsForAllGroups),
		TotalAgentLoginCount:                perUserResp.TotalAgentLoginCount,
		AverageActiveDayCountPerUser:        perUserResp.AverageActiveDayCountPerUser,
		ActiveAgentCount:                    perUserResp.ActiveAgentCount,
		AverageActiveAgentCountPerFrequency: perUserResp.AverageActiveAgentCountPerFrequency,
	}, nil
}

func convertPerUserToPerGroupAgentStats(agentStatsPerUser []*analyticspb.AgentStats) []*analyticspb.AgentStats {
	// Initial and append per-frequency stats per group.
	agentStatsByTimestamp := make(map[int64]*analyticspb.AgentStats)
	for _, agentStats := range agentStatsPerUser {
		key := agentStats.TimeRange.StartTimestamp.Seconds
		if _, exists := agentStatsByTimestamp[key]; !exists {
			agentStatsByTimestamp[key] = &analyticspb.AgentStats{
				TimeRange: agentStats.TimeRange,
			}
		}
		agentStatsByTimestamp[key].TotalAgentLoginCount += agentStats.TotalAgentLoginCount
		agentStatsByTimestamp[key].ActiveAgentCount += agentStats.ActiveAgentCount
		agentStatsByTimestamp[key].AverageActiveAgentCountPerFrequency += agentStats.AverageActiveAgentCountPerFrequency
	}

	// Assemble per-frequency stats.
	agentStatsPerGroup := []*analyticspb.AgentStats{}
	for _, agentStats := range agentStatsByTimestamp {
		averageActiveDayCountPerUser := 0
		if agentStats.TotalAgentLoginCount > 0 {
			averageActiveDayCountPerUser = 1
		}
		agentStatsPerGroup = append(agentStatsPerGroup, &analyticspb.AgentStats{
			TimeRange:                           agentStats.TimeRange,
			TotalAgentLoginCount:                agentStats.TotalAgentLoginCount,
			ActiveAgentCount:                    agentStats.ActiveAgentCount,
			AverageActiveAgentCountPerFrequency: agentStats.AverageActiveAgentCountPerFrequency,
			AverageActiveDayCountPerUser:        float32(averageActiveDayCountPerUser),
		})
	}
	sort.Slice(agentStatsPerGroup, func(i, j int) bool {
		return agentStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < agentStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return agentStatsPerGroup
}

func countNonEmptyFrequence(agentStats []*analyticspb.AgentStats) int {
	count := 0
	for _, stats := range agentStats {
		if stats.TotalAgentLoginCount > 0 {
			count += 1
		}
	}
	return count
}
