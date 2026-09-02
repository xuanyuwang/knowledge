# Release and feature-flag verification

## Trigger

Linear comment `380247f9-7c51-46a3-8415-89f5febdc9bb` reports that Cliff Hawker still showed one completed scorecard on 2026-08-17 despite two evaluations.

## Release evidence

- Director PR `cresta/director#21534` merged as `d51b8ee3e32ae602fb8b003d9f113898958ee280` on 2026-08-07.
- The `2026-08-13` Director tag points to `a115dd918f53ff6c86c4f64790933d11f8abe59d` and contains the frontend fix commit.
- That exact release commit deployed to `02-prod-early` on 2026-08-10 and `03-prod-main` on 2026-08-12.
- Go backend PR `cresta/go-servers#30635` merged as `6d2c1d552cba4a35c1e4de75579ce78b84f367dd` on 2026-08-06.
- Go deployment SHAs from 2026-08-07 onward contain the backend fix; the later 2026-08-15 prod-early SHA is 245 commits ahead and zero behind it.
- Linear's issue is marked Released, but its linked `Director 2026-08-13` release record remains stale in the Planned stage. GitHub tag and deployment evidence are authoritative for actual rollout.

## Feature-flag evidence

- The frontend change only adds `timeRangeFilterTarget=SUBMIT_TIME` when `filterByScorecardSubmitTime` is enabled.
- The flag's documented default is `false`.
- Current config search finds no enabled override anywhere in `cresta/config`.
- Current Holiday Inn production frontend configs for `transfers-voice`, `voice`, `club-voice`, and `owners-voice` omit `filterByScorecardSubmitTime`.
- The `transfers-voice` config was updated as recently as 2026-08-18 and still omits the flag.

## Conclusion

The release is deployed, but the fix is not active for Holiday Inn because the feature flag is off. The new 2026-08-17 report is consistent with the old interaction-time behavior. Enable `filterByScorecardSubmitTime: true` for the intended Holiday Inn production profiles, deploy config, and then rerun the Cliff Hawker Aug 17 comparison.
