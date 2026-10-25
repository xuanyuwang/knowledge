-- ATTENTION: please use the fixed zookeeper path with version `v0.24`.

CREATE TABLE IF NOT EXISTS `conversation` ON CLUSTER 'conversations'
(
  `agent_user_id` String,
  `channel` Int32,
  `conversation_duration_secs` Int32,
  `conversation_end_time` DateTime64(6),
  `conversation_id` String,
  `conversation_source` Int32,
  `conversation_start_time` DateTime64(6),
  `create_time` DateTime64(6),
  `customer_id` String,
  `is_voice_mail` Bool,
  `language_code` String,
  `platform_account_id` String,
  `platform_chat_id` String,
  `platform_id` String,
  `profile_id` String,
  `raise_hand_state` Int32,
  `umbrella_end_customer_id` String,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `is_dev_user` Bool
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/conversation', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), agent_user_id)
ORDER BY (toStartOfHour(conversation_start_time), agent_user_id, conversation_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

-- Note: The `Distributed` tables don't support `{database}` keyword.
-- Please replace `DATABASE_NAME_TO_REPLACE` (database name) with the actual customer profile IDs.
CREATE TABLE IF NOT EXISTS `conversation_d` ON CLUSTER 'conversations' AS `conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `conversation_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/conversation_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (conversation_id)
ORDER BY (conversation_id)
POPULATE AS
SELECT
  customer_id,
  profile_id,
  conversation_id,
  conversation_start_time,
  update_time
FROM `conversation`;

CREATE TABLE IF NOT EXISTS `conversation_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `conversation_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation_mv_by_conversation', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `message` ON CLUSTER 'conversations'
(
  `agent_user_id` String,
  `conversation_duration_secs` Int32,
  `conversation_end_time` DateTime64(6),
  `conversation_id` String,
  `conversation_source` Int32,
  `conversation_start_time` DateTime64(6),
  `create_time` DateTime64(6),
  `customer_id` String,
  `end_time_ms` Int32,
  `handle_time_secs` Int32,
  `is_voice_mail` Bool,
  `language_code` String,
  `message_id` String,
  `message_text` String,
  `platform_sequence_id` Int32,
  `platform_time` DateTime64(6),
  `profile_id` String,
  `raise_hand_state` Int32,
  `speaker_role` String,
  `start_time_ms` Int32,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `is_dev_user` Bool,
  `message_text_length` Int32,
  `message_word_count` Int32
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/message', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), agent_user_id)
ORDER BY (toStartOfHour(conversation_start_time), agent_user_id, conversation_id, message_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `message_d` ON CLUSTER 'conversations' AS `message`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'message', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `message_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/message_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (conversation_id, message_id)
ORDER BY (conversation_id, message_id)
POPULATE AS
SELECT
  customer_id,
  profile_id,
  conversation_id,
  message_id,
  conversation_start_time,
  update_time
FROM `message`;

CREATE TABLE IF NOT EXISTS `message_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `message_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'message_mv_by_conversation', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `action_annotation` ON CLUSTER 'conversations'
(
  `action_annotation_id` String,
  `action_detailed_type` Int32,
  `action_id` String,
  `action_type` Int32,
  `adherence_type` Int32,
  `agent_user_id` String,
  `conversation_duration_secs` Int32,
  `conversation_end_time` DateTime64(6),
  `conversation_id` String,
  `conversation_source` Int32,
  `conversation_start_time` DateTime64(6),
  `create_time` DateTime64(6),
  `customer_id` String,
  `is_voice_mail` Bool,
  `language_code` String,
  `message_id` String,
  `payload.manager_whisper.manager_user_id` String,
  `policy_id` String,
  `policy_source_type` Int32,
  `profile_id` String,
  `raise_hand_state` Int32,
  `taxonomy` String,
  `taxonomy_state` Int32,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `is_dev_user` Bool,
  `knowledge_payload_knowledge_session_id` String,
  `summarization_payload_is_valid_summary` Bool,
  `payload.workflow_automation_hint.workflow_automation_definition_id` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/action_annotation', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), agent_user_id, policy_id)
ORDER BY (toStartOfHour(conversation_start_time), agent_user_id, policy_id, conversation_id, message_id, action_annotation_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `action_annotation_d` ON CLUSTER 'conversations' AS `action_annotation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'action_annotation', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `action_annotation_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/action_annotation_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (conversation_id, message_id, action_annotation_id)
ORDER BY (conversation_id, message_id, action_annotation_id)
POPULATE AS
SELECT
  customer_id,
  profile_id,
  conversation_id,
  message_id,
  action_annotation_id,
  conversation_start_time,
  update_time
FROM `action_annotation`;

CREATE TABLE IF NOT EXISTS `action_annotation_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `action_annotation_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'action_annotation_mv_by_conversation', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `moment_annotation` ON CLUSTER 'conversations'
(
  `adherence_action_adherence_type` Int32,
  `adherence_action_annotation_id` String,
  `adherence_action_detailed_type` Int32,
  `adherence_action_id` String,
  `adherence_action_policy_id` String,
  `adherence_action_policy_source_type` Int32,
  `adherence_action_type` Int32,
  `adherence_moment_adherence_type` Int32,
  `adherence_moment_annotation_id` String,
  `adherence_moment_detailed_type` Int32,
  `adherence_moment_policy_id` String,
  `adherence_moment_policy_source_type` Int32,
  `adherence_moment_type` Int32,
  `adherence_type` Int32,
  `agent_user_id` String,
  `behavior_id` String,
  `conversation_duration_secs` Int32,
  `conversation_end_time` DateTime64(6),
  `conversation_id` String,
  `conversation_source` Int32,
  `conversation_start_time` DateTime64(6),
  `create_time` DateTime64(6),
  `customer_id` String,
  `is_voice_mail` Bool,
  `labels.hiding_in_performance_insights` Bool,
  `language_code` String,
  `message_id` String,
  `moment_annotation_id` String,
  `moment_detailed_type` Int32,
  `moment_template_id` String,
  `moment_type` Int32,
  `policy_id` String,
  `policy_source_type` Int32,
  `profile_id` String,
  `raise_hand_state` Int32,
  `taxonomy` String,
  `taxonomy_state` Int32,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `is_dev_user` Bool,
  `metadata_bool_value` Bool,
  `metadata_number_value` Float64,
  `metadata_source` Int32,
  `metadata_string_value` String,
  `metadata_type` Int32,
  `knowledge_payload_knowledge_session_id` String,
  `creator_user_id` String,
  `moment_annotation_payload` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/moment_annotation', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), agent_user_id, policy_id)
ORDER BY (toStartOfHour(conversation_start_time), agent_user_id, policy_id, conversation_id, message_id, moment_annotation_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `moment_annotation_d` ON CLUSTER 'conversations' AS `moment_annotation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'moment_annotation', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `moment_annotation_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/moment_annotation_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (conversation_id, message_id, moment_annotation_id)
ORDER BY (conversation_id, message_id, moment_annotation_id)
POPULATE AS
SELECT
  customer_id,
  profile_id,
  conversation_id,
  message_id,
  moment_annotation_id,
  conversation_start_time,
  update_time
FROM `moment_annotation`;

CREATE TABLE IF NOT EXISTS `moment_annotation_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `moment_annotation_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'moment_annotation_mv_by_conversation', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `metadata_moment_value_count_mv` ON CLUSTER 'conversations'
(
    `partition_month` Date,
    `moment_template_id` String,
    `usecase_id` String,
    `metadata_bool_value` Bool,
    `metadata_number_value` Float64,
    `metadata_string_value` String,
    `metadata_value_count` AggregateFunction(uniq, String)
)
ENGINE = AggregatingMergeTree()
PARTITION BY partition_month
PRIMARY KEY (moment_template_id, usecase_id, metadata_bool_value, metadata_number_value, metadata_string_value)
ORDER BY (moment_template_id, usecase_id, metadata_bool_value, metadata_number_value, metadata_string_value)
POPULATE AS
SELECT
    toDate(toStartOfMonth(conversation_start_time)) AS partition_month,
    moment_template_id,
    usecase_id,
    metadata_bool_value,
    metadata_number_value,
    metadata_string_value,
    uniqState(moment_annotation_id) AS metadata_value_count
FROM `moment_annotation`
WHERE moment_type = 19
GROUP BY partition_month, moment_template_id, usecase_id, metadata_bool_value, metadata_number_value, metadata_string_value;

CREATE TABLE IF NOT EXISTS `metadata_moment_value_count_mv_d` ON CLUSTER 'conversations'
AS `metadata_moment_value_count_mv`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'metadata_moment_value_count_mv');

CREATE TABLE IF NOT EXISTS `conversation_event` ON CLUSTER 'conversations'
(
  `action_adherence_type` Int32,
  `action_annotation_id` String,
  `action_detailed_type` Int32,
  `action_id` String,
  `action_policy_id` String,
  `action_policy_source_type` Int32,
  `action_type` Int32,
  `agent_user_id` String,
  `conversation_duration_secs` Int32,
  `conversation_end_time` DateTime64(6),
  `conversation_id` String,
  `conversation_source` Int32,
  `conversation_start_time` DateTime64(6),
  `create_time` DateTime64(6),
  `customer_id` String,
  `event_id` String,
  `event_type` Int32,
  `is_voice_mail` Bool,
  `language_code` String,
  `message_id` String,
  `profile_id` String,
  `raise_hand_state` Int32,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `is_dev_user` Bool,
  `knowledge_payload_knowledge_session_id` String,
  `summarization_payload_data_type` Int32,
  `payload.used_kb_document.doc_id` String,
  `payload.used_kb_document.doc_type` Int32,
  `payload.used_kb_document.session_id` String,
  `payload.used_kb_search.session_id` String,
  `payload.used_note_taking.data_type` Int32,
  `payload.used_summarization.data_type` Int32,
  `payload.workflow_automation.workflow_automation_run_id` String,
  `payload.workflow_automation.workflow_automation_definition_id` String,
  `payload.workflow_automation.workflow_automation_definition_version` Int32,
  `payload.workflow_automation.trigger_source` Int32,
  `payload.workflow_automation.result_status` Int32
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/conversation_event', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), agent_user_id, action_policy_id)
ORDER BY (toStartOfHour(conversation_start_time), agent_user_id, action_policy_id, conversation_id, message_id, event_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `conversation_event_d` ON CLUSTER 'conversations' AS `conversation_event`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation_event', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `conversation_event_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/conversation_event_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (conversation_id, message_id, event_id)
ORDER BY (conversation_id, message_id, event_id)
POPULATE AS
SELECT
  customer_id,
  profile_id,
  conversation_id,
  message_id,
  event_id,
  conversation_start_time,
  update_time
FROM `conversation_event`;

CREATE TABLE IF NOT EXISTS `conversation_event_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `conversation_event_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation_event_mv_by_conversation', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `scorecard_score` ON CLUSTER 'conversations'
(
  `agent_user_id` String,
  `ai_scored` Bool,
  `ai_value` Float64,
  `auto_failed` Bool,
  `conversation_duration_bins_id` Int32,
  `conversation_formatted_time` DateTime64(6),
  `conversation_start_time` DateTime64(6),
  `conversation_id` String,
  `criterion_id` String,
  `customer_id` String,
  `float_weight` Float64,
  `is_voice_mail` Bool,
  `language_code` String,
  `manually_scored` Bool,
  `max_value` Float64,
  `not_applicable` Bool,
  `numeric_value` Float64,
  `percentage_value` Float64,  -- Using `-1` to represent NULL value.
  `profile_id` String,
  `score_id` String,
  `scorecard_id` String,
  `scorecard_last_update_time` DateTime64(6),
  `scorecard_template_id` String,
  `scorecard_template_revision` String,
  `text_value` String,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `weight` Int32,
  `is_dev_user` Bool
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/scorecard_score', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_formatted_time), scorecard_template_id, criterion_id, agent_user_id)
ORDER BY (toStartOfHour(conversation_formatted_time), scorecard_template_id, criterion_id, agent_user_id, customer_id, profile_id, scorecard_id, score_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `scorecard_score_d` ON CLUSTER 'conversations' AS `scorecard_score`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'scorecard_score', toUnixTimestamp(conversation_start_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `scorecard_score_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/scorecard_score_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (conversation_id, scorecard_id, score_id)
ORDER BY (conversation_id, scorecard_id, score_id)
POPULATE AS
SELECT
  customer_id,
  profile_id,
  conversation_id,
  scorecard_id,
  score_id,
  conversation_start_time,
  update_time
FROM `scorecard_score`;

CREATE TABLE IF NOT EXISTS `scorecard_score_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `scorecard_score_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'scorecard_score_mv_by_conversation', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `scorecard` ON CLUSTER 'conversations'
(
  `acknowledge_comment` String,
  `agent_user_id` String,
  `ai_score_time` DateTime64(6),
  `auto_failed` Bool,
  `coaching_plan_id` String,
  `conversation_duration_bins_id` Int32,
  `conversation_duration_secs` Int32,
  `conversation_id` String,
  `creator_user_id` String,
  `customer_id` String,
  `is_dev_user` Bool,
  `is_voice_mail` Bool,
  `last_updator_user_id` String,
  `manually_scored` Bool,
  `profile_id` String,
  `score` Float64,
  `scorecard_acknowledge_time` DateTime64(6),
  `scorecard_create_time` DateTime64(6),
  `scorecard_id` String,
  `scorecard_last_update_time` DateTime64(6),
  `scorecard_submit_time` DateTime64(6),
  `scorecard_template_id` String,
  `scorecard_template_revision` String,
  `scorecard_time` DateTime64(6),
  `submitter_user_id` String,
  `publish_time` DateTime64(6),
  `publisher_user_id` String,
  `update_time` DateTime64(6),
  `usecase_id` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/scorecard', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(scorecard_time), scorecard_template_id, agent_user_id)
ORDER BY (toStartOfHour(scorecard_time), scorecard_template_id, agent_user_id, customer_id, profile_id, scorecard_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `scorecard_d` ON CLUSTER 'conversations' AS `scorecard`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'scorecard', toUnixTimestamp(scorecard_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `scorecard_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/scorecard_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(scorecard_time)
PRIMARY KEY (conversation_id, scorecard_id)
ORDER BY (conversation_id, scorecard_id)
POPULATE AS
SELECT
    customer_id,
    profile_id,
    conversation_id,
    scorecard_id,
    scorecard_time,
    update_time
FROM `scorecard`;

CREATE TABLE IF NOT EXISTS `scorecard_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `scorecard_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'scorecard_mv_by_conversation', toUnixTimestamp(scorecard_time));

CREATE TABLE IF NOT EXISTS `score` ON CLUSTER 'conversations'
(
  `acknowledge_comment` String,
  `agent_user_id` String,
  `ai_score_time` DateTime64(6),
  `ai_scored` Bool,
  `ai_value` Float64,
  `auto_failed` Bool,
  `coaching_plan_id` String,
  `conversation_duration_bins_id` Int32,
  `conversation_duration_secs` Int32,
  `conversation_id` String,
  `conversation_start_time` DateTime64(6),
  `creator_user_id` String,
  `criterion_id` String,
  `customer_id` String,
  `float_weight` Float64,
  `is_dev_user` Bool,
  `is_voice_mail` Bool,
  `language_code` String,
  `last_updator_user_id` String,
  `manually_scored` Bool,
  `max_value` Float64,
  `not_applicable` Bool,
  `numeric_value` Float64,
  `percentage_value` Float64,
  `profile_id` String,
  `score_id` String,
  `scorecard_acknowledge_time` DateTime64(6),
  `scorecard_create_time` DateTime64(6),
  `scorecard_id` String,
  `scorecard_last_update_time` DateTime64(6),
  `scorecard_publish_time` DateTime64(6),
  `scorecard_score` Float64,
  `scorecard_submit_time` DateTime64(6),
  `scorecard_template_id` String,
  `scorecard_template_revision` String,
  `scorecard_time` DateTime64(6),
  `submitter_user_id` String,
  `text_value` String,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `weight` Int32
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/score', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(scorecard_time), scorecard_template_id, criterion_id, agent_user_id)
ORDER BY (toStartOfHour(scorecard_time), scorecard_template_id, criterion_id, agent_user_id, customer_id, profile_id, scorecard_id, score_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `score_d` ON CLUSTER 'conversations' AS `score`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'score', toUnixTimestamp(scorecard_time));

CREATE MATERIALIZED VIEW IF NOT EXISTS `score_mv_by_conversation` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/score_mv_by_conversation', '{replica}', update_time)
PARTITION BY toYYYYMM(scorecard_time)
PRIMARY KEY (conversation_id, scorecard_id, score_id)
ORDER BY (conversation_id, scorecard_id, score_id)
POPULATE AS
SELECT
    customer_id,
    profile_id,
    conversation_id,
    scorecard_id,
    score_id,
    scorecard_time,
    update_time
FROM `score`;

CREATE TABLE IF NOT EXISTS `score_d_mv_by_conversation` ON CLUSTER 'conversations'
AS `score_mv_by_conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'score_mv_by_conversation', toUnixTimestamp(scorecard_time));

-- The small table (queue) to batch delete conversations.
-- See one-pager design in https://docs.google.com/document/d/1EPQw2czWo-WGhjodiHD5FfJPY27f5h7Je168hctUm9w/
CREATE TABLE IF NOT EXISTS `delete_conversation_queue` ON CLUSTER 'conversations'
(
  `conversation_id` String,
  `enqueue_time` DateTime64(6),
  `table_name` String,
  `cut_off_time` DateTime64(6)
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/delete_conversation_queue', '{replica}', enqueue_time)
PRIMARY KEY (enqueue_time, conversation_id)
ORDER BY (enqueue_time, conversation_id)
TTL toDateTime(enqueue_time) + INTERVAL 2 WEEK DELETE
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `delete_conversation_queue_d` ON CLUSTER 'conversations' AS `delete_conversation_queue`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'delete_conversation_queue', toUnixTimestamp(enqueue_time));

-- conversation and their labels
CREATE TABLE IF NOT EXISTS `conversation_with_labels` ON CLUSTER 'conversations'
(
  `agent_user_id` String,
  `conversation_end_time` DateTime64(6),
  `conversation_id` String,
  `conversation_start_time` DateTime64(6),
  `customer_id` String,
  `has_agent_assistance` Bool,
  `profile_id` String,
  `update_time` DateTime64(6),
  `usecase_id` String,
  `create_time` DateTime64(6),
  `message_id` String DEFAULT '',
  `message_agent_user_id` String DEFAULT '',
  `message_time` DateTime64(6) DEFAULT toDateTime64(0, 6)
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/conversation_with_labels', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_end_time), agent_user_id)
ORDER BY (toStartOfHour(conversation_end_time), agent_user_id, conversation_id, customer_id, profile_id, message_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `conversation_with_labels_d` ON CLUSTER 'conversations' AS `conversation_with_labels`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation_with_labels', toUnixTimestamp(conversation_end_time));

-- new materialized view for metadata annotation along with its distributed table
CREATE MATERIALIZED VIEW IF NOT EXISTS `moment_annotation_mv_by_metadata` ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/moment_annotation_mv_by_metadata', '{replica}', update_time)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), conversation_id, moment_template_id)
ORDER BY (toStartOfHour(conversation_start_time), conversation_id, moment_template_id)
POPULATE AS
SELECT
  conversation_id,
  conversation_start_time,
  moment_template_id,
  metadata_string_value,
  metadata_number_value,
  metadata_bool_value,
  update_time
FROM `moment_annotation`
WHERE moment_type = 19;

CREATE TABLE IF NOT EXISTS `moment_annotation_mv_by_metadata_d` ON CLUSTER 'conversations'
AS `moment_annotation_mv_by_metadata`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'moment_annotation_mv_by_metadata', toUnixTimestamp(conversation_start_time));

-- Storage for conversation-outcome (CLO) moment annotations (moment_type = 14).
-- create_time: source-DB annotation time. update_time: CH ingest + ReplacingMergeTree version.
CREATE TABLE IF NOT EXISTS `moment_annotation_by_conversation_outcome` ON CLUSTER 'conversations'
(
    `conversation_id` String,
    `conversation_start_time` DateTime64(6),
    `moment_template_id` String,
    `outcome_string_value` String,
    `outcome_number_value` Float64,
    `outcome_bool_value` Bool,
    `outcome_value_type` String,
    `create_time` DateTime64(6),
    `update_time` DateTime64(6)
)
ENGINE = ReplicatedReplacingMergeTree(
  '/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/moment_annotation_by_conversation_outcome',
  '{replica}',
  update_time
)
PARTITION BY toYYYYMM(conversation_start_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), conversation_id, moment_template_id)
ORDER BY (toStartOfHour(conversation_start_time), conversation_id, moment_template_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS `moment_annotation_mv_by_conversation_outcome` ON CLUSTER 'conversations'
TO `moment_annotation_by_conversation_outcome`
AS
SELECT
  conversation_id,
  conversation_start_time,
  moment_template_id,
  JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') AS outcome_string_value,
  JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') AS outcome_number_value,
  JSONExtractBool(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') AS outcome_bool_value,
  multiIf(
    JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value'), 'string',
    JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value'), 'number',
    JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value'), 'boolean',
    ''
  ) AS outcome_value_type,
  create_time,
  update_time
FROM `moment_annotation`
WHERE moment_type = 14;

CREATE TABLE IF NOT EXISTS `moment_annotation_by_conversation_outcome_d` ON CLUSTER 'conversations'
AS `moment_annotation_by_conversation_outcome`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'moment_annotation_by_conversation_outcome', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `user_outcome_field_value` ON CLUSTER 'conversations'
(
    `create_time` DateTime64(6),
    `customer_id` String,
    `data_source` String,
    `data_source_record_id` String,
    `field_definition_id` String,
    `profile_id` String,
    `raw_value` String,
    `outcome_time` DateTime64(6),
    `update_time` DateTime64(6),
    `user_id` String,
    `value_bool` Bool,
    `value_numeric` Float64,
    `value_text` String,
    `weight_field_definition_id` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/user_outcome_field_value', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(outcome_time), field_definition_id, user_id)
ORDER BY (toStartOfHour(outcome_time), field_definition_id, user_id, data_source_record_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `user_outcome_field_value_d` ON CLUSTER 'conversations' AS `user_outcome_field_value`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'user_outcome_field_value', toUnixTimestamp(outcome_time));

CREATE TABLE IF NOT EXISTS `screen_analytics_event` ON CLUSTER 'conversations'
(
  `conversations` Array(String),
  `customer_id` String,
  `device_id` String,
  `event_id` String,
  `event_time` DateTime64(6),
  `event_type` Int32,
  `payload` String,
  `platform` Int32,
  `profile_id` String,
  `screen_recording_sessions` Array(String),
  `server_time` DateTime64(6),
  `session_id` String,
  `user_id` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/screen_analytics_event', '{replica}', server_time)
PARTITION BY toYYYYMM(event_time)
PRIMARY KEY (toStartOfHour(event_time), user_id)
ORDER BY (toStartOfHour(event_time), user_id, event_time, event_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `screen_analytics_event_d` ON CLUSTER 'conversations' AS `screen_analytics_event`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'screen_analytics_event', toUnixTimestamp(event_time));

CREATE TABLE IF NOT EXISTS `screen_analytics_classified_spans` ON CLUSTER 'conversations'
(
  `customer_id` String,
  `profile_id` String,
  `user_id` String,
  `span_start` DateTime64(6),
  `span_end` DateTime64(6),
  `conversation_ids` Array(String),
  `entry_type` Int32,
  `entry_identifier` String,
  `entry_display_name` String,
  `rule_id` String,
  `rule_revision` String,
  `rule_action` Int32,
  `backfill_run_id` String,
  `written_at` DateTime64(6) DEFAULT now64(6),
  `url_covered` Bool DEFAULT false,
  INDEX idx_conversation_ids conversation_ids TYPE bloom_filter(0.01) GRANULARITY 4
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/screen_analytics_classified_spans', '{replica}', written_at)
PARTITION BY toYYYYMM(span_start)
ORDER BY (user_id, span_start, entry_type)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `screen_analytics_classified_spans_d` ON CLUSTER 'conversations' AS `screen_analytics_classified_spans`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'screen_analytics_classified_spans', toUnixTimestamp(span_start));