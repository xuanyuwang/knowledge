package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type conversationAssistanceUsageInput struct {
	population              []conversationAssistanceUsagePopulationRow
	users                   *ParseUserFilterResult
	includeBotConversations bool
}

type conversationAssistanceUsagePopulationRow struct {
	conversationID   string
	agentUserID      string
	conversationTime time.Time
}

func (a AnalyticsServiceImpl) RetrieveConversationAssistanceUsageStats(
	ctx context.Context, req *analyticspb.RetrieveConversationAssistanceUsageStatsRequest,
) (*analyticspb.RetrieveConversationAssistanceUsageStatsResponse, error) {
	if err := validateConversationAssistanceUsageRequest(req); err != nil {
		return nil, err
	}
	input, err := a.readConversationAssistanceUsagePopulation(ctx, req)
	if err != nil {
		return nil, err
	}
	evidence, err := a.readConversationAssistanceUsageEvidence(ctx, req.Parent, input.population, input.includeBotConversations)
	if err != nil {
		return nil, err
	}
	return aggregateConversationAssistanceUsage(req, input.population, evidence, input.users)
}

func (a AnalyticsServiceImpl) readConversationAssistanceUsagePopulation(
	ctx context.Context, req *analyticspb.RetrieveConversationAssistanceUsageStatsRequest,
) (*conversationAssistanceUsageInput, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	attribute := &analyticspb.Attribute{}
	if req.FilterByAttribute != nil {
		attribute = proto.Clone(req.FilterByAttribute).(*analyticspb.Attribute)
	}
	shared.CountFilter(ctx, attribute)
	userFilter, err := ParseUserFilterForAnalytics(
		ctx, a.logger, a.userClientRegistry, a.internalUserServiceClientRegistry,
		a.configClient, a.resourceACLHelperProvider.Get(), parent.CustomerID, parent.ProfileID,
		attribute.GetUsers(), attribute.GetGroups(),
		shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT),
		shared.IsIncludeDirectGroupMembershipsOnly(attribute.GetGroupMembershipFilter()),
		req.GetFilterToAgentsOnly(), req.GetIncludePeerUserStats(), attribute.GetExcludeDeactivatedUsers(),
	)
	if err != nil {
		return nil, fmt.Errorf("resolve assistance usage population users: %w", err)
	}
	queryUserFilter := *userFilter
	restrictUserPopulation := req.GetFilterToAgentsOnly() || attribute.GetExcludeDeactivatedUsers()
	if restrictUserPopulation {
		// The all-users optimization would discard the restricted user-service result.
		queryUserFilter.ShouldQueryAllUsers = false
	}
	useExternalUsers := a.enableExtTableForUserFilter || restrictUserPopulation
	if ApplyUserFilterFromResult(&queryUserFilter, &attribute.Users, &attribute.Groups, useExternalUsers) {
		return &conversationAssistanceUsageInput{users: userFilter}, nil
	}
	includeBotConversations, err := shared.IncludeConversationSourceBot(ctx, a.configClient, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("resolve assistance usage population bot policy: %w", err)
	}
	population, err := buildConversationPopulationQuery(ctx, conversationPopulationInputs{
		timeRange:                        req.FilterByTimeRange,
		attribute:                        attribute,
		includeBotConversations:          includeBotConversations,
		allowMatchingStaleMetadataValues: req.AllowMatchingStaleMetadataValues,
		useExternalUserTable:             useExternalUsers,
	})
	if err != nil {
		return nil, fmt.Errorf("build assistance usage population query: %w", err)
	}
	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("connect to assistance usage population for %s: %w", req.Parent, err)
	}
	ctx = attachExtTablesToContext(ctx, population.externalTables...)
	query := "SELECT conversation_id, agent_user_id, conversation_time FROM (" + population.query + ")"
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, population.args...)
	if err != nil {
		return nil, fmt.Errorf("query assistance usage population: %w", err)
	}
	// fail-open: Closing rows cannot change the read result.
	defer rows.Close()

	var result []conversationAssistanceUsagePopulationRow
	for rows.Next() {
		var row conversationAssistanceUsagePopulationRow
		if err := rows.Scan(&row.conversationID, &row.agentUserID, &row.conversationTime); err != nil {
			return nil, fmt.Errorf("scan assistance usage population: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read assistance usage population: %w", err)
	}
	return &conversationAssistanceUsageInput{
		population:              result,
		users:                   userFilter,
		includeBotConversations: includeBotConversations || attribute.GetIncludeCrestaBot(),
	}, nil
}

func validateConversationAssistanceUsageRequest(req *analyticspb.RetrieveConversationAssistanceUsageStatsRequest) error {
	if _, err := shared.ParseCustomerProfile(req.Parent); err != nil {
		return err
	}
	if req.FilterByTimeRange.GetStartTimestamp() == nil || req.FilterByTimeRange.GetEndTimestamp() == nil {
		return status.Error(codes.InvalidArgument, "filter_by_time_range requires start and end timestamps")
	}
	if err := req.FilterByTimeRange.StartTimestamp.CheckValid(); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid start timestamp: %v", err)
	}
	if err := req.FilterByTimeRange.EndTimestamp.CheckValid(); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid end timestamp: %v", err)
	}
	if _, _, err := shared.ParseFilterByTimeRange(req.FilterByTimeRange); err != nil {
		return err
	}
	if req.Frequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED && req.Frequency != analyticspb.Frequency_DAILY && req.Frequency != analyticspb.Frequency_WEEKLY {
		return status.Error(codes.InvalidArgument, "frequency must be unspecified, DAILY or WEEKLY")
	}
	if len(req.GroupByAttributeTypes) > 1 {
		return status.Error(codes.InvalidArgument, "at most one grouping is supported")
	}
	if len(req.GroupByAttributeTypes) == 1 && req.GroupByAttributeTypes[0] != analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT && req.GroupByAttributeTypes[0] != analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP {
		return status.Error(codes.InvalidArgument, "grouping must be Agent or Team")
	}
	if req.Metadata.GetStartDayOfWeek() < analyticspb.Metadata_DAY_OF_WEEK_UNSPECIFIED || req.Metadata.GetStartDayOfWeek() > analyticspb.Metadata_SUNDAY {
		return status.Error(codes.InvalidArgument, "invalid start day of week")
	}
	timezoneID := req.Metadata.GetTimeZoneId()
	if timezoneID == "" {
		timezoneID = shared.DefaultTimeZone
	}
	if _, err := shared.TimezoneOffsetSecs(timezoneID); err != nil {
		return err
	}
	if req.IncludeConversationUsageDetails {
		return status.Error(codes.Unimplemented, "conversation usage details are not supported in this phase")
	}
	return nil
}

type assistanceUsageCounts struct {
	conversations         set.Set[string]
	assistedConversations set.Set[string]
}

func newAssistanceUsageCounts() *assistanceUsageCounts {
	return &assistanceUsageCounts{
		conversations:         set.New[string](),
		assistedConversations: set.New[string](),
	}
}

type assistanceUsageAccumulator struct {
	period  *assistanceUsageCounts
	buckets map[time.Time]*assistanceUsageCounts
}

func newAssistanceUsageAccumulator() *assistanceUsageAccumulator {
	return &assistanceUsageAccumulator{
		period:  newAssistanceUsageCounts(),
		buckets: map[time.Time]*assistanceUsageCounts{},
	}
}

func (a *assistanceUsageAccumulator) add(conversationID string, used bool, bucket time.Time) {
	a.period.conversations.Add(conversationID)
	if used {
		a.period.assistedConversations.Add(conversationID)
	}
	if bucket.IsZero() {
		return
	}
	counts, ok := a.buckets[bucket]
	if !ok {
		counts = newAssistanceUsageCounts()
		a.buckets[bucket] = counts
	}
	counts.conversations.Add(conversationID)
	if used {
		counts.assistedConversations.Add(conversationID)
	}
}

func (a *assistanceUsageAccumulator) result(frequency analyticspb.Frequency) *analyticspb.ConversationAssistanceUsageStatsResult {
	result := &analyticspb.ConversationAssistanceUsageStatsResult{
		TotalConversationCount:               int32(a.period.conversations.Len()),
		ConversationsWithAssistanceUsedCount: int32(a.period.assistedConversations.Len()),
	}
	for start, counts := range a.buckets {
		days := 1
		if frequency == analyticspb.Frequency_WEEKLY {
			days = 7
		}
		result.ConversationAssistanceUsageStats = append(result.ConversationAssistanceUsageStats, &analyticspb.ConversationAssistanceUsageStats{
			TimeRange: &analyticspb.TimeRange{
				StartTimestamp: timestamppb.New(start),
				EndTimestamp:   timestamppb.New(start.AddDate(0 /* years */, 0 /* months */, days)),
			},
			TotalConversationCount:               int32(counts.conversations.Len()),
			ConversationsWithAssistanceUsedCount: int32(counts.assistedConversations.Len()),
		})
	}
	sortAssistanceUsageBuckets(result)
	return result
}

func sortAssistanceUsageBuckets(result *analyticspb.ConversationAssistanceUsageStatsResult) {
	slices.SortFunc(result.ConversationAssistanceUsageStats, func(a, b *analyticspb.ConversationAssistanceUsageStats) int {
		return a.TimeRange.StartTimestamp.AsTime().Compare(b.TimeRange.StartTimestamp.AsTime())
	})
}

func aggregateConversationAssistanceUsage(req *analyticspb.RetrieveConversationAssistanceUsageStatsRequest, population []conversationAssistanceUsagePopulationRow, evidence set.Set[assistanceUsagePair], users *ParseUserFilterResult) (*analyticspb.RetrieveConversationAssistanceUsageStatsResponse, error) {
	timezoneID := req.Metadata.GetTimeZoneId()
	if timezoneID == "" {
		timezoneID = shared.DefaultTimeZone
	}
	offset, err := shared.TimezoneOffsetSecs(timezoneID)
	if err != nil {
		return nil, err
	}
	// Match parseClickhouseGroupByForTimeColumn's current fixed, whole-hour offset.
	location := time.FixedZone(timezoneID, offset/3600*3600)
	overall := newAssistanceUsageAccumulator()
	agents := map[string]*assistanceUsageAccumulator{}
	for _, row := range population {
		agent, ok := agents[row.agentUserID]
		if !ok {
			agent = newAssistanceUsageAccumulator()
			agents[row.agentUserID] = agent
		}
		used := evidence.Has(assistanceUsagePair{row.conversationID, row.agentUserID})
		bucket := assistanceUsageBucketStart(row.conversationTime, req.Frequency, req.Metadata, location)
		overall.add(row.conversationID, used, bucket)
		agent.add(row.conversationID, used, bucket)
	}
	overallResult := overall.result(req.Frequency)
	response := &analyticspb.RetrieveConversationAssistanceUsageStatsResponse{
		TotalConversationCount:               overallResult.TotalConversationCount,
		ConversationsWithAssistanceUsedCount: overallResult.ConversationsWithAssistanceUsedCount,
	}
	if len(req.GroupByAttributeTypes) == 0 {
		response.ConversationAssistanceUsageStatsResults = []*analyticspb.ConversationAssistanceUsageStatsResult{overallResult}
		return response, nil
	}
	agentResults := map[string]*analyticspb.ConversationAssistanceUsageStatsResult{}
	for _, user := range users.FinalUsers {
		name, err := userpb.ParseUserName(user.Name)
		if err != nil {
			return nil, fmt.Errorf("parse assistance usage agent: %w", err)
		}
		agent, ok := agents[name.UserID]
		if !ok {
			agent = newAssistanceUsageAccumulator()
		}
		result := agent.result(req.Frequency)
		result.Attribute = &analyticspb.Attribute{Users: []*userpb.User{user}}
		agentResults[user.Name] = result
	}
	if req.GroupByAttributeTypes[0] == analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT {
		for _, user := range users.FinalUsers {
			response.ConversationAssistanceUsageStatsResults = append(response.ConversationAssistanceUsageStatsResults, agentResults[user.Name])
		}
		return response, nil
	}
	groupResults := map[string]*analyticspb.ConversationAssistanceUsageStatsResult{}
	for _, group := range users.GroupsToAggregate {
		groupResults[group.Name] = &analyticspb.ConversationAssistanceUsageStatsResult{Attribute: &analyticspb.Attribute{Groups: []*userpb.Group{group}}}
	}
	for userName, groupNames := range users.UserNameToGroupNamesMap {
		agent, ok := agentResults[userName]
		if !ok {
			continue
		}
		for groupName := range set.New(groupNames...).All() {
			group, ok := groupResults[groupName]
			if !ok {
				continue
			}
			group.TotalConversationCount += agent.TotalConversationCount
			group.ConversationsWithAssistanceUsedCount += agent.ConversationsWithAssistanceUsedCount
			addAssistanceUsageTeamBuckets(group, agent)
		}
	}
	for _, group := range users.GroupsToAggregate {
		result := groupResults[group.Name]
		sortAssistanceUsageBuckets(result)
		response.ConversationAssistanceUsageStatsResults = append(response.ConversationAssistanceUsageStatsResults, result)
	}
	return response, nil
}

func assistanceUsageBucketStart(conversationTime time.Time, frequency analyticspb.Frequency, metadata *analyticspb.Metadata, location *time.Location) time.Time {
	if frequency == analyticspb.Frequency_FREQUENCY_UNSPECIFIED {
		return time.Time{}
	}
	local := conversationTime.In(location)
	bucket := time.Date(local.Year(), local.Month(), local.Day(), 0 /* hour */, 0 /* min */, 0 /* sec */, 0 /* nsec */, location)
	if frequency == analyticspb.Frequency_WEEKLY {
		bias := shared.ParseStartDayOfWeek(metadata, true /* isWeekly */)
		daysSinceWeekStart := (int(local.Weekday()) + 6 + bias) % 7
		bucket = bucket.AddDate(0 /* years */, 0 /* months */, -daysSinceWeekStart)
	}
	return bucket.UTC()
}

func addAssistanceUsageTeamBuckets(group, agent *analyticspb.ConversationAssistanceUsageStatsResult) {
	byStart := map[int64]*analyticspb.ConversationAssistanceUsageStats{}
	for _, bucket := range group.ConversationAssistanceUsageStats {
		byStart[bucket.TimeRange.StartTimestamp.Seconds] = bucket
	}
	for _, source := range agent.ConversationAssistanceUsageStats {
		key := source.TimeRange.StartTimestamp.Seconds
		bucket, ok := byStart[key]
		if !ok {
			bucket = &analyticspb.ConversationAssistanceUsageStats{TimeRange: source.TimeRange}
			group.ConversationAssistanceUsageStats = append(group.ConversationAssistanceUsageStats, bucket)
			byStart[key] = bucket
		}
		bucket.TotalConversationCount += source.TotalConversationCount
		bucket.ConversationsWithAssistanceUsedCount += source.ConversationsWithAssistanceUsedCount
	}
}
