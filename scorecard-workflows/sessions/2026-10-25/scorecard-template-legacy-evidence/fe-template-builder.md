# FE: Scorecard Template Builder — Criterion Options Editing

**Created:** 2026-03-31
**Updated:** 2026-03-31

## Overview

The template builder lives at `director/packages/director-app/src/features/admin/coaching/template-builder/`. It's a 2-tab form:

1. **Configure Criteria** (`TabType.CONFIGURE`) — chapters, criteria, outcomes
2. **Scorecard Access** (`TabType.ACCESS`) — permissions

For NAScore, we only touch tab 1, specifically the criterion options configuration.

## Component Hierarchy (Criterion Options)

```
TemplateBuilderForm.tsx                       ← top-level, tab navigation
  └─ TemplateBuilderFormConfigurationStep     ← tab 1 content
       └─ TemplateBuilderCriterion            ← each criterion card
            └─ TemplateBuilderCriterionConfiguration  ← criterion settings panel
                 └─ TemplateBuilderScoreType.tsx       ← score type selector + options editor
                      │
                      ├─ (LabeledRadios / DropdownNumericValues)
                      │   └─ CriteriaLabeledOptions.tsx  ← options table + "Allow N/A" checkbox
                      │
                      └─ (NumericRadios)
                          └─ CriteriaRangeOptions.tsx    ← min/max range + "Allow N/A" checkbox
```

## Key File: `CriteriaLabeledOptions.tsx`

This is the primary file we need to modify. It renders the options table for `labeled-radios` and `dropdown-numeric-values` criterion types.

### Layout

```
┌─────────────────────────────────────────────────┐
│  Value (label)                    │  Score       │   ← header row (line 134-147)
├───────────────────────────────────┼──────────────┤
│  [Yes              ]              │  [1  ] [🗑]  │   ← option rows (line 149-219)
│  [No               ]              │  [0  ] [🗑]  │      each: TextInput(label) + NumberInput(score) + delete
├───────────────────────────────────┴──────────────┤
│                    ☐ Allow N/A      [+ Add Option]│   ← footer (line 221-236)
└─────────────────────────────────────────────────┘
```

### Form Fields Wired

| Field | Hook | Form Path | Purpose |
|-------|------|-----------|---------|
| Options array | `useFieldArray` (line 32) | `settings.options` | Label/value pairs |
| Scores array | `useFieldArray` (line 58) | `settings.scores` | Separate score values (when `enableDuplicateScoreForCriteria` flag is ON) |
| Show N/A | `useController` (line 40) | `settings.showNA` | "Allow N/A" checkbox |
| Exclude from QA | `useController` (line 44) | `settings.excludeFromQAScores` | Controls whether Score column is visible |
| AutoQA detected | `useController` (line 48) | `auto_qa.detected` | Reset on option delete |
| AutoQA not detected | `useController` (line 52) | `auto_qa.not_detected` | Reset on option delete |

### Score Input — Two Modes

The `enableDuplicateScoreForCriteria` feature flag controls score handling:

**When flag is ON** (line 165-182):
- Score comes from `settings.scores[index].score` (separate from option value)
- Label is in `settings.options[index].label`, value is auto-assigned index
- Allows different scores than the option value

**When flag is OFF** (line 183-203):
- Score comes from `settings.options[index].value` (value IS the score)
- Changing score resets auto-QA detected fields

### "Allow N/A" Checkbox

- Rendered in the footer (line 222-225)
- Bound to `settings.showNA` via `useController`
- Uses `checkedControllerFieldToMantine()` helper to convert react-hook-form field to Mantine checkbox props
- Currently: toggling ON/OFF only sets `showNA` boolean — no additional UI appears for configuring N/A score

### Option Management

- **Add**: `onAddLabel()` (line 82-88) — appends `{ label: '', value: maxId + 1 }`, plus matching score entry if flag is ON
- **Delete**: `handleRemoveOption()` (line 90-130) — removes option + score, renormalizes indexes, remaps branch conditions and auto-QA detected fields
- **Initialize**: `useOnMount` (line 66-75) — creates first option if empty, initializes scores array for legacy templates

### CSS Layout (`CriteriaLabeledOptions.module.css`)

```css
.optionsRow        { display: flex; align-items: center; }
.optionsRow__label { flex: 4; }       /* label takes most space */
.optionsRow__value { width: 9em; }    /* score + delete button */
.footer            { display: flex; justify-content: flex-end; gap: var(--spacing-md); }
```

## Key File: `CriteriaRangeOptions.tsx`

Simpler component for `numeric-radios`. Renders min/max range inputs and the same "Allow N/A" checkbox.

```
┌────────────────────────────────────────┐
│  From [0   ]   To [10  ]              │   ← range inputs (line 49-64)
├────────────────────────────────────────┤
│                          ☐ Allow N/A  │   ← footer (line 66-68)
└────────────────────────────────────────┘
```

No options table here — just min/max. If we want NAScore for numeric-radios, we'd add a score input next to the checkbox.

## Key File: `TemplateBuilderScoreType.tsx`

Routes to the correct options editor based on criterion type (line 296-318):

```tsx
{(typeField.value === CriterionTypes.LabeledRadios ||
  typeField.value === CriterionTypes.DropdownNumericValues) && (
  <>
    {/* multi-select checkbox for dropdown only */}
    <CriteriaLabeledOptions fieldName={fieldName} showNumericOptions={!excludeFromQAScores} />
  </>
)}
{typeField.value === CriterionTypes.NumericRadios && <CriteriaRangeOptions fieldName={fieldName} />}
```

Note: `showNumericOptions` prop controls whether the Score column is visible. When "Evaluate Scores" is OFF (`excludeFromQAScores: true`), the score column is hidden.

Also handles score type switching (line 93-104): when switching FROM `Sentence` to a scorable type, sets `settings: { showNA: true }`.

## Key File: `useSaveScorecardTemplate.ts` — Form → API Transform

The save flow transforms form data before sending to the API.

### `transformValueCriterionSettingsForApi()` (line 61-80)

Extracts shared settings from the form criterion:

```typescript
return {
    autoFail,
    showNA: settings.showNA,
    excludeOutcomeInsights: settings.excludeOutcomeInsights || settings.excludeFromQAScores,
    excludeFromQAScores: settings.excludeFromQAScores,
};
```

**This is where we need to add `naScore: settings.naScore`.**

### `extractCriterionSettingsForApi()` (line 129-176)

Type-specific transform. Calls `transformValueCriterionSettingsForApi()` then adds type-specific fields (range, options, scores, enableMultiSelect). The shared settings (including `showNA`) come from the spread `...transformValueCriterionSettingsForApi(criterion.settings)`.

## Type Definitions

### API Type: `ScorecardCriterionTemplateBaseWithValue` (`director-api/src/types/models/scoring.ts:161-173`)

```typescript
export type ScorecardCriterionTemplateBaseWithValue = ScorecardCriterionTemplateBase & {
  settings?: {
    showNA?: boolean;
    autoFail?: AutoFail;
    excludeOutcomeInsights?: boolean;
    excludeFromQAScores?: boolean;
  };
  auto_qa?: ScorecardTemplateAutoQA;
};
```

**We need to add `naScore?: number;` to this settings type.**

### Form Type: `TemplateBuilderFormType` (`formTypes.ts`)

Uses the API types directly (no separate form-specific settings type). The form path `settings.naScore` will be available once the API type is updated.

### Constants: `consts.ts`

```typescript
export const DEFAULT_CRITERION = {
  type: CriterionTypes.LabeledRadios,
  weight: 1,
  displayName: 'This is a new criterion',
  required: true,
  settings: {
    options: DEFAULT_CRITERION_SETTINGS_OPTIONS,
    range: DEFAULT_CRITERION_SETTINGS_RANGE,
    showNA: true,
  },
};
```

No change needed here — `naScore` defaults to `undefined` (legacy skip behavior).

## What We Need to Change for NAScore

### 1. API Type (`director-api/src/types/models/scoring.ts`)
- Add `naScore?: number` to `ScorecardCriterionTemplateBaseWithValue.settings`

### 2. `CriteriaLabeledOptions.tsx` — Main Change
- Add `useController` for `settings.naScore`
- When `showNA` is checked, render an N/A row at the bottom of the options table:
  - Label: disabled TextInput, value "N/A"
  - Score: NumberInput bound to `settings.naScore`, placeholder "no score"
  - No delete button, no drag handle
- When `showNA` is unchecked, clear `naScore` field

### 3. `CriteriaRangeOptions.tsx` — Optional
- If we want NAScore for numeric-radios, add a score input conditionally shown when "Allow N/A" is checked

### 4. `useSaveScorecardTemplate.ts`
- In `transformValueCriterionSettingsForApi()`, add: `naScore: settings?.naScore ?? undefined`

### 5. Feature Flag
- Gate the N/A score input behind `enableNAScore` feature flag (already created: cresta/config#142788)

### 6. No changes needed
- `consts.ts` — `naScore` defaults to `undefined` naturally
- `formTypes.ts` — uses API types, no separate definition
- `TemplateBuilderScoreType.tsx` — no routing changes, options editors handle it internally
- `TemplateBuilderForm.tsx` — no changes
