# CLO MV config flag (contract only)

Date: 2026-07-28
Scope: cresta-proto Insights flag + planned config schema sync. No go-servers consumption, no customer enablement.

## Change

Added mirrored Insights field:

```protobuf
// When true, query conversation-outcome moment annotations from the
// conversation-outcome materialized view instead of moment_annotation_d.
// Unset/false keeps the existing moment_annotation_d path.
bool use_conversation_outcome_moment_annotation_materialized_view = 30;
```

Files:
- `cresta/config/features.proto`
- `cresta/v1/config/features/insights.proto`

Branch: `convi-7049-clo-mv-config-flag`
PR: https://github.com/cresta/cresta-proto/pull/9400

## Field number note

Plan draft used field **28**. On current `main`, 28 and 29 are already taken:
- 28: `allow_hint_stats_calculation_action_annotation_based_query`
- 29: `disable_dashboard_builder`

Used next free number **30**.

## Validation

- `bazel build //cresta/config/... //cresta/v1/config/features/...` — success
- `buf lint` / `buf breaking` against `origin/main` for the two paths — success
- Bazel-generated Go exposes:
  - JSON: `useConversationOutcomeMomentAnnotationMaterializedView`
  - Accessor: `GetUseConversationOutcomeMomentAnnotationMaterializedView()`
- `mage -v lint` — not run locally (Colima/Docker not running); rely on CI
- CI gates applied:
  - PR label `allow-feature-flag-proto-changes` (feature-flag proto freeze)
  - PR body line `ALLOW_CHANGES_TO_CONFIG=true` (legacy `cresta/config/**` read-only)
  - Both gates passing after re-run; lint/breaking/FF freeze green; remaining gen jobs still running at handoff

## Config schema sync (blocked)

Cannot bump `github.com/cresta/cresta-proto/v2` or regenerate `json-schema/configv3/**` until this PR merges and a module version is published.

Next steps after merge/release:
1. Trigger `update-proto-config-schema.yaml` in `cresta/config` (or wait for post-merge dispatch).
2. Confirm bump of `go.mod`/`go.sum` and regenerated Insights schemas include the camelCase bool.
3. Validate with `yarn test:ci:v3` (or full `yarn test:ci`).
4. Do **not** edit `magefiles/configwizard.go`, customer YAML, or `src/CustomerConfig.ts`.

## Default-off verification

- No customer/configv3 YAML edited for this flag.
- No configwizard defaults changed.
- No frontend CustomerConfig changes.
- Proto3 omission defaults the bool to `false` → no runtime path change until consumers read the flag and a customer opts in.

## Follow-up (handoff check-in)

Related Linear: [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via) (related to CONVI-7049; MV performance follow-up including this flag).

Todo status after handoff:
- `proto-contract` — completed
- `verify-default-off` — completed
- `config-schema-sync` — pending, blocked on #9400 merge + `cresta-proto/v2` publish

PR #9400 (as of the initial check-in): open, GitHub `MERGEABLE`, `mergeStateStatus: BLOCKED`. Required proto gates that finished were green (breaking, FF freeze, lint protobuf, config read-only, go-servers PR requirement). Remaining blockers included `@cresta/core` codeowner review and in-flight generation jobs.

## Go-servers generation failure

The completed CI run later failed `Check go-servers go generate`. The bot message overstates causality: field 30 caused the workflow to select the root go-servers module and run repository-wide `go generate ./...`, but neither generator error involved the new Insights flag.

Actual stale-contract failures:
- `config/shared/model/converter/converter.go`: API `EmailConfig.BodyCleaningRules` existed after cresta-proto #9380, but the go-servers DB schema had no matching field.
- `shared/converters/momentconverter/converter/converter.go`: API `MESSAGE_TYPE_SYSTEM` existed after cresta-proto #9388, but the go-servers internal Moment enum lacked it.

go-servers PR [#30519](https://github.com/cresta/go-servers/pull/30519) merged at 2026-07-28 14:02 UTC, after #9400's failed check at 13:42 UTC. It bumped cresta-proto to v2.15.73, added the Moment enum, ignored the not-yet-ingested email field in the converter, regenerated outputs, and verified `go generate ./...`. A rerun of #9400 against current go-servers `main` should therefore pass; no CLO-MV-specific go-servers companion change is required for this contract-only flag.

Explicitly not done yet: merge #9400, config schema sync, customer enablement.
