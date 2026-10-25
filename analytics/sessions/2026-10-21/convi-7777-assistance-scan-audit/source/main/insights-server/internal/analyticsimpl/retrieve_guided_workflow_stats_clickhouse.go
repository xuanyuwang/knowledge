package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type guidedWorkflowStatsRow struct {
	clickhouseKeyTimeRow
	totalGuidedWorkflowUsedCount uint64
	convoCount                   uint64
	userCount                    uint64
}

func guidedWorkflowStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (string, []any) {
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}

	rowsQuery, args := buildGuidedWorkflowUsageRowsQuery(conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	return fmt.Sprintf(`
			WITH usage_rows AS (%s)
			SELECT
			%s
			COUNT(DISTINCT IF(
			event_type = 4 AND -- 4: USED_KB_DOCUMENT
			doc_type = '2' AND -- 2: KB_DOCUMENT_TYPE_GUIDED_WORKFLOW
			guided_workflow_name <> '',
			session_metadata_id,
			NULL)) AS total_gw_used_count,
			COUNT(DISTINCT conversation_id) AS convo_count,
			COUNT(DISTINCT agent_user_id) AS user_count
			FROM usage_rows
			%s
			`, rowsQuery, concatQueries(groupByQueries[eventConversationEventTable]), optionalGroupBy), args
}

func (a AnalyticsServiceImpl) readGuidedWorkflowStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveGuidedWorkflowStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveGuidedWorkflowStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		conversationTable,
		eventConversationEventTable,
	}
	conditionAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
	if err != nil {
		return nil, err
	}

	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, targetTables)
	if err != nil {
		return nil, err
	}
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, guidedWorkflowNameColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, fmt.Errorf("group_by keys %v are not supported", v)
	}
	chQuery, chArgs := guidedWorkflowStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0
	conn, err := a.getEventsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	if hasMetadataFiltering {
		conn, err = a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
		if err != nil {
			return nil, err
		}
	}

	ctx = attachExtTablesToContext(ctx, usersExtTable)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	chRows := []guidedWorkflowStatsRow{}
	for rows.Next() {
		row := guidedWorkflowStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			case guidedWorkflowNameColumn:
				vals = append(vals, &row.guidedWorkflowName)
			}
		}
		vals = append(vals, &row.totalGuidedWorkflowUsedCount, &row.convoCount, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[guidedWorkflowStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[guidedWorkflowStats] ClickHouse (events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToGuidedWorkflowStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToGuidedWorkflowStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []guidedWorkflowStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveGuidedWorkflowStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveGuidedWorkflowStatsResponse{}, nil
	}
	groupByAttributeToGuidedWorkflowStatsResult := make(map[groupByAttribute]*analyticspb.GuidedWorkflowStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		guidedWorkflowStats := analyticspb.GuidedWorkflowStat{
			TimeRange:                     &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalGuidedWorkflowClickCount: int32(row.totalGuidedWorkflowUsedCount),
			TotalConversationCount:        int32(row.convoCount),
			AverageGuidedWorkflowClickCountPerConversation:     safeDivideFloat(int32(row.totalGuidedWorkflowUsedCount), int32(row.convoCount)),
			AverageGuidedWorkflowClickCountPerFrequencyPerUser: safeDivideFloat(int32(row.totalGuidedWorkflowUsedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToGuidedWorkflowStatsResult[groupBy]; !exists {
			groupByAttributeToGuidedWorkflowStatsResult[groupBy] = &analyticspb.GuidedWorkflowStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToGuidedWorkflowStatsResult[groupBy].GuidedWorkflowStats = append(groupByAttributeToGuidedWorkflowStatsResult[groupBy].GuidedWorkflowStats, &guidedWorkflowStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToGuidedWorkflowStatsResult {
		attr := k.GetAttribute(customerID, profileID)
		if len(attr.Users) > 0 {
			if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
				attr.Users[0].FullName = user.FullName
				attr.Users[0].Username = user.Username
			}
		}

		groupByAttributeToAttribute[k] = attr
		attributes = append(attributes, attr)
	}

	ignoredAttributes, err := attributesToIgnore(a.appsDB.DB(ctx), attributes)
	if err != nil {
		return nil, err
	}

	totalUserFrequencyCount := int32(0)
	resp := &analyticspb.RetrieveGuidedWorkflowStatsResponse{}
	for k, v := range groupByAttributeToGuidedWorkflowStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, guidedWorkflowStats := range v.GuidedWorkflowStats {
			v.TotalGuidedWorkflowClickCount += guidedWorkflowStats.TotalGuidedWorkflowClickCount
			v.TotalConversationCount += guidedWorkflowStats.TotalConversationCount
		}

		v.AverageGuidedWorkflowClickCountPerConversation = safeDivideFloat(v.TotalGuidedWorkflowClickCount, v.TotalConversationCount)
		v.AverageGuidedWorkflowClickCountPerFrequencyPerUser = safeDivideFloat(v.TotalGuidedWorkflowClickCount, groupByAttributeToUserCount[k])
		// Sort result based on timestamp.
		sort.Slice(v.GuidedWorkflowStats, func(i, j int) bool {
			return v.GuidedWorkflowStats[i].TimeRange.StartTimestamp.AsTime().Before(v.GuidedWorkflowStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		resp.TotalGuidedWorkflowClickCount += v.TotalGuidedWorkflowClickCount
		resp.TotalConversationCount += v.TotalConversationCount
		resp.GuidedWorkflowStatsResults = append(resp.GuidedWorkflowStatsResults, v)
	}
	resp.GuidedWorkflowStatsResults = sortResults(resp.GuidedWorkflowStatsResults)
	resp.AverageGuidedWorkflowClickCountPerConversation = safeDivideFloat(resp.TotalGuidedWorkflowClickCount, resp.TotalConversationCount)
	resp.AverageGuidedWorkflowClickCountPerFrequencyPerUser = safeDivideFloat(resp.TotalGuidedWorkflowClickCount, totalUserFrequencyCount)

	return resp, nil
}
