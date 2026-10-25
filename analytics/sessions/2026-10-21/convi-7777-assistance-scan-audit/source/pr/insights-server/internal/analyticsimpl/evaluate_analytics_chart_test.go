// nolint:dupl
package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	v1config "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config"
	v1featuresconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"

	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/shared-go/fn"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
)

type EvaluateAnalyticsChartTestSuite struct {
	AnalyticsV2TestSuite
}

func assertResponsesEqual(t *testing.T, expected *analyticspb.EvaluateAnalyticsChartResponse, actual *analyticspb.EvaluateAnalyticsChartResponse) {
	if diff := cmp.Diff(expected, actual,
		protocmp.Transform(), protocmp.IgnoreFields(&analyticspb.EvaluateAnalyticsChartResponse{}, "update_timestamp")); diff != "" {
		t.Fatalf("EvaluateAnalyticsChart return unexpected response with diff (-want +got):\n %v", diff)
	}
}

func (s *EvaluateAnalyticsChartTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	envutil.SetEnv("ENABLE_ACL_FILTERING_IN_EVALUATE_ANALYTICS_CHART", "true")
	s.AnalyticsV2TestSuite.SetupSuite()

	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "agent-id-1",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 1",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "1",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "agent-id-2",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 2",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "2",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "agent-id-3",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 3",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "3",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "agent-id-4",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 4",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "4",
	}).Error)

	s.mockUserClient.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		AnyTimes().
		DoAndReturn(func(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.ListUsersResponse, error) {
			users := make([]*userpb.User, 0, len(req.GetFilter().GetUserIds()))
			for _, userID := range req.GetFilter().GetUserIds() {
				users = append(users, &userpb.User{
					Name: userpb.UserName{
						CustomerID: s.customerID,
						UserID:     userID,
					}.String(),
				})
			}
			return &userpb.ListUsersResponse{Users: users}, nil
		})
}

func TestEvaluateAnalyticsChart_ScorecardTemplate(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "scorecardTemplate_as_filter",
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
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
														ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
														ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_as_filter_es_response.json",
		},

		{
			desc:                            "scorecardTemplate_as_groupby",
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
											StatsType: &dashboardpb.StatsTypeForScorecardTemplate_CommonStatsType{
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_as_groupby_es_response.json",
		},
		{
			desc:                            "scorecardTemplate_aggregation_on_score",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               100,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               200,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               300,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               400,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_aggregation_on_score_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_aggregation_on_score_es_response.json",
		},
		{
			desc:                            "scorecardTemplate_unique_agent_aggregation_on_score",
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
									GroupByValueIndexes: []int32{0},
									Value:               0.33,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               0.11,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_unique_agent_aggregation_on_score_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_unique_agent_aggregation_on_score_es_response.json",
		},
		{
			// Top-N ranking of a scorecard-template score by agent. The composite-aggregation
			// feature flag (enableCompositeAggregationTopBottomN) is left off for this suite, so
			// this exercises the terms-aggregation top-N path: the request carries a single terms
			// aggregation with "size": 5 ordered by the metric, and exactly one ES Search is issued
			// (composite aggregation would first issue a separate cardinality-check request).
			desc:                            "scorecardTemplate_top5_agents_by_score_terms_agg",
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
								Config: &dashboardpb.ChartGroupBy_Config{
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-4",
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
									Value:               0.44,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               0.33,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               0.11,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_top5_agents_by_score_terms_agg_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_top5_agents_by_score_terms_agg_es_response.json",
		},
		{
			desc:                            "scorecardTemplatesss_aggregation_on_score",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
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
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplatesss_aggregation_on_score_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplatesss_aggregation_on_score_es_response.json",
		},
		{
			desc:                            "scorecardTemplatesss_unique_agent_aggregation_on_score",
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
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplatesss_unique_agent_aggregation_on_score_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplatesss_unique_agent_aggregation_on_score_es_response.json",
		},
		{
			desc: "scorecardTemplate_withReviewer_as_filter",
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
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
														ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
														ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardSubmitterUserName{
														ScorecardSubmitterUserName: "customers/customer_id/users/user-1",
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardSubmitterUserName{
														ScorecardSubmitterUserName: "customers/customer_id/users/user-2",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_reviewer_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_as_filter_es_response.json",
		},
		{
			desc: "scorecardTemplate_withScorecardStatus_as_filter",
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
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
														ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardTemplateValue{
														ScorecardTemplateValue: &dashboardpb.ScorecardTemplateForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardStatusFilter{
														ScorecardStatusFilter: &dashboardpb.ScorecardStatusFilter{
															ScorecardStatuses: []dashboardpb.ScorecardStatus{
																dashboardpb.ScorecardStatus_SCORECARD_STATUS_MANUALLY_SUBMITTED,
															},
															AppealScorecardStatuses: []dashboardpb.AppealScorecardStatus{
																dashboardpb.AppealScorecardStatus_APPEAL_SCORECARD_STATUS_REQUESTED,
															},
															CalibrationScorecardStatuses: []dashboardpb.CalibrationScorecardStatus{
																dashboardpb.CalibrationScorecardStatus_CALIBRATION_SCORECARD_STATUS_DRAFT,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_scorecardTemplate_scorecardStatus_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecardTemplate_as_filter_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
				GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
				Return(testCustomerConfig.ListProfiles()[0], nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ScorecardCriterion(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "criterion_as_filter",
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
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
														ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
															ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
															CriterionId:           "test-criterion-id1",
														},
													},
												},
												{
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
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_as_filter_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_as_groupby",
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
											StatsType: &dashboardpb.StatsTypeForScorecardCriterion_CommonStatsType{
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_as_groupby_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_as_groupby_including_not_applicable",
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
												Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
													{
														Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{
															NotApplicable: true,
														},
													},
												},
												CriterionId: "test-criterion-id2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardCriterion{
											StatsType: &dashboardpb.StatsTypeForScorecardCriterion_CommonStatsType{
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
														Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
															{
																Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{
																	NotApplicable: true,
																},
															},
														},
														CriterionId: "test-criterion-id2",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_as_groupby_including_not_applicable_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_as_groupby_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_values_as_groupby",
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
												Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
													{
														Value: &dashboardpb.ScorecardCriterionForChart_Score_NumericValue{
															NumericValue: 1,
														},
													},
												},
											},
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
												CriterionId:           "test-criterion-id1",
												Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
													{
														Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{
															NotApplicable: true,
														},
													},
												},
											},
											{
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
												CriterionId:           "test-criterion-id1",
												Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
													{
														Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{
															NotApplicable: true,
														},
													},
													{
														Value: &dashboardpb.ScorecardCriterionForChart_Score_NumericValue{
															NumericValue: 1,
														},
													},
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForScorecardCriterion{
											StatsType: &dashboardpb.StatsTypeForScorecardCriterion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id1",
														Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
															{
																Value: &dashboardpb.ScorecardCriterionForChart_Score_NumericValue{
																	NumericValue: 1,
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id1",
														Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
															{
																Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{
																	NotApplicable: true,
																},
															},
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ScorecardCriterionValue{
													ScorecardCriterionValue: &dashboardpb.ScorecardCriterionForChart{
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_1@revision_id",
														CriterionId:           "test-criterion-id1",
														Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
															{
																Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{
																	NotApplicable: true,
																},
															},
															{
																Value: &dashboardpb.ScorecardCriterionForChart_Score_NumericValue{
																	NumericValue: 1,
																},
															},
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               6132,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               6941,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               6141,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               600,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               644,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               599,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               600,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               644,
								},
								{
									GroupByValueIndexes: []int32{2, 2},
									Value:               599,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_values_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_values_as_groupby_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_aggregation_on_score_multi_criterions",
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
												ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
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
						GroupBy: []*dashboardpb.ChartGroupBy{},
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
														ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/scorecard_template_2@revision_id",
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
									GroupByValueIndexes: []int32{0},
									Value:               0.11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               0.22,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_multi_criterions_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_multi_criterions_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_aggregation_on_score_groupby_time_multi_criterions",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
								}, {
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
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
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_time_multi_criterions_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_time_multi_criterions_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_aggregation_on_score_groupby_unique_agent_and_time_single_criterion",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
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
									GroupByValueIndexes: []int32{0, 2},
									Value:               0.13,
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
									GroupByValueIndexes: []int32{1, 2},
									Value:               0.23,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               0.31,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               0.32,
								},
								{
									GroupByValueIndexes: []int32{2, 2},
									Value:               0.33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_and_time_single_criterion_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_and_time_single_criterion_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_aggregation_on_score_groupby_unique_agent_single_criterion",
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
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               0.33,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               0.11,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_single_criterion_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_single_criterion_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_aggregation_on_score_groupby_unique_agent_multi_criterions",
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
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_multi_criterions_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_groupby_unique_agent_multi_criterions_es_response.json",
		},
		{
			desc:                            "scorecard_criterion_aggregation_on_score_single_criterion",
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
															Timestamp: timestamppb.New(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2025, 07, 31, 0, 0, 0, 0, time.UTC)),
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_MONTH,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               0.11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               0.22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               0.33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_single_criterion_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_scorecard_criterion_aggregation_on_score_single_criterion_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ConversationHandleTimeRange(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "conversationHandleTimeRange_as_groupBy",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
																ConversationHandleTimeRange: &dashboardpb.ChartFloatRange{
																	LowerBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       10,
																		IsExclusive: true,
																	},
																	UpperBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       20,
																		IsExclusive: false,
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
																ConversationHandleTimeRange: &dashboardpb.ChartFloatRange{
																	LowerBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       20,
																		IsExclusive: true,
																	},
																	UpperBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       30,
																		IsExclusive: false,
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
												Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
													ConversationHandleTimeRange: &dashboardpb.ChartFloatRange{
														LowerBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       10,
															IsExclusive: true,
														},
														UpperBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       20,
															IsExclusive: false,
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
													ConversationHandleTimeRange: &dashboardpb.ChartFloatRange{
														LowerBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       20,
															IsExclusive: true,
														},
														UpperBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       30,
															IsExclusive: false,
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{0, 3},
									Value:               40,
								},
								{
									GroupByValueIndexes: []int32{0, 4},
									Value:               50,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               15,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               25,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               35,
								},
								{
									GroupByValueIndexes: []int32{1, 3},
									Value:               45,
								},
								{
									GroupByValueIndexes: []int32{1, 4},
									Value:               55,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationHandleTimeRange_as_groupBy_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_conversationHandleTimeRange_as_groupBy_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ConversationMessageCountRange(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "conversationMessageCountRange_as_groupBy",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationMessageCountRange{
																ConversationMessageCountRange: &dashboardpb.ChartFloatRange{
																	LowerBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       10,
																		IsExclusive: true,
																	},
																	UpperBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       20,
																		IsExclusive: false,
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationMessageCountRange{
																ConversationMessageCountRange: &dashboardpb.ChartFloatRange{
																	LowerBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       20,
																		IsExclusive: true,
																	},
																	UpperBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       30,
																		IsExclusive: false,
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
												Value: &dashboardpb.ChartAttributeValue_ConversationMessageCountRange{
													ConversationMessageCountRange: &dashboardpb.ChartFloatRange{
														LowerBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       10,
															IsExclusive: true,
														},
														UpperBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       20,
															IsExclusive: false,
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ConversationMessageCountRange{
													ConversationMessageCountRange: &dashboardpb.ChartFloatRange{
														LowerBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       20,
															IsExclusive: true,
														},
														UpperBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       30,
															IsExclusive: false,
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{0, 3},
									Value:               40,
								},
								{
									GroupByValueIndexes: []int32{0, 4},
									Value:               50,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               15,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               25,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               35,
								},
								{
									GroupByValueIndexes: []int32{1, 3},
									Value:               45,
								},
								{
									GroupByValueIndexes: []int32{1, 4},
									Value:               55,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationMessageCountRange_as_groupBy_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_conversationMessageCountRange_as_groupBy_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ConversationNumHoldsRange(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "conversationNumHoldsRange_as_groupBy",
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
													AggregationAttribute: dashboardpb.StatsTypeForConversation_AggregationOnConversationAttribute_CONVERSATION_NUM_HOLDS,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
																ConversationNumHoldsRange: &dashboardpb.ChartFloatRange{
																	LowerBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       1,
																		IsExclusive: true,
																	},
																	UpperBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       3,
																		IsExclusive: false,
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
																ConversationNumHoldsRange: &dashboardpb.ChartFloatRange{
																	LowerBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       3,
																		IsExclusive: true,
																	},
																	UpperBound: &dashboardpb.ChartFloatRange_Bound{
																		Value:       5,
																		IsExclusive: false,
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
												Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
													ConversationNumHoldsRange: &dashboardpb.ChartFloatRange{
														LowerBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       1,
															IsExclusive: true,
														},
														UpperBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       3,
															IsExclusive: false,
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
													ConversationNumHoldsRange: &dashboardpb.ChartFloatRange{
														LowerBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       3,
															IsExclusive: true,
														},
														UpperBound: &dashboardpb.ChartFloatRange_Bound{
															Value:       5,
															IsExclusive: false,
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{0, 3},
									Value:               40,
								},
								{
									GroupByValueIndexes: []int32{0, 4},
									Value:               50,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               15,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               25,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               35,
								},
								{
									GroupByValueIndexes: []int32{1, 3},
									Value:               45,
								},
								{
									GroupByValueIndexes: []int32{1, 4},
									Value:               55,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationNumHoldsRange_as_groupBy_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_conversationNumHoldsRange_as_groupBy_es_response.json",
		},
		{
			desc:                            "conversationNumHoldsRange_as_filter",
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
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationNumHoldsRange{
														ConversationNumHoldsRange: &dashboardpb.ChartFloatRange{
															LowerBound: &dashboardpb.ChartFloatRange_Bound{
																Value:       2,
																IsExclusive: false,
															},
															UpperBound: &dashboardpb.ChartFloatRange_Bound{
																Value:       5,
																IsExclusive: false,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               100,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               200,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               150,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               300,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               250,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationNumHoldsRange_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_conversationNumHoldsRange_as_filter_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_OperaBlock(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "operablock_as_filter",
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
								Metric: &dashboardpb.ChartMetric_ForOperaBlock_{
									ForOperaBlock: &dashboardpb.ChartMetric_ForOperaBlock{
										OperaBlocks: []*dashboardpb.OperaBlockForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-from-metric",
											},
										},
										StatsType: &dashboardpb.StatsTypeForOperaBlock{
											StatsType: &dashboardpb.StatsTypeForOperaBlock_CommonStatsType{
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
														OperaBlockValue: &dashboardpb.OperaBlockForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
														OperaBlockValue: &dashboardpb.OperaBlockForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-2",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_operablock_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_operablock_as_filter_es_response.json",
		},
		{
			desc:                            "operablock_as_groupby",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
																OperaBlockValue: &dashboardpb.OperaBlockForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
																OperaBlockValue: &dashboardpb.OperaBlockForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-2",
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
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
													OperaBlockValue: &dashboardpb.OperaBlockForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_OperaBlockValue{
													OperaBlockValue: &dashboardpb.OperaBlockForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/operablock-2",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_operablock_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_operablock_as_groupby_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_Cluster(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "cluster_as_filter_and_groupby",
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
															Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ClusterValue{
														ClusterValue: &dashboardpb.ClusterForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ClusterValue{
														ClusterValue: &dashboardpb.ClusterForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-2",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_ClusterValue{
																ClusterValue: &dashboardpb.ClusterForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-3",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_ClusterValue{
																ClusterValue: &dashboardpb.ClusterForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-4",
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
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_ClusterValue{
													ClusterValue: &dashboardpb.ClusterForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-3",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ClusterValue{
													ClusterValue: &dashboardpb.ClusterForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-4",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_cluster_as_filter_and_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_cluster_as_filter_and_groupby_es_response.json",
		},
		{
			desc:                            "cluster_as_metric_conversation_count",
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
								Metric: &dashboardpb.ChartMetric_ForCluster_{
									ForCluster: &dashboardpb.ChartMetric_ForCluster{
										Clusters: []*dashboardpb.ClusterForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForCluster{
											StatsType: &dashboardpb.StatsTypeForCluster_CommonStatsType{
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
			esReqPath:  "EvaluateAnalyticsChart_cluster_as_metric_conversation_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_cluster_as_filter_and_groupby_es_response.json",
		},
		{
			// Occurrence count for a non-TDv3 (TDv1/v2) cluster. Cluster annotations are
			// conversation-anchored: the metric runs on a conversation-level query that drills into the
			// cluster's message_group child document through a children aggregation. This path requires a
			// group-by unique value (here, the conversation agent) and a single cluster.
			desc:                            "cluster_as_metric_occurrence_count",
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
								Metric: &dashboardpb.ChartMetric_ForCluster_{
									ForCluster: &dashboardpb.ChartMetric_ForCluster{
										Clusters: []*dashboardpb.ClusterForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-1",
											},
										},
										StatsType: &dashboardpb.StatsTypeForCluster{
											StatsType: &dashboardpb.StatsTypeForCluster_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_cluster_as_metric_occurrence_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_cluster_as_metric_occurrence_count_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

// TestEvaluateAnalyticsChart_TopicDiscoveryV3Cluster verifies that both filtering and grouping by a V3
// category expand it to its full subtree closure. A filter OR-matches the closure in one has_child; a
// group-by collapses that same closure into ONE bucket. The category tree used here:
//
//	tdv3-cat-1
//	├── tdv3-cat-1-1              (leaf)
//	└── tdv3-cat-1-2             (non-leaf)
//	    └── tdv3-cat-1-2-1        (leaf)
//
// Grouping by tdv3-cat-1 covers the whole tree; grouping by tdv3-cat-1-2 covers only its own subtree,
// confirming each bucket expands to exactly its own descendants.
func TestEvaluateAnalyticsChart_TopicDiscoveryV3Cluster(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	const catRoot = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1"
	const catSub = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1-2"
	v3TemplatesResp := &momentpb.ListMomentsResponse{Moments: []*momentpb.Moment{
		v3Template("tdv3-cat-1", ""),
		v3Template("tdv3-cat-1-1", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2-1", "tdv3-cat-1-2"),
	}}
	resourceACL := &auth.ResourceACL{
		IsRootAccess:    true,
		ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
		ManagedGroupIDs: []string{"4", "5", "6"},
	}

	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: r.profileResourceName,
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DisplayName: "display_name",
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
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Operator: dashboardpb.ChartDataFilter_IN,
						ValueSet: &dashboardpb.ChartValueSet{
							Values: &dashboardpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardpb.ChartValueSet_Single{
									AttributeValues: []*dashboardpb.ChartAttributeValue{
										{
											Value: &dashboardpb.ChartAttributeValue_ClusterValue{
												ClusterValue: &dashboardpb.ClusterForChart{
													MomentName:   catRoot,
													DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
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
						GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
							ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ClusterValue{
														ClusterValue: &dashboardpb.ClusterForChart{
															MomentName:   catRoot,
															DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ClusterValue{
														ClusterValue: &dashboardpb.ClusterForChart{
															MomentName:   catSub,
															DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
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
					{
						GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
							ByTime: &dashboardpb.ChartGroupBy_Time{
								Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
	}

	// Each group-by category collapses to a single bucket, so there are two cluster values (the two
	// queried categories) across the three day buckets.
	wantResp := &analyticspb.EvaluateAnalyticsChartResponse{
		ResultForChart: &dashboardpb.AnalyticsResultForChart{
			AnalyticsResults: []*dashboardpb.AnalyticsResult{
				{
					GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
						{
							GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
								{
									Value: &dashboardpb.ChartAttributeValue{
										Value: &dashboardpb.ChartAttributeValue_ClusterValue{
											ClusterValue: &dashboardpb.ClusterForChart{
												MomentName:   catRoot,
												DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
											},
										},
									},
								},
								{
									Value: &dashboardpb.ChartAttributeValue{
										Value: &dashboardpb.ChartAttributeValue_ClusterValue{
											ClusterValue: &dashboardpb.ClusterForChart{
												MomentName:   catSub,
												DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
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
								{
									Value: &dashboardpb.ChartAttributeValue{
										Value: &dashboardpb.ChartAttributeValue_Timestamp{
											Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
										},
									},
								},
							},
						},
					},
					AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
						{GroupByValueIndexes: []int32{0, 0}, Value: 9710},
						{GroupByValueIndexes: []int32{0, 1}, Value: 10596},
						{GroupByValueIndexes: []int32{0, 2}, Value: 9739},
						{GroupByValueIndexes: []int32{1, 0}, Value: 688},
						{GroupByValueIndexes: []int32{1, 1}, Value: 741},
						{GroupByValueIndexes: []int32{1, 2}, Value: 664},
					},
				},
			},
		},
	}

	esReqPath := "EvaluateAnalyticsChart_topic_discovery_v3_cluster_es_request.json"
	// Two-cluster-bucket-by-three-days aggregation response: each group-by category collapses into one
	// positional bucket (tdv3-cat-1, then tdv3-cat-1-2), so two cluster buckets each span three days.
	esRespPath := "EvaluateAnalyticsChart_topic_discovery_v3_cluster_es_response.json"

	// Given
	r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(esRespPath))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(esReqPath), cmputil.IgnoreJSONFields("fields"))
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
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).Times(1).
		Return(testCustomerConfig, nil)
	r.setupACLHelper(gomock.Any(), 1, resourceACL)
	// The V3 cluster expansion issues a single ListMoments fetch for the V3 category tree. Capture the
	// request so we can assert its exact shape (parent + active-INSIGHTS-V3-TOPIC filter + page size).
	var capturedListMomentsReq *momentpb.ListMomentsRequest
	r.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...grpc.CallOption) (*momentpb.ListMomentsResponse, error) {
			capturedListMomentsReq = req
			return v3TemplatesResp, nil
		})

	// When
	resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, req)

	// Then
	r.Require().NoError(err)
	assertResponsesEqual(t, wantResp, resp)

	wantListMomentsReq := &momentpb.ListMomentsRequest{
		Parent: r.profileResourceName,
		Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
			MomentTypes:   []momentpb.Moment_Type{momentpb.Moment_TOPIC},
			DetailedTypes: []momentpb.Moment_DetailedType{momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC},
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			States:        []momentpb.Moment_State{momentpb.Moment_STATE_ACTIVE, momentpb.Moment_STATE_BUILD_IN_PROGRESS},
		},
		PageSize: 20,
	}
	r.Require().NotNil(capturedListMomentsReq)
	if diff := cmp.Diff(wantListMomentsReq, capturedListMomentsReq, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected ListMoments request (-want +got):\n%s", diff)
	}
}

// TestEvaluateAnalyticsChart_TopicDiscoveryV3ClusterMetric verifies that a V3 category used as a chart
// *metric* (ForCluster) expands to its full subtree closure, OR-ed inside a single message-anchored
// has_child filter (V3 categories are message-anchored, unlike message_group-anchored TDv1/v2 clusters).
func TestEvaluateAnalyticsChart_TopicDiscoveryV3ClusterMetric(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	const catRoot = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1"
	const catSub = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1-2"
	v3TemplatesResp := &momentpb.ListMomentsResponse{Moments: []*momentpb.Moment{
		v3Template("tdv3-cat-1", ""),
		v3Template("tdv3-cat-1-1", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2-1", "tdv3-cat-1-2"),
	}}
	resourceACL := &auth.ResourceACL{
		IsRootAccess:    true,
		ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
		ManagedGroupIDs: []string{"4", "5", "6"},
	}

	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: r.profileResourceName,
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DisplayName: "display_name",
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{
						Metric: &dashboardpb.ChartMetric_ForCluster_{
							ForCluster: &dashboardpb.ChartMetric_ForCluster{
								Clusters: []*dashboardpb.ClusterForChart{
									{
										MomentName:   catRoot,
										DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
									},
									{
										MomentName:   catSub,
										DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
									},
								},
								StatsType: &dashboardpb.StatsTypeForCluster{
									StatsType: &dashboardpb.StatsTypeForCluster_CommonStatsType{
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
						GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
							ByTime: &dashboardpb.ChartGroupBy_Time{
								Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
	}

	esReqPath := "EvaluateAnalyticsChart_topic_discovery_v3_cluster_metric_es_request.json"
	// Reuse the filters-agg-with-date-histogram response shape; this test asserts the ES request and the
	// V3 ListMoments expansion, not the numeric response values.
	esRespPath := "EvaluateAnalyticsChart_cluster_as_filter_and_groupby_es_response.json"

	// Given
	r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(esRespPath))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(esReqPath), cmputil.IgnoreJSONFields("fields"))
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
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).Times(1).
		Return(testCustomerConfig, nil)
	r.setupACLHelper(gomock.Any(), 1, resourceACL)
	// The V3 cluster expansion issues a single ListMoments fetch for the V3 category tree.
	var capturedListMomentsReq *momentpb.ListMomentsRequest
	r.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...grpc.CallOption) (*momentpb.ListMomentsResponse, error) {
			capturedListMomentsReq = req
			return v3TemplatesResp, nil
		})

	// When
	_, err := r.Client.EvaluateAnalyticsChart(r.ctx, req)

	// Then
	r.Require().NoError(err)

	wantListMomentsReq := &momentpb.ListMomentsRequest{
		Parent: r.profileResourceName,
		Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
			MomentTypes:   []momentpb.Moment_Type{momentpb.Moment_TOPIC},
			DetailedTypes: []momentpb.Moment_DetailedType{momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC},
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			States:        []momentpb.Moment_State{momentpb.Moment_STATE_ACTIVE, momentpb.Moment_STATE_BUILD_IN_PROGRESS},
		},
		PageSize: 20,
	}
	r.Require().NotNil(capturedListMomentsReq)
	if diff := cmp.Diff(wantListMomentsReq, capturedListMomentsReq, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected ListMoments request (-want +got):\n%s", diff)
	}
}

// TestEvaluateAnalyticsChart_TopicDiscoveryV3ClusterOccurrenceCount verifies the occurrence-count
// metric path for a V3 category cluster grouped by time. A V3 category is message-anchored, so the
// cluster becomes a message-level data filter and the count is matching message docs bucketed by day
// (plain date_histogram). Its subtree closure is OR-ed inside the nested moment_annotations filter.
func TestEvaluateAnalyticsChart_TopicDiscoveryV3ClusterOccurrenceCount(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	const catRoot = "customers/customer_id/profiles/profile_id/moments/tdv3-cat-1"
	v3TemplatesResp := &momentpb.ListMomentsResponse{Moments: []*momentpb.Moment{
		v3Template("tdv3-cat-1", ""),
		v3Template("tdv3-cat-1-1", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2", "tdv3-cat-1"),
		v3Template("tdv3-cat-1-2-1", "tdv3-cat-1-2"),
	}}
	resourceACL := &auth.ResourceACL{
		IsRootAccess:    true,
		ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
		ManagedGroupIDs: []string{"4", "5", "6"},
	}

	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: r.profileResourceName,
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DisplayName: "display_name",
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{
						Metric: &dashboardpb.ChartMetric_ForCluster_{
							ForCluster: &dashboardpb.ChartMetric_ForCluster{
								Clusters: []*dashboardpb.ClusterForChart{
									{
										MomentName:   catRoot,
										DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
									},
								},
								StatsType: &dashboardpb.StatsTypeForCluster{
									StatsType: &dashboardpb.StatsTypeForCluster_CommonStatsType{
										CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
						GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
							ByTime: &dashboardpb.ChartGroupBy_Time{
								Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
	}

	// The metric value for each day bucket is the message-count occurrence total for messages annotated
	// anywhere in the V3 subtree closure, so the three day buckets report 6132/6941/6141.
	wantResp := &analyticspb.EvaluateAnalyticsChartResponse{
		ResultForChart: &dashboardpb.AnalyticsResultForChart{
			AnalyticsResults: []*dashboardpb.AnalyticsResult{
				{
					GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
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
								{
									Value: &dashboardpb.ChartAttributeValue{
										Value: &dashboardpb.ChartAttributeValue_Timestamp{
											Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
										},
									},
								},
							},
						},
					},
					AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
						{GroupByValueIndexes: []int32{0}, Value: 6132},
						{GroupByValueIndexes: []int32{1}, Value: 6941},
						{GroupByValueIndexes: []int32{2}, Value: 6141},
					},
				},
			},
		},
	}

	esReqPath := "EvaluateAnalyticsChart_topic_discovery_v3_cluster_occurrence_count_es_request.json"
	// Reuse the sentiment message-anchored occurrence-count response: a date_histogram whose day buckets
	// each carry a total occurrence count. The response shape is identical for any message-anchored
	// occurrence-count-by-time metric, so it does not need a V3-specific copy.
	esRespPath := "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_single_entity_es_response.json"

	// Given
	r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(esRespPath))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(esReqPath), cmputil.IgnoreJSONFields("fields"))
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
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).Times(1).
		Return(testCustomerConfig, nil)
	r.setupACLHelper(gomock.Any(), 1, resourceACL)
	// The V3 cluster expansion issues a single ListMoments fetch for the V3 category tree.
	var capturedListMomentsReq *momentpb.ListMomentsRequest
	r.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...grpc.CallOption) (*momentpb.ListMomentsResponse, error) {
			capturedListMomentsReq = req
			return v3TemplatesResp, nil
		})

	// When
	resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, req)

	// Then
	r.Require().NoError(err)
	assertResponsesEqual(t, wantResp, resp)

	wantListMomentsReq := &momentpb.ListMomentsRequest{
		Parent: r.profileResourceName,
		Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
			MomentTypes:   []momentpb.Moment_Type{momentpb.Moment_TOPIC},
			DetailedTypes: []momentpb.Moment_DetailedType{momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC},
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			States:        []momentpb.Moment_State{momentpb.Moment_STATE_ACTIVE, momentpb.Moment_STATE_BUILD_IN_PROGRESS},
		},
		PageSize: 20,
	}
	r.Require().NotNil(capturedListMomentsReq)
	if diff := cmp.Diff(wantListMomentsReq, capturedListMomentsReq, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected ListMoments request (-want +got):\n%s", diff)
	}
}

// TestEvaluateAnalyticsChart_NonV3ClusterOccurrenceCountByTimeUnsupported locks in that occurrence
// count for a TDv1/v2 (non-V3) cluster grouped by time is rejected with InvalidArgument: it drives a
// message-level query, but TDv1/v2 cluster annotations live on the message_group child, which is
// unreachable from a message-level query (unlike message-anchored V3 categories).
func TestEvaluateAnalyticsChart_NonV3ClusterOccurrenceCountByTimeUnsupported(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: r.profileResourceName,
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			DisplayName: "display_name",
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{
						Metric: &dashboardpb.ChartMetric_ForCluster_{
							ForCluster: &dashboardpb.ChartMetric_ForCluster{
								Clusters: []*dashboardpb.ClusterForChart{
									{
										MomentName: "customers/customer_id/profiles/profile_id/moments/cluster-1",
									},
								},
								StatsType: &dashboardpb.StatsTypeForCluster{
									StatsType: &dashboardpb.StatsTypeForCluster_CommonStatsType{
										CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
						GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
							ByTime: &dashboardpb.ChartGroupBy_Time{
								Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
	}

	// Given
	cConfig := nonpubconfig.CustomerConfig{
		Customer: "customers/customer_id",
		ProfileConfigs: []*nonpubconfig.ProfileConfig{
			{Name: "customers/customer_id/profiles/profile_id"},
		},
	}
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(testCustomerConfig.ListProfiles()[0], nil)
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(testCustomerConfig, nil)

	// When
	_, err := r.Client.EvaluateAnalyticsChart(r.ctx, req)

	// Then
	r.Require().Error(err)
	r.Equal(codes.InvalidArgument, status.Code(err))
}

func TestEvaluateAnalyticsChart_Emotion(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                                                        string
		enableAdvancedDataAccessControl                             bool
		enableReadingDenormalizedConversationMetadataFromMessageDoc bool
		denormalizedConversationMetadataMomentTemplateIds           []string
		resourceACL                                                 *auth.ResourceACL
		esReqPath                                                   string
		esRespPath                                                  string
		req                                                         *analyticspb.EvaluateAnalyticsChartRequest
		resp                                                        *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "emotion_as_filter",
			enableAdvancedDataAccessControl: true,
			enableReadingDenormalizedConversationMetadataFromMessageDoc: true,
			denormalizedConversationMetadataMomentTemplateIds: []string{
				"numeric-metadata-with-value",
			},
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_EmotionValue{
														EmotionValue: &dashboardpb.EmotionForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_EmotionValue{
														EmotionValue: &dashboardpb.EmotionForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_emotion_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_emotion_as_filter_es_response.json",
		},
		{
			desc:                            "emotion_as_groupby",
			enableAdvancedDataAccessControl: true,
			enableReadingDenormalizedConversationMetadataFromMessageDoc: true,
			denormalizedConversationMetadataMomentTemplateIds: []string{
				"numeric-metadata-with-value",
			},
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_EmotionValue{
																EmotionValue: &dashboardpb.EmotionForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_EmotionValue{
																EmotionValue: &dashboardpb.EmotionForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
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
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_EmotionValue{
													EmotionValue: &dashboardpb.EmotionForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_EmotionValue{
													EmotionValue: &dashboardpb.EmotionForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_emotion_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_emotion_as_groupby_es_response.json",
		},
		{
			desc:                            "emotion_occurrence_with_denormalized_metadata_filter",
			enableAdvancedDataAccessControl: true,
			enableReadingDenormalizedConversationMetadataFromMessageDoc: true,
			denormalizedConversationMetadataMomentTemplateIds: []string{
				"numeric-metadata-with-value",
			},
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
								Metric: &dashboardpb.ChartMetric_ForEmotion_{
									ForEmotion: &dashboardpb.ChartMetric_ForEmotion{
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_EmotionValue{
														EmotionValue: &dashboardpb.EmotionForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_EmotionValue{
														EmotionValue: &dashboardpb.EmotionForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
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
							},
						},
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_EmotionValue{
													EmotionValue: &dashboardpb.EmotionForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_EmotionValue{
													EmotionValue: &dashboardpb.EmotionForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               111,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               122,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               133,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               211,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               222,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               233,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_emotion_occurrence_with_denormalized_metadata_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_emotion_occurrence_with_denormalized_metadata_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			profileConfig := &nonpubconfig.ProfileConfig{
				Name: "customers/customer_id/profiles/profile_id",
			}
			profileConfig.Features = &v1config.Features{
				Conversation: &v1featuresconfig.ConversationFeatures{
					EnableReadingDenormalizedConversationMetadataFromMessageDoc: tc.enableReadingDenormalizedConversationMetadataFromMessageDoc,
					DenormalizedConversationMetadataMomentTemplateIds:           tc.denormalizedConversationMetadataMomentTemplateIds,
				},
			}
			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					profileConfig,
				},
				AuthConfig: &nonpubconfig.CustomerAuthConfig{
					EnableAdvancedDataAccessControl: tc.enableAdvancedDataAccessControl,
				},
			}
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
				Return(testCustomerConfig.ListProfiles()[0], nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_Behavior(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "behavior_as_filter",
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
														BehaviorValue: &dashboardpb.BehaviorForChart{
															BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-1",
															AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
																dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_DO_X,
																dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_NOT_DO_X,
															},
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
														BehaviorValue: &dashboardpb.BehaviorForChart{
															BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-2",
															AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
																dashboardpb.BehaviorForChart_ADHERENCE_TYPE_SHOULD_DO_X,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_behavior_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_behavior_as_filter_es_response.json",
		},
		{
			desc:                            "behavior_as_groupby",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
																BehaviorValue: &dashboardpb.BehaviorForChart{
																	BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-1",
																	AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
																		dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_DO_X,
																		dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_NOT_DO_X,
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
																BehaviorValue: &dashboardpb.BehaviorForChart{
																	BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-2",
																	AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
																		dashboardpb.BehaviorForChart_ADHERENCE_TYPE_SHOULD_DO_X,
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
							},
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
													BehaviorValue: &dashboardpb.BehaviorForChart{
														BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-1",
														AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
															dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_DO_X,
															dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_NOT_DO_X,
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_BehaviorValue{
													BehaviorValue: &dashboardpb.BehaviorForChart{
														BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-2",
														AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
															dashboardpb.BehaviorForChart_ADHERENCE_TYPE_SHOULD_DO_X,
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_behavior_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_behavior_as_groupby_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_Sentiment(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		precisionThresholdForCardAgg    int32
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
		expectedACLCalls                int // Add field to specify expected ACL calls
	}{
		{
			desc:                            "sentiment_for_msg_as_filter",
			enableAdvancedDataAccessControl: true,
			precisionThresholdForCardAgg:    1000,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			expectedACLCalls: 2, // Has both user filters and user groupBys
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_UserNameValue{
														UserNameValue: "customers/customer_id/users/user-1",
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_UserNameValue{
														UserNameValue: "customers/customer_id/users/user-2",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/user-3",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/user-4",
															},
														},
													},
												},
											},
										},
									},
								},
							},
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: "customers/customer_id/users/user-3",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: "customers/customer_id/users/user-4",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_as_filter_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_convo_count_0_groupby",
			expectedACLCalls:                1, // Default ACL calls
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
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
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									Value: 1234,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_convo_count_0_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_convo_count_0_groupby_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_occurrence_count_0_groupby",
			expectedACLCalls:                1,
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
								Metric: &dashboardpb.ChartMetric_ForSentiment_{
									ForSentiment: &dashboardpb.ChartMetric_ForSentiment{
										Sentiments: []*dashboardpb.SentimentForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
												Type:       dashboardpb.SentimentForChart_MESSAGE,
											},
										},
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
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
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									Value: 1234,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_0_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_0_groupby_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_filter_and_groupby_usecase",
			expectedACLCalls:                1,
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
															Type:       dashboardpb.SentimentForChart_MESSAGE,
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
														UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/usecase-1",
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
														UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/usecase-2",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
																UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/usecase-1",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
																UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/usecase-2",
															},
														},
													},
												},
											},
										},
									},
								},
							},
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
													UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/usecase-1",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UsecaseNameValue{
													UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/usecase-2",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_filter_and_groupby_usecase_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_filter_and_groupby_usecase_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_as_groupby",
			expectedACLCalls:                1,
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 5, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 10, 1, 5, 0, 0, 0, time.UTC)),
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_SentimentValue{
																SentimentValue: &dashboardpb.SentimentForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
																	Type:       dashboardpb.SentimentForChart_MESSAGE,
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SentimentValue{
																SentimentValue: &dashboardpb.SentimentForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
																	Type:       dashboardpb.SentimentForChart_MESSAGE,
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
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
														Type:       dashboardpb.SentimentForChart_MESSAGE,
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
														Type:       dashboardpb.SentimentForChart_MESSAGE,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_as_groupby_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_occurrence_count",
			expectedACLCalls:                1,
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
								Metric: &dashboardpb.ChartMetric_ForSentiment_{
									ForSentiment: &dashboardpb.ChartMetric_ForSentiment{
										Sentiments: []*dashboardpb.SentimentForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
												Type:       dashboardpb.SentimentForChart_MESSAGE,
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
												Type:       dashboardpb.SentimentForChart_MESSAGE,
											},
										},
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 22, 20, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 10, 1, 22, 20, 0, 0, time.UTC)),
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
														Type:       dashboardpb.SentimentForChart_MESSAGE,
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
														Type:       dashboardpb.SentimentForChart_MESSAGE,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               6132,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               6941,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               6141,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               600,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               644,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               599,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_avg_occurrence_count_per_convo",
			expectedACLCalls:                1,
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
								Metric: &dashboardpb.ChartMetric_ForSentiment_{
									ForSentiment: &dashboardpb.ChartMetric_ForSentiment{
										Sentiments: []*dashboardpb.SentimentForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
												Type:       dashboardpb.SentimentForChart_MESSAGE,
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
												Type:       dashboardpb.SentimentForChart_MESSAGE,
											},
										},
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_AggregationOnOccurrenceCountPerConversation{
												AggregationOnOccurrenceCountPerConversation: &dashboardpb.AggregationOnOccurrenceCountPerConversation{
													AggregationOperator: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 22, 20, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 10, 1, 22, 20, 0, 0, time.UTC)),
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
														Type:       dashboardpb.SentimentForChart_MESSAGE,
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
														Type:       dashboardpb.SentimentForChart_MESSAGE,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               2,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               3,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               0,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_avg_occurrence_count_per_convo_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_avg_occurrence_count_per_convo_es_response.json",
		},
		{
			desc:                            "sentiment_for_msg_occurrence_count_single_entity",
			expectedACLCalls:                1,
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
								Metric: &dashboardpb.ChartMetric_ForSentiment_{
									ForSentiment: &dashboardpb.ChartMetric_ForSentiment{
										Sentiments: []*dashboardpb.SentimentForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
												Type:       dashboardpb.SentimentForChart_MESSAGE,
											},
										},
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               6132,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               6941,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               6141,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_single_entity_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_msg_occurrence_count_single_entity_es_response.json",
		},
		{
			desc:                            "sentiment_for_convo_as_as_filter",
			expectedACLCalls:                1,
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
															Type:       dashboardpb.SentimentForChart_CONVERSATION,
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
															Type:       dashboardpb.SentimentForChart_CONVERSATION,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_convo_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_convo_as_filter_es_response.json",
		},
		{
			desc:                            "sentiment_for_convo_as_groupby",
			expectedACLCalls:                1,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_SentimentValue{
																SentimentValue: &dashboardpb.SentimentForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
																	Type:       dashboardpb.SentimentForChart_CONVERSATION,
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SentimentValue{
																SentimentValue: &dashboardpb.SentimentForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
																	Type:       dashboardpb.SentimentForChart_CONVERSATION,
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
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
														Type:       dashboardpb.SentimentForChart_CONVERSATION,
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
														Type:       dashboardpb.SentimentForChart_CONVERSATION,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_convo_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_convo_as_groupby_es_response.json",
		},
		{
			desc:                            "sentiment_for_convo_occurrence_count",
			expectedACLCalls:                1,
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
								Metric: &dashboardpb.ChartMetric_ForSentiment_{
									ForSentiment: &dashboardpb.ChartMetric_ForSentiment{
										Sentiments: []*dashboardpb.SentimentForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
												Type:       dashboardpb.SentimentForChart_CONVERSATION,
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
												Type:       dashboardpb.SentimentForChart_CONVERSATION,
											},
										},
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
														Type:       dashboardpb.SentimentForChart_CONVERSATION,
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-2",
														Type:       dashboardpb.SentimentForChart_CONVERSATION,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               6132,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               6941,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               6141,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               600,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               644,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               599,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sentiment_for_convo_occurrence_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sentiment_for_convo_occurrence_count_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					{
						Name: "customers/customer_id/profiles/profile_id",
						Features: &v1config.Features{
							Insights: &v1featuresconfig.Insights{
								PrecisionThresholdForElasticsearchCardinalityAgg: tc.precisionThresholdForCardAgg,
							},
						},
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

			// Use expectedACLCalls from test case, default to 1 if not specified
			expectedCalls := tc.expectedACLCalls
			if expectedCalls == 0 {
				expectedCalls = 1
			}

			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(expectedCalls).
				Return(testCustomerConfig, nil)
			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), expectedCalls, tc.resourceACL)
			}
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_BehaviorHint(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "behavior_hint_as_filter",
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
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
														AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
															Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
															AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
																dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_USED,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               40,
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
			esReqPath:  "EvaluateAnalyticsChart_behavior_hint_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_behavior_hint_as_filter_es_response.json",
		},
		{
			desc:                            "behavior_hint_as_groupby",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
																AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
																	Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
																	AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
																		dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_SENT,
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
																AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
																	Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
																	AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
																		dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_USED,
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
							},
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
													AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
														Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
														AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
															dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_SENT,
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
													AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
														Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
														AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
															dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_USED,
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               9710,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10596,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               9739,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               688,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               741,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               664,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_behavior_hint_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_behavior_hint_as_groupby_es_response.json",
		},
		{
			desc:                            "behavior_hint_occurrence_count",
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
								Metric: &dashboardpb.ChartMetric_ForAgentAssistance_{
									ForAgentAssistance: &dashboardpb.ChartMetric_ForAgentAssistance{
										AgentAssistances: []*dashboardpb.AgentAssistanceForChart{
											{
												Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
												AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
													dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_SENT,
												},
											},
											{
												Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
												AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
													dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_USED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForAgentAssistance{
											StatsType: &dashboardpb.StatsTypeForAgentAssistance_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 1, 22, 20, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2024, 10, 1, 22, 20, 0, 0, time.UTC)),
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
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
												Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
													AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
														Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
														AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
															dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_SENT,
														},
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AgentAssistanceValue{
													AgentAssistanceValue: &dashboardpb.AgentAssistanceForChart{
														Value: &dashboardpb.AgentAssistanceForChart_BehaiviorHint{},
														AdherenceTypes: []dashboardpb.AgentAssistanceForChart_AdherenceType{
															dashboardpb.AgentAssistanceForChart_ADHERENCE_TYPE_USED,
														},
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               6132,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               6941,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               6141,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               600,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               644,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               599,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_behavior_hint_occurrence_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_behavior_hint_occurrence_count_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ConversationSource(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "conversationSource_as_filter_IN",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationSource{
														ConversationSource: convopb.Conversation_SOURCE_UNSPECIFIED,
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationSource{
														ConversationSource: convopb.Conversation_INGESTION_PIPELINE,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationSource_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "conversationSource_as_filter_NOT_IN",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_NOT_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationSource{
														ConversationSource: convopb.Conversation_BOT,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationSource_not_in_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "conversationSource_as_groupby",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationSource{
																ConversationSource: convopb.Conversation_SOURCE_UNSPECIFIED,
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_ConversationSource{
																ConversationSource: convopb.Conversation_INGESTION_PIPELINE,
															},
														},
													},
												},
											},
										},
									},
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
												Value: &dashboardpb.ChartAttributeValue_ConversationSource{
													ConversationSource: convopb.Conversation_SOURCE_UNSPECIFIED,
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_ConversationSource{
													ConversationSource: convopb.Conversation_INGESTION_PIPELINE,
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               7591,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               7412,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               6920,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               1199,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               1218,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               2036,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationSource_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_conversationSource_as_groupby_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ConversationChannel(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "conversationChannel_as_filter_IN_single",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationChannel{
														ConversationChannel: convopb.Conversation_CHAT,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationChannel_as_filter_single_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "conversationChannel_as_filter_IN_multi",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationChannel{
														ConversationChannel: convopb.Conversation_CHAT,
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationChannel{
														ConversationChannel: convopb.Conversation_VOICE,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationChannel_as_filter_multi_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "conversationChannel_as_filter_NOT_IN",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_NOT_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationChannel{
														ConversationChannel: convopb.Conversation_EMAIL,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationChannel_not_in_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_ConversationLanguageCode(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "conversationLanguageCode_as_filter_IN_single",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationLanguageCode{
														ConversationLanguageCode: "en-US",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationLanguageCode_as_filter_single_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "conversationLanguageCode_as_filter_IN_multi",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationLanguageCode{
														ConversationLanguageCode: "en-US",
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationLanguageCode{
														ConversationLanguageCode: "es-MX",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationLanguageCode_as_filter_multi_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "conversationLanguageCode_as_filter_NOT_IN",
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Operator: dashboardpb.ChartDataFilter_NOT_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ConversationLanguageCode{
														ConversationLanguageCode: "fr-FR",
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_WEEK,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 7, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_conversationLanguageCode_not_in_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))
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
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_LlmAnnotationGroupBy(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	profileName := "customers/customer_id/profiles/profile_id"
	stringMomentName := profileName + "/moments/llm_annotation_string_moment"
	numberMomentName := profileName + "/moments/llm_annotation_number_moment"
	booleanMomentName := profileName + "/moments/llm_annotation_boolean_moment"

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
		getMomentResp                   *momentpb.GetMomentResponse
	}{
		{
			desc:                            "llm_annotation_string_value_groupby",
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
												MomentName: stringMomentName,
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
					Name: stringMomentName,
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
														MomentName: stringMomentName,
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
														MomentName: stringMomentName,
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
														MomentName: stringMomentName,
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
			esReqPath:  "EvaluateAnalyticsChart_llm_annotation_string_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_llm_annotation_string_groupby_es_response.json",
		},
		{
			desc:                            "llm_annotation_number_value_groupby",
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
					DisplayName: "LLM Annotation Number GroupBy",
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
												MomentName: numberMomentName,
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
					Name: numberMomentName,
					Type: momentpb.Moment_LLM_ANNOTATION,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_LlmAnnotationMomentData{
							LlmAnnotationMomentData: &momentpb.LlmAnnotationMomentData{
								ValueType: momentpb.LlmAnnotationMomentData_NUMBER,
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
														MomentName: numberMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_NumberValue{
																	NumberValue: 95.5,
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
														MomentName: numberMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_NumberValue{
																	NumberValue: 85.0,
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
														MomentName: numberMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_NumberValue{
																	NumberValue: 70.5,
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
									Value:               40,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               35,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               25,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_llm_annotation_number_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_llm_annotation_number_groupby_es_response.json",
		},
		{
			desc:                            "llm_annotation_boolean_value_groupby",
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
					DisplayName: "LLM Annotation Boolean GroupBy",
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
												MomentName: booleanMomentName,
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
					Name: booleanMomentName,
					Type: momentpb.Moment_LLM_ANNOTATION,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_LlmAnnotationMomentData{
							LlmAnnotationMomentData: &momentpb.LlmAnnotationMomentData{
								ValueType: momentpb.LlmAnnotationMomentData_BOOLEAN,
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
														MomentName: booleanMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_BooleanValue{
																	BooleanValue: true,
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
														MomentName: booleanMomentName,
														Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
															{
																Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_BooleanValue{
																	BooleanValue: false,
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
									Value:               60,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               40,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_llm_annotation_boolean_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_llm_annotation_boolean_groupby_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)

			// Given
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))

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

			r.mockMomentClient.EXPECT().
				GetMoment(gomock.Any(), gomock.Any()).
				Times(2).
				Return(tc.getMomentResp, nil)

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
