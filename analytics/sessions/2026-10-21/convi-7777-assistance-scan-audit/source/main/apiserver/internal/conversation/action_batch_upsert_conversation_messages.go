package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	aicommonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/common"
	orchestrator_pb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/orchestrator"
	redactionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/redaction"
	featurespb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/config/features"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/extrawork"
	"github.com/cresta/shared-go/framework/stats"
	"github.com/cresta/shared-go/testing/testvalue"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/cresta/shared-go/util/errgroup"
	"github.com/cresta/shared-go/util/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/plugin/dbresolver"

	"github.com/cresta/go-servers/apiserver/shared/deploymentclient"
	"github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/common"
	dbconvpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/conversation"
	redisconvpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/redis/conversation"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	sharedconversation "github.com/cresta/go-servers/shared/conversation"
	"github.com/cresta/go-servers/shared/conversation/converter"
	"github.com/cresta/go-servers/shared/dao"
	dbconverter "github.com/cresta/go-servers/shared/db/converter"
	"github.com/cresta/go-servers/shared/db/pgerrors"
	"github.com/cresta/go-servers/shared/piiredaction"
	"github.com/cresta/go-servers/shared/utils"
)

type dbUpsertResult int

const (
	// The message does not exist. A new db row is inserted
	dbResultInserted dbUpsertResult = iota

	// The message already exists and is updated. Both the existing and the
	// current messages are incomplete messages.
	dbResultUpdateIncomplete

	// The message already exists and is updated from incomplete state to
	// complete state.
	//
	// NOTE: If the feature flag of skip_db_for_incomplete_message is enabled,
	// this result is not returned.
	dbResultUpdateIncompleteToComplete

	// The message already exists and is updated. Both the existing and the
	// current messages are complete messages.
	dbResultUpdateComplete

	// The message already exists and is not updated because there is no
	// change.
	dbResultNoop

	// Only thrown with errors.
	dbResultError
)

var (
	updatedMessagesCounter  = stats.NewMetric("apiserver.conversation.batch_upsert_messages.updated.count")
	insertedMessagesCounter = stats.NewMetric("apiserver.conversation.batch_upsert_messages.inserted.count")
	skippedMessagesCounter  = stats.NewMetric("apiserver.conversation.batch_upsert_messages.skipped.count")
	latencyPerMessageMetric = stats.NewMetric("apiserver.conversation.batch_upsert_messages.latency_per_message_ms")

	piiRedactionRequestCounter = stats.NewMetric("apiserver.conversation.pii_redaction.request.count")
	piiRedactionErrorCounter   = stats.NewMetric("apiserver.conversation.pii_redaction.error.count")
	piiRedactionDurationMetric = stats.NewMetric("apiserver.conversation.pii_redaction.duration_ms")

	piiRedactionWithOrchEntitiesRequestCounter = stats.NewMetric("apiserver.conversation.pii_redaction_with_orch_entities.request.count")
	piiRedactionWithOrchEntitiesErrorCounter   = stats.NewMetric("apiserver.conversation.pii_redaction_with_orch_entities.error.count")

	piiRedactionWithOrchEntityBackfillRequestCounter = stats.NewMetric("apiserver.conversation.pii_redaction_with_orch_entity_backfill.request.count")
	piiRedactionWithOrchEntityBackfillErrorCounter   = stats.NewMetric("apiserver.conversation.pii_redaction_with_orch_entity_backfill.error.count")
)

var (
	orchestratorTimeout               = envflag.Duration("ORCHESTRATOR_TIMEOUT", 5*time.Minute, "Timeout for calling orchestrator")
	transferMessageThreshold          = envflag.Int("TRANSFER_MESSAGE_THRESHOLD", 5, "If the number of new messages in a batch upsert request is equal to or greater than this number, they are considered transferred messages. AI services may have different treatment for transferred messages.")
	forceSynchronousTransfer          = envflag.Bool("FORCE_SYNCHRONOUS_TRANSFER", false, "If enabled, enforce processing transferred conversation first and then process incoming messages.")
	processTransferMessageWait        = envflag.Duration("PROCESS_TRANSFER_MESSAGE_WAIT_SECONDS", time.Second*2, "Allow Orchestrator up to N seconds to process transferred conversation, before process incoming messages.")
	strictHTMLMode                    = envflag.Bool("STRICT_HTML_MODE", false, "If enabled, agent and system message texts are not considered HTML unless explicitly set to HTML")
	piiRedactionTimeout               = envflag.Duration("PII_REDACTION_TIMEOUT", 30*time.Second, "Timeout for realtime PII redaction")
	runCloseExtraWorkForUpsertMsg     = envflag.Bool("RUN_CLOSE_EXTRA_WORK_FOR_UPSERT_MSG", false, "If enabled, upserting messages to closed chat/voice conversations will run CloseConversationExtraWorkWorkflow.")
	alwaysSkipDBForIncompleteMessages = envflag.Bool("ALWAYS_SKIP_DB_FOR_INCOMPLETE_MESSAGES", false, "If enabled, always skip DB incomplete messages even if conversation.skip_db_for_incomplete_message feature flags is not enabled. Used to generally roll out the DB performance improvement.")
	fillAgentIDFromSpeakerInfo        = envflag.Bool("FILL_AGENT_ID_FROM_SPEAKER_INFO", true, "If enabled, fill agent ID from speaker info for agent messages if agent ID is not provided. This is used to handle the case when the client does not have agent user ID available but has platform speaker ID available.")
)

var (
	//nolint:gosec // False positive
	TestValueCallOrchResult = "go-servers:apiserver:conversation:batch_upsert_messages:call_orchestrator_result"
	digitsRegex             = regexp.MustCompile(`(?i)\d+|\b(zero|one|two|three|four|five|six|seven|eight|nine|ten|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety)\b`)
)

func (s *ServiceImpl) BatchUpsertConversationMessages(ctx context.Context, req *conversationpb.BatchUpsertConversationMessagesRequest) (*conversationpb.BatchUpsertConversationMessagesResponse, error) {
	startTime := time.Now()
	defer func() {
		if len(req.ConversationMessages) > 0 {
			latencyPerMessageMetric.Distribution(ctx, float64(time.Since(startTime).Milliseconds())/float64(len(req.ConversationMessages)))
		}
	}()

	ctx = tagVoiceIntegrationUpstreamID(ctx)
	conversationName, err := s.parseAndValidateBatchUpsertConversationMessagesRequest(ctx, req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeConversationMutation(ctx, "BatchUpsertConversationMessages", conversationName); err != nil {
		return nil, err
	}

	// Read conversation and existing messages
	conversation, cachedMsgs, err := s.conversationCache.ListMessages(ctx, conversationName, sharedconversation.NoListMessagesLimit)
	// Check if the conversation exists
	var notFoundError *sharedconversation.NotFoundError
	if errors.As(err, &notFoundError) {
		s.logger.Infof(ctx, err.Error())
		return nil, status.Errorf(codes.NotFound, "conversation %s is not found", req.Parent)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to read conversation and message from cache: %v", err)
	}

	// Read cached events
	_, cachedEvents, err := s.conversationCache.ListEvents(ctx, conversationName, sharedconversation.NoListEventsLimit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to read conversation and events from cache: %v", err)
	}
	cachedIncompleteMsgs, err := s.conversationCache.ListIncompleteMessages(ctx, conversationName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to read incomplete conversation from cache: %v", err)
	}

	// upsert the conversations.
	msgsToUpsert := req.ConversationMessages
	err = validateOrPopulateSequenceIDs(conversation, msgsToUpsert)
	if err != nil {
		return nil, err
	}
	messageSyncStatus := req.GetSyncStatus()
	if !s.getConfigs(ctx, conversationName, conversation.UsecaseId).GetConversation().GetEnableNewTransfer() {
		// Overwrite to old behavior when flag is not enabled.
		messageSyncStatus = conversationpb.BatchUpsertConversationMessagesRequest_MESSAGE_SYNC_STATUS_UNSPECIFIED
	}
	upserted, err := s.upsertMessagesInDB(
		ctx,
		msgsToUpsert,
		cachedMsgs,
		cachedIncompleteMsgs,
		conversationName,
		conversation,
		req.SkipUpdate,
		messageSyncStatus)
	if err != nil {
		return nil, err
	}

	if err = s.afterUpsertMessagesToDB(
		ctx,
		conversationName,
		conversation,
		cachedMsgs,
		cachedIncompleteMsgs,
		upserted,
		cachedEvents,
		messageSyncStatus); err != nil {
		return nil, err
	}

	return &conversationpb.BatchUpsertConversationMessagesResponse{}, nil
}

func (s *ServiceImpl) parseAndValidateBatchUpsertConversationMessagesRequest(ctx context.Context, req *conversationpb.BatchUpsertConversationMessagesRequest) (*conversationpb.ConversationName, error) {
	conversationName, err := conversationpb.ParseConversationName(req.Parent)
	if err != nil {
		return nil, err
	}
	if err := s.validateConversationName(ctx, conversationName); err != nil {
		return nil, err
	}
	return conversationName, nil
}

// upsertMessagesInDB inserts or updates messages to the DB.
//
// The implementation upserts the messages in 3 steps:
//  1. Look up existing messages and update them.
//  2. Insert non-existent messages to the DB. Some insertion may fail with unique violation error if there
//     are concurrent insertion for the same message. Those messages are collected for re-update.
//  3. Update the messages failed to be inserted due to unique violoation.
//
// This strategy works under the following assumptions:
//   - Batch upsert is not atomic operation. We allow some messages successfully upserted and others fail.
//   - The messages won't get deleted between step 2 and 3.
func (s *ServiceImpl) upsertMessagesInDB(
	ctx context.Context,
	msgsToUpsert []*conversationpb.ConversationMessage,
	cachedMsgs []*sharedconversation.CacheMessage,
	cachedIncompleteMsgs map[redisconvpb.ConversationMessage_SpeakerRole]*sharedconversation.CacheMessage,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	skipUpdate bool,
	messageSyncStatus conversationpb.BatchUpsertConversationMessagesRequest_MessageSyncStatus,
) (upserted map[dbUpsertResult][]*dbmodel.Messages, err error) {
	idToExistingMsg := make(map[string]*dbmodel.Messages, len(cachedMsgs)+len(cachedIncompleteMsgs))
	skipIncomplete := s.getFeatureFlags(ctx, conversationName, config.WithOptionalUsecaseID(conversation.GetUsecaseId())).GetSkipDbForIncompleteMessage()
	if *alwaysSkipDBForIncompleteMessages {
		skipIncomplete = true
	}
	conversationParticipants := NewConversationParticipants(conversationName.ConversationID, conversation.Participants)
	redactPreviousParticipants := s.redactEmailParticipantsInPreviousMessages(ctx, conversationName, conversation)
	conversationParticipants.RedactEmailParticipantsInPreviousMessages = redactPreviousParticipants
	// Adding the incomplete msgs first to make sure the if there is
	// duplicate the complete one overrides the incomplete one.
	for _, m := range cachedIncompleteMsgs {
		idToExistingMsg[m.MessageId], err = convertCachedMsgToDB(conversationName, m, false)
		if err != nil {
			return nil, err
		}
	}
	for _, m := range cachedMsgs {
		idToExistingMsg[m.MessageId], err = convertCachedMsgToDB(conversationName, m, true)
		if err != nil {
			return nil, err
		}
	}
	dbMsgsToUpsert, err := s.convertToDBMsgSlice(ctx, msgsToUpsert, conversationName, conversation, conversationParticipants)
	if err != nil {
		return nil, err
	}

	// Step 1, update existing messages. Also get the new messages that do not exists yet.
	msgsToInsert, updated, err := s.batchUpdateExistingMsgs(ctx, conversationName, dbMsgsToUpsert, idToExistingMsg, skipUpdate, skipIncomplete, redactPreviousParticipants)
	if err != nil {
		return nil, err
	}
	upserted = updated

	// Step 2, insert new messages
	markTransferredMsgs(msgsToInsert, messageSyncStatus)
	conflictMsgs, inserted, err := s.batchInsertMsgs(ctx, msgsToInsert, skipIncomplete)
	if err != nil {
		return nil, err
	}
	for _, m := range inserted {
		if isFinalized(m) {
			upserted[dbResultInserted] = append(upserted[dbResultInserted], m)
		} else {
			upserted[dbResultUpdateIncomplete] = append(upserted[dbResultUpdateIncomplete], m)
		}
	}

	// Step 3. There are race conditions - some of the new messages are
	// inserted by concurrent transactions. We start over from the
	// loop and they should be updated if there are difference. In
	// some chat platforms the message agent may get updated.
	notUpdateableMsgs, updatedForConflict, err := s.batchUpdateExistingMsgs(ctx, conversationName, conflictMsgs, nil, skipUpdate, skipIncomplete, redactPreviousParticipants)
	if err != nil {
		return nil, err
	}
	if len(notUpdateableMsgs) > 0 {
		return nil, status.Errorf(
			codes.Internal,
			"cannot update %v conflict messages for conversation %v. The first message has ID: %v, sequence ID: %v",
			len(notUpdateableMsgs),
			conversationName,
			notUpdateableMsgs[0].ConversationMessageID.String,
			notUpdateableMsgs[0].PlatformSequenceID)
	}
	for r, m := range updatedForConflict {
		upserted[r] = append(upserted[r], m...)
	}

	// Step 4, update conversation if there are any changes.
	err = s.maybeUpdateConversationForUpsertMsgs(ctx, conversationName, dbMsgsToUpsert, conversationParticipants)
	if err != nil {
		// Fail open as the message upsert is more important.
		s.logger.Errorf(ctx, "failed to update conversation %v: %v", conversationName, err)
	}

	return upserted, nil
}

func (s *ServiceImpl) convertToDBMsgSlice(
	ctx context.Context,
	msgsToUpsert []*conversationpb.ConversationMessage,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	conversationParticipants *ConversationParticipants,
) ([]*dbmodel.Messages, error) {
	agentIDToUser := make(map[string]dao.UserInterface)
	dbMsgs := make([]*dbmodel.Messages, len(msgsToUpsert))
	now := time.Now()
	usecaseName := &customerpb.UsecaseName{
		CustomerID: conversationName.CustomerID,
		ProfileID:  conversationName.ProfileID,
		UsecaseID:  conversation.UsecaseId,
	}
	var emailSettings *EmailSettings
	for i, msg := range msgsToUpsert {
		msgName, agentUserName, err := s.parseAndValidateConversationMessageName(ctx, conversationName, msg)
		if err != nil {
			return nil, err
		}
		chatAgentID := sql.NullInt32{}
		if conversation.LegacyAgentId > 0 {
			chatAgentID = dbconverter.ConvertToNullInt32(conversation.LegacyAgentId)
		}
		skipAIAgent := s.getFeatureFlags(ctx, conversationName, config.WithOptionalUsecaseID(conversation.GetUsecaseId())).GetConvMsgSkipAiAgent() ||
			s.getFeatureFlags(ctx, conversationName, config.WithOptionalUsecaseID(conversation.GetUsecaseId())).GetSkipOrchestrator() ||
			conversation.GetFeatureOverride().GetSkipAnnotations()
		dbMsg, err := s.converter.ConvertAPIToDBMessage(
			msg,
			msgName,
			conversation.LegacyConversationId,
			conversation.AgentUserId,
			chatAgentID,
			skipAIAgent,
			conversation.StartTime.AsTime(),
			*strictHTMLMode,
			now)
		if err != nil {
			return nil, err
		}
		if dbMsg.Metadata.Message.Email != nil {
			if emailSettings == nil {
				emailSettings, err = s.getEmailSettings(ctx, usecaseName, conversation.LanguageCode)
				if err != nil {
					return nil, err
				}
			}
			if err := s.updateDBMsgForEmail(ctx, usecaseName, dbMsg, conversationParticipants, emailSettings); err != nil {
				return nil, err
			}
		}
		// Handle agent
		if agentUserName != nil {
			// NOTE(caibin.chen): We don't need this layer of
			// caching after fully migrated to user client because
			// it already handles caching.
			agentUser, ok := agentIDToUser[agentUserName.UserID]
			if !ok {
				agentUser, agentUserName, err = s.lookupAgent(ctx, conversationName, agentUserName)
				if err != nil {
					return nil, err
				}
				agentIDToUser[agentUserName.UserID] = agentUser
			}
			dbMsg.CurrentAgentID = dbconverter.ConvertToNullInt32(int32(agentUser.GetUserId()))
			dbMsg.AgentUserID = dbconverter.ConvertToNullString(agentUserName.UserID)
			agentParticipant := &dbconvpb.ConversationParticipant{
				DisplayName: agentUser.GetDisplayName(),
				UserId:      dbMsg.AgentUserID.String,
				Occurrence:  &dbconvpb.ConversationParticipant_Occurrence{Agent: true},
				SpeakerRole: common.SpeakerRole_AGENT,
			}
			generateParticipantID(conversationName.ConversationID, agentParticipant)
			conversationParticipants.add(agentParticipant)
		} else if *fillAgentIDFromSpeakerInfo && msg.Speaker == conversationpb.ConversationMessage_AGENT && msg.SpeakerInfo.GetPlatformSpeakerId() != "" {
			platformID := msg.SpeakerInfo.GetPlatformSpeakerId()
			agentUser, ok := agentIDToUser[platformID]
			var resolvedName *userpb.UserName
			if !ok {
				var platformAgentUser dao.UserInterface
				platformAgentUser, resolvedName, err = s.lookupAgentByPlatformID(ctx, conversationName, platformID)
				if err != nil {
					return nil, err
				}
				if platformAgentUser != nil {
					agentUser = platformAgentUser
					agentIDToUser[platformID] = agentUser
				}
			}
			if agentUser != nil {
				dbMsg.CurrentAgentID = dbconverter.ConvertToNullInt32(int32(agentUser.GetUserId()))
				dbMsg.AgentUserID = dbconverter.ConvertToNullString(resolvedName.UserID)
				agentParticipant := &dbconvpb.ConversationParticipant{
					DisplayName: agentUser.GetDisplayName(),
					UserId:      dbMsg.AgentUserID.String,
					Occurrence:  &dbconvpb.ConversationParticipant_Occurrence{Agent: true},
					SpeakerRole: common.SpeakerRole_AGENT,
				}
				generateParticipantID(conversationName.ConversationID, agentParticipant)
				conversationParticipants.add(agentParticipant)
			}
		}

		// Normalize transrcipt language
		transcriptLanguage := dbMsg.Metadata.Message.GetTranscriptionInfo().GetTranscribedLanguage()
		if transcriptLanguage != "" {
			dbMsg.Metadata.Message.TranscriptionInfo.NormalizedLanguageCode = s.normalizeLanguageCode(ctx, conversationName, transcriptLanguage, ignoreUnknownLanguage)
		}
		dbMsgs[i] = dbMsg
	}
	return dbMsgs, nil
}

func (s *ServiceImpl) parseAndValidateConversationMessageName(ctx context.Context, conversationName *conversationpb.ConversationName, msg *conversationpb.ConversationMessage) (*conversationpb.ConversationMessageName, *userpb.UserName, error) {
	if err := s.validateMessageText(ctx, conversationName, msg.Text); err != nil {
		return nil, nil, err
	}
	msgName, err := conversationpb.ParseConversationMessageName(msg.Name)
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if msgName.CustomerID != conversationName.CustomerID ||
		msgName.ProfileID != conversationName.ProfileID ||
		msgName.ConversationID != conversationName.ConversationID {
		return nil, nil, status.Errorf(codes.InvalidArgument, "message %v does not belong to conversation %v", msg.Name, conversationName)
	}
	if err := s.validateMessageID(ctx, conversationName, msgName.MessageID); err != nil {
		return nil, nil, err
	}
	var agentUserName *userpb.UserName
	if msg.Agent != "" {
		agentUserName, err = userpb.ParseUserName(msg.Agent)
		if err != nil {
			return nil, nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if err := s.validateUserName(ctx, conversationName, agentUserName); err != nil {
			return nil, nil, err
		}
	}
	return msgName, agentUserName, nil
}

func (s *ServiceImpl) batchUpdateExistingMsgs(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	msgsToUpsert []*dbmodel.Messages,
	idToExistingMsgFromCache map[string]*dbmodel.Messages,
	skipUpdate bool,
	skipIncomplete bool,
	updateEmailParticipants bool,
) (msgsToInsert []*dbmodel.Messages, updatedMsgs map[dbUpsertResult][]*dbmodel.Messages, err error) {
	if len(msgsToUpsert) == 0 {
		return nil, nil, nil
	}
	db := s.appsDB.DB(ctx)

	idToExistingMsg := idToExistingMsgFromCache
	if idToExistingMsg == nil {
		// This is calling by upsert step 3 - update the messages with insert conflict.
		idToExistingMsg = make(map[string]*dbmodel.Messages, len(msgsToUpsert))
		// Updateing messages failed to insert. Query from db for their latest status.
		msgIDs := make([]string, len(msgsToUpsert))
		for i, dbMsg := range msgsToUpsert {
			msgIDs[i] = dbMsg.ConversationMessageID.String
		}
		// Fetch existing messages
		var existingDBMsgs []*dbmodel.Messages
		// The messages are likely inserted by another batch upsert
		// request with race condition. We need to read from the read
		// replicas to find the just-inserted messages.
		result := db.Clauses(dbresolver.Write).Find(
			&existingDBMsgs,
			"customer_id = ? AND profile_id = ? AND conversation_id = ? AND conversation_message_id in ?",
			conversationName.CustomerID,
			conversationName.ProfileID,
			conversationName.ConversationID,
			msgIDs)
		if result.Error != nil {
			return nil, nil, status.Errorf(codes.Internal, "undable to read existing messages: %v", result.Error)
		}
		s.logger.Debugf(ctx, "batch upsert msg: found %v existing messages", len(existingDBMsgs))
		for _, m := range existingDBMsgs {
			idToExistingMsg[m.ConversationMessageID.String] = m
		}
	}
	msgsToInsert, msgsToUpdateMap, _ := splitMsgSlice(msgsToUpsert, idToExistingMsg, updateEmailParticipants)

	msgsToUpdate := msgsToUpdateMap[dbResultUpdateComplete]
	if skipIncomplete && idToExistingMsgFromCache != nil {
		// When skipping incomplete messages is true and idToExistingMsgFromCache
		// is not nil, the incomplete messages are not in DB, only in cache.
		// Trying to update them from incomplete to complete is a no-op.
		// We need to insert them into DB instead.
		//
		// NOTE: It's possible that there are incomplete messages in DB
		// when skipIncomplete is true. This can happen when the feature
		// flag is flipped between true and false. Some servers have already
		// picked up the flag while some haven't. After it's fully rolled out,
		// the idToExistingMsgFromCache != nil check can be removed.
		//
		// Note that msgsToUpdateMap[dbResultUpdateIncompleteToComplete]
		// only contains partial information for the message for update.
		// We need the append the original messages from msgsToUpsert
		completedMsgIDs := set.New[string]()
		for _, m := range msgsToUpdateMap[dbResultUpdateIncompleteToComplete] {
			completedMsgIDs.Add(m.ConversationMessageID.String)
		}
		for _, m := range msgsToUpsert {
			if completedMsgIDs.Has(m.ConversationMessageID.String) {
				msgsToInsert = append(msgsToInsert, m)
			}
		}
		delete(msgsToUpdateMap, dbResultUpdateIncompleteToComplete)
	} else {
		msgsToUpdate = append(msgsToUpdate, msgsToUpdateMap[dbResultUpdateIncomplete]...)
		msgsToUpdate = append(msgsToUpdate, msgsToUpdateMap[dbResultUpdateIncompleteToComplete]...)
	}
	skippedMessagesCounter.Count(ctx, int64(len(msgsToUpsert)-len(msgsToInsert)-len(msgsToUpdate)))

	if skipUpdate {
		skippedMessagesCounter.Count(ctx, int64(len(msgsToUpdate)))
		msgsToUpdateMap = map[dbUpsertResult][]*dbmodel.Messages{
			dbResultNoop: msgsToUpdate,
		}
	} else {
		for _, m := range msgsToUpdate {
			result := db.Where(&dbmodel.Messages{
				CustomerID:            m.CustomerID,
				ProfileID:             m.ProfileID,
				ConversationID:        m.ConversationID,
				ConversationMessageID: m.ConversationMessageID,
			}).Updates(m)
			if result.Error != nil {
				return nil, nil, result.Error
			}
		}
		updatedMessagesCounter.Count(ctx, int64(len(msgsToUpdate)))
	}
	return msgsToInsert, msgsToUpdateMap, nil
}

func (s *ServiceImpl) batchInsertMsgs(ctx context.Context, msgs []*dbmodel.Messages, skipIncomplete bool) (conflicts []*dbmodel.Messages, inserted []*dbmodel.Messages, err error) {
	db := s.appsDB.DB(ctx)
	inserted = make([]*dbmodel.Messages, 0, len(msgs))
	for _, m := range msgs {
		if skipIncomplete && !isFinalized(m) {
			inserted = append(inserted, m)
			continue
		}
		result := db.Create(m)
		if result.Error == nil {
			inserted = append(inserted, m)
			continue
		}
		if pgerrors.IsUniqueViolation(result.Error) {
			conflicts = append(conflicts, m)
		} else {
			return nil, nil, status.Errorf(codes.Internal, "unable to insert message: %v", result.Error)
		}
	}
	insertedMessagesCounter.Count(ctx, int64(len(msgs)-len(conflicts)))
	return conflicts, inserted, nil
}

func splitMsgSlice(
	msgsToUpsert []*dbmodel.Messages,
	idToExistingMsg map[string]*dbmodel.Messages,
	updateEmailParticipants bool,
) (msgsToInsert []*dbmodel.Messages, msgsToUpdate map[dbUpsertResult][]*dbmodel.Messages, err error) {
	// there may be dupdate between cachedMsgs and cachedIncompleteMsgs, so we reserve more spaces for msgsToInsert
	msgsToInsert = make([]*dbmodel.Messages, 0)
	msgsToUpdate = make(map[dbUpsertResult][]*dbmodel.Messages, 0)
	for _, msg := range msgsToUpsert {
		if existingMsg, ok := idToExistingMsg[msg.ConversationMessageID.String]; ok {
			toUpdate, result, err := buildMessageToUpdate(existingMsg, msg, updateEmailParticipants)
			if err != nil {
				return nil, nil, err
			}
			if result != dbResultNoop {
				msgsToUpdate[result] = append(msgsToUpdate[result], toUpdate)
			}
		} else {
			msgsToInsert = append(msgsToInsert, msg)
		}
	}
	return msgsToInsert, msgsToUpdate, err
}

func convertCachedMsgToDB(conversationName *conversationpb.ConversationName, cachedMsg *sharedconversation.CacheMessage, isFinalized bool) (*dbmodel.Messages, error) {
	dbMsg := &dbmodel.Messages{
		CustomerID:            dbconverter.ConvertToNullString(conversationName.CustomerID),
		ProfileID:             dbconverter.ConvertToNullString(conversationName.ProfileID),
		ConversationID:        dbconverter.ConvertToNullString(conversationName.ConversationID),
		ConversationMessageID: dbconverter.ConvertToNullString(cachedMsg.MessageId),
		PlatformSequenceID:    cachedMsg.SequencePosition,
		ID:                    int64(cachedMsg.LegacyMessageId),
		Text:                  cachedMsg.Text,
		IsFinalized:           dbconverter.ConvertToNullBool(isFinalized),
		CreatedAt:             cachedMsg.CreateTime.AsTime(),
	}
	if cachedMsg.Metadata != nil {
		dbMsg.Metadata.Message = proto.Clone(cachedMsg.Metadata).(*dbconvpb.MessageMetadata)
	} else {
		// TODO(caibin.chen): This fallback logic can be removed after 2024/10/1.
		dbMsg.Metadata = dbconvpb.MessageMetadataSQL{
			Message: &dbconvpb.MessageMetadata{
				IsTransferred: cachedMsg.IsTransferred, //nolint
				Redaction:     &dbconvpb.MessageRedaction{},
			},
		}
		switch cachedMsg.RedactionState {
		case redisconvpb.ConversationMessage_REDACTED_NOT_CLEANED_UP:
			dbMsg.Metadata.Message.OriginalText = cachedMsg.Text
			dbMsg.Metadata.Message.Redaction = &dbconvpb.MessageRedaction{
				State: dbconvpb.MessageRedaction_REDACTED_NOT_CLEANED_UP,
			}
		case redisconvpb.ConversationMessage_REDACTED_CLEANED_UP:
			dbMsg.Metadata.Message.Redaction = &dbconvpb.MessageRedaction{
				State: dbconvpb.MessageRedaction_REDACTED_CLEANED_UP,
			}
		case redisconvpb.ConversationMessage_UNREDACTED, redisconvpb.ConversationMessage_REDACTION_STATE_UNSPECIFIED:
			// No op
		default:
			return nil, fmt.Errorf("unknown redaction status %v", cachedMsg.RedactionState)
		}
	}
	if cachedMsg.PlatformCreateTime != nil {
		dbMsg.PlatformTimestamp = cachedMsg.PlatformCreateTime.AsTime()
	} else {
		// This may happen for old cached messages. Can remove this
		// block after rolling out for 1 month.
		dbMsg.PlatformTimestamp = cachedMsg.CreateTime.AsTime()
	}
	if cachedMsg.AgentUserId != "" {
		dbMsg.AgentUserID = dbconverter.ConvertToNullString(cachedMsg.AgentUserId)
	}
	if cachedMsg.TranscriptionInfo != nil {
		words := cachedMsg.TranscriptionInfo.TranscriptionWords
		jsons := make([]*sharedconversation.ConfidenceJSON, len(words))
		for i, word := range words {
			jsons[i] = &sharedconversation.ConfidenceJSON{
				Word:        word.Word,
				Confidence:  word.Confidence,
				StartTimeMs: float32(word.StartOffset.AsDuration().Milliseconds()),
				EndTimeMs:   float32(word.EndOffset.AsDuration().Milliseconds()),
			}
		}
		encoded, err := json.Marshal(jsons)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "batch upsert msgs: failed to generate confidence JSON: %v", err)
		}
		dbMsg.ConfidenceJSON = sql.NullString{String: string(encoded), Valid: true}
		if len(jsons) > 0 {
			dbMsg.StartTimeMs = dbconverter.ConvertToNullInt32(int32(jsons[0].StartTimeMs))
			dbMsg.EndTimeMs = dbconverter.ConvertToNullInt32(int32(jsons[len(jsons)-1].EndTimeMs))
		}
	}
	switch cachedMsg.Speaker {
	case redisconvpb.ConversationMessage_AGENT:
		dbMsg.SpeakerRole = sharedconversation.DBSpeakerRoleAgent
	case redisconvpb.ConversationMessage_VISITOR:
		dbMsg.SpeakerRole = sharedconversation.DBSpeakerRoleVisitor
	case redisconvpb.ConversationMessage_SYSTEM_MESSAGE:
		dbMsg.SpeakerRole = sharedconversation.DBSpeakerRoleSystem
	case redisconvpb.ConversationMessage_BOT:
		dbMsg.SpeakerRole = sharedconversation.DBSpeakerRoleBot
	default:
		return nil, status.Errorf(codes.Internal, "batch upsert msgs: unknown speaker role from cached msg: %v", cachedMsg.Speaker)
	}
	return dbMsg, nil
}

// buildMessageToUpdate builds the messesage to be used for updating the DB row.
// If no update is needed, the returned result is dbResultNoop.
// updateEmailParticipants also rewrites the stored email participants when they
// differ (redactEmailParticipantsInPreviousMessages flag).
func buildMessageToUpdate(existingMsg, newMsg *dbmodel.Messages, updateEmailParticipants bool) (*dbmodel.Messages, dbUpsertResult, error) {
	toUpdate := *existingMsg
	if existingMsg.Metadata.Message.GetCreatedBy() == common.DBWriterEnum_DB_WRITER_UNSPECIFIED {
		// TODO(caibin.chen): This logic can be removed after 2024/10/01.
		//
		// existingMsg may be converted from cache message. Old cache
		// messages don't have full metadata and results in the
		// converted existingMsg to lack of certain metadata fields. To
		// avoid overriding the DB metadata column with the incomplete
		// metadata from existingMsg, we override the metadata column as
		// nil.
		toUpdate.Metadata.Message = nil
	}
	hasUpdate := false
	finalizedChanged := false
	if toUpdate.Text != newMsg.Text && toUpdate.Metadata.Message.GetOriginalText() != newMsg.Text {
		hasUpdate = true
		toUpdate.Text = newMsg.Text
		// Make a clone to avoid modifying the existing message's metadata.
		toUpdate.Metadata.Message = proto.Clone(existingMsg.Metadata.Message).(*dbconvpb.MessageMetadata)
		if !piiredaction.NeedsRedaction(existingMsg) {
			// Clear redaction info to re-trigger PII redaction.
			toUpdate.Metadata.Message = proto.Clone(existingMsg.Metadata.Message).(*dbconvpb.MessageMetadata)
			toUpdate.Metadata.Message.Redaction = nil
			toUpdate.Metadata.Message.OriginalText = ""
		}
	}
	if toUpdate.ConfidenceJSON != newMsg.ConfidenceJSON {
		hasUpdate = true
		toUpdate.ConfidenceJSON = newMsg.ConfidenceJSON
	}
	if toUpdate.AgentUserID != newMsg.AgentUserID && newMsg.AgentUserID.Valid {
		hasUpdate = true
		toUpdate.AgentUserID = newMsg.AgentUserID
	}
	if toUpdate.UsedCoachSuggestionID != newMsg.UsedCoachSuggestionID && newMsg.UsedCoachSuggestionID.Valid {
		hasUpdate = true
		toUpdate.UsedCoachSuggestionID = newMsg.UsedCoachSuggestionID
	}
	if toUpdate.UsedSuggestionID != newMsg.UsedSuggestionID && newMsg.UsedSuggestionID.Valid {
		hasUpdate = true
		toUpdate.UsedSuggestionID = newMsg.UsedSuggestionID
	}
	if toUpdate.StartTimeMs != newMsg.StartTimeMs {
		hasUpdate = true
		toUpdate.StartTimeMs = newMsg.StartTimeMs
	}
	if toUpdate.EndTimeMs != newMsg.EndTimeMs {
		hasUpdate = true
		toUpdate.EndTimeMs = newMsg.EndTimeMs
	}
	if toUpdate.PlatformTimestamp != newMsg.PlatformTimestamp {
		// We allow updating PlatformTimestamp because in some cases for
		// voice, there may be hallucinations that transcribed a wrong
		// first word from silence, then erased with actual words. The
		// PlatformTimestamp will be updated to the start time of the
		// actual first word instead of the hallucination one.
		//
		// NOTE: even though the sequence ID may be derived from
		// PlatformTimestamp, we won't update it here because it's
		// treated as an ID.
		hasUpdate = true
		toUpdate.PlatformTimestamp = newMsg.PlatformTimestamp
	}
	if toUpdate.Metadata.Message.GetTranscriptionInfo().GetTranscribedLanguage() != newMsg.Metadata.Message.GetTranscriptionInfo().GetTranscribedLanguage() {
		if toUpdate.Metadata.Message.TranscriptionInfo == nil {
			toUpdate.Metadata.Message.TranscriptionInfo = &dbconvpb.MessageMetadata_TranscriptionInfo{}
		}
		toUpdate.Metadata.Message.TranscriptionInfo.TranscribedLanguage = newMsg.Metadata.Message.GetTranscriptionInfo().GetTranscribedLanguage()
		toUpdate.Metadata.Message.TranscriptionInfo.NormalizedLanguageCode = newMsg.Metadata.Message.GetTranscriptionInfo().GetNormalizedLanguageCode()
		hasUpdate = true
	}
	if toUpdate.IsFinalized != newMsg.IsFinalized {
		if toUpdate.IsFinalized.Valid && toUpdate.IsFinalized.Bool {
			return nil, dbResultError, status.Errorf(codes.FailedPrecondition, "cannot update a complete message to incomplete")
		}
		toUpdate.IsFinalized = newMsg.IsFinalized
		hasUpdate = true
		finalizedChanged = true
	}
	if toUpdate.Metadata.Message.GetTranslationInfo() != newMsg.Metadata.Message.GetTranslationInfo() {
		if toUpdate.Metadata.Message != nil {
			toUpdate.Metadata.Message.TranslationInfo = newMsg.Metadata.Message.GetTranslationInfo()
			hasUpdate = true
		}
	}
	// Participants of a re-upserted email may now be redacted while the stored
	// copy is not. Only the participants are updated; the stored body is kept.
	existingEmail, newEmail := toUpdate.Metadata.Message.GetEmail(), newMsg.Metadata.Message.GetEmail()
	if updateEmailParticipants && existingEmail != nil && newEmail != nil && !emailParticipantsEqual(existingEmail, newEmail) {
		toUpdate.Metadata.Message = proto.Clone(toUpdate.Metadata.Message).(*dbconvpb.MessageMetadata)
		toUpdate.Metadata.Message.Email.FromParticipant = newEmail.FromParticipant
		toUpdate.Metadata.Message.Email.ToParticipants = newEmail.ToParticipants
		toUpdate.Metadata.Message.Email.CcParticipants = newEmail.CcParticipants
		toUpdate.Metadata.Message.Email.BccParticipants = newEmail.BccParticipants
		hasUpdate = true
	}
	var updateResult dbUpsertResult
	//nolint // Changing this to switch-case doesn't make sense
	if !hasUpdate {
		updateResult = dbResultNoop
	} else {
		toUpdate.UpdatedAt = newMsg.UpdatedAt
		if finalizedChanged {
			updateResult = dbResultUpdateIncompleteToComplete
		} else if isFinalized(&toUpdate) {
			updateResult = dbResultUpdateComplete
		} else {
			updateResult = dbResultUpdateIncomplete
		}
	}
	if toUpdate.SpeakerInfo != newMsg.SpeakerInfo {
		toUpdate.SpeakerInfo = newMsg.SpeakerInfo
	}
	return &toUpdate, updateResult, nil
}

func emailParticipantsEqual(a, b *dbconvpb.EmailMessage) bool {
	participantsEqual := func(x, y []*dbconvpb.ConversationParticipant) bool {
		return slices.EqualFunc(x, y, func(p, q *dbconvpb.ConversationParticipant) bool {
			return proto.Equal(p, q)
		})
	}
	return proto.Equal(a.GetFromParticipant(), b.GetFromParticipant()) &&
		participantsEqual(a.GetToParticipants(), b.GetToParticipants()) &&
		participantsEqual(a.GetCcParticipants(), b.GetCcParticipants()) &&
		participantsEqual(a.GetBccParticipants(), b.GetBccParticipants())
}

func (s *ServiceImpl) afterUpsertMessagesToDB(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	cachedCompleteMsgs []*sharedconversation.CacheMessage,
	cachedIncompleteMsgs map[redisconvpb.ConversationMessage_SpeakerRole]*sharedconversation.CacheMessage,
	upserted map[dbUpsertResult][]*dbmodel.Messages,
	pastEvents []*redisconvpb.ConversationEvent,
	messageSyncStatus conversationpb.BatchUpsertConversationMessagesRequest_MessageSyncStatus,
) error {
	// Publish messages before they were redacted.
	upsertedAPIConvMsgs, err := convertToAPIConversationMessages(s.converter, upserted)
	if err != nil {
		return err
	}
	features := config.GetFeaturesFailOpen(ctx, s.configServiceClient, conversationName, config.WithOptionalUsecaseID(conversation.UsecaseId)).GetConversation()
	if features.GetRedactAllDigitsOnPublishMessage() {
		// To mitigate PII redaction false negative for certain sensitive customers, we
		// over-redact all digits in the message before publishing to walter-client via
		// client subscription.
		for _, msg := range upsertedAPIConvMsgs {
			matches := digitsRegex.FindAllStringIndex(msg.Text, -1)
			// Replace the matched digits with *. Note that the text
			// length must be kept unchanged, so that walter-client
			// can apply entities correctly.
			//
			// NOTE: this is not correct if there are multi-byte
			// characters. We don't have that case in the regex now.
			text := []byte(msg.Text)
			for _, match := range matches {
				for i := match[0]; i < match[1]; i++ {
					text[i] = '*'
				}
			}
			msg.Text = string(text)
		}
	}
	// On by default.
	if *enableConvMsgPublish {
		if err := s.publishBatchConversation(ctx, conversationName, conversation.Source, upsertedAPIConvMsgs); err != nil {
			return err
		}
	}

	// If the customer is correcting their correlation ids, potentially refresh
	// the corrected correlation id key in redis.
	if features.GetCorrelationIdCorrectionMetadataKey() != "" {
		s.refreshCorrelationIDKeyTTL(ctx, conversation)
	}

	s.invokeWebhookForUpsertedMessages(ctx, conversationName, conversation, upserted)

	skipPIIRedaction := features.GetSkipRealTimePiiRedaction() ||
		conversation.GetFeatureOverride().GetSkipPiiRedation() ||
		features.GetSkipPerMessagePiiRedaction()
	redactWithOrchEntities := features.GetPopulateAnnotationsForPiiRedaction() &&
		!conversation.GetFeatureOverride().GetSkipAnnotations() &&
		!skipPIIRedaction
	byMessageLanguage := piiredaction.ByMessageLanguageEnabled(ctx, s.featureFlagClient, s.logger, conversationName, conversation.GetUsecaseId())

	if !skipPIIRedaction && !redactWithOrchEntities {

		callRedactorFunc := func(ctx context.Context) error {
			toRedact := make([]*dbmodel.Messages, 0, len(upserted[dbResultInserted])+len(upserted[dbResultUpdateIncompleteToComplete]))
			for _, msg := range upserted[dbResultInserted] {
				if isFinalized(msg) {
					toRedact = append(toRedact, msg)
				}
			}
			toRedact = append(toRedact, upserted[dbResultUpdateIncompleteToComplete]...)

			redactedMessages, err := s.redactor.RedactMessagesSync(ctx, conversationName, conversation.UsecaseId, conversation.LanguageCode, cachedCompleteMsgs, toRedact, redactionpb.RedactionScenario_REALTIME_STREAMING,
				piiredaction.WithByMessageLanguage(byMessageLanguage))
			msgIDs := fn.Map(toRedact, func(m *dbmodel.Messages) string {
				return m.ConversationMessageID.String
			})
			if err != nil {
				s.logger.Errorf(ctx, "pii redaction: conversation: %v, msgs: %v: %v", conversationName, msgIDs, err)
				return err
			}

			if err := s.conversationCache.AddRedactedTextForMessages(ctx, conversationName, redactedMessages); err != nil {
				s.logger.Errorf(ctx, "pii redaction cache: conversation: %v, msgs: %v: %v", conversationName, msgIDs, err)
			}

			if features.GetSendRedactedTextCsub() {
				return s.sendMessagesToClientSubscription(ctx, conversationName, conversation.Source, msgIDs...)
			}

			return nil
		}

		extrawork.RunWithTimeout(ctx, *piiRedactionTimeout,
			callRedactorFunc,
			extrawork.WithCountMetric(piiRedactionRequestCounter),
			extrawork.WithErrorMetric(piiRedactionErrorCounter),
			extrawork.WithDurationMetric(piiRedactionDurationMetric),
			extrawork.RunSynchronously(SyncExtraWork),
		)
	}

	completeMsgs, incompleteMsgs, err := s.updateConversationCache(ctx, conversationName, cachedCompleteMsgs, cachedIncompleteMsgs, upserted)
	if err != nil {
		return err
	}

	orchErr := s.callOrchestrator(
		ctx,
		conversationName,
		conversation,
		completeMsgs,
		incompleteMsgs,
		upserted,
		pastEvents,
		messageSyncStatus,
		redactWithOrchEntities,
		byMessageLanguage,
	)
	if orchErr != nil {
		s.logger.Errorf(ctx, "failed to call orchestrator: %v", err)
		// delay returning error to allow starting the workflow for closed conversation
	}

	if conversation.GetEndTime() != nil && conversation.GetSource() != conversationpb.Conversation_INGESTION_PIPELINE {
		// For realtime closed conversations, we need to trigger the close conversation extra workflow here because
		// CloseConversation will be no-op or may not be called at all. Other source types except ingestion pipeline
		// should behave similarly.
		//
		// For ingestion pipeline, we don't need to run the close conversation extra workflow because
		// the ingestion pipeline is expected to have follow up stages to execute the same tasks for a
		// batch of conversations.
		if *runCloseExtraWorkForUpsertMsg || conversation.GetChannel() == conversationpb.Conversation_EMAIL {
			err = s.startCloseConversationExtraWorkflow(
				ctx,
				conversationName,
				conversation.GetUsecaseId(),
				conversation.GetLanguageCode(),
				conversation.GetFeatureOverride(),
				conversation.GetSource(),
			)
			if err != nil {
				s.logger.Errorf(ctx, "failed to start index conversation workflow: %v", err)
				return err
			}
		} else {
			s.logger.Warnf(ctx, "Upserting messages to a closed %v channel conversation %v", conversation.GetChannel(), conversationName)
		}
	}
	return orchErr
}

func (s *ServiceImpl) updateConversationCache(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	cachedCompleteMsgs []*sharedconversation.CacheMessage,
	cachedIncompleteMsgs map[redisconvpb.ConversationMessage_SpeakerRole]*sharedconversation.CacheMessage,
	upsertedMsgs map[dbUpsertResult][]*dbmodel.Messages,
) (completeMsgs []*sharedconversation.CacheMessage, incompleteMsgs []*sharedconversation.CacheMessage, err error) {
	completeMsgs, err = s.updateCachedCompleteMsgs(ctx, conversationName, cachedCompleteMsgs, upsertedMsgs)
	if err != nil {
		return nil, nil, err
	}
	incompleteMsgs, err = s.updateCachedIncompleteMsgs(ctx, conversationName, cachedIncompleteMsgs, upsertedMsgs)
	if err != nil {
		return nil, nil, err
	}
	// Handle incomplete msgs
	return completeMsgs, incompleteMsgs, nil
}

func (s *ServiceImpl) updateCachedCompleteMsgs(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	cachedCompleteMsgs []*sharedconversation.CacheMessage,
	upsertedMsgs map[dbUpsertResult][]*dbmodel.Messages,
) ([]*sharedconversation.CacheMessage, error) {
	if len(upsertedMsgs[dbResultUpdateComplete]) > 0 {
		// Some complete messages got updated. The cache does not
		// support updating complete messages, so we need to refresh the
		// cache from DB.
		s.logger.Infof(ctx, "batch upsert msgs: refreshing cache from DB for conversation %v - %v complete messages updated", conversationName, len(upsertedMsgs[dbResultUpdateComplete]))
		_, completeMsgs, _, err := s.conversationCache.ReloadConversationFromDB(ctx, conversationName)
		return completeMsgs, err
	}
	insertedCompleteMsgs := slices.Concat(
		upsertedMsgs[dbResultInserted],
		upsertedMsgs[dbResultUpdateIncompleteToComplete],
	)
	if len(insertedCompleteMsgs) == 0 {
		return cachedCompleteMsgs, nil
	}

	// Check if we can append the messages to the cache
	slices.SortFunc(insertedCompleteMsgs, func(a, b *dbmodel.Messages) int {
		return getMessageSortingTime(a).Compare(getMessageSortingTime(b))
	})
	earliestNewlyInsertedMsgTime := getMessageSortingTime(insertedCompleteMsgs[0])
	for _, m := range cachedCompleteMsgs {
		if earliestNewlyInsertedMsgTime.Before(m.SortingTime()) {
			s.logger.Infof(
				ctx, "batch upsert msgs: refreshing cache from DB for conversation %v - message %v inserted before the latest cached message %v. Cache size: %v, inserted msgs size: %v",
				conversationName,
				insertedCompleteMsgs[0].ConversationMessageID.String,
				m.MessageId,
				len(cachedCompleteMsgs),
				len(insertedCompleteMsgs),
			)
			_, completeMsgs, _, err := s.conversationCache.ReloadConversationFromDB(ctx, conversationName)
			return completeMsgs, err
		}
	}
	s.logger.Debugf(ctx, "batch upsert msgs: appending %v messages to cache for conversation %v", len(insertedCompleteMsgs), conversationName)
	appendedMsgs, err := s.appendMsgsToCache(ctx, conversationName, insertedCompleteMsgs)
	if err != nil {
		return nil, err
	}

	return slices.Concat(cachedCompleteMsgs, appendedMsgs), nil
}

func (s *ServiceImpl) updateCachedIncompleteMsgs(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	cachedIncompleteMsgs map[redisconvpb.ConversationMessage_SpeakerRole]*sharedconversation.CacheMessage,
	upsertedMsgs map[dbUpsertResult][]*dbmodel.Messages,
) (incompleteMsgs []*sharedconversation.CacheMessage, err error) {
	excludeMsgIDs := set.New[string]()
	for _, m := range upsertedMsgs[dbResultUpdateIncompleteToComplete] {
		excludeMsgIDs.Add(m.ConversationMessageID.String)
	}
	for _, m := range upsertedMsgs[dbResultInserted] {
		if isFinalized(m) {
			// When the skip_db_for_incomplete_messages feature flag
			// is enabled, incomplete->complete updates will be
			// inserted and fall into the dbResultInserted category
			// instead of dbResultUpdateIncompleteToComplete
			excludeMsgIDs.Add(m.ConversationMessageID.String)
		}
	}
	for _, m := range upsertedMsgs[dbResultUpdateIncomplete] {
		cacheMsg, err := sharedconversation.ConvertToCacheMessage(m)
		if err != nil {
			return nil, err
		}
		incompleteMsgs = append(incompleteMsgs, cacheMsg)
		excludeMsgIDs.Add(cacheMsg.MessageId)
	}
	for _, m := range cachedIncompleteMsgs {
		if !excludeMsgIDs.Has(m.MessageId) {
			incompleteMsgs = append(incompleteMsgs, m)
		}
	}
	slices.SortFunc(incompleteMsgs, func(a, b *sharedconversation.CacheMessage) int {
		return a.SortingTime().Compare(b.SortingTime())
	})
	err = s.conversationCache.PutIncompleteMessages(ctx, conversationName, incompleteMsgs...)
	return incompleteMsgs, err
}

func (s *ServiceImpl) appendMsgsToCache(ctx context.Context, conversationName *conversationpb.ConversationName, msgs []*dbmodel.Messages) (appendedCacheMsgs []*sharedconversation.CacheMessage, err error) {
	cacheMsgs := make([]*sharedconversation.CacheMessage, len(msgs))
	for i, m := range msgs {
		cacheMsgs[i], err = sharedconversation.ConvertToCacheMessage(m)
		if err != nil {
			return nil, err
		}
	}
	return cacheMsgs, s.conversationCache.AddMessages(ctx, conversationName, cacheMsgs...)
}

func (s *ServiceImpl) callOrchestrator(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	completeMsgs []*sharedconversation.CacheMessage,
	incompleteMsgs []*sharedconversation.CacheMessage,
	upsertedMsgs map[dbUpsertResult][]*dbmodel.Messages,
	pastEvents []*redisconvpb.ConversationEvent,
	messageSyncStatus conversationpb.BatchUpsertConversationMessagesRequest_MessageSyncStatus,
	redactMessages bool,
	byMessageLanguage bool,
) error {
	if conversation.GetFeatureOverride().GetSkipAnnotations() || s.getFeatureFlags(ctx, conversationName, config.WithOptionalUsecaseID(conversation.GetUsecaseId())).GetSkipOrchestrator() {
		return nil
	}
	if messageSyncStatus == conversationpb.BatchUpsertConversationMessagesRequest_MESSAGE_SYNC_STATUS_UNSPECIFIED {
		return s.callOrchestratorWithOldTransferBehavior(ctx, conversationName, conversation, completeMsgs, incompleteMsgs, upsertedMsgs, pastEvents, redactMessages, byMessageLanguage)
	}
	// New transfer logic
	incomingMsgIDs := set.New[string]()
	incompleteMsgIDs := set.New[string]()
	for k, msgs := range upsertedMsgs {
		// Orchestrator doesn't care about completed messages being
		// updated. In future if it needs to handle some updates (e.g.
		// updating text or agent), we should create a new category for
		// the update.
		if k == dbResultNoop || k == dbResultUpdateComplete {
			continue
		}
		for _, m := range msgs {
			incomingMsgIDs.Add(m.ConversationMessageID.String)
		}
	}
	// We allow 0 incoming messages when sync status is SYNCHRONIZED to trigger on transfer suggestions.
	if incomingMsgIDs.Len() == 0 && messageSyncStatus != conversationpb.BatchUpsertConversationMessagesRequest_SYNCHRONIZED {
		s.logger.Debugf(ctx, "no incoming messages and message sync status is not synchronized. skip calling orchestrator")
		return nil
	}
	for _, m := range incompleteMsgs {
		incompleteMsgIDs.Add(m.MessageId)
	}
	allMsgs := slices.Concat(completeMsgs, incompleteMsgs)
	incomingOrchMessages := []*orchestrator_pb.ConversationMessage{}
	pastOrchMessages := []*orchestrator_pb.ConversationMessage{}
	features := config.GetFeaturesFailOpen(ctx, s.configServiceClient, conversationName, config.WithOptionalUsecaseID(conversation.GetUsecaseId())).GetConversation()
	translatedToOriginalMessageIDMap := map[string]string{}
	for _, msg := range allMsgs {
		if incompleteMsgIDs.Has(msg.MessageId) && features.GetSkipIncompleteMessageProcess() {
			continue
		}
		if msg.GetMetadata().GetTranslationInfo().GetTranslated() {
			translatedToOriginalMessageIDMap[msg.MessageId] = msg.GetMetadata().GetTranslationInfo().GetOriginalConversationMessageId()
		}
		orchMsg, err := sharedconversation.ConvertToOrchestratorMessage(conversationName, msg, !incompleteMsgIDs.Has(msg.MessageId), false /* useRedactedText */)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to construct request for orchestrator: %v", err)
		}
		setOrchestratorMessageLanguage(orchMsg, msg, conversation, byMessageLanguage)
		if incomingMsgIDs.Has(msg.MessageId) {
			incomingOrchMessages = append(incomingOrchMessages, orchMsg)
		} else {
			pastOrchMessages = append(pastOrchMessages, orchMsg)
		}
	}

	if conversation.GetTranslationInfo().GetTranslationEnabled() {
		var err error
		incomingOrchMessages, pastOrchMessages, err = sharedconversation.PostProcessOrchestratorTranslationMessages(
			incomingOrchMessages,
			pastOrchMessages,
			translatedToOriginalMessageIDMap,
			conversation.GetLanguageCode(),
			conversation.GetTranslationInfo(),
		)
		if err != nil {
			return fmt.Errorf("fail to post process orchestrator translation messages %w", err)
		}
		if len(incomingOrchMessages) == 0 && messageSyncStatus != conversationpb.BatchUpsertConversationMessagesRequest_SYNCHRONIZED {
			s.logger.Debugf(ctx, "no incoming messages and message sync status is not synchronized. skip calling orchestrator")
			return nil
		}
	}

	orchConversation := sharedconversation.ConvertCacheToOrchestratorConversation(
		conversationName,
		conversation,
		deploymentclient.CheckModelDeploymentInfo(
			ctx,
			s.getConfigs(ctx, conversationName, conversation.GetUsecaseId()).GetOrchestrator(),
			s.deploymentClient,
			s.logger,
			&customerpb.UsecaseName{
				CustomerID: conversationName.CustomerID,
				ProfileID:  conversationName.ProfileID,
				UsecaseID:  conversation.GetUsecaseId(),
			},
			conversation.GetLanguageCode()),
	)
	internalPastEvents, err := sharedconversation.ConvertCacheEventsToInternalEvents(conversationName, pastEvents)
	if err != nil {
		return fmt.Errorf("fail to convert cache event to internal event %v", err)
	}
	s.logger.Debugf(ctx, "batch upsert: calling orchestrator\n message sync state: %v\n redaction: %v\n incoming: %v past: %v",
		messageSyncStatus,
		redactMessages,
		incomingOrchMessages,
		pastOrchMessages,
	)

	callOrchFunc := func(ctx context.Context) (err error) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				s.logger.Errorf(ctx, "callOrchFunc: recovered from panic: %v", panicErr)
				testvalue.Set(TestValueCallOrchResult, panicErr)
			} else {
				testvalue.Set(TestValueCallOrchResult, err)
			}
		}()

		var orchReq *orchestrator_pb.ProcessMessageRequest

		// Handle redaction after orchestrator call completes or fails.
		defer func() {
			if redactMessages {
				orchReqForEntityBackfill := orchReq
				if err == nil {
					// If orchestrator returns no error, set orchReqForEntityBackfill to nil to indicate fallback is not needed.
					orchReqForEntityBackfill = nil
				}

				// Even if orchestrator returns error, it can be a
				// partial error and entity may still get
				// generated. So proceed with PII redaction
				redactErr := s.redactMessagesWithOrchEntities(ctx, conversationName, conversation.Source, conversation.UsecaseId, upsertedMsgs, orchReqForEntityBackfill)
				if redactErr != nil {
					s.logger.Errorf(ctx, "fail to redact message: %v", redactErr)
					err = redactErr
				}
			}
		}()

		switch messageSyncStatus {
		case conversationpb.BatchUpsertConversationMessagesRequest_SYNCHRONIZING:
			req := orchestrator_pb.ProcessConversationTransferRequest{
				Messages:     incomingOrchMessages,
				Conversation: orchConversation,
			}
			_, err = s.orchestrator.Client(ctx).ProcessConversationTransfer(ctx, &req)
			if err != nil {
				s.logger.Errorf(ctx, "fail to call orchestrator to process transferred messages %v", err)
				return err
			}
		case conversationpb.BatchUpsertConversationMessagesRequest_SYNCHRONIZED:
			// Usecase: For chat transfer (bot to agent or agent to agent), the frontend will update the conversation agent,
			// following a batch upsert messages call with SYNCHRONIZED stats. The batch upsert messages call may contain zero messages,
			// or messages that are already upserted and deduped. In either case, we want to trigger a transfer suggestion by sending a
			// ProcessMessage call to Orchestrator with empty incoming messages.
			if len(incomingOrchMessages) == 0 {
				orchReq = &orchestrator_pb.ProcessMessageRequest{
					Conversation: orchConversation,
					PastMessages: pastOrchMessages,
					Events:       internalPastEvents,
					ForTransfer:  true,
				}
				_, err = s.orchestrator.Client(ctx).ProcessMessage(ctx, orchReq)
				if err != nil {
					s.logger.Errorf(ctx, "fail to call orchestrator processMessage %v", err)
					return err
				}
				break
			}
			// Try to process n-1 messages as transfer message first, and process the last incoming message as normal message.
			var finishSignal chan struct{}
			var transferErr error
			timeout := false
			transferredMsgs := incomingOrchMessages[:len(incomingOrchMessages)-1]
			if len(transferredMsgs) > 0 {
				finishSignal = make(chan struct{})
				go utils.AvoidPanic(ctx, func() {
					defer close(finishSignal)
					req := orchestrator_pb.ProcessConversationTransferRequest{
						Messages:     transferredMsgs,
						Conversation: orchConversation,
					}
					_, transferErr = s.orchestrator.Client(ctx).ProcessConversationTransfer(ctx, &req)
					if transferErr != nil {
						s.logger.Errorf(ctx, "fail to call orchestrator to process transferred messages %v", transferErr)
					}
				})()
				if *forceSynchronousTransfer {
					// Block until ProcessConversationTransfer finishes.
					<-finishSignal
				} else {
					// wait for transferMsgs to be handled up to N seconds.
					select {
					case <-finishSignal:
						// Do nothing.
						s.logger.Infof(ctx, "processTransferMessageWait finished on conversation %v", conversationName)
					case <-time.After(*processTransferMessageWait):
						timeout = true
						s.logger.Warnf(ctx, "processTransferMessageWait timeout on conversation %v", conversationName)
					}
				}
			}

			markMessagesForOnFlyRedaction(transferredMsgs, transferErr, timeout)

			// Now just processed transfer messages are already past.
			pastOrchMessages = append(pastOrchMessages, transferredMsgs...)

			orchReq = &orchestrator_pb.ProcessMessageRequest{
				Conversation:     orchConversation,
				PastMessages:     pastOrchMessages,
				IncomingMessages: incomingOrchMessages[len(incomingOrchMessages)-1:],
				Events:           internalPastEvents,
				ForTransfer:      true,
			}
			_, err = s.orchestrator.Client(ctx).ProcessMessage(ctx, orchReq)
			if err != nil {
				s.logger.Errorf(ctx, "fail to call orchestrator processMessage %v", err)
			}

			// wait until ProcessConversationTransfer finishes.
			if timeout {
				<-finishSignal
			}

		case conversationpb.BatchUpsertConversationMessagesRequest_IN_SYNC:
			orchReq = &orchestrator_pb.ProcessMessageRequest{
				Conversation:     orchConversation,
				PastMessages:     pastOrchMessages,
				IncomingMessages: incomingOrchMessages,
				Events:           internalPastEvents,
			}
			_, err = s.orchestrator.Client(ctx).ProcessMessage(ctx, orchReq)
			if err != nil {
				s.logger.Errorf(ctx, "fail to call orchestrator processMessage %v", err)
				return err
			}
		default:
			s.logger.Errorf(ctx, "unknown message sync status: %v", messageSyncStatus)
			return fmt.Errorf("unknown message sync status: %v", messageSyncStatus)
		}

		return nil
	}

	extrawork.RunWithTimeout(ctx, *orchestratorTimeout,
		callOrchFunc,
		extrawork.RunSynchronously(conversation.GetFeatureOverride().GetSynchronousBackend()),
	)
	return nil
}

// setOrchestratorMessageLanguage sets the language the orchestrator
// PredictEntity node extracts orchMsg's entities in: the language msg was
// spoken in (see sharedconversation.ResolveMessageLanguageCode). It only does
// so when byMessageLanguage is set and translation is off; with translation on,
// PostProcessOrchestratorTranslationMessages sets the message languages.
func setOrchestratorMessageLanguage(
	orchMsg *orchestrator_pb.ConversationMessage,
	msg *sharedconversation.CacheMessage,
	conversation *redisconvpb.Conversation,
	byMessageLanguage bool,
) {
	if !byMessageLanguage || conversation.GetTranslationInfo().GetTranslationEnabled() {
		return
	}
	orchMsg.LanguageCode = sharedconversation.ResolveMessageLanguageCode(msg.GetMetadata().GetLanguageCode(), conversation.GetLanguageCode())
}

func (s *ServiceImpl) callOrchestratorWithOldTransferBehavior(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	completeMsgs []*sharedconversation.CacheMessage,
	incompleteMsgs []*sharedconversation.CacheMessage,
	upsertedMsgs map[dbUpsertResult][]*dbmodel.Messages,
	pastEvents []*redisconvpb.ConversationEvent,
	redactMessages bool,
	byMessageLanguage bool,
) error {
	incomingMsgIDs := set.New[string]()
	transferredMsgIDs := set.New[string]()
	for k, msgs := range upsertedMsgs {
		// Orchestrator doesn't care about completed messages being
		// updated. In future if it needs to handle some updates (e.g.
		// updating text or agent), we should create a new category for
		// the update.
		if k == dbResultNoop || k == dbResultUpdateComplete {
			continue
		}
		for _, m := range msgs {
			msgID := m.ConversationMessageID.String
			if k == dbResultInserted && m.Metadata.Message != nil && m.Metadata.Message.IsTransferred {
				transferredMsgIDs.Add(msgID)
			} else {
				incomingMsgIDs.Add(msgID)
			}
		}
	}
	if incomingMsgIDs.Len() == 0 && transferredMsgIDs.Len() == 0 {
		s.logger.Debugf(ctx, "no incoming or transferred messages. skip calling orchestrator")
		return nil
	}

	incompleteMsgIDs := map[string]struct{}{}
	for _, msg := range incompleteMsgs {
		incompleteMsgIDs[msg.MessageId] = struct{}{}
	}

	orchConversation := sharedconversation.ConvertCacheToOrchestratorConversation(
		conversationName,
		conversation,
		deploymentclient.CheckModelDeploymentInfo(
			ctx,
			s.getConfigs(ctx, conversationName, conversation.GetUsecaseId()).GetOrchestrator(),
			s.deploymentClient,
			s.logger,
			&customerpb.UsecaseName{
				CustomerID: conversationName.CustomerID,
				ProfileID:  conversationName.ProfileID,
				UsecaseID:  conversation.GetUsecaseId(),
			},
			conversation.GetLanguageCode()),
	)

	incomingMsgs := []*orchestrator_pb.ConversationMessage{}
	transferredMsgs := []*orchestrator_pb.ConversationMessage{}
	allMsgs := make([]*sharedconversation.CacheMessage, len(completeMsgs), len(completeMsgs)+len(incompleteMsgs))
	copy(allMsgs, completeMsgs)
	allMsgs = append(allMsgs, incompleteMsgs...)
	pastOrchMessages := make([]*orchestrator_pb.ConversationMessage, 0, len(allMsgs)-1)
	latestMessageID := int32(0)
	latestMessageCreateTime := time.UnixMilli(0)
	features := config.GetFeaturesFailOpen(ctx, s.configServiceClient, conversationName, config.WithOptionalUsecaseID(conversation.GetUsecaseId())).GetConversation()
	translatedToOriginalMessageIDMap := map[string]string{}
	for _, msg := range allMsgs {
		_, isIncomplete := incompleteMsgIDs[msg.MessageId]
		if isIncomplete && features.GetSkipIncompleteMessageProcess() {
			continue
		}
		if msg.GetMetadata().GetTranslationInfo().GetTranslated() {
			translatedToOriginalMessageIDMap[msg.MessageId] = msg.GetMetadata().GetTranslationInfo().GetOriginalConversationMessageId()
		}
		orchMsg, err := sharedconversation.ConvertToOrchestratorMessage(conversationName, msg, !isIncomplete, false /* useRedactedText */)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to construct request for orchestrator: %v", err)
		}
		setOrchestratorMessageLanguage(orchMsg, msg, conversation, byMessageLanguage)
		switch {
		case transferredMsgIDs.Has(msg.MessageId):
			transferredMsgs = append(transferredMsgs, orchMsg)
		case incomingMsgIDs.Has(msg.MessageId):
			incomingMsgs = append(incomingMsgs, orchMsg)
		default:
			pastOrchMessages = append(pastOrchMessages, orchMsg)
		}
		// Edge case: there can be multiple transferred messages with the same platform time.
		// Use !Before instead of After as the condition so that the last message ID is picked
		// in this case.
		if !msg.SortingTime().Before(latestMessageCreateTime) {
			latestMessageID = msg.LegacyMessageId
			latestMessageCreateTime = msg.SortingTime()
		}
	}

	// If all msgs are transferredMsgs and the last msg in transferredMsgs is the latest, will let Orchestrator to treat the latest msg as incoming.
	if len(incomingMsgs) == 0 && len(transferredMsgs) > 0 && (transferredMsgs[len(transferredMsgs)-1].MessageId == latestMessageID || features.GetAlwaysProcessLastTransferMessage()) {
		incomingMsgs = append(incomingMsgs, transferredMsgs[len(transferredMsgs)-1])
		transferredMsgs = transferredMsgs[:len(transferredMsgs)-1]
	}

	// Process transfer is not supported for translation.
	if conversation.GetTranslationInfo().GetTranslationEnabled() {
		var err error
		incomingMsgs, pastOrchMessages, err = sharedconversation.PostProcessOrchestratorTranslationMessages(
			incomingMsgs,
			pastOrchMessages,
			translatedToOriginalMessageIDMap,
			conversation.GetLanguageCode(),
			conversation.GetTranslationInfo(),
		)
		if err != nil {
			return fmt.Errorf("fail to post process orchestrator translation messages %w", err)
		}
	}

	internalPastEvents, err := sharedconversation.ConvertCacheEventsToInternalEvents(conversationName, pastEvents)
	if err != nil {
		return fmt.Errorf("fail to convert cache event to internal event %v", err)
	}

	callOrchFunc := func(ctx context.Context) error {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				s.logger.Errorf(ctx, "callOrchFunc: recovered from panic: %v", panicErr)
				testvalue.Set(TestValueCallOrchResult, panicErr)
			} else {
				testvalue.Set(TestValueCallOrchResult, err)
			}
		}()
		// Try to process transfer messages first.
		// If there are N transferred messages, only the first N-1 will be sent to ProcessConversationTransfer API
		// The Nth message will be sent to ProcessMessage API.
		// This is needed because we want to sent suggestion and hints when agent takes over a transfer call without saying anything.
		var finishSignal chan struct{}
		var transferErr error
		timeout := false
		if len(transferredMsgs) > 0 {
			finishSignal = make(chan struct{})
			go utils.AvoidPanic(ctx, func() {
				defer close(finishSignal)
				req := orchestrator_pb.ProcessConversationTransferRequest{
					Messages:     transferredMsgs,
					Conversation: orchConversation,
				}
				_, transferErr = s.orchestrator.Client(ctx).ProcessConversationTransfer(ctx, &req)
				if transferErr != nil {
					s.logger.Errorf(ctx, "fail to call orchestrator to process transferred messages %v", transferErr)
				}
			})()
			if *forceSynchronousTransfer {
				// Block until ProcessConversationTransfer finishes.
				<-finishSignal
			} else {
				// wait for transferMsgs to be handled up to N seconds.
				select {
				case <-finishSignal:
					// Do nothing.
					s.logger.Infof(ctx, "processTransferMessageWait (oldTransfer) finished on conversation %v", conversationName)
				case <-time.After(*processTransferMessageWait):
					timeout = true
					s.logger.Warnf(ctx, "processTransferMessageWait (oldTransfer) timeout on conversation %v", conversationName)
				}
			}
		}

		// Now just processed transfer messages are already past.
		pastOrchMessages = append(pastOrchMessages, transferredMsgs...)
		var orchReq *orchestrator_pb.ProcessMessageRequest
		var orchProcessMessageErr error
		if len(incomingMsgs) > 0 {
			markMessagesForOnFlyRedaction(transferredMsgs, transferErr, timeout)

			orchReq = &orchestrator_pb.ProcessMessageRequest{
				Message:          incomingMsgs[len(incomingMsgs)-1],
				Conversation:     orchConversation,
				PastMessages:     pastOrchMessages,
				IncomingMessages: incomingMsgs,
				Events:           internalPastEvents,
				ForTransfer:      len(transferredMsgs) > 0,
			}
			_, orchProcessMessageErr = s.orchestrator.Client(ctx).ProcessMessage(ctx, orchReq)
			if orchProcessMessageErr != nil {
				s.logger.Errorf(ctx, "fail to call orchestrator processMessage %v", orchProcessMessageErr)
			}
		}

		// wait until ProcessConversationTransfer finishes.
		if timeout {
			<-finishSignal
		}

		// Handle redaction after orchestrator call completes or fails.
		if redactMessages {
			orchReqForEntityBackfill := orchReq
			if orchProcessMessageErr == nil {
				// If orchestrator returns no error, set orchReqForEntityBackfill to nil to indicate fallback is not needed.
				orchReqForEntityBackfill = nil
			}

			err := s.redactMessagesWithOrchEntities(ctx, conversationName, conversation.Source, conversation.UsecaseId, upsertedMsgs, orchReqForEntityBackfill)
			if err != nil {
				s.logger.Errorf(ctx, "fail to redact message: %v", err)
			}
		}

		return nil
	}

	extrawork.RunWithTimeout(ctx, *orchestratorTimeout,
		callOrchFunc,
		extrawork.RunSynchronously(conversation.GetFeatureOverride().GetSynchronousBackend()),
	)

	return nil
}

func (s *ServiceImpl) redactMessagesWithOrchEntities(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	conversationSource conversationpb.Conversation_Source,
	usecaseID string,
	upserted map[dbUpsertResult][]*dbmodel.Messages,
	orchRequest *orchestrator_pb.ProcessMessageRequest,
) error {
	toRedact := make([]*dbmodel.Messages, 0, len(upserted[dbResultInserted])+len(upserted[dbResultUpdateIncompleteToComplete]))
	for _, msg := range upserted[dbResultInserted] {
		if isFinalized(msg) {
			toRedact = append(toRedact, msg)
		}
	}
	toRedact = append(toRedact, upserted[dbResultUpdateIncompleteToComplete]...)

	if len(toRedact) == 0 {
		s.logger.Debugf(ctx, "pii redaction: no messages to redact for conversation %v", conversationName)
		return nil
	}

	// Attempt to fetch entity payloads from Redis cache (if enabled).
	// We treat **all** messages being present as a cache-hit; anything
	// missing is a miss that may trigger a backfill later.
	cachedEntitiesByMsgID := map[string][]*aicommonpb.EntityPayload{}
	allEntitiesCached := false

	if *piiredaction.EnableEntityCache {
		if *piiredaction.EnableEntityCacheLogging {
			s.logger.Debugf(ctx,
				"pii redaction cache: reading entity cache for messages: %v",
				fn.Map(toRedact, func(msg *dbmodel.Messages) string {
					return fmt.Sprintf("profiles/%v/conversations/%v/messages/%v", conversationName.ProfileID, conversationName.ConversationID, msg.ConversationMessageID.String)
				}))
		}

		existingCachedEntitiesByMsgID := s.getCachedEntities(ctx, conversationName, toRedact)
		allEntitiesCached = len(existingCachedEntitiesByMsgID) == len(toRedact)

		if allEntitiesCached {
			cachedEntitiesByMsgID = existingCachedEntitiesByMsgID
			piiredaction.MetricEntityCacheHit.Incr(ctx)
		} else {
			piiredaction.MetricEntityCacheMiss.Incr(ctx)
			if *piiredaction.EnableEntityCacheLogging {
				s.logger.Debugf(ctx,
					"pii redaction cache: cache miss for messages: %v",
					fn.Map(toRedact, func(msg *dbmodel.Messages) string {
						return fmt.Sprintf("profiles/%v/conversations/%v/messages/%v", conversationName.ProfileID, conversationName.ConversationID, msg.ConversationMessageID.String)
					}))
			}

			if len(existingCachedEntitiesByMsgID) > 0 {
				s.logger.Debugf(ctx, "pii redaction cache: partial cache miss for %v messages, but %v entities are cached", len(toRedact), len(existingCachedEntitiesByMsgID))
			}
		}
	}

	// If the previous orchestrator call failed (orchRequest != nil) AND we still don't have entities
	// for every message, make a reduced ProcessMessage backfill call to generate the missing annotations.
	if orchRequest != nil && !allEntitiesCached {
		s.logger.Debugf(ctx, "backfill entities for %v via orchestrator for redaction", conversationName)
		piiRedactionWithOrchEntityBackfillRequestCounter.Incr(ctx)

		if err := s.backfillEntitiesViaOrchestrator(ctx, conversationName, toRedact, orchRequest); err != nil {
			piiRedactionWithOrchEntityBackfillErrorCounter.Incr(ctx)
			return err
		}
	}

	// High-level counter for this redaction path.
	piiRedactionWithOrchEntitiesRequestCounter.Incr(ctx)

	redactedMsgs, err := s.redactor.RedactMessagesWithExistingEntities(
		ctx,
		conversationName,
		toRedact,
		true, /* useWriteReplica */
		redactionpb.RedactionScenario_REALTIME_STREAMING,
		cachedEntitiesByMsgID,
	)
	if err != nil {
		piiRedactionWithOrchEntitiesErrorCounter.Incr(ctx)
		return err
	}

	if err := s.conversationCache.AddRedactedTextForMessages(ctx, conversationName, redactedMsgs); err != nil {
		msgIDs := fn.Map(toRedact, func(m *dbmodel.Messages) string { return m.ConversationMessageID.String })
		s.logger.Errorf(ctx, "pii redaction cache: conversation: %v, msgs: %v: %v", conversationName, msgIDs, err)
	}

	features := config.GetFeaturesFailOpen(ctx, s.configServiceClient, conversationName, config.WithOptionalUsecaseID(usecaseID)).GetConversation()
	if features.GetSendRedactedTextCsub() {
		err = s.sendMessagesToClientSubscription(
			ctx,
			conversationName,
			conversationSource,
			fn.Map(toRedact, func(msg *dbmodel.Messages) string {
				return msg.ConversationMessageID.String
			})...)
	}
	return err
}

func (s *ServiceImpl) getCachedEntities(ctx context.Context, conversationName *conversationpb.ConversationName, messagesToRedact []*dbmodel.Messages) map[string][]*aicommonpb.EntityPayload {
	keys := fn.Map(messagesToRedact, func(msg *dbmodel.Messages) string {
		return piiredaction.GetEntityRedactionCacheKey(conversationName.CustomerID, conversationName.ProfileID, conversationName.ConversationID, msg.ConversationMessageID.String)
	})

	resp := s.redisClient.MGet(ctx, keys...)
	if resp.Err() != nil {
		s.logger.Warnf(ctx, "[Redaction Entity Cache] Failed to get entity redaction cache: %v", resp.Err())
		return map[string][]*aicommonpb.EntityPayload{}
	}

	results, err := resp.Result()
	if err != nil {
		s.logger.Warnf(ctx, "[Redaction Entity Cache] Failed to unmarshal entity redaction cache: %v", err)
		return map[string][]*aicommonpb.EntityPayload{}
	}

	cachedEntitiesByMsgID := map[string][]*aicommonpb.EntityPayload{}
	for i, result := range results {
		entities := []*aicommonpb.EntityPayload{}
		if result == nil {
			s.logger.Warnf(ctx, "[Redaction Entity Cache] Cache miss for message %v, aborting cache usage", messagesToRedact[i].ConversationMessageID.String)
			return map[string][]*aicommonpb.EntityPayload{}
		}

		if err := json.Unmarshal([]byte(result.(string)), &entities); err == nil {
			cachedEntitiesByMsgID[messagesToRedact[i].ConversationMessageID.String] = entities
		}
	}

	return cachedEntitiesByMsgID
}

func (s *ServiceImpl) backfillEntitiesViaOrchestrator(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	messagesToRedact []*dbmodel.Messages,
	orchRequest *orchestrator_pb.ProcessMessageRequest,
) error {
	// Must use write replica to read the freshly inserted moment annotations
	entitiesByMsgID, msgsWithDeferredEOCEntities, msgsWithAnyEntityAnnotation, fetchEntityErr :=
		s.redactor.ReadEntitiesForMessages(ctx, conversationName, messagesToRedact, true /* useWriteReplica */, redactionpb.RedactionScenario_REALTIME_STREAMING)

	if fetchEntityErr != nil {
		s.logger.Errorf(ctx, "failed to read entities for messages %v: %v", messagesToRedact, fetchEntityErr)
		return fetchEntityErr
	}

	orchConversation := orchRequest.Conversation
	orchConversation.FeatureConfigFlag = &featurespb.Orchestrator{
		EnableEntity: true,
	}
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(10) // Limit the number of concurrent requests to orchestrator.

	for i, msg := range orchRequest.IncomingMessages {
		// Skip the backfill ProcessMessage if the orchestrator already extracted
		// entities for this message. Three buckets all count as "already extracted",
		// and any re-fire would re-persist duplicate annotations:
		//   1. entitiesByMsgID — REDACTION entities applying under REALTIME_STREAMING.
		//   2. msgsWithDeferredEOCEntities — REDACTION entities deferred to EOC
		//      (e.g. realtime.redact=true with redaction_mode != STREAMING).
		//   3. msgsWithAnyEntityAnnotation — unfiltered catch-all covering any
		//      annotation in the DB regardless of SurfaceMode or scenario filter
		//      (NOTE_TAKING entities, postcall-only REDACTION entities, etc).
		//      This bucket subsumes (1) and (2) for the dedup decision but we
		//      keep the explicit checks above for defense-in-depth.
		// msg.GetName() is the full resource path; entitiesByMsgID and the sets
		// are keyed by the bare MessageID (the DB column), so parse first.
		if parsedMsgName, parseErr := conversationpb.ParseConversationMessageName(msg.GetName()); parseErr == nil {
			msgID := parsedMsgName.MessageID
			if _, ok := entitiesByMsgID[msgID]; ok {
				continue
			}
			if msgsWithDeferredEOCEntities.Has(msgID) {
				continue
			}
			if msgsWithAnyEntityAnnotation.Has(msgID) {
				continue
			}
		}

		orchRequestForEntityBackfill := &orchestrator_pb.ProcessMessageRequest{
			Conversation:     orchConversation,
			PastMessages:     append(orchRequest.PastMessages, orchRequest.IncomingMessages[:i]...),
			IncomingMessages: []*orchestrator_pb.ConversationMessage{msg},
		}

		errGroup.Go(func() error {
			_, err := s.orchestrator.Client(ctx).ProcessMessage(ctx, orchRequestForEntityBackfill)
			return err
		})
	}

	err := errGroup.Wait()
	if err != nil {
		s.logger.Errorf(ctx, "orchestrator processMessage rquests to backfill entity annotations failed again %v", err)
	}
	return err
}

func (s *ServiceImpl) sendMessagesToClientSubscription(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	conversationSource conversationpb.Conversation_Source,
	messageIDs ...string,
) error {
	if len(messageIDs) == 0 {
		return nil
	}
	// Need to use write replica to read the just updated message. Read
	// replica likely still have staled unredacted message.
	msgs, err := s.conversationDaoProvider.GetWriteReplica(ctx).BatchGetConversationMessages(conversationName, messageIDs...)
	if err != nil {
		return fmt.Errorf("failed to read messages from conversation %v / %v: %v", conversationName, messageIDs, err)
	}
	apiMsgs, err := fn.MapE(
		msgs,
		func(dbMsg *dbmodel.Messages) (*conversationpb.ConversationMessage, error) {
			return s.converter.ConvertToAPIConversationMessage(dbMsg, nil /* botdata */)
		})
	if err != nil {
		return err
	}
	return s.publishBatchConversation(ctx, conversationName, conversationSource, apiMsgs)
}

// markTransferredMsgs marks the msgsToInsert as transferred messages if the
// number of to-be-inserted messages is equal to or more than
// transferMessageThreshold
func markTransferredMsgs(
	msgsToInsert []*dbmodel.Messages,
	messageSyncStatus conversationpb.BatchUpsertConversationMessagesRequest_MessageSyncStatus,
) {
	if messageSyncStatus == conversationpb.BatchUpsertConversationMessagesRequest_MESSAGE_SYNC_STATUS_UNSPECIFIED {
		// if message sync status is unspecified, it means the traffic comes from hasrua adapter or frontend hasn't passed in the message sync status.
		if len(msgsToInsert) < *transferMessageThreshold {
			return
		}
		for _, m := range msgsToInsert {
			if m.Metadata.Message == nil {
				m.Metadata.Message = &dbconvpb.MessageMetadata{}
			}
			m.Metadata.Message.IsTransferred = true
		}
		return
	}

	// New logic will purely rely on message sync status
	if messageSyncStatus == conversationpb.BatchUpsertConversationMessagesRequest_SYNCHRONIZING ||
		messageSyncStatus == conversationpb.BatchUpsertConversationMessagesRequest_SYNCHRONIZED {
		for _, m := range msgsToInsert {
			if m.Metadata.Message == nil {
				m.Metadata.Message = &dbconvpb.MessageMetadata{}
			}
			m.Metadata.Message.IsTransferred = true
		}
	}
}

func getMessageSortingTime(msg *dbmodel.Messages) time.Time {
	return msg.PlatformTimestamp
}

func convertToAPIConversationMessages(
	converter *converter.Converter,
	upserted map[dbUpsertResult][]*dbmodel.Messages,
) ([]*conversationpb.ConversationMessage, error) {
	// Estimated capacity since length is at least len(upserted).
	apiMsgs := make([]*conversationpb.ConversationMessage, 0, len(upserted))
	for upsertResult, dbMsgs := range upserted {
		// Skip the result types that are not updated.
		if upsertResult == dbResultNoop || upsertResult == dbResultError {
			continue
		}
		for _, dbMsg := range dbMsgs {
			apiMsg, err := converter.ConvertToAPIConversationMessage(dbMsg, nil)
			if err != nil {
				return nil, err
			}
			apiMsgs = append(apiMsgs, apiMsg)
		}
	}
	return apiMsgs, nil
}

func validateOrPopulateSequenceIDs(conversation *redisconvpb.Conversation, msgs []*conversationpb.ConversationMessage) error {
	// This assumes the messages either all have sequence IDs or none have sequence IDs
	sequenceIDs := make(map[int32]string, len(msgs))
	for _, message := range msgs {
		var sequenceID int32
		switch {
		case message.SequencePosition != nil:
			sequenceID = *message.SequencePosition
		case message.CreateTime == nil:
			return status.Errorf(codes.InvalidArgument, "none of sequence_position or create_time is populated for message %s", message.Name)
		default:
			// We add one day in case conversation create time is later than
			// message create time. This may happen when we can get the
			// message time from the platform but using the observed
			// conversation time as the conversation time.
			sequenceID = int32(message.CreateTime.AsTime().Add(24 * time.Hour).Sub(conversation.StartTime.AsTime()).Milliseconds())
		}

		for {
			otherMsg, conflict := sequenceIDs[sequenceID]
			if !conflict {
				break
			}
			//nolint
			if message.SequencePosition != nil {
				// The caller provides duplicated sequence position. Reject with validation error
				return status.Errorf(codes.InvalidArgument, "sequence_position for message %s is the same as message %s: %v", message.Name, otherMsg, sequenceID)
			}
			// The sequence ID is calculated from message create
			// time. Increase the ID until there is no conflict
			// because we want to allow multiple messages with the
			// same create time in the same batch.
			sequenceID++
		}
		//nolint
		if message.SequencePosition == nil {
			message.SequencePosition = &sequenceID
		}
		sequenceIDs[sequenceID] = message.Name
	}
	return nil
}

// Enables redaction for messages in orchestrator for those whose entities were extracted.
// Errored messages are skipped as entities may not exist, allowing leaking of PII.
func markMessagesForOnFlyRedaction(msgs []*orchestrator_pb.ConversationMessage, transferErr error, timedout bool) {
	// Don't enable redaction if the transfer timed out. We have no way of guaranteeing that the entities are available for any messages.
	if timedout {
		return
	}

	failedMessageNames := set.New[string]()
	if transferErr != nil {
		details := grpc.GetErrorDetails(transferErr)
		for _, detail := range details {
			for msgName := range detail.GetMetadata() {
				failedMessageNames.Add(msgName)
			}
		}
	}

	for _, msg := range msgs {
		if !failedMessageNames.Has(msg.Name) {
			msg.OnFlyRedactUsingExistingEntities = true
		}
	}
}

func (s *ServiceImpl) invokeWebhookForUpsertedMessages(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	conversation *redisconvpb.Conversation,
	upserted map[dbUpsertResult][]*dbmodel.Messages,
) {
	for dur, msgs := range upserted {
		if dur != dbResultInserted && dur != dbResultUpdateIncompleteToComplete {
			continue
		}
		for _, dbMsg := range msgs {
			s.invokeMessageWebhook(ctx, conversationName, conversation, dbMsg)
		}
	}
}

func (s *ServiceImpl) maybeUpdateConversationForUpsertMsgs(
	ctx context.Context,
	conversationName *conversationpb.ConversationName,
	dbMsgs []*dbmodel.Messages,
	conversationParticipants *ConversationParticipants,
) (err error) {
	err = s.updateConversation(ctx, conversationName, func(existing *dbmodel.Chats) (toUpdate interface{}, err error) {
		startTime := existing.StartedAt
		endTime := existing.EndedAt
		hasParticipantUpdate := conversationParticipants.hasDiff(ctx, existing.Metadata.Message.GetParticipants())
		hasUpdate := hasParticipantUpdate
		newDisplayName := ""
		for _, dbMsg := range dbMsgs {
			if dbMsg.PlatformTimestamp.Before(startTime) {
				startTime = dbMsg.PlatformTimestamp
				hasUpdate = true
			}
			if endTime.Valid {
				msgEndTime := dbMsg.PlatformTimestamp
				if dbMsg.EndTimeMs.Valid && dbMsg.StartTimeMs.Valid {
					msgEndTime = msgEndTime.Add(time.Duration(dbMsg.EndTimeMs.Int32-dbMsg.StartTimeMs.Int32) * time.Millisecond)
				}
				if msgEndTime.After(endTime.Time) {
					endTime.Time = msgEndTime
					hasUpdate = true
				}
			}

			email := dbMsg.Metadata.Message.GetEmail()
			if email != nil {
				if email.GetSubject() != "" {
					// Update conversation display name to be the email subject if:
					//  1. Conversation display name is not set yet.
					//  2. or, conversation display name is not the same as the email subject, and the email is the first message of the conversation.
					if existing.Metadata.Message.GetDisplayName() == "" || (existing.Metadata.Message.GetDisplayName() != email.GetSubject() && startTime.Equal(dbMsg.PlatformTimestamp)) {
						newDisplayName = email.GetSubject()
						hasUpdate = true
					}
				}
			}
		}
		if !hasUpdate {
			return nil, nil
		}

		ret := &dbmodel.Chats{}
		if startTime != existing.StartedAt {
			ret.StartedAt = startTime
		}
		if endTime.Valid && endTime.Time != existing.EndedAt.Time {
			ret.EndedAt = endTime
			ret.LastIngestedAt = dbconverter.ConvertToNullTime(time.Now())
		}
		if hasParticipantUpdate || newDisplayName != "" {
			ret.Metadata.Message = s.CloneOrNewChatMetadata(existing.Metadata.Message)
		}
		if newDisplayName != "" {
			ret.Metadata.Message.DisplayName = newDisplayName
		}
		if hasParticipantUpdate {
			newParticipantsList := conversationParticipants.getParticipants()
			slices.SortFunc(newParticipantsList, func(a, b *dbconvpb.ConversationParticipant) int {
				return strings.Compare(a.Id, b.Id)
			})
			ret.Metadata.Message.Participants = newParticipantsList
		}
		return ret, nil
	})
	return err
}
