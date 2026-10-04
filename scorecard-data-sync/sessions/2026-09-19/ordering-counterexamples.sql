-- Isolated clickhouse-local checks; synthetic values, no production connection.
-- Each result must be 1. These check sink semantics, not distributed delivery.
CREATE TABLE wall_version (id UInt64, source_revision UInt64, payload String, update_time UInt64)
ENGINE = ReplacingMergeTree(update_time) ORDER BY id;
INSERT INTO wall_version VALUES (1, 2, 'new', 20), (1, 1, 'old-delayed', 30);
SELECT 'wall_clock_stale_wins', payload = 'old-delayed' FROM wall_version FINAL;
OPTIMIZE TABLE wall_version FINAL;
SELECT 'query_cannot_recover_merged_version', max(source_revision) = 1 FROM wall_version;

CREATE TABLE source_version (id UInt64, source_revision UInt64, payload String, indexed_at UInt64)
ENGINE = ReplacingMergeTree(source_revision) ORDER BY id;
INSERT INTO source_version VALUES (1, 2, 'new', 20), (1, 1, 'old-delayed', 30);
SELECT 'source_version_rejects_late_old', payload = 'new' FROM source_version FINAL;

-- Logical times: PG modified at 10, workflow cutoff 100, indexed at 110.
CREATE TABLE cleanup (id UInt64, update_time UInt64, indexed_at UInt64)
ENGINE = MergeTree ORDER BY id;
INSERT INTO cleanup VALUES (1, 10, 110), (2, 11, 110);
SELECT 'all_fresh_rows_match_old_cleanup', countIf(update_time < 100) = count() FROM cleanup;
SELECT 'index_clock_distinguishes_fresh_rows', countIf(indexed_at < 100) = 0 FROM cleanup;
ALTER TABLE cleanup DELETE WHERE update_time < 100 SETTINGS mutations_sync = 1;
SELECT 'cleanup_actually_deletes_fresh_rows', count() = 0 FROM cleanup;

-- Existing wall-clock version can defeat a repair that switches to source time.
CREATE TABLE migration (id UInt64, payload String, version UInt64)
ENGINE = ReplacingMergeTree(version) ORDER BY id;
INSERT INTO migration VALUES (1, 'old-wall-clock', 200), (1, 'correct-source-clock', 10);
SELECT 'mixed_version_domains_reject_repair', payload = 'old-wall-clock' FROM migration FINAL;

CREATE TABLE moved_key (id UInt64, agent String, version UInt64)
ENGINE = ReplacingMergeTree(version) ORDER BY (agent, id);
INSERT INTO moved_key VALUES (1, 'old-agent', 1), (1, 'new-agent', 2);
SELECT 'changed_sort_key_survives_final', count() = 2 FROM moved_key FINAL;

CREATE TABLE tombstone (id UInt64, revision UInt64, deleted UInt8)
ENGINE = ReplacingMergeTree(revision) ORDER BY id;
INSERT INTO tombstone VALUES (1, 3, 1), (1, 2, 0);
SELECT 'newer_tombstone_suppresses_late_insert', countIf(deleted = 0) = 0 FROM tombstone FINAL;
