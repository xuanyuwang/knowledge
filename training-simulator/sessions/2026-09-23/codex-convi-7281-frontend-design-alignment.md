# CONVI-7281 frontend design alignment

**Date:** 2026-09-23
**Primary source repo:** `/Users/xuanyu.wang/repos/director-convi-7281`
**Branch:** `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`

## Input

The user supplied the intended Finalize design after reviewing the first implementation. The design distinguishes a section title from the field label and gives each option an icon, title, and explanatory description.

## Changes

- Kept “Applicable product areas” as the section heading and changed the row label to “Apply rule to.”
- Changed the option titles to “All conversations” and “Only Training Simulator.”
- Added the design descriptions explaining production and simulated-conversation behavior.
- Added conversation and clipboard-list icons to the dropdown options.
- Added custom option spacing, typography, normal description wrapping, and a responsive 512 px selector width matching the design's logical dimensions.
- Preserved the existing `PolicySourceType` mapping, feature gate, draft save, and create/edit persistence behavior.

## Validation

- Focused Opera mapping tests: 3 files, 23 tests passed.
- Repository TypeScript: passed.
- Changed-package precommit lint: passed.
- `git diff --check`: passed.

## PR handoff

- Rebased onto current Director `main` without conflicts.
- Commit: `d3639ac4a3` (`[CONVI-7281] Add Training Simulator rule applicability`).
- Draft PR: [director#23164](https://github.com/cresta/director/pull/23164).
- Used `docs/pull_request_template.md` unchanged except for filling the Description, per the repository `director-commit-and-pr` skill.
- Pushed over HTTPS using GitHub CLI authentication; no SSH credential was used.
- CI was queued/pending immediately after PR creation.
