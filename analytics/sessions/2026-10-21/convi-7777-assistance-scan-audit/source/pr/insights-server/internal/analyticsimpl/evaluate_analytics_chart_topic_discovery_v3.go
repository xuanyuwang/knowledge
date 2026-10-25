package analyticsimpl

import (
	"context"
	"fmt"

	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/shared-go/container/set"
)

// Topic Discovery V3 support for EvaluateAnalyticsChart cluster attributes.
//
// Charts reference topic-discovery categories through dashboardpb.ClusterForChart values (the
// crestaEntityCluster attribute). A TDv3 category is a Moment_TOPIC template carrying
// DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC, arranged in a tree via ParentMomentTemplateId, with
// classification annotations anchored on the "message" child document (unlike TDv1/v2 cluster
// annotations, which live on the "message_group" child document).
//
// Both filters and group-bys expand a V3 category to the SAME full subtree closure (the node itself
// + every descendant, leaf AND non-leaf):
//   - A filter on a parent category matches every conversation classified anywhere beneath it. All
//     descendants are OR-ed within one IN filter, so a value set stays a single filter.
//   - A group-by on a category collapses its whole subtree into a SINGLE bucket (identical closure to
//     the filter), so the bubble for that category counts every conversation classified anywhere
//     beneath it. Because the value set is left unexpanded, each original cluster still maps 1:1 to
//     one bucket and one response group-by value; the closure lives inside the bucket filter, not in
//     the value set.
//
// The closure is computed once per request and threaded to nestedFromClusters as a
// map[momentResourceName][]closureMomentResourceName. Keys and values are always fully qualified
// moment resource names (bare moment_template_ids from the chart are converted). nestedFromClusters
// replaces a V3 cluster's single moment id with the closure's moment ids in the terms filter.

// clustersFromValueSet returns the cluster attribute values referenced by a chart value set.
func clustersFromValueSet(valueSet *dashboardpb.ChartValueSet) []*dashboardpb.ClusterForChart {
	var clusters []*dashboardpb.ClusterForChart
	for _, attributeValue := range valueSet.GetSingleValues().GetAttributeValues() {
		if cluster := attributeValue.GetClusterValue(); cluster != nil {
			clusters = append(clusters, cluster)
		}
	}
	return clusters
}

// collectV3ClusterNames returns the moment names of every V3 cluster referenced across the given
// value sets.
func collectV3ClusterNames(valueSets []*dashboardpb.ChartValueSet) []string {
	names := set.New[string]()
	for _, valueSet := range valueSets {
		for _, cluster := range clustersFromValueSet(valueSet) {
			if cluster.GetDetailedType() != dashboardpb.ClusterForChart_CONVERSATION_REASON_V3 || cluster.GetMomentName() == "" {
				continue
			}
			names.Add(cluster.GetMomentName())
		}
	}
	return names.Slice()
}

// collectFilterClusterNames returns the moment names of every V3 cluster referenced in the chart's
// filter value sets.
func collectFilterClusterNames(filters []*dashboardpb.ChartDataFilter) []string {
	valueSets := make([]*dashboardpb.ChartValueSet, 0, len(filters))
	for _, filter := range filters {
		valueSets = append(valueSets, filter.GetValueSet())
	}
	return collectV3ClusterNames(valueSets)
}

// collectGroupByClusterNames returns the moment names of every V3 cluster referenced in the chart's
// group-by (provided-value) value sets.
func collectGroupByClusterNames(groupBys []*dashboardpb.ChartGroupBy) []string {
	valueSets := make([]*dashboardpb.ChartValueSet, 0, len(groupBys))
	for _, groupBy := range groupBys {
		valueSets = append(valueSets, groupBy.GetByProvidedValue().GetValueSet())
	}
	return collectV3ClusterNames(valueSets)
}

// buildTopicDiscoveryV3ClusterClosures returns, for every V3 category cluster referenced in the
// chart's filters or group-bys, the full subtree closure (the category itself + every descendant,
// leaf AND non-leaf) of moment resource names. Filters and group-bys share the SAME closure, so a
// group-by on a category counts exactly the conversations a filter on that category would match.
//
// The returned map is keyed by moment resource name and every closure entry is a moment resource
// name. Chart clusters may carry a bare moment_template_id; those are normalized to resource names
// before resolution so downstream consumers can always ParseMomentName the map contents. Returns
// nil (a no-op for nestedFromClusters) when no V3 clusters are referenced.
func (a AnalyticsServiceImpl) buildTopicDiscoveryV3ClusterClosures(
	ctx context.Context,
	profile shared.CustomerProfile,
	chartDataConfig *dashboardpb.ChartDataConfig,
) (map[string][]string, error) {
	clusterNames := set.New[string]()
	clusterNames.AddAll(collectFilterClusterNames(chartDataConfig.GetFilters())...)
	clusterNames.AddAll(collectGroupByClusterNames(chartDataConfig.GetGroupBy())...)
	if clusterNames.Len() == 0 {
		return nil, nil
	}

	// Fetch the V3 template tree once (the only RPC), then resolve every cluster's subtree closure
	// from the same template list.
	v3Templates, err := a.getTopicDiscoveryV3Templates(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("fetching Topic Discovery V3 category templates for chart cluster expansion: %w", err)
	}
	a.logger.Infof(ctx, "Retrieved %d Topic Discovery V3 category templates for chart cluster expansion", len(v3Templates))
	idToName := v3TemplateIDToResourceName(v3Templates)
	v3Names := set.New[string]()
	for _, template := range v3Templates {
		if template != nil && template.Name != "" {
			v3Names.Add(template.Name)
		}
	}
	if v3Names.Len() == 0 {
		return nil, nil
	}

	normalizedNames := normalizeToV3ResourceNames(clusterNames.Slice(), idToName)
	if len(normalizedNames) == 0 {
		return nil, nil
	}

	return resolveV3CategoryDescendants(normalizedNames, v3Names, v3ChildrenByParent(v3Templates)), nil
}

// normalizeToV3ResourceNames converts chart cluster identifiers (resource name or bare
// moment_template_id) to fully qualified V3 template resource names using idToName.
func normalizeToV3ResourceNames(nameOrIDs []string, idToName map[string]string) []string {
	normalized := set.New[string]()
	for _, nameOrID := range nameOrIDs {
		if resourceName, ok := idToName[nameOrID]; ok {
			normalized.Add(resourceName)
			continue
		}
		if id := bareMomentID(nameOrID); id != nameOrID {
			if resourceName, ok := idToName[id]; ok {
				normalized.Add(resourceName)
			}
		}
	}
	return normalized.Slice()
}

// lookupV3ClusterClosure finds the subtree closure for a chart cluster moment identifier.
// v3ClusterClosures is keyed by resource name; momentNameOrID may be a resource name or bare id.
func lookupV3ClusterClosure(closures map[string][]string, momentNameOrID string) ([]string, bool) {
	if len(closures) == 0 || momentNameOrID == "" {
		return nil, false
	}
	if closure, ok := closures[momentNameOrID]; ok {
		return closure, true
	}
	momentID := bareMomentID(momentNameOrID)
	if momentID == "" {
		return nil, false
	}
	for key, closure := range closures {
		if bareMomentID(key) == momentID {
			return closure, true
		}
	}
	return nil, false
}
