package analyticsimpl

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	configpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/shared-go/fn"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RetrievePartialConversationsTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrievePartialConversationsTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
}

func TestRetrievePartialConversations_Failure(t *testing.T) {
	r := RetrievePartialConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()
	r.mockConfigClient.EXPECT().Get("customer_id").AnyTimes().Return(&config.CustomerConfig{}, nil)

	for _, tc := range []struct {
		desc string
		req  *analyticspb.RetrievePartialConversationsRequest
		err  string
	}{
		{
			desc: "EmptyMessageWindowsAndMessageLists",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent:         r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{},
				MessageLists:   []*analyticspb.MessageList{},
			},
			err: "at least one message window or message list must be specified",
		},
		{
			desc: "BothMessageWindowsAndMessageListsProvided",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 3,
							},
						},
						MessagePositionDirection: analyticspb.MessageWindow_FORWARD,
					},
				},
				MessageLists: []*analyticspb.MessageList{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessageIds:   []string{"dummy-msg-id-1", "dummy-msg-id-2"},
					},
				},
			},
			err: "either a message window or a message list must be specified, both are not supported",
		},
		{
			desc: "MessageNamesNotMatched",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: "customers/customer_id/profiles/profile_id/conversations/conversation_id",
					},
					{
						Conversation: "customers/diff_customer_id/profiles/profile_id/conversations/conversation_id",
					},
				},
			},
			err: "does not match the parent's customer or profile ID",
		},
	} {
		r.mockConfigClient.EXPECT().
			GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
			Return(&config.ProfileConfig{}, nil)
		_, err := r.Client.RetrievePartialConversations(r.ctx, tc.req)
		r.Error(err)
		r.ErrorContains(err, tc.err)
	}
}

func TestRetrievePartialConversations_Success(t *testing.T) {
	r := RetrievePartialConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	const flagInsightsMessageCreateTime = "INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME"
	prevUsePlatformTS := *usePlatformTimestampForMessageCreateTime
	prevEnv, hadPrevEnv := os.LookupEnv(flagInsightsMessageCreateTime)
	envutil.SetEnv(flagInsightsMessageCreateTime, "true")
	if err := envflag.Set(flagInsightsMessageCreateTime, "true"); err != nil {
		t.Fatalf("envflag.Set(%q): %v", flagInsightsMessageCreateTime, err)
	}
	t.Cleanup(func() {
		if hadPrevEnv {
			envutil.SetEnv(flagInsightsMessageCreateTime, prevEnv)
		} else {
			os.Unsetenv(flagInsightsMessageCreateTime)
		}
		_ = envflag.Set(flagInsightsMessageCreateTime, strconv.FormatBool(prevUsePlatformTS))
	})

	for _, tc := range []struct {
		desc                            string
		esReqPath                       string
		esRespPath                      string
		req                             *analyticspb.RetrievePartialConversationsRequest
		want                            *analyticspb.RetrievePartialConversationsResponse
		enableAdvancedDataAccessControl bool
		resourceACL                     *auth.ResourceACL
	}{
		{
			desc: "BasicRequest",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 3,
							},
						},
						MessagePositionDirection: analyticspb.MessageWindow_FORWARD,
					},
					{
						Conversation: r.profileResourceName + "/conversations/9609e7ab-8947-4dd6-9120-1673ea399a34",
					},
				},
			},
			esReqPath: "RetrievePartialConversations_BasicRequest_es_request.json",
		},
		{
			desc: "OnlyOneBoundSpecified",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 1,
							},
						},
					},
					{
						Conversation: r.profileResourceName + "/conversations/9609e7ab-8947-4dd6-9120-1673ea399a34",
						MessagePositionRange: &analyticspb.IntRange{
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 1,
							},
						},
					},
				},
			},
			esReqPath: "RetrievePartialConversations_OnlyOneBoundSpecified_es_request.json",
		},
		{
			desc: "SearchByBackwardRange",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 3,
							},
						},
						MessagePositionDirection: analyticspb.MessageWindow_BACKWARD,
					},
				},
			},
			esReqPath: "RetrievePartialConversations_SearchByBackwardRange_es_request.json",
		},
		{
			desc: "UseExclusiveBound",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value:       0,
								IsExclusive: true,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value:       3,
								IsExclusive: true,
							},
						},
					},
				},
			},
			esReqPath: "RetrievePartialConversations_UseExclusiveBound_es_request.json",
		},
		{
			desc: "UseMessageList",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageLists: []*analyticspb.MessageList{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessageIds:   []string{"dummy-msg-id-1", "dummy-msg-id-2"},
					},
				},
			},
			esReqPath: "RetrievePartialConversations_UseMessageList_es_request.json",
		},
		{
			desc: "VerifyResponse",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 4,
							},
						},
					},
					{
						Conversation: r.profileResourceName + "/conversations/9609e7ab-8947-4dd6-9120-1673ea399a34",
					},
				},
				PageSize:  2,
				PageToken: "dummy-page-token",
			},
			esReqPath:  "RetrievePartialConversations_VerifyResponse_es_request.json",
			esRespPath: "RetrievePartialConversations_VerifyResponse_es_response.json",
			want: &analyticspb.RetrievePartialConversationsResponse{
				ConversationMessages: []*conversationpb.ConversationMessage{
					{
						Name:             r.profileResourceName + "/conversations/dummy-convo-id/messages/dummy-msg-id-1",
						Text:             "Thank you for calling. My name is John. How can I assist you today?",
						Speaker:          conversationpb.ConversationMessage_AGENT,
						SequencePosition: proto.Int32(0),
						CreateTime:       timestamppb.New(time.Date(2024, 9, 2, 6, 16, 40, 801000000, time.UTC)),
					},
					{
						Name:             r.profileResourceName + "/conversations/dummy-convo-id/messages/dummy-msg-id-2",
						Text:             "Hi, John. I'm Jane. I have a problem with my latest bill. It seems higher than usual.",
						Speaker:          conversationpb.ConversationMessage_VISITOR,
						SequencePosition: proto.Int32(1),
						CreateTime:       timestamppb.New(time.Date(2024, 9, 2, 6, 16, 46, 780000000, time.UTC)),
					},
				},
				NextPageToken: "2024-09-02T06:17:18.823Z###2024-09-02T06:16:48.901Z",
			},
		},
		{
			desc: "EnableACL",
			req: &analyticspb.RetrievePartialConversationsRequest{
				Parent: r.profileResourceName,
				MessageWindows: []*analyticspb.MessageWindow{
					{
						Conversation: r.profileResourceName + "/conversations/4aa51fe7-f744-4775-98b6-57ec1e6b3837",
						MessagePositionRange: &analyticspb.IntRange{
							LowerBound: &analyticspb.IntRange_Bound{
								Value: 0,
							},
							UpperBound: &analyticspb.IntRange_Bound{
								Value: 3,
							},
						},
						MessagePositionDirection: analyticspb.MessageWindow_FORWARD,
					},
					{
						Conversation: r.profileResourceName + "/conversations/9609e7ab-8947-4dd6-9120-1673ea399a34",
					},
				},
			},
			esReqPath:                       "RetrievePartialConversations_includeACLUsers_es_request.json",
			enableAdvancedDataAccessControl: true,
			resourceACL: &auth.ResourceACL{
				IsRootAccess:   false,
				ManagedUserIDs: []string{"35636a572d7ec364", "2f540f161f3a452c"},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================", tc.desc)
			if tc.esRespPath != "" {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			} else {
				r.mockESServer.Resps = append(r.mockESServer.Resps, emptyESHitsResponse)
			}
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))
			cConfig := configpb.CustomerConfig{}
			r.Require().NoError(protojson.Unmarshal([]byte(fmt.Sprintf(`{
					"customer": "customers/customer_id",
					"profileConfigs": [
						{
							"name": "customers/customer_id/profiles/profile_id",
							"features": {
								"insights": {
								}
							}
						}
					],
					"authConfig": {
						"enableAdvancedDataAccessControl": %v
					}}
				`, tc.enableAdvancedDataAccessControl)), &cConfig))
			testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).Times(1).
				Return(testCustomerConfig, nil)
			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)

			if tc.enableAdvancedDataAccessControl && tc.resourceACL != nil {
				r.setupACLHelper(gomock.Any(), 1, tc.resourceACL)
			}
			resp, err := r.Client.RetrievePartialConversations(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("RetrievePartialConversations returned unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}
