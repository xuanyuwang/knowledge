-- Read-only ClickHouse audit run against comcast_prod, 2026-09-25.
-- America/New_York message days Aug 18-22; exact conversation-ID join.
SELECT
    toDate(toTimeZone(l.message_time, 'America/New_York')) AS day,
    count() AS labeled_rows,
    uniqExact(l.conversation_id) AS conversations,
    uniqExactIf(l.conversation_id, l.agent_user_id != c.current_agent_id) AS mismatch_conversations,
    uniqExactIf(l.agent_user_id, l.agent_user_id != c.current_agent_id) AS old_agents
FROM
(
    SELECT conversation_id, agent_user_id, message_time
    FROM conversation_with_labels_d FINAL
    WHERE customer_id = 'comcast' AND profile_id = 'prod' AND usecase_id = 'acquisition'
      AND has_agent_assistance = 1 AND message_id != ''
      AND message_time >= toDateTime64('2026-08-18 04:00:00', 6)
      AND message_time < toDateTime64('2026-08-23 04:00:00', 6)
) AS l
INNER JOIN
(
    SELECT conversation_id, argMax(agent_user_id, update_time) AS current_agent_id
    FROM conversation_d FINAL
    WHERE customer_id = 'comcast' AND profile_id = 'prod' AND usecase_id = 'acquisition'
      AND conversation_start_time >= toDateTime64('2026-08-18 04:00:00', 6)
      AND conversation_start_time < toDateTime64('2026-08-23 04:00:00', 6)
    GROUP BY conversation_id
) AS c ON l.conversation_id = c.conversation_id
GROUP BY day ORDER BY day;

-- Observed mapping uniqueness across Aug 18-19; not a full merge manifest.
SELECT count() AS old_ids,
       countIf(targets != 1) AS ambiguous_old_ids,
       countIf(conversations < 2) AS single_conversation_old_ids,
       sum(conversations) AS matched_conversations
FROM
(
    SELECT old_agent_id,
           uniqExact(current_agent_id) AS targets,
           uniqExact(conversation_id) AS conversations
    FROM
    (
        SELECT l.agent_user_id AS old_agent_id,
               l.conversation_id AS conversation_id,
               c.current_agent_id AS current_agent_id
        FROM
        (
            SELECT DISTINCT conversation_id, agent_user_id
            FROM conversation_with_labels_d FINAL
            WHERE customer_id = 'comcast' AND profile_id = 'prod' AND usecase_id = 'acquisition'
              AND has_agent_assistance = 1 AND message_id != ''
              AND message_time >= toDateTime64('2026-08-18 04:00:00', 6)
              AND message_time < toDateTime64('2026-08-20 04:00:00', 6)
        ) AS l
        INNER JOIN
        (
            SELECT conversation_id, argMax(agent_user_id, update_time) AS current_agent_id
            FROM conversation_d FINAL
            WHERE customer_id = 'comcast' AND profile_id = 'prod' AND usecase_id = 'acquisition'
              AND conversation_start_time >= toDateTime64('2026-08-18 04:00:00', 6)
              AND conversation_start_time < toDateTime64('2026-08-20 04:00:00', 6)
            GROUP BY conversation_id
        ) AS c ON l.conversation_id = c.conversation_id
        WHERE l.agent_user_id != c.current_agent_id
    )
    GROUP BY old_agent_id
);
