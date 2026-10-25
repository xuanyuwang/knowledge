package analyticsimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	npconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	metricspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metrics"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	"github.com/cresta/go-servers/shared/prometheus"
	prometheusmocks "github.com/cresta/go-servers/shared/prometheus/mocks"
	testingmock "github.com/cresta/go-servers/shared/testing/mock"
	log "github.com/cresta/shared-go/framework/log"
	serverauth "github.com/cresta/shared-go/framework/server/auth"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// infraMetricsTestSuite covers both the pure helper functions and the
// evaluateInfraMetricsChart method. No database or gRPC server is needed.
type infraMetricsTestSuite struct {
	suite.Suite
	ctx              context.Context
	mockPromClient   *prometheusmocks.MockClient
	mockConfigClient *mockconfig.MockClient
	svc              AnalyticsServiceImpl
}

func TestInfraMetrics(t *testing.T) { suite.Run(t, new(infraMetricsTestSuite)) }

func (s *infraMetricsTestSuite) SetupTest() {
	s.ctx = context.Background()
	ctrl := gomock.NewController(testingmock.NewTestReporter(s.T()))
	s.mockPromClient = prometheusmocks.NewMockClient(ctrl)
	s.mockConfigClient = mockconfig.NewMockClient(gomock.NewController(testingmock.NewTestReporter(s.T())))
	s.svc = AnalyticsServiceImpl{
		prometheusClient: s.mockPromClient,
		configClient:     s.mockConfigClient,
		logger:           log.NoOpsLogger,
	}
}

// ---------------------------------------------------------------------------
// validateInfraMetricsFilters
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestValidateInfraMetricsFilters_EmptyFilters() {
	// When
	err := validateInfraMetricsFilters(nil)

	// Then
	s.Require().NoError(err)
}

func (s *infraMetricsTestSuite) TestValidateInfraMetricsFilters_AllowedTypes() {
	s.Run("time range filter value", func() {
		// Given
		filters := []*dashboardpb.ChartDataFilter{
			{ValueSet: &dashboardpb.ChartValueSet{
				Values: &dashboardpb.ChartValueSet_SingleValues{
					SingleValues: &dashboardpb.ChartValueSet_Single{
						AttributeValues: []*dashboardpb.ChartAttributeValue{
							{Value: &dashboardpb.ChartAttributeValue_TimeRangeFilter{}},
						},
					},
				},
			}},
		}

		// When
		err := validateInfraMetricsFilters(filters)

		// Then
		s.Require().NoError(err)
	})

	s.Run("duration value", func() {
		// Given
		filters := []*dashboardpb.ChartDataFilter{
			{ValueSet: &dashboardpb.ChartValueSet{
				Values: &dashboardpb.ChartValueSet_SingleValues{
					SingleValues: &dashboardpb.ChartValueSet_Single{
						AttributeValues: []*dashboardpb.ChartAttributeValue{
							{Value: &dashboardpb.ChartAttributeValue_Duration{}},
						},
					},
				},
			}},
		}

		// When
		err := validateInfraMetricsFilters(filters)

		// Then
		s.Require().NoError(err)
	})

	s.Run("timestamp range value", func() {
		// Given
		filters := []*dashboardpb.ChartDataFilter{
			{ValueSet: &dashboardpb.ChartValueSet{
				Values: &dashboardpb.ChartValueSet_RangeValues{
					RangeValues: &dashboardpb.ChartValueSet_Range{
						RangeValues: []*dashboardpb.ChartValueRange{
							{FromValue: &dashboardpb.ChartAttributeValue{
								Value: &dashboardpb.ChartAttributeValue_Timestamp{},
							}},
						},
					},
				},
			}},
		}

		// When
		err := validateInfraMetricsFilters(filters)

		// Then
		s.Require().NoError(err)
	})

	s.Run("relative timestamp range value", func() {
		// Given
		filters := []*dashboardpb.ChartDataFilter{
			{ValueSet: &dashboardpb.ChartValueSet{
				Values: &dashboardpb.ChartValueSet_RangeValues{
					RangeValues: &dashboardpb.ChartValueSet_Range{
						RangeValues: []*dashboardpb.ChartValueRange{
							{FromValue: &dashboardpb.ChartAttributeValue{
								Value: &dashboardpb.ChartAttributeValue_RelativeTimestamp{},
							}},
						},
					},
				},
			}},
		}

		// When
		err := validateInfraMetricsFilters(filters)

		// Then
		s.Require().NoError(err)
	})
}

func (s *infraMetricsTestSuite) TestValidateInfraMetricsFilters_RejectsStringValue() {
	// Given
	filters := []*dashboardpb.ChartDataFilter{
		{ValueSet: &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{Value: &dashboardpb.ChartAttributeValue_StringValue{StringValue: "bad"}},
					},
				},
			},
		}},
	}

	// When
	err := validateInfraMetricsFilters(filters)

	// Then
	s.Require().Error(err)
	s.Equal(codes.Unimplemented, status.Code(err))
}

// ---------------------------------------------------------------------------
// infraMetricsIntervalToDuration
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestInfraMetricsIntervalToDuration() {
	s.Run("HOUR", func() {
		// When
		result, err := infraMetricsIntervalToDuration(dashboardpb.ChartGroupBy_Time_HOUR)

		// Then
		s.Require().NoError(err)
		s.Equal(time.Hour, result)
	})

	s.Run("DAY", func() {
		// When
		result, err := infraMetricsIntervalToDuration(dashboardpb.ChartGroupBy_Time_DAY)

		// Then
		s.Require().NoError(err)
		s.Equal(24*time.Hour, result)
	})

	s.Run("WEEK", func() {
		// When
		result, err := infraMetricsIntervalToDuration(dashboardpb.ChartGroupBy_Time_WEEK)

		// Then
		s.Require().NoError(err)
		s.Equal(7*24*time.Hour, result)
	})

	s.Run("MONTH", func() {
		// When
		_, err := infraMetricsIntervalToDuration(dashboardpb.ChartGroupBy_Time_MONTH)

		// Then
		s.Equal(codes.Unimplemented, status.Code(err))
	})

	s.Run("unknown defaults to DAY", func() {
		// When
		result, err := infraMetricsIntervalToDuration(dashboardpb.ChartGroupBy_Time_INTERVAL_UNSPECIFIED)

		// Then
		s.Require().NoError(err)
		s.Equal(24*time.Hour, result)
	})
}

// ---------------------------------------------------------------------------
// infraBuildLabelKeyValues
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestInfraBuildLabelKeyValues_EmptySeries() {
	// When
	values, indexOf := infraBuildLabelKeyValues(nil, []string{"env"})

	// Then
	s.Len(values, 1)
	s.Empty(values[0])
	s.Len(indexOf, 1)
	s.Empty(indexOf[0])
}

func (s *infraMetricsTestSuite) TestInfraBuildLabelKeyValues_DeduplicatesValues() {
	// Given
	series := []map[string]string{
		{"env": "prod", "region": "us-east"},
		{"env": "staging", "region": "us-east"},
		{"env": "prod", "region": "eu-west"},
	}

	// When
	values, indexOf := infraBuildLabelKeyValues(series, []string{"env", "region"})

	// Then
	s.Equal([]string{"prod", "staging"}, values[0])
	s.Equal([]string{"us-east", "eu-west"}, values[1])
	s.Equal(int32(0), indexOf[0]["prod"])
	s.Equal(int32(1), indexOf[0]["staging"])
	s.Equal(int32(0), indexOf[1]["us-east"])
	s.Equal(int32(1), indexOf[1]["eu-west"])
}

// ---------------------------------------------------------------------------
// infraMetricsVectorToAnalyticsResponse
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestInfraMetricsVectorToAnalyticsResponse_UngroupedMultipleSeries() {
	// Given
	vectors := []prometheus.VectorResult{
		{Metric: map[string]string{}, Value: []any{float64(1000000), "10.0"}},
		{Metric: map[string]string{}, Value: []any{float64(1000000), "20.0"}},
	}

	// When
	_, err := infraMetricsVectorToAnalyticsResponse(vectors, nil)

	// Then
	s.Require().Error(err)
	s.Equal(codes.Internal, status.Code(err))
}

func (s *infraMetricsTestSuite) TestInfraMetricsVectorToAnalyticsResponse_NoLabels() {
	s.Run("no vectors returns zero value", func() {
		// When
		resp, err := infraMetricsVectorToAnalyticsResponse(nil, nil)

		// Then
		s.Require().NoError(err)
		results := resp.GetResultForChart().GetAnalyticsResults()
		s.Require().Len(results, 1)
		s.Require().Len(results[0].GetAnalyticsNumbers(), 1)
		s.Equal(float32(0), results[0].GetAnalyticsNumbers()[0].GetValue())
	})

	s.Run("single vector returns its value", func() {
		// Given
		vectors := []prometheus.VectorResult{
			{Metric: map[string]string{}, Value: []any{float64(1000000), "42.5"}},
		}

		// When
		resp, err := infraMetricsVectorToAnalyticsResponse(vectors, nil)

		// Then
		s.Require().NoError(err)
		results := resp.GetResultForChart().GetAnalyticsResults()
		s.Require().Len(results, 1)
		s.Require().Len(results[0].GetAnalyticsNumbers(), 1)
		s.InDelta(42.5, results[0].GetAnalyticsNumbers()[0].GetValue(), 0.001)
	})
}

func (s *infraMetricsTestSuite) TestInfraMetricsVectorToAnalyticsResponse_WithLabels() {
	// Given
	vectors := []prometheus.VectorResult{
		{Metric: map[string]string{"env": "prod"}, Value: []any{float64(1000000), "10.0"}},
		{Metric: map[string]string{"env": "staging"}, Value: []any{float64(1000000), "5.0"}},
	}
	labelKeys := []string{"env"}

	// When
	resp, err := infraMetricsVectorToAnalyticsResponse(vectors, labelKeys)

	// Then
	s.Require().NoError(err)
	results := resp.GetResultForChart().GetAnalyticsResults()
	s.Require().Len(results, 1)
	result := results[0]
	s.Require().Len(result.GetGroupByValuesForAllKeys(), 1)

	groupByValues := result.GetGroupByValuesForAllKeys()[0].GetGroupByValues()
	s.Require().Len(groupByValues, 2)
	s.Equal("prod", groupByValues[0].GetValue().GetStringValue())
	s.Equal("staging", groupByValues[1].GetValue().GetStringValue())

	analyticsNumbers := result.GetAnalyticsNumbers()
	s.Require().Len(analyticsNumbers, 2)
	s.InDelta(10.0, analyticsNumbers[0].GetValue(), 0.001)
	s.Equal([]int32{0}, analyticsNumbers[0].GetGroupByValueIndexes())
	s.InDelta(5.0, analyticsNumbers[1].GetValue(), 0.001)
	s.Equal([]int32{1}, analyticsNumbers[1].GetGroupByValueIndexes())
}

// ---------------------------------------------------------------------------
// infraMetricsMatrixToAnalyticsResponse
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestInfraMetricsMatrixToAnalyticsResponse_EmptyMatrix() {
	// When
	resp, err := infraMetricsMatrixToAnalyticsResponse(nil, nil)

	// Then
	s.Require().NoError(err)
	results := resp.GetResultForChart().GetAnalyticsResults()
	s.Require().Len(results, 1)
	s.Empty(results[0].GetAnalyticsNumbers())
}

func (s *infraMetricsTestSuite) TestInfraMetricsMatrixToAnalyticsResponse_TimestampsOnly() {
	// Given
	ts1 := float64(1000000)
	ts2 := float64(1003600)
	matrices := []prometheus.MatrixResult{
		{
			Metric: map[string]string{},
			Values: [][]any{
				{ts1, "10.0"},
				{ts2, "20.0"},
			},
		},
	}

	// When
	resp, err := infraMetricsMatrixToAnalyticsResponse(matrices, nil)

	// Then
	s.Require().NoError(err)
	results := resp.GetResultForChart().GetAnalyticsResults()
	s.Require().Len(results, 1)
	result := results[0]

	groupByAllKeys := result.GetGroupByValuesForAllKeys()
	s.Require().Len(groupByAllKeys, 1, "expected one timestamp groupby dimension")
	s.Require().Len(groupByAllKeys[0].GetGroupByValues(), 2)
	s.Equal(
		time.Unix(int64(ts1), 0).UTC(),
		groupByAllKeys[0].GetGroupByValues()[0].GetValue().GetTimestamp().AsTime().UTC(),
	)
	s.Equal(
		time.Unix(int64(ts2), 0).UTC(),
		groupByAllKeys[0].GetGroupByValues()[1].GetValue().GetTimestamp().AsTime().UTC(),
	)

	numbers := result.GetAnalyticsNumbers()
	s.Require().Len(numbers, 2)
	s.InDelta(10.0, numbers[0].GetValue(), 0.001)
	s.Equal([]int32{0}, numbers[0].GetGroupByValueIndexes())
	s.InDelta(20.0, numbers[1].GetValue(), 0.001)
	s.Equal([]int32{1}, numbers[1].GetGroupByValueIndexes())
}

func (s *infraMetricsTestSuite) TestInfraMetricsMatrixToAnalyticsResponse_WithLabelKeys() {
	// Given
	ts := float64(1000000)
	matrices := []prometheus.MatrixResult{
		{Metric: map[string]string{"env": "prod"}, Values: [][]any{{ts, "10.0"}}},
		{Metric: map[string]string{"env": "staging"}, Values: [][]any{{ts, "5.0"}}},
	}

	// When
	resp, err := infraMetricsMatrixToAnalyticsResponse(matrices, []string{"env"})

	// Then
	s.Require().NoError(err)
	result := resp.GetResultForChart().GetAnalyticsResults()[0]

	// First dimension = timestamps, second = env label
	groupByAllKeys := result.GetGroupByValuesForAllKeys()
	s.Require().Len(groupByAllKeys, 2)
	s.Require().Len(groupByAllKeys[0].GetGroupByValues(), 1, "one timestamp")
	s.Require().Len(groupByAllKeys[1].GetGroupByValues(), 2, "two env values")
	s.Equal("prod", groupByAllKeys[1].GetGroupByValues()[0].GetValue().GetStringValue())
	s.Equal("staging", groupByAllKeys[1].GetGroupByValues()[1].GetValue().GetStringValue())

	numbers := result.GetAnalyticsNumbers()
	s.Require().Len(numbers, 2)
	// indexes: [tsIdx, labelIdx]
	s.Equal([]int32{0, 0}, numbers[0].GetGroupByValueIndexes())
	s.InDelta(10.0, numbers[0].GetValue(), 0.001)
	s.Equal([]int32{0, 1}, numbers[1].GetGroupByValueIndexes())
	s.InDelta(5.0, numbers[1].GetValue(), 0.001)
}

// ---------------------------------------------------------------------------
// evaluateInfraMetricsChart — error paths
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_NilPrometheusClient() {
	// Given
	svcWithoutProm := AnalyticsServiceImpl{prometheusClient: nil, configClient: s.mockConfigClient}
	req := s.makeInstantQueryRequest("my_metric", nil)

	// When
	_, err := svcWithoutProm.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.Unavailable, status.Code(err))
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_ValidationErrors() {
	s.Run("no metrics", func() {
		// Given
		req := &analyticspb.EvaluateAnalyticsChartRequest{
			Parent: "customers/customer_id/profiles/profile_id",
			AnalyticsChart: &dashboardpb.AnalyticsChart{
				DataConfig: &dashboardpb.ChartDataConfig{Metrics: nil},
			},
		}

		// When
		_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("more than one metric", func() {
		// Given
		req := &analyticspb.EvaluateAnalyticsChartRequest{
			Parent: "customers/customer_id/profiles/profile_id",
			AnalyticsChart: &dashboardpb.AnalyticsChart{
				DataConfig: &dashboardpb.ChartDataConfig{
					Metrics: []*dashboardpb.ChartMetric{
						{Metric: &dashboardpb.ChartMetric_ForInfraMetric_{ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: "m1"}}},
						{Metric: &dashboardpb.ChartMetric_ForInfraMetric_{ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: "m2"}}},
					},
				},
			},
		}

		// When
		_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("non-infra metric type", func() {
		// Given
		req := &analyticspb.EvaluateAnalyticsChartRequest{
			Parent: "customers/customer_id/profiles/profile_id",
			AnalyticsChart: &dashboardpb.AnalyticsChart{
				DataConfig: &dashboardpb.ChartDataConfig{
					Metrics: []*dashboardpb.ChartMetric{
						{}, // GetForInfraMetric() == nil
					},
				},
			},
		}

		// When
		_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_InvertedTimeRange() {
	// Given
	profileConfig := s.makeProfileConfig(map[string]string{
		"my_metric": "sum(increase(counter[{{duration}}]))",
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)
	startTime := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // end before start
	req := s.makeInstantQueryRequestWithTimeRange("my_metric", nil, startTime, endTime)

	// When
	_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_MetricNotFound() {
	// Given
	profileConfig := s.makeProfileConfig(map[string]string{})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)
	req := s.makeInstantQueryRequest("nonexistent_metric", nil)

	// When
	_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.NotFound, status.Code(err))
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_MissingTimeRange() {
	// Given
	profileConfig := s.makeProfileConfig(map[string]string{
		"my_metric": "sum(increase(counter[{{duration}}]))",
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)
	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: "customers/customer_id/profiles/profile_id",
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{Metric: &dashboardpb.ChartMetric_ForInfraMetric_{
						ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: "my_metric"},
					}},
				},
				// No filters → no time range extracted
			},
		},
	}

	// When
	_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_UnsupportedFilter() {
	// Given
	profileConfig := s.makeProfileConfig(map[string]string{
		"my_metric": "sum(increase(counter[{{duration}}]))",
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)
	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: "customers/customer_id/profiles/profile_id",
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{Metric: &dashboardpb.ChartMetric_ForInfraMetric_{
						ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: "my_metric"},
					}},
				},
				Filters: []*dashboardpb.ChartDataFilter{
					{ValueSet: &dashboardpb.ChartValueSet{
						Values: &dashboardpb.ChartValueSet_SingleValues{
							SingleValues: &dashboardpb.ChartValueSet_Single{
								AttributeValues: []*dashboardpb.ChartAttributeValue{
									{Value: &dashboardpb.ChartAttributeValue_StringValue{StringValue: "unsupported"}},
								},
							},
						},
					}},
				},
			},
		},
	}

	// When
	_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.Unimplemented, status.Code(err))
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_UnsupportedGroupBy() {
	// Given
	profileConfig := s.makeProfileConfig(map[string]string{
		"my_metric": "sum(increase(counter[{{duration}}])) by ({{group_by}})",
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)
	req := s.makeInstantQueryRequest("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
			ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{},
		}},
	})

	// When
	_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.Unimplemented, status.Code(err))
}

// ---------------------------------------------------------------------------
// evaluateInfraMetricsChart — happy paths
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_InstantQueryNoGroupBy() {
	// Given
	const promqlTemplate = `sum(increase(counter{customer="{{customer_id}}",profile="{{profile_id}}"}[{{duration}}]))`
	profileConfig := s.makeProfileConfig(map[string]string{"my_metric": promqlTemplate})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	// 604800s = 7 days (Jan 1 → Jan 8).
	const expectedQuery = `sum(increase(counter{customer="customer_id",profile="profile_id"}[604800s]))`

	vectorJSON, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{}, "value": []any{float64(1704067200), "99.5"}},
	})
	// Verify the PromQL query string sent to Prometheus matches the expected query.
	s.mockPromClient.EXPECT().
		Query(gomock.Any(), expectedQuery, gomock.Any()).
		Return(&prometheus.QueryResult{
			Status: "success",
			Data:   prometheus.QueryData{ResultType: "vector", Result: vectorJSON},
		}, nil)

	req := s.makeInstantQueryRequestWithTimeRange("my_metric", nil, startTime, endTime)

	// When
	resp, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().NoError(err)
	results := resp.GetResultForChart().GetAnalyticsResults()
	s.Require().Len(results, 1)
	s.Require().Len(results[0].GetAnalyticsNumbers(), 1)
	s.InDelta(99.5, results[0].GetAnalyticsNumbers()[0].GetValue(), 0.001)
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_LabelKeyNotConfigured() {
	// Given
	profileConfig := s.makeProfileConfigFromDefs(map[string]*configpb.MetricDefinition{
		"my_metric": {
			PromqlQuery: "sum(rate(counter[{{duration}}]))",
			Metric:      &metricspb.Metric{LabelKeys: []string{"env"}},
		},
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)
	req := s.makeInstantQueryRequest("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByInfraLabelKeys_{
			ByInfraLabelKeys: &dashboardpb.ChartGroupBy_ByInfraLabelKeys{LabelKeys: []string{"region"}},
		}},
	})

	// When
	_, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().Error(err)
	s.Equal(codes.InvalidArgument, status.Code(err))
	s.Contains(err.Error(), "region")
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_InstantQueryWithLabelGroupBy() {
	// Given
	const promqlTemplate = `sum(increase(counter{customer="{{customer_id}}"}[{{duration}}])) by ({{group_by}})`
	profileConfig := s.makeProfileConfigFromDefs(map[string]*configpb.MetricDefinition{
		"my_metric": {PromqlQuery: promqlTemplate, Metric: &metricspb.Metric{LabelKeys: []string{"env"}}},
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)

	vectorJSON, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{"env": "prod"}, "value": []any{float64(1704067200), "10.0"}},
		{"metric": map[string]string{"env": "staging"}, "value": []any{float64(1704067200), "5.0"}},
	})
	s.mockPromClient.EXPECT().
		Query(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&prometheus.QueryResult{
			Status: "success",
			Data:   prometheus.QueryData{ResultType: "vector", Result: vectorJSON},
		}, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	req := s.makeInstantQueryRequestWithTimeRange("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByInfraLabelKeys_{
			ByInfraLabelKeys: &dashboardpb.ChartGroupBy_ByInfraLabelKeys{LabelKeys: []string{"env"}},
		}},
	}, startTime, startTime.Add(24*time.Hour))

	// When
	resp, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().NoError(err)
	result := resp.GetResultForChart().GetAnalyticsResults()[0]
	s.Require().Len(result.GetGroupByValuesForAllKeys(), 1)
	groupByValues := result.GetGroupByValuesForAllKeys()[0].GetGroupByValues()
	s.Require().Len(groupByValues, 2)
	s.Equal("prod", groupByValues[0].GetValue().GetStringValue())
	s.Equal("staging", groupByValues[1].GetValue().GetStringValue())
	s.Require().Len(result.GetAnalyticsNumbers(), 2)
	s.InDelta(10.0, result.GetAnalyticsNumbers()[0].GetValue(), 0.001)
	s.InDelta(5.0, result.GetAnalyticsNumbers()[1].GetValue(), 0.001)
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_RangeQueryByTime() {
	// Given
	const promqlTemplate = `sum(increase(counter[{{duration}}])) by ({{group_by}})`
	profileConfig := s.makeProfileConfig(map[string]string{"my_metric": promqlTemplate})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)

	ts1 := float64(1704067200)
	ts2 := float64(1704153600)
	matrixJSON, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{}, "values": []any{
			[]any{ts1, "10.0"},
			[]any{ts2, "20.0"},
		}},
	})
	s.mockPromClient.EXPECT().
		QueryRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 24*time.Hour).
		Return(&prometheus.QueryResult{
			Status: "success",
			Data:   prometheus.QueryData{ResultType: "matrix", Result: matrixJSON},
		}, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	req := s.makeInstantQueryRequestWithTimeRange("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
			ByTime: &dashboardpb.ChartGroupBy_Time{
				Interval: dashboardpb.ChartGroupBy_Time_DAY,
			},
		}},
	}, startTime, endTime)

	// When
	resp, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().NoError(err)
	want := &analyticspb.EvaluateAnalyticsChartResponse{
		ResultForChart: &dashboardpb.AnalyticsResultForChart{
			AnalyticsResults: []*dashboardpb.AnalyticsResult{
				{
					GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
						{GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
							{Value: &dashboardpb.ChartAttributeValue{Value: &dashboardpb.ChartAttributeValue_Timestamp{
								Timestamp: timestamppb.New(time.Unix(int64(ts1), 0)),
							}}},
							{Value: &dashboardpb.ChartAttributeValue{Value: &dashboardpb.ChartAttributeValue_Timestamp{
								Timestamp: timestamppb.New(time.Unix(int64(ts2), 0)),
							}}},
						}},
					},
					AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
						{Value: 10.0, GroupByValueIndexes: []int32{0}},
						{Value: 20.0, GroupByValueIndexes: []int32{1}},
					},
				},
			},
		},
	}
	if diff := cmp.Diff(want, resp,
		protocmp.Transform(),
		protocmp.IgnoreFields(&analyticspb.EvaluateAnalyticsChartResponse{}, "update_timestamp"),
	); diff != "" {
		s.Failf("unexpected response", "(-want +got):\n%s", diff)
	}
}

func (s *infraMetricsTestSuite) TestInfraMetricsPromQLGroupBy() {
	s.Run("no quantiles leaves group-by unchanged", func() {
		// When
		groupBy := infraMetricsPromQLGroupBy([]string{"env"}, nil)

		// Then
		s.Equal("env", groupBy)
	})

	s.Run("quantiles append le as last key", func() {
		// When
		groupBy := infraMetricsPromQLGroupBy([]string{"env"}, []float32{0.5, 0.99})

		// Then
		s.Equal("env, le", groupBy)
	})

	s.Run("quantiles with no request keys group by le only", func() {
		// When
		groupBy := infraMetricsPromQLGroupBy(nil, []float32{0.5})

		// Then
		s.Equal("le", groupBy)
	})

	s.Run("quantiles do not duplicate le when already requested", func() {
		// When
		groupBy := infraMetricsPromQLGroupBy([]string{"env", "le"}, []float32{0.5})

		// Then
		s.Equal("env, le", groupBy)
	})
}

// ---------------------------------------------------------------------------
// evaluateInfraMetricsChart — dashboard_quantiles
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_QuantilesInstantQuery() {
	// Given
	const promqlTemplate = `sum(rate(latency_bucket{customer="{{customer_id}}"}[{{duration}}]))`
	quantiles := []float32{0.5, 0.99}
	profileConfig := s.makeProfileConfigWithQuantiles(map[string]string{"my_metric": promqlTemplate}, "my_metric", quantiles)
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	baseQuery := prometheus.PromQLTemplateVars{
		CustomerID: "customer_id",
		ProfileID:  "profile_id",
		Duration:   fmt.Sprintf("%ds", int64(endTime.Sub(startTime).Seconds())),
		GroupBy:    "",
	}.Substitute(promqlTemplate)

	vectorFor50, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{}, "value": []any{float64(1704067200), "42.0"}},
	})
	vectorFor99, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{}, "value": []any{float64(1704067200), "99.0"}},
	})
	s.mockPromClient.EXPECT().
		Query(gomock.Any(), fmt.Sprintf("histogram_quantile(0.5, (%s))", baseQuery), gomock.Any()).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "vector", Result: vectorFor50}}, nil)
	s.mockPromClient.EXPECT().
		Query(gomock.Any(), fmt.Sprintf("histogram_quantile(0.99, (%s))", baseQuery), gomock.Any()).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "vector", Result: vectorFor99}}, nil)

	req := s.makeInstantQueryRequestWithTimeRange("my_metric", nil, startTime, endTime)

	// When
	resp, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().NoError(err)
	results := resp.GetResultForChart().GetAnalyticsResults()
	s.Require().Len(results, 1)
	result := results[0]

	// First (and only) groupby dimension: quantiles
	s.Require().Len(result.GetGroupByValuesForAllKeys(), 1)
	quantileDim := result.GetGroupByValuesForAllKeys()[0].GetGroupByValues()
	s.Require().Len(quantileDim, 2)
	s.InDelta(0.5, quantileDim[0].GetValue().GetFloatValue(), 0.001)
	s.InDelta(0.99, quantileDim[1].GetValue().GetFloatValue(), 0.001)

	// One analytics number per quantile
	numbers := result.GetAnalyticsNumbers()
	s.Require().Len(numbers, 2)
	s.InDelta(42.0, numbers[0].GetValue(), 0.001)
	s.Equal([]int32{0}, numbers[0].GetGroupByValueIndexes())
	s.InDelta(99.0, numbers[1].GetValue(), 0.001)
	s.Equal([]int32{1}, numbers[1].GetGroupByValueIndexes())
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_QuantilesInstantQueryWithLabelKeys() {
	// Given
	const promqlTemplate = `sum(rate(latency_bucket[{{duration}}])) by ({{group_by}})`
	quantiles := []float32{0.5, 0.99}
	profileConfig := s.makeProfileConfigFromDefs(map[string]*configpb.MetricDefinition{
		"my_metric": {
			PromqlQuery:        promqlTemplate,
			DashboardQuantiles: quantiles,
			Metric:             &metricspb.Metric{LabelKeys: []string{"env"}},
		},
	})
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	baseQuery := prometheus.PromQLTemplateVars{
		CustomerID: "customer_id",
		ProfileID:  "profile_id",
		Duration:   fmt.Sprintf("%ds", int64(endTime.Sub(startTime).Seconds())),
		GroupBy:    "env, le",
	}.Substitute(promqlTemplate)

	vectorFor50, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{"env": "prod"}, "value": []any{float64(1704067200), "40.0"}},
		{"metric": map[string]string{"env": "staging"}, "value": []any{float64(1704067200), "30.0"}},
	})
	vectorFor99, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{"env": "prod"}, "value": []any{float64(1704067200), "90.0"}},
		{"metric": map[string]string{"env": "staging"}, "value": []any{float64(1704067200), "80.0"}},
	})
	s.mockPromClient.EXPECT().
		Query(gomock.Any(), fmt.Sprintf("histogram_quantile(0.5, (%s))", baseQuery), gomock.Any()).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "vector", Result: vectorFor50}}, nil)
	s.mockPromClient.EXPECT().
		Query(gomock.Any(), fmt.Sprintf("histogram_quantile(0.99, (%s))", baseQuery), gomock.Any()).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "vector", Result: vectorFor99}}, nil)

	req := s.makeInstantQueryRequestWithTimeRange("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByInfraLabelKeys_{
			ByInfraLabelKeys: &dashboardpb.ChartGroupBy_ByInfraLabelKeys{LabelKeys: []string{"env"}},
		}},
	}, startTime, endTime)

	// When
	resp, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().NoError(err)
	result := resp.GetResultForChart().GetAnalyticsResults()[0]

	// Two groupby dimensions: [env, quantile]
	s.Require().Len(result.GetGroupByValuesForAllKeys(), 2)
	envDim := result.GetGroupByValuesForAllKeys()[0].GetGroupByValues()
	s.Require().Len(envDim, 2)
	s.Equal("prod", envDim[0].GetValue().GetStringValue())
	s.Equal("staging", envDim[1].GetValue().GetStringValue())

	quantileDim := result.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
	s.Require().Len(quantileDim, 2)
	s.InDelta(0.5, quantileDim[0].GetValue().GetFloatValue(), 0.001)
	s.InDelta(0.99, quantileDim[1].GetValue().GetFloatValue(), 0.001)

	// 4 numbers: 2 quantiles × 2 envs
	numbers := result.GetAnalyticsNumbers()
	s.Require().Len(numbers, 4)
	// q=0.5, env=prod
	s.Equal([]int32{0, 0}, numbers[0].GetGroupByValueIndexes())
	s.InDelta(40.0, numbers[0].GetValue(), 0.001)
	// q=0.5, env=staging
	s.Equal([]int32{1, 0}, numbers[1].GetGroupByValueIndexes())
	s.InDelta(30.0, numbers[1].GetValue(), 0.001)
	// q=0.99, env=prod
	s.Equal([]int32{0, 1}, numbers[2].GetGroupByValueIndexes())
	s.InDelta(90.0, numbers[2].GetValue(), 0.001)
	// q=0.99, env=staging
	s.Equal([]int32{1, 1}, numbers[3].GetGroupByValueIndexes())
	s.InDelta(80.0, numbers[3].GetValue(), 0.001)
}

func (s *infraMetricsTestSuite) TestEvaluateInfraMetricsChart_QuantilesRangeQuery() {
	// Given
	const promqlTemplate = `sum(rate(latency_bucket[{{duration}}]))`
	quantiles := []float32{0.5, 0.99}
	profileConfig := s.makeProfileConfigWithQuantiles(map[string]string{"my_metric": promqlTemplate}, "my_metric", quantiles)
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Return(profileConfig, nil)

	ts1 := float64(1704067200)
	ts2 := float64(1704153600)
	matrixFor50, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{}, "values": []any{[]any{ts1, "42.0"}, []any{ts2, "43.0"}}},
	})
	matrixFor99, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{}, "values": []any{[]any{ts1, "99.0"}, []any{ts2, "100.0"}}},
	})
	s.mockPromClient.EXPECT().
		QueryRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 24*time.Hour).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "matrix", Result: matrixFor50}}, nil)
	s.mockPromClient.EXPECT().
		QueryRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 24*time.Hour).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "matrix", Result: matrixFor99}}, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	req := s.makeInstantQueryRequestWithTimeRange("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
			ByTime: &dashboardpb.ChartGroupBy_Time{Interval: dashboardpb.ChartGroupBy_Time_DAY},
		}},
	}, startTime, endTime)

	// When
	resp, err := s.svc.evaluateInfraMetricsChart(s.ctx, req)

	// Then
	s.Require().NoError(err)
	result := resp.GetResultForChart().GetAnalyticsResults()[0]

	// Two groupby dimensions: [timestamp, quantile]
	s.Require().Len(result.GetGroupByValuesForAllKeys(), 2)
	s.Require().Len(result.GetGroupByValuesForAllKeys()[0].GetGroupByValues(), 2, "two timestamps")
	quantileDim := result.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
	s.Require().Len(quantileDim, 2)
	s.InDelta(0.5, quantileDim[0].GetValue().GetFloatValue(), 0.001)
	s.InDelta(0.99, quantileDim[1].GetValue().GetFloatValue(), 0.001)

	// 4 numbers: 2 timestamps × 2 quantiles
	numbers := result.GetAnalyticsNumbers()
	s.Require().Len(numbers, 4)
	// ts=0, q=0 (p50)
	s.Equal([]int32{0, 0}, numbers[0].GetGroupByValueIndexes())
	s.InDelta(42.0, numbers[0].GetValue(), 0.001)
	// ts=1, q=0 (p50)
	s.Equal([]int32{1, 0}, numbers[1].GetGroupByValueIndexes())
	s.InDelta(43.0, numbers[1].GetValue(), 0.001)
	// ts=0, q=1 (p99)
	s.Equal([]int32{0, 1}, numbers[2].GetGroupByValueIndexes())
	s.InDelta(99.0, numbers[2].GetValue(), 0.001)
	// ts=1, q=1 (p99)
	s.Equal([]int32{1, 1}, numbers[3].GetGroupByValueIndexes())
	s.InDelta(100.0, numbers[3].GetValue(), 0.001)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeProfileConfigFromDefs constructs a ProfileConfig from a map of fully-configured MetricDefinitions.
func (s *infraMetricsTestSuite) makeProfileConfigFromDefs(defs map[string]*configpb.MetricDefinition) *config.ProfileConfig {
	profileConfig, err := config.NewProfileConfig(&npconfigpb.ProfileConfig{
		Name:    "customers/customer_id/profiles/profile_id",
		Metrics: defs,
	})
	s.Require().NoError(err)
	return profileConfig
}

// makeProfileConfig constructs a ProfileConfig with the given metric ID → PromQL map.
func (s *infraMetricsTestSuite) makeProfileConfig(metrics map[string]string) *config.ProfileConfig {
	defs := make(map[string]*configpb.MetricDefinition, len(metrics))
	for id, promql := range metrics {
		defs[id] = &configpb.MetricDefinition{PromqlQuery: promql}
	}
	return s.makeProfileConfigFromDefs(defs)
}

// makeProfileConfigWithQuantiles constructs a ProfileConfig where the named metric has quantiles set.
func (s *infraMetricsTestSuite) makeProfileConfigWithQuantiles(metrics map[string]string, quantileMetricID string, quantiles []float32) *config.ProfileConfig {
	defs := make(map[string]*configpb.MetricDefinition, len(metrics))
	for id, promql := range metrics {
		def := &configpb.MetricDefinition{PromqlQuery: promql}
		if id == quantileMetricID {
			def.DashboardQuantiles = quantiles
		}
		defs[id] = def
	}
	return s.makeProfileConfigFromDefs(defs)
}

// makeInstantQueryRequest returns a request with a default 7-day time range and the given group-by.
func (s *infraMetricsTestSuite) makeInstantQueryRequest(metricID string, groupBy []*dashboardpb.ChartGroupBy) *analyticspb.EvaluateAnalyticsChartRequest {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	return s.makeInstantQueryRequestWithTimeRange(metricID, groupBy, startTime, endTime)
}

// makeInstantQueryRequestWithTimeRange returns a request with an explicit time range.
func (s *infraMetricsTestSuite) makeInstantQueryRequestWithTimeRange(metricID string, groupBy []*dashboardpb.ChartGroupBy, startTime, endTime time.Time) *analyticspb.EvaluateAnalyticsChartRequest {
	return &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: "customers/customer_id/profiles/profile_id",
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{Metric: &dashboardpb.ChartMetric_ForInfraMetric_{
						ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: metricID},
					}},
				},
				Filters: []*dashboardpb.ChartDataFilter{
					{ValueSet: &dashboardpb.ChartValueSet{
						Values: &dashboardpb.ChartValueSet_RangeValues{
							RangeValues: &dashboardpb.ChartValueSet_Range{
								RangeValues: []*dashboardpb.ChartValueRange{
									{
										FromValue: &dashboardpb.ChartAttributeValue{
											Value: &dashboardpb.ChartAttributeValue_Timestamp{
												Timestamp: timestamppb.New(startTime),
											},
										},
										ToValue: &dashboardpb.ChartAttributeValue{
											Value: &dashboardpb.ChartAttributeValue_Timestamp{
												Timestamp: timestamppb.New(endTime),
											},
										},
									},
								},
							},
						},
					}},
				},
				GroupBy: groupBy,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// EvaluateAnalyticsChart — smoke test for infra metrics routing (3 groupBys)
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestEvaluateAnalyticsChart_InfraMetricsSmoke() {
	// Given: metric with 2 quantiles and an "env" label key; chart groups by ByTime (DAY) +
	// ByInfraLabelKeys ["env"]. Together with the quantile dimension this produces 3 dimensions:
	// [timestamp, env, quantile]. This exercises the full EvaluateAnalyticsChart → evaluateInfraMetricsChart path.
	const promqlTemplate = `sum(rate(latency_bucket[{{duration}}])) by ({{group_by}})`
	quantiles := []float32{0.5, 0.99}
	profileConfig := s.makeProfileConfigFromDefs(map[string]*configpb.MetricDefinition{
		"my_metric": {
			PromqlQuery:        promqlTemplate,
			DashboardQuantiles: quantiles,
			Metric:             &metricspb.Metric{LabelKeys: []string{"env"}},
		},
	})
	// GetProfileConfig is called twice: once in the outer EvaluateAnalyticsChart handler and
	// once inside evaluateInfraMetricsChart.
	s.mockConfigClient.EXPECT().
		GetProfileConfig("customer_id", "profile_id").
		Times(2).
		Return(profileConfig, nil)

	ts1 := float64(1704067200)
	ts2 := float64(1704153600)
	matrixFor50, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{"env": "prod"}, "values": []any{[]any{ts1, "10.0"}, []any{ts2, "20.0"}}},
		{"metric": map[string]string{"env": "staging"}, "values": []any{[]any{ts1, "5.0"}, []any{ts2, "15.0"}}},
	})
	matrixFor99, _ := json.Marshal([]map[string]any{
		{"metric": map[string]string{"env": "prod"}, "values": []any{[]any{ts1, "90.0"}, []any{ts2, "80.0"}}},
		{"metric": map[string]string{"env": "staging"}, "values": []any{[]any{ts1, "70.0"}, []any{ts2, "60.0"}}},
	})
	// Verify the PromQL query strings sent to Prometheus for each quantile.
	const baseQuery = `sum(rate(latency_bucket[24h])) by (env, le)`
	s.mockPromClient.EXPECT().
		QueryRange(gomock.Any(), fmt.Sprintf("histogram_quantile(0.5, (%s))", baseQuery), gomock.Any(), gomock.Any(), 24*time.Hour).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "matrix", Result: matrixFor50}}, nil)
	s.mockPromClient.EXPECT().
		QueryRange(gomock.Any(), fmt.Sprintf("histogram_quantile(0.99, (%s))", baseQuery), gomock.Any(), gomock.Any(), 24*time.Hour).
		Return(&prometheus.QueryResult{Status: "success", Data: prometheus.QueryData{ResultType: "matrix", Result: matrixFor99}}, nil)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	req := s.makeInstantQueryRequestWithTimeRange("my_metric", []*dashboardpb.ChartGroupBy{
		{GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
			ByTime: &dashboardpb.ChartGroupBy_Time{Interval: dashboardpb.ChartGroupBy_Time_DAY},
		}},
		{GroupByMode: &dashboardpb.ChartGroupBy_ByInfraLabelKeys_{
			ByInfraLabelKeys: &dashboardpb.ChartGroupBy_ByInfraLabelKeys{LabelKeys: []string{"env"}},
		}},
	}, startTime, endTime)
	// EvaluateAnalyticsChart requires an auth principal in the context.
	ctx := serverauth.NewContextWithAPIKey(s.ctx, "test-api-key")

	// When
	resp, err := s.svc.EvaluateAnalyticsChart(ctx, req)

	// Then
	s.Require().NoError(err)
	result := resp.GetResultForChart().GetAnalyticsResults()[0]

	// Three groupby dimensions: [timestamp, env, quantile]
	s.Require().Len(result.GetGroupByValuesForAllKeys(), 3)
	s.Require().Len(result.GetGroupByValuesForAllKeys()[0].GetGroupByValues(), 2, "two timestamps")
	envDim := result.GetGroupByValuesForAllKeys()[1].GetGroupByValues()
	s.Require().Len(envDim, 2)
	s.Equal("prod", envDim[0].GetValue().GetStringValue())
	s.Equal("staging", envDim[1].GetValue().GetStringValue())
	quantileDim := result.GetGroupByValuesForAllKeys()[2].GetGroupByValues()
	s.Require().Len(quantileDim, 2)
	s.InDelta(0.5, quantileDim[0].GetValue().GetFloatValue(), 0.001)
	s.InDelta(0.99, quantileDim[1].GetValue().GetFloatValue(), 0.001)

	// 8 numbers: 2 timestamps × 2 quantiles × 2 envs, indexed as [tsIdx, envIdx, qIdx]
	numbers := result.GetAnalyticsNumbers()
	s.Require().Len(numbers, 8)
	// q=p50, env=prod
	s.Equal([]int32{0, 0, 0}, numbers[0].GetGroupByValueIndexes())
	s.InDelta(10.0, numbers[0].GetValue(), 0.001)
	s.Equal([]int32{1, 0, 0}, numbers[1].GetGroupByValueIndexes())
	s.InDelta(20.0, numbers[1].GetValue(), 0.001)
	// q=p50, env=staging
	s.Equal([]int32{0, 1, 0}, numbers[2].GetGroupByValueIndexes())
	s.InDelta(5.0, numbers[2].GetValue(), 0.001)
	s.Equal([]int32{1, 1, 0}, numbers[3].GetGroupByValueIndexes())
	s.InDelta(15.0, numbers[3].GetValue(), 0.001)
	// q=p99, env=prod
	s.Equal([]int32{0, 0, 1}, numbers[4].GetGroupByValueIndexes())
	s.InDelta(90.0, numbers[4].GetValue(), 0.001)
	s.Equal([]int32{1, 0, 1}, numbers[5].GetGroupByValueIndexes())
	s.InDelta(80.0, numbers[5].GetValue(), 0.001)
	// q=p99, env=staging
	s.Equal([]int32{0, 1, 1}, numbers[6].GetGroupByValueIndexes())
	s.InDelta(70.0, numbers[6].GetValue(), 0.001)
	s.Equal([]int32{1, 1, 1}, numbers[7].GetGroupByValueIndexes())
	s.InDelta(60.0, numbers[7].GetValue(), 0.001)
}

// ---------------------------------------------------------------------------
// infraMetricsTruncateStartTime
// ---------------------------------------------------------------------------

func (s *infraMetricsTestSuite) TestInfraMetricsTruncateStartTime() {
	utc := time.UTC
	// Wednesday 2025-01-08 14:37:42 UTC
	wednesday := time.Date(2025, 1, 8, 14, 37, 42, 0, utc)

	s.Run("HOUR truncates minutes and seconds", func() {
		// When
		result := infraMetricsTruncateStartTime(wednesday, dashboardpb.ChartGroupBy_Time_HOUR, utc, 0)

		// Then
		s.Equal(time.Date(2025, 1, 8, 14, 0, 0, 0, utc), result)
	})

	s.Run("DAY truncates to midnight UTC", func() {
		// When
		result := infraMetricsTruncateStartTime(wednesday, dashboardpb.ChartGroupBy_Time_DAY, utc, 0)

		// Then
		s.Equal(time.Date(2025, 1, 8, 0, 0, 0, 0, utc), result)
	})

	s.Run("DAY respects timezone: 02:00 UTC is previous day in UTC-5", func() {
		// Given
		// 2025-01-08 02:00 UTC = 2025-01-07 21:00 America/New_York
		nyLoc, err := time.LoadLocation("America/New_York")
		s.Require().NoError(err)
		earlyUTC := time.Date(2025, 1, 8, 2, 0, 0, 0, utc)

		// When
		result := infraMetricsTruncateStartTime(earlyUTC, dashboardpb.ChartGroupBy_Time_DAY, nyLoc, 0)

		// Then

		s.Equal(time.Date(2025, 1, 7, 0, 0, 0, 0, nyLoc), result)
	})

	s.Run("WEEK with unspecified start day defaults to Sunday", func() {
		// Wednesday Jan 8 → previous Sunday Jan 5
		// When
		result := infraMetricsTruncateStartTime(wednesday, dashboardpb.ChartGroupBy_Time_WEEK, utc, analyticspb.Metadata_DAY_OF_WEEK_UNSPECIFIED)

		// Then
		s.Equal(time.Date(2025, 1, 5, 0, 0, 0, 0, utc), result)
	})

	s.Run("WEEK with Monday start", func() {
		// Wednesday Jan 8 → previous Monday Jan 6
		// When
		result := infraMetricsTruncateStartTime(wednesday, dashboardpb.ChartGroupBy_Time_WEEK, utc, analyticspb.Metadata_MONDAY)

		// Then
		s.Equal(time.Date(2025, 1, 6, 0, 0, 0, 0, utc), result)
	})

	s.Run("WEEK already on start day returns same day midnight", func() {
		// Sunday Jan 5 with Sunday start → same day
		sunday := time.Date(2025, 1, 5, 9, 0, 0, 0, utc)

		// When
		result := infraMetricsTruncateStartTime(sunday, dashboardpb.ChartGroupBy_Time_WEEK, utc, analyticspb.Metadata_SUNDAY)

		// Then
		s.Equal(time.Date(2025, 1, 5, 0, 0, 0, 0, utc), result)
	})
}
