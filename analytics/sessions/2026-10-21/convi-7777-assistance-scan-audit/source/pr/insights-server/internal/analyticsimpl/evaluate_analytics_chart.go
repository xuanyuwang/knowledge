package analyticsimpl

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"

	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/dashboarding"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	clickhouseevents "github.com/cresta/go-servers/shared/clickhouse/events"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
	"github.com/cresta/go-servers/shared/dbuuid"
	"github.com/cresta/go-servers/shared/requestcache"
	"github.com/cresta/shared-go/framework/extrawork"
	"github.com/cresta/shared-go/util/envflag"
)

var (
	enableACLFilteringInEvaluateAnalyticsChart = envflag.Bool("ENABLE_ACL_FILTERING_IN_EVALUATE_ANALYTICS_CHART", false, "Whether to apply ACL filtering to the chart data config.")
	enableCachingInEvaluateAnalyticsChart      = envflag.Bool("ENABLE_CACHING_IN_EVALUATE_ANALYTICS_CHART", false, "Whether to use and read from the request cache for chart evaluation.")
	evaluateAnalyticsChartCacheTTL             = envflag.Duration("EVALUATE_ANALYTICS_CHART_CACHE_TTL", 7*24*time.Hour, "The TTL for the request cache for chart evaluation.")
)

func (a AnalyticsServiceImpl) EvaluateAnalyticsChart(ctx context.Context, req *analyticspb.EvaluateAnalyticsChartRequest) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	if req == nil || req.AnalyticsChart == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request for EvaluateAnalyticsChart not allowed")
	}
	// A combo chart (exactly 2 metrics) is evaluated as two independent single-metric charts
	// merged onto a shared outer axis; see evaluate_analytics_chart_combo.go. The single-metric
	// body below is left unchanged and handles the 1-metric (and legacy >2-metric) paths.
	if isComboChartRequest(req) {
		return a.evaluateComboChart(ctx, req)
	}
	if err := shared.ValidateAnalyticsChartDataConfig(ctx, req.AnalyticsChart.DataConfig); err != nil {
		return nil, err
	}
	queryEngineType, err := determineQueryEngine(req.AnalyticsChart)
	if err != nil {
		return nil, err
	}
	if queryEngineType != QueryEngineES && queryEngineType != QueryEngineInfraMetrics {
		return nil, status.Errorf(codes.Internal, "only ES and InfraMetrics query engine is supported, but found %v", queryEngineType)
	}
	a.logger.Infof(ctx, "evaluating analytics chart %v\n", req.AnalyticsChart.Name)
	if len(req.AnalyticsChart.Name) == 0 {
		a.logger.Infof(ctx, "payload for non-existent chart %v\n", req.AnalyticsChart)
	}

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	var insightFeatureConfig *features.Insights
	if profileConfig != nil && profileConfig.GetFeatures() != nil {
		insightFeatureConfig = profileConfig.GetFeatures().GetInsights()
	}
	precisionThresholdForCardAgg := getPrecisionThresholdForCardinalityAgg(insightFeatureConfig)

	authEntity, err := auth.GetPrincipalEntityFromAuthContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "error during authentication: %v", err)
	}

	// Apply ACL filtering to chart data config
	if !authEntity.HasAnyRole(authpb.AuthProto_CI_ADMIN) && *enableACLFilteringInEvaluateAnalyticsChart {
		noAccessibleFilterUsers, err := a.ApplyACLToChartDataConfig(ctx, parent.CustomerID, parent.ProfileID, req.AnalyticsChart.DataConfig)
		if err != nil {
			return nil, err
		}
		if noAccessibleFilterUsers {
			return &analyticspb.EvaluateAnalyticsChartResponse{}, nil
		}
	}

	chartDataConfig := req.AnalyticsChart.DataConfig
	if chartDataConfig == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty chart_data_config not allowed")
	}
	groupByFromMetric, filterFromMetric, err := extractGroupByAndFilterFromMetric(chartDataConfig.Metrics[0])
	if err != nil {
		return nil, err
	}

	// When moving metrics into filters/groupBys, skip adding if an existing filter/groupBy already
	// has the same subcategory moment names. This de-duplicates the common case where dashboards
	// specify the same subcategories in both metrics and filters/groupBys.
	if filterFromMetric != nil && !filtersContainMatchingSubcategoryValueSet(ctx, chartDataConfig.Filters, filterFromMetric.GetValueSet()) {
		chartDataConfig.Filters = append(chartDataConfig.Filters, filterFromMetric)
	}

	if groupByFromMetric != nil && !groupBysContainMatchingSubcategoryValueSet(ctx, chartDataConfig.GroupBy, groupByFromMetric.GetByProvidedValue().GetValueSet()) {
		// Order of group by matters: affect how ES query is constructed; also sorting can
		// only be applied to outer groupby in ES query.
		chartDataConfig.GroupBy = append([]*dashboardpb.ChartGroupBy{groupByFromMetric}, chartDataConfig.GroupBy...)
	}

	// Ensure config of outer groupby is respected.
	if len(chartDataConfig.GroupBy) > 1 {
		outerGroupbyIdx := []int{}
		for i, groupby := range chartDataConfig.GroupBy {
			if groupby.Config != nil && groupby.Config.GroupbyLayer == dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER {
				outerGroupbyIdx = append(outerGroupbyIdx, i)
			}
		}
		if l := len(outerGroupbyIdx); l > 1 {
			return nil, status.Errorf(codes.InvalidArgument, "only allow at most one outer groupby, but found %v", l)
		}
		if len(outerGroupbyIdx) == 1 {
			chartDataConfig.GroupBy[0], chartDataConfig.GroupBy[outerGroupbyIdx[0]] = chartDataConfig.GroupBy[outerGroupbyIdx[0]], chartDataConfig.GroupBy[0]
		}
	}

	timeRange, err := a.getChartTimeRangeFilter(ctx, chartDataConfig, req.Metadata)
	if err != nil {
		return nil, err
	}
	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	// Only ES-backed charts read conversations-v2; a nil timeRange means an all-time (full-index) read.
	if queryEngineType == QueryEngineES {
		var chartReadFrom *timestamppb.Timestamp
		if timeRange != nil {
			chartReadFrom = timeRange.from
		}
		recordConversationReadRecency(ctx, chartReadFrom, parent.CustomerID, parent.ProfileID, "evaluate_analytics_chart")
	}
	// Corner case when grouping by time on invalid time range.
	if timeRange != nil {
		from, to := timeRange.from, timeRange.to
		if from != nil && to != nil && from.AsTime().After(to.AsTime()) {
			for _, groupby := range chartDataConfig.GroupBy {
				if groupby.GetByTime() != nil {
					return &analyticspb.EvaluateAnalyticsChartResponse{}, nil
				}
			}
		}
	}

	// Shuffle groupbys order of if needed.
	// E.g., grouping by criterions has to be inner groupby in ES query.
	shuffleGroupBysIfNeeded := func(dataConfig *dashboardpb.ChartDataConfig) (bool, error) {
		if len(dataConfig.GetGroupBy()) < 2 {
			return false, nil
		}

		if forCriterion := dataConfig.Metrics[0].GetForScorecardCriterion(); forCriterion != nil && len(forCriterion.ScorecardCriterions) > 1 && forCriterion.StatsType.GetAggregationOnScore() != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_UNSPECIFIED {
			groupbys := dataConfig.GroupBy
			if byProvidedValue := groupbys[0].GetByProvidedValue(); byProvidedValue != nil {
				csve, err := fromChartValueSet(ctx, byProvidedValue.ValueSet)
				if err != nil {
					return false, err
				}
				if csve.attributeValueType == attributeValuetypeScorecardCriterion {
					groupbys[0], groupbys[1] = groupbys[1], groupbys[0]
					return true, nil
				}
			}
		}

		if forScorecardTemplate := dataConfig.Metrics[0].GetForScorecardTemplate(); forScorecardTemplate != nil && len(forScorecardTemplate.ScorecardTemplates) > 1 && forScorecardTemplate.StatsType.GetAggregationOnScore() != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_UNSPECIFIED {
			groupbys := dataConfig.GroupBy
			if byProvidedValue := groupbys[0].GetByProvidedValue(); byProvidedValue != nil {
				csve, err := fromChartValueSet(ctx, byProvidedValue.ValueSet)
				if err != nil {
					return false, err
				}
				if csve.attributeValueType == attributeValueTypeScorecardTemplate {
					groupbys[0], groupbys[1] = groupbys[1], groupbys[0]
					return true, nil
				}
			}
		}
		return false, nil
	}
	groupbyShuffled, err := shuffleGroupBysIfNeeded(chartDataConfig)
	if err != nil {
		return nil, err
	}

	emptyResp, err := a.excludeDeactivatedUsersFromProvidedUserGroupBys(ctx, parent, chartDataConfig)
	if err != nil {
		return nil, err
	}
	if emptyResp {
		return &analyticspb.EvaluateAnalyticsChartResponse{}, nil
	}

	outerGroupMemberIDs, noOuterValuesLeft, err := a.scopeOuterGroupByToUserFilters(ctx, parent, chartDataConfig)
	if err != nil {
		return nil, err
	}
	if noOuterValuesLeft {
		return &analyticspb.EvaluateAnalyticsChartResponse{}, nil
	}

	groupby := chartDataConfig.GroupBy
	if len(groupby) == 2 && hasSorting(groupby[0]) {
		if f := filterFromGroupby(groupby[1]); f != nil {
			chartDataConfig.Filters = append(chartDataConfig.Filters, f)
		}
	}

	// Note: Cache read here is at the earliest place it can be right now. Any
	// earlier and we might still be modifying req (via
	// req.AnalyticsChart.DataConfig).
	cacheKey := a.getCachedRequestProto(ctx, req, authEntity)
	if a.isCachingEnabled(insightFeatureConfig) && !req.ReadFromLiveQuery {
		if !cacheKey.GetIsAdmin() {
			a.logger.Infof(ctx, "skipping cache retrieval for non-admin user")
		} else {
			resp := &analyticspb.EvaluateAnalyticsChartResponse{}
			found, err := requestcache.GetRequestCache(a.appsDB.DB(ctx), profileConfig.GetProfileName(), cacheKey, resp)
			if err != nil {
				a.logger.Warnf(ctx, "failed to get request cache in DB: %v", err)
			}
			if found {
				a.logger.Infof(ctx, "cache hit for chart %v", req.AnalyticsChart.Name)
				a.emitEvaluateAnalyticsChartCacheEventAsync(parent, authEntity, req.AnalyticsChart.Name, clickhouseevents.EvaluateAnalyticsChartCacheHitEventName)
				return resp, nil
			}
			a.logger.Infof(ctx, "cache miss for chart %v, with request %s", req.AnalyticsChart.Name, cacheKey)
			a.emitEvaluateAnalyticsChartCacheEventAsync(parent, authEntity, req.AnalyticsChart.Name, clickhouseevents.EvaluateAnalyticsChartCacheMissEventName)
		}
	}

	if queryEngineType == QueryEngineInfraMetrics {
		return a.evaluateInfraMetricsChart(ctx, req)
	}

	// Resolve any Topic Discovery V3 category clusters referenced in filters/group-bys to their full
	// subtree closure (the category itself + every descendant, leaf AND non-leaf). Filters and
	// group-bys share the same closure: a filter on a parent category matches every conversation
	// classified anywhere beneath it, and a group-by on that category collapses the whole subtree into
	// a single bucket. Keys and values are always moment resource names (bare moment_template_ids are
	// converted). The closure is threaded to nestedFromClusters, where a V3 cluster's moment id is
	// replaced by the closure's message-anchored moment ids. Empty when no V3 clusters are referenced.
	v3ClusterClosures, err := a.buildTopicDiscoveryV3ClusterClosures(ctx, *parent, chartDataConfig)
	if err != nil {
		a.logger.Errorf(
			ctx,
			"failed to resolve topic discovery v3 clusters for chart evaluation (chart=%q, parent=%q): %v",
			req.GetAnalyticsChart().GetName(),
			req.GetParent(),
			err,
		)
		return nil, err
	}

	// Prefetch topic details and aggregate expansions to avoid repeated fetching inside inner loops.
	fetchedTopics, aggregateExpansions, err := a.fetchTopicDetailsForChart(ctx, req.Parent, chartDataConfig.GroupBy, chartDataConfig.Filters)
	if err != nil {
		a.logger.Errorf(
			ctx,
			"failed to fetch topic details for chart evaluation (chart=%q, parent=%q, groupby_count=%d, filter_count=%d): %v",
			req.GetAnalyticsChart().GetName(),
			req.GetParent(),
			len(chartDataConfig.GetGroupBy()),
			len(chartDataConfig.GetFilters()),
			err,
		)
		return nil, err
	}

	metricsAggregation, esQueryDocType, metricsAggDocType, statsType, err := a.convertChartMetricToESMetric(ctx, parent, profileConfig, chartDataConfig.Metrics[0], chartDataConfig.GroupBy, precisionThresholdForCardAgg, fetchedTopics, aggregateExpansions, v3ClusterClosures)
	if err != nil {
		a.logger.Errorf(
			ctx,
			"failed to convert chart metric to ES metric (chart=%q, parent=%q, metric_type=%T): %v",
			req.GetAnalyticsChart().GetName(),
			req.GetParent(),
			chartDataConfig.Metrics[0].Metric,
			err,
		)
		return nil, err
	}

	canRemoveJoinsByFilteringOnMessage, err := a.canRemoveJoinsByFilteringOnMessage(ctx, esQueryDocType, chartDataConfig.GroupBy, chartDataConfig.Filters, statsType, hasPinnedClusters(fetchedTopics), profileConfig)
	if err != nil {
		return nil, err
	}
	if esQueryDocType == elasticsearch.DocumentTypeConversation && canRemoveJoinsByFilteringOnMessage {
		esQueryDocType, metricsAggDocType = elasticsearch.DocumentTypeMessage, elasticsearch.DocumentTypeMessage
	}
	useConversationCountViaDocCount(metricsAggregation, statsType, metricsAggDocType, chartDataConfig.Metrics[0], chartDataConfig.GroupBy)
	eSQuery, err := a.convertChartToESQuery(ctx, *parent, profileConfig, esQueryDocType, chartDataConfig, req.ConvoFilter, req.Metadata, fetchedTopics, nil /*=convoFilterFromMsgEntity*/, aggregateExpansions, v3ClusterClosures, false /*=agentFilterAlwaysOnConversation*/)
	if err != nil {
		a.logger.Errorf(
			ctx,
			"failed to convert chart to ES query (chart=%q, parent=%q, document_type=%v): %v",
			req.GetAnalyticsChart().GetName(),
			req.GetParent(),
			esQueryDocType,
			err,
		)
		return nil, err
	}

	indexName := sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID})
	enableCompositeAggregationTopBottomN, err := a.shouldUseCompositeAggregation(ctx, req, &eSQuery, indexName, *parent, precisionThresholdForCardAgg, metricsAggregation)
	if err != nil {
		a.logger.Warnf(ctx, "Failed to detect whether to use composite aggregation, fall back to terms aggregation: %v", err)
	}

	bucketAggs, err := a.convertGroupBys(ctx, *parent, profileConfig, timeRange, req.Metadata, esAggDocTypeParams{startType: esQueryDocType, endType: metricsAggDocType}, chartDataConfig.GroupBy, metricsAggregation, precisionThresholdForCardAgg, fetchedTopics, aggregateExpansions, v3ClusterClosures, outerGroupMemberIDs, enableCompositeAggregationTopBottomN)
	if err != nil {
		a.logger.Errorf(
			ctx,
			"failed to convert chart groupbys (chart=%q, parent=%q, groupby_count=%d): %v",
			req.GetAnalyticsChart().GetName(),
			req.GetParent(),
			len(chartDataConfig.GetGroupBy()),
			err,
		)
		return nil, err
	}

	aggs := mergeMetricAgg(bucketAggs, metricsAggregation)

	if len(chartDataConfig.GroupBy) >= 1 && chartDataConfig.GroupBy[0].GetByUniqueValue() != nil {
		if err := correctOrderByForTermsAgg(chartDataConfig, metricsAggregation, aggs); err != nil {
			return nil, err
		}
	}

	esRequest := ChartESRequest{
		Query: &eSQuery,
		Aggs:  aggs,
	}
	if chartFiltersHaveAdvancedTimeRule(chartDataConfig.Filters) {
		timeZone := req.Metadata.GetTimeZoneId()
		if timeZone == "" {
			timeZone = shared.DefaultTimeZone
		}
		esRequest.RuntimeMappings = elasticsearch.BuildRuntimeMappingsForTruncatedTimeRangeFilters(
			timeZone, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED,
		)
	}

	var esResp ChartESResponse
	if enableCompositeAggregationTopBottomN && len(chartDataConfig.GroupBy) >= 1 && chartDataConfig.GroupBy[0].GetByUniqueValue() != nil {
		sortValuePath, err := compositeAggregationGetSortValuePath(chartDataConfig, metricsAggregation, aggs)
		if err != nil {
			return nil, err
		}
		esResp, err = a.topBottomNCompositeAgg(
			ctx,
			chartDataConfig.GroupBy[0].Config,
			indexName,
			&esRequest,
			sortValuePath,
			fmt.Sprintf("EvaluateAnalyticsChart for %v", req.AnalyticsChart.Name))
		if err != nil {
			return nil, err
		}
	} else {
		esResp = ChartESResponse{}
		err = elasticsearch.Query(
			ctx,
			a.esClient,
			a.logger,
			indexName,
			elasticsearch.EsModeCount,
			0, /*=pageSize*/
			true,
			esRequest,
			&esResp,
			fmt.Sprintf("EvaluateAnalyticsChart for %v",
				req.AnalyticsChart.Name))
		if err != nil {
			return nil, err
		}
	}

	if hasConversationAgentUniqueGroupBy(chartDataConfig.GroupBy) {
		if err := a.excludeDeactivatedUsersFromConversationAgentBuckets(ctx, parent, &esResp); err != nil {
			return nil, err
		}
	}

	// Clip out-of-range boundary buckets: WEEK/MONTH always, DAY/HOUR only under a day-of-week/time-of-day rule (INSI-2281).
	timeZone := req.Metadata.GetTimeZoneId()
	if timeZone == "" {
		timeZone = shared.DefaultTimeZone
	}
	timeRangeRule := extractTimeRangeRule(chartDataConfig.Filters)
	if err := filterESRespTimeBuckets(&esResp, chartDataConfig.GroupBy, timeRangeRule, timeZone, timeRange); err != nil {
		return nil, err
	}

	// metricDerivedInnerGroupBy: the metric prepended a groupby (e.g.
	// forAggregateSubcategory) now at groupBys[1]. The outer orderBy read returns 0
	// in the 2-groupby case (traversal overshoots into the inner agg), leaving the
	// manual Top/Bottom-N sort a no-op in provided order (INSI-4358); rank by the
	// inner buckets' sum instead — they are subcategory-scoped via the top-level
	// filter. Gated to count/occurrence metrics; summing inner buckets is invalid
	// for averages and numeric-field aggs (e.g. scorecard AggregationOnScore).
	metricDerivedInnerGroupBy := groupByFromMetric != nil &&
		len(chartDataConfig.GroupBy) == 2 &&
		chartDataConfig.GroupBy[1] == groupByFromMetric &&
		!metricsAggregation.isAverageOccurrenceCountPerConvo &&
		metricsAggregation.MetricsAggOnNumericField == nil
	convertedResp, err := a.convertESResponse(ctx, parent, esQueryDocType, chartDataConfig.GroupBy, metricsAggregation, groupbyShuffled, metricDerivedInnerGroupBy, esResp)
	if err != nil {
		return nil, err
	}
	if a.isCachingEnabled(insightFeatureConfig) && req.WriteToCache {
		if !cacheKey.GetIsAdmin() {
			a.logger.Infof(ctx, "skipping cache write for non-admin user")
		} else {
			if err := requestcache.StoreRequestCache(a.appsDB.DB(ctx), profileConfig.GetProfileName(), cacheKey, convertedResp, *evaluateAnalyticsChartCacheTTL); err != nil {
				a.logger.Warnf(ctx, "failed to store request cache: %v", err)
			} else {
				a.logger.Infof(ctx, "successfully wrote into cache for chart %v, with request %v", req.AnalyticsChart.Name, cacheKey)
			}
		}
	}

	return convertedResp, err
}

// emitEvaluateAnalyticsChartCacheEventAsync fires a cache hit/miss event in the background so it
// does not add latency to the RPC. It captures local copies of parent, agentUserID, chartName,
// and eventName, then spawns a goroutine that logs errors and does not affect the RPC.
func (a AnalyticsServiceImpl) emitEvaluateAnalyticsChartCacheEventAsync(
	parent *shared.CustomerProfile,
	authEntity interface{ GetClaimUserID() (string, error) },
	chartName, eventName string,
) {
	parentCopy := &shared.CustomerProfile{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}
	agentUserID := ""
	if authEntity != nil {
		if id, err := authEntity.GetClaimUserID(); err == nil {
			agentUserID = id
		}
	}
	chartNameCopy := chartName
	eventNameCopy := eventName
	extrawork.RunWithTimeout(context.Background(), 5*time.Second, func(ctx context.Context) error {
		a.emitEvaluateAnalyticsChartCacheEvent(ctx, parentCopy, agentUserID, chartNameCopy, eventNameCopy)
		return nil
	})
}

// emitEvaluateAnalyticsChartCacheEvent writes a cache hit/miss event to ClickHouse via
// WriteAnalyticsEventIntoClickhouse so it is available in Hex dashboards via CrestaTrino.
func (a AnalyticsServiceImpl) emitEvaluateAnalyticsChartCacheEvent(
	ctx context.Context,
	parent *shared.CustomerProfile,
	agentUserID, chartName, eventName string,
) {
	if a.clickhouseEventsClient == nil {
		return
	}
	eventProperties, err := structpb.NewStruct(map[string]any{
		"chart_name": chartName,
	})
	if err != nil {
		a.logger.Warnf(ctx, "emit evaluate analytics chart cache event: build event properties: %v", err)
		return
	}
	if err := clickhouseevents.WriteAnalyticsEventIntoClickhouse(
		ctx,
		a.clickhouseEventsClient,
		parent.CustomerID,
		parent.ProfileID,
		agentUserID,
		dbuuid.NewString(),
		eventName,
		eventProperties,
	); err != nil {
		a.logger.Warnf(ctx, "emit evaluate analytics chart cache event: write to ClickHouse: %v", err)
	}
}

func getPrecisionThresholdForCardinalityAgg(insightsFeatureConfig *features.Insights) int {
	ret := elasticsearch.MaxCardinalityPrecisionThreshold
	if insightsFeatureConfig != nil && insightsFeatureConfig.PrecisionThresholdForElasticsearchCardinalityAgg > 0 {
		ret = int(insightsFeatureConfig.PrecisionThresholdForElasticsearchCardinalityAgg)
	}
	return ret
}

func extractUserNamesAndGroupNamesFromChartDataConfig(chartDataConfig *dashboardpb.ChartDataConfig) ([]*userpb.Group, []*userpb.User, []*userpb.Group, []*userpb.User) {
	var filterGroups []*userpb.Group
	var filterUsers []*userpb.User
	var groupByGroups []*userpb.Group
	var groupByUsers []*userpb.User

	// Extract groups and users from filters
	for _, filter := range chartDataConfig.GetFilters() {
		groupNames := dashboarding.MapGroupsValueSetToNames(filter.GetValueSet())
		for _, groupName := range groupNames {
			filterGroups = append(filterGroups, &userpb.Group{Name: groupName})
		}
		userNames := dashboarding.MapUsersValueSetToNames(filter.GetValueSet())
		for _, userName := range userNames {
			filterUsers = append(filterUsers, &userpb.User{Name: userName})
		}
	}

	// Extract groups and users from groupBy
	for _, groupBy := range chartDataConfig.GetGroupBy() {
		if groupBy.GetByProvidedValue() != nil {
			groupNames := dashboarding.MapGroupsValueSetToNames(groupBy.GetByProvidedValue().GetValueSet())
			for _, groupName := range groupNames {
				groupByGroups = append(groupByGroups, &userpb.Group{Name: groupName})
			}
			userNames := dashboarding.MapUsersValueSetToNames(groupBy.GetByProvidedValue().GetValueSet())
			for _, userName := range userNames {
				groupByUsers = append(groupByUsers, &userpb.User{Name: userName})
			}
		}
	}

	return filterGroups, filterUsers, groupByGroups, groupByUsers
}

func hasConversationAgentUniqueGroupBy(groupBys []*dashboardpb.ChartGroupBy) bool {
	for _, groupBy := range groupBys {
		byUniqueValue := groupBy.GetByUniqueValue()
		if byUniqueValue.GetAttribute() == dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT {
			return true
		}
	}
	return false
}

func (a AnalyticsServiceImpl) excludeDeactivatedUsersFromProvidedUserGroupBys(ctx context.Context, parent *shared.CustomerProfile, chartDataConfig *dashboardpb.ChartDataConfig) (bool, error) {
	// Note that this filtering is done post-ES query, which means that any filtered deactivated users here will cause the bucket count to be less than what was requested for.
	// TODO: we should filter deactivated users at the ES query level if possible.
	userNames := map[string]struct{}{}
	for _, groupBy := range chartDataConfig.GetGroupBy() {
		for _, userName := range dashboarding.MapUsersValueSetToNames(groupBy.GetByProvidedValue().GetValueSet()) {
			userNames[userName] = struct{}{}
		}
	}
	if len(userNames) == 0 {
		return false, nil
	}

	activeUserNames, err := a.activeUserNames(ctx, parent, userNames)
	if err != nil {
		return false, err
	}
	for _, groupBy := range chartDataConfig.GetGroupBy() {
		singleValues := groupBy.GetByProvidedValue().GetValueSet().GetSingleValues()
		if singleValues == nil {
			continue
		}
		hadUserValue := false
		filteredValues := make([]*dashboardpb.ChartAttributeValue, 0, len(singleValues.GetAttributeValues()))
		for _, value := range singleValues.GetAttributeValues() {
			userName := value.GetUserNameValue()
			if userName == "" {
				filteredValues = append(filteredValues, value)
				continue
			}
			hadUserValue = true
			if _, ok := activeUserNames[userName]; ok {
				filteredValues = append(filteredValues, value)
			}
		}
		if hadUserValue && len(filteredValues) == 0 {
			return true, nil
		}
		singleValues.AttributeValues = filteredValues
	}
	return false, nil
}

func (a AnalyticsServiceImpl) excludeDeactivatedUsersFromConversationAgentBuckets(ctx context.Context, parent *shared.CustomerProfile, esResp *ChartESResponse) error {
	node, err := traverseToDesiredAggInESResp(esResp.Aggregations, multiBucketAggType)
	if err != nil {
		return err
	}
	if node.Buckets == nil || len(*node.Buckets) == 0 {
		return nil
	}

	userNames := make(map[string]struct{}, len(*node.Buckets))
	for _, bucket := range *node.Buckets {
		agentUserID, ok := bucket.Key.(string)
		if !ok || agentUserID == "" {
			continue
		}
		userNames[buildUserResourceName(parent.CustomerID, agentUserID)] = struct{}{}
	}
	activeUserNames, err := a.activeUserNames(ctx, parent, userNames)
	if err != nil {
		return err
	}

	filteredBuckets := make([]ESRespSingleBucket, 0, len(*node.Buckets))
	for _, bucket := range *node.Buckets {
		agentUserID, ok := bucket.Key.(string)
		if !ok || agentUserID == "" {
			filteredBuckets = append(filteredBuckets, bucket)
			continue
		}
		userName := buildUserResourceName(parent.CustomerID, agentUserID)
		if _, ok := activeUserNames[userName]; ok {
			filteredBuckets = append(filteredBuckets, bucket)
		}
	}
	node.Buckets = &filteredBuckets
	return nil
}

func (a AnalyticsServiceImpl) activeUserNames(ctx context.Context, parent *shared.CustomerProfile, userNames map[string]struct{}) (map[string]struct{}, error) {
	activeUserNames := map[string]struct{}{}
	if len(userNames) == 0 {
		return activeUserNames, nil
	}

	userIDs := make([]string, 0, len(userNames))
	for userName := range userNames {
		parsedUserName, err := userpb.ParseUserName(userName)
		if err != nil {
			return nil, err
		}
		userIDs = append(userIDs, parsedUserName.UserID)
	}
	sort.Strings(userIDs)

	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user client for customer %v profile %v: %v", parent.CustomerID, parent.ProfileID, err)
	}
	pageToken := ""
	for {
		resp, err := userClient.ListUsers(ctx, &userpb.ListUsersRequest{
			Parent:    customerpb.CustomerName{CustomerID: parent.CustomerID}.String(),
			ProfileId: parent.ProfileID,
			PageSize:  int32(shared.DefaultListUsersPageSize),
			PageToken: pageToken,
			Filter: &userpb.ListUsersRequest_ListUsersFilter{
				CustomerProfiles: []string{parent.String()},
				UserIds:          userIDs,
				State:            userpb.User_ACTIVE,
			},
			OrderField: "user_id",
		})
		if err != nil {
			return nil, err
		}
		for _, user := range resp.GetUsers() {
			activeUserNames[user.GetName()] = struct{}{}
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	return activeUserNames, nil
}

func hasSorting(gb *dashboardpb.ChartGroupBy) bool {
	if gb == nil {
		return false
	}
	if gb.GetByUniqueValue() != nil {
		return true
	}
	if gb.Config != nil && gb.Config.SortDirection != dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED {
		return true
	}
	return false
}

// construct a chart filter from a chart groupby.
func filterFromGroupby(gb *dashboardpb.ChartGroupBy) *dashboardpb.ChartDataFilter {
	if gb == nil || gb.GetByProvidedValue() == nil {
		return nil
	}
	return &dashboardpb.ChartDataFilter{
		Operator: dashboardpb.ChartDataFilter_IN,
		ValueSet: gb.GetByProvidedValue().ValueSet,
	}
}

func needManualSortFirstGroupby(groupBys []*dashboardpb.ChartGroupBy) bool {
	// Grouping by unique values uses terms aggregration which has its own sorting feature.
	return len(groupBys) > 0 && groupBys[0].GetByUniqueValue() == nil && groupBys[0].Config != nil && groupBys[0].Config.SortDirection != dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED
}

// Merge metrics aggregation to bucket aggregation.
func mergeMetricAgg(aggsFromGroupBy, metricsAggs *ChartESAggregation) *ChartESAggregation {
	if aggsFromGroupBy == nil {
		return nil
	}
	// Handle the special case of 0 groupby.
	if aggsFromGroupBy.BucketAggs == nil {
		return metricsAggs
	}
	// Use built-in metrics doc_count as total occurrence count, so no custom metrics needed.
	if metricsAggs.isTotalOccurrenceCount || metricsAggs.isConversationCountViaDocCount {
		return aggsFromGroupBy
	}

	// Traverse to the leaf node.
	node := aggsFromGroupBy.BucketAggs
	foundEnd := false
	for !foundEnd {
		switch {
		case node.Aggs == nil:
			foundEnd = true
		case node.Aggs.BucketAggs != nil:
			node = node.Aggs.BucketAggs
		}
	}
	node.Aggs = metricsAggs

	return aggsFromGroupBy
}

func getSortValuePathFromNode(node *BucketAggregation, defaultPathComponent SortValuePathComponent) ([]SortValuePathComponent, error) {
	if node == nil {
		return []SortValuePathComponent{}, status.Errorf(codes.Internal, "expected node to not be nil")
	}

	sortValuePath := []SortValuePathComponent{}

	foundMetric := false
	for !foundMetric {
		var path SortValuePathComponent
		switch {
		// no metrics aggregation.
		case node.Aggs == nil:
			path = defaultPathComponent
			foundMetric = true
		case node.Aggs.MetricsAggOnNumericField != nil:
			path = MetricsAggOnNumericField
			foundMetric = true
		case node.Aggs.MetricsConversationCount != nil:
			path = MetricsConversationCount
			foundMetric = true
		case node.Aggs.MetricsConversationMessageCount != nil:
			path = MetricsConversationMessageCount
			foundMetric = true
		case node.Aggs.BucketAggsForOrderByMetrics != nil:
			path = BucketAggsForOrderByMetrics
			node = node.Aggs.BucketAggsForOrderByMetrics
		case node.Aggs.BucketAggs != nil:
			path = BucketAggs
			node = node.Aggs.BucketAggs
		default:
			path = defaultPathComponent
			foundMetric = true
		}
		sortValuePath = append(sortValuePath, path)
	}

	return sortValuePath, nil
}

func correctOrderByForTermsAgg(chartDataConfig *dashboardpb.ChartDataConfig, metricsAgg, mergedAggs *ChartESAggregation) error {
	if len(chartDataConfig.Metrics) > 1 {
		return status.Errorf(codes.InvalidArgument, "multiple metrics not supported")
	}

	// Fall back to sort by default doc_count for occurrence-count metrics, where the sorting logic is
	// not well defined. Multiple scorecard templates and criteria are handled instead by ordering on a
	// sibling aggregate over every selected template/criterion (metricsAgg.orderByBucketAggsOverride).
	if metricsAggUsesDocCount(metricsAgg) {
		return nil
	}

	var termsAgg *elasticsearch.TermsAggs
	var parentOfTermsAgg *BucketAggregation

	for mergedAggs != nil && mergedAggs.BucketAggs != nil {
		if mergedAggs.BucketAggs.Terms != nil {
			parentOfTermsAgg = mergedAggs.BucketAggs
			termsAgg = mergedAggs.BucketAggs.Terms
			break
		}
		mergedAggs = mergedAggs.BucketAggs.Aggs
	}
	// Do nothing when there is no terms aggregation.
	if termsAgg == nil {
		return nil
	}

	var orderByDirection string
	if l := len(termsAgg.OrderBy); l != 1 {
		return fmt.Errorf("terms_agg.order_by is only supposed to have 1 key|value pair, but found %v", l)
	}
	for _, v := range termsAgg.OrderBy {
		orderByDirection = v
	}

	// TODO(zhucaiyi): not sure how to handle averge occurrence per convo.
	sortValuePath, err := getSortValuePathFromNode(parentOfTermsAgg, DocumentCount)
	if err != nil {
		return err
	}

	orderByStringPaths := make([]string, 0, len(sortValuePath))

	for _, pathComponent := range sortValuePath {
		var path string
		switch pathComponent {
		case DocumentCount:
			path = defaultOrderByPath
		case MetricsAggOnNumericField:
			path = "metrics_agg_on_numeric_field"
		case MetricsConversationCount:
			path = "metrics_conversation_count"
		case MetricsConversationMessageCount:
			path = "metrics_conversation_message_count"
		case BucketAggsForOrderByMetrics:
			path = "bucket_aggs_for_order_by_metrics"
		case BucketAggs:
			path = "bucket_aggs"
		default:
			path = defaultOrderByPath
		}
		orderByStringPaths = append(orderByStringPaths, path)
	}

	termsAgg.OrderBy = map[string]string{
		strings.Join(orderByStringPaths, ">"): orderByDirection,
	}
	return nil
}

func metricsAggIsOccurrenceCount(metricsAgg *ChartESAggregation) bool {
	nodeForMetricsAgg := metricsAgg
	for nodeForMetricsAgg != nil {
		if nodeForMetricsAgg.isTotalOccurrenceCount || nodeForMetricsAgg.isAverageOccurrenceCountPerConvo {
			return true
		}
		if nodeForMetricsAgg.BucketAggs == nil {
			break
		}
		nodeForMetricsAgg = nodeForMetricsAgg.BucketAggs.Aggs
	}
	return false
}

func metricsAggUsesDocCount(metricsAgg *ChartESAggregation) bool {
	nodeForMetricsAgg := metricsAgg
	for nodeForMetricsAgg != nil {
		if nodeForMetricsAgg.isTotalOccurrenceCount || nodeForMetricsAgg.isConversationCountViaDocCount {
			return true
		}
		if nodeForMetricsAgg.BucketAggs == nil {
			break
		}
		nodeForMetricsAgg = nodeForMetricsAgg.BucketAggs.Aggs
	}
	return false
}

type esAggType int

// Define the enum values as constants
const (
	multiBucketAggType esAggType = iota
	metricsAggType
)

// Traverse to a node with desired field populated: metrics field or "Buckets" field.
func traverseToDesiredAggInESResp(node *ESRespSingleBucket, aggType esAggType) (*ESRespSingleBucket, error) {
	found := false
	for !found {
		switch {
		// Found multi-bucket agg.
		case aggType == multiBucketAggType && node.Buckets != nil:
			found = true
		// Found metrics agg.
		case aggType == metricsAggType && (node.MetricsConversationMessageCount != nil || node.MetricsConversationCount != nil || node.MetricsAggOnNumericField != nil || node.MetricsBucketFieldCardinality != nil):
			found = true
		// When looking for metrics, prefer BucketAggsForOrderByMetrics over BucketAggs.
		// This handles cases where both fields are present: BucketAggsForOrderByMetrics
		// contains the metrics for ordering, while BucketAggs may point to another
		// bucket aggregation (e.g., date_histogram) without metrics.
		case aggType == metricsAggType && node.BucketAggsForOrderByMetrics != nil:
			node = node.BucketAggsForOrderByMetrics
		// Response for single bucket aggregation (standard).
		case node.BucketAggs != nil:
			node = node.BucketAggs
		// Response for single bucket aggregation (orderby metrics) - for non-metrics traversal.
		case node.BucketAggsForOrderByMetrics != nil:
			node = node.BucketAggsForOrderByMetrics
		// When no metrics agg, instead built-in metrics doc_count is being used.
		case aggType == metricsAggType:
			found = true
		default:
			return nil, status.Errorf(codes.Internal, "target aggregation type %v, traversing ES response at node: %+v", aggType, node)
		}
	}

	return node, nil
}

func traverseToDesiredAggInESReq(node *ChartESAggregation, aggType esAggType) (*ChartESAggregation, error) {
	if aggType != metricsAggType {
		return nil, fmt.Errorf("currently only support traversing for metrics agg")
	}
	found := false
	for !found {
		switch {
		case node.MetricsConversationCount != nil || node.MetricsConversationMessageCount != nil || node.MetricsAggOnNumericField != nil || node.isTotalOccurrenceCount || node.isConversationCountViaDocCount:
			found = true
		case node.BucketAggs != nil:
			node = node.BucketAggs.Aggs
		default:
			return nil, fmt.Errorf("does not found target aggregation %v", aggType)
		}
	}
	return node, nil
}

func (a AnalyticsServiceImpl) convertESResponse(ctx context.Context, customerProfile *shared.CustomerProfile, documentType elasticsearch.DocumentType, groupBys []*dashboardpb.ChartGroupBy, metricsAggregation *ChartESAggregation, groupbyShuffled bool, metricDerivedInnerGroupBy bool, esResponse ChartESResponse) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	var err error
	// Find the leaf node of metricsAggregation that contains a direct field which is metrics agg.
	metricsAggregation, err = traverseToDesiredAggInESReq(metricsAggregation, metricsAggType)
	if err != nil {
		return nil, err
	}
	readESMetricsValue := func(node ESRespSingleBucket) (*ESRespMetricsValue, error) {
		return esMetricsValueFromBucket(&node, metricsAggregation, len(groupBys), documentType)
	}
	// Handle special case of 0 groupby.
	if len(groupBys) == 0 {
		metricsVal, err := esMetricsValueWithoutGroupBy(metricsAggregation, esResponse, documentType)
		if err != nil {
			return nil, err
		}
		analyticsNumbers := []*dashboardpb.AnalyticsResult_AnalyticsNumber{}
		if analyticsNumber := analyticsNumbersValue(*metricsVal, nil, metricsAggregation.omitNullMetricValue); analyticsNumber != nil {
			analyticsNumbers = append(analyticsNumbers, analyticsNumber)
		}
		return &analyticspb.EvaluateAnalyticsChartResponse{
			ResultForChart: &dashboardpb.AnalyticsResultForChart{
				AnalyticsResults: []*dashboardpb.AnalyticsResult{
					{
						AnalyticsNumbers: analyticsNumbers,
					},
				},
			},
			UpdateTimestamp: timestamppb.New(time.Now()),
		}, nil
	}

	groupByValuesForAllKeys := []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{}
	for i, groupBy := range groupBys {
		groupByValueForSingleKey := &dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{}
		switch groupBy.GroupByMode.(type) {
		case *dashboardpb.ChartGroupBy_ByProvidedValue:
			cvse, err := fromChartValueSet(ctx, groupBy.GetByProvidedValue().GetValueSet())
			if err != nil {
				return nil, err
			}
			switch cvse.attributeValueType {
			case crestaEntityAgentAssistance:
				for _, aa := range cvse.agentAssistances {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
								AgentAssistanceValue: aa,
							},
						},
					})
				}
			case crestaEntitySubcategory:
				for _, sub := range cvse.subcategories {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: sub,
							},
						},
					})
				}
				for _, sub := range cvse.subcategoriesExclude {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: sub,
							},
						},
					})
				}
			case crestaEntityAggregateSubcategory:
				for _, aggSub := range cvse.aggregateSubcategories {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: aggSub,
							},
						},
					})
				}
				for _, aggSub := range cvse.aggregateSubcategoriesExclude {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: aggSub,
							},
						},
					})
				}
			case crestaEntityCluster:
				for _, c := range cvse.clusters {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ClusterValue{
								ClusterValue: c,
							},
						},
					})
				}
			case crestaEntityOperaBlock:
				for _, ob := range cvse.operaBlocks {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
								OperaBlockValue: ob,
							},
						},
					})
				}
			case crestaEntityBehavior:
				for _, b := range cvse.behaviors {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
								BehaviorValue: b,
							},
						},
					})
				}
			case crestaEntityEmotion:
				for _, em := range cvse.emotions {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_EmotionValue{
								EmotionValue: em,
							},
						},
					})
				}
			case crestaEntitySentiment:
				for _, s := range cvse.sentiments {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_SentimentValue{
								SentimentValue: s,
							},
						},
					})
				}
			case crestaEntityMetadata:
				for _, md := range cvse.metadatas {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_MetadataValue{
								MetadataValue: md,
							},
						},
					})
				}
			case crestaEntityLlmAnnotation:
				for _, llmAnnotation := range cvse.llmAnnotations {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
								LlmAnnotation: llmAnnotation,
							},
						},
					})
				}
			case attributeValueTypeScorecardTemplate:
				for _, st := range cvse.scorecardTemplates {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
								ScorecardTemplateValue: st,
							},
						},
					})
				}
			case attributeValuetypeScorecardCriterion:
				for _, crit := range cvse.scorecardCriterions {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
								ScorecardCriterionValue: crit,
							},
						},
					})
				}
			case attributeValueTypeAgentName:
				for _, agentName := range cvse.agentNames {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_UserNameValue{
								UserNameValue: agentName,
							},
						},
					})
				}
			case attributeValueTypeGroupName:
				for _, groupName := range cvse.groupNames {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_GroupNameValue{
								GroupNameValue: groupName,
							},
						},
					})
				}
			case attributeValueTypeUsecaseName:
				for _, usecaseName := range cvse.usecaseNames {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
								UsecaseNameValue: usecaseName,
							},
						},
					})
				}
			case attributeValueTypeConversationHandleTimeRange:
				for _, timeRange := range cvse.conversationHandleTimeRanges {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
								ConversationHandleTimeRange: timeRange,
							},
						},
					})
				}
			case attributeValueTypeConversationMessageCountRange:
				for _, msgCountRange := range cvse.conversationMessageCountRanges {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ConversationMessageCountRange{
								ConversationMessageCountRange: msgCountRange,
							},
						},
					})
				}
			case attributeValueTypeConversationNumHoldsRange:
				for _, numHoldsRange := range cvse.conversationNumHoldsRanges {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
								ConversationNumHoldsRange: numHoldsRange,
							},
						},
					})
				}
			case crestaEntityAggregateAndLeafSubcategory:
				for _, sub := range cvse.subcategories {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: sub,
							},
						},
					})
				}
				for _, aggSub := range cvse.aggregateSubcategories {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: aggSub,
							},
						},
					})
				}
				for _, sub := range cvse.subcategoriesExclude {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: sub,
							},
						},
					})
				}
				for _, aggSub := range cvse.aggregateSubcategoriesExclude {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: aggSub,
							},
						},
					})
				}
			case attributeValueTypeConversationSource:
				for _, cs := range cvse.conversationSources {
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_ConversationSource{
								ConversationSource: conversation.Conversation_Source(cs),
							},
						},
					})
				}
			default:
				return nil, status.Errorf(codes.InvalidArgument, "group by provided value does not support type %v", cvse.attributeValueType)
			}
		case *dashboardpb.ChartGroupBy_ByTime:
			switch i {
			case 0:
				node, err := traverseToDesiredAggInESResp(esResponse.Aggregations, multiBucketAggType)
				if err != nil {
					return nil, err
				}
				for _, b := range *node.Buckets {
					timestamp := shared.ConvertTimeToAPITimestamp(time.Unix(int64(b.Key.(float64)/1000), 0))
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_Timestamp{
								Timestamp: timestamp,
							},
						},
					})
				}
			case 1:
				node1, err := traverseToDesiredAggInESResp(esResponse.Aggregations, multiBucketAggType)
				if err != nil {
					return nil, err
				}
				// When grouping by unique values, ES may returns empty buckets if there is 0 filtered convos.
				if len(*node1.Buckets) > 0 {
					node2, err := traverseToDesiredAggInESResp(&(*node1.Buckets)[0], multiBucketAggType)
					if err != nil {
						return nil, err
					}
					for _, b := range *node2.Buckets {
						timestamp := shared.ConvertTimeToAPITimestamp(time.Unix(int64(b.Key.(float64)/1000), 0))
						groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
							Value: &dashboardpb.ChartAttributeValue{
								Value: &dashboardpb.ChartAttributeValue_Timestamp{
									Timestamp: timestamp,
								},
							},
						})
					}
				}
			}
		case *dashboardpb.ChartGroupBy_ByUniqueValue:
			if i != 0 {
				return nil, status.Errorf(codes.InvalidArgument, "only support grouping by unique value in first groupby")
			}
			// Traverse to muti-buckets aggregation.
			node, err := traverseToDesiredAggInESResp(esResponse.Aggregations, multiBucketAggType)
			if err != nil {
				return nil, err
			}

			byUniqueValue := groupBy.GetByUniqueValue()
			switch byUniqueValue.UniqueValueFrom.(type) {
			case *dashboardpb.ChartGroupBy_UniqueValue_Metadata:
				// TODO(zhucaiyi): avoid calling BatchGetMoments twice.
				metadataValueField := ""
				if byUniqueValue.GetMetadata() != nil {
					fields, err := a.metadataValueFieldForChart(ctx, customerProfile.String(), []string{byUniqueValue.GetMetadata().MomentName})
					if err != nil {
						return nil, err
					}
					metadataValueField = fields[0]
				}
				for _, b := range *node.Buckets {
					var metadataValue *dashboardpb.MetadataForChart_MetadataValue
					switch metadataValueField {
					case elasticsearch.ConversationMetadataPayloadStringValueField:
						metadataValue = &dashboardpb.MetadataForChart_MetadataValue{
							Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
								StringValue: b.Key.(string),
							},
						}
					case elasticsearch.ConversationMetadataPayloadNumberValueField, elasticsearch.ConversationMetadataPayloadNumberValueFieldExplicitMapping:
						val, ok := safeCastToFloat32(b.Key)
						if !ok {
							return nil, status.Errorf(codes.Internal, "failed to cast metadata value to float32")
						}
						metadataValue = &dashboardpb.MetadataForChart_MetadataValue{
							Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
								NumberValue: val,
							},
						}
					default:
						return nil, status.Errorf(codes.Internal, "invalid metadata value field for groupby unique value %v", metadataValueField)
					}
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_MetadataValue{
								MetadataValue: &dashboardpb.MetadataForChart{
									MomentName: byUniqueValue.GetMetadata().MomentName,
									Values: []*dashboardpb.MetadataForChart_MetadataValue{
										metadataValue,
									},
								},
							},
						},
					})
				}

			case *dashboardpb.ChartGroupBy_UniqueValue_LlmAnnotation:
				llmAnnotation := byUniqueValue.GetLlmAnnotation()
				valueField, err := a.llmAnnotationValueFieldForChart(ctx, llmAnnotation.MomentName)
				if err != nil {
					return nil, err
				}
				for _, b := range *node.Buckets {
					var annotationValue *dashboardpb.LlmAnnotationForChart_AnnotationValue
					switch valueField {
					case elasticsearch.LlmAnnotationPayloadStringValueField:
						val, ok := b.Key.(string)
						if !ok {
							return nil, status.Errorf(codes.Internal, "failed to cast llm annotation value to string")
						}
						annotationValue = &dashboardpb.LlmAnnotationForChart_AnnotationValue{
							Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
								StringValue: val,
							},
						}
					case elasticsearch.LlmAnnotationPayloadNumberValueField, elasticsearch.LlmAnnotationPayloadNumberValueFieldExplicitMapping:
						val, ok := safeCastToFloat32(b.Key)
						if !ok {
							return nil, status.Errorf(codes.Internal, "failed to cast llm annotation value to float32")
						}
						annotationValue = &dashboardpb.LlmAnnotationForChart_AnnotationValue{
							Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_NumberValue{
								NumberValue: val,
							},
						}
					case elasticsearch.LlmAnnotationPayloadBooleanValueField:
						// ES returns a boolean terms-agg key as a bool or numeric 1/0.
						boolVal := false
						switch k := b.Key.(type) {
						case bool:
							boolVal = k
						default:
							val, ok := safeCastToFloat32(b.Key)
							if !ok {
								return nil, status.Errorf(codes.Internal, "failed to interpret llm annotation boolean value %v", b.Key)
							}
							boolVal = val != 0
						}
						annotationValue = &dashboardpb.LlmAnnotationForChart_AnnotationValue{
							Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_BooleanValue{
								BooleanValue: boolVal,
							},
						}
					default:
						return nil, status.Errorf(codes.Internal, "invalid llm annotation value field for groupby unique value %v", valueField)
					}
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
								LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
									MomentName: llmAnnotation.MomentName,
									Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
										annotationValue,
									},
								},
							},
						},
					})
				}

			case *dashboardpb.ChartGroupBy_UniqueValue_Attribute_:
				for _, b := range *node.Buckets {
					agentUserID := b.Key.(string)
					groupByValueForSingleKey.GroupByValues = append(groupByValueForSingleKey.GroupByValues, &dashboardpb.AnalyticsResult_GroupByValue{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_UserNameValue{
								UserNameValue: userpb.UserName{
									CustomerID: customerProfile.CustomerID,
									UserID:     agentUserID,
								}.String(),
							},
						},
					})
				}
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupport groupby type %t", groupBy.GroupByMode)
		}
		if len(groupByValueForSingleKey.GroupByValues) > 0 {
			groupByValuesForAllKeys = append(groupByValuesForAllKeys, groupByValueForSingleKey)
		}
	}

	analyticsNumbers := []*dashboardpb.AnalyticsResult_AnalyticsNumber{}
	node, err := traverseToDesiredAggInESResp(esResponse.Aggregations, multiBucketAggType)
	if err != nil {
		return nil, err
	}

	needManualSort := needManualSortFirstGroupby(groupBys)
	// Order-by values for first-groupby.
	orderByMetricValues := []ESRespMetricsValue{}
	for i, b1 := range *node.Buckets {
		// The chart has only 1 group-by.
		if len(groupBys) == 1 {
			groupByValueIndexes := []int32{int32(i)}
			metricValue, err := readESMetricsValue(b1)
			if err != nil {
				return nil, err
			}
			if needManualSort {
				orderByMetricValues = append(orderByMetricValues, *metricValue)
			}
			if analyticsNumber := analyticsNumbersValue(*metricValue, groupByValueIndexes, metricsAggregation.omitNullMetricValue); analyticsNumber != nil {
				analyticsNumbers = append(analyticsNumbers, analyticsNumber)
			}
		} else {
			// Non-derived inner groupby (e.g. ForConversation + byTime): the outer
			// orderBy copy IS the metric value, so rank by it directly (INSI-4358).
			if needManualSort && !metricDerivedInnerGroupBy {
				metricValue, err := readESMetricsValue(b1)
				if err != nil {
					return nil, err
				}
				orderByMetricValues = append(orderByMetricValues, *metricValue)
			}
			node2, err := traverseToDesiredAggInESResp(&b1, multiBucketAggType)
			if err != nil {
				return nil, err
			}
			// The chart has 2 groupby.
			var firstGroupbyMetricSum float64
			firstGroupbyMetricHasValue := false
			for j, b2 := range *node2.Buckets {
				metricValue, err := readESMetricsValue(b2)
				if err != nil {
					return nil, err
				}
				if analyticsNumber := analyticsNumbersValue(*metricValue, []int32{int32(i), int32(j)}, metricsAggregation.omitNullMetricValue); analyticsNumber != nil {
					analyticsNumbers = append(analyticsNumbers, analyticsNumber)
				}
				// Derived inner groupby: rank the outer groupby by the inner buckets'
				// sum (the charted volume). The outer orderBy read returns 0 here (see
				// the metricDerivedInnerGroupBy comment above), so the inner sum is used.
				if !needManualSort || !metricDerivedInnerGroupBy {
					continue
				}
				if value, ok := metricValueForSort(*metricValue, metricsAggregation.omitNullMetricValue); ok {
					firstGroupbyMetricSum += value
					firstGroupbyMetricHasValue = true
				}
			}
			if needManualSort && metricDerivedInnerGroupBy {
				sortValue := ESRespMetricsValue{}
				if firstGroupbyMetricHasValue {
					sortValue = *newESRespMetricsValue(firstGroupbyMetricSum)
				}
				orderByMetricValues = append(orderByMetricValues, sortValue)
			}
		}
	}
	if needManualSort {
		type Pair struct {
			index int
			value ESRespMetricsValue
		}
		var pairs []Pair
		for idx, val := range orderByMetricValues {
			pairs = append(pairs, Pair{
				index: idx,
				value: val,
			})
		}
		// The index of the Top|Bottom N groupby values.
		var idxes []int
		config := groupBys[0].Config
		if config.SortDirection == dashboardpb.ChartGroupBy_Config_ASC {
			// Sort pairs by value descending
			sort.SliceStable(pairs, func(i, j int) bool {
				left, leftOK := metricValueForSort(pairs[i].value, metricsAggregation.omitNullMetricValue)
				right, rightOK := metricValueForSort(pairs[j].value, metricsAggregation.omitNullMetricValue)
				if leftOK != rightOK {
					return leftOK
				}
				return left < right
			})
		} else {
			// Sort pairs by value descending
			sort.SliceStable(pairs, func(i, j int) bool {
				left, leftOK := metricValueForSort(pairs[i].value, metricsAggregation.omitNullMetricValue)
				right, rightOK := metricValueForSort(pairs[j].value, metricsAggregation.omitNullMetricValue)
				if leftOK != rightOK {
					return leftOK
				}
				return left > right
			})
		}
		for i := 0; i < len(pairs) && i < int(config.ValueCountLimit); i++ {
			idxes = append(idxes, pairs[i].index)
		}
		newGroupByValues := make([]*dashboardpb.AnalyticsResult_GroupByValue, len(idxes))
		newAnalyticsNumbers := []*dashboardpb.AnalyticsResult_AnalyticsNumber{}
		for i, idx := range idxes {
			for j, val := range groupByValuesForAllKeys[0].GroupByValues {
				if j == idx {
					newGroupByValues[i] = val
				}
			}
			for _, v := range analyticsNumbers {
				val := v
				if val.GroupByValueIndexes[0] != int32(idx) {
					continue
				}

				cloned := proto.Clone(val).(*dashboardpb.AnalyticsResult_AnalyticsNumber)
				cloned.GroupByValueIndexes[0] = int32(i)

				newAnalyticsNumbers = append(newAnalyticsNumbers, cloned)
			}
		}
		groupByValuesForAllKeys[0].GroupByValues = newGroupByValues
		analyticsNumbers = newAnalyticsNumbers
	}

	// TODO: support Chart with multiple Metrics.
	resp := &analyticspb.EvaluateAnalyticsChartResponse{
		ResultForChart: &dashboardpb.AnalyticsResultForChart{
			AnalyticsResults: []*dashboardpb.AnalyticsResult{
				{
					GroupByValuesForAllKeys: groupByValuesForAllKeys,
					AnalyticsNumbers:        analyticsNumbers,
				},
			},
		},
	}
	// Restore groupby back if groupby is shuffled.
	// Namely 1st and 2nd groupby were shuffled when generating ES query.
	if groupbyShuffled {
		groupbys, analyticsNumbers := resp.ResultForChart.AnalyticsResults[0].GroupByValuesForAllKeys, resp.ResultForChart.AnalyticsResults[0].AnalyticsNumbers
		groupbys[0], groupbys[1] = groupbys[1], groupbys[0]

		for _, val := range analyticsNumbers {
			val.GroupByValueIndexes[0], val.GroupByValueIndexes[1] = val.GroupByValueIndexes[1], val.GroupByValueIndexes[0]
		}

		sort.Slice(analyticsNumbers, func(i, j int) bool {
			idx1, idx2 := analyticsNumbers[i].GroupByValueIndexes, analyticsNumbers[j].GroupByValueIndexes
			return idx1[0]*int32(len(analyticsNumbers))+idx1[1] < idx2[0]*int32(len(analyticsNumbers))+idx2[1]
		})
	}

	resp.UpdateTimestamp = timestamppb.New(time.Now())
	return resp, nil
}

// This function is used to convert the ES metrics value to the analytics number value.
// If the ES metrics contains elements in the Values map:
//   - If there is only one element, it will return it as a single value.
//   - If there are multiple elements, we assume that the values are the box plot values.
//
// If the ES metrics Values map is empty, it will return the single value.
func analyticsNumbersValue(esRespMetricsValue ESRespMetricsValue, groupByValueIndexes []int32, omitNullMetricValue bool) *dashboardpb.AnalyticsResult_AnalyticsNumber {
	if len(esRespMetricsValue.Values) == 1 {
		for k := range esRespMetricsValue.Values {
			return &dashboardpb.AnalyticsResult_AnalyticsNumber{
				Value:               float32(esRespMetricsValue.Values[k]),
				GroupByValueIndexes: groupByValueIndexes,
			}
		}
	} else if len(esRespMetricsValue.Values) > 1 {
		boxPlotValues := &dashboardpb.AnalyticsResult_BoxPlotValues{}
		if v, ok := esRespMetricsValue.Values["0.0"]; ok {
			boxPlotValues.P0 = float32(v)
		}
		if v, ok := esRespMetricsValue.Values["25.0"]; ok {
			boxPlotValues.P25 = float32(v)
		}
		if v, ok := esRespMetricsValue.Values["50.0"]; ok {
			boxPlotValues.P50 = float32(v)
		}
		if v, ok := esRespMetricsValue.Values["75.0"]; ok {
			boxPlotValues.P75 = float32(v)
		}
		if v, ok := esRespMetricsValue.Values["100.0"]; ok {
			boxPlotValues.P100 = float32(v)
		}
		return &dashboardpb.AnalyticsResult_AnalyticsNumber{
			BoxPlotValues:       boxPlotValues,
			GroupByValueIndexes: groupByValueIndexes,
		}
	}
	value, ok := (&esRespMetricsValue).numericValue()
	if !ok {
		if !omitNullMetricValue {
			value = 0
		} else {
			return nil
		}
	}
	return &dashboardpb.AnalyticsResult_AnalyticsNumber{
		Value:               float32(value),
		GroupByValueIndexes: groupByValueIndexes,
	}
}

func metricValueForSort(esRespMetricsValue ESRespMetricsValue, omitNullMetricValue bool) (float64, bool) {
	value, ok := (&esRespMetricsValue).numericValue()
	if ok {
		return value, true
	}
	if omitNullMetricValue {
		return 0, false
	}
	return 0, true
}

func (a AnalyticsServiceImpl) getCachedRequestProto(ctx context.Context, req *analyticspb.EvaluateAnalyticsChartRequest, authEntity *auth.PrincipalEntity) *analyticspb.EvaluateAnalyticsChartCachedRequest {
	analyticsChartForCacheKey := proto.Clone(req.AnalyticsChart).(*dashboardpb.AnalyticsChart)
	if err := sortAndDeduplicateFilters(analyticsChartForCacheKey); err != nil {
		a.logger.Warnf(ctx, "failed to sort and deduplicate filters: %v", err)
	}
	innerRequestForCacheKey := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent:         req.Parent,
		AnalyticsChart: trimRequest(analyticsChartForCacheKey),
		ConvoFilter:    req.GetConvoFilter(),
	}

	if authEntity.IsServer() ||
		authEntity.HasAnyRole(
			authpb.AuthProto_ADMIN,
			authpb.AuthProto_SUPER_ADMIN,
			authpb.AuthProto_CI_ADMIN,
			authpb.AuthProto_INSIGHTS_VIEWER,
			authpb.AuthProto_QA_ADMIN,
			authpb.AuthProto_OPERA_ADMIN) {
		return &analyticspb.EvaluateAnalyticsChartCachedRequest{
			ChartRequest: innerRequestForCacheKey,
			AclConfig: &analyticspb.EvaluateAnalyticsChartCachedRequest_IsAdmin{
				IsAdmin: true,
			},
		}
	}

	return &analyticspb.EvaluateAnalyticsChartCachedRequest{
		ChartRequest: innerRequestForCacheKey,
		AclConfig: &analyticspb.EvaluateAnalyticsChartCachedRequest_IsAdmin{
			IsAdmin: false, // TODO: Populate non-admin ACL if caching for non-admin users is to be supported.
		},
	}
}

func (a AnalyticsServiceImpl) isCachingEnabled(insightFeatureConfig *features.Insights) bool {
	if *enableCachingInEvaluateAnalyticsChart {
		return true
	}
	return insightFeatureConfig.GetEnableDashboardBuilderCaching()
}

func sortAndDeduplicateFilters(analyticsChartForCacheKey *dashboardpb.AnalyticsChart) error {
	dc := analyticsChartForCacheKey.GetDataConfig()
	if dc == nil || len(dc.Filters) == 0 {
		return nil
	}
	opts := proto.MarshalOptions{Deterministic: true}
	filterKey := make(map[*dashboardpb.ChartDataFilter][]byte, len(dc.Filters))
	seen := make(map[string]struct{}, len(dc.Filters))
	unique := make([]*dashboardpb.ChartDataFilter, 0, len(dc.Filters))
	for _, f := range dc.Filters {
		b, err := opts.Marshal(f)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to marshal filter: %v", err)
		}
		filterKey[f] = b
		key := string(b)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, f)
	}
	dc.Filters = unique
	if len(dc.Filters) > 1 {
		sort.SliceStable(dc.Filters, func(i, j int) bool {
			return bytes.Compare(filterKey[dc.Filters[i]], filterKey[dc.Filters[j]]) < 0
		})
	}
	return nil
}

func trimRequest(analyticsChartForCacheKey *dashboardpb.AnalyticsChart) *dashboardpb.AnalyticsChart {
	analyticsChartForCacheKey.RenderingConfig = nil
	return analyticsChartForCacheKey
}
