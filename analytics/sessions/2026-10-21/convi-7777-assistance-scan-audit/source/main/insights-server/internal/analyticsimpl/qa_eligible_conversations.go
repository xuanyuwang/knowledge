package analyticsimpl

import (
	"context"
	"fmt"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/util/errgroup"
	"gorm.io/gorm"

	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
	"github.com/cresta/go-servers/shared/qa"
)

// countEligibleConversationsByAgent returns how many conversations of each agent in agents match the task's
// conversation filters in the period, keyed by agent user ID. Agents with no eligible conversation are not in the
// result. It builds the same filter and query as SuggestManualQAConversations, so the counts match the conversations
// reviewers are offered.
func (a AnalyticsServiceImpl) countEligibleConversationsByAgent(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	parent *customerpb.CustomerOrProfileName,
	template *dbmodel.ScorecardTemplates,
	period *qa.EvaluationPeriod,
	agents map[string]*userpb.User,
) (map[string]int32, error) {
	counts := make(map[string]int32)
	if len(agents) == 0 {
		return counts, nil
	}

	filter, err := GetQAConversationFilters(period, fn.MapValueToSlice(agents), template, nil /* filterAttribute */)
	if err != nil {
		return nil, fmt.Errorf("building conversation filters for template %s: %w", template.ResourceID, err)
	}
	topicDetails, err := a.expandConversationFilter(ctx, shared.CustomerProfile{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}, filter)
	if err != nil {
		return nil, fmt.Errorf("expanding conversation filters for template %s: %w", template.ResourceID, err)
	}

	timeRangeTarget := getTimeRangeTarget(period, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED)
	query, timeZone, err := a.buildClosedConversationsQuery(
		ctx, profileConfig, nil /* metadata */, filter, timeRangeTarget, hardcodedConversationDurationBuckets, topicDetails, filter.Users)
	if err != nil {
		return nil, fmt.Errorf("building eligible conversations query for template %s: %w", template.ResourceID, err)
	}

	size := int32(0)
	esRequest := Request{
		Query: &query,
		Size:  &size,
		Aggs: &ReqAggs{
			Agents: &ReqAgents{
				Terms: &ReqTerms{
					Field: "agent_user_id.keyword",
					Size:  int32(len(agents)),
				},
			},
		},
	}
	if len(filter.TruncatedTimeRanges) > 0 {
		esRequest.RuntimeMappings = elasticsearch.BuildRuntimeMappingsForTruncatedTimeRangeFilters(timeZone, timeRangeTarget)
	}

	esResponse := Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /* pageSize */, false /* logESQuery */, esRequest, &esResponse, "CountEligibleQAConversations")
	if err != nil {
		return nil, fmt.Errorf("counting eligible conversations for template %s: %w", template.ResourceID, err)
	}

	if esResponse.Aggregations.Agents == nil {
		return counts, nil
	}
	for _, bucket := range esResponse.Aggregations.Agents.Buckets {
		counts[bucket.Key] = bucket.DocCount
	}
	return counts, nil
}

// resolveTaskAgents returns the latest revision of the template and the agents in the task's audience.
func (a AnalyticsServiceImpl) resolveTaskAgents(
	ctx context.Context,
	db *gorm.DB,
	parent *customerpb.CustomerOrProfileName,
	templateID string,
	taskConfig *qadbpb.QATaskConfig,
) (*dbmodel.ScorecardTemplates, map[string]*userpb.User, error) {
	templates, err := qa.ListScorecardTemplates(db, parent.CustomerID, parent.ProfileID, templateID, nil /* usecases */)
	if err != nil {
		return nil, nil, fmt.Errorf("listing scorecard template %s: %w", templateID, err)
	}
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, nil, fmt.Errorf("creating user client for %s: %w", parent, err)
	}
	taskUsers, err := qa.ResolveTaskUsersFromConfig(ctx, userClient, parent, templates[0], taskConfig, qa.WithUsersPageSize(shared.DefaultListUsersPageSize))
	if err != nil {
		return nil, nil, fmt.Errorf("resolving task users for template %s: %w", templateID, err)
	}
	return templates[0], taskUsers, nil
}

func isConversationCoveragePeriod(period *qa.EvaluationPeriod) bool {
	return qa.IsConversationCoverageTask(period.QATaskConfig.GetTaskCompleteConfig())
}

// countCoverageConversationsForPeriods returns, for each period, how many conversations of the agents with agentIDs
// must be evaluated under the period's coverage percentage. Each agent's count is rounded on its own, so the total
// matches the sum of the agents' counts. IDs not in taskUsers are ignored.
func (a AnalyticsServiceImpl) countCoverageConversationsForPeriods(
	ctx context.Context,
	parent *customerpb.CustomerOrProfileName,
	template *dbmodel.ScorecardTemplates,
	periods []*qa.EvaluationPeriod,
	taskUsers map[string]*userpb.User,
	agentIDs []string,
) ([]int32, error) {
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("getting profile config for %s: %w", parent, err)
	}

	agents := make(map[string]*userpb.User, len(agentIDs))
	for _, agentID := range agentIDs {
		agent, ok := taskUsers[agentID]
		if !ok {
			continue
		}
		agents[agentID] = agent
	}

	totals := make([]int32, len(periods))
	eg := errgroup.New(errgroup.WithLimit(maxConcurrentESQueries))
	for i, period := range periods {
		eg.Go(func() error {
			counts, err := a.countEligibleConversationsByAgent(ctx, profileConfig, parent, template, period, agents)
			if err != nil {
				return err
			}
			for _, count := range counts {
				totals[i] += qa.ConversationCoverageCount(period.QATaskConfig.GetTaskCompleteConfig(), count)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return totals, nil
}
