-- Read-only investigation SQL. Set session limits separately on every connection.


-- sizeQuery1
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT profile,count(*) AS cards,max(octet_length(comment)) AS max_comment_bytes,sum(octet_length(comment)) AS total_comment_bytes,max(octet_length(acknowledge_comment)) AS max_ack_bytes FROM director.scorecards WHERE customer='cresta-security' GROUP BY profile;
SELECT profile,count(*) AS scores,max(octet_length(comment)) AS max_comment_bytes,sum(octet_length(comment)) AS total_comment_bytes,max(octet_length(text_value)) AS max_text_bytes,sum(octet_length(text_value)) AS total_text_bytes FROM director.scores WHERE customer='cresta-security' GROUP BY profile;
SELECT profile,count(*),max(pg_column_size(template)) AS max_stored_template_bytes FROM director.scorecard_templates WHERE customer='cresta-security' GROUP BY profile;

-- sizeQuery2
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT count(*) AS templates,max(octet_length(template::text)) AS max_template_json_bytes,sum(octet_length(template::text)) AS total_template_json_bytes FROM director.scorecard_templates WHERE customer='cresta-security' AND profile='ps';
SELECT count(*) AS comments,max(octet_length(content)) AS max_content_bytes,sum(octet_length(content)) AS total_content_bytes FROM app.chat_comments WHERE customer_id='cresta-security' AND profile_id='ps';
SELECT count(*) AS coaching_comments,max(octet_length(message)) AS max_message_bytes FROM director.coaching_comments WHERE customer_id='cresta-security' AND profile_id='ps';
SELECT resource_id,conversation_id,template_id,template_revision,agent_user_id,creator_user_id,last_updater_user_id,submitter_user_id,publisher_user_id,octet_length(comment) AS comment_bytes FROM director.scorecards WHERE customer='cresta-security' AND profile='ps' ORDER BY created_at DESC;

-- authSize1
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT user_id,octet_length(display_name) AS display_name_bytes,octet_length(full_name) AS full_name_bytes,octet_length(profile_picture_path) AS picture_path_bytes,octet_length(ui_setting::text) AS ui_setting_json_bytes,octet_length(labels::text) AS labels_json_bytes,cardinality(custom_role_ids) AS custom_role_count FROM auth.users WHERE customer_id='cresta-security' AND user_id IN ('3224f25bd6f2de84','87fd42472e35f08a') ORDER BY user_id;
SELECT count(*) AS users,max(octet_length(display_name)) AS max_name_bytes,max(octet_length(profile_picture_path)) AS max_picture_bytes,max(pg_column_size(ui_setting)) AS max_stored_ui_setting_bytes,max(pg_column_size(labels)) AS max_stored_labels_bytes FROM auth.users WHERE customer_id='cresta-security';
SELECT user_id,count(*) AS direct_memberships FROM auth.user_group_memberships WHERE customer_id='cresta-security' AND user_id IN ('3224f25bd6f2de84','87fd42472e35f08a') GROUP BY user_id;
SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='auth' AND table_name='user_profile_configs' ORDER BY ordinal_position;

-- authSize2
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT user_id,profile_id,pg_column_size(payload) AS stored_payload_bytes,octet_length(payload::text) AS payload_json_bytes FROM auth.user_profile_configs WHERE customer_id='cresta-security' AND user_id IN ('3224f25bd6f2de84','87fd42472e35f08a') ORDER BY user_id,profile_id;
SELECT count(*) AS configs,max(pg_column_size(payload)) AS max_stored_payload_bytes FROM auth.user_profile_configs WHERE customer_id='cresta-security';
SELECT user_id,octet_length(username::text) AS username_bytes,octet_length(email::text) AS email_bytes,octet_length(full_name) AS full_name_bytes,octet_length(external_idm_id) AS external_idm_bytes,octet_length(ui_setting::text) AS ui_bytes,octet_length(labels::text) AS labels_bytes FROM auth.users WHERE customer_id='cresta-security' AND user_id IN ('3224f25bd6f2de84','87fd42472e35f08a');

-- authSize3
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT user_id,octet_length(login_status::text) AS login_status_json_bytes,octet_length(labels::text) AS labels_bytes,octet_length(custom_role_ids::text) AS custom_roles_bytes,octet_length(locale) AS locale_bytes FROM auth.users WHERE customer_id='cresta-security' AND user_id IN ('3224f25bd6f2de84','87fd42472e35f08a');
SELECT user_id,type,octet_length(platform_id) AS platform_id_bytes,octet_length(payload::text) AS payload_json_bytes FROM auth.user_platform_profiles WHERE customer_id='cresta-security' AND user_id IN ('3224f25bd6f2de84','87fd42472e35f08a');
SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='auth' AND table_name='user_platform_profiles' ORDER BY ordinal_position;

-- authSize4
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT e.key,jsonb_typeof(e.value) AS value_type,octet_length(e.value::text) AS value_json_bytes,CASE WHEN jsonb_typeof(e.value)='array' THEN jsonb_array_length(e.value) END AS array_length FROM auth.users u CROSS JOIN LATERAL jsonb_each(u.login_status) e WHERE u.customer_id='cresta-security' AND u.user_id='87fd42472e35f08a' ORDER BY value_json_bytes DESC;
SELECT user_id,updated_at,last_login_status_update_time,octet_length(login_status::text) AS login_status_json_bytes FROM auth.users WHERE customer_id='cresta-security' ORDER BY octet_length(login_status::text) DESC LIMIT 5;

-- authSize5
SET statement_timeout='60s';
SET lock_timeout='3s';
SET default_transaction_read_only=on;
SELECT count(*) AS status_entries,sum(octet_length(e->>'navigationPath')) AS total_navigation_path_bytes,max(octet_length(e->>'navigationPath')) AS max_navigation_path_bytes,count(*) FILTER (WHERE octet_length(e->>'navigationPath')>1048576) AS paths_over_1_mib FROM auth.users u CROSS JOIN LATERAL jsonb_array_elements(u.login_status->'applicationStatus') e WHERE u.customer_id='cresta-security' AND u.user_id='87fd42472e35f08a';
SELECT ordinality,octet_length(e->>'navigationPath') AS navigation_path_bytes,octet_length(e::text) AS entry_json_bytes,e->>'application' AS application,e->>'loginStatus' AS login_status,e->>'lastOnlineTime' AS last_online_time FROM auth.users u CROSS JOIN LATERAL jsonb_array_elements(u.login_status->'applicationStatus') WITH ORDINALITY AS t(e,ordinality) WHERE u.customer_id='cresta-security' AND u.user_id='87fd42472e35f08a' ORDER BY navigation_path_bytes DESC NULLS LAST LIMIT 10;
