package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/stretchr/testify/suite"
)

type hintUsageRelationSmokeSuite struct {
	HintStatsUsageIntegrationTestSuite
}

func TestHintUsageRelationSmoke(t *testing.T) { suite.Run(t, new(hintUsageRelationSmokeSuite)) }
func (s *hintUsageRelationSmokeSuite) TestRawRows() {
	for _, mode := range []bool{false, true} {
		for _, metadata := range []bool{false, true} {
			s.Run(fmt.Sprintf("Action%t_Metadata%t", mode, metadata), func() {
				// Given
				action := s.hintAction("delivered", "c", "agent")
				action.AdherenceType = 1
				s.insertHintRows("action_annotation_d", action, s.hintAction("regular", "c", "agent"))
				s.insertHintRows("moment_annotation_d", s.hintMoment("m1", "delivered", "c", "agent", 2 /* adherence */), s.hintMoment("m2", "delivered", "c", "agent", 2 /* adherence */), s.hintMoment("m3", "missing", "c", "agent", 2 /* adherence */))
				s.insertHintRows("conversation_event_d", s.hintEvent("e1", "regular", "c", "agent"), s.hintEvent("e2", "regular", "c", "agent"))
				request := s.hintRequest()
				request.FilterByAttribute.Users = []*userpb.User{{Name: fmt.Sprintf("customers/%s/users/agent", s.customerID)}}
				if metadata {
					s.insertHintRows("moment_annotation_d", s.metadataMoment("meta", "c", "region", "wanted", time.Second))
					request.FilterByAttribute.MomentGroups = []*analyticspb.MomentGroup{s.metadataGroup("region", "wanted"), {ExcludedMoments: []*momentpb.Moment{{Name: s.profileResourceName + "/moments/blocked", Type: momentpb.Moment_CONVERSATION_METADATA}}}}
				}
				tables := []ClickhouseTable{momentAnnotationTable, actionAnnotationTable, conversationEventTable}
				conditions, included, excluded, external, err := parseClickhouseFilter(s.T().Context(), request.FilterByTimeRange, request.FilterByAttribute, tables, true /* useExtTable */)
				s.Require().NoError(err)
				relations := buildHintUsageRowRelations(conditions, false /* includeBotConversations */, included, excluded, mode)
				expectedSent := uint64(3)
				if mode {
					expectedSent = 1
				}
				ctx := attachExtTablesToContext(clickhouse.Context(s.T().Context(), clickhouse.WithSettings(clickhouse.Settings{"join_use_nulls": 1})), external)
				conn := s.GetConversationConnection(s.customerID, s.profileID)
				for _, test := range []struct {
					name     string
					relation hintUsageRowRelation
					count    uint64
				}{{"BehavioralSent", relations.behavioralSent, expectedSent}, {"NonBehavioralSent", relations.nonBehavioralSent, 1}, {"BehavioralFollowed", relations.behavioralFollowed, 3}, {"KnowledgeFollowed", relations.knowledgeFollowed, 2}} {
					query, args := test.relation.query("", nil /* groupByKeys */)
					var count, conversations, agents uint64

					// When
					err = conn.QueryRow(ctx, "SELECT count(),uniqExact(conversation_id),uniqExact(agent_user_id) FROM ("+query+")", args...).Scan(&count, &conversations, &agents)

					// Then
					s.Require().NoError(err, test.name)
					s.Equal(test.count, count, test.name)
					s.Equal(uint64(1), conversations, test.name)
					s.Equal(uint64(1), agents, test.name)
				}
			})
		}
	}
}
