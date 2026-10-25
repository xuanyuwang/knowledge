package analyticsimpl

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
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
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	mockconfig "github.com/cresta/go-servers/config/shared/grpcclient/config/mock_config"
	authmock "github.com/cresta/go-servers/shared/auth/mocks"
	clickhouseconv "github.com/cresta/go-servers/shared/clickhouse/conversations"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
	dbtesting "github.com/cresta/go-servers/shared/db/testing"
	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type RetrieveSuggestionStatsTestSuite struct {
	suite.Suite
	dbtesting.DBTestSuite
	chtesting.ClickhouseContainerTestSuite
	server                  *AnalyticsServiceImpl
	profileID               string
	start                   time.Time
	originalExcludedMoments bool
}

func (s *RetrieveSuggestionStatsTestSuite) SetupSuite() {
	s.DBTestSuite.SetupSuite(s.T())
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
	s.server = &AnalyticsServiceImpl{appsDB: s.DBTestSuite.AppsDBRegistry(), logger: log.GetLogger(), clickhouseConvoRegistry: s.GetConversationTestRegistry()}
	s.start = time.Date(2021, 1, 6, 7, 30, 0, 0, time.UTC)
	s.originalExcludedMoments = *enableExcludedMoments
	s.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", "true"))
}

func (s *RetrieveSuggestionStatsTestSuite) SetupTest()    { s.reset() }
func (s *RetrieveSuggestionStatsTestSuite) SetupSubTest() { s.reset() }
func (s *RetrieveSuggestionStatsTestSuite) reset() {
	s.profileID = uuid.NewString()
	s.server.clickhouseConvoRegistry = s.GetConversationTestRegistry()
	s.server.enableExtTableForUserFilter = false
	s.setupRPCClients()
}
func (s *RetrieveSuggestionStatsTestSuite) TearDownSuite() {
	s.Require().NoError(envflag.Set("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", strconv.FormatBool(s.originalExcludedMoments)))
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
	s.DBTestSuite.TearDownSuite(s.T())
}
func (s *RetrieveSuggestionStatsTestSuite) AfterTest(_, _ string) { s.DBTestSuite.AfterTest() }

func (s *RetrieveSuggestionStatsTestSuite) request() *analyticspb.RetrieveSuggestionStatsRequest {
	return &analyticspb.RetrieveSuggestionStatsRequest{
		Parent:            "customers/suggestions/profiles/" + s.profileID,
		FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(s.start), EndTimestamp: timestamppb.New(s.start.Add(48 * time.Hour))},
		FilterByAttribute: &analyticspb.Attribute{},
	}
}
func (s *RetrieveSuggestionStatsTestSuite) seedEvent(row clickhouseconv.ConversationEventRow) {
	if row.ConversationStartTime.IsZero() {
		row.ConversationStartTime = s.start
	}
	if row.EventID == "" {
		row.EventID = uuid.NewString()
	}
	row.CustomerID, row.ProfileID = "suggestions", s.profileID
	row.CreateTime, row.UpdateTime = s.start, s.start
	conn := s.GetConversationConnection("suggestions", s.profileID)
	batch, err := clickhouseshared.PrepareBatchWithRetry(s.T().Context(), conn, fmt.Sprintf("INSERT INTO conversation_event (%s)", clickhouseshared.ExtractCHFields(row)))
	s.Require().NoError(err)
	s.Require().NoError(batch.AppendStruct(&row))
	s.Require().NoError(clickhouseshared.SendBatchWithRetry(s.T().Context(), batch))
}
func (s *RetrieveSuggestionStatsTestSuite) seedMetadata(conversation, template, value string, updated time.Time) {
	conn := s.GetConversationConnection("suggestions", s.profileID)
	s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO moment_annotation (moment_annotation_id, conversation_id, agent_user_id, conversation_start_time, moment_template_id, moment_type, metadata_string_value, create_time, update_time) VALUES (?, ?, 'alice', ?, ?, ?, ?, ?, ?)`, uuid.NewString(), conversation, s.start, template, int32(momentpb.Moment_CONVERSATION_METADATA), value, updated, updated))
}
func (s *RetrieveSuggestionStatsTestSuite) group(template string, excluded bool) *analyticspb.MomentGroup {
	moment := &momentpb.Moment{Name: "customers/suggestions/profiles/" + s.profileID + "/moments/" + template, Type: momentpb.Moment_CONVERSATION_METADATA}
	if excluded {
		return &analyticspb.MomentGroup{ExcludedMoments: []*momentpb.Moment{moment}}
	}
	return &analyticspb.MomentGroup{Moments: []*momentpb.Moment{moment}, MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{Value: &commonpb.MetadataValue_StringValue{StringValue: "wanted"}}}}}
}

type suggestionRegressionBucket struct {
	agent, at                  string
	suggestions, conversations int32
	perConversation, perUser   float32
}
type suggestionRegressionSnapshot struct {
	suggestions, conversations int32
	perConversation, perUser   float32
	buckets                    []suggestionRegressionBucket
}

func snapshotSuggestions(response *analyticspb.RetrieveSuggestionStatsResponse) suggestionRegressionSnapshot {
	result := suggestionRegressionSnapshot{suggestions: response.GetTotalSuggestionUsedCount(), conversations: response.GetTotalConversationCount(), perConversation: response.GetAverageSuggestionUsedCountPerConversation(), perUser: response.GetAverageSuggestionUsedCountPerFrequencyPerUser()}
	for _, group := range response.GetSuggestionStatsResults() {
		agent := ""
		if len(group.GetAttribute().GetUsers()) != 0 {
			agent = group.Attribute.Users[0].Name
			agent = agent[strings.LastIndex(agent, "/")+1:]
		}
		for _, bucket := range group.GetSuggestionStats() {
			at := ""
			if timestamp := bucket.GetTimeRange().GetStartTimestamp().AsTime(); !timestamp.IsZero() {
				at = timestamp.UTC().Format(time.RFC3339)
			}
			result.buckets = append(result.buckets, suggestionRegressionBucket{agent: agent, at: at, suggestions: bucket.GetTotalSuggestionUsedCount(), conversations: bucket.GetTotalConversationCount(), perConversation: bucket.GetAverageSuggestionUsedCountPerConversation(), perUser: bucket.GetAverageSuggestionUsedCountPerFrequencyPerUser()})
		}
	}
	slices.SortFunc(result.buckets, func(a, b suggestionRegressionBucket) int {
		if c := strings.Compare(a.agent, b.agent); c != 0 {
			return c
		}
		return strings.Compare(a.at, b.at)
	})
	return result
}

func (s *RetrieveSuggestionStatsTestSuite) TestEventCountsAndGrouping() {
	for _, scenario := range []struct {
		name                        string
		selected, external, grouped bool
		frequency                   analyticspb.Frequency
		want                        suggestionRegressionSnapshot
	}{
		{name: "Overall", want: suggestionRegressionSnapshot{4, 3, 4.0 / 3, 4.0 / 3, []suggestionRegressionBucket{{suggestions: 4, conversations: 3, perConversation: 4.0 / 3, perUser: 4.0 / 3}}}},
		{name: "ByAgent", grouped: true, want: suggestionRegressionSnapshot{4, 4, 1, 4.0 / 3, []suggestionRegressionBucket{{agent: "alice", suggestions: 2, conversations: 2, perConversation: 1, perUser: 2}, {agent: "bob", suggestions: 1, conversations: 1, perConversation: 1, perUser: 1}, {agent: "charlie", suggestions: 1, conversations: 1, perConversation: 1, perUser: 1}}}},
		{name: "SelectedInline", selected: true, want: suggestionRegressionSnapshot{3, 2, 1.5, 1.5, []suggestionRegressionBucket{{suggestions: 3, conversations: 2, perConversation: 1.5, perUser: 1.5}}}},
		{name: "SelectedExternal", selected: true, external: true, want: suggestionRegressionSnapshot{3, 2, 1.5, 1.5, []suggestionRegressionBucket{{suggestions: 3, conversations: 2, perConversation: 1.5, perUser: 1.5}}}},
		{name: "DailyHonolulu", frequency: analyticspb.Frequency_DAILY, want: suggestionRegressionSnapshot{4, 3, 4.0 / 3, 4.0 / 3, []suggestionRegressionBucket{{at: "2021-01-05T10:00:00Z", suggestions: 3, conversations: 2, perConversation: 1.5, perUser: 1.5}, {at: "2021-01-06T10:00:00Z", suggestions: 1, conversations: 1, perConversation: 1, perUser: 1}}}},
		{name: "WeeklySunday", frequency: analyticspb.Frequency_WEEKLY, want: suggestionRegressionSnapshot{4, 3, 4.0 / 3, 4.0 / 3, []suggestionRegressionBucket{{at: "2021-01-03T10:00:00Z", suggestions: 4, conversations: 3, perConversation: 4.0 / 3, perUser: 4.0 / 3}}}},
	} {
		s.Run(scenario.name, func() {
			// Given
			for _, event := range []clickhouseconv.ConversationEventRow{
				{ConversationID: "shared", AgentUserID: "alice", EventID: "a1", EventType: 4},
				{ConversationID: "shared", AgentUserID: "alice", EventID: "a1", EventType: 4},
				{ConversationID: "shared", AgentUserID: "alice", EventID: "a2", EventType: 5},
				{ConversationID: "shared", AgentUserID: "bob", EventID: "b1", EventType: 10},
				{ConversationID: "other-event-only", AgentUserID: "alice", EventType: 13},
				{ConversationID: "third", AgentUserID: "charlie", EventType: 4, ConversationStartTime: s.start.Add(24 * time.Hour)},
			} {
				s.seedEvent(event)
			}
			request := s.request()
			request.Frequency = scenario.frequency
			request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu", StartDayOfWeek: analyticspb.Metadata_SUNDAY}
			if scenario.grouped {
				request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
			}
			if scenario.selected {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/suggestions/users/alice"}, {Name: "customers/suggestions/users/bob"}}
			}
			s.server.enableExtTableForUserFilter = scenario.external

			// When
			response, err := s.server.readSuggestionStatsFromClickhouse(s.T().Context(), request, nil /* users */)

			// Then
			s.Require().NoError(err)
			s.Equal(scenario.want, snapshotSuggestions(response))
		})
	}
}

func (s *RetrieveSuggestionStatsTestSuite) TestEventFiltersAndTimeBoundaries() {
	for _, scenario := range []struct {
		name      string
		attribute *analyticspb.Attribute
		want      []string
	}{
		{name: "NoAttribute", want: []string{"developer", "match", "other-language", "short", "source", "voicemail"}},
		{name: "Language", attribute: &analyticspb.Attribute{LanguageCodes: []string{"en-US"}}, want: []string{"developer", "match", "short", "source", "voicemail"}},
		{name: "Usecase", attribute: &analyticspb.Attribute{UsecaseNames: []string{"customers/suggestions/profiles/profile/usecases/sales"}}, want: []string{"developer", "match", "other-language", "short", "source", "voicemail"}},
		{name: "Duration", attribute: &analyticspb.Attribute{ConversationDurationRange: &analyticspb.DurationRange{MinDuration: durationpb.New(time.Minute), MaxDuration: durationpb.New(2 * time.Minute)}}, want: []string{"developer", "match", "other-language", "source", "voicemail"}},
		{name: "ExcludeVoicemail", attribute: &analyticspb.Attribute{ConversationCriterions: []analyticspb.ConversationCriterion{analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL}}, want: []string{"developer", "match", "other-language", "short", "source"}},
		{name: "AbsentUser", attribute: &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/suggestions/users/absent"}}}},
	} {
		s.Run(scenario.name, func() {
			// Given
			for _, agent := range []string{"developer", "match", "other-language", "short", "source", "voicemail", "before", "upper", ""} {
				event := clickhouseconv.ConversationEventRow{ConversationID: agent, AgentUserID: agent, EventType: 4, LanguageCode: "en-US", UsecaseID: "sales", ConversationDurationSecs: 90}
				switch agent {
				case "developer":
					event.IsDevUser = true
				case "other-language":
					event.LanguageCode = "fr"
				case "short":
					event.ConversationDurationSecs = 30
				case "source":
					event.ConversationSource = 9
				case "voicemail":
					event.IsVoiceMail = true
				case "before":
					event.ConversationStartTime = s.start.Add(-time.Second)
				case "upper":
					event.ConversationStartTime = s.start.Add(48 * time.Hour)
				}
				s.seedEvent(event)
			}
			request := s.request()
			request.FilterByAttribute = scenario.attribute
			request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
			want := suggestionRegressionSnapshot{}
			if len(scenario.want) > 0 {
				want = suggestionRegressionSnapshot{suggestions: int32(len(scenario.want)), conversations: int32(len(scenario.want)), perConversation: 1, perUser: 1}
				for _, agent := range scenario.want {
					want.buckets = append(want.buckets, suggestionRegressionBucket{agent: agent, suggestions: 1, conversations: 1, perConversation: 1, perUser: 1})
				}
			}

			// When
			response, err := s.server.readSuggestionStatsFromClickhouse(s.T().Context(), request, nil /* users */)

			// Then
			s.Require().NoError(err)
			s.Equal(want, snapshotSuggestions(response))
		})
	}
}

func (s *RetrieveSuggestionStatsTestSuite) TestMetadataIsConversationScopedAndUsesLatestValue() {
	for _, scenario := range []struct {
		name                                  string
		include, exclude, external, nullJoins bool
		want                                  []string
	}{
		{name: "SelectedLatest", include: true, want: []string{"alice", "bob"}},
		{name: "SelectedExternal", include: true, external: true, want: []string{"alice", "bob"}},
		{name: "MissingDefaultJoin", exclude: true},
		{name: "MissingNullableJoin", exclude: true, nullJoins: true, want: []string{"missing"}},
		{name: "SelectedAndNotBlockedDefaultJoin", include: true, exclude: true},
		{name: "SelectedAndNotBlockedNullableJoin", include: true, exclude: true, nullJoins: true, want: []string{"alice", "bob"}},
	} {
		s.Run(scenario.name, func() {
			// Given
			for _, fixture := range []struct{ conversation, agent string }{{"shared", "alice"}, {"shared", "bob"}, {"changed", "changed"}, {"missing", "missing"}} {
				s.seedEvent(clickhouseconv.ConversationEventRow{ConversationID: fixture.conversation, AgentUserID: fixture.agent, EventType: 4})
			}
			s.seedMetadata("shared", "team", "wanted", s.start)
			s.seedMetadata("shared", "team", "wanted", s.start.Add(time.Minute))
			s.seedMetadata("changed", "team", "wanted", s.start)
			s.seedMetadata("changed", "team", "other", s.start.Add(time.Hour))
			s.seedMetadata("changed", "blocked", "present", s.start)
			request := s.request()
			request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
			if scenario.include {
				request.FilterByAttribute.MomentGroups = append(request.FilterByAttribute.MomentGroups, s.group("team", false /* excluded */))
			}
			if scenario.exclude {
				template := "team"
				if scenario.include {
					template = "blocked"
				}
				request.FilterByAttribute.MomentGroups = append(request.FilterByAttribute.MomentGroups, s.group(template, true /* excluded */))
			}
			for _, agent := range []string{"alice", "bob", "changed", "missing"} {
				request.FilterByAttribute.Users = append(request.FilterByAttribute.Users, &userpb.User{Name: "customers/suggestions/users/" + agent})
			}
			s.server.enableExtTableForUserFilter = scenario.external
			ctx := s.T().Context()
			if scenario.nullJoins {
				ctx = ch.Context(ctx, ch.WithSettings(ch.Settings{"join_use_nulls": 1}))
			}
			want := suggestionRegressionSnapshot{}
			if len(scenario.want) > 0 {
				want = suggestionRegressionSnapshot{suggestions: int32(len(scenario.want)), conversations: int32(len(scenario.want)), perConversation: 1, perUser: 1}
			}
			for _, agent := range scenario.want {
				want.buckets = append(want.buckets, suggestionRegressionBucket{agent: agent, suggestions: 1, conversations: 1, perConversation: 1, perUser: 1})
			}

			// When
			response, err := s.server.readSuggestionStatsFromClickhouse(ctx, request, nil /* users */)

			// Then
			s.Require().NoError(err)
			s.Equal(want, snapshotSuggestions(response))
		})
	}
}

func (s *RetrieveSuggestionStatsTestSuite) TestNonSuggestionEventsRetainDenominator() {
	// Given
	s.seedEvent(clickhouseconv.ConversationEventRow{ConversationID: "other", AgentUserID: "alice", EventType: 13})

	// When
	response, err := s.server.readSuggestionStatsFromClickhouse(s.T().Context(), s.request(), nil /* users */)

	// Then
	s.Require().NoError(err)
	s.Equal(suggestionRegressionSnapshot{conversations: 1, buckets: []suggestionRegressionBucket{{conversations: 1}}}, snapshotSuggestions(response))
}

func TestRetrieveSuggestionStats(t *testing.T) {
	suite.Run(t, new(RetrieveSuggestionStatsTestSuite))
}

func (s *RetrieveSuggestionStatsTestSuite) TestEmailAndChatCountersRemainIndependent() {
	for _, scenario := range []struct {
		name                                               string
		external, selected                                 bool
		wantChat, wantPartial, wantFull, wantConversations int32
	}{
		{name: "AllEvents", wantChat: 3, wantPartial: 2, wantFull: 1, wantConversations: 3},
		{name: "SelectedInline", selected: true, wantChat: 3, wantPartial: 2, wantFull: 1, wantConversations: 2},
		{name: "SelectedExternal", selected: true, external: true, wantChat: 3, wantPartial: 2, wantFull: 1, wantConversations: 2},
	} {
		s.Run(scenario.name, func() {
			// Given
			for _, event := range []clickhouseconv.ConversationEventRow{
				{ConversationID: "chat", AgentUserID: "alice", EventID: "chat4", EventType: 4},
				{ConversationID: "chat", AgentUserID: "alice", EventID: "chat4", EventType: 4},
				{ConversationID: "chat", AgentUserID: "alice", EventID: "chat5", EventType: 5},
				{ConversationID: "chat", AgentUserID: "alice", EventID: "chat10", EventType: 10},
				{ConversationID: "email", AgentUserID: "alice", EventID: "partial1", EventType: 25},
				{ConversationID: "email", AgentUserID: "alice", EventID: "partial1", EventType: 25},
				{ConversationID: "email", AgentUserID: "alice", EventID: "partial2", EventType: 25},
				{ConversationID: "email", AgentUserID: "alice", EventID: "full", EventType: 27},
				{ConversationID: "ignored", AgentUserID: "bob", EventID: "not-used", EventType: 13},
			} {
				s.seedEvent(event)
			}
			request := s.request()
			s.server.enableExtTableForUserFilter = scenario.external
			users := int32(2)
			if scenario.selected {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/suggestions/users/alice"}}
				users = 1
			}
			chatPerConversation := float32(scenario.wantChat) / float32(scenario.wantConversations)
			partialPerConversation := float32(scenario.wantPartial) / float32(scenario.wantConversations)
			fullPerConversation := float32(scenario.wantFull) / float32(scenario.wantConversations)
			want := &analyticspb.RetrieveSuggestionStatsResponse{
				TotalConversationCount: scenario.wantConversations, TotalSuggestionUsedCount: scenario.wantChat,
				AverageSuggestionUsedCountPerConversation: chatPerConversation, AverageSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantChat) / float32(users),
				TotalEmailPartialSuggestionUsedCount: scenario.wantPartial, AverageEmailPartialSuggestionUsedCountPerConversation: partialPerConversation, AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantPartial) / float32(users),
				TotalEmailFullSuggestionUsedCount: scenario.wantFull, AverageEmailFullSuggestionUsedCountPerConversation: fullPerConversation, AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantFull) / float32(users),
				SuggestionStatsResults: []*analyticspb.SuggestionStatsResult{{Attribute: &analyticspb.Attribute{},
					TotalConversationCount: scenario.wantConversations, TotalSuggestionUsedCount: scenario.wantChat,
					AverageSuggestionUsedCountPerConversation: chatPerConversation, AverageSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantChat) / float32(users),
					TotalEmailPartialSuggestionUsedCount: scenario.wantPartial, AverageEmailPartialSuggestionUsedCountPerConversation: partialPerConversation, AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantPartial) / float32(users),
					TotalEmailFullSuggestionUsedCount: scenario.wantFull, AverageEmailFullSuggestionUsedCountPerConversation: fullPerConversation, AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantFull) / float32(users),
					SuggestionStats: []*analyticspb.SuggestionStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Time{})},
						TotalConversationCount: scenario.wantConversations, TotalSuggestionUsedCount: scenario.wantChat,
						AverageSuggestionUsedCountPerConversation: chatPerConversation, AverageSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantChat) / float32(users),
						TotalEmailPartialSuggestionUsedCount: scenario.wantPartial, AverageEmailPartialSuggestionUsedCountPerConversation: partialPerConversation, AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantPartial) / float32(users),
						TotalEmailFullSuggestionUsedCount: scenario.wantFull, AverageEmailFullSuggestionUsedCountPerConversation: fullPerConversation, AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: float32(scenario.wantFull) / float32(users),
					}}}},
			}
			// When
			response, err := s.server.readSuggestionStatsFromClickhouse(s.T().Context(), request, nil /* users */)
			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(want, response, protocmp.Transform()))
		})
	}
}

func (s *RetrieveSuggestionStatsTestSuite) setupRPCClients() {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	configClient := mockconfig.NewMockClient(controller)
	configClient.EXPECT().Get("suggestions").AnyTimes().Return(&config.CustomerConfig{}, nil)
	configClient.EXPECT().GetProfileConfig("suggestions", s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	s.server.configClient = configClient
	aclProvider := authmock.NewMockResourceACLHelperProvider(controller)
	aclProvider.EXPECT().Get().AnyTimes().Return(authmock.NewMockResourceACLHelper(controller))
	s.server.resourceACLHelperProvider = aclProvider
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	var users []*internaluserpb.LiteUser
	for _, agent := range []string{"alice", "bob"} {
		users = append(users, &internaluserpb.LiteUser{UserId: agent, Username: agent, FullName: agent + " name", Memberships: []*internaluserpb.UserGroupMembership{{Group: &internaluserpb.LiteGroup{GroupId: "team", DisplayName: "Team", GroupType: internaluserpb.LiteGroup_TEAM}, IsDirectMember: true}}})
	}
	internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{Parent: "customers/suggestions", ProfileIds: []string{s.profileID}, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	userClient := usermock.NewMockUserServiceClient(controller)
	for _, selected := range []bool{false, true} {
		for _, memberships := range []bool{false, true} {
			filter := &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{"customers/suggestions/profiles/" + s.profileID}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}}
			if selected {
				filter.GroupIds = []string{"team"}
			}
			userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{Parent: "customers/suggestions", PageSize: 200, Filter: filter, IncludeGroupMemberships: memberships, IncludeIndirectGroupMemberships: memberships})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: []*userpb.Group{{Name: "customers/suggestions/groups/team", DisplayName: "Team", Type: userpb.Group_TEAM}}}, nil)
		}
	}
	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](userClient)
}

func (s *RetrieveSuggestionStatsTestSuite) TestRPCSeededResponse() {
	for _, scenario := range []struct {
		name                                                                     string
		selected, absent, external, weekly, team, agentAndTeam, include, exclude bool
	}{
		{name: "DailyAgents"}, {name: "DailyAgentsExternal", external: true}, {name: "SelectedBob", selected: true}, {name: "SelectedBobExternal", selected: true, external: true}, {name: "AbsentSelectedUser", absent: true},
		{name: "WeeklyAgents", weekly: true}, {name: "Team", team: true}, {name: "AgentAndTeam", team: true, agentAndTeam: true},
		{name: "LatestMetadata", include: true}, {name: "ExcludedMetadataDefaults", exclude: true}, {name: "IncludeAndExcludeMetadataDefaults", include: true, exclude: true},
	} {
		s.Run(scenario.name, func() {
			// Given
			for _, row := range []clickhouseconv.ConversationEventRow{
				{ConversationID: "alice", AgentUserID: "alice", EventID: "a1", EventType: 4},
				{ConversationID: "alice", AgentUserID: "alice", EventID: "a1", EventType: 4},
				{ConversationID: "alice", AgentUserID: "alice", EventID: "a2", EventType: 4},
				{ConversationID: "alice-other", AgentUserID: "alice", EventID: "other", EventType: 13},
				{ConversationID: "bob", AgentUserID: "bob", EventID: "b1", EventType: 4},
			} {
				s.seedEvent(row)
			}
			s.seedMetadata("alice", "team", "other", s.start)
			s.seedMetadata("alice", "team", "wanted", s.start.Add(time.Minute))
			s.seedMetadata("bob", "blocked", "present", s.start)
			request := s.request()
			request.Frequency = analyticspb.Frequency_DAILY
			request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu", StartDayOfWeek: analyticspb.Metadata_SUNDAY}
			request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
			if scenario.weekly {
				request.Frequency = analyticspb.Frequency_WEEKLY
			}
			if scenario.selected {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/suggestions/users/bob"}}
			}
			if scenario.absent {
				request.FilterByAttribute.Users = []*userpb.User{{Name: "customers/suggestions/users/absent"}}
			}
			if scenario.team {
				request.FilterByAttribute.Groups = []*userpb.Group{{Name: "customers/suggestions/groups/team"}}
				request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP}
				if scenario.agentAndTeam {
					request.GroupByAttributeTypes = append(request.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
				}
			}
			if scenario.include {
				request.FilterByAttribute.MomentGroups = append(request.FilterByAttribute.MomentGroups, s.group("team", false /* excluded */))
			}
			if scenario.exclude {
				request.FilterByAttribute.MomentGroups = append(request.FilterByAttribute.MomentGroups, s.group("blocked", true /* excluded */))
			}
			s.server.enableExtTableForUserFilter = scenario.external
			at := time.Date(2021, 1, 5, 10, 0, 0, 0, time.UTC)
			if scenario.weekly {
				at = time.Date(2021, 1, 3, 10, 0, 0, 0, time.UTC)
			}
			want := &analyticspb.RetrieveSuggestionStatsResponse{}
			if !scenario.absent && !scenario.exclude {
				for _, agent := range []string{"alice", "bob"} {
					if scenario.selected && agent != "bob" || scenario.include && agent != "alice" {
						continue
					}
					used, conversations := int32(1), int32(1)
					if agent == "alice" {
						used, conversations = 2, 2
						if scenario.include {
							conversations = 1
						}
					}
					attribute := &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/suggestions/users/" + agent, Username: agent, FullName: agent + " name"}}}
					if scenario.team {
						attribute.Groups = []*userpb.Group{{Name: "customers/suggestions/groups/team", DisplayName: "Team", Type: userpb.Group_TEAM}}
					}
					bucket := &analyticspb.SuggestionStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(at)}, TotalSuggestionUsedCount: used, TotalConversationCount: conversations, AverageSuggestionUsedCountPerConversation: float32(used) / float32(conversations), AverageSuggestionUsedCountPerFrequencyPerUser: float32(used)}
					want.SuggestionStatsResults = append(want.SuggestionStatsResults, &analyticspb.SuggestionStatsResult{Attribute: attribute, SuggestionStats: []*analyticspb.SuggestionStats{bucket}, TotalSuggestionUsedCount: used, TotalConversationCount: conversations, AverageSuggestionUsedCountPerConversation: float32(used) / float32(conversations), AverageSuggestionUsedCountPerFrequencyPerUser: float32(used)})
					want.TotalSuggestionUsedCount += used
					want.TotalConversationCount += conversations
				}
				want.AverageSuggestionUsedCountPerConversation = float32(want.TotalSuggestionUsedCount) / float32(want.TotalConversationCount)
				want.AverageSuggestionUsedCountPerFrequencyPerUser = float32(want.TotalSuggestionUsedCount) / float32(len(want.SuggestionStatsResults))
				if scenario.team && !scenario.agentAndTeam {
					want.SuggestionStatsResults = []*analyticspb.SuggestionStatsResult{{Attribute: &analyticspb.Attribute{Groups: []*userpb.Group{{Name: "customers/suggestions/groups/team", DisplayName: "Team", Type: userpb.Group_TEAM}}}, TotalSuggestionUsedCount: 3, TotalConversationCount: 3, AverageSuggestionUsedCountPerConversation: 1, AverageSuggestionUsedCountPerFrequencyPerUser: 1.5, SuggestionStats: []*analyticspb.SuggestionStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(at)}, TotalSuggestionUsedCount: 3, TotalConversationCount: 3, AverageSuggestionUsedCountPerConversation: 1, AverageSuggestionUsedCountPerFrequencyPerUser: 1.5}}}}
				}
			}

			// When
			response, err := s.server.RetrieveSuggestionStats(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			s.Empty(cmp.Diff(want, response, protocmp.Transform(), protocmp.SortRepeatedFields(&analyticspb.RetrieveSuggestionStatsResponse{}, "suggestion_stats_results")))
		})
	}
}

func (s *RetrieveSuggestionStatsTestSuite) TestRPCInvalidRequests() {
	for _, scenario := range []struct {
		name    string
		request *analyticspb.RetrieveSuggestionStatsRequest
	}{
		{name: "MissingTimeRange", request: &analyticspb.RetrieveSuggestionStatsRequest{Parent: "customers/suggestions/profiles/" + s.profileID, Frequency: analyticspb.Frequency_DAILY}},
		{name: "TooManyGroupingKeys", request: &analyticspb.RetrieveSuggestionStatsRequest{Parent: "customers/suggestions/profiles/" + s.profileID, Frequency: analyticspb.Frequency_DAILY, GroupByAttributeTypes: []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT, analyticspb.AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE}}},
	} {
		s.Run(scenario.name, func() {
			// Given
			scenario.request.Parent = "customers/suggestions/profiles/" + s.profileID

			// When
			_, err := s.server.RetrieveSuggestionStats(s.T().Context(), scenario.request)

			// Then
			s.Equal(codes.InvalidArgument, status.Code(err))
		})
	}
}

func (s *RetrieveSuggestionStatsTestSuite) TestRPCMultiDayResponse() {
	// Given
	for _, fixture := range []struct {
		conversation, agent string
		day, uses           int
	}{
		{conversation: "alice-first", agent: "alice", day: 0, uses: 2},
		{conversation: "alice-second", agent: "alice", day: 1, uses: 1},
		{conversation: "bob-second", agent: "bob", day: 1, uses: 1},
	} {
		for _, feature := range []struct{ eventType, multiplier int }{{4, 1}, {25, 2}, {27, 3}} {
			for index := 0; index < fixture.uses*feature.multiplier; index++ {
				s.seedEvent(clickhouseconv.ConversationEventRow{ConversationID: fixture.conversation, AgentUserID: fixture.agent, ConversationStartTime: s.start.Add(time.Duration(fixture.day) * 24 * time.Hour), EventID: fmt.Sprintf("%s-%d-%d", fixture.conversation, feature.eventType, index), EventType: int32(feature.eventType)})
			}
		}
	}
	request := s.request()
	request.Frequency = analyticspb.Frequency_DAILY
	request.Metadata = &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"}
	request.GroupByAttributeTypes = []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
	bucket := func(day int, uses, conversations int32) *analyticspb.SuggestionStats {
		return &analyticspb.SuggestionStats{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(time.Date(2021, 1, 5+day, 10, 0, 0, 0, time.UTC))}, TotalSuggestionUsedCount: uses, TotalConversationCount: conversations, AverageSuggestionUsedCountPerConversation: float32(uses) / float32(conversations), AverageSuggestionUsedCountPerFrequencyPerUser: float32(uses), TotalEmailPartialSuggestionUsedCount: uses * 2, AverageEmailPartialSuggestionUsedCountPerConversation: float32(uses*2) / float32(conversations), AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: float32(uses * 2), TotalEmailFullSuggestionUsedCount: uses * 3, AverageEmailFullSuggestionUsedCountPerConversation: float32(uses*3) / float32(conversations), AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: float32(uses * 3)}
	}
	want := &analyticspb.RetrieveSuggestionStatsResponse{TotalSuggestionUsedCount: 4, TotalConversationCount: 3, AverageSuggestionUsedCountPerConversation: 4.0 / 3, AverageSuggestionUsedCountPerFrequencyPerUser: 4.0 / 3, TotalEmailPartialSuggestionUsedCount: 8, AverageEmailPartialSuggestionUsedCountPerConversation: 8.0 / 3, AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: 8.0 / 3, TotalEmailFullSuggestionUsedCount: 12, AverageEmailFullSuggestionUsedCountPerConversation: 12.0 / 3, AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: 12.0 / 3,
		SuggestionStatsResults: []*analyticspb.SuggestionStatsResult{
			{Attribute: &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/suggestions/users/alice", Username: "alice", FullName: "alice name"}}}, TotalSuggestionUsedCount: 3, TotalConversationCount: 2, AverageSuggestionUsedCountPerConversation: 1.5, AverageSuggestionUsedCountPerFrequencyPerUser: 1.5, TotalEmailPartialSuggestionUsedCount: 6, AverageEmailPartialSuggestionUsedCountPerConversation: 6.0 / 2, AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: 6.0 / 2, TotalEmailFullSuggestionUsedCount: 9, AverageEmailFullSuggestionUsedCountPerConversation: 9.0 / 2, AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: 9.0 / 2, SuggestionStats: []*analyticspb.SuggestionStats{bucket(0 /* day */, 2 /* uses */, 1 /* conversations */), bucket(1 /* day */, 1 /* uses */, 1 /* conversations */)}},
			{Attribute: &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/suggestions/users/bob", Username: "bob", FullName: "bob name"}}}, TotalSuggestionUsedCount: 1, TotalConversationCount: 1, AverageSuggestionUsedCountPerConversation: 1, AverageSuggestionUsedCountPerFrequencyPerUser: 1, TotalEmailPartialSuggestionUsedCount: 2, AverageEmailPartialSuggestionUsedCountPerConversation: 2.0 / 1, AverageEmailPartialSuggestionUsedCountPerFrequencyPerUser: 2.0 / 1, TotalEmailFullSuggestionUsedCount: 3, AverageEmailFullSuggestionUsedCountPerConversation: 3.0 / 1, AverageEmailFullSuggestionUsedCountPerFrequencyPerUser: 3.0 / 1, SuggestionStats: []*analyticspb.SuggestionStats{bucket(1 /* day */, 1 /* uses */, 1 /* conversations */)}},
		},
	}

	// When
	response, err := s.server.RetrieveSuggestionStats(s.T().Context(), request)

	// Then
	s.Require().NoError(err)
	s.Empty(cmp.Diff(want, response, protocmp.Transform(), protocmp.SortRepeatedFields(&analyticspb.RetrieveSuggestionStatsResponse{}, "suggestion_stats_results")))
}

func (s *RetrieveSuggestionStatsTestSuite) setupSQLGoldenClients() {
	controller := gomock.NewController(testmock.NewTestReporter(s.T()))
	configClient := mockconfig.NewMockClient(controller)
	configClient.EXPECT().Get("suggestions").AnyTimes().Return(&config.CustomerConfig{}, nil)
	configClient.EXPECT().GetProfileConfig("suggestions", s.profileID).AnyTimes().Return(&config.ProfileConfig{}, nil)
	s.server.configClient = configClient
	aclProvider := authmock.NewMockResourceACLHelperProvider(controller)
	aclProvider.EXPECT().Get().AnyTimes().Return(authmock.NewMockResourceACLHelper(controller))
	s.server.resourceACLHelperProvider = aclProvider
	internalUsers := internalusermock.NewMockInternalUserServiceClient(controller)
	var users []*internaluserpb.LiteUser
	for _, agent := range []string{"1", "2", "3", "4"} {
		groupID, displayName := "group-2", "Group 2"
		if agent == "1" {
			groupID, displayName = "group-1", "Group 1"
		}
		users = append(users, &internaluserpb.LiteUser{UserId: agent, Username: agent, FullName: agent + " name", Memberships: []*internaluserpb.UserGroupMembership{{Group: &internaluserpb.LiteGroup{GroupId: groupID, DisplayName: displayName, GroupType: internaluserpb.LiteGroup_TEAM}, IsDirectMember: true}}})
	}
	for _, groupIDs := range [][]string{nil, {"group-1", "group-2"}} {
		internalUsers.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{Parent: "customers/suggestions", ProfileIds: []string{s.profileID}, GroupIds: groupIDs, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true})).AnyTimes().Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: users}, nil)
	}
	s.server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](internalUsers)
	userClient := usermock.NewMockUserServiceClient(controller)
	for _, selected := range []bool{false, true} {
		for _, memberships := range []bool{false, true} {
			filter := &userpb.ListGroupsRequest_ListGroupsFilter{CustomerProfiles: []string{"customers/suggestions/profiles/" + s.profileID}, GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP}}
			if selected {
				filter.GroupIds = []string{"group-1", "group-2"}
			}
			userClient.EXPECT().ListGroups(gomock.Any(), testmock.ProtoEq(&userpb.ListGroupsRequest{Parent: "customers/suggestions", PageSize: 200, Filter: filter, IncludeGroupMemberships: memberships, IncludeIndirectGroupMemberships: memberships})).AnyTimes().Return(&userpb.ListGroupsResponse{Groups: []*userpb.Group{{Name: "customers/suggestions/groups/group-1", DisplayName: "Group 1", Type: userpb.Group_TEAM}, {Name: "customers/suggestions/groups/group-2", DisplayName: "Group 2", Type: userpb.Group_TEAM}}}, nil)
		}
	}
	s.server.userClientRegistry = clientregistry.NewTestRegistry[userpb.UserServiceClient](userClient)
}

func (s *RetrieveSuggestionStatsTestSuite) TestOriginalSQLGoldens() {
	// Given
	janFirst := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	janTenth := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	janEleventh := time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveSuggestionStatsRequest
		chReqPath string
	}{
		{
			desc: "Test_DailyFilterAndGroupByAgent",
			request: &analyticspb.RetrieveSuggestionStatsRequest{
				Parent: s.request().Parent,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(janFirst),
					EndTimestamp:   timestamppb.New(janTenth),
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{{
						Name: userpb.UserName{
							CustomerID: "suggestions",
							UserID:     "1",
						}.String(),
					}},
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "Pacific/Honolulu",
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
			},
			chReqPath: "clickhouse_RetrieveSuggestionStats_FilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_BasicFilterAndGroupByAgent",
			request: &analyticspb.RetrieveSuggestionStatsRequest{
				Parent: s.request().Parent,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(janFirst),
					EndTimestamp:   timestamppb.New(janTenth),
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "Pacific/Honolulu",
				},
				FilterByAttribute: &analyticspb.Attribute{
					Users: []*userpb.User{
						{
							Name: "customers/suggestions/users/1",
						},
						{
							Name: "customers/suggestions/users/2",
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
			chReqPath: "clickhouse_RetrieveSuggestionStats_BasicFilterAndGroupByAgent_request.sql",
		},
		{
			desc: "Test_DailyFilterAndGroupByGroup",
			request: &analyticspb.RetrieveSuggestionStatsRequest{
				Parent: s.request().Parent,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(janFirst),
					EndTimestamp:   timestamppb.New(janTenth),
				},
				Frequency: analyticspb.Frequency_DAILY,
				Metadata: &analyticspb.Metadata{
					TimeZoneId: "Pacific/Honolulu",
				},
				FilterByAttribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{
						{Name: "customers/suggestions/groups/group-1"},
						{Name: "customers/suggestions/groups/group-2"},
					},
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
				},
			},
			chReqPath: "clickhouse_RetrieveSuggestionStats_FilterAndGroupByGroups_request.sql",
		},
		{
			desc: "Test_WeeklyGroupByAgent",
			request: &analyticspb.RetrieveSuggestionStatsRequest{
				Parent: s.request().Parent,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(janFirst),
					EndTimestamp:   timestamppb.New(janTenth),
				},
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
			},
			chReqPath: "clickhouse_RetrieveSuggestionStats_WeeklyGroupByAgent_request.sql",
		},
		{
			desc: "Test_FilterByMetadataMomentGroups_IncludeAndExcludeMoments",
			request: &analyticspb.RetrieveSuggestionStatsRequest{
				Parent: s.request().Parent,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(janFirst),
					EndTimestamp:   timestamppb.New(janEleventh),
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
			chReqPath: "clickhouse_RetrieveSuggestionStats_FilterByMetadataMomentGroups_IncludeAndExcludeMoments_request.sql",
		},
	} {
		s.Run(tc.desc, func() {
			// Given
			s.setupSQLGoldenClients()
			request := tc.request
			request.Parent = s.request().Parent
			s.seedEvent(clickhouseconv.ConversationEventRow{ConversationID: "golden", AgentUserID: "1", ConversationStartTime: janFirst.Add(12 * time.Hour), ConversationDurationSecs: 60, EventType: 4})
			connection := s.GetConversationConnection("suggestions", s.profileID)
			recording := recordConversationSQL(s.server, connection, "suggestions", s.profileID)
			start, end := "2024-01-01 00:00:00", "2024-01-10 00:00:00"
			args := []any{start, end}
			metadata := strings.Contains(tc.chReqPath, "Metadata")
			if metadata {
				end = "2024-01-11 00:00:00"
				args = []any{start, end, "moment_id_1", start, end, "moment_id_1", "include-value-1", start, end, "moment_id_2", start, end}
			} else if strings.Contains(tc.chReqPath, "Basic") {
				args = append(args, []string{"1", "2"})
				args = append(args, float64(10), float64(100))
			} else if strings.Contains(tc.chReqPath, "Groups") {
				args = append(args, []string{"1", "2", "3", "4"})
			} else if !strings.Contains(tc.chReqPath, "Weekly") {
				args = append(args, []string{"1"})
			}
			want := &analyticspb.RetrieveSuggestionStatsResponse{}
			if !metadata {
				at := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
				if request.Frequency == analyticspb.Frequency_WEEKLY {
					at = time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
				}
				attribute := &analyticspb.Attribute{Users: []*userpb.User{{Name: "customers/suggestions/users/1", Username: "1", FullName: "1 name"}}}
				if strings.Contains(tc.chReqPath, "Groups") {
					attribute = &analyticspb.Attribute{Groups: []*userpb.Group{{Name: "customers/suggestions/groups/group-1", DisplayName: "Group 1", Type: userpb.Group_TEAM}}}
				}
				want = &analyticspb.RetrieveSuggestionStatsResponse{TotalSuggestionUsedCount: 1, TotalConversationCount: 1, AverageSuggestionUsedCountPerConversation: 1, AverageSuggestionUsedCountPerFrequencyPerUser: 1, SuggestionStatsResults: []*analyticspb.SuggestionStatsResult{{Attribute: attribute, TotalSuggestionUsedCount: 1, TotalConversationCount: 1, AverageSuggestionUsedCountPerConversation: 1, AverageSuggestionUsedCountPerFrequencyPerUser: 1, SuggestionStats: []*analyticspb.SuggestionStats{{TimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(at)}, TotalSuggestionUsedCount: 1, TotalConversationCount: 1, AverageSuggestionUsedCountPerConversation: 1, AverageSuggestionUsedCountPerFrequencyPerUser: 1}}}}}
			}

			// When
			response, err := s.server.RetrieveSuggestionStats(s.T().Context(), request)

			// Then
			s.Require().NoError(err)
			assertRecordedSQLGolden(s.T(), recording, tc.chReqPath, args)
			s.Empty(cmp.Diff(want, response, protocmp.Transform()))
		})
	}
}
