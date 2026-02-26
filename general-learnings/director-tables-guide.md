# Director Tables Guide: Adding Columns

**Created:** 2026-02-25
**Source:** agent-quintiles-support project experience

## Overview

Director has two table components:
- **DirectorTable** — Tanstack React Table wrapper with sticky columns and column groups. Used for Agent Leaderboard, Performance tables.
- **GridTable** — Simpler table with `static: true` columns for left-side pinning. Used for LeaderboardPerCriterion.

## DirectorTable: Column Definition

Columns use Tanstack's `ColumnDef<TData>` with extended `meta`:

```typescript
columnHelper.accessor('quintileRank', {
  header: t('columns.quintile-rank', 'Quintile Rank'),
  cell: (cell) => {
    const value = cell.getValue();
    if (!value || value === QuintileRank.QUINTILE_RANK_UNSPECIFIED) return '--';
    return String(QuintileRankNumber[value]);
  },
  meta: {
    fixedWidth: 80,          // exact width in px
    sticky: true,            // pin to left on horizontal scroll (requires fixedWidth)
    cellBackgroundColor: (cell) => getHeatmapColor(cell.getValue()),
    headerAriaLabel: 'Accessible label',
    exportHeader: 'CSV Header',
    exportValue: (row) => row.field,
  },
})
```

Key `meta` properties:
| Property | Purpose |
|----------|---------|
| `sticky` | Pin column to left during horizontal scroll |
| `fixedWidth` | Exact width (required for sticky columns) |
| `cellBackgroundColor` | Dynamic background per cell (for heatmaps) |
| `exportHeader` / `exportValue` | Custom CSV export |

## Column Groups

Related columns grouped under a shared header:

```typescript
// Sticky group with no visible header
columnHelper.group({
  header: ' ',
  columns: [iconColumn, nameColumn, teamColumn],
  meta: { sticky: true, fixedWidth: stickyHeadersWidth },
})

// Named metric group
columnHelper.group({
  id: 'Performance',
  header: 'Performance',
  columns: [qaScoreColumn, targetMetColumn],
})
```

**Sticky width tracking** — accumulate widths of all columns in the sticky group:

```typescript
let stickyHeadersWidth = 0;
stickyHeadersWidth += ICON_COL_WIDTH;   // 60
stickyHeadersWidth += NAME_COL_WIDTH;   // 150
stickyHeadersWidth += TEAM_COL_WIDTH;   // 150

// Set total on the group
meta: { sticky: true, fixedWidth: stickyHeadersWidth }
```

## Adding Icons Inline with Agent Names

Two approaches used in the quintiles project:

### 1. Icon inside the name cell (most common)

```typescript
columnHelper.accessor('name', {
  header: t('columns.name', 'Name'),
  cell: (cell) => {
    const row = cell.row.original;
    return (
      <SharedTooltip label={t('columns.name-tooltip', { name: cell.getValue() })}>
        <Flex align="center" gap="xs">
          {enableQuintileRank && <QuintileRankIcon quintileRank={row.quintileRank} />}
          <Link to={agentOverviewPath}>{cell.getValue()}</Link>
        </Flex>
      </SharedTooltip>
    );
  },
  meta: { sticky: true, fixedWidth: NAME_COL_WIDTH },
})
```

### 2. Passing a lookup map to sub-tables

When the parent page fetches quintile data but a child table doesn't have it on its row type:

```typescript
// Parent builds map
const agentToQuintileRank = useMemo(() => {
  const map = new Map<string, QuintileRank>();
  for (const score of qaScores) {
    const name = score.groupedBy?.user?.name;
    if (name) map.set(name, score.groupedBy.quintileRank);
  }
  return map;
}, [qaScores]);

// Pass to child
<AgentLeaderboardByMetric agentToQuintileRank={agentToQuintileRank} />

// Child reads from map in cell renderer
{enableQuintileRank && <QuintileRankIcon quintileRank={agentToQuintileRank.get(row.resourceName)} />}
```

## Feature Flag Gating for Columns

### Via `useVisibleColumnsForLeaderboards` hook

Central hook that returns a `Set<keyof LeaderboardRow>` of visible column keys:

```typescript
const enableQuintileRank = useFeatureFlag('enableQuintileRank');

return useMemo((): Set<keyof LeaderboardRow> => {
  const visibleColumns: Set<keyof LeaderboardRow> = new Set([...alwaysVisible]);
  if (enableQuintileRank) {
    visibleColumns.add('quintileRank');
  }
  return visibleColumns;
}, [enableQuintileRank]);
```

### In column definitions

```typescript
// Guard the column
if (visibleColumns.has('quintileRank')) {
  columns.push(columnHelper.accessor('quintileRank', { ... }));
}

// Guard inline icons
{enableQuintileRank && <QuintileRankIcon quintileRank={row.quintileRank} />}
```

## Data Threading: API Response -> Row -> Column

### Step 1: Add field to row type

```typescript
// In types.ts
interface LeaderboardRow {
  // ... existing fields
  quintileRank?: QuintileRank;
}
```

### Step 2: Populate from API response

QA score data comes from `useQAScoreStats`. Map it onto rows by matching `groupedBy?.user?.name` (the resource name, e.g. `users/12345`):

```typescript
// In AgentLeaderboardPage.tsx or similar
for (const score of qaScoreResult.scores) {
  const row = agentIdToData.get(score.groupedBy?.user?.name);
  if (row) {
    row.quintileRank = score.groupedBy?.quintileRank;
  }
}
```

### Step 3: Use in column cell

```typescript
cell: (cell) => String(QuintileRankNumber[cell.getValue()])
```

## GridTable (Alternative Pattern)

Used by `LeaderboardPerCriterion`. Simpler API with `static: true` instead of sticky meta:

```typescript
const columns: ColumnDefinition<LeaderboardPerCriterionRow>[] = [
  {
    columnId: 'avatar',
    header: '',
    static: true,           // pinned to left
    CellRender: ({ row }) => <ProfileAvatarWithLoginStatus ... />,
    meta: { fixedWidth: 50 },
  },
  {
    columnId: 'name',
    header: 'Name',
    static: true,
    accessor: 'name',
    meta: { fixedWidth: 188 },
  },
  // Dynamic date columns (scrollable)
  ...dateColumns.map(date => ({
    columnId: date,
    header: date,
    static: false,          // scrolls horizontally
    meta: { fixedWidth: 120 },
  })),
];

<GridTable columns={columns} data={rows} pagination initialPageSize={10} />
```

| | DirectorTable | GridTable |
|--|---------------|-----------|
| Pinning | `meta.sticky: true` | `static: true` |
| Column groups | Yes (multi-level headers) | No (flat) |
| Best for | Leaderboards with metric groups | Tables with many scrollable date/criterion columns |

## Column Position: Where to Insert

Column position matters. In Agent Leaderboard, the order is:

1. Sticky group (Icon + Name + Team)
2. Conversation Volume group
3. Performance group
4. AHT group
5. Assistance group
6. Engagement group
7. Live Assist group
8. **Quintile Rank** (standalone, after Live Assist)
9. Outcome Metrics group

Insert by pushing to the `columns` array at the right index, or use `splice` if inserting between existing groups.

## Checklist: Adding a Column to a DirectorTable

1. **Row type** — Add the field to the row interface (e.g., `LeaderboardRow` in `types.ts`)
2. **Data threading** — Populate the field from the API response in the data-building `useMemo`
3. **Feature flag** (if conditional) — Add to `useVisibleColumnsForLeaderboards` hook
4. **Column definition** — Create with `columnHelper.accessor()`, set `meta.fixedWidth`
5. **Column position** — Push to `columns` array at the correct position
6. **Sticky width** (if sticky) — Add width to `stickyHeadersWidth` accumulator
7. **Inline icons** (if needed) — Add to the name column's cell renderer, gated by feature flag
8. **CSV export** (if needed) — Add header and value to the export callback
9. **i18n** — Wrap column header and tooltips in `t()` calls
