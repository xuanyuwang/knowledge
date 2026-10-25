WITH knowledge_base_event_rows AS (
SELECT conversation_event_d.conversation_id AS conversation_id, conversation_event_d.agent_user_id AS agent_user_id, conversation_start_time, event_id, event_type
FROM
  conversation_event_d
WHERE
  agent_user_id <> ''
  AND ((conversation_start_time >= '2024-01-01 00:00:00')
    AND (conversation_start_time < '2024-01-10 00:00:00')
    AND (agent_user_id IN ('1', '2')))  -- Additional filters
)
SELECT
  DATE_TRUNC('day',
  conversation_start_time + toIntervalDay(0) + toIntervalHour(-10)
  ) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time,
  agent_user_id,
  COUNT(DISTINCT IF(event_type = 13, event_id, NULL)) AS total_kb_search_count,
  COUNT(DISTINCT conversation_id) AS convo_count,
  COUNT(DISTINCT agent_user_id) AS user_count
FROM knowledge_base_event_rows
GROUP BY
  truncated_time, agent_user_id
