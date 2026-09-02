# CONVI-7543 CLO filter GA rollout

**Date:** 2026-08-20
**Source repo:** `/Users/xuanyu.wang/repos/config`
**Worktree:** `/Users/xuanyu.wang/repos/config-convi-7543`
**Branch:** `xw/convi-7543-enable-clo-filter-all-customers`

## Inputs

- Linear [CONVI-7543](https://linear.app/cresta/issue/CONVI-7543/ga-enable-clo-filter-for-all-customers), whose title defines GA scope and has no additional description or comments.
- Prior pilot PR [config#151643](https://github.com/cresta/config/pull/151643) for NCLH and Holiday Inn.
- The established standard-production rollout scope from CONVI-7162, including Comcast and Schwab release-flow exclusions.

## Implementation

- Created an isolated worktree from `origin/master` to preserve an unrelated edit in the main config checkout.
- Added `enableCLOFilters: true` to every standard-production feature map selected by the established GA eligibility set.
- Existing pilot values were left intact and no config outside `customer_config_history/prod/customers` changed.
- Commit: `9c843eca8c`.
- PR: [config#152031](https://github.com/cresta/config/pull/152031).

## Coverage

- Eligible feature maps: 1,867.
- Already enabled: 36.
- New additions: 1,831 across 352 files.
- Explicitly excluded: 117 maps across 17 Comcast/Schwab config files.

## Validation

- Parsed every changed YAML file using Ruby Psych: 352/352 passed.
- Structural assertion: all 1,867 eligible maps have exactly one `enableCLOFilters: true`.
- Duplicate assertion: zero duplicate CLO keys in feature maps.
- Diff assertion: 1,831 additions, zero deletions, and only the intended flag value was added.
- `git diff --check`: passed.
- `yarn ajv-validate:configdb-frontend` could not start because the new worktree has no Yarn dependency state. No dependency installation was attempted.
- PR CI subsequently passed: ConfigService dry-run, schema proto, config consistency, JavaScript/Python YAML lint, credential scan, and Schwab no-change validation.

## Staging follow-up

- Production PR #152031 merged before the requested staging expansion was ready.
- Added `enableCLOFilters: true` to every staging feature map: 174 total maps, one already enabled, 173 additions across all 28 staging customer-history files.
- Rebased the staging-only change onto current `origin/master` after post-merge config regeneration, producing clean commit `92db84ab2f` on `xw/convi-7543-enable-clo-filter-staging`.
- Opened [config#152040](https://github.com/cresta/config/pull/152040).
- Parsed all 28 changed YAML files; verified all 174 staging maps contain exactly one enabled CLO flag; diff is 173 additions and zero deletions; `git diff --check` passed.
- At handoff, 18 PR checks passed with none failing; ConfigService dry-run, CodeRabbit, and the final credential scan remained pending.

## Security hygiene

- No AWS profiles, database credentials, or SSH key files were inspected.
- Existing Git and GitHub CLI authentication was used for fetch, push, and PR creation.

## Next

- Wait for config#152040 required checks and review.
- After approval, merge and verify staging ConfigService sync.
- Roll out through separate Comcast and Schwab release plans independently.
