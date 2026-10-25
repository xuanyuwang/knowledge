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

type BatchDeleteInsightsTopicsTestSuite struct {
	AnalyticsV2TestSuite
}

func TestBatchDeleteInsightsTopics(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	ts := BatchDeleteInsightsTopicsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{
		logger:               logger.GetLogger(),
		momentClientRegistry: fakeMomentClientRegistry,
		configClient:         ts.mockConfigClient,
	}
	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")
	// Insert some cluster moments.
	clusters := []*momentpb.Moment{}
	for i := 1; i <= 10; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: &momentpb.Moment{
			Name: fmt.Sprintf("%s/moments/%s", parent, "cluster-id1"),
			Type: momentpb.Moment_TOPIC,
		}})
		if err != nil {
			t.Fatalf("CreateMoment(): %v", err)
		}
		clusters = append(clusters, resp.Moment)
	}

	// Insert some folder moments.
	folders := []*momentpb.Moment{}
	for i := 1; i <= 2; i++ {
		resp, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: &momentpb.Moment{
			Name: fmt.Sprintf("%s/moments/folder-%d", parent, i),
			Type: momentpb.Moment_COMPOSITE,
		}})
		if err != nil {
			t.Fatalf("CreateMoment() for folder: %v", err)
		}
		folders = append(folders, resp.Moment)
	}

	insightTopic := &analyticspb.InsightsTopic{
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

	cConfig := configpb.CustomerConfig{}
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

	genResp, err := s.GenerateInsightsTopic(context.Background(), &analyticspb.GenerateInsightsTopicRequest{
		Parent:        parent,
		InsightsTopic: insightTopic,
	})
	if err != nil {
		t.Fatalf("GenerateInsightsTopic() failed: %v", err)
	}
	want := proto.Clone(genResp.InsightsTopic).(*analyticspb.InsightsTopic)
	want.TopicMoment.TaxonomyState = momentpb.Moment_STATE_DEPRECATED
	want.IncludedKeywordMoment.TaxonomyState = momentpb.Moment_STATE_DEPRECATED
	want.ExcludedKeywordMoment.TaxonomyState = momentpb.Moment_STATE_DEPRECATED
	for _, cl := range want.ClusterMoments {
		cl.Labels.Labels["pinned_to"] = ""
	}

	folderNames := []string{}
	for _, f := range folders {
		folderNames = append(folderNames, f.Name)
	}

	got, err := s.BatchDeleteInsightsTopics(context.Background(), &analyticspb.BatchDeleteInsightsTopicsRequest{
		Parent:      parent,
		Names:       []string{want.TopicMoment.Name},
		FolderNames: folderNames,
	})
	if err != nil {
		t.Fatalf("BatchDeleteInsightsTopics() failed: %v", err)
	}

	if diff := cmp.Diff(want, got.InsightsTopics[0], protocmp.Transform(), protocmp.IgnoreFields(&momentpb.Moment{}, "create_time", "update_time")); diff != "" {
		t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
	}

	// Verify that folder moments are deprecated.
	for _, folderName := range folderNames {
		getMomentResp, err := s.momentClientRegistry.Client(context.Background()).GetMoment(context.Background(), &momentpb.GetMomentRequest{
			Name: folderName,
		})
		if err != nil {
			t.Fatalf("GetMoment(%s) failed: %v", folderName, err)
		}
		if getMomentResp.Moment.TaxonomyState != momentpb.Moment_STATE_DEPRECATED {
			t.Fatalf("Folder moment %s was not deprecated, got state: %v", folderName, getMomentResp.Moment.TaxonomyState)
		}
	}
}

func TestBatchDeleteInsightsTopics_EmptyNamesAndFolderNames(t *testing.T) {
	fakeMomentClient := &testingfake.FakeMomentServer{}
	fakeMomentClientRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	ts := BatchDeleteInsightsTopicsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	ts.SetupSuite()
	s := &AnalyticsServiceImpl{
		logger:               logger.GetLogger(),
		momentClientRegistry: fakeMomentClientRegistry,
		configClient:         ts.mockConfigClient,
	}
	parent := fmt.Sprintf("customers/%s/profiles/%s", "customer_id", "profile_id")

	_, err := s.BatchDeleteInsightsTopics(context.Background(), &analyticspb.BatchDeleteInsightsTopicsRequest{
		Parent:      parent,
		Names:       []string{},
		FolderNames: []string{},
	})
	if err == nil {
		t.Fatal("BatchDeleteInsightsTopics() expected error when both Names and FolderNames are empty, got nil")
	}
}
