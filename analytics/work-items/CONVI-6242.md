# CONVI-6242: Stale conversation labels and the Alaska backfill

**Record state:** Historical investigation and completed customer backfill; current ticket/rollout status not rechecked.
**Primary domain:** analytics
**Primary subdomain:** active-days
**Official ticket:** [CONVI-6242](https://linear.app/cresta/issue/CONVI-6242)
**Last substantive source update:** 2026-02-19
**Consolidated:** 2026-10-25

## Objective and Impact

Correct stale agent/use-case attribution in `conversation_with_labels` so Alaska Air's Active Days reflects the assigned agent. Role: diagnosed the labeler and recorded the customer backfill execution.

## Source Context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`; related repo: `clickhouse-schema`.
- Historical labeler: `cron/sync-users/internal/conversation-agent-assistance/task.go`.
- Referenced changes: [go-servers #25706](https://github.com/cresta/go-servers/pull/25706), [clickhouse-schema #172](https://github.com/cresta/clickhouse-schema/pull/172).
- [Original overview](../sessions/2026-10-25/convi-6242-legacy-evidence/README.md), [analysis and execution results](../sessions/2026-10-25/convi-6242-legacy-evidence/data-analysis.md), [historical backfill plan](../sessions/2026-10-25/convi-6242-legacy-evidence/backfill-plan.md), [original daily log](../sessions/2026-10-25/convi-6242-legacy-evidence/log/2026-02-18.md).

## Findings and Decisions

The historical labeler processed open conversations by creation time and advanced a forward-only watermark. Later reassignment or use-case changes did not revisit the saved labels. Its recorded ReplacingMergeTree key included mutable agent/end-hour values: rewriting an updated conversation could create a second key, and `FINAL` could not reconcile the different keys.

The backfill plan proposed deleting the affected old rows, waiting for mutations, and regenerating labels. This requirement belongs to the recorded schema and repair, not every future label-table design. A proposed switch to ended-time labeling reduces one source of staleness but does not prove labels can never change after closure.

## Recorded Outcome and Limits

The execution section records a completed Alaska backfill on February 19: 49 daily jobs, 1,539,530 resulting rows/distinct conversations, zero duplicates, and about 16.5 minutes overall. This supersedes the README's stale “In Progress” wording for that customer operation. It does not prove the ticket is currently Done, either referenced PR is deployed now, or other customers were repaired.

The saved comparison retains a 29,112-row difference from the separately filtered PostgreSQL population. Its suggested explanation was not independently proved during this migration; zero duplicates does not establish exact population equality.

## Validation and Next Actions

- The four backfill/verification scripts are preserved as historical evidence and were not executed. Some hardcode the `conversations` database, while the analysis identifies the customer database as `alaska_air_us_east_1`; they are not current runbooks.
- Before another repair, verify the current customer database, table key, time basis, filter population, writer behavior, and affected range. Reconcile population differences separately from duplicate checks.
- Durable lessons: [Active Days](../subdomains/active-days/README.md).

## Timeline

- 2026-02-18 — Diagnosis, repair plan, scripts, and pre-backfill measurements recorded.
- 2026-02-19 — Saved execution results report the Alaska repair completed with zero duplicates.
- 2026-07-15 — Legacy README gained domain navigation.
- 2026-10-25 — Evidence consolidated; see [migration session](../sessions/2026-10-25/codex-five-legacy-project-consolidation.md).
