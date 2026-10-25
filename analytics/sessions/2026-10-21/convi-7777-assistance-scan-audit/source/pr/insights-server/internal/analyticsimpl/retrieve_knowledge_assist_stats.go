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
func (a AnalyticsServiceImpl) RetrieveKnowledgeAssistStats(
	ctx context.Context, req *analyticspb.RetrieveKnowledgeAssistStatsRequest,
) (resp *analyticspb.RetrieveKnowledgeAssistStatsResponse, err error) {
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
	listAgentOnly := req.GetFilterToAgentsOnly() // Knowledge assist stats track agent activities
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
		false, // includePeerUserStats - not used for knowledge assist stats
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
		return &analyticspb.RetrieveKnowledgeAssistStatsResponse{}, nil
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveKnowledgeAssistStatsRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterByAttribute)
	var cachedResp analyticspb.RetrieveKnowledgeAssistStatsResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveKnowledgeAssistStats", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveKnowledgeAssistStatsInternalForGroupsInSingleQuery(ctx, req, hasAgentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.retrieveKnowledgeAssistStatsInternal(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveKnowledgeAssistStatsInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveKnowledgeAssistStatsRequest,
	hasAgentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveKnowledgeAssistStatsResponse, error) {
	// Retrieve per-agent knowledge assist stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveKnowledgeAssistStatsRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	perUserResp, err := a.retrieveKnowledgeAssistStatsInternal(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		return convertRowsPerUserToPerUserPerGroupKnowledgeAssistStatsResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	return convertRowsPerUserToPerGroupKnowledgeAssistStatsResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func (a AnalyticsServiceImpl) retrieveKnowledgeAssistStatsInternal(
	ctx context.Context,
	req *analyticspb.RetrieveKnowledgeAssistStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveKnowledgeAssistStatsResponse, error) {
	return a.readKnowledgeAssistStatsFromClickhouse(ctx, req, users)
}

func convertRowsPerUserToPerUserPerGroupKnowledgeAssistStatsResponse(
	perUserResp *analyticspb.RetrieveKnowledgeAssistStatsResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveKnowledgeAssistStatsResponse, error) {
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
	knowledgeAssistStatsResultsPerUserPerGroup := []*analyticspb.KnowledgeAssistStatsResult{}
	for _, knowledgeAssistStatsResult := range perUserResp.KnowledgeAssistStatsResults {
		if len(knowledgeAssistStatsResult.Attribute.Users) == 0 {
			continue
		}
		userName := knowledgeAssistStatsResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					knowledgeAssistStatsResult.Attribute.Groups = append(knowledgeAssistStatsResult.Attribute.Groups, group)
				}
			}
			knowledgeAssistStatsResultsPerUserPerGroup = append(knowledgeAssistStatsResultsPerUserPerGroup, knowledgeAssistStatsResult)
		}
	}
	return &analyticspb.RetrieveKnowledgeAssistStatsResponse{
		KnowledgeAssistStatsResults:                    sortResults(knowledgeAssistStatsResultsPerUserPerGroup),
		GenaiAnswersViewedCount:                        perUserResp.GenaiAnswersViewedCount,
		GenaiAnswersUsedCount:                          perUserResp.GenaiAnswersUsedCount,
		KnowledgeMomentDetectedCount:                   perUserResp.KnowledgeMomentDetectedCount,
		AverageGenaiAnswerUsedCountPerFrequencyPerUser: perUserResp.AverageGenaiAnswerUsedCountPerFrequencyPerUser,
	}, nil
}

func convertRowsPerUserToPerGroupKnowledgeAssistStatsResponse(
	perUserResp *analyticspb.RetrieveKnowledgeAssistStatsResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveKnowledgeAssistStatsResponse, error) {
	// Initialize the per-group knowledge assist stats results.
	knowledgeAssistStatsResultsByGroup := make(map[string]*analyticspb.KnowledgeAssistStatsResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		knowledgeAssistStatsResultsByGroup[group.GetName()] = &analyticspb.KnowledgeAssistStatsResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, knowledgeAssistStatsResults := range perUserResp.KnowledgeAssistStatsResults {
		if len(knowledgeAssistStatsResults.Attribute.Users) == 0 {
			continue
		}
		userName := knowledgeAssistStatsResults.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := knowledgeAssistStatsResultsByGroup[groupName]; exists {
					knowledgeAssistStatsResultsByGroup[groupName].GenaiAnswersViewedCount += knowledgeAssistStatsResults.GenaiAnswersViewedCount
					knowledgeAssistStatsResultsByGroup[groupName].GenaiAnswersUsedCount += knowledgeAssistStatsResults.GenaiAnswersUsedCount
					knowledgeAssistStatsResultsByGroup[groupName].KnowledgeMomentDetectedCount += knowledgeAssistStatsResults.KnowledgeMomentDetectedCount
					knowledgeAssistStatsResultsByGroup[groupName].KnowledgeAssistStats = append(knowledgeAssistStatsResultsByGroup[groupName].KnowledgeAssistStats, knowledgeAssistStatsResults.KnowledgeAssistStats...)
				}
			}
		}
	}

	// Assemble per-group stats.
	knowledgeAssistStatsResultsForAllGroups := []*analyticspb.KnowledgeAssistStatsResult{}
	for _, knowledgeAssistStatsResults := range knowledgeAssistStatsResultsByGroup {
		if len(knowledgeAssistStatsResults.KnowledgeAssistStats) == 0 {
			continue
		}
		userFrequencyCount := len(knowledgeAssistStatsResults.KnowledgeAssistStats)
		knowledgeAssistStatsResults.AverageGenaiAnswerUsedCountPerFrequencyPerUser = safeDivideFloat(knowledgeAssistStatsResults.GenaiAnswersUsedCount, int32(userFrequencyCount))
		knowledgeAssistStatsResults.KnowledgeAssistStats = convertPerUserToPerGroupKnowledgeAssistStats(knowledgeAssistStatsResults.KnowledgeAssistStats)
		knowledgeAssistStatsResultsForAllGroups = append(knowledgeAssistStatsResultsForAllGroups, knowledgeAssistStatsResults)
	}

	return &analyticspb.RetrieveKnowledgeAssistStatsResponse{
		KnowledgeAssistStatsResults:                    sortResults(knowledgeAssistStatsResultsForAllGroups),
		GenaiAnswersViewedCount:                        perUserResp.GenaiAnswersViewedCount,
		GenaiAnswersUsedCount:                          perUserResp.GenaiAnswersUsedCount,
		KnowledgeMomentDetectedCount:                   perUserResp.KnowledgeMomentDetectedCount,
		AverageGenaiAnswerUsedCountPerFrequencyPerUser: perUserResp.AverageGenaiAnswerUsedCountPerFrequencyPerUser,
	}, nil
}

func convertPerUserToPerGroupKnowledgeAssistStats(knowledgeAssistStatsPerUser []*analyticspb.KnowledgeAssistStats) []*analyticspb.KnowledgeAssistStats {
	// Initial and append per-frequency stats per group.
	knowledgeAssistStatsByTimestamp := make(map[int64]*analyticspb.KnowledgeAssistStats)
	userFrequencyCountByTimestamp := make(map[int64]int32)
	for _, knowledgeAssistStats := range knowledgeAssistStatsPerUser {
		key := knowledgeAssistStats.TimeRange.StartTimestamp.Seconds
		if _, exists := knowledgeAssistStatsByTimestamp[key]; !exists {
			knowledgeAssistStatsByTimestamp[key] = &analyticspb.KnowledgeAssistStats{
				TimeRange: knowledgeAssistStats.TimeRange,
			}
		}
		knowledgeAssistStatsByTimestamp[key].GenaiAnswersViewedCount += knowledgeAssistStats.GenaiAnswersViewedCount
		knowledgeAssistStatsByTimestamp[key].GenaiAnswersUsedCount += knowledgeAssistStats.GenaiAnswersUsedCount
		knowledgeAssistStatsByTimestamp[key].KnowledgeMomentDetectedCount += knowledgeAssistStats.KnowledgeMomentDetectedCount
		userFrequencyCountByTimestamp[key]++
	}

	// Assemble per-frequency stats.
	knowledgeAssistStatsPerGroup := []*analyticspb.KnowledgeAssistStats{}
	for key, knowledgeAssistStats := range knowledgeAssistStatsByTimestamp {
		userFrequencyCount := userFrequencyCountByTimestamp[key]
		knowledgeAssistStatsPerGroup = append(knowledgeAssistStatsPerGroup, &analyticspb.KnowledgeAssistStats{
			TimeRange:                                      knowledgeAssistStats.TimeRange,
			GenaiAnswersViewedCount:                        knowledgeAssistStats.GenaiAnswersViewedCount,
			GenaiAnswersUsedCount:                          knowledgeAssistStats.GenaiAnswersUsedCount,
			KnowledgeMomentDetectedCount:                   knowledgeAssistStats.KnowledgeMomentDetectedCount,
			AverageGenaiAnswerUsedCountPerFrequencyPerUser: safeDivideFloat(knowledgeAssistStats.GenaiAnswersUsedCount, userFrequencyCount),
		})
	}
	sort.Slice(knowledgeAssistStatsPerGroup, func(i, j int) bool {
		return knowledgeAssistStatsPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < knowledgeAssistStatsPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return knowledgeAssistStatsPerGroup
}
