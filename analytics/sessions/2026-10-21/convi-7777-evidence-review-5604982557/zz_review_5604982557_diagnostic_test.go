package analyticsimpl

import (
	"fmt"
	"time"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/google/uuid"
)

func (s *ConversationAssistanceUsageEvidenceCHTestSuite) TestReviewEvidenceBoundaries() {
	for _, scenario := range []struct {
		name  string
		skew  time.Duration
		large bool
	}{
		{name: "LargeAgentList", large: true},
		{name: "CopiedTimeFourteenDaysEarlier", skew: -14 * 24 * time.Hour},
		{name: "CopiedTimeFourteenDaysLater", skew: 14 * 24 * time.Hour},
	} {
		s.Run(scenario.name, func() {
			// Given
			conversationID := uuid.NewString()
			start := time.Date(2021, 1, 15, 0, 0, 0, 0, time.UTC)
			conn := s.GetConversationConnection(s.customerID, s.profileID)
			s.Require().NoError(conn.Exec(s.T().Context(), `INSERT INTO conversation_event_d (event_id,conversation_id,agent_user_id,conversation_start_time,event_type) VALUES (?,?,'1',?,13)`, uuid.NewString(), conversationID, start.Add(scenario.skew)))
			agentIDs := []string{"1"}
			if scenario.large {
				for i := 0; i < 8000; i++ {
					agentIDs = append(agentIDs, fmt.Sprintf("00000000-0000-0000-0000-%012d", i))
				}
			}
			parent, err := shared.ParseCustomerProfile(s.profileResourceName)
			s.Require().NoError(err)
			population := []conversationAssistanceUsagePopulationRow{{conversationID: conversationID, agentUserID: "1", conversationTime: start}}
			query := conversationAssistanceUsageEvidenceQueries(true /* includeBotConversations */, agentIDs)[0]

			// When
			pairs, err := s.server.readConversationAssistanceUsageFeature(s.T().Context(), parent, population, query)

			// Then
			if scenario.large {
				s.Require().Error(err)
				s.ErrorContains(err, "Max query size exceeded")
				s.T().Logf("REVIEW_RESULT LargeAgentList: %v", err)
			} else {
				s.Require().NoError(err)
				s.Equal([]assistanceUsagePair{{conversationID, "1"}}, pairs)
				s.T().Logf("REVIEW_RESULT %s: eligible pair qualifies with copied time %s from population start", scenario.name, scenario.skew)
			}
		})
	}
}
