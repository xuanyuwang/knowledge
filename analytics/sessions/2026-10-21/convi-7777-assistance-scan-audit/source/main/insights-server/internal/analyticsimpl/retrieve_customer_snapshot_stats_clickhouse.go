package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// ClickHouse ConversationEventType values for the customer snapshot panel
// events (see cresta/nonpublic/clickhouse/events/event_enums.proto):
//   27 = CUSTOMER_SNAPSHOT_PRESENTED
//   28 = CUSTOMER_SNAPSHOT_CLOSED
// Both manual and automatic close triggers are paired: the panel visibility
// interval ends on any close, regardless of trigger.

// customerSnapshotStatsRow is an aggregated display-time bucket: the summed
// display time and conversation count for one (agent, truncated time bucket)
// group. Produced by the ClickHouse query.
type customerSnapshotStatsRow struct {
	clickhouseKeyTimeRow
	totalDisplaySeconds            uint64
	totalConversationsWithSnapshot uint64
	userCount                      uint64
}

// customerSnapshotStatsClickhouseQuery builds the display-time metric query.
// The filtered conversations + events CTEs and positional args are shared, so
// the pairing and merging run entirely in ClickHouse and only the small
// aggregated result set returns to the process.
//
// momentConditionsAndArgsList / excludedMomentConditionsAndArgs carry
// FilterByAttribute.MomentGroups (metadata-moment annotation filters), turned
// into CTE/JOIN fragments by generateMetadataFilters. When present they
// restrict both the conversations and cs_events sets, mirroring
// RetrieveGuidedWorkflowStats. Positional args are ordered: metadata-filter
// args, then conversation conditions, then conversation-event conditions.
//
// Pairing is per display session (conversation_id, session_id, content_hash):
// each session's interval spans the first PRESENTED (minIf) to the last CLOSED
// (maxIf), and sessions missing either a presented or a closed event, or whose
// last closed precedes the first presented, are dropped (HAVING). In practice
// each session_id is a unique per-display UUID with exactly one PRESENTED and one
// CLOSED (verified against staging data), so minIf/maxIf is equivalent to a
// positional present/close zip. Overlapping or adjacent visibility intervals are
// then merged with an islands-and-gaps pass so the summed duration does not
// double-count overlapping windows. The per-conversation total display time is
// the sum of the merged interval durations.
func customerSnapshotStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (metricQuery string, args []any) {
	aliasDict := map[string]string{
		"conversation":      "conversation_d",
		"conversationEvent": "conversation_event_d",
	}
	// isLastCTE=false so the generated sub-queries get a trailing comma when
	// present (more CTEs follow in eventsCTE); empty strings when absent.
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false, true)
	args = append(args, metadataFilters.AllArgs...)

	convConditions, convArgs := concatConditionsAndArgs(conditionsAndArgs[conversationTable])
	convCondition := convConditions
	if convCondition != "" {
		convCondition = "AND " + convCondition
	}
	ceConditions, ceArgs := concatConditionsAndArgs(conditionsAndArgs[eventConversationEventTable])
	ceCondition := ceConditions
	if ceCondition != "" {
		ceCondition = "AND " + ceCondition
	}
	ceGroupByQueries := concatQueries(groupByQueries[eventConversationEventTable])
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	args = append(args, convArgs...)
	args = append(args, ceArgs...)

	// eventsCTE holds the shared filtered conversations + events CTEs. The
	// moment-annotation JOINs and exclude WHERE are spliced into both the
	// conversations and cs_events CTEs so the metadata-moment cohort restricts
	// both sides; ev then joins the already-filtered sets.
	eventsCTE := fmt.Sprintf(`
	WITH
	%s
	%s
	conversations AS (
		SELECT DISTINCT conversation_id
		FROM conversation_d
		-- Join moment annotation filters to include criteria
		%s
		-- Join moment annotation filters to exclude criteria
		%s
		WHERE agent_user_id <> ''
			%s
			%s
	),
	cs_events AS (
		SELECT
			conversation_id,
			agent_user_id,
			conversation_start_time,
			event_type,
			toUnixTimestamp(create_time) AS ts,
			-- Typed Object('json') subcolumn access: reads only the needed
			-- sessionId/contentHash subcolumns instead of materializing the full
			-- event_payload JSON string, and CAST normalizes a missing field to ''
			-- (matching JSONExtractString). ifNull is not required.
			if(event_type = 27,
				CAST(event_payload.customerSnapshotPresented.sessionId AS String),
				CAST(event_payload.customerSnapshotClosed.sessionId   AS String)) AS session_id,
			if(event_type = 27,
				CAST(event_payload.customerSnapshotPresented.contentHash AS String),
				CAST(event_payload.customerSnapshotClosed.contentHash   AS String)) AS content_hash
		FROM conversation_event_d
		-- Join moment annotation filters to include criteria
		%s
		-- Join moment annotation filters to exclude criteria
		%s
		WHERE agent_user_id <> ''
			AND event_type IN (27, 28)
			%s
			%s
	),
	ev AS (
		SELECT
			conversation_id,
			agent_user_id,
			conversation_start_time,
			event_type,
			ts,
			session_id,
			content_hash
		FROM cs_events JOIN conversations USING (conversation_id)
	)`,
		metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
		metadataFilters.MomentJoinQueries["conversation"], metadataFilters.ExcludedMomentJoinQueries["conversation"], convCondition, metadataFilters.ExcludedMomentWhereQueries,
		metadataFilters.MomentJoinQueries["conversationEvent"], metadataFilters.ExcludedMomentJoinQueries["conversationEvent"], ceCondition, metadataFilters.ExcludedMomentWhereQueries)

	metricQuery = eventsCTE + fmt.Sprintf(`,
	-- Per-session pre-aggregation: one interval per (conversation, session,
	-- content_hash), from the first presented to the last closed. Sessions
	-- without both a presented and a closed event, or where the last close
	-- precedes the first present, are dropped. This pre-aggregation shrinks
	-- the input to the overlap-merge below from per-event to per-session rows.
	session_intervals AS (
		SELECT
			conversation_id,
			session_id,
			any(agent_user_id)           AS agent_user_id,
			any(conversation_start_time) AS conversation_start_time,
			minIf(ts, event_type = 27)   AS start_ts,
			maxIf(ts, event_type = 28)   AS end_ts
		FROM ev
		GROUP BY conversation_id, session_id, content_hash
		HAVING countIf(event_type = 27) > 0
			AND countIf(event_type = 28) > 0
			AND end_ts >= start_ts
	),
	-- Merge overlapping/adjacent session intervals (islands-and-gaps) so the
	-- summed duration does not double-count overlapping visibility windows.
	ordered AS (
		SELECT conversation_id, agent_user_id, conversation_start_time, start_ts, end_ts, session_id,
			max(end_ts) OVER (PARTITION BY conversation_id ORDER BY start_ts, end_ts, session_id
				ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS prev_max_end
		FROM session_intervals
	),
	islands AS (
		SELECT conversation_id, agent_user_id, conversation_start_time, start_ts, end_ts, session_id,
		sum(if(prev_max_end = 0 OR start_ts > prev_max_end, 1, 0)) OVER
				(PARTITION BY conversation_id ORDER BY start_ts, end_ts, session_id
				ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS island_id
		FROM ordered
	),
	merged AS (
		SELECT conversation_id,
			any(agent_user_id)           AS agent_user_id,
			any(conversation_start_time) AS conversation_start_time,
			island_id, min(start_ts) AS s, max(end_ts) AS e
		FROM islands
		GROUP BY conversation_id, island_id
	),
	per_conversation AS (
		SELECT conversation_id,
			any(agent_user_id)           AS agent_user_id,
			any(conversation_start_time) AS conversation_start_time,
			-- toUInt64 keeps the summed duration UInt64 (bare e-s promotes to
			-- Int64), matching the uint64 row field the driver scans into.
			sum(toUInt64(e - s))         AS display_secs
		FROM merged
		GROUP BY conversation_id
	)
	SELECT
		%s
		sum(display_secs) AS total_display_seconds,
		count()           AS total_conversations_with_snapshot,
		COUNT(DISTINCT agent_user_id) AS user_count
	FROM per_conversation
	%s`, ceGroupByQueries, optionalGroupBy)

	return metricQuery, args
}

func (a AnalyticsServiceImpl) readCustomerSnapshotStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveCustomerSnapshotStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveCustomerSnapshotStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		conversationTable,
		eventConversationEventTable,
	}
	conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
	if err != nil {
		return nil, err
	}

	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, targetTables)
	if err != nil {
		return nil, err
	}
	// Only agent/group and truncated time group-by keys are supported, mirroring
	// RetrieveKnowledgeAssistStats.
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "group_by keys %v are not supported", v)
	}

	metricQuery, chArgs := customerSnapshotStatsClickhouseQuery(conditionsAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
	a.logger.Infof(ctx, "clickhouse metric query: %v", metricQuery)

	// Metadata-moment filters join the conversations table, which lives on the
	// conversations connection, so switch off the events connection when active.
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
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, metricQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse for customer snapshot stats: %w", err)
	}
	defer rows.Close()

	statsRows := []customerSnapshotStatsRow{}
	for rows.Next() {
		row := customerSnapshotStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.totalDisplaySeconds, &row.totalConversationsWithSnapshot, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[customerSnapshotStats] scanning clickhouse rows: %w", err)
		}
		statsRows = append(statsRows, row)
	}
	a.logger.Infof(ctx, "[customerSnapshotStats] ClickHouse (events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToCustomerSnapshotStatsResponse(ctx, parent.CustomerID, parent.ProfileID, statsRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToCustomerSnapshotStatsResponse(
	ctx context.Context,
	customerID, profileID string,
	rows []customerSnapshotStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveCustomerSnapshotStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveCustomerSnapshotStatsResponse{}, nil
	}
	groupByAttributeToResult := make(map[groupByAttribute]*analyticspb.CustomerSnapshotStatsResult)
	// groupByAttributeToUserCount accumulates the per-row distinct-agent count
	// (user x frequency-bucket combinations) per group-by slice; it is the
	// denominator for average_display_time_per_frequency_per_user at the result
	// and top levels, mirroring RetrieveGuidedWorkflowStats.
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)
	for _, row := range rows {
		bucketStat := analyticspb.CustomerSnapshotStats{
			TimeRange:                             &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalDisplayTime:                      int64(row.totalDisplaySeconds),
			AverageDisplayTime:                    safeDivideFloat64(int64(row.totalDisplaySeconds), int64(row.totalConversationsWithSnapshot)),
			AverageDisplayTimePerFrequencyPerUser: safeDivideFloat64(int64(row.totalDisplaySeconds), int64(row.userCount)),
			TotalConversationsWithSnapshot:        int32(row.totalConversationsWithSnapshot),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToResult[groupBy]; !exists {
			groupByAttributeToResult[groupBy] = &analyticspb.CustomerSnapshotStatsResult{}
			groupByAttributeToUserCount[groupBy] = 0
		}
		groupByAttributeToResult[groupBy].CustomerSnapshotStats = append(groupByAttributeToResult[groupBy].CustomerSnapshotStats, &bucketStat)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, user := range users {
		usernameToUserMap[user.Name] = user
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for groupBy := range groupByAttributeToResult {
		attribute := groupBy.GetAttribute(customerID, profileID)
		if len(attribute.Users) > 0 {
			if user, exists := usernameToUserMap[attribute.Users[0].Name]; exists {
				attribute.Users[0].FullName = user.FullName
				attribute.Users[0].Username = user.Username
			}
		}
		groupByAttributeToAttribute[groupBy] = attribute
		attributes = append(attributes, attribute)
	}

	ignoredAttributes, err := attributesToIgnore(a.appsDB.DB(ctx), attributes)
	if err != nil {
		return nil, err
	}

	totalUserFrequencyCount := int32(0)
	resp := &analyticspb.RetrieveCustomerSnapshotStatsResponse{}
	for groupBy, result := range groupByAttributeToResult {
		attribute := groupByAttributeToAttribute[groupBy]
		if ignoredAttributes[attribute] {
			continue
		}
		result.Attribute = attribute
		for _, bucketStat := range result.CustomerSnapshotStats {
			result.TotalDisplayTime += bucketStat.TotalDisplayTime
			result.TotalConversationsWithSnapshot += bucketStat.TotalConversationsWithSnapshot
		}
		result.AverageDisplayTime = safeDivideFloat64(result.TotalDisplayTime, int64(result.TotalConversationsWithSnapshot))
		result.AverageDisplayTimePerFrequencyPerUser = safeDivideFloat64(result.TotalDisplayTime, int64(groupByAttributeToUserCount[groupBy]))
		sort.Slice(result.CustomerSnapshotStats, func(i, j int) bool {
			return result.CustomerSnapshotStats[i].TimeRange.StartTimestamp.AsTime().Before(result.CustomerSnapshotStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserFrequencyCount += groupByAttributeToUserCount[groupBy]
		resp.CustomerSnapshotStatsResults = append(resp.CustomerSnapshotStatsResults, result)
		resp.TotalDisplayTime += result.TotalDisplayTime
		resp.TotalConversationsWithSnapshot += result.TotalConversationsWithSnapshot
	}
	resp.CustomerSnapshotStatsResults = sortResults(resp.CustomerSnapshotStatsResults)
	resp.AverageDisplayTime = safeDivideFloat64(resp.TotalDisplayTime, int64(resp.TotalConversationsWithSnapshot))
	resp.AverageDisplayTimePerFrequencyPerUser = safeDivideFloat64(resp.TotalDisplayTime, int64(totalUserFrequencyCount))
	return resp, nil
}
