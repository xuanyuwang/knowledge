package analyticsimpl

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"

	clickhouseTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
)

type RetrieveScorecardStatsTestSuite struct {
	AnalyticsTestSuite
	agentUserA                  *dbmodel.Users
	agentUserB                  *dbmodel.Users
	janFirst                    time.Time
	janFirstProto               timestamppb.Timestamp
	sundayOfTheWeekOfJanFirst   timestamppb.Timestamp
	janSeventh                  time.Time
	janSeventhProto             timestamppb.Timestamp
	sundayOfTheWeekOfJanSeventh timestamppb.Timestamp
	janTenth                    time.Time
	janTenthProto               timestamppb.Timestamp
	janEleventhProto            timestamppb.Timestamp
}

func (r *RetrieveScorecardStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.janFirst = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	r.janFirstProto = *timestamppb.New(r.janFirst)
	r.sundayOfTheWeekOfJanFirst = *timestamppb.New(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC))
	r.janSeventh = time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)
	r.janSeventhProto = *timestamppb.New(r.janSeventh)
	r.sundayOfTheWeekOfJanSeventh = *timestamppb.New(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC))
	r.janTenth = time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	r.janTenthProto = *timestamppb.New(r.janTenth)
	r.janEleventhProto = *timestamppb.New(time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC))
	r.agentUserA = &dbmodel.Users{
		ID: 1,
		UserID: sql.NullString{
			String: "1",
			Valid:  true,
		},
		Password: "",
		Username: "agent_a",
		FullName: sql.NullString{
			String: "agent a",
			Valid:  true,
		},
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: r.customerID,
			Valid:  true,
		},
		TeamID: sql.NullString{
			String: "TeamA",
			Valid:  true,
		},
	}
	r.agentUserB = &dbmodel.Users{
		ID: 2,
		UserID: sql.NullString{
			String: "2",
			Valid:  true,
		},
		Password: "",
		Username: "agent_b",
		FullName: sql.NullString{
			String: "agent b",
			Valid:  true,
		},
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: r.customerID,
			Valid:  true,
		},
		TeamID: sql.NullString{
			String: "TeamB",
			Valid:  true,
		},
	}
	r.CreateModels(
		r.agentUserA, r.agentUserB,
	)

	r.UseMockedClickhouseConnections()

	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func (r *RetrieveScorecardStatsTestSuite) Test_InvalidRequestThrows() {
	_, err := r.server.RetrieveScorecardStats(r.ctx, &analyticspb.RetrieveScorecardStatsRequest{})
	r.assertError(err, codes.InvalidArgument, "`filter_by_time_range` must be set.")
}

func (r *RetrieveScorecardStatsTestSuite) TestRetrieveScorecardStatsFromClickHouse() {
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveScorecardStatsRequest
		chReqPath string
	}{
		{
			desc: "Test_BasicFilterAndGroupByAgent",
			request: &analyticspb.RetrieveScorecardStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name: "customers/test_customer/users/agent-id1",
						},
						{
							Name: "customers/test_customer/users/agent-id2",
						},
					},
					ConversationDurationRange: &analyticspb.DurationRange{
						MinDuration: &durationpb.Duration{
							Seconds: 10,
						},
						MaxDuration: &durationpb.Duration{
							Seconds: 100,
						},
					},
					UsecaseNames: []string{"customers/test_customer/profiles/test_profile/usecases/test_usecase"},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveScorecardStats_BasicFilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_DailyFilterAndGroupByAgent",
			request: &analyticspb.RetrieveScorecardStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{{
						Name: userpb.UserName{
							CustomerID: r.customerID,
							UserID:     r.agentUserA.UserID.String,
						}.String(),
					}},
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveScorecardStats_FilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_WeeklyGroupByAgent",
			request: &analyticspb.RetrieveScorecardStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
			},
			chReqPath: "clickhouse_RetrieveScorecardStats_WeeklyGroupByAgent_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			// Mock Clickhouse event client
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			mockRows := clickhouseTesting.NewMockRows(nil)
			r.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// Execute tests.
			_, err := r.server.RetrieveScorecardStats(r.ctx, tc.request)
			r.Require().NoError(err, "RetrieveScorecardStats()")

			r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
			r.Require().Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method, "Expected a call to method %q, but found %q", clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
			gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			wantCHQuery := r.getFile(tc.chReqPath)
			diff := cmp.Diff(formatSQL(wantCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
			r.Empty(diff, "Wrong ClickHouse query: got: \n%v\n want: \n%v\n", formatSQL(combineQueryAndArgs(gotQuery, gotArgs)), formatSQL(wantCHQuery))
		})
	}
}

func (r *RetrieveScorecardStatsTestSuite) Test_BasicRequestAndResponse() {
	// Prepare.
	// The mock values
	// --------------------
	//    | 01/01 | 01/07 |
	// ---+-------+-------+
	//  1 | (2,1) |       |
	//  2 | (1,1) | (1,1) |
	// --------------------
	//
	// where (a,b) means
	// - a: scorecard_completed
	// - b: user_count
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockValues := [][]any{
		{r.janFirst, "1", uint64(2), uint64(1)},
		{r.janFirst, "2", uint64(1), uint64(1)},
		{r.janSeventh, "2", uint64(1), uint64(1)},
	}
	mockRows := clickhouseTesting.NewMockRows(mockValues)
	r.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	response, err := r.server.RetrieveScorecardStats(r.ctx, &analyticspb.RetrieveScorecardStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janTenthProto,
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata: &analyticspb.Metadata{
			TimeZoneId: r.losAngeles,
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
	})

	// Then
	r.Require().NoError(err, "RetrieveScorecardStats()")
	fmt.Printf("response: %v", response)
	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
	expectedResponse := &analyticspb.RetrieveScorecardStatsResponse{
		ScorecardStatsResults: []*analyticspb.ScorecardStatsResult{
			{
				ScorecardStats: []*analyticspb.ScorecardStats{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						ScorecardCompleted:               2,
						AverageScorecardCompletedPerUser: 2.0 / 1,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name:     "customers/customer_id/users/1",
							Username: "agent_a",
							FullName: "agent a",
						},
					},
				},
				ScorecardCompleted:               2,
				AverageScorecardCompletedPerUser: 2.0 / 1,
			},
			{
				ScorecardStats: []*analyticspb.ScorecardStats{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						ScorecardCompleted:               1,
						AverageScorecardCompletedPerUser: 1.0,
					},
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janSeventhProto,
						},
						ScorecardCompleted:               1,
						AverageScorecardCompletedPerUser: 1.0,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name:     "customers/customer_id/users/2",
							Username: "agent_b",
							FullName: "agent b",
						},
					},
				},
				ScorecardCompleted:               2,
				AverageScorecardCompletedPerUser: 2.0 / 1,
			},
		},
		ScorecardCompleted:               4,
		AverageScorecardCompletedPerUser: 4.0 / 2,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveScorecardStatsResponse{}, "scorecard_stats_results"))
	r.Empty(diff, "Proto mismatch (-want, +got):\n%s", diff)
}

func TestRetrieveScorecardStats(t *testing.T) {
	suite.Run(t, &RetrieveScorecardStatsTestSuite{})
}
