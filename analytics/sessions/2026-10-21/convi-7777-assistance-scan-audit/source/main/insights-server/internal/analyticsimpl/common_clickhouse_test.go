package analyticsimpl

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConversationOutcomeMVEnabled(t *testing.T) {
	t.Run("enabled for start-time filters when flag is on", func(t *testing.T) {
		// When
		got := conversationOutcomeMVEnabled(true, conversationStartTimeColumn)

		// Then
		if !got {
			t.Fatal("expected CLO MV to be enabled for start-time filters when the flag is on")
		}
	})
	t.Run("disabled for end-time filters even when flag is on", func(t *testing.T) {
		// When
		got := conversationOutcomeMVEnabled(true, conversationEndTimeColumn)

		// Then
		if got {
			t.Fatal("expected CLO MV to be disabled for end-time filters because storage lacks conversation_end_time")
		}
	})
	t.Run("disabled when flag is off", func(t *testing.T) {
		// When
		got := conversationOutcomeMVEnabled(false, conversationStartTimeColumn)

		// Then
		if got {
			t.Fatal("expected CLO MV to be disabled when the flag is off")
		}
	})
}

func TestMomentAnnotationMVSupportsTimeColumn(t *testing.T) {
	t.Run("supports start-time", func(t *testing.T) {
		// When
		got := momentAnnotationMVSupportsTimeColumn(conversationStartTimeColumn)

		// Then
		if !got {
			t.Fatal("expected metadata/CLO MV to support conversation_start_time")
		}
	})
	t.Run("does not support end-time", func(t *testing.T) {
		// When
		got := momentAnnotationMVSupportsTimeColumn(conversationEndTimeColumn)

		// Then
		if got {
			t.Fatal("expected metadata/CLO MV to reject conversation_end_time")
		}
	})
}

func TestParseMomentConditionsForQAAttribute_MetadataFallsBackToRawForEndTime(t *testing.T) {
	// Given
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/test_customer/profiles/test_profile/moments/moment-id1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{
						MetadataValue: &commonpb.MetadataValue{
							Value: &commonpb.MetadataValue_StringValue{StringValue: "metadata-value-1"},
						},
					},
				},
			},
		},
	}

	// When
	gotMomentFilters, _, _, err := parseMomentConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		true, // CLO MV flag on — should not affect metadata end-time fallback
		conversationEndTimeColumn,
	)

	// Then
	if err != nil {
		t.Fatalf("parseMomentConditionsForQAAttribute() failed: %v", err)
	}
	if len(gotMomentFilters) != 1 {
		t.Fatalf("got %d moment filters, want 1", len(gotMomentFilters))
	}
	if gotMomentFilters[0].source != momentAnnotationFilterSourceRaw {
		t.Fatalf("expected raw source for metadata end-time filter, got %v", gotMomentFilters[0].source)
	}
	conditions, _ := concatConditionsAndArgs(gotMomentFilters[0].conditions)
	if !strings.Contains(conditions, conversationEndTimeColumn) {
		t.Fatalf("expected end-time predicate in conditions, got %s", conditions)
	}
	if !strings.Contains(conditions, "moment_type") {
		t.Fatalf("expected moment_type predicate on raw path, got %s", conditions)
	}
}

func TestParseClickhouseFilter(t *testing.T) {
	gotConditionsAndArgs, _, _, _, err := parseClickhouseFilter(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		}, &analyticspb.Attribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
			AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
				{
					Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
				},
				{
					Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
				},
			},
			ConversationDurationRange: &analyticspb.DurationRange{
				MinDuration: &durationpb.Duration{
					Seconds: 10,
				},
				MaxDuration: &durationpb.Duration{
					Seconds: 100,
				},
			},
		},
		[]ClickhouseTable{momentAnnotationTable}, false)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs[momentAnnotationTable])
	wantConditions := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(agent_user_id IN (?)) AND " +
		"(policy_id IN (?)) AND " +
		"(conversation_duration_secs >= ?) AND " +
		"(conversation_duration_secs < ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1", "agent-id2"},
		[]string{"policy-id1", "policy-id2"},
		10.0,
		100.0,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilter_withMomentFilters(t *testing.T) {
	gotConditionsAndArgs, gotMomentConditionsAndArgs, gotExcludedMomentConditionsAndArgs, _, err := parseClickhouseFilter(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		}, &analyticspb.Attribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
			AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
				{
					Name:        "customers/test_customer/profiles/test_profile/policies/policy-id1",
					DisplayName: "policy-id1",
				},
				{
					Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
				},
			},
			ConversationDurationRange: &analyticspb.DurationRange{
				MinDuration: &durationpb.Duration{
					Seconds: 10,
				},
				MaxDuration: &durationpb.Duration{
					Seconds: 100,
				},
			},
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{
							Name: "customers/test_customer/profiles/test_profile/moments/moment-id1",
							Type: momentpb.Moment_CONVERSATION_METADATA,
						},
					},
					MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
						{
							MetadataValue: &commonpb.MetadataValue{
								Value: &commonpb.MetadataValue_StringValue{
									StringValue: "metadata-value-1",
								},
							},
						},
						{
							MetadataValue: &commonpb.MetadataValue{
								Value: &commonpb.MetadataValue_StringValue{
									StringValue: "metadata-value-2",
								},
							},
						},
					},
				},
				{
					Moments: []*momentpb.Moment{
						{
							Name: "customers/test_customer/profiles/test_profile/moments/moment-id2",
							Type: momentpb.Moment_CONVERSATION_METADATA,
						},
					},
					MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
						{
							NumericBin: &commonpb.NumericBin{
								FromValue: 0,
								ToValue:   100,
							},
						},
						{
							NumericBin: &commonpb.NumericBin{
								FromValue: 100,
								ToValue:   250,
							},
						},
					},
				},
				{
					ExcludedMoments: []*momentpb.Moment{
						{
							Name: "customers/test_customer/profiles/test_profile/moments/moment-id3",
							Type: momentpb.Moment_CONVERSATION_METADATA,
						},
					},
				},
			},
		},
		[]ClickhouseTable{momentAnnotationTable}, false)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs[momentAnnotationTable])
	wantConditions := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(agent_user_id IN (?)) AND " +
		"(policy_id IN (?)) AND " +
		"(conversation_duration_secs >= ?) AND " +
		"(conversation_duration_secs < ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1", "agent-id2"},
		[]string{"policy-id1", "policy-id2"},
		10.0,
		100.0,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}

	if len(gotMomentConditionsAndArgs) != 2 {
		t.Fatalf("gotMomentConditionsAndArgs should have 2 slices but not %d", len(gotMomentConditionsAndArgs))
	}
	gotMomentConditionsForString, gotMomentArgsForString := concatConditionsAndArgs(gotMomentConditionsAndArgs[0])
	wantMomentConditionsForString := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_type = ?) AND " +
		"(moment_template_id = ?) AND " +
		"(metadata_string_value = ? OR metadata_string_value = ?))"
	if diff := cmp.Diff(wantMomentConditionsForString, gotMomentConditionsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong moment conditions for string metadata value with diff (-want +got): %v", diff)
	}
	wantMomentArgsForString := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		int32(19),
		"moment-id1",
		"metadata-value-1", "metadata-value-2",
	}
	if diff := cmp.Diff(wantMomentArgsForString, gotMomentArgsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments for string metadata value with diff (-want +got): %v", diff)
	}
	gotMomentConditionsForNumberBin, gotMomentArgsForNumberBin := concatConditionsAndArgs(gotMomentConditionsAndArgs[1])
	wantMomentConditionsForNumberBin := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_type = ?) AND " +
		"(moment_template_id = ?) AND " +
		"((metadata_number_value >= ? AND metadata_number_value < ?) OR " +
		"(metadata_number_value >= ? AND metadata_number_value < ?)))"
	if diff := cmp.Diff(wantMomentConditionsForNumberBin, gotMomentConditionsForNumberBin); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong moment conditions for number metadata value with diff (-want +got): %v", diff)
	}
	wantMomentArgsForNumberBin := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		int32(19),
		"moment-id2",
		float64(0.0), float64(100.0),
		float64(100.0), float64(250.0),
	}
	if diff := cmp.Diff(wantMomentArgsForNumberBin, gotMomentArgsForNumberBin); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments for number metadata value with diff (-want +got): %v", diff)
	}

	if len(gotExcludedMomentConditionsAndArgs) != 1 {
		t.Fatalf("gotExcludedMomentConditionsAndArgs should have 1 slice but not %d", len(gotExcludedMomentConditionsAndArgs))
	}
	gotExcludedMomentConditionsForString, gotExcludedMomentArgsForString := concatConditionsAndArgs(gotExcludedMomentConditionsAndArgs[0])
	wantExcludedMomentConditionsForString := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_type = ?) AND " +
		"(moment_template_id = ?))"
	if diff := cmp.Diff(wantExcludedMomentConditionsForString, gotExcludedMomentConditionsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong excluded moment conditions for string metadata value with diff (-want +got): %v", diff)
	}
	wantExcludedMomentArgsForString := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		int32(19),
		"moment-id3",
	}
	if diff := cmp.Diff(wantExcludedMomentArgsForString, gotExcludedMomentArgsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments for exluded moments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilter_RejectsValueOrMissingMetadataFilter(t *testing.T) {
	originalEnabled := *enableExcludedMoments
	t.Cleanup(func() { *enableExcludedMoments = originalEnabled })
	moment := func(id string) *momentpb.Moment {
		return &momentpb.Moment{
			Name: "customers/test_customer/profiles/test_profile/moments/" + id,
			Type: momentpb.Moment_CONVERSATION_METADATA,
		}
	}
	a, b := moment("A"), moment("B")
	untypedA := &momentpb.Moment{Name: a.Name}
	invalid := &momentpb.Moment{Name: "invalid", Type: momentpb.Moment_CONVERSATION_METADATA}
	cases := []struct {
		name                       string
		included, excluded         []*momentpb.Moment
		disabled                   bool
		wantError                  string
		wantIncludes, wantExcludes int
	}{
		{name: "singleton overlap", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{a}, wantError: "value-or-missing"},
		{name: "excluded type omitted", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{untypedA}, wantError: "value-or-missing"},
		{name: "included type omitted", included: []*momentpb.Moment{untypedA}, excluded: []*momentpb.Moment{a}, wantError: "value-or-missing"},
		{name: "metadata elsewhere in includes", included: []*momentpb.Moment{untypedA, b}, excluded: []*momentpb.Moment{untypedA}, wantError: "value-or-missing"},
		{name: "metadata elsewhere in exclusions", included: []*momentpb.Moment{untypedA}, excluded: []*momentpb.Moment{untypedA, b}, wantError: "value-or-missing"},
		{name: "nonmetadata overlap unchanged", included: []*momentpb.Moment{untypedA}, excluded: []*momentpb.Moment{untypedA}},
		{name: "one include multiple exclusions", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{b, a}, wantError: "value-or-missing"},
		{name: "multiple includes one exclusion", included: []*momentpb.Moment{b, a}, excluded: []*momentpb.Moment{a}, wantError: "value-or-missing"},
		{name: "multiple includes and exclusions", included: []*momentpb.Moment{a, b}, excluded: []*momentpb.Moment{b, a}, wantError: "value-or-missing"},
		{name: "duplicate IDs", included: []*momentpb.Moment{a, a}, excluded: []*momentpb.Moment{a, a}, wantError: "value-or-missing"},
		{name: "invalid included name", included: []*momentpb.Moment{invalid, a}, excluded: []*momentpb.Moment{b}, wantError: "unparsable included moment name"},
		{name: "invalid excluded name", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{invalid, b}, wantError: "unparsable excluded moment name"},
		{name: "distinct IDs retain AND", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{b}, wantIncludes: 1, wantExcludes: 1},
		{name: "include only", included: []*momentpb.Moment{a}, wantIncludes: 1},
		{name: "exclude only", excluded: []*momentpb.Moment{a}, wantExcludes: 1},
		{name: "exclusions disabled", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{a}, disabled: true, wantIncludes: 1},
		{name: "omitted type exclusions disabled", included: []*momentpb.Moment{a}, excluded: []*momentpb.Moment{untypedA}, disabled: true, wantIncludes: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			*enableExcludedMoments = !tc.disabled
			attribute := &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{{
				Moments: tc.included, ExcludedMoments: tc.excluded,
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
					MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "selected"}},
				}},
			}}}

			// When
			_, includes, excludes, _, err := parseClickhouseFilter(context.Background(), &analyticspb.TimeRange{
				StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
			}, attribute, []ClickhouseTable{messageTable}, false)

			// Then
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("got error %v, want %q", err, tc.wantError)
				}
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("got error code %v, want InvalidArgument", status.Code(err))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(includes) != tc.wantIncludes || len(excludes) != tc.wantExcludes {
				t.Fatalf("got %d includes, %d excludes; want %d, %d", len(includes), len(excludes), tc.wantIncludes, tc.wantExcludes)
			}
		})
	}
}

func TestParseClickhouseFilterForQAAttribute(t *testing.T) {
	gotConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
		},
		scorecardScoreTable, false,
	)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}
	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((conversation_formatted_time >= ?) AND " +
		"(conversation_formatted_time < ?) AND " +
		"(agent_user_id IN (?)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1", "agent-id2"},
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilterForQAAttribute_withNewScoreTable(t *testing.T) {
	gotConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
		},
		scoreTable, false,
	)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}
	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((scorecard_time >= ?) AND " +
		"(scorecard_time < ?) AND " +
		"(agent_user_id IN (?)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1", "agent-id2"},
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

// score_d and scorecard_d have no conversation_end_time column, so a request filtering by
// conversation end time must still bound their own scorecard_time, the leading component of
// their primary key. An unbounded scan of either table reads every granule of every partition.
func TestParseClickhouseFilterForQAAttribute_filterByConversationEndedAtStaysBounded(t *testing.T) {
	testCases := []struct {
		desc           string
		table          ClickhouseTable
		wantConditions string
	}{
		{
			desc:  "score table",
			table: scoreTable,
			wantConditions: "((scorecard_time >= ?) AND " +
				"(scorecard_time < ?) AND " +
				"(agent_user_id IN (?)) AND " +
				"(percentage_value >= ?) AND " +
				"(not_applicable <> ?))",
		},
		{
			desc:  "scorecard table",
			table: scorecardTable,
			wantConditions: "((scorecard_time >= ?) AND " +
				"(scorecard_time < ?) AND " +
				"(agent_user_id IN (?)) AND " +
				"(score >= ?))",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Given
			timeRange := &analyticspb.TimeRange{
				StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
			}
			qaAttribute := &analyticspb.QAAttribute{
				Users: []*userpb.User{
					{Name: "customers/test_customer/users/agent-id1"},
				},
			}

			// When
			gotConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
				timeRange, qaAttribute, tc.table, false, /* useExtTable */
				WithTimeRangeType(analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT),
			)

			// Then
			if err != nil {
				t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
			}
			gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
			if diff := cmp.Diff(tc.wantConditions, gotConditions); diff != "" {
				t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
			}
			wantTimeArgs := []any{
				"2021-01-01 00:00:00",
				"2021-01-31 00:00:00",
			}
			if diff := cmp.Diff(wantTimeArgs, gotArgs[:len(wantTimeArgs)]); diff != "" {
				t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong time arguments with diff (-want +got): %v", diff)
			}
		})
	}
}

// Bounding scorecard_time in place of conversation_end_time under-counts, so with the flag on the
// request is rejected rather than answered with a number that is quietly a little low.
func TestParseClickhouseFilterForQAAttribute_failClosedOnUnsupportedTimeRange(t *testing.T) {
	testCases := []struct {
		desc    string
		table   ClickhouseTable
		wantErr bool
	}{
		{desc: "score table has no conversation_end_time", table: scoreTable, wantErr: true},
		{desc: "scorecard table has no conversation_end_time", table: scorecardTable, wantErr: true},
		{desc: "conversation table has conversation_end_time", table: conversationTable, wantErr: false},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Given
			timeRange := &analyticspb.TimeRange{
				StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
			}

			// When
			_, _, err := parseScoreConditionsForQAAttribute(
				timeRange, &analyticspb.QAAttribute{}, tc.table, false, /* useExtTable */
				WithTimeRangeType(analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT),
				WithFailClosedOnUnsupportedTimeRange(true),
			)

			// Then
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("parseScoreConditionsForQAAttribute() should reject a time range the table cannot express")
			}
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Fatalf("parseScoreConditionsForQAAttribute() returned code %v, want %v", got, codes.InvalidArgument)
			}
		})
	}
}

// The flag is the rollout control, so the substitution has to survive until it is turned on.
func TestParseClickhouseFilterForQAAttribute_failClosedFlagOffKeepsFallback(t *testing.T) {
	// Given
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}

	// When
	gotConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		timeRange, &analyticspb.QAAttribute{}, scoreTable, false, /* useExtTable */
		WithTimeRangeType(analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT),
		WithFailClosedOnUnsupportedTimeRange(false),
	)

	// Then
	if err != nil {
		t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
	}
	gotConditions, _ := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((scorecard_time >= ?) AND (scorecard_time < ?) AND " +
		"(percentage_value >= ?) AND (not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilterForQAAttribute_scorecardConditions(t *testing.T) {
	gotConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(
		&analyticspb.QAAttribute{
			ScorecardStatuses: []analyticspb.QAAttribute_ScorecardStatus{
				analyticspb.QAAttribute_MANUALLY_SUBMITTED,
				analyticspb.QAAttribute_DRAFT,
			},
		},
	)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}
	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((((scorecard_submit_time <> ?) OR " +
		"(scorecard_submit_time = ? AND manually_scored = ?))))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{0, 0, true}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseScorecardConditionsForQAAttribute_published(t *testing.T) {
	gotConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(
		&analyticspb.QAAttribute{
			ScorecardStatuses: []analyticspb.QAAttribute_ScorecardStatus{
				analyticspb.QAAttribute_PUBLISHED,
			},
		},
	)
	if err != nil {
		t.Fatalf("parseScorecardConditionsForQAAttribute() failed with error: %v", err)
	}
	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((((scorecard_publish_time <> ?))))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScorecardConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{0}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScorecardConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseScorecardConditionsForQAAttribute_notPublished(t *testing.T) {
	gotConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(
		&analyticspb.QAAttribute{
			ScorecardStatuses: []analyticspb.QAAttribute_ScorecardStatus{
				analyticspb.QAAttribute_NOT_PUBLISHED,
			},
		},
	)
	if err != nil {
		t.Fatalf("parseScorecardConditionsForQAAttribute() failed with error: %v", err)
	}
	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "(((((scorecard_publish_time = ? AND scorecard_submit_time <> ?)))))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScorecardConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{0, 0}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScorecardConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseScorecardConditionsForQAAttribute_publishedAndManuallySubmitted(t *testing.T) {
	gotConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(
		&analyticspb.QAAttribute{
			ScorecardStatuses: []analyticspb.QAAttribute_ScorecardStatus{
				analyticspb.QAAttribute_PUBLISHED,
				analyticspb.QAAttribute_MANUALLY_SUBMITTED,
			},
		},
	)
	if err != nil {
		t.Fatalf("parseScorecardConditionsForQAAttribute() failed with error: %v", err)
	}
	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((((scorecard_publish_time <> ?) OR " +
		"(scorecard_submit_time <> ?))))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScorecardConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{0, 0}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScorecardConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilterForQAAttribute_withMomentFilters(t *testing.T) {
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	qaAttribute := &analyticspb.QAAttribute{
		Users: []*userpb.User{
			{
				Name: "customers/test_customer/users/agent-id1",
			},
			{
				Name: "customers/test_customer/users/agent-id2",
			},
		},
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/test_customer/profiles/test_profile/moments/moment-id1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{
						MetadataValue: &commonpb.MetadataValue{
							Value: &commonpb.MetadataValue_StringValue{
								StringValue: "metadata-value-1",
							},
						},
					},
					{
						MetadataValue: &commonpb.MetadataValue{
							Value: &commonpb.MetadataValue_StringValue{
								StringValue: "metadata-value-2",
							},
						},
					},
				},
			},
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/test_customer/profiles/test_profile/moments/moment-id2",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{
						NumericBin: &commonpb.NumericBin{
							FromValue: 0,
							ToValue:   100,
						},
					},
					{
						NumericBin: &commonpb.NumericBin{
							FromValue: 100,
							ToValue:   250,
						},
					},
				},
			},
			{
				ExcludedMoments: []*momentpb.Moment{
					{
						Name: "customers/test_customer/profiles/test_profile/moments/moment-id3",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
			},
		},
	}
	gotConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scorecardScoreTable, false)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}
	gotMomentFilters, gotExcludedMomentFilters, _, err := parseMomentConditionsForQAAttribute(timeRange, qaAttribute, false, conversationStartTimeColumn)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((conversation_formatted_time >= ?) AND " +
		"(conversation_formatted_time < ?) AND " +
		"(agent_user_id IN (?)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1", "agent-id2"},
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}

	if len(gotMomentFilters) != 2 {
		t.Fatalf("gotMomentFilters should have 2 slices but not %d", len(gotMomentFilters))
	}
	if gotMomentFilters[0].source != momentAnnotationFilterSourceMetadataMV {
		t.Fatalf("expected metadata MV source for first moment filter, got %v", gotMomentFilters[0].source)
	}
	gotMomentConditionsForString, gotMomentArgsForString := concatConditionsAndArgs(gotMomentFilters[0].conditions)
	wantMomentConditionsForString := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_template_id = ?) AND " +
		"(metadata_string_value = ? OR metadata_string_value = ?))"
	if diff := cmp.Diff(wantMomentConditionsForString, gotMomentConditionsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong moment conditions for string metadata value with diff (-want +got): %v", diff)
	}
	wantMomentArgsForString := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		"moment-id1",
		"metadata-value-1", "metadata-value-2",
	}
	if diff := cmp.Diff(wantMomentArgsForString, gotMomentArgsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments for string metadata value with diff (-want +got): %v", diff)
	}
	gotMomentConditionsForNumberBin, gotMomentArgsForNumberBin := concatConditionsAndArgs(gotMomentFilters[1].conditions)
	wantMomentConditionsForNumberBin := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_template_id = ?) AND " +
		"((metadata_number_value >= ? AND metadata_number_value < ?) OR " +
		"(metadata_number_value >= ? AND metadata_number_value < ?)))"
	if diff := cmp.Diff(wantMomentConditionsForNumberBin, gotMomentConditionsForNumberBin); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong moment conditions for number metadata value with diff (-want +got): %v", diff)
	}
	wantMomentArgsForNumberBin := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		"moment-id2",
		float64(0.0), float64(100.0),
		float64(100.0), float64(250.0),
	}
	if diff := cmp.Diff(wantMomentArgsForNumberBin, gotMomentArgsForNumberBin); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments for number metadata value with diff (-want +got): %v", diff)
	}

	if len(gotExcludedMomentFilters) != 1 {
		t.Fatalf("gotExcludedMomentFilters should have 1 slice but not %d", len(gotExcludedMomentFilters))
	}
	gotExcludedMomentConditionsForString, gotExcludedMomentArgsForString := concatConditionsAndArgs(gotExcludedMomentFilters[0].conditions)
	wantExcludedMomentConditionsForString := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_template_id = ?))"
	if diff := cmp.Diff(wantExcludedMomentConditionsForString, gotExcludedMomentConditionsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong excluded moment conditions for string metadata value with diff (-want +got): %v", diff)
	}
	wantExcludedMomentArgsForString := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		"moment-id3",
	}
	if diff := cmp.Diff(wantExcludedMomentArgsForString, gotExcludedMomentArgsForString); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments for exluded moments with diff (-want +got): %v", diff)
	}
}

func TestParseMomentConditionsForQAAttribute_MetadataMomentOverlap(t *testing.T) {
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	baseMoment := &momentpb.Moment{
		Name: "customers/test_customer/profiles/test_profile/moments/moment-id1",
		Type: momentpb.Moment_CONVERSATION_METADATA,
	}
	stringValues := []*analyticspb.MetadataValueAttribute{
		{
			MetadataValue: &commonpb.MetadataValue{
				Value: &commonpb.MetadataValue_StringValue{
					StringValue: "metadata-value-1",
				},
			},
		},
		{
			MetadataValue: &commonpb.MetadataValue{
				Value: &commonpb.MetadataValue_StringValue{
					StringValue: "metadata-value-2",
				},
			},
		},
	}

	valueCondition := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_template_id = ?) AND " +
		"(metadata_string_value = ? OR metadata_string_value = ?))"
	valueArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		"moment-id1",
		"metadata-value-1", "metadata-value-2",
	}

	missingCondition := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(moment_template_id = ?))"
	missingArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		"moment-id1",
	}

	t.Run("value-only", func(t *testing.T) {
		// Given
		qaAttribute := &analyticspb.QAAttribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments:                 []*momentpb.Moment{baseMoment},
					MetadataValueAttributes: stringValues,
				},
			},
		}

		// When
		gotMomentFilters, gotExcludedMomentFilters, gotValueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
			timeRange, qaAttribute, false, conversationStartTimeColumn,
		)

		// Then
		if err != nil {
			t.Fatalf("parseMomentConditionsForQAAttribute() failed: %v", err)
		}
		if len(gotMomentFilters) != 1 {
			t.Fatalf("gotMomentFilters len = %d, want 1", len(gotMomentFilters))
		}
		if len(gotExcludedMomentFilters) != 0 {
			t.Fatalf("gotExcludedMomentFilters should be empty, got %d", len(gotExcludedMomentFilters))
		}
		if len(gotValueOrMissingFilters) != 0 {
			t.Fatalf("gotValueOrMissingFilters should be empty, got %d", len(gotValueOrMissingFilters))
		}
		if gotMomentFilters[0].source != momentAnnotationFilterSourceMetadataMV {
			t.Fatalf("expected metadata MV source, got %v", gotMomentFilters[0].source)
		}

		gotConditions, gotArgs := concatConditionsAndArgs(gotMomentFilters[0].conditions)
		if gotConditions != valueCondition {
			t.Fatalf("unexpected conditions: got %q, want %q", gotConditions, valueCondition)
		}
		if diff := cmp.Diff(valueArgs, gotArgs); diff != "" {
			t.Fatalf("unexpected args (-want +got): %s", diff)
		}
	})

	t.Run("no-value-only", func(t *testing.T) {
		// Given
		qaAttribute := &analyticspb.QAAttribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					ExcludedMoments: []*momentpb.Moment{baseMoment},
				},
			},
		}

		// When
		_, gotExcludedMomentFilters, gotValueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
			timeRange, qaAttribute, false, conversationStartTimeColumn,
		)

		// Then
		if err != nil {
			t.Fatalf("parseMomentConditionsForQAAttribute() failed: %v", err)
		}
		if len(gotValueOrMissingFilters) != 0 {
			t.Fatalf("gotValueOrMissingFilters should be empty, got %d", len(gotValueOrMissingFilters))
		}
		if len(gotExcludedMomentFilters) != 1 {
			t.Fatalf("gotExcludedMomentFilters len = %d, want 1", len(gotExcludedMomentFilters))
		}
		if gotExcludedMomentFilters[0].source != momentAnnotationFilterSourceMetadataMV {
			t.Fatalf("expected metadata MV source, got %v", gotExcludedMomentFilters[0].source)
		}

		gotConditions, gotArgs := concatConditionsAndArgs(gotExcludedMomentFilters[0].conditions)
		if gotConditions != missingCondition {
			t.Fatalf("unexpected conditions: got %q, want %q", gotConditions, missingCondition)
		}
		if diff := cmp.Diff(missingArgs, gotArgs); diff != "" {
			t.Fatalf("unexpected args (-want +got): %s", diff)
		}
	})

	t.Run("overlapping value+no-value", func(t *testing.T) {
		// Given
		qaAttribute := &analyticspb.QAAttribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments:                 []*momentpb.Moment{baseMoment},
					ExcludedMoments:         []*momentpb.Moment{baseMoment},
					MetadataValueAttributes: stringValues,
				},
			},
		}

		// When
		gotMomentFilters, gotExcludedMomentFilters, gotValueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
			timeRange, qaAttribute, false, conversationStartTimeColumn,
		)

		// Then
		if err != nil {
			t.Fatalf("parseMomentConditionsForQAAttribute() failed: %v", err)
		}
		if len(gotMomentFilters) != 0 {
			t.Fatalf("gotMomentFilters should be empty, got %d", len(gotMomentFilters))
		}
		if len(gotExcludedMomentFilters) != 0 {
			t.Fatalf("gotExcludedMomentFilters should be empty, got %d", len(gotExcludedMomentFilters))
		}
		if len(gotValueOrMissingFilters) != 1 {
			t.Fatalf("gotValueOrMissingFilters len = %d, want 1", len(gotValueOrMissingFilters))
		}
		if gotValueOrMissingFilters[0].selectedValueFilter.source != momentAnnotationFilterSourceMetadataMV {
			t.Fatalf("expected selectedValueFilter source metadata MV, got %v", gotValueOrMissingFilters[0].selectedValueFilter.source)
		}
		if gotValueOrMissingFilters[0].existenceFilter.source != momentAnnotationFilterSourceMetadataMV {
			t.Fatalf("expected existenceFilter source metadata MV, got %v", gotValueOrMissingFilters[0].existenceFilter.source)
		}

		gotValueConditions, gotValueArgs := concatConditionsAndArgs(gotValueOrMissingFilters[0].selectedValueFilter.conditions)
		if gotValueConditions != valueCondition {
			t.Fatalf("unexpected value conditions: got %q, want %q", gotValueConditions, valueCondition)
		}
		if diff := cmp.Diff(valueArgs, gotValueArgs); diff != "" {
			t.Fatalf("unexpected value args (-want +got): %s", diff)
		}

		gotMissingConditions, gotMissingArgs := concatConditionsAndArgs(gotValueOrMissingFilters[0].existenceFilter.conditions)
		if gotMissingConditions != missingCondition {
			t.Fatalf("unexpected missing conditions: got %q, want %q", gotMissingConditions, missingCondition)
		}
		if diff := cmp.Diff(missingArgs, gotMissingArgs); diff != "" {
			t.Fatalf("unexpected missing args (-want +got): %s", diff)
		}
	})

	t.Run("overlap plus independent exclusion", func(t *testing.T) {
		// Given
		excludedMoment := &momentpb.Moment{
			Name: "customers/test_customer/profiles/test_profile/moments/moment-id3",
			Type: momentpb.Moment_CONVERSATION_METADATA,
		}
		qaAttribute := &analyticspb.QAAttribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments:                 []*momentpb.Moment{baseMoment},
					ExcludedMoments:         []*momentpb.Moment{baseMoment},
					MetadataValueAttributes: stringValues,
				},
				{
					ExcludedMoments: []*momentpb.Moment{excludedMoment},
				},
			},
		}

		// When
		gotMomentFilters, gotExcludedMomentFilters, gotValueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
			timeRange, qaAttribute, false, conversationStartTimeColumn,
		)

		// Then
		if err != nil {
			t.Fatalf("parseMomentConditionsForQAAttribute() failed: %v", err)
		}
		if len(gotMomentFilters) != 0 {
			t.Fatalf("gotMomentFilters should be empty, got %d", len(gotMomentFilters))
		}
		if len(gotValueOrMissingFilters) != 1 {
			t.Fatalf("gotValueOrMissingFilters len = %d, want 1", len(gotValueOrMissingFilters))
		}
		if len(gotExcludedMomentFilters) != 1 {
			t.Fatalf("gotExcludedMomentFilters len = %d, want 1", len(gotExcludedMomentFilters))
		}

		gotExcludedConditions, gotExcludedArgs := concatConditionsAndArgs(gotExcludedMomentFilters[0].conditions)
		if gotExcludedConditions != missingCondition {
			t.Fatalf("unexpected excluded conditions: got %q, want %q", gotExcludedConditions, missingCondition)
		}
		expectedExcludedArgs := []any{
			"2021-01-01 00:00:00",
			"2021-01-31 00:00:00",
			"moment-id3",
		}
		if diff := cmp.Diff(expectedExcludedArgs, gotExcludedArgs); diff != "" {
			t.Fatalf("unexpected excluded args (-want +got): %s", diff)
		}
	})
}

func TestParseMomentConditionsForQAAttribute_MetadataMomentOverlapWithoutConversationTimeRange(t *testing.T) {
	// Given
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment-id1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				ExcludedMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment-id1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{
						MetadataValue: &commonpb.MetadataValue{
							Value: &commonpb.MetadataValue_StringValue{StringValue: "metadata-value-1"},
						},
					},
				},
			},
		},
	}

	// When
	momentFilters, excludedMomentFilters, valueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
		nil,
		qaAttribute,
		false,
		conversationStartTimeColumn,
	)

	// Then
	if err != nil {
		t.Fatalf("parseMomentConditionsForQAAttribute() failed: %v", err)
	}
	if len(momentFilters) != 0 {
		t.Fatalf("momentFilters should be empty, got %d", len(momentFilters))
	}
	if len(excludedMomentFilters) != 0 {
		t.Fatalf("excludedMomentFilters should be empty, got %d", len(excludedMomentFilters))
	}
	if len(valueOrMissingFilters) != 1 {
		t.Fatalf("valueOrMissingFilters len = %d, want 1", len(valueOrMissingFilters))
	}

	selectedValueConditions, selectedValueArgs := concatConditionsAndArgs(valueOrMissingFilters[0].selectedValueFilter.conditions)
	if strings.Contains(selectedValueConditions, conversationStartTimeColumn) {
		t.Fatalf("selected value conditions unexpectedly contain %s: %q", conversationStartTimeColumn, selectedValueConditions)
	}
	if diff := cmp.Diff([]any{"moment-id1", "metadata-value-1"}, selectedValueArgs); diff != "" {
		t.Fatalf("unexpected selected value args (-want +got): %s", diff)
	}

	existenceConditions, existenceArgs := concatConditionsAndArgs(valueOrMissingFilters[0].existenceFilter.conditions)
	if strings.Contains(existenceConditions, conversationStartTimeColumn) {
		t.Fatalf("existence conditions unexpectedly contain %s: %q", conversationStartTimeColumn, existenceConditions)
	}
	if diff := cmp.Diff([]any{"moment-id1"}, existenceArgs); diff != "" {
		t.Fatalf("unexpected existence args (-want +got): %s", diff)
	}
}

func TestParseMomentConditionsForQAAttribute_RejectsUnsupportedMixedMomentGroups(t *testing.T) {
	testCases := []struct {
		name           string
		includedMoment *momentpb.Moment
		excludedMoment *momentpb.Moment
		metadataValues []*analyticspb.MetadataValueAttribute
	}{
		{
			name: "different metadata moment IDs",
			includedMoment: &momentpb.Moment{
				Name: "customers/customer_id/profiles/profile_id/moments/included-id",
				Type: momentpb.Moment_CONVERSATION_METADATA,
			},
			excludedMoment: &momentpb.Moment{
				Name: "customers/customer_id/profiles/profile_id/moments/excluded-id",
				Type: momentpb.Moment_CONVERSATION_METADATA,
			},
			metadataValues: []*analyticspb.MetadataValueAttribute{
				{
					MetadataValue: &commonpb.MetadataValue{
						Value: &commonpb.MetadataValue_StringValue{StringValue: "selected-value"},
					},
				},
			},
		},
		{
			name: "conversation outcome",
			includedMoment: &momentpb.Moment{
				Name: "customers/customer_id/profiles/profile_id/moments/outcome-id",
				Type: momentpb.Moment_CONVERSATION_OUTCOME,
			},
			excludedMoment: &momentpb.Moment{
				Name: "customers/customer_id/profiles/profile_id/moments/outcome-id",
				Type: momentpb.Moment_CONVERSATION_OUTCOME,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			qaAttribute := &analyticspb.QAAttribute{
				MomentGroups: []*analyticspb.MomentGroup{
					{
						Moments:                 []*momentpb.Moment{testCase.includedMoment},
						ExcludedMoments:         []*momentpb.Moment{testCase.excludedMoment},
						MetadataValueAttributes: testCase.metadataValues,
					},
				},
			}

			// When
			_, _, _, err := parseMomentConditionsForQAAttribute(
				&analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
				},
				qaAttribute,
				false,
				conversationStartTimeColumn,
			)

			// Then
			if err == nil {
				t.Fatal("parseMomentConditionsForQAAttribute() unexpectedly succeeded")
			}
			if !strings.Contains(err.Error(), "require one CONVERSATION_METADATA moment with the same moment ID") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseClickhouseFilter_withExtTable(t *testing.T) {
	gotConditionsAndArgs, _, _, extTable, err := parseClickhouseFilter(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		}, &analyticspb.Attribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
			AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
				{
					Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
				},
				{
					Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
				},
			},
			ConversationDurationRange: &analyticspb.DurationRange{
				MinDuration: &durationpb.Duration{
					Seconds: 10,
				},
				MaxDuration: &durationpb.Duration{
					Seconds: 100,
				},
			},
		},
		[]ClickhouseTable{momentAnnotationTable}, true)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	if extTable == nil {
		t.Fatal("parseClickhouseFilter() should return non-nil ext table when useExtTable=true")
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs[momentAnnotationTable])
	wantConditions := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(agent_user_id IN (SELECT user_id FROM agent_filter)) AND " +
		"(policy_id IN (?)) AND " +
		"(conversation_duration_secs >= ?) AND " +
		"(conversation_duration_secs < ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	// User IDs should NOT be in args when using ext table
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"policy-id1", "policy-id2"},
		10.0,
		100.0,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilter_withExtTable_noUsers(t *testing.T) {
	gotConditionsAndArgs, _, _, extTable, err := parseClickhouseFilter(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		}, &analyticspb.Attribute{
			AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
				{
					Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
				},
			},
		},
		[]ClickhouseTable{momentAnnotationTable}, true)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	if extTable != nil {
		t.Fatal("parseClickhouseFilter() should return nil ext table when no users are provided")
	}

	gotConditions, _ := concatConditionsAndArgs(gotConditionsAndArgs[momentAnnotationTable])
	wantConditions := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(policy_id IN (?)))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
}

func TestParseScoreConditionsForQAAttribute_withExtTable(t *testing.T) {
	gotConditionsAndArgs, extTables, err := parseScoreConditionsForQAAttribute(
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
		},
		scorecardScoreTable, true,
	)
	if err != nil {
		t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
	}

	if len(extTables) != 1 {
		t.Fatal("parseScoreConditionsForQAAttribute() should return non-nil ext table when useExtTable=true")
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((conversation_formatted_time >= ?) AND " +
		"(conversation_formatted_time < ?) AND " +
		"(agent_user_id IN (SELECT user_id FROM agent_filter)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	// User IDs should NOT be in args when using ext table
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseScoreConditionsForQAAttribute_withExtTable_newScoreTable(t *testing.T) {
	gotConditionsAndArgs, extTables, err := parseScoreConditionsForQAAttribute(
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{
					Name: "customers/test_customer/users/agent-id1",
				},
				{
					Name: "customers/test_customer/users/agent-id2",
				},
			},
		},
		scoreTable, true,
	)
	if err != nil {
		t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
	}

	if len(extTables) != 1 {
		t.Fatal("parseScoreConditionsForQAAttribute() should return non-nil ext table when useExtTable=true")
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((scorecard_time >= ?) AND " +
		"(scorecard_time < ?) AND " +
		"(agent_user_id IN (SELECT user_id FROM agent_filter)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseScoreConditionsForQAAttribute_WithAgentAndSubmitterExtTables(t *testing.T) {
	gotConditionsAndArgs, extTables, err := parseScoreConditionsForQAAttribute(
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{Name: "customers/test_customer/users/agent-id1"},
			},
			ScorecardReviewerAudience: &analyticspb.UserAudience{
				Users: []string{"customers/test_customer/users/submitter-id1"},
			},
		},
		scoreTable,
		true,
	)
	if err != nil {
		t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
	}

	if len(extTables) != 2 {
		t.Fatalf("parseScoreConditionsForQAAttribute() should return 2 ext tables, got %d", len(extTables))
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((scorecard_time >= ?) AND " +
		"(scorecard_time < ?) AND " +
		"(agent_user_id IN (SELECT user_id FROM agent_filter)) AND " +
		"(submitter_user_id IN (SELECT user_id FROM submitter_filter)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseConversationConditionsForQAAttribute_DoesNotFilterSubmitterOnConversationTable(t *testing.T) {
	gotConditionsAndArgs, extTables, err := parseConversationConditionsForQAAttribute(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{Name: "customers/test_customer/users/agent-id1"},
			},
			ScorecardReviewerAudience: &analyticspb.UserAudience{
				Users: []string{"customers/test_customer/users/submitter-id1"},
			},
			ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
				{Id: "1"},
			},
		},
		false,
	)
	if err != nil {
		t.Fatalf("parseConversationConditionsForQAAttribute() failed with error: %v", err)
	}

	if len(extTables) != 0 {
		t.Fatalf("parseConversationConditionsForQAAttribute() should not return ext tables when useExtTable=false, got %d", len(extTables))
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((conversation_duration_secs >= ?) AND " +
		"(conversation_duration_secs < ?) AND " +
		"(conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(agent_user_id IN (?)))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseConversationConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		0.0,
		60.0,
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1"},
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseConversationConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilter_ConsecutiveDurationBuckets(t *testing.T) {
	gotConditionsAndArgs, _, _, _, err := parseClickhouseFilter(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		}, &analyticspb.Attribute{
			ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
				{Id: "3"},
				{Id: "4"},
			},
		},
		[]ClickhouseTable{conversationTable}, false)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs[conversationTable])
	wantConditions := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"(conversation_duration_secs >= ?) AND " +
		"(conversation_duration_secs < ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		120.0,
		240.0,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseFilter_NonConsecutiveDurationBuckets(t *testing.T) {
	gotConditionsAndArgs, _, _, _, err := parseClickhouseFilter(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		}, &analyticspb.Attribute{
			ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
				{Id: "1"},
				{Id: "3"},
			},
		},
		[]ClickhouseTable{conversationTable}, false)
	if err != nil {
		t.Fatalf("parseClickhouseFilter() failed with error: %v", err)
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs[conversationTable])
	wantConditions := "((conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?) AND " +
		"((conversation_duration_secs >= ? AND conversation_duration_secs < ?) OR " +
		"(conversation_duration_secs >= ? AND conversation_duration_secs < ?)))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		0.0,
		60.0,
		120.0,
		180.0,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseClickhouseFilter() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseConversationConditionsForQAAttribute_ConsecutiveDurationBuckets(t *testing.T) {
	gotConditionsAndArgs, extTables, err := parseConversationConditionsForQAAttribute(
		context.Background(),
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
				{Id: "3"},
				{Id: "4"},
			},
		},
		false,
	)
	if err != nil {
		t.Fatalf("parseConversationConditionsForQAAttribute() failed with error: %v", err)
	}

	if len(extTables) != 0 {
		t.Fatalf("parseConversationConditionsForQAAttribute() should not return ext tables when useExtTable=false, got %d", len(extTables))
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((conversation_duration_secs >= ?) AND " +
		"(conversation_duration_secs < ?) AND " +
		"(conversation_start_time >= ?) AND " +
		"(conversation_start_time < ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseConversationConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		120.0,
		240.0,
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseConversationConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseScoreConditionsForQAAttribute_WithAgentAndSubmitterInlineFilters(t *testing.T) {
	gotConditionsAndArgs, extTables, err := parseScoreConditionsForQAAttribute(
		&analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		&analyticspb.QAAttribute{
			Users: []*userpb.User{
				{Name: "customers/test_customer/users/agent-id1"},
			},
			ScorecardReviewerAudience: &analyticspb.UserAudience{
				Users: []string{"customers/test_customer/users/submitter-id1"},
			},
		},
		scoreTable,
		false,
	)
	if err != nil {
		t.Fatalf("parseScoreConditionsForQAAttribute() failed with error: %v", err)
	}
	if len(extTables) != 0 {
		t.Fatalf("parseScoreConditionsForQAAttribute() should not return ext tables when useExtTable=false, got %d", len(extTables))
	}

	gotConditions, gotArgs := concatConditionsAndArgs(gotConditionsAndArgs)
	wantConditions := "((scorecard_time >= ?) AND " +
		"(scorecard_time < ?) AND " +
		"(agent_user_id IN (?)) AND " +
		"(submitter_user_id IN (?)) AND " +
		"(percentage_value >= ?) AND " +
		"(not_applicable <> ?))"
	if diff := cmp.Diff(wantConditions, gotConditions); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong conditions with diff (-want +got): %v", diff)
	}
	wantArgs := []any{
		"2021-01-01 00:00:00",
		"2021-01-31 00:00:00",
		[]string{"agent-id1"},
		[]string{"submitter-id1"},
		0,
		true,
	}
	if diff := cmp.Diff(wantArgs, gotArgs); diff != "" {
		t.Fatalf("parseScoreConditionsForQAAttribute() returned wrong arguments with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseGroupByForQAAttribute_WithScorecardSubmitter(t *testing.T) {
	frequency := analyticspb.Frequency_FREQUENCY_UNSPECIFIED
	gotGroupByKeys, gotGroupByQueries, err := parseClickhouseGroupByForQAAttribute(
		&frequency,
		[]analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER},
		nil,
		scoreTable,
	)
	if err != nil {
		t.Fatalf("parseClickhouseGroupByForQAAttribute() failed with error: %v", err)
	}

	if diff := cmp.Diff([]string{submitterUserIDColumn}, gotGroupByKeys); diff != "" {
		t.Fatalf("parseClickhouseGroupByForQAAttribute() returned wrong group by keys with diff (-want +got): %v", diff)
	}
	if diff := cmp.Diff([]string{submitterUserIDColumn}, gotGroupByQueries); diff != "" {
		t.Fatalf("parseClickhouseGroupByForQAAttribute() returned wrong group by queries with diff (-want +got): %v", diff)
	}
}

func TestParseClickhouseGroupByForQAAttribute_WithMissingScorecardSubmitterColumn(t *testing.T) {
	frequency := analyticspb.Frequency_FREQUENCY_UNSPECIFIED
	gotGroupByKeys, gotGroupByQueries, err := parseClickhouseGroupByForQAAttribute(
		&frequency,
		[]analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER},
		nil,
		conversationTable,
	)
	if err != nil {
		t.Fatalf("parseClickhouseGroupByForQAAttribute() failed with error: %v", err)
	}

	if diff := cmp.Diff([]string{submitterUserIDColumn}, gotGroupByKeys); diff != "" {
		t.Fatalf("parseClickhouseGroupByForQAAttribute() returned wrong group by keys with diff (-want +got): %v", diff)
	}
	if diff := cmp.Diff([]string{fmt.Sprintf("NULL AS %s", submitterUserIDColumn)}, gotGroupByQueries); diff != "" {
		t.Fatalf("parseClickhouseGroupByForQAAttribute() returned wrong group by queries with diff (-want +got): %v", diff)
	}
}

func TestColumnGTE(t *testing.T) {
	columnName := "test_column"
	for _, tc := range []struct {
		desc  string
		input any
		want  string
	}{
		{
			desc:  "GTE for int",
			input: 32,
			want:  fmt.Sprintf("%s >= ?", columnName),
		},
		{
			desc:  "GTE for float",
			input: 32.1,
			want:  fmt.Sprintf("%s >= ?", columnName),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got, arg := columnGTE(columnName, tc.input)
			if got != tc.want {
				t.Fatalf("columnGTE() returned wrong result for %s: got %v, want %v", tc.desc, got, tc.want)
			}
			if arg != tc.input {
				t.Fatalf("columnGTE() returned wrong argument for %s: got %v, want %v", tc.desc, arg, tc.input)
			}
		})
	}
}

func TestConvertConversationOutcomeMomentGroupFilter(t *testing.T) {
	startTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)
	momentGroup := &analyticspb.MomentGroup{
		Moments: []*momentpb.Moment{
			{
				Name: "customers/customer_id/profiles/profile_id/moments/outcome_moment_id",
				Type: momentpb.Moment_CONVERSATION_OUTCOME,
			},
		},
		OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
			{
				OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
					Value: &momentpb.ConversationOutcomeValue{
						Value: &momentpb.ConversationOutcomeValue_StringValue{StringValue: "resolved"},
					},
				},
			},
			{
				OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
					Value: &momentpb.ConversationOutcomeValue{
						Value: &momentpb.ConversationOutcomeValue_BooleanValue{BooleanValue: false},
					},
				},
			},
			{
				OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
					Value: &momentpb.ConversationOutcomeValue{
						Value: &momentpb.ConversationOutcomeValue_NumberValue{NumberValue: 42},
					},
				},
			},
			{
				NumericBin: &commonpb.NumericBin{
					FromValue:            10,
					ToValue:              20,
					FromValueIsExclusive: true,
					ToValueIsInclusive:   true,
				},
			},
		},
	}

	got, err := convertConversationOutcomeMomentGroupFilter(momentGroup, startTime, endTime, conversationStartTimeColumn, false)
	if err != nil {
		t.Fatalf("convertConversationOutcomeMomentGroupFilter() failed with error: %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("convertConversationOutcomeMomentGroupFilter() returned %d conditions, want 5: %v", len(got), got)
	}
	if got[2].condition != "moment_type = ?" || got[2].arg != int32(momentpb.Moment_CONVERSATION_OUTCOME) {
		t.Fatalf("unexpected moment type condition: %+v", got[2])
	}
	if got[3].condition != "moment_template_id = ?" || got[3].arg != "outcome_moment_id" {
		t.Fatalf("unexpected moment template condition: %+v", got[3])
	}

	valueCondition := got[4].condition
	for _, want := range []string{
		"JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = ?",
		"JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = ?",
		"JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') = ?",
		"JSONExtractBool(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') = ?",
		"JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = ?",
		"JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = ?",
		"JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') > ?",
		"JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= ?",
	} {
		if !strings.Contains(valueCondition, want) {
			t.Fatalf("value condition missing %q: %s", want, valueCondition)
		}
	}
	wantArgs := []any{true, "resolved", true, false, true, float64(42), true, float64(10), float64(20)}
	if !reflect.DeepEqual(got[4].args, wantArgs) {
		t.Fatalf("unexpected value args: got %#v, want %#v", got[4].args, wantArgs)
	}
}

func TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroup(t *testing.T) {
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome_moment_id",
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
					},
				},
				OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
					{
						OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
							Value: &momentpb.ConversationOutcomeValue{
								Value: &momentpb.ConversationOutcomeValue_StringValue{StringValue: "resolved"},
							},
						},
					},
				},
			},
		},
	}

	if !qaAttributeHasConversationOutcomeMomentGroup(qaAttribute) {
		t.Fatalf("qaAttributeHasConversationOutcomeMomentGroup() returned false, want true")
	}

	gotMomentFilters, gotExcludedMomentFilters, _, err := parseMomentConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		false, // useConversationOutcomeMV: CLO filters use moment_annotation_d + JSON payload.
		conversationStartTimeColumn,
	)
	if err != nil {
		t.Fatalf("parseMomentConditionsForQAAttribute() failed with error: %v", err)
	}
	if len(gotExcludedMomentFilters) != 0 {
		t.Fatalf("parseMomentConditionsForQAAttribute() returned excluded moment conditions: %v", gotExcludedMomentFilters)
	}
	if len(gotMomentFilters) != 1 {
		t.Fatalf("parseMomentConditionsForQAAttribute() returned %d moment condition groups, want 1: %v", len(gotMomentFilters), gotMomentFilters)
	}
	if gotMomentFilters[0].source != momentAnnotationFilterSourceRaw {
		t.Fatalf("expected raw moment annotation source, got %v", gotMomentFilters[0].source)
	}

	got := gotMomentFilters[0].conditions
	if len(got) != 5 {
		t.Fatalf("parseMomentConditionsForQAAttribute() returned %d conditions, want 5: %v", len(got), got)
	}
	if got[2].condition != "moment_type = ?" || got[2].arg != int32(momentpb.Moment_CONVERSATION_OUTCOME) {
		t.Fatalf("unexpected moment type condition: %+v", got[2])
	}
	if got[3].condition != "moment_template_id = ?" || got[3].arg != "outcome_moment_id" {
		t.Fatalf("unexpected moment template condition: %+v", got[3])
	}
	wantValueCondition := "(JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = ? AND JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = ?)"
	if got[4].condition != wantValueCondition {
		t.Fatalf("unexpected value condition: got %s, want %s", got[4].condition, wantValueCondition)
	}
	wantArgs := []any{true, "resolved"}
	if !reflect.DeepEqual(got[4].args, wantArgs) {
		t.Fatalf("unexpected value args: got %#v, want %#v", got[4].args, wantArgs)
	}
}

func TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroupNumericBin(t *testing.T) {
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome_moment_id",
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
					},
				},
				OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
					{
						NumericBin: &commonpb.NumericBin{
							FromValue:            10,
							ToValue:              20,
							FromValueIsExclusive: false,
							ToValueIsInclusive:   false,
						},
					},
				},
			},
		},
	}

	if !qaAttributeHasConversationOutcomeMomentGroup(qaAttribute) {
		t.Fatalf("qaAttributeHasConversationOutcomeMomentGroup() returned false, want true")
	}

	gotMomentFilters, gotExcludedMomentFilters, _, err := parseMomentConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		false, // useConversationOutcomeMV: CLO filters use moment_annotation_d + JSON payload.
		conversationStartTimeColumn,
	)
	if err != nil {
		t.Fatalf("parseMomentConditionsForQAAttribute() failed with error: %v", err)
	}
	if len(gotExcludedMomentFilters) != 0 {
		t.Fatalf("parseMomentConditionsForQAAttribute() returned excluded moment conditions: %v", gotExcludedMomentFilters)
	}
	if len(gotMomentFilters) != 1 {
		t.Fatalf("parseMomentConditionsForQAAttribute() returned %d moment condition groups, want 1: %v", len(gotMomentFilters), gotMomentFilters)
	}

	got := gotMomentFilters[0].conditions
	if len(got) != 5 {
		t.Fatalf("parseMomentConditionsForQAAttribute() returned %d conditions, want 5: %v", len(got), got)
	}
	if got[2].condition != "moment_type = ?" || got[2].arg != int32(momentpb.Moment_CONVERSATION_OUTCOME) {
		t.Fatalf("unexpected moment type condition: %+v", got[2])
	}
	if got[3].condition != "moment_template_id = ?" || got[3].arg != "outcome_moment_id" {
		t.Fatalf("unexpected moment template condition: %+v", got[3])
	}
	wantValueCondition := "(JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = ? AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') >= ? AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') < ?)"
	if got[4].condition != wantValueCondition {
		t.Fatalf("unexpected value condition: got %s, want %s", got[4].condition, wantValueCondition)
	}
	wantArgs := []any{true, float64(10), float64(20)}
	if !reflect.DeepEqual(got[4].args, wantArgs) {
		t.Fatalf("unexpected value args: got %#v, want %#v", got[4].args, wantArgs)
	}
}

func TestColumnIn(t *testing.T) {
	got, args := columnIn("test_column", []string{"a", "b", "c"})
	want := "test_column IN (?)"
	if got != want {
		t.Fatalf("columnIn() returned wrong result: got %v, want %v", got, want)
	}
	wantArgs := []string{"a", "b", "c"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("columnIn() returned wrong arguments: got %v, want %v", args, wantArgs)
	}
}

func TestTruncatedTimestamp(t *testing.T) {
	got := truncatedTimestamp("create_time", truncatedTimeColumn, "hour", -4, 7)
	want := "DATE_TRUNC('hour', create_time + toIntervalDay(7) + toIntervalHour(-4)) - toIntervalDay(7) - toIntervalHour(-4) AS truncated_time"
	if got != want {
		t.Fatalf("truncatedTimestamp() returns wrong result: got %v want %v", got, want)
	}
}

func TestQATimeRangeFilterTargetOptions(t *testing.T) {
	opts, err := qaTimeRangeFilterTargetOptions(
		analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
		analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := getConversationQueryOptions(opts...)
	if got.timeRangeColumnOverride != scorecardSubmitTimeColumn {
		t.Fatalf("override = %q, want %q", got.timeRangeColumnOverride, scorecardSubmitTimeColumn)
	}
	if col := getConversationTimeRangeColumn(got); col != scorecardSubmitTimeColumn {
		t.Fatalf("column = %q, want %q", col, scorecardSubmitTimeColumn)
	}

	_, err = qaTimeRangeFilterTargetOptions(
		analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
		analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
	)
	if err == nil {
		t.Fatal("expected InvalidArgument for SUBMIT_TIME + ENDED_AT")
	}

	opts, err = qaTimeRangeFilterTargetOptions(
		analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_UNSPECIFIED,
		analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got = getConversationQueryOptions(opts...)
	if got.timeRangeColumnOverride != "" {
		t.Fatalf("unexpected override %q for unspecified basis", got.timeRangeColumnOverride)
	}
	if col := getConversationTimeRangeColumn(got); col != conversationStartTimeColumn {
		t.Fatalf("column = %q, want %q", col, conversationStartTimeColumn)
	}
}

func TestParseMomentConditionsForQAAttribute_MetadataNumericBinBoundaries(t *testing.T) {
	moment := &momentpb.Moment{
		Name: "customers/customer_id/profiles/profile_id/moments/numeric",
		Type: momentpb.Moment_CONVERSATION_METADATA,
	}
	cases := []struct {
		name          string
		bin           *commonpb.NumericBin
		wantPredicate string
	}{
		{"default half open", &commonpb.NumericBin{FromValue: 5, ToValue: 10}, "metadata_number_value >= ? AND metadata_number_value < ?"},
		{"open", &commonpb.NumericBin{FromValue: 5, ToValue: 10, FromValueIsExclusive: true}, "metadata_number_value > ? AND metadata_number_value < ?"},
		{"closed", &commonpb.NumericBin{FromValue: 5, ToValue: 10, ToValueIsInclusive: true}, "metadata_number_value >= ? AND metadata_number_value <= ?"},
		{"left open right closed", &commonpb.NumericBin{FromValue: 5, ToValue: 10, FromValueIsExclusive: true, ToValueIsInclusive: true}, "metadata_number_value > ? AND metadata_number_value <= ?"},
		{"singleton", &commonpb.NumericBin{FromValue: 5, ToValue: 5, ToValueIsInclusive: true}, "metadata_number_value >= ? AND metadata_number_value <= ?"},
	}
	for _, tc := range cases {
		for _, withMissing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/missing=%t", tc.name, withMissing), func(t *testing.T) {
				// Given: preserve the caller's open/closed interval, including [5,5].
				group := &analyticspb.MomentGroup{
					Moments:                 []*momentpb.Moment{moment},
					MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{NumericBin: tc.bin}},
				}
				if withMissing {
					group.ExcludedMoments = []*momentpb.Moment{moment}
				}

				// When
				includes, _, overlaps, err := parseMomentConditionsForQAAttribute(nil,
					&analyticspb.QAAttribute{MomentGroups: []*analyticspb.MomentGroup{group}}, false, conversationStartTimeColumn)
				if err != nil {
					t.Fatal(err)
				}
				var filter momentAnnotationFilter
				if withMissing {
					if len(overlaps) != 1 {
						t.Fatalf("got %d overlap filters, want 1", len(overlaps))
					}
					filter = overlaps[0].selectedValueFilter
				} else {
					if len(includes) != 1 {
						t.Fatalf("got %d include filters, want 1", len(includes))
					}
					filter = includes[0]
				}
				query, _, args := buildQAMomentFilterQuery(filter, 0, "LEFT JOIN", "qa_score_info.conversation_id", "conversation_id_0")

				// Then: the latest-value CTE has no value bound; the matching CTE has the exact requested interval.
				if strings.Count(query, tc.wantPredicate) != 1 {
					t.Fatalf("expected one %q predicate in query: %s", tc.wantPredicate, query)
				}
				wantArgs := []any{"numeric", "numeric", float64(tc.bin.FromValue), float64(tc.bin.ToValue)}
				if diff := cmp.Diff(wantArgs, args); diff != "" {
					t.Fatalf("query arguments (-want +got): %s", diff)
				}
			})
		}
	}
}

func TestParseMomentConditionsForQAAttribute_MetadataValueValidation(t *testing.T) {
	moment := &momentpb.Moment{
		Name: "customers/customer_id/profiles/profile_id/moments/metadata",
		Type: momentpb.Moment_CONVERSATION_METADATA,
	}
	valid := &analyticspb.MetadataValueAttribute{MetadataValue: &commonpb.MetadataValue{
		Value: &commonpb.MetadataValue_StringValue{StringValue: "selected"},
	}}
	empty := &analyticspb.MetadataValueAttribute{MetadataValue: &commonpb.MetadataValue{}}
	cases := []struct {
		name       string
		attributes []*analyticspb.MetadataValueAttribute
		wantError  bool
	}{
		{name: "empty value oneof", attributes: []*analyticspb.MetadataValueAttribute{empty}, wantError: true},
		{name: "missing attribute fields", attributes: []*analyticspb.MetadataValueAttribute{{}}, wantError: true},
		{name: "valid then empty", attributes: []*analyticspb.MetadataValueAttribute{valid, empty}, wantError: true},
		{name: "empty then valid", attributes: []*analyticspb.MetadataValueAttribute{empty, valid}, wantError: true},
		{name: "empty string is a value", attributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: ""}}}}},
		{name: "zero is a value", attributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_NumberValue{NumberValue: 0}}}}},
		{name: "false is a value", attributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_BooleanValue{BooleanValue: false}}}}},
		{name: "nil attribute", attributes: []*analyticspb.MetadataValueAttribute{nil}, wantError: true},
	}
	for _, tc := range cases {
		for _, withMissing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/missing=%t", tc.name, withMissing), func(t *testing.T) {
				// Given
				group := &analyticspb.MomentGroup{Moments: []*momentpb.Moment{moment}, MetadataValueAttributes: tc.attributes}
				if withMissing {
					group.ExcludedMoments = []*momentpb.Moment{moment}
				}

				// When
				includes, _, overlaps, err := parseMomentConditionsForQAAttribute(nil,
					&analyticspb.QAAttribute{MomentGroups: []*analyticspb.MomentGroup{group}}, false, conversationStartTimeColumn)

				// Then: reject malformed values before generating SQL, but retain explicit zero values.
				if tc.wantError {
					if status.Code(err) != codes.InvalidArgument {
						t.Fatalf("got %v, want InvalidArgument", err)
					}
					if len(includes) != 0 || len(overlaps) != 0 {
						t.Fatal("invalid request returned partial filters")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if withMissing && len(overlaps) != 1 || !withMissing && len(includes) != 1 {
					t.Fatalf("explicit zero value lost: includes=%d, overlaps=%d", len(includes), len(overlaps))
				}
			})
		}
	}
}
