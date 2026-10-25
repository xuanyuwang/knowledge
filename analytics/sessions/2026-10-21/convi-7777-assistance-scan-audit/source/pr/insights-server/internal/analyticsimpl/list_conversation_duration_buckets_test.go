package analyticsimpl

import (
	"fmt"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

type ListConversationDurationBucketsTestSuite struct {
	AnalyticsTestSuite
}

func TestListConversationDurationBuckets(t *testing.T) {
	sut := &ListConversationDurationBucketsTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	sut.SetupSuite()

	// Given
	req := &analyticspb.ListConversationDurationBucketsRequest{
		Parent: fmt.Sprintf("customers/%s/profiles/%s", CustomerID, ProfileID),
	}

	// When
	got, err := sut.server.ListConversationDurationBuckets(sut.ctx, req)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff(hardcodedConversationDurationBuckets, got.ConversationDurationBuckets, protocmp.Transform()); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%v", diff)
	}
}
