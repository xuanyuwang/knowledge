package analyticsimpl

import (
	"fmt"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhouseEventTesting "github.com/cresta/go-servers/shared/clickhouse/events/testing"
	"github.com/cresta/go-servers/shared/testing/assert"
)

type RetrieveManagerStatsTestSuite struct {
	AnalyticsTestSuite
	users *UsersData
	dates *TimePeriodData
}

func (r *RetrieveManagerStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.server.enableAgentCountFromCrestaConversation = true
	r.users = r.SetupUsersData()
	r.dates = r.SetupTimePeriodData()
	r.UseMockedClickhouseConnections()
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

// nolint:dupl
func (r *RetrieveManagerStatsTestSuite) TestCases() {
	for _, tc := range []struct {
		desc      string
		req       *analyticspb.RetrieveManagerStatsRequest
		want      *analyticspb.RetrieveManagerStatsResponse
		errorCode codes.Code
		chReqPath string
	}{
		{
			desc:      "TestEmptyRequest",
			req:       &analyticspb.RetrieveManagerStatsRequest{},
			errorCode: codes.InvalidArgument,
		},
		{
			desc: "TestRequest_InvalidTimeRange",
			req: &analyticspb.RetrieveManagerStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janTenthProto,
					EndTimestamp:   &r.dates.janFirstProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			errorCode: codes.InvalidArgument,
		},
		{
			desc: "TestRequest_InvalidTimeZone",
			req: &analyticspb.RetrieveManagerStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janTenthProto,
					EndTimestamp:   &r.dates.janFirstProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "UnknownTimeZone",
				},
			},
			errorCode: codes.InvalidArgument,
		},
		{
			desc: "TestRequest_Weekly_GroupByAgent",
			req: &analyticspb.RetrieveManagerStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janFirstProto,
					EndTimestamp:   &r.dates.janTenthProto,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			want: &analyticspb.RetrieveManagerStatsResponse{
				AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
					TotalDirectorPagesVisitCount:                            9,
					TotalDirectorLiveConversationPagesVisitCount:            5,
					TotalDirectorClosedConversationPagesVisitCount:          2,
					AverageDirectorPagesVisitCountPerUser:                   9.0 / 2,
					AverageDirectorLiveConversationPagesVisitCountPerUser:   5.0 / 2,
					AverageDirectorClosedConversationPagesVisitCountPerUser: 2.0 / 2,
				},
				ManagerStatsResults: []*analyticspb.ManagerStatsResult{
					{
						AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
							TotalDirectorPagesVisitCount:                            8,
							TotalDirectorLiveConversationPagesVisitCount:            5,
							TotalDirectorClosedConversationPagesVisitCount:          1,
							AverageDirectorPagesVisitCountPerUser:                   8.0,
							AverageDirectorLiveConversationPagesVisitCountPerUser:   5.0,
							AverageDirectorClosedConversationPagesVisitCountPerUser: 1.0,
						},
						ManagerStats: []*analyticspb.ManagerStats{
							{
								TimeRange: &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
									TotalDirectorPagesVisitCount:                            3,
									TotalDirectorLiveConversationPagesVisitCount:            3,
									TotalDirectorClosedConversationPagesVisitCount:          0,
									AverageDirectorPagesVisitCountPerUser:                   3.0,
									AverageDirectorLiveConversationPagesVisitCountPerUser:   3.0,
									AverageDirectorClosedConversationPagesVisitCountPerUser: 0,
								},
							},
							{
								TimeRange: &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
									TotalDirectorPagesVisitCount:                            5,
									TotalDirectorLiveConversationPagesVisitCount:            2,
									TotalDirectorClosedConversationPagesVisitCount:          1,
									AverageDirectorPagesVisitCountPerUser:                   5.0,
									AverageDirectorLiveConversationPagesVisitCountPerUser:   2.0,
									AverageDirectorClosedConversationPagesVisitCountPerUser: 1.0,
								},
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.users.agentUserA.UserID.String,
								}.String(),
								FullName: r.users.agentUserA.FullName.String,
								Username: r.users.agentUserA.Username,
							}},
						},
					},
					{
						AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
							TotalDirectorPagesVisitCount:                            1,
							TotalDirectorLiveConversationPagesVisitCount:            0,
							TotalDirectorClosedConversationPagesVisitCount:          1,
							AverageDirectorPagesVisitCountPerUser:                   1.0,
							AverageDirectorLiveConversationPagesVisitCountPerUser:   0.0,
							AverageDirectorClosedConversationPagesVisitCountPerUser: 1.0,
						},
						ManagerStats: []*analyticspb.ManagerStats{
							{
								TimeRange: &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								AggregatedManagerStats: &analyticspb.AggregatedManagerStats{
									TotalDirectorPagesVisitCount:                            1,
									TotalDirectorLiveConversationPagesVisitCount:            0,
									TotalDirectorClosedConversationPagesVisitCount:          1,
									AverageDirectorPagesVisitCountPerUser:                   1.0,
									AverageDirectorLiveConversationPagesVisitCountPerUser:   0.0,
									AverageDirectorClosedConversationPagesVisitCountPerUser: 1.0,
								},
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.users.agentUserB.UserID.String,
								}.String(),
								FullName: r.users.agentUserB.FullName.String,
								Username: r.users.agentUserB.Username,
							}},
						},
					},
				},
			},
			chReqPath: "clickhouse_RetrieveManagerStats_Weekly_GroupByAgent.sql",
		},
	} {
		r.Run(tc.desc, func() {
			// Prepare mocks.
			//
			// The session log events per user (manager) per day:
			//
			// |----------------+-----------+-----------+
			// |                |   01/01   |   01/07   |
			// |----------------|-----------|-----------|
			// | agentUserA (1) | (1,3,3,0) | (1,5,2,1) |
			// | agentUserB (2) | (0,0,0,0) | (1,1,0,1) |
			// |----------------+-----------+-----------+
			//
			// Where (a,b,c,d) represents
			// - a: count of user manager (only 0 or 1)
			// - b: count of director page vists (b >= c + d)
			// - c: count of director live convo page visits
			// - d: count of director closed convo page visits
			defer r.eventMockConn.AssertExpectationsAndReset(r.T())
			mockValues := [][]any{
				{r.dates.janFirst, "1", uint64(1), uint64(3), uint64(3), uint64(0)},
				{r.dates.janSeventh, "1", uint64(1), uint64(5), uint64(2), uint64(1)},
				{r.dates.janSeventh, "2", uint64(1), uint64(1), uint64(0), uint64(1)},
			}
			mockRows := clickhouseEventTesting.NewMockRows(mockValues)
			r.eventMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// Execute API.
			got, err := r.server.RetrieveManagerStats(r.ctx, tc.req)
			assert.ErrorCodeEquals(r.T(), err, tc.errorCode)
			if tc.errorCode != codes.OK {
				return
			}

			// Verify API resp.
			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				r.FailNow("Incorrect response with diff", "(-want +got):\n %v", diff)
			}

			// Verify clickhouse query.
			r.Require().Len(r.eventMockConn.Calls, 1, "expected to exactly query once")
			gotQuery := r.eventMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.eventMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			wantCHQuery := r.getFile(tc.chReqPath)
			diff := cmp.Diff(formatSQL(wantCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
			r.Require().Empty(diff, "Wrong ClickHouse query: got: \n%v\n want: \n%v\n", formatSQL(combineQueryAndArgs(gotQuery, gotArgs)), formatSQL(wantCHQuery))
		})
	}
}

func TestRetrieveManagerStats(t *testing.T) {
	suite.Run(t, &RetrieveManagerStatsTestSuite{})
}
