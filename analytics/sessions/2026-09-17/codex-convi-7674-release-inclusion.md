# CONVI-7674 Director release inclusion check

- Date: 2026-09-17
- Source repo: `/Users/xuanyu.wang/repos/director` (GitHub tree inspection; no local checkout mutation)
- Release: `release_director_2026-09-17-9c7412f`
- Resolved release commit: [`9c7412f7b7b3fa14ee6d1daf1515aa12fc9c2cdb`](https://github.com/cresta/director/commit/9c7412f7b7b3fa14ee6d1daf1515aa12fc9c2cdb)
- Fix: [Director PR #22772](https://github.com/cresta/director/pull/22772), merged 2026-09-15 as `f740a0fa253a6c304516502ac933387f09a1eb2c`

## Conclusion

The 2026-09-17 release does **not** include the CONVI-7674 fix.

## Evidence

1. GitHub's compare API reports the release commit and PR merge commit as diverged. The merge base is `ec60f5797d44a23f865f20e5bb174f17f0421c32`; the release is two commits ahead of that base and 36 commits behind the PR merge line. Therefore `f740a0fa` is not an ancestor of `9c7412f`.
2. The release tree still contains the pre-fix `modifyFiltersState` implementation. Its process-template branch clears conversation-derived filters but does not set `dateRangeTarget = undefined`.
3. The release tree still contains the pre-fix count-chart reconstruction. `unfilteredFilterState` calls `getInitialFiltersState` directly without the selected `scorecardTemplate` and without `normalizePerformanceFiltersState`.
4. Neither the fix symbol nor its two regression tests is present in the release versions of the four PR-touched files.

This checks both ancestry and effective source, covering the possibility of a cherry-pick or equivalent reconstruction under a different SHA.

## Rollout implication

RCG production will retain the state-dependent no-data behavior on this release. A later Director release must contain PR #22772's merged changes before production validation can close CONVI-7674.

## Why a PR merged before Thursday was excluded

Director uses a weekly release train rather than rebuilding Thursday production from the latest `main`:

1. The scheduled workflow runs Monday at 23:20 UTC, cuts the release branch from `main`, and assigns the following Thursday's release date.
2. That release branch is the fixed RC promoted through staging, prod-early, and prod-main during the week.
3. Ordinary PRs merged to `main` after the Monday cutoff do not enter the already-cut branch. Only explicit hotfix PRs targeting that release branch do.

For this train, the initial tag `director-2026-09-17` points to `ec60f5797d44a23f865f20e5bb174f17f0421c32`, committed at 2026-09-14 23:20:29 UTC, matching the Monday cutoff. PR #22772 merged to `main` at 2026-09-15 18:59:21 UTC, about 19 hours 39 minutes after the cutoff and after the initial release was published at 13:18:55 UTC that Tuesday.

The branch subsequently received two explicit hotfix PRs on 2026-09-16:

- #22870 produced `director-2026-09-17-1` at `f0f54e119116f0576ccd08f06d071610e2624f19`.
- #22883 produced `director-2026-09-17-2` at `9c7412f7b7b3fa14ee6d1daf1515aa12fc9c2cdb`.

That is why the Thursday build can contain Wednesday hotfixes yet exclude a normal PR merged to `main` on Tuesday. Without a deliberate backport/hotfix, PR #22772 belongs to the next weekly release train.

Existing GitHub CLI authentication was used. No AWS, Okta, Azure, or SSH credentials were used.
