package analyticsimpl

import (
	"fmt"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadata "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"strings"
	"testing"
	"time"
)

func review32440Group(id string, values []*analyticspb.MetadataValueAttribute, included, excluded bool) *analyticspb.MomentGroup {
	m := &momentpb.Moment{Name: "customers/c/profiles/p/moments/" + id, Type: momentpb.Moment_CONVERSATION_METADATA}
	g := &analyticspb.MomentGroup{MetadataValueAttributes: values}
	if included {
		g.Moments = []*momentpb.Moment{m}
	}
	if excluded {
		g.ExcludedMoments = []*momentpb.Moment{m}
	}
	return g
}
func review32440String(v string) []*analyticspb.MetadataValueAttribute {
	return []*analyticspb.MetadataValueAttribute{{MetadataValue: &metadata.MetadataValue{Value: &metadata.MetadataValue_StringValue{StringValue: v}}}}
}
func review32440TimeRange() *analyticspb.TimeRange {
	return &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), EndTimestamp: timestamppb.New(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))}
}
func TestReview32440ExportSQL(t *testing.T) {
	// Given
	orig := *enableExcludedMoments
	*enableExcludedMoments = true
	t.Cleanup(func() { *enableExcludedMoments = orig })
	vals := map[string][]*analyticspb.MetadataValueAttribute{
		"String":             review32440String("X"),
		"EmptyString":        review32440String(""),
		"MultiValue":         append(review32440String("X"), review32440String("Y")...),
		"BoolTrue":           {{MetadataValue: &metadata.MetadataValue{Value: &metadata.MetadataValue_BooleanValue{BooleanValue: true}}}},
		"BoolFalse":          {{MetadataValue: &metadata.MetadataValue{Value: &metadata.MetadataValue_BooleanValue{BooleanValue: false}}}},
		"Zero":               {{MetadataValue: &metadata.MetadataValue{Value: &metadata.MetadataValue_NumberValue{NumberValue: 0}}}},
		"Singleton":          {{NumericBin: &metadata.NumericBin{FromValue: 5, ToValue: 5, ToValueIsInclusive: true}}},
		"ExclusiveInclusive": {{NumericBin: &metadata.NumericBin{FromValue: 5, ToValue: 10, FromValueIsExclusive: true, ToValueIsInclusive: true}}},
	}
	scenarios := map[string][]*analyticspb.MomentGroup{}
	for name, v := range vals {
		scenarios[name] = []*analyticspb.MomentGroup{review32440Group("A", v, true, true)}
	}
	scenarios["MultipleGroups"] = []*analyticspb.MomentGroup{review32440Group("A", review32440String("X"), true, true), review32440Group("B", review32440String("X"), true, true)}
	scenarios["MixedGroups"] = []*analyticspb.MomentGroup{review32440Group("A", review32440String("X"), true, true), review32440Group("B", review32440String("X"), true, false), review32440Group("C", nil, false, true)}
	scenarios["IncludeOnly"] = []*analyticspb.MomentGroup{review32440Group("A", review32440String("X"), true, false)}
	scenarios["MissingOnly"] = []*analyticspb.MomentGroup{review32440Group("A", nil, false, true)}
	for name, groups := range scenarios {
		for _, ended := range []bool{false, true} {
			for _, stale := range []bool{false, true} {
				for _, grouped := range []bool{false, true} {
					name := fmt.Sprintf("%s_End%v_Stale%v_Group%v", name, ended, stale, grouped)
					t.Run(name, func(t *testing.T) {
						// Given
						attr := &analyticspb.Attribute{MomentGroups: append(append([]*analyticspb.MomentGroup{}, groups...), &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{{Name: "customers/c/profiles/p/moments/vm", Type: momentpb.Moment_VOICE_MAIL}}})}
						before := proto.Clone(attr)
						field := analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_STARTED_AT
						if ended {
							field = analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT
						}
						col := getConversationTimeRangeColumn(getConversationQueryOptions(WithTimeRangeType(field)))
						freq := analyticspb.Frequency_FREQUENCY_UNSPECIFIED
						var groupAttrs []analyticspb.AttributeType
						if grouped {
							freq = analyticspb.Frequency_DAILY
							groupAttrs = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
						}
						// When
						remaining, overlap, err := parseConversationStatsValueOrMissingFilters(review32440TimeRange(), attr, col)
						if err != nil {
							t.Fatal(err)
						}
						cond, inc, exc, _, err := parseClickhouseFilter(t.Context(), review32440TimeRange(), remaining, []ClickhouseTable{messageTable}, false, WithTimeRangeType(field))
						if err != nil {
							t.Fatal(err)
						}
						keys, groupQueries, err := parseClickhouseGroupBy(&freq, groupAttrs, &analyticspb.Metadata{TimeZoneId: "UTC"}, []ClickhouseTable{messageTable}, WithTimeRangeType(field))
						if err != nil {
							t.Fatal(err)
						}
						convCond, err := buildTimeRangeConditionAndArgs(review32440TimeRange(), []ClickhouseTable{conversationLabelTable, conversationTable}, col)
						if err != nil {
							t.Fatal(err)
						}
						aa, aaArgs := buildAgentAssistFiltersSubQuery(nil, convCond, "conversation_source IN (0, 8)", col)
						query, args := (AnalyticsServiceImpl{}).conversationStatsClickhouseQueryWithMoment(cond, inc, exc, overlap, keys, groupQueries, aa, aaArgs, "conversation_source IN (0, 8)", stale)
						// Then
						if !proto.Equal(before, attr) {
							t.Fatal("input mutated")
						}
						if strings.Count(query, "?") != len(args) {
							t.Fatalf("placeholder mismatch %d != %d", strings.Count(query, "?"), len(args))
						}
						if err := os.WriteFile("/tmp/pr32440-review/"+name+".sql", []byte(combineQueryAndArgs(query, args)), 0600); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
func TestReview32440RejectShapes(t *testing.T) {
	// Given
	orig := *enableExcludedMoments
	*enableExcludedMoments = true
	t.Cleanup(func() { *enableExcludedMoments = orig })
	for _, name := range []string{"MultiInclude", "MultiExclude", "BothMulti", "IncludeOmittedType", "ExcludeOmittedType", "EmptyValue", "NilValue", "NoValues", "InvalidName"} {
		t.Run(name, func(t *testing.T) {
			// Given
			g := review32440Group("A", review32440String("X"), true, true)
			g.ExcludedMoments = []*momentpb.Moment{proto.Clone(g.Moments[0]).(*momentpb.Moment)}
			b := review32440Group("B", nil, true, false).Moments[0]
			switch name {
			case "MultiInclude":
				g.Moments = append(g.Moments, b)
			case "MultiExclude":
				g.ExcludedMoments = append(g.ExcludedMoments, b)
			case "BothMulti":
				g.Moments = append(g.Moments, b)
				g.ExcludedMoments = append(g.ExcludedMoments, b)
			case "IncludeOmittedType":
				g.Moments[0].Type = 0
			case "ExcludeOmittedType":
				g.ExcludedMoments[0].Type = 0
			case "EmptyValue":
				g.MetadataValueAttributes = []*analyticspb.MetadataValueAttribute{{MetadataValue: &metadata.MetadataValue{}}}
			case "NilValue":
				g.MetadataValueAttributes = []*analyticspb.MetadataValueAttribute{nil}
			case "NoValues":
				g.MetadataValueAttributes = nil
			case "InvalidName":
				g.Moments[0].Name = "invalid"
			}
			// When
			remaining, _, err := parseConversationStatsValueOrMissingFilters(review32440TimeRange(), &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{g}}, conversationStartTimeColumn)
			if err == nil {
				_, _, _, _, err = parseClickhouseFilter(t.Context(), review32440TimeRange(), remaining, []ClickhouseTable{messageTable}, false)
			}
			// Then
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("want InvalidArgument, got %v", err)
			}
		})
	}
}
