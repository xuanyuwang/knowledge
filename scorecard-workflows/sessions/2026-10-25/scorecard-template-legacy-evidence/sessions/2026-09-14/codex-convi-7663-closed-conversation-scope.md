# CONVI-7663 Closed Conversations-only implementation

## Context

- Date: 2026-09-14 (America/Toronto)
- Source worktree: `/Users/xuanyu.wang/repos/director-convi-7663`
- Branch: `xw/convi-7663-usecase-scope`

## Scope decision

The initial worktree patch covered every known consumer of `useGetScorecardTemplatePermissions`. It was intentionally narrowed to the call sites mounted by the Closed Conversations scoring page, the dominant incident surface.

Retained product changes:

- `ScorecardFormLoader` scopes its permission request to the conversation use case and skips it until that use case is known.
- `useConversationCoachingDetails` uses the same conversation-use-case scope as its scorecard/template requests and skips the permission request while scope is unavailable.
- `ScorecardForm` receives the conversation permission scope from its loader.
- The Closed Conversations `ScorecardTab` passes its conversation scope to `ResetButton`; other `ResetButton` consumers preserve their previous behavior.
- `useGetScorecardTemplatePermissions` accepts a `skip` argument to support the transient loading state without issuing an all-use-case request.

Removed from this patch:

- reusable scorecard filters and level selectors;
- Coaching Hub agent scorecards;
- admin template management;
- AI Agent template attachment;
- Opera template and backfill flows.

## Validation

- `yarn lint:precommit`: passed.
- `yarn tsc`: passed.
- Focused `useGetScorecardTemplatePermissions` tests: 2 passed, covering bounded use-case forwarding and skip-before-scope behavior.
- Commit: `49e6af05e5` (`[CONVI-7663] Scope conversation template permission requests`).
- Draft PR: [cresta/director#22809](https://github.com/cresta/director/pull/22809).
- The PR uses `docs/pull_request_template.md` unchanged outside the description; the description records both Closed Conversations call paths.
- Review follow-up `ecb8f70c18` makes skipped permission lookups fail closed while preserving the permissive fallback for ordinary loading/error states. Focused coverage now has 3 passing tests.
- Review follow-up `1efbdb7431` separates omitted scope from an explicitly pending conversation scope. Omitted `usecaseNames` again performs the established unscoped lookup; only Closed Conversations opts into skipping while its use case is unavailable. Lint, TypeScript, and 5 focused tests pass.

## Remaining validation

- Verify an RCG Closed Conversations detail page emits only scoped permission requests and stays below the gRPC response limit.
- Other product surfaces still have unscoped permission requests and are explicitly outside this quick patch.
