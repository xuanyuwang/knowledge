package analyticsimpl

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	nonpubconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	"github.com/cresta/go-servers/shared/auth"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	chregistry "github.com/cresta/go-servers/shared/clickhouse/registry"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	sharedassert "github.com/cresta/go-servers/shared/testing/assert"
	sharedmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type WorkflowAutomationQuerySuite struct {
	suite.Suite
	chtesting.ClickhouseContainerTestSuite
	connection         driver.Conn
	admin              driver.Conn
	database           string
	native             bool
	service            AnalyticsServiceImpl
	configClient       *mockconfig.MockClient
	aclHelper          *authmocks.MockResourceACLHelper
	internalUserClient *internalusermock.MockInternalUserServiceClient
	userClient         *usermock.MockUserServiceClient
}

type workflowAutomationTestRegistry struct {
	chregistry.ConversationsClickhouseRegistry
	connection driver.Conn
}

func (r workflowAutomationTestRegistry) GetConnection(context.Context, string, string) (driver.Conn, error) {
	return r.connection, nil
}

func (s *WorkflowAutomationQuerySuite) SetupSubTest() {
	ctrl := gomock.NewController(sharedmock.NewTestReporter(s.T()))
	s.configClient = mockconfig.NewMockClient(ctrl)
	s.aclHelper = authmocks.NewMockResourceACLHelper(ctrl)
	s.internalUserClient = internalusermock.NewMockInternalUserServiceClient(ctrl)
	s.userClient = usermock.NewMockUserServiceClient(ctrl)
	provider := authmocks.NewMockResourceACLHelperProvider(ctrl)
	provider.EXPECT().Get().Return(s.aclHelper).AnyTimes()
	s.service = AnalyticsServiceImpl{
		logger:                            log.GetLogger(),
		configClient:                      s.configClient,
		resourceACLHelperProvider:         provider,
		internalUserServiceClientRegistry: clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](s.internalUserClient),
		userClientRegistry:                clientregistry.NewTestRegistry[userpb.UserServiceClient](s.userClient),
		clickhouseConvoRegistry:           workflowAutomationTestRegistry{connection: s.connection},
	}
}

func (s *WorkflowAutomationQuerySuite) TestHandlerAuthorization() {
	for _, testCase := range []struct {
		name                    string
		acl                     auth.ResourceACL
		externalUsers           bool
		aclError                error
		conversations, triggers int64
		bucketConversations     [3]int64
		bucketTriggers          [3]int64
	}{
		{name: "LimitedACL", acl: auth.ResourceACL{ManagedUserIDs: []string{"u2"}}, conversations: 1, bucketConversations: [3]int64{0, 1}},
		{name: "LimitedACLExternalTable", acl: auth.ResourceACL{ManagedUserIDs: []string{"u2"}}, externalUsers: true, conversations: 1, bucketConversations: [3]int64{0, 1}},
		{name: "EmptyACL"},
		{name: "EmptyACLExternalTable", externalUsers: true},
		{name: "RootAccess", acl: auth.ResourceACL{IsRootAccess: true}, conversations: 4, triggers: 4, bucketConversations: [3]int64{2, 2}, bucketTriggers: [3]int64{2, 2}},
		{name: "RootAccessExternalTable", acl: auth.ResourceACL{IsRootAccess: true}, externalUsers: true, conversations: 4, triggers: 4, bucketConversations: [3]int64{2, 2}, bucketTriggers: [3]int64{2, 2}},
		{name: "ACLError", aclError: status.Error(codes.PermissionDenied, "ACL denied")},
	} {
		s.Run(testCase.name, func() {
			// Given
			s.service.enableExtTableForUserFilter = testCase.externalUsers
			customerConfig, err := config.NewCustomerConfig(&nonpubconfigpb.CustomerConfig{Customer: "customers/c", AuthConfig: &nonpubconfigpb.CustomerAuthConfig{EnableAdvancedDataAccessControl: true}})
			s.Require().NoError(err)
			s.configClient.EXPECT().Get("c").Return(customerConfig, nil)
			s.aclHelper.EXPECT().GetResourceACL(gomock.Any(), "c", gomock.Any()).Return(&testCase.acl, testCase.aclError)
			s.internalUserClient.EXPECT().ListUsersForAnalytics(gomock.Any(), sharedmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent: "customers/c", ProfileIds: []string{"p"}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true,
			})).Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: []*internaluserpb.LiteUser{{UserId: "u1"}, {UserId: "u2"}, {UserId: "u3"}}}, nil)
			if testCase.conversations > 0 {
				s.userClient.EXPECT().ListGroups(gomock.Any(), sharedmock.ProtoEq(&userpb.ListGroupsRequest{
					Parent: "customers/c", PageSize: 200,
					Filter: &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{"customers/c/profiles/p"}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}},
				})).Return(&userpb.ListGroupsResponse{}, nil)
			} else {
				// An empty ACL or ACL failure must return before accessing ClickHouse.
				s.service.clickhouseConvoRegistry = nil
			}
			start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			request := &analyticspb.RetrieveWorkflowAutomationStatsRequest{
				Parent: "customers/c/profiles/p", Frequency: analyticspb.Frequency_DAILY,
				FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.AddDate(0, 0, 3))},
			}
			original := proto.Clone(request)

			// When
			response, err := s.service.RetrieveWorkflowAutomationStats(s.T().Context(), request)

			// Then
			sharedassert.CmpEquals(s.T(), original, request)
			if testCase.aclError != nil {
				s.Equal(codes.PermissionDenied, status.Code(err))
				s.ErrorContains(err, "resolving workflow automation analytics users")
				s.Nil(response)
				return
			}
			s.Require().NoError(err)
			s.Equal(testCase.conversations, response.GetAggregatedWorkflowAutomationStats().GetTotalConversationCount())
			s.Equal(testCase.triggers, response.GetAggregatedWorkflowAutomationStats().GetTotalWorkflowAutomationTriggerCount())
			s.Require().Len(response.GetWorkflowAutomationStatsResults(), 1)
			result := response.GetWorkflowAutomationStatsResults()[0]
			sharedassert.CmpEquals(s.T(), response.GetAggregatedWorkflowAutomationStats(), result.GetAggregatedWorkflowAutomationStats())
			s.Require().Len(result.GetWorkflowAutomationStats(), 3)
			for index, bucket := range result.GetWorkflowAutomationStats() {
				sharedassert.CmpEquals(s.T(), &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start.AddDate(0, 0, index)), EndTimestamp: timestamppb.New(start.AddDate(0, 0, index+1))}, bucket.GetTimeRange())
				s.Equal(testCase.bucketConversations[index], bucket.GetAggregatedWorkflowAutomationStats().GetTotalConversationCount())
				s.Equal(testCase.bucketTriggers[index], bucket.GetAggregatedWorkflowAutomationStats().GetTotalWorkflowAutomationTriggerCount())
			}
		})
	}
}

func TestWorkflowAutomationQuerySuite(t *testing.T) { suite.Run(t, new(WorkflowAutomationQuerySuite)) }
func (s *WorkflowAutomationQuerySuite) SetupSuite() {
	address := os.Getenv("WFA_ANALYTICS_TEST_CLICKHOUSE_ADDRESS")
	if address == "" {
		s.ClickhouseContainerTestSuite.SetupSuite(s.T())
		s.connection = s.GetConversationConnection("wfa-analytics-test", "p")
		s.seedWorkflowAutomationFacts()
		return
	}
	// Optional local override; CI uses the standard ClickHouse container suite.
	s.native = true
	s.database = "wfa_analytics_" + uuid.NewString()[:8]
	var err error
	s.admin, err = clickhouse.Open(&clickhouse.Options{Addr: []string{address}})
	s.Require().NoError(err)
	s.Require().NoError(s.admin.Exec(s.T().Context(), "CREATE DATABASE "+s.database))
	s.connection, err = clickhouse.Open(&clickhouse.Options{Addr: []string{address}, Auth: clickhouse.Auth{Database: s.database}})
	s.Require().NoError(err)
	for _, query := range []string{
		"CREATE TABLE conversation_d (customer_id String,profile_id String,conversation_id String,agent_user_id String,conversation_start_time DateTime64(6, 'UTC'),update_time DateTime64(6, 'UTC')) ENGINE=Memory",
		"CREATE TABLE conversation_event_d (customer_id String,profile_id String,conversation_id String,conversation_start_time DateTime64(6, 'UTC'),event_id String,event_type Int32,update_time DateTime64(6, 'UTC'),`payload.workflow_automation.workflow_automation_run_id` String,`payload.workflow_automation.workflow_automation_definition_id` String,`payload.workflow_automation.workflow_automation_definition_version` Int32,`payload.workflow_automation.trigger_source` Int32) ENGINE=Memory",
		"CREATE TABLE action_annotation_d (customer_id String,profile_id String,conversation_id String,conversation_start_time DateTime64(6, 'UTC'),action_type Int32,`payload.workflow_automation_hint.workflow_automation_definition_id` String) ENGINE=Memory",
		"CREATE TABLE moment_annotation_d (customer_id String,profile_id String,conversation_id String,moment_annotation_id String,moment_template_id String,metadata_string_value String,create_time DateTime64(6, 'UTC'),update_time DateTime64(6, 'UTC')) ENGINE=Memory",
	} {
		s.Require().NoError(s.connection.Exec(s.T().Context(), query))
	}
	s.seedWorkflowAutomationFacts()
}
func (s *WorkflowAutomationQuerySuite) TearDownSuite() {
	if !s.native {
		s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
		return
	}
	if s.connection != nil {
		s.NoError(s.connection.Close())
	}
	if s.admin != nil {
		s.NoError(s.admin.Exec(s.T().Context(), "DROP DATABASE "+s.database))
		s.NoError(s.admin.Close())
	}
}
func (s *WorkflowAutomationQuerySuite) TestAggregation() {
	// Given
	ctx := s.T().Context()
	req := &analyticspb.RetrieveWorkflowAutomationStatsRequest{Frequency: analyticspb.Frequency_DAILY, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)), EndTimestamp: timestamppb.New(time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC))}}
	conditions, _, _, _, err := parseClickhouseFilter(ctx, req.GetFilterByTimeRange(), nil /* attribute */, []ClickhouseTable{conversationTable}, false /* useExtTable */)
	s.Require().NoError(err)
	for _, testCase := range []struct {
		name                           string
		selectors                      []workflowAutomationSelector
		triggers, triggerConversations int64
		ignored                        int64
		hintsAvailable                 bool
		sourceCounts                   [4]uint64
	}{
		{name: "AllTriggersIncludingUnknown", triggers: 4, triggerConversations: 2, ignored: 2, hintsAvailable: true, sourceCounts: [4]uint64{1, 1, 1, 1}},
		{name: "ExecutedVersion", selectors: []workflowAutomationSelector{{definitionID: "d1", versions: []int32{7}}}, triggers: 2, triggerConversations: 2, sourceCounts: [4]uint64{0, 1, 0, 1}},
		{name: "paired definition versions", selectors: []workflowAutomationSelector{{definitionID: "d1", versions: []int32{3}}, {definitionID: "d2", versions: []int32{7}}}},
		{name: "DefinitionOnlyIncludesUnknownVersions", selectors: []workflowAutomationSelector{{definitionID: "d1"}}, triggers: 3, triggerConversations: 2, ignored: 1, hintsAvailable: true, sourceCounts: [4]uint64{1, 1, 0, 1}},
		{name: "DefinitionWithoutHints", selectors: []workflowAutomationSelector{{definitionID: "d2"}}, triggers: 1, triggerConversations: 1, hintsAvailable: true, sourceCounts: [4]uint64{0, 0, 1, 0}},
		{name: "MixedVersionSelectorsUnavailable", selectors: []workflowAutomationSelector{{definitionID: "d1"}, {definitionID: "d2", versions: []int32{3}}}, triggers: 4, triggerConversations: 2, sourceCounts: [4]uint64{1, 1, 1, 1}},
	} {
		s.Run(testCase.name, func() {
			// Given
			query, args := workflowAutomationStatsQuery(req, "c", "p", conditions[conversationTable], nil /* included */, nil /* excluded */, testCase.selectors, map[string][]string{"u1": {"g1", "g2"}, "u2": {"g1"}, "u3": {"g2"}})

			// When
			rows, err := queryWorkflowAutomationStats(ctx, s.connection, query, args)

			// Then
			s.Require().NoError(err)
			byKey := map[string]workflowAutomationStatsRow{}
			for _, row := range rows {
				byKey[fmt.Sprintf("%s/%d/%d", row.groupKey, row.isBucket, row.bucketMillis)] = row
			}
			total := byKey["/0/0"].stats()
			s.Equal(int64(4), total.GetTotalConversationCount())
			s.Equal(int64(3), total.GetTotalAgentCount())
			s.Equal(int64(4), total.GetTotalActiveAgentDayCount())
			s.Equal(testCase.triggers, total.GetTotalWorkflowAutomationTriggerCount())
			s.Equal(testCase.triggerConversations, total.GetTotalConversationWithTriggerCount())
			s.Equal(testCase.hintsAvailable, total.TotalIgnoredConversationCount != nil)
			s.Equal(testCase.ignored, total.GetTotalIgnoredConversationCount())
			s.Require().Len(total.GetTriggerSourceStats(), 4)
			for source, count := range testCase.sourceCounts {
				s.Equal(int32(source), int32(total.GetTriggerSourceStats()[source].GetTriggerSource()))
				s.Equal(int64(count), total.GetTriggerSourceStats()[source].GetTotalWorkflowAutomationTriggerCount())
			}
			for day := 1; day <= 2; day++ {
				bucketMillis := time.Date(2026, 9, day, 0, 0, 0, 0, time.UTC).UnixMilli()
				bucket, exists := byKey[fmt.Sprintf("/1/%d", bucketMillis)]
				s.Require().True(exists)
				s.Equal(uint64(2), bucket.conversationCount)
				expectedSources := testCase.sourceCounts
				if day == 1 {
					expectedSources[0], expectedSources[3] = 0, 0
				} else {
					expectedSources[1], expectedSources[2] = 0, 0
				}
				s.Equal(expectedSources, bucket.sourceCounts)
			}
			group1 := byKey["g1/0/0"].stats()
			group2 := byKey["g2/0/0"].stats()
			s.Equal(int64(3), group1.GetTotalConversationCount())
			s.Equal(int64(3), group2.GetTotalConversationCount())
			s.Equal(testCase.triggers, group1.GetTotalWorkflowAutomationTriggerCount())
			s.Equal(testCase.triggers, group2.GetTotalWorkflowAutomationTriggerCount())
			s.Equal(testCase.hintsAvailable, group1.TotalIgnoredConversationCount != nil)
			s.Equal(testCase.hintsAvailable, group2.TotalIgnoredConversationCount != nil)
		})
	}
}

func (s *WorkflowAutomationQuerySuite) TestFactScanTimeBounds() {
	for _, testCase := range []struct {
		name              string
		start, end        time.Time
		triggers, ignored int64
	}{
		{name: "ExclusiveEventEnd", start: time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC), end: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)},
		{name: "InclusiveEventStartWithLateUpdates", start: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), end: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC), triggers: 2, ignored: 1},
		{name: "ExclusiveHintEnd", start: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC), end: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), triggers: 2},
		{name: "InclusiveHintStart", start: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), end: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), ignored: 1},
	} {
		s.Run(testCase.name, func() {
			// Given
			request := &analyticspb.RetrieveWorkflowAutomationStatsRequest{
				Frequency:         analyticspb.Frequency_DAILY,
				FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(testCase.start), EndTimestamp: timestamppb.New(testCase.end)},
			}
			// Leave the population unbounded to verify each fact scan applies its own time bounds.
			query, args := workflowAutomationStatsQuery(request, "c", "p", nil /* conditions */, nil /* included */, nil /* excluded */, nil /* selectors */, nil /* memberships */)

			// When
			rows, err := queryWorkflowAutomationStats(s.T().Context(), s.connection, query, args)

			// Then
			s.Require().NoError(err)
			s.Require().Len(rows, 3)
			for _, row := range rows {
				if row.isBucket != 0 {
					continue
				}
				total := row.stats()
				s.Equal(int64(4), total.GetTotalConversationCount())
				s.Equal(testCase.triggers, total.GetTotalWorkflowAutomationTriggerCount())
				s.Require().NotNil(total.TotalIgnoredConversationCount)
				s.Equal(testCase.ignored, total.GetTotalIgnoredConversationCount())
			}
		})
	}
}

func (s *WorkflowAutomationQuerySuite) seedWorkflowAutomationFacts() {
	ctx := s.T().Context()

	s.Require().NoError(s.connection.Exec(ctx, `INSERT INTO conversation_d (customer_id,profile_id,conversation_id,agent_user_id,conversation_start_time,update_time) VALUES
 ('c','p','one','u1','2026-09-01 10:00:00','2026-09-03 00:00:00'),
 ('c','p','one','u1','2026-09-01 10:00:00','2026-09-03 01:00:00'),
 ('c','p','two','u1','2026-09-02 10:00:00','2026-09-03 00:00:00'),
 ('c','p','unused','u2','2026-09-02 12:00:00','2026-09-03 00:00:00'),
 ('c','p','zero','u3','2026-09-01 12:00:00','2026-09-03 00:00:00'),
 ('other','p','one','u1','2026-09-01 10:00:00','2026-09-03 00:00:00'),
 ('dst','p','before','u1','2026-03-07 19:00:00','2026-03-10 00:00:00'),
 ('dst','p','first','u1','2026-03-07 20:00:00','2026-03-10 00:00:00'),
 ('dst','p','before-midnight','u1','2026-03-08 07:30:00','2026-03-10 00:00:00'),
 ('dst','p','dst-day','u1','2026-03-09 06:30:00','2026-03-10 00:00:00'),
 ('dst','p','last','u1','2026-03-09 15:00:00','2026-03-10 00:00:00'),
 ('dst','p','after','u1','2026-03-09 17:00:00','2026-03-10 00:00:00')`))
	s.Require().NoError(s.connection.Exec(ctx, "INSERT INTO conversation_event_d (customer_id,profile_id,conversation_id,conversation_start_time,event_id,event_type,update_time,`payload.workflow_automation.workflow_automation_run_id`,`payload.workflow_automation.workflow_automation_definition_id`,`payload.workflow_automation.workflow_automation_definition_version`,`payload.workflow_automation.trigger_source`) VALUES "+`
 ('c','p','one','2026-09-01 10:00:00','t1',40,'2026-09-04 00:00:00','r1','d1',0,1),
 ('c','p','one','2026-09-01 10:00:00','t1',40,'2026-09-04 01:00:00','r1','d1',0,1),
 ('c','p','one','2026-09-01 10:00:00','i1',41,'2026-09-04 01:00:00','r1','d1',7,0),
 ('c','p','one','2026-09-01 10:00:00','o1',42,'2026-09-04 01:00:00','r1','d1',7,0),
 ('c','p','one','2026-09-01 10:00:00','t2',40,'2026-09-04 00:00:00','r2','d2',3,2),
 ('c','p','two','2026-09-02 10:00:00','t3',40,'2026-09-04 00:00:00','r3','d1',7,3),
 ('c','p','two','2026-09-02 10:00:00','t4',40,'2026-09-04 00:00:00','r4','d1',0,99),
 ('other','p','one','2026-09-01 10:00:00','foreign',40,'2026-09-04 00:00:00','r1','d1',7,1),
 ('c','other','one','2026-09-01 10:00:00','foreign-profile',40,'2026-09-04 00:00:00','r1','d1',7,1)`))
	s.Require().NoError(s.connection.Exec(ctx, "INSERT INTO action_annotation_d (customer_id,profile_id,conversation_id,conversation_start_time,action_type,`payload.workflow_automation_hint.workflow_automation_definition_id`) VALUES "+`
 ('c','p','one','2026-09-01 10:00:00',37,'d1'),('c','p','one','2026-09-01 10:00:00',37,'d1'),
 ('c','p','unused','2026-09-02 12:00:00',37,'d1'),('c','p','unused','2026-09-02 12:00:00',37,'d1'),
 ('c','p','zero','2026-09-01 12:00:00',37,''),('other','p','zero','2026-09-01 12:00:00',37,'d1'),('c','other','zero','2026-09-01 12:00:00',37,'d1')`))
	s.Require().NoError(s.connection.Exec(ctx, `INSERT INTO moment_annotation_d (customer_id,profile_id,conversation_id,moment_annotation_id,moment_template_id,metadata_string_value,update_time) VALUES
 ('c','p','one','a','a','red','2026-09-02 00:00:00'),('c','p','unused','a','a','red','2026-09-02 00:00:00'),
 ('c','p','one','b','b','x','2026-09-02 00:00:00'),('c','p','unused','b','b','y','2026-09-02 00:00:00'),
 ('other','p','zero','a','a','red','2026-09-02 00:00:00'),('other','p','zero','b','b','x','2026-09-02 00:00:00')`))
}

func (s *WorkflowAutomationQuerySuite) TestDailyBucketsAcrossDST() {
	// Given
	request := &analyticspb.RetrieveWorkflowAutomationStatsRequest{
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: "America/Los_Angeles"},
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 3, 7, 20, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 9, 16, 0, 0, 0, time.UTC)),
		},
	}
	conditions, _, _, _, err := parseClickhouseFilter(s.T().Context(), request.GetFilterByTimeRange(), nil /* attribute */, []ClickhouseTable{conversationTable}, false /* useExtTable */)
	s.Require().NoError(err)
	query, args := workflowAutomationStatsQuery(request, "dst", "p", conditions[conversationTable], nil /* included */, nil /* excluded */, nil /* selectors */, nil /* memberships */)
	expectedBuckets := map[int64]uint64{
		time.Date(2026, 3, 7, 8, 0, 0, 0, time.UTC).UnixMilli(): 2,
		time.Date(2026, 3, 8, 8, 0, 0, 0, time.UTC).UnixMilli(): 1,
		time.Date(2026, 3, 9, 7, 0, 0, 0, time.UTC).UnixMilli(): 1,
	}

	// When
	rows, err := queryWorkflowAutomationStats(s.T().Context(), s.connection, query, args)

	// Then
	s.Require().NoError(err)
	s.Require().Len(rows, 4)
	actualBuckets := map[int64]uint64{}
	for _, row := range rows {
		if row.isBucket == 0 {
			s.Equal(uint64(4), row.conversationCount)
			continue
		}
		actualBuckets[row.bucketMillis] = row.conversationCount
	}
	s.Equal(expectedBuckets, actualBuckets)
}

func (s *WorkflowAutomationQuerySuite) TestMetadataAndUserFilters() {
	// Given
	req := &analyticspb.RetrieveWorkflowAutomationStatsRequest{
		Frequency: analyticspb.Frequency_DAILY,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)),
		},
	}
	metadataFilter := func(template, value string) []conditionAndArg {
		return []conditionAndArg{{condition: "moment_template_id = ?", arg: template}, {condition: "metadata_string_value = ?", arg: value}}
	}
	for _, testCase := range []struct {
		name                    string
		included, excluded      [][]conditionAndArg
		population              []conditionAndArg
		conversations, triggers int64
		ignored                 int64
	}{
		{name: "multiple include groups", included: [][]conditionAndArg{metadataFilter("a", "red"), metadataFilter("b", "x")}, conversations: 1, triggers: 2},
		{name: "exclude group", excluded: [][]conditionAndArg{metadataFilter("a", "red")}, conversations: 2, triggers: 2, ignored: 1},
		{name: "include and exclude groups", included: [][]conditionAndArg{metadataFilter("a", "red")}, excluded: [][]conditionAndArg{metadataFilter("b", "x")}, conversations: 1, ignored: 1},
		{name: "multiple exclude groups", excluded: [][]conditionAndArg{metadataFilter("a", "red"), metadataFilter("b", "x")}, conversations: 2, triggers: 2, ignored: 1},
		{name: "authorized user scope", population: []conditionAndArg{{condition: "agent_user_id = ?", arg: "u2"}}, conversations: 1, ignored: 1},
		{name: "empty authorized scope", population: []conditionAndArg{{condition: "agent_user_id = ?", arg: "nobody"}}},
	} {
		s.Run(testCase.name, func() {
			// Given
			query, args := workflowAutomationStatsQuery(req, "c", "p", testCase.population, testCase.included, testCase.excluded, nil /* selectors */, nil /* memberships */)

			// When
			rows, err := queryWorkflowAutomationStats(s.T().Context(), s.connection, query, args)

			// Then
			s.Require().NoError(err)
			totals := (workflowAutomationStatsRow{ignoredCount: new(int64)}).stats()
			for _, row := range rows {
				if row.groupKey == "" && row.isBucket == 0 {
					totals = row.stats()
				}
			}
			s.Equal(testCase.conversations, totals.GetTotalConversationCount())
			s.Equal(testCase.triggers, totals.GetTotalWorkflowAutomationTriggerCount())
			s.Require().NotNil(totals.TotalIgnoredConversationCount)
			s.Equal(testCase.ignored, totals.GetTotalIgnoredConversationCount())
		})
	}
}
