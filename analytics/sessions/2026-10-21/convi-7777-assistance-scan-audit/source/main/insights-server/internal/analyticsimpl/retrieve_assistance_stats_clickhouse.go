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

type assistanceStatsRow struct {
	clickhouseKeyTimeRow
	convoCount        uint64
	userCount         uint64
	hintFollowedCount uint64
}

// assistanceStatsClickhouseQuery counts conversations, agents, and followed hints
// over a window.
//
// Every aggregate here counts distinct ids. moment_annotation_d is distributed
// over a base table, so ReplacingMergeTree has not necessarily collapsed a
// re-ingested annotation yet and a plain count reports one followed hint twice:
// over a 7-day window oportun holds 196,624 adherence rows for 173,439 distinct
// moment_annotation_ids.
//
// A followed hint is one adherence_action_annotation_id, not one moment
// annotation: several moments can adhere to the same hint. Same grain as
// RetrieveHintStats (CONVI-7387). frontdoor, same 7 days: 70,198 distinct
// moment annotations for 60,270 distinct hints.
func assistanceStatsClickhouseQuery(conditionsAndArgs map[ClickhouseTable][]conditionAndArg, groupByKeys []string, groupByQueries map[ClickhouseTable][]string, momentConditionsAndArgsList [][]conditionAndArg, excludedMomentConditionsAndArgs [][]conditionAndArg) (string, []any) {
	allArgs := []any{}

	// Check if we need metadata filtering
	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	// Generate metadata filters using refactored functions
	aliasDict := map[string]string{
		"message":           "message_d",
		"momentAnnotation":  "moment_annotation_d",
		"conversationEvent": "conversation_event_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false, true)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	// Build base conditions and arguments
	msgCondition, msgArgs := concatConditionsAndArgs(conditionsAndArgs[messageTable])
	maCondition, maArgs := concatConditionsAndArgs(conditionsAndArgs[momentAnnotationTable])
	ceCondition, ceArgs := concatConditionsAndArgs(conditionsAndArgs[conversationEventTable])

	msgGroupByQueries, maGroupByQueries, ceGroupByQueries := concatQueries(groupByQueries[messageTable]), concatQueries(groupByQueries[momentAnnotationTable]), concatQueries(groupByQueries[conversationEventTable])

	finalJoinKeys, finalGroupByQueries := concatKeys(groupByKeys), concatQueries(groupByKeys)

	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}

	allArgs = append(allArgs, msgArgs...)
	allArgs = append(allArgs, maArgs...)
	allArgs = append(allArgs, ceArgs...)

	if hasMetadataFiltering {
		return fmt.Sprintf(`
			WITH
			%s
			%s
			conversation_count AS (
				SELECT
					%s
					COUNT(DISTINCT conversation_id) AS convo_count,
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM
					message_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE 
					conversation_source in (0, 8)
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
					%s
				%s
			),
			hint_followed AS(
				SELECT
					%s
					COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count
				FROM
					moment_annotation_d
				%s
				%s
				WHERE
					adherence_type = 2
					AND adherence_action_annotation_id <> ''
					AND (conversation_source in (0, 8) OR conversation_source IS NULL)
					AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
					%s
				%s
				UNION ALL
				SELECT
					%s
					COUNT(DISTINCT action_annotation_id) as hint_followed_count
				FROM
					conversation_event_d
				%s
				%s
				WHERE
					event_type = 9
					AND (conversation_source in (0, 8) OR conversation_source IS NULL)
					AND action_policy_source_type=1
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
					%s
				%s
			)
			SELECT
				%s
				MAX(conversation_count.convo_count) AS convo_count,
				MAX(conversation_count.user_count) AS user_count,
				SUM(hint_followed.hint_followed_count) AS hint_followed_count
			FROM
				conversation_count LEFT JOIN hint_followed USING (%s)
			%s
			`, metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
			// conversation_count
			msgGroupByQueries, metadataFilters.MomentJoinQueries["message"], metadataFilters.ExcludedMomentJoinQueries["message"], msgCondition, metadataFilters.ExcludedMomentWhereQueries, optionalGroupBy,
			// hint_followed moment_annotation_d
			maGroupByQueries, metadataFilters.MomentJoinQueries["momentAnnotation"], metadataFilters.ExcludedMomentJoinQueries["momentAnnotation"], maCondition, metadataFilters.ExcludedMomentWhereQueries, optionalGroupBy,
			// hint_followed conversation_event_d
			ceGroupByQueries, metadataFilters.MomentJoinQueries["conversationEvent"], metadataFilters.ExcludedMomentJoinQueries["conversationEvent"], ceCondition, metadataFilters.ExcludedMomentWhereQueries, optionalGroupBy,
			// final select
			finalGroupByQueries, finalJoinKeys, optionalGroupBy), allArgs
	} else {
		return fmt.Sprintf(`
			WITH
			conversation_count AS (
				SELECT
					%s
					COUNT(DISTINCT conversation_id) AS convo_count,
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM
					message_d
				WHERE 
					conversation_source in (0, 8)
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
				%s
			),
			hint_followed AS(
				SELECT
					%s
					COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count
				FROM
					moment_annotation_d
				WHERE
					adherence_type = 2
					AND adherence_action_annotation_id <> ''
					AND (conversation_source in (0, 8) OR conversation_source IS NULL)
					AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
				%s
				UNION ALL
				SELECT
					%s
					COUNT(DISTINCT action_annotation_id) as hint_followed_count
				FROM
					conversation_event_d
				WHERE
					event_type = 9
					AND (conversation_source in (0, 8) OR conversation_source IS NULL)
					AND action_policy_source_type=1
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s -- Additional filters
				%s
			)
			SELECT
				%s
				MAX(conversation_count.convo_count) AS convo_count,
				MAX(conversation_count.user_count) AS user_count,
				SUM(hint_followed.hint_followed_count) AS hint_followed_count
			FROM
				conversation_count LEFT JOIN hint_followed USING (%s)
			%s
			`, msgGroupByQueries, msgCondition, optionalGroupBy,
			maGroupByQueries, maCondition, optionalGroupBy,
			ceGroupByQueries, ceCondition, optionalGroupBy,
			finalGroupByQueries, finalJoinKeys, optionalGroupBy), allArgs
	}
}

func (a AnalyticsServiceImpl) readAssistanceStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveAssistanceStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveAssistanceStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		messageTable,
		momentAnnotationTable,
		conversationEventTable,
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
	chQuery, chArgs := assistanceStatsClickhouseQuery(conditionsAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
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

	chRows := []assistanceStatsRow{}
	for rows.Next() {
		row := assistanceStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case policyIDColumn:
				vals = append(vals, &row.policyID)
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.convoCount, &row.userCount, &row.hintFollowedCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[assistance] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveAssistanceStats in seconds %v", time.Since(startT).Seconds())
	return a.convertCHResponseToAssistanceStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToAssistanceStatsResponse(
	ctx context.Context,
	customerID, profileID string,
	rows []assistanceStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveAssistanceStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveAssistanceStatsResponse{}, nil
	}
	groupByAttributeToAssistanceStatsResult := make(map[groupByAttribute]*analyticspb.AssistanceStatsResult)
	for _, row := range rows {
		hintStats := analyticspb.HintStats{
			TimeRange:                                   &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalConversationCount:                      int32(row.convoCount),
			TotalHintFollowedCount:                      int32(row.hintFollowedCount),
			AverageHintFollowedCountPerConversation:     safeDivideFloat(int32(row.hintFollowedCount), int32(row.convoCount)),
			AverageHintFollowedCountPerFrequencyPerUser: safeDivideFloat(int32(row.hintFollowedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToAssistanceStatsResult[groupBy]; !exists {
			groupByAttributeToAssistanceStatsResult[groupBy] = &analyticspb.AssistanceStatsResult{}
		}
		groupByAttributeToAssistanceStatsResult[groupBy].HintStats = append(groupByAttributeToAssistanceStatsResult[groupBy].HintStats, &hintStats)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToAssistanceStatsResult {
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

	totalUserFrequenyCount := int32(0)
	totalConversationCount := int32(0)
	resp := &analyticspb.RetrieveAssistanceStatsResponse{}
	for k, v := range groupByAttributeToAssistanceStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		conversationCount := int32(0)
		for _, hintStat := range v.HintStats {
			v.TotalHintFollowedCount += hintStat.TotalHintFollowedCount
			conversationCount += hintStat.TotalConversationCount
		}
		userFrequencyCount := int32(len(v.HintStats))
		v.AverageHintFollowedCountPerConversation = safeDivideFloat(v.TotalHintFollowedCount, conversationCount)
		v.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(v.TotalHintFollowedCount, userFrequencyCount)
		// Sort result based on timestamp.
		sort.Slice(v.HintStats, func(i, j int) bool {
			return v.HintStats[i].TimeRange.StartTimestamp.AsTime().Before(v.HintStats[j].TimeRange.StartTimestamp.AsTime())
		})

		totalUserFrequenyCount += userFrequencyCount
		totalConversationCount += conversationCount
		resp.AssistanceStatsResults = append(resp.AssistanceStatsResults, v)
		resp.TotalHintFollowedCount += v.TotalHintFollowedCount
	}
	resp.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalHintFollowedCount, totalUserFrequenyCount)
	resp.AverageHintFollowedCountPerConversation = safeDivideFloat(resp.TotalHintFollowedCount, totalConversationCount)
	return resp, nil
}
