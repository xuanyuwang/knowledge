package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

func (a AnalyticsServiceImpl) readAgentStatsFromClickhouseConvWithLabel(
	ctx context.Context,
	req *analyticspb.RetrieveAgentStatsRequest,
	profileID string,
	useCrestaConversationCount bool,
	users []*userpb.User,
	includeBotConversations bool,
	includeIngestionPipeline bool,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
	parent, err := shared.ParseCustomer(req.Parent)
	if err != nil {
		return nil, err
	}
	dbProvider, err := a.appsDB.GetProvider(parent.CustomerID, profileID)
	if err != nil {
		return nil, err
	}
	db := dbProvider.Get(ctx)

	chRows, err := a.queryAgentStatsFromConversationWithLabels(
		ctx, req, parent.CustomerID, profileID, includeBotConversations, includeIngestionPipeline,
	)
	if err != nil {
		return nil, err
	}
	return convertAgentStatsFromConvWithLabelsToAgentStatsResponse(
		parent.CustomerID, profileID,
		db, useCrestaConversationCount, chRows, users,
	)
}

func (a AnalyticsServiceImpl) queryAgentStatsFromConversationWithLabels(
	ctx context.Context,
	req *analyticspb.RetrieveAgentStatsRequest,
	customerID, profileID string,
	includeBotConversations bool,
	includeIngestionPipeline bool,
) ([]agentStatsWithAARow, error) {
	targetTables := []ClickhouseTable{
		conversationTable,
		conversationLabelTable,
	}
	conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
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

	chConvoQuery, chConvoArgs := agentConversationWithLabelCountClickhouseQuery(
		conditionsAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, includeBotConversations, includeIngestionPipeline, conversationStartTimeColumn,
	)
	a.logger.Infof(ctx, "clickhouse query for agentStats (conversation): %v", chConvoQuery)
	a.logger.Infof(ctx, "clickhouse query for agentStats (conversation) with args: %v", combineQueryAndArgsForLogging(chConvoQuery, chConvoArgs))

	conn, err := a.getConversationsClickhouseConn(ctx, customerID, profileID)
	if err != nil {
		return nil, err
	}

	ctx = attachExtTablesToContext(ctx, usersExtTable)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chConvoQuery, chConvoArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	chConvoRows := []agentStatsWithAARow{}
	for rows.Next() {
		row := agentStatsWithAARow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.totalAgentCount)
		vals = append(vals, &row.activeAgentCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[agentStats] scanning clickhouse (conversations) rows: %v", err)
		}
		chConvoRows = append(chConvoRows, row)
	}
	a.logger.Infof(ctx, "[agentStats] ClickHouse (conversations) query latency in seconds %v", time.Since(startT).Seconds())

	return chConvoRows, nil
}

func agentConversationWithLabelCountClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
	includeBotConversations bool,
	includeIngestionPipeline bool,
	conversationTimeColumn string,
) (string, []any) {
	allArgs := []any{}

	// Check if we need metadata filtering
	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	// Generate metadata filters using refactored functions
	aliasDict := map[string]string{
		"conversation":          "conversation_d",
		"conversationWithLabel": "conversation_with_labels_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false, true)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	convCondition, convArgs := concatConditionsAndArgs(conditionsAndArgs[conversationTable])
	convGroupByQueries := concatQueries(groupByQueries[conversationTable])

	// Split label conditions into time-range and non-time so we can apply the
	// backward-compatible message_time filter in convs_with_aa. New per-message
	// rows use message_time for the time filter and time bucketing; old
	// per-conversation rows (message_id='') fall back to conversationTimeColumn.
	labelConds := conditionsAndArgs[conversationLabelTable]
	convTimeCond, convTimeArgs, msgTimeCond, msgTimeArgs, nonTimeCond, nonTimeArgs :=
		agentAssistLabelTimeFilter(labelConds, conversationTimeColumn)

	// nonTimeCondClause is either "AND <agent-filter>" or empty when there is no agent filter.
	nonTimeCondClause := ""
	if nonTimeCond != "" {
		nonTimeCondClause = "AND " + nonTimeCond
	}

	// Replace conversationTimeColumn with IF(message_id != '', message_time, conversationTimeColumn)
	// in the GROUP BY SELECT expressions so new rows are bucketed by message time.
	ifTimeExpr := fmt.Sprintf("IF(message_id != '', message_time, %s)", conversationTimeColumn)
	labelGroupByWithMsgTime := make([]string, len(groupByQueries[conversationLabelTable]))
	for i, q := range groupByQueries[conversationLabelTable] {
		labelGroupByWithMsgTime[i] = strings.Replace(q, conversationTimeColumn, ifTimeExpr, 1)
	}
	convWithLabelGroupByQueries := concatQueries(labelGroupByWithMsgTime)

	optionalGroupBy := ""
	joinKeys := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
		joinKeys = concatKeys(groupByKeys)
	}
	crestaBotStr := ""
	if includeBotConversations {
		crestaBotStr = crestaBotSourceStr
	}
	// Build conversation source filter based on flags
	// Default: (0, 8) - SOURCE_UNSPECIFIED and INGESTION_PIPELINE
	// With includeIngestionPipeline=false: (0) - only SOURCE_UNSPECIFIED
	// With includeBotConversations: adds 7 (BOT)
	ingestionPipelineStr := ""
	if includeIngestionPipeline {
		ingestionPipelineStr = ", 8"
	}

	allArgs = append(allArgs, convArgs...)
	// Args for convs_with_aa WHERE: non-time conditions (agent filter), then message_time filter (new rows), then conv time filter (old rows).
	allArgs = append(allArgs, append(append(nonTimeArgs, msgTimeArgs...), convTimeArgs...)...)

	if hasMetadataFiltering {
		return fmt.Sprintf(`
			WITH
			%s
			%s
			convs AS (
				SELECT
					%s
					COUNT(DISTINCT agent_user_id) AS total_agent_count
				FROM
					conversation_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					conversation_source in (0%s%s)
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
					%s
				-- GROUP BY is optional
				%s
			),
			convs_with_aa AS (
				SELECT
					%s
					COUNT(DISTINCT agent_user_id) AS active_agent_count
				FROM
					conversation_with_labels_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					agent_user_id <> ''
					AND (has_agent_assistance = 1)
					%s
					AND (
						(message_id != '' AND %s)
						OR (message_id = '' AND %s)
					) -- Additional filters
					%s
				-- GROUP BY is optional
				%s
			)
			SELECT
				%s,
				COALESCE(convs.total_agent_count, 0) AS total_agent_count,
				COALESCE(convs_with_aa.active_agent_count, 0) AS active_agent_count
			FROM
				-- Using FULL OUTER JOIN to preserve rows from both CTEs.
				-- convs may miss days where agent only had non-qualifying source conversations.
				-- convs_with_aa captures agent assist from all sources, indicating agent login.
				convs FULL OUTER JOIN convs_with_aa USING (%s)
		`,
			// Metadata filters
			metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
			// convs CTE
			convGroupByQueries, metadataFilters.MomentJoinQueries["conversation"], metadataFilters.ExcludedMomentJoinQueries["conversation"], ingestionPipelineStr, crestaBotStr, convCondition, metadataFilters.ExcludedMomentWhereQueries, optionalGroupBy,
			// convs_with_aa CTE
			convWithLabelGroupByQueries, metadataFilters.MomentJoinQueries["conversationWithLabel"], metadataFilters.ExcludedMomentJoinQueries["conversationWithLabel"],
			nonTimeCondClause, msgTimeCond, convTimeCond, metadataFilters.ExcludedMomentWhereQueries, optionalGroupBy,
			// select keys and join keys
			joinKeys, joinKeys,
		), allArgs
	}
	return fmt.Sprintf(`
		WITH convs AS (
			SELECT
				%s
				COUNT(DISTINCT agent_user_id) AS total_agent_count
			FROM
				conversation_d
			WHERE
				conversation_source in (0%s%s)
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND %s -- Additional filters
			-- GROUP BY is optional
			%s
		),
		convs_with_aa AS (
			SELECT
				%s
				COUNT(DISTINCT agent_user_id) AS active_agent_count
			FROM
				conversation_with_labels_d
			WHERE
				agent_user_id <> ''
				AND (has_agent_assistance = 1)
				%s
				AND (
					(message_id != '' AND %s)
					OR (message_id = '' AND %s)
				) -- Additional filters
			-- GROUP BY is optional
			%s
		)
		SELECT
			%s,
			COALESCE(convs.total_agent_count, 0) AS total_agent_count,
			COALESCE(convs_with_aa.active_agent_count, 0) AS active_agent_count
		FROM
			-- Using FULL OUTER JOIN to preserve rows from both CTEs.
			-- convs may miss days where agent only had non-qualifying source conversations.
			-- convs_with_aa captures agent assist from all sources, indicating agent login.
			convs FULL OUTER JOIN convs_with_aa USING (%s)
	`,
		// conv sub query
		convGroupByQueries, ingestionPipelineStr, crestaBotStr, convCondition, optionalGroupBy,
		// conv_with_aa sub query
		convWithLabelGroupByQueries, nonTimeCondClause, msgTimeCond, convTimeCond, optionalGroupBy,
		// select keys and join keys
		joinKeys, joinKeys,
	), allArgs
}

func convertAgentStatsFromConvWithLabelsToAgentStatsResponse(
	customerID, profileID string,
	db *gorm.DB,
	useCrestaConversationWithAACount bool,
	chConvoRows []agentStatsWithAARow,
	users []*userpb.User,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
	if len(chConvoRows) == 0 {
		return &analyticspb.RetrieveAgentStatsResponse{}, nil
	}
	groupByAttributeToAgentStatsResults := make(map[groupByAttribute]*analyticspb.AgentStatsResult)
	// Construct AgentStats for each frequency
	for _, row := range chConvoRows {
		// agentCount is determined by both totalAgentCount (valid source conversations)
		// and activeAgentCount (conversations with agent assistance).
		// With FULL OUTER JOIN, a row can exist from convs_with_aa alone (agent logged in
		// via source 8) with totalAgentCount=0. In that case, agentCount should be 0
		// because there are no qualifying conversations from valid sources.
		agentCount := row.totalAgentCount
		if useCrestaConversationWithAACount && row.totalAgentCount > 0 {
			agentCount = row.activeAgentCount
		}
		agentStats := analyticspb.AgentStats{
			TimeRange: &analyticspb.TimeRange{
				StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime),
			},
			TotalAgentLoginCount:                int32(agentCount),
			AverageActiveAgentCountPerFrequency: float32(agentCount),
			AverageActiveDayCountPerUser:        float32(agentCount),
			ActiveAgentCount:                    int32(agentCount),
		}
		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToAgentStatsResults[groupBy]; !exists {
			groupByAttributeToAgentStatsResults[groupBy] = &analyticspb.AgentStatsResult{}
		}
		groupByAttributeToAgentStatsResults[groupBy].AgentStats = append(groupByAttributeToAgentStatsResults[groupBy].AgentStats, &agentStats)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToAgentStatsResults {
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

	ignoredAttributes, err := attributesToIgnore(db, attributes)
	if err != nil {
		return nil, err
	}
	resp := &analyticspb.RetrieveAgentStatsResponse{}
	totalUserFrequenyCount := int32(0)
	frequencyKeySet := make(map[int64]struct{})
	for k, v := range groupByAttributeToAgentStatsResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		// Construct group level stats
		for _, agentStats := range v.AgentStats {
			v.TotalAgentLoginCount += agentStats.TotalAgentLoginCount
			v.AverageActiveDayCountPerUser += agentStats.AverageActiveDayCountPerUser
			if agentStats.TotalAgentLoginCount > 0 {
				frequencyKeySet[agentStats.TimeRange.StartTimestamp.Seconds] = struct{}{}
			}
		}
		totalUserFrequenyCount += int32(len(v.AgentStats))
		v.AverageActiveAgentCountPerFrequency = safeDivideFloat(v.TotalAgentLoginCount, int32(len(v.AgentStats)))
		// Each AgentStatsResult represents one agent.
		if v.TotalAgentLoginCount > 0 {
			v.ActiveAgentCount = 1
			resp.ActiveAgentCount += 1
		}

		// Sort result based on timestamp.
		sort.Slice(v.AgentStats, func(i, j int) bool {
			return v.AgentStats[i].TimeRange.StartTimestamp.AsTime().Before(v.AgentStats[j].TimeRange.StartTimestamp.AsTime())
		})

		resp.TotalAgentLoginCount += v.TotalAgentLoginCount
		resp.AgentStatsResults = append(resp.AgentStatsResults, v)
	}
	resp.AgentStatsResults = sortResults(resp.AgentStatsResults)
	resp.AverageActiveDayCountPerUser = safeDivideFloat(resp.TotalAgentLoginCount, resp.ActiveAgentCount)
	resp.AverageActiveAgentCountPerFrequency = safeDivideFloat(resp.TotalAgentLoginCount, int32(len(frequencyKeySet)))
	return resp, nil
}

type agentStatsWithAARow struct {
	clickhouseKeyTimeRow
	totalAgentCount  uint64
	activeAgentCount uint64
}

// only used in logging
func combineQueryAndArgsForLogging(query string, args []any) string {
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			// Enclose string values in single quotes
			query = strings.Replace(query, "?", fmt.Sprintf("'%v'", strings.ReplaceAll(v, "'", "''")), 1)
		case []string:
			// Replace the first occurrence of "?" with the comma-split elements in the slice
			values := []string{}
			for _, sliceV := range v {
				values = append(values, fmt.Sprintf("'%v'", strings.ReplaceAll(sliceV, "'", "''")))
			}
			sort.Slice(values, func(i, j int) bool {
				return values[i] < values[j]
			})
			query = strings.Replace(query, "?", strings.Join(values, ","), 1)
		case []int:
			// Replace the first occurrence of "?" with the comma-split elements in the slice
			values := []string{}
			for _, sliceV := range v {
				values = append(values, fmt.Sprintf("%v", sliceV))
			}
			sort.Slice(values, func(i, j int) bool {
				return values[i] < values[j]
			})
			query = strings.Replace(query, "?", strings.Join(values, ","), 1)
		default:
			// For simple types, replace the first occurrence of "?" with the string representation
			query = strings.Replace(query, "?", fmt.Sprintf("%v", v), 1)
		}
	}
	return query
}
