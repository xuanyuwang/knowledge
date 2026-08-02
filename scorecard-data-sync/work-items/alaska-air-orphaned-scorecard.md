# Alaska Air orphaned scorecard

## Objective

Explain and prevent Performance Insights from returning ClickHouse score rows after the corresponding PostgreSQL scorecard has been removed.

## Status

Orphaned ClickHouse data was backed up and removed on 2026-07-30. Production evidence strongly supports an in-flight asynchronous write racing a reset/delete, but the preventive code fix remains pending instrumented confirmation of the operation ordering.

## Scope

- Ticket: https://linear.app/cresta/issue/CONVI-7397/scorecard-is-empty-despite-being-scored-in-performance-insights
- Linear investigation: https://linear.app/cresta/document/investigation-and-fix-35a0d178b84d
- Customer/profile: `alaska-air/us-east-1`
- Conversation: `019f500c-ada9-707d-b407-39ffef8e4af8`
- Scorecard: `019f6c3d-2203-7037-8c88-a7f22223b575`
- Template: `019e7591-67fb-77a9-9b2e-3b82cf20e0a8@941aee4e`
- Source worktree: `/Users/xuanyu.wang/repos/go-servers-alaska-scorecard-orphan`
- Branch: `debug/alaska-scorecard-orphan`

## Evidence

- Before repair, nine `score` rows and one `scorecard` row remained on every replica of ClickHouse shard 1.
- A completed, error-free score-row deletion mutation targeting the exact scorecard was created 15 seconds after the surviving rows' embedded update times.
- No scorecard deletion mutation from the original incident was present.
- Reset hard-deletes non-AI scorecards in PostgreSQL and performs ClickHouse deletion asynchronously.
- Async writers read PostgreSQL before building and writing ClickHouse rows, leaving an unguarded interval where reset can delete and a stale writer can subsequently insert.
- The 2026-07-30 repair mutations completed without failure on all shard-1 replicas; distributed and raw all-replica verification now return zero rows.

## Validation state

- ClickHouse exact-ID and all-replica checks: complete.
- Local backup and ClickHouse cleanup: complete.
- Exact-ID PostgreSQL check: blocked by a stalled secure connection.
- Instrumented reproduction: pending.
- Fix and post-fix verification: not started.

## Timeline

- 2026-07-29: triaged Slack report, identified the completed-but-bypassed ClickHouse deletion, created debug worktree, and added nine runtime probes. See `../sessions/2026-07-29/codex-alaska-orphaned-scorecard.md`.
- 2026-07-29: published the evidence-backed bug report at `../deliverables/alaska-air-orphaned-scorecard-report.md`.
- 2026-07-30: backed up 9 score rows and 1 scorecard row locally, deleted both entities from owning ClickHouse shard 1, and verified zero rows through distributed `FINAL` views and raw all-replica queries.
- 2026-07-30: published the full investigation, operational repair, checksums, and lossless Markdown representation of the backed-up records to the Linear document attached to CONVI-7397.
