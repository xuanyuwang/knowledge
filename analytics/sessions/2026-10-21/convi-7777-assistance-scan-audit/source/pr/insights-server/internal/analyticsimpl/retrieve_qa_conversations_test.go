package analyticsimpl

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/golang/mock/gomock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/insights-server/internal/shared/descriptors"
	sharedauth "github.com/cresta/go-servers/shared/auth"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	"github.com/cresta/go-servers/shared/db/converter"
	testingauth "github.com/cresta/go-servers/testing/shared/auth"
)

type RetrieveQAConversationsTestSuite struct {
	AnalyticsTestSuite
	users               *UsersData
	dates               *TimePeriodData
	templateID          string
	conversationID      string
	reuseConversationID bool
	nextLegacyID        int32
}

func TestRetrieveQAConversations(t *testing.T) {
	suite.Run(t, &RetrieveQAConversationsTestSuite{})
}

func (r *RetrieveQAConversationsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()

	r.users = r.SetupUsersData()
	r.dates = r.SetupTimePeriodData()
	r.nextLegacyID = 1
	r.UseMockedClickhouseConnections()

	mockAuthEntityProvider := authmocks.NewMockAuthEntityProvider(gomock.NewController(r.T()))
	mockAuthEntityProvider.EXPECT().GetAuthEntity(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, customerID string) (*sharedauth.PrincipalEntity, error) {
			return &sharedauth.PrincipalEntity{}, nil
		})
	r.server.authEntityProvider = mockAuthEntityProvider
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

func (r *RetrieveQAConversationsTestSuite) BeforeTest(suiteName, methodName string) {
	r.templateID = methodName
	r.conversationID = ""
	r.reuseConversationID = false
}

func (r *RetrieveQAConversationsTestSuite) TestRetrieveQaConversations() {
	// Given
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockRows := clickhousetesting.NewMockRows(nil)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	_, err := r.server.RetrieveQAConversations(
		r.ctx,
		&analyticspb.RetrieveQAConversationsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
				MomentGroups: []*analyticspb.MomentGroup{
					{
						Moments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
						MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
							{
								MetadataValue: &commonpb.MetadataValue{
									Value: &commonpb.MetadataValue_StringValue{
										StringValue: "metadata-value-1",
									},
								},
							},
							{
								MetadataValue: &commonpb.MetadataValue{
									Value: &commonpb.MetadataValue_StringValue{
										StringValue: "metadata-value-2",
									},
								},
							},
						},
					},
					{
						Moments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_2",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
						MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
							{
								NumericBin: &commonpb.NumericBin{
									FromValue: 0,
									ToValue:   100,
								},
							},
							{
								NumericBin: &commonpb.NumericBin{
									FromValue: 100,
									ToValue:   250,
								},
							},
						},
					},
					{
						ExcludedMoments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_3",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
					},
				},
			},
			FilterByTimeRange: &r.dates.janPeriod,
			Metadata:          &r.dates.metadataLA,
			PageSize:          50,
			PageToken:         "50",
		},
	)

	// Then
	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
	r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
	expectedCHQuery := r.getFile("clickhouse_RetrieveQaConversations_request.sql")
	r.Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
}

func (r *RetrieveQAConversationsTestSuite) TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue() {
	// Given
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockRows := clickhousetesting.NewMockRows(nil)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	_, err := r.server.RetrieveQAConversations(
		r.ctx,
		&analyticspb.RetrieveQAConversationsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
				MomentGroups: []*analyticspb.MomentGroup{
					{
						Moments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
						ExcludedMoments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
						MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
							{
								MetadataValue: &commonpb.MetadataValue{
									Value: &commonpb.MetadataValue_StringValue{
										StringValue: "metadata-value-1",
									},
								},
							},
							{
								MetadataValue: &commonpb.MetadataValue{
									Value: &commonpb.MetadataValue_StringValue{
										StringValue: "metadata-value-2",
									},
								},
							},
						},
					},
					{
						Moments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_2",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
						MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
							{
								NumericBin: &commonpb.NumericBin{
									FromValue: 0,
									ToValue:   100,
								},
							},
							{
								NumericBin: &commonpb.NumericBin{
									FromValue: 100,
									ToValue:   250,
								},
							},
						},
					},
					{
						ExcludedMoments: []*momentpb.Moment{
							{
								Name: "customers/customer_id/profiles/profile_id/moments/moment_id_3",
								Type: momentpb.Moment_CONVERSATION_METADATA,
							},
						},
					},
				},
			},
			FilterByTimeRange: &r.dates.janPeriod,
			Metadata:          &r.dates.metadataLA,
			PageSize:          50,
			PageToken:         "50",
		},
	)

	// Then
	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
	r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
	expectedCHQuery := r.getFile("clickhouse_RetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql")
	r.Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
}

func (r *RetrieveQAConversationsTestSuite) TestRetrieveQaConversations_FilterByConversationEndedAt() {
	// Given
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockRows := clickhousetesting.NewMockRows(nil)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	_, err := r.server.RetrieveQAConversations(
		r.ctx,
		&analyticspb.RetrieveQAConversationsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
			},
			FilterByTimeRange:          &r.dates.janPeriod,
			ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
			Metadata:                   &r.dates.metadataLA,
			PageSize:                   50,
			PageToken:                  "50",
		},
	)

	// Then
	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
	r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
	expectedCHQuery := r.getFile("clickhouse_RetrieveQaConversations_FilterByConversationEndedAt_request.sql")
	r.Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
}

func (r *RetrieveQAConversationsTestSuite) TestRetrieveQaConversations_ScorecardSubmitTime() {
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	mockRows := clickhousetesting.NewMockRows(nil)
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	_, err := r.server.RetrieveQAConversations(
		r.ctx,
		&analyticspb.RetrieveQAConversationsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
			},
			FilterByTimeRange:     &r.dates.janPeriod,
			TimeRangeFilterTarget: analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
			Metadata:              &r.dates.metadataLA,
			PageSize:              50,
			PageToken:             "50",
		},
	)

	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
	gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
	gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
	fmt.Printf("got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
	expectedCHQuery := r.getFile("clickhouse_RetrieveQaConversations_ScorecardSubmitTime_request.sql")
	r.Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
}

func (r *RetrieveQAConversationsTestSuite) TestRetrieveQaConversations_SubmitTimeWithMetadataOverlap() {
	// Given: selected metadata or no value, filtered by scorecard submission time.
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).
		Return(clickhousetesting.NewMockRows(nil), nil)
	moment := &momentpb.Moment{
		Name: "customers/customer_id/profiles/profile_id/moments/team",
		Type: momentpb.Moment_CONVERSATION_METADATA,
	}

	// When: invoke the endpoint so the test covers the range passed to the parser.
	_, err := r.server.RetrieveQAConversations(r.ctx, &analyticspb.RetrieveQAConversationsRequest{
		Parent: r.profileResourceName,
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{descriptors.ScorecardTemplateName{
				CustomerID: r.customerID,
				ProfileID:  r.profileID,
				TemplateID: r.templateID,
			}.String()},
			MomentGroups: []*analyticspb.MomentGroup{{
				Moments:         []*momentpb.Moment{moment},
				ExcludedMoments: []*momentpb.Moment{moment},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
					MetadataValue: &commonpb.MetadataValue{
						Value: &commonpb.MetadataValue_StringValue{StringValue: "selected-team"},
					},
				}},
			}},
		},
		FilterByTimeRange:     &r.dates.janPeriod,
		TimeRangeFilterTarget: analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
	})

	// Then: older conversations must retain their actual metadata, not appear missing.
	r.Require().NoError(err)
	r.Require().Len(r.convoMockConn.Calls, 1)
	call := r.convoMockConn.Calls[0]
	query := combineQueryAndArgs(call.Arguments.Get(1).(string), call.Arguments.Get(2).([]any))
	r.Contains(query, "scorecard_submit_time >= '2021-01-01 00:00:00'")
	r.Contains(query, "scorecard_submit_time < '2021-01-31 00:00:00'")
	r.Contains(query, "metadata_string_value = 'selected-team'")
	r.Contains(query, "ifNull(moment_annotation_exclude_filter_0.conversation_id_exclude_0, '') = ''")
	r.NotContains(query, "conversation_start_time >=")
	r.NotContains(query, "conversation_start_time <")
}

func (r *RetrieveQAConversationsTestSuite) TestRetrieveQaConversations_SubmitTimeWithEndedAtThrows() {
	_, err := r.server.RetrieveQAConversations(
		r.ctx,
		&analyticspb.RetrieveQAConversationsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
			},
			FilterByTimeRange:          &r.dates.janPeriod,
			ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
			TimeRangeFilterTarget:      analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
			Metadata:                   &r.dates.metadataLA,
		},
	)
	r.Require().Error(err)
	r.Contains(err.Error(), "time_range_filter_target=SUBMIT_TIME is incompatible with conversation_time_range_field=CONVERSATION_ENDED_AT")
}

// Test readQaConversationsFromClickhouse with include_comments=true
func (r *RetrieveQAConversationsTestSuite) TestReadQaConversationsFromClickhouse_WithComments() {
	// Given
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	scorecardID := "scorecard_with_comments"
	conversationID := "conv_with_comments"
	scoreID1 := "score_crit1"
	scoreID2 := "score_crit2"
	scoreID3 := "score_crit3_no_comment"

	templateRevision := "rev1"
	r.Require().NoError(r.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:   CustomerID,
		Profile:    ProfileID,
		ResourceID: r.templateID,
		Revision:   templateRevision,
		Title:      "Test Template",
		Template:   "{}",
		UsecaseIds: []string{UsecaseID},
	}).Error)

	scorecard := &dbmodel.Scorecards{
		Customer:         CustomerID,
		Profile:          ProfileID,
		ResourceID:       scorecardID,
		TemplateID:       r.templateID,
		TemplateRevision: templateRevision,
		ConversationID:   conversationID,
		AgentUserID:      r.users.agentUserA.UserID.String,
		SubmittedAt:      converter.ConvertToNullTime(time.Now().UTC()),
	}
	r.Require().NoError(r.DB().Create(scorecard).Error)

	scores := []*dbmodel.Scores{
		{
			Customer:            CustomerID,
			Profile:             ProfileID,
			ResourceID:          scoreID1,
			ScorecardID:         scorecardID,
			CriterionIdentifier: Crit1,
			Comment:             sql.NullString{Valid: true, String: "Excellent greeting"},
			NumericValue:        sql.NullFloat64{Valid: true, Float64: 2.0},
		},
		{
			Customer:            CustomerID,
			Profile:             ProfileID,
			ResourceID:          scoreID2,
			ScorecardID:         scorecardID,
			CriterionIdentifier: Crit2,
			Comment:             sql.NullString{Valid: true, String: "good"},
			NumericValue:        sql.NullFloat64{Valid: true, Float64: 1.5},
		},
		{
			Customer:            CustomerID,
			Profile:             ProfileID,
			ResourceID:          scoreID3,
			ScorecardID:         scorecardID,
			CriterionIdentifier: Crit3,
			Comment:             sql.NullString{Valid: false},
			NumericValue:        sql.NullFloat64{Valid: true, Float64: 1.0},
		},
	}
	r.Require().NoError(r.DB().Create(scores).Error)

	// Create conversation
	conversation := &dbmodel.Chats{
		CustomerID:        sql.NullString{Valid: true, String: CustomerID},
		ProfileID:         sql.NullString{Valid: true, String: ProfileID},
		ConversationID:    sql.NullString{Valid: true, String: conversationID},
		StartedAt:         r.dates.janSeventh,
		PlatformChatID:    sql.NullString{Valid: true, String: "platform123"},
		PlatformAccountID: "account123",
	}
	r.Require().NoError(r.DB().Create(conversation).Error)

	mockRows := clickhousetesting.NewMockRows([][]interface{}{
		{
			conversationID,                   // conversation_id
			r.dates.janSeventh,               // conversation_start_time
			r.users.agentUserA.UserID.String, // agent_user_id
			r.templateID,                     // scorecard_template_id
			"rev1",                           // scorecard_template_revision
			scorecardID,                      // scorecard_id
			scoreID1,                         // score_id
			Crit1,                            // criterion_id
			true,                             // manually_scored
			2.0,                              // numeric_value
			2.0,                              // max_value
			1.0,                              // percentage_value
			int32(1),                         // weight
			1.0,                              // ai_value
			true,                             // ai_scored
			false,                            // auto_failed
			false,                            // not_applicable
		},
		{
			conversationID,
			r.dates.janSeventh,
			r.users.agentUserA.UserID.String,
			r.templateID,
			"rev1",
			scorecardID,
			scoreID2,
			Crit2,
			true,
			1.5,
			2.0,
			0.75,
			int32(1),
			1.0,
			true,
			false,
			false,
		},
		{
			conversationID,
			r.dates.janSeventh,
			r.users.agentUserA.UserID.String,
			r.templateID,
			"rev1",
			scorecardID,
			scoreID3,
			Crit3,
			true,
			1.0,
			2.0,
			0.5,
			int32(1),
			1.0,
			true,
			false,
			false,
		},
	})
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	req := &analyticspb.RetrieveQAConversationsRequest{
		Parent: r.profileResourceName,
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{
				descriptors.ScorecardTemplateName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					TemplateID: r.templateID,
				}.String(),
			},
		},
		FilterByTimeRange: &r.dates.janPeriod,
		IncludeComments:   true,
	}

	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	pageSize := 20
	offset := 0

	authedUser := &dbmodel.Users{
		CustomerID: sql.NullString{Valid: true, String: r.customerID},
		UserID:     sql.NullString{Valid: true, String: r.users.agentUserA.UserID.String},
		UserRoles:  pq.Int32Array{},
	}
	ctx, _ := testingauth.GetIncomeCxtWithAuth(authedUser, "")

	resp, err := r.server.readQaConversationsFromClickhouse(ctx, req, parent, pageSize, offset)

	// Then: Assert response length and verify per-score comment checks
	r.Require().NoError(err)
	r.Require().Len(resp.ConversationInfos, 3, "should return all three scores")

	// Verify comments are included for scores that have them
	scoreIDToInfo := make(map[string]*analyticspb.QAConversationInfo)
	for _, info := range resp.ConversationInfos {
		scoreIDToInfo[info.ScoreId] = info
	}

	info1, exists := scoreIDToInfo[scoreID1]
	r.Require().True(exists, "scoreID1 should be in response")
	r.Equal(scoreID1, info1.ScoreId, "scoreID1 should match")
	r.Equal("Excellent greeting", info1.Comment, "scoreID1 should have comment")

	info2, exists := scoreIDToInfo[scoreID2]
	r.Require().True(exists, "scoreID2 should be in response")
	r.Equal(scoreID2, info2.ScoreId, "scoreID2 should match")
	r.Equal("good", info2.Comment, "scoreID2 should have comment")

	info3, exists := scoreIDToInfo[scoreID3]
	r.Require().True(exists, "scoreID3 should be in response")
	r.Equal(scoreID3, info3.ScoreId, "scoreID3 should match")
	r.Empty(info3.Comment, "scoreID3 should not have comment (NULL in DB)")
}

// Test readQaConversationsFromClickhouse with include_comments=false
func (r *RetrieveQAConversationsTestSuite) TestReadQaConversationsFromClickhouse_WithoutComments() {
	// Given
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	scorecardID := "scorecard_no_comments"
	conversationID := "conv_no_comments"
	scoreID1 := "score_no_comment"

	templateRevision := "rev1"
	r.Require().NoError(r.DB().Create(&dbmodel.ScorecardTemplates{
		Customer:   CustomerID,
		Profile:    ProfileID,
		ResourceID: r.templateID,
		Revision:   templateRevision,
		Title:      "Test Template",
		Template:   "{}",
		UsecaseIds: []string{UsecaseID},
	}).Error)

	scorecard := &dbmodel.Scorecards{
		Customer:         CustomerID,
		Profile:          ProfileID,
		ResourceID:       scorecardID,
		TemplateID:       r.templateID,
		TemplateRevision: templateRevision,
		ConversationID:   conversationID,
		AgentUserID:      r.users.agentUserA.UserID.String,
		SubmittedAt:      converter.ConvertToNullTime(time.Now().UTC()),
	}
	r.Require().NoError(r.DB().Create(scorecard).Error)

	score := &dbmodel.Scores{
		Customer:            CustomerID,
		Profile:             ProfileID,
		ResourceID:          scoreID1,
		ScorecardID:         scorecardID,
		CriterionIdentifier: Crit1,
		Comment:             sql.NullString{Valid: true, String: "this comment should not be included"},
		NumericValue:        sql.NullFloat64{Valid: true, Float64: 2.0},
	}
	r.Require().NoError(r.DB().Create(score).Error)

	conversation := &dbmodel.Chats{
		CustomerID:        sql.NullString{Valid: true, String: CustomerID},
		ProfileID:         sql.NullString{Valid: true, String: ProfileID},
		ConversationID:    sql.NullString{Valid: true, String: conversationID},
		StartedAt:         r.dates.janSeventh,
		PlatformChatID:    sql.NullString{Valid: true, String: "platform123"},
		PlatformAccountID: "account123",
	}
	r.Require().NoError(r.DB().Create(conversation).Error)

	mockRows := clickhousetesting.NewMockRows([][]interface{}{
		{
			conversationID,
			r.dates.janSeventh,
			r.users.agentUserA.UserID.String,
			r.templateID,
			"rev1",
			scorecardID,
			scoreID1,
			Crit1,
			true,
			2.0,
			2.0,
			1.0,
			int32(1),
			1.0,
			true,
			false,
			false,
		},
	})
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	req := &analyticspb.RetrieveQAConversationsRequest{
		Parent: r.profileResourceName,
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{
				descriptors.ScorecardTemplateName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					TemplateID: r.templateID,
				}.String(),
			},
		},
		FilterByTimeRange: &r.dates.janPeriod,
		IncludeComments:   false,
	}

	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	pageSize := 20
	offset := 0
	resp, err := r.server.readQaConversationsFromClickhouse(r.ctx, req, parent, pageSize, offset)

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.ConversationInfos, 1, "should return one conversation info")

	info := resp.ConversationInfos[0]
	r.Equal(scoreID1, info.ScoreId, "scoreID should match")
	r.Empty(info.Comment, "comment should not be included when include_comments=false")
}

// Test readQaConversationsFromClickhouse with scorecardTable (should not fetch comments)
func (r *RetrieveQAConversationsTestSuite) TestReadQaConversationsFromClickhouse_ScorecardTable_NoComments() {
	// Given
	defer r.convoMockConn.AssertExpectationsAndReset(r.T())
	// Setup with scorecardTable resource type
	conversation := &dbmodel.Chats{
		CustomerID:     converter.ConvertToNullString(r.customerID),
		ProfileID:      converter.ConvertToNullString(r.profileID),
		ConversationID: converter.ConvertToNullString("conv_1"),
	}

	r.Require().NoError(r.DB().Create(conversation).Error)

	mockRows := clickhousetesting.NewMockRows([][]interface{}{
		{
			"conv_1",
			r.dates.janSeventh,
			r.users.agentUserA.UserID.String,
			r.templateID,
			"rev1",
			"scorecard_1",
			"",
			"",
			true,
			2.0,
			1.0,
			1.0,
			int32(1),
			0.0,
			true,
			false,
			false,
		},
	})
	r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// When
	// include_comments=true but using scorecardTable
	req := &analyticspb.RetrieveQAConversationsRequest{
		Parent: r.profileResourceName,
		FilterByAttribute: &analyticspb.QAAttribute{
			ScorecardTemplates: []string{
				descriptors.ScorecardTemplateName{
					CustomerID: r.customerID,
					ProfileID:  r.profileID,
					TemplateID: r.templateID,
				}.String(),
			},
		},
		FilterByTimeRange: &r.dates.janPeriod,
		ScoreResource:     analyticspb.RetrieveQAConversationsRequest_QA_SCORE_RESOURCE_SCORECARD, // scorecardTable
		IncludeComments:   true,
	}

	parent := &shared.CustomerProfile{
		CustomerID: r.customerID,
		ProfileID:  r.profileID,
	}
	pageSize := 20
	offset := 0
	resp, err := r.server.readQaConversationsFromClickhouse(r.ctx, req, parent, pageSize, offset)

	// Then
	r.Require().NoError(err)
	r.Require().Len(resp.ConversationInfos, 1, "should return one conversation info")

	info := resp.ConversationInfos[0]
	r.Empty(info.ScoreId, "scorecardTable should have empty score_id")
	r.Empty(info.Comment, "comments should not be fetched for scorecardTable even when IncludeComments=true")
}
