# CONVI-7162 staging release investigation

**Date:** 2026-08-10
**Primary source repo:** `/Users/xuanyu.wang/repos/director`
**Context:** post-merge release and rollout verification; no source changes

## Question

All CONVI-7162 PRs were merged, but staging QA requests did not include `TIME_RANGE_FILTER_TARGET_SUBMIT_TIME`.

## Findings

- Proto PR [#9475](https://github.com/cresta/cresta-proto/pull/9475), backend PR [#30635](https://github.com/cresta/go-servers/pull/30635), and Director PR [#21534](https://github.com/cresta/director/pull/21534) are merged.
- The Director merge commit `d51b8ee3e32ae602fb8b003d9f113898958ee280` added the request helper and guarded both Manager Leaderboard request paths with `useFeatureFlag('filterByScorecardSubmitTime')`.
- The merge commit's own main build was cancelled by newer main activity, but later successful main build `31435169711` included the commit and published `/director` and `/director-beta` at 2026-08-10 22:18 UTC.
- Director release run `31442771206` published staging successfully. Flux PR [#316770](https://github.com/cresta/flux-deployments/pull/316770) deployed `main-ea98741` to 01-staging and merged at 2026-08-10 23:47 UTC; the staging publish job completed at 23:50 UTC. Commit `ea987415b3` contains `d51b8ee3e3`.
- The config schema flag was added by config PR [#150988](https://github.com/cresta/config/pull/150988). The automated Director schema sync PR [#21500](https://github.com/cresta/director/pull/21500) moved the flag from local-only type declarations into generated `SchemaFeatureFlag`.
- Current `config/origin/master` has no `filterByScorecardSubmitTime` value anywhere in staging customer config, including `configv3/staging/cresta/walter-dev/config.yaml`.
- `useFeatureFlag` returns true only for a URL/local-storage override or a truthy customer config feature flag. Therefore the deployed code evaluates the flag as false and leaves requests unchanged.
- Backend `insights-server` built and released successfully after merge; voice-staging tracks the newer `00-head` image, so backend availability is not the reason the request omits the field.

## Conclusion

The missing request field is not a release failure. The Director and backend changes reached staging, but the rollout flag was declared without being enabled for `cresta/walter-dev`. Enable `featuresConfig.featureFlags.filterByScorecardSubmitTime` for the staging profile, or use `?filterByScorecardSubmitTime=true` for an immediate browser-local verification.
