# CONVI-7281: Add Training Simulator applicability to Opera rules

**Status:** active
**Primary domain:** `training-simulator`
**Primary subdomain:** `evaluation`
**Official ticket:** [CONVI-7281](https://linear.app/cresta/issue/CONVI-7281/add-training-simulator-only-option-for-new-opera-rule-creation)
**Last updated:** 2026-09-28

## Objective and Impact

- **Objective:** Let an Opera rule author choose whether a rule applies to all conversations or only Training Simulator conversations.
- **Customer/system impact:** Training-only evaluation criteria can be kept out of normal conversations while unconstrained rules continue evaluating every conversation source. The setting must affect Opera policy selection before annotations are generated; a Finalize-step selector alone would not satisfy the behavior.
- **Role:** revised the contract and implemented the proto/backend selection path; implemented and opened the Director frontend PR; review and rollout remain.

## Scope

**In scope**

- A Training Simulator value in the existing public and DB `Policy.SourceType` enums.
- Persisting that source type in the existing `app.policies.source_type` column.
- Excluding Training Simulator-source policies from ordinary active-policy loading.
- Propagating conversation source to policy search so Training Simulator conversations load ordinary and Training Simulator-only active policies before annotations are generated.
- Opera create/edit Finalize UI, draft round-tripping, defaults, and validation.
- Focused proto, policy persistence/selection, Training Simulator startup, GoWalter, and frontend tests.

**Non-goals**

- Changing Training Simulator scoring, N/A handling, or module criteria storage.
- Reusing `Moment.AppType`; that field controls where moment templates are available in product UI/features, not which conversation traffic a policy evaluates.
- Adding a policy-to-conversation-source applicability contract or a new relational schema column.
- Resolving inactive-criterion warnings (CONVI-7280) or metadata-trigger requirements (CONVI-7438).

## Source Context

- **Repos:** `cresta-proto`, `go-servers`, `director`
- **Worktrees:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7281`, `/Users/xuanyu.wang/repos/cresta-proto-convi-7281-policy-search`, `/Users/xuanyu.wang/repos/go-servers-convi-7281-policy-source`, `/Users/xuanyu.wang/repos/go-servers-convi-7281`, `/Users/xuanyu.wang/repos/director-convi-7281`
- **Branch:** `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`
- **PRs:** merged source enum [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890), source transport [cresta-proto#9978](https://github.com/cresta/cresta-proto/pull/9978), merged foundation [go-servers#32650](https://github.com/cresta/go-servers/pull/32650), runtime [go-servers#32384](https://github.com/cresta/go-servers/pull/32384), Director frontend [director#23164](https://github.com/cresta/director/pull/23164)

## Current Understanding

### 2026-09-22 explicit-source implementation

The clarified truth table is now implemented in the open PRs. `cresta-proto#9978` carries public `Conversation.Source` through the internal Orchestrator conversation and adds `SearchPoliciesRequest.conversation_source`. Runtime PR #32384 copies the cached source, forwards it to policy search, and applies the active-policy query asymmetrically: ordinary/unspecified conversations exclude `SOURCE_TYPE_TRAINING_SIMULATOR`; Training Simulator conversations include all active source types. Existing filters still run after loading, and explicit overrides retain their existing replacement semantics.

The prior module behavior-to-policy resolver and automatic `policy_override` population were removed from the runtime diff. This prevents a training run from dropping globally applicable policies. The proto PR must merge and release before the go-servers PR can build in ordinary CI without a local repository override.

### 2026-09-22 Director frontend implementation

The frontend consumes the merged enum through the existing `Policy.sourceType` field and create/update APIs. Current Director `main` at `2ab017e85f` already pins `@cresta/web-client` `2.28.16`, so no dependency bump is required. The merged go-servers foundation PR #32650 maps the public enum to the DB enum and persists it through the existing `app.policies.source_type` column; Director does not save directly to the database.

The Director worktree implements the complete editor/list lifecycle: a shared, feature-gated single-selection `ListSelect` appears in both Finalize variants; `OperaStore` defaults, hydrates, resets, dirty-checks, and draft-saves the selected source; request mapping preserves the selection; legacy/unspecified policies normalize to Coach Builder; and the overview query includes Training Simulator policies so a newly created rule remains visible and editable. The two UI choices are “Quality Management and Training Simulator” (`SOURCE_TYPE_COACH_BUILDER`) and “Training Simulator only” (`SOURCE_TYPE_TRAINING_SIMULATOR`).

The compatibility warning from the earlier `00993a5289` resolver implementation is superseded. At runtime PR head `99d196e76f`, the module resolver and automatic policy override were removed. Conversation source is propagated to policy search instead: ordinary/unspecified conversations exclude Training Simulator-only policies, while Training Simulator conversations load all active sources. The earlier failure mode applied specifically because a non-empty override replaced normal active-policy loading; it does not apply to the current architecture.

### 2026-09-21 Opera expert review revision

Tianxin's review on [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890) rejects the current resource coupling in all three proposed contract locations:

1. classify the policy with its existing `Policy.SourceType` instead of adding `Conversation.Source` to `PolicyConfig`;
2. use `policy_override` to select Training Simulator policies in Orchestrator instead of propagating conversation source;
3. do not add conversation source to `PolicyConfigFilter`, because policy and conversation are separate resources.

The code supports that direction but exposes an integration gap. `Policy.SourceType` is already persisted in `app.policies.source_type`, mapped explicitly between public and DB enums, and filterable in `ListPolicies`. `SearchPolicies` uses exact policy IDs when `policy_override` is present. Ordinary `SearchPolicies`, however, currently loads every active policy without a source-type restriction, so adding the enum alone would make Training Simulator policies run on normal conversations.

Training Simulator startup currently sends lesson/module/scenario IDs as voice platform metadata. GoWalter recognizes those fields, creates a `TRAINING_SIMULATOR` conversation, and does not set a policy override. A module stores criterion `behavior_id` values, not policy IDs. Director can associate a selected behavior with a policy while authoring, and the CONVI-7438 prerequisite contract demonstrates server-side behavior-to-policy resolution, but the current run path does not expose all criterion policy IDs or carry them into `StartConversation`.

The revised plan is therefore:

1. Replace the three fields in proto PR #9890 with `Policy.SourceType.SOURCE_TYPE_TRAINING_SIMULATOR` only.
2. Add the matching DB enum value and public-to-DB converter mappings in go-servers; the public and DB numeric values need not match because the DB enum already has a backend-only value.
3. Make ordinary active-policy loading exclude Training Simulator-source policies. Preserve the explicit-override path so it can load those policies by ID.
4. Add a server-authoritative resolver from every module criterion `behavior_id` to its owning policy ID. Reuse the behavior-to-policy relationship represented by the CONVI-7438 prerequisite contract rather than relying on Director's authoring cache.
5. In `ConversationService.StartConversation`, resolve the module's policies from `training_module_id` and populate `Conversation.PolicyOverride` before persistence and Orchestrator dispatch. This keeps the resolver in apiserver, which already has `AppsDB`, and avoids a new GoWalter RPC or database dependency.
6. Map the Finalize UI states to policy source types: “all conversations” remains the ordinary Coach Builder source; “Training Simulator only” writes the new Training Simulator source.

The earlier conversation-source implementation in PRs #9890/#32384 is not the merge direction and must be revised rather than incrementally extended.

### 2026-09-21 Opera owner confirmation

Xuanyu summarized the replacement design to Tianxin: add `SOURCE_TYPE_TRAINING_SIMULATOR`, exclude that source type from the policy query used by normal conversations, resolve the Training Simulator policy IDs for a training run, and pass them through `policy_override`. Tianxin confirmed that this plan makes sense.

This closes the architectural question raised by the PR review. The implementation uses an apiserver-local shared resolver called from `ConversationService.StartConversation`.

### 2026-09-16 runtime-selection refresh (historical; contract superseded)

The runtime trace below remains useful, but its conversation-source contract conclusion was superseded by the 2026-09-21 Opera review above.

Current upstream snapshots still contain no policy conversation-source applicability contract or filter: `go-servers/origin/main` at `e4c0944dfd` (2026-09-16), `cresta-proto/origin/main` at `182306a5ea` (2026-09-15), and `director/origin/main` at `3a01637438` (2026-09-15). The runtime trace remains:

1. Director supplies Training Simulator identity metadata to the voice run.
2. GoWalter recognizes the metadata and starts a conversation with `Conversation.Source = TRAINING_SIMULATOR`.
3. Conversation creation assigns/persists a normal use case when the caller did not supply one.
4. Orchestrator's internal `Conversation` currently carries use-case ID, agent, language, time, and traffic type, but not conversation source.
5. `orchestrator/internal/persist/base_client.go` builds `SearchPoliciesRequest.PolicyConfigFilter` without conversation source.
6. `SearchPolicies` loads all active profile policies and `filterDBPolicies` filters only by use case, time, language, transferred-message handling, and real-time/post-call availability (plus audience later in the search path).
7. Opera generates behavior moment annotations for selected policies; `EvaluateTrainingConversation` does not execute rules itself and only reads annotations matching the module criteria's `behavior_id` values.

This confirms that no direct conversation-to-rule binding is the right enforcement point. The policy needs persisted applicability and `SearchPolicies` needs the conversation source before Opera runs. Filtering only in `EvaluateTrainingConversation` would be too late because unwanted annotations would already exist.

Current GoWalter also sets `StartConversationRequest.FeatureOverride.force_metadata_matches` for Training Simulator conversations. That intentionally makes metadata-gated rules evaluable on synthetic training traffic, but it further increases the need for explicit conversation-source applicability: use-case/audience filtering alone is not a product boundary.

### 2026-09-15 frontend refresh (historical; contract superseded)

The UI route and component findings remain useful, but the conversation-source representation below is no longer the planned contract.

The live ticket still explicitly places the control in step **4. Finalize**. On current `director/origin/main` (`0c5c9adb38`, 2026-09-14), the creation path is `OverviewPage.handleAddNewPolicy` -> `ABS_ROUTES.OPERA.RULE_CREATE` (`rule/create`) -> `CoachBuilderRouter` -> `CreatePolicy` -> shared `CreateAndEditPolicy variant="create"`. `CreateAndEditPolicy` renders the Finalize tab through either `ReviewPolicySection` or `ReviewPolicySectionWithPerformanceConfig`, selected by `enablePerformanceConfigSelection`. A new Finalize field therefore needs a shared component rendered in both variants, or a consolidation that prevents the feature-flag branch from producing different settings.

The closest visual and interaction precedent is Data Management's `ApplicableAreasSelect`: a `ListSelect` multi-select with `showSelectAll`, `hideSearchBar`, target-width popover, custom option descriptions, and “All areas” / “No areas” display copy. It should be copied as an interaction pattern, not reused directly, because it is coupled to React Hook Form field `applicableAreas`, `MomentAppType`, Data Management translations, and metadata-specific options. Within the Opera Finalize code, `selectedUsecases` is the closest state-flow precedent: a controlled value is updated through `OperaStore.updateState`, followed by `emitDraftSave`; however, `UsecasesPicker` itself is a larger chip-and-popover control and is unnecessary for two product-area options.

Current Director and `cresta-proto/origin/main` have no policy-level conversation-source applicability field. UI wiring will therefore require an additive contract before generated web-client types can carry the selection. The existing store/save/hydration chain remains `OperaStore` -> `getOperaPolicyFromState` -> `prepareToPolicy` for save, and `prepareToOperaPolicy` -> `CreateAndEditPolicy` hydration for edit/draft state.

This is a cross-repo contract and runtime-selection change, not a frontend-only option. Reuse `Conversation.Source` and add repeated `applicable_conversation_sources` to both the public and DB `PolicyConfig` protos. Empty means all conversation sources for backward compatibility; a non-empty list uses OR matching. Pass the conversation source through the orchestrator conversation and `PolicyConfigFilter`, then enforce the match in `filterDBPolicies`. Persist the field through existing JSONB converters; no SQL migration is required.

In Director, hydrate an empty source list as unconstrained/all, render the Finalize selector using the Data Management `ListSelect` interaction pattern, and round-trip either `[]` (all conversations) or `[TRAINING_SIMULATOR]` (Training Simulator only) through policy drafts/create/update. Gate exposure using the Training Simulator customer feature flag if product confirms non-enabled customers should not see the row, while keeping API/runtime support unconditional.

## Findings and Decisions

- Superseded on 2026-09-21: the conversation-source applicability design was rejected in Opera-domain review. See `decisions/2026-09-16-policy-applicable-conversation-sources.md`.
- Revised representation: ordinary/all-conversation Opera rules retain `SOURCE_TYPE_COACH_BUILDER`; Training Simulator-only rules use a new `SOURCE_TYPE_TRAINING_SIMULATOR`. “Quality Management only” remains outside ticket scope.
- Runtime enforcement is asymmetric: ordinary active loading excludes Training Simulator-source policies; Training Simulator active loading includes all source types.
- Tianxin confirmed this replacement architecture in an offline discussion on 2026-09-21.

- As of the original 2026-08-10 investigation, the ticket had no comments, blockers, customer needs, or release assignment. Its parent is [CONVI-7362](https://linear.app/cresta/issue/CONVI-7362/training-simulator-opera-master-ticket).
- Live Linear refresh on 2026-09-15: the ticket is Todo, assigned to Xuanyu Wang, has one comment requesting Phoebe Wang to add design, has no customer needs or releases, and is related to COA-2918 (“Design organization for Opera rules by source”). The description still calls for “Applicable product areas” in step 4 Finalize and references the Opera 3.0 settings design.
- Earlier investigation interpreted surrounding discussion as three states, but the ticket contract is authoritative here: all conversations or Training Simulator only. Target-agent matching still uses the human trainee and remains independent of source applicability.
- Training Simulator evaluation consumes Opera-produced moment annotations; it does not independently execute rule logic. Filtering must happen when orchestrator loads policies, before policy-engine evaluation.
- `Policy.SourceType` is already a relational field on `app.policies`, with public/DB enum conversion and `ListPolicies` filtering. No new column or policy-config JSON field is needed.
- Orchestrator now needs an explicit top-level conversation-source field on `SearchPoliciesRequest`; it remains outside `PolicyConfigFilter` because it selects the active policy population rather than matching policy configuration.
- Existing `Moment.AppType` / Data Management “Applicable product areas” is useful UI prior art only. `APP_TYPE_COACHBUILDER` identifies Opera availability and cannot distinguish production from Training Simulator traffic.
- Training Simulator currently builds evaluation choices from Opera policies through `useAutoQATriggers`. Under the two-state contract, both all-conversation and Training-only rules are valid Training Simulator criteria, so no additional criterion-picker filtering is required for this ticket.
- Existing Coach Builder policies remain ordinary/all-conversation rules, so no legacy-row migration is needed.
- Explicit policy overrides remain supported but are not used to implement Training Simulator applicability because they replace the active policy population.
- Superseded on 2026-09-22: exact-ID `policy_override` cannot implement the clarified applicability truth table because it replaces ordinary active-policy loading and bypasses parts of normal filtering. A Training Simulator conversation must include globally applicable active policies as well as Training Simulator-only policies.

## Blockers and Dependencies

- Runtime PR #32384 depends on source-transport [cresta-proto#9978](https://github.com/cresta/cresta-proto/pull/9978) merging and releasing before ordinary go-servers CI can compile it without a local override.
- Staging integration must verify ordinary exclusion and Training Simulator inclusion before rollout.

## Validation and Rollout

- Proto: lint/build internal Orchestrator and policy packages; verify additive source fields and generated clients.
- Backend: source-type converter round trips; cached source reaches policy search; ordinary active loading excludes Training Simulator policies; Training Simulator active loading includes ordinary and Training Simulator policies; explicit overrides retain existing behavior.
- Startup: Training Simulator source survives cache conversion and reaches policy search before the conversation configuration is cached.
- Frontend: store/default/hydration tests, policy request mapping tests, Finalize selector interaction/validation tests, and create/edit draft round-trip.
- Integration: create one ordinary rule and one Training Simulator-only rule, run one normal and one Training Simulator conversation, and assert normal traffic selects only the ordinary rule while training traffic selects both.
- Rollout should be additive and migration-free. Monitor policy search counts and missing-annotation/N/A rates for Training Simulator after enablement.

## Next Actions

1. Merge and release source-transport [cresta-proto#9978](https://github.com/cresta/cresta-proto/pull/9978), then refresh and run ordinary CI for [go-servers#32384](https://github.com/cresta/go-servers/pull/32384).
2. Review draft Director PR [#23164](https://github.com/cresta/director/pull/23164), add the required preview/QA metadata, and move it out of draft when ready.
3. Run the normal-vs-training staging integration matrix before rollout.

## Timeline

- 2026-09-28 — Addressed Director review feedback: preserve specified source types and AI Agent requirement read-only behavior, include Training Simulator policies in active/inactive metadata queries for role visibility enforcement, and update the tracked store-property expectation. Pushed `b8443e9e5a`; 36 focused tests, app/API TypeScript, targeted lint, formatting, and diff checks passed. GitHub reports [director#23164](https://github.com/cresta/director/pull/23164) mergeable with CI running. Evidence: `sessions/2026-09-28/codex-convi-7281-director-review-fixes.md`, `log/2026-09-28.md`.

- 2026-09-25 — Resolved a second Director current-main conflict, limited to the Finalize import block in `ReviewPolicySection.tsx`, preserving both the Training Simulator selector and upstream Deputy Agent compatibility logic. Pushed merge commit `5fcb473690`; 35 focused tests, typed ESLint, Biome, and the Director app TypeScript build passed, and GitHub reports [director#23164](https://github.com/cresta/director/pull/23164) mergeable. Evidence: `sessions/2026-09-25/codex-convi-7281-director-conflict-resolution.md`, `log/2026-09-25.md`.

- 2026-09-25 — Merged current Director `main` into [director#23164](https://github.com/cresta/director/pull/23164), resolved the two additive Opera utility-test conflicts, and pushed merge commit `8fd7d0ed5f` without rebasing published history. Focused tests (25), Biome, targeted typed ESLint, formatting, and diff checks passed; GitHub reports the PR as mergeable. Full typecheck and broad lint are blocked by unrelated current-main issues. Evidence: `sessions/2026-09-25/codex-convi-7281-director-conflict-resolution.md`, `log/2026-09-25.md`.

- 2026-09-23 — Rebased the Director change onto current `main`, committed it as `d3639ac4a3`, pushed via HTTPS, and opened draft [director#23164](https://github.com/cresta/director/pull/23164) using the repository template with only the Description filled. Post-rebase focused tests (23), TypeScript, commit hooks, and diff checks passed; CI started pending. Evidence: `sessions/2026-09-23/codex-convi-7281-frontend-design-alignment.md`, `log/2026-09-23.md`.

- 2026-09-23 — Aligned the Director selector with the supplied design: section and field labels, exact option titles/descriptions, icons, spacing, typography, and responsive width. Source-type persistence behavior is unchanged. Focused tests (23), TypeScript, precommit lint, and diff checks passed. Evidence: `sessions/2026-09-23/codex-convi-7281-frontend-design-alignment.md`, `log/2026-09-23.md`.

- 2026-09-22 — Implemented the Director frontend on `/Users/xuanyu.wang/repos/director-convi-7281`: added the feature-gated selector to both Finalize variants, full store/edit/draft/save round-tripping, legacy normalization, and overview visibility. Director already pins web-client `2.28.16`. Focused tests (23), TypeScript, precommit lint, and diff checks passed. Evidence: `sessions/2026-09-22/codex-convi-7281-frontend-implementation.md`, `log/2026-09-22.md`.

- 2026-09-22 — Opened source-transport [cresta-proto#9978](https://github.com/cresta/cresta-proto/pull/9978) and revised runtime [go-servers#32384](https://github.com/cresta/go-servers/pull/32384) to propagate cached conversation source into policy search. Ordinary traffic excludes Training Simulator-only policies; Training Simulator traffic loads all active policies. Removed module-derived overrides and resolver code. Local-override proto/runtime validation passed at `cf75a85fbe` and `99d196e76f`; the proto dependency allowlist explicitly permits the new exact Orchestrator import. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.

- 2026-09-22 — The frontend-readiness snapshot found a compatibility issue in the then-current module resolver and override architecture. That finding was valid for head `00993a5289` but was superseded when runtime PR #32384 removed the resolver/automatic override and moved source-aware selection into policy search. Evidence: `sessions/2026-09-22/codex-convi-7281-frontend-readiness.md`, `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`.

- 2026-08-10 — Investigated Linear, current domain knowledge, authoritative repo snapshots, Data Management prior art, policy persistence/filtering, orchestrator source propagation, and the July Opera/Training Simulator incident thread; produced a cross-repo implementation plan. Evidence: `sessions/2026-08-10/codex-convi-7281-plan.md`, `log/2026-08-10.md`.
- 2026-09-15 — Refreshed the live Linear ticket and current Director/cresta-proto code. Confirmed the creation route, shared four-step editor, two feature-flagged Finalize implementations, Data Management `ListSelect` prior art, Opera store/draft flow, and absence of a current policy product-area contract. Evidence: `sessions/2026-09-15/codex-convi-7281-creation-dropdown.md`, `log/2026-09-15.md`.
- 2026-09-16 — Revalidated current upstream proto, policy-selection, GoWalter conversation creation, Training Simulator evaluation, and criterion-picker paths. Confirmed the backend requirement and recorded the exact enforcement boundary. Evidence: `sessions/2026-09-16/codex-convi-7281-runtime-applicability.md`, `log/2026-09-16.md`.
- 2026-09-16 — Implemented and opened the additive proto and backend PRs. Added persistence conversion, source propagation, ordinary-selection filtering, override compatibility, and focused tests. Evidence: [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890), [go-servers#32384](https://github.com/cresta/go-servers/pull/32384).
- 2026-09-21 — Read Opera-domain review on proto PR #9890 and superseded the conversation-source applicability design. Traced the existing policy source-type persistence, exact-ID override loading, Director voice startup, GoWalter conversation creation, and the remaining behavior-to-policy resolution gap. Evidence: `sessions/2026-09-21/codex-convi-7281-opera-review.md`, `log/2026-09-21.md`.
- 2026-09-21 — Rewrote proto PR #9890 and backend PR #32384 around the confirmed source-type plus explicit-override architecture. Implemented the apiserver-local shared resolver and `StartConversation` integration; focused Go/Bazel validation passed and backend CI started. Evidence: `sessions/2026-09-21/codex-convi-7281-opera-review.md`, `log/2026-09-21.md`.
- 2026-09-22 — Consumed merged proto `v2.28.9` by rebasing backend PR #32384 onto current main, fixed existing-conversation retry idempotency by resolving policies only for new rows, added a deleted-module retry regression, squashed to commit `7bc9145d9e`, and reran focused Go/Bazel validation without a local proto override. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.
- 2026-09-22 — Split the backend into foundation PR #32650 (DB enum, generated DB proto, bidirectional converters, converter test) and stacked runtime PR #32384 (ordinary exclusion, module policy resolution, conversation override/retry behavior). Foundation commit `71bbb48b96` and runtime commit `373104c4a4` have non-overlapping four-file and eight-file diffs; local validation passed and both CI runs started. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.
- 2026-09-22 — Foundation PR #32650 merged as `bf663d5467`. Restacked runtime PR #32384 onto current `main`, restoring its eight-file runtime-only diff. Fixed its CI-only loader-test failure by asserting policy ID and source type instead of exact PostgreSQL timestamp nanoseconds; focused Go validation passed and CI restarted at commit `7f4e89b406`. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.
- 2026-09-22 — Made `ResolvePolicyIDsForModule` revision-aware: exact revision IDs select that module row, while exported `WildcardRevisionID = "*"` selects the latest by creation time with a deterministic revision-ID tie-breaker. The existing conversation path passes the wildcard. Focused Go tests and both resolver/conversation Bazel suites passed; runtime PR #32384 was updated at `82f9aa8930`. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.
- 2026-09-22 — Simplified `moduleBehaviorIDs` to one behavior-ID map and sorted its keys before policy resolution, avoiding a parallel `seen` set while preserving deterministic overrides. Focused resolver tests passed; runtime commit is `00993a5289`. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.
- 2026-09-22 — Clarified the ticket truth table: ordinary policies apply to normal and Training Simulator conversations; Training Simulator-source policies apply only to Training Simulator conversations. Confirmed the current exact-ID override path is exclusive and therefore drops globally applicable policies during simulator runs. Runtime design must carry explicit applicability context into policy search rather than treating simulator policies as a replacement set. Evidence: `sessions/2026-09-22/codex-convi-7281-backend-refresh.md`, `log/2026-09-22.md`.
