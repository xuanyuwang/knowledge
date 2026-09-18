# Direct frontend feature-flag PR path

Date: 2026-09-10
Source repos: `/Users/xuanyu.wang/repos/config`, `/Users/xuanyu.wang/repos/go-servers`, `cresta/tenant-admin`, `cresta/cresta-proto`
Related ticket: CONVI-7642

## Question

Which GitHub Action does the Cresta Admin Director Feature Flags page trigger, and how can an agent generate an equivalent reviewed PR without using the page?

## Finding

There is no GitHub Action behind this UI operation.

`tenant-admin/src/pages/customer/feature-flags/useSetFeatureFlags.ts` builds one `BatchSetFeatureFlagsRequest` per selected customer/profile scope and calls `ConfigApi.batchSetFeatureFlags` sequentially. When all profiles are selected for a customer, the resource is `customers/<id>/profiles/-/usecases/-`. Auto Inheritance uses `LEVEL_ALL`.

The proto exposes `ConfigService.BatchSetFeatureFlags` as a SUPER_ADMIN cross-customer method. Its Go handler in `go-servers/config/internal/service/config/action_batch_set_feature_flags.go`:

1. parses the customer/profile/use-case wildcard scope;
2. obtains the current customer configs;
3. merges each requested boolean into existing `profile.desktopAppConfig.legacyPublicConfig.featureFlags` and `usecase.desktopAppConfig.legacyPublicConfig.featureFlags` maps;
4. calls the multi-edit path with `CREATE_PR_TO_REVIEW` and description `Update feature flags`;
5. returns the GitHub review URI.

The ConfigService GitHub client creates/updates the PR directly. Existing config workflows with feature-flag-adjacent names are not equivalent:

- `add-director-feature-flag.yaml` adds the schema declaration for a new flag;
- `update-featureflags.yaml` regenerates docs from `configv3`;
- `sync_from_config_service.yaml` reads effective ConfigService state into `configv3`.

## Skill decision

Until a dedicated existing-customer enablement workflow is added, the reusable skill creates a normal reviewed config PR directly. It resolves the natural-language cohort into an explicit customer manifest and uses a deterministic text-preserving updater to reproduce the ConfigService merge across every existing profile/use-case frontend flag map. The updater validates the flag declaration, customer identity, YAML structure, map count, duplicates, requested boolean, and idempotency.

The downstream lifecycle is unchanged: PR CI performs a diff-only ConfigService check; merge performs the non-dry-run forward sync; scheduled or targeted sync-from provides independent effective read-back.

## CONVI-7642 staging application

Using current config `master` at `cf836c6bb776e8674ece0c740130f721f9e491fd`, the selection resolved to all 29 staging customer files. Neither Comcast nor Schwab exists as a staging customer identity; incidental product text mentioning Schwab in test fixtures was not treated as a customer match.

The updater changed 176 existing profile/use-case feature-flag maps across 29 files. The diff contains only 176 additions of `enableNAScore: true`, no deletions, and no production files. A repeat dry run reported all 176 scopes already desired with zero further changes. `yarn ajv-validate:configdb-frontend` passed. Commit `e5863e68589e689efb90ff3745abfe56e620f025` is under review in [config#153660](https://github.com/cresta/config/pull/153660).

Current lifecycle status is **PR ready for review**. The Batch Sync to ConfigService PR dry run and other checks are pending. No merge was performed; production is blocked until staging forward sync and effective read-back succeed.

At 2026-09-10 16:48 UTC, PR #153660 merged as `d7dce7a2fafdaa4c3c4ab13a0de9a1b357b2bb55` after the full PR suite, including the diff-only ConfigService sync, passed. Merge-triggered Batch Sync run #34504377738 is the authoritative non-dry-run forward sync and was still running at the 16:53 heartbeat. The lifecycle status advanced to **merged / forward sync pending** and was posted to CONVI-7642 through Linear MCP.

Run #34504377738 later completed successfully without cancellation. Its uploaded diff artifact contains exactly 176 `enableNAScore=true` changes: 48 profiles and 128 use cases, matching the planned customer-history scope.

ConfigService read-back is split by target customer groups. PR #153672 added the flag for `oportun`, `rcb-sandbox`, `se-interview-sbx`, `tranzact`, and `yelp`; PR #153675 added the profile values for `cresta`. Both auto-merged. An exact current-`configv3` inheritance-aware comparison confirms 83/176 intended scopes across those six customers. The remaining 93 scopes span the other 23 staging customers and have not yet appeared in `configv3`, so the lifecycle is **forward synced / read-back pending**, not effective or failed/diverged yet. Production remains blocked.

An on-demand monitor check at 2026-09-10 19:05 UTC inspected Sync From ConfigService run #34517259670 and its PR #153691. The run completed successfully but skipped both staging steps; the PR auto-merged three unrelated production changes and contained no `enableNAScore` diff. Re-running the exact inheritance-aware staging comparison against current master still produced 83/176 effective scopes across 6/29 customers. Because this was not a lifecycle transition, the monitor intentionally did not post another Linear comment.

## Production phase

The user accepted the QA team's prior staging testing and authorized production review in parallel with the remaining staging read-back. Current master contained 380 production customer-history files. The explicit selection includes 362 and excludes 18 Comcast/Schwab identities; `cmcst` was classified as Comcast Business and `cresta-sandbox` as Schwab-hosted based on their config identity and deployment metadata. Incidental mentions in `amex` and `bt-sandbox` were not treated as customer identity.

Six included customers each contain one email use case without a frontend feature-flag map. ConfigService `LEVEL_ALL` fails the entire customer in that condition. The rollout therefore applies the normal all-level update to 356 customers and a separate `LEVEL_PROFILE_ONLY` equivalent to `cresta-testing-au`, `cresta-testing-ca`, `cresta-testing-east`, `cresta-testing-gcp-prod-us`, `cresta-testing-west`, and `marriott-gc-sbx`. Their 17 use cases have no conflicting explicit value and inherit the profile value, including the six mapless email scopes.

[config#153693](https://github.com/cresta/config/pull/153693) contains 1,955 additions across 362 production files, every addition exactly `enableNAScore: true`, with no deletions, excluded customers, other environments, or unrelated changes. The inheritance-aware comparison resolves all 1,972 intended scopes to true. Repeat dry runs were idempotent and `yarn ajv-validate:configdb-frontend` passed. The PR is open for review with auto-merge disabled; no production forward sync has occurred.

All required PR checks later passed, including the authoritative `diff_only=true` Batch Sync to ConfigService, YAML/schema validation, config consistency, credential scanning, and the Schwab exclusion check. The lifecycle remains **production PR ready for review** because the PR has not been reviewed or merged; no production forward sync has occurred.

Full inspection of that successful dry-run artifact found 15 unintended removals of explicit empty `summarization_config` values. The affected customer-history use cases omitted `summarizationConfig`, while ConfigService stored `{}`; syncing each complete changed use case therefore exposed absent-versus-empty representation drift. Because the persistence path writes the field, this was a real prospective side effect and the PR was reclassified **unexpected diff / do not merge as-is** on Linear.

Commit `a3eedf33aa5cb125d5ea7472972088aa960ccbc7` adds `summarizationConfig: {}` only at the 15 affected use cases, reproducing their current stored state so the requested flag mutation does not remove it. `yarn ajv-validate:configdb-frontend` and whitespace validation pass. Replacement diff-only run #34525294546 is the acceptance boundary: the PR remains blocked until its complete artifact proves every effective change is only `enableNAScore: true` at an intended scope.

Run #34525294546 succeeded and confirmed those 15 removals were eliminated, but complete changed-line inspection found a sixteenth removal under `customers/the-zebra/profiles/us-west-2/usecases/ai_poc`. That stored use case is absent from both customer history and current `configv3`, so its exact state cannot be represented safely in this whole-customer PR. Commit `82720e96731c97c23aa12388d4e4fe216827469f` removes all five The Zebra flag additions, isolating that customer rather than accepting the unrelated removal. PR #153693 now covers 361 customers, 1,950 explicit additions, and 1,967 effective scopes. The Zebra is a documented five-scope gap requiring a separately reviewed patch-level mechanism. Replacement run #34526815160 is pending.

Run #34526815160 then surfaced unrelated concurrent-master changes because the rollout branch was 23 commits behind. The branch was rebased onto current `master` and force-updated safely at `0153d926a3a906e084b18fc39aef6b691d6be381`. Final diff-only run #34528022370 passed. Its complete artifact has exactly 1,950 changed configuration lines, every one an addition of `enableNAScore: true`; the only six minus-prefixed lines are Markdown separators. All required checks pass, the PR is cleanly mergeable, and auto-merge remains disabled. Lifecycle status is **PR ready for review for the safe cohort**, with The Zebra still unresolved.

A subsequent fast-moving `master` rebase introduced duplicate `summarizationConfig` keys where upstream had independently materialized 13 of the 15 preservation values. After removing those duplicates, the rollout contract was simplified: the source diff itself, not only the semantic artifact, must touch only `enableNAScore`. `cresta-testing-ca` was therefore isolated instead of retaining its two preservation fields. Current `master` also materialized The Zebra's `ai_poc` use case, but dry-run #34532229233 proved that syncing it would still remove one stored empty `summarization_config`; The Zebra was isolated again.

The final head `0e12472c31a8f763e0a55ed589c3dec9cf0cbbf9` changed 360 files with exactly 1,949 additions of `enableNAScore: true` and no source removals or other keys. Complete ConfigService artifact #34533387320 independently contains the same 1,949 flag additions and zero removals. All required checks passed. PR #153693 merged as `4264155cd9bb8bd2b1261068ceccc69c022cd1d2`; non-dry-run forward-sync run #34534431100 is queued. The safe cohort contains 1,963 effective profile/use-case scopes. `cresta-testing-ca` and The Zebra remain explicit patch-level gaps.

The merge-triggered run #34534431100 was cancelled by a newer `master` push before any regional sync step ran. The succeeding push run #34534865721 resolved the latest prior successful parent as `52890e59` and synced through head `25ed8522`, a commit range containing rollout merge `4264155c`. It completed successfully with `diff_only=false`; its full artifact includes all 1,949 expected `enableNAScore: true` additions and no flag removals. Production lifecycle advanced to **forward synced / read-back pending**.

Repository-dispatch fan-out then triggered multiple Sync From ConfigService runs, but cancel-in-progress left only final run #34536172681 active. It synced six production customers (`vivo`, `voices`, `wayfair`, `worldtravelholdings`, `xanterra`, and `yelp`) and auto-merged PR #153726. An initial explicit-line comparison counted six profile files and incorrectly treated omitted child values as missing effective scopes; the correction below supersedes that interpretation.

## Partial read-back recovery plan

The six-customer result has two separate failure signals. Repository-dispatch fan-out shared one concurrency group and cancelled all but the final run, explaining the narrow customer coverage. However, each surviving customer contributed only one confirmed scope and none is complete, so cancellation alone does not prove that ConfigService persisted every profile/use-case value from the successful forward-sync artifact.

The prior `enableCLOFilters` fleet rollout established the safe recovery sequence. Its green forward workflow did not match effective `configv3` state; the documented plan was a targeted read-back canary, an explicit non-dry-run forward replay if the canary was incomplete, another targeted read-back, and only then an explicit fleet replay. The actual follow-up manually replayed 354 eligible production customers successfully, though its final effective read-back was still pending in the preserved record.

For CONVI-7642, use manual `workflow_dispatch` rather than repository-dispatch fan-out. `sync_from_config_service.yaml` accepts an explicit customer list, environment, and branch name; its concurrency key includes the branch name for manual runs. Unique branch names therefore prevent the read-back batches from cancelling one another, and manual runs create review PRs without the repository-dispatch auto-merge behavior.

Recommended execution:

1. Use an inheritance-aware comparison: `configv3` intentionally omits a use-case field when it is identical to its profile parent, so an explicit profile value can cover all child scopes.
2. If a future comparison finds a genuinely missing or false effective value after inheritance, preview an explicit canary forward replay and persist only if the complete diff is flag-only, then read it back on a unique review branch.
3. Completion requires 1,963/1,963 safe-cohort scopes in effective read-back, no false values, exclusions unchanged, and no unreviewed unrelated flag changes. The separately isolated `cresta-testing-ca` and The Zebra gaps remain outside this safe cohort.

At 2026-09-11 00:06 UTC, repository-dispatch run #34544418123 read back `marriott-sbx` and auto-merged PR #153728. It added `enableNAScore: true` at the profile and omitted the four identical child values through normal `configv3` inheritance de-duplication. It also added unrelated `enableRegexKeywordMoment: true`.

Run #34546721969 later targeted `reprise` and auto-merged PR #153731. It added the flag at the profile, covering the child use case by inheritance, and also added `enableRegexKeywordMoment` plus 53 lines of browser-redaction configuration unrelated to this rollout. A repeated `marriott-sbx` read-back immediately beforehand produced no PR, confirming its effective state was stable.

Scheduled run #34547566828 then read back all environments and auto-merged PR #153732. The PR added 436 remaining `enableNAScore: true` profile values across exactly the remaining 352 safe-cohort customers. Together with the eight profiles from the earlier targeted PRs, current `configv3` contains the flag at all 444 expected profiles across all 360 safe-cohort customers. The other 1,519 intended scopes are use cases inheriting those profile values, so the exact inheritance-aware result is 1,963/1,963 true with no false values. No Comcast/Schwab identity was added by the flag read-back. PR #153732 contained substantial unrelated read-back drift overall (851 additions and seven deletions), but its 436 flag additions match the expected remaining safe-cohort profiles exactly. Lifecycle is **configv3 effective / Admin verification pending**; `cresta-testing-ca` and The Zebra remain separate gaps.
