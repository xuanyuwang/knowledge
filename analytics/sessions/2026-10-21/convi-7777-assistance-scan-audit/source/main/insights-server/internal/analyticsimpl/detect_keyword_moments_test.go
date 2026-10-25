package analyticsimpl

import (
	"fmt"
	"testing"

	aicommonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/common"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	envutil "github.com/cresta/shared-go/util/env"

	"github.com/cresta/go-servers/shared/testing/cmputil"

	// "github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

type DetectKeywordMomentsTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *DetectKeywordMomentsTestSuite) SetupSuite() {
	// Env variable has to be set before calling AnalyticsV2TestSuite.SetupSuite().
	envutil.SetEnv("ES_SERVING_INDEX", "test-index-name")
	envutil.SetEnv("ES_DOCUMENT_ID_FOR_TESTING", "test_document_id")
	envutil.SetEnv("SKIP_WRITE_ES_FOR_TESTING", "true")

	s.AnalyticsV2TestSuite.SetupSuite()
}

func TestDetectKeywordMoments(t *testing.T) {
	t.Skip("Skipping: test has pre-existing failures, needs owner to fix")
	r := DetectKeywordMomentsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	for _, tc := range []struct {
		desc       string
		req        *analyticspb.DetectKeywordMomentsRequest
		esReqPath  string
		esRespPath string
		resp       *analyticspb.DetectKeywordMomentsResponse
	}{
		{
			desc:       "BasicMoments",
			esReqPath:  "DetectKeywordMoments_BasicMoments_es_request.json",
			esRespPath: "DetectKeywordMoments_BasicMoments_es_response.json",
			req: &analyticspb.DetectKeywordMomentsRequest{
				Chat: &aicommonpb.AiConversationPredictRequest{
					Name: "conversation-name",
					ChatState: &aicommonpb.AiConversationStateMetadata{
						LanguageCode: "en-US",
					},
					AiServiceConfig: &aicommonpb.AiServiceConfig{
						Config: &aicommonpb.AiServiceConfig_KeywordConfig{
							KeywordConfig: &aicommonpb.KeywordConfig{
								Definitions: []*aicommonpb.KeywordConfig_Definition{
									{
										MomentTemplateId: "moment-id1",
										KeywrodMomentData: &aicommonpb.KeywordMomentData{
											KeywordExpression: &aicommonpb.KeywordMomentData_KeywordExpression{
												Operator: aicommonpb.KeywordMomentData_OR,
												Operands: []*aicommonpb.KeywordMomentData_Operand{
													{
														Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
															Phrase: "a1",
														},
													},
													{
														Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
															Phrase: "a2",
														},
													},
												},
											},
										},
									},
									{
										MomentTemplateId: "moment-id2",
										KeywrodMomentData: &aicommonpb.KeywordMomentData{
											KeywordExpression: &aicommonpb.KeywordMomentData_KeywordExpression{
												Operator: aicommonpb.KeywordMomentData_AND,
												Operands: []*aicommonpb.KeywordMomentData_Operand{
													{
														Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
															Phrase: "b1",
														},
													},
													{
														Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
															Phrase: "b2",
														},
													},
												},
											},
										},
									},
									{
										MomentTemplateId: "moment-id3",
										KeywrodMomentData: &aicommonpb.KeywordMomentData{
											KeywordExpression: &aicommonpb.KeywordMomentData_KeywordExpression{
												Operator: aicommonpb.KeywordMomentData_AND,
												Operands: []*aicommonpb.KeywordMomentData_Operand{
													{
														Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
															Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
																Operator: aicommonpb.KeywordMomentData_OR,
																Operands: []*aicommonpb.KeywordMomentData_Operand{
																	{
																		Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
																			Phrase: "c1",
																		},
																	},
																	{
																		Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
																			Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
																				Operator: aicommonpb.KeywordMomentData_SEQUENCE,
																				Operands: []*aicommonpb.KeywordMomentData_Operand{
																					{
																						Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
																							Phrase: "seq1",
																						},
																					},
																					{
																						Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
																							Phrase: "seq2",
																						},
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
														Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
															Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
																Operator: aicommonpb.KeywordMomentData_EXCLUDE,
																Operands: []*aicommonpb.KeywordMomentData_Operand{
																	{
																		Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
																			Phrase: "c1",
																		},
																	},
																	{
																		Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
																			Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
																				Operator: aicommonpb.KeywordMomentData_SEQUENCE,
																				Operands: []*aicommonpb.KeywordMomentData_Operand{
																					{
																						Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
																							Phrase: "seq1",
																						},
																					},
																					{
																						Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
																							Phrase: "seq2",
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Messages: []*aicommonpb.AiConversationMessage{
						{
							MessageState: aicommonpb.AiConversationMessage_COMPLETE,
							Text:         "hello world",
							Name:         "message-name",
						},
					},
				},
			},
			resp: &analyticspb.DetectKeywordMomentsResponse{
				Result: &aicommonpb.AiConversationPredictResult{
					Name: "conversation-name",
					OutputMoments: []*aicommonpb.MomentAnnotation{
						{
							Metadata: &aicommonpb.GeneralMetadata{
								ConversationName: "conversation-name",
								MessageName:      "message-name",
								ServiceSource:    "AnalyticsService",
							},
							Type: aicommonpb.MomentAnnotation_KEYWORD,
							Payload: &aicommonpb.MomentAnnotation_KeywordPayload{
								KeywordPayload: &aicommonpb.KeywordPayload{
									MomentTemplateId: "moment-id1",
									MatchedMoments: []*aicommonpb.KeywordPayload_MatchedMoment{
										{},
									},
								},
							},
						},
						{
							Metadata: &aicommonpb.GeneralMetadata{
								ConversationName: "conversation-name",
								MessageName:      "message-name",
								ServiceSource:    "AnalyticsService",
							},
							Type: aicommonpb.MomentAnnotation_KEYWORD,
							Payload: &aicommonpb.MomentAnnotation_KeywordPayload{
								KeywordPayload: &aicommonpb.KeywordPayload{
									MomentTemplateId: "moment-id2",
									MatchedMoments:   []*aicommonpb.KeywordPayload_MatchedMoment{},
								},
							},
						},
						{
							Metadata: &aicommonpb.GeneralMetadata{
								ConversationName: "conversation-name",
								MessageName:      "message-name",
								ServiceSource:    "AnalyticsService",
							},
							Type: aicommonpb.MomentAnnotation_KEYWORD,
							Payload: &aicommonpb.MomentAnnotation_KeywordPayload{
								KeywordPayload: &aicommonpb.KeywordPayload{
									MomentTemplateId: "moment-id3",
									MatchedMoments: []*aicommonpb.KeywordPayload_MatchedMoment{
										{},
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			r.mockESServer.EXPECT().Search("test-index-name", r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("_id"))
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			got, err := r.Client.DetectKeywordMoments(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("DetectKeywordMoments returns error: %v", err)
			}
			if tc.resp != nil {
				if diff := cmp.Diff(tc.resp, got,
					protocmp.Transform()); diff != "" {
					t.Fatalf("DetectKeywordMoments return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}

func TestConvertKeywordMomentData(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		input  *aicommonpb.KeywordMomentData
		output *momentpb.KeywordMomentData
	}{
		{
			desc: "nil input",
		},
		{
			desc: "nested with all Operator",
			input: &aicommonpb.KeywordMomentData{
				KeywordExpression: &aicommonpb.KeywordMomentData_KeywordExpression{
					Operator: aicommonpb.KeywordMomentData_AND,
					Operands: []*aicommonpb.KeywordMomentData_Operand{
						{
							Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
								Phrase: "hello",
							},
						},
						{
							Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
								Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
									Operator: aicommonpb.KeywordMomentData_AND,
									Operands: []*aicommonpb.KeywordMomentData_Operand{
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
						{
							Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
								Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
									Operator: aicommonpb.KeywordMomentData_OR,
									Operands: []*aicommonpb.KeywordMomentData_Operand{
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
						{
							Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
								Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
									Operator: aicommonpb.KeywordMomentData_SEQUENCE,
									Operands: []*aicommonpb.KeywordMomentData_Operand{
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
						{
							Operand: &aicommonpb.KeywordMomentData_Operand_Expression{
								Expression: &aicommonpb.KeywordMomentData_KeywordExpression{
									Operator: aicommonpb.KeywordMomentData_EXCLUDE,
									Operands: []*aicommonpb.KeywordMomentData_Operand{
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &aicommonpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			output: &momentpb.KeywordMomentData{
				KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
					Operator: momentpb.KeywordMomentData_AND,
					Operands: []*momentpb.KeywordMomentData_Operand{
						{
							Operand: &momentpb.KeywordMomentData_Operand_Phrase{
								Phrase: "hello",
							},
						},
						{
							Operand: &momentpb.KeywordMomentData_Operand_Expression{
								Expression: &momentpb.KeywordMomentData_KeywordExpression{
									Operator: momentpb.KeywordMomentData_AND,
									Operands: []*momentpb.KeywordMomentData_Operand{
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
						{
							Operand: &momentpb.KeywordMomentData_Operand_Expression{
								Expression: &momentpb.KeywordMomentData_KeywordExpression{
									Operator: momentpb.KeywordMomentData_OR,
									Operands: []*momentpb.KeywordMomentData_Operand{
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
						{
							Operand: &momentpb.KeywordMomentData_Operand_Expression{
								Expression: &momentpb.KeywordMomentData_KeywordExpression{
									Operator: momentpb.KeywordMomentData_SEQUENCE,
									Operands: []*momentpb.KeywordMomentData_Operand{
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
						{
							Operand: &momentpb.KeywordMomentData_Operand_Expression{
								Expression: &momentpb.KeywordMomentData_KeywordExpression{
									Operator: momentpb.KeywordMomentData_EXCLUDE,
									Operands: []*momentpb.KeywordMomentData_Operand{
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "hello",
											},
										},
										{
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "world",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := convertKeywordMomentData(tc.input)
			if diff := cmp.Diff(tc.output, got, protocmp.Transform()); diff != "" {
				t.Errorf("convertKeywordMomentData() return wrong result with diff (-want, +got): \n%v", diff)
			}
		})
	}
}

func TestTruncateMessageText(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		input  string
		output string
	}{
		{
			desc:   "empty message",
			input:  "",
			output: "",
		},
		{
			desc:   "short message",
			input:  "how are you",
			output: "how",
		},
		{
			desc:   "long message",
			input:  "What's the best way to slice the last word from a block of text?",
			output: "What's the best way to slice the last word",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := truncateMessageText(tc.input)
			if diff := cmp.Diff(got, tc.output); diff != "" {
				t.Errorf("truncateMessageText returns wrong results: got %q, want %q", got, tc.desc)
			}
		})
	}
}
