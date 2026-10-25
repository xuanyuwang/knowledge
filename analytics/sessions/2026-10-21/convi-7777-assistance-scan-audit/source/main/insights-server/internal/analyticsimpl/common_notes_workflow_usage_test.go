package analyticsimpl

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	authmock "github.com/cresta/go-servers/shared/auth/mocks"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	dbtesting "github.com/cresta/go-servers/shared/db/testing"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type notesWorkflowStatsSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite
	server                                                          *AnalyticsServiceImpl
	customerID, profileID, profileResourceName, losAngeles          string
	start, janFirst, janSeventh, janTenth                           time.Time
	janFirstProto, janSeventhProto, janTenthProto, janEleventhProto timestamppb.Timestamp
	group1, group2                                                  *userpb.Group
}

func (s *notesWorkflowStatsSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.customerID = "customer_id"
	s.losAngeles = "Pacific/Honolulu"
	s.janFirst = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	s.janSeventh = s.janFirst.Add(6 * 24 * time.Hour)
	s.janTenth = s.janFirst.Add(9 * 24 * time.Hour)
	s.janFirstProto, s.janSeventhProto, s.janTenthProto = *timestamppb.New(s.janFirst), *timestamppb.New(s.janSeventh), *timestamppb.New(s.janTenth)
	s.janEleventhProto = *timestamppb.New(s.janFirst.Add(10 * 24 * time.Hour))
	s.group1 = &userpb.Group{Name: "customers/customer_id/groups/group-1", DisplayName: "Group 1", Type: userpb.Group_TEAM}
	s.group2 = &userpb.Group{Name: "customers/customer_id/groups/group-2", DisplayName: "Group 2", Type: userpb.Group_TEAM}
	s.server = &AnalyticsServiceImpl{appsDB: s.DBTestSuite.AppsDBRegistry(), logger: log.GetLogger(), clickhouseConvoRegistry: s.GetConversationTestRegistry(), clickhouseEventRegistry: s.GetEventsTestRegistry()}
}

func (s *notesWorkflowStatsSuite) TearDownSuite() {
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
	s.DBTestSuite.TearDownSuite(s.T())
}
func (s *notesWorkflowStatsSuite) AfterTest(_, _ string) { s.DBTestSuite.AfterTest() }

func (s *notesWorkflowStatsSuite) SetupTest() {
	s.profileID = "notes-workflow-" + uuid.NewString()
	s.profileResourceName = fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	s.start = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	s.server.enableExtTableForUserFilter = false
	s.setupRPCClients()
}

func (s *notesWorkflowStatsSuite) SetupSubTest() { s.SetupTest() }

func (s *notesWorkflowStatsSuite) connection(metadata bool) chdriver.Conn {
	if metadata {
		return s.GetConversationConnection(s.customerID, s.profileID)
	}
	return s.GetEventsConnection(s.customerID, s.profileID)
}

func (s *notesWorkflowStatsSuite) conversation(metadata bool, conversationID, agentID string, start time.Time) {
	s.Require().NoError(s.connection(metadata).Exec(s.T().Context(), `INSERT INTO conversation_d
 (conversation_id, agent_user_id, conversation_start_time, conversation_end_time, customer_id, profile_id, create_time, update_time, conversation_duration_secs)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 60)`, conversationID, agentID, start, start.Add(time.Hour), s.customerID, s.profileID, s.start, s.start))
}

type notesWorkflowEvent struct {
	conversationID, agentID, eventID, eventName, payload, document, sessionID string
	eventType, noteDataType, docType                                          int32
	start                                                                     time.Time
}

func (s *notesWorkflowStatsSuite) event(metadata bool, event notesWorkflowEvent) {
	if event.start.IsZero() {
		event.start = s.start
	}
	if event.payload == "" {
		event.payload = `{}`
	}
	if metadata {
		s.Require().NoError(s.connection(true).Exec(s.T().Context(), `INSERT INTO conversation_event_d
 (conversation_id, agent_user_id, conversation_start_time, customer_id, profile_id, create_time, update_time, event_id, event_type,
 `+"`payload.used_note_taking.data_type`, `payload.used_kb_document.doc_id`, `payload.used_kb_document.doc_type`, `payload.used_kb_document.session_id`"+`)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.conversationID, event.agentID, event.start, s.customerID, s.profileID, s.start, s.start, event.eventID, event.eventType, event.noteDataType, event.document, event.docType, event.sessionID))
		return
	}
	s.Require().NoError(s.connection(false).Exec(s.T().Context(), `INSERT INTO conversation_event_d
 (conversation_id, agent_user_id, conversation_start_time, customer_id, profile_id, create_time, event_id, event_type, event_name, event_payload)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.conversationID, event.agentID, event.start, s.customerID, s.profileID, s.start, event.eventID, event.eventType, event.eventName, event.payload))
}

func (s *notesWorkflowStatsSuite) metadata(conversationID, momentID, value string, updated time.Time) {
	s.Require().NoError(s.connection(true).Exec(s.T().Context(), `INSERT INTO moment_annotation_d
 (conversation_id, agent_user_id, conversation_start_time, customer_id, profile_id, create_time, update_time, moment_annotation_id, moment_template_id, moment_type, metadata_string_value)
 VALUES (?, '1', ?, ?, ?, ?, ?, ?, ?, 19, ?)`, conversationID, s.start, s.customerID, s.profileID, updated, updated, uuid.NewString(), momentID, value))
}

func (s *notesWorkflowStatsSuite) metadataFilter(exclude bool) *analyticspb.Attribute {
	moment := func(id string) *momentpb.Moment {
		return &momentpb.Moment{Name: s.profileResourceName + "/moments/" + id, Type: momentpb.Moment_CONVERSATION_METADATA}
	}
	groups := []*analyticspb.MomentGroup{{Moments: []*momentpb.Moment{moment("include")}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "selected"}}}}}}
	if exclude {
		groups = append(groups, &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{moment("exclude")}})
	}
	return &analyticspb.Attribute{MomentGroups: groups}
}

func (s *notesWorkflowStatsSuite) timeRange() *analyticspb.TimeRange {
	return &analyticspb.TimeRange{StartTimestamp: timestamppb.New(s.start), EndTimestamp: timestamppb.New(s.start.Add(48 * time.Hour))}
}

func (s *notesWorkflowStatsSuite) userFilter(agentID string) *analyticspb.Attribute {
	return &analyticspb.Attribute{Users: []*userpb.User{{Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, agentID)}}}
}

func guidedWorkflowUsagePayload(document, docType string, session any) string {
	data := map[string]any{"document": document, "docType": docType}
	if session != nil {
		data["sessionMetadata"] = map[string]any{"sessionId": session}
	}
	payload, _ := json.Marshal(map[string]any{"usedKbDocument": data})
	return string(payload)
}

func (s *notesWorkflowStatsSuite) setupRPCClients() {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	configClient := mockconfig.NewMockClient(controller)
	configClient.EXPECT().Get(s.customerID).AnyTimes().Return(&config.CustomerConfig{}, nil)
	configClient.EXPECT().GetProfileConfig(s.customerID, s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	s.server.configClient = configClient
	aclProvider := authmock.NewMockResourceACLHelperProvider(controller)
	aclProvider.EXPECT().Get().AnyTimes().Return(authmock.NewMockResourceACLHelper(controller))
	s.server.resourceACLHelperProvider = aclProvider
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	groups := []*internaluserpb.LiteGroup{{GroupId: "group-1", DisplayName: "Group 1", GroupType: internaluserpb.LiteGroup_TEAM}, {GroupId: "group-2", DisplayName: "Group 2", GroupType: internaluserpb.LiteGroup_TEAM}}
	users := []*internaluserpb.LiteUser{
		{UserId: "1", Username: "agent_a", FullName: "agent a", Memberships: []*internaluserpb.UserGroupMembership{{Group: groups[0], IsDirectMember: true}}},
		{UserId: "2", Username: "agent_b", FullName: "agent b", Memberships: []*internaluserpb.UserGroupMembership{{Group: groups[1], IsDirectMember: true}}},
	}
	for _, groupIDs := range [][]string{nil, {"group-1"}, {"group-2"}, {"group-1", "group-2"}} {
		selected := []*internaluserpb.LiteUser{}
		for _, user := range users {
			if len(groupIDs) == 0 || slices.Contains(groupIDs, user.Memberships[0].Group.GroupId) {
				selected = append(selected, user)
			}
		}
		internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
			Parent: "customers/" + s.customerID, ProfileIds: []string{s.profileID}, GroupIds: groupIDs, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true,
		})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: selected}, nil)
	}
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	userClient := usermock.NewMockUserServiceClient(controller)
	for _, request := range []struct {
		groupIDs           []string
		includeMemberships bool
	}{
		{groupIDs: nil, includeMemberships: false},
		{groupIDs: []string{"group-1", "group-2"}, includeMemberships: false},
		{groupIDs: []string{"group-1", "group-2"}, includeMemberships: true},
	} {
		userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{
			Parent: "customers/" + s.customerID, PageSize: 200,
			Filter:                  &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupIds: request.groupIDs, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}},
			IncludeGroupMemberships: request.includeMemberships, IncludeIndirectGroupMemberships: request.includeMemberships,
		})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: []*userpb.Group{s.group1, s.group2}}, nil)
	}

	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](userClient)
}

func (s *notesWorkflowStatsSuite) seedRPCEvents(workflow, responseFixture, metadata bool) {
	type rpcEventFixture struct {
		id, agent string
		at        time.Time
		uses      int
	}
	rows := []rpcEventFixture{{"one", "1", s.janFirst.Add(2 * time.Hour), 1}, {"two", "2", s.janFirst.Add(2 * time.Hour), 1}}
	if responseFixture {
		rows[0].uses = 2
		rows = append(rows, rpcEventFixture{"three", "2", s.janSeventh.Add(2 * time.Hour), 1}, rpcEventFixture{"four", "2", s.janSeventh.Add(2 * time.Hour), 0})
	}
	for _, row := range rows {
		s.conversation(metadata, row.id, row.agent, row.at)
		if metadata {
			s.metadata(row.id, "include", "selected", s.start)
		}
		eventCount := row.uses
		if eventCount == 0 {
			eventCount = 1
		}
		for index := 0; index < eventCount; index++ {
			event := notesWorkflowEvent{conversationID: row.id, agentID: row.agent, eventID: fmt.Sprintf("%s-%d", row.id, index), start: row.at, eventType: 5, eventName: "VoiceAgentAssist.NoteTaking.Click.CopyEntity", noteDataType: 1}
			if workflow {
				event.eventType, event.document, event.docType = 4, "gw"+row.agent, 2
				event.sessionID = event.eventID
				event.payload = guidedWorkflowUsagePayload(event.document, "2", event.sessionID)
			}
			if row.uses == 0 {
				event.eventType, event.eventName = 1, "unrelated"
			}
			s.event(metadata, event)
		}
	}
}

func (s *notesWorkflowStatsSuite) TearDownSubTest() {
	s.server.clickhouseConvoRegistry = s.GetConversationTestRegistry()
	s.server.clickhouseEventRegistry = s.GetEventsTestRegistry()
}

func (s *notesWorkflowStatsSuite) seedSQLGolden(workflow, grouped, metadata bool) *chtesting.RecordingConnection {
	agents := []string{"1", "2"}
	if grouped {
		agents = append(agents, "3", "4")
		s.setupSQLGoldenGroupUsers()
	}
	for _, agent := range agents {
		conversationID := "golden-" + agent
		at := s.janFirst.Add(2 * time.Hour)
		s.conversation(metadata, conversationID, agent, at)
		event := notesWorkflowEvent{conversationID: conversationID, agentID: agent, eventID: conversationID, start: at, eventType: 5, eventName: "VoiceAgentAssist.NoteTaking.Click.CopyEntity", noteDataType: 1}
		if workflow {
			event.eventType, event.document, event.docType, event.sessionID = 4, "gw"+agent, 2, conversationID
			event.payload = guidedWorkflowUsagePayload(event.document, "2", event.sessionID)
		}
		s.event(metadata, event)
		if metadata {
			s.metadata(conversationID, "moment_id_1", "include-value-1", s.start)
		}
	}
	if metadata {
		return recordConversationSQL(s.server, s.connection(true /* metadata */), s.customerID, s.profileID)
	}
	recording := chtesting.NewRecordingConnection(s.connection(false /* metadata */))
	s.server.clickhouseEventRegistry = &sqlGoldenConversationRegistry{
		ConversationsClickhouseRegistry: s.server.clickhouseEventRegistry,
		recording:                       recording, customerID: s.customerID, profileID: s.profileID,
	}
	return recording
}

func (s *notesWorkflowStatsSuite) setupSQLGoldenGroupUsers() {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	users := []*internaluserpb.LiteUser{}
	for index, agent := range []string{"1", "2", "3", "4"} {
		groupID, displayName := "group-2", "Group 2"
		if agent == "1" {
			groupID, displayName = "group-1", "Group 1"
		}
		users = append(users, &internaluserpb.LiteUser{UserId: agent, Username: "agent_" + string(rune('a'+index)), FullName: "agent " + string(rune('a'+index)), Memberships: []*internaluserpb.UserGroupMembership{{Group: &internaluserpb.LiteGroup{GroupId: groupID, DisplayName: displayName, GroupType: internaluserpb.LiteGroup_TEAM}, IsDirectMember: true}}})
	}
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	for _, groupIDs := range [][]string{nil, {"group-1", "group-2"}} {
		internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{Parent: "customers/" + s.customerID, ProfileIds: []string{s.profileID}, GroupIds: groupIDs, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	}
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
}

func notesWorkflowGoldenArgs(description string) []any {
	start, end := "2024-01-01 00:00:00", "2024-01-10 00:00:00"
	if description == "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments" {
		end = "2024-01-11 00:00:00"
		return []any{start, end, "moment_id_1", start, end, "moment_id_1", "include-value-1", start, end, "moment_id_2", start, end, start, end}
	}
	conversationArgs, eventArgs := []any{start, end}, []any{start, end}
	switch description {
	case "Test_DailyFilterAndGroupByAgent":
		conversationArgs = append(conversationArgs, []string{"1"})
		eventArgs = append(eventArgs, []string{"1"})
	case "Test_BasicFilterAndGroupByAgent":
		conversationArgs = append(conversationArgs, []string{"1", "2"}, float64(10), float64(100))
		eventArgs = append(eventArgs, []string{"1", "2"})
	case "Test_DailyFilterAndGroupByGroup":
		conversationArgs = append(conversationArgs, []string{"1", "2", "3", "4"})
		eventArgs = append(eventArgs, []string{"1", "2", "3", "4"})
	}
	return append(conversationArgs, eventArgs...)
}
