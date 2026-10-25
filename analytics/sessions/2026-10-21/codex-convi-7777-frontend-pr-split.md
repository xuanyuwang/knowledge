# CONVI-7777 frontend two-PR split

- Date: 2026-10-21 (America/Toronto)
- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`
- Original branch/base: `xw/convi-7777-assistance-usage` / `cd7b46d052c694ec6cd689b8f6ab9f01e9690104`
- Requested outcome: two stacked frontend PRs. Preparation first; table integration second. The user explicitly deferred adding the feature flag to the first PR until later.

## Split plan

1. Preparation (`xw/convi-7777-assistance-usage-preparation`): query key, API response interfaces, dedicated endpoint transport and transformer, query-options factory, translated load error, API tests. Export query from the feature public API; preserve lazy-loading via feature barrel. No request invocation, row-model addition, column, CSV or feature-flag registration.
2. Column (`xw/convi-7777-assistance-usage`), based on preparation: Agent/Team query invocation and prop wiring, numeric row field and seed-before-enrich mapping, accessor/format/visibility, CSV and headers, locale table/CSV labels and hook tests.
3. Publish draft PRs, second targeting first. No merge. Feature flag is a later amendment to preparation; retain draft status for rollout work.

## Preservation

All 15 original changed/new files were copied into `convi-7777-frontend-pr-split-evidence/original/` before isolating preparation. The reviewed HTML remains an immutable pre-split snapshot. No knowledge commit or other worktree cleanup is authorized.

## Validation and publication

Preparation isolated: 11 API tests, both package type checks and production knip pass on the preparation tree. Initial commit lint rejected a deep barrel export; corrected it to export through an api/index.ts public boundary. Commit hooks run normally. Fetched main remains ahead of the implementation baseline; preserve the reviewed baseline for this scope-only split rather than migrating client dependencies as part of it. No explicit credential/profile selection; git and gh authenticate themselves.


## Published result

- Preparation: [Director #24019](https://github.com/cresta/director/pull/24019), draft, `e5b95d2a8e5552ec843ca5714628285e213842be`, targets `main`, 10 files. Includes API tests and only the query load-error locale. The feature/API barrels satisfy the production knip check while retaining the lazy feature boundary.
- Column: [Director #24020](https://github.com/cresta/director/pull/24020), draft, `5f5fb98147ef0b8cd79abc19bf073e827a05e6e2`, targets preparation branch, 9 files. Includes only Agent/Team wiring, row/column/visibility/CSV changes, three header locales and hook tests relative to preparation.
- Both remote PR heads, draft status, base branches, file scopes and mergeability verified. One commit per PR; worktree clean on the column branch. Both PRs attached to this chat.
- All 15 original implementation files remain byte-identical. The only extra source changes are public API barrels and the lazy import needed for independently valid preparation. Saved per-PR patches and published stack identities are beside the original snapshot.
- Fresh checks: preparation API suites 11/11 tests; both package type checks; preparation and combined production knip. Column hooks/CSV 3/3 tests and app type check. Both commits pass the repository's extraction/lint/i18next/format hooks. Frontend tests emit existing sourcemap warnings.
- GitHub CI still running at handoff. Preparation's QA metadata check failed: “A QA testing request checkbox must be selected (`none`|`smoke-UI`|`smoke-functional`|`full`)”. Per the invoked Director commit-and-PR skill, only the template description was filled and the checkboxes/other sections were unchanged. No live QA result is claimed.
- Next: add the feature flag to preparation as the user requested for later, propagate/gate the table integration, finalize draft labels and QA metadata/proof, then complete CI/live integration review before rollout. No merge performed.
