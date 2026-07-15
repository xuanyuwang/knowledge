# Repair and Backfill Playbook

## Repair Decision

Choose the narrowest production path that rebuilds the canonical projection logic.

| Situation | Preferred action |
|---|---|
| One missing or stale scorecard with correct PG state | Targeted `JOB_TYPE_REINDEX_SCORECARDS` by resource name |
| Missing process scorecards | Scorecard-centric reindex using process scorecard IDs or a bounded process range |
| Many conversation scorecards in a known time range | Conversation reindex/backfill with conservative windows |
| Existing CH rows are no longer eligible | Explicit CH cleanup, then reindex the desired set |
| Monitor finds missing IDs inside safety threshold | Auto-heal/targeted reindex after eligibility filtering |
| PG state itself is wrong | Stop; repair the workflow/source-of-truth problem before CH |

Do not write ad hoc scorecard rows directly to ClickHouse. Prefer production projection code so transformation, sharding, and version fields stay consistent.

## Targeted Reindex

Use targeted scorecard reindex for an existing-but-stale row; the missing-ID monitor will not detect it.

After completion, compare:

- scorecard-level score/state/identity/timestamps;
- emitted criterion ID set and values;
- zero/default submit timestamps;
- latest CH version using `FINAL` or equivalent.

## Conversation vs Process Recovery

- Conversation backfill is driven by conversations and is appropriate only for conversation-backed scorecards.
- Process scorecards require `JOB_TYPE_REINDEX_SCORECARDS`; they are not recovered by `BatchIndexConversations`.
- Process sharding depends on process interaction time rather than conversation lookup.

## Cleanup Before Reindex

Reindex is normally insert/upsert oriented. If a code change removes entities or criteria from the desired projection, old rows can remain.

For cleanup:

1. Establish the exact customer/profile, table, IDs/time range, and desired post-cleanup count.
2. Delete from local ClickHouse tables with `ON CLUSTER`, not only distributed `_d` tables or a single shard.
3. Understand mutation scope and backlog before launching more deletes.
4. Reindex/backfill only after the cleanup boundary is explicit.
5. Validate across the distributed view after mutations and new writes converge.

Never copy credentials into commands saved in this repository.

## Large Backfills

Historical runs show the main scaling constraint is often PostgreSQL conversation read volume, not CH write volume.

- Start with a conservative window.
- Use 5–10 day windows for large customers only after measuring.
- Fall back to one-day sequential windows for the largest profiles or when concurrency causes deadlocks/timeouts.
- Track progress durably so VPN/process interruption does not require restarting completed windows.
- Avoid high parallelism during business traffic; weekend parallelism was effective in historical runs but is not a universal guarantee.
- Set execution/polling timeouts based on observed multi-hour workflows.

For process scorecards, prefer the production scorecard reindex workflow over historical standalone Python reconstruction.

## Validation Checklist

- [ ] PG source row and intended analytics eligibility confirmed.
- [ ] Repair path covers the scorecard type and shard rule.
- [ ] Pre-repair CH state captured without secrets.
- [ ] Job/workflow ID recorded.
- [ ] Scorecard-level fields match after repair.
- [ ] Emitted criterion IDs/values match after accounting for chapter rows.
- [ ] No stale rows remain when cleanup was required.
- [ ] Monitor/accounting totals are unchanged or intentionally updated.
- [ ] Customer/time-range scope and residual risk documented.

## Historical Execution Sources

Detailed scripts, run logs, and exact historical results remain in:

- `backfill-scorecards/`
- `convi-6298-reindex-process-scorecards/`
- `convi-5565-scorecard-ch-pg-sync/tools/`

These are evidence and historical tooling, not automatically safe current runbooks. Revalidate code paths, configuration, access, and production safeguards before reuse.
