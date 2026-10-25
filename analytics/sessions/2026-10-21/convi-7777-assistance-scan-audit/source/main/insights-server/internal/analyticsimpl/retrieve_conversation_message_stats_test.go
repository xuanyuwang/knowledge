// nolint:dupl
package analyticsimpl

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	labelspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	envutil "github.com/cresta/shared-go/util/env"
)

type RetrieveConversationMessageStatsTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveConversationMessageStatsTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()
}

// To run a sub test case, using this command
// bazel test insights-server/internal/analyticsimpl:retrieve_conversation_message_stats_test --test_filter=TestRetrieveConversationMessageStats/Preview --test_output=all
func TestRetrieveConversationMessageStats(t *testing.T) {
	r := RetrieveConversationMessageStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	testCases := []struct {
		desc                   string
		esReqPath              string
		esRespPath             string
		req                    *analyticspb.RetrieveConversationMessageStatsRequest
		want                   *analyticspb.RetrieveConversationMessageStatsResponse
		getCompositeMomentResp *momentpb.BatchGetMomentsResponse
		getChildMomentResp     *momentpb.BatchGetMomentsResponse
		batchGetMomentResps    []*momentpb.BatchGetMomentsResponse
		listMomentsResp        *momentpb.ListMomentsResponse
		// v3TemplatesResp is the V3 category tree for the applyTopicDiscoveryV3Expansion fetch. Set on
		// V3 cases only; other cases leave it nil so the expansion is a no-op. Kept separate from
		// listMomentsResp, which on aggregate/composite cases carries the COMPOSITE template list.
		v3TemplatesResp *momentpb.ListMomentsResponse
	}{
		{
			desc: "AhtForSubcategory",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{
					ConversationDurationRange: &analyticspb.DurationRange{
						MinDuration: &durationpb.Duration{Seconds: 60},
					},
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
										Type: momentpb.Moment_COMPOSITE,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
										Type: momentpb.Moment_COMPOSITE,
									},
								},
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
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
			esReqPath:  "RetrieveConversationMessageStats_AhtForSubcategory_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_AhtForSubcategory_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 100,
					AverageHandleTime:             &durationpb.Duration{Seconds: 0},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        8,
							TotalConversationMessageCount: 80,
							AverageHandleTime:             &durationpb.Duration{Seconds: 100},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
											Type: momentpb.Moment_COMPOSITE,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 100},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
											Type: momentpb.Moment_COMPOSITE,
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
			desc: "AggregateSubcategoryExpansion",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			esReqPath: "RetrieveConversationMessageStats_AggregateSubcategoryExpansion_es_request.json",
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						{
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-1", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-1",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-topic-cluster-subcategory-1-1", r.customerID, r.profileID),
																},
															},
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-1", r.customerID, r.profileID),
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
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-2", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-1",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_NOT,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/excluded-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
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
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-1", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-2",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-1", r.customerID, r.profileID),
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
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-2", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-2",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-2", r.customerID, r.profileID),
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
					Moments: []*momentpb.Moment{
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-topic-cluster-subcategory-1-1", r.customerID, r.profileID),
							Type: momentpb.Moment_TOPIC,
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-1", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-1-1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-1-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/excluded-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded-keyword-keyword-subcategory-1-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-1", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-2-1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-2-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: momentpb.MomentName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								MomentID:   "aggregate-subcategory-parent-id-1",
							}.String(),
							Type: momentpb.Moment_COMPOSITE,
							Labels: &labelspb.Labels{
								Labels: map[string]string{
									"message_filters": "agent",
								},
							},
						},
						{
							Name: momentpb.MomentName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								MomentID:   "aggregate-subcategory-parent-id-2",
							}.String(),
							Type: momentpb.Moment_COMPOSITE,
							Labels: &labelspb.Labels{
								Labels: map[string]string{
									"message_filters": "agent",
								},
							},
						},
					},
				},
			},
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						Type: momentpb.Moment_COMPOSITE,
					},
					{
						Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						Type: momentpb.Moment_COMPOSITE,
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-1", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-2", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-1", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-2", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
		},
		{
			desc: "BasicRequestConversationDocument",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/sentiment-moment-1",
									Type: momentpb.Moment_SENTIMENT,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/sentiment-moment-2",
									Type: momentpb.Moment_SENTIMENT,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
										Type: momentpb.Moment_TOPIC,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
										Type: momentpb.Moment_TOPIC,
									},
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_BasicRequestConversationDocument_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_BasicRequestConversationDocument_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 100,
					AverageHandleTime:             &durationpb.Duration{Seconds: 0},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        8,
							TotalConversationMessageCount: 80,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
											Type: momentpb.Moment_TOPIC,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
											Type: momentpb.Moment_TOPIC,
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
			desc: "BasicRequest",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/sentiment-moment-1",
									Type: momentpb.Moment_SENTIMENT,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/sentiment-moment-2",
									Type: momentpb.Moment_SENTIMENT,
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
					// TODO: Check ConversationDurationBuckets.
					// ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
					// 	{
					// 		Id: "1",
					// 	},
					// 	{
					// 		Id: "2",
					// 	},
					// },
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
										Type: momentpb.Moment_TOPIC,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
										Type: momentpb.Moment_TOPIC,
									},
								},
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
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
			esReqPath:  "RetrieveConversationMessageStats_BasicRequest_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_BasicRequest_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 100,
					AverageHandleTime:             &durationpb.Duration{Seconds: 0},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        8,
							TotalConversationMessageCount: 80,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
											Type: momentpb.Moment_TOPIC,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
											Type: momentpb.Moment_TOPIC,
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
			desc: "BasicRequest_SkipConvMsgCount",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/sentiment-moment-1",
									Type: momentpb.Moment_SENTIMENT,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/sentiment-moment-2",
									Type: momentpb.Moment_SENTIMENT,
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
					// TODO: Check ConversationDurationBuckets.
					// ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
					// 	{
					// 		Id: "1",
					// 	},
					// 	{
					// 		Id: "2",
					// 	},
					// },
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
										Type: momentpb.Moment_TOPIC,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
										Type: momentpb.Moment_TOPIC,
									},
								},
							},
						},
					},
				},
				ElasticsearchQueryConfig: &analyticspb.ElasticsearchQueryConfig{
					SkipConversationMessageCount: true,
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/composite-moment-2",
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
			esReqPath:  "RetrieveConversationMessageStats_BasicRequest_SkipConvMsgCount_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_BasicRequest_SkipConvMsgCount_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount: 10,
					AverageHandleTime:      &durationpb.Duration{Seconds: 2},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount: 8,
							AverageHandleTime:      &durationpb.Duration{Seconds: 2},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
											Type: momentpb.Moment_TOPIC,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount: 2,
							AverageHandleTime:      &durationpb.Duration{Seconds: 2},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
											Type: momentpb.Moment_TOPIC,
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
			desc: "Preview",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_DAILY,
				FilterMessageByAttribute: &analyticspb.Attribute{
					InsightsTopics: []*analyticspb.InsightsTopic{
						{
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
															Phrase: "msg keyword1",
														},
													},
													{
														Operand: &momentpb.KeywordMomentData_Operand_Phrase{
															Phrase: "msg keyword2",
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
				FilterConversationByAttribute: &analyticspb.Attribute{
					InsightsTopics: []*analyticspb.InsightsTopic{
						{
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
															Phrase: "convo keyword1",
														},
													},
													{
														Operand: &momentpb.KeywordMomentData_Operand_Phrase{
															Phrase: "convo keyword2",
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
			esReqPath: "RetrieveConversationMessageStats_Preview_es_request.json",
		},
		{
			desc: "PreviewWithRegex",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_DAILY,
				FilterMessageByAttribute: &analyticspb.Attribute{
					InsightsTopics: []*analyticspb.InsightsTopic{
						{
							TopicMoment: &momentpb.Moment{
								Type: momentpb.Moment_COMPOSITE,
							},
							IncludedKeywordMoment: &momentpb.Moment{
								Type: momentpb.Moment_KEYWORD,
								MomentData: &momentpb.MomentData{
									Moment: &momentpb.MomentData_KeywordMomentData{
										KeywordMomentData: &momentpb.KeywordMomentData{
											RegexConfig: &momentpb.KeywordMomentData_RegexConfig{
												Expression:        `hello|world`,
												ExcludeExpression: `not.this`,
											},
										},
									},
								},
							},
						},
					},
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					InsightsTopics: []*analyticspb.InsightsTopic{
						{
							TopicMoment: &momentpb.Moment{
								Type: momentpb.Moment_COMPOSITE,
							},
							IncludedKeywordMoment: &momentpb.Moment{
								Type: momentpb.Moment_KEYWORD,
								MomentData: &momentpb.MomentData{
									Moment: &momentpb.MomentData_KeywordMomentData{
										KeywordMomentData: &momentpb.KeywordMomentData{
											RegexConfig: &momentpb.KeywordMomentData_RegexConfig{
												Expression: `\d{5}`,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessageStats_PreviewWithRegex_es_request.json",
		},
		{
			desc: "ConversationDocumentSentimentCount",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},

				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{
					ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
						{
							Id: "1",
						},
						{
							Id: "2",
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_SENTIMENT,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/4",
									Type: momentpb.Moment_SENTIMENT,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/1",
										Type: momentpb.Moment_SENTIMENT,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/4",
										Type: momentpb.Moment_SENTIMENT,
									},
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_ConversationDocumentSentimentCount_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_ConversationDocumentSentimentCount_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 100,
					AverageHandleTime:             &durationpb.Duration{Seconds: 2},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        8,
							TotalConversationMessageCount: 80,
							AverageHandleTime:             &durationpb.Duration{Seconds: 2},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/1",
											Type: momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 4},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/4",
											Type: momentpb.Moment_SENTIMENT,
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
			desc: "TopicDetails",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					SpeakerRoles: []string{"visitor", "agent"},
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					SpeakerRoles: []string{"visitor"},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/2",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/3",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/1",
										Type: momentpb.Moment_COMPOSITE,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/4",
										Type: momentpb.Moment_SENTIMENT,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/5",
										Type: momentpb.Moment_SENTIMENT,
									},
									{
										Name: "customers/customer_id/profiles/profile_id/moments/6",
										Type: momentpb.Moment_SENTIMENT,
									},
								},
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/2",
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
					{
						Name: "customers/customer_id/profiles/profile_id/moments/3",
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
														{
															// TODO: Add moment Flexible: true.
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
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_3_keyword_included_1",
															},
														},
														{
															// TODO: Add moment Flexible: true.
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_3_keyword_included_2",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_3_cluster_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_3_cluster_2",
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
													Phrase: "excluded keyword1",
												},
											},
										},
									},
								},
							},
						},
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
													Phrase: "Connect to Compete",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "C2C",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Affordable Connectivity Program",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "ACP",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Emergency Broadband Benefit",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "EBB",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Government internet",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Straight Up internet",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "SUI",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "School internet",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "low income asisstance",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "low income internet",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "pre-paid",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "cheap internet",
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
													Phrase: "promo",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "promotion",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "discount",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "campaign",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "contract",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "fell off",
												},
											},
										},
									},
								},
							},
						},
					},
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
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_3_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_3_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_3_keyword_included_1",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "keyword incldued_1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_3_keyword_included_2",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "keyword incldued_2",
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
			esReqPath:  "RetrieveConversationMessageStats_TopicDetails_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_TopicDetails_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 100,
					AverageHandleTime:             &durationpb.Duration{Seconds: 0},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/1",
											Type: momentpb.Moment_COMPOSITE,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/4",
											Type: momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/5",
											Type: momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2,
							TotalConversationMessageCount: 20,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/6",
											Type: momentpb.Moment_SENTIMENT,
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
			desc: "ClusterOnly",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					SpeakerRoles: []string{"visitor"},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: "customers/customer_id/profiles/profile_id/moments/pretopic-moment-1",
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/cluster-moment-1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/pretopic-moment-1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/cluster-moment-1",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath: "RetrieveConversationMessageStats_ClusterOnly_es_request.json",
		},
		{
			// Grouping by a non-leaf V3 category produces a single bucket whose moment_annotations
			// nested filter OR-matches the whole subtree closure.
			desc: "TopicDiscoveryV3CategoryGroupBy",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name:         v3TestPrefix + "cat-1",
										Type:         momentpb.Moment_TOPIC,
										DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
									},
								},
							},
						},
					},
				},
			},
			v3TemplatesResp: &momentpb.ListMomentsResponse{Moments: v3Tree()},
			esReqPath:       "RetrieveConversationMessageStats_TopicDiscoveryV3CategoryGroupBy_es_request.json",
			esRespPath:      "RetrieveConversationMessageStats_TopicDiscoveryV3CategoryGroupBy_es_response.json",
			// One bucket labeled cat-1: the grouped-by category stays a single moment stamped TOPIC + V3,
			// locking down the positional bucket -> MomentGroup mapping.
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 100,
					AverageHandleTime:             &durationpb.Duration{Seconds: 3},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        10,
							TotalConversationMessageCount: 100,
							AverageHandleTime:             &durationpb.Duration{Seconds: 3},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name:         v3TestPrefix + "cat-1",
											Type:         momentpb.Moment_TOPIC,
											DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
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
			desc: "GroupByWeekly",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_GroupByWeekly_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_GroupByWeekly_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        1669,
					TotalConversationMessageCount: 11884,
					AverageHandleTime:             &durationpb.Duration{Seconds: 3},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        7,
							TotalConversationMessageCount: 14,
							AverageHandleTime:             &durationpb.Duration{Seconds: 1},
						},
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2022, 4, 10, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        18,
							TotalConversationMessageCount: 40,
							AverageHandleTime:             &durationpb.Duration{Seconds: 2},
						},
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2022, 4, 17, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
					},
				},
			},
		},
		{
			// Asserts that a CustomBucketInterval request emits a date_histogram
			// with `fixed_interval` (no calendar_interval / weekly offset). The
			// 2-day window stays well under the 5000-bucket cap for 20-minute buckets.
			desc: "GroupByCustomBucketInterval",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 4, 15, 0, 0, 0, 0, time.UTC)),
				},
				CustomBucketInterval: durationpb.New(20 * time.Minute),
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_GroupByCustomBucketInterval_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_GroupByCustomBucketInterval_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        10,
					TotalConversationMessageCount: 22,
					AverageHandleTime:             &durationpb.Duration{Seconds: 2},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        4,
							TotalConversationMessageCount: 9,
							AverageHandleTime:             &durationpb.Duration{Seconds: 2},
						},
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        6,
							TotalConversationMessageCount: 13,
							AverageHandleTime:             &durationpb.Duration{Seconds: 3},
						},
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 20, 0, 0, time.UTC)),
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "GroupByLanguageCode",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
				Frequency: analyticspb.Frequency_WEEKLY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE,
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_GroupByLanguageCode_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_GroupByLanguageCode_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        1273652,
					TotalConversationMessageCount: 1272686,
					AverageHandleTime:             &durationpb.Duration{Seconds: 0},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        902393,
							TotalConversationMessageCount: 900934,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							LanguageCodes: []string{"es-MX"},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        372354,
							TotalConversationMessageCount: 371486,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							LanguageCodes: []string{"en-US"},
						},
					},
				},
			},
		},
		{
			desc: "GroupByTimeAndMoment",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/moment_template_id",
										Type: momentpb.Moment_CONVERSATION_METADATA,
									},
								},
								MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
									{
										MetadataValue: &metadata.MetadataValue{
											Value: &metadata.MetadataValue_StringValue{
												StringValue: "California",
											},
										},
									},
									{
										MetadataValue: &metadata.MetadataValue{
											Value: &metadata.MetadataValue_StringValue{
												StringValue: "Connecticut",
											},
										},
									},
								},
							},
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessageStats_GroupByTimeAndMoment_es_request.json",
		},
		{
			desc: "NumericConversationMetadataHistogram",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/manual_kpi_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									NumericBin: &metadata.NumericBin{
										FromValue: 0.,
										ToValue:   20.,
									},
								},
								{
									NumericBin: &metadata.NumericBin{
										FromValue:            100.,
										FromValueIsExclusive: true,
										ToValue:              200.,
										ToValueIsInclusive:   true,
									},
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/manual_kpi_1",
										Type: momentpb.Moment_CONVERSATION_METADATA,
									},
								},
								MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
									{
										NumericBin: &metadata.NumericBin{
											FromValue: 0.,
											ToValue:   20.,
										},
									},
									{
										NumericBin: &metadata.NumericBin{
											FromValue:            100.,
											FromValueIsExclusive: true,
											ToValue:              200.,
											ToValueIsInclusive:   true,
										},
									},
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_NumericConversationMetadataHistogram_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_NumericConversationMetadataHistogram_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        3169,
					TotalConversationMessageCount: 815300,
					AverageHandleTime:             &durationpb.Duration{Seconds: 10},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        174,
							TotalConversationMessageCount: 48271,
							AverageHandleTime:             &durationpb.Duration{Seconds: 10},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/manual_kpi_1",
											Type: momentpb.Moment_CONVERSATION_METADATA,
										},
									},
									MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
										{
											NumericBin: &metadata.NumericBin{
												FromValue: 0.,
												ToValue:   20.,
											},
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        2995,
							TotalConversationMessageCount: 767432,
							AverageHandleTime:             &durationpb.Duration{Seconds: 10},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/manual_kpi_1",
											Type: momentpb.Moment_CONVERSATION_METADATA,
										},
									},
									MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
										{
											NumericBin: &metadata.NumericBin{
												FromValue:            100.,
												FromValueIsExclusive: true,
												ToValue:              200.,
												ToValueIsInclusive:   true,
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
			desc: "ConversationOutcome",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/outcome",
									Type: momentpb.Moment_CONVERSATION_OUTCOME,
								},
							},
							OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
								{
									OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
										Value: &momentpb.ConversationOutcomeValue{
											Value: &momentpb.ConversationOutcomeValue_BooleanValue{
												BooleanValue: true,
											},
										},
										Description: "sale",
									},
								},
								{
									OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
										Value: &momentpb.ConversationOutcomeValue{
											Value: &momentpb.ConversationOutcomeValue_BooleanValue{
												BooleanValue: false,
											},
										},
										Description: "no sale",
									},
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/outcome",
										Type: momentpb.Moment_CONVERSATION_OUTCOME,
									},
								},
								OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
									{
										OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
											Value: &momentpb.ConversationOutcomeValue{
												Value: &momentpb.ConversationOutcomeValue_BooleanValue{
													BooleanValue: true,
												},
											},
											Description: "sale",
										},
									},
									{
										OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
											Value: &momentpb.ConversationOutcomeValue{
												Value: &momentpb.ConversationOutcomeValue_BooleanValue{
													BooleanValue: false,
												},
											},
											Description: "no sale",
										},
									},
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_ConversationOutcome_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_ConversationOutcome_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        104258,
					TotalConversationMessageCount: 104569,
					AverageHandleTime:             &durationpb.Duration{Seconds: 0},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        13139,
							TotalConversationMessageCount: 13141,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/outcome",
											Type: momentpb.Moment_CONVERSATION_OUTCOME,
										},
									},
									OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
										{
											OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
												Value: &momentpb.ConversationOutcomeValue{
													Value: &momentpb.ConversationOutcomeValue_BooleanValue{
														BooleanValue: true,
													},
												},
												Description: "sale",
											},
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        91167,
							TotalConversationMessageCount: 91379,
							AverageHandleTime:             &durationpb.Duration{Seconds: 0},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/outcome",
											Type: momentpb.Moment_CONVERSATION_OUTCOME,
										},
									},
									OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
										{
											OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
												Value: &momentpb.ConversationOutcomeValue{
													Value: &momentpb.ConversationOutcomeValue_BooleanValue{
														BooleanValue: false,
													},
												},
												Description: "no sale",
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
			desc: "StringConversationMetadataHistogram",
			req: &analyticspb.RetrieveConversationMessageStatsRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_template_id",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{
											StringValue: "California",
										},
									},
								},
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{
											StringValue: "Connecticut",
										},
									},
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
							MomentGroup: &analyticspb.MomentGroup{
								Moments: []*momentpb.Moment{
									{
										Name: "customers/customer_id/profiles/profile_id/moments/moment_template_id",
										Type: momentpb.Moment_CONVERSATION_METADATA,
									},
								},
								MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
									{
										MetadataValue: &metadata.MetadataValue{
											Value: &metadata.MetadataValue_StringValue{
												StringValue: "California",
											},
										},
									},
									{
										MetadataValue: &metadata.MetadataValue{
											Value: &metadata.MetadataValue_StringValue{
												StringValue: "Connecticut",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveConversationMessageStats_StringConversationMetadataHistogram_es_request.json",
			esRespPath: "RetrieveConversationMessageStats_StringConversationMetadataHistogram_es_response.json",
			want: &analyticspb.RetrieveConversationMessageStatsResponse{
				AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
					TotalConversationCount:        195,
					TotalConversationMessageCount: 6206,
					AverageHandleTime:             &durationpb.Duration{Seconds: 10},
				},
				ConversationMessageStats: []*analyticspb.ConversationMessageStats{
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        190,
							TotalConversationMessageCount: 6103,
							AverageHandleTime:             &durationpb.Duration{Seconds: 10},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/moment_template_id",
											Type: momentpb.Moment_CONVERSATION_METADATA,
										},
									},
									MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
										{
											MetadataValue: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "California",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        5,
							TotalConversationMessageCount: 103,
							AverageHandleTime:             &durationpb.Duration{Seconds: 10},
						},
						Attribute: &analyticspb.Attribute{
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/moment_template_id",
											Type: momentpb.Moment_CONVERSATION_METADATA,
										},
									},
									MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
										{
											MetadataValue: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "Connecticut",
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
	}
	// applyTopicDiscoveryV3Expansion fires a V3 category-tree fetch for ANY TOPIC filter/group-by.
	// Register it ONCE with AnyTimes reading currentV3TemplatesResp (set per-subtest below): a
	// per-subtest AnyTimes never exhausts, so the first would shadow all later ones (FIFO match).
	var currentV3TemplatesResp *momentpb.ListMomentsResponse
	r.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).
		AnyTimes().
		DoAndReturn(func(_ context.Context, _ *momentpb.ListMomentsRequest, _ ...grpc.CallOption) (*momentpb.ListMomentsResponse, error) {
			if currentV3TemplatesResp != nil {
				return currentV3TemplatesResp, nil
			}
			return &momentpb.ListMomentsResponse{}, nil
		})
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			currentV3TemplatesResp = tc.v3TemplatesResp
			switch {
			case len(tc.batchGetMomentResps) > 0:
				calls := make([]*gomock.Call, 0, len(tc.batchGetMomentResps)+1)
				if tc.listMomentsResp != nil {
					calls = append(calls, r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil))
				}
				for _, resp := range tc.batchGetMomentResps {
					calls = append(calls, r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(resp, nil))
				}
				gomock.InOrder(calls...)
			case tc.getCompositeMomentResp != nil && tc.getChildMomentResp != nil:
				listResp := tc.listMomentsResp
				if listResp == nil {
					listResp = &momentpb.ListMomentsResponse{}
				}
				gomock.InOrder(
					r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(listResp, nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getChildMomentResp, nil))
			case tc.getCompositeMomentResp != nil:
				listResp := tc.listMomentsResp
				if listResp == nil {
					listResp = &momentpb.ListMomentsResponse{}
				}
				gomock.InOrder(
					r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(listResp, nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil))
			}

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).AnyTimes().
				Return(&config.CustomerConfig{}, nil)

			if tc.esRespPath != "" {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			} else {
				r.mockESServer.Resps = append(r.mockESServer.Resps, emptyESHitsResponse)
			}
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			resp, err := r.Client.RetrieveConversationMessageStats(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform(),
					protocmp.IgnoreFields(&convopb.Conversation{}, "start_time"),
					protocmp.IgnoreFields(&convopb.ConversationMessage{}, "create_time")); diff != "" {
					t.Fatalf("RetrieveConversationMessageStats return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}
