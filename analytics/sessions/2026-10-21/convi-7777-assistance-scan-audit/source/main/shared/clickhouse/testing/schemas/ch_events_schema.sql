
CREATE TABLE IF NOT EXISTS `conversation_event` ON CLUSTER 'events'
(
  `agent_user_id` String,
  `conversation_id` String,
  `conversation_start_time` DateTime64(6),
  `create_time` DateTime64(6),
  `customer_id` String,
  `event_id` String,
  `event_name` String,
  `event_payload` Object('json'),
  `event_type` Int32,
  `message_id` String,
  `profile_id` String,
  `usecase_id` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/conversation_event', '{replica}', create_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), event_type, event_name, agent_user_id)
ORDER BY (toStartOfHour(conversation_start_time), event_type, event_name, agent_user_id, customer_id, profile_id, conversation_id, event_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `conversation_event_d` ON CLUSTER 'events' AS `conversation_event`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation_event', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `user_event` ON CLUSTER 'events'
(
  `agent_user_id` String,
  `create_time` DateTime64(6),
  `customer_id` String,
  `event_id` String,
  `event_name` String,
  `event_payload` Object('json'),
  `event_time` DateTime64(6),
  `event_type` Int32
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/user_event', '{replica}', create_time)
PRIMARY KEY (toStartOfHour(event_time), event_type, event_name, agent_user_id)
ORDER BY (toStartOfHour(event_time), event_type, event_name, agent_user_id, customer_id, event_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `user_event_d` ON CLUSTER 'events' AS `user_event`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'user_event', toUnixTimestamp(event_time));

CREATE TABLE IF NOT EXISTS `analytics_event` ON CLUSTER 'events'
(
  `agent_user_id` String,
  `create_time` DateTime64(6),
  `customer_id` String,
  `event_id` String,
  `event_name` String,
  `event_payload` Object('json'),
  `event_time` DateTime64(6),
  `profile_id` String,
  `is_dev_user` Bool
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/analytics_event', '{replica}', create_time)
PRIMARY KEY (toStartOfHour(event_time), event_name, agent_user_id)
ORDER BY (toStartOfHour(event_time), event_name, agent_user_id, customer_id, profile_id, event_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `analytics_event_d` ON CLUSTER 'events' AS `analytics_event`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'analytics_event', toUnixTimestamp(event_time));

CREATE TABLE IF NOT EXISTS `conversation` ON CLUSTER 'events'
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
  `is_dev_user` Bool,
  `is_voice_mail` Bool,
  `language_code` String,
  `platform_account_id` String,
  `platform_chat_id` String,
  `platform_id` String,
  `profile_id` String,
  `raise_hand_state` Int32,
  `umbrella_end_customer_id` String,
  `update_time` DateTime64(6),
  `usecase_id` String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v1.0/conversation', '{replica}', update_time)
PRIMARY KEY (toStartOfHour(conversation_start_time), agent_user_id)
ORDER BY (toStartOfHour(conversation_start_time), agent_user_id, conversation_id, customer_id, profile_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `conversation_d` ON CLUSTER 'events' AS `conversation`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'conversation', toUnixTimestamp(conversation_start_time));

CREATE TABLE IF NOT EXISTS `client_event` ON CLUSTER 'events'
(
  `client` String,
  `customer_id` String,
  `event_id` String,
  `event_payload` String,
  `event_time` DateTime64(6),
  `event_type` Int32
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}-{shard}/{database}/v1.0/client_event', '{replica}')
PRIMARY KEY (toStartOfHour(event_time), event_type, client, customer_id)
ORDER BY (toStartOfHour(event_time), event_type, client, customer_id, event_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `client_event_d` ON CLUSTER 'events' AS `client_event`
ENGINE = Distributed('{cluster}', 'DATABASE_NAME_TO_REPLACE', 'client_event', toUnixTimestamp(event_time));
