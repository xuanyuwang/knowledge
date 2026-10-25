WITH suggestion_event_rows AS (
SELECT conversation_event_d.conversation_id AS conversation_id, conversation_event_d.agent_user_id AS agent_user_id, conversation_start_time, event_id, event_type
FROM
  conversation_event_d
WHERE
  agent_user_id <> ''
  AND ((conversation_start_time >= '2024-01-01 00:00:00')
    AND (conversation_start_time < '2024-01-10 00:00:00')) -- Additional filters

)
SELECT
  DATE_TRUNC('week', 
  conversation_start_time + toIntervalDay(1) + toIntervalHour(0)
  ) - toIntervalDay(1) - toIntervalHour(0) AS truncated_time, 
  agent_user_id,
  COUNT(DISTINCT IF(event_type IN (4, 5, 10), event_id, NULL)) AS total_suggestions_count, -- 4: USED_SUGGESTION; 5: USED_HINT_SUGGESTION; 10: USED_SEARCH_SUGGESTION
  COUNT(DISTINCT IF(event_type IN (25), event_id, NULL)) AS total_email_piecemeal_suggestions_count, -- 25: EMAIL_USED_PIECEMEAL_SUGGESTION in sql-schema apiserver/sql-schema/protos/event/event.proto
  COUNT(DISTINCT IF(event_type IN (27), event_id, NULL)) AS total_email_full_reply_suggestions_count, -- 27: EMAIL_USED_FULL_REPLY_SUGGESTION in sql-schema apiserver/sql-schema/protos/event/event.proto
  COUNT(DISTINCT conversation_id) AS convo_count,
  COUNT(DISTINCT agent_user_id) AS user_count
FROM suggestion_event_rows
GROUP BY 
  truncated_time, agent_user_id
