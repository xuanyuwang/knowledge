package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	// https://stackoverflow.com/questions/47794740/gos-time-doesnt-work-under-the-docker-image-from-scratch
	_ "time/tzdata"

	actionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/action"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	labelspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	convpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	intentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/intent"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	policypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/policy"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cresta/go-servers/apiserver/sql-schema/gen/dao"
	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/shared/db/query"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/conversation/converter"
	"github.com/cresta/go-servers/shared/db/condition"
)

type AttributeKeys struct {
	GroupByKey       string
	GroupBySecondKey string
}

type KeyTimeRow struct {
	GroupByKey             string
	GroupByKeyAlias1       string
	GroupByKeyAlias2       string
	GroupByKeyAlias3       string
	GroupByKeyAlias4       string
	GroupByKeyAlias5       string
	GroupByKeyAlias6       string
	GroupByKeyAlias7       string
	GroupBySecondKey       string
	GroupBySecondKeyAlias1 string
	GroupBySecondKeyAlias2 string
	GroupBySecondKeyAlias3 string
	GroupBySecondKeyAlias4 string
	GroupBySecondKeyAlias5 string
	GroupBySecondKeyAlias6 string
	GroupBySecondKeyAlias7 string
	TruncatedCreatedAt     time.Time
}

const (
	TrueCondition     = "TRUE"
	AgentIDColumn     = "agent_id"
	AgentUserIDColumn = "agent_user_id"
	// AllUserTeamTeamIDsColumn            = "all_user_team.team_ids"
	// AllUserTeamTeamDisplayNamesColumn   = "all_user_team.team_display_names"
	// AllUserTeamTeamManagerIDsColumn     = "all_user_team.manager_user_ids"
	// AllUserTeamUserIDColumn             = "all_user_team.user_id"
	// AllUserTeamUserFullNameColumn       = "all_user_team.user_full_name"
	// AllUserTeamUserNameColumn           = "all_user_team.username"
	// AllUserTeamProfilePicturePathColumn = "all_user_team.profile_picture_path"
	BehaviorIDColumn          = "app.behaviors.behavior_id"
	BehaviorDisplayNameColumn = "app.behaviors.display_name"
	MomentTemplateIDColumn    = "app.moment_templates.moment_template_id"
	// use citext since the behavior id in historic.agent_behavior_hourly_conversation_stats is citext
	MomentTemplateTaxonomyColumn     = "app.moment_templates.taxonomy::citext"
	MomentTemplateDisplayNameColumn  = "app.moment_templates.display_name"
	MomentTemplateDisplayOrderColumn = "app.moment_templates.display_order"
	PolicyIDColumn                   = "app.policies.policy_id"
	PolicyDisplayNameColumn          = "app.policies.display_name"
	PolicySourceTypeColumn           = "app.policies.source_type"
	PolicyLabelsColumn               = "app.policies.labels::jsonb->'labels'"
	ConversationReasonColumn         = "conversation_reason"
	ConversationDurationBinsIDColumn = "conversation_duration_bins_id"
	GuidedWorkflowNameColumn         = "workflow_name"
	ChatBotMajorVersionColumn        = "major_version"
	ChatBotMinorVersionColumn        = "minor_version"
	ChatBotRevisionVersionColumn     = "revision_version"
	ChatBotIntentNameColumn          = "intent_name"
	ChatBotUserIDColumn              = "all_users.user_id"
	ChatBotUserResponseTypeColumn    = "user_response_type"
	ChatBotDeflectionTypeColumn      = "deflection_type"
	AppUserUserDisplayName           = "app.users.display_name"
	SpeakerRoleColumn                = "speaker_role"
	ConversationPropertyColumn       = "conversation_property"
	ScorecardTemplateIDColumn        = "scorecard_template_id"
	AiScoredColumn                   = "ai_scored"
	AiScoredType                     = 2
	ManuallyScoredColumn             = "manually_scored"
	ManuallyScoredType               = 1
	ScoreTypeColumn                  = "score_type"
	NotApplicableTypeColumn          = "not_applicable_type"
	NotApplicableTypeApplicable      = 0
	NotApplicableTypeNotApplicable   = 1
	CriterionIDColumn                = "criterion_identifier"
	PercentageValueColumn            = "percentage_value"
	WeightedPercentageSumColumn      = "weighted_percentage_sum"
	FloatWeightedPercentageSumColumn = "float_weighted_percentage_sum"
	NotApplicableColumn              = "not_applicable"
	UsecaseIDColumn                  = "usecase_id"
	LanguageCodeColumn               = "language_code"
	IsVoiceMailColumn                = "is_voice_mail"
)

var (
	CustomerRegex                                  = regexp.MustCompile("^customers/([^/]+)$")
	CustomerProfileRegex                           = regexp.MustCompile("^customers/([^/]+)/profiles/([^/]+)$")
	PostgresArrayToStringRegex                     = regexp.MustCompile("^{(.*)}$")
	PostgresArrayWithDoubleQWuotationToStringRegex = regexp.MustCompile("^{\"(.*)\"}$")
)

const (
	// ConversationPropertyColumnVoiceMailValue 1 is the value of entries that are identified as voice mail
	ConversationPropertyColumnVoiceMailValue = 1
)

type ActionRow struct {
	Title        string
	DisplayName  string
	Description  string
	Suggestion   string
	ActionID     string
	ActionName   string
	DetailedType int
	Type         int
	Taxonomy     string
}

func (r KeyTimeRow) GetAttributeKeys() AttributeKeys {
	return AttributeKeys{GroupByKey: r.GroupByKey, GroupBySecondKey: r.GroupBySecondKey}
}

func (r KeyTimeRow) GetAttributes() [][]string {
	return [][]string{
		{r.GroupByKey, r.GroupByKeyAlias1, r.GroupByKeyAlias2, r.GroupByKeyAlias3, r.GroupByKeyAlias4, r.GroupByKeyAlias5, r.GroupByKeyAlias6, r.GroupByKeyAlias7},
		{r.GroupBySecondKey, r.GroupBySecondKeyAlias1, r.GroupBySecondKeyAlias2, r.GroupBySecondKeyAlias3, r.GroupBySecondKeyAlias4, r.GroupBySecondKeyAlias5, r.GroupBySecondKeyAlias6, r.GroupBySecondKeyAlias7},
	}
}
func (r KeyTimeRow) GetTruncatedCreatedAt() time.Time { return r.TruncatedCreatedAt }

func attributeTypeToGroupByKeys() map[analyticspb.AttributeType][]string {
	intentGroupByKeys := []string{
		MomentTemplateTaxonomyColumn, MomentTemplateDisplayNameColumn, MomentTemplateDisplayOrderColumn, MomentTemplateIDColumn,
	}
	return map[analyticspb.AttributeType][]string{
		analyticspb.AttributeType_ATTRIBUTE_TYPE_UNSPECIFIED: {"'*'", "'*'", "'*'"}, // Can we remove this?
		analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT: {
			AgentUserIDColumn,
			// AllUserTeamUserIDColumn, AllUserTeamUserFullNameColumn, AllUserTeamUserNameColumn, AllUserTeamTeamIDsColumn,
			// AllUserTeamTeamDisplayNamesColumn, AllUserTeamProfilePicturePathColumn,
		},
		analyticspb.AttributeType_ATTRIBUTE_TYPE_INTENT: intentGroupByKeys,
		analyticspb.AttributeType_ATTRIBUTE_TYPE_CONVERSATION_DURATION_BUCKET: {
			ConversationDurationBinsIDColumn,
		},
		analyticspb.AttributeType_ATTRIBUTE_TYPE_GUIDED_WORKFLOW: {GuidedWorkflowNameColumn},
		analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY: {
			PolicyIDColumn, PolicyDisplayNameColumn, PolicySourceTypeColumn, PolicyLabelsColumn,
		},
		analyticspb.AttributeType_ATTRIBUTE_TYPE_MOMENT_POLICY: {
			MomentTemplateTaxonomyColumn, MomentTemplateDisplayNameColumn, MomentTemplateDisplayOrderColumn, MomentTemplateIDColumn,
		},
		analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR: {
			BehaviorIDColumn, BehaviorDisplayNameColumn,
		},
	}
}

func chatBotAttributeTypeToGroupByKeys() map[analyticspb.ChatBotAttributeType][]string {
	return map[analyticspb.ChatBotAttributeType][]string{
		analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_UNSPECIFIED:        {"'*'", "'*'", "'*'"},
		analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_INTENT:             {ChatBotIntentNameColumn, ChatBotIntentNameColumn, ChatBotIntentNameColumn},
		analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_USER_RESPONSE_TYPE: {ChatBotUserResponseTypeColumn, ChatBotUserResponseTypeColumn, ChatBotUserResponseTypeColumn},
		analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_DEFLECTION_TYPE:    {ChatBotDeflectionTypeColumn, ChatBotDeflectionTypeColumn, ChatBotDeflectionTypeColumn},
		analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_USER:               {ChatBotUserIDColumn, AppUserUserDisplayName, AppUserUserDisplayName},
		analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_VERSION:            {ChatBotMajorVersionColumn, ChatBotMinorVersionColumn, ChatBotRevisionVersionColumn},
	}
}

// columnIn returns a WHERE condition for checking that `columnName` contains one of the `values`.
func columnIn[T any](columnName string, values []T) condition.Conditions {
	return condition.New(fmt.Sprintf("%s IN ?", columnName), values)
}

// columnIn returns a WHERE condition for checking that `columnName` contains none of the `values`.
func columnNotIn[T any](columnName string, values []T) condition.Conditions {
	return condition.New(fmt.Sprintf("%s NOT IN ?", columnName), values)
}

// columnEq returns a WHERE condition for checking that `columnName` is equal to the `value`.
func columnEq[T any](columnName string, value T) condition.Conditions {
	return condition.New(fmt.Sprintf("%s = ?", columnName), value)
}

// columnNotEq returns a WHERE condition for checking that `columnName` is not equal to the `value`.
func columnNotEq[T any](columnName string, value T) condition.Conditions {
	return condition.New(fmt.Sprintf("%s <> ?", columnName), value)
}

// columnEq returns a WHERE condition for checking that `columnName` is greater than the `value`.
func columnGreater[T any](columnName string, value T) condition.Conditions {
	return condition.New(fmt.Sprintf("%s > ?", columnName), value)
}

// columnNotNull returns a WHERE condition for checking that `columnName` is NOT NULL.
func columnNotNull(columnName string) condition.Conditions {
	return condition.New(fmt.Sprintf("%s IS NOT NULL", columnName))
}

// columnNotNull returns a WHERE condition for checking that `columnName` is NOT NULL.
func columnBetween[T any](columnName string, start T, end T) condition.Conditions {
	return condition.New(fmt.Sprintf("%s BETWEEN ? AND ?", columnName), start, end)
}

// Note: This filter should be used via a gORM scope: https://gorm.io/docs/scopes.html#Query.
// nolint:nakedret
func ParseFilterByAttributeToCondition(
	filterByAttribute *analyticspb.Attribute,
) (scope func(db *gorm.DB) *gorm.DB, hasIntent, hasConversationReason, hasConversationDuration, hasAgent, hasMoment, hasPolicy, hasHintType, hasBehavior bool, returnedErr error) {
	var userConditions condition.ConditionsCollection
	var conditions condition.ConditionsCollection
	if filterByAttribute != nil {
		var err error
		hasAgent, userConditions, err = parseUserFiltersToConditions(filterByAttribute.GetUsers())
		if err != nil {
			returnedErr = status.Error(codes.InvalidArgument, err.Error())
			return
		}
		if intentFilter := filterByAttribute.Intents; intentFilter != nil {
			var intentIDs []string
			for _, intent := range intentFilter {
				intentName, err := intentpb.ParseIntentName(intent.Name)
				if err != nil {
					// TODO(INS-173): throw invalid_argument error.
					intentIDs = append(intentIDs, intent.Name)
				} else {
					intentIDs = append(intentIDs, intentName.IntentID)
				}
			}
			conditions = append(conditions, columnIn(MomentTemplateTaxonomyColumn, intentIDs))
			hasIntent = true
		}
		if momentFilter := filterByAttribute.Moments; momentFilter != nil {
			var momentIDs []string
			for _, moment := range momentFilter {
				momentName, err := momentpb.ParseMomentName(moment.Name)
				if err != nil {
					returnedErr = status.Error(codes.InvalidArgument, err.Error())
					return
				}
				momentIDs = append(momentIDs, momentName.MomentID)
			}
			conditions = append(conditions, columnIn(MomentTemplateIDColumn, momentIDs))
			hasMoment = true
		}
		if attributePolicies := filterByAttribute.AttributePolicies; len(attributePolicies) > 0 {
			var policyIDs []string
			for _, p := range attributePolicies {
				policyName, err := policypb.ParsePolicyName(p.Name)
				if err != nil {
					returnedErr = err
					return
				}
				policyIDs = append(policyIDs, policyName.PolicyID)
			}
			conditions = append(conditions, columnIn(PolicyIDColumn, policyIDs))
			hasPolicy = true
		}
		if speakerRoleFilter := filterByAttribute.SpeakerRoles; speakerRoleFilter != nil {
			conditions = append(conditions, columnIn(SpeakerRoleColumn, speakerRoleFilter))
		}
		if conversationReasonFilter := filterByAttribute.ConversationReasons; conversationReasonFilter != nil {
			var reasonIDs []string
			for _, conversationReason := range conversationReasonFilter {
				reasonIDs = append(reasonIDs, conversationReason.Id)
			}
			conditions = append(conditions, columnIn(ConversationReasonColumn, reasonIDs))
			hasConversationReason = true
		}
		if hintType := filterByAttribute.HintType; hintType != analyticspb.Attribute_HINT_TYPE_UNSPECIFIED {
			hasHintType = true
			// Determine a row in agent_behavior_hourly_conversation_stats belongs to a behavior hint or
			// regular hint. The rationale is that for behavior hint, sdx_count is always greater than 0, and for
			// regular hint, sdx_count always equals to 0.
			// The pre-condition is that when action_type is non-zero, hint_sent_count > 0
			// Check `select COUNT(*) from historic.agent_behavior_hourly_conversation_stats where action_type<>0 and hint_sent_count=0`
			actionType, err := HintTypeToActionType(hintType)
			if err != nil {
				returnedErr = err
				return
			}
			conditions = append(conditions, columnEq("action_type", actionType))
			if hintType == analyticspb.Attribute_REGULAR_HINT {
				conditions = append(conditions, columnEq("sdx_count", 0))
			}
			if hintType == analyticspb.Attribute_BEHAVIOR_HINT {
				conditions = append(conditions, columnGreater("sdx_count", 0))
			}
		}
		conditions, hasConversationDuration = appendConversationBucketConditions(conditions, filterByAttribute.ConversationDurationBuckets)
		if guidedWorkflowFilter := filterByAttribute.GuidedWorkflows; guidedWorkflowFilter != nil {
			var guidedWorkflowNames []string
			for _, guidedWorkflow := range guidedWorkflowFilter {
				guidedWorkflowNames = append(guidedWorkflowNames, guidedWorkflow.Name)
			}
			conditions = append(conditions, columnIn(GuidedWorkflowNameColumn, guidedWorkflowNames))
		}
		if conversationCriterionsFilter := filterByAttribute.ConversationCriterions; conversationCriterionsFilter != nil {
			voiceMailValue := []int{ConversationPropertyColumnVoiceMailValue}
			for _, criterion := range conversationCriterionsFilter {
				if criterion == analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL {
					conditions = append(conditions, columnNotIn(ConversationPropertyColumn, voiceMailValue))
				}
			}
		}
		if usecaseFilter := filterByAttribute.UsecaseNames; len(usecaseFilter) > 0 {
			var usecaseIDs []string
			for _, usecaseResourceName := range usecaseFilter {
				customerProfileUsecaseName, err := shared.ParseCustomerProfileUsecase(usecaseResourceName)
				if err != nil {
					// TODO(xiaoyi.ge): clean up the legacy usecase resource name parsing.
					customerUsecaseName, err2 := shared.ParseCustomerUsecase(usecaseResourceName)
					if err2 != nil {
						returnedErr = err
						return
					}
					usecaseIDs = append(usecaseIDs, customerUsecaseName.UsecaseID)
				} else {
					usecaseIDs = append(usecaseIDs, customerProfileUsecaseName.UsecaseID)
				}
			}
			conditions = append(conditions, columnIn(UsecaseIDColumn, usecaseIDs))
		}
		if languageCodesFilter := filterByAttribute.LanguageCodes; len(languageCodesFilter) > 0 {
			conditions = append(conditions, columnIn(LanguageCodeColumn, languageCodesFilter))
		}
		if behaviorFilter := filterByAttribute.Behaviors; len(behaviorFilter) > 0 {
			var behaviorIDs []string
			for _, behavior := range behaviorFilter {
				behaviorName, err := behaviorpb.ParseBehaviorName(behavior.Name)
				if err != nil {
					returnedErr = status.Error(codes.InvalidArgument, err.Error())
					return
				}
				behaviorIDs = append(behaviorIDs, behaviorName.BehaviorID)
			}
			conditions = append(conditions, columnIn(BehaviorIDColumn, behaviorIDs))
			hasBehavior = true
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		if len(userConditions) > 0 {
			var clauses []clause.Expression
			for _, condition := range userConditions {
				clauses = append(clauses, clause.And(db.Statement.BuildCondition(condition[0], condition[1:]...)...))
			}
			db = db.Clauses(clause.Where{Exprs: []clause.Expression{clause.And(clause.Or(clauses...))}})
		}
		for _, condition := range conditions {
			db = db.Where(condition[0], condition[1:]...)
		}
		return db
	}, hasIntent, hasConversationReason, hasConversationDuration, hasAgent, hasMoment, hasPolicy, hasHintType, hasBehavior, nil
}

func HintTypeToActionType(hintType analyticspb.Attribute_HintType) (int, error) {
	switch hintType {
	case analyticspb.Attribute_REGULAR_HINT, analyticspb.Attribute_BEHAVIOR_HINT:
		return 1, nil
	case analyticspb.Attribute_GW_HINT:
		return 13, nil
	case analyticspb.Attribute_KB_HINT:
		return 14, nil
	case analyticspb.Attribute_CHECKLIST_HINT:
		return 21, nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "unsupported hint_type %v", hintType)
	}
}

// Note: This filter should be used via a gORM scope: https://gorm.io/docs/scopes.html#Query.
func ParseFilterByChatBotAttributeToCondition(filterByAttribute *analyticspb.ChatBotAttribute) (func(db *gorm.DB) *gorm.DB, error) {
	var conditions [][]any
	if filterByAttribute != nil {
		if userFilter := filterByAttribute.Users; userFilter != nil {
			var userIDs []string
			for _, user := range userFilter {
				userName, err := userpb.ParseUserName(user.Name)
				if err != nil {
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
				userIDs = append(userIDs, userName.UserID)
			}
			conditions = append(conditions, columnIn(ChatBotUserIDColumn, userIDs))
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		for _, condition := range conditions {
			db = db.Where(condition[0], condition[1:]...)
		}
		return db
	}, nil
}

// Note: This filter should be used via a gORM scope: https://gorm.io/docs/scopes.html#Query.
// nolint:nakedret
func parseUserFiltersToConditions(
	userFilter []*userpb.User,
) (hasAgent bool, userConditions condition.ConditionsCollection, returnedErr error) {
	if userFilter != nil {
		var userIDs []string
		for _, user := range userFilter {
			userName, err := userpb.ParseUserName(user.Name)
			if err != nil {
				returnedErr = status.Error(codes.InvalidArgument, err.Error())
				return
			}
			userIDs = append(userIDs, userName.UserID)
		}
		userConditions = append(userConditions, columnIn(AgentUserIDColumn, userIDs))
		hasAgent = true
	}
	return hasAgent, userConditions, nil
}

func appendConversationBucketConditions(conditions condition.ConditionsCollection, conversationDurationBuckets []*analyticspb.ConversationDurationBucket) (condition.ConditionsCollection, bool) {
	if conversationDurationBuckets == nil {
		return conditions, false
	}
	conversationDurationBucketsIDs := make([]string, len(conversationDurationBuckets))
	for i, conversationDurationBucket := range conversationDurationBuckets {
		conversationDurationBucketsIDs[i] = conversationDurationBucket.Id
	}
	return append(conditions, columnIn(ConversationDurationBinsIDColumn, conversationDurationBucketsIDs)), true
}

func ParseGroupByAttributeTypes(
	attributeTypes []analyticspb.AttributeType,
) (groupByKeys [][]string, hasIntent, hasConversationDurationBucket, hasAgent, hasMoment, hasPolicy, hasBehavior bool, err error) {
	if len(attributeTypes) > 2 {
		return [][]string{}, hasIntent, hasConversationDurationBucket, hasAgent, hasMoment, hasPolicy, hasBehavior,
			status.Errorf(codes.InvalidArgument, "No more than 2 `group_by_attribute_types` are supported.")
	}
	attributeTypeToKeys := attributeTypeToGroupByKeys()
	hasIntent = shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_INTENT)
	hasConversationDurationBucket = shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_CONVERSATION_DURATION_BUCKET)
	hasAgent = shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
	hasMoment = shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_MOMENT_POLICY)
	hasBehavior = shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR)
	hasPolicy = shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY)
	for _, att := range attributeTypes {
		groupByKeys = append(groupByKeys, attributeTypeToKeys[att])
	}
	if len(attributeTypes) == 0 {
		groupByKeys = [][]string{attributeTypeToKeys[analyticspb.AttributeType_ATTRIBUTE_TYPE_UNSPECIFIED]}
	}
	return groupByKeys, hasIntent, hasConversationDurationBucket, hasAgent, hasMoment, hasPolicy, hasBehavior, nil
}

func containsChatBotAttributeType(attributeTypes []analyticspb.ChatBotAttributeType, attributeType analyticspb.ChatBotAttributeType) bool {
	for _, t := range attributeTypes {
		if attributeType == t {
			return true
		}
	}
	return false
}

func ParseGroupByChatBotAttributeTypes(
	attributeTypes []analyticspb.ChatBotAttributeType,
) (groupByKeys [][]string, hasIntent, hasVersion bool, err error) {
	if len(attributeTypes) > 2 {
		return [][]string{}, hasIntent, hasVersion, status.Errorf(codes.InvalidArgument, "No more than 2 `group_by_chat_bot_attribute_types` are supported.")
	}
	attributeTypeToKeys := chatBotAttributeTypeToGroupByKeys()
	hasIntent = containsChatBotAttributeType(attributeTypes, analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_INTENT)
	hasVersion = containsChatBotAttributeType(attributeTypes, analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_VERSION)
	for _, att := range attributeTypes {
		groupByKeys = append(groupByKeys, attributeTypeToKeys[att])
	}
	if len(attributeTypes) == 0 {
		groupByKeys = [][]string{attributeTypeToKeys[analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_UNSPECIFIED]}
	}
	return groupByKeys, hasIntent, hasVersion, nil
}

type SetAttributeAsGroupByKeyOptions struct {
	DB                    *gorm.DB
	AttributeTypes        []analyticspb.AttributeType
	CustomerID, ProfileID string
	HintType              analyticspb.Attribute_HintType
	GroupByKeyValues      [][]string
}

func SetAttributeAsGroupByKey(opts SetAttributeAsGroupByKeyOptions, usernameToUserMap map[string]*userpb.User) *analyticspb.Attribute {
	attributeTypes, groupByKeyValues, customerID, profileID, hintType, db := opts.AttributeTypes, opts.GroupByKeyValues, opts.CustomerID, opts.ProfileID, opts.HintType, opts.DB

	// TODO: revisit this: when group-by not provided, we may return nil attribute rather than '*' placeholder.
	if len(attributeTypes) == 0 {
		return &analyticspb.Attribute{
			Users:               []*userpb.User{{Name: groupByKeyValues[0][0]}},
			Intents:             []*intentpb.Intent{{Name: groupByKeyValues[0][0], Description: groupByKeyValues[0][1]}},
			ConversationReasons: []*analyticspb.ConversationReason{{Id: groupByKeyValues[0][0], Description: groupByKeyValues[0][1]}},
			SpeakerRoles:        []string{},
		}
	}
	attribute := analyticspb.Attribute{}
	for i := 0; i < len(attributeTypes) && i < len(groupByKeyValues); i++ {
		switch attributeTypes[i] {
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT:
			user := &userpb.User{
				Name: userpb.UserName{CustomerID: customerID, UserID: groupByKeyValues[i][0]}.String(),
				// Should coming from listUsers call.
				// Username:           groupByKeyValues[i][2],
				// FullName:           groupByKeyValues[i][1],
				// ProfilePicturePath: groupByKeyValues[i][5],
			}
			if u, exists := usernameToUserMap[user.Name]; exists {
				user.Username = u.Username
				user.FullName = u.FullName
			}
			attribute.Users = append(attribute.Users, user)
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_INTENT:
			intVal, _ := strconv.ParseInt(groupByKeyValues[i][2], 0, 32)
			intent := &intentpb.Intent{
				Name: intentpb.IntentName{
					CustomerID: customerID,
					ProfileID:  profileID,
					IntentID:   groupByKeyValues[i][0],
				}.String(),
				Description: groupByKeyValues[i][1],
				Order:       int32(intVal),
			}
			attribute.Intents = append(attribute.Intents, intent)
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_CONVERSATION_REASON:
			conversationReason := &analyticspb.ConversationReason{Id: groupByKeyValues[i][0], Description: groupByKeyValues[i][1]}
			attribute.ConversationReasons = append(attribute.ConversationReasons, conversationReason)
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_CONVERSATION_DURATION_BUCKET:
			conversationDurationBucket := &analyticspb.ConversationDurationBucket{Id: groupByKeyValues[i][0]}
			attribute.ConversationDurationBuckets = append(attribute.ConversationDurationBuckets, conversationDurationBucket)
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY:
			sourceType, _ := strconv.ParseInt(groupByKeyValues[i][2], 0, 32)
			var labels map[string]string
			if err := json.Unmarshal([]byte(groupByKeyValues[i][3]), &labels); err != nil {
				fmt.Println(err)
			}
			policyID := groupByKeyValues[i][0]
			policy := policypb.Policy{
				Name: policypb.PolicyName{
					CustomerID: customerID,
					ProfileID:  profileID,
					PolicyID:   policyID,
				}.String(),
				DisplayName: groupByKeyValues[i][1],
				SourceType:  policypb.Policy_SourceType(sourceType),
				Labels: &labelspb.Labels{
					Labels: labels,
				},
			}
			attribute.AttributePolicies = append(attribute.AttributePolicies, &analyticspb.Attribute_AttributePolicy{
				Name:        policy.Name,
				DisplayName: policy.DisplayName,
			})
			actionRows := SearchActionsViaPolicies(customerID, profileID, policyID, "", hintType, db)
			for _, a := range actionRows {
				attribute.Actions = append(attribute.Actions, ConvertActionRowToActionProto(customerID, profileID, a))
			}
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_MOMENT_POLICY:
			order, _ := strconv.ParseInt(groupByKeyValues[i][2], 0, 32)
			momentTemplateID := groupByKeyValues[i][3]
			moment := momentpb.Moment{
				Name: momentpb.MomentName{
					CustomerID: customerID,
					ProfileID:  profileID,
					MomentID:   momentTemplateID,
				}.String(),
				Taxonomy:     groupByKeyValues[i][0],
				DisplayName:  groupByKeyValues[i][1],
				DisplayOrder: int32(order),
			}
			attribute.Moments = append(attribute.Moments, &moment)
			// TODO: we can improve on latency by avoiding DB read for every moments.
			// First set action search via policies, if not found, then search for action by matching taxonomy.
			actionRows := SearchActionsViaPolicies(customerID, profileID, "", momentTemplateID, hintType, db)
			if len(actionRows) > 0 {
				for _, a := range actionRows {
					attribute.Actions = append(attribute.Actions, ConvertActionRowToActionProto(customerID, profileID, a))
				}
			} else {
				var actionRow ActionRow
				if db != nil {
					db.Select(actionSelectClause).
						Model(model.Actions{}).
						Where("taxonomy = @taxonomy AND customer_id = @customer_id AND profile_id = @profile_id"+
							" and type = 1",
							sql.Named("taxonomy", groupByKeyValues[i][0]),
							sql.Named("customer_id", customerID),
							sql.Named("profile_id", profileID)).
						// This is for "Hint Page" to determinately show SDX > DDX / DNX.
						Order("detailed_type ASC").
						Take(&actionRow)
				}
				if actionRow.ActionID != "" {
					attribute.Actions = append(attribute.Actions, ConvertActionRowToActionProto(customerID, profileID, actionRow))
				}
			}
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR:
			behaviorID := groupByKeyValues[i][0]
			behaviorDisplayName := groupByKeyValues[i][1]
			behavior := &behaviorpb.Behavior{
				Name: behaviorpb.BehaviorName{
					CustomerID: customerID,
					ProfileID:  profileID,
					BehaviorID: behaviorID,
				}.String(),
				DisplayName: behaviorDisplayName,
			}
			attribute.Behaviors = append(attribute.Behaviors, behavior)
		}
	}
	return &attribute
}

func ConvertActionRowToActionProto(customerID, profileID string, actionRow ActionRow) *actionpb.Action {
	return &actionpb.Action{
		Name: actionpb.ActionName{
			CustomerID: customerID,
			ProfileID:  profileID,
			ActionID:   actionRow.ActionID,
		}.String(),
		DisplayName: actionRow.DisplayName,
		ActionData: &actionpb.ActionData{
			Action: &actionpb.ActionData_HintAction{
				HintAction: &actionpb.HintAction{
					Title:       actionRow.Title,
					Description: actionRow.Description,
					Suggestion:  actionRow.Suggestion,
				},
			},
		},
		Type:         actionpb.Action_Type(actionRow.Type),
		DetailedType: actionpb.Action_DetailedType(actionRow.DetailedType),
		Taxonomy:     actionRow.Taxonomy,
	}
}

func GetUserIDsInDBFromUserFilter(db *gorm.DB, includeDevUsers, excludeDeactivatedUsers bool, userFilter []*userpb.User) ([]string, error) {
	attribute := &analyticspb.Attribute{
		Users:                   userFilter,
		IncludeDevUsers:         includeDevUsers,
		ExcludeDeactivatedUsers: excludeDeactivatedUsers,
	}

	return GetUserIDsInDB(db, attribute)
}

func GetUserIDsInDB(db *gorm.DB, attribute *analyticspb.Attribute) ([]string, error) {
	if attribute == nil {
		attribute = &analyticspb.Attribute{}
	}
	includeDevUsers, excludeDeactivatedUsers := attribute.IncludeDevUsers, attribute.ExcludeDeactivatedUsers
	userIDs, err := fn.MapE(
		attribute.Users,
		func(user *userpb.User) (string, error) {
			userName, err := userpb.ParseUserName(user.Name)
			if err != nil {
				return "", status.Error(codes.InvalidArgument, err.Error())
			}
			return userName.UserID, nil
		})
	if err != nil {
		return nil, err
	}

	return getUserIDsInDBUsers(userIDs, includeDevUsers, excludeDeactivatedUsers, db)
}

func getUserIDsInDBUsers(userIDs []string, includeDevUsers, excludeDeactivatedUsers bool, db *gorm.DB) ([]string, error) {
	// Active users are included by default, this include_deactiavated_users determines
	// whether to include deactivated users.
	userStatesToInclude := []int32{int32(userpb.User_ACTIVE)}
	if !excludeDeactivatedUsers {
		userStatesToInclude = append(userStatesToInclude, int32(userpb.User_INACTIVE))
	}

	conditions := condition.New(`active_state IN ?`, userStatesToInclude)
	if len(userIDs) > 0 {
		conditions = condition.AppendWhereIn(conditions, "user_id", userIDs)
	}
	if !includeDevUsers {
		conditions = condition.AppendSQLAndClause(conditions, "NOT is_dev_user")
	}

	var userIDsInDB []string
	if err := db.
		Model(&model.Users{}).
		Where(conditions.QueryString(), conditions.QueryParams()...).
		Select("DISTINCT(COALESCE(user_id, '')) as user_id").
		Order("user_id ASC").
		Scan(&userIDsInDB).Error; err != nil {
		return nil, fmt.Errorf("failed fetching UserIDsInDBUsers: %w", err)
	}
	return userIDsInDB, nil
}

func ReadDBConversations(
	db *gorm.DB,
	c *converter.Converter,
	customerProfile *shared.CustomerProfile, conversationIDs []string,
) ([]*convpb.Conversation, error) {
	convs := []*convpb.Conversation{}
	if len(conversationIDs) == 0 {
		return convs, nil
	}

	var result []*converter.ChatWithUsers
	conditions := condition.ConditionsCollection{
		columnEq("app.chats.customer_id", customerProfile.CustomerID),
		columnEq("app.chats.profile_id", customerProfile.ProfileID),
		columnIn("conversation_id", conversationIDs),
	}
	cc := conditions.And()

	_, err := dao.ListChats(
		db.Preload("User"),
		&result,
		query.WithPageSize(len(conversationIDs)),
		query.WithCondition(cc...),
		query.OrderByDesc("ended_at"),
	)

	for _, chat := range result {
		var dbVisitor *model.ChatVisitors
		if chat.VisitorID.Valid {
			var chatVisitors []*model.ChatVisitors
			if result := db.Find(&chatVisitors, &model.ChatVisitors{
				ID: chat.VisitorID.Int32,
			}); result.Error != nil {
				return nil, result.Error
			}
			if len(chatVisitors) == 1 {
				dbVisitor = chatVisitors[0]
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "Expected to get 1 visitor for id %d, but got %d", chat.VisitorID.Int32, len(chatVisitors))
			}
		}
		apiConversation, err := c.ConvertFromDBConversationWithUsers(chat, dbVisitor)
		if err != nil {
			return nil, err
		}
		convs = append(convs, apiConversation)
	}
	return convs, err
}

type PolicyRow struct {
	PolicyID         string
	TriggerMoment    string
	OutputActions    string // in the format of `["customers/cox/profiles/retention/actions/action_id1", "customers/cox/profiles/retention/actions/action_id2"]`
	AdherenceMoments string // in the format of `{"moment_id1", "moment_id2"}`
}

const actionSelectClause = "payload->'hint'->'title'#>> '{}' AS title," +
	" display_name," +
	" payload->'hint'->'description'#>> '{}' AS description," +
	" payload->'hint'->'suggestion' #>> '{}' AS suggestion," +
	" action_id," +
	" detailed_type," +
	" type," +
	" taxonomy"

func SearchActionsViaPolicies(customerID, profileID, policyID, momentTemplateID string, hintType analyticspb.Attribute_HintType, db *gorm.DB) []ActionRow {
	policySelectionClause := "policy_id AS policy_id," +
		"input->'momentTemplate'#>>'{}' AS trigger_moment," +
		"output->'actions'->'actionNames'#>>'{}' AS output_actions," +
		"adherence_moments AS adherence_moments"
	var policyRows []PolicyRow
	if db != nil {
		conditions := condition.New(
			"customer_id = @customer_id AND profile_id = @profile_id",
			sql.Named("customer_id", customerID),
			sql.Named("profile_id", profileID),
		)
		if policyID != "" {
			conditions = condition.AppendSQLAndClause(conditions, "policy_id = @policy_id", sql.Named("policy_id", policyID))
		}
		db.Select(policySelectionClause).
			Model(model.Policies{}).
			Where(conditions.QueryString(), conditions.QueryParams()...).
			Scan(&policyRows)
	}
	if len(policyRows) == 0 {
		return nil
	}
	outputActions := []string{}
	if policyID != "" {
		outputActions = append(outputActions, policyRows[0].OutputActions)
	} else {
		for _, p := range policyRows {
			// First check trigger moment.
			if strings.Contains(p.TriggerMoment, momentTemplateID) || strings.Contains(p.AdherenceMoments, momentTemplateID) {
				outputActions = append(outputActions, p.OutputActions)
			}
		}
	}
	actionIDs := []string{}
	for _, names := range outputActions {
		for _, actionName := range strings.Split(names, ",") {
			strs := strings.Split(strings.Trim(actionName, "[]\""), "/")
			actionIDs = append(actionIDs, strs[len(strs)-1])
		}
	}

	if len(actionIDs) == 0 {
		return nil
	}
	actionRows := []ActionRow{}
	conditions := condition.New(
		"customer_id = @customer_id AND profile_id = @profile_id AND action_id in @action_ids"+
			" AND (detailed_type is NULL OR detailed_type not in @detailed_types)",
		sql.Named("customer_id", customerID),
		sql.Named("profile_id", profileID),
		sql.Named("action_ids", actionIDs),
		sql.Named("detailed_types", []int32{int32(actionpb.Action_DETAILED_TYPE_DID_DO_X_HINT), int32(actionpb.Action_DETAILED_TYPE_DID_NOT_DO_X_HINT)}), // ignore DDX/DNX hints.
	)
	if hintType != analyticspb.Attribute_HINT_TYPE_UNSPECIFIED {
		actionType, err := HintTypeToActionType(hintType)
		if err != nil {
			return nil
		}
		// For checklist hint: we are looking for action.type=Checklist (20), and action_annotations.type=Checklist_Item (21).
		if hintType == analyticspb.Attribute_CHECKLIST_HINT {
			actionType = 20
		}
		conditions = condition.AppendSQLAndClause(conditions, "type=@action_type", sql.Named("action_type", actionType))
	}
	if db != nil {
		db.Select(actionSelectClause).
			Model(model.Actions{}).
			Where(conditions.QueryString(), conditions.QueryParams()...).
			Order("detailed_type ASC").
			Scan(&actionRows)
	}
	return actionRows
}

func ListPolicies(db *gorm.DB, policyIDs []string) []model.Policies {
	policies := []model.Policies{}
	if len(policyIDs) > 0 {
		db.Where("policy_id IN ?", policyIDs).Find(&policies)
	}
	return policies
}

func ListMoments(db *gorm.DB, momentIDs []string) []model.MomentTemplates {
	moments := []model.MomentTemplates{}
	if len(momentIDs) > 0 {
		db.Where("moment_template_id IN ?", momentIDs).Find(&moments)
	}
	return moments
}
