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

type knowledgeBaseStatsRow struct {
	clickhouseKeyTimeRow
	totalKbSearchCount uint64
	convoCount         uint64
	userCount          uint64
}

func knowledgeBaseStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (string, []any) {
	eventQuery, args := buildKnowledgeBaseEventRowsQuery(conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	return fmt.Sprintf(`WITH knowledge_base_event_rows AS (%s)
		SELECT %s
			COUNT(DISTINCT IF(event_type = %d, event_id, NULL)) AS total_kb_search_count,
			COUNT(DISTINCT conversation_id) AS convo_count,
			COUNT(DISTINCT agent_user_id) AS user_count
		FROM knowledge_base_event_rows
		%s`, eventQuery, concatQueries(groupByQueries[conversationEventTable]), usedKBSearchEventType, optionalGroupBy), args
}

func (a AnalyticsServiceImpl) readKnowledgeBaseStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveKnowledgeBaseStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveKnowledgeBaseStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		conversationEventTable,
	}
	conditionAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
	if err != nil {
		return nil, err
	}

	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, targetTables)
	if err != nil {
		return nil, err
	}
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, fmt.Errorf("group_by keys %v are not supported", v)
	}
	chQuery, chArgs := knowledgeBaseStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	// As of 08/27/2024, we are choosing to query the clickhouse conversations cluster to
	// avoid backfilling, because this query doesn't need to have conditions on payload.
	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	ctx = attachExtTablesToContext(ctx, usersExtTable)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	chRows := []knowledgeBaseStatsRow{}
	for rows.Next() {
		row := knowledgeBaseStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.totalKbSearchCount, &row.convoCount, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[knowledgeBaseStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[knowledgeBaseStats] ClickHouse(events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToKnowledgeBaseStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToKnowledgeBaseStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []knowledgeBaseStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveKnowledgeBaseStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveKnowledgeBaseStatsResponse{}, nil
	}
	groupByAttributeToKnowledgeBaseStatsResult := make(map[groupByAttribute]*analyticspb.KnowledgeBaseStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		knowledgeBaseStats := analyticspb.KBSearchStats{
			TimeRange:                               &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalKbSearchCount:                      int32(row.totalKbSearchCount),
			TotalConversationCount:                  int32(row.convoCount),
			AverageKbSearchCountPerConversation:     safeDivideFloat(int32(row.totalKbSearchCount), int32(row.convoCount)),
			AverageKbSearchCountPerFrequencyPerUser: safeDivideFloat(int32(row.totalKbSearchCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToKnowledgeBaseStatsResult[groupBy]; !exists {
			groupByAttributeToKnowledgeBaseStatsResult[groupBy] = &analyticspb.KnowledgeBaseStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToKnowledgeBaseStatsResult[groupBy].KbSearchStats = append(groupByAttributeToKnowledgeBaseStatsResult[groupBy].KbSearchStats, &knowledgeBaseStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToKnowledgeBaseStatsResult {
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
	resp := &analyticspb.RetrieveKnowledgeBaseStatsResponse{}
	for k, v := range groupByAttributeToKnowledgeBaseStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, kbSearchStat := range v.KbSearchStats {
			v.TotalKbSearchCount += kbSearchStat.TotalKbSearchCount
			v.TotalConversationCount += kbSearchStat.TotalConversationCount
		}

		v.AverageKbSearchCountPerConversation = safeDivideFloat(v.TotalKbSearchCount, v.TotalConversationCount)
		v.AverageKbSearchCountPerFrequencyPerUser = safeDivideFloat(v.TotalKbSearchCount, groupByAttributeToUserCount[k])
		// Sort result based on timestamp.
		sort.Slice(v.KbSearchStats, func(i, j int) bool {
			return v.KbSearchStats[i].TimeRange.StartTimestamp.AsTime().Before(v.KbSearchStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		resp.TotalKbSearchCount += v.TotalKbSearchCount
		resp.TotalConversationCount += v.TotalConversationCount
		resp.KnowledgeBaseStatsResults = append(resp.KnowledgeBaseStatsResults, v)
	}
	resp.KnowledgeBaseStatsResults = sortResults(resp.KnowledgeBaseStatsResults)
	resp.AverageKbSearchCountPerConversation = safeDivideFloat(resp.TotalKbSearchCount, resp.TotalConversationCount)
	resp.AverageKbSearchCountPerFrequencyPerUser = safeDivideFloat(resp.TotalKbSearchCount, totalUserFrequencyCount)

	return resp, nil
}
