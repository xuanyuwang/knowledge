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

type summarizationStatsRow struct {
	clickhouseKeyTimeRow
	totalSummarizationGeneratedCount uint64
	totalSummarizationUsedCount      uint64
	convoCount                       uint64
	userCount                        uint64
}

func summarizationStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (string, []any) {
	allArgs := []any{}

	// Check if we need metadata filtering
	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	// Generate metadata filters using refactored functions
	aliasDict := map[string]string{
		"conversation":      "conversation_d",
		"actionAnnotation":  "action_annotation_d",
		"conversationEvent": "conversation_event_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false, true)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	convCondition, convArgs := concatConditionsAndArgs(conditionsAndArgs[conversationTable])
	usageCTEsQuery, usageRowsQuery, usageArgs := buildSummarizationUsageRowsQuery(conditionsAndArgs, metadataFilters, hasMetadataFiltering)
	convGroupByQueries := concatQueries(groupByQueries[conversationTable])
	ceGroupByQueries := concatQueries(groupByQueries[conversationEventTable])
	finalJoinKeys, finalGroupByQueries := concatKeys(groupByKeys), concatQueries(groupByKeys)

	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}

	allArgs = append(allArgs, convArgs...)
	allArgs = append(allArgs, usageArgs...)

	if hasMetadataFiltering {
		return fmt.Sprintf(`
			WITH
			%s
			%s
			conversation_stats AS (
				SELECT %s
					COUNT(DISTINCT conversation_id) AS convo_count,
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM 
					conversation_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					agent_user_id <> ''
					AND %s -- Additional filters
					%s
				-- GROUP BY is optional.
				%s
			),
			%s
			summarization_stats AS (
				SELECT %s
					COUNT(DISTINCT convo_with_valid_summarization.conversation_id) AS generated_summarization_count,
					COUNT(DISTINCT CASE WHEN convo_with_used_summarization.conversation_id <> '' THEN convo_with_used_summarization.conversation_id END) AS used_summarization_count
				FROM
					%s
				-- GROUP BY is optional.
				%s
			)
			SELECT 
				%s
				SUM(generated_summarization_count) AS generated_summarization_count, 
				SUM(used_summarization_count) AS used_summarization_count,
				MAX(convo_count) AS convo_count, 
				MAX(user_count) AS user_count
			FROM 
				conversation_stats LEFT JOIN summarization_stats USING (%s)
			-- GROUP BY is optional.
			%s
			`,
				// Metadata filters
				metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
				// conversation_stats
				convGroupByQueries, metadataFilters.MomentJoinQueries["conversation"], metadataFilters.ExcludedMomentJoinQueries["conversation"], convCondition, metadataFilters.ExcludedMomentWhereQueries, optionalGroupBy,
				usageCTEsQuery,
				// summarization_stats
				ceGroupByQueries, usageRowsQuery, optionalGroupBy,
				// main query
				finalGroupByQueries, finalJoinKeys, optionalGroupBy),
			allArgs
	} else {
		return fmt.Sprintf(`
			WITH
			conversation_stats AS (
				SELECT %s
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
			%s
			summarization_stats AS (
				SELECT %s
					COUNT(DISTINCT convo_with_valid_summarization.conversation_id) AS generated_summarization_count,
					COUNT(DISTINCT CASE WHEN convo_with_used_summarization.conversation_id <> '' THEN convo_with_used_summarization.conversation_id END) AS used_summarization_count
				FROM
					%s
				-- GROUP BY is optional.
				%s
			)
			SELECT 
				%s
				SUM(generated_summarization_count) AS generated_summarization_count, 
				SUM(used_summarization_count) AS used_summarization_count,
				MAX(convo_count) AS convo_count, 
				MAX(user_count) AS user_count
			FROM 
				conversation_stats LEFT JOIN summarization_stats USING (%s)
			-- GROUP BY is optional.
			%s
			`,
				// conversation_stats
				convGroupByQueries, convCondition, optionalGroupBy,
				usageCTEsQuery,
				// summarization_stats
				ceGroupByQueries, usageRowsQuery, optionalGroupBy,
				// main query
				finalGroupByQueries, finalJoinKeys, optionalGroupBy),
			allArgs
	}
}

func (a AnalyticsServiceImpl) readSummarizationStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveSummarizationStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveSummarizationStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		conversationTable,
		actionAnnotationTable,
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
	chQuery, chArgs := summarizationStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
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

	chRows := []summarizationStatsRow{}
	for rows.Next() {
		row := summarizationStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals,
			&row.totalSummarizationGeneratedCount,
			&row.totalSummarizationUsedCount,
			&row.convoCount,
			&row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[summarizationStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[summarizationStats] ClickHouse (events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToSummarizationStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToSummarizationStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []summarizationStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveSummarizationStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveSummarizationStatsResponse{}, nil
	}
	groupByAttributeToSummarizationStatsResult := make(map[groupByAttribute]*analyticspb.SummarizationStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		summarizationStats := analyticspb.SummarizationStats{
			TimeRange:                                    &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalConversationCount:                       int32(row.convoCount),
			TotalSummarizationUsedCount:                  int32(row.totalSummarizationUsedCount),
			TotalSummarizationGeneratedCount:             int32(row.totalSummarizationGeneratedCount),
			AverageSummarizationUsedRatio:                min(safeDivideFloat(int32(row.totalSummarizationUsedCount), int32(row.totalSummarizationGeneratedCount)), 1) * 100,
			AverageSummarizationUsedCountPerConversation: safeDivideFloat(int32(row.totalSummarizationUsedCount), int32(row.convoCount)),
			AverageSummarizationUsedCountPerUser:         safeDivideFloat(int32(row.totalSummarizationUsedCount), int32(row.userCount)),
			AverageSummarizationGeneratedCountPerUser:    safeDivideFloat(int32(row.totalSummarizationGeneratedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToSummarizationStatsResult[groupBy]; !exists {
			groupByAttributeToSummarizationStatsResult[groupBy] = &analyticspb.SummarizationStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToSummarizationStatsResult[groupBy].SummarizationStats = append(groupByAttributeToSummarizationStatsResult[groupBy].SummarizationStats, &summarizationStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToSummarizationStatsResult {
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
	resp := &analyticspb.RetrieveSummarizationStatsResponse{}
	for k, v := range groupByAttributeToSummarizationStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, summarizationStat := range v.SummarizationStats {
			v.TotalConversationCount += summarizationStat.TotalConversationCount
			v.TotalSummarizationUsedCount += summarizationStat.TotalSummarizationUsedCount
			v.TotalSummarizationGeneratedCount += summarizationStat.TotalSummarizationGeneratedCount
		}

		v.AverageSummarizationUsedRatio = min(safeDivideFloat(v.TotalSummarizationUsedCount, v.TotalSummarizationGeneratedCount), 1) * 100
		v.AverageSummarizationUsedCountPerConversation = safeDivideFloat(v.TotalSummarizationUsedCount, v.TotalConversationCount)
		v.AverageSummarizationUsedCountPerFrequencyPerUser = safeDivideFloat(v.TotalSummarizationUsedCount, groupByAttributeToUserCount[k])
		v.AverageSummarizationGeneratedCountPerFrequencyPerUser = safeDivideFloat(v.TotalSummarizationGeneratedCount, groupByAttributeToUserCount[k])
		// Sort result based on timestamp.
		sort.Slice(v.SummarizationStats, func(i, j int) bool {
			return v.SummarizationStats[i].TimeRange.StartTimestamp.AsTime().Before(v.SummarizationStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		resp.TotalConversationCount += v.TotalConversationCount
		resp.TotalSummarizationUsedCount += v.TotalSummarizationUsedCount
		resp.TotalSummarizationGeneratedCount += v.TotalSummarizationGeneratedCount
		resp.SummarizationStatsResults = append(resp.SummarizationStatsResults, v)
	}
	resp.SummarizationStatsResults = sortResults(resp.SummarizationStatsResults)
	resp.AverageSummarizationUsedRatio = min(safeDivideFloat(resp.TotalSummarizationUsedCount, resp.TotalSummarizationGeneratedCount), 1) * 100
	resp.AverageSummarizationUsedCountPerConversation = safeDivideFloat(resp.TotalSummarizationUsedCount, resp.TotalConversationCount)
	resp.AverageSummarizationUsedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalSummarizationUsedCount, totalUserFrequencyCount)
	resp.AverageSummarizationGeneratedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalSummarizationGeneratedCount, totalUserFrequencyCount)
	return resp, nil
}
