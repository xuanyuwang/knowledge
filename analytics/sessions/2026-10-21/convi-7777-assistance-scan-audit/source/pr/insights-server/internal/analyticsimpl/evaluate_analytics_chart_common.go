package analyticsimpl

import (
	"context"
	"slices"

	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/time"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/shared-go/container/set"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxMomentNamesInErrorLog = 10

type QueryEngineType int32

const (
	// Using Elasticsearch as query engine.
	QueryEngineES QueryEngineType = iota
	// Using ClickHouse as query engine.
	QueryEngineCH
	// Using Prometheus as query engine (for ForInfraMetric charts).
	QueryEngineInfraMetrics
)

type ChartESRequest struct {
	Query           *ChartESQuery                  `json:"query,omitempty"`
	Highlight       *elasticsearch.ReqHighlight    `json:"highlight,omitempty"`
	Aggs            *ChartESAggregation            `json:"aggs,omitempty"`
	SearchAfter     *[]string                      `json:"search_after,omitempty"`
	Sort            *[]elasticsearch.Sort          `json:"sort,omitempty"`
	Source          *elasticsearch.Source          `json:"_source,omitempty"`
	RuntimeMappings *elasticsearch.RuntimeMappings `json:"runtime_mappings,omitempty"`
}

type chartValueSetEquivalent struct {
	attributeValueType             attributeValueType
	subcategories                  []*dashboardpb.SubcategoryForChart
	subcategoriesExclude           []*dashboardpb.SubcategoryForChart
	aggregateSubcategories         []*dashboardpb.AggregateSubcategoryForChart
	aggregateSubcategoriesExclude  []*dashboardpb.AggregateSubcategoryForChart
	clusters                       []*dashboardpb.ClusterForChart
	operaBlocks                    []*dashboardpb.OperaBlockForChart
	metadatas                      []*dashboardpb.MetadataForChart
	llmAnnotations                 []*dashboardpb.LlmAnnotationForChart
	behaviors                      []*dashboardpb.BehaviorForChart
	agentAssistances               []*dashboardpb.AgentAssistanceForChart
	sentiments                     []*dashboardpb.SentimentForChart
	emotions                       []*dashboardpb.EmotionForChart
	scorecardTemplates             []*dashboardpb.ScorecardTemplateForChart
	scorecardCriterions            []*dashboardpb.ScorecardCriterionForChart
	conversationHandleTimeRanges   []*dashboardpb.ChartFloatRange
	conversationMessageCountRanges []*dashboardpb.ChartFloatRange
	conversationDurationRanges     []*dashboardpb.ChartDurationRange
	conversationNumHoldsRanges     []*dashboardpb.ChartFloatRange
	usecaseNames                   []string
	agentNames                     []string
	groupNames                     []string
	scorecardSubmitterNames        []string
	scorecardStatusFilters         []*dashboardpb.ScorecardStatusFilter
	advancedTimeFilters            []*commonpb.TimeRangeFilter
	conversationSources            []int32
	conversationChannels           []int32
	conversationLanguageCodes      []string
	keywordFilter                  *dashboardpb.KeywordFilterForChart
}

type attributeValueType int32

const (
	crestaEntitySubcategory attributeValueType = iota
	crestaEntityAggregateSubcategory
	crestaEntityAggregateAndLeafSubcategory
	crestaEntityCluster
	crestaEntityOperaBlock
	crestaEntityBehavior
	crestaEntityAgentAssistance
	crestaEntityMetadata
	crestaEntityLlmAnnotation
	crestaEntitySentiment
	crestaEntityEmotion
	attributeValueTypeUsecaseName
	attributeValueTypeAgentName
	attributeValueTypeGroupName
	attributeValueTypeScorecardTemplate
	attributeValuetypeScorecardCriterion
	attributeValueTypeConversationHandleTimeRange
	attributeValueTypeConversationMessageCountRange
	attributeValueTypeConversationDurationRange
	attributeValueTypeScorecardSubmitterName
	attributeValueTypeScorecardStatusFilter
	attributeValueTypeAdvancedTimeFilter
	attributeValueTypeConversationSource
	attributeValueTypeConversationNumHoldsRange
	attributeValueTypeConversationChannel
	attributeValueTypeConversationLanguageCode
	attributeValueTypeTranscriptKeywords
)

// This method determines whether the entity represented by the given ChartValueSet is at
// message level or conversation level or both.
func (a AnalyticsServiceImpl) isMessageOrConvoEntity(ctx context.Context, profileConfig *config.ProfileConfig, chartValueSet *dashboardpb.ChartValueSet) (isMessageLevel bool, isConversationLevel bool, err error) {
	cvse, err := fromChartValueSet(ctx, chartValueSet)
	if err != nil {
		return false, false, err
	}
	switch cvse.attributeValueType {
	// Message level entities.
	case crestaEntitySubcategory, crestaEntityAggregateSubcategory, crestaEntityAggregateAndLeafSubcategory, crestaEntityCluster, crestaEntityOperaBlock, crestaEntityBehavior, crestaEntityAgentAssistance, crestaEntityEmotion:
		isMessageLevel = true
	// Conversation level entities.
	case attributeValueTypeAgentName, attributeValueTypeGroupName, attributeValueTypeScorecardTemplate, attributeValuetypeScorecardCriterion, attributeValueTypeScorecardSubmitterName, attributeValueTypeScorecardStatusFilter, attributeValueTypeConversationSource, attributeValueTypeConversationNumHoldsRange, attributeValueTypeConversationChannel, attributeValueTypeConversationLanguageCode, attributeValueTypeTranscriptKeywords, crestaEntityLlmAnnotation:
		isConversationLevel = true
	// Both message level and conversation level entities.
	// Handle time and advanced time are unconditional (PR #30858): conversation_handle_time_secs has been
	// denormalized onto message docs long enough that all message docs carry it.
	case attributeValueTypeUsecaseName, attributeValueTypeConversationMessageCountRange, attributeValueTypeConversationHandleTimeRange, attributeValueTypeAdvancedTimeFilter:
		isConversationLevel = true
		isMessageLevel = true
	// Conversation duration is denormalized onto message docs (INSI-4329), so it can be read from either
	// document. Unlike handle_time, the denormalization is not yet globally backfilled, so the message-level
	// classification is gated behind a rollout flag enabled per-customer only after that customer's message
	// docs are backfilled with conversation_duration_secs. Default off preserves today's conversation-only
	// behavior (duration filter forces the conversation-doc + join path).
	case attributeValueTypeConversationDurationRange:
		isConversationLevel = true
		durationFlagOn, err := a.featureFlagClient.BoolValue(ctx, "enable_reading_denormalized_conversation_duration_from_message_doc", false)
		if err != nil {
			// fail-open: flag evaluation errors fall back to the default (off), preserving conversation-only behavior.
			a.logger.Warnf(ctx, "feature flag fallback used for enable_reading_denormalized_conversation_duration_from_message_doc: %v", err)
		}
		if durationFlagOn {
			isMessageLevel = true
		}
	// Special handling needed for metadata due to denormalization.
	case crestaEntityMetadata:
		isConversationLevel = true
		// Also need to check if metadata is denormalized to message level.
		momentName, err := momentpb.ParseMomentName(cvse.metadatas[0].MomentName)
		if err != nil {
			return false, false, err
		}
		enableReadingDenormalizedConversationMetadataFromMessageDoc, denormalizedMetadataMomentTemplateIds := profileConfig.GetFeatures().GetConversation().GetEnableReadingDenormalizedConversationMetadataFromMessageDoc(), profileConfig.GetFeatures().GetConversation().GetDenormalizedConversationMetadataMomentTemplateIds()
		a.logger.Infof(ctx, "flag enableReadingDenormalizedConversationMetadataFromMessageDoc: %v, denormalizedMetadataMomentTemplateIds: %v", enableReadingDenormalizedConversationMetadataFromMessageDoc, denormalizedMetadataMomentTemplateIds)
		if enableReadingDenormalizedConversationMetadataFromMessageDoc &&
			slices.Contains(denormalizedMetadataMomentTemplateIds, momentName.MomentID) {
			isMessageLevel = true
		}
	// Sentiment can be either conversation level or message level.
	case crestaEntitySentiment:
		if cvse.sentiments[0].Type == dashboardpb.SentimentForChart_CONVERSATION {
			isConversationLevel = true
		} else {
			isMessageLevel = true
		}
	default:
		return false, false, status.Errorf(codes.InvalidArgument, "In method IsMessageOrConvoEntity, attribute value type %v not supported", cvse.attributeValueType)

	}
	return isMessageLevel, isConversationLevel, nil
}

// Returns the document type on which the occurrence of the Cresta entity is attached to.
// Eg for Subcategory, it is Message, for Metadata, it is Conversation.
// TODO(zhucaiyi): consolidate this method with isMessageOrConvoEntity in evaluate_analytics_chart_es_response.go since they have similar logic.
func occurrenceDocumentType(chartAttributeValue *dashboardpb.ChartAttributeValue) elasticsearch.DocumentType {
	ret := elasticsearch.DocumentTypeUndefined
	switch chartAttributeValue.Value.(type) {
	case *dashboardpb.ChartAttributeValue_AgentAssistanceValue,
		*dashboardpb.ChartAttributeValue_BehaviorValue,
		*dashboardpb.ChartAttributeValue_ClusterValue,
		*dashboardpb.ChartAttributeValue_EmotionValue,
		*dashboardpb.ChartAttributeValue_OperaBlockValue,
		*dashboardpb.ChartAttributeValue_SubcategoryValue,
		*dashboardpb.ChartAttributeValue_AggregateSubcategoryValue:
		ret = elasticsearch.DocumentTypeMessage
	case *dashboardpb.ChartAttributeValue_MetadataValue:
		ret = elasticsearch.DocumentTypeConversation
	case *dashboardpb.ChartAttributeValue_LlmAnnotation:
		ret = elasticsearch.DocumentTypeConversation
	case *dashboardpb.ChartAttributeValue_SentimentValue:
		if t := chartAttributeValue.GetSentimentValue().Type; t == dashboardpb.SentimentForChart_CONVERSATION {
			ret = elasticsearch.DocumentTypeConversation
		} else if t == dashboardpb.SentimentForChart_MESSAGE {
			ret = elasticsearch.DocumentTypeMessage
		}
	}
	return ret
}

func IsConversationCountForScorecardCriterionMetric(chartDataConfig *dashboardpb.ChartDataConfig) bool {
	chartMetric := chartDataConfig.GetMetrics()[0]
	if _, ok := chartMetric.Metric.(*dashboardpb.ChartMetric_ForScorecardCriterion_); ok {
		return chartMetric.GetForScorecardCriterion().GetStatsType().GetCommonStatsType() == dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT && !hasNotApplicableValue(chartMetric.GetForScorecardCriterion())
	}

	return false
}

func fromChartValueSet(ctx context.Context, chartValueSet *dashboardpb.ChartValueSet) (*chartValueSetEquivalent, error) {
	if chartValueSet == nil {
		return nil, status.Errorf(codes.InvalidArgument, "found empty chartValueSet")
	}
	ret := &chartValueSetEquivalent{}
	attributeValueTypes := map[attributeValueType]interface{}{}
	// isExclude returns true when the attribute value should be excluded from the query.
	isExclude := func(v *dashboardpb.ChartAttributeValue) bool {
		return v != nil && v.GetOperator() == dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE
	}
	switch chartValueSet.Values.(type) {
	case *dashboardpb.ChartValueSet_SingleValues:
		for _, val := range chartValueSet.GetSingleValues().AttributeValues {
			switch val.Value.(type) {
			case *dashboardpb.ChartAttributeValue_TimeRangeFilter:
				attributeValueTypes[attributeValueTypeAdvancedTimeFilter] = nil
				ret.advancedTimeFilters = append(ret.advancedTimeFilters, val.GetTimeRangeFilter())
			case *dashboardpb.ChartAttributeValue_AgentAssistanceValue:
				attributeValueTypes[crestaEntityAgentAssistance] = nil
				ret.agentAssistances = append(ret.agentAssistances, val.GetAgentAssistanceValue())
			case *dashboardpb.ChartAttributeValue_BehaviorValue:
				attributeValueTypes[crestaEntityBehavior] = nil
				ret.behaviors = append(ret.behaviors, val.GetBehaviorValue())
			case *dashboardpb.ChartAttributeValue_OperaBlockValue:
				attributeValueTypes[crestaEntityOperaBlock] = nil
				ret.operaBlocks = append(ret.operaBlocks, val.GetOperaBlockValue())
			case *dashboardpb.ChartAttributeValue_MetadataValue:
				attributeValueTypes[crestaEntityMetadata] = nil
				ret.metadatas = append(ret.metadatas, val.GetMetadataValue())
			case *dashboardpb.ChartAttributeValue_LlmAnnotation:
				attributeValueTypes[crestaEntityLlmAnnotation] = nil
				ret.llmAnnotations = append(ret.llmAnnotations, val.GetLlmAnnotation())
			case *dashboardpb.ChartAttributeValue_SubcategoryValue:
				attributeValueTypes[crestaEntitySubcategory] = nil
				if isExclude(val) {
					ret.subcategoriesExclude = append(ret.subcategoriesExclude, val.GetSubcategoryValue())
				} else {
					ret.subcategories = append(ret.subcategories, val.GetSubcategoryValue())
				}
			case *dashboardpb.ChartAttributeValue_AggregateSubcategoryValue:
				attributeValueTypes[crestaEntityAggregateSubcategory] = nil
				if isExclude(val) {
					ret.aggregateSubcategoriesExclude = append(ret.aggregateSubcategoriesExclude, val.GetAggregateSubcategoryValue())
				} else {
					ret.aggregateSubcategories = append(ret.aggregateSubcategories, val.GetAggregateSubcategoryValue())
				}
			case *dashboardpb.ChartAttributeValue_ClusterValue:
				attributeValueTypes[crestaEntityCluster] = nil
				ret.clusters = append(ret.clusters, val.GetClusterValue())
			case *dashboardpb.ChartAttributeValue_EmotionValue:
				attributeValueTypes[crestaEntityEmotion] = nil
				ret.emotions = append(ret.emotions, val.GetEmotionValue())
			case *dashboardpb.ChartAttributeValue_SentimentValue:
				attributeValueTypes[crestaEntitySentiment] = nil
				ret.sentiments = append(ret.sentiments, val.GetSentimentValue())
			case *dashboardpb.ChartAttributeValue_UsecaseNameValue:
				attributeValueTypes[attributeValueTypeUsecaseName] = nil
				ret.usecaseNames = append(ret.usecaseNames, val.GetUsecaseNameValue())
			case *dashboardpb.ChartAttributeValue_GroupNameValue:
				attributeValueTypes[attributeValueTypeGroupName] = nil
				ret.groupNames = append(ret.groupNames, val.GetGroupNameValue())
			case *dashboardpb.ChartAttributeValue_UserNameValue:
				attributeValueTypes[attributeValueTypeAgentName] = nil
				ret.agentNames = append(ret.agentNames, val.GetUserNameValue())
			case *dashboardpb.ChartAttributeValue_ScorecardTemplateValue:
				attributeValueTypes[attributeValueTypeScorecardTemplate] = nil
				ret.scorecardTemplates = append(ret.scorecardTemplates, val.GetScorecardTemplateValue())
			case *dashboardpb.ChartAttributeValue_ScorecardCriterionValue:
				attributeValueTypes[attributeValuetypeScorecardCriterion] = nil
				ret.scorecardCriterions = append(ret.scorecardCriterions, val.GetScorecardCriterionValue())
			case *dashboardpb.ChartAttributeValue_ConversationHandleTimeRange:
				attributeValueTypes[attributeValueTypeConversationHandleTimeRange] = nil
				ret.conversationHandleTimeRanges = append(ret.conversationHandleTimeRanges, val.GetConversationHandleTimeRange())
			case *dashboardpb.ChartAttributeValue_ConversationMessageCountRange:
				attributeValueTypes[attributeValueTypeConversationMessageCountRange] = nil
				ret.conversationMessageCountRanges = append(ret.conversationMessageCountRanges, val.GetConversationMessageCountRange())
			case *dashboardpb.ChartAttributeValue_ConversationDurationRange:
				attributeValueTypes[attributeValueTypeConversationDurationRange] = nil
				ret.conversationDurationRanges = append(ret.conversationDurationRanges, val.GetConversationDurationRange())
			case *dashboardpb.ChartAttributeValue_ScorecardSubmitterUserName:
				attributeValueTypes[attributeValueTypeScorecardSubmitterName] = nil
				ret.scorecardSubmitterNames = append(ret.scorecardSubmitterNames, val.GetScorecardSubmitterUserName())
			case *dashboardpb.ChartAttributeValue_ScorecardStatusFilter:
				attributeValueTypes[attributeValueTypeScorecardStatusFilter] = nil
				ret.scorecardStatusFilters = append(ret.scorecardStatusFilters, val.GetScorecardStatusFilter())
			case *dashboardpb.ChartAttributeValue_ConversationSource:
				attributeValueTypes[attributeValueTypeConversationSource] = nil
				ret.conversationSources = append(ret.conversationSources, int32(val.GetConversationSource()))
			case *dashboardpb.ChartAttributeValue_ConversationChannel:
				attributeValueTypes[attributeValueTypeConversationChannel] = nil
				ret.conversationChannels = append(ret.conversationChannels, int32(val.GetConversationChannel()))
			case *dashboardpb.ChartAttributeValue_ConversationLanguageCode:
				attributeValueTypes[attributeValueTypeConversationLanguageCode] = nil
				ret.conversationLanguageCodes = append(ret.conversationLanguageCodes, val.GetConversationLanguageCode())
			case *dashboardpb.ChartAttributeValue_ConversationNumHoldsRange:
				attributeValueTypes[attributeValueTypeConversationNumHoldsRange] = nil
				ret.conversationNumHoldsRanges = append(ret.conversationNumHoldsRanges, val.GetConversationNumHoldsRange())
			default:
				return nil, status.Errorf(codes.InvalidArgument, "%T not supported in ChartAttributeValue", val.Value)
			}
		}
	case *dashboardpb.ChartValueSet_RangeValues:
		return nil, status.Errorf(codes.InvalidArgument, "range_values not supported in ChartValueSet")
	case *dashboardpb.ChartValueSet_KeywordFilterValue:
		if err := validateChartKeywordFilter(chartValueSet.GetKeywordFilterValue()); err != nil {
			return nil, err
		}
		attributeValueTypes[attributeValueTypeTranscriptKeywords] = nil
		ret.keywordFilter = chartValueSet.GetKeywordFilterValue()
	default:
		return nil, status.Errorf(codes.InvalidArgument, "type %T not supported in ChartValueSet", chartValueSet.Values)
	}

	// Special case: if both subcategory and aggregate subcategory are present, treat as aggregate+leaf
	if len(attributeValueTypes) == 2 {
		_, hasSubcategory := attributeValueTypes[crestaEntitySubcategory]
		_, hasAggregateSubcategory := attributeValueTypes[crestaEntityAggregateSubcategory]
		if hasSubcategory && hasAggregateSubcategory {
			ret.attributeValueType = crestaEntityAggregateAndLeafSubcategory
			return ret, nil
		}
		return nil, status.Errorf(codes.InvalidArgument, "expecting exactly one value type in ChartValueSet, but found %v", attributeValueTypes)
	}

	if len(attributeValueTypes) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "expecting exactly one value type in ChartValueSet, but found %v", attributeValueTypes)
	}
	for attributeValueType := range attributeValueTypes {
		ret.attributeValueType = attributeValueType
		break
	}
	return ret, nil
}

func toChartValueSet(cvse chartValueSetEquivalent) *dashboardpb.ChartValueSet {
	values := []*dashboardpb.ChartAttributeValue{}
	switch cvse.attributeValueType {
	case attributeValueTypeTranscriptKeywords:
		return &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_KeywordFilterValue{KeywordFilterValue: cvse.keywordFilter},
		}
	case crestaEntityAgentAssistance:
		for _, val := range cvse.agentAssistances {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
					AgentAssistanceValue: val,
				},
			})
		}

	case crestaEntityOperaBlock:
		for _, val := range cvse.operaBlocks {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
					OperaBlockValue: val,
				},
			})
		}
	case crestaEntityMetadata:
		for _, val := range cvse.metadatas {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_MetadataValue{
					MetadataValue: val,
				},
			})
		}
	case crestaEntityLlmAnnotation:
		for _, llmAnnotation := range cvse.llmAnnotations {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
					LlmAnnotation: llmAnnotation,
				},
			})
		}
	case crestaEntityBehavior:
		for _, val := range cvse.behaviors {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
					BehaviorValue: val,
				},
			})
		}
	case crestaEntitySubcategory:
		for _, val := range cvse.subcategories {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
					SubcategoryValue: val,
				},
			})
		}
	case crestaEntityAggregateSubcategory:
		for _, val := range cvse.aggregateSubcategories {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
					AggregateSubcategoryValue: val,
				},
			})
		}
	case crestaEntityCluster:
		for _, val := range cvse.clusters {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ClusterValue{
					ClusterValue: val,
				},
			})
		}
	case crestaEntityEmotion:
		for _, val := range cvse.emotions {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_EmotionValue{
					EmotionValue: val,
				},
			})
		}
	case crestaEntitySentiment:
		for _, val := range cvse.sentiments {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_SentimentValue{
					SentimentValue: val,
				},
			})
		}
	case attributeValueTypeUsecaseName:
		for _, val := range cvse.usecaseNames {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
					UsecaseNameValue: val,
				},
			})
		}
	case attributeValueTypeAgentName:
		for _, val := range cvse.agentNames {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_UserNameValue{
					UserNameValue: val,
				},
			})
		}
	case attributeValueTypeGroupName:
		for _, val := range cvse.groupNames {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_GroupNameValue{
					GroupNameValue: val,
				},
			})
		}
	case attributeValueTypeScorecardSubmitterName:
		for _, val := range cvse.scorecardSubmitterNames {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ScorecardSubmitterUserName{
					ScorecardSubmitterUserName: val,
				},
			})
		}
	case attributeValueTypeScorecardStatusFilter:
		for _, val := range cvse.scorecardStatusFilters {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ScorecardStatusFilter{
					ScorecardStatusFilter: val,
				},
			})
		}
	case attributeValueTypeScorecardTemplate:
		for _, val := range cvse.scorecardTemplates {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
					ScorecardTemplateValue: val,
				},
			})
		}
	case attributeValuetypeScorecardCriterion:
		for _, val := range cvse.scorecardCriterions {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
					ScorecardCriterionValue: val,
				},
			})
		}
	case attributeValueTypeConversationHandleTimeRange:
		if len(cvse.conversationHandleTimeRanges) > 0 {
			for _, timeRange := range cvse.conversationHandleTimeRanges {
				values = append(values, &dashboardpb.ChartAttributeValue{
					Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
						ConversationHandleTimeRange: timeRange,
					},
				})
			}
		}
	case attributeValueTypeConversationMessageCountRange:
		if len(cvse.conversationMessageCountRanges) > 0 {
			for _, countRange := range cvse.conversationMessageCountRanges {
				values = append(values, &dashboardpb.ChartAttributeValue{
					Value: &dashboardpb.ChartAttributeValue_ConversationMessageCountRange{
						ConversationMessageCountRange: countRange,
					},
				})
			}
		}
	case attributeValueTypeConversationDurationRange:
		for _, val := range cvse.conversationDurationRanges {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ConversationDurationRange{
					ConversationDurationRange: val,
				},
			})
		}
	case attributeValueTypeConversationNumHoldsRange:
		for _, numHoldsRange := range cvse.conversationNumHoldsRanges {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
					ConversationNumHoldsRange: numHoldsRange,
				},
			})
		}
	case attributeValueTypeConversationSource:
		for _, val := range cvse.conversationSources {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ConversationSource{
					ConversationSource: convopb.Conversation_Source(val),
				},
			})
		}
	case attributeValueTypeConversationChannel:
		for _, val := range cvse.conversationChannels {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ConversationChannel{
					ConversationChannel: convopb.Conversation_Channel(val),
				},
			})
		}
	case attributeValueTypeConversationLanguageCode:
		for _, val := range cvse.conversationLanguageCodes {
			values = append(values, &dashboardpb.ChartAttributeValue{
				Value: &dashboardpb.ChartAttributeValue_ConversationLanguageCode{
					ConversationLanguageCode: val,
				},
			})
		}
	}

	return &dashboardpb.ChartValueSet{
		Values: &dashboardpb.ChartValueSet_SingleValues{
			SingleValues: &dashboardpb.ChartValueSet_Single{
				AttributeValues: values,
			},
		},
	}
}

// Determine the query engine for specified AnalyticsChart.
// Returns an error if metrics mix ForInfraMetric with other metric types.
func determineQueryEngine(chart *dashboardpb.AnalyticsChart) (QueryEngineType, error) {
	if chart != nil && chart.DataConfig != nil {
		if metrics := chart.DataConfig.Metrics; len(metrics) > 0 {
			infraCount := 0
			for _, m := range metrics {
				if m.GetForInfraMetric() != nil {
					infraCount++
				}
			}
			if infraCount > 0 && infraCount < len(metrics) {
				return 0, status.Errorf(codes.InvalidArgument, "chart mixes ForInfraMetric and non-infra metrics; all metrics must use ForInfraMetric")
			}
			if infraCount == len(metrics) {
				return QueryEngineInfraMetrics, nil
			}
			for _, m := range metrics {
				if m.GetForSubcategory() != nil {
					return QueryEngineES, nil
				}
			}
		}
		if filters := chart.DataConfig.Filters; len(filters) > 0 {
			for _, f := range filters {
				if f.ValueSet != nil {
					//nolint:gocritic
					switch f.ValueSet.Values.(type) {
					case *dashboardpb.ChartValueSet_SingleValues:
						for _, val := range f.ValueSet.GetSingleValues().AttributeValues {
							if val.GetSubcategoryValue() != nil {
								return QueryEngineES, nil
							}
						}
					}
				}
			}
		}
		if groupBys := chart.DataConfig.GroupBy; len(groupBys) > 0 {
			for _, groupby := range groupBys {
				//nolint:gocritic
				switch groupby.GroupByMode.(type) {
				case *dashboardpb.ChartGroupBy_ByProvidedValue:
					if valueSet := groupby.GetByProvidedValue().ValueSet; valueSet != nil {
						//nolint:gocritic
						switch valueSet.Values.(type) {
						case *dashboardpb.ChartValueSet_SingleValues:
							for _, val := range valueSet.GetSingleValues().AttributeValues {
								if val.GetSubcategoryValue() != nil {
									return QueryEngineES, nil
								}
							}
						}
					}
				}
			}
		}
	}
	// TODO(zhucaiyi): change default to ClickHouse	once ClickHouse	queries are implemented.
	return QueryEngineES, nil
}

func hasNotApplicableValue(forScorecardCriterion *dashboardpb.ChartMetric_ForScorecardCriterion) bool {
	for _, scorecardCriterion := range forScorecardCriterion.GetScorecardCriterions() {
		for _, score := range scorecardCriterion.GetScores() {
			if score.GetNotApplicable() {
				return true
			}
		}
	}
	return false
}

func isSubcategoryAttributeType(t attributeValueType) bool {
	return t == crestaEntitySubcategory ||
		t == crestaEntityAggregateSubcategory ||
		t == crestaEntityAggregateAndLeafSubcategory
}

func sortedSubcategoryMomentNames(cvse *chartValueSetEquivalent) []string {
	names := make([]string, 0, len(cvse.subcategories))
	for _, s := range cvse.subcategories {
		names = append(names, s.MomentName)
	}
	slices.Sort(names)
	return names
}

func sortedAggregateSubcategoryMomentNames(cvse *chartValueSetEquivalent) []string {
	names := make([]string, 0, len(cvse.aggregateSubcategories))
	for _, s := range cvse.aggregateSubcategories {
		names = append(names, s.MomentName)
	}
	slices.Sort(names)
	return names
}

// subcategoryValueSetsMatch checks whether two chartValueSetEquivalents of the same subcategory
// type have identical moment names. For subcategory and aggregate subcategory types, all moment
// names must match.
func subcategoryValueSetsMatch(a, b *chartValueSetEquivalent) bool {
	if a.attributeValueType != b.attributeValueType {
		return false
	}
	switch a.attributeValueType {
	case crestaEntitySubcategory:
		return slices.Equal(sortedSubcategoryMomentNames(a), sortedSubcategoryMomentNames(b))
	case crestaEntityAggregateSubcategory:
		return slices.Equal(sortedAggregateSubcategoryMomentNames(a), sortedAggregateSubcategoryMomentNames(b))
	case crestaEntityAggregateAndLeafSubcategory:
		return slices.Equal(sortedSubcategoryMomentNames(a), sortedSubcategoryMomentNames(b)) &&
			slices.Equal(sortedAggregateSubcategoryMomentNames(a), sortedAggregateSubcategoryMomentNames(b))
	default:
		return false
	}
}

// filtersContainMatchingSubcategoryValueSet checks whether any existing ChartDataFilter has the
// same subcategory-related type with matching moment names as the given ChartValueSet. This is
// used to de-duplicate when metrics are moved into filters, since dashboards may already have a
// filter with the same subcategories. Only applies to subcategory and aggregate subcategory types.
func filtersContainMatchingSubcategoryValueSet(ctx context.Context, filters []*dashboardpb.ChartDataFilter, valueSet *dashboardpb.ChartValueSet) bool {
	if valueSet == nil {
		return false
	}
	newCvse, err := fromChartValueSet(ctx, valueSet)
	if err != nil || !isSubcategoryAttributeType(newCvse.attributeValueType) {
		return false
	}
	for _, f := range filters {
		if f.GetValueSet() == nil {
			continue
		}
		existingCvse, err := fromChartValueSet(ctx, f.GetValueSet())
		if err != nil {
			continue
		}
		if subcategoryValueSetsMatch(newCvse, existingCvse) {
			return true
		}
	}
	return false
}

// groupBysContainMatchingSubcategoryValueSet checks whether any existing ChartGroupBy
// (byProvidedValue) has the same subcategory-related type with matching moment names as the
// given ChartValueSet. This is used to de-duplicate when metrics are moved into groupBys, since
// dashboards may already have a groupBy with the same subcategories. Only applies to subcategory and
// aggregate subcategory.
func groupBysContainMatchingSubcategoryValueSet(ctx context.Context, groupBys []*dashboardpb.ChartGroupBy, valueSet *dashboardpb.ChartValueSet) bool {
	if valueSet == nil {
		return false
	}
	newCvse, err := fromChartValueSet(ctx, valueSet)
	if err != nil || !isSubcategoryAttributeType(newCvse.attributeValueType) {
		return false
	}
	for _, g := range groupBys {
		if g.GetByProvidedValue() == nil || g.GetByProvidedValue().GetValueSet() == nil {
			continue
		}
		existingCvse, err := fromChartValueSet(ctx, g.GetByProvidedValue().GetValueSet())
		if err != nil {
			continue
		}
		if subcategoryValueSetsMatch(newCvse, existingCvse) {
			return true
		}
	}
	return false
}

// Check if:
// - we're dealing with subcategories in groupby
// - there are no conversation attribute filters
// - the number of message level attributes in filters and groupby is less than 2
// - there are no excluded subcategories (if there are we need to query on conversation doc and join with message to ensure we remove conversations with excluded subcategories)
// We can remove joins by filtering on message if all the conditions are met.
func (a AnalyticsServiceImpl) canRemoveJoinsByFilteringOnMessage(ctx context.Context, documentTypeForAggregation elasticsearch.DocumentType, chartGroupBys []*dashboardpb.ChartGroupBy, chartDataFilters []*dashboardpb.ChartDataFilter, statsType *statsType, subCategoryHasPinnedClusters bool, profileConfig *config.ProfileConfig) (bool, error) {
	a.logger.Infof(ctx, "flag enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart: %v", a.enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart)
	hasSubcategoryGroupBy, hasSubcategoryFilter, hasConversationAttributeFiltersOrGroupBys := false, false, false
	hasExcludedSubcategory := false
	// The number of filters and groupbys where message-level attributes are used.
	numberOfFilterAndGroupByWithMsgLevelAttribute := 0
	if a.enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart && documentTypeForAggregation == elasticsearch.DocumentTypeConversation {
		for _, groupBy := range chartGroupBys {
			if groupBy.GetByProvidedValue() == nil || groupBy.GetByProvidedValue().GetValueSet() == nil || groupBy.GetByProvidedValue().GetValueSet().GetSingleValues() == nil {
				continue
			}

			valueSet := groupBy.GetByProvidedValue().GetValueSet()
			isMsg, isConvo, err := a.isMessageOrConvoEntity(ctx, profileConfig, valueSet)
			if err != nil {
				return false, err
			}
			if isMsg && !isConvo {
				numberOfFilterAndGroupByWithMsgLevelAttribute++
			}
			if isConvo && !isMsg {
				hasConversationAttributeFiltersOrGroupBys = true
			}

			for _, val := range valueSet.GetSingleValues().GetAttributeValues() {
				if val.GetSubcategoryValue() != nil || val.GetAggregateSubcategoryValue() != nil {
					hasSubcategoryGroupBy = true
					if val.GetOperator() == dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE {
						hasExcludedSubcategory = true
					}
				}
			}
		}
		for _, filter := range chartDataFilters {
			if filter.GetValueSet() == nil || filter.GetValueSet().GetSingleValues() == nil {
				continue
			}
			isMsg, isConvo, err := a.isMessageOrConvoEntity(ctx, profileConfig, filter.GetValueSet())
			if err != nil {
				return false, err
			}
			if isMsg && !isConvo {
				numberOfFilterAndGroupByWithMsgLevelAttribute++
			}
			if isConvo && !isMsg {
				hasConversationAttributeFiltersOrGroupBys = true
			}
			for _, val := range filter.GetValueSet().GetSingleValues().GetAttributeValues() {
				if val.GetSubcategoryValue() != nil || val.GetAggregateSubcategoryValue() != nil {
					hasSubcategoryFilter = true
					if val.GetOperator() == dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE {
						hasExcludedSubcategory = true
					}
				}
			}

		}
	}
	a.logger.Infof(ctx, "canRemoveJoinsByFilteringOnMessage: statsTypeAsConvoCount=%v, hasSubcategoryGroupBy=%v, hasSubcategoryFilter=%v, noPinnedClusters=%v, noConvoAttributeAsFiltersOrGroupBys=%v, numberOfFilterAndGroupByWithMsgLevelAttribute=%v, hasExcludedSubcategory=%v",
		statsType.isConvoCount, hasSubcategoryGroupBy, hasSubcategoryFilter, !subCategoryHasPinnedClusters, !hasConversationAttributeFiltersOrGroupBys, numberOfFilterAndGroupByWithMsgLevelAttribute, hasExcludedSubcategory)
	return statsType.isConvoCount &&
		(hasSubcategoryGroupBy || hasSubcategoryFilter) &&
		!subCategoryHasPinnedClusters &&
		!hasConversationAttributeFiltersOrGroupBys &&
		!hasExcludedSubcategory &&
		(numberOfFilterAndGroupByWithMsgLevelAttribute <= 1), nil
}

func (a AnalyticsServiceImpl) fetchTopicDetailsForChart(ctx context.Context, profileName string, chartGroupBys []*dashboardpb.ChartGroupBy, chartDataFilters []*dashboardpb.ChartDataFilter) (topics map[string]elasticsearch.TopicDetails, aggregateExpansions map[string][]string, err error) {
	subcategoryNames := []string{}
	aggregateSubcategoryNames := []string{}

	// Extract subcategory and aggregate subcategory names from groupBys
	for _, groupBy := range chartGroupBys {
		if groupBy.GetByProvidedValue() != nil && groupBy.GetByProvidedValue().ValueSet != nil {
			if groupBy.GetByProvidedValue().ValueSet.GetSingleValues() != nil {
				for _, val := range groupBy.GetByProvidedValue().ValueSet.GetSingleValues().AttributeValues {
					if val.GetSubcategoryValue() != nil {
						subcategoryNames = append(subcategoryNames, val.GetSubcategoryValue().MomentName)
					}
					if val.GetAggregateSubcategoryValue() != nil {
						aggregateSubcategoryNames = append(aggregateSubcategoryNames, val.GetAggregateSubcategoryValue().MomentName)
					}
				}
			}
		}
	}

	// Extract subcategory and aggregate subcategory names from filters
	for _, filter := range chartDataFilters {
		if filter.GetValueSet() != nil && filter.GetValueSet().GetSingleValues() != nil {
			for _, value := range filter.GetValueSet().GetSingleValues().GetAttributeValues() {
				if value.GetSubcategoryValue() != nil {
					subcategoryNames = append(subcategoryNames, value.GetSubcategoryValue().MomentName)
				}
				if value.GetAggregateSubcategoryValue() != nil {
					aggregateSubcategoryNames = append(aggregateSubcategoryNames, value.GetAggregateSubcategoryValue().MomentName)
				}
			}
		}
	}

	// Collect all moment names to fetch in a single batch
	allMomentNamesToFetch := set.New[string]()

	// Add regular subcategory names
	for _, name := range subcategoryNames {
		allMomentNamesToFetch.Add(name)
	}

	// Expand aggregate subcategories and collect their leaf nodes
	if len(aggregateSubcategoryNames) > 0 {
		profile, err := shared.ParseCustomerProfile(profileName)
		if err != nil {
			a.logger.Errorf(
				ctx,
				"failed to parse customer profile while fetching chart topic details (profile=%q): %v",
				profileName,
				err,
			)
			return nil, nil, err
		}

		aggregateToLeaves, nonAggregateNames, err := a.resolveAggregateMoments(ctx, *profile, aggregateSubcategoryNames)
		if err != nil {
			a.logger.Errorf(
				ctx,
				"failed to resolve aggregate subcategories (profile=%q, aggregate_subcategory_count=%d, aggregate_subcategories=%v): %v",
				profileName,
				len(aggregateSubcategoryNames),
				truncateMomentNamesForErrorLog(aggregateSubcategoryNames),
				err,
			)
			return nil, nil, err
		}

		// Store the aggregate expansions to return (so downstream code can reuse)
		aggregateExpansions = aggregateToLeaves

		// Collect all leaf nodes from aggregate expansion
		for _, leafNodes := range aggregateToLeaves {
			for _, leafName := range leafNodes {
				allMomentNamesToFetch.Add(leafName)
			}
		}

		// Also add non-aggregate names (these are leaf nodes themselves)
		for name := range nonAggregateNames.All() {
			allMomentNamesToFetch.Add(name)
		}
	}

	// Fetch topic details for all subcategories and leaf nodes in a single call
	if allMomentNamesToFetch.Len() > 0 {
		topics, err = a.fetchTopicDetails(ctx, profileName, allMomentNamesToFetch.Slice())
		if err != nil {
			a.logger.Errorf(
				ctx,
				"failed to fetch topic details for chart (profile=%q, subcategory_count=%d, aggregate_subcategory_count=%d, total_moment_count=%d, sample_moments=%v): %v",
				profileName,
				len(subcategoryNames),
				len(aggregateSubcategoryNames),
				allMomentNamesToFetch.Len(),
				truncateMomentNamesForErrorLog(allMomentNamesToFetch.Slice()),
				err,
			)
			return nil, nil, err
		}
	} else {
		topics = map[string]elasticsearch.TopicDetails{}
	}

	return topics, aggregateExpansions, nil
}

func hasPinnedClusters(topics map[string]elasticsearch.TopicDetails) bool {
	for _, topic := range topics {
		if len(topic.Clusters) > 0 {
			return true
		}
	}
	return false
}

func (a AnalyticsServiceImpl) convertSubcategories(ctx context.Context, profileName string, included []*dashboardpb.SubcategoryForChart, excluded []*dashboardpb.SubcategoryForChart, prefetchedTopics map[string]elasticsearch.TopicDetails) (includedTopicBools []*elasticsearch.TopicBool, excludedTopicBools []*elasticsearch.TopicBool, err error) {
	convert := func(subcategories []*dashboardpb.SubcategoryForChart) ([]*elasticsearch.TopicBool, error) {
		var missingNames []string
		for _, subcate := range subcategories {
			if _, ok := prefetchedTopics[subcate.MomentName]; !ok {
				missingNames = append(missingNames, subcate.MomentName)
			}
		}

		var additionalTopics map[string]elasticsearch.TopicDetails
		if len(missingNames) > 0 {
			var err error
			additionalTopics, err = a.fetchTopicDetails(ctx, profileName, missingNames)
			if err != nil {
				a.logger.Errorf(
					ctx,
					"failed to fetch missing subcategory topic details (profile=%q, missing_moment_count=%d, missing_moments=%v): %v",
					profileName,
					len(missingNames),
					truncateMomentNamesForErrorLog(missingNames),
					err,
				)
				return nil, err
			}
		}

		var result []*elasticsearch.TopicBool
		for _, subcate := range subcategories {
			topic, ok := prefetchedTopics[subcate.MomentName]
			if !ok && additionalTopics != nil {
				topic, ok = additionalTopics[subcate.MomentName]
			}
			if !ok {
				continue
			}
			topicBool, err := elasticsearch.ConvertTopicDetailToTopicBool(topic, []string{})
			if err != nil {
				a.logger.Errorf(
					ctx,
					"failed to convert topic details to topic bool for subcategory (profile=%q, moment_name=%q): %v",
					profileName,
					subcate.MomentName,
					err,
				)
				return nil, err
			}
			result = append(result, topicBool)
		}
		return result, nil
	}
	includedTopicBools, err = convert(included)
	if err != nil {
		return nil, nil, err
	}
	excludedTopicBools, err = convert(excluded)
	if err != nil {
		return nil, nil, err
	}
	return includedTopicBools, excludedTopicBools, nil
}

// splitAggregateMoments classifies moment names into aggregate and leaf categories based on
// the fetched moment data. An aggregate moment is of type composite with empty moment data.
// Moment names not found in the fetched list are classified as leaves.
func splitAggregateMoments(allMoments []*momentpb.Moment, momentNames []string) (aggregateNames, leafNames set.Set[string]) {
	aggregateNames = set.New[string]()
	leafNames = set.New[string]()
	momentByName := make(map[string]*momentpb.Moment, len(allMoments))
	for _, m := range allMoments {
		momentByName[m.GetName()] = m
	}
	for _, name := range momentNames {
		m, ok := momentByName[name]
		if ok && isAggregateCategory(m) {
			aggregateNames.Add(name)
		} else {
			leafNames.Add(name)
		}
	}
	return aggregateNames, leafNames
}

func truncateMomentNamesForErrorLog(momentNames []string) []string {
	if len(momentNames) <= maxMomentNamesInErrorLog {
		return momentNames
	}
	return momentNames[:maxMomentNamesInErrorLog]
}

func (a AnalyticsServiceImpl) convertAggregateSubcategory(ctx context.Context, profile shared.CustomerProfile, included []*dashboardpb.AggregateSubcategoryForChart, excluded []*dashboardpb.AggregateSubcategoryForChart, prefetchedTopics map[string]elasticsearch.TopicDetails, prefetchedAggregateExpansions map[string][]string) (includedResult map[string][]*elasticsearch.TopicBool, excludedResult map[string][]*elasticsearch.TopicBool, err error) {
	allInputs := make([]*dashboardpb.AggregateSubcategoryForChart, 0, len(included)+len(excluded))
	allInputs = append(allInputs, included...)
	allInputs = append(allInputs, excluded...)
	includedResult = map[string][]*elasticsearch.TopicBool{}
	excludedResult = map[string][]*elasticsearch.TopicBool{}
	if len(allInputs) == 0 {
		return includedResult, excludedResult, nil
	}

	var aggregateToLeaves map[string][]string
	var leafSet set.Set[string]

	// Use prefetched aggregate expansions if available, otherwise resolve them
	if len(prefetchedAggregateExpansions) > 0 {
		aggregateToLeaves = prefetchedAggregateExpansions
		// Build leafSet from the names that are NOT in aggregateToLeaves
		leafSet = set.New[string]()
		for _, input := range allInputs {
			if _, isAggregate := aggregateToLeaves[input.MomentName]; !isAggregate {
				leafSet.Add(input.MomentName)
			}
		}
	} else {
		// Fall back to resolving aggregates (for backward compatibility)
		allMomentNames := make([]string, 0, len(allInputs))
		for _, input := range allInputs {
			allMomentNames = append(allMomentNames, input.MomentName)
		}

		var err error
		aggregateToLeaves, leafSet, err = a.resolveAggregateMoments(ctx, profile, allMomentNames)
		if err != nil {
			return nil, nil, err
		}
	}

	// Handle leaf moments using convertSubcategories.
	for _, inc := range included {
		if !leafSet.Has(inc.MomentName) {
			continue
		}
		topicBools, _, err := a.convertSubcategories(
			ctx, profile.String(),
			[]*dashboardpb.SubcategoryForChart{{MomentName: inc.MomentName}},
			nil, prefetchedTopics,
		)
		if err != nil {
			return nil, nil, err
		}
		includedResult[inc.MomentName] = topicBools
	}
	for _, exc := range excluded {
		if !leafSet.Has(exc.MomentName) {
			continue
		}
		_, topicBools, err := a.convertSubcategories(
			ctx, profile.String(),
			nil,
			[]*dashboardpb.SubcategoryForChart{{MomentName: exc.MomentName}},
			prefetchedTopics,
		)
		if err != nil {
			return nil, nil, err
		}
		excludedResult[exc.MomentName] = topicBools
	}

	// Handle aggregate moments with hierarchy traversal.
	for _, inc := range included {
		if leafNodes, isAggregate := aggregateToLeaves[inc.MomentName]; isAggregate {
			leafSubcategories := make([]*dashboardpb.SubcategoryForChart, 0, len(leafNodes))
			for _, leafName := range leafNodes {
				leafSubcategories = append(leafSubcategories, &dashboardpb.SubcategoryForChart{
					MomentName: leafName,
				})
			}

			topicBools, _, err := a.convertSubcategories(ctx, profile.String(), leafSubcategories, nil, prefetchedTopics)
			if err != nil {
				return nil, nil, err
			}
			includedResult[inc.MomentName] = topicBools
		}
	}

	for _, exc := range excluded {
		if leafNodes, isAggregate := aggregateToLeaves[exc.MomentName]; isAggregate {
			leafSubcategories := make([]*dashboardpb.SubcategoryForChart, 0, len(leafNodes))
			for _, leafName := range leafNodes {
				leafSubcategories = append(leafSubcategories, &dashboardpb.SubcategoryForChart{
					MomentName: leafName,
				})
			}

			_, topicBools, err := a.convertSubcategories(ctx, profile.String(), nil, leafSubcategories, prefetchedTopics)
			if err != nil {
				return nil, nil, err
			}
			excludedResult[exc.MomentName] = topicBools
		}
	}

	return includedResult, excludedResult, nil
}
