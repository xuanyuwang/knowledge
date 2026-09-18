# CONVI-7642: Release enableNAScore to all customers

## Objective

Enable the existing Director frontend flag `enableNAScore` for every customer except Comcast and Schwab. Staging was initiated first; prior QA testing was accepted and production review is proceeding in parallel with staging read-back.

## Current status

**Production customer cohort is effective in `configv3`.** Staging PR [config#153660](https://github.com/cresta/config/pull/153660) merged and its forward sync covered all 176 planned changes. Current staging read-back has 47/48 profiles explicitly true; their 126 child use cases inherit true, yielding 173/176 effective scopes across 28/29 customers with no false values. Only `customerg/us-west-2` and its two use cases remain absent. Production PR [config#153693](https://github.com/cresta/config/pull/153693) completed the initial 360-identity safe cohort at 1,963/1,963 effective scopes. The Admin-generated Zebra follow-up [config#153777](https://github.com/cresta/config/pull/153777) forward synced all six requested additions through run #34609424358. After several read-backs initially omitted Zebra, scheduled run #34621026458 auto-merged [config#153802](https://github.com/cresta/config/pull/153802), adding the Zebra profile value to `configv3`; its five use cases inherit true. The requested production customer cohort is therefore 1,969/1,969 effective scopes across 361 included identities, with no false values. Comcast and Schwab remain excluded. `cresta-testing-ca` remains an internal test identity outside the user's customer-completion statement.

Linear status: **Done** as of 2026-09-11. The remaining staging `customerg` read-back discrepancy is documented but does not block the completed production release because prior staging QA was accepted.

## Staging scope and validation

- Explicit manifest: all 29 current staging customer identities.
- Exclusions: no Comcast or Schwab staging identities exist, so zero files were removed.
- Mutation: `enableNAScore: true` in all 176 existing profile and use-case frontend feature-flag maps.
- Exact diff: 29 files, 176 additions, no deletions, no production files, and no unrelated values.
- Validation: idempotent repeat dry run, `git diff --check`, and `yarn ajv-validate:configdb-frontend` passed.

## Source state

- Worktree: `/Users/xuanyu.wang/repos/config-convi-7642-prod`
- Branch: `xw/convi-7642-enable-na-score-prod`
- Merged commit: `4264155cd9bb8bd2b1261068ceccc69c022cd1d2`
- Linear: [CONVI-7642](https://linear.app/cresta/issue/CONVI-7642/release-enablenascore-ff-to-all-customers)

## Production scope and validation

- Inventory: 380 production identities at rollout resolution; 361 customer identities included, 18 Comcast/Schwab identities excluded by request, and `cresta-testing-ca` treated separately as an internal test identity.
- Effective production customer result: 1,969 profile/use-case scopes.
- Requested mutation: 1,955 explicit `enableNAScore: true` additions across the initial safe-cohort PR and Zebra follow-up.
- Five customers with mapless email use cases use a documented profile-only fallback; all 14 use cases inherit the profile value.
- Validation: exact structural comparison, idempotent updater dry runs, `git diff --check`, and `yarn ajv-validate:configdb-frontend` passed.

## Next gates

1. Repair and verify the remaining staging `customerg/us-west-2` profile and its two inherited use cases.
2. Treat `cresta-testing-ca` separately if the internal test identity is later brought into scope.

## Latest monitor check

At 2026-09-11, an inheritance-aware comparison against current `master` corrected the staging result. All 48 planned profiles and 128 use cases still exist. Forty-seven profiles explicitly contain `enableNAScore: true`, and their 126 child use cases inherit true; no staging scope contains an explicit false value. Only `configv3/staging/customerg/us-west-2/frontend.yaml` lacks the flag, leaving that profile and its two use cases absent. Staging is therefore 173/176 effective scopes across 28/29 customers, not the earlier explicit-line-based 83/176 count.

Scheduled production read-back run #34621026458 later auto-merged PR #153802, adding `enableNAScore: true` at the Zebra profile. All five Zebra use cases omit conflicting child values and inherit true. Production customer coverage is complete at 1,969/1,969 effective scopes across 361 included identities.
