SELECT
  DATE_TRUNC('day', 
    scorecard_submit_time + toIntervalDay(0) + toIntervalHour(-10)
    ) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time, 
  submitter_user_id,
  COUNT(DISTINCT scorecard_id) AS scorecard_completed,
  COUNT(DISTINCT submitter_user_id) AS user_count
FROM
  scorecard_d
WHERE
  submitter_user_id <> ''
  AND ((scorecard_submit_time >= '2024-01-01 00:00:00')
    AND (scorecard_submit_time < '2024-01-10 00:00:00')
    AND (submitter_user_id IN ('1'))) -- Additional filters
-- GROUP BY is optional.
GROUP BY 
  truncated_time, submitter_user_id