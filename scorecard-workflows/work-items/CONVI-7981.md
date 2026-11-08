# Delta organization targets report a missing template

**Status:** In Progress in Linear — reopened for backend nonempty-scope validation and PR; Delta inventory remains clean
**Primary domain:** scorecard-workflows
**Primary subdomain:** template-authoring-and-versioning
**Official ticket:** [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-enforce-template-use-case-validation-in-backend-apis) — High priority, In Progress as of 2026-11-06; [oncall thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1793944533660419)
**Last updated:** 2026-11-06

## Objective and impact

Diagnose Delta organization-target creation failures ahead of a reported Monday demo. Role: diagnosed and created the requested follow-up ticket. The user emphasized that backend APIs must enforce data correctness independently of frontend defaults, since scripts, imports, AI agents, and other clients can call them directly.

## Source context

Primary repo `/Users/xuanyu.wang/repos/go-servers`, main checkout inspected using fetched origin/main `b013c5ae3b`. Related Director `7fcbe35c81` and cresta-proto `3e804086cf`. Backend implementation: `/Users/xuanyu.wang/repos/go-servers-convi-7981-template-usecases`, branch `xw/convi-7981-template-usecase-validation`, base `ea73b7a535`, commit `5f4331c58a`. [PR #35339](https://github.com/cresta/go-servers/pull/35339) is open, not merged/deployed.

## Current understanding

[PR #35339](https://github.com/cresta/go-servers/pull/35339) adds the shared Create/Commit guard rejecting omitted/empty usecase_names with InvalidArgument. Both API suites pass (30 JUnit tests, zero failures/errors); API regressions assert rejected writes leave stored rows/scope unchanged. Gazelle and CI-version lint pass. Delta’s post-repair inventory is clean across all 4 templates / 17 revisions; the following investigation details describe the earlier incident state.

Expanded inventory: Delta has four templates / 17 revisions. POC Template and Customer Experience are the only two with empty scope; all their five combined revisions have descriptive non-UUID criterion IDs. Their oldest stored revisions were created about 695 ms apart by the same creator e2f03fc79f3250aa. The two older Delta Care templates have populated scope and UUID criteria across all 12 revisions. POC criterion IDs match none of the other templates' historical IDs. A common setup/API operation is plausible; copying or a particular client is not established. [Comparison evidence](../sessions/2026-11-06/codex-delta-template-comparison.md).

Generic CreateTarget creates an organization target when coachingPlanName is absent. Template `01a1a9fc-4b07-70c7-ae85-ae2ded1a923a` exists under delta-air-lines/us-west-2. All three revisions contain both reported criterion IDs, have empty usecase_ids, and have NULL deletion/deactivation timestamps. The target lookup excludes all three because it requires overlap with synthetic-care or all-usecases. Template listing explicitly includes empty arrays, so listing and creation disagree. Ordinary soft deletion is not the cause here.

## Validation and limitations

Live Chrome follow-up: Delta's user tab was editing the existing POC Template (scope empty), whereas Walter's was a new-template form. A separate unsaved Delta fresh form displayed voice-care and synthetic-care correctly. The visible 0 is numeric array length rendered by JSX `&&`; a missing display-name lookup would instead leave a rendered blank field. Four React rendering checks passed. No Save clicked; original browser use-case selection restored and temporary tab closed. See the frontend session for details.

During the initial investigation, two read-only production metadata/predicate queries succeeded through the customer app DB skill and cresta-cli self-authentication. No AWS credential/profile or SSH key explicitly selected. Codex performed no customer-data writes or deployed API verification. The subsequent backend code change is in PR #35339. Slack screenshots were not independently viewed.

## Next actions

1. Migrate the confirmed incompatible shared-ingestion Opera setup client before deploying PR #35339: its Create payload omits scope. Clients preserving existing scope also require valid existing data. Then review/merge, deploy, and verify rejection of omitted/empty scope.
2. Verify Delta organization-target creation through the deployed API/UI; the read-only inventory only establishes that empty scopes are gone.
3. Separately assess lookup/listing error consistency, UI rendering, and the original authoring client. These are not implemented by the nonempty-scope PR.

## Timeline

- 2026-11-06 — Traced API and verified current production mismatch. [Session evidence](../sessions/2026-11-06/codex-delta-organization-target-not-found.md).
- 2026-11-06 — Follow-up source inspection confirms template Create/Commit permit empty scope and save it directly; UI new-template default is the selected use case, but existing empty scope is preserved. Original Delta authoring request remains unknown.
- 2026-11-06 — [Detailed frontend path investigation](../sessions/2026-11-06/codex-delta-template-frontend-path.md) found no fresh-create scope-loss path; six source-function/form-control checks pass. Ordinary copying can propagate an already-empty template; cross-use-case copying requires a destination. Production criterion identifiers differ from normal builder creation output. Original client/request remains unconfirmed.
- 2026-11-06 — Compared live user Chrome tabs and fresh unsaved Delta forms. Current Delta setup resolves both use cases; zero reproduces empty template scope through the JSX length condition. Browser session authentication only, no product writes.
- 2026-11-06 — [All-Delta template comparison](../sessions/2026-11-06/codex-delta-template-comparison.md) identified Customer Experience as a second empty-scope/descriptive-ID template created by the same actor within 695 ms of Agent Adherence. Other templates have nonempty scope and UUID IDs throughout history; zero POC IDs overlap other historical templates. Read-only inventory only.

- 2026-11-06 — Created [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-enforce-template-use-case-validation-in-backend-apis) with High priority in Backlog, after duplicate searches. Ticket centers backend invariant enforcement across all API clients; includes both affected templates, direct API coverage, existing-data handling, and secondary UI rendering. No product/customer-data changes.

- 2026-11-06 — Prepared [operator-run synthetic-care repair SQL](../sessions/2026-11-06/delta-template-synthetic-care-repair.sql) and writable connection command at the user’s request. Targets the five known empty-scope revisions of both affected templates; not executed. Operator commit and subsequent target-creation verification remain pending.

- 2026-11-06 16:02:58 UTC — Read-only post-repair inventory: all 4 Delta templates / 17 revisions now have nonempty scope; zero NULL/blank array elements. Codex performed no mutation. Exact assigned scope and successful target creation were not checked by this inventory.

- 2026-11-06 — Updated [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-resolve-organization-target-failure-caused-by-empty) with post-repair results and marked Done at the user’s request. Retitled to describe the Delta incident resolution. Closure covers incident/data remediation; exact assigned scope and target-creation replay were not checked. Backend/API/UI prevention recommendations remain explicitly unimplemented.

- 2026-11-06 — Reopened as In Progress at user request. Backend implementation in /Users/xuanyu.wang/repos/go-servers-convi-7981-template-usecases, branch xw/convi-7981-template-usecase-validation, base ea73b7a535. [Implementation session](../sessions/2026-11-06/codex-convi-7981-backend-validation.md).

- 2026-11-06 — Opened [backend PR #35339](https://github.com/cresta/go-servers/pull/35339) at commit 5f4331c58a. Four red cases reproduced; both Create/Commit suites pass (30 JUnit tests, no failures/errors), Gazelle succeeds with no BUILD diff, golangci-lint v2.13.2 reports zero new issues, and independent review found no correctness issues. Ticket remains In Progress; merge/deployment pending.

- 2026-11-06 — [Organization client audit](../sessions/2026-11-06/codex-convi-7981-client-compatibility.md) found a definite incompatible Create caller in shared-conversation-ingestion-pipeline Opera setup. It already computes use cases but omits them from template payloads. Names/descriptive criterion IDs are a strong lead for Delta origin, not execution proof. Other authored writers set destination scope or preserve existing scope. Proto unchanged per user decision; no client source changes. Client migration required before backend deployment.
