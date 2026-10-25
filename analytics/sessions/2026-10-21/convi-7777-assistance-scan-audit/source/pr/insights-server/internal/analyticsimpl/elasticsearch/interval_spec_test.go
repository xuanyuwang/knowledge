package elasticsearch

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IntervalSpecTestSuite struct {
	suite.Suite
}

func TestIntervalSpec(t *testing.T) {
	suite.Run(t, new(IntervalSpecTestSuite))
}

func (s *IntervalSpecTestSuite) TestParseInterval() {
	dailyFreq := analyticspb.Frequency_DAILY
	weeklyFreq := analyticspb.Frequency_WEEKLY
	monthlyFreq := analyticspb.Frequency_MONTHLY
	unspecifiedFreq := analyticspb.Frequency_FREQUENCY_UNSPECIFIED

	s.Run("daily frequency returns calendar day", func() {
		// Given
		// no custom interval; only frequency provided

		// When
		got, err := ParseInterval(&dailyFreq, nil)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Calendar: "day"}, got)
		s.False(got.IsCalendarWeek())
	})

	s.Run("weekly frequency returns calendar week", func() {
		// Given/When
		got, err := ParseInterval(&weeklyFreq, nil)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Calendar: "week"}, got)
		s.True(got.IsCalendarWeek())
	})

	s.Run("monthly frequency returns calendar month", func() {
		// Given/When
		got, err := ParseInterval(&monthlyFreq, nil)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Calendar: "month"}, got)
	})

	s.Run("custom 20m interval returns fixed", func() {
		// Given
		custom := durationpb.New(20 * time.Minute)

		// When
		got, err := ParseInterval(&unspecifiedFreq, custom)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Fixed: "20m", Duration: 20 * time.Minute}, got)
		s.False(got.IsCalendarWeek())
	})

	s.Run("custom 60m interval is formatted as 1h", func() {
		// Given
		custom := durationpb.New(60 * time.Minute)

		// When
		got, err := ParseInterval(&unspecifiedFreq, custom)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Fixed: "1h", Duration: 60 * time.Minute}, got)
	})

	s.Run("custom 120m interval is formatted as 2h", func() {
		// Given
		custom := durationpb.New(120 * time.Minute)

		// When
		got, err := ParseInterval(&unspecifiedFreq, custom)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Fixed: "2h", Duration: 120 * time.Minute}, got)
	})

	s.Run("nil frequency with nil custom is rejected", func() {
		// Given/When
		_, err := ParseInterval(&unspecifiedFreq, nil)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("both frequency and custom interval is rejected", func() {
		// Given
		custom := durationpb.New(20 * time.Minute)

		// When
		_, err := ParseInterval(&dailyFreq, custom)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("zero-duration custom interval falls back to frequency", func() {
		// Given: a zero duration is treated as "unset", so we fall back to frequency.
		zero := durationpb.New(0)

		// When
		got, err := ParseInterval(&dailyFreq, zero)

		// Then
		s.Require().NoError(err)
		s.Equal(IntervalSpec{Calendar: "day"}, got)
	})

	s.Run("sub-minute custom interval is rejected", func() {
		// Given: tighter bounds live at the proto-validation layer; the Go layer
		// is just a defensive guard against nonsensical fixed_interval values.
		custom := durationpb.New(30 * time.Second)

		// When
		_, err := ParseInterval(&unspecifiedFreq, custom)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("custom interval larger than a day is rejected", func() {
		// Given
		custom := durationpb.New(48 * time.Hour)

		// When
		_, err := ParseInterval(&unspecifiedFreq, custom)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("negative custom interval is rejected", func() {
		// Given
		custom := durationpb.New(-1 * time.Minute)

		// When
		_, err := ParseInterval(&unspecifiedFreq, custom)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})
}

func (s *IntervalSpecTestSuite) TestValidateBucketCount() {
	fixed20m := IntervalSpec{Fixed: "20m", Duration: 20 * time.Minute}
	calendarDay := IntervalSpec{Calendar: "day"}

	s.Run("calendar interval is always valid", func() {
		// Given: a huge time range that would blow past the cap if it were fixed.
		timeRange := &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)),
		}

		// When
		err := calendarDay.ValidateBucketCount(timeRange)

		// Then
		s.Require().NoError(err)
	})

	s.Run("nil time range is allowed", func() {
		// Given/When
		err := fixed20m.ValidateBucketCount(nil)

		// Then
		s.Require().NoError(err)
	})

	s.Run("missing endpoints are allowed", func() {
		// Given
		timeRange := &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Now()),
		}

		// When
		err := fixed20m.ValidateBucketCount(timeRange)

		// Then
		s.Require().NoError(err)
	})

	s.Run("inverted time range is allowed", func() {
		// Given: a non-positive duration short-circuits without error; upstream parsing
		// owns the start<=end invariant.
		timeRange := &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		}

		// When
		err := fixed20m.ValidateBucketCount(timeRange)

		// Then
		s.Require().NoError(err)
	})

	s.Run("range that produces exactly the cap is allowed", func() {
		// Given: 5000 * 20m = 5000 buckets.
		start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		timeRange := &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(start),
			EndTimestamp:   timestamppb.New(start.Add(time.Duration(maxFixedIntervalBucketCount) * 20 * time.Minute)),
		}

		// When
		err := fixed20m.ValidateBucketCount(timeRange)

		// Then
		s.Require().NoError(err)
	})

	s.Run("range that produces one more than the cap is rejected", func() {
		// Given: cap+1 full buckets.
		start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		timeRange := &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(start),
			EndTimestamp:   timestamppb.New(start.Add(time.Duration(maxFixedIntervalBucketCount+1) * 20 * time.Minute)),
		}

		// When
		err := fixed20m.ValidateBucketCount(timeRange)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})

	s.Run("partial trailing bucket counts toward the cap", func() {
		// Given: cap full buckets plus a 1ns sliver should round up to cap+1.
		start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		timeRange := &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(start),
			EndTimestamp:   timestamppb.New(start.Add(time.Duration(maxFixedIntervalBucketCount)*20*time.Minute + time.Nanosecond)),
		}

		// When
		err := fixed20m.ValidateBucketCount(timeRange)

		// Then
		s.Require().Error(err)
		s.Equal(codes.InvalidArgument, status.Code(err))
	})
}

// Sanity check for the package-level constant; if the cap changes, tests above
// will need to be revisited.
func TestMaxFixedIntervalBucketCountIsStable(t *testing.T) {
	require.Equal(t, 5000, maxFixedIntervalBucketCount)
}
