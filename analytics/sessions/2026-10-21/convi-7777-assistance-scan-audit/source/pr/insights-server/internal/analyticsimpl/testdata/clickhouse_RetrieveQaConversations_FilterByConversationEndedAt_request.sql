
		WITH 
		
		
		
		
			conversation AS (
				SELECT
					conversation_id, latest.1 AS conversation_end_time
				FROM (
					SELECT
						conversation_id,
						argMax(tuple(conversation_end_time), update_time) AS latest
					FROM
						conversation_d
					WHERE
						((conversation_start_time >= '2020-12-31 00:00:00') AND (conversation_start_time < '2021-02-01 00:00:00') AND (conversation_end_time >= '2021-01-01 00:00:00' OR update_time >= '2021-01-01 00:00:00'))
						AND conversation_id GLOBAL IN (
							SELECT
								conversation_id
							FROM
								conversation_d
							WHERE
								((conversation_start_time >= '2020-12-31 00:00:00') AND (conversation_start_time < '2021-02-01 00:00:00') AND (conversation_end_time >= '2021-01-01 00:00:00' OR update_time >= '2021-01-01 00:00:00'))
								AND ((conversation_end_time >= '2021-01-01 00:00:00') AND (conversation_end_time < '2021-01-31 00:00:00'))
						)
					GROUP BY
						conversation_id
				)
				WHERE
					((conversation_end_time >= '2021-01-01 00:00:00') AND (conversation_end_time < '2021-01-31 00:00:00'))
			),
	
		
			scorecard AS (
				SELECT
					scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
				FROM
					scorecard_d
					 WHERE ((scorecard_time >= '2021-01-01 00:00:00') AND (scorecard_time < '2021-01-31 00:00:00') AND (scorecard_template_id IN ('TestRetrieveQaConversations_FilterByConversationEndedAt'))) 
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
				JOIN conversation ON scorecard.conversation_id = conversation.conversation_id
				
			),
	
		
			scorecard_score AS (
				SELECT
					*
				FROM
					score_d
				WHERE
					((scorecard_time >= '2021-01-01 00:00:00') AND (scorecard_time < '2021-01-31 00:00:00') AND (scorecard_template_id IN ('TestRetrieveQaConversations_FilterByConversationEndedAt')) AND (percentage_value >= 0) AND (not_applicable <> true))
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
			
			
		-- WHERE is optional.
		
		ORDER BY conversation_id, qa_score_info.scorecard_last_update_time DESC
		LIMIT 50 OFFSET 50
	