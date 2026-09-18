# CONVI-7281: Add Training Simulator applicability to Opera rules

**Status:** active
**Primary domain:** `training-simulator`
**Primary subdomain:** `evaluation`
**Official ticket:** [CONVI-7281](https://linear.app/cresta/issue/CONVI-7281/add-training-simulator-only-option-for-new-opera-rule-creation)
**Last updated:** 2026-09-16

## Objective and Impact

- **Objective:** Let an Opera rule author choose whether a rule applies to all conversations or only Training Simulator conversations.
- **Customer/system impact:** Training-only evaluation criteria can be kept out of normal conversations while unconstrained rules continue evaluating every conversation source. The setting must affect Opera policy selection before annotations are generated; a Finalize-step selector alone would not satisfy the behavior.
- **Role:** investigated, designed, and implemented backend changes

## Scope

**In scope**

- A policy-level `applicable_conversation_sources` contract using the existing `Conversation.Source` enum.
- Backward-compatible persistence in the existing policy-config JSONB.
- Runtime policy filtering using `Conversation.Source.TRAINING_SIMULATOR`.
- Opera create/edit Finalize UI, draft round-tripping, defaults, and validation.
- Focused proto, backend, orchestrator, converter, and frontend tests.

**Non-goals**

- Changing Training Simulator scoring, N/A handling, or module criteria storage.
- Reusing `Moment.AppType`; that field controls where moment templates are available in product UI/features, not which conversation traffic a policy evaluates.
- Migrating existing policy rows or adding a relational schema column.
- Resolving inactive-criterion warnings (CONVI-7280) or metadata-trigger requirements (CONVI-7438).

## Source Context

- **Repos:** `cresta-proto`, `go-servers`, `director`
- **Worktrees:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7281`, `/Users/xuanyu.wang/repos/go-servers-convi-7281`
- **Branch:** `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`
- **PRs:** [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890), [go-servers#32384](https://github.com/cresta/go-servers/pull/32384)

## Current Understanding

### 2026-09-16 runtime-selection refresh

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

### 2026-09-15 frontend refresh

The live ticket still explicitly places the control in step **4. Finalize**. On current `director/origin/main` (`0c5c9adb38`, 2026-09-14), the creation path is `OverviewPage.handleAddNewPolicy` -> `ABS_ROUTES.OPERA.RULE_CREATE` (`rule/create`) -> `CoachBuilderRouter` -> `CreatePolicy` -> shared `CreateAndEditPolicy variant="create"`. `CreateAndEditPolicy` renders the Finalize tab through either `ReviewPolicySection` or `ReviewPolicySectionWithPerformanceConfig`, selected by `enablePerformanceConfigSelection`. A new Finalize field therefore needs a shared component rendered in both variants, or a consolidation that prevents the feature-flag branch from producing different settings.

The closest visual and interaction precedent is Data Management's `ApplicableAreasSelect`: a `ListSelect` multi-select with `showSelectAll`, `hideSearchBar`, target-width popover, custom option descriptions, and “All areas” / “No areas” display copy. It should be copied as an interaction pattern, not reused directly, because it is coupled to React Hook Form field `applicableAreas`, `MomentAppType`, Data Management translations, and metadata-specific options. Within the Opera Finalize code, `selectedUsecases` is the closest state-flow precedent: a controlled value is updated through `OperaStore.updateState`, followed by `emitDraftSave`; however, `UsecasesPicker` itself is a larger chip-and-popover control and is unnecessary for two product-area options.

Current Director and `cresta-proto/origin/main` have no policy-level conversation-source applicability field. UI wiring will therefore require an additive contract before generated web-client types can carry the selection. The existing store/save/hydration chain remains `OperaStore` -> `getOperaPolicyFromState` -> `prepareToPolicy` for save, and `prepareToOperaPolicy` -> `CreateAndEditPolicy` hydration for edit/draft state.

This is a cross-repo contract and runtime-selection change, not a frontend-only option. Reuse `Conversation.Source` and add repeated `applicable_conversation_sources` to both the public and DB `PolicyConfig` protos. Empty means all conversation sources for backward compatibility; a non-empty list uses OR matching. Pass the conversation source through the orchestrator conversation and `PolicyConfigFilter`, then enforce the match in `filterDBPolicies`. Persist the field through existing JSONB converters; no SQL migration is required.

In Director, hydrate an empty source list as unconstrained/all, render the Finalize selector using the Data Management `ListSelect` interaction pattern, and round-trip either `[]` (all conversations) or `[TRAINING_SIMULATOR]` (Training Simulator only) through policy drafts/create/update. Gate exposure using the Training Simulator customer feature flag if product confirms non-enabled customers should not see the row, while keeping API/runtime support unconditional.

## Findings and Decisions

- Accepted on 2026-09-16: reuse `Conversation.Source` rather than create a policy product-area enum. `applicable_conversation_sources = []` means all sources; otherwise a policy matches when the conversation source equals any configured source. See `decisions/2026-09-16-policy-applicable-conversation-sources.md`.
- The request-side source is an ordinary enum field: absent and `SOURCE_UNSPECIFIED` both represent a normal conversation; any other value identifies that source.
- Ticket scope has exactly two states: `[]` for all conversations and `[TRAINING_SIMULATOR]` for Training Simulator only. “Quality Management only” is not a supported state.

- As of the original 2026-08-10 investigation, the ticket had no comments, blockers, customer needs, or release assignment. Its parent is [CONVI-7362](https://linear.app/cresta/issue/CONVI-7362/training-simulator-opera-master-ticket).
- Live Linear refresh on 2026-09-15: the ticket is Todo, assigned to Xuanyu Wang, has one comment requesting Phoebe Wang to add design, has no customer needs or releases, and is related to COA-2918 (“Design organization for Opera rules by source”). The description still calls for “Applicable product areas” in step 4 Finalize and references the Opera 3.0 settings design.
- Earlier investigation interpreted surrounding discussion as three states, but the ticket contract is authoritative here: all conversations or Training Simulator only. Target-agent matching still uses the human trainee and remains independent of source applicability.
- Training Simulator evaluation consumes Opera-produced moment annotations; it does not independently execute rule logic. Filtering must happen when orchestrator loads policies, before policy-engine evaluation.
- `PolicyConfig` is already persisted as protobuf JSON in `app.policies.policy_config`; adding a repeated enum is forward/backward compatible when empty means all.
- `orchestrator/internal/persist/base_client.go` builds `SearchPoliciesRequest.PolicyConfigFilter`, but the orchestrator `Conversation` currently lacks source. Both online conversion (`shared/conversation/orchestrator_converter.go`) and offline/backtest request construction (`temporal/ai_services/backtest/shared/execute_conversation.go`) need to propagate source.
- The backend has a source-lookup precedent in `orchestrator/shared/redaction_requirement.go`, but carrying source on the orchestrator conversation is preferred here: online and offline builders already possess the source, and policy loading should not add a database lookup.
- Existing `Moment.AppType` / Data Management “Applicable product areas” is useful UI prior art only. `APP_TYPE_COACHBUILDER` identifies Opera availability and cannot distinguish production from Training Simulator traffic.
- Training Simulator currently builds evaluation choices from Opera policies through `useAutoQATriggers`. Under the two-state contract, both all-conversation and Training-only rules are valid Training Simulator criteria, so no additional criterion-picker filtering is required for this ticket.
- Accepted default: API/DB empty means unconstrained/all conversation sources for legacy compatibility.
- Accepted filtering: policies with empty applicability apply to every conversation; otherwise the request source must match any configured source.
- Explicit policy overrides used by Opera simulation/backtests keep existing behavior and bypass conversation-source restrictions, so authors can test a rule outside ordinary traffic selection.

## Blockers and Dependencies

- Product confirmation is needed on whether the Finalize row should be hidden behind `enableTrainingSimulator` and whether edit flows are formally in scope. Engineering recommendation: gate the row for non-entitled customers but support create and edit for consistency.
- Proto must land and generated Go/web-client dependencies must be consumed before backend/frontend changes compile.

## Validation and Rollout

- Proto: lint/build `cresta/v1/policy`, `cresta/nonpublic/orchestrator`, and generated dependency checks.
- Backend: converter round-trip tests; `filterDBPolicies` matrix for empty/all and Training-only policies with absent, normal, and Training Simulator request sources; orchestrator request mapping; online/cache and offline/backtest propagation tests.
- Frontend: store/default/hydration tests, policy request mapping tests, Finalize selector interaction/validation tests, and create/edit draft round-trip.
- Integration: create one all-conversations rule and one Training-only rule, run one normal and one Training Simulator conversation, and assert only the expected policy/behavior annotations exist. Verify a legacy rule with no field still evaluates both conversations.
- Rollout should be additive and migration-free. Monitor policy search counts and missing-annotation/N/A rates for Training Simulator after enablement.

## Next Actions

1. Review and merge [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890), then consume its generated artifacts in [go-servers#32384](https://github.com/cresta/go-servers/pull/32384).
2. Update `director` Finalize UI and state/request round-trip after the generated web client contains the new field.
3. Run the production-vs-training integration matrix before rollout.

## Timeline

- 2026-08-10 — Investigated Linear, current domain knowledge, authoritative repo snapshots, Data Management prior art, policy persistence/filtering, orchestrator source propagation, and the July Opera/Training Simulator incident thread; produced a cross-repo implementation plan. Evidence: `sessions/2026-08-10/codex-convi-7281-plan.md`, `log/2026-08-10.md`.
- 2026-09-15 — Refreshed the live Linear ticket and current Director/cresta-proto code. Confirmed the creation route, shared four-step editor, two feature-flagged Finalize implementations, Data Management `ListSelect` prior art, Opera store/draft flow, and absence of a current policy product-area contract. Evidence: `sessions/2026-09-15/codex-convi-7281-creation-dropdown.md`, `log/2026-09-15.md`.
- 2026-09-16 — Revalidated current upstream proto, policy-selection, GoWalter conversation creation, Training Simulator evaluation, and criterion-picker paths. Confirmed the backend requirement and recorded the exact enforcement boundary. Evidence: `sessions/2026-09-16/codex-convi-7281-runtime-applicability.md`, `log/2026-09-16.md`.
- 2026-09-16 — Implemented and opened the additive proto and backend PRs. Added persistence conversion, source propagation, ordinary-selection filtering, override compatibility, and focused tests. Evidence: [cresta-proto#9890](https://github.com/cresta/cresta-proto/pull/9890), [go-servers#32384](https://github.com/cresta/go-servers/pull/32384).
