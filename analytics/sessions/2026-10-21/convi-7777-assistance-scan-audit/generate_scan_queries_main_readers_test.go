package analyticsimpl

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const scanAuditOutput = "/Users/xuanyu.wang/repos/knowledge/analytics/sessions/2026-10-21/convi-7777-assistance-scan-audit"

type scanAuditQuery struct {
	Name  string
	Mode  string
	Query string
	Args  []any
}
type ScanAuditSuite struct{ suite.Suite }

func TestAssistanceScanAudit(t *testing.T) { suite.Run(t, new(ScanAuditSuite)) }
func (s *ScanAuditSuite) TestGeneratedQueries() {
	// Given
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	timeRange := &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(24 * time.Hour))}
	tables := []ClickhouseTable{conversationTable, messageTable, conversationEventTable, eventConversationEventTable, actionAnnotationTable, momentAnnotationTable, eventAnalyticsEventTable}
	// When
	queries, err := s.generateQueries(timeRange, tables)
	// Then
	s.Require().NoError(err)
	s.Len(queries, 83)
	data, err := json.MarshalIndent(queries, "", "  ")
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(scanAuditOutput+"/generated-queries-main-readers.json", data, 0600))
}
func (s *ScanAuditSuite) generateQueries(timeRange *analyticspb.TimeRange, tables []ClickhouseTable) ([]scanAuditQuery, error) {
	var result []scanAuditQuery
	for _, mode := range []string{"none", "include", "exclude", "both", "daily", "weekly", "two-day-daily"} {
		attribute := &analyticspb.Attribute{}
		included := &momentpb.Moment{Name: "customers/c/profiles/p/moments/audit", Type: momentpb.Moment_CONVERSATION_METADATA}
		excluded := &momentpb.Moment{Name: "customers/c/profiles/p/moments/blocked", Type: momentpb.Moment_CONVERSATION_METADATA}
		if mode == "include" || mode == "both" {
			attribute.MomentGroups = append(attribute.MomentGroups, &analyticspb.MomentGroup{Moments: []*momentpb.Moment{included}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "yes"}}}}})
		}
		if mode == "exclude" || mode == "both" {
			attribute.MomentGroups = append(attribute.MomentGroups, &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{excluded}})
		}
		currentTimeRange := timeRange
		if mode == "two-day-daily" {
			currentTimeRange = &analyticspb.TimeRange{StartTimestamp: timeRange.StartTimestamp, EndTimestamp: timestamppb.New(timeRange.EndTimestamp.AsTime().Add(24 * time.Hour))}
		}
		conditions, moments, excludedMoments, _, err := parseClickhouseFilter(s.T().Context(), currentTimeRange, attribute, tables, false /* useExtTable */)
		if err != nil {
			return nil, err
		}
		frequency := analyticspb.Frequency_FREQUENCY_UNSPECIFIED
		if mode == "daily" || mode == "two-day-daily" {
			frequency = analyticspb.Frequency_DAILY
		}
		if mode == "weekly" {
			frequency = analyticspb.Frequency_WEEKLY
		}
		keys, groups, err := parseClickhouseGroupBy(&frequency, []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}, &analyticspb.Metadata{TimeZoneId: "UTC"}, tables)
		if err != nil {
			return nil, err
		}
		add := func(name, query string, args []any) { result = append(result, scanAuditQuery{name, mode, query, args}) }
		query, args := knowledgeBaseStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments)
		add("KB", query, args)
		query, args = suggestionStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments)
		add("Suggestions", query, args)
		query, args = noteTakingStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments, len(moments) > 0 || len(excludedMoments) > 0)
		add("Notes", query, args)
		query, args = guidedWorkflowStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments)
		add("Workflow", query, args)
		query, args = knowledgeAssistStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments)
		add("GenAI", query, args)
		query, args = summarizationStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments)
		add("Summary", query, args)
		query, args = hintStatsClickhouseQuery(conditions, keys, groups, false /* includeBot */, moments, excludedMoments, false /* useActionAnnotation */)
		add("HintsMoment", query, args)
		query, args = hintStatsClickhouseQuery(conditions, keys, groups, false /* includeBot */, moments, excludedMoments, true /* useActionAnnotation */)
		add("HintsAction", query, args)
		query, args = assistanceStatsClickhouseQuery(conditions, keys, groups, moments, excludedMoments)
		add("LegacyAssistance", query, args)
		query, args = adherencesClickhouseQuery(conditions, keys, groups, false /* includeBot */)
		add("Adherences", query, args)
		if mode == "none" || mode == "daily" || mode == "weekly" || mode == "two-day-daily" {
			query, args = smartComposeStatsClickhouseQuery(conditions, keys, groups)
			add("SmartCompose", query, args)
		}
	}
	for _, q := range conversationAssistanceUsageEvidenceQueries(false /* includeBot */) {
		result = append(result, scanAuditQuery{q.name, "new", q.query, q.args})
	}
	population, err := buildConversationPopulationQuery(s.T().Context(), conversationPopulationInputs{timeRange: timeRange})
	if err != nil {
		return nil, err
	}
	result = append(result, scanAuditQuery{"Population", "new", population.query, population.args})
	fractionalRange := &analyticspb.TimeRange{StartTimestamp: timestamppb.New(timeRange.StartTimestamp.AsTime().Add(500 * time.Millisecond)), EndTimestamp: timestamppb.New(timeRange.StartTimestamp.AsTime().Add(1500 * time.Millisecond))}
	fractionalConditions, err := buildTimeRangeConditionAndArgs(fractionalRange, []ClickhouseTable{conversationEventTable}, conversationStartTimeColumn)
	if err != nil {
		return nil, err
	}
	fractionalQuery, fractionalArgs := knowledgeBaseStatsClickhouseQuery(fractionalConditions, []string{agentUserIDColumn}, map[ClickhouseTable][]string{conversationEventTable: {agentUserIDColumn}}, nil, nil)
	result = append(result, scanAuditQuery{"KB", "fractional", fractionalQuery, fractionalArgs})
	return result, nil
}
