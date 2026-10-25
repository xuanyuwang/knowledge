package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (a AnalyticsServiceImpl) RetrieveInsightsTopics(ctx context.Context, req *analyticspb.RetrieveInsightsTopicsRequest) (*analyticspb.RetrieveInsightsTopicsResponse, error) {
	_, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	if len(req.TopicMomentNames) == 0 {
		return nil, status.Error(codes.InvalidArgument, "TopicMomentNames is not set")
	}

	val, err := a.fetchInsightsTopics(ctx, req.Parent, req.TopicMomentNames, false)
	if err != nil {
		return nil, err
	}
	topics := toAPIInsightsTopic(val)

	return &analyticspb.RetrieveInsightsTopicsResponse{InsightsTopics: topics}, nil
}
