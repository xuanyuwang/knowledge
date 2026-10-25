package elasticsearch

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	actionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/action"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	collaborationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/collaboration"
	enumpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	dbactionpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/action"
	dbconvopb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/conversation"
	dbeventpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/event"
	dbmomentpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/go-servers/shared/conversation"
	"github.com/cresta/go-servers/shared/conversation/elastic"
	"github.com/cresta/go-servers/shared/converters/momentconverter"
)

const (
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations-metrics-cardinality-aggregation.html#_precision_control
	// The precision_threshold options allows to trade memory for accuracy, and defines a unique count below which counts are expected to be close to accurate.
	// Above this value, counts might become a bit more fuzzy.
	// The maximum supported value is 40000, thresholds above this number will have the same effect as a threshold of 40000.
	MaxCardinalityPrecisionThreshold = 40000
	minShouldMatch                   = 1
)

var (
	defaultConversationSources                          = envflag.String("DEFAULT_CONVERSATION_SOURCES", "0,7,8", "conversation sources returned by default")
	esQueryAttendeesForAgentUserIDs                     = envflag.Bool("ENABLE_ES_QUERY_ATTENDEES_FOR_AGENT_USER_IDS", false, "Whether to query attendees for agent user ids.")
	enableExcludingConsultationConvo                    = envflag.Bool("ENABLE_EXCLUDING_CONSULTATION_CONVERSATION", false, "Whether to exclude consultation conversation (agent to agent calls)")
	enableUsageOfExplicitMappingMomentAnnotationPayload = envflag.Bool("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", false, "Whether to use the explicit mapping for metadata annotation payload.")
)

const (
	// The number of extra terms when matching a flexible phrase.
	maxSlopForFlexiblePhrase = 5
	// The ES fields within the multi_match query.
	phraseType                 = "phrase"
	MomentAnnotationsPath      = "moment_annotations"
	actionAnnotationsPath      = "action_annotations"
	conversationEventsPath     = "conversation_events"
	ScorecardsPath             = "scorecards"
	ScorecardScoresPath        = "scorecards.scorecard_scores"
	attendeesPath              = "attendees"
	commentsPath               = "comments"
	scorecardsScoresPath       = "scorecards.scorecard_scores"
	scriptConvoIDAndConvoMsgID = "return [doc['conversation_id.keyword'] + '-' + doc['conversation_message_id.keyword']]"
)

// The extra configuration for Elasticsearch query, some come from API payload, some comes from
// environment variables, and some comes from customer flags. This replace analyticspb.ElasticsearchQueryConfig.
type QueryConfig struct {
	SkipConversationMessageCount bool
	// Include conversations that has no messages.
	IncludeEmptyConversations bool
	// Read from new typed 'percentage_score2' field.
	ReadFromPercentageScore2Field bool
	// ES Documenttype
	EsDocumentType      DocumentType
	EnableNumHoldsField bool
}

type MatchPharesType int32

const (
	MatchTypeUnknown MatchPharesType = iota
	MatchAny
	MatchAll
)

// MatchNone is an empty struct that serializes to {} in JSON, representing an ES match_none query
type MatchNone struct{}

type DocumentType string

const (
	DocumentTypeUndefined      DocumentType = "undefined"
	DocumentTypeMessage        DocumentType = "message"
	DocumentTypeMessageGroup   DocumentType = "message_group"
	DocumentTypeConversation   DocumentType = "conversation"
	DocumentTypeScorecard      DocumentType = "scorecard"
	DocumentTypeScorecardScore DocumentType = "scorecard_score"
)

const (
	languageCodeField = "effective_language.keyword"
	DefaultPageSize   = 50
)

const (
	ConversationMetadataPayloadStringValueField                = "moment_annotations.payload.conversationMetadataPayload.values.stringValue.keyword"
	ConversationMetadataPayloadNumberValueField                = "moment_annotations.payload.conversationMetadataPayload.values.numberValue"
	ConversationMetadataPayloadNumberValueFieldExplicitMapping = "moment_annotations.payload_with_explicit_mapping.metadata_number_value"
	ConversationMetadataPayloadBooleanValueField               = "moment_annotations.payload.conversationMetadataPayload.values.booleanValue"
)

const (
	ConversationOutcomePayloadStringValueField                = "moment_annotations.payload.conversationOutcome.stringValue.keyword"
	ConversationOutcomePayloadNumberValueField                = "moment_annotations.payload.conversationOutcome.numberValue"
	ConversationOutcomePayloadNumberValueFieldExplicitMapping = "moment_annotations.payload_with_explicit_mapping.outcome_number_value"
	ConversationOutcomePayloadBoolValueField                  = "moment_annotations.payload.conversationOutcome.booleanValue"
)

const (
	LlmAnnotationPayloadStringValueField                = "moment_annotations.payload.llmAnnotationPayload.stringValue.keyword"
	LlmAnnotationPayloadNumberValueField                = "moment_annotations.payload.llmAnnotationPayload.numberValue"
	LlmAnnotationPayloadNumberValueFieldExplicitMapping = "moment_annotations.payload_with_explicit_mapping.llm_annotation_number_value"
	LlmAnnotationPayloadBooleanValueField               = "moment_annotations.payload.llmAnnotationPayload.booleanValue"
)

const (
	ScorecardPercentageValueField        = "scorecards.percentage_score"
	ScorecardScoresPercentageValueField  = "scorecards.scorecard_scores.percentage_score"
	ScorecardPercentageValue2Field       = "scorecards.percentage_score2"
	ScorecardScoresPercentageValue2Field = "scorecards.scorecard_scores.percentage_score2"
)

// The config that specifies the boolean logic of moments in a TopicDetails to match ES documents.
type ChildMomentsBooleanLogic int32

const (
	// The default preview logic: excluded_moment AND (included_moment OR (OR clusters))
	DefaultChildMomentsBooleanConfig ChildMomentsBooleanLogic = iota
	// The preview logic: excluded_moment AND included_moment AND (AND clusters)
	AllMomentAtTopLevelWithAND
)

func nestedHasParentConversation(nested *Nested) *HasParent {
	return &HasParent{
		ParentType: "conversation",
		Query: ConvoQuery{
			Bool: ConvoBool{
				Filter: &[]ConvoFilter{
					{Nested: nested},
				},
			},
		},
	}
}

func nestedHasChildMessage(nested *Nested) *HasChild {
	return &HasChild{
		Type: string(DocumentTypeMessage),
		Query: InnerMsgQuery{
			Bool: &InnerMsgBool{
				Filter: &[]MsgFilter{
					{Nested: nested},
				},
			},
		},
	}
}

var (
	DefaultHightlight = ReqHighlight{
		PreTags:  []string{"<mark>"},
		PostTags: []string{"</mark>"},
		Fields: HighlightFields{
			MessageText: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWhitespace: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerEnglish: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerSpanish: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerFrench: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerItalian: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerGerman: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerDutch: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerPortuguese: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerRussian: &HighlightConfig{
				NumberOfFragments: 0,
			},
			MessageTextWithCustomAnalyzerThai: &HighlightConfig{
				NumberOfFragments: 0,
			},
		},
	}
	DefaultSort = &[]Sort{
		{
			ConversationEndedAt: &SortKey{
				Format: "strict_date_optional_time_nanos",
				Order:  "desc",
			},
		},
	}
	SecureSortForConversation = &[]Sort{
		(*DefaultSort)[0],
		{
			ConversationID: &SortKey{
				Order: "asc",
			},
		},
	}
	DefaultSortForMessageDoc = &[]Sort{
		(*DefaultSort)[0],
		{
			CreatedAt: &SortKey{
				Format: "strict_date_optional_time_nanos",
				Order:  "asc",
			},
		},
	}
	SortByScore = &[]Sort{
		{
			Score: &SortKey{
				Order: "desc",
			},
		},
		(*DefaultSort)[0],
	}
	DefaultIgnoredFields = []string{
		"moment_annotations",
		"action_annotations",
		"conversation_events",
		"scorecards",
	}
)

var (
	// The ES fields within the multi_match query.
	defaultMsgTextField = "message_text"

	// message text field that uses whitespace analyzer for matching.
	// See details in https://www.elastic.co/guide/en/elasticsearch/reference/current/analysis-whitespace-analyzer.html
	whitespaceMsgTextField = "message_text_whitespace"

	langCodeToMsgTextField = map[string]string{
		// German
		"de-DE": "message_text_with_custom_analyzer.german",
		// English
		"en-AU": "message_text_with_custom_analyzer.english",
		"en-GB": "message_text_with_custom_analyzer.english",
		"en-NZ": "message_text_with_custom_analyzer.english",
		"en-US": "message_text_with_custom_analyzer.english",
		// Spanish
		"es-419": "message_text_with_custom_analyzer.spanish",
		"es-ES":  "message_text_with_custom_analyzer.spanish",
		"es-MX":  "message_text_with_custom_analyzer.spanish",
		// French
		"fr-CA": "message_text_with_custom_analyzer.french",
		"fr-FR": "message_text_with_custom_analyzer.french",
		// Italian
		"it-IT": "message_text_with_custom_analyzer.italian",
		// Dutch
		"nl-NL": "message_text_with_custom_analyzer.dutch",
		// Portuguese
		"pt-BR": "message_text_with_custom_analyzer.portuguese",
		"pt-PT": "message_text_with_custom_analyzer.portuguese",
		// Russian
		"ru-RU": "message_text_with_custom_analyzer.russian",
		// Thai
		"th-TH": "message_text_with_custom_analyzer.thai",
		// By default for all the other languages
		// We still need to explicitly list all supported language codes here.
		"sv-SE": defaultMsgTextField,
		"ja-JP": defaultMsgTextField,
		"ko-KR": defaultMsgTextField,
		"zh-CN": defaultMsgTextField,
		"zh-HK": defaultMsgTextField,
		"zh-TW": defaultMsgTextField,
		"pl-PL": defaultMsgTextField,
		"ms-MY": defaultMsgTextField,
		// TODO(zhucaiyi): change to corresponding message_text_with_custom_analyzer.
		"hu-HU": defaultMsgTextField,
		"cs-CZ": defaultMsgTextField,
		"da-DK": defaultMsgTextField,
		"ar-AE": defaultMsgTextField,
	}
)

// ElasticSearch Request
type TimeRange struct {
	GTE      string `json:"gte"`
	LT       string `json:"lt"`
	TimeZone string `json:"time_zone"`
}

type IntRange struct {
	GTE *int32 `json:"gte,omitempty"`
	LTE *int32 `json:"lte,omitempty"`
}

type FloatRange struct {
	GT  *float64 `json:"gt,omitempty"`
	GTE *float64 `json:"gte,omitempty"`
	LT  *float64 `json:"lt,omitempty"`
	LTE *float64 `json:"lte,omitempty"`
}

type RangeFilter struct {
	ConversationStartedAt                   *TimeRange  `json:"conversation_started_at,omitempty"`
	ConversationEndedAt                     *TimeRange  `json:"conversation_ended_at,omitempty"`
	ConversationDurationSec                 *FloatRange `json:"conversation_duration_secs,omitempty"`
	ConversationHandleTimeSec               *FloatRange `json:"handle_time_secs,omitempty"`
	MetadataNumberValue                     *FloatRange `json:"moment_annotations.payload.conversationMetadataPayload.values.numberValue,omitempty"`
	MetadataNumberValueExplicitMapping      *FloatRange `json:"moment_annotations.payload_with_explicit_mapping.metadata_number_value,omitempty"`
	OutcomeNumberValue                      *FloatRange `json:"moment_annotations.payload.conversationOutcome.numberValue,omitempty"`
	OutcomeNumberValueExplicitMapping       *FloatRange `json:"moment_annotations.payload_with_explicit_mapping.outcome_number_value,omitempty"`
	LlmAnnotationNumberValue                *FloatRange `json:"moment_annotations.payload.llmAnnotationPayload.numberValue,omitempty"`
	LlmAnnotationNumberValueExplicitMapping *FloatRange `json:"moment_annotations.payload_with_explicit_mapping.llm_annotation_number_value,omitempty"`
	FowardPositionRange                     *IntRange   `json:"message_forward_position,omitempty"`
	BackwardPostionRange                    *IntRange   `json:"message_backward_position,omitempty"`
	ForwardPositionInSecs                   *FloatRange `json:"message_forward_position_in_secs,omitempty"`
	ConversationMessageCount                *FloatRange `json:"message_count,omitempty"`
	TruncatedDayOfWeek                      *FloatRange `json:"truncated_day_of_week,omitempty"`
	TruncatedMinuteOfDay                    *FloatRange `json:"truncated_minute_of_day,omitempty"`
	NumHolds                                *FloatRange `json:"num_holds,omitempty"`
}

// Elasticsearch Term query support case sensitivity, while Terms query does not. Default to case sensitive.
// Use this struct only if case insensitive is required.
type TermFilter struct {
	MetadataPayloadStringValues *ValueField `json:"moment_annotations.payload.conversationMetadataPayload.values.stringValue.keyword,omitempty"`
}

type Nested struct {
	Path  string        `json:"path,omitempty"`
	Query *NestedFilter `json:"query,omitempty"`
}

type NestedFilter struct {
	Bool           *NestedFilterBool  `json:"bool,omitempty"`
	Terms          *NestedTermsFilter `json:"terms,omitempty"`
	Term           *TermFilter        `json:"term,omitempty"`
	WildcardFilter *WildcardFilter    `json:"wildcard,omitempty"`
	Range          *RangeFilter       `json:"range,omitempty"`
	Exists         *Field             `json:"exists,omitempty"`
	Nested         *Nested            `json:"nested,omitempty"`
}

type NestedFilterBool struct {
	MustNot            *[]NestedFilter `json:"must_not,omitempty"`
	Should             *[]NestedFilter `json:"should,omitempty"`
	Filter             *[]NestedFilter `json:"filter,omitempty"`
	MinimumShouldMatch int32           `json:"minimum_should_match,omitempty"`
}

// TODO: rename this struct to NestedFilterTerms
type NestedTermsFilter struct {
	// For fields of action annotations.
	ActionIDs   *[]string `json:"action_annotations.action_id.keyword,omitempty"`
	ActionTypes *[]int    `json:"action_annotations.type,omitempty"`
	// For fields of moment annotations.
	MomentTemplateIDs                       *[]string  `json:"moment_annotations.moment_template_id.keyword,omitempty"`
	MomenAnnotationAdherenceActionIDs       *[]string  `json:"moment_annotations.adherence_action_id.keyword,omitempty"`
	BehaviorIDs                             *[]string  `json:"moment_annotations.behavior_id.keyword,omitempty"`
	MomentAdherenceTypes                    *[]int     `json:"moment_annotations.adherence_type,omitempty"`
	MomentTypes                             *[]int     `json:"moment_annotations.type,omitempty"`
	MetadataStringPayloadValues             *[]string  `json:"moment_annotations.payload.conversationMetadataPayload.values.stringValue.keyword,omitempty"`
	MetadataNumberPayloadValues             *[]float32 `json:"moment_annotations.payload.conversationMetadataPayload.values.numberValue,omitempty"`
	MetadataNumberValueExplicitMapping      *[]float32 `json:"moment_annotations.payload_with_explicit_mapping.metadata_number_value,omitempty"`
	MetadataBooleanPayloadValues            *[]bool    `json:"moment_annotations.payload.conversationMetadataPayload.values.booleanValue,omitempty"`
	OutcomeStringPayloadValues              *[]string  `json:"moment_annotations.payload.conversationOutcome.stringValue.keyword,omitempty"`
	OutcomeNumberPayloadValues              *[]float32 `json:"moment_annotations.payload.conversationOutcome.numberValue,omitempty"`
	OutcomeNumberValueExplicitMapping       *[]float32 `json:"moment_annotations.payload_with_explicit_mapping.outcome_number_value,omitempty"`
	OutcomeBooleanPayloadValues             *[]bool    `json:"moment_annotations.payload.conversationOutcome.booleanValue,omitempty"`
	LlmAnnotationStringPayloadValues        *[]string  `json:"moment_annotations.payload.llmAnnotationPayload.stringValue.keyword,omitempty"`
	LlmAnnotationNumberPayloadValues        *[]float32 `json:"moment_annotations.payload.llmAnnotationPayload.numberValue,omitempty"`
	LlmAnnotationNumberValueExplicitMapping *[]float32 `json:"moment_annotations.payload_with_explicit_mapping.llm_annotation_number_value,omitempty"`
	LlmAnnotationBooleanPayloadValues       *[]bool    `json:"moment_annotations.payload.llmAnnotationPayload.booleanValue,omitempty"`
	KnowledgeMomentTypes                    *[]string  `json:"moment_annotations.payload.knowledgePayload.knowledgeMomentType.keyword,omitempty"`
	// For fields of conversation events.
	ConversationEventsAdherenceActionIDs *[]string `json:"conversation_events.action_id.keyword,omitempty"`
	ConversationEventTypes               *[]int    `json:"conversation_events.type,omitempty"`
	ConversationEventKBDocIDs            *[]string `json:"conversation_events.payload.usedKbDocument.docId.keyword,omitempty"`
	ConversationEventNewLangCodes        *[]string `json:"conversation_events.payload.conversationLanguageChange.newLanguageCode.keyword,omitempty"`
	ConversationEventOldLangCodes        *[]string `json:"conversation_events.payload.conversationLanguageChange.oldLanguageCode.keyword,omitempty"`
	// For fields of scorecards.
	ScorecardTypes               *[]int     `json:"scorecards.scorecard_type,omitempty"`
	ScorecardStatuses            *[]int     `json:"scorecards.status,omitempty"`
	ScorecardTemplateIDs         *[]string  `json:"scorecards.template_id.keyword,omitempty"`
	SubmitterUserIDs             *[]string  `json:"scorecards.submitter_user_id.keyword,omitempty"`
	ScorecardScoresCriterionIDs  *[]string  `json:"scorecards.scorecard_scores.criterion_id.keyword,omitempty"`
	ScorecardScoresNumericValues *[]float32 `json:"scorecards.scorecard_scores.numeric_value,omitempty"`
	ScorecardScoresNotApplicable *[]bool    `json:"scorecards.scorecard_scores.not_applicable,omitempty"`
	// For fields of attendees.
	ParticipantRoles       *[]string `json:"attendees.role.keyword,omitempty"`
	ParticipantPlatformIDs *[]string `json:"attendees.platform_attendee_id.keyword,omitempty"`
	// For fields of nested `comments` documents indexed on the conversation.
	CommentTypes *[]int `json:"comments.type,omitempty"`
}

type TermsFilter struct {
	SpeakerRoles                                  *[]string `json:"speaker_role,omitempty"`
	AgentUserIDs                                  *[]string `json:"agent_user_id.keyword,omitempty"`
	ConversationTagIDs                            *[]string `json:"tag_ids.keyword,omitempty"`
	TeamGroupIDs                                  *[]string `json:"team_group_id.keyword,omitempty"`
	ConversationIDs                               *[]string `json:"conversation_id.keyword,omitempty"`
	ConversationMessageIDs                        *[]string `json:"conversation_message_id.keyword,omitempty"`
	ConversationCorrelationIDs                    *[]string `json:"conversation_correlation_id.keyword,omitempty"`
	PlatformConversationIDs                       *[]string `json:"platform_conversation_id.keyword,omitempty"`
	UsecaseIDs                                    *[]string `json:"usecase_id.keyword,omitempty"`
	LangCodes                                     *[]string `json:"effective_language.keyword,omitempty"`
	ConversationSources                           *[]int32  `json:"conversation_source,omitempty"`
	ConversationChannels                          *[]int32  `json:"conversation_channel,omitempty"`
	HasComment                                    *[]bool   `json:"has_comment,omitempty"`
	WorkflowAutomationConditions                  *[]int32  `json:"workflow_automation_conditions,omitempty"`
	HasEmailAttachments                           *[]bool   `json:"has_email_attachments,omitempty"`
	RaiseHandStates                               *[]int32  `json:"raise_hand_state,omitempty"`
	DocumentIDs                                   *[]string `json:"_id,omitempty"`
	ConversationToMessageJoin                     *[]string `json:"conversation_to_message_join,omitempty"`
	TransferInfoParticipantInteractionTypes       *[]int32  `json:"transfer_info.participant_interaction_type,omitempty"`
	TransferInfoUmbrellaConversationTransferTypes *[]int32  `json:"transfer_info.umbrella_conversation_transfer_types,omitempty"`
	TranslationInfoTranslated                     *[]bool   `json:"translation_info.translated,omitempty"`
	VirtualAgentRevisionIDs                       *[]string `json:"virtual_agent_revision_id.keyword,omitempty"`
	IngestedByJobIDs                              *[]string `json:"ingested_by_job_id.keyword,omitempty"`
	DisplayNames                                  *[]string `json:"display_name.keyword,omitempty"`
	// Denormalized "<moment_template_id>_<value>" pairs indexed on the conversation
	// document for conversation-level metadata. See converter.buildMetadataIDValuePairsFromMomentAnnotation.
	MetadataIDValuePairs *[]string `json:"metadata_id_value_pairs,omitempty"`
}

type MetadataPayloadString struct {
	MetadataStringPayloadValue string `json:"moment_annotations.payload.conversationMetadataPayload.values.stringValue,omitempty"`
}

type MatchFilter struct {
	DisplayName *MatchFieldQuery `json:"display_name,omitempty"`
}

type MatchFieldQuery struct {
	Query string `json:"query"`
}

type WildcardFilter struct {
	// wildcard query on metadata string payload. Has to be keyword (unanalyzed)
	MetadataPayload *ValueField `json:"moment_annotations.payload.conversationMetadataPayload.values.stringValue.keyword,omitempty"`
	// wildcard query on llm annotation string payload. Has to be keyword (unanalyzed)
	LlmAnnotationPayload *ValueField `json:"moment_annotations.payload.llmAnnotationPayload.stringValue.keyword,omitempty"`
	// wildcard query on platform conversation id
	PlatformConversationID *ValueField `json:"platform_conversation_id.keyword,omitempty"`
	// wildcard query on conversation correlation id
	ConversationCorrelationID *ValueField `json:"conversation_correlation_id.keyword,omitempty"`
}

type ValueField struct {
	Value           string `json:"value"`
	CaseInsensitive *bool  `json:"case_insensitive,omitempty"`
}

type MultiMatchField struct {
	Query  string   `json:"query"`
	Type   string   `json:"type"`
	Fields []string `json:"fields"`
	Slop   *int32   `json:"slop,omitempty"`
}

type InnerMsgBool struct {
	Filter             *[]MsgFilter `json:"filter,omitempty"`
	Must               *[]MsgFilter `json:"must,omitempty"`
	Should             *[]MsgFilter `json:"should,omitempty"`
	MinimumShouldMatch int32        `json:"minimum_should_match,omitempty"`
}

type InnerMsgQuery struct {
	Bool *InnerMsgBool `json:"bool,omitempty"`
}

type EmptyStruct struct{}

// MsgFilter is a single query clause applied to message documents. It is the
// union of the (formerly separate) "inner" and "outer" message filters: every
// field either variant supported is available here, so the same struct can be
// used wherever a message-level clause is needed.
type MsgFilter struct {
	Terms  *TermsFilter `json:"terms,omitempty"`
	Nested *Nested      `json:"nested,omitempty"`
	// Bool holds either *TopicBool or *TopicsBool (both serialize to the ES
	// "bool" key). Use an interface here so a single struct can carry both,
	// mirroring OuterQuery.Bool and ReqAnnotationSubFilter.Bool.
	Bool            any              `json:"bool,omitempty"`
	MultiMatchField *MultiMatchField `json:"multi_match,omitempty"`
	MatchNone       *MatchNone       `json:"match_none,omitempty"`
	HasParent       *HasParent       `json:"has_parent,omitempty"`
	Range           *RangeFilter     `json:"range,omitempty"`
}

// TODO(zhucaiyi): merge ConvoFilter, ConvoFilterBool, ConvoFilterBoolFilter
// ConvoFilterBoolFilterBool, ConvoFilterBoolFilterBoolFilter, NotHasChildBool.
type ConvoFilter struct {
	Range         *RangeFilter     `json:"range,omitempty"`
	Terms         *TermsFilter     `json:"terms,omitempty"`
	Match         *MatchFilter     `json:"match,omitempty"`
	Nested        *Nested          `json:"nested,omitempty"`
	HasChild      *HasChild        `json:"has_child,omitempty"`
	Bool          *ConvoFilterBool `json:"bool,omitempty"`
	FunctionScore *json.RawMessage `json:"function_score,omitempty"`
}

type ConvoFilterBool struct {
	Should             *[]ConvoFilterBoolFilter `json:"should,omitempty"`
	MinimumShouldMatch int32                    `json:"minimum_should_match,omitempty"`
}

type ConvoFilterBoolFilter struct {
	HasChild      *HasChild                  `json:"has_child,omitempty"`
	Nested        *Nested                    `json:"nested,omitempty"`
	Bool          *ConvoFilterBoolFilterBool `json:"bool,omitempty"`
	Terms         *TermsFilter               `json:"terms,omitempty"`
	Match         *MatchFilter               `json:"match,omitempty"`
	Range         *RangeFilter               `json:"range,omitempty"`
	FunctionScore *json.RawMessage           `json:"function_score,omitempty"`
	Wildcard      *WildcardFilter            `json:"wildcard,omitempty"`
	MatchNone     *MatchNone                 `json:"match_none,omitempty"`
}

type ConvoFilterBoolFilterBool struct {
	MustNot *ConvoFilterBoolFilterBoolFilter `json:"must_not,omitempty"`
	// Handle the case where there complicated boolean logic over multiple has_child filter
	Should             *[]ConvoFilterBoolFilterBoolFilter `json:"should,omitempty"`
	MinimumShouldMatch *int32                             `json:"minimum_should_match,omitempty"`
}

type ConvoFilterBoolFilterBoolFilter struct {
	HasChild *HasChild        `json:"has_child,omitempty"`
	Nested   *Nested          `json:"nested,omitempty"`
	Bool     *NotHasChildBool `json:"bool,omitempty"`
	Terms    *TermsFilter     `json:"terms,omitempty"`
	Exists   *Field           `json:"exists,omitempty"`
}

type NotHasChildBool struct {
	MustNot *HasChildFilter `json:"must_not,omitempty"`
}

// A wrapper of HasChild struct so that it can be an item in `should` `must` and `filter` in bool query.
type HasChildFilter struct {
	HasChild *HasChild `json:"has_child,omitempty"`
}

type Field struct {
	Field string `json:"field"`
}

type HasChild struct {
	Type  string        `json:"type,omitempty"`
	Query InnerMsgQuery `json:"query"`
}

type ConvoBool struct {
	Filter  *[]ConvoFilter `json:"filter,omitempty"`
	MustNot *[]ConvoFilter `json:"must_not,omitempty"`
}

type ConvoQuery struct {
	Bool ConvoBool `json:"bool"`
}

type HasParent struct {
	ParentType string     `json:"parent_type,omitempty"`
	Query      ConvoQuery `json:"query"`
}

// The struct is to serve `Moment of Interests` page of Closed Conversations UI, where all insights moments
// are queried in one API call.
type TopicsBool struct {
	Should *[]TopicsShould `json:"should,omitempty"`
}

type TopicsShould struct {
	Terms     *TermsFilter `json:"terms,omitempty"`
	TopicBool *TopicBool   `json:"bool,omitempty"`
	// When a moment group contains both composite and non-composite moments.
	// TODO(zhucaiyi): cleanup, remove this field and use other structs for non-composite moments.
	Nested    *Nested    `json:"nested,omitempty"`
	HasParent *HasParent `json:"has_parent,omitempty"`
}

type TopicBool struct {
	// The covers excluded keywords in a topic.
	// TODO(zhucaiyi): remove this struct since it can be replaced by KeywordsIncludedOrClustersShould.
	MustNot *[]KeywordExcludedMustNot `json:"must_not,omitempty"`
	// Using this value to shift from `included OR (OR clusters) to included AND (AND clusters).
	MinimumShouldMatch int32 `json:"minimum_should_match,omitempty"`
	// This covers cluster ids and included keywords.
	Should *[]KeywordsIncludedOrClustersShould `json:"should,omitempty"`
	// This covers filter like prerequirement moments.
	Filter *[]TopicBoolFilter `json:"filter,omitempty"`
}

type TopicBoolFilter struct {
	Nested      *Nested      `json:"nested,omitempty"`
	Terms       *TermsFilter `json:"terms,omitempty"`
	RangeFilter *RangeFilter `json:"range,omitempty"`
}

type KeywordExcludedMustNot struct {
	KeywordsToExclude *KeywordMomentBool `json:"bool,omitempty"`
	Field             *Field             `json:"field,omitempty"`
	Exists            *Field             `json:"exists,omitempty"`
}

type KeywordsIncludedOrClustersShould struct {
	Nested            *Nested            `json:"nested,omitempty"`
	KeywordsToInclude *KeywordMomentBool `json:"bool,omitempty"`
}

// Representing the boolean query for a keyword moment.
type KeywordMomentBool struct {
	// For other attributes
	Filters []KeywordsIncludedFilter `json:"filter,omitempty"`
	// For OR operator.
	Should *[]KeywordMomentBoolFilter `json:"should,omitempty"`
	// For AND operator
	Must *[]KeywordMomentBoolFilter `json:"must,omitempty"`
	// For EXCLUDE Operator.
	MustNot            *[]KeywordMomentBoolFilter `json:"must_not,omitempty"`
	MinimumShouldMatch int32                      `json:"minimum_should_match,omitempty"`
}

type KeywordMomentBoolFilter struct {
	// For Phrase operand.
	MultiMatch *MultiMatchField `json:"multi_match,omitempty"`
	// For KeywordExpression oprand.
	Bool *KeywordMomentBool `json:"bool,omitempty"`
	// For Regex operand.
	Regexp *RegexpField `json:"regexp,omitempty"`
}

type RegexpField struct {
	MessageTextKeyword RegexpValue `json:"message_text.keyword"`
}

type RegexpValue struct {
	Value           string `json:"value"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty"`
}

// Each KeywordsIncludedFilter.
type KeywordsIncludedFilter struct {
	TermsFilter           *TermsFilter `json:"terms,omitempty"`
	MessagePositionFilter *RangeFilter `json:"range,omitempty"`
}

// The entry point of the ES query `query` section.
type OuterQuery struct {
	// Should be either *ConvoOuterBool or *MsgOuterBool.
	Bool interface{} `json:"bool,omitempty"`
	// Used for wrapping bool field when we need to randomize output result.
	FunctionScore *FunctionScore `json:"function_score,omitempty"`
}

// The OuterBool used for convo document filters.
type ConvoOuterBool struct {
	ConvoDocumentFilter *[]ConvoFilter      `json:"filter,omitempty"`
	MustNot             *[]OuterBoolMustNot `json:"must_not,omitempty"`
}

// The OuterBool userd for message document filters.
type MsgOuterBool struct {
	Filter             *[]MsgFilter `json:"filter,omitempty"`
	Must               *[]MsgFilter `json:"must,omitempty"`
	Should             *[]MsgFilter `json:"should,omitempty"`
	MinimumShouldMatch int32        `json:"minimum_should_match,omitempty"`
}

type OuterBoolMustNot struct {
	*ConvoFilter
}

// The entry point of ES query `aggs` field.
type ReqAggs struct {
	TotalConversationCount             *CardinalityCount          `json:"total_conversation_count,omitempty"`
	TotalConversationMessageCount      *CardinalityCount          `json:"total_conversation_message_count,omitempty"`
	TotalDocumentCount                 *CardinalityCount          `json:"total_document_count,omitempty"`
	TotalConversationHandleTimeSecsSum *AggregationOnNumericField `json:"total_conversation_handle_time_secs_sum,omitempty"`
	TimeRanges                         *ReqTimeRanges             `json:"time_ranges,omitempty"`
	Moments                            *ReqAnnotations            `json:"moments,omitempty"`
}

type SortKey struct {
	Format string `json:"format,omitempty"`
	Order  string `json:"order"`
}

type Sort struct {
	// Available in both MessageDoc and ConversationDoc.
	ConversationEndedAt *SortKey `json:"conversation_ended_at,omitempty"`
	// Creation timestamp for MessageDoc.
	CreatedAt      *SortKey `json:"created_at,omitempty"`
	ConversationID *SortKey `json:"conversation_id.keyword,omitempty"`
	Score          *SortKey `json:"_score,omitempty"`
}

type Script struct {
	Source string `json:"source"`
}

type Cardinality struct {
	Field              *string `json:"field,omitempty"`
	Script             *Script `json:"script,omitempty"`
	PrecisionThreshold int     `json:"precision_threshold"`
}

type CardinalityCount struct {
	Cardinality Cardinality `json:"cardinality,omitempty"`
}

type NumericField struct {
	Field *string `json:"field,omitempty"`
	// Default value when the field is None.
	Missing *int32 `json:"missing,omitempty"`
}

type PercentilesMetric struct {
	Field    string    `json:"field,omitempty"`
	Percents []float64 `json:"percents,omitempty"`
}

// ES Metrics Aggregation on numeric field.
type AggregationOnNumericField struct {
	Sum         *NumericField      `json:"sum,omitempty"`
	Avg         *NumericField      `json:"avg,omitempty"`
	Max         *NumericField      `json:"max,omitempty"`
	Min         *NumericField      `json:"min,omitempty"`
	Percentiles *PercentilesMetric `json:"percentiles,omitempty"`
}

type ReqDateHistogram struct {
	Field            string                `json:"field"`
	TimeZone         string                `json:"time_zone"`
	Offset           *string               `json:"offset,omitempty"`
	CalendarInterval string                `json:"calendar_interval,omitempty"`
	FixedInterval    string                `json:"fixed_interval,omitempty"`
	MinDocCount      *int32                `json:"min_doc_count,omitempty"`
	ExtendedBound    *ExtendedBoundForTime `json:"extended_bounds,omitempty"`
}

type ExtendedBoundForTime struct {
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
}

type ReqNestedAggCounts struct {
	TotalConversationCount             *CardinalityCount          `json:"total_conversation_count,omitempty"`
	TotalConversationMessageCount      *CardinalityCount          `json:"total_conversation_message_count,omitempty"`
	TotalConversationHandleTimeSecsSum *AggregationOnNumericField `json:"total_conversation_handle_time_secs_sum,omitempty"`
	Moments                            *ReqAnnotations            `json:"moments,omitempty"`
}

// Add
type ReqTimeRanges struct {
	DateHistogram ReqDateHistogram   `json:"date_histogram"`
	Aggs          ReqNestedAggCounts `json:"aggs"`
}

type ReqResource struct {
	Includes []string `json:"includes"`
}

type ReqTopHits struct {
	Size   int         `json:"size"`
	Source ReqResource `json:"_source"`
}

type ReqAnnotationHit struct {
	TopHits ReqTopHits `json:"top_hits"`
}

type ReqMomentAnnotationFilter struct {
	Bool NestedFilterBool `json:"bool,omitempty"`
}

type TermsAggs struct {
	Field     *string `json:"field,omitempty"`
	Size      *int32  `json:"size,omitempty"`
	ShardSize *int32  `json:"shard_size,omitempty"`
	// Only allow single keyword:value pair. And value can only be desc or asc.
	OrderBy map[string]string `json:"order,omitempty"`
}

type StatsAggs struct {
	Field *string `json:"field,omitempty"`
}

type ReqPayloadValue struct {
	Terms *TermsAggs `json:"terms,omitempty"`
	Stats *StatsAggs `json:"stats,omitempty"`
}

type ReqMomentAnnotationPayloadValueAggs struct {
	PayloadValue ReqPayloadValue `json:"payload_value"`
}

type ReqMomentAnnotations struct {
	Filter ReqMomentAnnotationFilter           `json:"filter"`
	Aggs   ReqMomentAnnotationPayloadValueAggs `json:"aggs"`
}

type ReqAggAnnotation struct {
	TotalConversationCount             *CardinalityCount          `json:"total_conversation_count,omitempty"`
	TotalConversationMessageCount      *CardinalityCount          `json:"total_conversation_message_count,omitempty"`
	TotalConversationHandleTimeSecsSum *AggregationOnNumericField `json:"total_conversation_handle_time_secs_sum,omitempty"`
	TotalDocumentCount                 *CardinalityCount          `json:"total_document_count,omitempty"`
	AnnotationHit                      *ReqAnnotationHit          `json:"annotation_hit,omitempty"`
	MomentAnnotations                  *ReqMomentAnnotations      `json:"moment_annotations,omitempty"`
}

type ReqAnnotationSubFilter struct {
	Nested *Nested `json:"nested,omitempty"`
	// The type can be *TopicBool and *KeywordMomentBool
	Bool     interface{} `json:"bool,omitempty"`
	HasChild *HasChild   `json:"has_child,omitempty"`
}

type ReqAnnotationFilters struct {
	Filters []ReqAnnotationSubFilter `json:"filters"`
}

// TODO(zhucaiyi): rename this to something more generic rather than just for moment.
// Also remember to update the json tags that reference this struct.
type ReqAnnotations struct {
	Nested  *Nested               `json:"nested,omitempty"`
	Filters *ReqAnnotationFilters `json:"filters,omitempty"`
	Aggs    *ReqAggAnnotation     `json:"aggs,omitempty"`
	Terms   *TermsAggs            `json:"terms,omitempty"`
}

// The entry point of ES query `highlight field`.
type ReqHighlight struct {
	PreTags  []string        `json:"pre_tags"`
	PostTags []string        `json:"post_tags"`
	Fields   HighlightFields `json:"fields,omitempty"`
}

type HighlightConfig struct {
	NumberOfFragments int32 `json:"number_of_fragments"`
}

type HighlightFields struct {
	// Default
	MessageText           *HighlightConfig `json:"message_text,omitempty"`
	MessageTextWhitespace *HighlightConfig `json:"message_text_whitespace,omitempty"`

	// English with custom analyzer
	MessageTextWithCustomAnalyzerEnglish *HighlightConfig `json:"message_text_with_custom_analyzer.english,omitempty"`
	// Spanish with custom analyzer
	MessageTextWithCustomAnalyzerSpanish *HighlightConfig `json:"message_text_with_custom_analyzer.spanish,omitempty"`
	// French with custom analyzer
	MessageTextWithCustomAnalyzerFrench *HighlightConfig `json:"message_text_with_custom_analyzer.french,omitempty"`
	// Italian with custom analyzer
	MessageTextWithCustomAnalyzerItalian *HighlightConfig `json:"message_text_with_custom_analyzer.italian,omitempty"`
	// German with custom analyzer
	MessageTextWithCustomAnalyzerGerman *HighlightConfig `json:"message_text_with_custom_analyzer.german,omitempty"`
	// Dutch with custom analyzer
	MessageTextWithCustomAnalyzerDutch *HighlightConfig `json:"message_text_with_custom_analyzer.dutch,omitempty"`
	// Portuguese with custom analyzer
	MessageTextWithCustomAnalyzerPortuguese *HighlightConfig `json:"message_text_with_custom_analyzer.portuguese,omitempty"`
	// Russian with custom analyzer
	MessageTextWithCustomAnalyzerRussian *HighlightConfig `json:"message_text_with_custom_analyzer.russian,omitempty"`
	// Thai with custom analyzer
	MessageTextWithCustomAnalyzerThai *HighlightConfig `json:"message_text_with_custom_analyzer.thai,omitempty"`
}

type Source struct {
	Includes *[]string `json:"includes,omitempty"`
	Excludes *[]string `json:"excludes,omitempty"`
}

type RuntimeMappings map[string]RuntimeMapping

type RuntimeMapping struct {
	Type   string         `json:"type"`
	Script *RuntimeScript `json:"script,omitempty"`
}

type RuntimeScript struct {
	Source string            `json:"source"`
	Params map[string]string `json:"params,omitempty"`
}

type Request struct {
	Source          *Source          `json:"_source,omitempty"`
	Highlight       *ReqHighlight    `json:"highlight,omitempty"`
	Query           *OuterQuery      `json:"query,omitempty"`
	SearchAfter     *[]string        `json:"search_after,omitempty"`
	Sort            *[]Sort          `json:"sort,omitempty"`
	Aggs            *ReqAggs         `json:"aggs,omitempty"`
	RuntimeMappings *RuntimeMappings `json:"runtime_mappings,omitempty"`
}

type FunctionScore struct {
	Query       any                  `json:"query,omitempty"`
	RandomScore *RandomScoreFunction `json:"random_score,omitempty"`
	BoostMode   string               `json:"boost_mode,omitempty"`
}

type RandomScoreFunction struct {
	Seed  int32  `json:"seed,omitempty"`
	Field string `json:"field,omitempty"`
}

// Filter parameters applied to conversation documents.
type ConversationFilterParams struct {
	TimeRange                      *TimeRange
	TargetFieldForTimeRangeFilter  analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange
	ConversationDurationSecs       *FloatRange
	ConversationDurationSecsRanges []*FloatRange
	ConversationHandleTimeSecs     *FloatRange
	MessageCountRange              *FloatRange
	AgentUserIDs                   []string
	// AccessControlAgentUserIDs and AccessControlGroupIDs work together to filter convos.
	// The logic is AccessControlAgentUserIDs OR AccessControlGroupIDs.
	AccessControlAgentUserIDs []string
	AccessControlGroupIDs     []string
	ConversationSources       []int32
	ConversationChannels      []int32
	ConversationIDs           []string
	PlatformConversationIDs   []string
	UsecaseIDs                []string
	// TODO: remove dependency from Momentgroup.
	MomentGroups               []*analyticspb.MomentGroup
	Comments                   analyticspb.Attribute_Comments
	AIAgentFeedback            analyticspb.Attribute_AIAgentFeedback
	IncludeDevUser             bool
	ScorecardTemplateIDs       []string
	ScorecardStatuses          []analyticspb.Attribute_ScorecardStatus
	ScorecardStatusFilters     []*analyticspb.Attribute_ScorecardStatusFilter
	ConversationTagIDs         []string
	LiveAssists                []analyticspb.Attribute_LiveAssist
	SummaryUsages              []analyticspb.Attribute_SummaryUsage
	SuggestionUsages           []analyticspb.Attribute_SuggestionUsage
	NoteTakingUsages           []analyticspb.Attribute_NoteTakingUsage
	LanguageTransfers          []*analyticspb.Attribute_LanguageTransferEvent
	ConversationOnHoldCriteria *analyticspb.Attribute_ConversationHoldsCriterion
	LanguageCodes              []string
	KBDocumentIDs              []string
	GWDocumentIDs              []string
	ScreenRecording            analyticspb.Attribute_ScreenRecording
	ScorecardFilters           []*analyticspb.ScorecardFilter
	// If present, it will override the ConversationSources.
	OverrideConversationSources []int32
	ScorecardSubmitterUserIDs   []string
	CalibrationStatuses         []analyticspb.Attribute_ScorecardStatus
	ChildMessageFilter          *MessageFilterParams
	PlatformVisitorID           []string
	TransferInfo                *analyticspb.Attribute_TransferInfoFilter
	TruncatedTimeRanges         []*analyticspb.Attribute_TruncatedTimeRange
	VirtualAgentRevisionIDs     []string
	IngestedByJobIDs            []string
	DisplayNames                []string
	// Filter by email attachment conditions. Backed by the denormalized
	// has_email_attachments flag on the conversation document.
	EmailAttachments analyticspb.Attribute_EmailAttachments

	WorkflowAutomationConditions []analyticspb.Attribute_WorkflowAutomationCondition
}

// Filter parameters applied to message documents.
type MessageFilterParams struct {
	MomentGroups                  []*analyticspb.MomentGroup
	ActionGroups                  []*analyticspb.ActionGroup
	SpeakerRoles                  []string
	UsecaseIDs                    []string
	KeywordsExpressions           []string
	LanguageCodes                 []string
	KnowledgeAssist               *analyticspb.Attribute_KnowledgeMomentFilter
	AgentUserIDs                  []string
	TimeRange                     *TimeRange
	TargetFieldForTimeRangeFilter analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange
	ConversationIDs               []string
	PlatformConversationIDs       []string
	ConversationSources           []int32
	MessageCountRange             *FloatRange
}

func BuildRuntimeMappingsForTruncatedTimeRangeFilters(timeZone string, targetField analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange) *RuntimeMappings {
	if timeZone == "" {
		timeZone = shared.DefaultTimeZone
	}
	baseField := "conversation_started_at"
	if targetField == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		baseField = "conversation_ended_at"
	}

	baseSource := fmt.Sprintf("def z=params.tz; def id=(z.startsWith('+')||z.startsWith('-')) ? ZoneOffset.of(z) : ZoneId.of(z); def v=doc['%s'].value.withZoneSameInstant(id)", baseField)

	dayGetter := ".getDayOfWeek().getValue()"
	hoursToMinutesGetter := ".getHour()*60"
	minuteGetter := ".getMinute()"

	daySource := fmt.Sprintf("%s; emit(v%s)", baseSource, dayGetter)
	minuteOfDaySource := fmt.Sprintf("%s; emit(v%s + v%s)", baseSource, hoursToMinutesGetter, minuteGetter)

	return &RuntimeMappings{
		"truncated_day_of_week": RuntimeMapping{
			Type: "long",
			Script: &RuntimeScript{
				Source: daySource,
				Params: map[string]string{"tz": timeZone},
			},
		},
		"truncated_minute_of_day": RuntimeMapping{
			Type: "long",
			Script: &RuntimeScript{
				Source: minuteOfDaySource,
				Params: map[string]string{"tz": timeZone},
			},
		},
	}
}

func BuildTruncatedTimeRangeFilters(truncatedTimeRanges []*analyticspb.Attribute_TruncatedTimeRange) *ConvoFilter {
	if len(truncatedTimeRanges) == 0 {
		return nil
	}

	var shouldClauses []ConvoFilterBoolFilter

	for _, timeRange := range truncatedTimeRanges {
		if timeRange.Start == nil && timeRange.End == nil {
			continue
		}

		var rangeFilters []ConvoFilter
		var minuteOfDayRange *FloatRange

		if start := timeRange.Start; start != nil {
			if timeOfDay := start.GetTimeOfDay(); timeOfDay != nil {
				hours := 0
				minutes := 0
				if timeOfDay.Hour >= 0 {
					hours = int(timeOfDay.Hour)
				}
				if timeOfDay.Minute >= 0 {
					minutes = int(timeOfDay.Minute)
				}
				minuteOfDayRange = &FloatRange{GTE: proto.Float64(float64(hours*60 + minutes))}
			}
		}
		if end := timeRange.End; end != nil {
			if timeOfDay := end.GetTimeOfDay(); timeOfDay != nil {
				if minuteOfDayRange == nil {
					minuteOfDayRange = &FloatRange{}
				}
				hours := 0
				minutes := 0
				if timeOfDay.Hour >= 0 {
					hours = int(timeOfDay.Hour)
				}
				if timeOfDay.Minute >= 0 {
					minutes = int(timeOfDay.Minute)
				}
				minuteOfDayRange.LTE = proto.Float64(float64(hours*60 + minutes))
			}
		}
		if minuteOfDayRange != nil {
			rangeFilters = append(rangeFilters, ConvoFilter{
				Range: &RangeFilter{
					TruncatedMinuteOfDay: minuteOfDayRange,
				},
			})
		}

		var dayOfWeekRange *FloatRange
		if start := timeRange.Start; start != nil {
			if dayOfWeek := start.GetDayOfWeek(); dayOfWeek != nil && dayOfWeek.DayOfWeek != 0 {
				dayOfWeekRange = &FloatRange{
					GTE: proto.Float64(float64(dayOfWeek.DayOfWeek)),
				}
			}
		}
		if end := timeRange.End; end != nil {
			if dayOfWeek := end.GetDayOfWeek(); dayOfWeek != nil && dayOfWeek.DayOfWeek != 0 {
				if dayOfWeekRange == nil {
					dayOfWeekRange = &FloatRange{}
				}
				dayOfWeekRange.LTE = proto.Float64(float64(dayOfWeek.DayOfWeek))
			}
		}
		if dayOfWeekRange != nil {
			rangeFilters = append(rangeFilters, ConvoFilter{
				Range: &RangeFilter{
					TruncatedDayOfWeek: dayOfWeekRange,
				},
			})
		}

		if len(rangeFilters) > 0 {
			jsonData := map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"filter": rangeFilters,
					},
				},
			}
			jsonBytes, _ := json.Marshal(jsonData)
			rawMessage := json.RawMessage(jsonBytes)

			shouldClauses = append(shouldClauses, ConvoFilterBoolFilter{
				FunctionScore: &rawMessage,
			})
		}
	}

	if len(shouldClauses) == 0 {
		return nil
	}

	return &ConvoFilter{
		Bool: &ConvoFilterBool{
			Should:             &shouldClauses,
			MinimumShouldMatch: 1,
		},
	}
}

// nolint: gocyclo
func ParseFilterByAttributeToQuery(
	convoDocFilter *ConversationFilterParams,
	msgDocFilter *MessageFilterParams,
	targetDocType DocumentType,
	topicDetails map[string]TopicDetails,
	esQueryConfig QueryConfig,
) (OuterQuery, error) {
	// Conversation level filter.
	convoFilters, excludeConvoFilters := []ConvoFilter{}, []ConvoFilter{}
	if convoDocFilter.TimeRange != nil {
		convoFilters = append(convoFilters, ConvoFilter{
			Range: extractTimeRangeFromConversationFilterParams(convoDocFilter),
		})
	}

	filters, excludeFilters, err := parseConvoFilter(convoDocFilter, topicDetails, esQueryConfig.EnableNumHoldsField)
	if err != nil {
		return OuterQuery{}, err
	}
	convoFilters = append(convoFilters, filters...)
	excludeConvoFilters = append(excludeConvoFilters, excludeFilters...)

	truncatedTimeRangeFilter := BuildTruncatedTimeRangeFilters(convoDocFilter.TruncatedTimeRanges)
	if truncatedTimeRangeFilter != nil {
		convoFilters = append(convoFilters, *truncatedTimeRangeFilter)
	}

	// Filter conversation documents to return.
	if targetDocType == DocumentTypeConversation {
		matchConversationDocument := ConvoFilter{
			Terms: &TermsFilter{
				ConversationToMessageJoin: &[]string{string(DocumentTypeConversation)},
			},
		}
		convoFilters = append(convoFilters, matchConversationDocument)

		hasHasChild := false
		for _, f := range convoFilters {
			if f.HasChild != nil {
				hasHasChild = true
				break
			}
		}
		// Add a match_all has_child query to ensure empty convos are excluded.
		if (!hasConversationIDConvoFilter(convoDocFilter)) && (!esQueryConfig.IncludeEmptyConversations && !hasHasChild) && convoDocFilter.MessageCountRange == nil {
			convoFilters = append(convoFilters, ConvoFilter{
				Range: &RangeFilter{
					ConversationMessageCount: &FloatRange{
						GT: proto.Float64(0.0),
					},
				},
			})
		}

		outerBool := ConvoOuterBool{
			ConvoDocumentFilter: &convoFilters,
		}
		if len(excludeConvoFilters) > 0 {
			mustNot := []OuterBoolMustNot{}
			for _, f := range excludeConvoFilters {
				// Need to declare a new var inside for loop, because f is only created once at the beginning of the for loop.
				filter := f
				mustNot = append(mustNot, OuterBoolMustNot{ConvoFilter: &filter})
			}
			outerBool.MustNot = &mustNot
		}
		return OuterQuery{Bool: &outerBool}, nil
	}

	// Filter message documents to return.
	convoBool := ConvoBool{Filter: &convoFilters}
	if len(excludeConvoFilters) > 0 {
		convoBool.MustNot = &excludeConvoFilters
	}
	outerMsgFilters := []MsgFilter{}
	// We can only convert specific conversation filters to message filters if the documentType is message.
	if targetDocType == DocumentTypeMessage && ConvoFiltersOnlyContainDenormalizedFields(convoDocFilter) && msgDocFilter != nil {
		matchConversationDocument := MsgFilter{
			Terms: &TermsFilter{
				ConversationToMessageJoin: &[]string{string(DocumentTypeMessage), string(DocumentTypeMessageGroup)},
			},
		}
		outerMsgFilters = append(outerMsgFilters, matchConversationDocument)
		if msgDocFilter.TimeRange != nil {
			outerMsgFilters = append(outerMsgFilters, MsgFilter{
				Range: extractTimeRangeFromMessageFilterParams(msgDocFilter),
			})
		}
		if msgDocFilter.MessageCountRange != nil {
			outerMsgFilters = append(outerMsgFilters, MsgFilter{
				Range: &RangeFilter{
					ConversationMessageCount: msgDocFilter.MessageCountRange,
				},
			})
		}
		if len(msgDocFilter.AgentUserIDs) > 0 {
			outerMsgFilters = append(outerMsgFilters, MsgFilter{
				Terms: &TermsFilter{
					AgentUserIDs: &msgDocFilter.AgentUserIDs,
				},
			})
		}
		if !hasConversationIDMsgFilter(msgDocFilter) && len(msgDocFilter.ConversationSources) > 0 {
			outerMsgFilters = append(outerMsgFilters, convertConvoSourcesToMessageFilter(msgDocFilter.ConversationSources))
		}
		if len(msgDocFilter.ConversationIDs) > 0 {
			outerMsgFilters = append(outerMsgFilters, MsgFilter{
				Terms: &TermsFilter{
					ConversationIDs: &msgDocFilter.ConversationIDs,
				},
			})
		}
		if len(msgDocFilter.PlatformConversationIDs) > 0 {
			outerMsgFilters = append(outerMsgFilters, MsgFilter{
				Terms: &TermsFilter{
					PlatformConversationIDs: &msgDocFilter.PlatformConversationIDs,
				},
			})
		}
		// UsecaseIDs are handled below (line ~1320) as part of normal message filter processing,
		// so we don't need to add them here in the denormalized path.
	} else {
		outerMsgFilters = append(outerMsgFilters, []MsgFilter{
			{
				HasParent: &HasParent{
					ParentType: "conversation",
					Query: ConvoQuery{
						Bool: convoBool,
					},
				},
			},
		}...)
	}

	if msgDocFilter == nil {
		return OuterQuery{Bool: &MsgOuterBool{Filter: &outerMsgFilters}}, nil
	}

	// Message level filter.
	for _, momentGroup := range msgDocFilter.MomentGroups {
		if len(momentGroup.ExcludedMoments) > 0 && len(momentGroup.Moments) > 0 {
			// TODO(zhucaiyi): consolidate validation logic for MomentGroup.
			return OuterQuery{}, status.Errorf(codes.InvalidArgument, "found both `excluded_moments` and `moments` are set")
		}

		if len(momentGroup.ExcludedMoments) > 0 {
			momentIDs := []string{}
			for _, m := range momentGroup.ExcludedMoments {
				if IsInsightsCompositeMoment(m) {
					return OuterQuery{}, status.Errorf(codes.InvalidArgument, "does not support insights Composite moment in `excluded_moments`")
				}
				momentName, err := momentpb.ParseMomentName(m.Name)
				if err != nil {
					return OuterQuery{}, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", m.Name, err)
				}
				momentIDs = append(momentIDs, momentName.MomentID)
			}
			nested := &Nested{
				Path: MomentAnnotationsPath,
				Query: &NestedFilter{
					Bool: &NestedFilterBool{
						MustNot: &[]NestedFilter{
							{
								Terms: &NestedTermsFilter{
									MomentTemplateIDs: &momentIDs,
								},
							},
						},
					},
				},
			}
			outerMsgFilters = append(outerMsgFilters, MsgFilter{Nested: nested})
			if IsConversationReasonMomentGroup(momentGroup) {
				outerMsgFilters = append(outerMsgFilters, MsgFilter{
					HasParent: nestedHasParentConversation(nested),
				})
			}
		} else {
			insightsCompositeNames := []string{}
			nonInsightsCompositeIDs := []string{}
			for _, moment := range momentGroup.Moments {
				if IsInsightsCompositeMoment(moment) {
					insightsCompositeNames = append(insightsCompositeNames, moment.Name)
				} else {
					momentName, err := momentpb.ParseMomentName(moment.Name)
					if err != nil {
						return OuterQuery{}, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", moment.Name, err)
					}
					nonInsightsCompositeIDs = append(nonInsightsCompositeIDs, momentName.MomentID)
				}
			}

			// Fall back to the case where all moments are non-composite.
			if len(insightsCompositeNames) == 0 {
				filters := []NestedFilter{}
				should := []NestedFilter{}
				if momentGroup.MomentType != momentpb.Moment_TYPE_UNSPECIFIED {
					dbMomentType, err := momentconverter.ServiceToDBMomentType(momentGroup.MomentType)
					if err != nil {
						return OuterQuery{}, err
					}
					filters = append(filters, NestedFilter{
						Terms: &NestedTermsFilter{
							MomentTypes: &[]int{int(dbMomentType)},
						},
					})
				}
				if len(nonInsightsCompositeIDs) > 0 || len(momentGroup.Behaviors) > 0 {
					f, s, err := nestedFilterWithMomentAdherenceType(momentGroup.Adherence, momentGroup.AdherenceTypesForMoments, nonInsightsCompositeIDs, momentGroup.Behaviors)
					if err != nil {
						return OuterQuery{}, err
					}
					filters = append(filters, f...)
					should = append(should, s...)
				}
				nestedFilterBool := NestedFilterBool{}
				if len(filters) > 0 {
					nestedFilterBool.Filter = &filters
				}
				if len(should) > 0 {
					nestedFilterBool.Should = &should
					nestedFilterBool.MinimumShouldMatch = 1
				}
				nested := &Nested{
					Path: MomentAnnotationsPath,
					Query: &NestedFilter{
						Bool: &nestedFilterBool,
					},
				}
				if IsConversationReasonMomentGroup(momentGroup) {
					outerMsgFilters = append(outerMsgFilters, MsgFilter{Bool: &TopicsBool{Should: &[]TopicsShould{
						{Nested: nested},
						{HasParent: nestedHasParentConversation(nested)},
					}}})
				} else {
					outerMsgFilters = append(outerMsgFilters, MsgFilter{Nested: nested})
				}
			} else {
				should := []TopicsShould{}
				for _, name := range insightsCompositeNames {
					if _, exist := topicDetails[name]; !exist {
						return OuterQuery{}, fmt.Errorf("topic details for moment %q does not exists", name)
					}
					topicBool, err := ConvertTopicDetailToTopicBool(topicDetails[name], msgDocFilter.LanguageCodes)
					if err != nil {
						return OuterQuery{}, err
					}
					should = append(should, TopicsShould{TopicBool: topicBool})
				}
				if len(nonInsightsCompositeIDs) > 0 {
					nested := &Nested{
						Path: MomentAnnotationsPath,
						Query: &NestedFilter{
							Bool: &NestedFilterBool{
								Filter: &[]NestedFilter{
									{
										Terms: &NestedTermsFilter{MomentTemplateIDs: &nonInsightsCompositeIDs},
									},
								},
							},
						},
					}
					should = append(should, TopicsShould{Nested: nested})
					if IsConversationReasonMomentGroup(momentGroup) {
						should = append(should, TopicsShould{HasParent: nestedHasParentConversation(nested)})
					}
				}
				outerMsgFilters = append(outerMsgFilters, MsgFilter{Bool: &TopicsBool{Should: &should}})
			}
		}
	}

	if len(msgDocFilter.ActionGroups) > 1 || (len(msgDocFilter.ActionGroups) == 1 && len(msgDocFilter.ActionGroups[0].Actions) > 1) {
		return OuterQuery{}, status.Errorf(codes.InvalidArgument, "only support at most 1 action_group and at most 1 action in action_group in message filter")
	}
	for _, ag := range msgDocFilter.ActionGroups {
		// Ignore hint type filter since it is not needed at the moment.
		// TODO(zhucaiyi): consolidate logic that convert MomentGroup to filters.
		_, filtersForHints, err := convertActionGroupToNestedFilters(ag)
		if err != nil {
			return OuterQuery{}, err
		}
		if len(filtersForHints) == 0 {
			continue
		}
		for _, fh := range filtersForHints {
			ff := HintFilterForMessage(fh)
			outerMsgFilters = append(outerMsgFilters, ff...)
		}
	}

	if len(msgDocFilter.SpeakerRoles) != 0 {
		esSpeakerRoles, err := shared.ConvertAPIToESSpeakerRoles(msgDocFilter.SpeakerRoles)
		if err != nil {
			return OuterQuery{}, err
		}

		outerMsgFilters = append(outerMsgFilters, MsgFilter{Terms: &TermsFilter{SpeakerRoles: &esSpeakerRoles}})
	}

	if len(msgDocFilter.UsecaseIDs) > 0 {
		outerMsgFilters = append(outerMsgFilters, MsgFilter{Terms: &TermsFilter{UsecaseIDs: &msgDocFilter.UsecaseIDs}})
	}

	if len(msgDocFilter.LanguageCodes) > 0 {
		outerMsgFilters = append(outerMsgFilters, MsgFilter{Terms: &TermsFilter{LangCodes: &msgDocFilter.LanguageCodes}})
	}

	outerBool := MsgOuterBool{
		Filter: &outerMsgFilters,
	}

	if len(msgDocFilter.KeywordsExpressions) != 0 {
		keywordMust, keywordShould, err := parseSearchKeyword(
			msgDocFilter.KeywordsExpressions[0], msgDocFilter.LanguageCodes)
		if err != nil {
			return OuterQuery{}, err
		}
		if len(keywordMust) > 0 {
			outerBool.Must = &keywordMust
		}
		if len(keywordShould) > 0 {
			outerBool.Should = &keywordShould
			outerBool.MinimumShouldMatch = minShouldMatch
		}
	}
	return OuterQuery{
		Bool: &outerBool,
	}, nil
}

func extractTimeRangeFromConversationFilterParams(conversationFilterParams *ConversationFilterParams) *RangeFilter {
	// By default, time range filter applies to conversation_started_at.
	if conversationFilterParams.TargetFieldForTimeRangeFilter == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		return &RangeFilter{
			ConversationEndedAt: conversationFilterParams.TimeRange,
		}
	} else {
		return &RangeFilter{
			ConversationStartedAt: conversationFilterParams.TimeRange,
		}
	}
}

func extractTimeRangeFromMessageFilterParams(messageFilterParams *MessageFilterParams) *RangeFilter {
	if messageFilterParams.TargetFieldForTimeRangeFilter == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		return &RangeFilter{
			ConversationEndedAt: messageFilterParams.TimeRange,
		}
	}

	return &RangeFilter{
		ConversationStartedAt: messageFilterParams.TimeRange,
	}
}

// ConvoFiltersOnlyContainDenormalizedFields returns true when the conversation filters
// are limited to fields that have been denormalized to message documents as well. This allows the
// message query to avoid a parent join if it only relies on shared fields.
func ConvoFiltersOnlyContainDenormalizedFields(conversationFilterParams *ConversationFilterParams) bool {
	if conversationFilterParams == nil {
		return true
	}

	// Strip message-document fields and check if anything non-message remains.
	copied := *conversationFilterParams
	copied.TimeRange = nil
	copied.TargetFieldForTimeRangeFilter = analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED
	copied.MessageCountRange = nil
	copied.AgentUserIDs = nil
	copied.ConversationSources = nil
	copied.ConversationIDs = nil
	copied.PlatformConversationIDs = nil
	copied.OverrideConversationSources = nil
	copied.UsecaseIDs = nil

	return cmp.Equal(copied, ConversationFilterParams{}, cmpopts.EquateEmpty())
}

func GetDefaultConversationSources() []int32 {
	sources := []int32{}
	for _, val := range strings.Split(*defaultConversationSources, ",") {
		s, err := strconv.Atoi(val)
		if err != nil {
			continue
		} else {
			sources = append(sources, int32(s)) //nolint:gosec
		}

	}
	return sources
}

func convertConvoSourcesToConvoFilter(sources []int32) ConvoFilter {
	f := ConvoFilter{
		Bool: &ConvoFilterBool{
			Should: &[]ConvoFilterBoolFilter{
				{
					Terms: &TermsFilter{
						ConversationSources: &sources,
					},
				},
			},
		},
	}

	for _, s := range sources {
		// Production traffic includes source=0 and source not exist.
		if s == 0 {
			*f.Bool.Should = append(*f.Bool.Should, ConvoFilterBoolFilter{
				Bool: &ConvoFilterBoolFilterBool{
					MustNot: &ConvoFilterBoolFilterBoolFilter{
						Exists: &Field{
							Field: "conversation_source",
						},
					},
				},
			})
			break
		}
	}
	return f
}

// TODO(zhucaiyi): do not use TopicsBool for conversation source filter.
func convertConvoSourcesToMessageFilter(sources []int32) MsgFilter {
	topicsBool := &TopicsBool{
		Should: &[]TopicsShould{
			{
				Terms: &TermsFilter{
					ConversationSources: &sources,
				},
			},
		},
	}

	for _, s := range sources {
		// Production traffic includes source=0 and source not exist.
		if s == 0 {
			*topicsBool.Should = append(*topicsBool.Should, TopicsShould{
				TopicBool: &TopicBool{
					MustNot: &[]KeywordExcludedMustNot{
						{
							Exists: &Field{
								Field: "conversation_source",
							},
						},
					},
				},
			})
			break
		}
	}

	return MsgFilter{Bool: topicsBool}
}

func extractConvoSourcesFromConversationFilterParams(conversationFilterParams *ConversationFilterParams) []int32 {
	// 0: prodction traffic, 7: bot conversation, 8: historic calls from ingestion pipeline.
	// See ConversationSource in https://github.com/cresta/go-servers/blob/main/apiserver/sql-schema/protos/conversation/conversation.proto.
	convoSources := GetDefaultConversationSources()
	convoSources = append(convoSources, conversationFilterParams.ConversationSources...)
	// If overrideConversationSources set, only use it to build ES query.
	if len(conversationFilterParams.OverrideConversationSources) > 0 {
		convoSources = conversationFilterParams.OverrideConversationSources
	}
	return convoSources
}

// convertConvoChannelsToConvoFilter builds a conversation-level terms filter
// restricting to the given conversation channels (e.g. CHAT, VOICE, EMAIL).
func convertConvoChannelsToConvoFilter(channels []int32) ConvoFilter {
	return ConvoFilter{
		Terms: &TermsFilter{
			ConversationChannels: &channels,
		},
	}
}

func convoFilterFromExcludingConsultConvo() ConvoFilter {
	return ConvoFilter{
		Terms: &TermsFilter{
			TransferInfoParticipantInteractionTypes: &[]int32{int32(dbconvopb.ConversationMetadata_CONSULTATION)},
		},
	}
}

func parseConvoFilter(
	convoDocFilter *ConversationFilterParams,
	topicDetails map[string]TopicDetails,
	enableNumHoldsField bool,
) ([]ConvoFilter, []ConvoFilter, error) {
	switch convoDocFilter.AIAgentFeedback {
	case analyticspb.Attribute_AI_AGENT_FEEDBACK_UNSPECIFIED,
		analyticspb.Attribute_HAS_AI_AGENT_FEEDBACK,
		analyticspb.Attribute_HAS_NO_AI_AGENT_FEEDBACK:
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported AI agent feedback filter: %v", convoDocFilter.AIAgentFeedback)
	}

	convoFilters, excludeFilters := []ConvoFilter{}, []ConvoFilter{}
	if !hasConversationIDConvoFilter(convoDocFilter) {
		convoFilters = append(convoFilters, convertConvoSourcesToConvoFilter(extractConvoSourcesFromConversationFilterParams(convoDocFilter)))
		if len(convoDocFilter.ConversationChannels) > 0 {
			convoFilters = append(convoFilters, convertConvoChannelsToConvoFilter(convoDocFilter.ConversationChannels))
		}
	}

	// API request can override this env flag by passing a bool.
	if *enableExcludingConsultationConvo {
		excludeFilters = append(excludeFilters, convoFilterFromExcludingConsultConvo())
	}

	if len(convoDocFilter.ConversationDurationSecsRanges) > 0 {
		durationRanges := convoDocFilter.ConversationDurationSecsRanges
		if len(durationRanges) == 1 {
			convoFilters = append(convoFilters, ConvoFilter{
				Range: &RangeFilter{
					ConversationDurationSec: durationRanges[0],
				},
			})
		} else {
			should := make([]ConvoFilterBoolFilter, 0, len(durationRanges))
			for _, durationRange := range durationRanges {
				should = append(should, ConvoFilterBoolFilter{
					Range: &RangeFilter{
						ConversationDurationSec: durationRange,
					},
				})
			}
			convoFilters = append(convoFilters, ConvoFilter{
				Bool: &ConvoFilterBool{
					Should:             &should,
					MinimumShouldMatch: 1,
				},
			})
		}
	} else if convoDocFilter.ConversationDurationSecs != nil {
		convoFilters = append(convoFilters, ConvoFilter{
			Range: &RangeFilter{
				ConversationDurationSec: convoDocFilter.ConversationDurationSecs,
			},
		})
	}

	if convoDocFilter.ConversationHandleTimeSecs != nil {
		convoFilters = append(convoFilters, ConvoFilter{
			Range: &RangeFilter{
				ConversationHandleTimeSec: convoDocFilter.ConversationHandleTimeSecs,
			},
		})
	}

	if convoDocFilter.MessageCountRange != nil {
		convoFilters = append(convoFilters, ConvoFilter{
			Range: &RangeFilter{
				ConversationMessageCount: convoDocFilter.MessageCountRange,
			},
		})
	}

	// TODO: Support IncludeDevUser.
	if len(convoDocFilter.AgentUserIDs) > 0 {
		if *esQueryAttendeesForAgentUserIDs {
			convoFilters = append(convoFilters, ConvoFilter{
				Bool: &ConvoFilterBool{
					Should: &[]ConvoFilterBoolFilter{
						{
							Terms: &TermsFilter{AgentUserIDs: &convoDocFilter.AgentUserIDs},
						},
						{
							Nested: &Nested{
								Path: attendeesPath,
								Query: &NestedFilter{
									Bool: &NestedFilterBool{
										Filter: &[]NestedFilter{
											{
												Terms: &NestedTermsFilter{
													ParticipantRoles: &[]string{conversation.DBSpeakerRoleAgent},
												},
											},
											{
												Terms: &NestedTermsFilter{
													ParticipantPlatformIDs: &convoDocFilter.AgentUserIDs,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			})
		} else {
			convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{AgentUserIDs: &convoDocFilter.AgentUserIDs}})
		}
	}
	attendeesFilters := []NestedFilter{}
	// Support Filter with Platform VisitorID
	if len(convoDocFilter.PlatformVisitorID) > 0 {

		attendeesFilters = append(attendeesFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ParticipantRoles: &[]string{conversation.DBSpeakerRoleVisitor},
			},
		})
		attendeesFilters = append(attendeesFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ParticipantPlatformIDs: &convoDocFilter.PlatformVisitorID,
			},
		})
	}
	if len(attendeesFilters) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{
			Nested: &Nested{
				Path: attendeesPath,
				Query: &NestedFilter{
					Bool: &NestedFilterBool{
						Filter: &attendeesFilters,
					},
				},
			},
		})
	}

	if tagIDs := convoDocFilter.ConversationTagIDs; len(tagIDs) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{ConversationTagIDs: &tagIDs}})
	}
	// Filter screen recording present.
	if convoDocFilter.ScreenRecording == analyticspb.Attribute_HAS_SCREEN_RECORDING {
		convoFilters = append(convoFilters, ConvoFilter{
			Bool: &ConvoFilterBool{
				Should: &[]ConvoFilterBoolFilter{
					{
						Bool: &ConvoFilterBoolFilterBool{
							Should: &[]ConvoFilterBoolFilterBoolFilter{
								{
									Exists: &Field{
										Field: "teleport_session_ids",
									},
								},
							},
						},
					},
				},
			},
		})
	}
	// Filter screen recording not present.
	if convoDocFilter.ScreenRecording == analyticspb.Attribute_HAS_NO_SCREEN_RECORDING {
		convoFilters = append(convoFilters, ConvoFilter{
			Bool: &ConvoFilterBool{
				Should: &[]ConvoFilterBoolFilter{
					{
						Bool: &ConvoFilterBoolFilterBool{
							MustNot: &ConvoFilterBoolFilterBoolFilter{
								Exists: &Field{
									Field: "teleport_session_ids",
								},
							},
						},
					},
				},
			},
		})
	}

	if len(convoDocFilter.AccessControlAgentUserIDs) > 0 || len(convoDocFilter.AccessControlGroupIDs) > 0 {
		should := []ConvoFilterBoolFilter{}
		if len(convoDocFilter.AccessControlAgentUserIDs) > 0 {
			should = append(should, ConvoFilterBoolFilter{
				Terms: &TermsFilter{
					AgentUserIDs: &convoDocFilter.AccessControlAgentUserIDs,
				},
			})
		}
		if len(convoDocFilter.AccessControlGroupIDs) > 0 {
			should = append(should, ConvoFilterBoolFilter{
				Terms: &TermsFilter{
					TeamGroupIDs: &convoDocFilter.AccessControlGroupIDs,
				},
			})
		}
		convoFilters = append(convoFilters, ConvoFilter{Bool: &ConvoFilterBool{
			Should: &should,
		}})
	}

	if len(convoDocFilter.ConversationIDs) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{ConversationIDs: &convoDocFilter.ConversationIDs}})
	}

	if len(convoDocFilter.PlatformConversationIDs) > 0 {
		should := []ConvoFilterBoolFilter{}
		for _, platformConversationID := range convoDocFilter.PlatformConversationIDs {
			// if platform conversation id contains wildcard, support wildcard query
			if strings.Contains(platformConversationID, "*") {
				should = append(should, ConvoFilterBoolFilter{
					Wildcard: &WildcardFilter{PlatformConversationID: &ValueField{Value: platformConversationID, CaseInsensitive: proto.Bool(true)}},
				})
				should = append(should, ConvoFilterBoolFilter{
					Wildcard: &WildcardFilter{ConversationCorrelationID: &ValueField{Value: platformConversationID, CaseInsensitive: proto.Bool(true)}},
				})
			} else {
				should = append(should, ConvoFilterBoolFilter{
					Terms: &TermsFilter{PlatformConversationIDs: &[]string{platformConversationID}},
				})
				should = append(should, ConvoFilterBoolFilter{
					Terms: &TermsFilter{ConversationCorrelationIDs: &[]string{platformConversationID}},
				})
			}
		}
		convoFilters = append(convoFilters, ConvoFilter{
			Bool: &ConvoFilterBool{
				Should: &should,
			},
		})
	}

	if len(convoDocFilter.UsecaseIDs) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{UsecaseIDs: &convoDocFilter.UsecaseIDs}})
	}

	if len(convoDocFilter.LanguageCodes) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{LangCodes: &convoDocFilter.LanguageCodes}})
	}

	if len(convoDocFilter.VirtualAgentRevisionIDs) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{VirtualAgentRevisionIDs: &convoDocFilter.VirtualAgentRevisionIDs}})
	}

	if len(convoDocFilter.IngestedByJobIDs) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{IngestedByJobIDs: &convoDocFilter.IngestedByJobIDs}})
	}

	if len(convoDocFilter.DisplayNames) != 0 {
		displayNameFilters, err := parseDisplayNameExpression(convoDocFilter.DisplayNames[0])
		if err != nil {
			return nil, nil, err
		}
		convoFilters = append(convoFilters, displayNameFilters...)
	}

	if convoDocFilter.Comments == analyticspb.Attribute_HAS_COMMENTS {
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{HasComment: &[]bool{true}}})
	}

	// AI agent feedback comments live in the nested `comments` field on the
	// conversation ES doc.
	switch convoDocFilter.AIAgentFeedback {
	case analyticspb.Attribute_HAS_AI_AGENT_FEEDBACK:
		convoFilters = append(convoFilters, aiAgentFeedbackNestedFilter())
	case analyticspb.Attribute_HAS_NO_AI_AGENT_FEEDBACK:
		excludeFilters = append(excludeFilters, aiAgentFeedbackNestedFilter())
	}

	switch convoDocFilter.EmailAttachments {
	case analyticspb.Attribute_HAS_EMAIL_ATTACHMENTS:
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{HasEmailAttachments: &[]bool{true}}})
	case analyticspb.Attribute_HAS_NO_EMAIL_ATTACHMENTS:
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{HasEmailAttachments: &[]bool{false}}})
	}

	// Scorecard status filters
	if len(convoDocFilter.ScorecardStatusFilters) > 0 {
		scorecardFilters := []ConvoFilter{}
		for _, filter := range convoDocFilter.ScorecardStatusFilters {
			switch {
			case len(filter.ScorecardStatuses) > 0:
				normalScorecardFilters := ConvertNormalScorecardStatusesToConvoFilter(
					filter.ScorecardStatuses, convoDocFilter.ScorecardTemplateIDs)
				if normalScorecardFilters != nil {
					scorecardFilters = append(scorecardFilters, *normalScorecardFilters)
				}
			case len(filter.AppealScorecardStatuses) > 0:
				appealScorecardFilters := ConvertAppealScorecardStatusesToConvoFilter(
					filter.AppealScorecardStatuses, convoDocFilter.ScorecardTemplateIDs)
				if appealScorecardFilters != nil {
					scorecardFilters = append(scorecardFilters, *appealScorecardFilters)
				}
			case len(filter.CalibrationScorecardStatuses) > 0:
				calibrationStatuses := []analyticspb.Attribute_ScorecardStatus{}
				for _, status := range filter.CalibrationScorecardStatuses {
					switch status {
					case analyticspb.Attribute_CALIBRATION_SCORECARD_STATUS_UNSPECIFIED:
						continue
					case analyticspb.Attribute_CALIBRATION_SCORECARD_STATUS_SUBMITTED:
						calibrationStatuses = append(calibrationStatuses, analyticspb.Attribute_MANUALLY_SUBMITTED)
					case analyticspb.Attribute_CALIBRATION_SCORECARD_STATUS_DRAFT:
						calibrationStatuses = append(calibrationStatuses, analyticspb.Attribute_DRAFT)
					}
				}
				calibrationScorecardFilters := ConvertCalibrationScorecardStatusesToConvoFilter(
					calibrationStatuses, convoDocFilter.ScorecardTemplateIDs)
				if calibrationScorecardFilters != nil {
					scorecardFilters = append(scorecardFilters, *calibrationScorecardFilters)
				}
			case len(filter.PublishedScorecardStatuses) > 0:
				publishedScorecardFilters := ConvertPublishedScorecardStatusesToConvoFilter(
					filter.PublishedScorecardStatuses, convoDocFilter.ScorecardTemplateIDs)
				if publishedScorecardFilters != nil {
					scorecardFilters = append(scorecardFilters, *publishedScorecardFilters)
				}
			default:
				return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported scorecard status filter: %+v", filter)
			}
		}

		// If we have multiple scorecard filters, combine them with OR logic
		if len(scorecardFilters) > 1 {
			scorecardBoolFilters := []ConvoFilterBoolFilter{}
			for _, filter := range scorecardFilters {
				boolFilter := ConvoFilterBoolFilter{}
				if filter.Nested != nil {
					boolFilter.Nested = filter.Nested
				}
				scorecardBoolFilters = append(scorecardBoolFilters, boolFilter)
			}
			convoFilters = append(convoFilters, ConvoFilter{
				Bool: &ConvoFilterBool{
					Should:             &scorecardBoolFilters,
					MinimumShouldMatch: 1,
				},
			})
		} else if len(scorecardFilters) == 1 {
			convoFilters = append(convoFilters, scorecardFilters[0])
		}
	} else {
		normalScorecardFilters := ConvertNormalScorecardStatusesToConvoFilter(
			convoDocFilter.ScorecardStatuses, convoDocFilter.ScorecardTemplateIDs)
		if normalScorecardFilters != nil {
			convoFilters = append(convoFilters, *normalScorecardFilters)
		}

		calibrationScorecardFilter := ConvertCalibrationScorecardStatusesToConvoFilter(
			convoDocFilter.CalibrationStatuses, convoDocFilter.ScorecardTemplateIDs)
		if calibrationScorecardFilter != nil {
			convoFilters = append(convoFilters, *calibrationScorecardFilter)
		}
	}

	if len(convoDocFilter.ScorecardFilters) > 0 {
		filterOriginalScorecard := NestedFilter{
			Bool: &NestedFilterBool{
				MustNot: &[]NestedFilter{
					{
						Exists: &Field{
							Field: "scorecards.calibrated_scorecard_id",
						},
					},
					{
						Exists: &Field{
							Field: "scorecards.scorecard_type",
						},
					},
				},
			},
		}
		for _, filter := range convoDocFilter.ScorecardFilters {
			criterionFilters := []NestedFilter{}
			for _, criterion := range filter.CriterionFilters {
				var valueFilters []NestedFilter
				numericValues := []float32{}
				for _, score := range criterion.Scores {
					if score.NotApplicable {
						valueFilters = append(valueFilters, NestedFilter{
							Terms: &NestedTermsFilter{
								ScorecardScoresNotApplicable: &[]bool{true},
							},
						})
					} else {
						numericValues = append(numericValues, score.NumericValue)
					}
				}
				// filter by any numeric value or not applicable
				if len(numericValues) > 0 {
					valueFilters = append(valueFilters, NestedFilter{
						Terms: &NestedTermsFilter{
							ScorecardScoresNumericValues: &numericValues,
						},
					})
				}
				criterionFilter := NestedFilter{
					Nested: &Nested{
						Path: scorecardsScoresPath,
						Query: &NestedFilter{
							Bool: &NestedFilterBool{
								Filter: &[]NestedFilter{
									// filter by criterion id and either numeric value or not applicable
									{
										Terms: &NestedTermsFilter{
											ScorecardScoresCriterionIDs: &[]string{criterion.CriterionId},
										},
									},
								},
							},
						},
					},
				}
				if len(valueFilters) > 0 {
					*criterionFilter.Nested.Query.Bool.Filter = append(*criterionFilter.Nested.Query.Bool.Filter, NestedFilter{
						Bool: &NestedFilterBool{
							Should: &valueFilters,
						},
					})
				}
				criterionFilters = append(criterionFilters, criterionFilter)
			}
			templateFilter := NestedFilter{
				// filter by template name and its criteria
				Bool: &NestedFilterBool{
					Filter: &[]NestedFilter{
						// filter by criterion id and either numeric value or not applicable
						{
							Terms: &NestedTermsFilter{
								ScorecardTemplateIDs: &[]string{filter.ScorecardTemplateName},
							},
						},
					},
				},
			}
			if len(criterionFilters) > 0 {
				*templateFilter.Bool.Filter = append(*templateFilter.Bool.Filter, NestedFilter{
					Bool: &NestedFilterBool{
						Filter: &criterionFilters,
					},
				})
			}
			convoFilters = append(convoFilters, ConvoFilter{
				Nested: &Nested{
					Path: ScorecardsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							Filter: &[]NestedFilter{
								templateFilter,
								filterOriginalScorecard,
							},
						},
					},
				},
			})
		}
	}

	if len(convoDocFilter.ScorecardSubmitterUserIDs) > 0 {
		convoFilters = append(convoFilters, ConvoFilter{
			Nested: &Nested{
				Path: ScorecardsPath,
				Query: &NestedFilter{
					Bool: &NestedFilterBool{
						Filter: &[]NestedFilter{
							{
								Terms: &NestedTermsFilter{
									SubmitterUserIDs: &convoDocFilter.ScorecardSubmitterUserIDs,
								},
							},
						},
					},
				},
			},
		})
	}

	if liveAssists := convoDocFilter.LiveAssists; len(liveAssists) > 0 {
		f, err := convertLiveAssistToConvoFilter(liveAssists)
		if err != nil {
			return nil, nil, err
		}
		convoFilters = append(convoFilters, f)
	}

	if len(convoDocFilter.WorkflowAutomationConditions) > 0 {
		usages := make([]int32, 0, len(convoDocFilter.WorkflowAutomationConditions))
		for _, usage := range convoDocFilter.WorkflowAutomationConditions {
			switch usage {
			case analyticspb.Attribute_AUTOMATIC_FAILED,
				analyticspb.Attribute_AUTOMATIC_COMPLETED,
				analyticspb.Attribute_AGENT_INITIATED_FAILED,
				analyticspb.Attribute_AGENT_INITIATED_COMPLETED,
				analyticspb.Attribute_AUTO_DETECTED_IGNORED,
				analyticspb.Attribute_AUTO_DETECTED_FAILED,
				analyticspb.Attribute_AUTO_DETECTED_COMPLETED:
				usages = append(usages, int32(usage))
			default:
				return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported workflow automation usage: %v", usage)
			}
		}
		convoFilters = append(convoFilters, ConvoFilter{Terms: &TermsFilter{WorkflowAutomationConditions: &usages}})
	}

	if l := len(convoDocFilter.SummaryUsages); l > 0 {
		if l > 1 {
			return nil, nil, status.Errorf(codes.InvalidArgument, "summary_usages can at most contain one element")
		}
		switch val := convoDocFilter.SummaryUsages[0]; val {
		case analyticspb.Attribute_SUMMARY_USED:
			f := ConvoFilter{
				Nested: &Nested{
					Path: conversationEventsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							Filter: &[]NestedFilter{
								{
									Terms: &NestedTermsFilter{
										ConversationEventTypes: &[]int{int(dbeventpb.Event_SUMMARIZATION_FEEDBACK)},
									},
								},
							},
						},
					},
				},
			}
			convoFilters = append(convoFilters, f)
		case analyticspb.Attribute_SUMMARY_NOT_USED:
			f := ConvoFilter{
				Nested: &Nested{
					Path: conversationEventsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							MustNot: &[]NestedFilter{
								{
									Terms: &NestedTermsFilter{
										ConversationEventTypes: &[]int{int(dbeventpb.Event_SUMMARIZATION_FEEDBACK)},
									},
								},
							},
						},
					},
				},
			}
			convoFilters = append(convoFilters, f)
		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported SummaryUsage value: %v", val.String())
		}
	}

	if l := len(convoDocFilter.SuggestionUsages); l > 0 {
		if l > 1 {
			return nil, nil, status.Errorf(codes.InvalidArgument, "suggestion_usages can at most contain one element")
		}
		switch val := convoDocFilter.SuggestionUsages[0]; val {
		case analyticspb.Attribute_SUGGESTION_USED:
			f := ConvoFilter{
				Nested: &Nested{
					Path: conversationEventsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							Filter: &[]NestedFilter{
								{
									Terms: &NestedTermsFilter{
										ConversationEventTypes: &[]int{int(dbeventpb.Event_USED_SUGGESTION), int(dbeventpb.Event_USED_HINT_SUGGESTION)},
									},
								},
							},
						},
					},
				},
			}
			convoFilters = append(convoFilters, f)
		case analyticspb.Attribute_SUGGESTION_NOT_USED:
			f := ConvoFilter{
				Nested: &Nested{
					Path: conversationEventsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							MustNot: &[]NestedFilter{
								{
									Terms: &NestedTermsFilter{
										ConversationEventTypes: &[]int{int(dbeventpb.Event_USED_SUGGESTION), int(dbeventpb.Event_USED_HINT_SUGGESTION)},
									},
								},
							},
						},
					},
				},
			}
			convoFilters = append(convoFilters, f)
		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported SuggestionUsage value: %v", val.String())
		}
	}

	if l := len(convoDocFilter.NoteTakingUsages); l > 0 {
		if l > 1 {
			return nil, nil, status.Errorf(codes.InvalidArgument, "note_taking_usage can at most contain one element")
		}
		switch val := convoDocFilter.NoteTakingUsages[0]; val {
		case analyticspb.Attribute_NOTE_TAKING_USED:
			f := ConvoFilter{
				Nested: &Nested{
					Path: conversationEventsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							Filter: &[]NestedFilter{
								{
									Terms: &NestedTermsFilter{
										ConversationEventTypes: &[]int{int(dbeventpb.Event_USED_NOTE_TAKING)},
									},
								},
							},
						},
					},
				},
			}
			convoFilters = append(convoFilters, f)
		case analyticspb.Attribute_NOTE_TAKING_NOT_USED:
			f := ConvoFilter{
				Nested: &Nested{
					Path: conversationEventsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							MustNot: &[]NestedFilter{
								{
									Terms: &NestedTermsFilter{
										ConversationEventTypes: &[]int{int(dbeventpb.Event_USED_NOTE_TAKING)},
									},
								},
							},
						},
					},
				},
			}
			convoFilters = append(convoFilters, f)
		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported NoteTakingUsage value: %v", val.String())
		}
	}

	if len(convoDocFilter.GWDocumentIDs) > 0 {
		nestedFilters := []NestedFilter{
			{
				Terms: &NestedTermsFilter{
					ConversationEventTypes: &[]int{int(dbeventpb.Event_USED_KB_DOCUMENT)},
				},
			},
			{
				Terms: &NestedTermsFilter{
					ConversationEventKBDocIDs: &convoDocFilter.GWDocumentIDs,
				},
			},
		}
		convoFilters = append(convoFilters, ConvoFilter{
			Nested: &Nested{
				Path: conversationEventsPath,
				Query: &NestedFilter{
					Bool: &NestedFilterBool{
						Filter: &nestedFilters,
					},
				},
			},
		})
	}

	if len(convoDocFilter.KBDocumentIDs) > 0 {
		nestedFilters := []NestedFilter{
			{
				Terms: &NestedTermsFilter{
					ConversationEventTypes: &[]int{int(dbeventpb.Event_USED_KB_DOCUMENT)},
				},
			},
			{
				Terms: &NestedTermsFilter{
					ConversationEventKBDocIDs: &convoDocFilter.KBDocumentIDs,
				},
			},
		}
		convoFilters = append(convoFilters, ConvoFilter{
			Nested: &Nested{
				Path: conversationEventsPath,
				Query: &NestedFilter{
					Bool: &NestedFilterBool{
						Filter: &nestedFilters,
					},
				},
			},
		})
	}

	for _, lt := range convoDocFilter.LanguageTransfers {
		nestedFilters := []NestedFilter{
			{
				Terms: &NestedTermsFilter{
					ConversationEventTypes: &[]int{int(dbeventpb.Event_CONVERSATION_LANGUAGE_CHANGE)},
				},
			},
			{
				Terms: &NestedTermsFilter{
					ConversationEventOldLangCodes: &[]string{lt.OldLanguageCode},
				},
			},
			{
				Terms: &NestedTermsFilter{
					ConversationEventNewLangCodes: &[]string{lt.NewLanguageCode},
				},
			},
		}
		convoFilters = append(convoFilters, ConvoFilter{
			Nested: &Nested{
				Path: conversationEventsPath,
				Query: &NestedFilter{
					Bool: &NestedFilterBool{
						Filter: &nestedFilters,
					},
				},
			},
		})
	}
	if crit := convoDocFilter.ConversationOnHoldCriteria; crit != nil {
		var f ConvoFilter
		var err error
		if enableNumHoldsField {
			f, err = holdsCountRangeFilter(crit)
		} else {
			f, err = functionScoreForOnHoldCount(crit)
		}
		if err != nil {
			return nil, nil, err
		}
		convoFilters = append(convoFilters, f)
	}
	if transferInfo := convoDocFilter.TransferInfo; transferInfo != nil {
		if transferTypes := transferInfo.UmbrellaConversationTransferTypes; len(transferTypes) > 0 {
			vals, err := convertUmbrellaConversationTransferType(transferTypes)
			if err != nil {
				return nil, nil, err
			}
			convoFilters = append(convoFilters, ConvoFilter{
				Terms: &TermsFilter{
					TransferInfoUmbrellaConversationTransferTypes: &vals,
				},
			})
		}
		if interactionTypes := transferInfo.ParticipantInteractionTypes; len(interactionTypes) > 0 {
			vals, err := convertParticipantInteractionType(interactionTypes)
			if err != nil {
				return nil, nil, err
			}
			convoFilters = append(convoFilters, ConvoFilter{
				Terms: &TermsFilter{
					TransferInfoParticipantInteractionTypes: &vals,
				},
			})
		}
	}

	for _, mg := range convoDocFilter.MomentGroups {
		inclFilter, exclFilter, err := convertConvoMomentGroupToConvoFilters(mg)
		if err != nil {
			return convoFilters, excludeFilters, err
		}
		if inclFilter != nil {
			convoFilters = append(convoFilters, *inclFilter)
		}
		if exclFilter != nil {
			excludeFilters = append(excludeFilters, *exclFilter)
		}
	}

	if convoDocFilter.ChildMessageFilter == nil {
		return convoFilters, excludeFilters, nil
	}

	childMsgFilter := convoDocFilter.ChildMessageFilter
	// Spearker role is always applied, so compute it at the beginning.
	var speakerRoleFilter *MsgFilter
	if len(childMsgFilter.SpeakerRoles) != 0 {
		esSpeakerRoles, err := shared.ConvertAPIToESSpeakerRoles(childMsgFilter.SpeakerRoles)
		if err != nil {
			return nil, nil, err
		}
		speakerRoleFilter = &MsgFilter{Terms: &TermsFilter{SpeakerRoles: &esSpeakerRoles}}
	}

	for _, momentGroup := range childMsgFilter.MomentGroups {
		if len(momentGroup.Moments) > 0 || momentGroup.MomentType != momentpb.Moment_TYPE_UNSPECIFIED || len(momentGroup.Behaviors) > 0 {
			filters, err := convertMessageMomentsToConvoFilters(
				momentGroup.Moments, momentGroup.Behaviors, momentGroup.MomentType, momentGroup.Adherence, momentGroup.AdherenceTypesForMoments, speakerRoleFilter, topicDetails, childMsgFilter.LanguageCodes)
			if err != nil {
				return nil, nil, err
			}
			convoFilters = append(convoFilters, filters...)
		}
		if len(momentGroup.ExcludedMoments) > 0 {
			filters, err := convertMessageMomentsToConvoFilters(
				momentGroup.ExcludedMoments, momentGroup.Behaviors, momentpb.Moment_TYPE_UNSPECIFIED, momentGroup.Adherence, momentGroup.AdherenceTypesForMoments, speakerRoleFilter, topicDetails, childMsgFilter.LanguageCodes)
			if err != nil {
				return nil, nil, err
			}
			excludeFilters = append(excludeFilters, filters...)
		}
	}
	for _, ag := range childMsgFilter.ActionGroups {
		f, err := convertActionGroupToConvoFilter(ag)
		if err != nil {
			return nil, nil, err
		}
		convoFilters = append(convoFilters, f)
	}

	if len(childMsgFilter.KeywordsExpressions) != 0 {
		// Conversation level keyword is used to search message documents, not message_group documents.
		convoFilter := ConvoFilter{
			HasChild: &HasChild{
				Type: string(DocumentTypeMessage),
				Query: InnerMsgQuery{
					Bool: &InnerMsgBool{},
				},
			},
		}
		msgFilters := []MsgFilter{}
		if speakerRoleFilter != nil {
			msgFilters = append(msgFilters, *speakerRoleFilter)
		}
		if len(msgFilters) > 0 {
			convoFilter.HasChild.Query.Bool.Filter = &msgFilters
		}

		// convoDocFilter.LanguageCodes is applied to child message keyword filter to maintain existing behavior.
		keywordMust, keywordShould, err := parseSearchKeyword(
			childMsgFilter.KeywordsExpressions[0], convoDocFilter.LanguageCodes)
		if err != nil {
			return nil, nil, err
		}
		if len(keywordMust) > 0 {
			convoFilter.HasChild.Query.Bool.Must = &keywordMust
		}
		if len(keywordShould) > 0 {
			convoFilter.HasChild.Query.Bool.Should = &keywordShould
			convoFilter.HasChild.Query.Bool.MinimumShouldMatch = minShouldMatch
		}
		convoFilters = append(convoFilters, convoFilter)
	}

	if ka := childMsgFilter.KnowledgeAssist; ka != nil {
		momentType := int(dbmomentpb.Moment_MOMENT_TYPE_KNOWLEDGE)
		fieldPathForRetrievalArticleAA := "moment_annotations.payload.knowledgePayload.associatedActionAnnotations.retrievalArticleActionAnnotationIds"
		fieldPathForRetrievalQuestionAA := "moment_annotations.payload.knowledgePayload.associatedActionAnnotations.retrievalQuestionActionAnnotationIds"
		fieldPathForKaGenAiReplyUsedAA := "moment_annotations.payload.knowledgePayload.associatedMomentAnnotations.kaGenAiReplyUsedMomentAnnotationIds"

		shouldFilters := []ConvoFilterBoolFilter{}
		for _, km := range ka.KnowledgeMoments {
			filters := []NestedFilter{
				{
					Terms: &NestedTermsFilter{
						MomentTypes: &[]int{momentType},
					},
				},
			}
			switch v := km.Type.(type) {
			case *analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_:
				filters = append(filters, NestedFilter{
					Terms: &NestedTermsFilter{
						KnowledgeMomentTypes: &[]string{momentpb.KnowledgePayload_AGENT_INITIATED.String()},
					},
				})
				if status := v.AgentInitiated.Status; status == analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_STATUS_VIEWED {
					filters = append(filters, NestedFilter{
						Exists: &Field{
							Field: fieldPathForRetrievalArticleAA,
						},
					})
				} else if status == analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_STATUS_USED {
					filters = append(filters, NestedFilter{
						Exists: &Field{
							Field: fieldPathForRetrievalArticleAA,
						},
					})
					filters = append(filters, NestedFilter{
						Exists: &Field{
							Field: fieldPathForKaGenAiReplyUsedAA,
						},
					})
				}
			case *analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_:
				filters = append(filters, NestedFilter{
					Terms: &NestedTermsFilter{
						KnowledgeMomentTypes: &[]string{momentpb.KnowledgePayload_AUTO_DETECTED.String()},
					},
				})
				filters = append(filters, NestedFilter{
					Exists: &Field{
						Field: fieldPathForRetrievalQuestionAA,
					},
				})
				if status := v.AutoDetected.Status; status == analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_IGNORED {
					filters = append(filters, NestedFilter{
						Bool: &NestedFilterBool{
							MustNot: &[]NestedFilter{{
								Exists: &Field{
									Field: fieldPathForRetrievalArticleAA,
								},
							}},
						},
					})
				} else if status == analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_VIEWED {
					filters = append(filters, NestedFilter{
						Exists: &Field{
							Field: fieldPathForRetrievalArticleAA,
						},
					})
				} else if status == analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_USED {
					filters = append(filters, NestedFilter{
						Exists: &Field{
							Field: fieldPathForRetrievalArticleAA,
						},
					})
					filters = append(filters, NestedFilter{
						Exists: &Field{
							Field: fieldPathForKaGenAiReplyUsedAA,
						},
					})
				}
			}

			hasChild := &HasChild{
				Type: string(DocumentTypeMessage),
				Query: InnerMsgQuery{
					Bool: &InnerMsgBool{
						Filter: &[]MsgFilter{
							{
								Nested: nestedFromNestedFilters(&filters, MomentAnnotationsPath),
							},
						},
					},
				},
			}

			if ka.MatchType == analyticspb.MatchType_MATCH_ALL {
				convoFilters = append(convoFilters, ConvoFilter{HasChild: hasChild})
			} else {
				shouldFilters = append(shouldFilters, ConvoFilterBoolFilter{HasChild: hasChild})
			}
		}

		if ka.MatchType != analyticspb.MatchType_MATCH_ALL && len(shouldFilters) > 0 {
			convoFilters = append(convoFilters, ConvoFilter{
				Bool: &ConvoFilterBool{
					Should:             &shouldFilters,
					MinimumShouldMatch: 1,
				},
			})
		}
	}

	return convoFilters, excludeFilters, nil
}

func convertUmbrellaConversationTransferType(input []analyticspb.Attribute_TransferInfoFilter_UmbrellaConversationTransferType) ([]int32, error) {
	ret := []int32{}
	for _, val := range input {
		switch val {
		case analyticspb.Attribute_TransferInfoFilter_COLD_TRANSFER:
			ret = append(ret, int32(dbconvopb.ConversationMetadata_COLD_TRANSFER))
		case analyticspb.Attribute_TransferInfoFilter_WARM_TRANSFER:
			ret = append(ret, int32(dbconvopb.ConversationMetadata_WARM_TRANSFER))
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported umbrella_conversation_transfer_type %v", val)
		}
	}
	return ret, nil
}

func hasConversationIDConvoFilter(f *ConversationFilterParams) bool {
	return f != nil && (len(f.ConversationIDs) > 0 || len(f.PlatformConversationIDs) > 0)
}

func hasConversationIDMsgFilter(f *MessageFilterParams) bool {
	return f != nil && (len(f.ConversationIDs) > 0 || len(f.PlatformConversationIDs) > 0)
}

func convertParticipantInteractionType(input []analyticspb.Attribute_TransferInfoFilter_ParticipantInteractionType) ([]int32, error) {
	ret := []int32{}
	for _, val := range input {
		switch val {
		case analyticspb.Attribute_TransferInfoFilter_CONSULTATION:
			ret = append(ret, int32(dbconvopb.ConversationMetadata_CONSULTATION))
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported participant_interaction_type %v", val)
		}
	}
	return ret, nil
}

func convertKeywordMomentConstraintToFilter(c *KeywordMomentConstraint) []KeywordsIncludedFilter {
	if c == nil {
		return nil
	}
	f := []KeywordsIncludedFilter{}
	if c.MessageForwardPostionUpperBound != nil {
		f = append(f, KeywordsIncludedFilter{
			MessagePositionFilter: &RangeFilter{
				FowardPositionRange: &IntRange{
					LTE: c.MessageForwardPostionUpperBound,
				},
			},
		})
	}
	if c.MessageBackwardPostionUpperBound != nil {
		f = append(f, KeywordsIncludedFilter{
			MessagePositionFilter: &RangeFilter{
				BackwardPostionRange: &IntRange{
					LTE: c.MessageBackwardPostionUpperBound,
				},
			},
		})
	}
	return f
}

func SplitTopicBoolToMsgAndMsgGroup(t *TopicBool) (*TopicBool, *TopicBool) {
	if t == nil {
		return nil, nil
	}
	shouldForMsg, shouldForMessageGroup := []KeywordsIncludedOrClustersShould{}, []KeywordsIncludedOrClustersShould{}
	if t.Should != nil {
		for _, s := range *(t.Should) {
			if s.KeywordsToInclude != nil {
				shouldForMsg = append(shouldForMsg, KeywordsIncludedOrClustersShould{
					KeywordsToInclude: s.KeywordsToInclude,
				})
			}
			if s.Nested != nil {
				shouldForMessageGroup = append(shouldForMessageGroup, KeywordsIncludedOrClustersShould{
					Nested: s.Nested,
				})
			}
		}
	}

	// Handle special case where the composite moment only have excluded keyword.
	if len(shouldForMsg) == 0 && len(shouldForMessageGroup) == 0 {
		return &TopicBool{
			Filter:  t.Filter,
			MustNot: t.MustNot,
		}, nil
	}

	var forMessage *TopicBool
	if len(shouldForMsg) > 0 {
		forMessage = &TopicBool{
			Filter:             t.Filter,
			MustNot:            t.MustNot,
			Should:             &shouldForMsg,
			MinimumShouldMatch: 1,
		}
	}

	var forMessageGroup *TopicBool
	if len(shouldForMessageGroup) > 0 {
		forMessageGroup = &TopicBool{
			Filter:             t.Filter,
			MustNot:            t.MustNot,
			Should:             &shouldForMessageGroup,
			MinimumShouldMatch: 1,
		}
	}
	return forMessage, forMessageGroup
}

func ConvertTopicDetailToTopicBool(t TopicDetails, langCodes []string) (*TopicBool, error) {
	effectiveLangCodes := langCodes
	if len(langCodes) > 0 {
		effectiveLangCodes = append(slices.Clone(langCodes), t.LanguageCode)
	} else if t.LanguageCode != "" && t.LanguageCode != consts.AllLanguages {
		effectiveLangCodes = []string{t.LanguageCode}
	}

	ret := TopicBool{}

	filters := []TopicBoolFilter{}
	for _, prereqMoment := range t.PrerequrementMoments {
		momentName, err := momentpb.ParseMomentName(prereqMoment)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		filters = append(filters, TopicBoolFilter{Nested: &Nested{
			Path: MomentAnnotationsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &[]NestedFilter{
						{
							Terms: &NestedTermsFilter{MomentTemplateIDs: &[]string{momentName.MomentID}},
						},
					},
				},
			},
		}})
	}
	if len(t.SpeakerRoles) > 0 {
		filters = append(filters, TopicBoolFilter{
			Terms: &TermsFilter{
				SpeakerRoles: &t.SpeakerRoles,
			},
		})
	}
	if t.MessageWindowAsMessageCount != nil {
		filters = append(filters, TopicBoolFilter{
			RangeFilter: &RangeFilter{
				FowardPositionRange: &IntRange{
					LTE: t.MessageWindowAsMessageCount,
				},
			},
		})
	}
	if t.MessageWindownAsDurationInMin != nil {
		filters = append(filters, TopicBoolFilter{
			RangeFilter: &RangeFilter{
				ForwardPositionInSecs: &FloatRange{
					LTE: proto.Float64(float64(*t.MessageWindownAsDurationInMin) * 60),
				},
			},
		})
	}
	if len(filters) > 0 {
		ret.Filter = &filters
	}
	should := []KeywordsIncludedOrClustersShould{}
	if len(t.IncludedPhrases) > 0 {
		keywordsIncluded := &KeywordMomentBool{
			Filters: []KeywordsIncludedFilter{
				{
					TermsFilter: &TermsFilter{
						ConversationToMessageJoin: &[]string{string(DocumentTypeMessage)},
					},
				},
			},
		}
		if t.KeywordMomentConstraint != nil {
			filters := convertKeywordMomentConstraintToFilter(t.KeywordMomentConstraint)
			keywordsIncluded.Filters = append(keywordsIncluded.Filters, filters...)
		}

		filters := []KeywordMomentBoolFilter{}
		for _, in := range t.IncludedPhrases {
			f, err := convertPhraseToKeywordMomentFilter(in, effectiveLangCodes)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
		if t.IncludedPhrasesMatchType == MatchAll {
			keywordsIncluded.Must = &filters
		} else {
			keywordsIncluded.Should = &filters
			keywordsIncluded.MinimumShouldMatch = 1
		}
		should = append(should, KeywordsIncludedOrClustersShould{KeywordsToInclude: keywordsIncluded})
	}

	if t.RegexExpression != "" {
		luceneExpr, ok := prepareLuceneRegex(t.RegexExpression)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "preview unavailable: regex uses anchors or word boundaries (^, $, \\b) which are not supported in search")
		}
		regexIncluded := &KeywordMomentBool{
			Filters: []KeywordsIncludedFilter{
				{
					TermsFilter: &TermsFilter{
						ConversationToMessageJoin: &[]string{string(DocumentTypeMessage)},
					},
				},
			},
			Must: &[]KeywordMomentBoolFilter{
				{Regexp: &RegexpField{MessageTextKeyword: RegexpValue{Value: luceneExpr, CaseInsensitive: true}}},
			},
		}
		should = append(should, KeywordsIncludedOrClustersShould{KeywordsToInclude: regexIncluded})
	}

	momentIDsForClusters := []string{}
	for _, cluster := range t.Clusters {
		momentName, err := momentpb.ParseMomentName(cluster)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		momentIDsForClusters = append(momentIDsForClusters, momentName.MomentID)
	}
	if len(momentIDsForClusters) > 0 {
		should = append(should, KeywordsIncludedOrClustersShould{Nested: &Nested{
			Path: MomentAnnotationsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &[]NestedFilter{
						{
							Terms: &NestedTermsFilter{MomentTemplateIDs: &momentIDsForClusters},
						},
					},
				},
			},
		}})
	}

	if len(should) > 0 {
		ret.Should = &should
		ret.MinimumShouldMatch = 1
	}

	mustNot := []KeywordMomentBoolFilter{}
	for _, ex := range t.ExcludedPhrases {
		f, err := convertPhraseToKeywordMomentFilter(ex, effectiveLangCodes)
		if err != nil {
			return nil, err
		}
		mustNot = append(mustNot, f)
	}
	if len(mustNot) > 0 {
		if t.ExcludedPhrasesMatchType == MatchAny {
			ret.MustNot = &[]KeywordExcludedMustNot{{
				KeywordsToExclude: &KeywordMomentBool{
					Should: &mustNot,
				},
			}}
		} else {
			ret.MustNot = &[]KeywordExcludedMustNot{{
				KeywordsToExclude: &KeywordMomentBool{
					Must: &mustNot,
				},
			}}
		}
	}
	if t.ExcludeRegexExpression != "" {
		luceneExcludeExpr, ok := prepareLuceneRegex(t.ExcludeRegexExpression)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "preview unavailable: exclude regex uses anchors or word boundaries (^, $, \\b) which are not supported in search")
		}
		excludeRegexFilter := KeywordMomentBoolFilter{
			Regexp: &RegexpField{MessageTextKeyword: RegexpValue{Value: luceneExcludeExpr, CaseInsensitive: true}},
		}
		excludeMustNot := KeywordExcludedMustNot{
			KeywordsToExclude: &KeywordMomentBool{
				Must: &[]KeywordMomentBoolFilter{excludeRegexFilter},
			},
		}
		if ret.MustNot == nil {
			ret.MustNot = &[]KeywordExcludedMustNot{excludeMustNot}
		} else {
			*ret.MustNot = append(*ret.MustNot, excludeMustNot)
		}
	}
	return &ret, nil
}

func convertPhraseToKeywordMomentFilter(ph Phrase, langCodes []string) (KeywordMomentBoolFilter, error) {
	mm := KeywordMomentBoolFilter{MultiMatch: &MultiMatchField{
		Query: ph.Text,
		Type:  phraseType,
	}}
	if ph.SkipStemming {
		if shouldApplyWhitespaceAnalyzer(ph.Text) {
			mm.MultiMatch.Fields = []string{whitespaceMsgTextField}
		} else {
			mm.MultiMatch.Fields = []string{defaultMsgTextField}
		}
	} else {
		langCodeMsgTextFields, err := convertLangCodesToMsgTextFields(langCodes)
		if err != nil {
			return KeywordMomentBoolFilter{}, err
		}
		mm.MultiMatch.Fields = langCodeMsgTextFields
		if ph.Flexible {
			mm.MultiMatch.Slop = proto.Int32(maxSlopForFlexiblePhrase)
		}
	}
	return mm, nil
}

func parseSearchKeyword(keywordExpression string, langCodes []string) ([]MsgFilter, []MsgFilter, error) {
	// TODO: add more validation for keywords: possible validations:
	// 1) The keyword only contains alphabetical letters and some special chars like space, single quotes
	// 2) The keyword only contains valid english words.
	andString, orString := " AND ", " OR "
	if strings.Contains(keywordExpression, andString) && strings.Contains(keywordExpression, orString) {
		return nil, nil, fmt.Errorf("does not support %q and %q at the same time", andString, orString)
	}

	var (
		keywords  []string
		matchType MatchPharesType
	)
	switch {
	case strings.Contains(keywordExpression, orString):
		keywords = strings.Split(keywordExpression, orString)
		matchType = MatchAny
	case strings.Contains(keywordExpression, andString):
		keywords = strings.Split(keywordExpression, andString)
		matchType = MatchAll
	default:
		keywords = append(keywords, keywordExpression)
		matchType = MatchAll
	}
	for i := range keywords {
		keywords[i] = strings.Trim(keywords[i], " ")
	}
	langCodeMsgTextFields, err := convertLangCodesToMsgTextFields(langCodes)
	if err != nil {
		return nil, nil, err
	}
	mm := []MsgFilter{}
	for _, keyword := range keywords {
		if isExactKeyword(keyword) {
			keyword = strings.Trim(keyword, "\"")
			msgTextField := defaultMsgTextField
			if shouldApplyWhitespaceAnalyzer(keyword) {
				msgTextField = whitespaceMsgTextField
			}
			mm = append(mm, MsgFilter{
				MultiMatchField: &MultiMatchField{
					Query:  keyword,
					Type:   phraseType,
					Fields: []string{msgTextField},
				},
			})
		} else {
			mm = append(mm, MsgFilter{
				MultiMatchField: &MultiMatchField{
					Query:  strings.Trim(keyword, "'"),
					Type:   phraseType,
					Fields: langCodeMsgTextFields,
				},
			})
		}
	}
	var must, should []MsgFilter
	if matchType == MatchAny {
		should = mm
	} else {
		must = mm
	}
	return must, should, nil
}

func parseDisplayNameExpression(expression string) ([]ConvoFilter, error) {
	andString, orString := " AND ", " OR "
	if strings.Contains(expression, andString) && strings.Contains(expression, orString) {
		return nil, fmt.Errorf("does not support %q and %q at the same time", andString, orString)
	}

	var (
		parts     []string
		matchType MatchPharesType
	)
	switch {
	case strings.Contains(expression, orString):
		parts = strings.Split(expression, orString)
		matchType = MatchAny
	case strings.Contains(expression, andString):
		parts = strings.Split(expression, andString)
		matchType = MatchAll
	default:
		parts = []string{expression}
		matchType = MatchAll
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	filters := make([]ConvoFilterBoolFilter, 0, len(parts))
	for _, part := range parts {
		if isExactKeyword(part) {
			trimmed := strings.Trim(part, "\"")
			filters = append(filters, ConvoFilterBoolFilter{
				Terms: &TermsFilter{DisplayNames: &[]string{trimmed}},
			})
		} else {
			filters = append(filters, ConvoFilterBoolFilter{
				Match: &MatchFilter{
					DisplayName: &MatchFieldQuery{Query: part},
				},
			})
		}
	}

	if matchType == MatchAny {
		return []ConvoFilter{{
			Bool: &ConvoFilterBool{
				Should:             &filters,
				MinimumShouldMatch: minShouldMatch,
			},
		}}, nil
	}

	result := make([]ConvoFilter, 0, len(filters))
	for _, f := range filters {
		result = append(result, ConvoFilter{
			Terms: f.Terms,
			Match: f.Match,
		})
	}
	return result, nil
}

func convertScorecardStatus(statuses []analyticspb.Attribute_ScorecardStatus) []int {
	ret := []int{}
	for _, s := range statuses {
		if s != analyticspb.Attribute_SCORECARD_STATUS_UNSPECIFIED {
			// https://github.com/cresta/go-servers/blob/main/apiserver/shared/elasticsearch/converter.go#L567-L576
			// ES defines:
			// - ScorecardStatusManuallySubmitted = 2 --> submitted scorecard
			// - ScorecardStatusDraft = 1             --> not submitted, manually editeds
			// - ScorecardStatusAuto = 0              --> the others, i.e. not submitted, auto scorecards
			switch s {
			case analyticspb.Attribute_MANUALLY_SUBMITTED:
				ret = append(ret, int(elastic.ScorecardStatusManuallySubmitted))
			case analyticspb.Attribute_DRAFT:
				ret = append(ret, int(elastic.ScorecardStatusDraft))
			case analyticspb.Attribute_AUTO:
				ret = append(ret, int(elastic.ScorecardStatusAuto))
			}
		}
	}
	return ret
}

func ConvertNormalScorecardStatusesToConvoFilter(
	scorecardStatuses []analyticspb.Attribute_ScorecardStatus,
	scorecardTemplateIDs []string,
) *ConvoFilter {
	if len(scorecardStatuses) == 0 {
		return nil
	}

	scorecardFilters := []NestedFilter{}
	convertedStatuses := convertScorecardStatus(scorecardStatuses)
	if len(convertedStatuses) == 0 {
		return nil
	}

	if slices.Contains(convertedStatuses, int(elastic.ScorecardStatusAuto)) {
		// When Scorecard.Status is AUTO, it's omitted in ES documents.
		statusShouldFilters := []NestedFilter{
			{
				Terms: &NestedTermsFilter{
					ScorecardStatuses: &convertedStatuses,
				},
			},
			{
				Bool: &NestedFilterBool{
					MustNot: &[]NestedFilter{
						{
							Exists: &Field{
								Field: ScorecardsPath + ".status",
							},
						},
					},
				},
			},
		}
		scorecardFilters = append(scorecardFilters, NestedFilter{
			Bool: &NestedFilterBool{
				Should:             &statusShouldFilters,
				MinimumShouldMatch: 1,
			},
		})
	} else {
		// When status 0 is not included, use simple terms filter
		scorecardFilters = append(scorecardFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ScorecardStatuses: &convertedStatuses,
			},
		})
	}

	// Add filter to ensure scorecard_type is either 0 or doesn't exist (normal scorecards only)
	scorecardTypeFilters := []NestedFilter{
		{
			Terms: &NestedTermsFilter{
				ScorecardTypes: &[]int{0},
			},
		},
		{
			Bool: &NestedFilterBool{
				MustNot: &[]NestedFilter{
					{
						Exists: &Field{
							Field: ScorecardsPath + ".scorecard_type",
						},
					},
				},
			},
		},
	}
	scorecardFilters = append(scorecardFilters, NestedFilter{
		Bool: &NestedFilterBool{
			Should:             &scorecardTypeFilters,
			MinimumShouldMatch: 1,
		},
	})

	// Add filter to ensure calibrated_scorecard_id doesn't exist (not a calibration scorecard)
	scorecardFilters = append(scorecardFilters, NestedFilter{
		Bool: &NestedFilterBool{
			MustNot: &[]NestedFilter{
				{
					Exists: &Field{
						Field: ScorecardsPath + ".calibrated_scorecard_id",
					},
				},
			},
		},
	})

	if len(scorecardTemplateIDs) > 0 {
		scorecardFilters = append(scorecardFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ScorecardTemplateIDs: &scorecardTemplateIDs,
			},
		})
	}

	if len(scorecardFilters) == 0 {
		return nil
	}

	return &ConvoFilter{
		Nested: &Nested{
			Path: ScorecardsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &scorecardFilters,
				},
			},
		},
	}
}

func ConvertCalibrationScorecardStatusesToConvoFilter(
	calibrationStatuses []analyticspb.Attribute_ScorecardStatus,
	scorecardTemplateIDs []string,
) *ConvoFilter {
	if len(calibrationStatuses) == 0 {
		return nil
	}

	convertedStatuses := convertScorecardStatus(calibrationStatuses)
	if len(convertedStatuses) == 0 {
		return nil
	}

	calibrationScorecardFilters := []NestedFilter{
		{
			Terms: &NestedTermsFilter{
				ScorecardStatuses: &convertedStatuses,
			},
		},
		{
			Exists: &Field{
				Field: ScorecardsPath + ".calibrated_scorecard_id",
			},
		},
	}

	if len(scorecardTemplateIDs) > 0 {
		calibrationScorecardFilters = append(calibrationScorecardFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ScorecardTemplateIDs: &scorecardTemplateIDs,
			},
		})
	}

	return &ConvoFilter{
		Nested: &Nested{
			Path: ScorecardsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &calibrationScorecardFilters,
				},
			},
		},
	}
}

func ConvertAppealScorecardStatusesToConvoFilter(
	appealStatuses []analyticspb.Attribute_AppealScorecardStatus,
	scorecardTemplateIDs []string,
) *ConvoFilter {
	if len(appealStatuses) == 0 {
		return nil
	}

	var scorecardTypes []int
	for _, status := range appealStatuses {
		switch status {
		case analyticspb.Attribute_APPEAL_SCORECARD_STATUS_REQUESTED:
			scorecardTypes = append(scorecardTypes, 2)
		case analyticspb.Attribute_APPEAL_SCORECARD_STATUS_RESOLVED:
			scorecardTypes = append(scorecardTypes, 3)
		}
	}
	if len(scorecardTypes) == 0 {
		return nil
	}

	appealScorecardFilters := []NestedFilter{
		{
			Terms: &NestedTermsFilter{
				ScorecardTypes: &scorecardTypes,
			},
		},
		{
			Terms: &NestedTermsFilter{
				ScorecardStatuses: &[]int{int(elastic.ScorecardStatusManuallySubmitted)},
			},
		},
	}

	if len(scorecardTemplateIDs) > 0 {
		appealScorecardFilters = append(appealScorecardFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ScorecardTemplateIDs: &scorecardTemplateIDs,
			},
		})
	}

	return &ConvoFilter{
		Nested: &Nested{
			Path: ScorecardsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &appealScorecardFilters,
				},
			},
		},
	}
}

func ConvertPublishedScorecardStatusesToConvoFilter(publishedScorecardStatuses []analyticspb.Attribute_PublishedScorecardStatus, scorecardTemplateIDs []string) *ConvoFilter {
	if len(publishedScorecardStatuses) == 0 {
		return nil
	}

	scorecardFilters := []NestedFilter{}

	// Scorecard type is either 0 or doesn't exist (normal scorecards only)
	scorecardTypeFilters := []NestedFilter{
		{
			Terms: &NestedTermsFilter{
				ScorecardTypes: &[]int{0},
			},
		},
		{
			Bool: &NestedFilterBool{
				MustNot: &[]NestedFilter{
					{
						Exists: &Field{
							Field: ScorecardsPath + ".scorecard_type",
						},
					},
				},
			},
		},
	}
	scorecardFilters = append(scorecardFilters, NestedFilter{
		Bool: &NestedFilterBool{
			Should:             &scorecardTypeFilters,
			MinimumShouldMatch: 1,
		},
	})

	publishedScorecardFilters := []NestedFilter{}
	for _, status := range publishedScorecardStatuses {
		switch status {
		case analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_PUBLISHED:
			publishedScorecardFilters = append(publishedScorecardFilters, NestedFilter{
				Exists: &Field{
					Field: ScorecardsPath + ".published_at",
				},
			})
		case analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_NOT_PUBLISHED:
			publishedScorecardFilters = append(publishedScorecardFilters, NestedFilter{
				Bool: &NestedFilterBool{
					MustNot: &[]NestedFilter{
						{
							Exists: &Field{
								Field: ScorecardsPath + ".published_at",
							},
						},
					},
					Filter: &[]NestedFilter{
						{
							Terms: &NestedTermsFilter{
								ScorecardStatuses: &[]int{int(elastic.ScorecardStatusManuallySubmitted)},
							},
						},
					},
				},
			})
		}
	}

	scorecardFilters = append(scorecardFilters, NestedFilter{
		Bool: &NestedFilterBool{
			Should:             &publishedScorecardFilters,
			MinimumShouldMatch: 1,
		},
	})

	if len(scorecardTemplateIDs) > 0 {
		scorecardFilters = append(scorecardFilters, NestedFilter{
			Terms: &NestedTermsFilter{
				ScorecardTemplateIDs: &scorecardTemplateIDs,
			},
		})
	}

	return &ConvoFilter{
		Nested: &Nested{
			Path: ScorecardsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &scorecardFilters,
				},
			},
		},
	}
}

// Raise-hand, manager-whisper and manager-transfer action annotations are all on
// conversations. Values OR together.
func convertLiveAssistToConvoFilter(liveAssists []analyticspb.Attribute_LiveAssist) (ConvoFilter, error) {
	f := []ConvoFilterBoolFilter{}
	for _, la := range liveAssists {
		switch la {
		case analyticspb.Attribute_RAISE_HAND_UNANSWERED:
			// raise_hand_state=active
			filterOnRaiseHandState := ConvoFilterBoolFilterBoolFilter{
				Terms: &TermsFilter{RaiseHandStates: &[]int32{int32(elastic.RaiseHandActive)}},
			}
			f = append(f, ConvoFilterBoolFilter{Bool: &ConvoFilterBoolFilterBool{
				Should:             &[]ConvoFilterBoolFilterBoolFilter{filterOnRaiseHandState},
				MinimumShouldMatch: proto.Int32(1),
			}})
		case analyticspb.Attribute_RAISE_HAND_ANSWERED:
			// raise_hand_state=raise_hand_handled AND raise_hand action_annotation exists
			filterOnRaiseHandState := ConvoFilterBoolFilterBoolFilter{
				Terms: &TermsFilter{RaiseHandStates: &[]int32{int32(elastic.RaiseHandHandled)}},
			}
			filterOnAA := ConvoFilterBoolFilterBoolFilter{
				Nested: &Nested{
					Path: actionAnnotationsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							Filter: &[]NestedFilter{{
								Terms: &NestedTermsFilter{ActionTypes: &[]int{int(dbactionpb.Action_AGENT_RAISE_HAND)}},
							}},
						},
					},
				},
			}
			f = append(f, ConvoFilterBoolFilter{Bool: &ConvoFilterBoolFilterBool{
				Should:             &[]ConvoFilterBoolFilterBoolFilter{filterOnRaiseHandState, filterOnAA},
				MinimumShouldMatch: proto.Int32(2),
			}})
		case analyticspb.Attribute_LIVE_ASSIST_WITHOUT_RAISE_HAND:
			// raise_hand_state=raise_hand_handled AND rase_hand action_annotation NOT exists
			filterOnRaiseHandState := ConvoFilterBoolFilterBoolFilter{
				Terms: &TermsFilter{RaiseHandStates: &[]int32{int32(elastic.RaiseHandHandled)}},
			}
			filterOnAA := ConvoFilterBoolFilterBoolFilter{
				Nested: &Nested{
					Path: actionAnnotationsPath,
					Query: &NestedFilter{
						Bool: &NestedFilterBool{
							Filter: &[]NestedFilter{{
								Terms: &NestedTermsFilter{ActionTypes: &[]int{int(dbactionpb.Action_AGENT_RAISE_HAND)}},
							}},
						},
					},
				},
			}
			f = append(f, ConvoFilterBoolFilter{Bool: &ConvoFilterBoolFilterBool{
				Should:             &[]ConvoFilterBoolFilterBoolFilter{filterOnRaiseHandState},
				MinimumShouldMatch: proto.Int32(1),
				MustNot:            &filterOnAA,
			}})
		case analyticspb.Attribute_HAS_MANAGER_TRANSFER:
			f = append(f, ConvoFilterBoolFilter{Nested: managerTransferNestedFilter()})
		case analyticspb.Attribute_HAS_NO_MANAGER_TRANSFER:
			f = append(f, ConvoFilterBoolFilter{Bool: &ConvoFilterBoolFilterBool{
				MustNot: &ConvoFilterBoolFilterBoolFilter{Nested: managerTransferNestedFilter()},
			}})

		default:
			return ConvoFilter{}, status.Errorf(codes.InvalidArgument, "unsupported live_assist value %v", la)
		}
	}

	return ConvoFilter{
		Bool: &ConvoFilterBool{
			Should: &f,
		},
	}, nil
}

func managerTransferNestedFilter() *Nested {
	return &Nested{
		Path: actionAnnotationsPath,
		Query: &NestedFilter{
			Bool: &NestedFilterBool{
				Filter: &[]NestedFilter{{
					Terms: &NestedTermsFilter{ActionTypes: &[]int{int(dbactionpb.Action_MANAGER_TRANSFER)}},
				}},
			},
		},
	}
}

// Convert numeric bin to ES range filter for metadata outcome or cresta-modeled outcome.
func ConvertNumericBinToESRange(numericBin *metadata.NumericBin, isCrestaModelledOutCome bool) NestedFilter {
	numericRange := FloatRange{}
	fromValue := float64(numericBin.FromValue)
	toValue := float64(numericBin.ToValue)
	if numericBin.FromValueIsExclusive {
		numericRange.GT = &fromValue
	} else {
		numericRange.GTE = &fromValue
	}
	if numericBin.ToValueIsInclusive {
		numericRange.LTE = &toValue
	} else {
		numericRange.LT = &toValue
	}
	NestedFilter := NestedFilter{
		Range: &RangeFilter{},
	}
	if isCrestaModelledOutCome {
		if *enableUsageOfExplicitMappingMomentAnnotationPayload {
			NestedFilter.Range.OutcomeNumberValueExplicitMapping = &numericRange
		} else {
			NestedFilter.Range.OutcomeNumberValue = &numericRange
		}
	} else {
		if *enableUsageOfExplicitMappingMomentAnnotationPayload {
			NestedFilter.Range.MetadataNumberValueExplicitMapping = &numericRange
		} else {
			NestedFilter.Range.MetadataNumberValue = &numericRange
		}
	}
	return NestedFilter
}

func convertMetadataValueToFilter(mg *analyticspb.MomentGroup, valueIndex int) (Nested, error) {
	if len(mg.Moments) != 1 {
		return Nested{}, status.Error(codes.InvalidArgument, "must only include a single metadata moment in the moment group.")
	}
	momentName, err := momentpb.ParseMomentName(mg.Moments[0].Name)
	if err != nil {
		return Nested{}, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", mg.Moments[0].Name, err)
	}
	if len(mg.MetadataValueAttributes) == 0 && mg.BinaryOutcomeValueEnum == enumpb.BinaryOutcomeValueEnum_BINARY_OUTCOME_VALUE_ENUM_UNSPECIFIED {
		return Nested{
			Path: MomentAnnotationsPath,
			Query: &NestedFilter{
				Bool: &NestedFilterBool{
					Filter: &[]NestedFilter{
						{
							Terms: &NestedTermsFilter{MomentTemplateIDs: &[]string{momentName.MomentID}},
						},
					},
				},
			},
		}, nil
	}
	var attributes []*analyticspb.MetadataValueAttribute
	if len(mg.MetadataValueAttributes) > 0 {
		attributes = mg.MetadataValueAttributes
	}
	if mg.BinaryOutcomeValueEnum != enumpb.BinaryOutcomeValueEnum_BINARY_OUTCOME_VALUE_ENUM_UNSPECIFIED {
		metadataValueType := mg.Moments[0].GetMomentData().GetConversationMetadataMomentData().GetType()
		switch metadataValueType {
		case metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
			switch mg.BinaryOutcomeValueEnum {
			case enumpb.BinaryOutcomeValueEnum_POSITIVE:
				attributes = append(attributes, &analyticspb.MetadataValueAttribute{
					NumericBin: &metadata.NumericBin{
						FromValue:            0,
						FromValueIsExclusive: true,
						ToValue:              math.MaxFloat32,
						ToValueIsInclusive:   false,
					},
				})
			case enumpb.BinaryOutcomeValueEnum_NEGATIVE:
				attributes = append(attributes, &analyticspb.MetadataValueAttribute{
					NumericBin: &metadata.NumericBin{
						FromValue:            0,
						FromValueIsExclusive: false,
						ToValue:              0,
						ToValueIsInclusive:   true,
					},
				})
			default:
				return Nested{}, status.Errorf(codes.InvalidArgument, "unsupported binary outcome value:%v", mg.BinaryOutcomeValueEnum)
			}
		case metadata.ConversationMetadataEnum_TYPE_CUSTOM_STRING:
			for _, allowedValue := range mg.Moments[0].GetMomentData().GetConversationMetadataMomentData().GetAllowedValues() {
				if allowedValue.GetBinaryOutcomeValueEnum() == mg.BinaryOutcomeValueEnum {
					attributes = append(attributes, &analyticspb.MetadataValueAttribute{
						MetadataValue: allowedValue.Value,
					})
				}
			}
			if len(attributes) == 0 {
				return Nested{}, status.Errorf(codes.InvalidArgument, "failed to find the metadata value for outcome: %v", mg.BinaryOutcomeValueEnum)
			}
		default:
			return Nested{}, status.Errorf(codes.InvalidArgument, "doesn't support the metadata type:%v", metadataValueType)
		}
	}

	var nestedFilterBool *NestedFilterBool
	switch {
	case attributes[0].NumericBin != nil:
		if momentName.MomentID == "*" {
			return Nested{}, status.Error(codes.InvalidArgument, "numeric metadata does not support wildcard moment id")
		}
		metadataRanges := []NestedFilter{}
		for index, metadataAttribute := range attributes {
			if valueIndex >= 0 && index != valueIndex {
				continue
			}
			if metadataAttribute.NumericBin == nil {
				return Nested{}, status.Error(codes.InvalidArgument, "the NumericBin is missing for numeric metadata.")
			}
			metadataRanges = append(metadataRanges, ConvertNumericBinToESRange(metadataAttribute.NumericBin, false))
		}
		nestedFilterBool = &NestedFilterBool{
			Filter: &[]NestedFilter{
				{
					Terms: &NestedTermsFilter{MomentTemplateIDs: &[]string{momentName.MomentID}},
				},
			},
			Should:             &metadataRanges,
			MinimumShouldMatch: 1,
		}
	case attributes[0].MetadataValue != nil:
		var (
			stringPayloadValues  []string
			numericPayloadValues []float32
			booleanPayloadValues []bool
		)

		for index, metadataAttribute := range attributes {
			if valueIndex >= 0 && index != valueIndex {
				continue
			}
			switch val := metadataAttribute.MetadataValue; val.Value.(type) {
			case *metadata.MetadataValue_StringValue:
				stringPayloadValues = append(stringPayloadValues, val.GetStringValue())
			case *metadata.MetadataValue_NumberValue:
				numericPayloadValues = append(numericPayloadValues, val.GetNumberValue())
			case *metadata.MetadataValue_BooleanValue:
				booleanPayloadValues = append(booleanPayloadValues, val.GetBooleanValue())
			}
		}

		filters := []NestedFilter{}
		if momentName.MomentID != "*" {
			filters = append(filters, NestedFilter{
				Terms: &NestedTermsFilter{MomentTemplateIDs: &[]string{momentName.MomentID}},
			})
			if len(stringPayloadValues) > 0 {
				filters = append(filters, NestedFilter{Terms: &NestedTermsFilter{
					MetadataStringPayloadValues: &stringPayloadValues,
				}})
			}
			if len(numericPayloadValues) > 0 {
				if *enableUsageOfExplicitMappingMomentAnnotationPayload {
					filters = append(filters, NestedFilter{Terms: &NestedTermsFilter{
						MetadataNumberValueExplicitMapping: &numericPayloadValues,
					}})
				} else {
					filters = append(filters, NestedFilter{Terms: &NestedTermsFilter{
						MetadataNumberPayloadValues: &numericPayloadValues,
					}})
				}
			}
			if len(booleanPayloadValues) > 0 {
				filters = append(filters, NestedFilter{Terms: &NestedTermsFilter{
					MetadataBooleanPayloadValues: &booleanPayloadValues,
				}})
			}
		} else {
			if len(stringPayloadValues) == 0 {
				return Nested{}, status.Error(codes.InvalidArgument, "no string value specified for categorical metadata.")
			}
			if val := stringPayloadValues[0]; isExactKeyword(val) {
				filters = append(filters, NestedFilter{Term: &TermFilter{
					MetadataPayloadStringValues: &ValueField{
						Value:           strings.Trim(val, "\""),
						CaseInsensitive: proto.Bool(true),
					},
				}})
			} else {
				filters = append(filters, NestedFilter{
					WildcardFilter: &WildcardFilter{
						MetadataPayload: &ValueField{
							Value:           fmt.Sprintf("*%s*", val),
							CaseInsensitive: proto.Bool(true),
						},
					},
				})
			}
		}
		nestedFilterBool = &NestedFilterBool{
			Filter: &filters,
		}
	default:
		return Nested{}, status.Error(codes.InvalidArgument, "both numeric_bin and metadata_value are missing in the metadata attribute.")
	}

	return Nested{
		Path: MomentAnnotationsPath,
		Query: &NestedFilter{
			Bool: nestedFilterBool,
		},
	}, nil
}

func convertConversationOutcomeValueToFilter(mg *analyticspb.MomentGroup, valueIndex int) (Nested, error) {
	if len(mg.Moments) != 1 {
		return Nested{}, status.Error(codes.InvalidArgument, "must only include a single conversation outcome moment in the moment group.")
	}
	momentName, err := momentpb.ParseMomentName(mg.Moments[0].Name)
	if err != nil {
		return Nested{}, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", mg.Moments[0].Name, err)
	}

	attributes := []*analyticspb.OutcomeValueAttribute{}
	switch {
	case mg.BinaryOutcomeValueEnum != enumpb.BinaryOutcomeValueEnum_BINARY_OUTCOME_VALUE_ENUM_UNSPECIFIED:
		for _, value := range mg.Moments[0].GetMomentData().GetConversationOutcomeMomentData().GetOutcomeValues() {
			if value.GetBinaryOutcomeValueEnum() == mg.BinaryOutcomeValueEnum {
				attributes = append(attributes, &analyticspb.OutcomeValueAttribute{
					OutcomeValue: value,
				})
			}
		}
		if len(attributes) == 0 {
			return Nested{}, status.Errorf(codes.InvalidArgument, "failed to find the outcome value for %v", mg.BinaryOutcomeValueEnum)
		}
	case len(mg.OutcomeValueAttributes) > 0:
		attributes = mg.OutcomeValueAttributes
	}

	outcomeBoolValues := []bool{}
	outcomeStringValues := []string{}
	outcomeNummericValues := []float32{}
	outcomeNumericBins := []*metadata.NumericBin{}
	for index, attribute := range attributes {
		if valueIndex >= 0 && index != valueIndex {
			continue
		}
		if bin := attribute.NumericBin; bin != nil {
			outcomeNumericBins = append(outcomeNumericBins, bin)
		}
		if attribute.GetOutcomeValue() != nil && attribute.GetOutcomeValue().GetValue() != nil {
			switch v := attribute.GetOutcomeValue().GetValue().Value.(type) {
			case *momentpb.ConversationOutcomeValue_StringValue:
				outcomeStringValues = append(outcomeStringValues, v.StringValue)
			case *momentpb.ConversationOutcomeValue_BooleanValue:
				outcomeBoolValues = append(outcomeBoolValues, v.BooleanValue)
			case *momentpb.ConversationOutcomeValue_NumberValue:
				outcomeNummericValues = append(outcomeNummericValues, v.NumberValue)
			default:
				return Nested{}, status.Errorf(codes.InvalidArgument, "unsupported conversation outcome value %v", v)
			}
		}

	}

	filters := []NestedFilter{
		{
			Terms: &NestedTermsFilter{MomentTemplateIDs: &[]string{momentName.MomentID}},
		},
	}
	if len(outcomeBoolValues) > 0 {
		filters = append(filters, NestedFilter{
			Terms: &NestedTermsFilter{
				OutcomeBooleanPayloadValues: &outcomeBoolValues,
			},
		})
	}
	if len(outcomeStringValues) > 0 {
		filters = append(filters, NestedFilter{
			Terms: &NestedTermsFilter{
				OutcomeStringPayloadValues: &outcomeStringValues,
			},
		})
	}
	if len(outcomeNummericValues) > 0 {
		if *enableUsageOfExplicitMappingMomentAnnotationPayload {
			filters = append(filters, NestedFilter{
				Terms: &NestedTermsFilter{
					OutcomeNumberValueExplicitMapping: &outcomeNummericValues,
				},
			})
		} else {
			filters = append(filters, NestedFilter{
				Terms: &NestedTermsFilter{
					OutcomeNumberPayloadValues: &outcomeNummericValues,
				},
			})
		}
	}
	ret := Nested{
		Path: MomentAnnotationsPath,
		Query: &NestedFilter{
			Bool: &NestedFilterBool{
				Filter: &filters,
			},
		},
	}
	should := []NestedFilter{}
	for _, bin := range outcomeNumericBins {
		should = append(should, ConvertNumericBinToESRange(bin, true))
	}
	if len(should) > 0 {
		ret.Query.Bool.Should = &should
		ret.Query.Bool.MinimumShouldMatch = 1
	}
	return ret, nil
}

func convertConvoMomentGroupToConvoFilters(mg *analyticspb.MomentGroup) (*ConvoFilter, *ConvoFilter, error) {
	isMetadata := IsConversationMetadataMomentGroup(mg)
	isOutcome := IsConversationOutcomeMomentGroup(mg)

	if (isMetadata || isOutcome) && len(mg.ExcludedMoments) > 0 && len(mg.Moments) > 0 {
		excludedByID := map[string]*momentpb.Moment{}
		for _, m := range mg.ExcludedMoments {
			mn, err := momentpb.ParseMomentName(m.Name)
			if err != nil {
				return nil, nil, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", m.Name, err)
			}
			excludedByID[mn.MomentID] = m
		}

		should := []ConvoFilterBoolFilter{}
		for _, m := range mg.Moments {
			mn, err := momentpb.ParseMomentName(m.Name)
			if err != nil {
				return nil, nil, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", m.Name, err)
			}
			isOverlap := false
			if _, ok := excludedByID[mn.MomentID]; ok {
				isOverlap = true
				delete(excludedByID, mn.MomentID)
			}

			subMg := &analyticspb.MomentGroup{
				Moments:                []*momentpb.Moment{m},
				BinaryOutcomeValueEnum: mg.BinaryOutcomeValueEnum,
			}

			var nested Nested
			if isMetadata {
				subMg.MetadataValueAttributes = mg.MetadataValueAttributes
				nested, err = convertMetadataValueToFilter(subMg, -1)
			} else {
				subMg.OutcomeValueAttributes = mg.OutcomeValueAttributes
				nested, err = convertConversationOutcomeValueToFilter(subMg, -1)
			}
			if err != nil {
				return nil, nil, err
			}
			should = append(should, ConvoFilterBoolFilter{Nested: &nested})

			if isOverlap {
				id := mn.MomentID
				should = append(should, ConvoFilterBoolFilter{
					Bool: &ConvoFilterBoolFilterBool{
						MustNot: &ConvoFilterBoolFilterBoolFilter{
							Nested: &Nested{
								Path: MomentAnnotationsPath,
								Query: &NestedFilter{
									Bool: &NestedFilterBool{
										Filter: &[]NestedFilter{{
											Terms: &NestedTermsFilter{MomentTemplateIDs: &[]string{id}},
										}},
									},
								},
							},
						},
					},
				})
			}
		}

		var inclFilter *ConvoFilter
		if len(should) > 0 {
			inclFilter = &ConvoFilter{Bool: &ConvoFilterBool{Should: &should, MinimumShouldMatch: 1}}
		}

		// Remaining excluded-only moments
		excludedOnly := []*momentpb.Moment{}
		for _, m := range excludedByID {
			excludedOnly = append(excludedOnly, m)
		}

		var exclFilter *ConvoFilter
		if len(excludedOnly) > 0 {
			nested, err := convertNonCompositeMomentsToNested(momentpb.Moment_TYPE_UNSPECIFIED, mg.Adherence, nil, excludedOnly, mg.Behaviors)
			if err != nil {
				return nil, nil, err
			}
			exclFilter = &ConvoFilter{Nested: nested}
		}
		return inclFilter, exclFilter, nil
	}

	var inclFilter *ConvoFilter
	var exclFilter *ConvoFilter

	// MomentType filter does not apply to excluded moment filter.
	if len(mg.ExcludedMoments) > 0 {
		nested, err := convertNonCompositeMomentsToNested(momentpb.Moment_TYPE_UNSPECIFIED, mg.Adherence, nil, mg.ExcludedMoments, mg.Behaviors)
		if err != nil {
			return nil, nil, err
		}
		if IsConversationReasonMomentGroup(mg) {
			exclFilter = &ConvoFilter{
				Bool: &ConvoFilterBool{
					Should: &[]ConvoFilterBoolFilter{
						{Nested: nested},
						{HasChild: nestedHasChildMessage(nested)},
					},
					MinimumShouldMatch: 1,
				},
			}
		} else {
			exclFilter = &ConvoFilter{
				Nested: nested,
			}
		}
		return nil, exclFilter, nil
	}
	if isMetadata {
		nested, err := convertMetadataValueToFilter(mg, -1)
		if err != nil {
			return nil, nil, err
		}
		inclFilter = &ConvoFilter{Nested: &nested}
		return inclFilter, nil, nil
	} else if isOutcome {
		nested, err := convertConversationOutcomeValueToFilter(mg, -1)
		if err != nil {
			return nil, nil, err
		}
		inclFilter = &ConvoFilter{Nested: &nested}
		return inclFilter, nil, nil
	}
	if len(mg.Moments) > 0 || mg.MomentType != momentpb.Moment_TYPE_UNSPECIFIED || len(mg.Behaviors) > 0 {
		nested, err := convertNonCompositeMomentsToNested(mg.MomentType, mg.Adherence, mg.AdherenceTypesForMoments, mg.Moments, mg.Behaviors)
		if err != nil {
			return nil, nil, err
		}
		if IsConversationReasonMomentGroup(mg) {
			inclFilter = &ConvoFilter{
				Bool: &ConvoFilterBool{
					Should: &[]ConvoFilterBoolFilter{
						{Nested: nested},
						{HasChild: nestedHasChildMessage(nested)},
					},
					MinimumShouldMatch: 1,
				},
			}
		} else {
			inclFilter = &ConvoFilter{
				Nested: nested,
			}
		}
	}
	return inclFilter, exclFilter, nil
}

// Convo MomentGroup is always non-insisghts composite moment, thus matching via moment annotations.
func convertNonCompositeMomentsToNested(
	momentType momentpb.Moment_Type,
	adherenceTypeForMomentGroup enumpb.AdherenceType,
	adherenceTypeForMoments []*analyticspb.MomentGroup_MomentAdherenceTypeConfig,
	moments []*momentpb.Moment,
	behaviors []*behaviorpb.Behavior,
) (*Nested, error) {
	if len(moments) == 0 && momentType == momentpb.Moment_TYPE_UNSPECIFIED && len(behaviors) == 0 {
		return nil, fmt.Errorf("both moments and behaviors are empty")
	}
	filters := []NestedFilter{}
	should := []NestedFilter{}
	if momentType != momentpb.Moment_TYPE_UNSPECIFIED {
		dbMomentType, err := momentconverter.ServiceToDBMomentType(momentType)
		if err != nil {
			return nil, err
		}

		filters = append(filters, NestedFilter{
			Terms: &NestedTermsFilter{
				MomentTypes: &[]int{int(dbMomentType)},
			},
		})
	}
	momentIDs := []string{}
	for _, moment := range moments {
		momentName, err := momentpb.ParseMomentName(moment.Name)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", moment.Name, err)
		}
		momentIDs = append(momentIDs, momentName.MomentID)
	}
	// adherence types has to work together with moment_id/behavior
	if len(momentIDs) > 0 || len(behaviors) > 0 {
		f, s, err := nestedFilterWithMomentAdherenceType(adherenceTypeForMomentGroup, adherenceTypeForMoments, momentIDs, behaviors)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f...)
		should = append(should, s...)
	}
	nestedFilterBool := &NestedFilterBool{}
	if len(filters) > 0 {
		nestedFilterBool.Filter = &filters
	}
	if len(should) > 0 {
		nestedFilterBool.Should = &should
		nestedFilterBool.MinimumShouldMatch = 1
	}
	return &Nested{
		Path: MomentAnnotationsPath,
		Query: &NestedFilter{
			Bool: nestedFilterBool,
		},
	}, nil
}

func nestedFilterWithMomentAdherenceType(adherenceTypeForMomentGroup enumpb.AdherenceType, adherenceTypeForMoments []*analyticspb.MomentGroup_MomentAdherenceTypeConfig, momentIDs []string, behaviors []*behaviorpb.Behavior) ([]NestedFilter, []NestedFilter, error) {
	if len(behaviors) > 0 {
		if len(momentIDs) != 0 {
			return nil, nil, status.Error(codes.InvalidArgument, "set either moments or behaviors in the moment group.")
		}
		if len(adherenceTypeForMoments) != len(behaviors) {
			return nil, nil, status.Error(codes.InvalidArgument, "length of behaviors and adherenceTypeForMoments has to be the same")
		}
	} else {
		if len(momentIDs) == 0 {
			return nil, nil, status.Error(codes.InvalidArgument, "both moments or behaviors are empty in the moment group.")
		}
		if len(adherenceTypeForMoments) > 0 && len(adherenceTypeForMoments) != len(momentIDs) {
			return nil, nil, status.Error(codes.InvalidArgument, "length of moments and adherenceTypeForMoments has to be the same")
		}
	}

	filters := []NestedFilter{}
	should := []NestedFilter{}
	if len(adherenceTypeForMoments) == 0 {
		filters = append(filters, NestedFilter{
			Terms: &NestedTermsFilter{MomentTemplateIDs: &momentIDs},
		})
		if adherenceTypeForMomentGroup != enumpb.AdherenceType_ADHERENCE_TYPE_UNSPECIFIED {
			filters = append(filters, NestedFilter{
				Terms: &NestedTermsFilter{MomentAdherenceTypes: &[]int{int(adherenceTypeForMomentGroup)}},
			})
		}
	} else {
		momentOrBehaviorIDs := &momentIDs
		if len(behaviors) > 0 {
			behaviorIDs := []string{}
			for _, behavior := range behaviors {
				behaviorName, err := behaviorpb.ParseBehaviorName(behavior.Name)
				if err != nil {
					return nil, nil, err
				}
				behaviorIDs = append(behaviorIDs, behaviorName.BehaviorID)
			}
			momentOrBehaviorIDs = &behaviorIDs
		}
		for i := range *momentOrBehaviorIDs {
			var adherenceTypes []int
			for _, adherenceType := range adherenceTypeForMoments[i].AdherenceTypes {
				adherenceTypes = append(adherenceTypes, int(adherenceType))
			}
			var momentOrBehaviorFilter *NestedTermsFilter
			if len(behaviors) > 0 {
				momentOrBehaviorFilter = &NestedTermsFilter{
					BehaviorIDs: &[]string{(*momentOrBehaviorIDs)[i]},
				}
			} else {
				momentOrBehaviorFilter = &NestedTermsFilter{
					MomentTemplateIDs: &[]string{(*momentOrBehaviorIDs)[i]},
				}
			}
			filtersPerMoment := []NestedFilter{
				{
					Terms: momentOrBehaviorFilter,
				},
				{
					Terms: &NestedTermsFilter{
						MomentAdherenceTypes: &adherenceTypes,
					},
				},
			}
			should = append(should, NestedFilter{Bool: &NestedFilterBool{
				Filter: &filtersPerMoment,
			}})
		}
	}
	return filters, should, nil
}

func convertActionGroupToConvoFilter(
	ag *analyticspb.ActionGroup,
) (ConvoFilter, error) {
	filterForType, filtersForHints, err := convertActionGroupToNestedFilters(ag)
	if err != nil {
		return ConvoFilter{}, err
	}
	if (filterForType == nil) && (len(filtersForHints) == 0) {
		return ConvoFilter{}, fmt.Errorf("either filterForType or filtersForHints is non-empty")
	}
	if filterForType != nil {
		return ConvoFilter{
			HasChild: &HasChild{
				Type: string(DocumentTypeMessage),
				Query: InnerMsgQuery{
					Bool: &InnerMsgBool{
						Filter: &[]MsgFilter{
							{
								Nested: nestedFromNestedFilters(filterForType, actionAnnotationsPath),
							},
						},
					},
				},
			},
		}, nil
	}
	convoFilterBoolFilters := []ConvoFilterBoolFilter{}
	for _, fh := range filtersForHints {
		ff := HintFilterForConvo(fh)
		convoFilterBoolFilters = append(convoFilterBoolFilters, ConvoFilterBoolFilter{
			Bool: &ConvoFilterBoolFilterBool{
				Should:             &ff,
				MinimumShouldMatch: proto.Int32(int32(len(ff))),
			},
		})
	}
	return ConvoFilter{
		Bool: &ConvoFilterBool{
			Should:             &convoFilterBoolFilters,
			MinimumShouldMatch: 1,
		},
	}, nil
}

func HintFilterForMessage(fh FilterForHint) []MsgFilter {
	ret := []MsgFilter{}
	// Ignore hint_sent filter when looking for hint_followed instances.
	switch {
	case fh.ForHintFollowed != nil:
		ret = append(ret, MsgFilter{Nested: nestedFromNestedFilters(fh.ForHintFollowed, fh.PathForHintEngagement)})
	case fh.ForHintNotFollowed != nil:
		// TODO(zhucaiyi): should include condition for hint sent.
		ret = append(ret, MsgFilter{Nested: nestedFromNestedFilters(&[]NestedFilter{
			{
				Bool: &NestedFilterBool{
					MustNot: &[]NestedFilter{{
						Bool: &NestedFilterBool{
							Filter: fh.ForHintNotFollowed,
						},
					}},
				},
			},
		}, fh.PathForHintEngagement)})
	case fh.ForHintSent != nil:
		ret = append(ret, MsgFilter{Nested: nestedFromNestedFilters(&[]NestedFilter{*fh.ForHintSent}, actionAnnotationsPath)})
	}
	return ret
}

func HintFilterForConvo(fh FilterForHint) []ConvoFilterBoolFilterBoolFilter {
	ret := []ConvoFilterBoolFilterBoolFilter{}
	if fh.ForHintSent != nil {
		ret = append(ret, ConvoFilterBoolFilterBoolFilter{
			HasChild: &HasChild{
				Type: string(DocumentTypeMessage),
				Query: InnerMsgQuery{
					Bool: &InnerMsgBool{
						Filter: &[]MsgFilter{
							{
								Nested: nestedFromNestedFilters(&[]NestedFilter{*fh.ForHintSent}, actionAnnotationsPath),
							},
						},
					},
				},
			},
		})
	}
	if fh.ForHintFollowed != nil {
		ret = append(ret, ConvoFilterBoolFilterBoolFilter{HasChild: &HasChild{
			Type: string(DocumentTypeMessage),
			Query: InnerMsgQuery{
				Bool: &InnerMsgBool{
					Filter: &[]MsgFilter{
						{
							Nested: nestedFromNestedFilters(fh.ForHintFollowed, fh.PathForHintEngagement),
						},
					},
				},
			},
		}})
	}
	if fh.ForHintNotFollowed != nil {
		ret = append(ret, ConvoFilterBoolFilterBoolFilter{
			Bool: &NotHasChildBool{
				MustNot: &HasChildFilter{
					HasChild: &HasChild{
						Type: string(DocumentTypeMessage),
						Query: InnerMsgQuery{
							Bool: &InnerMsgBool{
								Filter: &[]MsgFilter{
									{
										Nested: nestedFromNestedFilters(fh.ForHintNotFollowed, fh.PathForHintEngagement),
									},
								},
							},
						},
					},
				},
			},
		})
	}
	return ret
}

type FilterForHint struct {
	ForHintSent           *NestedFilter
	ForHintFollowed       *[]NestedFilter
	ForHintNotFollowed    *[]NestedFilter
	PathForHintEngagement string // ConversationEvents or MomentAnnotations
}

func convertActionGroupToNestedFilters(ag *analyticspb.ActionGroup) (*[]NestedFilter, []FilterForHint, error) {
	if len(ag.Actions) == 0 && ag.ActionType == actionpb.Action_TYPE_UNSPECIFIED {
		return nil, nil, fmt.Errorf("both actionType and actionRecords are empty")
	}
	var filterForActionType *[]NestedFilter
	filtersForHints := []FilterForHint{}

	// ignore action_type when specific actions are provided.
	if len(ag.Actions) == 0 && ag.ActionType != actionpb.Action_TYPE_UNSPECIFIED {
		dbType, err := serviceToDBActionType(ag.ActionType)
		if err != nil {
			return nil, nil, fmt.Errorf("both actionType and actionRecords are empty")
		}
		filterForActionType = &[]NestedFilter{{
			Terms: &NestedTermsFilter{ActionTypes: &[]int{int(dbType)}},
		}}
	}

	for _, ac := range ag.Actions {
		f, err := ConvertHintToFilter(convertActionRecord(ac))
		if err != nil {
			return nil, nil, err
		}
		filtersForHints = append(filtersForHints, *f)
	}
	return filterForActionType, filtersForHints, nil
}

func convertActionRecord(ac *analyticspb.ActionGroup_ActionRecord) HintParam {
	ret := HintParam{
		Action: ac.Action,
	}
	switch ac.Engagement {
	case analyticspb.ActionGroup_ActionRecord_ENGAGED:
		ret.EngagmentType = HintEngaged
	case analyticspb.ActionGroup_ActionRecord_NOT_ENGAGED:
		ret.EngagmentType = HintNotEngaged
	}
	return ret
}

type HintEngagementType int32

const (
	HintEngagementTypeUnknown HintEngagementType = iota
	HintEngaged
	HintNotEngaged
)

type HintParam struct {
	EngagmentType HintEngagementType
	Action        *actionpb.Action
}

func ConvertHintToFilter(hintParam HintParam) (*FilterForHint, error) {
	actionName, err := actionpb.ParseActionName(hintParam.Action.Name)
	if err != nil {
		return nil, err
	}
	fh := FilterForHint{
		ForHintSent: &NestedFilter{Terms: &NestedTermsFilter{ActionIDs: &[]string{actionName.ActionID}}},
	}
	if engagementConfig := hintParam.EngagmentType; engagementConfig != HintEngagementTypeUnknown {
		switch actionType := hintParam.Action.Type; actionType {
		case actionpb.Action_HINT, actionpb.Action_CHECKLIST:
			// For behavior and checklist hint type, hint is engaged via moment annotation.
			fh.PathForHintEngagement = MomentAnnotationsPath
			ff := []NestedFilter{
				{
					Terms: &NestedTermsFilter{
						MomenAnnotationAdherenceActionIDs: &[]string{actionName.ActionID},
					},
				},
				{
					Terms: &NestedTermsFilter{
						MomentAdherenceTypes: &[]int{2},
					},
				},
			}
			if engagementConfig == HintEngaged {
				fh.ForHintFollowed = &ff
			} else {
				fh.ForHintNotFollowed = &ff
			}
		case actionpb.Action_GUIDED_WORKFLOW_HINT, actionpb.Action_KB_HINT:
			fh.PathForHintEngagement = conversationEventsPath
			// For GW/KB hint type, hint is engaged via conversation events.
			ff := []NestedFilter{
				{
					Terms: &NestedTermsFilter{
						ConversationEventsAdherenceActionIDs: &[]string{actionName.ActionID},
					},
				},
			}
			if engagementConfig == HintEngaged {
				fh.ForHintFollowed = &ff
			} else {
				fh.ForHintNotFollowed = &ff
			}
		case actionpb.Action_MANAGER_ALERT:
			// No-op, intentionally left blank.
		default:
			return nil, status.Errorf(codes.InvalidArgument, "action type %v not supported", actionType)
		}
	}
	return &fh, nil
}

func convertMessageMomentsToConvoFilters(
	moments []*momentpb.Moment,
	behaviors []*behaviorpb.Behavior,
	momentType momentpb.Moment_Type,
	adherenceTypeForMomentGroup enumpb.AdherenceType,
	adherenceTypeForMoments []*analyticspb.MomentGroup_MomentAdherenceTypeConfig,
	speakerRoleFilter *MsgFilter,
	topicDetails map[string]TopicDetails,
	languageCodes []string,
) ([]ConvoFilter, error) {
	convoFilters := []ConvoFilter{}
	insightsCompositeNames := []string{}
	nonInsightsCompositeIDs := []string{}
	childMsgType := DocumentTypeMessage
	for _, moment := range moments {
		switch {
		case IsInsightsCompositeMoment(moment):
			insightsCompositeNames = append(insightsCompositeNames, moment.Name)
		default:
			// TDv1/v2 topics live on message_group child docs; V3 topics are message-anchored on
			// the message child doc, so they keep the default DocumentTypeMessage and use the
			// has_child placement below.
			if moment.Type == momentpb.Moment_TOPIC && !IsTopicDiscoveryV3Moment(moment) {
				childMsgType = DocumentTypeMessageGroup
			}
			momentName, err := momentpb.ParseMomentName(moment.Name)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "unparsable moment name %q: %v", moment.Name, err)
			}
			nonInsightsCompositeIDs = append(nonInsightsCompositeIDs, momentName.MomentID)
		}
	}
	filters := []NestedFilter{}
	should := []NestedFilter{}
	if momentType != momentpb.Moment_TYPE_UNSPECIFIED {
		dbMomentType, err := momentconverter.ServiceToDBMomentType(momentType)
		if err != nil {
			return nil, err
		}
		filters = append(filters, NestedFilter{
			Terms: &NestedTermsFilter{
				MomentTypes: &[]int{int(dbMomentType)},
			},
		})
	}
	if len(nonInsightsCompositeIDs) > 0 || len(behaviors) > 0 {
		f, s, err := nestedFilterWithMomentAdherenceType(adherenceTypeForMomentGroup, adherenceTypeForMoments, nonInsightsCompositeIDs, behaviors)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f...)
		should = append(should, s...)
	}
	if len(filters)+len(should) > 0 {
		nestedFilterBool := &NestedFilterBool{}
		if len(filters) > 0 {
			nestedFilterBool.Filter = &filters
		}
		if len(should) > 0 {
			nestedFilterBool.Should = &should
			nestedFilterBool.MinimumShouldMatch = 1
		}
		nested := &Nested{
			Path: MomentAnnotationsPath,
			Query: &NestedFilter{
				Bool: nestedFilterBool,
			},
		}
		msgFilters := []MsgFilter{{Nested: nested}}
		if speakerRoleFilter != nil {
			msgFilters = append(msgFilters, *speakerRoleFilter)
		}
		hasChild := &HasChild{
			Type: string(childMsgType),
			Query: InnerMsgQuery{
				Bool: &InnerMsgBool{
					Filter: &msgFilters,
				},
			},
		}
		switch {
		case isConversationReasonMoment(momentType, moments):
			// Legacy CONVERSATION_REASON annotations may be anchored either on a child
			// message document or on the conversation document, so match both placements
			// via should{}.
			convoFilters = append(convoFilters, ConvoFilter{
				Bool: &ConvoFilterBool{
					Should: &[]ConvoFilterBoolFilter{
						{HasChild: hasChild},
						{Nested: nested},
					},
					MinimumShouldMatch: 1,
				},
			})
		case ContainsTopicDiscoveryV3Moment(moments):
			// V3 topic annotations (DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC) are always
			// anchored on the child message document, so only the has_child placement is
			// needed.
			convoFilters = append(convoFilters, ConvoFilter{HasChild: hasChild})
		default:
			convoFilters = append(convoFilters, ConvoFilter{HasChild: hasChild})
		}
	}

	if len(insightsCompositeNames) > 0 {
		topicBoolsForMsg, topicBoolsForMsgGroup := []*TopicBool{}, []*TopicBool{}
		hasSpeakerRoleFilter := false
		for _, name := range insightsCompositeNames {
			td, exist := topicDetails[name]
			if !exist {
				return nil, fmt.Errorf("topic details for moment %q does not exists", name)
			}
			if len(td.SpeakerRoles) > 0 {
				hasSpeakerRoleFilter = true
			}
			topicBool, err := ConvertTopicDetailToTopicBool(td, languageCodes)
			if err != nil {
				return nil, err
			}
			topicBoolForMsg, topicBoolForMsgGroup := SplitTopicBoolToMsgAndMsgGroup(topicBool)

			if topicBoolForMsg != nil {
				topicBoolsForMsg = append(topicBoolsForMsg, topicBoolForMsg)
			}
			if topicBoolForMsgGroup != nil {
				topicBoolsForMsgGroup = append(topicBoolsForMsgGroup, topicBoolForMsgGroup)
			}
		}

		hasChildFilters := []ConvoFilterBoolFilter{}

		for _, t := range topicBoolsForMsg {
			f := ConvoFilterBoolFilter{HasChild: &HasChild{
				Type: string(DocumentTypeMessage),
				Query: InnerMsgQuery{
					Bool: &InnerMsgBool{
						Filter: &[]MsgFilter{{
							Bool: t,
						}},
					},
				},
			}}
			if speakerRoleFilter != nil && !hasSpeakerRoleFilter {
				*f.HasChild.Query.Bool.Filter = append(*f.HasChild.Query.Bool.Filter, *speakerRoleFilter)
			}
			hasChildFilters = append(hasChildFilters, f)
		}

		for _, t := range topicBoolsForMsgGroup {
			f := ConvoFilterBoolFilter{HasChild: &HasChild{
				Type: string(DocumentTypeMessageGroup),
				Query: InnerMsgQuery{
					Bool: &InnerMsgBool{
						Filter: &[]MsgFilter{{
							Bool: t,
						}},
					},
				},
			}}
			if speakerRoleFilter != nil && !hasSpeakerRoleFilter {
				*f.HasChild.Query.Bool.Filter = append(*f.HasChild.Query.Bool.Filter, *speakerRoleFilter)
			}
			hasChildFilters = append(hasChildFilters, f)
		}

		convoFilters = append(convoFilters, ConvoFilter{
			Bool: &ConvoFilterBool{
				Should: &hasChildFilters,
			},
		})
	}
	return convoFilters, nil
}

// filterMessageByAttribute is parsed as agg so that
// moments on-the-fly are returned.
func ParseFilterByAttributeStructuresToAgg(
	momentGroup *analyticspb.MomentGroup,
	topicDetails map[string]TopicDetails,
	pageSize int,
	esQueryConfig QueryConfig,
) (*ReqAggs, error) {
	if momentGroup == nil {
		return nil, nil
	}
	convCount := CreateReqTotalConversationCount(MaxCardinalityPrecisionThreshold)
	reqAgg := ReqAggs{
		TotalConversationCount: convCount,
	}
	aggByAnnotations := ReqAnnotations{
		Aggs: &ReqAggAnnotation{
			TotalConversationCount: convCount,
			AnnotationHit: &ReqAnnotationHit{
				TopHits: ReqTopHits{
					Size: pageSize,
				},
			},
		},
	}
	if includeConversationMessageCount(esQueryConfig) {
		msgCount := CreateReqTotalMessageCount(MaxCardinalityPrecisionThreshold)
		reqAgg.TotalConversationMessageCount = msgCount
		aggByAnnotations.Aggs.TotalConversationMessageCount = msgCount
	}
	aggByAnnotations.Aggs.AnnotationHit.TopHits.Source.Includes = []string{MomentAnnotationsPath, "conversation_id", "conversation_message_id", "message_text"}
	var err error
	aggByAnnotations.Filters, err = ConvertMomentGroupToFilter(momentGroup, topicDetails, esQueryConfig)
	reqAgg.Moments = &aggByAnnotations
	return &reqAgg, err
}

func ParseGroupByAttributeStructuresToAgg(
	attributeStructures []*analyticspb.AttributeStructure,
	topicDetails map[string]TopicDetails,
	timeZone string,
	intervalSpec IntervalSpec,
	startDayOfWeekBias int,
	esQueryConfig QueryConfig,
) (ReqAggs, *analyticspb.MomentGroup, error) {
	convCount := CreateReqTotalConversationCount(MaxCardinalityPrecisionThreshold)
	msgCount := CreateReqTotalMessageCount(MaxCardinalityPrecisionThreshold)
	handleTimeSecsSum := createReqTotalHandleTimeSecsSum()
	reqAgg := ReqAggs{
		TotalConversationCount:             convCount,
		TotalConversationHandleTimeSecsSum: handleTimeSecsSum,
	}
	aggByAnnotations := ReqAnnotations{
		Aggs: &ReqAggAnnotation{
			TotalConversationCount:             convCount,
			TotalConversationHandleTimeSecsSum: handleTimeSecsSum,
			AnnotationHit: &ReqAnnotationHit{
				TopHits: ReqTopHits{
					Size: 1,
				},
			},
		},
	}
	if includeConversationMessageCount(esQueryConfig) {
		reqAgg.TotalConversationMessageCount = msgCount
		aggByAnnotations.Aggs.TotalConversationMessageCount = msgCount
	}
	if len(attributeStructures) == 0 {
		return reqAgg, nil, nil
	}
	dateHistogramWeeklyOffset := ParseStartDayOfWeekBiasToOffset(startDayOfWeekBias)
	if len(attributeStructures) == 1 {
		switch attributeStructures[0].AttributeOneof.(type) {
		case *analyticspb.AttributeStructure_AttributeType:
			if at := attributeStructures[0].GetAttributeType(); at == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE {
				reqAgg.TimeRanges = &ReqTimeRanges{
					DateHistogram: buildDateHistogram(timeZone, intervalSpec, dateHistogramWeeklyOffset),
					Aggs: ReqNestedAggCounts{
						TotalConversationCount:             convCount,
						TotalConversationHandleTimeSecsSum: handleTimeSecsSum,
					},
				}
				if includeConversationMessageCount(esQueryConfig) {
					reqAgg.TimeRanges.Aggs.TotalConversationMessageCount = msgCount
				}
			} else if at := attributeStructures[0].GetAttributeType(); at == analyticspb.AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE {
				aggByAnnotations.Terms = &TermsAggs{
					Field: proto.String(languageCodeField),
				}
				aggByAnnotations.Aggs.AnnotationHit = nil
				reqAgg.Moments = &aggByAnnotations
			} else {
				return ReqAggs{}, nil, status.Errorf(codes.InvalidArgument, "AttributeType only support TIME_RANGE and LANGUAGE_CODE")
			}
			return reqAgg, nil, nil
		case *analyticspb.AttributeStructure_MomentGroup:
			aggByAnnotations.Aggs.AnnotationHit.TopHits.Source.Includes = []string{MomentAnnotationsPath}
			reqAgg.Moments = &aggByAnnotations
			var err error
			reqAgg.Moments.Filters, err = ConvertMomentGroupToFilter(attributeStructures[0].GetMomentGroup(), topicDetails, esQueryConfig)
			return reqAgg, attributeStructures[0].GetMomentGroup(), err
		default:
			return ReqAggs{}, nil, status.Errorf(
				codes.InvalidArgument, "`group_by_attribute_structures` only supports at most 2 of "+
					"`AttributeType_ATTRIBUTE_TYPE_TIME_RANGE`, `AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE`, `MomentGroup`")
		}
	}
	if len(attributeStructures) == 2 {
		var (
			attType analyticspb.AttributeType
			moments *analyticspb.MomentGroup
		)
		for _, v := range attributeStructures {
			switch v.AttributeOneof.(type) {
			case *analyticspb.AttributeStructure_AttributeType:
				attType = v.GetAttributeType()
			case *analyticspb.AttributeStructure_MomentGroup:
				moments = v.GetMomentGroup()
			}
		}
		if attType != analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE {
			return ReqAggs{}, nil, status.Errorf(codes.InvalidArgument, "when 2 AttributeStructes are provided, attribute_type.time_range has to be provided.")
		}
		if moments != nil {
			aggByAnnotations.Aggs.AnnotationHit.TopHits.Source.Includes = []string{MomentAnnotationsPath}
			reqAgg.TimeRanges = &ReqTimeRanges{
				DateHistogram: buildDateHistogram(timeZone, intervalSpec, dateHistogramWeeklyOffset),
				Aggs: ReqNestedAggCounts{
					Moments: &aggByAnnotations,
				},
			}
			var err error
			reqAgg.TimeRanges.Aggs.Moments.Filters, err = ConvertMomentGroupToFilter(moments, topicDetails, esQueryConfig)
			return reqAgg, moments, err
		}
	}
	return ReqAggs{}, nil, status.Errorf(codes.InvalidArgument, "No more than 2 `group_by_attribute_structures` is supported.")
}

// buildDateHistogram constructs a date_histogram aggregation for the
// conversation_started_at field. Exactly one of calendar_interval or
// fixed_interval is populated based on the IntervalSpec. The weekly offset is
// only meaningful for calendar week intervals; it is ignored for fixed intervals.
func buildDateHistogram(timeZone string, intervalSpec IntervalSpec, weeklyOffset *string) ReqDateHistogram {
	histogram := ReqDateHistogram{
		Field:    "conversation_started_at",
		TimeZone: timeZone,
	}
	if intervalSpec.Fixed != "" {
		histogram.FixedInterval = intervalSpec.Fixed
		return histogram
	}
	histogram.CalendarInterval = intervalSpec.Calendar
	histogram.Offset = weeklyOffset
	return histogram
}

func ParseStartDayOfWeekBiasToOffset(startDayOfWeekBias int) *string {
	if startDayOfWeekBias == 0 {
		return nil
	}
	offset := fmt.Sprintf("-%dd", startDayOfWeekBias)
	return &offset
}

func ConvertTopicBoolsToConvoFilter(topicBools []*TopicBool) ConvoFilter {
	topicBoolsForMsg, topicBoolsForMsgGroup := []*TopicBool{}, []*TopicBool{}
	for _, topicBool := range topicBools {
		forMsg, forMsgGroup := SplitTopicBoolToMsgAndMsgGroup(topicBool)
		if forMsg != nil {
			topicBoolsForMsg = append(topicBoolsForMsg, forMsg)
		}
		if forMsgGroup != nil {
			topicBoolsForMsgGroup = append(topicBoolsForMsgGroup, forMsgGroup)
		}
	}

	hasChilds := []*HasChild{}
	if len(topicBoolsForMsg) > 0 {
		ffs := []MsgFilter{}
		for _, t := range topicBoolsForMsg {
			ffs = append(ffs, MsgFilter{Bool: t})
		}
		hasChilds = append(hasChilds, &HasChild{
			Type: string(DocumentTypeMessage),
			Query: InnerMsgQuery{
				Bool: &InnerMsgBool{
					Should:             &ffs,
					MinimumShouldMatch: 1,
				},
			},
		})
	}
	if len(topicBoolsForMsgGroup) > 0 {
		ffs := []MsgFilter{}
		for _, t := range topicBoolsForMsgGroup {
			ffs = append(ffs, MsgFilter{Bool: t})
		}
		hasChilds = append(hasChilds, &HasChild{
			Type: string(DocumentTypeMessageGroup),
			Query: InnerMsgQuery{
				Bool: &InnerMsgBool{
					Should:             &ffs,
					MinimumShouldMatch: 1,
				},
			},
		})
	}
	convoFilter := ConvoFilter{}
	if len(hasChilds) == 1 {
		convoFilter = ConvoFilter{
			HasChild: hasChilds[0],
		}
	} else {
		convoFilter = ConvoFilter{
			Bool: &ConvoFilterBool{
				Should: &[]ConvoFilterBoolFilter{},
			},
		}
		for _, hasChild := range hasChilds {
			*convoFilter.Bool.Should = append(*convoFilter.Bool.Should, ConvoFilterBoolFilter{
				HasChild: hasChild,
			})
		}
		convoFilter.Bool.MinimumShouldMatch = 1
	}
	return convoFilter
}

func ConvertMomentGroupToFilter(momentGroup *analyticspb.MomentGroup, topicDetails map[string]TopicDetails, esQueryConfig QueryConfig) (*ReqAnnotationFilters, error) {
	if len(momentGroup.Moments) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty moment group in filter is not allowed")
	}
	momentFilters := ReqAnnotationFilters{}
	for _, moment := range momentGroup.Moments {
		switch {
		case IsInsightsCompositeMoment(moment):
			if _, exist := topicDetails[moment.Name]; !exist {
				return nil, fmt.Errorf("topic details for moment %q does not exists", moment.Name)
			}
			topicBool, err := ConvertTopicDetailToTopicBool(topicDetails[moment.Name], []string{})
			if err != nil {
				return nil, err
			}
			if esQueryConfig.EsDocumentType == DocumentTypeConversation {
				convoFilter := ConvertTopicBoolsToConvoFilter([]*TopicBool{topicBool})
				subFilter := ReqAnnotationSubFilter{
					Bool: ConvoBool{
						Filter: &[]ConvoFilter{convoFilter},
					},
				}
				momentFilters.Filters = append(momentFilters.Filters, subFilter)
			} else {
				subFilter := ReqAnnotationSubFilter{Bool: topicBool}
				momentFilters.Filters = append(momentFilters.Filters, subFilter)
			}

		case moment.Type == momentpb.Moment_CONVERSATION_METADATA:
			if len(momentGroup.MetadataValueAttributes) == 0 {
				return nil, status.Error(codes.InvalidArgument, "requires the MetadataValueAttribute for metadata moment.")
			}
			for index := range momentGroup.MetadataValueAttributes {
				nested, err := convertMetadataValueToFilter(momentGroup, index)
				if err != nil {
					return nil, err
				}
				momentFilters.Filters = append(momentFilters.Filters, ReqAnnotationSubFilter{Nested: &nested})
			}
		case moment.Type == momentpb.Moment_CONVERSATION_OUTCOME:
			if len(momentGroup.OutcomeValueAttributes) == 0 {
				return nil, status.Error(codes.InvalidArgument, "requires the OutcomeValueAttributes for outcome moment.")
			}
			for index := range momentGroup.OutcomeValueAttributes {
				nested, err := convertConversationOutcomeValueToFilter(momentGroup, index)
				if err != nil {
					return nil, err
				}
				momentFilters.Filters = append(momentFilters.Filters, ReqAnnotationSubFilter{Nested: &nested})
			}
		case moment.Type == momentpb.Moment_TOPIC:
			momentName, err := momentpb.ParseMomentName(moment.Name)
			if err != nil {
				return &momentFilters, status.Error(codes.InvalidArgument, err.Error())
			}
			// A grouped-by V3 category's TopicDetails carries its subtree closure; OR-match it. Every
			// other topic matches only its own template id.
			templateIDs := []string{momentName.MomentID}
			if closure := topicDetails[moment.Name].V3SubtreeClosureTemplateIDs; len(closure) > 0 {
				templateIDs = closure
			}
			if esQueryConfig.EsDocumentType == DocumentTypeConversation {
				// Support has child for DocumentTypeConversation. TDv1/v2 topics live on
				// message_group child docs; V3 topics are message-anchored on the "message" child.
				topicChildType := DocumentTypeMessageGroup
				if IsTopicDiscoveryV3Moment(moment) {
					topicChildType = DocumentTypeMessage
				}
				momentFilters.Filters = append(momentFilters.Filters, ReqAnnotationSubFilter{HasChild: &HasChild{
					Type: string(topicChildType),
					Query: InnerMsgQuery{
						Bool: &InnerMsgBool{
							Filter: &[]MsgFilter{
								{
									Nested: &Nested{
										Path: MomentAnnotationsPath,
										Query: &NestedFilter{
											Bool: &NestedFilterBool{
												Filter: &[]NestedFilter{
													{
														Terms: &NestedTermsFilter{MomentTemplateIDs: &templateIDs},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}})
			} else {
				momentFilters.Filters = append(momentFilters.Filters, createReqAnnotationSubFilterFromTemplateIDs(templateIDs))
			}
		default:
			momentName, err := momentpb.ParseMomentName(moment.Name)
			if err != nil {
				return &momentFilters, status.Error(codes.InvalidArgument, err.Error())
			}
			momentFilters.Filters = append(momentFilters.Filters, createReqAnnotationSubFilterFromMomentName(momentName))
		}
	}

	return &momentFilters, nil
}

func createReqAnnotationSubFilterFromMomentName(momentName *momentpb.MomentName) ReqAnnotationSubFilter {
	return createReqAnnotationSubFilterFromTemplateIDs([]string{momentName.MomentID})
}

// createReqAnnotationSubFilterFromTemplateIDs builds a nested moment_annotations filter matching any of
// the given moment template ids.
func createReqAnnotationSubFilterFromTemplateIDs(templateIDs []string) ReqAnnotationSubFilter {
	return ReqAnnotationSubFilter{Nested: &Nested{
		Path: MomentAnnotationsPath,
		Query: &NestedFilter{
			Bool: &NestedFilterBool{
				Filter: &[]NestedFilter{
					{
						Terms: &NestedTermsFilter{MomentTemplateIDs: &templateIDs},
					},
				},
			},
		},
	}}
}

func convertLangCodesToMsgTextFields(langCodes []string) ([]string, error) {
	langCodeMsgTextFields := map[string]interface{}{}
	if len(langCodes) > 0 {
		for _, languageCode := range langCodes {
			msgTextField := msgTextFieldForLanguage(languageCode)
			langCodeMsgTextFields[msgTextField] = nil
		}
	} else {
		for _, languageCode := range listSupportedLangCodes() {
			msgTextField := msgTextFieldForLanguage(languageCode)
			if msgTextField == "" {
				return nil, fmt.Errorf("unknown language code: %v", languageCode)
			}
			langCodeMsgTextFields[msgTextField] = nil
		}
	}
	ret := []string{}
	for k := range langCodeMsgTextFields {
		if k != defaultMsgTextField {
			ret = append(ret, k)
		}
	}
	sort.Strings(ret)
	return ret, nil
}

func msgTextFieldForLanguage(langCode string) string {
	if val, exists := langCodeToMsgTextField[langCode]; exists {
		return val
	}
	return defaultMsgTextField
}

func listSupportedLangCodes() []string {
	langCodes := make([]string, 0, len(langCodeToMsgTextField))
	for langCode := range langCodeToMsgTextField {
		langCodes = append(langCodes, langCode)
	}
	sort.Strings(langCodes)
	return langCodes
}

func IsEmptyTopicDetails(t TopicDetails) bool {
	return len(t.Clusters) == 0 && len(t.ExcludedPhrases) == 0 && len(t.IncludedPhrases) == 0 && t.RegexExpression == ""
}

func CreateReqTotalConversationCount(precisionThreshold int) *CardinalityCount {
	return &CardinalityCount{
		Cardinality: Cardinality{
			Field:              proto.String("conversation_id.keyword"),
			PrecisionThreshold: precisionThreshold,
		},
	}
}

func CreateReqTotalMessageCount(precisionThreshold int) *CardinalityCount {
	return &CardinalityCount{
		Cardinality: Cardinality{
			Script:             &Script{Source: scriptConvoIDAndConvoMsgID},
			PrecisionThreshold: precisionThreshold,
		},
	}
}

func createReqTotalHandleTimeSecsSum() *AggregationOnNumericField {
	return &AggregationOnNumericField{
		Sum: &NumericField{
			Field: proto.String("handle_time_secs"),
		},
	}
}

func nestedFromNestedFilters(ff *[]NestedFilter, esDocPath string) *Nested {
	return &Nested{
		Path: esDocPath,
		Query: &NestedFilter{
			Bool: &NestedFilterBool{
				Filter: ff,
			},
		},
	}
}

func shouldApplyWhitespaceAnalyzer(keyword string) bool {
	punctuations := []string{"&", "-", "/"}
	res := false
	for _, p := range punctuations {
		if strings.Contains(keyword, p) {
			res = true
			break
		}
	}
	return res
}

func includeConversationMessageCount(esQueryConfig QueryConfig) bool {
	return !esQueryConfig.SkipConversationMessageCount
}

// Convert service action type to db action type. The enum value are different.
// eg enum value of GW in service is 15, but 13 in DB (postgres/ES).
func serviceToDBActionType(actionType actionpb.Action_Type) (dbactionpb.Action_ActionType, error) {
	switch actionType {
	case actionpb.Action_HINT:
		return dbactionpb.Action_HINT, nil
	case actionpb.Action_GUIDED_WORKFLOW_HINT:
		return dbactionpb.Action_GUIDED_WORKFLOW_HINT, nil
	case actionpb.Action_KB_HINT:
		return dbactionpb.Action_KB_HINT, nil
	case actionpb.Action_MANAGER_ALERT:
		return dbactionpb.Action_MANAGER_ALERT, nil
	case actionpb.Action_CHECKLIST:
		return dbactionpb.Action_NEW_CHECKLIST, nil
	default:
		return dbactionpb.Action_ACTION_TYPE_UNSPECIFIED, status.Errorf(
			codes.InvalidArgument, "action type %v not supported", actionType)
	}
}

func functionScoreForOnHoldCount(crit *analyticspb.Attribute_ConversationHoldsCriterion) (ConvoFilter, error) {
	if crit.HoldsCountRange.LowerBound == nil {
		return ConvoFilter{}, fmt.Errorf("lower_bound should be set")
	}
	minCount := crit.HoldsCountRange.LowerBound.Value
	if crit.HoldsCountRange.LowerBound.IsExclusive {
		minCount += 1
	}

	maxCount := int32(9999)
	if crit.HoldsCountRange.UpperBound != nil {
		maxCount = crit.HoldsCountRange.UpperBound.Value
		if crit.HoldsCountRange.UpperBound.IsExclusive {
			maxCount -= 1
		}
	}
	if minCount > maxCount {
		return ConvoFilter{}, fmt.Errorf("derived upper_bound %d is lower than lower_bound %d from criteria %v", maxCount, minCount, crit)
	}
	if minCount < 0 || maxCount < 0 {
		return ConvoFilter{}, fmt.Errorf("derived upper_bound %d or lower_bound %d is negative from criteria %v", maxCount, minCount, crit)
	}

	convoEventTypeForHold := int(dbeventpb.Event_CONVERSATION_HOLD)

	fsQuery := func(lowerBound, upperBound int32) json.RawMessage {
		f := fmt.Sprintf(`{
			"min_score":%d,
			"boost_mode":"replace",
			"script_score":{
				"script":{
					"source":"if (_score > params['upper_bound']) { return 0; } return _score;",
					"lang":"painless",
					"params":{
						"upper_bound":%d
					}
				}
			},
			"query":{
				"nested":{
					"path":"conversation_events",
					"boost":1,
					"score_mode":"sum",
					"query":{
						"terms":{
							"conversation_events.type":[
								%d
							]
						}
					}
				}
			}
		}`, lowerBound, upperBound, convoEventTypeForHold)

		return json.RawMessage(f)
	}

	holdExist := Nested{
		Path: conversationEventsPath,
		Query: &NestedFilter{
			Bool: &NestedFilterBool{
				Filter: &[]NestedFilter{
					{
						Terms: &NestedTermsFilter{
							ConversationEventTypes: &[]int{convoEventTypeForHold},
						},
					},
				},
			},
		},
	}

	if minCount == 0 {
		should := []ConvoFilterBoolFilter{
			{
				Bool: &ConvoFilterBoolFilterBool{
					MustNot: &ConvoFilterBoolFilterBoolFilter{
						Nested: &holdExist,
					},
				},
			},
		}
		if maxCount > 0 {
			f := fsQuery(1, maxCount)
			should = append(should, ConvoFilterBoolFilter{
				FunctionScore: &f,
			})
		}
		return ConvoFilter{
			Bool: &ConvoFilterBool{
				Should:             &should,
				MinimumShouldMatch: 1,
			},
		}, nil
	} else {
		f := fsQuery(minCount, maxCount)
		return ConvoFilter{
			FunctionScore: &f,
		}, nil
	}
}

func holdsCountRangeFilter(crit *analyticspb.Attribute_ConversationHoldsCriterion) (ConvoFilter, error) {
	if crit.HoldsCountRange == nil {
		return ConvoFilter{}, fmt.Errorf("holds_count_range should be set")
	}
	if crit.HoldsCountRange.LowerBound == nil {
		return ConvoFilter{}, fmt.Errorf("lower_bound should be set")
	}
	minCount := crit.HoldsCountRange.LowerBound.Value
	if crit.HoldsCountRange.LowerBound.IsExclusive {
		minCount++
	}

	var maxCount *int32
	if crit.HoldsCountRange.UpperBound != nil {
		upperBound := crit.HoldsCountRange.UpperBound.Value
		if crit.HoldsCountRange.UpperBound.IsExclusive {
			upperBound--
		}
		maxCount = &upperBound
	}

	if maxCount != nil && minCount > *maxCount {
		return ConvoFilter{}, fmt.Errorf("derived upper_bound %d is lower than lower_bound %d from criteria %v", *maxCount, minCount, crit)
	}
	if minCount < 0 || (maxCount != nil && *maxCount < 0) {
		return ConvoFilter{}, fmt.Errorf("derived lower_bound %d or upper_bound is negative from criteria %v", minCount, crit)
	}

	minCountFloat := float64(minCount)
	var maxCountFloat *float64
	if maxCount != nil {
		v := float64(*maxCount)
		maxCountFloat = &v
	}

	rangeFilter := ConvoFilterBoolFilter{
		Range: &RangeFilter{
			NumHolds: &FloatRange{GTE: &minCountFloat, LTE: maxCountFloat},
		},
	}

	// When minCount is 0, also match documents where num_holds field doesn't exist (pre-backfill documents).
	if minCount == 0 {
		should := []ConvoFilterBoolFilter{
			rangeFilter,
			{
				Bool: &ConvoFilterBoolFilterBool{
					MustNot: &ConvoFilterBoolFilterBoolFilter{
						Exists: &Field{Field: "num_holds"},
					},
				},
			},
		}
		return ConvoFilter{
			Bool: &ConvoFilterBool{
				Should:             &should,
				MinimumShouldMatch: 1,
			},
		}, nil
	}

	return ConvoFilter{
		Range: &RangeFilter{
			NumHolds: &FloatRange{GTE: &minCountFloat, LTE: maxCountFloat},
		},
	}, nil
}

func isExactKeyword(ss string) bool {
	return strings.HasPrefix(ss, "\"") && strings.HasSuffix(ss, "\"")
}

// aiAgentFeedbackNestedFilter builds the nested query matching conversations
// whose `comments[]` contains a comment with type = AI_AGENT_FEEDBACK.
func aiAgentFeedbackNestedFilter() ConvoFilter {
	return ConvoFilter{
		Nested: &Nested{
			Path: commentsPath,
			Query: &NestedFilter{
				Terms: &NestedTermsFilter{
					CommentTypes: &[]int{int(collaborationpb.Comment_AI_AGENT_FEEDBACK)},
				},
			},
		},
	}
}
