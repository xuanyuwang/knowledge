# Production CLO enablement for NCLH and Holiday Inn

**Date:** 2026-08-14
**Source repo:** `/Users/xuanyu.wang/repos/config`
**Target branch/worktree:** `xw/convi-7049-clo-production-customers` at `/Users/xuanyu.wang/repos/config-convi-7049-clo-prod`
**Related work:** CONVI-7049, config PRs [#151310](https://github.com/cresta/config/pull/151310) and [#151311](https://github.com/cresta/config/pull/151311)

## Objective

Ensure production NCLH `us-east-1` and the supported Holiday Inn voice profiles expose the Cresta-modeled outcome filter in Performance Insights and route eligible conversation-outcome moment-annotation reads through the CLO materialized view. Keep legacy `holidayinn_chat` disabled.

## Initial repository findings

- The config repository's local `master` checkout is 5,427 commits behind its local `origin/master` tracking ref and contains an unrelated modification to `configv3/prod/oportun/us-west-2/frontend.yaml`; it will not be used for edits.
- Local `origin/master` is `4cc375cfaf6f0dc6e55ea985500e73ba31a1d580` (`Synced customers from ConfigService (#151320)`, 2026-08-11).
- Config PR #151310 added `useConversationOutcomeMomentAnnotationMaterializedView: true` to NCLH production profile/use-case Insights config. Current NCLH config also contains `enableCLOFilters: true`.
- Config PR #151311 added the MV-read flag only to Holiday Inn `club-voice`, `owners-voice`, `social-voice`, `transfers-voice`, and `voice` profile/use-case Insights config.
- No `enableCLOFilters` entry exists in the current Holiday Inn production config, so the customer-facing filter is the missing idempotent change.
- Prior Phase B evidence records completed MV/backfill work for supported production profiles and explicitly excludes `holidayinn_chat`, whose legacy source lacks `moment_annotation_payload` and therefore has no trigger MV.

## Safety

- No production, cloud, database, GitHub, or package-registry credential was intentionally selected or authenticated with.
- One overly broad local YAML inspection printed an embedded frontend telemetry credential field. Reading counts as credential use under workspace policy. The value was not reused, validated, transmitted, stored in notes, or passed to another tool; all subsequent YAML checks select exact non-secret keys.
- For PR creation, the GitHub-specific SSH identity and `gh:github.com` keychain item were individually inspected and had no restriction marking. The separate break-glass private key was not read or authenticated with. Its public companion was inadvertently printed after the restriction was discovered in the same combined inspection command; it was not used further.
- No merge, deployment, or production mutation is in scope.
- The unrelated Oportun working-tree change will remain untouched.

## Next steps

1. Create an isolated config worktree from local `origin/master`.
2. Confirm exact profile identifiers, environments/regions, use-case inheritance, and prior MV/backfill evidence.
3. Add only the missing Holiday Inn frontend flags, explicitly leaving `holidayinn_chat` unchanged.
4. Run scoped and repository validation, inspect the diff, and update the daily log.

## Final configuration state

Repository configuration confirms:

- NCLH production is exactly `customers/nclh/profiles/us-east-1`, `awsRegion: us-east-1`, cluster `us-east-1-prod`. Its profile had both `enableCLOFilters: true` and `useConversationOutcomeMomentAnnotationMaterializedView: true`, and all 18 configured use cases had the MV-read flag. However, only `o-and-r-intl-air-ops` explicitly had the Director flag; the config editor correctly warned that the other 17 use cases disabled it. This session added the 17 missing use-case `enableCLOFilters: true` entries.
- Holiday Inn production is `awsRegion: us-west-2`. The five supported profiles run on `voice-prod`: `club-voice`, `owners-voice`, `social-voice`, `transfers-voice`, and `voice`.
- Config PR #151311 had already enabled MV reads at both profile and use-case scopes for those five profiles (11 Insights additions total). This session added the Director flag at all five profile scopes and all six supported use-case scopes; `social-voice` owns both `compliance` and `social-voice` use cases.
- `customers/holidayinn/profiles/chat` runs on `chat-prod` and remains unset for both flags. Its use case `customers/holidayinn/profiles/chat/usecases/chat` also remains unset.
- The final source diff changes only `customer_config_history/prod/customers/nclh.yaml` and `customer_config_history/prod/customers/holidayinn.yaml`: 17 NCLH use-case additions plus 11 Holiday Inn profile/use-case additions, 28 lines total.

## MV and backfill evidence

- ClickHouse schema PR #249 (`b5d0faa`) defines the required storage table `moment_annotation_by_conversation_outcome`, trigger MV `moment_annotation_mv_by_conversation_outcome`, and distributed read table `moment_annotation_by_conversation_outcome_d`.
- The 2026-08-07 production Phase B record documents successful `voice-prod` dry run `31220530743` and apply `31220875375`: all 24 non-empty databases were validated against 80,747,749 expected deduplicated source keys, with only concurrent trigger-write positive deltas and no failed target. Repository config maps every enabled Holiday Inn voice profile to that cluster.
- The same record documents successful `us-east-1-prod` dry run `31232323695` and apply `31232485782`: 68 non-empty databases validated with 317,036,293 expected keys and 317,041,632 target rows, no target behind. Repository config maps NCLH `us-east-1` to that cluster.
- The legacy `holidayinn_chat` source lacks `moment_annotation_payload`; prior rollout evidence confirms it has storage + distributed table only and no trigger MV. It is therefore deliberately excluded.

## Validation

Passed:

- changed-file YAML validation: `yarn yaml-validator customer_config_history/prod/customers/holidayinn.yaml .yamllint.yaml`
- config-history frontend schema validation: `yarn ajv-validate:configdb-frontend`
- CI YAML validation: `yarn yaml-validate:ci`
- `yq` structural assertions/readback for exact profile, region, cluster, frontend flag, MV flag, and chat exclusion
- `git diff --check`

Final assertions show NCLH enabled at 18/18 use cases and Holiday Inn enabled at 6/6 supported use cases across the five voice profiles. `holidayinn_chat` remains unset at both profile and use-case scopes.

The broad `yarn yaml-validate` wrapper hit macOS `E2BIG` because it expands every YAML path into one command. The aggregate `yarn ajv-validate` wrapper could not start because reused local dependencies lack `run-p`. Dependencies were not installed because installation could consult uncleared package-registry credentials. The specific schema validator that owns the changed config passed.

## Pull request

- Rebased onto current `origin/master` (`e721dd009a`) before the final validation pass.
- Commit: `89ed8cf80002c17bc933da7bb669b189eb869cbb` (`CONVI-7049 Enable CLO filters for NCLH and Holiday Inn`).
- Ready-for-review PR: [config#151643](https://github.com/cresta/config/pull/151643).
- The PR is open against `master`; it was not merged or deployed.

## PR comment and precedent validation

Reviewed all issue comments, inline review comments, review states, and checks on config#151643, then compared config#151310, config#151311, config#149728, and config#148221.

- Current PR checks are all successful or intentionally skipped, including Batch Sync to ConfigService, JavaScript/Python YAML lint, `config-consistency`, Schema proto test, freeze checks, CodeRabbit, Cursor Bugbot, Linear ticket, and secret scanning. GitHub reports `CLEAN` and `MERGEABLE` with no non-success checks.
- GitOps semantic-diff comments show only `enableCLOFilters: true` additions. They do not report the earlier profile-enabled/use-case-disabled inconsistency.
- Exact scope comparison now passes: NCLH has MV and CLO flags on the same 1 profile + 18 use cases; Holiday Inn has both flags on the same 5 profiles + 6 supported use cases. No non-CLO line is added by the PR.
- Automated config#151310 established the NCLH MV shape: 1 production profile + all 18 production use cases (19 additions), plus 2 sandbox scopes. Automated config#151311 established the Holiday Inn MV shape: 5 supported profiles + 6 supported use cases (11 additions), excluding chat. The current frontend flag matches those production scope sets exactly.
- Automated config#149728 added the NCLH frontend flag only at profile scope (plus later generated churn), which explains the config-editor warning and is not sufficient precedent for inheritance. config#148221 is the stronger frontend precedent: Rivo rollout explicitly wrote `enableCLOFilters` at profile and use-case scopes.
- CodeRabbit posted one inline request to add `yaml-language-server` directives. This is a false positive for `customer_config_history`: none of the 367 production customer-history YAML files has that directive, the same NCLH/Holiday Inn files lacked it in the automated predecessor PRs, and all owning validators/checks pass. No schema-header change should be added to this PR.
- Cursor Bugbot says Holiday Inn has 10 changed scopes, but the repository diff and CodeRabbit correctly count 11 (5 profiles + 6 use cases). This is a summary miscount, not a config defect.
