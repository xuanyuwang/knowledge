package analyticsimpl

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	workflowpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/workflowautomation"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowAutomationStatsSuite struct{ suite.Suite }

func TestWorkflowAutomationStatsSuite(t *testing.T) { suite.Run(t, new(WorkflowAutomationStatsSuite)) }

func (s *WorkflowAutomationStatsSuite) TestEmptyAndZeroUsageDenominators() {
	for _, testCase := range []struct {
		name    string
		row     workflowAutomationStatsRow
		present bool
	}{
		{name: "no conversations"},
		{name: "conversations without triggers", row: workflowAutomationStatsRow{conversationCount: 4, agentCount: 2, agentDayCount: 3}, present: true},
	} {
		s.Run(testCase.name, func() {
			// When
			stats := testCase.row.stats()

			// Then
			s.Equal(testCase.present, stats.WorkflowAutomationUsageRatio != nil)
			s.Equal(testCase.present, stats.AverageWorkflowAutomationTriggerCountPerAgent != nil)
			s.Equal(testCase.present, stats.AverageWorkflowAutomationTriggerCountPerActiveAgentDay != nil)
			s.Zero(stats.GetWorkflowAutomationUsageRatio())
			s.Len(stats.GetTriggerSourceStats(), 4)
		})
	}
}

func (s *WorkflowAutomationStatsSuite) TestPeriodAveragesHaveDistinctDenominators() {
	// Given
	row := workflowAutomationStatsRow{conversationCount: 8, conversationsWithTriggerCount: 3, agentCount: 2, agentDayCount: 5, sourceCounts: [4]uint64{1, 2, 3, 4}}

	// When
	stats := row.stats()

	// Then
	s.Equal(int64(10), stats.GetTotalWorkflowAutomationTriggerCount())
	s.Equal(3.0/8, stats.GetWorkflowAutomationUsageRatio())
	s.Equal(5.0, stats.GetAverageWorkflowAutomationTriggerCountPerAgent())
	s.Equal(2.0, stats.GetAverageWorkflowAutomationTriggerCountPerActiveAgentDay())
}

func (s *WorkflowAutomationStatsSuite) TestBucketsRespectDSTAndClipBoundaries() {
	// Given
	req := &analyticspb.RetrieveWorkflowAutomationStatsRequest{Frequency: analyticspb.Frequency_DAILY, Metadata: &analyticspb.Metadata{TimeZoneId: "America/Los_Angeles"}, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2026, 3, 7, 20, 0, 0, 0, time.UTC)), EndTimestamp: timestamppb.New(time.Date(2026, 3, 9, 16, 0, 0, 0, time.UTC))}}

	// When
	buckets, _, err := workflowAutomationBuckets(req)

	// Then
	s.Require().NoError(err)
	s.Require().Len(buckets, 3)
	s.Equal(req.GetFilterByTimeRange().GetStartTimestamp(), buckets[0].GetStartTimestamp())
	s.Equal(req.GetFilterByTimeRange().GetEndTimestamp(), buckets[2].GetEndTimestamp())
	s.Equal(23*time.Hour, buckets[1].GetEndTimestamp().AsTime().Sub(buckets[1].GetStartTimestamp().AsTime()))
}

func (s *WorkflowAutomationStatsSuite) TestDefinitionFiltersRejectOtherTenants() {
	for _, name := range []string{"customers/other/profiles/p/workflowAutomationDefinitions/d", "customers/c/profiles/other/workflowAutomationDefinitions/d", "invalid"} {
		s.Run(name, func() {
			// When
			_, err := parseWorkflowAutomationSelectors([]*analyticspb.WorkflowAutomationFilter{{WorkflowAutomationDefinition: name}}, "c", "p")

			// Then
			s.Equal(codes.InvalidArgument, status.Code(err))
		})
	}
}

func (s *WorkflowAutomationStatsSuite) TestDefinitionFiltersParseCanonicalNamesAndVersions() {
	// Given
	filter := &analyticspb.WorkflowAutomationFilter{
		WorkflowAutomationDefinition: workflowpb.WorkflowAutomationDefinitionName{CustomerID: "c", ProfileID: "p", WorkflowAutomationDefinitionID: "definition"}.String(),
		Versions:                     []int32{2, 7},
	}

	// When
	selectors, err := parseWorkflowAutomationSelectors([]*analyticspb.WorkflowAutomationFilter{filter}, "c", "p")

	// Then
	s.Require().NoError(err)
	s.Equal([]workflowAutomationSelector{{definitionID: "definition", versions: []int32{2, 7}}}, selectors)
}

func (s *WorkflowAutomationStatsSuite) TestOverlappingMembershipsAndEmptyGroups() {
	// Given
	user := &userpb.User{Name: "customers/c/users/u"}
	users := &ParseUserFilterResult{FinalUsers: []*userpb.User{user}, GroupsToAggregate: []*userpb.Group{{Name: "g1"}, {Name: "g2"}, {Name: "empty"}}, UserNameToGroupNamesMap: map[string][]string{user.GetName(): {"g1", "g2", "g1"}}}

	// When
	groups, memberships, err := workflowAutomationGroups(users, false /* byAgent */, true /* byGroup */)

	// Then
	s.Require().NoError(err)
	s.Len(groups, 3)
	s.Equal([]string{"g1", "g2"}, memberships["u"])
}

func (s *WorkflowAutomationStatsSuite) TestEmptyHintCountsRespectVersionAvailability() {
	for _, testCase := range []struct {
		name      string
		selectors []workflowAutomationSelector
		available bool
	}{
		{name: "Unfiltered", available: true},
		{name: "DefinitionOnly", selectors: []workflowAutomationSelector{{definitionID: "d"}}, available: true},
		{name: "VersionFiltered", selectors: []workflowAutomationSelector{{definitionID: "d", versions: []int32{7}}}},
		{name: "MixedSelectors", selectors: []workflowAutomationSelector{{definitionID: "d"}, {definitionID: "other", versions: []int32{3}}}},
	} {
		s.Run(testCase.name, func() {
			// Given
			groups := map[string]*analyticspb.WorkflowAutomationStatsResult{"agent": {}}
			buckets := []*analyticspb.TimeRange{{}}

			// When
			response := emptyWorkflowAutomationResponse(groups, buckets, workflowAutomationHintsAvailable(testCase.selectors))

			// Then
			s.Equal(testCase.available, response.GetAggregatedWorkflowAutomationStats().TotalIgnoredConversationCount != nil)
			s.Zero(response.GetAggregatedWorkflowAutomationStats().GetTotalIgnoredConversationCount())
			s.Require().Len(response.GetWorkflowAutomationStatsResults(), 1)
			group := response.GetWorkflowAutomationStatsResults()[0]
			s.Equal(testCase.available, group.GetAggregatedWorkflowAutomationStats().TotalIgnoredConversationCount != nil)
			s.Require().Len(group.GetWorkflowAutomationStats(), 1)
			s.Equal(testCase.available, group.GetWorkflowAutomationStats()[0].GetAggregatedWorkflowAutomationStats().TotalIgnoredConversationCount != nil)
		})
	}
}
