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
func (a AnalyticsServiceImpl) RetrieveLiveAssistStats(
	ctx context.Context, req *analyticspb.RetrieveLiveAssistStatsRequest,
) (resp *analyticspb.RetrieveLiveAssistStatsResponse, err error) {
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
	listAgentOnly := req.GetFilterToAgentsOnly() // Use request parameter to determine filtering
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

	// Reset if not grouping by agent or group
	if !shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) &&
		!shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		users = []*userpb.User{}
		groupsToAggregate = []*userpb.Group{}
		userNameToGroupNamesMap = map[string][]string{}
	}

	if shouldEarlyReturn := ApplyUserFilterFromResult(result, &req.FilterByAttribute.Users, &req.FilterByAttribute.Groups, a.enableExtTableForUserFilter); shouldEarlyReturn {
		return &analyticspb.RetrieveLiveAssistStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveLiveAssistStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveLiveAssistStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveLiveAssistStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveLiveAssistStatsInternalForGroupsInSingleQuery(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.retrieveLiveAssistStatsInternal(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveLiveAssistStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveLiveAssistStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
	// Retrieve per-agent stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveLiveAssistStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.retrieveLiveAssistStatsInternal(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupLiveAssistStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupLiveAssistStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func (a AnalyticsServiceImpl) retrieveLiveAssistStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveLiveAssistStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
	_, err := postgres.ParseArgs(&req.Frequency, req.Metadata)
	if err != nil {
		return nil, err
	}

	return a.readLiveAssistStatsFromClickhouse(ctx, req, users)
}

func convertRowsPerUserToPerUserPerGroupLiveAssistStatsResponse(
	perUserResp *analyticspb.RetrieveLiveAssistStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
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
	liveAssistResultsPerUserPerGroup := []*analyticspb.LiveAssistStatsResult{}
	for _, liveAssistResult := range perUserResp.LiveAssistResults {
		if len(liveAssistResult.Attribute.Users) == 0 {
			continue
		}
		userName := liveAssistResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					liveAssistResult.Attribute.Groups = append(liveAssistResult.Attribute.Groups, group)
				}
			}
			liveAssistResultsPerUserPerGroup = append(liveAssistResultsPerUserPerGroup, liveAssistResult)
		}
	}
	return &analyticspb.RetrieveLiveAssistStatsResponse{
		LiveAssistResults:            sortResults(liveAssistResultsPerUserPerGroup),
		TotalRaisedHandCount:         perUserResp.TotalRaisedHandCount,
		TotalRaisedHandAnsweredCount: perUserResp.TotalRaisedHandAnsweredCount,
		TotalWhisperedToAgentCount:   perUserResp.TotalWhisperedToAgentCount,
		TotalWhisperByManagerCount:   perUserResp.TotalWhisperByManagerCount,
	}, nil
}

func convertRowsPerUserToPerGroupLiveAssistStatsResponse(
	perUserResp *analyticspb.RetrieveLiveAssistStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
	// Initialize the per-group live assist stats results.
	liveAssistStatsResultsByGroup := make(map[string]*analyticspb.LiveAssistStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		liveAssistStatsResultsByGroup[group.GetName()] = &analyticspb.LiveAssistStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, liveAssistResults := range perUserResp.LiveAssistResults {
		if len(liveAssistResults.Attribute.Users) == 0 {
			continue
		}
		userName := liveAssistResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := liveAssistStatsResultsByGroup[groupName]; exists {
					liveAssistStatsResultsByGroup[groupName].LiveAssistStats = append(liveAssistStatsResultsByGroup[groupName].LiveAssistStats, liveAssistResults.LiveAssistStats...)
					liveAssistStatsResultsByGroup[groupName].TotalRaisedHandAnsweredCount += liveAssistResults.TotalRaisedHandAnsweredCount
					liveAssistStatsResultsByGroup[groupName].TotalRaisedHandCount += liveAssistResults.TotalRaisedHandCount
					liveAssistStatsResultsByGroup[groupName].TotalWhisperByManagerCount += liveAssistResults.TotalWhisperByManagerCount
					liveAssistStatsResultsByGroup[groupName].TotalWhisperedToAgentCount += liveAssistResults.TotalWhisperedToAgentCount
				}
			}
		}
	}

	// Assemble per-group stats.
	liveAssistStatsResultsForAllGroups := []*analyticspb.LiveAssistStatsResult{}
	for _, liveAssistStatsResults := range liveAssistStatsResultsByGroup {
		if len(liveAssistStatsResults.LiveAssistStats) == 0 {
			continue
		}
		liveAssistStatsResults.LiveAssistStats = convertPerUserToPerGroupLiveAssistStats(liveAssistStatsResults.LiveAssistStats)
		liveAssistStatsResultsForAllGroups = append(liveAssistStatsResultsForAllGroups, liveAssistStatsResults)
	}

	return &analyticspb.RetrieveLiveAssistStatsResponse{
		LiveAssistResults:            sortResults(liveAssistStatsResultsForAllGroups),
		TotalRaisedHandCount:         perUserResp.TotalRaisedHandCount,
		TotalRaisedHandAnsweredCount: perUserResp.TotalRaisedHandAnsweredCount,
		TotalWhisperedToAgentCount:   perUserResp.TotalWhisperedToAgentCount,
		TotalWhisperByManagerCount:   perUserResp.TotalWhisperByManagerCount,
	}, nil
}

func convertPerUserToPerGroupLiveAssistStats(liveAssistStatsPerUser []*analyticspb.LiveAssistStats) []*analyticspb.LiveAssistStats {
	// Initial and append per-frequency stats per group.
	liveAssistStatsByTimestamp := make(map[int64]*analyticspb.LiveAssistStats)
	for _, liveAssistStats := range liveAssistStatsPerUser {
		key := liveAssistStats.TimeRange.StartTimestamp.Seconds
		if _, exists := liveAssistStatsByTimestamp[key]; !exists {
			liveAssistStatsByTimestamp[key] = &analyticspb.LiveAssistStats{
				TimeRange: liveAssistStats.TimeRange,
			}
		}
		liveAssistStatsByTimestamp[key].TotalRaisedHandCount += liveAssistStats.TotalRaisedHandCount
		liveAssistStatsByTimestamp[key].TotalRaisedHandAnsweredCount += liveAssistStats.TotalRaisedHandAnsweredCount
		liveAssistStatsByTimestamp[key].TotalWhisperedToAgentCount += liveAssistStats.TotalWhisperedToAgentCount
		liveAssistStatsByTimestamp[key].TotalWhisperByManagerCount += liveAssistStats.TotalWhisperByManagerCount
	}

	// Assemble per-frequency stats.
	liveAssistStatsPerGroup := []*analyticspb.LiveAssistStats{}
	for _, liveAssistStats := range liveAssistStatsByTimestamp {
		liveAssistStatsPerGroup = append(liveAssistStatsPerGroup, &analyticspb.LiveAssistStats{
			TimeRange:                    liveAssistStats.TimeRange,
			TotalRaisedHandCount:         liveAssistStats.TotalRaisedHandCount,
			TotalRaisedHandAnsweredCount: liveAssistStats.TotalRaisedHandAnsweredCount,
			TotalWhisperedToAgentCount:   liveAssistStats.TotalWhisperedToAgentCount,
			TotalWhisperByManagerCount:   liveAssistStats.TotalWhisperByManagerCount,
		})
	}
	sort.Slice(liveAssistStatsPerGroup, func(i, j int) bool {
		return liveAssistStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < liveAssistStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return liveAssistStatsPerGroup
}
