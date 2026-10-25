// nolint:dupl
package analyticsimpl

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	actionpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/action"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	enumpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	labelspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	gwpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/guidedworkflow"
	kbpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/knowledgebase"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	tagpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/tag"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/auth"
)

type RetrieveClosedConversationsTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveClosedConversationsTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")

	s.AnalyticsV2TestSuite.SetupSuite()
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "35636a572d7ec364",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 1",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "1",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "2f540f161f3a452c",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 2",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "2",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "d072a51a8359fe51",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 3",
			Valid:  true,
		},
		IsDevUser: false,
		Username:  "3",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Users{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		UserID: sql.NullString{
			String: "user-id4-deactivated",
			Valid:  true,
		},
		FullName: sql.NullString{
			String: "Fake Agent Name 4",
			Valid:  true,
		},
		ActiveState: sql.NullInt32{
			// Deactivated user.
			Int32: 2,
			Valid: true,
		},
		IsDevUser: false,
		Username:  "4",
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfileID: sql.NullString{
			String: s.profileID,
			Valid:  true,
		},
		ID: 17684332,
		ConversationID: sql.NullString{
			String: "1",
			Valid:  true,
		},
		AgentUserID: sql.NullString{
			String: "35636a572d7ec364",
			Valid:  true,
		},
		PlatformID: "1",
		EndedAt:    sql.NullTime{Valid: true, Time: time.Date(2022, 11, 16, 5, 41, 37, 742, time.UTC)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfileID: sql.NullString{
			String: s.profileID,
			Valid:  true,
		},
		ID: 17684276,
		ConversationID: sql.NullString{
			String: "2",
			Valid:  true,
		},
		AgentUserID: sql.NullString{
			String: "2f540f161f3a452c",
			Valid:  true,
		},
		PlatformID: "2",
		EndedAt:    sql.NullTime{Valid: true, Time: time.Date(2022, 11, 16, 4, 52, 12, 436, time.UTC)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfileID: sql.NullString{
			String: s.profileID,
			Valid:  true,
		},
		ID: 17684271,
		ConversationID: sql.NullString{
			String: "3",
			Valid:  true,
		},
		AgentUserID: sql.NullString{
			String: "d072a51a8359fe51",
			Valid:  true,
		},
		PlatformID: "3",
		EndedAt:    sql.NullTime{Valid: true, Time: time.Date(2022, 11, 16, 4, 46, 50, 326, time.UTC)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfileID: sql.NullString{
			String: s.profileID,
			Valid:  true,
		},
		ID: 17684249,
		ConversationID: sql.NullString{
			String: "4",
			Valid:  true,
		},
		AgentUserID: sql.NullString{
			String: "35636a572d7ec364",
			Valid:  true,
		},
		PlatformID: "4",
		EndedAt:    sql.NullTime{Valid: true, Time: time.Date(2022, 11, 16, 4, 29, 13, 616, time.UTC)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfileID: sql.NullString{
			String: s.profileID,
			Valid:  true,
		},
		ID: 17684248,
		ConversationID: sql.NullString{
			String: "5",
			Valid:  true,
		},
		AgentUserID: sql.NullString{
			String: "35636a572d7ec364",
			Valid:  true,
		},
		PlatformID: "shared-conversation-identifier",
		EndedAt:    sql.NullTime{Valid: true, Time: time.Date(2022, 11, 16, 4, 28, 27, 102, time.UTC)},
	}).Error)
	s.Require().NoError(s.DB().Create(&dbmodel.ChatVisitors{
		ID:         1,
		PlatformID: "123456789",
		FirstName: sql.NullString{
			String: "TestVisitor",
			Valid:  true,
		},
	}).Error)

	s.Require().NoError(s.DB().Create(&dbmodel.Chats{
		CustomerID: sql.NullString{
			String: s.customerID,
			Valid:  true,
		},
		ProfileID: sql.NullString{
			String: s.profileID,
			Valid:  true,
		},
		VisitorID: sql.NullInt32{
			Valid: true,
			Int32: 1,
		},
		ID: 17817211,
		ConversationID: sql.NullString{
			String: "6",
			Valid:  true,
		},
		AgentUserID: sql.NullString{
			String: "35636a572d7ec364",
			Valid:  true,
		},
		AudioAnalysis: sql.NullString{
			String: "{\"agent-energy\": [1], \"visitor-energy\": [0]}",
			Valid:  true,
		},
		ConversationCorrelationID: sql.NullString{Valid: true, String: "shared-conversation-identifier"},
		PlatformID:                "6",
		EndedAt:                   sql.NullTime{Valid: true, Time: time.Now()},
	}).Error)
}

func TestRetrieveClosedConversations(t *testing.T) {
	r := RetrieveClosedConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	// V3 category tree shared by the V3 cases: a parent `v3-root` with two leaf children.
	v3TemplatesResp := &momentpb.ListMomentsResponse{
		Moments: []*momentpb.Moment{
			{
				Name:         fmt.Sprintf("customers/%s/profiles/%s/moments/v3-root", r.customerID, r.profileID),
				Type:         momentpb.Moment_TOPIC,
				DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
			},
			{
				Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/v3-child-a", r.customerID, r.profileID),
				Type:                   momentpb.Moment_TOPIC,
				DetailedType:           momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
				ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/v3-root", r.customerID, r.profileID),
			},
			{
				Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/v3-child-b", r.customerID, r.profileID),
				Type:                   momentpb.Moment_TOPIC,
				DetailedType:           momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
				ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/v3-root", r.customerID, r.profileID),
			},
		},
	}

	for _, tc := range []struct {
		desc                                       string
		esReqPath                                  string
		esRespPath                                 string
		req                                        *analyticspb.RetrieveClosedConversationsRequest
		want                                       *analyticspb.RetrieveClosedConversationsResponse
		getCompositeMomentResp                     *momentpb.BatchGetMomentsResponse
		getChildMomentResp                         *momentpb.BatchGetMomentsResponse
		batchGetMomentResps                        []*momentpb.BatchGetMomentsResponse
		listMomentsResp                            *momentpb.ListMomentsResponse
		enableAdvancedDataAccessControl            bool
		resourceACL                                *auth.ResourceACL
		enableFilterConvosWithLastMomentAnnotation bool
		enableEsQueryAttendeesForAgentUserIDs      bool
		includeEmptyConvos                         bool
		enableNumHoldsField                        bool
		expectedListUsersForAnalyticsResponse      *internaluserpb.ListUsersForAnalyticsResponse
		expectedListUsersFromDynamicGroupsResponse *userpb.ListUsersResponse
	}{
		{
			desc: "BasicRequest",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/35636a572d7ec364"},
						{Name: "customers/customer_id/users/2f540f161f3a452c"},
						{Name: "customers/customer_id/users/d072a51a8359fe51"},
					},
					LanguageCodes: []string{"en-US"},
				},
				TargetFieldForTimeRange: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
				PageSize:                5,
				PageToken:               strings.Join([]string{"sort-key-for-convo_ended_at", "sort-key-for-convo_id"}, esSortKeySeparator),
			},
			esReqPath:  "RetrieveClosedConversations_BasicRequest_es_request.json",
			esRespPath: "RetrieveClosedConversations_BasicRequest_es_response.json",
			want: &analyticspb.RetrieveClosedConversationsResponse{
				TotalConversationCount: 5,
				Conversations: []*convopb.Conversation{
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/1",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						LegacyChatId:  17684332,
						AgentUserName: "1",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 19},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/2",
						Agent:         "customers/customer_id/users/2f540f161f3a452c",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 2"},
						LegacyChatId:  17684276,
						AgentUserName: "2",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 10},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/3",
						Agent:         "customers/customer_id/users/d072a51a8359fe51",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 3"},
						LegacyChatId:  17684271,
						AgentUserName: "3",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 8},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/4",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						LegacyChatId:  17684249,
						AgentUserName: "1",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 17},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/5",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						LegacyChatId:  17684248,
						AgentUserName: "1",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 9},
					},
				},
				NextPageToken: strings.Join([]string{"2022-11-16T04:18:27.409Z", "5"}, esSortKeySeparator),
			},
			enableEsQueryAttendeesForAgentUserIDs: true,
		},
		{
			desc: "IncludeEmptyConvos",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/35636a572d7ec364"},
						{Name: "customers/customer_id/users/2f540f161f3a452c"},
						{Name: "customers/customer_id/users/d072a51a8359fe51"},
					},
					LanguageCodes: []string{"en-US"},
				},
				PageSize:  5,
				PageToken: strings.Join([]string{"sort-key-for-convo_ended_at", "sort-key-for-convo_id"}, esSortKeySeparator),
			},
			includeEmptyConvos: true,
			esReqPath:          "RetrieveClosedConversations_IncludeEmptyConvos_es_request.json",
		},
		{
			desc: "MomentGroup_AggregateSubcategoryExpansion",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
				PageSize: 5,
			},
			esReqPath: "RetrieveClosedConversations_MomentGroup_AggregateSubcategoryExpansion_es_request.json",
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						{
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-1", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-1",
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_CompositeMomentData{
									CompositeMomentData: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_AND,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_OR,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-topic-cluster-subcategory-1-1", r.customerID, r.profileID),
																},
															},
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-1", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-2", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-1",
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_CompositeMomentData{
									CompositeMomentData: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_AND,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_OR,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_NOT,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/excluded-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-1", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-2",
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_CompositeMomentData{
									CompositeMomentData: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_AND,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_OR,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-1", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-2", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-2",
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_CompositeMomentData{
									CompositeMomentData: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_AND,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_OR,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-2", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Moments: []*momentpb.Moment{
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-topic-cluster-subcategory-1-1", r.customerID, r.profileID),
							Type: momentpb.Moment_TOPIC,
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-1", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-1-1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-1-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/excluded-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded-keyword-keyword-subcategory-1-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-1", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-2-1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-2-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: momentpb.MomentName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								MomentID:   "aggregate-subcategory-parent-id-1",
							}.String(),
							Type: momentpb.Moment_COMPOSITE,
							Labels: &labelspb.Labels{
								Labels: map[string]string{
									"message_filters": "agent",
								},
							},
						},
						{
							Name: momentpb.MomentName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								MomentID:   "aggregate-subcategory-parent-id-2",
							}.String(),
							Type: momentpb.Moment_COMPOSITE,
							Labels: &labelspb.Labels{
								Labels: map[string]string{
									"message_filters": "agent",
								},
							},
						},
					},
				},
			},
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						Type: momentpb.Moment_COMPOSITE,
						// No MomentData - this is an aggregate
					},
					{
						Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						Type: momentpb.Moment_COMPOSITE,
						// No MomentData - this is an aggregate
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-1", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-2", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-1", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-2", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
		},
		{
			// A parent Topic Discovery V3 category expands to its full subtree (the node itself
			// plus every descendant), producing a has_child filter over all of their template ids.
			desc: "TopicDiscoveryV3_ParentCategory",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name:         fmt.Sprintf("customers/%s/profiles/%s/moments/v3-root", r.customerID, r.profileID),
									Type:         momentpb.Moment_TOPIC,
									DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
								},
							},
						},
					},
				},
				PageSize: 5,
			},
			esReqPath:       "RetrieveClosedConversations_TopicDiscoveryV3_ParentCategory_es_request.json",
			listMomentsResp: v3TemplatesResp,
		},
		{
			// A leaf Topic Discovery V3 category has no descendants, so it expands to just itself.
			desc: "TopicDiscoveryV3_LeafCategory",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name:         fmt.Sprintf("customers/%s/profiles/%s/moments/v3-child-a", r.customerID, r.profileID),
									Type:         momentpb.Moment_TOPIC,
									DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
								},
							},
						},
					},
				},
				PageSize: 5,
			},
			esReqPath:       "RetrieveClosedConversations_TopicDiscoveryV3_LeafCategory_es_request.json",
			listMomentsResp: v3TemplatesResp,
		},
		{
			desc: "KeywordExpressions",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					KeywordsExpressions: []string{"gift card AND \"gift card\" AND \"gift-card\""},
				},
			},
			esReqPath: "RetrieveClosedConversations_KeywordExpressions_es_request.json",
		},
		{
			desc:                            "AccessControl_isNotRoot_withUserFilter_aclUsers",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:   false,
				ManagedUserIDs: []string{"35636a572d7ec364", "2f540f161f3a452c", "d072a51a8359fe51"},
			},
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/35636a572d7ec364"},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AccessControl_isNotRoot_withUserFilter_aclUsers_es_request.json",
		},
		{
			desc:                            "AccessControl_isNotRoot_withoutUserFilter_aclUsers",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:   false,
				ManagedUserIDs: []string{"35636a572d7ec364", "2f540f161f3a452c", "d072a51a8359fe51"},
			},
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AccessControl_isNotRoot_withoutUserFilter_aclUsers_es_request.json",
		},
		{
			desc:                            "AccessControl_isNotRoot_withGroupFilter_filterAclGroups",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    false,
				ManagedUserIDs:  []string{"35636a572d7ec364", "2f540f161f3a452c", "d072a51a8359fe51"},
				ManagedGroupIDs: []string{"test_group_1", "test_group_2"},
			},
			expectedListUsersForAnalyticsResponse: &internaluserpb.ListUsersForAnalyticsResponse{
				Users: []*internaluserpb.LiteUser{
					{UserId: "35636a572d7ec364"},
					{UserId: "2f540f161f3a452c"},
				},
			},
			expectedListUsersFromDynamicGroupsResponse: &userpb.ListUsersResponse{
				Users: []*userpb.User{
					{Name: "customers/customer_id/users/35636a572d7ec364"},
					{Name: "customers/customer_id/users/d072a51a8359fe51"},
				},
			},
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Groups: []*userpb.Group{
						{Name: "customers/customer_id/groups/test_group_1"},
					},
					DynamicGroups: []*userpb.Group{
						{Name: "customers/customer_id/groups/test_dynamic_group_1"},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AccessControl_isNotRoot_withGroupFilter_filterAclGroups_es_request.json",
		},
		{
			desc:                            "AccessControl_isRoot_withUserFilter_aclUsers",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:   true,
				ManagedUserIDs: []string{"35636a572d7ec364", "2f540f161f3a452c", "d072a51a8359fe51"},
			},
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/35636a572d7ec364"},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AccessControl_isRoot_withUserFilter_aclUsers_es_request.json",
		},
		{
			desc:                            "AccessControl_isRoot_withoutUserFilter_aclUsers",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:   true,
				ManagedUserIDs: []string{"35636a572d7ec364", "2f540f161f3a452c", "d072a51a8359fe51"},
			},
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AccessControl_isRoot_withoutUserFilter_aclUsers_es_request.json",
		},
		{
			desc:                            "AccessControl_isRoot_withGroupFilter_filterAclGroups",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:    true,
				ManagedUserIDs:  []string{"35636a572d7ec364", "2f540f161f3a452c", "d072a51a8359fe51"},
				ManagedGroupIDs: []string{"test_group_1", "test_group_2"},
			},
			expectedListUsersForAnalyticsResponse: &internaluserpb.ListUsersForAnalyticsResponse{
				Users: []*internaluserpb.LiteUser{
					{UserId: "35636a572d7ec364"},
					{UserId: "2f540f161f3a452c"},
				},
			},
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Groups: []*userpb.Group{
						{Name: "customers/customer_id/groups/test_group_1"},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AccessControl_isRoot_withGroupFilter_filterAclGroups_es_request.json",
		},
		{
			desc: "DurationRange",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationDurationRange: &analyticspb.DurationRange{
						MaxDuration: &durationpb.Duration{
							Seconds: 60,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_DurationRange_es_request.json",
		},
		{
			desc: "NonConsecutiveDurationBuckets",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
						{Id: "1"},
						{Id: "3"},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_NonConsecutiveDurationBuckets_es_request.json",
		},
		{
			desc: "ConsecutiveDurationBuckets",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
						{Id: "3"},
						{Id: "4"},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConsecutiveDurationBuckets_es_request.json",
		},
		{
			desc: "ConvoHandleTime",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHandleTimeInSecs: &analyticspb.FloatRange{
						LowerBound: &analyticspb.FloatRange_Bound{
							IsExclusive: true,
							Value:       10,
						},
						UpperBound: &analyticspb.FloatRange_Bound{
							Value: 100,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConvoHandleTime_es_request.json",
		},
		{
			desc: "MessageCountRange",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MessageCountRange: &analyticspb.IntRange{
						LowerBound: &analyticspb.IntRange_Bound{
							Value: 8,
						},
						UpperBound: &analyticspb.IntRange_Bound{
							Value: 10,
						},
					},
				},
			},
			esReqPath:  "RetrieveClosedConversations_MessageCountRange_es_request.json",
			esRespPath: "RetrieveClosedConversations_MessageCountRange_es_response.json",
		},
		{
			desc: "ExcludeDeactivatedUsers",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ExcludeDeactivatedUsers: true,
				},
			},
			esReqPath: "RetrieveClosedConversations_ExcludeDeactivatedUsers_es_request.json",
			expectedListUsersForAnalyticsResponse: &internaluserpb.ListUsersForAnalyticsResponse{
				Users: []*internaluserpb.LiteUser{
					{
						UserId: "35636a572d7ec364",
					},
					{
						UserId: "2f540f161f3a452c",
					},
					{
						UserId: "d072a51a8359fe51",
					},
				},
			},
		},
		{
			desc: "IncludeCrestaBot",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					IncludeCrestaBot: true,
				},
			},
			esReqPath: "RetrieveClosedConversations_IncludeCrestaBot_es_request.json",
		},
		{
			desc: "ConversationSources",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationSources: []convopb.Conversation_Source{5},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationSources_es_request.json",
		},
		{
			desc: "WorkflowAutomation",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					WorkflowAutomationConditions: []analyticspb.Attribute_WorkflowAutomationCondition{analyticspb.Attribute_AUTO_DETECTED_IGNORED, analyticspb.Attribute_AUTOMATIC_COMPLETED},
				},
			},
			esReqPath: "RetrieveClosedConversations_WorkflowAutomation_es_request.json",
		},
		{
			desc: "SummaryUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					SummaryUsages: []analyticspb.Attribute_SummaryUsage{analyticspb.Attribute_SUMMARY_USED},
				},
			},
			esReqPath: "RetrieveClosedConversations_SummaryUsed_es_request.json",
		},
		{
			desc: "SummaryNotUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					SummaryUsages: []analyticspb.Attribute_SummaryUsage{analyticspb.Attribute_SUMMARY_NOT_USED},
				},
			},
			esReqPath: "RetrieveClosedConversations_SummaryNotUsed_es_request.json",
		},
		{
			desc: "SuggestionUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					SuggestionUsages: []analyticspb.Attribute_SuggestionUsage{analyticspb.Attribute_SUGGESTION_USED},
				},
			},
			esReqPath: "RetrieveClosedConversations_SuggestionUsed_es_request.json",
		},
		{
			desc: "SuggestionNotUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					SuggestionUsages: []analyticspb.Attribute_SuggestionUsage{analyticspb.Attribute_SUGGESTION_NOT_USED},
				},
			},
			esReqPath: "RetrieveClosedConversations_SuggestionNotUsed_es_request.json",
		},
		{
			desc: "NoteTakingUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					NoteTakingUsages: []analyticspb.Attribute_NoteTakingUsage{analyticspb.Attribute_NOTE_TAKING_USED},
				},
			},
			esReqPath: "RetrieveClosedConversations_NoteTakingUsed_es_request.json",
		},
		{
			desc: "NoteTakingNoteUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					NoteTakingUsages: []analyticspb.Attribute_NoteTakingUsage{analyticspb.Attribute_NOTE_TAKING_NOT_USED},
				},
			},
			esReqPath: "RetrieveClosedConversations_NoteTakingNotUsed_es_request.json",
		},
		{
			desc: "HoldFilter",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHoldsCriterion: &analyticspb.Attribute_ConversationHoldsCriterion{
						HoldsCountRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 10,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 20,
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_HoldFilter_es_request.json",
		},
		{
			desc: "HoldCount0",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHoldsCriterion: &analyticspb.Attribute_ConversationHoldsCriterion{
						HoldsCountRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_HoldCount0_es_request.json",
		},
		{
			desc: "HoldCountGTE0",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHoldsCriterion: &analyticspb.Attribute_ConversationHoldsCriterion{
						HoldsCountRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 10,
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_HoldCountGTE0_es_request.json",
		},
		{
			desc: "HoldFilter_NumHoldsField",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHoldsCriterion: &analyticspb.Attribute_ConversationHoldsCriterion{
						HoldsCountRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 10,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 20,
							},
						},
					},
				},
			},
			esReqPath:           "RetrieveClosedConversations_HoldFilter_NumHoldsField_es_request.json",
			enableNumHoldsField: true,
		},
		{
			desc: "HoldCount0_NumHoldsField",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHoldsCriterion: &analyticspb.Attribute_ConversationHoldsCriterion{
						HoldsCountRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
						},
					},
				},
			},
			esReqPath:           "RetrieveClosedConversations_HoldCount0_NumHoldsField_es_request.json",
			enableNumHoldsField: true,
		},
		{
			desc: "HoldCountGTE0_NumHoldsField",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationHoldsCriterion: &analyticspb.Attribute_ConversationHoldsCriterion{
						HoldsCountRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 10,
							},
						},
					},
				},
			},
			esReqPath:           "RetrieveClosedConversations_HoldCountGTE0_NumHoldsField_es_request.json",
			enableNumHoldsField: true,
		},
		{
			desc: "SentimentMomentType",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							MomentType: momentpb.Moment_SENTIMENT,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_SentimentMomentType_es_request.json",
		},
		{
			desc: "ConversationReasonMomentType",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							MomentType: momentpb.Moment_CONVERSATION_REASON,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationReasonMomentType_es_request.json",
		},
		{
			desc: "ConversationMetadata_No_Metadata_Values",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_No_Metadata_Values_es_request.json",
			enableFilterConvosWithLastMomentAnnotation: true,
		},
		{
			desc: "ConversationOutcome_No_Values",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_OUTCOME,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_No_Metadata_Values_es_request.json",
		},
		{
			desc: "ConversationOutcome_String_Values",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_OUTCOME,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							ExcludedMoments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_OUTCOME,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
								{
									OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
										BinaryOutcomeValueEnum: enumpb.BinaryOutcomeValueEnum_NEGATIVE,
										Value: &momentpb.ConversationOutcomeValue{
											Value: &momentpb.ConversationOutcomeValue_StringValue{
												StringValue: "resolved",
											},
										},
									},
								},
								{
									OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
										BinaryOutcomeValueEnum: enumpb.BinaryOutcomeValueEnum_POSITIVE,
										Value: &momentpb.ConversationOutcomeValue{
											Value: &momentpb.ConversationOutcomeValue_StringValue{
												StringValue: "escalated",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationOutcome_StringValues_es_request.json",
		},
		{
			desc: "ConversationMetadata_Excluded",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_Excluded_es_request.json",
			enableFilterConvosWithLastMomentAnnotation: true,
		},
		{
			desc: "ConversationMetadata_NumericBins",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									NumericBin: &metadata.NumericBin{
										FromValue: 50,
										ToValue:   100,
									},
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveClosedConversations_ConversationMetadata_NumericBins_es_request.json",
			esRespPath: "RetrieveClosedConversations_ConversationMetadata_NumericBins_es_response.json",
			want: &analyticspb.RetrieveClosedConversationsResponse{
				TotalConversationCount: 5,
				Conversations: []*convopb.Conversation{
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/2",
						Agent:         "customers/customer_id/users/2f540f161f3a452c",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 2"},
						LegacyChatId:  17684276,
						AgentUserName: "2",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 10},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
		},
		{
			desc: "ConversationMetadata_StringValues",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{StringValue: "California"},
									},
								},
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{StringValue: "Ohio"},
									},
								},
							},
						},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
			esReqPath:  "RetrieveClosedConversations_ConversationMetadata_StringValues_es_request.json",
			esRespPath: "RetrieveClosedConversations_ConversationMetadata_StringValues_es_response.json",
			want: &analyticspb.RetrieveClosedConversationsResponse{
				TotalConversationCount: 5,
				Conversations: []*convopb.Conversation{
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/2",
						Agent:         "customers/customer_id/users/2f540f161f3a452c",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 2"},
						LegacyChatId:  17684276,
						AgentUserName: "2",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 10},
					},
				},
			},
		},
		{
			desc: "ConversationMetadata_IncludeAndExcludeSameMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							ExcludedMoments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{StringValue: "value-1"},
									},
								},
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{StringValue: "value-2"},
									},
								},
							},
						},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
			esReqPath:  "RetrieveClosedConversations_ConversationMetadata_IncludeAndExcludeSameMoment_es_request.json",
			esRespPath: "RetrieveClosedConversations_ConversationMetadata_StringValues_es_response.json",
		},
		{
			desc: "ConversationMetadata_StringValues_WildcardMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/*",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{StringValue: "California"},
									},
								},
							},
						},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_StringValues_WildcardMoment_es_request.json",
		},
		{
			desc: "ConversationMetadata_StringValues_WildcardMoment_ExactMatch",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/*",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_StringValue{StringValue: "\"California\""},
									},
								},
							},
						},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_StringValues_WildcardMoment_ExactMatch_es_request.json",
		},
		{
			desc: "ConversationMetadata_NumericValues",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_NumberValue{NumberValue: 20},
									},
								},
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_NumberValue{NumberValue: 40},
									},
								},
							},
						},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_NumericValues_es_request.json",
		},
		{
			desc: "ConversationMetadata_BooleanValues",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_CONVERSATION_METADATA,
									Name: "customers/customer_id/profiles/profile_id/moments/moment-id-1",
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_BooleanValue{BooleanValue: true},
									},
								},
								{
									MetadataValue: &metadata.MetadataValue{
										Value: &metadata.MetadataValue_BooleanValue{BooleanValue: false},
									},
								},
							},
						},
					},
				},
			},
			enableFilterConvosWithLastMomentAnnotation: true,
			esReqPath: "RetrieveClosedConversations_ConversationMetadata_BooleanValues_es_request.json",
		},
		{
			desc: "VoicemailMomentIncluded",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_VOICE_MAIL,
									Name: "customers/customer_id/profiles/profile_id/moments/voice-mail-moment-id",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_VoicemailMomentIncluded_es_request.json",
		},
		{
			desc: "VoicemailMomentExcluded",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_VOICE_MAIL,
									Name: "customers/customer_id/profiles/profile_id/moments/excluded-voicemail-moment-id",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_VoicemailMomentExcluded_es_request.json",
		},
		{
			desc: "EmotionMomentType",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							MomentType: momentpb.Moment_EMOTION,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_EmotionMomentType_es_request.json",
		},
		{
			desc: "CompositeMomentsNotFromInsights",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_COMPOSITE,
									AppTypes: []momentpb.Moment_AppType{
										momentpb.Moment_APP_TYPE_COACHBUILDER,
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_CompositeMomentsNotFromInsights_es_request.json",
		},
		{
			desc: "AdherenceForMomentGroup_MessageMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_KEYWORD,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/2",
									Type: momentpb.Moment_KEYWORD,
								},
							},
							Adherence: enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AdherenceForMomentGroup_MessageMoment_es_request.json",
		},
		{
			desc: "AdherenceForMoments_MessageMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_KEYWORD,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/2",
									Type: momentpb.Moment_KEYWORD,
								},
							},
							AdherenceTypesForMoments: []*analyticspb.MomentGroup_MomentAdherenceTypeConfig{
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
									},
								},
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AdherenceForMoments_MessageMoment_es_request.json",
		},
		{
			desc: "AdherenceForBehaviors_MessageMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Behaviors: []*behaviorpb.Behavior{
								{
									Name: "customers/customer_id/profiles/profile_id/behaviors/1",
								},
								{
									Name: "customers/customer_id/profiles/profile_id/behaviors/2",
								},
							},
							AdherenceTypesForMoments: []*analyticspb.MomentGroup_MomentAdherenceTypeConfig{
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
									},
								},
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AdherenceForBehaviors_MessageMoment_es_request.json",
		},
		{
			desc: "AdherenceForAllAndBehaviors_MessageMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Behaviors: []*behaviorpb.Behavior{
								{
									Name: "customers/customer_id/profiles/profile_id/behaviors/1",
								},
								{
									Name: "customers/customer_id/profiles/profile_id/behaviors/2",
								},
							},
							AdherenceTypesForMoments: []*analyticspb.MomentGroup_MomentAdherenceTypeConfig{
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
									},
								},
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
									},
								},
							},
							BooleanLogicOperator: analyticspb.MomentGroup_BOOLEAN_LOGIC_OPERATOR_AND,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AdherenceForAllAndBehaviors_MessageMoment_es_request.json",
		},
		{
			desc: "AdherenceForMomentGroup_ConvoMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_SENTIMENT,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/2",
									Type: momentpb.Moment_SENTIMENT,
								},
							},
							Adherence: enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AdherenceForMomentGroup_ConvoMoment_es_request.json",
		},
		{
			desc: "AdherenceForMoments_ConvoMoment",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/1",
									Type: momentpb.Moment_SENTIMENT,
								},
								{
									Name: "customers/customer_id/profiles/profile_id/moments/2",
									Type: momentpb.Moment_SENTIMENT,
								},
							},
							AdherenceTypesForMoments: []*analyticspb.MomentGroup_MomentAdherenceTypeConfig{
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
									},
								},
								{
									AdherenceTypes: []enumpb.AdherenceType{
										enumpb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
										enumpb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_AdherenceForMoments_ConvoMoment_es_request.json",
		},
		{
			desc: "Actions",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ActionGroups: []*analyticspb.ActionGroup{
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/1",
									},
								},
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/2",
									},
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_Actions_es_request.json",
		},
		{
			desc: "ActionsFromDeprecatedActionsField",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Actions: []*actionpb.Action{
						{
							Name: "customers/customer_id/profiles/profile_id/actions/1",
						},
						{
							Name: "customers/customer_id/profiles/profile_id/actions/2",
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_Actions_es_request.json",
		},
		{
			desc: "ActionType",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ActionGroups: []*analyticspb.ActionGroup{
						{
							ActionType: actionpb.Action_GUIDED_WORKFLOW_HINT,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ActionType_es_request.json",
		},
		{
			desc: "HintFollowedNotFollowed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ActionGroups: []*analyticspb.ActionGroup{
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/kb_hint_1",
										Type: actionpb.Action_KB_HINT,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_ENGAGED,
								},
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/kb_hint_2",
										Type: actionpb.Action_KB_HINT,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_NOT_ENGAGED,
								},
							},
						},
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/behavior_hint_1",
										Type: actionpb.Action_HINT,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_ENGAGED,
								},
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/behavior_hint_2",
										Type: actionpb.Action_HINT,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_NOT_ENGAGED,
								},
							},
						},
						{
							Actions: []*analyticspb.ActionGroup_ActionRecord{
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/checklist_hint_1",
										Type: actionpb.Action_CHECKLIST,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_ENGAGED,
								},
								{
									Action: &actionpb.Action{
										Name: "customers/customer_id/profiles/profile_id/actions/checklist_hint_2",
										Type: actionpb.Action_CHECKLIST,
									},
									Engagement: analyticspb.ActionGroup_ActionRecord_NOT_ENGAGED,
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_HintFollowedNotFollowed_es_request.json",
		},
		{
			desc: "HasComments",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Comments: analyticspb.Attribute_HAS_COMMENTS,
				},
			},
			esReqPath: "RetrieveClosedConversations_HasComments_es_request.json",
		},
		{
			desc: "HasEmailAttachments",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					EmailAttachments: analyticspb.Attribute_HAS_EMAIL_ATTACHMENTS,
				},
			},
			esReqPath: "RetrieveClosedConversations_HasEmailAttachments_es_request.json",
		},
		{
			desc: "HasNoEmailAttachments",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					EmailAttachments: analyticspb.Attribute_HAS_NO_EMAIL_ATTACHMENTS,
				},
			},
			esReqPath: "RetrieveClosedConversations_HasNoEmailAttachments_es_request.json",
		},
		{
			desc: "LiveAssist",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					LiveAssist: analyticspb.Attribute_LIVE_ASSIST_WITHOUT_RAISE_HAND,
				},
			},
			esReqPath: "RetrieveClosedConversations_LiveAssist_es_request.json",
		},
		{
			desc: "PlatformConversationID",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Conversations: []*convopb.Conversation{
						{
							PlatformInfo: &convopb.Conversation_PlatformInfo{
								PlatformConversationId: "shared-conversation-identifier",
							},
						},
					},
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/use-case-id"},
				},
			},
			esReqPath:  "RetrieveClosedConversations_PlatformConversationID_es_request.json",
			esRespPath: "RetrieveClosedConversations_PlatformConversationID_es_response.json",
			want: &analyticspb.RetrieveClosedConversationsResponse{
				Conversations: []*convopb.Conversation{
					{
						Name:                      "customers/customer_id/profiles/profile_id/conversations/6",
						ConversationCorrelationId: "shared-conversation-identifier",
						Agent:                     "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:                 &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						AgentUserName:             "1",
						LegacyChatId:              17817211,
						AudioAnalysis: &convopb.AudioAnalysis{
							AgentAudioEnergy:   []float32{1.0},
							VisitorAudioEnergy: []float32{0.0},
						},
						State: convopb.Conversation_CLOSED,
						Visitor: &convopb.Visitor{
							PlatformVisitorId: "123456789",
							DisplayName:       "TestVisitor",
						},
						ExtendedInfo: &convopb.Conversation_ExtendedInfo{MessageCount: 99},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/5",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						AgentUserName: "1",
						LegacyChatId:  17684248,
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 99},
					},
				},
				TotalConversationCount: 2,
			},
		},
		{
			desc: "PlatformConversationIDWithWildcard",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Conversations: []*convopb.Conversation{
						{
							PlatformInfo: &convopb.Conversation_PlatformInfo{
								PlatformConversationId: "shared-conversation-identifier*",
							},
						},
					},
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/use-case-id"},
				},
			},
			esReqPath:  "RetrieveClosedConversations_PlatformConversationIDWithWildcard_es_request.json",
			esRespPath: "RetrieveClosedConversations_PlatformConversationID_es_response.json",
			want: &analyticspb.RetrieveClosedConversationsResponse{
				Conversations: []*convopb.Conversation{
					{
						Name:                      "customers/customer_id/profiles/profile_id/conversations/6",
						ConversationCorrelationId: "shared-conversation-identifier",
						Agent:                     "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:                 &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						AgentUserName:             "1",
						LegacyChatId:              17817211,
						AudioAnalysis: &convopb.AudioAnalysis{
							AgentAudioEnergy:   []float32{1.0},
							VisitorAudioEnergy: []float32{0.0},
						},
						State: convopb.Conversation_CLOSED,
						Visitor: &convopb.Visitor{
							PlatformVisitorId: "123456789",
							DisplayName:       "TestVisitor",
						},
						ExtendedInfo: &convopb.Conversation_ExtendedInfo{MessageCount: 99},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/5",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						AgentUserName: "1",
						LegacyChatId:  17684248,
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 99},
					},
				},
				TotalConversationCount: 2,
			},
		},
		{
			desc: "ConversationIdentifiers",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationIdentifiers: []*analyticspb.Attribute_ConversationIdentifier{
						{
							Name: "customers/customer_id/profiles/profile_id/conversations/convo-id",
						},
					},
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/use-case-id"},
				},
			},
			esReqPath: "RetrieveClosedConversations_ConversationIdentifiers_es_request.json",
		},
		{
			desc: "Tags",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Tags: []*tagpb.Tag{
						{
							Name: tagpb.TagName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								TagID:      "tag-id1",
							}.String(),
						},
						{
							Name: tagpb.TagName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								TagID:      "tag-id2",
							}.String(),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_Tags_es_request.json",
		},
		{
			desc: "ScreenRecordings",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ScreenRecording: analyticspb.Attribute_HAS_SCREEN_RECORDING,
				},
			},
			esReqPath: "RetrieveClosedConversations_ScreenRecordings_es_request.json",
		},
		{
			desc: "NoScreenRecordings",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ScreenRecording: analyticspb.Attribute_HAS_NO_SCREEN_RECORDING,
				},
			},
			esReqPath: "RetrieveClosedConversations_NoScreenRecordings_es_request.json",
		},
		{
			desc: "Scorecard",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					ScorecardStatus: analyticspb.Attribute_MANUALLY_SUBMITTED,
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_Scorecard_es_request.json",
		},
		{
			desc: "ScorecardStatusFilters",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					ScorecardStatusFilters: []*analyticspb.Attribute_ScorecardStatusFilter{
						{
							ScorecardStatuses: []analyticspb.Attribute_ScorecardStatus{analyticspb.Attribute_AUTO},
						},
						{
							CalibrationScorecardStatuses: []analyticspb.Attribute_CalibrationScorecardStatus{analyticspb.Attribute_CALIBRATION_SCORECARD_STATUS_SUBMITTED},
						},
						{
							AppealScorecardStatuses: []analyticspb.Attribute_AppealScorecardStatus{analyticspb.Attribute_APPEAL_SCORECARD_STATUS_RESOLVED},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ScorecardStatusFilters_es_request.json",
		},
		{
			desc: "PublishedScorecardStatusPublished",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					ScorecardStatusFilters: []*analyticspb.Attribute_ScorecardStatusFilter{
						{
							PublishedScorecardStatuses: []analyticspb.Attribute_PublishedScorecardStatus{
								analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_PUBLISHED,
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_PublishedScorecardStatusPublished_es_request.json",
		},
		{
			desc: "PublishedScorecardStatusNotPublished",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					ScorecardStatusFilters: []*analyticspb.Attribute_ScorecardStatusFilter{
						{
							PublishedScorecardStatuses: []analyticspb.Attribute_PublishedScorecardStatus{
								analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_NOT_PUBLISHED,
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_PublishedScorecardStatusNotPublished_es_request.json",
		},
		{
			desc: "PublishedScorecardStatusBoth",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					ScorecardStatusFilters: []*analyticspb.Attribute_ScorecardStatusFilter{
						{
							PublishedScorecardStatuses: []analyticspb.Attribute_PublishedScorecardStatus{
								analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_PUBLISHED,
								analyticspb.Attribute_PUBLISHED_SCORECARD_STATUS_NOT_PUBLISHED,
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_PublishedScorecardStatusBoth_es_request.json",
		},
		{
			desc: "Criterion",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ScorecardFilters: []*analyticspb.ScorecardFilter{
						{
							ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
							CriterionFilters: []*analyticspb.ScorecardCriterionFilter{
								{
									CriterionId: "criterion_id1",
									Scores: []*analyticspb.CriterionScore{
										{
											NumericValue: 1,
										},
										{
											NumericValue: 2,
										},
									},
								},
								{
									CriterionId: "criterion_id2",
									Scores: []*analyticspb.CriterionScore{
										{
											NotApplicable: true,
										},
									},
								},
							},
						},
						{
							ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b14@revision_id",
							CriterionFilters: []*analyticspb.ScorecardCriterionFilter{
								{
									CriterionId: "criterion_id3",
									Scores: []*analyticspb.CriterionScore{
										{
											NumericValue: 3,
										},
										{
											NotApplicable: true,
										},
									},
								},
								{
									CriterionId: "criterion_id4",
								},
							},
						},
						{
							ScorecardTemplateName: "customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b15@revision_id",
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_Criterion_es_request.json",
		},
		{
			desc: "MultipleScorecardStatusesAndLiveAssists",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					ScorecardStatuses: []analyticspb.Attribute_ScorecardStatus{analyticspb.Attribute_MANUALLY_SUBMITTED, analyticspb.Attribute_DRAFT},
					LiveAssists:       []analyticspb.Attribute_LiveAssist{analyticspb.Attribute_RAISE_HAND_UNANSWERED, analyticspb.Attribute_RAISE_HAND_ANSWERED, analyticspb.Attribute_LIVE_ASSIST_WITHOUT_RAISE_HAND},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_MultipleScorecardStatusesAndLiveAssists_es_request.json",
		},
		{
			desc: "ManagerTransfer",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					LiveAssists: []analyticspb.Attribute_LiveAssist{analyticspb.Attribute_HAS_MANAGER_TRANSFER},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ManagerTransfer_es_request.json",
		},
		{
			desc: "NoManagerTransfer",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					LiveAssists: []analyticspb.Attribute_LiveAssist{analyticspb.Attribute_HAS_NO_MANAGER_TRANSFER},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_NoManagerTransfer_es_request.json",
		},
		{
			desc: "ManagerTransferWithRaiseHandAssists",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					LiveAssists: []analyticspb.Attribute_LiveAssist{analyticspb.Attribute_RAISE_HAND_ANSWERED, analyticspb.Attribute_HAS_MANAGER_TRANSFER},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ManagerTransferWithRaiseHandAssists_es_request.json",
		},
		{
			desc: "NoManagerTransferWithRaiseHandAssists",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					LiveAssists: []analyticspb.Attribute_LiveAssist{analyticspb.Attribute_RAISE_HAND_ANSWERED, analyticspb.Attribute_HAS_NO_MANAGER_TRANSFER},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_NoManagerTransferWithRaiseHandAssists_es_request.json",
		},
		{
			desc: "GWKBUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					GuidedWorkflows: []*gwpb.GuidedWorkflow{
						{
							Name: "customers/customer_id/profiles/profile_id/guidedWorkflows/gw_id1",
						},
						{
							Name: "customers/customer_id/profiles/profile_id/guidedWorkflows/gw_id2",
						},
					},
					KbDocuments: []*kbpb.Document{
						{
							Name: "customers/customer_id/profiles/profile_id/knowledgeBases/default/docs/kb_article_id1",
						},
						{
							Name: "customers/customer_id/profiles/profile_id/knowledgeBases/default/docs/kb_article_id2",
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_GWKBUsed_es_request.json",
		},
		{
			desc: "LanguageTransfer",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					LanguageTransferEvents: []*analyticspb.Attribute_LanguageTransferEvent{
						{
							OldLanguageCode: "es-MX",
							NewLanguageCode: "en-US",
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_LanguageTransfer_es_request.json",
		},
		{
			desc: "RandomConversationOrder",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					Users: []*userpb.User{
						{Name: "customers/customer_id/users/35636a572d7ec364"},
						{Name: "customers/customer_id/users/2f540f161f3a452c"},
						{Name: "customers/customer_id/users/d072a51a8359fe51"},
					},
					LanguageCodes: []string{"en-US"},
				},
				PageSize:                      5,
				PageToken:                     strings.Join([]string{"0.5555", "sort-key-for-convo_ended_at"}, esSortKeySeparator),
				EnableRandomConversationOrder: true,
				SeedForRandomSort:             12345,
			},
			esReqPath:  "RetrieveClosedConversations_RandomConversationOrder_es_request.json",
			esRespPath: "RetrieveClosedConversations_RandomConversationOrder_es_response.json",
			want: &analyticspb.RetrieveClosedConversationsResponse{
				TotalConversationCount: 5,
				Conversations: []*convopb.Conversation{
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/1",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						LegacyChatId:  17684332,
						AgentUserName: "1",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 19},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/2",
						Agent:         "customers/customer_id/users/2f540f161f3a452c",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 2"},
						LegacyChatId:  17684276,
						AgentUserName: "2",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 10},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/3",
						Agent:         "customers/customer_id/users/d072a51a8359fe51",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 3"},
						LegacyChatId:  17684271,
						AgentUserName: "3",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 8},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/4",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						LegacyChatId:  17684249,
						AgentUserName: "1",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 17},
					},
					{
						Name:          "customers/customer_id/profiles/profile_id/conversations/5",
						Agent:         "customers/customer_id/users/35636a572d7ec364",
						AgentInfo:     &convopb.ConversationMessage_SpeakerInfo{DisplayName: "Fake Agent Name 1"},
						LegacyChatId:  17684248,
						AgentUserName: "1",
						State:         convopb.Conversation_CLOSED,
						ExtendedInfo:  &convopb.Conversation_ExtendedInfo{MessageCount: 9},
					},
				},
				NextPageToken: "0.54321###2022-11-16T04:28:27.102Z",
			},
		},
		{
			desc: "CalibratedScorecardStatus",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardTemplateNames: []string{
						"customers/customer_id/profiles/profile_id/scorecardTemplates/dc08eb5f-2e39-4de3-ba1f-a26716517b13@revision_id",
					},
					CalibrationStatuses: []analyticspb.Attribute_ScorecardStatus{
						analyticspb.Attribute_DRAFT,
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_CalibratedScorecardStatus_es_request.json",
		},
		{
			desc: "ScorecardSubmitterUserNames",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					ScorecardSubmitterUserNames: []string{
						"customers/customer1/users/user1", "customers/customer1/users/user2",
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_ScorecardSubmitterUserNames_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AgentInitiatedUnspecified",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_{
									AgentInitiated: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_STATUS_UNSPECIFIED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AgentInitiatedUnspecified_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AgentInitiatedViewed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_{
									AgentInitiated: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_STATUS_VIEWED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AgentInitiatedViewed_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AgentInitiatedUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_{
									AgentInitiated: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_STATUS_USED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AgentInitiatedUsed_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AutoDetectedViewed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_{
									AutoDetected: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_VIEWED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AutoDetectedViewed_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AutoDetectedUsed",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_{
									AutoDetected: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_USED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AutoDetectedUsed_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AutoDetectedIgnored",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_{
									AutoDetected: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_IGNORED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AutoDetectedIgnored_es_request.json",
		},
		{
			desc: "TransferInfo_Consultation",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					TransferInfoFilter: &analyticspb.Attribute_TransferInfoFilter{
						ParticipantInteractionTypes: []analyticspb.Attribute_TransferInfoFilter_ParticipantInteractionType{
							analyticspb.Attribute_TransferInfoFilter_CONSULTATION,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_TransferInfo_Consultation_es_request.json",
		},
		{
			desc: "TransferInfo_WarmTransferAndColdTransfer",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					TransferInfoFilter: &analyticspb.Attribute_TransferInfoFilter{
						UmbrellaConversationTransferTypes: []analyticspb.Attribute_TransferInfoFilter_UmbrellaConversationTransferType{
							analyticspb.Attribute_TransferInfoFilter_WARM_TRANSFER,
							analyticspb.Attribute_TransferInfoFilter_COLD_TRANSFER,
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_TransferInfo_WarmTransferAndColdTransfer_es_request.json",
		},
		{
			desc: "KnowledgeAssist_AutoDetectedStatusUnspecified",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_{
									AutoDetected: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected{},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_AutoDetectedStatusUnspecified_es_request.json",
		},
		{
			desc: "KnowledgeAssist_MultipleMoments",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					KnowledgeMomentFilter: &analyticspb.Attribute_KnowledgeMomentFilter{
						MatchType: analyticspb.MatchType_MATCH_ALL,
						KnowledgeMoments: []*analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment{
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AgentInitiated_{},
							},
							{
								Type: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_{
									AutoDetected: &analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected{
										Status: analyticspb.Attribute_KnowledgeMomentFilter_KnowledgeMoment_AutoDetected_STATUS_IGNORED,
									},
								},
							},
						},
					},
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_KnowledgeAssist_MultipleMoments_es_request.json",
		},
		{
			desc: "PlatformVisitorID",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationPlatformVisitorId: []string{"123456789"},
				},
			},
			esReqPath: "RetrieveClosedConversations_PlatformVisitorID_es_request.json",
		},
		{
			desc: "VirtualAgentRevisionIds",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					VirtualAgentRevisionIds: []string{"12", "13"},
				},
			},
			esReqPath: "RetrieveClosedConversations_VirtualAgentRevisionIds_es_request.json",
		},
		{
			desc: "IngestedByJobIds",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					IngestedByJobIds: []string{"job-1", "job-2"},
				},
			},
			esReqPath: "RetrieveClosedConversations_IngestedByJobIds_es_request.json",
		},
		{
			desc: "DisplayName_ExactMatch",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationDisplayNames: []string{`"Booking Confirmation"`},
				},
			},
			esReqPath: "RetrieveClosedConversations_DisplayName_ExactMatch_es_request.json",
		},
		{
			desc: "DisplayName_PartialMatch",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationDisplayNames: []string{"reservation inquiry"},
				},
			},
			esReqPath: "RetrieveClosedConversations_DisplayName_PartialMatch_es_request.json",
		},
		{
			desc: "DisplayName_OrExpression",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					ConversationDisplayNames: []string{`reservation OR "Booking Confirmation"`},
				},
			},
			esReqPath: "RetrieveClosedConversations_DisplayName_OrExpression_es_request.json",
		},
		{
			desc: "LlmAnnotation_StringValue",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_LLM_ANNOTATION,
									Name: "customers/customer_id/profiles/profile_id/moments/llm-annotation-moment-id",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_LlmAnnotation_StringValue_es_request.json",
		},
		{
			desc: "LlmAnnotation_NumericValue",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_LLM_ANNOTATION,
									Name: "customers/customer_id/profiles/profile_id/moments/llm-annotation-numeric-moment-id",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_LlmAnnotation_NumericValue_es_request.json",
		},
		{
			desc: "LlmAnnotation_BooleanValue",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{
						{
							StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
							EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
						},
					},
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Type: momentpb.Moment_LLM_ANNOTATION,
									Name: "customers/customer_id/profiles/profile_id/moments/llm-annotation-boolean-moment-id",
								},
							},
						},
					},
				},
			},
			esReqPath: "RetrieveClosedConversations_LlmAnnotation_BooleanValue_es_request.json",
		},
		{
			desc: "TruncatedTimeRanges_basic",
			req: &analyticspb.RetrieveClosedConversationsRequest{
				Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
				Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
				FilterByAttribute: &analyticspb.Attribute{
					TimeRanges: []*analyticspb.TimeRange{{
						StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
					}},
					TruncatedTimeRanges: []*analyticspb.Attribute_TruncatedTimeRange{
						{
							Start: &analyticspb.Attribute_TruncatedTimeRange_TruncatedTime{
								DayOfWeek: &analyticspb.Attribute_TruncatedTimeRange_TruncatedDayOfWeek{DayOfWeek: 1},
								TimeOfDay: &analyticspb.Attribute_TruncatedTimeRange_TruncatedTimeOfDay{Hour: 9, Minute: 0},
							},
							End: &analyticspb.Attribute_TruncatedTimeRange_TruncatedTime{
								DayOfWeek: &analyticspb.Attribute_TruncatedTimeRange_TruncatedDayOfWeek{DayOfWeek: 5},
								TimeOfDay: &analyticspb.Attribute_TruncatedTimeRange_TruncatedTimeOfDay{Hour: 17, Minute: 0},
							},
						},
					},
				},
				PageSize: 5,
			},
			esReqPath:  "RetrieveClosedConversations_TruncatedTimeRanges_basic_es_request.json",
			esRespPath: "RetrieveClosedConversations_TruncatedTimeRanges_basic_es_response.json",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			switch {
			case len(tc.batchGetMomentResps) > 0:
				calls := make([]*gomock.Call, 0, len(tc.batchGetMomentResps)+1)
				if tc.listMomentsResp != nil {
					calls = append(calls, r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil))
				}
				for _, resp := range tc.batchGetMomentResps {
					calls = append(calls, r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(resp, nil))
				}
				gomock.InOrder(calls...)
			case tc.getCompositeMomentResp != nil && tc.getChildMomentResp != nil:
				gomock.InOrder(
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil),
					r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getChildMomentResp, nil))
			case tc.getCompositeMomentResp != nil:
				r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.getCompositeMomentResp, nil)
			case tc.listMomentsResp != nil:
				// V3 category expansion fetches the template tree via a single ListMoments call.
				r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil)
			}

			cConfig := configpb.CustomerConfig{}
			r.Require().NoError(protojson.Unmarshal([]byte(fmt.Sprintf(`{
					"customer": "customers/customer_id",
					"profileConfigs": [
						{
							"name": "customers/customer_id/profiles/profile_id",
							"features": {
								"insights": {
									"matchConversationsWithLatestMomentAnnotation": %v,
									"includeEmptyConversationInElasticsearchQuery": %v,
									"enableNumHoldsFieldForHoldCountFilter": %v
								}
							}
						}
					],
					"authConfig": {
						"enableAdvancedDataAccessControl": %v
					}}
				`, tc.enableFilterConvosWithLastMomentAnnotation, tc.includeEmptyConvos, tc.enableNumHoldsField, tc.enableAdvancedDataAccessControl)), &cConfig))
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(testCustomerConfig.ListProfiles()[0], nil)

			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}

			if tc.req.FilterByAttribute.Groups != nil || tc.req.FilterByAttribute.ExcludeDeactivatedUsers {
				r.mockInternalUserClient.EXPECT().
					ListUsersForAnalytics(gomock.Any(), gomock.Any()).Times(1).
					Return(tc.expectedListUsersForAnalyticsResponse, nil)
			}

			if tc.enableEsQueryAttendeesForAgentUserIDs {
				err := envflag.Set("ENABLE_ES_QUERY_ATTENDEES_FOR_AGENT_USER_IDS", "true")
				r.Require().NoError(err)
			}

			if tc.expectedListUsersFromDynamicGroupsResponse != nil {
				err := envflag.Set("ENABLE_DYNAMIC_GROUPS_ACL", "true")
				r.Require().NoError(err)
				r.mockUserClient.EXPECT().
					ListUsers(gomock.Any(), gomock.Any()).Times(1).
					Return(tc.expectedListUsersFromDynamicGroupsResponse, nil)
			}

			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))

			if tc.esRespPath != "" {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			} else {
				r.mockESServer.Resps = append(r.mockESServer.Resps, emptyESHitsResponse)
			}
			resp, err := r.Client.RetrieveClosedConversations(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform(),
					protocmp.IgnoreFields(&convopb.Conversation{}, "create_time", "start_time", "end_time", "platform_info", "raise_hand_state", "audio_persistence_state", "multi_language_mode", "audio_retention_info")); diff != "" {
					t.Fatalf("RetrieveClosedConversations return unexpected response with diff (-want +got):\n %v", diff)
				}
			}

			if tc.enableEsQueryAttendeesForAgentUserIDs {
				err := envflag.Set("ENABLE_ES_QUERY_ATTENDEES_FOR_AGENT_USER_IDS", "false")
				r.Require().NoError(err)
			}
		})
	}
}

func TestRetrieveClosedConversations_EmptyMomentName(t *testing.T) {
	r := RetrieveClosedConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	// Given
	cConfig := configpb.CustomerConfig{}
	r.Require().NoError(protojson.Unmarshal([]byte(`{
		"customer": "customers/customer_id",
		"profileConfigs": [
			{
				"name": "customers/customer_id/profiles/profile_id",
				"features": {
					"insights": {}
				}
			}
		],
		"authConfig": {}
	}`), &cConfig))
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).Times(1).
		Return(testCustomerConfig, nil)
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(testCustomerConfig.ListProfiles()[0], nil)

	req := &analyticspb.RetrieveClosedConversationsRequest{
		Parent:   fmt.Sprintf("customers/%s/profiles/%s", r.customerID, r.profileID),
		Metadata: &analyticspb.Metadata{TimeZoneId: "Pacific/Honolulu"},
		FilterByAttribute: &analyticspb.Attribute{
			TimeRanges: []*analyticspb.TimeRange{
				{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
			},
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{
							Name: "",
						},
					},
				},
			},
		},
	}

	// When
	_, err := r.Client.RetrieveClosedConversations(r.ctx, req)

	// Then
	r.Require().Error(err)
	r.Equal(codes.InvalidArgument, status.Code(err), "expected INVALID_ARGUMENT error for empty moment name")
	r.Contains(status.Convert(err).Message(), "moment")
}

type ClosedConversationsWorkflowAutomationSuite struct {
	suite.Suite
	previousDeprecatePostgresUserIDQuery bool
}

func TestClosedConversationsWorkflowAutomation(t *testing.T) {
	suite.Run(t, new(ClosedConversationsWorkflowAutomationSuite))
}

func (s *ClosedConversationsWorkflowAutomationSuite) SetupSuite() {
	s.previousDeprecatePostgresUserIDQuery = *enableDeprecatePostgresUserIDQuery
	s.Require().NoError(envflag.Set("ENABLE_DEPRECATE_POSTGRES_USER_ID_QUERY", "true"))
}

func (s *ClosedConversationsWorkflowAutomationSuite) TearDownSuite() {
	s.Require().NoError(envflag.Set("ENABLE_DEPRECATE_POSTGRES_USER_ID_QUERY", strconv.FormatBool(s.previousDeprecatePostgresUserIDQuery)))
}

func (s *ClosedConversationsWorkflowAutomationSuite) TestConversationIdentifiers() {
	for _, testCase := range []struct {
		name        string
		filter      *analyticspb.Attribute
		condition   analyticspb.Attribute_WorkflowAutomationCondition
		wantIDQuery string
		wantCode    codes.Code
	}{
		{
			name: "ConversationName",
			filter: &analyticspb.Attribute{Conversations: []*convopb.Conversation{{
				Name: "customers/customer/profiles/profile/conversations/conversation",
			}}},
			condition:   analyticspb.Attribute_AUTOMATIC_FAILED,
			wantIDQuery: `{"terms":{"conversation_id.keyword":["conversation"]}}`,
		},
		{
			name: "PlatformIdentifier",
			filter: &analyticspb.Attribute{ConversationIdentifiers: []*analyticspb.Attribute_ConversationIdentifier{{
				PlatformConversationId: "platform-id",
			}}},
			condition:   analyticspb.Attribute_AUTO_DETECTED_COMPLETED,
			wantIDQuery: `{"bool":{"should":[{"terms":{"platform_conversation_id.keyword":["platform-id"]}},{"terms":{"conversation_correlation_id.keyword":["platform-id"]}}]}}`,
		},
		{
			name: "InvalidConditionWithConversationName",
			filter: &analyticspb.Attribute{Conversations: []*convopb.Conversation{{
				Name: "customers/customer/profiles/profile/conversations/conversation",
			}}},
			condition: 999,
			wantCode:  codes.InvalidArgument,
		},
		{
			name: "UnspecifiedConditionWithPlatformIdentifier",
			filter: &analyticspb.Attribute{ConversationIdentifiers: []*analyticspb.Attribute_ConversationIdentifier{{
				PlatformConversationId: "platform-id",
			}}},
			condition: analyticspb.Attribute_WORKFLOW_AUTOMATION_CONDITION_UNSPECIFIED,
			wantCode:  codes.InvalidArgument,
		},
	} {
		s.Run(testCase.name, func() {
			// Given
			testCase.filter.TimeRanges = []*analyticspb.TimeRange{{
				StartTimestamp: timestamppb.New(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)),
				EndTimestamp:   timestamppb.New(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)),
			}}
			testCase.filter.WorkflowAutomationConditions = []analyticspb.Attribute_WorkflowAutomationCondition{testCase.condition}
			service := AnalyticsServiceImpl{}

			// When
			request, err := service.convertRetrieveClosedConversationsRequstToESRequest(
				s.T().Context(), &config.ProfileConfig{}, &analyticspb.Metadata{TimeZoneId: "UTC"}, testCase.filter,
				analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_STARTED_AT,
				nil /* durationBuckets */, nil /* searchAfter */, nil, /* topicDetails */
				false /* randomize */, 0 /* seedForRandomSort */, []*userpb.User{{Name: "customers/customer/users/agent"}},
			)

			// Then
			s.Require().Equal(testCase.wantCode, status.Code(err))
			if testCase.wantCode != codes.OK {
				return
			}
			queryJSON, err := json.Marshal(request.Query)
			s.Require().NoError(err)
			s.JSONEq(fmt.Sprintf(`{"bool":{"filter":[
				{"terms":{"agent_user_id.keyword":["agent"]}},
				%s,
				{"terms":{"workflow_automation_conditions":[%d]}},
				{"terms":{"conversation_to_message_join":["conversation"]}}
			]}}`, testCase.wantIDQuery, testCase.condition), string(queryJSON))
		})
	}
}

func (s *ClosedConversationsWorkflowAutomationSuite) TestWorkflowAutomationConditions() {
	for _, testCase := range []struct {
		name       string
		conditions []analyticspb.Attribute_WorkflowAutomationCondition
		wantCode   codes.Code
	}{
		{name: "WithoutWFA"},
		{name: "WithWFA", conditions: []analyticspb.Attribute_WorkflowAutomationCondition{analyticspb.Attribute_AUTOMATIC_FAILED}},
		{name: "InvalidCondition", conditions: []analyticspb.Attribute_WorkflowAutomationCondition{999}, wantCode: codes.InvalidArgument},
	} {
		s.Run(testCase.name, func() {
			// Given
			filter := &analyticspb.Attribute{
				TimeRanges: []*analyticspb.TimeRange{{
					StartTimestamp: timestamppb.New(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)),
				}},
				WorkflowAutomationConditions: testCase.conditions,
			}
			service := AnalyticsServiceImpl{}

			// When
			_, err := service.convertRetrieveClosedConversationsRequstToESRequest(
				s.T().Context(), &config.ProfileConfig{}, &analyticspb.Metadata{TimeZoneId: "UTC"}, filter,
				analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_STARTED_AT,
				nil /* durationBuckets */, nil /* searchAfter */, nil, /* topicDetails */
				false /* randomize */, 0 /* seedForRandomSort */, []*userpb.User{{Name: "customers/customer/users/agent"}},
			)

			// Then
			s.Equal(testCase.wantCode, status.Code(err))
		})
	}
}
