package analyticsimpl

import (
	"context"
	"slices"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/util/errgroup"
	"github.com/cresta/shared-go/util/sync/future"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// This is a wrapper api around EvaluateAnalyticsChart that adds the ability to fetch percentage related data.
// Prior to this, frontend had to orchestrate two additional calls to EvaluateAnalyticsChart, to retrieve percentage
// related data.
// Since we want to be able to replicate the same logic when generating the charts in the backend, and don't want to
// have duplicated orchestration logic, we've added this api.
func (a AnalyticsServiceImpl) RetrieveChartEvaluationData(ctx context.Context, req *analyticspb.RetrieveChartEvaluationDataRequest) (*analyticspb.RetrieveChartEvaluationDataResponse, error) {
	// Combo charts (exactly 2 metrics) return two aligned main results, which breaks the
	// single-metric percentage-baseline fan-out below (its sub-requests each return one result,
	// so the equal-count check would fail). Combo requests compute per-metric baselines instead.
	if isComboChartRequest(req.GetEvaluateAnalyticsChartRequest()) {
		return a.retrieveComboChartEvaluationData(ctx, req)
	}

	eg := errgroup.New()

	// We need to clone the request to avoid modifying the original request. Because inside EvaluateAnalyticsChart we modify the request in place.
	copiedEvaluateAnalyticsChartReq := proto.Clone(req.GetEvaluateAnalyticsChartRequest()).(*analyticspb.EvaluateAnalyticsChartRequest)
	evaluateAnalyticsChartFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
		return a.EvaluateAnalyticsChart(ctx, copiedEvaluateAnalyticsChartReq)
	})

	var evaluateAnalyticsChartResults []*dashboardingpb.AnalyticsResult
	var ungroupedTotals []int32
	var groupsAsFiltersTotals []int32
	var primaryGroupOnlyTotals []*dashboardingpb.AnalyticsResult

	if req.GetFetchPercentageRelatedData() {
		// Start parallel EvaluateAnalyticsChartCalls
		ungroupedReq := getUngroupedRequest(req.GetEvaluateAnalyticsChartRequest())
		ungroupedFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
			return a.EvaluateAnalyticsChart(ctx, ungroupedReq)
		})

		groupsAsFiltersReq := getGroupsAsFiltersRequest(req.GetEvaluateAnalyticsChartRequest())
		groupsAsFiltersFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
			return a.EvaluateAnalyticsChart(ctx, groupsAsFiltersReq)
		})

		// TODO: Ideally, we should use either ungroup request or primaryGroupOnlyReq, based on some flag.
		// Also groupsAsFiltersReq is not needed only for PIE charts.
		primaryGroupOnlyReq := getPrimaryGroupOnlyRequest(req.GetEvaluateAnalyticsChartRequest())
		primaryGroupOnlyFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
			return a.EvaluateAnalyticsChart(ctx, primaryGroupOnlyReq)
		})

		// Wait for everything to finish, check for any errors
		if err := eg.Wait(); err != nil {
			return nil, err
		}

		evaluateAnalyticsChartResp, err := evaluateAnalyticsChartFuture.Result()
		if err != nil {
			a.logger.Errorf(
				ctx,
				"(RetrieveChartEvaluationData) main request failed (chart=%q, parent=%q, fetch_percentage_related_data=%t): %s",
				req.GetEvaluateAnalyticsChartRequest().GetAnalyticsChart().GetName(),
				req.GetEvaluateAnalyticsChartRequest().GetParent(),
				req.GetFetchPercentageRelatedData(),
				err.Error(),
			)
			return nil, err
		}
		evaluateAnalyticsChartResults = evaluateAnalyticsChartResp.GetResultForChart().GetAnalyticsResults()
		ungroupedResp, err := ungroupedFuture.Result()
		if err != nil {
			a.logger.Errorf(
				ctx,
				"(RetrieveChartEvaluationData) ungrouped request failed (chart=%q, parent=%q): %s",
				req.GetEvaluateAnalyticsChartRequest().GetAnalyticsChart().GetName(),
				req.GetEvaluateAnalyticsChartRequest().GetParent(),
				err.Error(),
			)
			return nil, err
		}
		ungroupedResults := ungroupedResp.GetResultForChart().GetAnalyticsResults()

		groupsAsFiltersResp, err := groupsAsFiltersFuture.Result()
		if err != nil {
			a.logger.Errorf(
				ctx,
				"(RetrieveChartEvaluationData) groups-as-filters request failed (chart=%q, parent=%q): %s",
				req.GetEvaluateAnalyticsChartRequest().GetAnalyticsChart().GetName(),
				req.GetEvaluateAnalyticsChartRequest().GetParent(),
				err.Error(),
			)
			return nil, err
		}
		groupsAsFiltersResults := groupsAsFiltersResp.GetResultForChart().GetAnalyticsResults()

		primaryGroupOnlyResp, err := primaryGroupOnlyFuture.Result()
		if err != nil {
			a.logger.Errorf(
				ctx,
				"(RetrieveChartEvaluationData) primary-group-only request failed (chart=%q, parent=%q): %s",
				req.GetEvaluateAnalyticsChartRequest().GetAnalyticsChart().GetName(),
				req.GetEvaluateAnalyticsChartRequest().GetParent(),
				err.Error(),
			)
			return nil, err
		}
		primaryGroupOnlyResults := primaryGroupOnlyResp.GetResultForChart().GetAnalyticsResults()

		// Check all responses have the same result count
		mainResultsCount := len(evaluateAnalyticsChartResults)
		otherResultCounts := map[string]int{
			"ungrouped":          len(ungroupedResults),
			"groups-as-filters":  len(groupsAsFiltersResults),
			"primary group only": len(primaryGroupOnlyResults),
		}
		for requestName, count := range otherResultCounts {
			if count != mainResultsCount {
				return nil, status.Errorf(codes.Internal, "unexpected number of results for %s request, expected %d, got %d", requestName, mainResultsCount, count)
			}
		}

		// Write results in slices to be used for return value
		ungroupedTotals = make([]int32, mainResultsCount)
		for i, result := range ungroupedResults {
			ungroupedTotals[i] = sumAnalyticsNumbersForAnalyticsResult(result)
		}

		groupsAsFiltersTotals = make([]int32, mainResultsCount)
		for i, result := range groupsAsFiltersResults {
			groupsAsFiltersTotals[i] = sumAnalyticsNumbersForAnalyticsResult(result)
		}

		primaryGroupOnlyTotals = make([]*dashboardingpb.AnalyticsResult, mainResultsCount)
		copy(primaryGroupOnlyTotals, primaryGroupOnlyResults)

	} else {
		// We only have one EvaluateAnalyticsChart call, wait to finish and assign default values to other totals slices
		if err := eg.Wait(); err != nil {
			return nil, err
		}
		evaluateAnalyticsChartResp, err := evaluateAnalyticsChartFuture.Result()
		if err != nil {
			return nil, err
		}

		evaluateAnalyticsChartResults = evaluateAnalyticsChartResp.GetResultForChart().GetAnalyticsResults()
		resultsCount := len(evaluateAnalyticsChartResults)
		ungroupedTotals = make([]int32, resultsCount)
		groupsAsFiltersTotals = make([]int32, resultsCount)
		primaryGroupOnlyTotals = make([]*dashboardingpb.AnalyticsResult, resultsCount)
	}

	metricsEvaluationData := []*dashboardingpb.AnalyticsChartEvaluationData_MetricEvaluationData{}
	for i, result := range evaluateAnalyticsChartResults {
		metricsEvaluationData = append(metricsEvaluationData, &dashboardingpb.AnalyticsChartEvaluationData_MetricEvaluationData{
			AnalyticsResult:         result,
			UngroupedTotal:          ungroupedTotals[i],
			GroupsAsFiltersTotal:    groupsAsFiltersTotals[i],
			GroupedByPrimaryResults: primaryGroupOnlyTotals[i],
		})
	}

	return &analyticspb.RetrieveChartEvaluationDataResponse{
		EvaluationData: &dashboardingpb.AnalyticsChartEvaluationData{
			MetricsEvaluationData: metricsEvaluationData,
		},
	}, nil
}

// getUngroupedRequest creates a request with no groupBy clauses and a conversation count metric
func getUngroupedRequest(originalReq *analyticspb.EvaluateAnalyticsChartRequest) *analyticspb.EvaluateAnalyticsChartRequest {
	ungroupedReq := proto.Clone(originalReq).(*analyticspb.EvaluateAnalyticsChartRequest)
	ungroupedReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{}
	ungroupedReq.AnalyticsChart.DataConfig.Metrics = []*dashboardingpb.ChartMetric{
		{
			Metric: &dashboardingpb.ChartMetric_ForConversation_{
				ForConversation: &dashboardingpb.ChartMetric_ForConversation{
					StatsType: &dashboardingpb.StatsTypeForConversation{
						StatsType: &dashboardingpb.StatsTypeForConversation_CommonStatsType{
							CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
						},
					},
				},
			},
		},
	}
	return ungroupedReq
}

// removeConfigFields clones a ChartGroupBy and removes sortDirection and valueCountLimit from its config
func removeConfigFields(group *dashboardingpb.ChartGroupBy) *dashboardingpb.ChartGroupBy {
	clonedGroup := proto.Clone(group).(*dashboardingpb.ChartGroupBy)
	if clonedGroup.Config != nil {
		clonedGroup.Config.SortDirection = dashboardingpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED
		clonedGroup.Config.ValueCountLimit = 0
	}
	return clonedGroup
}

// getPrimaryGroupOnlyRequest creates a request with a single groupBy based on priority:
// 1. PRIMARY priority group if exists (sets metrics to conversation count)
// 2. If SECONDARY exists but no PRIMARY, returns empty groupBy (keeps original metrics)
// 3. Time-based group if exists (sets metrics to conversation count)
// 4. First group otherwise (sets metrics to conversation count)
// Also removes sortDirection and valueCountLimit from config
func getPrimaryGroupOnlyRequest(originalReq *analyticspb.EvaluateAnalyticsChartRequest) *analyticspb.EvaluateAnalyticsChartRequest {
	primaryGroupReq := proto.Clone(originalReq).(*analyticspb.EvaluateAnalyticsChartRequest)

	groupBy := originalReq.GetAnalyticsChart().GetDataConfig().GetGroupBy()
	foundPrimary := false

	// First, try to find a PRIMARY priority group
	primaryGroups := fn.Filter(groupBy, func(group *dashboardingpb.ChartGroupBy) bool {
		return group.GetConfig() != nil && group.GetConfig().GetGroupbyPriority() == dashboardingpb.ChartGroupBy_Config_PRIMARY
	})

	// Check if there's a SECONDARY priority group
	secondaryGroups := fn.Filter(groupBy, func(group *dashboardingpb.ChartGroupBy) bool {
		return group.GetConfig() != nil && group.GetConfig().GetGroupbyPriority() == dashboardingpb.ChartGroupBy_Config_SECONDARY
	})

	switch {
	case len(primaryGroups) > 0:
		// If primary is found, use it and set conversation count metric
		foundPrimary = true
		primaryGroupReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{removeConfigFields(primaryGroups[0])}
	case len(secondaryGroups) > 0:
		if len(groupBy) == 2 {
			// If secondary exists and there is another groupBy, keep the other one
			foundPrimary = true
			otherIndex := slices.IndexFunc(groupBy, func(group *dashboardingpb.ChartGroupBy) bool {
				return group.GetConfig().GetGroupbyPriority() != dashboardingpb.ChartGroupBy_Config_SECONDARY
			})
			primaryGroupReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{removeConfigFields(groupBy[otherIndex])}
		} else {
			// If secondary exists but no primary, return empty groupBy and keep original metrics
			primaryGroupReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{}
		}
	default:
		// No primary or secondary, look for time-based or use first group
		// This is for old created charts, that weren't updated after topAndBottom flag was added.
		foundPrimary = true

		// Look for a time-based group
		timeBasedGroups := fn.Filter(groupBy, func(group *dashboardingpb.ChartGroupBy) bool {
			return group.GetByTime() != nil
		})

		switch {
		case len(timeBasedGroups) > 0:
			primaryGroupReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{removeConfigFields(timeBasedGroups[0])}
		case len(groupBy) > 0:
			primaryGroupReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{removeConfigFields(groupBy[0])}
		default:
			primaryGroupReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{}
		}
	}

	// Only set metrics to conversation count if foundPrimary is true
	// Otherwise, keep the original metrics from the cloned request
	if foundPrimary {
		primaryGroupReq.AnalyticsChart.DataConfig.Metrics = []*dashboardingpb.ChartMetric{
			{
				Metric: &dashboardingpb.ChartMetric_ForConversation_{
					ForConversation: &dashboardingpb.ChartMetric_ForConversation{
						StatsType: &dashboardingpb.StatsTypeForConversation{
							StatsType: &dashboardingpb.StatsTypeForConversation_CommonStatsType{
								CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
							},
						},
					},
				},
			},
		}
	}

	return primaryGroupReq
}

// comboMetricBaselineFutures holds the three in-flight percentage-baseline evaluations for a
// single combo-chart metric. Each future resolves to a single-metric response (one result).
type comboMetricBaselineFutures struct {
	ungrouped        future.Future[*analyticspb.EvaluateAnalyticsChartResponse]
	groupsAsFilters  future.Future[*analyticspb.EvaluateAnalyticsChartResponse]
	primaryGroupOnly future.Future[*analyticspb.EvaluateAnalyticsChartResponse]
}

// retrieveComboChartEvaluationData is the combo (2-metric) counterpart of RetrieveChartEvaluationData.
// The main evaluation goes through the combo path in EvaluateAnalyticsChart and returns two aligned
// results (primary, secondary). Percentage baselines are computed per metric: each metric is
// evaluated as its own single-metric chart so the existing ungrouped / groups-as-filters /
// primary-group-only helpers each yield exactly one result. Every request runs concurrently under a
// single errgroup, mirroring the single-metric fan-out.
func (a AnalyticsServiceImpl) retrieveComboChartEvaluationData(ctx context.Context, req *analyticspb.RetrieveChartEvaluationDataRequest) (*analyticspb.RetrieveChartEvaluationDataResponse, error) {
	originalReq := req.GetEvaluateAnalyticsChartRequest()
	metrics := originalReq.GetAnalyticsChart().GetDataConfig().GetMetrics()

	eg := errgroup.New()

	// We clone the request because EvaluateAnalyticsChart mutates it in place.
	mainClone := proto.Clone(originalReq).(*analyticspb.EvaluateAnalyticsChartRequest)
	mainFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
		return a.EvaluateAnalyticsChart(ctx, mainClone)
	})

	baselineFutures := make([]comboMetricBaselineFutures, len(metrics))
	if req.GetFetchPercentageRelatedData() {
		for i := range metrics {
			singleMetricReq := requestWithSingleMetric(originalReq, i)
			ungroupedReq := getUngroupedRequest(singleMetricReq)
			groupsAsFiltersReq := getGroupsAsFiltersRequest(singleMetricReq)
			primaryGroupOnlyReq := getPrimaryGroupOnlyRequest(singleMetricReq)
			baselineFutures[i] = comboMetricBaselineFutures{
				ungrouped: future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
					return a.EvaluateAnalyticsChart(ctx, ungroupedReq)
				}),
				groupsAsFilters: future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
					return a.EvaluateAnalyticsChart(ctx, groupsAsFiltersReq)
				}),
				primaryGroupOnly: future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
					return a.EvaluateAnalyticsChart(ctx, primaryGroupOnlyReq)
				}),
			}
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	mainResp, err := mainFuture.Result()
	if err != nil {
		a.logger.Errorf(
			ctx,
			"(RetrieveChartEvaluationData) combo main request failed (chart=%q, parent=%q): %s",
			originalReq.GetAnalyticsChart().GetName(),
			originalReq.GetParent(),
			err.Error(),
		)
		return nil, err
	}
	alignedResults := mainResp.GetResultForChart().GetAnalyticsResults()

	metricsEvaluationData := make([]*dashboardingpb.AnalyticsChartEvaluationData_MetricEvaluationData, 0, len(alignedResults))
	for i, result := range alignedResults {
		metricData := &dashboardingpb.AnalyticsChartEvaluationData_MetricEvaluationData{
			AnalyticsResult: result,
		}
		if req.GetFetchPercentageRelatedData() && i < len(baselineFutures) {
			ungroupedTotal, groupsAsFiltersTotal, primaryGroupOnly, err := resolveComboMetricBaselines(baselineFutures[i])
			if err != nil {
				return nil, err
			}
			metricData.UngroupedTotal = ungroupedTotal
			metricData.GroupsAsFiltersTotal = groupsAsFiltersTotal
			metricData.GroupedByPrimaryResults = primaryGroupOnly
		}
		metricsEvaluationData = append(metricsEvaluationData, metricData)
	}

	return &analyticspb.RetrieveChartEvaluationDataResponse{
		EvaluationData: &dashboardingpb.AnalyticsChartEvaluationData{
			MetricsEvaluationData: metricsEvaluationData,
		},
	}, nil
}

// resolveComboMetricBaselines waits on one metric's three baseline evaluations and reduces them to
// the totals stored on a MetricEvaluationData. Each baseline is a single-metric evaluation, so the
// ungrouped/groups-as-filters totals are the sum of the single result's numbers and the
// primary-group-only value is that single result (nil when absent).
func resolveComboMetricBaselines(futures comboMetricBaselineFutures) (int32, int32, *dashboardingpb.AnalyticsResult, error) {
	ungroupedResp, err := futures.ungrouped.Result()
	if err != nil {
		return 0, 0, nil, err
	}
	groupsAsFiltersResp, err := futures.groupsAsFilters.Result()
	if err != nil {
		return 0, 0, nil, err
	}
	primaryGroupOnlyResp, err := futures.primaryGroupOnly.Result()
	if err != nil {
		return 0, 0, nil, err
	}

	ungroupedTotal := sumFirstAnalyticsResultTotal(ungroupedResp)
	groupsAsFiltersTotal := sumFirstAnalyticsResultTotal(groupsAsFiltersResp)

	var primaryGroupOnly *dashboardingpb.AnalyticsResult
	if primaryGroupOnlyResults := primaryGroupOnlyResp.GetResultForChart().GetAnalyticsResults(); len(primaryGroupOnlyResults) > 0 {
		primaryGroupOnly = primaryGroupOnlyResults[0]
	}
	return ungroupedTotal, groupsAsFiltersTotal, primaryGroupOnly, nil
}

// sumFirstAnalyticsResultTotal sums the analytics numbers of a single-metric response's only result.
func sumFirstAnalyticsResultTotal(resp *analyticspb.EvaluateAnalyticsChartResponse) int32 {
	results := resp.GetResultForChart().GetAnalyticsResults()
	if len(results) == 0 {
		return 0
	}
	return sumAnalyticsNumbersForAnalyticsResult(results[0])
}

// sumAnalyticsNumbersForAnalyticsResult extracts and sums all numeric values from an AnalyticsResult
func sumAnalyticsNumbersForAnalyticsResult(result *dashboardingpb.AnalyticsResult) int32 {
	var total int32
	if result.AnalyticsNumbers != nil {
		for _, analyticsNumber := range result.AnalyticsNumbers {
			// Sum the Value field from each AnalyticsNumber
			total += int32(analyticsNumber.Value)
		}
	}
	return total
}

// getGroupsAsFiltersRequest creates a groups-as-filters version of the EvaluateAnalyticsChartRequest
// This moves groupBy clauses to filters and clears groupBy
func getGroupsAsFiltersRequest(originalReq *analyticspb.EvaluateAnalyticsChartRequest) *analyticspb.EvaluateAnalyticsChartRequest {
	groupsAsFiltersReq := proto.Clone(originalReq).(*analyticspb.EvaluateAnalyticsChartRequest)
	groupsAsFiltersReq.AnalyticsChart.DataConfig.GroupBy = []*dashboardingpb.ChartGroupBy{}

	// Convert groupBy clauses to filters (excluding byTime and byUniqueValue)
	if originalReq.AnalyticsChart.DataConfig.GroupBy != nil {
		for _, group := range originalReq.AnalyticsChart.DataConfig.GroupBy {
			// Skip byTime and byUniqueValue groups
			if group.GetByTime() != nil || group.GetByUniqueValue() != nil {
				continue
			}
			byProvidedValue := group.GetByProvidedValue()
			// Convert byProvidedValue groups to filters
			if byProvidedValue.GetValueSet() != nil {
				filter := &dashboardingpb.ChartDataFilter{
					TargetAttribute: byProvidedValue.GetAttribute(),
					Operator:        dashboardingpb.ChartDataFilter_IN,
					ValueSet:        byProvidedValue.GetValueSet(),
				}
				groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters = append(
					groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters,
					filter,
				)
			}
		}
	}

	// Handle subcategories and emotions having overlap conversations
	// Convert forSubcategory and forEmotion metrics to filters
	if originalReq.AnalyticsChart.DataConfig.Metrics != nil {
		for _, metric := range originalReq.AnalyticsChart.DataConfig.Metrics {
			// Skip forConversation metrics
			if metric.GetForConversation() != nil {
				continue
			}
			if forSubcategory := metric.GetForSubcategory(); forSubcategory != nil {
				// Create attribute values for subcategories
				attributeValues := make([]*dashboardingpb.ChartAttributeValue, 0, len(forSubcategory.GetSubcategories()))
				for _, subcategory := range forSubcategory.GetSubcategories() {
					attributeValues = append(attributeValues, &dashboardingpb.ChartAttributeValue{
						Value: &dashboardingpb.ChartAttributeValue_SubcategoryValue{
							SubcategoryValue: subcategory,
						},
					})
				}

				if len(attributeValues) > 0 {
					filter := &dashboardingpb.ChartDataFilter{
						TargetAttribute: dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED,
						Operator:        dashboardingpb.ChartDataFilter_IN,
						ValueSet: &dashboardingpb.ChartValueSet{
							Values: &dashboardingpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardingpb.ChartValueSet_Single{
									AttributeValues: attributeValues,
								},
							},
						},
					}
					groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters = append(
						groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters,
						filter,
					)
				}
			}

			if forEmotion := metric.GetForEmotion(); forEmotion != nil {
				// Create attribute values for emotions
				attributeValues := make([]*dashboardingpb.ChartAttributeValue, 0, len(forEmotion.GetEmotions()))
				for _, emotion := range forEmotion.GetEmotions() {
					attributeValues = append(attributeValues, &dashboardingpb.ChartAttributeValue{
						Value: &dashboardingpb.ChartAttributeValue_EmotionValue{
							EmotionValue: emotion,
						},
					})
				}

				if len(attributeValues) > 0 {
					filter := &dashboardingpb.ChartDataFilter{
						TargetAttribute: dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED,
						Operator:        dashboardingpb.ChartDataFilter_IN,
						ValueSet: &dashboardingpb.ChartValueSet{
							Values: &dashboardingpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardingpb.ChartValueSet_Single{
									AttributeValues: attributeValues,
								},
							},
						},
					}
					groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters = append(
						groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters,
						filter,
					)
				}
			}

			if forCluster := metric.GetForCluster(); forCluster != nil {
				// Create attribute values for clusters
				attributeValues := make([]*dashboardingpb.ChartAttributeValue, 0, len(forCluster.GetClusters()))
				for _, cluster := range forCluster.GetClusters() {
					attributeValues = append(attributeValues, &dashboardingpb.ChartAttributeValue{
						Value: &dashboardingpb.ChartAttributeValue_ClusterValue{
							ClusterValue: cluster,
						},
					})
				}

				if len(attributeValues) > 0 {
					filter := &dashboardingpb.ChartDataFilter{
						TargetAttribute: dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED,
						Operator:        dashboardingpb.ChartDataFilter_IN,
						ValueSet: &dashboardingpb.ChartValueSet{
							Values: &dashboardingpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardingpb.ChartValueSet_Single{
									AttributeValues: attributeValues,
								},
							},
						},
					}
					groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters = append(
						groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters,
						filter,
					)
				}
			}

			if forAggregateSubcategory := metric.GetForAggregateSubcategory(); forAggregateSubcategory != nil {
				// Create attribute values for aggregate subcategories
				attributeValues := make([]*dashboardingpb.ChartAttributeValue, 0, len(forAggregateSubcategory.GetAggregateSubcategories()))
				for _, aggregateSubcategory := range forAggregateSubcategory.GetAggregateSubcategories() {
					attributeValues = append(attributeValues, &dashboardingpb.ChartAttributeValue{
						Value: &dashboardingpb.ChartAttributeValue_AggregateSubcategoryValue{
							AggregateSubcategoryValue: aggregateSubcategory,
						},
					})
				}

				if len(attributeValues) > 0 {
					filter := &dashboardingpb.ChartDataFilter{
						TargetAttribute: dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED,
						Operator:        dashboardingpb.ChartDataFilter_IN,
						ValueSet: &dashboardingpb.ChartValueSet{
							Values: &dashboardingpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardingpb.ChartValueSet_Single{
									AttributeValues: attributeValues,
								},
							},
						},
					}
					groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters = append(
						groupsAsFiltersReq.AnalyticsChart.DataConfig.Filters,
						filter,
					)
				}
			}
		}
	}

	// Transform metrics: replace forSubcategory, forEmotion, forCluster, and forAggregateSubcategory with conversation count
	transformedMetrics := make([]*dashboardingpb.ChartMetric, 0, len(originalReq.AnalyticsChart.DataConfig.Metrics))
	for _, metric := range originalReq.AnalyticsChart.DataConfig.Metrics {
		if metric.GetForSubcategory() != nil || metric.GetForEmotion() != nil || metric.GetForCluster() != nil || metric.GetForAggregateSubcategory() != nil {
			// Replace with conversation count metric
			transformedMetrics = append(transformedMetrics, &dashboardingpb.ChartMetric{
				Metric: &dashboardingpb.ChartMetric_ForConversation_{
					ForConversation: &dashboardingpb.ChartMetric_ForConversation{
						StatsType: &dashboardingpb.StatsTypeForConversation{
							StatsType: &dashboardingpb.StatsTypeForConversation_CommonStatsType{
								CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
							},
						},
					},
				},
			})
		} else {
			// Keep original metric
			transformedMetrics = append(transformedMetrics, metric)
		}
	}
	groupsAsFiltersReq.AnalyticsChart.DataConfig.Metrics = transformedMetrics

	return groupsAsFiltersReq
}
