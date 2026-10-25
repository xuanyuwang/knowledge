package analyticsimpl

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	envutil "github.com/cresta/shared-go/util/env"
)

// ConversationMessageStatsCacheRPCSmokeTestSuite verifies RetrieveConversationMessageStats
// request caching (Postgres request cache + Elasticsearch): first call executes ES and stores
// the response; second call hits the cache without issuing another Search when flags allow it.
type ConversationMessageStatsCacheRPCSmokeTestSuite struct {
	AnalyticsV2TestSuite
}

func TestConversationMessageStatsCacheRPCSmoke(t *testing.T) {
	suite.Run(t, &ConversationMessageStatsCacheRPCSmokeTestSuite{})
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) SetupTest() {
	*insightsStatsAPICachingDuration = 4 * time.Hour
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) SetupSubTest() {
	*insightsConversationMessageStatsCachingCustomerIDs = map[string]struct{}{}
	*insightsConversationMessageStatsCachingAllCustomers = false
	s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) TearDownSubTest() {
	*insightsConversationMessageStatsCachingCustomerIDs = map[string]struct{}{}
	*insightsConversationMessageStatsCachingAllCustomers = false
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) assertCacheStored() {
	var count int64
	s.Require().NoError(s.DB().Table("app.cached_requests").Count(&count).Error)
	s.Equal(int64(1), count, "expected cache entry to be stored")
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) groupByWeeklyRequest() *analyticspb.RetrieveConversationMessageStatsRequest {
	return &analyticspb.RetrieveConversationMessageStatsRequest{
		Parent:   s.profileResourceName,
		Metadata: &analyticspb.Metadata{TimeZoneId: "UTC"},
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
		},
		Frequency: analyticspb.Frequency_WEEKLY,
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
		},
	}
}

func (s *ConversationMessageStatsCacheRPCSmokeTestSuite) TestRetrieveConversationMessageStats_Caching() {
	const (
		esReqPath  = "RetrieveConversationMessageStats_GroupByWeekly_es_request.json"
		esRespPath = "RetrieveConversationMessageStats_GroupByWeekly_es_response.json"
	)

	testCases := []struct {
		name              string
		allowlistCustomer bool
		allCustomers      bool
		expectESCalls     int
		expectCacheStored bool
	}{
		{
			name:              "caches on second call when customer is allowlisted",
			allowlistCustomer: true,
			expectESCalls:     1,
			expectCacheStored: true,
		},
		{
			name:              "cache disabled when customer not allowlisted",
			allowlistCustomer: false,
			expectESCalls:     2,
			expectCacheStored: false,
		},
		{
			name:              "caches when all customers enabled without allowlist",
			allCustomers:      true,
			expectESCalls:     1,
			expectCacheStored: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Given
			if tc.allowlistCustomer {
				*insightsConversationMessageStatsCachingCustomerIDs = map[string]struct{}{s.customerID: {}}
			}
			*insightsConversationMessageStatsCachingAllCustomers = tc.allCustomers

			req := s.groupByWeeklyRequest()
			s.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(2).
				Return(&config.ProfileConfig{}, nil)
			s.mockConfigClient.EXPECT().
				Get(gomock.Any()).AnyTimes().
				Return(&config.CustomerConfig{}, nil)

			for range tc.expectESCalls {
				s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile(esRespPath))
				s.mockESServer.EXPECT().Search(s.IndexName, s.getFile(esReqPath), cmputil.IgnoreJSONFields("fields"))
			}

			// When — first call runs Elasticsearch (or is served from cache on repeat)
			resp1, err := s.Client.RetrieveConversationMessageStats(s.ctx, req)

			// Then
			s.Require().NoError(err)
			s.NotNil(resp1)
			if tc.expectCacheStored {
				s.assertCacheStored()
			} else {
				var count int64
				s.Require().NoError(s.DB().Table("app.cached_requests").Count(&count).Error)
				s.Equal(int64(0), count, "expected no cache entry")
			}

			// When — second call (hits cache or Elasticsearch depending on eligibility)
			resp2, err := s.Client.RetrieveConversationMessageStats(s.ctx, req)

			// Then
			s.Require().NoError(err)
			s.NotNil(resp2)
		})
	}
}
