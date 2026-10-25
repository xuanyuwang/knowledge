package analyticsimpl

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/ext"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// Callers resolve ACLs and return early for an empty authorized user selection
// before constructing these inputs. An empty attribute user list means all users.
type conversationPopulationInputs struct {
	timeRange                        *analyticspb.TimeRange
	attribute                        *analyticspb.Attribute
	timeRangeField                   analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange
	includeBotConversations          bool
	allowMatchingStaleMetadataValues bool
	useExternalUserTable             bool
}

// query is a complete SELECT with one row per eligible conversation/agent pair:
// conversation_id, agent_user_id, conversation_time, and matching-message aht_sec.
// Keep its bind arguments and external tables when embedding it in another query.
type conversationPopulationQuery struct {
	query          string
	args           []any
	externalTables []*ext.Table
}

func buildConversationPopulationQuery(ctx context.Context, inputs conversationPopulationInputs) (conversationPopulationQuery, error) {
	timeColumn := getConversationTimeRangeColumn(getConversationQueryOptions(WithTimeRangeType(inputs.timeRangeField)))
	attribute, valueOrMissingFilters, err := parseConversationPopulationValueOrMissingFilters(inputs.timeRange, inputs.attribute, timeColumn)
	if err != nil {
		return conversationPopulationQuery{}, err
	}
	conditions, moments, excludedMoments, usersTable, err := parseClickhouseFilter(ctx, inputs.timeRange, attribute, []ClickhouseTable{messageTable}, inputs.useExternalUserTable, WithTimeRangeType(inputs.timeRangeField))
	if err != nil {
		return conversationPopulationQuery{}, err
	}
	conversationTables := []ClickhouseTable{conversationLabelTable, conversationTable}
	conversationConditions, err := buildTimeRangeConditionAndArgs(inputs.timeRange, conversationTables, timeColumn)
	if err != nil {
		return conversationPopulationQuery{}, err
	}
	if inputs.attribute != nil {
		userConditions, conversationUsersTable, err := buildUsersConditionAndArgs(inputs.attribute.GetUsers(), conversationTables, inputs.useExternalUserTable)
		if err != nil {
			return conversationPopulationQuery{}, err
		}
		if usersTable == nil {
			usersTable = conversationUsersTable
		}
		conversationConditions = mergeTableConditionsAndArgs(conversationConditions, userConditions)
	}
	sourceCondition := "conversation_source IN (0, 8)"
	if inputs.includeBotConversations {
		sourceCondition = "conversation_source IN (0, 7, 8)"
	}
	conversationQuery, conversationArgs := buildAgentAssistFiltersSubQuery(inputs.attribute.GetAgentAssistFilters(), conversationConditions, sourceCondition, timeColumn)
	query, args := buildConversationPopulationSelect(conditions, moments, excludedMoments, valueOrMissingFilters, timeColumn, conversationQuery, conversationArgs, sourceCondition, inputs.allowMatchingStaleMetadataValues)
	result := conversationPopulationQuery{query: query, args: args}
	if usersTable != nil {
		result.externalTables = []*ext.Table{usersTable}
	}
	return result, nil
}

func buildAgentAssistFiltersSubQuery(
	agentAssistFilters []*analyticspb.Attribute_AgentAssistFilter,
	conversationCondAndArgs map[ClickhouseTable][]conditionAndArg,
	conversationSourceCondition string,
	conversationTimeColumn string,
) (string, []any) {
	conds, args := concatConditionsAndArgs(conversationCondAndArgs[conversationLabelTable])
	// in default, use all conversation IDs when there is no AA filter
	convWithAASubQuery := fmt.Sprintf(`
		%s AS (
			SELECT
				agent_user_id, conversation_id, %s
			FROM
				conversation_d
			WHERE TRUE
				AND %s
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND %s
			ORDER BY update_time DESC
			LIMIT 1 BY agent_user_id, conversation_id
		)`, ConvWithAa, conversationTimeColumn, conversationSourceCondition, conds)
	if len(agentAssistFilters) == 0 {
		return convWithAASubQuery, args
	}

	agentAssistState := agentAssistFilters[0].AgentAssistUsed
	switch agentAssistState {
	case analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON:
		// Filter conversations by their conversation time range (not message time), because
		// the question is "did this conversation have agent assistance?" — not "did assistance
		// happen at a specific message time?". max(has_agent_assistance) handles backward
		// compatibility: old per-conversation rows (message_id='') and new per-message rows
		// (message_id=uuid) coexist; if any row is true, the conversation is assisted.
		// The value true is hardcoded (not a placeholder) so args stays unchanged.
		// Rows of one conversation can carry different conversation times, such as the
		// placeholder end (end = start) and the real end, so the time comes from the
		// newest row, keeping one row per agent and conversation like the other
		// variants. The alias differs from the column so that conds reads the column.
		convWithAASubQuery = fmt.Sprintf(`
		%s AS (
				SELECT
					agent_user_id, conversation_id, latest_conversation_time AS %s
				FROM (
					SELECT
						agent_user_id, conversation_id,
						argMax(%s, update_time) AS latest_conversation_time,
						max(has_agent_assistance) AS max_has_assistance
					FROM
						conversation_with_labels_d
					WHERE
						agent_user_id <> ''
						AND %s
					GROUP BY agent_user_id, conversation_id
				)
				WHERE max_has_assistance = true
			)`, ConvWithAa, conversationTimeColumn, conversationTimeColumn, conds)

	case analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF:
		// For conversations WITHOUT assistance: start from conversation_d and anti-join
		// against any true row in conversation_with_labels_d. Scoped by conversation time
		// (not message time) for the same reason as the ON case. Backward compatible:
		// - Old data (message_id=''): false rows only → no true row → correctly included.
		// - New data: unassisted conversations have no rows → no true row → included.
		// - Mixed (old false + new true): true row exists → excluded correctly.
		// conds appears twice so args are duplicated: first for the anti-join subquery,
		// then for the outer WHERE on conversation_d.
		args = append(append([]any{}, args...), args...)
		convWithAASubQuery = fmt.Sprintf(`
		%s AS (
				SELECT
					agent_user_id, conversation_id, %s
				FROM conversation_d
				GLOBAL LEFT ANTI JOIN (
					SELECT DISTINCT conversation_id, agent_user_id
					FROM conversation_with_labels_d
					WHERE has_agent_assistance = true
					AND %s
				) AS assisted USING (conversation_id, agent_user_id)
				WHERE TRUE
					AND %s
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND %s
				ORDER BY update_time DESC
				LIMIT 1 BY agent_user_id, conversation_id
			)`, ConvWithAa, conversationTimeColumn, conds, conversationSourceCondition, conds)
	}
	return convWithAASubQuery, args
}

func buildConversationPopulationMomentFilterQuery(
	filter momentAnnotationFilter,
	index int,
	joinType string,
	allowMatchingStaleMetadataValues bool,
) (string, string, []any) {
	conditions, args := concatConditionsAndArgs(filter.conditions)
	resultConversationID := fmt.Sprintf("conversation_id_%d", index)
	if allowMatchingStaleMetadataValues {
		return fmt.Sprintf(`
			moment_annotation_filter_%d AS (
				SELECT DISTINCT
					conversation_id AS %s
				FROM
					%s
				WHERE
					%s
			),
		`, index, resultConversationID, filter.source.tableName(), conditions), fmt.Sprintf(`
			%s moment_annotation_filter_%d ON
				conversation_population_per_pair.conversation_id = moment_annotation_filter_%d.%s
		`, joinType, index, index, resultConversationID), args
	}

	conditionsNoMetadataValue, argsNoMetadataValue := concatConditionsAndArgsWithPredicate(filter.conditions, func(caa conditionAndArg) bool {
		for _, keyword := range momentAnnotationValueConditionKeywords() {
			if strings.Contains(caa.condition, keyword) {
				return true
			}
		}
		return false
	}, true /* concatWithAnd */)
	latestTimeColumn := filter.source.latestTimeColumn()
	return fmt.Sprintf(`
			t1_moment_annotation_filter_%d AS (
				SELECT
					-- Rename the column to avoid ambiguous column issue.
					conversation_id,
					MAX(%s) AS latest_create_time
				FROM
					%s
				WHERE
					%s
				GROUP BY conversation_id
			),
			t2_moment_annotation_filter_%d AS (
				SELECT
					conversation_id,
					%s
				FROM
					%s
				WHERE
					%s
			),
			moment_annotation_filter_%d AS (
				SELECT DISTINCT
					t1.conversation_id AS %s
				FROM
					t1_moment_annotation_filter_%d AS t1
				JOIN t2_moment_annotation_filter_%d AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_create_time = t2.%s
			),
		`, index, latestTimeColumn, filter.source.tableName(), conditionsNoMetadataValue,
			index, latestTimeColumn, filter.source.tableName(), conditions,
			index, resultConversationID, index, index, latestTimeColumn), fmt.Sprintf(`
			%s moment_annotation_filter_%d ON
				conversation_population_per_pair.conversation_id = moment_annotation_filter_%d.%s
		`, joinType, index, index, resultConversationID), append(argsNoMetadataValue, args...)
}

func parseConversationPopulationValueOrMissingFilters(
	timeRange *analyticspb.TimeRange,
	attribute *analyticspb.Attribute,
	timeColumn string,
) (*analyticspb.Attribute, []qaMomentValueOrMissingFilter, error) {
	if !*enableExcludedMoments || attribute == nil {
		return attribute, nil, nil
	}
	startTime, endTime, err := shared.ParseFilterByTimeRange(timeRange)
	if err != nil {
		return nil, nil, err
	}

	remainingAttribute := proto.Clone(attribute).(*analyticspb.Attribute)
	remainingAttribute.MomentGroups = nil
	valueOrMissingFilters := []qaMomentValueOrMissingFilter{}
	for _, momentGroup := range attribute.MomentGroups {
		valueOrMissingFilter, err := parseValueOrMissingMetadataMomentGroup(
			momentGroup,
			startTime,
			endTime,
			false, /* fetchFromMetadataView */
			timeColumn,
		)
		if err != nil {
			return nil, nil, err
		}
		if valueOrMissingFilter != nil {
			valueOrMissingFilters = append(valueOrMissingFilters, *valueOrMissingFilter)
			continue
		}
		remainingAttribute.MomentGroups = append(remainingAttribute.MomentGroups, momentGroup)
	}
	return remainingAttribute, valueOrMissingFilters, nil
}

func buildConversationPopulationSelect(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	momentConditionsAndArgsList, excludedMomentConditionsAndArgsList [][]conditionAndArg,
	valueOrMissingFilters []qaMomentValueOrMissingFilter,
	timeColumn string,
	agentAssistFiltersSubQuery string,
	agentAssistFiltersSubArgs []any,
	conversationSourceCondition string,
	allowMatchingStaleMetadataValues bool,
) (string, []any) {
	allArgs := []any{}

	// Build sub queries based on `moment_annotation_d` table.
	momentSubQueryList := []string{}
	momentJoinQueryList := []string{}
	momentIndex := 0
	for _, momentConditionsAndArgs := range momentConditionsAndArgsList {
		filter := momentAnnotationFilter{conditions: momentConditionsAndArgs, source: momentAnnotationFilterSourceRaw}
		subQuery, joinQuery, args := buildConversationPopulationMomentFilterQuery(
			filter,
			momentIndex,
			"INNER JOIN",
			allowMatchingStaleMetadataValues,
		)
		momentSubQueryList = append(momentSubQueryList, subQuery)
		momentJoinQueryList = append(momentJoinQueryList, joinQuery)
		allArgs = append(allArgs, args...)
		momentIndex++
	}

	selectedValueIndices := make([]int, len(valueOrMissingFilters))
	for filterIndex, filter := range valueOrMissingFilters {
		selectedValueIndices[filterIndex] = momentIndex
		subQuery, joinQuery, args := buildConversationPopulationMomentFilterQuery(
			filter.selectedValueFilter,
			momentIndex,
			"LEFT JOIN",
			allowMatchingStaleMetadataValues,
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
	for i, excludedMomentConditionsAndArgs := range excludedMomentConditionsAndArgsList {
		conditions, args := concatConditionsAndArgs(excludedMomentConditionsAndArgs)
		excludedMomentSubQueryList = append(excludedMomentSubQueryList, fmt.Sprintf(`
			moment_annotation_exclude_filter_%d AS (
				SELECT DISTINCT
					-- Rename the column to avoid ambiguous column issue.
					conversation_id AS conversation_id_exclude_%d
				FROM
					moment_annotation_d
				WHERE
					%s
			),
		`, i, i, conditions))
		allArgs = append(allArgs, args...)
		excludedMomentJoinQueryList = append(excludedMomentJoinQueryList, fmt.Sprintf(`
			LEFT JOIN moment_annotation_exclude_filter_%d ON
				conversation_population_per_pair.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
		`, i, i, i))
		excludedMomentWhereQueryList = append(excludedMomentWhereQueryList, fmt.Sprintf(`
			AND moment_annotation_exclude_filter_%d.conversation_id_exclude_%d = ''
		`, i, i))
	}
	valueOrMissingWhereQueryList := []string{}
	missingBaseIndex := len(excludedMomentConditionsAndArgsList)
	for filterIndex, filter := range valueOrMissingFilters {
		missingIndex := missingBaseIndex + filterIndex
		conditions, args := concatConditionsAndArgs(filter.existenceFilter.conditions)
		excludedMomentSubQueryList = append(excludedMomentSubQueryList, fmt.Sprintf(`
			moment_annotation_exclude_filter_%d AS (
				SELECT DISTINCT
					conversation_id AS conversation_id_exclude_%d
				FROM
					%s
				WHERE
					%s
			),
		`, missingIndex, missingIndex, filter.existenceFilter.source.tableName(), conditions))
		allArgs = append(allArgs, args...)
		excludedMomentJoinQueryList = append(excludedMomentJoinQueryList, fmt.Sprintf(`
			LEFT JOIN moment_annotation_exclude_filter_%d ON
				conversation_population_per_pair.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
		`, missingIndex, missingIndex, missingIndex))

		selectedValueIndex := selectedValueIndices[filterIndex]
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

	// Build conditions and args for conversation_stats query on `message_d` table.
	messageCondition, messageArgs := concatConditionsAndArgs(conditionsAndArgs[messageTable])
	allArgs = append(allArgs, agentAssistFiltersSubArgs...)
	allArgs = append(allArgs, messageArgs...)
	// Bucket time must come from the selected conversation/label row. Message
	// copies can have placeholder end times from before the conversation closed.
	return fmt.Sprintf(`
		WITH
		%s
		%s
		%s,
        %s AS (
			SELECT
				agent_user_id,
				conversation_id,
				MAX(handle_time_secs) AS aht_sec
			FROM
				message_d
			WHERE TRUE
				AND %s
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND %s	-- Additional filters
			GROUP BY agent_user_id, conversation_id
		),
		conversation_population_per_pair AS (
			SELECT *
			FROM %s JOIN %s ON %s.conversation_id = %s.conversation_id
				AND %s.agent_user_id = %s.agent_user_id
		)
		SELECT
			conversation_id, agent_user_id, %s AS conversation_time, aht_sec
		FROM
			conversation_population_per_pair
				%s
				%s
		-- WHERE is optional.
		%s`,
			momentSubQueries,
			excludedMomentSubQueries,
			agentAssistFiltersSubQuery,
			// group-by-conv cte
			ConvWithAht,
			conversationSourceCondition, messageCondition,
			// conversation_population_per_pair
			ConvWithAht, ConvWithAa,
			ConvWithAht, ConvWithAa,
			ConvWithAht, ConvWithAa,
			// main query
			// select
			timeColumn,
			// from
			momentJoinQueries, excludedMomentJoinQueries,
			// where
			excludedMomentWhereQueries),
		allArgs
}
