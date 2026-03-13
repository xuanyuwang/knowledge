---
name: query-pg-coaching
description: Query coaching/QA data from Cresta PostgreSQL databases (scorecard templates, scorecards, criteria, coaching plans). Use when the user needs to look up PG data in the director schema.
user-invocable: true
allowed-tools: Bash(*psql*), Bash(*cresta-cli*), Read
argument-hint: "[customer] [profile] [cluster] [query description]"
---

# Query Coaching Data from PostgreSQL

Query coaching/QA data from Cresta PostgreSQL databases.

User request: $ARGUMENTS

## Connection Setup

### psql location

```
/opt/homebrew/opt/postgresql@15/bin/psql
```

### Build connection string with cresta-cli

```bash
# Usage: cresta-cli connstring [-r] <account> <cluster> <database>
# The <database> is the profile name
# IMPORTANT: AWS_REGION must match the cluster region

# Staging (us-west-2)
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only <cluster> <cluster> <profile>) && /opt/homebrew/opt/postgresql@15/bin/psql "$CONN"

# Examples:
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only voice-staging voice-staging walter-dev)
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only chat-staging chat-staging cresta-sales)

# Production (match region to cluster)
AWS_REGION=us-east-1 CONN=$(cresta-cli connstring -i --read-only us-east-1-prod us-east-1-prod <profile>)
AWS_REGION=us-west-2 CONN=$(cresta-cli connstring -i --read-only us-west-2-prod us-west-2-prod <profile>)
```

**Warning**: Do NOT use `--clear-cache` flag unless absolutely necessary.

### Running a query

```bash
AWS_REGION=<region> CONN=$(cresta-cli connstring -i --read-only <cluster> <cluster> <profile> 2>&1) && /opt/homebrew/opt/postgresql@15/bin/psql "$CONN" -c "<SQL QUERY>" 2>&1
```

## Schema

All coaching/QA tables live in the **`director`** schema. Source of truth: `go-servers/apiserver/sql-schema/director/director-schema.sql`

### Key Tables

#### `director.scorecard_templates`
Scorecard template definitions (each revision is a separate row).

| Column | Type | Notes |
|--------|------|-------|
| customer | VARCHAR | PK part |
| profile | VARCHAR | PK part |
| resource_id | VARCHAR | Template ID (PK part) |
| revision | VARCHAR | Template revision (PK part) |
| title | VARCHAR | Display name |
| template | JSONB | Full template definition including criteria (items[].identifier, items[].displayName, items[].settings, items[].auto_qa) |
| usecase_ids | VARCHAR[] | Associated usecases |
| status | SMALLINT | 1=active |
| deactivated_at | TIMESTAMPTZ | Null if active |
| created_at | TIMESTAMPTZ | |
| creator_user_id | VARCHAR | |
| qa_task_config | JSONB | |
| qa_score_config | JSONB | |

#### `director.scorecard_template_revisions`
Revision history for templates.

| Column | Type | Notes |
|--------|------|-------|
| customer, profile | VARCHAR | PK parts |
| resource_id | VARCHAR | Revision ID (PK part) |
| template_id | VARCHAR | References scorecard_templates.resource_id (PK part) |
| template | JSONB | Template content at this revision |
| title | VARCHAR | |
| created_at | TIMESTAMPTZ | |

#### `director.scorecards`
Individual scorecard instances (one per conversation evaluation).

| Column | Type | Notes |
|--------|------|-------|
| customer, profile | VARCHAR | PK parts |
| resource_id | VARCHAR | Scorecard ID (PK) |
| conversation_id | VARCHAR | |
| agent_user_id | VARCHAR | Agent being evaluated |
| creator_user_id | VARCHAR | Who created the scorecard |
| template_id | VARCHAR | FK to scorecard_templates |
| template_revision | VARCHAR | FK to scorecard_templates |
| score | FLOAT | Overall score |
| submitted_at | TIMESTAMPTZ | When submitted (null = draft) |
| ai_scored_at | TIMESTAMPTZ | When AI scored |
| manually_scored | BOOLEAN | |
| auto_failed | BOOLEAN | |
| usecase_id | VARCHAR | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

### Common Queries

```sql
-- Find a scorecard template and its criteria
SELECT resource_id, title, status, created_at,
       template->'items' as criteria
FROM director.scorecard_templates
WHERE resource_id = '<template_id>'
ORDER BY created_at DESC LIMIT 1;

-- List criterion identifiers from a template
SELECT resource_id, title,
       jsonb_array_elements(template->'items')->>'identifier' as criterion_id,
       jsonb_array_elements(template->'items')->>'displayName' as criterion_name
FROM director.scorecard_templates
WHERE resource_id = '<template_id>'
ORDER BY created_at DESC LIMIT 1;

-- Count scorecards for a template in a time range
SELECT count(*), min(created_at), max(created_at)
FROM director.scorecards
WHERE template_id = '<template_id>'
  AND created_at >= '<start>' AND created_at < '<end>';

-- Find scorecards for a specific agent
SELECT resource_id, conversation_id, score, submitted_at, manually_scored
FROM director.scorecards
WHERE agent_user_id = '<user_id>'
  AND template_id = '<template_id>'
ORDER BY created_at DESC LIMIT 10;
```

## Instructions

1. Ask the user for customer, profile, and cluster if not provided
2. Build the connection string using `cresta-cli connstring`
3. Run the query using `/opt/homebrew/opt/postgresql@15/bin/psql`
4. Always use `--read-only` flag unless write access is explicitly needed
5. All coaching tables are in the `director` schema - always prefix table names with `director.`
