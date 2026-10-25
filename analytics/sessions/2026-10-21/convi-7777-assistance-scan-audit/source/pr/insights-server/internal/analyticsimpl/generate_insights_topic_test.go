package analyticsimpl

import (
	"context"
	"fmt"
	"testing"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	labelpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	testingfake "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/testing"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
	"github.com/cresta/shared-go/fn"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/google/go-cmp/cmp"
)

type GenerateInsightsTopicTestSuite struct {
	AnalyticsV2TestSuite
}

func TestGenerateInsightsTopic_WithPrerequirementMoments(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	ts := GenerateInsightsTopicTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{momentClientRegistry: fakeMomentClientRegistry, configClient: ts.mockConfigClient, logger: logger.GetLogger()}

	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")
	var clusters []*momentpb.Moment
	for i := 1; i <= 2; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: &momentpb.Moment{
			Name: fmt.Sprintf("%s/moments/%s", parent, fmt.Sprintf("cluster-id-%d", i)),
			Type: momentpb.Moment_TOPIC,
		}})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		clusters = append(clusters, resp.Moment)
	}

	prerequirementMoments := []*momentpb.Moment{
		{
			Name: fmt.Sprintf("%s/%s", parent, "convo-reason-prerequirement-moment-id"),
		},
		{
			Name: fmt.Sprintf("%s/%s", parent, "visitor-question-prerequirement-moment-id"),
		},
	}
	fakeMomentClient.PopulatePrerequirementMoments(context.Background(), prerequirementMoments)

	insightsTopic := &analyticspb.InsightsTopic{
		TopicMoment: &momentpb.Moment{
			Type:     momentpb.Moment_COMPOSITE,
			AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Labels: &labelpb.Labels{
				Labels: map[string]string{
					"topic_types": "10, 11 ",
				},
			},
			Taxonomy:               "taxonomy1",
			DisplayName:            "display-name1",
			MomentSource:           momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState:          momentpb.Moment_STATE_ACTIVE,
			ParentMomentTemplateId: "parent-moment-id1",
		},
		IncludedKeywordMoment: &momentpb.Moment{
			Type:          momentpb.Moment_KEYWORD,
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Taxonomy:      "taxonomy1.1",
			DisplayName:   "display-name1",
			MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState: momentpb.Moment_STATE_ACTIVE,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: "non-flexible phrase",
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
														Phrase: "phrase",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		ExcludedKeywordMoment: &momentpb.Moment{
			Type:          momentpb.Moment_KEYWORD,
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Taxonomy:      "taxonomy1.2",
			DisplayName:   "display-name1",
			MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState: momentpb.Moment_STATE_ACTIVE,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: "non-flexible phrase",
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
														Phrase: "phrase",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		ClusterMoments: clusters,
	}
	// proto.Clone deep copy a proto struct.
	want := &analyticspb.InsightsTopic{
		TopicMoment:           proto.Clone(insightsTopic.TopicMoment).(*momentpb.Moment),
		IncludedKeywordMoment: proto.Clone(insightsTopic.IncludedKeywordMoment).(*momentpb.Moment),
		ExcludedKeywordMoment: proto.Clone(insightsTopic.ExcludedKeywordMoment).(*momentpb.Moment),
		ClusterMoments:        insightsTopic.ClusterMoments,
		PrerequirementMoments: prerequirementMoments,
	}
	// Taxonomy suffixes are appended by GenerateInsightsTopic
	want.IncludedKeywordMoment.Taxonomy = "taxonomy1.1.include"
	want.ExcludedKeywordMoment.Taxonomy = "taxonomy1.2.exclude"

	want.TopicMoment.MomentData = &momentpb.MomentData{
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
											BaseMoment: insightsTopic.ExcludedKeywordMoment.Name,
										},
									},
								},
							},
						},
					},
					{
						Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
							BaseMoment: prerequirementMoments[0].Name,
						},
					},
					{
						Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
							BaseMoment: prerequirementMoments[1].Name,
						},
					},
					{
						Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
							CompositeMoment: &momentpb.CompositeMomentData{
								Operator: momentpb.CompositeMomentData_OR,
								Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: insightsTopic.ClusterMoments[0].Name,
										},
									},
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: insightsTopic.ClusterMoments[1].Name,
										},
									},
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: insightsTopic.IncludedKeywordMoment.Name,
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

	cConfig := nonpubconfig.CustomerConfig{}
	ts.Require().NoError(protojson.Unmarshal([]byte(`{
		"customer": "customers/customer_id",
		"profileConfigs": [
			{
				"name": "customers/customer_id/profiles/profile_id",
				"features": {
					"insights": {
						"enableMomentPrerequirementsUsage": true
					}
				}
			}
		]
	}`), &cConfig))
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	ts.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)

	got, err := s.GenerateInsightsTopic(context.Background(), &analyticspb.GenerateInsightsTopicRequest{
		Parent:        parent,
		InsightsTopic: insightsTopic,
	})
	if err != nil {
		t.Fatalf("GenerateInsightsTopic() failed: %v", err)
	}
	if diff := cmp.Diff(want, got.InsightsTopic, protocmp.Transform(), protocmp.IgnoreFields(&momentpb.Moment{}, "create_time", "update_time", "name"), protocmp.IgnoreFields(&momentpb.CompositeMomentData_CompositeMomentOperand{}, "base_moment")); diff != "" {
		t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
	}
}

func TestGenerateInsightsTopic_WithoutPrerequirementMoments(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)

	ts := GenerateInsightsTopicTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{momentClientRegistry: fakeMomentClientRegistry, configClient: ts.mockConfigClient, logger: logger.GetLogger()}

	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")
	var clusters []*momentpb.Moment
	for i := 1; i <= 2; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: &momentpb.Moment{
			Name: fmt.Sprintf("%s/moments/%s", parent, fmt.Sprintf("cluster-id-%d", i)),
			Type: momentpb.Moment_TOPIC,
		}})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		clusters = append(clusters, resp.Moment)
	}

	insightsTopic := &analyticspb.InsightsTopic{
		TopicMoment: &momentpb.Moment{
			Type:     momentpb.Moment_COMPOSITE,
			AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Labels: &labelpb.Labels{
				Labels: map[string]string{
					"topic_types": "10, 11 ",
				},
			},
			Taxonomy:               "taxonomy1",
			DisplayName:            "display-name1",
			MomentSource:           momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState:          momentpb.Moment_STATE_ACTIVE,
			ParentMomentTemplateId: "parent-moment-id1",
		},
		IncludedKeywordMoment: &momentpb.Moment{
			Type:          momentpb.Moment_KEYWORD,
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Taxonomy:      "taxonomy1.1",
			DisplayName:   "display-name1",
			MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState: momentpb.Moment_STATE_ACTIVE,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: "non-flexible phrase",
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
														Phrase: "phrase",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		ExcludedKeywordMoment: &momentpb.Moment{
			Type:          momentpb.Moment_KEYWORD,
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Taxonomy:      "taxonomy1.2",
			DisplayName:   "display-name1",
			MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState: momentpb.Moment_STATE_ACTIVE,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: "non-flexible phrase",
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
														Phrase: "phrase",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		ClusterMoments: clusters,
	}
	// proto.Clone deep copy a proto struct.
	want := &analyticspb.InsightsTopic{
		TopicMoment:           proto.Clone(insightsTopic.TopicMoment).(*momentpb.Moment),
		IncludedKeywordMoment: proto.Clone(insightsTopic.IncludedKeywordMoment).(*momentpb.Moment),
		ExcludedKeywordMoment: proto.Clone(insightsTopic.ExcludedKeywordMoment).(*momentpb.Moment),
		ClusterMoments:        insightsTopic.ClusterMoments,
	}
	// Taxonomy suffixes are appended by GenerateInsightsTopic
	want.IncludedKeywordMoment.Taxonomy = "taxonomy1.1.include"
	want.ExcludedKeywordMoment.Taxonomy = "taxonomy1.2.exclude"

	want.TopicMoment.MomentData = &momentpb.MomentData{
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
											BaseMoment: insightsTopic.ExcludedKeywordMoment.Name,
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
											BaseMoment: insightsTopic.ClusterMoments[0].Name,
										},
									},
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: insightsTopic.ClusterMoments[1].Name,
										},
									},
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: insightsTopic.IncludedKeywordMoment.Name,
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

	cConfig := nonpubconfig.CustomerConfig{}
	ts.Require().NoError(protojson.Unmarshal([]byte(`{
		"customer": "customers/customer_id",
		"profileConfigs": [
			{
				"name": "customers/customer_id/profiles/profile_id",
				"features": {
					"insights": {
					}
				}
			}
		]
	}`), &cConfig))
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	ts.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)

	got, err := s.GenerateInsightsTopic(context.Background(), &analyticspb.GenerateInsightsTopicRequest{
		Parent:        parent,
		InsightsTopic: insightsTopic,
	})
	if err != nil {
		t.Fatalf("GenerateInsightsTopic() failed: %v", err)
	}
	if diff := cmp.Diff(want, got.InsightsTopic, protocmp.Transform(), protocmp.IgnoreFields(&momentpb.Moment{}, "create_time", "update_time", "name"), protocmp.IgnoreFields(&momentpb.CompositeMomentData_CompositeMomentOperand{}, "base_moment")); diff != "" {
		t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
	}
}

func TestGenerateInsightsTopic_InheritParentAttributes(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)

	ts := GenerateInsightsTopicTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{momentClientRegistry: fakeMomentClientRegistry, configClient: ts.mockConfigClient, logger: logger.GetLogger()}

	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")

	// Create parent included keyword moment
	parentIncludedKeywordMoment := &momentpb.Moment{
		Name: fmt.Sprintf("%s/moments/%s", parent, "parent-included-keyword-moment-id"),
		Type: momentpb.Moment_KEYWORD,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_OR,
						Operands: []*momentpb.KeywordMomentData_Operand{
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "parent included phrase",
								},
							},
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "shared included phrase", // This will be in both parent and child
								},
							},
							{
								Operand: &momentpb.KeywordMomentData_Operand_Expression{
									Expression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_SEQUENCE,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "parent",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "sequence",
												},
											},
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
	_, err := fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: parentIncludedKeywordMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for parent included keyword: %v", err)
	}

	// Create parent excluded keyword moment
	parentExcludedKeywordMoment := &momentpb.Moment{
		Name: fmt.Sprintf("%s/moments/%s", parent, "parent-excluded-keyword-moment-id"),
		Type: momentpb.Moment_KEYWORD,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_OR,
						Operands: []*momentpb.KeywordMomentData_Operand{
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "parent excluded phrase",
								},
							},
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "shared excluded phrase", // This will be in both parent and child
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: parentExcludedKeywordMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for parent excluded keyword: %v", err)
	}

	// Create parent composite moment first (we'll need its name for pinning)
	parentCompositeMoment := &momentpb.Moment{
		Name: fmt.Sprintf("%s/moments/%s", parent, "parent-composite-moment-id"),
		Type: momentpb.Moment_COMPOSITE,
	}

	// Create parent cluster moments
	var parentClusters []*momentpb.Moment
	for i := 1; i <= 2; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
			Parent: parent,
			Moment: &momentpb.Moment{
				Name: fmt.Sprintf("%s/moments/%s", parent, fmt.Sprintf("parent-cluster-id-%d", i)),
				Type: momentpb.Moment_TOPIC,
				Labels: &labelpb.Labels{
					Labels: map[string]string{
						"pinned_to": parentCompositeMoment.Name,
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		parentClusters = append(parentClusters, resp.Moment)
	}

	// Now set the parent composite moment data and create it
	parentCompositeMoment.MomentData = &momentpb.MomentData{
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
											BaseMoment: parentExcludedKeywordMoment.Name,
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
											BaseMoment: parentClusters[0].Name,
										},
									},
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: parentClusters[1].Name,
										},
									},
									{
										Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
											BaseMoment: parentIncludedKeywordMoment.Name,
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
	_, err = fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: parentCompositeMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for parent composite: %v", err)
	}

	// Create new child clusters
	var childClusters []*momentpb.Moment
	for i := 1; i <= 2; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
			Parent: parent,
			Moment: &momentpb.Moment{
				Name: fmt.Sprintf("%s/moments/%s", parent, fmt.Sprintf("child-cluster-id-%d", i)),
				Type: momentpb.Moment_TOPIC,
			},
		})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		childClusters = append(childClusters, resp.Moment)
	}

	// Create child keyword moments
	childIncludedKeywordMoment := &momentpb.Moment{
		Name:          fmt.Sprintf("%s/moments/%s", parent, "child-included-keyword-moment-id"),
		Type:          momentpb.Moment_KEYWORD,
		AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
		MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
		TaxonomyState: momentpb.Moment_STATE_ACTIVE,
		Taxonomy:      "child-taxonomy1.1",
		DisplayName:   "child-display-name1",
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_OR,
						Operands: []*momentpb.KeywordMomentData_Operand{
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "child included phrase",
								},
							},
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "shared included phrase", // Duplicate from parent - should be deduplicated
								},
							},
							{
								Operand: &momentpb.KeywordMomentData_Operand_Expression{
									Expression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_SEQUENCE,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "parent",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "sequence",
												},
											},
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
	_, err = fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: childIncludedKeywordMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for child included keyword: %v", err)
	}

	childExcludedKeywordMoment := &momentpb.Moment{
		Name:          fmt.Sprintf("%s/moments/%s", parent, "child-excluded-keyword-moment-id"),
		Type:          momentpb.Moment_KEYWORD,
		AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
		MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
		TaxonomyState: momentpb.Moment_STATE_ACTIVE,
		Taxonomy:      "child-taxonomy1.2",
		DisplayName:   "child-display-name1",
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_OR,
						Operands: []*momentpb.KeywordMomentData_Operand{
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "child excluded phrase",
								},
							},
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "shared excluded phrase", // Duplicate from parent - should be deduplicated
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: childExcludedKeywordMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for child excluded keyword: %v", err)
	}

	prerequirementMoments := []*momentpb.Moment{
		{
			Name: fmt.Sprintf("%s/%s", parent, "convo-reason-prerequirement-moment-id"),
		},
	}
	fakeMomentClient.PopulatePrerequirementMoments(context.Background(), prerequirementMoments)

	insightsTopic := &analyticspb.InsightsTopic{
		TopicMoment: &momentpb.Moment{
			Type:     momentpb.Moment_COMPOSITE,
			AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Labels: &labelpb.Labels{
				Labels: map[string]string{
					"topic_types": "10",
				},
			},
			Taxonomy:               "child-taxonomy1",
			DisplayName:            "child-display-name1",
			MomentSource:           momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState:          momentpb.Moment_STATE_ACTIVE,
			ParentMomentTemplateId: parentCompositeMoment.Name,
		},
		IncludedKeywordMoment: childIncludedKeywordMoment,
		ExcludedKeywordMoment: childExcludedKeywordMoment,
		ClusterMoments:        childClusters,
	}

	cConfig := nonpubconfig.CustomerConfig{}
	ts.Require().NoError(protojson.Unmarshal([]byte(`{
		"customer": "customers/customer_id",
		"profileConfigs": [
			{
				"name": "customers/customer_id/profiles/profile_id",
				"features": {
					"insights": {
					}
				}
			}
		]
	}`), &cConfig))
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	ts.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)

	// Call GenerateInsightsTopic with InheritParentAttributes set to true
	got, err := s.GenerateInsightsTopic(context.Background(), &analyticspb.GenerateInsightsTopicRequest{
		Parent:                  parent,
		InsightsTopic:           insightsTopic,
		InheritParentAttributes: true,
	})
	if err != nil {
		t.Fatalf("GenerateInsightsTopic() failed: %v", err)
	}

	// Validate that keyword expressions are merged correctly with deduplication
	if got.InsightsTopic.IncludedKeywordMoment == nil {
		t.Fatal("Expected included keyword moment to be created")
	}
	includedKeywordData := got.InsightsTopic.IncludedKeywordMoment.GetMomentData().GetKeywordMomentData()
	if includedKeywordData == nil || includedKeywordData.KeywordExpression == nil {
		t.Fatal("Expected included keyword expression to be set")
	}
	// Parent has 3 operands (parent phrase, shared phrase, parent sequence)
	// Child has 3 operands (child phrase, shared phrase, parent sequence) - 2 duplicates with parent
	// After deduplication: should have 4 unique operands
	if len(includedKeywordData.KeywordExpression.Operands) != 4 {
		t.Errorf("Expected 4 unique operands in merged included keyword expression after deduplication, got %d", len(includedKeywordData.KeywordExpression.Operands))
	}

	// Verify that specific phrases are present
	foundParentPhrase := false
	foundChildPhrase := false
	foundSharedPhrase := false
	foundSequence := false
	for _, operand := range includedKeywordData.KeywordExpression.Operands {
		if operand.GetPhrase() == "parent included phrase" {
			foundParentPhrase = true
		}
		if operand.GetPhrase() == "child included phrase" {
			foundChildPhrase = true
		}
		if operand.GetPhrase() == "shared included phrase" {
			foundSharedPhrase = true
		}
		if operand.GetExpression() != nil && operand.GetExpression().Operator == momentpb.KeywordMomentData_SEQUENCE {
			foundSequence = true
		}
	}
	if !foundParentPhrase {
		t.Error("Expected to find 'parent included phrase' in merged expression")
	}
	if !foundChildPhrase {
		t.Error("Expected to find 'child included phrase' in merged expression")
	}
	if !foundSharedPhrase {
		t.Error("Expected to find 'shared included phrase' in merged expression")
	}
	if !foundSequence {
		t.Error("Expected to find sequence expression in merged expression")
	}

	if got.InsightsTopic.ExcludedKeywordMoment == nil {
		t.Fatal("Expected excluded keyword moment to be created")
	}
	excludedKeywordData := got.InsightsTopic.ExcludedKeywordMoment.GetMomentData().GetKeywordMomentData()
	if excludedKeywordData == nil || excludedKeywordData.KeywordExpression == nil {
		t.Fatal("Expected excluded keyword expression to be set")
	}
	// Parent has 2 operands (parent phrase, shared phrase)
	// Child has 2 operands (child phrase, shared phrase) - 1 duplicate with parent
	// After deduplication: should have 3 unique operands
	if len(excludedKeywordData.KeywordExpression.Operands) != 3 {
		t.Errorf("Expected 3 unique operands in merged excluded keyword expression after deduplication, got %d", len(excludedKeywordData.KeywordExpression.Operands))
	}

	// Verify that specific phrases are present
	foundParentExcluded := false
	foundChildExcluded := false
	foundSharedExcluded := false
	for _, operand := range excludedKeywordData.KeywordExpression.Operands {
		if operand.GetPhrase() == "parent excluded phrase" {
			foundParentExcluded = true
		}
		if operand.GetPhrase() == "child excluded phrase" {
			foundChildExcluded = true
		}
		if operand.GetPhrase() == "shared excluded phrase" {
			foundSharedExcluded = true
		}
	}
	if !foundParentExcluded {
		t.Error("Expected to find 'parent excluded phrase' in merged expression")
	}
	if !foundChildExcluded {
		t.Error("Expected to find 'child excluded phrase' in merged expression")
	}
	if !foundSharedExcluded {
		t.Error("Expected to find 'shared excluded phrase' in merged expression (exactly once after deduplication)")
	}

	// Validate that old keyword moments are deprecated (by checking BatchDeprecateMoments was called)
	expectedDeprecatedMoments := []string{
		parentIncludedKeywordMoment.Name,
		parentExcludedKeywordMoment.Name,
	}

	if len(fakeMomentClient.DeprecatedMoments) != len(expectedDeprecatedMoments) {
		t.Errorf("Expected %d moments to be deprecated, got %d", len(expectedDeprecatedMoments), len(fakeMomentClient.DeprecatedMoments))
	}

	deprecatedMap := make(map[string]bool)
	for _, name := range fakeMomentClient.DeprecatedMoments {
		deprecatedMap[name] = true
	}

	for _, expectedName := range expectedDeprecatedMoments {
		if !deprecatedMap[expectedName] {
			t.Errorf("Expected moment %s to be deprecated, but BatchDeprecateMoments was not called for it", expectedName)
		}
	}

	// Validate that parent clusters were unpinned (by checking UpdateMoment was called to remove pinned_to)
	// Create a map of updated moments by name for easy lookup
	updatedMomentsMap := make(map[string][]*momentpb.Moment)
	for _, updated := range fakeMomentClient.UpdatedMoments {
		updatedMomentsMap[updated.Name] = append(updatedMomentsMap[updated.Name], updated)
	}

	// Check that parent clusters were updated to remove pinning from parent composite moment
	for _, parentCluster := range parentClusters {
		updates, wasUpdated := updatedMomentsMap[parentCluster.Name]
		if !wasUpdated {
			t.Errorf("Expected parent cluster %s to be updated (unpinned), but UpdateMoment was not called", parentCluster.Name)
			continue
		}

		// Find an update where the pinned_to was modified to remove parent composite moment
		foundUnpin := false
		for _, update := range updates {
			if update.Labels != nil && update.Labels.Labels != nil {
				pinnedTo := update.Labels.Labels["pinned_to"]
				// After unpinning from parent, it should either be empty or not contain parent composite moment
				if pinnedTo != parentCompositeMoment.Name {
					foundUnpin = true
					break
				}
			}
		}
		if !foundUnpin {
			t.Errorf("Expected parent cluster %s to be unpinned from parent composite moment %s", parentCluster.Name, parentCompositeMoment.Name)
		}
	}

	// Validate that new clusters (merged parent + child) are pinned
	expectedClusters := make([]*momentpb.Moment, 0, len(parentClusters)+len(childClusters))
	expectedClusters = append(expectedClusters, parentClusters...)
	expectedClusters = append(expectedClusters, childClusters...)
	if len(got.InsightsTopic.ClusterMoments) != len(expectedClusters) {
		t.Errorf("Expected %d cluster moments, got %d", len(expectedClusters), len(got.InsightsTopic.ClusterMoments))
	}

	// All clusters (parent + child) should be pinned to the new topic (by checking UpdateMoment was called with pinned_to set)
	for _, cluster := range got.InsightsTopic.ClusterMoments {
		updates, wasUpdated := updatedMomentsMap[cluster.Name]
		if !wasUpdated {
			t.Errorf("Expected cluster %s to be updated (pinned), but UpdateMoment was not called", cluster.Name)
			continue
		}

		// Find an update where pinned_to contains the new topic
		foundPin := false
		for _, update := range updates {
			if update.Labels != nil && update.Labels.Labels != nil {
				pinnedTo := update.Labels.Labels["pinned_to"]
				// Should be pinned to the new topic moment
				if pinnedTo == got.InsightsTopic.TopicMoment.Name {
					foundPin = true
					break
				}
			}
		}
		if !foundPin {
			t.Errorf("Expected cluster %s to be pinned to new topic %s", cluster.Name, got.InsightsTopic.TopicMoment.Name)
		}
	}

	// Validate that the topic composite moment includes all expected operands
	topicMomentData := got.InsightsTopic.TopicMoment.GetMomentData().GetCompositeMomentData()
	if topicMomentData == nil {
		t.Fatal("Expected topic moment data to be set")
	}
	if topicMomentData.Operator != momentpb.CompositeMomentData_AND {
		t.Errorf("Expected topic operator to be AND, got %v", topicMomentData.Operator)
	}

	// Should have: excluded moment (NOT) and OR composite with clusters + included keyword
	// Expected structure: AND(NOT(excluded), OR(parent-cluster-1, parent-cluster-2, child-cluster-1, child-cluster-2, included))
	// That's 2 operands at the top level: NOT, OR
	if len(topicMomentData.Operands) != 2 {
		t.Errorf("Expected 2 operands in topic moment data (NOT excluded, OR clusters+included), got %d", len(topicMomentData.Operands))
	}
}

func TestGenerateInsightsTopic_InheritParentAttributes_MomentDataIncompatibleWithChild(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	ts := GenerateInsightsTopicTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{momentClientRegistry: fakeMomentClientRegistry, configClient: ts.mockConfigClient, logger: logger.GetLogger()}

	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")

	// Create parent included keyword moment with OR operator
	parentIncludedKeywordMoment := &momentpb.Moment{
		Name: fmt.Sprintf("%s/moments/%s", parent, "parent-included-keyword-moment-id"),
		Type: momentpb.Moment_KEYWORD,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_OR,
						Operands: []*momentpb.KeywordMomentData_Operand{
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "parent phrase",
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: parentIncludedKeywordMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for parent included keyword: %v", err)
	}

	// Create parent composite moment
	parentCompositeMoment := &momentpb.Moment{
		Name: fmt.Sprintf("%s/moments/%s", parent, "parent-composite-moment-id"),
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
												BaseMoment: parentIncludedKeywordMoment.Name,
											},
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
	_, err = fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: parentCompositeMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for parent composite: %v", err)
	}

	// Create child included keyword moment with AND operator (incompatible with parent's OR)
	childIncludedKeywordMoment := &momentpb.Moment{
		Name:          fmt.Sprintf("%s/moments/%s", parent, "child-included-keyword-moment-id"),
		Type:          momentpb.Moment_KEYWORD,
		AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
		MomentSource:  momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
		TaxonomyState: momentpb.Moment_STATE_ACTIVE,
		Taxonomy:      "child-taxonomy1.1",
		DisplayName:   "child-display-name1",
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_AND, // Different operator from parent's OR
						Operands: []*momentpb.KeywordMomentData_Operand{
							{
								Operand: &momentpb.KeywordMomentData_Operand_Phrase{
									Phrase: "child phrase",
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = fakeMomentClient.CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
		Parent: parent,
		Moment: childIncludedKeywordMoment,
	})
	if err != nil {
		t.Fatalf("CreateMoment() for child included keyword: %v", err)
	}

	// Create new child clusters
	var childClusters []*momentpb.Moment
	for i := 1; i <= 2; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{
			Parent: parent,
			Moment: &momentpb.Moment{
				Name: fmt.Sprintf("%s/moments/%s", parent, fmt.Sprintf("child-cluster-id-%d", i)),
				Type: momentpb.Moment_TOPIC,
			},
		})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		childClusters = append(childClusters, resp.Moment)
	}

	insightsTopic := &analyticspb.InsightsTopic{
		TopicMoment: &momentpb.Moment{
			Type:     momentpb.Moment_COMPOSITE,
			AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			Labels: &labelpb.Labels{
				Labels: map[string]string{
					"topic_types": "10",
				},
			},
			Taxonomy:               "child-taxonomy1",
			DisplayName:            "child-display-name1",
			MomentSource:           momentpb.MomentSource_MOMENT_SOURCE_CUSTOM,
			TaxonomyState:          momentpb.Moment_STATE_ACTIVE,
			ParentMomentTemplateId: parentCompositeMoment.Name,
		},
		IncludedKeywordMoment: childIncludedKeywordMoment,
		ClusterMoments:        childClusters,
	}

	cConfig := nonpubconfig.CustomerConfig{}
	ts.Require().NoError(protojson.Unmarshal([]byte(`{
		"customer": "customers/customer_id",
		"profileConfigs": [
			{
				"name": "customers/customer_id/profiles/profile_id",
				"features": {
					"insights": {
					}
				}
			}
		]
	}`), &cConfig))
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	ts.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)

	// Call GenerateInsightsTopic with InheritParentAttributes set to true
	got, err := s.GenerateInsightsTopic(context.Background(), &analyticspb.GenerateInsightsTopicRequest{
		Parent:                  parent,
		InsightsTopic:           insightsTopic,
		InheritParentAttributes: true,
	})
	if err != nil {
		t.Fatalf("GenerateInsightsTopic() failed: %v", err)
	}

	// Validate that keyword expressions are NOT merged due to incompatibility
	if got.InsightsTopic.IncludedKeywordMoment == nil {
		t.Fatal("Expected included keyword moment to be created")
	}
	includedKeywordData := got.InsightsTopic.IncludedKeywordMoment.GetMomentData().GetKeywordMomentData()
	if includedKeywordData == nil || includedKeywordData.KeywordExpression == nil {
		t.Fatal("Expected included keyword expression to be set")
	}

	// Should only have the child's operand (not merged with parent due to incompatibility)
	if len(includedKeywordData.KeywordExpression.Operands) != 1 {
		t.Errorf("Expected 1 operand in child keyword expression (not merged with parent), got %d", len(includedKeywordData.KeywordExpression.Operands))
	}

	// Verify that only child phrase is present (parent phrase should not be merged)
	foundChildPhrase := false
	foundParentPhrase := false
	for _, operand := range includedKeywordData.KeywordExpression.Operands {
		if operand.GetPhrase() == "child phrase" {
			foundChildPhrase = true
		}
		if operand.GetPhrase() == "parent phrase" {
			foundParentPhrase = true
		}
	}
	if !foundChildPhrase {
		t.Error("Expected to find 'child phrase' in expression")
	}
	if foundParentPhrase {
		t.Error("Did not expect to find 'parent phrase' - parent and child should not be merged due to incompatible operators")
	}

	// Verify the operator is AND (child's operator, not merged)
	if includedKeywordData.KeywordExpression.Operator != momentpb.KeywordMomentData_AND {
		t.Errorf("Expected operator to be AND (child's operator), got %v", includedKeywordData.KeywordExpression.Operator)
	}

	// Validate that child keyword moment was NOT deprecated (because it wasn't merged)
	if len(fakeMomentClient.DeprecatedMoments) > 0 {
		t.Errorf("Expected no moments to be deprecated when parent and child are incompatible, got %d deprecated moments: %v", len(fakeMomentClient.DeprecatedMoments), fakeMomentClient.DeprecatedMoments)
	}

	// Validate that the topic composite moment was created successfully
	topicMomentData := got.InsightsTopic.TopicMoment.GetMomentData().GetCompositeMomentData()
	if topicMomentData == nil {
		t.Fatal("Expected topic moment data to be set")
	}
	if topicMomentData.Operator != momentpb.CompositeMomentData_AND {
		t.Errorf("Expected topic operator to be AND, got %v", topicMomentData.Operator)
	}
}
