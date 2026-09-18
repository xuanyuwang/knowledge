
		WITH 
		
		t1_moment_annotation_filter_0 AS (
			SELECT
				-- Rename the column to avoid ambiguous column issue.
				conversation_id,
				MAX(update_time) AS latest_update_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'A'))
			GROUP BY conversation_id
		),
		t2_moment_annotation_filter_0 AS (
			SELECT
				conversation_id,
				update_time
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'A') AND (metadata_string_value = 'X'))
		),
		moment_annotation_filter_0 AS (
			SELECT DISTINCT
				t1.conversation_id AS conversation_id_0
			FROM
				t1_moment_annotation_filter_0 AS t1
			JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
		),
	
		
		moment_annotation_exclude_filter_0 AS (
			SELECT DISTINCT
			    -- Rename the column to avoid ambiguous column issue.
				conversation_id AS conversation_id_exclude_0
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((moment_template_id = 'A'))
		),
	
		
		
		
			scorecard AS (
				SELECT
					scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
				FROM
					scorecard_d
					 WHERE ((scorecard_submit_time >= '2021-01-01 00:00:00') AND (scorecard_submit_time < '2021-01-31 00:00:00')) 
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
					*
				FROM
					score_d
				WHERE
					((scorecard_submit_time >= '2021-01-01 00:00:00') AND (scorecard_submit_time < '2021-01-31 00:00:00') AND (percentage_value >= 0) AND (not_applicable <> true))
			)
			
	,
		qa_score_info AS (
			SELECT DISTINCT
			conversation_id,
			conversation_start_time,
			agent_user_id,
			scorecard_template_id,
			scorecard_template_revision,
			scorecard_id,
			score_id,
			criterion_id,
			manually_scored,
			numeric_value,
			max_value,
			percentage_value,
			weight,
			ai_value,
			ai_scored,
			auto_failed,
			not_applicable,
			scorecard_last_update_time
			FROM
				scorecard_score JOIN filtered_scorecard ON scorecard_score.scorecard_id = filtered_scorecard.scorecard_id
					AND scorecard_score.scorecard_last_update_time = filtered_scorecard.scorecard_last_update_time
		)
	
		SELECT 
			qa_score_info.conversation_id AS conversation_id,
			qa_score_info.conversation_start_time AS conversation_start_time,
			qa_score_info.agent_user_id AS agent_user_id,
			qa_score_info.scorecard_template_id AS scorecard_template_id,
			qa_score_info.scorecard_template_revision AS scorecard_template_revision,
			qa_score_info.scorecard_id AS scorecard_id,
			qa_score_info.score_id AS score_id,
			qa_score_info.criterion_id AS criterion_id,
			qa_score_info.manually_scored AS manually_scored,
			qa_score_info.numeric_value AS numeric_value,
			qa_score_info.max_value AS max_value,
			qa_score_info.percentage_value AS percentage_value,
			qa_score_info.weight AS weight,
			qa_score_info.ai_value AS ai_value,
			qa_score_info.ai_scored AS ai_scored,
			qa_score_info.auto_failed AS auto_failed,
			qa_score_info.not_applicable AS not_applicable
		FROM 
			qa_score_info 
			
		LEFT JOIN moment_annotation_filter_0 ON
			qa_score_info.conversation_id = moment_annotation_filter_0.conversation_id_0
	
			
		LEFT JOIN moment_annotation_exclude_filter_0 ON
			qa_score_info.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	
		-- WHERE is optional.
		WHERE TRUE 
		AND (
			ifNull(moment_annotation_filter_0.conversation_id_0, '') != '' OR
			ifNull(moment_annotation_exclude_filter_0.conversation_id_exclude_0, '') = ''
		)
	
		ORDER BY conversation_id, qa_score_info.scorecard_last_update_time DESC
		LIMIT 50 
	