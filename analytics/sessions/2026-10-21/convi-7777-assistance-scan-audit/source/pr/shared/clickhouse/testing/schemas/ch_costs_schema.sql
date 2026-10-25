-- Schema for costs database used in ClickHouse test containers

-- Create the 'costs' database that the application code expects (hardcoded in client.go line 34)
CREATE DATABASE IF NOT EXISTS costs;

-- Create the tables in the 'costs' database as required by the application code
CREATE TABLE IF NOT EXISTS costs.cost_item (
    cost_time DateTime64(3),
    cluster String,
    customer_id String,
    profile_id String,
    usecase_id String,
    is_backtest Bool,
    cost_source String,
    feature String,
    category String,
    details String,
    amount Float64,
    calculation_time DateTime64(3)
) ENGINE = ReplacingMergeTree(calculation_time)
ORDER BY (customer_id, profile_id, cluster, cost_source, feature, category, cost_time);

CREATE TABLE IF NOT EXISTS costs.feature_conversation_summary (
    measured_time DateTime64(3),
    cluster String,
    customer_id String,
    profile_id String,
    usecase_id String,
    conversation_source String,
    feature String,
    is_backfill Bool,
    attributed_voice_conversation_count Int32,
    attributed_text_conversation_count Int32,
    attributed_email_conversation_count Int32,
    attributed_voice_message_count Int32,
    attributed_text_message_count Int32,
    attributed_email_message_count Int32,
    attributed_voice_total_duration_millis Int64,
    calculation_time DateTime64(3)
) ENGINE = ReplacingMergeTree(calculation_time)
ORDER BY (customer_id, profile_id, cluster, conversation_source, feature, measured_time);
