package analyticsimpl

import (
	"database/sql"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	conversationpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/conversation"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/shared/testing/assert"
)

type RetrieveTeamUsecaseConversationMappingsTestSuite struct {
	AnalyticsTestSuite
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) BeforeTest(suiteName, methodName string) {
	s.Require().NoError(s.DB().Delete(&dbmodel.Chats{}, "1=1").Error)
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) TestAllInRange() {
	// given
	startedSince := time.Date(2023, time.May, 6, 1, 10, 0, 0, time.UTC)
	startedBefore := time.Date(2023, time.August, 6, 1, 10, 0, 0, time.UTC)
	s.withConversations(startedSince.Add(-time.Hour), "usecase-before-range", "team-before-range", 2)
	expectedMappings := []*analyticspb.TeamUsecaseConversationMapping{
		s.withConversations(startedSince, "some-usecase-exact", "some-team-exact", 5),
		s.withConversations(startedSince, "some-usecase-exact", "other-team-exact", 2),
		s.withConversations(startedSince, "other-usecase-exact", "some-team-exact", 1),
		s.withConversations(startedSince.Add(time.Hour), "some-usecase-later", "some-team-later", 8),
		s.withConversations(startedSince.Add(2*time.Hour), "some-usecase-later", "", 3),
		s.withConversations(startedSince.Add(2*time.Hour), "", "some-team-later", 4),
	}
	s.withConversations(startedBefore, "usecase-after-range", "team-after-range", 2)

	// when
	res, err := s.server.RetrieveTeamUsecaseConversationMappings(s.ctx, &analyticspb.RetrieveTeamUsecaseConversationMappingsRequest{
		Profile:       s.profileResourceName,
		StartedSince:  timestamppb.New(startedSince),
		StartedBefore: timestamppb.New(startedBefore),
	})

	// then
	s.Require().NoError(err)
	s.assertMappings(res, expectedMappings...)
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) TestFilteredGroups() {
	// given
	startedSince := time.Date(2023, time.May, 6, 1, 10, 0, 0, time.UTC)
	startedBefore := time.Date(2023, time.August, 6, 1, 10, 0, 0, time.UTC)
	// before range
	s.withConversations(startedSince.Add(-time.Hour), "team1-before-range", "team1", 2)
	s.withConversations(startedSince.Add(-time.Hour), "team2-before-range", "team2", 2)
	s.withConversations(startedSince.Add(-time.Hour), "team6-before-range", "team6", 2)
	// within range and filtered teams
	expectedMappings := []*analyticspb.TeamUsecaseConversationMapping{
		s.withConversations(startedSince, "team1-in-range", "team1", 5),
		s.withConversations(startedSince.Add(2*time.Hour), "team1-in-range2", "team1", 3),
		s.withConversations(startedSince, "team2-in-range", "team2", 4),
		s.withConversations(startedSince, "team3-in-range", "team3", 2),
	}
	// within range but not filtered teams
	s.withConversations(startedSince, "team4-in-range", "team4", 8)
	s.withConversations(startedSince, "team5-in-range", "team5", 7)
	// after range
	s.withConversations(startedBefore.Add(2*time.Hour), "team1-after-range", "team1", 3)
	s.withConversations(startedBefore.Add(2*time.Hour), "team3-after-range", "team3", 3)
	s.withConversations(startedBefore.Add(2*time.Hour), "team5-after-range", "team5", 3)

	// when
	res, err := s.server.RetrieveTeamUsecaseConversationMappings(s.ctx, &analyticspb.RetrieveTeamUsecaseConversationMappingsRequest{
		Profile:       s.profileResourceName,
		StartedSince:  timestamppb.New(startedSince),
		StartedBefore: timestamppb.New(startedBefore),
		Groups:        s.groupNames("team1", "team2", "team3"),
	})

	// then
	s.Require().NoError(err)
	s.assertMappings(res, expectedMappings...)
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) TestWithLimit() {
	// given
	refTime := time.Date(2023, time.May, 6, 1, 10, 0, 0, time.UTC)
	expectedMappings := []*analyticspb.TeamUsecaseConversationMapping{
		s.withConversations(refTime, "usecase-1", "team-1", 4),
		s.withConversations(refTime.Add(-time.Hour), "usecase-1", "team-2", 2),
		s.withConversations(refTime.Add(-2*time.Hour), "usecase-2", "team-2", 1),
	}
	s.withConversations(refTime.Add(-2*time.Hour), "usecase-2", "team-2", 2)
	s.withConversations(refTime.Add(-3*time.Hour), "usecase-1", "team-1", 10)
	s.withConversations(refTime.Add(-4*time.Hour), "usecase-1", "team-1", 3)

	// when
	res, err := s.server.RetrieveTeamUsecaseConversationMappings(s.ctx, &analyticspb.RetrieveTeamUsecaseConversationMappingsRequest{
		Profile: s.profileResourceName,
		Limit:   7,
	})

	// then
	s.Require().NoError(err)
	s.assertMappings(res, expectedMappings...)
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) withConversations(
	startedAt time.Time, usecaseID string, teamID string, count int,
) *analyticspb.TeamUsecaseConversationMapping {
	var convMetadata *conversationpb.ConversationMetadata
	var usecaseName, teamName string
	if teamID != "" {
		convMetadata = &conversationpb.ConversationMetadata{
			TeamGroupId: teamID,
		}
		teamName = userpb.GroupName{
			CustomerID: s.customerID,
			GroupID:    teamID,
		}.String()
	}
	if usecaseID != "" {
		usecaseName = customerpb.UsecaseName{
			CustomerID: s.customerID,
			ProfileID:  s.profileID,
			UsecaseID:  usecaseID,
		}.String()
	}

	for range count {
		randomID := uuid.NewString()
		conv := &dbmodel.Chats{
			CustomerID:     sql.NullString{String: s.customerID, Valid: true},
			ProfileID:      sql.NullString{String: s.profileID, Valid: true},
			ConversationID: sql.NullString{String: randomID, Valid: true},
			AgentUserID:    sql.NullString{String: "agent1", Valid: true},
			TeamGroupID:    sql.NullString{String: teamID, Valid: teamID != ""},
			PlatformID:     "platform-" + randomID,
			StartedAt:      startedAt.UTC(),
			EndedAt:        sql.NullTime{Valid: true, Time: startedAt.UTC().Add(time.Hour)},
			UsecaseID:      sql.NullString{Valid: usecaseID != "", String: usecaseID},
			Metadata: conversationpb.ConversationMetadataSQL{
				Message: convMetadata,
			},
		}

		s.CreateModels(conv)
	}
	return &analyticspb.TeamUsecaseConversationMapping{
		Usecase: usecaseName,
		Team:    teamName,
		Count:   int32(count),
	}
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) assertMappings(res *analyticspb.RetrieveTeamUsecaseConversationMappingsResponse,
	expectedMappings ...*analyticspb.TeamUsecaseConversationMapping,
) {
	s.Len(res.GetMappings(), len(expectedMappings), "number of expected mappings")
	gotMap := make(map[string]*analyticspb.TeamUsecaseConversationMapping, len(res.GetMappings()))
	for i, got := range res.GetMappings() {
		gotMap[s.key(got)] = res.Mappings[i]
	}

	expectedKeys := make([]string, len(expectedMappings))
	for i, expected := range expectedMappings {
		expectedKeys[i] = s.key(expected)
		got, found := gotMap[s.key(expected)]
		s.Require().True(found, "missing mapping: %s", s.key(expected))
		assert.ProtoEqual(s.T(), expected, got)
	}
	s.ElementsMatch(expectedKeys, fn.MapKeyToSlice(gotMap))
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) key(mapping *analyticspb.TeamUsecaseConversationMapping) string {
	return mapping.Usecase + "--" + mapping.Team
}

func (s *RetrieveTeamUsecaseConversationMappingsTestSuite) groupNames(groupIDs ...string) []string {
	return fn.Map(groupIDs, func(groupID string) string {
		return userpb.GroupName{
			CustomerID: s.customerID,
			GroupID:    groupID,
		}.String()
	})
}

func TestRetrieveTeamUsecaseConversationMappingsTestSuite(t *testing.T) {
	suite.Run(t, &RetrieveTeamUsecaseConversationMappingsTestSuite{})
}
