# CLO filter performance-test plan session

**Date:** 2026-07-27
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `85d639ecaf75626fd060852a5ee4196df492361c` (local checkout was 462 commits behind `origin/main`)
**Related schema repo:** `/Users/xuanyu.wang/repos/clickhouse-schema` at `ed4d69160f72c32ad7dcebfddeb0dd32a81d97e3` (local checkout was 8 commits behind `origin/main`)

## Request

Create an executable, evidence-driven plan to measure CLO filtering cost, metadata regression under request-wide raw routing, mixed requests, scale by time/selectivity/volume/group count, and dominant query components. Do not implement code changes or run production queries.

## Sources reviewed

- Existing project README, data flow, moment-annotation background, and 2026-07-23 performance review.
- Current query routing/building in `retrieve_qa_score_stats_clickhouse.go` and `common_clickhouse.go`.
- Current generated SQL fixtures and query-builder tests.
- Canonical `moment_annotation` and metadata MV DDL.
- Director request construction and backend cache/fan-out paths.
- ClickHouse shared query-ID generation and repository system-table guidance.

## Validated findings

- Routing is request-wide: one outcome moment group makes every parsed metadata group use the raw table.
- The router detects an outcome moment even without outcome value attributes; the parser does not emit such a group.
- Each included group generates two annotation scans, a latest-timestamp self-join, and an outer join to score results.
- MV routing uses `update_time`; raw routing uses `create_time`, so simulated per-group comparisons require result-equivalence checks.
- Exact boolean false and numeric zero use `JSONHas` guards. Numeric bins repeat `JSONExtractFloat` for both bounds.
- API cache hits can bypass ClickHouse; template QA-score configuration can fan one RPC into several bounded-parallel ClickHouse queries.
- Local source checkouts are behind their remotes, so execution must record deployed revisions and revalidate line references/query shapes before benchmarking.

## Output

- Created `deliverables/clo-filter-performance-test-plan.md` with:
  - precise downgrade definitions and ratios;
  - A–J matrix and component-decomposition experiments;
  - payload/SQL capture and UI correlation instructions;
  - staging and controlled-production safety gates;
  - query-log SQL, recording templates, confounders, criteria, and open questions.

## Actions not taken

- No ClickHouse queries were executed.
- No browser/UI measurements were performed.
- No product or schema code was changed.
- No production credentials were accessed.
