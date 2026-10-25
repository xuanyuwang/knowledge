package analyticsimpl

import (
	"fmt"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	enumpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	gwpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/guidedworkflow"
	kbpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/knowledgebase"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	tagpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/tag"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

var (
	enableDeprecatePostgresUserIDQuery = envflag.Bool("ENABLE_DEPRECATE_POSTGRES_USER_ID_QUERY", false, "enable deprecating postgres user id query")
)

func parseMessageFilterParams(filterMessageByAttribute *analyticspb.Attribute,
	timeRange *analyticspb.TimeRange,
	timeZone string,
	userIDs []string,
	includeConversationFieldsInMsg bool,
	targetFieldForTimeRangeFilter analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
) (*elasticsearch.MessageFilterParams, error) {
	msgFilter := elasticsearch.MessageFilterParams{}

	// These filters are independent of the filterMessageByAttribute.
	var conversationSources = elasticsearch.GetDefaultConversationSources()

	if len(conversationSources) > 0 {
		msgFilter.ConversationSources = conversationSources
	}

	if timeRange != nil {
		msgFilter.TimeRange = parseTimeRange(timeRange, timeZone)
	}

	if includeConversationFieldsInMsg {
		msgFilter.TargetFieldForTimeRangeFilter = targetFieldForTimeRangeFilter
	}

	if len(userIDs) > 0 {
		msgFilter.AgentUserIDs = append(msgFilter.AgentUserIDs, userIDs...)
	}

	if filterMessageByAttribute == nil {
		return &msgFilter, nil
	}

	// These filters are added only when filterMessageByAttribute is not nil.
	esSpeakerRoles, err := shared.ConvertAPIToESSpeakerRoles(filterMessageByAttribute.SpeakerRoles)
	if err != nil {
		return nil, err
	}

	usecaseIDs, err := parseUsercaseNames(filterMessageByAttribute.UsecaseNames)
	if err != nil {
		return nil, err
	}

	if err := validateMomentGroups(filterMessageByAttribute.MomentGroups); err != nil {
		return nil, err
	}
	msgFilter.MomentGroups = filterMessageByAttribute.MomentGroups
	msgFilter.ActionGroups = filterMessageByAttribute.ActionGroups
	msgFilter.SpeakerRoles = esSpeakerRoles
	msgFilter.UsecaseIDs = usecaseIDs
	msgFilter.KeywordsExpressions = filterMessageByAttribute.KeywordsExpressions
	msgFilter.LanguageCodes = filterMessageByAttribute.LanguageCodes
	msgFilter.KnowledgeAssist = filterMessageByAttribute.KnowledgeMomentFilter

	if messageCountFilter := filterMessageByAttribute.MessageCountRange; messageCountFilter != nil {
		if messageCountFilter.LowerBound == nil && messageCountFilter.UpperBound == nil {
			return nil, status.Error(codes.InvalidArgument, "at least one of lower_bound or upper_bound of message_count_range should be set")
		}
		msgFilter.MessageCountRange, err = parseMessageCountRangeToFloatRange(messageCountFilter)
		if err != nil {
			return nil, err
		}
	}

	for _, convo := range filterMessageByAttribute.Conversations {
		if convo.Name != "" {
			convoName, err := convo.ParseResourceName()
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			msgFilter.ConversationIDs = append(msgFilter.ConversationIDs, convoName.GetConversationID())
		} else if platformInfo := convo.PlatformInfo; platformInfo != nil && platformInfo.PlatformConversationId != "" {
			msgFilter.PlatformConversationIDs = append(msgFilter.PlatformConversationIDs, platformInfo.PlatformConversationId)
		}
	}
	for _, convo := range filterMessageByAttribute.ConversationIdentifiers {
		if n := convo.Name; n != "" {
			name, err := convopb.ParseConversationName(n)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			msgFilter.ConversationIDs = append(msgFilter.ConversationIDs, name.ConversationID)
		}
		if id := convo.PlatformConversationId; id != "" {
			msgFilter.PlatformConversationIDs = append(msgFilter.PlatformConversationIDs, id)
		}
	}

	if filterMessageByAttribute.IncludeCrestaBot {
		msgFilter.ConversationSources = append(msgFilter.ConversationSources, int32(convopb.Conversation_BOT))
	}

	overrideConversationSources := []int32{}
	for _, conversationSource := range filterMessageByAttribute.ConversationSources {
		overrideConversationSources = append(overrideConversationSources, int32(conversationSource))
	}
	if len(overrideConversationSources) > 0 {
		msgFilter.ConversationSources = overrideConversationSources
	}

	return &msgFilter, nil
}

func parseConversationFilterParams(filterConversationByAttribute, filterConversationDocumentByConversationByAttribute *analyticspb.Attribute, timeRange *analyticspb.TimeRange, timeZone string, referenceBuckets []*analyticspb.ConversationDurationBucket, userIDs []string) (*elasticsearch.ConversationFilterParams, error) {
	convoFilter := elasticsearch.ConversationFilterParams{}
	if len(userIDs) > 0 {
		convoFilter.AgentUserIDs = userIDs
	}
	convoFilter.TimeRange = parseTimeRange(timeRange, timeZone)
	if filterConversationDocumentByConversationByAttribute != nil && len(filterConversationDocumentByConversationByAttribute.MomentGroups) > 0 {
		if err := validateMomentGroups(filterConversationDocumentByConversationByAttribute.MomentGroups); err != nil {
			return nil, err
		}
		convoFilter.MomentGroups = append(convoFilter.MomentGroups, filterConversationDocumentByConversationByAttribute.MomentGroups...)
	}
	if filterConversationByAttribute == nil {
		return &convoFilter, nil
	}
	if err := validateMomentGroups(filterConversationByAttribute.MomentGroups); err != nil {
		return nil, err
	}

	convoFilter.TruncatedTimeRanges = filterConversationByAttribute.TruncatedTimeRanges

	if len(filterConversationByAttribute.ScorecardFilters) > 0 {
		for _, scorecardFilter := range filterConversationByAttribute.ScorecardFilters {
			criteria := []*analyticspb.ScorecardCriterionFilter{}
			for _, criterion := range scorecardFilter.CriterionFilters {
				scores := []*analyticspb.CriterionScore{}
				for _, score := range criterion.Scores {
					scores = append(scores, &analyticspb.CriterionScore{
						NumericValue:  score.NumericValue,
						NotApplicable: score.NotApplicable,
					})
				}
				criteria = append(criteria, &analyticspb.ScorecardCriterionFilter{
					CriterionId: criterion.CriterionId,
					Scores:      scores,
				})
			}
			scorecardTemplate, err := coachingpb.ParseScorecardTemplateName(scorecardFilter.ScorecardTemplateName)
			if err != nil {
				return nil, err
			}
			convoFilter.ScorecardFilters = append(convoFilter.ScorecardFilters, &analyticspb.ScorecardFilter{
				ScorecardTemplateName: scorecardTemplate.GetScorecardTemplateID(),
				CriterionFilters:      criteria,
			})
		}
	}

	if filterConversationByAttribute.IncludeCrestaBot {
		convoFilter.ConversationSources = []int32{int32(convopb.Conversation_BOT)}
	}

	durationRange := filterConversationByAttribute.ConversationDurationRange
	if len(filterConversationByAttribute.ConversationDurationBuckets) > 0 && (durationRange != nil && (durationRange.MinDuration != nil || durationRange.MaxDuration != nil)) {
		return nil, status.Errorf(codes.InvalidArgument, "cannot set both `conversation_duration_buckets` and `conversation_duration_range`.")
	}
	if len(filterConversationByAttribute.ConversationDurationBuckets) > 0 {
		convoFilter.ConversationDurationSecsRanges = parseDurationBuckets(
			filterConversationByAttribute.ConversationDurationBuckets,
			referenceBuckets,
		)
	}

	// TODO(zhucaiyi): do not hard-code
	if convoHandleTimeFilter := filterConversationByAttribute.ConversationHandleTimeInSecs; convoHandleTimeFilter != nil {
		if convoHandleTimeFilter.LowerBound == nil && convoHandleTimeFilter.UpperBound == nil {
			return nil, status.Error(codes.InvalidArgument, "at least one of lower_bound or upper_bound of conversation_handle_time should be set")
		}
		floatRange := elasticsearch.FloatRange{}
		if lower := convoHandleTimeFilter.LowerBound; lower != nil {
			if lower.IsExclusive {
				floatRange.GT = proto.Float64(float64(lower.Value))
			} else {
				floatRange.GTE = proto.Float64(float64(lower.Value))
			}
		}
		if upper := convoHandleTimeFilter.UpperBound; upper != nil {
			if upper.IsExclusive {
				floatRange.LT = proto.Float64(float64(upper.Value))
			} else {
				floatRange.LTE = proto.Float64(float64(upper.Value))
			}
		}
		convoFilter.ConversationHandleTimeSecs = &floatRange
	}

	if durationRange != nil && (durationRange.MinDuration != nil || durationRange.MaxDuration != nil) {
		floatRange := elasticsearch.FloatRange{}
		if durationRange.MinDuration != nil {
			floatRange.GTE = proto.Float64(float64(durationRange.MinDuration.Seconds))
		}
		if durationRange.MaxDuration != nil {
			floatRange.LT = proto.Float64(float64(durationRange.MaxDuration.Seconds))
		}
		convoFilter.ConversationDurationSecs = &floatRange
	}
	for _, convo := range filterConversationByAttribute.Conversations {
		// TODO: Remove this if else when Director's FE passes the proper conversation_platform_id.
		// Currently FE passes "unknown" as a placeholder when name is passed.
		if convo.Name != "" {
			convoName, err := convo.ParseResourceName()
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			convoFilter.ConversationIDs = append(convoFilter.ConversationIDs, convoName.GetConversationID())
		} else if platformInfo := convo.PlatformInfo; platformInfo != nil && platformInfo.PlatformConversationId != "" {
			convoFilter.PlatformConversationIDs = append(convoFilter.PlatformConversationIDs, platformInfo.PlatformConversationId)
		}
	}
	for _, convo := range filterConversationByAttribute.ConversationIdentifiers {
		if n := convo.Name; n != "" {
			name, err := convopb.ParseConversationName(n)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			convoFilter.ConversationIDs = append(convoFilter.ConversationIDs, name.ConversationID)
		}
		if id := convo.PlatformConversationId; id != "" {
			convoFilter.PlatformConversationIDs = append(convoFilter.PlatformConversationIDs, id)
		}
	}

	if len(convoFilter.PlatformConversationIDs) > 0 && len(convoFilter.ConversationIDs) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "found both platform_conversation_id filter and conversation_id filter, at most one is allowed")
	}

	convoFilter.LanguageCodes = filterConversationByAttribute.LanguageCodes

	usecaseIDs, err := parseUsercaseNames(filterConversationByAttribute.UsecaseNames)
	if err != nil {
		return nil, err
	}
	convoFilter.UsecaseIDs = usecaseIDs

	tagIDs := []string{}
	for _, tag := range filterConversationByAttribute.Tags {
		name, err := tagpb.ParseTagName(tag.Name)
		if err != nil {
			return nil, err
		}
		tagIDs = append(tagIDs, name.TagID)
	}
	convoFilter.ConversationTagIDs = tagIDs

	convoFilter.Comments = filterConversationByAttribute.Comments
	convoFilter.AIAgentFeedback = filterConversationByAttribute.AiAgentFeedback

	convoFilter.IncludeDevUser = filterConversationByAttribute.IncludeDevUsers

	for _, scorecardTemplateName := range filterConversationByAttribute.ScorecardTemplateNames {
		scorecardTemplate, err := coachingpb.ParseScorecardTemplateName(scorecardTemplateName)
		if err != nil {
			return nil, err
		}
		convoFilter.ScorecardTemplateIDs = append(convoFilter.ScorecardTemplateIDs, scorecardTemplate.GetScorecardTemplateID())
	}
	convoFilter.ScorecardStatuses = filterConversationByAttribute.ScorecardStatuses
	// Remove this after scorecard_status is deprecated.
	if v := filterConversationByAttribute.ScorecardStatus; v != analyticspb.Attribute_SCORECARD_STATUS_UNSPECIFIED {
		convoFilter.ScorecardStatuses = append(convoFilter.ScorecardStatuses, v)
	}
	convoFilter.ScorecardStatusFilters = filterConversationByAttribute.ScorecardStatusFilters
	convoFilter.LiveAssists = filterConversationByAttribute.LiveAssists
	// Remove this after live_assist is deprecated.
	if v := filterConversationByAttribute.LiveAssist; v != analyticspb.Attribute_LIVE_ASSIST_UNSPECIFIED {
		convoFilter.LiveAssists = append(convoFilter.LiveAssists, v)
	}

	convoFilter.SummaryUsages = filterConversationByAttribute.SummaryUsages
	convoFilter.SuggestionUsages = filterConversationByAttribute.SuggestionUsages
	convoFilter.NoteTakingUsages = filterConversationByAttribute.NoteTakingUsages
	convoFilter.ConversationOnHoldCriteria = filterConversationByAttribute.ConversationHoldsCriterion

	if messageCountFilter := filterConversationByAttribute.MessageCountRange; messageCountFilter != nil {
		if messageCountFilter.LowerBound == nil && messageCountFilter.UpperBound == nil {
			return nil, status.Error(codes.InvalidArgument, "at least one of lower_bound or upper_bound of message_count_range should be set")
		}
		convoFilter.MessageCountRange, err = parseMessageCountRangeToFloatRange(messageCountFilter)
		if err != nil {
			return nil, err
		}
	}

	for _, gw := range filterConversationByAttribute.GuidedWorkflows {
		name, err := gwpb.ParseGuidedWorkflowName(gw.Name)
		if err != nil {
			return nil, err
		}
		convoFilter.GWDocumentIDs = append(convoFilter.GWDocumentIDs, name.GuidedWorkflowID)
	}

	for _, kb := range filterConversationByAttribute.KbDocuments {
		name, err := kbpb.ParseDocumentName(kb.Name)
		if err != nil {
			return nil, err
		}
		convoFilter.KBDocumentIDs = append(convoFilter.KBDocumentIDs, name.GetDocID())
	}

	convoFilter.LanguageTransfers = filterConversationByAttribute.LanguageTransferEvents

	convoMoments := []*analyticspb.MomentGroup{}
	msgMoments := []*analyticspb.MomentGroup{}
	for _, mg := range filterConversationByAttribute.MomentGroups {
		if isConvoSentiment(mg) || elasticsearch.IsConversationMetadataMomentGroup(mg) || elasticsearch.IsConversationOutcomeMomentGroup(mg) || elasticsearch.IsVoicemailMomentGroup(mg) {
			convoMoments = append(convoMoments, mg)
		} else {
			msgMoments = append(msgMoments, mg)
		}
	}
	convoFilter.MomentGroups = append(convoFilter.MomentGroups, convoMoments...)

	// Populate ChildMessageFilter.
	esSpeakerRoles, err := shared.ConvertAPIToESSpeakerRoles(filterConversationByAttribute.SpeakerRoles)
	if err != nil {
		return nil, err
	}
	convoFilter.ScreenRecording = filterConversationByAttribute.ScreenRecording
	if areMessageFiltersPresent(filterConversationByAttribute, msgMoments, esSpeakerRoles) {
		convoFilter.ChildMessageFilter = &elasticsearch.MessageFilterParams{
			MomentGroups:        msgMoments,
			ActionGroups:        filterConversationByAttribute.ActionGroups,
			SpeakerRoles:        esSpeakerRoles,
			KeywordsExpressions: filterConversationByAttribute.KeywordsExpressions,
			KnowledgeAssist:     filterConversationByAttribute.KnowledgeMomentFilter,
			// Insights composite moments in filterConversationByAttribute are routed to the child
			// message filter. Carry the conversation-level language codes so their included/excluded
			// keywords drill down to the matching message_text_with_custom_analyzer.<language> field
			// instead of fanning out across every supported analyzer.
			LanguageCodes: filterConversationByAttribute.LanguageCodes,
		}
	}

	// If filterConversationByAttribute.ConversationSources set, only values in the field are used in ES query.
	for _, conversationSource := range filterConversationByAttribute.ConversationSources {
		convoFilter.OverrideConversationSources = append(convoFilter.OverrideConversationSources, int32(conversationSource))
	}

	// If filterConversationByAttribute.ConversationChannels set, only conversations with those channels match.
	for _, conversationChannel := range filterConversationByAttribute.ConversationChannels {
		convoFilter.ConversationChannels = append(convoFilter.ConversationChannels, int32(conversationChannel))
	}

	scorecardSubmitterUserIDs := []string{}

	for _, userName := range filterConversationByAttribute.ScorecardSubmitterUserNames {
		parsedName, err := userpb.ParseUserName(userName)
		if err != nil {
			return nil, err
		}
		scorecardSubmitterUserIDs = append(scorecardSubmitterUserIDs, parsedName.UserID)
	}
	convoFilter.ScorecardSubmitterUserIDs = scorecardSubmitterUserIDs
	convoFilter.CalibrationStatuses = filterConversationByAttribute.CalibrationStatuses
	convoFilter.PlatformVisitorID = filterConversationByAttribute.ConversationPlatformVisitorId
	convoFilter.TransferInfo = filterConversationByAttribute.TransferInfoFilter
	convoFilter.VirtualAgentRevisionIDs = filterConversationByAttribute.VirtualAgentRevisionIds
	convoFilter.IngestedByJobIDs = filterConversationByAttribute.GetIngestedByJobIds()
	convoFilter.DisplayNames = filterConversationByAttribute.ConversationDisplayNames
	convoFilter.EmailAttachments = filterConversationByAttribute.GetEmailAttachments()

	return &convoFilter, nil
}

func parseMessageCountRangeToFloatRange(messageCountFilter *analyticspb.IntRange) (*elasticsearch.FloatRange, error) {
	if messageCountFilter == nil {
		return nil, nil
	}
	if messageCountFilter.LowerBound == nil && messageCountFilter.UpperBound == nil {
		return nil, status.Error(codes.InvalidArgument, "at least one of lower_bound or upper_bound of message_count_range should be set")
	}
	floatRange := elasticsearch.FloatRange{}
	if lower := messageCountFilter.LowerBound; lower != nil {
		if lower.IsExclusive {
			floatRange.GT = proto.Float64(float64(lower.Value))
		} else {
			floatRange.GTE = proto.Float64(float64(lower.Value))
		}
	}
	if upper := messageCountFilter.UpperBound; upper != nil {
		if upper.IsExclusive {
			floatRange.LT = proto.Float64(float64(upper.Value))
		} else {
			floatRange.LTE = proto.Float64(float64(upper.Value))
		}
	}
	return &floatRange, nil
}

func areMessageFiltersPresent(filterConversationByAttribute *analyticspb.Attribute, messageMomentGroups []*analyticspb.MomentGroup, esSpeakerRoles []string) bool {
	return len(messageMomentGroups) > 0 ||
		len(esSpeakerRoles) > 0 ||
		len(filterConversationByAttribute.ActionGroups) > 0 ||
		len(filterConversationByAttribute.KeywordsExpressions) > 0 ||
		filterConversationByAttribute.KnowledgeMomentFilter != nil
}

func parseUsercaseNames(usecaseNames []string) ([]string, error) {
	usecaseIDs := []string{}
	for _, usecaseResourceName := range usecaseNames {
		customerProfileUsecaseName, err := shared.ParseCustomerProfileUsecase(usecaseResourceName)
		if err != nil {
			return nil, err
		}
		usecaseIDs = append(usecaseIDs, customerProfileUsecaseName.UsecaseID)
	}
	return usecaseIDs, nil
}

func isConvoSentiment(mg *analyticspb.MomentGroup) bool {
	if mg == nil {
		return false
	}
	if mg.MomentType == momentpb.Moment_SENTIMENT {
		return true
	}
	for _, m := range mg.Moments {
		if m.Type == momentpb.Moment_SENTIMENT {
			return true
		}
	}
	for _, m := range mg.ExcludedMoments {
		if m.Type == momentpb.Moment_SENTIMENT {
			return true
		}
	}
	return false
}

func parseTimeRange(timeRange *analyticspb.TimeRange, timeZone string) *elasticsearch.TimeRange {
	return &elasticsearch.TimeRange{
		GTE:      timeRange.StartTimestamp.AsTime().Format(time.RFC3339),
		LT:       timeRange.EndTimestamp.AsTime().Format(time.RFC3339),
		TimeZone: timeZone,
	}
}

func parseDurationBuckets(
	durationBuckets []*analyticspb.ConversationDurationBucket,
	referenceBuckets []*analyticspb.ConversationDurationBucket,
) []*elasticsearch.FloatRange {
	referenceByID := make(map[string]*analyticspb.ConversationDurationBucket, len(referenceBuckets))
	for _, bucket := range referenceBuckets {
		referenceByID[bucket.Id] = bucket
	}

	durationWindows := make([]*elasticsearch.FloatRange, 0, len(durationBuckets))
	seenBucketIDs := make(map[string]struct{}, len(durationBuckets))
	for _, bucket := range durationBuckets {
		referenceBucket, exists := referenceByID[bucket.Id]
		if !exists {
			continue
		}
		if _, alreadySelected := seenBucketIDs[bucket.Id]; alreadySelected {
			continue
		}
		seenBucketIDs[bucket.Id] = struct{}{}
		durationWindows = append(durationWindows, durationBucketToFloatRange(referenceBucket))
	}

	sort.Slice(durationWindows, func(i, j int) bool {
		return *durationWindows[i].GTE < *durationWindows[j].GTE
	})
	if len(durationWindows) == 0 {
		return nil
	}
	return mergeConsecutiveDurationWindows(durationWindows)
}

func durationBucketToFloatRange(bucket *analyticspb.ConversationDurationBucket) *elasticsearch.FloatRange {
	lowerbound := bucket.LowerBound
	result := &elasticsearch.FloatRange{
		GTE: &lowerbound,
	}
	if bucket.UpperBound == 0 {
		return result
	}
	upperbound := bucket.UpperBound
	result.LT = &upperbound
	return result
}

func mergeConsecutiveDurationWindows(durationWindows []*elasticsearch.FloatRange) []*elasticsearch.FloatRange {
	if len(durationWindows) == 0 {
		return nil
	}

	merged := make([]*elasticsearch.FloatRange, 0, len(durationWindows))
	runStart := durationWindows[0]
	runEnd := durationWindows[0]
	for i := 1; i < len(durationWindows); i++ {
		candidateWindow := durationWindows[i]
		if runEnd.LT != nil && candidateWindow.GTE != nil && *runEnd.LT == *candidateWindow.GTE {
			runEnd = candidateWindow
			continue
		}
		merged = append(merged, mergeDurationWindowRun(runStart, runEnd))
		runStart = candidateWindow
		runEnd = candidateWindow
	}
	return append(merged, mergeDurationWindowRun(runStart, runEnd))
}

func mergeDurationWindowRun(runStart, runEnd *elasticsearch.FloatRange) *elasticsearch.FloatRange {
	result := &elasticsearch.FloatRange{}
	if runStart.GTE != nil {
		lowerbound := *runStart.GTE
		result.GTE = &lowerbound
	}
	if runEnd.LT != nil {
		upperbound := *runEnd.LT
		result.LT = &upperbound
	}
	return result
}

func clickhouseConditionForFloatRange(columnName string, floatRange *elasticsearch.FloatRange) (string, []any) {
	if floatRange == nil {
		return "", nil
	}
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if val := floatRange.GT; val != nil {
		condition, arg := columnGT(columnName, *val)
		conditions = append(conditions, condition)
		args = append(args, arg)
	}
	if val := floatRange.GTE; val != nil {
		condition, arg := columnGTE(columnName, *val)
		conditions = append(conditions, condition)
		args = append(args, arg)
	}
	if val := floatRange.LT; val != nil {
		condition, arg := columnLT(columnName, *val)
		conditions = append(conditions, condition)
		args = append(args, arg)
	}
	if val := floatRange.LTE; val != nil {
		condition, arg := columnLTE(columnName, *val)
		conditions = append(conditions, condition)
		args = append(args, arg)
	}
	if len(conditions) == 0 {
		return "", nil
	}
	if len(conditions) == 1 {
		return conditions[0], args
	}
	return strings.Join(conditions, " AND "), args
}

func clickhouseConditionForFloatRanges(columnName string, floatRanges []*elasticsearch.FloatRange) (string, []any) {
	if len(floatRanges) == 0 {
		return "", nil
	}
	if len(floatRanges) == 1 {
		return clickhouseConditionForFloatRange(columnName, floatRanges[0])
	}

	conditions := make([]string, 0, len(floatRanges))
	args := make([]any, 0, len(floatRanges)*2)
	for _, floatRange := range floatRanges {
		condition, rangeArgs := clickhouseConditionForFloatRange(columnName, floatRange)
		if condition == "" {
			continue
		}
		conditions = append(conditions, fmt.Sprintf("(%s)", condition))
		args = append(args, rangeArgs...)
	}
	if len(conditions) == 0 {
		return "", nil
	}
	if len(conditions) == 1 {
		return conditions[0], args
	}
	return strings.Join(conditions, " OR "), args
}

func appendClickhouseFloatRangeConditions(
	conditionsAndArgs []conditionAndArg,
	columnName string,
	floatRanges []*elasticsearch.FloatRange,
) []conditionAndArg {
	if len(floatRanges) == 0 {
		return conditionsAndArgs
	}
	if len(floatRanges) == 1 {
		floatRange := floatRanges[0]
		if val := floatRange.GT; val != nil {
			condition, arg := columnGT(columnName, *val)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition, arg: arg})
		}
		if val := floatRange.GTE; val != nil {
			condition, arg := columnGTE(columnName, *val)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition, arg: arg})
		}
		if val := floatRange.LT; val != nil {
			condition, arg := columnLT(columnName, *val)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition, arg: arg})
		}
		if val := floatRange.LTE; val != nil {
			condition, arg := columnLTE(columnName, *val)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition, arg: arg})
		}
		return conditionsAndArgs
	}

	condition, args := clickhouseConditionForFloatRanges(columnName, floatRanges)
	if condition != "" {
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition, args: args})
	}
	return conditionsAndArgs
}

// Returns the resource names of composite moments representing topics in API request.
func getInsightsCompositeMoments(
	filterConversationByAttribute *analyticspb.Attribute,
	filterMessageByAttribute *analyticspb.Attribute,
	groupByAttributes []*analyticspb.AttributeStructure,
) ([]string, bool) {
	momentNames, hasCompositeMoment := map[string]bool{}, false
	// From FilterConversationByAttribute.
	if att := filterConversationByAttribute; att != nil {
		for _, momentGroup := range att.MomentGroups {
			for _, m := range momentGroup.Moments {
				if elasticsearch.IsInsightsCompositeMoment(m) {
					momentNames[m.Name] = true
					hasCompositeMoment = true
				}
			}
			for _, m := range momentGroup.ExcludedMoments {
				if elasticsearch.IsInsightsCompositeMoment(m) {
					momentNames[m.Name] = true
					hasCompositeMoment = true
				}
			}
		}
		if len(att.InsightsTopics) > 0 {
			hasCompositeMoment = true
		}
	}
	// From FilterMessageByAttribute.
	if att := filterMessageByAttribute; att != nil {
		for _, momentGroup := range att.MomentGroups {
			for _, m := range momentGroup.Moments {
				if elasticsearch.IsInsightsCompositeMoment(m) {
					momentNames[m.Name] = true
					hasCompositeMoment = true
				}
			}
			for _, m := range momentGroup.ExcludedMoments {
				if elasticsearch.IsInsightsCompositeMoment(m) {
					momentNames[m.Name] = true
					hasCompositeMoment = true
				}
			}
		}
		if len(att.InsightsTopics) > 0 {
			hasCompositeMoment = true
		}
	}
	// From GroupByAttributes.
	for _, att := range groupByAttributes {
		if att.GetMomentGroup() != nil {
			for _, m := range att.GetMomentGroup().GetMoments() {
				if elasticsearch.IsInsightsCompositeMoment(m) {
					momentNames[m.Name] = true
					hasCompositeMoment = true
				}
			}
		}
	}

	res := []string{}
	for name := range momentNames {
		res = append(res, name)
	}
	return res, hasCompositeMoment
}

func getMetadataMomentGroups(attribute *analyticspb.Attribute) []*analyticspb.MomentGroup {
	if attribute == nil {
		return nil
	}
	ret := []*analyticspb.MomentGroup{}
	for _, mg := range attribute.MomentGroups {
		if elasticsearch.IsConversationMetadataMomentGroup(mg) {
			ret = append(ret, mg)
		}
	}
	return ret
}

func getESQuery(
	timeRange *analyticspb.TimeRange,
	metadata *analyticspb.Metadata,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	filterConversationByAttribute,
	filterMessageByAttribute,
	filterConversationDocumentByConversationByAttribute *analyticspb.Attribute,
	groupByAttributes []*analyticspb.AttributeStructure,
	topicDetails map[string]elasticsearch.TopicDetails,
	db *gorm.DB,
	esQueryConfig elasticsearch.QueryConfig,
	allowFilteringByConvoFieldsInMsg bool,
) (*elasticsearch.OuterQuery, error) {
	return getESQueryInternal(
		timeRange,
		metadata,
		durationBuckets,
		filterConversationByAttribute,
		filterMessageByAttribute,
		filterConversationDocumentByConversationByAttribute,
		groupByAttributes,
		topicDetails,
		db,
		esQueryConfig,
		allowFilteringByConvoFieldsInMsg,
		*enableDeprecatePostgresUserIDQuery)
}

// Returns the Query phrase in the ES request.
func getESQueryInternal(
	timeRange *analyticspb.TimeRange,
	metadata *analyticspb.Metadata,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	filterConversationByAttribute,
	filterMessageByAttribute,
	filterConversationDocumentByConversationByAttribute *analyticspb.Attribute,
	groupByAttributes []*analyticspb.AttributeStructure,
	topicDetails map[string]elasticsearch.TopicDetails,
	db *gorm.DB,
	esQueryConfig elasticsearch.QueryConfig,
	allowFilteringByConvoFieldsInMsg bool,
	deprecatePostgresUserIDQuery bool,
) (*elasticsearch.OuterQuery, error) {
	timeZone, err := shared.ParseTimeZone(metadata)
	if err != nil {
		return nil, err
	}
	var userIDs []string
	if deprecatePostgresUserIDQuery {
		if filterConversationByAttribute != nil {
			userIDs, err = extractUserIDsFromUsers(filterConversationByAttribute.Users)
			if err != nil {
				return nil, err
			}
		}
	} else {
		userIDs, err = postgres.GetUserIDsInDB(db, filterConversationByAttribute)
		if err != nil {
			return nil, err
		}
	}

	convoDocFilter, err := parseConversationFilterParams(filterConversationByAttribute, filterConversationDocumentByConversationByAttribute, timeRange, timeZone, durationBuckets, userIDs)
	if err != nil {
		return nil, err
	}

	onlyDenormalizedFieldsInConvoFilters := allowFilteringByConvoFieldsInMsg && elasticsearch.ConvoFiltersOnlyContainDenormalizedFields(convoDocFilter)
	// If all conversation filters are applicable to message documents, rely solely on
	// message-level filters to avoid parent joins.
	if onlyDenormalizedFieldsInConvoFilters {
		filterConversationByAttribute, filterMessageByAttribute = moveConvoLevelFiltersToMessageFilter(filterConversationByAttribute, filterMessageByAttribute)
		convoDocFilter, err = parseConversationFilterParams(filterConversationByAttribute, filterConversationDocumentByConversationByAttribute, timeRange, timeZone, durationBuckets, userIDs)
		if err != nil {
			return nil, err
		}
	}
	msgDocFilter, err := parseMessageFilterParams(
		filterMessageByAttribute,
		timeRange,
		timeZone,
		userIDs,
		onlyDenormalizedFieldsInConvoFilters,
		convoDocFilter.TargetFieldForTimeRangeFilter,
	)

	if err != nil {
		return nil, err
	}

	targetDocumentType := inferTargetDocumentType(groupByAttributes, filterMessageByAttribute, esQueryConfig.EsDocumentType, onlyDenormalizedFieldsInConvoFilters)
	// TODO: we should avoid applying the save filter from topic details in "query" part,
	// to make the ES query shorter and run faster.
	query, err := elasticsearch.ParseFilterByAttributeToQuery(
		convoDocFilter,
		msgDocFilter,
		targetDocumentType,
		topicDetails,
		esQueryConfig)
	if err != nil {
		return nil, err
	}

	return &query, nil
}

func inferTargetDocumentType(groupByAttributes []*analyticspb.AttributeStructure, filterMessageByAttribute *analyticspb.Attribute, documentType elasticsearch.DocumentType, onlyMessageDocFields bool) elasticsearch.DocumentType {
	// If the document type is already set to conversation, return it.
	if documentType == elasticsearch.DocumentTypeConversation && !onlyMessageDocFields {
		return elasticsearch.DocumentTypeConversation
	}

	// Check filterMessageByAttribute for TOPIC moments. V3 categories are message-anchored, so
	// they stay on the message document rather than routing to message_group.
	if filterMessageByAttribute != nil {
		for _, mg := range filterMessageByAttribute.MomentGroups {
			for _, m := range mg.Moments {
				if m.Type == momentpb.Moment_TOPIC && !elasticsearch.IsTopicDiscoveryV3Moment(m) {
					return elasticsearch.DocumentTypeMessageGroup
				}
			}
		}
	}

	// Check groupByAttributes for TOPIC moments
	for _, v := range groupByAttributes {
		if v.GetMomentGroup() != nil {
			for _, m := range v.GetMomentGroup().GetMoments() {
				if m.Type == momentpb.Moment_TOPIC && !elasticsearch.IsTopicDiscoveryV3Moment(m) {
					return elasticsearch.DocumentTypeMessageGroup
				}
			}
		}
	}

	// Default return when no TOPIC moments are found
	return elasticsearch.DocumentTypeMessage
}

func validateMomentGroups(mgs []*analyticspb.MomentGroup) error {
	for _, mg := range mgs {
		if mg == nil {
			return status.Error(codes.InvalidArgument, "invalid nil MomentGroup")
		}
		if mg.Adherence != enumpb.AdherenceType_ADHERENCE_TYPE_UNSPECIFIED && len(mg.AdherenceTypesForMoments) != 0 {
			return status.Error(codes.InvalidArgument, "at most one of field 'adherence' and adherence_type_for_moments can be provided")
		}

		if l := len(mg.AdherenceTypesForMoments); l != 0 && l != len(mg.Moments) && l != len(mg.Behaviors) {
			return status.Error(codes.InvalidArgument, "when adherence_types_for_moments is not empty, its length has to be the same of 'moments' or 'behaviors'")
		}
	}
	return nil
}

func RemoveConvoLevelMomentsFromMessageFilter(
	filterConversationByAttribute,
	filterMessageByAttribute *analyticspb.Attribute,
) (*analyticspb.Attribute, *analyticspb.Attribute) {
	if filterMessageByAttribute == nil || len(filterMessageByAttribute.MomentGroups) == 0 {
		return filterConversationByAttribute, filterMessageByAttribute
	}

	convoLevelMomentGroupInFilterMessage := []*analyticspb.MomentGroup{}
	messageLevelMomentGroupInFilterMessage := []*analyticspb.MomentGroup{}
	for _, mg := range filterMessageByAttribute.MomentGroups {
		if elasticsearch.IsConversationMetadataMomentGroup(mg) || elasticsearch.IsConversationOutcomeMomentGroup(mg) {
			convoLevelMomentGroupInFilterMessage = append(convoLevelMomentGroupInFilterMessage, mg)
		} else {
			messageLevelMomentGroupInFilterMessage = append(messageLevelMomentGroupInFilterMessage, mg)
		}
	}
	if len(convoLevelMomentGroupInFilterMessage) == 0 {
		return filterConversationByAttribute, filterMessageByAttribute
	}
	filterMessageByAttribute.MomentGroups = messageLevelMomentGroupInFilterMessage

	if filterConversationByAttribute == nil {
		filterConversationByAttribute = &analyticspb.Attribute{}
	}
	filterConversationByAttribute.MomentGroups = append(filterConversationByAttribute.MomentGroups, convoLevelMomentGroupInFilterMessage...)
	return filterConversationByAttribute, filterMessageByAttribute
}

func moveConvoLevelFiltersToMessageFilter(
	filterConversationByAttribute,
	filterMessageByAttribute *analyticspb.Attribute,
) (*analyticspb.Attribute, *analyticspb.Attribute) {
	if filterConversationByAttribute == nil {
		return filterConversationByAttribute, filterMessageByAttribute
	}
	if filterMessageByAttribute == nil {
		filterMessageByAttribute = &analyticspb.Attribute{}
	}

	// Move conversation sources to message filter.
	filterMessageByAttribute.IncludeCrestaBot = filterConversationByAttribute.IncludeCrestaBot
	filterConversationByAttribute.IncludeCrestaBot = false
	filterMessageByAttribute.ConversationSources = filterConversationByAttribute.ConversationSources
	filterConversationByAttribute.ConversationSources = []convopb.Conversation_Source{}

	// Move message count range to message filter.
	filterMessageByAttribute.MessageCountRange = filterConversationByAttribute.MessageCountRange
	filterConversationByAttribute.MessageCountRange = nil

	// Move conversations to message filter.
	filterMessageByAttribute.Conversations = filterConversationByAttribute.Conversations
	filterConversationByAttribute.Conversations = []*convopb.Conversation{}

	// Move platform conversation IDs to message filter.
	filterMessageByAttribute.ConversationIdentifiers = filterConversationByAttribute.ConversationIdentifiers
	filterConversationByAttribute.ConversationIdentifiers = []*analyticspb.Attribute_ConversationIdentifier{}

	// Move usecase names to message filter.
	if len(filterMessageByAttribute.UsecaseNames) == 0 {
		filterMessageByAttribute.UsecaseNames = filterConversationByAttribute.UsecaseNames
	}
	filterConversationByAttribute.UsecaseNames = []string{}

	return filterConversationByAttribute, filterMessageByAttribute
}
