package analyticsimpl

import (
	"testing"

	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/stretchr/testify/suite"
)

type EvaluateAnalyticsChartMetricTestSuite struct {
	suite.Suite
}

func TestEvaluateAnalyticsChartMetricTestSuite(t *testing.T) {
	suite.Run(t, new(EvaluateAnalyticsChartMetricTestSuite))
}

func (s *EvaluateAnalyticsChartMetricTestSuite) TestUseConversationCountViaDocCount() {
	s.Run("uses doc_count for conversation count on conversation documents", func() {
		// Given
		metricsAgg := &ChartESAggregation{
			MetricsConversationCount: elasticsearch.CreateReqTotalConversationCount(40000),
		}
		statsType := &statsType{isConvoCount: true}

		// When
		useConversationCountViaDocCount(metricsAgg, statsType, elasticsearch.DocumentTypeConversation, nil, []*dashboardpb.ChartGroupBy{{}})

		// Then
		s.True(metricsAgg.isConversationCountViaDocCount)
		s.Nil(metricsAgg.MetricsConversationCount)
	})

	s.Run("keeps cardinality when aggregating on message documents", func() {
		// Given
		cardinalityAgg := elasticsearch.CreateReqTotalConversationCount(40000)
		metricsAgg := &ChartESAggregation{
			MetricsConversationCount: cardinalityAgg,
		}
		statsType := &statsType{isConvoCount: true}

		// When
		useConversationCountViaDocCount(metricsAgg, statsType, elasticsearch.DocumentTypeMessage, nil, []*dashboardpb.ChartGroupBy{{}})

		// Then
		s.False(metricsAgg.isConversationCountViaDocCount)
		s.Equal(cardinalityAgg, metricsAgg.MetricsConversationCount)
	})

	s.Run("keeps cardinality for average occurrence count per conversation", func() {
		// Given
		cardinalityAgg := elasticsearch.CreateReqTotalConversationCount(40000)
		metricsAgg := &ChartESAggregation{
			MetricsConversationCount:         cardinalityAgg,
			isAverageOccurrenceCountPerConvo: true,
		}
		statsType := &statsType{isConvoCount: false}

		// When
		useConversationCountViaDocCount(metricsAgg, statsType, elasticsearch.DocumentTypeConversation, nil, []*dashboardpb.ChartGroupBy{{}})

		// Then
		s.False(metricsAgg.isConversationCountViaDocCount)
		s.Equal(cardinalityAgg, metricsAgg.MetricsConversationCount)
	})

	s.Run("keeps cardinality when metric has groupby from metrics config", func() {
		// Given
		cardinalityAgg := elasticsearch.CreateReqTotalConversationCount(40000)
		metricsAgg := &ChartESAggregation{
			MetricsConversationCount: cardinalityAgg,
		}
		statsType := &statsType{isConvoCount: true}
		chartMetric := &dashboardpb.ChartMetric{
			GroupbyConfig: &dashboardpb.ChartGroupBy_Config{
				SortDirection: dashboardpb.ChartGroupBy_Config_DESC,
			},
		}

		// When
		useConversationCountViaDocCount(metricsAgg, statsType, elasticsearch.DocumentTypeConversation, chartMetric, []*dashboardpb.ChartGroupBy{{}})

		// Then
		s.False(metricsAgg.isConversationCountViaDocCount)
		s.Equal(cardinalityAgg, metricsAgg.MetricsConversationCount)
	})

	s.Run("keeps cardinality when chart has no groupby", func() {
		// Given
		cardinalityAgg := elasticsearch.CreateReqTotalConversationCount(40000)
		metricsAgg := &ChartESAggregation{
			MetricsConversationCount: cardinalityAgg,
		}
		statsType := &statsType{isConvoCount: true}

		// When
		useConversationCountViaDocCount(metricsAgg, statsType, elasticsearch.DocumentTypeConversation, nil, nil)

		// Then
		s.False(metricsAgg.isConversationCountViaDocCount)
		s.Equal(cardinalityAgg, metricsAgg.MetricsConversationCount)
	})
}

func (s *EvaluateAnalyticsChartMetricTestSuite) TestCorrectOrderByForTermsAggWithConversationCountViaDocCount() {
	// Given
	chartDataConfig := &dashboardpb.ChartDataConfig{
		Metrics: []*dashboardpb.ChartMetric{{}},
		GroupBy: []*dashboardpb.ChartGroupBy{{}},
	}
	metricsAgg := &ChartESAggregation{isConversationCountViaDocCount: true}
	mergedAggs := &ChartESAggregation{
		BucketAggs: &BucketAggregation{
			Nested: &NestedDocumentPath{Path: "moment_annotations"},
			Aggs: &ChartESAggregation{
				BucketAggs: &BucketAggregation{
					Terms: &elasticsearch.TermsAggs{
						OrderBy: map[string]string{"_count": "desc"},
					},
					Aggs: &ChartESAggregation{
						BucketAggs: &BucketAggregation{
							ReverseNested: &elasticsearch.EmptyStruct{},
						},
					},
				},
			},
		},
	}

	// When
	err := correctOrderByForTermsAgg(chartDataConfig, metricsAgg, mergedAggs)

	// Then
	s.Require().NoError(err)
	termsAgg := mergedAggs.BucketAggs.Aggs.BucketAggs.Terms
	s.Equal(map[string]string{"bucket_aggs>_count": "desc"}, termsAgg.OrderBy)
}

func (s *EvaluateAnalyticsChartMetricTestSuite) TestMergeMetricAggUsesDocCountForConversationCount() {
	// Given
	bucketAggs := &ChartESAggregation{
		BucketAggs: &BucketAggregation{
			DateHistogram: &elasticsearch.ReqDateHistogram{},
		},
	}
	metricsAgg := &ChartESAggregation{
		isConversationCountViaDocCount: true,
	}

	// When
	merged := mergeMetricAgg(bucketAggs, metricsAgg)

	// Then
	s.Equal(bucketAggs, merged)
	s.Nil(merged.BucketAggs.Aggs)
}
