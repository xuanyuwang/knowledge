package analyticsimpl

import (
	"context"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	dbtesting "github.com/cresta/go-servers/shared/db/testing"
)

func TestStatsAPICacheSuite(t *testing.T) {
	suite.Run(t, &StatsAPICacheTestSuite{})
}

type StatsAPICacheTestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite

	server *AnalyticsServiceImpl
	ctx    context.Context
}

func (s *StatsAPICacheTestSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.ctx = context.TODO()
	s.server = &AnalyticsServiceImpl{
		appsDB: s.DBTestSuite.AppsDBRegistry(),
		logger: logger.GetLogger(),
	}
}

func (s *StatsAPICacheTestSuite) TearDownSuite() {
	s.DBTestSuite.TearDownSuite(s.T())
}

func (s *StatsAPICacheTestSuite) SetupTest() {
	*insightsStatsAPICachingCustomerIDs = map[string]struct{}{}
	dur := 4 * time.Hour
	*insightsStatsAPICachingDuration = dur
}

func (s *StatsAPICacheTestSuite) TestSortAttributeUsersAndGroupsForStatsCache() {
	tests := []struct {
		name     string
		attr     *analyticspb.Attribute
		expected *analyticspb.Attribute
	}{
		{
			name:     "nil attribute",
			attr:     nil,
			expected: nil,
		},
		{
			name:     "empty attribute",
			attr:     &analyticspb.Attribute{},
			expected: &analyticspb.Attribute{},
		},
		{
			name: "users already sorted",
			attr: &analyticspb.Attribute{
				Users: []*userpb.User{
					{Name: "alice"},
					{Name: "bob"},
				},
			},
			expected: &analyticspb.Attribute{
				Users: []*userpb.User{
					{Name: "alice"},
					{Name: "bob"},
				},
			},
		},
		{
			name: "users unsorted",
			attr: &analyticspb.Attribute{
				Users: []*userpb.User{
					{Name: "charlie"},
					{Name: "alice"},
					{Name: "bob"},
				},
			},
			expected: &analyticspb.Attribute{
				Users: []*userpb.User{
					{Name: "alice"},
					{Name: "bob"},
					{Name: "charlie"},
				},
			},
		},
		{
			name: "groups unsorted",
			attr: &analyticspb.Attribute{
				Groups: []*userpb.Group{
					{Name: "z-group"},
					{Name: "a-group"},
				},
			},
			expected: &analyticspb.Attribute{
				Groups: []*userpb.Group{
					{Name: "a-group"},
					{Name: "z-group"},
				},
			},
		},
		{
			name: "both users and groups unsorted",
			attr: &analyticspb.Attribute{
				Users: []*userpb.User{
					{Name: "bob"},
					{Name: "alice"},
				},
				Groups: []*userpb.Group{
					{Name: "z-group"},
					{Name: "a-group"},
				},
			},
			expected: &analyticspb.Attribute{
				Users: []*userpb.User{
					{Name: "alice"},
					{Name: "bob"},
				},
				Groups: []*userpb.Group{
					{Name: "a-group"},
					{Name: "z-group"},
				},
			},
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// Given
			attr := tc.attr

			// When
			sortAttributeUsersAndGroupsForStatsCache(attr)

			// Then
			s.True(proto.Equal(tc.expected, attr),
				"expected %v, got %v", tc.expected, attr)
		})
	}
}

func (s *StatsAPICacheTestSuite) TestSortQAAttributeUsersAndGroupsForStatsCache() {
	tests := []struct {
		name     string
		attr     *analyticspb.QAAttribute
		expected *analyticspb.QAAttribute
	}{
		{
			name:     "nil attribute",
			attr:     nil,
			expected: nil,
		},
		{
			name:     "empty attribute",
			attr:     &analyticspb.QAAttribute{},
			expected: &analyticspb.QAAttribute{},
		},
		{
			name: "users unsorted",
			attr: &analyticspb.QAAttribute{
				Users: []*userpb.User{
					{Name: "charlie"},
					{Name: "alice"},
				},
			},
			expected: &analyticspb.QAAttribute{
				Users: []*userpb.User{
					{Name: "alice"},
					{Name: "charlie"},
				},
			},
		},
		{
			name: "groups unsorted",
			attr: &analyticspb.QAAttribute{
				Groups: []*userpb.Group{
					{Name: "z-group"},
					{Name: "a-group"},
				},
			},
			expected: &analyticspb.QAAttribute{
				Groups: []*userpb.Group{
					{Name: "a-group"},
					{Name: "z-group"},
				},
			},
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// Given
			attr := tc.attr

			// When
			sortQAAttributeUsersAndGroupsForStatsCache(attr)

			// Then
			s.True(proto.Equal(tc.expected, attr),
				"expected %v, got %v", tc.expected, attr)
		})
	}
}

func (s *StatsAPICacheTestSuite) TestSortRetrieveMetadataValuesRequestForStatsCache() {
	tests := []struct {
		name     string
		req      *analyticspb.RetrieveMetadataValuesRequest
		expected []string
	}{
		{
			name:     "nil request",
			req:      nil,
			expected: nil,
		},
		{
			name:     "nil filter",
			req:      &analyticspb.RetrieveMetadataValuesRequest{},
			expected: nil,
		},
		{
			name: "sorts usecase names",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				FilterOnConversation: &analyticspb.RetrieveMetadataValuesRequest_FilterOnConversation{
					UsecaseNames: []string{"charlie", "alice", "bob"},
				},
			},
			expected: []string{"alice", "bob", "charlie"},
		},
		{
			name: "already sorted",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				FilterOnConversation: &analyticspb.RetrieveMetadataValuesRequest_FilterOnConversation{
					UsecaseNames: []string{"a", "b", "c"},
				},
			},
			expected: []string{"a", "b", "c"},
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// When
			sortRetrieveMetadataValuesRequestForStatsCache(tc.req)

			// Then
			if tc.expected == nil {
				return
			}
			s.Equal(tc.expected, tc.req.FilterOnConversation.UsecaseNames)
		})
	}
}

func (s *StatsAPICacheTestSuite) TestGetInsightsStatsFromCache() {
	profileName := &customerpb.ProfileName{
		CustomerID: "test-customer",
		ProfileID:  "test-profile",
	}
	cacheKey := &analyticspb.RetrieveConversationStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", profileName.CustomerID, profileName.ProfileID),
	}

	tests := []struct {
		name            string
		enabledCustomer string
		preSeed         bool
		expectFound     bool
		expectStoreNoop bool
	}{
		{
			name:            "caching disabled returns noop store",
			enabledCustomer: "",
			preSeed:         false,
			expectFound:     false,
			expectStoreNoop: true,
		},
		{
			name:            "cache miss returns working store",
			enabledCustomer: "test-customer",
			preSeed:         false,
			expectFound:     false,
			expectStoreNoop: false,
		},
		{
			name:            "cache hit returns noop store",
			enabledCustomer: "test-customer",
			preSeed:         true,
			expectFound:     true,
			expectStoreNoop: true,
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// Given
			s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)
			if tc.enabledCustomer != "" {
				*insightsStatsAPICachingCustomerIDs = map[string]struct{}{tc.enabledCustomer: {}}
			} else {
				*insightsStatsAPICachingCustomerIDs = map[string]struct{}{}
			}

			if tc.preSeed {
				seededResp := &analyticspb.RetrieveConversationStatsResponse{
					TotalConversationCount: 42,
				}
				_, seedStore, err := s.server.getInsightsStatsFromCache(s.ctx, "TestMethod", insightsStatsAPICachingCustomerIDs, false, profileName, cacheKey, &analyticspb.RetrieveConversationStatsResponse{})
				s.Require().NoError(err)
				seedStore(seededResp, nil)
			}

			// When
			var out analyticspb.RetrieveConversationStatsResponse
			found, store, err := s.server.getInsightsStatsFromCache(s.ctx, "TestMethod", insightsStatsAPICachingCustomerIDs, false, profileName, cacheKey, &out)

			// Then
			s.Require().NoError(err)
			s.Equal(tc.expectFound, found)

			if tc.expectFound {
				s.Equal(int32(42), out.TotalConversationCount)
			}

			// Verify whether the store function is a noop by calling it and checking DB state.
			s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)
			store(&analyticspb.RetrieveConversationStatsResponse{TotalConversationCount: 99}, nil)

			var count int64
			s.Require().NoError(s.DB().Table("app.cached_requests").Count(&count).Error)
			if tc.expectStoreNoop {
				s.Equal(int64(0), count, "store should be noop")
			} else {
				s.Equal(int64(1), count, "store should have written to DB")
			}
		})
	}
}

func (s *StatsAPICacheTestSuite) TestStoreFuncSkipsOnNilResponseOrError() {
	profileName := &customerpb.ProfileName{
		CustomerID: "test-customer",
		ProfileID:  "test-profile",
	}
	cacheKey := &analyticspb.RetrieveConversationStatsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", profileName.CustomerID, profileName.ProfileID),
	}

	tests := []struct {
		name        string
		response    proto.Message
		err         error
		expectStore bool
	}{
		{
			name:        "nil response skips store",
			response:    nil,
			err:         nil,
			expectStore: false,
		},
		{
			name:        "non-nil error skips store",
			response:    &analyticspb.RetrieveConversationStatsResponse{TotalConversationCount: 1},
			err:         fmt.Errorf("some error"),
			expectStore: false,
		},
		{
			name:        "valid response and nil error stores",
			response:    &analyticspb.RetrieveConversationStatsResponse{TotalConversationCount: 1},
			err:         nil,
			expectStore: true,
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// Given
			*insightsStatsAPICachingCustomerIDs = map[string]struct{}{"test-customer": {}}
			s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)

			_, store, err := s.server.getInsightsStatsFromCache(
				s.ctx, "TestMethod", insightsStatsAPICachingCustomerIDs, false, profileName, cacheKey, &analyticspb.RetrieveConversationStatsResponse{})
			s.Require().NoError(err)

			// When
			store(tc.response, tc.err)

			// Then
			var count int64
			s.Require().NoError(s.DB().Table("app.cached_requests").Count(&count).Error)
			if tc.expectStore {
				s.Equal(int64(1), count)
			} else {
				s.Equal(int64(0), count)
			}
		})
	}
}

// TestCacheRoundTripPerRPC is a smoke test that verifies the cache store+retrieve round-trip
// works for each stats RPC's request/response proto type pair.
func (s *StatsAPICacheTestSuite) TestCacheRoundTripPerRPC() {
	profileName := &customerpb.ProfileName{
		CustomerID: "smoke-customer",
		ProfileID:  "smoke-profile",
	}
	parent := fmt.Sprintf("customers/%s/profiles/%s", profileName.CustomerID, profileName.ProfileID)

	tests := []struct {
		method   string
		cacheKey proto.Message
		storeVal proto.Message
		out      func() proto.Message
	}{
		{
			method:   "RetrieveConversationStats",
			cacheKey: &analyticspb.RetrieveConversationStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveConversationStatsResponse{TotalConversationCount: 7},
			out:      func() proto.Message { return &analyticspb.RetrieveConversationStatsResponse{} },
		},
		{
			method:   "RetrieveHintStats",
			cacheKey: &analyticspb.RetrieveHintStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveHintStatsResponse{TotalConversationCount: 8},
			out:      func() proto.Message { return &analyticspb.RetrieveHintStatsResponse{} },
		},
		{
			method:   "RetrieveKnowledgeAssistStats",
			cacheKey: &analyticspb.RetrieveKnowledgeAssistStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveKnowledgeAssistStatsResponse{GenaiAnswersViewedCount: 9},
			out:      func() proto.Message { return &analyticspb.RetrieveKnowledgeAssistStatsResponse{} },
		},
		{
			method:   "RetrieveSummarizationStats",
			cacheKey: &analyticspb.RetrieveSummarizationStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveSummarizationStatsResponse{TotalConversationCount: 10},
			out:      func() proto.Message { return &analyticspb.RetrieveSummarizationStatsResponse{} },
		},
		{
			method:   "RetrieveNoteTakingStats",
			cacheKey: &analyticspb.RetrieveNoteTakingStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveNoteTakingStatsResponse{TotalConversationCount: 11},
			out:      func() proto.Message { return &analyticspb.RetrieveNoteTakingStatsResponse{} },
		},
		{
			method:   "RetrieveGuidedWorkflowStats",
			cacheKey: &analyticspb.RetrieveGuidedWorkflowStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveGuidedWorkflowStatsResponse{TotalConversationCount: 12},
			out:      func() proto.Message { return &analyticspb.RetrieveGuidedWorkflowStatsResponse{} },
		},
		{
			method:   "RetrieveMetadataValues",
			cacheKey: &analyticspb.RetrieveMetadataValuesRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveMetadataValuesResponse{},
			out:      func() proto.Message { return &analyticspb.RetrieveMetadataValuesResponse{} },
		},
		{
			method:   "RetrieveQAScoreStats",
			cacheKey: &analyticspb.RetrieveQAScoreStatsRequest{Parent: parent},
			storeVal: &analyticspb.RetrieveQAScoreStatsResponse{QaScoreResult: &analyticspb.QAScoreResult{TotalScorecardCount: 13}},
			out:      func() proto.Message { return &analyticspb.RetrieveQAScoreStatsResponse{} },
		},
	}
	for _, tc := range tests {
		s.Run(tc.method, func() {
			// Given
			*insightsStatsAPICachingCustomerIDs = map[string]struct{}{"smoke-customer": {}}
			s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)

			_, store, err := s.server.getInsightsStatsFromCache(s.ctx, tc.method, insightsStatsAPICachingCustomerIDs, false, profileName, tc.cacheKey, tc.out())
			s.Require().NoError(err)
			store(tc.storeVal, nil)

			// When
			out := tc.out()
			found, _, err := s.server.getInsightsStatsFromCache(s.ctx, tc.method, insightsStatsAPICachingCustomerIDs, false, profileName, tc.cacheKey, out)

			// Then
			s.Require().NoError(err)
			s.True(found, "expected cache hit for %s", tc.method)
			s.True(proto.Equal(tc.storeVal, out),
				"round-trip mismatch for %s: want %v, got %v", tc.method, tc.storeVal, out)
		})
	}

	s.Run("RetrieveAnnotatedTimeSeries", func() {
		// Given
		*insightsAnnotatedTimeSeriesCachingCustomerIDs = map[string]struct{}{"smoke-customer": {}}
		s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)

		cacheKey := &analyticspb.RetrieveAnnotatedTimeSeriesRequest{Parent: parent}
		storeVal := &analyticspb.RetrieveAnnotatedTimeSeriesResponse{}

		_, store, err := s.server.getInsightsStatsFromCache(
			s.ctx, "RetrieveAnnotatedTimeSeries", insightsAnnotatedTimeSeriesCachingCustomerIDs, false, profileName, cacheKey, &analyticspb.RetrieveAnnotatedTimeSeriesResponse{})
		s.Require().NoError(err)
		store(storeVal, nil)

		// When
		var out analyticspb.RetrieveAnnotatedTimeSeriesResponse
		found, _, err := s.server.getInsightsStatsFromCache(
			s.ctx, "RetrieveAnnotatedTimeSeries", insightsAnnotatedTimeSeriesCachingCustomerIDs, false, profileName, cacheKey, &out)

		// Then
		s.Require().NoError(err)
		s.True(found, "expected cache hit for RetrieveAnnotatedTimeSeries")
		s.True(proto.Equal(storeVal, &out),
			"round-trip mismatch for RetrieveAnnotatedTimeSeries: want %v, got %v", storeVal, &out)
	})

	s.Run("RetrieveConversationMessageStats", func() {
		// Given
		*insightsConversationMessageStatsCachingCustomerIDs = map[string]struct{}{"smoke-customer": {}}
		*insightsConversationMessageStatsCachingAllCustomers = false
		s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)

		cacheKey := &analyticspb.RetrieveConversationMessageStatsRequest{Parent: parent}
		storeVal := &analyticspb.RetrieveConversationMessageStatsResponse{
			AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
				TotalConversationCount: 42,
			},
		}

		_, store, err := s.server.getInsightsStatsFromCache(
			s.ctx, "RetrieveConversationMessageStats", insightsConversationMessageStatsCachingCustomerIDs, false, profileName, cacheKey, &analyticspb.RetrieveConversationMessageStatsResponse{})
		s.Require().NoError(err)
		store(storeVal, nil)

		// When
		var out analyticspb.RetrieveConversationMessageStatsResponse
		found, _, err := s.server.getInsightsStatsFromCache(
			s.ctx, "RetrieveConversationMessageStats", insightsConversationMessageStatsCachingCustomerIDs, false, profileName, cacheKey, &out)

		// Then
		s.Require().NoError(err)
		s.True(found, "expected cache hit for RetrieveConversationMessageStats")
		s.True(proto.Equal(storeVal, &out),
			"round-trip mismatch for RetrieveConversationMessageStats: want %v, got %v", storeVal, &out)
	})

	s.Run("RetrieveConversationMessageStats_allowAllCustomers", func() {
		// Given — empty per-customer allowlist, but INSIGHTS_CONVERSATION_MESSAGE_STATS_CACHING_ALL_CUSTOMERS
		*insightsConversationMessageStatsCachingCustomerIDs = map[string]struct{}{}
		*insightsConversationMessageStatsCachingAllCustomers = true
		defer func() {
			*insightsConversationMessageStatsCachingAllCustomers = false
		}()
		s.Require().NoError(s.DB().Exec("DELETE FROM app.cached_requests WHERE true").Error)

		cacheKey := &analyticspb.RetrieveConversationMessageStatsRequest{Parent: parent}
		storeVal := &analyticspb.RetrieveConversationMessageStatsResponse{
			AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
				TotalConversationCount: 43,
			},
		}

		_, store, err := s.server.getInsightsStatsFromCache(
			s.ctx, "RetrieveConversationMessageStats", insightsConversationMessageStatsCachingCustomerIDs, true, profileName, cacheKey, &analyticspb.RetrieveConversationMessageStatsResponse{})
		s.Require().NoError(err)
		store(storeVal, nil)

		// When
		var out analyticspb.RetrieveConversationMessageStatsResponse
		found, _, err := s.server.getInsightsStatsFromCache(
			s.ctx, "RetrieveConversationMessageStats", insightsConversationMessageStatsCachingCustomerIDs, true, profileName, cacheKey, &out)

		// Then
		s.Require().NoError(err)
		s.True(found)
		s.True(proto.Equal(storeVal, &out))
	})
}

func (s *StatsAPICacheTestSuite) TestSortRetrieveConversationMessageStatsRequestForCache_NormalizesMomentOrder() {
	// Given — same logical moments in different slice orders
	parent := "customers/smoke-customer/profiles/smoke-profile"
	reqA := &analyticspb.RetrieveConversationMessageStatsRequest{
		Parent: parent,
		FilterConversationByAttribute: &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{Name: "customers/smoke-customer/profiles/smoke-profile/moments/m-b", Type: momentpb.Moment_TOPIC},
						{Name: "customers/smoke-customer/profiles/smoke-profile/moments/m-a", Type: momentpb.Moment_TOPIC},
					},
				},
			},
		},
	}
	reqB := proto.Clone(reqA).(*analyticspb.RetrieveConversationMessageStatsRequest)
	reqB.FilterConversationByAttribute.MomentGroups[0].Moments = []*momentpb.Moment{
		{Name: "customers/smoke-customer/profiles/smoke-profile/moments/m-a", Type: momentpb.Moment_TOPIC},
		{Name: "customers/smoke-customer/profiles/smoke-profile/moments/m-b", Type: momentpb.Moment_TOPIC},
	}

	// When
	keyA := proto.Clone(reqA).(*analyticspb.RetrieveConversationMessageStatsRequest)
	sortRetrieveConversationMessageStatsRequestForCache(keyA)
	keyB := proto.Clone(reqB).(*analyticspb.RetrieveConversationMessageStatsRequest)
	sortRetrieveConversationMessageStatsRequestForCache(keyB)

	// Then — normalized cache keys match despite differing input order
	s.True(proto.Equal(keyA, keyB))
}

func (s *StatsAPICacheTestSuite) TestSortRetrieveConversationMessageStatsRequestForCache_NormalizesGroupByMomentOrder() {
	// Given — same logical GroupByAttributes moments in different slice orders
	parent := "customers/smoke-customer/profiles/smoke-profile"
	momentA := &momentpb.Moment{Name: "customers/smoke-customer/profiles/smoke-profile/moments/m-a", Type: momentpb.Moment_TOPIC}
	momentB := &momentpb.Moment{Name: "customers/smoke-customer/profiles/smoke-profile/moments/m-b", Type: momentpb.Moment_TOPIC}
	reqA := &analyticspb.RetrieveConversationMessageStatsRequest{
		Parent: parent,
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{momentB, momentA},
					},
				},
			},
		},
	}
	reqB := proto.Clone(reqA).(*analyticspb.RetrieveConversationMessageStatsRequest)
	reqB.GroupByAttributes[0].GetMomentGroup().Moments = []*momentpb.Moment{momentA, momentB}

	// When
	keyA := proto.Clone(reqA).(*analyticspb.RetrieveConversationMessageStatsRequest)
	sortRetrieveConversationMessageStatsRequestForCache(keyA)
	keyB := proto.Clone(reqB).(*analyticspb.RetrieveConversationMessageStatsRequest)
	sortRetrieveConversationMessageStatsRequestForCache(keyB)

	// Then — normalized cache keys match despite differing input order
	s.True(proto.Equal(keyA, keyB))
}

func (s *StatsAPICacheTestSuite) TestConversationMessageStatsCachingEligible() {
	profileName := &customerpb.ProfileName{CustomerID: "cust-a", ProfileID: "prof-1"}
	tests := []struct {
		name         string
		allowlist    map[string]struct{}
		allCustomers bool
		want         bool
	}{
		{
			name:         "allowlisted customer",
			allowlist:    map[string]struct{}{"cust-a": {}},
			allCustomers: false,
			want:         true,
		},
		{
			name:         "not allowlisted",
			allowlist:    map[string]struct{}{"other": {}},
			allCustomers: false,
			want:         false,
		},
		{
			name:         "all customers override",
			allowlist:    map[string]struct{}{},
			allCustomers: true,
			want:         true,
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			defer func() {
				*insightsConversationMessageStatsCachingCustomerIDs = map[string]struct{}{}
				*insightsConversationMessageStatsCachingAllCustomers = false
			}()
			*insightsConversationMessageStatsCachingCustomerIDs = tc.allowlist
			*insightsConversationMessageStatsCachingAllCustomers = tc.allCustomers

			// When
			got := conversationMessageStatsCachingEligible(profileName)

			// Then
			s.Equal(tc.want, got)
		})
	}
}

func (s *StatsAPICacheTestSuite) TestGroupByMomentsAllTopicForCache() {
	topicMoment := &momentpb.Moment{Name: "customers/c/profiles/p/moments/m-topic", Type: momentpb.Moment_TOPIC}
	nonTopicMoment := &momentpb.Moment{Name: "customers/c/profiles/p/moments/m-keyword", Type: momentpb.Moment_KEYWORD}
	momentGroupAttr := func(moments ...*momentpb.Moment) *analyticspb.AttributeStructure {
		return &analyticspb.AttributeStructure{
			AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
				MomentGroup: &analyticspb.MomentGroup{Moments: moments},
			},
		}
	}
	timeRangeAttr := &analyticspb.AttributeStructure{
		AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
			AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
		},
	}

	tests := []struct {
		name              string
		groupByAttributes []*analyticspb.AttributeStructure
		flagEnabled       bool
		want              bool
	}{
		{
			name:              "flag disabled — always true regardless of attributes",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr(nonTopicMoment)},
			flagEnabled:       false,
			want:              true,
		},
		{
			name:              "empty groupBy — false",
			groupByAttributes: nil,
			flagEnabled:       true,
			want:              false,
		},
		{
			name:              "single TOPIC moment group — true",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr(topicMoment)},
			flagEnabled:       true,
			want:              true,
		},
		{
			name:              "multiple TOPIC moment groups — true",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr(topicMoment), momentGroupAttr(topicMoment)},
			flagEnabled:       true,
			want:              true,
		},
		{
			name:              "empty moment group — false",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr()},
			flagEnabled:       true,
			want:              false,
		},
		{
			name:              "non-TOPIC moment in group — false",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr(nonTopicMoment)},
			flagEnabled:       true,
			want:              false,
		},
		{
			name:              "mixed TOPIC and non-TOPIC moments in same group — false",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr(topicMoment, nonTopicMoment)},
			flagEnabled:       true,
			want:              false,
		},
		{
			name:              "non-MomentGroup attribute — false",
			groupByAttributes: []*analyticspb.AttributeStructure{timeRangeAttr},
			flagEnabled:       true,
			want:              false,
		},
		{
			name:              "mix of MomentGroup and non-MomentGroup attributes — false",
			groupByAttributes: []*analyticspb.AttributeStructure{momentGroupAttr(topicMoment), timeRangeAttr},
			flagEnabled:       true,
			want:              false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			// Given
			*insightsConversationMessageStatsCachingRequireTopicMoments = tc.flagEnabled
			defer func() { *insightsConversationMessageStatsCachingRequireTopicMoments = true }()

			// When
			got := groupByMomentsAllTopicForCache(tc.groupByAttributes)

			// Then
			s.Equal(tc.want, got)
		})
	}
}
