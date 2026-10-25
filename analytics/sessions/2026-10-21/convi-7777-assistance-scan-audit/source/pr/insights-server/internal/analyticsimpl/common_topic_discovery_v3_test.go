package analyticsimpl

import (
	"context"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/shared-go/container/set"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestResolveV3CategoryDescendants(t *testing.T) {
	templates := v3Tree()
	v3Names := set.New[string]()
	for _, m := range templates {
		v3Names.Add(m.Name)
	}
	childrenByParent := v3ChildrenByParent(templates)

	t.Run("non-leaf parent expands to self plus all descendants", func(t *testing.T) {
		// When
		closures := resolveV3CategoryDescendants(
			[]string{v3TestPrefix + "cat-1"}, v3Names, childrenByParent)

		// Then
		require.Contains(t, closures, v3TestPrefix+"cat-1")
		assert.ElementsMatch(t, []string{
			v3TestPrefix + "cat-1",
			v3TestPrefix + "cat-1-1",
			v3TestPrefix + "cat-1-2",
			v3TestPrefix + "cat-1-2-1",
		}, closures[v3TestPrefix+"cat-1"])
	})

	t.Run("non-V3 name is ignored", func(t *testing.T) {
		// When
		closures := resolveV3CategoryDescendants(
			[]string{v3TestPrefix + "not-a-v3-category"}, v3Names, childrenByParent)

		// Then
		assert.Empty(t, closures)
	})
}

func TestExpandV3MomentsInAttribute(t *testing.T) {
	// Given
	closure := map[string][]string{
		v3TestPrefix + "cat-1": {
			v3TestPrefix + "cat-1",
			v3TestPrefix + "cat-1-1",
			v3TestPrefix + "cat-1-2",
		},
	}
	attr := &analyticspb.Attribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{Name: v3TestPrefix + "cat-1", Type: momentpb.Moment_TOPIC},
					{Name: v3TestPrefix + "untouched", Type: momentpb.Moment_KEYWORD},
				},
			},
		},
	}

	// When
	expandV3MomentsInAttribute(attr, closure)

	// Then
	got := attr.MomentGroups[0].Moments
	require.Len(t, got, 4) // 3 from closure + 1 untouched
	for _, m := range got[:3] {
		assert.Equal(t, momentpb.Moment_TOPIC, m.Type)
		assert.Equal(t, momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC, m.DetailedType)
	}
	assert.Equal(t, v3TestPrefix+"untouched", got[3].Name)
	assert.Equal(t, momentpb.Moment_KEYWORD, got[3].Type)
}

func TestResolveV3GroupByClosureTemplateIDs(t *testing.T) {
	templates := v3Tree()
	v3Names := set.New[string]()
	for _, m := range templates {
		v3Names.Add(m.Name)
	}
	childrenByParent := v3ChildrenByParent(templates)

	t.Run("non-leaf category stays one moment and maps to its whole subtree closure", func(t *testing.T) {
		// Given
		category := &momentpb.Moment{Name: v3TestPrefix + "cat-1", Type: momentpb.Moment_TOPIC}
		groupBy := []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{category},
					},
				},
			},
		}

		// When
		closureTemplateIDs, err := resolveV3GroupByClosureTemplateIDs(groupBy, v3Names, childrenByParent)

		// Then
		require.NoError(t, err)
		// The grouped-by category stays a single moment, stamped with the V3 detailed type in place.
		got := groupBy[0].GetMomentGroup().Moments
		require.Len(t, got, 1)
		assert.Equal(t, v3TestPrefix+"cat-1", got[0].Name)
		assert.Equal(t, momentpb.Moment_TOPIC, got[0].Type)
		assert.Equal(t, momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC, got[0].DetailedType)
		// Its bucket OR-matches the full subtree closure template ids (self + every descendant).
		assert.ElementsMatch(t, []string{
			"cat-1",
			"cat-1-1",
			"cat-1-2",
			"cat-1-2-1",
		}, closureTemplateIDs[v3TestPrefix+"cat-1"])
	})

	t.Run("non-V3 group-by moment is ignored", func(t *testing.T) {
		// Given
		groupBy := []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{Name: v3TestPrefix + "not-a-v3-category", Type: momentpb.Moment_TOPIC},
						},
					},
				},
			},
		}

		// When
		closureTemplateIDs, err := resolveV3GroupByClosureTemplateIDs(groupBy, v3Names, childrenByParent)

		// Then
		require.NoError(t, err)
		assert.Empty(t, closureTemplateIDs)
		assert.Equal(t, momentpb.Moment_DETAILED_TYPE_UNSPECIFIED, groupBy[0].GetMomentGroup().Moments[0].DetailedType)
	})
}

// TestClusterChildDocumentType verifies that a chart cluster value set routes to the correct ES child
// document type based on detailed_type, and that mixing Topic Discovery V3 and TDv1/v2 clusters in a
// single value set is rejected (a single has_child filter can only target one child document type).
func TestClusterChildDocumentType(t *testing.T) {
	v3Cluster := &dashboardpb.ClusterForChart{
		MomentName:   v3TestPrefix + "cat-1",
		DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3,
	}
	legacyCluster := &dashboardpb.ClusterForChart{
		MomentName: v3TestPrefix + "cluster-1",
	}

	t.Run("all TDv1/v2 clusters route to the message_group child doc", func(t *testing.T) {
		// When
		got, err := tdClusterDocumentType([]*dashboardpb.ClusterForChart{legacyCluster})

		// Then
		require.NoError(t, err)
		assert.Equal(t, elasticsearch.DocumentTypeMessageGroup, got)
	})

	t.Run("all TDv3 clusters route to the message child doc", func(t *testing.T) {
		// When
		got, err := tdClusterDocumentType([]*dashboardpb.ClusterForChart{v3Cluster})

		// Then
		require.NoError(t, err)
		assert.Equal(t, elasticsearch.DocumentTypeMessage, got)
	})

	t.Run("mixing TDv3 and TDv1/v2 clusters is rejected", func(t *testing.T) {
		// When
		_, err := tdClusterDocumentType([]*dashboardpb.ClusterForChart{v3Cluster, legacyCluster})

		// Then
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

// TestNestedFromClustersMomentIDs verifies nestedFromClusters accepts either a full moment resource
// name or a bare moment ID (including in V3 subtree closures), and dedupes when both forms refer to
// the same template.
func TestNestedFromClustersMomentIDs(t *testing.T) {
	ctx := context.Background()
	momentIDsFromNested := func(t *testing.T, nested *elasticsearch.Nested) []string {
		t.Helper()
		require.NotNil(t, nested)
		require.NotNil(t, nested.Query)
		require.NotNil(t, nested.Query.Bool)
		require.NotNil(t, nested.Query.Bool.Filter)
		filters := *nested.Query.Bool.Filter
		require.Len(t, filters, 1)
		require.NotNil(t, filters[0].Terms)
		require.NotNil(t, filters[0].Terms.MomentTemplateIDs)
		return *filters[0].Terms.MomentTemplateIDs
	}

	t.Run("resource name extracts the moment id", func(t *testing.T) {
		// When
		nested, err := nestedFromClusters(ctx, []*dashboardpb.ClusterForChart{
			{MomentName: v3TestPrefix + "cat-1"},
		}, nil)

		// Then
		require.NoError(t, err)
		assert.Equal(t, []string{"cat-1"}, momentIDsFromNested(t, nested))
	})

	t.Run("bare moment id is used as-is", func(t *testing.T) {
		// When
		nested, err := nestedFromClusters(ctx, []*dashboardpb.ClusterForChart{
			{MomentName: "cat-1"},
		}, nil)

		// Then
		require.NoError(t, err)
		assert.Equal(t, []string{"cat-1"}, momentIDsFromNested(t, nested))
	})

	t.Run("mixed resource names and bare ids, with dedupe", func(t *testing.T) {
		// When
		nested, err := nestedFromClusters(ctx, []*dashboardpb.ClusterForChart{
			{MomentName: v3TestPrefix + "cat-1"},
			{MomentName: "cat-1"},
			{MomentName: "cat-2"},
			{MomentName: v3TestPrefix + "cat-3"},
		}, nil)

		// Then
		require.NoError(t, err)
		assert.Equal(t, []string{"cat-1", "cat-2", "cat-3"}, momentIDsFromNested(t, nested))
	})

	t.Run("v3 closure accepts resource names and bare ids", func(t *testing.T) {
		// Given — closures are keyed/valued by resource name (buildTopicDiscoveryV3ClusterClosures invariant)
		clusterName := v3TestPrefix + "cat-1"
		closures := map[string][]string{
			clusterName: {
				v3TestPrefix + "cat-1",
				v3TestPrefix + "cat-1-1",
				v3TestPrefix + "cat-1-2",
			},
		}

		// When
		nested, err := nestedFromClusters(ctx, []*dashboardpb.ClusterForChart{
			{MomentName: clusterName, DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3},
		}, closures)

		// Then
		require.NoError(t, err)
		assert.Equal(t, []string{"cat-1", "cat-1-1", "cat-1-2"}, momentIDsFromNested(t, nested))
	})

	t.Run("v3 closure keyed by resource name is found when cluster carries bare id", func(t *testing.T) {
		// Given
		closures := map[string][]string{
			v3TestPrefix + "cat-1": {
				v3TestPrefix + "cat-1",
				v3TestPrefix + "cat-1-1",
			},
		}

		// When
		nested, err := nestedFromClusters(ctx, []*dashboardpb.ClusterForChart{
			{MomentName: "cat-1", DetailedType: dashboardpb.ClusterForChart_CONVERSATION_REASON_V3},
		}, closures)

		// Then
		require.NoError(t, err)
		assert.Equal(t, []string{"cat-1", "cat-1-1"}, momentIDsFromNested(t, nested))
	})

	t.Run("empty moment name or id is rejected", func(t *testing.T) {
		// When
		_, err := nestedFromClusters(ctx, []*dashboardpb.ClusterForChart{
			{MomentName: ""},
		}, nil)

		// Then
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

// TestV3ClusterClosuresUseResourceNames verifies the generation path always produces a closures map
// keyed and valued by fully qualified moment resource names, even when the chart cluster (or the
// template ParentMomentTemplateId) carries a bare moment_template_id.
func TestV3ClusterClosuresUseResourceNames(t *testing.T) {
	templates := v3Tree()
	idToName := v3TemplateIDToResourceName(templates)
	v3Names := set.New[string]()
	for _, m := range templates {
		v3Names.Add(m.Name)
	}
	childrenByParent := v3ChildrenByParent(templates)

	t.Run("bare parent ids still build a resource-name keyed child map", func(t *testing.T) {
		// Then — ParentMomentTemplateId is bare in v3Tree; children must still hang off resource names
		assert.ElementsMatch(t, []string{
			v3TestPrefix + "cat-1-1",
			v3TestPrefix + "cat-1-2",
		}, childrenByParent[v3TestPrefix+"cat-1"])
		assert.Empty(t, childrenByParent["cat-1"], "children must not be keyed by bare parent id")
	})

	t.Run("bare cluster moment_template_id normalizes to resource-name closure", func(t *testing.T) {
		// When
		normalized := normalizeToV3ResourceNames([]string{"cat-1"}, idToName)
		closures := resolveV3CategoryDescendants(normalized, v3Names, childrenByParent)

		// Then
		require.Len(t, closures, 1)
		require.Contains(t, closures, v3TestPrefix+"cat-1")
		assert.NotContains(t, closures, "cat-1")
		assert.ElementsMatch(t, []string{
			v3TestPrefix + "cat-1",
			v3TestPrefix + "cat-1-1",
			v3TestPrefix + "cat-1-2",
			v3TestPrefix + "cat-1-2-1",
		}, closures[v3TestPrefix+"cat-1"])
		for _, name := range closures[v3TestPrefix+"cat-1"] {
			parsed, err := momentpb.ParseMomentName(name)
			require.NoError(t, err, "closure entry %q must be a resource name", name)
			assert.NotEmpty(t, parsed.GetMomentID())
		}
	})

	t.Run("resource-name cluster is unchanged", func(t *testing.T) {
		// When
		normalized := normalizeToV3ResourceNames([]string{v3TestPrefix + "cat-1"}, idToName)
		closures := resolveV3CategoryDescendants(normalized, v3Names, childrenByParent)

		// Then
		require.Contains(t, closures, v3TestPrefix+"cat-1")
		assert.ElementsMatch(t, []string{
			v3TestPrefix + "cat-1",
			v3TestPrefix + "cat-1-1",
			v3TestPrefix + "cat-1-2",
			v3TestPrefix + "cat-1-2-1",
		}, closures[v3TestPrefix+"cat-1"])
	})
}
