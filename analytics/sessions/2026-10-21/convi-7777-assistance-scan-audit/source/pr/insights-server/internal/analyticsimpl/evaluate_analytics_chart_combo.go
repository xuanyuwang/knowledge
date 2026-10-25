package analyticsimpl

import (
	"context"
	"fmt"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/shared-go/util/errgroup"
	"github.com/cresta/shared-go/util/sync/future"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// comboChartMetricCount is the exact number of metrics a combo chart carries: a primary
// metric (rendered as bars on the left Y-axis) and a secondary metric (rendered as a line
// on the right Y-axis).
const comboChartMetricCount = 2

// primaryMetricIdx is the index of the metric that drives bucket selection: ordering, top-N and the
// canonical outer group-by the secondary is projected onto.
const primaryMetricIdx = 0

// secondaryMetricIdx is the index of the metric projected onto the primary's buckets.
const secondaryMetricIdx = 1

// maxGroupByValueCountLimit is the largest value_count_limit a group-by accepts, per its proto
// documentation ("This value can not exceed 1000"). Used to ask for the widest bucket set available
// when a metric must not do its own selection.
const maxGroupByValueCountLimit = 1000

// isComboChartRequest reports whether the request should be handled by the combo-chart path.
// Combo mode is always-on for any exactly-two-metric request (no feature flag). Single-metric
// and 3+-metric requests continue through the ordinary single-metric pipeline.
func isComboChartRequest(req *analyticspb.EvaluateAnalyticsChartRequest) bool {
	return len(req.GetAnalyticsChart().GetDataConfig().GetMetrics()) == comboChartMetricCount
}

// evaluateComboChart handles a two-metric chart. The single-metric EvaluateAnalyticsChart
// pipeline is built end-to-end around Metrics[0] (implicit group-by/filter extraction, ES
// aggregation building, response building) and correctOrderByForTermsAgg rejects >1 metric
// outright. Rather than thread two metrics through one ES query, this splits the request into
// two single-metric requests, evaluates each through the existing pipeline, and merges the
// outputs onto the primary metric's outer group-by buckets so both series share an axis.
//
// Ordering / top-N are driven by whichever metric ranking_metric_index names, defaulting to the
// primary. The secondary is projected onto the ranking metric's outer buckets (buckets it does not
// have are dropped, buckets missing from the secondary filled with 0). Infra metrics are not
// supported.
func (a AnalyticsServiceImpl) evaluateComboChart(ctx context.Context, req *analyticspb.EvaluateAnalyticsChartRequest) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	metrics := req.GetAnalyticsChart().GetDataConfig().GetMetrics()
	if len(metrics) != comboChartMetricCount {
		return nil, status.Errorf(codes.InvalidArgument, "combo chart requires exactly %d metrics, got %d", comboChartMetricCount, len(metrics))
	}
	for i, metric := range metrics {
		if metric.GetForInfraMetric() != nil {
			return nil, status.Errorf(codes.Unimplemented, "combo charts do not support infra metrics (metric %d is a ForInfraMetric)", i)
		}
	}

	rankingMetricIdx, err := resolveRankingMetricIdx(req.GetAnalyticsChart().GetDataConfig())
	if err != nil {
		return nil, err
	}

	// Clone per metric: EvaluateAnalyticsChart mutates its request's DataConfig in place, so
	// each single-metric clone must be independent. Filters/group_by are shared by construction.
	primaryReq := requestWithSingleMetric(req, primaryMetricIdx)
	secondaryReq := requestWithSingleMetric(req, secondaryMetricIdx)

	// Only the ranking metric selects buckets. The other has to run unlimited so it can be projected
	// onto that selection afterwards; see clearBucketSelection.
	if rankingMetricIdx == primaryMetricIdx {
		clearBucketSelection(secondaryReq.GetAnalyticsChart().GetDataConfig())
	} else {
		clearBucketSelection(primaryReq.GetAnalyticsChart().GetDataConfig())
	}

	eg := errgroup.New()
	primaryFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
		return a.EvaluateAnalyticsChart(ctx, primaryReq)
	})
	secondaryFuture := future.GoInErrGroup(eg, func() (*analyticspb.EvaluateAnalyticsChartResponse, error) {
		return a.EvaluateAnalyticsChart(ctx, secondaryReq)
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	primaryResp, err := primaryFuture.Result()
	if err != nil {
		return nil, fmt.Errorf("evaluating primary combo-chart metric: %w", err)
	}
	secondaryResp, err := secondaryFuture.Result()
	if err != nil {
		return nil, fmt.Errorf("evaluating secondary combo-chart metric: %w", err)
	}

	// An empty ranking result selected no buckets, e.g. no Top-N team overlaps the team filter.
	// Alignment has no outer layer to project onto and would keep every bucket of the other metric.
	rankingResp := primaryResp
	if rankingMetricIdx == secondaryMetricIdx {
		rankingResp = secondaryResp
	}
	if len(rankingResp.GetResultForChart().GetAnalyticsResults()) == 0 {
		return &analyticspb.EvaluateAnalyticsChartResponse{}, nil
	}

	primaryResult := firstAnalyticsResult(primaryResp.GetResultForChart())
	secondaryResult := firstAnalyticsResult(secondaryResp.GetResultForChart())

	if err := validateComboResultShapes(primaryResult, secondaryResult); err != nil {
		return nil, err
	}

	// The ranking metric selected the buckets, so its result supplies the canonical outer list and the
	// other metric is projected onto it. The response order stays [primary, secondary] either way.
	alignedSecondary := secondaryResult
	if rankingMetricIdx == secondaryMetricIdx {
		primaryResult = alignToCanonicalBuckets(secondaryResult, primaryResult)
	} else {
		alignedSecondary = alignToCanonicalBuckets(primaryResult, secondaryResult)
	}

	// Only now is the outer bucket set final, so only now can an inner limit be ranked against the
	// data the chart will actually show. Ranking before alignment would total inner values over
	// buckets that are about to be dropped and could keep the wrong ones. Both metrics are limited
	// with their own config: either may be the one carrying a dimension.
	primaryResult = limitInnerGroupByValues(primaryResult, metrics[primaryMetricIdx])
	alignedSecondary = limitInnerGroupByValues(alignedSecondary, metrics[secondaryMetricIdx])

	return &analyticspb.EvaluateAnalyticsChartResponse{
		ResultForChart: &dashboardpb.AnalyticsResultForChart{
			AnalyticsResults: []*dashboardpb.AnalyticsResult{primaryResult, alignedSecondary},
		},
		UpdateTimestamp: primaryResp.GetUpdateTimestamp(),
	}, nil
}

// requestWithSingleMetric deep-clones the request and reduces its DataConfig to the single
// metric at metricIdx, so the clone flows through the ordinary single-metric pipeline without
// affecting the original request or the other metric.
//
// Bucket selection is left alone here: which metric is allowed to rank is a decision for the caller,
// and retrieveComboChartEvaluationData uses this helper to build percentage baselines where the
// question does not arise.
func requestWithSingleMetric(req *analyticspb.EvaluateAnalyticsChartRequest, metricIdx int) *analyticspb.EvaluateAnalyticsChartRequest {
	cloned := proto.Clone(req).(*analyticspb.EvaluateAnalyticsChartRequest)
	metrics := cloned.AnalyticsChart.DataConfig.Metrics
	cloned.AnalyticsChart.DataConfig.Metrics = []*dashboardpb.ChartMetric{metrics[metricIdx]}
	return cloned
}

// resolveRankingMetricIdx returns the index of the metric that decides which buckets are shown.
//
// ranking_metric_index rides on the same Config as the sort and limit it qualifies, so it is read
// from whichever config actually carries a limit: a shared group_by when the user is ranking a
// breakdown, or a metric's own groupby_config when they are ranking that metric's dimension.
func resolveRankingMetricIdx(dataConfig *dashboardpb.ChartDataConfig) (int, error) {
	if config := rankedGroupByConfig(dataConfig); config != nil {
		idx := int(config.GetRankingMetricIndex())
		if idx < 0 || idx >= comboChartMetricCount {
			return 0, status.Errorf(codes.InvalidArgument, "ranking_metric_index %d is out of range for a combo chart's %d metrics", idx, comboChartMetricCount)
		}
		return idx, nil
	}

	// A limit on a metric's own dimension can only be ranked by that metric: the other one is not
	// grouped by it and so has no value to rank the buckets with.
	for metricIdx, metric := range dataConfig.GetMetrics() {
		config := metric.GetGroupbyConfig()
		if config.GetValueCountLimit() <= 0 {
			continue
		}
		// ranking_metric_index has no field presence, so an unset one reads as 0. Treating that as
		// "the primary ranks" would reject every top-N on the secondary's own dimension written by a
		// client that predates the field. Zero means unspecified here, i.e. the owning metric.
		if idx := int(config.GetRankingMetricIndex()); idx != 0 && idx != metricIdx {
			return 0, status.Errorf(codes.InvalidArgument,
				"ranking_metric_index %d cannot rank metric %d's own dimension: only that metric is grouped by it", idx, metricIdx)
		}
		return metricIdx, nil
	}

	return primaryMetricIdx, nil
}

// rankedGroupByConfig returns the config of the group-by whose limit ES will actually apply, or nil
// if no group-by is limited.
//
// EvaluateAnalyticsChart swaps the GROUPBY_LAYER_OUTER entry to index 0 and sorts there, so request
// order is not evaluation order. Prefer the outer entry; fall back to the first limited one for
// requests that leave the layer unpinned, where the backend picks the outer group-by itself.
func rankedGroupByConfig(dataConfig *dashboardpb.ChartDataConfig) *dashboardpb.ChartGroupBy_Config {
	var firstLimited *dashboardpb.ChartGroupBy_Config
	for _, groupBy := range dataConfig.GetGroupBy() {
		config := groupBy.GetConfig()
		if config.GetValueCountLimit() <= 0 {
			continue
		}
		if config.GetGroupbyLayer() == dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER {
			return config
		}
		if firstLimited == nil {
			firstLimited = config
		}
	}
	return firstLimited
}

// clearBucketSelection strips ordering and top-N limits from a non-ranking metric's request.
//
// group_by is shared by both metrics, so a valueCountLimit left on it is applied to each evaluation
// independently: each returns its own top N ranked by its own metric. Those sets generally differ,
// and for every canonical bucket the other metric did not happen to rank, alignToCanonicalBuckets
// has no value to project and fills 0 — indistinguishable from a real zero in the rendered chart.
//
// Exactly one metric decides which buckets are shown (see resolveRankingMetricIdx). The other has to
// report values for as many of them as it can and be projected down afterwards.
//
// "As many as it can" rather than "all": there is no way to ask for an unlimited group-by. A zero
// value_count_limit means "use the default", which convertGroupByUniqueValue reads as 1000 and the
// composite-agg path reads from a tunable env flag that could be lower. The maximum is requested
// explicitly so the non-ranking metric's coverage never depends on that tuning.
//
// A breakdown with more than maxGroupByValueCountLimit distinct values can therefore still return a
// set that misses some of the ranking metric's buckets, and those render as 0. Closing that would
// mean evaluating the ranking metric first and filtering the other to its buckets, which costs the
// parallelism the two queries currently get.
func clearBucketSelection(dataConfig *dashboardpb.ChartDataConfig) {
	for _, groupBy := range dataConfig.GetGroupBy() {
		if config := groupBy.GetConfig(); config != nil {
			config.SortDirection = dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED
			config.ValueCountLimit = maxGroupByValueCountLimit
		}
	}
	// Defensive: a secondary metric is not expected to carry its own group-by config, but one would
	// limit the metric's own dimension the same way.
	for _, metric := range dataConfig.GetMetrics() {
		if config := metric.GetGroupbyConfig(); config != nil {
			config.SortDirection = dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED
			config.ValueCountLimit = maxGroupByValueCountLimit
		}
	}
}

// limitInnerGroupByValues applies a top/bottom-N limit to the inner group-by layer, which ES cannot
// do itself.
//
// A combo chart pins the bucket both metrics share as the outer group-by so they agree on an x-axis.
// That leaves the primary metric's own dimension (five emotions, say) as the inner layer, and ES only
// sorts and truncates the outer one — so a limit on the metric's dimension reaches us unapplied and
// every value is still present. Applying it here is what makes "top 2 emotions by week" expressible.
//
// The N are chosen once across the whole range, not per outer bucket: each inner value is reduced to
// a single score over every bucket, and the survivors appear in all of them. Per-bucket selection
// would give each bucket a different set, so a series would exist only in the buckets where it
// placed and the chart would be full of holes.
//
// It must run after bucket alignment. Scoring against the unaligned result would rank inner values
// over buckets that alignment is about to drop, and could keep values that lose badly in the buckets
// the chart actually shows.
//
// Survivors keep their original relative order rather than being reordered by rank: order decides
// series and legend position, and shuffling it as N changes buys nothing when colours are keyed off
// the attribute rather than its index.
//
// A result with fewer than two layers is returned untouched — there is no inner layer, which means
// the dimension was the outer group-by and ES has already applied the limit.
func limitInnerGroupByValues(result *dashboardpb.AnalyticsResult, metric *dashboardpb.ChartMetric) *dashboardpb.AnalyticsResult {
	config := metric.GetGroupbyConfig()
	limit := int(config.GetValueCountLimit())
	layers := result.GetGroupByValuesForAllKeys()
	if limit <= 0 || len(layers) < 2 {
		return result
	}
	innerValues := layers[1].GetGroupByValues()
	if limit >= len(innerValues) {
		return result
	}

	scores, present := innerValueScores(result, len(innerValues), metricAggregationOperator(metric))

	ranked := make([]int, len(innerValues))
	for i := range ranked {
		ranked[i] = i
	}
	ascending := config.GetSortDirection() == dashboardpb.ChartGroupBy_Config_ASC
	// Stable so that equal scores keep their original order and the selection is deterministic.
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		// An inner value with no rows left after alignment scores 0, which under ASC would beat every
		// real value and put an entirely empty series on the chart in place of the genuine lowest one.
		// Absent values sort last in both directions and are only picked if there are fewer than N
		// real ones.
		if present[a] != present[b] {
			return present[a]
		}
		if ascending {
			return scores[a] < scores[b]
		}
		return scores[a] > scores[b]
	})

	keptOldIndexes := ranked[:limit]
	kept := make(map[int32]bool, limit)
	for _, idx := range keptOldIndexes {
		kept[int32(idx)] = true
	}

	remapped := make(map[int32]int32, limit)
	survivors := make([]*dashboardpb.AnalyticsResult_GroupByValue, 0, limit)
	for oldIdx, value := range innerValues {
		if !kept[int32(oldIdx)] {
			continue
		}
		remapped[int32(oldIdx)] = int32(len(survivors))
		survivors = append(survivors, value)
	}

	numbers := make([]*dashboardpb.AnalyticsResult_AnalyticsNumber, 0, len(result.GetAnalyticsNumbers()))
	for _, number := range result.GetAnalyticsNumbers() {
		indexes := number.GetGroupByValueIndexes()
		// Rows that do not reference the inner layer are not scoped by this limit.
		if len(indexes) < 2 {
			numbers = append(numbers, number)
			continue
		}
		newInnerIdx, ok := remapped[indexes[1]]
		if !ok {
			continue
		}
		cloned := proto.Clone(number).(*dashboardpb.AnalyticsResult_AnalyticsNumber)
		cloned.GroupByValueIndexes[1] = newInnerIdx
		numbers = append(numbers, cloned)
	}

	limitedLayers := make([]*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey, len(layers))
	copy(limitedLayers, layers)
	limitedLayers[1] = &dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{GroupByValues: survivors}

	return &dashboardpb.AnalyticsResult{
		GroupByValuesForAllKeys: limitedLayers,
		AnalyticsNumbers:        numbers,
	}
}

// innerValueScores reduces each inner value's per-bucket values to the single number the top-N
// ranking compares.
//
// Values are combined across buckets the same way the metric combined them inside one. Summing an
// average is meaningless and actively misleading: an emotion averaging 40 across three weeks would
// total 120 and outrank one averaging 100 in a single week. Minimum and maximum are likewise the
// minimum and maximum of the per-bucket values, not their sum. Anything else is a count and sums.
//
// The average is unweighted, because per-bucket denominators are not in the result: a bucket built
// from two conversations counts as much as one built from two hundred.
//
// The second return reports which inner values actually had rows. A score of 0 is ambiguous on its
// own — it could be a real zero or no data at all — and the caller has to keep the two apart when
// ranking ascending.
func innerValueScores(result *dashboardpb.AnalyticsResult, innerCount int, operator dashboardpb.AggregationOperator) ([]float64, []bool) {
	scores := make([]float64, innerCount)
	bucketCounts := make([]int, innerCount)

	for _, number := range result.GetAnalyticsNumbers() {
		indexes := number.GetGroupByValueIndexes()
		if len(indexes) < 2 {
			continue
		}
		innerIdx := int(indexes[1])
		// Both bounds: a negative index would panic on the write below.
		if innerIdx < 0 || innerIdx >= innerCount {
			continue
		}
		value := float64(number.GetValue())
		switch operator {
		case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MIN:
			if bucketCounts[innerIdx] == 0 || value < scores[innerIdx] {
				scores[innerIdx] = value
			}
		case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MAX:
			if bucketCounts[innerIdx] == 0 || value > scores[innerIdx] {
				scores[innerIdx] = value
			}
		default:
			// AVG accumulates here and is divided through below; everything else is a plain total.
			scores[innerIdx] += value
		}
		bucketCounts[innerIdx]++
	}

	present := make([]bool, innerCount)
	for i, count := range bucketCounts {
		present[i] = count > 0
		if operator == dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG && count > 0 {
			scores[i] /= float64(count)
		}
	}
	return scores, present
}

// firstAnalyticsResult returns the single AnalyticsResult a single-metric evaluation produces,
// or an empty result if none is present.
func firstAnalyticsResult(result *dashboardpb.AnalyticsResultForChart) *dashboardpb.AnalyticsResult {
	if len(result.GetAnalyticsResults()) == 0 {
		return &dashboardpb.AnalyticsResult{}
	}
	return result.GetAnalyticsResults()[0]
}

// validateComboResultShapes enforces the MVP-supported result shapes:
//   - Case A: both metrics reduce to the same outer bucket shape (e.g. [week] / [week]).
//   - Case B: one metric adds a single extra inner grouping layer sharing the same outer bucket
//     (e.g. [week(outer), emotion(inner)] / [week(outer)]).
//
// Group-by layers are ordered outer→inner (index 0 = outer). The outer layers must be the same
// kind of attribute, and layer counts may differ by at most one (the extra layer being inner).
func validateComboResultShapes(primary, secondary *dashboardpb.AnalyticsResult) error {
	primaryLayers := primary.GetGroupByValuesForAllKeys()
	secondaryLayers := secondary.GetGroupByValuesForAllKeys()

	if diff := len(primaryLayers) - len(secondaryLayers); diff < -1 || diff > 1 {
		return status.Errorf(codes.InvalidArgument,
			"combo chart metrics must have compatible group-by structures: primary has %d group-by layers, secondary has %d (they may differ by at most one inner layer)",
			len(primaryLayers), len(secondaryLayers))
	}

	// When both metrics group by an outer bucket, that outer bucket must be the same kind.
	if len(primaryLayers) > 0 && len(secondaryLayers) > 0 {
		primaryKind := outerGroupByKind(primaryLayers[0])
		secondaryKind := outerGroupByKind(secondaryLayers[0])
		if primaryKind != secondaryKind {
			return status.Errorf(codes.InvalidArgument,
				"combo chart metrics must share the same outer group-by bucket, but got %q (primary) and %q (secondary)",
				primaryKind, secondaryKind)
		}
	}
	return nil
}

// outerGroupByKind returns a stable discriminant for the attribute kind of an outer group-by
// layer (the populated oneof field of its ChartAttributeValue, e.g. "time_value"), used to
// confirm both metrics share the same outer bucket dimension. It reads the set oneof via proto
// reflection so it does not depend on the generated Go field name.
func outerGroupByKind(layer *dashboardpb.AnalyticsResult_GroupByValuesForSingleKey) string {
	for _, groupByValue := range layer.GetGroupByValues() {
		attributeValue := groupByValue.GetValue()
		if attributeValue == nil {
			continue
		}
		message := attributeValue.ProtoReflect()
		oneofs := message.Descriptor().Oneofs()
		for i := 0; i < oneofs.Len(); i++ {
			if field := message.WhichOneof(oneofs.Get(i)); field != nil {
				return string(field.Name())
			}
		}
	}
	return ""
}

// alignToCanonicalBuckets re-projects `other` onto the outer group-by buckets of `canonical` so both
// series share the same outer axis in the same order:
//   - outer buckets only `other` has are dropped (the canonical result owns bucket selection),
//   - canonical buckets missing from `other` are filled with a 0 value (only when `other` is scalar,
//     i.e. has just the outer layer),
//   - `other`'s own inner layer (if any) is preserved untouched.
//
// `canonical` is the ranking metric's result, which is usually but not always the primary — see
// resolveRankingMetricIdx.
//
// Buckets are matched by the outer ChartAttributeValue payload, not by raw index, because each
// metric applies its own sort/top-N and compacts index 0 independently.
func alignToCanonicalBuckets(canonical, other *dashboardpb.AnalyticsResult) *dashboardpb.AnalyticsResult {
	canonicalLayers := canonical.GetGroupByValuesForAllKeys()
	otherLayers := other.GetGroupByValuesForAllKeys()

	// No outer bucket to anchor on (both scalar): return the other result unchanged.
	if len(canonicalLayers) == 0 || len(otherLayers) == 0 {
		return other
	}

	canonicalOuter := canonicalLayers[0]
	canonicalIndexByKey := make(map[string]int32, len(canonicalOuter.GetGroupByValues()))
	for idx, groupByValue := range canonicalOuter.GetGroupByValues() {
		canonicalIndexByKey[groupByValueKey(groupByValue)] = int32(idx)
	}

	otherOuter := otherLayers[0]
	coveredOuter := make(map[int32]bool)
	remapped := make([]*dashboardpb.AnalyticsResult_AnalyticsNumber, 0, len(other.GetAnalyticsNumbers()))
	for _, number := range other.GetAnalyticsNumbers() {
		indexes := number.GetGroupByValueIndexes()
		if len(indexes) == 0 || int(indexes[0]) >= len(otherOuter.GetGroupByValues()) {
			continue
		}
		canonicalIdx, ok := canonicalIndexByKey[groupByValueKey(otherOuter.GetGroupByValues()[indexes[0]])]
		if !ok {
			// An outer bucket the canonical result did not select.
			continue
		}
		cloned := proto.Clone(number).(*dashboardpb.AnalyticsResult_AnalyticsNumber)
		cloned.GroupByValueIndexes = append([]int32{canonicalIdx}, indexes[1:]...)
		remapped = append(remapped, cloned)
		if len(indexes) == 1 {
			coveredOuter[canonicalIdx] = true
		}
	}

	// Fill canonical outer buckets `other` lacks. Only well-defined when `other` is scalar (single
	// outer layer); a grouped one keeps only its actual inner combinations.
	if len(otherLayers) == 1 {
		for idx := range canonicalOuter.GetGroupByValues() {
			if coveredOuter[int32(idx)] {
				continue
			}
			remapped = append(remapped, &dashboardpb.AnalyticsResult_AnalyticsNumber{
				Value:               0,
				GroupByValueIndexes: []int32{int32(idx)},
			})
		}
	}

	sort.SliceStable(remapped, func(i, j int) bool {
		return lessIndexVector(remapped[i].GetGroupByValueIndexes(), remapped[j].GetGroupByValueIndexes())
	})

	// Outer layer becomes the canonical outer list; any inner layer stays `other`'s.
	alignedLayers := make([]*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey, 0, len(otherLayers))
	alignedLayers = append(alignedLayers, proto.Clone(canonicalOuter).(*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey))
	alignedLayers = append(alignedLayers, otherLayers[1:]...)

	return &dashboardpb.AnalyticsResult{
		GroupByValuesForAllKeys: alignedLayers,
		AnalyticsNumbers:        remapped,
	}
}

// groupByValueKey returns a stable identity string for a group-by bucket value, used to match
// the same outer bucket across the two independent metric evaluations.
func groupByValueKey(groupByValue *dashboardpb.AnalyticsResult_GroupByValue) string {
	marshaled, err := proto.MarshalOptions{Deterministic: true}.Marshal(groupByValue.GetValue())
	if err != nil {
		return groupByValue.String()
	}
	return string(marshaled)
}

// lessIndexVector orders group-by index vectors lexicographically (outer index first).
func lessIndexVector(a, b []int32) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
