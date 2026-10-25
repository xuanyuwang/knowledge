package elasticsearch

import (
	"encoding/json"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkflowAutomationFilterSuite struct {
	suite.Suite
}

func TestWorkflowAutomationFilter(t *testing.T) {
	suite.Run(t, new(WorkflowAutomationFilterSuite))
}

func (s *WorkflowAutomationFilterSuite) TestConditions() {
	// Given
	for _, tc := range []struct {
		name   string
		usages []analyticspb.Attribute_WorkflowAutomationCondition
		want   string
		code   codes.Code
	}{
		{name: "Empty"},
		{name: "Failed", usages: []analyticspb.Attribute_WorkflowAutomationCondition{analyticspb.Attribute_AUTOMATIC_FAILED}, want: `[1]`},
		{name: "MultipleConditionsUseOneTermsFilter", usages: []analyticspb.Attribute_WorkflowAutomationCondition{1, 2, 3, 4, 5, 6, 7}, want: `[1,2,3,4,5,6,7]`},
		{name: "DuplicatesAccepted", usages: []analyticspb.Attribute_WorkflowAutomationCondition{3, 3}, want: `[3,3]`},
		{name: "Unspecified", usages: []analyticspb.Attribute_WorkflowAutomationCondition{0}, code: codes.InvalidArgument},
		{name: "Unknown", usages: []analyticspb.Attribute_WorkflowAutomationCondition{3, 999}, code: codes.InvalidArgument},
	} {
		s.Run(tc.name, func() {
			// Given
			params := &ConversationFilterParams{WorkflowAutomationConditions: tc.usages, AgentUserIDs: []string{"agent"}}

			// When
			filters, _, err := parseConvoFilter(params, nil, false /* enableNumHoldsField */)

			// Then
			s.Require().Equal(tc.code, status.Code(err))
			if tc.code != codes.OK {
				return
			}
			matches := 0
			hasAgentFilter := false
			for _, filter := range filters {
				if filter.Terms == nil {
					continue
				}
				if filter.Terms.AgentUserIDs != nil {
					hasAgentFilter = true
				}
				if filter.Terms.WorkflowAutomationConditions == nil {
					continue
				}
				matches++
				data, err := json.Marshal(filter)
				s.Require().NoError(err)
				s.Require().JSONEq(`{"terms":{"workflow_automation_conditions":`+tc.want+`}}`, string(data))
			}
			s.Require().True(hasAgentFilter, "WFA must compose with the existing agent filter")
			if tc.want == "" {
				s.Require().Zero(matches)
			} else {
				s.Require().Equal(1, matches)
			}
		})
	}
}
