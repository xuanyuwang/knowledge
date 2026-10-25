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
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"

	clickhouseTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
)

type RetrieveSmartComposeStatsTestSuite struct {
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

func (r *RetrieveSmartComposeStatsTestSuite) SetupSuite() {
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

func (r *RetrieveSmartComposeStatsTestSuite) Test_InvalidRequestThrows() {
	_, err := r.server.RetrieveSmartComposeStats(r.ctx, &analyticspb.RetrieveSmartComposeStatsRequest{})
	r.assertError(err, codes.InvalidArgument, "`filter_by_time_range` must be set.")
}

func (r *RetrieveSmartComposeStatsTestSuite) TestRetrieveSmartComposeStatsFromClickHouse() {
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveSmartComposeStatsRequest
		chReqPath string
	}{
		{
			desc: "Test_DailyFilterAndGroupByAgent",
			request: &analyticspb.RetrieveSmartComposeStatsRequest{
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
					UsecaseNames: []string{
						"customers/test_customer/profiles/test_profile/usecases/test_usecase",
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveSmartComposeStats_FilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_BasicFilterAndGroupByAgent",
			request: &analyticspb.RetrieveSmartComposeStatsRequest{
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
							Name: "customers/customer_id/users/1",
						},
						{
							Name: "customers/customer_id/users/2",
						},
					},
					UsecaseNames: []string{
						"customers/test_customer/profiles/test_profile/usecases/test_usecase",
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveSmartComposeStats_BasicFilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_DailyFilterAndGroupByGroup",
			request: &analyticspb.RetrieveSmartComposeStatsRequest{
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
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
					UsecaseNames: []string{
						"customers/test_customer/profiles/test_profile/usecases/test_usecase",
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			chReqPath: "clickhouse_RetrieveSmartComposeStats_FilterAndGroupByGroups_request.sql",
		},
		{
			desc: "Test_WeeklyGroupByAgent",
			request: &analyticspb.RetrieveSmartComposeStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				FilterByAttribute: &analyticspb.Attribute{
					UsecaseNames: []string{
						"customers/test_customer/profiles/test_profile/usecases/test_usecase",
					},
				},
			},
			chReqPath: "clickhouse_RetrieveSmartComposeStats_WeeklyGroupByAgent_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			// Mock Clickhouse event client
			defer r.eventMockConn.AssertExpectationsAndReset(r.T())
			mockRows := clickhouseTesting.NewMockRows(nil)
			r.eventMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// When.
			_, err := r.server.RetrieveSmartComposeStats(r.ctx, tc.request)

			// Then
			r.Require().NoError(err, "RetrieveSmartComposeStats()")

			r.Require().Len(r.eventMockConn.Calls, 1, "expected to exactly query once")
			r.Require().Equal(clickhouseQueryMethod, r.eventMockConn.Calls[0].Method, "Expected a call to method %q, but found %q", clickhouseQueryMethod, r.eventMockConn.Calls[0].Method)
			gotQuery := r.eventMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.eventMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			wantCHQuery := r.getFile(tc.chReqPath)
			diff := cmp.Diff(formatSQL(wantCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
			r.Empty(diff, "Wrong ClickHouse query: got: \n%v\n want: \n%v\n", formatSQL(combineQueryAndArgs(gotQuery, gotArgs)), formatSQL(wantCHQuery))
		})
	}
}

func (r *RetrieveSmartComposeStatsTestSuite) Test_BasicRequestAndResponse_GroupByAgent() {
	// Prepare.
	// The mock values from clickhouse.
	// ------------------------
	//    |  01/01  |  01/07  |
	// ---+---------+---------+
	//  1 | (2,1,1) |         |
	//  2 | (1,1,1) | (1,2,1) |
	// ------------------------
	//
	// where (a,b,c) means
	// - a: total_smart_compose_used_count
	// - b: convo_count
	// - c: user_count
	defer r.eventMockConn.AssertExpectationsAndReset(r.T())
	mockValues := [][]any{
		{r.janFirst, "1", uint64(2), uint64(1), uint64(1)},
		{r.janFirst, "2", uint64(1), uint64(1), uint64(1)},
		{r.janSeventh, "2", uint64(1), uint64(2), uint64(1)},
	}
	mockRows := clickhouseTesting.NewMockRows(mockValues)
	r.eventMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// Execute.
	response, err := r.server.RetrieveSmartComposeStats(r.ctx, &analyticspb.RetrieveSmartComposeStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janTenthProto,
		},
		Frequency: analyticspb.Frequency_DAILY,
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
	})

	// Then
	r.Require().NoError(err, "RetrieveSmartComposeStats()")

	fmt.Printf("response: %v", response)
	gotQuery := r.eventMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.eventMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))

	// Clickhouse data:
	// --------------------
	//    | 01/01 | 01/07 |
	// ---+-------+-------+
	//  1 | (2,1) |       |
	//  2 | (1,1) | (1,2) |
	// --------------------
	expectedResponse := &analyticspb.RetrieveSmartComposeStatsResponse{
		SmartComposeStatsResults: []*analyticspb.SmartComposeStatsResult{
			{
				SmartComposeStats: []*analyticspb.SmartComposeStats{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalSmartComposeUsedCount:                      2,
						TotalConversationCount:                          1,
						AverageSmartComposeUsedCountPerConversation:     2.0 / 1,
						AverageSmartComposeUsedCountPerFrequencyPerUser: 2.0 / 1,
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
				TotalSmartComposeUsedCount:                      2,
				TotalConversationCount:                          1,
				AverageSmartComposeUsedCountPerConversation:     2.0 / 1,
				AverageSmartComposeUsedCountPerFrequencyPerUser: 2.0 / 1,
			},
			{
				SmartComposeStats: []*analyticspb.SmartComposeStats{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalSmartComposeUsedCount:                      1,
						TotalConversationCount:                          1,
						AverageSmartComposeUsedCountPerConversation:     1.0 / 1,
						AverageSmartComposeUsedCountPerFrequencyPerUser: 1.0,
					},
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janSeventhProto,
						},
						TotalSmartComposeUsedCount:                      1,
						TotalConversationCount:                          2,
						AverageSmartComposeUsedCountPerConversation:     1.0 / 2,
						AverageSmartComposeUsedCountPerFrequencyPerUser: 1.0,
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
				TotalSmartComposeUsedCount:                      2,
				TotalConversationCount:                          3,
				AverageSmartComposeUsedCountPerConversation:     2.0 / 3,
				AverageSmartComposeUsedCountPerFrequencyPerUser: 2.0 / 2,
			},
		},
		TotalSmartComposeUsedCount:                      4,
		TotalConversationCount:                          4,
		AverageSmartComposeUsedCountPerConversation:     4.0 / 4,
		AverageSmartComposeUsedCountPerFrequencyPerUser: 4.0 / 3,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveSmartComposeStatsResponse{}, "smart_compose_stats_results"))
	r.Empty(diff, "Proto mismatch (-want, +got):\n%s", diff)
}

func (r *RetrieveSmartComposeStatsTestSuite) Test_BasicRequestAndResponse_NoGroupBy() {
	// Prepare.
	// The mock values from clickhouse.
	// ------------------------
	//    |  01/01  |  01/07  |
	// ---+---------+---------+
	//  1 | (2,1,1) |         |
	//  2 | (1,1,1) | (1,2,1) |
	// ---+---------+---------|
	//  * | (3,2,2) | (1,2,1) |
	// ------------------------
	//
	// where (a,b,c) means
	// - a: total_smart_compose_used_count
	// - b: convo_count
	// - c: user_count
	defer r.eventMockConn.AssertExpectationsAndReset(r.T())
	mockValues := [][]any{
		{r.janFirst, uint64(3), uint64(2), uint64(2)},
		{r.janSeventh, uint64(1), uint64(2), uint64(1)},
	}
	mockRows := clickhouseTesting.NewMockRows(mockValues)
	r.eventMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// Execute.
	response, err := r.server.RetrieveSmartComposeStats(r.ctx, &analyticspb.RetrieveSmartComposeStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janTenthProto,
		},
		Frequency: analyticspb.Frequency_DAILY,
	})

	// Verify.
	r.Require().NoError(err, "RetrieveSmartComposeStats()")
	fmt.Printf("response: %v", response)
	gotQuery := r.eventMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.eventMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))

	// Clickhouse data (no group by):
	// --------------------
	//    | 01/01 | 01/07 |
	// ---+-------+-------|
	//  * | (3,2) | (1,2) |
	// --------------------
	expectedResponse := &analyticspb.RetrieveSmartComposeStatsResponse{
		SmartComposeStatsResults: []*analyticspb.SmartComposeStatsResult{
			{
				SmartComposeStats: []*analyticspb.SmartComposeStats{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalSmartComposeUsedCount:                      3,
						TotalConversationCount:                          2,
						AverageSmartComposeUsedCountPerConversation:     3.0 / 2,
						AverageSmartComposeUsedCountPerFrequencyPerUser: 3.0 / 2,
					},
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janSeventhProto,
						},
						TotalSmartComposeUsedCount:                      1,
						TotalConversationCount:                          2,
						AverageSmartComposeUsedCountPerConversation:     1.0 / 2,
						AverageSmartComposeUsedCountPerFrequencyPerUser: 1.0 / 1,
					},
				},
				Attribute:                                       &analyticspb.Attribute{},
				TotalSmartComposeUsedCount:                      4,
				TotalConversationCount:                          4,
				AverageSmartComposeUsedCountPerConversation:     4.0 / 4,
				AverageSmartComposeUsedCountPerFrequencyPerUser: 4.0 / 3,
			},
		},
		TotalSmartComposeUsedCount:                      4,
		TotalConversationCount:                          4,
		AverageSmartComposeUsedCountPerConversation:     4.0 / 4,
		AverageSmartComposeUsedCountPerFrequencyPerUser: 4.0 / 3,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveSmartComposeStatsResponse{}, "smart_compose_stats_results"))
	r.Empty(diff, "Proto mismatch (-want, +got):\n%s", diff)
}

func TestRetrieveSmartComposeStats(t *testing.T) {
	suite.Run(t, &RetrieveSmartComposeStatsTestSuite{})
}
