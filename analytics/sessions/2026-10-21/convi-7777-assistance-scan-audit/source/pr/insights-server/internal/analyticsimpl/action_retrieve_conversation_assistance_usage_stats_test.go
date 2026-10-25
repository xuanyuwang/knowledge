package analyticsimpl

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	nonpubconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config"
	featurespb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	"github.com/cresta/go-servers/shared/auth"
	authmock "github.com/cresta/go-servers/shared/auth/mocks"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	sharedassert "github.com/cresta/go-servers/shared/testing/assert"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type assistanceUsageTestBase struct {
	suite.Suite
	chtesting.ClickhouseContainerTestSuite
	server                                                           *AnalyticsServiceImpl
	customerID, profileID, customerResourceName, profileResourceName string
	mockConfigClient                                                 *mockconfig.MockClient
	mockACLHelper                                                    *authmock.MockResourceACLHelper
	convoMockConn                                                    *clickhousetesting.MockConn
}

func (s *assistanceUsageTestBase) SetupSuite() {
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.customerID, s.profileID = "population", uuid.NewString()
	s.customerResourceName = "customers/" + s.customerID
	s.profileResourceName = s.customerResourceName + "/profiles/" + s.profileID
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	s.mockConfigClient = mockconfig.NewMockClient(controller)
	s.mockConfigClient.EXPECT().Get(s.customerID).AnyTimes().Return(&config.CustomerConfig{}, nil)
	s.mockACLHelper = authmock.NewMockResourceACLHelper(controller)
	aclProvider := authmock.NewMockResourceACLHelperProvider(controller)
	aclProvider.EXPECT().Get().AnyTimes().Return(s.mockACLHelper)
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
		Parent: s.customerResourceName, ProfileIds: []string{s.profileID}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true,
	})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: []*internaluserpb.LiteUser{
		{UserId: "1"}, {UserId: "2"}, {UserId: "3"},
	}}, nil)
	users := usermock.NewMockUserServiceClient(controller)
	users.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{
		Parent: s.customerResourceName, PageSize: 200,
		Filter: &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}},
	})).AnyTimes().Return(&userpb.ListGroupsResponse{}, nil)
	s.server = &AnalyticsServiceImpl{
		logger: log.GetLogger(), configClient: s.mockConfigClient, resourceACLHelperProvider: aclProvider,
		clickhouseConvoRegistry:           s.GetConversationTestRegistry(),
		userClientRegistry:                clientregistry.NewTestRegistry[userpb.UserServiceClient](users),
		internalUserServiceClientRegistry: clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers),
	}
}

func (s *assistanceUsageTestBase) TearDownSuite() {
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
}

type ConversationAssistanceUsageValidationTestSuite struct {
	suite.Suite
}

func TestConversationAssistanceUsageValidation(t *testing.T) {
	suite.Run(t, new(ConversationAssistanceUsageValidationTestSuite))
}

func (s *ConversationAssistanceUsageValidationTestSuite) TestInvalidParent() {
	// Given
	server := AnalyticsServiceImpl{}
	request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{Parent: "invalid"}

	// When
	response, err := server.RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

	// Then
	s.Nil(response)
	s.Equal(codes.InvalidArgument, status.Code(err))
}

type ConversationAssistanceUsagePopulationCHTestSuite struct {
	assistanceUsageTestBase
	start    time.Time
	sharedID string
}

func TestConversationAssistanceUsagePopulationCH(t *testing.T) {
	suite.Run(t, new(ConversationAssistanceUsagePopulationCHTestSuite))
}

func (s *ConversationAssistanceUsagePopulationCHTestSuite) SetupSuite() {
	s.assistanceUsageTestBase.SetupSuite()
	s.mockConfigClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	s.start = time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	s.sharedID = uuid.NewString()
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	for _, fixture := range []struct {
		conversationID string
		agentID        string
		start          time.Time
		hasMessage     bool
	}{
		{s.sharedID, "1", s.start, true},
		{s.sharedID, "2", s.start, true},
		{s.sharedID, "3", s.start, true},
		{uuid.NewString(), "1", s.start, false},
		{uuid.NewString(), "1", s.start.Add(24 * time.Hour), true},
	} {
		s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, conversation_end_time, update_time) VALUES (?, ?, ?, ?, ?)`, fixture.conversationID, fixture.agentID, fixture.start, fixture.start.Add(48*time.Hour), fixture.start))
		if fixture.hasMessage {
			s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO message (message_id, conversation_id, agent_user_id, conversation_start_time, conversation_end_time) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), fixture.conversationID, fixture.agentID, fixture.start, fixture.start.Add(48*time.Hour)))
		}
	}
}

func (s *ConversationAssistanceUsagePopulationCHTestSuite) TestUserRestrictionsWithExternalTableFlagDisabled() {
	for _, scenario := range []struct {
		name                                               string
		agentsOnly, excludeInactive, empty, largeDirectory bool
	}{
		{name: "AgentsOnly", agentsOnly: true},
		{name: "ActiveOnly", excludeInactive: true},
		{name: "AgentsOnlyLargeDirectory", agentsOnly: true, largeDirectory: true},
		{name: "ActiveOnlyLargeDirectory", excludeInactive: true, largeDirectory: true},
		{name: "NoEligibleAgents", agentsOnly: true, empty: true},
		{name: "NoActiveUsers", excludeInactive: true, empty: true},
	} {
		s.Run(scenario.name, func() {
			// Given
			server := *s.server
			server.enableExtTableForUserFilter = false
			users := internalusermock.NewMockInternalUserServiceClient(gomock.NewController(testmock.NewTestReporter(s.T())))
			eligible := []*internaluserpb.LiteUser{{UserId: "1"}}
			if scenario.largeDirectory {
				// UUID-length IDs exceed the SQL size limit when expanded into IN lists.
				for i := 0; i < 8000; i++ {
					eligible = append(eligible, &internaluserpb.LiteUser{UserId: fmt.Sprintf("00000000-0000-0000-0000-%012d", i)})
				}
			}
			if scenario.empty {
				eligible = nil
			}
			users.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent: s.customerResourceName, ProfileIds: []string{s.profileID}, AgentOnly: scenario.agentsOnly,
				IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: !scenario.excludeInactive,
			})).Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: eligible}, nil)
			server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](users)
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{
				Parent: s.profileResourceName, FilterToAgentsOnly: scenario.agentsOnly,
				FilterByAttribute: &analyticspb.Attribute{ExcludeDeactivatedUsers: scenario.excludeInactive},
				FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(s.start), EndTimestamp: timestamppb.New(s.start.Add(time.Hour))},
			}

			// When
			input, err := server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			if scenario.empty {
				s.Empty(input.population)
			} else {
				s.Equal([]conversationAssistanceUsagePopulationRow{{conversationID: s.sharedID, agentUserID: "1", conversationTime: s.start}}, input.population)
			}
		})
	}
}

func (s *ConversationAssistanceUsagePopulationCHTestSuite) TestEligiblePairsUseStartTimeAndPreserveRequest() {
	for _, externalUsers := range []bool{false, true} {
		s.Run(fmt.Sprintf("ExternalUsers_%t", externalUsers), func() {
			// Given
			s.server.enableExtTableForUserFilter = externalUsers
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(s.start),
					EndTimestamp:   timestamppb.New(s.start.Add(24 * time.Hour)),
				},
				FilterByAttribute: &analyticspb.Attribute{Users: []*userpb.User{
					{Name: "customers/" + s.customerID + "/users/1"},
					{Name: "customers/" + s.customerID + "/users/2"},
				}},
			}
			original := proto.Clone(request)

			// When
			population, err := s.server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			s.ElementsMatch([]conversationAssistanceUsagePopulationRow{
				{conversationID: s.sharedID, agentUserID: "1", conversationTime: s.start},
				{conversationID: s.sharedID, agentUserID: "2", conversationTime: s.start},
			}, population.population)
			s.True(proto.Equal(original, request), "the request must not be mutated")
		})
	}
}

func (s *ConversationAssistanceUsagePopulationCHTestSuite) AfterTest(_, _ string) {
	s.server.configClient = s.mockConfigClient
	s.server.enableExtTableForUserFilter = false
}

func (s *ConversationAssistanceUsagePopulationCHTestSuite) TestACLRestrictsEligibleAgents() {
	// Given
	setupAssistanceUsageACL(&s.assistanceUsageTestBase, []string{"1"})
	request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(s.start),
			EndTimestamp:   timestamppb.New(s.start.Add(24 * time.Hour)),
		},
	}

	// When
	population, err := s.server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

	// Then
	s.Require().NoError(err)
	s.Equal([]conversationAssistanceUsagePopulationRow{
		{conversationID: s.sharedID, agentUserID: "1", conversationTime: s.start},
	}, population.population)
}

func (s *ConversationAssistanceUsagePopulationCHTestSuite) TestProfileBotPolicy() {
	for _, disableBots := range []bool{false, true} {
		s.Run(fmt.Sprintf("DisableBots_%t", disableBots), func() {
			// Given
			start := s.start.Add(-48 * time.Hour)
			conversationID := uuid.NewString()
			conn := s.GetConversationConnection(s.customerID, s.profileID)
			s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, update_time, conversation_source) VALUES (?, '1', ?, ?, 7)`, conversationID, start, start))
			s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO message (message_id, conversation_id, agent_user_id, conversation_start_time, conversation_source) VALUES (?, ?, '1', ?, 7)`, uuid.NewString(), conversationID, start))
			profileConfig, err := config.NewProfileConfig(&nonpubconfigpb.ProfileConfig{
				Name: s.profileResourceName,
				Features: &configpb.Features{Insights: &featurespb.Insights{
					DisableStatsForConversationSourceBot: disableBots,
				}},
			})
			s.Require().NoError(err)
			configClient := mockconfig.NewMockClient(gomock.NewController(testmock.NewTestReporter(s.T())))
			configClient.EXPECT().Get(s.customerID).Return(&config.CustomerConfig{}, nil)
			configClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).Return(profileConfig, nil)
			s.server.configClient = configClient
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(time.Hour)),
				},
			}

			// When
			population, err := s.server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			if disableBots {
				s.Empty(population.population)
			} else {
				s.Equal([]conversationAssistanceUsagePopulationRow{{conversationID: conversationID, agentUserID: "1", conversationTime: start}}, population.population)
			}
		})
	}
}

func setupAssistanceUsageACL(s *assistanceUsageTestBase, userIDs []string) {
	configClient := mockconfig.NewMockClient(gomock.NewController(testmock.NewTestReporter(s.T())))
	customerConfig, err := config.NewCustomerConfig(&nonpubconfigpb.CustomerConfig{
		Customer: s.customerResourceName,
		AuthConfig: &nonpubconfigpb.CustomerAuthConfig{
			EnableAdvancedDataAccessControl: true,
		},
	})
	s.Require().NoError(err)
	configClient.EXPECT().Get(s.customerID).Return(customerConfig, nil)
	configClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	s.server.configClient = configClient
	s.mockACLHelper.EXPECT().GetResourceACL(gomock.Any(), s.customerID, gomock.Any()).Return(&auth.ResourceACL{ManagedUserIDs: userIDs}, nil)
}

type ConversationAssistanceUsageTestSuite struct {
	assistanceUsageTestBase
}

func TestConversationAssistanceUsage(t *testing.T) {
	suite.Run(t, new(ConversationAssistanceUsageTestSuite))
}

func (s *ConversationAssistanceUsageTestSuite) SetupSuite() {
	s.assistanceUsageTestBase.SetupSuite()
	s.convoMockConn = new(clickhousetesting.MockConn)
	s.SetConversationConnectionMock(s.customerID, s.profileID, s.convoMockConn)
	s.server.clickhouseEventRegistry = s.GetEventsTestRegistry()
	s.SetEventsConnectionMock(s.customerID, s.profileID, s.convoMockConn)
	s.mockConfigClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
}

func (s *ConversationAssistanceUsageTestSuite) AfterTest(_, _ string) {
	s.convoMockConn.AssertExpectations(s.T())
	s.convoMockConn.Mock = mock.Mock{}
	s.server.configClient = s.mockConfigClient
}

func (s *ConversationAssistanceUsageTestSuite) TearDownSubTest() {
	s.convoMockConn.AssertExpectations(s.T())
	s.convoMockConn.Mock = mock.Mock{}
}

func (s *ConversationAssistanceUsageTestSuite) request() *analyticspb.RetrieveConversationAssistanceUsageStatsRequest {
	start := time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	return &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(start),
			EndTimestamp:   timestamppb.New(start.Add(24 * time.Hour)),
		},
	}
}

func (s *ConversationAssistanceUsageTestSuite) TestEmptySelectedAudienceDoesNotQueryAllUsers() {
	// Given
	request := s.request()
	request.FilterByAttribute = &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + s.customerID + "/users/missing"}}}

	// When
	population, err := s.server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

	// Then
	s.Require().NoError(err)
	s.Empty(population.population)
	s.convoMockConn.AssertNotCalled(s.T(), "Query", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ConversationAssistanceUsageTestSuite) TestEmptyACLAudienceDoesNotQueryAllUsers() {
	// Given
	setupAssistanceUsageACL(&s.assistanceUsageTestBase, nil /* userIDs */)
	request := s.request()

	// When
	population, err := s.server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

	// Then
	s.Require().NoError(err)
	s.Empty(population.population)
	s.convoMockConn.AssertNotCalled(s.T(), "Query", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ConversationAssistanceUsageTestSuite) TestRPCReturnsDistinctPopulationCount() {
	for _, hasPopulation := range []bool{false, true} {
		s.Run(fmt.Sprintf("HasPopulation_%t", hasPopulation), func() {
			// Given
			request := s.request()
			var values [][]any
			if hasPopulation {
				values = [][]any{
					{"shared", "1", request.FilterByTimeRange.StartTimestamp.AsTime()},
					{"shared", "2", request.FilterByTimeRange.StartTimestamp.AsTime()},
					{"other", "1", request.FilterByTimeRange.StartTimestamp.AsTime()},
				}
			}
			s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).
				Return(clickhousetesting.NewMockRows(values), nil).Once()
			s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(clickhousetesting.NewMockRows(nil), nil)

			// When
			response, err := s.server.RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			want := &analyticspb.RetrieveConversationAssistanceUsageStatsResponse{ConversationAssistanceUsageStatsResults: []*analyticspb.ConversationAssistanceUsageStatsResult{{}}}
			if hasPopulation {
				want.TotalConversationCount = 2
				want.ConversationAssistanceUsageStatsResults[0].TotalConversationCount = 2
			}
			s.True(proto.Equal(want, response), "want %v, got %v", want, response)
		})
	}
}

type assistanceUsageIterationErrorRows struct {
	*clickhousetesting.MockRows
	err error
}

func (r *assistanceUsageIterationErrorRows) Err() error { return r.err }

func (s *ConversationAssistanceUsageTestSuite) TestRPCReadErrorsDiscardPartialRows() {
	for _, operation := range []string{"Query", "Scan", "Iteration"} {
		s.Run(operation, func() {
			// Given
			request := s.request()
			failure := errors.New("population read failed")
			rows := &assistanceUsageIterationErrorRows{MockRows: clickhousetesting.NewMockRows([][]any{
				{"conversation", "1", request.FilterByTimeRange.StartTimestamp.AsTime()},
			})}
			var queryErr error
			if operation == "Query" {
				queryErr = failure
			}
			if operation == "Scan" {
				rows.MockRows = clickhousetesting.NewMockRows([][]any{{"conversation", "1", "invalid timestamp"}})
			}
			if operation == "Iteration" {
				rows.err = failure
			}
			s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(rows, queryErr)

			// When
			response, err := s.server.RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

			// Then
			s.Require().Error(err)
			s.Nil(response)
			if operation == "Scan" {
				s.ErrorContains(err, "scan assistance usage population")
			} else {
				s.ErrorIs(err, failure)
			}
		})
	}
}

func (s *ConversationAssistanceUsageTestSuite) TestRPCUsesEvidenceUnion() {
	// Given
	request := s.request()
	s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).
		Return(clickhousetesting.NewMockRows([][]any{
			{"shared", "1", request.FilterByTimeRange.StartTimestamp.AsTime()},
			{"shared", "2", request.FilterByTimeRange.StartTimestamp.AsTime()},
			{"unused", "1", request.FilterByTimeRange.StartTimestamp.AsTime()},
		}), nil).Once()
	for _, query := range conversationAssistanceUsageEvidenceQueries(true /* includeBotConversations */) {
		s.convoMockConn.On("Query", mock.Anything, query.query, mock.Anything).
			Return(clickhousetesting.NewMockRows([][]any{{"shared", "1"}, {"shared", "1"}, {"unused", "ineligible"}}), nil).Once()
	}

	// When
	response, err := s.server.RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

	// Then
	s.Require().NoError(err)
	expected := &analyticspb.RetrieveConversationAssistanceUsageStatsResponse{
		TotalConversationCount:               2,
		ConversationsWithAssistanceUsedCount: 1,
		ConversationAssistanceUsageStatsResults: []*analyticspb.ConversationAssistanceUsageStatsResult{{
			TotalConversationCount:               2,
			ConversationsWithAssistanceUsedCount: 1,
		}},
	}
	sharedassert.CmpEquals(s.T(), expected, response)
}

func (s *ConversationAssistanceUsageValidationTestSuite) TestAggregationAttributionAndBuckets() {
	for _, grouping := range []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP} {
		s.Run(grouping.String(), func() {
			// Given
			start := time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
			alice := &userpb.User{Name: "customers/c/users/1"}
			bob := &userpb.User{Name: "customers/c/users/2"}
			groups := []*userpb.Group{{Name: "customers/c/groups/both"}, {Name: "customers/c/groups/alice"}, {Name: "customers/c/groups/bob"}}
			scope := &ParseUserFilterResult{FinalUsers: []*userpb.User{alice, bob}, GroupsToAggregate: groups, UserNameToGroupNamesMap: map[string][]string{alice.Name: {groups[0].Name, groups[1].Name}, bob.Name: {groups[0].Name, groups[2].Name}}}
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{Parent: "customers/c/profiles/p", Frequency: analyticspb.Frequency_DAILY, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}, GroupByAttributeTypes: []analyticspb.AttributeType{grouping}}
			population := []conversationAssistanceUsagePopulationRow{{"shared", "1", start}, {"shared", "2", start}, {"shared", "1", start}, {"unused", "1", start.Add(24 * time.Hour)}}
			evidence := set.New(assistanceUsagePair{"shared", "1"}, assistanceUsagePair{"unused", "ineligible"})
			first := time.Date(2021, 1, 5, 10, 0, 0, 0, time.UTC)
			bucket := func(start time.Time, total, used int32) *analyticspb.ConversationAssistanceUsageStats {
				return &analyticspb.ConversationAssistanceUsageStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(24 * time.Hour))}, TotalConversationCount: total, ConversationsWithAssistanceUsedCount: used}
			}
			results := []*analyticspb.ConversationAssistanceUsageStatsResult{
				{Attribute: &analyticspb.Attribute{Users: []*userpb.User{alice}}, TotalConversationCount: 2, ConversationsWithAssistanceUsedCount: 1, ConversationAssistanceUsageStats: []*analyticspb.ConversationAssistanceUsageStats{bucket(first, 1, 1), bucket(first.Add(24*time.Hour), 1, 0)}},
				{Attribute: &analyticspb.Attribute{Users: []*userpb.User{bob}}, TotalConversationCount: 1, ConversationAssistanceUsageStats: []*analyticspb.ConversationAssistanceUsageStats{bucket(first, 1, 0)}},
			}
			if grouping == analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP {
				results = []*analyticspb.ConversationAssistanceUsageStatsResult{
					{Attribute: &analyticspb.Attribute{Groups: groups[:1]}, TotalConversationCount: 3, ConversationsWithAssistanceUsedCount: 1, ConversationAssistanceUsageStats: []*analyticspb.ConversationAssistanceUsageStats{bucket(first, 2, 1), bucket(first.Add(24*time.Hour), 1, 0)}},
					{Attribute: &analyticspb.Attribute{Groups: groups[1:2]}, TotalConversationCount: 2, ConversationsWithAssistanceUsedCount: 1, ConversationAssistanceUsageStats: []*analyticspb.ConversationAssistanceUsageStats{bucket(first, 1, 1), bucket(first.Add(24*time.Hour), 1, 0)}},
					{Attribute: &analyticspb.Attribute{Groups: groups[2:]}, TotalConversationCount: 1, ConversationAssistanceUsageStats: []*analyticspb.ConversationAssistanceUsageStats{bucket(first, 1, 0)}},
				}
			}
			expected := &analyticspb.RetrieveConversationAssistanceUsageStatsResponse{TotalConversationCount: 2, ConversationsWithAssistanceUsedCount: 1, ConversationAssistanceUsageStatsResults: results}

			// When
			actual, err := aggregateConversationAssistanceUsage(request, population, evidence, scope)

			// Then
			s.Require().NoError(err)
			sharedassert.CmpEquals(s.T(), expected, actual)
		})
	}
}

func (s *ConversationAssistanceUsageValidationTestSuite) TestWeeklyAndUnspecifiedFrequency() {
	for _, firstDay := range []analyticspb.Metadata_DayOfWeek{analyticspb.Metadata_SUNDAY, analyticspb.Metadata_MONDAY} {
		s.Run(firstDay.String(), func() {
			// Given
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{Parent: "customers/c/profiles/p", Frequency: analyticspb.Frequency_WEEKLY, Metadata: &analyticspb.Metadata{TimeZoneId: "UTC", StartDayOfWeek: firstDay}}
			population := []conversationAssistanceUsagePopulationRow{{"c", "1", time.Date(2021, 1, 3, 12, 0, 0, 0, time.UTC)}}
			start := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)
			if firstDay == analyticspb.Metadata_MONDAY {
				start = time.Date(2020, 12, 28, 0, 0, 0, 0, time.UTC)
			}
			expected := &analyticspb.RetrieveConversationAssistanceUsageStatsResponse{TotalConversationCount: 1, ConversationsWithAssistanceUsedCount: 1, ConversationAssistanceUsageStatsResults: []*analyticspb.ConversationAssistanceUsageStatsResult{{TotalConversationCount: 1, ConversationsWithAssistanceUsedCount: 1, ConversationAssistanceUsageStats: []*analyticspb.ConversationAssistanceUsageStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(7 * 24 * time.Hour))}, TotalConversationCount: 1, ConversationsWithAssistanceUsedCount: 1}}}}}

			// When
			actual, err := aggregateConversationAssistanceUsage(request, population, set.New(assistanceUsagePair{"c", "1"}), &ParseUserFilterResult{})

			// Then
			s.Require().NoError(err)
			sharedassert.CmpEquals(s.T(), expected, actual)
		})
	}
}

func (s *ConversationAssistanceUsageValidationTestSuite) TestUnsupportedRequestOptions() {
	for _, kind := range []string{"Monthly", "MultipleGroupings", "BehaviorGrouping", "Details", "Timezone", "MissingTime", "ReversedTime"} {
		s.Run(kind, func() {
			// Given
			start := time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC)
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{Parent: "customers/c/profiles/p", FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(time.Hour))}}
			expectedCode := codes.InvalidArgument
			switch kind {
			case "Monthly":
				request.Frequency = analyticspb.Frequency_MONTHLY
			case "MultipleGroupings":
				request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP}
			case "BehaviorGrouping":
				request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR}
			case "Details":
				request.IncludeConversationUsageDetails = true
				expectedCode = codes.Unimplemented
			case "Timezone":
				request.Metadata = &analyticspb.Metadata{TimeZoneId: "Not/AZone"}
			case "MissingTime":
				request.FilterByTimeRange = nil
			case "ReversedTime":
				request.FilterByTimeRange.EndTimestamp = timestamppb.New(start.Add(-time.Hour))
			}

			// When
			response, err := (AnalyticsServiceImpl{}).RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

			// Then
			s.Nil(response)
			s.Equal(expectedCode, status.Code(err))
		})
	}
}

type ConversationAssistanceUsageEvidenceCHTestSuite struct{ assistanceUsageTestBase }

func TestConversationAssistanceUsageEvidenceCH(t *testing.T) {
	suite.Run(t, new(ConversationAssistanceUsageEvidenceCHTestSuite))
}

func (s *ConversationAssistanceUsageEvidenceCHTestSuite) SetupSuite() {
	s.assistanceUsageTestBase.SetupSuite()
	s.server.clickhouseEventRegistry = s.GetEventsTestRegistry()
	s.mockConfigClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
}

func (s *ConversationAssistanceUsageEvidenceCHTestSuite) TestFeaturePredicatesAndPairIntersection() {
	cases := []struct {
		name string
		want int32
	}{
		{"KBSearch", 1}, {"LateEvidence", 1}, {"Suggestion", 1}, {"HintSuggestion", 1}, {"SearchSuggestion", 1}, {"GenAI", 1}, {"GenAIWithoutSession", 0}, {"GenAIBotSource", 0},
		{"SummaryCopy", 1}, {"SummaryEditType2", 1}, {"SummaryWithoutCopy", 0}, {"SummaryMismatchedTime", 0}, {"SummaryWithoutGeneration", 0}, {"SummarySingleNoteType3", 0},
		{"BehavioralHint", 1}, {"ChecklistHint", 1}, {"UnfollowedHint", 0}, {"RegularHint", 0}, {"KBHint", 1}, {"WorkflowHint", 1},
		{"NotesCopy", 1}, {"NotesEdit", 0}, {"WorkflowDocument", 1}, {"OrdinaryDocument", 0}, {"WrongAgent", 0}, {"OfferedOnly", 0}, {"MultipleFeatures", 1},
	}
	for i, tc := range cases {
		s.Run(tc.name, func() {
			// Given
			start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * 24 * time.Hour)
			conversationID := uuid.NewString()
			conv := s.GetConversationConnection(s.customerID, s.profileID)
			events := s.GetEventsConnection(s.customerID, s.profileID)
			for _, conn := range []interface {
				Exec(context.Context, string, ...any) error
			}{conv, events} {
				s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO conversation_d (conversation_id,agent_user_id,conversation_start_time,update_time) VALUES (?,'1',?,?)`, conversationID, start, start))
			}
			s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO message (message_id,conversation_id,agent_user_id,conversation_start_time) VALUES (?,?,'1',?)`, uuid.NewString(), conversationID, start))
			evidenceStart := start
			if tc.name == "LateEvidence" {
				evidenceStart = start.Add(7 * 24 * time.Hour)
			}
			conversationEvent := func(eventType int32, agent string, actionType int32) {
				s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,conversation_start_time,event_type,action_type,action_policy_source_type,action_annotation_id) VALUES (?,?,?,?,?,?,1,?)`, uuid.NewString(), conversationID, agent, evidenceStart, eventType, actionType, uuid.NewString()))
			}
			switch tc.name {
			case "KBSearch", "MultipleFeatures", "LateEvidence":
				conversationEvent(13, "1", 0)
			case "WrongAgent":
				conversationEvent(13, "2", 0)
			case "Suggestion":
				conversationEvent(4, "1", 0)
			case "HintSuggestion":
				conversationEvent(5, "1", 0)
			case "SearchSuggestion":
				conversationEvent(10, "1", 0)
			case "KBHint":
				conversationEvent(9, "1", 14)
			case "WorkflowHint":
				conversationEvent(9, "1", 13)
			case "GenAI", "GenAIWithoutSession", "GenAIBotSource":
				session := "session"
				if tc.name == "GenAIWithoutSession" {
					session = ""
				}
				source := int32(0)
				if tc.name == "GenAIBotSource" {
					source = 7
				}
				s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO moment_annotation_d (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,moment_type,knowledge_payload_knowledge_session_id,conversation_source) VALUES (?,?,'1',?,38,?,?)`, uuid.NewString(), conversationID, start, session, source))
			case "SummaryCopy", "SummaryEditType2", "SummaryWithoutCopy", "SummaryWithoutGeneration", "SummarySingleNoteType3", "SummaryMismatchedTime":
				if tc.name != "SummaryWithoutGeneration" {
					s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO action_annotation_d (action_annotation_id,conversation_id,agent_user_id,conversation_start_time,summarization_payload_is_valid_summary) VALUES (?,?,'1',?,1)`, uuid.NewString(), conversationID, start))
				}
				if tc.name != "SummaryWithoutCopy" {
					dataType := int32(1)
					if tc.name == "SummaryEditType2" {
						dataType = 2
					}
					if tc.name == "SummarySingleNoteType3" {
						dataType = 3
					}
					if tc.name == "SummaryMismatchedTime" {
						evidenceStart = start.Add(time.Minute)
					}
					s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,conversation_start_time,event_type,summarization_payload_data_type) VALUES (?,?,'1',?,16,?)`, uuid.NewString(), conversationID, evidenceStart, dataType))
				}
			case "BehavioralHint", "ChecklistHint", "UnfollowedHint", "RegularHint":
				actionType, adherence, actionAdherence := int32(1), int32(2), int32(1)
				if tc.name == "ChecklistHint" {
					actionType = 21
				}
				if tc.name == "UnfollowedHint" {
					adherence = 3
				}
				if tc.name == "RegularHint" {
					actionAdherence = 0
				}
				s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO moment_annotation_d (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,adherence_action_annotation_id,adherence_type,adherence_action_type,adherence_action_adherence_type,adherence_action_policy_source_type) VALUES (?,?,'1',?,?,?,?,?,1)`, uuid.NewString(), conversationID, start, uuid.NewString(), adherence, actionType, actionAdherence))
			case "NotesCopy", "NotesEdit":
				eventName := "VoiceAgentAssist.NoteTaking.Click.CopyEntity"
				if tc.name == "NotesEdit" {
					eventName = "VoiceAgentAssist.NoteTaking.Click.EditEntity"
				}
				s.Require().NoError(events.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,conversation_start_time,event_type,event_name,event_payload) VALUES (?,?,'1',?,5,?, '{}')`, uuid.NewString(), conversationID, start, eventName))
			case "WorkflowDocument", "OrdinaryDocument":
				docType := "2"
				if tc.name == "OrdinaryDocument" {
					docType = "1"
				}
				payload := fmt.Sprintf(`{"usedKbDocument":{"document":"doc","docType":"%s","sessionMetadata":{"sessionId":"session"}}}`, docType)
				s.Require().NoError(events.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,conversation_start_time,event_type,event_payload) VALUES (?,?,'1',?,4,?)`, uuid.NewString(), conversationID, start, payload))
			case "OfferedOnly":
				s.Require().NoError(conv.Exec(s.T().Context(), `INSERT INTO action_annotation_d (action_annotation_id,conversation_id,agent_user_id,conversation_start_time,action_type,knowledge_payload_knowledge_session_id) VALUES (?,?,'1',?,24,'session')`, uuid.NewString(), conversationID, start))
			}
			if tc.name == "MultipleFeatures" {
				conversationEvent(13, "1", 0)
				conversationEvent(4, "1", 0)
				conversationEvent(9, "1", 14)
			}
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(time.Hour))}}
			expected := &analyticspb.RetrieveConversationAssistanceUsageStatsResponse{TotalConversationCount: 1, ConversationsWithAssistanceUsedCount: tc.want, ConversationAssistanceUsageStatsResults: []*analyticspb.ConversationAssistanceUsageStatsResult{{TotalConversationCount: 1, ConversationsWithAssistanceUsedCount: tc.want}}}

			// When
			actual, err := s.server.RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			sharedassert.CmpEquals(s.T(), expected, actual)
		})
	}
}

func (s *ConversationAssistanceUsageTestSuite) TestRequiredEvidenceFailuresDiscardResults() {
	for _, query := range conversationAssistanceUsageEvidenceQueries(true /* includeBotConversations */) {
		for _, operation := range []string{"Query", "Scan", "Iteration"} {
			s.Run(query.name+"_"+operation, func() {
				// Given
				request := s.request()
				failure := errors.New("required evidence failed")
				s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(clickhousetesting.NewMockRows([][]any{{"c", "1", request.FilterByTimeRange.StartTimestamp.AsTime()}}), nil).Once()
				rows := &assistanceUsageIterationErrorRows{MockRows: clickhousetesting.NewMockRows([][]any{{"c", "1"}})}
				var queryError error
				switch operation {
				case "Query":
					queryError = failure
				case "Scan":
					rows.MockRows = clickhousetesting.NewMockRows([][]any{{"c"}})
				case "Iteration":
					rows.err = failure
				}
				s.convoMockConn.On("Query", mock.Anything, query.query, mock.Anything).Return(rows, queryError)
				s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(clickhousetesting.NewMockRows(nil), nil)

				// When
				response, err := s.server.RetrieveConversationAssistanceUsageStats(s.T().Context(), request)

				// Then
				s.Require().Error(err)
				s.Nil(response)
				s.ErrorContains(err, query.name)
				if operation != "Scan" {
					s.ErrorIs(err, failure)
				}
			})
		}
	}
}

func (s *ConversationAssistanceUsageValidationTestSuite) TestEvidenceQueriesAvoidGlobalBroadcast() {
	for _, includeBots := range []bool{false, true} {
		s.Run(fmt.Sprintf("IncludeBots_%t", includeBots), func() {
			// When
			queries := conversationAssistanceUsageEvidenceQueries(includeBots)

			// Then
			for _, query := range queries {
				s.NotContains(query.query, "GLOBAL", query.name)
			}
		})
	}
}

func (s *ConversationAssistanceUsageEvidenceCHTestSuite) TestLargeAgentPopulation() {
	// Given
	conversationID := uuid.NewString()
	agentUserID := uuid.NewString()
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	for _, pair := range []assistanceUsagePair{
		{conversationID, agentUserID},
		{uuid.NewString(), agentUserID},
		{conversationID, uuid.NewString()},
	} {
		s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,event_type) VALUES (?,?,?,13)`, uuid.NewString(), pair.conversationID, pair.agentUserID))
	}
	population := []conversationAssistanceUsagePopulationRow{{conversationID: conversationID, agentUserID: agentUserID}}
	for range 8000 {
		population = append(population, conversationAssistanceUsagePopulationRow{conversationID: uuid.NewString(), agentUserID: uuid.NewString()})
	}

	// When
	actual, err := s.server.readConversationAssistanceUsageEvidence(s.T().Context(), s.profileResourceName, population, true /* includeBotConversations */)

	// Then
	s.Require().NoError(err)
	s.ElementsMatch([]assistanceUsagePair{{conversationID, agentUserID}}, actual.Slice())
}

func (s *ConversationAssistanceUsageEvidenceCHTestSuite) TestMultipleEligibleAgents() {
	// Given
	conversationID := uuid.NewString()
	outsideConversationID := uuid.NewString()
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	for _, pair := range []assistanceUsagePair{
		{conversationID, "1"}, {conversationID, "2"}, {conversationID, "3"}, {outsideConversationID, "1"},
	} {
		s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,event_type) VALUES (?,?,?,13)`, uuid.NewString(), pair.conversationID, pair.agentUserID))
	}
	population := []conversationAssistanceUsagePopulationRow{
		{conversationID: conversationID, agentUserID: "1"},
		{conversationID: conversationID, agentUserID: "2"},
		{conversationID: conversationID, agentUserID: "1"},
	}

	// When
	actual, err := s.server.readConversationAssistanceUsageEvidence(s.T().Context(), s.profileResourceName, population, true /* includeBotConversations */)

	// Then
	s.Require().NoError(err)
	s.ElementsMatch([]assistanceUsagePair{{conversationID, "1"}, {conversationID, "2"}}, actual.Slice())
}
