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

type scorecardStatsRow struct {
	clickhouseKeyTimeRow
	scorecardCompleted uint64
	userCount          uint64
}

func scorecardStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
) (string, []any) {
	scCondition, scArgs := concatConditionsAndArgs(conditionsAndArgs[scorecardTable])
	scGroupByQueries := concatQueries(groupByQueries[scorecardTable])

	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	return strings.ReplaceAll(strings.ReplaceAll(
			fmt.Sprintf(`
				SELECT
					%s
					COUNT(DISTINCT scorecard_id) AS scorecard_completed,
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM
					scorecard_d
				WHERE
					agent_user_id <> ''
					AND %s -- Additional filters
				-- GROUP BY is optional.
				%s
				`,
				scGroupByQueries, scCondition, optionalGroupBy),
			// We should be using `scorecard_submit_time` (instead of `conversation_start_time`).
			scorecardTimeColumn, scorecardSubmitTimeColumn),
			// We should be using `submitter_user_id` (instead of `agent_user_id`).
			agentUserIDColumn, submitterUserIDColumn),
		scArgs
}

func (a AnalyticsServiceImpl) readScorecardStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveScorecardStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveScorecardStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		scorecardTable,
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
	chQuery, chArgs := scorecardStatsClickhouseQuery(conditionAndArgs, groupByKeys, groupByQueries)
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

	chRows := []scorecardStatsRow{}
	for rows.Next() {
		row := scorecardStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.scorecardCompleted, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[scorecardStats] scanning clickhouse row: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "[scorecardStats] ClickHouse (events) query latency in seconds: %v", time.Since(startT).Seconds())

	return a.convertCHResponseToScorecardStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users, shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT))
}

func (a AnalyticsServiceImpl) convertCHResponseToScorecardStatsResponse(
	ctx context.Context,
	customerID string,
	profileID string,
	rows []scorecardStatsRow,
	users []*userpb.User,
	isGroupingByAgent bool,
) (*analyticspb.RetrieveScorecardStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveScorecardStatsResponse{}, nil
	}
	groupByAttributeToScorecardStatsResult := make(map[groupByAttribute]*analyticspb.ScorecardStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		scorecardStats := analyticspb.ScorecardStats{
			TimeRange:                        &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			ScorecardCompleted:               int32(row.scorecardCompleted),
			AverageScorecardCompletedPerUser: safeDivideFloat(int32(row.scorecardCompleted), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToScorecardStatsResult[groupBy]; !exists {
			groupByAttributeToScorecardStatsResult[groupBy] = &analyticspb.ScorecardStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToScorecardStatsResult[groupBy].ScorecardStats = append(groupByAttributeToScorecardStatsResult[groupBy].ScorecardStats, &scorecardStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToScorecardStatsResult {
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

	totalUserCount := int32(0)
	resp := &analyticspb.RetrieveScorecardStatsResponse{}
	for k, v := range groupByAttributeToScorecardStatsResult {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, scorecardStat := range v.ScorecardStats {
			v.ScorecardCompleted += scorecardStat.ScorecardCompleted
			v.AverageScorecardCompletedPerUser += scorecardStat.AverageScorecardCompletedPerUser
		}
		// Sort result based on timestamp.
		sort.Slice(v.ScorecardStats, func(i, j int) bool {
			return v.ScorecardStats[i].TimeRange.StartTimestamp.AsTime().Before(v.ScorecardStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserCount += groupByAttributeToUserCount[k]
		resp.ScorecardCompleted += v.ScorecardCompleted
		resp.ScorecardStatsResults = append(resp.ScorecardStatsResults, v)
	}
	resp.ScorecardStatsResults = sortResults(resp.ScorecardStatsResults)
	if isGroupingByAgent {
		totalUserCount = int32(len(resp.ScorecardStatsResults))
	}
	resp.AverageScorecardCompletedPerUser = safeDivideFloat(resp.ScorecardCompleted, totalUserCount)
	return resp, nil
}
