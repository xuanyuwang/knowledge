package elasticsearch

import "fmt"

// ElasticSearch Response.
type ResTotalConversationCount struct {
	Value float64
}

type ResTotalConversationHandleTimeSecsSum struct {
	Value float64
}

type ResTotalConversationMessageCount struct {
	Value float64
}

type ResTimeRangeBucket struct {
	TotalConversationCount             ResTotalConversationCount             `mapstructure:"total_conversation_count"`
	TotalConversationMessageCount      ResTotalConversationMessageCount      `mapstructure:"total_conversation_message_count"`
	TotalConversationHandleTimeSecsSum ResTotalConversationHandleTimeSecsSum `mapstructure:"total_conversation_handle_time_secs_sum"`
	Moments                            ResAnnotation                         `mapstructure:"moments"`
	Key                                float64
	DocCount                           int `mapstructure:"doc_count,omitempty"`
}

type ResMetadataValue struct {
	StringValue  string  `mapstructure:"stringValue"`
	BooleanValue bool    `mapstructure:"booleanValue"`
	NumberValue  float32 `mapstructure:"numberValue"`
}

type ResConversationMetadataPayload struct {
	Values []ResMetadataValue `mapstructure:"values"`
}

type ResTopicPayload struct {
	ConversationReason string `mapstructure:"conversationReason,omitempty"`
	ExtractedQuestion  string `mapstructure:"extractedQuestion,omitempty"`
}

type ResMomentPayload struct {
	ConversationMetadataPayload ResConversationMetadataPayload `mapstructure:"conversationMetadataPayload,omitempty"`
	TopicPayload                ResTopicPayload                `mapstructure:"topicPayload,omitempty"`
}

type ResMomentAnnotation struct {
	MomentTemplateID string            `mapstructure:"moment_template_id"`
	Taxonomy         string            `mapstructure:"taxonomy"`
	DisplayName      string            `mapstructure:"display_name"`
	Type             int               `mapstructure:"type"`
	Payload          *ResMomentPayload `mapstructure:"payload,omitempty"`
}

type MessageTextWithLang struct {
	// German
	DeDe string `mapstructure:"de-DE"`
	// English
	EnAu string `mapstructure:"en-AU"`
	EnGb string `mapstructure:"en-GB"`
	EnNz string `mapstructure:"en-NZ"`
	EnUs string `mapstructure:"en-US"`
	// Spanish
	Es419 string `mapstructure:"es-419"`
	EsEs  string `mapstructure:"es-ES"`
	EsMx  string `mapstructure:"es-MX"`
	// French
	FrCa string `mapstructure:"fr-CA"`
	FrFr string `mapstructure:"fr-FR"`
	// Italian
	ItIt string `mapstructure:"it-IT"`
	// Dutch
	NlNl string `mapstructure:"nl-NL"`
	// Portuguese
	PtBr string `mapstructure:"pt-BR"`
	PtPt string `mapstructure:"pt-PT"`
	// Swedish
	SvSe string `mapstructure:"sv-SE"`
}

type ResSource struct {
	Agent                     int    `mapstructure:"agent"`
	AgentName                 string `mapstructure:"agent_name"`
	AgentUserID               string `mapstructure:"agent_user_id"`
	ChatID                    int    `mapstructure:"chat_id"`
	MessageID                 int    `mapstructure:"message_id"`
	MessageIDs                []int  `mapstructure:"message_ids"`
	ConversationID            string `mapstructure:"conversation_id"`
	PlatformConversationID    string `mapstructure:"platform_conversation_id"`
	PlatformAccountID         string `mapstructure:"platform_account_id"`
	ConversationMessageID     string `mapstructure:"conversation_message_id"`
	ConversationCorrelationID string `mapstructure:"conversation_correlation_id"`
	// Using "any" because conversation_to_message_join field is a string type for conversation
	// and map type for message and message group.
	ConversationToMessageJoin any                   `mapstructure:"conversation_to_message_join"`
	CreatedAt                 string                `mapstructure:"created_at"`
	PlatformTimestamp         string                `mapstructure:"platform_timestamp"`
	MessageText               string                `mapstructure:"message_text"`
	MessageForwardPosition    int                   `mapstructure:"message_forward_position"`
	MessageTextWithLang       MessageTextWithLang   `mapstructure:"message_text_with_lang"`
	MessageCount              int                   `mapstructure:"message_count"`
	SpeakerRole               string                `mapstructure:"speaker_role"`
	MomentAnnotations         []ResMomentAnnotation `mapstructure:"moment_annotations"`
	ConversationCreatedAt     string                `mapstructure:"conversation_created_at"`
	ConversationStartedAt     string                `mapstructure:"conversation_started_at"`
	ConversationEndedAt       string                `mapstructure:"conversation_ended_at"`
}

type ResHighlight struct {
	// Default
	MessageText []string `mapstructure:"message_text"`

	// For whitespace analyzer
	MessageTextWithWhitespaceAanalyzer []string `mapstructure:"message_text_whitespace"`

	// For custom analyzer
	MessageTextWithCustomAnalyzerEnglish    []string `mapstructure:"message_text_with_custom_analyzer.english"`
	MessageTextWithCustomAnalyzerSpanish    []string `mapstructure:"message_text_with_custom_analyzer.spanish"`
	MessageTextWithCustomAnalyzerFrench     []string `mapstructure:"message_text_with_custom_analyzer.french"`
	MessageTextWithCustomAnalyzerItalian    []string `mapstructure:"message_text_with_custom_analyzer.italian"`
	MessageTextWithCustomAnalyzerGerman     []string `mapstructure:"message_text_with_custom_analyzer.german"`
	MessageTextWithCustomAnalyzerDutch      []string `mapstructure:"message_text_with_custom_analyzer.dutch"`
	MessageTextWithCustomAnalyzerPortuguese []string `mapstructure:"message_text_with_custom_analyzer.portuguese"`
	MessageTextWithCustomAnalyzerRussian    []string `mapstructure:"message_text_with_custom_analyzer.russian"`
	MessageTextWithCustomAnalyzerThai       []string `mapstructure:"message_text_with_custom_analyzer.thai"`
}

type ResHit struct {
	Source    ResSource    `mapstructure:"_source"`
	Highlight ResHighlight `mapstructure:"highlight"`
	Sort      []any        `mapstructure:"sort"`
}

func (s ResSource) ReadConversationToMessageJoin() (string, error) {
	switch val := s.ConversationToMessageJoin.(type) {
	case string:
		return val, nil
	case map[string]interface{}:
		return val["name"].(string), nil
	default:
		return "", fmt.Errorf("failed parsing 'conversation_to_message_join' %v", val)
	}
}

type ResTotal struct {
	Value int `mapstructure:"value"`
}

type ResHits struct {
	Hits  []ResHit `mapstructure:"hits"`
	Total ResTotal `mapstructure:"total"`
}

type ResAnnotationHit struct {
	Hits ResHits `mapstructure:"hits"`
}

type ResAnnotationBucket struct {
	TotalConversationCount             ResTotalConversationCount             `mapstructure:"total_conversation_count"`
	TotalConversationHandleTimeSecsSum ResTotalConversationHandleTimeSecsSum `mapstructure:"total_conversation_handle_time_secs_sum"`
	TotalConversationMessageCount      ResTotalConversationMessageCount      `mapstructure:"total_conversation_message_count"`
	DocCount                           int32                                 `mapstructure:"doc_count"`
	AnnotationHit                      *ResAnnotationHit                     `mapstructure:"annotation_hit"`
	BucketKey                          string                                `mapstructure:"key,omitempty"`
}

type ResTimeRange struct {
	Buckets []ResTimeRangeBucket `mapstructure:"buckets"`
}

type ResBucket struct {
	Key any `mapstructure:"key,omitempty"`
}

type ResPayloadValue struct {
	Buckets []ResBucket `mapstructure:"buckets,omitempty"`
	Count   float64     `mapstructure:"count,omitempty"`
	Min     float64     `mapstructure:"min,omitempty"`
	Max     float64     `mapstructure:"max,omitempty"`
	Avg     float64     `mapstructure:"avg,omitempty"`
	Sum     float64     `mapstructure:"sum,omitempty"`
}

type ResMomentAnnotations struct {
	PayloadValue ResPayloadValue `json:"payload_value" mapstructure:"payload_value"`
}

type ResAnnotation struct {
	Buckets           *[]ResAnnotationBucket `mapstructure:"buckets,omitempty"`
	MomentAnnotations *ResMomentAnnotations  `mapstructure:"moment_annotations,omitempty"`
}

type ResAggregations struct {
	Moments                            *ResAnnotation                        `mapstructure:"moments,omitempty"`
	TimeRanges                         ResTimeRange                          `mapstructure:"time_ranges,omitempty"`
	TotalConversationCount             ResTotalConversationCount             `mapstructure:"total_conversation_count,omitempty"`
	TotalConversationHandleTimeSecsSum ResTotalConversationHandleTimeSecsSum `mapstructure:"total_conversation_handle_time_secs_sum"`
	TotalConversationMessageCount      ResTotalConversationMessageCount      `mapstructure:"total_conversation_message_count,omitempty"`
}

type Response struct {
	Aggregations ResAggregations `mapstructure:"aggregations"`
	Hits         ResHits         `mapstructure:"hits"`
}
