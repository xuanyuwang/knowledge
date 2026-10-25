package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type noteTakingStatsRow struct {
	clickhouseKeyTimeRow
	totalNoteTakingUsedCount uint64
	convoCount               uint64
	userCount                uint64
}

func noteTakingStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
	hasMetadataFiltering bool,
) (string, []any) {
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}

	copyEntityCondition := "event_name = 'VoiceAgentAssist.NoteTaking.Click.CopyEntity'"
	if hasMetadataFiltering {
		copyEntityCondition = "note_taking_data_type = 1"
	}
	groupQueries := ""
	if len(groupByQueries[eventConversationEventTable]) > 0 {
		groupQueries = strings.Join(groupByQueries[eventConversationEventTable], ", ") + ", "
	}
	rowsQuery, args := buildNoteTakingUsageRowsQuery(conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, hasMetadataFiltering)
	return fmt.Sprintf(`
			WITH usage_rows AS (%s)
			SELECT
			%s
			COUNT(DISTINCT IF(
			event_type = 5 AND -- 5: USED_NOTE_TAKING
			%s,
			event_id,
			NULL)) AS total_note_taking_used_count,
			COUNT(DISTINCT conversation_id) AS convo_count,
			COUNT(DISTINCT agent_user_id) AS user_count
			FROM usage_rows
			%s
			`, rowsQuery, groupQueries, copyEntityCondition, optionalGroupBy), args
}

func (a AnalyticsServiceImpl) readNoteTakingStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveNoteTakingStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveNoteTakingStatsResponse, error) {
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
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, fmt.Errorf("group_by keys %v are not supported", v)
	}

	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0
	chQuery, chArgs := noteTakingStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, hasMetadataFiltering)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

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

	chRows := []noteTakingStatsRow{}
	for rows.Next() {
		row := noteTakingStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.totalNoteTakingUsedCount, &row.convoCount, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[noteTakingStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[noteTakingStats] ClickHouse (conversations) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToNoteTakingStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToNoteTakingStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []noteTakingStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveNoteTakingStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveNoteTakingStatsResponse{}, nil
	}
	groupByAttributeToNoteTakingStatsResult := make(map[groupByAttribute]*analyticspb.NoteTakingStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		noteTakingStats := analyticspb.NoteTakingStats{
			TimeRange:                &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalNoteTakingUsedCount: int32(row.totalNoteTakingUsedCount),
			TotalConversationCount:   int32(row.convoCount),
			AverageNoteTakingUsedCountPerConversation: safeDivideFloat(int32(row.totalNoteTakingUsedCount), int32(row.convoCount)),
			AverageNoteTakingUsedCountPerUser:         safeDivideFloat(int32(row.totalNoteTakingUsedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToNoteTakingStatsResult[groupBy]; !exists {
			groupByAttributeToNoteTakingStatsResult[groupBy] = &analyticspb.NoteTakingStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToNoteTakingStatsResult[groupBy].NoteTakingStats = append(groupByAttributeToNoteTakingStatsResult[groupBy].NoteTakingStats, &noteTakingStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToNoteTakingStatsResult {
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
	resp := &analyticspb.RetrieveNoteTakingStatsResponse{}
	for k, v := range groupByAttributeToNoteTakingStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, noteTakingStats := range v.NoteTakingStats {
			v.TotalNoteTakingUsedCount += noteTakingStats.TotalNoteTakingUsedCount
			v.TotalConversationCount += noteTakingStats.TotalConversationCount
		}

		v.AverageNoteTakingUsedCountPerConversation = safeDivideFloat(v.TotalNoteTakingUsedCount, v.TotalConversationCount)
		v.AverageNoteTakingUsedCountPerFrequencyPerUser = safeDivideFloat(v.TotalNoteTakingUsedCount, groupByAttributeToUserCount[k])
		// Sort result based on timestamp.
		sort.Slice(v.NoteTakingStats, func(i, j int) bool {
			return v.NoteTakingStats[i].TimeRange.StartTimestamp.AsTime().Before(v.NoteTakingStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		resp.TotalNoteTakingUsedCount += v.TotalNoteTakingUsedCount
		resp.TotalConversationCount += v.TotalConversationCount
		resp.NoteTakingStatsResults = append(resp.NoteTakingStatsResults, v)
	}
	resp.NoteTakingStatsResults = sortResults(resp.NoteTakingStatsResults)
	resp.AverageNoteTakingUsedCountPerConversation = safeDivideFloat(resp.TotalNoteTakingUsedCount, resp.TotalConversationCount)
	resp.AverageNoteTakingUsedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalNoteTakingUsedCount, totalUserFrequencyCount)

	return resp, nil
}
