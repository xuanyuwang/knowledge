WITH convs AS (
        SELECT DATE_TRUNC(
                        'day',
                        conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
                ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
                agent_user_id,
                COUNT(DISTINCT agent_user_id) AS total_agent_count
        FROM conversation_d
        WHERE conversation_source in (0, 7)
                AND is_dev_user = 0
                AND agent_user_id <> ''
                AND (
                        (conversation_start_time >= '2021-01-01 00:00:00')
                        AND (conversation_start_time < '2021-01-10 00:00:00')
                        AND (agent_user_id IN ('1', '2', '3', '4'))
                ) -- Additional filters
                -- GROUP BY is optional
        GROUP BY truncated_time,
                agent_user_id
),
convs_with_aa AS (
        SELECT DATE_TRUNC(
                        'day',
                        IF(message_id != '', message_time, conversation_start_time) + toIntervalDay(0) + toIntervalHour(0)
                ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
                agent_user_id,
                COUNT(DISTINCT agent_user_id) AS active_agent_count
        FROM conversation_with_labels_d
        WHERE agent_user_id <> ''
                AND (has_agent_assistance = 1)
                AND ((agent_user_id IN ('1', '2', '3', '4')))
                AND (
                        (message_id != '' AND ((message_time >= '2021-01-01 00:00:00') AND (message_time < '2021-01-10 00:00:00')))
                        OR (message_id = '' AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00')))
                ) -- Additional filters
                -- GROUP BY is optional
        GROUP BY truncated_time,
                agent_user_id
)
SELECT truncated_time,
        agent_user_id,
        COALESCE(convs.total_agent_count, 0) AS total_agent_count,
        COALESCE(convs_with_aa.active_agent_count, 0) AS active_agent_count
FROM -- Using FULL OUTER JOIN to preserve rows from both CTEs.
        -- convs may miss days where agent only had non-qualifying source conversations.
        -- convs_with_aa captures agent assist from all sources, indicating agent login.
        convs
        FULL OUTER JOIN convs_with_aa USING (truncated_time, agent_user_id)
