package analyticsimpl

import (
	"context"
	"database/sql"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	qadbpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/qa"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	AgentQuotaManager "github.com/cresta/go-servers/shared/agent-quota-manager"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/utils"
)

// nolint:gosec
func (a AnalyticsServiceImpl) RetrieveManualQAProgress(
	ctx context.Context, req *analyticspb.RetrieveManualQAProgressRequest,
) (*analyticspb.RetrieveManualQAProgressResponse, error) {
	a.logger.Infof(ctx, "RetrieveManualQAProgress RPC starts, req = %v", req)
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}

	// Validates the parent.
	parent, err := customerpb.ParseCustomerOrProfileName(req.Parent)
	if err != nil {
		return nil, err
	}

	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	// Get resource ACL based on the caller identity.
	req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(ctx, a.configClient, parent.CustomerID, a.resourceACLHelperProvider.Get(), req.FilterByAttribute.Users, req.FilterByAttribute.Groups, false)
	if err != nil {
		return nil, err
	}

	if req.FilterByAttribute != nil && (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
		includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
		filterByAttribute, err := shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			req.FilterByAttribute,
			includeDirectGroupMembershipsOnly,
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		if len(filterByAttribute.Users) == 0 {
			return &analyticspb.RetrieveManualQAProgressResponse{}, nil
		}
		req.FilterByAttribute = filterByAttribute
	}

	err = validateRetrieveManualQAProgressRequest(req)
	if err != nil {
		return nil, err
	}

	db := a.appsDB.DB(ctx)

	// Parses the QA analyst.
	var qaUserName *userpb.UserName
	if req.GetQaAnalyst() != nil {
		qaUserName, err = userpb.ParseUserName(req.GetQaAnalyst().Name)
		if err != nil {
			return nil, err
		}
	}

	// Finds the evaluation period.
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.FilterByAttribute.ScorecardTemplateNames[0])
	if err != nil {
		return nil, err
	}
	taskID := ""
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err := coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
		taskID = taskName.DirectorTaskID
	}
	templates, err := qa.ListScorecardTemplates(
		db,
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
		scorecardTemplateName.ScorecardTemplateID,
		req.FilterByAttribute.UsecaseNames)
	if err != nil {
		return nil, err
	}
	qaTaskConfigs, err := qa.ListQATaskConfigs(db, scorecardTemplateName.CustomerID, scorecardTemplateName.ProfileID, templates, taskID)
	if err != nil {
		return nil, err
	}

	var periods []*qa.EvaluationPeriod
	periods, err = qa.ListEvaluationPeriodsForTask(db, scorecardTemplateName.CustomerID, scorecardTemplateName.ProfileID, req.FilterByAttribute.UsecaseNames, taskID)
	if err != nil {
		return nil, err
	}
	if len(periods) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no evaluation period is found for the scorecard template.")
	}
	var period *qa.EvaluationPeriod
	if len(req.FilterByEvaluationPeriods) > 0 {
		period = FindEvaluationPeriod(req.FilterByEvaluationPeriods[0], periods)
		if period == nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to find the corresponding config for the invalid evaluation period: %v", req.FilterByEvaluationPeriods[0])
		}
	} else {
		// Return the current period if no evaluation period is specified for its config
		period = periods[len(periods)-1]
	}

	// Lists the template audiences.
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	var taskConfig *qadbpb.QATaskConfig
	if len(qaTaskConfigs) != 0 {
		taskConfig = qaTaskConfigs[0]
	}

	resolvedTasksUsers, err := qa.ResolveTaskUsersFromConfig(ctx, userClient, parent, templates[0], taskConfig)
	if err != nil {
		return nil, err
	}

	// Lists the filtered agent/groups.
	filteredAgents, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	if len(req.FilterByAttribute.TaskNames) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.FilterByAttribute.TaskNames[0])
		if err != nil {
			return nil, err
		}
	}

	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}

	assignCoverageConversations := len(req.FilterByEvaluationPeriods) > 0 && qa.IsConversationCoverageTask(period.QATaskConfig.GetTaskCompleteConfig())
	countEligibleConversations := req.ReturnHandledConversationsCount || assignCoverageConversations
	errorChan := make(chan error, 1)
	var agentUserIDConversationsCounts map[string]int32
	if countEligibleConversations {
		// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
		// Only this branch issues a conversations-v2 ES read (countEligibleConversationsByAgent).
		recordConversationReadRecency(ctx, req.GetFilterByTimeRange().GetStartTimestamp(), parent.CustomerID, parent.ProfileID, "retrieve_manual_qa_progress")
		go utils.AvoidPanic(ctx, func() {
			agentUsers := resolvedTasksUsers
			if len(filteredAgents) > 0 {
				agentUsers = filteredAgents
			}
			counts, err := a.countEligibleConversationsByAgent(ctx, profileConfig, parent, templates[0], period, agentUsers)
			agentUserIDConversationsCounts = counts
			errorChan <- err
		})()
	}

	// Queries the scorecards in the evalution period.
	startTime, endTime := computeOverallTimeRange(req.FilterByEvaluationPeriods, req.FilterByTimeRange)
	scorecardDetailsQuery := db.Model(&dbmodel.Scorecards{}).
		Select(`submitted_at as submit_time,
			su.user_id as qa_user_id,
			CASE WHEN su.full_name IS NULL OR su.full_name = '' THEN su.display_name ELSE su.full_name END as qa_display_name,
			scorecards.resource_id as scorecard_id,
			scorecards.conversation_id as conversation_id,
			scorecards.message_ids as message_ids,
			au.user_id as agent_user_id,
			CASE WHEN au.full_name IS NULL OR au.full_name = '' THEN au.display_name ELSE au.full_name END as agent_display_name`).
		Joins("JOIN app.users su ON su.customer_id = scorecards.customer AND scorecards.submitter_user_id = su.user_id").
		Joins("JOIN app.users au ON au.customer_id = scorecards.customer AND scorecards.agent_user_id = au.user_id").
		Where("manually_scored = TRUE").
		Where("scorecards.customer = ?", parent.CustomerID).
		Where("scorecards.profile = ?", parent.ProfileID).
		Where("scorecards.template_id = ?", scorecardTemplateName.ScorecardTemplateID).
		Where("scorecards.calibrated_scorecard_id IS NULL").
		Where("scorecards.scorecard_type IS NULL OR scorecards.scorecard_type = 0")

	scorecardDetailsQuery = scorecardDetailsQuery.Where("submitted_at >= @start_ts_sec AND submitted_at <= @end_ts_sec",
		sql.Named("start_ts_sec", startTime),
		sql.Named("end_ts_sec", endTime))

	if taskName != nil {
		scorecardDetailsQuery = scorecardDetailsQuery.Where("? = ANY(scorecards.task_ids)", taskName.DirectorTaskID)
	}

	if len(usecaseIDs) > 0 {
		scorecardDetailsQuery = scorecardDetailsQuery.Where("scorecards.usecase_id IN ?", usecaseIDs)
	}

	if len(filteredAgents) > 0 {
		scorecardDetailsQuery = scorecardDetailsQuery.Where("scorecards.agent_user_id IN ?", fn.MapKeyToSlice(filteredAgents))
	}

	var scorecardDetailsRows []rowScorecardDetails
	err = scorecardDetailsQuery.Scan(&scorecardDetailsRows).Error
	if err != nil {
		return nil, err
	}

	assignFixedQuota := len(req.FilterByEvaluationPeriods) > 0 && !assignCoverageConversations
	var quotaManager *AgentQuotaManager.AgentQuotaManager
	if assignFixedQuota {
		quotaManager, err = AgentQuotaManager.NewAgentQuotaManager(ctx, parent, userClient, []*qadbpb.TaskCompleteConfig{period.QATaskConfig.TaskCompleteConfig})
		if err != nil {
			return nil, err
		}
	}

	// Aggregates the stats for all agents in the template audiences.
	agentIDStatsMap := make(map[string]*analyticspb.ManualQAProgress)
	for agentID, agentUser := range resolvedTasksUsers {
		if len(filteredAgents) > 0 && filteredAgents[agentID] == nil {
			continue
		}

		stats := &analyticspb.ManualQAProgress{
			Attribute: &analyticspb.Attribute{
				Users: []*userpb.User{agentUser},
			},
		}

		if assignFixedQuota {
			quota, err := quotaManager.GetQuota(agentID, period.QATaskConfig.TaskCompleteConfig)
			if err != nil {
				return nil, err
			}
			stats.AssignedConversationCount = quota
		}
		agentIDStatsMap[agentID] = stats
	}

	for _, scorecardDetails := range scorecardDetailsRows {
		if _, exist := agentIDStatsMap[scorecardDetails.AgentUserID]; !exist {
			continue
		}
		agentStats := agentIDStatsMap[scorecardDetails.AgentUserID]

		agentStats.EvaluatedConversationCount += 1

		submitTime, err := shared.ConvertStringToAPITimestamp(scorecardDetails.SubmitTime)
		if err != nil {
			return nil, err
		}
		if agentStats.GetLastEvaluatedTime() == nil || submitTime.AsTime().After(agentStats.GetLastEvaluatedTime().AsTime()) {
			agentStats.LastQaAnalyst = &userpb.User{
				Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
				DisplayName: scorecardDetails.QaDisplayName,
			}
			agentStats.LastEvaluatedTime = submitTime
		}

		if qaUserName == nil || scorecardDetails.QaUserID == qaUserName.UserID {
			messageNames := []string{}
			if len(scorecardDetails.MessageIds) > 0 {
				for _, messageID := range scorecardDetails.MessageIds {
					messageNames = append(messageNames, conversationpb.ConversationMessageName{
						CustomerID:     parent.CustomerID,
						ProfileID:      parent.ProfileID,
						ConversationID: scorecardDetails.ConversationID,
						MessageID:      messageID,
					}.String())
				}
			}
			agentStats.EvaluatedConversations = append(
				agentStats.EvaluatedConversations,
				&analyticspb.ManualQAConversationInfo{
					Conversation: &conversationpb.Conversation{
						Name: conversationpb.ConversationName{
							CustomerID:     parent.CustomerID,
							ProfileID:      parent.ProfileID,
							ConversationID: scorecardDetails.ConversationID,
						}.String(),
					},
					MessageNames: messageNames,
					ScorecardName: coachingpb.ScorecardName{
						CustomerID:  parent.CustomerID,
						ProfileID:   parent.ProfileID,
						ScorecardID: scorecardDetails.ScorecardID,
					}.String(),
					Agent: &userpb.User{
						Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.AgentUserID}.String(),
						DisplayName: scorecardDetails.AgentDisplayName,
					},
					QaAnalyst: &userpb.User{
						Name:        userpb.UserName{CustomerID: parent.CustomerID, UserID: scorecardDetails.QaUserID}.String(),
						DisplayName: scorecardDetails.QaDisplayName,
					},
				},
			)
		}
	}

	if countEligibleConversations {
		err = <-errorChan
		if err != nil {
			return nil, err
		}
		for agentID, stats := range agentIDStatsMap {
			if req.ReturnHandledConversationsCount {
				stats.HandledConversationsCount = agentUserIDConversationsCounts[agentID]
			}
			if assignCoverageConversations {
				stats.AssignedConversationCount = qa.ConversationCoverageCount(period.QATaskConfig.GetTaskCompleteConfig(), agentUserIDConversationsCounts[agentID])
			}
		}
	}

	stats := fn.MapValueToSlice(agentIDStatsMap)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Attribute.Users[0].Name <= stats[j].Attribute.Users[0].Name
	})
	return &analyticspb.RetrieveManualQAProgressResponse{
		ManualQaProgresses: stats,
	}, nil
}

func validateRetrieveManualQAProgressRequest(req *analyticspb.RetrieveManualQAProgressRequest) error {
	if len(req.FilterByAttribute.ScorecardTemplateNames) != 1 {
		return status.Errorf(codes.InvalidArgument, "single scorecard template name in the filter_by_attribute is required.")
	}
	if len(req.FilterByAttribute.TaskNames) > 1 {
		return status.Errorf(codes.InvalidArgument, "only one or none task name in the filter_by_attribute is supported when multi-task is disabled.")
	}
	return nil
}
