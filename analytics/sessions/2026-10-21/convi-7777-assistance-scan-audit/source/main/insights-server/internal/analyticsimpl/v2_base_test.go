// nolint:dupl
package analyticsimpl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	trendandanomalypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/trend_and_anomaly"
	internaljobpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/job"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	jobmock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/job"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	behaviormock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/behavior"
	coachingmock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/coaching"
	metadatamock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/metadataframework"
	momentmock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/moment"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	grpcRegistry "github.com/cresta/shared-go/framework/client/grpc/registry"
	"github.com/cresta/shared-go/framework/module"
	testserver "github.com/cresta/shared-go/framework/server/testing"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/metadata"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	testingfake "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/testing"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/auth/authserviceclient"
	"github.com/cresta/go-servers/shared/auth/customroles"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	clickhouseConvoTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	clickhouseeventsclient "github.com/cresta/go-servers/shared/clickhouse/events/client"
	clickhouseEventTesting "github.com/cresta/go-servers/shared/clickhouse/events/testing"
	"github.com/cresta/go-servers/shared/clickhouse/registry"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	"github.com/cresta/go-servers/shared/dao"
	dbtesting "github.com/cresta/go-servers/shared/db/testing"
	estesting "github.com/cresta/go-servers/shared/elasticsearch/testing"
	"github.com/cresta/go-servers/shared/featureflag"
	"github.com/cresta/go-servers/shared/prometheus"
	testingmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

const (
	FakeCustomerID = "customer_id"
	FakeProfileID  = "profile_id"

	// Minimal valid ES search response for request-only golden tests that do not
	// set esRespPath. Avoids appending getFile("") which produces an empty body.
	emptyESHitsResponse = `{"hits":{"hits":[]}}`
)

type AnalyticsV2TestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite

	ctx                         context.Context
	server                      *testserver.TestServer
	Client                      analyticspb.AnalyticsServiceClient
	mockESServer                *estesting.MockElasticSearchServer
	mockMomentClient            *momentmock.MockMomentServiceClient
	mockBehaviorClient          *behaviormock.MockBehaviorServiceClient
	mockCoachingClient          *coachingmock.MockCoachingServiceClient
	mockMetadataClient          *metadatamock.MockMetadataServiceClient
	mockUserClient              *usermock.MockUserServiceClient
	mockInternalJobClient       *jobmock.MockInternalJobServiceClient
	mockInternalUserClient      *internalusermock.MockInternalUserServiceClient
	mockConfigClient            *mockconfig.MockClient
	resourceACLHelperProvider   *authmocks.MockResourceACLHelperProvider
	resourceACLHelper           *authmocks.MockResourceACLHelper
	profileResourceName         string
	customerID                  string
	profileID                   string
	IndexName                   string
	testData                    map[string]string
	mockClickhouseConn          *clickhouseConvoTesting.MockConn
	convoMockConn               *clickhouseConvoTesting.MockConn
	authEventMockConn           *clickhouseEventTesting.MockConn
	eventMockConn               *clickhouseEventTesting.MockConn
	featureFlagStubClientConfig featureflag.StubClientConfig
}

func newTestSuite(t *testing.T) AnalyticsV2TestSuite {
	s := suite.Suite{}
	s.SetT(t)
	return AnalyticsV2TestSuite{Suite: s} //nolint
}

func (s *AnalyticsV2TestSuite) getFile(fname string) string {
	return s.testData[fname]
}

func (s *AnalyticsV2TestSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.DBTestSuite.LoadDirectorSchema(s.T())
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.mockESServer = estesting.NewMockElasticSearchServer(s.T())
	s.customerID = FakeCustomerID
	s.profileID = FakeProfileID
	s.profileResourceName = "customers/customer_id/profiles/profile_id"
	s.IndexName = "customer_id-profile_id-conversations-v2"
	envutil.SetEnv("CUSTOMER_ID", FakeCustomerID)
	envutil.SetEnv("PROFILE_ID", FakeProfileID)
	envutil.SetEnv("ES_INDEX_VERSION", "2")
	s.server = testserver.NewTestServer(s.T(), "")
	s.mockMomentClient = momentmock.NewMockMomentServiceClient(gomock.NewController(s.T()))
	s.mockBehaviorClient = behaviormock.NewMockBehaviorServiceClient(gomock.NewController(s.T()))
	s.mockCoachingClient = coachingmock.NewMockCoachingServiceClient(gomock.NewController(s.T()))
	s.mockMetadataClient = metadatamock.NewMockMetadataServiceClient(gomock.NewController(s.T()))
	s.mockUserClient = usermock.NewMockUserServiceClient(gomock.NewController(s.T()))
	s.mockInternalJobClient = jobmock.NewMockInternalJobServiceClient(gomock.NewController(s.T()))
	s.mockInternalUserClient = internalusermock.NewMockInternalUserServiceClient(gomock.NewController(s.T()))
	s.mockConfigClient = mockconfig.NewMockClient(gomock.NewController(testingmock.NewTestReporter(s.T())))
	s.mockConfigClient.EXPECT().ListCustomerConfigs().AnyTimes().Return(map[string]*config.CustomerConfig{})
	s.resourceACLHelper = authmocks.NewMockResourceACLHelper(gomock.NewController(testingmock.NewTestReporter(s.T())))
	s.resourceACLHelperProvider = authmocks.NewMockResourceACLHelperProvider(gomock.NewController(testingmock.NewTestReporter(s.T())))
	s.resourceACLHelperProvider.EXPECT().Get().AnyTimes().Return(s.resourceACLHelper)
	s.Require().NoError(s.server.AddModule(
		s.DBTestSuite.Module(),
		s.mockESServer.ClientModule(),
		dao.Module,
		s.ClickhouseContainerTestSuite.Module(),
		registry.ConversationsClickhouseModule,
		registry.AuthEventsClickhouseModule,
		registry.EventsClickhouseModule,
		clickhouseeventsclient.Module,
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[momentpb.MomentServiceClient] {
				return clientregistry.NewTestRegistry[momentpb.MomentServiceClient](s.mockMomentClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() userpb.UserServiceClient {
				return s.mockUserClient
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() auth.ResourceACLHelperProvider {
				return s.resourceACLHelperProvider
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[behaviorpb.BehaviorServiceClient] {
				return clientregistry.NewTestRegistry[behaviorpb.BehaviorServiceClient](s.mockBehaviorClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[coachingpb.CoachingServiceClient] {
				return clientregistry.NewTestRegistry[coachingpb.CoachingServiceClient](s.mockCoachingClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[userpb.UserServiceClient] {
				return clientregistry.NewTestRegistry[userpb.UserServiceClient](s.mockUserClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[metadatapb.MetadataServiceClient] {
				return clientregistry.NewTestRegistry[metadatapb.MetadataServiceClient](s.mockMetadataClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[internaljobpb.InternalJobServiceClient] {
				return clientregistry.NewTestRegistry[internaljobpb.InternalJobServiceClient](s.mockInternalJobClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() grpcRegistry.Registry[internaluserpb.InternalUserServiceClient] {
				return clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](s.mockInternalUserClient)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() (grpcRegistry.Registry[trendandanomalypb.TrendAndAnomalyServiceClient], error) {
				return clientregistry.NewTestRegistry[trendandanomalypb.TrendAndAnomalyServiceClient](&testingfake.FakeTrendAndAnomalyServer{}), nil
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Bind(s.mockConfigClient, module.As(new(config.Client)))
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() featureflag.Client {
				return featureflag.NewStubClient(s.featureFlagStubClientConfig)
			})
		}),
		module.NewSimpleModule(func(binder module.Binder) error {
			return binder.Provide(func() prometheus.Client {
				return nil
			})
		}),
		customroles.Module,
		authserviceclient.AuthServiceClientModule,
		auth.AuthEntityProviderModule,
		Module,
	))
	s.server.StartServer()
	s.Client = analyticspb.NewAnalyticsServiceClient(s.server.ClientConn)
	s.ctx = metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("cresta-access-token-claim", `{"User":{"name":"customers/testcustomer/users/testuser"}}`))
	// s.Require().NoError(s.DB().Exec("DELETE FROM app.users").Error)

	files, err := filepath.Glob("testdata/*.*")
	s.Require().NoError(err)

	s.testData = make(map[string]string)
	for _, fpath := range files {
		f, err := os.ReadFile(fpath)
		s.Require().NoError(err)
		s.testData[filepath.Base(fpath)] = string(f)
	}

	// Mock ClickHouse conn.
	s.mockClickhouseConn = new(clickhouseConvoTesting.MockConn)
	s.mockClickhouseConn.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.mockClickhouseConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(clickhouseConvoTesting.NewMockRows(nil), nil)
}

func (s *AnalyticsV2TestSuite) TearDownSuite() {
	s.Require().NoError(s.server.StopServer())
	s.DBTestSuite.TearDownSuite(s.T())
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
}

// Clean app, director, and historic schema
func (s *AnalyticsV2TestSuite) CleanDB() {
	db := s.DB()
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS director CASCADE").Error)
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS historic CASCADE").Error)
	s.Require().NoError(db.Exec("DROP SCHEMA IF EXISTS app CASCADE").Error)

	s.LoadAppSchema(s.T())
	s.LoadDirectorSchema(s.T())
}

func formatChQuery(s string) string {
	reg := regexp.MustCompile("[\n\t ]+")
	return reg.ReplaceAllString(s, "")
}

func v2CombineQueryAndArgs(query string, args []any) string {
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

func (s *AnalyticsV2TestSuite) setupACLHelper(matcher interface{}, times int, acl *auth.ResourceACL) {
	if times >= 0 {
		s.resourceACLHelper.EXPECT().GetResourceACL(gomock.Any(), matcher, gomock.Any()).Times(times).Return(acl, nil)
	} else {
		s.resourceACLHelper.EXPECT().GetResourceACL(gomock.Any(), matcher, gomock.Any()).AnyTimes().Return(acl, nil)
	}
}

func (s *AnalyticsV2TestSuite) UseMockedClickhouseConnections() {
	s.convoMockConn = new(clickhouseConvoTesting.MockConn)
	s.ClickhouseContainerTestSuite.SetConversationConnectionMock(s.customerID, s.profileID, s.convoMockConn)
	s.authEventMockConn = new(clickhouseEventTesting.MockConn)
	s.ClickhouseContainerTestSuite.SetAuthEventsConnectionMock(s.customerID, s.authEventMockConn)
	s.eventMockConn = new(clickhouseEventTesting.MockConn)
	s.ClickhouseContainerTestSuite.SetEventsConnectionMock(s.customerID, s.profileID, s.eventMockConn)
}
