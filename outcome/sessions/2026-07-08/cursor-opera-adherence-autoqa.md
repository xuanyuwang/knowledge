# Session: Opera adherence annotations and email AutoQA

**Date:** 2026-07-08
**Tool:** Cursor
**Repos:** go-servers, knowledge

## Goal

Understand how SDX/DDX/DNX annotations are generated and used in email AutoQA; document findings; review PR #29694.

## Key findings

### Two phases

1. **Runtime (Opera):** emits SDX, DDX, DNX, SNX, synthetic SDX, NOX on `moment_annotations`.
2. **Post-close (AutoQA):** reads annotations, filters per agent/message, maps to DETECTED / NOT_DETECTED / N/A.

AutoQA does not scan messages to infer outcomes.

### Positive behaviors

- SDX opens window; DDX on detection (point-only for email); DNX when window closes without detection (spans for email).

### SNX (should NOT do)

- Opera emits SNX trigger (`SHOULD_NOT_DO_X`) + synthetic SDX anchor (`SHOULD_DO_X` with `negative_adherence` label) + DDX/DNX outcome.
- Synthetic SDX exists because window machinery is SDX-based.
- Email spanning rules invert: DDX spans, DNX point-only.

### PR #29694

- Fixes COA-2566: `spansWindow := isDDX(dxx) == isSNX`.
- Recommend approve; optional nits on doc comment and `index == -1` SNX trigger guard.

## Artifacts

- Project: `~/repos/knowledge/outcome/`
- PR: https://github.com/cresta/go-servers/pull/29694
