package analyticsimpl

import (
	"testing"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ESResponseTraversalTestSuite struct {
	suite.Suite
}

func TestESResponseTraversalTestSuite(t *testing.T) {
	suite.Run(t, new(ESResponseTraversalTestSuite))
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_WithBucketAggs() {
	s.Run("should traverse through standard BucketAggs to find metrics", func() {
		// Given
		metricsValue := newESRespMetricsValue(42.0)
		innerBucket := &ESRespSingleBucket{
			MetricsConversationCount: metricsValue,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggs: innerBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsConversationCount)
	})

	s.Run("should traverse through multiple levels of BucketAggs", func() {
		// Given
		metricsValue := newESRespMetricsValue(100.0)
		deepestBucket := &ESRespSingleBucket{
			MetricsAggOnNumericField: metricsValue,
		}
		middleBucket := &ESRespSingleBucket{
			BucketAggs: deepestBucket,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggs: middleBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsAggOnNumericField)
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_WithBucketAggsForOrderByMetrics() {
	s.Run("should traverse through BucketAggsForOrderByMetrics to find metrics", func() {
		// Given
		metricsValue := newESRespMetricsValue(123.0)
		innerBucket := &ESRespSingleBucket{
			MetricsConversationCount: metricsValue,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: innerBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsConversationCount)
	})

	s.Run("should traverse through nested BucketAggsForOrderByMetrics", func() {
		// Given
		metricsValue := newESRespMetricsValue(456.0)
		deepestBucket := &ESRespSingleBucket{
			MetricsConversationMessageCount: metricsValue,
		}
		middleBucket := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: deepestBucket,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: middleBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsConversationMessageCount)
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_PriorityOrder() {
	s.Run("should prefer BucketAggsForOrderByMetrics over BucketAggs when looking for metrics", func() {
		// Given - Real-world scenario where both fields are present:
		// - BucketAggs points to date_histogram (multi-bucket, no metrics)
		// - BucketAggsForOrderByMetrics points to actual metrics for ordering
		bucketAggsDateHistogram := &ESRespSingleBucket{
			// date_histogram bucket with no metrics
			DocCount: 5,
		}
		orderByMetrics := newESRespMetricsValue(20.0)
		orderByNode := &ESRespSingleBucket{
			MetricsConversationCount: orderByMetrics,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggs:                  bucketAggsDateHistogram,
			BucketAggsForOrderByMetrics: orderByNode,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		// Should use BucketAggsForOrderByMetrics path, not BucketAggs
		s.Equal(orderByMetrics.Value, result.MetricsConversationCount.Value)
	})

	s.Run("should prefer BucketAggs for multi-bucket aggregation even when both present", func() {
		// Given
		buckets := []ESRespSingleBucket{{DocCount: 10}}
		bucketAggsNode := &ESRespSingleBucket{
			Buckets: &buckets,
		}
		orderByNode := &ESRespSingleBucket{
			Buckets: &[]ESRespSingleBucket{{DocCount: 20}},
		}
		rootNode := &ESRespSingleBucket{
			BucketAggs:                  bucketAggsNode,
			BucketAggsForOrderByMetrics: orderByNode,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, multiBucketAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.NotNil(result.Buckets)
		// Should use BucketAggs path for multi-bucket agg
		s.Equal(int32(10), (*result.Buckets)[0].DocCount)
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_WithMetricsDirectly() {
	s.Run("should find metrics at root level", func() {
		// Given
		metricsValue := newESRespMetricsValue(789.0)
		rootNode := &ESRespSingleBucket{
			MetricsConversationCount: metricsValue,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsConversationCount)
	})

	s.Run("should handle any metrics type at root", func() {
		// Given
		metricsValue := newESRespMetricsValue(999.0)
		rootNode := &ESRespSingleBucket{
			MetricsAggOnNumericField: metricsValue,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsAggOnNumericField)
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_WithMultiBucketAgg() {
	s.Run("should find buckets array for multi-bucket aggregation", func() {
		// Given
		buckets := []ESRespSingleBucket{
			{Key: "bucket1", DocCount: 10},
			{Key: "bucket2", DocCount: 20},
		}
		rootNode := &ESRespSingleBucket{
			Buckets: &buckets,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, multiBucketAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.NotNil(result.Buckets)
		s.Len(*result.Buckets, 2)
	})

	s.Run("should traverse to buckets through BucketAggs", func() {
		// Given
		buckets := []ESRespSingleBucket{
			{Key: "bucket1", DocCount: 5},
		}
		innerBucket := &ESRespSingleBucket{
			Buckets: &buckets,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggs: innerBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, multiBucketAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.NotNil(result.Buckets)
		s.Len(*result.Buckets, 1)
	})

	s.Run("should traverse to buckets through BucketAggsForOrderByMetrics", func() {
		// Given
		buckets := []ESRespSingleBucket{
			{Key: "metric_bucket", DocCount: 15},
		}
		innerBucket := &ESRespSingleBucket{
			Buckets: &buckets,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: innerBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, multiBucketAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.NotNil(result.Buckets)
		s.Len(*result.Buckets, 1)
	})
}

func (s *ESResponseTraversalTestSuite) TestEsMetricsValueFromBucket_ConversationCountViaDocCount() {
	s.Run("reads doc_count from date_histogram bucket when metrics_conversation_count is absent", func() {
		// Given
		metricsAggregation := &ChartESAggregation{isConversationCountViaDocCount: true}
		bucket := &ESRespSingleBucket{
			Key:      float64(1772323200000),
			DocCount: 43296,
		}

		// When
		metricValue, err := esMetricsValueFromBucket(bucket, metricsAggregation, 1, elasticsearch.DocumentTypeConversation)

		// Then
		s.Require().NoError(err)
		value, ok := metricValue.numericValue()
		s.Require().True(ok)
		s.Equal(float64(43296), value)
	})

	s.Run("reads doc_count from reverse_nested bucket for nested unique value groupby", func() {
		// Given
		metricsAggregation := &ChartESAggregation{isConversationCountViaDocCount: true}
		bucket := &ESRespSingleBucket{
			Key:      "high_priority",
			DocCount: 120,
			BucketAggs: &ESRespSingleBucket{
				DocCount: 50,
			},
		}

		// When
		metricValue, err := esMetricsValueFromBucket(bucket, metricsAggregation, 1, elasticsearch.DocumentTypeConversation)

		// Then
		s.Require().NoError(err)
		value, ok := metricValue.numericValue()
		s.Require().True(ok)
		s.Equal(float64(50), value)
	})
}

func (s *ESResponseTraversalTestSuite) TestEsMetricsValueWithoutGroupBy_ConversationCountViaDocCount() {
	// Given
	metricsAggregation := &ChartESAggregation{isConversationCountViaDocCount: true}
	esResponse := ChartESResponse{
		Hits: elasticsearch.ResHits{
			Total: elasticsearch.ResTotal{Value: 122340},
		},
	}

	// When
	metricValue, err := esMetricsValueWithoutGroupBy(metricsAggregation, esResponse, elasticsearch.DocumentTypeConversation)

	// Then
	s.Require().NoError(err)
	value, ok := metricValue.numericValue()
	s.Require().True(ok)
	s.Equal(float64(122340), value)
}

func (s *ESResponseTraversalTestSuite) TestEsMetricsValueWithoutGroupBy_ForwardsDocumentType() {
	s.Run("validates missing occurrence count metrics on message documents", func() {
		// Given
		metricsAggregation := &ChartESAggregation{isTotalOccurrenceCount: true}
		esResponse := ChartESResponse{
			Aggregations: &ESRespSingleBucket{DocCount: 10},
		}

		// When
		_, err := esMetricsValueWithoutGroupBy(metricsAggregation, esResponse, elasticsearch.DocumentTypeMessage)

		// Then
		s.Require().Error(err)
		s.Contains(err.Error(), "metrics conversation_message_count should be available")
	})

	s.Run("validates missing occurrence count metrics on conversation documents", func() {
		// Given
		metricsAggregation := &ChartESAggregation{isTotalOccurrenceCount: true}
		esResponse := ChartESResponse{
			Aggregations: &ESRespSingleBucket{DocCount: 10},
		}

		// When
		_, err := esMetricsValueWithoutGroupBy(metricsAggregation, esResponse, elasticsearch.DocumentTypeConversation)

		// Then
		s.Require().Error(err)
		s.Contains(err.Error(), "metrics conversation_count should be available")
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_WithDocCount() {
	s.Run("should use doc_count when no metrics agg present", func() {
		// Given
		rootNode := &ESRespSingleBucket{
			DocCount: 50,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(int32(50), result.DocCount)
	})

	s.Run("should traverse through BucketAggsForOrderByMetrics then use doc_count", func() {
		// Given
		innerBucket := &ESRespSingleBucket{
			DocCount: 75,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: innerBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(int32(75), result.DocCount)
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_ErrorCases() {
	s.Run("should return error when traversal gets stuck", func() {
		// Given - a node with no valid path forward
		rootNode := &ESRespSingleBucket{
			Key: "stuck_node",
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, multiBucketAggType)

		// Then
		s.Require().Error(err)
		s.Nil(result)
		grpcStatus, ok := status.FromError(err)
		s.True(ok)
		s.Equal(codes.Internal, grpcStatus.Code())
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_RealWorldScenario() {
	s.Run("should handle filter bucket with both BucketAggs and BucketAggsForOrderByMetrics", func() {
		// Given - Real-world ES response structure:
		// Filter bucket has:
		// - BucketAggs -> date_histogram (for grouping/breakdown)
		// - BucketAggsForOrderByMetrics -> metrics (for ordering)
		dateHistogramBuckets := []ESRespSingleBucket{
			{Key: "2024-01-01", DocCount: 10},
			{Key: "2024-01-02", DocCount: 15},
		}
		dateHistogramNode := &ESRespSingleBucket{
			Buckets: &dateHistogramBuckets,
		}
		orderByMetricsValue := newESRespMetricsValue(123.45)
		orderByMetricsNode := &ESRespSingleBucket{
			MetricsConversationCount: orderByMetricsValue,
		}
		filterBucket := &ESRespSingleBucket{
			BucketAggs:                  dateHistogramNode,
			BucketAggsForOrderByMetrics: orderByMetricsNode,
		}

		// When - Looking for metrics for ordering
		result, err := traverseToDesiredAggInESResp(filterBucket, metricsAggType)

		// Then - Should find metrics via BucketAggsForOrderByMetrics, not get stuck in date_histogram
		s.Require().NoError(err)
		s.NotNil(result)
		s.NotNil(result.MetricsConversationCount)
		s.Equal(orderByMetricsValue.Value, result.MetricsConversationCount.Value)
	})
}

func (s *ESResponseTraversalTestSuite) TestTraverseToDesiredAggInESResp_MixedTraversal() {
	s.Run("should traverse BucketAggs then BucketAggsForOrderByMetrics", func() {
		// Given
		metricsValue := newESRespMetricsValue(555.0)
		deepestBucket := &ESRespSingleBucket{
			MetricsAggOnNumericField: metricsValue,
		}
		orderByBucket := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: deepestBucket,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggs: orderByBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsAggOnNumericField)
	})

	s.Run("should traverse BucketAggsForOrderByMetrics then BucketAggs", func() {
		// Given
		metricsValue := newESRespMetricsValue(666.0)
		deepestBucket := &ESRespSingleBucket{
			MetricsConversationCount: metricsValue,
		}
		standardBucket := &ESRespSingleBucket{
			BucketAggs: deepestBucket,
		}
		rootNode := &ESRespSingleBucket{
			BucketAggsForOrderByMetrics: standardBucket,
		}

		// When
		result, err := traverseToDesiredAggInESResp(rootNode, metricsAggType)

		// Then
		s.Require().NoError(err)
		s.NotNil(result)
		s.Equal(metricsValue, result.MetricsConversationCount)
	})
}
