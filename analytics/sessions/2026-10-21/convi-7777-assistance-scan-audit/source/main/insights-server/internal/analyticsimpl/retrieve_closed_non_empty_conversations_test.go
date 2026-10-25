// nolint:dupl
package analyticsimpl

import (
	"fmt"
	"testing"
	"time"

	"database/sql"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/stretchr/testify/mock"
)

type RetrieveClosedNonEmptyConversationsTestSuite struct {
	AnalyticsV2TestSuite
}

func TestRetrieveClosedNonEmptyConversations(t *testing.T) {
	suite.Run(t, &RetrieveClosedNonEmptyConversationsTestSuite{})
}

func (s *RetrieveClosedNonEmptyConversationsTestSuite) SetupSuite() {
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
	s.UseMockedClickhouseConnections()
}

func (s *RetrieveClosedNonEmptyConversationsTestSuite) TearDownTest() {
	s.convoMockConn.AssertExpectations(s.T())
}

func TestRetrieveClosedNonEmptyConversations_Success(t *testing.T) {
	r := RetrieveClosedNonEmptyConversationsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                                  string
		request                               *analyticspb.RetrieveClosedNonEmptyConversationsRequest
		mockResponse                          [][]interface{}
		expectedCount                         int64
		expectedToken                         string
		enableEsQueryAttendeesForAgentUserIDs bool
	}{
		{
			desc: "BasicRequest",
			request: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				Parent: r.profileResourceName,
				StartTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 1, 1, 23, 59, 59, 0, time.UTC)),
				},
				EndTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 1, 2, 23, 59, 59, 0, time.UTC)),
				},
				PageSize:  2,
				PageToken: "2",
			},
			mockResponse: [][]interface{}{
				{
					"conv-789",
					time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC),
					time.Date(2023, 1, 1, 15, 0, 0, 0, time.UTC),
					uint64(8),
				},
			},
			expectedCount: 1,
			expectedToken: "", // No next page token since we have fewer results than page size
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)

			if tc.enableEsQueryAttendeesForAgentUserIDs {
				err := envflag.Set("ENABLE_ES_QUERY_ATTENDEES_FOR_AGENT_USER_IDS", "true")
				r.Require().NoError(err)
			}

			// Setup mock config for ACL resource
			// cConfig := configpb.CustomerConfig{}
			// r.Require().NoError(protojson.Unmarshal([]byte(`{
			// 		"customer": "customers/customer_id",
			// 		"profileConfigs": [
			// 			{
			// 				"name": "customers/customer_id/profiles/profile_id"
			// 			}
			// 		]}
			// 	`), &cConfig))
			// testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
			// r.mockConfigClient.EXPECT().
			// 	Get(gomock.Any()).Times(1).
			// 	Return(testCustomerConfig, nil)

			// Setup mock response
			mockRows := clickhousetesting.NewMockRows(tc.mockResponse)
			r.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).
				Return(mockRows, nil).Once()

			// Execute the request
			resp, err := r.Client.RetrieveClosedNonEmptyConversations(r.ctx, tc.request)

			// Validate the response
			require.NoError(t, err, "RetrieveClosedNonEmptyConversations should not return an error")
			require.NotNil(t, resp, "Response should not be nil")
			require.NotNil(t, resp.Conversations, "Conversations should not be nil")
			assert.Equal(t, tc.expectedCount, resp.TotalConversationCount, "Total conversation count should match")
			assert.Equal(t, tc.expectedToken, resp.NextPageToken, "Next page token should match")
			assert.Len(t, resp.Conversations, int(tc.expectedCount), "Number of conversations should match")

			// Validate clickhouse query was called
			require.Len(t, r.convoMockConn.Calls, 1, "Expected exactly one call to clickhouse")
			assert.Equal(t, "Query", r.convoMockConn.Calls[0].Method, "Expected call to Query method")

			// Validate conversation details for non-empty results
			if tc.expectedCount > 0 {
				r.validateConversations(resp.Conversations, tc.mockResponse)
			}
		})
	}
}

// Helper method to validate conversation details
func (s *RetrieveClosedNonEmptyConversationsTestSuite) validateConversations(
	conversations []*convopb.Conversation,
	mockData [][]interface{},
) {
	s.Require().Len(conversations, len(mockData), "Number of conversations should match mock data")

	for i, conv := range conversations {
		mockRow := mockData[i]
		expectedConvID := mockRow[0].(string)
		expectedStartTime := mockRow[1].(time.Time)
		expectedEndTime := mockRow[2].(time.Time)
		expectedMessageCount := int32(mockRow[3].(uint64))

		// Validate conversation name format
		expectedName := fmt.Sprintf("customers/%s/profiles/%s/conversations/%s",
			s.customerID, s.profileID, expectedConvID)
		s.Equal(expectedName, conv.Name, "Conversation name should match expected format")

		// Validate timestamps
		s.Equal(expectedStartTime, conv.StartTime.AsTime(), "Start time should match")
		s.Equal(expectedEndTime, conv.EndTime.AsTime(), "End time should match")

		// Validate extended info
		s.NotNil(conv.ExtendedInfo, "Extended info should not be nil")
		s.Equal(expectedMessageCount, conv.ExtendedInfo.MessageCount, "Message count should match")
	}
}
