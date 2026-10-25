package analyticsimpl

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/ext"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/util/errgroup"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type assistanceUsagePair struct {
	conversationID string
	agentUserID    string
}

type assistanceUsageEvidenceQuery struct {
	name, query   string
	args          []any
	eventsCluster bool
}

// Population membership owns request filters and time selection. Evidence rows
// can carry stale conversation times, so population membership uses only their
// conversation/agent keys. Feature-internal matching retains its existing keys.
func conversationAssistanceUsageEvidenceQueries(includeBotConversations bool) []assistanceUsageEvidenceQuery {
	conditions := map[ClickhouseTable][]conditionAndArg{}
	for _, table := range []ClickhouseTable{conversationTable, conversationEventTable, eventConversationEventTable, actionAnnotationTable, momentAnnotationTable} {
		conditions[table] = []conditionAndArg{{condition: "agent_user_id IN (SELECT user_id FROM assistance_usage_agents)"}}
	}
	project := func(query, predicate string) string {
		return fmt.Sprintf("SELECT DISTINCT conversation_id, agent_user_id FROM (%s) WHERE %s", query, predicate)
	}
	kb, kbArgs := buildKnowledgeBaseEventRowsQuery(conditions, nil, nil)
	suggestions, suggestionArgs := buildSuggestionEventRowsQuery(conditions, nil, nil)
	notes, noteArgs := buildNoteTakingUsageRowsQuery(conditions, nil, nil, false /* hasMetadataFiltering */)
	workflow, workflowArgs := buildGuidedWorkflowUsageRowsQuery(conditions, nil, nil)
	genAI, genAIArgs := buildKnowledgeAssistUsageRowsQuery(momentAnnotationTable, conditions[momentAnnotationTable], MetadataFilterResult{}, false /* hasMetadataFiltering */)
	summaryCTEs, _, summaryArgs := buildSummarizationUsageRowsQuery(conditions, MetadataFilterResult{}, false /* hasMetadataFiltering */)
	summary := fmt.Sprintf(`WITH %s
SELECT DISTINCT conversation_id, agent_user_id
FROM convo_with_valid_summarization
INNER JOIN convo_with_used_summarization
USING (conversation_id, conversation_start_time, agent_user_id)`, strings.TrimSuffix(strings.TrimSpace(summaryCTEs), ","))
	hints := buildHintUsageRowRelations(conditions, includeBotConversations, nil, nil, false /* useActionAnnotation */)
	behavioral, behavioralArgs := hints.behavioralFollowed.query("", nil)
	knowledge, knowledgeArgs := hints.knowledgeFollowed.query("", nil)
	hintQuery := project(behavioral, "(adherence_action_type = 1 AND adherence_action_adherence_type = 1) OR adherence_action_type = 21") + " UNION ALL " + project(knowledge, "action_type IN (13, 14)")
	queries := []assistanceUsageEvidenceQuery{
		{name: "KB search", query: project(kb, "event_type = 13"), args: kbArgs},
		{name: "Chat Suggestions", query: project(suggestions, "event_type IN (4, 5, 10)"), args: suggestionArgs},
		{name: "GenAI", query: project(genAI, "true"), args: genAIArgs},
		{name: "Summary", query: summary, args: summaryArgs},
		{name: "Hints", query: hintQuery, args: append(behavioralArgs, knowledgeArgs...)},
		{name: "Notes", query: project(notes, "event_type = 5 AND event_name = 'VoiceAgentAssist.NoteTaking.Click.CopyEntity'"), args: noteArgs, eventsCluster: true},
		{name: "Guided Workflow", query: project(workflow, "event_type = 4 AND doc_type = '2' AND guided_workflow_name <> ''"), args: workflowArgs, eventsCluster: true},
	}
	for i := range queries {
		// Reduce distributed feature rows before joining the eligible cohort.
		// Only the smaller agent list belongs in the source-table predicates.
		queries[i].query = fmt.Sprintf(`WITH assistance_usage_evidence AS (
SELECT conversation_id, agent_user_id FROM (%s)
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)`, queries[i].query)
	}
	return queries
}

func (a AnalyticsServiceImpl) readConversationAssistanceUsageEvidence(ctx context.Context, parentName string, population []conversationAssistanceUsagePopulationRow, includeBotConversations bool) (set.Set[assistanceUsagePair], error) {
	evidence := set.New[assistanceUsagePair]()
	if len(population) == 0 {
		return evidence, nil
	}
	parent, err := shared.ParseCustomerProfile(parentName)
	if err != nil {
		return nil, err
	}
	agentUserIDs := set.New[string]()
	for _, row := range population {
		agentUserIDs.Add(row.agentUserID)
	}
	selectedAgentUserIDs := agentUserIDs.Slice()
	queries := conversationAssistanceUsageEvidenceQueries(includeBotConversations)
	results := make([][]assistanceUsagePair, len(queries))
	group, queryContext := errgroup.WithContext(ctx, errgroup.WithLimit(3 /* n */))
	for i, query := range queries {
		group.Go(func() error {
			pairs, err := a.readConversationAssistanceUsageFeature(queryContext, parent, population, selectedAgentUserIDs, query)
			if err != nil {
				return fmt.Errorf("read %s assistance usage evidence: %w", query.name, err)
			}
			results[i] = pairs
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	for _, pairs := range results {
		evidence.AddAll(pairs...)
	}
	return evidence, nil
}

func (a AnalyticsServiceImpl) readConversationAssistanceUsageFeature(ctx context.Context, parent *shared.CustomerProfile, population []conversationAssistanceUsagePopulationRow, agentUserIDs []string, query assistanceUsageEvidenceQuery) ([]assistanceUsagePair, error) {
	// Each concurrent query owns its external table and its column buffers.
	table, err := ext.NewTable("assistance_usage_population", ext.Column("conversation_id", "String"), ext.Column("agent_user_id", "String"))
	if err != nil {
		return nil, fmt.Errorf("create eligible-pair table: %w", err)
	}
	for _, row := range population {
		if err := table.Append(row.conversationID, row.agentUserID); err != nil {
			return nil, fmt.Errorf("append eligible pair: %w", err)
		}
	}
	agentTable, err := buildUsersExtTable(agentUserIDs, "assistance_usage_agents", "user_id")
	if err != nil {
		return nil, fmt.Errorf("create evidence-agent table: %w", err)
	}
	connection := a.getConversationsClickhouseConn
	if query.eventsCluster {
		connection = a.getEventsClickhouseConn
	}
	conn, err := connection(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("resolve evidence connection for %s: %w", parent, err)
	}
	ctx = attachExtTablesToContext(ctx, table, agentTable)
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query.query, query.args...)
	if err != nil {
		return nil, fmt.Errorf("query feature rows: %w", err)
	}
	// fail-open: Closing rows cannot change the read result.
	defer rows.Close()
	var pairs []assistanceUsagePair
	for rows.Next() {
		var pair assistanceUsagePair
		if err := rows.Scan(&pair.conversationID, &pair.agentUserID); err != nil {
			return nil, fmt.Errorf("scan feature row: %w", err)
		}
		pairs = append(pairs, pair)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feature rows: %w", err)
	}
	return pairs, nil
}
