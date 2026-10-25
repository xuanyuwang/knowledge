package analyticsimpl

import "fmt"

// buildGuidedWorkflowUsageRowsQuery preserves the distinct event projection and conversation-only join.
// Rows with unrelated event types remain in the relation because they contribute to legacy denominators.
func buildGuidedWorkflowUsageRowsQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
) (string, []any) {
	allArgs := []any{}

	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	aliasDict := map[string]string{
		"conversation":      "conversation_d",
		"conversationEvent": "conversation_event_d",
	}
	metadataFilters := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, aliasDict, false /* isLastCTE */, true /* fetchFromMetadataView */)
	allArgs = append(allArgs, metadataFilters.AllArgs...)

	convConditions, convArgs := concatConditionsAndArgs(conditionsAndArgs[conversationTable])
	convCondition := convConditions
	if convCondition != "" {
		convCondition = "AND " + convCondition
	}

	ceConditions, ceArgs := concatConditionsAndArgs(conditionsAndArgs[eventConversationEventTable])
	ceCondition := ceConditions
	if ceCondition != "" {
		ceCondition = "AND " + ceCondition
	}
	allArgs = append(allArgs, convArgs...)
	allArgs = append(allArgs, ceArgs...)

	const (
		usedKBDocument    = "`payload.used_kb_document.doc_id`"
		docType           = "`payload.used_kb_document.doc_type`"
		sessionMetadataID = "`payload.used_kb_document.session_id`"
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
			gw_conversation_events AS (
				SELECT DISTINCT
					agent_user_id,
					conversation_id,
					conversation_start_time,
					event_type,
					%s AS guided_workflow_name,
					%s AS doc_type,
					%s AS session_metadata_id
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
				guided_workflow_name AS guided_workflow_name,
				doc_type AS doc_type,
				session_metadata_id AS session_metadata_id
			FROM
				conversations JOIN gw_conversation_events USING (conversation_id)
			`,
				// Metadata filters
				metadataFilters.MomentSubQuery, metadataFilters.ExcludedMomentSubQuery,
				// conversations
				metadataFilters.MomentJoinQueries["conversation"], metadataFilters.ExcludedMomentJoinQueries["conversation"], convCondition, metadataFilters.ExcludedMomentWhereQueries,
				// gw_conversation_events
				usedKBDocument, docType, sessionMetadataID,
				metadataFilters.MomentJoinQueries["conversationEvent"], metadataFilters.ExcludedMomentJoinQueries["conversationEvent"], ceCondition, metadataFilters.ExcludedMomentWhereQueries),
			allArgs
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
			gw_conversation_events AS (
				SELECT DISTINCT
					agent_user_id,
					conversation_id,
					conversation_start_time,
					event_type,
					event_name,
					JSONExtractString(
						JSONExtractString(
							toJSONString(event_payload),
							'usedKbDocument'),
						'document') AS guided_workflow_name,
					JSONExtractString(
						JSONExtractString(
							toJSONString(event_payload),
							'usedKbDocument'),
						'docType') AS doc_type,
					JSONExtractString(
						JSONExtractString(
							JSONExtractString(
								toJSONString(event_payload),
								'usedKbDocument'),
							'sessionMetadata'),
						'sessionId') AS session_metadata_id
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
				guided_workflow_name AS guided_workflow_name,
				doc_type AS doc_type,
				session_metadata_id AS session_metadata_id
			FROM
				conversations JOIN gw_conversation_events USING (conversation_id)
			`,
				// conversations
				convCondition,
				// gw_conversation_events
				ceCondition),
			allArgs
	}
}
