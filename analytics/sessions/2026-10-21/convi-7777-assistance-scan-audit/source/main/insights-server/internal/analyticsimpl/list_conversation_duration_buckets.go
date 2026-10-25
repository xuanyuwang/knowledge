package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"google.golang.org/protobuf/proto"
)

// hardcodedConversationDurationBuckets mirrors the static data in
// historic.conversation_duration_bins. The bucket IDs and edges are derived
// from toConversationBucket in shared/clickhouse/conversations/scorecard_score.go.
var hardcodedConversationDurationBuckets = []*analyticspb.ConversationDurationBucket{
	{Id: "1", LowerBound: 0, UpperBound: 60},
	{Id: "2", LowerBound: 60, UpperBound: 120},
	{Id: "3", LowerBound: 120, UpperBound: 180},
	{Id: "4", LowerBound: 180, UpperBound: 240},
	{Id: "5", LowerBound: 240, UpperBound: 300},
	{Id: "6", LowerBound: 300, UpperBound: 600},
	{Id: "7", LowerBound: 600, UpperBound: 1200},
	{Id: "8", LowerBound: 1200, UpperBound: 2400},
	{Id: "9", LowerBound: 2400, UpperBound: 3600},
	{Id: "10", LowerBound: 3600, UpperBound: 0},
}

func (a AnalyticsServiceImpl) ListConversationDurationBuckets(ctx context.Context, req *analyticspb.ListConversationDurationBucketsRequest) (*analyticspb.ListConversationDurationBucketsResponse, error) {
	if _, err := shared.ParseCustomerProfile(req.Parent); err != nil {
		return nil, err
	}
	out := make([]*analyticspb.ConversationDurationBucket, len(hardcodedConversationDurationBuckets))
	for i, b := range hardcodedConversationDurationBuckets {
		out[i] = proto.Clone(b).(*analyticspb.ConversationDurationBucket)
	}
	return &analyticspb.ListConversationDurationBucketsResponse{
		ConversationDurationBuckets: out,
	}, nil
}
