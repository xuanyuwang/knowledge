# Alaska Air orphaned scorecard investigation

## Source context

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Debug worktree: `/Users/xuanyu.wang/repos/go-servers-alaska-scorecard-orphan`
- Branch: `debug/alaska-scorecard-orphan`
- Slack report: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785330122154789
- Debug session: `11346f`

## Symptom

Performance Insights returns score rows for scorecard `019f6c3d-2203-7037-8c88-a7f22223b575`, but the conversation scorecard drawer is empty and the expected PostgreSQL lookup finds no scorecard for agent `b9ec349698bf1afd` and template `019e7591-67fb-77a9-9b2e-3b82cf20e0a8`.

## Hypotheses

- H1: ClickHouse contains an orphaned scorecard projection whose PostgreSQL source row was deleted.
- H2: PostgreSQL still contains the scorecard, but with a different agent/template identity, making the supplied query a false negative.
- H3: PostgreSQL contains only a partial entity (scorecard without scores or scores without scorecard).
- H4: asynchronous scorecard write and reset/delete work overlapped, allowing a stale write to land after the ClickHouse delete mutation.

## Runtime evidence collected

- `scorecard_d FINAL` contains the exact scorecard with update time `2026-07-16 18:46:41.149712`.
- `score_d FINAL` contains nine criterion rows with update time `2026-07-16 18:46:40.732034`.
- `clusterAllReplicas('conversations', system.mutations)` shows mutation `0000004112` on all three replicas of shard 1:
  - created `2026-07-16 18:46:55`;
  - targets the exact conversation and scorecard;
  - marks `_row_exists = 0`;
  - reports `is_done = 1` and no failure.
- Despite the completed mutation, all three replicas currently expose the same nine score rows and one scorecard row.
- No matching `scorecard` mutation is recorded for the scorecard ID.
- Code inspection shows reset hard-deletes non-AI scorecards from PostgreSQL, then asynchronously deletes `score` followed by `scorecard` in ClickHouse.
- The writer re-reads PostgreSQL once, builds rows, then performs ClickHouse writes. A concurrent reset can occur after the read/build but before insertion. The existing out-of-order test runs closures sequentially and does not exercise this in-flight interleaving.

## Current assessment

H4 is strongly supported by production timing: surviving rows were built around `18:46:40-41`, and the completed deletion was created at `18:46:55`. The likely interleaving is that stale work captured/built the pre-reset state, reset deleted PostgreSQL and issued the ClickHouse mutation, and the stale insertion landed after the mutation snapshot. The missing scorecard mutation is consistent with shard lookup occurring before the stale scorecard row became visible.

H1 is supported by the reported PostgreSQL absence and the ClickHouse rows. H2 and H3 still require an exact-ID PostgreSQL read; the secure app-DB connection stalled before returning results.

## Instrumentation

Nine collapsible NDJSON debug logs were added:

- request and ClickHouse result inventory;
- exact-ID PostgreSQL scorecard/score diagnostics;
- writer snapshot, write start, and write completion;
- reset deletion start, score deletion completion, and scorecard deletion completion.

Logs write only to `/Users/xuanyu.wang/repos/.cursor/debug-11346f.log`.

## Next step

Run the instrumented service and reproduce the drawer request. If possible, separately reproduce a create/submit versus reset overlap to capture the writer/deleter sequence. Analyze the clean debug log before changing behavior.

## 2026-07-30 operational repair

- Exported `score_d FINAL` and `scorecard_d FINAL` to `/Users/xuanyu.wang/repos/.cursor/backups/alaska-air-scorecard-019f6c3d-2203-7037-8c88-a7f22223b575-20260730T0319Z`.
- Verified 9 score rows and 1 scorecard row, then recorded file sizes and SHA-256 checksums in `manifest.json`.
- A cluster-wide delete attempt failed on shard 3 with Keeper connection-loss and shutdown errors and did not alter the target on shard 1.
- Connected directly to healthy shard-1 replica `chi-conversations-conversations-0-0-0` and issued exact, shard-local lightweight deletes.
- New mutations:
  - `score`: `0000004180`, complete without failure on all shard-1 replicas;
  - `scorecard`: `0000004767`, complete without failure on all shard-1 replicas.
- Final verification:
  - `score_d FINAL`: 0;
  - `scorecard_d FINAL`: 0;
  - raw `score` across all replicas: 0;
  - raw `scorecard` across all replicas: 0.
