package analyticsimpl

import (
	"database/sql"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

type RetrieveCommentStatsTestSuite struct {
	AnalyticsTestSuite
	usersData                           *UsersData
	janFirstPST                         time.Time
	janFirstPSTProto                    timestamppb.Timestamp
	sundayOfTheWeekOfJanFirstPSTProto   timestamppb.Timestamp
	janSeventhPST                       time.Time
	janSeventhPSTProto                  timestamppb.Timestamp
	sundayOfTheWeekOfJanSeventhPSTProto timestamppb.Timestamp
	janTenthPST                         time.Time
	janTenthPSTProto                    timestamppb.Timestamp
	janEleventhPSTProto                 timestamppb.Timestamp
}

func (r *RetrieveCommentStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.AnalyticsTestSuite.server.environment = shared.Chat
	pst, _ := time.LoadLocation(r.losAngeles)
	r.janFirstPST = time.Date(2021, 1, 1, 0, 0, 0, 0, pst)
	r.janFirstPSTProto = *timestamppb.New(r.janFirstPST)
	r.sundayOfTheWeekOfJanFirstPSTProto = *timestamppb.New(time.Date(2020, 12, 27, 0, 0, 0, 0, pst))
	r.janSeventhPST = time.Date(2021, 1, 7, 0, 0, 0, 0, pst)
	r.janSeventhPSTProto = *timestamppb.New(r.janSeventhPST)
	r.sundayOfTheWeekOfJanSeventhPSTProto = *timestamppb.New(time.Date(2021, 1, 3, 0, 0, 0, 0, pst))
	r.janTenthPST = time.Date(2021, 1, 10, 0, 0, 0, 0, pst)
	r.janTenthPSTProto = *timestamppb.New(r.janTenthPST)
	r.janEleventhPSTProto = *timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, pst))

	r.usersData = r.SetupUsersData()

	// Create chats and chat_comments rows in raw table.
	r.insertChatsRow("convo_1", UsecaseID)
	r.insertChatsRow("convo_2", UsecaseID)
	r.insertChatsRow("convo_3", UsecaseID)
	r.insertChatsRow("convo_4", UsecaseID)
	r.insertChatsRow("convo_5", UsecaseID)
	r.insertChatsRow("convo_6", UsecaseID)
	r.insertChatsRow("convo_7", UsecaseID)
	r.insertChatCommentsRow(r.usersData.agentUserA.UserID.String, r.janFirstPST, "convo_1", "1")
	r.insertChatCommentsRow(r.usersData.agentUserA.UserID.String, r.janSeventhPST, "convo_2", "2")
	r.insertChatCommentsRow(r.usersData.agentUserA.UserID.String, r.janSeventhPST, "convo_3", "3")
	r.insertChatCommentsRow(r.usersData.agentUserB.UserID.String, r.janFirstPST, "convo_4", "4")
	r.insertChatCommentsRow(r.usersData.agentUserB.UserID.String, r.janFirstPST, "convo_5", "5")
	r.insertChatCommentsRow(r.usersData.agentUserB.UserID.String, r.janFirstPST, "convo_6", "6")
	r.insertChatCommentsRow(r.usersData.agentUserB.UserID.String, r.janSeventhPST, "convo_7", "7")
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func (r *RetrieveCommentStatsTestSuite) insertChatCommentsRow(
	managerUserID string, createdAt time.Time, conversationID, commentID string,
) {
	row := &dbmodel.ChatComments{
		CustomerID:     CustomerID,
		ProfileID:      ProfileID,
		ConversationID: conversationID,
		CommentID:      commentID,
		UserID:         managerUserID,
		CreatedAt:      createdAt,
	}
	r.CreateModels(row)
}

func (r *RetrieveCommentStatsTestSuite) insertChatsRow(
	conversationID, usecaseID string,
) {
	row := &dbmodel.Chats{
		CustomerID:     sql.NullString{String: CustomerID, Valid: true},
		ProfileID:      sql.NullString{String: ProfileID, Valid: true},
		ConversationID: sql.NullString{String: conversationID, Valid: true},
		UsecaseID:      sql.NullString{String: usecaseID, Valid: true},
	}
	r.CreateModels(row)
}

// nolint:dupl
func TestRetrieveCommentStats(t *testing.T) {
	r := RetrieveCommentStatsTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc      string
		req       *analyticspb.RetrieveCommentStatsRequest
		want      *analyticspb.RetrieveCommentStatsResponse
		errorCode codes.Code
	}{
		{
			desc: "TestRequestWeeklyGroupByAgent",
			req: &analyticspb.RetrieveCommentStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstPSTProto,
					EndTimestamp:   &r.janEleventhPSTProto,
				},
				Frequency: analyticspb.Frequency_WEEKLY,
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
			},
			want: &analyticspb.RetrieveCommentStatsResponse{
				TotalCommentsPlacedCount:          7,
				AverageCommentsPlacedCountPerUser: 7.0 / 2,
				CommentStatsResults: []*analyticspb.CommentStatsResult{
					{
						CommentStats: []*analyticspb.CommentStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanFirstPSTProto,
								},
								TotalCommentsPlacedCount:          1,
								AverageCommentsPlacedCountPerUser: 1.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanSeventhPSTProto,
								},
								TotalCommentsPlacedCount:          2,
								AverageCommentsPlacedCountPerUser: 2.0 / 1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.usersData.agentUserA.UserID.String,
								}.String(),
								FullName: r.usersData.agentUserA.FullName.String,
								Username: r.usersData.agentUserA.Username,
							}},
						},
						TotalCommentsPlacedCount:          3,
						AverageCommentsPlacedCountPerUser: 3.0 / 1,
					}, {
						CommentStats: []*analyticspb.CommentStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanFirstPSTProto,
								},
								TotalCommentsPlacedCount:          3,
								AverageCommentsPlacedCountPerUser: 3.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanSeventhPSTProto,
								},
								TotalCommentsPlacedCount:          1,
								AverageCommentsPlacedCountPerUser: 1.0 / 1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.usersData.agentUserB.UserID.String,
								}.String(),
								FullName: r.usersData.agentUserB.FullName.String,
								Username: r.usersData.agentUserB.Username,
							}},
						},
						TotalCommentsPlacedCount:          4,
						AverageCommentsPlacedCountPerUser: 4.0 / 1,
					},
				},
			},
		},
		{
			desc: "TestRequestDailyGroupByAgent",
			req: &analyticspb.RetrieveCommentStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstPSTProto,
					EndTimestamp:   &r.janEleventhPSTProto,
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.AttributeType{
					analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &analyticspb.Metadata{
					TimeZoneId: r.losAngeles,
				},
			},
			want: &analyticspb.RetrieveCommentStatsResponse{
				TotalCommentsPlacedCount:          7,
				AverageCommentsPlacedCountPerUser: 7.0 / 2,
				CommentStatsResults: []*analyticspb.CommentStatsResult{
					{
						CommentStats: []*analyticspb.CommentStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janFirstPSTProto,
								},
								TotalCommentsPlacedCount:          1,
								AverageCommentsPlacedCountPerUser: 1.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janSeventhPSTProto,
								},
								TotalCommentsPlacedCount:          2,
								AverageCommentsPlacedCountPerUser: 2.0 / 1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.usersData.agentUserA.UserID.String,
								}.String(),
								FullName: r.usersData.agentUserA.FullName.String,
								Username: r.usersData.agentUserA.Username,
							}},
						},
						TotalCommentsPlacedCount:          3,
						AverageCommentsPlacedCountPerUser: 3.0 / 1,
					}, {
						CommentStats: []*analyticspb.CommentStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janFirstPSTProto,
								},
								TotalCommentsPlacedCount:          3,
								AverageCommentsPlacedCountPerUser: 3.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janSeventhPSTProto,
								},
								TotalCommentsPlacedCount:          1,
								AverageCommentsPlacedCountPerUser: 1.0 / 1,
							},
						},
						Attribute: &analyticspb.Attribute{
							Users: []*userpb.User{{
								Name: userpb.UserName{
									CustomerID: r.customerID,
									UserID:     r.usersData.agentUserB.UserID.String,
								}.String(),
								FullName: r.usersData.agentUserB.FullName.String,
								Username: r.usersData.agentUserB.Username,
							}},
						},
						TotalCommentsPlacedCount:          4,
						AverageCommentsPlacedCountPerUser: 4.0 / 1,
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := r.server.RetrieveCommentStats(r.ctx, tc.req)
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
