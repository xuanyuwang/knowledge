WITH usage_rows AS (
WITH 
conversations AS (
  SELECT DISTINCT
    conversation_id
  FROM 
    conversation_d
  WHERE 
    agent_user_id <> ''
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00'))  -- Additional filters
),
conversation_events AS (
  SELECT DISTINCT
    agent_user_id,
    conversation_id,
    conversation_start_time,
    event_type,
    event_name,
    event_id
  FROM
    conversation_event_d
  WHERE
    agent_user_id <> ''
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00'))  -- Additional filters
)
SELECT
  agent_user_id AS agent_user_id,
  conversation_id AS conversation_id,
  conversation_start_time AS conversation_start_time,
  event_type AS event_type,
  event_name AS event_name,
  event_id AS event_id
FROM
  conversations JOIN conversation_events USING (conversation_id)
)
SELECT
  DATE_TRUNC('week', 
    conversation_start_time + toIntervalDay(1) + toIntervalHour(0)
    ) - toIntervalDay(1) - toIntervalHour(0) AS truncated_time, 
  agent_user_id,
  COUNT(DISTINCT IF(
    event_type = 5 AND -- 5: USED_NOTE_TAKING
      event_name = 'VoiceAgentAssist.NoteTaking.Click.CopyEntity',
    event_id,
    NULL)) AS total_note_taking_used_count,
  COUNT(DISTINCT conversation_id) AS convo_count,
  COUNT(DISTINCT agent_user_id) AS user_count
FROM usage_rows
GROUP BY 
  truncated_time, agent_user_id