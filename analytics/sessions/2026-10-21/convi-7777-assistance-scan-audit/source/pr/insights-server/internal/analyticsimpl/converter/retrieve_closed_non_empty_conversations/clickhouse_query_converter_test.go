package retrieveclosednonemptyconversations

import (
	"strings"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConvertRetrieveClosedNonEmptyConversationsRequestToClickhouseQuery(t *testing.T) {
	tests := []struct {
		name             string
		req              *analyticspb.RetrieveClosedNonEmptyConversationsRequest
		userIDs          []string
		expectedQuery    string
		expectedPageSize int
		expectedOffset   int
	}{
		{
			name: "BasicRequest",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				StartTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 1, 1, 23, 59, 59, 0, time.UTC)),
				},
				EndTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2023, 1, 2, 23, 59, 59, 0, time.UTC)),
				},
				PageSize:  10,
				PageToken: "100",
			},
			userIDs: []string{"user1", "user2"},
			expectedQuery: `
		WITH
			conv AS (
				SELECT
					conversation_id,
					latest.1 AS conversation_start_time,
					latest.2 AS conversation_end_time,
					latest.3 AS conversation_source,
					latest.4 AS agent_user_id
				FROM (
					SELECT
						conversation_id,
						argMax(tuple(conversation_start_time, conversation_end_time, conversation_source, agent_user_id), update_time) AS latest
					FROM
						conversation_d
					WHERE
						conversation_id GLOBAL IN (
							SELECT
								conversation_id
							FROM
								conversation_d
							WHERE
								agent_user_id IN ('user1','user2') -- //nolint:sqlinjection
								AND conversation_start_time >= '2022-12-31 00:00:00.000000'
						AND conversation_start_time <= '2023-01-02 23:59:59.000000'
						AND (conversation_end_time >= '2023-01-02 00:00:00.000000' OR update_time >= '2023-01-02 00:00:00.000000')
						AND conversation_start_time <= '2023-01-03 23:59:59.000000'
						)
						AND conversation_start_time >= '2022-12-31 00:00:00.000000'
						AND conversation_start_time <= '2023-01-02 23:59:59.000000'
						AND (conversation_end_time >= '2023-01-02 00:00:00.000000' OR update_time >= '2023-01-02 00:00:00.000000')
						AND conversation_start_time <= '2023-01-03 23:59:59.000000'
					GROUP BY
						conversation_id
				)
				WHERE
					agent_user_id IN ('user1','user2') -- //nolint:sqlinjection
					AND conversation_source IN (0, 7, 8)
					AND conversation_start_time >= '2023-01-01 00:00:00.000000'
					AND conversation_start_time <= '2023-01-01 23:59:59.000000'
					AND conversation_end_time >= '2023-01-02 00:00:00.000000'
					AND conversation_end_time <= '2023-01-02 23:59:59.000000'
			),
			msg AS (
				SELECT
					message_id,
					conversation_id
				FROM
					message_d
			)
		SELECT
			conv.conversation_id,
			conv.conversation_start_time AS conversation_start_time,
			conv.conversation_end_time AS conversation_end_time,
			COUNT(DISTINCT msg.message_id) as message_count
		FROM
			conv
			JOIN msg ON conv.conversation_id = msg.conversation_id
		GROUP BY
			conv.conversation_id,
			conv.conversation_start_time,
			conv.conversation_end_time
		HAVING
			message_count > 0
		ORDER BY
			conversation_start_time DESC
		LIMIT 10
		OFFSET 100
		SETTINGS select_sequential_consistency = 1
	`,
			expectedPageSize: 10,
			expectedOffset:   100,
		},
		{
			name: "EmptyTimeRangeFiltersRequest",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				PageSize:  10,
				PageToken: "100",
			},
			userIDs: []string{"user1", "user2"},
			expectedQuery: `
		WITH
			conv AS (
				SELECT
					conversation_id,
					latest.1 AS conversation_start_time,
					latest.2 AS conversation_end_time,
					latest.3 AS conversation_source,
					latest.4 AS agent_user_id
				FROM (
					SELECT
						conversation_id,
						argMax(tuple(conversation_start_time, conversation_end_time, conversation_source, agent_user_id), update_time) AS latest
					FROM
						conversation_d
					WHERE
						conversation_id GLOBAL IN (
							SELECT
								conversation_id
							FROM
								conversation_d
							WHERE
								agent_user_id IN ('user1','user2') -- //nolint:sqlinjection
								AND TRUE
						)
						AND TRUE
					GROUP BY
						conversation_id
				)
				WHERE
					agent_user_id IN ('user1','user2') -- //nolint:sqlinjection
					AND conversation_source IN (0, 7, 8)
			),
			msg AS (
				SELECT
					message_id,
					conversation_id
				FROM
					message_d
			)
		SELECT
			conv.conversation_id,
			conv.conversation_start_time AS conversation_start_time,
			conv.conversation_end_time AS conversation_end_time,
			COUNT(DISTINCT msg.message_id) as message_count
		FROM
			conv
			JOIN msg ON conv.conversation_id = msg.conversation_id
		GROUP BY
			conv.conversation_id,
			conv.conversation_start_time,
			conv.conversation_end_time
		HAVING
			message_count > 0
		ORDER BY
			conversation_start_time DESC
		LIMIT 10
		OFFSET 100
		SETTINGS select_sequential_consistency = 1
	`,
			expectedPageSize: 10,
			expectedOffset:   100,
		},
		{
			name: "PartialTimeRangeFiltersRequest",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				StartTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				EndTimeRange: &analyticspb.TimeRange{
					EndTimestamp: timestamppb.New(time.Date(2023, 1, 2, 23, 59, 59, 0, time.UTC)),
				},
				PageSize:  10,
				PageToken: "100",
			},
			userIDs: []string{"user1", "user2"},
			expectedQuery: `
		WITH
			conv AS (
				SELECT
					conversation_id,
					latest.1 AS conversation_start_time,
					latest.2 AS conversation_end_time,
					latest.3 AS conversation_source,
					latest.4 AS agent_user_id
				FROM (
					SELECT
						conversation_id,
						argMax(tuple(conversation_start_time, conversation_end_time, conversation_source, agent_user_id), update_time) AS latest
					FROM
						conversation_d
					WHERE
						conversation_id GLOBAL IN (
							SELECT
								conversation_id
							FROM
								conversation_d
							WHERE
								agent_user_id IN ('user1','user2') -- //nolint:sqlinjection
								AND conversation_start_time >= '2022-12-31 00:00:00.000000'
						AND conversation_start_time <= '2023-01-03 23:59:59.000000'
						)
						AND conversation_start_time >= '2022-12-31 00:00:00.000000'
						AND conversation_start_time <= '2023-01-03 23:59:59.000000'
					GROUP BY
						conversation_id
				)
				WHERE
					agent_user_id IN ('user1','user2') -- //nolint:sqlinjection
					AND conversation_source IN (0, 7, 8)
					AND conversation_start_time >= '2023-01-01 00:00:00.000000'
					AND conversation_end_time <= '2023-01-02 23:59:59.000000'
			),
			msg AS (
				SELECT
					message_id,
					conversation_id
				FROM
					message_d
			)
		SELECT
			conv.conversation_id,
			conv.conversation_start_time AS conversation_start_time,
			conv.conversation_end_time AS conversation_end_time,
			COUNT(DISTINCT msg.message_id) as message_count
		FROM
			conv
			JOIN msg ON conv.conversation_id = msg.conversation_id
		GROUP BY
			conv.conversation_id,
			conv.conversation_start_time,
			conv.conversation_end_time
		HAVING
			message_count > 0
		ORDER BY
			conversation_start_time DESC
		LIMIT 10
		OFFSET 100
		SETTINGS select_sequential_consistency = 1
	`,
			expectedPageSize: 10,
			expectedOffset:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, pageSize, offset := ConvertRetrieveClosedNonEmptyConversationsRequestToClickhouseQuery(tt.req, tt.userIDs)

			// Normalize whitespace for comparison
			normalizedQuery := strings.ReplaceAll(strings.TrimSpace(query), "\t", "")
			normalizedExpected := strings.ReplaceAll(strings.TrimSpace(tt.expectedQuery), "\t", "")

			assert.Equal(t, normalizedExpected, normalizedQuery)
			assert.Equal(t, tt.expectedPageSize, pageSize)
			assert.Equal(t, tt.expectedOffset, offset)
		})
	}
}

func TestParsePageSizeAndOffset(t *testing.T) {
	tests := []struct {
		name             string
		req              *analyticspb.RetrieveClosedNonEmptyConversationsRequest
		expectedPageSize int
		expectedOffset   int
	}{
		{
			name:             "DefaultValues",
			req:              &analyticspb.RetrieveClosedNonEmptyConversationsRequest{},
			expectedPageSize: 20,
			expectedOffset:   0,
		},
		{
			name: "CustomPageSize",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				PageSize: 100,
			},
			expectedPageSize: 100,
			expectedOffset:   0,
		},
		{
			name: "ValidPageToken",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				PageSize:  50,
				PageToken: "200",
			},
			expectedPageSize: 50,
			expectedOffset:   200,
		},
		{
			name: "InvalidPageToken",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				PageSize:  30,
				PageToken: "not_a_number",
			},
			expectedPageSize: 30,
			expectedOffset:   0,
		},
		{
			name: "EmptyPageToken",
			req: &analyticspb.RetrieveClosedNonEmptyConversationsRequest{
				PageSize:  25,
				PageToken: "",
			},
			expectedPageSize: 25,
			expectedOffset:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pageSize, offset := parsePageSizeAndOffset(tt.req)
			assert.Equal(t, tt.expectedPageSize, pageSize)
			assert.Equal(t, tt.expectedOffset, offset)
		})
	}
}
