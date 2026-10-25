package elasticsearch

import (
	"fmt"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// IntervalSpec describes a time-bucket interval for a date_histogram aggregation.
// Exactly one of Calendar or Fixed is non-empty.
//   - Calendar uses Elasticsearch's `calendar_interval` ("day"/"week"/"month").
//   - Fixed uses Elasticsearch's `fixed_interval` (e.g. "20m", "1h", "1200s").
//
// Duration carries the parsed fixed interval (zero for calendar specs) so callers
// can apply guardrails (e.g. bounding the bucket count) without re-parsing.
type IntervalSpec struct {
	Calendar string
	Fixed    string
	Duration time.Duration
}

// IsCalendarWeek reports whether the interval is the calendar `week` bucketing.
// It is used to decide whether to apply the start-of-week offset, which is only
// meaningful for ES `calendar_interval: week` aggregations.
func (s IntervalSpec) IsCalendarWeek() bool { return s.Calendar == "week" }

// ValidateBucketCount rejects fixed-interval specs whose interval is too small
// for the supplied time range, to protect Elasticsearch from accidentally large
// bucket counts. Calendar specs and missing/empty time ranges are allowed.
func (s IntervalSpec) ValidateBucketCount(timeRange *analyticspb.TimeRange) error {
	if s.Fixed == "" || s.Duration <= 0 {
		return nil
	}
	if timeRange == nil || timeRange.StartTimestamp == nil || timeRange.EndTimestamp == nil {
		return nil
	}
	rangeDuration := timeRange.EndTimestamp.AsTime().Sub(timeRange.StartTimestamp.AsTime())
	if rangeDuration <= 0 {
		return nil
	}
	// Ceiling division so a range that ends mid-bucket still counts that bucket.
	bucketCount := (rangeDuration + s.Duration - 1) / s.Duration
	if bucketCount > maxFixedIntervalBucketCount {
		return status.Errorf(codes.InvalidArgument,
			"`custom_bucket_interval` is too small for the requested time range: would produce more than %d buckets.",
			maxFixedIntervalBucketCount)
	}
	return nil
}

// maxFixedIntervalBucketCount caps how many buckets a single fixed-interval
// query may produce, to protect Elasticsearch load from accidental tiny intervals
// over large time ranges.
const maxFixedIntervalBucketCount = 5000

// ParseInterval converts the request's frequency / custom_bucket_interval pair
// into an IntervalSpec. Exactly one of the two must be set.
func ParseInterval(frequency *analyticspb.Frequency, custom *durationpb.Duration) (IntervalSpec, error) {
	hasFrequency := frequency != nil && *frequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED
	hasCustom := custom != nil && custom.AsDuration() != 0
	if hasFrequency && hasCustom {
		return IntervalSpec{}, status.Error(codes.InvalidArgument,
			"only one of `frequency` or `custom_bucket_interval` may be set.")
	}
	if hasCustom {
		fixed, duration, err := formatFixedInterval(custom.AsDuration())
		if err != nil {
			return IntervalSpec{}, err
		}
		return IntervalSpec{Fixed: fixed, Duration: duration}, nil
	}
	calendar, err := shared.ParseFrequency(frequency)
	if err != nil {
		return IntervalSpec{}, err
	}
	return IntervalSpec{Calendar: calendar}, nil
}

// maxFixedBucketInterval bounds the largest fixed_interval we accept. Beyond a
// day, callers should use a calendar_interval (week/month) so ES can align
// buckets to calendar boundaries. Allowed values are constrained more tightly
// at the proto-validation layer; this is just a defensive lower/upper bound
// so we never emit a nonsensical fixed_interval to Elasticsearch.
// We can remove this check at any time if we need a bucket > 24h that is not daily/weekly
const maxFixedBucketInterval = 24 * time.Hour

func formatFixedInterval(d time.Duration) (string, time.Duration, error) {
	if d <= 0 {
		return "", 0, status.Error(codes.InvalidArgument,
			"`custom_bucket_interval` must be a positive duration.")
	}
	if d < time.Minute {
		return "", 0, status.Error(codes.InvalidArgument,
			"`custom_bucket_interval` must be at least 1 minute.")
	}
	if d%time.Minute != 0 {
		return "", 0, status.Error(codes.InvalidArgument,
			"`custom_bucket_interval` must be a whole number of minutes.")
	}
	if d > maxFixedBucketInterval {
		return "", 0, status.Errorf(codes.InvalidArgument,
			"`custom_bucket_interval` must be no larger than %s; use `frequency` for larger intervals.", maxFixedBucketInterval)
	}
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int64(d/time.Hour)), d, nil
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int64(d/time.Minute)), d, nil
	default:
		return fmt.Sprintf("%ds", int64(d/time.Second)), d, nil
	}
}
