WITH

t1_moment_annotation_filter_0 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '01964a99-d3ce-7b0d-823f-b7dbb50195c2'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_0 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '01964a99-d3ce-7b0d-823f-b7dbb50195c2') AND ((outcome_value_type = 'number' AND outcome_number_value >= -1 AND outcome_number_value <= -1) OR (outcome_value_type = 'number' AND outcome_number_value >= 0 AND outcome_number_value <= 6) OR (outcome_value_type = 'number' AND outcome_number_value > 6 AND outcome_number_value <= 10)))
),
moment_annotation_filter_0 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_0
  FROM t1_moment_annotation_filter_0 AS t1
  JOIN t2_moment_annotation_filter_0 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),

t1_moment_annotation_filter_1 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '0199e72b-551b-74ca-b5de-5c1d9491a983'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_1 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '0199e72b-551b-74ca-b5de-5c1d9491a983') AND ((outcome_value_type = 'number' AND outcome_number_value >= -1 AND outcome_number_value <= -1) OR (outcome_value_type = 'number' AND outcome_number_value >= 0 AND outcome_number_value <= 6) OR (outcome_value_type = 'number' AND outcome_number_value > 6 AND outcome_number_value <= 10)))
),
moment_annotation_filter_1 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_1
  FROM t1_moment_annotation_filter_1 AS t1
  JOIN t2_moment_annotation_filter_1 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),

t1_moment_annotation_filter_2 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '7252ec08-0620-4c83-93aa-6ccc84b6750b'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_2 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '7252ec08-0620-4c83-93aa-6ccc84b6750b') AND ((outcome_value_type = 'string' AND outcome_string_value = 'resolved') OR (outcome_value_type = 'string' AND outcome_string_value = 'unresolved') OR (outcome_value_type = 'string' AND outcome_string_value = 'not_care')))
),
moment_annotation_filter_2 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_2
  FROM t1_moment_annotation_filter_2 AS t1
  JOIN t2_moment_annotation_filter_2 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),

t1_moment_annotation_filter_3 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '921efbb7-7797-4dcb-874e-16103e1b5102'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_3 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-02-02 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '921efbb7-7797-4dcb-874e-16103e1b5102') AND ((outcome_value_type = 'boolean' AND outcome_bool_value = true) OR (outcome_value_type = 'boolean' AND outcome_bool_value = false)))
),
moment_annotation_filter_3 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_3
  FROM t1_moment_annotation_filter_3 AS t1
  JOIN t2_moment_annotation_filter_3 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),
scorecard AS (
  SELECT scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
  FROM scorecard_d
  WHERE ((scorecard_time >= '2026-02-02 04:00:00') AND (scorecard_time < '2026-08-01 03:59:59') AND (usecase_id IN ('walter-dev')))
),
scorecard_last_update AS (
  SELECT scorecard_id, max(scorecard_last_update_time) AS scorecard_last_update_time
  FROM scorecard GROUP BY scorecard_id
),
filtered_scorecard AS (
  SELECT DISTINCT scorecard.scorecard_id AS scorecard_id, scorecard.scorecard_last_update_time AS scorecard_last_update_time
  FROM scorecard
  JOIN scorecard_last_update ON scorecard.scorecard_id = scorecard_last_update.scorecard_id
    AND scorecard.scorecard_last_update_time = scorecard_last_update.scorecard_last_update_time
),
scorecard_score AS (
  SELECT DISTINCT scorecard_time, agent_user_id, conversation_id, scorecard_id, scorecard_last_update_time, criterion_id, percentage_value, float_weight, scorecard_submit_time, manually_scored, scorecard_publish_time
  FROM score_d
  WHERE ((scorecard_time >= '2026-02-02 04:00:00') AND (scorecard_time < '2026-08-01 03:59:59') AND (usecase_id IN ('walter-dev')) AND (percentage_value >= 0) AND (not_applicable <> true))
),
scorecard_score_per_conversation AS (
  SELECT
    conversation_id,
    scorecard_id,
    DATE_TRUNC('day', scorecard_time + toIntervalDay(0) + toIntervalHour(-4)) - toIntervalDay(0) - toIntervalHour(-4) AS truncated_time,
    SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
    SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum
  FROM scorecard_score
  JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
    AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
  GROUP BY conversation_id, scorecard_id, truncated_time
)
SELECT
  truncated_time,
  SUM(weighted_percentage_sum) AS weighted_percentage_sum,
  SUM(weight_sum) AS weight_sum,
  COUNT(DISTINCT conversation_id) AS total_conversation_count,
  COUNT(DISTINCT scorecard_id) AS total_scorecard_count
FROM scorecard_score_per_conversation
INNER JOIN moment_annotation_filter_0 ON
  scorecard_score_per_conversation.conversation_id = moment_annotation_filter_0.conversation_id_0
INNER JOIN moment_annotation_filter_1 ON
  scorecard_score_per_conversation.conversation_id = moment_annotation_filter_1.conversation_id_1
INNER JOIN moment_annotation_filter_2 ON
  scorecard_score_per_conversation.conversation_id = moment_annotation_filter_2.conversation_id_2
INNER JOIN moment_annotation_filter_3 ON
  scorecard_score_per_conversation.conversation_id = moment_annotation_filter_3.conversation_id_3
GROUP BY truncated_time
ORDER BY truncated_time
SETTINGS max_bytes_before_external_group_by = 5000000000
