# CONVI-6862: General availability for submitted-scorecard editing restrictions

**Status:** rolling out
**Primary domain:** `scorecard-workflows`
**Primary subdomain:** `permissions-and-visibility`
**Official ticket:** CONVI-6862
**Last updated:** 2026-08-10

## Objective and Impact

- **Objective:** Roll out submitted-scorecard editor restrictions to remaining production profiles through `disableEditingOnSubmittedScorecards`.
- **Customer/system impact:** Profiles enabled through configuration expose and enforce submitted-scorecard editor restrictions in Director and apiserver.
- **Role:** implemented and coordinated

## Scope

**In scope**

- Enable `disableEditingOnSubmittedScorecards` for the remaining production profiles in each deployment region.
- Keep frontend and backend rollout synchronized through the shared customer-config flag.

**Non-goals**

- Change submitted-scorecard editor semantics or scope.

## Source Context

- **Repos:** `director`, `go-servers`, `config`
- **Worktrees:** `/Users/xuanyu.wang/repos/director-ga-submitted-scorecard-lock`, `/Users/xuanyu.wang/repos/go-servers-ga-submitted-scorecard-lock`, `/Users/xuanyu.wang/repos/config-remove-submitted-scorecard-flag`
- **Branches:** `xwang/ga-submitted-scorecard-lock`, `convi-ga-submitted-scorecard-lock`, `convi-remove-submitted-scorecard-flag`
- **Active PR:** [config#151187](https://github.com/cresta/config/pull/151187)
- **Closed superseded PRs:** [director#21556](https://github.com/cresta/director/pull/21556), [go-servers#30861](https://github.com/cresta/go-servers/pull/30861), [config#151019](https://github.com/cresta/config/pull/151019)

## Current Understanding

The feature remains guarded by the shared frontend/backend customer-config flag. Regional GA is being completed by setting the flag to `true` for remaining production profiles.

## Findings and Decisions

- Frontend-only GA would be unsafe because backend write enforcement used the same flag.
- Separate repository PRs are required and cross-linked.
- The initial code-level override and complete-removal approaches were superseded by staged config rollout.

## Blockers and Dependencies

- The runtime PRs should deploy together; the config cleanup can follow once clients no longer consume the flag.
- Local `mage Lint apiserver/internal/coaching/scorecards` is blocked because golangci-lint was built with Go 1.24 while the repository targets Go 1.25; CI should provide the supported toolchain.

## Validation and Rollout

- Director unit tests, precommit lint, and TypeScript checks pass.
- Backend evaluator unit tests, permission API integration tests, update/reset integration tests, `go vet`, and Gazelle pass.

## Next Actions

1. Review and merge the remaining regional config rollout PRs.
2. Verify no production profiles remain disabled before considering future flag retirement.

## Timeline

- 2026-08-06 — Replaced the temporary override direction with complete flag removal across runtime and canonical config. Evidence: `sessions/2026-08-06/codex-ga-flag-override.md`, [director#21556](https://github.com/cresta/director/pull/21556), [go-servers#30861](https://github.com/cresta/go-servers/pull/30861), [config#151019](https://github.com/cresta/config/pull/151019).
- 2026-08-10 — Closed the flag-removal path and returned to staged configuration rollout; opened [config#151187](https://github.com/cresta/config/pull/151187) for 44 us-west-2 production profiles.
