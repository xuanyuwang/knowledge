package analyticsimpl

import (
	"container/heap"
	"context"
	"fmt"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	compositeAggDefaultValueCountLimit = envflag.Int("COMPOSITE_AGG_DEFAULT_VALUE_COUNT_LIMIT", 1000, "Default maximum number of buckets to return for composite aggregation if none is provided")
	compositeAggCardinalityLimit       = envflag.Int("COMPOSITE_AGG_CARDINALITY_LIMIT", 100000, "Cutoff for number of distinct values for the field used to determine buckets, at or above which composite aggregation is disabled and terms aggregation is used instead")
)

type SortValuePathComponent int

const (
	DocumentCount SortValuePathComponent = iota
	MetricsConversationCount
	MetricsConversationMessageCount
	MetricsAggOnNumericField
	BucketAggs
	BucketAggsForOrderByMetrics
)

type SortValueBucket struct {
	SortValue float64
	Bucket    *ESRespSingleBucket
}

type BucketHeap struct {
	Data          []*SortValueBucket
	IsMinHeap     bool
	SortValuePath []SortValuePathComponent
}

// These 5 are for implementing the heap interface
func (h BucketHeap) Len() int {
	return len(h.Data)
}
func (h BucketHeap) Less(i int, j int) bool {
	if h.IsMinHeap {
		return h.Data[i].SortValue < h.Data[j].SortValue
	} else {
		return h.Data[i].SortValue > h.Data[j].SortValue
	}
}
func (h *BucketHeap) Swap(i int, j int) {
	h.Data[i], h.Data[j] = h.Data[j], h.Data[i]
}
func (h *BucketHeap) Push(x any) {
	h.Data = append(h.Data, x.(*SortValueBucket))
}
func (h *BucketHeap) Pop() any {
	n := len(h.Data)
	x := h.Data[n-1]
	h.Data = h.Data[0 : n-1]
	return x
}

func (h *BucketHeap) HeapPush(bucket *ESRespSingleBucket) error {
	sortValue := float64(bucket.DocCount)
	node := bucket
	sortValueFound := false
	for _, path := range h.SortValuePath {
		if sortValueFound {
			break
		}
		if node == nil {
			return status.Errorf(codes.Internal, "SortValuePath doesn't match bucket shape")
		}
		switch path {
		case DocumentCount:
			sortValue = float64(node.DocCount)
			sortValueFound = true
		case MetricsConversationCount:
			if node.MetricsConversationCount == nil {
				return status.Errorf(codes.Internal, "SortValuePath doesn't match bucket shape")
			}
			var valid bool
			sortValue, valid = node.MetricsConversationCount.numericValue()
			if !valid {
				return nil
			}
			sortValueFound = true
		case MetricsConversationMessageCount:
			if node.MetricsConversationMessageCount == nil {
				return status.Errorf(codes.Internal, "SortValuePath doesn't match bucket shape")
			}
			var valid bool
			sortValue, valid = node.MetricsConversationMessageCount.numericValue()
			if !valid {
				return nil
			}
			sortValueFound = true
		case MetricsAggOnNumericField:
			if node.MetricsAggOnNumericField == nil {
				return status.Errorf(codes.Internal, "SortValuePath doesn't match bucket shape")
			}
			var valid bool
			sortValue, valid = node.MetricsAggOnNumericField.numericValue()
			if !valid {
				return nil
			}
			sortValueFound = true
		case BucketAggs:
			node = node.BucketAggs
		case BucketAggsForOrderByMetrics:
			node = node.BucketAggsForOrderByMetrics
		default:
			sortValue = float64(node.DocCount)
			sortValueFound = true
		}
	}
	heap.Push(h, &SortValueBucket{Bucket: bucket, SortValue: sortValue})

	return nil
}

func (h *BucketHeap) HeapPop() *ESRespSingleBucket {
	return heap.Pop(h).(*SortValueBucket).Bucket
}

func findParentOfCompositeAgg(aggs *ChartESAggregation) *BucketAggregation {
	for aggs != nil && aggs.BucketAggs != nil {
		if aggs.BucketAggs.Composite != nil {
			return aggs.BucketAggs
		}
		aggs = aggs.BucketAggs.Aggs
	}

	return nil
}

func compositeAggregationGetSortValuePath(chartDataConfig *dashboardpb.ChartDataConfig, metricsAgg, mergedAggs *ChartESAggregation) ([]SortValuePathComponent, error) {
	if len(chartDataConfig.Metrics) > 1 {
		return []SortValuePathComponent{}, status.Errorf(codes.InvalidArgument, "multiple metrics not supported")
	}

	defaultPathComponent := DocumentCount

	// Fall back to sort by default doc_count for occurrence-count metrics, where the sorting logic is
	// not well defined. Multiple scorecard templates and criteria are handled instead by ordering on a
	// sibling aggregate over every selected template/criterion (metricsAgg.orderByBucketAggsOverride).
	if metricsAggUsesDocCount(metricsAgg) {
		return []SortValuePathComponent{defaultPathComponent}, nil
	}

	node := findParentOfCompositeAgg(mergedAggs)

	// Require that there is a composite aggregation
	if node == nil {
		return []SortValuePathComponent{}, status.Errorf(codes.InvalidArgument, "expected a composite aggregation")
	}

	return getSortValuePathFromNode(node, defaultPathComponent)
}

func (a AnalyticsServiceImpl) topBottomNCompositeAgg(ctx context.Context, groupByConfig *dashboardpb.ChartGroupBy_Config, indexName string, esRequest *ChartESRequest, sortValuePath []SortValuePathComponent, queryID string) (ChartESResponse, error) {
	direction := sortDirectionDesc
	size := *compositeAggDefaultValueCountLimit
	if groupByConfig != nil {
		direction = sortDirection(groupByConfig.SortDirection)
		if count := groupByConfig.ValueCountLimit; count != 0 {
			size = int(count)
		}
	}

	parentOfCompositeAgg := findParentOfCompositeAgg(esRequest.Aggs)
	if parentOfCompositeAgg == nil {
		return ChartESResponse{}, status.Errorf(codes.InvalidArgument, "expected a composite aggregation")
	}
	requestCompositeAgg := parentOfCompositeAgg.Composite

	combinedBuckets := &BucketHeap{Data: []*SortValueBucket{}, IsMinHeap: direction == sortDirectionDesc, SortValuePath: sortValuePath}
	heap.Init(combinedBuckets)

	esResp := ChartESResponse{}

	for {
		err := elasticsearch.Query(
			ctx, a.esClient, a.logger,
			indexName,
			elasticsearch.EsModeCount, 0 /*=pageSize*/, true, esRequest, &esResp, queryID)
		if err != nil {
			return ChartESResponse{}, err
		}

		node, err := traverseToDesiredAggInESResp(esResp.Aggregations, multiBucketAggType)
		if err != nil {
			return ChartESResponse{}, err
		}

		for _, bucket := range *node.Buckets {
			if err := combinedBuckets.HeapPush(&bucket); err != nil {
				return ChartESResponse{}, err
			}
			if len(combinedBuckets.Data) > size {
				combinedBuckets.HeapPop()
			}
		}

		afterKey := node.AfterKey
		if afterKey == nil || afterKey.BucketAggs == nil {
			break
		}
		requestCompositeAgg.After = afterKey
		esResp.Aggregations = nil
	}

	finalBuckets := make([]ESRespSingleBucket, len(combinedBuckets.Data))
	for i := len(finalBuckets) - 1; i >= 0; i-- {
		finalBuckets[i] = *combinedBuckets.HeapPop()
		finalBuckets[i].Key = finalBuckets[i].Key.(map[string]any)["bucket_aggs"]
	}
	node, err := traverseToDesiredAggInESResp(esResp.Aggregations, multiBucketAggType)
	if err != nil {
		return ChartESResponse{}, err
	}
	node.Buckets = &finalBuckets
	node.AfterKey = nil

	return esResp, nil
}

func (a AnalyticsServiceImpl) shouldUseCompositeAggregation(ctx context.Context, req *analyticspb.EvaluateAnalyticsChartRequest, esQuery *ChartESQuery, indexName string, profile shared.CustomerProfile, precisionThresholdForCardAgg int, metricsAgg *ChartESAggregation) (bool, error) {
	enableCompositeAggregationTopBottomN, err := a.featureFlagClient.BoolValue(ctx, "enableCompositeAggregationTopBottomN", false)
	if err != nil {
		a.logger.Warnf(ctx, "feature flag enableCompositeAggregationTopBottomN fallback used: %v", err)
	}

	chartDataConfig := req.GetAnalyticsChart().GetDataConfig()

	if enableCompositeAggregationTopBottomN && len(chartDataConfig.GroupBy) >= 1 && chartDataConfig.GroupBy[0].GetByUniqueValue() != nil {
		// Block off if metric uses doc_count
		if metricsAggUsesDocCount(metricsAgg) {
			return false, nil
		}

		// Block off if aggregation is MAX with sort direction desc or MIN with sort direction asc
		groupByConfig := chartDataConfig.GroupBy[0].Config
		aggregationOperator := getAggregationOperator(chartDataConfig)
		direction := sortDirectionDesc
		if groupByConfig != nil {
			direction = sortDirection(groupByConfig.SortDirection)
		}
		if (aggregationOperator == dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MAX && direction == sortDirectionDesc) ||
			(aggregationOperator == dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MIN && direction == sortDirectionAsc) {
			return false, nil
		}

		// Check if number of unique values for the field used to generate the buckets is too high for terms aggregation
		cardinalityAggs, err := a.convertGroupByUniqueValueToCardinalityAggregation(ctx, profile, chartDataConfig.GroupBy[0].GetByUniqueValue(), precisionThresholdForCardAgg)
		if err != nil {
			return false, err
		}

		esRequest := ChartESRequest{
			Query: esQuery,
			Aggs:  cardinalityAggs,
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

		esResp := ChartESResponse{}
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
			fmt.Sprintf("Check cardinality of group by unique values for analytics chart %v", req.GetAnalyticsChart().GetName()))
		if err != nil {
			return false, err
		}

		node, err := traverseToDesiredAggInESResp(esResp.Aggregations, metricsAggType)
		if err != nil {
			return false, err
		}

		fieldCardinality, valid := node.MetricsBucketFieldCardinality.numericValue()
		if !valid {
			return false, fmt.Errorf("metrics_bucket_field_cardinality should have a value")
		}

		return int(fieldCardinality) < *compositeAggCardinalityLimit, nil
	} else {
		return false, nil
	}
}

func (a AnalyticsServiceImpl) convertGroupByUniqueValueToCardinalityAggregation(ctx context.Context, profile shared.CustomerProfile, byUniqueValue *dashboardpb.ChartGroupBy_UniqueValue, precisionThresholdForCardAgg int) (*ChartESAggregation, error) {

	if byUniqueValue.GetAttribute() == dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT {
		return &ChartESAggregation{
			MetricsBucketFieldCardinality: &elasticsearch.CardinalityCount{
				Cardinality: elasticsearch.Cardinality{
					Field:              proto.String("agent_user_id.keyword"),
					PrecisionThreshold: precisionThresholdForCardAgg,
				},
			},
		}, nil
	}
	if metadata := byUniqueValue.GetMetadata(); metadata != nil {
		if len(metadata.Values) > 0 || len(metadata.NumericValueRanges) > 0 {
			return nil, status.Errorf(codes.InvalidArgument, "no need to provide values when grouping by unique values of metadata")
		}
		momentName, err := momentpb.ParseMomentName(metadata.MomentName)
		if err != nil {
			return nil, err
		}
		fields, err := a.metadataValueFieldForChart(ctx, profile.String(), []string{metadata.MomentName})
		if err != nil {
			return nil, err
		}

		nestedAggs := ChartESAggregation{
			BucketAggs: &BucketAggregation{
				Nested: &NestedDocumentPath{
					Path: elasticsearch.MomentAnnotationsPath,
				},
				Aggs: &ChartESAggregation{
					BucketAggs: &BucketAggregation{
						Filter: elasticsearch.NestedFilter{
							Terms: &elasticsearch.NestedTermsFilter{
								MomentTemplateIDs: &[]string{momentName.MomentID},
							},
						},
						Aggs: &ChartESAggregation{
							MetricsBucketFieldCardinality: &elasticsearch.CardinalityCount{
								Cardinality: elasticsearch.Cardinality{
									Field:              proto.String(fields[0]),
									PrecisionThreshold: precisionThresholdForCardAgg,
								},
							},
						},
					},
				},
			},
		}
		return &nestedAggs, nil
	}
	if llmAnnotation := byUniqueValue.GetLlmAnnotation(); llmAnnotation != nil {
		if len(llmAnnotation.Values) > 0 || len(llmAnnotation.NumericValueRanges) > 0 {
			return nil, status.Errorf(codes.InvalidArgument, "no need to provide values when grouping by unique values of llm annotation")
		}
		momentName, err := momentpb.ParseMomentName(llmAnnotation.MomentName)
		if err != nil {
			return nil, err
		}
		field, err := a.llmAnnotationValueFieldForChart(ctx, llmAnnotation.MomentName)
		if err != nil {
			return nil, err
		}

		nestedAggs := ChartESAggregation{
			BucketAggs: &BucketAggregation{
				Nested: &NestedDocumentPath{
					Path: elasticsearch.MomentAnnotationsPath,
				},
				Aggs: &ChartESAggregation{
					BucketAggs: &BucketAggregation{
						Filter: elasticsearch.NestedFilter{
							Terms: &elasticsearch.NestedTermsFilter{
								MomentTemplateIDs: &[]string{momentName.MomentID},
							},
						},
						Aggs: &ChartESAggregation{
							MetricsBucketFieldCardinality: &elasticsearch.CardinalityCount{
								Cardinality: elasticsearch.Cardinality{
									Field:              proto.String(field),
									PrecisionThreshold: precisionThresholdForCardAgg,
								},
							},
						},
					},
				},
			},
		}
		return &nestedAggs, nil
	}
	return nil, status.Errorf(codes.InvalidArgument, "no field to check cardinality of is provided")
}

func getAggregationOperator(chartDataConfig *dashboardpb.ChartDataConfig) dashboardpb.AggregationOperator {
	return metricAggregationOperator(chartDataConfig.Metrics[0])
}

// metricAggregationOperator returns how a single metric combines the values inside a bucket, which
// is also how those values have to be combined across buckets. Split out of getAggregationOperator
// so combo charts can ask about a metric other than Metrics[0].
func metricAggregationOperator(chartMetric *dashboardpb.ChartMetric) dashboardpb.AggregationOperator {
	switch chartMetric.GetMetric().(type) {
	case *dashboardpb.ChartMetric_ForConversation_:
		statsType := chartMetric.GetForConversation().GetStatsType()
		return statsType.GetAggregationOnConversationAttribute().GetAggregationOperator()
	case *dashboardpb.ChartMetric_ForMetadata_:
		statsType := chartMetric.GetForMetadata().GetStatsType()
		return statsType.GetAggregationOnMetadataValue().GetAggregationOperator()
	case *dashboardpb.ChartMetric_ForLlmAnnotation_:
		statsType := chartMetric.GetForLlmAnnotation().GetStatsType()
		return statsType.GetAggregationOnValue().GetAggregationOperator()
	case *dashboardpb.ChartMetric_ForScorecardTemplate_:
		statsType := chartMetric.GetForScorecardTemplate().GetStatsType()
		return statsType.GetAggregationOnScore()
	case *dashboardpb.ChartMetric_ForScorecardCriterion_:
		statsType := chartMetric.GetForScorecardCriterion().GetStatsType()
		return statsType.GetAggregationOnScore()
	}

	return dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_UNSPECIFIED
}
