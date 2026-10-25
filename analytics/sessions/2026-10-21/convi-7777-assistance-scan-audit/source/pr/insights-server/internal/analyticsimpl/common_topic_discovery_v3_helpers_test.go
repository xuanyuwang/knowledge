package analyticsimpl

import (
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
)

const v3TestPrefix = "customers/customer_id/profiles/profile_id/moments/"

// v3Template builds an active Topic Discovery V3 category template for tests.
// ParentMomentTemplateId is a bare moment template id (matching production storage).
func v3Template(id, parentID string) *momentpb.Moment {
	m := &momentpb.Moment{
		Name:          v3TestPrefix + id,
		Type:          momentpb.Moment_TOPIC,
		DetailedType:  momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
		TaxonomyState: momentpb.Moment_STATE_ACTIVE,
		AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
	}
	if parentID != "" {
		m.ParentMomentTemplateId = parentID
	}
	return m
}

// v3ListMomentsMatcher matches the V3 category-tree fetch issued by applyTopicDiscoveryV3Expansion.
// Matching on the V3 detailed type keeps it disjoint from the COMPOSITE and InsightsTopics
// ListMoments calls, so registering it is purely additive.
type v3ListMomentsMatcher struct{}

func (v3ListMomentsMatcher) Matches(x any) bool {
	req, ok := x.(*momentpb.ListMomentsRequest)
	if !ok || req.Filter == nil {
		return false
	}
	for _, dt := range req.Filter.DetailedTypes {
		if dt == momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC {
			return true
		}
	}
	return false
}

func (v3ListMomentsMatcher) String() string {
	return "is a Topic Discovery V3 ListMoments fetch (Filter.DetailedTypes contains V3_TOPIC)"
}

// v3Tree returns a small taxonomy:
//
//	root (synthetic)
//	└── cat-1
//	    ├── cat-1-1            (leaf)
//	    └── cat-1-2            (non-leaf)
//	        └── cat-1-2-1      (leaf)
//	cat-2                      (leaf)
func v3Tree() []*momentpb.Moment {
	return []*momentpb.Moment{
		v3Template("root", ""),
		v3Template("cat-1", "root"),
		v3Template("cat-1-1", "cat-1"),
		v3Template("cat-1-2", "cat-1"),
		v3Template("cat-1-2-1", "cat-1-2"),
		v3Template("cat-2", "root"),
	}
}
