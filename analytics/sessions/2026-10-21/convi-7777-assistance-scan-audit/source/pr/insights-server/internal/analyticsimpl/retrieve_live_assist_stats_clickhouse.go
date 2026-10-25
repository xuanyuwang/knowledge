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

func liveAssistStatsClickHouseQuery(conditionsAndArgs map[ClickhouseTable][]conditionAndArg, groupByKeys []string, groupByQueries map[ClickhouseTable][]string) (string, []any) {
	groupBy := concatKeys(groupByKeys)
	aaConditions := conditionsAndArgs[actionAnnotationTable]
	for i := 0; i < len(aaConditions); i++ {
		c := aaConditions[i]
		if strings.Contains(c.condition, agentUserIDColumn) {
			cm := strings.ReplaceAll(c.condition, agentUserIDColumn, "manager_user_id")
			c.condition = strings.Join([]string{c.condition, cm}, " OR ")
			if c.arg != nil {
				c.args = append(c.args, c.arg)
			}
			aaConditions[i] = c
		}
	}
	aaCondition, aaArgs := concatConditionsAndArgs(aaConditions)

	aaGroupByQueries := concatQueries(groupByQueries[actionAnnotationTable])
	aaGroupByQueriesForManagerStats := strings.ReplaceAll(aaGroupByQueries, agentUserIDColumn, fmt.Sprintf("%s AS %s", "manager_user_id", agentUserIDColumn))
	return fmt.Sprintf(`
		WITH raised_hands_and_whispers AS (
			SELECT
				conversation_start_time,
				conversation_id,
				agent_user_id,
				-- The format of payload.manager_whisper.manager_user_id can be resource name.
				(CASE WHEN payload.manager_whisper.manager_user_id like '%%/%%' THEN substringIndex(payload.manager_whisper.manager_user_id,'/',-1) ELSE payload.manager_whisper.manager_user_id END) AS manager_user_id,
				MAX(CASE WHEN action_type = 7 THEN 1 ELSE 0 END) AS has_raised_hand,	-- 7: AGENT_RAISE_HAND
				MAX(CASE WHEN action_type = 12 THEN 1 ELSE 0 END) AS has_whisper	-- 12: MANAGER_WHISPER
			FROM
				action_annotation_d
			WHERE
				conversation_source IN (0, 8)
				AND action_type IN (7, 12)
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND %s	-- Additional filters
			GROUP BY 1,2,3,4
		),
		agent_live_assist_stats AS (
			SELECT
				%s
				COUNT(DISTINCT conversation_id) FILTER (WHERE has_raised_hand = 1) AS raised_hand_count, 
				COUNT(DISTINCT conversation_id) FILTER (WHERE has_raised_hand = 1 AND has_whisper = 1) AS raised_hand_answered_count, 
				COUNT(DISTINCT conversation_id) FILTER (WHERE has_whisper = 1) AS whispered_to_agent_count,
				0 AS whisper_by_manager_count
			FROM
				raised_hands_and_whispers
			GROUP BY %s
		),
		manager_live_assist_stats AS (
			SELECT
				%s
				0 AS raised_hand_count, 
				0 AS raised_hand_answered_count, 
				0 AS whispered_to_agent_count,
				COUNT(DISTINCT conversation_id) FILTER (WHERE has_whisper = 1) AS whisper_by_manager_count
			FROM
				raised_hands_and_whispers
			GROUP BY %s
		)
		SELECT * FROM agent_live_assist_stats 
		UNION ALL 
		SELECT * FROM manager_live_assist_stats
	`,
		// raised_hands_and_whispers
		aaCondition,
		// agent_live_assist_stats
		aaGroupByQueries, groupBy,
		// manager_live_assist_stats
		aaGroupByQueriesForManagerStats, groupBy,
	), aaArgs
}

func (a AnalyticsServiceImpl) readLiveAssistStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveLiveAssistStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		actionAnnotationTable,
	}
	conditionsAndArgs, _, _, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
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

	chQuery, chArgs := liveAssistStatsClickHouseQuery(conditionsAndArgs, groupByKeys, groupByQueries)
	a.logger.Infof(ctx, "clickhouse query: %v \n with args %v", chQuery, chArgs)

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

	chRows := []liveAssistStatsRow{}
	for rows.Next() {
		row := liveAssistStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.raisedHandCount, &row.raisedHandAnsweredCount, &row.whisperedToAgentCount, &row.whisperByManagerCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[liveAssistStats] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveLiveAssistStats in seconds %v", time.Since(startT).Seconds())
	return a.convertCHResponseToLiveAssistStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToLiveAssistStatsResponse(
	ctx context.Context,
	customerID, profileID string,
	rows []liveAssistStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveLiveAssistStatsResponse{}, nil
	}
	groupByAttributeToLiveAssistStatsResults := make(map[groupByAttribute]*analyticspb.LiveAssistStatsResult)
	for _, row := range rows {
		liveAssistStats := analyticspb.LiveAssistStats{
			TimeRange:                    &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalRaisedHandCount:         int32(row.raisedHandCount),
			TotalRaisedHandAnsweredCount: int32(row.raisedHandAnsweredCount),
			TotalWhisperedToAgentCount:   int32(row.whisperedToAgentCount),
			TotalWhisperByManagerCount:   int32(row.whisperByManagerCount),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToLiveAssistStatsResults[groupBy]; !exists {
			groupByAttributeToLiveAssistStatsResults[groupBy] = &analyticspb.LiveAssistStatsResult{}
		}
		groupByAttributeToLiveAssistStatsResults[groupBy].LiveAssistStats = append(groupByAttributeToLiveAssistStatsResults[groupBy].LiveAssistStats, &liveAssistStats)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToLiveAssistStatsResults {
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

	resp := &analyticspb.RetrieveLiveAssistStatsResponse{}
	for k, v := range groupByAttributeToLiveAssistStatsResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr

		for _, liveAssistStats := range v.LiveAssistStats {
			v.TotalRaisedHandCount += liveAssistStats.TotalRaisedHandCount
			v.TotalRaisedHandAnsweredCount += liveAssistStats.TotalRaisedHandAnsweredCount
			v.TotalWhisperedToAgentCount += liveAssistStats.TotalWhisperedToAgentCount
			v.TotalWhisperByManagerCount += liveAssistStats.TotalWhisperByManagerCount
		}

		// Sort result based on timestamp.
		sort.Slice(v.LiveAssistStats, func(i, j int) bool {
			return v.LiveAssistStats[i].TimeRange.StartTimestamp.AsTime().Before(v.LiveAssistStats[j].TimeRange.StartTimestamp.AsTime())
		})

		resp.TotalRaisedHandCount += v.TotalRaisedHandCount
		resp.TotalRaisedHandAnsweredCount += v.TotalRaisedHandAnsweredCount
		resp.TotalWhisperedToAgentCount += v.TotalWhisperedToAgentCount
		resp.TotalWhisperByManagerCount += v.TotalWhisperByManagerCount
		resp.LiveAssistResults = append(resp.LiveAssistResults, v)
	}
	resp.LiveAssistResults = sortResults(resp.LiveAssistResults)
	return resp, nil
}

type liveAssistStatsRow struct {
	clickhouseKeyTimeRow
	raisedHandCount         uint64
	raisedHandAnsweredCount uint64
	whisperedToAgentCount   uint64
	whisperByManagerCount   uint64
}
