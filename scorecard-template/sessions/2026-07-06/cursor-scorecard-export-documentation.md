# Scorecard export paths documentation

**Date:** 2026-07-06
**Source repos:** director, go-servers
**Trigger:** Document scorecard exportation as part of scorecard-template workflow reference; cross-link CONVI-7208 numeric grade investigation.

## Inputs reviewed

- `/Users/xuanyu.wang/repos/knowledge/group-calibration/sessions/2026-07-06/cursor-numeric-grade-csv-investigation.md`
- `director/.../buildCriterionCsvColumns.ts`
- `director/.../RecentCalibrationsThreeDotsMenu.tsx`
- `director/.../SessionSnapshotChart.tsx` (`getScoreLabel`)
- `go-servers/apiserver/internal/coaching/action_export_scorecards.go` (`getScoreValue`)
- `go-servers/shared/scoring/scorecard_templates.go` (`GetCriterionLabelForValue`)

## Findings

1. Two primary CSV export paths: Group Calibration (FE client-side) and QM/Coaching Hub (BE `ExportScorecards`).
2. Known inconsistency: Group Cal exports raw `numericValue`; BE maps labeled/dropdown criteria to option labels.
3. Criterion ordering parity fixed in CONVI-7208 (template order); grade label parity still open.
4. Bug introduced 2025-05-05 PR #11643 (`70fa3054b2`), not by CONVI-7208.

## Artifacts created

- `deliverables/scorecard-export-paths.md` — canonical export path reference
- Cross-link to `group-calibration/deliverables/convi-7208-numeric-grade-csv-fix-plan.md`

## Next steps

- Implement numeric grade fix per fix plan (director PR #20388 or follow-up).
