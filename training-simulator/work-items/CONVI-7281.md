# CONVI-7281: Add Training Simulator applicability to Opera rules

**Status:** active
**Primary domain:** `training-simulator`
**Primary subdomain:** `evaluation`
**Official ticket:** [CONVI-7281](https://linear.app/cresta/issue/CONVI-7281/add-training-simulator-only-option-for-new-opera-rule-creation)
**Last updated:** 2026-08-10

## Objective and Impact

- **Objective:** Let an Opera rule author choose whether a rule applies to Quality Management/production conversations, Training Simulator conversations, or both.
- **Customer/system impact:** Training-only evaluation criteria can be kept out of production QM traffic while shared criteria can continue evaluating both traffic classes. The setting must affect Opera policy selection before annotations are generated; a Finalize-step selector alone would not satisfy the behavior.
- **Role:** investigated and designed

## Scope

**In scope**

- A policy-level product-area contract with Quality Management and Training Simulator values.
- Backward-compatible persistence in the existing policy-config JSONB.
- Runtime policy filtering using `Conversation.Source.TRAINING_SIMULATOR`.
- Opera create/edit Finalize UI, draft round-tripping, defaults, and validation.
- Product-aware behavior/criterion pickers so Training Simulator modules cannot select production-only rules and QM surfaces do not offer Training-only rules.
- Focused proto, backend, orchestrator, converter, and frontend tests.

**Non-goals**

- Changing Training Simulator scoring, N/A handling, or module criteria storage.
- Reusing `Moment.AppType`; that field controls where moment templates are available in product UI/features, not which conversation traffic a policy evaluates.
- Migrating existing policy rows or adding a relational schema column.
- Resolving inactive-criterion warnings (CONVI-7280) or metadata-trigger requirements (CONVI-7438).

## Source Context

- **Repos:** `cresta-proto`, `go-servers`, `director`
- **Worktrees:** main checkouts inspected read-only; `director` local main was 1,109 commits behind its tracked `origin/main`, so authoritative code references were checked against `origin/main` dated 2026-08-06.
- **Branches:** no implementation branch created
- **PRs/commits:** none

## Current Understanding

This is a cross-repo contract and runtime-selection change, not a frontend-only option. Add a policy-level `ProductArea` enum and repeated `applicable_product_areas` to both the public and DB `PolicyConfig` protos, with empty meaning all product areas for backward compatibility. Pass the conversation source into the orchestrator conversation, map `TRAINING_SIMULATOR` to the Training Simulator product area and every other current source to Quality Management, add that area to `PolicyConfigFilter`, and filter policies in `filterDBPolicies`. Persist the field through existing JSONB converters; no SQL migration is required.

In Director, add explicit `[QUALITY_MANAGEMENT, TRAINING_SIMULATOR]` default state, hydrate legacy empty values as both, render a required multi-select on Finalize using the Data Management `ListSelect` interaction pattern, and round-trip it through policy drafts/create/update. Gate exposure using the Training Simulator customer feature flag if product confirms non-enabled customers should not see the row, while keeping API/runtime support unconditional. Filter `useAutoQATriggers` consumers by context: the Training Simulator module criterion picker accepts Training Simulator/both/legacy-empty policies, while QM scorecard/template consumers accept Quality Management/both/legacy-empty policies.

## Findings and Decisions

- The ticket has no comments, blockers, customer needs, or release assignment. Its parent is [CONVI-7362](https://linear.app/cresta/issue/CONVI-7362/training-simulator-opera-master-ticket).
- A July 20–28 Slack investigation confirms the product intent as “QM or Training Sim or both” and separately confirms that target-agent matching should continue using the human trainee. Product-area applicability therefore must not replace or alter audience targeting.
- Training Simulator evaluation consumes Opera-produced moment annotations; it does not independently execute rule logic. Filtering must happen when orchestrator loads policies, before policy-engine evaluation.
- `PolicyConfig` is already persisted as protobuf JSON in `app.policies.policy_config`; adding a repeated enum is forward/backward compatible when empty means all.
- `orchestrator/internal/persist/base_client.go` builds `SearchPoliciesRequest.PolicyConfigFilter`, but the orchestrator `Conversation` currently lacks source. Both online conversion (`shared/conversation/orchestrator_converter.go`) and offline/backtest request construction (`temporal/ai_services/backtest/shared/execute_conversation.go`) need to propagate source.
- The backend has a source-lookup precedent in `orchestrator/shared/redaction_requirement.go`, but carrying source on the orchestrator conversation is preferred here: online and offline builders already possess the source, and policy loading should not add a database lookup.
- Existing `Moment.AppType` / Data Management “Applicable product areas” is useful UI prior art only. `APP_TYPE_COACHBUILDER` identifies Opera availability and cannot distinguish production from Training Simulator traffic.
- Training Simulator currently builds evaluation choices from all active/inactive Opera policies through `useAutoQATriggers`; runtime filtering alone would leave invalid production-only choices visible and lead to N/A criteria.
- Proposed defaults: API/DB empty = both for legacy compatibility; Director new-rule state = both explicitly; UI disallows saving an empty selection.
- Proposed filtering: an unset requested product area means no filter for internal/legacy callers; a requested area accepts policies with an empty applicability list or a list containing that area.
- Explicit policy overrides used by Opera simulation/backtests need a deliberate semantic: recommended default is to keep existing override behavior and bypass product-area restrictions, so authors can test a rule outside live traffic selection.

## Blockers and Dependencies

- Product confirmation is needed on whether the Finalize row should be hidden behind `enableTrainingSimulator` and whether edit flows are formally in scope. Engineering recommendation: gate the row for non-entitled customers but support create and edit for consistency.
- Product confirmation is needed on override semantics. Engineering recommendation: explicit overrides bypass product-area filtering; ordinary Training Simulator conversations never do.
- Proto must land and generated Go/web-client dependencies must be consumed before backend/frontend changes compile.

## Validation and Rollout

- Proto: lint/build `cresta/v1/policy`, `cresta/nonpublic/orchestrator`, and generated dependency checks.
- Backend: converter round-trip tests; `filterDBPolicies` matrix for legacy empty, QM-only, Training-only, both, and unset filter; orchestrator request mapping for Training Simulator and non-training sources; online/cache and offline/backtest propagation tests.
- Frontend: store/default/hydration tests, policy request mapping tests, Finalize selector interaction/validation tests, create/edit draft round-trip, and product-aware `getTriggerOptions`/QM picker tests.
- Integration: create three rules (QM-only, Training-only, both), run one production and one Training Simulator conversation, and assert only the expected policy/behavior annotations exist. Verify a legacy rule with no field still evaluates both.
- Rollout should be additive and migration-free. Monitor policy search counts and missing-annotation/N/A rates for Training Simulator after enablement.

## Next Actions

1. Confirm the two product decisions: feature-flag visibility/create-vs-edit scope and explicit-override behavior.
2. Implement and merge the additive `cresta-proto` contract first.
3. Update `go-servers` persistence, policy filtering, and conversation-source propagation with the full compatibility test matrix.
4. Update `director` Finalize UI, state/request round-trip, and Training Simulator/QM behavior pickers after the generated web client contains the new enum/field.
5. Run the production-vs-training integration matrix before rollout.

## Timeline

- 2026-08-10 — Investigated Linear, current domain knowledge, authoritative repo snapshots, Data Management prior art, policy persistence/filtering, orchestrator source propagation, and the July Opera/Training Simulator incident thread; produced a cross-repo implementation plan. Evidence: `sessions/2026-08-10/codex-convi-7281-plan.md`, `log/2026-08-10.md`.
