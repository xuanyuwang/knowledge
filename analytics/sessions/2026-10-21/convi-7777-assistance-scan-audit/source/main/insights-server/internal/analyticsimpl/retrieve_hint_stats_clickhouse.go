package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	policypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/policy"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

type hintStatsRow struct {
	clickhouseKeyTimeRow
	hintSentCount     uint64
	hintFollowedCount uint64
	convoCount        uint64
	userCount         uint64
}

func hintStatsClickhouseQuery(
	conditionsAndArgs map[ClickhouseTable][]conditionAndArg,
	groupByKeys []string,
	groupByQueries map[ClickhouseTable][]string,
	includeBotConversations bool,
	momentConditionsAndArgsList [][]conditionAndArg,
	excludedMomentConditionsAndArgs [][]conditionAndArg,
	useActionAnnotation bool,
) (string, []any) {
	relations := buildHintUsageRowRelations(conditionsAndArgs, includeBotConversations, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, useActionAnnotation)
	maGroupByQueries, aaGroupByQueries, ceGroupByQueries := concatQueries(groupByQueries[momentAnnotationTable]), concatQueries(groupByQueries[actionAnnotationTable]), concatQueries(groupByQueries[conversationEventTable])
	finalJoinKeys, finalGroupByQueries := concatKeys(groupByKeys), concatQueries(groupByKeys)
	optionalGroupBy := ""
	if len(groupByKeys) > 0 {
		optionalGroupBy = fmt.Sprintf("GROUP BY %s", concatKeys(groupByKeys))
	}
	behavioralSentGroupBy, behavioralSentID := maGroupByQueries, "adherence_action_annotation_id"
	if useActionAnnotation {
		behavioralSentGroupBy, behavioralSentID = aaGroupByQueries, "action_annotation_id"
	}
	sentProjection := func(groupBy, sentIDColumn string) string {
		return fmt.Sprintf(`%s
 COUNT(DISTINCT %s) AS hint_sent_count,
 COUNT(DISTINCT conversation_id) AS convo_count,
 COUNT(DISTINCT agent_user_id) AS user_count`, groupBy, sentIDColumn)
	}
	behavioralSentQuery, behavioralSentArgs := relations.behavioralSent.query(sentProjection(behavioralSentGroupBy, behavioralSentID), groupByKeys)
	nonBehavioralSentQuery, nonBehavioralSentArgs := relations.nonBehavioralSent.query(sentProjection(aaGroupByQueries, "action_annotation_id"), groupByKeys)
	behavioralFollowedQuery, behavioralFollowedArgs := relations.behavioralFollowed.query(maGroupByQueries+"COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count", groupByKeys)
	knowledgeFollowedQuery, knowledgeFollowedArgs := relations.knowledgeFollowed.query(ceGroupByQueries+"COUNT(DISTINCT action_annotation_id) as hint_followed_count", groupByKeys)
	hintSentQuery := fmt.Sprintf(`%s
 %s
 hint_sent AS (
 -- behavioral hint
 %s
 UNION ALL
 -- non-behavior hint.
 %s
 ),`, relations.metadata.MomentSubQuery, relations.metadata.ExcludedMomentSubQuery, behavioralSentQuery, nonBehavioralSentQuery)
	allArgs := append([]any{}, relations.metadata.AllArgs...)
	for _, args := range [][]any{behavioralSentArgs, nonBehavioralSentArgs, behavioralFollowedArgs, knowledgeFollowedArgs} {
		allArgs = append(allArgs, args...)
	}
	return fmt.Sprintf(`
 WITH
 %s
 combined_hint_sent AS (
 SELECT
 %s
 -- Here max rather than sum is used because hint_sent already combines behavioral and non-behavioral hints in one single row.
 MAX(hint_sent.convo_count) AS convo_count,
 MAX(hint_sent.user_count) AS user_count,
 MAX(hint_sent.hint_sent_count) AS hint_sent_count
 FROM hint_sent
 %s
 ),
 -- hint_followed_count
 hint_followed AS(
 -- behavioral hint followed including checklist hint.
 %s
 UNION ALL
 -- GW|KB hint followed
 %s
 )
 SELECT
 %s
 -- Here max rather than sum is used because combined_hint_sent already combines behavioral and non-behavioral hints in one single row.
 MAX(combined_hint_sent.convo_count) AS convo_count,
 MAX(combined_hint_sent.user_count) AS user_count,
 MAX(combined_hint_sent.hint_sent_count) AS hint_sent_count,
 SUM(hint_followed.hint_followed_count) AS hint_followed_count
 FROM combined_hint_sent
 LEFT JOIN hint_followed USING (%s)
 %s`, hintSentQuery, finalGroupByQueries, optionalGroupBy,
		behavioralFollowedQuery,
		knowledgeFollowedQuery,
		finalGroupByQueries, finalJoinKeys, optionalGroupBy), allArgs
}

func (a AnalyticsServiceImpl) readHintStatsFromClickhouse(
	ctx context.Context,
	req *analyticspb.RetrieveHintStatsRequest,
	users []*userpb.User,
) (*analyticspb.RetrieveHintStatsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("getting profile config for hint stats: %w", err)
	}

	targetTables := []ClickhouseTable{
		momentAnnotationTable,
		actionAnnotationTable,
		conversationEventTable,
	}
	conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, usersExtTable, err := parseClickhouseFilter(ctx, req.FilterByTimeRange, req.FilterByAttribute, targetTables, a.enableExtTableForUserFilter)
	if err != nil {
		return nil, err
	}

	groupByKeys, groupByQueries, err := parseClickhouseGroupBy(&req.Frequency, req.GroupByAttributeTypes, req.Metadata, targetTables)
	if err != nil {
		return nil, err
	}
	if v := unsupportedGroupByKeys(groupByKeys, []string{agentUserIDColumn, behaviorIDColumn, policyIDColumn, truncatedTimeColumn}); len(v) > 0 {
		return nil, fmt.Errorf("group_by keys %v are not supported", v)
	}

	includeBotConversations := !profileConfig.GetFeatures().GetInsights().GetDisableStatsForConversationSourceBot()
	useActionAnnotation := *useActionAnnotationForHintStats || profileConfig.GetFeatures().GetInsights().GetAllowHintStatsCalculationActionAnnotationBasedQuery()

	// If includeBotConversations is true, we will include CONVERSATION_SOURCE_BOT as part of stats.
	// If req.FilterByAttribute.IncludeCrestaBot is true, we will include CONVERSATION_SOURCE_BOT as part of stats.
	// If both are true, we will include CONVERSATION_SOURCE_BOT as part of stats.
	chQuery, chArgs := hintStatsClickhouseQuery(conditionsAndArgs, groupByKeys, groupByQueries, includeBotConversations || req.FilterByAttribute.IncludeCrestaBot, momentConditionsAndArgsList, excludedMomentConditionsAndArgs, useActionAnnotation)
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	ctx = attachExtTablesToContext(ctx, usersExtTable)
	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	chRows := []hintStatsRow{}
	for rows.Next() {
		row := hintStatsRow{}
		vals := []interface{}{}
		for _, k := range groupByKeys {
			switch k {
			case policyIDColumn:
				vals = append(vals, &row.policyID)
			case agentUserIDColumn:
				vals = append(vals, &row.agentUserID)
			case truncatedTimeColumn:
				vals = append(vals, &row.truncatedTime)
			case behaviorIDColumn:
				vals = append(vals, &row.behaviorID)
			}
		}
		vals = append(vals, &row.convoCount, &row.userCount, &row.hintSentCount, &row.hintFollowedCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[hintStats] scanning clickhouse rows: %w", err)
		}
		chRows = append(chRows, row)
	}
	a.logger.Infof(ctx, "ClickHouse query latency for RetrieveHintStats in seconds %v", time.Since(startT).Seconds())
	resp, err := a.convertCHResponseToHintStatsResponse(ctx, parent.CustomerID, parent.ProfileID, chRows, users)
	if err != nil {
		return nil, err
	}

	policyIDs := []string{}
	hasTruncatedTimeKey := slices.Contains(groupByKeys, truncatedTimeColumn)
	for _, hintResult := range resp.HintStatsResults {
		if hintResult.Attribute != nil && len(hintResult.Attribute.AttributePolicies) > 0 {
			policyName, err := policypb.ParsePolicyName(hintResult.Attribute.AttributePolicies[0].Name)
			if err != nil {
				return nil, err
			}
			policyIDs = append(policyIDs, policyName.PolicyID)

			// Also populate `actions` when attribute.polices exists.
			actionRows := postgres.SearchActionsViaPolicies(parent.CustomerID, parent.ProfileID, policyName.PolicyID, "", req.GetFilterByAttribute().HintType, a.appsDB.DB(ctx))
			for _, a := range actionRows {
				hintResult.Attribute.Actions = append(hintResult.Attribute.Actions, postgres.ConvertActionRowToActionProto(parent.CustomerID, parent.ProfileID, a))
			}
		}
		// We allow caller without frequency to represent not grouping by truncated time.
		// And in this case, we will skip hint stats (which is key'ed by time).
		if !hasTruncatedTimeKey && len(hintResult.HintStats) > 0 {
			hintResult.HintStats = []*analyticspb.HintStats{}
		}
	}
	policies := postgres.ListPolicies(a.appsDB.DB(ctx), policyIDs)
	nameToPolicy := map[string]*policypb.Policy{}
	for _, p := range policies {
		name := policypb.PolicyName{
			CustomerID: parent.CustomerID,
			ProfileID:  parent.ProfileID,
			PolicyID:   p.PolicyID,
		}.String()
		policyLabels := map[string]string{}
		if p.Labels.Message != nil && p.Labels.Message.Labels != nil {
			policyLabels = p.Labels.Message.Labels
		}
		nameToPolicy[name] = &policypb.Policy{
			Name:        name,
			DisplayName: p.DisplayName,
			SourceType:  policypb.Policy_SourceType(p.SourceType.Int32),
			Labels: &labels.Labels{
				Labels: policyLabels,
			},
			Status: policypb.Policy_Status(p.Status),
		}
	}

	// We remove hint results that meet some criteria:
	//   - attribute.moments/policies, but action can't be resolved.
	//   - attribute.policies[0] is at Archived status.
	indicesToRemove := []int{}
	for i, hintResult := range resp.HintStatsResults {
		if hintResult.Attribute != nil && len(hintResult.Attribute.AttributePolicies) > 0 {
			if p, exists := nameToPolicy[hintResult.Attribute.AttributePolicies[0].Name]; exists {
				hintResult.Attribute.AttributePolicies[0] = &analyticspb.Attribute_AttributePolicy{
					Name:        p.Name,
					DisplayName: p.DisplayName,
				}
				if p.GetStatus() == policypb.Policy_ARCHIVED || len(hintResult.Attribute.Actions) == 0 {
					indicesToRemove = append(indicesToRemove, i)
				}
			}
		}
	}

	if req.FilterByAttribute.HintType != analyticspb.Attribute_HINT_TYPE_UNSPECIFIED {
		removeHintResults(resp, indicesToRemove)
	}
	resp.HintStatsResults = sortResults(resp.HintStatsResults)

	return resp, nil
}

func (a AnalyticsServiceImpl) convertCHResponseToHintStatsResponse(
	ctx context.Context,
	customerID, profileID string,
	rows []hintStatsRow,
	users []*userpb.User,
) (*analyticspb.RetrieveHintStatsResponse, error) {
	if len(rows) == 0 {
		return &analyticspb.RetrieveHintStatsResponse{}, nil
	}
	groupByAttributeToHintStatsResults := make(map[groupByAttribute]*analyticspb.HintStatsResult)
	groupByAttributeToUserCount := make(map[groupByAttribute]int32)

	for _, row := range rows {
		hintStats := analyticspb.HintStats{
			TimeRange:                                   &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.truncatedTime)},
			TotalConversationCount:                      int32(row.convoCount),
			TotalHintSentCount:                          int32(row.hintSentCount),
			TotalHintFollowedCount:                      int32(row.hintFollowedCount),
			AverageHintSentCountPerConversation:         safeDivideFloat(int32(row.hintSentCount), int32(row.convoCount)),
			AverageHintFollowedCountPerConversation:     safeDivideFloat(int32(row.hintFollowedCount), int32(row.convoCount)),
			AverageHintSentCountPerFrequencyPerUser:     safeDivideFloat(int32(row.hintSentCount), int32(row.userCount)),
			AverageHintFollowedCountPerFrequencyPerUser: safeDivideFloat(int32(row.hintFollowedCount), int32(row.userCount)),
		}

		groupBy := row.GetGroupByAttribute()
		if _, exists := groupByAttributeToHintStatsResults[groupBy]; !exists {
			groupByAttributeToHintStatsResults[groupBy] = &analyticspb.HintStatsResult{}
			groupByAttributeToUserCount[groupBy] = int32(0)
		}
		groupByAttributeToHintStatsResults[groupBy].HintStats = append(groupByAttributeToHintStatsResults[groupBy].HintStats, &hintStats)
		groupByAttributeToUserCount[groupBy] += int32(row.userCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	groupByAttributeToAttribute := map[groupByAttribute]*analyticspb.Attribute{}
	attributes := []*analyticspb.Attribute{}
	for k := range groupByAttributeToHintStatsResults {
		attr := k.GetAttribute(customerID, profileID)
		if len(attr.Users) > 0 {
			if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
				attr.Users[0].FullName = user.FullName
				attr.Users[0].Username = user.Username
			}
		}

		groupByAttributeToAttribute[k] = attr
		attributes = append(attributes, attr)
	}

	ignoredAttributes, err := attributesToIgnore(a.appsDB.DB(ctx), attributes)
	if err != nil {
		return nil, err
	}

	totalUserFrequencyCount := int32(0)
	resp := &analyticspb.RetrieveHintStatsResponse{}
	for k, v := range groupByAttributeToHintStatsResults {
		attr := groupByAttributeToAttribute[k]
		if exists := ignoredAttributes[attr]; exists {
			continue
		}
		v.Attribute = attr
		for _, hintStat := range v.HintStats {
			v.TotalHintSentCount += hintStat.TotalHintSentCount
			v.TotalHintFollowedCount += hintStat.TotalHintFollowedCount
			v.TotalConversationCount += hintStat.TotalConversationCount
		}

		v.AverageHintSentCountPerConversation = safeDivideFloat(v.TotalHintSentCount, v.TotalConversationCount)
		v.AverageHintFollowedCountPerConversation = safeDivideFloat(v.TotalHintFollowedCount, v.TotalConversationCount)
		// This per_frequency_per_user values are only valid when group_by_agent is specified in API request.
		v.AverageHintSentCountPerFrequencyPerUser = safeDivideFloat(v.TotalHintSentCount, groupByAttributeToUserCount[k])
		v.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(v.TotalHintFollowedCount, groupByAttributeToUserCount[k])

		// Sort result based on timestamp.
		sort.Slice(v.HintStats, func(i, j int) bool {
			return v.HintStats[i].TimeRange.StartTimestamp.AsTime().Before(v.HintStats[j].TimeRange.StartTimestamp.AsTime())
		})
		resp.HintStatsResults = append(resp.HintStatsResults, v)
		resp.TotalHintSentCount += v.TotalHintSentCount
		resp.TotalHintFollowedCount += v.TotalHintFollowedCount
		resp.TotalConversationCount += v.TotalConversationCount
		totalUserFrequencyCount += groupByAttributeToUserCount[k]
	}
	resp.AverageHintSentCountPerConversation = safeDivideFloat(resp.TotalHintSentCount, resp.TotalConversationCount)
	resp.AverageHintFollowedCountPerConversation = safeDivideFloat(resp.TotalHintFollowedCount, resp.TotalConversationCount)
	resp.AverageHintSentCountPerFrequencyPerUser = safeDivideFloat(resp.TotalHintSentCount, totalUserFrequencyCount)
	resp.AverageHintFollowedCountPerFrequencyPerUser = safeDivideFloat(resp.TotalHintFollowedCount, totalUserFrequencyCount)

	return resp, nil
}
