package events

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	clickhouseeventspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/clickhouse/events"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	sessionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/session"
	"github.com/cresta/shared-go/util/envflag"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	redisconvpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/redis/conversation"
	"github.com/cresta/go-servers/shared/auth"
	eventsclient "github.com/cresta/go-servers/shared/clickhouse/events/client"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

const (
	smartComposeInserted           = "SMART_COMPOSE_INSERTED"
	smartComposeInsertedWordByWord = "SMART_COMPOSE_INSERTED_WORD_BY_WORD"
	crestaString                   = "cresta"

	// EvaluateAnalyticsChart cache events for Hex dashboards (CrestaTrino-queryable).
	EvaluateAnalyticsChartCacheHitEventName  = "EVALUATE_ANALYTICS_CHART_CACHE_HIT"
	EvaluateAnalyticsChartCacheMissEventName = "EVALUATE_ANALYTICS_CHART_CACHE_MISS"

	// OperaGPT usage tracking for Hex dashboards.
	OperaGptSessionStartedEventName = "OPERA_GPT_SESSION_STARTED"
)

var (
	// As of 05/23/2024, we only write the whitelisted conversation events into Clickhouse.
	// In future, we will consider writing more/all conversation events.
	whitelistConversationEventTypes = []conversationpb.ConversationEvent_EventType{
		conversationpb.ConversationEvent_USED_SUMMARIZATION,
		conversationpb.ConversationEvent_USED_SUGGESTION,
		conversationpb.ConversationEvent_USED_KB_SEARCH,
		conversationpb.ConversationEvent_USED_KB_DOCUMENT,
		conversationpb.ConversationEvent_USED_NOTE_TAKING,
		conversationpb.ConversationEvent_USED_SMART_COMPOSE,
		conversationpb.ConversationEvent_KA_GEN_AI_EXTRACTED_QUESTION_FEEDBACK,
		conversationpb.ConversationEvent_TOPIC_SUBMISSION,
		conversationpb.ConversationEvent_USED_EXTERNAL_URLS_IN_HINT,
		conversationpb.ConversationEvent_KA_GEN_AI_RATIONALE_CLICKED,
		conversationpb.ConversationEvent_EMAIL_USED_PIECEMEAL_SUGGESTION,
		conversationpb.ConversationEvent_EMAIL_VIEWED_PIECEMEAL_SUGGESTION,
		conversationpb.ConversationEvent_EMAIL_USED_FULL_REPLY_SUGGESTION,
		conversationpb.ConversationEvent_EMAIL_VIEWED_FULL_REPLY_SUGGESTION,
		conversationpb.ConversationEvent_EMAIL_INSERTED_SEARCH_RESULT,
		conversationpb.ConversationEvent_EMAIL_USED_SMART_COMPOSE,
		conversationpb.ConversationEvent_AGENT_FEEDBACK,
		conversationpb.ConversationEvent_EMAIL_CASE_WINDOW_OPENED,
		conversationpb.ConversationEvent_EMAIL_MESSAGE_EDITING_STARTED,
		conversationpb.ConversationEvent_SUMMARIZATION_UPSERTED,
		conversationpb.ConversationEvent_KA_CONTEXT_FIELD_STATE_UPDATED,
		conversationpb.ConversationEvent_CHAT_TRANSFER_INITIATED,
		conversationpb.ConversationEvent_CHAT_AGENT_ACCEPTED,
		conversationpb.ConversationEvent_WORKFLOW_AUTOMATION_TRIGGERED,
		conversationpb.ConversationEvent_WORKFLOW_AUTOMATION_INPUT,
		conversationpb.ConversationEvent_WORKFLOW_AUTOMATION_OUTPUT,
		conversationpb.ConversationEvent_WORKFLOW_AUTOMATION_RESULT,
		conversationpb.ConversationEvent_CUSTOMER_SNAPSHOT_PRESENTED,
		conversationpb.ConversationEvent_CUSTOMER_SNAPSHOT_CLOSED,
		conversationpb.ConversationEvent_CUSTOMER_SNAPSHOT_ITEM_CLICKED,
	}
	whitelistAnalyticsEventName = []string{
		// Used smart compose events.
		smartComposeInserted,
		smartComposeInsertedWordByWord,
		// EvaluateAnalyticsChart cache metrics for Hex dashboards.
		EvaluateAnalyticsChartCacheHitEventName,
		EvaluateAnalyticsChartCacheMissEventName,
		// OperaGPT usage tracking for Hex dashboards.
		OperaGptSessionStartedEventName,
	}
	whitelistSessionLogType = []sessionpb.SessionLog_Type{
		// The session log for director page visit.
		sessionpb.SessionLog_DIRECTOR_PAGE_VISIT,
		sessionpb.SessionLog_TYPE_DIRECTOR_BUTTON_CLICK,
		sessionpb.SessionLog_TYPE_LLM_ANALYST_USER_FEEDBACK,
		sessionpb.SessionLog_TYPE_ASR_FEEDBACK,
		sessionpb.SessionLog_TYPE_DIRECTOR_TRANSLATION_USER_FEEDBACK,
		sessionpb.SessionLog_TYPE_REALTIME_TRENDS_USER_FEEDBACK,
		sessionpb.SessionLog_TYPE_DIRECTOR_UI_SWITCH_FEEDBACK,
		sessionpb.SessionLog_TYPE_DIRECTOR_UI_FEEDBACK,
	}

	crestaEmailRegex = regexp.MustCompile(`.+@(cresta\.com|cresta\.ai|cresta\.test)$`)

	clickhouseEnableIsDevUser = envflag.Bool("CLICKHOUSE_ENABLE_IS_DEV_USER", false, "Whether to enable is_dev_user in clickhouse")
)

type ConversationEventRow struct {
	AgentUserID           string    `ch:"agent_user_id"`
	ConversationID        string    `ch:"conversation_id"`
	ConversationStartTime time.Time `ch:"conversation_start_time"`
	CreateTime            time.Time `ch:"create_time"`
	CustomerID            string    `ch:"customer_id"`
	EventID               string    `ch:"event_id"`
	EventName             string    `ch:"event_name"`
	EventPayload          string    `ch:"event_payload"`
	EventType             int32     `ch:"event_type"`
	MessageID             string    `ch:"message_id"`
	ProfileID             string    `ch:"profile_id"`
	UsecaseID             string    `ch:"usecase_id"`
}

type UserEventRow struct {
	AgentUserID  string         `ch:"agent_user_id"`
	CreateTime   time.Time      `ch:"create_time"`
	CustomerID   string         `ch:"customer_id"`
	EventID      string         `ch:"event_id"`
	EventName    string         `ch:"event_name"`
	EventPayload map[string]any `ch:"event_payload"`
	EventTime    time.Time      `ch:"event_time"`
	EventType    int32          `ch:"event_type"`
}

type AnalyticsEventRow struct {
	AgentUserID  string    `ch:"agent_user_id"`
	CreateTime   time.Time `ch:"create_time"`
	CustomerID   string    `ch:"customer_id"`
	EventID      string    `ch:"event_id"`
	EventName    string    `ch:"event_name"`
	EventPayload string    `ch:"event_payload"`
	EventTime    time.Time `ch:"event_time"`
	ProfileID    string    `ch:"profile_id"`
	IsDevUser    bool      `ch:"is_dev_user"`
}

type ClientEventRow struct {
	Client       string    `ch:"client"`
	CustomerID   string    `ch:"customer_id"`
	EventID      string    `ch:"event_id"`
	EventPayload string    `ch:"event_payload"`
	EventTime    time.Time `ch:"event_time"`
	EventType    int32     `ch:"event_type"`
}

// buildStartTimeHoursForQuery builds a list of hour-truncated times for ClickHouse primary key filtering.
// It returns the unique hours (including hour before and after each start time for boundary cases)
// and whether all conversation IDs have corresponding start times.
func buildStartTimeHoursForQuery(
	conversationIDs []string,
	conversationStartTimes map[string]time.Time,
) (startTimeHours []time.Time, hasAllStartTimes bool) {
	startTimeHoursSet := make(map[time.Time]struct{})
	hasAllStartTimes = true
	for _, convID := range conversationIDs {
		startTime, ok := conversationStartTimes[convID]
		if !ok {
			hasAllStartTimes = false
			continue
		}
		truncatedHour := startTime.Truncate(time.Hour)
		for _, hour := range []time.Time{
			truncatedHour.Add(-time.Hour), // hour before
			truncatedHour,                 // exact hour
			truncatedHour.Add(time.Hour),  // hour after
		} {
			if _, exists := startTimeHoursSet[hour]; !exists {
				startTimeHoursSet[hour] = struct{}{}
				startTimeHours = append(startTimeHours, hour)
			}
		}
	}
	return startTimeHours, hasAllStartTimes
}

func ReadConversationEventsByConversation(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	customerID, profileID string,
	conversationIDs []string,
	conversationStartTimes map[string]time.Time,
	eventTypes []int32,
) (map[string]map[string]*ConversationEventRow, error) {
	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		c.GetLogger().Errorf(ctx, "failed to connect ClickHouse: %v", err)
		return nil, err
	}

	// Build hour-truncated start times for primary key filtering.
	startTimeHours, hasAllStartTimes := buildStartTimeHoursForQuery(conversationIDs, conversationStartTimes)

	var rows driver.Rows
	if hasAllStartTimes && len(startTimeHours) > 0 {
		// Optimized query with primary key filter for efficient index usage.
		rows, err = clickhouseshared.QueryWithRetry(
			ctx, conn, `
			SELECT
				event_id, event_type, create_time,
				conversation_id, message_id
			FROM conversation_event_d
			WHERE toStartOfHour(conversation_start_time) IN (?)
				AND event_type IN (?)
				AND conversation_id IN (?)
			`, startTimeHours, eventTypes, conversationIDs)
	} else {
		// Fallback to original query when start times are not available or incomplete.
		// This is less efficient but ensures we don't miss any events.
		c.GetLogger().Warnf(ctx, "falling back to slower query pattern: have %d start times for %d conversations", len(conversationStartTimes), len(conversationIDs))
		rows, err = clickhouseshared.QueryWithRetry(
			ctx, conn, `
			SELECT
				event_id, event_type, create_time,
				conversation_id, message_id
			FROM conversation_event_d
			WHERE conversation_id IN (?)
				AND event_type IN (?)
			`, conversationIDs, eventTypes)
	}
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %v", err)
	}
	defer rows.Close()

	conversationEventsByConversationID := map[string]map[string]*ConversationEventRow{}
	for rows.Next() {
		row := ConversationEventRow{}
		vals := []interface{}{
			&row.EventID, &row.EventType, &row.CreateTime,
			&row.ConversationID, &row.MessageID,
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		conversationID := row.ConversationID
		if _, exists := conversationEventsByConversationID[conversationID]; !exists {
			conversationEventsByConversationID[conversationID] = map[string]*ConversationEventRow{}
		}
		conversationEventsByConversationID[conversationID][row.EventID] = &row
	}
	return conversationEventsByConversationID, nil
}

func BatchWriteConversationEventsIntoClickhouse(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	events []*conversationpb.ConversationEvent,
) ([]*conversationpb.ConversationEvent, error) {
	// Filter the whitelisted event types.
	whitelistedEvents := filterWhitelistedConversationEventTypes(events)
	if len(whitelistedEvents) == 0 {
		return whitelistedEvents, nil
	}
	c.GetLogger().Debugf(ctx, "BatchWriteEventsIntoClickhouse: writing %v events", len(whitelistedEvents))

	if err := batchWriteConversationEventsInternal(ctx, c, conversationName, conversation, whitelistedEvents); err != nil {
		return nil, err
	}
	return whitelistedEvents, nil
}

// Deprecated
func BatchWriteUserEventsIntoClickhouseLegacy(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	customerID string,
	events []*UserEventRow,
) error {
	conn, err := c.ConnectOrGetConnAuth(customerID)
	if err != nil {
		return fmt.Errorf("failed to connect to Clickhouse: %v", err)
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/events/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO user_event_d (%s)", clickhouseshared.ExtractCHFields(UserEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write user events to ClickHouse: failed to abort batch due to %v", err)
			}
		}
	}()

	if len(events) > 0 {
		for _, r := range events {
			err = batch.AppendStruct(r)
			if err != nil {
				return err
			}
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func WriteClientEventIntoClickhouse(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	event *clickhouseeventspb.ClientEvent,
) error {
	conn, err := c.ConnectOrGetConnAuth(event.GetCustomerId())
	if err != nil {
		return fmt.Errorf("failed to connect to Clickhouse: %v", err)
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/events/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO client_event_d (%s)", clickhouseshared.ExtractCHFields(ClientEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write client events to ClickHouse: failed to abort batch due to %v", err)
			}
		}
	}()

	clientEventRow, err := buildClientEventRow(event)
	if err != nil {
		return err
	}

	err = batch.AppendStruct(clientEventRow)
	if err != nil {
		return err
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func FilterWhitelistedAnalyticsEventNames(eventName string) bool {
	return slices.Contains(whitelistAnalyticsEventName, eventName)
}

func WriteAnalyticsEventIntoClickhouse(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	customerID, profileID, agentUserID, eventID, eventName string,
	eventProperties *structpb.Struct,
) error {
	if !FilterWhitelistedAnalyticsEventNames(eventName) {
		return nil
	}

	conn, err := c.ConnectOrGetConn(customerID, profileID)
	if err != nil {
		return fmt.Errorf("failed to connect to Clickhouse: %v", err)
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/events/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO analytics_event_d (%s)", clickhouseshared.ExtractCHFields(AnalyticsEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write analytics events to ClickHouse: failed to abort batch due to %v", err)
			}
		}
	}()

	// Determine if the user is a dev user based on context
	var isDevUser bool
	if *clickhouseEnableIsDevUser {
		isDevUser = isDevUserFromContext(ctx, customerID)
	}

	analyticsEventsRow, err := buildAnalyticsEventRow(customerID, profileID, agentUserID, eventID, eventName, eventProperties, isDevUser)
	if err != nil {
		return nil
	}
	if analyticsEventsRow != nil {
		err = batch.AppendStruct(analyticsEventsRow)
		if err != nil {
			return err
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func FilterWhitelistedSessionLogType(sessionLogType sessionpb.SessionLog_Type) bool {
	return slices.Contains(whitelistSessionLogType, sessionLogType)
}

func WriteSessionLogAsAnalyticsEventIntoClickhouse(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	log *sessionpb.SessionLog,
) error {
	if !FilterWhitelistedSessionLogType(log.GetType()) {
		return nil
	}

	sessionLogName, err := sessionpb.ParseSessionLogName(log.GetName())
	if err != nil {
		return err
	}
	conn, err := c.ConnectOrGetConn(sessionLogName.GetCustomerID(), sessionLogName.GetProfileID())
	if err != nil {
		return fmt.Errorf("failed to connect to Clickhouse: %v", err)
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/events/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO analytics_event_d (%s)", clickhouseshared.ExtractCHFields(AnalyticsEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write session logs to ClickHouse: failed to abort batch due to %v", err)
			}
		}
	}()

	agentUserID, eventName, eventPayloadString, err := convertSessionLogToAnalyticsEventInClickhouse(sessionLogName.GetSessionID(), log.GetLogPayload())
	if err != nil {
		return err
	}

	// Determine if the user is a dev user based on context
	var isDevUser bool
	if *clickhouseEnableIsDevUser {
		isDevUser = isDevUserFromContext(ctx, sessionLogName.GetCustomerID())
	}

	err = batch.AppendStruct(&AnalyticsEventRow{
		AgentUserID:  agentUserID,
		CreateTime:   time.Now(),
		CustomerID:   sessionLogName.GetCustomerID(),
		EventID:      sessionLogName.GetSessionLogID(),
		EventName:    eventName,
		EventPayload: eventPayloadString,
		EventTime:    log.GetCreateTime().AsTime(),
		ProfileID:    sessionLogName.GetProfileID(),
		IsDevUser:    isDevUser,
	})
	if err != nil {
		return err
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func batchWriteConversationEventsInternal(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	events []*conversationpb.ConversationEvent,
) error {
	conn, err := c.ConnectOrGetConn(conversationName.GetCustomerID(), conversationName.GetProfileID())
	if err != nil {
		return fmt.Errorf("failed to connect to Clickhouse: %v", err)
	}

	// The ClickHouse table schema:
	// https://github.com/cresta/clickhouse-schema/tree/main/events/migrations
	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn, fmt.Sprintf("INSERT INTO conversation_event_d (%s)", clickhouseshared.ExtractCHFields(ConversationEventRow{})))
	if err != nil {
		return err
	}
	defer func() {
		if !batch.IsSent() {
			if err := batch.Abort(); err != nil {
				c.GetLogger().Errorf(ctx, "Write conversation events to ClickHouse: failed to abort batch due to %v", err)
			}
		}
	}()

	conversationEventsRows, err := buildConversationEventRows(ctx, c, conversationName, conversation, events)
	if err != nil {
		return nil
	}
	if len(conversationEventsRows) > 0 {
		for _, r := range conversationEventsRows {
			err = batch.AppendStruct(r)
			if err != nil {
				return err
			}
		}
	}
	return clickhouseshared.SendBatchWithRetry(ctx, batch)
}

func buildConversationEventRows(
	ctx context.Context,
	c *eventsclient.ClickhouseEventsClient,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	events []*conversationpb.ConversationEvent,
) ([]*ConversationEventRow, error) {
	conversationEvents := []*ConversationEventRow{}
	for _, event := range events {
		// Parse event name.
		eventResourceName, err := conversationpb.ParseConversationEventName(event.GetName())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		// Parse associate message ID.
		messageID := ""
		if len(event.GetAssociatedMessage()) > 0 {
			messageName, err := conversationpb.ParseConversationMessageName(event.GetAssociatedMessage())
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			if len(messageName.MessageID) > 0 {
				messageID = messageName.MessageID
			}
		}

		eventType, err := convertAPIConversationEventTypeToClickhouse(event.Type)
		if err != nil {
			c.GetLogger().ErrorZap(ctx, "failed to convert API event type to Clickhouse", zap.String("event_type", event.Type.String()), zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		eventName, eventPayload, err := convertAPIConversationEventToClickhouseEventNameAndPayload(event)
		if err != nil {
			c.GetLogger().ErrorZap(ctx, "failed to convert API event to Clickhouse", zap.String("event_name", event.GetName()), zap.String("event_type", event.Type.String()), zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		eventPayloadString, err := convertEventPayloadProtoToString(eventPayload)
		if err != nil {
			c.GetLogger().ErrorZap(ctx, "failed to convert event payload to string", zap.String("event_name", event.GetName()), zap.String("event_type", event.Type.String()), zap.Error(err))
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}

		conversationEvents = append(conversationEvents, &ConversationEventRow{
			AgentUserID:           conversation.GetAgentUserId(),
			ConversationID:        conversationName.ConversationID,
			ConversationStartTime: conversation.GetStartTime().AsTime(),
			CreateTime:            time.Now(),
			CustomerID:            conversationName.CustomerID,
			EventID:               eventResourceName.EventID,
			EventName:             eventName,
			EventPayload:          eventPayloadString,
			EventType:             int32(eventType.Number()),
			MessageID:             messageID,
			ProfileID:             conversationName.ProfileID,
			UsecaseID:             conversation.GetUsecaseId(),
		})
	}
	return conversationEvents, nil
}

func buildAnalyticsEventRow(
	customerID, profileID, agentUserID, eventID, eventName string,
	eventProperties *structpb.Struct,
	isDevUser bool,
) (*AnalyticsEventRow, error) {
	clickhouseEventName, clickhouseEventPayload, err := convertAPIAnalyticsEventToClickhouseEventNameAndPayload(eventName, eventProperties)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	clickhouseEventPayloadString, err := convertEventPayloadProtoToString(clickhouseEventPayload)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	return &AnalyticsEventRow{
		AgentUserID:  agentUserID,
		CreateTime:   time.Now(),
		CustomerID:   customerID,
		EventID:      eventID,
		EventName:    clickhouseEventName,
		EventPayload: clickhouseEventPayloadString,
		EventTime:    time.Now(),
		ProfileID:    profileID,
		IsDevUser:    isDevUser,
	}, nil
}

func filterWhitelistedConversationEventTypes(events []*conversationpb.ConversationEvent) []*conversationpb.ConversationEvent {
	whitelistedEvents := []*conversationpb.ConversationEvent{}
	for _, event := range events {
		if slices.Contains(whitelistConversationEventTypes, event.Type) {
			whitelistedEvents = append(whitelistedEvents, event)
		}
	}
	return whitelistedEvents
}

// isDevUserFromContext determines if the authenticated user is a dev user
// based on the authentication context and customer information.
func isDevUserFromContext(ctx context.Context, customerID string) bool {
	// Do not set is_dev_user for cresta internal endpoints.
	if customerID == crestaString {
		return false
	}

	authEntity, err := auth.GetPrincipalEntityFromAuthContext(ctx)
	if err != nil {
		// If we can't get auth context, assume not dev user
		return false
	}

	// Server, API key, or service contexts are not dev users in this context
	if authEntity.IsServer() || authEntity.IsAPIKey() {
		return false
	}

	// isDevUser determines if a user is a dev user based on the same logic as auth service.
	// User is considered DevUser if it satisfies one of the conditions:
	// 1. has SuperAdmin role
	// 2. has cresta email
	if authEntity.HasAnyRole(authpb.AuthProto_SUPER_ADMIN) {
		// note that we don't check for CrestaAdmin custom role here because we count on double writing still
		return true
	}

	// Get user information to check dev user status
	apiUser, err := authEntity.ToAPIUser()
	if err != nil {
		return false
	}

	return apiUser.Email != "" && crestaEmailRegex.MatchString(apiUser.Email)
}

func buildClientEventRow(event *clickhouseeventspb.ClientEvent) (*ClientEventRow, error) {
	// Convert event_payload Struct to JSON string
	eventPayloadString := ""
	if event.GetEventPayload() != nil {
		payloadBytes, err := protojson.Marshal(event.GetEventPayload())
		if err != nil {
			return nil, fmt.Errorf("failed to convert event payload to string: %v", err)
		}
		eventPayloadString = string(payloadBytes)
	}

	return &ClientEventRow{
		Client:       event.GetClient(),
		CustomerID:   event.GetCustomerId(),
		EventID:      event.GetEventId(),
		EventPayload: eventPayloadString,
		EventTime:    event.GetEventTime().AsTime(),
		EventType:    event.GetEventType(),
	}, nil
}
