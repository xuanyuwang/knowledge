package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
)

type RetrieveAssistanceStatsTestSuite struct {
	AnalyticsTestSuite
	janFirstPSTProto    timestamppb.Timestamp
	janEleventhPSTProto timestamppb.Timestamp
}

func (r *RetrieveAssistanceStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	pst, _ := time.LoadLocation(r.losAngeles)
	r.janFirstPSTProto = *timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, pst))
	r.janEleventhPSTProto = *timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, pst))
	r.UseMockedClickhouseConnections()
}

func (r *RetrieveAssistanceStatsTestSuite) AfterTest() {
	r.convoMockConn.AssertExpectationsAndReset(r.T())
}

func (r *RetrieveAssistanceStatsTestSuite) TestWithClickhouseRead() {
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveAssistanceStatsRequest
		chReqPath string
	}{
		{
			desc: "Basic",
			request: &analyticspb.RetrieveAssistanceStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/1"},
						{Name: "customers/customer_id/users/2"},
					},
					ConversationDurationRange: &analyticspb.DurationRange{
						MinDuration: &durationpb.Duration{Seconds: 10},
						MaxDuration: &durationpb.Duration{Seconds: 100},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAssistanceStats_Basic_request.sql",
		},
		{
			desc: "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveAssistanceStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstPSTProto,
					EndTimestamp:   &r.janEleventhPSTProto,
				},
				FilterByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/test_customer/profiles/test_profile/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "include-value-1",
										},
									},
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/test_customer/profiles/test_profile/moments/moment_id_2",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAssistanceStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			mockRows := clickhousetesting.NewMockRows(nil)
			r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)

			_, err := r.server.RetrieveAssistanceStats(r.ctx, tc.request)
			r.Require().NoError(err)
			r.Require().Len(r.convoMockConn.Calls, 1, "expected to query clickhouse exactly once")
			r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
			gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			expectedCHQuery := r.getFile(tc.chReqPath)
			r.Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
		})
	}
}

func TestRetrieveAssistanceStats(t *testing.T) {
	suite.Run(t, &RetrieveAssistanceStatsTestSuite{})
}
