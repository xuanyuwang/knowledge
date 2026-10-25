package analyticsimpl

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RetrieveGuidedWorkflowStatsTestSuite struct{ notesWorkflowStatsSuite }

func TestRetrieveGuidedWorkflowStats(t *testing.T) {
	suite.Run(t, new(RetrieveGuidedWorkflowStatsTestSuite))
}

func (r *RetrieveGuidedWorkflowStatsTestSuite) Test_InvalidRequestThrows() {
	// When
	_, err := r.server.RetrieveGuidedWorkflowStats(r.T().Context(), &analyticspb.RetrieveGuidedWorkflowStatsRequest{})

	// Then
	r.Equal(codes.InvalidArgument, status.Code(err), "`filter_by_time_range` must be set.")
}
func (r *RetrieveGuidedWorkflowStatsTestSuite) TestCases() {
	for _, tc := range []struct {
		desc    string
		request *analyticspb.RetrieveGuidedWorkflowStatsRequest
	}{
		{
			desc: "Test_DailyFilterAndGroupByAgent",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{{
						Name: userpb.UserName{
							CustomerID: r.customerID,
							UserID:     "1",
						}.String(),
					}},
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
		},
		{
			desc: "Test_BasicFilterAndGroupByAgent",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/1",
						},
						{
							Name: "customers/customer_id/users/2",
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
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
		},
		{
			desc: "Test_DailyFilterAndGroupByGroup",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.group1,
						r.group2,
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
		},
		{
			desc: "Test_WeeklyGroupByAgent",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
			},
		},
		{
			desc: "Test_DailyGroupByGuidedWorkflow",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janTenthProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GUIDED_WORKFLOW,
				},
			},
		},
		{
			desc: "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstProto,
					EndTimestamp:   &r.janEleventhProto,
				},
				FilterByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/test_customer/profiles/test_profile/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "include-value-1",
										},
									},
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/test_customer/profiles/test_profile/moments/moment_id_2",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
		},
	} {
		r.Run(tc.desc, func() {
			// Given
			tc.request.Parent = r.profileResourceName
			metadata := tc.desc == "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments"
			if metadata {
				tc.request.FilterByAttribute = r.metadataFilter(true /* exclude */)
			}
			r.seedRPCEvents(true /* workflow */, false /* responseFixture */, metadata)
			want := &analyticspb.RetrieveGuidedWorkflowStatsResponse{}
			if !metadata {
				agents := []string{"1", "2"}
				if tc.desc == "Test_DailyFilterAndGroupByAgent" {
					agents = agents[:1]
				}
				want = r.expected(int32(len(agents)), int32(len(agents)), int32(len(agents)))
				want.GuidedWorkflowStatsResults = nil
				for _, agent := range agents {
					result := r.expected(1, 1, 1).GuidedWorkflowStatsResults[0]
					result.Attribute.Users = []*userpb.User{{Name: "customers/" + r.customerID + "/users/" + agent, Username: "agent_" + map[string]string{"1": "a", "2": "b"}[agent], FullName: "agent " + map[string]string{"1": "a", "2": "b"}[agent]}}
					bucket := r.janFirst
					if tc.desc == "Test_WeeklyGroupByAgent" {
						bucket = time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
					}
					if tc.desc == "Test_DailyGroupByGuidedWorkflow" {
						result.Attribute = &analyticspb.Attribute{GuidedWorkflowNames: []string{"gw" + agent}}
					}
					if tc.desc == "Test_DailyFilterAndGroupByGroup" {
						group := r.group1
						if agent == "2" {
							group = r.group2
						}
						result.Attribute = &analyticspb.Attribute{Groups: []*userpb.Group{group}}
					}
					result.GuidedWorkflowStats[0].TimeRange.StartTimestamp = timestamppb.New(bucket)
					want.GuidedWorkflowStatsResults = append(want.GuidedWorkflowStatsResults, result)
				}
			}

			// When
			response, err := r.server.RetrieveGuidedWorkflowStats(r.T().Context(), tc.request)

			// Then
			r.Require().NoError(err)
			r.equalResponse(want, response)
		})
	}
}
func (r *RetrieveGuidedWorkflowStatsTestSuite) Test_BasicRequestAndResponse() {
	// Given
	r.seedRPCEvents(true /* workflow */, true /* responseFixture */, false /* metadata */)

	// When
	response, err := r.server.RetrieveGuidedWorkflowStats(r.T().Context(), &analyticspb.RetrieveGuidedWorkflowStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janTenthProto,
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata: &analyticspb.Metadata{
			TimeZoneId: r.losAngeles,
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
	})

	// Then
	r.Require().NoError(err, "RetrieveGuidedWorkflowStats()")
	expectedResponse := &analyticspb.RetrieveGuidedWorkflowStatsResponse{
		GuidedWorkflowStatsResults: []*analyticspb.GuidedWorkflowStatsResult{
			{
				GuidedWorkflowStats: []*analyticspb.GuidedWorkflowStat{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalGuidedWorkflowClickCount:                      2,
						TotalConversationCount:                             1,
						AverageGuidedWorkflowClickCountPerConversation:     2.0 / 1,
						AverageGuidedWorkflowClickCountPerFrequencyPerUser: 2.0 / 1,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name:     "customers/customer_id/users/1",
							Username: "agent_a",
							FullName: "agent a",
						},
					},
				},
				TotalGuidedWorkflowClickCount:                      2,
				TotalConversationCount:                             1,
				AverageGuidedWorkflowClickCountPerConversation:     2.0 / 1,
				AverageGuidedWorkflowClickCountPerFrequencyPerUser: 2.0 / 1,
			},
			{
				GuidedWorkflowStats: []*analyticspb.GuidedWorkflowStat{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalGuidedWorkflowClickCount:                      1,
						TotalConversationCount:                             1,
						AverageGuidedWorkflowClickCountPerConversation:     1,
						AverageGuidedWorkflowClickCountPerFrequencyPerUser: 1.0,
					},
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janSeventhProto,
						},
						TotalGuidedWorkflowClickCount:                      1,
						TotalConversationCount:                             2,
						AverageGuidedWorkflowClickCountPerConversation:     1.0 / 2,
						AverageGuidedWorkflowClickCountPerFrequencyPerUser: 1.0,
					},
				},
				Attribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name:     "customers/customer_id/users/2",
							Username: "agent_b",
							FullName: "agent b",
						},
					},
				},
				TotalGuidedWorkflowClickCount:                      2,
				TotalConversationCount:                             3,
				AverageGuidedWorkflowClickCountPerConversation:     2.0 / 3,
				AverageGuidedWorkflowClickCountPerFrequencyPerUser: 1,
			},
		},
		TotalGuidedWorkflowClickCount:                      4,
		TotalConversationCount:                             4,
		AverageGuidedWorkflowClickCountPerConversation:     4.0 / 4,
		AverageGuidedWorkflowClickCountPerFrequencyPerUser: 4.0 / 3,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveGuidedWorkflowStatsResponse{}, "guided_workflow_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func (r *RetrieveGuidedWorkflowStatsTestSuite) Test_BasicRequestAndResponse_GroupByGuidedWorkflowName() {
	// Given
	r.seedRPCEvents(true /* workflow */, true /* responseFixture */, false /* metadata */)

	// When
	response, err := r.server.RetrieveGuidedWorkflowStats(r.T().Context(), &analyticspb.RetrieveGuidedWorkflowStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstProto,
			EndTimestamp:   &r.janTenthProto,
		},
		Frequency: analyticspb.Frequency_DAILY,
		Metadata: &analyticspb.Metadata{
			TimeZoneId: r.losAngeles,
		},
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_GUIDED_WORKFLOW,
		},
	})

	// Then
	r.Require().NoError(err, "RetrieveGuidedWorkflowStats()")
	expectedResponse := &analyticspb.RetrieveGuidedWorkflowStatsResponse{
		GuidedWorkflowStatsResults: []*analyticspb.GuidedWorkflowStatsResult{
			{
				GuidedWorkflowStats: []*analyticspb.GuidedWorkflowStat{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalGuidedWorkflowClickCount:                      2,
						TotalConversationCount:                             1,
						AverageGuidedWorkflowClickCountPerConversation:     2.0 / 1,
						AverageGuidedWorkflowClickCountPerFrequencyPerUser: 2.0 / 1,
					},
				},
				Attribute: &analyticspb.Attribute{
					GuidedWorkflowNames: []string{"gw1"},
				},
				TotalGuidedWorkflowClickCount:                      2,
				TotalConversationCount:                             1,
				AverageGuidedWorkflowClickCountPerConversation:     2.0 / 1,
				AverageGuidedWorkflowClickCountPerFrequencyPerUser: 2.0 / 1,
			},
			{
				GuidedWorkflowStats: []*analyticspb.GuidedWorkflowStat{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janFirstProto,
						},
						TotalGuidedWorkflowClickCount:                      1,
						TotalConversationCount:                             1,
						AverageGuidedWorkflowClickCountPerConversation:     1,
						AverageGuidedWorkflowClickCountPerFrequencyPerUser: 1.0,
					},
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.janSeventhProto,
						},
						TotalGuidedWorkflowClickCount:                      1,
						TotalConversationCount:                             2,
						AverageGuidedWorkflowClickCountPerConversation:     1.0 / 2,
						AverageGuidedWorkflowClickCountPerFrequencyPerUser: 1.0,
					},
				},
				Attribute: &analyticspb.Attribute{
					GuidedWorkflowNames: []string{"gw2"},
				},
				TotalGuidedWorkflowClickCount:                      2,
				TotalConversationCount:                             3,
				AverageGuidedWorkflowClickCountPerConversation:     2.0 / 3,
				AverageGuidedWorkflowClickCountPerFrequencyPerUser: 1,
			},
		},
		TotalGuidedWorkflowClickCount:                      4,
		TotalConversationCount:                             4,
		AverageGuidedWorkflowClickCountPerConversation:     4.0 / 4,
		AverageGuidedWorkflowClickCountPerFrequencyPerUser: 4.0 / 3,
	}
	diff := cmp.Diff(
		expectedResponse, response,
		protocmp.Transform(), protocmp.IgnoreEmptyMessages(),
		protocmp.SortRepeatedFields(&analyticspb.RetrieveGuidedWorkflowStatsResponse{}, "guided_workflow_stats_results"))
	r.Empty(diff, "Proto mismatch", "(-want, +got):\n%s", diff)
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) request() *analyticspb.RetrieveGuidedWorkflowStatsRequest {
	return &analyticspb.RetrieveGuidedWorkflowStatsRequest{Parent: s.profileResourceName, FilterByTimeRange: s.timeRange()}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) expected(used, conversations, users int32) *analyticspb.RetrieveGuidedWorkflowStatsResponse {
	averageConversation, averageUser := float32(0), float32(0)
	if conversations > 0 {
		averageConversation = float32(used) / float32(conversations)
	}
	if users > 0 {
		averageUser = float32(used) / float32(users)
	}
	return &analyticspb.RetrieveGuidedWorkflowStatsResponse{
		TotalGuidedWorkflowClickCount: used, TotalConversationCount: conversations, AverageGuidedWorkflowClickCountPerConversation: averageConversation, AverageGuidedWorkflowClickCountPerFrequencyPerUser: averageUser,
		GuidedWorkflowStatsResults: []*analyticspb.GuidedWorkflowStatsResult{{
			Attribute:                     &analyticspb.Attribute{},
			TotalGuidedWorkflowClickCount: used, TotalConversationCount: conversations, AverageGuidedWorkflowClickCountPerConversation: averageConversation, AverageGuidedWorkflowClickCountPerFrequencyPerUser: averageUser,
			GuidedWorkflowStats: []*analyticspb.GuidedWorkflowStat{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Time{})}, TotalGuidedWorkflowClickCount: used, TotalConversationCount: conversations, AverageGuidedWorkflowClickCountPerConversation: averageConversation, AverageGuidedWorkflowClickCountPerFrequencyPerUser: averageUser}},
		}},
	}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) equalResponse(want, got *analyticspb.RetrieveGuidedWorkflowStatsResponse) {
	s.Empty(cmp.Diff(want, got, protocmp.Transform(), protocmp.IgnoreEmptyMessages(), protocmp.SortRepeatedFields(&analyticspb.RetrieveGuidedWorkflowStatsResponse{}, "guided_workflow_stats_results")))
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestDocumentPayloadGates() {
	for _, scenario := range []struct {
		name, payload   string
		eventType, want int32
	}{
		{"ValidSession", guidedWorkflowUsagePayload("workflow", "2", "session"), 4, 1},
		{"EmptySessionCountsOnce", guidedWorkflowUsagePayload("workflow", "2", ""), 4, 1},
		{"MissingSessionCountsOnce", guidedWorkflowUsagePayload("workflow", "2", nil), 4, 1},
		{"NumericSessionRemainsCountable", guidedWorkflowUsagePayload("workflow", "2", 42), 4, 1},
		{"WhitespaceSessionCounts", guidedWorkflowUsagePayload("workflow", "2", " "), 4, 1},
		{"EmptyDocument", guidedWorkflowUsagePayload("", "2", "session"), 4, 0},
		{"WrongDocumentType", guidedWorkflowUsagePayload("workflow", "1", "session"), 4, 0},
		{"MissingPayload", `{}`, 4, 0},
		{"WrongEventType", guidedWorkflowUsagePayload("workflow", "2", "session"), 5, 0},
		{"NumericDocumentType", `{"usedKbDocument":{"document":"workflow","docType":2,"sessionMetadata":{"sessionId":"session"}}}`, 4, 1},
	} {
		s.Run(scenario.name, func() {
			// Given
			s.conversation(false /* metadata */, "one", "1", s.start)
			s.event(false /* metadata */, notesWorkflowEvent{conversationID: "one", agentID: "1", eventID: "event", eventType: scenario.eventType, payload: scenario.payload})
			request := s.request()
			want := s.expected(scenario.want, 1, 1)

			// When
			got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

			// Then
			s.Require().NoError(err)
			s.equalResponse(want, got)
		})
	}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestDistinctSessionsAcrossEventsDocumentsAndConversations() {
	// Given
	for _, id := range []string{"one", "two", "shared", "no-events"} {
		s.conversation(false /* metadata */, id, "1", s.start)
	}
	for _, event := range []notesWorkflowEvent{
		{conversationID: "one", agentID: "1", eventID: "first", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", "same")},
		{conversationID: "one", agentID: "1", eventID: "first", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", "same")},
		{conversationID: "two", agentID: "1", eventID: "second", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-b", "2", "same")},
		{conversationID: "shared", agentID: "2", eventID: "third", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", "other")},
		{conversationID: "two", agentID: "3", eventID: "unrelated", eventType: 1, payload: guidedWorkflowUsagePayload("workflow-a", "2", "unrelated")},
		{conversationID: "one", agentID: "1", eventID: "empty", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", "")},
		{conversationID: "two", agentID: "1", eventID: "missing", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", nil)},
		{conversationID: "orphan", agentID: "1", eventID: "orphan", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", "orphan")},
		{conversationID: "one", agentID: "", eventID: "empty-agent", eventType: 4, payload: guidedWorkflowUsagePayload("workflow-a", "2", "empty-agent")},
	} {
		s.event(false /* metadata */, event)
	}
	request := s.request()
	want := s.expected(3, 3, 3)

	// When
	got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

	// Then
	s.Require().NoError(err)
	s.equalResponse(want, got)
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestMetadataTypedDocumentGatesAndEmptySession() {
	// Given
	s.conversation(true /* metadata */, "one", "1", s.start)
	s.metadata("one", "include", "selected", s.start)
	for _, event := range []notesWorkflowEvent{
		{conversationID: "one", agentID: "1", eventID: "valid", eventType: 4, document: "workflow", docType: 2, sessionID: "session"},
		{conversationID: "one", agentID: "1", eventID: "duplicate-session", eventType: 4, document: "other-workflow", docType: 2, sessionID: "session"},
		{conversationID: "one", agentID: "1", eventID: "empty-session", eventType: 4, document: "workflow", docType: 2},
		{conversationID: "one", agentID: "1", eventID: "missing-doc", eventType: 4, docType: 2, sessionID: "missing-doc"},
		{conversationID: "one", agentID: "1", eventID: "wrong-type", eventType: 4, document: "workflow", docType: 1, sessionID: "wrong-type"},
		{conversationID: "one", agentID: "1", eventID: "wrong-event", eventType: 5, document: "workflow", docType: 2, sessionID: "wrong-event"},
	} {
		s.event(true /* metadata */, event)
	}
	request := s.request()
	request.FilterByAttribute = s.metadataFilter(false /* exclude */)
	want := s.expected(2, 1, 1)

	// When
	got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

	// Then
	s.Require().NoError(err)
	s.equalResponse(want, got)
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestInlineAndExternalUserFilters() {
	for _, external := range []bool{false, true} {
		name := "Inline"
		if external {
			name = "External"
		}
		s.Run(name, func() {
			// Given
			s.server.enableExtTableForUserFilter = external
			for _, agentID := range []string{"1", "2"} {
				s.conversation(false /* metadata */, agentID, agentID, s.start)
				s.event(false /* metadata */, notesWorkflowEvent{conversationID: agentID, agentID: agentID, eventID: agentID, eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session"), document: "workflow", docType: 2, sessionID: "session"})
			}
			request := s.request()
			request.FilterByAttribute = s.userFilter("1")
			want := s.expected(1, 1, 1)

			// When
			got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

			// Then
			s.Require().NoError(err)
			s.equalResponse(want, got)
		})
	}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestMetadataLatestIncludeAndExclude() {
	for _, scenario := range []struct {
		name    string
		exclude bool
	}{{"LatestInclude", false}, {"IncludeAndExclude", true}, {"ExcludeOnly", true}} {
		s.Run(scenario.name, func() {
			// Given
			for _, id := range []string{"latest-selected", "latest-other", "excluded", "missing"} {
				s.conversation(true /* metadata */, id, "1", s.start)
				s.event(true /* metadata */, notesWorkflowEvent{conversationID: id, agentID: "1", eventID: id, eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session"), document: "workflow", docType: 2, sessionID: "session-" + id})
			}
			s.metadata("latest-selected", "include", "other", s.start)
			s.metadata("latest-selected", "include", "selected", s.start.Add(time.Second))
			s.metadata("latest-other", "include", "selected", s.start)
			s.metadata("latest-other", "include", "other", s.start.Add(time.Second))
			s.metadata("excluded", "include", "selected", s.start)
			s.metadata("excluded", "exclude", "present", s.start)
			request := s.request()
			request.FilterByAttribute = s.metadataFilter(scenario.exclude)
			if scenario.name == "ExcludeOnly" {
				request.FilterByAttribute.MomentGroups = request.FilterByAttribute.MomentGroups[1:]
			}
			used := int32(2)
			if scenario.exclude {
				used = 0
			}
			want := s.expected(used, used, 1)

			// When
			got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

			// Then
			s.Require().NoError(err)
			s.equalResponse(want, got)
		})
	}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestNoMatchingDataAndNoEventConversation() {
	for _, grouped := range []bool{false, true} {
		name := "Ungrouped"
		if grouped {
			name = "Grouped"
		}
		s.Run(name, func() {
			// Given
			s.conversation(false /* metadata */, "no-events", "1", s.start)
			request := s.request()
			want := s.expected(0, 0, 0)
			if grouped {
				request.Frequency = analyticspb.Frequency_DAILY
				want = &analyticspb.RetrieveGuidedWorkflowStatsResponse{}
			}

			// When
			got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

			// Then
			s.Require().NoError(err)
			s.equalResponse(want, got)
		})
	}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestTimeBoundsAndTimezoneGrouping() {
	// Given
	for i, start := range []time.Time{s.start.Add(-time.Second), s.start, s.start.Add(12 * time.Hour), s.start.Add(48 * time.Hour)} {
		id := []string{"before", "start", "second-bucket", "end"}[i]
		s.conversation(false /* metadata */, id, "1", start)
		s.event(false /* metadata */, notesWorkflowEvent{conversationID: id, agentID: "1", eventID: id, start: start, eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session"), document: "workflow", docType: 2, sessionID: "session"})
	}
	request := s.request()
	request.Frequency = analyticspb.Frequency_DAILY
	request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}
	request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
	users := []*userpb.User{{Name: "customers/" + s.customerID + "/users/1", Username: "agent", FullName: "Agent One"}}
	want := s.expected(2, 2, 2)
	result := want.GuidedWorkflowStatsResults[0]
	result.Attribute.Users = users
	first := result.GuidedWorkflowStats[0]
	first.TotalGuidedWorkflowClickCount = 1
	first.TotalConversationCount = 1
	first.AverageGuidedWorkflowClickCountPerConversation = 1
	first.AverageGuidedWorkflowClickCountPerFrequencyPerUser = 1
	first.TimeRange.StartTimestamp = timestamppb.New(s.start.Add(-14 * time.Hour))
	second := *first
	second.TimeRange = &analyticspb.TimeRange{StartTimestamp: timestamppb.New(s.start.Add(10 * time.Hour))}
	result.GuidedWorkflowStats = append(result.GuidedWorkflowStats, &second)

	// When
	got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, users)

	// Then
	s.Require().NoError(err)
	s.equalResponse(want, got)
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestSameConversationMultipleAgentsGroupedAndOverall() {
	for _, grouped := range []bool{false, true} {
		name := "Overall"
		if grouped {
			name = "PerAgent"
		}
		s.Run(name, func() {
			// Given
			s.conversation(false /* metadata */, "shared", "1", s.start)
			s.conversation(false /* metadata */, "shared", "2", s.start)
			for _, agentID := range []string{"1", "2"} {
				s.event(false /* metadata */, notesWorkflowEvent{conversationID: "shared", agentID: agentID, eventID: "shared-event", eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "shared-session")})
			}
			request := s.request()
			want := s.expected(1, 1, 2)
			if grouped {
				request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
				want = s.expected(2, 2, 2)
				want.GuidedWorkflowStatsResults = nil
				for _, agentID := range []string{"1", "2"} {
					result := s.expected(1, 1, 1).GuidedWorkflowStatsResults[0]
					result.Attribute.Users = []*userpb.User{{Name: "customers/" + s.customerID + "/users/" + agentID}}
					want.GuidedWorkflowStatsResults = append(want.GuidedWorkflowStatsResults, result)
				}
			}

			// When
			got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

			// Then
			s.Require().NoError(err)
			s.equalResponse(want, got)
		})
	}
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestGroupingByDocumentCountsSessionWithinEachDocument() {
	// Given
	s.conversation(false /* metadata */, "one", "1", s.start)
	for _, document := range []string{"workflow-a", "workflow-b"} {
		s.event(false /* metadata */, notesWorkflowEvent{conversationID: "one", agentID: "1", eventID: document, eventType: 4, payload: guidedWorkflowUsagePayload(document, "2", "shared-session")})
	}
	request := s.request()
	request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_GUIDED_WORKFLOW}
	want := s.expected(2, 2, 2)
	want.GuidedWorkflowStatsResults = nil
	for _, document := range []string{"workflow-a", "workflow-b"} {
		result := s.expected(1, 1, 1).GuidedWorkflowStatsResults[0]
		result.Attribute.GuidedWorkflowNames = []string{document}
		want.GuidedWorkflowStatsResults = append(want.GuidedWorkflowStatsResults, result)
	}

	// When
	got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

	// Then
	s.Require().NoError(err)
	s.equalResponse(want, got)
}

func (s *RetrieveGuidedWorkflowStatsTestSuite) TestConversationAndEventTimeFiltersApplyBeforeJoin() {
	// Given
	s.conversation(false /* metadata */, "conversation-outside", "1", s.start.Add(-time.Hour))
	s.conversation(false /* metadata */, "event-outside", "1", s.start)
	s.conversation(false /* metadata */, "both-inside", "1", s.start)
	s.conversation(false /* metadata */, "empty-conversation-agent", "", s.start)
	for _, event := range []notesWorkflowEvent{
		{conversationID: "conversation-outside", agentID: "1", eventID: "conversation-outside", start: s.start, eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session")},
		{conversationID: "event-outside", agentID: "1", eventID: "event-outside", start: s.start.Add(48 * time.Hour), eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session")},
		{conversationID: "both-inside", agentID: "1", eventID: "inside", start: s.start, eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session")},
		{conversationID: "empty-conversation-agent", agentID: "1", eventID: "empty-agent", start: s.start, eventType: 4, payload: guidedWorkflowUsagePayload("workflow", "2", "session")},
	} {
		s.event(false /* metadata */, event)
	}
	request := s.request()
	want := s.expected(1, 1, 1)

	// When
	got, err := s.server.readGuidedWorkflowStatsFromClickhouse(s.T().Context(), request, nil)

	// Then
	s.Require().NoError(err)
	s.equalResponse(want, got)
}

func (r *RetrieveGuidedWorkflowStatsTestSuite) TestOriginalSQLGoldenCases() {
	goldenStart := timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	goldenEnd := timestamppb.New(time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC))
	goldenMetadataEnd := timestamppb.New(time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC))
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveGuidedWorkflowStatsRequest
		chReqPath string
	}{
		{
			desc: "Test_DailyFilterAndGroupByAgent",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: goldenStart,
					EndTimestamp:   goldenEnd,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{{
						Name: userpb.UserName{
							CustomerID: r.customerID,
							UserID:     "1",
						}.String(),
					}},
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveGuidedWorkflowStats_FilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_BasicFilterAndGroupByAgent",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: goldenStart,
					EndTimestamp:   goldenEnd,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name: "customers/customer_id/users/1",
						},
						{
							Name: "customers/customer_id/users/2",
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
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveGuidedWorkflowStats_BasicFilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_DailyFilterAndGroupByGroup",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: goldenStart,
					EndTimestamp:   goldenEnd,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						r.group1,
						r.group2,
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			chReqPath: "clickhouse_RetrieveGuidedWorkflowStats_FilterAndGroupByGroups_request.sql",
		},
		{
			desc: "Test_WeeklyGroupByAgent",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: goldenStart,
					EndTimestamp:   goldenEnd,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
			},
			chReqPath: "clickhouse_RetrieveGuidedWorkflowStats_WeeklyGroupByAgent_request.sql",
		},
		{
			desc: "Test_DailyGroupByGuidedWorkflow",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: goldenStart,
					EndTimestamp:   goldenEnd,
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GUIDED_WORKFLOW,
				},
			},
			chReqPath: "clickhouse_RetrieveGuidedWorkflowStats_DailyGroupByGuidedWorkflow_request.sql",
		},
		{
			desc: "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveGuidedWorkflowStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: goldenStart,
					EndTimestamp:   goldenMetadataEnd,
				},
				FilterByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/test_customer/profiles/test_profile/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "include-value-1",
										},
									},
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/test_customer/profiles/test_profile/moments/moment_id_2",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			chReqPath: "clickhouse_RetrieveGuidedWorkflowStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			// Given
			tc.request.Parent = r.profileResourceName
			metadata := tc.desc == "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments"
			grouped := tc.desc == "Test_DailyFilterAndGroupByGroup"
			recording := r.seedSQLGolden(true /* workflow */, grouped, metadata)
			expectedArgs := notesWorkflowGoldenArgs(tc.desc)
			want := &analyticspb.RetrieveGuidedWorkflowStatsResponse{}
			if !metadata {
				agents := []string{"1", "2"}
				if tc.desc == "Test_DailyFilterAndGroupByAgent" {
					agents = agents[:1]
				}
				total := int32(len(agents))
				if grouped {
					total = 4
				}
				want = r.expected(total, total, total)
				want.GuidedWorkflowStatsResults = nil
				for _, agent := range agents {
					count := int32(1)
					if grouped && agent == "2" {
						count = 3
					}
					result := r.expected(count, count, count).GuidedWorkflowStatsResults[0]
					result.Attribute = &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + r.customerID + "/users/" + agent, Username: "agent_" + map[string]string{"1": "a", "2": "b"}[agent], FullName: "agent " + map[string]string{"1": "a", "2": "b"}[agent]}}}
					if grouped {
						group := r.group1
						if agent == "2" {
							group = r.group2
						}
						result.Attribute = &analyticspb.Attribute{Groups: []*userpb.Group{group}}
					}
					if tc.desc == "Test_DailyGroupByGuidedWorkflow" {
						result.Attribute = &analyticspb.Attribute{GuidedWorkflowNames: []string{"gw" + agent}}
					}
					bucket := r.janFirst
					if tc.desc == "Test_WeeklyGroupByAgent" {
						bucket = time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
					}
					result.GuidedWorkflowStats[0].TimeRange.StartTimestamp = timestamppb.New(bucket)
					want.GuidedWorkflowStatsResults = append(want.GuidedWorkflowStatsResults, result)
				}
			}

			// When
			response, err := r.server.RetrieveGuidedWorkflowStats(r.T().Context(), tc.request)

			// Then
			r.Require().NoError(err)
			r.equalResponse(want, response)
			assertRecordedSQLGolden(r.T(), recording, tc.chReqPath, expectedArgs)
		})
	}
}
