package analyticsimpl

import (
	"context"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RetrieveChartEvaluationDataTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveChartEvaluationDataTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
}

func TestRetrieveChartEvaluationData(t *testing.T) {
	suite.Run(t, &RetrieveChartEvaluationDataTestSuite{})
}

// Helper function to create a basic request with custom configurations
func (s *RetrieveChartEvaluationDataTestSuite) createBasicRequest(
	metrics []*dashboardingpb.ChartMetric,
	groupBy []*dashboardingpb.ChartGroupBy,
	filters []*dashboardingpb.ChartDataFilter,
) *analyticspb.EvaluateAnalyticsChartRequest {
	return &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: s.profileResourceName,
		AnalyticsChart: &dashboardingpb.AnalyticsChart{
			Name:        "test_chart",
			DisplayName: "Test Chart",
			Description: "Test chart description",
			DataConfig: &dashboardingpb.ChartDataConfig{
				Metrics: metrics,
				GroupBy: groupBy,
				Filters: filters,
			},
			RenderingConfig: &dashboardingpb.RenderingConfig{
				ChartDisplayType: dashboardingpb.ChartDisplayType_BAR_CHART,
			},
			Audience:      &dashboardingpb.DashboardAudience{},
			Accessibility: dashboardingpb.DashboardAccessibility_PERSONAL,
		},
	}
}

// Helper function to create a conversation count metric
func (s *RetrieveChartEvaluationDataTestSuite) createConversationCountMetric() *dashboardingpb.ChartMetric {
	return &dashboardingpb.ChartMetric{
		Metric: &dashboardingpb.ChartMetric_ForConversation_{
			ForConversation: &dashboardingpb.ChartMetric_ForConversation{
				StatsType: &dashboardingpb.StatsTypeForConversation{
					StatsType: &dashboardingpb.StatsTypeForConversation_CommonStatsType{
						CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
					},
				},
			},
		},
	}
}

// Helper function to create a subcategory metric
func (s *RetrieveChartEvaluationDataTestSuite) createSubcategoryMetric() *dashboardingpb.ChartMetric {
	return &dashboardingpb.ChartMetric{
		Metric: &dashboardingpb.ChartMetric_ForSubcategory_{
			ForSubcategory: &dashboardingpb.ChartMetric_ForSubcategory{
				StatsType: &dashboardingpb.StatsTypeForSubcategory{
					StatsType: &dashboardingpb.StatsTypeForSubcategory_CommonStatsType{
						CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
					},
				},
				Subcategories: []*dashboardingpb.SubcategoryForChart{
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
					},
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
					},
				},
			},
		},
	}
}

// Helper function to create an emotion metric
func (s *RetrieveChartEvaluationDataTestSuite) createEmotionMetric() *dashboardingpb.ChartMetric {
	return &dashboardingpb.ChartMetric{
		Metric: &dashboardingpb.ChartMetric_ForEmotion_{
			ForEmotion: &dashboardingpb.ChartMetric_ForEmotion{
				StatsType: &dashboardingpb.StatsTypeForEmotion{
					StatsType: &dashboardingpb.StatsTypeForEmotion_CommonStatsType{
						CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
					},
				},
				Emotions: []*dashboardingpb.EmotionForChart{
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
					},
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
					},
				},
			},
		},
	}
}

// Helper function to create an aggregate subcategory metric
func (s *RetrieveChartEvaluationDataTestSuite) createAggregateSubcategoryMetric() *dashboardingpb.ChartMetric {
	return &dashboardingpb.ChartMetric{
		Metric: &dashboardingpb.ChartMetric_ForAggregateSubcategory_{
			ForAggregateSubcategory: &dashboardingpb.ChartMetric_ForAggregateSubcategory{
				StatsType: &dashboardingpb.StatsTypeForAggregateSubcategory{
					StatsType: &dashboardingpb.StatsTypeForAggregateSubcategory_CommonStatsType{
						CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
					},
				},
				AggregateSubcategories: []*dashboardingpb.AggregateSubcategoryForChart{
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
					},
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
					},
				},
			},
		},
	}
}

// Helper function to create a byTime group
func (s *RetrieveChartEvaluationDataTestSuite) createByTimeGroup() *dashboardingpb.ChartGroupBy {
	return &dashboardingpb.ChartGroupBy{
		GroupByMode: &dashboardingpb.ChartGroupBy_ByTime{
			ByTime: &dashboardingpb.ChartGroupBy_Time{
				Interval: dashboardingpb.ChartGroupBy_Time_DAY,
			},
		},
	}
}

func (s *RetrieveChartEvaluationDataTestSuite) createUsecaseFilter() *dashboardingpb.ChartDataFilter {
	return &dashboardingpb.ChartDataFilter{
		TargetAttribute: dashboardingpb.ChartAttribute_CONVERSATION_ID,
		Operator:        dashboardingpb.ChartDataFilter_IN,
		ValueSet: &dashboardingpb.ChartValueSet{
			Values: &dashboardingpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardingpb.ChartValueSet_Single{
					AttributeValues: []*dashboardingpb.ChartAttributeValue{
						{
							Value: &dashboardingpb.ChartAttributeValue_UsecaseNameValue{
								UsecaseNameValue: "id1",
							},
						},
					},
				},
			},
		},
	}
}

// Helper function to create a byProvidedValue group
func (s *RetrieveChartEvaluationDataTestSuite) createByProvidedValueGroup() *dashboardingpb.ChartGroupBy {
	return &dashboardingpb.ChartGroupBy{
		GroupByMode: &dashboardingpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardingpb.ChartGroupBy_ProvidedValue{
				ValueSet: &dashboardingpb.ChartValueSet{
					Values: &dashboardingpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardingpb.ChartValueSet_Single{
							AttributeValues: []*dashboardingpb.ChartAttributeValue{
								{
									Value: &dashboardingpb.ChartAttributeValue_SubcategoryValue{
										SubcategoryValue: &dashboardingpb.SubcategoryForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
										},
									},
								},
								{
									Value: &dashboardingpb.ChartAttributeValue_SubcategoryValue{
										SubcategoryValue: &dashboardingpb.SubcategoryForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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

// Helper function to create a byProvidedValue group with cluster values and no attribute
func (s *RetrieveChartEvaluationDataTestSuite) createByProvidedValueClusterGroup() *dashboardingpb.ChartGroupBy {
	return &dashboardingpb.ChartGroupBy{
		GroupByMode: &dashboardingpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardingpb.ChartGroupBy_ProvidedValue{
				ValueSet: &dashboardingpb.ChartValueSet{
					Values: &dashboardingpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardingpb.ChartValueSet_Single{
							AttributeValues: []*dashboardingpb.ChartAttributeValue{
								{
									Value: &dashboardingpb.ChartAttributeValue_ClusterValue{
										ClusterValue: &dashboardingpb.ClusterForChart{
											MomentName:   "customers/salt-river-project/profiles/us-west-2/moments/cluster-1",
											DetailedType: dashboardingpb.ClusterForChart_CONVERSATION_REASON_V3,
										},
									},
								},
								{
									Value: &dashboardingpb.ChartAttributeValue_ClusterValue{
										ClusterValue: &dashboardingpb.ClusterForChart{
											MomentName:   "customers/salt-river-project/profiles/us-west-2/moments/cluster-2",
											DetailedType: dashboardingpb.ClusterForChart_CONVERSATION_REASON_V3,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Config: &dashboardingpb.ChartGroupBy_Config{
			GroupbyPriority: dashboardingpb.ChartGroupBy_Config_PRIMARY,
		},
	}
}

// Helper function to create a byUniqueValue group
func (s *RetrieveChartEvaluationDataTestSuite) createByUniqueValueGroup() *dashboardingpb.ChartGroupBy {
	return &dashboardingpb.ChartGroupBy{
		GroupByMode: &dashboardingpb.ChartGroupBy_ByUniqueValue{
			ByUniqueValue: &dashboardingpb.ChartGroupBy_UniqueValue{
				UniqueValueFrom: &dashboardingpb.ChartGroupBy_UniqueValue_Attribute_{
					Attribute: dashboardingpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
				},
			},
		},
	}
}

// Helper function to create a byProvidedValue group with specific attribute
func (s *RetrieveChartEvaluationDataTestSuite) createByProvidedValueGroupWithAttribute(attribute dashboardingpb.ChartAttribute) *dashboardingpb.ChartGroupBy {
	return &dashboardingpb.ChartGroupBy{
		GroupByMode: &dashboardingpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardingpb.ChartGroupBy_ProvidedValue{
				Attribute: attribute,
				ValueSet: &dashboardingpb.ChartValueSet{
					Values: &dashboardingpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardingpb.ChartValueSet_Single{
							AttributeValues: []*dashboardingpb.ChartAttributeValue{
								{
									Value: &dashboardingpb.ChartAttributeValue_UsecaseNameValue{
										UsecaseNameValue: "test-value-1",
									},
								},
								{
									Value: &dashboardingpb.ChartAttributeValue_UsecaseNameValue{
										UsecaseNameValue: "test-value-2",
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

// Tests for getUngroupedRequest function
func (s *RetrieveChartEvaluationDataTestSuite) TestGetUngroupedRequest_GivenRequestWithMultipleGroupsAndMetrics() {
	// Given: A request with multiple groups and metrics
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{
			s.createSubcategoryMetric(),
			s.createEmotionMetric(),
		},
		[]*dashboardingpb.ChartGroupBy{
			s.createByTimeGroup(),
			s.createByProvidedValueGroup(),
		},
		[]*dashboardingpb.ChartDataFilter{
			{
				Operator: dashboardingpb.ChartDataFilter_IN,
				ValueSet: &dashboardingpb.ChartValueSet{
					Values: &dashboardingpb.ChartValueSet_RangeValues{
						RangeValues: &dashboardingpb.ChartValueSet_Range{
							RangeValues: []*dashboardingpb.ChartValueRange{
								{
									FromValue: &dashboardingpb.ChartAttributeValue{
										Value: &dashboardingpb.ChartAttributeValue_Timestamp{
											Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
										},
									},
									ToValue: &dashboardingpb.ChartAttributeValue{
										Value: &dashboardingpb.ChartAttributeValue_Timestamp{
											Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	)

	// When: getUngroupedRequest is called
	result := getUngroupedRequest(originalRequest)

	// Then: The result should have empty GroupBy and single conversation count metric
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")

	// Then: Other fields should remain unchanged
	s.Equal(originalRequest.AnalyticsChart.DataConfig.Filters, result.AnalyticsChart.DataConfig.Filters,
		"Filters should remain unchanged")
	s.Equal(originalRequest.Parent, result.Parent, "Parent should remain unchanged")
	s.Equal(originalRequest.AnalyticsChart.Name, result.AnalyticsChart.Name, "Chart name should remain unchanged")
	s.Equal(originalRequest.AnalyticsChart.DisplayName, result.AnalyticsChart.DisplayName, "Chart display name should remain unchanged")
	s.Equal(originalRequest.AnalyticsChart.Description, result.AnalyticsChart.Description, "Chart description should remain unchanged")
	s.Equal(originalRequest.AnalyticsChart.RenderingConfig, result.AnalyticsChart.RenderingConfig, "Rendering config should remain unchanged")
	s.Equal(originalRequest.AnalyticsChart.Audience, result.AnalyticsChart.Audience, "Audience should remain unchanged")
	s.Equal(originalRequest.AnalyticsChart.Accessibility, result.AnalyticsChart.Accessibility, "Accessibility should remain unchanged")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetUngroupedRequest_GivenRequestWithNoGroups() {
	// Given: A request with no groups
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{}, // No groups
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getUngroupedRequest is called
	result := getUngroupedRequest(originalRequest)

	// Then: The result should have empty GroupBy and single conversation count metric
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")

	// Then: Other fields should remain unchanged
	s.Empty(result.AnalyticsChart.DataConfig.Filters, "Filters should be empty")
	s.Equal(originalRequest.Parent, result.Parent, "Parent should remain unchanged")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetUngroupedRequest_GivenRequestWithConversationCountMetric() {
	// Given: A request with conversation count metric
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{s.createByTimeGroup()},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getUngroupedRequest is called
	result := getUngroupedRequest(originalRequest)

	// Then: The result should have empty GroupBy and single conversation count metric
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")

	// Then: Other fields should remain unchanged
	s.Empty(result.AnalyticsChart.DataConfig.Filters, "Filters should be empty")
	s.Equal(originalRequest.Parent, result.Parent, "Parent should remain unchanged")
}

// Tests for getGroupsAsFiltersRequest function
func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithByProvidedValueGroup_WhenCalled_ThenConvertsGroupToFilter() {
	// Given
	// A request with byProvidedValue group containing subcategory values
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{s.createByProvidedValueGroup()},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Require().Len(result.AnalyticsChart.DataConfig.Filters, 1, "Should have one filter")

	filter := result.AnalyticsChart.DataConfig.Filters[0]
	s.Equal(dashboardingpb.ChartDataFilter_IN, filter.Operator, "Filter should use IN operator")
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, filter.TargetAttribute, "Filter should use unspecified attribute")
	s.Require().NotNil(filter.ValueSet, "Filter should have value set")

	singleValues := filter.ValueSet.GetSingleValues()
	s.Require().NotNil(singleValues, "Filter should have SingleValues")
	s.Require().Len(singleValues.AttributeValues, 2, "Filter should have two attribute values")
	s.Equal(
		"customers/customer_id/profiles/profile_id/moments/subcategory-1",
		singleValues.AttributeValues[0].GetSubcategoryValue().GetMomentName(),
		"First value should match",
	)
	s.Equal(
		"customers/customer_id/profiles/profile_id/moments/subcategory-2",
		singleValues.AttributeValues[1].GetSubcategoryValue().GetMomentName(),
		"Second value should match",
	)

	s.Len(originalRequest.AnalyticsChart.DataConfig.GroupBy, 1, "Original request should not be modified")
	s.Empty(originalRequest.AnalyticsChart.DataConfig.Filters, "Original request should not be modified")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithByProvidedValueClusterGroupWithoutAttribute_WhenCalled_ThenConvertsGroupToFilter() {
	// Given: A request with byProvidedValue cluster group and no target attribute
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{s.createByProvidedValueClusterGroup()},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Require().Len(result.AnalyticsChart.DataConfig.Filters, 1, "Should have one filter from cluster groupBy")

	filter := result.AnalyticsChart.DataConfig.Filters[0]
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, filter.TargetAttribute, "Cluster groupBy filter should use unspecified attribute")
	s.Equal(dashboardingpb.ChartDataFilter_IN, filter.Operator, "Filter should use IN operator")
	s.Require().NotNil(filter.ValueSet.GetSingleValues(), "Filter should have single values")
	s.Require().Len(filter.ValueSet.GetSingleValues().AttributeValues, 2, "Filter should include all cluster values")
	s.Equal(
		"customers/salt-river-project/profiles/us-west-2/moments/cluster-1",
		filter.ValueSet.GetSingleValues().AttributeValues[0].GetClusterValue().GetMomentName(),
		"First cluster should match",
	)
	s.Equal(
		dashboardingpb.ClusterForChart_CONVERSATION_REASON_V3,
		filter.ValueSet.GetSingleValues().AttributeValues[0].GetClusterValue().GetDetailedType(),
		"First cluster detailed type should match",
	)
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenAggregateSubcategoryMetricWithClusterGroupBy_WhenCalled_ThenConvertsBothToFilters() {
	// Given: SRP-style pie chart with aggregate subcategory metric and cluster groupBy
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createAggregateSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{s.createByProvidedValueClusterGroup()},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Require().Len(result.AnalyticsChart.DataConfig.Filters, 2, "Should have cluster groupBy and aggregate subcategory metric filters")
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, result.AnalyticsChart.DataConfig.Filters[0].TargetAttribute)
	singleValues0 := result.AnalyticsChart.DataConfig.Filters[0].ValueSet.GetSingleValues()
	s.Require().NotNil(singleValues0)
	s.Require().NotNil(singleValues0.AttributeValues[0].GetClusterValue())
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, result.AnalyticsChart.DataConfig.Filters[1].TargetAttribute)
	singleValues1 := result.AnalyticsChart.DataConfig.Filters[1].ValueSet.GetSingleValues()
	s.Require().NotNil(singleValues1)
	s.Require().NotNil(singleValues1.AttributeValues[0].GetAggregateSubcategoryValue())
	s.Require().Len(result.AnalyticsChart.DataConfig.Metrics, 1)
	s.Equal(
		dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().GetStatsType().GetCommonStatsType(),
		"Metric should be replaced with conversation count",
	)
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithByTimeGroup_WhenCalled_ThenSkipsTimeGroupConversion() {
	// Given: A request with byTime group
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{s.createByTimeGroup()},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should have empty GroupBy and no additional filters
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Empty(result.AnalyticsChart.DataConfig.Filters, "Should not create filters from byTime groups")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithByUniqueValueGroup_WhenCalled_ThenSkipsUniqueValueGroupConversion() {
	// Given: A request with byUniqueValue group
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{s.createByUniqueValueGroup()},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should have empty GroupBy and no additional filters
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Empty(result.AnalyticsChart.DataConfig.Filters, "Should not create filters from byUniqueValue groups")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithSubcategoryMetric_WhenCalled_ThenConvertsMetricToFilterAndReplacesWithConversationCount() {
	// Given: A request with subcategory metric
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should have subcategory filter and conversation count metric
	s.Len(result.AnalyticsChart.DataConfig.Filters, 1, "Should have one filter from subcategory metric")
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have one metric")

	// Then: Check the filter was created correctly
	filter := result.AnalyticsChart.DataConfig.Filters[0]
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, filter.TargetAttribute,
		"Filter should have unspecified target attribute for subcategory")
	s.Equal(dashboardingpb.ChartDataFilter_IN, filter.Operator, "Filter should use IN operator")
	s.NotNil(filter.ValueSet, "Filter should have value set")

	// Then: Check the ValueSet structure for subcategories
	s.NotNil(filter.ValueSet.GetSingleValues(), "Filter should have SingleValues")
	s.Len(filter.ValueSet.GetSingleValues().AttributeValues, 2, "Filter should have two attribute values for subcategories")

	// Then: Check subcategory values
	subcategoryValue1 := filter.ValueSet.GetSingleValues().AttributeValues[0].GetSubcategoryValue()
	s.NotNil(subcategoryValue1, "First attribute value should be subcategory")
	s.Equal("customers/customer_id/profiles/profile_id/moments/subcategory-1", subcategoryValue1.MomentName, "First subcategory should match")

	subcategoryValue2 := filter.ValueSet.GetSingleValues().AttributeValues[1].GetSubcategoryValue()
	s.NotNil(subcategoryValue2, "Second attribute value should be subcategory")
	s.Equal("customers/customer_id/profiles/profile_id/moments/subcategory-2", subcategoryValue2.MomentName, "Second subcategory should match")

	// Then: Check the metric was replaced with conversation count
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithEmotionMetric_WhenCalled_ThenConvertsMetricToFilterAndReplacesWithConversationCount() {
	// Given: A request with emotion metric
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createEmotionMetric()},
		[]*dashboardingpb.ChartGroupBy{},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should have emotion filter and conversation count metric
	s.Require().Len(result.AnalyticsChart.DataConfig.Filters, 1, "Should have one filter from emotion metric")
	s.Require().Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have one metric")

	// Then: Check the filter was created correctly
	filter := result.AnalyticsChart.DataConfig.Filters[0]
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, filter.TargetAttribute,
		"Filter should have unspecified target attribute for emotion")
	s.Equal(dashboardingpb.ChartDataFilter_IN, filter.Operator, "Filter should use IN operator")
	s.Require().NotNil(filter.ValueSet, "Filter should have value set")

	// Then: Check the ValueSet structure for emotions
	s.Require().NotNil(filter.ValueSet.GetSingleValues(), "Filter should have SingleValues")
	s.Require().Len(filter.ValueSet.GetSingleValues().AttributeValues, 2, "Filter should have two attribute values for emotions")

	// Then: Check emotion values
	emotionValue1 := filter.ValueSet.GetSingleValues().AttributeValues[0].GetEmotionValue()
	s.Require().NotNil(emotionValue1, "First attribute value should be emotion")
	s.Equal("customers/customer_id/profiles/profile_id/moments/emotion-1", emotionValue1.GetMomentName(), "First emotion should match")

	emotionValue2 := filter.ValueSet.GetSingleValues().AttributeValues[1].GetEmotionValue()
	s.Require().NotNil(emotionValue2, "Second attribute value should be emotion")
	s.Equal("customers/customer_id/profiles/profile_id/moments/emotion-2", emotionValue2.GetMomentName(), "Second emotion should match")

	// Then: Check the metric was replaced with conversation count
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithAggregateSubcategoryMetric_WhenCalled_ThenConvertsMetricToFilterAndReplacesWithConversationCount() {
	// Given: A request with aggregate subcategory metric
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createAggregateSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should have aggregate subcategory filter and conversation count metric
	s.Require().Len(result.AnalyticsChart.DataConfig.Filters, 1, "Should have one filter from aggregate subcategory metric")
	s.Require().Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have one metric")

	// Then: Check the filter was created correctly
	filter := result.AnalyticsChart.DataConfig.Filters[0]
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, filter.TargetAttribute,
		"Filter should have unspecified target attribute for aggregate subcategory")
	s.Equal(dashboardingpb.ChartDataFilter_IN, filter.Operator, "Filter should use IN operator")
	s.Require().NotNil(filter.ValueSet, "Filter should have value set")

	// Then: Check the ValueSet structure for aggregate subcategories
	s.Require().NotNil(filter.ValueSet.GetSingleValues(), "Filter should have SingleValues")
	s.Require().Len(filter.ValueSet.GetSingleValues().AttributeValues, 2, "Filter should have two attribute values for aggregate subcategories")

	// Then: Check aggregate subcategory values
	aggregateSubcategoryValue1 := filter.ValueSet.GetSingleValues().AttributeValues[0].GetAggregateSubcategoryValue()
	s.Require().NotNil(aggregateSubcategoryValue1, "First attribute value should be aggregate subcategory")
	s.Equal("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1", aggregateSubcategoryValue1.GetMomentName(), "First aggregate subcategory should match")

	aggregateSubcategoryValue2 := filter.ValueSet.GetSingleValues().AttributeValues[1].GetAggregateSubcategoryValue()
	s.Require().NotNil(aggregateSubcategoryValue2, "Second attribute value should be aggregate subcategory")
	s.Equal("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2", aggregateSubcategoryValue2.GetMomentName(), "Second aggregate subcategory should match")

	// Then: Check the metric was replaced with conversation count
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithConversationMetric_WhenCalled_ThenKeepsOriginalMetric() {
	// Given: A request with conversation metric
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{},
		[]*dashboardingpb.ChartDataFilter{},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should keep the original conversation metric
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should keep conversation count metric")
	s.Empty(result.AnalyticsChart.DataConfig.Filters, "Should not create additional filters")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenComplexRequestWithMultipleGroupsAndMetrics_WhenCalled_ThenHandlesAllConversions() {
	// Given: A complex request with multiple groups and metrics
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{
			s.createSubcategoryMetric(),
			s.createConversationCountMetric(),
		},
		[]*dashboardingpb.ChartGroupBy{
			s.createByProvidedValueGroup(),
			s.createByTimeGroup(),        // Should be skipped
			s.createByUniqueValueGroup(), // Should be skipped
		},
		[]*dashboardingpb.ChartDataFilter{
			{
				TargetAttribute: dashboardingpb.ChartAttribute_CONVERSATION_ID,
				Operator:        dashboardingpb.ChartDataFilter_IN,
				ValueSet: &dashboardingpb.ChartValueSet{
					Values: &dashboardingpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardingpb.ChartValueSet_Single{
							AttributeValues: []*dashboardingpb.ChartAttributeValue{
								{
									Value: &dashboardingpb.ChartAttributeValue_UsecaseNameValue{
										UsecaseNameValue: "id1",
									},
								},
							},
						},
					},
				},
			},
		},
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should have empty GroupBy, multiple filters, and transformed metrics
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Len(result.AnalyticsChart.DataConfig.Filters, 3, "Should have three filters: original + byProvidedValue + subcategory")
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 2, "Should have two metrics")

	// Then: Check filter order - original filters first, then group filters, then metric filters
	// First filter should be the original one
	originalFilter := result.AnalyticsChart.DataConfig.Filters[0]
	s.Equal(dashboardingpb.ChartAttribute_CONVERSATION_ID, originalFilter.TargetAttribute, "First filter should be original")
	s.Equal("id1", originalFilter.ValueSet.GetSingleValues().AttributeValues[0].GetUsecaseNameValue(), "Original filter value should match")

	// Second filter should be from byProvidedValue group
	groupFilter := result.AnalyticsChart.DataConfig.Filters[1]
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, groupFilter.TargetAttribute, "Second filter should be from group")
	s.Equal(dashboardingpb.ChartDataFilter_IN, groupFilter.Operator, "Group filter should use IN operator")
	s.Require().NotNil(groupFilter.ValueSet.GetSingleValues().AttributeValues[0].GetSubcategoryValue(), "Group filter should have subcategory values")

	// Third filter should be from subcategory metric
	metricFilter := result.AnalyticsChart.DataConfig.Filters[2]
	s.Equal(dashboardingpb.ChartAttribute_CHART_ATTRIBUTE_UNSPECIFIED, metricFilter.TargetAttribute, "Third filter should be from metric")
	s.NotNil(metricFilter.ValueSet.GetSingleValues().AttributeValues[0].GetSubcategoryValue(), "Metric filter should have subcategory values")

	// Then: Check that both metrics are conversation count
	for _, metric := range result.AnalyticsChart.DataConfig.Metrics {
		s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
			metric.GetForConversation().StatsType.GetCommonStatsType(),
			"All metrics should be conversation count")
	}

	// Then: Original request should not be modified
	s.Len(originalRequest.AnalyticsChart.DataConfig.GroupBy, 3, "Original request should not be modified")
	s.Len(originalRequest.AnalyticsChart.DataConfig.Filters, 1, "Original request should not be modified")
	s.Len(originalRequest.AnalyticsChart.DataConfig.Metrics, 2, "Original request should not be modified")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetGroupsAsFiltersRequest_GivenRequestWithNilValues_WhenCalled_ThenHandlesGracefully() {
	// Given: A request with nil values
	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		nil, // Nil GroupBy
		nil, // Nil Filters
	)

	// When: getGroupsAsFiltersRequest is called
	result := getGroupsAsFiltersRequest(originalRequest)

	// Then: The result should handle nil values gracefully
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "GroupBy should be empty")
	s.Empty(result.AnalyticsChart.DataConfig.Filters, "Filters should be empty")
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have one metric")
}

// Test for sumAnalyticsNumbersForAnalyticsResult function
func (s *RetrieveChartEvaluationDataTestSuite) TestSumAnalyticsNumbersForAnalyticsResult_GivenMultipleResults_WhenCalled_ThenReturnsCorrectSum() {
	// Given: Multiple analytics results with different values
	results := []*dashboardingpb.AnalyticsResult{
		{
			AnalyticsNumbers: []*dashboardingpb.AnalyticsResult_AnalyticsNumber{
				{Value: 10.0},
				{Value: 20.0},
			},
		},
		{
			AnalyticsNumbers: []*dashboardingpb.AnalyticsResult_AnalyticsNumber{
				{Value: 5.0},
			},
		},
		{
			AnalyticsNumbers: []*dashboardingpb.AnalyticsResult_AnalyticsNumber{
				{Value: 15.0},
				{Value: 25.0},
				{Value: 8.0},
			},
		},
	}

	// When: sumAnalyticsNumbersForAnalyticsResult is called
	result := sumAnalyticsNumbersForAnalyticsResult(results[2])

	// Then: The result should be the sum of all values
	expected := int32(15.0 + 25.0 + 8.0)
	s.Equal(expected, result, "Should return correct sum")
}

// Tests for getPrimaryGroupOnlyRequest function
func (s *RetrieveChartEvaluationDataTestSuite) TestGetPrimaryGroupOnlyRequest_GivenRequestWithPrimaryPriorityGroup_WhenCalled_ThenReturnsPrimaryGroupOnly() {
	// Given: A request with PRIMARY priority group
	primaryGroup := s.createByProvidedValueGroupWithAttribute(dashboardingpb.ChartAttribute_CONVERSATION_ID)
	primaryGroup.Config = &dashboardingpb.ChartGroupBy_Config{
		GroupbyPriority: dashboardingpb.ChartGroupBy_Config_PRIMARY,
	}

	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{
			primaryGroup,
			s.createByTimeGroup(),
		},
		[]*dashboardingpb.ChartDataFilter{s.createUsecaseFilter()},
	)

	// When: getPrimaryGroupOnlyRequest is called
	result := getPrimaryGroupOnlyRequest(originalRequest)

	// Then: The result should have only the PRIMARY group
	s.Len(result.AnalyticsChart.DataConfig.GroupBy, 1, "Should have exactly one group")
	s.Equal(primaryGroup, result.AnalyticsChart.DataConfig.GroupBy[0], "Should be the PRIMARY group")

	// Then: Metrics should be conversation count
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")

	// Then: Other fields should remain unchanged
	s.Equal(originalRequest.AnalyticsChart.DataConfig.Filters, result.AnalyticsChart.DataConfig.Filters, "Filters should remain unchanged")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetPrimaryGroupOnlyRequest_GivenNoPrimaryButTimeBasedGroup_WhenCalled_ThenReturnsTimeBasedGroup() {
	// Given: A request with no PRIMARY but has time-based group
	timeGroup := s.createByTimeGroup()
	uniqueValueGroup := s.createByUniqueValueGroup()

	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{
			uniqueValueGroup,
			timeGroup,
		},
		[]*dashboardingpb.ChartDataFilter{s.createUsecaseFilter()},
	)

	// When: getPrimaryGroupOnlyRequest is called
	result := getPrimaryGroupOnlyRequest(originalRequest)

	// Then: The result should have only the time-based group
	s.Len(result.AnalyticsChart.DataConfig.GroupBy, 1, "Should have exactly one group")
	s.Equal(timeGroup, result.AnalyticsChart.DataConfig.GroupBy[0], "Should be the time-based group")

	// Then: Metrics should be conversation count
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetPrimaryGroupOnlyRequest_GivenNoPrimaryNoTimeBasedGroup_WhenCalled_ThenReturnsFirstGroup() {
	// Given: A request with no PRIMARY and no time-based group
	uniqueValueGroup := s.createByUniqueValueGroup()
	providedValueGroup := s.createByProvidedValueGroup()

	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createConversationCountMetric()},
		[]*dashboardingpb.ChartGroupBy{
			uniqueValueGroup,
			providedValueGroup,
		},
		[]*dashboardingpb.ChartDataFilter{s.createUsecaseFilter()},
	)

	// When: getPrimaryGroupOnlyRequest is called
	result := getPrimaryGroupOnlyRequest(originalRequest)

	// Then: The result should have only the first group
	s.Len(result.AnalyticsChart.DataConfig.GroupBy, 1, "Should have exactly one group")
	s.Equal(uniqueValueGroup, result.AnalyticsChart.DataConfig.GroupBy[0], "Should be the first group")

	// Then: Metrics should be conversation count
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetPrimaryGroupOnlyRequest_GivenByTimeAndSecondaryGroups_WhenCalled_ThenByTimeTakesPrecedence() {
	// Given: A request with both ByTime and SECONDARY priority groups (no PRIMARY)
	byTimeGroup := s.createByTimeGroup()
	secondaryGroup := s.createByProvidedValueGroupWithAttribute(dashboardingpb.ChartAttribute_CONVERSATION_ID)
	secondaryGroup.Config = &dashboardingpb.ChartGroupBy_Config{
		GroupbyPriority: dashboardingpb.ChartGroupBy_Config_SECONDARY,
	}

	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{
			secondaryGroup,
			byTimeGroup,
		},
		[]*dashboardingpb.ChartDataFilter{s.createUsecaseFilter()},
	)

	// When: getPrimaryGroupOnlyRequest is called
	result := getPrimaryGroupOnlyRequest(originalRequest)

	// Then: The result should have the ByTime group, not the SECONDARY group
	s.Len(result.AnalyticsChart.DataConfig.GroupBy, 1, "Should have exactly one group")
	s.Equal(byTimeGroup, result.AnalyticsChart.DataConfig.GroupBy[0], "Should be the ByTime group, not SECONDARY")

	// Then: Metrics should be conversation count (foundPrimary is true)
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have exactly one metric")
	s.Equal(dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
		result.AnalyticsChart.DataConfig.Metrics[0].GetForConversation().StatsType.GetCommonStatsType(),
		"Should have conversation count metric")
}

func (s *RetrieveChartEvaluationDataTestSuite) TestGetPrimaryGroupOnlyRequest_GivenOnlySecondaryGroup_WhenCalled_ThenReturnsEmptyGroupBy() {
	// Given: A request with only SECONDARY priority group (no PRIMARY, no ByTime)
	secondaryGroup := s.createByProvidedValueGroupWithAttribute(dashboardingpb.ChartAttribute_CONVERSATION_ID)
	secondaryGroup.Config = &dashboardingpb.ChartGroupBy_Config{
		GroupbyPriority: dashboardingpb.ChartGroupBy_Config_SECONDARY,
	}

	originalRequest := s.createBasicRequest(
		[]*dashboardingpb.ChartMetric{s.createSubcategoryMetric()},
		[]*dashboardingpb.ChartGroupBy{
			secondaryGroup,
		},
		[]*dashboardingpb.ChartDataFilter{s.createUsecaseFilter()},
	)

	// When: getPrimaryGroupOnlyRequest is called
	result := getPrimaryGroupOnlyRequest(originalRequest)

	// Then: The result should have empty GroupBy
	s.Empty(result.AnalyticsChart.DataConfig.GroupBy, "Should have empty GroupBy when only SECONDARY exists")

	// Then: Original metrics should be preserved (foundPrimary is false, so metrics not changed)
	s.Len(result.AnalyticsChart.DataConfig.Metrics, 1, "Should have one metric")
	s.NotNil(result.AnalyticsChart.DataConfig.Metrics[0].GetForSubcategory(), "Should preserve original subcategory metric")
}

// TestRetrieveChartEvaluationData_TopicDiscoveryV3ClustersGroupBy exercises the full RetrieveChartEvaluationData
// fan-out for a single-emotion conversation-count chart grouped by Topic Discovery V3 clusters, with
// FetchPercentageRelatedData=true. That flag drives four concurrent EvaluateAnalyticsChart calls:
//
//   - main:               emotion conversation count grouped by the two V3 clusters
//   - ungrouped:          conversation count with no group-by (metric replaced with plain conversation count)
//   - groups-as-filters:  conversation count with the two clusters AND the emotion converted to IN filters
//   - primary-group-only: conversation count grouped by the PRIMARY-priority cluster group (metric replaced
//     with plain conversation count)
//
// Unlike a plain conversation-count chart, the emotion metric makes all four requests distinct: the emotion
// survives only on the main query (as a top-level metric filter) and on the groups-as-filters query (as a
// second IN filter). getUngroupedRequest and getPrimaryGroupOnlyRequest both replace the emotion metric with
// a plain conversation count, so primary-group-only no longer collapses onto main. Each of the four queries is
// asserted against its own golden file.
//
// Because the metric carries a single emotion, that emotion folds into the main query as a top-level filter
// rather than an extra bucketing level, so the main result stays one-dimensional (one number per cluster
// bucket) — a two-emotion metric would instead add an emotion grouping level and make the main result 2-D.
//
// The category tree (see v3Tree / v3Template) is:
//
//	tdv3-cat-1
//	├── tdv3-cat-1-1              (leaf)
//	└── tdv3-cat-1-2             (non-leaf)
//	    └── tdv3-cat-1-2-1        (leaf)
//
// Grouping by tdv3-cat-1 expands to the whole subtree; grouping by tdv3-cat-1-2 expands only to its own
// subtree, so the grouped query emits two cluster filter buckets.
func TestRetrieveChartEvaluationData_TopicDiscoveryV3ClustersGroupBy(t *testing.T) {
	// These flags gate the ES query shape (moment-annotation payload mapping + ACL filtering) and are read
	// during Init(), so they must be set before SetupSuite wires the server. t.Setenv restores each
	// variable's original value (and unset state) after the test.
	t.Setenv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	t.Setenv("ENABLE_ACL_FILTERING_IN_EVALUATE_ANALYTICS_CHART", "true")

	r := RetrieveChartEvaluationDataTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	t.Cleanup(r.TearDownSuite)

	const catRoot = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1"
	const catSub = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1-2"
	v3TemplatesResp := &momentpb.ListMomentsResponse{Moments: []*momentpb.Moment{
		v3Template("tdv3-cat-1", ""),
		v3Template("tdv3-cat-1-1", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2-1", "tdv3-cat-1-2"),
	}}
	resourceACL := &auth.ResourceACL{IsRootAccess: true}

	clusterGroup := &dashboardingpb.ChartGroupBy{
		GroupByMode: &dashboardingpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardingpb.ChartGroupBy_ProvidedValue{
				ValueSet: &dashboardingpb.ChartValueSet{
					Values: &dashboardingpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardingpb.ChartValueSet_Single{
							AttributeValues: []*dashboardingpb.ChartAttributeValue{
								{
									Value: &dashboardingpb.ChartAttributeValue_ClusterValue{
										ClusterValue: &dashboardingpb.ClusterForChart{
											MomentName:   catRoot,
											DetailedType: dashboardingpb.ClusterForChart_CONVERSATION_REASON_V3,
										},
									},
								},
								{
									Value: &dashboardingpb.ChartAttributeValue_ClusterValue{
										ClusterValue: &dashboardingpb.ClusterForChart{
											MomentName:   catSub,
											DetailedType: dashboardingpb.ClusterForChart_CONVERSATION_REASON_V3,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Config: &dashboardingpb.ChartGroupBy_Config{
			GroupbyPriority: dashboardingpb.ChartGroupBy_Config_PRIMARY,
		},
	}
	timeRangeFilter := &dashboardingpb.ChartDataFilter{
		Operator: dashboardingpb.ChartDataFilter_IN,
		ValueSet: &dashboardingpb.ChartValueSet{
			Values: &dashboardingpb.ChartValueSet_RangeValues{
				RangeValues: &dashboardingpb.ChartValueSet_Range{
					RangeValues: []*dashboardingpb.ChartValueRange{
						{
							FromValue: &dashboardingpb.ChartAttributeValue{
								Value: &dashboardingpb.ChartAttributeValue_Timestamp{
									Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
								},
							},
							ToValue: &dashboardingpb.ChartAttributeValue{
								Value: &dashboardingpb.ChartAttributeValue_Timestamp{
									Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
					},
				},
			},
		},
	}

	// A single-emotion metric (unlike the two-emotion createEmotionMetric helper) does not add an extra
	// grouping dimension: the one emotion collapses into the metric computation itself, so the main result
	// stays one-dimensional (one number per cluster bucket).
	singleEmotionMetric := &dashboardingpb.ChartMetric{
		Metric: &dashboardingpb.ChartMetric_ForEmotion_{
			ForEmotion: &dashboardingpb.ChartMetric_ForEmotion{
				StatsType: &dashboardingpb.StatsTypeForEmotion{
					StatsType: &dashboardingpb.StatsTypeForEmotion_CommonStatsType{
						CommonStatsType: dashboardingpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
					},
				},
				Emotions: []*dashboardingpb.EmotionForChart{
					{
						MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
					},
				},
			},
		},
	}
	evaluateReq := r.createBasicRequest(
		[]*dashboardingpb.ChartMetric{singleEmotionMetric},
		[]*dashboardingpb.ChartGroupBy{clusterGroup},
		[]*dashboardingpb.ChartDataFilter{timeRangeFilter},
	)
	// createBasicRequest sets a non-resource-name chart name ("test_chart") that fails AnalyticsChart
	// resource-name validation; an empty name is treated as an unsaved ad-hoc chart and is accepted.
	evaluateReq.AnalyticsChart.Name = ""
	req := &analyticspb.RetrieveChartEvaluationDataRequest{
		FetchPercentageRelatedData:    true,
		EvaluateAnalyticsChartRequest: evaluateReq,
	}

	mainReqPath := "RetrieveChartEvaluationData_tdv3_cluster_groupby_emotion_main_es_request.json"
	primaryGroupReqPath := "RetrieveChartEvaluationData_tdv3_cluster_groupby_grouped_es_request.json"
	ungroupedReqPath := "RetrieveChartEvaluationData_tdv3_cluster_groupby_ungrouped_es_request.json"
	groupsAsFiltersReqPath := "RetrieveChartEvaluationData_tdv3_cluster_groupby_emotion_groups_as_filters_es_request.json"
	// A single response satisfies every fanned-out query shape: the grouped queries read the two cluster
	// filter buckets (doc_count / metrics_conversation_count 100, 50); the ungrouped and groups-as-filters
	// queries read the top-level metrics_conversation_count cardinality (150). Because every response is
	// byte-identical, the mock ES server's positional response selection stays deterministic under the
	// concurrent fan-out.
	esRespPath := "RetrieveChartEvaluationData_tdv3_cluster_groupby_es_response.json"

	// Given
	// Four concurrent EvaluateAnalyticsChart calls => four distinct ES searches (the emotion metric survives
	// only on main and groups-as-filters; ungrouped and primary-group-only replace it with plain conversation
	// count). Appending a few extra identical responses guards the mock's positional Resps indexing against
	// the concurrent fan-out.
	for i := 0; i < 8; i++ {
		r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(esRespPath))
	}
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(mainReqPath), cmputil.IgnoreJSONFields("fields"))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(primaryGroupReqPath), cmputil.IgnoreJSONFields("fields"))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(ungroupedReqPath), cmputil.IgnoreJSONFields("fields"))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(groupsAsFiltersReqPath), cmputil.IgnoreJSONFields("fields"))

	cConfig := nonpubconfig.CustomerConfig{
		Customer: "customers/customer_id",
		ProfileConfigs: []*nonpubconfig.ProfileConfig{
			{Name: "customers/customer_id/profiles/profile_id"},
		},
		AuthConfig: &nonpubconfig.CustomerAuthConfig{
			EnableAdvancedDataAccessControl: true,
		},
	}
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	// The four fan-out calls each resolve profile config, customer config, and the resource ACL, and every
	// call that references clusters (main, groups-as-filters, primary-group-only) fetches the V3 tree once.
	// Use AnyTimes so the test is not brittle to the exact fan-out call counts.
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(testCustomerConfig.ListProfiles()[0], nil)
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(testCustomerConfig, nil)
	r.setupACLHelper(gomock.Any(), -1, resourceACL)
	r.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).AnyTimes().
		DoAndReturn(func(_ context.Context, _ *momentpb.ListMomentsRequest, _ ...grpc.CallOption) (*momentpb.ListMomentsResponse, error) {
			return v3TemplatesResp, nil
		})

	// When
	resp, err := r.Client.RetrieveChartEvaluationData(r.ctx, req)

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.GetEvaluationData().GetMetricsEvaluationData(), 1, "one conversation-count metric => one metric evaluation entry")

	metricData := resp.GetEvaluationData().GetMetricsEvaluationData()[0]
	// The single emotion folds into the main query as a top-level filter (not an extra bucketing level), so the
	// main result stays one-dimensional: one number per cluster bucket (100, 50).
	groupedNumbers := metricData.GetAnalyticsResult().GetAnalyticsNumbers()
	r.Require().Len(groupedNumbers, 2, "main result has one number per cluster bucket")
	r.Equal(float32(100), groupedNumbers[0].GetValue(), "first cluster bucket count")
	r.Equal(float32(50), groupedNumbers[1].GetValue(), "second cluster bucket count")

	// Ungrouped and groups-as-filters queries report the whole-population conversation count (hits.total).
	r.Equal(int32(150), metricData.GetUngroupedTotal(), "ungrouped total from hits.total.value")
	r.Equal(int32(150), metricData.GetGroupsAsFiltersTotal(), "groups-as-filters total from hits.total.value")

	// primary-group-only replaces the emotion metric with a plain conversation count. Like the main
	// result, it stays one-dimensional: one number per cluster bucket (100, 50).
	primaryNumbers := metricData.GetGroupedByPrimaryResults().GetAnalyticsNumbers()
	r.Require().Len(primaryNumbers, 2, "primary-group-only result has one number per cluster bucket")
	r.Equal(float32(100), primaryNumbers[0].GetValue(), "first primary-group bucket count")
	r.Equal(float32(50), primaryNumbers[1].GetValue(), "second primary-group bucket count")
}
