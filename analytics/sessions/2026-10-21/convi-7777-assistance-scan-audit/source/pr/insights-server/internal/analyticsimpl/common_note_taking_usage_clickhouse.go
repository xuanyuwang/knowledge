package analyticsimpl

import "fmt"

// buildNoteTakingUsageRowsQuery preserves the distinct event projection and conversation-only join.
// Rows with unrelated event types remain in the relation because they contribute to legacy denominators.
func buildNoteTakingUsageRowsQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
	hasMetadataFiltering bool,
) (string, []any) {
	allArgs := []any{}

	aliasDict := map[string]string{
		"conversation":      "conversation_d",
		"conversationEvent": "conversation_event_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false /* isLastCTE */, true /* fetchFromMetadataView */)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	convConditions, convArgs := concatConditionsAndArgs(conditionsAndArgs[conversationTable])
	allArgs = append(allArgs, convArgs...)
	convCondition := convConditions
	if convCondition != "" {
		convCondition = "AND " + convCondition
	}

	ceConditions, ceArgs := concatConditionsAndArgs(conditionsAndArgs[eventConversationEventTable])
	allArgs = append(allArgs, ceArgs...)
	ceCondition := ceConditions
	if ceCondition != "" {
		ceCondition = "AND " + ceCondition
	}

	const (
		noteTakingDataTypeColumn = "`payload.used_note_taking.data_type`"
	)

	if hasMetadataFiltering {
		return fmt.Sprintf(`
			WITH
			%s
			%s
			conversations AS (
				SELECT DISTINCT
					conversation_id
				FROM
					conversation_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					agent_user_id <> ''
					%s
					%s -- Additional filters
			),
			conversation_events AS (
				SELECT DISTINCT
					agent_user_id,
					conversation_id,
					conversation_start_time,
					event_type,
					event_id,
					%s AS note_taking_data_type
				FROM
					conversation_event_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					agent_user_id <> ''
					%s
					%s -- Additional filters
			)
			SELECT
				agent_user_id AS agent_user_id,
				conversation_id AS conversation_id,
				conversation_start_time AS conversation_start_time,
				event_type AS event_type,
				event_id AS event_id,
				note_taking_data_type AS note_taking_data_type
			FROM
				conversations
				JOIN conversation_events USING (conversation_id)
			`,
			metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
			metadataFilters.MomentJoinQueries["conversation"], metadataFilters.ExcludedMomentJoinQueries["conversation"], convCondition, metadataFilters.ExcludedMomentWhereQueries,
			noteTakingDataTypeColumn, metadataFilters.MomentJoinQueries["conversationEvent"], metadataFilters.ExcludedMomentJoinQueries["conversationEvent"], ceCondition, metadataFilters.ExcludedMomentWhereQueries), allArgs
	} else {
		return fmt.Sprintf(`
			WITH
			conversations AS (
				SELECT DISTINCT
					conversation_id
				FROM
					conversation_d
				WHERE
					agent_user_id <> ''
					%s -- Additional filters
			),
			conversation_events AS (
				SELECT DISTINCT
					agent_user_id,
					conversation_id,
					conversation_start_time,
					event_type,
					event_name,
					event_id
				FROM
					conversation_event_d
				WHERE
					agent_user_id <> ''
					%s -- Additional filters
			)
			SELECT
				agent_user_id AS agent_user_id,
				conversation_id AS conversation_id,
				conversation_start_time AS conversation_start_time,
				event_type AS event_type,
				event_name AS event_name,
				event_id AS event_id
			FROM
				conversations
				JOIN conversation_events USING (conversation_id)
			`,
			convCondition, ceCondition), allArgs
	}
}
