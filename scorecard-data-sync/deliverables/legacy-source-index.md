# Legacy Source Index

The October 25 follow-up consolidated 4 more inactive projects. Their original records now live in dated domain session evidence; [the synthesis](inactive-legacy-project-consolidation-2026-10-25.md) lists canonical work items, substantive dates and preserved sources.

This index records where migrated knowledge came from. The domain deliverables are canonical synthesis; dated session evidence retains raw investigations, scripts, run logs, and ticket history.

| Legacy source | Canonical contribution | Disposition |
|---|---|---|
| [ui-pg-ch-ordering](../work-items/ui-pg-ch-ordering.md) | Theory, taxonomy, monitor incidents, Pack Rat case, investigator skill | Migrated synthesis; legacy evidence retained |
| [CONVI-5565](../work-items/CONVI-5565.md) | Async ordering, PG lost update, production verification | Migrated synthesis; legacy ticket evidence retained |
| [CONVI-6298](../work-items/CONVI-6298.md) | Process-scorecard coverage gap and scorecard-centric reindex design | Migrated synthesis; implementation/run evidence retained |
| [CONVI-6209](../work-items/CONVI-6209.md) | Large backfill operations, cleanup semantics, capacity lessons | Migrated synthesis; historical scripts/results retained |
| `historic-scorecard-missing` | Async historic-write omission analysis | Migrated synthesis; embedded product/tool code still needs source-repo review |

## Reclassified Sources

| Source | Correct primary destination | Reason |
|---|---|---|
| [process-scorecard-create-autosave-race](../../scorecard-workflows/work-items/process-scorecard-create-autosave-race.md) | `scorecard-workflows` | PG replica read-after-create behavior; no CH projection failure required |
| [hilton-coaching-report-leaderboard-parity](../../analytics/work-items/hilton-coaching-report-leaderboard-parity.md) | Retained coaching analytics reference | Coaching session/efficiency and filtering behavior, not scorecard projection |
| CONVI-7378 (SCAN Consent to Call) | `analytics` | PG/CH aligned; mixed-revision QA score semantics, not projection failure |

## Current-State Notes

- Legacy READMEs contain migration banners and remain searchable.
- [ui-pg-ch-ordering](../work-items/ui-pg-ch-ordering.md) includes the latest CONVI-7186 session/log evidence that existed uncommitted before migration.
- Credential-shaped connection material in `historic-scorecard-missing/investigation.md` was redacted from the current tree. Git history and credential rotation require a separate authorized security response.
- Historical scripts are not promoted as canonical executables; validate them against current source code before use.
