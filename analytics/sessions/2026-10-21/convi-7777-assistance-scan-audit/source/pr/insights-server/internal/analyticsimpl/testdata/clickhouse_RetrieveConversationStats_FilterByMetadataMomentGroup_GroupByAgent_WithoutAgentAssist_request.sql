WITH eligible_population AS (
		WITH
			t1_moment_annotation_filter_0 AS (
				SELECT
					-- Rename the column to avoid ambiguous column issue.
					conversation_id,
					MAX(create_time) AS latest_create_time
				FROM
					moment_annotation_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_type = 19) AND (moment_template_id = 'moment_id_1'))
				GROUP BY conversation_id
			),
			t2_moment_annotation_filter_0 AS (
				SELECT
					conversation_id,
					create_time
				FROM
					moment_annotation_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_type = 19) AND (moment_template_id = 'moment_id_1') AND (metadata_string_value = 'metadata-value-1' OR metadata_string_value = 'metadata-value-2'))
			),
			moment_annotation_filter_0 AS (
				SELECT DISTINCT
					t1.conversation_id AS conversation_id_0
				FROM
					t1_moment_annotation_filter_0 AS t1
				JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_create_time = t2.create_time
			),
			t1_moment_annotation_filter_1 AS (
				SELECT
					-- Rename the column to avoid ambiguous column issue.
					conversation_id,
					MAX(create_time) AS latest_create_time
				FROM
					moment_annotation_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_type = 19) AND (moment_template_id = 'moment_id_2'))
				GROUP BY conversation_id
			),
			t2_moment_annotation_filter_1 AS (
				SELECT
					conversation_id,
					create_time
				FROM
					moment_annotation_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_type = 19) AND (moment_template_id = 'moment_id_2') AND ((metadata_number_value >= 0 AND metadata_number_value < 100) OR (metadata_number_value >= 100 AND metadata_number_value < 250)))
			),
			moment_annotation_filter_1 AS (
				SELECT DISTINCT
					t1.conversation_id AS conversation_id_1
				FROM
					t1_moment_annotation_filter_1 AS t1
				JOIN t2_moment_annotation_filter_1 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_create_time = t2.create_time
			),
			moment_annotation_exclude_filter_0 AS (
				SELECT DISTINCT
					-- Rename the column to avoid ambiguous column issue.
					conversation_id AS conversation_id_exclude_0
				FROM
					moment_annotation_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (moment_type = 19) AND (moment_template_id = 'moment_id_3'))
			),
		conv_with_aa AS (
			SELECT
				agent_user_id, conversation_id, conversation_start_time
			FROM
				conversation_d
			WHERE TRUE
				AND conversation_source IN (0, 7, 8)
				AND is_dev_user = 0
				AND agent_user_id <> ''
				AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00'))
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
				AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-10 00:00:00') AND (is_voice_mail <> 1))	-- Additional filters
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
			INNER JOIN moment_annotation_filter_0 ON
				conversation_population_per_pair.conversation_id = moment_annotation_filter_0.conversation_id_0
			INNER JOIN moment_annotation_filter_1 ON
				conversation_population_per_pair.conversation_id = moment_annotation_filter_1.conversation_id_1
			LEFT JOIN moment_annotation_exclude_filter_0 ON
				conversation_population_per_pair.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
		-- WHERE is optional.
		WHERE TRUE
			AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 = ''
		)
 SELECT DATE_TRUNC('day', conversation_time + toIntervalDay(0) + toIntervalHour(-10)) - toIntervalDay(0) - toIntervalHour(-10) AS truncated_time,
agent_user_id,
 COUNT(DISTINCT conversation_id) AS convo_count,
 COUNT(DISTINCT agent_user_id) AS user_count,
 SUM(aht_sec) AS aht_sec
 FROM eligible_population
 GROUP BY truncated_time, agent_user_id
