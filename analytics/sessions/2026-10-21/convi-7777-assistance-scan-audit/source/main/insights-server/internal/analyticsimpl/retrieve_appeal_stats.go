package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"gorm.io/gorm"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/fnutils"
	"github.com/cresta/go-servers/shared/scoring"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	userUtils "github.com/cresta/go-servers/shared/user"
	userfilter "github.com/cresta/go-servers/shared/user-filter"
)

var (
	disablePerMessageAppealStats = envflag.Bool("DISABLE_PER_MESSAGE_APPEAL_STATS", false, "Disable per message appeal stats computation")
)

//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
type ScorecardWithFrequency struct {
	dbmodel.Scorecards
	Frequency sql.NullTime `gorm:"column:frequency"`
}

type CriterionConvKey struct {
	ScorecardTemplateID string
	CriterionID         string
	ConversationID      string
	ScorecardID         string // original scorecard id
	MessageID           string // empty string for non-per-message criteria
}

type AppealInfo struct {
	CriterionID       string
	ConversationID    string
	ScorecardID       string // original scorecard id
	MessageID         string // empty string for non-per-message criteria
	IsAppealed        bool
	IsAdjusted        bool
	IsResolved        bool
	AgentID           string
	QAAnalystID       string
	AppealSubmitterID string
	AppealResolverID  *string
	Frequency         sql.NullTime // start time of a time range represents (start time, start time + frequency)
	ConvCreateTime    time.Time
}

func (a AnalyticsServiceImpl) RetrieveAppealStats(
	ctx context.Context,
	req *analytics.RetrieveAppealStatsRequest,
) (*analytics.RetrieveAppealStatsResponse, error) {
	profile, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", req.Parent)
	}
	userClient, err := a.userClientRegistry.CreateClient(profile.CustomerID, profile.ProfileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user client: %v", err)
	}
	return retrieveAppealStatsInternal(
		a.logger,
		a.appsDB.DB(ctx),
		profile,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		userClient,
		&userfilter.UserFilterParserImpl{},
		RetrieveAppealStatsRequestInternal{
			Context:                    ctx,
			RetrieveAppealStatsRequest: req,
		},
	)
}

func retrieveAppealStatsInternal(
	logger log.Logger,
	db *gorm.DB,
	profile *shared.CustomerProfile,
	configClient config.Client,
	resourceACLHelper auth.ResourceACLHelper,
	userClient userpb.UserServiceClient,
	userFilterParser userfilter.UserFilterParser,
	req RetrieveAppealStatsRequestInternal,
) (*analytics.RetrieveAppealStatsResponse, error) {
	err := req.Validate()
	if err != nil {
		return nil, err
	}
	groupBy, err := req.ParseGroupByType()
	if err != nil {
		return nil, err
	}
	conditions, err := req.GetRetrieveAppealStatsConditions(
		logger, profile, configClient, resourceACLHelper,
		userClient, userFilterParser,
	)
	if err != nil {
		return nil, err
	}
	if len(conditions.ConversationIDs) == 0 && len(conditions.UserNames) == 0 {
		logger.Infof(req.Context, "There is not valid users or convos to retrieve appeal stats")
		return &analytics.RetrieveAppealStatsResponse{}, nil
	}

	appealScorecards, err := retrieveAppealScorecards(db, profile, &req.Frequency, req.GetMetadata(), conditions)
	if err != nil {
		return nil, err
	}

	appealWorkflows, err := retrieveAppealWorkflows(db, profile, appealScorecards)
	if err != nil {
		return nil, err
	}

	appealWorkflows, err = filterAppealWorkflows(profile, userClient, req, appealWorkflows)
	if err != nil {
		return nil, err
	}

	scorecardMap := map[string]*dbmodel.Scorecards{}
	for _, appeal := range appealWorkflows {
		for _, c := range appeal.ToDBScorecards() {
			scorecardMap[c.ResourceID] = c
		}
	}

	conversations, err := getConversationInfo(
		db,
		fn.Map(fn.MapValueToSlice(appealScorecards), func(c *ScorecardWithFrequency) string { return c.ConversationID }))
	if err != nil {
		return nil, err
	}

	scorecardIDs := fn.MapKeyToSlice(scorecardMap)
	scorecards := fn.MapValueToSlice(scorecardMap)
	scorecardIDToScores, err := scoring.FindExistingScores(
		db, profile.ToPBProfileName(), scorecardIDs, false, /* readFromWriteReplica */
	)
	if err != nil {
		return nil, err
	}
	userIDToUser, err := getUserDetails(req.Context, profile, scorecards, userClient)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get users: %v", err)
	}

	criteriaIDs, err := getCriteriaIDList(db, profile, fn.MapValueToSlice(appealScorecards))
	if err != nil {
		return nil, err
	}

	criterionAppealInfos := analyzeCriterionAppealInfo(
		appealWorkflows, scorecardIDToScores, appealScorecards, conversations, criteriaIDs)

	groupStats, totalStats := aggregateGroupAppealStats(criterionAppealInfos, groupBy, userIDToUser, profile)
	return &analytics.RetrieveAppealStatsResponse{
		AppealStats:      groupStats,
		TotalAppealStats: totalStats,
	}, nil
}

func filterAppealWorkflows(
	profile *shared.CustomerProfile,
	userClient userpb.UserServiceClient,
	req RetrieveAppealStatsRequestInternal,
	appealWorkflows []*scoring.AppealWorkflow,
) ([]*scoring.AppealWorkflow, error) {
	parent := &customer.CustomerOrProfileName{
		CustomerID: profile.CustomerID,
		ProfileID:  profile.ProfileID,
	}

	scorecardReviewersMap, err := getUsersFromUserAndGroupNames(
		req.Context,
		userClient,
		parent,
		req.FilterByAttribute.GetScorecardReviewerAudience().GetUsers(),
		req.FilterByAttribute.GetScorecardReviewerAudience().GetGroups(),
		nil, /* no role filter */
	)
	if err != nil {
		return nil, err
	}

	if len(scorecardReviewersMap) != 0 {
		appealWorkflows = fn.Filter(appealWorkflows, func(appeal *scoring.AppealWorkflow) bool {
			return appeal.Original.SubmitterUserID.Valid && scorecardReviewersMap[appeal.Original.SubmitterUserID.String] != nil
		})
	}

	return appealWorkflows, nil
}

type RetrieveAppealStatsGroupByType int32

const (
	RetrieveAppealStatsGroupByNone RetrieveAppealStatsGroupByType = iota
	RetrieveAppealStatsGroupByAgent
	RetrieveAppealStatsGroupByQAAnlyst
	RetrieveAppealStatsGroupByFrequency
	RetrieveAppealStatsGroupByAgentAndFrequency
	RetrieveAppealStatsGroupByQAAnlystAndFrequency
	RetrieveAppealStatsGroupByAppealSubmitterAndFrequency
	RetrieveAppealStatsGroupByAppealResolverAndFrequency
	RetrieveAppealStatsGroupByAgentAndCriterion
	RetrieveAppealStatsGroupByQAAnlystAndCriterion
	RetrieveAppealStatsGroupByAppealSubmitterAndCriterion
	RetrieveAppealStatsGroupByAppealResolverAndCriterion
)

type RetrieveAppealStatsGroupByKey struct {
	AgentID           string
	QAAnalystID       string
	AppealSubmitterID string
	AppealResolverID  *string
	Frequency         sql.NullTime
	CriterionID       string
}

func (k RetrieveAppealStatsGroupByKey) IsEmpty() bool {
	return k.AgentID == "" && k.QAAnalystID == "" && k.AppealSubmitterID == "" && k.AppealResolverID == nil && !k.Frequency.Valid && k.CriterionID == ""
}

func (k RetrieveAppealStatsGroupByKey) ToQAAttribute(userNameToUser map[string]*userpb.User) *analytics.QAAttribute {
	attribute := &analytics.QAAttribute{}
	if attribute.Users == nil {
		attribute.Users = []*userpb.User{}
	}
	if k.AgentID != "" {
		attribute.Users = append(attribute.Users, userNameToUser[k.AgentID])
	}
	if k.QAAnalystID != "" {
		attribute.Users = append(attribute.Users, userNameToUser[k.QAAnalystID])
	}
	if k.AppealSubmitterID != "" {
		attribute.Users = append(attribute.Users, userNameToUser[k.AppealSubmitterID])
	}
	if k.AppealResolverID != nil && *k.AppealResolverID != "" {
		attribute.Users = append(attribute.Users, userNameToUser[*k.AppealResolverID])
	}
	if k.Frequency.Valid {
		attribute.TimeRange = &analytics.TimeRange{
			StartTimestamp: timestamppb.New(k.Frequency.Time),
		}
	}
	if k.CriterionID != "" {
		attribute.CriterionIdentifiers = []string{k.CriterionID}
	}
	return attribute
}

type RetrieveAppealStatsGroupByKeyGetter func(*AppealInfo) RetrieveAppealStatsGroupByKey

func RetrieveAppealStatsGroupByToGroupKeyFunc(groupBy RetrieveAppealStatsGroupByType) RetrieveAppealStatsGroupByKeyGetter {
	switch groupBy {
	case RetrieveAppealStatsGroupByAgent:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				AgentID: info.AgentID,
			}
		}
	case RetrieveAppealStatsGroupByQAAnlyst:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				QAAnalystID: info.QAAnalystID,
			}
		}
	case RetrieveAppealStatsGroupByAgentAndFrequency:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				AgentID:   info.AgentID,
				Frequency: info.Frequency,
			}
		}
	case RetrieveAppealStatsGroupByQAAnlystAndFrequency:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				QAAnalystID: info.QAAnalystID,
				Frequency:   info.Frequency,
			}
		}
	case RetrieveAppealStatsGroupByFrequency:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				Frequency: info.Frequency,
			}
		}
	case RetrieveAppealStatsGroupByAgentAndCriterion:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				AgentID:     info.AgentID,
				CriterionID: info.CriterionID,
			}
		}
	case RetrieveAppealStatsGroupByQAAnlystAndCriterion:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{
				QAAnalystID: info.QAAnalystID,
				CriterionID: info.CriterionID,
			}
		}
	case RetrieveAppealStatsGroupByNone:
		fallthrough
	default:
		return func(info *AppealInfo) RetrieveAppealStatsGroupByKey {
			return RetrieveAppealStatsGroupByKey{}
		}
	}
}

func getUserDetails(
	ctx context.Context,
	profile *shared.CustomerProfile,
	scorecards []*dbmodel.Scorecards,
	userServiceClient userpb.UserServiceClient,
) (map[string]*userpb.User, error) {
	// Get the agent and qa analyst (submitter)
	userIDs := set.New[string]()
	for _, scorecard := range scorecards {
		userIDs.Add(scorecard.AgentUserID)
		if scorecard.SubmitterUserID.Valid {
			userIDs.Add(scorecard.SubmitterUserID.String)
		}
	}
	includeGroupMembership := true
	users := []*userpb.User{}
	err := userUtils.CollectUsersPaginated(
		ctx, userServiceClient,
		profile.CustomerID, profile.ProfileID,
		&userpb.ListUsersRequest_ListUsersFilter{
			UserIds: userIDs.Slice(),
		},
		1000,
		"user_id",
		userpb.ListUsersRequest_ASC,
		includeGroupMembership,
		func(u []*userpb.User) error {
			users = append(users, u...)
			return nil
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch agents and qa analysts: %v", err)
	}

	userIDToUser := map[string]*userpb.User{}
	for _, u := range users {
		userName, err := userpb.ParseUserName(u.Name)
		if err != nil {
			return nil, err
		}
		userIDToUser[userName.GetUserID()] = u
	}
	return userIDToUser, nil
}

func retrieveAppealScorecards(
	db *gorm.DB,
	profile *shared.CustomerProfile,
	frequency *analytics.Frequency,
	metadata *analytics.Metadata,
	conditions *RetrieveAppealStatsConditions,
) (map[string]*ScorecardWithFrequency, error) {
	frequencyArgs, err := postgres.ParseArgs(frequency, metadata)
	if err != nil {
		return nil, err
	}

	userIds, err := fn.MapE(conditions.UserNames, func(n string) (string, error) {
		name, err := userpb.ParseUserName(n)
		if err != nil {
			return "", err
		}
		return name.GetUserID(), nil
	})
	if err != nil {
		return nil, err
	}

	// Queries all the submitted appeal scorecards.
	var appealScorecards []*ScorecardWithFrequency
	query := db.Model(&dbmodel.Scorecards{}).
		Where("customer = ? AND profile = ?", profile.CustomerID, profile.ProfileID).
		Where("scorecard_type = ?", coaching.ScorecardType_SCORECARD_TYPE_APPEAL_REQUEST).
		Where("submitted_at >= ? AND submitted_at <= ?",
			conditions.AppealRequestSubmitAtTimeRange.Start,
			conditions.AppealRequestSubmitAtTimeRange.End).
		Select(`
            *,
            DATE_TRUNC(
                @frequency,
                submitted_at AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias
            ) AT TIME ZONE @time_zone - INTERVAL '1 day' * @first_day_of_week_bias AS frequency
        `, frequencyArgs...)

	// Only filter by template ID if it's provided
	if conditions.ScorecardTemplateID != "" {
		query = query.Where("template_id = ?", conditions.ScorecardTemplateID)
	}

	if len(userIds) > 0 {
		query = query.Where("agent_user_id IN (?)", userIds)
	}
	if len(conditions.ConversationIDs) > 0 {
		query = query.Where("conversation_id IN (?)", conditions.ConversationIDs)
	}

	err = query.Find(&appealScorecards).Error
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query appeal scorecards due to %v", err)
	}

	appealScorecardMap := make(map[string]*ScorecardWithFrequency)
	for _, appealScorecard := range appealScorecards {
		appealScorecardMap[appealScorecard.ResourceID] = appealScorecard
	}

	return appealScorecardMap, nil
}

func getCriteriaIDList(
	db *gorm.DB,
	profile *shared.CustomerProfile,
	scorecards []*ScorecardWithFrequency,
) (map[string]bool, error) {
	if len(scorecards) == 0 {
		return map[string]bool{}, nil
	}

	// Get the (template and revision) tuple list of all scorecards.
	templateRevisionIDs := [][]string{}
	for _, scorecard := range scorecards {
		templateRevisionIDs = append(templateRevisionIDs, []string{scorecard.TemplateID, scorecard.TemplateRevision})
	}

	// Query the templates from db.
	var templates []*dbmodel.ScorecardTemplates
	err := db.Model(&dbmodel.ScorecardTemplates{}).
		Where("customer = ? AND profile = ?", profile.CustomerID, profile.ProfileID).
		Where("(resource_id, revision) IN (?)", templateRevisionIDs).
		Find(&templates).Error
	if err != nil {
		return nil, err
	}

	// Parses the criterion list from the templates.
	criteriaIDs := make(map[string]bool)
	for _, template := range templates {
		templateStructure, err := scoring.ParseScorecardTemplateStructure(template.Template)
		if err != nil {
			return nil, err
		}
		for _, criterion := range templateStructure.GetCriteriaSlice(false /*rootOnly*/) {
			criteriaIDs[criterion.GetIdentifier()] = true
		}
	}
	return criteriaIDs, nil
}

func retrieveAppealWorkflows(
	db *gorm.DB,
	profile *shared.CustomerProfile,
	appealScorecards map[string]*ScorecardWithFrequency,
) ([]*scoring.AppealWorkflow, error) {
	// Queries the appeal workflows for all the appeals.
	groupedAppeals, err := scoring.NewGroupedAppealWorkflowsFromScorecardIDs(
		db, &customer.ProfileName{CustomerID: profile.CustomerID, ProfileID: profile.ProfileID}, fn.MapKeyToSlice(appealScorecards))
	if err != nil {
		return nil, fmt.Errorf("failed to get appeal workflows due to %v", err)
	}

	// Makes sure the appeal request itself is submitted. The original and its replica may legitimately
	// be unsubmitted: since CONVI-7485 an appeal can be filed against an unsubmitted auto-scored
	// scorecard, and the replica snapshots the original as it was at appeal time.
	allAppeals := groupedAppeals.FlattenedAppealWorkflows()
	appealWorkflows := make([]*scoring.AppealWorkflow, 0, len(allAppeals))
	for _, appeal := range allAppeals {
		if appeal.Original != nil && appeal.OriginalReplica != nil && appeal.AppealRequest != nil {
			if !appeal.AppealRequest.SubmittedAt.Valid {
				continue
			}
			if appeal.AppealResolve != nil && !appeal.AppealResolve.SubmittedAt.Valid {
				appeal.AppealResolve = nil
			}
			appealWorkflows = append(appealWorkflows, appeal)
		}
	}

	return appealWorkflows, nil
}

func collectAppealStats(
	criterionAppealInfos []*AppealInfo,
	profileName *shared.CustomerProfile,
) *analytics.AppealStats {
	type AppealCount struct {
		AppealedCount int32
		ResolvedCount int32
		AdjustedCount int32
	}

	totalCriteriaAppealCount := &AppealCount{}
	appealInfoPerScorecard := make(map[string]*AppealInfo)
	criteriaAppealCountPerScorecard := make(map[string]*AppealCount)
	for _, info := range criterionAppealInfos {
		if _, exist := appealInfoPerScorecard[info.ScorecardID]; !exist {
			appealInfoPerScorecard[info.ScorecardID] = &AppealInfo{
				ConversationID: info.ConversationID,
				ConvCreateTime: info.ConvCreateTime,
				ScorecardID:    info.ScorecardID,
			}
			criteriaAppealCountPerScorecard[info.ScorecardID] = &AppealCount{}
		}
		if info.IsAppealed {
			totalCriteriaAppealCount.AppealedCount += 1
			appealInfoPerScorecard[info.ScorecardID].IsAppealed = true
			criteriaAppealCountPerScorecard[info.ScorecardID].AppealedCount += 1
		}
		if info.IsResolved {
			totalCriteriaAppealCount.ResolvedCount += 1
			appealInfoPerScorecard[info.ScorecardID].IsResolved = true
			criteriaAppealCountPerScorecard[info.ScorecardID].ResolvedCount += 1
		}
		if info.IsAdjusted {
			totalCriteriaAppealCount.AdjustedCount += 1
			appealInfoPerScorecard[info.ScorecardID].IsAdjusted = true
			criteriaAppealCountPerScorecard[info.ScorecardID].AdjustedCount += 1
		}
	}

	convs := []*analytics.AppealedConversation{}
	totalConvoAppealCount := &AppealCount{}
	for scorecardID, convoAppealInfo := range appealInfoPerScorecard {
		convName := conversation.ConversationName{
			CustomerID:     profileName.CustomerID,
			ProfileID:      profileName.ProfileID,
			ConversationID: convoAppealInfo.ConversationID,
		}.String()
		conv := &analytics.AppealedConversation{
			ConversationName:       convName,
			ConversationCreateTime: timestamppb.New(convoAppealInfo.ConvCreateTime),
			OriginalScorecardName: coaching.ScorecardName{
				CustomerID:  profileName.CustomerID,
				ProfileID:   profileName.ProfileID,
				ScorecardID: scorecardID,
			}.String(),
		}
		if convoAppealInfo.IsAppealed {
			totalConvoAppealCount.AppealedCount += 1
			conv.AppealStatus = analytics.AppealStatus_APPEAL_STATUS_APPEALED
		}
		if convoAppealInfo.IsResolved {
			totalConvoAppealCount.ResolvedCount += 1
			conv.AppealStatus = analytics.AppealStatus_APPEAL_STATUS_RESOLVED
		}
		if convoAppealInfo.IsAdjusted {
			totalConvoAppealCount.AdjustedCount += 1
			conv.AdjustedStatus = analytics.AdjustedStatus_ADJUSTED_STATUS_ADJUSTED
		} else {
			conv.AdjustedStatus = analytics.AdjustedStatus_ADJUSTED_STATUS_NOT_ADJUSTED
		}
		if conv.AppealStatus != analytics.AppealStatus_APPEAL_STATUS_UNSPECIFIED {
			conv.AppealedCriteriaCount = criteriaAppealCountPerScorecard[scorecardID].AppealedCount
			conv.ResolvedCriteriaCount = criteriaAppealCountPerScorecard[scorecardID].ResolvedCount
			conv.AdjustedCriteriaCount = criteriaAppealCountPerScorecard[scorecardID].AdjustedCount
			convs = append(convs, conv)
		}
	}

	return &analytics.AppealStats{
		AppealedCriteriaCount:     totalCriteriaAppealCount.AppealedCount,
		ResolvedCriteriaCount:     totalCriteriaAppealCount.ResolvedCount,
		AdjustedCriteriaCount:     totalCriteriaAppealCount.AdjustedCount,
		AppealedConversationCount: totalConvoAppealCount.AppealedCount,
		ResolvedConversationCount: totalConvoAppealCount.ResolvedCount,
		AdjustedConversationCount: totalConvoAppealCount.AdjustedCount,
		AppealedConversations:     convs,
	}
}

func aggregateGroupAppealStats(
	criterionAppealInfos map[CriterionConvKey]*AppealInfo,
	groupBy RetrieveAppealStatsGroupByType,
	userIDToUser map[string]*userpb.User,
	profileName *shared.CustomerProfile,
) ([]*analytics.AppealStats, *analytics.AppealStats) {
	groupedAppealInfos := map[RetrieveAppealStatsGroupByKey][]*AppealInfo{}
	groupKeyGetter := RetrieveAppealStatsGroupByToGroupKeyFunc(groupBy)
	for _, criterionAppealInfo := range criterionAppealInfos {
		groupKey := groupKeyGetter(criterionAppealInfo)
		if _, exist := groupedAppealInfos[groupKey]; !exist {
			groupedAppealInfos[groupKey] = []*AppealInfo{}
		}
		groupedAppealInfos[groupKey] = append(groupedAppealInfos[groupKey], criterionAppealInfo)
	}

	groups := []*analytics.AppealStats{}
	for groupKey, appealInfos := range groupedAppealInfos {
		if groupKey.IsEmpty() {
			continue
		}
		stats := collectAppealStats(appealInfos, profileName)
		stats.Attribute = groupKey.ToQAAttribute(userIDToUser)
		groups = append(groups, stats)
	}
	total := collectAppealStats(fn.MapValueToSlice(criterionAppealInfos), profileName)
	return groups, total
}

// The result is conversation ID to conversation DB model.
func getConversationInfo(db *gorm.DB, conversationIDs []string) (map[string]*dbmodel.Chats, error) {
	var conversations []*dbmodel.Chats
	// The only conv data we need are conv ID and create time.
	err := db.Model(&dbmodel.Chats{}).
		Where("conversation_id IN (?)", conversationIDs).
		Select("conversation_id, created_at").
		Find(&conversations).Error
	if err != nil {
		return nil, err
	}
	res := map[string]*dbmodel.Chats{}
	for _, c := range conversations {
		res[c.ConversationID.String] = c
	}
	return res, err
}

func analyzeCriterionAppealInfo(
	appealWorkflows []*scoring.AppealWorkflow,
	scorecardIDToScores map[string][]*dbmodel.Scores,
	appealScorecards map[string]*ScorecardWithFrequency,
	conversations map[string]*dbmodel.Chats,
	criteriaIDs map[string]bool,
) map[CriterionConvKey]*AppealInfo {
	buildScoresMap := func(scorecard *dbmodel.Scorecards) map[string][]*dbmodel.Scores {
		scores := map[string][]*dbmodel.Scores{}
		if scorecard == nil {
			return scores
		}
		for _, score := range scorecardIDToScores[scorecard.ResourceID] {
			scores[score.CriterionIdentifier] = append(scores[score.CriterionIdentifier], score)
		}
		return scores
	}

	infos := map[CriterionConvKey]*AppealInfo{}
	for _, appealWorkflow := range appealWorkflows {
		appealRequestScores := buildScoresMap(appealWorkflow.AppealRequest)
		originalScoresMap := buildScoresMap(appealWorkflow.OriginalReplica)
		resolveScoresMap := buildScoresMap(appealWorkflow.AppealResolve)

		for criterionID, appealScores := range appealRequestScores {
			if !criteriaIDs[criterionID] {
				// Skips the chapter criteria.
				continue
			}

			// When feature flag is enabled, use old behavior without per-message grouping
			if *disablePerMessageAppealStats {
				var isAppealed, isResolved, isAdjusted bool
				originalScores := originalScoresMap[criterionID]
				if len(originalScores) == 0 || len(appealScores) != len(originalScores) {
					isAppealed = true
				} else {
					isAppealed = hasScoresChanged(originalScores, appealScores)
				}
				if isAppealed {
					resolveScores, resolveScoreExist := resolveScoresMap[criterionID]
					if resolveScoreExist && len(resolveScores) > 0 {
						isResolved = true
						if len(originalScores) > 0 && len(resolveScores) == len(originalScores) {
							isAdjusted = hasScoresChanged(originalScores, resolveScores)
						} else {
							isAdjusted = true
						}
					} else {
						isResolved = false
						isAdjusted = false
					}
				}

				info := &AppealInfo{
					IsAppealed:        isAppealed,
					IsResolved:        isResolved,
					IsAdjusted:        isAdjusted,
					CriterionID:       criterionID,
					ConversationID:    appealWorkflow.Original.ConversationID,
					ScorecardID:       appealWorkflow.Original.ResourceID,
					AgentID:           appealWorkflow.Original.AgentUserID,
					QAAnalystID:       appealWorkflow.Original.SubmitterUserID.String,
					AppealSubmitterID: appealWorkflow.AppealRequest.SubmitterUserID.String,
					Frequency:         appealScorecards[appealWorkflow.AppealRequest.ResourceID].Frequency,
					ConvCreateTime:    conversations[appealWorkflow.AppealRequest.ConversationID].CreatedAt,
				}
				if appealWorkflow.AppealResolve != nil {
					info.AppealResolverID = &appealWorkflow.AppealResolve.SubmitterUserID.String
				}
				infos[CriterionConvKey{
					ScorecardTemplateID: appealWorkflow.AppealRequest.TemplateID,
					CriterionID:         criterionID,
					ConversationID:      appealWorkflow.AppealRequest.ConversationID,
					ScorecardID:         appealWorkflow.Original.ResourceID,
				}] = info
				continue
			}

			// Group scores by message ID to handle per-message criteria
			originalByMsg := fnutils.GroupBy(originalScoresMap[criterionID], getMessageIDFromScore)
			appealByMsg := fnutils.GroupBy(appealScores, getMessageIDFromScore)
			resolveByMsg := fnutils.GroupBy(resolveScoresMap[criterionID], getMessageIDFromScore)

			// Get all unique message IDs (empty string "" for non-per-message criteria)
			allMsgIDs := getAllUniqueMessageIDs(originalByMsg, appealByMsg, resolveByMsg)

			for _, msgID := range allMsgIDs {
				originalScores := originalByMsg[msgID]
				appealMsgScores := appealByMsg[msgID]
				resolveMsgScores := resolveByMsg[msgID]

				isAppealed, isResolved, isAdjusted := computeAppealStatus(originalScores, appealMsgScores, resolveMsgScores)

				info := &AppealInfo{
					IsAppealed:        isAppealed,
					IsResolved:        isResolved,
					IsAdjusted:        isAdjusted,
					CriterionID:       criterionID,
					ConversationID:    appealWorkflow.Original.ConversationID,
					ScorecardID:       appealWorkflow.Original.ResourceID,
					MessageID:         msgID,
					AgentID:           appealWorkflow.Original.AgentUserID,
					QAAnalystID:       appealWorkflow.Original.SubmitterUserID.String,
					AppealSubmitterID: appealWorkflow.AppealRequest.SubmitterUserID.String,
					Frequency:         appealScorecards[appealWorkflow.AppealRequest.ResourceID].Frequency,
					ConvCreateTime:    conversations[appealWorkflow.AppealRequest.ConversationID].CreatedAt,
				}
				if appealWorkflow.AppealResolve != nil {
					info.AppealResolverID = &appealWorkflow.AppealResolve.SubmitterUserID.String
				}
				infos[CriterionConvKey{
					ScorecardTemplateID: appealWorkflow.AppealRequest.TemplateID,
					CriterionID:         criterionID,
					ConversationID:      appealWorkflow.AppealRequest.ConversationID,
					ScorecardID:         appealWorkflow.Original.ResourceID,
					MessageID:           msgID,
				}] = info
			}
		}
	}
	return infos
}

func getMessageIDFromScore(score *dbmodel.Scores) string {
	if score.MessageID.Valid {
		return score.MessageID.String
	}
	return ""
}

func getAllUniqueMessageIDs(maps ...map[string][]*dbmodel.Scores) []string {
	seen := set.New[string]()
	for _, m := range maps {
		for messageID := range m {
			seen.Add(messageID)
		}
	}
	return seen.Slice()
}

func isScoreValueEqual(scoreA, scoreB *dbmodel.Scores) bool {
	if (scoreA == nil && scoreB != nil) || (scoreA != nil && scoreB == nil) {
		return false
	}
	if scoreA == nil && scoreB == nil {
		return true
	}
	equalTextValue := cmp.Equal(scoreA.TextValue, scoreB.TextValue)
	fraction := 0.01 // see (https://pkg.go.dev/github.com/google/go-cmp@v0.6.0/cmp/cmpopts#EquateApprox
	margin := 0.01
	equalNumericValue := cmp.Equal(scoreA.NumericValue, scoreB.NumericValue, cmpopts.EquateNaNs(), cmpopts.EquateApprox(fraction, margin))
	equalNAValue := cmp.Equal(scoreA.NotApplicable, scoreB.NotApplicable)
	return equalTextValue && equalNumericValue && equalNAValue
}

func hasScoresChanged(originalScores, appealScores []*dbmodel.Scores) bool {
	if len(originalScores) == 0 {
		return true
	}
	if len(originalScores) != len(appealScores) {
		return true
	}
	hasChanged := false
	for _, originalScore := range originalScores {
		originalScoreFound := slices.ContainsFunc(appealScores, func(score *dbmodel.Scores) bool {
			return isScoreValueEqual(originalScore, score)
		})
		if !originalScoreFound {
			hasChanged = true
			break
		}
	}
	return hasChanged
}

func computeAppealStatus(originalScores, appealScores, resolveScores []*dbmodel.Scores) (isAppealed, isResolved, isAdjusted bool) {
	isAppealed = hasScoresChanged(originalScores, appealScores)
	isResolved = isAppealed && len(resolveScores) > 0
	isAdjusted = isResolved && hasScoresChanged(originalScores, resolveScores)

	return isAppealed, isResolved, isAdjusted
}

type RetrieveAppealStatsRequestInternal struct {
	context.Context
	*analytics.RetrieveAppealStatsRequest
}

func (r RetrieveAppealStatsRequestInternal) Validate() error {
	timeRangeFilter := r.GetFilterByTimeRange()
	if timeRangeFilter == nil || timeRangeFilter.StartTimestamp == nil || timeRangeFilter.EndTimestamp == nil {
		return status.Errorf(codes.InvalidArgument, "a time range with start and end time is required")
	}
	if len(r.GetFilterByAttribute().GetUsecaseNames()) != 1 {
		return status.Errorf(codes.InvalidArgument, "there should be one usecase name")
	}
	if len(r.GetFilterByAttribute().GetConversationNames()) > 0 {
		if len(r.GetFilterByAttribute().GetUsers()) > 0 ||
			len(r.GetFilterByAttribute().GetGroups()) > 0 {
			return status.Errorf(codes.InvalidArgument,
				"don't support filter by the conversations and filter by users together")
		}
	}
	if len(r.GetFilterByAttribute().GetScorecardTemplates()) > 1 {
		return status.Errorf(codes.InvalidArgument, "there should be one or none scorecard template name")
	}
	return nil
}

func (r RetrieveAppealStatsRequestInternal) GetRetrieveAppealStatsConditions(
	logger log.Logger,
	profile *shared.CustomerProfile,
	configClient config.Client,
	resourceACLHelper auth.ResourceACLHelper,
	userClient userpb.UserServiceClient,
	userFilterParser userfilter.UserFilterParser,
) (*RetrieveAppealStatsConditions, error) {
	attribute := r.GetFilterByAttribute()

	// Handle template IDs - allow empty to return all templates
	var templateID string
	if len(attribute.ScorecardTemplates) > 0 {
		templateIDs, err := fn.MapE(
			attribute.ScorecardTemplates,
			func(n string) (string, error) {
				templateName, err := coaching.ParseScorecardTemplateName(n)
				if err != nil {
					return "", err
				}
				if templateName.CustomerID != profile.CustomerID || templateName.ProfileID != profile.ProfileID {
					return "", status.Error(codes.InvalidArgument, "the given scorecard template doesn't belong to the current customer")
				}
				return templateName.ScorecardTemplateID, nil
			},
		)
		if err != nil {
			return nil, err
		}
		templateID = templateIDs[0]
	}

	conversationIDs, err := fn.MapE(
		attribute.GetConversationNames(),
		func(n string) (string, error) {
			conversationName, err := conversation.ParseConversationName(n)
			if err != nil {
				return "", err
			}
			if conversationName.CustomerID != profile.CustomerID || conversationName.ProfileID != profile.ProfileID {
				return "", status.Error(codes.InvalidArgument, "the given convos don't belong to the current customer")
			}
			return conversationName.ConversationID, nil
		},
	)
	if err != nil {
		return nil, err
	}

	filtered := &userfilter.FilteredUsersAndGroups{}
	if len(conversationIDs) == 0 {
		includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(attribute.GetGroupMembershipFilter())
		filtered, err = r.ParseAgentAndGroupFilter(
			r.Context, logger, profile, configClient, resourceACLHelper,
			userClient, attribute,
			userFilterParser, includeDirectGroupMembershipsOnly,
		)
		if err != nil {
			return nil, err
		}
	}

	timeRange := r.GetFilterByTimeRange()
	return &RetrieveAppealStatsConditions{
		ScorecardTemplateID: templateID,
		AppealRequestSubmitAtTimeRange: TimeRange{
			Start: timeRange.GetStartTimestamp().AsTime(),
			End:   timeRange.GetEndTimestamp().AsTime(),
		},
		ConversationIDs:        conversationIDs,
		FilteredUsersAndGroups: filtered,
	}, nil
}

func (r RetrieveAppealStatsRequestInternal) ParseAgentAndGroupFilter(
	ctx context.Context,
	logger log.Logger,
	profile *shared.CustomerProfile,
	configClient config.Client,
	resourceACLHelper auth.ResourceACLHelper,
	userServiceClient userpb.UserServiceClient,
	attribute *analytics.QAAttribute,
	userFilterParser userfilter.UserFilterParser,
	includeDirectGroupMembershipsOnly bool,
) (*userfilter.FilteredUsersAndGroups, error) {
	var err error
	groupNames := fn.Map(attribute.Groups, func(g *userpb.Group) string {
		return g.Name
	})
	groupsByGroupType := map[userpb.Group_Type][]string{}
	if len(groupNames) > 0 {
		groupsByGroupType, err = userfilter.GroupsByGroupType(
			ctx, profile.CustomerID, profile.ProfileID, groupNames, userServiceClient,
		)
	}
	if err != nil {
		return nil, err
	}
	userNames := fn.Map(attribute.Users, func(u *userpb.User) string {
		return u.Name
	})
	userFilterConditions := &userfilter.UserFilterConditions{
		SelectedUserNames:         userNames,
		SelectedVirtualGroupNames: groupsByGroupType[userpb.Group_DYNAMIC_GROUP],
		SelectedTeamGroupNames:    groupsByGroupType[userpb.Group_TEAM],
		UserTypes: []enums.UserType{
			enums.UserType_PROD_USER,
			enums.UserType_GUEST_USER,
		},
		Roles: []authpb.AuthProto_Role{
			authpb.AuthProto_AGENT,
		},
		DirectTeamOnly: includeDirectGroupMembershipsOnly,
	}
	if attribute.ExcludeDeactivatedUsers {
		userFilterConditions.State = userpb.User_ACTIVE
	}
	filteredUsersAndGroups, err := userFilterParser.Parse(
		ctx, profile.CustomerID, userFilterConditions,
		userServiceClient, configClient, resourceACLHelper, logger,
	)
	if err != nil {
		return nil, err
	}
	return filteredUsersAndGroups, nil
}

func (r RetrieveAppealStatsRequestInternal) ParseGroupByType() (RetrieveAppealStatsGroupByType, error) {
	reqGroupByAttributes := r.GetGroupByAttributes()
	switch {
	case len(reqGroupByAttributes) == 0:
		return RetrieveAppealStatsGroupByNone, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
	):
		return RetrieveAppealStatsGroupByAgent, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_QA_ANALYST,
	):
		return RetrieveAppealStatsGroupByQAAnlyst, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return RetrieveAppealStatsGroupByFrequency, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return RetrieveAppealStatsGroupByAgentAndFrequency, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_QA_ANALYST,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return RetrieveAppealStatsGroupByQAAnlystAndFrequency, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_APPEAL_SUBMITTER,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return RetrieveAppealStatsGroupByAppealSubmitterAndFrequency, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_APPEAL_RESOLVER,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
	):
		return RetrieveAppealStatsGroupByAppealResolverAndFrequency, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
	):
		return RetrieveAppealStatsGroupByAgentAndCriterion, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_QA_ANALYST,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
	):
		return RetrieveAppealStatsGroupByQAAnlystAndCriterion, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_APPEAL_SUBMITTER,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
	):
		return RetrieveAppealStatsGroupByAppealSubmitterAndCriterion, nil
	case shared.OnlyContainsQAAttributeTypes(
		reqGroupByAttributes,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_APPEAL_RESOLVER,
		analytics.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
	):
		return RetrieveAppealStatsGroupByAppealResolverAndCriterion, nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "the given group-by is not supported")
	}
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type RetrieveAppealStatsConditions struct {
	ScorecardTemplateID            string
	AppealRequestSubmitAtTimeRange TimeRange
	ConversationIDs                []string
	*userfilter.FilteredUsersAndGroups
}
