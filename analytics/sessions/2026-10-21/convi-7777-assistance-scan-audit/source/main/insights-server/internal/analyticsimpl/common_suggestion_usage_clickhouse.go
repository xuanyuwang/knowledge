package analyticsimpl

import "fmt"

// Keep all event types: legacy suggestion statistics include them in conversation/user denominators.
func buildSuggestionEventRowsQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (string, []any) {
	allArgs := []any{}

	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	aliasDict := map[string]string{
		"conversationEvent": "conversation_event_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, true /* isLastCTE */, true /* fetchFromMetadataView */)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	ceCondition, ceArgs := concatConditionsAndArgs(conditionsAndArgs[conversationEventTable])

	allArgs = append(allArgs, ceArgs...)

	if hasMetadataFiltering {
		return fmt.Sprintf(`
			WITH
			%s
			%s
			SELECT
				conversation_event_d.conversation_id AS conversation_id,
				conversation_event_d.agent_user_id AS agent_user_id,
				conversation_start_time, event_id, event_type
			FROM
				conversation_event_d
			-- Join moment annotation filters to include criteria
			%s
			-- Join moment annotation filters to exclude criteria
			%s
			WHERE
				agent_user_id <> ''
				AND %s -- Additional filters
				%s
			`, metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery, metadataFilters.MomentJoinQueries["conversationEvent"], metadataFilters.ExcludedMomentJoinQueries["conversationEvent"], ceCondition, metadataFilters.ExcludedMomentWhereQueries), allArgs
	} else {
		return fmt.Sprintf(`
			SELECT
				conversation_event_d.conversation_id AS conversation_id,
				conversation_event_d.agent_user_id AS agent_user_id,
				conversation_start_time, event_id, event_type
			FROM
				conversation_event_d
			WHERE
				agent_user_id <> ''
				AND %s -- Additional filters
			`, ceCondition), allArgs
	}
}
