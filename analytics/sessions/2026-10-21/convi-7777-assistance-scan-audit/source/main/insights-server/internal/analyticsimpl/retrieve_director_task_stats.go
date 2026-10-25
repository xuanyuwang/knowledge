package analyticsimpl

import (
	"context"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

func (a AnalyticsServiceImpl) RetrieveDirectorTaskStats(
	ctx context.Context,
	req *analytics.RetrieveDirectorTaskStatsRequest,
) (*analytics.RetrieveDirectorTaskStatsResponse, error) {
	profile, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", req.Parent)
	}
	userClient, err := a.userClientRegistry.CreateClient(profile.CustomerID, profile.ProfileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user client: %v", err)
	}
	if req.DirectorTaskType == int32(coaching.DirectorTaskType_DIRECTOR_TASK_TYPE_GROUP_CALIBRATION) {
		return retrieveGroupCalibrationStats(ctx, a.appsDB.DB(ctx), profile, userClient, req, a.authEntityProvider)
	}
	if req.DirectorTaskType == int32(coaching.DirectorTaskType_DIRECTOR_TASK_TYPE_QM) {
		return a.RetrieveQMTaskStats(ctx, req)
	}
	return &analytics.RetrieveDirectorTaskStatsResponse{
		DirectorTaskStats:      []*analytics.DirectorTaskStats{},
		TotalDirectorTaskStats: &analytics.DirectorTaskStats{},
	}, nil
}
