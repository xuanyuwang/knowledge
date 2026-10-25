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

type smartComposeStatsRow struct {
	clickhouseKeyTimeRow
	totalSmartComposeUsedCount uint64
	convoCount                 uint64
	userCount                  uint64
}

func smartComposeStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
) (string, []any) {
	convCondition, convArgs := concatConditionsAndArgs(conditionsAndArgs[conversationTable])
	aeCondition, aeArgs := concatConditionsAndArgs(conditionsAndArgs[eventAnalyticsEventTable])
	convGroupByQueries := concatQueries(groupByQueries[conversationTable])
	aeGroupByQueries := concatQueries(groupByQueries[eventAnalyticsEventTable])
	finalJoinKeys, finalGroupByQueries := concatKeys(groupByKeys), concatQueries(groupByKeys)

	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}

	// As of 08/28/2024, we still read smart compose stats from analytics events (equivalent
	// to segment events). And in future when we have enough newly defined conversation events
	// populated into ClickHouse, we will likely migrate this to conversation events.
	// Note: during the query below, we are dedupping the smart compose used count based on
	// the event timestamp and smart compose text.
	//
	// Additionally smart compose events don't have direct mapping with conversations and thus
	// we cannot directly filter usecase on smart compose uage. However, we could indirectly
	// filter on which agent(s) having conversations on filtered usecases. The assumption is that
	// different agents might not take care of conversations across usecases.
	// As of 09/27/2024, this is mainly for united airlines. We will figure out smart compose by
	// conversation events in future.
	return fmt.Sprintf(`
		WITH 
		conversation_stats AS (
			SELECT
				%s
				COUNT(DISTINCT conversation_id) AS convo_count,
				COUNT(DISTINCT agent_user_id) AS user_count
			FROM
				conversation_d
			WHERE
				agent_user_id <> ''
				AND %s -- Additional filters
			-- GROUP BY is optional.
			%s
		),
		agent_hourly_conversation AS (
			SELECT DISTINCT
				agent_user_id,
				DATE_TRUNC('hour', conversation_start_time) AS event_time
			FROM
				conversation_d
			WHERE
				agent_user_id <> ''
				AND %s -- Additional filters
		), 
		agent_hourly_smart_compose_stats AS (
			SELECT
				agent_user_id,
				DATE_TRUNC('hour', event_time) AS event_time,
				COUNT(DISTINCT
					DATE_TRUNC('hour', event_time),
					JSONExtractString(JSONExtractString(toJSONString(event_payload), 'smartComposeInserted'), 'smartComposeText')
				) AS smart_compose_used_count
			FROM 
				analytics_event_d
			WHERE TRUE
				AND event_name = 'ChatAgentAssist.SmartCompose.Insert.SmartComposeText'
				AND agent_user_id <> ''
				AND %s -- Additional filters
			GROUP BY agent_user_id, event_time
		),
		smart_compose_stats AS (
			SELECT
				%s
				SUM(smart_compose_used_count) AS total_smart_compose_used_count
			FROM
				agent_hourly_smart_compose_stats
					JOIN agent_hourly_conversation USING (agent_user_id, event_time)
			-- GROUP BY is optional.
			%s
		)
		SELECT 
			%s
			SUM(total_smart_compose_used_count),
			MAX(convo_count), MAX(user_count)
		FROM
			conversation_stats LEFT JOIN smart_compose_stats USING (%s)
		-- GROUP BY is optional.
		%s
		`,
			// conversation_stats
			convGroupByQueries, convCondition, optionalGroupBy,
			// agent_hourly_conversation
			convCondition,
			// agent_hourly_smart_compose_stats
			aeCondition,
			// smart_compose_stats
			aeGroupByQueries, optionalGroupBy,
			// main query
			finalGroupByQueries, finalJoinKeys, optionalGroupBy),
		append(convArgs, append(convArgs, aeArgs...)...)
}

func (a AnalyticsServiceImpl) readSmartComposeStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveSmartComposeStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveSmartComposeStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		conversationTable,
		eventAnalyticsEventTable,
	}
	conditionAndArgs, _, _, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
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
	chQuery, chArgs := smartComposeStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	conn, err := a.getEventsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
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

	chRows := []smartComposeStatsRow{}
	for rows.Next() {
		row := smartComposeStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.totalSmartComposeUsedCount, &row.convoCount, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[smartComposeStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[smartComposeStats] ClickHouse (events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToSmartComposeStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToSmartComposeStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []smartComposeStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveSmartComposeStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveSmartComposeStatsResponse{}, nil
	}
	groupByAttributeToSmartComposeStatsResult := make(map[groupByAttribute]*analyticspb.SmartComposeStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		smartComposeStats := analyticspb.SmartComposeStats{
			TimeRange:                                       &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalSmartComposeUsedCount:                      int32(row.totalSmartComposeUsedCount),
			TotalConversationCount:                          int32(row.convoCount),
			AverageSmartComposeUsedCountPerConversation:     safeDivideFloat(int32(row.totalSmartComposeUsedCount), int32(row.convoCount)),
			AverageSmartComposeUsedCountPerFrequencyPerUser: safeDivideFloat(int32(row.totalSmartComposeUsedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToSmartComposeStatsResult[groupBy]; !exists {
			groupByAttributeToSmartComposeStatsResult[groupBy] = &analyticspb.SmartComposeStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToSmartComposeStatsResult[groupBy].SmartComposeStats = append(groupByAttributeToSmartComposeStatsResult[groupBy].SmartComposeStats, &smartComposeStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToSmartComposeStatsResult {
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
	resp := &analyticspb.RetrieveSmartComposeStatsResponse{}
	for k, v := range groupByAttributeToSmartComposeStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, smartComposeStats := range v.SmartComposeStats {
			v.TotalSmartComposeUsedCount += smartComposeStats.TotalSmartComposeUsedCount
			v.TotalConversationCount += smartComposeStats.TotalConversationCount
		}

		v.AverageSmartComposeUsedCountPerConversation = safeDivideFloat(v.TotalSmartComposeUsedCount, v.TotalConversationCount)
		v.AverageSmartComposeUsedCountPerFrequencyPerUser = safeDivideFloat(v.TotalSmartComposeUsedCount, groupByAttributeToUserCount[k])
		// Sort result based on timestamp.
		sort.Slice(v.SmartComposeStats, func(i, j int) bool {
			return v.SmartComposeStats[i].TimeRange.StartTimestamp.AsTime().Before(v.SmartComposeStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		resp.TotalSmartComposeUsedCount += v.TotalSmartComposeUsedCount
		resp.TotalConversationCount += v.TotalConversationCount
		resp.SmartComposeStatsResults = append(resp.SmartComposeStatsResults, v)
	}
	resp.SmartComposeStatsResults = sortResults(resp.SmartComposeStatsResults)
	resp.AverageSmartComposeUsedCountPerConversation = safeDivideFloat(resp.TotalSmartComposeUsedCount, resp.TotalConversationCount)
	resp.AverageSmartComposeUsedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalSmartComposeUsedCount, totalUserFrequencyCount)

	return resp, nil
}
