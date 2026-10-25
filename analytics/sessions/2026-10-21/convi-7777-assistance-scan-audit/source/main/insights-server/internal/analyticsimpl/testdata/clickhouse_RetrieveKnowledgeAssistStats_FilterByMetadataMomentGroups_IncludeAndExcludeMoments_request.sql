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
),
moment_annotation_exclude_filter_0 AS (
	SELECT DISTINCT
		conversation_id AS conversation_id_exclude_0
	FROM
		moment_annotation_mv_by_metadata_d
	WHERE
		((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00') AND (moment_template_id = 'moment_id_2'))
),
messages AS (
	SELECT
		DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
		agent_user_id,
		COUNT(DISTINCT agent_user_id) AS user_count
	FROM
		message_d
	-- Join moment annotation filters to include criteria
	GLOBAL INNER JOIN moment_annotation_filter_0 ON
		message_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	-- Join moment annotation filters to exclude criteria
	GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
		message_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	WHERE
		conversation_source IN (0, 8)
		AND is_dev_user = 0
		AND agent_user_id <> ''
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00')) -- Additional filters
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	GROUP BY truncated_time, agent_user_id
),
action_annotations AS (
	SELECT
		DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
		agent_user_id,
		COUNT(DISTINCT CASE WHEN action_type = 22 THEN action_annotation_id END) AS question_extraction_count,
		COUNT(DISTINCT CASE WHEN action_type = 24 THEN knowledge_payload_knowledge_session_id END) AS retrieval_article_count
	FROM
		action_annotation_d
	-- Join moment annotation filters to include criteria
	GLOBAL INNER JOIN moment_annotation_filter_0 ON
		action_annotation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	-- Join moment annotation filters to exclude criteria
	GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
		action_annotation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	WHERE
		conversation_source IN (0, 8)
		AND is_dev_user = 0
		AND agent_user_id <> ''
		AND knowledge_payload_knowledge_session_id <> ''
		AND action_type in (22, 24)  -- (RETRIEVAL_QUESTION, RETRIEVAL_ARTICLE)
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00')) -- Additional filters
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	GROUP BY truncated_time, agent_user_id
),
moment_annotations AS (
	SELECT
		DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
		agent_user_id,
		COUNT(DISTINCT knowledge_payload_knowledge_session_id) AS answer_used_count
	FROM
		moment_annotation_d
	-- Join moment annotation filters to include criteria
	GLOBAL INNER JOIN moment_annotation_filter_0 ON
		moment_annotation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
	-- Join moment annotation filters to exclude criteria
	GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
		moment_annotation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	WHERE
		conversation_source IN (0, 8)
		AND is_dev_user = 0
		AND agent_user_id <> ''
		AND knowledge_payload_knowledge_session_id <> ''
		AND moment_type = 38  -- KA_GEN_AI_REPLY_USED
		AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-11 00:00:00')) -- Additional filters
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
	GROUP BY truncated_time, agent_user_id
)
SELECT
	messages.truncated_time,messages.agent_user_id,
	action_annotations.retrieval_article_count AS gen_ai_answer_viewed_count,
	moment_annotations.answer_used_count AS gen_ai_answer_used_count,
	action_annotations.question_extraction_count AS knowledge_moments_detected_count,
	messages.user_count
FROM
	messages
	LEFT JOIN action_annotations ON messages.truncated_time = action_annotations.truncated_time AND messages.agent_user_id = action_annotations.agent_user_id
	LEFT JOIN moment_annotations ON messages.truncated_time = moment_annotations.truncated_time AND messages.agent_user_id = moment_annotations.agent_user_id
