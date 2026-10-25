package analyticsimpl

import (
	"fmt"

	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	internalusermock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/nonpublic/user"
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"

	testmock "github.com/cresta/go-servers/shared/testing/mock"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

func (s *ConversationAssistanceUsagePopulationCHTestSuite) TestReviewLargeRestrictedDirectory() {
	for _, external := range []bool{false, true} {
		s.Run(fmt.Sprintf("ExternalUsers_%t", external), func() {
			// Given
			server := *s.server
			server.enableExtTableForUserFilter = external
			users := internalusermock.NewMockInternalUserServiceClient(gomock.NewController(testmock.NewTestReporter(s.T())))
			eligible := []*internaluserpb.LiteUser{{UserId: "1"}}
			for i := 0; i < 8000; i++ {
				eligible = append(eligible, &internaluserpb.LiteUser{UserId: fmt.Sprintf("00000000-0000-0000-0000-%012d", i)})
			}
			users.EXPECT().ListUsersForAnalytics(gomock.Any(), testmock.ProtoEq(&internaluserpb.ListUsersForAnalyticsRequest{
				Parent: s.customerResourceName, ProfileIds: []string{s.profileID}, AgentOnly: true, IncludeIndirectGroupMemberships: true, IncludeInactiveUsers: true,
			})).Return(&internaluserpb.ListUsersForAnalyticsResponse{Users: eligible}, nil)
			server.internalUserServiceClientRegistry = clientregistry.NewTestRegistry[internaluserpb.InternalUserServiceClient](users)
			request := &analyticspb.RetrieveConversationAssistanceUsageStatsRequest{Parent: s.profileResourceName, FilterToAgentsOnly: true, FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: timestamppb.New(s.start), EndTimestamp: timestamppb.New(s.start.Add(time.Hour))}}

			// When
			input, err := server.readConversationAssistanceUsagePopulation(s.T().Context(), request)

			// Then
			if external {
				s.Require().NoError(err)
				s.Equal([]conversationAssistanceUsagePopulationRow{{conversationID: s.sharedID, agentUserID: "1", conversationTime: s.start}}, input.population)
				s.T().Log("REVIEW_RESULT external=true: restricted population query succeeds with 8001 eligible users")
			} else {
				s.Require().Error(err)
				s.ErrorContains(err, "Max query size exceeded")
				s.T().Logf("REVIEW_RESULT external=false: %v", err)
			}
		})
	}
}
