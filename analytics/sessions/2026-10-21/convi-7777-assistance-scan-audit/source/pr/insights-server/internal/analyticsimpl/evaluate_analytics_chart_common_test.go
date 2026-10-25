package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/time"
	v1config "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config"
	v1featuresconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
	"github.com/cresta/shared-go/fn"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/suite"
	"google.golang.org/genproto/googleapis/type/dayofweek"
	"google.golang.org/genproto/googleapis/type/timeofday"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
)

type SubcategoryDeduplicationTestSuite struct {
	suite.Suite
	ctx context.Context
}

func assertCommonResponsesEqual(t *testing.T, expected *analyticspb.EvaluateAnalyticsChartResponse, actual *analyticspb.EvaluateAnalyticsChartResponse) {
	if diff := cmp.Diff(expected, actual,
		protocmp.Transform(), protocmp.IgnoreFields(&analyticspb.EvaluateAnalyticsChartResponse{}, "update_timestamp")); diff != "" {
		t.Fatalf("EvaluateAnalyticsChart return unexpected response with diff (-want +got):\n %v", diff)
	}
}

func TestSubcategoryDeduplication(t *testing.T) {
	suite.Run(t, new(SubcategoryDeduplicationTestSuite))
}

func (s *SubcategoryDeduplicationTestSuite) SetupTest() {
	s.ctx = context.Background()
}

type EvaluateAnalyticsChartCommonTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *EvaluateAnalyticsChartCommonTestSuite) SetupSuite() {
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
}

func TestEvaluateAnalyticsChart_Subcategory(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
		getCompositeMomentResp          *momentpb.BatchGetMomentsResponse
		getChildMomentResp              *momentpb.BatchGetMomentsResponse
	}{
		{
			desc:                            "subcategory_as_filter",
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
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_filter_es_response.json",
		},
		{
			desc:                            "subcategory_filter_one_included_one_excluded",
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
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
														},
													},
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_INCLUDE,
												},
												{
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
														},
													},
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_1"}},
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_2"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_2"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_2_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
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
								{GroupByValueIndexes: []int32{0}, Value: 10},
								{GroupByValueIndexes: []int32{1}, Value: 20},
								{GroupByValueIndexes: []int32{2}, Value: 30},
								{GroupByValueIndexes: []int32{3}, Value: 40},
								{GroupByValueIndexes: []int32{4}, Value: 50},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_filter_one_included_one_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_filter_one_included_one_excluded_es_response.json",
		},
		{
			desc:                            "subcategory_filter_two_excluded",
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
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
														},
													},
												},
												{
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_2",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_2_keyword_excluded_2"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_2_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_filter_two_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_filter_two_excluded_es_response.json",
		},
		{
			desc:                            "subcategory_filter_one_excluded",
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
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_filter_one_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_filter_one_excluded_es_response.json",
		},
		{
			desc:                            "subcategory_groupby_one_included_one_excluded",
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_INCLUDE,
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_1"}},
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_2"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_2"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_2_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
								{GroupByValueIndexes: []int32{0, 0}, Value: 10},
								{GroupByValueIndexes: []int32{0, 1}, Value: 20},
								{GroupByValueIndexes: []int32{0, 2}, Value: 30},
								{GroupByValueIndexes: []int32{0, 3}, Value: 40},
								{GroupByValueIndexes: []int32{0, 4}, Value: 50},
								{GroupByValueIndexes: []int32{1, 0}, Value: 60},
								{GroupByValueIndexes: []int32{1, 1}, Value: 70},
								{GroupByValueIndexes: []int32{1, 2}, Value: 80},
								{GroupByValueIndexes: []int32{1, 3}, Value: 90},
								{GroupByValueIndexes: []int32{1, 4}, Value: 100},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_groupby_one_included_one_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_groupby_one_included_one_excluded_es_response.json",
		},
		{
			desc:                            "subcategory_on_the_fly_as_filter_convo_count",
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
				ConvoFilter: &analyticspb.AnalyticsChartFilter{
					InsightsTopic: &analyticspb.InsightsTopic{
						IncludedKeywordMoment: &momentpb.Moment{
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included keyword1",
													},
												},
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included keyword2",
													},
												},
											},
										},
									},
								},
							},
						},
						ExcludedKeywordMoment: &momentpb.Moment{
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded keyword1",
													},
												},
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded keyword2",
													},
												},
											},
										},
									},
								},
							},
						},
						ClusterMoments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/cluster1",
								Type: momentpb.Moment_TOPIC,
							},
							{
								Name: "customers/customer_id/profiles/profile_id/moments/cluster2",
								Type: momentpb.Moment_TOPIC,
							},
						},
						SpeakerRoles: []string{"agent"},
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_on_the_fly_as_filter_convo_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_on_the_fly_as_filter_convo_count_es_response.json",
		},
		{
			desc:                            "subcategory_on_the_fly_as_filter_occurrence_count",
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
				ConvoFilter: &analyticspb.AnalyticsChartFilter{
					InsightsTopic: &analyticspb.InsightsTopic{
						IncludedKeywordMoment: &momentpb.Moment{
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included keyword1",
													},
												},
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included keyword2",
													},
												},
											},
										},
									},
								},
							},
						},
						ExcludedKeywordMoment: &momentpb.Moment{
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded keyword1",
													},
												},
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded keyword2",
													},
												},
											},
										},
									},
								},
							},
						},
						ClusterMoments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/cluster1",
								Type: momentpb.Moment_TOPIC,
							},
							{
								Name: "customers/customer_id/profiles/profile_id/moments/cluster2",
								Type: momentpb.Moment_TOPIC,
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_on_the_fly_as_filter_occurrence_count_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_on_the_fly_as_filter_occurrence_count_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby",
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
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_es_response.json",
		},
		{
			desc:                            "avg_convo_handle_time_group_by_subcategoryAndTime",
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
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
									Value:               1,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               2,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               3,
								},
								{
									GroupByValueIndexes: []int32{0, 3},
									Value:               4,
								},
								{
									GroupByValueIndexes: []int32{0, 4},
									Value:               5,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{1, 3},
									Value:               40,
								},
								{
									GroupByValueIndexes: []int32{1, 4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_avg_convo_handle_time_group_by_subcategoryAndTime_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_avg_convo_handle_time_group_by_subcategoryAndTime_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby_two_excluded",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "customers/customer_id/profiles/profile_id/analyticsCharts/0199bf08-3f37-7caf-9ec0-9a2cbc42c950",
					DisplayName: "",
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
								RenderingConfig: &dashboardpb.ChartMetric_RenderingConfig{
									Colors:       []string{"#228BE6", "#FD7E14", "#2F9E44", "#5246E0", "#E03131", "#15AABF", "#FAB005", "#F06595", "#868E96", "#94D82D"},
									ValueDisplay: dashboardpb.ChartMetric_RenderingConfig_ABSOLUTE,
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
															Timestamp: timestamppb.New(time.Date(2026, 1, 28, 22, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2026, 2, 27, 21, 59, 59, 999000000, time.UTC)),
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
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
								},
							},
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/excluded-subcategory-moment",
																},
															},
														},
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/excluded-subcategory-moment-2",
																},
															},
														},
													},
												},
											},
										},
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_SECONDARY,
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
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "Etc/GMT-2",
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded-subcategory-moment",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/excluded-subcategory-moment-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_2",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_excluded_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_1_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_2_keyword_excluded_2"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: "moment_2_keyword_included_1"}},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_two_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_two_excluded_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			if tc.getCompositeMomentResp != nil && tc.getChildMomentResp != nil {
				gomock.InOrder(
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getChildMomentResp, nil))
			} else if tc.getCompositeMomentResp != nil {
				r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil)
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

			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertCommonResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

// TestEvaluateAnalyticsChart_AggregateSubcategory tests analytics evaluation with aggregate subcategories.
//
// Key Concept: Aggregate subcategories are composite moments WITHOUT moment data.
// - They serve as grouping containers for child subcategories
// - Child subcategories (leaf nodes) are composite moments WITH moment data that define the actual matching logic
// - The aggregate subcategory itself has no matching logic - it only aggregates its children's results
func TestEvaluateAnalyticsChart_AggregateSubcategory(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	newKeywordMoment := func(name, phrase string) *momentpb.Moment {
		return &momentpb.Moment{
			Name: name,
			Type: momentpb.Moment_KEYWORD,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: phrase,
									},
								},
							},
						},
					},
				},
			},
		}
	}

	newKeywordMomentMatchAll := func(name string, phrases []string) *momentpb.Moment {
		operands := []*momentpb.KeywordMomentData_Operand{}
		for _, phrase := range phrases {
			operands = append(operands, &momentpb.KeywordMomentData_Operand{
				Operand: &momentpb.KeywordMomentData_Operand_Phrase{
					Phrase: phrase,
				},
			})
		}
		return &momentpb.Moment{
			Name: name,
			Type: momentpb.Moment_KEYWORD,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_AND,
							Operands: operands,
						},
					},
				},
			},
		}
	}

	newClusterMoment := func(name string) *momentpb.Moment {
		return &momentpb.Moment{
			Name: name,
			Type: momentpb.Moment_TOPIC,
		}
	}

	// newAggregateSubcategoryMoment creates an aggregate subcategory moment.
	// Aggregate subcategories are composite moments WITHOUT moment data - they serve as grouping containers
	// for child subcategories that do have moment data.
	newAggregateSubcategoryMoment := func(name string) *momentpb.Moment {
		return &momentpb.Moment{
			Name:       name,
			Type:       momentpb.Moment_COMPOSITE,
			MomentData: &momentpb.MomentData{}, // Aggregate subcategories contain no composite moment data.
		}
	}

	// newSubcategoryMoment creates a child subcategory moment with actual moment data.
	// Unlike aggregate subcategories, these leaf subcategories have composite moment data
	// that defines the actual moment matching logic.
	newSubcategoryMoment := func(name, parent string, excluded []string, includes []string) *momentpb.Moment {
		operands := []*momentpb.CompositeMomentData_CompositeMomentOperand{}
		if len(excluded) > 0 {
			notOperands := []*momentpb.CompositeMomentData_CompositeMomentOperand{}
			for _, ex := range excluded {
				notOperands = append(notOperands, &momentpb.CompositeMomentData_CompositeMomentOperand{
					Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
						BaseMoment: ex,
					},
				})
			}
			operands = append(operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
					CompositeMoment: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_NOT,
						Operands: notOperands,
					},
				},
			})
		}
		includeOperands := []*momentpb.CompositeMomentData_CompositeMomentOperand{}
		for _, in := range includes {
			includeOperands = append(includeOperands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
					BaseMoment: in,
				},
			})
		}
		if len(includeOperands) > 0 {
			operands = append(operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
					CompositeMoment: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_OR,
						Operands: includeOperands,
					},
				},
			})
		}

		return &momentpb.Moment{
			Name:                   name,
			Type:                   momentpb.Moment_COMPOSITE,
			ParentMomentTemplateId: parent,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_CompositeMomentData{
					CompositeMomentData: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_AND,
						Operands: operands,
					},
				},
			},
		}
	}

	for _, tc := range []struct {
		desc                             string
		req                              *analyticspb.EvaluateAnalyticsChartRequest
		resp                             *analyticspb.EvaluateAnalyticsChartResponse
		listMomentsResp                  *momentpb.ListMomentsResponse
		batchGetMomentResps              []*momentpb.BatchGetMomentsResponse
		esReqPath                        string
		esRespPath                       string
		resourceACL                      *auth.ResourceACL
		enableDataAccessFilter           bool
		batchGetMomentsBeforeListMoments bool // true when prefetch contains aggregate subcategory metrics (so BatchGetMoments runs before ListMoments)
		listMomentsFirst                 bool // true when ListMoments is called before any BatchGetMoments (no prefetch)
		expectListUsers                  bool // true when a byProvidedValue groupby uses UserNameValue, triggering excludeDeactivatedUsersFromProvidedUserGroupBys
	}{
		{
			desc:                   "aggregate_subcategory_as_filter",
			listMomentsFirst:       true, // With optimization, ListMoments is called first in fetchTopicDetailsForChart
			enableDataAccessFilter: true,
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
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_as_filter_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					// Aggregate subcategories - composite moments without moment data
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					// Child subcategories - have ParentMomentTemplateId pointing to their aggregate parent
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2", "moment_1_keyword_excluded_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1", "moment_2_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2", "moment_2_keyword_excluded_2"),
						newKeywordMomentMatchAll("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1", []string{"moment_2_keyword_included_1a", "moment_2_keyword_included_1b"}),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2", "moment_2_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2"),
						// Aggregate subcategories returned in batch - composite moments without moment data
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_filter_one_included_leaf_one_excluded_leaf_one_excluded_aggregate",
			listMomentsFirst:       true, // With optimization, ListMoments is called first in fetchTopicDetailsForChart
			enableDataAccessFilter: true,
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
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_INCLUDE,
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
														},
													},
												},
												{
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
														},
													},
												},
												{
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
								{GroupByValueIndexes: []int32{0}, Value: 10},
								{GroupByValueIndexes: []int32{1}, Value: 20},
								{GroupByValueIndexes: []int32{2}, Value: 30},
								{GroupByValueIndexes: []int32{3}, Value: 40},
								{GroupByValueIndexes: []int32{4}, Value: 50},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_filter_one_included_one_excluded_leaf_one_excluded_agg_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_filter_one_included_one_excluded_leaf_one_excluded_agg_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			// Call order (after optimization): 1) ListMoments (resolveAggregateMoments in fetchTopicDetailsForChart),
			// 2) BatchGetMoments (ALL composite moments: leaf-1, leaf-2, agg-2-leaf-1, agg-2-leaf-2),
			// 3) BatchGetMoments (ALL child moments from all leaves).
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				// 1. fetchTopicDetailsForChart: ALL composite moments (all leaves from both aggregates).
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
							},
						),
					},
				},
				// 2. fetchTopicDetailsForChart: ALL child moments from all 4 leaves.
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2", "moment_1_keyword_excluded_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1", "moment_2_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2", "moment_2_keyword_excluded_2"),
						newKeywordMomentMatchAll("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1", []string{"moment_2_keyword_included_1a", "moment_2_keyword_included_1b"}),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2", "moment_2_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_as_groupby",
			listMomentsFirst:       true, // With optimization, ListMoments is called first in fetchTopicDetailsForChart
			enableDataAccessFilter: true,
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
								Metric: &dashboardpb.ChartMetric_ForAggregateSubcategory_{
									ForAggregateSubcategory: &dashboardpb.ChartMetric_ForAggregateSubcategory{
										AggregateSubcategories: []*dashboardpb.AggregateSubcategoryForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForAggregateSubcategory{
											StatsType: &dashboardpb.StatsTypeForAggregateSubcategory_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
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
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_as_groupby_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					// Aggregate subcategories - composite moments without moment data
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					// Child subcategories - have ParentMomentTemplateId pointing to their aggregate parent
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2", "moment_1_keyword_excluded_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1", "moment_2_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2", "moment_2_keyword_excluded_2"),
						newKeywordMomentMatchAll("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1", []string{"moment_2_keyword_included_1a", "moment_2_keyword_included_1b"}),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2", "moment_2_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2"),
						// Aggregate subcategories returned in batch - composite moments without moment data
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_as_inner_groupby_with_outer_provided_value_sort",
			listMomentsFirst:       true, // With optimization, ListMoments is called first in fetchTopicDetailsForChart
			enableDataAccessFilter: true,
			expectListUsers:        true, // byProvidedValue with UserNameValue triggers excludeDeactivatedUsersFromProvidedUserGroupBys
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
								Metric: &dashboardpb.ChartMetric_ForAggregateSubcategory_{
									ForAggregateSubcategory: &dashboardpb.ChartMetric_ForAggregateSubcategory{
										AggregateSubcategories: []*dashboardpb.AggregateSubcategoryForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForAggregateSubcategory{
											StatsType: &dashboardpb.StatsTypeForAggregateSubcategory_CommonStatsType{
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
															Timestamp: timestamppb.New(time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)),
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
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: userpb.UserName{
																	CustomerID: r.customerID,
																	UserID:     "agent-id-1",
																}.String(),
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: userpb.UserName{
																	CustomerID: r.customerID,
																	UserID:     "agent-id-2",
																}.String(),
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: userpb.UserName{
																	CustomerID: r.customerID,
																	UserID:     "agent-id-3",
																}.String(),
															},
														},
														{
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
									},
								},
								Config: &dashboardpb.ChartGroupBy_Config{
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 2,
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
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
			// With the INSI-4358 fix (metricDerivedInnerGroupBy), the outer groupby is
			// ranked by the SUM of inner subcategory buckets (50, 30, 20, 10) and truncated
			// to valueCountLimit=2, yielding [agent-id-3, agent-id-2].
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
														UserID:     "agent-id-2",
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
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
													},
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{GroupByValueIndexes: []int32{0, 0}, Value: 25},
								{GroupByValueIndexes: []int32{0, 1}, Value: 25},
								{GroupByValueIndexes: []int32{1, 0}, Value: 15},
								{GroupByValueIndexes: []int32{1, 1}, Value: 15},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_as_inner_groupby_outer_sort_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_as_inner_groupby_outer_sort_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2", "moment_1_keyword_excluded_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1", "moment_2_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2", "moment_2_keyword_excluded_2"),
						newKeywordMomentMatchAll("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1", []string{"moment_2_keyword_included_1a", "moment_2_keyword_included_1b"}),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2", "moment_2_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_groupby_one_included_leaf_one_excluded_leaf_one_excluded_aggregate",
			listMomentsFirst:       true, // With optimization, ListMoments is called first in fetchTopicDetailsForChart
			enableDataAccessFilter: true,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_INCLUDE,
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
																},
															},
														},
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
																},
															},
														},
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
															Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
																AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
								{GroupByValueIndexes: []int32{0, 0}, Value: 10},
								{GroupByValueIndexes: []int32{0, 1}, Value: 20},
								{GroupByValueIndexes: []int32{0, 2}, Value: 30},
								{GroupByValueIndexes: []int32{0, 3}, Value: 40},
								{GroupByValueIndexes: []int32{0, 4}, Value: 50},
								{GroupByValueIndexes: []int32{1, 0}, Value: 60},
								{GroupByValueIndexes: []int32{1, 1}, Value: 70},
								{GroupByValueIndexes: []int32{1, 2}, Value: 80},
								{GroupByValueIndexes: []int32{1, 3}, Value: 90},
								{GroupByValueIndexes: []int32{1, 4}, Value: 100},
								{GroupByValueIndexes: []int32{2, 0}, Value: 110},
								{GroupByValueIndexes: []int32{2, 1}, Value: 120},
								{GroupByValueIndexes: []int32{2, 2}, Value: 130},
								{GroupByValueIndexes: []int32{2, 3}, Value: 140},
								{GroupByValueIndexes: []int32{2, 4}, Value: 150},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_groupby_one_included_one_excluded_leaf_one_excluded_agg_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_groupby_one_included_one_excluded_leaf_one_excluded_agg_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				// 1. Prefetch: composite moments for groupBy leaf subcategories (leaf-1, leaf-2).
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
							},
						),
					},
				},
				// 2. Prefetch: child moments for leaf-1 and leaf-2.
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2", "moment_1_keyword_excluded_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
				// 3. convertAggregateSubcategory(excluded agg-2): composite moments for agg-2 leaves.
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
							},
						),
					},
				},
				// 4. Child moments for agg-2 leaves.
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1", "moment_2_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2", "moment_2_keyword_excluded_2"),
						newKeywordMomentMatchAll("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1", []string{"moment_2_keyword_included_1a", "moment_2_keyword_included_1b"}),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2", "moment_2_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_as_filter_one_excluded",
			listMomentsFirst:       true, // With optimization, ListMoments is called first in fetchTopicDetailsForChart
			enableDataAccessFilter: true,
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
													Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_as_filter_one_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_as_filter_one_excluded_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_as_groupby_one_excluded",
			enableDataAccessFilter: true,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Operator: dashboardpb.ChartAttributeValue_OPERATOR_EXCLUDE,
															Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
																AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
								{GroupByValueIndexes: []int32{0, 0}, Value: 10},
								{GroupByValueIndexes: []int32{0, 1}, Value: 20},
								{GroupByValueIndexes: []int32{0, 2}, Value: 30},
								{GroupByValueIndexes: []int32{0, 3}, Value: 40},
								{GroupByValueIndexes: []int32{0, 4}, Value: 50},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_as_groupby_one_excluded_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_as_groupby_one_excluded_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_without_children_as_filter",
			listMomentsFirst:       true, // With optimization, tests empty aggregateExpansions handling
			enableDataAccessFilter: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2"},
				ManagedGroupIDs: []string{"4", "5"},
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
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-empty",
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
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               0,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_without_children_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_without_children_as_filter_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					// Aggregate subcategory without children - composite moment without moment data
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-empty"),
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{},
		},
		{
			desc:                   "mixed_subcategory_and_aggregate_subcategory_as_filter",
			listMomentsFirst:       true,
			enableDataAccessFilter: true,
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
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
														},
													},
												},
												{
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_mixed_subcategory_and_aggregate_subcategory_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_mixed_subcategory_and_aggregate_subcategory_as_filter_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					// Regular subcategory - composite moment without full moment data in ListMoments
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
					},
					// Aggregate subcategory - composite moment without moment data
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					// Child of aggregate subcategory
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				// First BatchGetMoments: Fetch regular subcategory-1
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/subcategory-1",
							"",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/keyword-1"},
						),
					},
				},
				// Second BatchGetMoments: Fetch base moments for regular subcategory (keyword-1)
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-1", "keyword1"),
					},
				},
				// Third BatchGetMoments: Fetch aggregate subcategory leaf nodes
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/keyword-2"},
						),
					},
				},
				// Fourth BatchGetMoments: Fetch base moments for aggregate leaf (keyword-2) and aggregate itself
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-2", "keyword2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "mixed_subcategory_and_aggregate_subcategory_as_groupby",
			listMomentsFirst:       true,
			enableDataAccessFilter: true,
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
																AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               5,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               7,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               12,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               15,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               20,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_mixed_subcategory_and_aggregate_subcategory_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_mixed_subcategory_and_aggregate_subcategory_as_groupby_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					// Regular subcategory - composite moment without full moment data in ListMoments
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
					},
					// Second regular subcategory to test multiple leaf subcategories
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
					},
					// Aggregate subcategory - composite moment without moment data
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					// Child of aggregate subcategory
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				// First BatchGetMoments: Fetch regular subcategory-1 and subcategory-2
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/subcategory-1",
							"",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/keyword-1"},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/subcategory-2",
							"",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/keyword-3"},
						),
					},
				},
				// Second BatchGetMoments: Fetch base moments for regular subcategories (keyword-1, keyword-3)
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-1", "keyword1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-3", "keyword3"),
					},
				},
				// Third BatchGetMoments: Fetch aggregate subcategory leaf nodes
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/keyword-2"},
						),
					},
				},
				// Fourth BatchGetMoments: Fetch base moments for aggregate leaf (keyword-2) and aggregate itself
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-2", "keyword2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "mixed_aggregate_and_leaf_in_metric_as_groupby",
			listMomentsFirst:       true,
			enableDataAccessFilter: true,
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
								Metric: &dashboardpb.ChartMetric_ForAggregateSubcategory_{
									ForAggregateSubcategory: &dashboardpb.ChartMetric_ForAggregateSubcategory{
										AggregateSubcategories: []*dashboardpb.AggregateSubcategoryForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/leaf-subcategory-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
											},
										},
										StatsType: &dashboardpb.StatsTypeForAggregateSubcategory{
											StatsType: &dashboardpb.StatsTypeForAggregateSubcategory_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
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
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/leaf-subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
									GroupByValueIndexes: []int32{1, 0},
									Value:               15,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               25,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_mixed_aggregate_and_leaf_in_metric_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_mixed_aggregate_and_leaf_in_metric_as_groupby_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					// Leaf subcategory - composite moment WITH moment data (classified as leaf by splitAggregateMoments)
					newSubcategoryMoment(
						"customers/customer_id/profiles/profile_id/moments/leaf-subcategory-1",
						"",
						[]string{},
						[]string{"customers/customer_id/profiles/profile_id/moments/leaf_keyword_1"},
					),
					// Aggregate subcategory - composite moment WITHOUT moment data
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					// Child of aggregate subcategory
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				// 1. convertSubcategories for leaf-subcategory-1: fetch composite moment
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/leaf-subcategory-1",
							"",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/leaf_keyword_1"},
						),
					},
				},
				// 2. convertSubcategories for leaf-subcategory-1: fetch child keyword moment
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/leaf_keyword_1", "leaf_keyword_1"),
					},
				},
				// 3. convertAggregateSubcategory for aggregate-subcategory-1: fetch hierarchy leaf composite moment
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{},
							[]string{"customers/customer_id/profiles/profile_id/moments/agg_keyword_1"},
						),
					},
				},
				// 4. convertAggregateSubcategory for aggregate-subcategory-1: fetch child keyword moment + aggregate parent
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/agg_keyword_1", "agg_keyword_1"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
		{
			desc:                   "aggregate_subcategory_avg_occurrence_count_per_convo_as_groupby",
			listMomentsFirst:       true,
			enableDataAccessFilter: true,
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
								Metric: &dashboardpb.ChartMetric_ForAggregateSubcategory_{
									ForAggregateSubcategory: &dashboardpb.ChartMetric_ForAggregateSubcategory{
										AggregateSubcategories: []*dashboardpb.AggregateSubcategoryForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
											},
										},
										StatsType: &dashboardpb.StatsTypeForAggregateSubcategory{
											StatsType: &dashboardpb.StatsTypeForAggregateSubcategory_AggregationOnOccurrenceCountPerConversation{
												AggregationOnOccurrenceCountPerConversation: &dashboardpb.AggregationOnOccurrenceCountPerConversation{
													AggregationOperator: dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_AVG,
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
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
													AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_avg_occurrence_count_per_convo_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_avg_occurrence_count_per_convo_as_groupby_es_response.json",
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
							},
						),
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2-leaf-2",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
							[]string{"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2"},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
								"customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_1", "moment_1_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded_2", "moment_1_keyword_excluded_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1", "moment_1_keyword_included_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_1", "moment_2_keyword_excluded_1"),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded_2", "moment_2_keyword_excluded_2"),
						newKeywordMomentMatchAll("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1", []string{"moment_2_keyword_included_1a", "moment_2_keyword_included_1b"}),
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2", "moment_2_keyword_included_2"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Call order when prefetch runs first: BatchGetMoments x2, ListMoments, BatchGetMoments x2 (only for cases with 4 batch responses and not batchGetMomentsBeforeListMoments).
			prefetchFirstOrder := len(tc.batchGetMomentResps) == 4 && tc.listMomentsResp != nil && !tc.batchGetMomentsBeforeListMoments && !tc.listMomentsFirst
			// Merged subcategory+aggregate groupBy: prefetch does 2 BatchGetMoments (composite then child moments), then ListMoments, then 2 BatchGetMoments for aggregate leaves. Requires 4 batch responses.
			batchFirstOrder := tc.batchGetMomentsBeforeListMoments && tc.listMomentsResp != nil && len(tc.batchGetMomentResps) >= 4
			switch {
			case prefetchFirstOrder:
				gomock.InOrder(
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(tc.batchGetMomentResps[0], nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(tc.batchGetMomentResps[1], nil),
					r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(tc.batchGetMomentResps[2], nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(tc.batchGetMomentResps[3], nil),
				)
			case batchFirstOrder:
				// Prefetch: BatchGetMoments(composite), BatchGetMoments(child moments); then ListMoments; then convertAggregateSubcategory: BatchGetMoments(agg leaves), BatchGetMoments(agg child moments).
				calls := make([]*gomock.Call, 0, 2+1+len(tc.batchGetMomentResps)-2)
				calls = append(calls, r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(tc.batchGetMomentResps[0], nil))
				calls = append(calls, r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(tc.batchGetMomentResps[1], nil))
				calls = append(calls, r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil))
				for _, resp := range tc.batchGetMomentResps[2:] {
					calls = append(calls, r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(resp, nil))
				}
				gomock.InOrder(calls...)
			default:
				if tc.listMomentsResp != nil {
					r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil)
				}
				if len(tc.batchGetMomentResps) > 0 {
					calls := make([]*gomock.Call, 0, len(tc.batchGetMomentResps))
					for _, resp := range tc.batchGetMomentResps {
						call := r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(resp, nil)
						calls = append(calls, call)
					}
					gomock.InOrder(calls...)
				}
			}
			if tc.expectListUsers {
				r.mockUserClient.EXPECT().
					ListUsers(gomock.Any(), gomock.Any()).
					AnyTimes().
					DoAndReturn(func(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.ListUsersResponse, error) {
						users := make([]*userpb.User, 0, len(req.GetFilter().GetUserIds()))
						for _, userID := range req.GetFilter().GetUserIds() {
							users = append(users, &userpb.User{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     userID,
								}.String(),
							})
						}
						return &userpb.ListUsersResponse{Users: users}, nil
					})
			}

			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					{
						Name: "customers/customer_id/profiles/profile_id",
					},
				},
				AuthConfig: &nonpubconfig.CustomerAuthConfig{
					EnableAdvancedDataAccessControl: tc.enableDataAccessFilter,
				},
			}
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(testCustomerConfig.ListProfiles()[0], nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			if tc.enableDataAccessFilter && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}

			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))

			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertCommonResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_SubcategorySimplified(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	envutil.SetEnv("ENABLE_SIMPLIFIED_SUBCATEGORY_GROUP_BY_IN_EVALUATE_ANALYTICS_CHART", "true")
	r.SetupSuite()

	newKeywordMoment := func(name, phrase string) *momentpb.Moment {
		return &momentpb.Moment{
			Name: name,
			Type: momentpb.Moment_KEYWORD,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: phrase}},
							},
						},
					},
				},
			},
		}
	}
	newKeywordMomentMatchAll := func(name string, phrases []string) *momentpb.Moment {
		operands := []*momentpb.KeywordMomentData_Operand{}
		for _, phrase := range phrases {
			operands = append(operands, &momentpb.KeywordMomentData_Operand{
				Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: phrase},
			})
		}
		return &momentpb.Moment{
			Name: name,
			Type: momentpb.Moment_KEYWORD,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_AND,
							Operands: operands,
						},
					},
				},
			},
		}
	}
	newClusterMoment := func(name string) *momentpb.Moment {
		return &momentpb.Moment{Name: name, Type: momentpb.Moment_TOPIC}
	}
	newAggregateSubcategoryMoment := func(name string) *momentpb.Moment {
		return &momentpb.Moment{Name: name, Type: momentpb.Moment_COMPOSITE}
	}
	newSubcategoryMoment := func(name, parent string, excluded []string, includes []string) *momentpb.Moment {
		operands := []*momentpb.CompositeMomentData_CompositeMomentOperand{}
		if len(excluded) > 0 {
			notOperands := []*momentpb.CompositeMomentData_CompositeMomentOperand{}
			for _, ex := range excluded {
				notOperands = append(notOperands, &momentpb.CompositeMomentData_CompositeMomentOperand{
					Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{BaseMoment: ex},
				})
			}
			operands = append(operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
					CompositeMoment: &momentpb.CompositeMomentData{Operator: momentpb.CompositeMomentData_NOT, Operands: notOperands},
				},
			})
		}
		includeOperands := []*momentpb.CompositeMomentData_CompositeMomentOperand{}
		for _, in := range includes {
			includeOperands = append(includeOperands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{BaseMoment: in},
			})
		}
		if len(includeOperands) > 0 {
			operands = append(operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
					CompositeMoment: &momentpb.CompositeMomentData{Operator: momentpb.CompositeMomentData_OR, Operands: includeOperands},
				},
			})
		}
		return &momentpb.Moment{
			Name: name, Type: momentpb.Moment_COMPOSITE, ParentMomentTemplateId: parent,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_CompositeMomentData{
					CompositeMomentData: &momentpb.CompositeMomentData{Operator: momentpb.CompositeMomentData_AND, Operands: operands},
				},
			},
		}
	}

	// Suppress unused warnings for helpers that may only be used in some test cases.
	_ = newKeywordMomentMatchAll
	_ = newClusterMoment

	testCases := []struct {
		desc                                                        string
		enableAdvancedDataAccessControl                             bool
		enableReadingDenormalizedConversationMetadataFromMessageDoc bool
		denormalizedConversationMetadataMomentTemplateIds           []string
		resourceACL                                                 *auth.ResourceACL
		esReqPath                                                   string
		esRespPath                                                  string
		req                                                         *analyticspb.EvaluateAnalyticsChartRequest
		resp                                                        *analyticspb.EvaluateAnalyticsChartResponse
		getCompositeMomentResp                                      *momentpb.BatchGetMomentsResponse
		getChildMomentResp                                          *momentpb.BatchGetMomentsResponse
		listMomentsResp                                             *momentpb.ListMomentsResponse
		batchGetMomentResps                                         []*momentpb.BatchGetMomentsResponse
	}{
		{
			desc:                            "subcategory_as_groupby_simplified",
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
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_simplified_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby_subcategory_has_cluster_not_simplified",
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
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_subcategory_has_cluster_not_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_subcategory_has_cluster_not_simplified_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby_aht_not_simplied",
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
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
									Value:               1,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               2,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               3,
								},
								{
									GroupByValueIndexes: []int32{0, 3},
									Value:               4,
								},
								{
									GroupByValueIndexes: []int32{0, 4},
									Value:               5,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               10,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               20,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               30,
								},
								{
									GroupByValueIndexes: []int32{1, 3},
									Value:               40,
								},
								{
									GroupByValueIndexes: []int32{1, 4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_aht_not_simplied_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_aht_not_simplied_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby_and_conversation_attributes_as_filters_not_simplified",
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
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
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
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_and_conversation_attributes_as_filters_not_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_and_conversation_attributes_as_filters_not_simplified_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby_and_denormalized_metadata_as_filters_simplified",
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
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
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
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_and_denormalized_metadata_as_filters_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_and_denormalized_metadata_as_filters_simplified_es_response.json",
		},
		{
			desc:                            "subcategory_as_filter_simplified",
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
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_filter_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_filter_simplified_es_response.json",
		},
		{
			desc:                            "subcategory_as_groupby_and_sentiment_as_filter_not_simplified",
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
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
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
						GroupBy: []*dashboardpb.ChartGroupBy{
							{
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
																SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
													SubcategoryValue: &dashboardpb.SubcategoryForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
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
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_2_keyword_included_1",
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_groupby_and_sentiment_as_filter_not_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_groupby_and_sentiment_as_filter_not_simplified_es_response.json",
		},
		{
			desc:                            "subcategory_as_filter_and_sentiment_as_filter_not_simplified",
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
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
													Value: &dashboardpb.ChartAttributeValue_SentimentValue{
														SentimentValue: &dashboardpb.SentimentForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-1",
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
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
			},
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_excluded_2",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment_1_keyword_included_2",
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
			esReqPath:  "EvaluateAnalyticsChart_subcategory_as_filter_and_sentiment_as_filter_not_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_subcategory_as_filter_and_sentiment_as_filter_not_simplified_es_response.json",
		},
		{
			// Test dedup: metric has 1 subcategory (→ filter), same subcategory already in existing filter.
			// After dedup the metric-derived filter is removed; the ES request is the same as
			// subcategory_as_filter_simplified (ForConversation metric with same filter).
			// Uses identical moment data and produces identical ES request to subcategory_as_filter_simplified.
			desc:                            "same_subcategory_in_metric_and_filter_deduped",
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
								Metric: &dashboardpb.ChartMetric_ForSubcategory_{
									ForSubcategory: &dashboardpb.ChartMetric_ForSubcategory{
										Subcategories: []*dashboardpb.SubcategoryForChart{
											{MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1"},
										},
										StatsType: &dashboardpb.StatsTypeForSubcategory{
											StatsType: &dashboardpb.StatsTypeForSubcategory_CommonStatsType{
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
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
			// Same moment data and expected response as subcategory_as_filter_simplified.
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1"}},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2"}},
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
			getChildMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					newKeywordMoment("customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1", "moment_1_keyword_excluded_1"),
					newKeywordMoment("customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2", "moment_1_keyword_included_2"),
				},
			},
			resp:       nil, // Response validation skipped; this test verifies dedup produces correct ES request.
			esReqPath:  "EvaluateAnalyticsChart_same_subcategory_in_metric_and_filter_deduped_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_same_subcategory_in_metric_and_filter_deduped_es_response.json",
		},
		{
			// Test dedup: metric has 1 aggregate subcategory (→ filter), same aggregate already in existing filter.
			// After dedup the metric-derived filter is removed; the ES request is the same as
			// aggregate_subcategory_as_filter (ForConversation metric with same filter).
			desc:                            "same_aggregate_subcategory_in_metric_and_filter_deduped",
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
								Metric: &dashboardpb.ChartMetric_ForAggregateSubcategory_{
									ForAggregateSubcategory: &dashboardpb.ChartMetric_ForAggregateSubcategory{
										AggregateSubcategories: []*dashboardpb.AggregateSubcategoryForChart{
											{MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"},
										},
										StatsType: &dashboardpb.StatsTypeForAggregateSubcategory{
											StatsType: &dashboardpb.StatsTypeForAggregateSubcategory_CommonStatsType{
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
													Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
														AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
			resp: nil, // Response validation skipped; this test verifies dedup produces correct ES request.
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1", "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1", []string{}, []string{"customers/customer_id/profiles/profile_id/moments/keyword-1"}),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-1", "keyword1"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_same_aggregate_subcategory_in_metric_and_filter_deduped_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_same_aggregate_subcategory_in_metric_and_filter_deduped_es_response.json",
		},
		{
			// Test that query is NOT simplified when an aggregate subcategory's leaf node has a cluster.
			// Even though all other simplification criteria are met, the presence of a cluster prevents optimization.
			// This tests that aggregate expansion happens BEFORE simplification logic is assessed.
			desc:                            "aggregate_subcategory_with_cluster_in_leaf_not_simplified",
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
															Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
																AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
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
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						newSubcategoryMoment(
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1-leaf-1",
							"customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
							[]string{},
							[]string{
								"customers/customer_id/profiles/profile_id/moments/keyword-1",
								"customers/customer_id/profiles/profile_id/moments/cluster-1",
							},
						),
					},
				},
				{
					Moments: []*momentpb.Moment{
						newKeywordMoment("customers/customer_id/profiles/profile_id/moments/keyword-1", "keyword1"),
						newClusterMoment("customers/customer_id/profiles/profile_id/moments/cluster-1"),
						newAggregateSubcategoryMoment("customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1"),
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_aggregate_subcategory_with_cluster_in_leaf_not_simplified_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_aggregate_subcategory_with_cluster_in_leaf_not_simplified_es_response.json",
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)

			// Create fresh suite for this subtest to ensure proper mock isolation
			subR := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
			subR.SetupSuite()

			if tc.listMomentsResp != nil {
				subR.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil)
			}
			switch {
			case len(tc.batchGetMomentResps) > 0:
				calls := make([]*gomock.Call, 0, len(tc.batchGetMomentResps))
				for _, resp := range tc.batchGetMomentResps {
					call := subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Return(resp, nil)
					calls = append(calls, call)
				}
				gomock.InOrder(calls...)
			case tc.getCompositeMomentResp != nil && tc.getChildMomentResp != nil:
				gomock.InOrder(
					subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil),
					subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getChildMomentResp, nil))
			case tc.getCompositeMomentResp != nil:
				subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil)
			}
			profileConfig := &nonpubconfig.ProfileConfig{
				Name: "customers/customer_id/profiles/profile_id",
			}
			if tc.enableReadingDenormalizedConversationMetadataFromMessageDoc || len(tc.denormalizedConversationMetadataMomentTemplateIds) > 0 {
				profileConfig.Features = &v1config.Features{
					Conversation: &v1featuresconfig.ConversationFeatures{
						EnableReadingDenormalizedConversationMetadataFromMessageDoc: tc.enableReadingDenormalizedConversationMetadataFromMessageDoc,
						DenormalizedConversationMetadataMomentTemplateIds:           tc.denormalizedConversationMetadataMomentTemplateIds,
					},
				}
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

			subR.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
				Return(testCustomerConfig.ListProfiles()[0], nil)
			subR.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				subR.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}

			subR.mockESServer.Resps = append(subR.mockESServer.Resps, subR.getFile(tc.esRespPath))
			subR.mockESServer.EXPECT().Search(subR.IndexName, subR.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			resp, err := subR.Client.EvaluateAnalyticsChart(subR.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				assertCommonResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_SortGroupby(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	for _, tc := range []struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		batchGetMomentResp              *momentpb.BatchGetMomentsResponse
		metricMomentResp                *momentpb.BatchGetMomentsResponse
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		resp                            *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "groupby_unique_value_metrics_avg_convo_duration_groupby_convo_agent",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-5",
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
									Value:               9,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               8,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               7,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               6,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               5,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_avg_convo_duration_groupby_convo_agent_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_avg_convo_duration_groupby_convo_agent_es_response.json",
		},
		{
			desc: "groupby_unique_agent_metrics_metadata_value",
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
									Value:               9,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               8,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               7,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_agent_metrics_metadata_value_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_agent_metrics_metadata_value_es_response.json",
		},
		{
			desc: "groupby_unique_agent_and_time_metrics_metadata_value",
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
															Timestamp: timestamppb.New(time.Date(2024, 8, 15, 0, 0, 0, 0, time.UTC)),
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
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               12,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_agent_and_time_metrics_metadata_value_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_agent_and_time_metrics_metadata_value_es_response.json",
		},
		{
			desc:                            "groupby_unique_agent_metrics_outcome_value",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
															Timestamp: timestamppb.New(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)),
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-5",
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
									Value:               9,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               8,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               7,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               6,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               5,
								},
							},
						},
					},
				},
			},
			metricMomentResp: binlessMetadataMomentResp(),
			esReqPath:        "EvaluateAnalyticsChart_groupby_unique_agent_metrics_outcome_value_es_request.json",
			esRespPath:       "EvaluateAnalyticsChart_groupby_unique_agent_metrics_outcome_value_es_response.json",
		},
		{
			desc:                            "groupby_unique_value_metrics_convo_count_groupby_convo_agent",
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
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_UserNameValue{
													UserNameValue: userpb.UserName{
														CustomerID: r.customerID,
														UserID:     "agent-id-5",
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
									Value:               7,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               2,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               1,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               1,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               3,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_convo_agent_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_convo_agent_es_response.json",
		},
		{
			desc:                            "groupby_unique_value_metrics_convo_count_groupby_convo_agent_and_time",
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
															Timestamp: timestamppb.New(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)),
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
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               31,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_convo_agent_and_time_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_convo_agent_and_time_es_response.json",
		},
		{
			desc:                            "groupby_unique_value_metrics_convo_count_groupby_metadata1",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			batchGetMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
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
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
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
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value1",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value2",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value3",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value4",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value5",
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
									Value:               9,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               8,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               7,
								},
								{
									GroupByValueIndexes: []int32{3},
									Value:               6,
								},
								{
									GroupByValueIndexes: []int32{4},
									Value:               5,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_metadata_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_metadata_es_response.json",
		},
		{
			desc:                            "groupby_unique_value_metrics_convo_count_groupby_metadata_empty_buckets",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			batchGetMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
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
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
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
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{},
							AnalyticsNumbers:        []*dashboardpb.AnalyticsResult_AnalyticsNumber{},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_metadata_empty_buckets_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_metadata_empty_buckets_es_response.json",
		},
		{
			desc:                            "groupby_unique_value_metrics_convo_count_groupby_metadata_and_time",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			batchGetMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
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
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
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
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value1",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value2",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "metadata-value3",
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
													Timestamp: timestamppb.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
												},
											},
										},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0, 0},
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               12,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               13,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               23,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{2, 2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_metadata_and_time_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_convo_count_groupby_metadata_and_time_es_response.json",
		},
		{
			desc:                            "groupby_unique_value_metrics_occurrence_count_groupby_agent_and_time",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4", "agent-id-5"},
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
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-id1",
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
															Timestamp: timestamppb.New(time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)),
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
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               12,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               13,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               23,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{2, 2},
									Value:               33,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_unique_value_metrics_occurrence_count_groupby_agent_and_time_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_unique_value_metrics_occurrence_count_groupby_agent_and_time_es_response.json",
		},
		{
			desc:                            "sort_groupby_time_metrics_occurrence_count_of_single_entity",
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
								Metric: &dashboardpb.ChartMetric_ForEmotion_{
									ForEmotion: &dashboardpb.ChartMetric_ForEmotion{
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-id1",
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
															Timestamp: timestamppb.New(time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)),
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
														{
															Value: &dashboardpb.ChartAttributeValue_SentimentValue{
																SentimentValue: &dashboardpb.SentimentForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-3",
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
								Config: &dashboardpb.ChartGroupBy_Config{
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 2,
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
												Value: &dashboardpb.ChartAttributeValue_SentimentValue{
													SentimentValue: &dashboardpb.SentimentForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/sentiment-3",
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
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               33,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               23,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_sort_groupby_time_metrics_occurrence_count_of_single_entity_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_sort_groupby_time_metrics_occurrence_count_of_single_entity_es_response.json",
		},
		{
			desc:                            "groupby_provided_value_metrics_convo_duration_groupby_convo_agent_and_time",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4", "agent-id-5"},
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
								Config: &dashboardpb.ChartGroupBy_Config{
									ValueCountLimit: 3,
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
								},
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-1",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-2",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-3",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-4",
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
														UserID:     "agent-id-2",
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
									Value:               11,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               12,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               22,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_provided_value_metrics_convo_duration_groupby_convo_agent_and_time_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_provided_value_metrics_convo_duration_groupby_convo_agent_and_time_es_response.json",
		},
		{
			desc:                            "groupby_provided_value_metrics_convo_duration_groupby_convo_agent_only",
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
								Config: &dashboardpb.ChartGroupBy_Config{
									ValueCountLimit: 3,
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
								},
								GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
									ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
										ValueSet: &dashboardpb.ChartValueSet{
											Values: &dashboardpb.ChartValueSet_SingleValues{
												SingleValues: &dashboardpb.ChartValueSet_Single{
													AttributeValues: []*dashboardpb.ChartAttributeValue{
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-1",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-2",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-3",
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_UserNameValue{
																UserNameValue: "customers/customer_id/users/agent-id-4",
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
														UserID:     "agent-id-2",
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
									Value:               4,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               3,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               2,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_provided_value_metrics_convo_duration_groupby_convo_agent_only_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_provided_value_metrics_convo_duration_groupby_convo_agent_only_es_response.json",
		},
		{
			desc:                            "groupby_provided_value_groupby_from_metrics",
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
								Metric: &dashboardpb.ChartMetric_ForEmotion_{
									ForEmotion: &dashboardpb.ChartMetric_ForEmotion{
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-4",
											},
										},
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
									},
								},
								GroupbyConfig: &dashboardpb.ChartGroupBy_Config{
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
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
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{
									GroupByValueIndexes: []int32{0},
									Value:               4,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               3,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               2,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_provided_value_groupby_from_metrics_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_provided_value_groupby_from_metrics_es_response.json",
		},
		{
			desc:                            "groupby_provided_value_metrics_occurrence_count_groupby_from_metrics_only",
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
								Metric: &dashboardpb.ChartMetric_ForEmotion_{
									ForEmotion: &dashboardpb.ChartMetric_ForEmotion{
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-4",
											},
										},
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
											},
										},
									},
								},
								GroupbyConfig: &dashboardpb.ChartGroupBy_Config{
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_EmotionValue{
													EmotionValue: &dashboardpb.EmotionForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-4",
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
									Value:               170,
								},
								{
									GroupByValueIndexes: []int32{1},
									Value:               130,
								},
								{
									GroupByValueIndexes: []int32{2},
									Value:               90,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_provided_value_metrics_occurrence_count_groupby_from_metrics_only_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_provided_value_metrics_occurrence_count_groupby_from_metrics_only_es_response.json",
		},
		{
			// Maybe add MetricsConversationMessageCount on sorting layer.
			desc:                            "groupby_provided_value_metrics_occurrence_count_groupby_from_metrics_and_groupby_time",
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
								Metric: &dashboardpb.ChartMetric_ForEmotion_{
									ForEmotion: &dashboardpb.ChartMetric_ForEmotion{
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-4",
											},
										},
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
											},
										},
									},
								},
								GroupbyConfig: &dashboardpb.ChartGroupBy_Config{
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
													},
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_EmotionValue{
													EmotionValue: &dashboardpb.EmotionForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-4",
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
									Value:               41,
								},
								{
									GroupByValueIndexes: []int32{0, 1},
									Value:               42,
								},
								{
									GroupByValueIndexes: []int32{0, 2},
									Value:               43,
								},
								{
									GroupByValueIndexes: []int32{1, 0},
									Value:               31,
								},
								{
									GroupByValueIndexes: []int32{1, 1},
									Value:               32,
								},
								{
									GroupByValueIndexes: []int32{1, 2},
									Value:               33,
								},
								{
									GroupByValueIndexes: []int32{2, 0},
									Value:               21,
								},
								{
									GroupByValueIndexes: []int32{2, 1},
									Value:               22,
								},
								{
									GroupByValueIndexes: []int32{2, 2},
									Value:               23,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_groupby_provided_value_metrics_occurrence_count_groupby_from_metrics_and_groupby_time_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_groupby_provided_value_metrics_occurrence_count_groupby_from_metrics_and_groupby_time_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			if tc.batchGetMomentResp != nil {
				r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(2).Return(tc.batchGetMomentResp, nil)
			}
			if tc.metricMomentResp != nil {
				r.mockMomentClient.EXPECT().GetMoment(gomock.Any(), gomock.Any()).Times(1).Return(&momentpb.GetMomentResponse{Moment: tc.metricMomentResp.Moments[0]}, nil)
			}
			r.mockUserClient.EXPECT().
				ListUsers(gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.ListUsersResponse, error) {
					users := make([]*userpb.User, 0, len(req.GetFilter().GetUserIds()))
					for _, userID := range req.GetFilter().GetUserIds() {
						users = append(users, &userpb.User{
							Name: userpb.UserName{
								CustomerID: r.customerID,
								UserID:     userID,
							}.String(),
						})
					}
					return &userpb.ListUsersResponse{Users: users}, nil
				})
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

			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))

			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}

			if tc.resp != nil {
				assertCommonResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

// To cover basic test cases that does not belong to any Cresta entity.
func TestEvaluateAnalyticsChart_Basic(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
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
			desc:                            "start_of_week_default",
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
			esReqPath:  "EvaluateAnalyticsChart_basic_start_of_week_default_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_default_es_response.json",
		},
		{
			desc:                            "start_of_week_is_monday",
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
				Metadata: &analyticspb.Metadata{
					StartDayOfWeek: analyticspb.Metadata_MONDAY,
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
			esReqPath:  "EvaluateAnalyticsChart_basic_start_of_week_is_monday_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_start_of_week_is_monday_es_response.json",
		},
		{
			desc:                            "convo_duration_as_filter",
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
													Value: &dashboardpb.ChartAttributeValue_ConversationDurationRange{
														ConversationDurationRange: &dashboardpb.ChartDurationRange{
															LowerBound: &dashboardpb.ChartDurationRange_Bound{
																Value:       &durationpb.Duration{Seconds: 123},
																IsExclusive: true,
															},
															UpperBound: &dashboardpb.ChartDurationRange_Bound{
																Value: &durationpb.Duration{Seconds: 456},
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_ScorecardStatusFilter{
														ScorecardStatusFilter: &dashboardpb.ScorecardStatusFilter{
															ScorecardStatuses: []dashboardpb.ScorecardStatus{dashboardpb.ScorecardStatus_SCORECARD_STATUS_UNSPECIFIED},
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
			esReqPath:  "EvaluateAnalyticsChart_basic_convo_duration_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_convo_duration_as_filter_es_response.json",
		},
		{
			desc:                            "advanced_time_filter",
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
													Value: &dashboardpb.ChartAttributeValue_TimeRangeFilter{
														TimeRangeFilter: &commonpb.TimeRangeFilter{
															OuterTimeRange: &commonpb.TimeRange{
																Value: &commonpb.TimeRange_AbsoluteTimeRange_{
																	AbsoluteTimeRange: &commonpb.TimeRange_AbsoluteTimeRange{
																		StartTime: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
																		EndTime:   timestamppb.New(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
																	},
																},
															},
															Rule: &commonpb.TimeRangeRule{
																DaysOfWeek: []dayofweek.DayOfWeek{
																	dayofweek.DayOfWeek_MONDAY,
																	dayofweek.DayOfWeek_TUESDAY,
																	dayofweek.DayOfWeek_WEDNESDAY,
																},
																TimeOfDayRanges: []*commonpb.TimeOfDayRange{
																	{
																		StartTimeOfDay: &timeofday.TimeOfDay{Hours: 9, Minutes: 0},
																		EndTimeOfDay:   &timeofday.TimeOfDay{Hours: 12, Minutes: 0},
																	},
																	{
																		StartTimeOfDay: &timeofday.TimeOfDay{Hours: 12, Minutes: 0},
																		EndTimeOfDay:   &timeofday.TimeOfDay{Hours: 17, Minutes: 0},
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
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_basic_advanced_date_filter_merged_time_ranges_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_advanced_date_filter_merged_time_ranges_es_response.json",
		},
		{
			desc:                            "advanced_time_filter_daily_with_dow_filter",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "America/Toronto",
				},
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
													Value: &dashboardpb.ChartAttributeValue_TimeRangeFilter{
														TimeRangeFilter: &commonpb.TimeRangeFilter{
															OuterTimeRange: &commonpb.TimeRange{
																Value: &commonpb.TimeRange_AbsoluteTimeRange_{
																	AbsoluteTimeRange: &commonpb.TimeRange_AbsoluteTimeRange{
																		StartTime: timestamppb.New(time.Date(2024, 7, 1, 4, 0, 0, 0, time.UTC)),
																		EndTime:   timestamppb.New(time.Date(2024, 7, 8, 4, 0, 0, 0, time.UTC)),
																	},
																},
															},
															Rule: &commonpb.TimeRangeRule{
																DaysOfWeek: []dayofweek.DayOfWeek{
																	dayofweek.DayOfWeek_MONDAY,
																	dayofweek.DayOfWeek_TUESDAY,
																	dayofweek.DayOfWeek_WEDNESDAY,
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
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 1, 4, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 2, 4, 0, 0, 0, time.UTC)),
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue{
												Value: &dashboardpb.ChartAttributeValue_Timestamp{
													Timestamp: timestamppb.New(time.Date(2024, 7, 3, 4, 0, 0, 0, time.UTC)),
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
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_basic_advanced_date_filter_daily_with_dow_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_basic_advanced_date_filter_daily_with_dow_filter_es_response.json",
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
				assertCommonResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

// binlessMetadataMomentResp is the BatchGetMoments response for a chart metric on a metadata
// moment that declares no numeric bins: no value bounds are declared, so the metric
// aggregation stays unfiltered. INSI-4673.
func binlessMetadataMomentResp() *momentpb.BatchGetMomentsResponse {
	return &momentpb.BatchGetMomentsResponse{
		Moments: []*momentpb.Moment{{
			Name: "customers/customer_id/profiles/profile_id/moments/metadata-1",
			Type: momentpb.Moment_CONVERSATION_METADATA,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_ConversationMetadataMomentData{
					ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
						Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
					},
				},
			},
		}},
	}
}

// csatNumberOutcomeMomentResp is the BatchGetMoments response for a CSAT number outcome moment
// without declared bins: the metric aggregation restricts numberValue to the predictor's own
// [0, 10] clamp, keeping its unscored -1 sentinel out of chart averages. INSI-4673.
func csatNumberOutcomeMomentResp() *momentpb.BatchGetMomentsResponse {
	return &momentpb.BatchGetMomentsResponse{
		Moments: []*momentpb.Moment{{
			Name: "customers/customer_id/profiles/profile_id/moments/metadata-1",
			Type: momentpb.Moment_CONVERSATION_OUTCOME,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
					ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
						OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER,
						OutcomeSubtype:   enumspb.OutcomeSubtype_OUTCOME_SUBTYPE_CSAT,
					},
				},
			},
		}},
	}
}

// binnedOutcomeMomentResp is the BatchGetMoments response for an outcome moment declaring
// numeric bins [2, 5] and (6, 10]: the metric aggregation restricts numberValue to the bins'
// envelope (gte 2, lte 10), keeping values outside the declared scale out of chart averages.
// INSI-4673.
func binnedOutcomeMomentResp() *momentpb.BatchGetMomentsResponse {
	return &momentpb.BatchGetMomentsResponse{
		Moments: []*momentpb.Moment{{
			Name: "customers/customer_id/profiles/profile_id/moments/metadata-1",
			Type: momentpb.Moment_CONVERSATION_OUTCOME,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
					ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
						OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER,
						NumericBins: []*metadatapb.NumericBin{
							{
								FromValue:            2,
								FromValueIsExclusive: false,
								ToValue:              5,
								ToValueIsInclusive:   true,
							},
							{
								FromValue:            6,
								FromValueIsExclusive: true,
								ToValue:              10,
								ToValueIsInclusive:   true,
							},
						},
					},
				},
			},
		}},
	}
}

// averageTalkTimeOutcomeMomentResp is the BatchGetMoments response for a non-CSAT number
// outcome moment without declared bins: no sentinel-free clamp applies, so the metric
// aggregation stays unfiltered. INSI-4673.
func averageTalkTimeOutcomeMomentResp() *momentpb.BatchGetMomentsResponse {
	return &momentpb.BatchGetMomentsResponse{
		Moments: []*momentpb.Moment{{
			Name: "customers/customer_id/profiles/profile_id/moments/metadata-1",
			Type: momentpb.Moment_CONVERSATION_OUTCOME,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
					ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
						OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER,
						OutcomeSubtype:   enumspb.OutcomeSubtype_OUTCOME_SUBTYPE_AVERAGE_TALK_TIME,
					},
				},
			},
		}},
	}
}

func TestEvaluateAnalyticsChart_Metadata(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                                                        string
		enableAdvancedDataAccessControl                             bool
		enableReadingDenormalizedConversationMetadataFromMessageDoc bool
		denormalizedConversationMetadataMomentTemplateIds           []string
		resourceACL                                                 *auth.ResourceACL
		esReqPath                                                   string
		esRespPath                                                  string
		metricMomentResp                                            *momentpb.BatchGetMomentsResponse
		req                                                         *analyticspb.EvaluateAnalyticsChartRequest
		resp                                                        *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			desc:                            "metadata_as_filter",
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
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
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
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								// Filtering by metadata with no value is equivalent to filtering by all values.
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/cresta-modelled-outcome",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
															NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
																{
																	LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value: 0.1,
																	},
																	UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value:       100,
																		IsExclusive: true,
																	},
																},
																{
																	LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value: 100.1,
																	},
																	UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value:       1000.1,
																		IsExclusive: true,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/string-metadata",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "string-value-1",
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "string-value-2",
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/boolean-metadata",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{
																		BooleanValue: true,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_as_filter_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_as_filter_es_response.json",
		},
		{
			desc:                            "metadata_as_filter_with_denormalized_metadata",
			enableAdvancedDataAccessControl: true,
			enableReadingDenormalizedConversationMetadataFromMessageDoc: true,
			denormalizedConversationMetadataMomentTemplateIds: []string{
				"numeric-metadata-with-value",
				"cresta-modelled-outcome",
				"numeric-metadata-with-bin",
				"string-metadata",
				"boolean-metadata",
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
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
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
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								// Filtering by metadata with no value is equivalent to filtering by all values.
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/cresta-modelled-outcome",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
															NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
																{
																	LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value: 0.1,
																	},
																	UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value:       100,
																		IsExclusive: true,
																	},
																},
																{
																	LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value: 100.1,
																	},
																	UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value:       1000.1,
																		IsExclusive: true,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/string-metadata",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "string-value-1",
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "string-value-2",
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/boolean-metadata",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{
																		BooleanValue: true,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_as_filter_with_denormalized_metadata_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_as_filter_with_denormalized_metadata_es_response.json",
		},
		{
			desc:                            "metadata_as_groupby",
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
															Value: &dashboardpb.ChartAttributeValue_MetadataValue{
																MetadataValue: &dashboardpb.MetadataForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-string",
																	Values: []*dashboardpb.MetadataForChart_MetadataValue{
																		{
																			Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																				StringValue: "string-value-1",
																			},
																		},
																		{
																			Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																				StringValue: "string-value-2",
																			},
																		},
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_MetadataValue{
																MetadataValue: &dashboardpb.MetadataForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-string",
																	Values: []*dashboardpb.MetadataForChart_MetadataValue{
																		{
																			Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																				StringValue: "string-value-3",
																			},
																		},
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_MetadataValue{
																MetadataValue: &dashboardpb.MetadataForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-string",
																	Values: []*dashboardpb.MetadataForChart_MetadataValue{
																		{
																			Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																				StringValue: "#NO_VALUE_METADATA#",
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
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-string",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "string-value-1",
																},
															},
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "string-value-2",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-string",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "string-value-3",
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-string",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{
																Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																	StringValue: "#NO_VALUE_METADATA#",
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_as_groupby_es_response.json",
		},
		{
			desc:                            "metadata_buckets_as_groupby",
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
															Value: &dashboardpb.ChartAttributeValue_MetadataValue{
																MetadataValue: &dashboardpb.MetadataForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
																	NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
																		{
																			LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																				Value: 0,
																			},
																			UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																				Value:       100,
																				IsExclusive: true,
																			},
																		},
																	},
																},
															},
														},
														{
															Value: &dashboardpb.ChartAttributeValue_MetadataValue{
																MetadataValue: &dashboardpb.MetadataForChart{
																	MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
																	NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
																		{
																			LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																				Value: 100,
																			},
																			UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																				Value:       1000,
																				IsExclusive: true,
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
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
														NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
															{
																LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																	Value: 0,
																},
																UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																	Value:       100,
																	IsExclusive: true,
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
														MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
														NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
															{
																LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																	Value: 100,
																},
																UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																	Value:       1000,
																	IsExclusive: true,
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_buckets_as_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_buckets_as_groupby_es_response.json",
		},
		{
			desc:                            "metadata_avg_value_group_by_time",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
								{
									GroupByValueIndexes: []int32{4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			esReqPath:  "EvaluateAnalyticsChart_metadata_avg_value_group_by_time_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_avg_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_avg_value_group_by_time",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
								{
									GroupByValueIndexes: []int32{4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			metricMomentResp: binlessMetadataMomentResp(),
			esReqPath:        "EvaluateAnalyticsChart_metadata_outcome_avg_value_group_by_time_es_request.json",
			esRespPath:       "EvaluateAnalyticsChart_metadata_outcome_avg_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_p25_value_group_by_time",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P25,
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
								{
									GroupByValueIndexes: []int32{4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			metricMomentResp: binlessMetadataMomentResp(),
			esReqPath:        "EvaluateAnalyticsChart_metadata_outcome_p25_value_group_by_time_es_request.json",
			esRespPath:       "EvaluateAnalyticsChart_metadata_outcome_p25_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_p50_value_group_by_time",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P50,
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
								{
									GroupByValueIndexes: []int32{4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			metricMomentResp: binlessMetadataMomentResp(),
			esReqPath:        "EvaluateAnalyticsChart_metadata_outcome_p50_value_group_by_time_es_request.json",
			esRespPath:       "EvaluateAnalyticsChart_metadata_outcome_p50_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_p75_value_group_by_time",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_P75,
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
								{
									GroupByValueIndexes: []int32{4},
									Value:               0,
								},
							},
						},
					},
				},
			},
			metricMomentResp: binlessMetadataMomentResp(),
			esReqPath:        "EvaluateAnalyticsChart_metadata_outcome_p75_value_group_by_time_es_request.json",
			esRespPath:       "EvaluateAnalyticsChart_metadata_outcome_p75_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_box_plot_percentiles_value_group_by_time",
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
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
													AggregationOperator:  dashboardpb.AggregationOperator_AGGREGATION_OPERATOR_BOX_PLOT_PERCENTILES,
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
								GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
									ByTime: &dashboardpb.ChartGroupBy_Time{
										Interval: dashboardpb.ChartGroupBy_Time_DAY,
									},
								},
							},
						},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BOX_PLOT,
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
									BoxPlotValues: &dashboardpb.AnalyticsResult_BoxPlotValues{
										P0:   100,
										P25:  200,
										P50:  300,
										P75:  400,
										P100: 500,
									},
								},
								{
									GroupByValueIndexes: []int32{1},
									BoxPlotValues: &dashboardpb.AnalyticsResult_BoxPlotValues{
										P0:   110,
										P25:  210,
										P50:  310,
										P75:  410,
										P100: 510,
									},
								},
								{
									GroupByValueIndexes: []int32{2},
									BoxPlotValues: &dashboardpb.AnalyticsResult_BoxPlotValues{
										P0:   120,
										P25:  220,
										P50:  320,
										P75:  420,
										P100: 520,
									},
								},
								{
									GroupByValueIndexes: []int32{3},
									BoxPlotValues: &dashboardpb.AnalyticsResult_BoxPlotValues{
										P0:   130,
										P25:  230,
										P50:  330,
										P75:  430,
										P100: 530,
									},
								},
								{
									GroupByValueIndexes: []int32{4},
									BoxPlotValues: &dashboardpb.AnalyticsResult_BoxPlotValues{
										P0:   140,
										P25:  240,
										P50:  340,
										P75:  440,
										P100: 540,
									},
								},
							},
						},
					},
				},
			},
			metricMomentResp: binlessMetadataMomentResp(),
			esReqPath:        "EvaluateAnalyticsChart_metadata_outcome_box_plot_percentiles_value_group_by_time_es_request.json",
			esRespPath:       "EvaluateAnalyticsChart_metadata_outcome_box_plot_percentiles_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_as_filter_with_missing_values",
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
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
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
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								// Filtering by metadata with no value is equivalent to filtering by all values.
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-value",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CUSTOMER_PROVIDED,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/cresta-modelled-outcome",
															OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
																Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
															},
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 1.1,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{
																		NumberValue: 2.2,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
															NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
																{
																	LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value: 0.1,
																	},
																	UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value:       100,
																		IsExclusive: true,
																	},
																},
																{
																	LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value: 100.1,
																	},
																	UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{
																		Value:       1000.1,
																		IsExclusive: true,
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/string-metadata",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "string-value-1",
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "string-value-2",
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{
																		StringValue: "#NO_VALUE_METADATA#",
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
								Operator: dashboardpb.ChartDataFilter_IN,
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_MetadataValue{
														MetadataValue: &dashboardpb.MetadataForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/boolean-metadata",
															Values: []*dashboardpb.MetadataForChart_MetadataValue{
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{
																		BooleanValue: true,
																	},
																},
																{
																	Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_as_filter_with_missing_values_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_as_filter_es_response.json",
		},
		{
			desc:                            "metadata_values_groupby_time_with_top_n",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "America/Toronto",
				},
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					DisplayName: "display_name",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
												Values: []*dashboardpb.MetadataForChart_MetadataValue{
													{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "#NO_VALUE_METADATA#"}},
												},
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
												Values: []*dashboardpb.MetadataForChart_MetadataValue{
													{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "metadata-value-1"}},
												},
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
												Values: []*dashboardpb.MetadataForChart_MetadataValue{
													{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "metadata-value-2"}},
												},
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
												Values: []*dashboardpb.MetadataForChart_MetadataValue{
													{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "metadata-value-3"}},
												},
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-moment-id",
												Values: []*dashboardpb.MetadataForChart_MetadataValue{
													{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "metadata-value-4"}},
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
									},
								},
								RenderingConfig: &dashboardpb.ChartMetric_RenderingConfig{
									Colors: []string{
										"#228BE6", "#FD7E14", "#2F9E44", "#5246E0", "#E03131",
										"#15AABF", "#FAB005", "#F06595", "#8E9FA5", "#94D82D",
									},
									ValueDisplay: dashboardpb.ChartMetric_RenderingConfig_ABSOLUTE,
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
															Timestamp: timestamppb.New(time.Date(2026, 5, 1, 4, 0, 0, 0, time.UTC)),
														},
													},
													ToValue: &dashboardpb.ChartAttributeValue{
														Value: &dashboardpb.ChartAttributeValue_Timestamp{
															Timestamp: timestamppb.New(time.Date(2026, 5, 16, 3, 59, 59, 999000000, time.UTC)),
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
														UsecaseNameValue: "customers/customer_id/profiles/profile_id/usecases/call-center-voice",
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
								Config: &dashboardpb.ChartGroupBy_Config{
									GroupbyPriority: dashboardpb.ChartGroupBy_Config_PRIMARY,
									SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
									ValueCountLimit: 3,
									GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_values_groupby_time_with_top_n_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_values_groupby_time_with_top_n_es_response.json",
		},
		{
			desc:                            "metadata_outcome_avg_value_excludes_unscored_sentinel",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			metricMomentResp: csatNumberOutcomeMomentResp(),
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_outcome_avg_value_excludes_unscored_sentinel_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_avg_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_avg_value_within_declared_bins",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			metricMomentResp: binnedOutcomeMomentResp(),
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_outcome_avg_value_within_declared_bins_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_avg_value_group_by_time_es_response.json",
		},
		{
			desc:                            "metadata_outcome_avg_value_non_csat_unfiltered",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
				ManagedGroupIDs: []string{"4", "5", "6"},
			},
			metricMomentResp: averageTalkTimeOutcomeMomentResp(),
			req: &analyticspb.EvaluateAnalyticsChartRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					Name:        "",
					DisplayName: "display_name",
					Description: "",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForMetadata_{
									ForMetadata: &dashboardpb.ChartMetric_ForMetadata{
										Metadatas: []*dashboardpb.MetadataForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/metadata-1",
												OutcomeConfig: &dashboardpb.MetadataForChart_OutcomeConfig{
													Source: dashboardpb.MetadataForChart_OutcomeConfig_CRESTA_MODELLED,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForMetadata{
											StatsType: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_{
												AggregationOnMetadataValue: &dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue{
													AggregationAttribute: dashboardpb.StatsTypeForMetadata_AggregationOnMetadataValue_NUMBER_VALUE,
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
			esReqPath:  "EvaluateAnalyticsChart_metadata_outcome_avg_value_non_csat_unfiltered_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_metadata_avg_value_group_by_time_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			if tc.metricMomentResp != nil {
				r.mockMomentClient.EXPECT().GetMoment(gomock.Any(), gomock.Any()).Times(1).Return(&momentpb.GetMomentResponse{Moment: tc.metricMomentResp.Moments[0]}, nil)
			}
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
				assertCommonResponsesEqual(t, tc.resp, resp)
			}
		})
	}
}

func TestEvaluateAnalyticsChart_MetadataAsFilter_ConversationLevelIDValuePairs(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	tc := struct {
		desc                            string
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
	}{
		desc:                            "metadata_as_filter_using_convo_level_id_value_pairs",
		enableAdvancedDataAccessControl: true,
		resourceACL: &auth.ResourceACL{
			IsRootAccess:    true,
			ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
			ManagedGroupIDs: []string{"4", "5", "6"},
		},
		req: &analyticspb.EvaluateAnalyticsChartRequest{
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
								Values: &dashboardpb.ChartValueSet_SingleValues{
									SingleValues: &dashboardpb.ChartValueSet_Single{
										AttributeValues: []*dashboardpb.ChartAttributeValue{
											{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/string-metadata",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "string-value-1"}},
															{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "string-value-2"}},
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
															{Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{NumberValue: 1.1}},
															{Value: &dashboardpb.MetadataForChart_MetadataValue_NumberValue{NumberValue: 2.2}},
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
							Operator: dashboardpb.ChartDataFilter_IN,
							ValueSet: &dashboardpb.ChartValueSet{
								Values: &dashboardpb.ChartValueSet_SingleValues{
									SingleValues: &dashboardpb.ChartValueSet_Single{
										AttributeValues: []*dashboardpb.ChartAttributeValue{
											{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/boolean-metadata",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{BooleanValue: true}},
															{Value: &dashboardpb.MetadataForChart_MetadataValue_BooleanValue{BooleanValue: false}},
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
							// The #NO_VALUE_METADATA# sentinel still uses the nested must_not subquery.
							Operator: dashboardpb.ChartDataFilter_IN,
							ValueSet: &dashboardpb.ChartValueSet{
								Values: &dashboardpb.ChartValueSet_SingleValues{
									SingleValues: &dashboardpb.ChartValueSet_Single{
										AttributeValues: []*dashboardpb.ChartAttributeValue{
											{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/string-metadata-with-missing",
														Values: []*dashboardpb.MetadataForChart_MetadataValue{
															{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "string-value-1"}},
															{Value: &dashboardpb.MetadataForChart_MetadataValue_StringValue{StringValue: "#NO_VALUE_METADATA#"}},
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
							// Numeric ranges cannot be expressed via metadata_id_value_pairs and
							// therefore fall back to the original nested query.
							Operator: dashboardpb.ChartDataFilter_IN,
							ValueSet: &dashboardpb.ChartValueSet{
								Values: &dashboardpb.ChartValueSet_SingleValues{
									SingleValues: &dashboardpb.ChartValueSet_Single{
										AttributeValues: []*dashboardpb.ChartAttributeValue{
											{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/numeric-metadata-with-bin",
														NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
															{
																LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{Value: 0.1},
																UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{Value: 100, IsExclusive: true},
															},
															{
																LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{Value: 100.1},
																UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{Value: 1000.1, IsExclusive: true},
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
							// Exists-only metadata filter: the MetadataForChart has no Values and no
							// NumericValueRanges, so buildMetadataIDValuePairs returns an empty slice
							// and there is nothing to look up in metadata_id_value_pairs. The fast
							// path must preserve the prior exists-only behavior by falling back to the
							// nested filter on moment_template_id rather than returning an error.
							Operator: dashboardpb.ChartDataFilter_IN,
							ValueSet: &dashboardpb.ChartValueSet{
								Values: &dashboardpb.ChartValueSet_SingleValues{
									SingleValues: &dashboardpb.ChartValueSet_Single{
										AttributeValues: []*dashboardpb.ChartAttributeValue{
											{
												Value: &dashboardpb.ChartAttributeValue_MetadataValue{
													MetadataValue: &dashboardpb.MetadataForChart{
														MomentName: "customers/customer_id/profiles/profile_id/moments/string-metadata-exists-only",
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
		esReqPath:  "EvaluateAnalyticsChart_metadata_as_filter_using_convo_level_id_value_pairs_es_request.json",
		esRespPath: "EvaluateAnalyticsChart_metadata_as_filter_es_response.json",
	}

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
							EnableMetadataFilteringConversationLevelMetadata: true,
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
			GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
			Return(testCustomerConfig.ListProfiles()[0], nil)
		r.mockConfigClient.EXPECT().
			Get(gomock.Any()).Times(1).
			Return(testCustomerConfig, nil)
		if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
			r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
		}
		_, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
		if err != nil {
			t.Fatalf("Got code %v in error: %v", err, status.Code(err))
		}
	})
}

func TestExcludeDeactivatedUsersFromConversationAgentBuckets(t *testing.T) {
	activeUserID := "agent-id-1"
	deactivatedUserID := "agent-id-deactivated"
	customerID := "customer_id"
	profileID := "profile_id"
	parent := &shared.CustomerProfile{
		CustomerID: customerID,
		ProfileID:  profileID,
	}

	mockUserServiceClient := usermock.NewMockUserServiceClient(gomock.NewController(t))
	mockUserServiceClient.
		EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.ListUsersResponse, error) {
			if req.GetParent() != "customers/customer_id" ||
				req.GetProfileId() != profileID ||
				req.GetPageSize() != int32(shared.DefaultListUsersPageSize) ||
				req.GetPageToken() != "" ||
				req.GetOrderField() != "user_id" ||
				!cmp.Equal(req.GetFilter().GetCustomerProfiles(), []string{"customers/customer_id/profiles/profile_id"}) ||
				!cmp.Equal(req.GetFilter().GetUserIds(), []string{activeUserID, deactivatedUserID}) ||
				req.GetFilter().GetState() != userpb.User_ACTIVE {
				t.Fatalf("unexpected ListUsers request: %v", req)
			}
			return &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{
						Name: userpb.UserName{
							CustomerID: customerID,
							UserID:     activeUserID,
						}.String(),
					},
				},
			}, nil
		}).
		Times(1)

	service := AnalyticsServiceImpl{
		userClientRegistry: clientregistry.NewTestRegistry[userpb.UserServiceClient](mockUserServiceClient),
	}
	buckets := []ESRespSingleBucket{
		{
			Key:      activeUserID,
			DocCount: 2,
		},
		{
			Key:      deactivatedUserID,
			DocCount: 1,
		},
	}
	esResp := &ChartESResponse{
		Aggregations: &ESRespSingleBucket{
			Buckets: &buckets,
		},
	}

	err := service.excludeDeactivatedUsersFromConversationAgentBuckets(context.Background(), parent, esResp)
	if err != nil {
		t.Fatalf("excludeDeactivatedUsersFromConversationAgentBuckets returned error: %v", err)
	}

	if esResp.Aggregations.Buckets == nil {
		t.Fatal("expected buckets to be populated")
	}
	filteredBuckets := *esResp.Aggregations.Buckets
	if len(filteredBuckets) != 1 {
		t.Fatalf("expected only active user bucket to remain, got %d buckets", len(filteredBuckets))
	}
	if filteredBuckets[0].Key != activeUserID {
		t.Fatalf("expected active user bucket %q, got %v", activeUserID, filteredBuckets[0].Key)
	}
}

func (s *SubcategoryDeduplicationTestSuite) TestFiltersContainMatchingSubcategoryValueSet() {
	s.Run("same subcategory in filter and metric is deduped", func() {
		// Given
		// Existing filter with subcategory-1.
		existingFilters := []*dashboardpb.ChartDataFilter{
			{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: []*dashboardpb.ChartAttributeValue{
								{
									Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
										SubcategoryValue: &dashboardpb.SubcategoryForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// Metric-derived filter with the same subcategory-1.
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := filtersContainMatchingSubcategoryValueSet(s.ctx, existingFilters, metricValueSet)

		// Then
		s.Require().True(result, "should detect duplicate subcategory filter")
	})

	s.Run("different subcategory in filter and metric is not deduped", func() {
		// Given
		existingFilters := []*dashboardpb.ChartDataFilter{
			{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: []*dashboardpb.ChartAttributeValue{
								{
									Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
										SubcategoryValue: &dashboardpb.SubcategoryForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := filtersContainMatchingSubcategoryValueSet(s.ctx, existingFilters, metricValueSet)

		// Then
		s.Require().False(result, "should not match different subcategory moment names")
	})

	s.Run("different aggregate and leaf subcategory in filter and metric is not deduped", func() {
		// Given
		// Existing filter has aggregate-subcategory-1 and subcategory-1.
		existingFilters := []*dashboardpb.ChartDataFilter{
			{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: []*dashboardpb.ChartAttributeValue{
								{
									Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
										AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
										},
									},
								},
								{
									Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
										SubcategoryValue: &dashboardpb.SubcategoryForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// Metric-derived filter has different aggregate-subcategory-2 and subcategory-2.
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := filtersContainMatchingSubcategoryValueSet(s.ctx, existingFilters, metricValueSet)

		// Then
		s.Require().False(result, "should not match when aggregate+leaf subcategory moment names differ")
	})

	s.Run("non subcategory attribute type is not deduped", func() {
		// Given
		existingFilters := []*dashboardpb.ChartDataFilter{
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
							},
						},
					},
				},
			},
		}
		metricValueSet := &dashboardpb.ChartValueSet{
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
					},
				},
			},
		}

		// When
		result := filtersContainMatchingSubcategoryValueSet(s.ctx, existingFilters, metricValueSet)

		// Then
		s.Require().False(result, "should not match non-subcategory attribute types")
	})
}

func (s *SubcategoryDeduplicationTestSuite) TestGroupBysContainMatchingSubcategoryValueSet() {
	s.Run("same aggregate and leaf subcategory in groupby and metric is deduped", func() {
		// Given
		// Existing groupBy with 2 aggregate subcategories and 2 subcategories.
		existingGroupBys := []*dashboardpb.ChartGroupBy{
			{
				GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
					ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
						ValueSet: &dashboardpb.ChartValueSet{
							Values: &dashboardpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardpb.ChartValueSet_Single{
									AttributeValues: []*dashboardpb.ChartAttributeValue{
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
												SubcategoryValue: &dashboardpb.SubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
												SubcategoryValue: &dashboardpb.SubcategoryForChart{
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
			},
			{
				GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
					ByTime: &dashboardpb.ChartGroupBy_Time{
						Interval: dashboardpb.ChartGroupBy_Time_DAY,
					},
				},
			},
		}
		// Metric-derived groupBy with the same 2 aggregate subcategories and 2 subcategories.
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := groupBysContainMatchingSubcategoryValueSet(s.ctx, existingGroupBys, metricValueSet)

		// Then
		s.Require().True(result, "should detect duplicate aggregate+leaf subcategory groupBy")
	})

	s.Run("same aggregate and leaf subcategory in different order is deduped", func() {
		// Given
		existingGroupBys := []*dashboardpb.ChartGroupBy{
			{
				GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
					ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
						ValueSet: &dashboardpb.ChartValueSet{
							Values: &dashboardpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardpb.ChartValueSet_Single{
									AttributeValues: []*dashboardpb.ChartAttributeValue{
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
												SubcategoryValue: &dashboardpb.SubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
												SubcategoryValue: &dashboardpb.SubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
		}
		// Metric-derived groupBy with the same values in a different order.
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := groupBysContainMatchingSubcategoryValueSet(s.ctx, existingGroupBys, metricValueSet)

		// Then
		s.Require().True(result, "should detect duplicate even when moment names are in different order")
	})

	s.Run("aggregate and leaf subcategory with partial overlap is not deduped", func() {
		// Given
		// Existing groupBy has aggregate-subcategory-1 + subcategory-1.
		existingGroupBys := []*dashboardpb.ChartGroupBy{
			{
				GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
					ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
						ValueSet: &dashboardpb.ChartValueSet{
							Values: &dashboardpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardpb.ChartValueSet_Single{
									AttributeValues: []*dashboardpb.ChartAttributeValue{
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
												SubcategoryValue: &dashboardpb.SubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
		}
		// Metric-derived groupBy has same aggregate-subcategory-1 but different subcategory-2.
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-2",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := groupBysContainMatchingSubcategoryValueSet(s.ctx, existingGroupBys, metricValueSet)

		// Then
		s.Require().False(result, "should not match when subcategory moment names differ even if aggregate subcategory matches")
	})

	s.Run("same aggregate subcategory in groupby and metric is deduped", func() {
		// Given
		existingGroupBys := []*dashboardpb.ChartGroupBy{
			{
				GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
					ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
						ValueSet: &dashboardpb.ChartValueSet{
							Values: &dashboardpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardpb.ChartValueSet_Single{
									AttributeValues: []*dashboardpb.ChartAttributeValue{
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
												},
											},
										},
										{
											Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
												AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
													MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
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
		}
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-1",
								},
							},
						},
						{
							Value: &dashboardpb.ChartAttributeValue_AggregateSubcategoryValue{
								AggregateSubcategoryValue: &dashboardpb.AggregateSubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/aggregate-subcategory-2",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := groupBysContainMatchingSubcategoryValueSet(s.ctx, existingGroupBys, metricValueSet)

		// Then
		s.Require().True(result, "should detect duplicate aggregate subcategory groupBy")
	})

	s.Run("no existing groupbys returns false", func() {
		// Given
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := groupBysContainMatchingSubcategoryValueSet(s.ctx, nil, metricValueSet)

		// Then
		s.Require().False(result, "should return false when no groupBys exist")
	})

	s.Run("byTime groupby is skipped", func() {
		// Given
		existingGroupBys := []*dashboardpb.ChartGroupBy{
			{
				GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
					ByTime: &dashboardpb.ChartGroupBy_Time{
						Interval: dashboardpb.ChartGroupBy_Time_DAY,
					},
				},
			},
		}
		metricValueSet := &dashboardpb.ChartValueSet{
			Values: &dashboardpb.ChartValueSet_SingleValues{
				SingleValues: &dashboardpb.ChartValueSet_Single{
					AttributeValues: []*dashboardpb.ChartAttributeValue{
						{
							Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
								SubcategoryValue: &dashboardpb.SubcategoryForChart{
									MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
								},
							},
						},
					},
				},
			},
		}

		// When
		result := groupBysContainMatchingSubcategoryValueSet(s.ctx, existingGroupBys, metricValueSet)

		// Then
		s.Require().False(result, "should skip byTime groupBys")
	})
}

// ---------------------------------------------------------------------------
// determineQueryEngine
// ---------------------------------------------------------------------------

type determineQueryEngineTestSuite struct {
	suite.Suite
}

func TestDetermineQueryEngine(t *testing.T) {
	suite.Run(t, new(determineQueryEngineTestSuite))
}

func (s *determineQueryEngineTestSuite) TestInfraMetrics() {
	infraMetric := func(id string) *dashboardpb.ChartMetric {
		return &dashboardpb.ChartMetric{
			Metric: &dashboardpb.ChartMetric_ForInfraMetric_{
				ForInfraMetric: &dashboardpb.ChartMetric_ForInfraMetric{MetricId: id},
			},
		}
	}

	s.Run("single ForInfraMetric metric selects InfraMetrics engine", func() {
		// Given
		chart := &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{infraMetric("my_metric")},
			},
		}

		// When
		engine, err := determineQueryEngine(chart)

		// Then
		s.Require().NoError(err)
		s.Equal(QueryEngineInfraMetrics, engine)
	})

	s.Run("multiple ForInfraMetric metrics selects InfraMetrics engine", func() {
		// Given
		chart := &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{infraMetric("metric_a"), infraMetric("metric_b")},
			},
		}

		// When
		engine, err := determineQueryEngine(chart)

		// Then
		s.Require().NoError(err)
		s.Equal(QueryEngineInfraMetrics, engine)
	})

	s.Run("mixing ForInfraMetric with non-infra metric returns error", func() {
		// Given
		chart := &dashboardpb.AnalyticsChart{
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					infraMetric("my_metric"),
					{Metric: &dashboardpb.ChartMetric_ForScorecardCriterion_{
						ForScorecardCriterion: &dashboardpb.ChartMetric_ForScorecardCriterion{},
					}},
				},
			},
		}

		// When
		_, err := determineQueryEngine(chart)

		// Then
		s.Error(err)
	})
}

// TestLlmAnnotationValueSetRoundTrip verifies that an LLM annotation chart value set carrying
// both scalar values and numeric value ranges (field 33, llm_annotation) survives the
// fromChartValueSet -> toChartValueSet round-trip. This is the regression guard for numeric
// LLM-annotation ranges being dropped from chart filters and breakdowns (COA-2740): the value
// set now uses LlmAnnotationForChart, mirroring metadata_value, instead of the scalar-only
// deprecated llm_annotation_value (field 30).
func TestLlmAnnotationValueSetRoundTrip(t *testing.T) {
	// Given
	momentName := "customers/customer_id/profiles/profile_id/moments/llm_annotation_number_moment"
	input := &dashboardpb.ChartValueSet{
		Values: &dashboardpb.ChartValueSet_SingleValues{
			SingleValues: &dashboardpb.ChartValueSet_Single{
				AttributeValues: []*dashboardpb.ChartAttributeValue{
					{
						Value: &dashboardpb.ChartAttributeValue_LlmAnnotation{
							LlmAnnotation: &dashboardpb.LlmAnnotationForChart{
								MomentName: momentName,
								Values: []*dashboardpb.LlmAnnotationForChart_AnnotationValue{
									{Value: &dashboardpb.LlmAnnotationForChart_AnnotationValue_NumberValue{NumberValue: 3}},
								},
								NumericValueRanges: []*dashboardpb.MetadataForChart_NumericRange{
									{
										LowerBound: &dashboardpb.MetadataForChart_NumericRange_Bound{Value: 0},
										UpperBound: &dashboardpb.MetadataForChart_NumericRange_Bound{Value: 5, IsExclusive: true},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// When
	cvse, err := fromChartValueSet(context.Background(), input)

	// Then
	if err != nil {
		t.Fatalf("fromChartValueSet returned error: %v", err)
	}
	if cvse.attributeValueType != crestaEntityLlmAnnotation {
		t.Fatalf("attributeValueType = %v, want crestaEntityLlmAnnotation", cvse.attributeValueType)
	}
	if len(cvse.llmAnnotations) != 1 {
		t.Fatalf("llmAnnotations length = %d, want 1", len(cvse.llmAnnotations))
	}
	if got := len(cvse.llmAnnotations[0].GetNumericValueRanges()); got != 1 {
		t.Fatalf("numeric value ranges preserved through fromChartValueSet = %d, want 1", got)
	}

	// When
	got := toChartValueSet(*cvse)

	// Then
	if diff := cmp.Diff(input, got, protocmp.Transform()); diff != "" {
		t.Errorf("value set round-trip mismatch (-want +got):\n%s", diff)
	}
}
