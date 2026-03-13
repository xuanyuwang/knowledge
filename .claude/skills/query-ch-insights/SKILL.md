---
name: query-ch-insights
description: Query analytics/insights/coaching data from Cresta ClickHouse databases (scores, scorecards, conversations). Use when the user needs to look up CH data for analytics.
user-invocable: true
allowed-tools: Bash(*clickhouse*), Read
argument-hint: "[customer] [profile] [environment] [query description]"
---

# Query ClickHouse Insights Data

Query analytics/insights data from Cresta ClickHouse databases.

User request: $ARGUMENTS

## Connection Setup

### ClickHouse client location

```
/opt/homebrew/bin/clickhouse client
```

### Connection string format

The CH connection requires:
- **host**: `clickhouse-conversations.<environment>.internal.cresta.ai`
- **port**: `9440`
- **user**: `admin`
- **password**: fetched from k8s secret (see below)
- **secure**: always use TLS

### Get password from Kubernetes

```bash
kubectl --context "<environment>_dev" -n clickhouse \
  get secrets clickhouse-cluster --template '{{.data.admin_password}}' \
  | base64 -d
```

### Available CH clusters

| Cluster | Purpose |
|---------|---------|
| `clickhouse-conversations` | Conversation & QA analytics (most common) |
| `clickhouse-request-log` | Request logging |
| `clickhouse-events` | Event data |

### Running a query

```bash
/opt/homebrew/bin/clickhouse client \
  --host=clickhouse-conversations.<environment>.internal.cresta.ai \
  --port=9440 \
  --user=admin \
  --password='<password>' \
  --secure \
  --database=<database_name> \
  --query="<SQL QUERY>" 2>&1
```

### Database naming convention

The database name is derived from customer and profile: `cresta_<profile_with_underscores>`

Examples:
- customer=cresta, profile=walter-dev -> `cresta_walter_dev`
- customer=cresta, profile=sales -> `cresta_sales`
- customer=hilton, profile=care-voice -> `hilton_care_voice`

To verify, run: `SHOW DATABASES` and look for the matching database.

## Schema: Scorecard & Score Tables

Each database has both **distributed** (`_d` suffix) and **local** tables. Always query the distributed tables (`_d` suffix) for correct results across shards.

### `scorecard_d` (distributed) / `scorecard` (local)

Scorecard-level data (one row per scorecard per update). Local engine: `ReplicatedReplacingMergeTree` with `update_time` as version column. Distributed engine shards by `toUnixTimestamp(scorecard_time)`.

| Column | Type | Notes |
|--------|------|-------|
| scorecard_id | String | Unique scorecard identifier |
| scorecard_template_id | String | FK to template |
| scorecard_template_revision | String | |
| agent_user_id | String | Agent being evaluated |
| creator_user_id | String | Who created it |
| conversation_id | String | |
| usecase_id | String | |
| score | Float64 | Overall score (0-100 scale) |
| scorecard_time | DateTime64(6) | Primary time column (conversation start time) |
| scorecard_create_time | DateTime64(6) | |
| scorecard_last_update_time | DateTime64(6) | |
| scorecard_submit_time | DateTime64(6) | Zero if not submitted |
| scorecard_acknowledge_time | DateTime64(6) | |
| ai_score_time | DateTime64(6) | |
| manually_scored | Bool | |
| auto_failed | Bool | |
| is_dev_user | Bool | |
| is_voice_mail | Bool | |
| conversation_duration_secs | Int32 | |
| customer_id | String | |
| profile_id | String | |

**Primary key**: `(toStartOfHour(scorecard_time), scorecard_template_id, agent_user_id)`
**Order by**: `(toStartOfHour(scorecard_time), scorecard_template_id, agent_user_id, customer_id, profile_id, scorecard_id)`

### `score_d` (distributed) / `score` (local)

Criterion-level score data (one row per criterion per scorecard per update). Local engine: `ReplicatedReplacingMergeTree` with `update_time` as version column. Distributed engine shards by `toUnixTimestamp(scorecard_time)`.

| Column | Type | Notes |
|--------|------|-------|
| score_id | String | Unique score identifier |
| scorecard_id | String | FK to scorecard |
| criterion_id | String | Criterion identifier (matches template JSON items[].identifier) |
| scorecard_template_id | String | |
| agent_user_id | String | |
| conversation_id | String | |
| usecase_id | String | |
| percentage_value | Float64 | Score as percentage (0-1), -1 means N/A |
| float_weight | Float64 | Weight for weighted average |
| numeric_value | Float64 | Raw numeric value |
| max_value | Float64 | |
| ai_value | Float64 | |
| not_applicable | Bool | |
| ai_scored | Bool | |
| manually_scored | Bool | |
| scorecard_time | DateTime64(6) | Primary time column |
| scorecard_last_update_time | DateTime64(6) | |
| scorecard_submit_time | DateTime64(6) | |
| scorecard_score | Float64 | Overall scorecard score |
| conversation_start_time | DateTime64(6) | |
| text_value | String | |
| language_code | String | |
| customer_id | String | |
| profile_id | String | |

**Primary key**: `(toStartOfHour(scorecard_time), scorecard_template_id, criterion_id, agent_user_id)`
**Order by**: `(toStartOfHour(scorecard_time), scorecard_template_id, criterion_id, agent_user_id, customer_id, profile_id, scorecard_id, score_id)`

### Important: ReplacingMergeTree deduplication

Both tables use `ReplacingMergeTree` with `update_time`. This means:
- Multiple rows may exist for the same scorecard/score until a merge happens
- For correct latest-state queries, use `FINAL` keyword or deduplicate manually:

```sql
-- Option 1: Use FINAL (slower but correct)
SELECT * FROM scorecard_d FINAL WHERE ...

-- Option 2: Manual dedup with max(update_time)
SELECT scorecard_id, argMax(score, update_time) as latest_score
FROM scorecard_d
WHERE ...
GROUP BY scorecard_id
```

## Common Queries

```sql
-- Count scores for a template in a time range
SELECT count(*) FROM score_d
WHERE scorecard_template_id = '<template_id>'
  AND scorecard_time >= '<start>'
  AND scorecard_time < '<end>'
  AND usecase_id = '<usecase>';

-- Check what criteria exist for a template
SELECT criterion_id, count(*) as cnt
FROM score_d
WHERE scorecard_template_id = '<template_id>'
GROUP BY criterion_id ORDER BY cnt DESC;

-- Weighted average QA score per agent (matching insights-server logic)
SELECT agent_user_id,
       SUM(percentage_value * float_weight) / SUM(float_weight) as avg_score,
       COUNT(DISTINCT conversation_id) as conversation_count,
       COUNT(DISTINCT scorecard_id) as scorecard_count
FROM score_d
WHERE scorecard_template_id = '<template_id>'
  AND scorecard_time >= '<start>' AND scorecard_time < '<end>'
  AND usecase_id = '<usecase>'
  AND percentage_value >= 0
  AND not_applicable != true
GROUP BY agent_user_id
ORDER BY avg_score DESC;

-- Check scorecard-level data
SELECT scorecard_id, agent_user_id, score, scorecard_time,
       scorecard_submit_time, manually_scored
FROM scorecard_d
WHERE scorecard_template_id = '<template_id>'
  AND scorecard_time >= '<start>' AND scorecard_time < '<end>'
ORDER BY scorecard_time DESC LIMIT 20;

-- List all tables in a database
SHOW TABLES;

-- List all databases (to find the right one)
SHOW DATABASES;
```

## Instructions

1. Ask the user for customer, profile, environment, and CH password if not provided
2. Derive the database name: `<customer>_<profile_with_hyphens_replaced_by_underscores>`
3. Always query distributed tables (`_d` suffix), not local tables
4. Use `--secure` flag always
5. Be aware of ReplacingMergeTree - use FINAL or manual dedup for exact counts
6. The `scorecard_time` column is the primary time filter (maps to conversation start time in most cases)
