package shared

import (
	"math"
	"testing"

	"google.golang.org/grpc/status"
)

const tolerance = .001

func TestComputeTConfidenceInterval(t *testing.T) {
	var tc = []struct {
		count                  int32
		mean                   float32
		std                    float32
		confidenceIntervalLow  float32
		confidenceIntervalHigh float32
	}{
		{
			count:                  2,
			mean:                   15.,
			std:                    10.,
			confidenceIntervalLow:  -112.06,
			confidenceIntervalHigh: 142.06,
		},
		{
			count:                  5,
			mean:                   15.,
			std:                    10.,
			confidenceIntervalLow:  1.12,
			confidenceIntervalHigh: 28.88,
		},
		{
			count:                  120,
			mean:                   15.,
			std:                    10.,
			confidenceIntervalLow:  13.184,
			confidenceIntervalHigh: 16.816,
		},
		{
			count:                  4000,
			mean:                   15.,
			std:                    10.,
			confidenceIntervalLow:  14.690,
			confidenceIntervalHigh: 15.310,
		},
	}
	for _, tt := range tc {
		low, high, err := ComputeTConfidenceInterval(tt.count, tt.mean, tt.std)
		if err != nil {
			t.Fatalf("Got code %v in error: %v", err, status.Code(err))
		}
		if math.Abs(float64(tt.confidenceIntervalLow-low)) > tolerance {
			t.Fatalf("got %v, wanted %v", low, tt.confidenceIntervalLow)
		}
		if math.Abs(float64(tt.confidenceIntervalHigh-high)) > tolerance {
			t.Fatalf("got %v, wanted %v", high, tt.confidenceIntervalHigh)
		}
	}
}
