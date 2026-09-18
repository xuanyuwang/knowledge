# RCG Consumer Outreach Casino DTQ score mismatch

## Objective

Determine why conversation `01a08bec-b88b-7768-809b-e7b2757f710c` displays 50% for the active `Consumer Outreach - Casino DTQ` template when the selected criteria reportedly imply 80%, compare PostgreSQL and ClickHouse, and use the narrowest supported repair only if PostgreSQL is correct and ClickHouse is stale.

## Status

Investigation complete on 2026-09-17. PostgreSQL is internally inconsistent: persisted criterion/chapter rows calculate to 80%, while the parent scorecard stores 50%. ClickHouse was intentionally not queried and no backfill was triggered. PR #32447 is a strong fix for the likely submit/autosave interleaving in this incident, but it does not close concurrent or out-of-order `UpdateScorecard` writes.

## Evidence

- Slack incident: `https://crestalabs.slack.com/archives/C04NB5AMV0F/p1789659565422529`
- Conversation: `customers/rcg/profiles/us-east-1/conversations/01a08bec-b88b-7768-809b-e7b2757f710c`
- Template name: `Consumer Outreach - Casino DTQ`
- Reported symptom: stored/displayed total 50%; expected total 80%; only `Policy and Procedure` was answered No.
- Glean confirms the root report and replies through 19:34 UTC. Its current index does not include the later race hypothesis.
- Current code permits a partial late save to compute a parent total from only the incoming rows while retaining prior criterion rows absent from that request, a concrete mechanism for a PostgreSQL parent/child mismatch.
- Exact active template: `019d521b-68c7-727e-bead-301e50d92c7c@c1af7a35` for use case `rci-casino-fit-Groups`.
- Exact scorecard: `01a09154-585f-71cf-8809-c95bf13e165e`.
- Manual calculation: four 100% criteria and one 0% criterion, each weight 20, equals 80%; stored chapter rows also read `100/0/100/100/100`, while `scorecards.score = 50`.
- The scorecard's `submitted_at` and `updated_at` are only 516 microseconds apart, supporting (but not proving) the PR's submit-overwrites-newer-autosave mechanism.
- PR #32447 makes `SubmitScorecard` preserve stored scoring fields and reload after commit. Its tests cover submit versus a newer score update, not two concurrent `UpdateScorecard` requests.
- Director's debounced autosave does not queue or version mutations, and the backend does not reject stale updates. Requests from overlapping autosaves, tabs, users, or other clients can still commit in a different order from the edits that produced them.

## Next actions

1. Repair the PostgreSQL scorecard through a supported source-of-truth workflow; do not use PG-to-CH reindex as the first repair.
2. Treat PR #32447 as the submit-race fix, then design a backend fix for `UpdateScorecard` ordering and parent/child atomic consistency.
3. Add deterministic concurrent-`UpdateScorecard` regression coverage for stale full and partial snapshots, branch-inactive score preservation, retries, and multiple concurrent editors.
4. After PostgreSQL is corrected, reindex the exact scorecard only if ClickHouse does not converge from the supported repair workflow.

## Repair mechanism assessment

- No Temporal workflow fixes the PostgreSQL parent/child mismatch; scorecard reindex is ClickHouse-only.
- The suspended `cron-manual-scorecard-scores-backfill` is designed to recompute PostgreSQL through `GetScorecard` -> `UpdateScorecard` and then trigger normal ClickHouse projection.
- RCG's checked-in cron config currently targets an obsolete August 2025 window, and the common deployment keeps the job suspended with the inner task disabled. It requires a narrowly scoped one-shot configuration/run; it is not presently automatic.
- Prefer the controlled cron or an equivalent single-resource RPC over direct SQL.

## Timeline

- 2026-09-17: Investigation opened from the `#convo-intelligence` Slack report.
- 2026-09-17: Confirmed the active template/scorecard and an 80%-versus-50% inconsistency in PostgreSQL; stopped before ClickHouse/backfill.
- 2026-09-17: Reviewed PR #32447; it likely prevents this incident's leading submit/autosave race but leaves the general update/update stale-write hazard open.
