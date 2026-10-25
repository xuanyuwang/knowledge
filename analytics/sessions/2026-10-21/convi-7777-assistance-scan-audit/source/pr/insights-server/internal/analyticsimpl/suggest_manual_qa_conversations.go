package analyticsimpl

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	qapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/qa"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/go-servers/shared/converters/analyticsconverter"
	"github.com/cresta/go-servers/shared/converters/analyticsconverter/generated"
	qaconverter "github.com/cresta/go-servers/shared/converters/qaconverter/generated"
	"github.com/cresta/go-servers/shared/db/condition"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/shared-go/util/envflag"
)

const (
	defaultSuggestedConversationsCount  = 5
	pageSizeRetrieveClosedConversations = 200
)

// draftLockDuration is the window during which an in-progress manual scorecard
// counts toward an agent's completion progress, preventing TaskHome from
// assigning the same agent to a second reviewer while one is already active.
var draftLockDuration = envflag.Duration("QM_TASK_HOME_DRAFT_LOCK_DURATION", time.Hour, "Duration window for counting in-progress manual scorecards toward agent progress in TaskHome suggestions.")

// DisableManualQAConversationsShuffle is a global flag for disabling shuffling to ensure consistent unit test result
var DisableManualQAConversationsShuffle = false

// Template type constants
type templateType int

const (
	templateTypePerMessageEmail templateType = iota
	templateTypePerConversationEmail
	templateTypeChatVoice
)

type agentProgressAndConversations struct {
	AgentUserID                           string
	EvaluatedScorecardCount               int32
	InProgressScorecardCount              int32
	AssignedScorecardCount                int32
	EvaluatedConversationWithMessageNames map[string]bool
	AgentConversationWithMessages         []*conversationWithMessage
	SuggestedConversationWithMessages     map[string]*conversationWithMessage
}

type conversationWithMessage struct {
	Conversation   *conversationpb.Conversation
	ConversationID string
	AgentUserID    string
	Message        *model.Messages
}

// nolint:gosec
func (a AnalyticsServiceImpl) SuggestManualQAConversations(
	ctx context.Context, req *analyticspb.SuggestManualQAConversationsRequest,
) (*analyticspb.SuggestManualQAConversationsResponse, error) {
	a.logger.Infof(ctx, "SuggestManualQAConversations RPC starts, req = %v", req)

	// Validates the parent.
	_, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	db := a.appsDB.DB(ctx)

	// Loads the latest QA task config for the scorecard template.
	scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.ScorecardTemplateName)
	if err != nil {
		return nil, err
	}

	var taskName *coachingpb.DirectorTaskName
	taskID := ""
	if len(req.DirectorTaskName) > 0 {
		taskName, err = coachingpb.ParseDirectorTaskName(req.DirectorTaskName)
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
		[]string{req.UsecaseName})
	if err != nil {
		return nil, err
	}
	var periods []*qa.EvaluationPeriod
	periods, err = qa.ListEvaluationPeriodsForTask(db, scorecardTemplateName.CustomerID, scorecardTemplateName.ProfileID, []string{req.UsecaseName}, taskID)
	if err != nil {
		return nil, err
	}

	if len(periods) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no evaluation period is found for the scorecard template.")
	}

	currentPeriod := periods[len(periods)-1]
	currentTemplate := templates[0]
	templateType := getTemplateType(currentTemplate)

	// Fetches the progress status of the agents.
	request := &analyticspb.RetrieveManualQAProgressRequest{
		Parent: req.Parent,
		FilterByEvaluationPeriods: []*analyticspb.TimeRange{
			{
				StartTimestamp: currentPeriod.ScorecardTemplateEvaluationPeriod.StartTime,
				EndTimestamp:   currentPeriod.ScorecardTemplateEvaluationPeriod.EndTime,
			},
		},
		FilterByAttribute: &analyticspb.Attribute{
			ScorecardTemplateNames: []string{req.ScorecardTemplateName},
			Users:                  req.PrioritizedAgents,
			UsecaseNames:           []string{req.UsecaseName},
		},
	}

	if taskName != nil {
		request.FilterByAttribute.TaskNames = []string{taskName.String()}
	}

	progressResp, err := a.RetrieveManualQAProgress(ctx, request)
	if err != nil {
		return nil, err
	}
	agentProgressAndConversationsMap := make(map[string]*agentProgressAndConversations)
	for _, agentProgress := range progressResp.ManualQaProgresses {
		agentName, err := userpb.ParseUserName(agentProgress.GetAttribute().GetUsers()[0].Name)
		if err != nil {
			return nil, err
		}
		agentUserID := agentName.UserID
		agentProgressAndConversationsMap[agentUserID] = &agentProgressAndConversations{
			AgentUserID:             agentUserID,
			EvaluatedScorecardCount: agentProgress.GetEvaluatedConversationCount(),
			AssignedScorecardCount:  agentProgress.GetAssignedConversationCount(),
		}
		agentProgressAndConversationsMap[agentUserID].EvaluatedConversationWithMessageNames = make(map[string]bool)
		for _, convo := range agentProgress.GetEvaluatedConversations() {
			conversationName, err := conversationpb.ParseConversationName(convo.GetConversation().GetName())
			if err != nil {
				return nil, err
			}
			if len(convo.GetMessageNames()) > 0 {
				for _, messageResourceName := range convo.GetMessageNames() {
					messageName, err := conversationpb.ParseConversationMessageName(messageResourceName)
					if err != nil {
						return nil, err
					}
					agentProgressAndConversationsMap[agentUserID].EvaluatedConversationWithMessageNames[getConversationWithMessageName(
						conversationName.ConversationID, messageName.MessageID)] = true
				}
			} else {
				agentProgressAndConversationsMap[agentUserID].EvaluatedConversationWithMessageNames[getConversationWithMessageName(
					conversationName.ConversationID, "")] = true
			}
		}
		agentProgressAndConversationsMap[agentUserID].SuggestedConversationWithMessages = make(map[string]*conversationWithMessage)
		agentProgressAndConversationsMap[agentUserID].AgentConversationWithMessages = make([]*conversationWithMessage, 0)
	}

	// Count in-progress (draft) manual scorecards updated within the lock window and apply
	// them to each agent's progress. This prevents TaskHome from assigning the same agent
	// to a second reviewer while one is already actively evaluating.
	// Skip when draftLockDuration is 0 (feature disabled).
	var inProgressCounts map[string]int32
	if *draftLockDuration > 0 {
		inProgressCounts, err = a.queryInProgressScorecardCountPerAgent(ctx, scorecardTemplateName)
		if err != nil {
			return nil, err
		}
	}
	for agentUserID, count := range inProgressCounts {
		stats, ok := agentProgressAndConversationsMap[agentUserID]
		if !ok {
			// The agent has an in-progress scorecard for this template but is not part of this
			// task's audience (e.g., the draft belongs to a different task sharing the same
			// template). Skip: we have no quota or candidate conversations for them here.
			a.logger.Infof(ctx, "SuggestManualQAConversations skipping in-progress scorecard for agent %v: not in task audience", agentUserID)
			continue
		}
		stats.InProgressScorecardCount = count
	}

	// The suggestion target.
	targetSuggestedConversationsCount := int(req.SuggestedConversationsCount)
	if targetSuggestedConversationsCount == 0 {
		targetSuggestedConversationsCount = defaultSuggestedConversationsCount
	}

	// Calls the RetrieveClosedConversations RPC to list the agent conversations.
	apiConversationFilters, err := GetQAConversationFilters(currentPeriod, req.PrioritizedAgents, currentTemplate, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	timeRangeTarget := getTimeRangeTarget(currentPeriod, req.ConversationTimeRangeField)

	conversationNamesToSkip := make(map[string]bool)
	for _, conversationName := range req.ConversationsToSkip {
		conversationNamesToSkip[conversationName] = true
	}

	// TODO: skip reading db to load all the conversation details.
	closedConversationsResp, err := a.RetrieveClosedConversations(ctx, &analyticspb.RetrieveClosedConversationsRequest{
		Parent:                        req.Parent,
		FilterByAttribute:             apiConversationFilters,
		PageSize:                      int32(pageSizeRetrieveClosedConversations),
		EnableRandomConversationOrder: true,
		TargetFieldForTimeRange:       timeRangeTarget,
	})
	if err != nil {
		return nil, err
	}

	authedUserID, err := auth.ExtractUserIDFromIncomingUserContext(ctx)
	if err != nil {
		a.logger.Errorf(ctx, "Failed to get principal entity from auth token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid user auth")
	}

	candidateConvoIDs := []string{}
	for _, closedConversation := range closedConversationsResp.GetConversations() {
		conversationName, err := conversationpb.ParseConversationName(closedConversation.Name)
		if err != nil {
			return nil, err
		}
		candidateConvoIDs = append(candidateConvoIDs, conversationName.ConversationID)
	}
	convoIDToScorecardsMap, err := a.queryScorecardsInConvos(ctx, scorecardTemplateName, candidateConvoIDs)
	if err != nil {
		return nil, err
	}

	var convoIDToMessagesMap map[string][]*model.Messages
	if templateType == templateTypePerMessageEmail || templateType == templateTypePerConversationEmail {
		convoIDToMessagesMap, err = a.queryMessagesInConvos(ctx, scorecardTemplateName.CustomerID, scorecardTemplateName.ProfileID, candidateConvoIDs)
		if err != nil {
			return nil, err
		}
	}

	for _, closedConversation := range closedConversationsResp.GetConversations() {
		// Skips the already suggested conversations.
		if conversationNamesToSkip[closedConversation.Name] {
			continue
		}

		conversationName, err := conversationpb.ParseConversationName(closedConversation.Name)
		if err != nil {
			return nil, err
		}
		scorecards := convoIDToScorecardsMap[conversationName.ConversationID]

		switch templateType {
		case templateTypePerMessageEmail:
			// Handles the case of the email template evaluating the message.
			messages := convoIDToMessagesMap[conversationName.ConversationID]
			for _, message := range messages {
				// Skips the visitor messages.
				if !message.AgentUserID.Valid {
					continue
				}
				a.populateConversationWithMessageCandidateForAgent(
					ctx, message.AgentUserID.String, conversationName, closedConversation, message, agentProgressAndConversationsMap, scorecardTemplateName, authedUserID, func() *model.Scorecards {
						return findScorecardWithMessageID(scorecards, message.ConversationMessageID.String)
					})
			}

		case templateTypePerConversationEmail:
			// Handles the case of the template is email template evaluating the conversation.
			// The email conversation can have multiple agents.
			messages := convoIDToMessagesMap[conversationName.ConversationID]
			agentUserIDs := set.New[string]()
			for _, message := range messages {
				if message.AgentUserID.Valid {
					agentUserIDs.Add(message.AgentUserID.String)
				}
			}
			for agentUserID := range agentUserIDs.All() {
				a.populateConversationWithMessageCandidateForAgent(
					ctx, agentUserID, conversationName, closedConversation, nil, agentProgressAndConversationsMap, scorecardTemplateName, authedUserID, func() *model.Scorecards { return findScorecardWithAgentUserID(scorecards, agentUserID) })
			}

		case templateTypeChatVoice:
			// Handles the normal chat/voice template which evaluates the conversation which has only one agent.
			agentUserName, err := userpb.ParseUserName(closedConversation.Agent)
			if err != nil {
				return nil, err
			}
			agentUserID := agentUserName.UserID
			a.populateConversationWithMessageCandidateForAgent(
				ctx, agentUserID, conversationName, closedConversation, nil, agentProgressAndConversationsMap, scorecardTemplateName, authedUserID, func() *model.Scorecards { return findScorecardWithAgentUserID(scorecards, agentUserID) })
		}
	}

	// Shuffles the candidate conversations for each agent.
	if !DisableManualQAConversationsShuffle {
		for _, agentProgressAndConversations := range agentProgressAndConversationsMap {
			rand.Shuffle(len(agentProgressAndConversations.AgentConversationWithMessages), func(i, j int) {
				agentProgressAndConversations.AgentConversationWithMessages[i], agentProgressAndConversations.AgentConversationWithMessages[j] = agentProgressAndConversations.AgentConversationWithMessages[j], agentProgressAndConversations.AgentConversationWithMessages[i]
			})
		}
	}

	suggestedConversations := []*analyticspb.ManualQAConversationInfo{}
	agentConversationsArray := fn.MapValueToSlice(agentProgressAndConversationsMap)
	for i := 0; i < targetSuggestedConversationsCount; i++ {
		suggestedConvoWithMessage := suggestNextConversationWithMessage(agentConversationsArray)
		if suggestedConvoWithMessage == nil {
			break
		}
		convo := &analyticspb.ManualQAConversationInfo{
			Conversation: suggestedConvoWithMessage.Conversation,
			Agent: &userpb.User{Name: userpb.UserName{
				CustomerID: scorecardTemplateName.CustomerID,
				UserID:     suggestedConvoWithMessage.AgentUserID,
			}.String()},
		}
		messageID := ""
		if suggestedConvoWithMessage.Message != nil {
			messageID = suggestedConvoWithMessage.Message.ConversationMessageID.String
			convo.MessageNames = []string{conversationpb.ConversationMessageName{
				CustomerID:     scorecardTemplateName.CustomerID,
				ProfileID:      scorecardTemplateName.ProfileID,
				ConversationID: suggestedConvoWithMessage.ConversationID,
				MessageID:      messageID,
			}.String()}
		}
		suggestedConversations = append(suggestedConversations, convo)
		a.suggestedConvoCache.SetConversationSuggested(
			scorecardTemplateName.ScorecardTemplateID, suggestedConvoWithMessage.ConversationID, messageID, suggestedConvoWithMessage.AgentUserID, authedUserID)

	}

	return &analyticspb.SuggestManualQAConversationsResponse{
		SuggestedConversations: suggestedConversations,
	}, nil
}

func getTimeRangeTarget(
	period *qa.EvaluationPeriod,
	conversationTimeRangeField analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
) analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange {
	if conversationTimeRangeField != analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED {
		return conversationTimeRangeField
	}
	return qaconverter.NewConverter().DBToServiceTargetFieldForTimeRange(period.QATaskConfig.GetConversationSuggestionConfig().GetTargetFieldForTimeRange())
}

func GetQAConversationFilters(
	period *qa.EvaluationPeriod,
	filteredUsers []*userpb.User,
	template *model.ScorecardTemplates,
	filterAttribute *analyticspb.Attribute,
) (*analyticspb.Attribute, error) {
	apiConversationFilters := &analyticspb.Attribute{}
	var err error
	if filterAttribute != nil {
		apiConversationFilters = filterAttribute
	} else {
		dbConversationFilters := period.QATaskConfig.GetConversationSuggestionConfig().GetConversationFilters()
		if dbConversationFilters != nil {
			attributeConverter := analyticsconverter.Converter{GenConverter: generated.NewGenConverter()}
			apiConversationFilters, err = attributeConverter.DBToServiceAttribute(dbConversationFilters)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(filteredUsers) > 0 {
		apiConversationFilters.Users = filteredUsers
	} else {
		templateAudience, err := qa.GetTemplateAudience(template)
		if err != nil {
			return nil, err
		}
		if templateAudience != nil {
			for _, userName := range templateAudience.Users {
				apiConversationFilters.Users = append(apiConversationFilters.Users, &userpb.User{Name: userName})
			}
			for _, groupName := range templateAudience.Groups {
				apiConversationFilters.Groups = append(apiConversationFilters.Groups, &userpb.Group{Name: groupName})
			}
		}
	}

	if len(apiConversationFilters.TimeRanges) == 0 {
		apiConversationFilters.TimeRanges = append(apiConversationFilters.TimeRanges, &analyticspb.TimeRange{
			StartTimestamp: period.ConversationEvaluationPeriod.StartTime,
			EndTimestamp:   period.ConversationEvaluationPeriod.EndTime,
		})
	}

	if len(apiConversationFilters.UsecaseNames) == 0 {
		appliedAllUsecases := false
		for _, usecaseID := range template.UsecaseIds {
			if usecaseID == consts.AllUsecase {
				appliedAllUsecases = true
				break
			}
		}
		if !appliedAllUsecases {
			for _, usecaseID := range template.UsecaseIds {
				apiConversationFilters.UsecaseNames = append(
					apiConversationFilters.UsecaseNames,
					customerpb.UsecaseName{
						CustomerID: template.Customer,
						ProfileID:  template.Profile,
						UsecaseID:  usecaseID,
					}.String())
			}
		}
	}

	apiConversationFilters.MomentGroups, err = combineTemplateMetadataCondition(
		apiConversationFilters.MomentGroups,
		template,
	)
	if err != nil {
		return nil, err
	}

	return apiConversationFilters, nil
}

// combineTemplateMetadataCondition returns moment groups requiring both the
// task's filters and the template's metadata condition. The template is
// authoritative: on any field both use, the task's group is dropped so it
// can't loosen the template's condition.
func combineTemplateMetadataCondition(taskMomentGroups []*analyticspb.MomentGroup, template *model.ScorecardTemplates) ([]*analyticspb.MomentGroup, error) {
	qaScoreConfig, err := qa.ConvertDBToServiceQAScoreConfig(template)
	if err != nil {
		return nil, fmt.Errorf("convert template qa score config: %w", err)
	}
	templateMomentGroups := qaScoreConfig.GetConversationFilters().GetMomentGroups()
	if len(templateMomentGroups) == 0 {
		return taskMomentGroups, nil
	}

	templateFields := set.New[string]()
	for _, group := range templateMomentGroups {
		fieldID, ok, err := metadataGroupFieldID(group)
		if err != nil {
			return nil, fmt.Errorf("extract template metadata field: %w", err)
		}
		if ok {
			templateFields.Add(fieldID)
		}
	}

	combined := make([]*analyticspb.MomentGroup, 0, len(taskMomentGroups)+len(templateMomentGroups))
	for _, group := range taskMomentGroups {
		fieldID, ok, err := metadataGroupFieldID(group)
		if err != nil {
			return nil, fmt.Errorf("extract task metadata field: %w", err)
		}
		if ok && templateFields.Has(fieldID) {
			continue
		}
		combined = append(combined, group)
	}
	return append(combined, templateMomentGroups...), nil
}

// metadataGroupFieldID returns the moment-template ID a group filters on: its
// included moment, or its excluded moment when it only negates. Returns false
// when the group carries no moment, or is a wildcard ("*") that targets any
// moment rather than one field, so such a group never counts as a shared field.
func metadataGroupFieldID(group *analyticspb.MomentGroup) (string, bool, error) {
	moments := group.GetMoments()
	if len(moments) == 0 {
		moments = group.GetExcludedMoments()
	}
	if len(moments) == 0 {
		return "", false, nil
	}
	momentName, err := momentpb.ParseMomentName(moments[0].GetName())
	if err != nil {
		return "", false, err
	}
	if momentName.MomentID == "*" {
		return "", false, nil
	}
	return momentName.MomentID, true, nil
}

func (a AnalyticsServiceImpl) queryScorecardsInConvos(
	ctx context.Context, scorecardTemplateName *coachingpb.ScorecardTemplateName, conversationIDs []string,
) (map[string][]*model.Scorecards, error) {
	db := a.appsDB.DB(ctx)
	conditions := condition.New(
		"customer = ? AND profile = ?",
		scorecardTemplateName.CustomerID,
		scorecardTemplateName.ProfileID,
	)
	conditions = condition.AppendSQLAndClause(conditions, "template_id = ?", scorecardTemplateName.ScorecardTemplateID)
	conditions = condition.AppendWhereIn(conditions, "conversation_id", conversationIDs)

	dbScorecards := []*model.Scorecards{}
	err := db.Model(&model.Scorecards{}).
		Where(conditions.QueryString(), conditions.QueryParams()...).
		Find(&dbScorecards).Error
	if err != nil {
		return nil, err
	}

	conversationIDToScorecardsMap := make(map[string][]*model.Scorecards)
	for _, scorecard := range dbScorecards {
		conversationID := scorecard.ConversationID
		if conversationIDToScorecardsMap[conversationID] == nil {
			conversationIDToScorecardsMap[conversationID] = make([]*model.Scorecards, 0)
		}
		conversationIDToScorecardsMap[conversationID] = append(conversationIDToScorecardsMap[conversationID], scorecard)
	}
	return conversationIDToScorecardsMap, nil
}

func (a AnalyticsServiceImpl) queryInProgressScorecardCountPerAgent(
	ctx context.Context, scorecardTemplateName *coachingpb.ScorecardTemplateName,
) (map[string]int32, error) {
	type agentDraftCount struct {
		AgentUserID string
		Count       int32
	}
	db := a.appsDB.DB(ctx)
	var results []agentDraftCount
	err := db.Model(&model.Scorecards{}).
		Select("agent_user_id, count(*) as count").
		Where(
			"customer = ? AND profile = ? AND template_id = ? AND submitted_at IS NULL AND manually_scored = true AND updated_at > ?",
			scorecardTemplateName.CustomerID,
			scorecardTemplateName.ProfileID,
			scorecardTemplateName.ScorecardTemplateID,
			time.Now().Add(-*draftLockDuration),
		).
		Group("agent_user_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	agentToDraftCount := make(map[string]int32, len(results))
	for _, r := range results {
		agentToDraftCount[r.AgentUserID] = r.Count
	}
	return agentToDraftCount, nil
}

func (a AnalyticsServiceImpl) queryMessagesInConvos(
	ctx context.Context, customerID string, profileID string, conversationIDs []string,
) (map[string][]*model.Messages, error) {
	db := a.appsDB.DB(ctx)
	conditions := condition.New(
		"customer_id = ? AND profile_id = ?",
		customerID,
		profileID,
	)
	conditions = condition.AppendWhereIn(conditions, "conversation_id", conversationIDs)

	dbMessages := []*model.Messages{}
	err := db.Model(&model.Messages{}).
		Where(conditions.QueryString(), conditions.QueryParams()...).
		Find(&dbMessages).Error
	if err != nil {
		return nil, err
	}

	conversationIDToMessagesMap := make(map[string][]*model.Messages)
	for _, message := range dbMessages {
		conversationID := message.ConversationID.String
		if conversationIDToMessagesMap[conversationID] == nil {
			conversationIDToMessagesMap[conversationID] = make([]*model.Messages, 0)
		}
		conversationIDToMessagesMap[conversationID] = append(conversationIDToMessagesMap[conversationID], message)
	}
	return conversationIDToMessagesMap, nil
}

func computeCompleteRate(stats *agentProgressAndConversations) float32 {
	if stats.AssignedScorecardCount == 0 {
		return 1.0
	}
	return safeDivideFloat(
		stats.EvaluatedScorecardCount+stats.InProgressScorecardCount+int32(len(stats.SuggestedConversationWithMessages)),
		stats.AssignedScorecardCount)
}

func suggestNextConversationWithMessage(
	agentProgressAndConversationsArray []*agentProgressAndConversations,
) *conversationWithMessage {
	// Sorts the array based on the complete rate.
	sort.Slice(agentProgressAndConversationsArray, func(i, j int) bool {
		completeRateLhs := computeCompleteRate(agentProgressAndConversationsArray[i])
		completeRateRhs := computeCompleteRate(agentProgressAndConversationsArray[j])
		if completeRateLhs != completeRateRhs {
			return completeRateLhs < completeRateRhs
		}
		return len(agentProgressAndConversationsArray[i].SuggestedConversationWithMessages) < len(agentProgressAndConversationsArray[j].SuggestedConversationWithMessages)
	})

	for _, stats := range agentProgressAndConversationsArray {
		if len(stats.AgentConversationWithMessages) > 0 && len(stats.AgentConversationWithMessages) > len(stats.SuggestedConversationWithMessages) {
			for _, convoWithMessage := range stats.AgentConversationWithMessages {
				var conversationWithMessageName string
				if convoWithMessage.Message != nil {
					conversationWithMessageName = getConversationWithMessageName(
						convoWithMessage.ConversationID, convoWithMessage.Message.ConversationMessageID.String)
				} else {
					conversationWithMessageName = getConversationWithMessageName(
						convoWithMessage.ConversationID, "")
				}
				if _, exist := stats.SuggestedConversationWithMessages[conversationWithMessageName]; !exist {
					stats.SuggestedConversationWithMessages[conversationWithMessageName] = convoWithMessage
					return convoWithMessage
				}
			}
		}
	}
	return nil
}

func getConversationWithMessageName(conversationID string, messageID string) string {
	return strings.Join([]string{conversationID, messageID}, ":")
}

func findScorecardWithMessageID(scorecards []*model.Scorecards, messageID string) *model.Scorecards {
	for _, scorecard := range scorecards {
		for _, scorecardMessageID := range scorecard.MessageIds {
			if scorecardMessageID == messageID {
				return scorecard
			}
		}
	}
	return nil
}

func findScorecardWithAgentUserID(scorecards []*model.Scorecards, agentUserID string) *model.Scorecards {
	for _, scorecard := range scorecards {
		if scorecard.AgentUserID == agentUserID {
			return scorecard
		}
	}
	return nil
}

// getTemplateType determines the template type based on the scoring subject in the template
func getTemplateType(template *model.ScorecardTemplates) templateType {
	scoreSubject, err := qa.GetScorecardTemplateScoreSubject(template)
	if err != nil {
		// Default to chat/voice template if we can't determine the scoring subject
		return templateTypeChatVoice
	}

	// As of June 2025, only email template will set the score subject.
	if scoreSubject == qapb.ScorecardScoringSubject_SCORECARD_SCORING_SUBJECT_MESSAGE {
		return templateTypePerMessageEmail
	}
	if scoreSubject == qapb.ScorecardScoringSubject_SCORECARD_SCORING_SUBJECT_CONVERSATION {
		return templateTypePerConversationEmail
	}
	return templateTypeChatVoice
}

func (a AnalyticsServiceImpl) populateConversationWithMessageCandidateForAgent(
	ctx context.Context, agentUserID string, conversationName *conversationpb.ConversationName, closedConversation *conversationpb.Conversation, message *model.Messages, agentProgressAndConversationsMap map[string]*agentProgressAndConversations, scorecardTemplateName *coachingpb.ScorecardTemplateName, authedUserID string, findScorecardFunc func() *model.Scorecards,
) {
	// Skips the agent if the agent has no progress.
	stats := agentProgressAndConversationsMap[agentUserID]
	if stats == nil {
		return
	}

	// Determine message ID for evaluation check
	messageID := ""
	if message != nil {
		messageID = message.ConversationMessageID.String
	}

	// Skips the agent if the agent has already evaluated the conversation.
	if stats.EvaluatedConversationWithMessageNames[getConversationWithMessageName(
		conversationName.ConversationID, messageID)] {
		return
	}

	reviewerUserID, exist := a.suggestedConvoCache.GetConversationSuggested(
		scorecardTemplateName.ScorecardTemplateID, conversationName.ConversationID, messageID, agentUserID)
	if exist && reviewerUserID != authedUserID {
		a.logger.Infof(ctx, "SuggestManualQAConversations the conversation %v-%v is skipped because it has been already suggested to user %v",
			conversationName.ConversationID, messageID, authedUserID)
		return
	}

	// Skips the conversations that have been already updated by another reviewer.
	scorecard := findScorecardFunc()
	if scorecard != nil {
		if scorecard.SubmittedAt.Valid {
			return
		}
		if scorecard.LastUpdaterUserID.Valid && scorecard.LastUpdaterUserID.String != authedUserID {
			a.logger.Infof(ctx, "SuggestManualQAConversations the conversation %v-%v is skipped because it has been already updated by user %v",
				conversationName.ConversationID, messageID, scorecard.LastUpdaterUserID.String)
			return
		}
	}

	// Adds the conversation to the agent's candidate list.
	stats.AgentConversationWithMessages = append(stats.AgentConversationWithMessages, &conversationWithMessage{
		Conversation:   closedConversation,
		ConversationID: conversationName.ConversationID,
		Message:        message,
		AgentUserID:    agentUserID,
	})
}
