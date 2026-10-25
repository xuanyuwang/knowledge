# CONVI-7777 standalone Director proto client bump

- Source repo: `/Users/xuanyu.wang/repos/director`.
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`; branch `xw/convi-7777-bump-web-client`, forked from fetched main `9195a299c2`.
- Request: separate cresta-proto dependency bump PR to unblock implementation PRs #24019/#24020. Preserve those branches.
- Current main already pins @cresta/web-client 2.32.47; implementation stack still pins 2.30.40. Generated CONVI-7777 API first appears in v2.32.15 and is present in v2.32.47.
- Latest published release observed through Renovate #23970 is 2.32.59. That existing PR's CI fails its old exhaustive TestCaseLLMSettingsModel map, but latest main already maps MODEL_GPT_6_LUNA to gpt-6-luna; no source compatibility edit is needed on current main.
- Plan: update all exact direct pins plus root resolution to 2.32.59, regenerate lock with Yarn, verify generated AnalyticsService API/types and package type checks, commit/push a separate draft PR with CONVI-7777 title. No flag/targeting, adapter migration, or existing implementation-branch edit.

## Execution

Created separate draft [Director #24065](https://github.com/cresta/director/pull/24065), base main, commit `2d7bf81c08`, branch `xw/convi-7777-bump-web-client`. Eight files only: root resolution, six exact workspace pins, Yarn lockfile. No product source or implementation-branch edits.

Installed actual published @cresta/web-client 2.32.59 via Yarn 4.18.1 under Node 24.11.1. Lockfile has only expected version/archive/checksum updates, no unrelated package churn. Peer warnings remain (legacy SDK web-client range plus existing ecosystem peers); install succeeds. Initial invocation used the older branch's Yarn 4.18.0 filename; reran with current main's bundled 4.18.1. A provisional runtime probe used a nonexistent class name; corrected to the actual exported AnalyticsService and confirmed its usage RPC is a function.

Verification: root `yarn tsc` passes all workspace type checks; full director-api suite passes 49 files / 323 tests; root `yarn check:deps` passes; runtime generated-service availability check passes; git diff check passes. Commit hooks pass or skip checks inapplicable to manifest-only changes. No new tests needed for a dependency-only edit. CI starting, no green CI or browser/integration claim. PR template checkboxes remain unchanged per commit-and-PR skill.

Important qualification: main's prior 2.32.47 already includes the required usage API. Refreshing the old 2.30.40 implementation branches onto current main independently addresses the original missing-contract gap. This PR provides a separately reviewable latest-client baseline, matching Renovate #23970's target while including newer main's existing enum mapping. Follow-up: merge/review bump, refresh implementation stack and replace temporary transport bridge using generated types in implementation scope; timezone-test/build blockers remain separate.

Credentials: git/gh self-authentication and repository-configured Yarn GitHub package authentication; no AWS, Okta, Azure or SSH credential/profile manually selected.


## Merge conflict follow-up

User requested conflict resolution on #24065. Fetched main `6b323f4e9f` and merged normally without rewriting history. Two overlapping adjacent-line conflicts in `packages/testkit-utils/package.json` and `yarn.lock`: kept web-client 2.32.59 from this PR and Playwright 1.64.1 from main. Yarn immutable install and diff checks pass; diff against current main remains exactly the original eight dependency/lock files, 17 insertions/deletions. Full workspace type check passes. Published merge commit `cf7566689c52fffa7461c639ee95b99ae6d965da`; GitHub confirms #24065 is MERGEABLE (merge-state BLOCKED by outstanding PR requirements), worktree clean. Existing git/gh authentication only; no manually selected restricted credentials.
