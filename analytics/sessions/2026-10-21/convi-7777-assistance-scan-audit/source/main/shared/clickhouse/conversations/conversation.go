package conversations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	clickhouseeventspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/clickhouse/events"
	internalconvpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/conversation"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	workflowpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/workflowautomation"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/framework/stats"
	"github.com/cresta/shared-go/util/envflag"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"

	dbactionpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/action"
	dbcommonpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/common"
	dbeventpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/event"
	dbmomentpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/moment"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	clickhouseclient "github.com/cresta/go-servers/shared/clickhouse/conversations/client"
	sharedclickhouseevents "github.com/cresta/go-servers/shared/clickhouse/events"
	clickhouseeventsclient "github.com/cresta/go-servers/shared/clickhouse/events/client"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	conv "github.com/cresta/go-servers/shared/conversation"
	"github.com/cresta/go-servers/shared/converters"
	"github.com/cresta/go-servers/shared/dao"
	"github.com/cresta/go-servers/shared/db/condition"
	"github.com/cresta/go-servers/shared/db/registry"
	"github.com/cresta/go-servers/shared/scoring"
	sharedsummarization "github.com/cresta/go-servers/shared/summarization"
	"github.com/cresta/go-servers/shared/utils"
)

var (
	clickHouseShardCount                                 = envflag.Int("CLICK_HOUSE_SHARD_COUNT", 3, "The count of ClickHouse shards in cluster.")
	clickHouseEnableKnowledgeSessionID                   = envflag.Bool("CLICK_HOUSE_ENABLE_KNOWLEDGE_SESSION_ID", true, "Whether to enable knowledge_session_id in clickhouse.")
	clickHouseEnableWritingConversationIntoEventsCluster = envflag.Bool("CLICK_HOUSE_ENABLE_WRITING_CONVERSATION_INTO_EVENT_CLUSTER", true, "Whether to enable writing conversation data into events cluster.")
	clickHouseEnableMomentAnnotationPayload              = envflag.Bool("CLICK_HOUSE_ENABLE_MOMENT_ANNOTATION_PAYLOAD", false, "Whether to enable extracting and storing moment annotation payloads in clickhouse.")

	// Note: Do not call it clickhouse.client.* because then Datadog will consider the metric
	// non-custom and you can't, e.g., change the tags.
	indexMetricCountWithStatus = stats.NewMetric("clickhouse_client.index.status")
	indexMetricInflightCount   = stats.NewMetric("clickhouse_client.index.inflight_count")
	indexMetricDuration        = stats.NewMetric("clickhouse_client.index.duration_ms")

	defaultTime = time.Unix(0, 0)
)

type scorecardTemplateRevisionKey struct {
	templateID string
	revision   string
}

type ConversationWithLabelsRow struct {
	CustomerID            string    `ch:"customer_id"`
	ProfileID             string    `ch:"profile_id"`
	AgentUserID           string    `ch:"agent_user_id"`
	ConversationID        string    `ch:"conversation_id"`
	ConversationStartTime time.Time `ch:"conversation_start_time"`
	ConversationEndTime   time.Time `ch:"conversation_end_time"`
	UsecaseID             string    `ch:"usecase_id"`
	WithAgentAssistance   bool      `ch:"has_agent_assistance"`
	UpdateTime            time.Time `ch:"update_time"`
	CreateTime            time.Time `ch:"create_time"`
	MessageID             string    `ch:"message_id"`
	MessageAgentUserID    string    `ch:"message_agent_user_id"`
	MessageTime           time.Time `ch:"message_time"`
}

type MessageRow struct {
	AgentUserID              string    `ch:"agent_user_id"`
	ConversationDurationSecs int32     `ch:"conversation_duration_secs"`
	ConversationEndTime      time.Time `ch:"conversation_end_time"`
	ConversationID           string    `ch:"conversation_id"`
	ConversationSource       int32     `ch:"conversation_source"`
	ConversationStartTime    time.Time `ch:"conversation_start_time"`
	CreateTime               time.Time `ch:"create_time"`
	CustomerID               string    `ch:"customer_id"`
	EndTimeMs                int32     `ch:"end_time_ms"`
	HandleTimeSecs           int32     `ch:"handle_time_secs"`
	IsDevUser                bool      `ch:"is_dev_user"`
	IsVoiceMail              bool      `ch:"is_voice_mail"`
	LanguageCode             string    `ch:"language_code"`
	MessageID                string    `ch:"message_id"`
	MessageText              string    `ch:"message_text"`
	PlatformSequenceID       int32     `ch:"platform_sequence_id"`
	PlatformTime             time.Time `ch:"platform_time"`
	ProfileID                string    `ch:"profile_id"`
	RaiseHandState           int32     `ch:"raise_hand_state"`
	SpeakerRole              string    `ch:"speaker_role"`
	StartTimeMs              int32     `ch:"start_time_ms"`
	UpdateTime               time.Time `ch:"update_time"`
	UsecaseID                string    `ch:"usecase_id"`
	MessageTextLength        int32     `ch:"message_text_length"`
	MessageWordCount         int32     `ch:"message_word_count"`
}

type MomentAnnotationRow struct {
	AdherenceActionAdherenceType      int32     `ch:"adherence_action_adherence_type"`
	AdherenceActionAnnotationID       string    `ch:"adherence_action_annotation_id"`
	AdherenceActionDetailedType       int32     `ch:"adherence_action_detailed_type"`
	AdherenceActionID                 string    `ch:"adherence_action_id"`
	AdherenceActionPolicyID           string    `ch:"adherence_action_policy_id"`
	AdherenceActionPolicySourceType   int32     `ch:"adherence_action_policy_source_type"`
	AdherenceActionType               int32     `ch:"adherence_action_type"`
	AdherenceMomentAdherenceType      int32     `ch:"adherence_moment_adherence_type"`
	AdherenceMomentAnnotationID       string    `ch:"adherence_moment_annotation_id"`
	AdherenceMomentDetailedType       int32     `ch:"adherence_moment_detailed_type"`
	AdherenceMomentPolicyID           string    `ch:"adherence_moment_policy_id"`
	AdherenceMomentPolicySourceType   int32     `ch:"adherence_moment_policy_source_type"`
	AdherenceMomentType               int32     `ch:"adherence_moment_type"`
	AdherenceType                     int32     `ch:"adherence_type"`
	AgentUserID                       string    `ch:"agent_user_id"`
	BehaviorID                        string    `ch:"behavior_id"`
	ConversationDurationSecs          int32     `ch:"conversation_duration_secs"`
	ConversationEndTime               time.Time `ch:"conversation_end_time"`
	ConversationID                    string    `ch:"conversation_id"`
	ConversationSource                int32     `ch:"conversation_source"`
	ConversationStartTime             time.Time `ch:"conversation_start_time"`
	CreateTime                        time.Time `ch:"create_time"`
	CustomerID                        string    `ch:"customer_id"`
	IsDevUser                         bool      `ch:"is_dev_user"`
	IsVoiceMail                       bool      `ch:"is_voice_mail"`
	LabelsHidingInPerformanceInsights bool      `ch:"labels.hiding_in_performance_insights"`
	LanguageCode                      string    `ch:"language_code"`
	MessageID                         string    `ch:"message_id"`
	MomentAnnotationID                string    `ch:"moment_annotation_id"`
	MomentDetailedType                int32     `ch:"moment_detailed_type"`
	MomentTemplateID                  string    `ch:"moment_template_id"`
	MomentType                        int32     `ch:"moment_type"`
	PolicyID                          string    `ch:"policy_id"`
	PolicySourceType                  int32     `ch:"policy_source_type"`
	ProfileID                         string    `ch:"profile_id"`
	RaiseHandState                    int32     `ch:"raise_hand_state"`
	Taxonomy                          string    `ch:"taxonomy"`
	TaxonomyState                     int32     `ch:"taxonomy_state"`
	UpdateTime                        time.Time `ch:"update_time"`
	UsecaseID                         string    `ch:"usecase_id"`
	CreatorUserID                     string    `ch:"creator_user_id"`
	// For knowloedge moment annotations only.
	KnowledgePayloadKnowledgeSessionID string `ch:"knowledge_payload_knowledge_session_id"`
	// For conversation metadata annotations only.
	MetadataType        int32   `ch:"metadata_type"`
	MetadataBoolValue   bool    `ch:"metadata_bool_value"`
	MetadataNumberValue float64 `ch:"metadata_number_value"`
	MetadataStringValue string  `ch:"metadata_string_value"`
	MetadataSource      int32   `ch:"metadata_source"`
	// For generic moment annotation payload
	MomentAnnotationPayload string `ch:"moment_annotation_payload"`
}

type ActionAnnotationRow struct {
	ActionAnnotationID                        string    `ch:"action_annotation_id"`
	ActionDetailedType                        int32     `ch:"action_detailed_type"`
	ActionID                                  string    `ch:"action_id"`
	ActionType                                int32     `ch:"action_type"`
	AdherenceType                             int32     `ch:"adherence_type"`
	AgentUserID                               string    `ch:"agent_user_id"`
	ConversationDurationSecs                  int32     `ch:"conversation_duration_secs"`
	ConversationEndTime                       time.Time `ch:"conversation_end_time"`
	ConversationID                            string    `ch:"conversation_id"`
	ConversationSource                        int32     `ch:"conversation_source"`
	ConversationStartTime                     time.Time `ch:"conversation_start_time"`
	CreateTime                                time.Time `ch:"create_time"`
	CustomerID                                string    `ch:"customer_id"`
	IsDevUser                                 bool      `ch:"is_dev_user"`
	IsVoiceMail                               bool      `ch:"is_voice_mail"`
	LanguageCode                              string    `ch:"language_code"`
	MessageID                                 string    `ch:"message_id"`
	PayloadManagerWhisperManagerUserID        string    `ch:"payload.manager_whisper.manager_user_id"`
	PolicyID                                  string    `ch:"policy_id"`
	PolicySourceType                          int32     `ch:"policy_source_type"`
	ProfileID                                 string    `ch:"profile_id"`
	RaiseHandState                            int32     `ch:"raise_hand_state"`
	Taxonomy                                  string    `ch:"taxonomy"`
	TaxonomyState                             int32     `ch:"taxonomy_state"`
	UpdateTime                                time.Time `ch:"update_time"`
	UsecaseID                                 string    `ch:"usecase_id"`
	KnowledgePayloadKnowledgeSessionID        string    `ch:"knowledge_payload_knowledge_session_id"`
	SummarizationPayloadIsValidSummary        bool      `ch:"summarization_payload_is_valid_summary"`
	PayloadWorkflowAutomationHintDefinitionID string    `ch:"payload.workflow_automation_hint.workflow_automation_definition_id"`
}

type ConversationEventRow struct {
	ActionAdherenceType                        int32     `ch:"action_adherence_type"`
	ActionAnnotationID                         string    `ch:"action_annotation_id"`
	ActionDetailedType                         int32     `ch:"action_detailed_type"`
	ActionID                                   string    `ch:"action_id"`
	ActionPolicyID                             string    `ch:"action_policy_id"`
	ActionPolicySourceType                     int32     `ch:"action_policy_source_type"`
	ActionType                                 int32     `ch:"action_type"`
	AgentUserID                                string    `ch:"agent_user_id"`
	ConversationDurationSecs                   int32     `ch:"conversation_duration_secs"`
	ConversationEndTime                        time.Time `ch:"conversation_end_time"`
	ConversationID                             string    `ch:"conversation_id"`
	ConversationSource                         int32     `ch:"conversation_source"`
	ConversationStartTime                      time.Time `ch:"conversation_start_time"`
	CreateTime                                 time.Time `ch:"create_time"`
	CustomerID                                 string    `ch:"customer_id"`
	EventID                                    string    `ch:"event_id"`
	EventType                                  int32     `ch:"event_type"`
	IsDevUser                                  bool      `ch:"is_dev_user"`
	IsVoiceMail                                bool      `ch:"is_voice_mail"`
	LanguageCode                               string    `ch:"language_code"`
	MessageID                                  string    `ch:"message_id"`
	ProfileID                                  string    `ch:"profile_id"`
	RaiseHandState                             int32     `ch:"raise_hand_state"`
	UpdateTime                                 time.Time `ch:"update_time"`
	UsecaseID                                  string    `ch:"usecase_id"`
	KnowledgePayloadKnowledgeSessionID         string    `ch:"knowledge_payload_knowledge_session_id"`
	SummarizationPayloadDataType               int32     `ch:"summarization_payload_data_type"`
	PayloadUsedKbDocumentDocID                 string    `ch:"payload.used_kb_document.doc_id"`
	PayloadUsedKbDocumentDocType               int32     `ch:"payload.used_kb_document.doc_type"`
	PayloadUsedKbDocumentSessionID             string    `ch:"payload.used_kb_document.session_id"`
	PayloadUsedKbSearchSessionID               string    `ch:"payload.used_kb_search.session_id"`
	PayloadUsedNoteTakingDataType              int32     `ch:"payload.used_note_taking.data_type"`
	PayloadUsedSummarizationDataType           int32     `ch:"payload.used_summarization.data_type"`
	PayloadWorkflowAutomationRunID             string    `ch:"payload.workflow_automation.workflow_automation_run_id"`
	PayloadWorkflowAutomationDefinitionID      string    `ch:"payload.workflow_automation.workflow_automation_definition_id"`
	PayloadWorkflowAutomationDefinitionVersion int32     `ch:"payload.workflow_automation.workflow_automation_definition_version"`
	PayloadWorkflowAutomationTriggerSource     int32     `ch:"payload.workflow_automation.trigger_source"`
	PayloadWorkflowAutomationResultStatus      int32     `ch:"payload.workflow_automation.result_status"`
}

type ScorecardRow struct {
	AcknowledgeComment         string    `ch:"acknowledge_comment"`
	AgentUserID                string    `ch:"agent_user_id"`
	AiScoreTime                time.Time `ch:"ai_score_time"`
	AutoFailed                 bool      `ch:"auto_failed"`
	CoachingPlanID             string    `ch:"coaching_plan_id"`
	ConversationDurationBinsID int32     `ch:"conversation_duration_bins_id"`
	ConversationDurationSecs   int32     `ch:"conversation_duration_secs"`
	ConversationID             string    `ch:"conversation_id"`
	CreatorUserID              string    `ch:"creator_user_id"`
	CustomerID                 string    `ch:"customer_id"`
	IsDevUser                  bool      `ch:"is_dev_user"`
	IsVoiceMail                bool      `ch:"is_voice_mail"`
	LastUpdatorUserID          string    `ch:"last_updator_user_id"`
	ManuallyScored             bool      `ch:"manually_scored"`
	ProfileID                  string    `ch:"profile_id"`
	PublishTime                time.Time `ch:"publish_time"`
	PublisherUserID            string    `ch:"publisher_user_id"`
	Score                      float64   `ch:"score"`
	ScorecardAcknowledgeTime   time.Time `ch:"scorecard_acknowledge_time"`
	ScorecardCreateTime        time.Time `ch:"scorecard_create_time"`
	ScorecardID                string    `ch:"scorecard_id"`
	ScorecardLastUpdateTime    time.Time `ch:"scorecard_last_update_time"`
	ScorecardSubmitTime        time.Time `ch:"scorecard_submit_time"`
	ScorecardTemplateID        string    `ch:"scorecard_template_id"`
	ScorecardTemplateRevision  string    `ch:"scorecard_template_revision"`
	ScorecardTime              time.Time `ch:"scorecard_time"`
	SubmitterUserID            string    `ch:"submitter_user_id"`
	UpdateTime                 time.Time `ch:"update_time"`
	UsecaseID                  string    `ch:"usecase_id"`
}

// Score will replace ScorecardScore.
type ScoreRow struct {
	AcknowledgeComment         string    `ch:"acknowledge_comment"`
	AgentUserID                string    `ch:"agent_user_id"`
	AiScoreTime                time.Time `ch:"ai_score_time"`
	AiScored                   bool      `ch:"ai_scored"`
	AiValue                    float64   `ch:"ai_value"`
	AutoFailed                 bool      `ch:"auto_failed"`
	CoachingPlanID             string    `ch:"coaching_plan_id"`
	ConversationDurationBinsID int32     `ch:"conversation_duration_bins_id"`
	ConversationDurationSecs   int32     `ch:"conversation_duration_secs"`
	ConversationID             string    `ch:"conversation_id"`
	ConversationStartTime      time.Time `ch:"conversation_start_time"`
	CreatorUserID              string    `ch:"creator_user_id"`
	CriterionID                string    `ch:"criterion_id"`
	CustomerID                 string    `ch:"customer_id"`
	FloatWeight                float64   `ch:"float_weight"`
	IsDevUser                  bool      `ch:"is_dev_user"`
	IsVoiceMail                bool      `ch:"is_voice_mail"`
	LanguageCode               string    `ch:"language_code"`
	LastUpdatorUserID          string    `ch:"last_updator_user_id"`
	ManuallyScored             bool      `ch:"manually_scored"`
	MaxValue                   float64   `ch:"max_value"`
	NotApplicable              bool      `ch:"not_applicable"`
	NumericValue               float64   `ch:"numeric_value"`
	PercentageValue            float64   `ch:"percentage_value"`
	ProfileID                  string    `ch:"profile_id"`
	ScoreID                    string    `ch:"score_id"`
	ScorecardAcknowledgeTime   time.Time `ch:"scorecard_acknowledge_time"`
	ScorecardCreateTime        time.Time `ch:"scorecard_create_time"`
	ScorecardID                string    `ch:"scorecard_id"`
	ScorecardLastUpdateTime    time.Time `ch:"scorecard_last_update_time"`
	ScorecardPublishTime       time.Time `ch:"scorecard_publish_time"`
	ScorecardScore             float64   `ch:"scorecard_score"`
	ScorecardSubmitTime        time.Time `ch:"scorecard_submit_time"`
	ScorecardTemplateID        string    `ch:"scorecard_template_id"`
	ScorecardTemplateRevision  string    `ch:"scorecard_template_revision"`
	ScorecardTime              time.Time `ch:"scorecard_time"`
	SubmitterUserID            string    `ch:"submitter_user_id"`
	TextValue                  string    `ch:"text_value"`
	UpdateTime                 time.Time `ch:"update_time"`
	UsecaseID                  string    `ch:"usecase_id"`
	Weight                     int32     `ch:"weight"`
}

type DeleteConversationQueueRow struct {
	ConversationID string    `ch:"conversation_id"`
	EnqueueTime    time.Time `ch:"enqueue_time"`
	TableName      string    `ch:"table_name"`
	CutOffTime     time.Time `ch:"cut_off_time"`
}

type ClickhouseRows struct {
	Conversations      []*clickhouseshared.ConversationRow
	Messages           []*MessageRow
	MomentAnnotations  []*MomentAnnotationRow
	ActionAnnotations  []*ActionAnnotationRow
	ConversationEvents []*ConversationEventRow
	Scorecards         []*ScorecardRow
	Scores             []*ScoreRow
}

type ConversationMessageID struct {
	ConverastionID string
	MessageID      string
}

type ScorecardScoreID struct {
	ScorecardID string
	ScoreID     string
}

type ScorecardID struct {
	ScorecardID string
}

type ScorecardExtra struct {
	ConversationDurationBinsID int32
	IsDevUser                  bool
	IsVoiceMail                bool
	ScorecardTime              time.Time
}

type (
	ResourceIDColumn  string
	ResourceTable     string
	ResourceViewTable string
)

const (
	ResourceIDColumnConversation      ResourceIDColumn = "conversation_id"
	ResourceIDColumnMomentAnnotation  ResourceIDColumn = "moment_annotation_id"
	ResourceIDColumnActionAnnotation  ResourceIDColumn = "action_annotation_id"
	ResourceIDColumnConversationEvent ResourceIDColumn = "event_id"
	// Note: When adding a table here please don't forget to update AllTables below.
	ResourceTableConversation          ResourceTable     = "conversation"
	ResourceTableMessage               ResourceTable     = "message"
	ResourceTableMomentAnnotation      ResourceTable     = "moment_annotation"
	ResourceTableActionAnnotation      ResourceTable     = "action_annotation"
	ResourceTableConversationEvent     ResourceTable     = "conversation_event"
	ResourceTableScorecardScore        ResourceTable     = "scorecard_score"
	ResourceTableScorecard             ResourceTable     = "scorecard"
	ResourceTableScore                 ResourceTable     = "score"
	ResourceViewTableConversation      ResourceViewTable = "conversation_d_mv_by_conversation"
	ResourceViewTableMomentAnnotation  ResourceViewTable = "moment_annotation_d_mv_by_conversation"
	ResourceViewTableActionAnnotation  ResourceViewTable = "action_annotation_d_mv_by_conversation"
	ResourceViewTableConversationEvent ResourceViewTable = "conversation_event_d_mv_by_conversation"
	ResourceViewTableScorecard         ResourceViewTable = "scorecard_d_mv_by_conversation"
	ResourceViewTableScore             ResourceViewTable = "score_d_mv_by_conversation"
)

var AllTables = [...]ResourceTable{
	ResourceTableConversation,
	ResourceTableMessage,
	ResourceTableMomentAnnotation,
	ResourceTableActionAnnotation,
	ResourceTableConversationEvent,
	ResourceTableScorecardScore,
	ResourceTableScorecard,
	ResourceTableScore,
}

var AbleToDeleteTables = []ResourceTable{
	ResourceTableConversation,
	ResourceTableMessage,
	ResourceTableMomentAnnotation,
	ResourceTableActionAnnotation,
	ResourceTableConversationEvent,
	ResourceTableScorecard,
	ResourceTableScore,
}

const (
	timestampFormat             = "2006-01-02 15:04:05"
	baseConversationFilterQuery = "conversation_id IN (?)"
)

// MomentAnnotationPayload is the payload of the moment annotation.
//
// It implements json.Marshaler and json.Unmarshaler interfaces
type MomentAnnotationPayloadProto dbmomentpb.MomentAnnotationPayload

func (m *MomentAnnotationPayloadProto) MarshalJSON() ([]byte, error) {
	return protojson.Marshal((*dbmomentpb.MomentAnnotationPayload)(m))
}

func (m *MomentAnnotationPayloadProto) UnmarshalJSON(content []byte) error {
	if string(content) == "null" {
		// no-op
		return nil
	}
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(content, (*dbmomentpb.MomentAnnotationPayload)(m))
}

func GetClickHouseShardCount() int {
	return *clickHouseShardCount
}

func FindShardForConversation(
	ctx context.Context,
	conn driver.Conn,
	resourceViewTableColumn string,
	resourceViewTable ResourceViewTable,
	conversationID string,
) (int, error) {
	rows, err := clickhouseshared.QueryWithRetry(
		ctx, conn,
		fmt.Sprintf("SELECT toUnixTimestamp(%s) %% %d AS shard FROM %s WHERE conversation_id = ? LIMIT 1",
			resourceViewTableColumn,
			*clickHouseShardCount,
			resourceViewTable),
		conversationID)
	if err != nil {
		return -1, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	res := -1
	for rows.Next() {
		var shard uint8
		vals := []interface{}{&shard}
		if err := rows.Scan(vals...); err != nil {
			return 0, err
		}
		if int(shard) < *clickHouseShardCount {
			res = int(shard)
			break
		}
	}
	return res, nil
}

func FindShardsForConversations(
	ctx context.Context,
	conn driver.Conn,
	resourceViewTableColumn string,
	resourceViewTable ResourceViewTable,
	conversationIDs []string,
) ([]int, error) {
	shards := []int{}
	rows, err := clickhouseshared.QueryWithRetry(
		ctx, conn,
		fmt.Sprintf("SELECT DISTINCT toUnixTimestamp(%s) %% %d AS shard FROM %s WHERE conversation_id IN ?",
			resourceViewTableColumn,
			*clickHouseShardCount,
			resourceViewTable),
		conversationIDs)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var shard uint8
		vals := []interface{}{&shard}
		if err := rows.Scan(vals...); err != nil {
			return shards, err
		}
		if int(shard) < *clickHouseShardCount {
			shards = append(shards, int(shard))
		}
	}
	return shards, nil
}

func ReadExistingResourceIDsFromConversations(
	ctx context.Context, conn driver.Conn,
	resourceIDColumn ResourceIDColumn,
	resourceViewTable ResourceViewTable,
	conversationIDs []string,
) ([]string, error) {
	rows, err := clickhouseshared.QueryWithRetry(
		ctx, conn,
		fmt.Sprintf("SELECT %s FROM %s WHERE conversation_id IN (?)",
			resourceIDColumn, resourceViewTable),
		conversationIDs)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	resourceIDs := []string{}
	for rows.Next() {
		id := ""
		vals := []interface{}{&id}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		resourceIDs = append(resourceIDs, id)
	}
	return resourceIDs, nil
}

func ReadExistingConversationMessageIDs(
	ctx context.Context, conn driver.Conn,
	conversationIDs []string,
) ([]string, error) {
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, `
		SELECT conversation_id, message_id
		FROM message_d_mv_by_conversation
		WHERE conversation_id IN (?)
	`, conversationIDs)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	convMsgIDs := []string{}
	for rows.Next() {
		convMsgID := ConversationMessageID{}
		vals := []interface{}{&convMsgID.ConverastionID, &convMsgID.MessageID}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		convMsgIDs = append(convMsgIDs, fmt.Sprintf("%s_%s", convMsgID.ConverastionID, convMsgID.MessageID))
	}
	return convMsgIDs, nil
}

func ReadExistingScorecardScoreIDs(
	ctx context.Context, conn driver.Conn,
	conversationIDs []string,
) ([]string, error) {
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, `
		SELECT scorecard_id, score_id
		FROM scorecard_score_d_mv_by_conversation
		WHERE conversation_id IN (?)
	`, conversationIDs)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	scorecardScoreIDs := []string{}
	for rows.Next() {
		scorecardScoreID := ScorecardScoreID{}
		vals := []interface{}{&scorecardScoreID.ScorecardID, &scorecardScoreID.ScoreID}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		scorecardScoreIDs = append(scorecardScoreIDs, fmt.Sprintf("%s_%s", scorecardScoreID.ScorecardID, scorecardScoreID.ScoreID))
	}
	return scorecardScoreIDs, nil
}

func ReadExistingScorecardIDs(
	ctx context.Context, conn driver.Conn,
	conversationIDs []string,
) ([]string, error) {
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, `
		SELECT scorecard_id
		FROM scorecard_d_mv_by_conversation
		WHERE conversation_id IN (?)
	`, conversationIDs)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	scorecardIDs := []string{}
	for rows.Next() {
		scorecardID := ScorecardID{}
		vals := []interface{}{&scorecardID.ScorecardID}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		scorecardIDs = append(scorecardIDs, scorecardID.ScorecardID)
	}
	return scorecardIDs, nil
}

func ReadExistingScoreIDs(
	ctx context.Context, conn driver.Conn,
	conversationIDs []string,
) ([]string, error) {
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, `
		SELECT scorecard_id, score_id
		FROM score_d_mv_by_conversation
		WHERE conversation_id IN (?)
	`, conversationIDs)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	scorecardScoreIDs := []string{}
	for rows.Next() {
		scorecardScoreID := ScorecardScoreID{}
		vals := []interface{}{&scorecardScoreID.ScorecardID, &scorecardScoreID.ScoreID}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		scorecardScoreIDs = append(scorecardScoreIDs, fmt.Sprintf("%s_%s", scorecardScoreID.ScorecardID, scorecardScoreID.ScoreID))
	}
	return scorecardScoreIDs, nil
}

func ReadConversationIDsFromDeleteQueue(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	customerID, profileID string,
	startTime, endTime time.Time,
) ([]string, error) {
	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Clickhouse: %w", err)
	}

	// The empty table name means ALL tables (as the existing behavior). We need this
	// scenario to avoid an unexpected release from job service so that this cronjob
	// deletes the conversation from all tables without cut-off time.
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, `
		SELECT
			conversation_id
		FROM 
			delete_conversation_queue_d
		WHERE 
			enqueue_time >= ? AND enqueue_time < ?
			AND table_name = ''
	`, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	conversationIDs := []string{}
	for rows.Next() {
		cid := ""
		vals := []interface{}{&cid}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		if !slices.Contains(conversationIDs, cid) {
			conversationIDs = append(conversationIDs, cid)
		}
	}
	return conversationIDs, nil
}

func ReadConversationsWithTableAndCutOffTimeFromDeleteQueue(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	customerID, profileID string,
	startTime, endTime time.Time,
) (map[string]map[string][]string, error) {
	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Clickhouse: %w", err)
	}

	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, `
		SELECT
			conversation_id, table_name, cut_off_time
		FROM 
			delete_conversation_queue_d
		WHERE 
			enqueue_time >= ? AND enqueue_time < ?
			AND table_name != ''
	`, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	tableToCutOffTimeToConvoIDs := map[string]map[string][]string{}
	for rows.Next() {
		cid := ""
		table := ""
		cutOffTime := time.Now()
		vals := []interface{}{&cid, &table, &cutOffTime}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		if _, ok := tableToCutOffTimeToConvoIDs[table]; !ok {
			tableToCutOffTimeToConvoIDs[table] = map[string][]string{}
		}
		formatCutOffTime := cutOffTime.Format(timestampFormat)
		if _, ok := tableToCutOffTimeToConvoIDs[table][formatCutOffTime]; !ok {
			tableToCutOffTimeToConvoIDs[table][formatCutOffTime] = []string{}
		}
		tableToCutOffTimeToConvoIDs[table][formatCutOffTime] = append(tableToCutOffTimeToConvoIDs[table][formatCutOffTime], cid)
	}
	return tableToCutOffTimeToConvoIDs, nil
}

func WriteClickHouseConversation(
	ctx context.Context,
	convoClient *clickhouseclient.ClickHouseClient,
	eventClient *clickhouseeventsclient.ClickhouseEventsClient,
	conversationName *conversationpb.ConversationName,
	existingClickhouseResourceIDs map[internalconvpb.BatchIndexConversationsRequest_ClickhouseTable][]string,
	clickhouseTables []internalconvpb.BatchIndexConversationsRequest_ClickhouseTable,
	conversationEventsFromClickhouseEvents []*sharedclickhouseevents.ConversationEventRow,
	useDatabaseWriteReplica bool,
) (err error) {
	startTime := time.Now()
	indexMetricInflightCount.Incr(ctx)
	defer func() {
		var statusTag string
		if ctx.Err() == context.Canceled {
			statusTag = "status:cancelled"
		} else {
			statusTag = fmt.Sprintf("status:%v", strings.ToLower(status.Code(err).String()))
		}
		indexMetricCountWithStatus.Incr(ctx, statusTag)
		indexMetricDuration.Distribution(ctx, float64(time.Since(startTime).Milliseconds()), statusTag)
		indexMetricInflightCount.Decr(ctx)
	}()

	// Read conversation data from Postgres DB.
	//
	// When useDatabaseWriteReplica is true, reads are routed to the write replica (primary) to
	// avoid replication lag. This is required for the index-after-mutation path, which runs
	// immediately after the conversation/messages were written on the primary and would otherwise
	// race the read replica (e.g. StartConversation indexing an email conversation that is closed
	// at creation, producing "not found" errors on a lagging replica).
	convDao := clickhouseConversationDao(ctx, convoClient, useDatabaseWriteReplica)
	conversation, err := convDao.GetConversationByResourceName(conversationName)
	if err != nil {
		return fmt.Errorf("failed reading DB convo %v: %w", conversationName, err)
	}
	if !conversation.EndedAt.Valid {
		// Return early if the conversation doesn't end yet.
		return nil
	}

	appsDB := clickhouseAppsDB(ctx, convoClient, useDatabaseWriteReplica)
	messages, messagesByID, err := readMessagesByConversation(ctx, convoClient, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return fmt.Errorf("failed reading DB messages for convo %v: %w", conversationName, err)
	}
	agentUsersByID, err := readAgentUsers(ctx, convoClient, conversation, messagesByID)
	if err != nil {
		return fmt.Errorf("failed reading DB agent user for convo %v: %w", conversationName, err)
	}
	momentAnnotationsByID, err := ReadMomentAnnotationsForConversation(appsDB, conversationName)
	if err != nil {
		return fmt.Errorf("failed reading DB moment annotations for convo %v: %w", conversationName, err)
	}
	momentTemplatesByID, err := readTemplatesForConversation(appsDB, conversationName, momentAnnotationsByID)
	if err != nil {
		return fmt.Errorf("failed reading DB moment templates for convo %v: %w", conversationName, err)
	}
	actionAnnotationsByID, err := readActionAnnotations(ctx, convoClient, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return fmt.Errorf("failed reading DB action annotations for convo %v: %w", conversationName, err)
	}
	conversationEventsByID, err := readConversationEvents(ctx, convoClient, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return fmt.Errorf("failed reading DB events for convo %v: %w", conversationName, err)
	}
	existingConversationEventIDs := existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_CONVERSATION_EVENT]
	workflowAutomationRunMetadataByID, err := readWorkflowAutomationRunMetadata(appsDB, conversationName, conversationEventsByID, existingConversationEventIDs)
	if err != nil {
		return err
	}
	scorecards, err := readScorecards(ctx, convoClient, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return fmt.Errorf("failed reading DB scorecards for convo %v: %w", conversationName, err)
	}
	scoresByScorecard, err := readScoresForScorecards(ctx, convoClient, conversationName, scorecards, useDatabaseWriteReplica)
	if err != nil {
		return fmt.Errorf("failed reading DB scores for convo %v: %w", conversationName, err)
	}
	isVoiceMail := HasVoiceMailMoments(momentAnnotationsByID)
	policiesByID, err := readPoliciesByIDFromActionAnnotations(ctx, convoClient, conversationName, actionAnnotationsByID, useDatabaseWriteReplica)
	if err != nil {
		return fmt.Errorf("failed reading DB policies for convo %v: %w", conversationName, err)
	}
	conversationDurationSecs := int32(0)
	conversationEndTime := conversation.StartedAt
	if conversation.EndedAt.Valid {
		conversationDurationSecs = int32(conversation.EndedAt.Time.Sub(conversation.StartedAt).Seconds())
		conversationEndTime = conversation.EndedAt.Time
	}

	templateStructuresByRevision, err := readTemplateStructuresForScorecards(appsDB, conversationName, scorecards)
	if err != nil {
		return fmt.Errorf("failed reading template structures for convo %v: %w", conversationName, err)
	}

	// Build score rows from director.scores
	allScoreRows := []*ScoreRow{}
	for _, scorecard := range scorecards {
		directorScores := scoresByScorecard[scorecard.ResourceID]
		if len(directorScores) > 0 {
			templateStructure, ok := templateStructuresByRevision[scorecardTemplateRevisionKey{
				templateID: scorecard.TemplateID,
				revision:   scorecard.TemplateRevision,
			}]
			if !ok {
				return fmt.Errorf("failed finding template structure for scorecard %s template %s revision %s", scorecard.ResourceID, scorecard.TemplateID, scorecard.TemplateRevision)
			}
			isDevUser := false
			if agentUser, found := agentUsersByID[scorecard.AgentUserID]; found {
				isDevUser = agentUser.IsDevUser
			}
			scoreRows, err := BuildScoreRowsFromDirectorScores(directorScores, scorecard, conversation, false, isDevUser, isVoiceMail, &templateStructure)
			if err != nil {
				return fmt.Errorf("failed building score rows for scorecard %s: %w", scorecard.ResourceID, err)
			}
			allScoreRows = append(allScoreRows, scoreRows...)
		}
	}

	conn, err := convoClient.ConnectOrGetConn(conversationName.GetCustomerID(), conversationName.GetProfileID())
	if err != nil {
		return fmt.Errorf("failed to connect to ClickHouse (conversations): %w", err)
	}
	convoClient.GetLogger().Debugf(ctx, "[%v] Write conversation to ClickHouse: the conversations conn (pool) has %d open (max: %d) and %d idle (max: %d) threads", time.Now().Format("2006-01-02 15:04:05.000"), conn.Stats().Open, conn.Stats().MaxOpenConns, conn.Stats().Idle, conn.Stats().MaxIdleConns)

	// Write the new Clickhouse rows.
	convoClient.GetLogger().Debugf(ctx, "[%v] Write conversation to ClickHouse: starting to write clickhouse for conversation %s", time.Now().Format("2006-01-02 15:04:05.000"), conversationName)
	if err := writeConversation(ctx, convoClient.GetLogger(), conn, existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_CONVERSATION], conversation, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime); err != nil {
		return fmt.Errorf("failed writing ClickHouse conversation for convo %v: %w", conversationName, err)
	}

	if *clickHouseEnableWritingConversationIntoEventsCluster {
		eventsConn, err := eventClient.ConnectOrGetConn(conversationName.GetCustomerID(), conversationName.GetProfileID())
		if err != nil {
			return fmt.Errorf("failed to connect to ClickHouse (events): %w", err)
		}
		eventClient.GetLogger().Debugf(ctx, "[%v] Write conversation to ClickHouse: the events conn (pool) has %d open (max: %d) and %d idle (max: %d) threads", time.Now().Format("2006-01-02 15:04:05.000"), eventsConn.Stats().Open, eventsConn.Stats().MaxOpenConns, eventsConn.Stats().Idle, eventsConn.Stats().MaxIdleConns)

		// Also double write the conversation row into clickhouse events cluster.
		// Note: we need this for filtering conversation events.
		if err := writeConversation(ctx, eventClient.GetLogger(), eventsConn, existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_CONVERSATION], conversation, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime); err != nil {
			return fmt.Errorf("failed writing ClickHouse conversation (into events cluster) for convo %v: %w", conversationName, err)
		}
	}

	if err := writeMessages(ctx, convoClient.GetLogger(), conn, existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_MESSAGE], conversation, messages, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime); err != nil {
		return fmt.Errorf("failed writing ClickHouse message for convo %v: %w", conversationName, err)
	}
	if err := writeMomentAnnotations(ctx, convoClient.GetLogger(), conn, existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_MOMENT_ANNOTATION], conversation, messagesByID, agentUsersByID, momentAnnotationsByID, actionAnnotationsByID, momentTemplatesByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime); err != nil {
		return fmt.Errorf("failed writing ClickHouse moment annotation for convo %v: %w", conversationName, err)
	}
	if err := writeActionAnnotations(ctx, convoClient.GetLogger(), conn, existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_ACTION_ANNOTATION], conversation, messagesByID, agentUsersByID, actionAnnotationsByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime); err != nil {
		return fmt.Errorf("failed writing ClickHouse action annotation for convo %v: %w", conversationName, err)
	}
	if err := writeConversationEvents(ctx, convoClient.GetLogger(), conn, existingConversationEventIDs, conversation, messagesByID, agentUsersByID, conversationEventsByID, workflowAutomationRunMetadataByID, conversationEventsFromClickhouseEvents, actionAnnotationsByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime); err != nil {
		return fmt.Errorf("failed writing ClickHouse conversation event for convo %v: %w", conversationName, err)
	}

	// Write scores to ClickHouse
	if len(allScoreRows) > 0 {
		if err := BatchWriteScores(ctx, convoClient.GetLogger(), conn, allScoreRows); err != nil {
			return fmt.Errorf("failed writing ClickHouse scores for convo %v: %w", conversationName, err)
		}
	}

	scorecardRows := BuildScorecardRows(existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_SCORECARD], scorecards, false /* isProcessScorecard */, allScoreRows)
	if len(scorecardRows) > 0 {
		if err := BatchWriteScorecards(ctx, convoClient.GetLogger(), conn, scorecardRows); err != nil {
			return fmt.Errorf("failed writing ClickHouse scorecards for convo %v: %w", conversationName, err)
		}
	}

	convoClient.GetLogger().Debugf(ctx, "[%v] Write conversation to ClickHouse: finished writing clickhouse for conversation %s", time.Now().Format("2006-01-02 15:04:05.000"), conversationName)
	return nil
}

func BuildClickHouseDataForBatchWrite(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	conversationName *conversationpb.ConversationName,
	existingClickhouseResourceIDs map[internalconvpb.BatchIndexConversationsRequest_ClickhouseTable][]string,
	conversationEventsFromClickhouseEvents []*sharedclickhouseevents.ConversationEventRow,
	useDatabaseWriteReplica bool,
) (clickhouseRows *ClickhouseRows, err error) {
	startTime := time.Now()
	indexMetricInflightCount.Incr(ctx)
	defer func() {
		var statusTag string
		if ctx.Err() == context.Canceled {
			statusTag = "status:cancelled"
		} else {
			statusTag = fmt.Sprintf("status:%v", strings.ToLower(status.Code(err).String()))
		}
		indexMetricCountWithStatus.Incr(ctx, statusTag)
		indexMetricDuration.Distribution(ctx, float64(time.Since(startTime).Milliseconds()), statusTag)
		indexMetricInflightCount.Decr(ctx)
	}()

	// Read conversation data from Postgres DB.
	convDao := clickhouseConversationDao(ctx, c, useDatabaseWriteReplica)
	conversation, err := convDao.GetConversationByResourceName(conversationName)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB convo %v: %w", conversationName, err)
	}
	messages, messagesByID, err := readMessagesByConversation(ctx, c, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB messages for convo %v: %w", conversationName, err)
	}
	agentUsersByID, err := readAgentUsers(ctx, c, conversation, messagesByID)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB agent user for convo %v: %w", conversationName, err)
	}
	appsDB := clickhouseAppsDB(ctx, c, useDatabaseWriteReplica)
	momentAnnotationsByID, err := ReadMomentAnnotationsForConversation(appsDB, conversationName)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB moment annotations for convo %v: %w", conversationName, err)
	}
	momentTemplatesByID, err := readTemplatesForConversation(appsDB, conversationName, momentAnnotationsByID)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB moment templates for convo %v: %w", conversationName, err)
	}
	actionAnnotationsByID, err := readActionAnnotations(ctx, c, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB action annotations for convo %v: %w", conversationName, err)
	}
	conversationEventsByID, err := readConversationEvents(ctx, c, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB events for convo %v: %w", conversationName, err)
	}
	existingConversationEventIDs := existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_CONVERSATION_EVENT]
	workflowAutomationRunMetadataByID, err := readWorkflowAutomationRunMetadata(appsDB, conversationName, conversationEventsByID, existingConversationEventIDs)
	if err != nil {
		return nil, err
	}
	scorecards, err := readScorecards(ctx, c, conversationName, useDatabaseWriteReplica)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB scorecards for convo %v: %w", conversationName, err)
	}
	scoresByScorecard, err := readScoresForScorecards(ctx, c, conversationName, scorecards, useDatabaseWriteReplica)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB scores for convo %v: %w", conversationName, err)
	}
	isVoiceMail := HasVoiceMailMoments(momentAnnotationsByID)
	policiesByID, err := readPoliciesByIDFromActionAnnotations(ctx, c, conversationName, actionAnnotationsByID, useDatabaseWriteReplica)
	if err != nil {
		return nil, fmt.Errorf("failed reading DB policies for convo %v: %w", conversationName, err)
	}
	conversationDurationSecs := int32(0)
	conversationEndTime := conversation.StartedAt
	if conversation.EndedAt.Valid {
		conversationDurationSecs = int32(conversation.EndedAt.Time.Sub(conversation.StartedAt).Seconds())
		conversationEndTime = conversation.EndedAt.Time
	}

	// Build clickhouse rows by table.
	clickhouseRows = &ClickhouseRows{}
	conversationRow := buildConversationRow(existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_CONVERSATION], conversation, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if conversationRow != nil {
		clickhouseRows.Conversations = append(clickhouseRows.Conversations, conversationRow)
	}
	messagesRows := buildMessageRows(existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_MESSAGE], conversation, messages, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if len(messagesRows) > 0 {
		clickhouseRows.Messages = append(clickhouseRows.Messages, messagesRows...)
	}
	momentAnnotationRows, err := buildMomentAnnotationRows(ctx, c.GetLogger(), existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_MOMENT_ANNOTATION], conversation, messagesByID, agentUsersByID, momentAnnotationsByID, actionAnnotationsByID, momentTemplatesByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if err != nil {
		return nil, err
	}
	if len(momentAnnotationRows) > 0 {
		clickhouseRows.MomentAnnotations = append(clickhouseRows.MomentAnnotations, momentAnnotationRows...)
	}
	actionAnnotationRows, err := buildActionAnnotationRows(ctx, c.GetLogger(), existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_ACTION_ANNOTATION], conversation, messagesByID, agentUsersByID, actionAnnotationsByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if err != nil {
		return nil, err
	}
	if len(actionAnnotationRows) > 0 {
		clickhouseRows.ActionAnnotations = append(clickhouseRows.ActionAnnotations, actionAnnotationRows...)
	}
	conversationEventRows, err := buildConversationEventRows(existingConversationEventIDs, conversation, messagesByID, agentUsersByID, conversationEventsByID, workflowAutomationRunMetadataByID, conversationEventsFromClickhouseEvents, actionAnnotationsByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if err != nil {
		return nil, err
	}
	if len(conversationEventRows) > 0 {
		clickhouseRows.ConversationEvents = append(clickhouseRows.ConversationEvents, conversationEventRows...)
	}

	templateStructuresByRevision, err := readTemplateStructuresForScorecards(appsDB, conversationName, scorecards)
	if err != nil {
		return nil, fmt.Errorf("failed reading template structures for convo %v: %w", conversationName, err)
	}

	// Build score rows from director.scores
	allScoreRows := []*ScoreRow{}
	for _, scorecard := range scorecards {
		directorScores := scoresByScorecard[scorecard.ResourceID]
		if len(directorScores) > 0 {
			templateStructure, ok := templateStructuresByRevision[scorecardTemplateRevisionKey{
				templateID: scorecard.TemplateID,
				revision:   scorecard.TemplateRevision,
			}]
			if !ok {
				return nil, fmt.Errorf("failed finding template structure for scorecard %s template %s revision %s", scorecard.ResourceID, scorecard.TemplateID, scorecard.TemplateRevision)
			}
			isDevUser := false
			if agentUser, found := agentUsersByID[scorecard.AgentUserID]; found {
				isDevUser = agentUser.IsDevUser
			}
			scoreRows, err := BuildScoreRowsFromDirectorScores(directorScores, scorecard, conversation, false, isDevUser, isVoiceMail, &templateStructure)
			if err != nil {
				return nil, fmt.Errorf("failed building score rows for scorecard %s: %w", scorecard.ResourceID, err)
			}
			allScoreRows = append(allScoreRows, scoreRows...)
		}
	}
	if len(allScoreRows) > 0 {
		clickhouseRows.Scores = append(clickhouseRows.Scores, allScoreRows...)
	}

	scorecardRows := BuildScorecardRows(existingClickhouseResourceIDs[internalconvpb.BatchIndexConversationsRequest_CLICKHOUSE_TABLE_SCORECARD], scorecards, false /* isProcessScorecard */, allScoreRows)
	if len(scorecardRows) > 0 {
		clickhouseRows.Scorecards = append(clickhouseRows.Scorecards, scorecardRows...)
	}

	c.GetLogger().Debugf(ctx, "[%v] BuildClickHouseDataForBatchWrite: built %d conversations, %d messages, %d moment annotations, %d action annotations, %d conversation events, %d scores, %d scorecards.",
		time.Now().Format("2006-01-02 15:04:05.000"), len(clickhouseRows.Conversations), len(clickhouseRows.Messages), len(clickhouseRows.MomentAnnotations), len(clickhouseRows.ActionAnnotations), len(clickhouseRows.ConversationEvents), len(clickhouseRows.Scores), len(clickhouseRows.Scorecards))
	return clickhouseRows, nil
}

func BatchWriteConversationWithLabels(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*ConversationWithLabelsRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversations/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO conversation_with_labels_d (%s)", clickhouseshared.ExtractCHFields(ConversationWithLabelsRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation_with_labels to ClickHouse: failed to abort batch for conversation %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteConversations(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*clickhouseshared.ConversationRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for conversation %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteMessages(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*MessageRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO message_d (%s)", clickhouseshared.ExtractCHFields(MessageRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for message %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteMomentAnnotations(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*MomentAnnotationRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO moment_annotation_d (%s)", clickhouseshared.ExtractCHFields(MomentAnnotationRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for moment annotation %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteActionAnnotations(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*ActionAnnotationRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO action_annotation_d (%s)", clickhouseshared.ExtractCHFields(ActionAnnotationRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for action annotation %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteConversationEvents(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*ConversationEventRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO conversation_event_d (%s)", clickhouseshared.ExtractCHFields(ConversationEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for conversation event %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteScorecards(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*ScorecardRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO scorecard_d (%s)", clickhouseshared.ExtractCHFields(ScorecardRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for scorecard %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchWriteScores(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chRows []*ScoreRow,
) error {
	if len(chRows) == 0 {
		return nil
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO score_d (%s)", clickhouseshared.ExtractCHFields(ScoreRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for score %v", err)
			}
		}
	}()

	for _, row := range chRows {
		err = batch.AppendStruct(row)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchEnqueueConversationsToDeleteQueue(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	customerID, profileID string,
	conversationIDs []string,
) error {
	if len(conversationIDs) == 0 {
		return nil
	}

	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		c.GetLogger().Errorf(ctx, "cannot connect ClickHouse for customer %v profile %v: %v", customerID, profileID, err)
		return err
	}
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO delete_conversation_queue_d (%s)", clickhouseshared.ExtractCHFields(DeleteConversationQueueRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for delete conversation queue %v", err)
			}
		}
	}()

	for _, cid := range conversationIDs {
		err = batch.AppendStruct(&DeleteConversationQueueRow{
			ConversationID: cid,
			EnqueueTime:    time.Now(),
			// The table_name and cut_off_time here don't matter (won't be used).
			TableName:  "",
			CutOffTime: time.Now(),
		})
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func BatchEnqueueConversationsToDeleteQueueWithTable(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	customerID, profileID string,
	conversationIDs []string,
	tables []ResourceTable,
	cutOffTime time.Time,
) error {
	if len(conversationIDs) == 0 {
		return nil
	}

	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		c.GetLogger().Errorf(ctx, "cannot connect ClickHouse for customer %v profile %v: %v", customerID, profileID, err)
		return err
	}
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO delete_conversation_queue_d (%s)", clickhouseshared.ExtractCHFields(DeleteConversationQueueRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for delete conversation queue %v", err)
			}
		}
	}()

	for _, t := range tables {
		if !slices.Contains(AbleToDeleteTables, t) {
			continue
		}
		for _, cid := range conversationIDs {
			err = batch.AppendStruct(&DeleteConversationQueueRow{
				ConversationID: cid,
				EnqueueTime:    time.Now(),
				TableName:      string(t),
				CutOffTime:     cutOffTime,
			})
			if err != nil {
				return err
			}
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func DeleteFromClickhouseWhere(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	customerID, profileID string,
	tables []ResourceTable,
	condition string,
	conditionArgs []interface{},
) error {
	c.GetLogger().Debugf(ctx, "DeleteFromClickhouseWhere for customers/%s/profiles/%s for tables %s on cluster %s with condition %s", customerID, profileID, tables, c.GetClusterName(), condition)

	for i := 0; i < GetClickHouseShardCount(); i++ {
		connOnShard, err := c.ConnectOrGetConnOnShard(customerID, profileID, i)
		if err != nil {
			return err
		}
		for _, table := range tables {
			query := fmt.Sprintf(`DELETE FROM %s WHERE %s SETTINGS replication_wait_for_inactive_replica_timeout = 0`, table, condition)
			if err := clickhouseshared.ExecWithRetry(ctx, connOnShard, query, conditionArgs...); err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteOnShardFromClickhouseWhere(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	shards []int,
	customerID, profileID string,
	tableName ResourceTable,
	conditions condition.Conditions,
) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE %s`, tableName, conditions.QueryString())
	c.GetLogger().Debugf(ctx, "[%v] DeleteOnShardFromClickhouseWhere within %v shards for customer %v profile %v: %s", time.Now().Format("2006-01-02 15:04:05.000"), len(shards), customerID, profileID, query)

	// Delete clickhouse on each shard in parallel.
	// Note: the concurrency is up to the max shard count.
	errorChannels := make([]chan error, len(shards))
	for i, shard := range shards {
		channel := make(chan error, 1)
		go utils.AvoidPanic(ctx, func() {
			c.GetLogger().Debugf(ctx, "[%v] deleting on shard %v for customer %v profile %v", time.Now().Format("2006-01-02 15:04:05.000"), shard, customerID, profileID)
			connOnShard, err := c.ConnectOrGetConnOnShard(customerID, profileID, shard)
			if err != nil {
				c.GetLogger().Errorf(ctx, "cannot connect ClickHouse on shard %v for customer %v profile %v: %v", shard, customerID, profileID, err)
				channel <- err
			}
			if err := clickhouseshared.ExecWithRetry(ctx, connOnShard, query, conditions.QueryParams()...); err != nil {
				c.GetLogger().Errorf(ctx, "failed to delete ClickHouse on shard %v for customer %v profile %v: %v", shard, customerID, profileID, err)
				channel <- err
			}
			channel <- nil
		})()
		errorChannels[i] = channel
	}

	// Wrap up the errors (if any) for all per-shard deletion.
	results := make([]error, len(shards))
	for i, channel := range errorChannels {
		results[i] = <-channel
	}
	c.GetLogger().Debugf(ctx, "[%v] DeleteOnShardFromClickhouseWhere done within %v shards for customer %v profile %v", time.Now().Format("2006-01-02 15:04:05.000"), len(shards), customerID, profileID)
	errs := []error{}
	for _, err := range results {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Delete the conversation data in Conversation Cluster.
func DeleteConversationsFromClickhouse(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	tables []ResourceTable,
	customerID, profileID string,
	conversationIds []string,
	conversationStartTimeMin, conversationStartTimeMax *time.Time,
) error {
	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		c.GetLogger().Errorf(ctx, "cannot connect ClickHouse for customer %v profile %v: %v", customerID, profileID, err)
		return err
	}
	// Find the right shards to delete data from.
	// The conversation data are sharded by toUnixTimestamp(conversation_start_time).
	shards, err := FindShardsForConversations(
		ctx, conn, "conversation_start_time", ResourceViewTableConversation, conversationIds)
	if err != nil {
		c.GetLogger().Errorf(ctx, " %v: %v", conversationIds, err)
		return err
	}
	if len(shards) == 0 {
		// If no shards found, the deletion will be skipped.
		//
		// Note: this could happen very rarely (e.g. on Carmax on 03/09/2024), when the initial indexing didn't find the conversation
		// from read-only DB replica and indexing was actually skipped. In this case, we don't need to delete the existing
		// scorecard scores in clickhouse. We could directly go forward with insertion.
		c.GetLogger().Infof(ctx, "found no conversations to delete for customer %v and profile %v", customerID, profileID)
		return nil
	}

	// Delete conversation data from all tables in clickhouse.
	conditions := condition.New(baseConversationFilterQuery, conversationIds)
	if conversationStartTimeMin != nil && !conversationStartTimeMin.IsZero() && conversationStartTimeMax != nil && !conversationStartTimeMax.IsZero() {
		conditions = condition.AppendSQLAndClause(conditions,
			"conversation_start_time >= ? AND conversation_start_time < ?",
			conversationStartTimeMin.Format(timestampFormat), conversationStartTimeMax.Format(timestampFormat),
		)
	}

	if slices.Contains(tables, ResourceTableConversation) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableConversation, conditions); err != nil {
			return err
		}
	}
	if slices.Contains(tables, ResourceTableMessage) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableMessage, conditions); err != nil {
			return err
		}
	}
	if slices.Contains(tables, ResourceTableMomentAnnotation) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableMomentAnnotation, conditions); err != nil {
			return err
		}
	}
	if slices.Contains(tables, ResourceTableActionAnnotation) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableActionAnnotation, conditions); err != nil {
			return err
		}
	}
	if slices.Contains(tables, ResourceTableConversationEvent) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableConversationEvent, conditions); err != nil {
			return err
		}
	}

	if slices.Contains(tables, ResourceTableScorecard) {
		shards, err := FindShardsForConversations(
			ctx, conn, "scorecard_time", ResourceViewTableScorecard, conversationIds)
		if err != nil {
			c.GetLogger().Errorf(ctx, " %v: %v", conversationIds, err)
			return err
		}
		if len(shards) == 0 {
			c.GetLogger().Infof(ctx, "found no conversations to delete for customer %v and profile %v", customerID, profileID)
			return nil
		}

		conditions := condition.New(baseConversationFilterQuery, conversationIds)
		if conversationStartTimeMin != nil && !conversationStartTimeMin.IsZero() && conversationStartTimeMax != nil && !conversationStartTimeMax.IsZero() {
			conditions = condition.AppendSQLAndClause(conditions,
				"scorecard_time >= ? AND scorecard_time < ?",
				conversationStartTimeMin.Format(timestampFormat), conversationStartTimeMax.Format(timestampFormat),
			)
		}
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableScorecard, conditions); err != nil {
			return err
		}
	}
	if slices.Contains(tables, ResourceTableScore) {
		shards, err := FindShardsForConversations(
			ctx, conn, "scorecard_time", ResourceViewTableScore, conversationIds)
		if err != nil {
			c.GetLogger().Errorf(ctx, " %v: %v", conversationIds, err)
			return err
		}
		if len(shards) == 0 {
			c.GetLogger().Infof(ctx, "found no conversations to delete for customer %v and profile %v", customerID, profileID)
			return nil
		}

		conditions := condition.New(baseConversationFilterQuery, conversationIds)
		if conversationStartTimeMin != nil && !conversationStartTimeMin.IsZero() && conversationStartTimeMax != nil && !conversationStartTimeMax.IsZero() {
			conditions = condition.AppendSQLAndClause(conditions,
				"scorecard_time >= ? AND scorecard_time < ?",
				conversationStartTimeMin.Format(timestampFormat), conversationStartTimeMax.Format(timestampFormat),
			)
		}
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableScore, conditions); err != nil {
			return err
		}
	}

	return nil
}

// Delete the conversation data with cut-off time in Conversation Cluster.
func DeleteConversationsFromClickhouseWithCutOffTime(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	table string,
	customerID, profileID string,
	conversationIds []string,
	conversationStartTimeMin, conversationStartTimeMax *time.Time,
	cutOffTimeString string,
) error {
	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		c.GetLogger().Errorf(ctx, "cannot connect ClickHouse for customer %v profile %v: %v", customerID, profileID, err)
		return err
	}
	// Find the right shards to delete data from.
	// The conversation data are sharded by toUnixTimestamp(conversation_start_time).
	shards, err := FindShardsForConversations(
		ctx, conn, "conversation_start_time", ResourceViewTableConversation, conversationIds)
	if err != nil {
		c.GetLogger().Errorf(ctx, " %v: %v", conversationIds, err)
		return err
	}
	if len(shards) == 0 {
		// If no shards found, the deletion will be skipped.
		//
		// Note: this could happen very rarely (e.g. on Carmax on 03/09/2024), when the initial indexing didn't find the conversation
		// from read-only DB replica and indexing was actually skipped. In this case, we don't need to delete the existing
		// scorecard scores in clickhouse. We could directly go forward with insertion.
		c.GetLogger().Infof(ctx, "found no conversations to delete for customer %v and profile %v", customerID, profileID)
		return nil
	}

	// Delete conversation data from all tables in clickhouse.
	conditions := condition.New(baseConversationFilterQuery, conversationIds)
	if conversationStartTimeMin != nil && !conversationStartTimeMin.IsZero() && conversationStartTimeMax != nil && !conversationStartTimeMax.IsZero() {
		conditions = condition.AppendSQLAndClause(conditions,
			"conversation_start_time >= ? AND conversation_start_time < ?",
			conversationStartTimeMin.Format(timestampFormat), conversationStartTimeMax.Format(timestampFormat),
		)
	}
	if _, err := time.Parse(timestampFormat, cutOffTimeString); err == nil {
		conditions = condition.AppendSQLAndClause(conditions,
			"update_time < ?", cutOffTimeString,
		)
	}

	if table == string(ResourceTableMomentAnnotation) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableMomentAnnotation, conditions); err != nil {
			return err
		}
	}
	if table == string(ResourceTableActionAnnotation) {
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableActionAnnotation, conditions); err != nil {
			return err
		}
	}

	if table == string(ResourceTableScorecard) {
		shards, err := FindShardsForConversations(
			ctx, conn, "scorecard_time", ResourceViewTableScorecard, conversationIds)
		if err != nil {
			c.GetLogger().Errorf(ctx, " %v: %v", conversationIds, err)
			return err
		}
		if len(shards) == 0 {
			c.GetLogger().Infof(ctx, "found no conversations to delete for customer %v and profile %v", customerID, profileID)
			return nil
		}

		conditions := condition.New(baseConversationFilterQuery, conversationIds)
		if conversationStartTimeMin != nil && !conversationStartTimeMin.IsZero() && conversationStartTimeMax != nil && !conversationStartTimeMax.IsZero() {
			conditions = condition.AppendSQLAndClause(conditions,
				"scorecard_time >= ? AND scorecard_time < ?",
				conversationStartTimeMin.Format(timestampFormat), conversationStartTimeMax.Format(timestampFormat),
			)
		}
		if _, err := time.Parse(timestampFormat, cutOffTimeString); err == nil {
			conditions = condition.AppendSQLAndClause(conditions,
				"update_time < ?", cutOffTimeString,
			)
		}
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableScorecard, conditions); err != nil {
			return err
		}
	}
	if table == string(ResourceTableScore) {
		shards, err := FindShardsForConversations(
			ctx, conn, "scorecard_time", ResourceViewTableScore, conversationIds)
		if err != nil {
			c.GetLogger().Errorf(ctx, " %v: %v", conversationIds, err)
			return err
		}
		if len(shards) == 0 {
			c.GetLogger().Infof(ctx, "found no conversations to delete for customer %v and profile %v", customerID, profileID)
			return nil
		}

		conditions := condition.New(baseConversationFilterQuery, conversationIds)
		if conversationStartTimeMin != nil && !conversationStartTimeMin.IsZero() && conversationStartTimeMax != nil && !conversationStartTimeMax.IsZero() {
			conditions = condition.AppendSQLAndClause(conditions,
				"scorecard_time >= ? AND scorecard_time < ?",
				conversationStartTimeMin.Format(timestampFormat), conversationStartTimeMax.Format(timestampFormat),
			)
		}
		if _, err := time.Parse(timestampFormat, cutOffTimeString); err == nil {
			conditions = condition.AppendSQLAndClause(conditions,
				"update_time < ?", cutOffTimeString,
			)
		}
		if err := DeleteOnShardFromClickhouseWhere(ctx, c, shards, customerID, profileID, ResourceTableScore, conditions); err != nil {
			return err
		}
	}

	return nil
}

func clickhouseAppsDB(ctx context.Context, c *clickhouseclient.ClickHouseClient, useDatabaseWriteReplica bool) *gorm.DB {
	return registry.DBForRead(c.GetAppsDB().DB(ctx), useDatabaseWriteReplica)
}

func clickhouseConversationDao(ctx context.Context, c *clickhouseclient.ClickHouseClient, useDatabaseWriteReplica bool) *dao.ConversationDao {
	if useDatabaseWriteReplica {
		return c.GetConversationDaoProvider().GetWriteReplica(ctx)
	}
	return c.GetConversationDaoProvider().Get(ctx)
}

func clickhouseUserDao(ctx context.Context, c *clickhouseclient.ClickHouseClient) dao.UserDao {
	return c.GetUserDaoProvider().Get(ctx)
}

func readMessagesByConversation(ctx context.Context, c *clickhouseclient.ClickHouseClient, conversationName *conversationpb.ConversationName, useDatabaseWriteReplica bool) ([]*dbmodel.Messages, map[string]*dbmodel.Messages, error) {
	messages, err := clickhouseConversationDao(ctx, c, useDatabaseWriteReplica).ListConversationMessagesByResourceName(conversationName)
	if err != nil {
		return nil, nil, err
	}
	messagesByID := make(map[string]*dbmodel.Messages, len(messages))
	for _, m := range messages {
		messagesByID[m.ConversationMessageID.String] = m
	}
	return messages, messagesByID, nil
}

func readAgentUsers(ctx context.Context, c *clickhouseclient.ClickHouseClient, conversation *dbmodel.Chats, messagesByID map[string]*dbmodel.Messages) (map[string]*dao.User, error) {
	agentUsersByID := make(map[string]*dao.User)
	if conversation.AgentUserID.Valid {
		agentUsersByID[conversation.AgentUserID.String] = nil
	}
	for _, m := range messagesByID {
		if m.AgentUserID.Valid {
			agentUsersByID[m.AgentUserID.String] = nil
		}
	}
	userIDs := make([]string, 0, len(agentUsersByID))
	for id := range agentUsersByID {
		userIDs = append(userIDs, id)
	}
	users, err := clickhouseUserDao(ctx, c).BulkFindUsersByUserID(conversation.CustomerID.String, userIDs, 1000 /* batchSize */)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		agentUsersByID[user.UserID.String] = user
	}
	// Remove the nil pointers from map just in case we get partial results from BulkFindUsersByUserID.
	result := make(map[string]*dao.User)
	for k, v := range agentUsersByID {
		if v != nil {
			result[k] = v
		}
	}
	return result, nil
}

// TODO: this function should be moved out of clickhouse package
// It's better to be included in an interface like ConversationsDAO
func ReadMomentAnnotationsForConversation(
	db *gorm.DB,
	conversationName *conversationpb.ConversationName,
) (map[string]*dbmodel.MomentAnnotations, error) {
	var momentAnnotations []*dbmodel.MomentAnnotations
	result := db.Find(&momentAnnotations, &dbmodel.MomentAnnotations{
		CustomerID:     conversationName.CustomerID,
		ProfileID:      conversationName.ProfileID,
		ConversationID: conversationName.ConversationID,
	})
	if result.Error != nil {
		return nil, result.Error
	}

	momentAnnotationsByID := make(map[string]*dbmodel.MomentAnnotations, len(momentAnnotations))
	for _, momentAnnotation := range momentAnnotations {
		momentAnnotationsByID[momentAnnotation.MomentAnnotationID] = momentAnnotation
	}
	return momentAnnotationsByID, nil
}

func readTemplatesForConversation(
	db *gorm.DB,
	conversationName *conversationpb.ConversationName,
	momentAnnotationsByID map[string]*dbmodel.MomentAnnotations,
) (map[string]*dbmodel.MomentTemplates, error) {
	momentTemplateIDMap := make(map[string]struct{})
	for _, momentAnnotation := range momentAnnotationsByID {
		if momentAnnotation.MomentTemplateID.Valid {
			momentTemplateIDMap[momentAnnotation.MomentTemplateID.String] = struct{}{}
		}
	}

	var momentTemplates []*dbmodel.MomentTemplates
	momentTemplateIDs := make([]string, 0, len(momentTemplateIDMap))
	for k := range momentTemplateIDMap {
		momentTemplateIDs = append(momentTemplateIDs, k)
	}
	result := db.Find(
		&momentTemplates,
		"customer_id = ? AND profile_id = ? AND moment_template_id IN ?",
		conversationName.CustomerID, conversationName.ProfileID, momentTemplateIDs)
	if result.Error != nil {
		return nil, result.Error
	}
	momentTemplatesByID := make(map[string]*dbmodel.MomentTemplates, len(momentTemplates))
	for _, momentTemplate := range momentTemplates {
		momentTemplatesByID[momentTemplate.MomentTemplateID] = momentTemplate
	}
	return momentTemplatesByID, nil
}

func readActionAnnotations(
	ctx context.Context, c *clickhouseclient.ClickHouseClient,
	conversationName *conversationpb.ConversationName,
	useDatabaseWriteReplica bool,
) (map[string]*dbmodel.ActionAnnotations, error) {
	var actionAnnotations []*dbmodel.ActionAnnotations
	db := clickhouseAppsDB(ctx, c, useDatabaseWriteReplica)
	result := db.Find(&actionAnnotations, &dbmodel.ActionAnnotations{
		CustomerID:     conversationName.CustomerID,
		ProfileID:      conversationName.ProfileID,
		ConversationID: conversationName.ConversationID,
	})
	if result.Error != nil {
		return nil, result.Error
	}
	c.GetLogger().Debugf(ctx, "write conversation to ClickHouse: found %v action annotations for conversation %v", len(actionAnnotations), conversationName)

	actionAnnotationsByID := make(map[string]*dbmodel.ActionAnnotations, len(actionAnnotations))
	for _, actionAnnotation := range actionAnnotations {
		actionAnnotationsByID[actionAnnotation.ActionAnnotationID] = actionAnnotation
	}
	return actionAnnotationsByID, nil
}

func readConversationEvents(
	ctx context.Context, c *clickhouseclient.ClickHouseClient,
	conversationName *conversationpb.ConversationName,
	useDatabaseWriteReplica bool,
) (map[string]*dbmodel.ConversationEvents, error) {
	var conversationEvents []*dbmodel.ConversationEvents
	db := clickhouseAppsDB(ctx, c, useDatabaseWriteReplica)
	result := db.Find(&conversationEvents, &dbmodel.ConversationEvents{
		CustomerID:     conversationName.CustomerID,
		ProfileID:      conversationName.ProfileID,
		ConversationID: conversationName.ConversationID,
	})
	if result.Error != nil {
		return nil, result.Error
	}
	c.GetLogger().Debugf(ctx, "write conversation to ClickHouse: found %v conversation events for conversation %v", len(conversationEvents), conversationName)

	conversationEventsByID := make(map[string]*dbmodel.ConversationEvents, len(conversationEvents))
	for _, conversationEvent := range conversationEvents {
		conversationEventsByID[conversationEvent.EventID] = conversationEvent
	}
	return conversationEventsByID, nil
}

type workflowAutomationRunMetadata struct {
	definitionID      string
	definitionVersion int32
}

func workflowAutomationRunIDFromEvent(conversationEvent *dbmodel.ConversationEvents) string {
	switch dbeventpb.Event_EventType(conversationEvent.Type) {
	case dbeventpb.Event_WORKFLOW_AUTOMATION_TRIGGERED:
		return conversationEvent.Payload.Message.GetWorkflowAutomationTriggered().GetWorkflowAutomationRunId()
	case dbeventpb.Event_WORKFLOW_AUTOMATION_INPUT:
		return conversationEvent.Payload.Message.GetWorkflowAutomationInput().GetWorkflowAutomationRunId()
	case dbeventpb.Event_WORKFLOW_AUTOMATION_OUTPUT:
		return conversationEvent.Payload.Message.GetWorkflowAutomationOutput().GetWorkflowAutomationRunId()
	case dbeventpb.Event_WORKFLOW_AUTOMATION_RESULT:
		return conversationEvent.Payload.Message.GetWorkflowAutomationResult().GetWorkflowAutomationRunId()
	default:
		return ""
	}
}

func readWorkflowAutomationRunMetadata(
	db *gorm.DB,
	conversationName *conversationpb.ConversationName,
	conversationEventsByID map[string]*dbmodel.ConversationEvents,
	existingConversationEventIDs []string,
) (map[string]workflowAutomationRunMetadata, error) {
	existingConversationEventIDSet := set.New[string](existingConversationEventIDs...)
	runIDs := set.New[string]()
	for _, conversationEvent := range conversationEventsByID {
		if existingConversationEventIDSet.Has(conversationEvent.EventID) {
			continue
		}
		switch dbeventpb.Event_EventType(conversationEvent.Type) {
		case dbeventpb.Event_WORKFLOW_AUTOMATION_INPUT,
			dbeventpb.Event_WORKFLOW_AUTOMATION_OUTPUT,
			dbeventpb.Event_WORKFLOW_AUTOMATION_RESULT:
		default:
			continue
		}
		runID := workflowAutomationRunIDFromEvent(conversationEvent)
		if runID == "" {
			continue
		}
		runIDs.Add(runID)
	}
	if runIDs.Len() == 0 {
		return nil, nil
	}

	var runs []*dbmodel.WorkflowAutomationRuns
	if err := db.Select("workflow_automation_run_id", "workflow_automation_definition_id", "workflow_automation_definition_version").
		Where("customer_id = ? AND profile_id = ? AND conversation_id = ?", conversationName.CustomerID, conversationName.ProfileID, conversationName.ConversationID).
		Where("workflow_automation_run_id IN ?", runIDs.Slice()).
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("reading workflow automation runs for conversation %s: %w", conversationName, err)
	}
	metadataByRunID := make(map[string]workflowAutomationRunMetadata, len(runs))
	for _, run := range runs {
		metadataByRunID[run.WorkflowAutomationRunID] = workflowAutomationRunMetadata{
			definitionID:      run.WorkflowAutomationDefinitionID,
			definitionVersion: run.WorkflowAutomationDefinitionVersion,
		}
	}
	return metadataByRunID, nil
}

func readScorecards(
	ctx context.Context, c *clickhouseclient.ClickHouseClient,
	conversationName *conversationpb.ConversationName,
	useDatabaseWriteReplica bool,
) ([]*dbmodel.Scorecards, error) {
	var scorecards []*dbmodel.Scorecards
	dbProvider, err := c.GetAppsDB().GetProvider(conversationName.CustomerID, conversationName.ProfileID)
	if err != nil {
		return nil, err
	}
	db := registry.DBForRead(dbProvider.Get(ctx), useDatabaseWriteReplica)
	result := db.Where("(scorecard_type IS NULL OR scorecard_type = ?)", int32(coachingpb.ScorecardType_SCORECARD_TYPE_UNSPECIFIED)).
		Find(&scorecards, &dbmodel.Scorecards{
			Customer:       conversationName.CustomerID,
			Profile:        conversationName.ProfileID,
			ConversationID: conversationName.ConversationID,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	c.GetLogger().Debugf(ctx, "write conversation to ClickHouse: found %v original scorecards for conversation %v", len(scorecards), conversationName)
	return scorecards, nil
}

func readScoresForScorecards(
	ctx context.Context,
	c *clickhouseclient.ClickHouseClient,
	conversationName *conversationpb.ConversationName,
	scorecards []*dbmodel.Scorecards,
	useDatabaseWriteReplica bool,
) (map[string][]*dbmodel.Scores, error) {
	if len(scorecards) == 0 {
		return make(map[string][]*dbmodel.Scores), nil
	}

	dbProvider, err := c.GetAppsDB().GetProvider(conversationName.CustomerID, conversationName.ProfileID)
	if err != nil {
		return nil, err
	}
	db := registry.DBForRead(dbProvider.Get(ctx), useDatabaseWriteReplica)

	// Collect all scorecard IDs
	scorecardIDs := make([]string, 0, len(scorecards))
	for _, sc := range scorecards {
		scorecardIDs = append(scorecardIDs, sc.ResourceID)
	}

	// Read all scores for these scorecards
	var scores []*dbmodel.Scores
	result := db.Where("customer = ? AND profile = ? AND scorecard_id IN ?",
		conversationName.CustomerID, conversationName.ProfileID, scorecardIDs).
		Find(&scores)
	if result.Error != nil {
		return nil, result.Error
	}

	// Group scores by scorecard_id
	scoresByScorecard := make(map[string][]*dbmodel.Scores)
	for _, score := range scores {
		scoresByScorecard[score.ScorecardID] = append(scoresByScorecard[score.ScorecardID], score)
	}

	c.GetLogger().Debugf(ctx, "write conversation to ClickHouse: found %v scores for %v scorecards in conversation %v",
		len(scores), len(scorecards), conversationName)
	return scoresByScorecard, nil
}

func readTemplateStructuresForScorecards(
	db *gorm.DB,
	conversationName *conversationpb.ConversationName,
	scorecards []*dbmodel.Scorecards,
) (map[scorecardTemplateRevisionKey]scoring.ScorecardTemplateStructure, error) {
	if len(scorecards) == 0 {
		return map[scorecardTemplateRevisionKey]scoring.ScorecardTemplateStructure{}, nil
	}

	templateIDs := make(map[string]struct{}, len(scorecards))
	revisions := make(map[string]struct{}, len(scorecards))
	for _, scorecard := range scorecards {
		templateIDs[scorecard.TemplateID] = struct{}{}
		revisions[scorecard.TemplateRevision] = struct{}{}
	}

	templateIDList := make([]string, 0, len(templateIDs))
	for templateID := range templateIDs {
		templateIDList = append(templateIDList, templateID)
	}
	revisionList := make([]string, 0, len(revisions))
	for revision := range revisions {
		revisionList = append(revisionList, revision)
	}

	var templates []*dbmodel.ScorecardTemplates
	err := db.Where(
		"customer = ? AND profile = ? AND resource_id IN ? AND revision IN ?",
		conversationName.CustomerID, conversationName.ProfileID, templateIDList, revisionList,
	).Find(&templates).Error
	if err != nil {
		return nil, err
	}

	templateStructuresByRevision := make(map[scorecardTemplateRevisionKey]scoring.ScorecardTemplateStructure, len(templates))
	for _, template := range templates {
		templateStructure, err := scoring.ParseScorecardTemplateStructure(template.Template)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s revision %s: %w", template.ResourceID, template.Revision, err)
		}
		templateStructuresByRevision[scorecardTemplateRevisionKey{
			templateID: template.ResourceID,
			revision:   template.Revision,
		}] = templateStructure
	}

	return templateStructuresByRevision, nil
}

func writeConversation(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chConversationIDs []string,
	conversation *dbmodel.Chats,
	agentUsersByID map[string]*dao.User,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) error {
	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for conversation %v", err)
			}
		}
	}()

	conversationRow := buildConversationRow(chConversationIDs, conversation, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if conversationRow != nil {
		err = batch.AppendStruct(conversationRow)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func buildConversationRow(
	chConversationIDs []string,
	conversation *dbmodel.Chats,
	agentUsersByID map[string]*dao.User,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) *clickhouseshared.ConversationRow {
	if slices.Contains(chConversationIDs, conversation.ConversationID.String) {
		return nil
	}
	isDevUser := false
	if user, exists := agentUsersByID[conversation.AgentUserID.String]; exists {
		isDevUser = user.IsDevUser
	}
	return &clickhouseshared.ConversationRow{
		AgentUserID:              conversation.AgentUserID.String,
		Channel:                  int32(conversation.Channel.Number()),
		ConversationDurationSecs: conversationDurationSecs,
		ConversationEndTime:      conversationEndTime,
		ConversationID:           conversation.ConversationID.String,
		ConversationSource:       int32(conversation.Source.Number()),
		ConversationStartTime:    conversation.StartedAt,
		CreateTime:               conversation.CreatedAt,
		CustomerID:               conversation.CustomerID.String,
		IsDevUser:                isDevUser,
		IsVoiceMail:              isVoiceMail,
		LanguageCode:             conversation.Metadata.Message.GetEffectiveLanguage(),
		PlatformAccountID:        conversation.PlatformAccountID,
		PlatformChatID:           conversation.PlatformChatID.String,
		PlatformID:               conversation.PlatformID,
		ProfileID:                conversation.ProfileID.String,
		RaiseHandState:           conversation.RaiseHandState.Int32,
		UmbrellaEndCustomerID:    conversation.UmbrellaEndCustomerID.String,
		UpdateTime:               time.Now(),
		UsecaseID:                conversation.UsecaseID.String,
	}
}

func writeMessages(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chConversationMessageIDs []string,
	conversation *dbmodel.Chats,
	messages []*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) error {
	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO message_d (%s)", clickhouseshared.ExtractCHFields(MessageRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for message %v", err)
			}
		}
	}()

	for _, messageRow := range buildMessageRows(chConversationMessageIDs, conversation, messages, agentUsersByID, isVoiceMail, conversationDurationSecs, conversationEndTime) {
		err = batch.AppendStruct(messageRow)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func buildMessageRows(
	chConversationMessageIDs []string,
	conversation *dbmodel.Chats,
	messages []*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) []*MessageRow {
	messageRows := []*MessageRow{}
	handleTimeSecsByMessageID := conv.ParseHandleTimeSecsByAgent(conversation, messages)
	for _, message := range messages {
		// Skip the Clickhouse rows if existing.
		if slices.Contains(chConversationMessageIDs, fmt.Sprintf("%s_%s", message.ConversationID.String, message.ConversationMessageID.String)) {
			continue
		}
		handleTimeSecs := int32(0)
		if t, exists := handleTimeSecsByMessageID[message.ID]; exists {
			// Note: for some conversation, we might have message inserted later than conversation closed.
			// In this case, we only allow the handle time to be at most conversation duration.
			if t > conversationDurationSecs {
				handleTimeSecs = conversationDurationSecs
			} else {
				handleTimeSecs = t
			}
		}
		agentUserID := conversation.AgentUserID.String
		if message.AgentUserID.Valid {
			agentUserID = message.AgentUserID.String
		}
		isDevUser := false
		if user, exists := agentUsersByID[agentUserID]; exists {
			isDevUser = user.IsDevUser
		}
		languageCode := conv.ResolveMessageLanguageCode(message.Metadata.Message.GetLanguageCode(), conversation.Metadata.Message.GetEffectiveLanguage())
		messageRows = append(messageRows, &MessageRow{
			AgentUserID:              agentUserID,
			ConversationDurationSecs: conversationDurationSecs,
			ConversationEndTime:      conversationEndTime,
			ConversationID:           conversation.ConversationID.String,
			ConversationSource:       int32(conversation.Source.Number()),
			ConversationStartTime:    conversation.StartedAt,
			CreateTime:               message.CreatedAt,
			CustomerID:               conversation.CustomerID.String,
			EndTimeMs:                message.EndTimeMs.Int32,
			HandleTimeSecs:           handleTimeSecs,
			IsDevUser:                isDevUser,
			IsVoiceMail:              isVoiceMail,
			LanguageCode:             languageCode,
			MessageID:                message.ConversationMessageID.String,
			// WARNING WARNING WARNING: Do not populate the message text here before getting
			// a review from the platform + security teams. Before we can write the message text
			// to ClickHouse, we need to consider the implications. For example, as of
			// 2024/08/17 manual redaction will not trigger reindexing to ClickHouse:
			// https://github.com/cresta/go-servers/blob/c6297c98e20a3d006141d63e92e8d839e257e512/apiserver/internal/internalconversation/action_cleanup_conversation_pii_redactions.go#L80-L80
			MessageText:        "",
			PlatformSequenceID: message.PlatformSequenceID,
			PlatformTime:       message.PlatformTimestamp,
			ProfileID:          conversation.ProfileID.String,
			RaiseHandState:     conversation.RaiseHandState.Int32,
			SpeakerRole:        message.SpeakerRole,
			StartTimeMs:        message.StartTimeMs.Int32,
			UpdateTime:         time.Now(),
			UsecaseID:          conversation.UsecaseID.String,
			MessageTextLength:  int32(len([]rune(message.Text))),
			// This might not be accurate. It won't work for languages that
			// don't separate words with spaces.
			MessageWordCount: int32(len(strings.Fields(message.Text))),
		})
	}
	return messageRows
}

func writeMomentAnnotations(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chMomentAnnotationIDs []string,
	conversation *dbmodel.Chats,
	messagesByID map[string]*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	momentAnnotationsByID map[string]*dbmodel.MomentAnnotations,
	actionAnnotationsByID map[string]*dbmodel.ActionAnnotations,
	momentTemplatesByID map[string]*dbmodel.MomentTemplates,
	policiesByID map[string]*dbmodel.Policies,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) error {
	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO moment_annotation_d (%s)", clickhouseshared.ExtractCHFields(MomentAnnotationRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for moment annotation %v", err)
			}
		}
	}()

	momentAnnotationRows, err := buildMomentAnnotationRows(ctx, logger, chMomentAnnotationIDs, conversation, messagesByID, agentUsersByID, momentAnnotationsByID, actionAnnotationsByID, momentTemplatesByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if err != nil {
		return err
	}
	for _, momentAnnotationRow := range momentAnnotationRows {
		err = batch.AppendStruct(momentAnnotationRow)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func buildMomentAnnotationRows(
	ctx context.Context,
	logger log.Logger,
	chMomentAnnotationIDs []string,
	conversation *dbmodel.Chats,
	messagesByID map[string]*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	momentAnnotationsByID map[string]*dbmodel.MomentAnnotations,
	actionAnnotationsByID map[string]*dbmodel.ActionAnnotations,
	momentTemplatesByID map[string]*dbmodel.MomentTemplates,
	policiesByID map[string]*dbmodel.Policies,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) ([]*MomentAnnotationRow, error) {
	momentAnnotations := []*MomentAnnotationRow{}

	// For conversation-level metadata moment annotations, we only assume the latest value is valid.
	metadataMomentToLatestAnnotation := map[string]*dbmodel.MomentAnnotations{}
	for _, momentAnnotation := range momentAnnotationsByID {
		if momentAnnotation.Type != int32(dbmomentpb.Moment_MOMENT_TYPE_CONVERSATION_METADATA) {
			continue
		}
		if !momentAnnotation.MomentTemplateID.Valid {
			continue
		}
		momentTemplateID := momentAnnotation.MomentTemplateID.String
		latest, exists := metadataMomentToLatestAnnotation[momentTemplateID]
		if !exists || momentAnnotation.CreatedAt.After(latest.CreatedAt) {
			metadataMomentToLatestAnnotation[momentTemplateID] = momentAnnotation
		}
	}

	for _, momentAnnotation := range momentAnnotationsByID {
		// Skip the Clickhouse rows if existing.
		if slices.Contains(chMomentAnnotationIDs, momentAnnotation.MomentAnnotationID) {
			continue
		}

		// From adherence action annotation (if exists)
		adherenceActionAdherenceType := int32(0)
		adherenceActionDetailedType := int32(0)
		adherenceActionID := ""
		adherenceActionPolicyID := ""
		adherenceActionPolicySourceType := int32(0)
		adherenceActionType := int32(0)
		if momentAnnotation.AdherenceActionAnnotationID.Valid {
			if adherenceActionAnnotation, exists := actionAnnotationsByID[momentAnnotation.AdherenceActionAnnotationID.String]; exists {
				adherenceActionAdherenceType = adherenceActionAnnotation.AdherenceType.Int32
				adherenceActionDetailedType = adherenceActionAnnotation.DetailedType.Int32
				adherenceActionID = adherenceActionAnnotation.ActionID
				adherenceActionType = adherenceActionAnnotation.Type
				if adherenceActionAnnotation.PolicyID.Valid {
					adherenceActionPolicyID = adherenceActionAnnotation.PolicyID.String
					if adherencePolicy, exists := policiesByID[adherenceActionAnnotation.PolicyID.String]; exists {
						adherenceActionPolicySourceType = adherencePolicy.SourceType.Int32
					}
				}
			}
		}

		// From adherence moment annotation (if exists)
		adherenceMomentAdherenceType := int32(0)
		adherenceMomentDetailedType := int32(0)
		adherenceMomentPolicyID := ""
		adherenceMomentPolicySourceType := int32(0)
		adherenceMomentType := int32(0)
		if momentAnnotation.AdherenceMomentAnnotationID.Valid {
			if adherenceMomentAnnotation, exists := momentAnnotationsByID[momentAnnotation.AdherenceMomentAnnotationID.String]; exists {
				adherenceMomentAdherenceType = adherenceMomentAnnotation.AdherenceType.Int32
				adherenceMomentDetailedType = adherenceMomentAnnotation.DetailedType.Int32
				adherenceMomentType = adherenceMomentAnnotation.Type
				if adherenceMomentAnnotation.PolicyID.Valid {
					adherenceMomentPolicyID = adherenceMomentAnnotation.PolicyID.String
					if adherencePolicy, exists := policiesByID[adherenceMomentAnnotation.PolicyID.String]; exists {
						adherenceMomentPolicySourceType = adherencePolicy.SourceType.Int32
					}
				}
			}
		}

		// From moment template (if exists)
		taxonomyState := int32(0)
		hidingInPerformanceInsights := false
		if momentTemplate, exists := momentTemplatesByID[momentAnnotation.MomentAnnotationID]; exists {
			if momentTemplate.TaxonomyState.Valid {
				taxonomyState = momentTemplate.TaxonomyState.Int32
			}
			if momentTemplate.Labels.Message.GetLabels() != nil {
				momentTemplateLabels := momentTemplate.Labels.Message.GetLabels()
				if momentTemplateLabels["hiding_in_performance_insights"] != "" {
					hidingInPerformanceInsights = strings.ToLower(momentTemplateLabels["hiding_in_performance_insights"]) == "true"
				}
			}
		}

		// From policies (if exists)
		policySourceType := int32(0)
		if momentAnnotation.PolicyID.Valid {
			if policy, exists := policiesByID[momentAnnotation.PolicyID.String]; exists {
				policySourceType = policy.SourceType.Int32
			}
		}

		// From Knowledge or KAGenAIReplyUsed moment (if exists)
		knowledgePayloadKnowledgeSessionID := ""
		if *clickHouseEnableKnowledgeSessionID {
			dbMomentAnnotationPayload := &dbmomentpb.MomentAnnotationPayload{}
			if err := converters.JSONToProtoObject(momentAnnotation.Payload, dbMomentAnnotationPayload); err != nil {
				return nil, err
			}
			switch momentAnnotation.Type {
			case int32(dbmomentpb.Moment_MOMENT_TYPE_KNOWLEDGE):
				knowledgePayloadKnowledgeSessionID = dbMomentAnnotationPayload.GetKnowledgePayload().GetKnowledgeSessionId()
			case int32(dbmomentpb.Moment_MOMENT_TYPE_KA_GEN_AI_REPLY_USED):
				knowledgePayloadKnowledgeSessionID = dbMomentAnnotationPayload.GetKaGenAiReplyUsedPayload().GetKnowledgeSessionId()
			default:
				knowledgePayloadKnowledgeSessionID = ""
			}
		}

		// From messages and users for agent messages.
		isDevUser := false
		agentUserID := conversation.AgentUserID.String
		if message, exists := messagesByID[momentAnnotation.MessageID.String]; exists {
			if message.AgentUserID.Valid {
				agentUserID = message.AgentUserID.String
			}
		}
		if user, exists := agentUsersByID[agentUserID]; exists {
			isDevUser = user.IsDevUser
		}

		// For creator user ID in general payload.
		creatorUserID := ""
		if len(momentAnnotation.Payload) > 0 {
			momentAnnotationPayload := &MomentAnnotationPayloadProto{}
			err := json.Unmarshal([]byte(momentAnnotation.Payload), momentAnnotationPayload)
			if err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "cannot unmarshal payload for moment annotation %s", momentAnnotation.MomentAnnotationID)
			}
			if momentAnnotationPayload.Metadata != nil && len(momentAnnotationPayload.Metadata.GetCreator()) > 0 {
				creatorUserResourceName := momentAnnotationPayload.Metadata.GetCreator()
				creatorUserName, err := userpb.ParseUserName(creatorUserResourceName)
				if err != nil {
					return nil, err
				}
				creatorUserID = creatorUserName.UserID
			}
		}

		// For conversation metadata annotations only.
		metadataType := int32(0)
		metadataBoolValue := false
		metadataNumberValue := float64(0.0)
		metadataStringValue := ""
		metadataSource := int32(0)
		if momentAnnotation.Type == int32(dbmomentpb.Moment_MOMENT_TYPE_CONVERSATION_METADATA) {
			// For CONVERSATION_METADATA moment annotations, we only index the latest value.
			latest, exists := metadataMomentToLatestAnnotation[momentAnnotation.MomentTemplateID.String]
			if !exists || momentAnnotation.MomentAnnotationID != latest.MomentAnnotationID {
				continue
			}
			momentAnnotationPayload := &MomentAnnotationPayloadProto{}
			err := json.Unmarshal([]byte(momentAnnotation.Payload), momentAnnotationPayload)
			if err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "cannot unmarshal payload for moment annotation %s", momentAnnotation.MomentAnnotationID)
			}
			logger.Debugf(ctx, "after Unmarshal: got moment annotation payload: %v", momentAnnotationPayload)
			if v, ok := momentAnnotationPayload.Annotation.(*dbmomentpb.MomentAnnotationPayload_ConversationMetadataPayload); ok {
				metadataPayload := v.ConversationMetadataPayload
				metadataType = int32(metadataPayload.MetadataType)
				metadataSource = int32(metadataPayload.Source)
				if len(metadataPayload.Values) > 0 {
					// We only support one single metadata value per annotation.
					metadataValue := metadataPayload.Values[0]
					if metadataValue == nil {
						return nil, status.Errorf(codes.FailedPrecondition, "conversation metadata value must have one of bool_value, number_value, and string_value populated")
					}
					switch v := metadataValue.Value.(type) {
					case *dbcommonpb.ConversationMetadataValue_NumberValue:
						metadataNumberValue = float64(v.NumberValue)
					case *dbcommonpb.ConversationMetadataValue_StringValue:
						metadataStringValue = v.StringValue
					case *dbcommonpb.ConversationMetadataValue_BooleanValue:
						metadataBoolValue = v.BooleanValue
					default:
						return nil, status.Errorf(codes.FailedPrecondition, "unknown metadata value %v", v)
					}
				}
			}
		}

		// For generic moment annotation payload
		var momentAnnotationPayload string
		if *clickHouseEnableMomentAnnotationPayload || momentAnnotation.Type == int32(dbmomentpb.Moment_MOMENT_TYPE_CONVERSATION_OUTCOME) {
			var err error
			momentAnnotationPayload, err = extractMomentAnnotationPayloadAsString(momentAnnotation.Payload)
			if err != nil {
				return nil, err
			}
		}

		momentAnnotations = append(momentAnnotations, &MomentAnnotationRow{
			AdherenceActionAdherenceType:      adherenceActionAdherenceType,
			AdherenceActionAnnotationID:       momentAnnotation.AdherenceActionAnnotationID.String,
			AdherenceActionDetailedType:       adherenceActionDetailedType,
			AdherenceActionID:                 adherenceActionID,
			AdherenceActionPolicyID:           adherenceActionPolicyID,
			AdherenceActionPolicySourceType:   adherenceActionPolicySourceType,
			AdherenceActionType:               adherenceActionType,
			AdherenceMomentAdherenceType:      adherenceMomentAdherenceType,
			AdherenceMomentAnnotationID:       momentAnnotation.AdherenceMomentAnnotationID.String,
			AdherenceMomentDetailedType:       adherenceMomentDetailedType,
			AdherenceMomentPolicyID:           adherenceMomentPolicyID,
			AdherenceMomentPolicySourceType:   adherenceMomentPolicySourceType,
			AdherenceMomentType:               adherenceMomentType,
			AdherenceType:                     momentAnnotation.AdherenceType.Int32,
			AgentUserID:                       agentUserID,
			BehaviorID:                        momentAnnotation.BehaviorID.String,
			ConversationDurationSecs:          conversationDurationSecs,
			ConversationEndTime:               conversationEndTime,
			ConversationID:                    conversation.ConversationID.String,
			ConversationSource:                int32(conversation.Source.Number()),
			ConversationStartTime:             conversation.StartedAt,
			CreateTime:                        momentAnnotation.CreatedAt,
			CustomerID:                        conversation.CustomerID.String,
			IsDevUser:                         isDevUser,
			IsVoiceMail:                       isVoiceMail,
			LabelsHidingInPerformanceInsights: hidingInPerformanceInsights,
			LanguageCode:                      conversation.Metadata.Message.GetEffectiveLanguage(),
			MessageID:                         momentAnnotation.MessageID.String,
			MomentAnnotationID:                momentAnnotation.MomentAnnotationID,
			MomentDetailedType:                momentAnnotation.DetailedType.Int32,
			MomentTemplateID:                  momentAnnotation.MomentTemplateID.String,
			MomentType:                        momentAnnotation.Type,
			PolicyID:                          momentAnnotation.PolicyID.String,
			PolicySourceType:                  policySourceType,
			ProfileID:                         conversation.ProfileID.String,
			RaiseHandState:                    conversation.RaiseHandState.Int32,
			Taxonomy:                          momentAnnotation.Taxonomy.String,
			TaxonomyState:                     taxonomyState,
			UpdateTime:                        time.Now(),
			UsecaseID:                         conversation.UsecaseID.String,
			CreatorUserID:                     creatorUserID,
			// For knowledge moment annotations only.
			KnowledgePayloadKnowledgeSessionID: knowledgePayloadKnowledgeSessionID,
			// For conversation metadata annotations only.
			MetadataType:        metadataType,
			MetadataBoolValue:   metadataBoolValue,
			MetadataNumberValue: metadataNumberValue,
			MetadataStringValue: metadataStringValue,
			MetadataSource:      metadataSource,
			// For generic moment annotation payload
			MomentAnnotationPayload: momentAnnotationPayload,
		})
	}
	return momentAnnotations, nil
}

// extractMomentAnnotationPayloadAsString converts a moment annotation payload to a string representation
func extractMomentAnnotationPayloadAsString(payload string) (string, error) {
	if payload == "" {
		return "", nil
	}

	// First unmarshal to DB payload type
	dbPayload := &dbmomentpb.MomentAnnotationPayload{}
	unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOpts.Unmarshal([]byte(payload), dbPayload); err != nil {
		return "", status.Errorf(codes.FailedPrecondition, "cannot unmarshal payload: %v", err)
	}

	// Convert DB payload to ClickHouse payload
	chPayload, err := ConvertDBMomentAnnotationPayloadToClickHouse(dbPayload)
	if err != nil {
		return "", fmt.Errorf("cannot convert payload to clickhouse: %v", err)
	}

	// Marshal ClickHouse payload to string with specific formatting options
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseEnumNumbers:  false,
		UseProtoNames:   true,
	}

	payloadJSON, err := marshaler.Marshal(chPayload)
	if err != nil {
		// In case of error, just return an empty string
		// We don't want to fail the entire process for a payload serialization issue
		return "", fmt.Errorf("cannot marshal clickhouse payload: %v", err)
	}

	// Convert to a more compact representation by removing unnecessary whitespace
	compactJSON := &bytes.Buffer{}
	if err := json.Compact(compactJSON, payloadJSON); err != nil {
		return string(payloadJSON), nil
	}

	return compactJSON.String(), nil
}

func writeActionAnnotations(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chActionAnnotationIDs []string,
	conversation *dbmodel.Chats,
	messagesByID map[string]*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	actionAnnotationsByID map[string]*dbmodel.ActionAnnotations,
	policiesByID map[string]*dbmodel.Policies,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) error {
	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO action_annotation_d (%s)", clickhouseshared.ExtractCHFields(ActionAnnotationRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for action annotation %v", err)
			}
		}
	}()

	actionAnnotationRows, err := buildActionAnnotationRows(ctx, logger, chActionAnnotationIDs, conversation, messagesByID, agentUsersByID, actionAnnotationsByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if err != nil {
		return err
	}
	for _, actionAnnotationRow := range actionAnnotationRows {
		err = batch.AppendStruct(actionAnnotationRow)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func buildActionAnnotationRows(
	ctx context.Context,
	logger log.Logger,
	chActionAnnotationIDs []string,
	conversation *dbmodel.Chats,
	messagesByID map[string]*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	actionAnnotationsByID map[string]*dbmodel.ActionAnnotations,
	policiesByID map[string]*dbmodel.Policies,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) ([]*ActionAnnotationRow, error) {
	actionAnnotations := []*ActionAnnotationRow{}
	for _, actionAnnotation := range actionAnnotationsByID {
		// Skip the Clickhouse rows if existing.
		if slices.Contains(chActionAnnotationIDs, actionAnnotation.ActionAnnotationID) {
			continue
		}

		policySourceType := int32(0)
		// From policies (if exists)
		if actionAnnotation.PolicyID.Valid {
			policyID := actionAnnotation.PolicyID.String
			if policy, exists := policiesByID[policyID]; exists {
				policySourceType = policy.SourceType.Int32
			}
		}

		whisperManagerUserID := ""
		dbActionAnnotationPayload := &dbactionpb.ActionAnnotationPayload{}
		if err := converters.JSONToProtoObject(actionAnnotation.Payload, dbActionAnnotationPayload); err != nil {
			return nil, err
		}
		if dbActionAnnotationPayload.GetManagerWhisper() != nil {
			whisperManagerUserID = dbActionAnnotationPayload.GetManagerWhisper().GetManagerUserId()
		}

		// From knowledge session ID (if exists)
		var knowledgePayloadKnowledgeSessionID string
		if *clickHouseEnableKnowledgeSessionID {
			switch actionAnnotation.Type {
			case int32(dbactionpb.Action_RETRIEVAL_QUESTION):
				knowledgePayloadKnowledgeSessionID = dbActionAnnotationPayload.GetRetrievalQuestion().GetKnowledgeSessionId()
			case int32(dbactionpb.Action_RETRIEVAL_ARTICLE):
				knowledgePayloadKnowledgeSessionID = dbActionAnnotationPayload.GetRetrievalArticleActionPayload().GetKnowledgeSessionId()
			case int32(dbactionpb.Action_RETRIEVAL_ANSWER):
				knowledgePayloadKnowledgeSessionID = dbActionAnnotationPayload.GetRetrievalAnswerActionPayload().GetKnowledgeSessionId()
			case int32(dbactionpb.Action_RETRIEVAL_REPLY):
				knowledgePayloadKnowledgeSessionID = dbActionAnnotationPayload.GetRetrievalReplyActionPayload().GetKnowledgeSessionId()
			default:
				knowledgePayloadKnowledgeSessionID = ""
			}
		}

		// From messages and users for agent messages.
		isDevUser := false
		agentUserID := conversation.AgentUserID.String
		if message, exists := messagesByID[actionAnnotation.MessageID.String]; exists {
			if message.AgentUserID.Valid {
				agentUserID = message.AgentUserID.String
			}
		}
		if user, exists := agentUsersByID[agentUserID]; exists {
			isDevUser = user.IsDevUser
		}

		// From summarization payload. Only summaries generated from real time conversations are valid.
		summarizationPayloadIsValidSummary := false
		if actionAnnotation.Type == int32(dbactionpb.Action_SUMMARY) &&
			dbActionAnnotationPayload.GetSummary() != nil &&
			len(dbActionAnnotationPayload.GetSummary().GetSummaryItems()) > 0 {
			summaryItem := dbActionAnnotationPayload.GetSummary().GetSummaryItems()[0]
			summarizationPayloadIsValidSummary = isValidSummaryItem(summaryItem, conversation.Source)
		}

		workflowAutomationDefinitionID, err := workflowAutomationHintDefinitionID(dbActionAnnotationPayload.GetWorkflowAutomationTriggerHint(), conversation.CustomerID.String, conversation.ProfileID.String)
		if err != nil {
			// fail-open: an invalid optional hint definition must not block conversation indexing.
			workflowAutomationDefinitionID = ""
			logger.WarnZap(ctx, "Ignoring invalid workflow automation hint definition",
				zap.String("action_annotation_id", actionAnnotation.ActionAnnotationID),
				zap.String("conversation_id", conversation.ConversationID.String))
		}

		actionAnnotations = append(actionAnnotations, &ActionAnnotationRow{
			ActionAnnotationID:                 actionAnnotation.ActionAnnotationID,
			ActionDetailedType:                 actionAnnotation.DetailedType.Int32,
			ActionID:                           actionAnnotation.ActionID,
			ActionType:                         actionAnnotation.Type,
			AdherenceType:                      actionAnnotation.AdherenceType.Int32,
			AgentUserID:                        agentUserID,
			ConversationDurationSecs:           conversationDurationSecs,
			ConversationEndTime:                conversationEndTime,
			ConversationID:                     conversation.ConversationID.String,
			ConversationSource:                 int32(conversation.Source.Number()),
			ConversationStartTime:              conversation.StartedAt,
			CreateTime:                         actionAnnotation.CreatedAt,
			CustomerID:                         conversation.CustomerID.String,
			IsDevUser:                          isDevUser,
			IsVoiceMail:                        isVoiceMail,
			LanguageCode:                       conversation.Metadata.Message.GetEffectiveLanguage(),
			MessageID:                          actionAnnotation.MessageID.String,
			PayloadManagerWhisperManagerUserID: whisperManagerUserID,
			PolicyID:                           actionAnnotation.PolicyID.String,
			PolicySourceType:                   policySourceType,
			ProfileID:                          conversation.ProfileID.String,
			RaiseHandState:                     conversation.RaiseHandState.Int32,
			Taxonomy:                           actionAnnotation.Taxonomy.String,
			TaxonomyState:                      int32(0),
			UpdateTime:                         time.Now(),
			UsecaseID:                          conversation.UsecaseID.String,
			KnowledgePayloadKnowledgeSessionID: knowledgePayloadKnowledgeSessionID,
			SummarizationPayloadIsValidSummary: summarizationPayloadIsValidSummary,
			PayloadWorkflowAutomationHintDefinitionID: workflowAutomationDefinitionID,
		})
	}
	return actionAnnotations, nil
}

// Hints recorded without a definition name retain an empty definition ID in ClickHouse.
func workflowAutomationHintDefinitionID(hint *dbactionpb.WorkflowAutomationTriggerHintPayload, customerID, profileID string) (string, error) {
	if hint.GetWorkflowAutomationDefinition() == "" {
		return "", nil
	}
	definitionName, err := workflowpb.ParseWorkflowAutomationDefinitionName(hint.GetWorkflowAutomationDefinition())
	if err != nil {
		return "", fmt.Errorf("parsing hint definition: %w", err)
	}
	if definitionName.CustomerID != customerID || definitionName.ProfileID != profileID {
		return "", fmt.Errorf("hint definition does not belong to conversation profile")
	}
	return definitionName.WorkflowAutomationDefinitionID, nil
}

// isValidSummaryItem reports whether a SUMMARY action annotation is a real, model-generated summary
// from a real-time conversation. It excludes summaries where model inference was skipped and the
// "conversation too short" fallbacks emitted by the legacy and V2/OOB summarization models.
func isValidSummaryItem(summaryItem *dbactionpb.SummaryItem, conversationSource conversationpb.Conversation_Source) bool {
	return !summaryItem.SkippedModelInference &&
		!sharedsummarization.IsPlaceholderSummaryText(summaryItem.GetSummaryText()) &&
		// Conversation_SOURCE_UNSPECIFIED means the conversation is real time.
		summaryItem.Metadata != nil && conversationSource == conversationpb.Conversation_SOURCE_UNSPECIFIED
}

func writeConversationEvents(
	ctx context.Context,
	logger log.Logger,
	conn driver.Conn,
	chConversationEventIDs []string,
	conversation *dbmodel.Chats,
	messagesByID map[string]*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	conversationEventsByID map[string]*dbmodel.ConversationEvents,
	workflowAutomationRunMetadataByID map[string]workflowAutomationRunMetadata,
	conversationEventsFromClickhouseEvents []*sharedclickhouseevents.ConversationEventRow,
	actionAnnotationsByID map[string]*dbmodel.ActionAnnotations,
	policiesByID map[string]*dbmodel.Policies,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) error {
	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/conversation/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO conversation_event_d (%s)", clickhouseshared.ExtractCHFields(ConversationEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				logger.Errorf(ctx, "Write conversation to ClickHouse: failed to abort batch for conversation event %v", err)
			}
		}
	}()
	conversationEventRows, err := buildConversationEventRows(chConversationEventIDs, conversation, messagesByID, agentUsersByID, conversationEventsByID, workflowAutomationRunMetadataByID, conversationEventsFromClickhouseEvents, actionAnnotationsByID, policiesByID, isVoiceMail, conversationDurationSecs, conversationEndTime)
	if err != nil {
		return err
	}
	for _, conversationEventRow := range conversationEventRows {
		err = batch.AppendStruct(conversationEventRow)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func buildConversationEventRows(
	chConversationEventIDs []string,
	conversation *dbmodel.Chats,
	messagesByID map[string]*dbmodel.Messages,
	agentUsersByID map[string]*dao.User,
	conversationEventsByID map[string]*dbmodel.ConversationEvents,
	workflowAutomationRunMetadataByID map[string]workflowAutomationRunMetadata,
	conversationEventsFromClickhouseEvents []*sharedclickhouseevents.ConversationEventRow,
	actionAnnotationsByID map[string]*dbmodel.ActionAnnotations,
	policiesByID map[string]*dbmodel.Policies,
	isVoiceMail bool,
	conversationDurationSecs int32,
	conversationEndTime time.Time,
) ([]*ConversationEventRow, error) {
	conversationEvents := []*ConversationEventRow{}
	// The conversation events from psql DB.
	for _, conversationEvent := range conversationEventsByID {
		// Skip the Clickhouse rows if existing.
		if slices.Contains(chConversationEventIDs, conversationEvent.EventID) {
			continue
		}

		actionAdherenceType := int32(0)
		actionDetailedType := int32(0)
		actionID := ""
		actionPolicyID := ""
		actionPolicySourceType := int32(0)
		actionType := int32(0)
		if conversationEvent.ActionAnnotationID.Valid {
			if actionAnnotation, exists := actionAnnotationsByID[conversationEvent.ActionAnnotationID.String]; exists {
				actionAdherenceType = actionAnnotation.AdherenceType.Int32
				actionDetailedType = actionAnnotation.DetailedType.Int32
				actionID = actionAnnotation.ActionID
				actionType = actionAnnotation.Type
				if actionAnnotation.PolicyID.Valid {
					actionPolicyID = actionAnnotation.PolicyID.String
					if policy, exists := policiesByID[actionAnnotation.PolicyID.String]; exists {
						actionPolicySourceType = policy.SourceType.Int32
					}
				}
			}
		}

		// From action annotations
		var knowledgePayloadKnowledgeSessionID string
		if *clickHouseEnableKnowledgeSessionID {
			switch conversationEvent.Type {
			case int32(dbeventpb.Event_USED_KB_DOCUMENT):
				knowledgePayloadKnowledgeSessionID = conversationEvent.Payload.Message.GetUsedKbDocument().GetKnowledgeSessionId()
			case int32(dbeventpb.Event_KA_GEN_AI_REPLY_FEEDBACK):
				knowledgePayloadKnowledgeSessionID = conversationEvent.Payload.Message.GetKaGenAiReplyFeedback().GetKnowledgeSessionId()
			case int32(dbeventpb.Event_KA_GEN_AI_REPLY_COPIED):
				knowledgePayloadKnowledgeSessionID = conversationEvent.Payload.Message.GetKaGenAiReplyCopied().GetKnowledgeSessionId()
			case int32(dbeventpb.Event_KA_GEN_AI_REPLY_INSERTED):
				knowledgePayloadKnowledgeSessionID = conversationEvent.Payload.Message.GetKaGenAiReplyInserted().GetKnowledgeSessionId()
			case int32(dbeventpb.Event_KA_GEN_AI_RATIONALE_CLICKED):
				knowledgePayloadKnowledgeSessionID = conversationEvent.Payload.Message.GetKaGenAiRationaleClicked().GetKnowledgeSessionId()
			default:
				knowledgePayloadKnowledgeSessionID = ""
			}
		}

		// From messages and users for agent messages.
		isDevUser := false
		agentUserID := conversation.AgentUserID.String
		if message, exists := messagesByID[conversationEvent.MessageID.String]; exists {
			if message.AgentUserID.Valid {
				agentUserID = message.AgentUserID.String
			}
		}
		if user, exists := agentUsersByID[agentUserID]; exists {
			isDevUser = user.IsDevUser
		}

		// From used summarization event.
		// TODO(xiaoyi.ge): deprecate this.
		var summarizationDataType int32
		if conversationEvent.Type == int32(dbeventpb.Event_USED_SUMMARIZATION) &&
			conversationEvent.Payload.Message.GetUsedSummarization() != nil {
			switch conversationEvent.Payload.Message.GetUsedSummarization().GetData().(type) {
			case *dbeventpb.UsedSummarization_CopyAllNotes_:
				summarizationDataType = 1
			case *dbeventpb.UsedSummarization_CopySummarizationNote_:
				summarizationDataType = 2
			case *dbeventpb.UsedSummarization_EditSummarizationNote_:
				summarizationDataType = 3
			default:
				summarizationDataType = 0
			}
		}

		// From conversation event payloads.
		// For USED_KB_DOCUMENT conversation event payload.
		usedKbDocumentID := ""
		usedKbDocumentType := int32(0)
		usedKbDocumentSessionID := ""
		if conversationEvent.Type == int32(dbeventpb.Event_USED_KB_DOCUMENT) &&
			conversationEvent.Payload.Message.GetUsedKbDocument() != nil {
			usedKbDocument := conversationEvent.Payload.Message.GetUsedKbDocument()
			usedKbDocumentID = usedKbDocument.GetDocId()
			usedKbDocumentType = int32(usedKbDocument.GetDocType())
			if usedKbDocument.GetSessionMetadata() != nil {
				usedKbDocumentSessionID = usedKbDocument.GetSessionMetadata().GetSessionId()
			}
		}
		// For USED_KB_SEARCH conversation event payload.
		usedKbSearchSessionID := ""
		if conversationEvent.Type == int32(dbeventpb.Event_USED_KB_SEARCH) &&
			conversationEvent.Payload.Message.GetUsedKbSearch() != nil {
			usedKbSearch := conversationEvent.Payload.Message.GetUsedKbSearch()
			if usedKbSearch.GetSessionMetadata() != nil {
				usedKbSearchSessionID = usedKbSearch.GetSessionMetadata().GetSessionId()
			}
		}
		// For USED_NOTE_TAKING conversation event payload.
		usedNoteTakingDataType := int32(0)
		if conversationEvent.Type == int32(dbeventpb.Event_USED_NOTE_TAKING) &&
			conversationEvent.Payload.Message.GetUsedNoteTaking() != nil {
			usedNoteTaking := conversationEvent.Payload.Message.GetUsedNoteTaking()
			// Keep consistent with DB proto:
			// apiserver/sql-schema/protos/event/event.proto:415
			switch usedNoteTaking.GetData().(type) {
			case *dbeventpb.UsedNoteTaking_CopyEntity_:
				usedNoteTakingDataType = 1
			case *dbeventpb.UsedNoteTaking_UpdateEntity_:
				usedNoteTakingDataType = 2
			default:
				usedNoteTakingDataType = 0
			}
		}
		// For USED_SUMMARIZATION conversation event payload.
		usedSummarizationDataType := int32(0)
		if conversationEvent.Type == int32(dbeventpb.Event_USED_SUMMARIZATION) &&
			conversationEvent.Payload.Message.GetUsedSummarization() != nil {
			usedSummarization := conversationEvent.Payload.Message.GetUsedSummarization()
			// Keep consistent with DB proto:
			// apiserver/sql-schema/protos/event/event.proto:380
			switch usedSummarization.GetData().(type) {
			case *dbeventpb.UsedSummarization_CopyAllNotes_:
				usedSummarizationDataType = 1
			case *dbeventpb.UsedSummarization_EditSummarizationNote_:
				usedSummarizationDataType = 2
			case *dbeventpb.UsedSummarization_CopySummarizationNote_:
				usedSummarizationDataType = 3
			default:
				usedSummarizationDataType = 0
			}
		}

		workflowAutomationRunID := workflowAutomationRunIDFromEvent(conversationEvent)
		var workflowAutomationDefinitionID string
		var workflowAutomationDefinitionVersion, workflowAutomationTriggerSource, workflowAutomationResultStatus int32
		switch dbeventpb.Event_EventType(conversationEvent.Type) {
		case dbeventpb.Event_WORKFLOW_AUTOMATION_TRIGGERED:
			trigger := conversationEvent.Payload.Message.GetWorkflowAutomationTriggered()
			workflowAutomationDefinitionID = trigger.GetWorkflowAutomationDefinitionId()
			workflowAutomationDefinitionVersion = trigger.GetWorkflowAutomationDefinitionVersion()
			workflowAutomationTriggerSource = int32(trigger.GetTriggerSource())
		case dbeventpb.Event_WORKFLOW_AUTOMATION_RESULT:
			result := conversationEvent.Payload.Message.GetWorkflowAutomationResult()
			workflowAutomationResultStatus = int32(result.GetStatus())
		}
		if runMetadata, exists := workflowAutomationRunMetadataByID[workflowAutomationRunID]; exists &&
			conversationEvent.Type != int32(dbeventpb.Event_WORKFLOW_AUTOMATION_TRIGGERED) {
			workflowAutomationDefinitionID = runMetadata.definitionID
			workflowAutomationDefinitionVersion = runMetadata.definitionVersion
		}

		conversationEvents = append(conversationEvents, &ConversationEventRow{
			ActionAdherenceType:                   actionAdherenceType,
			ActionAnnotationID:                    conversationEvent.ActionAnnotationID.String,
			ActionDetailedType:                    actionDetailedType,
			ActionID:                              actionID,
			ActionPolicyID:                        actionPolicyID,
			ActionPolicySourceType:                actionPolicySourceType,
			ActionType:                            actionType,
			AgentUserID:                           agentUserID,
			ConversationDurationSecs:              conversationDurationSecs,
			ConversationEndTime:                   conversationEndTime,
			ConversationID:                        conversation.ConversationID.String,
			ConversationSource:                    int32(conversation.Source.Number()),
			ConversationStartTime:                 conversation.StartedAt,
			CreateTime:                            conversationEvent.CreatedAt,
			CustomerID:                            conversation.CustomerID.String,
			EventID:                               conversationEvent.EventID,
			EventType:                             conversationEvent.Type,
			IsDevUser:                             isDevUser,
			IsVoiceMail:                           isVoiceMail,
			LanguageCode:                          conversation.Metadata.Message.GetEffectiveLanguage(),
			MessageID:                             conversationEvent.MessageID.String,
			ProfileID:                             conversation.ProfileID.String,
			RaiseHandState:                        conversation.RaiseHandState.Int32,
			UpdateTime:                            time.Now(),
			UsecaseID:                             conversation.UsecaseID.String,
			KnowledgePayloadKnowledgeSessionID:    knowledgePayloadKnowledgeSessionID,
			SummarizationPayloadDataType:          summarizationDataType,
			PayloadUsedKbDocumentDocID:            usedKbDocumentID,
			PayloadUsedKbDocumentDocType:          usedKbDocumentType,
			PayloadUsedKbDocumentSessionID:        usedKbDocumentSessionID,
			PayloadUsedKbSearchSessionID:          usedKbSearchSessionID,
			PayloadUsedNoteTakingDataType:         usedNoteTakingDataType,
			PayloadUsedSummarizationDataType:      usedSummarizationDataType,
			PayloadWorkflowAutomationRunID:        workflowAutomationRunID,
			PayloadWorkflowAutomationDefinitionID: workflowAutomationDefinitionID,
			PayloadWorkflowAutomationDefinitionVersion: workflowAutomationDefinitionVersion,
			PayloadWorkflowAutomationTriggerSource:     workflowAutomationTriggerSource,
			PayloadWorkflowAutomationResultStatus:      workflowAutomationResultStatus,
		})
	}
	// The conversation events from ClickHouse events cluster.
	for _, conversationEvent := range conversationEventsFromClickhouseEvents {
		// From messages and users for agent messages.
		isDevUser := false
		agentUserID := conversation.AgentUserID.String
		if message, exists := messagesByID[conversationEvent.MessageID]; exists {
			if message.AgentUserID.Valid {
				agentUserID = message.AgentUserID.String
			}
		}
		if user, exists := agentUsersByID[agentUserID]; exists {
			isDevUser = user.IsDevUser
		}

		// Convert event type from clickhouse events cluster to conversations cluster (DB proto).
		var eventType int32
		switch conversationEvent.EventType {
		case int32(clickhouseeventspb.ConversationEventType_USED_SUMMARIZATION):
			eventType = int32(dbeventpb.Event_USED_SUMMARIZATION)
		case int32(clickhouseeventspb.ConversationEventType_USED_SUGGESTION):
			eventType = int32(dbeventpb.Event_USED_SUGGESTION)
		case int32(clickhouseeventspb.ConversationEventType_USED_KB_SEARCH):
			eventType = int32(dbeventpb.Event_USED_KB_SEARCH)
		case int32(clickhouseeventspb.ConversationEventType_USED_KB_DOCUMENT):
			eventType = int32(dbeventpb.Event_USED_KB_DOCUMENT)
		case int32(clickhouseeventspb.ConversationEventType_USED_NOTE_TAKING):
			eventType = int32(dbeventpb.Event_USED_NOTE_TAKING)
		case int32(clickhouseeventspb.ConversationEventType_USED_SMART_COMPOSE):
			eventType = int32(dbeventpb.Event_USED_SMART_COMPOSE)
		case int32(clickhouseeventspb.ConversationEventType_CONVERSATION_EVENT_TYPE_UNSPECIFIED):
			eventType = int32(dbeventpb.Event_EVENT_TYPE_UNSPECIFIED)
		default:
			return nil, fmt.Errorf("The conversation event type %v not supported", conversationEvent.EventType)
		}

		conversationEvents = append(conversationEvents, &ConversationEventRow{
			ActionAdherenceType:                0,
			ActionAnnotationID:                 "",
			ActionDetailedType:                 0,
			ActionID:                           "",
			ActionPolicyID:                     "",
			ActionPolicySourceType:             0,
			ActionType:                         0,
			AgentUserID:                        agentUserID,
			ConversationDurationSecs:           conversationDurationSecs,
			ConversationEndTime:                conversationEndTime,
			ConversationID:                     conversation.ConversationID.String,
			ConversationSource:                 int32(conversation.Source.Number()),
			ConversationStartTime:              conversation.StartedAt,
			CreateTime:                         conversationEvent.CreateTime,
			CustomerID:                         conversation.CustomerID.String,
			EventID:                            conversationEvent.EventID,
			EventType:                          eventType,
			IsDevUser:                          isDevUser,
			IsVoiceMail:                        isVoiceMail,
			LanguageCode:                       conversation.Metadata.Message.GetEffectiveLanguage(),
			MessageID:                          conversationEvent.MessageID,
			ProfileID:                          conversation.ProfileID.String,
			RaiseHandState:                     conversation.RaiseHandState.Int32,
			UpdateTime:                         time.Now(),
			UsecaseID:                          conversation.UsecaseID.String,
			KnowledgePayloadKnowledgeSessionID: "",
			SummarizationPayloadDataType:       0,
			PayloadUsedKbDocumentDocID:         "",
			PayloadUsedKbDocumentDocType:       0,
			PayloadUsedKbDocumentSessionID:     "",
			PayloadUsedKbSearchSessionID:       "",
			PayloadUsedNoteTakingDataType:      0,
			PayloadUsedSummarizationDataType:   0,
		})
	}
	return conversationEvents, nil
}

// Note that the dbmodel.Chats only has partial information.
func BuildScorecardRows(
	chScorecardIDs []string,
	scorecards []*dbmodel.Scorecards,
	isProcessScorecard bool,
	scores []*ScoreRow,
) []*ScorecardRow {
	scorecardRows := []*ScorecardRow{}
	// For conversation scorecards, skip if no criteria scores.
	// For process scorecards, we still write scorecard metadata even without score rows.
	if len(scores) == 0 && !isProcessScorecard {
		return scorecardRows
	}

	for _, scorecard := range scorecards {
		// Skip the Clickhouse rows if existing.
		if slices.Contains(chScorecardIDs, scorecard.ResourceID) {
			continue
		}

		// For conversation scorecards, skip if no scores found for this scorecard.
		// For process scorecards, use default values if no scores.
		var scoreRow *ScoreRow
		for _, s := range scores {
			if s.ScorecardID == scorecard.ResourceID {
				scoreRow = s
				break
			}
		}
		if scoreRow == nil && !isProcessScorecard {
			continue
		}

		// Build scorecard extra from scoreRow if available, otherwise use defaults
		extra := &ScorecardExtra{}
		if scoreRow != nil {
			extra.ConversationDurationBinsID = scoreRow.ConversationDurationBinsID
			extra.IsDevUser = scoreRow.IsDevUser
			extra.IsVoiceMail = scoreRow.IsVoiceMail
			extra.ScorecardTime = scoreRow.ConversationStartTime
		}

		// Use processInteractionAt for process scorecards
		if isProcessScorecard {
			extra.ScorecardTime = scorecard.ProcessInteractionAt.Time
		}

		scorecardRows = append(scorecardRows, BuildScorecardRow(scorecard, extra))
	}
	return scorecardRows
}

func BuildScorecardRow(
	scorecard *dbmodel.Scorecards,
	scorecardExtra *ScorecardExtra,
) *ScorecardRow {
	score := -1.
	if scorecard.Score.Valid {
		score = scorecard.Score.Float64
	}

	aiScoreTime := defaultTime
	if scorecard.AiScoredAt.Valid {
		aiScoreTime = scorecard.AiScoredAt.Time
	}
	scorecardAcknowledgeTime := defaultTime
	if scorecard.AcknowledgedAt.Valid {
		scorecardAcknowledgeTime = scorecard.AcknowledgedAt.Time
	}
	scorecardSubmitTime := defaultTime
	if scorecard.SubmittedAt.Valid {
		scorecardSubmitTime = scorecard.SubmittedAt.Time
	}
	publishTime := defaultTime
	if scorecard.PublishedAt.Valid {
		publishTime = scorecard.PublishedAt.Time
	}

	return &ScorecardRow{
		AcknowledgeComment:         scorecard.AcknowledgeComment.String,
		AgentUserID:                scorecard.AgentUserID,
		AiScoreTime:                aiScoreTime,
		AutoFailed:                 scorecard.AutoFailed.Bool,
		CoachingPlanID:             scorecard.CoachingPlanID.String,
		ConversationDurationBinsID: scorecardExtra.ConversationDurationBinsID,
		ConversationID:             scorecard.ConversationID,
		CreatorUserID:              scorecard.CreatorUserID.String,
		CustomerID:                 scorecard.Customer,
		IsDevUser:                  scorecardExtra.IsDevUser,
		IsVoiceMail:                scorecardExtra.IsVoiceMail,
		LastUpdatorUserID:          scorecard.LastUpdaterUserID.String,
		ManuallyScored:             scorecard.ManuallyScored.Bool,
		ProfileID:                  scorecard.Profile,
		PublishTime:                publishTime,
		PublisherUserID:            scorecard.PublisherUserID.String,
		Score:                      score,
		ScorecardAcknowledgeTime:   scorecardAcknowledgeTime,
		ScorecardCreateTime:        scorecard.CreatedAt,
		ScorecardID:                scorecard.ResourceID,
		ScorecardLastUpdateTime:    scorecard.UpdatedAt,
		ScorecardSubmitTime:        scorecardSubmitTime,
		ScorecardTemplateID:        scorecard.TemplateID,
		ScorecardTemplateRevision:  scorecard.TemplateRevision,
		ScorecardTime:              scorecardExtra.ScorecardTime,
		SubmitterUserID:            scorecard.SubmitterUserID.String,
		UpdateTime:                 time.Now(),
		UsecaseID:                  scorecard.UsecaseID.String,
	}
}

func readPoliciesByIDFromActionAnnotations(ctx context.Context, c *clickhouseclient.ClickHouseClient, conversationName *conversationpb.ConversationName, actionAnnotationsByID map[string]*dbmodel.ActionAnnotations, useDatabaseWriteReplica bool) (map[string]*dbmodel.Policies, error) {
	policyIDMap := make(map[string]struct{})
	for _, actionAnnotation := range actionAnnotationsByID {
		if actionAnnotation.PolicyID.Valid {
			policyIDMap[actionAnnotation.PolicyID.String] = struct{}{}
		}
	}
	policyIDs := make([]string, 0, len(policyIDMap))
	for k := range policyIDMap {
		policyIDs = append(policyIDs, k)
	}

	var policies []*dbmodel.Policies
	db := clickhouseAppsDB(ctx, c, useDatabaseWriteReplica)
	result := db.Find(
		&policies,
		"customer_id = ? AND profile_id = ? AND policy_id IN ?",
		conversationName.CustomerID, conversationName.ProfileID, policyIDs)
	if result.Error != nil {
		return nil, result.Error
	}

	policiesByID := make(map[string]*dbmodel.Policies, len(policies))
	for _, policy := range policies {
		policiesByID[policy.PolicyID] = policy
	}
	return policiesByID, nil
}

// TODO: this function should be moved out of clickhouse
func HasVoiceMailMoments(momentAnnotationsByID map[string]*dbmodel.MomentAnnotations) bool {
	for _, moment := range momentAnnotationsByID {
		if moment.Type == int32(dbmomentpb.Moment_MOMENT_TYPE_VOICE_MAIL) {
			return true
		}
	}
	return false
}

// TODO: this function should be moved out of clickhouse
func IsVoiceMail(
	db *gorm.DB,
	conversationName *conversationpb.ConversationName,
) (bool, error) {
	momentAnnotationsByID, err := ReadMomentAnnotationsForConversation(db, conversationName)
	if err != nil {
		return false, err
	}
	return HasVoiceMailMoments(momentAnnotationsByID), nil
}
