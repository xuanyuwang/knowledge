# CONVI-6955: Required Comments Dropdown Duplicates and Mis-remaps N/A

**Created:** 2026-06-01
**Issue:** [CONVI-6955](https://linear.app/cresta/issue/CONVI-6955/na-in-scorecard-duplicate-na-entries-in-require-comments-dropdown)
**PR:** [director#19238](https://github.com/cresta/director/pull/19238)

## Investigation

The template-builder "Require comments" dropdown was built from option indices, with a synthetic legacy N/A entry appended whenever `showNA` was enabled. That logic did not distinguish between:

- legacy N/A exposed through `showNA`
- scored N/A represented as a real option with `isNA: true`

Separately, deleting labeled options already remapped branch and AutoQA indices, but it did not remap `settings.commentSettings.requiredForValues`, which is also stored as option indices.

## Root Cause

This ticket had one independent root cause with two visible symptoms:

1. `CriterionConfigurationRequireComments.tsx` treated `showNA` as sufficient to append a synthetic N/A entry, even when `settings.options` already contained a real scored-N/A option.
2. `CriteriaLabeledOptions.tsx` did not remap `commentSettings.requiredForValues` when options were deleted, so stale indices could shift onto the wrong option, including N/A.

## Solution

Implemented in `director` on branch `xwang/convi-6955-na-required-comments`.

- Added a dedicated helper to build the require-comments dropdown options.
- Synthetic N/A is now added only when `showNA` is enabled and no real scored-N/A option exists.
- Scored N/A is shown only once, using the real option index.
- Added remapping for `commentSettings.requiredForValues` during option deletion.
- Deleting a normal option now drops that requirement instead of silently converting it to N/A.
- Deleting a scored N/A converts the requirement to the legacy N/A sentinel only when `showNA` stays enabled.

## Verification

Targeted regression coverage was added for:

- single N/A entry when scored N/A exists
- deleting a regular option does not remap the selection to N/A
- deleting scored N/A falls back to the legacy sentinel only when `showNA` remains enabled

Command run:

```bash
node /Users/xuanyu.wang/repos/director/node_modules/vitest/vitest.mjs run --environment jsdom packages/director-app/src/features/admin/coaching/template-builder/configuration/CriterionConfigurationRequireComments.utils.test.ts
```
