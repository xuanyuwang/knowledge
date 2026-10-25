package analyticsimpl

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhouseTesting "github.com/cresta/go-servers/shared/clickhouse/events/testing"
)

type RetrieveCustomerSnapshotStatsTestSuite struct {
	AnalyticsTestSuite
	agentUserA      *dbmodel.Users
	agentUserB      *dbmodel.Users
	janFirst        time.Time
	janFirstProto   timestamppb.Timestamp
	janSeventh      time.Time
	janSeventhProto timestamppb.Timestamp
}

func (r *RetrieveCustomerSnapshotStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.janFirst = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	r.janFirstProto = *timestamppb.New(r.janFirst)
	r.janSeventh = time.Date(2021, 1, 7, 0, 0, 0, 0, time.UTC)
	r.janSeventhProto = *timestamppb.New(r.janSeventh)
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
	}
	r.CreateModels(
		r.agentUserA, r.agentUserB,
	)
	r.UseMockedClickhouseConnections()
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

// metricQueryMatcher matches the display-time metric query (contains the
// session_intervals CTE), which runs on the events connection.
func metricQueryMatcher() any {
	return mock.MatchedBy(func(q string) bool { return strings.Contains(q, "session_intervals") })
}

func (r *RetrieveCustomerSnapshotStatsTestSuite) TestCases() {
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveCustomerSnapshotStatsRequest
		chReqPath string
	}{
		{
			desc: "Test_DailyFilterAndGroupByAgent",
			request: &analyticspb.RetrieveCustomerSnapshotStatsRequest{
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
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveCustomerSnapshotStats_DailyFilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_DailyFilterAndGroupByGroup",
			request: &analyticspb.RetrieveCustomerSnapshotStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveCustomerSnapshotStats_DailyFilterAndGroupByGroup_request.sql",
		},
		{
			desc: "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveCustomerSnapshotStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, time.UTC)),
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
			chReqPath: "clickhouse_RetrieveCustomerSnapshotStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			// Given
			defer r.eventMockConn.AssertExpectationsAndReset(r.T())
			// The RPC issues one metric query (session_intervals). Without
			// metadata-moment filters it runs on the events connection; with
			// them it switches to the conversations connection. Both return
			// empty rows here, so the response is empty. The metric query is
			// captured from whichever connection ran it for the golden-file
			// comparison.
			r.eventMockConn.On(clickhouseQueryMethod, mock.Anything, metricQueryMatcher(), mock.Anything).
				Return(clickhouseTesting.NewMockRows(nil), nil)

			hasMetadataFiltering := strings.Contains(tc.desc, "FilterByMetadataMomentGroups")
			var mainQueryCall mock.Call
			if hasMetadataFiltering {
				defer r.convoMockConn.AssertExpectationsAndReset(r.T())
				r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, metricQueryMatcher(), mock.Anything).
					Return(clickhouseTesting.NewMockRows(nil), nil)
			}

			// When
			_, err := r.server.RetrieveCustomerSnapshotStats(r.ctx, tc.request)

			// Then
			r.Require().NoError(err, "RetrieveCustomerSnapshotStats()")
			if hasMetadataFiltering {
				r.Require().Len(r.convoMockConn.Calls, 1, "expected the metric query to run on the conversations connection")
				mainQueryCall = r.convoMockConn.Calls[0]
			} else {
				r.Require().Len(r.eventMockConn.Calls, 1, "expected the metric query to run on the events connection")
				mainQueryCall = r.eventMockConn.Calls[0]
			}
			r.Equal(clickhouseQueryMethod, mainQueryCall.Method)
			gotQuery := mainQueryCall.Arguments.Get(1).(string)
			gotArgs := mainQueryCall.Arguments.Get(2).([]any)
			fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			wantCHQuery := r.getFile(tc.chReqPath)
			if diff := cmp.Diff(formatSQL(wantCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs))); diff != "" {
				r.Require().Failf("Wrong ClickHouse query", "got: \n%v\n want: \n%v\n", formatSQL(combineQueryAndArgs(gotQuery, gotArgs)), formatSQL(wantCHQuery))
			}
		})
	}
}

func (r *RetrieveCustomerSnapshotStatsTestSuite) Test_BasicRequestAndResponse() {
	// Given
	// The mock values (metric query rows, scan order: truncated_time, agent_user_id,
	// total_display_seconds, total_conversations_with_snapshot, user_count)
	// ------------------------
	//    |  01/01          |
	// ---+------------------+
	//  1 | (200,10,1)       |
	//  2 | (100,10,1)       |
	// ------------------------
	//
	// where (a,b,c) means
	// - a: total_display_seconds
	// - b: total_conversations_with_snapshot
	// - c: user_count (distinct agents in the bucket; 1 since grouped by agent)
	defer r.eventMockConn.AssertExpectationsAndReset(r.T())
	metricRows := [][]any{
		{r.janFirst, "1", uint64(200), uint64(10), uint64(1)},
		{r.janFirst, "2", uint64(100), uint64(10), uint64(1)},
	}
	r.eventMockConn.On(clickhouseQueryMethod, mock.Anything, metricQueryMatcher(), mock.Anything).
		Return(clickhouseTesting.NewMockRows(metricRows), nil)

	// When
	response, err := r.server.RetrieveCustomerSnapshotStats(r.ctx, &analyticspb.RetrieveCustomerSnapshotStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Frequency: analyticspb.Frequency_DAILY,
	})

	// Then
	r.Require().NoError(err, "RetrieveCustomerSnapshotStats()")
	fmt.Printf("response: %v", response)
	expectedResponse := &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults: []*analyticspb.CustomerSnapshotStatsResult{
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      200,
						AverageDisplayTime:                    200.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
						TotalConversationsWithSnapshot:        10,
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
				TotalDisplayTime:                      200,
				AverageDisplayTime:                    200.0 / 10,
				AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
				TotalConversationsWithSnapshot:        10,
			},
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      100,
						AverageDisplayTime:                    100.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 100.0 / 1,
						TotalConversationsWithSnapshot:        10,
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
				TotalDisplayTime:                      100,
				AverageDisplayTime:                    100.0 / 10,
				AverageDisplayTimePerFrequencyPerUser: 100.0 / 1,
				TotalConversationsWithSnapshot:        10,
			},
		},
		TotalDisplayTime:                      300,
		AverageDisplayTime:                    300.0 / 20,
		AverageDisplayTimePerFrequencyPerUser: 300.0 / 2,
		TotalConversationsWithSnapshot:        20,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveCustomerSnapshotStatsResponse{}, "customer_snapshot_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func (r *RetrieveCustomerSnapshotStatsTestSuite) Test_MultiBucketRequestAndResponse() {
	// Given
	// Mock rows (scan order: truncated_time, agent_user_id, total_display_seconds,
	// total_conversations_with_snapshot, user_count):
	//    |  01/01          |  01/07     |
	// ---+-----------------+------------+
	//  1 | (200,10,1)      |            |
	//  2 | (100,10,1)      | (50,5,1)   |
	// ---+-----------------+------------+
	// where (a,b,c) = (total_display_seconds, total_conversations_with_snapshot, user_count).
	// Two buckets for agent 2 exercise per-result aggregation across time and the
	// top-level AverageDisplayTimePerFrequencyPerUser denominator (3 user×bucket combos).
	defer r.eventMockConn.AssertExpectationsAndReset(r.T())
	metricRows := [][]any{
		{r.janFirst, "1", uint64(200), uint64(10), uint64(1)},
		{r.janFirst, "2", uint64(100), uint64(10), uint64(1)},
		{r.janSeventh, "2", uint64(50), uint64(5), uint64(1)},
	}
	r.eventMockConn.On(clickhouseQueryMethod, mock.Anything, metricQueryMatcher(), mock.Anything).
		Return(clickhouseTesting.NewMockRows(metricRows), nil)

	// When
	response, err := r.server.RetrieveCustomerSnapshotStats(r.ctx, &analyticspb.RetrieveCustomerSnapshotStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Frequency: analyticspb.Frequency_DAILY,
	})

	// Then
	r.Require().NoError(err, "RetrieveCustomerSnapshotStats()")
	expectedResponse := &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults: []*analyticspb.CustomerSnapshotStatsResult{
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      200,
						AverageDisplayTime:                    200.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
						TotalConversationsWithSnapshot:        10,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/1", Username: "agent_a", FullName: "agent a"},
					},
				},
				TotalDisplayTime:                      200,
				AverageDisplayTime:                    200.0 / 10,
				AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
				TotalConversationsWithSnapshot:        10,
			},
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      100,
						AverageDisplayTime:                    100.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 100.0 / 1,
						TotalConversationsWithSnapshot:        10,
					},
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janSeventhProto},
						TotalDisplayTime:                      50,
						AverageDisplayTime:                    50.0 / 5,
						AverageDisplayTimePerFrequencyPerUser: 50.0 / 1,
						TotalConversationsWithSnapshot:        5,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/2", Username: "agent_b", FullName: "agent b"},
					},
				},
				TotalDisplayTime:                      150,
				AverageDisplayTime:                    150.0 / 15,
				AverageDisplayTimePerFrequencyPerUser: 150.0 / 2,
				TotalConversationsWithSnapshot:        15,
			},
		},
		TotalDisplayTime:                      350,
		AverageDisplayTime:                    350.0 / 25,
		AverageDisplayTimePerFrequencyPerUser: 350.0 / 3,
		TotalConversationsWithSnapshot:        25,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveCustomerSnapshotStatsResponse{}, "customer_snapshot_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func (r *RetrieveCustomerSnapshotStatsTestSuite) Test_GroupByGroupRequestAndResponse() {
	// Given
	// Group-1 has user 1; Group-2 has users 2, 3, 4. The per-agent metric query
	// returns rows only for users 1 and 2. Per-user rows are then merged per
	// group, collapsing user×bucket entries into per-group per-bucket buckets.
	//    |  01/01     |  01/07   |
	// ---+------------+----------+
	//  1 | (200,10,1) |          |   -> group-1
	//  2 | (100,10,1) | (50,5,1) |   -> group-2
	// ---+------------+----------+
	defer r.eventMockConn.AssertExpectationsAndReset(r.T())
	metricRows := [][]any{
		{r.janFirst, "1", uint64(200), uint64(10), uint64(1)},
		{r.janFirst, "2", uint64(100), uint64(10), uint64(1)},
		{r.janSeventh, "2", uint64(50), uint64(5), uint64(1)},
	}
	r.eventMockConn.On(clickhouseQueryMethod, mock.Anything, metricQueryMatcher(), mock.Anything).
		Return(clickhouseTesting.NewMockRows(metricRows), nil)

	// When
	response, err := r.server.RetrieveCustomerSnapshotStats(r.ctx, &analyticspb.RetrieveCustomerSnapshotStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		FilterByAttribute: &analyticspb.Attribute{
			Groups: []*userpb.Group{
				r.agentsAndGroups.groupUser1,
				r.agentsAndGroups.groupUser2,
			},
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
		},
		Frequency: analyticspb.Frequency_DAILY,
	})

	// Then
	r.Require().NoError(err, "RetrieveCustomerSnapshotStats()")
	expectedResponse := &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults: []*analyticspb.CustomerSnapshotStatsResult{
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      200,
						AverageDisplayTime:                    200.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
						TotalConversationsWithSnapshot:        10,
					},
				},
				Attribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{r.agentsAndGroups.groupUser1},
				},
				TotalDisplayTime:                      200,
				AverageDisplayTime:                    200.0 / 10,
				AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
				TotalConversationsWithSnapshot:        10,
			},
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      100,
						AverageDisplayTime:                    100.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 100.0 / 1,
						TotalConversationsWithSnapshot:        10,
					},
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janSeventhProto},
						TotalDisplayTime:                      50,
						AverageDisplayTime:                    50.0 / 5,
						AverageDisplayTimePerFrequencyPerUser: 50.0 / 1,
						TotalConversationsWithSnapshot:        5,
					},
				},
				Attribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{r.agentsAndGroups.groupUser2},
				},
				TotalDisplayTime:                      150,
				AverageDisplayTime:                    150.0 / 15,
				AverageDisplayTimePerFrequencyPerUser: 150.0 / 2,
				TotalConversationsWithSnapshot:        15,
			},
		},
		TotalDisplayTime:                      350,
		AverageDisplayTime:                    350.0 / 25,
		AverageDisplayTimePerFrequencyPerUser: 350.0 / 3,
		TotalConversationsWithSnapshot:        25,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveCustomerSnapshotStatsResponse{}, "customer_snapshot_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func (r *RetrieveCustomerSnapshotStatsTestSuite) Test_GroupByGroupAndAgentRequestAndResponse() {
	// Given
	// Group-by group + agent keeps per-agent results but attaches each agent's
	// group(s) to its attribute. Same mock rows as the group-only case.
	defer r.eventMockConn.AssertExpectationsAndReset(r.T())
	metricRows := [][]any{
		{r.janFirst, "1", uint64(200), uint64(10), uint64(1)},
		{r.janFirst, "2", uint64(100), uint64(10), uint64(1)},
		{r.janSeventh, "2", uint64(50), uint64(5), uint64(1)},
	}
	r.eventMockConn.On(clickhouseQueryMethod, mock.Anything, metricQueryMatcher(), mock.Anything).
		Return(clickhouseTesting.NewMockRows(metricRows), nil)

	// When
	response, err := r.server.RetrieveCustomerSnapshotStats(r.ctx, &analyticspb.RetrieveCustomerSnapshotStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		FilterByAttribute: &analyticspb.Attribute{
			Groups: []*userpb.Group{
				r.agentsAndGroups.groupUser1,
				r.agentsAndGroups.groupUser2,
			},
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Frequency: analyticspb.Frequency_DAILY,
	})

	// Then
	r.Require().NoError(err, "RetrieveCustomerSnapshotStats()")
	expectedResponse := &analyticspb.RetrieveCustomerSnapshotStatsResponse{
		CustomerSnapshotStatsResults: []*analyticspb.CustomerSnapshotStatsResult{
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      200,
						AverageDisplayTime:                    200.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
						TotalConversationsWithSnapshot:        10,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/1", Username: "agent_a", FullName: "agent a"},
					},
					Groups: []*userpb.Group{r.agentsAndGroups.groupUser1},
				},
				TotalDisplayTime:                      200,
				AverageDisplayTime:                    200.0 / 10,
				AverageDisplayTimePerFrequencyPerUser: 200.0 / 1,
				TotalConversationsWithSnapshot:        10,
			},
			{
				CustomerSnapshotStats: []*analyticspb.CustomerSnapshotStats{
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janFirstProto},
						TotalDisplayTime:                      100,
						AverageDisplayTime:                    100.0 / 10,
						AverageDisplayTimePerFrequencyPerUser: 100.0 / 1,
						TotalConversationsWithSnapshot:        10,
					},
					{
						TimeRange:                             &analyticspb.TimeRange{StartTimestamp: &r.janSeventhProto},
						TotalDisplayTime:                      50,
						AverageDisplayTime:                    50.0 / 5,
						AverageDisplayTimePerFrequencyPerUser: 50.0 / 1,
						TotalConversationsWithSnapshot:        5,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/2", Username: "agent_b", FullName: "agent b"},
					},
					Groups: []*userpb.Group{r.agentsAndGroups.groupUser2},
				},
				TotalDisplayTime:                      150,
				AverageDisplayTime:                    150.0 / 15,
				AverageDisplayTimePerFrequencyPerUser: 150.0 / 2,
				TotalConversationsWithSnapshot:        15,
			},
		},
		TotalDisplayTime:                      350,
		AverageDisplayTime:                    350.0 / 25,
		AverageDisplayTimePerFrequencyPerUser: 350.0 / 3,
		TotalConversationsWithSnapshot:        25,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveCustomerSnapshotStatsResponse{}, "customer_snapshot_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func TestRetrieveCustomerSnapshotStats(t *testing.T) {
	suite.Run(t, &RetrieveCustomerSnapshotStatsTestSuite{})
}
