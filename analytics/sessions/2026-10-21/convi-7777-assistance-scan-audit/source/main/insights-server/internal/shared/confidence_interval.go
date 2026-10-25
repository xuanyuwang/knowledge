package shared

import (
	"math"
	"sort"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The t Table reference: https://www.sjsu.edu/faculty/gerstman/StatPrimer/t-table.pdf
var dfTTable95 = [...]int32{
	1,
	2,
	3,
	4,
	5,
	6,
	7,
	8,
	9,
	10,
	11,
	12,
	13,
	14,
	15,
	16,
	17,
	18,
	19,
	20,
	21,
	22,
	23,
	24,
	25,
	26,
	27,
	28,
	29,
	30,
	40,
	50,
	60,
	70,
	80,
	90,
	100,
	150,
	200,
	500,
	1000,
	1000000,
}

var tTTable95 = [...]float32{
	12.706,
	4.303,
	3.182,
	2.776,
	2.571,
	2.447,
	2.365,
	2.306,
	2.262,
	2.228,
	2.201,
	2.179,
	2.16,
	2.145,
	2.131,
	2.12,
	2.11,
	2.101,
	2.093,
	2.086,
	2.08,
	2.074,
	2.069,
	2.064,
	2.06,
	2.056,
	2.052,
	2.048,
	2.045,
	2.042,
	2.021,
	2.009,
	2,
	1.994,
	1.99,
	1.987,
	1.984,
	1.976,
	1.972,
	1.965,
	1.962,
	1.96,
}

// Computes the T confidence interval based on the mean and std.
// The result verifications: https://github.com/cresta/chat-ai/blob/master/notebooks/insights/confidence-interval.ipynb
func ComputeTConfidenceInterval(count int32, mean float32, std float32) (float32, float32, error) {
	if count < 2 {
		return 0, 0, status.Error(
			codes.Internal,
			"Need at least two data points for computing confidence interval")
	}
	df := count - 1
	i := sort.Search(len(dfTTable95), func(i int) bool {
		return dfTTable95[i] >= df
	})
	if i == len(dfTTable95) {
		i = len(dfTTable95) - 1
	}
	var t float32
	if dfTTable95[i] == df {
		t = tTTable95[i]
	} else {
		// The linear interpolation for the t value.
		t = tTTable95[i-1] + (tTTable95[i]-tTTable95[i-1])*float32(df-dfTTable95[i-1])/float32(dfTTable95[i]-dfTTable95[i-1])
	}
	stdDDOF1 := math.Sqrt(float64(std * std * float32(count) / float32(count-1)))
	se := float32(stdDDOF1 / math.Sqrt(float64(count)))
	return mean - se*t, mean + se*t, nil
}

func ComputeProportionConfidenceInterval(totalCount int32, positiveCount int32) (float32, float32) {
	totalCount += 4
	positiveCount += 2
	mean := float64(positiveCount) / float64(totalCount)
	error := 1.96 * math.Sqrt(mean*(1-mean)/float64(totalCount))
	return float32(math.Max(mean-error, 0)), float32(math.Min(mean+error, 1))
}
