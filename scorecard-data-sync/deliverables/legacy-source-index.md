# Legacy Source Index

This index records where migrated knowledge came from. The domain deliverables are canonical synthesis; legacy folders retain raw investigations, scripts, run logs, and ticket history.

| Legacy source | Canonical contribution | Disposition |
|---|---|---|
| `pg-ch-scorecard-sync-investigation` | Theory, taxonomy, monitor incidents, Pack Rat case, investigator skill | Migrated synthesis; legacy evidence retained |
| `convi-5565-scorecard-ch-pg-sync` | Async ordering, PG lost update, production verification | Migrated synthesis; legacy ticket evidence retained |
| `convi-6298-reindex-process-scorecards` | Process-scorecard coverage gap and scorecard-centric reindex design | Migrated synthesis; implementation/run evidence retained |
| `backfill-scorecards` | Large backfill operations, cleanup semantics, capacity lessons | Migrated synthesis; historical scripts/results retained |
| `historic-scorecard-missing` | Async historic-write omission analysis | Migrated synthesis; embedded product/tool code still needs source-repo review |

## Reclassified Sources

| Source | Correct primary destination | Reason |
|---|---|---|
| `convi-6841-process-scorecard-update-race` | `scorecard-workflows` | PG replica read-after-create behavior; no CH projection failure required |
| `hilton-coaching-discrepancy` | Retained coaching analytics reference | Coaching session/efficiency and filtering behavior, not scorecard projection |
| CONVI-7378 (SCAN Consent to Call) | `analytics` | PG/CH aligned; mixed-revision QA score semantics, not projection failure |

## Current-State Notes

- Legacy READMEs contain migration banners and remain searchable.
- `pg-ch-scorecard-sync-investigation` includes the latest CONVI-7186 session/log evidence that existed uncommitted before migration.
- Credential-shaped connection material in `historic-scorecard-missing/investigation.md` was redacted from the current tree. Git history and credential rotation require a separate authorized security response.
- Historical scripts are not promoted as canonical executables; validate them against current source code before use.
