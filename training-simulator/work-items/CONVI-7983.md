# CONVI-7983: Apply the selected use case to Training Simulator CSV export

**Status:** validating (draft PR open)
**Primary domain:** training-simulator
**Primary subdomain:** reporting
**Last updated:** 2026-11-06

## Objective

Send the selected use-case ID in CSV task-run requests for sessions, lessons, and modules. Backend support is already merged in go-servers #34850.

## Source Context

- Repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/.codex/worktrees/training-csv-usecase/director` (native app tool selected this path).
- Branch: `xw/convi-7983-training-csv-usecase`, based on `origin/main` `7d8ce5f6ba`.

## Plan and validation

Read the current use-case config in the shared CSV button, pass the ID into the loader, and add `usecaseIds` to every task-run batch. Avoid unfiltered downloads when no use case is available. Extend existing loader tests for all export types, batching, and missing use-case handling; run focused tests, typecheck, and lint. Open a draft PR using the repository template. No backend changes.

## Timeline

- 2026-11-06: User authorized frontend implementation and draft PR, and supplied CONVI-7983. Investigation: [CSV use-case verification](../sessions/2026-11-06/codex-csv-usecase-filter.md).

## Result

- Draft PR: [director #24711](https://github.com/cresta/director/pull/24711).
- Commit: `91c2d5b154`, pushed to `xw/convi-7983-training-csv-usecase`; source checkout clean.
- 22 CSV tests, app typecheck, changed-file ESLint/Biome, and commit hooks passed; focused reviewer found no actionable issues.
- Live export and deployment validation remain pending. An explicit use-case filter also excludes attempts with unset use cases, following existing backend semantics.
- Next: review CI and validate exports with populated data across multiple use cases before merging.
- Evidence: [implementation session](../sessions/2026-11-06/codex-convi-7983-csv-usecase.md).
