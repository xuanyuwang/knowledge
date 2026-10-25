package analyticsimpl

import (
	"database/sql"
	"strconv"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RetrieveChatBotConversationStatsTestSuite struct {
	AnalyticsTestSuite
	botA                *dbmodel.Users
	botB                *dbmodel.Users
	chatVisitorA        *dbmodel.ChatVisitors
	janFirstPST         time.Time
	janFirstPSTProto    timestamppb.Timestamp
	janSeventhPST       time.Time
	janSeventhPSTProto  timestamppb.Timestamp
	janTenthPST         time.Time
	janTenthPSTProto    timestamppb.Timestamp
	janEleventhPSTProto timestamppb.Timestamp
	speakerRole         *dbmodel.SpeakerRoleTypes
}

func (r *RetrieveChatBotConversationStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	pst, _ := time.LoadLocation(r.losAngeles)
	r.janFirstPST = time.Date(2021, 1, 1, 0, 0, 0, 0, pst)
	r.janFirstPSTProto = *timestamppb.New(r.janFirstPST)
	r.janSeventhPST = time.Date(2021, 1, 7, 0, 0, 0, 0, pst)
	r.janSeventhPSTProto = *timestamppb.New(r.janSeventhPST)
	r.janTenthPST = time.Date(2021, 1, 10, 0, 0, 0, 0, pst)
	r.janTenthPSTProto = *timestamppb.New(r.janTenthPST)
	r.janEleventhPSTProto = *timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, pst))
	r.botA = &dbmodel.Users{
		CustomerID: sql.NullString{
			String: CustomerID,
			Valid:  true,
		},
		ID: 1,
		UserID: sql.NullString{
			String: "1",
			Valid:  true,
		},
		Password:  "",
		Username:  "agent_a",
		IsDevUser: false,
		FullName: sql.NullString{
			String: "agent a",
			Valid:  true,
		},
		TeamID: sql.NullString{
			String: "TeamA",
			Valid:  true,
		},
	}
	r.botB = &dbmodel.Users{
		CustomerID: sql.NullString{
			String: CustomerID,
			Valid:  true,
		},
		ID: 2,
		UserID: sql.NullString{
			String: "2",
			Valid:  true,
		},
		Password:  "",
		Username:  "agent_b",
		IsDevUser: false,
		FullName: sql.NullString{
			String: "agent b",
			Valid:  true,
		},
		TeamID: sql.NullString{
			String: "TeamB",
			Valid:  true,
		},
	}
	r.chatVisitorA = &dbmodel.ChatVisitors{
		ID:         3,
		PlatformID: "",
	}
	r.speakerRole = &dbmodel.SpeakerRoleTypes{
		ID:        "speaker_role",
		CreatedAt: time.Time{},
	}
	r.CreateModels(r.botA, r.botB, r.speakerRole)
	r.createRow(r.janSeventhPST, r.botA.ID, 10, 100)
	r.createRow(r.janSeventhPST, r.botB.ID, 1, 10)
	r.createRow(r.janSeventhPST, r.botA.ID, 1, 10)
}

func (r *RetrieveChatBotConversationStatsTestSuite) createRow(
	createdAt time.Time, botID int32,
	conversationCount int32, messageCount int32,
) {
	startConvoID := int32(r.CountRows(&dbmodel.Chats{})) + 1
	for chatID := int(startConvoID); chatID < int(startConvoID+conversationCount); chatID++ {
		convRow := &dbmodel.Chats{
			CustomerID: sql.NullString{
				String: CustomerID,
				Valid:  true,
			},
			ProfileID: sql.NullString{
				String: ProfileID,
				Valid:  true,
			},
			ID: int32(chatID),
			LatestAgentID: sql.NullInt32{
				Int32: botID,
				Valid: true,
			},
			PlatformID:         "",
			OriginalEventData:  "",
			CreatedAt:          createdAt,
			IsFinalizedByAgent: false,
			StartedAt:          createdAt,
			PlatformVisitInfo:  sql.NullString{},
			PlatformAccountID:  strconv.Itoa(chatID),
			ConversationID: sql.NullString{
				String: strconv.Itoa(chatID),
				Valid:  true,
			},
			AgentType: sql.NullInt32{
				Int32: 1,
				Valid: true,
			},
		}
		r.CreateModels(convRow)
		startMessageID := int(r.CountRows(&dbmodel.Messages{})) + 1
		for msgID := startMessageID; msgID < int(messageCount)/int(conversationCount)+startMessageID; msgID++ {
			msgRow := &dbmodel.Messages{
				CustomerID: sql.NullString{
					String: CustomerID,
					Valid:  true,
				},
				ProfileID: sql.NullString{
					String: ProfileID,
					Valid:  true,
				},
				ID:                 int64(msgID),
				Text:               "",
				ChatID:             int32(chatID),
				CreatedAt:          createdAt,
				UsedSuggestionID:   sql.NullInt32{},
				PlatformTimestamp:  createdAt,
				OriginalEventData:  "",
				AuthorIsVisitor:    false,
				PlatformSequenceID: int32(msgID),
				SpeakerRole:        r.speakerRole.ID,
				ConversationID: sql.NullString{
					String: strconv.Itoa(chatID),
					Valid:  true,
				},
				ConversationMessageID: sql.NullString{
					String: strconv.Itoa(msgID),
					Valid:  true,
				},
			}
			r.CreateModels(msgRow)
		}
	}
}

// TODO: investigate this test case with magic number
// nolint:dupl
func TestRealTimeRequest_FilterByUserId(t *testing.T) {
	r := RetrieveChatBotConversationStatsTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	r.SetupSuite()
	request := analyticspb.RetrieveChatBotConversationStatsRequest{
		Parent: r.profileResourceName,
		FilterByChatBotAttribute: &analyticspb.ChatBotAttribute{
			Users: []*userpb.User{{
				Name: userpb.UserName{
					CustomerID: r.customerID,
					UserID:     r.botA.UserID.String,
				}.String(),
			}},
		},
		FilterByTimeRange: &analyticspb.TimeRange{StartTimestamp: &r.janFirstPSTProto, EndTimestamp: &r.janTenthPSTProto},
		Metadata:          &analyticspb.Metadata{TimeZoneId: r.losAngeles},
		Mode:              analyticspb.Mode_MODE_REAL_TIME,
	}
	resultStats := make([]*analyticspb.ChatBotConversationStats, 110)
	for j := 0; j < 110; j++ {
		resultStats[j] = &analyticspb.ChatBotConversationStats{
			TotalConversationCount: int32(1),
			TotalMessageCount:      int32(1),
			TimeRange: &analyticspb.TimeRange{
				StartTimestamp: &r.janSeventhPSTProto,
				EndTimestamp:   &r.janSeventhPSTProto,
			},
		}
	}

	expectedResponse := analyticspb.RetrieveChatBotConversationStatsResponse{
		TotalConversationCount: int32(11),
		TotalMessageCount:      int32(110),
		ConversationStatsResults: []*analyticspb.ChatBotConversationStatsResult{
			{
				TotalMessageCount:        int32(110),
				TotalConversationCount:   int32(11),
				ChatBotConversationStats: resultStats,
			},
		},
	}

	got, err := r.server.RetrieveChatBotConversationStats(r.ctx, &request)
	if err != nil {
		t.Fatalf("RetrieveChatBotConversationStats failed: %v", err)
	}
	// nolint:govet // false positive
	if diff := cmp.Diff(expectedResponse, got, protocmp.Transform(), protocmp.IgnoreEmptyMessages()); diff != "" {
		t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
	}
}

// nolint:dupl
func TestRetrieveChatBotConversationStats(t *testing.T) {
	r := RetrieveChatBotConversationStatsTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc      string
		req       *analyticspb.RetrieveChatBotConversationStatsRequest
		want      *analyticspb.RetrieveChatBotConversationStatsResponse
		errorCode codes.Code
	}{
		{
			desc:      "TestEmptyRequest",
			req:       &analyticspb.RetrieveChatBotConversationStatsRequest{},
			errorCode: codes.InvalidArgument,
		},
		{
			desc: "TestBatchModeUnsupported",
			req: &analyticspb.RetrieveChatBotConversationStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstPSTProto,
					EndTimestamp:   &r.janTenthPSTProto,
				},
				Metadata: &analyticspb.Metadata{TimeZoneId: r.losAngeles},
			},
			errorCode: codes.Unimplemented,
		},
		{
			desc: "TestRealTimeRequest_InvalidTimeRange",
			req: &analyticspb.RetrieveChatBotConversationStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janTenthPSTProto,
					EndTimestamp:   &r.janFirstPSTProto,
				},
				Mode: analyticspb.Mode_MODE_REAL_TIME,
			},
			errorCode: codes.InvalidArgument,
		},
		{
			desc: "TestRealTimeRequest_EmptyResult",
			req: &analyticspb.RetrieveChatBotConversationStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janTenthPSTProto,
					EndTimestamp:   &r.janEleventhPSTProto,
				},
				Mode: analyticspb.Mode_MODE_REAL_TIME,
			},
			want: &analyticspb.RetrieveChatBotConversationStatsResponse{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := r.server.RetrieveChatBotConversationStats(r.ctx, tc.req)
			if c := status.Code(err); c != tc.errorCode {
				t.Fatalf("Got code %v in error: %v, but want code %v", err, c, tc.errorCode)
			}
			if tc.errorCode != codes.OK {
				return
			}
			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
			}
		})
	}
}
