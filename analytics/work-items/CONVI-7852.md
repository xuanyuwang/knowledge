# CONVI-7852: AdventHealth dual-role users shown under the wrong team

**Status:** Auth #35092 and analytics #35094 merged; Director inline integration published as draft #24670 at 0ec809a7ae. Local tests/static checks pass; testkit changes removed by user request; browser validation pending; CI and deployed validation pending
**Primary domain:** analytics
**Primary subdomain:** leaderboard
**Official ticket:** [CONVI-7852](https://linear.app/cresta/issue/CONVI-7852)
**Last updated:** 2026-11-05

## Objective and Impact

Explain the reported Agent Leaderboard team-selection problem with current source and customer membership evidence. The ticket reports leads across SER, MAR, GLR, RMR, and SWR. Role: investigated source and production auth data; identified the selection mechanism and separate ambiguous cases.

## Source Context

- Implementation source repo: `/Users/xuanyu.wang/repos/director`; worktree `/Users/xuanyu.wang/repos/director-convi-7852`, branch `xw/convi-7852-agent-team`, base `764bb8bf15`.
- Related source references: go-servers, cresta-proto, config.
- Director inline fix: [draft #24670](https://github.com/cresta/director/pull/24670), commit `9e42ac1f6e7cd331129f2ddf3075ea3a1abec1bb`. Backend PRs are merged; no deployed/customer verification or production mutation.
- Linear was verified In Progress on 2026-11-02; assigned to Xuanyu Wang.

## Current Understanding

Ticket-reported behavior: users who are Agents on their home team and Manager 2nd elsewhere can display under a Manager 2nd team. Sarah Young is the clean example: Agent on Brittni Howard, Manager 2nd once on parent Christina Gusby, still reportedly wrong after consolidation. Expected: Agent/home team for dual-role users; designated leader team for manager-only users.

Current source plus production memberships explain Sarah and Allison: the backend sorts all direct TEAM resource names by ID, and Director displays `attribute.groups[0]`. At investigation time analytics memberships lacked roles. Merged auth/analytics changes now return all supplied memberships with legacy roles inline; the Director draft consumes them for Agent display selection. Sarah therefore resolves to Christina Gusby rather than Brittni Howard; Allison resolves to Silvia Martin rather than Miranda Adcock. These labels are source/data predictions reproduced through actual Director transformers, not captured live UI/API values.

Read-only production auth checks found 33 active users with exactly one Agent team whose first sorted direct team differs from it (43 have one Agent team plus other direct teams; 25 have multiple Agent teams). These are potential mismatches, not proven visible rows in a selected date range. Gary and Stephanie have Agent on three teams; Anita has no Agent team and expects the parent of her managed team; Ana's current membership order predicts her Agent team correctly. See [full investigation](../deliverables/convi-7852-investigation.md).

## Findings and Decisions

- Read the full issue and its one comment through Linear MCP. The October 20 comment requests a long-term fix timeline and adds no technical diagnosis.
- The ticket says membership consolidation has not resolved Sarah's case; investigate team selection rather than assume tenant cleanup is sufficient.
- Distinguish display label selection from group filtering, metric attribution, totals, ranking, exports, and access control.
- CE-8071/CE-8075 are described by the ticket as separate access-control work; no evidence here establishes a shared cause.
- Read historical Slack text via Glean; current source has stable ID sorting introduced by #28217 in May 2026. The older shorthand “non-root” omits current ordering and Agent-path root/default behavior.
- Agent conversation stats are computed per user before teams are attached; the selected label also flows to CSV. Team rollups use all memberships, so changing shared membership mapping would have additional metric effects.
- The shared analytics converter continues to select the first group. The Director draft derives a response only in AgentLeaderboardPage, preserving shared cache data and other consumers. Deployed versions remain unverified.

## Approved Rule and Implementation — October 30

The user approved direct Agent-role TEAM preference in Leaderboard → Agent, choosing the lowest resource ID among qualifying teams. All other cases retain the existing first-team policy, including manager-only users. This supersedes the open reporting-team questions above; no manager hierarchy rule is added.

The initial public ListUsers lookup prototype is superseded by the November 5 inline integration. AgentLeaderboardPage derives its display response from RetrieveConversationStats users' groupMemberships. Candidate teams are intersected with each row's analytics groups; only the singular display group is replaced. Both Agent tables and existing CSV exports consume it. Metrics, group order, population, filters, ACLs and Team/Manager tabs retain their existing paths. Missing memberships/roles, indirect/non-TEAM/off-row candidates and custom-role-only identity retain the existing fallback. QA-only rows keep their prior rule.

## Next Actions

1. Review Director [draft #24670](https://github.com/cresta/director/pull/24670) and CI for `9e42ac1f6e`; do not merge without user direction.
2. Keep testkit unchanged as requested; perform authenticated browser validation when a test login is available.
3. Verify backend deployment and Agent labels/unchanged metrics and Team behavior against an authenticated deployed build. Backend merges alone do not prove rollout.

## Validation and Rollout

November 5 Director validation: 28 leaderboard tests across six files and 24 Insights API tests across three files pass. Tests cover inline converter preservation, direct legacy Agent selection, dual roles, deterministic ties, indirect/non-TEAM/off-row exclusion, custom-role/missing-role fallback, immutable groups/metrics, and existing Agent/Team/Manager behavior. App and testkit TypeScript, changed-file ESLint/Biome, root dependency checks, API/app Knip, diff checks and all commit hooks pass. Independent review found no actionable issues. The attempted Playwright run stopped in auth setup for a missing local token; testkit edits were subsequently removed at the user’s request; no browser/customer success is claimed. PR CI was queued at creation. Existing pinned web-client 2.33.35 exposes all required fields; no dependency upgrade needed. Details: [frontend session](../sessions/2026-11-05/codex-convi-7852-inline-frontend.md).

Earlier investigation included read-only production auth membership checks, not current live API/UI verification. This frontend session used git/gh and Yarn's existing authentication; no AWS/Okta/Azure/SSH credential or profile explicitly selected or read. No production data mutation.

## Timeline

- 2026-10-29 — Reviewed ticket and comment and saved the problem summary and proposed investigation. See [session](../sessions/2026-10-29/codex-convi-7852-investigation-plan.md).
- 2026-10-29 — Traced the full selection path, checked all six users and potential population in production auth, verified historical sorting change, and reproduced Sarah/Allison via Director transformers. [Report](../deliverables/convi-7852-investigation.md), [SQL](../sessions/2026-10-29/convi-7852-membership-checks.sql), [replay](../sessions/2026-10-29/convi-7852-team-label-replay.cjs).

- 2026-10-30 — Implemented approved Agent-team preference in an isolated Director worktree. Added selection and paginated lookup regressions; validation results are recorded in the [implementation session](../sessions/2026-10-30/codex-convi-7852-agent-team-fix.md).

- 2026-11-01 — Source review found conditional reuse through ALO users and audience-filter team-member roles; neither fully covers every analytics row. Current lookup remains unchanged. [Reuse investigation](../sessions/2026-11-01/codex-convi-7852-existing-membership-reuse.md).

## Analytics role metadata follow-up — November 1

Read-only investigation supports exposing direct per-membership old-system roles and custom role resource names in ListUsersForAnalytics. Global roles alone cannot choose an Agent team. The membership view intentionally returns empty arrays for indirect memberships; effective inheritance needs separate resolution. Existing Team aggregation credits every mapped team, so the approved Agent display preference must not change those totals.

The proposed backend path carries eligible candidate memberships through to per-agent public User responses, addressing both metadata drops and warm stats-cache responses, then lets Director select its display team without the extra public lookup. No source changes or replacement of the validated Director worktree. See the [source-backed recommendation and tests](../deliverables/list-users-for-analytics-roles-recommendation.md) and [session](../sessions/2026-11-01/codex-list-users-for-analytics-roles.md). Custom-role reporting designation and any future Team attribution policy remain product questions; Agent ties and fallback are already decided.

- 2026-11-02 — Confirmed proposed per-membership proto fields and exact auth projection/conversion changes. Existing database view supplies direct roles; no extra query or migration needed. Source remains unchanged. [Implementation investigation](../sessions/2026-11-02/codex-analytics-membership-role-fields.md).

## Protobuf PR — November 2

Created [cresta-proto #10550](https://github.com/cresta/cresta-proto/pull/10550) for direct legacy per-membership `AuthProto.Role` values (`roles = 3`); custom roles are deferred by user decision. Only the internal user proto changes. Backend population, Insights propagation and Director integration remain separate work. The existing Director worktree is preserved.

Commit `bd08570dcb20a781aca27149e5d3d58f6fba3689`, branch `convi-7852-adventhealth-leaderboard-shows-dual-role-users-agent-manager`, worktree `/Users/xuanyu.wang/repos/cresta-proto-convi-7852-membership-roles`. Buf checks, full proto/API lint, dependency/import/HTTP-path checks, formatting, Gazelle and Bazel user-package generation/build passed on preceding main revision `8507e6296cd2a1f06b4c92acaff3652a2c012d43`. Current main `7d51703ec3` already contains duplicate permission-feature enum value 91; the PR documents this CI limitation. See [PR session](../sessions/2026-11-02/codex-convi-7852-proto-roles-pr.md).

## Backend PRs — November 3

Merged cresta-proto #10550 supplies direct legacy roles only; custom roles are deferred. Auth [#35092](https://github.com/cresta/go-servers/pull/35092), commit 6a727f919d, reads role_id through the existing smart projection, preserves the public converter's malformed-string/unknown-number handling, leaves indirect memberships empty, and versions the logical cache key before hashing. Full nonpublic user-service test target passed.

Analytics [#35094](https://github.com/cresta/go-servers/pull/35094), commit 9fada718de, is stacked on the auth branch. It retains eligible membership metadata separately from query users and decorates only Agent+Group conversation response users with direct TEAM roles intersected with each row's existing groups. Decoration follows metrics cache storage and also runs on cache hits. Responses and role slices are cloned; group order, filters, ACLs, population, metrics and Team rollups remain unchanged.

Full parser (1.7s) and conversation-statistics (258.9s) test targets passed against local Postgres/ClickHouse. The final five affected conversation methods passed again (8.1s) after fixture simplification. Gazelle, dependency tidy/generation, formatting and diff checks completed. Both source worktrees are clean. Auth automated checks are green except pending code-owner approval at handoff; analytics service CI waits until the stacked PR is retargeted to main. Staging/customer verification and performance measurement were not performed. [Detailed record](../sessions/2026-11-03/codex-convi-7852-backend-roles-prs.md).

## Branch refresh — November 4

Rebased and atomically pushed auth #35092 at `cf06a739ba` and stacked analytics #35094 at `25086d9489` on main `555b5fe058`. Both inherit main's `v2.34.6-0.20261104130341-34bced4abda2`; auth no longer edits go.mod/go.sum. Auth's three-file feature patch is preserved; analytics remains the identical six-file consumer commit. Gazelle and diff checks pass, both worktrees clean.

Full auth target (16.9s), full parser target (1.5s), and all five affected conversation membership/cache/Team methods (6.4s with explicit verbose names) pass. Full conversation runs failed from local ClickHouse total-memory rejection and container OOM kill, not role assertions; no unrelated code changes were made. PR descriptions record these limits. New CI status is not yet verified. [Refresh details](../sessions/2026-11-04/codex-convi-7852-branch-refresh.md).

### Shared converter cleanup — November 4

After user review, exported `auth/shared/model.StringRoleIDsToRoles` and reused it for public user conversion and analytics membership roles, removing duplicate `analyticsMembershipRoles`. Body/behavior unchanged. Auth [#35092](https://github.com/cresta/go-servers/pull/35092) is now `76223191bb`; stacked analytics [#35094](https://github.com/cresta/go-servers/pull/35094) is `c30fca8d40` with identical consumer patch. Same full auth/model test targets pass before/after; Gazelle, formatting and diff checks pass. [Details](../sessions/2026-11-04/codex-convi-7852-shared-role-converter.md).

### Unconditional parsed metadata — November 4

User approved retaining membership metadata regardless of grouping while excluding it from metric cache identity. Analytics #35094 now at `f05c8789a1` builds the ACL-filtered `UserNameToMemberships` map unconditionally; FinalUsers remain membership-free and current response/cache behavior is preserved. New WithoutAgentGrouping case failed before the fix, then both grouping cases and full parser target passed (1.5s). Five focused conversation role/cache/Team tests passed (8.3s). Auth branch unchanged. [Implementation and evidence](../sessions/2026-11-04/codex-convi-7852-unconditional-memberships.md).

### Helper organization — November 4

User requested one-line comments and consolidating conversation_memberships.go into common_user_filter.go. Both helper bodies moved unchanged; old file removed and Gazelle updated its source list. PR #35094 now at `1d0fc06feb`. Full parser and five focused conversation tests pass (1.6s/8.0s). [Details](../sessions/2026-11-04/codex-convi-7852-membership-helpers-location.md).

### All response memberships — November 4

User approved returning all supplied user memberships and leaving displayed-team selection to Director. Analytics #35094 advanced to `86259f78ab`: removed row-group intersection/direct-TEAM filtering and the conversationUserMemberships helper, converted inline because no reusable public converter exists. Preserve group types, display names, order, direct/indirect markers, and cloned legacy roles. Existing Agent+Group enrichment timing, metric/cache identity, filters and Team rollups are unchanged. Updated regression failed before implementation; full parser and five focused conversation methods pass. PR description updated. [Details](../sessions/2026-11-04/codex-convi-7852-all-memberships.md).

### Custom-role propagation investigation — November 4

Read-only investigation identifies additive internal custom_roles=4, RoleIds smart projection/name conversion in auth, and analytics CustomRoles pass-through. Public proto, FE model/transformer, and exported single-role base resolver already exist. FE still needs inline metadata consumption and definitions covering other users' custom roles; current-user definitions and current role hook pagination/filter defaults are insufficient to guarantee coverage. Existing double-written proto roles remain usable; generic Base Role supplies no Agent/Manager identity. No source changes requested or made. [Full gaps/evidence](../sessions/2026-11-04/codex-convi-7852-custom-role-gaps.md).

### Custom-role protobuf PR — November 4

Opened [cresta-proto #10601](https://github.com/cresta/cresta-proto/pull/10601), commit `720c300b2a`, semver:minor, from current main `33f0f79da7`. Adds only internal UserGroupMembership.custom_roles=4 as directly assigned role resource names, empty for indirect memberships. Scoped Buf build/lint/breaking, full lint/API lint/dependency checks, import/HTTP checks, clang-format, Gazelle and affected Bazel Go build all pass; worktree clean. Auth/analytics/FE custom-role propagation remains pending. [Session](../sessions/2026-11-04/codex-convi-7852-custom-roles-proto-pr.md).

### Review follow-up and legacy functional identity — November 5

Auth #35092 is merged. User explicitly confirmed legacy/proto roles remain the accepted source for functional Agent/Manager/Manager2nd identity in a team, matching Sam's approval; prior custom-role base-inference proposal is not the implementation plan. Analytics #35094 now targets main; follow-up `ffbd308b56` clarifies intentional all-membership metadata and renames the off-row fixture. Addressed/resolved the sole open CodeRabbit thread with auth-scope and public indirect-membership evidence, preserving prior all-membership approval. Full parser (79 cases) and five focused conversation tests pass; Gazelle/format/diff checks pass; source clean. No proto/custom-role/Director changes or merge. [Session](../sessions/2026-11-05/codex-convi-7852-review-comments.md).

### CI recovery — November 5

All automated CI is green on unchanged ffbd308b56 after rerunning transient GitHub dependency-download HTTP 504 failures. BUILD lint and full Insights CI (lint/build/five shards/required) pass; no code edits or merge. Codeowner approval is separate. [Evidence](../sessions/2026-11-05/codex-convi-7852-ci-failures.md).

### Normal membership response caching — November 5

User approved caching memberships with the complete conversation stats response. #35094 is now 79a43980e7: membership conversion occurs inline in the existing Agent+Group row converter; each response user is cloned, filter users/cache keys remain unchanged, and deferred whole-response decoration is removed. Cache hits retain stored memberships until normal expiry. Full parser and five focused conversation tests pass; Gazelle/gofmt/diff checks pass. CI for this new head is queued, no merge. [Evidence](../sessions/2026-11-05/codex-convi-7852-membership-cache-simplification.md).

### Membership contract docs — November 5

Opened [cresta-proto #10620](https://github.com/cresta/cresta-proto/pull/10620) at a05b252c6f to document existing direct/indirect User.group_memberships and is_indirect. Proto lint/API lint/format/breaking/Gazelle/affected build pass; eight local FE selector tests pass. Responded to and resolved analytics review r4485910411 with current Director helper and unshipped-prototype distinctions; no runtime change. New proto PR CI pending. [Evidence](../sessions/2026-11-05/codex-convi-7852-membership-contract-docs.md).

### Inline Director integration — November 5

Published draft [Director #24670](https://github.com/cresta/director/pull/24670) at `9e42ac1f6e`. Refreshed the existing worktree to main `764bb8bf15` while preserving the prototype, replaced the extra user fetch with inline membership selection, and removed the lookup hook/tests. Existing generated/model/converter contracts already preserve roles and indirect flags, verified by regression tests. 52 focused tests and required local static checks pass; browser smoke blocked on missing test login, CI/deployed verification pending. [Session](../sessions/2026-11-05/codex-convi-7852-inline-frontend.md).

### Testkit preference — November 5

User requested avoiding testkit edits where possible. Pushed `0ec809a7ae` removing both testkit changes from Director #24670; retained the selector and converter unit tests. No production code changed in this follow-up.

### Live deployed reproduction — November 6

Configured Chrome Agent leaderboard for Sarah Young, AdventHealth Voice Care, Oct 6–Nov 6, Template All, all minutes, exclude voicemail Yes, scorecard-based calculation. Both deployed Agent tables display Christina Gusby, reproducing the reported symptom; prior expected Agent team is Brittni Howard. Current memberships and draft PR behavior were not revalidated. Filtered tab remains open. [Browser evidence](../sessions/2026-11-06/codex-convi-7852-chrome-reproduction.md).

### Local browser verification — November 6

Local branch `0ec809a7ae` connected to AdventHealth still shows Christina Gusby in both Sarah Young Agent tables with matching metrics. Tanstack query Data Explorer shows `groupedByAttributes.user.groupMemberships: []` and two row groups, so approved fallback is expected. Corrected-label verification remains blocked on populated membership metadata; backend rollout/cache cause not established. Local tab left open, no source/testkit changes. [Evidence](../sessions/2026-11-06/codex-convi-7852-local-browser-verification.md).
