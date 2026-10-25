package analyticsimpl

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
)

// TestExtractUserIDsFromUsers validates that user IDs are correctly extracted from user protos.
//
// This test directly exercises the fix for INSI-2735 where user filters were ignored when
// ENABLE_DEPRECATE_POSTGRES_USER_ID_QUERY flag is enabled.
//
// Context: The codebase has two code paths for user ID extraction:
//  1. OLD PATH (postgres.GetUserIDsInDB): Parses user proto names → queries DB for active users → returns IDs
//  2. NEW PATH (extractUserIDsFromUsers): Parses user proto names → returns IDs directly
//
// The deprecation flag (ENABLE_DEPRECATE_POSTGRES_USER_ID_QUERY) enables path #2, skipping the DB query
// since upstream code (MoveFiltersToUserFilter + ExcludeDeactivatedUsers) already handles filtering.
//
// This unit test validates that extractUserIDsFromUsers (path #2) correctly parses user proto names.
// Integration tests (TestRetrieveConversationMessages, TestRetrieveConversationMessageStats) validate
// that both paths produce identical ES queries with agent_user_id.keyword filters, proving backward
// compatibility and correctness of the fix.
func TestExtractUserIDsFromUsers(t *testing.T) {
	tests := []struct {
		name    string
		users   []*userpb.User
		wantIDs []string
		wantErr bool
	}{
		{
			name: "single user",
			users: []*userpb.User{
				{Name: "customers/customer_id/users/user-123"},
			},
			wantIDs: []string{"user-123"},
		},
		{
			name: "multiple users",
			users: []*userpb.User{
				{Name: "customers/customer_id/users/user-1"},
				{Name: "customers/customer_id/users/user-2"},
				{Name: "customers/customer_id/users/user-3"},
			},
			wantIDs: []string{"user-1", "user-2", "user-3"},
		},
		{
			name:    "empty users",
			users:   []*userpb.User{},
			wantIDs: nil,
		},
		{
			name:    "nil users",
			users:   nil,
			wantIDs: nil,
		},
		{
			name: "invalid user name",
			users: []*userpb.User{
				{Name: "invalid-format"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIDs, err := extractUserIDsFromUsers(tt.users)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestParseDurationBuckets(t *testing.T) {
	fp := func(v float64) *float64 { return proto.Float64(v) }

	tests := []struct {
		name     string
		selected []*analyticspb.ConversationDurationBucket
		want     []struct {
			gte *float64
			lt  *float64
		}
	}{
		{
			name:     "highest two buckets including open-ended — regression case",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "9"}, {Id: "10"}},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(2400), lt: nil},
			},
		},
		{
			name:     "open-ended bucket only",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "10"}},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(3600), lt: nil},
			},
		},
		{
			name:     "middle range buckets 3 and 4",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "3"}, {Id: "4"}},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(120), lt: fp(240)},
			},
		},
		{
			name:     "single low bucket",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "1"}},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(0), lt: fp(60)},
			},
		},
		{
			name: "all buckets selected",
			selected: func() []*analyticspb.ConversationDurationBucket {
				all := make([]*analyticspb.ConversationDurationBucket, len(hardcodedConversationDurationBuckets))
				for i, bucket := range hardcodedConversationDurationBuckets {
					all[i] = &analyticspb.ConversationDurationBucket{Id: bucket.Id}
				}
				return all
			}(),
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(0), lt: nil},
			},
		},
		{
			name:     "non-consecutive buckets produce separate ranges",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "1"}, {Id: "3"}},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(0), lt: fp(60)},
				{gte: fp(120), lt: fp(180)},
			},
		},
		{
			name:     "duplicate bucket IDs are deduplicated",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "3"}, {Id: "3"}, {Id: "4"}},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(120), lt: fp(240)},
			},
		},
		{
			name:     "unknown bucket IDs return nil",
			selected: []*analyticspb.ConversationDurationBucket{{Id: "99"}, {Id: "100"}},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := parseDurationBuckets(tt.selected, hardcodedConversationDurationBuckets)

			// Then
			if tt.want == nil {
				require.Nil(t, got)
				return
			}
			require.Len(t, got, len(tt.want))
			for i, wantRange := range tt.want {
				require.Equal(t, wantRange.gte, got[i].GTE, "range %d GTE mismatch", i)
				require.Equal(t, wantRange.lt, got[i].LT, "range %d LT mismatch", i)
				require.Nil(t, got[i].GT, "range %d GT should always be nil", i)
				require.Nil(t, got[i].LTE, "range %d LTE should always be nil", i)
			}
		})
	}
}

func TestMergeConsecutiveDurationWindows(t *testing.T) {
	fp := func(v float64) *float64 { return proto.Float64(v) }
	window := func(gte, lt *float64) *elasticsearch.FloatRange {
		return &elasticsearch.FloatRange{GTE: gte, LT: lt}
	}

	tests := []struct {
		name    string
		windows []*elasticsearch.FloatRange
		want    []struct {
			gte *float64
			lt  *float64
		}
	}{
		{
			name: "consecutive windows are merged",
			windows: []*elasticsearch.FloatRange{
				window(fp(120), fp(180)),
				window(fp(180), fp(240)),
			},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(120), lt: fp(240)},
			},
		},
		{
			name: "non-consecutive windows stay separate",
			windows: []*elasticsearch.FloatRange{
				window(fp(0), fp(60)),
				window(fp(120), fp(180)),
			},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(0), lt: fp(60)},
				{gte: fp(120), lt: fp(180)},
			},
		},
		{
			name: "open-ended window absorbs preceding consecutive window",
			windows: []*elasticsearch.FloatRange{
				window(fp(2400), fp(3600)),
				window(fp(3600), nil),
			},
			want: []struct {
				gte *float64
				lt  *float64
			}{
				{gte: fp(2400), lt: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := mergeConsecutiveDurationWindows(tt.windows)

			// Then
			require.Len(t, got, len(tt.want))
			for i, wantRange := range tt.want {
				require.Equal(t, wantRange.gte, got[i].GTE)
				require.Equal(t, wantRange.lt, got[i].LT)
			}
		})
	}
}

// TestParseConversationFilterParams_AIAgentFeedback pins the mapping from the
// analytics `Attribute.ai_agent_feedback` enum onto the internal ES filter
// param. The ES query builder reads `ConversationFilterParams.AIAgentFeedback`
// to decide whether to emit / must_not a nested `comments.type` filter, so any
// silent drop here would mean the Closed Conversations "has AI feedback" toggle
// stops filtering anything.
func TestParseConversationFilterParams_AIAgentFeedback(t *testing.T) {
	// Given
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	tests := []struct {
		name string
		in   analyticspb.Attribute_AIAgentFeedback
	}{
		{name: "unspecified", in: analyticspb.Attribute_AI_AGENT_FEEDBACK_UNSPECIFIED},
		{name: "has ai agent feedback", in: analyticspb.Attribute_HAS_AI_AGENT_FEEDBACK},
		{name: "has no ai agent feedback", in: analyticspb.Attribute_HAS_NO_AI_AGENT_FEEDBACK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := parseConversationFilterParams(
				&analyticspb.Attribute{AiAgentFeedback: tt.in},
				nil, timeRange, "UTC", nil, nil,
			)

			// Then
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tt.in, got.AIAgentFeedback)
		})
	}

	t.Run("defaults to unspecified when attribute omitted", func(t *testing.T) {
		// When
		got, err := parseConversationFilterParams(&analyticspb.Attribute{}, nil, timeRange, "UTC", nil, nil)

		// Then
		require.NoError(t, err)
		require.Equal(t, analyticspb.Attribute_AI_AGENT_FEEDBACK_UNSPECIFIED, got.AIAgentFeedback)
	})
}
