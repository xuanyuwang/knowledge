
WITH


conversations AS (
	SELECT DISTINCT conversation_id
	FROM conversation_d
	-- Join moment annotation filters to include criteria

	-- Join moment annotation filters to exclude criteria

	WHERE agent_user_id <> ''
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00') AND (agent_user_id IN ('1','2','3','4')))

),
cs_events AS (
	SELECT
		conversation_id,
		agent_user_id,
		conversation_start_time,
		event_type,
		toUnixTimestamp(create_time) AS ts,
				-- Typed Object('json') subcolumn access: reads only the needed
		-- sessionId/contentHash subcolumns instead of materializing the full
		-- event_payload JSON string, and CAST normalizes a missing field to ''
		-- (matching JSONExtractString). ifNull is not required.
if(event_type = 27,
				CAST(event_payload.customerSnapshotPresented.sessionId AS String),
					CAST(event_payload.customerSnapshotClosed.sessionId   AS String)) AS session_id,
				if(event_type = 27,
					CAST(event_payload.customerSnapshotPresented.contentHash AS String),
					CAST(event_payload.customerSnapshotClosed.contentHash   AS String)) AS content_hash
	FROM conversation_event_d
	-- Join moment annotation filters to include criteria

	-- Join moment annotation filters to exclude criteria

	WHERE agent_user_id <> ''
		AND event_type IN (27, 28)
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00') AND (agent_user_id IN ('1','2','3','4')))

),
ev AS (
	SELECT
		conversation_id,
		agent_user_id,
		conversation_start_time,
		event_type,
		ts,
		session_id,
		content_hash
	FROM cs_events JOIN conversations USING (conversation_id)
),
-- Per-session pre-aggregation: one interval per (conversation, session,
-- content_hash), from the first presented to the last closed. Sessions
-- without both a presented and a closed event, or where the last close
-- precedes the first present, are dropped. This pre-aggregation shrinks
-- the input to the overlap-merge below from per-event to per-session rows.
session_intervals AS (
	SELECT
		conversation_id,
		session_id,
		any(agent_user_id)           AS agent_user_id,
		any(conversation_start_time) AS conversation_start_time,
		minIf(ts, event_type = 27)   AS start_ts,
		maxIf(ts, event_type = 28)   AS end_ts
	FROM ev
	GROUP BY conversation_id, session_id, content_hash
	HAVING countIf(event_type = 27) > 0
		AND countIf(event_type = 28) > 0
		AND end_ts >= start_ts
),
-- Merge overlapping/adjacent session intervals (islands-and-gaps) so the
-- summed duration does not double-count overlapping visibility windows.
ordered AS (
	SELECT conversation_id, agent_user_id, conversation_start_time, start_ts, end_ts, session_id,
		max(end_ts) OVER (PARTITION BY conversation_id ORDER BY start_ts, end_ts, session_id
			ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS prev_max_end
	FROM session_intervals
),
islands AS (
	SELECT conversation_id, agent_user_id, conversation_start_time, start_ts, end_ts, session_id,
		sum(if(prev_max_end = 0 OR start_ts > prev_max_end, 1, 0)) OVER
			(PARTITION BY conversation_id ORDER BY start_ts, end_ts, session_id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS island_id
	FROM ordered
),
merged AS (
	SELECT conversation_id,
		any(agent_user_id)           AS agent_user_id,
		any(conversation_start_time) AS conversation_start_time,
		island_id, min(start_ts) AS s, max(end_ts) AS e
	FROM islands
	GROUP BY conversation_id, island_id
),
per_conversation AS (
	SELECT conversation_id,
		any(agent_user_id)           AS agent_user_id,
		any(conversation_start_time) AS conversation_start_time,
		-- toUInt64 keeps the summed duration UInt64 (bare e-s promotes to
		-- Int64), matching the uint64 row field the driver scans into.
		sum(toUInt64(e - s))         AS display_secs
	FROM merged
	GROUP BY conversation_id
)
SELECT
	DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
agent_user_id,
	sum(display_secs) AS total_display_seconds,
	count()           AS total_conversations_with_snapshot,
	COUNT(DISTINCT agent_user_id) AS user_count
FROM per_conversation
GROUP BY truncated_time, agent_user_id
