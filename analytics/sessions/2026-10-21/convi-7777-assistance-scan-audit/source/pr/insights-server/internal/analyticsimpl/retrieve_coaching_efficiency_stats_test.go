package analyticsimpl

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	analyticsmock "github.com/cresta/cresta-proto/v2/gen/gomock/cresta/v1/analytics"
	"github.com/cresta/shared-go/fn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	userfilter "github.com/cresta/go-servers/shared/user-filter"
)

func TestRetrieveCoachingEfficiencyStats(t *testing.T) {
	suite.Run(t, &RetrieveCoachingEfficiencyStatsTestSuite{})
}

type RetrieveCoachingEfficiencyStatsTestSuite struct {
	AnalyticsTestSuite
}

type criterionItemParams struct {
	ID                  string
	ExcludeFromQAScores bool
	IsOutcome           bool
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) generateTemplate(criterionItemParamsSlice []*criterionItemParams) string {
	performanceItemTemplate := `
        {
            "auto_qa": null,
            "displayCommentField": null,
            "displayName": "Agent Hello",
            "helpText": null,
            "identifier": "%s",
            "notRemovable": null,
            "required": true,
            "settings": {
                "excludeFromQAScores": %v,
                "excludeOutcomeInsights": true,
                "options": [ { "label": "Yes", "value": 1 }, { "label": "No", "value": 0 } ],
                "showNA": true
            },
            "shortName": null,
            "type": "labeled-radios",
            "weight": 1
        }`
	outcomeItemTemplate := `
        {
            "auto_qa": {
                "triggers": [
                    {
                        "type": "metadata",
                        "resource_name": "customers/%s/profiles/%s/moments/test-outcome-moment"
                    }
                ]
            },
            "displayCommentField": null,
            "displayName": "Outcome Criterion",
            "helpText": null,
            "identifier": "%s",
            "notRemovable": null,
            "required": true,
            "settings": {
                "excludeFromQAScores": %v,
                "excludeOutcomeInsights": true,
                "options": [ { "label": "Yes", "value": 1 }, { "label": "No", "value": 0 } ],
                "showNA": true
            },
            "shortName": null,
            "type": "labeled-radios",
            "weight": 1
        }`
	items := fn.Map(criterionItemParamsSlice, func(params *criterionItemParams) string {
		if params.IsOutcome {
			return fmt.Sprintf(outcomeItemTemplate, r.customerID, r.profileID, params.ID, params.ExcludeFromQAScores)
		}
		return fmt.Sprintf(performanceItemTemplate, params.ID, params.ExcludeFromQAScores)
	})
	return fmt.Sprintf(`{
            "displayCommentField": null,
            "items": [%s],
            "version": 2
        }`, strings.Join(items, ","))
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) SetupSuite() {
	*EnabledRtrieveCoachingEfficiencyStatsLog = true
	r.AnalyticsTestSuite.SetupSuite()
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestReadSessionCriteria() {
	agentAUserID := "agent A"
	agentBUserID := "agent B"
	r.Run("get sessions without criteria", func() {
		criterionAID := "criterionA-" + uuid.NewString()
		dbScorecardTemplate := model.ScorecardTemplates{
			Customer:   r.customerID,
			Profile:    r.profileID,
			ResourceID: "templateA-" + uuid.NewString(),
			Revision:   "rv1-" + uuid.NewString(),
			Template: r.generateTemplate([]*criterionItemParams{{
				ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
			}}),
			UsecaseIds: []string{r.usecaseID},
		}
		dbPlanA := model.CoachingPlans{
			Customer:   r.customerID,
			Profile:    r.profileID,
			ResourceID: "planA-" + uuid.NewString(),
		}
		dbSessionA := model.CoachingSessions{
			Customer:               r.customerID,
			Profile:                r.profileID,
			ResourceID:             "sessionA-" + uuid.NewString(),
			AgentUserID:            agentAUserID,
			CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
			SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
			ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
			ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
			UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
			FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
		}
		dbTargetA := model.Targets{
			Customer:             r.customerID,
			Profile:              r.profileID,
			TargetID:             "targetA-" + uuid.NewString(),
			ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
			CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
			CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
			UsecaseID:            r.usecaseID,
		}
		dbPlanB := model.CoachingPlans{
			Customer:   r.customerID,
			Profile:    r.profileID,
			ResourceID: "planB-" + uuid.NewString(),
		}
		dbSessionB := model.CoachingSessions{
			Customer:               r.customerID,
			Profile:                r.profileID,
			ResourceID:             "sessionB-" + uuid.NewString(),
			AgentUserID:            agentBUserID,
			CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanB.ResourceID},
			SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
			ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
			ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
			UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
			FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
		}
		r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbSessionB))

		parent := &shared.CustomerProfile{
			CustomerID: r.customerID,
			ProfileID:  r.profileID,
		}
		filter := &retrieveCoachingEfficiencyStatsFilter{
			UsecaseIDs: []string{dbSessionA.UsecaseID.String},
			UsersAndGroups: &userfilter.FilteredUsersAndGroups{
				UserNames: []string{userpb.UserName{CustomerID: r.customerID, UserID: dbSessionA.AgentUserID}.String()},
			},
		}
		timeRange := &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -7)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time),
		}
		freq := analytics.Frequency_FREQUENCY_UNSPECIFIED
		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_UNSPECIFIED)
		r.Require().NoError(err)
		r.Len(criteria, 1)
		expectedCriteriaA := coachingSessionCriterion{
			AgentUserID:                dbSessionA.AgentUserID,
			CoachingSessionID:          dbSessionA.ResourceID,
			CoachingSessionAt:          dbSessionA.SessionAt.Time,
			ScorecardTemplateID:        dbTargetA.ScorecardTemplateID,
			CriterionID:                dbTargetA.CriterionOrChapterID.String,
			CoachingSessionSubmitterID: dbSessionA.ManagerSubmitterUserID.String,
			IsEvaluable:                true,
		}
		r.Assert().EqualExportedValues(expectedCriteriaA, *criteria[0])
	})
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestReadSessionCriterionSessionTypeFilter() {
	// Given
	agentAUserID := "agent-A-" + uuid.NewString()
	agentBUserID := "agent-B-" + uuid.NewString()
	criterionAID := "criterionA-" + uuid.NewString()
	groupSessionID := "group-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	focusCriteria := []string{dbScorecardTemplate.ResourceID + "/" + criterionAID}
	sessionAt := sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))}
	// 1:1 session: group_session_id is NULL.
	dbSessionOneOnOne := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "session1on1-" + uuid.NewString(),
		AgentUserID:            agentAUserID,
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
	}
	// 1:1 session with an explicit empty-string group_session_id (not NULL): the COALESCE/NULLIF
	// SQL must treat empty string the same as NULL, i.e. as a 1:1 session.
	dbSessionEmptyGroup := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionEmptyGroup-" + uuid.NewString(),
		AgentUserID:            agentAUserID,
		GroupSessionID:         sql.NullString{Valid: true, String: ""},
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
	}
	// Group/bulk coaching event: two agent rows share one group_session_id.
	dbSessionGroupA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionGroupA-" + uuid.NewString(),
		AgentUserID:            agentAUserID,
		GroupSessionID:         sql.NullString{Valid: true, String: groupSessionID},
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
	}
	dbSessionGroupB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionGroupB-" + uuid.NewString(),
		AgentUserID:            agentBUserID,
		GroupSessionID:         sql.NullString{Valid: true, String: groupSessionID},
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbSessionOneOnOne, &dbSessionEmptyGroup, &dbSessionGroupA, &dbSessionGroupB))

	parent := &shared.CustomerProfile{CustomerID: r.customerID, ProfileID: r.profileID}
	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				userpb.UserName{CustomerID: r.customerID, UserID: agentAUserID}.String(),
				userpb.UserName{CustomerID: r.customerID, UserID: agentBUserID}.String(),
			},
		},
	}
	timeRange := &analytics.TimeRange{
		StartTimestamp: timestamppb.New(sessionAt.Time.AddDate(0, 0, -7)),
		EndTimestamp:   timestamppb.New(sessionAt.Time),
	}
	freq := analytics.Frequency_FREQUENCY_UNSPECIFIED

	r.Run("ALL returns every 1:1 and group row", func() {
		// When
		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_ALL)

		// Then
		r.Require().NoError(err)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.ElementsMatch([]string{dbSessionOneOnOne.ResourceID, dbSessionEmptyGroup.ResourceID, dbSessionGroupA.ResourceID, dbSessionGroupB.ResourceID}, sessionIDs)
	})

	r.Run("ONE_ON_ONE returns only ungrouped rows (NULL and empty string)", func() {
		// When
		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_ONE_ON_ONE)

		// Then
		r.Require().NoError(err)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.ElementsMatch([]string{dbSessionOneOnOne.ResourceID, dbSessionEmptyGroup.ResourceID}, sessionIDs)
	})

	r.Run("GROUP returns only grouped rows", func() {
		// When
		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_GROUP)

		// Then
		r.Require().NoError(err)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.ElementsMatch([]string{dbSessionGroupA.ResourceID, dbSessionGroupB.ResourceID}, sessionIDs)
	})
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestIncludeSessionsWithoutFocusCriteria() {
	agentAUserID := "agent-with-criteria-" + uuid.NewString()
	agentBUserID := "agent-without-criteria-" + uuid.NewString()
	agentCUserID := "agent-with-criteria-no-target-" + uuid.NewString()
	agentDUserID := "agent-without-criteria-no-target-" + uuid.NewString()

	criterionAID := "criterionA-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}

	// Plan A: Has target with template and criterion
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	// Session A: Has focus criteria + plan has target
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            agentAUserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}

	// Session B: Has focus criteria (backfilled from plan target)
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            agentBUserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID}, // Backfilled from target
	}

	// Plan C: NO target
	dbPlanC := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planC-" + uuid.NewString(),
	}

	// Session C: Has focus criteria, but plan has NO target
	dbSessionC := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionC-" + uuid.NewString(),
		AgentUserID:            agentCUserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanC.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachC-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}

	// Session D: NO focus criteria AND plan has NO target
	dbSessionD := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionD-" + uuid.NewString(),
		AgentUserID:            agentDUserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanC.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachD-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{}, // Empty - no focus criteria
	}

	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbSessionB, &dbPlanC, &dbSessionC, &dbSessionD))

	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	timeRange := &analytics.TimeRange{
		StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -7)),
		EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time),
	}
	freq := analytics.Frequency_FREQUENCY_UNSPECIFIED

	r.Run("flag=false with template filter - existing behavior", func() {
		filter := &retrieveCoachingEfficiencyStatsFilter{
			UsecaseIDs:           []string{r.usecaseID},
			ScorecardTemplateIDs: []string{dbScorecardTemplate.ResourceID},
			UsersAndGroups: &userfilter.FilteredUsersAndGroups{
				UserNames: []string{
					userpb.UserName{CustomerID: r.customerID, UserID: agentAUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentBUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentCUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentDUserID}.String(),
				},
			},
		}

		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_UNSPECIFIED)
		r.Require().NoError(err)

		// Original behavior: WHERE (COALESCE(focus_criteria_template, target_template) IN (filter))
		// - Session A: COALESCE(template, template) = template → INCLUDED
		// - Session B: COALESCE(NULL, template) = template → INCLUDED
		// - Session C: COALESCE(template, NULL) = template → INCLUDED
		// - Session D: COALESCE(NULL, NULL) = NULL → EXCLUDED (doesn't match filter)
		r.Len(criteria, 3)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.Contains(sessionIDs, dbSessionA.ResourceID)
		r.Contains(sessionIDs, dbSessionB.ResourceID)
		r.Contains(sessionIDs, dbSessionC.ResourceID)
		r.NotContains(sessionIDs, dbSessionD.ResourceID)
	})

	r.Run("flag=true - includes all sessions", func() {
		filter := &retrieveCoachingEfficiencyStatsFilter{
			UsecaseIDs: []string{r.usecaseID},
			UsersAndGroups: &userfilter.FilteredUsersAndGroups{
				UserNames: []string{
					userpb.UserName{CustomerID: r.customerID, UserID: agentAUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentBUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentCUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentDUserID}.String(),
				},
			},
		}

		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_UNSPECIFIED)
		r.Require().NoError(err)

		// Should get all 4 sessions
		r.Len(criteria, 4)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.Contains(sessionIDs, dbSessionA.ResourceID)
		r.Contains(sessionIDs, dbSessionB.ResourceID)
		r.Contains(sessionIDs, dbSessionC.ResourceID)
		r.Contains(sessionIDs, dbSessionD.ResourceID)
	})

	r.Run("flag=true with template filter - only includes sessions related to template", func() {
		filter := &retrieveCoachingEfficiencyStatsFilter{
			UsecaseIDs:           []string{r.usecaseID},
			ScorecardTemplateIDs: []string{dbScorecardTemplate.ResourceID},
			UsersAndGroups: &userfilter.FilteredUsersAndGroups{
				UserNames: []string{
					userpb.UserName{CustomerID: r.customerID, UserID: agentAUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentBUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentCUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentDUserID}.String(),
				},
			},
		}

		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_UNSPECIFIED)
		r.Require().NoError(err)

		// When template filter is set, flag doesn't matter - only include sessions related to the template
		// - Session A: COALESCE(template, template) = template → INCLUDED
		// - Session B: COALESCE(NULL, template) = template → INCLUDED
		// - Session C: COALESCE(template, NULL) = template → INCLUDED (has focus criteria for template)
		// - Session D: COALESCE(NULL, NULL) = NULL → EXCLUDED (no relation to filtered template)
		r.Len(criteria, 3)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.Contains(sessionIDs, dbSessionA.ResourceID)
		r.Contains(sessionIDs, dbSessionB.ResourceID)
		r.Contains(sessionIDs, dbSessionC.ResourceID)
		r.NotContains(sessionIDs, dbSessionD.ResourceID)
	})

	r.Run("flag=true with criterion filter - only includes sessions related to criterion", func() {
		filter := &retrieveCoachingEfficiencyStatsFilter{
			UsecaseIDs:   []string{r.usecaseID},
			CriterionIDs: []string{criterionAID},
			UsersAndGroups: &userfilter.FilteredUsersAndGroups{
				UserNames: []string{
					userpb.UserName{CustomerID: r.customerID, UserID: agentAUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentBUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentCUserID}.String(),
					userpb.UserName{CustomerID: r.customerID, UserID: agentDUserID}.String(),
				},
			},
		}

		criteria, err := r.server.readCoachingSessionCriterion(r.ctx, parent, filter, timeRange, &freq, nil, false, analytics.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_UNSPECIFIED)
		r.Require().NoError(err)

		// When criterion filter is set, flag doesn't matter - only include sessions related to the criterion
		// - Session A: COALESCE(criterion, criterion) = criterion → INCLUDED
		// - Session B: COALESCE(NULL, criterion) = criterion → INCLUDED
		// - Session C: COALESCE(criterion, NULL) = criterion → INCLUDED (has focus criteria for criterion)
		// - Session D: COALESCE(NULL, NULL) = NULL → EXCLUDED (no relation to filtered criterion)
		r.Len(criteria, 3)
		sessionIDs := fn.Map(criteria, func(c *coachingSessionCriterion) string { return c.CoachingSessionID })
		r.Contains(sessionIDs, dbSessionA.ResourceID)
		r.Contains(sessionIDs, dbSessionB.ResourceID)
		r.Contains(sessionIDs, dbSessionC.ResourceID)
		r.NotContains(sessionIDs, dbSessionD.ResourceID)
	})
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestDummyImplementation() {
	r.T().Skip()
	r.mockACLHelperProvider.EXPECT().Get().Return(r.mockACLHelper).Times(1)
	_, err := r.server.RetrieveCoachingEfficiencyStats(r.ctx, &analytics.RetrieveCoachingEfficiencyStatsRequest{
		Parent:            r.profileResourceName,
		FilterByAttribute: &analytics.Attribute{},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(time.Now().AddDate(0, -1, 0)),
			EndTimestamp:   timestamppb.New(time.Now()),
		},
		GroupByAttributeTypes: []analytics.AttributeType{
			analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION,
			analytics.AttributeType_ATTRIBUTE_TYPE_GROUP,
		},
	})
	r.Require().Error(err)
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestFillBeforeAndAfterScores() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessionA := &coachingSessionCriterion{
		AgentUserID:         "agent A",
		CoachingSessionID:   "session A",
		CriterionID:         "criterion A",
		ScorecardTemplateID: "scorecard template A",
		CoachingSessionAt:   fn.Must(time.Parse(time.DateOnly, "2020-01-10")),
	}
	sessionB := &coachingSessionCriterion{
		AgentUserID:       "agent B",
		CoachingSessionID: "session B",
		CoachingSessionAt: fn.Must(time.Parse(time.DateOnly, "2020-01-10")),
	}
	sessions := []*coachingSessionCriterion{sessionA, sessionB}
	scorecardTemplateNames := []*coaching.ScorecardTemplateName{{
		CustomerID:                  r.customerID,
		ProfileID:                   r.profileID,
		ScorecardTemplateID:         "scorecard template A",
		ScorecardTemplateRevisionID: "revision 1",
	}}
	filter := &retrieveCoachingEfficiencyStatsFilter{
		ScorecardTemplateNames: fn.Map(scorecardTemplateNames, func(n *coaching.ScorecardTemplateName) string { return n.String() }),
		CriterionIDs:           []string{sessions[0].CriterionID, "criterion B"},
	}
	// set up the mock call
	sessionAAgentName := userpb.UserName{CustomerID: r.customerID, UserID: sessionA.AgentUserID}
	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{sessionA.AgentUserID},
		[]string{sessionA.CriterionID},
		getEndOfDay(sessionA.CoachingSessionAt.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{sessionA.AgentUserID},
		[]string{sessionA.CriterionID},
		getEndOfDay(sessionA.CoachingSessionAt.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         sessionA.CriterionID,
						ScorecardTemplateId: sessionA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         sessionA.CriterionID,
						ScorecardTemplateId: sessionA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()

	// When
	err := fillBeforeAndAfterQAScores(r.ctx, r.server.logger, parent, &qaScoreGetter, r.server, filter, sessions)

	// verify
	r.Require().NoError(err)
	r.Require().Len(sessions, 2)
	r.Assert().InEpsilon(0.3, sessionA.BeforeSessionScore, 0.001)
	r.Assert().EqualValues(1, sessionA.BeforeSessionConvCount)
	r.Assert().InEpsilon(0.4, sessionA.AfterSessionScore, 0.001)
	r.Assert().EqualValues(10, sessionA.AfterSessionConvCount)
	r.Assert().EqualValues(0, sessionB.BeforeSessionScore)
	r.Assert().EqualValues(0, sessionB.BeforeSessionConvCount)
	r.Assert().EqualValues(0, sessionB.AfterSessionScore)
	r.Assert().EqualValues(0, sessionB.AfterSessionConvCount)
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestRetrieveQAScoreStatsForAgentCriterion() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessions := []*coachingSessionCriterion{{
		AgentUserID:         "agent A",
		CoachingSessionID:   "session A",
		CriterionID:         "criterion A",
		ScorecardTemplateID: "scorecard template A",
		CoachingSessionAt:   fn.Must(time.Parse(time.DateOnly, "2020-01-10")),
	}}
	timeRange := &analytics.TimeRange{
		StartTimestamp: timestamppb.New(getEndOfDay(sessions[0].CoachingSessionAt.AddDate(0, 0, -1)).AddDate(0, 0, -7)),
		EndTimestamp:   timestamppb.New(getEndOfDay(sessions[0].CoachingSessionAt.AddDate(0, 0, -1))),
	}
	groupBy := []analytics.QAAttributeType{analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT, analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION}
	scorecardTemplateNames := []*coaching.ScorecardTemplateName{{
		CustomerID:                  r.customerID,
		ProfileID:                   r.profileID,
		ScorecardTemplateID:         "scorecard template A",
		ScorecardTemplateRevisionID: "revision 1",
	}}
	filter := &retrieveCoachingEfficiencyStatsFilter{
		ScorecardTemplateNames: fn.Map(scorecardTemplateNames, func(n *coaching.ScorecardTemplateName) string { return n.String() }),
		CriterionIDs:           []string{sessions[0].CriterionID, "criterion B"},
	}
	// set up the mock call
	users := fn.Map(sessions, func(c *coachingSessionCriterion) *userpb.User {
		return &userpb.User{
			Name: userpb.UserName{CustomerID: parent.CustomerID, UserID: c.AgentUserID}.String(),
		}
	})
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, &analytics.RetrieveQAScoreStatsRequest{
		Parent:            parent.String(),
		FilterByTimeRange: timeRange,
		FilterByAttribute: &analytics.QAAttribute{
			Users:                users,
			CriterionIdentifiers: []string{sessions[0].CriterionID},
			ScorecardTemplates:   filter.ScorecardTemplateNames,
		},
		GroupByAttributeTypes:        groupBy,
		RespectTemplateQaScoreConfig: true,
	}).Return(&analytics.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analytics.QAScoreResult{
			Scores: []*analytics.QAScore{{
				Score: 0.3,
				GroupedBy: &analytics.QAScoreGroupBy{
					User:                &userpb.User{Name: users[0].Name},
					CriterionId:         sessions[0].CriterionID,
					ScorecardTemplateId: sessions[0].ScorecardTemplateID,
				},
				TotalConversationCount: 1,
			}},
		},
	}, nil)

	// When
	err := retrieveQAScoreStatsForSameDayAgentCriterion(
		r.ctx, r.server.logger, &qaScoreGetter, parent,
		sessions, filter, sessions[0].CoachingSessionAt, true,
	)

	// verify
	r.Require().NoError(err)
	r.Assert().InEpsilon(float32(0.3), sessions[0].BeforeSessionScore, 0.001)
	r.Assert().Equal(int32(1), sessions[0].BeforeSessionConvCount)
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupByGroups() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}
	groupA := userpb.GroupMembership{
		Group:      userpb.GroupName{CustomerID: parent.CustomerID, GroupID: "groupA-" + uuid.NewString()}.String(),
		IsIndirect: false,
	}
	groupB := userpb.GroupMembership{
		Group:      userpb.GroupName{CustomerID: parent.CustomerID, GroupID: "groupB-" + uuid.NewString()}.String(),
		IsIndirect: false,
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames:  []string{sessionAAgentName.String(), sessionBAgentName.String()},
			GroupNames: []string{groupA.Group, groupB.Group},
			UserNameToDirectGroupNames: map[string][]string{
				sessionAAgentName.String(): {groupA.Group},
				sessionBAgentName.String(): {groupB.Group},
			},
			UserNameToAllGroupNames: map[string][]string{
				sessionAAgentName.String(): {groupA.Group},
				sessionBAgentName.String(): {groupB.Group},
			},
			GroupNameToDirectMembers: map[string][]string{
				groupA.Group: {sessionAAgentName.String()},
				groupB.Group: {sessionBAgentName.String()},
			},
			GroupNameToAllMembers: map[string][]string{
				groupA.Group: {sessionAAgentName.String()},
				groupB.Group: {sessionBAgentName.String()},
			},
		},
		UsecaseIDs: []string{r.usecaseID},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_GROUP},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.4), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.Groups, 1)
		switch stats.Attribute.Groups[0].Name {
		case groupA.Group:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case groupB.Group:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
		default:
			r.FailNowf("can't find group", stats.Attribute.Groups[0].Name)
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupByGroupsAndTimeRange() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}
	groupA := userpb.GroupMembership{
		Group:      userpb.GroupName{CustomerID: parent.CustomerID, GroupID: "groupA-" + uuid.NewString()}.String(),
		IsIndirect: false,
	}
	groupB := userpb.GroupMembership{
		Group:      userpb.GroupName{CustomerID: parent.CustomerID, GroupID: "groupB-" + uuid.NewString()}.String(),
		IsIndirect: false,
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames:  []string{sessionAAgentName.String(), sessionBAgentName.String()},
			GroupNames: []string{groupA.Group, groupB.Group},
			UserNameToDirectGroupNames: map[string][]string{
				sessionAAgentName.String(): {groupA.Group},
				sessionBAgentName.String(): {groupB.Group},
			},
			UserNameToAllGroupNames: map[string][]string{
				sessionAAgentName.String(): {groupA.Group},
				sessionBAgentName.String(): {groupB.Group},
			},
			GroupNameToDirectMembers: map[string][]string{
				groupA.Group: {sessionAAgentName.String()},
				groupB.Group: {sessionBAgentName.String()},
			},
			GroupNameToAllMembers: map[string][]string{
				groupA.Group: {sessionAAgentName.String()},
				groupB.Group: {sessionBAgentName.String()},
			},
		},
		UsecaseIDs: []string{r.usecaseID},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_GROUP, analytics.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE},
		Frequency:             analytics.Frequency_MONTHLY,
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}},
			},
		}, nil).Once()

	const date = "2020-01-01"

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.4), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.Groups, 1)
		switch attr := struct {
			groups    []*userpb.Group
			timeRange []*analytics.TimeRange
		}{
			groups:    stats.Attribute.Groups,
			timeRange: stats.Attribute.TimeRanges,
		}; {
		case attr.groups[0].Name == groupA.Group && attr.timeRange[0].StartTimestamp.AsTime().Format(time.DateOnly) == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())

		case attr.groups[0].Name == groupB.Group && attr.timeRange[0].StartTimestamp.AsTime().Format(time.DateOnly) == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())

		default:
			r.FailNowf("can't find group %v and time range %v", stats.Attribute.Groups[0].Name, stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly))
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupByAgents() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}

	// Test case A
	// Agent A has a coaching plan with focus criteria, and coaching session
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	coachAName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach A",
	}

	agentAName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}

	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}

	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            agentAName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachAName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}

	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	// Test case B
	// Agent B has a coaching plan with focus criteria, but no coaching session
	agentBName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}

	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanB.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	// Test case C
	// Agent C has a coaching plan with coaching session, but no focus criteria
	coachCName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach C",
	}

	agentCName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent C",
	}

	dbPlanC := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planC-" + uuid.NewString(),
	}

	dbSessionC := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionC-" + uuid.NewString(),
		AgentUserID:            agentCName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanC.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachCName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				agentAName.String(),
				agentBName.String(),
				agentCName.String(),
			},
		},
	}

	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbPlanC, &dbSessionC))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_AGENT},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
		// Backend auto-computes includeSessionsWithoutFocusCriteria based on filters
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{agentAName.UserID},
		[]string{dbTargetA.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{agentAName.UserID},
		[]string{dbTargetA.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	// Only mock QA scores for Agent A (who has focus_criteria_ids)
	// Agent C has no focus_criteria_ids, so no QA scores will be queried for them
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 20,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.1), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.Users, 1)
		switch stats.Attribute.Users[0].Name {
		case agentAName.String():
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().EqualValues(false, stats.IsEfficiencyScoreInvalid)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case agentCName.String():
			r.Assert().True(stats.IsEfficiencyScoreInvalid)
		default:
			r.Assert().FailNow("unexpected agent", stats.Attribute.Users[0].Name)
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupByAgentsAndTimeRange() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}

	// Test case A
	// Agent A has a coaching plan with focus criteria, and coaching session
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	coachAName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach A",
	}

	agentAName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}

	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}

	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            agentAName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachAName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}

	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	// Test case B
	// Agent B has a coaching plan with focus criteria, but no coaching session
	agentBName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}

	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanB.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	// Test case C
	// Agent C has a coaching plan with coaching session, but no focus criteria
	coachCName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach C",
	}

	agentCName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent C",
	}

	dbPlanC := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planC-" + uuid.NewString(),
	}

	dbSessionC := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionC-" + uuid.NewString(),
		AgentUserID:            agentCName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanC.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachCName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				agentAName.String(),
				agentBName.String(),
				agentCName.String(),
			},
		},
	}

	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbPlanC, &dbSessionC))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_AGENT, analytics.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE},
		Frequency:             analytics.Frequency_MONTHLY,
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
		// Backend auto-computes includeSessionsWithoutFocusCriteria based on filters
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{agentAName.UserID},
		[]string{dbTargetA.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{agentAName.UserID},
		[]string{dbTargetA.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	// Only mock QA scores for Agent A (who has focus_criteria_ids)
	// Agent C has no focus_criteria_ids, so no QA scores will be queried for them
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 20,
				}},
			},
		}, nil).Once()

	const date = "2020-01-01"

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.1), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.Users, 1)
		switch attr := struct {
			agentName string
			timeRange string
		}{
			agentName: stats.Attribute.Users[0].Name,
			timeRange: stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly),
		}; {
		case attr.agentName == agentAName.String() && attr.timeRange == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().EqualValues(false, stats.IsEfficiencyScoreInvalid)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case attr.agentName == agentCName.String() && attr.timeRange == date:
			r.Assert().True(stats.IsEfficiencyScoreInvalid)
		default:
			r.FailNowf("unexpected agent", "%v and time range %v", stats.Attribute.Users[0].Name, stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly))
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupByCoaches() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	coachAName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach A",
	}
	coachBName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach B",
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				sessionAAgentName.String(),
				sessionBAgentName.String(),
			},
		},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachAName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachBName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION_SUBMITTER},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.4), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.Users, 1)
		switch stats.Attribute.Users[0].Name {
		case coachAName.String():
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case coachBName.String():
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
		default:
			r.Assert().FailNow("unexpected agent", stats.Attribute.Users[0].Name)
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupByCoachesAndTimeRange() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	coachAName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach A",
	}
	coachBName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "coach B",
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				sessionAAgentName.String(),
				sessionBAgentName.String(),
			},
		},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachAName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: coachBName.UserID, Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION_SUBMITTER, analytics.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE},
		Frequency:             analytics.Frequency_MONTHLY,
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}},
			},
		}, nil).Once()

	const date = "2020-01-01"

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.4), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.Users, 1)
		switch attr := struct {
			coachName string
			timeRange string
		}{
			coachName: stats.Attribute.Users[0].Name,
			timeRange: stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly),
		}; {
		case attr.coachName == coachAName.String() && attr.timeRange == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case attr.coachName == coachBName.String() && attr.timeRange == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
		default:
			r.FailNowf("unexpected results", "unexpected coach %v and time range %v", stats.Attribute.Users[0].Name, stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly))
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupBySessions() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				sessionAAgentName.String(),
				sessionBAgentName.String(),
			},
		},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.4), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	sessionAName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionA.ResourceID,
	}.String()
	sessionBName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionB.ResourceID,
	}.String()
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.CoachingSessionNames, 1)
		switch stats.Attribute.CoachingSessionNames[0] {
		case sessionAName:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case sessionBName:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
		default:
			r.Assert().FailNow("unexpected session", stats.Attribute.CoachingSessionNames[0])
		}
	}
}

// TestGetCoachingEfficiencySharedCriterionAcrossTemplates covers the CONVI-7068 case: two
// templates share one criterion id, with one agent coached on each. The QA scores differ only
// by scorecard_template_id, so each session must get its own template's score rather than
// collapsing to a single value.
func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencySharedCriterionAcrossTemplates() {
	// Given
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	agentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentName.String()},
		},
	}

	sharedCriterionID := "shared-" + uuid.NewString()
	dbTemplateX := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateX-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: sharedCriterionID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbTemplateY := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateY-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: sharedCriterionID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}

	sessionAt := fn.Must(time.Parse(time.DateOnly, "2020-01-10"))
	dbPlanX := model.CoachingPlans{Customer: r.customerID, Profile: r.profileID, ResourceID: "planX-" + uuid.NewString()}
	dbSessionX := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionX-" + uuid.NewString(),
		AgentUserID:            agentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanX.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: sessionAt},
		ManagerSubmitterUserID: sql.NullString{String: "coachX-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: sessionAt},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbTemplateX.ResourceID + "/" + sharedCriterionID},
	}
	dbTargetX := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetX-" + uuid.NewString(),
		ScorecardTemplateID:  dbTemplateX.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: sharedCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanX.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	dbPlanY := model.CoachingPlans{Customer: r.customerID, Profile: r.profileID, ResourceID: "planY-" + uuid.NewString()}
	dbSessionY := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionY-" + uuid.NewString(),
		AgentUserID:            agentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanY.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: sessionAt},
		ManagerSubmitterUserID: sql.NullString{String: "coachY-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: sessionAt},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbTemplateY.ResourceID + "/" + sharedCriterionID},
	}
	dbTargetY := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetY-" + uuid.NewString(),
		ScorecardTemplateID:  dbTemplateY.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: sharedCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanY.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	r.T().Cleanup(r.CreateModels(&dbTemplateX, &dbTemplateY, &dbPlanX, &dbTargetX, &dbSessionX, &dbPlanY, &dbTargetY, &dbSessionY))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(sessionAt.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(sessionAt.AddDate(0, 0, 1)),
		},
	}

	// Both sessions are the same agent on the same criterion id, so the batched QA request
	// carries one agent and one criterion; only scorecard_template_id separates the two scores.
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{agentName.UserID},
		[]string{sharedCriterionID},
		getEndOfDay(sessionAt.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{agentName.UserID},
		[]string{sharedCriterionID},
		getEndOfDay(sessionAt.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentName.String()},
						CriterionId:         sharedCriterionID,
						ScorecardTemplateId: dbTemplateX.ResourceID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentName.String()},
						CriterionId:         sharedCriterionID,
						ScorecardTemplateId: dbTemplateY.ResourceID,
					},
					TotalConversationCount: 1,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.5,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentName.String()},
						CriterionId:         sharedCriterionID,
						ScorecardTemplateId: dbTemplateX.ResourceID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.8,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentName.String()},
						CriterionId:         sharedCriterionID,
						ScorecardTemplateId: dbTemplateY.ResourceID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)

	// Then
	r.Require().NoError(err)
	r.Len(qaScoreGetter.Calls, 2)
	r.Len(response.CoachingEfficiencyStats, 2)
	sessionXName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionX.ResourceID,
	}.String()
	sessionYName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionY.ResourceID,
	}.String()
	for _, stats := range response.CoachingEfficiencyStats {
		r.Require().Len(stats.Attribute.CoachingSessionNames, 1)
		switch stats.Attribute.CoachingSessionNames[0] {
		case sessionXName:
			// templateX: 0.5 - 0.3
			r.Assert().InEpsilon(0.2, stats.CoachingEfficiency, 0.001)
		case sessionYName:
			// templateY: 0.8 - 0.4
			r.Assert().InEpsilon(0.4, stats.CoachingEfficiency, 0.001)
		default:
			r.Assert().FailNow("unexpected session", stats.Attribute.CoachingSessionNames[0])
		}
	}
	// ((0.5-0.3)*10 + (0.8-0.4)*10) / (10+10)
	r.Assert().InEpsilon(0.3, response.TotalCoachingEfficiency.CoachingEfficiency, 0.001)
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupBySessionsAndTimeRange() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				sessionAAgentName.String(),
				sessionBAgentName.String(),
			},
		},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "criterionA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}
	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbPlanB, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION, analytics.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE},
		Frequency:             analytics.Frequency_MONTHLY,
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}},
			},
		}, nil).Once()

	const date = "2020-01-01"

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.4), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.001))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 2)
	sessionAName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionA.ResourceID,
	}.String()
	sessionBName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionB.ResourceID,
	}.String()
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.CoachingSessionNames, 1)
		switch attr := struct {
			sessionName string
			timeRange   string
		}{
			sessionName: stats.Attribute.CoachingSessionNames[0],
			timeRange:   stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly),
		}; {
		case attr.sessionName == sessionAName && attr.timeRange == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
		case attr.sessionName == sessionBName && attr.timeRange == date:
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
			r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
			r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
		default:
			r.FailNowf("unexpected results", "unexpected session %v and time range %v", stats.Attribute.CoachingSessionNames[0], stats.Attribute.TimeRanges[0].StartTimestamp.AsTime().Format(time.DateOnly))
		}
	}
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGetCoachingEfficiencyGroupBySessionsAndCriterion() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	sessionAAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}
	sessionBAgentName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent B",
	}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{
				sessionAAgentName.String(),
				sessionBAgentName.String(),
			},
		},
	}
	criterionAID := "criterionA-" + uuid.NewString()
	criterionBID := "criterionB-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}, {
			ID: criterionBID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionA := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionA-" + uuid.NewString(),
		AgentUserID:            sessionAAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID, dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetA := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetA-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionAID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	dbSessionB := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionB-" + uuid.NewString(),
		AgentUserID:            sessionBAgentName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachB-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + criterionAID, dbScorecardTemplate.ResourceID + "/" + criterionBID},
	}
	dbTargetB := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetB-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: criterionBID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetA, &dbSessionA, &dbTargetB, &dbSessionB))
	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION, analytics.AttributeType_ATTRIBUTE_TYPE_CRITERION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionA.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// set up the mock call of RetrieveQAScoreStats
	// for session A
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{dbSessionA.AgentUserID, dbSessionB.AgentUserID},
		[]string{dbTargetA.CriterionOrChapterID.String, dbTargetB.CriterionOrChapterID.String},
		getEndOfDay(dbSessionA.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}, {
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 1,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 3,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 30,
				}, {
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionAAgentName.String()},
						CriterionId:         dbTargetB.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetB.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}, {
					Score: 0.9,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: sessionBAgentName.String()},
						CriterionId:         dbTargetA.CriterionOrChapterID.String,
						ScorecardTemplateId: dbTargetA.ScorecardTemplateID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)
	r.Require().NoError(err)
	r.Require().InEpsilon(float32(0.37), response.TotalCoachingEfficiency.CoachingEfficiency, float64(0.01))
	r.Require().Equal(int32(2), response.TotalCoachingEfficiency.TotalNumberOfSessions)
	r.Len(qaScoreGetter.Calls, 2)

	r.Len(response.CoachingEfficiencyStats, 6)
	sessionAName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionA.ResourceID,
	}.String()
	sessionBName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionB.ResourceID,
	}.String()
	for _, stats := range response.CoachingEfficiencyStats {
		r.Len(stats.Attribute.CoachingSessionNames, 1)
		r.Assert().LessOrEqual(len(stats.Attribute.CriterionIdentifiers), 1)
		switch stats.Attribute.CoachingSessionNames[0] {
		case sessionAName:
			if len(stats.Attribute.CriterionIdentifiers) == 0 {
				r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
				r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
				r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
			} else {
				switch stats.Attribute.CriterionIdentifiers[0] {
				case criterionAID:
					r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
					r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
					r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
				case criterionBID:
					r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
					r.Assert().InEpsilon(0.1, stats.CoachingEfficiency, 0.001)
					r.Assert().Equal(dbSessionA.SessionAt.Time, stats.LastSessionDate.AsTime())
				}
			}
		case sessionBName:
			if len(stats.Attribute.CriterionIdentifiers) == 0 {
				r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
				r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
				r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
			} else {
				switch stats.Attribute.CriterionIdentifiers[0] {
				case criterionAID:
					r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
					r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
					r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
				case criterionBID:
					r.Assert().EqualValues(1, stats.TotalNumberOfSessions)
					r.Assert().InEpsilon(0.5, stats.CoachingEfficiency, 0.001)
					r.Assert().Equal(dbSessionB.SessionAt.Time, stats.LastSessionDate.AsTime())
				}
			}
		default:
			r.Assert().FailNow("unexpected session", stats.Attribute.CoachingSessionNames[0])
		}
	}
}

func buildMockRequest(
	parent *shared.CustomerProfile,
	agentUserIDs, criterionIDs []string,
	startTime time.Time,
	filter *retrieveCoachingEfficiencyStatsFilter,
) *analytics.RetrieveQAScoreStatsRequest {
	users := fn.Map(agentUserIDs, func(s string) *userpb.User {
		return &userpb.User{
			Name: userpb.UserName{
				CustomerID: parent.CustomerID,
				UserID:     s,
			}.String(),
		}
	})

	request := &analytics.RetrieveQAScoreStatsRequest{
		Parent: parent.String(),
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(startTime),
			EndTimestamp:   timestamppb.New(startTime.AddDate(0, 0, 7)),
		},
		FilterByAttribute: &analytics.QAAttribute{
			Users:                users,
			CriterionIdentifiers: criterionIDs,
			ScorecardTemplates:   filter.ScorecardTemplateNames,
		},
		GroupByAttributeTypes: []analytics.QAAttributeType{
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
			analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
		},
		RespectTemplateQaScoreConfig: true,
	}
	return request
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestFilterByCriterionCategoryOutcome() {
	// Given
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	agentAUserName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}

	outcomeCriterionID := "outcome-" + uuid.NewString()
	performanceCriterionID := "performance-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID:                  outcomeCriterionID,
			ExcludeFromQAScores: true,
			IsOutcome:           true,
		}, {
			ID:                  performanceCriterionID,
			ExcludeFromQAScores: false,
			IsOutcome:           false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionOutcome := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionOutcome-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + outcomeCriterionID},
	}
	dbTargetOutcome := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetOutcome-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: outcomeCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbSessionPerformance := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionPerformance-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + performanceCriterionID},
	}
	dbTargetPerformance := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetPerformance-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: performanceCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetOutcome, &dbSessionOutcome, &dbTargetPerformance, &dbSessionPerformance))

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentAUserName.String()},
		},
		CriterionCategories: []analytics.Attribute_CriterionCategory{
			analytics.Attribute_CRITERION_CATEGORY_OUTCOME,
		},
	}

	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionOutcome.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionOutcome.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{agentAUserName.UserID},
		[]string{outcomeCriterionID},
		getEndOfDay(dbSessionOutcome.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{agentAUserName.UserID},
		[]string{outcomeCriterionID},
		getEndOfDay(dbSessionOutcome.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         outcomeCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 1,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         outcomeCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)

	// Then
	r.Require().NoError(err)
	r.Len(response.CoachingEfficiencyStats, 1, "Should only return outcome criterion session")
	sessionOutcomeName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionOutcome.ResourceID,
	}.String()
	r.Assert().Equal(sessionOutcomeName, response.CoachingEfficiencyStats[0].Attribute.CoachingSessionNames[0])
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestFilterByCriterionCategoryPerformance() {
	// Given
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	agentAUserName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}

	outcomeCriterionID := "outcome-" + uuid.NewString()
	performanceCriterionID := "performance-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID:                  outcomeCriterionID,
			ExcludeFromQAScores: true,
			IsOutcome:           true,
		}, {
			ID:                  performanceCriterionID,
			ExcludeFromQAScores: false,
			IsOutcome:           false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	dbSessionOutcome := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionOutcome-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + outcomeCriterionID},
	}
	dbTargetOutcome := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetOutcome-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: outcomeCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	dbSessionPerformance := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionPerformance-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + performanceCriterionID},
	}
	dbTargetPerformance := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetPerformance-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: performanceCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetOutcome, &dbSessionOutcome, &dbTargetPerformance, &dbSessionPerformance))

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentAUserName.String()},
		},
		CriterionCategories: []analytics.Attribute_CriterionCategory{
			analytics.Attribute_CRITERION_CATEGORY_PERFORMANCE,
		},
	}

	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionPerformance.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionPerformance.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{agentAUserName.UserID},
		[]string{performanceCriterionID},
		getEndOfDay(dbSessionPerformance.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{agentAUserName.UserID},
		[]string{performanceCriterionID},
		getEndOfDay(dbSessionPerformance.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         performanceCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 1,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         performanceCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)

	// Then
	r.Require().NoError(err)
	r.Len(response.CoachingEfficiencyStats, 1, "Should only return performance criterion session")
	sessionPerformanceName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionPerformance.ResourceID,
	}.String()
	r.Assert().Equal(sessionPerformanceName, response.CoachingEfficiencyStats[0].Attribute.CoachingSessionNames[0])
}

func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestFilterByCriterionCategoryPerformanceExcludesNonEvaluable() {
	// Given: Test that non-evaluable performance criteria are filtered out when requesting performance category
	// This tests the new logic: if !criteria.IsEvaluable && criterionCategory == CRITERION_CATEGORY_PERFORMANCE { continue }
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	agentAUserName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}

	evaluablePerformanceCriterionID := "evaluable-performance-" + uuid.NewString()
	nonEvaluablePerformanceCriterionID := "non-evaluable-performance-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID:                  evaluablePerformanceCriterionID,
			ExcludeFromQAScores: false,
			IsOutcome:           false,
		}, {
			ID:                  nonEvaluablePerformanceCriterionID,
			ExcludeFromQAScores: true,
			IsOutcome:           false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	// Session with evaluable performance criterion (should be included)
	dbSessionEvaluablePerformance := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionEvaluablePerformance-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + evaluablePerformanceCriterionID},
	}
	dbTargetEvaluablePerformance := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetEvaluablePerformance-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: evaluablePerformanceCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	// Session with non-evaluable performance criterion (should be filtered out)
	dbSessionNonEvaluablePerformance := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionNonEvaluablePerformance-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + nonEvaluablePerformanceCriterionID},
	}
	dbTargetNonEvaluablePerformance := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetNonEvaluablePerformance-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: nonEvaluablePerformanceCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}
	r.T().Cleanup(r.CreateModels(&dbScorecardTemplate, &dbPlanA, &dbTargetEvaluablePerformance, &dbSessionEvaluablePerformance, &dbTargetNonEvaluablePerformance, &dbSessionNonEvaluablePerformance))

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentAUserName.String()},
		},
		CriterionCategories: []analytics.Attribute_CriterionCategory{
			analytics.Attribute_CRITERION_CATEGORY_PERFORMANCE,
		},
	}

	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionEvaluablePerformance.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionEvaluablePerformance.SessionAt.Time.AddDate(0, 0, 1)),
		},
	}

	// Only expect QA score requests for evaluable performance criterion
	mockBeforeScoreRequest := buildMockRequest(
		parent,
		[]string{agentAUserName.UserID},
		[]string{evaluablePerformanceCriterionID},
		getEndOfDay(dbSessionEvaluablePerformance.SessionAt.Time.AddDate(0, 0, -1)).AddDate(0, 0, -7),
		filter,
	)
	mockAfterScoreRequest := buildMockRequest(
		parent,
		[]string{agentAUserName.UserID},
		[]string{evaluablePerformanceCriterionID},
		getEndOfDay(dbSessionEvaluablePerformance.SessionAt.Time.AddDate(0, 0, -1)),
		filter,
	)
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockBeforeScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         evaluablePerformanceCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 1,
				}},
			},
		}, nil).Once()
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mockAfterScoreRequest).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         evaluablePerformanceCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once()

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)

	// Then
	r.Require().NoError(err)
	r.Len(response.CoachingEfficiencyStats, 1, "Should only return evaluable performance criterion session, non-evaluable should be filtered out")
	sessionEvaluablePerformanceName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionEvaluablePerformance.ResourceID,
	}.String()
	r.Assert().Equal(sessionEvaluablePerformanceName, response.CoachingEfficiencyStats[0].Attribute.CoachingSessionNames[0])
	r.Assert().InEpsilon(0.1, response.CoachingEfficiencyStats[0].CoachingEfficiency, 0.001)
}

// TestFilterByCriterionCategoryWithIncludeSessionsWithoutFocusCriteria tests the fix for the Vivint bug
// where sessions without focus criteria were incorrectly filtered out when CriterionCategories filter was present.
// This test verifies that:
// 1. Sessions WITH focus criteria matching the category filter are included
// 2. Sessions WITHOUT focus criteria are also included when includeSessionsWithoutFocusCriteria=true
// 3. Sessions with focus criteria NOT matching the category filter are excluded
// 4. Sessions without focus criteria have IsEfficiencyScoreInvalid=true
func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestFilterByCriterionCategoryWithIncludeSessionsWithoutFocusCriteria() {
	// Given
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	agentAUserName := userpb.UserName{
		CustomerID: r.customerID,
		UserID:     "agent A",
	}

	// Create a scorecard template with both performance and outcome criteria
	outcomeCriterionID := "outcome-" + uuid.NewString()
	performanceCriterionID := "performance-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID:                  outcomeCriterionID,
			ExcludeFromQAScores: true,
			IsOutcome:           true,
		}, {
			ID:                  performanceCriterionID,
			ExcludeFromQAScores: false,
			IsOutcome:           false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}

	dbPlanA := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}

	// Plan B has NO targets - used for session without criteria to avoid fallback to targets
	dbPlanB := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planB-" + uuid.NewString(),
	}

	// Session A: WITH performance focus criteria (should be included, has valid efficiency)
	dbSessionWithPerformanceCriteria := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionWithPerformance-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + performanceCriterionID},
	}

	// Session B: WITH outcome focus criteria (should be filtered out by PERFORMANCE category filter)
	dbSessionWithOutcomeCriteria := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionWithOutcome-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{dbScorecardTemplate.ResourceID + "/" + outcomeCriterionID},
	}

	// Session C: WITHOUT any focus criteria (should be included due to flag, but has invalid efficiency)
	// IMPORTANT: Uses dbPlanB which has NO targets, to avoid SQL fallback to targets table
	dbSessionWithoutCriteria := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "sessionWithoutCriteria-" + uuid.NewString(),
		AgentUserID:            agentAUserName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlanB.ResourceID}, // Use planB with no targets
		SessionAt:              sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		ManagerSubmitterUserID: sql.NullString{String: "coachA-" + uuid.NewString(), Valid: true},
		ManagerSubmittedAt:     sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))},
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       []string{}, // Explicitly empty - this is the key part of the test
	}

	// Create targets for sessions with criteria
	dbTargetPerformance := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetPerformance-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: performanceCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	dbTargetOutcome := model.Targets{
		Customer:             r.customerID,
		Profile:              r.profileID,
		TargetID:             "targetOutcome-" + uuid.NewString(),
		ScorecardTemplateID:  dbScorecardTemplate.ResourceID,
		CriterionOrChapterID: sql.NullString{Valid: true, String: outcomeCriterionID},
		CoachingPlanID:       sql.NullString{Valid: true, String: dbPlanA.ResourceID},
		UsecaseID:            r.usecaseID,
	}

	r.T().Cleanup(r.CreateModels(
		&dbScorecardTemplate,
		&dbPlanA,
		&dbPlanB, // Plan with no targets for session without criteria
		&dbTargetPerformance,
		&dbTargetOutcome,
		&dbSessionWithPerformanceCriteria,
		&dbSessionWithOutcomeCriteria,
		&dbSessionWithoutCriteria,
	))

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentAUserName.String()},
		},
		CriterionCategories: []analytics.Attribute_CriterionCategory{
			analytics.Attribute_CRITERION_CATEGORY_PERFORMANCE,
		},
	}

	request := &analytics.RetrieveCoachingEfficiencyStatsRequest{
		GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION},
		FilterByTimeRange: &analytics.TimeRange{
			StartTimestamp: timestamppb.New(dbSessionWithPerformanceCriteria.SessionAt.Time.AddDate(0, 0, -1)),
			EndTimestamp:   timestamppb.New(dbSessionWithPerformanceCriteria.SessionAt.Time.AddDate(0, 0, 1)),
		},
		// Backend auto-computes includeSessionsWithoutFocusCriteria based on filters
	}

	// Mock QA scores only for the session with performance criteria (session A)
	// Session C (without criteria) won't have QA scores since it has no criterion_id
	// Set up two different responses: one for "before" scores and one for "after" scores
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mock.AnythingOfType("*analytics.RetrieveQAScoreStatsRequest")).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.3,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         performanceCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 10,
				}},
			},
		}, nil).Once() // First call - before scores
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mock.AnythingOfType("*analytics.RetrieveQAScoreStatsRequest")).
		Return(&analytics.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analytics.QAScoreResult{
				Scores: []*analytics.QAScore{{
					Score: 0.4,
					GroupedBy: &analytics.QAScoreGroupBy{
						User:                &userpb.User{Name: agentAUserName.String()},
						CriterionId:         performanceCriterionID,
						ScorecardTemplateId: dbScorecardTemplate.ResourceID,
					},
					TotalConversationCount: 20,
				}},
			},
		}, nil).Once() // Second call - after scores

	// When
	response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
		r.ctx, &qaScoreGetter, request, filter, parent)

	// Then
	r.Require().NoError(err)
	r.Require().Len(response.CoachingEfficiencyStats, 2, "Should return 2 sessions: one with performance criteria and one without criteria")

	// Build expected session names for verification
	sessionWithPerformanceName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionWithPerformanceCriteria.ResourceID,
	}.String()
	sessionWithoutCriteriaName := coaching.CoachingSessionName{
		CustomerID:        r.customerID,
		ProfileID:         r.profileID,
		CoachingSessionID: dbSessionWithoutCriteria.ResourceID,
	}.String()

	// Verify both sessions are present and check their properties
	var sessionWithPerformanceStats *analytics.CoachingEfficiencyStats
	var sessionWithoutCriteriaStats *analytics.CoachingEfficiencyStats

	for _, stats := range response.CoachingEfficiencyStats {
		sessionName := stats.Attribute.CoachingSessionNames[0]
		switch sessionName {
		case sessionWithPerformanceName:
			sessionWithPerformanceStats = stats
		case sessionWithoutCriteriaName:
			sessionWithoutCriteriaStats = stats
		default:
			r.FailNowf("Unexpected session returned", "Session: %s", sessionName)
		}
	}

	// Verify session with performance criteria
	r.Require().NotNil(sessionWithPerformanceStats, "Session with performance criteria should be present")
	r.Assert().EqualValues(1, sessionWithPerformanceStats.TotalNumberOfSessions, "Session with performance criteria should count as 1 session")
	r.Assert().InEpsilon(0.1, sessionWithPerformanceStats.CoachingEfficiency, 0.001, "Session with performance criteria should have valid efficiency calculated from QA scores")
	r.Assert().False(sessionWithPerformanceStats.IsEfficiencyScoreInvalid, "Session with performance criteria should have valid efficiency score")

	// Verify session without criteria - THIS IS THE KEY PART OF THE TEST
	r.Require().NotNil(sessionWithoutCriteriaStats, "Session without focus criteria should be present (this was the bug)")
	r.Assert().EqualValues(1, sessionWithoutCriteriaStats.TotalNumberOfSessions, "Session without criteria should count as 1 session")
	r.Assert().True(sessionWithoutCriteriaStats.IsEfficiencyScoreInvalid, "Session without criteria should have invalid efficiency score")
	r.Assert().EqualValues(0, sessionWithoutCriteriaStats.CoachingEfficiency, "Session without criteria should have 0 efficiency")
}

// TestGroupSessionCountsAsOne verifies the "sessions held" counting model: a group (bulk)
// coaching event delivered to multiple agents counts once for the Overview total and for the
// Session Coach leaderboard, while ungrouped 1:1 sessions keep counting individually.
func (r *RetrieveCoachingEfficiencyStatsTestSuite) TestGroupSessionCountsAsOne() {
	qaScoreGetter := analyticsmock.AnalyticsServiceServer{}
	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	coachName := userpb.UserName{CustomerID: r.customerID, UserID: "coach A"}
	agentAName := userpb.UserName{CustomerID: r.customerID, UserID: "agent A"}
	agentBName := userpb.UserName{CustomerID: r.customerID, UserID: "agent B"}
	agentCName := userpb.UserName{CustomerID: r.customerID, UserID: "agent C"}

	filter := &retrieveCoachingEfficiencyStatsFilter{
		UsecaseIDs: []string{r.usecaseID},
		UsersAndGroups: &userfilter.FilteredUsersAndGroups{
			UserNames: []string{agentAName.String(), agentBName.String(), agentCName.String()},
		},
	}

	criterionAID := "criterionA-" + uuid.NewString()
	dbScorecardTemplate := model.ScorecardTemplates{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "templateA-" + uuid.NewString(),
		Revision:   "rv1-" + uuid.NewString(),
		Template: r.generateTemplate([]*criterionItemParams{{
			ID: criterionAID, ExcludeFromQAScores: false, IsOutcome: false,
		}}),
		UsecaseIds: []string{r.usecaseID},
	}
	dbPlan := model.CoachingPlans{
		Customer:   r.customerID,
		Profile:    r.profileID,
		ResourceID: "planA-" + uuid.NewString(),
	}
	sessionAt := sql.NullTime{Valid: true, Time: fn.Must(time.Parse(time.DateOnly, "2020-01-10"))}
	focusCriteria := []string{dbScorecardTemplate.ResourceID + "/" + criterionAID}

	// Two sessions delivered as one bulk event: same coach, different agents, shared group id.
	groupSessionID := uuid.NewString()
	dbGroupSession1 := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "groupSession1-" + uuid.NewString(),
		AgentUserID:            agentAName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlan.ResourceID},
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{Valid: true, String: coachName.UserID},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
		GroupSessionID:         sql.NullString{Valid: true, String: groupSessionID},
	}
	dbGroupSession2 := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "groupSession2-" + uuid.NewString(),
		AgentUserID:            agentBName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlan.ResourceID},
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{Valid: true, String: coachName.UserID},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
		GroupSessionID:         sql.NullString{Valid: true, String: groupSessionID},
	}
	// One ungrouped 1:1 session by the same coach.
	dbIndividualSession := model.CoachingSessions{
		Customer:               r.customerID,
		Profile:                r.profileID,
		ResourceID:             "individualSession-" + uuid.NewString(),
		AgentUserID:            agentCName.UserID,
		CoachingPlanID:         sql.NullString{Valid: true, String: dbPlan.ResourceID},
		SessionAt:              sessionAt,
		ManagerSubmitterUserID: sql.NullString{Valid: true, String: coachName.UserID},
		ManagerSubmittedAt:     sessionAt,
		UsecaseID:              sql.NullString{Valid: true, String: r.usecaseID},
		FocusCriteriaIds:       focusCriteria,
	}
	r.T().Cleanup(r.CreateModels(
		&dbScorecardTemplate, &dbPlan, &dbGroupSession1, &dbGroupSession2, &dbIndividualSession,
	))

	// QA scores are irrelevant to the session count; return empty so efficiency is invalid
	// but every session row still contributes to the count.
	qaScoreGetter.On("RetrieveQAScoreStats", mock.Anything, mock.Anything).
		Return(&analytics.RetrieveQAScoreStatsResponse{QaScoreResult: &analytics.QAScoreResult{}}, nil)

	timeRange := &analytics.TimeRange{
		StartTimestamp: timestamppb.New(sessionAt.Time.AddDate(0, 0, -1)),
		EndTimestamp:   timestamppb.New(sessionAt.Time.AddDate(0, 0, 1)),
	}

	r.Run("session coach leaderboard counts the group event once", func() {
		// When
		response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
			r.ctx, &qaScoreGetter, &analytics.RetrieveCoachingEfficiencyStatsRequest{
				GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_COACHING_SESSION_SUBMITTER},
				FilterByTimeRange:     timeRange,
			}, filter, parent)

		// Then
		r.Require().NoError(err)
		// Overview total: 1 group event + 1 individual = 2 (not 3 sessions).
		r.Require().EqualValues(2, response.TotalCoachingEfficiency.TotalNumberOfSessions)
		r.Require().Len(response.CoachingEfficiencyStats, 1)
		coachStats := response.CoachingEfficiencyStats[0]
		r.Require().Len(coachStats.Attribute.Users, 1)
		r.Assert().Equal(coachName.String(), coachStats.Attribute.Users[0].Name)
		r.Assert().EqualValues(2, coachStats.TotalNumberOfSessions)
	})

	r.Run("agent leaderboard still counts each agent's own session", func() {
		// When
		response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
			r.ctx, &qaScoreGetter, &analytics.RetrieveCoachingEfficiencyStatsRequest{
				GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_AGENT},
				FilterByTimeRange:     timeRange,
			}, filter, parent)

		// Then
		r.Require().NoError(err)
		r.Require().EqualValues(2, response.TotalCoachingEfficiency.TotalNumberOfSessions)
		r.Require().Len(response.CoachingEfficiencyStats, 3)
		for _, stats := range response.CoachingEfficiencyStats {
			r.Assert().EqualValues(1, stats.TotalNumberOfSessions, "each agent counts their own session")
		}
	})

	r.Run("team leaderboard keeps sessions-received (group not collapsed)", func() {
		// Given — all three agents belong to one team.
		teamName := userpb.GroupName{CustomerID: r.customerID, GroupID: "team-" + uuid.NewString()}.String()
		teamFilter := &retrieveCoachingEfficiencyStatsFilter{
			UsecaseIDs: []string{r.usecaseID},
			UsersAndGroups: &userfilter.FilteredUsersAndGroups{
				UserNames:  []string{agentAName.String(), agentBName.String(), agentCName.String()},
				GroupNames: []string{teamName},
				UserNameToAllGroupNames: map[string][]string{
					agentAName.String(): {teamName},
					agentBName.String(): {teamName},
					agentCName.String(): {teamName},
				},
			},
		}

		// When
		response, err := r.server.retrieveCoachingEfficiencyStatsInternal(
			r.ctx, &qaScoreGetter, &analytics.RetrieveCoachingEfficiencyStatsRequest{
				GroupByAttributeTypes: []analytics.AttributeType{analytics.AttributeType_ATTRIBUTE_TYPE_GROUP},
				FilterByTimeRange:     timeRange,
			}, teamFilter, parent)

		// Then
		r.Require().NoError(err)
		// Overview total still collapses the group event: 1 group + 1 individual = 2.
		r.Require().EqualValues(2, response.TotalCoachingEfficiency.TotalNumberOfSessions)
		r.Require().Len(response.CoachingEfficiencyStats, 1)
		teamStats := response.CoachingEfficiencyStats[0]
		// Team counts every session received: 2 from the bulk event + 1 individual = 3.
		r.Assert().EqualValues(3, teamStats.TotalNumberOfSessions, "team uses sessions-received, group is not collapsed")
	})
}
