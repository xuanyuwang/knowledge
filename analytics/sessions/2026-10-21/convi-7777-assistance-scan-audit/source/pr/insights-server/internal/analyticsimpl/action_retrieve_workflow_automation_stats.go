package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	workflowpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/workflowautomation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// RetrieveWorkflowAutomationStats returns usage over the authorized conversation population.
func (a AnalyticsServiceImpl) RetrieveWorkflowAutomationStats(ctx context.Context, request *analyticspb.RetrieveWorkflowAutomationStatsRequest) (*analyticspb.RetrieveWorkflowAutomationStatsResponse, error) {
	req := proto.Clone(request).(*analyticspb.RetrieveWorkflowAutomationStatsRequest)
	if req.GetFilterByAttribute() == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	parent, err := shared.ParseCustomerProfile(req.GetParent())
	if err != nil {
		return nil, err
	}
	buckets, bucketIndices, err := workflowAutomationBuckets(req)
	if err != nil {
		return nil, err
	}
	selectors, err := parseWorkflowAutomationSelectors(req.GetFilterByAttribute().GetWorkflowAutomationFilters(), parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	groupByAgent := slices.Contains(req.GetGroupByAttributeTypes(), analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
	groupByGroup := slices.Contains(req.GetGroupByAttributeTypes(), analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)
	filter := req.GetFilterByAttribute()
	users, err := ParseUserFilterForAnalytics(
		ctx, a.logger, a.userClientRegistry, a.internalUserServiceClientRegistry,
		a.configClient, a.resourceACLHelperProvider.Get(), parent.CustomerID, parent.ProfileID,
		filter.GetUsers(), filter.GetGroups(), groupByAgent,
		shared.IsIncludeDirectGroupMembershipsOnly(filter.GetGroupMembershipFilter()),
		req.GetFilterToAgentsOnly(), req.GetIncludePeerUserStats(), filter.GetExcludeDeactivatedUsers(),
	)
	if err != nil {
		return nil, fmt.Errorf("resolving workflow automation analytics users: %w", err)
	}
	groups, memberships, err := workflowAutomationGroups(users, groupByAgent, groupByGroup)
	if err != nil {
		return nil, err
	}
	response := emptyWorkflowAutomationResponse(groups, buckets, workflowAutomationHintsAvailable(selectors))
	// Role/status filters require the ground-truth user set even for admins.
	restrictUserPopulation := req.GetFilterToAgentsOnly() || filter.GetExcludeDeactivatedUsers()
	useExternalUsers := a.enableExtTableForUserFilter || restrictUserPopulation
	if restrictUserPopulation && len(users.FinalUsers) == 0 {
		return response, nil
	}
	if ApplyUserFilterFromResult(users, &filter.Users, &filter.Groups, useExternalUsers) {
		return response, nil
	}
	rows, err := a.readWorkflowAutomationStats(ctx, req, selectors, memberships, useExternalUsers)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		stats := row.stats()
		if row.groupKey == "" && row.isBucket == 0 {
			response.AggregatedWorkflowAutomationStats = stats
		}
		result := groups[row.groupKey]
		if result == nil {
			continue
		}
		if row.isBucket == 0 {
			result.AggregatedWorkflowAutomationStats = stats
			continue
		}
		if index, exists := bucketIndices[row.bucketMillis]; exists {
			result.WorkflowAutomationStats[index].AggregatedWorkflowAutomationStats = stats
		}
	}
	return response, nil
}

type workflowAutomationSelector struct {
	definitionID string
	versions     []int32
}

func parseWorkflowAutomationSelectors(filters []*analyticspb.WorkflowAutomationFilter, customerID, profileID string) ([]workflowAutomationSelector, error) {
	selectors := make([]workflowAutomationSelector, 0, len(filters))
	for _, filter := range filters {
		name, err := workflowpb.ParseWorkflowAutomationDefinitionName(filter.GetWorkflowAutomationDefinition())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid workflow automation definition: %v", err)
		}
		if name.CustomerID != customerID || name.ProfileID != profileID {
			return nil, status.Error(codes.InvalidArgument, "workflow automation definition must belong to the requested profile")
		}
		selectors = append(selectors, workflowAutomationSelector{definitionID: name.WorkflowAutomationDefinitionID, versions: filter.GetVersions()})
	}
	return selectors, nil
}

// Each conversation contributes once to the total and once to each requested
// group. Membership overlap never feeds back into the independent total.
func workflowAutomationGroups(users *ParseUserFilterResult, byAgent, byGroup bool) (map[string]*analyticspb.WorkflowAutomationStatsResult, map[string][]string, error) {
	groups := map[string]*analyticspb.WorkflowAutomationStatsResult{}
	memberships := map[string][]string{}
	if !byAgent && !byGroup {
		groups[""] = &analyticspb.WorkflowAutomationStatsResult{Attribute: &analyticspb.Attribute{}}
		return groups, memberships, nil
	}
	groupNames := map[string]*userpb.Group{}
	for _, group := range users.GroupsToAggregate {
		copyGroup := proto.Clone(group).(*userpb.Group)
		copyGroup.Members = nil
		groupNames[group.GetName()] = copyGroup
		if byAgent {
			continue
		}
		groups[group.GetName()] = &analyticspb.WorkflowAutomationStatsResult{Attribute: &analyticspb.Attribute{Groups: []*userpb.Group{copyGroup}}}
	}
	for _, user := range users.FinalUsers {
		name, err := userpb.ParseUserName(user.GetName())
		if err != nil {
			return nil, nil, fmt.Errorf("parsing analytics user name: %w", err)
		}
		keys := []string{}
		if !byGroup {
			keys = append(keys, user.GetName())
			groups[user.GetName()] = &analyticspb.WorkflowAutomationStatsResult{Attribute: &analyticspb.Attribute{Users: []*userpb.User{user}}}
		}
		for _, groupName := range users.UserNameToGroupNamesMap[user.GetName()] {
			group := groupNames[groupName]
			if !byGroup || group == nil {
				continue
			}
			key := groupName
			if byAgent {
				key = user.GetName() + "|" + groupName
				groups[key] = &analyticspb.WorkflowAutomationStatsResult{Attribute: &analyticspb.Attribute{Users: []*userpb.User{user}, Groups: []*userpb.Group{group}}}
			}
			if !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
		memberships[name.UserID] = keys
	}
	return groups, memberships, nil
}

func workflowAutomationBuckets(req *analyticspb.RetrieveWorkflowAutomationStatsRequest) ([]*analyticspb.TimeRange, map[int64]int, error) {
	timeRange := req.GetFilterByTimeRange()
	if timeRange.GetStartTimestamp() == nil || timeRange.GetEndTimestamp() == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "both time range timestamps are required")
	}
	if err := timeRange.GetStartTimestamp().CheckValid(); err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid start timestamp: %v", err)
	}
	if err := timeRange.GetEndTimestamp().CheckValid(); err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid end timestamp: %v", err)
	}
	start, end := timeRange.GetStartTimestamp().AsTime(), timeRange.GetEndTimestamp().AsTime()
	if !start.Before(end) {
		return nil, nil, status.Error(codes.InvalidArgument, "start timestamp must precede end timestamp")
	}
	location, err := time.LoadLocation(workflowAutomationTimeZone(req))
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid time zone: %v", err)
	}
	if req.GetMetadata().GetStartDayOfWeek() < analyticspb.Metadata_DAY_OF_WEEK_UNSPECIFIED || req.GetMetadata().GetStartDayOfWeek() > analyticspb.Metadata_SUNDAY {
		return nil, nil, status.Error(codes.InvalidArgument, "invalid start day of week")
	}
	if req.GetFrequency() < analyticspb.Frequency_DAILY || req.GetFrequency() > analyticspb.Frequency_MONTHLY {
		return nil, nil, status.Error(codes.InvalidArgument, "frequency must be daily, weekly, or monthly")
	}
	buckets := []*analyticspb.TimeRange{}
	bucketIndices := map[int64]int{}
	for cursor := workflowAutomationBucketStart(start, req, location); cursor.Before(end); {
		next := cursor.AddDate(0, 0, 1)
		switch req.GetFrequency() {
		case analyticspb.Frequency_WEEKLY:
			next = cursor.AddDate(0, 0, 7)
		case analyticspb.Frequency_MONTHLY:
			next = cursor.AddDate(0, 1, 0)
		}
		clippedStart, clippedEnd := cursor, next
		if clippedStart.Before(start) {
			clippedStart = start
		}
		if clippedEnd.After(end) {
			clippedEnd = end
		}
		bucketIndices[cursor.UnixMilli()] = len(buckets)
		buckets = append(buckets, &analyticspb.TimeRange{StartTimestamp: timestamppb.New(clippedStart), EndTimestamp: timestamppb.New(clippedEnd)})
		if len(buckets) > 10000 {
			return nil, nil, status.Error(codes.InvalidArgument, "time range exceeds 10000 buckets")
		}
		cursor = next
	}
	return buckets, bucketIndices, nil
}

func workflowAutomationTimeZone(req *analyticspb.RetrieveWorkflowAutomationStatsRequest) string {
	if zone := req.GetMetadata().GetTimeZoneId(); zone != "" {
		return zone
	}
	return "UTC"
}

func workflowAutomationBucketStart(value time.Time, req *analyticspb.RetrieveWorkflowAutomationStatsRequest, location *time.Location) time.Time {
	value = value.In(location)
	start := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
	switch req.GetFrequency() {
	case analyticspb.Frequency_MONTHLY:
		return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, location)
	case analyticspb.Frequency_WEEKLY:
		firstDay := int(req.GetMetadata().GetStartDayOfWeek()) % 7
		return start.AddDate(0, 0, -(int(start.Weekday())-firstDay+7)%7)
	default:
		return start
	}
}

func emptyWorkflowAutomationResponse(groups map[string]*analyticspb.WorkflowAutomationStatsResult, buckets []*analyticspb.TimeRange, hintsAvailable bool) *analyticspb.RetrieveWorkflowAutomationStatsResponse {
	emptyRow := workflowAutomationStatsRow{}
	if hintsAvailable {
		emptyRow.ignoredCount = proto.Int64(0)
	}
	response := &analyticspb.RetrieveWorkflowAutomationStatsResponse{AggregatedWorkflowAutomationStats: emptyRow.stats()}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		group.AggregatedWorkflowAutomationStats = emptyRow.stats()
		for _, bucket := range buckets {
			group.WorkflowAutomationStats = append(group.WorkflowAutomationStats, &analyticspb.WorkflowAutomationStats{TimeRange: bucket, AggregatedWorkflowAutomationStats: emptyRow.stats()})
		}
		response.WorkflowAutomationStatsResults = append(response.WorkflowAutomationStatsResults, group)
	}
	return response
}

func workflowAutomationHintsAvailable(selectors []workflowAutomationSelector) bool {
	return !slices.ContainsFunc(selectors, func(selector workflowAutomationSelector) bool { return len(selector.versions) > 0 })
}
