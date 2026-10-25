package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	usermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/user"
	"github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	authmock "github.com/cresta/go-servers/shared/auth/mocks"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	dbtesting "github.com/cresta/go-servers/shared/db/testing"
	sharedassert "github.com/cresta/go-servers/shared/testing/assert"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type RetrieveConversationStatsTestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite
	server                                     *AnalyticsServiceImpl
	customerID, profileID, profileResourceName string
	start                                      time.Time
	originalExcludedMoments                    bool
}

func (r *RetrieveConversationStatsTestSuite) SetupSuite() {
	r.DBTestSuite.SetupSuite(r.T())
	r.ClickhouseContainerTestSuite.SetupSuite(r.T())
	r.customerID = "population"
	r.server = &AnalyticsServiceImpl{appsDB: r.DBTestSuite.AppsDBRegistry(), logger: log.GetLogger(), clickhouseConvoRegistry: r.GetConversationTestRegistry()}
	r.start = time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	r.originalExcludedMoments = *enableExcludedMoments
}

func (r *RetrieveConversationStatsTestSuite) SetupTest()    { r.resetPopulation() }
func (r *RetrieveConversationStatsTestSuite) SetupSubTest() { r.resetPopulation() }
func (r *RetrieveConversationStatsTestSuite) resetPopulation() {
	r.profileID = uuid.NewString()
	r.profileResourceName = fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID)
	r.server.enableExtTableForUserFilter = false
	r.server.clickhouseConvoRegistry = r.GetConversationTestRegistry()
	r.setupRPCClients()
	r.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", "true"))
}
func (r *RetrieveConversationStatsTestSuite) AfterTest(_, _ string) { r.DBTestSuite.AfterTest() }

func (r *RetrieveConversationStatsTestSuite) TearDownSuite() {
	r.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", strconv.FormatBool(r.originalExcludedMoments)))
	r.ClickhouseContainerTestSuite.TearDownSuite(r.T())
	r.DBTestSuite.TearDownSuite(r.T())
}

func (r *RetrieveConversationStatsTestSuite) seedConversation(row clickhouseshared.ConversationRow) {
	if row.ConversationStartTime.IsZero() {
		row.ConversationStartTime = r.start
	}
	if row.ConversationEndTime.IsZero() {
		row.ConversationEndTime = r.start.Add(time.Hour)
	}
	if row.UpdateTime.IsZero() {
		row.UpdateTime = r.start
	}
	row.CustomerID, row.ProfileID = r.customerID, r.profileID
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(r.populationSeedContext(), conn, fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(row)))
	r.Require().NoError(err)
	r.Require().NoError(batch.AppendStruct(&row))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(r.populationSeedContext(), batch))
}
func (r *RetrieveConversationStatsTestSuite) seedMessage(row clickhouseconv.MessageRow) {
	if row.ConversationStartTime.IsZero() {
		row.ConversationStartTime = r.start
	}
	if row.ConversationEndTime.IsZero() {
		row.ConversationEndTime = r.start.Add(time.Hour)
	}
	if row.MessageID == "" {
		row.MessageID = uuid.NewString()
	}
	row.CustomerID, row.ProfileID = r.customerID, r.profileID
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(r.populationSeedContext(), conn, fmt.Sprintf("INSERT INTO message (%s)", clickhouseshared.ExtractCHFields(row)))
	r.Require().NoError(err)
	r.Require().NoError(batch.AppendStruct(&row))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(r.populationSeedContext(), batch))
}
func (r *RetrieveConversationStatsTestSuite) seedPair(conversationID, agent string, handles ...int32) {
	r.seedConversation(clickhouseshared.ConversationRow{ConversationID: conversationID, AgentUserID: agent})
	for _, handle := range handles {
		r.seedMessage(clickhouseconv.MessageRow{ConversationID: conversationID, AgentUserID: agent, HandleTimeSecs: handle})
	}
}
func (r *RetrieveConversationStatsTestSuite) seedLabel(row clickhouseconv.ConversationWithLabelsRow) {
	if row.ConversationStartTime.IsZero() {
		row.ConversationStartTime = r.start
	}
	if row.ConversationEndTime.IsZero() {
		row.ConversationEndTime = r.start.Add(time.Hour)
	}
	if row.UpdateTime.IsZero() {
		row.UpdateTime = r.start
	}
	row.CustomerID, row.ProfileID = r.customerID, r.profileID
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(r.populationSeedContext(), conn, fmt.Sprintf("INSERT INTO conversation_with_labels (%s)", clickhouseshared.ExtractCHFields(row)))
	r.Require().NoError(err)
	r.Require().NoError(batch.AppendStruct(&row))
	r.Require().NoError(clickhouseshared.SendBatchWithRetry(r.populationSeedContext(), batch))
}
func (r *RetrieveConversationStatsTestSuite) seedMetadata(conversationID, agent, template, value string, created time.Time) {
	conn := r.GetConversationConnection(r.customerID, r.profileID)
	r.Require().NoError(conn.Exec(r.populationSeedContext(), `INSERT INTO moment_annotation (moment_annotation_id, conversation_id, agent_user_id, conversation_start_time, conversation_end_time, moment_template_id, moment_type, metadata_string_value, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), conversationID, agent, r.start, r.start.Add(time.Hour), template, int32(momentpb.Moment_CONVERSATION_METADATA), value, created, created))
}
func (r *RetrieveConversationStatsTestSuite) metadataGroup(template string, selected, missing bool) *analyticspb.MomentGroup {
	moment := &momentpb.Moment{Name: fmt.Sprintf("customers/%s/profiles/%s/moments/%s", r.customerID, r.profileID, template), Type: momentpb.Moment_CONVERSATION_METADATA}
	group := &analyticspb.MomentGroup{}
	if selected {
		group.Moments = []*momentpb.Moment{moment}
		group.MetadataValueAttributes = []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "wanted"}}}}
	}
	if missing {
		group.ExcludedMoments = []*momentpb.Moment{moment}
	}
	return group
}
func (r *RetrieveConversationStatsTestSuite) request() *analyticspb.RetrieveConversationStatsRequest {
	return &analyticspb.RetrieveConversationStatsRequest{
		Parent:                r.profileResourceName,
		FilterByTimeRange:     &analyticspb.TimeRange{StartTimestamp: timestamppb.New(r.start), EndTimestamp: timestamppb.New(r.start.Add(48 * time.Hour))},
		FilterByAttribute:     &analyticspb.Attribute{},
		GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT},
	}
}

type populationBucket struct {
	agent, time string
	count       int32
	handle      int64
	perUser     float32
}
type populationSnapshot struct {
	count                 int32
	handle                int64
	perFrequency, perUser float32
	buckets               []populationBucket
}

func snapshotPopulation(resp *analyticspb.RetrieveConversationStatsResponse) populationSnapshot {
	result := populationSnapshot{count: resp.GetTotalConversationCount(), handle: resp.GetAverageHandleTime().GetSeconds(), perFrequency: resp.GetAverageConversationCountPerFrequency(), perUser: resp.GetAverageConversationCountPerFrequencyPerUser()}
	for _, group := range resp.GetConversationStatsResults() {
		agent := ""
		if len(group.GetAttribute().GetUsers()) != 0 {
			agent = group.Attribute.Users[0].Name[strings.LastIndex(group.Attribute.Users[0].Name, "/")+1:]
		}
		for _, bucket := range group.ConversationStats {
			bucketTime := ""
			if at := bucket.GetTimeRange().GetStartTimestamp().AsTime(); !at.IsZero() {
				bucketTime = at.UTC().Format(time.RFC3339)
			}
			result.buckets = append(result.buckets, populationBucket{agent: agent, time: bucketTime, count: bucket.GetTotalConversationCount(), handle: bucket.GetAverageHandleTime().GetSeconds(), perUser: bucket.GetAverageConversationCountPerFrequencyPerUser()})
		}
	}
	slices.SortFunc(result.buckets, func(a, b populationBucket) int {
		if c := strings.Compare(a.agent, b.agent); c != 0 {
			return c
		}
		return strings.Compare(a.time, b.time)
	})
	return result
}

func (r *RetrieveConversationStatsTestSuite) TestPairMatchingHandleTimeAndUsers() {
	for _, scenario := range []struct {
		name                                  string
		external, grouped, allUsers, metadata bool
		want                                  populationSnapshot
	}{
		{name: "SelectedInlineOverall", want: populationSnapshot{count: 1, handle: 50, perFrequency: 1, perUser: 0.5, buckets: []populationBucket{{count: 1, handle: 50, perUser: 0.5}}}},
		{name: "SelectedExternalOverall", external: true, want: populationSnapshot{count: 1, handle: 50, perFrequency: 1, perUser: 0.5, buckets: []populationBucket{{count: 1, handle: 50, perUser: 0.5}}}},
		{name: "SelectedInlineByAgent", grouped: true, want: populationSnapshot{count: 2, handle: 25, perFrequency: 2, perUser: 1, buckets: []populationBucket{{agent: "alice", count: 1, handle: 20, perUser: 1}, {agent: "bob", count: 1, handle: 30, perUser: 1}}}},
		{name: "SelectedExternalByAgentWithMetadata", grouped: true, external: true, metadata: true, want: populationSnapshot{count: 2, handle: 25, perFrequency: 2, perUser: 1, buckets: []populationBucket{{agent: "alice", count: 1, handle: 20, perUser: 1}, {agent: "bob", count: 1, handle: 30, perUser: 1}}}},
		{name: "AllUsersOverall", allUsers: true, want: populationSnapshot{count: 1, handle: 120, perFrequency: 1, perUser: 1.0 / 3, buckets: []populationBucket{{count: 1, handle: 120, perUser: 1.0 / 3}}}},
		{name: "AllUsersByAgentWithMetadata", allUsers: true, grouped: true, metadata: true, want: populationSnapshot{count: 3, handle: 40, perFrequency: 3, perUser: 1, buckets: []populationBucket{{agent: "alice", count: 1, handle: 20, perUser: 1}, {agent: "bob", count: 1, handle: 30, perUser: 1}, {agent: "charlie", count: 1, handle: 70, perUser: 1}}}},
	} {
		r.Run(scenario.name, func() {
			// Given
			r.seedPair("shared", "alice", 10 /* handleTimeSecs */, 20 /* handleTimeSecs */)
			r.seedPair("shared", "bob", 30 /* handleTimeSecs */)
			r.seedPair("shared", "charlie", 70 /* handleTimeSecs */)
			r.seedPair("no-messages", "alice")
			r.seedConversation(clickhouseshared.ConversationRow{ConversationID: "wrong-agent", AgentUserID: "alice"})
			r.seedMessage(clickhouseconv.MessageRow{ConversationID: "wrong-agent", AgentUserID: "bob", HandleTimeSecs: 500})
			r.seedMessage(clickhouseconv.MessageRow{ConversationID: "message-only", AgentUserID: "alice", HandleTimeSecs: 500})
			r.seedMessage(clickhouseconv.MessageRow{ConversationID: "shared", AgentUserID: "alice", HandleTimeSecs: 900, IsVoiceMail: true})
			r.seedMetadata("shared", "alice", "team", "wanted", r.start)
			request := r.request()
			request.FilterByAttribute.ConversationCriterions = []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}
			if !scenario.allUsers {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + r.customerID + "/users/alice"}, {Name: "customers/" + r.customerID + "/users/bob"}}
			}
			if !scenario.grouped {
				request.GroupByAttributeTypes = nil
			}
			if scenario.metadata {
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", true /* selected */, false /* missing */)}
			}
			r.server.enableExtTableForUserFilter = scenario.external

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(scenario.want, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestAssistanceSourceAndDeveloperPolicies() {
	for _, scenario := range []struct {
		name           string
		state          analyticspb.Attribute_AgentAssistFilter_AgentAssistStateType
		bots, metadata bool
		want           []string
	}{
		{name: "Default", want: []string{"assisted", "external-source", "unassisted"}},
		{name: "DefaultWithBots", bots: true, want: []string{"assisted", "bot", "bot-unassisted", "external-source", "unassisted"}},
		{name: "Unspecified", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_UNSPECIFIED, want: []string{"assisted", "external-source", "unassisted"}},
		{name: "On", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"assisted", "conversation-dev", "conversation-source", "external-source", "label-only"}},
		{name: "OnWithBots", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, bots: true, want: []string{"assisted", "bot", "conversation-dev", "conversation-source", "external-source", "label-only"}},
		{name: "OnWithMetadata", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, metadata: true, want: []string{"assisted", "conversation-dev", "conversation-source", "external-source", "label-only"}},
		{name: "Off", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, want: []string{"unassisted"}},
		{name: "OffWithBots", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, bots: true, want: []string{"bot-unassisted", "unassisted"}},
		{name: "OffWithMetadata", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, metadata: true, want: []string{"unassisted"}},
	} {
		r.Run(scenario.name, func() {
			// Given
			for _, fixture := range []struct {
				id                                                      string
				conversationSource, messageSource                       int32
				conversationDev, messageDev, unassisted, noConversation bool
			}{
				{id: "assisted"}, {id: "unassisted", unassisted: true}, {id: "bot", conversationSource: 7, messageSource: 7},
				{id: "bot-unassisted", conversationSource: 7, messageSource: 7, unassisted: true},
				{id: "external-source", conversationSource: 8, messageSource: 8}, {id: "conversation-dev", conversationDev: true},
				{id: "conversation-source", conversationSource: 9}, {id: "message-dev", messageDev: true}, {id: "message-source", messageSource: 9},
				{id: "empty-agent"}, {id: "label-only", noConversation: true},
			} {
				agent := fixture.id
				if fixture.id == "empty-agent" {
					agent = ""
				}
				if !fixture.noConversation {
					r.seedConversation(clickhouseshared.ConversationRow{ConversationID: fixture.id, AgentUserID: agent, ConversationSource: fixture.conversationSource, IsDevUser: fixture.conversationDev})
				}
				r.seedMessage(clickhouseconv.MessageRow{ConversationID: fixture.id, AgentUserID: agent, ConversationSource: fixture.messageSource, IsDevUser: fixture.messageDev, HandleTimeSecs: 10})
				if !fixture.unassisted {
					r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: fixture.id, AgentUserID: agent, MessageID: uuid.NewString(), WithAgentAssistance: false})
					r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: fixture.id, AgentUserID: agent, MessageID: uuid.NewString(), WithAgentAssistance: true})
				}
				r.seedMetadata(fixture.id, agent, "team", "wanted", r.start)
			}
			request := r.request()
			if scenario.name != "Default" && scenario.name != "DefaultWithBots" {
				request.FilterByAttribute.AgentAssistFilters = []*analyticspb.Attribute_AgentAssistFilter{{AgentAssistUsed: scenario.state}}
			}
			if scenario.metadata {
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", true /* selected */, false /* missing */)}
			}
			expected := populationSnapshot{count: int32(len(scenario.want)), handle: 10, perFrequency: float32(len(scenario.want)), perUser: 1}
			for _, agent := range scenario.want {
				expected.buckets = append(expected.buckets, populationBucket{agent: agent, count: 1, handle: 10, perUser: 1})
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, scenario.bots, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(expected, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestLegacyAndPerMessageAssistance() {
	for _, scenario := range []struct {
		name     string
		state    analyticspb.Attribute_AgentAssistFilter_AgentAssistStateType
		external bool
		want     []string
	}{
		{name: "OnInline", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"legacy-true", "mixed", "per-message"}},
		{name: "OnExternal", external: true, state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"legacy-true", "mixed", "per-message"}},
		{name: "OffInline", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, want: []string{"legacy-false", "no-label", "other-agent", "outside-label"}},
		{name: "OffExternal", external: true, state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, want: []string{"legacy-false", "no-label", "other-agent", "outside-label"}},
	} {
		r.Run(scenario.name, func() {
			// Given
			for _, agent := range []string{"legacy-true", "legacy-false", "mixed", "per-message", "no-label", "other-agent", "outside-label"} {
				r.seedPair(agent, agent, 10 /* handleTimeSecs */)
			}
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "legacy-true", AgentUserID: "legacy-true", WithAgentAssistance: true})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "legacy-false", AgentUserID: "legacy-false"})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "mixed", AgentUserID: "mixed"})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "mixed", AgentUserID: "mixed", MessageID: uuid.NewString(), WithAgentAssistance: true})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "per-message", AgentUserID: "per-message", MessageID: uuid.NewString(), WithAgentAssistance: true})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "per-message", AgentUserID: "per-message", MessageID: uuid.NewString()})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "other-agent", AgentUserID: "not-selected", MessageID: uuid.NewString(), WithAgentAssistance: true})
			r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "outside-label", AgentUserID: "outside-label", MessageID: uuid.NewString(), WithAgentAssistance: true, ConversationStartTime: r.start.Add(-time.Hour)})
			request := r.request()
			request.FilterByAttribute.AgentAssistFilters = []*analyticspb.Attribute_AgentAssistFilter{{AgentAssistUsed: scenario.state}}
			for _, agent := range []string{"legacy-true", "legacy-false", "mixed", "per-message", "no-label", "other-agent", "outside-label"} {
				request.FilterByAttribute.Users = append(request.FilterByAttribute.Users, &userpb.User{Name: "customers/" + r.customerID + "/users/" + agent})
			}
			r.server.enableExtTableForUserFilter = scenario.external
			expected := populationSnapshot{count: int32(len(scenario.want)), handle: 10, perFrequency: float32(len(scenario.want)), perUser: 1}
			for _, agent := range scenario.want {
				expected.buckets = append(expected.buckets, populationBucket{agent: agent, count: 1, handle: 10, perUser: 1})
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(expected, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestMetadataSelectedMissingLatestAndStale() {
	for _, scenario := range []struct {
		name                               string
		selected, missing, stale, disabled bool
		want                               []string
	}{
		{name: "SelectedLatest", selected: true, want: []string{"wanted"}},
		{name: "SelectedStale", selected: true, stale: true, want: []string{"changed", "wanted"}},
		{name: "Missing", missing: true, want: []string{"missing", "outside"}},
		{name: "SelectedOrMissingLatest", selected: true, missing: true, want: []string{"missing", "outside", "wanted"}},
		{name: "SelectedOrMissingStale", selected: true, missing: true, stale: true, want: []string{"changed", "missing", "outside", "wanted"}},
		{name: "MissingDisabled", missing: true, disabled: true, want: []string{"changed", "missing", "other", "outside", "wanted"}},
	} {
		r.Run(scenario.name, func() {
			// Given
			for _, agent := range []string{"changed", "missing", "other", "outside", "wanted"} {
				r.seedPair(agent, agent, 10 /* handleTimeSecs */)
			}
			r.seedMetadata("wanted", "wanted", "team", "wanted", r.start)
			r.seedMetadata("wanted", "wanted", "team", "wanted", r.start.Add(time.Minute))
			r.seedMetadata("other", "other", "team", "other", r.start)
			r.seedMetadata("changed", "changed", "team", "wanted", r.start)
			r.seedMetadata("changed", "changed", "team", "other", r.start.Add(time.Hour))
			conn := r.GetConversationConnection(r.customerID, r.profileID)
			r.Require().NoError(conn.Exec(r.T().Context(), `INSERT INTO moment_annotation (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,moment_template_id,moment_type,metadata_string_value,create_time) VALUES (?, 'outside', 'outside', ?, 'team', ?, 'wanted', ?)`, uuid.NewString(), r.start.Add(-time.Hour), int32(momentpb.Moment_CONVERSATION_METADATA), r.start))
			request := r.request()
			request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", scenario.selected, scenario.missing)}
			r.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", strconv.FormatBool(!scenario.disabled)))
			expected := populationSnapshot{count: int32(len(scenario.want)), handle: 10, perFrequency: float32(len(scenario.want)), perUser: 1}
			for _, agent := range scenario.want {
				expected.buckets = append(expected.buckets, populationBucket{agent: agent, count: 1, handle: 10, perUser: 1})
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, scenario.stale)

			// Then
			r.Require().NoError(err)
			r.Equal(expected, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestMetadataGroupsIntersectAtConversationScope() {
	for _, scenario := range []struct {
		name          string
		missingSecond bool
		want          []populationBucket
		count         int32
		handle        int64
	}{
		{name: "TwoSelectedGroups", want: []populationBucket{{agent: "alice", count: 1, handle: 20, perUser: 1}, {agent: "bob", count: 1, handle: 30, perUser: 1}}, count: 2, handle: 25},
		{name: "SelectedAndMissingGroup", missingSecond: true, want: []populationBucket{{agent: "charlie", count: 1, handle: 40, perUser: 1}}, count: 1, handle: 40},
	} {
		r.Run(scenario.name, func() {
			// Given
			r.seedPair("shared", "alice", 20 /* handleTimeSecs */)
			r.seedPair("shared", "bob", 30 /* handleTimeSecs */)
			r.seedPair("first-only", "charlie", 40 /* handleTimeSecs */)
			r.seedMetadata("shared", "alice", "team", "wanted", r.start)
			r.seedMetadata("shared", "alice", "region", "wanted", r.start)
			r.seedMetadata("first-only", "charlie", "team", "wanted", r.start)
			request := r.request()
			request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", true /* selected */, false /* missing */), r.metadataGroup("region", !scenario.missingSecond, scenario.missingSecond)}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(populationSnapshot{count: scenario.count, handle: scenario.handle, perFrequency: float32(scenario.count), perUser: 1, buckets: scenario.want}, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestMessageAttributeFilters() {
	for _, scenario := range []struct {
		name      string
		attribute *analyticspb.Attribute
		want      []string
	}{
		{name: "VoicemailIncluded", attribute: &analyticspb.Attribute{}, want: []string{"match", "short", "voicemail", "wrong-language", "wrong-usecase"}},
		{name: "VoicemailExcluded", attribute: &analyticspb.Attribute{ConversationCriterions: []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}}, want: []string{"match", "short", "wrong-language", "wrong-usecase"}},
		{name: "Language", attribute: &analyticspb.Attribute{LanguageCodes: []string{"en-US"}}, want: []string{"match", "short", "voicemail", "wrong-usecase"}},
		{name: "Usecase", attribute: &analyticspb.Attribute{UsecaseNames: []string{"customers/customer_id/profiles/profile/usecases/sales"}}, want: []string{"match", "short", "voicemail", "wrong-language"}},
		{name: "DurationRange", attribute: &analyticspb.Attribute{ConversationDurationRange: &analyticspb.DurationRange{MinDuration: durationpb.New(60 * time.Second), MaxDuration: durationpb.New(120 * time.Second)}}, want: []string{"match", "voicemail", "wrong-language", "wrong-usecase"}},
		{name: "Combined", attribute: &analyticspb.Attribute{LanguageCodes: []string{"en-US"}, UsecaseNames: []string{"customers/customer_id/profiles/profile/usecases/sales"}, ConversationCriterions: []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}, ConversationDurationRange: &analyticspb.DurationRange{MinDuration: durationpb.New(60 * time.Second)}}, want: []string{"match"}},
	} {
		r.Run(scenario.name, func() {
			// Given
			for _, agent := range []string{"match", "short", "voicemail", "wrong-language", "wrong-usecase"} {
				r.seedConversation(clickhouseshared.ConversationRow{ConversationID: agent, AgentUserID: agent})
				message := clickhouseconv.MessageRow{ConversationID: agent, AgentUserID: agent, HandleTimeSecs: 10, LanguageCode: "en-US", UsecaseID: "sales", ConversationDurationSecs: 90}
				switch agent {
				case "short":
					message.ConversationDurationSecs = 30
				case "voicemail":
					message.IsVoiceMail = true
				case "wrong-language":
					message.LanguageCode = "fr"
				case "wrong-usecase":
					message.UsecaseID = "support"
				}
				r.seedMessage(message)
			}
			request := r.request()
			request.FilterByAttribute = scenario.attribute
			expected := populationSnapshot{count: int32(len(scenario.want)), handle: 10, perFrequency: float32(len(scenario.want)), perUser: 1}
			for _, agent := range scenario.want {
				expected.buckets = append(expected.buckets, populationBucket{agent: agent, count: 1, handle: 10, perUser: 1})
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(expected, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestTimeBoundariesAndMessageCopies() {
	for _, scenario := range []struct {
		name              string
		endTime, metadata bool
	}{
		{name: "StartTime"}, {name: "StartTimeWithMetadata", metadata: true}, {name: "EndTime", endTime: true}, {name: "EndTimeWithMetadata", endTime: true, metadata: true},
	} {
		r.Run(scenario.name, func() {
			// Given
			for _, fixture := range []struct {
				agent  string
				offset time.Duration
			}{
				{agent: "before", offset: -time.Second}, {agent: "lower", offset: 0}, {agent: "inside", offset: time.Hour}, {agent: "upper", offset: 48 * time.Hour},
			} {
				at := r.start.Add(fixture.offset)
				conversationStart, conversationEnd := at, at.Add(time.Hour)
				if scenario.endTime {
					conversationStart, conversationEnd = r.start.Add(-time.Hour), at
				}
				r.seedConversation(clickhouseshared.ConversationRow{ConversationID: fixture.agent, AgentUserID: fixture.agent, ConversationStartTime: conversationStart, ConversationEndTime: conversationEnd})
				r.seedMessage(clickhouseconv.MessageRow{ConversationID: fixture.agent, AgentUserID: fixture.agent, ConversationStartTime: conversationStart, ConversationEndTime: conversationEnd, HandleTimeSecs: 10})
				conn := r.GetConversationConnection(r.customerID, r.profileID)
				r.Require().NoError(conn.Exec(r.T().Context(), `INSERT INTO moment_annotation (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,conversation_end_time,moment_template_id,moment_type,metadata_string_value,create_time) VALUES (?,?,?,?,?,'team',?,'wanted',?)`, uuid.NewString(), fixture.agent, fixture.agent, conversationStart, conversationEnd, int32(momentpb.Moment_CONVERSATION_METADATA), r.start))
			}
			// Each half of the join must match the interval independently.
			r.seedConversation(clickhouseshared.ConversationRow{ConversationID: "message-outside", AgentUserID: "message-outside"})
			r.seedMessage(clickhouseconv.MessageRow{ConversationID: "message-outside", AgentUserID: "message-outside", ConversationStartTime: r.start.Add(49 * time.Hour), ConversationEndTime: r.start.Add(49 * time.Hour), HandleTimeSecs: 100})
			r.seedConversation(clickhouseshared.ConversationRow{ConversationID: "conversation-outside", AgentUserID: "conversation-outside", ConversationStartTime: r.start.Add(49 * time.Hour), ConversationEndTime: r.start.Add(49 * time.Hour)})
			r.seedMessage(clickhouseconv.MessageRow{ConversationID: "conversation-outside", AgentUserID: "conversation-outside", HandleTimeSecs: 100})
			r.seedMetadata("message-outside", "message-outside", "team", "wanted", r.start)
			r.seedMetadata("conversation-outside", "conversation-outside", "team", "wanted", r.start)
			request := r.request()
			if scenario.endTime {
				request.ConversationTimeRangeField = analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT
			}
			if scenario.metadata {
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", true /* selected */, false /* missing */)}
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(populationSnapshot{count: 2, handle: 10, perFrequency: 2, perUser: 1, buckets: []populationBucket{{agent: "inside", count: 1, handle: 10, perUser: 1}, {agent: "lower", count: 1, handle: 10, perUser: 1}}}, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestDailyWeeklyBucketsAndMatchingVersions() {
	for _, scenario := range []struct {
		name      string
		frequency analyticspb.Frequency
		metadata  bool
		wantTime  string
	}{
		{name: "Daily", frequency: analyticspb.Frequency_DAILY, wantTime: "2021-01-05T10:00:00Z"},
		{name: "DailyWithMetadata", frequency: analyticspb.Frequency_DAILY, metadata: true, wantTime: "2021-01-05T10:00:00Z"},
		{name: "WeeklySunday", frequency: analyticspb.Frequency_WEEKLY, wantTime: "2021-01-03T10:00:00Z"},
		{name: "WeeklySundayWithMetadata", frequency: analyticspb.Frequency_WEEKLY, metadata: true, wantTime: "2021-01-03T10:00:00Z"},
	} {
		r.Run(scenario.name, func() {
			// Given
			r.seedPair("versions", "alice", 20 /* handleTimeSecs */)
			r.seedConversation(clickhouseshared.ConversationRow{ConversationID: "versions", AgentUserID: "alice", ConversationStartTime: r.start.Add(72 * time.Hour), ConversationEndTime: r.start.Add(73 * time.Hour), UpdateTime: r.start.Add(74 * time.Hour)})
			r.seedMetadata("versions", "alice", "team", "wanted", r.start)
			request := r.request()
			request.Frequency = scenario.frequency
			request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu", StartDayOfWeek: analyticspb.Metadata_SUNDAY}
			if scenario.metadata {
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", true /* selected */, false /* missing */)}
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(populationSnapshot{count: 1, handle: 20, perFrequency: 1, perUser: 1, buckets: []populationBucket{{agent: "alice", time: scenario.wantTime, count: 1, handle: 20, perUser: 1}}}, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestEmptyPopulationAndEmptyUserSelection() {
	for _, scenario := range []struct {
		name           string
		seed, selected bool
	}{
		{name: "EmptyDatabase"}, {name: "SelectedUserAbsent", seed: true, selected: true}, {name: "NoMessages", seed: true},
	} {
		r.Run(scenario.name, func() {
			// Given
			request := r.request()
			if scenario.seed {
				r.seedConversation(clickhouseshared.ConversationRow{ConversationID: "no-messages", AgentUserID: "alice"})
			}
			if scenario.selected {
				r.seedMessage(clickhouseconv.MessageRow{ConversationID: "no-messages", AgentUserID: "alice", HandleTimeSecs: 10})
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + r.customerID + "/users/bob"}}
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

			// Then
			r.Require().NoError(err)
			r.Equal(populationSnapshot{}, snapshotPopulation(resp))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestAssistanceMatchesBothConversationAndAgent() {
	for _, external := range []bool{false, true} {
		for _, metadata := range []bool{false, true} {
			for _, scenario := range []struct {
				name   string
				state  analyticspb.Attribute_AgentAssistFilter_AgentAssistStateType
				agent  string
				handle int64
			}{
				{name: "On", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, agent: "alice", handle: 20},
				{name: "Off", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, agent: "bob", handle: 30},
			} {
				r.Run(fmt.Sprintf("%s_External_%t_Metadata_%t", scenario.name, external, metadata), func() {
					// Given
					r.seedPair("shared", "alice", 20 /* handleTimeSecs */)
					r.seedPair("shared", "bob", 30 /* handleTimeSecs */)
					r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "shared", AgentUserID: "alice", MessageID: uuid.NewString(), WithAgentAssistance: true})
					r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: "shared", AgentUserID: "bob", MessageID: uuid.NewString()})
					r.seedMetadata("shared", "alice", "team", "wanted", r.start)
					request := r.request()
					request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + r.customerID + "/users/alice"}, {Name: "customers/" + r.customerID + "/users/bob"}}
					request.FilterByAttribute.AgentAssistFilters = []*analyticspb.Attribute_AgentAssistFilter{{AgentAssistUsed: scenario.state}}
					if metadata {
						request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{r.metadataGroup("team", true /* selected */, false /* missing */)}
					}
					r.server.enableExtTableForUserFilter = external

					// When
					resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, false /* allowMatchingStaleMetadataValues */)

					// Then
					r.Require().NoError(err)
					r.Equal(populationSnapshot{count: 1, handle: scenario.handle, perFrequency: 1, perUser: 1, buckets: []populationBucket{{agent: scenario.agent, count: 1, handle: scenario.handle, perUser: 1}}}, snapshotPopulation(resp))
				})
			}
		}
	}
}

func (r *RetrieveConversationStatsTestSuite) TestMultipleValueOrMissingGroupsWithIncludeAndExclude() {
	for _, stale := range []bool{false, true} {
		r.Run(fmt.Sprintf("AllowStale_%t", stale), func() {
			// Given
			for _, fixture := range []struct{ agent, team, blocked, category, priority string }{
				{agent: "category-selected", team: "wanted", category: "wanted"},
				{agent: "priority-selected", team: "wanted", priority: "wanted"},
				{agent: "both-missing", team: "wanted"},
				{agent: "blocked", team: "wanted", blocked: "present"},
				{agent: "wrong-team", team: "other"},
				{agent: "wrong-category", team: "wanted", category: "other"},
				{agent: "wrong-priority", team: "wanted", priority: "other"},
				{agent: "changed-category", team: "wanted", category: "wanted"},
			} {
				r.seedPair(fixture.agent, fixture.agent, 10 /* handleTimeSecs */)
				for _, field := range []struct{ template, value string }{{"team", fixture.team}, {"blocked", fixture.blocked}, {"category", fixture.category}, {"priority", fixture.priority}} {
					if field.value == "" {
						continue
					}
					r.seedMetadata(fixture.agent, fixture.agent, field.template, field.value, r.start)
				}
			}
			r.seedMetadata("changed-category", "changed-category", "category", "other", r.start.Add(time.Hour))
			request := r.request()
			request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{
				r.metadataGroup("team", true /* selected */, false /* missing */),
				r.metadataGroup("blocked", false /* selected */, true /* missing */),
				r.metadataGroup("category", true /* selected */, true /* missing */),
				r.metadataGroup("priority", true /* selected */, true /* missing */),
			}
			expected := populationSnapshot{count: 3, handle: 10, perFrequency: 3, perUser: 1, buckets: []populationBucket{{agent: "both-missing", count: 1, handle: 10, perUser: 1}, {agent: "category-selected", count: 1, handle: 10, perUser: 1}, {agent: "priority-selected", count: 1, handle: 10, perUser: 1}}}
			if stale {
				expected = populationSnapshot{count: 4, handle: 10, perFrequency: 4, perUser: 1, buckets: []populationBucket{{agent: "both-missing", count: 1, handle: 10, perUser: 1}, {agent: "category-selected", count: 1, handle: 10, perUser: 1}, {agent: "changed-category", count: 1, handle: 10, perUser: 1}, {agent: "priority-selected", count: 1, handle: 10, perUser: 1}}}
			}

			// When
			resp, err := r.server.readConversationStatsFromClickhouse(r.T().Context(), request, nil /* users */, false /* includeBotConversations */, stale)

			// Then
			r.Require().NoError(err)
			r.Equal(expected, snapshotPopulation(resp))
		})
	}
}

func TestRetrieveConversationStats(t *testing.T) {
	suite.Run(t, new(RetrieveConversationStatsTestSuite))
}

func (r *RetrieveConversationStatsTestSuite) setupRPCClients() {
	controller := gomock.NewController(testmock.NewTestReporter(r.T()))
	configClient := mockconfig.NewMockClient(controller)
	configClient.EXPECT().Get(r.customerID).AnyTimes().Return(&config.CustomerConfig{}, nil)
	configClient.EXPECT().GetProfileConfig(r.customerID, r.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	r.server.configClient = configClient
	aclProvider := authmock.NewMockResourceACLHelperProvider(controller)
	aclProvider.EXPECT().Get().AnyTimes().Return(authmock.NewMockResourceACLHelper(controller))
	r.server.resourceACLHelperProvider = aclProvider
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	var users []*internaluserpb.LiteUser
	for _, agent := range []string{"1", "2", "alice", "bob", "changed", "missing", "wrong", "blocked", "high-number", "voicemail"} {
		users = append(users, &internaluserpb.LiteUser{UserId: agent, Username: agent, FullName: agent + " name"})
	}
	internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
		Parent: "customers/" + r.customerID, ProfileIds: []string{r.profileID}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true,
	})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	r.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	userClient := usermock.NewMockUserServiceClient(controller)
	userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{
		Parent: "customers/" + r.customerID, PageSize: 200,
		Filter: &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{r.profileResourceName}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}},
	})).AnyTimes().Return(&userpb.ListGroupsResponse{}, nil)
	r.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](userClient)
}

func (r *RetrieveConversationStatsTestSuite) TestRPCResponseAndUserFiltering() {
	for _, scenario := range []struct {
		name               string
		selected, external bool
	}{
		{name: "AllUsers"}, {name: "AllUsersExternal", external: true},
		{name: "SelectedBob", selected: true}, {name: "SelectedBobExternal", selected: true, external: true},
	} {
		r.Run(scenario.name, func() {
			// Given
			first := time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC)
			seventh := first.Add(6 * 24 * time.Hour)
			for _, fixture := range []struct {
				id, agent string
				start     time.Time
				handle    int32
			}{
				{"alice-first", "alice", first, 4}, {"alice-seventh-1", "alice", seventh, 2},
				{"alice-seventh-2", "alice", seventh, 4}, {"bob-seventh", "bob", seventh, 2},
			} {
				r.seedConversation(clickhouseshared.ConversationRow{ConversationID: fixture.id, AgentUserID: fixture.agent, ConversationStartTime: fixture.start, ConversationEndTime: fixture.start.Add(time.Hour)})
				r.seedMessage(clickhouseconv.MessageRow{ConversationID: fixture.id, AgentUserID: fixture.agent, ConversationStartTime: fixture.start, ConversationEndTime: fixture.start.Add(time.Hour), HandleTimeSecs: fixture.handle})
			}
			request := &analyticspb.RetrieveConversationStatsRequest{
				Parent: r.profileResourceName, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(first.Add(-12 * time.Hour)), EndTimestamp: timestamppb.New(seventh.Add(24 * time.Hour))},
				FilterByAttribute: &analyticspb.Attribute{}, Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT},
			}
			r.server.enableExtTableForUserFilter = scenario.external
			alice := &userpb.User{Name: "customers/" + r.customerID + "/users/alice", Username: "alice", FullName: "alice name"}
			bob := &userpb.User{Name: "customers/" + r.customerID + "/users/bob", Username: "bob", FullName: "bob name"}
			bobResult := &analyticspb.ConversationStatsResult{
				Attribute: &analyticspb.Attribute{Users: []*userpb.User{bob}}, TotalConversationCount: 1, AverageHandleTime: durationpb.New(2 * time.Second), AverageConversationCountPerFrequency: 1, AverageConversationCountPerFrequencyPerUser: 1,
				ConversationStats: []*analyticspb.ConversationStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(seventh.Add(-2 * time.Hour))}, TotalConversationCount: 1, AverageHandleTime: durationpb.New(2 * time.Second), AverageConversationCountPerFrequencyPerUser: 1}},
			}
			expected := &analyticspb.RetrieveConversationStatsResponse{
				TotalConversationCount: 4, AverageHandleTime: durationpb.New(3 * time.Second), AverageConversationCountPerFrequency: 2, AverageConversationCountPerFrequencyPerUser: 4.0 / 3,
				ConversationStatsResults: []*analyticspb.ConversationStatsResult{{
					Attribute: &analyticspb.Attribute{Users: []*userpb.User{alice}}, TotalConversationCount: 3, AverageHandleTime: durationpb.New(3 * time.Second), AverageConversationCountPerFrequency: 1.5, AverageConversationCountPerFrequencyPerUser: 1.5,
					ConversationStats: []*analyticspb.ConversationStats{
						{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(first.Add(-2 * time.Hour))}, TotalConversationCount: 1, AverageHandleTime: durationpb.New(4 * time.Second), AverageConversationCountPerFrequencyPerUser: 1},
						{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(seventh.Add(-2 * time.Hour))}, TotalConversationCount: 2, AverageHandleTime: durationpb.New(3 * time.Second), AverageConversationCountPerFrequencyPerUser: 2},
					},
				}, bobResult},
			}
			if scenario.selected {
				request.FilterByAttribute.Users = []*userpb.User{{Name: bob.Name}}
				expected = &analyticspb.RetrieveConversationStatsResponse{TotalConversationCount: 1, AverageHandleTime: durationpb.New(2 * time.Second), AverageConversationCountPerFrequency: 1, AverageConversationCountPerFrequencyPerUser: 1, ConversationStatsResults: []*analyticspb.ConversationStatsResult{bobResult}}
			}

			// When
			response, err := r.server.RetrieveConversationStats(r.T().Context(), request)

			// Then
			r.Require().NoError(err)
			sharedassert.CmpEquals(r.T(), expected, response)
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) TestRPCMetadataAssistanceAndTimeField() {
	for _, scenario := range []struct {
		name                                     string
		metadata, valueOrMissing, stale, endTime bool
		selected                                 string
		state                                    analyticspb.Attribute_AgentAssistFilter_AgentAssistStateType
		want                                     []string
	}{
		{name: "SelectedAgent", selected: "bob", want: []string{"bob"}},
		{name: "SelectedAgentOn", selected: "bob", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"bob"}},
		{name: "SelectedAgentOff", selected: "bob", state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF},
		{name: "Metadata", metadata: true, want: []string{"alice", "bob"}},
		{name: "MetadataOn", metadata: true, state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"bob"}},
		{name: "MetadataStale", metadata: true, stale: true, want: []string{"alice", "bob", "changed"}},
		{name: "ValueOrMissingLatest", metadata: true, valueOrMissing: true, want: []string{"alice", "bob", "missing"}},
		{name: "ValueOrMissingStale", metadata: true, valueOrMissing: true, stale: true, want: []string{"alice", "bob", "changed", "missing"}},
		{name: "SelectedAgentEndTime", selected: "bob", endTime: true, want: []string{"bob"}},
		{name: "SelectedAgentEndTimeOn", selected: "bob", endTime: true, state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"bob"}},
		{name: "MetadataEndTime", metadata: true, endTime: true, want: []string{"alice", "bob"}},
		{name: "MetadataEndTimeOn", metadata: true, endTime: true, state: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, want: []string{"bob"}},
	} {
		r.Run(scenario.name, func() {
			// Given
			for _, agent := range []string{"alice", "bob", "changed", "missing", "wrong", "blocked", "high-number", "voicemail"} {
				conversationStart := r.start
				if scenario.endTime {
					conversationStart = r.start.Add(-24 * time.Hour)
				}
				end := r.start.Add(time.Hour)
				r.seedConversation(clickhouseshared.ConversationRow{ConversationID: agent, AgentUserID: agent, ConversationStartTime: conversationStart, ConversationEndTime: end})
				r.seedMessage(clickhouseconv.MessageRow{ConversationID: agent, AgentUserID: agent, ConversationStartTime: conversationStart, ConversationEndTime: end, HandleTimeSecs: 10, IsVoiceMail: agent == "voicemail"})
				r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: agent, AgentUserID: agent, ConversationStartTime: conversationStart, ConversationEndTime: end, MessageTime: r.start, MessageID: uuid.NewString(), WithAgentAssistance: agent == "bob"})
				conn := r.GetConversationConnection(r.customerID, r.profileID)
				if agent != "missing" {
					value := "metadata-value-1"
					if agent == "bob" {
						value = "metadata-value-2"
					}
					if agent == "wrong" {
						value = "other"
					}
					for version := 0; version < 2; version++ {
						if agent == "changed" && version == 1 {
							value = "other"
						}
						at := r.start.Add(time.Duration(version) * time.Minute)
						r.Require().NoError(conn.Exec(r.T().Context(), `INSERT INTO moment_annotation (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,conversation_end_time,moment_template_id,moment_type,metadata_string_value,create_time,update_time) VALUES (?,?,?,?,?,'team',?,?,?,?)`, uuid.NewString(), agent, agent, conversationStart, end, int32(momentpb.Moment_CONVERSATION_METADATA), value, at, at))
					}
				}
				numericValue := 50.0
				if agent == "bob" {
					numericValue = 150
				}
				if agent == "high-number" {
					numericValue = 300
				}
				r.Require().NoError(conn.Exec(r.T().Context(), `INSERT INTO moment_annotation (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,conversation_end_time,moment_template_id,moment_type,metadata_number_value,create_time,update_time) VALUES (?,?,?,?,?,'amount',?,?,?,?)`, uuid.NewString(), agent, agent, conversationStart, end, int32(momentpb.Moment_CONVERSATION_METADATA), numericValue, r.start, r.start))
				if agent == "blocked" {
					r.Require().NoError(conn.Exec(r.T().Context(), `INSERT INTO moment_annotation (moment_annotation_id,conversation_id,agent_user_id,conversation_start_time,conversation_end_time,moment_template_id,moment_type,create_time,update_time) VALUES (?,?,?,?,?,'blocked',?,?,?)`, uuid.NewString(), agent, agent, conversationStart, end, int32(momentpb.Moment_CONVERSATION_METADATA), r.start, r.start))
				}
			}
			request := r.request()
			request.AllowMatchingStaleMetadataValues = scenario.stale
			request.Frequency = analyticspb.Frequency_DAILY
			request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}
			request.FilterByAttribute.ConversationCriterions = []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}
			if scenario.selected != "" {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + r.customerID + "/users/" + scenario.selected}}
			}
			if scenario.state != analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_UNSPECIFIED {
				request.FilterByAttribute.AgentAssistFilters = []*analyticspb.Attribute_AgentAssistFilter{{AgentAssistUsed: scenario.state}}
			}
			if scenario.endTime {
				request.ConversationTimeRangeField = analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT
			}
			if scenario.metadata {
				group := &analyticspb.MomentGroup{Moments: []*momentpb.Moment{{Name: r.profileResourceName + "/moments/team", Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "metadata-value-1"}}},
					{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "metadata-value-2"}}},
				}}
				if scenario.valueOrMissing {
					group.ExcludedMoments = []*momentpb.Moment{proto.Clone(group.Moments[0]).(*momentpb.Moment)}
				}
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{group, {
					Moments: []*momentpb.Moment{{Name: r.profileResourceName + "/moments/amount", Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{NumericBin: &commonpb.NumericBin{FromValue: 0, ToValue: 100}}, {NumericBin: &commonpb.NumericBin{FromValue: 100, ToValue: 250}}},
				}, r.metadataGroup("blocked", false /* selected */, true /* missing */)}
			}
			if scenario.valueOrMissing {
				request.FilterByAttribute.ConversationCriterions = nil
				request.FilterByAttribute.MomentGroups = append(request.FilterByAttribute.MomentGroups, &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{{Name: r.profileResourceName + "/moments/voicemail", Type: momentpb.Moment_VOICE_MAIL}}})
			}
			expected := populationSnapshot{}
			if len(scenario.want) > 0 {
				expected = populationSnapshot{count: int32(len(scenario.want)), handle: 10, perFrequency: float32(len(scenario.want)), perUser: 1}
				for _, agent := range scenario.want {
					expected.buckets = append(expected.buckets, populationBucket{agent: agent, time: "2021-01-05T10:00:00Z", count: 1, handle: 10, perUser: 1})
				}
			}

			// When
			response, err := r.server.RetrieveConversationStats(r.T().Context(), request)

			// Then
			r.Require().NoError(err)
			r.Equal(expected, snapshotPopulation(response))
		})
	}
}

func (r *RetrieveConversationStatsTestSuite) populationSeedContext() context.Context {
	return clickhouse.Context(r.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"insert_distributed_sync": 1}))
}

func (r *RetrieveConversationStatsTestSuite) TestRPCSQLGoldens() {
	for _, scenario := range []struct {
		name                                          string
		agent, metadata, valueOrMissing, ended, stale bool
		assistance                                    analyticspb.Attribute_AgentAssistFilter_AgentAssistStateType
		secondCount, totalCount                       int32
		secondHandle, totalHandle                     int64
	}{
		{name: "FilterByAgent_GroupByAgent_WithoutAgentAssistFilter", agent: true, secondCount: 6, totalCount: 6, secondHandle: 8, totalHandle: 8},
		{name: "FilterByAgent_GroupByAgent_WithAgentAssistFilter", agent: true, assistance: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, secondCount: 5, totalCount: 5, secondHandle: 9, totalHandle: 9},
		{name: "FilterByAgent_GroupByAgent_WithoutAgentAssistanceFilter", agent: true, assistance: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF, secondCount: 1, totalCount: 1, secondHandle: 6, totalHandle: 6},
		{name: "FilterByAgent_GroupByAgent_ConversationEndedAt_WithoutAgentAssistFilter", agent: true, ended: true, secondCount: 6, totalCount: 6, secondHandle: 8, totalHandle: 8},
		{name: "FilterByAgent_GroupByAgent_ConversationEndedAt_WithAgentAssistFilter", agent: true, ended: true, assistance: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, secondCount: 5, totalCount: 5, secondHandle: 9, totalHandle: 9},
		{name: "FilterByMetadataMomentGroup_GroupByAgent_WithoutAgentAssist", metadata: true, secondCount: 2, totalCount: 5, secondHandle: 4, totalHandle: 3},
		{name: "FilterByMetadataMomentGroup_GroupByAgent_WithAgentAssist", metadata: true, assistance: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, secondCount: 1, totalCount: 4, secondHandle: 2, totalHandle: 3},
		{name: "FilterByMetadataMomentGroup_GroupByAgent_ConversationEndedAt_WithoutAgentAssist", metadata: true, ended: true, secondCount: 2, totalCount: 5, secondHandle: 4, totalHandle: 3},
		{name: "FilterByMetadataMomentGroup_GroupByAgent_ConversationEndedAt_WithAgentAssist", metadata: true, ended: true, assistance: analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_ON, secondCount: 1, totalCount: 4, secondHandle: 2, totalHandle: 3},
		{name: "FilterByMetadataMomentGroup_GroupByAgent_AllowMatchingStaleMetadataValues", metadata: true, stale: true, secondCount: 3, totalCount: 6, secondHandle: 5, totalHandle: 4},
		{name: "FilterByMetadataMomentGroup_ValueOrMissing_GroupByAgent_LatestValue", valueOrMissing: true, secondCount: 4, totalCount: 7, secondHandle: 7, totalHandle: 5},
		{name: "FilterByMetadataMomentGroup_ValueOrMissing_GroupByAgent_AllowMatchingStaleMetadataValues", valueOrMissing: true, stale: true, secondCount: 5, totalCount: 8, secondHandle: 7, totalHandle: 5},
	} {
		r.Run(scenario.name, func() {
			// Given
			first := time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC)
			seventh := first.Add(6 * 24 * time.Hour)
			start, end := "2021-01-01 00:00:00", "2021-01-10 00:00:00"
			request := &analyticspb.RetrieveConversationStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(first.Add(-12 * time.Hour)), EndTimestamp: timestamppb.New(first.Add(9*24*time.Hour - 12*time.Hour))},
				FilterByAttribute: &analyticspb.Attribute{ConversationCriterions: []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}},
				Frequency:         analyticspb.Frequency_DAILY, Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				GroupByAttributeTypes:            []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT},
				AllowMatchingStaleMetadataValues: scenario.stale,
			}
			if scenario.agent {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/" + r.customerID + "/users/2"}}
			}
			if scenario.ended {
				request.ConversationTimeRangeField = analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT
			}
			if scenario.assistance != analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_UNSPECIFIED {
				request.FilterByAttribute.AgentAssistFilters = []*analyticspb.Attribute_AgentAssistFilter{{AgentAssistUsed: scenario.assistance}}
			}
			stringMoment := &momentpb.Moment{Name: r.profileResourceName + "/moments/moment_id_1", Type: momentpb.Moment_CONVERSATION_METADATA}
			stringValues := []*analyticspb.MetadataValueAttribute{
				{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "metadata-value-1"}}},
				{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "metadata-value-2"}}},
			}
			if scenario.metadata {
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{stringMoment}, MetadataValueAttributes: stringValues},
					{Moments: []*momentpb.Moment{{Name: r.profileResourceName + "/moments/moment_id_2", Type: momentpb.Moment_CONVERSATION_METADATA}}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{NumericBin: &commonpb.NumericBin{FromValue: 0, ToValue: 100}}, {NumericBin: &commonpb.NumericBin{FromValue: 100, ToValue: 250}}}},
					{ExcludedMoments: []*momentpb.Moment{{Name: r.profileResourceName + "/moments/moment_id_3", Type: momentpb.Moment_CONVERSATION_METADATA}}},
				}
			}
			if scenario.valueOrMissing {
				request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{{Moments: []*momentpb.Moment{stringMoment}, ExcludedMoments: []*momentpb.Moment{proto.Clone(stringMoment).(*momentpb.Moment)}, MetadataValueAttributes: stringValues}}
			}

			// Bind positions are part of the query contract, independently of rendered SQL.
			expectedArgs := []any{}
			if scenario.metadata || scenario.valueOrMissing {
				if !scenario.stale {
					expectedArgs = append(expectedArgs, start, end, int32(19), "moment_id_1")
				}
				expectedArgs = append(expectedArgs, start, end, int32(19), "moment_id_1", "metadata-value-1", "metadata-value-2")
				if scenario.metadata {
					if !scenario.stale {
						expectedArgs = append(expectedArgs, start, end, int32(19), "moment_id_2")
					}
					expectedArgs = append(expectedArgs, start, end, int32(19), "moment_id_2", float64(0), float64(100), float64(100), float64(250))
					expectedArgs = append(expectedArgs, start, end, int32(19), "moment_id_3")
				} else {
					expectedArgs = append(expectedArgs, start, end, int32(19), "moment_id_1")
				}
			}
			populationArgs := []any{start, end}
			if scenario.agent {
				populationArgs = append(populationArgs, []string{"2"})
			}
			if scenario.assistance == analyticspb.Attribute_AgentAssistFilter_AGENT_ASSIST_STATE_TYPE_OFF {
				expectedArgs = append(expectedArgs, populationArgs...)
			}
			expectedArgs = append(expectedArgs, populationArgs...)
			expectedArgs = append(expectedArgs, populationArgs...)
			expectedArgs = append(expectedArgs, 1)

			for _, fixture := range []struct {
				id, agent, value                      string
				at                                    time.Time
				handle                                int32
				assisted, changed, blocked, voicemail bool
			}{
				{id: "first", agent: "1", value: "metadata-value-1", at: first, handle: 4, assisted: true},
				{id: "seventh-1", agent: "1", value: "metadata-value-2", at: seventh, handle: 2, assisted: true},
				{id: "seventh-2", agent: "1", value: "metadata-value-1", at: seventh, handle: 4, assisted: true},
				{id: "selected", agent: "2", value: "metadata-value-2", at: seventh, handle: 2, assisted: true},
				{id: "unassisted", agent: "2", value: "metadata-value-1", at: seventh, handle: 6},
				{id: "changed", agent: "2", value: "metadata-value-1", at: seventh, handle: 8, assisted: true, changed: true},
				{id: "missing", agent: "2", at: seventh, handle: 10, assisted: true},
				{id: "blocked", agent: "2", value: "metadata-value-1", at: seventh, handle: 12, assisted: true, blocked: true},
				{id: "wrong-value", agent: "2", value: "other", at: seventh, handle: 14, assisted: true},
				{id: "voicemail", agent: "2", value: "metadata-value-1", at: seventh, handle: 100, assisted: true, voicemail: true},
			} {
				r.seedConversation(clickhouseshared.ConversationRow{ConversationID: fixture.id, AgentUserID: fixture.agent, ConversationStartTime: fixture.at, ConversationEndTime: fixture.at.Add(time.Hour)})
				r.seedMessage(clickhouseconv.MessageRow{ConversationID: fixture.id, AgentUserID: fixture.agent, ConversationStartTime: fixture.at, ConversationEndTime: fixture.at.Add(time.Hour), HandleTimeSecs: fixture.handle, IsVoiceMail: fixture.voicemail})
				r.seedLabel(clickhouseconv.ConversationWithLabelsRow{ConversationID: fixture.id, AgentUserID: fixture.agent, ConversationStartTime: fixture.at, ConversationEndTime: fixture.at.Add(time.Hour), MessageID: fixture.id, WithAgentAssistance: fixture.assisted})
				if fixture.value != "" {
					r.seedMetadata(fixture.id, fixture.agent, "moment_id_1", fixture.value, r.start)
				}
				if fixture.changed {
					r.seedMetadata(fixture.id, fixture.agent, "moment_id_1", "other", r.start.Add(time.Hour))
				}
				r.Require().NoError(r.GetConversationConnection(r.customerID, r.profileID).Exec(r.populationSeedContext(), `INSERT INTO moment_annotation (moment_annotation_id, conversation_id, agent_user_id, conversation_start_time, conversation_end_time, moment_template_id, moment_type, metadata_number_value, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), fixture.id, fixture.agent, r.start, r.start.Add(time.Hour), "moment_id_2", int32(momentpb.Moment_CONVERSATION_METADATA), float64(50), r.start, r.start))
				if fixture.blocked {
					r.seedMetadata(fixture.id, fixture.agent, "moment_id_3", "blocked", r.start)
				}

			}
			second := &analyticspb.ConversationStatsResult{
				Attribute:              &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + r.customerID + "/users/2", Username: "2", FullName: "2 name"}}},
				TotalConversationCount: scenario.secondCount, AverageHandleTime: durationpb.New(time.Duration(scenario.secondHandle) * time.Second),
				AverageConversationCountPerFrequency: float32(scenario.secondCount), AverageConversationCountPerFrequencyPerUser: float32(scenario.secondCount),
				ConversationStats: []*analyticspb.ConversationStats{{
					TimeRange:              &analyticspb.TimeRange{StartTimestamp: timestamppb.New(seventh.Truncate(24 * time.Hour).Add(10 * time.Hour))},
					TotalConversationCount: scenario.secondCount, AverageHandleTime: durationpb.New(time.Duration(scenario.secondHandle) * time.Second), AverageConversationCountPerFrequencyPerUser: float32(scenario.secondCount),
				}},
			}
			expected := &analyticspb.RetrieveConversationStatsResponse{
				TotalConversationCount: scenario.totalCount, AverageHandleTime: durationpb.New(time.Duration(scenario.totalHandle) * time.Second),
				AverageConversationCountPerFrequency: float32(scenario.totalCount), AverageConversationCountPerFrequencyPerUser: float32(scenario.totalCount),
				ConversationStatsResults: []*analyticspb.ConversationStatsResult{second},
			}
			if !scenario.agent {
				firstAgent := &analyticspb.ConversationStatsResult{
					Attribute:              &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/" + r.customerID + "/users/1", Username: "1", FullName: "1 name"}}},
					TotalConversationCount: 3, AverageHandleTime: durationpb.New(3 * time.Second), AverageConversationCountPerFrequency: 1.5, AverageConversationCountPerFrequencyPerUser: 1.5,
					ConversationStats: []*analyticspb.ConversationStats{
						{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(first.Truncate(24 * time.Hour).Add(10 * time.Hour))}, TotalConversationCount: 1, AverageHandleTime: durationpb.New(4 * time.Second), AverageConversationCountPerFrequencyPerUser: 1},
						{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(seventh.Truncate(24 * time.Hour).Add(10 * time.Hour))}, TotalConversationCount: 2, AverageHandleTime: durationpb.New(3 * time.Second), AverageConversationCountPerFrequencyPerUser: 2},
					},
				}
				expected.ConversationStatsResults = []*analyticspb.ConversationStatsResult{firstAgent, second}
				expected.AverageConversationCountPerFrequency = float32(scenario.totalCount) / 2
				expected.AverageConversationCountPerFrequencyPerUser = float32(scenario.totalCount) / 3
			}
			recording := recordConversationSQL(r.server, r.GetConversationConnection(r.customerID, r.profileID), r.customerID, r.profileID)

			// When
			response, err := r.server.RetrieveConversationStats(r.T().Context(), request)

			// Then
			r.Require().NoError(err)
			sharedassert.CmpEquals(r.T(), expected, response)
			assertRecordedSQLGolden(r.T(), recording, "clickhouse_RetrieveConversationStats_"+scenario.name+"_request.sql", expectedArgs)
		})
	}
}
