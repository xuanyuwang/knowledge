# Comcast production database connection parameters

- Source repo: `/Users/xuanyu.wang/repos/config` (read-only checkout on `master`, validated against `origin/master` at `e0d6dd6dfdbcc8e1ac03114159da300e19cc65b7`).
- No source repo changes or customer data query were made. Follow-up verification ran `SELECT 1` against both databases.

## Access policy

- [How to Get Cresta Prod Access](https://docs.superhuman.com/d/_ddxoF0QlIUp/_su8BX22W) says an approved per-customer Cresta Prod Access grant is required for production RDS access. Geo-group membership is a separate requirement.
- `configv3/prod/comcast/config.yaml` has `enableCrestaOrgGroupRestriction: true`, with `application_access_cresta prod_customer_comcast`, on-call, and QA groups. `allowedOktaGeoGroups` includes Canada. The current group membership of this workstation's user was not verified.
- [Geographical Customer Data Access Restrictions](https://docs.superhuman.com/d/_dcb8CvmtLT6/_suiDVWGq) describes location restrictions and warns against access outside approved locations. Glean's `non_us_policy_documents.tf` search did not list Comcast among customers blocked outside the US; the Comcast config is the narrower current customer-specific evidence.

## Config-derived targets

- App PG: `configv3/prod/comcast/prod/config.yaml` has `profileId: prod`, `deployment.k8sCluster: comcast-prod`, and `deployment.sqlConn` pointing to the `comcast-prod/comcast` RDS secret in AWS account `233896340203`, region `us-east-2`. The read-only `cresta-cli` arguments are `comcast-prod comcast-prod comcast`.
- Auth PG is separate: customer-level `authDbConnArn` identifies cluster `auth-comcast-prod` and database `comcast`.
- Conversations ClickHouse: deployment cluster `comcast-prod`; native secure endpoint `clickhouse-conversations.comcast-prod.internal.cresta.ai:9440`. `clickhouse-schema/conversations/scripts/create_database_and_tables_if_absent/main.py:171-173` derives database name as `customer_id + '_' + profile_id`: `comcast_prod`. The read-only launcher resolved this database and `SELECT 1` succeeded. `clickhouse-schema/conversations/scripts/github_actions/run_operation.sh` identifies the admin secret path `clickhouse/comcast-prod/users/admin` in account `233896340203`, `us-east-2`. Never log or print its password.

## Validation boundary

- The Comcast SSO profile was refreshed. AWS STS confirmed `comcast-prod_dev` in account `233896340203`, role `DevLimitedModify_Canada`.
- The standard PG launcher initially failed because `cresta-cli` v0.145.3 rejects dedicated `comcast-prod` as an invalid account. A retry with `--force --profile comcast-prod_dev` failed because the CLI's cached profile list was stale. `--clear-cache` resolved that. The final invocation retained `--read-only`, captured the URI only in process memory, and `SELECT 1` succeeded against app PG.
- AWS RDS `DescribeDBClusters` confirmed the read-only Aurora PG endpoint `comcast-prod.cluster-ro-cla40uiwg4pu.us-east-2.rds.amazonaws.com:5432`.
- No credential-bearing URI or password was printed or saved. The local config still had an unrelated worktree modification; it was not changed by this work.
- During local configuration inspection, a restricted AWS credential entry was inadvertently displayed. It was not used to authenticate or passed to any database tool, and no credential values were copied into this note.
