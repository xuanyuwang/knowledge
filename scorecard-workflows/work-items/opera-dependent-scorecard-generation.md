# Opera-dependent generation and historical backfill

**Primary domain:** scorecard-workflows
**Primary subdomain:** process-scorecards-and-generation
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-06-29
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

Performance Config Time Machine can recompute scorecards only after Opera has supplied the required behavior/outcome annotations. The June 29 investigation recommends a one-off `JOB_TYPE_DOWNSTREAM_BACKFILL_JOB` that generates Opera annotations before scorecards; a scorecard-only job works only where those annotations already exist. A new recurring cron was not selected.

`enableLLMEvaluationMoment` controls Director authoring/editing and does not establish that stored `LLM_EVALUATION` annotations are hidden in Closed Conversations. Platform Visibility may hide rendered annotations by role, but the examined `ListMomentAnnotations` backend did not appear to enforce that role policy. Any backfill-only flag override would need to travel through job payload, policy backfill, and orchestrator input; that was a design estimate, not an implementation.

This source also includes PG/ClickHouse monitor work: missing/stale metadata detection, disabled-by-default allowlisted auto-heal, thresholds, payload chunking, and a staging experiment with 748 accepted candidates, 1,122 skipped candidates, then a manual override. The residual 71 missing candidates were not resolved by reindex. Those are projection-repair findings owned by Scorecard Data Sync, not proof that 71 scorecards should be generated.

## Source context and limits

go-servers main checkout; implementation-and-validation-summary.md records monitor image main-20260622_203131z-aa39df67.

Inactivity evidence: 2026-07-03 is an explicit wrap-up with no new investigation. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

When a concrete customer/template request returns, confirm annotation availability and customer visibility, stage the downstream job, then verify generated scores. Investigate the residual monitor candidates under Scorecard Data Sync.

## Preserved evidence

The original folder is retained intact at [auto-backfill-missing-scorecards legacy evidence](../sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [sessions/2026-06-29/codex-opera-outcome-backfill.md](../sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/sessions/2026-06-29/codex-opera-outcome-backfill.md)
- [implementation-and-validation-summary.md](../sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/implementation-and-validation-summary.md)
- [log/2026-07-03.md](../sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/log/2026-07-03.md)
