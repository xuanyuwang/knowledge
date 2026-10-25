package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// evaluateInfraMetricsChart handles EvaluateAnalyticsChart for ForInfraMetric charts.
// It looks up the MetricDefinition PromQL template from the profile config, substitutes
// customer/profile/time template variables, and executes the query against Prometheus.
func (a AnalyticsServiceImpl) evaluateInfraMetricsChart(ctx context.Context, req *analyticspb.EvaluateAnalyticsChartRequest) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	if a.prometheusClient == nil {
		return nil, status.Errorf(codes.Unavailable, "metrics client not configured: ForInfraMetric not available")
	}

	metrics := req.AnalyticsChart.DataConfig.GetMetrics()
	switch {
	case len(metrics) == 0:
		return nil, status.Errorf(codes.InvalidArgument, "ForInfraMetric chart requires exactly one metric, got none")
	case len(metrics) > 1:
		return nil, status.Errorf(codes.InvalidArgument, "ForInfraMetric chart requires exactly one metric, got %d", len(metrics))
	case metrics[0].GetForInfraMetric() == nil:
		return nil, status.Errorf(codes.InvalidArgument, "ForInfraMetric chart metric must be ForInfraMetric type")
	}

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	chartDataConfig := req.GetAnalyticsChart().GetDataConfig()
	metricID := metrics[0].GetForInfraMetric().GetMetricId()

	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile config: %w", err)
	}

	metricDef := profileConfig.GetRawProto().GetMetrics()[metricID]
	if metricDef == nil {
		return nil, status.Errorf(codes.NotFound, "metric %q not found in profile config for profile %s", metricID, parent.String())
	}

	tr, err := a.getChartTimeRangeFilter(ctx, chartDataConfig, req.GetMetadata())
	if err != nil {
		return nil, err
	}
	if err := validateInfraMetricsFilters(chartDataConfig.GetFilters()); err != nil {
		return nil, err
	}

	if tr == nil || tr.from == nil {
		return nil, status.Errorf(codes.InvalidArgument, "ForInfraMetric chart requires a time range filter with a start time")
	}
	startTime := tr.from.AsTime()
	endTime := time.Now()
	if tr.to != nil {
		endTime = tr.to.AsTime()
	}
	totalDuration := endTime.Sub(startTime)
	if totalDuration <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "ForInfraMetric chart requires a positive time range (from must precede to)")
	}

	// Extract groupby: ByInfraLabelKeys and/or ByTime; reject all other modes.
	var labelKeys []string
	var byTimeGroupBy *dashboardpb.ChartGroupBy
	for _, groupBy := range chartDataConfig.GroupBy {
		switch groupBy.GroupByMode.(type) {
		case *dashboardpb.ChartGroupBy_ByInfraLabelKeys_:
			labelKeys = groupBy.GetByInfraLabelKeys().GetLabelKeys()
		case *dashboardpb.ChartGroupBy_ByTime:
			byTimeGroupBy = groupBy
		default:
			return nil, status.Errorf(codes.Unimplemented, "ForInfraMetric charts only support ByInfraLabelKeys and ByTime group-by modes, got unsupported type %T", groupBy.GroupByMode)
		}
	}
	promqlTemplate := metricDef.GetPromqlQuery()
	quantiles := metricDef.GetDashboardQuantiles()

	if len(labelKeys) > 0 {
		allowedKeys := make(map[string]struct{}, len(metricDef.GetMetric().GetLabelKeys()))
		for _, k := range metricDef.GetMetric().GetLabelKeys() {
			allowedKeys[k] = struct{}{}
		}
		for _, k := range labelKeys {
			if _, ok := allowedKeys[k]; !ok {
				return nil, status.Errorf(codes.InvalidArgument, "label key %q is not configured for metric %q (allowed: %v)", k, metricID, metricDef.GetMetric().GetLabelKeys())
			}
		}
	}

	if byTimeGroupBy != nil {
		interval := byTimeGroupBy.GetByTime().GetInterval()
		step, err := infraMetricsIntervalToDuration(interval)
		if err != nil {
			return nil, err
		}
		timezoneID := req.GetMetadata().GetTimeZoneId()
		if timezoneID == "" {
			timezoneID = shared.DefaultTimeZone
		}
		loc, err := time.LoadLocation(timezoneID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unknown time_zone_id %q: %s", timezoneID, err)
		}
		startTime = infraMetricsTruncateStartTime(startTime, interval, loc, req.GetMetadata().GetStartDayOfWeek())
		baseQuery := prometheus.PromQLTemplateVars{
			CustomerID: parent.CustomerID,
			ProfileID:  parent.ProfileID,
			Duration:   fmt.Sprintf("%dh", int(step.Hours())),
			GroupBy:    infraMetricsPromQLGroupBy(labelKeys, quantiles),
		}.Substitute(promqlTemplate)

		if len(quantiles) > 0 {
			allMatrices := make([][]prometheus.MatrixResult, 0, len(quantiles))
			for _, q := range quantiles {
				promQuery := fmt.Sprintf("histogram_quantile(%g, (%s))", q, baseQuery)
				result, err := a.prometheusClient.QueryRange(ctx, promQuery, startTime, endTime, step)
				if err != nil {
					return nil, fmt.Errorf("prometheus range query for quantile %g failed: %w", q, err)
				}
				matrices, err := prometheus.ParseMatrix(&result.Data)
				if err != nil {
					return nil, fmt.Errorf("failed to parse prometheus matrix result for quantile %g: %w", q, err)
				}
				allMatrices = append(allMatrices, matrices)
			}
			return infraMetricsToAnalyticsResponse(allMatrices, quantiles, labelKeys)
		}

		result, err := a.prometheusClient.QueryRange(ctx, baseQuery, startTime, endTime, step)
		if err != nil {
			return nil, fmt.Errorf("prometheus range query failed: %w", err)
		}
		matrices, err := prometheus.ParseMatrix(&result.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse prometheus matrix result: %w", err)
		}
		return infraMetricsMatrixToAnalyticsResponse(matrices, labelKeys)
	}

	// Instant query: evaluate at endTime with the full range as the increase() window.
	// Use seconds (not hours) so sub-hour time ranges are not truncated to "0h".
	durationSecs := int64(totalDuration.Seconds())
	if durationSecs < 1 {
		durationSecs = 1
	}
	baseQuery := prometheus.PromQLTemplateVars{
		CustomerID: parent.CustomerID,
		ProfileID:  parent.ProfileID,
		Duration:   fmt.Sprintf("%ds", durationSecs),
		GroupBy:    infraMetricsPromQLGroupBy(labelKeys, quantiles),
	}.Substitute(promqlTemplate)

	if len(quantiles) > 0 {
		allVectors := make([][]prometheus.VectorResult, 0, len(quantiles))
		for _, q := range quantiles {
			promQuery := fmt.Sprintf("histogram_quantile(%g, (%s))", q, baseQuery)
			result, err := a.prometheusClient.Query(ctx, promQuery, &endTime)
			if err != nil {
				return nil, fmt.Errorf("prometheus query for quantile %g failed: %w", q, err)
			}
			vectors, err := prometheus.ParseVector(&result.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse prometheus vector result for quantile %g: %w", q, err)
			}
			allVectors = append(allVectors, vectors)
		}
		return infraMetricsVectorToAnalyticsResponseGrouped(allVectors, quantiles, labelKeys)
	}

	result, err := a.prometheusClient.Query(ctx, baseQuery, &endTime)
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	vectors, err := prometheus.ParseVector(&result.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prometheus vector result: %w", err)
	}
	return infraMetricsVectorToAnalyticsResponse(vectors, labelKeys)
}

// validateInfraMetricsFilters returns UNIMPLEMENTED if any filter contains values
// that are not time-range or duration types. Time-range and duration values are
// consumed by getChartTimeRangeFilter; all other filter types are unsupported.
func validateInfraMetricsFilters(filters []*dashboardpb.ChartDataFilter) error {
	for _, f := range filters {
		for _, val := range f.GetValueSet().GetSingleValues().GetAttributeValues() {
			switch val.GetValue().(type) {
			case *dashboardpb.ChartAttributeValue_TimeRangeFilter,
				*dashboardpb.ChartAttributeValue_Duration:
				// consumed by getChartTimeRangeFilter — allowed
			default:
				return status.Errorf(codes.Unimplemented, "ForInfraMetric charts only support time range and duration filters, unsupported filter value type %T", val.GetValue())
			}
		}
		for _, r := range f.GetValueSet().GetRangeValues().GetRangeValues() {
			switch r.GetFromValue().GetValue().(type) {
			case *dashboardpb.ChartAttributeValue_Timestamp,
				*dashboardpb.ChartAttributeValue_RelativeTimestamp:
				// consumed by getChartTimeRangeFilter — allowed
			default:
				return status.Errorf(codes.Unimplemented, "ForInfraMetric charts only support time range and duration filters, unsupported range filter value type %T", r.GetFromValue().GetValue())
			}
		}
	}
	return nil
}

// infraMetricsIntervalToDuration converts a chart time interval enum to a Go duration.
// MONTH is not supported: month boundaries vary in length and cannot be expressed as a fixed
// Prometheus step without splitting into multiple expensive per-month queries.
func infraMetricsIntervalToDuration(interval dashboardpb.ChartGroupBy_Time_Interval) (time.Duration, error) {
	switch interval {
	case dashboardpb.ChartGroupBy_Time_HOUR:
		return time.Hour, nil
	case dashboardpb.ChartGroupBy_Time_DAY:
		return 24 * time.Hour, nil
	case dashboardpb.ChartGroupBy_Time_WEEK:
		return 7 * 24 * time.Hour, nil
	case dashboardpb.ChartGroupBy_Time_MONTH:
		// Querying exactly along month boundaries is hard to support efficiently.
		// We'd need multiple QueryRange calls. For now let's consider too expensive.
		return 0, status.Errorf(codes.Unimplemented, "ForInfraMetric ByTime does not support MONTH interval")
	default:
		return 24 * time.Hour, nil
	}
}

// infraMetricsTruncateStartTime truncates t to the start of the current interval period in loc.
// startDayOfWeek controls which day begins the week (unspecified defaults to Sunday, matching ES).
// DST transitions are not corrected for.
func infraMetricsTruncateStartTime(t time.Time, interval dashboardpb.ChartGroupBy_Time_Interval, loc *time.Location, startDayOfWeek analyticspb.Metadata_DayOfWeek) time.Time {
	t = t.In(loc)
	y, m, d := t.Date()
	switch interval {
	case dashboardpb.ChartGroupBy_Time_HOUR:
		return time.Date(y, m, d, t.Hour(), 0, 0, 0, loc)
	case dashboardpb.ChartGroupBy_Time_DAY:
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	case dashboardpb.ChartGroupBy_Time_WEEK:
		// Proto DayOfWeek: 1=Mon…6=Sat, 7=Sun, 0=unspecified (default Sunday).
		// Go time.Weekday:  0=Sun, 1=Mon…6=Sat. Conversion: protoDay % 7.
		weekStart := time.Weekday(int(startDayOfWeek) % 7)
		daysBack := (int(t.Weekday()) - int(weekStart) + 7) % 7
		return time.Date(y, m, d-daysBack, 0, 0, 0, 0, loc)
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
}

// infraBuildLabelKeyValues builds, for each label key, the ordered list of distinct values
// seen across all series, plus an index map for O(1) lookup. The order of values within
// each key matches first-appearance order across the series slice.
func infraBuildLabelKeyValues(series []map[string]string, labelKeys []string) (values [][]string, indexOf []map[string]int32) {
	values = make([][]string, len(labelKeys))
	indexOf = make([]map[string]int32, len(labelKeys))
	for i := range labelKeys {
		indexOf[i] = make(map[string]int32)
	}
	for _, metric := range series {
		for i, key := range labelKeys {
			v := metric[key]
			if _, exists := indexOf[i][v]; !exists {
				indexOf[i][v] = int32(len(values[i]))
				values[i] = append(values[i], v)
			}
		}
	}
	return values, indexOf
}

// infraStringGroupByKey converts a slice of string values into a GroupByValuesForSingleKey.
func infraStringGroupByKey(vals []string) *dashboardpb.AnalyticsResult_GroupByValuesForSingleKey {
	gbvs := make([]*dashboardpb.AnalyticsResult_GroupByValue, len(vals))
	for i, v := range vals {
		gbvs[i] = &dashboardpb.AnalyticsResult_GroupByValue{
			Value: &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_StringValue{StringValue: v},
			},
		}
	}
	return &dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{GroupByValues: gbvs}
}

// infraQuantileGroupByKey builds a GroupByValuesForSingleKey for a set of quantile float values.
func infraQuantileGroupByKey(quantiles []float32) *dashboardpb.AnalyticsResult_GroupByValuesForSingleKey {
	gbvs := make([]*dashboardpb.AnalyticsResult_GroupByValue, len(quantiles))
	for i, q := range quantiles {
		gbvs[i] = &dashboardpb.AnalyticsResult_GroupByValue{
			Value: &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_FloatValue{FloatValue: q},
			},
		}
	}
	return &dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{GroupByValues: gbvs}
}

// infraMetricsVectorToAnalyticsResponse converts a Prometheus instant-query vector into an
// EvaluateAnalyticsChartResponse. Charts with no labelKeys return a single ungrouped number;
// otherwise delegates to infraMetricsVectorToAnalyticsResponseGrouped.
func infraMetricsVectorToAnalyticsResponse(vectors []prometheus.VectorResult, labelKeys []string) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	if len(labelKeys) == 0 {
		if len(vectors) > 1 {
			return nil, status.Errorf(codes.Internal, "ForInfraMetric ungrouped instant query returned %d series, expected at most 1; PromQL template should aggregate to a scalar", len(vectors))
		}
		var value float32
		if len(vectors) > 0 {
			if len(vectors[0].Value) < 2 {
				return nil, fmt.Errorf("prometheus vector result has fewer than 2 elements")
			}
			val, err := prometheus.ParseValue(vectors[0].Value[1])
			if err != nil {
				return nil, fmt.Errorf("failed to parse prometheus value: %w", err)
			}
			value = float32(val)
		}
		return &analyticspb.EvaluateAnalyticsChartResponse{
			ResultForChart: &dashboardpb.AnalyticsResultForChart{
				AnalyticsResults: []*dashboardpb.AnalyticsResult{
					{AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{{Value: value}}},
				},
			},
			UpdateTimestamp: timestamppb.New(time.Now()),
		}, nil
	}
	return infraMetricsVectorToAnalyticsResponseGrouped([][]prometheus.VectorResult{vectors}, nil, labelKeys)
}

// infraMetricsVectorToAnalyticsResponseGrouped merges per-quantile instant-query results into
// a single response. When quantiles is non-empty the quantile dimension is appended last:
// [labelKey0, ..., quantile]. When nil, the simpler form [labelKey0, ...] is used.
func infraMetricsVectorToAnalyticsResponseGrouped(allVectors [][]prometheus.VectorResult, quantiles []float32, labelKeys []string) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	totalVectors := 0
	for _, vectors := range allVectors {
		totalVectors += len(vectors)
	}
	allMetrics := make([]map[string]string, 0, totalVectors)
	for _, vectors := range allVectors {
		for _, v := range vectors {
			allMetrics = append(allMetrics, v.Metric)
		}
	}
	keyValues, indexOf := infraBuildLabelKeyValues(allMetrics, labelKeys)

	hasQuantiles := len(quantiles) > 0
	groupByValuesForAllKeys := make([]*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey, 0, len(keyValues)+boolToInt(hasQuantiles))
	for _, vals := range keyValues {
		groupByValuesForAllKeys = append(groupByValuesForAllKeys, infraStringGroupByKey(vals))
	}
	if hasQuantiles {
		groupByValuesForAllKeys = append(groupByValuesForAllKeys, infraQuantileGroupByKey(quantiles))
	}

	quantileSuffixLen := boolToInt(hasQuantiles)
	var analyticsNumbers []*dashboardpb.AnalyticsResult_AnalyticsNumber
	for qIdx, vectors := range allVectors {
		for _, v := range vectors {
			if len(v.Value) < 2 {
				return nil, fmt.Errorf("prometheus vector result has fewer than 2 elements")
			}
			val, err := prometheus.ParseValue(v.Value[1])
			if err != nil {
				if hasQuantiles {
					return nil, fmt.Errorf("failed to parse prometheus value for quantile %g: %w", quantiles[qIdx], err)
				}
				return nil, fmt.Errorf("failed to parse prometheus value: %w", err)
			}
			indexes := make([]int32, len(labelKeys)+quantileSuffixLen)
			for j, key := range labelKeys {
				// A missing label reads as "" from the Prometheus metric map.
				// infraBuildLabelKeyValues registers "" as a distinct value, so missing labels
				// are grouped together under the empty string rather than being dropped.
				indexes[j] = indexOf[j][v.Metric[key]]
			}
			if hasQuantiles {
				indexes[len(labelKeys)] = int32(qIdx)
			}
			analyticsNumbers = append(analyticsNumbers, &dashboardpb.AnalyticsResult_AnalyticsNumber{
				Value:               float32(val),
				GroupByValueIndexes: indexes,
			})
		}
	}

	return infraMetricsWrapResponse(groupByValuesForAllKeys, analyticsNumbers), nil
}

// infraMetricsMatrixToAnalyticsResponse converts a Prometheus range-query matrix into an
// EvaluateAnalyticsChartResponse. Delegates to infraMetricsToAnalyticsResponse.
func infraMetricsMatrixToAnalyticsResponse(matrices []prometheus.MatrixResult, labelKeys []string) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	return infraMetricsToAnalyticsResponse([][]prometheus.MatrixResult{matrices}, nil, labelKeys)
}

// infraMetricsToAnalyticsResponse merges per-quantile range-query results into
// a single response. Timestamps form the first dimension; when quantiles is non-empty the
// quantile dimension is appended last. Dimension order: [timestamp, labelKey0, ..., quantile].
func infraMetricsToAnalyticsResponse(allMatrices [][]prometheus.MatrixResult, quantiles []float32, labelKeys []string) (*analyticspb.EvaluateAnalyticsChartResponse, error) {
	var firstSeries *prometheus.MatrixResult
	for _, matrices := range allMatrices {
		if len(matrices) > 0 {
			firstSeries = &matrices[0]
			break
		}
	}
	if firstSeries == nil {
		return &analyticspb.EvaluateAnalyticsChartResponse{
			ResultForChart: &dashboardpb.AnalyticsResultForChart{
				AnalyticsResults: []*dashboardpb.AnalyticsResult{{}},
			},
			UpdateTimestamp: timestamppb.New(time.Now()),
		}, nil
	}

	// Timestamps are taken from firstSeries and assumed to be identical across all series and
	// quantile queries (Prometheus guarantees this for identical start/end/step parameters).
	timestampGroupByValues := make([]*dashboardpb.AnalyticsResult_GroupByValue, len(firstSeries.Values))
	for i, point := range firstSeries.Values {
		if len(point) < 2 {
			return nil, fmt.Errorf("prometheus point at index %d has fewer than 2 elements", i)
		}
		tsFloat, ok := point[0].(float64)
		if !ok {
			return nil, fmt.Errorf("failed to cast prometheus timestamp at index %d", i)
		}
		timestampGroupByValues[i] = &dashboardpb.AnalyticsResult_GroupByValue{
			Value: &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_Timestamp{
					Timestamp: timestamppb.New(time.Unix(int64(tsFloat), 0)),
				},
			},
		}
	}

	totalSeries := 0
	for _, matrices := range allMatrices {
		totalSeries += len(matrices)
	}
	allMetrics := make([]map[string]string, 0, totalSeries)
	for _, matrices := range allMatrices {
		for _, series := range matrices {
			allMetrics = append(allMetrics, series.Metric)
		}
	}
	keyValues, indexOf := infraBuildLabelKeyValues(allMetrics, labelKeys)

	hasQuantiles := len(quantiles) > 0
	groupByValuesForAllKeys := make([]*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey, 0, 1+len(keyValues)+boolToInt(hasQuantiles))
	groupByValuesForAllKeys = append(groupByValuesForAllKeys, &dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{GroupByValues: timestampGroupByValues})
	for _, vals := range keyValues {
		groupByValuesForAllKeys = append(groupByValuesForAllKeys, infraStringGroupByKey(vals))
	}
	if hasQuantiles {
		groupByValuesForAllKeys = append(groupByValuesForAllKeys, infraQuantileGroupByKey(quantiles))
	}

	quantileSuffixLen := boolToInt(hasQuantiles)
	var analyticsNumbers []*dashboardpb.AnalyticsResult_AnalyticsNumber
	for qIdx, matrices := range allMatrices {
		for _, series := range matrices {
			labelIndexes := make([]int32, len(labelKeys))
			for j, key := range labelKeys {
				// A missing label reads as "" from the Prometheus metric map.
				// infraBuildLabelKeyValues registers "" as a distinct value, so missing labels
				// are grouped together under the empty string rather than being dropped.
				labelIndexes[j] = indexOf[j][series.Metric[key]]
			}
			for tsIdx, point := range series.Values {
				if len(point) < 2 {
					return nil, fmt.Errorf("prometheus point at index %d has fewer than 2 elements", tsIdx)
				}
				val, err := prometheus.ParseValue(point[1])
				if err != nil {
					if hasQuantiles {
						return nil, fmt.Errorf("failed to parse value at timestamp %d for quantile %g: %w", tsIdx, quantiles[qIdx], err)
					}
					return nil, fmt.Errorf("failed to parse value at timestamp %d: %w", tsIdx, err)
				}
				indexes := make([]int32, 1+len(labelKeys)+quantileSuffixLen)
				indexes[0] = int32(tsIdx)
				copy(indexes[1:], labelIndexes)
				if hasQuantiles {
					indexes[1+len(labelKeys)] = int32(qIdx)
				}
				analyticsNumbers = append(analyticsNumbers, &dashboardpb.AnalyticsResult_AnalyticsNumber{
					Value:               float32(val),
					GroupByValueIndexes: indexes,
				})
			}
		}
	}

	return infraMetricsWrapResponse(groupByValuesForAllKeys, analyticsNumbers), nil
}

func infraMetricsWrapResponse(groupByValuesForAllKeys []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey, analyticsNumbers []*dashboardpb.AnalyticsResult_AnalyticsNumber) *analyticspb.EvaluateAnalyticsChartResponse {
	return &analyticspb.EvaluateAnalyticsChartResponse{
		ResultForChart: &dashboardpb.AnalyticsResultForChart{
			AnalyticsResults: []*dashboardpb.AnalyticsResult{
				{
					GroupByValuesForAllKeys: groupByValuesForAllKeys,
					AnalyticsNumbers:        analyticsNumbers,
				},
			},
		},
		UpdateTimestamp: timestamppb.New(time.Now()),
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// infraMetricsPromQLGroupBy returns the comma-separated label keys for PromQL {{group_by}}
// substitution. When quantiles are configured, "le" is appended as the last key if not already
// present in the request group-by.
func infraMetricsPromQLGroupBy(labelKeys []string, quantiles []float32) string {
	groupByKeys := labelKeys
	if len(quantiles) > 0 && !slices.Contains(labelKeys, "le") {
		groupByKeys = append(slices.Clone(labelKeys), "le")
	}
	return strings.Join(groupByKeys, ", ")
}
