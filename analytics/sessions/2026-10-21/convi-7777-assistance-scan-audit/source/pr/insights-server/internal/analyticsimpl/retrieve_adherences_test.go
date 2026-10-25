// nolint:dupl
package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	nonpubconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config"
	featurespb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	sharedproto "github.com/cresta/go-servers/testing/shared/proto"
)

type RetrieveAdherencesTestSuite struct {
	AnalyticsTestSuite
	users                       *UsersData
	janFirstProto               timestamppb.Timestamp
	sundayOfTheWeekOfJanFirst   timestamppb.Timestamp
	janSeventhProto             timestamppb.Timestamp
	sundayOfTheWeekOfJanSeventh timestamppb.Timestamp
	janEleventhProto            timestamppb.Timestamp
}

func TestRetrieveAdherences(t *testing.T) {
	suite.Run(t, &RetrieveAdherencesTestSuite{})
}

func (r *RetrieveAdherencesTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()

	r.users = r.SetupUsersData()

	// dates
	janFirst := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	r.janFirstProto = *timestamppb.New(janFirst)
	r.sundayOfTheWeekOfJanFirst = *timestamppb.New(time.Date(2020, 12, 27, 0, 0, 0, 0, time.UTC))
	janSeventh := time.Date(2021, 1, 7, 0, 0, 0, 0, time.UTC)
	r.janSeventhProto = *timestamppb.New(janSeventh)
	r.sundayOfTheWeekOfJanSeventh = *timestamppb.New(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))
	r.janEleventhProto = *timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, time.UTC))
	r.UseMockedClickhouseConnections()
}

func (r *RetrieveAdherencesTestSuite) TestCases() {
	for _, tc := range []struct {
		desc                  string
		request               *analyticspb.RetrieveAdherencesRequest
		chReqPath             string
		configWithBotDisabled bool // when true, profile has DisableStatsForConversationSourceBot so bot (7) is excluded from SQL
	}{
		{
			desc: "Basic",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAdherences_Basic_request.sql",
		},
		{
			desc: "ExcludeBotByConfig",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath:             "clickhouse_RetrieveAdherences_ExcludeBotByConfig_request.sql",
			configWithBotDisabled: true,
		},
		{
			desc: "FilterAndGroupByMoment",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_MOMENT_POLICY,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Moments: []*momentpb.Moment{
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/%s", CustomerID, ProfileID, "m1"),
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/%s", CustomerID, ProfileID, "m2"),
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAdherences_FilterAndGroupByMoment_request.sql",
		},
		{
			desc: "FilterAndGroupByPolicy",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
				},
				FilterByAttribute: &analyticspb.Attribute{
					AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/policies/%s", CustomerID, ProfileID, "p1"),
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/policies/%s", CustomerID, ProfileID, "p2"),
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAdherences_FilterAndGroupByPolicy_request.sql",
		},
		{
			desc: "FilterAndGroupByGroups",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						{
							Name: fmt.Sprintf("customers/%s/groups/%s", CustomerID, "group-1"),
						},
						{
							Name: fmt.Sprintf("customers/%s/groups/%s", CustomerID, "group-2"),
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAdherences_FilterAndGroupByGroups_request.sql",
		},
		{
			desc: "FilterAndGroupByGroups_2",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						{
							Name: fmt.Sprintf("customers/%s/groups/%s", CustomerID, "group-3"),
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAdherences_FilterAndGroupByGroups_2_request.sql",
		},
		{
			desc: "IncludeBotConversations",
			request: &analyticspb.RetrieveAdherencesRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				FilterByAttribute: &analyticspb.Attribute{
					IncludeCrestaBot: true,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveAdherences_includeBotConversations_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			mockRows := clickhousetesting.NewMockRows(nil)
			r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// Setup mock config client
			testReporter := testmock.NewTestReporter(r.T())
			mockCtrl := gomock.NewController(testReporter)
			defer mockCtrl.Finish()
			mockConfigClient := mockconfig.NewMockClient(mockCtrl)
			mockCustomerConfig := fn.Must(config.NewCustomerConfig(sharedproto.ParseFromTextProto(`
				customer: "customers/customer_id"
				profile_configs {
					name: "customers/customer_id/profiles/profile_id"
					language_config {
						default_language_code: "en-US"
					}
				}
				deployment_configs {
					profiles: "customers/customer_id/profiles/profile_id"
					prod_db_conn_arn: 'arn::secret'
					analytics_db_conn_arn: 'arn::secret-analytics'
				}
			`, &nonpubconfigpb.CustomerConfig{})))
			if tc.configWithBotDisabled {
				profile := mockCustomerConfig.ListProfiles()[0]
				profile.GetRawProto().Features = &configpb.Features{
					Insights: &featurespb.Insights{
						DisableStatsForConversationSourceBot: true,
					},
				}
			}

			mockConfigClient.EXPECT().GetProfileConfig(gomock.Any(), gomock.Any()).Return(mockCustomerConfig.ListProfiles()[0], nil)
			mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(mockCustomerConfig, nil)
			r.server.configClient = mockConfigClient

			// When.
			_, err := r.server.RetrieveAdherences(r.ctx, tc.request)

			// Then
			r.Require().NoError(err, "RetrieveAdherences()")
			r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
			r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method, "Expected a call to method %q, but found %q", clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
			gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("Got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			wantCHQuery := r.getFile(tc.chReqPath)
			if diff := cmp.Diff(formatSQL(wantCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs))); diff != "" {
				r.FailNow("Wrong ch query", "got: \n%v\n want: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs), wantCHQuery)
			}
		})
	}
}

func (r *RetrieveAdherencesTestSuite) Test_FilterToAgentsOnly_ExplicitUsers() {
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockRows := clickhousetesting.NewMockRows(nil)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)
	r.server.configClient = r.mockConfigClient
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&config.ProfileConfig{}, nil)

	req := &analyticspb.RetrieveAdherencesRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janEleventhProto,
		},
		FilterByAttribute: &analyticspb.Attribute{
			Users: []*userpb.User{
				r.agentsAndGroups.agentUserA,
				{Name: fmt.Sprintf("customers/%s/users/5", CustomerID)},
			},
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Frequency:          analyticspb.Frequency_DAILY,
		FilterToAgentsOnly: true,
	}

	_, err := r.server.RetrieveAdherences(r.ctx, req)

	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")

	filteredUserIDs := make([]string, 0, len(req.FilterByAttribute.Users))
	for _, user := range req.FilterByAttribute.Users {
		userName, parseErr := userpb.ParseUserName(user.GetName())
		r.Require().NoError(parseErr)
		filteredUserIDs = append(filteredUserIDs, userName.GetUserID())
	}
	r.ElementsMatch([]string{"1"}, filteredUserIDs)

	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	gotSQL := combineQueryAndArgs(gotQuery, gotArgs)
	r.Contains(gotSQL, "'1'")
	r.NotContains(gotSQL, "'5'")
}

func (r *RetrieveAdherencesTestSuite) Test_FilterToAgentsOnly_EmptyFilterWithExtTable() {
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockRows := clickhousetesting.NewMockRows(nil)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)
	r.server.configClient = r.mockConfigClient
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&config.ProfileConfig{}, nil)

	origExtTable := r.server.enableExtTableForUserFilter
	r.server.enableExtTableForUserFilter = true
	defer func() { r.server.enableExtTableForUserFilter = origExtTable }()

	req := &analyticspb.RetrieveAdherencesRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janEleventhProto,
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Frequency:          analyticspb.Frequency_DAILY,
		FilterToAgentsOnly: true,
	}

	_, err := r.server.RetrieveAdherences(r.ctx, req)

	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")

	filteredUserIDs := make([]string, 0, len(req.GetFilterByAttribute().GetUsers()))
	for _, user := range req.GetFilterByAttribute().GetUsers() {
		userName, parseErr := userpb.ParseUserName(user.GetName())
		r.Require().NoError(parseErr)
		filteredUserIDs = append(filteredUserIDs, userName.GetUserID())
	}
	r.ElementsMatch([]string{"1", "2", "3", "4"}, filteredUserIDs)
	r.Empty(req.GetFilterByAttribute().GetGroups())
}
