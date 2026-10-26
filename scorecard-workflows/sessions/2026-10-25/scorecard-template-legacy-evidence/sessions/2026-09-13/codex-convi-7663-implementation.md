# CONVI-7663 Director use-case scoping implementation

> Superseded on 2026-09-14: the worktree was narrowed to Closed Conversations call sites only. See `../2026-09-14/codex-convi-7663-closed-conversation-scope.md`.

## Context

- Date: 2026-09-13 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7663`
- Branch: `xw/convi-7663-usecase-scope`
- Base: `origin/main` at `40734741778`

## Implemented change

Director permission lookups now use the same effective use-case scope as the scorecard/template list they filter. The patch covers conversation scoring, reusable template filters, coaching scorecards, admin template management, AI Agent template attachment, and Opera template/backfill flows.

The global default of `useGetScorecardTemplatePermissions` remains unchanged because explicit cross-use-case screens still require it. A new `skip` argument prevents bounded consumers from issuing an all-use-case permission request while their use case is not known or their paired list is skipped.

`useScorecardTemplatesLevelSelect` also now distinguishes three modes correctly:

- `allUsecases` loads every available use case;
- explicit `usecaseNames` remains exactly that set;
- omitted scope falls back to the selected use case.

Previously, operator precedence caused any truthy explicit `usecaseNames` value to expand to every available use case.

## Regression boundaries

- Explicit empty arrays still mean all use cases.
- Admin all-use-case/superset mode remains global.
- Existing notification/global selectors retain their behavior through the unchanged permission-hook default.
- Multi-use-case Opera flows pass the same union to the template and permission requests.
- Permission requests are skipped together with paired list requests where the caller has an explicit skip state.

## Validation

- `yarn tsc` in `packages/director-app`: passed.
- `yarn lint:precommit` in `packages/director-app`: passed.
- Focused Vitest run: 3 files, 11 tests passed:
  - `useGetScorecardTemplatesFilteredByPermissions.test.ts`
  - `useScorecardTemplatesLevelSelect.test.ts`
  - `PolicyBackfill.test.tsx`
- Added request-shape coverage for explicit scope, selected-use-case fallback, explicit all-use-case mode, selector expansion behavior, and coordinated skipping.

## Remaining validation

- Exercise an RCG conversation detail flow in a deployed environment and confirm the permission request contains the conversation use case and stays comfortably below the gRPC response limit.
- Historical template reassignment remains a product-wide behavior to validate separately; it was absent from the incident-time RCG template population.
