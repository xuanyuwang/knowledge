package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestEvaluateAnalyticsChart_EmailAttachments verifies that a ChartDataFilter carrying
// the EmailAttachmentsValue value-set is translated into an Elasticsearch terms clause on
// the denormalized has_email_attachments flag, for both HAS_EMAIL_ATTACHMENTS and
// HAS_NO_EMAIL_ATTACHMENTS. EMAIL_ATTACHMENTS_UNSPECIFIED omits the clause (covered by the
// "no value ignores the filter" contract).
func TestEvaluateAnalyticsChart_EmailAttachments(t *testing.T) {
	r := EvaluateAnalyticsChartTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc      string
		esReqPath string
		req       *analyticspb.EvaluateAnalyticsChartRequest
	}{
		{
			desc:      "email_attachments_has_attachments_as_filter",
			esReqPath: "EvaluateAnalyticsChart_HasEmailAttachments_es_request.json",
			req:       newEmailAttachmentsChartReq(r.profileResourceName, dashboardpb.EmailAttachments_HAS_EMAIL_ATTACHMENTS),
		},
		{
			desc:      "email_attachments_has_no_attachments_as_filter",
			esReqPath: "EvaluateAnalyticsChart_HasNoEmailAttachments_es_request.json",
			req:       newEmailAttachmentsChartReq(r.profileResourceName, dashboardpb.EmailAttachments_HAS_NO_EMAIL_ATTACHMENTS),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			// Reuse the by-month aggregation response from the scorecard-template filter test;
			// this test only asserts the ES request shape (the response is not compared).
			r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile("EvaluateAnalyticsChart_scorecardTemplate_as_filter_es_response.json"))
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&nonpubconfig.CustomerConfig{
				Customer: "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{
					{Name: "customers/customer_id/profiles/profile_id"},
				},
			}))
			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
				Return(testCustomerConfig.ListProfiles()[0], nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).AnyTimes().
				Return(testCustomerConfig, nil)
			if _, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req); err != nil {
				t.Fatalf("Got code %v in error: %v", status.Code(err), err)
			}
		})
	}
}

// newEmailAttachmentsChartReq builds a conversation-count-by-month chart with a fixed date
// range and an email-attachments filter, mirroring the scorecard-template filter test shape.
func newEmailAttachmentsChartReq(parent string, ea dashboardpb.EmailAttachments) *analyticspb.EvaluateAnalyticsChartRequest {
	return &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: parent,
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			Name:        "",
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
							Values: &dashboardpb.ChartValueSet_EmailAttachmentsValue{
								EmailAttachmentsValue: &dashboardpb.EmailAttachmentsValue{
									Value: ea,
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
	}
}
