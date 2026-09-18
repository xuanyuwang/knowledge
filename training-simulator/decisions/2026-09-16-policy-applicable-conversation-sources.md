# Policy applicability uses conversation sources

**Date:** 2026-09-16  
**Status:** implemented in backend PRs  
**Work item:** CONVI-7281

## Decision

- Reuse `cresta.v1.conversation.Conversation.Source`; do not add a generic product-area enum.
- Add `repeated Conversation.Source applicable_conversation_sources` to policy configuration.
- An empty list means the policy is unconstrained and applies to conversations of every source, preserving existing-policy behavior.
- A non-empty list uses OR semantics: a policy is eligible when the conversation's source equals any configured source.
- Carry source to `SearchPolicies` and enforce this rule before Opera generates annotations.

## Compatibility

- Existing policies deserialize with an empty list and remain applicable to all sources.
- An absent source and `SOURCE_UNSPECIFIED` both represent a normal conversation; any other enum value identifies that conversation source.
- No relational migration is required because policy configuration is stored in `app.policies.policy_config` JSONB; the public-to-DB mirror converters must preserve the new field.

## Ticket representation

The ticket exposes only two rule states:

- all conversations: `applicable_conversation_sources = []`;
- Training Simulator conversations only: `applicable_conversation_sources = [TRAINING_SIMULATOR]`.

There is no “Quality Management only” state in scope.
