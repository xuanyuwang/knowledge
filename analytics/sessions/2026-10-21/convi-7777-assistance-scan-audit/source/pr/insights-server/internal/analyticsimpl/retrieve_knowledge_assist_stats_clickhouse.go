package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type knowledgeAssistStatsRow struct {
	clickhouseKeyTimeRow
	genaiAnswersViewedCount      uint64
	genaiAnswersUsedCount        uint64
	knowledgeMomentDetectedCount uint64
	userCount                    uint64
}

func knowledgeAssistStatsClickhouseQuery(conditionsAndArgs map[ClickhouseTable][]conditionAndArg, groupByKeys []string, groupByQueries map[ClickhouseTable][]string, momentConditionsAndArgsList [][]conditionAndArg, excludedMomentConditionsAndArgs [][]conditionAndArg) (string, []any) {
	allArgs := []any{}

	// Check if we need metadata filtering
	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	// Generate metadata filters using refactored functions
	aliasDict := map[string]string{
		"message":          "message_d",
		"actionAnnotation": "action_annotation_d",
		"momentAnnotation": "moment_annotation_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false, true)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	messageRowsQuery, messageArgs := buildKnowledgeAssistUsageRowsQuery(messageTable, conditionsAndArgs[messageTable], metadataFilters, hasMetadataFiltering)
	actionRowsQuery, actionArgs := buildKnowledgeAssistUsageRowsQuery(actionAnnotationTable, conditionsAndArgs[actionAnnotationTable], metadataFilters, hasMetadataFiltering)
	usedMomentRowsQuery, usedMomentArgs := buildKnowledgeAssistUsageRowsQuery(momentAnnotationTable, conditionsAndArgs[momentAnnotationTable], metadataFilters, hasMetadataFiltering)

	maGroupByQueries, aaGroupByQueries, mGroupByQueries := concatQueries(groupByQueries[momentAnnotationTable]), concatQueries(groupByQueries[actionAnnotationTable]), concatQueries(groupByQueries[messageTable])

	messagesKeys := ""
	leftJoinActionAnnotations := "LEFT JOIN action_annotations ON "
	leftJoinMomentAnnotations := "LEFT JOIN moment_annotations ON "
	if slices.Contains(groupByKeys, truncatedTimeColumn) {
		messagesKeys += "messages.truncated_time,"
		leftJoinActionAnnotations += "messages.truncated_time = action_annotations.truncated_time"
		leftJoinMomentAnnotations += "messages.truncated_time = moment_annotations.truncated_time"
	}
	if slices.Contains(groupByKeys, agentUserIDColumn) {
		if slices.Contains(groupByKeys, truncatedTimeColumn) {
			leftJoinActionAnnotations += " AND "
			leftJoinMomentAnnotations += " AND "
		}
		messagesKeys += "messages.agent_user_id,"
		leftJoinActionAnnotations += "messages.agent_user_id = action_annotations.agent_user_id"
		leftJoinMomentAnnotations += "messages.agent_user_id = moment_annotations.agent_user_id"
	}

	allArgs = append(allArgs, messageArgs...)
	allArgs = append(allArgs, actionArgs...)
	allArgs = append(allArgs, usedMomentArgs...)

	if hasMetadataFiltering {
		return fmt.Sprintf(`
			WITH
			%s
			%s
			messages AS (
				SELECT
					%s
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM
					(%s) AS message_d
				GROUP BY %s
			),
			action_annotations AS (
				SELECT
					%s
					COUNT(DISTINCT CASE WHEN action_type = 22 THEN action_annotation_id END) AS question_extraction_count,
					COUNT(DISTINCT CASE WHEN action_type = 24 THEN knowledge_payload_knowledge_session_id END) AS retrieval_article_count
				FROM
					(%s) AS action_annotation_d
				GROUP BY %s
			),
			moment_annotations AS (
				SELECT
					%s
					COUNT(DISTINCT knowledge_payload_knowledge_session_id) AS answer_used_count
				FROM
					(%s) AS moment_annotation_d
				GROUP BY %s
			)
			SELECT
				%s
				action_annotations.retrieval_article_count AS gen_ai_answer_viewed_count,
				moment_annotations.answer_used_count AS gen_ai_answer_used_count,
				action_annotations.question_extraction_count AS knowledge_moments_detected_count,
				messages.user_count
			FROM
				messages
				%s
				%s
		`, metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
				mGroupByQueries, messageRowsQuery, concatKeys(groupByKeys),
				aaGroupByQueries, actionRowsQuery, concatKeys(groupByKeys),
				maGroupByQueries, usedMomentRowsQuery, concatKeys(groupByKeys),
				messagesKeys,
				leftJoinActionAnnotations,
				leftJoinMomentAnnotations),
			allArgs
	} else {
		return fmt.Sprintf(`
			WITH messages AS (
				SELECT
					%s
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM
					(%s) AS message_d
				GROUP BY %s
			),
			action_annotations AS (
				SELECT
					%s
					COUNT(DISTINCT CASE WHEN action_type = 22 THEN action_annotation_id END) AS question_extraction_count,
					COUNT(DISTINCT CASE WHEN action_type = 24 THEN knowledge_payload_knowledge_session_id END) AS retrieval_article_count
				FROM
					(%s) AS action_annotation_d
				GROUP BY %s
			),
			moment_annotations AS (
				SELECT
					%s
					COUNT(DISTINCT knowledge_payload_knowledge_session_id) AS answer_used_count
				FROM
					(%s) AS moment_annotation_d
				GROUP BY %s
			)
			SELECT
				%s
				action_annotations.retrieval_article_count AS gen_ai_answer_viewed_count,
				moment_annotations.answer_used_count AS gen_ai_answer_used_count,
				action_annotations.question_extraction_count AS knowledge_moments_detected_count,
				messages.user_count
			FROM
				messages
				%s
				%s
		`, mGroupByQueries, messageRowsQuery, concatKeys(groupByKeys),
				aaGroupByQueries, actionRowsQuery, concatKeys(groupByKeys),
				maGroupByQueries, usedMomentRowsQuery, concatKeys(groupByKeys),
				messagesKeys,
				leftJoinActionAnnotations,
				leftJoinMomentAnnotations),
			allArgs
	}
}

func (a AnalyticsServiceImpl) readKnowledgeAssistStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveKnowledgeAssistStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveKnowledgeAssistStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	targetTables := []ClickhouseTable{
		messageTable,
		momentAnnotationTable,
		actionAnnotationTable,
	}
	conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
	if err != nil {
		return nil, err
	}

	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, targetTables)
	if err != nil {
		return nil, err
	}
	// For retrieveKnowledgeAssistStats API, we only support grouping by
	// - 1) agent/group
	// - 2) truncated time
	// - 3) both agent/group and truncated time
	if len(groupByKeys) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "not supporting zero group-by attributes.")
	}
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "group_by keys %v are not supported", v)
	}

	chQuery, chArgs := knowledgeAssistStatsClickhouseQuery(conditionsAndArgs, groupByKeys, groupByQueries, momentConditionsAndArgsList, excludedMomentConditionsAndArgs)
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

	chRows := []knowledgeAssistStatsRow{}
	for rows.Next() {
		row := knowledgeAssistStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.genaiAnswersViewedCount, &row.genaiAnswersUsedCount, &row.knowledgeMomentDetectedCount, &row.userCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[knowledgeAssistStats] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveKnowledgeAssistStats in seconds %v", time.Since(startT).Seconds())
	resp, err := a.convertCHResponseToKnowledgeAssistStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
	if err != nil {
		return nil, err
	}

	resp.KnowledgeAssistStatsResults = sortResults(resp.KnowledgeAssistStatsResults)

	return resp, nil
}

func (a AnalyticsServiceImpl) convertCHResponseToKnowledgeAssistStatsResponse(
	ctx context.Context,
	customerID, profileID string,
	rows []knowledgeAssistStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveKnowledgeAssistStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveKnowledgeAssistStatsResponse{}, nil
	}
	groupByAttributeToKnowledgeAssistStatsResults := make(map[groupByAttribute]*analyticspb.KnowledgeAssistStatsResult)
	for _, row := range rows {
		knowledgeAssistStats := analyticspb.KnowledgeAssistStats{
			TimeRange:                                      &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			GenaiAnswersViewedCount:                        int32(row.genaiAnswersViewedCount),
			GenaiAnswersUsedCount:                          int32(row.genaiAnswersUsedCount),
			KnowledgeMomentDetectedCount:                   int32(row.knowledgeMomentDetectedCount),
			AverageGenaiAnswerUsedCountPerFrequencyPerUser: safeDivideFloat(int32(row.genaiAnswersUsedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToKnowledgeAssistStatsResults[groupBy]; !exists {
			groupByAttributeToKnowledgeAssistStatsResults[groupBy] = &analyticspb.KnowledgeAssistStatsResult{}
		}
		groupByAttributeToKnowledgeAssistStatsResults[groupBy].KnowledgeAssistStats = append(groupByAttributeToKnowledgeAssistStatsResults[groupBy].KnowledgeAssistStats, &knowledgeAssistStats)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToKnowledgeAssistStatsResults {
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
	resp := &analyticspb.RetrieveKnowledgeAssistStatsResponse{}
	for k, v := range groupByAttributeToKnowledgeAssistStatsResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, knowledgeAssistStat := range v.KnowledgeAssistStats {
			v.GenaiAnswersViewedCount += knowledgeAssistStat.GenaiAnswersViewedCount
			v.GenaiAnswersUsedCount += knowledgeAssistStat.GenaiAnswersUsedCount
			v.KnowledgeMomentDetectedCount += knowledgeAssistStat.KnowledgeMomentDetectedCount
		}

		totalUserFrequenyCount += int32(len(v.KnowledgeAssistStats))
		v.AverageGenaiAnswerUsedCountPerFrequencyPerUser = safeDivideFloat(v.GenaiAnswersUsedCount, int32(len(v.KnowledgeAssistStats)))

		// Sort result based on timestamp.
		sort.Slice(v.KnowledgeAssistStats, func(i, j int) bool {
			return v.KnowledgeAssistStats[i].TimeRange.StartTimestamp.AsTime().Before(v.KnowledgeAssistStats[j].TimeRange.StartTimestamp.AsTime())
		})
		resp.KnowledgeAssistStatsResults = append(resp.KnowledgeAssistStatsResults, v)
		resp.GenaiAnswersViewedCount += v.GenaiAnswersViewedCount
		resp.GenaiAnswersUsedCount += v.GenaiAnswersUsedCount
		resp.KnowledgeMomentDetectedCount += v.KnowledgeMomentDetectedCount
	}

	resp.AverageGenaiAnswerUsedCountPerFrequencyPerUser = safeDivideFloat(resp.GenaiAnswersUsedCount, int32(len(resp.KnowledgeAssistStatsResults)))

	return resp, nil
}
