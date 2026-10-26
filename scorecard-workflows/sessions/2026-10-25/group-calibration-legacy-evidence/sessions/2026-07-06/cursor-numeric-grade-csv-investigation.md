# Numeric grade CSV bug investigation (CONVI-7208 follow-up)

**Date:** 2026-07-06
**Ticket:** CONVI-7208
**Trigger:** Krystal PR review — Group Calibration CSV shows `0`/`3` instead of option labels.

## Bug introduction timeline

| Date | Commit | PR | Change |
|------|--------|-----|--------|
| **2025-05-05** | `70fa3054b2` | #11643 | **Bug introduced.** CSV download added; used raw `score.numericValue` with no label lookup. |
| 2025-05-26 | `617c7fae7f` | #12034 | Refactor to `displayName` keys; still `score.numericValue ?? 0`. |
| 2025-12-30 | `f48a1fecf4` | #15795 (CONVI-5920) | Added `notApplicable ? 'N/A'`; non-NA grades still raw numeric. |
| 2026-07-03 | `5df4ce647c` | #20388 (CONVI-7208) | Extracted `buildCriterionCsvColumns`; logic unchanged. |

Pre-existing — not introduced by template ordering or comment columns.

## Mapping feasibility

**Yes — no new API or DB access required.**

`buildCriterionCsvColumns` already receives `row.template` and iterates criteria via `getAllCriterionTemplates(template.template)`. Update `getCriterionGradeValue(score, criterion)` to mirror:

- FE: `SessionSnapshotChart.tsx` `getScoreLabel()`
- BE: `action_export_scorecards.go` `getScoreValue()` / `GetCriterionLabelForValue()`

| Criterion type | CSV value source |
|----------------|------------------|
| `labeled-radios` | `settings.options.find(o => o.value === numericValue)?.label` |
| `dropdown-numeric-values` | same option lookup |
| `numeric-radios` | formatted `numericValue` (BE uses `"%.1f"`) |
| `user` / `sentence` / `date` | `score.textValue` |
| N/A | `'N/A'` |
| missing score | blank |

## Snapfinance DB validation (scoped queries)

Connection: `cresta-cli connstring --read-only us-west-2-prod us-west-2-prod snapfinance-us-west-2`

Always filter: `customer = 'snapfinance' AND profile = 'us-west-2' AND template_id = '718c12b9-440c-4822-849f-638e975a64d3'`

Template: **TASK - Service Standards** (`718c12b9-440c-4822-849f-638e975a64d3`)

### Template structure
- Options live under `template->'items'` tree (recursive flatten), not top-level `criteria`.
- Criterion types in this template:
  - `labeled-radios`: Thoughtful, Kind, Accurate, Straightforward (values 0/2/3 → Needs Improvement/Good/Great)
  - `sentence`: *-Disposition criteria (would use `textValue`, not numeric)

### Score → label verification (group cal scorecard_type 1, 2)

| Criterion | numeric_value (CSV today) | mapped_label |
|-----------|---------------------------|--------------|
| Thoughtful | 3 | Great |
| Kind | 3 | Great |
| Accurate | 2 | Good |
| Straightforward | 3 | Great |

Confirms Krystal's observation: CSV exports integers that should be human-readable labels.

## Recommendation

Fix in `buildCriterionCsvColumns.ts` by passing `criterion` into `getCriterionGradeValue`. Can be same PR or follow-up ticket.
