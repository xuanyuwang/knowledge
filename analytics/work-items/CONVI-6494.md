# CONVI-6494: Raises Answered percent stays at zero

**Record state:** Partial implementation as last recorded 2026-05-07; current ticket/branch status not rechecked.
**Primary domain:** analytics
**Primary subdomain:** leaderboard
**Official ticket:** [CONVI-6494](https://linear.app/cresta/issue/CONVI-6494)
**Consolidated:** 2026-10-25

## Objective and Impact

Make the Agent Leaderboard's Raises Answered metric match raise/whisper event evidence. Role: diagnosed and compared the proposed query against Brinks data; later documented incomplete implementation.

## Source Context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`.
- Historical worktree: `/Users/xuanyu.wang/repos/go-servers-convi-6494`; branch: `convi-6494-fix-raises-answered-zero` (registered in `workspace/repos.yaml`, not revalidated here).
- Query: `liveAssistStatsClickHouseQuery`; fixture: `insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveLiveAssistStats_ExtTable_WithUsecaseFilter_request.sql`.
- [Original investigation](../sessions/2026-10-25/convi-6494-legacy-evidence/README.md), [March diagnosis](../sessions/2026-10-25/convi-6494-legacy-evidence/log/2026-03-24.md), [May implementation audit](../sessions/2026-10-25/convi-6494-legacy-evidence/sessions/2026-05-07/codex-day-end-wrapup.md).

## Findings and Decisions

The recorded combined CTE grouped by `manager_user_id`. Raise events (action type 7) had an empty manager ID, while whisper events (type 12) had a populated one, so the aggregation separated the events and could never see both flags in the same group. A metric that correlates those events must aggregate at a grain on which they can actually meet.

The proposed query splits the agent and manager aggregations: the agent CTE omits manager identity, and the manager CTE omits agent identity. The March Brinks comparison records answered raises changing from zero to 2,193 out of 2,442 raises. That comparison supports the proposed query shape for the saved case; it does not establish general deployment or complete regression coverage.

## Validation and Next Actions

The May 7 audit explicitly found only a dirty SQL fixture, with no corresponding production-query change in that wrap-up. Keep the work classified as partial as of that date. No tests, source edits, commit, or publication for the product fix were performed during consolidation.

If reopened, inspect today's branch and implementation first, then align query, fixture, and agent/manager regression cases. Do not treat the historical worktree path as proof the old dirty state still exists.

Durable reference: [Leaderboard](../subdomains/leaderboard/README.md).

## Timeline

- 2026-03-24 — Root cause and Brinks query comparison recorded.
- 2026-05-07 — Saved worktree audit found fixture-only partial implementation.
- 2026-07-15 — Legacy README gained domain navigation.
- 2026-10-25 — Evidence consolidated; see [migration session](../sessions/2026-10-25/codex-five-legacy-project-consolidation.md).
