# CONVI-7281 Director review fixes

**Date:** 2026-09-28
**Source repo:** `/Users/xuanyu.wang/repos/director-convi-7281`
**PR:** [director#23164](https://github.com/cresta/director/pull/23164)
**Commit:** `b8443e9e5a`

## Outcome

Addressed two valid review comments and the related CI expectation failure:

- Policy hydration now preserves every specified source type and normalizes only missing/unspecified legacy values to Coach Builder. AI Agent requirement policies therefore remain read-only and save with their original source. Editability also prefers the authoritative persisted `originPolicy.sourceType` over a stale frontend value.
- The shared active/inactive Opera policy metadata query now includes Training Simulator policies. `useAutoQATriggers` can therefore resolve `rolesWithVisibility` for their behaviors instead of treating missing policy metadata as unrestricted.
- Updated the Opera store draft-tracked property expectation from 19 to 20 for `policySourceType`.

## Validation

- Director app: 5 focused test files / 35 tests passed.
- Director API: 1 focused test passed.
- Director app and API TypeScript builds passed.
- Targeted typed ESLint, Biome, formatting, i18next lint, and diff checks passed.
- The broad precommit lint hook still hits the unrelated Zendesk schema TypeScript stack overflow; only that hook was skipped after targeted lint passed.

The commit was pushed over explicit HTTPS. GitHub reports the PR mergeable and fresh CI is running.

## Production-build CI diagnosis

The failing `Build / Build Production` job is unrelated to CONVI-7281. The PR diff contains no Receptionist files. The bundle comparator reports Receptionist entry `index` increasing from a 0-byte baseline, while the downloaded main baseline actually contains a 1,101,360-byte entry named `main`; total Receptionist assets decrease by 14.1 KB. Director main added the multi-customer Receptionist build after the PR's `d4c5c87d5d` base, changing the bundle entry layout. The older branch build and newer main artifact therefore use different entry keys, producing a false 1,075.6-KB increase. Refreshing the branch onto current main is the appropriate remedy; no CONVI-7281 bundle change is indicated.

## Credentials

Used GitHub CLI authentication over HTTPS. No AWS, Okta, Azure, or SSH credentials were used.
