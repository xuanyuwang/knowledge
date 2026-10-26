# Adding a New Filter to Performance / Leaderboard

**Created:** 2026-03-13

How to add a new boolean-style filter to the Performance and Leaderboard pages. Two placement options:

- **"+Filters" dropdown only** — filter lives inside the dropdown menu, no chip in the filter bar
- **Filter bar chip** — filter renders as a standalone chip in the filter bar (like Duration Buckets, Voicemail)

You can combine both, but usually pick one.

---

## Shared setup (required for both placements)

### 1. FilterKey

`src/types/filters/FilterKey.ts` — add a new enum value:

```typescript
MY_NEW_FILTER = 'my_new_filter',
```

### 2. State type

`src/components/insights/types.ts` — add to `CommonInsightsFiltersState`:

```typescript
myNewFilter?: boolean;
```

### 3. Filter state in hook files

For **Performance** (`usePerformanceFilters.tsx`) and **Leaderboard** (`useLeaderboardsFilters.tsx`):

- Add to the Pick type / state type (e.g. `DefaultLeaderboardsFiltersState`)
- Add to `LocalStorage*FiltersState` type: `myNewFilter?: boolean;`
- Add to initial state: `myNewFilter: true,` (or your default)
- Add to `localStateToFilterState`: `myNewFilter: state.myNewFilter ?? true,` (fallback for existing users)
- Add to `filterStateToLocalState`: `myNewFilter: state.myNewFilter,`

### 4. Feature flag guard (optional)

If behind a feature flag:

```typescript
const enableMyNewFilter = useFeatureFlag('enableMyNewFilter');

// In hiddenFilters memo:
if (!enableMyNewFilter) {
  hiddenFilters.push(FilterKey.MY_NEW_FILTER);
}

// In modifiedFiltersState memo:
if (!enableMyNewFilter) {
  return { ...state, myNewFilter: undefined };
}
```

### 5. API pass-through

Wire the filter value into API calls. Typical pattern for `useInsightsRequestParams`:

```typescript
const filterOptions = useMemo<FilterByAttributesOptions>(
  () => ({ myNewApiField: filtersState.myNewFilter }),
  [filtersState.myNewFilter]
);

// Pass as 5th arg:
const params = useInsightsRequestParams(dayRange, attributeStructure, 'daily', filterValues, filterOptions);
```

---

## Option A: "+Filters" dropdown only

The filter appears as a toggleable option inside the "+Filters" dropdown. No chip in the filter bar. This is the pattern used by "Agents only".

### A1. Level select hook

Create `src/components/filters/my-new-filter-level-select/useMyNewFilterLevelSelect.ts`:

```typescript
import type { SingleBooleanLevelSelectHookValue } from '@cresta/director-components';
import { useSingleBooleanLevelSelect } from '@cresta/director-components';

export function useMyNewFilterLevelSelect(): SingleBooleanLevelSelectHookValue {
  return useSingleBooleanLevelSelect('My filter label', 'Yes');
}
```

If both `true` and `false` are meaningful states (not just "on"), override `hasAValue`:

```typescript
import { useMemo } from 'react';

export function useMyNewFilterLevelSelect(): SingleBooleanLevelSelectHookValue {
  const base = useSingleBooleanLevelSelect('My filter label', 'Yes');
  return useMemo(() => ({
    ...base,
    hasAValue: (state: boolean | undefined) => state !== undefined,
  }), [base]);
}
```

Export from `index.ts` and re-export from `src/components/filters/index.ts`.

### A2. Register in hook file

In `useLeaderboardsFilters.tsx` (and/or `usePerformanceFilters.tsx`):

**FILTERS_SELECTION_HOOKS** — maps FilterKey to the level select hook:

```typescript
const FILTERS_SELECTION_HOOKS: LevelSelectHooks = {
  [FilterKey.DEACTIVATED_USERS]: useExcludeDeactivatedUsersLevelSelect,
  [FilterKey.MY_NEW_FILTER]: useMyNewFilterLevelSelect,  // add
};
```

**Menu options** — entry in the "+Filters" dropdown:

```typescript
// Performance: add to FILTER_SELECTION_MENU_OPTIONS in utils.ts
// Leaderboard: add to getFilterMenuOptions()
{
  key: FilterKey.MY_NEW_FILTER,
  label: 'My filter label',
  section: 'general',
},
```

**State accessors** — maps FilterKey to state property name:

```typescript
const FILTER_STATE_ACCESSORS = {
  [FilterKey.MY_NEW_FILTER]: 'myNewFilter',  // add
};
```

### A3. Do NOT add to `components` map

The `OrderedFilterBarFilters` components map is what renders chips in the filter bar. **Skip it** to keep the filter inside "+Filters" only. `OrderedFilterBar` returns `null` for filter keys without a component entry (line 226).

### A4. Do NOT add to `FEATURED_FILTERS`

`FEATURED_FILTERS` makes filters always visible in the bar. Skip it.

---

## Option B: Filter bar chip

The filter renders as a standalone chip in the filter bar. This is the pattern used by "Exclude deactivated users".

### B1. Level select hook

Same as A1. Required for the "+Filters" dropdown to manage selection state.

### B2. Register in hook file

Same as A2 (FILTERS_SELECTION_HOOKS, menu options, state accessors).

### B3. Add to `components` map

In the `useMemo` that builds `OrderedFilterBarFilters`:

```typescript
[FilterKey.MY_NEW_FILTER]: (options: FilterComponentOptions) => (
  <BooleanFilter
    className={options.className}
    filterKey={options.filterKey}
    value={filters.myNewFilter}
    onChange={updateFilters('myNewFilter')}
    label="My filter label"
    displayText={{
      yesText: 'Yes',
      noText: 'No',
      noValueText: 'Yes',
    }}
    isClearable={false}
  />
),
```

Add `filters.myNewFilter` to the `useMemo` dependency array.

### B4. Optionally add to `FEATURED_FILTERS`

If the filter should always be visible in the bar (not requiring "+Filters" selection first):

```typescript
const FEATURED_FILTERS = [
  FilterKey.MY_NEW_FILTER,  // always visible
];
```

If omitted, the chip only appears after the user selects it from "+Filters".

---

## Leaderboard: disabling on Manager tab

For Leaderboard, if the filter should be hidden on the Manager tab:

```typescript
// In hiddenFilters memo:
if (activeTab === LeaderboardTabs.MANAGERS) {
  hiddenFilters.push(FilterKey.MY_NEW_FILTER);
}
```

For a filter bar chip (Option B), you can also disable instead of hide:

```typescript
isDisabled={activeTab === LeaderboardTabs.MANAGERS}
```

---

## Key files reference

| Area | Performance | Leaderboard |
|------|------------|-------------|
| Filter hook | `src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx` | `src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx` |
| Menu options / utils | `src/components/insights/hooks/performance-filters/utils.ts` | Inline in hook file (`getFilterMenuOptions()`) |
| Level select hooks | `src/components/filters/<name>-level-select/` | Same (shared) |
| FilterKey | `src/types/filters/FilterKey.ts` | Same |
| State types | `src/components/insights/types.ts` | Same |
| Filter bar | `src/components/filters/filter-bar/OrderedFilterBar.tsx` | Same |
| `useFiltersSelection` | `src/components/filters/hooks/useFiltersSelection.tsx` | Same |

---

## How the pieces connect

```
"+Filters" dropdown
  └─ useFiltersSelection()
       ├─ FILTERS_SELECTION_HOOKS[filterKey]  →  level select hook (inline UI in dropdown)
       ├─ menu options                        →  dropdown entry label + section
       ├─ FILTER_STATE_ACCESSORS              →  maps FilterKey → state property name
       └─ hasAValue()                         →  determines "selected" state (checkmark in dropdown)

Filter bar chips
  └─ OrderedFilterBar
       ├─ filtersOrder                        →  which filter keys to render (from useFiltersSelectionState)
       ├─ filters[filterKey]                  →  component from OrderedFilterBarFilters map
       └─ FEATURED_FILTERS                    →  always-visible filter keys
```
