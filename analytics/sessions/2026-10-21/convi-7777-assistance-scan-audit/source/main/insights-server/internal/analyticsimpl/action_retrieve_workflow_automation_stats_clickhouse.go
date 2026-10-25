package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	workflowpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/workflowautomation"
	"github.com/cresta/shared-go/fn"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type workflowAutomationStatsRow struct {
	groupKey                                                                    string
	isBucket                                                                    uint8
	bucketMillis                                                                int64
	conversationCount, conversationsWithTriggerCount, agentCount, agentDayCount uint64
	sourceCounts                                                                [4]uint64
	ignoredCount                                                                *int64
}

func (row workflowAutomationStatsRow) stats() *analyticspb.AggregatedWorkflowAutomationStats {
	stats := &analyticspb.AggregatedWorkflowAutomationStats{
		TotalConversationCount:            int64(row.conversationCount),
		TotalConversationWithTriggerCount: int64(row.conversationsWithTriggerCount),
		TotalIgnoredConversationCount:     row.ignoredCount,
		TotalAgentCount:                   int64(row.agentCount),
		TotalActiveAgentDayCount:          int64(row.agentDayCount),
	}
	for source, count := range row.sourceCounts {
		stats.TotalWorkflowAutomationTriggerCount += int64(count)
		stats.TriggerSourceStats = append(stats.TriggerSourceStats, &analyticspb.WorkflowAutomationTriggerSourceStats{
			TriggerSource:                       workflowpb.WorkflowAutomationRun_TriggerSource(source),
			TotalWorkflowAutomationTriggerCount: int64(count),
		})
	}
	if stats.TotalConversationCount > 0 {
		stats.WorkflowAutomationUsageRatio = proto.Float64(float64(stats.TotalConversationWithTriggerCount) / float64(stats.TotalConversationCount))
		stats.AverageWorkflowAutomationTriggerCountPerConversation = proto.Float64(float64(stats.TotalWorkflowAutomationTriggerCount) / float64(stats.TotalConversationCount))
	}
	if stats.TotalAgentCount > 0 {
		stats.AverageWorkflowAutomationTriggerCountPerAgent = proto.Float64(float64(stats.TotalWorkflowAutomationTriggerCount) / float64(stats.TotalAgentCount))
	}
	if stats.TotalActiveAgentDayCount > 0 {
		stats.AverageWorkflowAutomationTriggerCountPerActiveAgentDay = proto.Float64(float64(stats.TotalWorkflowAutomationTriggerCount) / float64(stats.TotalActiveAgentDayCount))
	}
	return stats
}

func (a AnalyticsServiceImpl) readWorkflowAutomationStats(ctx context.Context, req *analyticspb.RetrieveWorkflowAutomationStatsRequest, selectors []workflowAutomationSelector, memberships map[string][]string, useExternalUsers bool) ([]workflowAutomationStatsRow, error) {
	parent, err := shared.ParseCustomerProfile(req.GetParent())
	if err != nil {
		return nil, err
	}
	conditions, included, excluded, usersTable, err := parseClickhouseFilter(ctx, req.GetFilterByTimeRange(), req.GetFilterByAttribute(), []ClickhouseTable{conversationTable}, useExternalUsers)
	if err != nil {
		return nil, err
	}
	query, args := workflowAutomationStatsQuery(req, parent.CustomerID, parent.ProfileID, conditions[conversationTable], included, excluded, selectors, memberships)
	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("connecting to workflow automation conversation analytics: %w", err)
	}
	return queryWorkflowAutomationStats(attachExtTablesToContext(ctx, usersTable), conn, query, args)
}

func queryWorkflowAutomationStats(ctx context.Context, conn driver.Conn, query string, args []any) ([]workflowAutomationStatsRow, error) {
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying workflow automation analytics: %w", err)
	}
	// fail-open: preserve the query or scan error; rows.Err reports close errors during iteration.
	defer rows.Close()
	results := []workflowAutomationStatsRow{}
	for rows.Next() {
		row := workflowAutomationStatsRow{}
		if err := rows.Scan(&row.groupKey, &row.isBucket, &row.bucketMillis, &row.conversationCount, &row.conversationsWithTriggerCount, &row.ignoredCount, &row.agentCount, &row.agentDayCount, &row.sourceCounts[0], &row.sourceCounts[1], &row.sourceCounts[2], &row.sourceCounts[3]); err != nil {
			return nil, fmt.Errorf("scanning workflow automation analytics: %w", err)
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading workflow automation analytics: %w", err)
	}
	return results, nil
}

func workflowAutomationSelectorSQL(selectors []workflowAutomationSelector, definitionColumn, versionColumn string) (string, []any) {
	if len(selectors) == 0 {
		return TrueString, nil
	}
	conditions := make([]string, 0, len(selectors))
	args := []any{}
	for _, selector := range selectors {
		condition := definitionColumn + " = ?"
		args = append(args, selector.definitionID)
		if len(selector.versions) > 0 {
			condition += " AND " + versionColumn + " IN (?)"
			args = append(args, selector.versions)
		}
		conditions = append(conditions, "("+condition+")")
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func workflowAutomationStatsQuery(req *analyticspb.RetrieveWorkflowAutomationStatsRequest, customerID, profileID string, conditions []conditionAndArg, included, excluded [][]conditionAndArg, selectors []workflowAutomationSelector, memberships map[string][]string) (string, []any) {
	metadata, metadataCondition := workflowAutomationMetadataFilters(included, excluded, customerID, profileID)
	populationFilter, populationArgs := concatConditionsAndArgs(conditions)
	if populationFilter != "" {
		populationFilter = " AND " + populationFilter
	}
	triggerFilter, triggerArgs := workflowAutomationSelectorSQL(selectors, "definition_id", "definition_version")
	definitionSelectors := fn.Map(selectors, func(selector workflowAutomationSelector) workflowAutomationSelector {
		return workflowAutomationSelector{definitionID: selector.definitionID}
	})
	hintFilter, hintArgs := workflowAutomationSelectorSQL(definitionSelectors, "`payload.workflow_automation_hint.workflow_automation_definition_id`", "")
	ignoredExpression := "CAST(countIf(has_hint AND trigger_count=0) AS Nullable(Int64))"
	if !workflowAutomationHintsAvailable(selectors) {
		ignoredExpression = "CAST(NULL AS Nullable(Int64))"
	}
	agentIDs := make([]string, 0, len(memberships))
	for agentID := range memberships {
		agentIDs = append(agentIDs, agentID)
	}
	sort.Strings(agentIDs)
	groupKeys := make([][]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		groupKeys = append(groupKeys, memberships[agentID])
	}
	// Cast empty maps explicitly so ungrouped queries have stable ClickHouse types.
	args := make([]any, 0, 13+len(metadata.AllArgs)+len(populationArgs)+len(triggerArgs)+len(hintArgs))
	args = append(args, agentIDs, groupKeys, workflowAutomationTimeZone(req))
	args = append(args, metadata.AllArgs...)
	args = append(args, customerID, profileID)
	args = append(args, populationArgs...)
	start := req.GetFilterByTimeRange().GetStartTimestamp().AsTime()
	end := req.GetFilterByTimeRange().GetEndTimestamp().AsTime()
	args = append(args, customerID, profileID, start, end)
	args = append(args, triggerArgs...)
	args = append(args, customerID, profileID, start, end)
	args = append(args, hintArgs...)
	frequency := "day"
	switch req.GetFrequency() {
	case analyticspb.Frequency_WEEKLY:
		frequency = weekFrequency
	case analyticspb.Frequency_MONTHLY:
		frequency = "month"
	}
	bucketExpression := "dateTrunc('" + frequency + "', conversation_start_time, timezone)"
	if req.GetFrequency() == analyticspb.Frequency_WEEKLY {
		bias := (8 - int(req.GetMetadata().GetStartDayOfWeek())) % 7
		bucketExpression = fmt.Sprintf("subtractDays(dateTrunc('week', addDays(conversation_start_time, %d), timezone), %d)", bias, bias)
	}
	query := fmt.Sprintf(`WITH
 mapFromArrays(CAST(? AS Array(String)), CAST(? AS Array(Array(String)))) AS memberships,
 ? AS timezone,
 %s
 %s
 filtered_conversations AS (
  SELECT conversation_d.conversation_id,conversation_d.agent_user_id,conversation_d.conversation_start_time,conversation_d.update_time
  FROM conversation_d
  WHERE conversation_d.customer_id = ? AND conversation_d.profile_id = ? %s %s
 ),
 population AS (
  SELECT conversation_id,argMax(agent_user_id,update_time) AS agent_user_id,
   argMax(conversation_start_time,update_time) AS conversation_start_time
  FROM filtered_conversations GROUP BY conversation_id
 ),
 events AS (
  SELECT conversation_id,event_id,
   argMax(tuple(event_type,
    `+"`payload.workflow_automation.workflow_automation_run_id`"+`,
    `+"`payload.workflow_automation.workflow_automation_definition_id`"+`,
    `+"`payload.workflow_automation.workflow_automation_definition_version`"+`,
    `+"`payload.workflow_automation.trigger_source`"+`), update_time) AS event,
   max(update_time) AS updated_at
  FROM conversation_event_d
  WHERE customer_id = ? AND profile_id = ? AND event_type IN (40,41,42,43)
   AND conversation_start_time >= ? AND conversation_start_time < ?
   AND conversation_id GLOBAL IN (SELECT conversation_id FROM population)
  GROUP BY conversation_id,event_id
 ),
 runs AS (
  SELECT conversation_id,event.2 AS run_id,
   argMaxIf(tuple(event.3,event.4),updated_at,event.1 IN (41,42,43) AND event.4 > 0) AS definition
  FROM events WHERE event.2 != '' GROUP BY conversation_id,run_id
 ),
 triggers AS (
  SELECT events.conversation_id AS conversation_id,
   if(runs.definition.1 != '',runs.definition.1,events.event.3) AS definition_id,
   if(runs.definition.2 > 0,runs.definition.2,events.event.4) AS definition_version,
   if(events.event.5 IN (1,2,3),events.event.5,0) AS source
  FROM events GLOBAL LEFT JOIN runs ON events.conversation_id=runs.conversation_id AND events.event.2=runs.run_id
  WHERE events.event.1=40
 ),
 usage AS (
  SELECT conversation_id,count() AS trigger_count,
   countIf(source=0) AS unspecified_count,countIf(source=1) AS manual_count,
   countIf(source=2) AS opera_count,countIf(source=3) AS opera_hint_count
  FROM triggers WHERE %s GROUP BY conversation_id
 ),
 hints AS (
  SELECT DISTINCT conversation_id,toUInt8(1) AS hint_present FROM action_annotation_d
  WHERE customer_id = ? AND profile_id = ? AND action_type=37
   AND conversation_start_time >= ? AND conversation_start_time < ? AND %s
   AND conversation_id GLOBAL IN (SELECT conversation_id FROM population)
 ),
 facts AS (
  SELECT population.conversation_id AS conversation_id,population.agent_user_id AS agent_user_id,
   population.conversation_start_time AS conversation_start_time,
   coalesce(usage.trigger_count,0) AS trigger_count,
   coalesce(usage.unspecified_count,0) AS unspecified_count,coalesce(usage.manual_count,0) AS manual_count,
   coalesce(usage.opera_count,0) AS opera_count,coalesce(usage.opera_hint_count,0) AS opera_hint_count,
   coalesce(hints.hint_present,0) AS has_hint
  FROM population GLOBAL LEFT JOIN usage ON population.conversation_id = usage.conversation_id
   GLOBAL LEFT JOIN hints ON population.conversation_id = hints.conversation_id
 )
 SELECT group_key,period.1 AS is_bucket,period.2 AS bucket_millis,
  count(),countIf(trigger_count>0),%s,
  uniqExactIf(agent_user_id,agent_user_id!=''),
  uniqExactIf(tuple(agent_user_id,toDate(conversation_start_time,timezone)),agent_user_id!=''),
  sum(unspecified_count),sum(manual_count),sum(opera_count),sum(opera_hint_count)
 FROM (SELECT *,arrayJoin(arrayConcat([''],memberships[agent_user_id])) AS group_key FROM facts)
 ARRAY JOIN [tuple(toUInt8(0),toInt64(0)),tuple(toUInt8(1),toUnixTimestamp64Milli(toDateTime64(%s,3,timezone)))] AS period
 GROUP BY group_key,is_bucket,bucket_millis
 ORDER BY group_key,is_bucket,bucket_millis`, metadata.MomentSubQuery, metadata.ExcludedMomentSubQuery, populationFilter, metadataCondition, triggerFilter, hintFilter, ignoredExpression, bucketExpression)
	return query, args
}

// Use semi-joins for every include/exclude group. This preserves conversation
// cardinality and does not depend on the connection's join_use_nulls setting.
func workflowAutomationMetadataFilters(included, excluded [][]conditionAndArg, customerID, profileID string) (MetadataFilterResult, string) {
	scope := func(groups [][]conditionAndArg) [][]conditionAndArg {
		scoped := make([][]conditionAndArg, 0, len(groups))
		for _, group := range groups {
			conditions := append([]conditionAndArg(nil), group...)
			conditions = append(conditions, conditionAndArg{condition: "customer_id = ?", arg: customerID}, conditionAndArg{condition: "profile_id = ?", arg: profileID})
			scoped = append(scoped, conditions)
		}
		return scoped
	}
	// The metadata view omits customer/profile columns; use the base table to keep tenant filters.
	metadata := generateMetadataFilters(scope(included), scope(excluded), nil /* aliasDict */, false /* isLastCTE */, false /* fetchFromMetadataView */)
	var conditions strings.Builder
	for index := range included {
		fmt.Fprintf(&conditions, " AND conversation_d.conversation_id GLOBAL IN (SELECT conversation_id_%d FROM moment_annotation_filter_%d)", index, index)
	}
	for index := range excluded {
		fmt.Fprintf(&conditions, " AND conversation_d.conversation_id GLOBAL NOT IN (SELECT conversation_id_exclude_%d FROM moment_annotation_exclude_filter_%d)", index, index)
	}
	return metadata, conditions.String()
}
