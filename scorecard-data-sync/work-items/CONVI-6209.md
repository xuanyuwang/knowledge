# CONVI-6209: Historical scorecard backfill and appeal cleanup

**Status:** historical runs completed; no new production validation
**Primary domain:** scorecard-data-sync
**Primary subdomain:** none
**Official ticket:** [CONVI-6209](https://linear.app/cresta/issue/CONVI-6209)
**Last updated:** 2026-10-25

## Objective and source context

Recover missing scorecard projections and remove appeal rows excluded by the newer analytics writer. Source repo: `/Users/xuanyu.wang/repos/go-servers`; the archive does not establish a currently active branch/worktree. Related implementation: [PR #25653](https://github.com/cresta/go-servers/pull/25653), with process recovery continued in [CONVI-6298](CONVI-6298.md).

## Historical outcomes

- January 2026 backfill dispatched 260 jobs across eight clusters: 256 completed initially; Marriott and United East succeeded on retry, while CVS and Oportun required 31 sequential daily jobs each. Concurrent daily jobs were canceled after causing database stress.
- February appeal cleanup completed delete plus backfill for 95 customers. Recorded removal totals were approximately 70.5 million scorecard rows and 650.5 million score rows. Oportun needed daily deletion chunks and 51 sequential backfill windows; four other large customers needed 5–10 day windows and multi-hour execution allowances.
- March 5–6 manual process recovery inserted 2,120 scorecards / 30,464 scores for Spirit and 43,953 / 501,456 for Oportun. These runs preceded the production scorecard-centric workflow. Oportun's historic table lacked a useful scorecard-ID access path, so the run reconstructed from director scores and template revisions.
- The Python comparison covered 1,784 scores with zero unexplained differences; 183 N/A or excluded-criterion differences were explained. That is a specific historical validation, not proof of universal scoring equivalence.

## Lessons and limits

Conversation reindex cannot discover process scorecards with empty conversation identity. Use the production scorecard-centric recovery path described in [CONVI-6298](CONVI-6298.md), rather than treating the old Python scripts as a current runbook.

Reindex inserts do not remove rows newly excluded by eligibility filters. Cleanup must target local tables on all required shards and replicas, then verify the intended rows and fields. Large mutations can obstruct unrelated work. Mutation completion alone does not prove convergence when asynchronous writers remain in flight.

Size conversation recovery by PostgreSQL conversation-read volume. The archive's successful weekend parallel runs do not establish that parallel execution is generally safe. Its large `FINAL`/ID-list checks hit query-size and memory limits; choose validation predicates and batching from current data shape.

## Next actions

No historical run is restarted by this consolidation. For a new repair, establish current source correctness, eligibility, worker capacity and deletion semantics; use [the canonical playbook](../deliverables/repair-and-backfill-playbook.md), record exact residual IDs and compare derived fields after completion.

## Evidence and timeline

- 2026-02-07–10 — January recovery and sequential fallback.
- 2026-02-21–23 — All-customer appeal cleanup and reprojection.
- 2026-03-05–06 — Manual process-scorecard recovery.
- 2026-03-10 — Last meaningful legacy update: PR #25916 review fixes and rebase, detailed in the archived daily log.
- 2026-10-25 — Consolidated the historical collection; preserved [original README](../sessions/2026-10-25/backfill-scorecards-legacy-evidence/README.md) and [March 10 record](../sessions/2026-10-25/backfill-scorecards-legacy-evidence/log/2026-03-10.md). No scripts or production operations were run.
