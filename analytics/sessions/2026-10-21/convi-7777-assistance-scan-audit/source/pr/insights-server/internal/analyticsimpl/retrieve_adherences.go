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
func (a AnalyticsServiceImpl) RetrieveAdherences(
	ctx context.Context, req *analyticspb.RetrieveAdherencesRequest,
) (*analyticspb.RetrieveAdherencesResponse, error) {
	a.logger.Infof(ctx, "RetrieveAdherences request:\n%s", req)
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	_, err = shared.ParseFrequency(&req.Frequency)
	if err != nil {
		return nil, err
	}

	var (
		users                             []*userpb.User
		groupsToAggregate                 []*userpb.Group
		userNameToGroupNamesMap           map[string][]string
		hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
		hasMomentAsGroupByKey             = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_MOMENT_POLICY)
		hasPolicyAsGroupByKey             = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY)
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
		false, // includePeerUserStats - not supported for adherences
		excludeDeactivatedUsers,
	)
	if err != nil {
		return nil, err
	}

	userNameToGroupNamesMap = result.UserNameToGroupNamesMap
	groupsToAggregate = result.GroupsToAggregate
	users = result.FinalUsers

	if !(shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) ||
		shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)) {
		users = []*userpb.User{}
		groupsToAggregate = []*userpb.Group{}
		userNameToGroupNamesMap = map[string][]string{}
	}

	if shouldEarlyReturn := ApplyUserFilterFromResult(result, &req.FilterByAttribute.Users, &req.FilterByAttribute.Groups, a.enableExtTableForUserFilter); shouldEarlyReturn {
		return &analyticspb.RetrieveAdherencesResponse{}, nil
	}

	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		return a.retrieveAdherencesInternalForGroupsInSingleQuery(ctx, req, hasAgentAsGroupByKey, hasPolicyAsGroupByKey, hasMomentAsGroupByKey, users, groupsToAggregate, userNameToGroupNamesMap)
	}
	return a.readAdherencesFromClickhouse(ctx, req, users)
}

func (a AnalyticsServiceImpl) retrieveAdherencesInternalForGroupsInSingleQuery(
	ctx context.Context,
	req *analyticspb.RetrieveAdherencesRequest,
	hasAgentAsGroupByKey, hasPolicyAsGroupByKey, hasMomentAsGroupByKey bool,
	users []*userpb.User,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAdherencesResponse, error) {
	// Retrieve per-agent adherence stats for ALL users fetched from the given groups.
	perUserReqForAll := proto.Clone(req).(*analyticspb.RetrieveAdherencesRequest)
	perUserReqForAll.GroupByAttributeTypes = shared.ConvertGroupToAgentAttributeType(req.GroupByAttributeTypes)
	if len(req.FilterByAttribute.Groups) > 0 {
		// Only add user filters when group filter is not empty to avoid user list being too long.
		perUserReqForAll.FilterByAttribute.Users = append([]*userpb.User{}, users...)
	}
	perUserResp, err := a.readAdherencesFromClickhouse(ctx, perUserReqForAll, users)
	if err != nil {
		return nil, err
	}

	if hasAgentAsGroupByKey {
		// Only append group info onto attribute.
		return convertRowsPerUserToPerUserPerGroupAdherencesResponse(perUserResp, users, groups, userNameToGroupNamesMap)
	}
	if hasMomentAsGroupByKey {
		// Convert per-agent/per-moment to per-group/per-moment stats.
		return convertRowsPerUserPerMomentToPerGroupPerMomentAdherencesResponse(perUserResp, groups, userNameToGroupNamesMap)
	}
	if hasPolicyAsGroupByKey {
		// Convert per-agent/per-policy to per-group/per-policy stats.
		return convertRowsPerUserPerPolicyToPerGroupPerPolicyAdherencesResponse(perUserResp, groups, userNameToGroupNamesMap)
	}
	// Convert per-agent to per-group stats.
	return convertRowsPerUserToPerGroupAdherencesResponse(perUserResp, groups, userNameToGroupNamesMap)
}

func convertRowsPerUserToPerUserPerGroupAdherencesResponse(
	perUserResp *analyticspb.RetrieveAdherencesResponse,
	users []*userpb.User, groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAdherencesResponse, error) {
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
	adherenceResultsPerUserPerGroup := []*analyticspb.AdherenceResult{}
	for _, adherenceResult := range perUserResp.AdherenceResults {
		if len(adherenceResult.Attribute.Users) == 0 {
			continue
		}
		userName := adherenceResult.Attribute.Users[0].Name
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if group, exists := groupNameToGroupMap[groupName]; exists {
					group.Members = []*userpb.GroupMembership{}
					adherenceResult.Attribute.Groups = append(adherenceResult.Attribute.Groups, group)
				}
			}
			adherenceResultsPerUserPerGroup = append(adherenceResultsPerUserPerGroup, adherenceResult)
		}
	}
	return &analyticspb.RetrieveAdherencesResponse{
		AdherenceResults: sortResults(adherenceResultsPerUserPerGroup),
		TotalSdxCount:    perUserResp.TotalSdxCount,
		AverageAdherence: perUserResp.AverageAdherence,
	}, nil
}

func convertRowsPerUserPerMomentToPerGroupPerMomentAdherencesResponse(
	perUserResp *analyticspb.RetrieveAdherencesResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAdherencesResponse, error) {
	// Initialize the per-group and per-moment adherence results.
	groupNameToGroup := map[string]*userpb.Group{}
	for _, group := range groups {
		groupNameToGroup[group.Name] = &userpb.Group{
			Name:        group.Name,
			DisplayName: group.DisplayName,
			Type:        group.Type,
		}
	}

	// Append per-user stats to per-group stats.
	adherenceResultsByGroupAndMoment := make(map[string]*analyticspb.AdherenceResult)
	for _, adherenceResult := range perUserResp.AdherenceResults {
		if len(adherenceResult.Attribute.Users) == 0 || len(adherenceResult.Attribute.Moments) == 0 {
			continue
		}
		userName := adherenceResult.Attribute.Users[0].GetName()
		momentName := ""
		if len(adherenceResult.Attribute.Moments) > 0 {
			momentName = adherenceResult.Attribute.Moments[0].GetName()
		}
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				key := groupName + momentName
				if _, exists := adherenceResultsByGroupAndMoment[key]; !exists {
					group, groupExists := groupNameToGroup[groupName]
					if !groupExists {
						continue
					}
					adherenceResultsByGroupAndMoment[key] = &analyticspb.AdherenceResult{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{group},
							// Keep the same moments from per-user results.
							Moments: adherenceResult.Attribute.Moments,
						},
					}
				}
				adherenceResultsByGroupAndMoment[key].TotalSdxCount += adherenceResult.TotalSdxCount
				adherenceResultsByGroupAndMoment[key].Adherences = append(adherenceResultsByGroupAndMoment[key].Adherences, adherenceResult.Adherences...)
			}
		}
	}

	// Assemble per-group stats.
	adherenceResultsForAllGroups := []*analyticspb.AdherenceResult{}
	totalSdxCount := int32(0)
	totalDdxCount := float32(0)
	for _, adherenceResult := range adherenceResultsByGroupAndMoment {
		if adherenceResult.TotalSdxCount == 0 {
			continue
		}
		adherenceResult.Adherences = convertPerUserToPerGroupAdherences(adherenceResult.Adherences)

		// Calculate AverageAdherence for the group/moment
		groupTotalSdx := int32(0)
		groupTotalDdx := float32(0)
		for _, adj := range adherenceResult.Adherences {
			groupTotalSdx += adj.TotalSdxCount
			groupTotalDdx += float32(adj.TotalSdxCount) * adj.Adherence
		}
		adherenceResult.AverageAdherence = safeDivideFloat(groupTotalDdx, float32(groupTotalSdx))

		adherenceResultsForAllGroups = append(adherenceResultsForAllGroups, adherenceResult)
		totalSdxCount += adherenceResult.TotalSdxCount
		totalDdxCount += groupTotalDdx
	}

	return &analyticspb.RetrieveAdherencesResponse{
		AdherenceResults: sortResults(adherenceResultsForAllGroups),
		TotalSdxCount:    totalSdxCount,
		AverageAdherence: safeDivideFloat(totalDdxCount, float32(totalSdxCount)),
	}, nil
}

func convertRowsPerUserPerPolicyToPerGroupPerPolicyAdherencesResponse(
	perUserResp *analyticspb.RetrieveAdherencesResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAdherencesResponse, error) {
	// Initialize the per-group and per-policy adherence results.
	groupNameToGroup := map[string]*userpb.Group{}
	for _, group := range groups {
		groupNameToGroup[group.Name] = &userpb.Group{
			Name:        group.Name,
			DisplayName: group.DisplayName,
			Type:        group.Type,
		}
	}
	// Append per-user stats to per-group stats.
	adherenceResultsByGroupAndPolicy := make(map[string]*analyticspb.AdherenceResult)
	for _, adherenceResult := range perUserResp.AdherenceResults {
		if len(adherenceResult.Attribute.Users) == 0 || len(adherenceResult.Attribute.AttributePolicies) == 0 {
			continue
		}
		userName := adherenceResult.Attribute.Users[0].GetName()
		policyName := ""
		if len(adherenceResult.Attribute.AttributePolicies) > 0 {
			policyName = adherenceResult.Attribute.AttributePolicies[0].GetName()
		}
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				key := groupName + policyName
				if _, exists := adherenceResultsByGroupAndPolicy[key]; !exists {
					group, groupExists := groupNameToGroup[groupName]
					if !groupExists {
						continue
					}
					adherenceResultsByGroupAndPolicy[key] = &analyticspb.AdherenceResult{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{group},
							// Keep the same policies from per-user results.
							AttributePolicies: adherenceResult.Attribute.AttributePolicies,
							Actions:           adherenceResult.Attribute.Actions,
						},
					}
				}
				adherenceResultsByGroupAndPolicy[key].TotalSdxCount += adherenceResult.TotalSdxCount
				adherenceResultsByGroupAndPolicy[key].Adherences = append(adherenceResultsByGroupAndPolicy[key].Adherences, adherenceResult.Adherences...)
			}
		}
	}

	// Assemble per-group stats.
	adherenceResultsForAllGroups := []*analyticspb.AdherenceResult{}
	totalSdxCount := int32(0)
	totalDdxCount := float32(0)
	for _, adherenceResult := range adherenceResultsByGroupAndPolicy {
		if adherenceResult.TotalSdxCount == 0 {
			continue
		}
		adherenceResult.Adherences = convertPerUserToPerGroupAdherences(adherenceResult.Adherences)

		// Calculate AverageAdherence for the group/policy
		groupTotalSdx := int32(0)
		groupTotalDdx := float32(0)
		for _, adj := range adherenceResult.Adherences {
			groupTotalSdx += adj.TotalSdxCount
			groupTotalDdx += float32(adj.TotalSdxCount) * adj.Adherence
		}
		adherenceResult.AverageAdherence = safeDivideFloat(groupTotalDdx, float32(groupTotalSdx))

		adherenceResultsForAllGroups = append(adherenceResultsForAllGroups, adherenceResult)
		totalSdxCount += adherenceResult.TotalSdxCount
		totalDdxCount += groupTotalDdx
	}

	return &analyticspb.RetrieveAdherencesResponse{
		AdherenceResults: sortResults(adherenceResultsForAllGroups),
		TotalSdxCount:    totalSdxCount,
		AverageAdherence: safeDivideFloat(totalDdxCount, float32(totalSdxCount)),
	}, nil
}

func convertRowsPerUserToPerGroupAdherencesResponse(
	perUserResp *analyticspb.RetrieveAdherencesResponse,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAdherencesResponse, error) {
	// Initialize the per-group adherence results.
	adherenceResultsByGroup := make(map[string]*analyticspb.AdherenceResult)
	for _, group := range groups {
		group.Members = []*userpb.GroupMembership{}
		adherenceResultsByGroup[group.GetName()] = &analyticspb.AdherenceResult{
			Attribute: &analyticspb.Attribute{
				Groups: []*userpb.Group{group},
			},
		}
	}

	// Append per-user stats to per-group stats.
	for _, adherenceResult := range perUserResp.AdherenceResults {
		if len(adherenceResult.Attribute.Users) == 0 {
			continue
		}
		userName := adherenceResult.Attribute.Users[0].GetName()
		if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
			for _, groupName := range groupNames {
				if _, exists := adherenceResultsByGroup[groupName]; exists {
					adherenceResultsByGroup[groupName].TotalSdxCount += adherenceResult.TotalSdxCount
					adherenceResultsByGroup[groupName].Adherences = append(adherenceResultsByGroup[groupName].Adherences, adherenceResult.Adherences...)
				}
			}
		}
	}

	// Assemble per-group stats.
	adherenceResultsForAllGroups := []*analyticspb.AdherenceResult{}
	totalSdxCount := int32(0)
	totalDdxCount := float32(0)
	for _, adherenceResult := range adherenceResultsByGroup {
		if adherenceResult.TotalSdxCount == 0 {
			continue
		}
		adherenceResult.Adherences = convertPerUserToPerGroupAdherences(adherenceResult.Adherences)

		// Calculate AverageAdherence for the group
		groupTotalSdx := int32(0)
		groupTotalDdx := float32(0)
		for _, adj := range adherenceResult.Adherences {
			groupTotalSdx += adj.TotalSdxCount
			groupTotalDdx += float32(adj.TotalSdxCount) * adj.Adherence
		}
		adherenceResult.AverageAdherence = safeDivideFloat(groupTotalDdx, float32(groupTotalSdx))

		adherenceResultsForAllGroups = append(adherenceResultsForAllGroups, adherenceResult)
		totalSdxCount += adherenceResult.TotalSdxCount
		totalDdxCount += groupTotalDdx
	}

	return &analyticspb.RetrieveAdherencesResponse{
		AdherenceResults: sortResults(adherenceResultsForAllGroups),
		TotalSdxCount:    totalSdxCount,
		AverageAdherence: safeDivideFloat(totalDdxCount, float32(totalSdxCount)),
	}, nil
}

func convertPerUserToPerGroupAdherences(adherencesPerUser []*analyticspb.Adherence) []*analyticspb.Adherence {
	// Initial and append per-frequency stats per group.
	type aggregate struct {
		sdx       int32
		ddx       float32
		timeRange *analyticspb.TimeRange
	}
	aggs := make(map[int64]*aggregate)

	for _, adherence := range adherencesPerUser {
		key := adherence.TimeRange.StartTimestamp.Seconds
		if _, exists := aggs[key]; !exists {
			aggs[key] = &aggregate{
				timeRange: adherence.TimeRange,
			}
		}
		aggs[key].sdx += adherence.TotalSdxCount
		aggs[key].ddx += float32(adherence.TotalSdxCount) * adherence.Adherence
	}

	// Assemble per-frequency stats.
	adherencesPerGroup := []*analyticspb.Adherence{}
	for _, agg := range aggs {
		adherencesPerGroup = append(adherencesPerGroup, &analyticspb.Adherence{
			TimeRange:     agg.timeRange,
			TotalSdxCount: agg.sdx,
			Adherence:     safeDivideFloat(agg.ddx, float32(agg.sdx)),
		})
	}
	sort.Slice(adherencesPerGroup, func(i, j int) bool {
		return adherencesPerGroup[i].TimeRange.StartTimestamp.GetSeconds() < adherencesPerGroup[j].TimeRange.StartTimestamp.GetSeconds()
	})
	return adherencesPerGroup
}
