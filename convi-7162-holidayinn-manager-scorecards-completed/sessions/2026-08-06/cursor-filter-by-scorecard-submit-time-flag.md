# Session: Guard Manager submit-time behind feature flag

**Date:** 2026-08-06
**Tool:** Cursor
**Worktree:** `/Users/xuanyu.wang/repos/director-convi-7162`

## Goal

Guard CONVI-7162 Manager Leaderboard submit-time filtering with feature flag `filterByScorecardSubmitTime`.

## Changes (director, uncommitted)

1. Renamed local FE API surface to match proto:
   - `ScorecardTimeBasis` → `TimeRangeFilterTarget`
   - `scorecardTimeBasis` → `timeRangeFilterTarget`
   - String enum values match web-client-lite / proto JSON names (`TIME_RANGE_FILTER_TARGET_SUBMIT_TIME`, etc.)
2. Added `filterByScorecardSubmitTime` to `packages/director-app/src/types/localFeatureFlags.ts`.
3. Wired flag at call sites:
   - `ManagerLeaderboardPage.tsx` (QA score stats + daily time-range stats)
   - `useManagerScorecardTemplateBreakdown.ts` (drawer conversations)
4. When flag is off, requests omit `timeRangeFilterTarget` (backend default = interaction / `scorecard_time`).

## Notes

- `@cresta/web-client` still at `2.15.65` (no generated field); local enum remains in `director-api` insights types until web-client bump.
- Flag is not yet in config schema; enable via customer `featuresConfig.featureFlags` or URL/local override `?filterByScorecardSubmitTime=true`.
