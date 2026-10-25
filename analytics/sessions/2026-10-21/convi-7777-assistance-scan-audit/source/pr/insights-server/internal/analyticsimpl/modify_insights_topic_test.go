package analyticsimpl

import (
	"context"
	"fmt"
	"testing"

	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	testingfake "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/testing"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
	"github.com/cresta/shared-go/fn"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type ModifyInsightsTopicTestSuite struct {
	AnalyticsV2TestSuite
}

func TestModifyInsightsTopic(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	ts := ModifyInsightsTopicTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{
		logger:               logger.GetLogger(),
		momentClientRegistry: fakeMomentClientRegistry,
		configClient:         ts.mockConfigClient,
	}
	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")
	// Insert some cluster moments.
	var clusters []*momentpb.Moment
	for i := 1; i <= 10; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: &momentpb.Moment{
			Name: fmt.Sprintf("%s/moments/%s", parent, fmt.Sprintf("cluster-id-%d", i)),
			Type: momentpb.Moment_TOPIC,
		}})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		clusters = append(clusters, resp.Moment)
	}

	oldInsightTopic := &analyticspb.InsightsTopic{
		TopicMoment: &momentpb.Moment{
			Type:                   momentpb.Moment_COMPOSITE,
			AppTypes:               []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
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
		ClusterMoments: clusters[:2],
	}

	oldInsightTopicWithNoKeywordMoments := proto.Clone(oldInsightTopic).(*analyticspb.InsightsTopic)
	oldInsightTopicWithNoKeywordMoments.IncludedKeywordMoment, oldInsightTopicWithNoKeywordMoments.ExcludedKeywordMoment = nil, nil
	type updateParams struct {
		updateMomentIncluded bool
		updateMomentExcluded bool
		updateClusters       bool
		newMomentExcluded    *momentpb.Moment
		newMomentIncluded    *momentpb.Moment
		newClusters          []*momentpb.Moment
	}
	for _, tc := range []struct {
		desc         string
		old          *analyticspb.InsightsTopic
		updateParams updateParams
	}{
		{
			desc: "pin and unpin cluster",
			old:  oldInsightTopic,
			updateParams: updateParams{
				updateClusters: true,
				newClusters: []*momentpb.Moment{
					{
						Name: clusters[0].Name,
					},
					{
						Name: clusters[3].Name,
					},
				},
			},
		},
		{
			desc: "update included keywords",
			old:  oldInsightTopic,
			updateParams: updateParams{
				updateMomentIncluded: true,
				newMomentIncluded: &momentpb.Moment{
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
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "some new non-flexible phrase",
											},
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
			desc: "delete included keywords",
			old:  oldInsightTopic,
			updateParams: updateParams{
				updateMomentIncluded: true,
				newMomentIncluded:    nil,
			},
		},
		{
			desc: "create included keywords",
			old:  oldInsightTopicWithNoKeywordMoments,
			updateParams: updateParams{
				updateMomentIncluded: true,
				newMomentIncluded: &momentpb.Moment{
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
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "some new non-flexible phrase",
											},
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
			desc: "update excluded keywords",
			old:  oldInsightTopic,
			updateParams: updateParams{
				updateMomentExcluded: true,
				newMomentExcluded: &momentpb.Moment{
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
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "some new non-flexible phrase",
											},
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
			desc: "delete excluded keywords",
			old:  oldInsightTopic,
			updateParams: updateParams{
				updateMomentExcluded: true,
				newMomentExcluded:    nil,
			},
		},
		{
			desc: "create excluded keywords",
			old:  oldInsightTopicWithNoKeywordMoments,
			updateParams: updateParams{
				updateMomentExcluded: true,
				newMomentExcluded: &momentpb.Moment{
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
											Operand: &momentpb.KeywordMomentData_Operand_Phrase{
												Phrase: "some new non-flexible phrase",
											},
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
			cConfig := configpb.CustomerConfig{}
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
			resp, err := s.GenerateInsightsTopic(context.Background(), &analyticspb.GenerateInsightsTopicRequest{
				Parent:        parent,
				InsightsTopic: tc.old,
			})
			if err != nil {
				t.Fatalf("GenerateInsightsTopic() failed: %v", err)
			}
			want := resp.InsightsTopic
			if tc.updateParams.updateMomentIncluded {
				want.IncludedKeywordMoment = tc.updateParams.newMomentIncluded
			}
			if tc.updateParams.updateMomentExcluded {
				want.ExcludedKeywordMoment = tc.updateParams.newMomentExcluded
			}
			if tc.updateParams.updateClusters {
				want.ClusterMoments = tc.updateParams.newClusters
			}

			got, err := s.ModifyInsightsTopic(context.Background(), &analyticspb.ModifyInsightsTopicRequest{Parent: parent, InsightsTopic: want})
			if err != nil {
				t.Fatalf("ModifyInsightsTopic() failed: %v", err)
			}
			if diff := cmp.Diff(want, got.InsightsTopic, protocmp.Transform(), protocmp.IgnoreFields(&momentpb.Moment{}, "create_time", "update_time", "name"), protocmp.IgnoreFields(&momentpb.CompositeMomentData_CompositeMomentOperand{}, "base_moment")); diff != "" {
				t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
			}
		})
	}
}
