# Session Note - 2026-07-28 - Codex - Customer ClickHouse Skill

**Started:** 2026-07-28 11:58 EDT
**Tool:** Codex
**Project:** `dev-environment-tips`
**Goal:** Verify and automate secure customer-to-ClickHouse connection discovery.

## Source Context

- **Primary repo:** `knowledge`
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch:** `main`
- **Related repos:** `/Users/xuanyu.wang/repos/config`, `/Users/xuanyu.wang/repos/xuanyu-scripts`, `/Users/xuanyu.wang/repos/flux-deployments`
- **Ticket / PR:** none

## Inputs Reviewed

- Latest `origin/master` of the `config` repository at `4eb343749909eebba65bc6cd33caae65d2823401`
- `/Users/xuanyu.wang/repos/xuanyu-scripts/connect-clickhouse.zsh`
- Config v3 customer placement fields and generated schema descriptions
- ClickHouse deployment host and Secrets Manager conventions in `flux-deployments`
- ClickHouse client environment-variable documentation

## Findings

- `deployment.k8sCluster` is a viable source for the ClickHouse deployment environment and corresponding `<cluster>_dev` AWS profile.
- The deployed host convention is `clickhouse-<service>.<k8sCluster>.internal.cresta.ai`; the admin secret convention is `clickhouse/<k8sCluster>/users/admin`.
- `analyticsDbNameOverride` describes the PostgreSQL analytics database and cannot be treated as authoritative for ClickHouse.
- The ClickHouse database must be resolved against the live `system.databases` catalog. For example, Oportun profile `us-west-2` maps to `oportun_us_west_2`.
- The old helper prints the password and puts it in process arguments. The replacement keeps it in memory and passes it only through `CLICKHOUSE_PASSWORD`.

## Actions Summary

- Created personal auto-invoked skill `~/.cursor/skills/connect-customer-clickhouse/`.
- Added a Zsh helper that fetches the latest config remote without changing the user's checkout, resolves profile ambiguity, performs AWS SSO login when needed, retrieves the secret, resolves the live database, and opens or queries ClickHouse.
- Added safeguards against printing, logging, copying, or persisting credentials.

## Validation

- Shell syntax validation passed.
- IDE lint checks reported no errors.
- End-to-end Oportun validation connected to `us-west-2-prod` and returned `oportun_us_west_2` from `SELECT currentDatabase()`.
- Cox validation stopped before secret retrieval and listed profiles because the customer spans multiple clusters and profiles.

## Follow-ups

- Use `--profile` when a customer has multiple deployments.
- Keep database resolution catalog-backed; do not replace it with a config-only naming assumption.
