package testing

import (
	"context"

	trendandanomalypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/trend_and_anomaly"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type FakeTrendAndAnomalyServer struct {
}

func (f *FakeTrendAndAnomalyServer) ComputeForecast(ctx context.Context, req *trendandanomalypb.ComputeForecastRequest, opts ...grpc.CallOption) (*trendandanomalypb.ComputeForecastResponse, error) {
	resp := &trendandanomalypb.ComputeForecastResponse{
		ForecastTimeSeries: &trendandanomalypb.TimeSeries{},
	}

	for _, dataPoint := range req.TimeSeries.DataPoints {
		resp.ForecastTimeSeries.DataPoints = append(resp.ForecastTimeSeries.DataPoints, &trendandanomalypb.TimeSeriesDataPoint{
			Time:                    dataPoint.Time,
			Value:                   dataPoint.Value,
			ForecastedValue:         structpb.NewNumberValue(150),
			ConfidenceIntervalLower: structpb.NewNumberValue(100),
			ConfidenceIntervalUpper: structpb.NewNumberValue(200),
		})
	}

	return resp, nil
}
