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

type RetrieveSummarizationStatsTestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite
	server                                     *AnalyticsServiceImpl
	customerID, profileID, profileResourceName string
	rpcACLMode                                 string
}

func TestRetrieveSummarizationStats(t *testing.T) {
	suite.Run(t, &RetrieveSummarizationStatsTestSuite{})
}
func (s *RetrieveSummarizationStatsTestSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.customerID = "customer_id"
	s.server = &AnalyticsServiceImpl{appsDB: s.AppsDBRegistry(), logger: log.GetLogger(), clickhouseConvoRegistry: s.GetConversationTestRegistry()}
}
func (s *RetrieveSummarizationStatsTestSuite) TearDownSuite() {
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
	s.DBTestSuite.TearDownSuite(s.T())
}
func (s *RetrieveSummarizationStatsTestSuite) AfterTest(_, _ string) { s.DBTestSuite.AfterTest() }
func (s *RetrieveSummarizationStatsTestSuite) BeforeTest(_, _ string) {
	s.rpcACLMode = ""
	s.SetupSubTest()
}
func (s *RetrieveSummarizationStatsTestSuite) SetupSubTest() {
	s.profileID = "usage_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	s.profileResourceName = fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	s.server.enableExtTableForUserFilter = false
	s.server.clickhouseConvoRegistry = s.GetConversationTestRegistry()
	s.setupRPCClients(s.rpcACLMode)
}

func (s *RetrieveSummarizationStatsTestSuite) setupRPCClients(aclMode string) {
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
func (s *RetrieveSummarizationStatsTestSuite) insert(table, columns string, values ...any) {
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	columns = "customer_id, profile_id, conversation_duration_secs, usecase_id, create_time, update_time, " + columns
	args := []any{s.customerID, s.profileID, 60, "usage", time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC), time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)}
	args = append(args, values...)
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, columns, strings.TrimSuffix(strings.Repeat("?,", len(args)), ","))
	s.Require().NoError(conn.Exec(s.T().Context(), query, args...))
}
func (s *RetrieveSummarizationStatsTestSuite) metadata(conversation, value string, updated time.Time) {
	conn := s.GetConversationConnection(s.customerID, s.profileID)
	s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO moment_annotation (customer_id,profile_id,conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,moment_template_id,metadata_string_value,create_time,update_time) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		s.customerID, s.profileID, conversation, "a", time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC), uuid.NewString(), int32(momentpb.Moment_CONVERSATION_METADATA), "tag", value, updated, updated))
}
func (s *RetrieveSummarizationStatsTestSuite) filter(exclude bool) *analyticspb.Attribute {
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

func (s *RetrieveSummarizationStatsTestSuite) TestReaderContracts() {
	start := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                           string
		generated, used, conversations int32
		kind                           string
	}{
		{"GeneratedAndCopyAll", 1, 1, 1, "copyall"}, {"CopySummary", 1, 1, 1, "copysummary"},
		{"GeneratedWithoutCopy", 1, 0, 1, "generated"}, {"CopyWithoutValidGeneration", 0, 0, 1, "invalid"},
		{"WrongEventAndCopyType", 1, 0, 1, "wrong"}, {"DuplicateGenerationAndCopies", 1, 1, 1, "duplicates"},
		{"CopyDifferentAgent", 1, 0, 1, "mismatchagent"}, {"CopyDifferentStart", 1, 0, 1, "mismatchstart"}, {"ConversationDenominatorWithoutActivity", 1, 1, 2, "denominator"},
		{"SameConversationTwoAgents", 2, 2, 2, "agents"}, {"InlineUserFilter", 1, 1, 1, "inline"}, {"ExternalUserFilter", 1, 1, 1, "external"},
		{"LatestMetadataInclude", 1, 1, 1, "include"}, {"StaleMetadataRejected", 0, 0, 0, "stale"}, {"ExcludedMetadata", 0, 0, 0, "exclude"}, {"MissingExcludedMetadataKeepsLegacyJoinDefaults", 0, 0, 0, "excludemissing"},
		{"SourceAndDevAreNotSummaryGates", 1, 1, 1, "source"}, {"EmptyAgentExcluded", 0, 0, 0, "emptyagent"},
		{"HalfOpenTimeBoundsAndTimezone", 1, 1, 1, "bounds"}, {"DurationFilterExcludesAllRows", 0, 0, 0, "duration"}, {"UsecaseFilterExcludesAllRows", 0, 0, 0, "usecase"}, {"EmptyData", 0, 0, 0, "empty"},
	} {
		s.Run(tc.name, func() {
			// Given
			req := &analyticspb.RetrieveSummarizationStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(24 * time.Hour))}, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}}
			agent := "a"
			if tc.kind == "emptyagent" {
				agent = ""
			}
			conversation := func(id, user string, at time.Time) {
				s.insert("conversation", "conversation_id,agent_user_id,conversation_start_time", id, user, at)
			}
			generated := func(id, user string, at time.Time, valid bool) {
				s.insert("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,summarization_payload_is_valid_summary", id, user, at, uuid.NewString(), valid)
			}
			copied := func(id, user string, at time.Time, event, datatype int) {
				s.insert("conversation_event", "conversation_id,agent_user_id,conversation_start_time,event_id,event_type,summarization_payload_data_type", id, user, at, uuid.NewString(), event, datatype)
			}
			if tc.kind != "empty" {
				conversation("one", agent, start)
				generated("one", agent, start, tc.kind != "invalid")
			}
			if tc.kind != "empty" && tc.kind != "generated" {
				event, datatype := 16, 1
				if tc.kind == "copysummary" {
					datatype = 2
				}
				if tc.kind == "wrong" {
					event = 15
				}
				copyAgent, copyStart := agent, start
				if tc.kind == "mismatchagent" {
					copyAgent = "b"
				}
				if tc.kind == "mismatchstart" {
					copyStart = start.Add(time.Hour)
				}
				copied("one", copyAgent, copyStart, event, datatype)
				if tc.kind == "wrong" {
					copied("one", agent, start, 16, 3)
				}
			}
			if tc.kind == "duplicates" {
				generated("one", agent, start, true)
				copied("one", agent, start, 16, 2)
			}
			if tc.kind == "denominator" {
				conversation("two", "a", start)
			}
			if tc.kind == "agents" {
				conversation("one", "b", start)
				generated("one", "b", start, true)
				copied("one", "b", start, 16, 1)
			}
			if tc.kind == "inline" || tc.kind == "external" {
				conversation("two", "b", start)
				generated("two", "b", start, true)
				copied("two", "b", start, 16, 1)
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
			if tc.kind == "source" {
				conn := s.GetConversationConnection(s.customerID, s.profileID)
				for _, table := range []string{"conversation", "action_annotation", "conversation_event"} {
					s.Require().NoError(conn.Exec(s.T().Context(), "ALTER TABLE "+table+" UPDATE conversation_source=7,is_dev_user=1 WHERE conversation_id='one' SETTINGS mutations_sync=2"))
				}
			}
			if tc.kind == "duration" {
				req.FilterByAttribute = &analyticspb.Attribute{ConversationDurationRange: &analyticspb.DurationRange{MinDuration: durationpb.New(61 * time.Second)}}
			}
			if tc.kind == "usecase" {
				req.FilterByAttribute = &analyticspb.Attribute{UsecaseNames: []string{s.profileResourceName + "/usecases/other"}}
			}
			if tc.kind == "bounds" {
				for i, at := range []time.Time{start.Add(-time.Second), start.Add(24 * time.Hour)} {
					id := fmt.Sprint("outside", i)
					conversation(id, "a", at)
					generated(id, "a", at, true)
					copied(id, "a", at, 16, 1)
				}
			}

			// When
			got, err := s.server.readSummarizationStatsFromClickhouse(s.T().Context(), req, nil /* users */)

			// Then
			s.Require().NoError(err)
			want := &analyticspb.RetrieveSummarizationStatsResponse{}
			if tc.conversations > 0 {
				agents := []string{"a"}
				if tc.kind == "agents" {
					agents = append(agents, "b")
				}
				want.TotalConversationCount = tc.conversations
				want.TotalSummarizationGeneratedCount = tc.generated
				want.TotalSummarizationUsedCount = tc.used
				ratio := float32(0)
				if tc.generated > 0 {
					ratio = float32(tc.used) / float32(tc.generated) * 100
				}
				want.AverageSummarizationUsedRatio = ratio
				want.AverageSummarizationUsedCountPerConversation = float32(tc.used) / float32(tc.conversations)
				want.AverageSummarizationUsedCountPerFrequencyPerUser = float32(tc.used) / float32(len(agents))
				want.AverageSummarizationGeneratedCountPerFrequencyPerUser = float32(tc.generated) / float32(len(agents))
				for _, user := range agents {
					conv, gen, use := tc.conversations, tc.generated, tc.used
					if tc.kind == "agents" {
						conv, gen, use = 1, 1, 1
					}
					stat := &analyticspb.SummarizationStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))}, TotalConversationCount: conv, TotalSummarizationGeneratedCount: gen, TotalSummarizationUsedCount: use, AverageSummarizationUsedRatio: ratio, AverageSummarizationUsedCountPerConversation: float32(use) / float32(conv), AverageSummarizationUsedCountPerUser: float32(use), AverageSummarizationGeneratedCountPerUser: float32(gen)}
					want.SummarizationStatsResults = append(want.SummarizationStatsResults, &analyticspb.SummarizationStatsResult{Attribute: &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + s.customerID + "/users/" + user}}}, SummarizationStats: []*analyticspb.SummarizationStats{stat}, TotalConversationCount: conv, TotalSummarizationGeneratedCount: gen, TotalSummarizationUsedCount: use, AverageSummarizationUsedRatio: ratio, AverageSummarizationUsedCountPerConversation: float32(use) / float32(conv), AverageSummarizationUsedCountPerFrequencyPerUser: float32(use), AverageSummarizationGeneratedCountPerFrequencyPerUser: float32(gen)})
				}
			}
			s.Empty(cmp.Diff(want, got, protocmp.Transform(), protocmp.SortRepeated(func(a, b *analyticspb.SummarizationStatsResult) bool {
				return a.GetAttribute().String() < b.GetAttribute().String()
			})))
		})
	}
}

func (s *RetrieveSummarizationStatsTestSuite) TestTimeGroupingDenominatorsAcrossDays() {
	// Given
	start := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		conversation, agent string
		at                  time.Time
	}{{"one", "a", start}, {"one", "b", start}, {"two", "a", start.Add(24 * time.Hour)}} {
		s.insert("conversation", "conversation_id,agent_user_id,conversation_start_time", row.conversation, row.agent, row.at)
		s.insert("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,summarization_payload_is_valid_summary", row.conversation, row.agent, row.at, uuid.NewString(), true /* valid */)
		s.insert("conversation_event", "conversation_id,agent_user_id,conversation_start_time,event_id,event_type,summarization_payload_data_type", row.conversation, row.agent, row.at, uuid.NewString(), 16 /* used summary */, 1 /* copy all */)
	}
	req := &analyticspb.RetrieveSummarizationStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(48 * time.Hour))}, Frequency: analyticspb.Frequency_DAILY, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}}
	wantStats := []*analyticspb.SummarizationStats{}
	for index, perUser := range []float32{0.5, 1} {
		wantStats = append(wantStats, &analyticspb.SummarizationStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2024, 1, 2+index, 10, 0, 0, 0, time.UTC))}, TotalConversationCount: 1, TotalSummarizationGeneratedCount: 1, TotalSummarizationUsedCount: 1, AverageSummarizationUsedRatio: 100, AverageSummarizationUsedCountPerConversation: 1, AverageSummarizationUsedCountPerUser: perUser, AverageSummarizationGeneratedCountPerUser: perUser})
	}
	want := &analyticspb.RetrieveSummarizationStatsResponse{TotalConversationCount: 2, TotalSummarizationGeneratedCount: 2, TotalSummarizationUsedCount: 2, AverageSummarizationUsedRatio: 100, AverageSummarizationUsedCountPerConversation: 1, AverageSummarizationUsedCountPerFrequencyPerUser: float32(2) / 3, AverageSummarizationGeneratedCountPerFrequencyPerUser: float32(2) / 3, SummarizationStatsResults: []*analyticspb.SummarizationStatsResult{{Attribute: &analyticspb.Attribute{}, SummarizationStats: wantStats, TotalConversationCount: 2, TotalSummarizationGeneratedCount: 2, TotalSummarizationUsedCount: 2, AverageSummarizationUsedRatio: 100, AverageSummarizationUsedCountPerConversation: 1, AverageSummarizationUsedCountPerFrequencyPerUser: float32(2) / 3, AverageSummarizationGeneratedCountPerFrequencyPerUser: float32(2) / 3}}}

	// When
	got, err := s.server.readSummarizationStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.Empty(cmp.Diff(want, got, protocmp.Transform()))
}

func (s *RetrieveSummarizationStatsTestSuite) TestRPCRequestValidation() {
	for _, tc := range []struct {
		name    string
		request *analyticspb.RetrieveSummarizationStatsRequest
		message string
	}{
		{name: "MissingTimeRange", request: &analyticspb.RetrieveSummarizationStatsRequest{Parent: s.profileResourceName}, message: "must set time_range for attribute"},
		{name: "InvalidParent", request: &analyticspb.RetrieveSummarizationStatsRequest{Parent: "invalid"}, message: ""},
		{name: "TooManyGroupKeys", request: &analyticspb.RetrieveSummarizationStatsRequest{Parent: s.profileResourceName, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE}}, message: "not supporting more than 2 grouping attribute types"},
	} {
		s.Run(tc.name, func() {
			// Given
			tc.request.Parent = s.profileResourceName
			if tc.name == "InvalidParent" {
				tc.request.Parent = "invalid"
			}

			// When
			response, err := s.server.RetrieveSummarizationStats(s.T().Context(), tc.request)

			// Then
			s.Nil(response)
			s.Equal(codes.InvalidArgument, status.Code(err))
			if tc.message != "" {
				s.ErrorContains(err, tc.message)
			}
		})
	}
}

func (s *RetrieveSummarizationStatsTestSuite) TestRPCSeededResponse() {
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
				s.insert("conversation", "conversation_id,agent_user_id,conversation_start_time", row.conversation, row.agent, row.at)
				s.insert("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,summarization_payload_is_valid_summary", row.conversation, row.agent, row.at, uuid.NewString(), true /* valid */)
				if row.used {
					s.insert("conversation_event", "conversation_id,agent_user_id,conversation_start_time,event_id,event_type,summarization_payload_data_type", row.conversation, row.agent, row.at, uuid.NewString(), 16 /* used summary */, 1 /* copy all */)
				}
			}
			req := &analyticspb.RetrieveSummarizationStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(48 * time.Hour))}, FilterByAttribute: &analyticspb.Attribute{}, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}}
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
			response, err := s.server.RetrieveSummarizationStats(s.T().Context(), req)

			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(expected, response, protocmp.Transform(), protocmp.SortRepeated(func(a, b *analyticspb.SummarizationStatsResult) bool {
				return a.GetAttribute().String() < b.GetAttribute().String()
			})))
		})
	}
}

func (s *RetrieveSummarizationStatsTestSuite) rpcExpectedResponse(shape string) *analyticspb.RetrieveSummarizationStatsResponse {
	user := func(agent string) *userpb.User {
		return &userpb.User{Name: "customers/" + s.customerID + "/users/" + agent, Username: "agent_" + agent, FullName: "Agent " + agent}
	}
	team := &userpb.Group{Name: "customers/" + s.customerID + "/groups/team", DisplayName: "Team", Type: userpb.Group_TEAM}
	at := func(day int) *analyticspb.TimeRange {
		return &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2024, 1, day, 10, 0, 0, 0, time.UTC))}
	}
	bucket := func(timeRange *analyticspb.TimeRange, generated, used, users int32) *analyticspb.SummarizationStats {
		return &analyticspb.SummarizationStats{
			TimeRange: timeRange, TotalConversationCount: generated, TotalSummarizationGeneratedCount: generated, TotalSummarizationUsedCount: used,
			AverageSummarizationUsedRatio:                float32(used) / float32(generated) * 100,
			AverageSummarizationUsedCountPerConversation: float32(used) / float32(generated),
			AverageSummarizationUsedCountPerUser:         float32(used) / float32(users), AverageSummarizationGeneratedCountPerUser: float32(generated) / float32(users),
		}
	}
	result := func(attr *analyticspb.Attribute, stats []*analyticspb.SummarizationStats, generated, used, users int32) *analyticspb.SummarizationStatsResult {
		return &analyticspb.SummarizationStatsResult{
			Attribute: attr, SummarizationStats: stats, TotalConversationCount: generated, TotalSummarizationGeneratedCount: generated, TotalSummarizationUsedCount: used,
			AverageSummarizationUsedRatio:                    float32(used) / float32(generated) * 100,
			AverageSummarizationUsedCountPerConversation:     float32(used) / float32(generated),
			AverageSummarizationUsedCountPerFrequencyPerUser: float32(used) / float32(users), AverageSummarizationGeneratedCountPerFrequencyPerUser: float32(generated) / float32(users),
		}
	}
	response := func(generated, used, users int32, results ...*analyticspb.SummarizationStatsResult) *analyticspb.RetrieveSummarizationStatsResponse {
		return &analyticspb.RetrieveSummarizationStatsResponse{
			SummarizationStatsResults: results, TotalConversationCount: generated, TotalSummarizationGeneratedCount: generated, TotalSummarizationUsedCount: used,
			AverageSummarizationUsedRatio:                    float32(used) / float32(generated) * 100,
			AverageSummarizationUsedCountPerConversation:     float32(used) / float32(generated),
			AverageSummarizationUsedCountPerFrequencyPerUser: float32(used) / float32(users), AverageSummarizationGeneratedCountPerFrequencyPerUser: float32(generated) / float32(users),
		}
	}
	alice := &analyticspb.Attribute{Users: []*userpb.User{user("a")}}
	bob := &analyticspb.Attribute{Users: []*userpb.User{user("b")}}
	firstA := bucket(at(2), 1 /* generated */, 1 /* used */, 1 /* users */)
	secondA := bucket(at(3), 1 /* generated */, 0 /* used */, 1 /* users */)
	firstB := bucket(at(2), 1 /* generated */, 1 /* used */, 1 /* users */)
	switch shape {
	case "empty":
		return &analyticspb.RetrieveSummarizationStatsResponse{}
	case "selected":
		return response(1 /* generated */, 1 /* used */, 1 /* users */, result(bob, []*analyticspb.SummarizationStats{firstB}, 1 /* generated */, 1 /* used */, 1 /* users */))
	case "first":
		return response(1 /* generated */, 1 /* used */, 1 /* users */, result(alice, []*analyticspb.SummarizationStats{firstA}, 1 /* generated */, 1 /* used */, 1 /* users */))
	case "group", "timeonly":
		attr := &analyticspb.Attribute{}
		resultUsers, responseUsers := int32(3), int32(3)
		if shape == "group" {
			attr.Groups = []*userpb.Group{team}
			resultUsers = 3
			responseUsers = 3
		}
		return response(3 /* generated */, 2 /* used */, responseUsers, result(attr, []*analyticspb.SummarizationStats{bucket(at(2), 2 /* generated */, 2 /* used */, 2 /* users */), secondA}, 3 /* generated */, 2 /* used */, resultUsers))
	case "weekly", "agentonly":
		timeRange := &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Time{})}
		if shape == "weekly" {
			timeRange.StartTimestamp = timestamppb.New(time.Date(2023, 12, 31, 10, 0, 0, 0, time.UTC))
		}
		return response(3 /* generated */, 2 /* used */, 2, /* users */
			result(alice, []*analyticspb.SummarizationStats{bucket(timeRange, 2 /* generated */, 1 /* used */, 1 /* users */)}, 2 /* generated */, 1 /* used */, 1 /* users */),
			result(bob, []*analyticspb.SummarizationStats{bucket(timeRange, 1 /* generated */, 1 /* used */, 1 /* users */)}, 1 /* generated */, 1 /* used */, 1 /* users */))
	case "daily", "agentgroup":
		if shape == "agentgroup" {
			alice.Groups = []*userpb.Group{proto.Clone(team).(*userpb.Group)}
			bob.Groups = []*userpb.Group{proto.Clone(team).(*userpb.Group)}
		}
		return response(3 /* generated */, 2 /* used */, 3, /* users */
			result(alice, []*analyticspb.SummarizationStats{firstA, secondA}, 2 /* generated */, 1 /* used */, 2 /* users */),
			result(bob, []*analyticspb.SummarizationStats{firstB}, 1 /* generated */, 1 /* used */, 1 /* users */))
	}
	s.T().Fatalf("unexpected expectation shape %s", shape)
	return nil
}

func (s *RetrieveSummarizationStatsTestSuite) setupOriginalSQLGoldenUsers(grouped bool) {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	groups := []*userpb.Group{
		{Name: "customers/" + s.customerID + "/groups/group-1", DisplayName: "Group 1", Type: userpb.Group_TEAM},
		{Name: "customers/" + s.customerID + "/groups/group-2", DisplayName: "Group 2", Type: userpb.Group_TEAM},
	}
	users := []*internaluserpb.LiteUser{}
	for _, agent := range []string{"1", "2", "3", "4"} {
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
	if grouped {
		for _, includeMemberships := range []bool{false, true} {
			selectedRequest := proto.Clone(groupRequest).(*userpb.ListGroupsRequest)
			selectedRequest.Filter.GroupIds = []string{"group-1", "group-2"}
			selectedRequest.IncludeGroupMemberships = includeMemberships
			selectedRequest.IncludeIndirectGroupMemberships = includeMemberships
			groupClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(selectedRequest)).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: groups}, nil)
		}
	}
	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](groupClient)
}

func (s *RetrieveSummarizationStatsTestSuite) TestRPCOriginalSQLGoldens() {
	for _, tc := range []struct{ name, golden, kind string }{{"SelectedAgent", "FilterAndGroupByAgent", "selected"},
		{"CombinedFilters", "BasicFilterAndGroupByAgent", "attributes"},
		{"SelectedGroups", "FilterAndGroupByGroups", "groups"},
		{"WeeklyAgents", "WeeklyGroupByAgent", "weekly"},
		{"MetadataIncludeAndExclude", "FilterByMetadataMomentGroups_IncludeAndExcludeMoments", "metadata"}} {
		s.Run(tc.name, func() {
			// Given
			s.setupOriginalSQLGoldenUsers(tc.kind == "groups")
			start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
			if tc.kind == "metadata" {
				end = time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)
			}
			at := start.Add(36 * time.Hour)
			req := &analyticspb.RetrieveSummarizationStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(end)}, FilterByAttribute: &analyticspb.Attribute{}, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}}
			rowArgs := []any{"2024-01-01 00:00:00", end.Format("2006-01-02 15:04:05")}
			req.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}
			switch tc.kind {
			case "selected":
				req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/1"}}
				rowArgs = append(rowArgs, []string{"1"})
			case "attributes":
				req.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/1"}, {Name: "customers/" + s.customerID + "/users/2"}}
				req.FilterByAttribute.ConversationDurationRange = &analyticspb.DurationRange{MinDuration: durationpb.New(10 * time.Second), MaxDuration: durationpb.New(100 * time.Second)}
				rowArgs = append(rowArgs, []string{"1", "2"})
				req.FilterByAttribute.UsecaseNames = []string{"customers/test_customer/profiles/test_profile/usecases/test_usecase"}
				rowArgs = append(rowArgs, []string{"test_usecase"})
				rowArgs = append(rowArgs, float64(10), float64(100))
			case "groups":
				req.FilterByAttribute.Groups = []*userpb.Group{{Name: "customers/" + s.customerID + "/groups/group-1"}, {Name: "customers/" + s.customerID + "/groups/group-2"}}
				req.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP}
				rowArgs = append(rowArgs, []string{"1", "2", "3", "4"})
			case "weekly":
				req.Frequency = analyticspb.Frequency_WEEKLY
				req.Metadata = nil
			case "metadata":
				req.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{{Name: "customers/test_customer/profiles/test_profile/moments/moment_id_1", Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "include-value-1"}}}}},
					{ExcludedMoments: []*momentpb.Moment{{Name: "customers/test_customer/profiles/test_profile/moments/moment_id_2", Type: momentpb.Moment_CONVERSATION_METADATA}}},
				}
				req.Metadata = nil
			}
			expectedArgs := []any{}
			if tc.kind == "metadata" {
				expectedArgs = append(expectedArgs, "2024-01-01 00:00:00", end.Format("2006-01-02 15:04:05"), "moment_id_1", "2024-01-01 00:00:00", end.Format("2006-01-02 15:04:05"), "moment_id_1", "include-value-1", "2024-01-01 00:00:00", end.Format("2006-01-02 15:04:05"), "moment_id_2")
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
				seed("conversation", "conversation_id,agent_user_id,conversation_start_time", "one-"+agent, agent, at)
				seed("action_annotation", "conversation_id,agent_user_id,conversation_start_time,action_annotation_id,summarization_payload_is_valid_summary", "one-"+agent, agent, at, uuid.NewString(), true /* valid */)
				seed("conversation_event", "conversation_id,agent_user_id,conversation_start_time,event_id,event_type,summarization_payload_data_type", "one-"+agent, agent, at, uuid.NewString(), 16 /* used summary */, 1 /* copy all */)
			}
			if tc.kind == "metadata" {
				seed("moment_annotation", "conversation_id,agent_user_id,conversation_start_time,moment_annotation_id,moment_type,moment_template_id,metadata_string_value", "one-1", "1", at, uuid.NewString(), 19 /* metadata */, "moment_id_1", "include-value-1")
			}
			expected := s.originalSQLGoldenResponse(tc.kind, at)
			recording := recordConversationSQL(s.server, connection, s.customerID, s.profileID)

			// When
			response, err := s.server.RetrieveSummarizationStats(s.T().Context(), req)

			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(expected, response, protocmp.Transform(), protocmp.SortRepeated(func(a, b *analyticspb.SummarizationStatsResult) bool {
				return a.GetAttribute().String() < b.GetAttribute().String()
			})))
			assertRecordedSQLGolden(s.T(), recording, "clickhouse_RetrieveSummarizationStats_"+tc.golden+"_request.sql", expectedArgs)
		})
	}
}

func (s *RetrieveSummarizationStatsTestSuite) originalSQLGoldenResponse(kind string, at time.Time) *analyticspb.RetrieveSummarizationStatsResponse {
	response := &analyticspb.RetrieveSummarizationStatsResponse{}
	if kind == "metadata" {
		return response
	}
	bucketAt := time.Date(at.Year(), 1, 2, 10, 0, 0, 0, time.UTC)
	if kind == "weekly" {
		bucketAt = time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	agents := []string{"1", "2"}
	if kind == "selected" {
		agents = []string{"1"}
	}
	for _, agent := range agents {
		attribute := &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + s.customerID + "/users/" + agent, Username: "agent_" + agent, FullName: "Agent " + agent}}}
		if kind == "groups" {
			attribute = &analyticspb.Attribute{Groups: []*userpb.Group{{Name: "customers/" + s.customerID + "/groups/group-" + agent, DisplayName: "Group " + agent, Type: userpb.Group_TEAM}}}
		}
		response.SummarizationStatsResults = append(response.SummarizationStatsResults, &analyticspb.SummarizationStatsResult{
			Attribute: attribute, SummarizationStats: []*analyticspb.SummarizationStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(bucketAt)}, TotalConversationCount: 1, TotalSummarizationGeneratedCount: 1, TotalSummarizationUsedCount: 1, AverageSummarizationUsedRatio: 100, AverageSummarizationUsedCountPerConversation: 1, AverageSummarizationUsedCountPerUser: 1, AverageSummarizationGeneratedCountPerUser: 1}},
			TotalConversationCount: 1, TotalSummarizationGeneratedCount: 1, TotalSummarizationUsedCount: 1, AverageSummarizationUsedRatio: 100, AverageSummarizationUsedCountPerConversation: 1, AverageSummarizationUsedCountPerFrequencyPerUser: 1, AverageSummarizationGeneratedCountPerFrequencyPerUser: 1,
		})
	}
	response.TotalConversationCount = int32(len(agents))
	response.TotalSummarizationGeneratedCount = int32(len(agents))
	response.TotalSummarizationUsedCount = int32(len(agents))
	response.AverageSummarizationUsedRatio = 100
	response.AverageSummarizationUsedCountPerConversation = 1
	response.AverageSummarizationUsedCountPerFrequencyPerUser = 1
	response.AverageSummarizationGeneratedCountPerFrequencyPerUser = 1
	return response
}
