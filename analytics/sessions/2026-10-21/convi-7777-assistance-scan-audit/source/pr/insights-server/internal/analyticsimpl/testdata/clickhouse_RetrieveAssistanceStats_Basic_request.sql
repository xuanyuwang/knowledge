WITH 
conversation_count AS (
  SELECT 
    DATE_TRUNC(
      'day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id, 
    COUNT(DISTINCT conversation_id) AS convo_count, 
    COUNT(DISTINCT agent_user_id) AS user_count 
  FROM 
    message_d 
  WHERE 
    conversation_source in (0, 8) 
    AND is_dev_user = 0 
    AND agent_user_id <> '' 
    AND ((conversation_start_time >= '2021-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-01 00:00:00')
      AND (agent_user_id IN ('1', '2'))
      AND (conversation_duration_secs >= 10)
      AND (conversation_duration_secs < 100)) -- Additional filters
  GROUP BY 
    truncated_time, 
    agent_user_id
), 
hint_followed AS(
  SELECT 
    DATE_TRUNC(
      'day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id, 
    COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count 
  FROM 
    moment_annotation_d 
  WHERE 
    adherence_type = 2 
    AND adherence_action_annotation_id <> '' 
    AND (
      conversation_source in (0, 8) 
      OR conversation_source IS NULL
    ) 
    AND adherence_action_policy_source_type = 1 -- TODO: add policy_source_type into moment_annotation table
    AND is_dev_user = 0 
    AND agent_user_id <> '' 
    AND ((conversation_start_time >= '2021-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-01 00:00:00')
      AND (agent_user_id IN ('1', '2'))
      AND (conversation_duration_secs >= 10)
      AND (conversation_duration_secs < 100)) -- Additional filters
  GROUP BY 
    truncated_time, 
    agent_user_id 
  UNION ALL 
  SELECT 
    DATE_TRUNC(
      'day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time, 
    agent_user_id, 
    COUNT(DISTINCT action_annotation_id) as hint_followed_count 
  FROM 
    conversation_event_d 
  WHERE 
    event_type = 9 
    AND (
      conversation_source in (0, 8) 
      OR conversation_source IS NULL
    ) 
    AND action_policy_source_type = 1 
    AND is_dev_user = 0 
    AND agent_user_id <> '' 
    AND ((conversation_start_time >= '2021-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-01 00:00:00')
      AND (agent_user_id IN ('1', '2'))
      AND (conversation_duration_secs >= 10)
      AND (conversation_duration_secs < 100)) -- Additional filters
  GROUP BY 
    truncated_time, 
    agent_user_id
) 
SELECT 
  truncated_time, 
  agent_user_id, 
  MAX(conversation_count.convo_count) AS convo_count, 
  MAX(conversation_count.user_count) AS user_count, 
  SUM(
    hint_followed.hint_followed_count
  ) AS hint_followed_count 
FROM 
  conversation_count 
  LEFT JOIN hint_followed USING (truncated_time, agent_user_id) 
GROUP BY 
  truncated_time, 
  agent_user_id
