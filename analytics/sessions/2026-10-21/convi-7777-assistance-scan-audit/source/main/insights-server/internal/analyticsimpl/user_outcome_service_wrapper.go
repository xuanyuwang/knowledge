package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
)

type userOutcomeServiceWrapper struct {
	analyticspb.UnimplementedUserOutcomeServiceServer
	analyticsService *AnalyticsServiceImpl
}

func (w *userOutcomeServiceWrapper) RetrieveUserOutcomeStats(
	ctx context.Context,
	req *analyticspb.RetrieveUserOutcomeStatsRequest,
) (*analyticspb.RetrieveUserOutcomeStatsResponse, error) {
	return w.analyticsService.RetrieveUserOutcomeStats(ctx, req)
}
