package analyticsimpl

import (
	"fmt"
	"strings"

	"github.com/cresta/shared-go/fn"
)

const (
	momentAnnotationTableAlias  = "momentAnnotation"
	actionAnnotationTableAlias  = "actionAnnotation"
	conversationEventTableAlias = "conversationEvent"
)

// hintUsageRowRelation retains source-row grain, including duplicate physical
// rows. The legacy statistics query applies its own distinct counts and groups.
type hintUsageRowRelation struct {
	table         string
	columns       []string
	joinsAndWhere string
	args          []any
	metadataCTEs  string
	metadataArgs  []any
}

// query with an empty projection returns standalone raw rows and metadata CTEs.
// An explicit projection shares metadata CTEs with the enclosing aggregate query.
// An action ID can occur in multiple rows, conversations, or agents; consumers
// must choose their own aggregation grain instead of treating rows as hint uses.
func (r hintUsageRowRelation) query(projection string, groupByKeys []string) (string, []any) {
	standaloneRows := projection == ""
	if standaloneRows {
		// ClickHouse retains table-qualified output names after joins without aliases.
		columns := fn.Map(r.columns, func(column string) string { return r.table + "." + column + " AS " + column })
		projection = strings.Join(columns, ", ")
	}
	query := fmt.Sprintf("SELECT %s FROM %s %s", projection, r.table, r.joinsAndWhere)
	if len(groupByKeys) > 0 {
		query += "\nGROUP BY " + concatKeys(groupByKeys)
	}
	if !standaloneRows {
		return query, r.args
	}
	if metadata := strings.TrimSuffix(strings.TrimSpace(r.metadataCTEs), ","); metadata != "" {
		query = "WITH " + metadata + "\n" + query
	}
	args := append([]any{}, r.metadataArgs...)
	return query, append(args, r.args...)
}

type hintUsageRowRelations struct {
	behavioralSent     hintUsageRowRelation
	nonBehavioralSent  hintUsageRowRelation
	behavioralFollowed hintUsageRowRelation
	knowledgeFollowed  hintUsageRowRelation
	metadata           MetadataFilterResult
}

func buildHintUsageRowRelations(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	includeBotConversations bool,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
	useActionAnnotation bool,
) hintUsageRowRelations {
	metadata := generateMetadataFilters(momentConditionsAndArgsList, excludedMomentConditionsAndArgs, map[string]string{
		momentAnnotationTableAlias:  "moment_annotation_d",
		actionAnnotationTableAlias:  "action_annotation_d",
		conversationEventTableAlias: "conversation_event_d",
	}, false /* isLastCTE */, true /* fetchFromMetadataView */)
	momentCondition, momentArgs := concatConditionsAndArgs(conditionsAndArgs[momentAnnotationTable])
	actionCondition, actionArgs := concatConditionsAndArgs(conditionsAndArgs[actionAnnotationTable])
	eventCondition, eventArgs := concatConditionsAndArgs(conditionsAndArgs[conversationEventTable])
	botSource := ""
	if includeBotConversations {
		botSource = crestaBotSourceStr
	}
	joins := func(alias string) string {
		if len(metadata.MomentJoinQueries) == 0 && len(metadata.ExcludedMomentJoinQueries) == 0 {
			return ""
		}
		return fmt.Sprintf(`-- Join moment annotation filters to include criteria
 %s
 -- Join moment annotation filters to exclude criteria
 %s`, metadata.MomentJoinQueries[alias], metadata.ExcludedMomentJoinQueries[alias])
	}
	momentColumns := []string{
		"moment_annotation_id",
		"create_time",
		"update_time",
		"conversation_id",
		"agent_user_id",
		"conversation_start_time",
		"behavior_id",
		"policy_id",
		"adherence_action_annotation_id",
		"adherence_type",
		"adherence_action_type",
		"adherence_action_adherence_type",
		"adherence_action_policy_id",
	}
	actionColumns := []string{
		"create_time",
		"update_time",
		"conversation_id",
		"agent_user_id",
		"conversation_start_time",
		"policy_id",
		"action_annotation_id",
		"action_type",
		"adherence_type",
	}
	eventColumns := []string{
		"event_id",
		"create_time",
		"update_time",
		"conversation_id",
		"agent_user_id",
		"conversation_start_time",
		"action_policy_id",
		"action_annotation_id",
		"event_type",
		"action_type",
		"action_adherence_type",
	}
	relation := func(table string, columns []string, body string, args []any) hintUsageRowRelation {
		return hintUsageRowRelation{
			table:         table,
			columns:       columns,
			joinsAndWhere: body,
			args:          args,
			metadataCTEs:  metadata.MomentSubQuery + metadata.ExcludedMomentSubQuery,
			metadataArgs:  metadata.AllArgs,
		}
	}
	behavioralSent := relation("moment_annotation_d", momentColumns, fmt.Sprintf(`%s
 WHERE adherence_type in (2, 3)
 AND adherence_action_annotation_id <> ''
 AND (conversation_source in (0, 8%s) OR conversation_source IS NULL)
 AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND %s -- Additional filters
 %s`, joins(momentAnnotationTableAlias), botSource, momentCondition, metadata.ExcludedMomentWhereQueries), momentArgs)
	if useActionAnnotation {
		behavioralSent = relation("action_annotation_d", actionColumns, fmt.Sprintf(`%s
 WHERE action_annotation_id GLOBAL IN (
 SELECT adherence_action_annotation_id
 FROM moment_annotation_d
 WHERE adherence_type IN (2, 3)
 AND adherence_action_annotation_id <> ''
 AND %s
 )
 AND (conversation_source in (0, 8%s) OR conversation_source IS NULL)
 AND policy_source_type = 1
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND %s -- Additional filters
 %s`, joins(actionAnnotationTableAlias), momentCondition, botSource, actionCondition, metadata.ExcludedMomentWhereQueries), append(append([]any{}, momentArgs...), actionArgs...))
	}
	nonBehavioralSent := relation("action_annotation_d", actionColumns, fmt.Sprintf(`%s
 WHERE action_type in (1,6,13,14) -- 1: regular+behavioral hint, 6: manager alert, 13: GW hint, 14: KB hint
 AND (conversation_source in (0, 8%s) OR conversation_source IS NULL)
 AND policy_source_type=1
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND adherence_type = 0
 AND %s -- Additional filters
 %s`, joins(actionAnnotationTableAlias), botSource, actionCondition, metadata.ExcludedMomentWhereQueries), actionArgs)
	behavioralFollowed := relation("moment_annotation_d", momentColumns, fmt.Sprintf(`%s
 %s
 WHERE adherence_type = 2
 AND adherence_action_annotation_id <> ''
 AND (conversation_source in (0, 8%s) OR conversation_source IS NULL)
 AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND %s -- Additional filters
 %s`, metadata.MomentJoinQueries[momentAnnotationTableAlias], metadata.ExcludedMomentJoinQueries[momentAnnotationTableAlias], botSource, momentCondition, metadata.ExcludedMomentWhereQueries), momentArgs)
	knowledgeFollowed := relation("conversation_event_d", eventColumns, fmt.Sprintf(`%s
 %s
 WHERE event_type = 9
 AND (conversation_source in (0, 8%s) OR conversation_source IS NULL)
 AND action_policy_source_type=1
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND %s -- Additional filters
 %s`, metadata.MomentJoinQueries[conversationEventTableAlias], metadata.ExcludedMomentJoinQueries[conversationEventTableAlias], botSource, eventCondition, metadata.ExcludedMomentWhereQueries), eventArgs)
	return hintUsageRowRelations{
		behavioralSent:     behavioralSent,
		nonBehavioralSent:  nonBehavioralSent,
		behavioralFollowed: behavioralFollowed,
		knowledgeFollowed:  knowledgeFollowed,
		metadata:           metadata,
	}
}
