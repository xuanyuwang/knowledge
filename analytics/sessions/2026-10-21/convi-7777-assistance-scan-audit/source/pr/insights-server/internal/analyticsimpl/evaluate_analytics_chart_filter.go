package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	actionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/action"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/time"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"

	dbmomentpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/moment"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedanalytics "github.com/cresta/go-servers/shared/analytics"
	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"

	"github.com/golang/protobuf/proto"
	"golang.org/x/text/language"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChartESQuery struct {
	Bool *ChartESQueryBool `json:"bool,omitempty"`
	// For random sorting when fetching sample documents.
	FunctionScore *elasticsearch.FunctionScore `json:"function_score,omitempty"`
}

type ChartESQueryBool struct {
	Filter *[]any `json:"filter,omitempty"`
}

// The ES filter converted from a ChartDataFilter object.
type ChartDataFilterFilter struct {
	Bool      *ChartDataFilterFilterBool `json:"bool,omitempty"`
	Terms     *elasticsearch.TermsFilter `json:"terms,omitempty"`
	HasChild  *elasticsearch.HasChild    `json:"has_child,omitempty"`
	HasParent *elasticsearch.HasParent   `json:"has_parent,omitempty"`
	Range     *elasticsearch.RangeFilter `json:"range,omitempty"`
}

// The bool field of FilterFromChartFilter.
type ChartDataFilterFilterBool struct {
	Filter *ChartValueSetFilter `json:"filter,omitempty"`
	// For NOT_IN operator.
	MustNot *ChartValueSetFilter `json:"must_not,omitempty"`
}

// The ES filter from ChartValueSet object.
// ChartValueSet is essentially a list of single values or ranges of different value types.
type ChartValueSetFilter struct {
	Bool *ChartValueSetFilterBool `json:"bool,omitempty"`
}

type ChartValueSetFilterBool struct {
	Should             *[]any `json:"should,omitempty"`
	MinimumShouldMatch int32  `json:"minimum_should_match,omitempty"`
	// Exclude filters: documents matching any of these must not be returned.
	MustNot *[]any `json:"must_not,omitempty"`
}

type timeRange struct {
	from *timestamppb.Timestamp
	to   *timestamppb.Timestamp
}

func (a AnalyticsServiceImpl) convertChartToESQuery(ctx context.Context, profile shared.CustomerProfile, profileConfig *config.ProfileConfig, documentType elasticsearch.DocumentType, chartDataConfig *dashboardpb.ChartDataConfig, convoFilter *analyticspb.AnalyticsChartFilter, metadata *analyticspb.Metadata, prefetchedTopics map[string]elasticsearch.TopicDetails, convoFilterFromMsgEntity map[*dashboardpb.ChartDataFilter]any, prefetchedAggregateExpansions map[string][]string, prefetchedV3ClusterClosures map[string][]string, agentFilterAlwaysOnConversation bool) (ChartESQuery, error) {
	filters := []any{}
	chartFilters := chartDataConfig.Filters
	chartNativeFilters, err := a.convertChartFilters(ctx, profile, profileConfig, documentType, chartFilters, metadata, prefetchedTopics, convoFilterFromMsgEntity, prefetchedAggregateExpansions, prefetchedV3ClusterClosures, agentFilterAlwaysOnConversation)
	if err != nil {
		return ChartESQuery{}, err
	}
	for _, f := range chartNativeFilters {
		filters = append(filters, f)
	}

	convoFilters, err := a.convertAdhocFilter(ctx, documentType, convoFilter)
	if err != nil {
		return ChartESQuery{}, err
	}
	filters = append(filters, convoFilters...)

	if len(filters) == 0 {
		return ChartESQuery{}, status.Errorf(codes.Internal, "empty filters in ES query")
	}
	if IsConversationCountForScorecardCriterionMetric(chartDataConfig) {
		filters = append(filters, filterCalibrationScorecardsForConversationCount())
	}
	return ChartESQuery{
		Bool: &ChartESQueryBool{
			Filter: &filters,
		},
	}, nil
}

func validateChartKeywordFilter(keywordFilter *dashboardpb.KeywordFilterForChart) error {
	if keywordFilter == nil {
		return status.Error(codes.InvalidArgument, "chart keyword_filter must be non-empty")
	}
	languageCode := keywordFilter.GetLanguageCode()
	if languageCode != "" && strings.TrimSpace(languageCode) == "" {
		return status.Error(codes.InvalidArgument, "chart keyword_filter.language_code must be non-blank when supplied")
	}
	if languageCode != "" && languageCode != consts.AllLanguages {
		if _, err := language.Parse(languageCode); err != nil || strings.Contains(languageCode, "_") {
			return status.Error(codes.InvalidArgument, "chart keyword_filter.language_code must be a valid BCP-47 language tag")
		}
	}
	expression := keywordFilter.GetKeywordExpression()
	if len(expression.GetOperands()) == 0 {
		return status.Error(codes.InvalidArgument, "chart keyword_filter.keyword_expression must contain operands")
	}
	if expression.GetOperator() != momentpb.KeywordMomentData_AND && expression.GetOperator() != momentpb.KeywordMomentData_OR {
		return status.Error(codes.InvalidArgument, "chart keyword_filter.keyword_expression operator must be AND or OR")
	}
	for _, operand := range expression.GetOperands() {
		if err := validateChartKeywordOperand(operand); err != nil {
			return err
		}
	}
	return nil
}

func keywordFilterToAnalyticsChartFilter(keywordFilter *dashboardpb.KeywordFilterForChart) (*analyticspb.AnalyticsChartFilter, error) {
	if err := validateChartKeywordFilter(keywordFilter); err != nil {
		return nil, err
	}
	return &analyticspb.AnalyticsChartFilter{
		InsightsTopic: &analyticspb.InsightsTopic{
			IncludedKeywordMoment: &momentpb.Moment{
				Type:         momentpb.Moment_KEYWORD,
				AppTypes:     []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
				MomentSource: momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
				LanguageCode: keywordFilter.GetLanguageCode(),
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_KeywordMomentData{
						KeywordMomentData: &momentpb.KeywordMomentData{
							KeywordExpression: keywordFilter.GetKeywordExpression(),
						},
					},
				},
			},
		},
	}, nil
}

func validateChartKeywordOperand(operand *momentpb.KeywordMomentData_Operand) error {
	if strings.TrimSpace(operand.GetPhrase()) != "" {
		return nil
	}
	sequence := operand.GetExpression()
	if sequence.GetOperator() != momentpb.KeywordMomentData_SEQUENCE || len(sequence.GetOperands()) == 0 {
		return status.Error(codes.InvalidArgument, "chart keyword_filter.keyword_expression operands must be non-empty phrases or SEQUENCE expressions")
	}
	for _, word := range sequence.GetOperands() {
		if strings.TrimSpace(word.GetPhrase()) == "" {
			return status.Error(codes.InvalidArgument, "chart keyword_filter.keyword_expression SEQUENCE operands must be non-empty phrases")
		}
	}
	return nil
}

func (a AnalyticsServiceImpl) convertAdhocFilter(ctx context.Context, documentType elasticsearch.DocumentType, convoFilter *analyticspb.AnalyticsChartFilter) ([]any, error) {
	ret := []any{}
	if convoFilter == nil {
		return ret, nil
	}
	if t := convoFilter.InsightsTopic; t != nil {
		td, err := a.convertInsightsTopicToTopicDetails(ctx, insightsTopic{apiInsightsTopic: t})
		if err != nil {
			a.logger.Errorf(
				ctx,
				"failed to convert adhoc insights topic to topic details (topic_moment_name=%q): %v",
				t.GetTopicMoment().GetName(),
				err,
			)
			return nil, err
		}
		if td != nil {
			topicBool, err := elasticsearch.ConvertTopicDetailToTopicBool(*td, []string{})
			if err != nil {
				a.logger.Errorf(
					ctx,
					"failed to convert adhoc insights topic details to topic bool (topic_moment_name=%q, document_type=%v): %v",
					t.GetTopicMoment().GetName(),
					documentType,
					err,
				)
				return nil, err
			}
			convoFilter := elasticsearch.ConvertTopicBoolsToConvoFilter([]*elasticsearch.TopicBool{topicBool})
			if documentType == elasticsearch.DocumentTypeConversation {
				ret = append(ret, convoFilter)
			} else {
				ret = append(ret, elasticsearch.MsgFilter{
					HasParent: &elasticsearch.HasParent{
						ParentType: string(elasticsearch.DocumentTypeConversation),
						Query: elasticsearch.ConvoQuery{
							Bool: elasticsearch.ConvoBool{
								Filter: &[]elasticsearch.ConvoFilter{
									convoFilter,
								},
							},
						},
					},
				})
			}
		}
	}
	return ret, nil
}

// conver object ChartDataFilter to a ES filter.
func (a AnalyticsServiceImpl) convertChartFilters(ctx context.Context, profile shared.CustomerProfile, profileConfig *config.ProfileConfig, esQueryDocType elasticsearch.DocumentType, chartDataFilters []*dashboardpb.ChartDataFilter, metadata *analyticspb.Metadata, prefetchedTopics map[string]elasticsearch.TopicDetails, convoFilterFromMsgEntity map[*dashboardpb.ChartDataFilter]any, prefetchedAggregateExpansions map[string][]string, prefetchedV3ClusterClosures map[string][]string, agentFilterAlwaysOnConversation bool) ([]ChartDataFilterFilter, error) {
	// When subcategory has pinned cluster, we have to add message_group along with message to conversation_to_message_join.
	hasPinnedCluster := hasPinnedClusters(prefetchedTopics)
	conversationToMessageJoin := []string{string(esQueryDocType)}
	if esQueryDocType == elasticsearch.DocumentTypeMessage && hasPinnedCluster {
		conversationToMessageJoin = append(conversationToMessageJoin, string(elasticsearch.DocumentTypeMessageGroup))
	}
	filters := []ChartDataFilterFilter{
		{
			Terms: &elasticsearch.TermsFilter{
				ConversationToMessageJoin: &conversationToMessageJoin,
			},
		},
	}

	// When an explicit conversation source IN filter is present, it replaces the default
	// conversation source filter to allow filtering by any source value.
	hasExplicitConvSourceINFilter := hasConversationSourceINFilter(chartDataFilters)
	filters = append(filters, a.getDefaultFilters(esQueryDocType, hasPinnedCluster, hasExplicitConvSourceINFilter)...)

	if len(chartDataFilters) == 0 {
		return filters, nil
	}

	for _, chartFilter := range chartDataFilters {
		isConvoFilterFromMsgEntity := false
		if _, found := convoFilterFromMsgEntity[chartFilter]; found {
			isConvoFilterFromMsgEntity = true
		}

		filterFromValueSet, err := a.filterFromChartValueSet(ctx, profile, profileConfig, esQueryDocType, hasPinnedCluster, chartFilter.ValueSet, metadata, prefetchedTopics, isConvoFilterFromMsgEntity, prefetchedAggregateExpansions, prefetchedV3ClusterClosures, agentFilterAlwaysOnConversation)
		if err != nil {
			return nil, err
		}
		if filterFromValueSet == nil {
			continue
		}

		f := ChartDataFilterFilter{
			Bool: &ChartDataFilterFilterBool{},
		}
		switch chartFilter.Operator {
		case dashboardpb.ChartDataFilter_IN:
			f.Bool.Filter = filterFromValueSet
		case dashboardpb.ChartDataFilter_NOT_IN:
			f.Bool.MustNot = filterFromValueSet
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported operator %v", chartFilter.Operator)
		}
		filters = append(filters, f)
	}
	return filters, nil
}

func (a AnalyticsServiceImpl) getDefaultFilters(documentType elasticsearch.DocumentType, hasPinnedCluster bool, skipConversationSourceFilter bool) []ChartDataFilterFilter {
	filters := []ChartDataFilterFilter{}

	if !skipConversationSourceFilter {
		defaultConvoSources := elasticsearch.GetDefaultConversationSources()
		// conversation_source are not denormalized message_group documents.
		// So if there are pinned clusters we cannot apply this filter on message_group documents
		// directly, instead we have to fall back to conversation documents only.
		// TODO(zhucaiyi): remove this logic once message_group document is deprecated.
		if documentType == elasticsearch.DocumentTypeMessage && hasPinnedCluster {
			filters = append(filters, ChartDataFilterFilter{
				HasParent: &elasticsearch.HasParent{
					ParentType: string(elasticsearch.DocumentTypeConversation),
					Query: elasticsearch.ConvoQuery{
						Bool: elasticsearch.ConvoBool{
							Filter: &[]elasticsearch.ConvoFilter{
								{
									Terms: &elasticsearch.TermsFilter{
										ConversationSources: &defaultConvoSources,
									},
								},
							},
						},
					},
				},
			})
		} else {
			filters = append(filters, ChartDataFilterFilter{
				Terms: &elasticsearch.TermsFilter{
					ConversationSources: &defaultConvoSources,
				},
			})
		}
	}

	// If document_type is message, there is no need to add this filter of message_count > 0.
	if documentType == elasticsearch.DocumentTypeConversation {
		filters = append(filters, ChartDataFilterFilter{
			Range: &elasticsearch.RangeFilter{
				ConversationMessageCount: &elasticsearch.FloatRange{
					GT: proto.Float64(0.0),
				},
			},
		})
	}
	return filters
}

// convert object ChartValueSet to a ES filter.
func (a AnalyticsServiceImpl) filterFromChartValueSet(ctx context.Context, profile shared.CustomerProfile, profileConfig *config.ProfileConfig, documentType elasticsearch.DocumentType, hasPinnedCluster bool, valSet *dashboardpb.ChartValueSet, metadata *analyticspb.Metadata, prefetchedTopics map[string]elasticsearch.TopicDetails, isConvoFilterFromMsgEntity bool, prefetchedAggregateExpansions map[string][]string, prefetchedV3ClusterClosures map[string][]string, agentFilterAlwaysOnConversation bool) (*ChartValueSetFilter, error) {
	if documentType != elasticsearch.DocumentTypeConversation && documentType != elasticsearch.DocumentTypeMessage {
		return nil, status.Errorf(codes.InvalidArgument, "target document type has to be message or conversation, found %v", documentType)
	}
	if valSet == nil {
		return nil, nil
	}

	filterSiblingMsg := func(f any) any {
		return map[string]any{
			"has_parent": map[string]any{
				"parent_type": "conversation",
				"query": map[string]any{
					"has_child": map[string]any{
						"type":  "message",
						"query": f,
					},
				},
			},
		}
	}

	should := []any{}
	mustNot := []any{}
	switch valSet.Values.(type) {
	case *dashboardpb.ChartValueSet_SingleValues, *dashboardpb.ChartValueSet_KeywordFilterValue:
		cvse, err := fromChartValueSet(ctx, valSet)
		if err != nil {
			return nil, err
		}
		//nolint:gocritic
		switch cvse.attributeValueType {
		case attributeValueTypeTranscriptKeywords:
			keywordFilter, err := keywordFilterToAnalyticsChartFilter(cvse.keywordFilter)
			if err != nil {
				return nil, err
			}
			keywordFilters, err := a.convertAdhocFilter(ctx, documentType, keywordFilter)
			if err != nil {
				return nil, err
			}
			if len(keywordFilters) == 0 {
				return nil, status.Error(codes.InvalidArgument, "chart keyword_filter must produce a query")
			}
			should = append(should, keywordFilters...)
		// TODO(zhucaiyi): simplify the query by calling filterSiblingMsg for subcategories after
		// message_group is deprecated.
		case crestaEntitySubcategory:
			topicBoolsIncluded, topicBoolsExcluded, err := a.convertSubcategories(ctx, profile.String(), cvse.subcategories, cvse.subcategoriesExclude, prefetchedTopics)
			if err != nil {
				return nil, err
			}
			if len(topicBoolsIncluded) > 0 {
				f, err := a.filterFromTopicBools(ctx, topicBoolsIncluded, documentType, isConvoFilterFromMsgEntity)
				if err != nil {
					return nil, err
				}
				should = append(should, f)
			}
			for _, topicBool := range topicBoolsExcluded {
				f, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{topicBool}, documentType, isConvoFilterFromMsgEntity)
				if err != nil {
					return nil, err
				}
				mustNot = append(mustNot, f)
			}
		case crestaEntityAggregateSubcategory:
			topicBoolsByAggregateIncluded, topicBoolsByAggregateExcluded, err := a.convertAggregateSubcategory(ctx, profile, cvse.aggregateSubcategories, cvse.aggregateSubcategoriesExclude, prefetchedTopics, prefetchedAggregateExpansions)
			if err != nil {
				return nil, err
			}
			for _, aggSub := range cvse.aggregateSubcategories {
				topicBools := topicBoolsByAggregateIncluded[aggSub.MomentName]
				var f any
				if len(topicBools) == 0 {
					f = createMatchNoneFilter(documentType)
				} else {
					f, err = a.filterFromTopicBools(ctx, topicBools, documentType, isConvoFilterFromMsgEntity)
					if err != nil {
						return nil, err
					}
				}
				should = append(should, f)
			}
			for _, aggSub := range cvse.aggregateSubcategoriesExclude {
				aggTopicBools := topicBoolsByAggregateExcluded[aggSub.MomentName]
				for _, topicBool := range aggTopicBools {
					f, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{topicBool}, documentType, isConvoFilterFromMsgEntity)
					if err != nil {
						return nil, err
					}
					mustNot = append(mustNot, f)
				}
			}
		case crestaEntityAggregateAndLeafSubcategory:
			topicBoolsForSubcategoriesIncluded, topicBoolsForSubcategoriesExcluded, err := a.convertSubcategories(ctx, profile.String(), cvse.subcategories, cvse.subcategoriesExclude, prefetchedTopics)
			if err != nil {
				return nil, err
			}
			topicBoolsForAggregateSubcategoriesIncluded, topicBoolsForAggregateSubcategoriesExcluded, err := a.convertAggregateSubcategory(ctx, profile, cvse.aggregateSubcategories, cvse.aggregateSubcategoriesExclude, prefetchedTopics, prefetchedAggregateExpansions)
			if err != nil {
				return nil, err
			}
			if len(topicBoolsForSubcategoriesIncluded) > 0 {
				subcategoryFilter, err := a.filterFromTopicBools(ctx, topicBoolsForSubcategoriesIncluded, documentType, isConvoFilterFromMsgEntity)
				if err != nil {
					return nil, err
				}
				should = append(should, subcategoryFilter)
			}
			for _, aggSub := range cvse.aggregateSubcategories {
				topicBools := topicBoolsForAggregateSubcategoriesIncluded[aggSub.MomentName]
				if len(topicBools) == 0 {
					continue
				}
				aggregateSubcategoryFilter, err := a.filterFromTopicBools(ctx, topicBools, documentType, isConvoFilterFromMsgEntity)
				if err != nil {
					return nil, err
				}
				should = append(should, aggregateSubcategoryFilter)
			}
			for _, topicBool := range topicBoolsForSubcategoriesExcluded {
				f, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{topicBool}, documentType, isConvoFilterFromMsgEntity)
				if err != nil {
					return nil, err
				}
				mustNot = append(mustNot, f)
			}
			for _, aggSub := range cvse.aggregateSubcategoriesExclude {
				aggTopicBools := topicBoolsForAggregateSubcategoriesExcluded[aggSub.MomentName]
				for _, topicBool := range aggTopicBools {
					f, err := a.filterFromTopicBools(ctx, []*elasticsearch.TopicBool{topicBool}, documentType, isConvoFilterFromMsgEntity)
					if err != nil {
						return nil, err
					}
					mustNot = append(mustNot, f)
				}
			}
		case crestaEntityCluster:
			f, err := filterFromClusters(ctx, cvse.clusters, documentType, prefetchedV3ClusterClosures)
			if err != nil {
				return nil, err
			}
			if documentType == elasticsearch.DocumentTypeMessage && isConvoFilterFromMsgEntity {
				f = filterSiblingMsg(f)
			}
			should = append(should, f)
		case crestaEntityOperaBlock:
			f, err := filterFromOperaBlocks(ctx, cvse.operaBlocks, documentType)
			if err != nil {
				return nil, err
			}
			if documentType == elasticsearch.DocumentTypeMessage && isConvoFilterFromMsgEntity {
				f = filterSiblingMsg(f)
			}
			should = append(should, f)
		case crestaEntityBehavior:
			// Optimized for performance: single has_child is faster than multiple
			// has_child query.
			f, err := filterFromBehaviors(ctx, cvse.behaviors, documentType)
			if err != nil {
				return nil, err
			}
			if documentType == elasticsearch.DocumentTypeMessage && isConvoFilterFromMsgEntity {
				f = filterSiblingMsg(f)
			}
			should = append(should, f)
		case crestaEntityEmotion:
			f, err := filterFromEmotions(ctx, cvse.emotions, documentType)
			if err != nil {
				return nil, err
			}
			if documentType == elasticsearch.DocumentTypeMessage && isConvoFilterFromMsgEntity {
				f = filterSiblingMsg(f)
			}
			should = append(should, f)
		case crestaEntitySentiment:
			f, err := filterFromSentiments(ctx, cvse.sentiments, documentType)
			if err != nil {
				return nil, err
			}
			if documentType == elasticsearch.DocumentTypeMessage && isConvoFilterFromMsgEntity {
				f = filterSiblingMsg(f)
			}
			should = append(should, f)
		case crestaEntityMetadata:
			// Consolidate values by moment name to reduce number of filters.
			// Clone each entry to avoid mutating the original protos, which may
			// be shared with the groupby ValueSet.
			mdByName := map[string]*dashboardpb.MetadataForChart{}
			for _, md := range cvse.metadatas {
				if _, found := mdByName[md.MomentName]; !found {
					mdByName[md.MomentName] = proto.Clone(md).(*dashboardpb.MetadataForChart)
				} else {
					mdByName[md.MomentName].Values = append(mdByName[md.MomentName].Values, md.Values...)
					mdByName[md.MomentName].NumericValueRanges = append(mdByName[md.MomentName].NumericValueRanges, md.NumericValueRanges...)
				}
			}
			for _, md := range mdByName {
				f, includeMissingValuesFilter, err := a.filterFromMetadatas(ctx, []*dashboardpb.MetadataForChart{md}, documentType, profileConfig)
				if err != nil {
					return nil, err
				}
				if f != nil {
					should = append(should, f)
				}
				if includeMissingValuesFilter != nil {
					should = append(should, includeMissingValuesFilter)
				}
			}
		case crestaEntityLlmAnnotation:
			// Consolidate values by moment name to reduce number of filters.
			// Clone each entry to avoid mutating the original protos, which may
			// be shared with the groupby ValueSet.
			llmAnnotationsByName := map[string]*dashboardpb.LlmAnnotationForChart{}
			for _, md := range cvse.llmAnnotations {
				if _, found := llmAnnotationsByName[md.MomentName]; !found {
					llmAnnotationsByName[md.MomentName] = proto.Clone(md).(*dashboardpb.LlmAnnotationForChart)
				} else {
					llmAnnotationsByName[md.MomentName].Values = append(llmAnnotationsByName[md.MomentName].Values, md.Values...)
					llmAnnotationsByName[md.MomentName].NumericValueRanges = append(llmAnnotationsByName[md.MomentName].NumericValueRanges, md.NumericValueRanges...)
				}
			}
			for _, md := range llmAnnotationsByName {
				f, _, err := a.filterFromLlmAnnotations(ctx, []*dashboardpb.LlmAnnotationForChart{md}, documentType)
				if err != nil {
					return nil, err
				}
				if f != nil {
					should = append(should, f)
				}
			}
		case attributeValueTypeScorecardTemplate:
			f, err := filterFromScorecardTemplates(ctx, cvse.scorecardTemplates, documentType)
			if err != nil {
				return nil, err
			}
			should = append(should, f)
		case attributeValuetypeScorecardCriterion:
			f, err := filterFromScorecardCriterions(ctx, cvse.scorecardCriterions, documentType)
			if err != nil {
				return nil, err
			}
			should = append(should, f)
		case crestaEntityAgentAssistance:
			for _, aa := range cvse.agentAssistances {
				f, err := filterFromAgentAssistance(ctx, aa, documentType)
				if err != nil {
					return nil, err
				}
				should = append(should, f)
			}
		case attributeValueTypeUsecaseName:
			f, err := filterFromUsecaseNames(ctx, cvse.usecaseNames)
			if err != nil {
				return nil, err
			}
			should = append(should, f)
		case attributeValueTypeAgentName:
			f, err := filterFromAgentNames(ctx, cvse.agentNames, documentType, agentFilterAlwaysOnConversation)
			if err != nil {
				return nil, err
			}
			should = append(should, f)
		case attributeValueTypeGroupName:
			f, err := a.filterFromGroupNames(ctx, profile.CustomerID, profile.ProfileID, cvse.groupNames, documentType, agentFilterAlwaysOnConversation)
			if err != nil {
				return nil, err
			}
			should = append(should, f)
		case attributeValueTypeConversationHandleTimeRange:
			for _, rangeValues := range cvse.conversationHandleTimeRanges {
				f, err := filterFromConversationHandleTimeRanges(ctx, rangeValues, documentType)
				if err != nil {
					return nil, err
				}
				should = append(should, f)
			}
		case attributeValueTypeConversationMessageCountRange:
			for _, rangeValues := range cvse.conversationMessageCountRanges {
				f, err := filterFromConversationMessageCountRanges(ctx, rangeValues, documentType)
				if err != nil {
					return nil, err
				}
				should = append(should, f)
			}
		case attributeValueTypeConversationDurationRange:
			for _, rangeVal := range cvse.conversationDurationRanges {
				f, err := a.filterFromConversationDurationRange(ctx, rangeVal, documentType)
				if err != nil {
					return nil, err
				}
				should = append(should, f)
			}
		case attributeValueTypeScorecardSubmitterName:
			f, err := filterFromScorecardSubmitterNames(ctx, cvse.scorecardSubmitterNames, documentType)
			if err != nil {
				return nil, err
			}
			should = append(should, f)
		case attributeValueTypeScorecardStatusFilter:
			f, err := filterFromScorecardStatusFilters(ctx, cvse.scorecardStatusFilters, documentType)
			if err != nil {
				return nil, err
			}
			if len(f) == 0 {
				a.logger.Warnf(ctx, "no valid scorecard status filter after conversion")
			}
			should = append(should, f...)
		case attributeValueTypeAdvancedTimeFilter:
			f, err := filterFromAdvancedTimeFilter(ctx, cvse.advancedTimeFilters, metadata)
			if err != nil {
				return nil, err
			}
			should = append(should, f...)
		case attributeValueTypeConversationSource:
			f := filterFromConversationSources(cvse.conversationSources, documentType, hasPinnedCluster)
			should = append(should, f)
		case attributeValueTypeConversationChannel:
			f := filterFromConversationChannels(cvse.conversationChannels, documentType, hasPinnedCluster)
			should = append(should, f)
		case attributeValueTypeConversationLanguageCode:
			f := filterFromLanguageCodes(cvse.conversationLanguageCodes)
			should = append(should, f)
		case attributeValueTypeConversationNumHoldsRange:
			for _, rangeValues := range cvse.conversationNumHoldsRanges {
				f, err := filterFromConversationNumHoldsRanges(ctx, rangeValues, documentType)
				if err != nil {
					return nil, err
				}
				should = append(should, f)
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "cresta entity %v not supported as filter", cvse.attributeValueType)
		}
	case *dashboardpb.ChartValueSet_EmailAttachmentsValue:
		ea := valSet.GetEmailAttachmentsValue().GetValue()
		var hasEmailAttachments *[]bool
		switch ea {
		case dashboardpb.EmailAttachments_HAS_EMAIL_ATTACHMENTS:
			hasEmailAttachments = &[]bool{true}
		case dashboardpb.EmailAttachments_HAS_NO_EMAIL_ATTACHMENTS:
			hasEmailAttachments = &[]bool{false}
		case dashboardpb.EmailAttachments_EMAIL_ATTACHMENTS_UNSPECIFIED:
			// No filter; ignore.
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported email attachments filter value: %v", ea)
		}
		if hasEmailAttachments != nil {
			convoFilter := elasticsearch.ConvoFilter{
				Terms: &elasticsearch.TermsFilter{HasEmailAttachments: hasEmailAttachments},
			}
			if documentType == elasticsearch.DocumentTypeMessage {
				should = append(should, msgFilterFromConvoFilter(convoFilter))
			} else {
				should = append(should, convoFilter)
			}
		}
	case *dashboardpb.ChartValueSet_RangeValues:
		if len(valSet.GetRangeValues().RangeValues) != 1 {
			return nil, status.Errorf(codes.InvalidArgument, "only support a single range in range filter")
		}
		var fromTime, toTime *timestamppb.Timestamp
		rangeVal := valSet.GetRangeValues().RangeValues[0]
		switch rangeVal.FromValue.Value.(type) {
		case *dashboardpb.ChartAttributeValue_Timestamp:
			fromTime, toTime = rangeVal.FromValue.GetTimestamp(), rangeVal.ToValue.GetTimestamp()
			if fromTime == nil || toTime == nil {
				return nil, status.Errorf(codes.InvalidArgument, "expect time range in range filter")
			}
		case *dashboardpb.ChartAttributeValue_RelativeTimestamp:
			var err error
			fromTime, toTime, err = sharedanalytics.ConvertRelativeDaysToAbsoluteTimestamp(rangeVal.FromValue.GetRelativeTimestamp(), rangeVal.ToValue.GetRelativeTimestamp(), metadata)
			if err != nil {
				return nil, err
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "expect time range in range filter")
		}
		should = append(should, TimeRangeFilter{
			Range: &elasticsearch.RangeFilter{
				ConversationStartedAt: &elasticsearch.TimeRange{
					// timestamps from timestamppb package is always in UTC.
					TimeZone: "+00:00",
					GTE:      fromTime.AsTime().Format(time.RFC3339),
					LT:       toTime.AsTime().Format(time.RFC3339),
				},
			},
		})
	default:
		return nil, status.Errorf(codes.InvalidArgument, "only support subcategories in ChartValueSet for now")
	}

	if len(should) == 0 && len(mustNot) == 0 {
		return nil, nil
	}

	result := &ChartValueSetFilter{
		Bool: &ChartValueSetFilterBool{},
	}
	if len(should) > 0 {
		result.Bool.Should = &should
		result.Bool.MinimumShouldMatch = 1
	}
	if len(mustNot) > 0 {
		result.Bool.MustNot = &mustNot
	}
	return result, nil
}

// chartFiltersHaveAdvancedTimeRule reports whether any filter carries a day-of-week/time-of-day rule,
// which requires ES runtime mappings.
func chartFiltersHaveAdvancedTimeRule(filters []*dashboardpb.ChartDataFilter) bool {
	return extractTimeRangeRule(filters) != nil
}

func extractTimeRangeRule(filters []*dashboardpb.ChartDataFilter) *commonpb.TimeRangeRule {
	for _, f := range filters {
		if f.GetValueSet() == nil || f.GetValueSet().GetSingleValues() == nil {
			continue
		}
		for _, val := range f.GetValueSet().GetSingleValues().GetAttributeValues() {
			if trf := val.GetTimeRangeFilter(); trf != nil {
				rule := trf.GetRule()
				if rule != nil && (len(rule.GetDaysOfWeek()) > 0 || len(rule.GetTimeOfDayRanges()) > 0) {
					return rule
				}
			}
		}
	}
	return nil
}

func filterESRespTimeBuckets(esResp *ChartESResponse, groupBys []*dashboardpb.ChartGroupBy, rule *commonpb.TimeRangeRule, timezone string, tr *timeRange) error {
	for i, gb := range groupBys {
		byTime := gb.GetByTime()
		if byTime == nil {
			continue
		}
		switch i {
		case 0:
			node, err := traverseToDesiredAggInESResp(esResp.Aggregations, multiBucketAggType)
			if err != nil {
				return err
			}
			if node.Buckets != nil {
				*node.Buckets = filterESResponseBucketsByTimeRule(*node.Buckets, byTime.Interval, rule, timezone, tr)
			}

		case 1:
			outerNode, err := traverseToDesiredAggInESResp(esResp.Aggregations, multiBucketAggType)
			if err != nil {
				return err
			}
			if outerNode.Buckets != nil {
				for j := range *outerNode.Buckets {
					innerNode, err := traverseToDesiredAggInESResp(&(*outerNode.Buckets)[j], multiBucketAggType)
					if err != nil {
						return err
					}
					if innerNode.Buckets != nil {
						*innerNode.Buckets = filterESResponseBucketsByTimeRule(*innerNode.Buckets, byTime.Interval, rule, timezone, tr)
					}
				}
			}
		}
		break
	}
	return nil
}

func filterESResponseBucketsByTimeRule(
	buckets []ESRespSingleBucket,
	interval dashboardpb.ChartGroupBy_Time_Interval,
	rule *commonpb.TimeRangeRule,
	timezone string,
	tr *timeRange,
) []ESRespSingleBucket {
	if len(buckets) == 0 {
		return buckets
	}
	// WEEK/MONTH always clip out-of-range boundary buckets; DAY/HOUR only filter under a rule.
	if interval == dashboardpb.ChartGroupBy_Time_WEEK || interval == dashboardpb.ChartGroupBy_Time_MONTH {
		return clipOutOfRangeBoundaryBuckets(buckets, rule, timezone, tr)
	}
	if rule == nil {
		return buckets
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return buckets
	}

	allowedDays := map[int32]bool{}
	for _, d := range rule.GetDaysOfWeek() {
		allowedDays[int32(d)] = true
	}
	mergedRanges := mergeTimeOfDayRanges(rule.GetTimeOfDayRanges())

	filtered := make([]ESRespSingleBucket, 0, len(buckets))
	for _, b := range buckets {
		keyTime, ok := bucketKeyTime(b)
		if !ok {
			filtered = append(filtered, b)
			continue
		}
		bucketTime := keyTime.In(loc)

		if len(allowedDays) > 0 && !allowedDays[isoWeekday(bucketTime)] {
			continue
		}

		if interval == dashboardpb.ChartGroupBy_Time_HOUR && len(mergedRanges) > 0 {
			bucketMinute := minuteOfDay(bucketTime)
			inRange := false
			for _, r := range mergedRanges {
				startOK := !r.hasStart || bucketMinute >= r.startMinute
				endOK := !r.hasEnd || bucketMinute < r.endMinute
				if startOK && endOK {
					inRange = true
					break
				}
			}
			if !inRange {
				continue
			}
		}

		filtered = append(filtered, b)
	}
	return filtered
}

// clipOutOfRangeBoundaryBuckets drops WEEK/MONTH buckets that cannot hold a document in [from, to).
//
// min_doc_count:0 + extended_bounds materialize empty boundary buckets outside the requested range: a
// leading week floored below `from` onto the histogram's week grid, and a trailing month created at
// the exclusive `to`. A boundary bucket that does overlap the range can still be guaranteed empty when
// a day-of-week / time-of-day rule excludes every instant in the overlapping slice.
//
// Both checks work off the bucket's real instant window, never off a local calendar day derived from
// the key. A key already is the exact bucket start in whatever timezone the date_histogram ran in, and
// that timezone is a fixed offset resolved independently of `timezone` (see convertGroupByTime: an
// offset snapshot of metadata.time_zone_id, or an offset inferred from the clock time of `from`).
// Re-truncating a key to midnight in a different frame slides the window by up to a full day and drops
// populated buckets (INSI-2281).
//
// date_histogram with min_doc_count:0 emits contiguous ascending buckets, so the next key is this
// bucket's end. The last kept bucket always ends at or after `to`, so `to` bounds it.
func clipOutOfRangeBoundaryBuckets(
	buckets []ESRespSingleBucket,
	rule *commonpb.TimeRangeRule,
	timezone string,
	tr *timeRange,
) []ESRespSingleBucket {
	if tr == nil || tr.from == nil || tr.to == nil {
		return buckets
	}
	from, to := tr.from.AsTime(), tr.to.AsTime()

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// fail-open: an unresolvable timezone only disables the rule check; the range clip below needs no zone.
		rule, loc = nil, time.UTC
	}
	allowedDays := set.New[int32]()
	for _, d := range rule.GetDaysOfWeek() {
		allowedDays.Add(int32(d))
	}
	mergedRanges := mergeTimeOfDayRanges(rule.GetTimeOfDayRanges())

	filtered := make([]ESRespSingleBucket, 0, len(buckets))
	for i, b := range buckets {
		bucketStart, ok := bucketKeyTime(b)
		if !ok {
			filtered = append(filtered, b)
			continue
		}
		bucketEnd := to
		if i+1 < len(buckets) {
			if nextStart, ok := bucketKeyTime(buckets[i+1]); ok {
				bucketEnd = nextStart
			}
		}

		windowStart, windowEnd := latestTime(bucketStart, from), earliestTime(bucketEnd, to)
		if !windowStart.Before(windowEnd) {
			continue
		}
		if !windowCanMatchTimeRangeRule(windowStart, windowEnd, loc, allowedDays, mergedRanges) {
			continue
		}
		filtered = append(filtered, b)
	}
	return filtered
}

// windowCanMatchTimeRangeRule reports whether any instant in [start, end) can satisfy the rule.
// Calendar days and clock minutes are read in loc, the IANA zone the truncated_day_of_week and
// truncated_minute_of_day runtime mappings use, so this agrees with what ES filtered on.
func windowCanMatchTimeRangeRule(
	start, end time.Time,
	loc *time.Location,
	allowedDays set.Set[int32],
	mergedRanges []minuteOfDayRange,
) bool {
	if allowedDays.Len() == 0 && len(mergedRanges) == 0 {
		return true
	}

	localStart := start.In(loc)
	day := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, loc)
	for ; day.Before(end); day = day.AddDate(0, 0, 1) {
		nextDay := day.AddDate(0, 0, 1)
		if allowedDays.Len() > 0 && !allowedDays.Has(isoWeekday(day)) {
			continue
		}
		if len(mergedRanges) == 0 {
			return true
		}

		// Clock-minute slice of this day the window actually covers.
		sliceStart, sliceEnd := latestTime(day, start), earliestTime(nextDay, end)
		if !sliceStart.Before(sliceEnd) {
			continue
		}
		startMinute := minuteOfDay(sliceStart.In(loc))
		endMinute := minutesPerDay
		if sliceEnd.Before(nextDay) {
			// ES's truncated_minute_of_day floors each doc to its clock minute, so a slice ending
			// mid-minute still holds docs ES assigns to that minute: ceil the exclusive end up.
			sliceEndLocal := sliceEnd.In(loc)
			endMinute = minuteOfDay(sliceEndLocal)
			if sliceEndLocal.Second() != 0 || sliceEndLocal.Nanosecond() != 0 {
				endMinute++
			}
		}
		if anyTimeOfDayRangeOverlaps(mergedRanges, startMinute, endMinute) {
			return true
		}
	}
	return false
}

// anyTimeOfDayRangeOverlaps reports whether any merged rule range intersects [startMinute, endMinute).
// An open-ended range extends to the corresponding edge of the day.
func anyTimeOfDayRangeOverlaps(mergedRanges []minuteOfDayRange, startMinute, endMinute int) bool {
	for _, r := range mergedRanges {
		ruleStart, ruleEnd := 0, minutesPerDay
		if r.hasStart {
			ruleStart = r.startMinute
		}
		if r.hasEnd {
			ruleEnd = r.endMinute
		}
		if startMinute < ruleEnd && ruleStart < endMinute {
			return true
		}
	}
	return false
}

const minutesPerDay = 24 * 60

func minuteOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

// isoWeekday returns the ISO-8601 weekday number (Monday=1 … Sunday=7) that ES's
// truncated_day_of_week runtime mapping emits.
func isoWeekday(t time.Time) int32 {
	if t.Weekday() == time.Sunday {
		return 7
	}
	return int32(t.Weekday())
}

func earliestTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func latestTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// bucketKeyTime reads an ES date_histogram bucket key (epoch millis, decoded from JSON as float64).
// A non-numeric key fails open so callers keep the bucket.
func bucketKeyTime(b ESRespSingleBucket) (time.Time, bool) {
	keyMillis, ok := b.Key.(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.UnixMilli(int64(keyMillis)).UTC(), true
}

func filterFromAdvancedTimeFilter(ctx context.Context, advancedTimeFilters []*commonpb.TimeRangeFilter, metadata *analyticspb.Metadata) ([]any, error) {
	var results []any
	for _, tf := range advancedTimeFilters {
		f, err := buildSingleAdvancedTimeFilter(tf, metadata)
		if err != nil {
			return nil, err
		}
		if f != nil {
			results = append(results, f)
		}
	}
	return results, nil
}

// buildSingleAdvancedTimeFilter converts one TimeRangeFilter proto into an ES filter (any).
// When only the outer time range is present, returns a simple range filter on conversation_started_at.
// When a rule (day-of-week / time-of-day) is also present, wraps the range and the function_score
// rule inside a bool.filter array so they are ANDed together.
// For message document type, the result is wrapped in has_parent to filter on conversation.
func buildSingleAdvancedTimeFilter(tf *commonpb.TimeRangeFilter, metadata *analyticspb.Metadata) (any, error) {
	if tf.GetOuterTimeRange() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "outer_time_range is required in TimeRangeFilter")
	}

	fromTime, toTime, err := resolveOuterTimeRange(tf.GetOuterTimeRange(), metadata)
	if err != nil {
		return nil, err
	}

	rangeFilter := TimeRangeFilter{
		Range: &elasticsearch.RangeFilter{
			ConversationStartedAt: &elasticsearch.TimeRange{
				TimeZone: "+00:00",
				GTE:      fromTime.AsTime().Format(time.RFC3339),
				LT:       toTime.AsTime().Format(time.RFC3339),
			},
		},
	}

	rule := tf.GetRule()
	hasRule := rule != nil && (len(rule.GetDaysOfWeek()) > 0 || len(rule.GetTimeOfDayRanges()) > 0)

	if !hasRule {
		// No rule: simple range filter on conversation_started_at.
		return rangeFilter, nil
	}

	// Has rule: combine range + function_score in bool.filter (ANDed).
	ruleFilters, err := filterFromTimeRangeRule(rule)
	if err != nil {
		return nil, err
	}

	ffs := []any{map[string]any{"range": rangeFilter.Range}}
	ffs = append(ffs, ruleFilters...)

	// This correctly ANDs the outer time range with the day/time rule.
	return map[string]any{
		"bool": map[string]any{
			"filter": ffs,
		},
	}, nil
}

// resolveOuterTimeRange converts a TimeRange (absolute or relative) to absolute timestamps.
func resolveOuterTimeRange(timeRange *commonpb.TimeRange, metadata *analyticspb.Metadata) (*timestamppb.Timestamp, *timestamppb.Timestamp, error) {
	switch tr := timeRange.GetValue().(type) {
	case *commonpb.TimeRange_AbsoluteTimeRange_:
		abs := tr.AbsoluteTimeRange
		if abs.GetStartTime() == nil || abs.GetEndTime() == nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "both start_time and end_time are required in absolute_time_range")
		}
		return abs.GetStartTime(), abs.GetEndTime(), nil
	case *commonpb.TimeRange_RelativeTimeRange_:
		rel := tr.RelativeTimeRange
		if rel.GetStartTime() == nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "start_time is required in relative_time_range")
		}
		fromTime, err := convertRelativeTimeToAbsolute(rel.GetStartTime(), metadata)
		if err != nil {
			return nil, nil, err
		}
		toTime, err := convertRelativeTimeToAbsolute(rel.GetEndTime(), metadata)
		if err != nil {
			return nil, nil, err
		}
		return fromTime, toTime, nil
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported time range type %T", timeRange.GetValue())
	}
}

func convertRelativeTimeToAbsolute(rel *commonpb.RelativeTimestamp, metadata *analyticspb.Metadata) (*timestamppb.Timestamp, error) {
	now := time.Now().UTC()
	if rel.GetDurationBeforeNow() == nil {
		return timestamppb.New(now), nil
	}

	if metadata.GetTimeZoneId() != "" {
		loc, err := time.LoadLocation(metadata.GetTimeZoneId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid timezone ID %s: %v", metadata.GetTimeZoneId(), err)
		}
		now = time.Now().In(loc)
	}
	// Default base time is the end of today in the specified timezone.
	// If hours or minutes are specified in the duration, use the current time as the base time to avoid confusion.
	// For example, "0 days 3 hours before now" is more intuitive to be interpreted as "3 hours before the current time" instead of "3 hours before the start of today".
	baseTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	if rel.GetDurationBeforeNow().Hours != 0 || rel.GetDurationBeforeNow().Minutes != 0 {
		baseTime = now
	}
	ret := baseTime.AddDate(0, 0, -int(rel.GetDurationBeforeNow().Days)).
		Add(-time.Duration(rel.GetDurationBeforeNow().Hours) * time.Hour).
		Add(-time.Duration(rel.GetDurationBeforeNow().Minutes) * time.Minute).UTC()
	return timestamppb.New(ret), nil
}

// filterFromTimeRangeRule return a list of filter from day-of-week and time-of-day rules.
func filterFromTimeRangeRule(rule *commonpb.TimeRangeRule) ([]any, error) {
	var ruleFilters []any

	// Days of week filter.
	if days := rule.GetDaysOfWeek(); len(days) > 0 {
		dayValues := make([]int32, 0, len(days))
		for _, d := range days {
			dayValues = append(dayValues, int32(d))
		}
		ruleFilters = append(ruleFilters, map[string]any{
			"terms": map[string]any{
				"truncated_day_of_week": dayValues,
			},
		})
	}

	// Time of day ranges filter.
	if timeRanges := rule.GetTimeOfDayRanges(); len(timeRanges) > 0 {
		merged := mergeTimeOfDayRanges(timeRanges)
		var timeOfDayShould []any
		for _, mr := range merged {
			minuteRange := map[string]any{}
			if mr.hasStart {
				minuteRange["gte"] = mr.startMinute
			}
			if mr.hasEnd {
				minuteRange["lt"] = mr.endMinute
			}
			if len(minuteRange) > 0 {
				timeOfDayShould = append(timeOfDayShould, map[string]any{
					"range": map[string]any{
						"truncated_minute_of_day": minuteRange,
					},
				})
			}
		}
		if len(timeOfDayShould) == 1 {
			ruleFilters = append(ruleFilters, timeOfDayShould[0])
		} else if len(timeOfDayShould) > 1 {
			ruleFilters = append(ruleFilters, map[string]any{
				"bool": map[string]any{
					"should":               timeOfDayShould,
					"minimum_should_match": 1,
				},
			})
		}
	}

	return ruleFilters, nil
}

type minuteOfDayRange struct {
	startMinute int
	endMinute   int
	hasStart    bool
	hasEnd      bool
}

// mergeTimeOfDayRanges merges overlapping or adjacent time-of-day ranges to reduce
// the number of ES range clauses. For example, [0,30) and [30,60) become [0,60).
func mergeTimeOfDayRanges(timeRanges []*commonpb.TimeOfDayRange) []minuteOfDayRange {
	var ranges []minuteOfDayRange
	for _, tr := range timeRanges {
		r := minuteOfDayRange{}
		if start := tr.GetStartTimeOfDay(); start != nil {
			r.hasStart = true
			r.startMinute = int(start.GetHours())*60 + int(start.GetMinutes())
		}
		if end := tr.GetEndTimeOfDay(); end != nil {
			r.hasEnd = true
			r.endMinute = int(end.GetHours())*60 + int(end.GetMinutes())
		}
		if r.hasStart || r.hasEnd {
			ranges = append(ranges, r)
		}
	}
	if len(ranges) <= 1 {
		return ranges
	}

	// Ranges without a start bound go first (unbounded lower); among the rest, sort by start.
	slices.SortFunc(ranges, func(a, b minuteOfDayRange) int {
		if !a.hasStart && b.hasStart {
			return -1
		}
		if a.hasStart && !b.hasStart {
			return 1
		}
		return a.startMinute - b.startMinute
	})

	merged := []minuteOfDayRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if canMergeRanges(*last, r) {
			if r.hasEnd {
				if !last.hasEnd || r.endMinute > last.endMinute {
					last.hasEnd = r.hasEnd
					last.endMinute = r.endMinute
				}
			} else {
				last.hasEnd = false
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// canMergeRanges returns true if two sorted ranges overlap or are adjacent (e.g. [0,30) and [30,60)).
func canMergeRanges(a, b minuteOfDayRange) bool {
	if !a.hasEnd {
		return true
	}
	if !b.hasStart {
		return true
	}
	return a.endMinute >= b.startMinute
}

func filterFromAgentNames(ctx context.Context, agentNames []string, documentType elasticsearch.DocumentType, agentFilterAlwaysOnConversation bool) (any, error) {
	agentUserIDs := []string{}
	for _, userName := range agentNames {
		name, err := userpb.ParseUserName(userName)
		if err != nil {
			return nil, err
		}
		agentUserIDs = append(agentUserIDs, name.UserID)
	}
	return filterFromAgentUserIDs(agentUserIDs, documentType, agentFilterAlwaysOnConversation), nil
}

func filterFromUsecaseNames(ctx context.Context, usecaseNames []string) (any, error) {
	usecaseIDs, err := parseUsercaseNames(usecaseNames)
	if err != nil {
		return nil, err
	}
	// MsgFilter vs ConvoFilter does not matter, since it just serves
	// as a wrapper of TermsFilter.
	return elasticsearch.MsgFilter{
		Terms: &elasticsearch.TermsFilter{
			UsecaseIDs: &usecaseIDs,
		},
	}, nil
}

func filterFromConversationSources(sources []int32, documentType elasticsearch.DocumentType, hasPinnedCluster bool) any {
	termsFilter := elasticsearch.TermsFilter{
		ConversationSources: &sources,
	}
	// conversation_source is not on message_group documents. When hasPinnedCluster is true
	// the target includes message_group, so filter on the parent conversation document instead.
	if documentType == elasticsearch.DocumentTypeMessage && hasPinnedCluster {
		return elasticsearch.MsgFilter{
			HasParent: &elasticsearch.HasParent{
				ParentType: string(elasticsearch.DocumentTypeConversation),
				Query: elasticsearch.ConvoQuery{
					Bool: elasticsearch.ConvoBool{
						Filter: &[]elasticsearch.ConvoFilter{
							{Terms: &termsFilter},
						},
					},
				},
			},
		}
	}
	return elasticsearch.MsgFilter{
		Terms: &termsFilter,
	}
}

func filterFromConversationChannels(channels []int32, documentType elasticsearch.DocumentType, hasPinnedCluster bool) any {
	termsFilter := elasticsearch.TermsFilter{
		ConversationChannels: &channels,
	}
	// conversation_channel is not on message_group documents. When hasPinnedCluster is true
	// the target includes message_group, so filter on the parent conversation document instead.
	if documentType == elasticsearch.DocumentTypeMessage && hasPinnedCluster {
		return elasticsearch.MsgFilter{
			HasParent: &elasticsearch.HasParent{
				ParentType: string(elasticsearch.DocumentTypeConversation),
				Query: elasticsearch.ConvoQuery{
					Bool: elasticsearch.ConvoBool{
						Filter: &[]elasticsearch.ConvoFilter{
							{Terms: &termsFilter},
						},
					},
				},
			},
		}
	}
	return elasticsearch.MsgFilter{
		Terms: &termsFilter,
	}
}

func filterFromLanguageCodes(languageCodes []string) any {
	// effective_language is denormalized onto both conversation and message documents,
	// so no parent traversal is needed (unlike ConversationChannel/ConversationSource).
	return elasticsearch.MsgFilter{
		Terms: &elasticsearch.TermsFilter{
			LangCodes: &languageCodes,
		},
	}
}

func hasConversationSourceValues(valSet *dashboardpb.ChartValueSet) bool {
	if valSet == nil {
		return false
	}
	for _, val := range valSet.GetSingleValues().GetAttributeValues() {
		if _, ok := val.Value.(*dashboardpb.ChartAttributeValue_ConversationSource); ok {
			return true
		}
	}
	return false
}

func hasConversationSourceINFilter(chartDataFilters []*dashboardpb.ChartDataFilter) bool {
	for _, cf := range chartDataFilters {
		if cf.Operator == dashboardpb.ChartDataFilter_IN && hasConversationSourceValues(cf.ValueSet) {
			return true
		}
	}
	return false
}

func filterFromScorecardSubmitterNames(ctx context.Context, scorecardSubmitterNames []string, documentType elasticsearch.DocumentType) (any, error) {
	nested, err := nestedFromScorecardSubmitterNames(ctx, scorecardSubmitterNames)
	if err != nil {
		return nil, err
	}

	filterOnConversation := elasticsearch.ConvoFilter{
		Nested: nested,
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}
	return ret, nil
}

func filterFromScorecardStatusFilters(ctx context.Context, scorecardStatusFilters []*dashboardpb.ScorecardStatusFilter, documentType elasticsearch.DocumentType) ([]any, error) {
	var allScorecardStatuses []dashboardpb.ScorecardStatus
	var allAppealScorecardStatuses []dashboardpb.AppealScorecardStatus
	var allCalibrationScorecardStatuses []dashboardpb.CalibrationScorecardStatus
	var allPublishedScorecardStatuses []dashboardpb.PublishedScorecardStatus

	for _, filter := range scorecardStatusFilters {
		allScorecardStatuses = append(allScorecardStatuses, filter.GetScorecardStatuses()...)
		allAppealScorecardStatuses = append(allAppealScorecardStatuses, filter.GetAppealScorecardStatuses()...)
		allCalibrationScorecardStatuses = append(allCalibrationScorecardStatuses, filter.GetCalibrationScorecardStatuses()...)
		allPublishedScorecardStatuses = append(allPublishedScorecardStatuses, filter.GetPublishedScorecardStatuses()...)
	}

	var allScorecardFilters []*elasticsearch.ConvoFilter
	if len(allScorecardStatuses) > 0 {
		normalScorecardConvoFilter := elasticsearch.ConvertNormalScorecardStatusesToConvoFilter(convertToAnalyticsScorecardStatus(allScorecardStatuses), []string{})
		allScorecardFilters = append(allScorecardFilters, normalScorecardConvoFilter)
	}

	if len(allAppealScorecardStatuses) > 0 {
		appealScorecardConvoFilter := elasticsearch.ConvertAppealScorecardStatusesToConvoFilter(convertToAnalyticsAppealScorecardStatus(allAppealScorecardStatuses), []string{})
		allScorecardFilters = append(allScorecardFilters, appealScorecardConvoFilter)
	}

	if len(allCalibrationScorecardStatuses) > 0 {
		calibrationScorecardConvoFilter := elasticsearch.ConvertCalibrationScorecardStatusesToConvoFilter(convertToAnalyticsCalibrationScorecardStatus(allCalibrationScorecardStatuses), []string{})
		allScorecardFilters = append(allScorecardFilters, calibrationScorecardConvoFilter)
	}

	if len(allPublishedScorecardStatuses) > 0 {
		publishedScorecardConvoFilter := elasticsearch.ConvertPublishedScorecardStatusesToConvoFilter(convertToAnalyticsPublishedScorecardStatus(allPublishedScorecardStatuses), []string{})
		allScorecardFilters = append(allScorecardFilters, publishedScorecardConvoFilter)
	}

	var allConvoFilters []any
	for _, filter := range allScorecardFilters {
		if filter == nil {
			continue
		}
		if documentType == elasticsearch.DocumentTypeMessage {
			allConvoFilters = append(allConvoFilters, msgFilterFromConvoFilter(*filter))
		} else {
			allConvoFilters = append(allConvoFilters, *filter)
		}
	}
	return allConvoFilters, nil
}

func (a AnalyticsServiceImpl) filterFromGroupNames(ctx context.Context, customerID, profileID string, groupNames []string, documentType elasticsearch.DocumentType, agentFilterAlwaysOnConversation bool) (any, error) {
	if len(groupNames) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "0 groups provided")
	}
	// TODO: Merge agentUserIDs from agentNames and group_names ACROSS filters.
	agentUserIDs, err := a.groupMemberUserIDs(ctx, customerID, profileID, groupNames)
	if err != nil {
		return nil, err
	}
	return filterFromAgentUserIDs(agentUserIDs, documentType, agentFilterAlwaysOnConversation), nil
}

// filterFromAgentUserIDs matches no document when agentUserIDs is empty.
// When agentFilterAlwaysOnConversation is set on a message-level query, agent_user_id is evaluated on
// the parent conversation document (agent is a conversation attribute, not denormalized onto messages).
func filterFromAgentUserIDs(agentUserIDs []string, documentType elasticsearch.DocumentType, agentFilterAlwaysOnConversation bool) any {
	termsFilter := elasticsearch.TermsFilter{
		AgentUserIDs: &agentUserIDs,
	}
	if agentFilterAlwaysOnConversation && documentType == elasticsearch.DocumentTypeMessage {
		return msgFilterFromConvoFilter(elasticsearch.ConvoFilter{Terms: &termsFilter})
	}
	return elasticsearch.MsgFilter{
		Terms: &termsFilter,
	}
}

// groupMemberUserIDs returns the user IDs of the direct and indirect members of any of groupNames,
// in a single user service call.
func (a AnalyticsServiceImpl) groupMemberUserIDs(ctx context.Context, customerID, profileID string, groupNames []string) ([]string, error) {
	groups := []*userpb.Group{}
	for _, groupName := range groupNames {
		groups = append(groups, &userpb.Group{
			Name: groupName,
		})
	}
	users, err := shared.ParseFiltersToUsers(
		ctx,
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		customerID,
		profileID,
		groups,
		false, // activeUserFilter
		false, // includeDirectGroupMembershipsOnly
		false, // listAgentOnly
	)
	if err != nil {
		return nil, fmt.Errorf("listing members of groups %v: %w", groupNames, err)
	}
	return fn.MapE(users, func(user *userpb.User) (string, error) {
		name, err := userpb.ParseUserName(user.Name)
		if err != nil {
			return "", err
		}
		return name.UserID, nil
	})
}

// createMatchNoneFilter creates a match_none query for the given document type
func createMatchNoneFilter(documentType elasticsearch.DocumentType) any {
	matchNone := &elasticsearch.MatchNone{}
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		return elasticsearch.MsgFilter{
			MatchNone: matchNone,
		}
	case elasticsearch.DocumentTypeConversation:
		return elasticsearch.ConvoFilterBoolFilter{
			MatchNone: matchNone,
		}
	default:
		return nil
	}
}

func (a AnalyticsServiceImpl) filterFromTopicBools(ctx context.Context, topicBools []*elasticsearch.TopicBool, documentType elasticsearch.DocumentType, isConvoFilterFromMsgEntity bool) (any, error) {
	if len(topicBools) == 0 {
		return createMatchNoneFilter(documentType), nil
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		if isConvoFilterFromMsgEntity {
			convoFilter := elasticsearch.ConvertTopicBoolsToConvoFilter(topicBools)
			return msgFilterFromConvoFilter(convoFilter), nil
		}
		should := []elasticsearch.MsgFilter{}
		for _, t := range topicBools {
			if t == nil {
				continue
			}
			should = append(should, elasticsearch.MsgFilter{Bool: t})
		}
		if len(should) == 1 {
			ret = should[0]
		} else {
			ret = elasticsearch.InnerMsgQuery{
				Bool: &elasticsearch.InnerMsgBool{
					Should:             &should,
					MinimumShouldMatch: 1,
				},
			}
		}
	case elasticsearch.DocumentTypeConversation:
		ret = elasticsearch.ConvertTopicBoolsToConvoFilter(topicBools)
	}
	return ret, nil
}

func filterFromClusters(ctx context.Context, clusters []*dashboardpb.ClusterForChart, documentType elasticsearch.DocumentType, v3ClusterClosures map[string][]string) (any, error) {
	nested, err := nestedFromClusters(ctx, clusters, v3ClusterClosures)
	if err != nil {
		return nil, err
	}
	filterOnMsg := elasticsearch.MsgFilter{
		Nested: nested,
	}
	// TDv1/v2 cluster annotations live on the "message_group" child document, whereas Topic Discovery
	// V3 category annotations are message-anchored on the "message" child document. A single has_child
	// filter targets exactly one child document type, so a value set mixing both is rejected.
	clusterChildType, err := tdClusterDocumentType(clusters)
	if err != nil {
		return nil, err
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		// Topic Discovery V3 category annotations are message-anchored, so a message-level query can
		// filter on the moment_annotations nested field directly (same shape as sentiments/emotions).
		// TDv1/v2 cluster annotations live on the message_group child, which a message-level query
		// cannot reach, so filtering by them here would silently match zero messages; reject instead.
		if clusterChildType != elasticsearch.DocumentTypeMessage {
			return nil, status.Errorf(codes.InvalidArgument, "do not support occurrence count when filtering by non-TDv3 clusters, because moment annotations are on document message_group")
		}
		ret = filterOnMsg
	case elasticsearch.DocumentTypeConversation:
		ret = convoFilterFromMsgFilter(filterOnMsg, clusterChildType)
	}
	return ret, nil
}

// tdClusterDocumentType returns the child document type a homogeneous slice of chart clusters
// filters against. Topic Discovery V3 categories are message-anchored ("message"), while TDv1/v2
// clusters are message_group-anchored. Because a single has_child filter targets exactly one child
// document type, a value set mixing V3 and non-V3 clusters is rejected with InvalidArgument.
func tdClusterDocumentType(clusters []*dashboardpb.ClusterForChart) (elasticsearch.DocumentType, error) {
	v3Count := 0
	for _, cluster := range clusters {
		if cluster.GetDetailedType() == dashboardpb.ClusterForChart_CONVERSATION_REASON_V3 {
			v3Count++
		}
	}
	switch v3Count {
	case 0:
		return elasticsearch.DocumentTypeMessageGroup, nil
	case len(clusters):
		return elasticsearch.DocumentTypeMessage, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "cannot mix Topic Discovery V3 and non-V3 clusters in a single chart value set")
	}
}

func nestedFromClusters(ctx context.Context, clusters []*dashboardpb.ClusterForChart, v3ClusterClosures map[string][]string) (*elasticsearch.Nested, error) {
	momentIDs := []string{}
	seen := set.New[string]()
	appendMomentID := func(clusterMomentName string) error {
		// Accept either a full moment resource name or a bare moment ID.
		momentID := clusterMomentName
		if parsed, err := momentpb.ParseMomentName(clusterMomentName); err == nil && parsed.GetMomentID() != "" {
			momentID = parsed.GetMomentID()
		}
		if momentID == "" {
			return status.Errorf(codes.InvalidArgument, "empty cluster moment name or id")
		}
		if seen.Add(momentID) {
			momentIDs = append(momentIDs, momentID)
		}
		return nil
	}
	for _, c := range clusters {
		// A Topic Discovery V3 cluster expands to its full subtree closure (self + every descendant,
		// leaf AND non-leaf) so a filter or a single group-by bucket matches every conversation
		// classified anywhere beneath the category. Non-V3 clusters (and V3 clusters with no resolved
		// closure) contribute only their own moment id.
		if closure, ok := lookupV3ClusterClosure(v3ClusterClosures, c.GetMomentName()); ok {
			for _, closureMomentName := range closure {
				if err := appendMomentID(closureMomentName); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := appendMomentID(c.MomentName); err != nil {
			return nil, err
		}
	}
	ret := &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{
					{
						Terms: &elasticsearch.NestedTermsFilter{
							MomentTemplateIDs: &momentIDs,
						},
					},
				},
			},
		},
	}
	return ret, nil
}

func filterFromOperaBlocks(ctx context.Context, operablocks []*dashboardpb.OperaBlockForChart, documentType elasticsearch.DocumentType) (any, error) {
	nested, err := nestedFromOperablocks(ctx, operablocks)
	if err != nil {
		return nil, err
	}
	filterOnMsg := elasticsearch.MsgFilter{
		Nested: nested,
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = filterOnMsg
	case elasticsearch.DocumentTypeConversation:
		ret = convoFilterFromMsgFilter(filterOnMsg, elasticsearch.DocumentTypeMessage)
	}
	return ret, nil
}

func nestedFromOperablocks(ctx context.Context, operaBlocks []*dashboardpb.OperaBlockForChart) (*elasticsearch.Nested, error) {
	momentIDs := []string{}
	for _, ob := range operaBlocks {
		momentName, err := momentpb.ParseMomentName(ob.MomentName)
		if err != nil {
			return nil, err
		}
		momentIDs = append(momentIDs, momentName.MomentID)
	}
	ret := &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{
					{
						Terms: &elasticsearch.NestedTermsFilter{
							MomentTemplateIDs: &momentIDs,
						},
					},
				},
			},
		},
	}
	return ret, nil
}

func filterFromAgentAssistance(ctx context.Context, agentAssistance *dashboardpb.AgentAssistanceForChart, documentType elasticsearch.DocumentType) (any, error) {
	var ret any
	switch agentAssistance.Value.(type) {
	case *dashboardpb.AgentAssistanceForChart_BehaiviorHint, *dashboardpb.AgentAssistanceForChart_KnowledgeBaseHint, *dashboardpb.AgentAssistanceForChart_GuidedWorkflowHint:
		hint, err := convertAgentAssistanceToESHint(ctx, agentAssistance)
		if err != nil {
			return nil, err
		}
		hintFilter, err := elasticsearch.ConvertHintToFilter(hint)
		if err != nil {
			return nil, err
		}
		switch documentType {
		case elasticsearch.DocumentTypeMessage:
			msgFilters := elasticsearch.HintFilterForMessage(*hintFilter)
			ret = elasticsearch.OuterQuery{
				Bool: elasticsearch.MsgOuterBool{
					Filter: &msgFilters,
				},
			}
		case elasticsearch.DocumentTypeConversation:
			convoFilters := elasticsearch.HintFilterForConvo(*hintFilter)
			ret = elasticsearch.ConvoFilterBoolFilter{
				Bool: &elasticsearch.ConvoFilterBoolFilterBool{
					Should:             &convoFilters,
					MinimumShouldMatch: proto.Int32(int32(len(convoFilters))),
				},
			}
		}
	}
	return ret, nil
}

// TODO(zhucaiyi): read action with policy resource name.
func convertAgentAssistanceToESHint(ctx context.Context, aa *dashboardpb.AgentAssistanceForChart) (elasticsearch.HintParam, error) {
	if len(aa.AdherenceTypes) > 1 {
		return elasticsearch.HintParam{}, status.Errorf(codes.InvalidArgument, "only support multiple adherence types")
	}
	ret := elasticsearch.HintParam{
		Action: &actionpb.Action{
			Name: "customers/customer-id/profiles/profile-id/actions/dummy-action-id",
			Type: actionpb.Action_HINT,
		},
	}
	if len(aa.AdherenceTypes) > 0 {
		if aa.AdherenceTypes[0] == dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_USED {
			ret.EngagmentType = elasticsearch.HintEngaged
			// TODO(zhucaiyi): add enum NOT_USED.
		}
	}
	return ret, nil
}

func (a AnalyticsServiceImpl) filterFromMetadatas(ctx context.Context, metadatas []*dashboardpb.MetadataForChart, documentType elasticsearch.DocumentType, profileConfig *config.ProfileConfig) (any, any, error) {
	if len(metadatas) != 1 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "only support a single metadata in filter")
	}
	md := metadatas[0]
	momentName, err := momentpb.ParseMomentName(md.MomentName)
	if err != nil {
		return nil, nil, err
	}

	// Fast path: use the denormalized conversation-level field metadata_id_value_pairs
	// (populated in converter.buildMetadataIDValuePairsFromMomentAnnotation) instead of a
	// nested query on moment_annotations. Exact-match only, so fall back to the nested
	// path when numeric ranges are involved.
	if isMetadataFilteringConversationLevelMetadataEnabled(profileConfig) && len(md.NumericValueRanges) == 0 {
		return a.filterFromMetadatasUsingIDValuePairs(ctx, md, momentName.MomentID, documentType)
	}

	nestedFilter, includeMissingValuesFilter, err := a.nestedFromMetadatas(ctx, metadatas)
	if err != nil {
		return nil, nil, err
	}

	if nestedFilter == nil {
		if includeMissingValuesFilter == nil {
			return nil, nil, status.Errorf(codes.Internal, "expecting at least non nil missing values filter if nested filter is nil")
		}
		return nil, includeMissingValuesFilter, nil
	}

	filterOnConversation := elasticsearch.ConvoFilter{
		Nested: nestedFilter,
	}

	var ret any
	enableReadingDenormalizedConversationMetadataFromMessageDoc, denormalizedMetadataMomentTemplateIds := profileConfig.GetFeatures().GetConversation().GetEnableReadingDenormalizedConversationMetadataFromMessageDoc(), profileConfig.GetFeatures().GetConversation().GetDenormalizedConversationMetadataMomentTemplateIds()
	if enableReadingDenormalizedConversationMetadataFromMessageDoc && slices.Contains(denormalizedMetadataMomentTemplateIds, momentName.MomentID) {
		ret = filterOnConversation
	} else {
		switch documentType {
		case elasticsearch.DocumentTypeMessage:
			ret = msgFilterFromConvoFilter(filterOnConversation)
		case elasticsearch.DocumentTypeConversation:
			ret = filterOnConversation
		}
	}

	// Don't remove this block, returning includeMissingValuesFilter is typed nil, which
	// is not equivalent to untyped nil any(nil).
	if includeMissingValuesFilter == nil {
		return ret, nil, nil
	}

	return ret, includeMissingValuesFilter, nil
}

// filterFromMetadatasUsingIDValuePairs builds a conversation-level metadata filter that matches
// against the denormalized metadata_id_value_pairs field. Concrete values (strings, numbers,
// booleans) produce a terms filter; the "missing value" sentinel keeps using the nested
// must_not approach since there is no exact-match representation for "any value of moment X".
func (a AnalyticsServiceImpl) filterFromMetadatasUsingIDValuePairs(ctx context.Context, md *dashboardpb.MetadataForChart, momentTemplateID string, documentType elasticsearch.DocumentType) (any, any, error) {
	nestedFilter, includeMissingValuesFilter, err := a.nestedFromMetadatas(ctx, []*dashboardpb.MetadataForChart{md})
	if err != nil {
		return nil, nil, err
	}

	pairs := buildMetadataIDValuePairs(momentTemplateID, md)
	if len(pairs) == 0 {
		// No concrete values to look up in metadata_id_value_pairs. Preserve the
		// exists-only semantics (match any conversation that has this metadata
		// annotated at all) by falling back to the nested filter from
		// nestedFromMetadatas.
		if nestedFilter == nil {
			// Don't remove this block, returning includeMissingValuesFilter is typed nil, which
			// is not equivalent to untyped nil any(nil).
			if includeMissingValuesFilter == nil {
				return nil, nil, status.Errorf(codes.Internal, "expecting at least non nil missing values filter if nested filter is nil")
			}
			return nil, includeMissingValuesFilter, nil
		}

		filterOnConversation := elasticsearch.ConvoFilter{
			Nested: nestedFilter,
		}
		var ret any
		switch documentType {
		case elasticsearch.DocumentTypeMessage:
			ret = msgFilterFromConvoFilter(filterOnConversation)
		case elasticsearch.DocumentTypeConversation:
			ret = filterOnConversation
		}

		if includeMissingValuesFilter == nil {
			return ret, nil, nil
		}
		return ret, includeMissingValuesFilter, nil
	}

	filterOnConversation := elasticsearch.ConvoFilter{
		Terms: &elasticsearch.TermsFilter{
			MetadataIDValuePairs: &pairs,
		},
	}

	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		// metadata_id_value_pairs lives only on the conversation document, so for a
		// message-doc query we have to cross the join boundary via has_parent.
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}

	if includeMissingValuesFilter == nil {
		return ret, nil, nil
	}
	return ret, includeMissingValuesFilter, nil
}

// buildMetadataIDValuePairs formats concrete metadata values into the "<moment_template_id>_<value>"
// strings that the ES ingestion pipeline writes into the conversation-level metadata_id_value_pairs
// field. The formatting must match converter.buildMetadataIDValuePairsFromMomentAnnotation.
func buildMetadataIDValuePairs(momentTemplateID string, md *dashboardpb.MetadataForChart) []string {
	pairs := make([]string, 0, len(md.Values))
	for _, val := range md.Values {
		var s string
		switch v := val.Value.(type) {
		case *dashboardpb.MetadataForChart_MetadataValue_StringValue:
			// The "missing value" sentinel is handled by a separate must_not filter; skip here.
			if v.StringValue == "#NO_VALUE_METADATA#" {
				continue
			}
			s = v.StringValue
		case *dashboardpb.MetadataForChart_MetadataValue_NumberValue:
			s = fmt.Sprintf("%v", v.NumberValue)
		case *dashboardpb.MetadataForChart_MetadataValue_BooleanValue:
			s = strconv.FormatBool(v.BooleanValue)
		default:
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s_%s", momentTemplateID, s))
	}
	return pairs
}

// Returns an array of nested filters for the metadata.
func (a AnalyticsServiceImpl) nestedFromMetadatas(ctx context.Context, metadatas []*dashboardpb.MetadataForChart) (*elasticsearch.Nested, *elasticsearch.NestedFilter, error) {
	if len(metadatas) > 1 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "only support a single metadata in filter")
	}
	md := metadatas[0]
	momentName, err := momentpb.ParseMomentName(md.MomentName)
	if err != nil {
		return nil, nil, err
	}
	var (
		numberValues []float32
		stringValues []string
		boolValues   []bool
	)
	includeMissingValues := false
	for _, val := range md.Values {
		switch val.Value.(type) {
		case *dashboardpb.MetadataForChart_MetadataValue_BooleanValue:
			boolValues = append(boolValues, val.GetBooleanValue())
		case *dashboardpb.MetadataForChart_MetadataValue_StringValue:
			stringValue := val.GetStringValue()
			if stringValue == "#NO_VALUE_METADATA#" {
				includeMissingValues = true
			} else {
				stringValues = append(stringValues, stringValue)
			}
		case *dashboardpb.MetadataForChart_MetadataValue_NumberValue:
			numberValues = append(numberValues, val.GetNumberValue())
		}
	}

	nestedFilters := []elasticsearch.NestedFilter{{
		Terms: &elasticsearch.NestedTermsFilter{
			MomentTemplateIDs: &[]string{momentName.MomentID},
		},
	}}
	isCrestaCLO := isCrestaModelledCLO(md)
	// To validate metadata values are of the same type.
	valueTypeCount := 0
	if len(numberValues) > 0 {
		if isCrestaCLO {
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						OutcomeNumberValueExplicitMapping: &numberValues,
					},
				})
			} else {
				nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						OutcomeNumberPayloadValues: &numberValues,
					},
				})
			}
		} else {
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						MetadataNumberValueExplicitMapping: &numberValues,
					},
				})
			} else {
				nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						MetadataNumberPayloadValues: &numberValues,
					},
				})
			}
		}
		valueTypeCount += 1
	}
	if len(stringValues) > 0 {
		if isCrestaCLO {
			nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					OutcomeStringPayloadValues: &stringValues,
				},
			})
		} else {
			nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					MetadataStringPayloadValues: &stringValues,
				},
			})
		}
		valueTypeCount += 1
	}
	if len(boolValues) > 0 {
		if isCrestaCLO {
			nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					OutcomeBooleanPayloadValues: &boolValues,
				},
			})
		} else {
			nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					MetadataBooleanPayloadValues: &boolValues,
				},
			})
		}
		valueTypeCount += 1
	}

	if valueTypeCount > 1 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "expect exact single type of value for metadata %v", momentName.MomentID)
	}

	convertNumericValueRange := func(numericRange *dashboardpb.MetadataForChart_NumericRange) elasticsearch.FloatRange {
		floatFrange := elasticsearch.FloatRange{}
		if lb := numericRange.LowerBound; lb != nil {
			if lb.IsExclusive {
				floatFrange.GT = proto.Float64(float64(lb.Value))
			} else {
				floatFrange.GTE = proto.Float64(float64(lb.Value))
			}
		}
		if ub := numericRange.UpperBound; ub != nil {
			if ub.IsExclusive {
				floatFrange.LT = proto.Float64(float64(ub.Value))
			} else {
				floatFrange.LTE = proto.Float64(float64(ub.Value))
			}
		}
		return floatFrange
	}

	if len(md.NumericValueRanges) == 1 {
		rangeFilter := convertNumericValueRange(md.NumericValueRanges[0])
		var f elasticsearch.NestedFilter
		if isCrestaCLO {
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				f = elasticsearch.NestedFilter{
					Range: &elasticsearch.RangeFilter{
						OutcomeNumberValueExplicitMapping: &rangeFilter,
					},
				}
			} else {
				f = elasticsearch.NestedFilter{
					Range: &elasticsearch.RangeFilter{
						OutcomeNumberValue: &rangeFilter,
					},
				}
			}
		} else {
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				f = elasticsearch.NestedFilter{
					Range: &elasticsearch.RangeFilter{
						MetadataNumberValueExplicitMapping: &rangeFilter,
					},
				}
			} else {
				f = elasticsearch.NestedFilter{
					Range: &elasticsearch.RangeFilter{
						MetadataNumberValue: &rangeFilter,
					},
				}
			}
		}
		nestedFilters = append(nestedFilters, f)
	}

	if len(md.NumericValueRanges) > 1 {
		ffs := []elasticsearch.NestedFilter{}
		for _, rg := range md.NumericValueRanges {
			rangeFilter := convertNumericValueRange(rg)
			var f elasticsearch.NestedFilter
			if isCrestaCLO {
				if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
					f = elasticsearch.NestedFilter{
						Range: &elasticsearch.RangeFilter{
							OutcomeNumberValueExplicitMapping: &rangeFilter,
						},
					}
				} else {
					f = elasticsearch.NestedFilter{
						Range: &elasticsearch.RangeFilter{
							OutcomeNumberValue: &rangeFilter,
						},
					}
				}
			} else {
				if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
					f = elasticsearch.NestedFilter{
						Range: &elasticsearch.RangeFilter{
							MetadataNumberValueExplicitMapping: &rangeFilter,
						},
					}
				} else {
					f = elasticsearch.NestedFilter{
						Range: &elasticsearch.RangeFilter{
							MetadataNumberValue: &rangeFilter,
						},
					}
				}
			}
			ffs = append(ffs, f)
		}
		nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Should:             &ffs,
				MinimumShouldMatch: *proto.Int32(1),
			},
		})
	}

	normalFilter := &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &nestedFilters,
			},
		},
	}

	var filterForMissingValues *elasticsearch.NestedFilter
	if includeMissingValues {
		if valueTypeCount == 0 && len(md.NumericValueRanges) == 0 {
			normalFilter = nil
		}
		filterForMissingValues = &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				MustNot: &[]elasticsearch.NestedFilter{
					{
						Nested: &elasticsearch.Nested{
							Path: elasticsearch.MomentAnnotationsPath,
							Query: &elasticsearch.NestedFilter{
								Bool: &elasticsearch.NestedFilterBool{
									Filter: &[]elasticsearch.NestedFilter{
										{
											Terms: &elasticsearch.NestedTermsFilter{
												MomentTemplateIDs: &[]string{momentName.MomentID},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	return normalFilter, filterForMissingValues, nil
}

// Determine whether the metadata is cresta-modelled CLO (which is outcome moment template).
func isCrestaModelledCLO(md *dashboardpb.MetadataForChart) bool {
	return md != nil && md.OutcomeConfig != nil && md.OutcomeConfig.Source == dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED
}

// filterFromLlmAnnotations builds a conversation-level filter for the given LLM annotation.
// LLM annotations are always conversation-level: for a message-doc query we cross the join
// boundary via has_parent. The second return value mirrors the metadata helpers' missing-value
// filter, but is always nil since LLM annotations do not support the missing-value sentinel.
func (a AnalyticsServiceImpl) filterFromLlmAnnotations(ctx context.Context, llmAnnotations []*dashboardpb.LlmAnnotationForChart, documentType elasticsearch.DocumentType) (any, any, error) {
	if len(llmAnnotations) != 1 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "only support a single llm annotation in filter")
	}

	nestedFilter, err := a.nestedFromLlmAnnotations(ctx, llmAnnotations)
	if err != nil {
		return nil, nil, err
	}

	if nestedFilter == nil {
		return nil, nil, nil
	}

	filterOnConversation := elasticsearch.ConvoFilter{
		Nested: nestedFilter,
	}

	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}

	return ret, nil, nil
}

// nestedFromLlmAnnotations returns a nested filter for the LLM annotation.
func (a AnalyticsServiceImpl) nestedFromLlmAnnotations(ctx context.Context, llmAnnotations []*dashboardpb.LlmAnnotationForChart) (*elasticsearch.Nested, error) {
	if len(llmAnnotations) > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "only support a single llm annotation in filter")
	}
	md := llmAnnotations[0]
	momentName, err := momentpb.ParseMomentName(md.MomentName)
	if err != nil {
		return nil, err
	}
	var (
		numberValues []float32
		stringValues []string
		boolValues   []bool
	)
	for _, val := range md.Values {
		switch val.Value.(type) {
		case *dashboardpb.LlmAnnotationForChart_AnnotationValue_BooleanValue:
			boolValues = append(boolValues, val.GetBooleanValue())
		case *dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue:
			stringValues = append(stringValues, val.GetStringValue())
		case *dashboardpb.LlmAnnotationForChart_AnnotationValue_NumberValue:
			numberValues = append(numberValues, val.GetNumberValue())
		}
	}

	nestedFilters := []elasticsearch.NestedFilter{{
		Terms: &elasticsearch.NestedTermsFilter{
			MomentTemplateIDs: &[]string{momentName.MomentID},
		},
	}}
	// To validate llm annotation values are of the same type.
	valueTypeCount := 0
	if len(numberValues) > 0 {
		if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
			nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					LlmAnnotationNumberValueExplicitMapping: &numberValues,
				},
			})
		} else {
			nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					LlmAnnotationNumberPayloadValues: &numberValues,
				},
			})
		}
		valueTypeCount += 1
	}
	if len(stringValues) > 0 {
		nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
			Terms: &elasticsearch.NestedTermsFilter{
				LlmAnnotationStringPayloadValues: &stringValues,
			},
		})
		valueTypeCount += 1
	}
	if len(boolValues) > 0 {
		nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
			Terms: &elasticsearch.NestedTermsFilter{
				LlmAnnotationBooleanPayloadValues: &boolValues,
			},
		})
		valueTypeCount += 1
	}

	if valueTypeCount > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "expect exact single type of value for llm annotation %v", momentName.MomentID)
	}

	convertNumericValueRange := func(numericRange *dashboardpb.MetadataForChart_NumericRange) elasticsearch.FloatRange {
		floatFrange := elasticsearch.FloatRange{}
		if lb := numericRange.LowerBound; lb != nil {
			if lb.IsExclusive {
				floatFrange.GT = proto.Float64(float64(lb.Value))
			} else {
				floatFrange.GTE = proto.Float64(float64(lb.Value))
			}
		}
		if ub := numericRange.UpperBound; ub != nil {
			if ub.IsExclusive {
				floatFrange.LT = proto.Float64(float64(ub.Value))
			} else {
				floatFrange.LTE = proto.Float64(float64(ub.Value))
			}
		}
		return floatFrange
	}

	if len(md.NumericValueRanges) == 1 {
		rangeFilter := convertNumericValueRange(md.NumericValueRanges[0])
		var f elasticsearch.NestedFilter
		if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
			f = elasticsearch.NestedFilter{
				Range: &elasticsearch.RangeFilter{
					LlmAnnotationNumberValueExplicitMapping: &rangeFilter,
				},
			}
		} else {
			f = elasticsearch.NestedFilter{
				Range: &elasticsearch.RangeFilter{
					LlmAnnotationNumberValue: &rangeFilter,
				},
			}
		}
		nestedFilters = append(nestedFilters, f)
	}

	if len(md.NumericValueRanges) > 1 {
		ffs := []elasticsearch.NestedFilter{}
		for _, rg := range md.NumericValueRanges {
			rangeFilter := convertNumericValueRange(rg)
			var f elasticsearch.NestedFilter
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				f = elasticsearch.NestedFilter{
					Range: &elasticsearch.RangeFilter{
						LlmAnnotationNumberValueExplicitMapping: &rangeFilter,
					},
				}
			} else {
				f = elasticsearch.NestedFilter{
					Range: &elasticsearch.RangeFilter{
						LlmAnnotationNumberValue: &rangeFilter,
					},
				}
			}
			ffs = append(ffs, f)
		}
		nestedFilters = append(nestedFilters, elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Should:             &ffs,
				MinimumShouldMatch: *proto.Int32(1),
			},
		})
	}

	normalFilter := &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &nestedFilters,
			},
		},
	}

	return normalFilter, nil
}

func filterFromScorecardTemplates(ctx context.Context, templates []*dashboardpb.ScorecardTemplateForChart, documentType elasticsearch.DocumentType) (any, error) {
	nested, err := nestedFromScorecardTemplate(ctx, templates)
	if err != nil {
		return nil, err
	}

	filterOnConversation := elasticsearch.ConvoFilter{
		Nested: nested,
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}
	return ret, nil
}

func msgFilterFromConvoFilter(filterOnConvo elasticsearch.ConvoFilter) elasticsearch.MsgFilter {
	return elasticsearch.MsgFilter{
		HasParent: &elasticsearch.HasParent{
			ParentType: string(elasticsearch.DocumentTypeConversation),
			Query: elasticsearch.ConvoQuery{
				Bool: elasticsearch.ConvoBool{
					Filter: &[]elasticsearch.ConvoFilter{
						filterOnConvo,
					},
				},
			},
		},
	}
}

func convoFilterFromMsgFilter(filterOnMsg elasticsearch.MsgFilter, msgType elasticsearch.DocumentType) elasticsearch.ConvoFilter {
	return elasticsearch.ConvoFilter{
		HasChild: &elasticsearch.HasChild{
			Type: string(msgType),
			Query: elasticsearch.InnerMsgQuery{
				Bool: &elasticsearch.InnerMsgBool{
					Filter: &[]elasticsearch.MsgFilter{filterOnMsg},
				},
			},
		},
	}
}

// filterFromScorecardCriterions matches conversations having a scorecard score for any of the given
// criteria.
//
// Criterion ids are only unique *within* a scorecard template, so each criterion id has to be
// matched against a scorecard of the template it was selected from. Criteria are therefore grouped
// by template and every group contributes a `scorecards` nested clause holding both the template
// term and its criterion terms, which forces ES to satisfy them on the same scorecard document.
// Filtering on criterion id alone made a criterion also match the same-id criterion on every other
// template the conversation was scored against, inflating counts (INSI-4445).
func filterFromScorecardCriterions(ctx context.Context, criterions []*dashboardpb.ScorecardCriterionForChart, documentType elasticsearch.DocumentType) (any, error) {
	if len(criterions) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "found empty list of criterions")
	}

	type criterionGroup struct {
		templateName string
		criterionIDs []string
		scoreValues  []*dashboardpb.ScorecardCriterionForChart_Score
	}
	// Preserve input order while de-duplicating to keep ES request deterministic. The dedup key is
	// (template, criterion id), because the same criterion id under two templates is two distinct
	// criteria.
	groupIndexByTemplate := map[string]int{}
	groups := []*criterionGroup{}
	seenCriterions := map[string]struct{}{}
	for _, crit := range criterions {
		groupIndex, ok := groupIndexByTemplate[crit.ScorecardTemplateName]
		if !ok {
			groups = append(groups, &criterionGroup{templateName: crit.ScorecardTemplateName})
			groupIndex = len(groups) - 1
			groupIndexByTemplate[crit.ScorecardTemplateName] = groupIndex
		}
		group := groups[groupIndex]
		if _, seen := seenCriterions[crit.ScorecardTemplateName+"\x00"+crit.CriterionId]; !seen {
			seenCriterions[crit.ScorecardTemplateName+"\x00"+crit.CriterionId] = struct{}{}
			group.criterionIDs = append(group.criterionIDs, crit.CriterionId)
		}
		if len(crit.Scores) > 0 {
			group.scoreValues = crit.Scores
		}
	}

	nestedPerTemplate := make([]*elasticsearch.Nested, 0, len(groups))
	for _, group := range groups {
		// Scores are attached to the group as a whole, so they can only be associated with an
		// unambiguous criterion.
		if len(group.scoreValues) > 0 && len(group.criterionIDs) > 1 {
			return nil, status.Errorf(codes.InvalidArgument, "expecting single criterion when criterion values are provided")
		}
		nested, err := nestedFromScorecardCriterionGroup(group.templateName, group.criterionIDs, group.scoreValues)
		if err != nil {
			return nil, err
		}
		nestedPerTemplate = append(nestedPerTemplate, nested)
	}

	var filterOnConversation elasticsearch.ConvoFilter
	if len(nestedPerTemplate) == 1 {
		filterOnConversation = elasticsearch.ConvoFilter{Nested: nestedPerTemplate[0]}
	} else {
		// A flat terms filter over several criteria means "any of them", so several template groups
		// are OR-ed rather than AND-ed.
		shoulds := make([]elasticsearch.ConvoFilterBoolFilter, 0, len(nestedPerTemplate))
		for _, nested := range nestedPerTemplate {
			shoulds = append(shoulds, elasticsearch.ConvoFilterBoolFilter{Nested: nested})
		}
		filterOnConversation = elasticsearch.ConvoFilter{
			Bool: &elasticsearch.ConvoFilterBool{
				Should:             &shoulds,
				MinimumShouldMatch: 1,
			},
		}
	}

	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}
	return ret, nil
}

// nestedFromScorecardCriterionGroup builds the `scorecards` nested clause for the criteria selected
// from a single scorecard template. The template term and the criterion terms deliberately live in
// the same nested query so that ES has to match them against the same scorecard.
func nestedFromScorecardCriterionGroup(
	scorecardTemplateName string,
	criterionIDs []string,
	scoreValues []*dashboardpb.ScorecardCriterionForChart_Score,
) (*elasticsearch.Nested, error) {
	f := elasticsearch.NestedFilter{
		Nested: &elasticsearch.Nested{
			Path: elasticsearch.ScorecardScoresPath,
		},
	}
	if len(scoreValues) == 0 {
		f.Nested.Query = &elasticsearch.NestedFilter{
			Terms: &elasticsearch.NestedTermsFilter{
				ScorecardScoresCriterionIDs: &criterionIDs,
			},
		}
	} else {
		ff := []elasticsearch.NestedFilter{}

		if len(criterionIDs) > 0 && criterionIDs[0] != "" {
			ff = append(ff, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					ScorecardScoresCriterionIDs: &criterionIDs,
				},
			})
		}

		numericValues := []float32{}
		notApplicable := false
		for _, val := range scoreValues {
			if val.GetNotApplicable() {
				notApplicable = val.GetNotApplicable()
			} else {
				numericValues = append(numericValues, val.GetNumericValue())
			}
		}
		switch {
		case len(numericValues) > 0 && notApplicable:
			ff = append(ff, elasticsearch.NestedFilter{
				Bool: &elasticsearch.NestedFilterBool{
					Should: &[]elasticsearch.NestedFilter{
						{
							Terms: &elasticsearch.NestedTermsFilter{
								ScorecardScoresNumericValues: &numericValues,
							},
						},
						{
							Terms: &elasticsearch.NestedTermsFilter{
								ScorecardScoresNotApplicable: &[]bool{true},
							},
						},
					},
				},
			})
		case len(numericValues) > 0:
			ff = append(ff, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					ScorecardScoresNumericValues: &numericValues,
				},
			})
		case notApplicable:
			ff = append(ff, elasticsearch.NestedFilter{
				Terms: &elasticsearch.NestedTermsFilter{
					ScorecardScoresNotApplicable: &[]bool{true},
				},
			})
		}
		f.Nested.Query = &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &ff,
			},
		}
	}

	scorecardFilters := []elasticsearch.NestedFilter{}
	// Charts saved before criteria carried their template cannot be scoped; matching on criterion id
	// alone keeps them rendering rather than erroring out.
	if scorecardTemplateName != "" {
		templateName, err := coachingpb.ParseScorecardTemplateName(scorecardTemplateName)
		if err != nil {
			return nil, err
		}
		scorecardFilters = append(scorecardFilters, elasticsearch.NestedFilter{
			Terms: &elasticsearch.NestedTermsFilter{
				ScorecardTemplateIDs: &[]string{templateName.ScorecardTemplateID},
			},
		})
	}
	scorecardFilters = append(scorecardFilters, f)

	return &elasticsearch.Nested{
		Path: elasticsearch.ScorecardsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &scorecardFilters,
			},
		},
	}, nil
}

func filterFromSentiments(ctx context.Context, sentiments []*dashboardpb.SentimentForChart, documentType elasticsearch.DocumentType) (any, error) {
	nested, err := nestedFromSentiments(ctx, sentiments)
	if err != nil {
		return nil, err
	}

	var ret any
	switch t := sentiments[0].Type; t {
	case dashboardpb.SentimentForChart_MESSAGE:
		filterOnMsg := elasticsearch.MsgFilter{
			Nested: nested,
		}
		switch documentType {
		case elasticsearch.DocumentTypeMessage:
			ret = filterOnMsg
		case elasticsearch.DocumentTypeConversation:
			ret = convoFilterFromMsgFilter(filterOnMsg, elasticsearch.DocumentTypeMessage)
		}
	case dashboardpb.SentimentForChart_CONVERSATION:
		filterOnConversation := elasticsearch.ConvoFilter{
			Nested: nested,
		}
		switch documentType {
		case elasticsearch.DocumentTypeMessage:
			ret = msgFilterFromConvoFilter(filterOnConversation)
		case elasticsearch.DocumentTypeConversation:
			ret = filterOnConversation
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid sentiment type %v", t)
	}
	return ret, nil
}

func nestedFromSentiments(ctx context.Context, sentiments []*dashboardpb.SentimentForChart) (*elasticsearch.Nested, error) {
	momentIDs := []string{}
	for _, ss := range sentiments {
		momentName, err := momentpb.ParseMomentName(ss.MomentName)
		if err != nil {
			return nil, err
		}
		momentIDs = append(momentIDs, momentName.MomentID)
	}
	ret := &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{
					{
						Terms: &elasticsearch.NestedTermsFilter{
							MomentTemplateIDs: &momentIDs,
						},
					},
				},
			},
		},
	}
	return ret, nil
}

func filterFromEmotions(ctx context.Context, emotions []*dashboardpb.EmotionForChart, documentType elasticsearch.DocumentType) (any, error) {
	nested, err := nestedFromEmotions(ctx, emotions)
	if err != nil {
		return nil, err
	}
	var ret any
	filterOnMsg := elasticsearch.MsgFilter{
		Nested: nested,
	}
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = filterOnMsg
	case elasticsearch.DocumentTypeConversation:
		ret = convoFilterFromMsgFilter(filterOnMsg, elasticsearch.DocumentTypeMessage)
	}
	return ret, nil
}

func nestedFromScorecardTemplate(ctx context.Context, templates []*dashboardpb.ScorecardTemplateForChart) (*elasticsearch.Nested, error) {
	templateIDs := []string{}
	for _, st := range templates {
		scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(st.ScorecardTemplateName)
		if err != nil {
			return nil, err
		}
		templateIDs = append(templateIDs, scorecardTemplateName.ScorecardTemplateID)
	}
	return &elasticsearch.Nested{
		Path: elasticsearch.ScorecardsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{
					{
						Terms: &elasticsearch.NestedTermsFilter{
							ScorecardTemplateIDs: &templateIDs,
						},
					},
				},
			},
		},
	}, nil
}

func nestedFromScorecardSubmitterNames(ctx context.Context, scorecardSubmitterNames []string) (*elasticsearch.Nested, error) {
	scorecardSubmitterUserIDs := []string{}
	for _, userName := range scorecardSubmitterNames {
		name, err := userpb.ParseUserName(userName)
		if err != nil {
			return nil, err
		}
		scorecardSubmitterUserIDs = append(scorecardSubmitterUserIDs, name.UserID)
	}
	return &elasticsearch.Nested{
		Path: elasticsearch.ScorecardsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{
					{
						Terms: &elasticsearch.NestedTermsFilter{
							SubmitterUserIDs: &scorecardSubmitterUserIDs,
						},
					},
				},
			},
		},
	}, nil
}

func nestedFromEmotions(ctx context.Context, emotions []*dashboardpb.EmotionForChart) (*elasticsearch.Nested, error) {
	momentIDs := []string{}
	for _, em := range emotions {
		momentName, err := momentpb.ParseMomentName(em.MomentName)
		if err != nil {
			return nil, err
		}
		momentIDs = append(momentIDs, momentName.MomentID)
	}
	ret := &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{
					{
						Terms: &elasticsearch.NestedTermsFilter{
							MomentTemplateIDs: &momentIDs,
						},
					},
				},
			},
		},
	}
	return ret, nil
}

func filterFromBehaviors(ctx context.Context, behaviors []*dashboardpb.BehaviorForChart, documentType elasticsearch.DocumentType) (any, error) {
	nested, err := nestedFromBehaviors(ctx, behaviors)
	if err != nil {
		return nil, err
	}
	filterOnMsg := elasticsearch.MsgFilter{
		Nested: nested,
	}

	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = filterOnMsg
	case elasticsearch.DocumentTypeConversation:
		ret = convoFilterFromMsgFilter(filterOnMsg, elasticsearch.DocumentTypeMessage)
	}
	return ret, nil
}

func nestedFromBehaviors(ctx context.Context, behaviors []*dashboardpb.BehaviorForChart) (*elasticsearch.Nested, error) {
	if len(behaviors) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "0 behaviors found")
	}
	nestedShould := []elasticsearch.NestedFilter{}

	for _, b := range behaviors {
		behaviorName, err := behaviorpb.ParseBehaviorName(b.BehaviorName)
		if err != nil {
			return nil, err
		}
		f1 := elasticsearch.NestedFilter{
			Terms: &elasticsearch.NestedTermsFilter{
				BehaviorIDs: &[]string{behaviorName.BehaviorID},
			},
		}
		convertedAdhereceTypes := convertBehaviorAdherenceTypes(b.AdherenceTypes)
		f2 := elasticsearch.NestedFilter{
			Terms: &elasticsearch.NestedTermsFilter{
				MomentAdherenceTypes: &convertedAdhereceTypes,
			},
		}
		nestedShould = append(nestedShould, elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Filter: &[]elasticsearch.NestedFilter{f1, f2},
			},
		})
	}
	if len(nestedShould) == 1 {
		return &elasticsearch.Nested{
			Path: elasticsearch.MomentAnnotationsPath,
			Query: &elasticsearch.NestedFilter{
				Bool: &elasticsearch.NestedFilterBool{
					Filter: nestedShould[0].Bool.Filter,
				},
			},
		}, nil
	}
	return &elasticsearch.Nested{
		Path: elasticsearch.MomentAnnotationsPath,
		Query: &elasticsearch.NestedFilter{
			Bool: &elasticsearch.NestedFilterBool{
				Should:             &nestedShould,
				MinimumShouldMatch: 1,
			},
		},
	}, nil
}

func filterFromConversationHandleTimeRanges(ctx context.Context, conversationHandleTimeRange *dashboardpb.ChartFloatRange, documentType elasticsearch.DocumentType) (any, error) {
	rangeFilter := convertNumericValueRange(conversationHandleTimeRange)

	filterOnConversation := elasticsearch.ConvoFilter{
		Range: &elasticsearch.RangeFilter{
			ConversationHandleTimeSec: &rangeFilter,
		},
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}
	return ret, nil
}

func filterFromConversationMessageCountRanges(ctx context.Context, conversationMessageCountRanges *dashboardpb.ChartFloatRange, documentType elasticsearch.DocumentType) (any, error) {
	rangeFilter := convertNumericValueRange(conversationMessageCountRanges)
	// Field message_count exists in both Message Document and Conversation Document.
	convoFilter := &elasticsearch.ConvoFilter{
		Range: &elasticsearch.RangeFilter{
			ConversationMessageCount: &rangeFilter,
		},
	}
	if documentType == elasticsearch.DocumentTypeMessage {
		return msgFilterFromConvoFilter(*convoFilter), nil
	}
	return convoFilter, nil
}

func (a AnalyticsServiceImpl) filterFromConversationDurationRange(ctx context.Context, convoDurationRange *dashboardpb.ChartDurationRange, documentType elasticsearch.DocumentType) (any, error) {
	floatRange := elasticsearch.FloatRange{}
	if lower := convoDurationRange.LowerBound; lower != nil {
		if lower.IsExclusive {
			floatRange.GT = proto.Float64(float64(lower.Value.Seconds))
		} else {
			floatRange.GTE = proto.Float64(float64(lower.Value.Seconds))
		}
	}
	if upper := convoDurationRange.UpperBound; upper != nil {
		if upper.IsExclusive {
			floatRange.LT = proto.Float64(float64(upper.Value.Seconds))
		} else {
			floatRange.LTE = proto.Float64(float64(upper.Value.Seconds))
		}
	}
	filterOnConversation := elasticsearch.ConvoFilter{
		Range: &elasticsearch.RangeFilter{
			ConversationDurationSec: &floatRange,
		},
	}

	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		// A message-doc query is used either when the simplified no-join path is selected for a
		// conversation-count chart (gated by the flag in isMessageOrConvoEntity) OR inherently for
		// any message-level metric chart regardless of the flag. The latter means message docs can
		// be queried before the denormalized conversation_duration_secs field is backfilled, so the
		// choice between filtering on the denormalized message-doc field and crossing the join via
		// has_parent must itself be flag-gated: with the flag off (default, pre-backfill) fall back
		// to has_parent against the conversation doc's pre-existing field so charts still return
		// correct results; with the flag on (post-backfill) filter on the denormalized field and
		// avoid the parent-join entirely.
		useDenormalizedDurationField, err := a.featureFlagClient.BoolValue(ctx, "enable_reading_denormalized_conversation_duration_from_message_doc", false)
		if err != nil {
			// fail-open: flag evaluation errors fall back to the default (off) -> has_parent.
			a.logger.Warnf(ctx, "feature flag fallback used for enable_reading_denormalized_conversation_duration_from_message_doc: %v", err)
		}
		if useDenormalizedDurationField {
			ret = elasticsearch.MsgFilter{
				Range: &elasticsearch.RangeFilter{
					ConversationDurationSec: &floatRange,
				},
			}
		} else {
			ret = msgFilterFromConvoFilter(filterOnConversation)
		}
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}
	return ret, nil
}

func filterFromConversationNumHoldsRanges(ctx context.Context, numHoldsRange *dashboardpb.ChartFloatRange, documentType elasticsearch.DocumentType) (any, error) {
	rangeFilter := convertNumericValueRange(numHoldsRange)

	filterOnConversation := elasticsearch.ConvoFilter{
		Range: &elasticsearch.RangeFilter{
			NumHolds: &rangeFilter,
		},
	}
	var ret any
	switch documentType {
	case elasticsearch.DocumentTypeMessage:
		ret = msgFilterFromConvoFilter(filterOnConversation)
	case elasticsearch.DocumentTypeConversation:
		ret = filterOnConversation
	}
	return ret, nil
}

var behvaiorAdherenceTypeMapping = map[dashboardpb.BehaviorForChart_AdherenceType]int{
	dashboardpb.BehaviorForChart_ADHERENCE_TYPE_SHOULD_DO_X:            int(dbmomentpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X),
	dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_DO_X:               int(dbmomentpb.AdherenceType_ADHERENCE_TYPE_DID_DO_X),
	dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_NOT_DO_X:           int(dbmomentpb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X),
	dashboardpb.BehaviorForChart_ADHERENCE_TYPE_NO_OPPORTUNITY_TO_DO_X: int(dbmomentpb.AdherenceType_ADHERENCE_TYPE_NO_OPPORTUNITY_TO_DO_X),
}

func convertBehaviorAdherenceTypes(adherenceTypes []dashboardpb.BehaviorForChart_AdherenceType) []int {
	ret := []int{}
	for _, adherenceType := range adherenceTypes {
		ret = append(ret, behvaiorAdherenceTypeMapping[adherenceType])
	}
	return ret
}

type TimeRangeFilter struct {
	Range *elasticsearch.RangeFilter `json:"range,omitempty"`
}

func (a AnalyticsServiceImpl) getChartTimeRangeFilter(ctx context.Context, chartDataConfig *dashboardpb.ChartDataConfig, metadata *analyticspb.Metadata) (*timeRange, error) {
	if chartDataConfig == nil {
		return nil, nil
	}

	hasTimeRangeFilter := false
	hasRangeValues := false
	timeRangeFilterCount := 0
	for _, f := range chartDataConfig.Filters {
		for _, val := range f.GetValueSet().GetSingleValues().GetAttributeValues() {
			if trf := val.GetTimeRangeFilter(); trf != nil {
				hasTimeRangeFilter = true
				timeRangeFilterCount++
			}
		}
		for _, r := range f.GetValueSet().GetRangeValues().GetRangeValues() {
			switch r.GetFromValue().GetValue().(type) {
			case *dashboardpb.ChartAttributeValue_Timestamp, *dashboardpb.ChartAttributeValue_RelativeTimestamp:
				hasRangeValues = true
			}
		}
	}
	if hasTimeRangeFilter && hasRangeValues {
		return nil, status.Errorf(codes.InvalidArgument, "cannot provide both TimeRangeFilter and RangeValues in chart filters")
	}
	if timeRangeFilterCount > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "only a single TimeRangeFilter is allowed in chart filters, found %d", timeRangeFilterCount)
	}

	for _, f := range chartDataConfig.Filters {
		for _, val := range f.GetValueSet().GetSingleValues().GetAttributeValues() {
			if trf := val.GetTimeRangeFilter(); trf != nil && trf.GetOuterTimeRange() != nil {
				fromTime, toTime, err := resolveOuterTimeRange(trf.GetOuterTimeRange(), metadata)
				if err != nil {
					return nil, err
				}
				if fromTime != nil || toTime != nil {
					return &timeRange{
						from: fromTime,
						to:   toTime,
					}, nil
				}
			}
		}
	}
	for _, f := range chartDataConfig.Filters {
		if f.ValueSet.GetRangeValues() == nil {
			continue
		}
		for _, r := range f.ValueSet.GetRangeValues().RangeValues {
			var fromTime, toTime *timestamppb.Timestamp
			switch r.FromValue.Value.(type) {
			case *dashboardpb.ChartAttributeValue_Timestamp:
				fromTime = r.FromValue.GetTimestamp()
				toTime = r.ToValue.GetTimestamp()
			case *dashboardpb.ChartAttributeValue_RelativeTimestamp:
				var err error
				fromTime, toTime, err = sharedanalytics.ConvertRelativeDaysToAbsoluteTimestamp(r.FromValue.GetRelativeTimestamp(), r.ToValue.GetRelativeTimestamp(), metadata)
				if err != nil {
					return nil, err
				}
			}
			if fromTime != nil || toTime != nil {
				return &timeRange{
					from: fromTime,
					to:   toTime,
				}, nil
			}
		}
	}
	return nil, nil
}

func convertNumericValueRange(inputRange *dashboardpb.ChartFloatRange) elasticsearch.FloatRange {
	outputRange := elasticsearch.FloatRange{}
	if lb := inputRange.LowerBound; lb != nil {
		if lb.IsExclusive {
			outputRange.GT = proto.Float64(float64(lb.Value))
		} else {
			outputRange.GTE = proto.Float64(float64(lb.Value))
		}
	}
	if ub := inputRange.UpperBound; ub != nil {
		if ub.IsExclusive {
			outputRange.LT = proto.Float64(float64(ub.Value))
		} else {
			outputRange.LTE = proto.Float64(float64(ub.Value))
		}
	}
	return outputRange
}

func convertToAnalyticsScorecardStatus(statuses []dashboardpb.ScorecardStatus) []analyticspb.Attribute_ScorecardStatus {
	ret := []analyticspb.Attribute_ScorecardStatus{}
	for _, status := range statuses {
		switch status {
		case dashboardpb.ScorecardStatus_SCORECARD_STATUS_UNSPECIFIED:
			ret = append(ret, analyticspb.Attribute_SCORECARD_STATUS_UNSPECIFIED)
		case dashboardpb.ScorecardStatus_SCORECARD_STATUS_MANUALLY_SUBMITTED:
			ret = append(ret, analyticspb.Attribute_MANUALLY_SUBMITTED)
		case dashboardpb.ScorecardStatus_SCORECARD_STATUS_DRAFT:
			ret = append(ret, analyticspb.Attribute_DRAFT)
		case dashboardpb.ScorecardStatus_SCORECARD_STATUS_AUTO:
			ret = append(ret, analyticspb.Attribute_AUTO)
		}
	}
	return ret
}

func convertToAnalyticsCalibrationScorecardStatus(statuses []dashboardpb.CalibrationScorecardStatus) []analyticspb.Attribute_ScorecardStatus {
	ret := []analyticspb.Attribute_ScorecardStatus{}
	for _, status := range statuses {
		switch status {
		case dashboardpb.CalibrationScorecardStatus_CALIBRATION_SCORECARD_STATUS_UNSPECIFIED:
			ret = append(ret, analyticspb.Attribute_SCORECARD_STATUS_UNSPECIFIED)
		case dashboardpb.CalibrationScorecardStatus_CALIBRATION_SCORECARD_STATUS_DRAFT:
			ret = append(ret, analyticspb.Attribute_DRAFT)
		case dashboardpb.CalibrationScorecardStatus_CALIBRATION_SCORECARD_STATUS_SUBMITTED:
			ret = append(ret, analyticspb.Attribute_MANUALLY_SUBMITTED)
		}
	}
	return ret
}

func convertToAnalyticsAppealScorecardStatus(statuses []dashboardpb.AppealScorecardStatus) []analyticspb.Attribute_AppealScorecardStatus {
	ret := []analyticspb.Attribute_AppealScorecardStatus{}
	for _, status := range statuses {
		switch status {
		case dashboardpb.AppealScorecardStatus_APPEAL_SCORECARD_STATUS_UNSPECIFIED:
			ret = append(ret, analyticspb.Attribute_APPEAL_SCORECARD_STATUS_UNSPECIFIED)
		case dashboardpb.AppealScorecardStatus_APPEAL_SCORECARD_STATUS_REQUESTED:
			ret = append(ret, analyticspb.Attribute_APPEAL_SCORECARD_STATUS_REQUESTED)
		case dashboardpb.AppealScorecardStatus_APPEAL_SCORECARD_STATUS_RESOLVED:
			ret = append(ret, analyticspb.Attribute_APPEAL_SCORECARD_STATUS_RESOLVED)
		}
	}
	return ret
}

func convertToAnalyticsPublishedScorecardStatus(statuses []dashboardpb.PublishedScorecardStatus) []analyticspb.Attribute_PublishedScorecardStatus {
	ret := []analyticspb.Attribute_PublishedScorecardStatus{}
	for _, status := range statuses {
		switch status {
		case dashboardpb.PublishedScorecardStatus_PUBLISHED_SCORECARD_STATUS_UNSPECIFIED:
			// skip
		case dashboardpb.PublishedScorecardStatus_PUBLISHED_SCORECARD_STATUS_PUBLISHED:
			ret = append(ret, analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_PUBLISHED)
		case dashboardpb.PublishedScorecardStatus_PUBLISHED_SCORECARD_STATUS_NOT_PUBLISHED:
			ret = append(ret, analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_NOT_PUBLISHED)
		}
	}
	return ret
}

func filterCalibrationScorecardsForConversationCount() elasticsearch.NestedFilter {
	return elasticsearch.NestedFilter{
		Bool: &elasticsearch.NestedFilterBool{
			MustNot: &[]elasticsearch.NestedFilter{
				{
					Exists: &elasticsearch.Field{
						Field: elasticsearch.ScorecardsPath + ".calibrated_scorecard_id",
					},
				},
			},
		},
	}
}

func isMetadataFilteringConversationLevelMetadataEnabled(profileConfig *config.ProfileConfig) bool {
	return profileConfig != nil &&
		profileConfig.GetFeatures() != nil &&
		profileConfig.GetFeatures().GetInsights() != nil &&
		profileConfig.GetFeatures().GetInsights().GetEnableMetadataFilteringConversationLevelMetadata()
}
