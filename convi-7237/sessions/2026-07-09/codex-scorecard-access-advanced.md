# CONVI-7237 Scorecard Access Advanced

Source repo: `/Users/xuanyu.wang/repos/director-convi-7237`
Branch: `convi-7237-performance-config-template-scorecard-access-tab-advanced`

## Notes

- Original issue: Advanced section in Performance Config template builder had inconsistent alignment/font when `disableEditingOnSubmittedScorecards` is enabled.
- Initial approach used `DirectorFormInputWithLabel`, but that moved the description under the left label.
- Revised approach keeps `PermissionInputWithLabel` for the Scorecard editors row and updates its `split` variant to align with surrounding row labels:
  - `gap="xxl"` instead of `gap="xl"`
  - `miw={120}` instead of `miw={160}`
  - `Scorecard editors` label uses `fw={550}` and default Text size instead of `fz="sm"`
  - Right-side question/description uses `fw={500}` and stays above the dropdown
- Split-variant descriptions now render a trailing parenthetical in secondary text without changing translation JSON content.
- `Require publish` now uses centered label alignment so the toggle and label align vertically.
- Locale JSON copy changes were reverted; no JSON file remains in the working diff.

## Verification

- `yarn lint:precommit` from `packages/director-app` passed.
- Root `yarn lint:precommit` printed Lerna/Nx success but returned status 1, so package-level lint was used as the reliable verification.
- Browser screenshot of the opened page confirmed the Advanced section styling updates.
