# CONVI-7663 review-comment validation

## Context

- Date: 2026-09-18 (America/Toronto)
- Source worktree: `/Users/xuanyu.wang/repos/director-convi-7663`
- Branch: `xw/convi-7663-usecase-scope`
- PR: [cresta/director#22809](https://github.com/cresta/director/pull/22809)
- Mode: review validation followed by the approved minimal product-code change

## Comment 1: options object for permission hook

The readability concern is valid: four positional arguments include three values whose meaning is not apparent at a call site, and adjacent booleans are easy to transpose. A complete conversion would touch 13 production callers plus tests, including admin, AI Agent, Coaching Hub, filters, and Opera paths intentionally excluded from this ticket.

Recommended action: acknowledge and defer the full options-object migration to a focused follow-up. Do not introduce a dual positional/object API merely to update this PR's callers; that leaves a less coherent API. If the reviewer requires the change here, migrate every caller mechanically and validate the broader surface.

## Comment 2: configurable waiting behavior

The concern about a public `waitForUsecaseBeforeLoadingTemplatePermissions` flag is valid, but always failing closed whenever `conversation.data.usecase` is absent is not safe. The Director `Conversation.usecase` field is optional, and Focus View plus Opera simulator also request conversation coaching details. An absent use case can therefore represent either loading or a legitimate missing value.

The generic hook already has enough state to avoid the configurable flag safely:

- skip template permissions while coaching details are disabled, because their query is disabled and the checker is unused;
- skip while the base conversation is still unavailable, because linked coaching fetching is also disabled;
- after the conversation loads, use its use case when present and retain the default unscoped permission lookup when it is absent.

Recommended action: remove the configurable flag and derive the skip condition from `!options?.loadConversationCoachingDetails || conversation.data === undefined`. Add tests for disabled coaching details, pending conversation data, loaded conversation with a use case, and loaded conversation without a use case.

## Suggested reviewer responses

1. Options object: agree on readability, explain the 13-caller blast radius and propose a dedicated mechanical follow-up to preserve this PR's Closed Conversations scope.
2. Waiting behavior: agree that the flag should not be configurable, but avoid treating an optional/missing use case as perpetual loading; derive skip from whether coaching details and the base conversation are available instead.

## Implementation and validation

- Removed `waitForUsecaseBeforeLoadingTemplatePermissions` from `ConversationDetailsHookOptions` and the Closed Conversations caller.
- `useConversationDetails` now skips template-permission loading only while coaching details are disabled or linked conversation data is unavailable. Once a conversation loads, its use case scopes the request when present; a legitimately missing use case retains the established unscoped lookup.
- Added focused coverage for disabled coaching details, a pending conversation, a loaded conversation with a use case, and a loaded conversation without one.
- Deferred the permission-hook options-object migration because a coherent conversion affects 13 production callers outside this PR's Closed Conversations scope.
- Rebased cleanly onto remote PR branch merge `35e2d577cc`; no conflict resolution or code alteration was required.
- Validation after rebase: `yarn lint:precommit`, `yarn tsc`, and three focused Vitest files all passed (9 tests).
- Pushed commit `12e36f4f75` to PR #22809 without force-pushing.

## CodeRabbit follow-up on loaded conversations without a use case

Review comment `discussion_r4047911671` correctly identifies that a loaded conversation without `Conversation.usecase` causes the permission hook to receive `usecaseNames: undefined` and `skipTemplatePermissions: false`. Because the permission hook's default parameter becomes an empty array, that lookup is unscoped.

The proposed global `|| !conversation.data?.usecase` fix is not safe or complete:

- `conversation.data === undefined` is the pending state. Once `conversation.data` exists without a use case, the optional API field is definitively absent rather than still loading.
- `useConversationDetails` also enables coaching details in Focus View and the Opera simulator. Skipping permissions returns the restrictive checker, so the coaching-details selector removes all templates and scorecards and disables grading actions on those consumers.
- `skipTemplatePermissions` only disables the permission hook's full-template request. `useConversationCoachingDetails` separately issues an agent-filtered `ListCurrentScorecardTemplates` call with the same undefined use-case scope, so the suggestion does not make every template request scoped.

Recommended action: do not apply the committable suggestion globally. If Closed Conversations must fail closed for a loaded conversation with no use case, retain an explicit consumer-specific policy (or move that page's coaching-details load behind an explicit scope) and test that page separately. The product decision should state whether a legitimately unassigned conversation must remain viewable/gradable; the generic hook cannot infer that from the optional field alone.

## Closed Conversations-only correction

After confirming the desired scope, implemented the consumer-specific policy as `requireUsecaseForTemplatePermissions`. Only `ClosedConversations.tsx` enables it. The generic hook continues to permit the established unscoped lookup for loaded conversations without a use case on Focus View, Opera, and other consumers.

The test matrix now proves both policies for the same loaded-without-use-case state: required scope skips permissions; default behavior does not. Lint, TypeScript, and 10 tests across the 3 focused suites passed. Commit `e976584f17` was pushed to PR #22809.
