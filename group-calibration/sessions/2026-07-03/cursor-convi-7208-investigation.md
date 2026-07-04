# CONVI-7208 Investigation: Group Calibration criterion-level comments in CSV export

**Date**: 2026-07-03  
**Tool**: Cursor  
**Ticket**: [CONVI-7208](https://linear.app/cresta/issue/CONVI-7208/group-calibration-add-column-for-scorecard-criterion-level-comments)  
**Source repo**: go-servers (`/Users/xuanyu.wang/repos/go-servers`, branch `main`)  
**Related repo**: director (`/Users/xuanyu.wang/repos/director`)

## Ticket summary

**Current**: "Download session as CSV" on a group calibration session exports answer-key and reviewer rows with criterion grades, but omits criterion-level comments.

**Expected**: Same column pattern as Coaching Hub / QM Report scorecard export — after each criterion grade column, add a comment column for the answer key creator and each participating reviewer.

**Assignee**: Xuanyu Wang  
**Reporter**: Krystal Truong

## Where the export lives today

The export is **not** implemented in go-servers. It is client-side in director:

- `director/packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/components/RecentCalibrationsThreeDotsMenu.tsx`
- Trigger: three-dots menu → "Download session as CSV"
- Data loaded by `useRecentCalibrationsTableData.ts`:
  - `useListDirectorTasks` — session metadata
  - `useQMReportGroupCalibrationStats` — consistency/completion
  - `useListAllScorecards` with `ListScorecardsRequestScorecardView.FULL` — answer key + response scorecards

### What the CSV builder does now

For each criterion (keyed by `score.displayName`):

```typescript
// Grade only — comment omitted
.map((score) => [score.displayName ?? '', score.notApplicable ? 'N/A' : (score.numericValue ?? 0)])
```

It also exports a single scorecard-level `comment` field (`answerKeyScorecard.comment` / `responseScorecard.comment`), which is different from per-criterion comments.

Headers are built in `getRecentCalibrationsRowCSVHeader()`; criterion columns use the display name directly (no paired comment column).

## go-servers data path (already supports comments)

### Storage

`director.scores.comment` — nullable VARCHAR on the scores table.

Model: `go-servers/apiserver/sql-schema/gen/model/scores.go`

### API: ListScorecards

`go-servers/apiserver/internal/coaching/transformers.go`:

```go
func convertScoreToPB(...) *coachingpb.Score {
    return &coachingpb.Score{
        ...
        Comment: converter.ConvertFromNullStringRef(score.Comment),
        ...
    }
}
```

`ListScorecards` with `FULL` view returns scores including `comment`. Group calibration answer-key and response scorecards use the same scores table.

### Frontend mapping

`director/packages/director-api/src/services/cresta-api/coaching/transformers.ts`:

```typescript
export function transformScoreToModel(score: ApiScore): Score {
  return {
    ...
    comment: score.comment,
    displayName: score.criterionDisplayName,
    ...
  };
}
```

**Conclusion**: No new proto field or DB column is required for the basic feature. Comments are already on the wire for `ListScorecards`.

## Reference implementation (desired CSV shape)

`go-servers/apiserver/internal/coaching/action_export_scorecards.go`:

1. `buildCriteriaHeaders()` — for each criterion, append `[displayName]` and `[displayName] comment`
2. `convertScorecardsToCSVBytes()` — score at index `i`, comment at index `i+1`
3. Comment source: `criterionScores[0].Comment.String`
4. Optionally appends collaboration/conversation comments: `conversationToCommentMap`

See also: `/Users/xuanyu.wang/repos/knowledge/export-appeal-comments/research.md`

## insights-server path (not needed for this CSV)

`go-servers/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go`:

- `findScores()` SQL selects `numeric_value`, `not_applicable` but **not** `scores.comment`
- `EvaluatedCriterion` proto (`cresta-proto/cresta/v1/analytics/director_task_stats.proto`) has no comment fields

This API powers stats/charts on the report page, **not** the session CSV download. A prior investigation (go-servers PR #29634) focused on extending this path; that is a separate scope unless product wants comments in stats APIs too.

## Requirements to clarify with PM / design

### 1. Implementation location

| Option | Pros | Cons |
|--------|------|------|
| **A. Director-only** (extend `RecentCalibrationsThreeDotsMenu.tsx`) | Smallest change; data already available via `ListScorecards`; matches current architecture | User expectation was "code in go-servers" |
| **B. New go-servers export RPC** | Server-side parity with QM Report export; reusable for bulk/automation | New proto + handler + director integration; larger scope |
| **C. Both** — director quick fix now, backend export later | Fast ship + long-term consistency | Two implementations to maintain unless director is migrated |

**Recommendation**: Confirm with ticket author whether director-only is acceptable. Data layer in go-servers is already complete for option A.

### 2. Comment content scope

- **Criterion score comment only** (`scores.comment` from scorecard submission)?
- **Also collaboration comments** quoted on the conversation (QM export merges these)?
- Group calibration sessions are QA-internal — are collaboration comments relevant?

### 3. Column naming

QM Report uses: `{Criterion Display Name}` and `{Criterion Display Name} comment`.

Current group calibration CSV uses criterion display name without a `criteria_` prefix in the header map value (internal key still uses `criteria_${displayName}`). Confirm whether comment columns should be `{name} comment` exactly like QM Report.

### 4. Criterion types

Current CSV export only outputs `numericValue` or `N/A`. QM export (`getScoreValue`) also handles:

- Text / sentence criteria (`text_value`)
- User criteria
- Per-message criteria (average percentage)

Which criterion types appear in group calibration templates? Should non-numeric scores export `text_value` in the grade column with comment alongside?

### 5. Answer key row

Confirm answer key creator per-criterion comments should appear on the answer key row (Answer Key = Yes), same as reviewer rows.

### 6. Empty / missing comments

Leave blank cell, or a sentinel like `None` (QM export uses `None` for null scorecard-level comment)?

### 7. Scorecard-level Comment column

Keep the existing single scorecard-level `Comment` column in addition to per-criterion comment columns? QM export has both patterns (per-criterion comments + scorecard comment field).

### 8. Permissions

`scores.comment_access_roles` exists. Should CSV respect role-based comment visibility, or is this always QA-admin facing?

## Proposed implementation

### Minimal path (director, ~0.5–1 day)

**Repo**: director

1. In `RecentCalibrationsThreeDotsMenu.tsx`, when building `criteriaNames`, emit pairs: `[criteria_${name}, name]` and `[criteria_${name}_comment, `${name} comment`]`.
2. When building rows, map `score.comment ?? ''` into `criteria_${name}_comment` for answer key and each response scorecard.
3. Add locale strings under `group-calibration.csv-export.headers` for comment suffix if needed.
4. Add/adjust unit or integration test if the component has test coverage.

**go-servers**: none required unless product mandates server-side export.

### Backend export path (go-servers, ~2–3 days)

**Repos**: cresta-proto, go-servers, director

1. Add RPC e.g. `ExportGroupCalibrationSession` on coaching or analytics service.
2. Reuse `action_export_scorecards.go` patterns:
   - Query answer-key + response scorecards for a `director_task_id`
   - `buildCriteriaHeaders` / score+comment column pairs
   - Handle group-calibration-specific columns (Answer Key Y/N, consistency score, session metadata)
3. Director replaces client-side `getCSV` with download from RPC response.

### If stats API also needs comments (go-servers + cresta-proto, ~2–3 days)

Separate from CSV; only if product asks for comments in `RetrieveDirectorTaskStats` / criterion drill-down:

1. Add `comment` to `EvaluatedCriterion` or `EvaluatedCriterionValue` in `director_task_stats.proto`
2. Select `scores.comment` in `findScores()` SQL
3. Populate in `aggregateGroupCalibrationCriteriaStats()`
4. Regenerate protos; update insights-server tests

## Files to touch (by option)

### Director-only

- `director/.../RecentCalibrationsThreeDotsMenu.tsx`
- `director/.../locales/en-US/director-app-coaching.json` (and other locales if required by team policy)

### go-servers backend export

- `cresta-proto/cresta/v1/coaching/coaching_service.proto` (new RPC)
- `go-servers/apiserver/internal/coaching/action_export_group_calibration.go` (new)
- `go-servers/apiserver/internal/coaching/action_export_scorecards.go` (extract shared helpers)
- `director/.../RecentCalibrationsThreeDotsMenu.tsx` (call new RPC)

### go-servers stats API extension (optional)

- `cresta-proto/cresta/v1/analytics/director_task_stats.proto`
- `go-servers/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go`
- `go-servers/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats_test.go`

## Test plan (draft)

1. Create a group calibration session with answer key + ≥2 reviewer scorecards.
2. Add distinct per-criterion comments on answer key and reviewer scorecards.
3. Download CSV; verify each criterion has grade + comment column pair.
4. Verify answer key row and reviewer rows both include comments.
5. Verify empty comment cells when no comment provided.
6. Regression: existing columns (session name, consistency score, total score) unchanged.

## Next

- Manual QA: download CSV with per-criterion comments on answer key and reviewer rows.

## Implementation (2026-07-03)

Director-only change in `RecentCalibrationsThreeDotsMenu.tsx`:

- Added `buildCriterionCsvColumns()` helper
- Each criterion exports grade column (`{displayName}`) then comment column (`{displayName} comment`) — aligned with go-servers `buildCriteriaHeaders`
- Uses `score.comment`; blank when empty; scorecard-level Comment column unchanged
