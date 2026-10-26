# CONVI-6298: Scorecard-centric process reindex

**Status:** implementation and historical rollout recorded; backfill exceptions unresolved in available evidence
**Primary domain:** scorecard-data-sync
**Primary subdomain:** none
**Official ticket:** [CONVI-6298](https://linear.app/cresta/issue/CONVI-6298)
**Last updated:** 2026-10-25

## Objective and source context

Recover process scorecards independently of conversation discovery. Primary repo: `/Users/xuanyu.wang/repos/go-servers`; related repo: `/Users/xuanyu.wang/repos/cresta-proto`. No active implementation branch/worktree is established by this archival review. Historical merged changes: [proto #7919](https://github.com/cresta/cresta-proto/pull/7919), [backend #25916](https://github.com/cresta/go-servers/pull/25916), and [cleanup fixes #26417](https://github.com/cresta/go-servers/pull/26417).

## Implemented design and validation

The chosen `JOB_TYPE_REINDEX_SCORECARDS` workflow reconstructs projections from director scores and template revisions through `GenerateHistoricScorecardScores`, avoiding dependence on historic-table completeness or indexing. Empty type selection defaults to process scorecards; `REINDEX_MODE` controls cron dispatch. A dedicated process-only job and extension of conversation reindex were rejected in favor of scorecard-centric selection.

Historical batch size was 50. Heartbeats record progress/liveness; retries restart from the beginning because OFFSET pagination cannot reliably resume against changing data. Replacement deduplication makes repeat projection useful, but does not resolve every ordering/deletion race.

Staging wrote 196 scorecards and 721 scores. Initial cleanup incorrectly targeted distributed tables and cron forced cleanup on. PR #26417 fixed local-table cluster cleanup and made cleanup configurable; both paths were verified March 20. The archive records production sandbox verification March 24.

## Backfill outcome, correcting stale README checkboxes

The March 28 results document supersedes the legacy README's unchecked backfill phases:

| Phase | Dispatched | Completed | Exception |
|---|---:|---:|---|
| 2026-01-01 to 2026-03-29 | 276 | 275 | Brinks Care duplicate criterion; Schwab dispatched zero workflows on old image |
| 2020-01-01 to 2026-01-01 | 276 | 275 | Different Brinks Care scorecard, same invalid-input class; Schwab still skipped |

Ten first-phase East workflows initially hit heartbeat timeouts under 90 concurrent workflows; serial redispatch completed all ten. The second phase's East workflows completed without those retries. Worker saturation is the archive's diagnosis, not an independently reproduced finding from this consolidation.

The Brinks Care generator failures are source-data validation errors. A successful cron invocation on Schwab was insufficient: its old image ignored process mode and dispatched conversation jobs instead.

## Next actions

If historical exceptions matter today, first verify current state: establish whether Schwab later received a compatible image and both ranges ran; determine whether the two Brinks Care duplicate-criterion scorecards were repaired or explicitly accepted. Do not infer universal completion from 275 successful workflows or loosen validation without a separate decision.

## Evidence and timeline

- 2026-02-24–27 — Selected scorecard-centric workflow and reconstruction from director data.
- 2026-03-19–24 — Merged implementation, fixed cleanup defects, verified staging and production sandbox.
- 2026-03-28 — Last meaningful legacy work: both backfill phases and recorded exceptions.
- 2026-10-25 — Preserved [original README](../sessions/2026-10-25/convi-6298-reindex-process-scorecards-legacy-evidence/README.md), [final results](../sessions/2026-10-25/convi-6298-reindex-process-scorecards-legacy-evidence/backfill-2026-results.md) and [daily record](../sessions/2026-10-25/convi-6298-reindex-process-scorecards-legacy-evidence/log/2026-03-28.md). No rerun or live rollout verification.
