# CONVI-7208: Director-only CSV export for criterion comments

**Date**: 2026-07-03  
**Status**: Accepted

## Decision

Implement per-criterion comment columns in the Group Calibration session CSV export as a **director-only** change. No new gRPC API in go-servers.

## Rationale

- `ListScorecards` already returns `scores.comment` for answer key and response scorecards.
- CSV is assembled client-side in `RecentCalibrationsThreeDotsMenu.tsx`.
- Smallest correct fix is to extend the existing export builder.

## Product rules

| Topic | Choice |
|-------|--------|
| Comment source | Criterion score comment only (`score.comment`); no collaboration comment merge |
| Column layout | Grade column, then `{displayName} comment` column — matches go-servers `action_export_scorecards.go` |
| Criterion values | Unchanged (numeric / N/A only) |
| Scorecard-level Comment | Keep existing column |
| Empty comments | Blank cell |
| Comment access roles | Not applied in export |

## Implementation

- **Repo**: director
- **File**: `packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/components/RecentCalibrationsThreeDotsMenu.tsx`
- **Helper**: `buildCriterionCsvColumns()` emits paired grade + comment fields and headers
