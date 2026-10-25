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

var (
	ConvWithAa  = "conv_with_aa"
	ConvWithAht = "conv_with_aht"
	ConvStats   = "conv_stats"
)

// agentAssistLabelTimeFilter splits conditions for conversation_with_labels_d into
// time-range conditions (those referencing conversationTimeColumn) and all others.
// It returns:
//   - convTimeCond: time conditions using conversationTimeColumn (for old rows, message_id="")
//   - convTimeArgs: args for convTimeCond
//   - msgTimeCond: same time conditions rewritten to use message_time (for new rows, message_id!="")
//   - msgTimeArgs: args for msgTimeCond (same values as convTimeArgs)
//   - nonTimeCond: all other conditions (e.g. agent filter); empty string when none
//   - nonTimeArgs: args for nonTimeCond
func agentAssistLabelTimeFilter(
	labelConds []conditionAndArg,
	conversationTimeColumn string,
) (convTimeCond string, convTimeArgs []any, msgTimeCond string, msgTimeArgs []any, nonTimeCond string, nonTimeArgs []any) {
	var timeConds, otherConds []conditionAndArg
	for _, ca := range labelConds {
		if strings.Contains(ca.condition, conversationTimeColumn) {
			timeConds = append(timeConds, ca)
		} else {
			otherConds = append(otherConds, ca)
		}
	}

	msgTimeConds := make([]conditionAndArg, len(timeConds))
	for i, ca := range timeConds {
		msgTimeConds[i] = conditionAndArg{
			condition: strings.Replace(ca.condition, conversationTimeColumn, "message_time", 1),
			arg:       ca.arg,
			args:      ca.args,
		}
	}

	convTimeCond, convTimeArgs = concatConditionsAndArgs(timeConds)
	msgTimeCond, msgTimeArgs = concatConditionsAndArgs(msgTimeConds)
	if len(otherConds) > 0 {
		nonTimeCond, nonTimeArgs = concatConditionsAndArgs(otherConds)
	}
	return
}

func conversationStatsPopulationAggregate(populationQuery string, groupByKeys, groupByQueries []string, timeColumn string) string {
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = "GROUP BY " + concatKeys(groupByKeys)
	}
	groupProjection := strings.ReplaceAll(concatQueries(groupByQueries), timeColumn, "conversation_time")
	return fmt.Sprintf(`WITH eligible_population AS (%s)
 SELECT %s
 COUNT(DISTINCT conversation_id) AS convo_count,
 COUNT(DISTINCT agent_user_id) AS user_count,
 SUM(aht_sec) AS aht_sec
 FROM eligible_population
 %s`, populationQuery, groupProjection, optionalGroupBy)
}

func (a AnalyticsServiceImpl) readConversationStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveConversationStatsRequest,
	users []*userpb.User,
	includeBotConversations bool,
	allowMatchingStaleMetadataValues bool,
) (*analyticspb.RetrieveConversationStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	population, err := buildConversationPopulationQuery(ctx, conversationPopulationInputs{
		timeRange:                        req.FilterByTimeRange,
		attribute:                        req.FilterByAttribute,
		timeRangeField:                   req.ConversationTimeRangeField,
		includeBotConversations:          includeBotConversations,
		allowMatchingStaleMetadataValues: allowMatchingStaleMetadataValues,
		useExternalUserTable:             a.enableExtTableForUserFilter,
	})
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{messageTable}
	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(
		&req.Frequency,
		req.GroupByAttributeTypes,
		req.Metadata,
		targetTables,
		WithTimeRangeType(req.ConversationTimeRangeField),
	)
	if err != nil {
		return nil, err
	}
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, fmt.Errorf("group_by keys %v are not supported", v)
	}

	chQuery := conversationStatsPopulationAggregate(population.query, groupByKeys, groupByQueries[messageTable], getConversationTimeRangeColumn(getConversationQueryOptions(WithTimeRangeType(req.ConversationTimeRangeField))))
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	ctx = attachExtTablesToContext(ctx, population.externalTables...)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, population.args...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	chRows := []conversationStatsRow{}
	for rows.Next() {
		row := conversationStatsRow{}
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
		vals = append(vals, &row.convoCount, &row.userCount, &row.ahtSec)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[conversationStats] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveConversationStats in seconds %v", time.Since(startT).Seconds())
	return a.convertCHResponseToConversationStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToConversationStatsResponse(
	ctx context.Context,
	customerID, profileID string,
	rows []conversationStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveConversationStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveConversationStatsResponse{}, nil
	}
	groupByAttributeToConversationStatsResults := make(map[groupByAttribute]*analyticspb.ConversationStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		conversationStats := analyticspb.ConversationStats{
			TimeRange:              &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalConversationCount: int32(row.convoCount),
			AverageHandleTime:      shared.ConvertSecondToAPIDuration(int64(safeDivideFloat(row.ahtSec, int64(row.convoCount)))),
			AverageConversationCountPerFrequencyPerUser: safeDivideFloat(int32(row.convoCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToConversationStatsResults[groupBy]; !exists {
			groupByAttributeToConversationStatsResults[groupBy] = &analyticspb.ConversationStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToConversationStatsResults[groupBy].ConversationStats = append(groupByAttributeToConversationStatsResults[groupBy].ConversationStats, &conversationStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToConversationStatsResults {
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

	resp := &analyticspb.RetrieveConversationStatsResponse{}
	totalAhtSec := int64(0)
	totalUserFrequencyCount := int32(0)
	frequencyKeyMap := make(map[int64]struct{})
	for k, v := range groupByAttributeToConversationStatsResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		ahtSec := int64(0)
		for _, conversationStats := range v.ConversationStats {
			v.TotalConversationCount += conversationStats.TotalConversationCount
			ahtSec += (int64(conversationStats.TotalConversationCount) * conversationStats.AverageHandleTime.Seconds)
			frequencyKeyMap[conversationStats.TimeRange.StartTimestamp.Seconds] = struct{}{}
		}
		totalAhtSec += ahtSec
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
		v.AverageHandleTime = shared.ConvertSecondToAPIDuration(int64(safeDivideFloat(ahtSec, int64(v.TotalConversationCount))))
		v.AverageConversationCountPerFrequency = safeDivideFloat(v.TotalConversationCount, int32(len(v.ConversationStats)))
		v.AverageConversationCountPerFrequencyPerUser = safeDivideFloat(v.TotalConversationCount, groupByAttributeToUserCount[k])

		// Sort result based on timestamp.
		sort.Slice(v.ConversationStats, func(i, j int) bool {
			return v.ConversationStats[i].TimeRange.StartTimestamp.AsTime().Before(v.ConversationStats[j].TimeRange.StartTimestamp.AsTime())
		})

		resp.TotalConversationCount += v.TotalConversationCount
		resp.ConversationStatsResults = append(resp.ConversationStatsResults, v)
	}
	resp.ConversationStatsResults = sortResults(resp.ConversationStatsResults)
	resp.AverageHandleTime = shared.ConvertSecondToAPIDuration(int64(safeDivideFloat(totalAhtSec, int64(resp.TotalConversationCount))))
	resp.AverageConversationCountPerFrequency = safeDivideFloat(resp.TotalConversationCount, int32(len(frequencyKeyMap)))
	resp.AverageConversationCountPerFrequencyPerUser = safeDivideFloat(resp.TotalConversationCount, totalUserFrequencyCount)
	return resp, nil
}

type conversationStatsRow struct {
	clickhouseKeyTimeRow
	convoCount uint64
	userCount  uint64
	ahtSec     int64
}
