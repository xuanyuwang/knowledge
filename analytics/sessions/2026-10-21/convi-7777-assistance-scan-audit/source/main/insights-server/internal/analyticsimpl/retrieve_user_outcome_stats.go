package analyticsimpl

import (
	"context"
	"fmt"

	"github.com/cresta/shared-go/container/slice"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/shared/scoring"
	"github.com/cresta/go-servers/shared/utils"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"

	"google.golang.org/protobuf/proto"
)

const (
	FieldDefinitionIDColumn           = "field_definition_id"
	BucketTimeColumn                  = "bucket_time"
	includeDirectGroupMembershipsOnly = false // For now we don't support direct group memberships only
)

// When grouping by group, only group, time_range, and outcome_field are allowed.
var allowedUserOutcomeAttributeTypesForGroupByGroup = set.NewFromSlice([]analyticspb.UserOutcomeAttributeType{
	analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_GROUP,
	analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
	analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
})

func (a *AnalyticsServiceImpl) RetrieveUserOutcomeStats(ctx context.Context, req *analyticspb.RetrieveUserOutcomeStatsRequest) (*analyticspb.RetrieveUserOutcomeStatsResponse, error) {
	a.logger.Infof(ctx, "RetrieveUserOutcomeStats RPC starts, req = %v", req)

	parent, err := shared.ParseCustomerProfile(req.GetParent())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse parent: %v", err)
	}

	err = validateUserOutcomeStatsRequest(req)
	if err != nil {
		return nil, err
	}

	groupByAttributeTypes := req.GetGroupBy().GetAttributeTypes()
	requestFieldDefinitionNames := req.GetFilter().GetOutcomeFieldDefinitionNames()

	users, groupsToAggregate, userNameToGroupNamesMap, shouldEarlyReturn, err := a.resolveUserOutcomeStatsUsersAndGroups(ctx, req, parent)
	if err != nil {
		return nil, err
	} else if shouldEarlyReturn {
		return &analyticspb.RetrieveUserOutcomeStatsResponse{
			UserOutcomeStatsSummary: &analyticspb.UserOutcomeStatsSummary{},
			UserOutcomeStats:        []*analyticspb.UserOutcomeStats{},
		}, nil
	}

	filteredFieldDefinitions, err := a.filterFieldDefinitions(
		ctx,
		parent,
		requestFieldDefinitionNames,
	)
	if err != nil {
		return nil, err
	}
	if len(filteredFieldDefinitions) == 0 {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"no field definitions found for the given field definition names: %v.",
			requestFieldDefinitionNames,
		)
	}

	if err := validateAggregationModeAgainstFieldDefinitions(req.GetGroupBy().GetAggregationMode(), filteredFieldDefinitions); err != nil {
		return nil, err
	}

	filteredFieldDefinitionNames := fn.Map(filteredFieldDefinitions, func(fd *metadatapb.FieldDefinition) string {
		return fd.GetName()
	})
	lowerIsBetterFieldDefs := extractLowerIsBetterFieldDefs(filteredFieldDefinitions)

	querySpec := &UserOutcomeQuerySpec{
		GroupByAttributeTypes:             groupByAttributeTypes,
		Filter:                            req.GetFilter(),
		GroupBy:                           req.GetGroupBy(),
		Parent:                            parent,
		Users:                             users,
		Groups:                            groupsToAggregate,
		UserNameToGroupNamesMap:           userNameToGroupNamesMap,
		IncludeDirectGroupMembershipsOnly: includeDirectGroupMembershipsOnly,
		FieldDefinitionNames:              filteredFieldDefinitionNames,
		Metadata:                          req.GetMetadata(),
		LowerIsBetterFieldDefinitions:     lowerIsBetterFieldDefs,
		UseExtTable:                       a.enableExtTableForUserFilter,
	}

	if slice.Contains(groupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_GROUP) {
		return a.getAggregatedOutcomeStatsPerGroup(ctx, querySpec)
	}

	return a.getAggregatedOutcomeStats(ctx, querySpec)
}

func (a *AnalyticsServiceImpl) resolveUserOutcomeStatsUsersAndGroups(
	ctx context.Context,
	req *analyticspb.RetrieveUserOutcomeStatsRequest,
	parent *shared.CustomerProfile,
) ([]*userpb.User, []*userpb.Group, map[string][]string, bool, error) {
	excludeDeactivatedUsers := req.GetFilter().GetExcludeDeactivatedUsers()
	groupByAttributeTypes := req.GetGroupBy().GetAttributeTypes()

	requestUsers := fn.Map(req.GetFilter().GetUsers(), func(userName string) *userpb.User {
		return &userpb.User{
			Name: userName,
		}
	})
	requestGroups := fn.Map(req.GetFilter().GetGroups(), func(groupName string) *userpb.Group {
		return &userpb.Group{
			Name: groupName,
		}
	})

	var (
		usersToQuery            []*userpb.User
		groupsToAggregate       []*userpb.Group
		userNameToGroupNamesMap map[string][]string
		hasUserAsGroupByKey     = slice.Contains(groupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER)
		hasGroupAsGroupByKey    = slice.Contains(groupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_GROUP)
	)

	if hasUserAsGroupByKey || hasGroupAsGroupByKey || len(requestUsers) > 0 || len(requestGroups) > 0 {
		result, err := ParseUserFilterForAnalytics(
			ctx,
			a.logger,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			a.configClient,
			a.resourceACLHelperProvider.Get(),
			parent.CustomerID,
			parent.ProfileID,
			requestUsers,
			requestGroups,
			hasUserAsGroupByKey,
			includeDirectGroupMembershipsOnly,
			true,  // listAgentOnly
			false, // includePeerUserStats
			excludeDeactivatedUsers,
		)
		if err != nil {
			return nil, nil, nil, true, err
		}

		a.logger.Infof(ctx, "[resolveUserOutcomeStatsUsersAndGroups] ParseUserFilterForAnalytics result: ShouldQueryAllUsers=%v FinalUsers length=%d", result.ShouldQueryAllUsers, len(result.FinalUsers))

		userNameToGroupNamesMap = result.UserNameToGroupNamesMap
		groupsToAggregate = result.GroupsToAggregate

		if shouldEarlyReturn := ApplyUserFilterFromResult(result, &usersToQuery, &requestGroups, a.enableExtTableForUserFilter); shouldEarlyReturn {
			a.logger.Infof(ctx, "[resolveUserOutcomeStatsUsersAndGroups] early return (no users to query)")
			return nil, nil, nil, true, nil
		}
		a.logger.Infof(ctx, "[resolveUserOutcomeStatsUsersAndGroups] hasUserAsGroupByKey=%v hasGroupAsGroupByKey=%v usersToQuery length=%d", hasUserAsGroupByKey, hasGroupAsGroupByKey, len(usersToQuery))
	} else {
		groupsToAggregate = []*userpb.Group{}
		userNameToGroupNamesMap = map[string][]string{}
	}

	return usersToQuery, groupsToAggregate, userNameToGroupNamesMap, false, nil
}

func (a *AnalyticsServiceImpl) getAggregatedOutcomeStatsPerGroup(
	ctx context.Context,
	querySpec *UserOutcomeQuerySpec,
) (*analyticspb.RetrieveUserOutcomeStatsResponse, error) {
	querySpec.GroupByAttributeTypes = append(
		fn.Filter(querySpec.GroupByAttributeTypes, func(groupByAttribute analyticspb.UserOutcomeAttributeType) bool {
			return groupByAttribute != analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_GROUP
		}),
		analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
	)

	perUserInternalStats, err := a.getInternalAggregatedOutcomeStats(ctx, querySpec)
	if err != nil {
		return nil, err
	}

	perGroupInternalStats, err := groupInternalUserOutcomeStats(
		perUserInternalStats,
		querySpec.Groups,
		querySpec.UserNameToGroupNamesMap,
	)
	if err != nil {
		return nil, err
	}

	response := transformInternalUserOutcomeStatsToResponse(perGroupInternalStats, querySpec.GroupBy.GetAggregationMode())

	// Avoid double counting for multi group memberships.
	response.UserOutcomeStatsSummary.RecordCount = sumInternalRecordCount(perUserInternalStats)
	return response, nil
}

func (a *AnalyticsServiceImpl) getInternalAggregatedOutcomeStats(
	ctx context.Context,
	spec *UserOutcomeQuerySpec,
) ([]*internalUserOutcomeStats, error) {
	if spec.UseExtTable && len(spec.Users) > 0 {
		userIDs, err := extractUserIDsFromUsers(spec.Users)
		if err != nil {
			return nil, err
		}
		extTable, err := buildUsersExtTable(userIDs, extTableAgentFilter, extTableAgentFilterColumn)
		if err != nil {
			return nil, err
		}
		ctx = attachExtTablesToContext(ctx, extTable)
	}

	query, queryArgs, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
	if err != nil {
		return nil, err
	}

	chRows, err := a.readOutcomeStatsFromClickhouse(ctx, spec.Parent, query, queryArgs, groupByKeys)
	if err != nil {
		return nil, err
	}
	a.logger.Infof(ctx, "[ReadOutcomeStatsFromClickhouse] clickhouse rows: %v", chRows)

	internalStats := transformCHRowsToInternalUserOutcomeStats(chRows, spec)
	if slice.Contains(spec.GroupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER) {
		internalStats, err = aggregateTopAgentsInternalUserOutcomeStats(internalStats, spec.GroupBy.GetAggregationMode(), spec.LowerIsBetterFieldDefinitions)
		if err != nil {
			return nil, err
		}
	}
	return internalStats, nil
}

func sumInternalRecordCount(stats []*internalUserOutcomeStats) int32 {
	totalRecordCount := int32(0)
	for _, stat := range stats {
		totalRecordCount += stat.RecordCount
	}
	return totalRecordCount
}

func validateUserOutcomeStatsRequest(req *analyticspb.RetrieveUserOutcomeStatsRequest) error {
	if req.GetFilter() == nil {
		return status.Errorf(codes.InvalidArgument, "filter is required.")
	}

	if req.GetGroupBy() == nil {
		return status.Errorf(codes.InvalidArgument, "group_by is required.")
	}

	if req.GetFilter().GetTimeRange() == nil {
		return status.Errorf(codes.InvalidArgument, "time_range is required.")
	}

	if req.GetGroupBy().GetAggregationMode() == analyticspb.AggregationMode_AGGREGATION_MODE_UNSPECIFIED {
		return status.Errorf(codes.InvalidArgument, "aggregation_mode is required.")
	}

	groupByAttributeTypes := req.GetGroupBy().GetAttributeTypes()
	hasUserAsGroupByKey := slice.Contains(groupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER)
	hasGroupAsGroupByKey := slice.Contains(groupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_GROUP)

	if len(groupByAttributeTypes) == 0 {
		return status.Errorf(codes.InvalidArgument, "group_by_attribute_types is required.")
	}

	if slice.Contains(groupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER) && hasUserAsGroupByKey {
		return status.Errorf(codes.InvalidArgument, "Not supporting agent tier attribute with user group attribute: %v.", groupByAttributeTypes)
	}

	if hasGroupAsGroupByKey {
		for _, a := range groupByAttributeTypes {
			if !allowedUserOutcomeAttributeTypesForGroupByGroup.Has(a) {
				return status.Errorf(codes.InvalidArgument,
					"when grouping by group, only time_range and outcome_field may be combined with group; got %v", groupByAttributeTypes)
			}
		}
	}

	return nil
}

func (a *AnalyticsServiceImpl) getAggregatedOutcomeStats(
	ctx context.Context,
	spec *UserOutcomeQuerySpec,
) (*analyticspb.RetrieveUserOutcomeStatsResponse, error) {
	internalStats, err := a.getInternalAggregatedOutcomeStats(ctx, spec)
	if err != nil {
		return nil, err
	}
	return transformInternalUserOutcomeStatsToResponse(internalStats, spec.GroupBy.GetAggregationMode()), nil
}

func aggregateTopAgentsInternalUserOutcomeStats(
	internalStats []*internalUserOutcomeStats,
	aggregationMode analyticspb.AggregationMode,
	lowerIsBetterFieldDefinitions set.Set[string],
) ([]*internalUserOutcomeStats, error) {
	groupByKeyToOutcomeStats := utils.GroupBy(internalStats, internalUserOutcomeStatsGroupedByToStringKey)

	aggregatedStats := []*internalUserOutcomeStats{}
	conversationVolumeMetric := func(userOutcomeStats *internalUserOutcomeStats) int32 {
		return userOutcomeStats.RecordCount
	}
	for _, outcomeStatsSlice := range groupByKeyToOutcomeStats {
		// Check if this outcome field group is lower-is-better (e.g., AHT, churn).
		// Negate metric so PartitionUsingVolumeAndMetric (ascending sort) places low-value agents in TOP.
		lowerIsBetter := lowerIsBetterFieldDefinitions != nil && len(outcomeStatsSlice) > 0 &&
			lowerIsBetterFieldDefinitions.Has(outcomeStatsSlice[0].GroupBy.GetOutcomeFieldDefinitionName())
		getValueMetric := func(userOutcomeStats *internalUserOutcomeStats) float32 {
			v := float32(computeOutcomeStatValue(userOutcomeStats, aggregationMode))
			if lowerIsBetter {
				return -v
			}
			return v
		}
		agentParitions := utils.PartitionUsingVolumeAndMetric(
			outcomeStatsSlice,
			getValueMetric,
			conversationVolumeMetric,
			[]float32{bottomAgentPercentage, bottomAgentPercentage + AvgAgentPercentage},
		)
		if len(agentParitions) != 3 {
			return nil, fmt.Errorf("expected 3 partitions, got %d", len(agentParitions))
		}
		groupedBy := outcomeStatsSlice[0].GroupBy
		for i, partition := range agentParitions {
			aggregatedStats = append(aggregatedStats, createTieredInternalUserOutcomeStatsObject(partition, groupedBy, agentTiers[i]))
		}
	}

	return aggregatedStats, nil
}

func internalUserOutcomeStatsGroupedByToStringKey(userOutcomeStats *internalUserOutcomeStats) string {
	return outcomeStatsKey(
		userOutcomeStats.GroupBy.GetGroup(),
		userOutcomeStats.GroupBy.GetTimeRange(),
		userOutcomeStats.GroupBy.GetOutcomeFieldDefinitionName(),
	)
}

func createTieredInternalUserOutcomeStatsObject(
	outcomeStats []*internalUserOutcomeStats,
	groupedBy *analyticspb.UserOutcomeStatsGroupBy,
	agentTier analyticspb.AgentTier,
) *internalUserOutcomeStats {
	groupedByCopy := proto.Clone(groupedBy).(*analyticspb.UserOutcomeStatsGroupBy)
	groupedByCopy.AgentTier = agentTier
	groupedByCopy.User = ""

	if len(outcomeStats) == 0 {
		return &internalUserOutcomeStats{
			GroupBy: groupedByCopy,
		}
	}

	aggregatedOutcomeStats := aggregateOutcomeStats(outcomeStats)
	aggregatedOutcomeStats.GroupBy = groupedByCopy
	return aggregatedOutcomeStats
}

func aggregateOutcomeStats(outcomeStats []*internalUserOutcomeStats) *internalUserOutcomeStats {
	aggregatedOutcomeStats := &internalUserOutcomeStats{}
	for _, stat := range outcomeStats {
		aggregatedOutcomeStats.RecordCount += stat.RecordCount
		aggregatedOutcomeStats.ValueSum += stat.ValueSum
		aggregatedOutcomeStats.WeightSum += stat.WeightSum
		aggregatedOutcomeStats.WeightedValueSum += stat.WeightedValueSum
	}
	return aggregatedOutcomeStats
}

// filterFieldDefinitions returns the outcome field definitions for the parent profile, excluding system fields
// (outcome_time, user_id) and optionally filtering to the provided field definition names.
func (a *AnalyticsServiceImpl) filterFieldDefinitions(ctx context.Context, parent *shared.CustomerProfile, fieldDefinitionNames []string) ([]*metadatapb.FieldDefinition, error) {
	outcomeResourceTypeName := metadatapb.ResourceTypeName{
		CustomerID:     parent.CustomerID,
		ProfileID:      parent.ProfileID,
		ResourceTypeID: useroutcomes.UserOutcomeResourceTypeID,
	}
	outcomeResourceType, err := a.metadataFrameworkClientRegistry.Client(ctx).GetResourceType(ctx, &metadatapb.GetResourceTypeRequest{
		Name: outcomeResourceTypeName.String(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get outcome resource type: %v", err)
	}

	fieldDefinitionNamesSet := set.NewFromSlice(fieldDefinitionNames)
	filteredFieldDefinitions := outcomeResourceType.GetResourceType().GetFields()

	// Exclude system fields (outcome_time, user_id); they are dimensions, not outcome metrics, and must not be
	// matched against outcome_field_definition_names in the request.
	filteredFieldDefinitions = fn.Filter(filteredFieldDefinitions, func(fd *metadatapb.FieldDefinition) bool {
		name, err := metadatapb.ParseFieldDefinitionName(fd.GetName())
		if err != nil {
			return true
		}
		return name.FieldDefinitionID != useroutcomes.TimeFieldID && name.FieldDefinitionID != useroutcomes.UserIDFieldID
	})

	if fieldDefinitionNamesSet.Len() > 0 {
		filteredFieldDefinitions = fn.Filter(filteredFieldDefinitions, func(fieldDefinition *metadatapb.FieldDefinition) bool {
			return fieldDefinitionNamesSet.Has(fieldDefinition.GetName())
		})
	}

	return filteredFieldDefinitions, nil
}

// validateAggregationModeAgainstFieldDefinitions rejects requests asking for SUM aggregation on field
// definitions that did not opt into total aggregation.
func validateAggregationModeAgainstFieldDefinitions(aggregationMode analyticspb.AggregationMode, fieldDefinitions []*metadatapb.FieldDefinition) error {
	if aggregationMode != analyticspb.AggregationMode_AGGREGATION_MODE_SUM {
		return nil
	}

	disallowedFieldNames := []string{}
	for _, fd := range fieldDefinitions {
		if !fd.GetFieldMetadata().GetUserOutcomesFieldMetadataConfig().GetAllowTotalAggregation() {
			disallowedFieldNames = append(disallowedFieldNames, fd.GetDisplayName())
		}
	}
	if len(disallowedFieldNames) > 0 {
		return status.Errorf(
			codes.InvalidArgument,
			"aggregation_mode SUM is not allowed for field definitions that do not opt into total aggregation: %v",
			disallowedFieldNames,
		)
	}
	return nil
}

// extractLowerIsBetterFieldDefs identifies field definitions with lower-is-better
// directionality (TIME, CHURN outcome types) from the given field definitions.
func extractLowerIsBetterFieldDefs(fieldDefinitions []*metadatapb.FieldDefinition) set.Set[string] {
	lowerIsBetterFieldDefs := set.New[string]()
	for _, fd := range fieldDefinitions {
		outcomeType := fd.GetFieldMetadata().GetUserOutcomesFieldMetadataConfig().GetOutcomeType()
		if outcomeType != enumspb.OutcomeTypeEnum_OUTCOME_TYPE_ENUM_UNSPECIFIED && scoring.IsLowerBetterOutcomeTypeEnum(outcomeType) {
			lowerIsBetterFieldDefs.Add(fd.GetName())
		}
	}
	return lowerIsBetterFieldDefs
}
