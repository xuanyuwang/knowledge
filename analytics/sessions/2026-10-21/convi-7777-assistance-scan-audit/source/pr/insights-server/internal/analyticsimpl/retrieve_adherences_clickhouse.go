package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

func adherencesClickhouseQuery(conditionsAndArgs map[ClickhouseTable][]conditionAndArg, groupByKeys []string, groupByQueries map[ClickhouseTable][]string, includeBotConversations bool) (string, []any) {
	maGroupByQueries := concatQueries(groupByQueries[momentAnnotationTable])
	maCondition, maArgs := concatConditionsAndArgs(conditionsAndArgs[momentAnnotationTable])
	finalJoinKeys, finalGroupByQueries := concatKeys(groupByKeys), concatQueries(groupByKeys)
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}

	crestaBotStr := ""
	if includeBotConversations {
		crestaBotStr = crestaBotSourceStr
	}

	return fmt.Sprintf(`
		With sdx AS (
			SELECT
				%s
				COUNT(DISTINCT adherence_moment_annotation_id) AS sdx_count,
				COUNT(DISTINCT conversation_id) AS convo_count
			FROM
				moment_annotation_d
			WHERE 
				adherence_type IN (2, 3, 5)				-- 2: ADHERENCE_TYPE_DID_DO_X, 3: ADHERENCE_TYPE_DID_NOT_DO_X, 5: ADHERENCE_TYPE_NO_OPPORTUNITY_TO_DO_X
				AND adherence_moment_adherence_type = 1	-- 1: ADHERENCE_TYPE_SHOULD_DO_X
				AND adherence_moment_annotation_id != ''
				AND conversation_source in (0, 8%s)
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND %s	-- Additional filters
			%s
		),
		ddx AS (
			SELECT
				%s
				COUNT(DISTINCT moment_annotation_id) AS ddx_count
			FROM
				moment_annotation_d
			WHERE
				adherence_type = 2	-- 2: ADHERENCE_TYPE_DID_DO_X
				AND conversation_source in (0, 8%s)
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND %s	-- Additional filters
			%s
		)
		SELECT
			%s
			SUM(sdx.convo_count) AS convo_count,
			SUM(sdx.sdx_count) AS sdx_count,
			SUM(ddx.ddx_count) AS ddx_count
		FROM
			sdx LEFT JOIN ddx USING (%s)
		%s
	`, maGroupByQueries, crestaBotStr, maCondition, optionalGroupBy,
			maGroupByQueries, crestaBotStr, maCondition, optionalGroupBy,
			finalGroupByQueries, finalJoinKeys, optionalGroupBy),
		append(maArgs, maArgs...)
}

func (a AnalyticsServiceImpl) readAdherencesFromClickhouse(ctx context.Context, req *analyticspb.RetrieveAdherencesRequest, users []*userpb.User) (*analyticspb.RetrieveAdherencesResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		momentAnnotationTable,
	}
	conditionsAndArgs, _, _, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
	if err != nil {
		return nil, err
	}
	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, targetTables)
	if err != nil {
		return nil, err
	}
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, policyIDColumn, truncatedTimeColumn, momentTemplateIDColumn}); len(v) > 0 {
		return nil, fmt.Errorf("group_by keys %v are not supported", v)
	}

	// Whether to include CONVERSATION_SOURCE_BOT as part of stats (default as False).
	includeBotConversations, err := shared.IncludeConversationSourceBot(ctx, a.configClient, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	// If includeBotConversations is true, we will include CONVERSATION_SOURCE_BOT as part of stats.
	// If req.FilterByAttribute.IncludeCrestaBot is true, we will include CONVERSATION_SOURCE_BOT as part of stats.
	// If both are true, we will include CONVERSATION_SOURCE_BOT as part of stats.
	chQuery, chArgs := adherencesClickhouseQuery(conditionsAndArgs, groupByKeys, groupByQueries, includeBotConversations || req.FilterByAttribute.IncludeCrestaBot)
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

	chRows := []adherencesRow{}
	for rows.Next() {
		row := adherencesRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case policyIDColumn:
				vals = append(vals, &row.policyID)
			case momentTemplateIDColumn:
				vals = append(vals, &row.momentTemplateID)
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.convoCount, &row.sdxCount, &row.ddxCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[adherence] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveAdherences in seconds %v", time.Since(startT).Seconds())
	return a.convertCHResponseToAdherencesResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
}

func (a AnalyticsServiceImpl) convertCHResponseToAdherencesResponse(ctx context.Context, customerID string, profileID string, rows []adherencesRow, users []*userpb.User) (*analyticspb.RetrieveAdherencesResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveAdherencesResponse{}, nil
	}
	groupByAttributeToAdherenceResults := make(map[groupByAttribute]*analyticspb.AdherenceResult)
	for _, row := range rows {
		adherence := analyticspb.Adherence{
			TimeRange:     &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalSdxCount: int32(row.sdxCount),
			Adherence:     safeDivideFloat(int(row.ddxCount), int(row.sdxCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToAdherenceResults[groupBy]; !exists {
			groupByAttributeToAdherenceResults[groupBy] = &analyticspb.AdherenceResult{}
		}
		groupByAttributeToAdherenceResults[groupBy].Adherences = append(groupByAttributeToAdherenceResults[groupBy].Adherences, &adherence)
	}

	userNameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		userNameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToAdherenceResults {
		attr := k.GetAttribute(customerID, profileID)
		if len(attr.Users) > 0 {
			if user, exists := userNameToUserMap[attr.Users[0].Name]; exists {
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

	resp := &analyticspb.RetrieveAdherencesResponse{}
	totalDdxCount := float32(0)
	for k, v := range groupByAttributeToAdherenceResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr

		ddxCount := float32(0)
		for _, adherence := range v.Adherences {
			v.TotalSdxCount += adherence.TotalSdxCount
			ddxCount += float32(adherence.TotalSdxCount) * adherence.Adherence
		}
		totalDdxCount += ddxCount
		v.AverageAdherence = safeDivideFloat(int32(ddxCount), v.TotalSdxCount)

		// Sort result based on timestamp.
		sort.Slice(v.Adherences, func(i, j int) bool {
			return v.Adherences[i].TimeRange.StartTimestamp.AsTime().Before(v.Adherences[j].TimeRange.StartTimestamp.AsTime())
		})
		resp.AdherenceResults = append(resp.AdherenceResults, v)
		resp.TotalSdxCount += v.TotalSdxCount
	}
	resp.AdherenceResults = sortResults(resp.AdherenceResults)
	resp.AverageAdherence = safeDivideFloat(int32(totalDdxCount), resp.TotalSdxCount)

	if err := a.populateMomentFields(ctx, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

type adherencesRow struct {
	clickhouseKeyTimeRow
	convoCount uint64
	sdxCount   uint64
	ddxCount   uint64
}

func (a AnalyticsServiceImpl) populateMomentFields(ctx context.Context, resp *analyticspb.RetrieveAdherencesResponse) error {
	momentIDs := []string{}
	for _, ar := range resp.AdherenceResults {
		if attr := ar.Attribute; attr != nil && len(attr.Moments) > 0 {
			name, err := momentpb.ParseMomentName(attr.Moments[0].Name)
			if err != nil {
				return err
			}
			momentIDs = append(momentIDs, name.MomentID)
		}
	}
	moments := postgres.ListMoments(a.appsDB.DB(ctx), momentIDs)
	nameToMoment := map[string]*momentpb.Moment{}
	for _, m := range moments {
		name := momentpb.MomentName{
			CustomerID: m.CustomerID,
			ProfileID:  m.ProfileID,
			MomentID:   m.MomentTemplateID,
		}.String()
		nameToMoment[name] = &momentpb.Moment{
			Name:        name,
			DisplayName: m.DisplayName,
			Taxonomy:    m.Taxonomy.String,
		}
	}
	for _, ar := range resp.AdherenceResults {
		if attr := ar.Attribute; attr != nil && len(attr.Moments) > 0 {
			if m, exists := nameToMoment[attr.Moments[0].Name]; exists {
				attr.Moments[0] = m
			}
		}
	}
	return nil
}
