# CONVI-7383 NCLH production flag-readiness check

**Date:** 2026-08-11
**Source repo:** `/Users/xuanyu.wang/repos/go-servers-convi-7383`
**Branch/worktree context:** `convi-7383-clo-mv-flag`
**Related repos:** `flux-deployments`, `config`, `clickhouse-schema`, `cresta-proto`

## Question

Is it safe to enable `useConversationOutcomeMomentAnnotationMaterializedView` for NCLH after the production MVs and backfill were created, specifically given uncertainty about the deployed Insights Server version?

## Evidence

- [go-servers#30588](https://github.com/cresta/go-servers/pull/30588) merged on 2026-08-07 at commit `68c0fd7d957f2e429b3e6c87270cd98357726eff`.
- The current `flux-deployments` `origin/master` declaration for Insights Server `03-prod-main` uses image `main-20260809_072100z-022144a6`.
- Git ancestry verification succeeded: routing commit `68c0fd7d` is an ancestor of image commit `022144a6b5fcd69819d409a0c5dbe2290e7c4ceb`.
- NCLH is served from `us-east-1-prod`; that environment imports `apps/insights-server/releases/03-prod-main`.
- The us-east production CLO Phase B backfill previously validated 68 non-empty databases with 317,036,293 expected keys and 317,041,632 target rows; no target was behind.
- [config#151310](https://github.com/cresta/config/pull/151310) enables the flag for NCLH and NCLH sandbox profiles/use cases only. The PR was still open during this check.
- Live Kubernetes verification could not be completed because the local AWS SSO session for `us-east-1-prod_ro` had expired. The conclusion therefore verifies current GitOps desired state and release ancestry, not the running pod image.

## Conclusion

It is safe to merge the NCLH flag PR with respect to Insights release compatibility: the production-main image declared for NCLH includes the flag consumer and CLO-MV routing implementation. The remaining operational caveat is to confirm Flux has reconciled and the live `us-east-1-prod` Insights deployment is on `main-20260809_072100z-022144a6` or newer; an older server would ignore the newly introduced config field rather than route to an incomplete table.

After merge, validate one representative NCLH start-time CLO query for correctness and reduced reads. End-time requests intentionally retain the raw-table path.

## Existing flag-off performance baseline

NCLH has a preliminary production baseline from 2026-07-28 for `nclh_us_east_1`, a six-month boolean-true CLO filter, and scorecard template `019dda54-f49a-77b2-a405-32e65391f4a3`:

- closest controlled full query: 85.830s with CLO versus 66.667s without CLO;
- CLO query reads: 2,764,516,920 rows / 349,162,856,860 bytes;
- no-CLO query reads: 272,919,872 rows / 46,599,372,822 bytes;
- direct raw CLO annotation component: 17.829s, 2,017,300,452 rows, 245,367,440,892 bytes;
- repeated CLO page-window queries: median 70.327s, p90 76.695s, maximum 85.830s.

This is useful historical evidence but not a clean flag-off/flag-on pair because it was collected two weeks earlier under potentially different cluster load and code. Capture the exact NCLH request again immediately before enablement if possible.

No equivalent Holiday Inn flag-off CLO performance baseline was found. Capture representative Holiday Inn start-time CLO requests before enabling. Do not treat the legacy `holidayinn_chat` database as safely enabled: it has storage + distributed table only, no trigger MV, because its source lacks `moment_annotation_payload`.

Config [#151311](https://github.com/cresta/config/pull/151311) does not include the legacy chat profile. Its 11 flag additions map only to `club-voice`, `owners-voice`, `social-voice`, `transfers-voice`, and `voice` at profile and selected use-case scopes.
