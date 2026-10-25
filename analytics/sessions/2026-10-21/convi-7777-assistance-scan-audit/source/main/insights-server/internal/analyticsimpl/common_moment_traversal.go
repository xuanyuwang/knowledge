package analyticsimpl

import (
	"context"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/shared-go/container/set"
)

// isAggregateCategory returns true if the moment is an aggregate (COMPOSITE with no moment data).
// An aggregate moment is a COMPOSITE moment with nil MomentData or nil CompositeMomentData.
// This is the canonical definition used throughout the codebase.
func isAggregateCategory(m *momentpb.Moment) bool {
	return m != nil &&
		m.GetType() == momentpb.Moment_COMPOSITE &&
		(m.GetMomentData() == nil || m.GetMomentData().GetCompositeMomentData() == nil)
}

// TraverseMomentTemplateHierarchy traverses the complete hierarchy of moment templates
// starting from the given parent template, returning all descendant moment names.
func (a AnalyticsServiceImpl) TraverseMomentTemplateHierarchy(ctx context.Context, momentName string, allMoments []*momentpb.Moment) ([]string, error) {
	a.logger.Infof(ctx, "TraverseMomentTemplateHierarchy: momentName: %s", momentName)

	// Build moment lookup map and parent-child relationships
	momentByName := make(map[string]*momentpb.Moment)
	childrenMap := make(map[string][]string)
	for _, moment := range allMoments {
		momentByName[moment.Name] = moment
		if parentID := moment.ParentMomentTemplateId; parentID != "" {
			childrenMap[parentID] = append(childrenMap[parentID], moment.Name)
		}
	}

	// Find root moment
	_, ok := momentByName[momentName]
	if !ok {
		a.logger.Errorf(ctx, "Root template: %s not found.", momentName)
		return nil, nil
	}

	// Traverse hierarchy and collect all descendants
	visitedMoments := set.New[string]()
	traverseDescendants(momentName, childrenMap, visitedMoments)

	// Return sorted list of all descendants
	result := visitedMoments.Slice()
	sort.Strings(result)
	return result, nil
}

// traverseDescendants recursively collects all moments in the hierarchy tree starting from nodeName.
func traverseDescendants(nodeName string, childrenMap map[string][]string, visited set.Set[string]) {
	visited.Add(nodeName)
	for _, childName := range childrenMap[nodeName] {
		if !visited.Has(childName) {
			traverseDescendants(childName, childrenMap, visited)
		}
	}
}

// getMomentTemplatesByMomentAndAppType retrieves all moment templates for the given customer_id, profile_id, moment_types, and app_types.
func (a AnalyticsServiceImpl) getMomentTemplatesByMomentAndAppType(ctx context.Context, profile shared.CustomerProfile, momentTypes []momentpb.Moment_Type, appTypes []momentpb.Moment_AppType) ([]*momentpb.Moment, error) {
	var allMoments []*momentpb.Moment
	pageToken := ""

	for {
		listResp, err := a.momentClientRegistry.Client(ctx).ListMoments(ctx, &momentpb.ListMomentsRequest{
			Parent: profile.String(),
			Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
				MomentTypes: momentTypes,
				AppTypes:    appTypes,
				States:      []momentpb.Moment_State{momentpb.Moment_STATE_ACTIVE},
			},
			PageSize:  500,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, err
		}

		allMoments = append(allMoments, listResp.Moments...)

		if listResp.NextPageToken == "" {
			break
		}
		pageToken = listResp.NextPageToken
	}

	return allMoments, nil
}

// resolveAggregateMoments classifies the given moment names into aggregate and non-aggregate,
// then traverses the moment hierarchy for each aggregate to collect its leaf descendant names.
// An aggregate moment is a COMPOSITE moment with empty moment data (no CompositeMomentData).
func (a AnalyticsServiceImpl) resolveAggregateMoments(
	ctx context.Context,
	profile shared.CustomerProfile,
	momentNames []string,
) (aggregateToLeaves map[string][]string, nonAggregateNames set.Set[string], err error) {
	if len(momentNames) == 0 {
		return nil, set.New[string](), nil
	}

	allCompositeMoments, err := a.getMomentTemplatesByMomentAndAppType(ctx, profile,
		[]momentpb.Moment_Type{momentpb.Moment_COMPOSITE},
		[]momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS})
	if err != nil {
		return nil, nil, err
	}
	a.logger.Infof(ctx, "Retrieved %d composite moment templates for hierarchy traversal", len(allCompositeMoments))

	aggregateSet, nonAggregateNames := splitAggregateMoments(allCompositeMoments, momentNames)
	if aggregateSet.Len() == 0 {
		return nil, nonAggregateNames, nil
	}

	// Build moment lookup map for classification
	momentByName := make(map[string]*momentpb.Moment)
	for _, moment := range allCompositeMoments {
		momentByName[moment.Name] = moment
	}

	aggregateToLeaves = make(map[string][]string)
	for name := range aggregateSet.All() {
		allDescendants, err := a.TraverseMomentTemplateHierarchy(ctx, name, allCompositeMoments)
		if err != nil {
			return nil, nil, err
		}

		// Collect all descendants except aggregates (the root itself and any nested aggregates).
		// We use momentByName to check if each descendant is an aggregate category.
		var leafNodes []string
		for _, descendantName := range allDescendants {
			// Skip the aggregate itself
			if descendantName == name {
				continue
			}
			// Look up the moment to check if it's an aggregate
			m := momentByName[descendantName]
			if m == nil {
				// Moment not in our list, skip it and log a warning
				a.logger.Warnf(ctx, "Descendant moment %s not found in momentByName map", descendantName)
				continue
			}
			// Skip aggregate categories.
			if isAggregateCategory(m) {
				continue
			}
			leafNodes = append(leafNodes, descendantName)
		}

		if len(leafNodes) == 0 {
			// Keep the aggregate in the map with an empty slice so callers (e.g.
			// convertAggregateSubcategory, expandMomentsList) treat it as an aggregate with
			// no leaf matches instead of omitting it and falling back to non-aggregate behavior.
			aggregateToLeaves[name] = []string{}
		} else {
			aggregateToLeaves[name] = leafNodes
		}
	}

	return aggregateToLeaves, nonAggregateNames, nil
}

// expandAggregateMomentsInAttribute replaces aggregate moments in the attribute's MomentGroups
// with their leaf descendants, so that downstream ES query building can look up topic details
// for each individual leaf moment.
func expandAggregateMomentsInAttribute(attr *analyticspb.Attribute, aggregateToLeaves map[string][]string) {
	if attr == nil || len(aggregateToLeaves) == 0 {
		return
	}
	for _, mg := range attr.MomentGroups {
		mg.Moments = expandMomentsList(mg.Moments, aggregateToLeaves)
		mg.ExcludedMoments = expandMomentsList(mg.ExcludedMoments, aggregateToLeaves)
	}
}

func expandMomentsList(moments []*momentpb.Moment, aggregateToLeaves map[string][]string) []*momentpb.Moment {
	var expanded []*momentpb.Moment
	for _, m := range moments {
		if leaves, ok := aggregateToLeaves[m.Name]; ok {
			for _, leafName := range leaves {
				expanded = append(expanded, &momentpb.Moment{
					Name: leafName,
					Type: momentpb.Moment_COMPOSITE,
				})
			}
		} else {
			expanded = append(expanded, m)
		}
	}
	return expanded
}

// expandAndDeduplicateMomentsList expands aggregate moments to their leaves and deduplicates
// the resulting list. This ensures that if a leaf moment appears both explicitly in the input
// and as a descendant of an aggregate, it only appears once in the output.
func expandAndDeduplicateMomentsList(moments []*momentpb.Moment, aggregateToLeaves map[string][]string) []*momentpb.Moment {
	expanded := expandMomentsList(moments, aggregateToLeaves)

	// Deduplicate by moment name
	seen := make(map[string]*momentpb.Moment)
	for _, m := range expanded {
		if _, exists := seen[m.Name]; !exists {
			seen[m.Name] = m
		}
	}

	// Convert back to slice maintaining a consistent order
	result := make([]*momentpb.Moment, 0, len(seen))
	for _, m := range expanded {
		if m2, exists := seen[m.Name]; exists {
			result = append(result, m2)
			delete(seen, m.Name) // Remove to avoid duplicates
		}
	}

	return result
}

// expandAggregateMomentsInMomentGroup replaces aggregate moments in a MomentGroup with their
// leaf descendants and deduplicates the result.
func expandAggregateMomentsInMomentGroup(mg *analyticspb.MomentGroup, aggregateToLeaves map[string][]string) {
	if mg == nil || len(aggregateToLeaves) == 0 {
		return
	}
	mg.Moments = expandAndDeduplicateMomentsList(mg.Moments, aggregateToLeaves)
	mg.ExcludedMoments = expandAndDeduplicateMomentsList(mg.ExcludedMoments, aggregateToLeaves)
}
