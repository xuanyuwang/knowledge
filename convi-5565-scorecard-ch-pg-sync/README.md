# CONVI-5565: Scorecard ClickHouse ↔ PostgreSQL Sync

**Created**: 2025-01-16
**Updated**: 2026-02-23
**Linear**: https://linear.app/cresta/issue/CONVI-5565/data-sync-between-ch-and-postgres
**Related**: CONVI-6076 (PostgreSQL race condition)

## Overview

Investigation and fix for scorecard data inconsistency between PostgreSQL (source of truth) and ClickHouse (analytics). Async work from UpdateScorecard and SubmitScorecard APIs could finish out of order, causing ClickHouse to keep stale data.

## Key Documents

| File | Description |
|------|-------------|
| `investigation.md` | Full investigation: root cause, 3 fix attempts, load test results |
| `tools/verify_sync/main.go` | Source: compare PG vs CH for a single scorecard |
| `tools/test_async_order/main.go` | Source: load test concurrent Update+Submit race condition |

## Summary of Fixes

1. **Fix 1 (reverted)**: Use PG `updated_at` for CH timestamp — failed because async work read stale closure data
2. **Fix 2 (merged, PR #24103)**: Atomic transactions — move historic writes inside transaction, async work re-reads from DB
3. **Fix 3 (merged)**: GORM `Omit` — prevent UpdateScorecard from overwriting `submitted_at`/`submitter_user_id`

## Verification Tools

Source code is preserved in `tools/` in this repo. These tools must be built from the `go-servers` repo since they depend on internal packages and proto definitions.

### 1. `verify_sync` — Compare PG vs CH for a single scorecard

**Source**: `tools/verify_sync/main.go` (copy from `go-servers/tools/verify_sync/`)

Compares a single scorecard across PostgreSQL and ClickHouse, checking:
- Scorecard exists in both
- Score count matches
- Submitted state consistency (submit_time, submitter)
- Overall score value
- Individual score values (numeric, AI, text, not_applicable, ai_scored)
- Shows RAW (all versions) and FINAL (merged) CH data

**Dependencies** (from BUILD.bazel):
- `//shared/scoring` (go-servers internal)
- `clickhouse-go/v2`
- `lib/pq`

```bash
# Option A: Build with Bazel (from go-servers root)
bazel build //tools/verify_sync

# Option B: Build with go build (from go-servers root)
cd tools/verify_sync && go build -o verify_sync .

# Run — verifies one scorecard
./verify_sync -name "customers/cox/profiles/sales/scorecards/<scorecard-id>"

# With explicit PG connection string
./verify_sync -name "customers/cox/profiles/sales/scorecards/<id>" \
  -pg "postgres://user:pass@host:5432/dbname"
```

**Default connections:**
- PostgreSQL: auto-fetched via `cresta-cli connstring -i --read-only chat-staging chat-staging cox-sales`
- ClickHouse: `clickhouse-conversations.chat-staging.internal.cresta.ai:9440` (admin/hardcoded)

### 2. `test_async_order` — Load test for concurrent API race condition

**Source**: `tools/test_async_order/main.go` (copy from `go-servers/tools/test_async_order/`)

Creates scorecards, calls Update and Submit concurrently with configurable delay, then verifies CH data.

**Dependencies** (from BUILD.bazel):
- `cresta-proto//cresta/v1/coaching` (gRPC client)
- `clickhouse-go/v2`
- `grpc` + `grpc/credentials` + `grpc/metadata`

```bash
# Option A: Build with Bazel (from go-servers root)
bazel build //tools/test_async_order

# Option B: Build with go build (from go-servers root)
cd tools/test_async_order && go build -o test_async_order .

# Default: 50 iterations, 10ms API delay, 2s CH wait
./test_async_order -iterations 50

# Custom API delay (100ms+ = ~0% failure rate)
./test_async_order -iterations 50 -api-delay 100 -wait 3

# Quick smoke test
./test_async_order -iterations 10 -api-delay 200 -wait 5
```

**Default config:** chat-staging, cox/sales customer/profile.
**Auth:** uses `cresta-cli cresta-token chat-staging cox --bearer`

## Key Findings

- **PostgreSQL is always correct** after GORM Omit fix
- **ClickHouse converges** when API calls are spaced >= 100ms apart
- ~10-20% failure rate only under extreme concurrency (< 10ms between calls)
- Real-world users don't trigger this — documented as acceptable limitation

## Log History

| Date | Summary |
|------|---------|
| 2025-01-16 | Investigation started |
| 2025-01-18 | Fix 1 reverted; Fix 2 (atomic transactions) merged as PR #24103 |
| 2025-01-18 | Discovered CONVI-6076 PostgreSQL race condition |
| 2025-01-23 | Fix 3 (GORM Omit) applied; load testing started |
| 2025-01-27 | Load testing complete; investigation documented |
| 2026-02-23 | Moved to knowledge repo; documented verification tools |
