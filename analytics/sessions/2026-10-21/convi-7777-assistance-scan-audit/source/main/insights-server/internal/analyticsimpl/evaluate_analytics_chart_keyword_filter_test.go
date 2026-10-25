package analyticsimpl

import (
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/go-servers/shared/protobuf"
	"github.com/cresta/go-servers/shared/testing/assert"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type chartKeywordFilterTestSuite struct {
	suite.Suite
	service AnalyticsServiceImpl
}

func TestChartKeywordFilter(test *testing.T) {
	suite.Run(test, new(chartKeywordFilterTestSuite))
}

func (testSuite *chartKeywordFilterTestSuite) SetupTest() {
	testSuite.service = AnalyticsServiceImpl{logger: logger.GetLogger()}
}

func chartKeywordExpression(operator momentpb.KeywordMomentData_Operator, phrase string) *momentpb.KeywordMomentData_KeywordExpression {
	return &momentpb.KeywordMomentData_KeywordExpression{
		Operator: operator,
		Operands: []*momentpb.KeywordMomentData_Operand{
			{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: phrase}},
		},
	}
}

func chartKeywordDataFilter(language string, expression *momentpb.KeywordMomentData_KeywordExpression) *dashboardpb.ChartDataFilter {
	return &dashboardpb.ChartDataFilter{
		Operator: dashboardpb.ChartDataFilter_IN,
		ValueSet: &dashboardpb.ChartValueSet{Values: &dashboardpb.ChartValueSet_KeywordFilterValue{
			KeywordFilterValue: &dashboardpb.KeywordFilterForChart{LanguageCode: language, KeywordExpression: expression},
		}},
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestMomentFidelity() {
	for _, language := range []string{"", "en", "en-US", "es-ES", "es-419", "zh-Hant-TW", "ta-IN", "EN-us", "de-CH-1901", "en-US-u-ca-gregory", "x-custom", "qaa", "i-klingon", consts.AllLanguages} {
		for _, operator := range []momentpb.KeywordMomentData_Operator{momentpb.KeywordMomentData_OR, momentpb.KeywordMomentData_AND} {
			testSuite.Run(language+"_"+operator.String(), func() {
				// Given
				expression := chartKeywordExpression(operator, "alpha beta")
				expression.Operands = append(expression.Operands, &momentpb.KeywordMomentData_Operand{
					Operand: &momentpb.KeywordMomentData_Operand_Expression{
						Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "beneficios"),
					},
				})
				keywordFilter := &dashboardpb.KeywordFilterForChart{LanguageCode: language, KeywordExpression: expression}
				expected := &analyticspb.AnalyticsChartFilter{
					InsightsTopic: &analyticspb.InsightsTopic{
						IncludedKeywordMoment: &momentpb.Moment{
							Type:         momentpb.Moment_KEYWORD,
							AppTypes:     []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
							MomentSource: momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
							LanguageCode: language,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{KeywordExpression: expression},
								},
							},
						},
					},
				}
				original := proto.Clone(keywordFilter)

				// When
				actual, err := keywordFilterToAnalyticsChartFilter(keywordFilter)

				// Then
				testSuite.Require().NoError(err)
				assert.CmpEquals(testSuite.T(), expected, actual)
				assert.CmpEquals(testSuite.T(), original, keywordFilter)
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestQueryPopulation() {
	for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
		for _, testCase := range []struct {
			name          string
			savedPhrases  []string
			requestPhrase string
		}{
			{name: "LegacyUnfiltered"},
			{name: "PersistedOnly", savedPhrases: []string{"alpha beta"}},
			{name: "AdhocOnly", requestPhrase: "pharmacy"},
			{name: "IdenticalFilters", savedPhrases: []string{"alpha beta"}, requestPhrase: "alpha beta"},
			{name: "DifferentFilters", savedPhrases: []string{"alpha beta"}, requestPhrase: "pharmacy"},
			{name: "MultiplePersistedFilters", savedPhrases: []string{"alpha beta", "member benefits"}, requestPhrase: "pharmacy"},
		} {
			testSuite.Run(string(documentType)+"_"+testCase.name, func() {
				// Given
				chartDataConfig := &dashboardpb.ChartDataConfig{
					Metrics: []*dashboardpb.ChartMetric{{}},
					Filters: []*dashboardpb.ChartDataFilter{{
						Operator: dashboardpb.ChartDataFilter_IN,
						ValueSet: &dashboardpb.ChartValueSet{Values: &dashboardpb.ChartValueSet_SingleValues{
							SingleValues: &dashboardpb.ChartValueSet_Single{AttributeValues: []*dashboardpb.ChartAttributeValue{{
								Value: &dashboardpb.ChartAttributeValue_UserNameValue{UserNameValue: "customers/test/users/user-1"},
							}}},
						}},
					}},
				}
				var requestFilter *analyticspb.AnalyticsChartFilter
				populationPhrases := []string{}
				for _, phrase := range testCase.savedPhrases {
					chartDataConfig.Filters = append(chartDataConfig.Filters, chartKeywordDataFilter("en-US", chartKeywordExpression(momentpb.KeywordMomentData_OR, phrase)))
					populationPhrases = append(populationPhrases, phrase)
				}
				if testCase.requestPhrase != "" {
					requestFilter = &analyticspb.AnalyticsChartFilter{
						InsightsTopic: &analyticspb.InsightsTopic{
							IncludedKeywordMoment: &momentpb.Moment{
								Type:         momentpb.Moment_KEYWORD,
								AppTypes:     []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
								MomentSource: momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
								LanguageCode: "en-US",
								MomentData: &momentpb.MomentData{
									Moment: &momentpb.MomentData_KeywordMomentData{
										KeywordMomentData: &momentpb.KeywordMomentData{
											KeywordExpression: chartKeywordExpression(momentpb.KeywordMomentData_OR, testCase.requestPhrase),
										},
									},
								},
							},
						},
					}
					populationPhrases = append(populationPhrases, testCase.requestPhrase)
				}
				documentTypes := []string{string(documentType)}
				sources := elasticsearch.GetDefaultConversationSources()
				expectedFilters := []any{
					ChartDataFilterFilter{Terms: &elasticsearch.TermsFilter{ConversationToMessageJoin: &documentTypes}},
					ChartDataFilterFilter{Terms: &elasticsearch.TermsFilter{ConversationSources: &sources}},
				}
				if documentType == elasticsearch.DocumentTypeConversation {
					expectedFilters = append(expectedFilters, ChartDataFilterFilter{
						Range: &elasticsearch.RangeFilter{ConversationMessageCount: &elasticsearch.FloatRange{GT: proto.Float64(0)}},
					})
				}
				expectedFilters = append(expectedFilters, ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{
					Filter: &ChartValueSetFilter{Bool: &ChartValueSetFilterBool{
						MinimumShouldMatch: 1,
						Should:             &[]any{elasticsearch.MsgFilter{Terms: &elasticsearch.TermsFilter{AgentUserIDs: &[]string{"user-1"}}}},
					}},
				}})
				for phraseIndex, phrase := range populationPhrases {
					topicBool, err := elasticsearch.ConvertTopicDetailToTopicBool(elasticsearch.TopicDetails{
						LanguageCode:             "en-US",
						IncludedPhrases:          []elasticsearch.Phrase{{Text: phrase, SkipStemming: true}},
						IncludedPhrasesMatchType: elasticsearch.MatchAny,
					}, nil)
					testSuite.Require().NoError(err)
					conversationFilter := elasticsearch.ConvertTopicBoolsToConvoFilter([]*elasticsearch.TopicBool{topicBool})
					var populationFilter any = conversationFilter
					if documentType == elasticsearch.DocumentTypeMessage {
						populationFilter = elasticsearch.MsgFilter{
							HasParent: &elasticsearch.HasParent{
								ParentType: string(elasticsearch.DocumentTypeConversation),
								Query:      elasticsearch.ConvoQuery{Bool: elasticsearch.ConvoBool{Filter: &[]elasticsearch.ConvoFilter{conversationFilter}}},
							},
						}
					}
					if phraseIndex < len(testCase.savedPhrases) {
						populationFilter = ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{
							Filter: &ChartValueSetFilter{Bool: &ChartValueSetFilterBool{Should: &[]any{populationFilter}, MinimumShouldMatch: 1}},
						}}
					}
					expectedFilters = append(expectedFilters, populationFilter)
				}
				expected := ChartESQuery{Bool: &ChartESQueryBool{Filter: &expectedFilters}}
				original := proto.Clone(chartDataConfig)

				// When
				actual, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, requestFilter, nil, nil, nil, nil, nil, false)

				// Then
				testSuite.Require().NoError(err)
				assert.CmpEquals(testSuite.T(), expected, actual)
				assert.CmpEquals(testSuite.T(), original, chartDataConfig)
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestPersistedFuzzyKeywordOperatorSameMessageScope() {
	for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
		for _, operator := range []momentpb.KeywordMomentData_Operator{momentpb.KeywordMomentData_AND, momentpb.KeywordMomentData_OR} {
			testSuite.Run(string(documentType)+"_"+operator.String(), func() {
				chartDataConfig := &dashboardpb.ChartDataConfig{
					Metrics: []*dashboardpb.ChartMetric{{}},
					Filters: []*dashboardpb.ChartDataFilter{chartKeywordDataFilter("en-US", &momentpb.KeywordMomentData_KeywordExpression{
						Operator: operator,
						Operands: []*momentpb.KeywordMomentData_Operand{
							{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "alpha")}},
							{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "beta")}},
						},
					})},
				}
				keywordClauses := []elasticsearch.KeywordMomentBoolFilter{
					{MultiMatch: &elasticsearch.MultiMatchField{Query: "alpha", Type: "phrase", Fields: []string{"message_text_with_custom_analyzer.english"}, Slop: proto.Int32(5)}},
					{MultiMatch: &elasticsearch.MultiMatchField{Query: "beta", Type: "phrase", Fields: []string{"message_text_with_custom_analyzer.english"}, Slop: proto.Int32(5)}},
				}
				keywordBool := &elasticsearch.KeywordMomentBool{
					Filters: []elasticsearch.KeywordsIncludedFilter{{TermsFilter: &elasticsearch.TermsFilter{ConversationToMessageJoin: &[]string{string(elasticsearch.DocumentTypeMessage)}}}},
				}
				if operator == momentpb.KeywordMomentData_AND {
					keywordBool.Must = &keywordClauses
				} else {
					keywordBool.Should = &keywordClauses
					keywordBool.MinimumShouldMatch = 1
				}
				conversationFilter := elasticsearch.ConvoFilter{HasChild: &elasticsearch.HasChild{
					Type: string(elasticsearch.DocumentTypeMessage),
					Query: elasticsearch.InnerMsgQuery{Bool: &elasticsearch.InnerMsgBool{
						Should: &[]elasticsearch.MsgFilter{{Bool: &elasticsearch.TopicBool{
							Should:             &[]elasticsearch.KeywordsIncludedOrClustersShould{{KeywordsToInclude: keywordBool}},
							MinimumShouldMatch: 1,
						}}},
						MinimumShouldMatch: 1,
					}},
				}}
				var expected any = conversationFilter
				if documentType == elasticsearch.DocumentTypeMessage {
					expected = elasticsearch.MsgFilter{HasParent: &elasticsearch.HasParent{
						ParentType: string(elasticsearch.DocumentTypeConversation),
						Query:      elasticsearch.ConvoQuery{Bool: elasticsearch.ConvoBool{Filter: &[]elasticsearch.ConvoFilter{conversationFilter}}},
					}}
				}

				query, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)

				testSuite.Require().NoError(err)
				testSuite.Require().NotNil(query.Bool)
				testSuite.Require().NotNil(query.Bool.Filter)
				filters := *query.Bool.Filter
				testSuite.Require().NotEmpty(filters)
				valueSetFilter := &ChartValueSetFilter{Bool: &ChartValueSetFilterBool{Should: &[]any{expected}, MinimumShouldMatch: 1}}
				assert.CmpEquals(testSuite.T(), ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{Filter: valueSetFilter}}, filters[len(filters)-1])
				chartDataConfig.Filters[0].Operator = dashboardpb.ChartDataFilter_NOT_IN
				excludedQuery, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)
				testSuite.Require().NoError(err)
				excludedFilters := *excludedQuery.Bool.Filter
				assert.CmpEquals(testSuite.T(), ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{MustNot: valueSetFilter}}, excludedFilters[len(excludedFilters)-1])
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestKeywordLanguageFallback() {
	for _, testCase := range []struct {
		name              string
		language          string
		effectiveLanguage string
	}{
		{name: "Omitted", effectiveLanguage: consts.AllLanguages},
		{name: "English", language: "en-US", effectiveLanguage: "en-US"},
		{name: "Spanish", language: "es-ES", effectiveLanguage: "es-ES"},
		{name: "NumericRegion", language: "es-419", effectiveLanguage: "es-419"},
		{name: "ScriptAndRegion", language: "zh-Hant-TW", effectiveLanguage: "zh-Hant-TW"},
		{name: "DefaultAnalyzer", language: "ta-IN", effectiveLanguage: "ta-IN"},
		{name: "AllLanguages", language: consts.AllLanguages, effectiveLanguage: consts.AllLanguages},
	} {
		for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
			testSuite.Run(testCase.name+"_"+string(documentType), func() {
				expression := &momentpb.KeywordMomentData_KeywordExpression{
					Operator: momentpb.KeywordMomentData_AND,
					Operands: []*momentpb.KeywordMomentData_Operand{
						{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "alpha")}},
						{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "beta")}},
					},
				}
				filter := chartKeywordDataFilter(testCase.language, expression)
				chartDataConfig := &dashboardpb.ChartDataConfig{Metrics: []*dashboardpb.ChartMetric{{}}, Filters: []*dashboardpb.ChartDataFilter{filter}}
				original := proto.Clone(chartDataConfig)
				topicBool, err := elasticsearch.ConvertTopicDetailToTopicBool(elasticsearch.TopicDetails{
					LanguageCode:             testCase.effectiveLanguage,
					IncludedPhrases:          []elasticsearch.Phrase{{Text: "alpha", Flexible: true}, {Text: "beta", Flexible: true}},
					IncludedPhrasesMatchType: elasticsearch.MatchAll,
				}, nil)
				testSuite.Require().NoError(err)
				conversationFilter := elasticsearch.ConvertTopicBoolsToConvoFilter([]*elasticsearch.TopicBool{topicBool})
				var expected any = conversationFilter
				if documentType == elasticsearch.DocumentTypeMessage {
					expected = elasticsearch.MsgFilter{HasParent: &elasticsearch.HasParent{
						ParentType: string(elasticsearch.DocumentTypeConversation),
						Query:      elasticsearch.ConvoQuery{Bool: elasticsearch.ConvoBool{Filter: &[]elasticsearch.ConvoFilter{conversationFilter}}},
					}}
				}
				valueSetFilter := &ChartValueSetFilter{Bool: &ChartValueSetFilterBool{Should: &[]any{expected}, MinimumShouldMatch: 1}}

				query, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)

				testSuite.Require().NoError(err)
				filters := *query.Bool.Filter
				assert.CmpEquals(testSuite.T(), ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{Filter: valueSetFilter}}, filters[len(filters)-1])
				assert.CmpEquals(testSuite.T(), original, chartDataConfig)
				filter.Operator = dashboardpb.ChartDataFilter_NOT_IN
				excludedQuery, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)
				testSuite.Require().NoError(err)
				excludedFilters := *excludedQuery.Bool.Filter
				assert.CmpEquals(testSuite.T(), ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{MustNot: valueSetFilter}}, excludedFilters[len(excludedFilters)-1])
				testSuite.Equal(testCase.language, filter.ValueSet.GetKeywordFilterValue().GetLanguageCode())
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestPinnedClusterAndOccurrenceQueryParity() {
	for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
		for _, pinned := range []bool{false, true} {
			for _, operator := range []momentpb.KeywordMomentData_Operator{momentpb.KeywordMomentData_AND, momentpb.KeywordMomentData_OR} {
				name := string(documentType) + "_" + operator.String() + "_Unpinned"
				if pinned {
					name = string(documentType) + "_" + operator.String() + "_Pinned"
				}
				testSuite.Run(name, func() {
					momentName := "customers/test/profiles/test/moments/subcategory"
					nativeFilter := &dashboardpb.ChartDataFilter{
						Operator: dashboardpb.ChartDataFilter_IN,
						ValueSet: &dashboardpb.ChartValueSet{Values: &dashboardpb.ChartValueSet_SingleValues{
							SingleValues: &dashboardpb.ChartValueSet_Single{AttributeValues: []*dashboardpb.ChartAttributeValue{{
								Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{SubcategoryValue: &dashboardpb.SubcategoryForChart{MomentName: momentName}},
							}}},
						}},
					}
					topicDetails := elasticsearch.TopicDetails{LanguageCode: "en-US", IncludedPhrases: []elasticsearch.Phrase{{Text: "topic"}}, IncludedPhrasesMatchType: elasticsearch.MatchAny}
					if pinned {
						topicDetails.Clusters = []string{"customers/test/profiles/test/moments/cluster-1"}
					}
					topics := map[string]elasticsearch.TopicDetails{momentName: topicDetails}
					expression := chartKeywordExpression(operator, "alpha")
					expression.Operands = append(expression.Operands, &momentpb.KeywordMomentData_Operand{
						Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "beta")},
					})
					keywordFilter := chartKeywordDataFilter("en-US", expression)
					legacyConfig := &dashboardpb.ChartDataConfig{Metrics: []*dashboardpb.ChartMetric{{}}, Filters: []*dashboardpb.ChartDataFilter{nativeFilter}}
					persistedConfig := &dashboardpb.ChartDataConfig{Metrics: legacyConfig.Metrics, Filters: []*dashboardpb.ChartDataFilter{nativeFilter, keywordFilter}}
					requestFilter, err := keywordFilterToAnalyticsChartFilter(keywordFilter.ValueSet.GetKeywordFilterValue())
					testSuite.Require().NoError(err)
					var occurrenceFilters map[*dashboardpb.ChartDataFilter]any
					if documentType == elasticsearch.DocumentTypeMessage {
						occurrenceFilters = map[*dashboardpb.ChartDataFilter]any{nativeFilter: nil, keywordFilter: nil}
					}

					persistedQuery, persistedErr := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, persistedConfig, nil, nil, topics, occurrenceFilters, nil, nil, false)
					legacyQuery, legacyErr := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, legacyConfig, requestFilter, nil, topics, occurrenceFilters, nil, nil, false)

					testSuite.Require().NoError(persistedErr)
					testSuite.Require().NoError(legacyErr)
					legacyFilters := *legacyQuery.Bool.Filter
					persistedFilters := *persistedQuery.Bool.Filter
					testSuite.Require().Len(persistedFilters, len(legacyFilters))
					assert.CmpEquals(testSuite.T(), legacyFilters[:len(legacyFilters)-1], persistedFilters[:len(persistedFilters)-1])
					assert.CmpEquals(testSuite.T(), ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{
						Filter: &ChartValueSetFilter{Bool: &ChartValueSetFilterBool{Should: &[]any{legacyFilters[len(legacyFilters)-1]}, MinimumShouldMatch: 1}},
					}}, persistedFilters[len(persistedFilters)-1])
				})
			}
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestJoinOptimizationDoesNotRescopeKeywords() {
	for _, testCase := range []struct {
		name        string
		document    elasticsearch.DocumentType
		convoCount  bool
		pinned      bool
		simplified  bool
		removeJoins bool
	}{
		{name: "ConversationCount", document: elasticsearch.DocumentTypeConversation, convoCount: true, simplified: true, removeJoins: true},
		{name: "PinnedConversationCount", document: elasticsearch.DocumentTypeConversation, convoCount: true, pinned: true, simplified: true},
		{name: "MessageMetric", document: elasticsearch.DocumentTypeMessage, simplified: true},
		{name: "SimplificationDisabled", document: elasticsearch.DocumentTypeConversation, convoCount: true},
	} {
		testSuite.Run(testCase.name, func() {
			testSuite.service.enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart = testCase.simplified
			filter := &dashboardpb.ChartDataFilter{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{Values: &dashboardpb.ChartValueSet_SingleValues{
					SingleValues: &dashboardpb.ChartValueSet_Single{AttributeValues: []*dashboardpb.ChartAttributeValue{{
						Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{SubcategoryValue: &dashboardpb.SubcategoryForChart{MomentName: "customers/test/profiles/test/moments/subcategory"}},
					}}},
				}},
			}
			stats := &statsType{isConvoCount: testCase.convoCount}
			legacyFilters := []*dashboardpb.ChartDataFilter{filter}
			persistedFilters := []*dashboardpb.ChartDataFilter{filter, chartKeywordDataFilter("en-US", chartKeywordExpression(momentpb.KeywordMomentData_AND, "alpha"))}

			legacyEligible, legacyErr := testSuite.service.canRemoveJoinsByFilteringOnMessage(testSuite.T().Context(), testCase.document, nil, legacyFilters, stats, testCase.pinned, nil)
			persistedEligible, persistedErr := testSuite.service.canRemoveJoinsByFilteringOnMessage(testSuite.T().Context(), testCase.document, nil, persistedFilters, stats, testCase.pinned, nil)

			testSuite.Require().NoError(legacyErr)
			testSuite.Require().NoError(persistedErr)
			testSuite.Equal(testCase.removeJoins, legacyEligible)
			testSuite.Equal(legacyEligible, persistedEligible)
		})
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestValueSetRoundTripAndScope() {
	for _, language := range []string{"", "en-US", "es-ES", consts.AllLanguages} {
		for _, operator := range []momentpb.KeywordMomentData_Operator{momentpb.KeywordMomentData_AND, momentpb.KeywordMomentData_OR} {
			testSuite.Run(language+"_"+operator.String(), func() {
				expression := chartKeywordExpression(operator, "alpha beta")
				expression.Operands = append(expression.Operands, &momentpb.KeywordMomentData_Operand{
					Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "beneficios")},
				})
				valueSet := chartKeywordDataFilter(language, expression).ValueSet
				original := proto.Clone(valueSet)

				equivalent, err := fromChartValueSet(testSuite.T().Context(), valueSet)

				testSuite.Require().NoError(err)
				testSuite.Equal(attributeValueTypeTranscriptKeywords, equivalent.attributeValueType)
				assert.CmpEquals(testSuite.T(), valueSet.GetKeywordFilterValue(), equivalent.keywordFilter)
				assert.CmpEquals(testSuite.T(), original, toChartValueSet(*equivalent))
				isMessage, isConversation, err := testSuite.service.isMessageOrConvoEntity(testSuite.T().Context(), nil, valueSet)
				testSuite.Require().NoError(err)
				testSuite.False(isMessage)
				testSuite.True(isConversation)
				assert.CmpEquals(testSuite.T(), original, valueSet)
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestKeywordFilterWithoutTargetAttribute() {
	for _, testCase := range []struct {
		name         string
		documentType elasticsearch.DocumentType
		operator     dashboardpb.ChartDataFilter_Operator
	}{
		{name: "ConversationIn", documentType: elasticsearch.DocumentTypeConversation, operator: dashboardpb.ChartDataFilter_IN},
		{name: "ConversationNotIn", documentType: elasticsearch.DocumentTypeConversation, operator: dashboardpb.ChartDataFilter_NOT_IN},
		{name: "MessageIn", documentType: elasticsearch.DocumentTypeMessage, operator: dashboardpb.ChartDataFilter_IN},
		{name: "MessageNotIn", documentType: elasticsearch.DocumentTypeMessage, operator: dashboardpb.ChartDataFilter_NOT_IN},
	} {
		testSuite.Run(testCase.name, func() {
			filter := chartKeywordDataFilter("en-US", chartKeywordExpression(momentpb.KeywordMomentData_AND, "alpha beta"))
			filter.Operator = testCase.operator
			chartDataConfig := &dashboardpb.ChartDataConfig{Metrics: []*dashboardpb.ChartMetric{{}}, Filters: []*dashboardpb.ChartDataFilter{filter}}
			serialized, serializationErr := (dashboardpb.ChartDataConfigSQL{Message: chartDataConfig}).Value()
			query, queryErr := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, testCase.documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)

			testSuite.Require().NoError(serializationErr)
			testSuite.Equal(dashboardpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, filter.GetTargetAttribute())
			testSuite.NotContains(string(serialized.([]byte)), "targetAttribute")
			testSuite.NotContains(string(serialized.([]byte)), "target_attribute")
			testSuite.Require().NoError(queryErr)
			testSuite.Require().NotNil(query.Bool)
			testSuite.Require().NotNil(query.Bool.Filter)
			filters := *query.Bool.Filter
			testSuite.Require().NotEmpty(filters)
			keywordQueryFilter, isChartFilter := filters[len(filters)-1].(ChartDataFilterFilter)
			testSuite.Require().True(isChartFilter)
			testSuite.Require().NotNil(keywordQueryFilter.Bool)
			if testCase.operator == dashboardpb.ChartDataFilter_IN {
				testSuite.NotNil(keywordQueryFilter.Bool.Filter)
				testSuite.Nil(keywordQueryFilter.Bool.MustNot)
			} else {
				testSuite.Nil(keywordQueryFilter.Bool.Filter)
				testSuite.NotNil(keywordQueryFilter.Bool.MustNot)
			}
		})
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestMalformedKeywordFilterContract() {
	for _, testCase := range []struct {
		name   string
		modify func(*dashboardpb.ChartDataFilter)
	}{
		{name: "EmptyValueSet", modify: func(filter *dashboardpb.ChartDataFilter) { filter.ValueSet = &dashboardpb.ChartValueSet{} }},
		{name: "NilKeywordPayload", modify: func(filter *dashboardpb.ChartDataFilter) {
			filter.ValueSet.Values = &dashboardpb.ChartValueSet_KeywordFilterValue{}
		}},
		{name: "MissingOperator", modify: func(filter *dashboardpb.ChartDataFilter) {
			filter.Operator = dashboardpb.ChartDataFilter_OPERATOR_UNSPECIFIED
		}},
		{name: "UnknownOperator", modify: func(filter *dashboardpb.ChartDataFilter) { filter.Operator = dashboardpb.ChartDataFilter_Operator(123) }},
	} {
		for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
			testSuite.Run(testCase.name+"_"+string(documentType), func() {
				filter := chartKeywordDataFilter("en-US", chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha"))
				testCase.modify(filter)
				chartDataConfig := &dashboardpb.ChartDataConfig{Metrics: []*dashboardpb.ChartMetric{{}}, Filters: []*dashboardpb.ChartDataFilter{filter}}

				query, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)

				testSuite.Require().Error(err)
				testSuite.Equal(codes.InvalidArgument, status.Code(err))
				testSuite.Nil(query.Bool)
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestMalformedPersistedFilter() {
	for _, testCase := range []struct {
		name       string
		language   string
		expression *momentpb.KeywordMomentData_KeywordExpression
	}{
		{name: "EmptyFilter"},
		{name: "WhitespaceLanguage", language: " ", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "UnderscoreLanguage", language: "en_US", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "InvalidLanguageName", language: "english", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "EmptyLanguageSubtag", language: "en--US", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "TrailingLanguageSeparator", language: "en-US-", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "InvalidLanguageRegion", language: "en-12", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "LeadingLanguageWhitespace", language: " en-US", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "TrailingLanguageWhitespace", language: "en-US ", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "InvalidLanguageCharacter", language: "en-@US", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "alpha")},
		{name: "MissingExpression", language: "en-US"},
		{name: "EmptyExpression", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{Operator: momentpb.KeywordMomentData_OR}},
		{name: "UnknownOperator", language: "en-US", expression: chartKeywordExpression(momentpb.KeywordMomentData_OPERATOR_UNSPECIFIED, "alpha")},
		{name: "UnsupportedRootSequence", language: "en-US", expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, "alpha")},
		{name: "UnsupportedRootExclude", language: "en-US", expression: chartKeywordExpression(momentpb.KeywordMomentData_EXCLUDE, "alpha")},
		{name: "EmptyPhrase", language: "en-US", expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, " ")},
		{name: "NilOperand", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{Operator: momentpb.KeywordMomentData_OR, Operands: []*momentpb.KeywordMomentData_Operand{nil}}},
		{name: "UnknownNestedOperator", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{
			Operator: momentpb.KeywordMomentData_AND,
			Operands: []*momentpb.KeywordMomentData_Operand{
				{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "alpha"}},
				{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_OPERATOR_UNSPECIFIED, "beta")}},
			},
		}},
		{name: "UnsupportedNestedAnd", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{
			Operator: momentpb.KeywordMomentData_AND,
			Operands: []*momentpb.KeywordMomentData_Operand{
				{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "alpha"}},
				{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_AND, "beta")}},
			},
		}},
		{name: "UnsupportedNestedOr", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{
			Operator: momentpb.KeywordMomentData_OR,
			Operands: []*momentpb.KeywordMomentData_Operand{
				{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "beta")}},
			},
		}},
		{name: "EmptySequence", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{
			Operator: momentpb.KeywordMomentData_OR,
			Operands: []*momentpb.KeywordMomentData_Operand{
				{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: &momentpb.KeywordMomentData_KeywordExpression{Operator: momentpb.KeywordMomentData_SEQUENCE}}},
			},
		}},
		{name: "EmptySequencePhrase", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{
			Operator: momentpb.KeywordMomentData_OR,
			Operands: []*momentpb.KeywordMomentData_Operand{
				{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_SEQUENCE, " ")}},
			},
		}},
		{name: "NestedSequenceExpression", language: "en-US", expression: &momentpb.KeywordMomentData_KeywordExpression{
			Operator: momentpb.KeywordMomentData_OR,
			Operands: []*momentpb.KeywordMomentData_Operand{
				{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: &momentpb.KeywordMomentData_KeywordExpression{
					Operator: momentpb.KeywordMomentData_SEQUENCE,
					Operands: []*momentpb.KeywordMomentData_Operand{
						{Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: chartKeywordExpression(momentpb.KeywordMomentData_OR, "beta")}},
					},
				}}},
			},
		}},
	} {
		for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
			testSuite.Run(testCase.name+"_"+string(documentType), func() {
				// Given
				chartDataConfig := &dashboardpb.ChartDataConfig{
					Metrics: []*dashboardpb.ChartMetric{{}},
					Filters: []*dashboardpb.ChartDataFilter{chartKeywordDataFilter(testCase.language, testCase.expression)},
				}

				// When
				query, err := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, &analyticspb.AnalyticsChartFilter{InsightsTopic: &analyticspb.InsightsTopic{SpeakerRoles: []string{"CUSTOMER"}}}, nil, nil, nil, nil, nil, false)

				// Then
				testSuite.Require().Error(err)
				testSuite.Equal(codes.InvalidArgument, status.Code(err))
				testSuite.Contains(err.Error(), "chart keyword_filter")
				testSuite.Nil(query.Bool)
				_, err = fromChartValueSet(testSuite.T().Context(), chartDataConfig.Filters[0].ValueSet)
				testSuite.Equal(codes.InvalidArgument, status.Code(err))
				chartDataConfig.Filters[0].Operator = dashboardpb.ChartDataFilter_NOT_IN
				_, err = testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, chartDataConfig, nil, nil, nil, nil, nil, nil, false)
				testSuite.Equal(codes.InvalidArgument, status.Code(err))
			})
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestPythonCompilerQueryParity() {
	for _, documentType := range []elasticsearch.DocumentType{elasticsearch.DocumentTypeConversation, elasticsearch.DocumentTypeMessage} {
		for _, language := range []string{"", "en-US", "es-ES", consts.AllLanguages} {
			for _, testCase := range []struct {
				matchType string
				operator  momentpb.KeywordMomentData_Operator
			}{
				{matchType: "Exact", operator: momentpb.KeywordMomentData_OR},
				{matchType: "Fuzzy", operator: momentpb.KeywordMomentData_OR},
				{matchType: "Mixed", operator: momentpb.KeywordMomentData_OR},
				{matchType: "Exact", operator: momentpb.KeywordMomentData_AND},
				{matchType: "Fuzzy", operator: momentpb.KeywordMomentData_AND},
				{matchType: "Mixed", operator: momentpb.KeywordMomentData_AND},
			} {
				testSuite.Run(string(documentType)+"_"+language+"_"+testCase.matchType+"_"+testCase.operator.String(), func() {
					// Given
					expression := &momentpb.KeywordMomentData_KeywordExpression{Operator: testCase.operator}
					if testCase.matchType != "Fuzzy" {
						expression.Operands = append(expression.Operands, &momentpb.KeywordMomentData_Operand{
							Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "alpha/beta"},
						}, &momentpb.KeywordMomentData_Operand{
							Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "member benefits"},
						})
					}
					if testCase.matchType != "Exact" {
						expression.Operands = append(expression.Operands, &momentpb.KeywordMomentData_Operand{
							Operand: &momentpb.KeywordMomentData_Operand_Expression{Expression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_SEQUENCE,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "over"}},
									{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "counter"}},
								},
							}},
						})
					}
					legacyConfig := &dashboardpb.ChartDataConfig{Metrics: []*dashboardpb.ChartMetric{{}}}
					persistedConfig := proto.Clone(legacyConfig).(*dashboardpb.ChartDataConfig)
					persistedConfig.Filters = []*dashboardpb.ChartDataFilter{chartKeywordDataFilter(language, expression)}
					requestFilter := &analyticspb.AnalyticsChartFilter{InsightsTopic: &analyticspb.InsightsTopic{
						IncludedKeywordMoment: &momentpb.Moment{
							Type: momentpb.Moment_KEYWORD, AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
							MomentSource: momentpb.MomentSource_MOMENT_SOURCE_CUSTOM, LanguageCode: language,
							MomentData: &momentpb.MomentData{Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{KeywordExpression: expression},
							}},
						},
					}}

					// When
					persistedQuery, persistedErr := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, persistedConfig, nil, nil, nil, nil, nil, nil, false)
					legacyQuery, legacyErr := testSuite.service.convertChartToESQuery(testSuite.T().Context(), shared.CustomerProfile{}, nil, documentType, legacyConfig, requestFilter, nil, nil, nil, nil, nil, false)

					// Then
					testSuite.Require().NoError(persistedErr)
					testSuite.Require().NoError(legacyErr)
					legacyFilters := *legacyQuery.Bool.Filter
					persistedFilters := *persistedQuery.Bool.Filter
					testSuite.Require().Len(persistedFilters, len(legacyFilters))
					assert.CmpEquals(testSuite.T(), legacyFilters[:len(legacyFilters)-1], persistedFilters[:len(persistedFilters)-1])
					assert.CmpEquals(testSuite.T(), ChartDataFilterFilter{Bool: &ChartDataFilterFilterBool{
						Filter: &ChartValueSetFilter{Bool: &ChartValueSetFilterBool{Should: &[]any{legacyFilters[len(legacyFilters)-1]}, MinimumShouldMatch: 1}},
					}}, persistedFilters[len(persistedFilters)-1])
				})
			}
		}
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestCachePopulationIsolation() {
	// Given
	ctx := metadata.NewIncomingContext(testSuite.T().Context(), metadata.Pairs("cresta-access-token-claim", `{"peer":{"hostname":"test-service"}}`))
	principal, err := auth.GetPrincipalEntityFromAuthContext(ctx)
	testSuite.Require().NoError(err)
	hashes := []string{}
	for _, testCase := range []struct {
		name           string
		savedLanguage  string
		savedPhrase    string
		requestSpeaker string
		exclude        bool
	}{
		{name: "Legacy"},
		{name: "SavedEnglish", savedLanguage: "en-US", savedPhrase: "alpha"},
		{name: "SavedAllAnalyzers", savedPhrase: "alpha"},
		{name: "SavedSpanish", savedLanguage: "es-ES", savedPhrase: "alpha"},
		{name: "SavedOtherPhrase", savedLanguage: "en-US", savedPhrase: "beta"},
		{name: "SavedExcluded", savedLanguage: "en-US", savedPhrase: "alpha", exclude: true},
		{name: "RequestOnly", requestSpeaker: "AGENT"},
		{name: "SavedAndRequest", savedLanguage: "en-US", savedPhrase: "alpha", requestSpeaker: "AGENT"},
		{name: "DifferentRequest", savedLanguage: "en-US", savedPhrase: "alpha", requestSpeaker: "CUSTOMER"},
	} {
		testSuite.Run(testCase.name, func() {
			// Given
			request := &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: "customers/test/profiles/test",
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					DataConfig:      &dashboardpb.ChartDataConfig{Metrics: []*dashboardpb.ChartMetric{{}}},
					RenderingConfig: &dashboardpb.RenderingConfig{ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART},
				},
			}
			if testCase.savedPhrase != "" {
				request.AnalyticsChart.DataConfig.Filters = []*dashboardpb.ChartDataFilter{chartKeywordDataFilter(testCase.savedLanguage, chartKeywordExpression(momentpb.KeywordMomentData_OR, testCase.savedPhrase))}
				if testCase.exclude {
					request.AnalyticsChart.DataConfig.Filters[0].Operator = dashboardpb.ChartDataFilter_NOT_IN
				}
			}
			if testCase.requestSpeaker != "" {
				request.ConvoFilter = &analyticspb.AnalyticsChartFilter{InsightsTopic: &analyticspb.InsightsTopic{SpeakerRoles: []string{testCase.requestSpeaker}}}
			}
			original := proto.Clone(request)

			// When
			cachedRequest := testSuite.service.getCachedRequestProto(ctx, request, principal)
			hash, err := protobuf.HashProto(cachedRequest)

			// Then
			testSuite.Require().NoError(err)
			testSuite.NotContains(hashes, hash)
			assert.CmpEquals(testSuite.T(), request.GetAnalyticsChart().GetDataConfig(), cachedRequest.GetChartRequest().GetAnalyticsChart().GetDataConfig())
			assert.CmpEquals(testSuite.T(), request.GetConvoFilter(), cachedRequest.GetChartRequest().GetConvoFilter())
			testSuite.Nil(cachedRequest.GetChartRequest().GetAnalyticsChart().GetRenderingConfig())
			assert.CmpEquals(testSuite.T(), original, request)
			hashes = append(hashes, hash)
		})
	}
}

func (testSuite *chartKeywordFilterTestSuite) TestPercentageBaselinesRetainPopulation() {
	for _, testCase := range []struct {
		name  string
		build func(*analyticspb.EvaluateAnalyticsChartRequest) *analyticspb.EvaluateAnalyticsChartRequest
	}{
		{name: "Ungrouped", build: getUngroupedRequest},
		{name: "PrimaryGroupOnly", build: getPrimaryGroupOnlyRequest},
		{name: "GroupsAsFilters", build: getGroupsAsFiltersRequest},
	} {
		testSuite.Run(testCase.name, func() {
			// Given
			request := &analyticspb.EvaluateAnalyticsChartRequest{
				AnalyticsChart: &dashboardpb.AnalyticsChart{DataConfig: &dashboardpb.ChartDataConfig{
					Filters: []*dashboardpb.ChartDataFilter{chartKeywordDataFilter("es-ES", chartKeywordExpression(momentpb.KeywordMomentData_OR, "beneficios beta"))},
					Metrics: []*dashboardpb.ChartMetric{{}},
				}},
				ConvoFilter: &analyticspb.AnalyticsChartFilter{InsightsTopic: &analyticspb.InsightsTopic{SpeakerRoles: []string{"CUSTOMER"}}},
			}
			original := proto.Clone(request)

			// When
			baseline := testCase.build(request)

			// Then
			assert.CmpEquals(testSuite.T(), request.GetAnalyticsChart().GetDataConfig().GetFilters(), baseline.GetAnalyticsChart().GetDataConfig().GetFilters())
			assert.CmpEquals(testSuite.T(), request.GetConvoFilter(), baseline.GetConvoFilter())
			assert.CmpEquals(testSuite.T(), original, request)
		})
	}
}
