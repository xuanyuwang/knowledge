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
        conversation_start_time >= '2021-01-01 10:00:00'
      ) 
      AND (
        conversation_start_time < '2021-01-11 10:00:00'
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
        conversation_start_time >= '2021-01-01 10:00:00'
      ) 
      AND (
        conversation_start_time < '2021-01-11 10:00:00'
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
        conversation_start_time >= '2021-01-01 10:00:00'
      ) 
      AND (
        conversation_start_time < '2021-01-11 10:00:00'
      )
      AND (
        moment_template_id = 'moment_id_2'
      )
    )
),
conversation_count AS (
  SELECT
    DATE_TRUNC('day', 
    conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id,
    COUNT(DISTINCT conversation_id) AS convo_count,
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
    conversation_source in (0, 8)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND ((conversation_start_time >= '2021-01-01 10:00:00')
      AND (conversation_start_time < '2021-01-11 10:00:00')) -- Additional filters
    AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
  GROUP BY 
    truncated_time, agent_user_id
),
hint_followed AS(
  SELECT
    DATE_TRUNC('day', 
    conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id,
    COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count
  FROM
    moment_annotation_d
  GLOBAL INNER JOIN moment_annotation_filter_0 ON
    moment_annotation_d.conversation_id = moment_annotation_filter_0.conversation_id_0
  GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
    moment_annotation_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
  WHERE
    adherence_type = 2
    AND adherence_action_annotation_id <> ''
    AND (conversation_source in (0, 8) OR conversation_source IS NULL)
    AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND ((conversation_start_time >= '2021-01-01 10:00:00')
      AND (conversation_start_time < '2021-01-11 10:00:00')) -- Additional filters
    AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
  GROUP BY 
    truncated_time, agent_user_id
  UNION ALL
  SELECT
    DATE_TRUNC('day', 
    conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id,
    COUNT(DISTINCT action_annotation_id) as hint_followed_count
  FROM
    conversation_event_d
  GLOBAL INNER JOIN moment_annotation_filter_0 ON
    conversation_event_d.conversation_id = moment_annotation_filter_0.conversation_id_0
  GLOBAL LEFT JOIN moment_annotation_exclude_filter_0 ON
    conversation_event_d.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
  WHERE
    event_type = 9
    AND (conversation_source in (0, 8) OR conversation_source IS NULL)
    AND action_policy_source_type=1
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND ((conversation_start_time >= '2021-01-01 10:00:00')
      AND (conversation_start_time < '2021-01-11 10:00:00')) -- Additional filters
    AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 IS NULL
  GROUP BY 
    truncated_time, agent_user_id
)
SELECT
  truncated_time, 
  agent_user_id,
  MAX(conversation_count.convo_count) AS convo_count,
  MAX(conversation_count.user_count) AS user_count,
  SUM(hint_followed.hint_followed_count) AS hint_followed_count
FROM
  conversation_count LEFT JOIN hint_followed USING (truncated_time, agent_user_id)
GROUP BY 
  truncated_time, agent_user_id
