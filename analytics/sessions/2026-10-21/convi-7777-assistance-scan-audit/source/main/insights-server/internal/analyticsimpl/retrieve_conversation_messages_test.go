// nolint:dupl
package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	actionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/action"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	labelspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
)

type RetrieveConversationMessagesTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveConversationMessagesTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "user-id1",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 1",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "user-id1",
	}).Error)
}

// setupAggregateSubcategoriesTest sets up mocks for testing aggregate moment expansion.
// Tests that:
// - aggregate-parent (with 2 children) is expanded to leaf-child-1 and leaf-child-2
// - aggregate-empty (with no children) is filtered out and does NOT appear in the ES query
// - leaf-child-1 appears twice in the result (once from expansion, once from direct inclusion)
func (s *RetrieveConversationMessagesTestSuite) setupAggregateSubcategoriesTest() {
	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	aggregateWithChildrenName := fmt.Sprintf("%s/moments/aggregate-parent", parent)
	aggregateNoChildrenName := fmt.Sprintf("%s/moments/aggregate-empty", parent)
	leafChild1Name := fmt.Sprintf("%s/moments/leaf-child-1", parent)
	leafChild2Name := fmt.Sprintf("%s/moments/leaf-child-2", parent)

	// Mock ListMoments to return the aggregate and leaf moments
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(&momentpb.ListMomentsResponse{
			Moments: []*momentpb.Moment{
				{
					Name:                   aggregateWithChildrenName,
					Type:                   momentpb.Moment_COMPOSITE,
					ParentMomentTemplateId: "",
					MomentData:             nil, // Empty = aggregate
				},
				{
					Name:                   aggregateNoChildrenName,
					Type:                   momentpb.Moment_COMPOSITE,
					ParentMomentTemplateId: aggregateWithChildrenName,
					MomentData:             nil, // Empty = aggregate with no children
				},
				{
					Name:                   leafChild1Name,
					Type:                   momentpb.Moment_COMPOSITE,
					ParentMomentTemplateId: aggregateWithChildrenName,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_CompositeMomentData{
							CompositeMomentData: &momentpb.CompositeMomentData{},
						},
					},
				},
				{
					Name:                   leafChild2Name,
					Type:                   momentpb.Moment_COMPOSITE,
					ParentMomentTemplateId: aggregateWithChildrenName,
					MomentData: &momentpb.MomentData{
						Moment: &momentpb.MomentData_CompositeMomentData{
							CompositeMomentData: &momentpb.CompositeMomentData{},
						},
					},
				},
			},
		}, nil)

	// Mock BatchGetMoments with flexible responses
	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			response := &momentpb.BatchGetMomentsResponse{Moments: []*momentpb.Moment{}}
			for _, name := range req.Names {
				if name == leafChild1Name || name == leafChild2Name {
					var keyword string
					if name == leafChild1Name {
						keyword = "keyword1"
					} else {
						keyword = "keyword2"
					}
					response.Moments = append(response.Moments, &momentpb.Moment{
						Name:     name,
						Type:     momentpb.Moment_COMPOSITE,
						AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_OR,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: fmt.Sprintf("%s/moments/%s", parent, keyword),
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					})
				} else if name == fmt.Sprintf("%s/moments/keyword1", parent) || name == fmt.Sprintf("%s/moments/keyword2", parent) {
					var phrase string
					if name == fmt.Sprintf("%s/moments/keyword1", parent) {
						phrase = "test keyword 1"
					} else {
						phrase = "test keyword 2"
					}
					response.Moments = append(response.Moments, &momentpb.Moment{
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
					})
				}
			}
			return response, nil
		})
}

// To run a sub test case, use the following command
// bazel test insights-server/internal/analyticsimpl:retrieve_conversation_messages_test --test_filter=TestRetrieveConversationMessages/Preview --test_output=all
func TestRetrieveConversationMessages(t *testing.T) {
	r := RetrieveConversationMessagesTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
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
		desc                   string
		esReqPath              string
		esRespPath             string
		req                    *analyticspb.RetrieveConversationMessagesRequest
		want                   *analyticspb.RetrieveConversationMessagesResponse
		getCompositeMomentResp *momentpb.BatchGetMomentsResponse
		getChildMomentResp     *momentpb.BatchGetMomentsResponse
		// v3TemplatesResp is the V3 category tree for the applyTopicDiscoveryV3Expansion fetch. Set on
		// V3 cases only; other cases leave it nil so the expansion is a no-op.
		v3TemplatesResp *momentpb.ListMomentsResponse
	}{
		{
			desc:      "ConversationIDWithCompositeMoments",
			esReqPath: "RetrieveConversationMessages_ConversationIDWithCompositeMoments_es_request.json",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Etc/GMT-2"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Unix(0, 0)),
					EndTimestamp:   timestamppb.New(time.Date(2025, 12, 15, 19, 10, 33, int(time.Millisecond)*140, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					Conversations: []*convopb.Conversation{
						{
							Name: "customers/customer_id/profiles/profile_id/conversations/019af074-af3b-79fb-a476-6ccef7fb21fe",
							PlatformInfo: &convopb.Conversation_PlatformInfo{
								PlatformConversationId: "fake",
								PlatformAccountId:      "fake",
							},
						},
					},
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
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
							},
						},
					},
					LanguageCodes: []string{"en-US"},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
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
						Name:                   "customers/customer_id/profiles/profile_id/moments/2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
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
													},
												},
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
						Name: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
						Type: momentpb.Moment_COMPOSITE,
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
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "flexible",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "keyword",
																},
															},
														},
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
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
		},
		{
			// Insights composite moments placed in FilterConversationByAttribute are routed to the
			// child message filter. The conversation-level language_codes must drill the composite
			// moments' included/excluded keyword phrases down to the matching
			// message_text_with_custom_analyzer.<language> field (spanish here) instead of fanning
			// out across every supported analyzer field.
			desc:      "ConversationAttributeCompositeMomentsWithLanguageCodes",
			esReqPath: "RetrieveConversationMessages_ConversationAttributeCompositeMomentsWithLanguageCodes_es_request.json",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Etc/GMT-2"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Unix(0, 0)),
					EndTimestamp:   timestamppb.New(time.Date(2025, 12, 15, 19, 10, 33, int(time.Millisecond)*140, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
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
							},
						},
					},
					LanguageCodes: []string{"es-ES"},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
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
						Name:                   "customers/customer_id/profiles/profile_id/moments/2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
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
													},
												},
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
						Name: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
						Type: momentpb.Moment_COMPOSITE,
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
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "flexible",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "keyword",
																},
															},
														},
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
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
		},
		{
			desc: "BasicRequest",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
				FilterConversationByAttribute: &analyticspb.Attribute{
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
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_BasicRequest_es_request.json",
		},
		{
			desc: "BasicRequestRandomSort",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
				FilterConversationByAttribute: &analyticspb.Attribute{
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
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
				SortOption:        analyticspb.RetrieveConversationMessagesRequest_SORT_OPTION_RANDOM,
				SeedForRandomSort: 123456,
			},
			esReqPath: "RetrieveConversationMessages_BasicRequestRandomSort_es_request.json",
		},
		{
			desc: "RandomSortWithPagination",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
				FilterConversationByAttribute: &analyticspb.Attribute{
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
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
				PageToken:         "value-for-sort-key1###value-for-sort-key2",
				PageSize:          2,
				SortOption:        analyticspb.RetrieveConversationMessagesRequest_SORT_OPTION_RANDOM,
				SeedForRandomSort: 123456,
			},
			esReqPath:  "RetrieveConversationMessages_RandomSortWithPagination_es_request.json",
			esRespPath: "RetrieveConversationMessages_RandomSortWithPagination_es_response.json",
			want: &analyticspb.RetrieveConversationMessagesResponse{
				ConversationMessageResults: []*analyticspb.ConversationMessageResult{
					{
						ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
							Conversation: &convopb.Conversation{
								Name:         fmt.Sprintf("customers/%s/profiles/%s/conversations/%s", r.customerID, r.profileID, "convo-id-1"),
								LegacyChatId: 111,
								StartTime:    timestamppb.New(time.Date(2022, 7, 11, 16, 0, 0, 0, time.UTC)),
								EndTime:      timestamppb.New(time.Date(2022, 7, 11, 17, 0, 0, 0, time.UTC)),
								PlatformInfo: &convopb.Conversation_PlatformInfo{
									PlatformConversationId: "803206930507824894",
									PlatformAccountId:      "807690351445127928",
								},
								ConversationCorrelationId: "convo-correlation-id-1",
							},
							MomentResults: []*analyticspb.MomentResult{
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-1"),
										DisplayName: "moment-displayname-1",
										Taxonomy:    "moment-taxonomy-1",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-2"),
										DisplayName: "moment-displayname-2",
										Taxonomy:    "moment-taxonomy-2",
										Type:        momentpb.Moment_KEYWORD,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-topic-1"),
										DisplayName: "moment-displayname-topic-1",
										Taxonomy:    "moment-taxonomy-topic-1",
										Type:        momentpb.Moment_TOPIC,
									},
									MomentAnnotation: &momentpb.MomentAnnotation{
										Type: momentpb.Moment_TOPIC,
										Payload: &momentpb.MomentAnnotation_TopicPayload{
											TopicPayload: &momentpb.TopicPayload{
												ConversationReason: "Visitor wants to buy a product.",
												ExtractedQuestion:  "How much does it cost?",
											},
										},
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
							},
						},
						MessageResults: []*analyticspb.ConversationMessageResult_MessageResult{
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-1"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-1",
									SequencePosition: proto.Int32(1),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 15, 59, 59, 500000000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-1"),
											DisplayName: "moment-displayname-1",
											Taxonomy:    "moment-taxonomy-1",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-2"),
											DisplayName: "moment-displayname-2",
											Taxonomy:    "moment-taxonomy-2",
											Type:        momentpb.Moment_KEYWORD,
										},
									},
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-topic-1"),
											DisplayName: "moment-displayname-topic-1",
											Taxonomy:    "moment-taxonomy-topic-1",
											Type:        momentpb.Moment_TOPIC,
										},
										MomentAnnotation: &momentpb.MomentAnnotation{
											Type: momentpb.Moment_TOPIC,
											Payload: &momentpb.MomentAnnotation_TopicPayload{
												TopicPayload: &momentpb.TopicPayload{
													ConversationReason: "Visitor wants to buy a product.",
													ExtractedQuestion:  "How much does it cost?",
												},
											},
										},
									},
								},
							},
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-2"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-2-message-group",
									SequencePosition: proto.Int32(2),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 16, 0, 1, 500000000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
											DisplayName: "moment-displayname-3",
											Taxonomy:    "moment-taxonomy-3",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-2"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-2+dummy_message_text-3",
									SequencePosition: proto.Int32(3),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 16, 0, 3, 433157000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
											DisplayName: "moment-displayname-3",
											Taxonomy:    "moment-taxonomy-3",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
						},
					},
				},
				NextPageToken: "2022-05-27T02:11:54.505Z###2022-05-27T02:11:52.505Z",
			},
		},
		{
			desc: "RandomSortPaginationFilterConversationDocument",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
				FilterConversationByAttribute: &analyticspb.Attribute{
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
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
				PageToken:         "value-for-sort-key1###value-for-sort-key2",
				PageSize:          2,
				SortOption:        analyticspb.RetrieveConversationMessagesRequest_SORT_OPTION_RANDOM,
				SeedForRandomSort: 123456,
			},
			esReqPath: "RetrieveConversationMessages_RandomSortPaginationFilterConversationDocument_es_request.json",
		},
		{
			desc: "ExcludedMomentsForMessage",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/excluded-moment1",
									Type: momentpb.Moment_TOPIC,
								},
							},
						},
					},
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
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
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_ExcludedMomentsForMessage_es_request.json",
		},
		{
			desc: "TopicMomentAsConvoLevelFilter",
			req: &analyticspb.RetrieveConversationMessagesRequest{
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
									Name: "customers/customer_id/profiles/profile_id/moments/f0a590e0-ccc7-4981-9634-4cfaf67c782c",
									Type: momentpb.Moment_TOPIC,
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_TopicMomentAsConvoLevelFilter_es_request.json",
		},
		{
			// A V3 category used as a conversation-level filter expands to its full subtree closure and
			// routes through the message-anchored has_child placement (type "message").
			desc: "TopicDiscoveryV3CategoryConvoFilter",
			req: &analyticspb.RetrieveConversationMessagesRequest{
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
									Name:         v3TestPrefix + "cat-1",
									Type:         momentpb.Moment_TOPIC,
									DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
								},
							},
						},
					},
				},
			},
			v3TemplatesResp: &momentpb.ListMomentsResponse{Moments: v3Tree()},
			esReqPath:       "RetrieveConversationMessages_TopicDiscoveryV3CategoryConvoFilter_es_request.json",
		},
		{
			desc: "IgnoreESFields",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
				FilterConversationByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
				ElasticsearchQueryConfig: &analyticspb.ElasticsearchQueryConfig{
					IgnoredFields: []string{"moment_annotations"},
				},
			},
			esReqPath: "RetrieveConversationMessages_IgnoreESFields_es_request.json",
		},
		{
			desc: "SkipHighlight",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
				FilterConversationByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					SpeakerRoles:        []string{"agent", "visitor"},
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/user-id1",
						},
					},
				},
				ElasticsearchQueryConfig: &analyticspb.ElasticsearchQueryConfig{
					SkipHighlightInMessageText: true,
				},
			},
			esReqPath: "RetrieveConversationMessages_SkipHighlight_es_request.json",
		},
		{
			desc: "Preview",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					LanguageCodes: []string{"pt-PT"},
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
													{
														Operand: &momentpb.KeywordMomentData_Operand_Phrase{
															Phrase: "AT&T",
														},
													},
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
				PageToken: "value-for-sort-key1###value-for-sort-key2",
			},
			esReqPath: "RetrieveConversationMessages_Preview_es_request.json",
		},
		{
			desc: "TopicDetailsWithDummyTopics",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'message_level_search_keyword1' OR 'message_level_search_keyword2'"},
					SpeakerRoles:        []string{"agent", "visitor"},
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
							},
						},
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/3",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'convo_level_search_keyword1' OR 'convo_level_search_keyword2'"},
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
							},
						},
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/3",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								// TODO: Update moment for
								// KeywordMomentConstraint: &elasticsearch.KeywordMomentConstraint{
								// 	MessageForwardPostionUpperBound:  proto.Int32(5),
								// 	MessageBackwardPostionUpperBound: proto.Int32(5),
								// },
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_NOT,
													// TODO: Add moment SkipStemming: true
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
																// TODO: Update moment for SkipStemming: true
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
						Name:                   "customers/customer_id/profiles/profile_id/moments/2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
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
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/3",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-2",
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
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_3_keyword_included_1",
															},
														},
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_3_keyword_included_1",
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
						Name: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
						Type: momentpb.Moment_COMPOSITE,
						Labels: &labelspb.Labels{
							Labels: map[string]string{
								speakerRoleFilterLabelKey:           "speaker_roles_of_parent1",
								messageWindowAsMessageCountLabelKey: "10",
								messageWindowAsDurationLabelKey:     "2",
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/parent-moment-2",
						Type: momentpb.Moment_COMPOSITE,
						Labels: &labelspb.Labels{
							Labels: map[string]string{
								speakerRoleFilterLabelKey: "speaker_roles_of_parent2",
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
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "flexible",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "keyword",
																},
															},
														},
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
													Phrase: "moment_3_keyword_included_1",
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
													Phrase: "moment_3_keyword_included_2",
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
				},
			},
			esReqPath: "RetrieveConversationMessages_TopicDetailsWithDummyTopics_es_request.json",
		},
		{
			desc: "PopulateMomentsMatchedOnTheFly",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
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
							},
						},
					},
				},
				PopulateMomentMatchOnTheFly: true,
			},
			getCompositeMomentResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name:                   "customers/customer_id/profiles/profile_id/moments/1",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								// TODO: Update moment for
								// KeywordMomentConstraint: &elasticsearch.KeywordMomentConstraint{
								// 	MessageForwardPostionUpperBound:  proto.Int32(5),
								// 	MessageBackwardPostionUpperBound: proto.Int32(5),
								// },
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
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded",
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
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included",
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
						Name:                   "customers/customer_id/profiles/profile_id/moments/2",
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/parent-moment-2",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								// TODO: Update moment for
								// KeywordMomentConstraint: &elasticsearch.KeywordMomentConstraint{
								// 	MessageForwardPostionUpperBound:  proto.Int32(5),
								// 	MessageBackwardPostionUpperBound: proto.Int32(5),
								// },
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
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded",
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
																BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included",
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
						Name: "customers/customer_id/profiles/profile_id/moments/parent-moment-1",
						Type: momentpb.Moment_COMPOSITE,
						Labels: &labelspb.Labels{
							Labels: map[string]string{
								speakerRoleFilterLabelKey: "speaker_roles_of_parent1",
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/parent-moment-2",
						Type: momentpb.Moment_COMPOSITE,
						Labels: &labelspb.Labels{
							Labels: map[string]string{
								speakerRoleFilterLabelKey: "speaker_roles_of_parent2",
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment1 included keyword",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_excluded",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment1 excluded keyword1",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment1 excluded keyword1",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_included",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment2 included keyword",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_keyword_excluded",
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "moment2 excluded keyword",
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
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_1",
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_2_cluster_2",
						Type: momentpb.Moment_TOPIC,
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_PopulateMomentsMatchedOnTheFly_es_request.json",
		},
		{
			desc: "ConvoIDs",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					Conversations: []*convopb.Conversation{
						{
							Name: "customers/customer_id/profiles/profile_id/conversations/29c4521b0791d2883e110da6c9f2eec6",
							PlatformInfo: &convopb.Conversation_PlatformInfo{
								PlatformConversationId: "fake",
								PlatformAccountId:      "fake",
							},
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_ConvoIDs_es_request.json",
		},
		{
			desc: "UsecaseIDs",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/usecase_id_1"},
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/usecase_id_1"},
				},
			},
			esReqPath: "RetrieveConversationMessages_UsecaseIDs_es_request.json",
		},
		{
			desc: "ConversationReasonMomentType",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							MomentType: momentpb.Moment_CONVERSATION_REASON,
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_ConversationReasonMomentType_es_request.json",
		},
		{
			desc: "OnlyLanguageCodes",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					LanguageCodes: []string{"en-US", "es-ES"},
				},
			},
			esReqPath: "RetrieveConversationMessages_OnlyLanguageCodes_es_request.json",
		},
		{
			desc: "KeywordAndLanguageCodes",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'internet promotion' OR 'discount'"},
					LanguageCodes:       []string{"en-US", "es-ES"},
				},
			},
			esReqPath: "RetrieveConversationMessages_KeywordAndLanguageCodes_es_request.json",
		},
		{
			desc: "ConversationSentiment",
			req: &analyticspb.RetrieveConversationMessagesRequest{
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
									Name:     "customers/customer_id/profiles/profile_id/moments/4",
									Taxonomy: "sentiment.neutral",
									Type:     momentpb.Moment_SENTIMENT,
								},
							},
						},
					},
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
			},
			esReqPath:  "RetrieveConversationMessages_ConversationSentiment_es_request.json",
			esRespPath: "RetrieveConversationMessages_ConversationSentiment_es_response.json",
			want: &analyticspb.RetrieveConversationMessagesResponse{
				ConversationMessageResults: []*analyticspb.ConversationMessageResult{
					{
						ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
							Conversation: &convopb.Conversation{
								Name:         "customers/customer_id/profiles/profile_id/conversations/convo-id-1",
								LegacyChatId: 123,
								AgentInfo: &convopb.ConversationMessage_SpeakerInfo{
									DisplayName: "agent name 1",
								},
								StartTime: timestamppb.New(time.Date(2022, 7, 11, 16, 0, 0, 0, time.UTC)),
								EndTime:   timestamppb.New(time.Date(2022, 7, 11, 17, 0, 0, 0, time.UTC)),
								PlatformInfo: &convopb.Conversation_PlatformInfo{
									PlatformConversationId: "eb34a0b5-8d15-4902-886d-a139a787b08e",
									PlatformAccountId:      "sample-platform_account_id",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "ActionGroup_HintEngaged",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					ActionGroups: []*analyticspb.ActionGroup{
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/convo-action-1",
									},
								},
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/convo-action-2",
									},
								},
							},
						},
					},
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					ActionGroups: []*analyticspb.ActionGroup{
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/msg-action-1",
										Type: actionpb.Action_HINT,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_ENGAGED,
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_ActionGroup_HintEngaged_es_request.json",
		},
		{
			desc: "ActionGroup_HintNotEngaged",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{},
				FilterMessageByAttribute: &analyticspb.Attribute{
					ActionGroups: []*analyticspb.ActionGroup{
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/msg-action-1",
										Type: actionpb.Action_HINT,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_NOT_ENGAGED,
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_ActionGroup_HintNotEngaged_es_request.json",
		},
		{
			desc: "ActionGroup_HintEngagementNotSpecified",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					ActionGroups: []*analyticspb.ActionGroup{
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/msg-action-1",
										Type: actionpb.Action_HINT,
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_ActionGroup_HintEngagementNotSpecified_es_request.json",
		},
		{
			desc: "NegativeOutcomeBehaviorConvoFilter",
			req: &analyticspb.RetrieveConversationMessagesRequest{
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
									Name:         "customers/customer_id/profiles/profile_id/moments/outcome",
									Type:         momentpb.Moment_CONVERSATION_METADATA,
									DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
									MomentData: &momentpb.MomentData{
										Moment: &momentpb.MomentData_ConversationMetadataMomentData{
											ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
												Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
											},
										},
									},
								},
							},
							BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior",
									Type: momentpb.Moment_INTENT,
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
					},
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
			},
			esReqPath: "RetrieveConversationMessages_NegativeOutcomeBehaviorConvoFilter_es_request.json",
		},
		{
			desc: "PositiveConversationOutcomeBehaviorMessageFilter",
			req: &analyticspb.RetrieveConversationMessagesRequest{
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
									Name: "customers/customer_id/profiles/profile_id/moments/outcome",
									Type: momentpb.Moment_CONVERSATION_OUTCOME,
									MomentData: &momentpb.MomentData{
										Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
											ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
												OutcomeValues: []*momentpb.ConversationOutcomeMomentData_OutcomeValue{
													{
														Value: &momentpb.ConversationOutcomeValue{
															Value: &momentpb.ConversationOutcomeValue_BooleanValue{
																BooleanValue: true,
															},
														},
														Description:            "sale",
														BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
													},
													{
														Value: &momentpb.ConversationOutcomeValue{
															Value: &momentpb.ConversationOutcomeValue_BooleanValue{
																BooleanValue: false,
															},
														},
														Description:            "no-sale",
														BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
													},
												},
											},
										},
									},
								},
							},
							BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
						},
					},
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior",
									Type: momentpb.Moment_INTENT,
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
					},
				},
			},
			esReqPath: "RetrieveConversationMessages_PositiveConversationOutcomeBehaviorMessageFilter_es_request.json",
		},
		{
			desc: "ConvoOutcomeNumericBinFilter",
			req: &analyticspb.RetrieveConversationMessagesRequest{
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
									Name: "customers/customer_id/profiles/profile_id/moments/outcome-1",
									Type: momentpb.Moment_CONVERSATION_OUTCOME,
								},
							},
							OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
								{
									NumericBin: &metadata.NumericBin{
										FromValue: 50,
										ToValue:   100,
									},
								},
								{
									OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
										Value: &momentpb.ConversationOutcomeValue{
											Value: &momentpb.ConversationOutcomeValue_NumberValue{
												NumberValue: 111,
											},
										},
									},
								},
							},
						},
					},
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
			},
			esReqPath: "RetrieveConversationMessages_ConvoOutcomeNumericBinFilter_es_request.json",
		},
		{
			desc: "VerifyResponse",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
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
									Name:     "customers/customer_id/profiles/profile_id/moments/4",
									Taxonomy: "sentiment.neutral",
									Type:     momentpb.Moment_SENTIMENT,
								},
							},
						},
					},
				},
				FilterConversationDocumentByConversationByAttribute: &analyticspb.Attribute{},
				PageSize:  2,
				PageToken: "value-for-sort-key1",
			},
			esReqPath:  "RetrieveConversationMessages_VerifyResponse_es_request.json",
			esRespPath: "RetrieveConversationMessages_VerifyResponse_es_response.json",
			want: &analyticspb.RetrieveConversationMessagesResponse{
				ConversationMessageResults: []*analyticspb.ConversationMessageResult{
					{
						ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
							Conversation: &convopb.Conversation{
								Name:         fmt.Sprintf("customers/%s/profiles/%s/conversations/%s", r.customerID, r.profileID, "convo-id-1"),
								LegacyChatId: 111,
								StartTime:    timestamppb.New(time.Date(2022, 7, 11, 16, 0, 0, 0, time.UTC)),
								EndTime:      timestamppb.New(time.Date(2022, 7, 11, 17, 0, 0, 0, time.UTC)),
								PlatformInfo: &convopb.Conversation_PlatformInfo{
									PlatformConversationId: "803206930507824894",
									PlatformAccountId:      "807690351445127928",
								},
								ConversationCorrelationId: "convo-correlation-id-1",
							},
							MomentResults: []*analyticspb.MomentResult{
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-1"),
										DisplayName: "moment-displayname-1",
										Taxonomy:    "moment-taxonomy-1",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-2"),
										DisplayName: "moment-displayname-2",
										Taxonomy:    "moment-taxonomy-2",
										Type:        momentpb.Moment_KEYWORD,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-topic-1"),
										DisplayName: "moment-displayname-topic-1",
										Taxonomy:    "moment-taxonomy-topic-1",
										Type:        momentpb.Moment_TOPIC,
									},
									MomentAnnotation: &momentpb.MomentAnnotation{
										Type: momentpb.Moment_TOPIC,
										Payload: &momentpb.MomentAnnotation_TopicPayload{
											TopicPayload: &momentpb.TopicPayload{
												ConversationReason: "Visitor wants to buy a product.",
												ExtractedQuestion:  "How much does it cost?",
											},
										},
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
							},
						},
						MessageResults: []*analyticspb.ConversationMessageResult_MessageResult{
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-1"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-1",
									SequencePosition: proto.Int32(1),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 15, 59, 59, 500000000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-1"),
											DisplayName: "moment-displayname-1",
											Taxonomy:    "moment-taxonomy-1",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-2"),
											DisplayName: "moment-displayname-2",
											Taxonomy:    "moment-taxonomy-2",
											Type:        momentpb.Moment_KEYWORD,
										},
									},
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-topic-1"),
											DisplayName: "moment-displayname-topic-1",
											Taxonomy:    "moment-taxonomy-topic-1",
											Type:        momentpb.Moment_TOPIC,
										},
										MomentAnnotation: &momentpb.MomentAnnotation{
											Type: momentpb.Moment_TOPIC,
											Payload: &momentpb.MomentAnnotation_TopicPayload{
												TopicPayload: &momentpb.TopicPayload{
													ConversationReason: "Visitor wants to buy a product.",
													ExtractedQuestion:  "How much does it cost?",
												},
											},
										},
									},
								},
							},
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-2"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-2-message-group",
									SequencePosition: proto.Int32(2),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 16, 0, 1, 500000000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
											DisplayName: "moment-displayname-3",
											Taxonomy:    "moment-taxonomy-3",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-2"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-2+dummy_message_text-3",
									SequencePosition: proto.Int32(3),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 16, 0, 3, 433157000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
											DisplayName: "moment-displayname-3",
											Taxonomy:    "moment-taxonomy-3",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
						},
					},
				},
				NextPageToken: "2022-05-27T02:11:54.505Z###2022-05-27T02:11:52.505Z",
			},
		},
		{
			desc: "VerifyResponse_KeywordExpression",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
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
									Name:     "customers/customer_id/profiles/profile_id/moments/4",
									Taxonomy: "sentiment.neutral",
									Type:     momentpb.Moment_SENTIMENT,
								},
							},
						},
					},
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					KeywordsExpressions: []string{"'this is'"},
				},
			},
			esReqPath:  "RetrieveConversationMessages_VerifyResponse_KeywordExpression_es_request.json",
			esRespPath: "RetrieveConversationMessages_VerifyResponse_KeywordExpression_es_response.json",
			want: &analyticspb.RetrieveConversationMessagesResponse{
				ConversationMessageResults: []*analyticspb.ConversationMessageResult{
					{
						ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
							Conversation: &convopb.Conversation{
								Name:         fmt.Sprintf("customers/%s/profiles/%s/conversations/%s", r.customerID, r.profileID, "convo-id-1"),
								LegacyChatId: 111,
								StartTime:    timestamppb.New(time.Date(2022, 7, 11, 16, 0, 0, 0, time.UTC)),
								EndTime:      timestamppb.New(time.Date(2022, 7, 11, 17, 0, 0, 0, time.UTC)),
								PlatformInfo: &convopb.Conversation_PlatformInfo{
									PlatformConversationId: "803206930507824894",
									PlatformAccountId:      "807690351445127928",
								},
							},
							MomentResults: []*analyticspb.MomentResult{
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-1"),
										DisplayName: "moment-displayname-1",
										Taxonomy:    "moment-taxonomy-1",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-2"),
										DisplayName: "moment-displayname-2",
										Taxonomy:    "moment-taxonomy-2",
										Type:        momentpb.Moment_KEYWORD,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
								{
									Moment: &momentpb.Moment{
										Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
										DisplayName: "moment-displayname-3",
										Taxonomy:    "moment-taxonomy-3",
										Type:        momentpb.Moment_SENTIMENT,
									},
								},
							},
						},
						MessageResults: []*analyticspb.ConversationMessageResult_MessageResult{
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-1"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "Great! Which credit card would you like to use for this payment? You have a card ending in [CREDIT-CARD] and another ending in [CREDIT-CARD]. Please remember, <mark>this is</mark> a demo and these are not real credit cards.",
									SequencePosition: proto.Int32(1),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 15, 59, 59, 500000000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-1"),
											DisplayName: "moment-displayname-1",
											Taxonomy:    "moment-taxonomy-1",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-2"),
											DisplayName: "moment-displayname-2",
											Taxonomy:    "moment-taxonomy-2",
											Type:        momentpb.Moment_KEYWORD,
										},
									},
								},
							},
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-2"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-2-message-group",
									SequencePosition: proto.Int32(2),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 16, 0, 1, 500000000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
											DisplayName: "moment-displayname-3",
											Taxonomy:    "moment-taxonomy-3",
											Type:        momentpb.Moment_SENTIMENT,
										},
									},
								},
							},
							{
								ConversationMessage: &convopb.ConversationMessage{
									Name:             fmt.Sprintf("customers/%s/profiles/%s/conversations/%s/messages/%s", r.customerID, r.profileID, "convo-id-1", "convo-msg-id-2"),
									Speaker:          convopb.ConversationMessage_VISITOR,
									Text:             "dummy-message-text-2+dummy_message_text-3",
									SequencePosition: proto.Int32(3),
									CreateTime:       timestamppb.New(time.Date(2022, 7, 11, 16, 0, 3, 433157000, time.UTC)),
								},
								MomentResults: []*analyticspb.MomentResult{
									{
										Moment: &momentpb.Moment{
											Name:        fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, "moment-id-3"),
											DisplayName: "moment-displayname-3",
											Taxonomy:    "moment-taxonomy-3",
											Type:        momentpb.Moment_SENTIMENT,
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
			desc:       "AggregateSubcategories",
			esReqPath:  "RetrieveConversationMessages_AggregateSubcategories_es_request.json",
			esRespPath: "RetrieveConversationMessages_AggregateSubcategories_es_response.json",
			req: &analyticspb.RetrieveConversationMessagesRequest{
				Parent:   r.profileResourceName,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Etc/GMT-2"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Unix(0, 0)),
					EndTimestamp:   timestamppb.New(time.Date(2025, 12, 15, 19, 10, 33, int(time.Millisecond)*140, time.UTC)),
				},
				FilterMessageByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									// aggregate-parent will be expanded to leaf-child-1 and leaf-child-2
									Name: "customers/customer_id/profiles/profile_id/moments/aggregate-parent",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									// aggregate-empty has no children, so it will NOT appear in the ES query
									Name: "customers/customer_id/profiles/profile_id/moments/aggregate-empty",
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									// leaf-child-1 is also directly included (in addition to being expanded from aggregate-parent)
									Name: "customers/customer_id/profiles/profile_id/moments/leaf-child-1",
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
			},
			// This will be handled with special mock setup below
		},
	}
	// applyTopicDiscoveryV3Expansion fires a V3 category-tree fetch for ANY TOPIC filter. Register it
	// ONCE with AnyTimes reading currentV3TemplatesResp (set per-subtest below): a per-subtest AnyTimes
	// never exhausts, so the first would shadow all later ones (FIFO match).
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
			// Special handling for AggregateSubcategories test
			if tc.desc == "AggregateSubcategories" {
				r.setupAggregateSubcategoriesTest()
			} else {
				// For tests with composite moments, mock ListMoments to return them
				// so resolveAggregateMoments can classify them correctly
				if tc.getCompositeMomentResp != nil {
					r.mockMomentClient.EXPECT().
						ListMoments(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&momentpb.ListMomentsResponse{
							Moments: tc.getCompositeMomentResp.Moments,
						}, nil)
				}
				// Note: No else block here. Tests without composite moments or with InsightsTopics
				// won't call ListMoments because resolveAggregateMoments returns early when momentNames is empty

				if tc.getCompositeMomentResp != nil && tc.getChildMomentResp != nil {
					gomock.InOrder(
						r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil),
						r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getChildMomentResp, nil))
				} else if tc.getCompositeMomentResp != nil {
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil)
				}
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
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))
			// No need to mock the second ES call when the first one returns no result.
			if tc.want != nil {
				if tc.esRespPath != "" {
					r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
				} else {
					r.mockESServer.Resps = append(r.mockESServer.Resps, emptyESHitsResponse)
				}
				r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("fetch_conversation_moment_annotation_request.json"))
			} else if tc.desc == "AggregateSubcategories" {
				// For aggregate test, also add expectations for moment annotation fetch
				if tc.esRespPath != "" {
					r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
				} else {
					r.mockESServer.Resps = append(r.mockESServer.Resps, emptyESHitsResponse)
				}
				r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("fetch_conversation_moment_annotation_request.json"))
			}
			resp, err := r.Client.RetrieveConversationMessages(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("RetrieveConversationMessages return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}
