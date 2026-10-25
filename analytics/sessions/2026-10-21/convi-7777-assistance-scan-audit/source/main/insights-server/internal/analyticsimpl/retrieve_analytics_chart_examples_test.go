// nolint:dupl
package analyticsimpl

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/shared-go/fn"

	envutil "github.com/cresta/shared-go/util/env"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/shared/testing/cmputil"
)

type RetrieveAnalyticsChartExamplesTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveAnalyticsChartExamplesTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()
}

// Most of code in this API are shared with RetrieveAnalyticsChartExamples API and thus
// we will only test sentiment for the diff parts.
func TestRetrieveAnalyticsChartExamples(t *testing.T) {
	r := RetrieveAnalyticsChartExamplesTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	const flagInsightsMessageCreateTime = "INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME"
	prevUsePlatformTS := *usePlatformTimestampForMessageCreateTime
	prevEnv, hadPrevEnv := os.LookupEnv(flagInsightsMessageCreateTime)
	envutil.SetEnv(flagInsightsMessageCreateTime, "true")
	if err := envflag.Set(flagInsightsMessageCreateTime, "true"); err != nil {
		t.Fatalf("envflag.Set(%q): %v", flagInsightsMessageCreateTime, err)
	}
	t.Cleanup(func() {
		if hadPrevEnv {
			envutil.SetEnv(flagInsightsMessageCreateTime, prevEnv)
		} else {
			os.Unsetenv(flagInsightsMessageCreateTime)
		}
		_ = envflag.Set(flagInsightsMessageCreateTime, strconv.FormatBool(prevUsePlatformTS))
	})

	testCases := []struct {
		desc                      string
		esReqPath                 string
		esRespPath                string
		req                       *analyticspb.RetrieveAnalyticsChartExamplesRequest
		resp                      *analyticspb.RetrieveAnalyticsChartExamplesResponse
		getCompositeMomentResp    *momentpb.BatchGetMomentsResponse
		getChildMomentResp        *momentpb.BatchGetMomentsResponse
		mockListUsersForAnalytics bool
	}{
		{
			desc: "multiple_cells_selected",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
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
															Timestamp: timestamppb.New(time.Date(2024, 9, 15, 0, 0, 0, 0, time.UTC)),
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
										Timestamp: timestamppb.New(time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC)),
									},
								},
							},
							{
								Value: &dashboardpb.ChartAttributeValue{
									Value: &dashboardpb.ChartAttributeValue_Timestamp{
										Timestamp: timestamppb.New(time.Date(2024, 7, 3, 0, 0, 0, 0, time.UTC)),
									},
								},
							},
						},
					},
				},
				RequestOccurrence: true,
			},
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Message{
						Message: &dashboardpb.MessageExamples{
							MessageItems: []*convopb.ConversationMessage{
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_1",
									Text:    "Awesome.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_2",
									Text:    "Cool.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_2/messages/message_id_3",
									Text:    "Amazing.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 2,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_multiple_cells_selected_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_multiple_cells_selected_response.json",
		},
		{
			desc: "request_occurrence",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
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
							Value: &dashboardpb.ChartAttributeValue_Timestamp{
								Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				},
				RequestOccurrence: true,
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
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Message{
						Message: &dashboardpb.MessageExamples{
							MessageItems: []*convopb.ConversationMessage{
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_1",
									Text:    "Awesome.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_2",
									Text:    "Cool.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_2/messages/message_id_3",
									Text:    "Amazing.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 2,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_request_occurrence_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_request_occurrence_response.json",
		},
		{
			// Two group_by keys are provided, both message-level emotions:
			//   - emotion-1 IS one of the chart metric emotions -> "related to the metric" -> it is
			//     skipped and keeps its occurrence semantics (applied directly on the occurrence
			//     message, no sibling-message wrapper).
			//   - emotion-3 is NOT a metric emotion -> it is treated like an original filter and
			//     added to convoFilterFromMsgEntity (applied as a sibling-message / conversation-level
			//     match).
			desc: "occurrence_group_by_skips_metric_related_filter",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					DisplayName: "display_name",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForEmotion_{
									ForEmotion: &dashboardpb.ChartMetric_ForEmotion{
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{},
					},
					RenderingConfig: &dashboardpb.RenderingConfig{
						ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
					},
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
				GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
					{
						// Related to the metric -> skipped.
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
						},
					},
					{
						// Not related to the metric -> treated like an original message-level filter.
						GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
							{
								Value: &dashboardpb.ChartAttributeValue{
									Value: &dashboardpb.ChartAttributeValue_EmotionValue{
										EmotionValue: &dashboardpb.EmotionForChart{
											MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-3",
										},
									},
								},
							},
						},
					},
				},
				RequestOccurrence: true,
			},
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Message{
						Message: &dashboardpb.MessageExamples{
							MessageItems: []*convopb.ConversationMessage{
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_1",
									Text:    "Awesome.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_2",
									Text:    "Cool.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_2/messages/message_id_3",
									Text:    "Amazing.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 2,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_occurrence_group_by_skips_metric_related_filter_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_request_occurrence_response.json",
		},
		{
			desc: "random_sort",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
										StatsType: &dashboardpb.StatsTypeForEmotion{
											StatsType: &dashboardpb.StatsTypeForEmotion_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
											},
										},
										Emotions: []*dashboardpb.EmotionForChart{
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-1",
											},
											{
												MomentName: "customers/customer_id/profiles/profile_id/moments/emotion-2",
											},
										},
									},
								},
							},
						},
						Filters: []*dashboardpb.ChartDataFilter{},
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
							Value: &dashboardpb.ChartAttributeValue_Timestamp{
								Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				},
				SortOptions: &analyticspb.RetrieveAnalyticsChartExamplesRequest_SortOptions{
					SortBy: analyticspb.RetrieveAnalyticsChartExamplesRequest_SortOptions_SORT_BY_RANDOM,
				},
			},
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Conversation{
						Conversation: &dashboardpb.ConversationExamples{
							ConversationItems: []*convopb.Conversation{
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_1",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_1",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_2",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_2",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 12345,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_random_sort_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_random_sort_response.json",
		},
		{
			desc: "sentiment_with_msg_filter_for_conv_count",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
						TopicMoment: &momentpb.Moment{
							Type: momentpb.Moment_COMPOSITE,
						},
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
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Conversation{
						Conversation: &dashboardpb.ConversationExamples{
							ConversationItems: []*convopb.Conversation{
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_1",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_1",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_2",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_2",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 12345,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_sentiment_with_msg_filter_for_conv_count_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_sentiment_with_msg_filter_for_conv_count_response.json",
		},
		{
			desc: "sentiment_with_msg_filter_for_conv_occur_count",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
											},
										},
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
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Conversation{
						Conversation: &dashboardpb.ConversationExamples{
							ConversationItems: []*convopb.Conversation{
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_1",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_1",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_2",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_2",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 12345,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_sentiment_with_msg_filter_for_conv_occur_count_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_sentiment_with_msg_filter_for_conv_occur_count_response.json",
		},
		{
			desc: "sentiment_with_msg_filter_for_msg_occur_count",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
										StatsType: &dashboardpb.StatsTypeForSentiment{
											StatsType: &dashboardpb.StatsTypeForSentiment_CommonStatsType{
												CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_OCCURRENCE_COUNT,
											},
										},
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
							Value: &dashboardpb.ChartAttributeValue_Timestamp{
								Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				},
			},
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Message{
						Message: &dashboardpb.MessageExamples{
							MessageItems: []*convopb.ConversationMessage{
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_1",
									Text:    "Awesome.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_2",
									Text:    "Cool.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_2/messages/message_id_3",
									Text:    "Amazing.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 2,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_sentiment_with_msg_filter_for_msg_occur_count_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_sentiment_with_msg_filter_for_msg_occur_count_response.json",
		},
		{
			desc: "subcategory_as_filter_for_conv_count",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Conversation{
						Conversation: &dashboardpb.ConversationExamples{
							ConversationItems: []*convopb.Conversation{
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_1",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_1",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
								{
									Name: "customers/customer_id/profiles/profile_id/conversations/conversation_id_2",
									AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									PlatformInfo: &convopb.Conversation_PlatformInfo{
										PlatformConversationId: "plat_conversation_id_2",
										PlatformAccountId:      "agent_id",
									},
									StartTime: timestamppb.New(time.Date(2024, 12, 12, 23, 0, 31, 0, time.UTC)),
									EndTime:   timestamppb.New(time.Date(2024, 12, 12, 23, 17, 7, 0, time.UTC)),
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 12345,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_subcategory_as_filter_for_conv_count_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_subcategory_as_filter_for_conv_count_response.json",
		},
		{
			desc: "subcategory_as_filter_for_occur_count",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
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
										StatsType: &dashboardpb.StatsTypeForSubcategory{
											StatsType: &dashboardpb.StatsTypeForSubcategory_CommonStatsType{
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
			resp: &analyticspb.RetrieveAnalyticsChartExamplesResponse{
				ChartExamples: &dashboardpb.AnalyticsChartExamples{
					Examples: &dashboardpb.AnalyticsChartExamples_Message{
						Message: &dashboardpb.MessageExamples{
							MessageItems: []*convopb.ConversationMessage{
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_1",
									Text:    "<mark>moment_1_keyword_included_1</mark>", // highlighted message text
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_1/messages/message_id_2",
									Text:    "Cool.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
								{
									Name:    "customers/customer_id/profiles/profile_id/conversations/conversation_id_2/messages/message_id_3",
									Text:    "Amazing.",
									Speaker: convopb.ConversationMessage_VISITOR,
									SpeakerInfo: &convopb.ConversationMessage_SpeakerInfo{
										DisplayName: "Test Agent",
									},
									CreateTime: &timestamppb.Timestamp{Seconds: 1734047618, Nanos: 598000000}, // 2024-12-12T23:53:38.598Z platform_timestamp
								},
							},
						},
					},
				},
				TotalExampleCount:      12345,
				TotalConversationCount: 2,
			},
			esReqPath:  "RetrieveAnalyticsChartExamples_subcategory_as_filter_for_occur_count_request.json",
			esRespPath: "RetrieveAnalyticsChartExamples_subcategory_as_filter_for_occur_count_response.json",
		},
		{
			desc: "behavior_request_occurrence_agent_filter_on_conversation",
			req: &analyticspb.RetrieveAnalyticsChartExamplesRequest{
				Parent: r.profileResourceName,
				AnalyticsChart: &dashboardpb.AnalyticsChart{
					DisplayName: "behavior examples",
					DataConfig: &dashboardpb.ChartDataConfig{
						Metrics: []*dashboardpb.ChartMetric{
							{
								Metric: &dashboardpb.ChartMetric_ForBehavior_{
									ForBehavior: &dashboardpb.ChartMetric_ForBehavior{
										Behaviors: []*dashboardpb.BehaviorForChart{
											{
												BehaviorName: "customers/customer_id/profiles/profile_id/behaviors/behavior-1",
												AdherenceTypes: []dashboardpb.BehaviorForChart_AdherenceType{
													dashboardpb.BehaviorForChart_ADHERENCE_TYPE_DID_DO_X,
												},
											},
										},
										StatsType: &dashboardpb.StatsTypeForBehavior{
											StatsType: &dashboardpb.StatsTypeForBehavior_CommonStatsType{
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
															Timestamp: timestamppb.New(time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC)),
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
					Audience:      &dashboardpb.DashboardAudience{},
					Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
				},
				GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
					{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_GroupNameValue{
								GroupNameValue: userpb.GroupName{CustomerID: "customer_id", GroupID: "group-1"}.String(),
							},
						},
					},
					{
						Value: &dashboardpb.ChartAttributeValue{
							Value: &dashboardpb.ChartAttributeValue_UserNameValue{
								UserNameValue: userpb.UserName{CustomerID: "customer_id", UserID: "agent-user-1"}.String(),
							},
						},
					},
				},
				RequestOccurrence: true,
			},
			mockListUsersForAnalytics: true,
			esReqPath:                 "RetrieveAnalyticsChartExamples_behavior_request_occurrence_agent_on_conversation_request.json",
			esRespPath:                "RetrieveAnalyticsChartExamples_request_occurrence_response.json",
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)

			// Create fresh suite for this subtest to ensure proper mock isolation
			subR := RetrieveAnalyticsChartExamplesTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
			subR.SetupSuite()

			// Set up profile config mock
			profileConfig := &nonpubconfig.ProfileConfig{
				Name: "customers/customer_id/profiles/profile_id",
			}
			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					profileConfig,
				},
			}
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
			subR.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(testCustomerConfig.ListProfiles()[0], nil)

			if tc.mockListUsersForAnalytics {
				subR.mockInternalUserClient.EXPECT().ListUsersForAnalytics(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
					func(_ context.Context, _ *internaluserpb.ListUsersForAnalyticsRequest, _ ...grpc.CallOption) (*internaluserpb.ListUsersForAnalyticsResponse, error) {
						return &internaluserpb.ListUsersForAnalyticsResponse{
							Users: []*internaluserpb.LiteUser{{UserId: "agent-user-1"}},
						}, nil
					})
			}

			// Prepare.
			if tc.getCompositeMomentResp != nil && tc.getChildMomentResp != nil {
				gomock.InOrder(
					subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil),
					subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getChildMomentResp, nil))
			} else if tc.getCompositeMomentResp != nil {
				subR.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil)
			}
			subR.mockESServer.Resps = append(subR.mockESServer.Resps, subR.getFile(tc.esRespPath))
			subR.mockESServer.EXPECT().Search(subR.IndexName, subR.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))

			// Execute.
			resp, err := subR.Client.RetrieveAnalyticsChartExamples(subR.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}

			// Verify.
			if tc.resp != nil {
				if diff := cmp.Diff(tc.resp, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("RetrieveAnalyticsChartExamples return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}
