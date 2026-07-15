# CONVI-7208 follow-up: numeric grade CSV fix plan

**Created:** 2026-07-06
**Status:** Ready to implement
**Ticket:** CONVI-7208 (or follow-up)
**PR:** [director #20388](https://github.com/cresta/director/pull/20388)

## Problem

Group Calibration session CSV exports raw `score.numericValue` (e.g. `0`, `3`) for `labeled-radios` and `dropdown-numeric-values`. QM / Coaching Hub scorecard export maps the same values to option labels (e.g. `"Needs Improvement"`, `"Great"`). Pre-existing since PR #11643 (`70fa3054b2`, 2025-05-05).

## Goal

Parity with BE `getScoreValue()` in `action_export_scorecards.go` and FE `getScoreLabel()` in `SessionSnapshotChart.tsx`.

## Scope

**In scope:** `buildCriterionCsvColumns.ts` grade cell formatting only.
**Out of scope:** user display-name resolution, per-message criteria, appeal comments, conversation metadata columns.

## Implementation

### 1. Update `getCriterionGradeValue` signature

```typescript
function getCriterionGradeValue(
  score: Score | undefined,
  criterion: ScorecardCriterionTemplateBase
): string | undefined
```

Import `CriterionTypes` and relevant criterion template types from `@cresta/director-api`.

### 2. Branch on `criterion.type`

| Type | Export value |
|------|--------------|
| `labeled-radios` | `settings.options.find(o => o.value === score.numericValue)?.label ?? ''` |
| `dropdown-numeric-values` | same option lookup |
| `numeric-radios` | `score.numericValue?.toFixed(1)` (matches BE `%.1f`) |
| `sentence` / `date` / `user` | `score.textValue ?? ''` |
| `notApplicable` | `'N/A'` |
| missing score | `undefined` (blank CSV cell) |

**Remove** `score.numericValue ?? 0` fallback — missing numeric should be blank, not `0`.

### 3. Wire criterion into loop

In `buildCriterionCsvColumns`, change:

```typescript
fields[gradeKey] = getCriterionGradeValue(score);
```

to:

```typescript
fields[gradeKey] = getCriterionGradeValue(score, criterion);
```

No changes to `RecentCalibrationsThreeDotsMenu.tsx` — `row.template` is already passed.

### 4. Reference implementations

| Layer | File | Function |
|-------|------|----------|
| FE (display) | `session-snapshot-chart/SessionSnapshotChart.tsx` | `getScoreLabel()` |
| BE (export) | `go-servers/.../action_export_scorecards.go` | `getScoreValue()` |
| BE (lookup) | `go-servers/shared/scoring/scorecard_templates.go` | `GetCriterionLabelForValue()` |

## Tests

No new test cases in `buildCriterionCsvColumns.test.ts`. Existing tests updated only where `numeric-radios` expectations change (`5` → `'5.0'`). Manual QA covers labeled-radios label mapping.

## Edge cases

| Case | Behavior |
|------|----------|
| `numericValue` not in `settings.options` | Empty string (match SessionSnapshotChart) |
| Branching inactive criteria | Blank grade (no score in map) — unchanged |
| Template missing | Return empty fields/columns — unchanged |
| `user` criterion | Export raw `textValue` (no userMap on FE; BE resolves display name — known parity gap) |

## Manual QA

1. Open snapfinance Group Calibrations Report with template **TASK - Service Standards**.
2. Download session CSV.
3. Verify labeled-radios columns show **Great** / **Good** / **Needs Improvement**, not `0`/`2`/`3`.
4. Cross-check one row against QM scorecard export for the same conversation.

## PR recommendation

Add to **CONVI-7208 PR #20388** while still open — same file, same export-parity goal, small diff. Use a separate ticket only if the PR is merge-imminent.

## Related

- `scorecard-template/deliverables/scorecard-export-paths.md` — export path inventory and known inconsistencies
- `sessions/2026-07-06/cursor-numeric-grade-csv-investigation.md` — bug timeline and DB validation
