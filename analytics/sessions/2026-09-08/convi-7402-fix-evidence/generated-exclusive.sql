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
				((moment_template_id = 'A') AND ((metadata_number_value > 5 AND metadata_number_value <= 10)))
		),
		moment_annotation_filter_0 AS (
			SELECT DISTINCT
				t1.conversation_id AS conversation_id_0
			FROM
				t1_moment_annotation_filter_0 AS t1
			JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
		),
	 existence AS (SELECT DISTINCT conversation_id AS eid FROM moment_annotation_mv_by_metadata_d WHERE ((moment_template_id = 'A'))) SELECT base.conversation_id FROM base 
		LEFT JOIN moment_annotation_filter_0 ON
			base.conversation_id = moment_annotation_filter_0.conversation_id_0
	 LEFT JOIN existence ON base.conversation_id=existence.eid WHERE ifNull(conversation_id_0,'')!='' OR ifNull(eid,'')='' ORDER BY base.conversation_id