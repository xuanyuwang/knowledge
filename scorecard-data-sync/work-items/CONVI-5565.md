# CONVI-5565: Historical PG/ClickHouse ordering repairs

**Status:** historical fixes merged; broader ordering design continues separately
**Primary domain:** scorecard-data-sync
**Primary subdomain:** none
**Official ticket:** [CONVI-5565](https://linear.app/cresta/issue/CONVI-5565)
**Last updated:** 2026-10-25

## Objective and source context

Prevent asynchronous scorecard writes from leaving stale analytical state and prevent update requests from clearing submission fields. Source repo: `/Users/xuanyu.wang/repos/go-servers`; no active implementation worktree/branch is established by this archive review. Historical changes include [atomic writes #24103](https://github.com/cresta/go-servers/pull/24103), GORM `Omit` for submission fields (CONVI-6076), and [legacy/feature-flag removal #26256](https://github.com/cresta/go-servers/pull/26256).

## What worked and what remained

Historic writes moved inside the PG transaction, and asynchronous CH work re-read committed PG state. Omitting `submitted_at` and `submitter_user_id` from update saves addressed a separate PG lost-update mechanism. These fixes did not provide durable ordered projection: an async update can read before submit commits, then write its stale snapshot later with a higher CH write-time version.

Spirit's March 11 comparison found zero score and submitter mismatches among 2,996 scorecards present in both stores, but 31 had zero CH submit time. A March 12 follow-up found 26 stale-submission cases with a slightly different range, including four stale scores. Those are different observations; the earlier zero-score-mismatch result must not hide the later four cases. `submission_source=1` means Closed Conversations, correcting an initial auto-scoring attribution. 24/26 were from that human UI path.

The observed timestamps support a stale async snapshot. A single remaining CH version cannot distinguish a superseded submit write from a failed/missing submit write. The archive's language that every case was conclusively the same race is stronger than that surviving evidence.

Historical load tests reported roughly 10–20% failure at rapid concurrent calls and near-zero with at least 100 ms spacing. These do not prove convergence for all workloads. The archive's statements that PG is always correct and the residual is acceptable are historical conclusions, not present guarantees or current product policy; subsequent aggregate-consistency work is tracked in [CONVI-7784](CONVI-7784.md).

## September correction to the first failed proposal

The original investigation says stale closure data explains failure of using PG `updated_at` as the CH version. That alone cannot explain an older version defeating a newer version for the same replacement key. The September 19 source and isolated-engine investigation instead demonstrates a cleanup contract conflict: reindex cleanup treats `update_time` as indexing time, so freshly rebuilt rows with old PG timestamps can qualify for deletion. Exact historical production attribution remains unverified.

Continue design in the existing [UI/PG/CH ordering work item](ui-pg-ch-ordering.md) and [timestamp-contract explainer](../deliverables/why-pg-updated-at-breaks-ch-cleanup.md). Do not create another design track or reintroduce the timestamp substitution independently of cleanup and migration semantics.

## Evidence and timeline

- 2026-03-11–12 — Spirit verification, corrected submission-source interpretation and legacy flag removal.
- 2026-09-19 — Last meaningful legacy content: explicitly dated correction in the README. Git commit `39db3b4` saved it October 4; this is not evidence of October-authored investigation.
- 2026-10-25 — Preserved [original README](../sessions/2026-10-25/convi-5565-scorecard-ch-pg-sync-legacy-evidence/README.md), [original investigation](../sessions/2026-10-25/convi-5565-scorecard-ch-pg-sync-legacy-evidence/investigation.md) and [March 12 evidence](../sessions/2026-10-25/convi-5565-scorecard-ch-pg-sync-legacy-evidence/log/2026-03-12.md). No historical verification tools, credentials or production queries were used.
