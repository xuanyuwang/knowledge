# CONVI-7533 — Pre-deletion scorecard backup

## Objective

Preserve the four identified `director.scorecards` rows and their 190 `director.scores` rows in a directly readable, integrity-verifiable format before deletion.

## Status

Completed on 2026-08-27. The existing Linear backup document now contains RFC 4180-compatible CSV in five indexed comments: one comment for all four scorecards and one scores comment per scorecard.

## Evidence

- Linear document: https://linear.app/cresta/document/convi-7533-pre-deletion-postgresql-backup-83fe3a3e16bf
- Captured source SHA-256: `d8f11bdbd997715ff0eef120bbf3d658090ea359a62219c9ff3adf059a46affd`
- Verified rows: 4 scorecards and 190 scores, split `21 / 56 / 57 / 56`
- Verified headers: all 33 scorecard columns and all 16 score columns from the source JSON objects
- Verification reread all five Linear comments and confirmed each CSV payload byte-for-byte against its indexed SHA-256.

## Scope

No production database was queried or mutated during CSV generation or publication.
