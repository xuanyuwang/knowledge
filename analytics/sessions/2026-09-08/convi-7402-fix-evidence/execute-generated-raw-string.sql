CREATE TABLE moment_annotation_mv_by_metadata_d (conversation_id String,conversation_start_time DateTime64(6), moment_template_id String,metadata_string_value String,metadata_number_value Float64,metadata_bool_value Bool,update_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE moment_annotation_d (conversation_id String, conversation_start_time DateTime64(6),conversation_end_time DateTime64(6),moment_template_id String,moment_type Int32,metadata_string_value String,metadata_number_value Float64,metadata_bool_value Bool,create_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE base (conversation_id String) ENGINE=Memory;
INSERT INTO base VALUES ('missing'),('selected'),('other'),('five'),('ten'),('six');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES
('selected','2020-12-01','A','X',0,false,'2021-01-02'),
('other','2020-12-01','A','Y',0,false,'2021-01-02'),
('five','2020-12-01','A','',5,false,'2021-01-02'),
('ten','2020-12-01','A','',10,false,'2021-01-02'),
('six','2020-12-01','A','',6,false,'2021-01-02');
INSERT INTO moment_annotation_d SELECT conversation_id,conversation_start_time,conversation_start_time,moment_template_id,19,metadata_string_value,metadata_number_value,metadata_bool_value,update_time FROM moment_annotation_mv_by_metadata_d;
WITH 
		t1_moment_annotation_filter_0 AS (
			SELECT
				-- Rename the column to avoid ambiguous column issue.
				conversation_id,
				MAX(create_time) AS latest_update_time
			FROM
				moment_annotation_d
			WHERE
				((moment_type = 19) AND (moment_template_id = 'A'))
			GROUP BY conversation_id
		),
		t2_moment_annotation_filter_0 AS (
			SELECT
				conversation_id,
				create_time
			FROM
				moment_annotation_d
			WHERE
				((moment_type = 19) AND (moment_template_id = 'A') AND (metadata_string_value = 'X'))
		),
		moment_annotation_filter_0 AS (
			SELECT DISTINCT
				t1.conversation_id AS conversation_id_0
			FROM
				t1_moment_annotation_filter_0 AS t1
			JOIN t2_moment_annotation_filter_0 AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.create_time
		),
	 existence AS (SELECT DISTINCT conversation_id AS eid FROM moment_annotation_d WHERE ((moment_type = 19) AND (moment_template_id = 'A'))) SELECT base.conversation_id FROM base 
		LEFT JOIN moment_annotation_filter_0 ON
			base.conversation_id = moment_annotation_filter_0.conversation_id_0
	 LEFT JOIN existence ON base.conversation_id=existence.eid WHERE ifNull(conversation_id_0,'')!='' OR ifNull(eid,'')='' ORDER BY base.conversation_id;