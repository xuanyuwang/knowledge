// nolint:dupl
package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"

	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/protobuf"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
)

type EvaluateAnalyticsChartCachingTestSuite struct {
	AnalyticsV2TestSuite
}

func TestEvaluateAnalyticsChart_Cache(t *testing.T) {
	r := EvaluateAnalyticsChartCachingTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	t.Setenv("ENABLE_CACHING_IN_EVALUATE_ANALYTICS_CHART", "true")
	r.SetupSuite()

	for _, tc := range []struct {
		desc       string
		esReqPath  string
		req        *analyticspb.EvaluateAnalyticsChartRequest
		resp       *analyticspb.EvaluateAnalyticsChartResponse
		isCacheHit bool
	}{
		{
			desc:      "cache_enabled",
			esReqPath: "EvaluateAnalyticsChartCaching_cache_enabled.json",
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
				ReadFromLiveQuery: false,
				WriteToCache:      true,
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
			isCacheHit: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			cConfig := nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					{
						Name: "customers/customer_id/profiles/profile_id",
					},
				},
				AuthConfig: &nonpubconfig.CustomerAuthConfig{
					EnableAdvancedDataAccessControl: false,
				},
			}
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(testCustomerConfig.ListProfiles()[0], nil)
			if tc.isCacheHit {
				cachedReq := &analyticspb.EvaluateAnalyticsChartCachedRequest{}
				err := protojson.Unmarshal([]byte(r.getFile(tc.esReqPath)), cachedReq)
				if err != nil {
					t.Fatalf("failed to unmarshal innerReq: %v", err)
				}
				cachedReq.ChartRequest.AnalyticsChart.RenderingConfig = nil
				hash, err := protobuf.HashProto(cachedReq)
				if err != nil {
					t.Fatalf("failed to hash proto: %v", err)
				}
				requestBytes, err := protojson.Marshal(cachedReq)
				if err != nil {
					t.Fatalf("failed to marshal cachedReq: %v", err)
				}
				responseBytes, err := protojson.Marshal(tc.resp)
				if err != nil {
					t.Fatalf("failed to marshal tc.resp: %v", err)
				}
				r.Require().NoError(r.AppsDBRegistry().DB(r.ctx).Create(&dbmodel.CachedRequests{
					CustomerID:  "customer_id",
					ProfileID:   "profile_id",
					RequestName: "cresta.v1.analytics.EvaluateAnalyticsChartCachedRequest",
					CacheHash:   hash,
					CreatedAt:   time.Now().UTC(),
					Request:     string(requestBytes),
					Response:    string(responseBytes),
				}).Error)
				r.ctx = metadata.NewOutgoingContext(
					r.ctx,
					metadata.Pairs("cresta-access-token-claim", `{"User":{"name":"customers/customer_id/users/testuser", "userRoles":["ADMIN"]}}`))
			}

			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.resp != nil {
				if diff := cmp.Diff(tc.resp, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("EvaluateAnalyticsChart return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}
