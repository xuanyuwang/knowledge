# US East ClickHouse Distributed backlog

## Case metadata

- **Status:** Resolved
- **Incident window:** 2026-07-29 through 2026-08-02
- **Knowledge owner:** `scorecard-data-sync`
- **Primary incident:** [INSI-4251](https://linear.app/cresta/issue/INSI-4251/missing-data-in-performance-insights-for-731)
- **Environment:** `us-east-1-prod`
- **Reported customers:** Guitar Center and Home Care Delivered
- **Regional impact:** 61 customer databases; scorecard, score, conversation, and other Distributed-table projections

## Summary

The scorecard missing-rate monitor was the first strong symptom of a region-wide ClickHouse Distributed delivery backlog. Writes were accepted into local spool files, but an unbatched 16-thread sender pool shared by 3,328 Distributed tables could not drain them as quickly as new files arrived. The queue peaked at approximately 6.36 million files / 107.6 GB.

The issue later surfaced as missing Performance Insights data, successful backfills whose rows remained invisible, and independent `score_d` / `scorecard_d` version skew. Enabling batched delivery and split-on-failure, increasing the sender pool from 16 to 48, and rolling the Conversations ClickHouse nodes restored delivery.

## Post-recovery verification

On August 3, the production sync monitor checked all 91 profiles for July 28 through August 3:

- regional missing rate: 56,676 / 16,858,725 (0.3362%);
- Guitar Center: 0 / 246,129 missing, 4 stale;
- Home Care Delivered: 0 / 57,360 missing, 10 stale;
- 72 profiles had zero missing rows.

The reported customer gaps were recovered. Before the repair run, CNG remained a separate outlier at 54,819 / 514,722 missing (10.65%), accounting for 96.72% of regional misses. Its score and scorecard Distributed queues were empty, so that residual gap was not a currently undrained backlog.

An all-profile auto-heal then created 47 reindex jobs for 57,773 missing/stale candidates. All workflows completed. Post-backfill verification found:

- CNG: zero missing and zero stale;
- Guitar Center: zero missing and zero stale;
- HCD: zero missing and zero stale;
- region: 26 / 16,908,176 missing (0.0002%) and 3 stale.

The only residual is 26 submitted Alaska Air conversation scorecards. Their automatic backfill completed without recreating them, so they require targeted diagnosis rather than another identical retry.

## Case artifacts

### Canonical synthesis

- [Full incident retrospective](../../deliverables/2026-08-us-east-distributed-backlog-incident.md)
- [Interactive incident timeline](../../../../../.cursor/projects/Users-xuanyu-wang-repos/canvases/us-east-clickhouse-backlog-incident.canvas.tsx)

The Canvas source remains in Cursor's managed `canvases` directory so it can be compiled and opened beside chat. This case index is its canonical knowledge-project entry.

### Investigation records

- [Initial July 29 missing-rate investigation](../../sessions/2026-07-30/codex-us-east-1-missing-spike.md)
- [Guitar Center and HCD incident investigation](../../sessions/2026-08-01/codex-performance-insights-missing-data.md)
- [Post-recovery all-customer verification](../../sessions/2026-08-03/codex-post-recovery-missing-rate.md)
- [INSI-4251 work item](../../work-items/INSI-4251.md)

### Daily movement

- [2026-08-01 incident investigation](../../log/2026-08-01.md)
- [2026-08-02 resolution](../../log/2026-08-02.md)
- [2026-08-03 post-recovery verification](../../log/2026-08-03.md)

## Production changes

- [PR 312915: batch Distributed sends](https://github.com/cresta/flux-deployments/pull/312915)
- [PR 312923: increase sender pool from 16 to 48](https://github.com/cresta/flux-deployments/pull/312923)

## External evidence

- [Incident Slack channel](https://cresta.enterprise.slack.com/archives/C0BMCK2301Y)
- [INSI-4251](https://linear.app/cresta/issue/INSI-4251/missing-data-in-performance-insights-for-731)

## Classification rationale

This case belongs to `scorecard-data-sync` because scorecard projection monitoring detected the incident and the main investigation established PG-to-CH scorecard correctness, backfill semantics, and repair behavior. The infrastructure failure affected broader ClickHouse ingestion, so the retrospective explicitly records that the root cause was not scorecard-specific.
