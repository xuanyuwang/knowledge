package shared

import (
	"context"

	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateAnalyticsChartDataConfig(ctx context.Context, dataConfig *dashboardpb.ChartDataConfig) error {
	if dataConfig == nil {
		return status.Errorf(codes.InvalidArgument, "empty data_config")
	}

	if len(dataConfig.Metrics) == 0 {
		return status.Errorf(codes.InvalidArgument, "empty metrics")
	}

	primaryGroupByCount := 0
	for _, groupby := range dataConfig.GroupBy {
		if groupby.Config != nil && groupby.Config.GroupbyPriority == dashboardpb.ChartGroupBy_Config_PRIMARY {
			primaryGroupByCount += 1
		}
	}
	for _, mt := range dataConfig.Metrics {
		if mt.GroupbyConfig != nil && mt.GroupbyConfig.GroupbyPriority == dashboardpb.ChartGroupBy_Config_PRIMARY {
			primaryGroupByCount += 1
		}
	}
	if primaryGroupByCount > 1 {
		return status.Errorf(codes.InvalidArgument, "more than 1 primary groupby is found")
	}
	return nil
}
