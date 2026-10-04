# CONVI-7281 Opera expert review and revised plan

**Date:** 2026-09-21
**Source repos:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7281`, `/Users/xuanyu.wang/repos/go-servers-convi-7281`, `/Users/xuanyu.wang/repos/director`
**PRs:** [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890), [go-servers#32384](https://github.com/cresta/go-servers/pull/32384)

## Input reviewed

Tianxin left three Opera-domain comments on proto PR #9890:

- `PolicyConfig.applicable_conversation_sources`: use the policy's existing source type instead of putting conversation source on policy configuration.
- Orchestrator `Conversation.source`: likely use `policy_override` to select Training Simulator policies.
- `PolicyConfigFilter.conversation_source`: do not reference conversation source because policy and conversation are separate resources.

Tinglin approved but explicitly requested Tianxin's review. The comments challenge the core model, so the approval does not make the current design accepted.

## Current behavior traced

- `Policy.SourceType` already exists publicly, is persisted in `app.policies.source_type`, has an independent DB enum, explicit converter maps, and a `ListPolicies` filter.
- Ordinary `SearchPolicies` loads every active policy. It does not currently restrict source type.
- When `policy_override.policy_ids` is non-empty, `SearchPolicies` loads those exact policies and skips the ordinary active-policy loader.
- Director starts a Training Simulator voice run with training session, lesson, module, scenario, and agent metadata. It does not pre-create the conversation or send policy IDs.
- GoWalter recognizes the metadata, sets `Conversation.Source = TRAINING_SIMULATOR`, and calls `StartConversation` without a policy override.
- A module criterion stores `behavior_id`. Director's authoring picker knows the related policy name, and the CONVI-7438 prerequisite contract demonstrates server-side behavior-to-policy lookup, but the live run has no complete policy-ID resolver in its startup path.
- The public conversation contract currently documents policy override as supported for Opera simulator traffic. Training Simulator support must be confirmed and tested.

## Options

| Option | Normal conversations | Training Simulator | Assessment |
|---|---|---|---|
| Conversation-source applicability from current PRs | Filter by `Conversation.Source` | Loads all ordinary plus matching training policies | Rejected by domain review because it couples policy and conversation resources |
| New policy source type plus implicit source matching | Exclude Training Simulator source | Select source types based on conversation source | Still requires conversation identity in policy selection and does not follow the `policy_override` recommendation |
| New policy source type plus explicit override | Exclude Training Simulator source | Resolve module criteria to exact policy IDs and override selection | Recommended; respects policy ownership and evaluates only policies the module needs |

## Revised plan

1. Proto: replace the three current fields with `Policy.SourceType.SOURCE_TYPE_TRAINING_SIMULATOR` only.
2. Persistence: add the corresponding DB enum value and public/DB mappings; reuse `app.policies.source_type`.
3. Ordinary selection: exclude Training Simulator-source policies from active-policy loading so normal conversations cannot evaluate them.
4. Training selection: resolve every module criterion behavior ID to its owning policy ID server-side and provide the exact set through `policy_override` before conversation start.
5. Startup integration: choose the boundary with Opera and Training Simulator owners. Candidate A is a Director preflight RPC whose result GoWalter receives and maps to `Conversation.PolicyOverride`; candidate B is GoWalter-owned server resolution using the module ID. Avoid using Director's authoring cache as the authority.
6. UI: map “all conversations” to the existing Coach Builder source and “Training Simulator only” to the new source; round-trip on create, edit, and draft save.
7. Validation: prove normal exclusion, exact training override selection, missing/inactive criterion references, and end-to-end annotations.

## Implemented boundary

The backend uses the preferred no-RPC design. `ConversationService.StartConversation` recognizes `TRAINING_SIMULATOR`, reads `training_module_id`, and calls `apiserver/shared/trainingsimulator.ResolvePolicyIDsForModule` before persistence and Orchestrator dispatch. A non-empty caller override is preserved.

The resolver loads the latest module revision, deduplicates non-empty criterion behavior IDs, selects active Training Simulator-source policies within the customer/profile, rejects missing or ambiguous behavior mappings, and returns deterministic deduplicated policy IDs. This avoids adding Training Simulator database access to GoWalter or a new startup RPC.

## Opera owner confirmation

In an offline follow-up, Xuanyu proposed the complete replacement behavior to Tianxin:

- add `SOURCE_TYPE_TRAINING_SIMULATOR`;
- update the normal-conversation policy query to exclude that source type;
- resolve Training Simulator policy IDs and populate `policy_override` for Training Simulator conversations.

Tianxin replied that the plan makes sense. This confirms the selection architecture; it does not yet choose the component or RPC responsible for the behavior-to-policy resolution.

## Current API/client check

- GoWalter has no `TrainingSimulatorServiceClient` or Training Simulator proto import. Its training-specific behavior is metadata recognition followed by a direct `ConversationService.StartConversation` call.
- There is no `GetTrainingModule` RPC. `ListTrainingModules` can be narrowed by `training_module_names`, and `ListTrainingLessons` can include nested modules.
- Both list paths return the existing `TrainingModule` resource. Its evaluation criteria contain `behavior_id`, not the owning policy ID.
- `RetrieveTrainingModulePrerequisites` is the only current Training Simulator contract that can return a policy resource name resolved from a criterion behavior. It is not a complete module-policy list: the response contains entries only for metadata prerequisites and is explicitly empty when no criterion needs metadata seeding.
- No `RetrieveTrainingModulePrerequisites` handler exists under the inspected `go-servers/main` Training Simulator implementation, so it is not currently available as the required runtime resolver.

## Resolver placement options

The policy resolver does not inherently require a new RPC. GoWalter is a separate binary and has no `AppsDB` dependency, while `ConversationService.StartConversation` already runs in apiserver and has `AppsDB` access. A root `shared/trainingsimulator` function imported by GoWalter would therefore still require adding database configuration and access to the voice service.

Preferred no-RPC design:

1. Add an apiserver-local resolver, for example under `apiserver/shared/trainingsimulator`.
2. Give it customer/profile/module identity and a database handle.
3. Load the latest module evaluation criteria, deduplicate non-empty behavior IDs, query active policies whose `behavior_ids` overlap, and require each criterion behavior to resolve unambiguously.
4. In `ConversationService.StartConversation`, when the source is `TRAINING_SIMULATOR` and no caller override is already present, extract the training module ID from the training metadata, call the resolver, and populate `Conversation.PolicyOverride` before persistence and Orchestrator dispatch.

Other options are a GoWalter-to-Training-Simulator RPC, a Director preflight plus a new voice transport field, or persisting policy IDs on the module. The RPC preserves service ownership but adds a startup dependency; the browser path adds trust/transport complexity; persisted policy IDs duplicate a relationship that can change when a rule is edited. The apiserver-local resolver is the smallest current change, at the cost of a deliberate Training Simulator hook in the generic conversation-start path.

## Credential use

No AWS, Okta, Azure, or SSH credentials were used. GitHub PR metadata was read through the existing GitHub CLI session.

## Published result

- Proto PR #9890 was rewritten to add only the public Training Simulator policy source enum; proto dependency checks, Gazelle, the policy Bazel build, and `git diff --check` passed.
- Backend PR #32384 now adds the DB enum/mappings, NULL-safe ordinary-policy exclusion, the shared resolver, and Training Simulator start-conversation override population.
- Shared resolver, policy, and conversation Bazel tests passed using a local override to the dependent proto worktree. Targeted Go tests, Gazelle, and `git diff --check` also passed.
- Remaining work is proto release/dependency consumption, Director UI, staging integration, and rollout.
