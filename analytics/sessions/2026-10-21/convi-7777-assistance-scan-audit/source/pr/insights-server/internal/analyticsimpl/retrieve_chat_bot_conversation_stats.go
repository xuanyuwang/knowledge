package analyticsimpl

import (
	"context"
	"database/sql"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

type keyChatBotConvStatsRow struct {
	postgres.KeyTimeRow
	ConversationSum int
	MessageSum      int
	CreatedAt       time.Time
}

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveChatBotConversationStats(
	ctx context.Context, req *analyticspb.RetrieveChatBotConversationStatsRequest,
) (*analyticspb.RetrieveChatBotConversationStatsResponse, error) {
	db := a.appsDB.DB(ctx)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	if req.Mode != analyticspb.Mode_MODE_REAL_TIME {
		return nil, status.Errorf(codes.Unimplemented, "batch mode is no longer supported; only MODE_REAL_TIME is available")
	}

	startTime, endTime, err := shared.ParseFilterByTimeRange(req.FilterByTimeRange)
	if err != nil {
		return nil, err
	}
	subQueryORM := db.Model(&model.Chats{}).
		Select(
			"DENSE_RANK() OVER (ORDER BY app.chats.id desc) + DENSE_RANK() OVER (ORDER BY app.chats.id asc) - 1  AS conversation_sum, "+
				"COUNT(app.messages.id) OVER () AS message_sum", "app.chats.id, app.messages.id, app.messages.created_at as created_at").
		Joins("LEFT JOIN app.messages ON app.chats.id = app.messages.chat_id").
		Joins(postgres.LeftJoinAppUserOnAppChatsTableQuery).
		Where("app.chats.agent_type = 1").
		Where(
			"app.chats.created_at >= @start_ts_sec AND app.chats.created_at < @end_ts_sec",
			sql.Named("start_ts_sec", startTime),
			sql.Named("end_ts_sec", endTime)).
		Where(
			"app.chats.customer_id = @customer_id AND app.chats.profile_id = @profile_id",
			sql.Named("customer_id", parent.CustomerID),
			sql.Named("profile_id", parent.ProfileID))
	conditions, err := postgres.ParseFilterByChatBotAttributeToCondition(
		req.FilterByChatBotAttribute)
	if err != nil {
		return nil, err
	}
	subQueryORM = subQueryORM.Scopes(conditions)

	var keyChatBotConvStatsRows []keyChatBotConvStatsRow
	subQueryORM.Scan(&keyChatBotConvStatsRows)
	if len(keyChatBotConvStatsRows) == 0 {
		return &analyticspb.RetrieveChatBotConversationStatsResponse{}, nil
	}

	return convertRowsToChatBotConversationStatsRealtimeResult(
		keyChatBotConvStatsRows[0], keyChatBotConvStatsRows)
}

func convertRowsToChatBotConversationStatsRealtimeResult(
	chatBotConvStatsRow keyChatBotConvStatsRow, keyChatBotConvStatsRows []keyChatBotConvStatsRow,
) (*analyticspb.RetrieveChatBotConversationStatsResponse, error) {
	if chatBotConvStatsRow.ConversationSum+len(keyChatBotConvStatsRows) == 0 {
		return &analyticspb.RetrieveChatBotConversationStatsResponse{}, nil
	}
	convStatsResult := &analyticspb.ChatBotConversationStatsResult{}
	convStatsResult.TotalConversationCount = int32(chatBotConvStatsRow.ConversationSum)
	convStatsResult.TotalMessageCount = int32(chatBotConvStatsRow.MessageSum)
	convStatsResult.ChatBotConversationStats = make([]*analyticspb.ChatBotConversationStats, len(keyChatBotConvStatsRows))
	for index, row := range keyChatBotConvStatsRows {
		convStatsResult.ChatBotConversationStats[index] = &analyticspb.ChatBotConversationStats{
			TotalConversationCount: 1,
			TotalMessageCount:      1,
			TimeRange: &analyticspb.TimeRange{
				StartTimestamp: timestamppb.New(row.CreatedAt),
				EndTimestamp:   timestamppb.New(row.CreatedAt),
			},
		}
	}
	conversationStatsResults := []*analyticspb.ChatBotConversationStatsResult{
		convStatsResult,
	}

	return &analyticspb.RetrieveChatBotConversationStatsResponse{
		TotalConversationCount:   int32(chatBotConvStatsRow.ConversationSum),
		TotalMessageCount:        int32(chatBotConvStatsRow.MessageSum),
		ConversationStatsResults: sortChatBotConversationStatsResult(conversationStatsResults),
	}, nil
}

func sortChatBotConversationStatsResult(
	conversationStatsResults []*analyticspb.ChatBotConversationStatsResult,
) []*analyticspb.ChatBotConversationStatsResult {
	sort.Slice(conversationStatsResults, func(i, j int) bool {
		if len(conversationStatsResults[i].ChatBotAttribute.UserResponseTypes) != 0 &&
			len(conversationStatsResults[j].ChatBotAttribute.UserResponseTypes) != 0 &&
			conversationStatsResults[i].ChatBotAttribute.UserResponseTypes[0] !=
				conversationStatsResults[j].ChatBotAttribute.UserResponseTypes[0] {
			return conversationStatsResults[i].ChatBotAttribute.UserResponseTypes[0] <
				conversationStatsResults[j].ChatBotAttribute.UserResponseTypes[0]
		}
		if len(conversationStatsResults[i].ChatBotAttribute.DeflectionTypes) != 0 &&
			len(conversationStatsResults[j].ChatBotAttribute.DeflectionTypes) != 0 &&
			conversationStatsResults[i].ChatBotAttribute.DeflectionTypes[0] !=
				conversationStatsResults[j].ChatBotAttribute.DeflectionTypes[0] {
			return conversationStatsResults[i].ChatBotAttribute.DeflectionTypes[0] <
				conversationStatsResults[j].ChatBotAttribute.DeflectionTypes[0]
		}
		if len(conversationStatsResults[i].ChatBotAttribute.Intents) != 0 &&
			len(conversationStatsResults[j].ChatBotAttribute.Intents) != 0 &&
			conversationStatsResults[i].ChatBotAttribute.Intents[0].Name !=
				conversationStatsResults[j].ChatBotAttribute.Intents[0].Name {
			return conversationStatsResults[i].ChatBotAttribute.Intents[0].Name <
				conversationStatsResults[j].ChatBotAttribute.Intents[0].Name
		}
		if len(conversationStatsResults[i].ChatBotAttribute.Versions) != 0 &&
			len(conversationStatsResults[j].ChatBotAttribute.Versions) != 0 {
			if conversationStatsResults[i].ChatBotAttribute.Versions[0].MajorVersion !=
				conversationStatsResults[j].ChatBotAttribute.Versions[0].MajorVersion {
				return conversationStatsResults[i].ChatBotAttribute.Versions[0].MajorVersion <
					conversationStatsResults[j].ChatBotAttribute.Versions[0].MajorVersion
			}
			if conversationStatsResults[i].ChatBotAttribute.Versions[0].MinorVersion !=
				conversationStatsResults[j].ChatBotAttribute.Versions[0].MinorVersion {
				return conversationStatsResults[i].ChatBotAttribute.Versions[0].MinorVersion <
					conversationStatsResults[j].ChatBotAttribute.Versions[0].MinorVersion
			}
			if conversationStatsResults[i].ChatBotAttribute.Versions[0].RevisionVersion !=
				conversationStatsResults[j].ChatBotAttribute.Versions[0].RevisionVersion {
				return conversationStatsResults[i].ChatBotAttribute.Versions[0].RevisionVersion <
					conversationStatsResults[j].ChatBotAttribute.Versions[0].RevisionVersion
			}
		}
		return true
	})
	return conversationStatsResults
}
