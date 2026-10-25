package analyticsimpl

import (
	"database/sql"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/db/converter"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

type RetrieveCoachingSessionStatsTestSuite struct {
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

func (r *RetrieveCoachingSessionStatsTestSuite) SetupSuite() {
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

	// Create coaching session rows in raw table.
	r.insertCoachingSessionsRow(r.usersData.agentUserA.UserID.String, r.janFirstPST, "1")
	r.insertCoachingSessionsRow(r.usersData.agentUserA.UserID.String, r.janSeventhPST, "2")
	r.insertCoachingSessionsRow(r.usersData.agentUserA.UserID.String, r.janSeventhPST, "3")
	r.insertCoachingSessionsRow(r.usersData.agentUserB.UserID.String, r.janFirstPST, "4")
	r.insertCoachingSessionsRow(r.usersData.agentUserB.UserID.String, r.janFirstPST, "5")
	r.insertCoachingSessionsRow(r.usersData.agentUserB.UserID.String, r.janFirstPST, "6")
	r.insertCoachingSessionsRow(r.usersData.agentUserB.UserID.String, r.janSeventhPST, "7")

	// Create a soft-deleted session that should be excluded from counts
	r.insertSoftDeletedCoachingSessionsRow(r.usersData.agentUserA.UserID.String, r.janFirstPST, "deleted_1")

	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func (r *RetrieveCoachingSessionStatsTestSuite) insertCoachingSessionsRow(
	managerUserID string, submittedAt time.Time, resourceID string,
) {
	r.insertCoachingSessionsRowWithGroup(managerUserID, submittedAt, resourceID, "")
}

// insertCoachingSessionsRowWithGroup seeds a coaching session row with an explicit
// group_session_id. An empty groupSessionID models a 1:1 session; a non-empty value
// (shared across rows) models a member of a group/bulk coaching event.
func (r *RetrieveCoachingSessionStatsTestSuite) insertCoachingSessionsRowWithGroup(
	managerUserID string, submittedAt time.Time, resourceID string, groupSessionID string,
) {
	row := &dbmodel.CoachingSessions{
		Customer:           CustomerID,
		Profile:            ProfileID,
		UsecaseID:          sql.NullString{String: UsecaseID, Valid: true},
		ResourceID:         resourceID,
		AgentUserID:        "some_agent_id",
		CreatorUserID:      managerUserID,
		ManagerSubmittedAt: sql.NullTime{Time: submittedAt, Valid: true},
		GroupSessionID:     converter.ConvertEmptyToInvalidNullString(groupSessionID),
	}
	r.CreateModels(row)
}

func (r *RetrieveCoachingSessionStatsTestSuite) insertSoftDeletedCoachingSessionsRow(
	managerUserID string, submittedAt time.Time, resourceID string,
) {
	row := &dbmodel.CoachingSessions{
		Customer:           CustomerID,
		Profile:            ProfileID,
		UsecaseID:          sql.NullString{String: UsecaseID, Valid: true},
		ResourceID:         resourceID,
		AgentUserID:        "some_agent_id",
		CreatorUserID:      managerUserID,
		ManagerSubmittedAt: sql.NullTime{Time: submittedAt, Valid: true},
		DeletedAt:          sql.NullTime{Time: submittedAt.Add(time.Hour), Valid: true},
	}
	r.CreateModels(row)
}

// nolint:dupl
func TestRetrieveCoachingSessionStats(t *testing.T) {
	r := RetrieveCoachingSessionStatsTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc      string
		req       *analyticspb.RetrieveCoachingSessionStatsRequest
		want      *analyticspb.RetrieveCoachingSessionStatsResponse
		errorCode codes.Code
	}{
		{
			desc: "TestRequestWeeklyGroupByAgent",
			req: &analyticspb.RetrieveCoachingSessionStatsRequest{
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
			want: &analyticspb.RetrieveCoachingSessionStatsResponse{
				CoachingSessionsSubmitted:               7,
				AverageCoachingSessionsSubmittedPerUser: 7.0 / 2,
				CoachingSessionStatsResults: []*analyticspb.CoachingSessionStatsResult{
					{
						CoachingSessionStats: []*analyticspb.CoachingSessionStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanFirstPSTProto,
								},
								CoachingSessionsSubmitted:               1,
								AverageCoachingSessionsSubmittedPerUser: 1.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanSeventhPSTProto,
								},
								CoachingSessionsSubmitted:               2,
								AverageCoachingSessionsSubmittedPerUser: 2.0 / 1,
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
						CoachingSessionsSubmitted:               3,
						AverageCoachingSessionsSubmittedPerUser: 3.0 / 1,
					}, {
						CoachingSessionStats: []*analyticspb.CoachingSessionStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanFirstPSTProto,
								},
								CoachingSessionsSubmitted:               3,
								AverageCoachingSessionsSubmittedPerUser: 3.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.sundayOfTheWeekOfJanSeventhPSTProto,
								},
								CoachingSessionsSubmitted:               1,
								AverageCoachingSessionsSubmittedPerUser: 1.0 / 1,
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
						CoachingSessionsSubmitted:               4,
						AverageCoachingSessionsSubmittedPerUser: 4.0 / 1,
					},
				},
			},
		},
		{
			desc: "TestRequestDailyGroupByAgent",
			req: &analyticspb.RetrieveCoachingSessionStatsRequest{
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
			want: &analyticspb.RetrieveCoachingSessionStatsResponse{
				CoachingSessionsSubmitted:               7,
				AverageCoachingSessionsSubmittedPerUser: 7.0 / 2,
				CoachingSessionStatsResults: []*analyticspb.CoachingSessionStatsResult{
					{
						CoachingSessionStats: []*analyticspb.CoachingSessionStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janFirstPSTProto,
								},
								CoachingSessionsSubmitted:               1,
								AverageCoachingSessionsSubmittedPerUser: 1.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janSeventhPSTProto,
								},
								CoachingSessionsSubmitted:               2,
								AverageCoachingSessionsSubmittedPerUser: 2.0 / 1,
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
						CoachingSessionsSubmitted:               3,
						AverageCoachingSessionsSubmittedPerUser: 3.0 / 1,
					}, {
						CoachingSessionStats: []*analyticspb.CoachingSessionStats{
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janFirstPSTProto,
								},
								CoachingSessionsSubmitted:               3,
								AverageCoachingSessionsSubmittedPerUser: 3.0 / 1,
							},
							{
								TimeRange: &analyticspb.TimeRange{
									StartTimestamp: &r.janSeventhPSTProto,
								},
								CoachingSessionsSubmitted:               1,
								AverageCoachingSessionsSubmittedPerUser: 1.0 / 1,
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
						CoachingSessionsSubmitted:               4,
						AverageCoachingSessionsSubmittedPerUser: 4.0 / 1,
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := r.server.RetrieveCoachingSessionStats(r.ctx, tc.req)
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

type RetrieveCoachingSessionStatsSessionTypeFilterTestSuite struct {
	AnalyticsTestSuite
	usersData                         *UsersData
	janFirstPSTProto                  timestamppb.Timestamp
	janEleventhPSTProto               timestamppb.Timestamp
	sundayOfTheWeekOfJanFirstPSTProto timestamppb.Timestamp
}

func (r *RetrieveCoachingSessionStatsSessionTypeFilterTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()
	r.AnalyticsTestSuite.server.environment = shared.Chat
	pst, _ := time.LoadLocation(r.losAngeles)
	janFirstPST := time.Date(2021, 1, 1, 0, 0, 0, 0, pst)
	r.janFirstPSTProto = *timestamppb.New(janFirstPST)
	r.janEleventhPSTProto = *timestamppb.New(time.Date(2021, 1, 11, 0, 0, 0, 0, pst))
	r.sundayOfTheWeekOfJanFirstPSTProto = *timestamppb.New(time.Date(2020, 12, 27, 0, 0, 0, 0, pst))

	r.usersData = r.SetupUsersData()
	managerUserID := r.usersData.agentUserA.UserID.String

	// One group/bulk event (two agent rows share group_session_id "g1") plus three 1:1 sessions:
	// two with a NULL group_session_id and one with an explicit empty string. All submitted by the
	// same coach in the same week.
	r.insertSessionRow(managerUserID, janFirstPST, "g1a", sql.NullString{String: "g1", Valid: true})
	r.insertSessionRow(managerUserID, janFirstPST, "g1b", sql.NullString{String: "g1", Valid: true})
	r.insertSessionRow(managerUserID, janFirstPST, "s1", sql.NullString{Valid: false})
	r.insertSessionRow(managerUserID, janFirstPST, "s2", sql.NullString{Valid: false})
	// Empty-string group_session_id must count as 1:1 via the group-session guard.
	r.insertSessionRow(managerUserID, janFirstPST, "s3", sql.NullString{String: "", Valid: true})

	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
}

func (r *RetrieveCoachingSessionStatsSessionTypeFilterTestSuite) insertSessionRow(
	managerUserID string, submittedAt time.Time, resourceID string, groupSessionID sql.NullString,
) {
	row := &dbmodel.CoachingSessions{
		Customer:           CustomerID,
		Profile:            ProfileID,
		UsecaseID:          sql.NullString{String: UsecaseID, Valid: true},
		ResourceID:         resourceID,
		AgentUserID:        "some_agent_id",
		CreatorUserID:      managerUserID,
		ManagerSubmittedAt: sql.NullTime{Time: submittedAt, Valid: true},
		GroupSessionID:     groupSessionID,
	}
	r.CreateModels(row)
}

// TestRetrieveCoachingSessionStatsGroupSessionCountsAsOne verifies the "sessions held" model on the
// managers leaderboard: a group (bulk) coaching event delivered to multiple agents counts once for
// the manager, while ungrouped 1:1 sessions keep counting individually.
func TestRetrieveCoachingSessionStatsGroupSessionCountsAsOne(t *testing.T) {
	r := RetrieveCoachingSessionStatsSessionTypeFilterTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	r.SetupSuite()
	t.Cleanup(r.TearDownSuite)

	// When
	got, err := r.server.RetrieveCoachingSessionStats(r.ctx, &analyticspb.RetrieveCoachingSessionStatsRequest{
		Parent: r.profileResourceName,
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: &r.janFirstPSTProto,
			EndTimestamp:   &r.janEleventhPSTProto,
		},
		Frequency: analyticspb.Frequency_WEEKLY,
		GroupByAttributeTypes: []analyticspb.AttributeType{
			analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
		},
		Metadata: &analyticspb.Metadata{TimeZoneId: r.losAngeles},
	})

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := &analyticspb.RetrieveCoachingSessionStatsResponse{
		// 1 group event + 3 individual sessions (NULL and empty group_session_id) = 4 held
		// sessions, collapsed from 5 rows.
		CoachingSessionsSubmitted:               4,
		AverageCoachingSessionsSubmittedPerUser: 4.0 / 1,
		CoachingSessionStatsResults: []*analyticspb.CoachingSessionStatsResult{
			{
				CoachingSessionStats: []*analyticspb.CoachingSessionStats{
					{
						TimeRange: &analyticspb.TimeRange{
							StartTimestamp: &r.sundayOfTheWeekOfJanFirstPSTProto,
						},
						CoachingSessionsSubmitted:               4,
						AverageCoachingSessionsSubmittedPerUser: 4.0 / 1,
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
				CoachingSessionsSubmitted:               4,
				AverageCoachingSessionsSubmittedPerUser: 4.0 / 1,
			},
		},
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Fatalf("Incorrect response with diff (-want +got):\n %v", diff)
	}
}

func TestRetrieveCoachingSessionStatsSessionTypeFilter(t *testing.T) {
	r := RetrieveCoachingSessionStatsSessionTypeFilterTestSuite{AnalyticsTestSuite: newAnalyticsTestSuite(t)}
	r.SetupSuite()
	t.Cleanup(r.TearDownSuite)

	for _, tc := range []struct {
		desc      string
		filter    analyticspb.CoachingSessionFilterType
		wantCount int32 // group-aware: a bulk event counts once
	}{
		{
			desc:      "AllCountsGroupEventOnce",
			filter:    analyticspb.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_ALL,
			wantCount: 4, // {g1, s1, s2, s3}
		},
		{
			desc:      "OneOnOneOnly",
			filter:    analyticspb.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_ONE_ON_ONE,
			wantCount: 3, // {s1, s2, s3} (s3 has empty-string group_session_id)
		},
		{
			desc:      "GroupOnly",
			filter:    analyticspb.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_GROUP,
			wantCount: 1, // {g1}
		},
		{
			desc:      "UnspecifiedBehavesAsAll",
			filter:    analyticspb.CoachingSessionFilterType_COACHING_SESSION_FILTER_TYPE_UNSPECIFIED,
			wantCount: 4,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Given
			req := &analyticspb.RetrieveCoachingSessionStatsRequest{
				Parent: r.profileResourceName,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: &r.janFirstPSTProto,
					EndTimestamp:   &r.janEleventhPSTProto,
				},
				Frequency:             analyticspb.Frequency_WEEKLY,
				Metadata:              &analyticspb.Metadata{TimeZoneId: r.losAngeles},
				CoachingSessionFilter: tc.filter,
			}

			// When
			got, err := r.server.RetrieveCoachingSessionStats(r.ctx, req)

			// Then
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CoachingSessionsSubmitted != tc.wantCount {
				t.Fatalf("CoachingSessionsSubmitted = %d, want %d", got.CoachingSessionsSubmitted, tc.wantCount)
			}
		})
	}
}
