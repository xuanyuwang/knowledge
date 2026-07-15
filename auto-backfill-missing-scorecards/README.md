# Auto Backfill Missing Scorecards

> Migrated navigation: [Scorecard Workflows / Process Scorecards and Generation](../scorecard-workflows/subdomains/process-scorecards-and-generation/README.md). PG/CH projection repair remains under Scorecard Data Sync.

## Status

Active investigation. This project tracks scorecard backfill and auto-heal work in `go-servers`.

## Current Focus

As of 2026-06-29, the current question is whether a one-time cron/script can backfill the last 90 days of a scorecard template that depends on Opera rule outcomes, and whether the required feature flag can be scoped to the backfill execution path.

Current recommendation: prefer a one-off internal `JOB_TYPE_DOWNSTREAM_BACKFILL_JOB` over a new recurring cron. The downstream job can run Opera annotation backfill first and then scorecard backfill. A scorecard-only backfill is lower risk but only works when the relevant Opera/LLM outcome annotations already exist.

Safety update: do not rely on `enableLLMEvaluationMoment` as a customer-visibility guard. It appears to gate Director Opera authoring/editing UI, while Closed Conversations already queries and renders `LLM_EVALUATION` annotation data. If historical Opera annotations are persisted, they should be treated as customer-visible unless protected by Platform Visibility / `rolesWithVisibility` or by adding an explicit read-side/backend guard.

## Source Repo

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`
- Current investigation context: main checkout, no product-code edits yet

## Key Artifacts

- `investigation.md`
- `auto-heal-design.md`
- `implementation-and-validation-summary.md`
- `sessions/2026-06-29/codex-opera-outcome-backfill.md`
- `log/2026-06-29.md`
- `log/2026-07-03.md`

## 2026-06-29 Findings

- Performance Config Time Machine backfills scorecards only for conversations where Opera has already annotated behavior/outcome data.
- Opera/policy backfill can generate the missing annotations and optionally chain scorecard backfill.
- `enableLLMEvaluationMoment` appears to be a Director UI flag; backend execution uses saved LLM evaluation policy/moment config plus orchestrator worker support.
- A backfill-only feature override is medium-high complexity if needed, because it must be represented in job payload/config and threaded through policy backfill into orchestrator inputs.
- Closed Conversations loads `LLM_EVALUATION` annotations independently of the `enableLLMEvaluationMoment` flag. Platform Visibility can hide rendered behavior annotations in Director for unauthorized roles, but backend `ListMomentAnnotations` does not appear to enforce policy-role visibility itself.