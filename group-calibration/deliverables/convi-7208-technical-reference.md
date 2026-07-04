# CONVI-7208: Technical reference

**Ticket**: [CONVI-7208](https://linear.app/cresta/issue/CONVI-7208/group-calibration-add-column-for-scorecard-criterion-level-comments)  
**Primary repo**: director (`/Users/xuanyu.wang/repos/director`)  
**Related repo**: go-servers (data layer only; no code changes for this ticket)

## Architecture

```text
Director (RecentCalibrationsThreeDotsMenu.tsx)
  ├─ useListDirectorTasks          → session metadata
  ├─ useQMReportGroupCalibrationStats → consistency / completion
  └─ useListAllScorecards (FULL)   → answer key + response scorecards
        └─ go-servers ListScorecards
              └─ convertScoreToPB → director.scores.comment (Postgres)
                    └─ director-api transformScoreToModel → score.comment
                          └─ buildCriterionCsvColumns → getCSV → saveAs
```

The CSV file is assembled entirely in the browser. There is no go-servers export endpoint for group calibration sessions.

**Trigger**: QA Group Calibrations Report → Recent Calibrations table → three-dots menu → "Download session as CSV"

## Data path (already supports comments)

| Layer | Location | Notes |
|-------|----------|-------|
| Storage | `director.scores.comment` | Nullable VARCHAR on scores table |
| API | `go-servers/apiserver/internal/coaching/transformers.go` | `convertScoreToPB` maps `score.Comment` to proto |
| Frontend | `director-api/.../coaching/transformers.ts` | `transformScoreToModel` maps `score.comment` |
| Export | `buildCriterionCsvColumns` | Reads `score.comment` when building CSV rows |

No new proto field or DB column is required.

## Reference implementation (QM Report)

`go-servers/apiserver/internal/coaching/action_export_scorecards.go`:

- `buildCriteriaHeaders()` — appends `{displayName}` and `{displayName} comment` per criterion
- `convertScorecardsToCSVBytes()` — grade at index `i`, comment at index `i+1`

Group Calibration CSV mirrors this naming; export remains client-side.

## Why insights-server changes are not needed

`go-servers/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go`:

- `findScores()` selects grades only (`numeric_value`, `not_applicable`), not `scores.comment`
- `EvaluatedCriterion` proto has no comment fields

This API powers stats/charts on the report page, **not** the session CSV download.

## Implementation (director)

| File | Change |
|------|--------|
| `packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/buildCriterionCsvColumns.ts` | New helper: paired grade + comment fields and headers |
| `packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/buildCriterionCsvColumns.test.ts` | Unit tests |
| `packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/components/RecentCalibrationsThreeDotsMenu.tsx` | Import helper; use in `downloadCSV` |

### `buildCriterionCsvColumns` behavior

For each score with `displayName`:

1. Grade field: `N/A` if `notApplicable`, else `numericValue ?? 0`
2. Comment field: `score.comment ?? ''`
3. Headers: `{displayName}` and `{displayName} comment`

Answer key row defines column order via `criteriaColumns`; reviewer rows populate the same keys.

## Out of scope

- `cresta-proto` / insights-server proto changes
- New `ExportGroupCalibrationSession` RPC
- Locale changes (comment headers use dynamic criterion display names)

## Related artifacts

- [Session investigation](../sessions/2026-07-03/cursor-convi-7208-investigation.md)
- [Decision record](../decisions/2026-07-03-convi-7208-director-only-export.md)
- [Comment access roles](./convi-7208-comment-access-roles.md)
- [Empty comment parity](./convi-7208-empty-comment-parity.md)
- `/Users/xuanyu.wang/repos/knowledge/export-appeal-comments/research.md`
