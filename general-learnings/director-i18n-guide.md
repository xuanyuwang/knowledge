# Director i18n Guide

**Created:** 2026-02-25
**Source:** agent-quintiles-support project experience

## Overview

Director uses **i18next** with Babel-based extraction. All user-facing strings must be wrapped in `t()` calls or `<Trans>` components. The pre-commit hook extracts keys into locale JSON files automatically.

## Basic Pattern: `useTranslation`

```typescript
const { t } = useTranslation('director-app-coaching', { keyPrefix: 'agent-coaching.plan.header' });

// Simple key with default value
t('history-button', 'Coaching Plan History')

// With variable interpolation
t('default-plan-name', {
  defaultValue: 'Coaching plan for {{agentName}}',
  agentName: agent.fullName || agent.username,
})
```

- **Namespace** = locale JSON file name (e.g., `director-app-coaching` -> `locales/en-US/director-app-coaching.json`)
- **keyPrefix** = dot-separated path into the JSON hierarchy, avoids repeating the prefix in every `t()` call
- Always provide a **default value** as the second argument or in `defaultValue` option — this is what the extractor uses

## Multiple Translation Namespaces

When a component needs keys from different namespaces or prefixes, create multiple `t` functions:

```typescript
const { t } = useTranslation('director-app-coaching', { keyPrefix: 'agent-coaching.plan.header' });
const { t: tCommon } = useTranslation('director-app-coaching', { keyPrefix: 'agent-coaching.quintile-rank' });
```

## `<Trans>` Component for Interpolating React Components

Use `<Trans>` when you need to embed React components inside translated text:

```typescript
<Trans
  ns="director-app-coaching"
  i18nKey="agent-coaching.plan.header.creatod-on.with-updated-at"
  defaults="Plan created on {{date}} by {{creator}}, last updated <timeAgo />"
  components={{
    timeAgo: <TimeAgo updating={loading} timestamp={timestamp} />,
  }}
  values={{
    date: getDateString(createdAt),
    creator: creatorName ?? '--',
  }}
/>
```

- `defaults` contains the template with `<componentName />` placeholders
- `components` maps placeholder names to actual React elements
- `values` provides simple variable substitutions (`{{var}}`)

## Ordinal Numbers

i18next supports ordinals via `count` + `ordinal: true`:

```typescript
t('tooltip', {
  count: QuintileRankNumber[quintileRank],  // e.g., 1, 2, 3
  ordinal: true,
  defaultValue: '{{count}}st quintile based on last 7 days',
})
```

This produces: "1st quintile...", "2nd quintile...", "3rd quintile...", "4th quintile..."

### `_I18N_EXTRACT_` for Ordinal/Plural Variants

The Babel extractor only sees the `t()` call in source code. For ordinals, English has 4 forms (`_ordinal_one`, `_ordinal_two`, `_ordinal_few`, `_ordinal_other`). You must explicitly list the variants so the extractor captures them:

```typescript
// The actual rendered call (only this one runs at runtime)
t('quintile-rank.rank-text', {
  count: QuintileRankNumber[quintileRank],
  ordinal: true,
  defaultValue: '{{count}}st quintile',
})

// Extraction hints (never execute — _I18N_EXTRACT_ is always false at runtime)
{_I18N_EXTRACT_ && t('quintile-rank.rank-text_ordinal_two', '{{count}}nd quintile')}
{_I18N_EXTRACT_ && t('quintile-rank.rank-text_ordinal_few', '{{count}}rd quintile')}
{_I18N_EXTRACT_ && t('quintile-rank.rank-text_ordinal_other', '{{count}}th quintile')}
```

`_I18N_EXTRACT_` is a global `declare const` (always falsy at runtime). The Babel extractor statically analyzes the source and picks up these `t()` calls to generate all 4 ordinal keys in the JSON.

**English ordinal suffixes:**
| Suffix | Numbers | Example |
|--------|---------|---------|
| `_ordinal_one` | 1, 21, 31... | 1st, 21st |
| `_ordinal_two` | 2, 22, 32... | 2nd, 22nd |
| `_ordinal_few` | 3, 23, 33... | 3rd, 23rd |
| `_ordinal_other` | 4-20, 24-30... | 4th, 5th, 11th |

## Locale JSON Structure

Files live in `packages/director-app/locales/en-US/`. Structure mirrors the keyPrefix hierarchy:

```json
{
  "agent-coaching": {
    "plan": {
      "header": {
        "history-button": "Coaching Plan History",
        "quintile-rank": {
          "label": "Overall agent rank",
          "rank-text_ordinal_one": "{{count}}st quintile",
          "rank-text_ordinal_two": "{{count}}nd quintile",
          "rank-text_ordinal_few": "{{count}}rd quintile",
          "rank-text_ordinal_other": "{{count}}th quintile"
        }
      }
    },
    "quintile-rank": {
      "tooltip_ordinal_one": "{{count}}st quintile based on last 7 days",
      "tooltip_ordinal_two": "{{count}}nd quintile based on last 7 days"
    }
  }
}
```

## Pre-commit Pipeline

Three hooks run on `.ts/.tsx` files in order:

1. **`precommit-i18n-extract.sh`** — Runs Babel extractor on changed files, updates locale JSONs. If new keys are found, they're added to the JSON and staged automatically.
2. **`precommit-i18next.sh`** — Runs ESLint with `eslint-plugin-i18next` rules (zero-warning tolerance).
3. **`precommit-format.sh`** — Runs `biome format --write` and `biome lint --write` on staged files.

**Important gap:** The format hook runs `biome format` and `biome lint`, but **not** `biome check`. Import sorting (`organizeImports`) is an "assist" action only triggered by `biome check`. This means imports can be committed unsorted. Run `npx biome check --write <file>` manually to fix.

## Common Patterns

### Column headers
```typescript
header: t('columns.quintile-rank', 'Quintile Rank'),
```

### Tooltips with variables
```typescript
<SharedTooltip label={t('name-tooltip', { defaultValue: 'Show agent overview for {{name}}', name: agentName })}>
```

### Conditional i18n (feature-flagged UI)
The `t()` call is only reached if the feature flag is on, but the extractor still picks up the key from static analysis. No special handling needed.

## Checklist: Adding i18n to a New Component

1. Add `useTranslation` with the correct namespace and keyPrefix
2. Wrap all user-facing strings in `t('key', 'Default value')`
3. Use `<Trans>` for strings with embedded React components
4. For ordinals: add `_I18N_EXTRACT_` lines for all variant suffixes
5. Commit — the pre-commit hook auto-extracts keys into the locale JSON
6. Verify the locale JSON diff looks correct (new keys added, no keys removed unexpectedly)
