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
      AND (conversation_start_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2'))
      AND (conversation_duration_secs >= 10)
      AND (conversation_duration_secs < 100))  -- Additional filters
),
gw_conversation_events AS (
  SELECT DISTINCT
    agent_user_id,
    conversation_id,
    conversation_start_time,
    event_type,
    event_name,
    JSONExtractString(
      JSONExtractString(
        toJSONString(event_payload), 
        'usedKbDocument'), 
      'document') AS guided_workflow_name,
    JSONExtractString(
      JSONExtractString(
        toJSONString(event_payload),
        'usedKbDocument'),
      'docType') AS doc_type,
    JSONExtractString(
      JSONExtractString(
        JSONExtractString(
          toJSONString(event_payload), 
          'usedKbDocument'), 
        'sessionMetadata'), 
      'sessionId') AS session_metadata_id
  FROM
    conversation_event_d
  WHERE
    agent_user_id <> ''
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2'))
      AND (true))  -- Additional filters
)
SELECT
  agent_user_id AS agent_user_id,
  conversation_id AS conversation_id,
  conversation_start_time AS conversation_start_time,
  event_type AS event_type,
  event_name AS event_name,
  guided_workflow_name AS guided_workflow_name,
  doc_type AS doc_type,
  session_metadata_id AS session_metadata_id
FROM
  conversations JOIN gw_conversation_events USING (conversation_id)
)
SELECT
  DATE_TRUNC('day', 
    conversation_start_time + toIntervalDay(0) + toIntervalHour(-10)
    ) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time, 
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
GROUP BY 
  truncated_time, agent_user_id
