SELECT
  DATE_TRUNC('week', event_time + toIntervalDay(1) + toIntervalHour(-10)) - toIntervalDay(1) - toIntervalHour(-10) AS truncated_time,
  agent_user_id,
  COUNT(DISTINCT agent_user_id) AS manager_count,
  COUNT(DISTINCT event_id) AS director_pages_visit_count,
  COUNT(DISTINCT CASE
    WHEN JSONExtractString(toJSONString(event_payload), 'sessionLog', 'directorPageVisitPayload', 'directorPage') = 'LIVE_CONVERSATION'
    THEN event_id
    END
  ) AS director_live_conversation_page_visit_count,
  COUNT(DISTINCT CASE
    WHEN JSONExtractString(toJSONString(event_payload), 'sessionLog', 'directorPageVisitPayload', 'directorPage') = 'CLOSED_CONVERSATION'
    THEN event_id
    END
  ) AS director_closed_conversation_page_visit_count
FROM
  analytics_event_d
WHERE
  agent_user_id <> ''
  AND event_name = 'Director.SessionLog.Visit.Page'
  AND ((event_time >= '2021-01-01 00:00:00') AND (event_time < '2021-01-10 00:00:00'))	-- Additional filters
-- GROUP BY is optional.
GROUP BY truncated_time, agent_user_id
