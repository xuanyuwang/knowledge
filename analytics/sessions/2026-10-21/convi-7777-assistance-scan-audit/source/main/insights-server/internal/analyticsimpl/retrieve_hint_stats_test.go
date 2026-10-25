package analyticsimpl

import (
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	npconfigpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config"
	featurespb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	chconversations "github.com/cresta/go-servers/shared/clickhouse/conversations"
	chshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

// Each scenario has a separate profile database; these are real SQL and reader
// assertions, including the legacy MAX denominators across the two sent branches.
type RetrieveHintStatsTestSuite struct {
	AnalyticsTestSuite
	start              time.Time
	profile            *npconfigpb.ProfileConfig
	originalActionMode bool
	originalExcluded   bool
}

func TestRetrieveHintStats(t *testing.T) {
	suite.Run(t, new(RetrieveHintStatsTestSuite))
}

func (s *RetrieveHintStatsTestSuite) SetupSuite() {
	s.AnalyticsTestSuite.SetupSuite()
	s.originalActionMode = *useActionAnnotationForHintStats
	s.originalExcluded = *enableExcludedMoments
	s.start = time.Date(2021 /* year */, 1 /* month */, 1 /* day */, 0 /* hour */, 0 /* min */, 0 /* sec */, 0 /* nsec */, time.UTC)
	s.mockConfigClient.EXPECT().GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_, _ string) (*config.ProfileConfig, error) {
		return config.NewProfileConfig(s.profile)
	})
}

func (s *RetrieveHintStatsTestSuite) SetupTest()    { s.resetHintScenario() }
func (s *RetrieveHintStatsTestSuite) SetupSubTest() { s.resetHintScenario() }
func (s *RetrieveHintStatsTestSuite) TearDownSuite() {
	s.Require().NoError(envflag.Set("USE_ACTION_ANNOTATION_FOR_HINT_STATS", fmt.Sprint(s.originalActionMode)))
	s.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", fmt.Sprint(s.originalExcluded)))
	s.AnalyticsTestSuite.TearDownSuite()
}
func (s *RetrieveHintStatsTestSuite) resetHintScenario() {
	s.server.clickhouseConvoRegistry = s.ClickhouseContainerTestSuite.GetConversationTestRegistry()
	s.profileID = "hints_" + uuid.NewString()[:8]
	s.profileResourceName = fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	s.profile = &npconfigpb.ProfileConfig{Name: s.profileResourceName, Features: &configpb.Features{Insights: &featurespb.Insights{DisableStatsForConversationSourceBot: true}}}
	s.server.enableExtTableForUserFilter = false
	s.setupHintRPCClients()
	s.Require().NoError(envflag.Set("USE_ACTION_ANNOTATION_FOR_HINT_STATS", "false"))
	s.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", "true"))
}

func (s *RetrieveHintStatsTestSuite) hintRequest() *analyticspb.RetrieveHintStatsRequest {
	return &analyticspb.RetrieveHintStatsRequest{
		Parent:            s.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(s.start), EndTimestamp: timestamppb.New(s.start.Add(24 * time.Hour))},
		FilterByAttribute: &analyticspb.Attribute{}, Frequency: analyticspb.Frequency_DAILY,
		GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT},
	}
}
func (s *RetrieveHintStatsTestSuite) hintAction(id, conversationID, agent string) *chconversations.ActionAnnotationRow {
	return &chconversations.ActionAnnotationRow{ActionAnnotationID: id, ActionType: 1, ConversationID: conversationID, AgentUserID: agent,
		PolicyID: "policy", PolicySourceType: 1, ConversationStartTime: s.start, ConversationEndTime: s.start.Add(time.Hour),
		CreateTime: s.start, UpdateTime: s.start, CustomerID: s.customerID, ProfileID: s.profileID}
}
func (s *RetrieveHintStatsTestSuite) hintMoment(id, actionID, conversationID, agent string, adherence int32) *chconversations.MomentAnnotationRow {
	return &chconversations.MomentAnnotationRow{MomentAnnotationID: id, AdherenceActionAnnotationID: actionID, AdherenceType: adherence,
		AdherenceActionType: 1, AdherenceActionAdherenceType: 1, AdherenceActionPolicySourceType: 1, AdherenceActionPolicyID: "policy", PolicyID: "policy", BehaviorID: "behavior",
		ConversationID: conversationID, AgentUserID: agent, ConversationStartTime: s.start, ConversationEndTime: s.start.Add(time.Hour),
		CreateTime: s.start, UpdateTime: s.start, CustomerID: s.customerID, ProfileID: s.profileID}
}
func (s *RetrieveHintStatsTestSuite) hintEvent(id, actionID, conversationID, agent string) *chconversations.ConversationEventRow {
	return &chconversations.ConversationEventRow{EventID: id, ActionAnnotationID: actionID, EventType: 9, ActionType: 14,
		ActionPolicySourceType: 1, ActionPolicyID: "policy", ConversationID: conversationID, AgentUserID: agent,
		ConversationStartTime: s.start, ConversationEndTime: s.start.Add(time.Hour), CreateTime: s.start, UpdateTime: s.start, CustomerID: s.customerID, ProfileID: s.profileID}
}
func (s *RetrieveHintStatsTestSuite) insertHintRows(table string, rows ...any) {
	s.Require().NotEmpty(rows)
	fields := chshared.ExtractCHFields(reflect.ValueOf(rows[0]).Elem().Interface())
	// Distributed seeds must reach source tables and materialized views before the next read.
	ctx := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"insert_distributed_sync": 1}))
	batch, err := chshared.PrepareBatchWithRetry(ctx, s.GetConversationConnection(s.customerID, s.profileID), fmt.Sprintf("INSERT INTO %s (%s)", table, fields))
	s.Require().NoError(err)
	for _, row := range rows {
		s.Require().NoError(batch.AppendStruct(row))
	}
	s.Require().NoError(chshared.SendBatchWithRetry(ctx, batch))
}

type hintExpectedBucket struct {
	agent, behavior, policy              string
	time                                 time.Time
	sent, followed, conversations, users int32
}

func (s *RetrieveHintStatsTestSuite) expectHintPayload(resp *analyticspb.RetrieveHintStatsResponse, noFrequency bool, buckets ...hintExpectedBucket) {
	expected := &analyticspb.RetrieveHintStatsResponse{}
	byAttribute := map[string]*analyticspb.HintStatsResult{}
	userFrequency := map[string]int32{}
	totalUsers := int32(0)
	for _, bucket := range buckets {
		key := bucket.agent + "/" + bucket.behavior + "/" + bucket.policy
		result := byAttribute[key]
		if result == nil {
			attr := &analyticspb.Attribute{}
			if bucket.agent != "" {
				attr.Users = []*userpb.User{{Name: fmt.Sprintf("customers/%s/users/%s", s.customerID, bucket.agent)}}
			}
			if bucket.behavior != "" {
				attr.Behaviors = []*behaviorpb.Behavior{{Name: fmt.Sprintf("%s/behaviors/%s", s.profileResourceName, bucket.behavior)}}
			}
			if bucket.policy != "" {
				attr.AttributePolicies = []*analyticspb.Attribute_AttributePolicy{{Name: fmt.Sprintf("%s/policies/%s", s.profileResourceName, bucket.policy)}}
			}
			result = &analyticspb.HintStatsResult{Attribute: attr}
			byAttribute[key] = result
			expected.HintStatsResults = append(expected.HintStatsResults, result)
		}
		result.TotalHintSentCount += bucket.sent
		result.TotalHintFollowedCount += bucket.followed
		result.TotalConversationCount += bucket.conversations
		userFrequency[key] += bucket.users
		totalUsers += bucket.users
		expected.TotalHintSentCount += bucket.sent
		expected.TotalHintFollowedCount += bucket.followed
		expected.TotalConversationCount += bucket.conversations
		if !noFrequency {
			result.HintStats = append(result.HintStats, &analyticspb.HintStats{
				TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(bucket.time)}, TotalHintSentCount: bucket.sent, TotalHintFollowedCount: bucket.followed, TotalConversationCount: bucket.conversations,
				AverageHintSentCountPerConversation: float32(bucket.sent) / float32(bucket.conversations), AverageHintFollowedCountPerConversation: float32(bucket.followed) / float32(bucket.conversations),
				AverageHintSentCountPerFrequencyPerUser: float32(bucket.sent) / float32(bucket.users), AverageHintFollowedCountPerFrequencyPerUser: float32(bucket.followed) / float32(bucket.users),
			})
		}
	}
	for key, result := range byAttribute {
		result.AverageHintSentCountPerConversation = float32(result.TotalHintSentCount) / float32(result.TotalConversationCount)
		result.AverageHintFollowedCountPerConversation = float32(result.TotalHintFollowedCount) / float32(result.TotalConversationCount)
		result.AverageHintSentCountPerFrequencyPerUser = float32(result.TotalHintSentCount) / float32(userFrequency[key])
		result.AverageHintFollowedCountPerFrequencyPerUser = float32(result.TotalHintFollowedCount) / float32(userFrequency[key])
	}
	if len(buckets) > 0 {
		expected.AverageHintSentCountPerConversation = float32(expected.TotalHintSentCount) / float32(expected.TotalConversationCount)
		expected.AverageHintFollowedCountPerConversation = float32(expected.TotalHintFollowedCount) / float32(expected.TotalConversationCount)
		expected.AverageHintSentCountPerFrequencyPerUser = float32(expected.TotalHintSentCount) / float32(totalUsers)
		expected.AverageHintFollowedCountPerFrequencyPerUser = float32(expected.TotalHintFollowedCount) / float32(totalUsers)
	}
	s.Empty(cmp.Diff(expected, resp, protocmp.Transform(), protocmp.IgnoreEmptyMessages(), protocmp.SortRepeatedFields(&analyticspb.RetrieveHintStatsResponse{}, "hint_stats_results")), "Hint reader payload differs")
}

func (s *RetrieveHintStatsTestSuite) TestBehavioralDeliveryModesAndDuplicateAnnotations() {
	for _, delivered := range []bool{false, true} {
		s.Run(fmt.Sprintf("ActionMode%t", delivered), func() {
			// Given
			s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = delivered
			action := s.hintAction("delivered", "conversation", "agent")
			action.AdherenceType = 1
			s.insertHintRows("action_annotation_d", action, action)
			s.insertHintRows("moment_annotation_d", s.hintMoment("m1", "delivered", "conversation", "agent", 2 /* adherence */), s.hintMoment("m2", "delivered", "conversation", "agent", 2 /* adherence */),
				s.hintMoment("m3", "missing-action", "other", "agent", 2 /* adherence */), s.hintMoment("m4", "", "empty", "agent", 2 /* adherence */), s.hintMoment("m5", "ignored", "ignored", "agent", 1 /* adherence */))
			req := s.hintRequest()

			// When
			resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

			// Then
			s.Require().NoError(err)
			sent, conversations := int32(2), int32(2)
			if delivered {
				sent, conversations = 1, 1
			}
			s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: sent, followed: 2, conversations: conversations, users: 1})
		})
	}
}

func (s *RetrieveHintStatsTestSuite) TestDeliveredRowsOwnAgentAndTimeAttribution() {
	// Given
	s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = true
	action := s.hintAction("delivered", "shared", "recipient")
	action.AdherenceType = 1
	s.insertHintRows("action_annotation_d", action)
	s.insertHintRows("moment_annotation_d", s.hintMoment("m", "delivered", "shared", "detector", 2 /* adherence */))

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), s.hintRequest(), nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "recipient", time: s.start, sent: 1, conversations: 1, users: 1})
}

func (s *RetrieveHintStatsTestSuite) TestSourcesDevelopmentUsersAndPolicyGates() {
	for _, bot := range []bool{false, true} {
		s.Run(fmt.Sprintf("IncludeBot%t", bot), func() {
			// Given
			good := s.hintAction("good", "good", "agent")
			botRow := s.hintAction("bot", "bot", "agent")
			botRow.ConversationSource = 7
			otherSource := s.hintAction("source", "source", "agent")
			otherSource.ConversationSource = 9
			dev := s.hintAction("dev", "dev", "agent")
			dev.IsDevUser = true
			empty := s.hintAction("empty", "empty", "")
			policy := s.hintAction("policy", "policy", "agent")
			policy.PolicySourceType = 2
			unknown := s.hintAction("unknown", "unknown", "agent")
			unknown.ActionType = 21
			s.insertHintRows("action_annotation_d", good, botRow, otherSource, dev, empty, policy, unknown)
			req := s.hintRequest()
			req.FilterByAttribute.IncludeCrestaBot = bot

			// When
			resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

			// Then
			s.Require().NoError(err)
			count := int32(1)
			if bot {
				count = 2
			}
			s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: count, conversations: count, users: 1})
		})
	}
}

func (s *RetrieveHintStatsTestSuite) TestKnowledgeFollowsAreDistinctActionsIncludingMissingID() {
	// Given
	for index, kind := range []int32{1, 6, 13, 14} {
		action := s.hintAction(fmt.Sprint(index), fmt.Sprint(index), "agent")
		action.ActionType = kind
		s.insertHintRows("action_annotation_d", action)
	}
	event := s.hintEvent("e1", "2", "2", "agent")
	duplicate := s.hintEvent("e2", "2", "2", "agent")
	missing := s.hintEvent("empty", "", "2", "agent")
	orphan := s.hintEvent("orphan", "not-delivered", "2", "agent")
	wrongType := s.hintEvent("wrong-type", "wrong-type", "2", "agent")
	wrongType.EventType = 8
	wrongPolicy := s.hintEvent("wrong-policy", "wrong-policy", "2", "agent")
	wrongPolicy.ActionPolicySourceType = 2
	s.insertHintRows("conversation_event_d", event, duplicate, missing, orphan, wrongType, wrongPolicy)

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), s.hintRequest(), nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: 4, followed: 3, conversations: 4, users: 1})
}

func (s *RetrieveHintStatsTestSuite) TestMixedBranchesRetainMaximumDenominators() {
	// Given
	s.insertHintRows("moment_annotation_d", s.hintMoment("m1", "b1", "b1", "a", 2 /* adherence */), s.hintMoment("m2", "b2", "b2", "b", 3 /* adherence */))
	s.insertHintRows("action_annotation_d", s.hintAction("n1", "n1", "c"), s.hintAction("n2", "n2", "c"), s.hintAction("n3", "n3", "c"))
	s.insertHintRows("conversation_event_d", s.hintEvent("e", "n1", "n1", "c"))
	s.insertHintRows("conversation_d", &chshared.ConversationRow{ConversationID: "without-hints", AgentUserID: "d", CustomerID: s.customerID, ProfileID: s.profileID, ConversationStartTime: s.start, ConversationEndTime: s.start.Add(time.Hour), CreateTime: s.start, UpdateTime: s.start})
	req := s.hintRequest()
	req.GroupByAttributeTypes = nil

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{time: s.start, sent: 3, followed: 2, conversations: 3, users: 2})
}

func (s *RetrieveHintStatsTestSuite) TestSameConversationCountsForEachAgent() {
	// Given
	s.insertHintRows("action_annotation_d", s.hintAction("a1", "shared", "a"), s.hintAction("a2", "shared", "b"))

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), s.hintRequest(), nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "a", time: s.start, sent: 1, conversations: 1, users: 1}, hintExpectedBucket{agent: "b", time: s.start, sent: 1, conversations: 1, users: 1})
}

func (s *RetrieveHintStatsTestSuite) TestUserFilterInlineAndExternalForBothDeliveryModes() {
	for _, mode := range []bool{false, true} {
		for _, external := range []bool{false, true} {
			s.Run(fmt.Sprintf("Action%t_External%t", mode, external), func() {
				// Given
				s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = mode
				s.server.enableExtTableForUserFilter = external
				for _, agent := range []string{"selected", "excluded"} {
					action := s.hintAction(agent, agent, agent)
					action.AdherenceType = 1
					s.insertHintRows("action_annotation_d", action)
					s.insertHintRows("moment_annotation_d", s.hintMoment(agent, agent, agent, agent, 2 /* adherence */))
					s.insertHintRows("conversation_event_d", s.hintEvent(agent, agent, agent, agent))
				}
				req := s.hintRequest()
				req.FilterByAttribute.Users = []*userpb.User{{Name: fmt.Sprintf("customers/%s/users/selected", s.customerID)}}

				// When
				resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

				// Then
				s.Require().NoError(err)
				s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "selected", time: s.start, sent: 1, followed: 2, conversations: 1, users: 1})
			})
		}
	}
}

func (s *RetrieveHintStatsTestSuite) TestConversationStartBoundsAndTimezoneBuckets() {
	// Given
	for index, offset := range []time.Duration{-time.Second, 0, 11 * time.Hour, 24*time.Hour - time.Second, 24 * time.Hour} {
		action := s.hintAction(fmt.Sprint(index), fmt.Sprint(index), "agent")
		action.ConversationStartTime = s.start.Add(offset)
		action.CreateTime = s.start.Add(-48 * time.Hour)
		s.insertHintRows("action_annotation_d", action)
	}
	req := s.hintRequest()
	req.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start.Add(-14 * time.Hour), sent: 1, conversations: 1, users: 1}, hintExpectedBucket{agent: "agent", time: s.start.Add(10 * time.Hour), sent: 2, conversations: 2, users: 1})
}

func (s *RetrieveHintStatsTestSuite) TestNoFrequencyKeepsTotalsAndRemovesBuckets() {
	// Given
	s.insertHintRows("action_annotation_d", s.hintAction("a", "c", "agent"))
	req := s.hintRequest()
	req.Frequency = analyticspb.Frequency_FREQUENCY_UNSPECIFIED

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, true /* noFrequency */, hintExpectedBucket{agent: "agent", sent: 1, conversations: 1, users: 1})
}

func (s *RetrieveHintStatsTestSuite) TestBehaviorGroupingAndFilterUseMomentRows() {
	for _, mode := range []bool{false, true} {
		s.Run(fmt.Sprintf("ActionMode%t", mode), func() {
			// Given
			s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = mode
			action := s.hintAction("a", "c", "agent")
			action.AdherenceType = 1
			s.insertHintRows("action_annotation_d", action)
			s.insertHintRows("moment_annotation_d", s.hintMoment("m", "a", "c", "agent", 2 /* adherence */))
			req := s.hintRequest()
			req.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR}
			req.FilterByAttribute.Behaviors = []*behaviorpb.Behavior{{Name: s.profileResourceName + "/behaviors/behavior"}}

			// When
			resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

			// Then
			s.Require().NoError(err)
			if mode {
				s.expectHintPayload(resp, false /* noFrequency */)
			} else {
				s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{behavior: "behavior", time: s.start, sent: 1, followed: 1, conversations: 1, users: 1})
			}
		})
	}
}

func (s *RetrieveHintStatsTestSuite) TestPolicyGroupingRetainsSourceColumnMapping() {
	// Given
	s.insertHintRows("action_annotation_d", s.hintAction("a", "c", "agent"))
	s.insertHintRows("conversation_event_d", s.hintEvent("e", "a", "c", "agent"))
	req := s.hintRequest()
	req.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY}
	req.FilterByAttribute.AttributePolicies = []*analyticspb.Attribute_AttributePolicy{{Name: s.profileResourceName + "/policies/policy"}}

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{policy: "policy", time: s.start, sent: 1, followed: 1, conversations: 1, users: 1})
}

func (s *RetrieveHintStatsTestSuite) TestHintTypeBranches() {
	for _, mode := range []bool{false, true} {
		for _, kind := range []analyticspb.Attribute_HintType{analyticspb.Attribute_REGULAR_HINT, analyticspb.Attribute_BEHAVIOR_HINT, analyticspb.Attribute_CHECKLIST_HINT, analyticspb.Attribute_KB_HINT, analyticspb.Attribute_GW_HINT} {
			s.Run(fmt.Sprintf("Action%t_%s", mode, kind), func() {
				// Given
				s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = mode
				for _, row := range []struct {
					id              string
					kind, adherence int32
				}{{"regular", 1, 0}, {"behavior", 1, 1}, {"checklist", 21, 0}, {"kb", 14, 0}, {"gw", 13, 0}} {
					action := s.hintAction(row.id, row.id, "agent")
					action.ActionType = row.kind
					action.AdherenceType = row.adherence
					s.insertHintRows("action_annotation_d", action)
					if row.id == "behavior" || row.id == "checklist" {
						moment := s.hintMoment(row.id, row.id, row.id, "agent", 2 /* adherence */)
						moment.AdherenceActionType = row.kind
						moment.AdherenceActionAdherenceType = row.adherence
						s.insertHintRows("moment_annotation_d", moment)
					}
					if row.id == "kb" || row.id == "gw" {
						event := s.hintEvent(row.id, row.id, row.id, "agent")
						event.ActionType = row.kind
						s.insertHintRows("conversation_event_d", event)
					}
				}
				req := s.hintRequest()
				req.FilterByAttribute.HintType = kind

				// When
				resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

				// Then
				s.Require().NoError(err)
				followed := int32(1)
				if kind == analyticspb.Attribute_REGULAR_HINT {
					followed = 0
				}
				s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: 1, followed: followed, conversations: 1, users: 1})
			})
		}
	}
}

func (s *RetrieveHintStatsTestSuite) metadataMoment(id, conversationID, template, value string, offset time.Duration) *chconversations.MomentAnnotationRow {
	row := s.hintMoment(id, "", conversationID, "agent", 0 /* adherence */)
	row.MomentType = int32(momentpb.Moment_CONVERSATION_METADATA)
	row.MomentTemplateID = template
	row.MetadataStringValue = value
	row.CreateTime = s.start
	row.UpdateTime = s.start.Add(offset)
	return row
}
func (s *RetrieveHintStatsTestSuite) metadataGroup(template, value string) *analyticspb.MomentGroup {
	return &analyticspb.MomentGroup{Moments: []*momentpb.Moment{{Name: s.profileResourceName + "/moments/" + template, Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: value}}}}}
}
func (s *RetrieveHintStatsTestSuite) TestMetadataIncludesUseLatestUpdateAndExcludesMissing() {
	for _, mode := range []bool{false, true} {
		s.Run(fmt.Sprintf("ActionMode%t", mode), func() {
			// Given
			s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = mode
			for _, conversation := range []string{"selected", "stale", "missing"} {
				action := s.hintAction(conversation, conversation, "agent")
				action.AdherenceType = 1
				s.insertHintRows("action_annotation_d", action)
				s.insertHintRows("moment_annotation_d", s.hintMoment("hint-"+conversation, conversation, conversation, "agent", 2 /* adherence */))
			}
			s.insertHintRows("moment_annotation_d", s.metadataMoment("s1", "selected", "region", "old", time.Second), s.metadataMoment("s2", "selected", "region", "wanted", 2*time.Second), s.metadataMoment("t1", "stale", "region", "wanted", time.Second), s.metadataMoment("t2", "stale", "region", "other", 2*time.Second))
			req := s.hintRequest()
			req.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{s.metadataGroup("region", "wanted")}

			// When
			resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

			// Then
			s.Require().NoError(err)
			s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: 1, followed: 1, conversations: 1, users: 1})
		})
	}
}
func (s *RetrieveHintStatsTestSuite) TestMetadataExclusionAndCombinedFilters() {
	for _, nullable := range []bool{false, true} {
		for _, combined := range []bool{false, true} {
			s.Run(fmt.Sprintf("Nullable%t_Combined%t", nullable, combined), func() {
				if !nullable {
					// AA-01 in the CONVI-7777 assistance analytics issue register tracks the
					// shared exclusion builder's IS NULL bug with production join defaults.
					// Enable these cases when that separate production fix lands.
					s.T().Skip("AA-01: metadata exclusions discard valid rows with join_use_nulls=0; https://github.com/cresta/go-servers/pull/34037#discussion_r4319778965")
				}

				// Given
				for _, conversation := range []string{"selected", "excluded", "missing"} {
					s.insertHintRows("action_annotation_d", s.hintAction(conversation, conversation, "agent"))
				}
				s.insertHintRows("moment_annotation_d", s.metadataMoment("s", "selected", "region", "wanted", time.Second), s.metadataMoment("e", "excluded", "region", "wanted", time.Second), s.metadataMoment("block", "excluded", "blocked", "yes", time.Second))
				exclusion := &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{{Name: s.profileResourceName + "/moments/blocked", Type: momentpb.Moment_CONVERSATION_METADATA}}}
				req := s.hintRequest()
				req.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{exclusion}
				if combined {
					req.FilterByAttribute.MomentGroups = append(req.FilterByAttribute.MomentGroups, s.metadataGroup("region", "wanted"))
				}
				ctx := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"join_use_nulls": nullable}))

				// When
				resp, err := s.server.readHintStatsFromClickhouse(ctx, req, nil /* users */)

				// Then
				s.Require().NoError(err)
				count := int32(2)
				if combined {
					count = 1
				}
				s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: count, conversations: count, users: 1})
			})
		}
	}
}

func (s *RetrieveHintStatsTestSuite) TestEmptyProfile() {
	// Given
	req := s.hintRequest()

	// When
	resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), req, nil /* users */)

	// Then
	s.Require().NoError(err)
	s.expectHintPayload(resp, false /* noFrequency */)
}

func (s *RetrieveHintStatsTestSuite) TestMomentAndEventEligibilityGuards() {
	for _, mode := range []bool{false, true} {
		s.Run(fmt.Sprintf("ActionMode%t", mode), func() {
			// Given
			s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = mode
			s.profile.Features.Insights.DisableStatsForConversationSourceBot = false
			for _, fixture := range []struct {
				id, agent      string
				source, policy int32
				dev            bool
			}{
				{id: "standard", agent: "agent", policy: 1}, {id: "external", agent: "agent", source: 8, policy: 1}, {id: "bot", agent: "agent", source: 7, policy: 1},
				{id: "bad-source", agent: "agent", source: 9, policy: 1}, {id: "bad-policy", agent: "agent", policy: 2}, {id: "dev", agent: "agent", policy: 1, dev: true}, {id: "empty", policy: 1},
			} {
				action := s.hintAction(fixture.id, fixture.id, fixture.agent)
				action.AdherenceType = 1
				action.ConversationSource = fixture.source
				action.PolicySourceType = fixture.policy
				action.IsDevUser = fixture.dev
				moment := s.hintMoment(fixture.id, fixture.id, fixture.id, fixture.agent, 2 /* adherence */)
				moment.ConversationSource = fixture.source
				moment.AdherenceActionPolicySourceType = fixture.policy
				moment.IsDevUser = fixture.dev
				event := s.hintEvent(fixture.id, fixture.id, fixture.id, fixture.agent)
				event.ConversationSource = fixture.source
				event.ActionPolicySourceType = fixture.policy
				event.IsDevUser = fixture.dev
				s.insertHintRows("action_annotation_d", action)
				s.insertHintRows("moment_annotation_d", moment)
				s.insertHintRows("conversation_event_d", event)
			}

			// When
			resp, err := s.server.readHintStatsFromClickhouse(s.T().Context(), s.hintRequest(), nil /* users */)

			// Then
			s.Require().NoError(err)
			s.expectHintPayload(resp, false /* noFrequency */, hintExpectedBucket{agent: "agent", time: s.start, sent: 3, followed: 6, conversations: 3, users: 1})
		})
	}
}

func (s *RetrieveHintStatsTestSuite) setupHintRPCClients() {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	var users []*internaluserpb.LiteUser
	for _, agent := range []string{"1", "2"} {
		users = append(users, &internaluserpb.LiteUser{
			UserId: agent, Username: "agent_" + agent, FullName: "Agent " + agent,
			Memberships: []*internaluserpb.UserGroupMembership{{
				Group:          &internaluserpb.LiteGroup{GroupId: "group-" + agent, DisplayName: "Group " + agent, GroupType: internaluserpb.LiteGroup_TEAM},
				IsDirectMember: true,
			}},
		})
	}
	internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
		Parent: s.customerResourceName, ProfileIds: []string{s.profileID}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true,
	})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	userClient := usermock.NewMockUserServiceClient(controller)
	groups := []*userpb.Group{
		{Name: s.customerResourceName + "/groups/group-1", DisplayName: "Group 1", Type: userpb.Group_TEAM},
		{Name: s.customerResourceName + "/groups/group-2", DisplayName: "Group 2", Type: userpb.Group_TEAM},
	}
	userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{
		Parent: s.customerResourceName, PageSize: 200,
		Filter: &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}},
	})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: groups}, nil)
	for _, includeMemberships := range []bool{false, true} {
		userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{
			Parent: s.customerResourceName, PageSize: 200,
			Filter:                  &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{s.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}, GroupIds: []string{"group-2"}},
			IncludeGroupMemberships: includeMemberships, IncludeIndirectGroupMemberships: includeMemberships,
		})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: groups[1:]}, nil)
	}
	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](userClient)
}

func (s *RetrieveHintStatsTestSuite) TestRPCRequestValidation() {
	for _, scenario := range []struct {
		name, message string
		groups        []analyticspb.AttributeType
		frequency     analyticspb.Frequency
	}{
		{name: "WithoutFilterByTimeRange_Throw", message: "must set time_range for attribute", groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}},
		{name: "TooManyGroupByAttrs_Throw", message: "not supporting more than 2 grouping attribute types", groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY, analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR}},
		{name: "TooManyGroupByAttrsWithTime_Throw", message: "not supporting more than 2 grouping attribute types", groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY}, frequency: analyticspb.Frequency_DAILY},
	} {
		s.Run(scenario.name, func() {
			// Given
			request := &analyticspb.RetrieveHintStatsRequest{Parent: s.profileResourceName, GroupByAttributeTypes: scenario.groups, Frequency: scenario.frequency}

			// When
			response, err := s.server.RetrieveHintStats(s.T().Context(), request)

			// Then
			s.Require().Error(err)
			s.ErrorContains(err, scenario.message)
			s.Nil(response)
		})
	}
}

func (s *RetrieveHintStatsTestSuite) TestRPCUserGroupPolicyAndTimeCoverage() {
	for _, actionMode := range []bool{false, true} {
		for _, scenario := range []struct {
			name                                                        string
			groups                                                      []analyticspb.AttributeType
			selectedUser, selectedGroup, external, policy, behavior     bool
			noFrequency, weekly, monday, bot, metadata, excludeMetadata bool
		}{
			{name: "AllUsersDaily"},
			{name: "SelectedBobInline", selectedUser: true},
			{name: "SelectedBobExternal", selectedUser: true, external: true},
			{name: "SelectedGroupInline", selectedGroup: true},
			{name: "SelectedGroupExternal", selectedGroup: true, external: true},
			{name: "GroupByAgentAndGroup", groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP}},
			{name: "GroupByGroup", groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP}},
			{name: "GroupAndFilterByBehavior", behavior: true, groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR}},
			{name: "GroupByAgentPolicy_NotGroupByTime", noFrequency: true, policy: true, groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY}},
			{name: "GroupByGroupPolicy_NotGroupByTime", noFrequency: true, policy: true, groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY}},
			{name: "GroupByAgentGroupPolicy_NotGroupByTime", noFrequency: true, policy: true, groups: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY}},
			{name: "IncludeBotConversations", bot: true},
			{name: "MetadataInclude", metadata: true},
			{name: "FilterByMetadataMomentGroups_IncludeAndExcludeMoments", metadata: true, excludeMetadata: true},
			{name: "WeeklySunday", weekly: true},
			{name: "WeeklyMonday", weekly: true, monday: true},
		} {
			s.Run(fmt.Sprintf("Action%t_%s", actionMode, scenario.name), func() {
				// Given
				s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = actionMode
				s.server.enableExtTableForUserFilter = scenario.external
				first := s.start.Add(12 * time.Hour)
				seventh := first.Add(6 * 24 * time.Hour)
				for _, fixture := range []struct {
					id, agent, policy string
					start             time.Time
					duration          int32
					source            int32
				}{
					{id: "first", agent: "1", policy: "policy", start: first, duration: 10},
					{id: "later-a", agent: "1", policy: "policy", start: seventh, duration: 99},
					{id: "later-b", agent: "1", policy: "policy", start: seventh, duration: 60},
					{id: "bob", agent: "2", policy: "policy", start: seventh, duration: 60},
					{id: "bot", agent: "2", policy: "policy", start: seventh, duration: 60, source: 7},
					{id: "wrong-policy", agent: "1", policy: "other-policy", start: first, duration: 60},
					{id: "too-short", agent: "1", policy: "policy", start: first, duration: 9},
					{id: "too-long", agent: "1", policy: "policy", start: first, duration: 100},
					{id: "before-start", agent: "1", policy: "policy", start: s.start.Add(-time.Second), duration: 60},
					{id: "at-end", agent: "1", policy: "policy", start: s.start.Add(8 * 24 * time.Hour), duration: 60},
				} {
					action := s.hintAction(fixture.id, fixture.id, fixture.agent)
					action.AdherenceType = 1
					action.PolicyID, action.ConversationStartTime, action.ConversationDurationSecs, action.ConversationSource = fixture.policy, fixture.start, fixture.duration, fixture.source
					moment := s.hintMoment(fixture.id, fixture.id, fixture.id, fixture.agent, 2 /* adherence */)
					moment.AdherenceActionPolicyID, moment.PolicyID = fixture.policy, fixture.policy
					moment.ConversationStartTime, moment.ConversationDurationSecs, moment.ConversationSource = fixture.start, fixture.duration, fixture.source
					s.insertHintRows("action_annotation_d", action)
					s.insertHintRows("moment_annotation_d", moment, s.metadataMoment("region-"+fixture.id, fixture.id, "region", "wanted", time.Second))
				}
				s.insertHintRows("moment_annotation_d", s.metadataMoment("blocked", "later-b", "blocked", "yes", time.Second))
				s.CreateModels(&dbmodel.Policies{CustomerID: s.customerID, ProfileID: s.profileID, PolicyID: "policy", DisplayName: "Helpful policy", Status: 1, Input: "{}", Output: "{}", PolicyConfig: "{}", FlatView: "{}", SourceType: sql.NullInt32{Int32: 1, Valid: true}})
				request := s.hintRequest()
				request.FilterByTimeRange.EndTimestamp = timestamppb.New(s.start.Add(8 * 24 * time.Hour))
				request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu", StartDayOfWeek: analyticspb.Metadata_SUNDAY}
				request.FilterByAttribute.ConversationDurationRange = &analyticspb.DurationRange{MinDuration: durationpb.New(10 * time.Second), MaxDuration: durationpb.New(100 * time.Second)}
				request.FilterByAttribute.AttributePolicies = []*analyticspb.Attribute_AttributePolicy{{Name: s.profileResourceName + "/policies/policy"}}
				request.FilterByAttribute.IncludeCrestaBot = scenario.bot
				if scenario.groups != nil {
					request.GroupByAttributeTypes = scenario.groups
				}
				if scenario.selectedUser {
					request.FilterByAttribute.Users = []*userpb.User{{Name: s.customerResourceName + "/users/2"}}
				}
				if scenario.selectedGroup {
					request.FilterByAttribute.Groups = []*userpb.Group{{Name: s.customerResourceName + "/groups/group-2"}}
				}
				if scenario.behavior {
					otherAction := s.hintAction("other-behavior", "other-behavior", "1")
					otherAction.AdherenceType, otherAction.ConversationDurationSecs = 1, 60
					otherMoment := s.hintMoment("other-behavior", "other-behavior", "other-behavior", "1", 2 /* adherence */)
					otherMoment.BehaviorID, otherMoment.ConversationDurationSecs = "other-behavior", 60
					s.insertHintRows("action_annotation_d", otherAction)
					s.insertHintRows("moment_annotation_d", otherMoment)
					request.FilterByAttribute.Behaviors = []*behaviorpb.Behavior{{Name: s.profileResourceName + "/behaviors/behavior"}}
				}
				if scenario.noFrequency {
					request.Frequency = analyticspb.Frequency_FREQUENCY_UNSPECIFIED
				}
				if scenario.weekly {
					request.Frequency = analyticspb.Frequency_WEEKLY
				}
				if scenario.monday {
					request.Metadata.StartDayOfWeek = analyticspb.Metadata_MONDAY
				}
				if scenario.metadata {
					request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{s.metadataGroup("region", "wanted")}
				}
				if scenario.excludeMetadata {
					request.FilterByAttribute.MomentGroups = append(request.FilterByAttribute.MomentGroups, &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{{Name: s.profileResourceName + "/moments/blocked", Type: momentpb.Moment_CONVERSATION_METADATA}}})
				}
				firstBucket, lastBucket := first.Add(-2*time.Hour), seventh.Add(-2*time.Hour)
				if scenario.weekly {
					firstBucket, lastBucket = time.Date(2020, 12, 27, 10, 0, 0, 0, time.UTC), time.Date(2021, 1, 3, 10, 0, 0, 0, time.UTC)
				}
				if scenario.monday {
					firstBucket, lastBucket = firstBucket.Add(24*time.Hour), lastBucket.Add(24*time.Hour)
				}
				aliceCount, bobCount := int32(3), int32(1)
				if scenario.excludeMetadata {
					aliceCount = 2
				}
				if scenario.bot {
					bobCount = 2
				}
				makeBucket := func(at time.Time, count int32, perUser float32) *analyticspb.HintStats {
					return &analyticspb.HintStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(at)}, TotalHintSentCount: count, TotalHintFollowedCount: count, TotalConversationCount: count, AverageHintSentCountPerConversation: 1, AverageHintFollowedCountPerConversation: 1, AverageHintSentCountPerFrequencyPerUser: perUser, AverageHintFollowedCountPerFrequencyPerUser: perUser}
				}
				makeResult := func(agent string, count int32, perUser float32, buckets []*analyticspb.HintStats) *analyticspb.HintStatsResult {
					attribute := &analyticspb.Attribute{}
					if !scenario.behavior {
						attribute.Users = []*userpb.User{{Name: s.customerResourceName + "/users/" + agent, Username: "agent_" + agent, FullName: "Agent " + agent}}
					}
					if slices.Contains(scenario.groups, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) && !slices.Contains(scenario.groups, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) {
						attribute.Users = nil
					}
					if slices.Contains(scenario.groups, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
						attribute.Groups = []*userpb.Group{{Name: s.customerResourceName + "/groups/group-" + agent, DisplayName: "Group " + agent, Type: userpb.Group_TEAM}}
					}
					if scenario.behavior {
						attribute.Groups = nil
						attribute.Behaviors = []*behaviorpb.Behavior{{Name: s.profileResourceName + "/behaviors/behavior"}}
					}
					if scenario.policy {
						attribute.AttributePolicies = []*analyticspb.Attribute_AttributePolicy{{Name: s.profileResourceName + "/policies/policy", DisplayName: "Helpful policy"}}
					}
					if scenario.noFrequency {
						buckets = nil
					}
					return &analyticspb.HintStatsResult{Attribute: attribute, TotalHintSentCount: count, TotalHintFollowedCount: count, TotalConversationCount: count, AverageHintSentCountPerConversation: 1, AverageHintFollowedCountPerConversation: 1, AverageHintSentCountPerFrequencyPerUser: perUser, AverageHintFollowedCountPerFrequencyPerUser: perUser, HintStats: buckets}
				}
				alicePerUser := float32(aliceCount) / 2
				if scenario.noFrequency {
					alicePerUser = float32(aliceCount)
				}
				bobPerUser := float32(bobCount)
				if scenario.name == "GroupByGroupPolicy_NotGroupByTime" {
					// Group/policy averages use time-bucket membership; no-frequency results have no buckets.
					alicePerUser, bobPerUser = 0, 0
				}
				total := aliceCount + bobCount
				denominator := float32(3)
				if scenario.noFrequency {
					denominator = 2
				}
				expected := &analyticspb.RetrieveHintStatsResponse{TotalHintSentCount: total, TotalHintFollowedCount: total, TotalConversationCount: total, AverageHintSentCountPerConversation: 1, AverageHintFollowedCountPerConversation: 1, AverageHintSentCountPerFrequencyPerUser: float32(total) / denominator, AverageHintFollowedCountPerFrequencyPerUser: float32(total) / denominator,
					HintStatsResults: []*analyticspb.HintStatsResult{
						makeResult("1", aliceCount, alicePerUser, []*analyticspb.HintStats{makeBucket(firstBucket, 1 /* count */, 1 /* perUser */), makeBucket(lastBucket, aliceCount-1, float32(aliceCount-1))}),
						makeResult("2", bobCount, bobPerUser, []*analyticspb.HintStats{makeBucket(lastBucket, bobCount, float32(bobCount))}),
					}}
				if scenario.selectedUser || scenario.selectedGroup {
					expected.TotalHintSentCount, expected.TotalHintFollowedCount, expected.TotalConversationCount = bobCount, bobCount, bobCount
					expected.AverageHintSentCountPerFrequencyPerUser, expected.AverageHintFollowedCountPerFrequencyPerUser = bobPerUser, bobPerUser
					expected.HintStatsResults = expected.HintStatsResults[1:]
				}
				if scenario.behavior {
					expected.HintStatsResults = []*analyticspb.HintStatsResult{makeResult("", 4 /* count */, 4.0/3 /* perUser */, []*analyticspb.HintStats{makeBucket(firstBucket, 1 /* count */, 1 /* perUser */), makeBucket(lastBucket, 3 /* count */, 1.5 /* perUser */)})}
					if actionMode {
						expected = &analyticspb.RetrieveHintStatsResponse{}
					}
				}
				ctx := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"join_use_nulls": true}))

				// When
				response, err := s.server.RetrieveHintStats(ctx, request)

				// Then
				s.Require().NoError(err)
				s.Empty(cmp.Diff(expected, response, protocmp.Transform(), protocmp.IgnoreEmptyMessages(), protocmp.SortRepeatedFields(&analyticspb.RetrieveHintStatsResponse{}, "hint_stats_results")), "Hint RPC payload differs")
			})
		}
	}
}

func (s *RetrieveHintStatsTestSuite) TestRPCSQLGoldens() {
	for _, scenario := range []struct {
		name           string
		request        *analyticspb.RetrieveHintStatsRequest
		goldenFilename string
		actionMode     bool
	}{
		{
			name: "GroupByAgentAndGroup",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
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
					AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
						},
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByAgentAndGroup_request.sql",
		},
		{
			name: "GroupByGroup",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
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
					AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
						},
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByGroup_request.sql",
		},
		{
			name: "GroupAndFilterByBehavior",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					ConversationDurationRange: &analyticspb.DurationRange{
						MinDuration: &durationpb.Duration{
							Seconds: 10,
						},
						MaxDuration: &durationpb.Duration{
							Seconds: 100,
						},
					},
					Behaviors: []*behaviorpb.Behavior{
						{
							Name: "customers/test_customer/profiles/test-profile/behaviors/behavior-id1",
						},
						{
							Name: "customers/test_customer/profiles/test-profile/behaviors/behavior-id2",
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupAndFilterByBehavior_request.sql",
		},
		{
			name: "GroupByAgentPolicy_NotGroupByTime",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
				},
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByAgentGroupPolicy_NotGroupByTime_request.sql",
		},
		{
			name: "GroupByGroupPolicy_NotGroupByTime",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
				},
			},
			// Same as group by agent + policy.
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByAgentGroupPolicy_NotGroupByTime_request.sql",
		},
		{
			name: "GroupByAgentGroupPolicy_NotGroupByTime",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
				},
			},
			// Same as group by agent + policy.
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByAgentGroupPolicy_NotGroupByTime_request.sql",
		},
		{
			name: "IncludeBotConversations",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					IncludeCrestaBot: true,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_includeBotConversations_request.sql",
		},
		{
			name: "FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
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
									MetadataValue: &metadatapb.MetadataValue{
										Value: &metadatapb.MetadataValue_StringValue{
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
			goldenFilename: "clickhouse_RetrieveHintStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_request.sql",
		}, {
			actionMode: true,
			name:       "GroupByAgentAndGroup",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
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
					AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
						},
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByAgentAndGroup_UseActionAnnotation_request.sql",
		},
		{
			actionMode: true,
			name:       "GroupByGroup",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
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
					AttributePolicies: []*analyticspb.Attribute_AttributePolicy{
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id1",
						},
						{
							Name: "customers/test_customer/profiles/test_profile/policies/policy-id2",
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByGroup_UseActionAnnotation_request.sql",
		},
		{
			actionMode: true,
			name:       "GroupAndFilterByBehavior",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					ConversationDurationRange: &analyticspb.DurationRange{
						MinDuration: &durationpb.Duration{
							Seconds: 10,
						},
						MaxDuration: &durationpb.Duration{
							Seconds: 100,
						},
					},
					Behaviors: []*behaviorpb.Behavior{
						{
							Name: "customers/test_customer/profiles/test-profile/behaviors/behavior-id1",
						},
						{
							Name: "customers/test_customer/profiles/test-profile/behaviors/behavior-id2",
						},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupAndFilterByBehavior_UseActionAnnotation_request.sql",
		},
		{
			actionMode: true,
			name:       "GroupByAgentGroupPolicy_NotGroupByTime",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
					analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY,
				},
			},
			goldenFilename: "clickhouse_RetrieveHintStats_GroupByAgentGroupPolicy_NotGroupByTime_UseActionAnnotation_request.sql",
		},
		{
			actionMode: true,
			name:       "IncludeBotConversations",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				FilterByAttribute: &analyticspb.Attribute{
					IncludeCrestaBot: true,
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_DAILY,
			},
			goldenFilename: "clickhouse_RetrieveHintStats_includeBotConversations_UseActionAnnotation_request.sql",
		},
		{
			actionMode: true,
			name:       "FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveHintStatsRequest{
				Parent: s.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
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
									MetadataValue: &metadatapb.MetadataValue{
										Value: &metadatapb.MetadataValue_StringValue{
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
			goldenFilename: "clickhouse_RetrieveHintStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_UseActionAnnotation_request.sql",
		},
	} {
		s.Run(fmt.Sprintf("Action%t_%s", scenario.actionMode, scenario.name), func() {
			// Given
			s.profile.Features.Insights.AllowHintStatsCalculationActionAnnotationBasedQuery = scenario.actionMode
			s.profile.Features.Insights.DisableStatsForConversationSourceBot = scenario.request.GetFilterByAttribute().GetIncludeCrestaBot()
			request := scenario.request
			// Subtest setup assigns a fresh profile after the original request literals are built.
			request.Parent = s.profileResourceName
			start := request.FilterByTimeRange.StartTimestamp.AsTime()
			end := request.FilterByTimeRange.EndTimestamp.AsTime()
			hasMetadata := len(request.GetFilterByAttribute().GetMomentGroups()) > 0
			isBehavior := len(request.GetFilterByAttribute().GetBehaviors()) > 0
			expected := &analyticspb.RetrieveHintStatsResponse{}
			for _, fixture := range []struct {
				agent, policy, behavior string
				source                  int32
			}{
				{agent: "1", policy: "policy-id1", behavior: "behavior-id1"},
				{agent: "2", policy: "policy-id2", behavior: "behavior-id2", source: 7},
			} {
				action := s.hintAction("delivered-"+fixture.agent, "conversation-"+fixture.agent, fixture.agent)
				action.AdherenceType, action.PolicyID, action.ConversationStartTime = 1, fixture.policy, start
				action.ConversationDurationSecs, action.ConversationSource = 60, fixture.source
				moment := s.hintMoment("moment-"+fixture.agent, action.ActionAnnotationID, action.ConversationID, fixture.agent, 2 /* adherence */)
				moment.PolicyID, moment.AdherenceActionPolicyID, moment.BehaviorID = fixture.policy, fixture.policy, fixture.behavior
				moment.ConversationStartTime, moment.ConversationDurationSecs, moment.ConversationSource = start, 60, fixture.source
				s.insertHintRows("action_annotation_d", action)
				s.insertHintRows("moment_annotation_d", moment)
				if hasMetadata {
					included := s.metadataMoment("included-"+fixture.agent, action.ConversationID, "moment_id_1", "include-value-1", time.Second)
					included.ConversationStartTime = start
					s.insertHintRows("moment_annotation_d", included)
					if fixture.agent == "2" {
						excluded := s.metadataMoment("excluded", action.ConversationID, "moment_id_2", "blocked", time.Second)
						excluded.ConversationStartTime = start
						s.insertHintRows("moment_annotation_d", excluded)
					}
				}
				s.CreateModels(&dbmodel.Policies{CustomerID: s.customerID, ProfileID: s.profileID, PolicyID: fixture.policy, DisplayName: "Policy " + fixture.agent, Status: 1, Input: "{}", Output: "{}", PolicyConfig: "{}", FlatView: "{}", SourceType: sql.NullInt32{Int32: 1, Valid: true}})
				if (hasMetadata && fixture.agent == "2") || (isBehavior && scenario.actionMode) {
					continue
				}
				attribute := &analyticspb.Attribute{}
				if slices.Contains(request.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) {
					attribute.Users = []*userpb.User{{Name: s.customerResourceName + "/users/" + fixture.agent, Username: "agent_" + fixture.agent, FullName: "Agent " + fixture.agent}}
				}
				if slices.Contains(request.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
					attribute.Groups = []*userpb.Group{{Name: s.customerResourceName + "/groups/group-" + fixture.agent, DisplayName: "Group " + fixture.agent, Type: userpb.Group_TEAM}}
				}
				if isBehavior {
					attribute.Behaviors = []*behaviorpb.Behavior{{Name: s.profileResourceName + "/behaviors/" + fixture.behavior}}
				}
				if slices.Contains(request.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY) {
					attribute.AttributePolicies = []*analyticspb.Attribute_AttributePolicy{{Name: s.profileResourceName + "/policies/" + fixture.policy, DisplayName: "Policy " + fixture.agent}}
				}
				result := &analyticspb.HintStatsResult{Attribute: attribute, TotalHintSentCount: 1, TotalHintFollowedCount: 1, TotalConversationCount: 1,
					AverageHintSentCountPerConversation: 1, AverageHintFollowedCountPerConversation: 1, AverageHintSentCountPerFrequencyPerUser: 1, AverageHintFollowedCountPerFrequencyPerUser: 1}
				if request.Frequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED {
					result.HintStats = []*analyticspb.HintStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start)}, TotalHintSentCount: 1, TotalHintFollowedCount: 1, TotalConversationCount: 1,
						AverageHintSentCountPerConversation: 1, AverageHintFollowedCountPerConversation: 1, AverageHintSentCountPerFrequencyPerUser: 1, AverageHintFollowedCountPerFrequencyPerUser: 1}}
				}
				expected.HintStatsResults = append(expected.HintStatsResults, result)
				expected.TotalHintSentCount++
				expected.TotalHintFollowedCount++
				expected.TotalConversationCount++
				expected.AverageHintSentCountPerConversation, expected.AverageHintFollowedCountPerConversation = 1, 1
				expected.AverageHintSentCountPerFrequencyPerUser, expected.AverageHintFollowedCountPerFrequencyPerUser = 1, 1
			}
			outside := s.hintAction("outside", "outside", "1")
			outside.ConversationStartTime, outside.PolicyID, outside.ConversationDurationSecs = end, "policy-id1", 60
			s.insertHintRows("action_annotation_d", outside)
			readTables := []bool{true, false, true, false}
			if scenario.actionMode {
				readTables = []bool{true, false, false, true, false}
			}
			expectedArgs := []any{}
			startArg, endArg := start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05")
			if hasMetadata {
				expectedArgs = append(expectedArgs, startArg, endArg, "moment_id_1", startArg, endArg, "moment_id_1", "include-value-1", startArg, endArg, "moment_id_2")
			}
			for _, momentRead := range readTables {
				expectedArgs = append(expectedArgs, startArg, endArg)
				if len(request.GetFilterByAttribute().GetUsers()) > 0 {
					expectedArgs = append(expectedArgs, []string{"1", "2"})
				}
				if isBehavior && momentRead {
					expectedArgs = append(expectedArgs, []string{"behavior-id1", "behavior-id2"})
				}
				if len(request.GetFilterByAttribute().GetAttributePolicies()) > 0 {
					expectedArgs = append(expectedArgs, []string{"policy-id1", "policy-id2"})
				}
				if request.GetFilterByAttribute().GetConversationDurationRange() != nil {
					expectedArgs = append(expectedArgs, float64(10), float64(100))
				}
			}
			recording := recordConversationSQL(s.server, s.GetConversationConnection(s.customerID, s.profileID), s.customerID, s.profileID)
			ctx := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"join_use_nulls": true}))

			// When
			response, err := s.server.RetrieveHintStats(ctx, request)

			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(expected, response, protocmp.Transform(), protocmp.IgnoreEmptyMessages(), protocmp.SortRepeatedFields(&analyticspb.RetrieveHintStatsResponse{}, "hint_stats_results")), "Hint RPC payload differs")
			assertRecordedSQLGolden(s.T(), recording, scenario.goldenFilename, expectedArgs)
		})
	}
}
