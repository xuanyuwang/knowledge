// nolint:dupl
package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/featureflag"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEvaluateAnalyticsChart_CompositeAggTopBottomN(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.featureFlagStubClientConfig = featureflag.StubClientConfig{
		Bools: map[string]bool{
			"enableCompositeAggregationTopBottomN": true,
		},
	}
	r.SetupSuite()

	profileName := "customers/customer_id/profiles/profile_id"
	metadataMomentName := profileName + "/moments/metadata-moment-id"
	llmAnnotationMomentName := profileName + "/moments/llm_annotation_string_moment"

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPaths                      []string
		esRespPaths                     []string
		batchGetMomentResp              *momentpb.BatchGetMomentsResponse
		getMomentResp                   *momentpb.GetMomentResponse
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "bottom5AgentsByAverageScore",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForScorecardTemplate_{
									ForScorecardTemplate: &dashboardpb.ChartMetric_ForScorecardTemplate{
										ScorecardTemplates: []*dashboardpb.ScorecardTemplateForChart{
											{
												ScorecardTemplateName: "customers/cresta/profiles/walter-dev/scorecardTemplates/0199ba42-d3af-76b9-95db-5611ef8d15e3@73b63392",
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardTemplate{
											StatsType: &dashboardpb.StatsTypeForScorecardTemplate_AggregationOnScore{
												AggregationOnScore: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_ASC,
									ValueCountLimit: 5,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{

							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "2988548d03900891",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "a9de139bc3485a27",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "8e41e791735c67de",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "9f21643654aa2b6",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "cb4222b829bcbc80",
													}.String(),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               13,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               33,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               42,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsScorecardCardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_request_page_1.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_request_page_2.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_request_page_3.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsCardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_response_page_1.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_response_page_2.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_response_page_3.json",
			},
		},
		{
			desc:                            "top3AgentsByAverageHandleTimeInnerGroupBy",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForConversation_{
									ForConversation: &dashboardpb.ChartMetric_ForConversation{
										StatsType: &dashboardpb.StatsTypeForConversation{
											StatsType: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_{
												AggregationOnConversationAttribute: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute{
													AggregationAttribute: dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_HANDLE_TIME,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
												},
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 11, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
								},
							},
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
									},
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_LINE_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{

							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "1705aedb427bfb33",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "2a5b8ff87816f4e",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "15682adab93cc700",
													}.String(),
												},
											},
										},
									},
								},
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               200,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               194,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               140,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               122,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               110,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               102,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsCardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_top3AgentsByAverageHandleTimeInnerGroupBy_es_request.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsCardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_top3AgentsByAverageHandleTimeInnerGroupBy_es_response.json",
			},
		},
		{
			desc:                            "bottom3AgentsByAverageMessageCount",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForConversation_{
									ForConversation: &dashboardpb.ChartMetric_ForConversation{
										StatsType: &dashboardpb.StatsTypeForConversation{
											StatsType: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_{
												AggregationOnConversationAttribute: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute{
													AggregationAttribute: dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_MESSAGE_COUNT,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
												},
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 11, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_ASC,
									ValueCountLimit: 3,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-low",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-mid",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-high-of-bottom",
													}.String(),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               2,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               5,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               8,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsCardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom3AgentsByAverageMessageCount_es_request.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsCardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom3AgentsByAverageMessageCount_es_response.json",
			},
		},
		{
			desc:                            "top3MetadataByAverageDuration",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			batchGetMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: metadataMomentName,
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForConversation_{
									ForConversation: &dashboardpb.ChartMetric_ForConversation{
										StatsType: &dashboardpb.StatsTypeForConversation{
											StatsType: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_{
												AggregationOnConversationAttribute: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute{
													AggregationAttribute: dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_DURATION,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
												},
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Metadata{
											Metadata: &dashboardpb.MetadataForChart{
												MomentName: metadataMomentName,
											},
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: metadataMomentName,
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value-high",
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: metadataMomentName,
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value-mid",
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: metadataMomentName,
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value-low",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               90,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               50,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               20,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_metadataCardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_top3MetadataByAverageDuration_es_request.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_momentsCardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_top3MetadataByAverageDuration_es_response.json",
			},
		},
		{
			desc:                            "top3LlmAnnotationBySumDuration",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			getMomentResp: &momentpb.GetMomentResponse{
				Moment: &momentpb.Moment{
					Name: llmAnnotationMomentName,
					Type: momentpb.Moment_LLM_ANNOTATION,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_LlmAnnotationMomentData{
							LlmAnnotationMomentData: &momentpb.LlmAnnotationMomentData{
								ValueType: momentpb.LlmAnnotationMomentData_STRING,
							},
						},
					},
				},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForConversation_{
									ForConversation: &dashboardpb.ChartMetric_ForConversation{
										StatsType: &dashboardpb.StatsTypeForConversation{
											StatsType: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_{
												AggregationOnConversationAttribute: &dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute{
													AggregationAttribute: dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_DURATION,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_SUM,
												},
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 11, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_LlmAnnotation{
											LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
												MomentName: llmAnnotationMomentName,
											},
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
													LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
														MomentName: llmAnnotationMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
																	StringValue: "high_priority",
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
													LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
														MomentName: llmAnnotationMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
																	StringValue: "medium_priority",
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
													LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
														MomentName: llmAnnotationMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
																	StringValue: "low_priority",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               50,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               20,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_llmAnotationCardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_top3LlmAnnotationBySumDuration_es_request.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_momentsCardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_top3LlmAnnotationBySumDuration_es_response.json",
			},
		},
		{
			desc:                            "bottom5AgentsByAverageScoreWithNullValues",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForScorecardTemplate_{
									ForScorecardTemplate: &dashboardpb.ChartMetric_ForScorecardTemplate{
										ScorecardTemplates: []*dashboardpb.ScorecardTemplateForChart{
											{
												ScorecardTemplateName: "customers/cresta/profiles/walter-dev/scorecardTemplates/0199ba42-d3af-76b9-95db-5611ef8d15e3@73b63392",
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardTemplate{
											StatsType: &dashboardpb.StatsTypeForScorecardTemplate_AggregationOnScore{
												AggregationOnScore: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_ASC,
									ValueCountLimit: 5,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{

							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "2988548d03900891",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "11a5bd23e583567",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "a0719c8f7b51d234",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "5051d39d5bab6ae3",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "3c9080ba22ca6b6c",
													}.String(),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               13,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               46,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               50,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsScorecardCardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScore_es_request_page_1.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_agentsCardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_bottom5AgentsByAverageScoreWithNullValues_es_response.json",
			},
		},
		{
			// Identical chart setup to the terms-agg test
			// scorecardTemplatesss_unique_agent_aggregation_on_score (2 scorecard
			// templates, AVG on score, grouped by unique CONVERSATION_AGENT), but under
			// the enableCompositeAggregationTopBottomN feature flag with a low field
			// cardinality, so the composite aggregation path is exercised instead of the
			// terms aggregation path.
			desc:                            "scorecardTemplatesss_unique_agent_aggregation_on_score_composite_agg",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForScorecardTemplate_{
									ForScorecardTemplate: &dashboardpb.ChartMetric_ForScorecardTemplate{
										ScorecardTemplates: []*dashboardpb.ScorecardTemplateForChart{
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
											},
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardTemplate{
											StatsType: &dashboardpb.StatsTypeForScorecardTemplate_AggregationOnScore{
												AggregationOnScore: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 11, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
													ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
													ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
													},
												},
											},
										},
									},
								},
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-1",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-2",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-3",
													}.String(),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               0.11,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               0.21,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               0.31,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               0.12,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               0.32,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_scorecardTemplatesss_unique_agent_cardinality_es_request.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_scorecardTemplatesss_unique_agent_es_request_page_1.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_scorecardTemplatesss_unique_agent_cardinality_es_response.json",
				"EvaluateAnalyticsChart_CompositeAggTopBottomN_scorecardTemplatesss_unique_agent_es_response_page_1.json",
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)

			// Given
			for _, respPath := range tc.esRespPaths {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(respPath))
			}
			for _, reqPath := range tc.esReqPaths {
				r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(reqPath))
			}

			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					{
						Name: "customers/customer_id/profiles/profile_id",
					},
				},
				AuthConfig: &nonpubconfig.CustomerAuthConfig{
					EnableAdvancedDataAccessControl: tc.enableAdvancedDataAccessControl,
				},
			}
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(testCustomerConfig.ListProfiles()[0], nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}
			if tc.batchGetMomentResp != nil {
				r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(3).Return(tc.batchGetMomentResp, nil)
			}
			if tc.getMomentResp != nil {
				r.mockMomentClient.EXPECT().GetMoment(gomock.Any(), gomock.Any()).Times(3).Return(tc.getMomentResp, nil)
			}

			// When
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)

			// Then
			r.Require().NoError(err)
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_CompositeAggTopBottomNTermsAggFallback(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.featureFlagStubClientConfig = featureflag.StubClientConfig{
		Bools: map[string]bool{
			"enableCompositeAggregationTopBottomN": true,
		},
	}
	r.SetupSuite()

	profileName := "customers/customer_id/profiles/profile_id"
	llmAnnotationMomentName := profileName + "/moments/llm_annotation_string_moment"

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPaths                      []string
		esRespPaths                     []string
		batchGetMomentResp              *momentpb.BatchGetMomentsResponse
		getMomentResp                   *momentpb.GetMomentResponse
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "too_high_field_cardinality",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForScorecardCriterion_{
									ForScorecardCriterion: &dashboardpb.ChartMetric_ForScorecardCriterion{
										ScorecardCriterions: []*dashboardpb.ScorecardCriterionForChart{
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
												CriterionId:           "test-criterion-id1",
											},
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
												CriterionId:           "test-criterion-id2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardCriterion{
											StatsType: &dashboardpb.StatsTypeForScorecardCriterion_AggregationOnScore{
												AggregationOnScore: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer: dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-1",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-2",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-3",
													}.String(),
												},
											},
										},
									},
								},
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id2",
													},
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               0.11,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               0.12,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               0.21,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               0.31,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               0.32,
								},
							},
						},
					},
				},
			},
			esReqPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomNTermsAggFallback_too_high_field_cardinality_es_request.json",
				"EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_multi_criterions_es_request.json",
			},
			esRespPaths: []string{
				"EvaluateAnalyticsChart_CompositeAggTopBottomNTermsAggFallback_too_high_field_cardinality_es_response.json",
				"EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_multi_criterions_es_response.json",
			},
		},
		{
			desc:                            "uses_min_aggregation",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForScorecardCriterion_{
									ForScorecardCriterion: &dashboardpb.ChartMetric_ForScorecardCriterion{
										ScorecardCriterions: []*dashboardpb.ScorecardCriterionForChart{
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
												CriterionId:           "test-criterion-id1",
											},
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
												CriterionId:           "test-criterion-id2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardCriterion{
											StatsType: &dashboardpb.StatsTypeForScorecardCriterion_AggregationOnScore{
												AggregationOnScore: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_MIN,
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_Attribute_{
											Attribute: dashboardpb.ChartGroupBy_UniqueValue_CONVERSATION_AGENT,
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyLayer:  dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
									SortDirection: dashboardpb.ChartGroupBy_Config_ASC,
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-1",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-2",
													}.String(),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-3",
													}.String(),
												},
											},
										},
									},
								},
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id2",
													},
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               0.11,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               0.12,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               0.21,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               0.31,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               0.32,
								},
							},
						},
					},
				},
			},
			esReqPaths:  []string{"EvaluateAnalyticsChart_CompositeAggTopBottomNTermsAggFallback_uses_min_aggregation_es_request.json"},
			esRespPaths: []string{"EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_multi_criterions_es_response.json"},
		},
		{
			desc:                            "uses_doc_count",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: profileName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "LLM Annotation String GroupBy",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForConversation_{
									ForConversation: &dashboardpb.ChartMetric_ForConversation{
										StatsType: &dashboardpb.StatsTypeForConversation{
											StatsType: &dashboardpb.StatsTypeForConversation_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_RangeValues{
										RangeValues: &dashboardpb.ChartValueSet_Range{
											RangeValues: []*dashboardpb.ChartValueRange{
												{
													FromValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 11, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByUniqueValue{
									ByUniqueValue: &dashboardpb.ChartGroupBy_UniqueValue{
										UniqueValueFrom: &dashboardpb.ChartGroupBy_UniqueValue_LlmAnnotation{
											LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
												MomentName: llmAnnotationMomentName,
											},
										},
									},
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
			},
			getMomentResp: &momentpb.GetMomentResponse{
				Moment: &momentpb.Moment{
					Name: llmAnnotationMomentName,
					Type: momentpb.Moment_LLM_ANNOTATION,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_LlmAnnotationMomentData{
							LlmAnnotationMomentData: &momentpb.LlmAnnotationMomentData{
								ValueType: momentpb.LlmAnnotationMomentData_STRING,
							},
						},
					},
				},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
													LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
														MomentName: llmAnnotationMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
																	StringValue: "high_priority",
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
													LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
														MomentName: llmAnnotationMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
																	StringValue: "medium_priority",
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
													LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
														MomentName: llmAnnotationMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_StringValue{
																	StringValue: "low_priority",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               50,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               20,
								},
							},
						},
					},
				},
			},
			esReqPaths:  []string{"EvaluateAnalyticsChart_llm_annotation_string_groupby_es_request.json"},
			esRespPaths: []string{"EvaluateAnalyticsChart_llm_annotation_string_groupby_es_response.json"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)

			// Given
			for _, respPath := range tc.esRespPaths {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(respPath))
			}
			for _, reqPath := range tc.esReqPaths {
				r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(reqPath))
			}

			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					{
						Name: "customers/customer_id/profiles/profile_id",
					},
				},
				AuthConfig: &nonpubconfig.CustomerAuthConfig{
					EnableAdvancedDataAccessControl: tc.enableAdvancedDataAccessControl,
				},
			}
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(testCustomerConfig.ListProfiles()[0], nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}
			if tc.batchGetMomentResp != nil {
				r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(2).Return(tc.batchGetMomentResp, nil)
			}
			if tc.getMomentResp != nil {
				r.mockMomentClient.EXPECT().GetMoment(gomock.Any(), gomock.Any()).Times(2).Return(tc.getMomentResp, nil)
			}

			// When
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)

			// Then
			r.Require().NoError(err)
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}
