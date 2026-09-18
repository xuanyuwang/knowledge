# CONVI-7281 Opera runtime applicability trace

**Date:** 2026-09-16
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** implementation in `/Users/xuanyu.wang/repos/go-servers-convi-7281` and `/Users/xuanyu.wang/repos/cresta-proto-convi-7281`, both on `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`

## Question

Determine whether CONVI-7281 can be a UI-only label or requires persisted backend applicability and runtime filtering so Training Simulator-only Opera rules do not run on production conversations.

## Findings

### A UI-only label is not sufficient

- Public `Policy`/`PolicyConfig`, the SQL-schema `PolicyConfig` mirror, and `PolicyConfigFilter` have no product-area field.
- `app.policies.policy_config` is JSONB encoded from the SQL-schema `PolicyConfig`, so an additive repeated enum can persist without a relational schema change.
- Policy drafts and histories store full `v1.policy.Policy` payloads, so the same public field naturally survives draft/history serialization once Director round-trips it.

### Where Opera rules are applied

1. GoWalter detects Training Simulator metadata and starts the conversation with source `TRAINING_SIMULATOR`.
2. Conversation creation assigns a normal use case when none is supplied.
3. Orchestrator loads conversation config by calling `SearchPolicies`.
4. `SearchPolicies` loads all active policies for the profile.
5. `filterDBPolicies` narrows by use case, time, language, transferred-message behavior, and traffic availability; the remaining search path handles audience and policy-dependent data.
6. Opera evaluates the selected configs and persists behavior moment annotations.
7. `EvaluateTrainingConversation` only fetches annotations for criterion behavior IDs and converts them into criterion results. It does not run or select policies.

Therefore the product boundary must be enforced in policy selection before Opera evaluation. Filtering only while scoring would leave unwanted annotations on the conversation and would not protect other consumers.

### Training-specific details

- Current GoWalter sets `force_metadata_matches` for Training Simulator conversations, intentionally making metadata-gated rules usable in synthetic training traffic.
- A Training Module criterion stores only `behavior_id`. Director's criterion picker loads all active/inactive Opera policies and behaviors through `useAutoQATriggers`, then maps behaviors back to policy names.
- Under the ticket's two-state contract, every rule remains valid for Training Simulator authoring: unconstrained rules apply everywhere and constrained rules apply only to Training Simulator. Runtime filtering is still mandatory to keep Training-only rules off normal conversations.

## Superseded initial contract proposal

The initial proposal introduced a separate `PolicyConfig.ProductArea` mapping. It is superseded by the accepted source-based contract below.

## Accepted contract refinement

The backend plan now reuses `cresta.v1.conversation.Conversation.Source` instead of introducing `PolicyConfig.ProductArea`:

- `PolicyConfig.applicable_conversation_sources` is repeated.
- Empty means all sources.
- Non-empty uses OR matching against the conversation source.
- `TRAINING_SIMULATOR`-only is represented by `[TRAINING_SIMULATOR]`.
- Request-side source fields are ordinary proto enum fields: absent and `SOURCE_UNSPECIFIED` both represent a normal conversation; any other value identifies that source.
- The ticket has only two states: empty/all conversations and `[TRAINING_SIMULATOR]`/Training Simulator only. There is no “Quality Management only” state.

High-level backend sequence: add the public/internal policy and orchestrator contracts; mirror and round-trip the field in SQL-schema policy config; propagate source through online and offline conversation builders; add it to `SearchPoliciesRequest`; filter in `filterDBPolicies`; cover compatibility and source/policy matrices; deploy the complete backend path before enabling Director writes.

## Implemented backend decision

Explicit Opera simulator/backtest overrides continue bypassing ordinary selection filters so authors can test a selected rule, while normal online and post-close conversations enforce applicability.

## Implementation and PRs

- [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890) adds the public policy field, request filter, and internal Orchestrator conversation source.
- The Orchestrator package dependency allowlist now explicitly permits `cresta/v1/conversation/`; `mage checkAllProtoDependencies` passes.
- [go-servers#32384](https://github.com/cresta/go-servers/pull/32384) mirrors the persisted JSONB field, round-trips it through converters, propagates source through online/cache/backtest paths, and filters policies before Opera annotation generation.
- Proto validation passed with Gazelle and focused Bazel builds. Focused Go tests passed using locally generated proto types; backend Bazel validation remains dependent on generated artifacts from the proto change.

## Validation matrix

- Policies: legacy/new empty (all conversations) and Training-only.
- Conversations: production and `TRAINING_SIMULATOR`, real-time and post-call where applicable.
- Assert selected policy configs and persisted behavior annotations, not only UI state or generated request snapshots.
- Verify Training Simulator and QM authoring pickers expose only compatible policies.

The cleared GitHub SSH credential selected by `~/.ssh/config` was used for fetch and push operations. A restricted SSH credential was skipped.
