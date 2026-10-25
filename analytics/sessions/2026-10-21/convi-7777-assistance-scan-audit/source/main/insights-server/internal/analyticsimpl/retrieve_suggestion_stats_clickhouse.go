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

type suggestionStatsRow struct {
	clickhouseKeyTimeRow
	totalSuggestionUsedCount               uint64
	totalEmailPiecemealSuggestionUsedCount uint64
	totalEmailFullReplySuggestionUsedCount uint64
	convoCount                             uint64
	userCount                              uint64
}

func suggestionStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (string, []any) {
	eventRowsQuery, args := buildSuggestionEventRowsQuery(conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	// Event types are stored using the SQL-schema enum, not the conversation proto enum.
	return fmt.Sprintf(`WITH suggestion_event_rows AS (%s)
 SELECT %s
 COUNT(DISTINCT IF(event_type IN (4, 5, 10), event_id, NULL)) AS total_suggestions_count, -- 4: USED_SUGGESTION; 5: USED_HINT_SUGGESTION; 10: USED_SEARCH_SUGGESTION
				COUNT(DISTINCT IF(event_type IN (25), event_id, NULL)) AS total_email_piecemeal_suggestions_count, -- 25: EMAIL_USED_PIECEMEAL_SUGGESTION in sql-schema apiserver/sql-schema/protos/event/event.proto
				COUNT(DISTINCT IF(event_type IN (27), event_id, NULL)) AS total_email_full_reply_suggestions_count, -- 27: EMAIL_USED_FULL_REPLY_SUGGESTION in sql-schema apiserver/sql-schema/protos/event/event.proto
				COUNT(DISTINCT conversation_id) AS convo_count,
				COUNT(DISTINCT agent_user_id) AS user_count
 FROM suggestion_event_rows
 %s`, eventRowsQuery, concatQueries(groupByQueries[conversationEventTable]), optionalGroupBy), args
}

func (a AnalyticsServiceImpl) readSuggestionStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveSuggestionStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveSuggestionStatsResponse, error) {
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
	chQuery, chArgs := suggestionStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

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

	chRows := []suggestionStatsRow{}
	for rows.Next() {
		row := suggestionStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.totalSuggestionUsedCount, &row.totalEmailPiecemealSuggestionUsedCount, &row.totalEmailFullReplySuggestionUsedCount, &row.convoCount, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[suggestionStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[suggestionStats] ClickHouse (events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToSuggestionStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToSuggestionStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []suggestionStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveSuggestionStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveSuggestionStatsResponse{}, nil
	}
	groupByAttributeToSuggestionStatsResult := make(map[groupByAttribute]*analyticspb.SuggestionStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		suggestionStats := analyticspb.SuggestionStats{
			TimeRange:                &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalConversationCount:   int32(row.convoCount),
			TotalSuggestionUsedCount: int32(row.totalSuggestionUsedCount),
			AverageSuggestionUsedCountPerConversation:                 safeDivideFloat(int32(row.totalSuggestionUsedCount), int32(row.convoCount)),
			AverageSuggestionUsedCountPerFrequencyPerUser:             safeDivideFloat(int32(row.totalSuggestionUsedCount), int32(row.userCount)),
			TotalEmailPartialSuggestionUsedCount:                      int32(row.totalEmailPiecemealSuggestionUsedCount),
			AverageEmailPartialSuggestionUsedCountPerConversation:     safeDivideFloat(int32(row.totalEmailPiecemealSuggestionUsedCount), int32(row.convoCount)),
			AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: safeDivideFloat(int32(row.totalEmailPiecemealSuggestionUsedCount), int32(row.userCount)),
			TotalEmailFullSuggestionUsedCount:                         int32(row.totalEmailFullReplySuggestionUsedCount),
			AverageEmailFullSuggestionUsedCountPerConversation:        safeDivideFloat(int32(row.totalEmailFullReplySuggestionUsedCount), int32(row.convoCount)),
			AverageEmailFullSuggestionUsedCountPerFrequencyPerUser:    safeDivideFloat(int32(row.totalEmailFullReplySuggestionUsedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToSuggestionStatsResult[groupBy]; !exists {
			groupByAttributeToSuggestionStatsResult[groupBy] = &analyticspb.SuggestionStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToSuggestionStatsResult[groupBy].SuggestionStats = append(groupByAttributeToSuggestionStatsResult[groupBy].SuggestionStats, &suggestionStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToSuggestionStatsResult {
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
	resp := &analyticspb.RetrieveSuggestionStatsResponse{}
	for k, v := range groupByAttributeToSuggestionStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, suggestionStat := range v.SuggestionStats {
			v.TotalSuggestionUsedCount += suggestionStat.TotalSuggestionUsedCount
			v.TotalEmailPartialSuggestionUsedCount += suggestionStat.TotalEmailPartialSuggestionUsedCount
			v.TotalEmailFullSuggestionUsedCount += suggestionStat.TotalEmailFullSuggestionUsedCount
			v.TotalConversationCount += suggestionStat.TotalConversationCount
		}

		v.AverageSuggestionUsedCountPerConversation = safeDivideFloat(v.TotalSuggestionUsedCount, v.TotalConversationCount)
		v.AverageSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(v.TotalSuggestionUsedCount, groupByAttributeToUserCount[k])
		v.AverageEmailPartialSuggestionUsedCountPerConversation = safeDivideFloat(v.TotalEmailPartialSuggestionUsedCount, v.TotalConversationCount)
		v.AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(v.TotalEmailPartialSuggestionUsedCount, groupByAttributeToUserCount[k])
		v.AverageEmailFullSuggestionUsedCountPerConversation = safeDivideFloat(v.TotalEmailFullSuggestionUsedCount, v.TotalConversationCount)
		v.AverageEmailFullSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(v.TotalEmailFullSuggestionUsedCount, groupByAttributeToUserCount[k])
		// Sort result based on timestamp.
		sort.Slice(v.SuggestionStats, func(i, j int) bool {
			return v.SuggestionStats[i].TimeRange.StartTimestamp.AsTime().Before(v.SuggestionStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		resp.TotalConversationCount += v.TotalConversationCount
		resp.TotalSuggestionUsedCount += v.TotalSuggestionUsedCount
		resp.TotalEmailPartialSuggestionUsedCount += v.TotalEmailPartialSuggestionUsedCount
		resp.TotalEmailFullSuggestionUsedCount += v.TotalEmailFullSuggestionUsedCount
		resp.SuggestionStatsResults = append(resp.SuggestionStatsResults, v)
	}
	resp.SuggestionStatsResults = sortResults(resp.SuggestionStatsResults)
	resp.AverageSuggestionUsedCountPerConversation = safeDivideFloat(resp.TotalSuggestionUsedCount, resp.TotalConversationCount)
	resp.AverageSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalSuggestionUsedCount, totalUserFrequencyCount)
	resp.AverageEmailPartialSuggestionUsedCountPerConversation = safeDivideFloat(resp.TotalEmailPartialSuggestionUsedCount, resp.TotalConversationCount)
	resp.AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalEmailPartialSuggestionUsedCount, totalUserFrequencyCount)
	resp.AverageEmailFullSuggestionUsedCountPerConversation = safeDivideFloat(resp.TotalEmailFullSuggestionUsedCount, resp.TotalConversationCount)
	resp.AverageEmailFullSuggestionUsedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalEmailFullSuggestionUsedCount, totalUserFrequencyCount)

	return resp, nil
}
