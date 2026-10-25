WITH
		scorecard AS (
				SELECT
						scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
				FROM
						scorecard_d
							WHERE ((scorecard_time >= '2021-01-01 00:00:00') AND (scorecard_time < '2021-01-31 00:00:00')) 
		),
		scorecard_last_update AS (
				SELECT
						scorecard_id,
						max(scorecard_last_update_time) AS scorecard_last_update_time
				FROM
						scorecard
				GROUP BY
						scorecard_id
		),
		filtered_scorecard AS (
				SELECT DISTINCT
						scorecard.scorecard_id AS scorecard_id,scorecard.scorecard_last_update_time AS scorecard_last_update_time
				FROM scorecard
				JOIN scorecard_last_update ON scorecard.scorecard_id = scorecard_last_update.scorecard_id
						AND scorecard.scorecard_last_update_time = scorecard_last_update.scorecard_last_update_time
		),
		scorecard_score AS (
				SELECT
						DISTINCT scorecard_time, agent_user_id, conversation_id, scorecard_id, scorecard_last_update_time, score, scorecard_submit_time
				FROM
						scorecard_d
				WHERE
						((scorecard_time >= '2021-01-01 00:00:00') AND (scorecard_time < '2021-01-31 00:00:00') AND (score >= 0))
		)
SELECT
		agent_user_id,
		SUM(score/100.) FILTER (WHERE score >= 0) AS weighted_percentage_sum,
		SUM(1.) FILTER (WHERE score >= 0) AS weight_sum,
		COUNT(DISTINCT conversation_id) AS total_conversation_count,
		COUNT(DISTINCT scorecard_id) AS total_scorecard_count
FROM
		scorecard_score JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
				AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
-- GROUP BY is optional.
GROUP BY agent_user_id
SETTINGS max_bytes_before_external_group_by = 5000000000
