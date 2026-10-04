# Superseded: policy applicability via conversation sources

**Date:** 2026-09-16
**Status:** superseded on 2026-09-21
**Work item:** CONVI-7281

## Original decision

- Reuse `cresta.v1.conversation.Conversation.Source`; do not add a generic product-area enum.
- Add `repeated Conversation.Source applicable_conversation_sources` to policy configuration.
- An empty list means the policy is unconstrained and applies to conversations of every source, preserving existing-policy behavior.
- A non-empty list uses OR semantics: a policy is eligible when the conversation's source equals any configured source.
- Carry source to `SearchPolicies` and enforce this rule before Opera generates annotations.

## Why it was superseded

Opera-domain review on [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890) identified that the proposal coupled two separate resources: policy configuration and conversation source. `Policy` already has a persisted `SourceType`, and `policy_override` already exists to select the exact policies evaluated for simulator traffic.

The replacement direction is:

- add `SOURCE_TYPE_TRAINING_SIMULATOR` to `Policy.SourceType` and its DB mirror;
- store Training Simulator-only rules with that source type;
- keep those policies out of ordinary active-policy loading;
- resolve a module's criterion behavior IDs to policy IDs before a Training Simulator conversation starts and pass those IDs through `policy_override`;
- remove the proposed policy-config applicability field, policy-service conversation-source filter, and Orchestrator conversation-source propagation.

Tianxin confirmed this replacement direction in an offline discussion on 2026-09-21 after Xuanyu explicitly described both the normal-conversation exclusion query and the Training Simulator `policy_override` path.

This replacement still needs one transport decision: the current run starts through Director voice metadata and GoWalter, while the module stores behavior IDs rather than policy IDs. The policy-ID resolver and the handoff into `StartConversation` must be designed before implementation resumes.

## Historical compatibility reasoning

- Existing policies deserialize with an empty list and remain applicable to all sources.
- An absent source and `SOURCE_UNSPECIFIED` both represent a normal conversation; any other enum value identifies that conversation source.
- No relational migration is required because policy configuration is stored in `app.policies.policy_config` JSONB; the public-to-DB mirror converters must preserve the new field.

## Ticket representation

The ticket exposes only two rule states:

- all conversations: `applicable_conversation_sources = []`;
- Training Simulator conversations only: `applicable_conversation_sources = [TRAINING_SIMULATOR]`.

There is no “Quality Management only” state in scope.
