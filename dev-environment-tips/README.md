# Dev Environment Tips

**Created:** 2026-02-07
**Updated:** 2026-07-28

## Overview

macOS, PostgreSQL, tooling setup tips and fixes for local development environment.

## Key Topics

### AI Workspace Layout

Current recommendation: keep `knowledge` as the durable context repository and keep source repositories as siblings under `/Users/xuanyu.wang/repos`. For AI tools that need a Git-backed root, prefer opening `/Users/xuanyu.wang/repos` as the broader workspace root or consider making `/Users/xuanyu.wang/repos` a lightweight workspace metadata repository that ignores nested source repositories.

Avoid moving all source repositories under `knowledge` by default. It can be made lean with `.gitignore`, but it introduces nested Git repositories and may reduce tool visibility into ignored child repos when opened from the `knowledge` root.

### Customer ClickHouse Connections

The personal Cursor skill at `~/.cursor/skills/connect-customer-clickhouse/` resolves a customer's deployment from the latest config remote, retrieves the ClickHouse admin credential through AWS SSO and Secrets Manager, resolves the live database, and opens the client without printing or persisting the secret.

Use `--profile` for customers with multiple deployments. Treat `deployment.k8sCluster` as the ClickHouse environment, but resolve the database against `system.databases`; `analyticsDbNameOverride` is not authoritative for ClickHouse.

### Customer PostgreSQL Connections

The personal Cursor skills at `~/.cursor/skills/connect-customer-app-db/` and `~/.cursor/skills/connect-customer-auth-db/` resolve database metadata from the latest config remote and open read-only `psql` sessions through `cresta-cli`.

App DB metadata comes from `deployment.prodDbConnArn` / `deployment.sqlConn`; auth DB metadata comes from `authConfig.authDbConnArn`, with `customerId` as the database name. Credential-bearing URIs remain in memory and are converted to `PG*` environment variables before `psql` starts.

### macOS PostgreSQL Shared Memory Issue

When running Go tests with embedded PostgreSQL on macOS, you may get:
```
FATAL: could not create shared memory segment: No space left on device
```

This is NOT a disk space issue - it's macOS kernel limit of **32 shared memory segments system-wide**.

**Quick Fix:**
```bash
pkill -u $(whoami) -f "embedded-postgres-go"
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -n1 ipcrm -m 2>/dev/null
```

**Long-term Solution:** Use external PostgreSQL instead of embedded.

See `log/2025-12-18.md` for full details.

## Log History

| Date | Summary |
|------|---------|
| 2026-07-28 | Added and validated secure customer ClickHouse, app DB, and auth DB connection skills |
| 2026-07-03 | Evaluated AI workspace layout options; recommended preserving sibling repos and considering a lightweight workspace-meta repo |
| 2025-12-18 | PostgreSQL shared memory issue - diagnosis, quick fix, and long-term solution |
