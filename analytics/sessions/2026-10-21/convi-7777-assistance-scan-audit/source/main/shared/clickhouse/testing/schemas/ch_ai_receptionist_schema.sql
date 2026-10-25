CREATE TABLE IF NOT EXISTS `ai_receptionist_usage_event` ON CLUSTER 'events'
(
  `agent_id`              String,
  `billability`           Int32,
  `channel`               Int32,
  `conversation_id`       String,
  `customer_id`           String,
  `cycle_start_time`      DateTime64(6),
  `ingested_at_time`      DateTime64(6),
  `interval_end_time`     DateTime64(6),
  `interval_start_time`   DateTime64(6),
  `message_count`         Int32,
  `metadata`              Map(String, String),
  `profile_id`            String,
  `tenant_id`             String,
  `usage_event_id`        String,
  `voice_duration_millis` Int64,
  `correlation_id`        String,
  `call_start_time`       DateTime64(6)
)
ENGINE = ReplicatedReplacingMergeTree(
  '/clickhouse/tables/{cluster}-{shard}/{database}/ai_receptionist_usage_event',
  '{replica}',
  ingested_at_time
)
PARTITION BY toYYYYMM(cycle_start_time)
PRIMARY KEY (customer_id, profile_id, agent_id, channel, cycle_start_time)
ORDER BY (customer_id, profile_id, agent_id, channel, cycle_start_time, usage_event_id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS `ai_receptionist_usage_event_d` ON CLUSTER 'events'
AS `ai_receptionist_usage_event`
ENGINE = Distributed(
  '{cluster}',
  'DATABASE_NAME_TO_REPLACE',
  'ai_receptionist_usage_event',
  xxHash64(agent_id)
);
