# CONVI-7281 backend refresh after proto release

**Date:** 2026-09-22
**Source repo:** `/Users/xuanyu.wang/repos/go-servers-convi-7281`
**Branch:** `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`
**PR:** [go-servers#32384](https://github.com/cresta/go-servers/pull/32384)

## Trigger

Proto PR #9890 merged and current go-servers main consumed its generated contract as `cresta-proto v2.28.9`. The backend branch still used `v2.27.0`, so CI could not compile `Policy_SOURCE_TYPE_TRAINING_SIMULATOR`.

## Changes

- Rebased onto go-servers `origin/main` at `7c4bbe55dc`, inheriting `cresta-proto v2.28.9`; no dependency files remain in the PR diff.
- Verified the released public enum uses value `5`; retained the independent DB enum value `6` and explicit converter mappings.
- Moved Training Simulator policy resolution into the new-row path inside `insertConversationIfMissing`. Existing conversations now bypass module/policy resolution and reuse their stored policy override.
- Added a regression where an existing Training Simulator conversation references a deleted module and still returns `ALREADY_EXISTS` with its stored override.
- Squashed the superseded conversation-source history into commit `7bc9145d9e` and force-pushed the branch.
- Updated the PR body to reference the merged proto release and current validation rather than a local repository override.

## Validation

- `go test ./apiserver/shared/trainingsimulator`
- targeted policy converter/loader Go tests
- targeted conversation override and retry Go tests
- `bazel test //apiserver/shared/trainingsimulator:trainingsimulator_test`
- `bazel test //apiserver/internal/policy:policy_test`
- `bazel test //apiserver/internal/conversation:conversation_test`
- `bazel run //:gazelle`
- `git diff --check`

All passed without `--override_repository`. Backend CI and code-owner review were pending at handoff.

## Foundation/runtime PR split

The DB enum and converter changes were separated from runtime behavior so infrastructure ownership and review are explicit:

- [go-servers#32650](https://github.com/cresta/go-servers/pull/32650), branch `xw/convi-7281-policy-source-infra`, commit `71bbb48b96`: four files covering the DB enum/generated proto, bidirectional source-type mappings, and converter test.
- [go-servers#32384](https://github.com/cresta/go-servers/pull/32384), branch `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`, commit `373104c4a4`: eight runtime files covering ordinary-policy exclusion, module behavior-to-policy resolution, and conversation override/retry behavior.

PR #32384 now targets the foundation branch. Its existing description and CodeRabbit summary were preserved while adding the dependency and clarifying validation on the complete stacked tree.

Foundation validation passed:

- `go test ./apiserver/internal/policy -run '^TestTrainingSimulatorSourceTypeConversion$'`
- `bazel test //apiserver/internal/policy:policy_test` (10 shards)
- `bazel run //:gazelle`
- `git diff --check`

Both PRs had started CI with no reported failures at the time of the split; code-owner approval remained pending.

## Foundation merge and runtime restack

Foundation PR #32650 merged to `main` as `bf663d5467`. GitHub automatically retargeted runtime PR #32384 to `main`, but the squash merge temporarily made the four foundation files reappear in its comparison. The runtime commit was cherry-picked onto the merged `main`, restoring the intended eight-file diff.

The prior runtime CI run also exposed a test-assertion issue in `TestLoadTrainingSimulatorPolicyFromOverride`: PostgreSQL returned `UpdatedAt` with 128 ns less precision than the in-memory fixture. The loader behavior was correct. The assertion now verifies one result with the expected policy ID and Training Simulator source type rather than requiring all storage timestamps to be byte-identical.

- Focused validation: `go test ./apiserver/internal/policy -run '^TestPolicyLoaderSuite$/^TestLoadTrainingSimulatorPolicyFromOverride$' -count=1`
- Result: passed
- Current runtime commit: `7f4e89b406`
- Current PR diff: eight runtime files; no DB proto or converter files

Runtime CI restarted after the force-push; code-owner approval remains pending.

## Revision-aware module policy resolution

`ResolvePolicyIDsForModule` now accepts a module revision ID. An exact value adds `revision_id = ?`; exported `WildcardRevisionID = "*"` orders matching revisions by `created_at DESC, revision_id DESC` and selects the first. The constant is owned by the shared resolver package because that package defines the wildcard semantics and the conversation package needs a shared non-literal value.

The current `StartConversation` integration still has only `training_module_id` metadata, so it explicitly passes the wildcard and preserves latest-revision behavior. Added resolver coverage proving that an older explicit revision resolves its own criteria while the wildcard resolves the latest revision.

Validation:

- `go test ./apiserver/shared/trainingsimulator -count=1`
- focused `TestStartConversation` policy-override tests
- `bazel test //apiserver/shared/trainingsimulator:trainingsimulator_test //apiserver/internal/conversation:conversation_test`
- `git diff --check`

All passed. Runtime PR #32384 was force-updated to commit `82f9aa8930`; CI restarted.

Follow-up simplification replaced the separate behavior-ID slice and `seen` set with one behavior-ID map. The helper returns sorted map keys so policy overrides remain deterministic despite Go map iteration order. The focused resolver Go test passed; PR #32384 now points to `00993a5289`.

## Applicability truth-table correction

The ticket semantics were clarified after the resolver work:

| Policy classification | Normal conversation | Training Simulator conversation |
| --- | --- | --- |
| Globally applicable (existing non-Training-Simulator source) | include | include |
| Training Simulator-only (`SOURCE_TYPE_TRAINING_SIMULATOR`) | exclude | include |

Current PR #32384 correctly implements the normal-conversation exclusion but does not implement the Training Simulator column correctly. `loadDBPolicies` treats any non-empty `policy_override` as an exclusive replacement set, and downstream filtering treats overrides specially. Populating exact module-derived IDs therefore removes globally applicable policies that were not named in the override and is not equivalent to “load all applicable policies.”

The policy classification itself is sufficient: `SOURCE_TYPE_TRAINING_SIMULATOR` is the restricted case; other active source types remain globally applicable. The missing piece is a request-level Training Simulator applicability signal at the Orchestrator-to-policy-service boundary. That signal should make active loading omit the source exclusion while preserving ordinary use-case, time, language, traffic, and audience filtering. It should not overload exact-ID override semantics.

No runtime code was changed for this correction pending selection of the transport contract.

### Why source knowledge at StartConversation is insufficient

`StartConversation` already knows and persists the public `Conversation.Source`. The later policy-selection path is cross-process: apiserver emits an internal Orchestrator `Conversation`, Orchestrator constructs `SearchPoliciesRequest`, and PolicyService runs the active-policy query. The current internal Orchestrator conversation has no source field, and the search request has no general conversation identity/source field, so PolicyService cannot infer Training Simulator context from the request it receives.

A protobuf change is therefore not logically required, but it is the clean way to preserve the fact across the existing RPC boundaries. No-proto alternatives exist but add hidden coupling: always populate the existing `evaluated_policies_conversation` field and overload it as policy-selection context, transmit a sentinel through `policy_override`, or make Orchestrator/PolicyService perform extra conversation-table lookups. The first misuses a field documented for skip-evaluated behavior, the second conflicts with exclusive override semantics, and the third moves policy-selection context into implicit database coupling.

## StartConversation to Orchestrator policy-loading trace

The runtime path after GoWalter calls `ConversationService.StartConversation` is:

1. A newly inserted conversation is placed in the conversation cache, then `createStartConversationEvent` constructs `CONVERSATION_START` and optional `CONVERSATION_METADATA` events and calls `BatchCreateConversationEvents`.
2. `batchCreateConversationEventsInternal` persists the events and, unless Orchestrator or annotations are disabled, calls `callOrchestratorProcessEvents`.
3. That helper converts the cached conversation to the internal Orchestrator conversation. It copies the stored `policy_override`, but the internal contract does not copy the public conversation source. It then invokes `OrchestratorService.ProcessEvents`.
4. `ProcessEvents` calls `loadConversationData`, whose first operation is `persistClient.GetConversationConfig`. The online client checks its conversation-config cache and otherwise calls `getConversationConfigNonCache`.
5. `getConversationConfigNonCache` constructs `SearchPoliciesRequest` from profile, agent, conversation start time, language, real-time traffic type, transfer state, use case, and any exact policy override. If Opera is enabled, it calls `PolicyService.SearchPolicies`.
6. `SearchPolicies` loads DB policies, filters them, reads their moments/actions/behaviors, and aggregates them into `AiServiceConfig` records returned to Orchestrator and cached as conversation config.
7. Without an override, PR #32384's active-policy loader selects active profile policies while excluding `SOURCE_TYPE_TRAINING_SIMULATOR`. With a non-empty override, loading switches to exact policy IDs/history IDs instead of merging them with active policies.
8. Processing the start event can immediately consume silence configuration. Subsequent message/event/close graphs reuse the loaded `SearchPoliciesResponse`; `updateGraphForOpera` extracts its configs, evaluates policy-engine definitions, and persists/publishes the resulting annotations.

This confirms the missing Training Simulator distinction sits on the internal conversation -> `SearchPoliciesRequest` path. `StartConversation` has the public source, but that source is lost before `SearchPolicies`, while exact override IDs change loading from ordinary active selection to replacement selection.

### Policy-search transport options

Current proto inspection leaves two clean designs and one rejected shortcut:

1. **Carry source explicitly.** Add public `Conversation.Source` to the internal Orchestrator conversation, copy the cached source, then add a top-level source/applicability field to `SearchPoliciesRequest`. Keep it outside `PolicyConfigFilter`, whose documented role is matching policy configuration. This avoids an extra DB read and makes the request self-contained, but changes two proto messages.
2. **Carry conversation identity and resolve source in PolicyService.** The internal Orchestrator conversation already has its resource name, so add an optional `conversation` resource-name field to `SearchPoliciesRequest`. `SearchPolicies` parses the name and reads `chats.source`; absent means ordinary/backward-compatible policy loading. This changes one proto message, but adds a database dependency/read to policy selection. Because the conversation was just inserted, the lookup must use the write/primary DB path or explicitly handle replica lag.
3. **Do not overload `evaluated_policies_conversation`.** That existing field is documented and consumed only when `skip_evaluated_policies` is true. Always populating it as general selection context would avoid a proto change but give one field two unrelated meanings.

Recommendation: explicit source propagation is the cleaner runtime contract. If minimizing contract changes is more important, an explicit top-level conversation resource in `SearchPoliciesRequest` is acceptable when read from the primary DB and covered for absent/deleted/cross-profile conversations. In both designs, normal traffic excludes Training Simulator-source policies; Training Simulator traffic removes that exclusion and retains all ordinary filters.

#### Response post-processing assessment

A proposed no-proto alternative is to let `SearchPolicies` return all policies and post-process `SearchPoliciesResponse` according to conversation source before Orchestrator caches it. This is not a safe narrow filter in the current response contract:

- Orchestrator still needs a no-proto source discriminator. The existing internal conversation has the conversation name but not source; resolving source therefore still requires a DB lookup, overloading an existing field/flag, or hidden transport metadata.
- `SearchPoliciesResponse` contains aggregated `AiServiceConfig` messages, not public policies. `PolicyEngineConfig.Policy` retains `policy_id`, but not `Policy.SourceType`.
- Trigger moment, keyword, composite, LLM-evaluation, silence, metadata, and other definitions are aggregated into separate configs before the response. Those definitions generally do not retain the owning policy ID/source. Removing only the policy-engine entry can leave orphan computation and annotations; complete pruning requires rebuilding the policy dependency graph already handled before `aggregateAIServiceConfigs`.

Therefore response post-processing is technically possible only with broader dependency-aware reconstruction or an expanded response contract, which is more invasive than filtering eligible DB policies before aggregation. The safer selection point remains inside `SearchPolicies` before `preparePolicyDependants`/`aggregateAIServiceConfigs`.

## Credential use

No AWS, Okta, Azure, or SSH credentials were used.

## Explicit-source implementation and PR update

Selected the explicit source transport. Opened [cresta-proto#9978](https://github.com/cresta/cresta-proto/pull/9978) from a fresh worktree with two additive fields: public `Conversation.Source` on the internal Orchestrator conversation and `conversation_source` on `SearchPoliciesRequest`. Proto build, lint, API lint, and diff checks passed; the PR has `semver:minor`.

Updated [go-servers#32384](https://github.com/cresta/go-servers/pull/32384) to remove module-to-policy resolution and automatic `policy_override` population. The cached source is now copied into the internal conversation, forwarded to `SearchPolicies`, and used by active loading: unspecified/non-Training-Simulator sources exclude `SOURCE_TYPE_TRAINING_SIMULATOR`, while Training Simulator includes every active source type. Explicit overrides retain their existing replacement behavior.

Validation used a temporary local Bazel repository override for cresta-proto#9978; it was removed before commit. The policy and persist suites passed. The initial conversation-converter run exposed two expected messages missing their newly propagated nonzero source; after correcting them, the focused conversation suite passed. `git diff --check` passed. Proto dependency lint initially rejected the new Orchestrator import; the package allowlist now explicitly permits the exact conversation proto and `mage -v checkAllProtoDependencies` passes. Proto head is `cf75a85fbe`; go-servers head is `99d196e76f`.

Credential report: pushed through the explicitly cleared GitHub SSH host/key entry. No AWS, Okta, or Azure credential was used. The restricted break-glass SSH credential was not used.
