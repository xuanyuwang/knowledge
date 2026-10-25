package analyticsimpl

import "fmt"

// buildKnowledgeAssistUsageRowsQuery preserves annotation rows and session IDs;
// distinct-session counting belongs to the consumer's requested grouping.
func buildKnowledgeAssistUsageRowsQuery(
	table ClickhouseTable,
	conditionsAndArgs []conditionAndArg,
	metadataFilters MetadataFilterResult,
	hasMetadataFiltering bool,
) (string, []any) {
	tableName, aliasKey, featureCondition := "message_d", "message", ""
	featureColumns := "message_d.message_id AS message_id"
	switch table {
	case actionAnnotationTable:
		tableName, aliasKey = "action_annotation_d", "actionAnnotation"
		featureColumns = `action_annotation_d.action_annotation_id AS action_annotation_id,
    action_annotation_d.action_type AS action_type,
    action_annotation_d.knowledge_payload_knowledge_session_id AS knowledge_payload_knowledge_session_id`
		featureCondition = `AND knowledge_payload_knowledge_session_id <> ''
     AND action_type in (22, 24)  -- (RETRIEVAL_QUESTION, RETRIEVAL_ARTICLE)
`
	case momentAnnotationTable:
		tableName, aliasKey = "moment_annotation_d", "momentAnnotation"
		featureColumns = `moment_annotation_d.moment_annotation_id AS moment_annotation_id,
    moment_annotation_d.moment_type AS moment_type,
    moment_annotation_d.knowledge_payload_knowledge_session_id AS knowledge_payload_knowledge_session_id`
		featureCondition = `AND knowledge_payload_knowledge_session_id <> ''
     AND moment_type = 38  -- KA_GEN_AI_REPLY_USED
`
	}
	// Explicit aliases keep outer grouping columns visible across metadata joins.
	columns := fmt.Sprintf(`%[1]s.conversation_id AS conversation_id,
    %[1]s.conversation_start_time AS conversation_start_time,
    %[1]s.agent_user_id AS agent_user_id,
    %[2]s`, tableName, featureColumns)
	condition, args := concatConditionsAndArgs(conditionsAndArgs)
	if hasMetadataFiltering {
		return fmt.Sprintf(`SELECT %s
   FROM %s
   -- Join moment annotation filters to include criteria
   %s
   -- Join moment annotation filters to exclude criteria
   %s
   WHERE conversation_source IN (0, 8)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    %s
    AND %s -- Additional filters
    %s`, columns, tableName, metadataFilters.MomentJoinQueries[aliasKey], metadataFilters.ExcludedMomentJoinQueries[aliasKey], featureCondition, condition, metadataFilters.ExcludedMomentWhereQueries), args
	}
	return fmt.Sprintf(`SELECT %s FROM %s
   WHERE conversation_source IN (0, 8)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    %s
    AND %s -- Additional filters
`, columns, tableName, featureCondition, condition), args
}
