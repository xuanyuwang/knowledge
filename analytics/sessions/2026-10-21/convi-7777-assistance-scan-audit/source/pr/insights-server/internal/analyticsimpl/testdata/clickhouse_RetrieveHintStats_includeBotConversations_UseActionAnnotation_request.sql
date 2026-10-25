WITH hint_sent AS (
    -- behavioral hint
    SELECT
        DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
agent_user_id,
        COUNT(DISTINCT action_annotation_id) AS hint_sent_count,
        COUNT(DISTINCT conversation_id) AS convo_count,
        COUNT(DISTINCT agent_user_id) AS user_count
    FROM
        action_annotation_d
    WHERE action_annotation_id GLOBAL IN (
        SELECT adherence_action_annotation_id
        FROM moment_annotation_d
        WHERE adherence_type IN (2, 3)
            AND adherence_action_annotation_id <> ''
            AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00'))
    )
        AND (conversation_source in (0, 8, 7) OR conversation_source IS NULL)
        AND policy_source_type = 1
        AND is_dev_user = 0
        AND agent_user_id <> ''
        AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00')) -- Additional filters
    GROUP BY truncated_time, agent_user_id
    UNION ALL
    -- non-behavior hint.
    SELECT
        DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
agent_user_id,
        COUNT(DISTINCT action_annotation_id) AS hint_sent_count,
        COUNT(DISTINCT conversation_id) AS convo_count,
        COUNT(DISTINCT agent_user_id) AS user_count
    FROM
        action_annotation_d
    WHERE action_type in (1,6,13,14) -- 1: regular+behavioral hint, 6: manager alert, 13: GW hint, 14: KB hint
        AND (conversation_source in (0, 8, 7) OR conversation_source IS NULL)
        AND policy_source_type=1
        AND is_dev_user = 0
        AND agent_user_id <> ''
        AND adherence_type = 0
        AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00')) -- Additional filters
    GROUP BY truncated_time, agent_user_id
),
combined_hint_sent AS (
    SELECT
        truncated_time,
agent_user_id,
        -- Here max rather than sum is used because hint_sent already combines behavioral and non-behavioral hints in one single row.
        MAX(hint_sent.convo_count) AS convo_count,
        MAX(hint_sent.user_count) AS user_count,
        MAX(hint_sent.hint_sent_count) AS hint_sent_count
    FROM
        hint_sent
    GROUP BY truncated_time, agent_user_id
),
-- hint_followed_count
hint_followed AS(
    -- behavioral hint followed including checklist hint.
    SELECT
        DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
agent_user_id,
        COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count
    FROM
        moment_annotation_d
    WHERE
        adherence_type = 2
        AND adherence_action_annotation_id <> ''
        AND (conversation_source in (0, 8, 7) OR conversation_source IS NULL)
        AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
        AND is_dev_user = 0
        AND agent_user_id <> ''
        AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00')) -- Additional filters
    GROUP BY truncated_time, agent_user_id
    UNION ALL
    -- GW|KB hint followed
    SELECT
        DATE_TRUNC('day', conversation_start_time + toIntervalDay(0) + toIntervalHour(0)) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
agent_user_id,
        COUNT(DISTINCT action_annotation_id) as hint_followed_count
    FROM
        conversation_event_d
    WHERE
        event_type = 9
        AND (conversation_source in (0, 8, 7) OR conversation_source IS NULL)
        AND action_policy_source_type=1
        AND is_dev_user = 0
        AND agent_user_id <> ''
        AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00')) -- Additional filters
    GROUP BY truncated_time, agent_user_id
)
SELECT
    truncated_time,
    agent_user_id,
    -- Here max rather than sum is used because combined_hint_sent already combines behavioral and non-behavioral hints in one single row.
    MAX(combined_hint_sent.convo_count) AS convo_count,
    MAX(combined_hint_sent.user_count) AS user_count,
    MAX(combined_hint_sent.hint_sent_count) AS hint_sent_count,
    SUM(hint_followed.hint_followed_count) AS hint_followed_count
FROM
    combined_hint_sent
    LEFT JOIN hint_followed USING (truncated_time, agent_user_id)
GROUP BY truncated_time, agent_user_id