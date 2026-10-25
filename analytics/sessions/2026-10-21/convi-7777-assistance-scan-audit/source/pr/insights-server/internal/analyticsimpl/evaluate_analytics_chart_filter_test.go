package analyticsimpl

import (
	"context"
	"testing"
	"time"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/time"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	"github.com/cresta/shared-go/fn"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/genproto/googleapis/type/dayofweek"
	"google.golang.org/genproto/googleapis/type/timeofday"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

// boundaryBucketClipTestSuite covers the INSI-2281 WEEK/MONTH boundary-bucket clip.
type boundaryBucketClipTestSuite struct {
	suite.Suite
}

func TestBoundaryBucketClip(t *testing.T) {
	suite.Run(t, new(boundaryBucketClipTestSuite))
}

func utcDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// clipCase drives clipOutOfRangeBoundaryBuckets with no time-range rule.
func (s *boundaryBucketClipTestSuite) clipCase(from, to time.Time, bucketStarts ...time.Time) []time.Time {
	return s.clipCaseWithRule(nil, "UTC", from, to, bucketStarts...)
}

// clipCaseWithRule drives clipOutOfRangeBoundaryBuckets from an ascending list of bucket-start
// instants, mirroring what a date_histogram with min_doc_count:0 emits, and returns the starts that
// survive.
func (s *boundaryBucketClipTestSuite) clipCaseWithRule(
	rule *commonpb.TimeRangeRule,
	timezone string,
	from, to time.Time,
	bucketStarts ...time.Time,
) []time.Time {
	buckets := make([]ESRespSingleBucket, 0, len(bucketStarts))
	for _, start := range bucketStarts {
		buckets = append(buckets, ESRespSingleBucket{Key: float64(start.UnixMilli())})
	}
	tr := &timeRange{from: timestamppb.New(from), to: timestamppb.New(to)}

	kept := make([]time.Time, 0, len(bucketStarts))
	for _, b := range clipOutOfRangeBoundaryBuckets(buckets, rule, timezone, tr) {
		kept = append(kept, time.UnixMilli(int64(b.Key.(float64))).UTC())
	}
	return kept
}

func (s *boundaryBucketClipTestSuite) TestClipOutOfRangeBoundaryBuckets() {
	// Range [Mon 2024-07-01, Mon 2024-07-15); `to` is exclusive.
	from := utcDate(2024, 7, 1)
	to := utcDate(2024, 7, 15)

	s.Run("WEEK leading ghost ending exactly at from is dropped", func() {
		// When
		got := s.clipCase(from, to,
			utcDate(2024, 6, 24), // ghost: ends at Jul 1 == from
			utcDate(2024, 7, 1),
			utcDate(2024, 7, 8),
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 7, 1), utcDate(2024, 7, 8)}, got)
	})

	s.Run("WEEK boundary week overlapping the range start is kept", func() {
		// When
		got := s.clipCase(from, to,
			utcDate(2024, 6, 23), // ghost: ends at Jun 30, before from
			utcDate(2024, 6, 30), // overlaps: holds Jul 1..6
			utcDate(2024, 7, 7),
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 6, 30), utcDate(2024, 7, 7)}, got)
	})

	s.Run("WEEK trailing bucket starting at the exclusive to is dropped", func() {
		// When
		got := s.clipCase(from, to,
			utcDate(2024, 7, 8),
			utcDate(2024, 7, 15), // ghost: starts at to
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 7, 8)}, got)
	})

	s.Run("WEEK last bucket overlapping the range end is kept", func() {
		// When
		got := s.clipCase(from, to, utcDate(2024, 7, 8), utcDate(2024, 7, 14))

		// Then
		s.Equal([]time.Time{utcDate(2024, 7, 8), utcDate(2024, 7, 14)}, got)
	})

	s.Run("MONTH trailing ghost equal to the exclusive to is dropped", func() {
		// When
		got := s.clipCase(utcDate(2024, 7, 1), utcDate(2024, 8, 1),
			utcDate(2024, 7, 1),
			utcDate(2024, 8, 1), // ghost
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 7, 1)}, got)
	})

	s.Run("bucket keys offset from UTC midnight are clipped in their own frame", func() {
		// Given a histogram that ran in a fixed +09:00 offset while this code only knows UTC:
		// keys are 15:00Z on the previous UTC day. Only instant comparisons must decide.
		jst := func(year int, month time.Month, day int) time.Time {
			return utcDate(year, month, day).Add(-9 * time.Hour)
		}

		// When
		got := s.clipCase(jst(2024, 12, 1), jst(2025, 3, 1),
			jst(2024, 11, 1), // ghost: ends at Dec 1 == from
			jst(2024, 12, 1),
			jst(2025, 1, 1),
			jst(2025, 2, 1),
			jst(2025, 3, 1), // ghost: starts at to
		)

		// Then
		s.Equal([]time.Time{jst(2024, 12, 1), jst(2025, 1, 1), jst(2025, 2, 1)}, got)
	})

	s.Run("non-numeric bucket keys fail open", func() {
		// Given
		buckets := []ESRespSingleBucket{
			{Key: "2024-07-08"},
			{Key: float64(utcDate(2024, 7, 15).UnixMilli())}, // ghost: starts at to
		}
		tr := &timeRange{from: timestamppb.New(from), to: timestamppb.New(to)}

		// When
		got := clipOutOfRangeBoundaryBuckets(buckets, nil, "UTC", tr)

		// Then
		s.Len(got, 1)
		s.Equal("2024-07-08", got[0].Key)
	})

	s.Run("boundary week whose only in-range day the rule excludes is dropped", func() {
		// Given a Mon/Tue/Wed rule. The trailing week starts Sun Jul 14 and the range ends Jul 15,
		// so its only in-range day is a Sunday: ES can never place a document in it.
		monToWed := &commonpb.TimeRangeRule{DaysOfWeek: []dayofweek.DayOfWeek{
			dayofweek.DayOfWeek_MONDAY, dayofweek.DayOfWeek_TUESDAY, dayofweek.DayOfWeek_WEDNESDAY,
		}}

		// When
		got := s.clipCaseWithRule(monToWed, "UTC", from, to,
			utcDate(2024, 6, 30),
			utcDate(2024, 7, 7),
			utcDate(2024, 7, 14),
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 6, 30), utcDate(2024, 7, 7)}, got)
	})

	s.Run("boundary week whose in-range hours fall outside the rule is dropped", func() {
		// Given a 09:00-17:00 rule and a range ending Sun Jul 14 at 06:00, the trailing week's only
		// in-range hours are Jul 14 00:00-06:00.
		businessHours := &commonpb.TimeRangeRule{TimeOfDayRanges: []*commonpb.TimeOfDayRange{{
			StartTimeOfDay: &timeofday.TimeOfDay{Hours: 9},
			EndTimeOfDay:   &timeofday.TimeOfDay{Hours: 17},
		}}}

		// When
		got := s.clipCaseWithRule(businessHours, "UTC", from, utcDate(2024, 7, 14).Add(6*time.Hour),
			utcDate(2024, 7, 7),
			utcDate(2024, 7, 14),
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 7, 7)}, got)
	})

	s.Run("WEEK trailing bucket whose only in-range docs lie in a partial trailing minute is kept", func() {
		// Given a 10:30-11:00 rule and a range ending at 10:30:45, the trailing week's only in-range
		// instants are 10:30:00-10:30:44 on its last day. ES's truncated_minute_of_day floors those
		// docs to minute 630 and counts them; the clip must ceil the exclusive end to keep the bucket.
		tenThirtyToEleven := &commonpb.TimeRangeRule{TimeOfDayRanges: []*commonpb.TimeOfDayRange{{
			StartTimeOfDay: &timeofday.TimeOfDay{Hours: 10, Minutes: 30},
			EndTimeOfDay:   &timeofday.TimeOfDay{Hours: 11},
		}}}
		midMinuteTo := utcDate(2024, 7, 8).Add(10*time.Hour + 30*time.Minute + 45*time.Second)

		// When
		got := s.clipCaseWithRule(tenThirtyToEleven, "UTC", utcDate(2024, 7, 1), midMinuteTo,
			utcDate(2024, 7, 1),
			utcDate(2024, 7, 8), // trailing week: only in-range slice is 00:00-10:30:45 on Jul 8
		)

		// Then
		s.Equal([]time.Time{utcDate(2024, 7, 1), utcDate(2024, 7, 8)}, got)
	})
}

func (s *boundaryBucketClipTestSuite) TestFilterESResponseBucketsByTimeRule() {
	var noRule *commonpb.TimeRangeRule
	bucket := func(t time.Time) ESRespSingleBucket {
		return ESRespSingleBucket{Key: float64(t.UnixMilli())}
	}
	keysOf := func(buckets []ESRespSingleBucket) []int64 {
		keys := make([]int64, 0, len(buckets))
		for _, b := range buckets {
			keys = append(keys, int64(b.Key.(float64)))
		}
		return keys
	}
	rangeJulyFirstHalf := &timeRange{
		from: timestamppb.New(utcDate(2024, 7, 1)),
		to:   timestamppb.New(utcDate(2024, 7, 15)),
	}

	s.Run("WEEK with no rule drops the leading ghost bucket and keeps in-range weeks", func() {
		// Given
		buckets := []ESRespSingleBucket{
			bucket(utcDate(2024, 6, 23)), // leading ghost: entirely before Jul1
			bucket(utcDate(2024, 6, 30)),
			bucket(utcDate(2024, 7, 7)),
			bucket(utcDate(2024, 7, 14)),
		}

		// When
		got := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_WEEK, noRule, "UTC", rangeJulyFirstHalf)

		// Then
		s.Equal([]int64{
			utcDate(2024, 6, 30).UnixMilli(),
			utcDate(2024, 7, 7).UnixMilli(),
			utcDate(2024, 7, 14).UnixMilli(),
		}, keysOf(got))
	})

	s.Run("MONTH with no rule drops the trailing ghost month", func() {
		// Given
		buckets := []ESRespSingleBucket{
			bucket(utcDate(2024, 7, 1)),
			bucket(utcDate(2024, 8, 1)), // trailing ghost: == exclusive `to`
		}
		rangeJuly := &timeRange{
			from: timestamppb.New(utcDate(2024, 7, 1)),
			to:   timestamppb.New(utcDate(2024, 8, 1)),
		}

		// When
		got := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_MONTH, noRule, "UTC", rangeJuly)

		// Then
		s.Equal([]int64{utcDate(2024, 7, 1).UnixMilli()}, keysOf(got))
	})

	s.Run("DAY with no rule is a no-op", func() {
		// Given
		buckets := []ESRespSingleBucket{
			bucket(utcDate(2024, 6, 23)),
			bucket(utcDate(2024, 6, 30)),
			bucket(utcDate(2024, 7, 7)),
		}

		// When
		got := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_DAY, noRule, "UTC", rangeJulyFirstHalf)

		// Then
		s.Equal([]int64{
			utcDate(2024, 6, 23).UnixMilli(),
			utcDate(2024, 6, 30).UnixMilli(),
			utcDate(2024, 7, 7).UnixMilli(),
		}, keysOf(got))
	})

	s.Run("WEEK without a resolved range keeps every bucket", func() {
		// Given
		buckets := []ESRespSingleBucket{
			bucket(utcDate(2024, 6, 23)),
			bucket(utcDate(2024, 6, 30)),
		}

		// When
		got := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_WEEK, noRule, "UTC", &timeRange{})

		// Then
		s.Equal([]int64{
			utcDate(2024, 6, 23).UnixMilli(),
			utcDate(2024, 6, 30).UnixMilli(),
		}, keysOf(got))
	})

	s.Run("WEEK clip does not depend on the timezone argument", func() {
		// Given a January America/New_York chart whose date_histogram ran in the fixed "-04:00"
		// offset convertGroupByTime resolved from time.Now() during EDT: bucket keys are Monday
		// 04:00Z, an hour off the EST local midnights this code would derive. The clip must reach
		// the same verdict whatever timezone it is handed.
		mondayAt0400Z := func(day int) time.Time {
			return utcDate(2025, 1, day).Add(4 * time.Hour)
		}
		buckets := []ESRespSingleBucket{
			bucket(utcDate(2024, 12, 30).Add(4 * time.Hour)), // leading ghost: ends before `from`
			bucket(mondayAt0400Z(6)),                         // holds Sun Jan 12 00:00 EST
			bucket(mondayAt0400Z(13)),
			bucket(mondayAt0400Z(20)),
		}
		rangeJan12to27 := &timeRange{
			from: timestamppb.New(utcDate(2025, 1, 12).Add(5 * time.Hour)), // Jan 12 00:00 EST
			to:   timestamppb.New(utcDate(2025, 1, 27).Add(5 * time.Hour)),
		}
		want := []int64{
			mondayAt0400Z(6).UnixMilli(),
			mondayAt0400Z(13).UnixMilli(),
			mondayAt0400Z(20).UnixMilli(),
		}

		// When
		gotUTC := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_WEEK, noRule, "UTC", rangeJan12to27)
		gotNY := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_WEEK, noRule, "America/New_York", rangeJan12to27)

		// Then
		s.Equal(want, keysOf(gotUTC))
		s.Equal(want, keysOf(gotNY))
	})

	s.Run("WEEK with a day-of-week rule never drops a bucket the range overlaps", func() {
		// Given the same frame skew plus a Sunday-only rule. Weekday selection is enforced by the
		// ES truncated_day_of_week runtime mapping; re-deriving it here from a bucket key in the
		// wrong frame used to drop the week that holds the selected in-range Sunday (INSI-2281).
		sundayOnly := &commonpb.TimeRangeRule{DaysOfWeek: []dayofweek.DayOfWeek{dayofweek.DayOfWeek_SUNDAY}}
		buckets := []ESRespSingleBucket{
			bucket(utcDate(2025, 1, 6).Add(4 * time.Hour)), // holds Sun Jan 12, the first in-range day
			bucket(utcDate(2025, 1, 13).Add(4 * time.Hour)),
		}
		rangeFromSunJan12 := &timeRange{
			from: timestamppb.New(utcDate(2025, 1, 12).Add(5 * time.Hour)),
			to:   timestamppb.New(utcDate(2025, 1, 27).Add(5 * time.Hour)),
		}

		// When
		got := filterESResponseBucketsByTimeRule(buckets, dashboardpb.ChartGroupBy_Time_WEEK, sundayOnly, "UTC", rangeFromSunJan12)

		// Then
		s.Equal([]int64{
			utcDate(2025, 1, 6).Add(4 * time.Hour).UnixMilli(),
			utcDate(2025, 1, 13).Add(4 * time.Hour).UnixMilli(),
		}, keysOf(got))
	})
}

// scorecardCriterionFilterTestSuite covers the INSI-4445 template scoping: a criterion id is only
// unique within its scorecard template, so the criterion terms must be constrained by the template
// term inside the *same* `scorecards` nested query.
type scorecardCriterionFilterTestSuite struct {
	suite.Suite
}

func TestScorecardCriterionFilter(t *testing.T) {
	suite.Run(t, new(scorecardCriterionFilterTestSuite))
}

const (
	criterionFilterTemplateA = "customers/customer_id/profiles/profile_id/scorecardTemplates/template_a@revision_id"
	criterionFilterTemplateB = "customers/customer_id/profiles/profile_id/scorecardTemplates/template_b@revision_id"
)

// convoFilter drives filterFromScorecardCriterions for the conversation document type.
func (s *scorecardCriterionFilterTestSuite) convoFilter(criterions ...*dashboardpb.ScorecardCriterionForChart) elasticsearch.ConvoFilter {
	f, err := filterFromScorecardCriterions(context.Background(), criterions, elasticsearch.DocumentTypeConversation)
	s.Require().NoError(err)
	convoFilter, ok := f.(elasticsearch.ConvoFilter)
	s.Require().True(ok, "expected a ConvoFilter, got %T", f)
	return convoFilter
}

// assertTemplateScopedNested asserts that nested is a `scorecards` nested query whose bool filter
// holds the template term alongside the `scorecard_scores` nested criterion clause, and returns the
// criterion ids it matches on.
func (s *scorecardCriterionFilterTestSuite) assertTemplateScopedNested(nested *elasticsearch.Nested, wantTemplateID string) []string {
	s.Require().NotNil(nested)
	s.Equal(elasticsearch.ScorecardsPath, nested.Path)
	s.Require().NotNil(nested.Query)
	s.Require().NotNil(nested.Query.Bool)
	s.Require().NotNil(nested.Query.Bool.Filter)

	clauses := *nested.Query.Bool.Filter
	var gotTemplateIDs []string
	var criterionIDs []string
	for _, clause := range clauses {
		if clause.Terms != nil && clause.Terms.ScorecardTemplateIDs != nil {
			gotTemplateIDs = append(gotTemplateIDs, *clause.Terms.ScorecardTemplateIDs...)
		}
		if clause.Nested == nil {
			continue
		}
		// The criterion terms sit one level deeper, on the scorecard_scores nested path.
		s.Equal(elasticsearch.ScorecardScoresPath, clause.Nested.Path)
		s.Require().NotNil(clause.Nested.Query)
		if clause.Nested.Query.Terms != nil && clause.Nested.Query.Terms.ScorecardScoresCriterionIDs != nil {
			criterionIDs = append(criterionIDs, *clause.Nested.Query.Terms.ScorecardScoresCriterionIDs...)
		}
		if clause.Nested.Query.Bool != nil && clause.Nested.Query.Bool.Filter != nil {
			for _, inner := range *clause.Nested.Query.Bool.Filter {
				if inner.Terms != nil && inner.Terms.ScorecardScoresCriterionIDs != nil {
					criterionIDs = append(criterionIDs, *inner.Terms.ScorecardScoresCriterionIDs...)
				}
			}
		}
	}

	// The template term and the criterion clause must be siblings in one nested query: split across
	// two `scorecards` nested queries they could be satisfied by two different scorecards, which is
	// the cross-template over-count this guards against.
	s.Equal([]string{wantTemplateID}, gotTemplateIDs, "template term must live in the same scorecards nested query as the criterion clause")
	return criterionIDs
}

func (s *scorecardCriterionFilterTestSuite) TestSingleCriterionIsScopedToItsTemplate() {
	convoFilter := s.convoFilter(&dashboardpb.ScorecardCriterionForChart{
		ScorecardTemplateName: criterionFilterTemplateA,
		CriterionId:           "verifies-name",
	})

	s.Nil(convoFilter.Bool, "a single template needs no should-clause")
	criterionIDs := s.assertTemplateScopedNested(convoFilter.Nested, "template_a")
	s.Equal([]string{"verifies-name"}, criterionIDs)
}

func (s *scorecardCriterionFilterTestSuite) TestCriterionWithScoresIsScopedToItsTemplate() {
	convoFilter := s.convoFilter(&dashboardpb.ScorecardCriterionForChart{
		ScorecardTemplateName: criterionFilterTemplateA,
		CriterionId:           "verifies-name",
		Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
			{Value: &dashboardpb.ScorecardCriterionForChart_Score_NumericValue{NumericValue: 0}},
		},
	})

	criterionIDs := s.assertTemplateScopedNested(convoFilter.Nested, "template_a")
	s.Equal([]string{"verifies-name"}, criterionIDs)
}

// A criterion id shared by two templates is two distinct criteria; each must carry its own template
// term rather than collapsing into one unscoped terms filter.
func (s *scorecardCriterionFilterTestSuite) TestSameCriterionIDOnTwoTemplatesStaysSeparate() {
	convoFilter := s.convoFilter(
		&dashboardpb.ScorecardCriterionForChart{
			ScorecardTemplateName: criterionFilterTemplateA,
			CriterionId:           "verifies-name",
		},
		&dashboardpb.ScorecardCriterionForChart{
			ScorecardTemplateName: criterionFilterTemplateB,
			CriterionId:           "verifies-name",
		},
	)

	s.Nil(convoFilter.Nested, "several templates are OR-ed under a bool should")
	s.Require().NotNil(convoFilter.Bool)
	s.Require().NotNil(convoFilter.Bool.Should)
	s.EqualValues(1, convoFilter.Bool.MinimumShouldMatch)

	shoulds := *convoFilter.Bool.Should
	s.Require().Len(shoulds, 2)
	s.Equal([]string{"verifies-name"}, s.assertTemplateScopedNested(shoulds[0].Nested, "template_a"))
	s.Equal([]string{"verifies-name"}, s.assertTemplateScopedNested(shoulds[1].Nested, "template_b"))
}

func (s *scorecardCriterionFilterTestSuite) TestCriterionsOfOneTemplateShareOneNestedQuery() {
	convoFilter := s.convoFilter(
		&dashboardpb.ScorecardCriterionForChart{
			ScorecardTemplateName: criterionFilterTemplateA,
			CriterionId:           "verifies-name",
		},
		&dashboardpb.ScorecardCriterionForChart{
			ScorecardTemplateName: criterionFilterTemplateA,
			CriterionId:           "verifies-dob",
		},
		// Repeated selections must not duplicate terms, so the ES request stays deterministic.
		&dashboardpb.ScorecardCriterionForChart{
			ScorecardTemplateName: criterionFilterTemplateA,
			CriterionId:           "verifies-name",
		},
	)

	criterionIDs := s.assertTemplateScopedNested(convoFilter.Nested, "template_a")
	s.Equal([]string{"verifies-name", "verifies-dob"}, criterionIDs)
}

// Scores cannot be attributed to a specific criterion when a template contributes several, so that
// combination stays rejected.
func (s *scorecardCriterionFilterTestSuite) TestScoresWithMultipleCriterionsOfOneTemplateIsRejected() {
	_, err := filterFromScorecardCriterions(context.Background(), []*dashboardpb.ScorecardCriterionForChart{
		{
			ScorecardTemplateName: criterionFilterTemplateA,
			CriterionId:           "verifies-name",
			Scores: []*dashboardpb.ScorecardCriterionForChart_Score{
				{Value: &dashboardpb.ScorecardCriterionForChart_Score_NotApplicable{NotApplicable: true}},
			},
		},
		{
			ScorecardTemplateName: criterionFilterTemplateA,
			CriterionId:           "verifies-dob",
		},
	}, elasticsearch.DocumentTypeConversation)

	s.Require().Error(err)
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *scorecardCriterionFilterTestSuite) TestEmptyCriterionListIsRejected() {
	_, err := filterFromScorecardCriterions(context.Background(), nil, elasticsearch.DocumentTypeConversation)
	s.Require().Error(err)
	s.Equal(codes.InvalidArgument, status.Code(err))
}

// Criteria saved before they carried a template cannot be scoped; they must keep rendering on the
// criterion term alone instead of erroring out.
func (s *scorecardCriterionFilterTestSuite) TestCriterionWithoutTemplateFallsBackToCriterionIDOnly() {
	convoFilter := s.convoFilter(&dashboardpb.ScorecardCriterionForChart{CriterionId: "verifies-name"})

	s.Require().NotNil(convoFilter.Nested)
	s.Require().NotNil(convoFilter.Nested.Query.Bool.Filter)
	for _, clause := range *convoFilter.Nested.Query.Bool.Filter {
		if clause.Terms != nil {
			s.Nil(clause.Terms.ScorecardTemplateIDs)
		}
	}
}

const (
	scopeTestCustomerID = "customer_id"
	scopeTestProfileID  = "profile_id"
	scopeTestRegionID   = "region-emea"
	scopeTestCorkID     = "team-cork"
	scopeTestDubaiID    = "team-dubai"
)

// Cork agents: agent-1, agent-2. Dubai agent: agent-3. The EMEA region holds all three.
var scopeTestGroupMembers = map[string][]string{
	scopeTestRegionID: {"agent-1", "agent-2", "agent-3"},
	scopeTestCorkID:   {"agent-1", "agent-2"},
	scopeTestDubaiID:  {"agent-3"},
}

type ScopeOuterGroupByTestSuite struct {
	suite.Suite
	ctx                   context.Context
	server                AnalyticsServiceImpl
	mockInternalUser      *internalusermock.MockInternalUserServiceClient
	mockConfigClient      *mockconfig.MockClient
	mockResourceACLHelper *authmocks.MockResourceACLHelper
}

func TestScopeOuterGroupBy(t *testing.T) {
	suite.Run(t, new(ScopeOuterGroupByTestSuite))
}

func (s *ScopeOuterGroupByTestSuite) SetupTest() {
	ctrl := gomock.NewController(s.T())
	s.ctx = context.Background()
	s.mockInternalUser = internalusermock.NewMockInternalUserServiceClient(ctrl)
	s.mockConfigClient = mockconfig.NewMockClient(ctrl)
	s.mockResourceACLHelper = authmocks.NewMockResourceACLHelper(ctrl)
	resourceACLHelperProvider := authmocks.NewMockResourceACLHelperProvider(ctrl)
	resourceACLHelperProvider.EXPECT().Get().AnyTimes().Return(s.mockResourceACLHelper)
	s.server = AnalyticsServiceImpl{
		logger:                            logger.GetLogger(),
		internalUserServiceClientRegistry: clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](s.mockInternalUser),
		configClient:                      s.mockConfigClient,
		resourceACLHelperProvider:         resourceACLHelperProvider,
	}
	s.mockInternalUser.EXPECT().ListUsersForAnalytics(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(_ context.Context, req *internaluserpb.ListUsersForAnalyticsRequest, _ ...grpc.CallOption) (*internaluserpb.ListUsersForAnalyticsResponse, error) {
			resp := &internaluserpb.ListUsersForAnalyticsResponse{}
			for _, groupID := range req.GetGroupIds() {
				for _, userID := range scopeTestGroupMembers[groupID] {
					resp.Users = append(resp.Users, &internaluserpb.LiteUser{UserId: userID})
				}
			}
			return resp, nil
		})
}

func scopeTestGroupName(groupID string) string {
	return userpb.GroupName{CustomerID: scopeTestCustomerID, GroupID: groupID}.String()
}

func scopeTestUserName(userID string) string {
	return userpb.UserName{CustomerID: scopeTestCustomerID, UserID: userID}.String()
}

func groupValue(groupID string) *dashboardpb.ChartAttributeValue {
	return &dashboardpb.ChartAttributeValue{Value: &dashboardpb.ChartAttributeValue_GroupNameValue{GroupNameValue: scopeTestGroupName(groupID)}}
}

func userValue(userID string) *dashboardpb.ChartAttributeValue {
	return &dashboardpb.ChartAttributeValue{Value: &dashboardpb.ChartAttributeValue_UserNameValue{UserNameValue: scopeTestUserName(userID)}}
}

func singleValueSet(values ...*dashboardpb.ChartAttributeValue) *dashboardpb.ChartValueSet {
	return &dashboardpb.ChartValueSet{
		Values: &dashboardpb.ChartValueSet_SingleValues{
			SingleValues: &dashboardpb.ChartValueSet_Single{AttributeValues: values},
		},
	}
}

func inFilter(values ...*dashboardpb.ChartAttributeValue) *dashboardpb.ChartDataFilter {
	return &dashboardpb.ChartDataFilter{Operator: dashboardpb.ChartDataFilter_IN, ValueSet: singleValueSet(values...)}
}

func outerProvidedGroupBy(sortDirection dashboardpb.ChartGroupBy_Config_SortDirection, values ...*dashboardpb.ChartAttributeValue) *dashboardpb.ChartGroupBy {
	return &dashboardpb.ChartGroupBy{
		Config: &dashboardpb.ChartGroupBy_Config{
			SortDirection:   sortDirection,
			ValueCountLimit: 10,
			GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
		},
		GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
			ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{ValueSet: singleValueSet(values...)},
		},
	}
}

func outerValues(chartDataConfig *dashboardpb.ChartDataConfig) []*dashboardpb.ChartAttributeValue {
	return chartDataConfig.GetGroupBy()[0].GetByProvidedValue().GetValueSet().GetSingleValues().GetAttributeValues()
}

func (s *ScopeOuterGroupByTestSuite) TestScopeOuterGroupByToUserFilters() {
	parent := &shared.CustomerProfile{CustomerID: scopeTestCustomerID, ProfileID: scopeTestProfileID}

	s.Run("drops outer teams and users outside the team filter", func() {
		// Given
		chartDataConfig := &dashboardpb.ChartDataConfig{
			Filters: []*dashboardpb.ChartDataFilter{inFilter(groupValue(scopeTestCorkID))},
			GroupBy: []*dashboardpb.ChartGroupBy{outerProvidedGroupBy(dashboardpb.ChartGroupBy_Config_DESC,
				groupValue(scopeTestRegionID), groupValue(scopeTestCorkID), groupValue(scopeTestDubaiID),
				userValue("agent-3"), userValue("agent-1"))},
		}

		// When
		outerGroupMemberIDs, noValuesLeft, err := s.server.scopeOuterGroupByToUserFilters(s.ctx, parent, chartDataConfig)

		// Then
		s.Require().NoError(err)
		s.False(noValuesLeft)
		s.Equal([]string{scopeTestGroupName(scopeTestRegionID), scopeTestGroupName(scopeTestCorkID), scopeTestUserName("agent-1")},
			fn.Map(outerValues(chartDataConfig), func(value *dashboardpb.ChartAttributeValue) string {
				return value.GetGroupNameValue() + value.GetUserNameValue()
			}))
		s.Equal(map[string][]string{
			scopeTestGroupName(scopeTestRegionID): {"agent-1", "agent-2", "agent-3"},
			scopeTestGroupName(scopeTestCorkID):   {"agent-1", "agent-2"},
			scopeTestGroupName(scopeTestDubaiID):  {"agent-3"},
		}, outerGroupMemberIDs)
	})

	s.Run("requires overlap with every ANDed filter", func() {
		// Given
		chartDataConfig := &dashboardpb.ChartDataConfig{
			Filters: []*dashboardpb.ChartDataFilter{
				inFilter(groupValue(scopeTestRegionID)),
				inFilter(userValue("agent-2")),
			},
			GroupBy: []*dashboardpb.ChartGroupBy{outerProvidedGroupBy(dashboardpb.ChartGroupBy_Config_ASC,
				groupValue(scopeTestCorkID), groupValue(scopeTestDubaiID))},
		}

		// When
		_, noValuesLeft, err := s.server.scopeOuterGroupByToUserFilters(s.ctx, parent, chartDataConfig)

		// Then
		s.Require().NoError(err)
		s.False(noValuesLeft)
		s.Equal([]*dashboardpb.ChartAttributeValue{groupValue(scopeTestCorkID)}, outerValues(chartDataConfig))
	})

	s.Run("reports no values left when every outer value is outside the filter", func() {
		// Given
		chartDataConfig := &dashboardpb.ChartDataConfig{
			Filters: []*dashboardpb.ChartDataFilter{inFilter(groupValue(scopeTestDubaiID))},
			GroupBy: []*dashboardpb.ChartGroupBy{outerProvidedGroupBy(dashboardpb.ChartGroupBy_Config_DESC,
				groupValue(scopeTestCorkID), userValue("agent-1"))},
		}

		// When
		_, noValuesLeft, err := s.server.scopeOuterGroupByToUserFilters(s.ctx, parent, chartDataConfig)

		// Then
		s.Require().NoError(err)
		s.True(noValuesLeft)
		s.Empty(outerValues(chartDataConfig))
	})

	s.Run("leaves an unsorted outer groupby unchanged", func() {
		// Given
		chartDataConfig := &dashboardpb.ChartDataConfig{
			Filters: []*dashboardpb.ChartDataFilter{inFilter(groupValue(scopeTestCorkID))},
			GroupBy: []*dashboardpb.ChartGroupBy{outerProvidedGroupBy(dashboardpb.ChartGroupBy_Config_SORT_DIRECTION_UNSPECIFIED,
				groupValue(scopeTestCorkID), groupValue(scopeTestDubaiID))},
		}

		// When
		outerGroupMemberIDs, noValuesLeft, err := s.server.scopeOuterGroupByToUserFilters(s.ctx, parent, chartDataConfig)

		// Then
		s.Require().NoError(err)
		s.False(noValuesLeft)
		s.Nil(outerGroupMemberIDs)
		s.Equal([]*dashboardpb.ChartAttributeValue{groupValue(scopeTestCorkID), groupValue(scopeTestDubaiID)}, outerValues(chartDataConfig))
	})

	s.Run("leaves the outer groupby unchanged without a user or team filter", func() {
		// Given
		chartDataConfig := &dashboardpb.ChartDataConfig{
			Filters: []*dashboardpb.ChartDataFilter{{Operator: dashboardpb.ChartDataFilter_NOT_IN, ValueSet: singleValueSet(groupValue(scopeTestCorkID))}},
			GroupBy: []*dashboardpb.ChartGroupBy{outerProvidedGroupBy(dashboardpb.ChartGroupBy_Config_DESC,
				groupValue(scopeTestCorkID), groupValue(scopeTestDubaiID))},
		}

		// When
		outerGroupMemberIDs, noValuesLeft, err := s.server.scopeOuterGroupByToUserFilters(s.ctx, parent, chartDataConfig)

		// Then
		s.Require().NoError(err)
		s.False(noValuesLeft)
		s.Nil(outerGroupMemberIDs)
		s.Equal([]*dashboardpb.ChartAttributeValue{groupValue(scopeTestCorkID), groupValue(scopeTestDubaiID)}, outerValues(chartDataConfig))
	})
}

func (s *ScopeOuterGroupByTestSuite) TestApplyACLToChartDataConfigWithFilterTeamOutsideACL() {
	// Given
	customerConfig := fn.Must(config.NewCustomerConfig(&nonpubconfig.CustomerConfig{
		Customer:   "customers/" + scopeTestCustomerID,
		AuthConfig: &nonpubconfig.CustomerAuthConfig{EnableAdvancedDataAccessControl: true},
	}))
	s.mockConfigClient.EXPECT().Get(scopeTestCustomerID).AnyTimes().Return(customerConfig, nil)
	s.mockResourceACLHelper.EXPECT().GetResourceACL(gomock.Any(), scopeTestCustomerID, gomock.Any()).AnyTimes().Return(&auth.ResourceACL{
		IsRootAccess: false,
		// WithFilteredGroupIDs intersects the managed groups with the filter groups: Cork is not managed.
		ManagedUserIDs: []string{"agent-3"},
	}, nil)
	chartDataConfig := &dashboardpb.ChartDataConfig{
		Filters: []*dashboardpb.ChartDataFilter{inFilter(groupValue(scopeTestCorkID))},
	}

	// When
	noAccessibleFilterUsers, err := s.server.ApplyACLToChartDataConfig(s.ctx, scopeTestCustomerID, scopeTestProfileID, chartDataConfig)

	// Then
	s.Require().NoError(err)
	s.True(noAccessibleFilterUsers)
	s.Len(chartDataConfig.GetFilters(), 1, "the team filter must not be stripped")
}

// A dashboard team filter must scope a Top-N team groupby to the teams that share members with the
// filter. Otherwise, out-of-filter teams come back with 0 and pad the Top-N (INSI-4358, QA-2822).
func TestEvaluateAnalyticsChart_TeamFilterScopesOuterGroupBy(t *testing.T) {
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	const (
		regionGroupID = "team-region-emea"
		corkGroupID   = "team-cork"
		dubaiGroupID  = "team-dubai"
	)
	// The EMEA region holds the Cork agents (agent-id-1, agent-id-2) and the Dubai agent (agent-id-3).
	groupMemberIDs := map[string][]string{
		regionGroupID: {"agent-id-1", "agent-id-2", "agent-id-3"},
		corkGroupID:   {"agent-id-1", "agent-id-2"},
		dubaiGroupID:  {"agent-id-3"},
	}
	r.mockInternalUserClient.EXPECT().ListUsersForAnalytics(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(_ context.Context, req *internaluserpb.ListUsersForAnalyticsRequest, _ ...grpc.CallOption) (*internaluserpb.ListUsersForAnalyticsResponse, error) {
			resp := &internaluserpb.ListUsersForAnalyticsResponse{}
			for _, groupID := range req.GetGroupIds() {
				for _, userID := range groupMemberIDs[groupID] {
					resp.Users = append(resp.Users, &internaluserpb.LiteUser{UserId: userID})
				}
			}
			return resp, nil
		})
	// gomock matches the earliest registered expectation, so per-case AnyTimes expectations would
	// leak the first case's config into later cases. Each case sets testCustomerConfig instead.
	var testCustomerConfig *config.CustomerConfig
	r.mockConfigClient.EXPECT().GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(string, string) (*config.ProfileConfig, error) { return testCustomerConfig.ListProfiles()[0], nil })
	r.mockConfigClient.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(
		func(string) (*config.CustomerConfig, error) { return testCustomerConfig, nil })

	teamValue := func(groupID string) *dashboardpb.ChartAttributeValue {
		return &dashboardpb.ChartAttributeValue{
			Value: &dashboardpb.ChartAttributeValue_GroupNameValue{
				GroupNameValue: userpb.GroupName{CustomerID: r.customerID, GroupID: groupID}.String(),
			},
		}
	}
	chartRequest := func(filterGroupID string, outerGroupIDs ...string) *analyticspb.EvaluateAnalyticsChartRequest {
		return &analyticspb.EvaluateAnalyticsChartRequest{
			Parent: r.profileResourceName,
			AnalyticsChart: &dashboardpb.AnalyticsChart{
				DisplayName: "display_name",
				DataConfig: &dashboardpb.ChartDataConfig{
					Metrics: []*dashboardpb.ChartMetric{
						{
							Metric: &dashboardpb.ChartMetric_ForConversation_{
								ForConversation: &dashboardpb.ChartMetric_ForConversation{
									StatsType: &dashboardpb.StatsTypeForConversation{
										StatsType: &dashboardpb.StatsTypeForConversation_CommonStatsType{
											CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
										},
									},
								},
							},
						},
					},
					Filters: []*dashboardpb.ChartDataFilter{
						{
							Operator: dashboardpb.ChartDataFilter_IN,
							ValueSet: &dashboardpb.ChartValueSet{
								Values: &dashboardpb.ChartValueSet_RangeValues{
									RangeValues: &dashboardpb.ChartValueSet_Range{
										RangeValues: []*dashboardpb.ChartValueRange{
											{
												FromValue: &dashboardpb.ChartAttributeValue{
													Value: &dashboardpb.ChartAttributeValue_Timestamp{Timestamp: timestamppb.New(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))},
												},
												ToValue: &dashboardpb.ChartAttributeValue{
													Value: &dashboardpb.ChartAttributeValue_Timestamp{Timestamp: timestamppb.New(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC))},
												},
											},
										},
									},
								},
							},
						},
						{
							Operator: dashboardpb.ChartDataFilter_IN,
							ValueSet: &dashboardpb.ChartValueSet{
								Values: &dashboardpb.ChartValueSet_SingleValues{
									SingleValues: &dashboardpb.ChartValueSet_Single{
										AttributeValues: []*dashboardpb.ChartAttributeValue{teamValue(filterGroupID)},
									},
								},
							},
						},
					},
					GroupBy: []*dashboardpb.ChartGroupBy{
						{
							GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
								ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
									ValueSet: &dashboardpb.ChartValueSet{
										Values: &dashboardpb.ChartValueSet_SingleValues{
											SingleValues: &dashboardpb.ChartValueSet_Single{
												AttributeValues: fn.Map(outerGroupIDs, teamValue),
											},
										},
									},
								},
							},
							Config: &dashboardpb.ChartGroupBy_Config{
								SortDirection:   dashboardpb.ChartGroupBy_Config_DESC,
								ValueCountLimit: 10,
								GroupbyLayer:    dashboardpb.ChartGroupBy_Config_GROUPBY_LAYER_OUTER,
							},
						},
					},
				},
				RenderingConfig: &dashboardpb.RenderingConfig{ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART},
				Audience:        &dashboardpb.DashboardAudience{},
				Accessibility:   dashboardpb.DashboardAccessibility_PERSONAL,
			},
		}
	}
	// The primary metric ranks the outer groupby. The secondary runs unsorted, so it is not scoped.
	comboChartRequest := func(filterGroupID string, outerGroupIDs ...string) *analyticspb.EvaluateAnalyticsChartRequest {
		req := chartRequest(filterGroupID, outerGroupIDs...)
		dataConfig := req.GetAnalyticsChart().GetDataConfig()
		dataConfig.Metrics = append(dataConfig.Metrics, proto.Clone(dataConfig.GetMetrics()[0]).(*dashboardpb.ChartMetric))
		return req
	}

	for _, tc := range []struct {
		desc                            string
		req                             *analyticspb.EvaluateAnalyticsChartRequest
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
		// Empty when the chart must return an empty response without querying ES.
		esReqPath  string
		esRespPath string
		resp       *analyticspb.EvaluateAnalyticsChartResponse
	}{
		{
			// Dubai shares no member with the Cork filter, so it gets no bucket. EMEA contains the Cork
			// agents and keeps its bucket.
			desc:       "team_filter_drops_outer_teams_outside_the_filter",
			req:        chartRequest(corkGroupID, regionGroupID, corkGroupID, dubaiGroupID),
			esReqPath:  "EvaluateAnalyticsChart_team_filter_scopes_outer_groupby_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_team_filter_scopes_outer_groupby_es_response.json",
			resp: &analyticspb.EvaluateAnalyticsChartResponse{
				ResultForChart: &dashboardpb.AnalyticsResultForChart{
					AnalyticsResults: []*dashboardpb.AnalyticsResult{
						{
							GroupByValuesForAllKeys: []*dashboardpb.AnalyticsResult_GroupByValuesForSingleKey{
								{
									GroupByValues: []*dashboardpb.AnalyticsResult_GroupByValue{
										{Value: teamValue(regionGroupID)},
										{Value: teamValue(corkGroupID)},
									},
								},
							},
							AnalyticsNumbers: []*dashboardpb.AnalyticsResult_AnalyticsNumber{
								{GroupByValueIndexes: []int32{0}, Value: 12},
								{GroupByValueIndexes: []int32{1}, Value: 7},
							},
						},
					},
				},
			},
		},
		{
			desc: "team_filter_outside_every_outer_team_returns_empty_response",
			req:  chartRequest(dubaiGroupID, corkGroupID),
			resp: &analyticspb.EvaluateAnalyticsChartResponse{},
		},
		{
			// The unscoped secondary still returns the Cork bucket at 0. It must not reach the response.
			desc:       "combo_chart_team_filter_outside_every_outer_team_returns_empty_response",
			req:        comboChartRequest(dubaiGroupID, corkGroupID),
			esReqPath:  "EvaluateAnalyticsChart_team_filter_combo_non_ranking_metric_es_request.json",
			esRespPath: "EvaluateAnalyticsChart_team_filter_combo_non_ranking_metric_es_response.json",
			resp:       &analyticspb.EvaluateAnalyticsChartResponse{},
		},
		{
			// The ACL intersects the managed groups with the filter groups: Cork is not managed.
			desc:                            "team_filter_outside_acl_returns_empty_response",
			req:                             chartRequest(corkGroupID, regionGroupID, corkGroupID, dubaiGroupID),
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:   false,
				ManagedUserIDs: []string{"agent-id-3"},
			},
			resp: &analyticspb.EvaluateAnalyticsChartResponse{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Given
			testCustomerConfig = fn.Must(config.NewCustomerConfig(&nonpubconfig.CustomerConfig{
				Customer:       "customers/customer_id",
				ProfileConfigs: []*nonpubconfig.ProfileConfig{{Name: "customers/customer_id/profiles/profile_id"}},
				AuthConfig:     &nonpubconfig.CustomerAuthConfig{EnableAdvancedDataAccessControl: tc.enableAdvancedDataAccessControl},
			}))
			if tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}
			if tc.esReqPath != "" {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
				r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			}

			// When
			resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, tc.req)

			// Then
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.resp, resp, protocmp.Transform(),
				protocmp.IgnoreFields(&analyticspb.EvaluateAnalyticsChartResponse{}, "update_timestamp")))
		})
	}
}
