CREATE TABLE moment_annotation_mv_by_metadata_d (conversation_id String, conversation_start_time DateTime64(6), moment_template_id String, metadata_string_value String, metadata_number_value Float64, metadata_bool_value Bool, update_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE scorecard_d (scorecard_id String, scorecard_last_update_time DateTime64(6), conversation_id String, scorecard_submit_time DateTime64(6), manually_scored Bool, publish_time DateTime64(6), scorecard_time DateTime64(6), scorecard_template_id String) ENGINE=Memory;
CREATE TABLE score_d (conversation_id String, conversation_start_time DateTime64(6), agent_user_id String, scorecard_template_id String, scorecard_template_revision Int32, scorecard_id String, score_id String, criterion_id String, manually_scored Bool, numeric_value Float64, max_value Float64, percentage_value Float64, weight Float64, float_weight Float64, ai_value Float64, ai_scored Bool, auto_failed Bool, not_applicable Bool, scorecard_last_update_time DateTime64(6), scorecard_time DateTime64(6), scorecard_submit_time DateTime64(6), scorecard_publish_time DateTime64(6)) ENGINE=Memory;
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-selected','selected','2021-01-05','2020-12-28','2021-01-05');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight,agent_user_id) VALUES ('s-selected','selected','2021-01-05','2020-12-28','2021-01-05','2020-12-28',80,1,'agent');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-missing','missing','2021-01-05','2020-12-28','2021-01-05');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight,agent_user_id) VALUES ('s-missing','missing','2021-01-05','2020-12-28','2021-01-05','2020-12-28',80,1,'agent');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-other','other','2021-01-05','2020-12-28','2021-01-05');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight,agent_user_id) VALUES ('s-other','other','2021-01-05','2020-12-28','2021-01-05','2020-12-28',80,1,'agent');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-excluded','excluded','2021-01-05','2020-12-28','2021-01-05');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight,agent_user_id) VALUES ('s-excluded','excluded','2021-01-05','2020-12-28','2021-01-05','2020-12-28',80,1,'agent');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-outside','outside','2021-01-05','2020-12-28','2020-12-28');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight,agent_user_id) VALUES ('s-outside','outside','2021-01-05','2020-12-28','2020-12-28','2020-12-28',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('selected','2020-12-28','include','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('selected','2020-12-28','overlap','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('missing','2020-12-28','include','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('other','2020-12-28','include','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('other','2020-12-28','overlap','other',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('excluded','2020-12-28','include','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('excluded','2020-12-28','overlap','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('excluded','2020-12-28','exclude','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('outside','2020-12-28','include','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('outside','2020-12-28','overlap','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('outside','2020-12-28','exclude','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('no-score','2020-12-28','include','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('no-score','2020-12-28','overlap','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('no-score','2020-12-28','exclude','selected',0,false,'2021-01-05');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('other','2020-12-28','overlap','selected',0,false,'2021-01-01');

		WITH
		
		t1_moment_annotation_filter_0 AS (
			SELECT
				-- Rename the column to avoid ambiguous column issue.
				conversation_id,
				MAX(update_time) AS latest_update_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'include') AND (conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)))
			GROUP BY conversation_id
		),
		t2_moment_annotation_filter_0 AS (
			SELECT
				conversation_id,
				update_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'include') AND (metadata_string_value = 'selected') AND (conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)))
		),
		moment_annotation_filter_0 AS (
			SELECT DISTINCT
				t1.conversation_id AS conversation_id_0
			FROM
				t1_moment_annotation_filter_0 AS t1
			JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
		),
	

		t1_moment_annotation_filter_1 AS (
			SELECT
				-- Rename the column to avoid ambiguous column issue.
				conversation_id,
				MAX(update_time) AS latest_update_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'overlap') AND (conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)))
			GROUP BY conversation_id
		),
		t2_moment_annotation_filter_1 AS (
			SELECT
				conversation_id,
				update_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'overlap') AND (metadata_string_value = 'selected') AND (conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)))
		),
		moment_annotation_filter_1 AS (
			SELECT DISTINCT
				t1.conversation_id AS conversation_id_1
			FROM
				t1_moment_annotation_filter_1 AS t1
			JOIN t2_moment_annotation_filter_1 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
		),
	
		
			moment_annotation_exclude_filter_0 AS (
				SELECT DISTINCT
				    -- Rename the column to avoid ambiguous column issue.
					conversation_id AS conversation_id_exclude_0
				FROM
					moment_annotation_mv_by_metadata_d
				WHERE
					((moment_template_id = 'exclude') AND (conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)))
			),
		

			moment_annotation_exclude_filter_1 AS (
				SELECT DISTINCT
				    -- Rename the column to avoid ambiguous column issue.
					conversation_id AS conversation_id_exclude_1
				FROM
					moment_annotation_mv_by_metadata_d
				WHERE
					((moment_template_id = 'overlap') AND (conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)))
			),
		
		
		
			scorecard AS (
				SELECT
					scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
				FROM
					scorecard_d
					 WHERE ((scorecard_submit_time >= '2022-01-01 00:00:00') AND (scorecard_submit_time < '2022-01-31 00:00:00')) 
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
					DISTINCT scorecard_time, agent_user_id, conversation_id, scorecard_id, scorecard_last_update_time, criterion_id, percentage_value, float_weight, scorecard_submit_time, manually_scored, scorecard_publish_time
				FROM
					score_d
				WHERE
					((scorecard_submit_time >= '2022-01-01 00:00:00') AND (scorecard_submit_time < '2022-01-31 00:00:00') AND (percentage_value >= 0) AND (not_applicable <> true))
			)
			
	,
		scorecard_score_per_conversation AS (
			SELECT
				conversation_id,
				scorecard_id,
				agent_user_id,
				SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
		SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum
				
			FROM
				scorecard_score JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
					AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
			GROUP BY conversation_id, scorecard_id , agent_user_id
		)
		SELECT
			agent_user_id, 
			SUM(weighted_percentage_sum) AS weighted_percentage_sum,
			SUM(weight_sum) AS weight_sum,
			COUNT(DISTINCT conversation_id) AS total_conversation_count,
			COUNT(DISTINCT scorecard_id) AS total_scorecard_count
			
		FROM
			scorecard_score_per_conversation
				
		INNER JOIN moment_annotation_filter_0 ON
			scorecard_score_per_conversation.conversation_id = moment_annotation_filter_0.conversation_id_0
	

		LEFT JOIN moment_annotation_filter_1 ON
			scorecard_score_per_conversation.conversation_id = moment_annotation_filter_1.conversation_id_1
	
				
			LEFT JOIN moment_annotation_exclude_filter_0 ON
				scorecard_score_per_conversation.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
		

			LEFT JOIN moment_annotation_exclude_filter_1 ON
				scorecard_score_per_conversation.conversation_id = moment_annotation_exclude_filter_1.conversation_id_exclude_1
		
		-- WHERE is optional.
		WHERE TRUE 
			AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 = ''
		

			AND (
				ifNull(moment_annotation_filter_1.conversation_id_1, '') != '' OR
				ifNull(moment_annotation_exclude_filter_1.conversation_id_exclude_1, '') = ''
			)
		
		-- GROUP BY is optional.
		GROUP BY agent_user_id
		SETTINGS max_bytes_before_external_group_by = 5000000000
		