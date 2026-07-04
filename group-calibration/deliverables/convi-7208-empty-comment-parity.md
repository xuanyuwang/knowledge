# CONVI-7208: Empty comment behavior across exports

**Context**: CONVI-7208 decision — per-criterion empty comments export as **blank cells**. This document compares behavior across export surfaces.

## Summary table

| Export | Per-criterion empty | Scorecard-level empty |
|--------|---------------------|------------------------|
| QM Report (`ExportScorecards`) | Blank | `"None"` when SQL null |
| Performance Insights criteria export | Blank (`getCSV` nil → `''`) | N/A |
| Group Calibration CSV (before CONVI-7208) | N/A (no comment columns) | Blank (`?? ''`) |
| Group Calibration CSV (after CONVI-7208) | Blank (`score.comment ?? ''`) | Blank (unchanged) |

## QM Report / Coaching Hub (`ExportScorecards`)

**File**: `go-servers/apiserver/internal/coaching/action_export_scorecards.go`

**Per-criterion comment**:

```go
comment := criterionScores[0].Comment.String
scoreValues[criterionIdentifierToHeaderIndex[criterion.GetIdentifier()]+1] = comment
```

When no comment exists, `sql.NullString.String` is `""` → **blank cell**.

**Scorecard-level comment**:

```go
exporter.AppendNullString(scorecard.Comment, "None")
```

When SQL null → **`"None"`** placeholder.

Test expectation (`action_export_scorecards_test.go`): empty criterion comment columns are `""`; scorecard-level null comment is `"None"`.

## Performance Insights (director)

**File**: `director/packages/director-app/src/hooks/insights/useExportPerformanceInsightCriteria.tsx`

Sets `row.unnamedProperties[`${id}_comment`] = info?.comment`.

**File**: `director/packages/director-app/src/utils/csvUtils.ts`

```typescript
value = isNil(value) ? '' : String(value);
```

Missing comment → **blank cell**.

## Group Calibration session CSV

**Before**: scorecard-level `comment: answerKeyScorecard?.comment ?? ''` → blank when missing. No per-criterion comment columns.

**After (CONVI-7208)**: per-criterion `score.comment ?? ''` → blank when missing. Scorecard-level behavior unchanged.

## Parity conclusion

- **Per-criterion blank** in Group Calibration CSV aligns with QM Report and Performance Insights.
- **Scorecard-level blank** in Group Calibration differs from QM Report's `"None"` — this is pre-existing Group Calibration behavior and is intentionally unchanged for CONVI-7208.
