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

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
)

type RetrieveLiveAssistStatsTestSuite struct {
	AnalyticsTestSuite
	users *UsersData
	dates *TimePeriodData
}

func (r *RetrieveLiveAssistStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()

	r.users = r.SetupUsersData()
	r.dates = r.SetupTimePeriodData()
	r.UseMockedClickhouseConnections()
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func (r *RetrieveLiveAssistStatsTestSuite) TestCases() {
	for _, tc := range []struct {
		desc                        string
		request                     *analyticspb.RetrieveLiveAssistStatsRequest
		chReqPath                   string
		enableExtTableForUserFilter bool
	}{
		{
			desc: "Basic",
			request: &analyticspb.RetrieveLiveAssistStatsRequest{
				Parent: r.profileResourceName,
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
					UsecaseNames: []string{
						"customers/customer_id/profiles/profile_id/usecases/usecase_id",
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			chReqPath: "clickhouse_RetrieveLiveAssistStats_Basic_request.sql",
		},
		{
			// CONVI-6476: Verify ext table user filter does not inject nil args
			// that shift subsequent positional args (e.g., usecase filter).
			desc:                        "ExtTable_WithUsecaseFilter",
			enableExtTableForUserFilter: true,
			request: &analyticspb.RetrieveLiveAssistStatsRequest{
				Parent: r.profileResourceName,
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
					UsecaseNames: []string{
						"customers/customer_id/profiles/profile_id/usecases/usecase_id",
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			chReqPath: "clickhouse_RetrieveLiveAssistStats_ExtTable_WithUsecaseFilter_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			// Given
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			origExtTable := r.server.enableExtTableForUserFilter
			r.server.enableExtTableForUserFilter = tc.enableExtTableForUserFilter
			defer func() { r.server.enableExtTableForUserFilter = origExtTable }()

			mockRows := clickhousetesting.NewMockRows(nil)
			r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// When
			_, err := r.server.RetrieveLiveAssistStats(r.ctx, tc.request)

			// Then
			r.Require().NoError(err, "RetrieveLiveAssistStats()")
			r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
			r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
			gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			wantCHQuery := r.getFile(tc.chReqPath)
			diff := cmp.Diff(formatSQL(wantCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
			r.Require().Empty(diff, "Wrong ClickHouse query: got: \n%v\n want: \n%v\n", formatSQL(combineQueryAndArgs(gotQuery, gotArgs)), formatSQL(wantCHQuery))
		})
	}
}

func TestRetrieveLiveAssistStats(t *testing.T) {
	suite.Run(t, &RetrieveLiveAssistStatsTestSuite{})
}
