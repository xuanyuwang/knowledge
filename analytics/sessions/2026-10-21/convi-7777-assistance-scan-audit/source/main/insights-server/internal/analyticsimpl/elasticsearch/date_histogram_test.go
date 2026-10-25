package elasticsearch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type BuildDateHistogramTestSuite struct {
	suite.Suite
}

func TestBuildDateHistogram(t *testing.T) {
	suite.Run(t, new(BuildDateHistogramTestSuite))
}

func (s *BuildDateHistogramTestSuite) TestBuildDateHistogram() {
	weeklyOffset := "-2d"

	s.Run("calendar interval includes calendar_interval and offset", func() {
		// Given
		spec := IntervalSpec{Calendar: "week"}

		// When
		got := buildDateHistogram("UTC", spec, &weeklyOffset)

		// Then
		s.Equal("conversation_started_at", got.Field)
		s.Equal("UTC", got.TimeZone)
		s.Equal("week", got.CalendarInterval)
		s.Equal("", got.FixedInterval)
		s.Equal(&weeklyOffset, got.Offset)
	})

	s.Run("fixed interval omits calendar_interval and offset", func() {
		// Given: a fixed interval should serialize as `fixed_interval` only;
		// the weekly offset is meaningless for fixed buckets and must be dropped.
		spec := IntervalSpec{Fixed: "20m", Duration: 20 * time.Minute}

		// When
		got := buildDateHistogram("+05:30", spec, &weeklyOffset)

		// Then
		s.Equal("+05:30", got.TimeZone)
		s.Equal("20m", got.FixedInterval)
		s.Equal("", got.CalendarInterval)
		s.Nil(got.Offset)
	})

	s.Run("calendar interval without offset leaves offset nil", func() {
		// Given
		spec := IntervalSpec{Calendar: "day"}

		// When
		got := buildDateHistogram("UTC", spec, nil)

		// Then
		s.Equal("day", got.CalendarInterval)
		s.Nil(got.Offset)
		s.Equal("", got.FixedInterval)
	})
}
