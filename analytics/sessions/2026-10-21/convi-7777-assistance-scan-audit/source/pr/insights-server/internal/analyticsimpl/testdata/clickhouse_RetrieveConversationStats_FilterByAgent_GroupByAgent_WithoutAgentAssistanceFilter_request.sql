WITH eligible_population AS (
		WITH
		conv_with_aa AS (
				SELECT
					agent_user_id, conversation_id, conversation_start_time
				FROM conversation_d
				GLOBAL LEFT ANTI JOIN (
					SELECT DISTINCT conversation_id, agent_user_id
					FROM conversation_with_labels_d
					WHERE has_agent_assistance = true
					AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (agent_user_id IN ('2')))
				) AS assisted USING (conversation_id, agent_user_id)
				WHERE TRUE
					AND conversation_source IN (0, 7, 8)
					AND is_dev_user = 0
					AND agent_user_id <> ''
					AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (agent_user_id IN ('2')))
				ORDER BY update_time DESC
				LIMIT 1 BY agent_user_id, conversation_id
			),
        conv_with_aht AS (
			SELECT
				agent_user_id,
				conversation_id,
				MAX(handle_time_secs) AS aht_sec
			FROM
				message_d
			WHERE TRUE
				AND conversation_source IN (0, 7, 8)
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (agent_user_id IN ('2')) AND (is_voice_mail <> 1))	-- Additional filters
			GROUP BY agent_user_id, conversation_id
		),
		conversation_population_per_pair AS (
			SELECT *
			FROM conv_with_aht JOIN conv_with_aa ON conv_with_aht.conversation_id = conv_with_aa.conversation_id
				AND conv_with_aht.agent_user_id = conv_with_aa.agent_user_id
		)
		SELECT
			conversation_id, agent_user_id, conversation_start_time AS conversation_time, aht_sec
		FROM
			conversation_population_per_pair
		-- WHERE is optional.
		)
 SELECT DATE_TRUNC('day', conversation_time + toIntervalDay(0) + toIntervalHour(-10)) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time,
agent_user_id,
 COUNT(DISTINCT conversation_id) AS convo_count,
 COUNT(DISTINCT agent_user_id) AS user_count,
 SUM(aht_sec) AS aht_sec
 FROM eligible_population
 GROUP BY truncated_time, agent_user_id
