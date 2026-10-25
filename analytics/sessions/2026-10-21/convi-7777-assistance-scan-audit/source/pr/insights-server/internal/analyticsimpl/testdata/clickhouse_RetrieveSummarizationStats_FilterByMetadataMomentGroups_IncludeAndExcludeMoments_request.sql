WITH
-- Find the latest moment annotation for each conversation that matches non-metadata conditions
t1_moment_annotation_filter_0 AS (
  SELECT 
    conversation_id, 
    MAX(update_time) AS latest_time 
  FROM 
    moment_annotation_mv_by_metadata_d
  WHERE 
    (
      (
        conversation_start_time >= '2024-01-01 00:00:00'
      ) 
      AND (
        conversation_start_time < '2024-01-11 00:00:00'
      )
      AND (
        moment_template_id = 'moment_id_1'
      )
    ) 
  GROUP BY 
    conversation_id
), 
-- Get all moment annotations that match the complete conditions (including metadata values)
t2_moment_annotation_filter_0 AS (
  SELECT 
    conversation_id, 
    update_time 
  FROM 
    moment_annotation_mv_by_metadata_d
  WHERE 
    (
      (
        conversation_start_time >= '2024-01-01 00:00:00'
      ) 
      AND (
        conversation_start_time < '2024-01-11 00:00:00'
      )
      AND (
        moment_template_id = 'moment_id_1'
      ) 
      AND (
        metadata_string_value = 'include-value-1'
      )
    )
), 
-- Final filter: Only include conversations where the LATEST moment annotation matches metadata criteria
moment_annotation_filter_0 AS (
  SELECT 
    DISTINCT t1.conversation_id AS conversation_id_0
  FROM 
    t1_moment_annotation_filter_0 AS t1 
    JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id 
    AND t1.latest_time = t2.update_time
), 
moment_annotation_exclude_filter_0 AS (
  SELECT DISTINCT
    conversation_id AS conversation_id_exclude_0
  FROM
    moment_annotation_mv_by_metadata_d
  WHERE
    (
      (
        conversation_start_time >= '2024-01-01 00:00:00'
      ) 
      AND (
        conversation_start_time < '2024-01-11 00:00:00'
      )
      AND (
        moment_template_id = 'moment_id_2'
      )
    )
),
conversation_stats AS (
  SELECT 
    DATE_TRUNC('day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
      ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id,
    COUNT(DISTINCT conversation_id) AS convo_count,
    COUNT(DISTINCT agent_user_id) AS user_count
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
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-11 00:00:00')) -- Additional filters
    AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
  -- GROUP BY is optional.
  GROUP BY truncated_time, agent_user_id
),
convo_with_valid_summarization AS (
  SELECT DISTINCT 
    conversation_id, conversation_start_time, agent_user_id
  FROM
    action_annotation_d
  -- Join moment annotation filters to include criteria
  GLOBAL INNER JOIN moment_annotation_filter_0 ON
    action_annotation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
  -- Join moment annotation filters to exclude criteria
  GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
    action_annotation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
  WHERE 
    agent_user_id <> ''
    AND summarization_payload_is_valid_summary = 1
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-11 00:00:00')) -- Additional filters
    AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
),
convo_with_used_summarization AS (
  SELECT DISTINCT 
    conversation_id, conversation_start_time, agent_user_id
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
    AND event_type = 16 -- 16: USED_SUMMARIZATION
    AND summarization_payload_data_type IN (1, 2)  -- 1: CopyAllNotes, 2: CopySummarizationNote
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-11 00:00:00')) -- Additional filters
    AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
),
summarization_stats AS (
  SELECT
    DATE_TRUNC('day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
      ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id,
    COUNT(DISTINCT convo_with_valid_summarization.conversation_id) AS generated_summarization_count,
    COUNT(DISTINCT CASE WHEN convo_with_used_summarization.conversation_id <> '' THEN convo_with_used_summarization.conversation_id END) AS used_summarization_count
  FROM
    convo_with_valid_summarization
    LEFT JOIN convo_with_used_summarization USING (conversation_id, conversation_start_time, agent_user_id)
  -- GROUP BY is optional.
  GROUP BY truncated_time, agent_user_id
)
SELECT 
  truncated_time, agent_user_id,
  SUM(generated_summarization_count) AS generated_summarization_count, 
  SUM(used_summarization_count) AS used_summarization_count,
  MAX(convo_count) AS convo_count, 
  MAX(user_count) AS user_count
FROM 
  conversation_stats LEFT JOIN summarization_stats USING (truncated_time, agent_user_id)
-- GROUP BY is optional.
GROUP BY truncated_time, agent_user_id
