package analyticsimpl

import (
	"context"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/google/go-cmp/cmp"
)

// ComboChartLogicTestSuite covers the pure combo-chart helpers (detection, shape validation and
// primary-anchored alignment) plus the infra-metric rejection. These paths operate on protos
// directly and require no DB or Elasticsearch, so they run without the ES test suite.
type ComboChartLogicTestSuite struct {
	suite.Suite
}

func TestComboChartLogic(t *testing.T) {
	suite.Run(t, new(ComboChartLogicTestSuite))
}

func conversationCountMetric() *dashboardpb.ChartMetric {
	return &dashboardpb.ChartMetric{
		Metric: &dashboardpb.ChartMetric_ForConversation_{
			ForConversation: &dashboardpb.ChartMetric_ForConversation{
				StatsType: &dashboardpb.StatsTypeForConversation{
					StatsType: &dashboardpb.StatsTypeForConversation_CommonStatsType{
						CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
					},
				},
			},
		},
	}
}

func infraMetric(id string) *dashboardpb.ChartMetric {
	return &dashboardpb.ChartMetric{
		Metric: &dashboardpb.ChartMetric_ForInfraMetric_{
			ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: id},
		},
	}
}

func requestWithMetrics(metrics ...*dashboardpb.ChartMetric) *analyticspb.EvaluateAnalyticsChartRequest {
	return &analyticspb.EvaluateAnalyticsChartRequest{
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{Metrics: metrics},
		},
	}
}

func timeValue(t time.Time) *dashboardpb.AnalyticsResult_GroupByValue {
	return &dashboardpb.AnalyticsResult_GroupByValue{
		Value: &dashboardpb.ChartAttributeValue{
			Value: &dashboardpb.ChartAttributeValue_Timestamp{Timestamp: timestamppb.New(t)},
		},
	}
}

func usecaseValue(name string) *dashboardpb.AnalyticsResult_GroupByValue {
	return &dashboardpb.AnalyticsResult_GroupByValue{
		Value: &dashboardpb.ChartAttributeValue{
			Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{UsecaseNameValue: name},
		},
	}
}

func singleKey(values ...*dashboardpb.AnalyticsResult_GroupByValue) *dashboardpb.AnalyticsResult_GroupByValuesForSingleKey {
	return &dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{GroupByValues: values}
}

func number(value float32, indexes ...int32) *dashboardpb.AnalyticsResult_AnalyticsNumber {
	return &dashboardpb.AnalyticsResult_AnalyticsNumber{Value: value, GroupByValueIndexes: indexes}
}

func (s *ComboChartLogicTestSuite) TestIsComboChartRequest() {
	s.Run("no metrics is not combo", func() {
		// When
		got := isComboChartRequest(requestWithMetrics())

		// Then
		s.False(got)
	})

	s.Run("single metric is not combo", func() {
		// When
		got := isComboChartRequest(requestWithMetrics(conversationCountMetric()))

		// Then
		s.False(got)
	})

	s.Run("two metrics is combo", func() {
		// When
		got := isComboChartRequest(requestWithMetrics(conversationCountMetric(), conversationCountMetric()))

		// Then
		s.True(got)
	})

	s.Run("three metrics is not combo", func() {
		// When
		got := isComboChartRequest(requestWithMetrics(conversationCountMetric(), conversationCountMetric(), conversationCountMetric()))

		// Then
		s.False(got)
	})
}

func (s *ComboChartLogicTestSuite) TestRequestWithSingleMetric() {
	// Given
	original := requestWithMetrics(conversationCountMetric(), infraMetric("m2"))

	// When
	secondaryReq := requestWithSingleMetric(original, 1)

	// Then
	s.Require().Len(secondaryReq.GetAnalyticsChart().GetDataConfig().GetMetrics(), 1)
	s.Equal("m2", secondaryReq.GetAnalyticsChart().GetDataConfig().GetMetrics()[0].GetForInfraMetric().GetMetricId())
	// The original request must be left untouched (EvaluateAnalyticsChart mutates its request).
	s.Len(original.GetAnalyticsChart().GetDataConfig().GetMetrics(), 2)
}

func limitedGroupBy(limit int32, rankingMetricIdx int32) *dashboardpb.ChartGroupBy {
	return &dashboardpb.ChartGroupBy{
		Config: &dashboardpb.ChartGroupBy_Config{
			SortDirection:      dashboardpb.ChartGroupBy_Config_DESC,
			ValueCountLimit:    limit,
			GroupbyLayer:       dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
			RankingMetricIndex: rankingMetricIdx,
		},
	}
}

// A top-N limit lives on the shared group_by, so leaving it on the non-ranking clone would rank and
// truncate that metric by itself. The two would then select different buckets and
// alignToCanonicalBuckets would zero-fill wherever they disagreed.
func (s *ComboChartLogicTestSuite) TestClearBucketSelection() {
	// Given
	dataConfig := &dashboardpb.ChartDataConfig{
		Metrics: []*dashboardpb.ChartMetric{conversationCountMetric()},
		GroupBy: []*dashboardpb.ChartGroupBy{limitedGroupBy(3, 0)},
	}
	dataConfig.Metrics[0].GroupbyConfig = &dashboardpb.ChartGroupBy_Config{
		SortDirection:   dashboardpb.ChartGroupBy_Config_ASC,
		ValueCountLimit: 5,
	}

	// When
	clearBucketSelection(dataConfig)

	// Then
	groupByConfig := dataConfig.GetGroupBy()[0].GetConfig()
	s.Equal(dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED, groupByConfig.GetSortDirection())
	// The widest set available, not 0 -- 0 means "use the default", which is smaller.
	s.EqualValues(maxGroupByValueCountLimit, groupByConfig.GetValueCountLimit())
	// Everything else about the group-by is untouched.
	s.Equal(dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER, groupByConfig.GetGroupbyLayer())

	// A limit on the metric's own dimension goes too.
	metricConfig := dataConfig.GetMetrics()[0].GetGroupbyConfig()
	s.Equal(dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED, metricConfig.GetSortDirection())
	s.EqualValues(maxGroupByValueCountLimit, metricConfig.GetValueCountLimit())
}

func (s *ComboChartLogicTestSuite) TestResolveRankingMetricIdx() {
	s.Run("defaults to the primary when nothing is limited", func() {
		// When
		got, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{conversationCountMetric(), conversationCountMetric()},
			GroupBy: []*dashboardpb.ChartGroupBy{{}},
		})

		// Then
		s.NoError(err)
		s.Equal(primaryMetricIdx, got)
	})

	s.Run("reads the index off the limited groupby", func() {
		// When
		got, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{conversationCountMetric(), conversationCountMetric()},
			GroupBy: []*dashboardpb.ChartGroupBy{limitedGroupBy(3, secondaryMetricIdx)},
		})

		// Then
		s.NoError(err)
		s.Equal(secondaryMetricIdx, got)
	})

	s.Run("an out of range index is rejected", func() {
		// When
		_, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{conversationCountMetric(), conversationCountMetric()},
			GroupBy: []*dashboardpb.ChartGroupBy{limitedGroupBy(3, 2)},
		})

		// Then
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("a metric dimension limit resolves to that metric", func() {
		// Given: the limit is on the primary metric's own dimension.
		primary := conversationCountMetric()
		primary.GroupbyConfig = &dashboardpb.ChartGroupBy_Config{ValueCountLimit: 2, RankingMetricIndex: primaryMetricIdx}

		// When
		got, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{primary, conversationCountMetric()},
		})

		// Then
		s.NoError(err)
		s.Equal(primaryMetricIdx, got)
	})

	s.Run("ranking a metric dimension by the other metric is rejected", func() {
		// Given: only the primary is grouped by its own dimension, so the secondary has no value to
		// rank those buckets with.
		primary := conversationCountMetric()
		primary.GroupbyConfig = &dashboardpb.ChartGroupBy_Config{ValueCountLimit: 2, RankingMetricIndex: secondaryMetricIdx}

		// When
		_, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{primary, conversationCountMetric()},
		})

		// Then
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	// ranking_metric_index has no field presence, so a client that predates it sends 0. Reading that
	// as "the primary ranks" would reject every top-N on the secondary's own dimension.
	s.Run("an unset index on the secondary's own dimension is allowed", func() {
		// Given
		secondary := conversationCountMetric()
		secondary.GroupbyConfig = &dashboardpb.ChartGroupBy_Config{ValueCountLimit: 2}

		// When
		got, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{conversationCountMetric(), secondary},
		})

		// Then
		s.NoError(err)
		s.Equal(secondaryMetricIdx, got)
	})

	// EvaluateAnalyticsChart swaps the outer groupby to index 0 and sorts there, so the outer entry
	// decides the ranking even when an inner one is listed first.
	s.Run("the outer groupby wins over an earlier inner one", func() {
		// Given
		inner := limitedGroupBy(2, primaryMetricIdx)
		inner.Config.GroupbyLayer = dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_UNSPECIFIED

		// When
		got, err := resolveRankingMetricIdx(&dashboardpb.ChartDataConfig{
			Metrics: []*dashboardpb.ChartMetric{conversationCountMetric(), conversationCountMetric()},
			GroupBy: []*dashboardpb.ChartGroupBy{inner, limitedGroupBy(3, secondaryMetricIdx)},
		})

		// Then
		s.NoError(err)
		s.Equal(secondaryMetricIdx, got)
	})
}

// evaluateComboChart aligns before limiting. Scoring the inner layer first would rank it over
// buckets that alignment is about to discard, and could keep a value that loses in every bucket the
// chart shows. This pins the order those two steps have to run in.
func (s *ComboChartLogicTestSuite) TestInnerLimitIsScoredAgainstAlignedBuckets() {
	// Given: across w1..w3 e0 totals 502, but all of it comes from w3. Within w1, e1 wins 50 to 1.
	primary := &dashboardpb.AnalyticsResult{
		GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
			singleKey(usecaseValue("w1"), usecaseValue("w2"), usecaseValue("w3")),
			singleKey(usecaseValue("e0"), usecaseValue("e1")),
		},
		AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
			number(1, 0, 0), number(50, 0, 1),
			number(1, 1, 0), number(1, 1, 1),
			number(500, 2, 0), number(1, 2, 1),
		},
	}
	// The ranking metric selected w1 only.
	canonical := &dashboardpb.AnalyticsResult{
		GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(usecaseValue("w1"))},
		AnalyticsNumbers:        []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(7, 0)},
	}

	// When: align first, then limit -- the order evaluateComboChart uses.
	aligned := alignToCanonicalBuckets(canonical, primary)
	got := limitInnerGroupByValues(aligned, limitedMetric(1, dashboardpb.ChartGroupBy_Config_DESC))

	// Then: e1 survives, because it is the top emotion in the only week displayed.
	inner := got.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
	s.Require().Len(inner, 1)
	s.Equal("e1", inner[0].GetValue().GetUsecaseNameValue())

	// Limiting first would have kept e0 on the strength of w3, which is not displayed.
	limitedFirst := limitInnerGroupByValues(primary, limitedMetric(1, dashboardpb.ChartGroupBy_Config_DESC))
	s.Equal("e0", limitedFirst.GetGroupByValuesForAllKeys()[1].GetGroupByValues()[0].GetValue().GetUsecaseNameValue())
}

func (s *ComboChartLogicTestSuite) TestValidateComboResultShapes() {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)

	s.Run("case A same single outer layer is valid", func() {
		// Given
		primary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1), timeValue(t2))}}
		secondary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1))}}

		// When
		err := validateComboResultShapes(primary, secondary)

		// Then
		s.NoError(err)
	})

	s.Run("case B one extra inner layer sharing outer is valid", func() {
		// Given
		primary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
			singleKey(timeValue(t1), timeValue(t2)),
			singleKey(usecaseValue("a"), usecaseValue("b")),
		}}
		secondary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1), timeValue(t2))}}

		// When
		err := validateComboResultShapes(primary, secondary)

		// Then
		s.NoError(err)
	})

	s.Run("mismatched outer kind is rejected", func() {
		// Given
		primary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1))}}
		secondary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(usecaseValue("a"))}}

		// When
		err := validateComboResultShapes(primary, secondary)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("more than one layer difference is rejected", func() {
		// Given
		primary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
			singleKey(timeValue(t1)),
			singleKey(usecaseValue("a")),
			singleKey(usecaseValue("b")),
		}}
		secondary := &dashboardpb.AnalyticsResult{GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1))}}

		// When
		err := validateComboResultShapes(primary, secondary)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})
}

func (s *ComboChartLogicTestSuite) TestAlignToCanonicalBuckets() {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	t4 := time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)

	s.Run("scalar secondary is projected onto primary buckets", func() {
		// Given: primary owns the outer bucket set/order [t1, t2, t3].
		primary := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1), timeValue(t2), timeValue(t3))},
			AnalyticsNumbers:        []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(100, 0), number(200, 1), number(300, 2)},
		}
		// Secondary in its own order [t2, t4, t1]: t4 is secondary-only, t3 is missing.
		secondary := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t2), timeValue(t4), timeValue(t1))},
			AnalyticsNumbers:        []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(7, 0), number(9, 1), number(5, 2)},
		}

		// When
		aligned := alignToCanonicalBuckets(primary, secondary)

		// Then: outer layer becomes the primary's; t4 dropped, t3 filled with 0, ordered by primary index.
		want := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1), timeValue(t2), timeValue(t3))},
			AnalyticsNumbers:        []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(5, 0), number(7, 1), number(0, 2)},
		}
		if diff := cmp.Diff(want, aligned, protocmp.Transform()); diff != "" {
			s.Failf("unexpected aligned secondary", "diff (-want +got):\n%s", diff)
		}
	})

	s.Run("secondary inner layer is preserved and never fills missing outer", func() {
		// Given: primary is scalar [t1, t2]; secondary adds an inner usecase layer.
		primary := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{singleKey(timeValue(t1), timeValue(t2))},
			AnalyticsNumbers:        []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(100, 0), number(200, 1)},
		}
		// Secondary outer order [t2, t1] with an inner [a, b] layer; only t2 has data.
		secondary := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
				singleKey(timeValue(t2), timeValue(t1)),
				singleKey(usecaseValue("a"), usecaseValue("b")),
			},
			AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(3, 0, 0), number(4, 0, 1)},
		}

		// When
		aligned := alignToCanonicalBuckets(primary, secondary)

		// Then: outer remapped to primary order/values, inner layer untouched, no zero fill added.
		want := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
				singleKey(timeValue(t1), timeValue(t2)),
				singleKey(usecaseValue("a"), usecaseValue("b")),
			},
			AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(3, 1, 0), number(4, 1, 1)},
		}
		if diff := cmp.Diff(want, aligned, protocmp.Transform()); diff != "" {
			s.Failf("unexpected aligned secondary", "diff (-want +got):\n%s", diff)
		}
	})
}

func (s *ComboChartLogicTestSuite) TestEvaluateComboChartRejectsInfraMetrics() {
	// Given: a two-metric request where the secondary metric is an infra metric. The infra check
	// runs before any evaluation, so a zero-value service is sufficient.
	var service AnalyticsServiceImpl
	req := requestWithMetrics(conversationCountMetric(), infraMetric("cpu"))

	// When
	_, err := service.evaluateComboChart(context.Background(), req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.Unimplemented, status.Code(err))
}

// limitedMetric is a conversation-count metric carrying a top/bottom-N limit on its own dimension.
func limitedMetric(limit int32, direction dashboardpb.ChartGroupBy_Config_SortDirection) *dashboardpb.ChartMetric {
	metric := conversationCountMetric()
	metric.GroupbyConfig = &dashboardpb.ChartGroupBy_Config{SortDirection: direction, ValueCountLimit: limit}
	return metric
}

// averagingMetric is the same but aggregates by average, so its values cannot be summed across
// buckets.
func averagingMetric(limit int32, direction dashboardpb.ChartGroupBy_Config_SortDirection) *dashboardpb.ChartMetric {
	metric := limitedMetric(limit, direction)
	metric.GetForConversation().StatsType = &dashboardpb.StatsTypeForConversation{
		StatsType: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_{
			AggregationOnConversationAttribute: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute{
				AggregationOperator: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
			},
		},
	}
	return metric
}

// Two weeks by three emotions. Totals across the range: e0 = 30, e1 = 3, e2 = 21.
func twoLayerResult() *dashboardpb.AnalyticsResult {
	w1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	w2 := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	return &dashboardpb.AnalyticsResult{
		GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
			singleKey(timeValue(w1), timeValue(w2)),
			singleKey(usecaseValue("e0"), usecaseValue("e1"), usecaseValue("e2")),
		},
		AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
			number(10, 0, 0), number(1, 0, 1), number(20, 0, 2),
			number(20, 1, 0), number(2, 1, 1), number(1, 1, 2),
		},
	}
}

func (s *ComboChartLogicTestSuite) TestLimitInnerGroupByValues() {
	s.Run("top N is chosen by total across the range and kept in every bucket", func() {
		// When
		got := limitInnerGroupByValues(twoLayerResult(), limitedMetric(2, dashboardpb.ChartGroupBy_Config_DESC))

		// Then: e1 (total 3) is dropped; e0 and e2 survive in both weeks.
		inner := got.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
		s.Require().Len(inner, 2)
		s.Equal("e0", inner[0].GetValue().GetUsecaseNameValue())
		s.Equal("e2", inner[1].GetValue().GetUsecaseNameValue())

		// Indexes are remapped onto the shortened layer, and the outer layer is untouched.
		s.Len(got.GetGroupByValuesForAllKeys()[0].GetGroupByValues(), 2)
		want := []*dashboardpb.AnalyticsResult_AnalyticsNumber{
			number(10, 0, 0), number(20, 0, 1),
			number(20, 1, 0), number(1, 1, 1),
		}
		if diff := cmp.Diff(want, got.GetAnalyticsNumbers(), protocmp.Transform()); diff != "" {
			s.Failf("unexpected numbers", "(-want +got):\n%s", diff)
		}
	})

	s.Run("bottom N takes the smallest totals", func() {
		// When
		got := limitInnerGroupByValues(twoLayerResult(), limitedMetric(1, dashboardpb.ChartGroupBy_Config_ASC))

		// Then
		inner := got.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
		s.Require().Len(inner, 1)
		s.Equal("e1", inner[0].GetValue().GetUsecaseNameValue())
	})

	s.Run("a limit at or above the value count changes nothing", func() {
		// Given
		original := twoLayerResult()

		// When
		got := limitInnerGroupByValues(original, limitedMetric(3, dashboardpb.ChartGroupBy_Config_DESC))

		// Then
		s.Same(original, got)
	})

	s.Run("no config and no limit are both no-ops", func() {
		original := twoLayerResult()
		s.Same(original, limitInnerGroupByValues(original, conversationCountMetric()))
		s.Same(original, limitInnerGroupByValues(original, limitedMetric(0, dashboardpb.ChartGroupBy_Config_DESC)))
	})

	s.Run("a single-layer result is untouched because ES already applied the limit", func() {
		// Given: the dimension was the outer groupby, so there is no inner layer to trim.
		original := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
				singleKey(usecaseValue("e0"), usecaseValue("e1")),
			},
			AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{number(10, 0), number(5, 1)},
		}

		// When
		got := limitInnerGroupByValues(original, limitedMetric(1, dashboardpb.ChartGroupBy_Config_DESC))

		// Then
		s.Same(original, got)
	})

	// Summing per-bucket averages would rank an emotion averaging 40 across three buckets above one
	// averaging 100 in a single bucket.
	s.Run("an averaging metric ranks by mean, not by total", func() {
		// Given
		res := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
				singleKey(usecaseValue("w1"), usecaseValue("w2"), usecaseValue("w3")),
				singleKey(usecaseValue("e0"), usecaseValue("e1")),
			},
			AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
				number(100, 0, 0),
				number(40, 0, 1), number(40, 1, 1), number(40, 2, 1),
			},
		}

		// When
		got := limitInnerGroupByValues(res, averagingMetric(1, dashboardpb.ChartGroupBy_Config_DESC))

		// Then
		inner := got.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
		s.Require().Len(inner, 1)
		s.Equal("e0", inner[0].GetValue().GetUsecaseNameValue())
	})

	s.Run("a negative inner index is skipped rather than panicking", func() {
		// Given
		res := twoLayerResult()
		res.AnalyticsNumbers = append(res.AnalyticsNumbers, number(5, 0, -1))

		// When / Then: must not panic.
		got := limitInnerGroupByValues(res, limitedMetric(2, dashboardpb.ChartGroupBy_Config_DESC))
		s.Len(got.GetGroupByValuesForAllKeys()[1].GetGroupByValues(), 2)
	})

	// After alignment drops buckets an inner value can have no rows left. Its score is 0, which under
	// ASC would beat every real value and put an empty series on the chart.
	s.Run("bottom N ignores inner values with no rows", func() {
		// Given: e2 only ever had data in a bucket that alignment removed.
		res := &dashboardpb.AnalyticsResult{
			GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
				singleKey(usecaseValue("w1")),
				singleKey(usecaseValue("e0"), usecaseValue("e1"), usecaseValue("e2")),
			},
			AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
				number(10, 0, 0), number(20, 0, 1),
			},
		}

		// When
		got := limitInnerGroupByValues(res, limitedMetric(1, dashboardpb.ChartGroupBy_Config_ASC))

		// Then: e0, the genuine lowest, not the empty e2.
		inner := got.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
		s.Require().Len(inner, 1)
		s.Equal("e0", inner[0].GetValue().GetUsecaseNameValue())
		s.Len(got.GetAnalyticsNumbers(), 1)
	})

	s.Run("the caller's result is not mutated", func() {
		// Given
		original := twoLayerResult()

		// When
		limitInnerGroupByValues(original, limitedMetric(2, dashboardpb.ChartGroupBy_Config_DESC))

		// Then
		s.Len(original.GetGroupByValuesForAllKeys()[1].GetGroupByValues(), 3)
		s.Equal([]int32{0, 2}, original.GetAnalyticsNumbers()[2].GetGroupByValueIndexes())
	})
}
