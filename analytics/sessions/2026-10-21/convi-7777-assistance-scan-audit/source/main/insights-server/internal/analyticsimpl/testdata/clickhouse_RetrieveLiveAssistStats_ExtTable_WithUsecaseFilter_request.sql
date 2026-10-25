WITH raised_hands_and_whispers AS (
  SELECT
    conversation_start_time,
    conversation_id,
    agent_user_id,
    -- The format of payload.manager_whisper.manager_user_id can be resource name.
    (
      CASE WHEN payload.manager_whisper.manager_user_id like '%/%' THEN substringIndex(
        payload.manager_whisper.manager_user_id,
        '/',-1
      ) ELSE payload.manager_whisper.manager_user_id END
    ) AS manager_user_id,
    MAX(
      CASE WHEN action_type = 7 THEN 1 ELSE 0 END
    ) AS has_raised_hand,
    -- 7: AGENT_RAISE_HAND
    MAX(
      CASE WHEN action_type = 12 THEN 1 ELSE 0 END
    ) AS has_whisper -- 12: MANAGER_WHISPER
  FROM
    action_annotation_d
  WHERE
    conversation_source IN (0, 8)
    AND action_type IN (7, 12)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND (
      (
        conversation_start_time >= '2021-01-01 00:00:00'
      )
      AND (
        conversation_start_time < '2021-01-10 00:00:00'
      )
      AND (
        agent_user_id IN (SELECT user_id FROM agent_filter)
        OR manager_user_id IN (SELECT user_id FROM agent_filter)
      )
      AND (
        usecase_id IN ('usecase_id')
      )
    ) -- Additional filters
  GROUP BY
    1,
    2,
    3,
    4
),
agent_live_assist_stats AS (
  SELECT
    DATE_TRUNC(
      'day',
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
    agent_user_id,
    COUNT(DISTINCT conversation_id) FILTER (
      WHERE
        has_raised_hand = 1
    ) AS raised_hand_count,
    COUNT(DISTINCT conversation_id) FILTER (
      WHERE
        has_raised_hand = 1
        AND has_whisper = 1
    ) AS raised_hand_answered_count,
    COUNT(DISTINCT conversation_id) FILTER (
      WHERE
        has_whisper = 1
    ) AS whispered_to_agent_count,
    0 AS whisper_by_manager_count
  FROM
    raised_hands_and_whispers
  GROUP BY
    truncated_time,
    agent_user_id
),
manager_live_assist_stats AS (
  SELECT
    DATE_TRUNC(
      'day',
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
    manager_user_id AS agent_user_id,
    0 AS raised_hand_count,
    0 AS raised_hand_answered_count,
    0 AS whispered_to_agent_count,
    COUNT(DISTINCT conversation_id) FILTER (
      WHERE
        has_whisper = 1
    ) AS whisper_by_manager_count
  FROM
    raised_hands_and_whispers
  GROUP BY
    truncated_time,
    agent_user_id
)
SELECT
  *
FROM
  agent_live_assist_stats
UNION ALL
SELECT
  *
FROM
  manager_live_assist_stats
