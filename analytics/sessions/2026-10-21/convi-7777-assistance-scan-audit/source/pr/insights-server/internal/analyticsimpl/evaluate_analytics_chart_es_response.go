package analyticsimpl

import (
	"fmt"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
)

type ChartESResponse struct {
	Aggregations *ESRespSingleBucket   `mapstructure:"aggregations,omitempty"`
	Hits         elasticsearch.ResHits `mapstructure:"hits"`
}

// A response from a single bucket.
// Note the whole aggregation can be seen as a single bucket,
// and ES bucket aggregation (aka breakdown) is optional in general.
// TODO(zhucaiyi): nits: rename this to ESRespAggregation.
type ESRespSingleBucket struct {
	// Response for single-bucket aggs.
	BucketAggs *ESRespSingleBucket `mapstructure:"bucket_aggs,omitempty"`

	// Bucket aggregation for orderby metrics only.
	BucketAggsForOrderByMetrics *ESRespSingleBucket `mapstructure:"bucket_aggs_for_order_by_metrics,omitempty"`

	// Multi-bucket aggregation.
	Buckets *[]ESRespSingleBucket `mapstructure:"buckets,omitempty"`

	// Metrics aggs.
	MetricsConversationCount        *ESRespMetricsValue `mapstructure:"metrics_conversation_count,omitempty"`
	MetricsConversationMessageCount *ESRespMetricsValue `mapstructure:"metrics_conversation_message_count,omitempty"`
	MetricsBucketFieldCardinality   *ESRespMetricsValue `mapstructure:"metrics_bucket_field_cardinality,omitempty"`
	MetricsAggOnNumericField        *ESRespMetricsValue `mapstructure:"metrics_agg_on_numeric_field,omitempty"`

	DocCount int32 `mapstructure:"doc_count"`
	// Key can be in the form of string|number|boolean|nil
	// - number: group by metadata value and group by time.
	// - string: group by string type metadata value
	// - boolean: group by boolean type metadata value
	// - nil: not populated when group by provided value
	Key any `mapstructure:"key,omitempty"`

	AfterKey *BucketAggsKey `mapstructure:"after_key,omitempty"`
}

type ESRespMetricsValue struct {
	Value  *float64           `mapstructure:"value"`
	Values map[string]float64 `mapstructure:"values"`
}

func newESRespMetricsValue(value float64) *ESRespMetricsValue {
	return &ESRespMetricsValue{Value: &value}
}

func (v *ESRespMetricsValue) numericValue() (float64, bool) {
	if v == nil || v.Value == nil {
		return 0, false
	}
	return *v.Value, true
}

func esMetricsValueFromBucket(
	bucket *ESRespSingleBucket,
	metricsAggregation *ChartESAggregation,
	groupByCount int,
	documentType elasticsearch.DocumentType,
) (*ESRespMetricsValue, error) {
	resolvedBucket, err := traverseToDesiredAggInESResp(bucket, metricsAggType)
	if err != nil {
		return nil, err
	}

	var metricValue *ESRespMetricsValue
	switch {
	case metricsAggregation.isConversationCountViaDocCount:
		metricValue = newESRespMetricsValue(float64(resolvedBucket.DocCount))
	case metricsAggregation.MetricsConversationCount != nil:
		if metricsAggregation.isAverageOccurrenceCountPerConvo {
			convoCount, ok := resolvedBucket.MetricsConversationCount.numericValue()
			if !ok {
				return nil, fmt.Errorf("metrics conversation_count should have a value")
			}
			occurrenceCount := resolvedBucket.DocCount
			if groupByCount == 0 && documentType == elasticsearch.DocumentTypeMessage {
				if messageCount := resolvedBucket.MetricsConversationMessageCount; messageCount != nil {
					messageCountValue, ok := messageCount.numericValue()
					if !ok {
						return nil, fmt.Errorf("metrics conversation_message_count should have a value")
					}
					occurrenceCount = int32(messageCountValue)
				} else {
					return nil, fmt.Errorf("metrics conversation_message_count should be available")
				}
			}
			metricValue = newESRespMetricsValue(safeDivideFloat64(float64(occurrenceCount), convoCount))
		} else {
			metricValue = resolvedBucket.MetricsConversationCount
		}
	case metricsAggregation.MetricsAggOnNumericField != nil:
		metricValue = resolvedBucket.MetricsAggOnNumericField
	case metricsAggregation.isTotalOccurrenceCount:
		if groupByCount == 0 {
			if metricsAggregation.MetricsConversationMessageCount == nil && (documentType == elasticsearch.DocumentTypeMessage || documentType == elasticsearch.DocumentTypeMessageGroup) {
				return nil, fmt.Errorf("metrics conversation_message_count should be available")
			}
			if metricsAggregation.MetricsConversationCount == nil && documentType == elasticsearch.DocumentTypeConversation {
				return nil, fmt.Errorf("metrics conversation_count should be available")
			}
		}
		switch {
		case resolvedBucket.MetricsConversationMessageCount != nil:
			metricValue = resolvedBucket.MetricsConversationMessageCount
		case resolvedBucket.MetricsConversationCount != nil:
			metricValue = resolvedBucket.MetricsConversationCount
		default:
			metricValue = newESRespMetricsValue(float64(resolvedBucket.DocCount))
		}
	}
	if metricValue == nil {
		return nil, fmt.Errorf("can not find metricValue")
	}
	return metricValue, nil
}

func esMetricsValueWithoutGroupBy(metricsAggregation *ChartESAggregation, esResponse ChartESResponse, documentType elasticsearch.DocumentType) (*ESRespMetricsValue, error) {
	if metricsAggregation.isConversationCountViaDocCount {
		return newESRespMetricsValue(float64(esResponse.Hits.Total.Value)), nil
	}
	if esResponse.Aggregations == nil {
		return nil, fmt.Errorf("can not find metricValue")
	}
	return esMetricsValueFromBucket(esResponse.Aggregations, metricsAggregation, 0, documentType)
}
