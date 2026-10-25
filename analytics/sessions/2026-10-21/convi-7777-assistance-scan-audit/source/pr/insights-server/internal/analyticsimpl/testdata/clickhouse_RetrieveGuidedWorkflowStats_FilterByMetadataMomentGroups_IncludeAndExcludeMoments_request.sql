WITH usage_rows AS (
WITH
-- Find the latest moment annotation for each conversation that matches non-metadata conditions
t1_moment_annotation_filter_0 AS (
    SELECT
        conversation_id,
        MAX(update_time) AS latest_time
    FROM
        moment_annotation_mv_by_metadata_d
    WHERE
        ((conversation_start_time >= '2024-01-01 00:00:00') AND (conversation_start_time < '2024-01-11 00:00:00') AND (moment_template_id = 'moment_id_1'))
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
        ((conversation_start_time >= '2024-01-01 00:00:00') AND (conversation_start_time < '2024-01-11 00:00:00') AND (moment_template_id = 'moment_id_1') AND (metadata_string_value = 'include-value-1'))
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
        ((conversation_start_time >= '2024-01-01 00:00:00') AND (conversation_start_time < '2024-01-11 00:00:00') AND (moment_template_id = 'moment_id_2'))
),
conversations AS (
    SELECT DISTINCT
        conversation_id
    FROM 
        conversation_d
    -- Join moment annotation filters to include criteria
    GLOBAL INNER JOIN moment_annotation_filter_0 ON
        conversation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
    -- Join moment annotation filters to exclude criteria
    GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
        conversation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
    WHERE 
        agent_user_id <> ''
        AND ((conversation_start_time >= '2024-01-01 00:00:00') AND (conversation_start_time < '2024-01-11 00:00:00'))
        AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL -- Additional filters
),
gw_conversation_events AS (
    SELECT DISTINCT
        agent_user_id,
        conversation_id,
        conversation_start_time,
        event_type,
        `payload.used_kb_document.doc_id` AS guided_workflow_name,
        `payload.used_kb_document.doc_type` AS doc_type,
        `payload.used_kb_document.session_id` AS session_metadata_id
    FROM
        conversation_event_d
    -- Join moment annotation filters to include criteria
    GLOBAL INNER JOIN moment_annotation_filter_0 ON
        conversation_event_d.conversation_id = moment_annotation_filter_0.conversation_id_0
    -- Join moment annotation filters to exclude criteria
    GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
        conversation_event_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
    WHERE
        agent_user_id <> ''
        AND ((conversation_start_time >= '2024-01-01 00:00:00') AND (conversation_start_time < '2024-01-11 00:00:00'))
        AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL -- Additional filters
)
SELECT
  agent_user_id AS agent_user_id,
  conversation_id AS conversation_id,
  conversation_start_time AS conversation_start_time,
  event_type AS event_type,
  guided_workflow_name AS guided_workflow_name,
  doc_type AS doc_type,
  session_metadata_id AS session_metadata_id
FROM
    conversations JOIN gw_conversation_events USING (conversation_id)
)
SELECT
    DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
    agent_user_id,
    COUNT(DISTINCT IF(
        event_type = 4 AND 		-- 4: USED_KB_DOCUMENT
            doc_type = '2' AND 	-- 2: KB_DOCUMENT_TYPE_GUIDED_WORKFLOW
            guided_workflow_name <> '',
        session_metadata_id,
        NULL)) AS total_gw_used_count,
    COUNT(DISTINCT conversation_id) AS convo_count,
    COUNT(DISTINCT agent_user_id) AS user_count
FROM usage_rows
GROUP BY truncated_time, agent_user_id
