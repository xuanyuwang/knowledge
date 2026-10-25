package analyticsimpl

import (
	"context"
	"fmt"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/util/envflag"
)

// topicDiscoveryV3TemplatePageSize is the ListMoments page size used when fetching Topic Discovery V3
// category templates for cluster expansion.
var topicDiscoveryV3TemplatePageSize = envflag.Int("TOPIC_DISCOVERY_V3_TEMPLATE_PAGE_SIZE", 20, "Page size for fetching Topic Discovery V3 category templates for cluster expansion.")

// Topic Discovery V3 categories are stored as Moment_TOPIC templates carrying the
// DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC detailed type, arranged in a tree via
// ParentMomentTemplateId. A classification annotation lands on the conversation's last "message"
// child document (message-anchored), matched purely by moment_template_id. This file expands a
// queried category into the full subtree (the node itself + every descendant, leaf AND non-leaf) so
// a filter on a parent category matches every conversation classified anywhere beneath it.

// getTopicDiscoveryV3Templates returns all active Topic Discovery V3 category templates for the
// profile (Moment_TOPIC + DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC + APP_TYPE_INSIGHTS).
func (a AnalyticsServiceImpl) getTopicDiscoveryV3Templates(ctx context.Context, profile shared.CustomerProfile) ([]*momentpb.Moment, error) {
	var allMoments []*momentpb.Moment
	pageToken := ""
	for {
		listResp, err := a.momentClientRegistry.Client(ctx).ListMoments(ctx, &momentpb.ListMomentsRequest{
			Parent: profile.String(),
			Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
				MomentTypes:   []momentpb.Moment_Type{momentpb.Moment_TOPIC},
				DetailedTypes: []momentpb.Moment_DetailedType{momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC},
				AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
				States:        []momentpb.Moment_State{momentpb.Moment_STATE_ACTIVE, momentpb.Moment_STATE_BUILD_IN_PROGRESS},
			},
			PageSize:  int32(*topicDiscoveryV3TemplatePageSize),
			PageToken: pageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching Topic Discovery V3 category templates for TDv3 cluster expansion: %w", err)
		}
		allMoments = append(allMoments, listResp.Moments...)
		if listResp.NextPageToken == "" {
			break
		}
		pageToken = listResp.NextPageToken
	}
	return allMoments, nil
}

// bareMomentID returns the MomentID portion of a moment identifier. It accepts either a full
// resource name (e.g. "customers/c/profiles/p/moments/m") or a bare MomentID and returns the bare
// form in both cases; empty input returns "".
func bareMomentID(momentIDOrName string) string {
	if momentIDOrName == "" {
		return ""
	}
	if parsed, err := momentpb.ParseMomentName(momentIDOrName); err == nil && parsed.GetMomentID() != "" {
		return parsed.GetMomentID()
	}
	return momentIDOrName
}

// v3TemplateIDToResourceName maps both bare moment template ids and full resource names to the
// template's fully qualified resource name. ParentMomentTemplateId is stored as a bare id in
// production, while Moment.Name is a resource name — this index lets callers normalize either form.
func v3TemplateIDToResourceName(v3Templates []*momentpb.Moment) map[string]string {
	idToName := make(map[string]string, len(v3Templates)*2)
	for _, m := range v3Templates {
		if m == nil || m.Name == "" {
			continue
		}
		idToName[m.Name] = m.Name
		if id := bareMomentID(m.Name); id != "" {
			idToName[id] = m.Name
		}
	}
	return idToName
}

// v3ChildrenByParent maps each template's resource name to its child template resource names, built
// once so subtree closures can be resolved for many categories without rescanning the whole forest
// per lookup. ParentMomentTemplateId may be a bare id or a resource name; both are normalized to the
// parent's resource name before indexing.
func v3ChildrenByParent(v3Templates []*momentpb.Moment) map[string][]string {
	idToName := v3TemplateIDToResourceName(v3Templates)
	childrenByParent := make(map[string][]string, len(v3Templates))
	for _, m := range v3Templates {
		if m == nil || m.ParentMomentTemplateId == "" || m.Name == "" {
			continue
		}
		parentName, ok := idToName[m.ParentMomentTemplateId]
		if !ok {
			parentName, ok = idToName[bareMomentID(m.ParentMomentTemplateId)]
		}
		if !ok {
			continue
		}
		childrenByParent[parentName] = append(childrenByParent[parentName], m.Name)
	}
	return childrenByParent
}

// collectV3SubtreeClosure returns the category itself plus every descendant (leaf AND non-leaf), sorted.
func collectV3SubtreeClosure(categoryName string, childrenByParent map[string][]string) []string {
	visited := set.New[string]()
	traverseDescendants(categoryName, childrenByParent, visited)
	closure := visited.Slice()
	sort.Strings(closure)
	return closure
}

// resolveV3CategoryDescendants returns, for each name that is a V3 category, the closure of {the
// category itself + every descendant, leaf AND non-leaf}; non-V3 names are skipped. Unlike
// resolveAggregateMoments (leaves only, drops the root), V3 filtering matches the whole subtree.
func resolveV3CategoryDescendants(
	momentNames []string,
	v3Names set.Set[string],
	childrenByParent map[string][]string,
) map[string][]string {
	categoryToClosure := map[string][]string{}
	for _, name := range momentNames {
		if !v3Names.Has(name) {
			continue
		}
		categoryToClosure[name] = collectV3SubtreeClosure(name, childrenByParent)
	}
	return categoryToClosure
}

// expandV3MomentsList replaces any V3 category in the list with its full subtree closure, stamping
// each expanded moment as TOPIC/V3 so the ES builder routes it through the message-anchored path.
func expandV3MomentsList(moments []*momentpb.Moment, categoryToClosure map[string][]string) []*momentpb.Moment {
	if len(categoryToClosure) == 0 {
		return moments
	}
	seen := set.New[string]()
	var expanded []*momentpb.Moment
	for _, m := range moments {
		closure, ok := categoryToClosure[m.Name]
		if !ok {
			expanded = append(expanded, m)
			continue
		}
		for _, name := range closure {
			if seen.Has(name) {
				continue
			}
			seen.Add(name)
			expanded = append(expanded, &momentpb.Moment{
				Name:         name,
				Type:         momentpb.Moment_TOPIC,
				DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
			})
		}
	}
	return expanded
}

// expandV3MomentsInAttribute expands V3 categories in a filter attribute's moment groups (included and
// excluded) to their subtree closures.
func expandV3MomentsInAttribute(attr *analyticspb.Attribute, categoryToClosure map[string][]string) {
	if attr == nil || len(categoryToClosure) == 0 {
		return
	}
	for _, mg := range attr.MomentGroups {
		mg.Moments = expandV3MomentsList(mg.Moments, categoryToClosure)
		mg.ExcludedMoments = expandV3MomentsList(mg.ExcludedMoments, categoryToClosure)
	}
}

// resolveV3GroupByClosureTemplateIDs stamps each grouped-by V3 category with the V3 detailed type in
// place and returns a map from its moment name to its subtree-closure template ids. The category stays
// a single moment (one bucket / one series) whose bucket later OR-matches the whole closure.
func resolveV3GroupByClosureTemplateIDs(
	groupByAttributes []*analyticspb.AttributeStructure,
	v3Names set.Set[string],
	childrenByParent map[string][]string,
) (map[string][]string, error) {
	closureTemplateIDs := map[string][]string{}
	for _, att := range groupByAttributes {
		mg := att.GetMomentGroup()
		if mg == nil {
			continue
		}
		for _, m := range mg.Moments {
			if m == nil || m.Type != momentpb.Moment_TOPIC || !v3Names.Has(m.Name) {
				continue
			}
			// Stamp so ConvertMomentGroupToFilter routes this moment through the message-anchored V3 path.
			m.DetailedType = momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC
			if _, done := closureTemplateIDs[m.Name]; done {
				continue
			}
			templateIDs, err := momentNamesToTemplateIDs(collectV3SubtreeClosure(m.Name, childrenByParent))
			if err != nil {
				return nil, err
			}
			closureTemplateIDs[m.Name] = templateIDs
		}
	}
	return closureTemplateIDs, nil
}

// momentNamesToTemplateIDs extracts the template id from each fully qualified moment resource name.
func momentNamesToTemplateIDs(names []string) ([]string, error) {
	return fn.MapE(names, func(name string) (string, error) {
		parsed, err := momentpb.ParseMomentName(name)
		if err != nil {
			return "", err
		}
		return parsed.MomentID, nil
	})
}

// isV3TopicMoment reports whether a moment references a Topic Discovery V3 category. The frontend
// stamps DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC on V3 category moments, so gating on it lets
// ordinary TDv1/v2 topic filters skip the V3 template fetch entirely.
func isV3TopicMoment(m *momentpb.Moment) bool {
	return m != nil && m.Type == momentpb.Moment_TOPIC &&
		m.DetailedType == momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC && m.Name != ""
}

// collectFilterV3TopicNames returns the names of all V3-detailed-typed TOPIC moments in the filter
// attributes (included and excluded).
func collectFilterV3TopicNames(filterAttributes []*analyticspb.Attribute) []string {
	names := set.New[string]()
	for _, attr := range filterAttributes {
		if attr == nil {
			continue
		}
		for _, mg := range attr.MomentGroups {
			for _, m := range mg.Moments {
				if isV3TopicMoment(m) {
					names.Add(m.Name)
				}
			}
			for _, m := range mg.ExcludedMoments {
				if isV3TopicMoment(m) {
					names.Add(m.Name)
				}
			}
		}
	}
	return names.Slice()
}

// collectGroupByV3TopicNames returns the names of all V3-detailed-typed TOPIC moments in the
// group-by attributes.
func collectGroupByV3TopicNames(groupByAttributes []*analyticspb.AttributeStructure) []string {
	names := set.New[string]()
	for _, att := range groupByAttributes {
		mg := att.GetMomentGroup()
		if mg == nil {
			continue
		}
		for _, m := range mg.Moments {
			if isV3TopicMoment(m) {
				names.Add(m.Name)
			}
		}
	}
	return names.Slice()
}

// applyTopicDiscoveryV3Expansion rewrites V3 category filters/group-bys so the ES query targets
// message-anchored V3 annotations. Filters are expanded in place to their full subtree closure; group-by
// categories are stamped in place and their subtree-closure template ids are returned (moment name ->
// template ids) for the caller to record on TopicDetails. No-op (nil map) when no V3 categories appear.
func (a AnalyticsServiceImpl) applyTopicDiscoveryV3Expansion(
	ctx context.Context,
	profile shared.CustomerProfile,
	filterAttributes []*analyticspb.Attribute,
	groupByAttributes []*analyticspb.AttributeStructure,
) (map[string][]string, error) {
	filterNames := collectFilterV3TopicNames(filterAttributes)
	groupByNames := collectGroupByV3TopicNames(groupByAttributes)
	if len(filterNames) == 0 && len(groupByNames) == 0 {
		return nil, nil
	}

	// Fetch the V3 template tree once, then resolve both expansions from the same list.
	v3Templates, err := a.getTopicDiscoveryV3Templates(ctx, profile)
	if err != nil {
		return nil, err
	}
	a.logger.Infof(ctx, "Retrieved %d Topic Discovery V3 category templates for hierarchy traversal", len(v3Templates))
	v3Names := set.New[string]()
	for _, m := range v3Templates {
		v3Names.Add(m.Name)
	}
	if v3Names.Len() == 0 {
		return nil, nil
	}

	childrenByParent := v3ChildrenByParent(v3Templates)
	categoryToClosure := resolveV3CategoryDescendants(filterNames, v3Names, childrenByParent)
	for _, attr := range filterAttributes {
		expandV3MomentsInAttribute(attr, categoryToClosure)
	}

	return resolveV3GroupByClosureTemplateIDs(groupByAttributes, v3Names, childrenByParent)
}
