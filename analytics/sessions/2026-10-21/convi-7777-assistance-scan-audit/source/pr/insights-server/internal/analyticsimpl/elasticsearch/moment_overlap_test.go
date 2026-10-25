package elasticsearch

import (
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
)

func TestConvertConvoMomentGroup_IncludedAndExcludedSameMoment(t *testing.T) {
	momentName := "customers/cust/profiles/prof/moments/metadata-string"
	m := &momentpb.Moment{
		Name: momentName,
		Type: momentpb.Moment_CONVERSATION_METADATA,
	}
	mg := &analyticspb.MomentGroup{
		Moments:         []*momentpb.Moment{m},
		ExcludedMoments: []*momentpb.Moment{m},
		MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
			{
				MetadataValue: &metadata.MetadataValue{
					Value: &metadata.MetadataValue_StringValue{StringValue: "foo"},
				},
			},
		},
	}

	incl, excl, err := convertConvoMomentGroupToConvoFilters(mg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if excl != nil {
		t.Fatalf("unexpected excl filter: %+v", excl)
	}
	if incl == nil || incl.Bool == nil || incl.Bool.Should == nil {
		t.Fatalf("include filter should contain a Bool Should: %+v", incl)
	}
	if incl.Bool.MinimumShouldMatch != 1 {
		t.Fatalf("expected MinimumShouldMatch=1, got %d", incl.Bool.MinimumShouldMatch)
	}
	should := *incl.Bool.Should
	if len(should) != 2 {
		t.Fatalf("expected exactly 2 should branches (present, missing); got %d", len(should))
	}

	var present, missing *ConvoFilterBoolFilter
	for i := range should {
		br := should[i]
		if br.Nested != nil {
			present = &br
		} else if br.Bool != nil && br.Bool.MustNot != nil && br.Bool.MustNot.Nested != nil {
			missing = &br
		}
	}
	if present == nil || missing == nil {
		t.Fatalf("expected both present(Nested) and missing(MustNot Nested) branches: %+v", should)
	}

	if present.Nested.Path != MomentAnnotationsPath {
		t.Fatalf("present nested path mismatch: got %s", present.Nested.Path)
	}
	if present.Nested.Query == nil || present.Nested.Query.Bool == nil || present.Nested.Query.Bool.Filter == nil {
		t.Fatalf("present nested missing bool.filter: %+v", present.Nested)
	}
	presentFilters := *present.Nested.Query.Bool.Filter
	foundMomentIDTerms := false
	foundValueTerms := false
	for _, f := range presentFilters {
		if f.Terms != nil {
			if f.Terms.MomentTemplateIDs != nil {
				foundMomentIDTerms = true
			}
			if f.Terms.MetadataStringPayloadValues != nil || f.Terms.MetadataNumberPayloadValues != nil || f.Terms.MetadataBooleanPayloadValues != nil || f.Terms.MetadataNumberValueExplicitMapping != nil {
				foundValueTerms = true
			}
		}
	}
	if !foundMomentIDTerms || !foundValueTerms {
		t.Fatalf("present branch should include both moment id terms and value terms: %+v", presentFilters)
	}

	if missing.Bool.MustNot.Nested.Path != MomentAnnotationsPath {
		t.Fatalf("missing nested path mismatch: got %s", missing.Bool.MustNot.Nested.Path)
	}
	if missing.Bool.MustNot.Nested.Query == nil || missing.Bool.MustNot.Nested.Query.Bool == nil || missing.Bool.MustNot.Nested.Query.Bool.Filter == nil {
		t.Fatalf("missing nested missing bool.filter: %+v", missing.Bool.MustNot.Nested)
	}
	missingFilters := *missing.Bool.MustNot.Nested.Query.Bool.Filter
	foundMissingMomentID := false
	for _, f := range missingFilters {
		if f.Terms != nil && f.Terms.MomentTemplateIDs != nil {
			foundMissingMomentID = true
			break
		}
	}
	if !foundMissingMomentID {
		t.Fatalf("missing branch should include moment id terms: %+v", missingFilters)
	}
}
