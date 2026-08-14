# ca-central Phase A retry + sandbox column (2026-08-05)

## Task 1 — ca-central-1-prod retry

- Triggered: https://github.com/cresta/clickhouse-schema/actions/runs/31017776702
- Inputs: `cluster_target=ca-central-1-prod`, `dry_run=false`, ref `main`
- Job green (~1m38s) but **Failed DBs**: `cresta_ca_ca_central_1`, `telus_ca_central_1`
- Same Code **209** socket connect timeout to `clickhouse-conversations.ca-central-1-prod.internal.cresta.ai:9440`
- Preflight skipped (kubectl IAM Forbidden; Groundcover MCP disconnected)
- Infra connectivity blocker, not DDL

## Task 2 — `moment_annotation_payload` on `indeed_mg_sbx_us_west_2`

- Failure mode: Phase A CREATE MV selects `moment_annotation_payload` → Code 47 missing column
- Ready artifacts in `cresta/clickhouse-schema`:
  - `history_queries/queries_add_moment_annotation_payload.sql`
  - `alter_tables_in_existing_database/changes_by_table/moment_annotation.txt` (`ADD COLUMN IF NOT EXISTS`)
  - Verification: `historical_verification_scripts/check_moment_annotation_payload_field.*`
- Prefer **Alter Tables** GHA while Phase A DDL lives in `queries.sql` (do not overwrite `queries.sql` with history file)
- Sandbox evidence: customer `indeed-mg-sbx`, config `skipSchemaRelease: true` in `customerv2/indeed-mg-sbx-us-west-2.yaml`
- Recommendation: **skip** for CLO Phase A; optional hygiene via Alter Tables then targeted Run Queries with `customer_name=customers/indeed-mg-sbx`

## Linear

- Comment posted on CONVI-7431
- Ticket remains **In Progress** (ca-central blocker)
