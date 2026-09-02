# Global feature-flag rollout

## Objective

Enable the released Manager Leaderboard submit-time behavior for all production customers without relying on the Config Wizard's unreliable bulk update workflow.

## Implementation

- Created isolated config worktree `/Users/xuanyu.wang/repos/config-convi-7162-global-submit-time`.
- Branch: `xw/convi-7162-enable-submit-time-all-customers`.
- Followed the established Config Wizard output pattern: edit only `customer_config_history/prod/customers/*.yaml`, which is the ConfigService source of truth; allow post-merge workflows to regenerate `configv3` and legacy configs.
- Added `filterByScorecardSubmitTime: true` to every eligible flattened feature-flag map:
  - 354 customer files
  - 1,861 profile/use-case maps
  - 13 Holiday Inn maps
- Excluded Schwab and Comcast because they use separate release flows:
  - 17 related production/internal customer config files
  - 117 profile/use-case maps left unchanged
- Commit: `deb1df18ef577569671dc58979f9a8981ab046d0`
- Exclusion follow-up: `63ad6e1a9d`
- PR: https://github.com/cresta/config/pull/151886

## Validation

- `yarn ajv-validate:configdb-frontend`: passed.
- Targeted production-history YAML validation: passed.
- Mechanical invariant check: 1,861 eligible maps enabled exactly once; all 117 excluded maps unchanged; no duplicates.
- `git diff --check`: passed.
- `yarn test:ci`: reached an unrelated pre-existing `configv3/staging/e2e-target-gcp/profile-1/voice-support/frontend.yaml` validation failure (missing app `type` and unsupported `knowledge-assist-side-bar`); the PR does not modify that file.

## Next

Merge PR #151886, verify ConfigService and generated-config workflows complete, then rerun the Holiday Inn Cliff Hawker Aug 17 comparison.

## Master merge conflict resolution

- Fetched and merged `origin/master` on 2026-08-19.
- One simple conflict occurred in `customer_config_history/prod/customers/cresta.yaml`: master deleted the obsolete `cresta-compass-agent` profile while the rollout branch had added the flag inside it.
- Preserved master's deletion; no conflicting intent required escalation.
- Added the flag to 11 newly introduced eligible maps across six files.
- Merge commit: `f7fd02e28d05c7793d60fd5157610a401ac42148`.
- GitHub reports the PR mergeable; required checks are queued.
