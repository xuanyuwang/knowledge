package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/ext"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// qaStartTimeCorrectionSlack pads the conversation_start_time bounds in
// qaConversationScanBounds, because ingest can correct a conversation's start
// time after the first write. It matches startTimeCorrectionSlack in
// shared/clickhouse/conversations.
const qaStartTimeCorrectionSlack = 24 * time.Hour

// qaNewestVersionFilterColumns are the conversation_d columns whose conditions
// qaConversationQuery applies to each conversation's newest version. They change
// between versions: the real end replaces the placeholder end (end = start), and
// ingest can correct the start time, duration and voicemail flag. Conditions on
// agent_user_id and usecase_id match any version instead: the score and scorecard
// CTEs filter those columns on the scorecard's own copy, and checking the
// conversation's newest agent would drop agent A's scorecard on a conversation
// later reassigned to agent B.
var qaNewestVersionFilterColumns = []string{
	conversationStartTimeColumn,
	conversationEndTimeColumn,
	conversationDurationSecsColumn,
	isVoicemailColumn,
}

// qaConversationScanBounds bounds the conversation_d scans in qaConversationQuery
// on conversation_start_time, which leads the sort key. It returns nil when there
// is no conversation time range, and qaConversationQuery then skips the collapse.
//
// The bounds only prune; the time range itself is applied to the newest version.
// Both range types bound the start to the range padded by
// qaStartTimeCorrectionSlack. An end-time range needs no reach-back for
// conversations that started before it: parseCommonConditionsForQAAttribute bounds
// scorecard_time, the scorecard's copy of the conversation start, to the same
// range, so those conversations have no scores to join. An end-time range also
// adds the end-or-update condition, which limits the scan to conversations that
// ended or were rewritten since the range began. update_time is the wall-clock
// time of the write, which is never before a real end, so if any version passes
// that condition, the newest one does too.
func qaConversationScanBounds(
	timeRange *analyticspb.TimeRange,
	timeRangeType analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
) ([]conditionAndArg, error) {
	if timeRange == nil {
		return nil, nil
	}
	startTime, endTime, err := shared.ParseFilterByTimeRange(timeRange)
	if err != nil {
		return nil, err
	}
	conStart, argStart := columnGTE(conversationStartTimeColumn, startTime.Add(-qaStartTimeCorrectionSlack).Format(timestampFormat))
	conEnd, argEnd := columnLT(conversationStartTimeColumn, endTime.Add(qaStartTimeCorrectionSlack).Format(timestampFormat))
	bounds := []conditionAndArg{
		{condition: conStart, arg: argStart},
		{condition: conEnd, arg: argEnd},
	}
	if timeRangeType == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		rangeStart := startTime.Format(timestampFormat)
		bounds = append(bounds, conditionAndArg{
			condition: fmt.Sprintf("%s >= ? OR %s >= ?", conversationEndTimeColumn, updateTimeColumn),
			args:      []any{rangeStart, rangeStart},
		})
	}
	return bounds, nil
}

// qaConversationQuery builds the conversation CTE: one row per conversation that
// passes conversationConditionsAndArgs, with the newest version's value of each
// qaNewestVersionFilterColumns column the conditions filter on.
//
// conversation_d holds every version of a conversation until ReplacingMergeTree
// merges them, and versions that differ in start time or agent never merge, since
// both lead the sort key. Filtered as raw rows, an older version can pass, and
// selecting conversation_end_time returns one row per distinct end time, which the
// scorecard join then counts once per row. So the CTE collapses each conversation
// to its newest version with argMax and applies the newest-version conditions to
// that.
//
// A conversation is kept when some version passes every condition and its newest
// version passes the newest-version conditions. The GLOBAL IN candidate set
// applies the first part before the GROUP BY, so the collapse aggregates only
// those conversations rather than every conversation in the scan bounds. GLOBAL,
// because a conversation's versions can sit on different shards.
//
// One argMax over a tuple keeps all the columns from the same version, even if two
// versions tie on update_time. The aliases sit one scope above the scan; sharing a
// scope would let ClickHouse read a scan condition's column as the alias and
// reject the query for holding an aggregate in a WHERE.
//
// Without scan bounds (a submit-time range, where conversations are not
// time-bounded) the CTE is SELECT DISTINCT conversation_id over the raw rows. It
// returns one row per conversation, so it cannot double count, but an older
// version can pass a filter on a column that has since changed.
func qaConversationQuery(
	conversationConditionsAndArgs []conditionAndArg,
	conversationScanBounds []conditionAndArg,
) (string, []any) {
	if len(conversationConditionsAndArgs) == 0 {
		return "", []any{}
	}
	conditions, args := concatConditionsAndArgs(conversationConditionsAndArgs)
	newestVersionConditionsAndArgs, newestVersionColumns := qaNewestVersionConditions(conversationConditionsAndArgs)
	if len(conversationScanBounds) == 0 || len(newestVersionConditionsAndArgs) == 0 {
		return fmt.Sprintf(`
			conversation AS (
				SELECT DISTINCT
					conversation_id
				FROM
					conversation_d
				WHERE
					%s
			),
	`, conditions), args
	}

	selectColumns := []string{conversationIDColumn}
	for i, column := range newestVersionColumns {
		selectColumns = append(selectColumns, fmt.Sprintf("latest.%d AS %s", i+1, column))
	}
	bounds, boundArgs := concatConditionsAndArgs(conversationScanBounds)
	newestVersionConditions, newestVersionArgs := concatConditionsAndArgs(newestVersionConditionsAndArgs)
	return fmt.Sprintf(`
			conversation AS (
				SELECT
					%s
				FROM (
					SELECT
						conversation_id,
						argMax(tuple(%s), update_time) AS latest
					FROM
						conversation_d
					WHERE
						%s
						AND conversation_id GLOBAL IN (
							SELECT
								conversation_id
							FROM
								conversation_d
							WHERE
								%s
								AND %s
						)
					GROUP BY
						conversation_id
				)
				WHERE
					%s
			),
	`, strings.Join(selectColumns, ", "), strings.Join(newestVersionColumns, ", "), bounds, bounds, conditions, newestVersionConditions),
		slices.Concat(boundArgs, boundArgs, args, newestVersionArgs)
}

// qaNewestVersionConditions returns the conditions that reference a
// qaNewestVersionFilterColumns column, and the columns they reference, in
// qaNewestVersionFilterColumns order.
func qaNewestVersionConditions(conditionsAndArgs []conditionAndArg) (newestVersionConditionsAndArgs []conditionAndArg, columns []string) {
	referencesColumn := func(condition conditionAndArg, column string) bool {
		return strings.Contains(condition.condition, column)
	}
	for _, condition := range conditionsAndArgs {
		if !slices.ContainsFunc(qaNewestVersionFilterColumns, func(column string) bool { return referencesColumn(condition, column) }) {
			continue
		}
		newestVersionConditionsAndArgs = append(newestVersionConditionsAndArgs, condition)
	}
	for _, column := range qaNewestVersionFilterColumns {
		if !slices.ContainsFunc(newestVersionConditionsAndArgs, func(condition conditionAndArg) bool { return referencesColumn(condition, column) }) {
			continue
		}
		columns = append(columns, column)
	}
	return newestVersionConditionsAndArgs, columns
}

func qaScorecardQuery(
	commonConditionsAndArgs []conditionAndArg,
	scorecardConditionsAndArgs []conditionAndArg,
	conversationConditionsAndArgs []conditionAndArg,
	needsConversationEndTime bool,
) (string, []any) {
	tableName := getTableName(scorecardTable)
	commonConditions, commonArgs := concatConditionsAndArgs(commonConditionsAndArgs)
	scorecardConditions, scorecardArgs := concatConditionsAndArgs(scorecardConditionsAndArgs)
	optionalScorecardConditions := ""
	if scorecardConditions != "" {
		optionalScorecardConditions = fmt.Sprintf("WHERE %s", scorecardConditions)
	}

	optionalConversationJoin := ""
	filteredScorecardSelectColumns := []string{
		"scorecard.scorecard_id AS scorecard_id",
		"scorecard.scorecard_last_update_time AS scorecard_last_update_time",
	}

	if len(conversationConditionsAndArgs) > 0 || needsConversationEndTime {
		optionalConversationJoin = "JOIN conversation ON scorecard.conversation_id = conversation.conversation_id"
		if needsConversationEndTime {
			filteredScorecardSelectColumns = append(filteredScorecardSelectColumns, "conversation.conversation_end_time AS conversation_end_time")
		}
	}
	whereCondition := ""
	if commonConditions != "" {
		whereCondition = fmt.Sprintf(" WHERE %s ", commonConditions)
	}
	allArgs := make([]any, 0, len(commonArgs)+len(scorecardArgs))
	allArgs = append(allArgs, commonArgs...)
	allArgs = append(allArgs, scorecardArgs...)
	return fmt.Sprintf(`
			scorecard AS (
				SELECT
					scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
				FROM
					%s
					%s
			),
			scorecard_last_update AS (
				SELECT
					scorecard_id,
					max(scorecard_last_update_time) AS scorecard_last_update_time
				FROM
					scorecard
				GROUP BY
					scorecard_id
			),
			filtered_scorecard AS (
				SELECT DISTINCT
					%s
				FROM scorecard
				JOIN scorecard_last_update ON scorecard.scorecard_id = scorecard_last_update.scorecard_id
					AND scorecard.scorecard_last_update_time = scorecard_last_update.scorecard_last_update_time
				%s
				%s
			),
	`, tableName, whereCondition, strings.Join(filteredScorecardSelectColumns, ","), optionalConversationJoin, optionalScorecardConditions), allArgs
}

func qaScoreQuery(
	scoreConditionsAndArgs []conditionAndArg,
	table ClickhouseTable,
	onlySelectScoreColumns bool,
) (string, []any) {
	return qaScoreQueryWithSubmitterProjection(scoreConditionsAndArgs, table, onlySelectScoreColumns, false, false)
}

func qaScoreQueryWithSubmitterProjection(
	scoreConditionsAndArgs []conditionAndArg,
	table ClickhouseTable,
	onlySelectScoreColumns bool,
	includeSubmitterUserID bool,
	includeScorecardTemplateID bool,
) (string, []any) {
	tableName := getTableName(table)
	conditions, args := concatConditionsAndArgs(scoreConditionsAndArgs)
	// The columns should cover the columns used by the following filters & group-by's & scorings.
	//
	// The DISTINCT below is what keeps SUM(percentage_value * float_weight)
	// downstream from double counting. score_d and scorecard_d are distributed over
	// base tables, so ReplacingMergeTree has not necessarily collapsed a re-ingested
	// row yet: on a 7-day window 35.6% of raw score_d rows on cvs and 6.9% on oportun
	// are duplicates. Those duplicates are byte-identical across every column
	// projected here, so DISTINCT removes all of them — measured, the projected
	// row count equals uniqExact(conversation_id, scorecard_id, score_id,
	// scorecard_last_update_time) exactly on cvs and to within 6 rows on oportun.
	//
	// So do not add LIMIT 1 BY or FINAL here. Either one buys nothing over DISTINCT
	// and costs a sort or a merge over ~10-16M rows per dashboard query. What
	// DISTINCT does *not* cover is two rows sharing an identity and a version but
	// disagreeing on a projected value, which a re-score that skips the
	// scorecard_last_update_time bump would produce; neither customer shows one.
	selectQuery := ""
	if !onlySelectScoreColumns {
		selectQuery = `*`
	} else {
		scoreColumns := []string{"scorecard_time", "agent_user_id"}
		if includeSubmitterUserID {
			scoreColumns = append(scoreColumns, "submitter_user_id")
		}
		scoreColumns = append(scoreColumns, "conversation_id", "scorecard_id", "scorecard_last_update_time")
		selectQuery = `DISTINCT ` + strings.Join(scoreColumns, ", ")
		if table == scorecardTable {
			selectQuery += `, score, scorecard_submit_time`
		} else {
			selectQuery += `, criterion_id, percentage_value, float_weight, scorecard_submit_time, manually_scored, scorecard_publish_time`
			// Projected only when grouping by criterion, to key by (scorecard_template_id,
			// criterion_id) since criterion ids aren't unique across templates (CONVI-7014).
			if includeScorecardTemplateID {
				selectQuery += `, scorecard_template_id`
			}
		}
	}

	return fmt.Sprintf(`
			scorecard_score AS (
				SELECT
					%s
				FROM
					%s
				WHERE
					%s
			)
			
	`, selectQuery, tableName, conditions), args
}

// buildRankingSelectQuery returns an optional SQL fragment that computes a
// ranking_weighted_percentage_sum column. When lower-is-better criteria are
// present, it negates the weighted values for those criteria so that a
// standard descending sort ranks agents correctly for quintile assignment.
// The criterion IDs are supplied via a ClickHouse external table to handle
// arbitrarily large lists.
func buildRankingSelectQuery(hasLowerIsBetterCriteria bool, table ClickhouseTable) string {
	if !hasLowerIsBetterCriteria || table == scorecardTable {
		return ""
	}
	return fmt.Sprintf(`,
SUM(
	CASE WHEN criterion_id IN (SELECT %s FROM %s)
		THEN -percentage_value * float_weight
		ELSE percentage_value * float_weight
	END
) FILTER (WHERE percentage_value >= 0) AS ranking_weighted_percentage_sum`,
		extTableLowerIsBetterCriterionColumn, extTableLowerIsBetterCriterion)
}

func hasGroupByKey(groupByKeys []string, key string) bool {
	for _, groupByKey := range groupByKeys {
		if groupByKey == key {
			return true
		}
	}
	return false
}

func qaScoreStatsClickhouseQuery(
	commonConditionsAndArgs []conditionAndArg,
	scorecardConditionsAndArgs []conditionAndArg,
	scoreConditionsAndArgs []conditionAndArg,
	conversationConditionsAndArgs []conditionAndArg,
	conversationScanBounds []conditionAndArg,
	groupByKeys []string,
	groupByQueries []string,
	table ClickhouseTable,
	needsConversationEndTime bool,
	hasLowerIsBetterCriteria bool,
) (string, []any) {
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	scoreSelectQuery := `SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
		SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum,
		COUNT(DISTINCT conversation_id) AS total_conversation_count,
		COUNT(DISTINCT scorecard_id) AS total_scorecard_count`
	if table == scorecardTable {
		scoreSelectQuery = `SUM(score/100.) FILTER (WHERE score >= 0) AS weighted_percentage_sum,
			SUM(1.) FILTER (WHERE score >= 0) AS weight_sum,
			COUNT(DISTINCT conversation_id) AS total_conversation_count,
			COUNT(DISTINCT scorecard_id) AS total_scorecard_count`
	}
	rankingSelectQuery := buildRankingSelectQuery(hasLowerIsBetterCriteria, table)

	conversationQuery, conversationArgs := qaConversationQuery(conversationConditionsAndArgs, conversationScanBounds)
	// Dedup the latest scorecard version from scorecard_d (scorecard-level rows),
	// not from the scorecard_score CTE. scorecard_score reads score_d filtered by
	// the N/A value predicate (percentage_value >= 0 AND not_applicable != true),
	// so an all-N/A overwrite has no rows there; a score_d-based dedup would miss it
	// and fall back to the stale prior version (PR #27274 regression; see
	// TestDedupOptimization_LatestVersionAllNA).
	scorecardQuery, scorecardArgs := qaScorecardQuery(commonConditionsAndArgs, scorecardConditionsAndArgs, conversationConditionsAndArgs, needsConversationEndTime)
	scoreQuery, scoreArgs := qaScoreQueryWithSubmitterProjection(scoreConditionsAndArgs, table, true, hasGroupByKey(groupByKeys, submitterUserIDColumn), hasGroupByKey(groupByKeys, scorecardTemplateIDColumn))
	allArgs := make([]any, 0, len(conversationArgs)+len(scorecardArgs)+len(scoreArgs))
	allArgs = append(allArgs, conversationArgs...)
	allArgs = append(allArgs, scorecardArgs...)
	allArgs = append(allArgs, scoreArgs...)
	return fmt.Sprintf(`
		WITH
		%s
		%s
		%s
		SELECT
			%s
			%s
			%s
		FROM
			scorecard_score JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
				AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
		-- GROUP BY is optional.
		%s
		SETTINGS max_bytes_before_external_group_by = 5000000000
	`, conversationQuery, scorecardQuery, scoreQuery,
			concatQueries(groupByQueries), scoreSelectQuery, rankingSelectQuery, optionalGroupBy),
		allArgs
}

func qaScoreStatsClickhouseQueryWithMetadataView(
	commonConditionsAndArgs []conditionAndArg,
	scorecardConditionsAndArgs []conditionAndArg,
	scoreConditionsAndArgs []conditionAndArg,
	conversationConditionsAndArgs []conditionAndArg,
	conversationScanBounds []conditionAndArg,
	momentFilters, excludedMomentFilters []momentAnnotationFilter,
	valueOrMissingFilters []qaMomentValueOrMissingFilter,
	groupByKeys []string,
	groupByQueries []string,
	table ClickhouseTable,
	needsConversationEndTime bool,
	hasLowerIsBetterCriteria bool,
) (string, []any) {
	allArgs := []any{}

	// Build sub queries based on moment annotations (per-group table routing).
	momentSubQueryList := []string{}
	momentJoinQueryList := []string{}
	momentIndex := 0
	for _, momentFilter := range momentFilters {
		i := momentIndex
		resultConversationID := fmt.Sprintf("conversation_id_%d", i)
		subQuery, joinQuery, args := buildQAMomentFilterQuery(
			momentFilter,
			i,
			"INNER JOIN",
			"scorecard_score_per_conversation.conversation_id",
			resultConversationID,
		)
		momentSubQueryList = append(momentSubQueryList, subQuery)
		momentJoinQueryList = append(momentJoinQueryList, joinQuery)
		allArgs = append(allArgs, args...)
		momentIndex++
	}

	selectedValueIndices := make([]int, len(valueOrMissingFilters))
	for filterIdx, filter := range valueOrMissingFilters {
		i := momentIndex
		selectedValueIndices[filterIdx] = i

		resultConversationID := fmt.Sprintf("conversation_id_%d", i)
		subQuery, joinQuery, args := buildQAMomentFilterQuery(
			filter.selectedValueFilter,
			i,
			"LEFT JOIN",
			"scorecard_score_per_conversation.conversation_id",
			resultConversationID,
		)
		momentSubQueryList = append(momentSubQueryList, subQuery)
		momentJoinQueryList = append(momentJoinQueryList, joinQuery)
		allArgs = append(allArgs, args...)
		momentIndex++
	}

	momentSubQueries := strings.Join(momentSubQueryList, "\n")
	momentJoinQueries := strings.Join(momentJoinQueryList, "\n")

	// Build sub queries based on excluded (metadata) moments.
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
				scorecard_score_per_conversation.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
		`, i, i, i))
		excludedMomentWhereQueryList = append(excludedMomentWhereQueryList, fmt.Sprintf(`
			AND moment_annotation_exclude_filter_%d.conversation_id_exclude_%d = ''
		`, i, i))
	}
	valueOrMissingWhereQueryList := []string{}
	missingBaseIndex := len(excludedMomentFilters)
	for filterIdx, filter := range valueOrMissingFilters {
		missingIndex := missingBaseIndex + filterIdx
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
				scorecard_score_per_conversation.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
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

	optionalGroupBy := ""
	optionalGroupByKeysWithPrefixComma := ""
	optionalGroupByKeysWithSuffixComma := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
		optionalGroupByKeysWithPrefixComma = ", " + concatKeys(groupByKeys)
		optionalGroupByKeysWithSuffixComma = concatKeys(groupByKeys) + ", "
	}

	// Table name.
	scoreSelectQuery := `SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
		SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum`
	if table == scorecardTable {
		scoreSelectQuery = `SUM(score/100.) FILTER (WHERE score >= 0) AS weighted_percentage_sum,
			SUM(1.) FILTER (WHERE score >= 0) AS weight_sum`
	}
	rankingSelectQuery := buildRankingSelectQuery(hasLowerIsBetterCriteria, table)
	outerRankingSelect := ""
	if rankingSelectQuery != "" {
		outerRankingSelect = ",\nSUM(ranking_weighted_percentage_sum) AS ranking_weighted_percentage_sum"
	}

	conversationQuery, conversationArgs := qaConversationQuery(conversationConditionsAndArgs, conversationScanBounds)
	// Dedup the latest scorecard version from scorecard_d (scorecard-level rows),
	// not from the N/A-filtered scorecard_score CTE. See qaScoreStatsClickhouseQuery
	// for why (PR #27274 all-N/A regression).
	scorecardQuery, scorecardArgs := qaScorecardQuery(commonConditionsAndArgs, scorecardConditionsAndArgs, conversationConditionsAndArgs, needsConversationEndTime)
	scoreQuery, scoreArgs := qaScoreQueryWithSubmitterProjection(scoreConditionsAndArgs, table, true, hasGroupByKey(groupByKeys, submitterUserIDColumn), hasGroupByKey(groupByKeys, scorecardTemplateIDColumn))
	allArgs = append(allArgs, conversationArgs...)
	allArgs = append(allArgs, scorecardArgs...)
	allArgs = append(allArgs, scoreArgs...)
	return fmt.Sprintf(`
		WITH
		%s
		%s
		%s
		%s
		%s,
		scorecard_score_per_conversation AS (
			SELECT
				conversation_id,
				scorecard_id,
				%s
				%s
				%s
			FROM
				scorecard_score JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
					AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
			GROUP BY conversation_id, scorecard_id %s
		)
		SELECT
			%s
			SUM(weighted_percentage_sum) AS weighted_percentage_sum,
			SUM(weight_sum) AS weight_sum,
			COUNT(DISTINCT conversation_id) AS total_conversation_count,
			COUNT(DISTINCT scorecard_id) AS total_scorecard_count
			%s
		FROM
			scorecard_score_per_conversation
				%s
				%s
		-- WHERE is optional.
		%s
		-- GROUP BY is optional.
		%s
		SETTINGS max_bytes_before_external_group_by = 5000000000
		`, momentSubQueries, excludedMomentSubQueries,
			conversationQuery, scorecardQuery, scoreQuery,
			// scorecard_score_per_conversation
			concatQueries(groupByQueries), scoreSelectQuery, rankingSelectQuery, optionalGroupByKeysWithPrefixComma,
			// main query
			optionalGroupByKeysWithSuffixComma,
			outerRankingSelect,
			momentJoinQueries, excludedMomentJoinQueries,
			excludedMomentWhereQueries,
			optionalGroupBy),
		allArgs
}

func (a AnalyticsServiceImpl) readQaScoreStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveQAScoreStatsRequest,
	users []*userpb.User,
	lowerIsBetterCriteria []string,
) (*qaScoreStatsResult, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	table := scoreTable
	if req.ScoreResource == analyticspb.RetrieveQAScoreStatsRequest_QA_SCORE_RESOURCE_SCORECARD {
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
	if err != nil {
		return nil, err
	}
	if len(queryExtTables) == 0 {
		queryExtTables = append(queryExtTables, scoreExtTables...)
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
	if err != nil {
		return nil, err
	}
	if len(queryExtTables) == 0 {
		queryExtTables = append(queryExtTables, convExtTables...)
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
		a.logger.Errorf(ctx, "getting profile config for QA score stats: %v", err)
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
	if req.GetTimeRangeFilterTarget() == analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME {
		// Older conversations may have scorecards submitted in this window. Bound
		// every annotation scan by the selected scorecards, not conversation time.
		// GLOBAL IN shares the candidate IDs across distributed annotation shards.
		candidateConversations := conditionAndArg{condition: "conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)"}
		for i := range momentFilters {
			momentFilters[i].conditions = append(momentFilters[i].conditions, candidateConversations)
		}
		for i := range excludedMomentFilters {
			excludedMomentFilters[i].conditions = append(excludedMomentFilters[i].conditions, candidateConversations)
		}
		for i := range valueOrMissingFilters {
			filter := &valueOrMissingFilters[i]
			filter.selectedValueFilter.conditions = append(filter.selectedValueFilter.conditions, candidateConversations)
			filter.existenceFilter.conditions = append(filter.existenceFilter.conditions, candidateConversations)
		}
	}
	groupByKeys, groupByQueries, err := parseClickhouseGroupByForQAAttribute(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, table, timeRangeOpts...)
	if err != nil {
		return nil, err
	}
	needsConversationEndTime := req.ConversationTimeRangeField == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT &&
		req.GetTimeRangeFilterTarget() != analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME
	hasLowerIsBetterCriteria := len(lowerIsBetterCriteria) > 0 && table != scorecardTable

	// Build ext table for lower-is-better criterion IDs (used in ranking column).
	var lowerIsBetterExtTable *ext.Table
	if hasLowerIsBetterCriteria {
		var extErr error
		lowerIsBetterExtTable, extErr = buildLowerIsBetterCriteriaExtTable(lowerIsBetterCriteria)
		if extErr != nil {
			return nil, fmt.Errorf("building lower-is-better ext table: %w", extErr)
		}
	}

	var chQuery string
	var chArgs []any
	if len(momentFilters) == 0 && len(excludedMomentFilters) == 0 && len(valueOrMissingFilters) == 0 {
		chQuery, chArgs = qaScoreStatsClickhouseQuery(
			commonConditionsAndArgs,
			scorecardConditionsAndArgs,
			scoreConditionsAndArgs,
			conversationConditionsAndArgs,
			conversationScanBounds,
			groupByKeys,
			groupByQueries,
			table,
			needsConversationEndTime,
			hasLowerIsBetterCriteria,
		)
	} else {
		chQuery, chArgs = qaScoreStatsClickhouseQueryWithMetadataView(
			commonConditionsAndArgs,
			scorecardConditionsAndArgs,
			scoreConditionsAndArgs,
			conversationConditionsAndArgs,
			conversationScanBounds,
			momentFilters,
			excludedMomentFilters,
			valueOrMissingFilters,
			groupByKeys,
			groupByQueries,
			table,
			needsConversationEndTime,
			hasLowerIsBetterCriteria,
		)
	}
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	queryExtTables = append(queryExtTables, lowerIsBetterExtTable)
	ctx = attachExtTablesToContext(ctx, queryExtTables...)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	chRows := []qaScoreStatsRow{}
	for rows.Next() {
		row := qaScoreStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case criterionIDColumn:
				vals = append(vals, &row.criterionID)
			case scorecardTemplateIDColumn:
				vals = append(vals, &row.scorecardTemplateID)
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case submitterUserIDColumn:
				vals = append(vals, &row.submitterUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			}
		}
		vals = append(vals, &row.weightedPercentageSum, &row.weightSum, &row.totalConversationCount, &row.totalScorecardCount)
		if hasLowerIsBetterCriteria {
			vals = append(vals, &row.rankingWeightedPercentageSum)
			row.hasRankingScore = true
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[qaScoreStats] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveQAScoreStats in seconds %v", time.Since(startT).Seconds())
	return a.convertCHResponseToQaScoreStatsResponse(parent.CustomerID, chRows, req.GroupByAttributeTypes, users)
}

// qaScoreStatsResult holds the converted response along with an optional
// ranking scores map for quintile assignment when lower-is-better criteria exist.
type qaScoreStatsResult struct {
	response      *analyticspb.RetrieveQAScoreStatsResponse
	rankingScores map[string]float64 // User.Name -> ranking score; nil when no directionality needed.
}

func (a AnalyticsServiceImpl) convertCHResponseToQaScoreStatsResponse(
	customerID string,
	rows []qaScoreStatsRow,
	groupByAttributes []analyticspb.QAAttributeType,
	users []*userpb.User,
) (*qaScoreStatsResult, error) {
	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}

	var totalWeightedPercentage, totalWeightSum float64
	var totalConversationCountSum int64
	var totalScorecardCountSum int64
	scores := []*analyticspb.QAScore{}
	var rankingScores map[string]float64
	for _, row := range rows {
		totalWeightedPercentage += row.weightedPercentageSum
		totalWeightSum += row.weightSum
		totalConversationCountSum += int64(row.totalConversationCount)
		totalScorecardCountSum += int64(row.totalScorecardCount)
		scores = append(scores, &analyticspb.QAScore{
			GroupedBy:              convertQaGroupByForQaScoreStatsRow(row, customerID, groupByAttributes, usernameToUserMap),
			Score:                  safeDivideFloat(row.weightedPercentageSum, row.weightSum),
			WeightSum:              float32(row.weightSum),
			TotalConversationCount: int32(row.totalConversationCount),
			TotalScorecardCount:    int32(row.totalScorecardCount),
		})
		if row.hasRankingScore {
			if rankingScores == nil {
				rankingScores = make(map[string]float64, len(rows))
			}
			key := rankingScoreKey(scores[len(scores)-1])
			rankingScores[key] = float64(safeDivideFloat(row.rankingWeightedPercentageSum, row.weightSum))
		}
	}
	sort.Slice(scores, func(i, j int) bool {
		score1 := scores[i]
		score2 := scores[j]
		if score1.GroupedBy.TimeRange == nil || score2.GroupedBy.TimeRange == nil {
			return false
		}
		return score1.GroupedBy.TimeRange.StartTimestamp.AsTime().Before(score2.GroupedBy.TimeRange.StartTimestamp.AsTime())
	})
	resp := &analyticspb.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analyticspb.QAScoreResult{
			Scores:                 scores,
			AverageQaScore:         safeDivideFloat(totalWeightedPercentage, totalWeightSum),
			TotalConversationCount: int32(totalConversationCountSum),
			TotalScorecardCount:    int32(totalScorecardCountSum),
		},
	}
	return &qaScoreStatsResult{response: resp, rankingScores: rankingScores}, nil
}

func convertQaGroupByForQaScoreStatsRow(
	row qaScoreStatsRow,
	customerID string,
	groupByAttributes []analyticspb.QAAttributeType,
	users map[string]*userpb.User,
) *analyticspb.QAScoreGroupBy {
	groupBy := &analyticspb.QAScoreGroupBy{}
	for _, attribute := range groupByAttributes {
		switch attribute {
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT:
			groupBy.User = buildGroupByUser(customerID, row.agentUserID, users)
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER:
			groupBy.User = buildGroupByUser(customerID, row.agentUserID, users)
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER:
			groupBy.User = buildGroupByUser(customerID, row.submitterUserID, users)
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE:
			groupBy.TimeRange = &analyticspb.TimeRange{
				StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime),
			}
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION:
			groupBy.CriterionId = row.criterionID
			// Carry the template id: criterion ids aren't unique across templates (CONVI-7014).
			groupBy.ScorecardTemplateId = row.scorecardTemplateID
		}
	}
	return groupBy
}

func buildGroupByUser(customerID, userID string, users map[string]*userpb.User) *userpb.User {
	userName := buildUserResourceName(customerID, userID)
	if u, exists := users[userName]; exists {
		return &userpb.User{
			Name:        u.Name,
			FullName:    u.FullName,
			DisplayName: u.DisplayName,
			Username:    u.Username,
		}
	}
	return &userpb.User{
		Name: userName,
	}
}

func getTableName(table ClickhouseTable) string {
	switch table {
	case scorecardScoreTable:
		return scorecardScoreTableName
	case scoreTable:
		return scoreTableName
	case scorecardTable:
		return scorecardTableName
	default:
		return ""
	}
}

type qaScoreStatsRow struct {
	clickhouseKeyTimeRow
	weightedPercentageSum        float64
	weightSum                    float64
	totalConversationCount       uint64
	totalScorecardCount          uint64
	rankingWeightedPercentageSum float64 // Only populated when lower-is-better criteria exist.
	hasRankingScore              bool    // True when rankingWeightedPercentageSum is populated.
}
