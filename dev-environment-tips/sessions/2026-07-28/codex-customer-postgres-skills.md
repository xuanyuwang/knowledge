# Session Note - 2026-07-28 - Codex - Customer PostgreSQL Skills

**Started:** 2026-07-28 12:08 EDT
**Tool:** Codex
**Project:** `dev-environment-tips`
**Goal:** Create secure customer app DB and auth DB connection skills.

## Source Context

- **Primary repo:** `knowledge`
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch:** `main`
- **Related repos:** `/Users/xuanyu.wang/repos/config`, `/Users/xuanyu.wang/repos/tenant-admin`, `github.com/cresta/cresta-cli`
- **Ticket / PR:** none

## Inputs Reviewed

- Latest `origin/master` customer and profile configs
- Tenant Admin database action builders in `ProfileActionsCell.tsx`
- `cresta-cli connstring` help and version `0.137.0`
- `cresta-cli` AWS, GCP, and command implementations at commit `a86825fad72e72b086e95dc6278531af7f85925c`
- Existing `xuanyu-scripts/connect-postgres.zsh`

## Findings

- App DB source of truth is `deployment.prodDbConnArn`; raw config currently exposes the same value as `deployment.sqlConn`.
- Auth DB source of truth is `authConfig.authDbConnArn`, with `customerId` as the auth database name.
- Tenant Admin maps a `us-west-2` production auth DB to account `chat-prod`, other production regions to `<region>-prod`, and staging to `chat-staging`.
- `cresta-cli connstring -i --read-only` is the canonical access path. Its IAM token is not cached, although non-secret argument-discovery metadata may be cached.
- Passing the returned URI to `psql` as an argument or using `open $(...)` exposes credentials in process arguments. The new launchers parse the URI in memory, populate `PG*` environment variables, remove the URI, and then execute `psql`.
- `cresta-cli` enforces read-only access for AI agent environments; the skills preserve that guard.

## Actions Summary

- Created `~/.cursor/skills/connect-customer-app-db/`.
- Created `~/.cursor/skills/connect-customer-auth-db/`.
- Added AWS parsing, generic GCP resource parsing and explicit overrides, profile ambiguity handling, and read-only query support.
- Avoided the existing helper because it prints and copies credential-bearing connection strings.

## Validation

- Shell syntax and IDE lint checks passed.
- App DB validation connected Oportun profile `us-west-2` to database `oportun` on cluster `us-west-2-prod` as `oportun-iam`.
- Auth DB validation connected Oportun to database `oportun` on cluster `auth-prod` through account `chat-prod` as `oportun-iam`.
- Cox app DB validation stopped before credential retrieval and listed profiles because the customer has multiple app databases.
- Concurrent app/auth validation passed after adding bounded retries for config remote-ref lock races.

## Follow-ups

- Use `--profile` for multi-profile app DB customers.
- GCP config resource formats were not present in the local examples reviewed; use explicit `--account`, `--db-cluster`, and `--database` overrides if automatic resource parsing cannot resolve them.
