package analyticsimpl

import (
	"context"

	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	enableReadFromTypedPercentageScoreField = envflag.String("ENABLE_READ_FROM_TYPED_PERCENTAGE_SCORE_FIELD", "false", "Whether to read from the new typed percentage score field `percentage_score2`.")
)

const (
	esFieldPathConversationHandleTime   = "handle_time_secs"
	esFieldPathConversationDuration     = "conversation_duration_secs"
	esFieldPathConversationMessageCount = "message_count"
	esFieldPathConversationNumHolds     = "num_holds"
)

type statsType struct {
	isConvoCount bool
}

// Calculate the ES metrics aggregation based on ChartMetrics setup.
// Also calculate target document type to be aggregated: either message (including message_group) or conversation.
// For target document type:
//   - stats_type is occurrence count or aggregation on occurrence_count_per_convo:
//   - when annotation of occurrence is on message, target_doc_type=message, subcategory, operablock, behavior, AA etc.
//   - when annotation of occurrence is on document, target_doc_type=conversation, eg metadata, convo sentiment
//
// - stats_type is conversation_count
//   - target_doc_type can be both message or conversation because conversation_id exists in both message and convo doc.
//     we can simply set to conversation
//
// - stats_type is aggregation on conversation attributes (convo_handle_time, duration etc)
//   - target_doc_type=conversation, since these attributes only exists on conversation documents.
func (a AnalyticsServiceImpl) convertChartMetricToESMetric(ctx context.Context, customerProfile *shared.CustomerProfile, profileConfig *config.ProfileConfig, chartMetric *dashboardpb.ChartMetric, groupbys []*dashboardpb.ChartGroupBy, precisionThresholdForCardAgg int, prefetchedTopics map[string]elasticsearch.TopicDetails, prefetchedAggregateExpansions map[string][]string, prefetchedV3ClusterClosures map[string][]string) (*ChartESAggregation, elasticsearch.DocumentType, elasticsearch.DocumentType, *statsType, error) {
	// TODO(zhucaiyi): add validation to ensure error is returned when stats_type=occurrence_count of "multiple entities"
	// And sorting is on the other groupby dimension.
	profileName := customerProfile.String()
	hasGroupbyUniqueValue := false
	for _, gb := range groupbys {
		if gb.GetByUniqueValue() != nil {
			hasGroupbyUniqueValue = true
			break
		}
	}

	childrenAgg := func(metricsAgg *ChartESAggregation, filter any, childDocType elasticsearch.DocumentType) error {
		metricsAgg.BucketAggs = &BucketAggregation{
			Children: &ChildrenAggregation{
				Type: string(childDocType),
			},
			Aggs: &ChartESAggregation{
				BucketAggs: &BucketAggregation{
					Filter: filter,
					Aggs: &ChartESAggregation{
						isTotalOccurrenceCount:          true,
						MetricsConversationMessageCount: elasticsearch.CreateReqTotalMessageCount(precisionThresholdForCardAgg),
					},
				},
			},
		}
		return nil
	}

	metricsAgg := &ChartESAggregation{}
	// The context of ES query, can be either Conversation or Message.
	// It is also the initial context of ES aggregation.
	esQueryDocType := elasticsearch.DocumentTypeUndefined
	// The context of ES metrics.
	// This is also the ending context of ES bucket aggregation.
	esMetricsAggDocType := elasticsearch.DocumentTypeUndefined

	retStatsType := &statsType{}
	switch chartMetric.Metric.(type) {
	case *dashboardpb.ChartMetric_ForConversation_:
		esQueryDocType = elasticsearch.DocumentTypeConversation
		switch statsType := chartMetric.GetForConversation().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForConversation_CommonStatsType:
			if statsType.GetCommonStatsType() != dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT {
				return nil, esQueryDocType, esMetricsAggDocType, nil, status.Errorf(codes.Unimplemented, "within CommonStatsType, only support CONVERSATION_COUNT, %d not supported", statsType.GetCommonStatsType())
			}
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			retStatsType.isConvoCount = true
		case *dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_:
			numericFieldPath := ""
			aggOnConvoAttribute := statsType.GetAggregationOnConversationAttribute()
			switch aggOnConvoAttribute.AggregationAttribute {
			case dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_HANDLE_TIME:
				numericFieldPath = esFieldPathConversationHandleTime
			case dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_DURATION:
				numericFieldPath = esFieldPathConversationDuration
			case dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_MESSAGE_COUNT:
				numericFieldPath = esFieldPathConversationMessageCount
			case dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_NUM_HOLDS:
				numericFieldPath = esFieldPathConversationNumHolds
			default:
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "aggregation on %v not supported yet", aggOnConvoAttribute.AggregationAttribute)
			}
			numericField := &elasticsearch.NumericField{
				Field: proto.String(numericFieldPath),
			}
			metricsAggsOnConvoAttribute := &elasticsearch.AggregationOnNumericField{}
			switch aggOnConvoAttribute.AggregationOperator {
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG:
				metricsAggsOnConvoAttribute.Avg = numericField
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MAX:
				metricsAggsOnConvoAttribute.Max = numericField
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MIN:
				metricsAggsOnConvoAttribute.Min = numericField
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_SUM:
				metricsAggsOnConvoAttribute.Sum = numericField
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P25:
				metricsAggsOnConvoAttribute.Percentiles = &elasticsearch.PercentilesMetric{
					Field:    numericFieldPath,
					Percents: []float64{25},
				}
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P50:
				metricsAggsOnConvoAttribute.Percentiles = &elasticsearch.PercentilesMetric{
					Field:    numericFieldPath,
					Percents: []float64{50},
				}
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P75:
				metricsAggsOnConvoAttribute.Percentiles = &elasticsearch.PercentilesMetric{
					Field:    numericFieldPath,
					Percents: []float64{75},
				}
			case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_BOX_PLOT_PERCENTILES:
				metricsAggsOnConvoAttribute.Percentiles = &elasticsearch.PercentilesMetric{
					Field:    numericFieldPath,
					Percents: []float64{0, 25, 50, 75, 100},
				}
			default:
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "aggregation operator %v not supported yet", aggOnConvoAttribute.AggregationOperator)
			}
			metricsAgg.MetricsAggOnNumericField = metricsAggsOnConvoAttribute
		}
	case *dashboardpb.ChartMetric_ForSubcategory_:
		switch statsType := chartMetric.GetForSubcategory().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForSubcategory_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				// TODO: document_type should also include message_group.
				if !hasGroupbyUniqueValue {
					esQueryDocType = elasticsearch.DocumentTypeMessage
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					esQueryDocType = elasticsearch.DocumentTypeConversation
					subcategories := chartMetric.GetForSubcategory().Subcategories
					if len(subcategories) > 1 {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
					}
					topicBools, _, err := a.convertSubcategories(ctx, profileName, subcategories, nil, prefetchedTopics)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					f, err := a.filterFromTopicBools(ctx, topicBools, elasticsearch.DocumentTypeMessage, false)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					if err := childrenAgg(metricsAgg, f, elasticsearch.DocumentTypeMessage); err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}

				}
			}
		case *dashboardpb.StatsTypeForSubcategory_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForAggregateSubcategory_:
		esQueryDocType = elasticsearch.DocumentTypeConversation
		forAggregateSubcategory := chartMetric.GetForAggregateSubcategory()
		switch forAggregateSubcategory.GetStatsType().GetStatsType().(type) {
		case *dashboardpb.StatsTypeForAggregateSubcategory_CommonStatsType:
			switch forAggregateSubcategory.GetStatsType().GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				// TODO: document_type should also include message_group.
				if !hasGroupbyUniqueValue {
					esQueryDocType = elasticsearch.DocumentTypeMessage
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					aggregateSubcategories := chartMetric.GetForAggregateSubcategory().AggregateSubcategories
					if len(aggregateSubcategories) > 1 {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
					}
					topicBoolsByAggregate, _, err := a.convertAggregateSubcategory(ctx, *customerProfile, aggregateSubcategories, nil, prefetchedTopics, prefetchedAggregateExpansions)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					for _, aggregateSubcategory := range aggregateSubcategories {
						topicBools := topicBoolsByAggregate[aggregateSubcategory.MomentName]
						// Aggregate subcategories with no children should match nothing
						var f any
						if len(topicBools) == 0 {
							// Use match_none query to explicitly match no documents
							f = createMatchNoneFilter(elasticsearch.DocumentTypeMessage)
						} else {
							f, err = a.filterFromTopicBools(ctx, topicBools, elasticsearch.DocumentTypeMessage, false)
							if err != nil {
								return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
							}
						}
						if err := childrenAgg(metricsAgg, f, elasticsearch.DocumentTypeMessage); err != nil {
							return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
						}
					}
				}
			}
		case *dashboardpb.StatsTypeForAggregateSubcategory_AggregationOnOccurrenceCountPerConversation:
			if op := forAggregateSubcategory.GetStatsType().GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForOperaBlock_:
		switch statsType := chartMetric.GetForOperaBlock().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForOperaBlock_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				if !hasGroupbyUniqueValue {
					esQueryDocType = elasticsearch.DocumentTypeMessage
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					esQueryDocType = elasticsearch.DocumentTypeConversation
					operaBlocks := chartMetric.GetForOperaBlock().OperaBlocks
					if len(operaBlocks) > 1 {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
					}
					f, err := filterFromOperaBlocks(ctx, operaBlocks, elasticsearch.DocumentTypeMessage)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					if err := childrenAgg(metricsAgg, f, elasticsearch.DocumentTypeMessage); err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
				}
			}
		case *dashboardpb.StatsTypeForOperaBlock_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForAgentAssistance_:
		switch statsType := chartMetric.GetForAgentAssistance().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForAgentAssistance_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeMessage
				metricsAgg.isTotalOccurrenceCount = true
			}
		case *dashboardpb.StatsTypeForAgentAssistance_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForBehavior_:
		switch statsType := chartMetric.GetForBehavior().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForBehavior_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				if !hasGroupbyUniqueValue {
					esQueryDocType = elasticsearch.DocumentTypeMessage
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					esQueryDocType = elasticsearch.DocumentTypeConversation
					behaviors := chartMetric.GetForBehavior().Behaviors
					if len(behaviors) > 1 {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
					}
					f, err := filterFromBehaviors(ctx, behaviors, elasticsearch.DocumentTypeMessage)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					if err := childrenAgg(metricsAgg, f, elasticsearch.DocumentTypeMessage); err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
				}
			}
		case *dashboardpb.StatsTypeForBehavior_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForEmotion_:
		switch statsType := chartMetric.GetForEmotion().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForEmotion_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				if !hasGroupbyUniqueValue {
					esQueryDocType = elasticsearch.DocumentTypeMessage
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					esQueryDocType = elasticsearch.DocumentTypeConversation
					emotions := chartMetric.GetForEmotion().Emotions
					if len(emotions) > 1 {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
					}
					f, err := filterFromEmotions(ctx, emotions, elasticsearch.DocumentTypeMessage)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					if err := childrenAgg(metricsAgg, f, elasticsearch.DocumentTypeMessage); err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
				}
			}
		case *dashboardpb.StatsTypeForEmotion_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForCluster_:
		switch statsType := chartMetric.GetForCluster().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForCluster_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				if !hasGroupbyUniqueValue {
					esQueryDocType = elasticsearch.DocumentTypeMessage
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					esQueryDocType = elasticsearch.DocumentTypeConversation
					clusters := chartMetric.GetForCluster().Clusters
					if len(clusters) > 1 {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
					}
					// TDv1/v2 cluster annotations live on the "message_group" child document, whereas
					// Topic Discovery V3 category annotations (ClusterForChart.detailed_type ==
					// CONVERSATION_REASON_V3) are message-anchored on the "message" child document. The
					// occurrence-count children aggregation must drill into that same child document type.
					clusterChildType, err := tdClusterDocumentType(clusters)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					nested, err := nestedFromClusters(ctx, clusters, prefetchedV3ClusterClosures)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					childFilter := elasticsearch.MsgFilter{Nested: nested}
					if err := childrenAgg(metricsAgg, childFilter, clusterChildType); err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
				}
			}
		case *dashboardpb.StatsTypeForCluster_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForSentiment_:
		sentimentType := chartMetric.GetForSentiment().Sentiments[0].Type
		switch statsType := chartMetric.GetForSentiment().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForSentiment_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				esQueryDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				if sentimentType == dashboardpb.SentimentForChart_CONVERSATION {
					esQueryDocType = elasticsearch.DocumentTypeConversation
					metricsAgg.isTotalOccurrenceCount = true
				} else {
					if !hasGroupbyUniqueValue {
						esQueryDocType = elasticsearch.DocumentTypeMessage
						metricsAgg.isTotalOccurrenceCount = true
					} else {
						esQueryDocType = elasticsearch.DocumentTypeConversation
						msgSentiments := chartMetric.GetForSentiment().Sentiments
						if len(msgSentiments) > 1 {
							return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "does not support occurrence count of multiple entities with groupby unique values")
						}
						f, err := filterFromSentiments(ctx, msgSentiments, elasticsearch.DocumentTypeMessage)
						if err != nil {
							return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
						}
						if err := childrenAgg(metricsAgg, f, elasticsearch.DocumentTypeMessage); err != nil {
							return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
						}
					}
				}
			}
		case *dashboardpb.StatsTypeForSentiment_AggregationOnOccurrenceCountPerConversation:
			if sentimentType == dashboardpb.SentimentForChart_CONVERSATION {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation not supported for CONVERSATION sentiment")
			}
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only supports operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeMessage
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		}
	case *dashboardpb.ChartMetric_ForMetadata_:
		esQueryDocType = elasticsearch.DocumentTypeConversation
		switch statsType := chartMetric.GetForMetadata().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForMetadata_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				metricsAgg.isTotalOccurrenceCount = true
			}
		case *dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_:
			metadatas := chartMetric.GetForMetadata().Metadatas
			if len(metadatas) != 1 {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "expect exactly one metadata in Metric")
			}
			momentName, err := momentpb.ParseMomentName(metadatas[0].MomentName)
			if err != nil {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
			}
			aggOnMetadataValue := statsType.GetAggregationOnMetadataValue()
			if aggOnMetadataValue.AggregationAttribute != dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_AGGREGATION_ATTRIBUTE_UNSPECIFIED &&
				aggOnMetadataValue.AggregationAttribute != dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "only support aggregation on number_value for metadata")
			}
			pathOfNumericField := elasticsearch.ConversationMetadataPayloadNumberValueField
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				pathOfNumericField = elasticsearch.ConversationMetadataPayloadNumberValueFieldExplicitMapping
			}
			isCrestaCLO := isCrestaModelledCLO(metadatas[0])
			if isCrestaCLO {
				if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
					pathOfNumericField = elasticsearch.ConversationOutcomePayloadNumberValueFieldExplicitMapping
				} else {
					pathOfNumericField = elasticsearch.ConversationOutcomePayloadNumberValueField
				}
			}
			aggOnNumericField, err := getESAggOnNumericField(pathOfNumericField, aggOnMetadataValue.AggregationOperator)
			if err != nil {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
			}
			// Only Cresta-modelled outcomes carry the predictor's unscored sentinel, so only
			// they are restricted to the moment's declared value bounds. INSI-4673.
			var numberValueRange *elasticsearch.RangeFilter
			if isCrestaCLO {
				numberValueRange = a.cloOutcomeNumberValueRangeFilterForChart(ctx, metadatas[0].MomentName, pathOfNumericField)
			}
			metricValueFilter := &elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					MomentTemplateIDs: &[]string{momentName.MomentID},
				},
			}
			if numberValueRange != nil {
				metricValueFilter = &elasticsearch.NestedFilter{
					Bool: &elasticsearch.NestedFilterBool{
						Filter: &[]elasticsearch.NestedFilter{
							*metricValueFilter,
							{Range: numberValueRange},
						},
					},
				}
			}

			metricsAgg.BucketAggs = &BucketAggregation{
				Nested: &NestedDocumentPath{
					Path: elasticsearch.MomentAnnotationsPath,
				},
				Aggs: &ChartESAggregation{
					BucketAggs: &BucketAggregation{
						Filter: metricValueFilter,
						Aggs: &ChartESAggregation{
							MetricsAggOnNumericField: aggOnNumericField,
						},
					},
				},
			}
		default:
			return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "metadata stats type %T not supported yet", statsType.StatsType)

		}
	case *dashboardpb.ChartMetric_ForLlmAnnotation_:
		esQueryDocType = elasticsearch.DocumentTypeConversation
		switch statsType := chartMetric.GetForLlmAnnotation().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForLlmAnnotation_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				metricsAgg.isTotalOccurrenceCount = true
			}
		case *dashboardpb.StatsTypeForLlmAnnotation_AggregationOnOccurrenceCountPerConversation:
			if op := statsType.GetAggregationOnOccurrenceCountPerConversation().GetAggregationOperator(); op != dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "AggregationOnOccurrenceCountPerConversation only support operator AVG, but found %v", op)
			}
			esQueryDocType = elasticsearch.DocumentTypeConversation
			metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			metricsAgg.isAverageOccurrenceCountPerConvo = true
		case *dashboardpb.StatsTypeForLlmAnnotation_AggregationOnValue:
			llmAnnotations := chartMetric.GetForLlmAnnotation().LlmAnnotations
			if len(llmAnnotations) != 1 {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "expect exactly one llm annotation in Metric")
			}
			momentName, err := momentpb.ParseMomentName(llmAnnotations[0].MomentName)
			if err != nil {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
			}
			aggOnValue := statsType.GetAggregationOnValue()
			if aggOnValue.GetAggregationAttribute() != dashboardpb.StatsTypeForLlmAnnotation_AggregationOnAnnotationValue_AGGREGATION_ATTRIBUTE_UNSPECIFIED &&
				aggOnValue.GetAggregationAttribute() != dashboardpb.StatsTypeForLlmAnnotation_AggregationOnAnnotationValue_NUMBER_VALUE {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "only support aggregation on number_value for llm annotation")
			}
			pathOfNumericField := elasticsearch.LlmAnnotationPayloadNumberValueField
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				pathOfNumericField = elasticsearch.LlmAnnotationPayloadNumberValueFieldExplicitMapping
			}
			aggOnNumericField, err := getESAggOnNumericField(pathOfNumericField, aggOnValue.GetAggregationOperator())
			if err != nil {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
			}

			metricsAgg.BucketAggs = &BucketAggregation{
				Nested: &NestedDocumentPath{
					Path: elasticsearch.MomentAnnotationsPath,
				},
				Aggs: &ChartESAggregation{
					BucketAggs: &BucketAggregation{
						Filter: &elasticsearch.NestedFilter{
							Terms: &elasticsearch.NestedTermsFilter{
								MomentTemplateIDs: &[]string{momentName.MomentID},
							},
						},
						Aggs: &ChartESAggregation{
							MetricsAggOnNumericField: aggOnNumericField,
						},
					},
				},
			}
		default:
			return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.Unimplemented, "llm annotation stats type %T not supported yet", statsType.StatsType)

		}
	case *dashboardpb.ChartMetric_ForScorecardTemplate_:
		esQueryDocType, esMetricsAggDocType = elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeConversation
		switch statsType := chartMetric.GetForScorecardTemplate().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForScorecardTemplate_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				metricsAgg.isTotalOccurrenceCount = true
			}
		case *dashboardpb.StatsTypeForScorecardTemplate_AggregationOnScore:
			scorecardTemplates := chartMetric.GetForScorecardTemplate().ScorecardTemplates
			if len(scorecardTemplates) == 0 {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "found 0 scorecard templates in Metric")
			}

			var percentageValueField string
			if isReadFromPercentageScore2Enabled, _ := a.isReadFromPercentageScore2FieldEnabled(profileConfig); isReadFromPercentageScore2Enabled {
				percentageValueField = elasticsearch.ScorecardPercentageValue2Field
			} else {
				percentageValueField = elasticsearch.ScorecardPercentageValueField
			}
			aggOnNumericField, err := getESAggOnNumericField(percentageValueField, statsType.GetAggregationOnScore())
			if err != nil {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
			}
			omitNullMetricValue := statsType.GetAggregationOnScore() == dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG
			if len(scorecardTemplates) == 1 {
				scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(scorecardTemplates[0].ScorecardTemplateName)
				if err != nil {
					return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
				}
				metricsAgg.BucketAggs = &BucketAggregation{
					Nested: &NestedDocumentPath{
						Path: elasticsearch.ScorecardsPath,
					},
					Aggs: &ChartESAggregation{
						BucketAggs: &BucketAggregation{
							Filter: &elasticsearch.NestedFilter{
								Terms: &elasticsearch.NestedTermsFilter{
									ScorecardTemplateIDs: &[]string{scorecardTemplateName.ScorecardTemplateID},
								},
							},
							Aggs: &ChartESAggregation{
								MetricsAggOnNumericField: aggOnNumericField,
								omitNullMetricValue:      omitNullMetricValue,
							},
						},
					},
				}
			} else {
				// For multiple templates, bucket agg ending context is Scorecard.
				esMetricsAggDocType = elasticsearch.DocumentTypeScorecard
				metricsAgg.MetricsAggOnNumericField = aggOnNumericField
				metricsAgg.omitNullMetricValue = omitNullMetricValue

				// The per-template output metric (a "filters" agg, one bucket per template) has no single
				// value to order agents by, so ordering used to fall back to doc_count. Instead, order by a
				// single aggregate over every selected template: nested scorecards -> filter(all template
				// ids) -> agg on percentage_score. See orderByMetricAggFromChartMetricsAgg.
				allTemplateIDs := make([]string, 0, len(scorecardTemplates))
				for _, scorecardTemplate := range scorecardTemplates {
					scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(scorecardTemplate.ScorecardTemplateName)
					if err != nil {
						return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
					}
					allTemplateIDs = append(allTemplateIDs, scorecardTemplateName.ScorecardTemplateID)
				}
				metricsAgg.orderByBucketAggsOverride = &BucketAggregation{
					Nested: &NestedDocumentPath{
						Path: elasticsearch.ScorecardsPath,
					},
					Aggs: &ChartESAggregation{
						BucketAggs: &BucketAggregation{
							Filter: &elasticsearch.NestedFilter{
								Terms: &elasticsearch.NestedTermsFilter{
									ScorecardTemplateIDs: &allTemplateIDs,
								},
							},
							Aggs: &ChartESAggregation{
								MetricsAggOnNumericField: aggOnNumericField,
								omitNullMetricValue:      omitNullMetricValue,
							},
						},
					},
				}
			}

		}
	case *dashboardpb.ChartMetric_ForScorecardCriterion_:
		esQueryDocType, esMetricsAggDocType = elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeConversation
		switch statsType := chartMetric.GetForScorecardCriterion().StatsType; statsType.StatsType.(type) {
		case *dashboardpb.StatsTypeForScorecardCriterion_CommonStatsType:
			switch statsType.GetCommonStatsType() {
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT:
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
				retStatsType.isConvoCount = true
			case dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT:
				// Assuming every convo has at most 1 occurrence
				metricsAgg.MetricsConversationCount = elasticsearch.CreateReqTotalConversationCount(precisionThresholdForCardAgg)
			}
		case *dashboardpb.StatsTypeForScorecardCriterion_AggregationOnScore:
			scorecardCriterions := chartMetric.GetForScorecardCriterion().ScorecardCriterions
			if len(scorecardCriterions) == 0 {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, status.Errorf(codes.InvalidArgument, "found 0 scorecard criterions in Metric")
			}

			aggOnNumericField, err := getESAggOnNumericField(elasticsearch.ScorecardScoresPercentageValueField, statsType.GetAggregationOnScore())
			if err != nil {
				return nil, elasticsearch.DocumentTypeUndefined, elasticsearch.DocumentTypeUndefined, nil, err
			}
			omitNullMetricValue := statsType.GetAggregationOnScore() == dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG
			if len(scorecardCriterions) == 1 {
				// Bucket agg ending context is conversation.
				esMetricsAggDocType = elasticsearch.DocumentTypeConversation
				metricsAgg.BucketAggs = &BucketAggregation{
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
									Filter: &elasticsearch.NestedFilter{
										Terms: &elasticsearch.NestedTermsFilter{
											ScorecardScoresCriterionIDs: &[]string{scorecardCriterions[0].CriterionId},
										},
									},
									Aggs: &ChartESAggregation{
										MetricsAggOnNumericField: aggOnNumericField,
										omitNullMetricValue:      omitNullMetricValue,
									},
								},
							},
						},
					},
				}
			} else {
				// For multiple criterion, bucket agg ending context is ScorecardScore.
				esMetricsAggDocType = elasticsearch.DocumentTypeScorecardScore
				metricsAgg.MetricsAggOnNumericField = aggOnNumericField
				metricsAgg.omitNullMetricValue = omitNullMetricValue

				// The per-criterion output metric (a "filters" agg, one bucket per criterion) has no single
				// value to order agents by, so ordering used to fall back to doc_count. Instead, order by a
				// single aggregate over every selected criterion: nested scorecards -> nested scorecard_scores
				// -> filter(all criterion ids) -> agg on percentage_score. See orderByMetricAggFromChartMetricsAgg.
				allCriterionIDs := make([]string, 0, len(scorecardCriterions))
				for _, scorecardCriterion := range scorecardCriterions {
					allCriterionIDs = append(allCriterionIDs, scorecardCriterion.CriterionId)
				}
				metricsAgg.orderByBucketAggsOverride = &BucketAggregation{
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
									Filter: &elasticsearch.NestedFilter{
										Terms: &elasticsearch.NestedTermsFilter{
											ScorecardScoresCriterionIDs: &allCriterionIDs,
										},
									},
									Aggs: &ChartESAggregation{
										MetricsAggOnNumericField: aggOnNumericField,
										omitNullMetricValue:      omitNullMetricValue,
									},
								},
							},
						},
					},
				}
			}
		}
	}

	// By default, metrics context is the same as query context if not explicitly specified.
	if esMetricsAggDocType == elasticsearch.DocumentTypeUndefined {
		esMetricsAggDocType = esQueryDocType
	}

	// Corner case.
	if metricsAgg.isTotalOccurrenceCount && esQueryDocType == elasticsearch.DocumentTypeMessage && len(groupbys) == 0 {
		metricsAgg.MetricsConversationMessageCount = elasticsearch.CreateReqTotalMessageCount(precisionThresholdForCardAgg)
	}
	return metricsAgg, esQueryDocType, esMetricsAggDocType, retStatsType, nil
}

// useConversationCountViaDocCount switches conversation-count metrics to built-in doc_count
// when aggregating on conversation documents. Each conversation document represents one
// conversation, so cardinality on conversation_id is unnecessary and memory-intensive.
func useConversationCountViaDocCount(metricsAgg *ChartESAggregation, statsType *statsType, metricsAggDocType elasticsearch.DocumentType, chartMetric *dashboardpb.ChartMetric, groupBys []*dashboardpb.ChartGroupBy) {
	if statsType == nil || !statsType.isConvoCount || metricsAgg == nil {
		return
	}
	if len(groupBys) == 0 {
		return
	}
	if chartMetric != nil && chartMetric.GroupbyConfig != nil {
		return
	}
	if metricsAggDocType != elasticsearch.DocumentTypeConversation ||
		metricsAgg.isAverageOccurrenceCountPerConvo ||
		metricsAgg.BucketAggs != nil {
		return
	}
	metricsAgg.isConversationCountViaDocCount = true
	metricsAgg.MetricsConversationCount = nil
}

// cloOutcomeNumberValueRangeFilterForChart resolves the Cresta-modelled outcome moment behind
// a chart metadata metric and maps its declared value bounds onto the ES range filter for the
// numberValue field the metric aggregates, so the predictor's unscored sentinel (e.g. the CSAT
// -1) stays out of chart averages. Returns nil when the moment declares no bounds — the
// aggregation then stays unfiltered, preserving legitimately negative scales. INSI-4673.
func (a AnalyticsServiceImpl) cloOutcomeNumberValueRangeFilterForChart(ctx context.Context, momentName string, numberValueField string) *elasticsearch.RangeFilter {
	getResp, err := a.momentClientRegistry.Client(ctx).GetMoment(ctx, &momentpb.GetMomentRequest{
		Name: momentName,
	})
	if err != nil {
		// fail-open: bounds are an accuracy enhancement, not a correctness gate — fall back to the legacy unfiltered metric instead of failing the chart (a deleted template surfaces as NotFound here)
		a.logger.Warnf(ctx, "failed to fetch outcome moment %q for chart numberValue bounds, leaving metric unfiltered: %v", momentName, err)
		return nil
	}
	if getResp.GetMoment() == nil {
		// fail-open: an unresolvable template matches no annotations anyway; keep the chart on the legacy unfiltered path
		a.logger.Warnf(ctx, "outcome moment %q not found for chart numberValue bounds, leaving metric unfiltered", momentName)
		return nil
	}
	return getOutcomeNumberValueRangeFilter(getResp.GetMoment(), numberValueField)
}

func getESAggOnNumericField(pathOfNumericField string, aggregationOperator dashboardpb.AggregationOperator) (*elasticsearch.AggregationOnNumericField, error) {
	esAggOnNumericField := &elasticsearch.AggregationOnNumericField{}
	numericField := &elasticsearch.NumericField{
		Field: proto.String(pathOfNumericField),
	}
	switch aggregationOperator {
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG:
		esAggOnNumericField.Avg = numericField
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MAX:
		esAggOnNumericField.Max = numericField
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MIN:
		esAggOnNumericField.Min = numericField
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_SUM:
		esAggOnNumericField.Sum = numericField
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P25:
		esAggOnNumericField.Percentiles = &elasticsearch.PercentilesMetric{
			Field:    *numericField.Field,
			Percents: []float64{25},
		}
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P50:
		esAggOnNumericField.Percentiles = &elasticsearch.PercentilesMetric{
			Field:    *numericField.Field,
			Percents: []float64{50},
		}
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P75:
		esAggOnNumericField.Percentiles = &elasticsearch.PercentilesMetric{
			Field:    *numericField.Field,
			Percents: []float64{75},
		}
	case dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_BOX_PLOT_PERCENTILES:
		esAggOnNumericField.Percentiles = &elasticsearch.PercentilesMetric{
			Field:    *numericField.Field,
			Percents: []float64{0, 25, 50, 75, 100},
		}
	default:
		return nil, status.Errorf(codes.Unimplemented, "aggregation operator %v not supported yet", aggregationOperator)
	}
	return esAggOnNumericField, nil
}

func (a AnalyticsServiceImpl) isReadFromPercentageScore2FieldEnabled(profileConfig *config.ProfileConfig) (bool, error) {
	if *enableReadFromTypedPercentageScoreField == TrueString {
		return true, nil
	}
	return profileConfig.GetFeatures().GetInsights().GetEnableReadFromTypedPercentageScoreField(), nil
}
