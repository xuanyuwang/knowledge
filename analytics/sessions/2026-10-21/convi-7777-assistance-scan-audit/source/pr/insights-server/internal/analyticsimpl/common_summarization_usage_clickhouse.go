package analyticsimpl

import "fmt"

// buildSummarizationUsageRowsQuery keeps generated and copied summaries at
// (conversation_id, conversation_start_time, agent_user_id) grain. Its LEFT JOIN
// relation admits a copy only when valid generation matches all three keys.
func buildSummarizationUsageRowsQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	metadataFilters MetadataFilterResult,
	hasMetadataFiltering bool,
) (ctes, relation string, args []any) {
	actionCondition, actionArgs := concatConditionsAndArgs(conditionsAndArgs[actionAnnotationTable])
	eventCondition, eventArgs := concatConditionsAndArgs(conditionsAndArgs[conversationEventTable])
	args = append(actionArgs, eventArgs...)
	relation = `convo_with_valid_summarization
 LEFT JOIN convo_with_used_summarization USING (conversation_id, conversation_start_time, agent_user_id)`
	if hasMetadataFiltering {
		return fmt.Sprintf(`			convo_with_valid_summarization AS (
				SELECT DISTINCT
					conversation_id, conversation_start_time, agent_user_id
				FROM
					action_annotation_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					agent_user_id <> ''
					AND summarization_payload_is_valid_summary = 1
					AND %s -- Additional filters
					%s
			),
			convo_with_used_summarization AS (
				SELECT DISTINCT
					conversation_id, conversation_start_time, agent_user_id
				FROM
					conversation_event_d
				-- Join moment annotation filters to include criteria
				%s
				-- Join moment annotation filters to exclude criteria
				%s
				WHERE
					agent_user_id <> ''
					AND event_type = 16 -- 16: USED_SUMMARIZATION
					AND summarization_payload_data_type IN (1, 2)  -- 1: CopyAllNotes, 2: CopySummarizationNote
					AND %s -- Additional filters
					%s
			),
`,
			metadataFilters.MomentJoinQueries["actionAnnotation"], metadataFilters.ExcludedMomentJoinQueries["actionAnnotation"], actionCondition, metadataFilters.ExcludedMomentWhereQueries,
			metadataFilters.MomentJoinQueries["conversationEvent"], metadataFilters.ExcludedMomentJoinQueries["conversationEvent"], eventCondition, metadataFilters.ExcludedMomentWhereQueries), relation, args
	}
	return fmt.Sprintf(`			convo_with_valid_summarization AS (
				SELECT DISTINCT
					conversation_id, conversation_start_time, agent_user_id
				FROM
					action_annotation_d
				WHERE
					agent_user_id <> ''
					AND summarization_payload_is_valid_summary = 1
					AND %s -- Additional filters
			),
			convo_with_used_summarization AS (
				SELECT DISTINCT
					conversation_id, conversation_start_time, agent_user_id
				FROM
					conversation_event_d
				WHERE
					agent_user_id <> ''
					AND event_type = 16 -- 16: USED_SUMMARIZATION
					AND summarization_payload_data_type IN (1, 2)  -- 1: CopyAllNotes, 2: CopySummarizationNote
					AND %s -- Additional filters
			),
`, actionCondition, eventCondition), relation, args
}
