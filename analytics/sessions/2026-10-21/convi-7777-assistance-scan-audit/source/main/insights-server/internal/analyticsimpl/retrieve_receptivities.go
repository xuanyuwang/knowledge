package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (a AnalyticsServiceImpl) RetrieveReceptivities(ctx context.Context, req *analyticspb.RetrieveReceptivitiesRequest) (*analyticspb.RetrieveReceptivitiesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method unimplemented")
}
