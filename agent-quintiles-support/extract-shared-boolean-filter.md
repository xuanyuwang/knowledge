# Extract Shared Boolean Filter Components

Created: 2026-03-24
PR Comment: https://github.com/cresta/director/pull/17558#discussion_r2978949515

## Context

PR reviewer suggested extracting a shared component for the `LIST_AGENT_ONLY` BooleanFilter since the same JSX is duplicated across 3 filter hooks. Investigation shows `DEACTIVATED_USERS` has the exact same duplication pattern.

## Duplicated Filters

### 1. `LIST_AGENT_ONLY` (BooleanFilter)

Identical JSX in 3 hooks:
- `usePerformanceFilters.tsx`
- `useLeaderboardsFilters.tsx`
- `useAssistanceFilters.tsx`

```tsx
[FilterKey.LIST_AGENT_ONLY]: (options: FilterComponentOptions) => (
  <BooleanFilter
    className={options.className}
    filterKey={options.filterKey}
    value={filters.listAgentOnly}
    onChange={updateFilters('listAgentOnly')}
    label={tCommon('agents-only-filter', 'Agents only')}
    displayText={{
      yesText: tCommon('boolean-filter.yes', 'Yes'),
      noText: tCommon('boolean-filter.no', 'No'),
      noValueText: tCommon('boolean-filter.no', 'No'),
    }}
  />
),
```

### 2. `DEACTIVATED_USERS` (BooleanFilter)

Duplicated in the same 3 hooks with minor variations:

**Base pattern (usePerformanceFilters):**
```tsx
[FilterKey.DEACTIVATED_USERS]: (options: FilterComponentOptions) => (
  <BooleanFilter
    className={options.className}
    filterKey={options.filterKey}
    value={filters.excludeDeactivatedUsers}
    onChange={updateFilters('excludeDeactivatedUsers')}
    label={tCommon('exclude-deactivated-users', 'Exclude deactivated users')}
    displayText={{
      yesText: tCommon('boolean-filter.yes', 'Yes'),
      noText: tCommon('boolean-filter.no', 'No'),
      noValueText: tCommon('boolean-filter.no', 'No'),
    }}
  />
),
```

**Variations:**
- **useLeaderboardsFilters**: adds `isDisabled={activeTab === LeaderboardTabs.MANAGERS}` and `isClearable={false}`
- **useAssistanceFilters**: adds `isClearable={false}`, uses `options?.className` (optional chaining)

## Proposed Solution

Extract shared filter component functions that accept `value`, `onChange`, and optional overrides:

### Option A: Shared factory functions

Create `createListAgentOnlyFilter` and `createExcludeDeactivatedUsersFilter` factory functions in a shared location (e.g., `components/filters/shared-boolean-filters/`).

```tsx
// shared-boolean-filters.tsx
export function createListAgentOnlyFilter(
  value: boolean | undefined,
  onChange: (val: boolean | undefined) => void,
  overrides?: Partial<BooleanFilterProps>
): (options: FilterComponentOptions) => JSX.Element {
  return (options) => (
    <BooleanFilter
      className={options.className}
      filterKey={options.filterKey}
      value={value}
      onChange={onChange}
      label={tCommon('agents-only-filter', 'Agents only')}
      displayText={{
        yesText: tCommon('boolean-filter.yes', 'Yes'),
        noText: tCommon('boolean-filter.no', 'No'),
        noValueText: tCommon('boolean-filter.no', 'No'),
      }}
      {...overrides}
    />
  );
}
```

### Option B: Wrapper components (simpler)

Create `<ListAgentOnlyFilter>` and `<ExcludeDeactivatedUsersFilter>` wrapper components:

```tsx
// ListAgentOnlyFilter.tsx
export function ListAgentOnlyFilter({
  options,
  value,
  onChange,
  ...rest
}: { options: FilterComponentOptions; value: boolean | undefined; onChange: ... } & Partial<BooleanFilterProps>) {
  const { t: tCommon } = useTranslation('director-app', { keyPrefix: 'common' });
  return (
    <BooleanFilter
      className={options.className}
      filterKey={options.filterKey}
      value={value}
      onChange={onChange}
      label={tCommon('agents-only-filter', 'Agents only')}
      displayText={{...}}
      {...rest}
    />
  );
}
```

**Recommendation**: Option B (wrapper components) is simpler and more idiomatic React. Each hook just passes `value`, `onChange`, and any overrides like `isDisabled` or `isClearable`.

## Files to Modify

### New files:
- `packages/director-app/src/components/filters/list-agent-only-filter/ListAgentOnlyFilter.tsx`
- `packages/director-app/src/components/filters/exclude-deactivated-users-filter/ExcludeDeactivatedUsersFilter.tsx`

### Modified files:
- `packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx`
- `packages/director-app/src/components/insights/leaderboards/hooks/useLeaderboardsFilters.tsx`
- `packages/director-app/src/components/insights/hooks/useAssistanceFilters.tsx`
