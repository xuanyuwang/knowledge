# CONVI-7682 consistency percentage fix

## Context

- Ticket: CONVI-7682
- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7682`
- Branch: `xw/convi-7682-consistency-score-percentage`
- Base: current `origin/main` at `ed18e228d06`

## Evidence reviewed

- Linear ticket content retrieved through the Linear MCP connection.
- Director Group Calibration task-home rendering paths and shared percentage formatter contracts.
- Git history and blame for the progress-table score cell.
- Woolworths production app PostgreSQL via a read-only connection, scoped to aggregate scorecard metadata only.
- Backend `ListScorecards`, scorecard transformation, consistency calculation, and associated tests.

## Findings

- The affected cell is `group-calibration-progress-table/hooks/useGroupCalibrationProgressTableColumns.tsx`.
- `fmtPercentage` expects a fraction between 0 and 1 and uses `Intl.NumberFormat` percentage scaling.
- The Group Calibration API value is already percentage-scaled; adjacent Group Calibration surfaces use `formatPercentage` for the same semantic kind of score.
- Git history identifies the regression in the 2025 QA task-home localization change: the cell previously used `formatPercentage(consistencyScore)`, then switched to `fmtPercentage(consistencyScore)` without dividing by 100.
- The exact introducing change is Director PR #14756, merge commit `d3d29a030bacaf47c3ea42a51198827b72641e9b`, merged 2025-11-07 for DIRP-7999. Its stated purpose was internationalization, not a score-contract change.
- Before PR #14756, `formatPercentage(90, 0)` produced `90%`. After it, `fmtPercentage(90, { numDecimals: 0 })` produced `9,000%` because `Intl.NumberFormat` percent style multiplies a fractional input by 100.
- PR #14756 changed 14 files and did not add or modify a test covering this Group Calibration score display. The recorded GitHub review contains approval but no explanation that the input scale had changed.
- Woolworths production has zero non-null `director.scorecards.calibration_consistency_score` values, including Group Calibration response rows. The column is not the active source for this UI.
- `AllTypesScorecardsToPBScorecards` intentionally does not read the DB consistency column. `ListScorecards` calls `AddConsistencyScore` and populates the protobuf response dynamically.
- `CalculateConsistencyScore` returns `100 * equalCount / scorableCriteriaCount`. Backend tests assert response values including `75.0`, `100 * 2 / 3`, and `100.0`.
- This is therefore a frontend formatting-boundary bug. Backend calculation and storage changes are not warranted.

## Implementation

- Normalize a defined consistency score with `consistencyScore / 100` immediately before `fmtPercentage`.
- Preserve `null` for the formatter fallback and preserve the existing `--` visibility placeholder.

## Validation

- Director app pre-commit lint passed.
- Full TypeScript check passed.
- Shared formatter utility suite passed all 18 tests.
- Diff whitespace check passed.
- Woolworths production app DB inspection used aggregate metadata only and confirmed the consistency column is not persisted.
- Regression provenance verified from Director git history and GitHub PR #14756.

## Pull request

- Commit: `4b301086af`
- Draft PR: https://github.com/cresta/director/pull/22763
- Template selection: `smoke-functional`
- Pending evidence: before/after full-screen video and preview environment link.
