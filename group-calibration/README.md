# Group Calibration

Central knowledge base for Group Calibration product work across backend and frontend.

**Status**: CONVI-7208 shipped — [director PR #20388](https://github.com/cresta/director/pull/20388) open  
**Linear project**: [Group Calibration](https://linear.app/cresta/project/group-calibration-b5db6c57-ef7f-49a2-80e9-bc40ad0c9de2)

## Active tickets

| Ticket | Title | Status | Repo focus |
|--------|-------|--------|------------|
| [CONVI-7208](https://linear.app/cresta/issue/CONVI-7208/group-calibration-add-column-for-scorecard-criterion-level-comments) | Add column for scorecard criterion-level comments in session CSV export | PR open | director |

## Problem (CONVI-7208)

On the QA Group Calibrations Report page, **Download session as CSV** exports one row for the answer key and one row per participating reviewer. Each row includes criterion grade columns and a single scorecard-level **Comment** column, but **no per-criterion comment columns**.

Product expectation: match Coaching Hub / QM Report scorecard export layout — for each criterion, emit a grade column followed by a `{criterion display name} comment` column.

## Current architecture

```text
Director (RecentCalibrationsThreeDotsMenu.tsx)
  ├─ ListDirectorTasks          → group calibration session metadata
  ├─ RetrieveDirectorTaskStats  → consistency / completion stats
  └─ ListScorecards (FULL)      → answer key + response scorecards with scores
        └─ go-servers apiserver (transformers.go convertScoreToPB)
              └─ director.scores.comment (Postgres)
```

The CSV file is assembled entirely in the browser via `getCSV()`; there is no go-servers export endpoint for group calibration sessions today.

## Key finding

`director.scores.comment` is already stored in Postgres and already returned by `ListScorecards` through `convertScoreToPB`. The frontend transformer (`director-api` `transformScoreToModel`) maps `score.comment`. The CSV builder was updated to read `score.comment` when building criterion columns.

Reference formatting: go-servers `action_export_scorecards.go` (`buildCriteriaHeaders`, `convertScorecardsToCSVBytes`).

## Deliverables

| Document | Purpose |
|----------|---------|
| [convi-7208-pm-behavior-summary.md](deliverables/convi-7208-pm-behavior-summary.md) | PM review — accepted behavior decisions |
| [convi-7208-technical-reference.md](deliverables/convi-7208-technical-reference.md) | Architecture, data path, file list |
| [convi-7208-comment-access-roles.md](deliverables/convi-7208-comment-access-roles.md) | Comment access roles definition and enforcement |
| [convi-7208-empty-comment-parity.md](deliverables/convi-7208-empty-comment-parity.md) | Empty comment behavior across exports |

## Decisions

[decisions/2026-07-03-convi-7208-director-only-export.md](decisions/2026-07-03-convi-7208-director-only-export.md)

## Log

- **2026-07-03**: Initial CONVI-7208 investigation; created `group-calibration` knowledge project.
- **2026-07-03**: Decision — director-only export; implemented paired grade + comment columns.
- **2026-07-03**: Published deliverables (PM summary, technical reference, access roles, empty-comment parity).
- **2026-07-03**: Knowledge wrap-up; investigation parked at downstream backfill recommendation.
- **2026-07-03**: Opened [director PR #20388](https://github.com/cresta/director/pull/20388) with CSV export change and unit tests.
