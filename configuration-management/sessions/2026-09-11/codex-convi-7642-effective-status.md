# CONVI-7642 effective status verification

Source repo: `/Users/xuanyu.wang/repos/config`

Reference: current fetched `origin/master` at `b19f73bda08927b22a1e92ef47d6f4726a78fa60`.

An exact comparison used the 29 staging customer-history files changed by PR #153660 as the rollout manifest. Those files currently contain 48 profiles and 128 use cases. Current `configv3/staging` contains `enableNAScore: true` at 47 expected profile files; no staging frontend config contains `enableNAScore: false`. Child values identical to a true profile parent are omitted by normal `configv3` inheritance.

The only missing profile is `configv3/staging/customerg/us-west-2/frontend.yaml`. Its two expected use cases are `retention` and `sales`; neither has a separate read-back frontend file, so both remain effectively absent with the profile. The corrected staging result is 173/176 effective scopes across 28/29 customers, with no false values.

Production remains 1,963/1,963 effective scopes for the 360-customer safe cohort in `configv3`, with Admin verification pending. `cresta-testing-ca` and The Zebra remain isolated rollout gaps.

The user subsequently used Admin to generate Zebra PR #153777. Its source diff contains six `enableNAScore: true` additions and one `summarizationConfig: {}` preservation entry; the complete PR dry-run artifact contains only the six effective flag additions. The PR merged as `4385ddaeebf975a3f36338c70f63c86942aa7f73`. Direct forward runs were cancelled by newer pushes; succeeding non-dry-run run #34609424358 succeeded at a head four commits after the Zebra merge, and its artifact contains exactly the six requested Zebra flag additions with no Zebra summarization change. `configv3` read-back remains pending. The monitor treats successful Zebra read-back as completion of the requested production customer cohort at 1,969 effective scopes. `cresta-testing-ca` remains an internal test identity outside that customer-completion statement unless explicitly brought into scope.

Full production read-back run #34611200691 completed after the Zebra forward sync and auto-merged PR #153783. That PR contained only one unrelated deletion and no Zebra file. GitHub code search against current `master` still finds no `enableNAScore` under `configv3/prod/the-zebra`. Because the missing profile value cannot be explained by child inheritance de-duplication, Zebra is classified as forward synced / read-back diverged. The next safe action is a targeted Zebra persistence/read-back repair rather than another broad retry.

Scheduled read-back run #34621026458 later auto-merged PR #153802 as commit `53260c311b5bafa0f7de4dbf48300919b815301a`. The PR added `enableNAScore: true` at `configv3/prod/the-zebra/us-west-2/frontend.yaml`; the five Zebra use-case frontend files contain no explicit value and inherit the true profile value. Production customer coverage is therefore 1,969/1,969 effective scopes across 361 included identities, with no false values and Comcast/Schwab exclusions preserved. PR #153802 contained unrelated read-back drift elsewhere, but its only `enableNAScore` change was the expected Zebra profile addition.
