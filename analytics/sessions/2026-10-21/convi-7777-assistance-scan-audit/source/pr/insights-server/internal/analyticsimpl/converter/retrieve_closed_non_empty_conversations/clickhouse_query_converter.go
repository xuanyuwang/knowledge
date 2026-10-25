package retrieveclosednonemptyconversations

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
)

const (
	defaultPageSize = 20
	// Time format for ClickHouse queries: YYYY-MM-DD HH:MM:SS.microseconds
	timeFormat = "2006-01-02 15:04:05.000000"
)

func ConvertRetrieveClosedNonEmptyConversationsRequestToClickhouseQuery(req *analyticspb.RetrieveClosedNonEmptyConversationsRequest, userIDs []string) (string, int, int) {
	pageSize, offset := parsePageSizeAndOffset(req)

	agentCondition := fmt.Sprintf("agent_user_id IN ('%s') -- //nolint:sqlinjection", strings.Join(userIDs, "','"))
	scanBounds := buildScanBounds(req)
	queryFilterConditions := buildQueryFilter(req, agentCondition)

	// conversation_d holds every version of a conversation until ReplacingMergeTree
	// merges them, and versions that differ in agent or start time never merge,
	// since both lead the sort key. So conv takes each conversation's newest version
	// with argMax and filters that. Filtering first would let an older version pass
	// for a conversation since reassigned to another agent, or with an end time it
	// no longer has. One argMax over a tuple keeps all the columns from the same
	// version, even if two versions tie on update_time; ClickHouse resolves it on
	// the merged stream at the initiator, so it holds across shards. The aliases sit
	// one scope above the scan, so the scan's WHERE reads the raw columns.
	//
	// The collapse reads only conversations that have a version with the agent: the
	// GLOBAL IN candidate set keeps the agent pruning on the sort key, and the outer
	// scan then reads every version of those conversations, including versions
	// with another agent. scanBounds prune both scans on time; see buildScanBounds
	// for which versions they can drop.
	//
	// conv has one row per conversation, so the outer GROUP BY gives each
	// conversation one output row, and LIMIT and OFFSET count conversations, not
	// versions. COUNT(DISTINCT message_id) counts a message_d row that was
	// re-ingested, and not yet merged, once.
	return fmt.Sprintf(`
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
								%s
								AND %s
						)
						AND %s
					GROUP BY
						conversation_id
				)
				WHERE
					%s
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
		LIMIT %d
		OFFSET %d
		SETTINGS select_sequential_consistency = 1
	`, agentCondition, scanBounds, scanBounds, queryFilterConditions, pageSize, offset),
		pageSize, offset
}

func parsePageSizeAndOffset(req *analyticspb.RetrieveClosedNonEmptyConversationsRequest) (int, int) {
	// Parse the page size and offset.
	pageSize := defaultPageSize
	if req.PageSize > 0 {
		pageSize = int(req.PageSize)
	}
	offset := 0
	if req.PageToken != "" {
		var err error
		offset, err = strconv.Atoi(req.PageToken)
		if err != nil {
			return pageSize, 0
		}
	}

	return pageSize, offset
}

// startTimeCorrectionSlack pads the conversation_start_time bounds in
// buildScanBounds, because ingest can correct a conversation's start time after
// the first write. It matches the constant of the same name in
// shared/clickhouse/conversations.
const startTimeCorrectionSlack = 24 * time.Hour

// buildScanBounds bounds the conversation_d scans on conversation_start_time,
// which leads the sort key. The bounds only prune: a version that passes
// buildQueryFilter also passes them, so if the newest version passes the filters,
// the collapse reads it.
//   - The start range and the upper end of the end range bound the start time,
//     padded by startTimeCorrectionSlack. A conversation never starts after it
//     ends.
//   - The lower end of the end range adds the end-or-update condition, which
//     keeps versions that ended or were rewritten since the range began.
//     update_time is the wall-clock time of the write, which is never before a
//     real end, so if any version passes it, the newest one does too. An end
//     range puts no lower bound on the start time, so a conversation of any
//     length is found.
func buildScanBounds(req *analyticspb.RetrieveClosedNonEmptyConversationsRequest) string {
	var conditions []string
	if startTimestamp := req.GetStartTimeRange().GetStartTimestamp(); startTimestamp != nil {
		lowerBound := startTimestamp.AsTime().Add(-startTimeCorrectionSlack).Format(timeFormat)
		conditions = append(conditions, fmt.Sprintf("conversation_start_time >= '%s'", lowerBound))
	}
	if endTimestamp := req.GetStartTimeRange().GetEndTimestamp(); endTimestamp != nil {
		upperBound := endTimestamp.AsTime().Add(startTimeCorrectionSlack).Format(timeFormat)
		conditions = append(conditions, fmt.Sprintf("conversation_start_time <= '%s'", upperBound))
	}
	if startTimestamp := req.GetEndTimeRange().GetStartTimestamp(); startTimestamp != nil {
		endRangeStart := startTimestamp.AsTime().Format(timeFormat)
		conditions = append(conditions,
			fmt.Sprintf("(conversation_end_time >= '%s' OR update_time >= '%s')", endRangeStart, endRangeStart))
	}
	if endTimestamp := req.GetEndTimeRange().GetEndTimestamp(); endTimestamp != nil {
		upperBound := endTimestamp.AsTime().Add(startTimeCorrectionSlack).Format(timeFormat)
		conditions = append(conditions, fmt.Sprintf("conversation_start_time <= '%s'", upperBound))
	}
	if len(conditions) == 0 {
		return "TRUE"
	}
	return strings.Join(conditions, "\n\t\t\t\t\t\tAND ")
}

// buildQueryFilter filters each conversation's newest version.
func buildQueryFilter(req *analyticspb.RetrieveClosedNonEmptyConversationsRequest, agentCondition string) string {
	conditions := []string{agentCondition, "conversation_source IN (0, 7, 8)"}

	// Add start time range conditions if present
	if req.StartTimeRange != nil {
		if req.StartTimeRange.StartTimestamp != nil {
			startTimeRangeStart := req.StartTimeRange.StartTimestamp.AsTime().Format(timeFormat)
			conditions = append(conditions, fmt.Sprintf("conversation_start_time >= '%s'", startTimeRangeStart))
		}

		if req.StartTimeRange.EndTimestamp != nil {
			startTimeRangeEnd := req.StartTimeRange.EndTimestamp.AsTime().Format(timeFormat)
			conditions = append(conditions, fmt.Sprintf("conversation_start_time <= '%s'", startTimeRangeEnd))
		}
	}

	// Add end time range conditions if present
	if req.EndTimeRange != nil {
		if req.EndTimeRange.StartTimestamp != nil {
			endTimeRangeStart := req.EndTimeRange.StartTimestamp.AsTime().Format(timeFormat)
			conditions = append(conditions, fmt.Sprintf("conversation_end_time >= '%s'", endTimeRangeStart))
		}

		if req.EndTimeRange.EndTimestamp != nil {
			endTimeRangeEnd := req.EndTimeRange.EndTimestamp.AsTime().Format(timeFormat)
			conditions = append(conditions, fmt.Sprintf("conversation_end_time <= '%s'", endTimeRangeEnd))
		}
	}

	return strings.Join(conditions, "\n\t\t\t\t\tAND ")
}
