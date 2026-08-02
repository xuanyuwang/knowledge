# Bug Report: Alaska Air Orphaned Scorecard

**Date:** 2026-07-29
**Domain:** Scorecard data sync
**Status:** Orphaned ClickHouse data repaired; root cause strongly supported, instrumented confirmation pending
**Slack:** https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785330122154789
**Ticket:** https://linear.app/cresta/issue/CONVI-7397/scorecard-is-empty-despite-being-scored-in-performance-insights
**Linear document:** https://linear.app/cresta/document/investigation-and-fix-35a0d178b84d

## Summary

Performance Insights returns scored criteria for a scorecard that the conversation drawer cannot load from PostgreSQL.

The strongest explanation is a concurrency race: an asynchronous writer captured PostgreSQL state before a reset, but inserted it into ClickHouse after the reset's delete mutation. The production evidence is strong, but the precise operation ordering still requires runtime-log confirmation.

## Affected Entity

- Customer/profile: `alaska-air/us-east-1`
- Conversation: `019f500c-ada9-707d-b407-39ffef8e4af8`
- Platform conversation: `300000010275588`
- Agent: `b9ec349698bf1afd`
- Template: `019e7591-67fb-77a9-9b2e-3b82cf20e0a8@941aee4e`
- Scorecard: `019f6c3d-2203-7037-8c88-a7f22223b575`

## User Impact

- Performance Insights counts the conversation as scored.
- Opening its conversation link displays an empty scorecard.
- Analytics includes data that cannot be inspected or managed through the PostgreSQL-backed scorecard UI.
- Confirmed scope for this instance: one scorecard and nine criterion rows.

## Evidence Timeline

- `2026-07-11 07:20:36.025874`: conversation started.
- `2026-07-16 18:46:40.732034`: surviving ClickHouse score rows' `update_time`.
- `2026-07-16 18:46:41.149712`: surviving ClickHouse scorecard row's `update_time`.
- `2026-07-16 18:46:55`: ClickHouse score deletion mutation created.
- `2026-07-29`: all rows remain visible on every replica checked.

The deletion was created approximately 14 seconds after the surviving rows were built.

## Operational Repair

On 2026-07-30, the orphaned data was backed up locally before deletion:

- Backup directory: `/Users/xuanyu.wang/repos/.cursor/backups/alaska-air-scorecard-019f6c3d-2203-7037-8c88-a7f22223b575-20260730T0319Z`
- `score.jsonl`: 9 rows, 12,746 bytes, SHA-256 `ef77bae721bb46628b28e64571915f565ab8e4b6719d8dcbcbe3ff3a170281b9`
- `scorecard.jsonl`: 1 row, 1,081 bytes, SHA-256 `2a573caf907af282d2a10d28151b7a18a02db0a0d4e12b443eb0b8bf28984858`
- `manifest.json` records the customer, profile, scorecard ID, source views, counts, sizes, and checksums.
- The directory and files were created under a restrictive `umask 077` and are outside the `knowledge` repository.

The first cluster-wide `ALTER TABLE ... ON CLUSTER conversations DELETE` attempt failed before reaching the owning shard because shard 3 reported Keeper connection loss and shutdown errors. Verification showed the target remained unchanged.

The repair then connected directly to a healthy replica of owning shard 1 and used the same shard-local lightweight-delete mechanism as the application:

- `score` mutation `0000004180`, created `2026-07-30 03:25:09`;
- `scorecard` mutation `0000004767`, created `2026-07-30 03:25:13`;
- both mutations report `is_done = 1` and no failure on all three shard-1 replicas.

Post-repair verification returned:

- `score_d FINAL`: 0 rows;
- `scorecard_d FINAL`: 0 rows;
- raw `score` across all replicas: 0 rows;
- raw `scorecard` across all replicas: 0 rows.

## Findings

### 1. Performance Insights is reading real ClickHouse rows — confirmed

Evidence:

- `scorecard_d FINAL` returned the exact scorecard.
- `score_d FINAL` returned nine distinct criterion rows.
- The API response contains the same scorecard ID, template revision, conversation, agent, and score IDs.
- `RetrieveQAConversations` joins the score and scorecard projections by `scorecard_id` and `scorecard_last_update_time` in `insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go`.

This is not a frontend-only artifact or stale browser state.

### 2. The rows are consistently replicated — confirmed

Evidence:

- `clusterAllReplicas('conversations', ...)` found nine `score` rows and one `scorecard` row on each of the three replicas of shard 1.
- All replicas returned identical identifiers and timestamps.

This rejects a single-replica read inconsistency.

### 3. A score-row deletion mutation targeted the exact scorecard — confirmed

Evidence:

- `system.mutations` contains mutation `0000004112` on all three shard replicas.
- Table: `score`.
- Created: `2026-07-16 18:46:55`.
- `is_done = 1`.
- `latest_fail_reason` is empty.
- The mutation predicate contains the exact conversation and scorecard IDs.
- The predicate matches `DeleteScoresOnShard` in `shared/clickhouse/conversations/scorecard_score.go`:

```sql
DELETE FROM score
WHERE toStartOfHour(scorecard_time) = toStartOfHour(toDateTime64(?, 0))
  AND conversation_id = ?
  AND scorecard_id = ?
SETTINGS replication_wait_for_inactive_replica_timeout = 0
```

The deletion did not report a ClickHouse execution failure.

This mutation alone does not prove that `ResetScorecard` triggered it, that PostgreSQL deletion succeeded, or that the ClickHouse `scorecard` row was deleted. It confirms only that a `score`-table mutation with the exact conversation and scorecard predicate was recorded and completed against the parts visible at that time.

### 4. No scorecard-level deletion mutation exists — confirmed

Evidence:

- The all-replica mutation query found the `score` mutation but no corresponding `scorecard` mutation for this scorecard ID.
- Reset executes `DeleteScoresOnShard` followed by `DeleteScorecardOnShard`.
- `DeleteScorecardOnShard` returns success without creating a mutation when shard discovery returns no scorecard row.

This is consistent with the scorecard row not being visible during deletion and being inserted afterward.

### 5. Reset can remove PostgreSQL state before asynchronous ClickHouse cleanup — confirmed by code

For a non-AI scorecard, `resetScorecardDataAtomic`:

1. deletes `director.scores`;
2. deletes `director.scorecards`;
3. commits the PostgreSQL transaction;
4. schedules `asyncDeleteScorecardWork`.

Source: `apiserver/internal/coaching/action_reset_scorecard.go`.

The observed mutation is strong evidence that this deletion path, or an equivalent caller, ran.

### 6. The asynchronous writer has an unprotected race window — confirmed by code

`scorecardAsyncWorkReadFromDB`:

1. reads the scorecard from the PostgreSQL write connection;
2. reads related user, conversation, and score data;
3. builds ClickHouse rows;
4. writes score rows;
5. writes the scorecard row.

There is no ordering guard between the initial PostgreSQL read and the ClickHouse writes. Reset can delete the source and execute its ClickHouse mutation during that interval.

Source: `apiserver/internal/coaching/action_create_scorecard.go`.

### 7. Existing tests do not exercise the suspected interleaving — confirmed

`TestCreateAndResetAsyncWorkOutOfOrder` executes:

1. reset async work to completion;
2. create async work afterward.

The writer then observes that PostgreSQL is already empty and correctly skips its write.

The test does not cover:

1. writer reads PostgreSQL;
2. reset deletes PostgreSQL and ClickHouse;
3. writer resumes and inserts its previously built rows.

Source: `apiserver/internal/coaching/action_scorecard_async_order_test.go`.

## PostgreSQL Evidence Gap

The supplied PostgreSQL query proves there is no scorecard matching the specified customer, profile, template, and agent combination.

It does not prove that the exact scorecard ID is absent or that no orphaned `director.scores` rows exist. The attempted exact-ID PostgreSQL query stalled before returning, leaving these alternatives unresolved:

- the scorecard exists under a different agent or template identity;
- the scorecard row is absent but score rows remain;
- both scorecard and score rows are absent.

The empty drawer and completed deletion mutation strongly support source deletion, but exact-ID PostgreSQL evidence is still required for full confirmation.

## Root-Cause Assessment

**Most likely root cause — high confidence, pending instrumented confirmation:**

An asynchronous scorecard writer read and built the scorecard before reset. Reset then deleted PostgreSQL state and issued the ClickHouse deletion. The stale writer inserted rows after the mutation snapshot. ClickHouse lightweight deletion only affected rows present when the mutation executed, so the later insertion survived indefinitely.

## Hypothesis Status

- **H1 — orphaned ClickHouse projection:** strongly supported.
- **H2 — PostgreSQL identity mismatch made the original query a false negative:** inconclusive.
- **H3 — partial PostgreSQL deletion:** inconclusive.
- **H4 — async writer inserted after reset/delete:** strongly supported and the leading root cause.

## Instrumentation

Nine scoped NDJSON probes are active in:

- Worktree: `/Users/xuanyu.wang/repos/go-servers-alaska-scorecard-orphan`
- Branch: `debug/alaska-scorecard-orphan`
- Log: `/Users/xuanyu.wang/repos/.cursor/debug-11346f.log`

They cover:

- analytics request and ClickHouse result inventory;
- exact-ID PostgreSQL scorecard and score diagnostics;
- asynchronous writer snapshot, write start, and completion;
- reset deletion start, score deletion completion, and scorecard deletion completion.

## Recommended Remediation

1. Query PostgreSQL by exact scorecard ID in both `director.scorecards` and `director.scores`.
2. Reproduce the concurrent read/delete/write sequence using synchronization hooks rather than timing sleeps.
3. Introduce ordering semantics that allow deletion to dominate older writes, preferably a monotonic source version plus a durable deletion tombstone.
4. Add a concurrent test where the writer pauses after its PostgreSQL read, reset completes, and the writer then resumes.
5. Monitor the repaired scorecard ID for reappearance; ordinary reindexing cannot reconstruct a scorecard absent from PostgreSQL and a still-active stale writer could insert it again.

## Related Artifacts

- Work item: `../work-items/alaska-air-orphaned-scorecard.md`
- Investigation session: `../sessions/2026-07-29/codex-alaska-orphaned-scorecard.md`
- Domain architecture: `architecture-and-invariants.md`
- Diagnosis ladder: `monitoring-and-diagnosis.md`
- Repair playbook: `repair-and-backfill-playbook.md`
