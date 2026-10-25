package analyticsimpl

import (
	"testing"
	"time"

	nonpubconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	clickhouseConvoTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	sharedproto "github.com/cresta/go-servers/testing/shared/proto"
)

type RetrieveAgentStatsTestSuite struct {
	AnalyticsTestSuite
	users *UsersData
	dates *TimePeriodData
}

func TestRetrieveAgentStatsSuite(t *testing.T) {
	suite.Run(t, &RetrieveAgentStatsTestSuite{})
}

func (r *RetrieveAgentStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.server.enableAgentCountFromCrestaConversation = true
	r.users = r.SetupUsersData()
	r.dates = r.SetupTimePeriodData()
	r.UseMockedClickhouseConnections()
}

func (r *RetrieveAgentStatsTestSuite) TearDownTest() {
}

func (r *RetrieveAgentStatsTestSuite) TearDownSubTest() {
}

type RetrieveAgentStatsTestCase struct {
	desc                      string
	req                       *analyticspb.RetrieveAgentStatsRequest
	want                      *analyticspb.RetrieveAgentStatsResponse
	errorCode                 codes.Code
	chQueryPathWithConvWithAA string
	excludeIngestionPipeline  bool    // If true, sets exclude_ingestion_pipeline_from_agent_stats in config
	mockValues                [][]any // Custom mock ClickHouse rows. If nil, uses default mock data.
}

func (r *RetrieveAgentStatsTestSuite) TestCases() {
	r.Run("TestEmptyRequest", func() {
		tc := RetrieveAgentStatsTestCase{
			desc:      "TestEmptyRequest",
			req:       &analyticspb.RetrieveAgentStatsRequest{},
			errorCode: codes.InvalidArgument,
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_InvalidTimeRange", func() {
		tc := RetrieveAgentStatsTestCase{
			desc: "TestRequest_InvalidTimeRange",
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janTenthProto,
					EndTimestamp:   &r.dates.janFirstProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			errorCode: codes.InvalidArgument,
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_InvalidTimeZone", func() {
		tc := RetrieveAgentStatsTestCase{
			desc: "TestRequest_InvalidTimeZone",
			req: &analyticspb.RetrieveAgentStatsRequest{
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
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_GroupByAgentAndGroup_WithConvWithAA", func() {
		tc := RetrieveAgentStatsTestCase{
			desc: "TestRequest_GroupByAgentAndGroup_WithConvWithAA",
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(fn.Must(time.Parse(time.DateOnly, "2024-01-01"))),
					EndTimestamp:   timestamppb.New(fn.Must(time.Parse(time.DateOnly, "2024-07-01"))),
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			// Response:
			//                         | 01/01 | 01/07 |
			// -----------|------------|-------|-------|
			// agentUserA | groupUser1 |   1   |   1   |
			// agentUserB | groupUser2 |  N/A  |   1   |
			// agentUserC | groupUser2 |   0   |  N/A  |
			// agentUserD | groupUser2 |  N/A  |   0   |
			want: &analyticspb.RetrieveAgentStatsResponse{
				TotalAgentLoginCount:                4,
				AverageActiveDayCountPerUser:        4.0 / 3,
				AverageActiveAgentCountPerFrequency: 4.0 / 2,
				ActiveAgentCount:                    3,
				AgentStatsResults: []*analyticspb.AgentStatsResult{
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 1,
						AverageActiveAgentCountPerFrequency: 2.0 / 2,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1,
								AverageActiveAgentCountPerFrequency: 1,
								ActiveAgentCount:                    1,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1,
								AverageActiveAgentCountPerFrequency: 1,
								ActiveAgentCount:                    1,
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
							Groups: []*userpb.Group{
								r.agentsAndGroups.groupUser1, // Direct group only.
							},
						},
					},
					{
						TotalAgentLoginCount:                1,
						AverageActiveDayCountPerUser:        1.0 / 1,
						AverageActiveAgentCountPerFrequency: 1.0 / 1,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1,
								AverageActiveAgentCountPerFrequency: 1,
								ActiveAgentCount:                    1,
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
							Groups: []*userpb.Group{
								r.agentsAndGroups.groupUser2,
							},
						},
					},
					{
						TotalAgentLoginCount:                0,
						AverageActiveDayCountPerUser:        0,
						AverageActiveAgentCountPerFrequency: 0,
						ActiveAgentCount:                    0,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.users.agentUserC.UserID.String,
								}.String(),
								FullName: r.users.agentUserC.FullName.String,
								Username: r.users.agentUserC.Username,
							}},
							Groups: []*userpb.Group{
								r.agentsAndGroups.groupUser2,
							},
						},
					},
					{
						TotalAgentLoginCount:                1,
						AverageActiveDayCountPerUser:        1.0 / 1,
						AverageActiveAgentCountPerFrequency: 1.0 / 2,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1,
								AverageActiveAgentCountPerFrequency: 1,
								ActiveAgentCount:                    1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.users.agentUserD.UserID.String,
								}.String(),
								FullName: r.users.agentUserD.FullName.String,
								Username: r.users.agentUserD.Username,
							}},
							Groups: []*userpb.Group{
								r.agentsAndGroups.groupUser2,
							},
						},
					},
				},
			},
			chQueryPathWithConvWithAA: "clickhouse_RetrieveAgentStats_GroupByAgentAndGroup_WithConvWithAA.sql",
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_GroupByGroup_WithConvWithAA", func() {
		tc := RetrieveAgentStatsTestCase{
			chQueryPathWithConvWithAA: "clickhouse_RetrieveAgentStats_GroupByGroup_WithConvWithAA.sql",
			desc:                      "TestRequest_GroupByGroup_WithConvWithAA",
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janFirstProto,
					EndTimestamp:   &r.dates.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			// Response:
			//              01/01 | 01/07 |
			// -----------|-------|-------|
			// groupUser1 |     1 |     1 |
			// groupUser2 |     0 |     1 |
			want: &analyticspb.RetrieveAgentStatsResponse{
				TotalAgentLoginCount:                4,
				AverageActiveDayCountPerUser:        4.0 / 3,
				AverageActiveAgentCountPerFrequency: 4.0 / 2,
				ActiveAgentCount:                    3,
				AgentStatsResults: []*analyticspb.AgentStatsResult{
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 1,
						AverageActiveAgentCountPerFrequency: 2.0 / 2,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser1},
						},
					},
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 2,
						AverageActiveAgentCountPerFrequency: 2.0 / 1,
						ActiveAgentCount:                    2,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                2,
								AverageActiveDayCountPerUser:        2.0 / 2,
								AverageActiveAgentCountPerFrequency: 2.0 / 1,
								ActiveAgentCount:                    2,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser2},
						},
					},
				},
			},
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_GroupByGroup_BOT", func() {
		tc := RetrieveAgentStatsTestCase{
			chQueryPathWithConvWithAA: "clickhouse_RetrieveAgentStats_GroupByGroup_WithConvWithAA_Bot.sql",
			desc:                      "TestRequest_GroupByGroup_BOT",
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janFirstProto,
					EndTimestamp:   &r.dates.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
					IncludeCrestaBot: true,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			// Response:
			//              01/01 | 01/07 |
			// -----------|-------|-------|
			// groupUser1 |     1 |     1 |
			// groupUser2 |     0 |     2 |
			want: &analyticspb.RetrieveAgentStatsResponse{
				TotalAgentLoginCount:                4,
				AverageActiveDayCountPerUser:        4.0 / 3,
				AverageActiveAgentCountPerFrequency: 4.0 / 2,
				ActiveAgentCount:                    3,
				AgentStatsResults: []*analyticspb.AgentStatsResult{
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 1,
						AverageActiveAgentCountPerFrequency: 2.0 / 2,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser1},
						},
					},
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 2,
						AverageActiveAgentCountPerFrequency: 2.0 / 1,
						ActiveAgentCount:                    2,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                2,
								AverageActiveDayCountPerUser:        2.0 / 2,
								AverageActiveAgentCountPerFrequency: 2.0 / 1,
								ActiveAgentCount:                    2,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser2},
						},
					},
				},
			},
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_GroupByGroup_ExcludeIngestionPipeline", func() {
		tc := RetrieveAgentStatsTestCase{
			chQueryPathWithConvWithAA: "clickhouse_RetrieveAgentStats_GroupByGroup_ExcludeIngestionPipeline.sql",
			desc:                      "TestRequest_GroupByGroup_ExcludeIngestionPipeline",
			excludeIngestionPipeline:  true,
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janFirstProto,
					EndTimestamp:   &r.dates.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			// Response:
			//              01/01 | 01/07 |
			// -----------|-------|-------|
			// groupUser1 |     1 |     1 |
			// groupUser2 |     0 |     2 |
			want: &analyticspb.RetrieveAgentStatsResponse{
				TotalAgentLoginCount:                4,
				AverageActiveDayCountPerUser:        4.0 / 3,
				AverageActiveAgentCountPerFrequency: 4.0 / 2,
				ActiveAgentCount:                    3,
				AgentStatsResults: []*analyticspb.AgentStatsResult{
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 1,
						AverageActiveAgentCountPerFrequency: 2.0 / 2,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser1},
						},
					},
					{
						TotalAgentLoginCount:                2,
						AverageActiveDayCountPerUser:        2.0 / 2,
						AverageActiveAgentCountPerFrequency: 2.0 / 1,
						ActiveAgentCount:                    2,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                2,
								AverageActiveDayCountPerUser:        2.0 / 2,
								AverageActiveAgentCountPerFrequency: 2.0 / 1,
								ActiveAgentCount:                    2,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser2},
						},
					},
				},
			},
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("TestRequest_ExcludeIngestionPipeline_AgentWithOnlySource8Conversations", func() {
		// This test verifies the FULL OUTER JOIN behavior when an agent has only
		// source 8 (INGESTION_PIPELINE) conversations. With FULL OUTER JOIN, the
		// convs_with_aa CTE produces a row (active=1) even though convs has no row
		// (total=0) because source 8 is excluded. The Go code should use
		// totalAgentCount to gate activeAgentCount, resulting in agentCount=0.
		tc := RetrieveAgentStatsTestCase{
			chQueryPathWithConvWithAA: "clickhouse_RetrieveAgentStats_GroupByGroup_ExcludeIngestionPipeline.sql",
			desc:                      "TestRequest_ExcludeIngestionPipeline_AgentWithOnlySource8Conversations",
			excludeIngestionPipeline:  true,
			// Mock data simulates FULL OUTER JOIN result:
			// agentUserA on 01/01: total=1, active=1 (has source 0 conversations with AA)
			// agentUserA on 01/07: total=0, active=1 (only source 8 conversations with AA)
			// agentUserB on 01/01: total=0, active=1 (only source 8 conversations with AA)
			// agentUserB on 01/07: total=1, active=0 (has source 0 conversations, no AA)
			mockValues: [][]any{
				{r.dates.janFirst, "1", uint64(1), uint64(1)},
				{r.dates.janSeventh, "1", uint64(0), uint64(1)},
				{r.dates.janFirst, "2", uint64(0), uint64(1)},
				{r.dates.janSeventh, "2", uint64(1), uint64(0)},
			},
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janFirstProto,
					EndTimestamp:   &r.dates.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			// Expected response:
			// agentUserA: 01/01 agentCount=1 (total=1, active=1), 01/07 agentCount=0 (total=0, gated)
			// agentUserB: 01/01 agentCount=0 (total=0, gated), 01/07 agentCount=0 (total=1, active=0)
			//
			// Group level (groupUser1 has agentA, groupUser2 has agentB):
			//              01/01 | 01/07 |
			// -----------|-------|-------|
			// groupUser1 |     1 |     0 |
			// groupUser2 |     0 |     0 |
			want: &analyticspb.RetrieveAgentStatsResponse{
				TotalAgentLoginCount:                1,
				AverageActiveDayCountPerUser:        1.0 / 1,
				AverageActiveAgentCountPerFrequency: 1.0 / 1,
				ActiveAgentCount:                    1,
				AgentStatsResults: []*analyticspb.AgentStatsResult{
					{
						TotalAgentLoginCount:                1,
						AverageActiveDayCountPerUser:        1.0 / 1,
						AverageActiveAgentCountPerFrequency: 1.0 / 1,
						ActiveAgentCount:                    1,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                1,
								AverageActiveDayCountPerUser:        1.0 / 1,
								AverageActiveAgentCountPerFrequency: 1.0 / 1,
								ActiveAgentCount:                    1,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser1},
						},
					},
					{
						TotalAgentLoginCount:                0,
						AverageActiveDayCountPerUser:        0,
						AverageActiveAgentCountPerFrequency: 0,
						ActiveAgentCount:                    0,
						AgentStats: []*analyticspb.AgentStats{
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janFirstProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
							{
								TimeRange:                           &analyticspb.TimeRange{StartTimestamp: &r.dates.janSeventhProto},
								TotalAgentLoginCount:                0,
								AverageActiveDayCountPerUser:        0,
								AverageActiveAgentCountPerFrequency: 0,
								ActiveAgentCount:                    0,
							},
						},
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{r.agentsAndGroups.groupUser2},
						},
					},
				},
			},
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})

	r.Run("Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_WithAAConv", func() {
		tc := RetrieveAgentStatsTestCase{
			desc: "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_WithAAConv",
			req: &analyticspb.RetrieveAgentStatsRequest{
				Parent:    r.customerResourceName,
				ProfileId: r.profileID,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.dates.janFirstProto,
					EndTimestamp:   &r.dates.janTenthProto,
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
			want:                      &analyticspb.RetrieveAgentStatsResponse{},
			chQueryPathWithConvWithAA: "clickhouse_RetrieveAgentStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_WithAAConv_request.sql",
		}
		r.runRetrieveAgentStatsTestCase(&tc)
	})
}

func (r *RetrieveAgentStatsTestSuite) runRetrieveAgentStatsTestCase(tc *RetrieveAgentStatsTestCase) {
	// Given
	defer r.authEventMockConn.AssertExpectationsAndReset(r.T())
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())

	// Prepare mocks.
	//
	// Group user mapping
	// - Group1: agentA
	// - Group2: agentB, agentC, agentD
	//
	// The conversation count per agent per day:
	//            | 01/01 | 01/07 |
	// -----------|-------|-------|
	// agentUserA |     1 |     1 |
	// agentUserB |     0 |     1 |
	// agentUserC |     1 |     0 |
	// agentUserD |     0 |     1 |
	// Mock ClickHouse client.
	// The conversation count per agent per day:
	//            | 01/01 | 01/07 |
	// -----------|-------|-------|
	// agentUserA |  (1,1)|  (1,1)|
	// agentUserB |    N/A|  (1,1)|
	// agentUserC |  (1,0)|    N/A|
	// agentUserD |  (1,0)|  (1,1)|
	// where (a,b) - a means total agent count from conv and b means active agent count from conv_aa
	var mockValues [][]any
	switch {
	case tc.mockValues != nil:
		mockValues = tc.mockValues
	case tc.desc == "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments" || tc.desc == "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_WithAAConv":
		mockValues = [][]any{}
	default:
		mockValues = [][]any{
			// truncated time, agent user id, total agent count, active agent count
			{r.dates.janFirst, "1", uint64(1), uint64(1)},
			{r.dates.janSeventh, "1", uint64(1), uint64(1)},
			{r.dates.janSeventh, "2", uint64(1), uint64(1)},
			{r.dates.janFirst, "3", uint64(1), uint64(0)},
			{r.dates.janFirst, "4", uint64(1), uint64(0)},
			{r.dates.janSeventh, "4", uint64(1), uint64(1)},
		}
	}
	mockRows := clickhouseConvoTesting.NewMockRows(mockValues)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// Setup mock config client
	testReporter := testmock.NewTestReporter(r.T())
	mockCtrl := gomock.NewController(testReporter)
	defer mockCtrl.Finish()
	mockConfigClient := mockconfig.NewMockClient(mockCtrl)

	// Build config text with optional features
	featuresConfig := ""
	if tc.excludeIngestionPipeline {
		featuresConfig = `
		features {
			insights {
				exclude_ingestion_pipeline_from_agent_stats: true
			}
		}`
	}
	mockCustomerConfig := fn.Must(config.NewCustomerConfig(sharedproto.ParseFromTextProto(`
		customer: "customers/customer_id"
		profile_configs {
			name: "customers/customer_id/profiles/profile_id"
			language_config {
				default_language_code: "en-US"
			}
			`+featuresConfig+`
		}
		deployment_configs {
			profiles: "customers/customer_id/profiles/profile_id"
			prod_db_conn_arn: 'arn::secret'
			analytics_db_conn_arn: 'arn::secret-analytics'
		}
	`, &nonpubconfigpb.CustomerConfig{})))

	// Set up mock expectations for all test cases
	mockConfigClient.EXPECT().GetProfileConfig(gomock.Any(), gomock.Any()).Return(mockCustomerConfig.ListProfiles()[0], nil).AnyTimes()
	mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(mockCustomerConfig, nil)
	r.server.configClient = mockConfigClient

	// When Execute API.
	got, err := r.server.RetrieveAgentStats(r.ctx, tc.req)

	// Then
	r.Require().Equalf(tc.errorCode, status.Code(err), "Got err %v, but want code %v", err, tc.errorCode)
	if tc.errorCode != codes.OK {
		return
	}

	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	if tc.chQueryPathWithConvWithAA != "" {
		expectedCHQuery := r.getFile(tc.chQueryPathWithConvWithAA)
		r.Assert().Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)), tc.desc)
	}

	// Verify results.
	if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
		r.FailNow("Incorrect response with diff ", "(-want +got):\n %v", diff)
	}
}
