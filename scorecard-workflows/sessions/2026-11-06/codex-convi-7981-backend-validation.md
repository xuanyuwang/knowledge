# CONVI-7981 backend scope validation

- Date: 2026-11-06, America/Toronto.
- Primary source repo: /Users/xuanyu.wang/repos/go-servers.
- Worktree: /Users/xuanyu.wang/repos/go-servers-convi-7981-template-usecases.
- Branch: xw/convi-7981-template-usecase-validation, based on fetched origin/main ea73b7a535.
- Canonical work item: [CONVI-7981](../../work-items/CONVI-7981.md).

## Request and approach

User explicitly requested reopening CONVI-7981 as In Progress and creating a backend PR to add a nonempty-use-case check. Linear update succeeded. Reject omitted/empty usecase_names in the converter shared by CreateScorecardTemplate and CommitScorecardTemplate, returning InvalidArgument before persistence; preserve existing per-entry validation and wildcard handling. Add API-level regressions confirming rejected requests do not create template/revision rows or clear existing scope. This request is limited to backend scope validation; lookup error mapping, UI rendering, criterion ID formats, and production data remain outside this PR.

## Validation

Baseline Create/Commit suites started; regression work pending.

## Test results so far

Red run: `bazel test //apiserver/internal/coaching:coaching_test --test_filter='Test(Create|Commit)ScorecardTemplate$' --test_output=errors` compiled successfully and reproduced all four omitted/empty regressions (nil error instead of InvalidArgument). All other Create/Commit cases passed.

Added the nonempty scope guard at the start of newDBScorecardTemplateFromProto. First green attempt surfaced one existing happy-path revision-ID fixture that omitted usecase_names; populated it from the source template. Second green run passed both suites (JUnit reports Create 14 and Commit 16, zero failures/errors). API tests also assert zero rows created by invalid Create and no new revision/scope changes on invalid Commit.

`bazel run //:gazelle` succeeded without BUILD changes; `git diff --check` passed. Independent read-only code review found no correctness issues. `mage ci:affectedServices origin/main` selects apiserver via workflow_call. Installed golangci-lint v1.64.5 was built with Go 1.24 and cannot lint this Go 1.27.1 repository; building CI's v2.13.2 into a temporary tools directory for a compatible lint run. No shared global tools changed.

## Delivery

CI-version golangci-lint v2.13.2 built with Go 1.27.1 completed with zero new issues using `--new-from-rev=origin/main`; the global v1 tool was not modified. Committed as 5f4331c58a and pushed xw/convi-7981-template-usecase-validation. Opened [PR #35339](https://github.com/cresta/go-servers/pull/35339) against main and attached it to the chat and CONVI-7981. GitHub confirms OPEN, non-draft; source worktree is clean. Linear confirms In Progress. PR has not been merged or deployed.

The check is enforced in the shared write converter, so only Create/Commit semantics change; existing read behavior and proto dependency remain unchanged. Existing clients committing templates must include at least one use case. Other lookup/error/UI recommendations remain outside this PR. External access used self-authenticating git, gh, and Linear connector programs. No AWS/Okta/Azure/SSH credential or profile was explicitly selected/read; no customer DB connection/write in this implementation turn.
