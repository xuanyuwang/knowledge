WITH
-- Find the latest moment annotation for each conversation that matches non-metadata conditions
t1_moment_annotation_filter_0 AS (
	SELECT
		conversation_id,
		MAX(update_time) AS latest_time
	FROM
		moment_annotation_mv_by_metadata_d
	WHERE
		((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_template_id = 'moment_id_1'))
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
		((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_template_id = 'moment_id_1') AND (metadata_string_value = 'include-value-1'))
),
-- Final filter: Only include conversations where the LATEST moment annotation matches metadata criteria
moment_annotation_filter_0 AS (
	SELECT DISTINCT
		t1.conversation_id AS conversation_id_0
	FROM
		t1_moment_annotation_filter_0 AS t1
	JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_time = t2.update_time
),
moment_annotation_exclude_filter_0 AS (
	SELECT DISTINCT
		conversation_id AS conversation_id_exclude_0
	FROM
		moment_annotation_mv_by_metadata_d
	WHERE
		((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_template_id = 'moment_id_2'))
),
convs AS (
	SELECT
		DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
		agent_user_id,
		COUNT(DISTINCT agent_user_id) AS total_agent_count
	FROM
		conversation_d
	-- Join moment annotation filters to include criteria
	GLOBAL INNER JOIN moment_annotation_filter_0 ON
		conversation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	-- Join moment annotation filters to exclude criteria
	GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
		conversation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	WHERE
		conversation_source in (0, 8, 7)
		AND is_dev_user = 0
		AND agent_user_id <> ''
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00')) -- Additional filters
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	-- GROUP BY is optional
	GROUP BY truncated_time, agent_user_id
),
convs_with_aa AS (
	SELECT
		DATE_TRUNC('day', IF(message_id != '', message_time, conversation_start_time) + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
		agent_user_id,
		COUNT(DISTINCT agent_user_id) AS active_agent_count
	FROM
		conversation_with_labels_d
	-- Join moment annotation filters to include criteria
	GLOBAL INNER JOIN moment_annotation_filter_0 ON
		conversation_with_labels_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	-- Join moment annotation filters to exclude criteria
	GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
		conversation_with_labels_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	WHERE
		agent_user_id <> ''
		AND (has_agent_assistance = 1)
		AND (
			(message_id != '' AND ((message_time >= '2021-01-01 00:00:00') AND (message_time < '2021-01-10 00:00:00')))
			OR (message_id = '' AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00')))
		) -- Additional filters
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	-- GROUP BY is optional
	GROUP BY truncated_time, agent_user_id
)
SELECT
	truncated_time,
	agent_user_id,
	COALESCE(convs.total_agent_count, 0) AS total_agent_count,
	COALESCE(convs_with_aa.active_agent_count, 0) AS active_agent_count
FROM
	-- Using FULL OUTER JOIN to preserve rows from both CTEs.
	-- convs may miss days where agent only had non-qualifying source conversations.
	-- convs_with_aa captures agent assist from all sources, indicating agent login.
	convs
	FULL OUTER JOIN convs_with_aa USING (truncated_time, agent_user_id)
