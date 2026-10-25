package analyticsimpl

import (
	"context"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/shared/consts"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/stretchr/testify/require"
)

func TestConvertInsightsTopicToTopicDetailsLanguageCode(t *testing.T) {
	testCases := []struct {
		name            string
		topicMoment     *momentpb.Moment
		keywordLanguage string
		wantLanguage    string
	}{
		{
			name:            "included keyword moment supplies English language",
			keywordLanguage: "en-US",
			wantLanguage:    "en-US",
		},
		{
			name:            "included keyword moment supplies Spanish language",
			keywordLanguage: "es-ES",
			wantLanguage:    "es-ES",
		},
		{
			name:            "topic moment language takes precedence",
			topicMoment:     &momentpb.Moment{LanguageCode: "ja-JP"},
			keywordLanguage: "en-US",
			wantLanguage:    "ja-JP",
		},
		{
			name:            "explicit all languages on topic moment takes precedence",
			topicMoment:     &momentpb.Moment{LanguageCode: consts.AllLanguages},
			keywordLanguage: "en-US",
			wantLanguage:    consts.AllLanguages,
		},
		{
			name:            "empty topic moment language falls back to keyword language",
			topicMoment:     &momentpb.Moment{},
			keywordLanguage: "en-US",
			wantLanguage:    "en-US",
		},
		{
			name:         "missing languages default to all languages",
			wantLanguage: consts.AllLanguages,
		},
		{
			name:         "empty topic moment language defaults to all languages",
			topicMoment:  &momentpb.Moment{},
			wantLanguage: consts.AllLanguages,
		},
	}
	service := AnalyticsServiceImpl{logger: logger.GetLogger()}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			topic := insightsTopic{apiInsightsTopic: &analyticspb.InsightsTopic{
				TopicMoment: testCase.topicMoment,
				IncludedKeywordMoment: &momentpb.Moment{
					LanguageCode: testCase.keywordLanguage,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_KeywordMomentData{
							KeywordMomentData: &momentpb.KeywordMomentData{
								KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
									Operator: momentpb.KeywordMomentData_OR,
									Operands: []*momentpb.KeywordMomentData_Operand{{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "red"},
									}},
								},
							},
						},
					},
				},
			}}

			details, err := service.convertInsightsTopicToTopicDetails(context.Background(), topic)

			require.NoError(t, err)
			require.NotNil(t, details)
			require.Equal(t, testCase.wantLanguage, details.LanguageCode)
			require.Len(t, details.IncludedPhrases, 1)
			require.Equal(t, "red", details.IncludedPhrases[0].Text)
		})
	}
}

func TestConvertInsightsTopicToTopicDetailsWithoutIncludedKeywordMoment(t *testing.T) {
	service := AnalyticsServiceImpl{logger: logger.GetLogger()}
	topic := insightsTopic{apiInsightsTopic: &analyticspb.InsightsTopic{
		ClusterMoments: []*momentpb.Moment{{Name: "cluster-1"}},
	}}

	details, err := service.convertInsightsTopicToTopicDetails(context.Background(), topic)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.Equal(t, consts.AllLanguages, details.LanguageCode)
}
