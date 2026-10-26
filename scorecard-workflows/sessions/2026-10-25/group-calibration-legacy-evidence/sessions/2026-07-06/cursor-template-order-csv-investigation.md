# Investigation: template-ordered criteria in Group Calibration session CSV

**Date**: 2026-07-06
**Ticket**: [CONVI-7208](https://linear.app/cresta/issue/CONVI-7208)
**Question**: Can Group Calibration session CSV use scorecard template order (like BE `ExportScorecards`) instead of `scores[]` array order?

## Recommendation

**Yes — low cost.** Template data is already loaded on the table row. Implementation is a small change in `buildCriterionCsvColumns` plus passing `row.template` from `RecentCalibrationsThreeDotsMenu`. No new API calls or backend work.

## Current behavior

Group Calibration CSV builds criterion columns by iterating `answerKeyScorecard.scores` in array order:

- `RecentCalibrationsThreeDotsMenu.tsx` → `buildCriterionCsvColumns(answerKeyScorecard?.scores)`
- `scores[]` comes from `ListScorecards` (FULL view) with **no ORDER BY** on score rows
- Column order is therefore DB-return order, not template order

Backend QM export (`action_export_scorecards.go`) explicitly orders columns via `buildCriteriaHeaders` → `templateStructure.GetCriteriaSlice(false)`.

Within the same Group Calibration report, `SessionSnapshotChart` already orders criteria by template using `getAllCriterionTemplates(template.template).filter(isScorable)`.

## Template data availability

### Already fetched

`useRecentCalibrationsTableData` calls `useListAllScorecards` with `scorecardView: FULL`. That response includes:

```typescript
type ListAllScorecardsResult = {
  scorecards: Scorecard[];
  templates: Record<string, ScorecardTemplate>;
};
```

Templates are keyed by template resource name (`keyBy(res.templates, 'name')`).

### Already resolved per row

```typescript
const template =
  templatesByName?.[
    calibration?.contentConfig?.groupCalibrationContentConfig?.tasks?.[0]?.scorecardTemplateName ?? ''
  ];
```

Row type already carries it:

```typescript
export type RecentCalibrationsTableData = {
  template?: ScorecardTemplate;
  answerKeyScorecard?: Scorecard;
  // ...
};
```

`RecentCalibrationsThreeDotsMenu` already destructures `template` from `row` (used for `templateDisplayName` and edit navigation).

### Template structure is complete

`transformScorecardTemplateToModel` maps proto `template` into `ScorecardTemplate.template: ScorecardTemplateStructure`, including criterion `identifier` and `displayName`.

## Proposed approach

1. Extend `buildCriterionCsvColumns(scores, template?)`.
2. When `template?.template` is present:
   - `orderedCriteria = getAllCriterionTemplates(template.template).filter(isScorable)`
   - Build columns in that order
   - Map scores by `criterionIdentifier` for values
   - Use `criterion.displayName` for headers (stable, matches template)
3. When template is missing: fall back to current `scores[]` iteration (defensive).

Pass `template` from `RecentCalibrationsThreeDotsMenu` for both answer key and reviewer rows so all rows share the same column set/order.

## Cost estimate

| Item | Effort |
|------|--------|
| `buildCriterionCsvColumns.ts` | ~30 lines |
| `RecentCalibrationsThreeDotsMenu.tsx` | Pass `template` (2 call sites) |
| Unit tests | 2 cases: template order + missing score blanks |
| New API / BE / proto | None |

## When template may be absent

`template` is typed optional. Lookup can fail if:

- `scorecardTemplateName` missing on director task config
- Template not present in `ListScorecards` `templates` map (unexpected for active sessions)
- Partial query failure

Mitigation: keep scores-array fallback when `template?.template` is undefined.

In normal Group Calibration flow, session creation requires a submitted answer key against a template, so template should be present whenever CSV download is meaningful.

## Parity notes vs BE `ExportScorecards`

| Aspect | BE export | Proposed FE export |
|--------|-----------|-------------------|
| Order source | `GetCriteriaSlice(false)` | `getAllCriterionTemplates` + `isScorable` |
| Header labels | `criterion.GetDisplayName()` | `criterion.displayName` |
| Missing score cell | Blank | Blank (`undefined` → `''` in CSV) |
| Chapters | Skipped in values | Not in criterion list |
| Sentence/User/Date criteria | Included in headers | Excluded by `isScorable` |
| Branch children | Included in template walk | Included in template walk |
| Per-message criteria | Average in export | Single value (pre-existing FE limitation) |

For typical QA group calibration templates (numeric criteria only), `isScorable` matches practical column set. If exact BE parity is required, drop `isScorable` and use all criteria from `getAllCriterionTemplates`.

## Risks / open questions

1. **Branching templates**: Template order includes all branch criteria; inactive branches may have no score row → blank grade/comment columns. Acceptable and aligned with BE (blank cells for missing values).
2. **Reviewer row column alignment**: Using shared template-derived `criteriaColumns` from answer key row ensures all rows align; reviewer `buildCriterionCsvColumns` only needs fields, not columns (already the pattern).
3. **Scores-only criteria not in template**: Unlikely in group cal (same template enforced). Template-ordered export would omit them; current scores-ordered export would include them. Prefer template order for consistency with QM export.

## Files involved

- `director/.../buildCriterionCsvColumns.ts` — ordering logic
- `director/.../RecentCalibrationsThreeDotsMenu.tsx` — pass `template`
- `director/.../useRecentCalibrationsTableData.ts` — no change (already supplies template)
- Reference: `SessionSnapshotChart.tsx` — existing template-order precedent in same feature

## Decision

Proceed with template-ordered columns when implementing. No additional data loading required.
