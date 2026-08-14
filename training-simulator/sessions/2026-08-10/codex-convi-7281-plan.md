# Codex Session: CONVI-7281 investigation and plan

**Date:** 2026-08-10
**Tool:** Cursor / Codex
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** `go-servers` main checkout (`origin/main` 875b210e, 2026-08-07); related read-only snapshots: `director/origin/main` d4711230 (2026-08-06), `cresta-proto/origin/main` 600b0b5e (2026-07-30)

## Objective

Investigate [CONVI-7281](https://linear.app/cresta/issue/CONVI-7281/add-training-simulator-only-option-for-new-opera-rule-creation) against the Training Simulator domain and produce an implementation-ready plan for Opera rules that apply to Quality Management, Training Simulator, or both.

## Inputs reviewed

- Training Simulator domain `project.yaml`, parent README, and `training-content`, `simulation-runtime`, and `evaluation` subdomain references.
- Linear ticket CONVI-7281, parent CONVI-7362, and sibling tickets CONVI-7280, CONVI-7346, and CONVI-7438. CONVI-7281 had no comments, customer needs, release, blockers, or related issues.
- [Slack thread in #proj-conversation-intelligence](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1784585156708259), July 20–28, 2026. This is recent, direct implementation/product evidence (high confidence): Krystal stated the product plan to mark rules as applicable to QM, Training Simulator, or both; Jack confirmed target-agent constraints should use the human trainee after the source/agent fix.
- Director Opera create/edit wizard and Data Management applicability prior art.
- Public and DB policy protos, policy JSON converters, policy filtering, orchestrator policy-loading path, online cache-to-orchestrator conversion, and offline/backtest request construction.
- Current repo state: Director local `main` was 1,109 commits behind `origin/main` and had an unrelated modified Insights file. Searches that informed the plan used `origin/main`; no source files were edited.
- Glean MCP was unavailable in this session, so no new Glean documents were presented. Existing vetted domain documentation and current code were sufficient for the plan.

## Evidence map

### Product and domain

- `knowledge/training-simulator/subdomains/evaluation/README.md`: Training Simulator evaluation consumes Opera-generated moment annotations and scores them; it does not own policy matching.
- Slack reply by Krystal Truong, 2026-07-20: “add a field in Opera to mark as applicable to QM or Training Sim or both.”
- Slack replies by Jack Jee, 2026-07-21 and 2026-07-28: the conversation's agent should be the trainee; Opera target-agent configuration should target human agents, not the simulator VA.

### Frontend

- `director/packages/director-app/src/features/opera/components/policy/CreateAndEditPolicy.tsx`: hydrates/reset Zustand state and renders step 4 through `ReviewPolicySection`.
- `director/packages/director-app/src/features/opera/components/policy/ReviewPolicySection.tsx`: Finalize grid and save path; correct place for an applicability row.
- `director/packages/director-app/src/features/opera/components/policy/useCreatePolicyToSaveFunction.ts`: store-to-`OperaPolicy` mapping.
- `director/packages/director-app/src/features/opera/utils/preparePolicy.ts`: `OperaPolicy`-to-API `PolicyConfig` mapping.
- `director/packages/director-app/src/components/opera/utils/prepareOperaPolicy.ts`: API-to-`OperaPolicy` hydration.
- `director/packages/director-app/src/features/opera/store/OperaStore.ts`: persistent wizard state and defaults.
- `director/packages/director-app/src/components/data-management/RegularMetadataDrawer/components/ApplicableAreasSelect.tsx` and `shared.ts`: interaction/copy prior art using `ListSelect`; not reusable as-is because it binds to `MomentAppType`.
- `director/packages/director-app/src/features/training-simulator/create-module/evaluation-criteria/getTriggerOptions.ts` and `hooks/coaching/useAutoQATriggers.ts`: Training Simulator criteria currently load all active/inactive Opera policies without product-area filtering.

### Contract and persistence

- `cresta-proto/cresta/v1/policy/policy.proto`: public `PolicyConfig`; appropriate home for `ProductArea` and `applicable_product_areas`.
- `cresta-proto/cresta/v1/policy/policy_service.proto`: `PolicyConfigFilter`; appropriate home for the requested product area used during policy search.
- `cresta-proto/cresta/nonpublic/orchestrator/orchestrator.proto`: orchestrator `Conversation`; currently does not carry `cresta.v1.conversation.Conversation.Source`.
- `go-servers/apiserver/sql-schema/protos/policy/policy.proto`: DB `PolicyConfig` persisted inside existing policy-config JSONB.
- `go-servers/apiserver/internal/policy/policy_converters.go`: service↔DB policy-config conversion.
- `go-servers/apiserver/internal/policy/policy_filters.go`: selects policies by time, language, use case, transfer behavior, and traffic type; add product-area matching here.

### Runtime propagation

- `go-servers/shared/conversation/orchestrator_converter.go`: online `redisconversation.Conversation`→orchestrator conversion already has `cachedChat.Source` available.
- `go-servers/orchestrator/internal/persist/base_client.go`: constructs `SearchPoliciesRequest.PolicyConfigFilter`.
- `go-servers/orchestrator/shared/redaction_requirement.go`: existing fallback pattern looks up source because orchestrator conversation does not carry it; for policy loading, propagating the source is preferable because both online and offline builders already have it.
- `go-servers/temporal/ai_services/backtest/shared/execute_conversation.go`: offline/backtest `cresta.v1.conversation.Conversation`→orchestrator construction already has `conv.GetSource()` available.
- Training conversations are created with `Conversation.Source.TRAINING_SIMULATOR`; this source is the authoritative discriminator and should be mapped once to a policy product area.

## Key reasoning

### Why this is not a frontend-only change

The Finalize selector must change which policies the orchestrator loads. Training Simulator's evaluator waits for and reads annotations after the normal Opera path runs. If the field were only stored/displayed in Director, production and training traffic would still generate identical annotations.

### Why not reuse `Moment.AppType`

`Moment.AppType` answers where a moment template can be used or surfaced (`COACHBUILDER`, filters, Insights chart, handoff, etc.). Every Opera-created moment currently uses `APP_TYPE_COACHBUILDER`. It does not encode conversation traffic. Overloading it would conflate template availability with policy execution and require changing every nested rule moment.

### Recommended contract

- Add `PolicyConfig.ProductArea` with `PRODUCT_AREA_UNSPECIFIED`, `QUALITY_MANAGEMENT`, and `TRAINING_SIMULATOR`.
- Add repeated `applicable_product_areas` to public and DB `PolicyConfig`.
- Add one requested `product_area` to `PolicyConfigFilter`.
- Add conversation `source` to the nonpublic orchestrator `Conversation`.
- Compatibility semantics:
  - Stored applicability empty: all product areas.
  - Filter product area unspecified: do not filter (legacy/internal callers).
  - Nonempty stored list: requested area must be present.
  - Orchestrator maps only `TRAINING_SIMULATOR` to Training Simulator; all current other sources map to Quality Management.

This keeps policy semantics product-oriented while using the canonical conversation-source signal at the runtime boundary.

### UI semantics

- New rule default should explicitly select both values.
- Legacy rules with empty API applicability hydrate as both.
- Empty user selection should be invalid; do not let an empty selection serialize as “all,” because deselecting everything would silently broaden scope.
- Use `ListSelect` and the Data Management layout/copy pattern, but create an Opera-specific component bound to `PolicyConfig.ProductArea`.
- Support both create and edit. Restricting the field to create would make a persisted property invisible and immutable after creation.
- Filter behavior choices by the consuming product area. Training Simulator module configuration should include Training Simulator/both/legacy-empty rules; QM scorecard/template consumers should include Quality Management/both/legacy-empty rules. The Opera rules overview can continue showing all rules.

### Overrides and backfills

`PolicyOverride` intentionally loads explicitly selected rules for Opera simulation/backtests. Recommended semantics are to bypass product-area restrictions for explicit overrides, preserving testability. Normal online and post-close Training Simulator flows do not use an override and must filter. Backfill paths that do not specify a product area retain legacy no-filter behavior unless product decides backfill must honor source; source-aware backfill can be enabled by propagating the field through `buildBasicOrchRequest`.

## Implementation plan

1. **Proto contract (`cresta-proto`)**
   - Add `PolicyConfig.ProductArea` and `applicable_product_areas` in `cresta/v1/policy/policy.proto`.
   - Add `PolicyConfig.ProductArea product_area` in `PolicyConfigFilter` in `policy_service.proto`.
   - Import `cresta/v1/conversation/conversation.proto` and add `Conversation.Source source` to `cresta/nonpublic/orchestrator/orchestrator.proto`.
   - Regenerate web-client/Go artifacts through normal CI and validate lint/build/dependency rules.

2. **Persistence and policy matching (`go-servers/apiserver`)**
   - Mirror the enum/field in `apiserver/sql-schema/protos/policy/policy.proto`; regenerate apiserver proto code.
   - Round-trip applicability in `dbPolicyConfigToServicePolicyConfig` and `servicePolicyConfigToDBPolicyConfig`.
   - Add a helper in `policy_filters.go` implementing empty-as-all and requested-unspecified-as-no-filter semantics.
   - Apply the filter during ordinary policy search. Preserve explicit override behavior unless product decides otherwise.
   - Add converter/create/update/get and filter matrix tests. No SQL migration is needed because `policy_config` is JSONB.

3. **Conversation-source propagation (`go-servers`)**
   - Set source in `shared/conversation/ConvertCacheToOrchestratorConversation`.
   - Set source in `temporal/ai_services/backtest/shared/buildBasicOrchRequest`.
   - In `orchestrator/internal/persist/base_client.go`, map source to `PolicyConfig.ProductArea` and populate the search filter.
   - Update focused converter, online-client/base-client, orchestrator request, and backtest tests.

4. **Opera Finalize UI (`director`)**
   - Add applicability to `OperaStore`, `OperaPolicy`, defaults, reset, persisted-policy hydration, and draft serialization.
   - Add API↔internal mapping in `prepareOperaPolicy.ts`, `useCreatePolicyToSaveFunction.ts`, and `preparePolicy.ts`.
   - Add an Opera-specific `ApplicableProductAreasSelect` to `ReviewPolicySection` step 4 with options “Quality Management” and “Training Simulator,” select-all support, and no-empty validation.
   - Gate row visibility with `enableTrainingSimulator` if confirmed, while defaulting hidden/non-enabled flows to Quality Management or preserving legacy values. Avoid dropping an existing Training Simulator value during edits if the row is hidden.
   - Filter `getTriggerOptions.ts` / `useAutoQATriggers` consumers by context. Prefer a shared pure applicability predicate; add a backend `ListPolicies`/`ListBehaviors` filter only if payload size or another non-Director consumer requires server-side filtering.
   - Add mapping/default/hydration, component interaction, and product-specific picker tests.

5. **Integration and rollout**
   - Validate QM-only, Training-only, both, and legacy-empty policies against one normal and one Training Simulator conversation.
   - Verify target-agent matching still uses the human trainee.
   - Verify explicit Opera simulation/backtest can exercise a Training-only rule.
   - Roll out additively without data migration; watch missing annotation/N/A rates and policy search counts.

## Open decisions

1. Should customers without `enableTrainingSimulator` see the row? Recommendation: no; hide it, but never erase stored applicability on edit.
2. Is edit support officially in scope? Recommendation: yes.
3. Should explicit policy overrides bypass applicability? Recommendation: yes.
4. Should a newly created rule default to both or Quality Management only? Recommendation: both, matching legacy current behavior and minimizing surprise for existing shared evaluation criteria. If product wants opt-in Training Simulator isolation, choose Quality Management explicitly and document the behavior change.

## Output

- Canonical work item: `training-simulator/work-items/CONVI-7281.md`
- Daily movement: `training-simulator/log/2026-08-10.md`
