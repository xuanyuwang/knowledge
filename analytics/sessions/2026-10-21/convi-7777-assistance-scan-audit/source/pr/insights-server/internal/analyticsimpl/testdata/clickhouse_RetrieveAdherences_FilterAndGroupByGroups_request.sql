With sdx AS (
  SELECT
    DATE_TRUNC(
      'day',
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
    agent_user_id,
    COUNT(
      DISTINCT adherence_moment_annotation_id
    ) AS sdx_count,
    COUNT(DISTINCT conversation_id) AS convo_count
  FROM
    moment_annotation_d
  WHERE
    adherence_type IN (2, 3, 5) -- 2: ADHERENCE_TYPE_DID_DO_X, 3: ADHERENCE_TYPE_DID_NOT_DO_X, 5: ADHERENCE_TYPE_NO_OPPORTUNITY_TO_DO_X
    AND adherence_moment_adherence_type = 1 -- 1: ADHERENCE_TYPE_SHOULD_DO_X
    AND adherence_moment_annotation_id != ''
    AND conversation_source in (0, 8, 7)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND ((conversation_start_time >= '2021-01-01 00:00:00')
      AND (conversation_start_time < '2021-01-11 00:00:00')
      AND (agent_user_id IN ('1','2','3','4'))) -- Additional filters
  GROUP BY
    truncated_time,
    agent_user_id
),
ddx AS (
  SELECT
    DATE_TRUNC(
      'day',
      conversation_start_time + toIntervalDay(0) + toIntervalHour(0)
    ) - toIntervalDay(0) - toIntervalHour(0) AS truncated_time,
    agent_user_id,
    COUNT(DISTINCT moment_annotation_id) AS ddx_count
  FROM
    moment_annotation_d
  WHERE
    adherence_type = 2 -- 2: ADHERENCE_TYPE_DID_DO_X
    AND conversation_source in (0, 8, 7)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND ((conversation_start_time >= '2021-01-01 00:00:00')
      AND (conversation_start_time < '2021-01-11 00:00:00')
      AND (agent_user_id IN ('1','2','3','4'))) -- Additional filters
  GROUP BY
    truncated_time,
    agent_user_id
)
SELECT
  truncated_time,
  agent_user_id,
  SUM(sdx.convo_count) AS convo_count,
  SUM(sdx.sdx_count) AS sdx_count,
  SUM(ddx.ddx_count) AS ddx_count
FROM
  sdx
  LEFT JOIN ddx USING (
    truncated_time, agent_user_id
  )
GROUP BY
  truncated_time,
  agent_user_id
