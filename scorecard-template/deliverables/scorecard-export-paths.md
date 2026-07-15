# Scorecard Export Paths

**Created:** 2026-07-06
**Status:** Working reference
**Purpose:** Document how scorecard data is exported across product surfaces and where representations diverge.

## Why This Document Exists

Scorecards are exported to CSV from multiple workflows. Each path assembles rows from the same underlying Postgres `scores` data, but **formatting logic is duplicated** across frontend and backend code. The same criterion grade can appear as a raw integer in one export and a human-readable label in another.

This is a known consistency issue surfaced during CONVI-7208 (Group Calibration session CSV) and cross-checked against Coaching Hub / QM Report scorecard export.

## Export Path Inventory

| Path | Surface | Where assembled | Key code |
|------|---------|-----------------|----------|
| **Group Calibration session CSV** | Director — QA Group Calibrations Report → Download session as CSV | Browser (client-side `getCSV`) | `RecentCalibrationsThreeDotsMenu.tsx` → `buildCriterionCsvColumns.ts` |
| **QM / Coaching Hub scorecard export** | Director — scorecard list / report export | go-servers `ExportScorecards` RPC | `action_export_scorecards.go` → `getScoreValue()` / `GetCriterionLabelForValue()` |

There is **no go-servers export endpoint** for group calibration sessions today. Session CSV is built entirely in Director from `ListScorecards` (FULL) data already on the row.

## Data Flow (Group Calibration)

```text
RecentCalibrationsThreeDotsMenu.downloadCSV()
  ├─ answerKeyScorecard.scores + row.template
  ├─ responseScorecard.scores + row.template (per reviewer)
  └─ buildCriterionCsvColumns(scores, template)
        └─ getAllCriterionTemplates(template.template)  // template order (CONVI-7208)
              └─ getCriterionGradeValue(score)         // grade formatting (known bug)
```

## Data Flow (QM / Coaching Hub)

```text
ExportScorecards RPC
  └─ convertScorecardsToCSVBytes()
        ├─ buildCriteriaHeaders()           // template order across templates
        └─ getScoreValue(scores, criterion, userMap)
              ├─ labeled-radios / dropdown-numeric-values → GetCriterionLabelForValue()
              ├─ numeric-radios → fmt.Sprintf("%.1f", numericValue)
              ├─ user → resolve textValue to display name via userMap
              └─ sentence / date → textValue
```

## Grade Value Mapping (Canonical — BE Export)

| Criterion type | Export value |
|----------------|--------------|
| `labeled-radios` / `dropdown-numeric-values` | Option **label** from `settings.options` matching `numericValue` |
| `numeric-radios` | Formatted `numericValue` (`"%.1f"`, e.g. `3` → `"3.0"`) |
| `user` / `sentence` / `date` | `textValue` (user resolved to display name on BE) |
| N/A | `'N/A'` |
| Missing score | blank |

## Known Inconsistencies

### 1. Grade representation (active bug)

**Group Calibration CSV** exports raw `score.numericValue` (e.g. `0`, `3`) for all criterion types.

**QM / Coaching Hub export** maps `labeled-radios` and `dropdown-numeric-values` to option labels (e.g. `"Needs Improvement"`, `"Great"`) via template `settings.options`.

| Criterion (snapfinance TASK - Service Standards) | DB `numeric_value` | Group Cal CSV (today) | BE export |
|--------------------------------------------------|--------------------|-----------------------|-----------|
| Thoughtful | 3 | `3` | `Great` |
| Accurate | 2 | `2` | `Good` |

**Bug introduced:** 2025-05-05, director PR #11643 (`70fa3054b2`) — CSV download added with `score.numericValue` and no label lookup. Not introduced by CONVI-7208 template ordering or comment columns.

**Fix plan:** `group-calibration/deliverables/convi-7208-numeric-grade-csv-fix-plan.md`

### 2. Criterion column ordering (fixed in CONVI-7208)

**Before CONVI-7208:** Group Cal CSV column order followed DB score row order.

**After CONVI-7208:** Both paths use **template definition order** (`getAllCriterionTemplates` / `GetCriteriaSlice`).

Reference: director PR #20388, `buildCriterionCsvColumns.ts`.

### 3. Comment columns (addressed in CONVI-7208)

Group Cal CSV now emits paired `{criterion} comment` columns matching BE export layout. See `group-calibration/deliverables/convi-7208-technical-reference.md`.

### 4. Remaining parity gaps (not in scope of CONVI-7208)

| Area | Group Cal CSV | BE export |
|------|---------------|-----------|
| User criterion | raw `textValue` (resource name) if present | resolved to agent display name |
| Per-message criteria | not handled | average percentage via `computePerMessageAverageForExport` |
| Appeal resolve comments | original score comment only | may substitute appeal resolve comment |
| Conversation metadata / CLO columns | not present | included when applicable |

## Reference Implementations

| Role | Location |
|------|----------|
| Group Cal CSV builder | `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/buildCriterionCsvColumns.ts` |
| Group Cal CSV trigger | `.../recent-calibrations-table/components/RecentCalibrationsThreeDotsMenu.tsx` |
| FE display label (chart) | `.../session-snapshot-chart/SessionSnapshotChart.tsx` — `getScoreLabel()` |
| BE export | `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/coaching/action_export_scorecards.go` — `getScoreValue()`, `GetCriterionLabelForValue()` |
| Option label lookup (Go) | `/Users/xuanyu.wang/repos/go-servers/shared/scoring/scorecard_templates.go` — `LabeledCriterion.GetCriterionLabelForValue()` |

## Related Knowledge

- `group-calibration/sessions/2026-07-06/cursor-numeric-grade-csv-investigation.md` — bug timeline and DB validation
- `group-calibration/deliverables/convi-7208-numeric-grade-csv-fix-plan.md` — implementation plan
- `group-calibration/deliverables/convi-7208-technical-reference.md` — CONVI-7208 architecture
