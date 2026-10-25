
WITH

		-- Find the latest moment annotation for each conversation that matches non-metadata conditions
		t1_moment_annotation_filter_0 AS (
			SELECT
				conversation_id,
				MAX(update_time) AS latest_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00') AND (moment_template_id = 'moment_id_1'))
			GROUP BY conversation_id
		),
		-- Get all moment annotations that match the complete conditions (including metadata values)
		t2_moment_annotation_filter_0 AS (
			SELECT 
				conversation_id,
				update_time
			FROM 
				moment_annotation_mv_by_metadata_d
			WHERE 
				((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00') AND (moment_template_id = 'moment_id_1') AND (metadata_string_value = 'include-value-1'))
		),
		-- Final filter: Only include conversations where the LATEST moment annotation matches metadata criteria
		moment_annotation_filter_0 AS (
			SELECT DISTINCT
				t1.conversation_id AS conversation_id_0
			FROM 
				t1_moment_annotation_filter_0 AS t1
			JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_time = t2.update_time
		)
	
,

		moment_annotation_exclude_filter_0 AS (
			SELECT DISTINCT
				conversation_id AS conversation_id_exclude_0
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00') AND (moment_template_id = 'moment_id_2'))
		)
	,
conversations AS (
	SELECT DISTINCT conversation_id
	FROM conversation_d
	-- Join moment annotation filters to include criteria
	
		GLOBAL INNER JOIN moment_annotation_filter_0 ON
			conversation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	
	-- Join moment annotation filters to exclude criteria
	
			GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
				conversation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
		
	WHERE agent_user_id <> ''
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00'))
		
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	
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
	
		GLOBAL INNER JOIN moment_annotation_filter_0 ON
			conversation_event_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	
	-- Join moment annotation filters to exclude criteria
	
			GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
				conversation_event_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
		
	WHERE agent_user_id <> ''
		AND event_type IN (27, 28)
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00'))
		
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	
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
