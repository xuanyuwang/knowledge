package analyticsimpl

import (
	"database/sql"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/shared-go/fn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/shared/testing/assert"
)

type RetrieveUserUsecaseConversationMappingsTestSuite struct {
	AnalyticsTestSuite
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) BeforeTest(suiteName, methodName string) {
	s.Require().NoError(s.DB().Delete(&dbmodel.Chats{}, "1=1").Error)
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) TestAllInRange() {
	// given
	startedSince := time.Date(2023, time.May, 6, 1, 10, 0, 0, time.UTC)
	startedBefore := time.Date(2023, time.August, 6, 1, 10, 0, 0, time.UTC)
	s.withConversations(startedSince.Add(-time.Hour), "usecase-before-range", "user-before-range", 2)
	expectedMappings := []*analyticspb.UserUsecaseConversationMapping{
		s.withConversations(startedSince, "some-usecase-exact", "some-user-exact", 5),
		s.withConversations(startedSince, "some-usecase-exact", "other-user-exact", 2),
		s.withConversations(startedSince, "other-usecase-exact", "some-user-exact", 1),
		s.withConversations(startedSince.Add(time.Hour), "some-usecase-later", "some-user-later", 8),
		s.withConversations(startedSince.Add(2*time.Hour), "", "some-user-later", 4),
	}
	s.withConversations(startedBefore, "usecase-after-range", "user-after-range", 2)

	// when
	res, err := s.server.RetrieveUserUsecaseConversationMappings(s.ctx, &analyticspb.RetrieveUserUsecaseConversationMappingsRequest{
		Profile:       s.profileResourceName,
		StartedSince:  timestamppb.New(startedSince),
		StartedBefore: timestamppb.New(startedBefore),
	})

	// then
	s.Require().NoError(err)
	s.assertMappings(res, expectedMappings...)
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) TestFilteredUsers() {
	// given
	startedSince := time.Date(2023, time.May, 6, 1, 10, 0, 0, time.UTC)
	startedBefore := time.Date(2023, time.August, 6, 1, 10, 0, 0, time.UTC)
	// before range
	s.withConversations(startedSince.Add(-time.Hour), "user1-before-range", "user1", 2)
	s.withConversations(startedSince.Add(-time.Hour), "user2-before-range", "user2", 2)
	s.withConversations(startedSince.Add(-time.Hour), "user6-before-range", "user6", 2)
	// within range and filtered users
	expectedMappings := []*analyticspb.UserUsecaseConversationMapping{
		s.withConversations(startedSince, "user1-in-range", "user1", 5),
		s.withConversations(startedSince.Add(2*time.Hour), "user1-in-range2", "user1", 3),
		s.withConversations(startedSince, "user2-in-range", "user2", 4),
		s.withConversations(startedSince, "user3-in-range", "user3", 2),
	}
	// within range but not filtered users
	s.withConversations(startedSince, "user4-in-range", "user4", 8)
	s.withConversations(startedSince, "user5-in-range", "user5", 7)
	// after range
	s.withConversations(startedBefore.Add(2*time.Hour), "user1-after-range", "user1", 3)
	s.withConversations(startedBefore.Add(2*time.Hour), "user3-after-range", "user3", 3)
	s.withConversations(startedBefore.Add(2*time.Hour), "user5-after-range", "user5", 3)

	// when
	res, err := s.server.RetrieveUserUsecaseConversationMappings(s.ctx, &analyticspb.RetrieveUserUsecaseConversationMappingsRequest{
		Profile:       s.profileResourceName,
		StartedSince:  timestamppb.New(startedSince),
		StartedBefore: timestamppb.New(startedBefore),
		Users:         s.userNames("user1", "user2", "user3"),
	})

	// then
	s.Require().NoError(err)
	s.assertMappings(res, expectedMappings...)
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) TestWithLimit() {
	// given
	refTime := time.Date(2023, time.May, 6, 1, 10, 0, 0, time.UTC)
	expectedMappings := []*analyticspb.UserUsecaseConversationMapping{
		s.withConversations(refTime, "usecase-1", "user-1", 4),
		s.withConversations(refTime.Add(-time.Hour), "usecase-1", "user-2", 2),
		s.withConversations(refTime.Add(-2*time.Hour), "usecase-2", "user-2", 1),
	}
	s.withConversations(refTime.Add(-2*time.Hour), "usecase-2", "user-2", 2)
	s.withConversations(refTime.Add(-3*time.Hour), "usecase-1", "user-1", 10)
	s.withConversations(refTime.Add(-4*time.Hour), "usecase-1", "user-1", 3)

	// when
	res, err := s.server.RetrieveUserUsecaseConversationMappings(s.ctx, &analyticspb.RetrieveUserUsecaseConversationMappingsRequest{
		Profile: s.profileResourceName,
		Limit:   7,
	})

	// then
	s.Require().NoError(err)
	s.assertMappings(res, expectedMappings...)
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) withConversations(
	startedAt time.Time, usecaseID string, userID string, count int,
) *analyticspb.UserUsecaseConversationMapping {
	var usecaseName string
	if usecaseID != "" {
		usecaseName = customerpb.UsecaseName{
			CustomerID: s.customerID,
			ProfileID:  s.profileID,
			UsecaseID:  usecaseID,
		}.String()
	}

	for i := 0; i < count; i++ {
		randomID := uuid.NewString()
		conv := &dbmodel.Chats{
			CustomerID:     sql.NullString{String: s.customerID, Valid: true},
			ProfileID:      sql.NullString{String: s.profileID, Valid: true},
			ConversationID: sql.NullString{String: randomID, Valid: true},
			AgentUserID:    sql.NullString{String: userID, Valid: true},
			PlatformID:     "platform-" + randomID,
			StartedAt:      startedAt.UTC(),
			EndedAt:        sql.NullTime{Valid: true, Time: startedAt.UTC().Add(time.Hour)},
			UsecaseID:      sql.NullString{Valid: usecaseID != "", String: usecaseID},
		}

		s.CreateModels(conv)
	}
	return &analyticspb.UserUsecaseConversationMapping{
		Usecase: usecaseName,
		User: userpb.UserName{
			CustomerID: s.customerID,
			UserID:     userID,
		}.String(),
		Count: int32(count),
	}
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) assertMappings(res *analyticspb.RetrieveUserUsecaseConversationMappingsResponse,
	expectedMappings ...*analyticspb.UserUsecaseConversationMapping,
) {
	s.Len(res.GetMappings(), len(expectedMappings), "number of expected mappings")
	gotMap := make(map[string]*analyticspb.UserUsecaseConversationMapping, len(res.GetMappings()))
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

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) key(mapping *analyticspb.UserUsecaseConversationMapping) string {
	return mapping.Usecase + "--" + mapping.User
}

func (s *RetrieveUserUsecaseConversationMappingsTestSuite) userNames(userIDs ...string) []string {
	return fn.Map(userIDs, func(userID string) string {
		return userpb.UserName{
			CustomerID: s.customerID,
			UserID:     userID,
		}.String()
	})
}

func TestRetrieveUserUsecaseConversationMappingsTestSuite(t *testing.T) {
	suite.Run(t, &RetrieveUserUsecaseConversationMappingsTestSuite{})
}
