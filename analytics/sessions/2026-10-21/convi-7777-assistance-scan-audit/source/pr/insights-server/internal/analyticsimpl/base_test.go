package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock/cresta/v1/user"
	"github.com/cresta/shared-go/container/cache"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	mockaclhelperprovider "github.com/cresta/go-servers/shared/auth/mocks"
	clickhouseConvoTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	clickhouseEventTesting "github.com/cresta/go-servers/shared/clickhouse/events/testing"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	dbtesting "github.com/cresta/go-servers/shared/db/testing"
	"github.com/cresta/go-servers/shared/featureflag"
	testingmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

const (
	SecondPerHour         = int64(time.Hour / time.Second)
	SecondPerMinute       = int64(time.Minute / time.Second)
	CustomerID            = "customer_id"
	ProfileID             = "profile_id"
	UsecaseID             = "usecase_id"
	Crit1                 = "crit1"
	Crit2                 = "crit2"
	Crit3                 = "crit3"
	LanguageCodeEnUs      = "en-US"
	AcceptedDelta         = 0.001 // should have no impact on full percentage
	clickhouseQueryMethod = "Query"
)

type AnalyticsTestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite

	server                  *AnalyticsServiceImpl
	ctx                     context.Context
	customerResourceName    string
	profileResourceName     string
	usecaseResourceName     string
	customerID              string
	profileID               string
	usecaseID               string
	esUseCase               string
	esIndexVersion          string
	losAngeles              string
	agentsAndGroups         *AgentsAndGroupsUserProto
	mockConfigClient        *mockconfig.MockClient
	testData                map[string]string
	mockACLHelperProvider   *mockaclhelperprovider.MockResourceACLHelperProvider
	mockACLHelper           *mockaclhelperprovider.MockResourceACLHelper
	mockListUsersCache      *shared.ListUsersCache
	mockSuggestedConvoCache *shared.SuggestedConvoCache
	convoMockConn           *clickhouseConvoTesting.MockConn
	authEventMockConn       *clickhouseEventTesting.MockConn
	eventMockConn           *clickhouseEventTesting.MockConn

	// Set before SetupSuite to override rollout flags; unset flags fall back to the call site's default.
	featureFlagStubClientConfig featureflag.StubClientConfig
}

type UsersData struct {
	agentUserA   *dbmodel.Users
	agentUserB   *dbmodel.Users
	agentUserC   *dbmodel.Users
	agentUserD   *dbmodel.Users
	agentUserE   *dbmodel.Users
	managerUserA *dbmodel.Users
	managerUserB *dbmodel.Users
	chatVisitorA *dbmodel.ChatVisitors
}

type AgentsAndGroupsUserProto struct {
	agentUserA                     *userpb.User
	agentUserB                     *userpb.User
	agentUserC                     *userpb.User
	agentUserD                     *userpb.User
	agentUserAWithGroupMemberships *userpb.User
	agentUserBWithGroupMemberships *userpb.User
	agentUserCWithGroupMemberships *userpb.User
	agentUserDWithGroupMemberships *userpb.User
	groupUser1                     *userpb.Group
	groupUser2                     *userpb.Group
	groupUser3                     *userpb.Group
}

type TimePeriodData struct {
	pst                                 *time.Location
	janFirst                            time.Time
	janFirstProto                       timestamppb.Timestamp
	sundayOfTheWeekOfJanFirstPSTProto   timestamppb.Timestamp
	janSeventh                          time.Time
	janSeventhProto                     timestamppb.Timestamp
	sundayOfTheWeekOfJanSeventhPSTProto timestamppb.Timestamp
	janTenth                            time.Time
	janTenthProto                       timestamppb.Timestamp
	janEleventhProto                    timestamppb.Timestamp
	janPeriod                           analyticspb.TimeRange
	metadataLA                          analyticspb.Metadata
}

// TODO: this method should not exist. Use zero values and call testsuite.Run instead.
func newAnalyticsTestSuite(t *testing.T) AnalyticsTestSuite {
	s := suite.Suite{}
	s.SetT(t)
	return AnalyticsTestSuite{DBTestSuite: dbtesting.DBTestSuite{}, Suite: s} //nolint
}

func (s *AnalyticsTestSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.DBTestSuite.LoadDirectorSchema(s.T())
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.ctx = context.TODO()
	s.server = &AnalyticsServiceImpl{
		appsDB:                      s.DBTestSuite.AppsDBRegistry(),
		appEnv:                      "test",
		logger:                      logger.GetLogger(),
		clickhouseConvoRegistry:     s.ClickhouseContainerTestSuite.GetConversationTestRegistry(),
		clickhouseEventRegistry:     s.ClickhouseContainerTestSuite.GetEventsTestRegistry(),
		clickhouseAuthEventRegistry: s.ClickhouseContainerTestSuite.GetAuthEventsTestRegistry(),
		featureFlagClient:           featureflag.NewStubClient(s.featureFlagStubClientConfig),
		// Tests construct this struct directly instead of going through the env-reading constructor,
		// so mirror the production default here. A zero value means errgroup.WithLimit(0), which is an
		// unbuffered semaphore that deadlocks the first eg.Go in the respect-config fan-out path.
		maxConcurrentQaScoreStatsQueries: defaultMaxConcurrentQaScoreStatsQueries,
	}
	// Delete existing hint types loaded from bootstrap-data.sql
	// We create the hint types in the test
	s.Require().NoError(s.DB().Exec("DELETE FROM app.hint_templates WHERE true").Error)
	s.Require().NoError(s.DB().Exec("DELETE FROM app.hint_types WHERE true").Error)
	s.customerResourceName = fmt.Sprintf("customers/%s", CustomerID)
	s.profileResourceName = fmt.Sprintf("customers/%s/profiles/%s", CustomerID, ProfileID)
	s.usecaseResourceName = fmt.Sprintf("customers/%s/profiles/%s/usecases/%s", CustomerID, ProfileID, UsecaseID)
	s.customerID = CustomerID
	s.profileID = ProfileID
	s.usecaseID = UsecaseID
	s.esUseCase = "conversations"
	s.esIndexVersion = "0"
	s.losAngeles = "Pacific/Honolulu" // Honolulu doesn't have daylight saving, which is consistent as `UTC-10:00`.

	// Build the mock UserServiceClient.
	agentsAndGroupsUserProto, mockUserServiceClient := s.SetupMockUserServiceClient()
	s.agentsAndGroups = agentsAndGroupsUserProto
	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](mockUserServiceClient)

	// Build the mock InternalUserServiceClient.
	mockInternalUserServiceClient := s.SetupMockInternalUserServiceClient()
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](mockInternalUserServiceClient)

	// Build the mock ConfigClient.
	testReporter := testingmock.NewTestReporter(s.T())
	s.mockConfigClient = mockconfig.NewMockClient(gomock.NewController(testReporter))
	s.server.configClient = s.mockConfigClient

	// Default mock for configClient.Get() - returns empty config (ACL disabled).
	// This is needed because ParseUserFilterForAnalytics calls applyResourceACL which calls configClient.Get().
	s.mockConfigClient.EXPECT().Get(gomock.Any()).AnyTimes().Return(&config.CustomerConfig{}, nil)

	// Build the mock ACL client
	s.mockACLHelperProvider = mockaclhelperprovider.NewMockResourceACLHelperProvider(gomock.NewController(testReporter))
	s.server.resourceACLHelperProvider = s.mockACLHelperProvider
	s.mockACLHelper = mockaclhelperprovider.NewMockResourceACLHelper(gomock.NewController(testReporter))

	// Mock listUsers memory cache.
	s.mockListUsersCache = &shared.ListUsersCache{
		UsersToGroupsCache: cache.New(cache.WithDefaultExpirationTimeout[string, map[string][]string](time.Second)),
		UsersCache:         cache.New(cache.WithDefaultExpirationTimeout[string, []*userpb.User](time.Second)),
		GroupsCache:        cache.New(cache.WithDefaultExpirationTimeout[string, []*userpb.Group](time.Second)),
	}
	s.server.listUsersCache = s.mockListUsersCache
	s.server.enableListUsersCache = true

	// Mock suggestedConvo memory cache.
	s.mockSuggestedConvoCache = &shared.SuggestedConvoCache{
		ConversationSuggested: cache.New(cache.WithDefaultExpirationTimeout[string, string](time.Second)),
	}
	s.server.suggestedConvoCache = s.mockSuggestedConvoCache

	s.testData = make(map[string]string)
	files, err := filepath.Glob("testdata/*.sql")
	s.Require().NoError(err)

	for _, fpath := range files {
		f, err := os.ReadFile(fpath)
		s.Require().NoError(err)
		s.testData[filepath.Base(fpath)] = string(f)
	}

	s.mockACLHelperProvider.EXPECT().Get().AnyTimes().Return(s.mockACLHelper)
}

func (s *AnalyticsTestSuite) getFile(fname string) string {
	val, exists := s.testData[fname]
	s.True(exists)
	return val
}

func (s *AnalyticsTestSuite) TearDownSuite() {
	s.DBTestSuite.TearDownSuite(s.T())
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
}

func (s *AnalyticsTestSuite) assertProtoEquals(want, actual proto.Message) {
	if diff := cmp.Diff(want, actual, protocmp.Transform(), protocmp.IgnoreEmptyMessages()); diff != "" {
		s.Failf("Proto mismatch", "(-want, +got):\n%s", diff)
	}
}

// Temporally disable lint until other error code is tested.
func (s *AnalyticsTestSuite) assertError(err error, code codes.Code, msg string) {
	status, _ := status.FromError(err)
	s.Equal(code, status.Code(), msg)
}

func (s *AnalyticsTestSuite) CreateModels(models ...dbmodel.Model) func() {
	for _, model := range models {
		err := s.DB().Create(model).Error
		s.Require().NoError(err)
	}
	return func() {
		for i := len(models) - 1; i >= 0; i-- {
			err := s.DB().Delete(models[i]).Error
			s.Require().NoError(err)
		}
	}
}

func (s *AnalyticsTestSuite) CountRows(model interface{}) int64 {
	var count int64
	s.DB().Model(model).Count(&count)
	return count
}

func (s *AnalyticsTestSuite) SetupTimePeriodData() *TimePeriodData {
	timePeriodData := &TimePeriodData{}
	pst, _ := time.LoadLocation(s.losAngeles)
	timePeriodData.pst = pst
	timePeriodData.janFirst = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	timePeriodData.janFirstProto = *timestamppb.New(timePeriodData.janFirst)
	timePeriodData.sundayOfTheWeekOfJanFirstPSTProto = *timestamppb.New(time.Date(2020, 12, 27, 0, 0, 0, 0, pst))
	timePeriodData.janSeventh = time.Date(2021, 1, 7, 0, 0, 0, 0, time.UTC)
	timePeriodData.janSeventhProto = *timestamppb.New(timePeriodData.janSeventh)
	timePeriodData.sundayOfTheWeekOfJanSeventhPSTProto = *timestamppb.New(time.Date(2021, 1, 3, 0, 0, 0, 0, pst))
	timePeriodData.janTenth = time.Date(2021, 1, 10, 0, 0, 0, 0, time.UTC)
	timePeriodData.janTenthProto = *timestamppb.New(timePeriodData.janTenth)
	timePeriodData.janEleventhProto = *timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, time.UTC))
	timePeriodData.janPeriod = analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	timePeriodData.metadataLA = analyticspb.Metadata{TimeZoneId: s.losAngeles, StartDayOfWeek: analyticspb.Metadata_SUNDAY}
	return timePeriodData
}

func (s *AnalyticsTestSuite) SetupUsersData() *UsersData {
	usersData := &UsersData{}
	usersData.agentUserA = &dbmodel.Users{
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
			String: s.customerID,
			Valid:  true,
		},
		ProfilePicturePath: sql.NullString{
			String: "testPathAgentA",
			Valid:  true,
		},
	}
	usersData.agentUserB = &dbmodel.Users{
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
			String: s.customerID,
			Valid:  true,
		},
	}
	usersData.agentUserC = &dbmodel.Users{
		ID: 3,
		UserID: sql.NullString{
			String: "3",
			Valid:  true,
		},
		Password: "",
		Username: "agent_c",
		FullName: sql.NullString{
			String: "agent c",
			Valid:  true,
		},
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfilePicturePath: sql.NullString{
			String: "testPathAgentC",
			Valid:  true,
		},
	}
	usersData.agentUserD = &dbmodel.Users{
		ID: 4,
		UserID: sql.NullString{
			String: "4",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "agent d",
			Valid:  true,
		},
		Password:  "",
		Username:  "agent_d",
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
	}
	usersData.agentUserE = &dbmodel.Users{
		ID: 7,
		UserID: sql.NullString{
			String: "7",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "agent e",
			Valid:  true,
		},
		Password:  "",
		Username:  "agent_e",
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
	}

	usersData.managerUserA = &dbmodel.Users{
		ID: 5,
		UserID: sql.NullString{
			String: "5",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "manager a",
			Valid:  true,
		},
		Password:  "",
		Username:  "manager_a",
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfilePicturePath: sql.NullString{
			String: "testPathManagerA",
			Valid:  true,
		},
	}
	usersData.managerUserB = &dbmodel.Users{
		ID: 6,
		UserID: sql.NullString{
			String: "6",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "manager b",
			Valid:  true,
		},
		Password:  "",
		Username:  "manager_b",
		IsDevUser: false,
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
	}

	usersData.chatVisitorA = &dbmodel.ChatVisitors{
		ID:         3,
		PlatformID: "",
	}

	s.CreateModels(
		usersData.agentUserA, usersData.agentUserB, usersData.agentUserC, usersData.agentUserD,
		usersData.managerUserA, usersData.managerUserB,
		usersData.chatVisitorA,
	)

	return usersData
}

func (s *AnalyticsTestSuite) SetupMockUserServiceClient() (*AgentsAndGroupsUserProto, *usermock.UserServiceClient) {
	agentsAndGroupsUserProto := &AgentsAndGroupsUserProto{}
	agentsAndGroupsUserProto.agentUserA = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/1", CustomerID),
		DisplayName: "Agent A",
		Username:    "agent_a",
		FullName:    "agent a",
	}
	agentsAndGroupsUserProto.agentUserB = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/2", CustomerID),
		DisplayName: "Agent B",
		Username:    "agent_b",
		FullName:    "agent b",
	}
	agentsAndGroupsUserProto.agentUserC = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/3", CustomerID),
		DisplayName: "Agent C",
		Username:    "agent_c",
		FullName:    "agent c",
	}
	agentsAndGroupsUserProto.agentUserD = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/4", CustomerID),
		DisplayName: "Agent D",
		Username:    "agent_d",
		FullName:    "agent d",
	}
	agentsAndGroupsUserProto.agentUserAWithGroupMemberships = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/1", CustomerID),
		DisplayName: "Agent A",
		Username:    "agent_a",
		FullName:    "agent a",
		GroupMemberships: []*userpb.GroupMembership{
			{
				Group:            fmt.Sprintf("customers/%s/groups/group-1", CustomerID),
				GroupDisplayName: "Group 1",
				IsIndirect:       false,
				GroupType:        userpb.Group_TEAM,
			},
			{
				Group:            fmt.Sprintf("customers/%s/groups/group-3", CustomerID),
				GroupDisplayName: "Group 3",
				IsIndirect:       true,
			},
		},
	}
	agentsAndGroupsUserProto.agentUserBWithGroupMemberships = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/2", CustomerID),
		DisplayName: "Agent B",
		Username:    "agent_b",
		FullName:    "agent b",
		GroupMemberships: []*userpb.GroupMembership{
			{
				Group:            fmt.Sprintf("customers/%s/groups/group-2", CustomerID),
				GroupDisplayName: "Group 2",
				IsIndirect:       false,
				GroupType:        userpb.Group_TEAM,
			},
		},
	}
	agentsAndGroupsUserProto.agentUserCWithGroupMemberships = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/3", CustomerID),
		DisplayName: "Agent C",
		Username:    "agent_c",
		FullName:    "agent c",
		GroupMemberships: []*userpb.GroupMembership{
			{
				Group:            fmt.Sprintf("customers/%s/groups/group-2", CustomerID),
				GroupDisplayName: "Group 2",
				IsIndirect:       false,
				GroupType:        userpb.Group_TEAM,
			},
		},
	}
	agentsAndGroupsUserProto.agentUserDWithGroupMemberships = &userpb.User{
		Name:        fmt.Sprintf("customers/%s/users/4", CustomerID),
		DisplayName: "Agent D",
		Username:    "agent_d",
		FullName:    "agent d",
		GroupMemberships: []*userpb.GroupMembership{
			{
				Group:            fmt.Sprintf("customers/%s/groups/group-2", CustomerID),
				GroupDisplayName: "Group 2",
				IsIndirect:       false,
				GroupType:        userpb.Group_TEAM,
			},
		},
	}
	agentsAndGroupsUserProto.groupUser1 = &userpb.Group{
		Name:        fmt.Sprintf("customers/%s/groups/group-1", CustomerID),
		DisplayName: "Group 1",
		Type:        userpb.Group_TEAM,
		IsRoot:      false,
	}
	agentsAndGroupsUserProto.groupUser2 = &userpb.Group{
		Name:        fmt.Sprintf("customers/%s/groups/group-2", CustomerID),
		DisplayName: "Group 2",
		Type:        userpb.Group_TEAM,
		IsRoot:      false,
	}
	agentsAndGroupsUserProto.groupUser3 = &userpb.Group{
		Name:        fmt.Sprintf("customers/%s/groups/group-3", CustomerID),
		DisplayName: "Group 3",
		Type:        userpb.Group_TEAM,
		IsRoot:      false,
	}

	mockUserServiceClient := new(usermock.UserServiceClient)
	// List users - Group 1
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					Groups:                          []string{agentsAndGroupsUserProto.groupUser1.Name},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserA,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					Groups:                          []string{agentsAndGroupsUserProto.groupUser1.Name},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserAWithGroupMemberships,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser1.Name,
						agentsAndGroupsUserProto.groupUser3.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserAWithGroupMemberships,
			},
		}, nil)
	// List users - Group 2
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser2.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserB,
				agentsAndGroupsUserProto.agentUserC,
				agentsAndGroupsUserProto.agentUserD,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser2.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserBWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserCWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserDWithGroupMemberships,
			},
		}, nil)
	// List users - Group 3
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser3.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserA,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser3.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserAWithGroupMemberships,
			},
		}, nil)
	// List users - Group 1 & Group 2
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser1.Name,
						agentsAndGroupsUserProto.groupUser2.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserA,
				agentsAndGroupsUserProto.agentUserB,
				agentsAndGroupsUserProto.agentUserC,
				agentsAndGroupsUserProto.agentUserD,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser1.Name,
						agentsAndGroupsUserProto.groupUser2.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserAWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserBWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserCWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserDWithGroupMemberships,
			},
		}, nil)
	// List users - Group 1 & Group 2 & Group 3
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser1.Name,
						agentsAndGroupsUserProto.groupUser2.Name,
						agentsAndGroupsUserProto.groupUser3.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserA,
				agentsAndGroupsUserProto.agentUserB,
				agentsAndGroupsUserProto.agentUserC,
				agentsAndGroupsUserProto.agentUserD,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					Groups: []string{
						agentsAndGroupsUserProto.groupUser1.Name,
						agentsAndGroupsUserProto.groupUser2.Name,
						agentsAndGroupsUserProto.groupUser3.Name,
					},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserAWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserBWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserCWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserDWithGroupMemberships,
			},
		}, nil)
	// List users - no groups (all)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					Groups:                          []string{},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserA,
				agentsAndGroupsUserProto.agentUserB,
				agentsAndGroupsUserProto.agentUserC,
				agentsAndGroupsUserProto.agentUserD,
			},
		}, nil)
	mockUserServiceClient.
		On("ListUsers", mock.Anything, mock.MatchedBy(func(req *userpb.ListUsersRequest) bool {
			return proto.Equal(&userpb.ListUsersRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				ProfileId: s.profileID,
				PageSize:  500,
				PageToken: "",
				Filter: &userpb.ListUsersRequest_ListUsersFilter{
					CustomerProfiles:                []string{"customers/customer_id/profiles/profile_id"},
					Groups:                          []string{},
					IncludeIndirectGroupMemberships: true,
				},
				IncludeUserMemberships: true,
				OrderField:             "user_id",
			}, req)
		})).
		Return(&userpb.ListUsersResponse{
			Users: []*userpb.User{
				agentsAndGroupsUserProto.agentUserAWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserBWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserCWithGroupMemberships,
				agentsAndGroupsUserProto.agentUserDWithGroupMemberships,
			},
		}, nil)

	// List groups
	mockUserServiceClient.
		On("ListGroups", mock.Anything, mock.MatchedBy(func(req *userpb.ListGroupsRequest) bool {
			return proto.Equal(&userpb.ListGroupsRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				PageSize:  200,
				PageToken: "",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
					GroupIds:         []string{"group-2", "group-1"},
				},
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			}, req)
		})).
		Return(&userpb.ListGroupsResponse{
			Groups: []*userpb.Group{
				agentsAndGroupsUserProto.groupUser1,
				agentsAndGroupsUserProto.groupUser2,
			},
		}, nil)
	mockUserServiceClient.
		On("ListGroups", mock.Anything, mock.MatchedBy(func(req *userpb.ListGroupsRequest) bool {
			return proto.Equal(&userpb.ListGroupsRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				PageSize:  200,
				PageToken: "",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
					GroupIds:         []string{"group-1", "group-2"},
				},
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			}, req)
		})).
		Return(&userpb.ListGroupsResponse{
			Groups: []*userpb.Group{
				agentsAndGroupsUserProto.groupUser1,
				agentsAndGroupsUserProto.groupUser2,
			},
		}, nil)
	mockUserServiceClient.
		On("ListGroups", mock.Anything, mock.MatchedBy(func(req *userpb.ListGroupsRequest) bool {
			return proto.Equal(&userpb.ListGroupsRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				PageSize:  200,
				PageToken: "",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			}, req)
		})).
		Return(&userpb.ListGroupsResponse{
			Groups: []*userpb.Group{
				agentsAndGroupsUserProto.groupUser1,
				agentsAndGroupsUserProto.groupUser2,
				agentsAndGroupsUserProto.groupUser3,
			},
		}, nil)
	mockUserServiceClient.
		On("ListGroups", mock.Anything, mock.MatchedBy(func(req *userpb.ListGroupsRequest) bool {
			return proto.Equal(&userpb.ListGroupsRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				PageSize:  200,
				PageToken: "",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupIds:         []string{"group-1", "group-2"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				IncludeGroupMemberships:         true,
				IncludeIndirectGroupMemberships: true,
			}, req)
		})).
		Return(&userpb.ListGroupsResponse{
			Groups: []*userpb.Group{
				agentsAndGroupsUserProto.groupUser1,
				agentsAndGroupsUserProto.groupUser2,
			},
		}, nil)
	mockUserServiceClient.
		On("ListGroups", mock.Anything, mock.MatchedBy(func(req *userpb.ListGroupsRequest) bool {
			return proto.Equal(&userpb.ListGroupsRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				PageSize:  200,
				PageToken: "",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupIds:         []string{"group-3"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			}, req)
		})).
		Return(&userpb.ListGroupsResponse{
			Groups: []*userpb.Group{
				agentsAndGroupsUserProto.groupUser3,
			},
		}, nil)
	mockUserServiceClient.
		On("ListGroups", mock.Anything, mock.MatchedBy(func(req *userpb.ListGroupsRequest) bool {
			return proto.Equal(&userpb.ListGroupsRequest{
				Parent:    fmt.Sprintf("customers/%s", CustomerID),
				PageSize:  200,
				PageToken: "",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupIds:         []string{"group-3"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				IncludeGroupMemberships:         true,
				IncludeIndirectGroupMemberships: true,
			}, req)
		})).
		Return(&userpb.ListGroupsResponse{
			Groups: []*userpb.Group{
				agentsAndGroupsUserProto.groupUser3,
			},
		}, nil)
	return agentsAndGroupsUserProto, mockUserServiceClient
}

func (s *AnalyticsTestSuite) SetupMockInternalUserServiceClient() *internalusermock.InternalUserServiceClient {
	groupUser1 := &internaluserpb.LiteGroup{
		GroupId:     "group-1",
		DisplayName: "Group 1",
		GroupType:   internaluserpb.LiteGroup_TEAM,
	}
	groupUser2 := &internaluserpb.LiteGroup{
		GroupId:     "group-2",
		DisplayName: "Group 2",
		GroupType:   internaluserpb.LiteGroup_TEAM,
	}
	groupUser3 := &internaluserpb.LiteGroup{
		GroupId:     "group-3",
		DisplayName: "Group 3",
		GroupType:   internaluserpb.LiteGroup_TEAM,
	}

	agentUserA := &internaluserpb.LiteUser{
		UserId:   "1",
		Username: "agent_a",
		FullName: "agent a",
		Memberships: []*internaluserpb.UserGroupMembership{
			{
				Group:          groupUser1,
				IsDirectMember: true,
			},
			{
				Group:          groupUser3,
				IsDirectMember: false,
			},
		},
	}
	agentUserB := &internaluserpb.LiteUser{
		UserId:   "2",
		Username: "agent_b",
		FullName: "agent b",
		Memberships: []*internaluserpb.UserGroupMembership{
			{
				Group:          groupUser2,
				IsDirectMember: true,
			},
		},
	}
	agentUserC := &internaluserpb.LiteUser{
		UserId:   "3",
		Username: "agent_c",
		FullName: "agent c",
		Memberships: []*internaluserpb.UserGroupMembership{
			{
				Group:          groupUser2,
				IsDirectMember: true,
			},
		},
	}
	agentUserD := &internaluserpb.LiteUser{
		UserId:   "4",
		Username: "agent_d",
		FullName: "agent d",
		Memberships: []*internaluserpb.UserGroupMembership{
			{
				Group:          groupUser2,
				IsDirectMember: true,
			},
		},
	}

	mockInternalUserServiceClient := new(internalusermock.InternalUserServiceClient)
	// List users - Group 1
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{groupUser1.GroupId},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA},
		}, nil)
	// List users - Group 1 & Group 3
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{groupUser1.GroupId, groupUser3.GroupId},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA},
		}, nil)
	// List users - Group 2
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{groupUser2.GroupId},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserB, agentUserC, agentUserD},
		}, nil)
	// List users - Group 1 & Group 2
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{groupUser1.GroupId, groupUser2.GroupId},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA, agentUserB, agentUserC, agentUserD},
		}, nil)

	// List users - Group 1 & Group 2 & Group 3
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{groupUser1.GroupId, groupUser2.GroupId, groupUser3.GroupId},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA, agentUserB, agentUserC, agentUserD},
		}, nil)

	// List users - Group 3
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{groupUser3.GroupId},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA},
		}, nil)

	// List users - no groups (all)
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA, agentUserB, agentUserC, agentUserD},
		}, nil)

	// Additional mocks for AgentOnly requests (initially needed for agent stats tests)
	// Mock for requests with no groups and AgentOnly: true
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
				AgentOnly:                       true,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA, agentUserB, agentUserC, agentUserD},
		}, nil)

	// Mock for requests with group-1 and group-2 and AgentOnly: true
	mockInternalUserServiceClient.
		On("ListUsersForAnalytics", mock.Anything, mock.MatchedBy(func(req *internaluserpb.ListUsersForAnalyticsRequest) bool {
			return proto.Equal(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent:                          fmt.Sprintf("customers/%s", CustomerID),
				ProfileIds:                      []string{s.profileID},
				GroupIds:                        []string{"group-1", "group-2"},
				IncludeIndirectGroupMemberships: true,
				IncludeInactiveUsers:            true,
				PageOffset:                      0,
				AgentOnly:                       true,
			}, req)
		})).
		Return(&internaluserpb.ListUsersForAnalyticsResponse{
			Users: []*internaluserpb.LiteUser{agentUserA, agentUserB, agentUserC, agentUserD},
		}, nil)

	return mockInternalUserServiceClient
}

func formatSQL(s string) string {
	reg := regexp.MustCompile("[\n\t ]+")
	return reg.ReplaceAllString(s, "")
}

// Only used in unit testing to compare query strings.
func combineQueryAndArgs(query string, args []any) string {
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			// Enclose string values in single quotes
			query = strings.Replace(query, "?", fmt.Sprintf("'%v'", strings.ReplaceAll(v, "'", "''")), 1)
		case []string:
			// Replace the first occurrence of "?" with the comma-split elements in the slice
			values := []string{}
			for _, sliceV := range v {
				values = append(values, fmt.Sprintf("'%v'", strings.ReplaceAll(sliceV, "'", "''")))
			}
			sort.Slice(values, func(i, j int) bool {
				return values[i] < values[j]
			})
			query = strings.Replace(query, "?", strings.Join(values, ","), 1)
		case []int:
			// Replace the first occurrence of "?" with the comma-split elements in the slice
			values := []string{}
			for _, sliceV := range v {
				values = append(values, fmt.Sprintf("%v", sliceV))
			}
			sort.Slice(values, func(i, j int) bool {
				return values[i] < values[j]
			})
			query = strings.Replace(query, "?", strings.Join(values, ","), 1)
		default:
			// For simple types, replace the first occurrence of "?" with the string representation
			query = strings.Replace(query, "?", fmt.Sprintf("%v", v), 1)
		}
	}
	return query
}

func (s *AnalyticsTestSuite) UseMockedClickhouseConnections() {
	s.convoMockConn = new(clickhouseConvoTesting.MockConn)
	s.ClickhouseContainerTestSuite.SetConversationConnectionMock(s.customerID, s.profileID, s.convoMockConn)
	s.authEventMockConn = new(clickhouseEventTesting.MockConn)
	s.ClickhouseContainerTestSuite.SetAuthEventsConnectionMock(s.customerID, s.authEventMockConn)
	s.eventMockConn = new(clickhouseEventTesting.MockConn)
	s.ClickhouseContainerTestSuite.SetEventsConnectionMock(s.customerID, s.profileID, s.eventMockConn)
}
