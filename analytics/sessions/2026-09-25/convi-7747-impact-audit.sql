-- Read-only ClickHouse impact audit, run against comcast_prod on 2026-09-25.
-- Pre-Aug-20 America/New_York Active Days: per-message rows use message_time;
-- legacy rows with message_id='' use conversation_start_time.
-- Latest conversation ID is a ClickHouse proxy for current PG ownership.
WITH current_conversations AS
(
    SELECT conversation_id,
           argMax(agent_user_id, update_time) AS current_agent_id,
           argMax(conversation_source, update_time) AS source,
           argMax(is_dev_user, update_time) AS dev,
           argMax(conversation_start_time, update_time) AS start_time
    FROM conversation_d FINAL
    WHERE customer_id = 'comcast' AND profile_id = 'prod' AND usecase_id = 'acquisition'
      AND conversation_start_time >= toDateTime64('2026-06-01 00:00:00', 6)
      AND conversation_start_time < toDateTime64('2026-08-21 00:00:00', 6)
    GROUP BY conversation_id
)
SELECT count() AS label_rows,
       uniqExact(l.conversation_id) AS labeled_conversations,
       countIf(l.agent_user_id != c.current_agent_id) AS mismatched_rows,
       uniqExactIf(l.conversation_id, l.agent_user_id != c.current_agent_id) AS mismatched_conversations,
       uniqExactIf(c.current_agent_id, l.agent_user_id != c.current_agent_id) AS affected_current_agents,
       countIf(l.agent_user_id != c.current_agent_id AND l.has_agent_assistance = 0) AS mismatched_false_rows,
       countIf(l.agent_user_id != c.current_agent_id AND l.message_id = '') AS mismatched_legacy_rows
FROM
(
    SELECT conversation_id, agent_user_id, message_id, has_agent_assistance,
           if(message_id != '', message_time, conversation_start_time) AS activity_time
    FROM conversation_with_labels_d FINAL
    WHERE customer_id = 'comcast' AND profile_id = 'prod' AND usecase_id = 'acquisition'
      AND if(message_id != '', message_time, conversation_start_time)
          < toDateTime64('2026-08-20 04:00:00', 6)
) AS l
INNER JOIN current_conversations AS c ON l.conversation_id = c.conversation_id;

-- To evaluate daily display exposure, group true labels by
-- (current_agent_id, toDate(toTimeZone(activity_time, 'America/New_York'))).
-- Count days with old-ID true labels and no current-ID true label, then join to
-- current_conversations grouped by source=0, dev=0, and local start day.
-- Result: 16,171 unmatched true-label agent-days; 16,062 also have a source-0
-- conversation starting that day (1,004 current agents). For Aug 8-19 alone:
-- 4,343 unmatched true-label agent-days; 4,325 have a same-day source-0
-- conversation (846 current agents).
