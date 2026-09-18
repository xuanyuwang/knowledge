CREATE TABLE moment_annotation_mv_by_metadata_d (conversation_id String, conversation_start_time DateTime64(6), moment_template_id String, metadata_string_value String, metadata_number_value Float64, metadata_bool_value Bool, update_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE scorecard_d (scorecard_id String, scorecard_last_update_time DateTime64(6), conversation_id String, scorecard_submit_time DateTime64(6), manually_scored Bool, publish_time DateTime64(6), scorecard_time DateTime64(6), scorecard_template_id String) ENGINE=Memory;
CREATE TABLE score_d (conversation_id String, conversation_start_time DateTime64(6), agent_user_id String, scorecard_template_id String, scorecard_template_revision Int32, scorecard_id String, score_id String, criterion_id String, manually_scored Bool, numeric_value Float64, max_value Float64, percentage_value Float64, weight Float64, float_weight Float64, ai_value Float64, ai_scored Bool, auto_failed Bool, not_applicable Bool, scorecard_last_update_time DateTime64(6), scorecard_time DateTime64(6), scorecard_submit_time DateTime64(6), scorecard_publish_time DateTime64(6)) ENGINE=Memory;
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('selected', '2021-01-02 00:00:00','moment_id_2','',1,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('selected', '2021-01-02 00:00:00','moment_id_1','metadata-value-1',0,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-selected','selected','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-selected','selected','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('other', '2021-01-02 00:00:00','moment_id_2','',1,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('other', '2021-01-02 00:00:00','moment_id_1','other',0,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-other','other','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-other','other','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('missing', '2021-01-02 00:00:00','moment_id_2','',1,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-missing','missing','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-missing','missing','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('excluded', '2021-01-02 00:00:00','moment_id_2','',1,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('excluded', '2021-01-02 00:00:00','moment_id_1','metadata-value-1',0,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('excluded', '2021-01-02 00:00:00','moment_id_3','',0,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-excluded','excluded','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-excluded','excluded','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-selected-new-other', '2021-01-02 00:00:00','moment_id_2','',1,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-selected-new-other', '2021-01-02 00:00:00','moment_id_1','metadata-value-1',0,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-old-selected-new-other','old-selected-new-other','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-old-selected-new-other','old-selected-new-other','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-other-new-selected', '2021-01-02 00:00:00','moment_id_2','',1,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-other-new-selected', '2021-01-02 00:00:00','moment_id_1','other',0,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-old-other-new-selected','old-other-new-selected','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-old-other-new-selected','old-other-new-selected','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('outside-numeric', '2021-01-02 00:00:00','moment_id_2','',300,false,'2021-01-03 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('outside-numeric', '2021-01-02 00:00:00','moment_id_1','metadata-value-1',0,false,'2021-01-03 00:00:00');
INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-outside-numeric','outside-numeric','2021-01-04 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue');
INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-outside-numeric','outside-numeric','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue',80,1,'agent');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-selected-new-other', '2021-01-02 00:00:00','moment_id_1','other',0,false,'2021-01-05 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-other-new-selected', '2021-01-02 00:00:00','moment_id_1','metadata-value-1',0,false,'2021-01-05 00:00:00');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('selected', '2021-01-02 00:00:00','moment_id_1','metadata-value-1',0,false,'2021-01-03 00:00:00');
		WITH 
		
			t1_moment_annotation_filter_0 AS (
				SELECT
					-- Rename the column to avoid ambiguous column issue.
					conversation_id,
					MAX(update_time) AS latest_update_time
				FROM
					moment_annotation_mv_by_metadata_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-31 00:00:00') AND (moment_template_id = 'moment_id_2'))
				GROUP BY conversation_id
			),
			t2_moment_annotation_filter_0 AS (
				SELECT
					conversation_id,
					update_time
				FROM
					moment_annotation_mv_by_metadata_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-31 00:00:00') AND (moment_template_id = 'moment_id_2') AND ((metadata_number_value >= 0 AND metadata_number_value < 100) OR (metadata_number_value >= 100 AND metadata_number_value < 250)))
			),
			moment_annotation_filter_0 AS (
				SELECT DISTINCT
					t1.conversation_id
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
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-31 00:00:00') AND (moment_template_id = 'moment_id_1'))
				GROUP BY conversation_id
			),
			t2_moment_annotation_filter_1 AS (
				SELECT
					conversation_id,
					update_time
				FROM
					moment_annotation_mv_by_metadata_d
				WHERE
					((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-31 00:00:00') AND (moment_template_id = 'moment_id_1') AND (metadata_string_value = 'metadata-value-1' OR metadata_string_value = 'metadata-value-2'))
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
				((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-31 00:00:00') AND (moment_template_id = 'moment_id_3'))
		),
	

		moment_annotation_exclude_filter_1 AS (
			SELECT DISTINCT
			    -- Rename the column to avoid ambiguous column issue.
				conversation_id AS conversation_id_exclude_1
			FROM
				moment_annotation_mv_by_metadata_d
			WHERE
				((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2021-01-31 00:00:00') AND (moment_template_id = 'moment_id_1'))
		),
	
		
		
		
			scorecard AS (
				SELECT
					scorecard_id, scorecard_last_update_time, conversation_id, scorecard_submit_time, manually_scored, publish_time AS scorecard_publish_time
				FROM
					scorecard_d
					 WHERE ((scorecard_time >= '2021-01-01 00:00:00') AND (scorecard_time < '2021-01-31 00:00:00') AND (scorecard_template_id IN ('TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue'))) 
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
					((scorecard_time >= '2021-01-01 00:00:00') AND (scorecard_time < '2021-01-31 00:00:00') AND (scorecard_template_id IN ('TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue')) AND (percentage_value >= 0) AND (not_applicable <> true))
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
			
			INNER JOIN moment_annotation_filter_0 ON
				qa_score_info.conversation_id = moment_annotation_filter_0.conversation_id
		

			LEFT JOIN moment_annotation_filter_1 ON
				qa_score_info.conversation_id = moment_annotation_filter_1.conversation_id_1
		
			
			LEFT JOIN moment_annotation_exclude_filter_0 ON
			qa_score_info.conversation_id = moment_annotation_exclude_filter_0.conversation_id_exclude_0
	
		LEFT JOIN moment_annotation_exclude_filter_1 ON
			qa_score_info.conversation_id = moment_annotation_exclude_filter_1.conversation_id_exclude_1
	
		-- WHERE is optional.
		WHERE TRUE 
		AND moment_annotation_exclude_filter_0.conversation_id_exclude_0 = ''
	

		AND (
			ifNull(moment_annotation_filter_1.conversation_id_1, '') != '' OR
			ifNull(moment_annotation_exclude_filter_1.conversation_id_exclude_1, '') = ''
		)
	
		ORDER BY conversation_id, qa_score_info.scorecard_last_update_time DESC
		LIMIT 50 OFFSET 0
;