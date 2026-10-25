package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ConversationPopulationCHTestSuite struct {
	suite.Suite
	chtesting.ClickhouseContainerTestSuite
}

func (s *ConversationPopulationCHTestSuite) SetupSuite() {
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
}
func (s *ConversationPopulationCHTestSuite) TearDownSuite() {
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
}

func (s *ConversationPopulationCHTestSuite) TestEligiblePairsAndMatchingHandleTime() {
	for _, externalUsers := range []bool{false, true} {
		s.Run(fmt.Sprintf("ExternalUsers_%t", externalUsers), func() {
			// Given
			conn := s.GetConversationConnection("population", uuid.NewString())
			seedContext := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"insert_distributed_sync": 1}))
			start := time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
			end := start.Add(48 * time.Hour)
			for _, pair := range [][2]string{{"shared", "alice"}, {"shared", "bob"}, {"shared", "charlie"}, {"no-messages", "alice"}, {"upper-bound", "alice"}} {
				conversationTime := start
				if pair[0] == "upper-bound" {
					conversationTime = end
				}
				s.Require().NoError(conn.Exec(seedContext, `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, conversation_end_time, update_time) VALUES (?, ?, ?, ?, ?)`, pair[0], pair[1], conversationTime, conversationTime.Add(time.Hour), start))
			}
			// The latest conversation row is outside the interval; the earlier matching row must survive.
			s.Require().NoError(conn.Exec(seedContext, `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, conversation_end_time, update_time) VALUES ('shared', 'alice', ?, ?, ?)`, end, end, end))
			for _, message := range []struct {
				conversation, agent string
				aht                 int
				voicemail           int
			}{{"shared", "alice", 10, 0}, {"shared", "alice", 20, 0}, {"shared", "alice", 90, 1}, {"shared", "bob", 15, 0}, {"shared", "charlie", 25, 0}, {"message-only", "alice", 30, 0}, {"upper-bound", "alice", 40, 0}} {
				s.Require().NoError(conn.Exec(seedContext, `INSERT INTO message (message_id, conversation_id, agent_user_id, conversation_start_time, conversation_end_time, handle_time_secs, is_voice_mail) VALUES (?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), message.conversation, message.agent, start, end, message.aht, message.voicemail))
			}
			inputs := conversationPopulationInputs{
				timeRange:            &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(end)},
				attribute:            &analyticspb.Attribute{ConversationCriterions: []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}, Users: []*userpb.User{{Name: "customers/population/users/alice"}, {Name: "customers/population/users/bob"}}},
				useExternalUserTable: externalUsers,
			}

			// When
			population, err := buildConversationPopulationQuery(s.T().Context(), inputs)
			var got []struct {
				Conversation string    `ch:"conversation_id"`
				Agent        string    `ch:"agent_user_id"`
				Time         time.Time `ch:"conversation_time"`
				AHT          int32     `ch:"aht_sec"`
			}
			if err == nil {
				err = conn.Select(attachExtTablesToContext(s.T().Context(), population.externalTables...), &got, population.query, population.args...)
			}
			var aggregate []struct {
				Conversations uint64 `ch:"convo_count"`
				Users         uint64 `ch:"user_count"`
				HandleTime    int64  `ch:"aht_sec"`
			}
			if err == nil {
				query := conversationStatsPopulationAggregate(population.query, nil /* groupByKeys */, nil /* groupByQueries */, conversationStartTimeColumn)
				err = conn.Select(attachExtTablesToContext(s.T().Context(), population.externalTables...), &aggregate, query, population.args...)
			}

			// Then
			s.Require().NoError(err)
			s.Require().Len(got, 2)
			byAgent := map[string]int32{}
			for _, row := range got {
				s.Equal("shared", row.Conversation)
				s.Equal(start, row.Time)
				byAgent[row.Agent] = row.AHT
			}
			s.Equal(map[string]int32{"alice": 20, "bob": 15}, byAgent)
			s.Require().Len(aggregate, 1)
			s.Equal(uint64(1), aggregate[0].Conversations)
			s.Equal(uint64(2), aggregate[0].Users)
			s.Equal(int64(35), aggregate[0].HandleTime)
			if externalUsers {
				s.Require().Len(population.externalTables, 1)
			}
		})
	}
}

func (s *ConversationPopulationCHTestSuite) TestMetadataEligibilityIsConversationScoped() {
	for _, allowStale := range []bool{false, true} {
		s.Run(fmt.Sprintf("AllowStale_%t", allowStale), func() {
			// Given
			conn := s.GetConversationConnection("population", uuid.NewString())
			seedContext := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"insert_distributed_sync": 1}))
			start := time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
			for _, agent := range []string{"alice", "bob"} {
				s.Require().NoError(conn.Exec(seedContext, `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, update_time) VALUES ('shared', ?, ?, ?)`, agent, start, start))
				s.Require().NoError(conn.Exec(seedContext, `INSERT INTO message (message_id, conversation_id, agent_user_id, conversation_start_time, handle_time_secs) VALUES (?, 'shared', ?, ?, 20)`, uuid.NewString(), agent, start))
			}
			for i, value := range []string{"wanted", "other"} {
				s.Require().NoError(conn.Exec(seedContext, `INSERT INTO moment_annotation (moment_annotation_id, conversation_id, agent_user_id, conversation_start_time, moment_template_id, moment_type, metadata_string_value, create_time) VALUES (?, 'shared', 'alice', ?, 'team', ?, ?, ?)`, uuid.NewString(), start, int32(momentpb.Moment_CONVERSATION_METADATA), value, start.Add(time.Duration(i)*time.Hour)))
			}
			inputs := conversationPopulationInputs{
				timeRange:                        &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(24 * time.Hour))},
				attribute:                        &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{{Moments: []*momentpb.Moment{{Name: "customers/population/profiles/profile/moments/team", Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "wanted"}}}}}}},
				allowMatchingStaleMetadataValues: allowStale,
			}

			// When
			population, err := buildConversationPopulationQuery(s.T().Context(), inputs)
			var agents []struct {
				Agent string `ch:"agent_user_id"`
			}
			if err == nil {
				err = conn.Select(s.T().Context(), &agents, "SELECT agent_user_id FROM ("+population.query+") ORDER BY agent_user_id", population.args...)
			}

			// Then
			s.Require().NoError(err)
			if allowStale {
				s.Require().Len(agents, 2)
				s.Equal("alice", agents[0].Agent)
				s.Equal("bob", agents[1].Agent)
			} else {
				s.Empty(agents)
			}
		})
	}
}

func TestConversationPopulationCH(t *testing.T) { suite.Run(t, new(ConversationPopulationCHTestSuite)) }

func (s *ConversationPopulationCHTestSuite) TestAgentAssistAndSourcePolicies() {
	// Given
	conn := s.GetConversationConnection("population", uuid.NewString())
	seedContext := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"insert_distributed_sync": 1}))
	start := time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	for _, fixture := range []struct {
		id                                                             string
		conversationSource, messageSource, conversationDev, messageDev int
		assisted                                                       bool
	}{
		{id: "unassisted"},
		{id: "assisted", assisted: true},
		{id: "bot", conversationSource: 7, messageSource: 7, assisted: true},
		{id: "conversation-dev", conversationDev: 1, assisted: true},
		{id: "conversation-source", conversationSource: 9, assisted: true},
		{id: "message-dev", messageDev: 1, assisted: true},
		{id: "message-source", messageSource: 9, assisted: true},
	} {
		s.Require().NoError(conn.Exec(seedContext, `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, conversation_source, is_dev_user, update_time) VALUES (?, 'alice', ?, ?, ?, ?)`, fixture.id, start, fixture.conversationSource, fixture.conversationDev, start))
		s.Require().NoError(conn.Exec(seedContext, `INSERT INTO message (message_id, conversation_id, agent_user_id, conversation_start_time, conversation_source, is_dev_user) VALUES (?, ?, 'alice', ?, ?, ?)`, uuid.NewString(), fixture.id, start, fixture.messageSource, fixture.messageDev))
		if fixture.assisted {
			for _, assisted := range []bool{false, true} {
				s.Require().NoError(conn.Exec(seedContext, `INSERT INTO conversation_with_labels (conversation_id, agent_user_id, conversation_start_time, has_agent_assistance, message_id, update_time) VALUES (?, 'alice', ?, ?, ?, ?)`, fixture.id, start, assisted, uuid.NewString(), start))
			}
		}
	}
	for _, scenario := range []struct {
		name     string
		state    analyticspb.Attribute_AgentAssistFilter_AgentAssistStateType
		bots     bool
		expected []string
	}{
		{name: "Default", expected: []string{"assisted", "unassisted"}},
		{name: "DefaultWithBots", bots: true, expected: []string{"assisted", "bot", "unassisted"}},
		{name: "On", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, expected: []string{"assisted", "conversation-dev", "conversation-source"}},
		{name: "OnWithBots", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, bots: true, expected: []string{"assisted", "bot", "conversation-dev", "conversation-source"}},
		{name: "Off", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, expected: []string{"unassisted"}},
	} {
		s.Run(scenario.name, func() {
			// Given
			inputs := conversationPopulationInputs{timeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(time.Hour))}, includeBotConversations: scenario.bots}
			if scenario.state != analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_UNSPECIFIED {
				inputs.attribute = &analyticspb.Attribute{AgentAssistFilters: []*analyticspb.Attribute_AgentAssistFilter{{AgentAssistUsed: scenario.state}}}
			}

			// When
			population, err := buildConversationPopulationQuery(s.T().Context(), inputs)
			var got []struct {
				Conversation string `ch:"conversation_id"`
			}
			if err == nil {
				err = conn.Select(s.T().Context(), &got, "SELECT conversation_id FROM ("+population.query+") ORDER BY conversation_id", population.args...)
			}

			// Then
			s.Require().NoError(err)
			expected := make([]struct {
				Conversation string `ch:"conversation_id"`
			}, len(scenario.expected))
			for i, id := range scenario.expected {
				expected[i].Conversation = id
			}
			s.Equal(expected, got)
		})
	}
}

func (s *ConversationPopulationCHTestSuite) TestMetadataSelectedMissingAndExcluded() {
	// Given
	conn := s.GetConversationConnection("population", uuid.NewString())
	seedContext := clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"insert_distributed_sync": 1}))
	start := time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	for _, conversation := range []string{"wanted", "other", "missing"} {
		s.Require().NoError(conn.Exec(seedContext, `INSERT INTO conversation_d (conversation_id, agent_user_id, conversation_start_time, update_time) VALUES (?, 'alice', ?, ?)`, conversation, start, start))
		s.Require().NoError(conn.Exec(seedContext, `INSERT INTO message (message_id, conversation_id, agent_user_id, conversation_start_time) VALUES (?, ?, 'alice', ?)`, uuid.NewString(), conversation, start))
		if conversation != "missing" {
			s.Require().NoError(conn.Exec(seedContext, `INSERT INTO moment_annotation (moment_annotation_id, conversation_id, agent_user_id, conversation_start_time, moment_template_id, moment_type, metadata_string_value, create_time) VALUES (?, ?, 'alice', ?, 'team', ?, ?, ?)`, uuid.NewString(), conversation, start, int32(momentpb.Moment_CONVERSATION_METADATA), conversation, start))
		}
	}
	for _, scenario := range []struct {
		name              string
		selected, missing bool
		expected          []string
	}{
		{name: "Selected", selected: true, expected: []string{"wanted"}},
		{name: "Missing", missing: true, expected: []string{"missing"}},
		{name: "SelectedOrMissing", selected: true, missing: true, expected: []string{"missing", "wanted"}},
	} {
		s.Run(scenario.name, func() {
			// Given
			moment := &momentpb.Moment{Name: "customers/population/profiles/profile/moments/team", Type: momentpb.Moment_CONVERSATION_METADATA}
			group := &analyticspb.MomentGroup{}
			if scenario.selected {
				group.Moments = []*momentpb.Moment{moment}
				group.MetadataValueAttributes = []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "wanted"}}}}
			}
			if scenario.missing {
				group.ExcludedMoments = []*momentpb.Moment{moment}
			}
			inputs := conversationPopulationInputs{timeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(start), EndTimestamp: timestamppb.New(start.Add(time.Hour))}, attribute: &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{group}}}

			// When
			population, err := buildConversationPopulationQuery(s.T().Context(), inputs)
			var got []struct {
				Conversation string `ch:"conversation_id"`
			}
			if err == nil {
				err = conn.Select(s.T().Context(), &got, "SELECT conversation_id FROM ("+population.query+") ORDER BY conversation_id", population.args...)
			}

			// Then
			s.Require().NoError(err)
			expected := make([]struct {
				Conversation string `ch:"conversation_id"`
			}, len(scenario.expected))
			for i, id := range scenario.expected {
				expected[i].Conversation = id
			}
			s.Equal(expected, got)
		})
	}
}
