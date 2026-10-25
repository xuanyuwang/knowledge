package analyticsimpl

import (
	"fmt"
	"strings"
	"testing"
	"time"

	nonpubconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	"github.com/cresta/go-servers/shared/auth"
	authmock "github.com/cresta/go-servers/shared/auth/mocks"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	dbtesting "github.com/cresta/go-servers/shared/db/testing"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type RetrieveKnowledgeAssistStatsClickhouseTestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite
	server                                     *AnalyticsServiceImpl
	customerID, profileID, profileResourceName string
	rpcACLMode                                 string
}

func TestRetrieveKnowledgeAssistStatsClickhouse(t *testing.T) {
	suite.Run(t, &RetrieveKnowledgeAssistStatsClickhouseTestSuite{})
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.customerID = "customer_id"
	s.server = &AnalyticsServiceImpl{appsDB: s.AppsDBRegistry(), logger: log.GetLogger(), clickhouseConvoRegistry: s.GetConversationTestRegistry()}
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) TearDownSuite() {
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
	s.DBTestSuite.TearDownSuite(s.T())
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) AfterTest(_, _ string) {
	s.DBTestSuite.AfterTest()
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) BeforeTest(_, _ string) {
	s.rpcACLMode = ""
	s.SetupSubTest()
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) SetupSubTest() {
	s.profileID = "usage_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	s.profileResourceName = fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	s.server.enableExtTableForUserFilter = false
	s.server.clickhouseConvoRegistry = s.GetConversationTestRegistry()
	s.setupRPCClients(s.rpcACLMode)
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) setupRPCClients(aclMode string) {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	configClient := mockconfig.NewMockClient(controller)
	customerConfig := &config.CustomerConfig{}
	if aclMode != "" {
		var err error
		customerConfig, err = config.NewCustomerConfig(&nonpubconfigpb.CustomerConfig{Customer: "customers/" + s.customerID, AuthConfig: &nonpubconfigpb.CustomerAuthConfig{EnableAdvancedDataAccessControl: true}})
		s.Require().NoError(err)
	}
	configClient.EXPECT().Get(s.customerID).AnyTimes().Return(customerConfig, nil)
	configClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	s.server.configClient = configClient
	aclProvider := authmock.NewMockResourceACLHelperProvider(controller)
	aclHelper := authmock.NewMockResourceACLHelper(controller)
	aclProvider.EXPECT().Get().AnyTimes().Return(aclHelper)
	s.server.resourceACLHelperProvider = aclProvider
	if aclMode != "" {
		acl := &auth.ResourceACL{}
		if aclMode == "restricted" {
			acl.ManagedUserIDs = []string{"b"}
		}
		aclHelper.EXPECT().GetResourceACL(gomock.Any(), s.customerID, gomock.Any()).Return(acl, nil)
	}
	users := []*internaluserpb.LiteUser{}
	for _, agent := range []string{"a", "b"} {
		users = append(users, &internaluserpb.LiteUser{UserId: agent, Username: "agent_" + agent, FullName: "Agent " + agent, Memberships: []*internaluserpb.UserGroupMembership{{Group: &internaluserpb.LiteGroup{GroupId: "team", DisplayName: "Team", GroupType: internaluserpb.LiteGroup_TEAM}, IsDirectMember: true}}})
	}
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{Parent: "customers/" + s.customerID, ProfileIds: []string{s.profileID}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	userClient := usermock.NewMockUserServiceClient(controller)
	userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{Parent: "customers/" + s.customerID, PageSize: 200, Filter: &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}}})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: []*userpb.Group{{Name: "customers/" + s.customerID + "/groups/team", Type: userpb.Group_TEAM, DisplayName: "Team"}}}, nil)
	for _, includeMemberships := range []bool{false, true} {
		userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{
			Parent: "customers/" + s.customerID, PageSize: 200,
			Filter:                  &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}, GroupIds: []string{"team"}},
			IncludeGroupMemberships: includeMemberships, IncludeIndirectGroupMemberships: includeMemberships,
		})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: []*userpb.Group{{Name: "customers/" + s.customerID + "/groups/team", Type: userpb.Group_TEAM, DisplayName: "Team"}}}, nil)
	}

	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](userClient)
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) insert(table, columns string, values ...any) {
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	columns = "customer_id, profile_id, conversation_duration_secs, usecase_id, create_time, update_time, " + columns
	args := []any{s.customerID, s.profileID, 60, "usage", time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC), time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)}
	args = append(args, values...)
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, columns, strings.TrimSuffix(strings.Repeat("?,", len(args)), ","))
	s.Require().NoError(conn.Exec(s.T().Context(), query, args...))
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) metadata(conversation, value string, updated time.Time) {
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO moment_annotation (customer_id,profile_id,conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,moment_template_id,metadata_string_value,create_time,update_time) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		s.customerID, s.profileID, conversation, "a", time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC), uuid.NewString(), int32(momentpb.Moment_CONVERSATION_METADATA), "tag", value, updated, updated))
}
func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) filter(exclude bool) *analyticspb.Attribute {
	moment := &momentpb.Moment{Name: s.profileResourceName + "/moments/tag", Type: momentpb.Moment_CONVERSATION_METADATA}
	group := &analyticspb.MomentGroup{}
	if exclude {
		group.ExcludedMoments = []*momentpb.Moment{moment}
	} else {
		group.Moments = []*momentpb.Moment{moment}
		group.MetadataValueAttributes = []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "yes"}}}}
	}
	return &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{group}}
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) TestReaderContracts() {
	start := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, kind              string
		viewed, used, questions int32
		results                 int
	}{
		{"DistinctSessionsAndQuestionIDs", "duplicates", 1, 1, 2, 1}, {"SessionDeduplicatesAcrossConversations", "crossconversation", 1, 1, 2, 1}, {"ViewedDoesNotMeanUsed", "viewed", 1, 0, 1, 1},
		{"SentDoesNotMeanUsed", "sent", 1, 0, 1, 1}, {"EmptySessionRejected", "emptysession", 0, 0, 0, 1},
		{"UseWithoutRetrieval", "useonly", 0, 1, 0, 1}, {"NoMessageDenominatorDropsUsage", "nomessage", 0, 0, 0, 0},
		{"MessageOnlyPreservesZeroDefaults", "messageonly", 0, 0, 0, 1}, {"SameConversationTwoAgents", "agents", 2, 2, 2, 2},
		{"InlineUserFilter", "inline", 1, 1, 1, 1}, {"ExternalUserFilter", "external", 1, 1, 1, 1},
		{"ProductionSourceEightAllowed", "source8", 1, 1, 1, 1}, {"OtherSourceExcluded", "source7", 0, 0, 0, 0}, {"DevUserExcluded", "dev", 0, 0, 0, 0},
		{"EmptyAgentExcluded", "emptyagent", 0, 0, 0, 0}, {"LatestMetadataInclude", "include", 1, 1, 1, 1}, {"StaleMetadataRejected", "stale", 0, 0, 0, 0}, {"ExcludedMetadata", "exclude", 0, 0, 0, 0}, {"MissingExcludedMetadataKeepsLegacyJoinDefaults", "excludemissing", 0, 0, 0, 0},
		{"HalfOpenTimeBoundsAndTimezone", "bounds", 1, 1, 1, 1}, {"TimeOnlyDenominator", "timeonly", 2, 2, 2, 1}, {"AgentOnlyGrouping", "agentonly", 1, 1, 1, 1}, {"DurationFilterExcludesAllRows", "duration", 0, 0, 0, 0}, {"UsecaseFilterExcludesAllRows", "usecase", 0, 0, 0, 0}, {"EmptyData", "empty", 0, 0, 0, 0},
	} {
		s.Run(tc.name, func() {
			// Given
			req := &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(24 * time.Hour))}, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}}
			agent := "a"
			if tc.kind == "emptyagent" {
				agent = ""
			}
			source := 0
			if tc.kind == "source8" {
				source = 8
			}
			if tc.kind == "source7" {
				source = 7
			}
			dev := tc.kind == "dev"
			session := "session"
			if tc.kind == "emptysession" {
				session = ""
			}
			message := func(id, user string, at time.Time) {
				s.insert("message", "conversation_id,agent_user_id,conversation_start_time,message_id,conversation_source,is_dev_user", id, user, at, uuid.NewString(), source, dev)
			}
			action := func(id, user, session string, at time.Time, kind int) {
				s.insert("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,action_type,knowledge_payload_knowledge_session_id,conversation_source,is_dev_user", id, user, at, uuid.NewString(), kind, session, source, dev)
			}
			used := func(id, user, session string, at time.Time, kind int) {
				s.insert("moment_annotation", "conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,knowledge_payload_knowledge_session_id,conversation_source,is_dev_user", id, user, at, uuid.NewString(), kind, session, source, dev)
			}
			seed := func(id, user, session string, at time.Time) {
				message(id, user, at)
				action(id, user, session, at, 22)
				action(id, user, session, at, 24)
				used(id, user, session, at, 38)
			}
			if tc.kind != "empty" {
				if tc.kind != "nomessage" {
					message("one", agent, start)
				}
				if tc.kind != "messageonly" {
					if tc.kind != "useonly" {
						action("one", agent, session, start, 22)
						action("one", agent, session, start, 24)
					}
					kind := 38
					if tc.kind == "sent" {
						kind = 39
					}
					if tc.kind != "viewed" {
						used("one", agent, session, start, kind)
					}
				}
			}
			if tc.kind == "sent" {
				action("one", agent, session, start, 25 /* retrieval answer */)
				action("one", agent, session, start, 26 /* retrieval reply */)
				for _, eventType := range []int{20, 21} {
					s.insert("conversation_event", "conversation_id,agent_user_id,conversation_start_time,event_id,event_type,knowledge_payload_knowledge_session_id", "one", agent, start, uuid.NewString(), eventType, session)
				}
			}
			if tc.kind == "duplicates" {
				message("one", agent, start)
				action("one", agent, session, start, 22)
				action("one", agent, session, start, 24)
				used("one", agent, session, start, 38)
			}
			if tc.kind == "crossconversation" {
				seed("two", "a", session, start)
			}
			if tc.kind == "agents" || tc.kind == "timeonly" {
				seed("one", "b", "second", start)
			}
			if tc.kind == "timeonly" {
				req.GroupByAttributeTypes = nil
			}
			if tc.kind == "agentonly" {
				req.Frequency = analyticspb.Frequency_FREQUENCY_UNSPECIFIED
			}
			if tc.kind == "inline" || tc.kind == "external" {
				seed("two", "b", "other", start)
				req.FilterByAttribute = &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + s.customerID + "/users/a"}}}
				s.server.enableExtTableForUserFilter = tc.kind == "external"
			}
			if tc.kind == "include" || tc.kind == "stale" {
				s.metadata("one", "yes", start)
				if tc.kind == "include" {
					s.metadata("one", "no", start.Add(-time.Hour))
					req.FilterByTimeRange.StartTimestamp = timestamppb.New(start.Add(-2 * time.Hour))
				} else {
					s.metadata("one", "no", start.Add(time.Hour))
				}
				req.FilterByAttribute = s.filter(false)
			}
			if tc.kind == "excludemissing" {
				req.FilterByAttribute = s.filter(true /* exclude */)
			}
			if tc.kind == "exclude" {
				s.metadata("one", "yes", start)
				req.FilterByAttribute = s.filter(true)
			}
			if tc.kind == "duration" {
				req.FilterByAttribute = &analyticspb.Attribute{ConversationDurationRange: &analyticspb.DurationRange{MinDuration: durationpb.New(61 * time.Second)}}
			}
			if tc.kind == "usecase" {
				req.FilterByAttribute = &analyticspb.Attribute{UsecaseNames: []string{s.profileResourceName + "/usecases/other"}}
			}
			if tc.kind == "bounds" {
				seed("before", "a", "before", start.Add(-time.Second))
				seed("end", "a", "end", start.Add(24*time.Hour))
			}

			// When
			got, err := s.server.readKnowledgeAssistStatsFromClickhouse(s.T().Context(), req, nil /* users */)

			// Then
			s.Require().NoError(err)
			want := &analyticspb.RetrieveKnowledgeAssistStatsResponse{GenaiAnswersViewedCount: tc.viewed, GenaiAnswersUsedCount: tc.used, KnowledgeMomentDetectedCount: tc.questions}
			if tc.results > 0 {
				want.AverageGenaiAnswerUsedCountPerFrequencyPerUser = float32(tc.used) / float32(tc.results)
				for i := 0; i < tc.results; i++ {
					view, use, question := tc.viewed, tc.used, tc.questions
					if tc.kind == "agents" {
						view, use, question = 1, 1, 1
					}
					attr := &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + s.customerID + "/users/" + []string{"a", "b"}[i]}}}
					bucket := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)
					if tc.kind == "agentonly" {
						bucket = time.Time{}
					}
					perUser := float32(use)
					if tc.kind == "timeonly" {
						attr = &analyticspb.Attribute{}
						perUser /= 2
					}
					stat := &analyticspb.KnowledgeAssistStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(bucket)}, GenaiAnswersViewedCount: view, GenaiAnswersUsedCount: use, KnowledgeMomentDetectedCount: question, AverageGenaiAnswerUsedCountPerFrequencyPerUser: perUser}
					want.KnowledgeAssistStatsResults = append(want.KnowledgeAssistStatsResults, &analyticspb.KnowledgeAssistStatsResult{Attribute: attr, KnowledgeAssistStats: []*analyticspb.KnowledgeAssistStats{stat}, GenaiAnswersViewedCount: view, GenaiAnswersUsedCount: use, KnowledgeMomentDetectedCount: question, AverageGenaiAnswerUsedCountPerFrequencyPerUser: float32(use)})
				}
			}
			s.Empty(cmp.Diff(want, got, protocmp.Transform(), protocmp.SortRepeated(func(a, b *analyticspb.KnowledgeAssistStatsResult) bool {
				return a.GetAttribute().String() < b.GetAttribute().String()
			})))
		})
	}
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) TestTimeGroupingDenominatorsAcrossDays() {
	// Given
	start := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		conversation, agent, session string
		at                           time.Time
	}{{"one", "a", "shared", start}, {"two", "b", "shared", start}, {"three", "a", "second", start.Add(24 * time.Hour)}} {
		s.insert("message", "conversation_id,agent_user_id,conversation_start_time,message_id", row.conversation, row.agent, row.at, uuid.NewString())
		for _, actionType := range []int{22, 24} {
			s.insert("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,action_type,knowledge_payload_knowledge_session_id", row.conversation, row.agent, row.at, uuid.NewString(), actionType, row.session)
		}
		s.insert("moment_annotation", "conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,knowledge_payload_knowledge_session_id", row.conversation, row.agent, row.at, uuid.NewString(), 38 /* reply used */, row.session)
	}
	req := &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(48 * time.Hour))}, Frequency: analyticspb.Frequency_DAILY, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}}
	wantStats := []*analyticspb.KnowledgeAssistStats{}
	for index, perUser := range []float32{0.5, 1} {
		wantStats = append(wantStats, &analyticspb.KnowledgeAssistStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2024, 1, 2+index, 10, 0, 0, 0, time.UTC))}, GenaiAnswersViewedCount: 1, GenaiAnswersUsedCount: 1, KnowledgeMomentDetectedCount: int32(2 - index), AverageGenaiAnswerUsedCountPerFrequencyPerUser: perUser})
	}
	want := &analyticspb.RetrieveKnowledgeAssistStatsResponse{GenaiAnswersViewedCount: 2, GenaiAnswersUsedCount: 2, KnowledgeMomentDetectedCount: 3, AverageGenaiAnswerUsedCountPerFrequencyPerUser: 2, KnowledgeAssistStatsResults: []*analyticspb.KnowledgeAssistStatsResult{{Attribute: &analyticspb.Attribute{}, KnowledgeAssistStats: wantStats, GenaiAnswersViewedCount: 2, GenaiAnswersUsedCount: 2, KnowledgeMomentDetectedCount: 3, AverageGenaiAnswerUsedCountPerFrequencyPerUser: 1}}}

	// When
	got, err := s.server.readKnowledgeAssistStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.Empty(cmp.Diff(want, got, protocmp.Transform()))
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) TestRPCRequestValidation() {
	for _, tc := range []struct {
		name    string
		request *analyticspb.RetrieveKnowledgeAssistStatsRequest
		message string
	}{
		{name: "MissingTimeRange", request: &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: s.profileResourceName}, message: "must set time_range for attribute"},
		{name: "InvalidParent", request: &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: "invalid"}, message: ""},
		{name: "TooManyGroupKeys", request: &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: s.profileResourceName, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE}}, message: "not supporting more than 2 grouping attribute types"},
	} {
		s.Run(tc.name, func() {
			// Given
			tc.request.Parent = s.profileResourceName
			if tc.name == "InvalidParent" {
				tc.request.Parent = "invalid"
			}

			// When
			response, err := s.server.RetrieveKnowledgeAssistStats(s.T().Context(), tc.request)

			// Then
			s.Nil(response)
			s.Equal(codes.InvalidArgument, status.Code(err))
			if tc.message != "" {
				s.ErrorContains(err, tc.message)
			}
		})
	}
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) TestRPCSeededResponse() {
	for _, tc := range []struct {
		name, shape, aclMode, filter string
		external                     bool
	}{
		{name: "DailyAgents", shape: "daily"},
		{name: "DailyAgentsExternal", shape: "daily", external: true},
		{name: "SelectedUser", shape: "selected"},
		{name: "SelectedUserExternal", shape: "selected", external: true},
		{name: "UnknownSelectedUser", shape: "empty"},
		{name: "WeeklyAgents", shape: "weekly"},
		{name: "AgentOnly", shape: "agentonly"},
		{name: "TimeOnly", shape: "timeonly"},
		{name: "DailyGroup", shape: "group"},
		{name: "DailyAgentAndGroup", shape: "agentgroup"},
		{name: "RestrictedACL", shape: "selected", aclMode: "restricted"},
		{name: "DeniedACL", shape: "empty", aclMode: "denied"},
		{name: "MetadataInclude", shape: "first", filter: "include"},
		{name: "MetadataIncludeAndExclude", shape: "empty", filter: "includeexclude"},
		{name: "DurationRange", shape: "empty", filter: "duration"},
		{name: "CombinedAttributeFilter", shape: "daily", filter: "attributes"},
	} {
		s.rpcACLMode = tc.aclMode
		s.Run(tc.name, func() {
			// Given
			start := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
			for _, row := range []struct {
				conversation, agent string
				at                  time.Time
				used                bool
			}{
				{"first", "a", start, true}, {"second", "b", start, true}, {"third", "a", start.Add(24 * time.Hour), false},
			} {
				s.insert("message", "conversation_id,agent_user_id,conversation_start_time,message_id", row.conversation, row.agent, row.at, uuid.NewString())
				for _, kind := range []int{22, 24} {
					s.insert("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,action_type,knowledge_payload_knowledge_session_id", row.conversation, row.agent, row.at, uuid.NewString(), kind, row.conversation)
				}
				if row.used {
					s.insert("moment_annotation", "conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,knowledge_payload_knowledge_session_id", row.conversation, row.agent, row.at, uuid.NewString(), 38 /* reply used */, row.conversation)
				}
			}
			req := &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(48 * time.Hour))}, FilterByAttribute: &analyticspb.Attribute{}, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}}
			s.server.enableExtTableForUserFilter = tc.external
			switch tc.shape {
			case "selected":
				if tc.aclMode == "" {
					req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/b"}}
				}
			case "weekly":
				req.Frequency = analyticspb.Frequency_WEEKLY
			case "agentonly":
				req.Frequency = analyticspb.Frequency_FREQUENCY_UNSPECIFIED
			case "timeonly":
				req.GroupByAttributeTypes = nil
			case "group":
				req.FilterByAttribute.Groups = []*userpb.Group{{Name: "customers/" + s.customerID + "/groups/team"}}
				req.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP}
			case "agentgroup":
				req.FilterByAttribute.Groups = []*userpb.Group{{Name: "customers/" + s.customerID + "/groups/team"}}
				req.GroupByAttributeTypes = append(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)
			}
			if tc.name == "UnknownSelectedUser" {
				req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/missing"}}
			}
			switch tc.filter {
			case "include", "includeexclude":
				s.metadata("first", "yes", start)
				req.FilterByAttribute = s.filter(false /* exclude */)
				if tc.filter == "includeexclude" {
					req.FilterByAttribute.MomentGroups = append(req.FilterByAttribute.MomentGroups, &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{{Name: s.profileResourceName + "/moments/blocked", Type: momentpb.Moment_CONVERSATION_METADATA}}})
				}
			case "attributes":
				req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/a"}, {Name: "customers/" + s.customerID + "/users/b"}}
				req.FilterByAttribute.UsecaseNames = []string{s.profileResourceName + "/usecases/usage"}
				req.FilterByAttribute.ConversationDurationRange = &analyticspb.DurationRange{MinDuration: durationpb.New(10 * time.Second), MaxDuration: durationpb.New(100 * time.Second)}
			case "duration":
				req.FilterByAttribute.ConversationDurationRange = &analyticspb.DurationRange{MinDuration: durationpb.New(61 * time.Second), MaxDuration: durationpb.New(100 * time.Second)}
			}
			expected := s.rpcExpectedResponse(tc.shape)

			// When
			response, err := s.server.RetrieveKnowledgeAssistStats(s.T().Context(), req)

			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(expected, response, protocmp.Transform(), protocmp.SortRepeated(func(a, b *analyticspb.KnowledgeAssistStatsResult) bool {
				return a.GetAttribute().String() < b.GetAttribute().String()
			})))
		})
	}
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) rpcExpectedResponse(shape string) *analyticspb.RetrieveKnowledgeAssistStatsResponse {
	user := func(agent string) *userpb.User {
		return &userpb.User{Name: "customers/" + s.customerID + "/users/" + agent, Username: "agent_" + agent, FullName: "Agent " + agent}
	}
	team := &userpb.Group{Name: "customers/" + s.customerID + "/groups/team", DisplayName: "Team", Type: userpb.Group_TEAM}
	at := func(day int) *analyticspb.TimeRange {
		return &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2024, 1, day, 10, 0, 0, 0, time.UTC))}
	}
	bucket := func(timeRange *analyticspb.TimeRange, viewed, used, users int32) *analyticspb.KnowledgeAssistStats {
		return &analyticspb.KnowledgeAssistStats{TimeRange: timeRange, GenaiAnswersViewedCount: viewed, GenaiAnswersUsedCount: used, KnowledgeMomentDetectedCount: viewed, AverageGenaiAnswerUsedCountPerFrequencyPerUser: float32(used) / float32(users)}
	}
	result := func(attr *analyticspb.Attribute, stats []*analyticspb.KnowledgeAssistStats, viewed, used, users int32) *analyticspb.KnowledgeAssistStatsResult {
		return &analyticspb.KnowledgeAssistStatsResult{Attribute: attr, KnowledgeAssistStats: stats, GenaiAnswersViewedCount: viewed, GenaiAnswersUsedCount: used, KnowledgeMomentDetectedCount: viewed, AverageGenaiAnswerUsedCountPerFrequencyPerUser: float32(used) / float32(users)}
	}
	response := func(viewed, used, users int32, results ...*analyticspb.KnowledgeAssistStatsResult) *analyticspb.RetrieveKnowledgeAssistStatsResponse {
		return &analyticspb.RetrieveKnowledgeAssistStatsResponse{KnowledgeAssistStatsResults: results, GenaiAnswersViewedCount: viewed, GenaiAnswersUsedCount: used, KnowledgeMomentDetectedCount: viewed, AverageGenaiAnswerUsedCountPerFrequencyPerUser: float32(used) / float32(users)}
	}
	alice := &analyticspb.Attribute{Users: []*userpb.User{user("a")}}
	bob := &analyticspb.Attribute{Users: []*userpb.User{user("b")}}
	firstA := bucket(at(2), 1 /* viewed */, 1 /* used */, 1 /* users */)
	secondA := bucket(at(3), 1 /* viewed */, 0 /* used */, 1 /* users */)
	firstB := bucket(at(2), 1 /* viewed */, 1 /* used */, 1 /* users */)
	switch shape {
	case "empty":
		return &analyticspb.RetrieveKnowledgeAssistStatsResponse{}
	case "selected":
		return response(1 /* viewed */, 1 /* used */, 1 /* users */, result(bob, []*analyticspb.KnowledgeAssistStats{firstB}, 1 /* viewed */, 1 /* used */, 1 /* users */))
	case "first":
		return response(1 /* viewed */, 1 /* used */, 1 /* users */, result(alice, []*analyticspb.KnowledgeAssistStats{firstA}, 1 /* viewed */, 1 /* used */, 1 /* users */))
	case "group", "timeonly":
		attr := &analyticspb.Attribute{}
		resultUsers, responseUsers := int32(2), int32(1)
		if shape == "group" {
			attr.Groups = []*userpb.Group{team}
			resultUsers = 3
			responseUsers = 2
		}
		return response(3 /* viewed */, 2 /* used */, responseUsers, result(attr, []*analyticspb.KnowledgeAssistStats{bucket(at(2), 2 /* viewed */, 2 /* used */, 2 /* users */), secondA}, 3 /* viewed */, 2 /* used */, resultUsers))
	case "weekly", "agentonly":
		timeRange := &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Time{})}
		if shape == "weekly" {
			timeRange.StartTimestamp = timestamppb.New(time.Date(2023, 12, 31, 10, 0, 0, 0, time.UTC))
		}
		return response(3 /* viewed */, 2 /* used */, 2, /* users */
			result(alice, []*analyticspb.KnowledgeAssistStats{bucket(timeRange, 2 /* viewed */, 1 /* used */, 1 /* users */)}, 2 /* viewed */, 1 /* used */, 1 /* users */),
			result(bob, []*analyticspb.KnowledgeAssistStats{bucket(timeRange, 1 /* viewed */, 1 /* used */, 1 /* users */)}, 1 /* viewed */, 1 /* used */, 1 /* users */))
	case "daily", "agentgroup":
		if shape == "agentgroup" {
			alice.Groups = []*userpb.Group{proto.Clone(team).(*userpb.Group)}
			bob.Groups = []*userpb.Group{proto.Clone(team).(*userpb.Group)}
		}
		return response(3 /* viewed */, 2 /* used */, 2, /* users */
			result(alice, []*analyticspb.KnowledgeAssistStats{firstA, secondA}, 2 /* viewed */, 1 /* used */, 2 /* users */),
			result(bob, []*analyticspb.KnowledgeAssistStats{firstB}, 1 /* viewed */, 1 /* used */, 1 /* users */))
	}
	s.T().Fatalf("unexpected expectation shape %s", shape)
	return nil
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) setupOriginalSQLGoldenUsers() {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	groups := []*userpb.Group{
		{Name: "customers/" + s.customerID + "/groups/group-1", DisplayName: "Group 1", Type: userpb.Group_TEAM},
		{Name: "customers/" + s.customerID + "/groups/group-2", DisplayName: "Group 2", Type: userpb.Group_TEAM},
	}
	users := []*internaluserpb.LiteUser{}
	for _, agent := range []string{"1", "2"} {
		groupIndex := 1
		if agent == "1" {
			groupIndex = 0
		}
		users = append(users, &internaluserpb.LiteUser{UserId: agent, Username: "agent_" + agent, FullName: "Agent " + agent, Memberships: []*internaluserpb.UserGroupMembership{{Group: &internaluserpb.LiteGroup{GroupId: fmt.Sprintf("group-%d", groupIndex+1), DisplayName: groups[groupIndex].DisplayName, GroupType: internaluserpb.LiteGroup_TEAM}, IsDirectMember: true}}})
	}
	userRequest := &internaluserpb.ListUsersForAnalyticsRequest{Parent: "customers/" + s.customerID, ProfileIds: []string{s.profileID}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true}
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(userRequest)).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	groupClient := usermock.NewMockUserServiceClient(controller)
	groupRequest := &userpb.ListGroupsRequest{Parent: "customers/" + s.customerID, PageSize: 200, Filter: &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}}}
	groupClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(groupRequest)).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: groups}, nil)

	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](groupClient)
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) TestRPCOriginalSQLGoldens() {
	for _, tc := range []struct{ name, golden, kind string }{{"CombinedFilters", "Basic", "attributes"},
		{"AgentOnly", "AgentOnly", "agentonly"},
		{"MetadataIncludeAndExclude", "FilterByMetadataMomentGroups_IncludeAndExcludeMoments", "metadata"}} {
		s.Run(tc.name, func() {
			// Given
			s.setupOriginalSQLGoldenUsers()
			start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			if tc.kind == "metadata" {
				end = time.Date(2021, 1, 11, 0, 0, 0, 0, time.UTC)
			}
			at := start.Add(36 * time.Hour)
			req := &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(end)}, FilterByAttribute: &analyticspb.Attribute{}, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}}
			rowArgs := []any{"2021-01-01 00:00:00", end.Format("2006-01-02 15:04:05")}

			switch tc.kind {
			case "attributes":
				req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/1"}, {Name: "customers/" + s.customerID + "/users/2"}}
				req.FilterByAttribute.ConversationDurationRange = &analyticspb.DurationRange{MinDuration: durationpb.New(10 * time.Second), MaxDuration: durationpb.New(100 * time.Second)}
				rowArgs = append(rowArgs, []string{"1", "2"})

				rowArgs = append(rowArgs, float64(10), float64(100))
			case "agentonly":
				req.Frequency = analyticspb.Frequency_FREQUENCY_UNSPECIFIED
				req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/1"}, {Name: "customers/" + s.customerID + "/users/2"}}
				rowArgs = append(rowArgs, []string{"1", "2"})
			case "metadata":
				req.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{{Name: "customers/test_customer/profiles/test_profile/moments/moment_id_1", Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "include-value-1"}}}}},
					{ExcludedMoments: []*momentpb.Moment{{Name: "customers/test_customer/profiles/test_profile/moments/moment_id_2", Type: momentpb.Moment_CONVERSATION_METADATA}}},
				}
				req.Metadata = nil
			}
			expectedArgs := []any{}
			if tc.kind == "metadata" {
				expectedArgs = append(expectedArgs, "2021-01-01 00:00:00", end.Format("2006-01-02 15:04:05"), "moment_id_1", "2021-01-01 00:00:00", end.Format("2006-01-02 15:04:05"), "moment_id_1", "include-value-1", "2021-01-01 00:00:00", end.Format("2006-01-02 15:04:05"), "moment_id_2")
			}
			for range 3 {
				expectedArgs = append(expectedArgs, rowArgs...)
			}
			connection := s.GetConversationConnection(s.customerID, s.profileID)
			seed := func(table, columns string, values ...any) {
				columns = "customer_id,profile_id,conversation_duration_secs,usecase_id,create_time,update_time," + columns
				args := append([]any{s.customerID, s.profileID, 60, "test_usecase", at, at}, values...)
				s.Require().NoError(connection.Exec(s.T().Context(), fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, columns, strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")), args...))
			}
			for _, agent := range []string{"1", "2"} {
				seed("message", "conversation_id,agent_user_id,conversation_start_time,message_id", "one-"+agent, agent, at, uuid.NewString())
				for _, kind := range []int{22, 24} {
					seed("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,action_type,knowledge_payload_knowledge_session_id", "one-"+agent, agent, at, uuid.NewString(), kind, "session-"+agent)
				}
				seed("moment_annotation", "conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,knowledge_payload_knowledge_session_id", "one-"+agent, agent, at, uuid.NewString(), 38 /* reply used */, "session-"+agent)
			}
			if tc.kind == "metadata" {
				seed("moment_annotation", "conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,moment_template_id,metadata_string_value", "one-1", "1", at, uuid.NewString(), 19 /* metadata */, "moment_id_1", "include-value-1")
			}
			expected := s.originalSQLGoldenResponse(tc.kind, at)
			recording := recordConversationSQL(s.server, connection, s.customerID, s.profileID)

			// When
			response, err := s.server.RetrieveKnowledgeAssistStats(s.T().Context(), req)

			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(expected, response, protocmp.Transform(), protocmp.SortRepeated(func(a, b *analyticspb.KnowledgeAssistStatsResult) bool {
				return a.GetAttribute().String() < b.GetAttribute().String()
			})))
			assertRecordedSQLGolden(s.T(), recording, "clickhouse_RetrieveKnowledgeAssistStats_"+tc.golden+"_request.sql", expectedArgs)
		})
	}
}

func (s *RetrieveKnowledgeAssistStatsClickhouseTestSuite) originalSQLGoldenResponse(kind string, at time.Time) *analyticspb.RetrieveKnowledgeAssistStatsResponse {
	response := &analyticspb.RetrieveKnowledgeAssistStatsResponse{}
	if kind == "metadata" {
		return response
	}
	bucketAt := time.Date(at.Year(), 1, 2, 0, 0, 0, 0, time.UTC)
	if kind == "agentonly" {
		bucketAt = time.Time{}
	}
	agents := []string{"1", "2"}
	for _, agent := range agents {
		attribute := &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + s.customerID + "/users/" + agent, Username: "agent_" + agent, FullName: "Agent " + agent}}}

		response.KnowledgeAssistStatsResults = append(response.KnowledgeAssistStatsResults, &analyticspb.KnowledgeAssistStatsResult{Attribute: attribute, KnowledgeAssistStats: []*analyticspb.KnowledgeAssistStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(bucketAt)}, GenaiAnswersViewedCount: 1, GenaiAnswersUsedCount: 1, KnowledgeMomentDetectedCount: 1, AverageGenaiAnswerUsedCountPerFrequencyPerUser: 1}}, GenaiAnswersViewedCount: 1, GenaiAnswersUsedCount: 1, KnowledgeMomentDetectedCount: 1, AverageGenaiAnswerUsedCountPerFrequencyPerUser: 1})
	}
	response.GenaiAnswersViewedCount = 2
	response.GenaiAnswersUsedCount = 2
	response.KnowledgeMomentDetectedCount = 2
	response.AverageGenaiAnswerUsedCountPerFrequencyPerUser = 1
	return response
}
