# Enable frontend feature flag skill

Date: 2026-09-10
Source repo: `/Users/xuanyu.wang/repos/coaching-qm-skills-frontend-feature-flag`
Branch: `xw/enable-frontend-feature-flag`

## Goal

Create a reusable skill that lets a user describe a Director frontend-feature-flag cohort conversationally, generates a reviewed `cresta/config` PR without relying on the Admin UI, and extends the workflow through final effective-state verification rather than stopping at PR creation.

## Evidence reviewed

- `configuration-management/sessions/2026-07-29/codex-global-frontend-feature-flag-rollout.md` for the Admin bulk editor, active-profile selection, Auto Inheritance, generated PR, and semantic coverage requirements.
- `convi-7049-clo-filter/sessions/2026-08-21/codex-spruce-config-collision.md` and `sessions/2026-08-23/codex-remote-master-verification.md` for the forward/back-sync lifecycle and the prior failure where history and green workflow state did not match effective ConfigService/Admin state.
- Current `config` workflows: `.github/workflows/sync_to_config_service_batch.yaml`, `.github/workflows/sync_from_config_service.yaml`, and `.github/workflows/update-featureflags.yaml`.
- Current `config/src/CustomerConfig.ts` and generated `docs/enabled-feature-flags.csv` surfaces for flag-definition and profile-level read-back checks.

## Design decisions

- Code tracing corrected the initial assumption: the Admin editor does not trigger a GitHub Action. It calls ConfigService `BatchSetFeatureFlags`, which performs a multi-edit with `CREATE_PR_TO_REVIEW` and opens/updates the config PR through the ConfigService GitHub app.
- None of the similarly named config workflows performs existing-customer flag enablement: `add-director-feature-flag.yaml` adds declarations, `update-featureflags.yaml` regenerates docs, and `sync_from_config_service.yaml` reads effective state back.
- The revised skill creates the review PR directly when no dedicated enablement workflow exists. A bundled deterministic updater reproduces the handler's merge semantics across existing profile and use-case legacy frontend flag maps while failing closed on missing maps, duplicates, invalid YAML, unknown customers, or unknown/deprecated flags.
- Natural-language selection resolves to an explicit environment/customer file manifest before mutation. For direct PRs, “all customers” is the current environment customer-history file set minus explicit exclusions.
- PR validation compares exact scope sets and rejects unrelated churn. A diff-only ConfigService check proves only prospective validity.
- Completion requires the merge-triggered non-dry-run forward sync and an independent ConfigService-to-`configv3` plus Admin read-back. A green forward workflow alone is insufficient.
- Broad retry inputs should be explicit customer lists rather than `customers=all`; divergence should be repaired through a single canary before bounded expansion.
- Long propagation should use a quiet recurring monitor that reports only meaningful transitions, failures, divergence, or required user action.

## Deliverable

- `enable-frontend-feature-flag/SKILL.md`
- `enable-frontend-feature-flag/references/config-lifecycle.md`
- `enable-frontend-feature-flag/agents/openai.yaml`
- `enable-frontend-feature-flag/scripts/update_frontend_feature_flag.py`
- Root README inventory entry

## Application status

The first version passed the Skill Creator package validator plus repository whitespace validation. The direct-PR revision passed a real staging fixture dry run, write, semantic verification, and idempotency check. It is published for review as [coaching-qm-skills#5](https://github.com/cresta/coaching-qm-skills/pull/5).

The first live application used current config `master` for CONVI-7642. The explicit staging manifest contains all 29 staging customer identities; Comcast and Schwab match no staging identities. The updater added `enableNAScore: true` to 176 existing profile/use-case maps with no deletions or unrelated fields, passed the repository ConfigService-history schema validation, and opened [config#153660](https://github.com/cresta/config/pull/153660). Production remains gated on staging effective-state verification.

Before handoff, the skill branch was rebased onto current `main` to resolve a README inventory conflict with the concurrently merged analytics skill. The resolved branch retains both entries, passed package validation again, and is mergeable at `c661cf4a99a66c87c847b2d439285e7f86690924` pending review and CodeRabbit.

CodeRabbit then found that the updater wrote each validated customer immediately, so a malformed later customer could leave a partial manifest applied locally. The finding was fixed in `a628c689c64f17c458fc813baa5d3a746362a59d` by staging all updated texts, validating the complete manifest, and only then writing. A two-customer regression case verified that failure on the second file leaves the first byte-for-byte unchanged. The bot marked the inline finding addressed; re-review is pending.

CodeRabbit subsequently approved the fix and PR #5 merged at 2026-09-10 16:55 UTC as `f90727177bbdef00f8844e88b19644d7a03fc401`.

CodeRabbit then identified a valid fail-closed gap: the updater wrote each customer immediately, so a malformed later file could leave an earlier subset changed. Commit `a628c689c64f17c458fc813baa5d3a746362a59d` stages all updated text in memory, completes YAML, identity, scope, and semantic validation for the full manifest, and only then writes the batch. Validation covered Python compilation, Skill Creator package validation, whitespace checks, a two-customer failure fixture proving the first file remains byte-for-byte unchanged when the second is malformed, and a successful two-file batch. The review thread was replied to and resolved; CodeRabbit re-reviewed and approved the commit, all checks passed, and the PR returned to a clean mergeable state.

The production application surfaced two cases not present in staging. First, production customer files can contain unrelated `featureFlags` blocks outside profile/use-case frontend configuration, so the global textual block counter rejected valid files. The updater now obtains exact profile/use-case block locations from parsed YAML line metadata and updates only those blocks. Second, six customers have a use case without a frontend flag map; ConfigService `LEVEL_ALL` rejects the entire customer in that case. A new explicit `--application-level profile-only` mode mirrors ConfigService `LEVEL_PROFILE_ONLY`, allowing those customers' use cases to inherit the profile value without inventing a frontend map.

The changes are under review in [coaching-qm-skills#6](https://github.com/cresta/coaching-qm-skills/pull/6) at commit `2bc1b4c372660cfe7ea670ab4f7b6c34a350df37`. The branch was rebased onto the squash-merged PR #5 commit so the review contains only the two intended skill files. Validation included Python compilation, frontmatter parsing, real production dry runs for 356 all-level and six profile-only customers, post-application idempotency, and an independent 1,972/1,972 effective-scope comparison.

The production ConfigService dry-run artifact later exposed 15 unrelated removals of stored empty `summarization_config` fields even though the Git diff only added the requested flag. This demonstrates that source-diff purity and a green workflow conclusion do not establish semantic purity. Commit `60766ae` updates the skill and lifecycle reference to require inspection of the complete dry-run artifact and reject every effective change not attributable to the requested flag. When customer-history omission differs from an explicit stored value, the skill may materialize that exact value only to preserve state, and must rerun until the effective ConfigService diff is flag-only. Skill Creator package validation and whitespace validation pass.

The replacement artifact revealed an additional case under The Zebra where the stored use case is absent from customer history, so its exact state cannot be preserved safely in the rollout PR. Commit `d578564` adds the corresponding fail-closed rule: isolate that customer, report the incomplete cohort, and use a separately reviewed patch-level mechanism. This prevents pressure to retain an “all customers” count from authorizing unrelated configuration loss.
