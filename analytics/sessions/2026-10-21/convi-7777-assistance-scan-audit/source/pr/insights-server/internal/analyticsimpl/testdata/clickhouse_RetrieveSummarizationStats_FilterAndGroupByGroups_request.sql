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
      AND (agent_user_id IN ('1', '2', '3', '4'))) -- Additional filters
  -- GROUP BY is optional.
  GROUP BY truncated_time, agent_user_id
),
convo_with_valid_summarization AS (
  SELECT DISTINCT 
    conversation_id, conversation_start_time, agent_user_id
  FROM
    action_annotation_d
  WHERE 
    agent_user_id <> ''
    AND summarization_payload_is_valid_summary = 1
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2', '3', '4'))) -- Additional filters
),
convo_with_used_summarization AS (
  SELECT DISTINCT 
    conversation_id, conversation_start_time, agent_user_id
  FROM
    conversation_event_d
  WHERE
    agent_user_id <> ''
    AND event_type = 16 -- 16: USED_SUMMARIZATION
    AND summarization_payload_data_type IN (1, 2)  -- 1: CopyAllNotes, 2: CopySummarizationNote
    AND ((conversation_start_time >= '2024-01-01 00:00:00')
      AND (conversation_start_time < '2024-01-10 00:00:00')
      AND (agent_user_id IN ('1', '2', '3', '4'))) -- Additional filters
),
summarization_stats AS (
  SELECT
    DATE_TRUNC('day', 
      conversation_start_time + toIntervalDay(0) + toIntervalHour(-10)
      ) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time, 
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
