package analyticsimpl

import (
	"context"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/featureflag"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/framework/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const testTDV2ClusterName = "customers/c/profiles/p/moments/cluster-1"

func testTDV2TimeRange() *analyticspb.TimeRange {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(start),
		EndTimestamp:   timestamppb.New(start.Add(24 * time.Hour)),
	}
}

func TestCompareSets(t *testing.T) {
	t.Run("both empty", func(t *testing.T) {
		// When
		esOnly, pgOnly, both := compareSets(set.New[string](), set.New[string]())

		// Then
		assert.Equal(t, 0, esOnly)
		assert.Equal(t, 0, pgOnly)
		assert.Equal(t, 0, both)
	})

	t.Run("perfect match", func(t *testing.T) {
		// Given
		esSet := set.NewFromSlice([]string{"a", "b"})
		pgSet := set.NewFromSlice([]string{"a", "b"})

		// When
		esOnly, pgOnly, both := compareSets(esSet, pgSet)

		// Then
		assert.Equal(t, 0, esOnly)
		assert.Equal(t, 0, pgOnly)
		assert.Equal(t, 2, both)
	})

	t.Run("es only", func(t *testing.T) {
		// When
		esOnly, pgOnly, both := compareSets(set.NewFromSlice([]string{"a", "b"}), set.New[string]())

		// Then
		assert.Equal(t, 2, esOnly)
		assert.Equal(t, 0, pgOnly)
		assert.Equal(t, 0, both)
	})

	t.Run("pg only", func(t *testing.T) {
		// When
		esOnly, pgOnly, both := compareSets(set.New[string](), set.NewFromSlice([]string{"a"}))

		// Then
		assert.Equal(t, 0, esOnly)
		assert.Equal(t, 1, pgOnly)
		assert.Equal(t, 0, both)
	})

	t.Run("mixed", func(t *testing.T) {
		// Given
		esSet := set.NewFromSlice([]string{"a", "b", "c"})
		pgSet := set.NewFromSlice([]string{"b", "c", "d"})

		// When
		esOnly, pgOnly, both := compareSets(esSet, pgSet)

		// Then
		assert.Equal(t, 1, esOnly)
		assert.Equal(t, 1, pgOnly)
		assert.Equal(t, 2, both)
	})
}

func TestExtractTDV2ClusterCandidates(t *testing.T) {
	t.Run("nil request filter", func(t *testing.T) {
		// When
		clusters := extractTDV2ClusterCandidates(&analyticspb.RetrieveConversationMessagesRequest{})

		// Then
		assert.Empty(t, clusters)
	})

	t.Run("v2 topic clusters only", func(t *testing.T) {
		// Given
		req := &analyticspb.RetrieveConversationMessagesRequest{
			FilterConversationByAttribute: &analyticspb.Attribute{
				MomentGroups: []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{
						{Name: "customers/c/profiles/p/moments/cluster-1", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
						{Name: "customers/c/profiles/p/moments/cluster-2", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_EXTRACTED_QUESTION_TOPIC},
					}},
				},
			},
		}

		// When
		clusters := extractTDV2ClusterCandidates(req)

		// Then
		require.Len(t, clusters, 2)
	})

	t.Run("excludes v3 topics", func(t *testing.T) {
		// Given
		req := &analyticspb.RetrieveConversationMessagesRequest{
			FilterConversationByAttribute: &analyticspb.Attribute{
				MomentGroups: []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{
						{Name: "customers/c/profiles/p/moments/v2-cluster", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
						{Name: "customers/c/profiles/p/moments/v3-cluster", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC},
					}},
				},
			},
		}

		// When
		clusters := extractTDV2ClusterCandidates(req)

		// Then
		require.Len(t, clusters, 1)
		assert.Equal(t, momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC, clusters[0].GetDetailedType())
	})

	t.Run("excludes non-topic moments", func(t *testing.T) {
		// Given
		req := &analyticspb.RetrieveConversationMessagesRequest{
			FilterConversationByAttribute: &analyticspb.Attribute{
				MomentGroups: []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{
						{Name: "customers/c/profiles/p/moments/keyword-1", Type: momentpb.Moment_KEYWORD},
					}},
				},
			},
		}

		// When
		clusters := extractTDV2ClusterCandidates(req)

		// Then
		assert.Empty(t, clusters)
	})
}

func TestEvaluateTDV2ReadMode(t *testing.T) {
	t.Run("nil client returns es_only", func(t *testing.T) {
		// Given
		a := AnalyticsServiceImpl{}

		// When
		mode := a.evaluateTDV2ReadMode(context.Background(), "c", "p")

		// Then
		assert.Equal(t, tdV2ReadModeESOnly, mode)
	})

	t.Run("flag returns dual_read", func(t *testing.T) {
		// Given
		a := AnalyticsServiceImpl{
			featureFlagClient: featureflag.NewStubClient(featureflag.StubClientConfig{
				Strings: map[string]string{tdV2ClusterPGReadModeFlag: "dual_read"},
			}),
		}

		// When
		mode := a.evaluateTDV2ReadMode(context.Background(), "c", "p")

		// Then
		assert.Equal(t, tdV2ReadModeDualRead, mode)
	})

	t.Run("flag returns pg_only", func(t *testing.T) {
		// Given
		a := AnalyticsServiceImpl{
			featureFlagClient: featureflag.NewStubClient(featureflag.StubClientConfig{
				Strings: map[string]string{tdV2ClusterPGReadModeFlag: "pg_only"},
			}),
		}

		// When
		mode := a.evaluateTDV2ReadMode(context.Background(), "c", "p")

		// Then
		assert.Equal(t, tdV2ReadModePGOnly, mode)
	})

	t.Run("unknown value returns es_only", func(t *testing.T) {
		// Given
		a := AnalyticsServiceImpl{
			featureFlagClient: featureflag.NewStubClient(featureflag.StubClientConfig{
				Strings: map[string]string{tdV2ClusterPGReadModeFlag: "bogus"},
			}),
			logger: log.GetLogger(),
		}

		// When
		mode := a.evaluateTDV2ReadMode(context.Background(), "c", "p")

		// Then
		assert.Equal(t, tdV2ReadModeESOnly, mode)
	})
}

func TestCompareTDV2ClusterMembership_NilTimeRange(t *testing.T) {
	// Given — appsDB is nil; if the guard is missing, queryPGClusterMembership panics.
	a := AnalyticsServiceImpl{logger: log.GetLogger()}
	req := &analyticspb.RetrieveConversationMessagesRequest{
		FilterConversationByAttribute: &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{
					{Name: "customers/c/profiles/p/moments/cluster-1", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
				}},
			},
		},
	}
	clusters := extractTDV2ClusterCandidates(req)
	parent := &shared.CustomerProfile{CustomerID: "c", ProfileID: "p"}

	// When / Then — should return without panic
	a.compareTDV2ClusterMembership(context.Background(), parent, req, &analyticspb.RetrieveConversationMessagesResponse{}, clusters)
}

func TestApplyPGOnlyClusterFilter_NilTimeRange(t *testing.T) {
	// Given
	a := AnalyticsServiceImpl{logger: log.GetLogger()}
	req := &analyticspb.RetrieveConversationMessagesRequest{
		FilterConversationByAttribute: &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{
					{Name: "customers/c/profiles/p/moments/cluster-1", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
				}},
			},
		},
	}
	clusters := extractTDV2ClusterCandidates(req)
	parent := &shared.CustomerProfile{CustomerID: "c", ProfileID: "p"}

	// When
	count, err := a.applyPGOnlyClusterFilter(context.Background(), parent, req, clusters)

	// Then
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyPGOnlyClusterFilter_PreExistingConversations(t *testing.T) {
	// Given — appsDB is nil; the guard must fire before any Postgres access.
	a := AnalyticsServiceImpl{logger: log.GetLogger()}
	req := &analyticspb.RetrieveConversationMessagesRequest{
		FilterByTimeRange: testTDV2TimeRange(),
		FilterConversationByAttribute: &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{
					{Name: testTDV2ClusterName, Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
				}},
			},
			Conversations: []*conversationpb.Conversation{
				{Name: "customers/c/profiles/p/conversations/existing-conv"},
			},
		},
	}
	clusters := extractTDV2ClusterCandidates(req)
	parent := &shared.CustomerProfile{CustomerID: "c", ProfileID: "p"}

	// When
	count, err := a.applyPGOnlyClusterFilter(context.Background(), parent, req, clusters)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pre-existing conversation filter")
	assert.Equal(t, 0, count)
}

func TestCompareTDV2ClusterMembership_Paginated(t *testing.T) {
	// Given — appsDB is nil; the pagination guard must fire before any Postgres access. A time
	// range is present so the guard under test is the only thing that can prevent the panic.
	a := AnalyticsServiceImpl{logger: log.GetLogger()}
	parent := &shared.CustomerProfile{CustomerID: "c", ProfileID: "p"}
	clusterFilter := &analyticspb.Attribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{Moments: []*momentpb.Moment{
				{Name: testTDV2ClusterName, Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
			}},
		},
	}

	t.Run("request carries a page token", func(t *testing.T) {
		// Given
		req := &analyticspb.RetrieveConversationMessagesRequest{
			PageToken:                     "sort-key-1",
			FilterByTimeRange:             testTDV2TimeRange(),
			FilterConversationByAttribute: clusterFilter,
		}

		// When / Then — returns without panicking on the nil appsDB
		a.compareTDV2ClusterMembership(context.Background(), parent, req, &analyticspb.RetrieveConversationMessagesResponse{}, extractTDV2ClusterCandidates(req))
	})

	t.Run("response has a next page", func(t *testing.T) {
		// Given
		req := &analyticspb.RetrieveConversationMessagesRequest{
			FilterByTimeRange:             testTDV2TimeRange(),
			FilterConversationByAttribute: clusterFilter,
		}
		resp := &analyticspb.RetrieveConversationMessagesResponse{NextPageToken: "sort-key-2"}

		// When / Then — returns without panicking on the nil appsDB
		a.compareTDV2ClusterMembership(context.Background(), parent, req, resp, extractTDV2ClusterCandidates(req))
	})
}

func TestValidateTDV2ClusterGroupsIsolated(t *testing.T) {
	clusterNames := set.NewFromSlice([]string{testTDV2ClusterName})
	clusterMoment := &momentpb.Moment{Name: testTDV2ClusterName, Type: momentpb.Moment_TOPIC}
	keywordMoment := &momentpb.Moment{Name: "customers/c/profiles/p/moments/keyword-1", Type: momentpb.Moment_KEYWORD}

	t.Run("cluster alone in its group is allowed", func(t *testing.T) {
		// Given
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{clusterMoment}},
				{Moments: []*momentpb.Moment{keywordMoment}},
			},
		}

		// When
		err := validateTDV2ClusterGroupsIsolated(attr, clusterNames)

		// Then
		assert.NoError(t, err)
	})

	t.Run("cluster OR-ed with another moment is rejected", func(t *testing.T) {
		// Given — stripping the cluster would turn (cluster OR behavior) into behavior AND conv IN (...)
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{clusterMoment, keywordMoment}},
			},
		}

		// When
		err := validateTDV2ClusterGroupsIsolated(attr, clusterNames)

		// Then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mixes TDv2 clusters with other moments")
	})

	t.Run("cluster group with excluded moments is rejected", func(t *testing.T) {
		// Given
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{clusterMoment}, ExcludedMoments: []*momentpb.Moment{keywordMoment}},
			},
		}

		// When
		err := validateTDV2ClusterGroupsIsolated(attr, clusterNames)

		// Then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "additional filters")
	})

	t.Run("AND-ed clusters are rejected", func(t *testing.T) {
		// Given — the Postgres read ORs template ids, so an AND-ed group would be widened
		secondCluster := &momentpb.Moment{Name: "customers/c/profiles/p/moments/cluster-2", Type: momentpb.Moment_TOPIC}
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments:              []*momentpb.Moment{clusterMoment, secondCluster},
					BooleanLogicOperator: analyticspb.MomentGroup_BOOLEAN_LOGIC_OPERATOR_AND,
				},
			},
		}

		// When
		err := validateTDV2ClusterGroupsIsolated(attr, set.NewFromSlice([]string{
			testTDV2ClusterName,
			"customers/c/profiles/p/moments/cluster-2",
		}))

		// Then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AND-s")
	})
}

func TestStripTDV2ClustersFromMomentGroups(t *testing.T) {
	t.Run("strips only TDv2 clusters", func(t *testing.T) {
		// Given
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{
					{Name: "customers/c/profiles/p/moments/v2-cluster", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC},
					{Name: "customers/c/profiles/p/moments/v3-topic", Type: momentpb.Moment_TOPIC, DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC},
				}},
			},
		}
		clusterNames := set.NewFromSlice([]string{"customers/c/profiles/p/moments/v2-cluster"})

		// When
		stripTDV2ClustersFromMomentGroups(attr, clusterNames)

		// Then
		require.Len(t, attr.MomentGroups, 1)
		require.Len(t, attr.MomentGroups[0].Moments, 1)
		assert.Equal(t, "customers/c/profiles/p/moments/v3-topic", attr.MomentGroups[0].Moments[0].GetName())
	})

	t.Run("removes empty moment groups", func(t *testing.T) {
		// Given
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{
					{Name: "customers/c/profiles/p/moments/v2-only", Type: momentpb.Moment_TOPIC},
				}},
				{Moments: []*momentpb.Moment{
					{Name: "customers/c/profiles/p/moments/keyword", Type: momentpb.Moment_KEYWORD},
				}},
			},
		}
		clusterNames := set.NewFromSlice([]string{"customers/c/profiles/p/moments/v2-only"})

		// When
		stripTDV2ClustersFromMomentGroups(attr, clusterNames)

		// Then
		require.Len(t, attr.MomentGroups, 1)
		assert.Equal(t, "customers/c/profiles/p/moments/keyword", attr.MomentGroups[0].Moments[0].GetName())
	})

	t.Run("all stripped yields empty", func(t *testing.T) {
		// Given
		attr := &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{Moments: []*momentpb.Moment{
					{Name: "customers/c/profiles/p/moments/c1", Type: momentpb.Moment_TOPIC},
					{Name: "customers/c/profiles/p/moments/c2", Type: momentpb.Moment_TOPIC},
				}},
			},
		}
		clusterNames := set.NewFromSlice([]string{
			"customers/c/profiles/p/moments/c1",
			"customers/c/profiles/p/moments/c2",
		})

		// When
		stripTDV2ClustersFromMomentGroups(attr, clusterNames)

		// Then
		assert.Empty(t, attr.MomentGroups)
	})

	t.Run("nil attr is safe", func(t *testing.T) {
		// When / Then — should not panic
		stripTDV2ClustersFromMomentGroups(nil, set.New[string]())
	})
}

func TestExtractESMembershipSet(t *testing.T) {
	t.Run("conversation mode extracts conv IDs", func(t *testing.T) {
		// Given
		resp := &analyticspb.RetrieveConversationMessagesResponse{
			ConversationMessageResults: []*analyticspb.ConversationMessageResult{
				{ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
					Conversation: &conversationpb.Conversation{Name: "customers/c/profiles/p/conversations/conv-1"},
				}},
				{ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
					Conversation: &conversationpb.Conversation{Name: "customers/c/profiles/p/conversations/conv-2"},
				}},
			},
		}

		// When
		esSet := extractESMembershipSet(resp, false /* isMessageMode */)

		// Then
		assert.Equal(t, 2, esSet.Len())
		assert.True(t, esSet.Has("conv-1"))
		assert.True(t, esSet.Has("conv-2"))
	})

	t.Run("message mode extracts conv:msg pairs", func(t *testing.T) {
		// Given
		resp := &analyticspb.RetrieveConversationMessagesResponse{
			ConversationMessageResults: []*analyticspb.ConversationMessageResult{
				{
					ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
						Conversation: &conversationpb.Conversation{Name: "customers/c/profiles/p/conversations/conv-1"},
					},
					MessageResults: []*analyticspb.ConversationMessageResult_MessageResult{
						{ConversationMessage: &conversationpb.ConversationMessage{Name: "customers/c/profiles/p/conversations/conv-1/messages/msg-A"}},
						{ConversationMessage: &conversationpb.ConversationMessage{Name: "customers/c/profiles/p/conversations/conv-1/messages/msg-B"}},
					},
				},
			},
		}

		// When
		esSet := extractESMembershipSet(resp, true /* isMessageMode */)

		// Then
		assert.Equal(t, 2, esSet.Len())
		assert.True(t, esSet.Has("conv-1:msg-A"))
		assert.True(t, esSet.Has("conv-1:msg-B"))
	})
}
