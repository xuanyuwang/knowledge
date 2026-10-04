# CONVI-7281 frontend readiness and scope verification

**Date:** 2026-09-22
**Primary source repo:** `/Users/xuanyu.wang/repos/director`
**Related repos:** `cresta-proto`, `go-servers`

## Question

Verify whether merged proto PR #9890 and backend PR #32384 leave only two frontend tasks: render the UI and save the new policy selection to the database.

## Verified release and PR state

- `cresta-proto#9890` merged as `4dcd1013d8` and adds only `Policy.SourceType.SOURCE_TYPE_TRAINING_SIMULATOR = 5` to the existing enum/field.
- Tags `v2.28.9`, `v2.28.10`, and `v2.28.11` all descend from that merge. `v2.28.9` is the earliest inspected compatible release.
- This was a point-in-time snapshot. Current Director `main` at frontend implementation time pins `@cresta/web-client` `2.28.16`, which includes the new enum; no dependency bump is required.
- go-servers foundation PR #32650 merged as `bf663d5467`; it adds the DB enum value, explicit public/DB converter mappings, and tests. The public value is 5 and DB value is 6.
- Runtime PR #32384 is open at `00993a5289`. Required CI is green, while code-owner approval is still pending.

## API conclusion

No new RPC, request message, policy field, or DB column is needed. This is still an API contract change because an existing public enum gained a value. Director sends the existing `Policy.sourceType` field through the existing create/update APIs; go-servers converts and persists it in the existing `app.policies.source_type` column. “Frontend saves to the database” is therefore an imprecise boundary description.

## Required frontend surface

1. Bump all aligned Director `@cresta/web-client` pins/lockfile entries to at least `2.28.9`.
2. Add an Opera-specific applicability selector to both `ReviewPolicySection` and `ReviewPolicySectionWithPerformanceConfig`.
3. Represent the exclusive contract as “All conversations” (`SOURCE_TYPE_COACH_BUILDER`) or “Training Simulator only” (`SOURCE_TYPE_TRAINING_SIMULATOR`). The merged enum cannot represent production-only plus both as three independent states.
4. Add state/default/edit hydration/reset/draft/dirty round-trip in `OperaStore` and `OperaPolicy`.
5. Replace `preparePolicy.ts`'s unconditional `SOURCE_TYPE_COACH_BUILDER` save assignment with the selected value.
6. Add `SOURCE_TYPE_TRAINING_SIMULATOR` to the Opera rules overview source filter so created rules remain visible/editable.
7. Cover create, edit, draft restoration, request mapping, selector behavior, legacy/default behavior, and overview listing with focused tests.

The standalone Opera simulator can continue excluding Training Simulator policies if it is meant to model ordinary traffic. Creator/editor simulation uses explicit policy history and should be verified, but its background policy list should not be broadened without a product decision.

## Superseded backend compatibility finding

The finding below was correct for runtime PR head `00993a5289`, which used module-derived exact policy overrides. It is not a blocker for the current frontend or current runtime architecture. Runtime PR #32384 was subsequently rewritten at `99d196e76f`: it removed the resolver and automatic override, propagates conversation source into policy search, excludes Training Simulator-only policies from ordinary/unspecified conversations, and loads all active sources for Training Simulator conversations.

At PR #32384 head `00993a5289`:

- `ResolvePolicyIDsForModule` filters policy rows to `SOURCE_TYPE_TRAINING_SIMULATOR` only.
- It requires one such mapping for every module criterion behavior and fails closed when none exists.
- `populateTrainingSimulatorPolicyOverride` sets the resolved IDs on every new Training Simulator conversation.
- `loadDBPolicies` chooses either explicit override policies or ordinary active policies; it does not union them.

Existing Training Simulator modules reference ordinary Coach Builder policies. The new “all conversations” selector state also saves Coach Builder. Therefore an existing or mixed module cannot resolve all criteria and can fail conversation start. If only Training Simulator IDs were returned, the non-empty override would also suppress ordinary active loading.

Before merging #32384, add compatibility behavior and tests for:

- a legacy/Coach Builder-only module;
- a Training Simulator-only module;
- a mixed Coach Builder plus Training Simulator module;
- exact criterion-policy selection without unrelated ordinary rules.

The implementation can either resolve both allowed policy sources into the exact override or deliberately merge ordinary applicable policies with Training Simulator overrides. Exact module-derived override IDs are the narrower and more deterministic option, but the final choice must preserve legacy modules and the “all conversations” state.
