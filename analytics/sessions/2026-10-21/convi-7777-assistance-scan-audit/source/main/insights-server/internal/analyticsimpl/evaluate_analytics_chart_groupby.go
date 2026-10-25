package analyticsimpl

import (
	"context"
	"fmt"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/shared-go/util/envflag"

	"github.com/jinzhu/copier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

var (
	termsAggShardSizeMultiplier = envflag.Float64("TERMS_AGG_SHARD_SIZE_MULTIPLIER", -1, "Multiplier on size of terms aggregation")
	compositeAggPageSize        = envflag.Int("COMPOSITE_AGG_PAGE_SIZE", 1000, "Page size to use for composite aggregation")
)

type ChartESAggregation struct {
	// Aggregation name for single|multi bucket aggregation.
	BucketAggs *BucketAggregation `json:"bucket_aggs,omitempty"`

	// Bucket aggregation for orderby metrics only. Adding a new field
	// so that we can have both bucket aggregation for groupby and bucket
	// aggregation for orderby metrics for terms aggregation at the same time.
	BucketAggsForOrderByMetrics *BucketAggregation `json:"bucket_aggs_for_order_by_metrics,omitempty"`

	// orderByBucketAggsOverride, when set, is used by orderByMetricAggFromChartMetricsAgg as the
	// order-by-metric sibling (BucketAggsForOrderByMetrics) placed at the inner group-by, instead of
	// copying BucketAggs. This is for the multi-template scorecard AggregationOnScore case, where the
	// per-template output metric (BucketAggs = filters, one bucket per template) differs from the single
	// aggregate used to order the outer unique-value group-by (one filter over every selected template
	// id). It is unexported so it is never serialized into the ES request directly.
	orderByBucketAggsOverride *BucketAggregation

	// For Metrics Aggregation.
	MetricsConversationCount        *elasticsearch.CardinalityCount `json:"metrics_conversation_count,omitempty"`
	MetricsConversationMessageCount *elasticsearch.CardinalityCount `json:"metrics_conversation_message_count,omitempty"`
	MetricsBucketFieldCardinality   *elasticsearch.CardinalityCount `json:"metrics_bucket_field_cardinality,omitempty"`
	// Aggregation on numeric field of conversations, eg convo handle time.
	MetricsAggOnNumericField *elasticsearch.AggregationOnNumericField `json:"metrics_agg_on_numeric_field,omitempty"`
	// Some metrics need to distinguish no data from a real zero value. For those,
	// a null ES metric value should not produce an AnalyticsNumber.
	omitNullMetricValue bool
	// Built-in doc_count is used as occurrence_count. No extra custom ES metrics needed.
	// occurrence_count is computed as doc_count of nesting document (message or conversation).
	isTotalOccurrenceCount bool
	// Built-in doc_count is used as conversation_count when aggregating on conversation documents.
	isConversationCountViaDocCount bool
	// stats type is average occurrence count.
	isAverageOccurrenceCountPerConvo bool
}

// The path that specified which nested document:
// moment annotation, action annotation, conversation events.
type NestedDocumentPath struct {
	Path string `json:"path"`
}

// An aggregation that requires a key as name.
// Could be single-bucket aggs or multiple bucket aggs.
// TODO(zhucaiyi): merge BucketAggregation and ChartESAggregation
type BucketAggregation struct {
	// "filters" aggregation. A multi-bucket aggregation
	// where each bucket contains the documents that match a query.
	Filters *FiltersAggregation `json:"filters,omitempty"`
	// "date_histogram" aggregation. A multi-bucket aggregation where each bucket
	// contains documents whose specifie time field falls into a time interval.
	DateHistogram *elasticsearch.ReqDateHistogram `json:"date_histogram,omitempty"`
	/*
		4 scenarios: document_type=Conversation|Message * groupby=convo_agent|metadata
	*/
	// "terms" aggregation. A multi-bucket aggregation on a specific attribute of document.
	// Only support terms aggregation on convo_agent, metadata_value.
	Terms *elasticsearch.TermsAggs `json:"terms,omitempty"`

	Composite *CompositeAggregation `json:"composite,omitempty"`

	/*
		Single bucket aggregation
	*/
	// "nested" aggregation. A single bucket aggregation which switch context from
	// a document to its nested document. (eg Conversation-> MomentAnnotation)
	Nested *NestedDocumentPath `json:"nested,omitempty"`

	// "reverse_nested" aggregation. A single bucket aggregation which switch context
	// from a nested document to its nesting document. (eg MomentAnnotation->Conversation)
	ReverseNested *elasticsearch.EmptyStruct `json:"reverse_nested,omitempty"`

	// A single bucket aggregation that narrows the set of documents to those that match a query.
	// This value can be any valid ES query.
	Filter any `json:"filter,omitempty"`

	// ES "children" aggregation, used to switch context from conversation doc to message doc.
	Children *ChildrenAggregation `json:"children,omitempty"`

	// for sub-aggregation, can be metrics or bucket aggregation.
	Aggs *ChartESAggregation `json:"aggs,omitempty"`
}

// Elasticsearch "children" bucket aggregation, used to switch context from parent doc to child doc.
type ChildrenAggregation struct {
	Type string `json:"type,omitempty"`
}

// ES bucket aggregation of "filters"
// Bucketizing based on provided values, each value prepresents a filter.
// This corresponds to "group by provided value", the type is "any" because the
// provided value can be different Cresta entities.
type FiltersAggregation struct {
	Filters *[]any `json:"filters,omitempty"`
}

type CompositeAggregation struct {
	Sources *[]ChartESAggregation `json:"sources,omitempty"`
	After   *BucketAggsKey        `json:"after,omitempty"`
	Size    int32                 `json:"size,omitempty"`
}

type BucketAggsKey struct {
	// BucketAggs Key can be in the form of string|number|boolean|nil
	// - number: group by metadata value and group by time.
	// - string: group by string type metadata value
	// - boolean: group by boolean type metadata value
	// - nil: not populated when group by provided value
	BucketAggs *any `json:"bucket_aggs" mapstructure:"bucket_aggs"`
}

type esAggDocTypeParams struct {
	// The starting context of bucket aggregation. This is also the context of ES query.
	startType elasticsearch.DocumentType
	// The ending context of bucket aggregation. This is also the starting context of ES metrics.
	endType elasticsearch.DocumentType
}

func needOrderByMetricAgg(groupBys []*dashboardpb.ChartGroupBy, metricsAgg *ChartESAggregation) bool {
	ret := false
	if len(groupBys) >= 2 {
		gb := groupBys[0]
		// For grouping by unique values.
		if gb.GetByUniqueValue() != nil && !metricsAggUsesDocCount(metricsAgg) {
			ret = true
		}
		// For grouping by provided values and grouping by time.
		if gb.GetByUniqueValue() == nil && gb.Config != nil && gb.Config.SortDirection != dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED {
			ret = true
		}
	}
	return ret
}

func (a AnalyticsServiceImpl) convertGroupBys(ctx context.Context, profile shared.CustomerProfile, profileConfig *config.ProfileConfig, timeRange *timeRange, metadata *analyticspb.Metadata, bucketAggDocType esAggDocTypeParams, groupBys []*dashboardpb.ChartGroupBy, metricsAgg *ChartESAggregation, precisionThresholdForCardAgg int, prefetchedTopics map[string]elasticsearch.TopicDetails, prefetchedAggregateExpansions map[string][]string, prefetchedV3ClusterClosures map[string][]string, prefetchedGroupMemberIDs map[string][]string, enableCompositeAggregationTopBottomN bool) (*ChartESAggregation, error) {
	if len(groupBys) > 2 {
		return nil, status.Errorf(codes.InvalidArgument, "more than 2 groupbys (including the one from metrics) is not allowed")
	}

	mergeBucketAggs := func(esAggs *ChartESAggregation, bucketAggForGroupby *BucketAggregation, orderByMetricAgg *ChartESAggregation) {
		node := esAggs
		for {
			switch {
			case node.BucketAggs == nil:
				node.BucketAggs = bucketAggForGroupby
				return
			case node.BucketAggs.Aggs == nil:
				agg := &ChartESAggregation{}
				if orderByMetricAgg != nil {
					agg = orderByMetricAgg
				}
				agg.BucketAggs = bucketAggForGroupby
				node.BucketAggs.Aggs = agg
				return
			default:
				node = node.BucketAggs.Aggs
			}
		}
	}

	ret := &ChartESAggregation{}
	for i, groupBy := range groupBys {
		var (
			bucketAgg *BucketAggregation
			err       error
		)
		switch groupBy.GroupByMode.(type) {
		case *dashboardpb.ChartGroupBy_ByTime:
			dateHistogram, err := convertGroupByTime(groupBy.GetByTime(), timeRange, metadata)
			if err != nil {
				return nil, err
			}
			bucketAgg = &BucketAggregation{
				DateHistogram: &dateHistogram,
			}
		case *dashboardpb.ChartGroupBy_ByProvidedValue:
			bucketAgg, err = a.convertGroupByProvidedValuesToBucketAgg(ctx, profile, profileConfig, bucketAggDocType, groupBy.GetByProvidedValue(), prefetchedTopics, prefetchedAggregateExpansions, prefetchedV3ClusterClosures, prefetchedGroupMemberIDs)
			if err != nil {
				return nil, err
			}
		case *dashboardpb.ChartGroupBy_ByUniqueValue:
			if i != 0 {
				return nil, status.Errorf(codes.InvalidArgument, "Only allow grouping by unique values as first groupby")
			}
			if enableCompositeAggregationTopBottomN {
				bucketAgg, err = a.convertGroupByUniqueValueCompositeAggregation(ctx, profile, groupBy.GetByUniqueValue(), groupBy.Config)
			} else {
				bucketAgg, err = a.convertGroupByUniqueValue(ctx, profile, groupBy.GetByUniqueValue(), groupBy.Config)
			}
			if err != nil {
				return nil, err
			}
		default:
			return nil, status.Errorf(codes.Unimplemented, "group by mode %t not implemented yet", groupBy.GroupByMode)

		}

		var orderByMetricAggs *ChartESAggregation
		if i == 1 && needOrderByMetricAgg(groupBys, metricsAgg) {
			orderByMetricAggs, err = orderByMetricAggFromChartMetricsAgg(metricsAgg, bucketAggDocType.startType, precisionThresholdForCardAgg)
			if err != nil {
				return nil, err
			}
		}
		mergeBucketAggs(ret, bucketAgg, orderByMetricAggs)
	}
	return ret, nil
}

func orderByMetricAggFromChartMetricsAgg(chartMetricsAgg *ChartESAggregation, startDocType elasticsearch.DocumentType, precisionThresholdForCardAgg int) (*ChartESAggregation, error) {
	orderByMetricAgg := &ChartESAggregation{}
	err := copier.Copy(orderByMetricAgg, chartMetricsAgg)
	if err != nil {
		return nil, err
	}
	// Multi-template scorecard AggregationOnScore: order the outer unique-value group-by by a single
	// aggregate over every selected template id, not by the per-template output metric. The override
	// already carries the nested -> filter(all ids) -> metric shape, so use it directly and drop the
	// bare per-template metric copied above.
	if override := chartMetricsAgg.orderByBucketAggsOverride; override != nil {
		orderByMetricAgg.BucketAggsForOrderByMetrics = override
		orderByMetricAgg.MetricsAggOnNumericField = nil
		return orderByMetricAgg, nil
	}
	if val := orderByMetricAgg.BucketAggs; val != nil {
		orderByMetricAgg.BucketAggsForOrderByMetrics = val
		orderByMetricAgg.BucketAggs = nil
	}
	// TODO(zhucaiyi): remove once we use explicit custom metrics rather than built-in doc_count for occurrence_count.
	if orderByMetricAgg.isTotalOccurrenceCount {
		switch startDocType {
		case elasticsearch.DocumentTypeMessage, elasticsearch.DocumentTypeMessageGroup:
			orderByMetricAgg.MetricsConversationMessageCount = elasticsearch.CreateReqTotalMessageCount(precisionThresholdForCardAgg)
		case elasticsearch.DocumentTypeConversation:
			orderByMetricAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
		}
	}
	return orderByMetricAgg, nil
}

const defaultOrderByPath = "_count"
const sortDirectionDesc = "desc"
const sortDirectionAsc = "asc"

func sortDirection(sortDir dashboardpb.ChartGroupBy_Config_SortDirection) string {
	direction := sortDirectionDesc
	if sortDir == dashboardpb.ChartGroupBy_Config_ASC {
		direction = sortDirectionAsc
	}
	return direction
}

func (a AnalyticsServiceImpl) convertGroupByUniqueValue(ctx context.Context, profile shared.CustomerProfile, byUniqueValue *dashboardpb.ChartGroupBy_UniqueValue, groupByConfig *dashboardpb.ChartGroupBy_Config) (*BucketAggregation, error) {
	// TODO(zhucaiyi): in addition to start_doc_type, we should also pass in a end_doc_type as parameter for each groupby
	// to be more generic.
	direction := sortDirectionDesc
	size := int32(1000)
	if groupByConfig != nil {
		direction = sortDirection(groupByConfig.SortDirection)
		if count := groupByConfig.ValueCountLimit; count != 0 {
			size = count
		}
	}
	var shardSize *int32
	// ES default multiplier is 1.5.
	// Do not allow smaller than default shard_size which will affect accuracy of results.
	if m := *termsAggShardSizeMultiplier; m > 1.5 {
		shardSize = proto.Int32(int32(m*float64(size) + 10))
	}

	if byUniqueValue.GetAttribute() == dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT {
		// order by path is updated later in correctOrderByForTermsAgg method.
		orderByMetricsPath := defaultOrderByPath
		orderByMap := map[string]string{orderByMetricsPath: direction}
		return &BucketAggregation{
			Terms: &elasticsearch.TermsAggs{
				Field:     proto.String("agent_user_id.keyword"),
				Size:      proto.Int32(size),
				ShardSize: shardSize,
				OrderBy:   orderByMap,
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
		// order by path is updated later in correctOrderByForTermsAgg method.
		orderByMetricsPath := defaultOrderByPath
		orderByMap := map[string]string{orderByMetricsPath: direction}

		nestedAggs := BucketAggregation{
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
						BucketAggs: &BucketAggregation{
							Terms: &elasticsearch.TermsAggs{
								Field:     proto.String(fields[0]),
								Size:      proto.Int32(size),
								ShardSize: shardSize,
								OrderBy:   orderByMap,
							},
							Aggs: &ChartESAggregation{
								BucketAggs: &BucketAggregation{
									ReverseNested: &elasticsearch.EmptyStruct{},
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
		// order by path is updated later in correctOrderByForTermsAgg method.
		orderByMetricsPath := defaultOrderByPath
		orderByMap := map[string]string{orderByMetricsPath: direction}

		nestedAggs := BucketAggregation{
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
						BucketAggs: &BucketAggregation{
							Terms: &elasticsearch.TermsAggs{
								Field:     proto.String(field),
								Size:      proto.Int32(size),
								ShardSize: shardSize,
								OrderBy:   orderByMap,
							},
							Aggs: &ChartESAggregation{
								BucketAggs: &BucketAggregation{
									ReverseNested: &elasticsearch.EmptyStruct{},
								},
							},
						},
					},
				},
			},
		}
		return &nestedAggs, nil
	}
	return nil, nil
}

func (a AnalyticsServiceImpl) convertGroupByUniqueValueCompositeAggregation(ctx context.Context, profile shared.CustomerProfile, byUniqueValue *dashboardpb.ChartGroupBy_UniqueValue, groupByConfig *dashboardpb.ChartGroupBy_Config) (*BucketAggregation, error) {
	pageSize := int32(*compositeAggPageSize)

	if byUniqueValue.GetAttribute() == dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT {
		return &BucketAggregation{
			Composite: &CompositeAggregation{
				Sources: &[]ChartESAggregation{
					{
						BucketAggs: &BucketAggregation{
							Terms: &elasticsearch.TermsAggs{
								Field: proto.String("agent_user_id.keyword"),
							},
						},
					},
				},
				Size: pageSize,
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

		nestedAggs := BucketAggregation{
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
						BucketAggs: &BucketAggregation{
							Composite: &CompositeAggregation{
								Sources: &[]ChartESAggregation{
									{
										BucketAggs: &BucketAggregation{
											Terms: &elasticsearch.TermsAggs{
												Field: proto.String(fields[0]),
											},
										},
									},
								},
								Size: pageSize,
							},
							Aggs: &ChartESAggregation{
								BucketAggs: &BucketAggregation{
									ReverseNested: &elasticsearch.EmptyStruct{},
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

		nestedAggs := BucketAggregation{
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
						BucketAggs: &BucketAggregation{
							Composite: &CompositeAggregation{
								Sources: &[]ChartESAggregation{
									{
										BucketAggs: &BucketAggregation{
											Terms: &elasticsearch.TermsAggs{
												Field: proto.String(field),
											},
										},
									},
								},
								Size: pageSize,
							},
							Aggs: &ChartESAggregation{
								BucketAggs: &BucketAggregation{
									ReverseNested: &elasticsearch.EmptyStruct{},
								},
							},
						},
					},
				},
			},
		}
		return &nestedAggs, nil
	}
	return nil, nil
}

func (a AnalyticsServiceImpl) metadataValueFieldForChart(ctx context.Context, profileName string, momentNames []string) ([]string, error) {
	batchGetReq := &momentpb.BatchGetMomentsRequest{
		Parent: profileName,
		Names:  momentNames,
	}
	batchgetResp, err := a.momentClientRegistry.Client(ctx).BatchGetMoments(ctx, batchGetReq)
	if err != nil {
		return nil, status.Errorf(status.Code(err), "calling BatchGetMoments() with request %v failed: %v", batchGetReq, err)
	}
	if len(batchgetResp.Moments) != len(momentNames) {
		return nil, fmt.Errorf("expecting one moment to be returned, but found %v", len(batchgetResp.Moments))
	}

	ret := []string{}

	for _, moment := range batchgetResp.Moments {
		// TODO(zhucaiyi): also handle conversation outcome moment.
		if moment.Type != momentpb.Moment_CONVERSATION_METADATA {
			return nil, fmt.Errorf("type of moment must be Moment_CONVERSATION_METADATA, but found %v", moment.Type)
		}
		metadataValueField, err := a.getMetadataMomentValueField(moment.MomentData.GetConversationMetadataMomentData())
		if err != nil {
			return nil, err
		}
		ret = append(ret, metadataValueField)
	}

	return ret, nil
}

// llmAnnotationValueFieldForChart resolves the ES value field path to group by,
// based on the LLM annotation moment's value type.
func (a AnalyticsServiceImpl) llmAnnotationValueFieldForChart(ctx context.Context, momentName string) (string, error) {
	getMomentReq := &momentpb.GetMomentRequest{
		Name: momentName,
	}
	getMomentResp, err := a.momentClientRegistry.Client(ctx).GetMoment(ctx, getMomentReq)
	if err != nil {
		return "", status.Errorf(status.Code(err), "calling GetMoment() with request %v failed: %v", getMomentReq, err)
	}
	moment := getMomentResp.GetMoment()
	if moment == nil {
		return "", status.Errorf(codes.NotFound, "moment %q not found", momentName)
	}
	if moment.Type != momentpb.Moment_LLM_ANNOTATION {
		return "", status.Errorf(codes.InvalidArgument, "type of moment must be Moment_LLM_ANNOTATION, but found %v", moment.Type)
	}
	switch moment.MomentData.GetLlmAnnotationMomentData().GetValueType() {
	case momentpb.LlmAnnotationMomentData_STRING:
		return elasticsearch.LlmAnnotationPayloadStringValueField, nil
	case momentpb.LlmAnnotationMomentData_NUMBER:
		if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
			return elasticsearch.LlmAnnotationPayloadNumberValueFieldExplicitMapping, nil
		}
		return elasticsearch.LlmAnnotationPayloadNumberValueField, nil
	case momentpb.LlmAnnotationMomentData_BOOLEAN:
		return elasticsearch.LlmAnnotationPayloadBooleanValueField, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "unsupported llm annotation value type %v", moment.MomentData.GetLlmAnnotationMomentData().GetValueType())
	}
}

// wrapBucketFilterWithExclude wraps a bucket filter in a bool that also applies must_not exclude filters,
// so each bucket matches the same exclude semantics as the chart filter.
func wrapBucketFilterWithExclude(bucketFilter any, excludeFilters []any) any {
	if len(excludeFilters) == 0 {
		return bucketFilter
	}
	boolClause := map[string]any{
		"must_not": excludeFilters,
	}
	if bucketFilter != nil {
		boolClause["filter"] = []any{bucketFilter}
	}
	return map[string]any{
		"bool": boolClause,
	}
}

func (a AnalyticsServiceImpl) convertGroupByProvidedValuesToBucketAgg(ctx context.Context, profile shared.CustomerProfile, profileConfig *config.ProfileConfig, bucketAggDocType esAggDocTypeParams, providedValue *dashboardpb.ChartGroupBy_ProvidedValue, prefetchedTopics map[string]elasticsearch.TopicDetails, prefetchedAggregateExpansions map[string][]string, prefetchedV3ClusterClosures map[string][]string, prefetchedGroupMemberIDs map[string][]string) (*BucketAggregation, error) {
	documentType := bucketAggDocType.startType
	if documentType != elasticsearch.DocumentTypeConversation && documentType != elasticsearch.DocumentTypeMessage {
		return nil, status.Errorf(codes.InvalidArgument, "target document type has to be message or conversation, found %v", documentType)
	}
	cvse, err := fromChartValueSet(ctx, providedValue.ValueSet)
	if err != nil {
		return nil, err
	}
	filters := []any{}
	switch cvse.attributeValueType {
	case crestaEntityAgentAssistance:
		for _, aa := range cvse.agentAssistances {
			f, err := filterFromAgentAssistance(ctx, aa, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case crestaEntitySubcategory:
		topicBools, topicBoolsExclude, err := a.convertSubcategories(ctx, profile.String(), cvse.subcategories, cvse.subcategoriesExclude, prefetchedTopics)
		if err != nil {
			return nil, err
		}
		for _, tb := range topicBools {
			f, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{tb}, documentType, false)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
		for _, tb := range topicBoolsExclude {
			fExclude, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{tb}, documentType, false)
			if err != nil {
				return nil, err
			}
			filters = append(filters, wrapBucketFilterWithExclude(nil, []any{fExclude}))
		}
	case crestaEntityAggregateSubcategory:
		topicBoolsByAggregate, topicBoolsByAggregateExclude, err := a.convertAggregateSubcategory(ctx, profile, cvse.aggregateSubcategories, cvse.aggregateSubcategoriesExclude, prefetchedTopics, prefetchedAggregateExpansions)
		if err != nil {
			return nil, err
		}
		for _, aggSub := range cvse.aggregateSubcategories {
			topicBools := topicBoolsByAggregate[aggSub.MomentName]
			var f any
			if len(topicBools) == 0 {
				f = createMatchNoneFilter(documentType)
			} else {
				f, err = a.filterFromTopicBools(ctx, topicBools, documentType, false)
				if err != nil {
					return nil, err
				}
			}
			filters = append(filters, f)
		}
		for _, aggSub := range cvse.aggregateSubcategoriesExclude {
			aggTopicBools := topicBoolsByAggregateExclude[aggSub.MomentName]
			var fExclude any
			if len(aggTopicBools) == 0 {
				fExclude = createMatchNoneFilter(documentType)
			} else {
				fExclude, err = a.filterFromTopicBools(ctx, aggTopicBools, documentType, false)
				if err != nil {
					return nil, err
				}
			}
			filters = append(filters, wrapBucketFilterWithExclude(nil, []any{fExclude}))
		}
	case crestaEntityAggregateAndLeafSubcategory:
		topicBoolsForSubcategories, topicBoolsExclude, err := a.convertSubcategories(ctx, profile.String(), cvse.subcategories, cvse.subcategoriesExclude, prefetchedTopics)
		if err != nil {
			return nil, err
		}
		topicBoolsForAggregateSubcategories, topicBoolsByAggregateExclude, err := a.convertAggregateSubcategory(ctx, profile, cvse.aggregateSubcategories, cvse.aggregateSubcategoriesExclude, prefetchedTopics, prefetchedAggregateExpansions)
		if err != nil {
			return nil, err
		}
		for _, tb := range topicBoolsForSubcategories {
			subcategoryFilter, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{tb}, documentType, false)
			if err != nil {
				return nil, err
			}
			filters = append(filters, subcategoryFilter)
		}
		for _, aggSub := range cvse.aggregateSubcategories {
			topicBools := topicBoolsForAggregateSubcategories[aggSub.MomentName]
			var aggregateSubcategoryFilter any
			if len(topicBools) == 0 {
				aggregateSubcategoryFilter = createMatchNoneFilter(documentType)
			} else {
				aggregateSubcategoryFilter, err = a.filterFromTopicBools(ctx, topicBools, documentType, false)
				if err != nil {
					return nil, err
				}
			}
			filters = append(filters, aggregateSubcategoryFilter)
		}
		for _, tb := range topicBoolsExclude {
			fExclude, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{tb}, documentType, false)
			if err != nil {
				return nil, err
			}
			filters = append(filters, wrapBucketFilterWithExclude(nil, []any{fExclude}))
		}
		for _, aggSub := range cvse.aggregateSubcategoriesExclude {
			aggTopicBools := topicBoolsByAggregateExclude[aggSub.MomentName]
			var fExclude any
			if len(aggTopicBools) == 0 {
				fExclude = createMatchNoneFilter(documentType)
			} else {
				fExclude, err = a.filterFromTopicBools(ctx, aggTopicBools, documentType, false)
				if err != nil {
					return nil, err
				}
			}
			filters = append(filters, wrapBucketFilterWithExclude(nil, []any{fExclude}))
		}
	case crestaEntityCluster:
		for _, c := range cvse.clusters {
			f, err := filterFromClusters(ctx, []*dashboardpb.ClusterForChart{c}, documentType, prefetchedV3ClusterClosures)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case crestaEntityOperaBlock:
		for _, ob := range cvse.operaBlocks {
			f, err := filterFromOperaBlocks(ctx, []*dashboardpb.OperaBlockForChart{ob}, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case crestaEntityBehavior:
		for _, b := range cvse.behaviors {
			f, err := filterFromBehaviors(ctx, []*dashboardpb.BehaviorForChart{b}, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case crestaEntityEmotion:
		for _, e := range cvse.emotions {
			f, err := filterFromEmotions(ctx, []*dashboardpb.EmotionForChart{e}, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case crestaEntitySentiment:
		for _, s := range cvse.sentiments {
			f, err := filterFromSentiments(ctx, []*dashboardpb.SentimentForChart{s}, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case attributeValueTypeScorecardTemplate:
		if bucketAggDocType.startType != elasticsearch.DocumentTypeConversation {
			return nil, fmt.Errorf("only support Conversation as ES query document type when grouping by scorecard templates, but found %v", bucketAggDocType.startType)
		}
		switch bucketAggDocType.endType {
		case elasticsearch.DocumentTypeConversation:
			for _, st := range cvse.scorecardTemplates {
				f, err := filterFromScorecardTemplates(ctx, []*dashboardpb.ScorecardTemplateForChart{st}, elasticsearch.DocumentTypeConversation)
				if err != nil {
					return nil, err
				}
				filters = append(filters, f)
			}
		case elasticsearch.DocumentTypeScorecard:
			var ff []any
			for _, st := range cvse.scorecardTemplates {
				scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(st.ScorecardTemplateName)
				if err != nil {
					return nil, err
				}
				ff = append(ff, elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						ScorecardTemplateIDs: &[]string{scorecardTemplateName.ScorecardTemplateID},
					},
				})
			}
			return &BucketAggregation{
				Nested: &NestedDocumentPath{
					Path: elasticsearch.ScorecardsPath,
				},
				Aggs: &ChartESAggregation{
					BucketAggs: &BucketAggregation{
						Filters: &FiltersAggregation{
							Filters: &ff,
						},
					},
				},
			}, nil
		}
	case attributeValuetypeScorecardCriterion:
		if bucketAggDocType.startType != elasticsearch.DocumentTypeConversation {
			return nil, fmt.Errorf("only support Conversation as ES query document type when grouping by scorecard criterions, but found %v", bucketAggDocType.startType)
		}
		switch bucketAggDocType.endType {
		case elasticsearch.DocumentTypeConversation:
			for _, crit := range cvse.scorecardCriterions {
				f, err := filterFromScorecardCriterions(ctx, []*dashboardpb.ScorecardCriterionForChart{crit}, elasticsearch.DocumentTypeConversation)
				if err != nil {
					return nil, err
				}
				filters = append(filters, f)
			}
		case elasticsearch.DocumentTypeScorecardScore:
			var ff []any
			for _, crit := range cvse.scorecardCriterions {
				ff = append(ff, elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						ScorecardScoresCriterionIDs: &[]string{crit.CriterionId},
					},
				})
			}
			return &BucketAggregation{
				Nested: &NestedDocumentPath{
					Path: elasticsearch.ScorecardsPath,
				},
				Aggs: &ChartESAggregation{
					BucketAggs: &BucketAggregation{
						Nested: &NestedDocumentPath{
							Path: elasticsearch.ScorecardScoresPath,
						},
						Aggs: &ChartESAggregation{
							BucketAggs: &BucketAggregation{
								Filters: &FiltersAggregation{
									Filters: &ff,
								},
							},
						},
					},
				},
			}, nil
		default:
			return nil, fmt.Errorf("unsupported ending document type for bucket aggregation %v", bucketAggDocType.endType)
		}
	case crestaEntityMetadata:
		for _, md := range cvse.metadatas {
			f, includeMissingValuesFilter, err := a.filterFromMetadatas(ctx, []*dashboardpb.MetadataForChart{md}, documentType, profileConfig)
			if err != nil {
				return nil, err
			}
			if f != nil {
				filters = append(filters, f)
			}
			if includeMissingValuesFilter != nil {
				filters = append(filters, includeMissingValuesFilter)
			}
		}
	case crestaEntityLlmAnnotation:
		for _, llmAnnotation := range cvse.llmAnnotations {
			f, includeMissingValuesFilter, err := a.filterFromLlmAnnotations(ctx, []*dashboardpb.LlmAnnotationForChart{llmAnnotation}, documentType)
			if err != nil {
				return nil, err
			}
			if f != nil {
				filters = append(filters, f)
			}
			if includeMissingValuesFilter != nil {
				filters = append(filters, includeMissingValuesFilter)
			}
		}
	case attributeValueTypeUsecaseName:
		for _, usecaseName := range cvse.usecaseNames {
			f, err := filterFromUsecaseNames(ctx, []string{usecaseName})
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case attributeValueTypeAgentName:
		for _, agentName := range cvse.agentNames {
			f, err := filterFromAgentNames(ctx, []string{agentName}, documentType, false /*=agentFilterAlwaysOnConversation*/)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case attributeValueTypeGroupName:
		for _, groupName := range cvse.groupNames {
			if memberIDs, ok := prefetchedGroupMemberIDs[groupName]; ok {
				filters = append(filters, filterFromAgentUserIDs(memberIDs, documentType, false /*=agentFilterAlwaysOnConversation*/))
				continue
			}
			// TODO(zhucaiyi): try to query agents for all groups in one call to avoid fanout to user service.
			f, err := a.filterFromGroupNames(ctx, profile.CustomerID, profile.ProfileID, []string{groupName}, documentType, false /*=agentFilterAlwaysOnConversation*/)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case attributeValueTypeConversationHandleTimeRange:
		for _, rangeValues := range cvse.conversationHandleTimeRanges {
			f, err := filterFromConversationHandleTimeRanges(ctx, rangeValues, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case attributeValueTypeConversationMessageCountRange:
		for _, rangeValues := range cvse.conversationMessageCountRanges {
			f, err := filterFromConversationMessageCountRanges(ctx, rangeValues, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	case attributeValueTypeConversationSource:
		hasPinnedClusters := hasPinnedClusters(prefetchedTopics)
		for _, s := range cvse.conversationSources {
			f := filterFromConversationSources([]int32{s}, documentType, hasPinnedClusters)
			filters = append(filters, f)
		}
	case attributeValueTypeConversationNumHoldsRange:
		for _, rangeValues := range cvse.conversationNumHoldsRanges {
			f, err := filterFromConversationNumHoldsRanges(ctx, rangeValues, documentType)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid value type %v in groupby", cvse.attributeValueType)
	}

	return &BucketAggregation{
		Filters: &FiltersAggregation{
			Filters: &filters,
		},
	}, nil
}

func extractGroupByAndFilterFromMetric(metric *dashboardpb.ChartMetric) (*dashboardpb.ChartGroupBy, *dashboardpb.ChartDataFilter, error) {
	var (
		groupByFromMetric *dashboardpb.ChartGroupBy
		filterFromMetric  *dashboardpb.ChartDataFilter
	)
	filterAndGroupByFromMetric := func(entityCount int, cvs *dashboardpb.ChartValueSet) {
		if entityCount == 1 {
			filterFromMetric = &dashboardpb.ChartDataFilter{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: cvs,
			}
		}
		if entityCount > 1 {
			groupByFromMetric = &dashboardpb.ChartGroupBy{
				GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
					ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
						ValueSet: cvs,
					},
				},
				Config: metric.GroupbyConfig,
			}
		}
	}
	switch metric.Metric.(type) {
	case *dashboardpb.ChartMetric_ForConversation_:
		// Intentionally left blank.
	case *dashboardpb.ChartMetric_ForMetadata_:
		entities := metric.GetForMetadata().Metadatas
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityMetadata,
			metadatas:          entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForLlmAnnotation_:
		entities := metric.GetForLlmAnnotation().GetLlmAnnotations()
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityLlmAnnotation,
			llmAnnotations:     entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForSubcategory_:
		entities := metric.GetForSubcategory().Subcategories
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntitySubcategory,
			subcategories:      entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForAggregateSubcategory_:
		entities := metric.GetForAggregateSubcategory().AggregateSubcategories
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType:     crestaEntityAggregateSubcategory,
			aggregateSubcategories: entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForOperaBlock_:
		entities := metric.GetForOperaBlock().OperaBlocks
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityOperaBlock,
			operaBlocks:        metric.GetForOperaBlock().OperaBlocks,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForBehavior_:
		entities := metric.GetForBehavior().Behaviors
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityBehavior,
			behaviors:          entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForSentiment_:
		entities := metric.GetForSentiment().Sentiments
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntitySentiment,
			sentiments:         entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForEmotion_:
		entities := metric.GetForEmotion().Emotions
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityEmotion,
			emotions:           entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForCluster_:
		entities := metric.GetForCluster().Clusters
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityCluster,
			clusters:           entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForAgentAssistance_:
		entities := metric.GetForAgentAssistance().AgentAssistances
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: crestaEntityAgentAssistance,
			agentAssistances:   entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForScorecardTemplate_:
		entities := metric.GetForScorecardTemplate().ScorecardTemplates
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType: attributeValueTypeScorecardTemplate,
			scorecardTemplates: entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForScorecardCriterion_:
		entities := metric.GetForScorecardCriterion().ScorecardCriterions
		chartValueSet := toChartValueSet(chartValueSetEquivalent{
			attributeValueType:  attributeValuetypeScorecardCriterion,
			scorecardCriterions: entities,
		})
		filterAndGroupByFromMetric(len(entities), chartValueSet)
	case *dashboardpb.ChartMetric_ForInfraMetric_:
		// No ES group-by or filter extraction needed; infra metrics are handled separately.
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "%v currently not implemented", metric.Metric)
	}
	return groupByFromMetric, filterFromMetric, nil
}

func convertGroupByTime(groupByTime *dashboardpb.ChartGroupBy_Time, timeRange *timeRange, metadata *analyticspb.Metadata) (elasticsearch.ReqDateHistogram, error) {
	var freq string
	switch groupByTime.Interval {
	case dashboardpb.ChartGroupBy_Time_DAY:
		freq = "day"
	case dashboardpb.ChartGroupBy_Time_WEEK:
		freq = "week" //nolint:goconst
	case dashboardpb.ChartGroupBy_Time_MONTH:
		freq = "month"
	case dashboardpb.ChartGroupBy_Time_HOUR:
		freq = "hour"
		if timeRange == nil || timeRange.from == nil || timeRange.to == nil {
			return elasticsearch.ReqDateHistogram{}, status.Errorf(codes.InvalidArgument, "time range is required when grouping by hour")
		}
		if timeRange.to.AsTime().Sub(timeRange.from.AsTime()) > 14*24*time.Hour {
			return elasticsearch.ReqDateHistogram{}, status.Errorf(codes.InvalidArgument, "time range must not exceed 14 days when grouping by hour")
		}
	default:
		return elasticsearch.ReqDateHistogram{}, status.Errorf(codes.InvalidArgument, "invalid time interval %v for group by time", groupByTime.Interval)
	}
	startDayOfWeekBias := shared.ParseStartDayOfWeek(metadata, freq == "week")
	ret := elasticsearch.ReqDateHistogram{
		Field:            "conversation_started_at",
		CalendarInterval: freq,
		TimeZone:         "+00:00",
		Offset:           elasticsearch.ParseStartDayOfWeekBiasToOffset(startDayOfWeekBias),
		MinDocCount:      proto.Int32(0),
	}

	timezoneID := ""
	if metadata != nil {
		timezoneID = metadata.TimeZoneId
	}

	if timezoneID != "" {
		timezoneOffset, err := shared.ConvertTimezoneToOffset(timezoneID)
		if err != nil {
			return elasticsearch.ReqDateHistogram{}, err
		}
		ret.TimeZone = timezoneOffset
	} else if timeRange != nil {
		// Currently FE pass 2024-12-01T05:00:00.000Z when the local timezone is UTC−05:00
		// When FE pass 2024-12-01T22:15:00.000Z, the local timezone is UTC+01:45.
		// To ensure the time range is 00:00 to 00:00 of local time. So when we see 05:00 in a timestamp,
		// The computed hourOffset should be -5, not 5.
		hourOffset, minuteOffset := timeRange.from.AsTime().Hour(), timeRange.from.AsTime().Minute()
		if hourOffset < 12 {

			hourOffset = 0 - hourOffset
			ret.TimeZone = shared.TimezoneOffsetString(hourOffset, minuteOffset)
		} else {
			hourOffset = 24 - hourOffset
			if minuteOffset != 0 {
				hourOffset -= 1
				minuteOffset = 60 - minuteOffset
			}
			ret.TimeZone = shared.TimezoneOffsetString(hourOffset, minuteOffset)
		}
	}

	if timeRange != nil {
		ret.ExtendedBound = &elasticsearch.ExtendedBoundForTime{}
		if t := timeRange.from; t != nil {
			ret.ExtendedBound.Min = t.AsTime().Format(time.RFC3339)
		}
		if t := timeRange.to; t != nil {
			ret.ExtendedBound.Max = t.AsTime().Format(time.RFC3339)
		}
	}
	return ret, nil
}
