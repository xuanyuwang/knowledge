package analyticsimpl

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/ext"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	convpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	"github.com/cresta/go-servers/shared/db/condition"
	"github.com/cresta/go-servers/shared/db/converter"
)

const (
	ScorecardScoreQueryBatchSize = 1000
)

func qaConversationsClickhouseQuery(
	commonConditionsAndArgs []conditionAndArg,
	scorecardConditionsAndArgs []conditionAndArg,
	scoreConditionsAndArgs []conditionAndArg,
	conversationConditionsAndArgs []conditionAndArg,
	conversationScanBounds []conditionAndArg,
	momentFilters, excludedMomentFilters []momentAnnotationFilter,
	valueOrMissingFilters []qaMomentValueOrMissingFilter,
	pageSize, offset int,
	table ClickhouseTable,
) (string, []any) {
	allArgs := []any{}

	// Moment filter (optional)
	momentSubQueries, momentJoinQueries := "", ""
	momentSubQueryList := []string{}
	momentJoinQueryList := []string{}
	for i, momentFilter := range momentFilters {
		subQuery, joinQuery, args := buildQAMomentFilterQuery(
			momentFilter,
			i,
			"INNER JOIN",
			"qa_score_info.conversation_id",
			conversationIDColumn,
		)
		momentSubQueryList = append(momentSubQueryList, subQuery)
		momentJoinQueryList = append(momentJoinQueryList, joinQuery)
		allArgs = append(allArgs, args...)
	}

	selectedValueIndices := make([]int, len(valueOrMissingFilters))
	momentIndexOffset := len(momentFilters)
	for filterIdx, filter := range valueOrMissingFilters {
		i := momentIndexOffset + filterIdx
		selectedValueIndices[filterIdx] = i

		resultConversationID := fmt.Sprintf("conversation_id_%d", i)
		subQuery, joinQuery, args := buildQAMomentFilterQuery(
			filter.selectedValueFilter,
			i,
			"LEFT JOIN",
			"qa_score_info.conversation_id",
			resultConversationID,
		)
		momentSubQueryList = append(momentSubQueryList, subQuery)
		momentJoinQueryList = append(momentJoinQueryList, joinQuery)
		allArgs = append(allArgs, args...)
	}
	momentSubQueries = strings.Join(momentSubQueryList, "\n")
	momentJoinQueries = strings.Join(momentJoinQueryList, "\n")

	// Moment excluded filter (optional).
	excludedMomentSubQueryList := []string{}
	excludedMomentJoinQueryList := []string{}
	excludedMomentWhereQueryList := []string{}
	for i, excludedMomentFilter := range excludedMomentFilters {
		conditions, args := concatConditionsAndArgs(excludedMomentFilter.conditions)
		excludedMomentSubQueryList = append(excludedMomentSubQueryList, fmt.Sprintf(`
		moment_annotation_exclude_filter_%d AS (
			SELECT DISTINCT
			    -- Rename the column to avoid ambiguous column issue.
				conversation_id AS conversation_id_exclude_%d
			FROM
				%s
			WHERE
				%s
		),
	`, i, i, excludedMomentFilter.source.tableName(), conditions))
		allArgs = append(allArgs, args...)
		excludedMomentJoinQueryList = append(excludedMomentJoinQueryList, fmt.Sprintf(`
		LEFT JOIN moment_annotation_exclude_filter_%d ON
			qa_score_info.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
	`, i, i, i))
		excludedMomentWhereQueryList = append(excludedMomentWhereQueryList, fmt.Sprintf(`
		AND moment_annotation_exclude_filter_%d.conversation_id_exclude_%d = ''
	`, i, i))
	}

	valueOrMissingWhereQueryList := []string{}
	missingIndexOffset := len(excludedMomentFilters)
	for filterIdx, filter := range valueOrMissingFilters {
		missingIndex := missingIndexOffset + filterIdx
		existenceFilter := filter.existenceFilter

		conditions, args := concatConditionsAndArgs(existenceFilter.conditions)
		excludedMomentSubQueryList = append(excludedMomentSubQueryList, fmt.Sprintf(`
		moment_annotation_exclude_filter_%d AS (
			SELECT DISTINCT
			    -- Rename the column to avoid ambiguous column issue.
				conversation_id AS conversation_id_exclude_%d
			FROM
				%s
			WHERE
				%s
		),
	`, missingIndex, missingIndex, existenceFilter.source.tableName(), conditions))
		allArgs = append(allArgs, args...)
		excludedMomentJoinQueryList = append(excludedMomentJoinQueryList, fmt.Sprintf(`
		LEFT JOIN moment_annotation_exclude_filter_%d ON
			qa_score_info.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
	`, missingIndex, missingIndex, missingIndex))

		selectedValueIndex := selectedValueIndices[filterIdx]
		valueOrMissingWhereQueryList = append(valueOrMissingWhereQueryList, fmt.Sprintf(`
		AND (
			ifNull(moment_annotation_filter_%d.conversation_id_%d, '') != '' OR
			ifNull(moment_annotation_exclude_filter_%d.conversation_id_exclude_%d, '') = ''
		)
	`, selectedValueIndex, selectedValueIndex, missingIndex, missingIndex))
	}
	excludedMomentSubQueries := strings.Join(excludedMomentSubQueryList, "\n")
	excludedMomentJoinQueries := strings.Join(excludedMomentJoinQueryList, "\n")
	excludedMomentWhereQueries := ""
	if len(excludedMomentWhereQueryList) > 0 || len(valueOrMissingWhereQueryList) > 0 {
		whereQueryList := make([]string, 0, len(excludedMomentWhereQueryList)+len(valueOrMissingWhereQueryList))
		whereQueryList = append(whereQueryList, excludedMomentWhereQueryList...)
		whereQueryList = append(whereQueryList, valueOrMissingWhereQueryList...)
		excludedMomentWhereQueries = "WHERE TRUE " + strings.Join(whereQueryList, "\n")
	}

	// Page size and offset
	optionalLimit := ""
	if pageSize > 0 {
		optionalLimit = fmt.Sprintf("LIMIT %d", pageSize)
	}
	optionalOffset := ""
	if pageSize > 0 && offset > 0 {
		optionalOffset = fmt.Sprintf("OFFSET %d", offset)
	}

	qaScoreSelectQuery := `SELECT DISTINCT
			conversation_id,
			conversation_start_time,
			agent_user_id,
			scorecard_template_id,
			scorecard_template_revision,
			scorecard_id,
			score_id,
			criterion_id,
			manually_scored,
			numeric_value,
			max_value,
			percentage_value,
			weight,
			ai_value,
			ai_scored,
			auto_failed,
			not_applicable,
			scorecard_last_update_time`
	if table == scorecardTable {
		qaScoreSelectQuery = `SELECT DISTINCT
				conversation_id,
				scorecard_time AS conversation_start_time,
				agent_user_id,
				scorecard_template_id,
				scorecard_template_revision,
				scorecard_id,
				'' AS score_id,
				'' AS criterion_id,
				manually_scored,
				score/100. AS numeric_value,
				1. AS max_value,
				score/100. AS percentage_value,
				toInt32(1) AS weight,
				0. AS ai_value,
				ai_score_time <> 0 AS ai_scored,
				auto_failed,
				FALSE AS not_applicable,
				scorecard_last_update_time`
	}
	conversationQuery, conversationArgs := qaConversationQuery(conversationConditionsAndArgs, conversationScanBounds)
	scoreQuery, scoreArgs := qaScoreQuery(scoreConditionsAndArgs, table, false)

	// Dedup the latest scorecard version from scorecard_d, not the N/A-filtered
	// scorecard_score CTE, so an all-N/A overwrite is not lost (PR #27274
	// regression; same root cause as RetrieveQAScoreStats).
	scorecardQuery, scorecardArgs := qaScorecardQuery(commonConditionsAndArgs, scorecardConditionsAndArgs, conversationConditionsAndArgs, false)

	allArgs = append(allArgs, conversationArgs...)
	allArgs = append(allArgs, scorecardArgs...)
	allArgs = append(allArgs, scoreArgs...)
	qaScoreInfoQuery := fmt.Sprintf(`
		%s
		%s
		%s,
		qa_score_info AS (
			%s
			FROM
				scorecard_score JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
					AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
		)
	`, conversationQuery, scorecardQuery, scoreQuery, qaScoreSelectQuery)

	return fmt.Sprintf(`
		WITH 
		%s
		%s
		%s
		SELECT 
			qa_score_info.conversation_id AS conversation_id,
			qa_score_info.conversation_start_time AS conversation_start_time,
			qa_score_info.agent_user_id AS agent_user_id,
			qa_score_info.scorecard_template_id AS scorecard_template_id,
			qa_score_info.scorecard_template_revision AS scorecard_template_revision,
			qa_score_info.scorecard_id AS scorecard_id,
			qa_score_info.score_id AS score_id,
			qa_score_info.criterion_id AS criterion_id,
			qa_score_info.manually_scored AS manually_scored,
			qa_score_info.numeric_value AS numeric_value,
			qa_score_info.max_value AS max_value,
			qa_score_info.percentage_value AS percentage_value,
			qa_score_info.weight AS weight,
			qa_score_info.ai_value AS ai_value,
			qa_score_info.ai_scored AS ai_scored,
			qa_score_info.auto_failed AS auto_failed,
			qa_score_info.not_applicable AS not_applicable
		FROM 
			qa_score_info 
			%s
			%s
		-- WHERE is optional.
		%s
		ORDER BY conversation_id, qa_score_info.scorecard_last_update_time DESC
		%s %s
	`,
			// For moment_annotation_filter
			momentSubQueries,
			// For moment_annotation_exclude_filter
			excludedMomentSubQueries,
			// For QA score query
			qaScoreInfoQuery,
			// For main query
			momentJoinQueries, excludedMomentJoinQueries,
			excludedMomentWhereQueries,
			optionalLimit, optionalOffset),
		allArgs
}

// This method will return RetrieveQAConversationsResponse from end to end.
func (a AnalyticsServiceImpl) readQaConversationsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveQAConversationsRequest,
	parent *shared.CustomerProfile,
	pageSize, offset int,
) (*analyticspb.RetrieveQAConversationsResponse, error) {
	// Parse conditions and args for scorecard_score table (and potentially moment_annotation table).
	table := scoreTable
	if req.ScoreResource == analyticspb.RetrieveQAConversationsRequest_QA_SCORE_RESOURCE_SCORECARD {
		table = scorecardTable
	}
	timeRangeOpts, err := qaTimeRangeFilterTargetOptions(req.GetTimeRangeFilterTarget(), req.ConversationTimeRangeField)
	if err != nil {
		return nil, err
	}
	failClosed, err := a.featureFlagClient.BoolValue(ctx, "qaStatsFailClosedOnUnsupportedTimeRange", false)
	if err != nil {
		// fail-open: an unreachable flagd keeps today's approximated bound rather than erroring.
		a.logger.Warnf(ctx, "evaluating qaStatsFailClosedOnUnsupportedTimeRange: %v", err)
	}
	timeRangeOpts = append(timeRangeOpts, WithFailClosedOnUnsupportedTimeRange(failClosed))
	commonConditionsAndArgs, usersExtTables, err := parseCommonConditionsForQAAttribute(req.FilterByTimeRange, req.FilterByAttribute, table, a.enableExtTableForUserFilter, timeRangeOpts...)
	if err != nil {
		return nil, err
	}
	queryExtTables := append([]*ext.Table{}, usersExtTables...)
	scorecardConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(req.FilterByAttribute)
	if err != nil {
		return nil, err
	}
	scoreConditionsAndArgs, scoreExtTables, err := parseScoreConditionsForQAAttribute(req.FilterByTimeRange, req.FilterByAttribute, table, a.enableExtTableForUserFilter, timeRangeOpts...)
	if len(queryExtTables) == 0 {
		queryExtTables = append(queryExtTables, scoreExtTables...)
	}
	if err != nil {
		return nil, err
	}
	conversationTimeRange := req.FilterByTimeRange
	if req.GetTimeRangeFilterTarget() == analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME {
		conversationTimeRange = nil
	}
	conversationConditionsAndArgs, convExtTables, err := parseConversationConditionsForQAAttribute(ctx, conversationTimeRange,
		req.FilterByAttribute,
		a.enableExtTableForUserFilter,
		WithTimeRangeType(req.ConversationTimeRangeField),
	)
	if len(queryExtTables) == 0 {
		queryExtTables = append(queryExtTables, convExtTables...)
	}
	if err != nil {
		return nil, err
	}
	conversationScanBounds, err := qaConversationScanBounds(conversationTimeRange, req.ConversationTimeRangeField)
	if err != nil {
		return nil, err
	}
	timeColumn := conversationStartTimeColumn
	if req.ConversationTimeRangeField == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		timeColumn = conversationEndTimeColumn
	}
	useConversationOutcomeMV := false
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		// fail-open: default to raw moment_annotation when profile config is unavailable.
		a.logger.Errorf(ctx, "getting profile config for QA conversations: %v", err)
	} else {
		useConversationOutcomeMV = profileConfig.GetFeatures().GetInsights().GetUseConversationOutcomeMomentAnnotationMaterializedView()
	}
	useConversationOutcomeMV = conversationOutcomeMVEnabled(useConversationOutcomeMV, timeColumn)
	momentFilters, excludedMomentFilters, valueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
		conversationTimeRange,
		req.FilterByAttribute,
		useConversationOutcomeMV,
		timeColumn,
	)
	if err != nil {
		return nil, err
	}
	chQuery, chArgs := qaConversationsClickhouseQuery(
		commonConditionsAndArgs,
		scorecardConditionsAndArgs,
		scoreConditionsAndArgs,
		conversationConditionsAndArgs,
		conversationScanBounds,
		momentFilters,
		excludedMomentFilters,
		valueOrMissingFilters,
		pageSize, offset, table)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	ctx = attachExtTablesToContext(ctx, queryExtTables...)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("[qaConversations] querying clickhouse: %w", err)
	}
	defer rows.Close()

	// Read QA conversations from scorecard_score_d table in clickhouse.
	chRows := []qaConversationRow{}
	conversationIDs := set.New[string]()
	for rows.Next() {
		row := qaConversationRow{}
		vals := []interface{}{
			&row.conversationID,
			&row.conversationStartTime,
			&row.agentUserID,
			&row.scorecardTemplateID,
			&row.scorecardTemplateRevision,
			&row.scorecardID,
			&row.scoreID,
			&row.criterionID,
			&row.manuallyScored,
			&row.numericValue,
			&row.maxValue,
			&row.percentageValue,
			&row.weight,
			&row.aiValue,
			&row.aiScored,
			&row.autoFailed,
			&row.notApplicable,
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[qaConversations] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
		if row.conversationID != "" {
			conversationIDs.Add(row.conversationID)
		}
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveQaConversations in seconds %v", time.Since(startT).Seconds())

	// Fetch conversation info from DB.
	conversationInfo, err := a.fetchConversationInfoFromDB(ctx, conversationIDs.Slice(), parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	includeComments := req.GetIncludeComments()
	scoreIDToComment := map[string]string{}
	if includeComments && table == scoreTable {
		authedEntity, err := a.authEntityProvider.GetAuthEntity(ctx, parent.CustomerID)
		if err != nil {
			a.logger.Errorf(ctx, "failed to get principal entity from auth token: %v", err)
			return nil, fmt.Errorf("invalid user auth: %w", err)
		}

		scoreIDs := set.New[string]()
		for _, row := range chRows {
			if row.scoreID != "" {
				scoreIDs.Add(row.scoreID)
			}
		}
		scoreIDToComment, err = a.fetchScoreCommentsFromDB(ctx, scoreIDs.Slice(), parent.CustomerID, parent.ProfileID, authedEntity)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch score comments: %w", err)
		}
	}

	// Calculate next page token (if number of returned rows is page size).
	nextPageToken := ""
	if len(chRows) == pageSize {
		nextPageToken = strconv.Itoa(offset + pageSize)
	}

	// Convert to API response.
	return a.convertCHResponseToQaConversationsResponse(parent, chRows, conversationInfo, scoreIDToComment, nextPageToken)
}

func (a AnalyticsServiceImpl) fetchConversationInfoFromDB(
	ctx context.Context,
	conversationIDs []string,
	customerID, profileID string,
) (map[string]*dbmodel.Chats, error) {
	if len(conversationIDs) == 0 {
		return map[string]*dbmodel.Chats{}, nil
	}

	dbChats := []*dbmodel.Chats{}
	err := a.appsDB.DB(ctx).Find(&dbChats,
		"customer_id = ? AND profile_id = ? AND conversation_id IN ?",
		customerID, profileID, conversationIDs,
	).Error
	if err != nil {
		return nil, err
	}

	conversationInfo := make(map[string]*dbmodel.Chats, len(dbChats))
	for _, dbChat := range dbChats {
		if dbChat.ConversationID.Valid {
			conversationInfo[dbChat.ConversationID.String] = dbChat
		}
	}
	return conversationInfo, nil
}

// fetches comments for scores from the database.
func (a AnalyticsServiceImpl) fetchScoreCommentsFromDB(
	ctx context.Context,
	scoreIDs []string,
	customerID, profileID string,
	authedEntity *auth.PrincipalEntity,
) (map[string]string, error) {
	if len(scoreIDs) == 0 {
		return map[string]string{}, nil
	}

	var results []*dbmodel.Scores

	for i := 0; i < len(scoreIDs); i += ScorecardScoreQueryBatchSize {
		end := i + ScorecardScoreQueryBatchSize
		if end > len(scoreIDs) {
			end = len(scoreIDs)
		}
		batch := scoreIDs[i:end]

		var dbScores []*dbmodel.Scores
		conditionBuilder := condition.Builder().
			AndWhere("customer = ?", customerID).
			AndWhere("profile = ?", profileID).
			And(condition.WhereIn("resource_id", batch))
		if err := conditionBuilder.ApplyWhere(a.appsDB.DB(ctx).Select("resource_id", "comment", "comment_access_roles")).Find(&dbScores).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch score comments batch [%d:%d]: %w", i, end, err)
		}

		results = append(results, dbScores...)
	}

	// Filter comments based on access permissions
	if !authedEntity.IsServerOrSuperAdmin() {
		results = fn.Filter(results, func(score *dbmodel.Scores) bool {
			permittedRoles := fn.Map(score.CommentAccessRoles, func(role int32) authpb.AuthProto_Role {
				return authpb.AuthProto_Role(role)
			})
			return len(permittedRoles) == 0 || authedEntity.HasAnyRole(permittedRoles...)
		})
	}

	byScoreID := make(map[string]string, len(results))
	for _, s := range results {
		if !s.Comment.Valid {
			continue
		}
		c := strings.TrimSpace(s.Comment.String)
		if c == "" {
			continue
		}
		byScoreID[s.ResourceID] = c
	}
	return byScoreID, nil
}

func (a AnalyticsServiceImpl) convertCHResponseToQaConversationsResponse(
	parent *shared.CustomerProfile,
	rows []qaConversationRow,
	dbConversations map[string]*dbmodel.Chats,
	scoreIDToComment map[string]string,
	nextPageToken string,
) (*analyticspb.RetrieveQAConversationsResponse, error) {
	scorecardScoreDedups := make(map[string]interface{})
	conversationInfos := make([]*analyticspb.QAConversationInfo, 0, len(rows))
	for _, row := range rows {
		key := row.scorecardID + "_" + row.scoreID
		if _, exists := scorecardScoreDedups[key]; exists {
			// Skip the duplicate rows with the same scorecard ID and score ID.
			continue
		}
		scorecardScoreDedups[key] = nil
		if row.conversationID != "" {
			if _, exists := dbConversations[row.conversationID]; !exists {
				// Skip the conversation not existing in DB.
				// This should not happen.
				continue
			}
		}

		agent := &userpb.User{
			Name: buildUserResourceName(parent.CustomerID, row.agentUserID),
			// Skip more agent info e.g. full name (which should not be used).
		}
		qaConversationInfo := &analyticspb.QAConversationInfo{
			User:                      agent,
			ScoreId:                   row.scoreID,
			ScorecardId:               row.scorecardID,
			ScorecardTemplateId:       row.scorecardTemplateID,
			ScorecardTemplateRevision: row.scorecardTemplateRevision,
			CriterionId:               row.criterionID,
			ManuallyScored:            row.manuallyScored,
			NumericValue:              float32(row.numericValue),
			MaxValue:                  float32(row.maxValue),
			PercentageValue:           float32(row.percentageValue),
			Weight:                    row.weight,
			AiValue:                   float32(row.aiValue),
			AiScored:                  row.aiScored,
			AutoFailed:                row.autoFailed,
			NotApplicable:             row.notApplicable,
		}

		if row.conversationID != "" {
			qaConversationInfo.Conversation = &convpb.Conversation{
				Name: convpb.ConversationName{
					CustomerID:     parent.CustomerID,
					ProfileID:      parent.ProfileID,
					ConversationID: row.conversationID,
				}.String(),
				PlatformInfo: &convpb.Conversation_PlatformInfo{
					PlatformConversationId: dbConversations[row.conversationID].PlatformChatID.String,
					PlatformAccountId:      dbConversations[row.conversationID].PlatformAccountID,
				},
				StartTime: shared.ConvertTimeToAPITimestamp(row.conversationStartTime),
				EndTime:   shared.ConvertTimeToAPITimestamp(converter.ConvertNullTimeToTime(dbConversations[row.conversationID].EndedAt)),
			}
		}

		if len(scoreIDToComment) > 0 && scoreIDToComment[row.scoreID] != "" {
			qaConversationInfo.Comment = scoreIDToComment[row.scoreID]
		}

		conversationInfos = append(conversationInfos, qaConversationInfo)
	}
	return &analyticspb.RetrieveQAConversationsResponse{
		ConversationInfos: conversationInfos,
		NextPageToken:     nextPageToken,
	}, nil
}

type qaConversationRow struct {
	conversationID        string
	conversationStartTime time.Time
	agentUserID           string
	// scorecard_score info
	scorecardTemplateID       string
	scorecardTemplateRevision string
	scorecardID               string
	scoreID                   string
	criterionID               string
	manuallyScored            bool
	numericValue              float64
	maxValue                  float64
	percentageValue           float64
	weight                    int32
	aiValue                   float64
	aiScored                  bool
	autoFailed                bool
	notApplicable             bool
}
