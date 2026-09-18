# CONVI-7663 review-comment validation

## Context

- Date: 2026-09-18 (America/Toronto)
- Source worktree: `/Users/xuanyu.wang/repos/director-convi-7663`
- Branch: `xw/convi-7663-usecase-scope`
- PR: [cresta/director#22809](https://github.com/cresta/director/pull/22809)
- Mode: review only; no product-code changes

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
