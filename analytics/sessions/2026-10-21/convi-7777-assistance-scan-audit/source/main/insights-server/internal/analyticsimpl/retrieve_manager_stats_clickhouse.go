package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

func sessionLogEventCountClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
) (string, []any) {
	eventGroupByQueries := concatQueries(groupByQueries[eventAnalyticsEventTable])
	conditions, args := concatConditionsAndArgs(conditionsAndArgs[eventAnalyticsEventTable])
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	return fmt.Sprintf(`
		SELECT
			%s
			COUNT(DISTINCT agent_user_id) AS manager_count,
			COUNT(DISTINCT event_id) AS director_pages_visit_count,
			COUNT(DISTINCT CASE
				WHEN JSONExtractString(toJSONString(event_payload), 'sessionLog', 'directorPageVisitPayload', 'directorPage') = 'LIVE_CONVERSATION'
				THEN event_id
				END
			) AS director_live_conversation_page_visit_count,
			COUNT(DISTINCT CASE
				WHEN JSONExtractString(toJSONString(event_payload), 'sessionLog', 'directorPageVisitPayload', 'directorPage') = 'CLOSED_CONVERSATION'
				THEN event_id
				END
			) AS director_closed_conversation_page_visit_count
		FROM
			analytics_event_d
		WHERE
			agent_user_id <> ''
			AND event_name = 'Director.SessionLog.Visit.Page'
			AND %s	-- Additional filters
		-- GROUP BY is optional.
		%s
	`, eventGroupByQueries, conditions, optionalGroupBy),
		args
}

func (a AnalyticsServiceImpl) queryManagerStatsFromSessionLogEvents(
	ctx context.Context,
	req *analyticspb.RetrieveManagerStatsRequest,
	customerID, profileID string,
) ([]managerStatsRow, error) {
	targetTables := []ClickhouseTable{
		eventAnalyticsEventTable,
	}
	// This API should only take user filters.
	attr := &analyticspb.Attribute{}
	attr.Users = req.FilterByAttribute.Users
	conditionsAndArgs, _, _, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, attr, targetTables, a.enableExtTableForUserFilter)
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

	var chEventQuery string
	var chEventArgs []any
	chEventQuery, chEventArgs = sessionLogEventCountClickhouseQuery(conditionsAndArgs, groupByKeys, groupByQueries)
	a.logger.Infof(ctx, "clickhouse query for managerStats (events): %v", chEventQuery)

	conn, err := a.getEventsClickhouseConn(ctx, customerID, profileID)
	if err != nil {
		return nil, err
	}

	ctx = attachExtTablesToContext(ctx, usersExtTable)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chEventQuery, chEventArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	chEventRows := []managerStatsRow{}
	for rows.Next() {
		row := managerStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.managerCount, &row.directorPagesVisitCount, &row.directorLiveConversationPageVisitCount, &row.directorClosedConversationPageVisitCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[managerStats] scanning clickhouse (events) rows: %w", err)
		}
		chEventRows = append(chEventRows, row)
	}
	a.logger.Infof(ctx, "[managerStats] ClickHouse (events) query latency in seconds %v", time.Since(startT).Seconds())

	return chEventRows, nil
}

func (a AnalyticsServiceImpl) readManagerStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveManagerStatsRequest,
	profileID string,
	users []*userpb.User,
) (*analyticspb.RetrieveManagerStatsResponse, error) {
	parent, err := shared.ParseCustomer(req.Parent)
	if err != nil {
		return nil, err
	}
	dbProvider, err := a.appsDB.GetProvider(parent.CustomerID, profileID)
	if err != nil {
		return nil, err
	}
	db := dbProvider.Get(ctx)
	chRows, err := a.queryManagerStatsFromSessionLogEvents(ctx, req, parent.CustomerID, profileID)
	if err != nil {
		return nil, err
	}
	return a.convertCHResponseToManagerStatsResponse(parent.CustomerID, profileID, db, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToManagerStatsResponse(
	customerID, profileID string,
	db *gorm.DB,
	chRows []managerStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveManagerStatsResponse, error) {
	if len(chRows) == 0 {
		return &analyticspb.RetrieveManagerStatsResponse{}, nil
	}

	groupByAttributeToManagerStatsResults := make(map[groupByAttribute]*analyticspb.ManagerStatsResult)
	for _, row := range chRows {
		managerStats := analyticspb.ManagerStats{
			TimeRange: &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
				TotalDirectorPagesVisitCount:                            int32(row.directorPagesVisitCount),
				TotalDirectorLiveConversationPagesVisitCount:            int32(row.directorLiveConversationPageVisitCount),
				TotalDirectorClosedConversationPagesVisitCount:          int32(row.directorClosedConversationPageVisitCount),
				AverageDirectorPagesVisitCountPerUser:                   safeDivideFloat(int32(row.directorPagesVisitCount), int32(row.managerCount)),
				AverageDirectorLiveConversationPagesVisitCountPerUser:   safeDivideFloat(int32(row.directorLiveConversationPageVisitCount), int32(row.managerCount)),
				AverageDirectorClosedConversationPagesVisitCountPerUser: safeDivideFloat(int32(row.directorClosedConversationPageVisitCount), int32(row.managerCount)),
			},
		}
		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToManagerStatsResults[groupBy]; !exists {
			groupByAttributeToManagerStatsResults[groupBy] = &analyticspb.ManagerStatsResult{}
		}
		groupByAttributeToManagerStatsResults[groupBy].ManagerStats = append(groupByAttributeToManagerStatsResults[groupBy].ManagerStats, &managerStats)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToManagerStatsResults {
		attr := k.GetAttribute(customerID, profileID)
		if len(attr.Users) > 0 {
			if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
				attr.Users[0].FullName = user.FullName
				attr.Users[0].Username = user.Username
			} else {
				// Remove the entry from groupByAttributeToManagerStatsResults since we can't find the user
				delete(groupByAttributeToManagerStatsResults, k)
				continue
			}
		}
		groupByAttributeToAttribute[k] = attr
		attributes = append(attributes, attr)
	}

	ignoredAttributes, err := attributesToIgnore(db, attributes)
	if err != nil {
		return nil, err
	}

	resp := &analyticspb.RetrieveManagerStatsResponse{}
	resp.AggregatedManagerStats = &analyticspb.AggregatedManagerStats{}
	for k, v := range groupByAttributeToManagerStatsResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		if len(attr.Users) > 0 {
			if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
				// We need to skip dev users from listUsers results, because we didn't store
				// dev user info in clickhouse events table.
				if user.Type == enumspb.UserType_DEV_USER {
					continue
				}
			}
		}
		v.Attribute = attr
		v.AggregatedManagerStats = &analyticspb.AggregatedManagerStats{}
		for _, managerStats := range v.ManagerStats {
			v.AggregatedManagerStats.TotalDirectorPagesVisitCount += managerStats.AggregatedManagerStats.TotalDirectorPagesVisitCount
			v.AggregatedManagerStats.TotalDirectorLiveConversationPagesVisitCount += managerStats.AggregatedManagerStats.TotalDirectorLiveConversationPagesVisitCount
			v.AggregatedManagerStats.TotalDirectorClosedConversationPagesVisitCount += managerStats.AggregatedManagerStats.TotalDirectorClosedConversationPagesVisitCount
			v.AggregatedManagerStats.AverageDirectorPagesVisitCountPerUser += managerStats.AggregatedManagerStats.AverageDirectorPagesVisitCountPerUser
			v.AggregatedManagerStats.AverageDirectorLiveConversationPagesVisitCountPerUser += managerStats.AggregatedManagerStats.AverageDirectorLiveConversationPagesVisitCountPerUser
			v.AggregatedManagerStats.AverageDirectorClosedConversationPagesVisitCountPerUser += managerStats.AggregatedManagerStats.AverageDirectorClosedConversationPagesVisitCountPerUser
		}

		// Sort result based on timestamp.
		sort.Slice(v.ManagerStats, func(i, j int) bool {
			return v.ManagerStats[i].TimeRange.StartTimestamp.AsTime().Before(v.ManagerStats[j].TimeRange.StartTimestamp.AsTime())
		})

		resp.AggregatedManagerStats.TotalDirectorPagesVisitCount += v.AggregatedManagerStats.TotalDirectorPagesVisitCount
		resp.AggregatedManagerStats.TotalDirectorLiveConversationPagesVisitCount += v.AggregatedManagerStats.TotalDirectorLiveConversationPagesVisitCount
		resp.AggregatedManagerStats.TotalDirectorClosedConversationPagesVisitCount += v.AggregatedManagerStats.TotalDirectorClosedConversationPagesVisitCount
		resp.ManagerStatsResults = append(resp.ManagerStatsResults, v)
	}
	resp.ManagerStatsResults = sortResults(resp.ManagerStatsResults)
	resp.AggregatedManagerStats.AverageDirectorPagesVisitCountPerUser = safeDivideFloat(resp.AggregatedManagerStats.TotalDirectorPagesVisitCount, int32(len(resp.ManagerStatsResults)))
	resp.AggregatedManagerStats.AverageDirectorLiveConversationPagesVisitCountPerUser = safeDivideFloat(resp.AggregatedManagerStats.TotalDirectorLiveConversationPagesVisitCount, int32(len(resp.ManagerStatsResults)))
	resp.AggregatedManagerStats.AverageDirectorClosedConversationPagesVisitCountPerUser = safeDivideFloat(resp.AggregatedManagerStats.TotalDirectorClosedConversationPagesVisitCount, int32(len(resp.ManagerStatsResults)))
	println(resp.String())
	return resp, nil
}

type managerStatsRow struct {
	clickhouseKeyTimeRow
	managerCount                             uint64
	directorPagesVisitCount                  uint64
	directorLiveConversationPageVisitCount   uint64
	directorClosedConversationPageVisitCount uint64
}
