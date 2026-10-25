package analyticsimpl

import (
	"testing"
	"time"

	trendandanomalypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/trend_and_anomaly"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	esLibrary "github.com/elastic/go-elasticsearch/v8"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	testingfake "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/testing"
	clickhouseConvoTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	clickhouseEventTesting "github.com/cresta/go-servers/shared/clickhouse/events/testing"
	estesting "github.com/cresta/go-servers/shared/elasticsearch/testing"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

// StatsAPICacheRPCSmokeTestSuite verifies that each stats RPC handler works
// correctly when caching is enabled: the call succeeds and the response is
// stored in the request cache table.
type StatsAPICacheRPCSmokeTestSuite struct {
	AnalyticsTestSuite
}

func TestStatsAPICacheRPCSmoke(t *testing.T) {
	suite.Run(t, &StatsAPICacheRPCSmokeTestSuite{})
}

func (s *StatsAPICacheRPCSmokeTestSuite) SetupSuite() {
	s.AnalyticsTestSuite.SetupSuite()
	s.UseMockedClickhouseConnections()

	// Some RPCs (ConversationStats, HintStats) call configClient.GetProfileConfig
	// via shared.IncludeConversationSourceBot. Return an empty profile config.
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

func (s *StatsAPICacheRPCSmokeTestSuite) SetupTest() {
	*insightsStatsAPICachingCustomerIDs = map[string]struct{}{s.customerID: {}}
	*insightsQAScoreStatsCachingCustomerIDs = map[string]struct{}{s.customerID: {}}
	*insightsStatsAPICachingDuration = 4 * time.Hour
	s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TearDownTest() {
	*insightsStatsAPICachingCustomerIDs = map[string]struct{}{}
	*insightsQAScoreStatsCachingCustomerIDs = map[string]struct{}{}
}

func (s *StatsAPICacheRPCSmokeTestSuite) setupConvoMockRowsOnce() {
	mockRows := clickhouseConvoTesting.NewMockRows(nil)
	s.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()
}

func (s *StatsAPICacheRPCSmokeTestSuite) setupEventMockRowsOnce() {
	mockRows := clickhouseEventTesting.NewMockRows(nil)
	s.eventMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()
}

func (s *StatsAPICacheRPCSmokeTestSuite) assertCacheStored() {
	var count int64
	s.Require().NoError(s.DB().Table("app.cached_requests").Count(&count).Error)
	s.Equal(int64(1), count, "expected cache entry to be stored")
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveConversationStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveConversationStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveConversationStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveConversationStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveHintStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveHintStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveHintStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveHintStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveKnowledgeAssistStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveKnowledgeAssistStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveKnowledgeAssistStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveKnowledgeAssistStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveSummarizationStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveSummarizationStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveSummarizationStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveSummarizationStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveNoteTakingStats() {
	// Given
	s.setupEventMockRowsOnce()
	defer s.eventMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveNoteTakingStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveNoteTakingStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveNoteTakingStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveQAScoreStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveQAScoreStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveQAScoreStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveQAScoreStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveQAScoreStats_CacheDisabledByAllowlist() {
	// Given
	*insightsQAScoreStatsCachingCustomerIDs = map[string]struct{}{}
	mockRows := clickhouseConvoTesting.NewMockRows(nil)
	s.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Twice()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveQAScoreStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call executes the query directly
	resp, err := s.server.RetrieveQAScoreStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	var count int64
	s.Require().NoError(s.DB().Table("app.cached_requests").Count(&count).Error)
	s.Equal(int64(0), count, "expected no cache entry when QA score stats customer is not allowlisted")

	// When - second call executes the query again
	resp2, err := s.server.RetrieveQAScoreStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveGuidedWorkflowStats() {
	// Given
	s.setupEventMockRowsOnce()
	defer s.eventMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveGuidedWorkflowStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveGuidedWorkflowStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveGuidedWorkflowStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveAgentStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveAgentStatsRequest{
		Parent:    s.customerResourceName,
		ProfileId: s.profileID,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveAgentStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveAgentStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveKnowledgeBaseStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveKnowledgeBaseStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveKnowledgeBaseStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveKnowledgeBaseStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveLiveAssistStats() {
	// Given
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveLiveAssistStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveLiveAssistStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveLiveAssistStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveSmartComposeStats() {
	// Given — SmartCompose uses events ClickHouse + historic Postgres DB
	s.setupEventMockRowsOnce()
	defer s.eventMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveSmartComposeStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveSmartComposeStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveSmartComposeStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveAssistanceStats() {
	// Given — Assistance uses conversations ClickHouse + historic Postgres DB
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveAssistanceStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveAssistanceStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveAssistanceStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveSuggestionStats() {
	// Given — SuggestionStats uses conversations ClickHouse
	s.setupConvoMockRowsOnce()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveSuggestionStatsRequest{
		Parent: s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.Now(),
			EndTimestamp:   timestamppb.Now(),
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata:  &analyticspb.Metadata{TimeZoneId: s.losAngeles},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveSuggestionStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveSuggestionStats(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveMetadataValues() {
	// Given — MetadataValues needs a moment client mock; use ClickHouse source to avoid ES.
	origMomentClientRegistry := s.server.momentClientRegistry
	defer func() { s.server.momentClientRegistry = origMomentClientRegistry }()

	fakeMomentClient := &testingfake.FakeMomentServer{}
	momentName := "customers/customer_id/profiles/profile_id/moments/test-moment-1"
	_, err := fakeMomentClient.CreateMoment(s.ctx, &momentpb.CreateMomentRequest{
		Parent: s.profileResourceName,
		Moment: &momentpb.Moment{
			Name: momentName,
			Type: momentpb.Moment_CONVERSATION_METADATA,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_ConversationMetadataMomentData{
					ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
						Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
					},
				},
			},
		},
	})
	s.Require().NoError(err)
	s.server.momentClientRegistry = clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)

	mockRows := clickhouseConvoTesting.NewMockRows([][]any{{"val1", uint64(1)}})
	s.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	req := &analyticspb.RetrieveMetadataValuesRequest{
		Parent:     s.profileResourceName,
		MomentName: momentName,
		Source:     analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveMetadataValues(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ClickHouse query)
	resp2, err := s.server.RetrieveMetadataValues(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}

func (s *StatsAPICacheRPCSmokeTestSuite) TestRetrieveAnnotatedTimeSeries() {
	// Given — AnnotatedTimeSeries needs ES client, trend-and-anomaly client,
	// and uses a separate cache allowlist. Each call issues two ES searches
	// (denominator + numerator); the mock ES server returns empty aggregation
	// buckets for both. Because the second call must be served from the cache,
	// only two searches are expected in total — if caching regressed, the extra
	// searches would fail the strict mock expectations below.
	mockESServer := estesting.NewMockElasticSearchServer(s.T())
	emptyESResponse := `{"aggregations":{"time_ranges":{"buckets":[]}}}`
	mockESServer.Resps = append(mockESServer.Resps, emptyESResponse, emptyESResponse)
	// This is a caching smoke test, not an ES query test, so match any search
	// body via FilterJSONFields() (which ignores all fields). The index name
	// follows the default convention from sharedes.GetIndexName.
	esIndexName := "customer_id-profile_id-conversations-v2"
	mockESServer.EXPECT().Search(esIndexName, "{}", cmputil.FilterJSONFields())
	mockESServer.EXPECT().Search(esIndexName, "{}", cmputil.FilterJSONFields())

	esClient, err := esLibrary.NewClient(esLibrary.Config{Addresses: []string{mockESServer.URL()}})
	s.Require().NoError(err)
	origESClient := s.server.esClient
	origConfigClient := s.server.configClient
	origTrendClient := s.server.trendAndAnomalyClientRegistry
	defer func() {
		s.server.esClient = origESClient
		s.server.configClient = origConfigClient
		s.server.trendAndAnomalyClientRegistry = origTrendClient
	}()
	s.server.esClient = esClient
	s.server.configClient = &testingfake.FakeConfig{}
	s.server.trendAndAnomalyClientRegistry = clientregistry.NewTestRegistry[trendandanomalypb.TrendAndAnomalyServiceClient](&testingfake.FakeTrendAndAnomalyServer{})

	*insightsAnnotatedTimeSeriesCachingCustomerIDs = map[string]struct{}{s.customerID: {}}
	defer func() { *insightsAnnotatedTimeSeriesCachingCustomerIDs = map[string]struct{}{} }()

	now := time.Now().UTC().Truncate(time.Hour)
	req := &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent: s.profileResourceName,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(now.Add(-35 * 24 * time.Hour)),
			EndTimestamp:   timestamppb.New(now),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(now.Add(-7 * 24 * time.Hour)),
			EndTimestamp:   timestamppb.New(now),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(now.Add(-14 * 24 * time.Hour)),
			EndTimestamp:   timestamppb.New(now),
		},
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_ABSOLUTE_CONVERSATION_COUNT,
		Frequency:    analyticspb.Frequency_DAILY,
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
				AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
			}},
			{AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
				MomentGroup: &analyticspb.MomentGroup{
					Moments: []*momentpb.Moment{{Name: "customers/customer_id/profiles/profile_id/moments/m1"}},
				},
			}},
		},
	}

	// When - first call populates the cache
	resp, err := s.server.RetrieveAnnotatedTimeSeries(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp)
	s.assertCacheStored()

	// When - second call should use the cache (no additional ES query)
	resp2, err := s.server.RetrieveAnnotatedTimeSeries(s.ctx, req)

	// Then
	s.Require().NoError(err)
	s.NotNil(resp2)
}
