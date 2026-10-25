WITH 
conversation_stats AS (
  SELECT 
    DATE_TRUNC('day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(-10)
      ) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time, 
    agent_user_id,
    COUNT(DISTINCT conversation_id) AS convo_count,
    COUNT(DISTINCT agent_user_id) AS user_count
  FROM 
    conversation_d
  WHERE
    agent_user_id <> ''
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2', '3', '4'))
      AND (usecase_id IN ('test_usecase')))  -- Additional filters
  -- GROUP BY is optional.
  GROUP BY truncated_time, agent_user_id
),
agent_hourly_conversation AS (
  SELECT DISTINCT 
    agent_user_id, 
    DATE_TRUNC('hour', conversation_start_time) AS event_time
  FROM
    conversation_d
  WHERE 
    agent_user_id <> ''
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2', '3', '4'))
      AND (usecase_id IN ('test_usecase')))  -- Additional filters
), 
agent_hourly_smart_compose_stats AS (
  SELECT 
    agent_user_id,
    DATE_TRUNC('hour', event_time) AS event_time,
    COUNT(DISTINCT 
      DATE_TRUNC('hour', event_time),
      JSONExtractString(JSONExtractString(toJSONString(event_payload), 'smartComposeInserted'), 'smartComposeText')
    ) AS smart_compose_used_count
  FROM 
    analytics_event_d
  WHERE TRUE
    AND event_name = 'ChatAgentAssist.SmartCompose.Insert.SmartComposeText'
    AND agent_user_id <> ''
    AND ((event_time >= '2024-01-01 00:00:00')
      AND (event_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2', '3', '4'))
      AND (true))  -- Additional filters
  GROUP BY agent_user_id, event_time
),
smart_compose_stats AS (
  SELECT 
    DATE_TRUNC('day', 
      event_time + toIntervalDay(0) + toIntervalHour(-10)
      ) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time, 
    agent_user_id, 
    SUM(smart_compose_used_count) AS total_smart_compose_used_count
  FROM 
    agent_hourly_smart_compose_stats 
      JOIN agent_hourly_conversation USING (agent_user_id, event_time)
  -- GROUP BY is optional.
  GROUP BY truncated_time, agent_user_id
)
SELECT 
  truncated_time, agent_user_id,
  SUM(total_smart_compose_used_count),
  MAX(convo_count), MAX(user_count)
FROM 
  conversation_stats LEFT JOIN smart_compose_stats USING (truncated_time, agent_user_id)
-- GROUP BY is optional.
GROUP BY truncated_time, agent_user_id