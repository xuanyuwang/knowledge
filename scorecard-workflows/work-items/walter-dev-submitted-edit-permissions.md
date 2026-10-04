# Walter-dev submitted scorecard edit permissions

**Status:** complete (diagnosis)
**Primary domain:** scorecard-workflows
**Primary subdomain:** permissions-and-visibility
**Last updated:** 2026-09-23

## Objective and scope

Explain why scorecard `customers/cresta/profiles/walter-dev/scorecards/01a0cf24-657c-757f-9a3e-8311ffc133c9` can still be edited despite a submitted-editor allowlist. Read-only database/config/user/permission queries; no changes or mutation replay. Role: diagnosed.

## Current understanding

The last updater `d634c90a8748bcb2` has SUPER_ADMIN. That role intentionally bypasses the allowlist. Live ConfigService confirms the flag is true, and EvaluateScorecardsPermissions returns allowed=true for this user/card. Latest template revision `98cfcf4c` matches the scorecard's pinned revision and explicitly lists only another user, `a07b887c425b17f4`.

## Source and validation

- Primary source/worktree: `/Users/xuanyu.wang/repos/go-servers`, main at `6e9999f0ad45c0e900c3860cbdb48ae2862fe40c`; no edits.
- Related sources: config, director, cresta-proto.
- Intentional exemption: [go-servers #31889](https://github.com/cresta/go-servers/pull/31889), CONVI-7612, commit `58b737c63b142f99ebced1f5897ec3ad8dce6009`.
- Live permission allowed for recorded updater and allowlisted comparison user; both are SUPER_ADMIN. No non-admin negative control performed.
- Live profile flag version: `183c59e8-f6db-5603-8283-939022a064bc`.
- Original update trace, historical roles/cache state, and exact deployed commit were not reconstructed; explanation is supported by live evaluation, current roles/config, and source authorization path.

## Timeline and evidence

- 2026-09-23 — Resolved expired AWS SSO, queried scorecard/template, then verified flag, evaluated permissions, and identified intentional SUPER_ADMIN exemption. [Detailed requests, responses, and findings](../sessions/2026-09-23/codex-walter-dev-submitted-edit-permissions.md).

## Next actions

No change needed to explain current behavior. A test of restricted ordinary users requires a suitable existing non-SUPER_ADMIN account outside the submitted-editor allowlist; do not change roles or scorecards to manufacture a fixture.
