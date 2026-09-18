# CONVI-6719: Unified parser implementation

**Status:** active
**Primary domain:** `analytics`
**Primary subdomain:** `insights-user-filter`
**Official ticket:** [CONVI-6719](https://linear.app/cresta/issue/CONVI-6719/phase-3-unified-parser-implementation)
**Last updated:** 2026-09-08

## Objective and Impact

- **Objective:** Implement the shared user-filter `Parser.Parse` without changing established analytics, coaching, ACL, role, group, hierarchy, active-state, or query-shaping behavior.
- **Customer/system impact:** This is the implementation layer intended to replace both `ParseUserFilterForAnalytics` and the older shared coaching parser. A semantic mismatch can broaden or narrow analytics visibility and can change leaderboard membership.
- **Role:** investigated and designed

## Scope

**In scope**

- Revalidate current `ParseUserFilterForAnalytics`, the older shared parser, and role-sensitive adjacent call paths before implementation.
- Correct Phase 3 assumptions and add missing regression coverage before porting behavior.
- Implement the shared parser only after the population-filter strategy is explicit.

**Non-goals**

- No product-code change during the 2026-09-08 validation pass.
- No caller migration, feature flag, shadow rollout, or removal of either existing parser in Phase 3.
- Do not revive canceled CONVI-7300/7301/7343 behavior implicitly.

## Source Context

- **Repos:** `go-servers` (primary), `cresta-proto` (current `ListUsersForAnalyticsRequest` capabilities)
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers` and `/Users/xuanyu.wang/repos/cresta-proto` for read-only validation; no CONVI-6719 worktree yet
- **Branches:** inspected `go-servers` `origin/main` at `782e72ba518f2ff817a5bca121aa5619f0b035e2`; local `main` is an ancestor but has no relevant diff in the inspected files
- **PRs/commits:** Phase 2 #26451 / `1c6f666a`; role-sensitive manual-QA change #28200 / `1c2c84e9`; audience reuse #28530 / `4fc30176`; dead-argument cleanup #29407 / `23d7af64`; user-filter performance change #30197 / `20243e10`; user-id narrowing #31489 / `c8cdec4e`

## Current Understanding

Phase 2 is merged and defines `Parser.Parse`, `ParseOptions`, and `ParseResult`, but `shared/user-filter/parse.go` does not exist and there are no new-parser callers. The existing analytics parser remains the production reference and now has 18 direct `retrieve_*` call sites plus the scorecard-reviewer-audience helper.

The prior Phase 3 plan cannot be implemented as a mechanical port. Analytics population filtering uses `ListUsersForAnalytics` with `AgentOnly` and inactive-user semantics. Coaching uses public `ListUsers` with `Roles`, `GroupRoles`, `UserTypes`, and `State`. Those filters are not equivalent, and the existing Phase 1 analytics suite does not prove the coaching/role contract.

## Findings and Decisions

- **Decision (2026-09-08):** The unified parser must support `Roles`, `GroupRoles`, `UserTypes`, and `State` as real population filters. `Roles=[]` means no role restriction. These fields must not remain unimplemented compatibility placeholders.
- `ListAgentOnly` remains a distinct constraint rather than an alias for `Roles=[AGENT]`; conflicting or combined population options require explicit, tested semantics rather than silent precedence.
- The original plan's `LiteUser` field-mapping table remains correct for output and membership construction, but its conclusion that `LiteUser` supports all parser requirements was incorrect. The plan did not specify how unsupported eligibility filters would be enforced. The README, implementation plan, behavioral standard, Phase 1 review, and implementation considerations were corrected on 2026-09-08.
- The Linear description is stale: it says `ParseV2`, while merged Phase 2 renamed the interface method to `Parse`.
- The remembered role-filter change is real but is not part of `ParseUserFilterForAnalytics`. CONVI-6930 added an optional `roles` filter to `getUsersFromUserAndGroupNames`; it applies only to users expanded from groups, while explicitly named users are always preserved. Agent audience resolution passes `[AGENT]`; reviewer audience resolution passes no role filter so non-agent reviewers remain included.
- `ParseUserFilterForAnalytics` still exposes only `listAgentOnly`, not arbitrary roles. `AgentOnly` means users whose role array contains no non-agent role; it is not equivalent to `Roles=[AGENT]`.
- `ListUsersForAnalyticsRequest` now has `user_types` and `user_ids`, but the go-servers auth implementation currently applies `user_ids` and ignores `user_types`. The user-type implementation and analytics threading from CONVI-7343 were reverted, and the ticket was canceled. Phase 3 must not assume user-type pushdown works.
- Child-team expansion is already fixed through `shared.ListGroups`; the behavioral standard's statement that the analytics parser still has the bug is stale.
- Current analytics behavior additionally includes sorted per-user group-name lists, lower-allocation resource-name construction, reuse of converted final users, and a scorecard-reviewer audience using `listAgentOnly=false`.
- Three formerly documented unused positional arguments were removed. `hasAgentAsGroupByKey` remains and still affects output mapping; the new parser should replace that coupling with complete direct/all maps plus caller post-processing.
- CONVI-7300/7301 short-circuit work and CONVI-7343 user-type filtering were canceled. Their proposed behavior is design input, not a compatibility baseline.
- The current `user_ids` capability creates a possible bounded-fetch optimization, but it must not weaken the base-population intersection invariant or omit ACL/group-expanded users.

## Blockers and Dependencies

- Decide and document how one `Parser.Parse` supports both analytics `AgentOnly` semantics and coaching `Roles`/`GroupRoles`/`UserTypes`/`State` semantics. Silent precedence between the two modes is unsafe.
- Add regression coverage for coaching and role-sensitive behavior; the 62-case Phase 1 suite covers the analytics reference, not every option present in `ParseOptions`.
- Reconcile the canonical behavioral standard and Linear ticket with current code before implementation.

## Validation and Rollout

- Read-only source comparison completed against the 2026-09-08 `origin/main` snapshot.
- Linear issue state and related canceled work were verified through the Linear connector.
- Existing Bazel regression suites passed on the local checkout: `common_user_filter_test` and `user-filter_test` (2/2 targets, 385.915 seconds). The inspected files have no local-to-`origin/main` diff.
- No source files, branches, worktrees, commits, or Linear issues were changed.

## Next Actions

1. Update the Phase 3 ticket/plan to use `Parser.Parse` and require full support for `Roles`, `GroupRoles`, `UserTypes`, and `State` alongside the analytics options.
2. Add role/coaching and newer audience regression cases before implementing `parse.go`.
3. Define option validation/combination semantics, especially `ListAgentOnly` with `Roles`, and `ExcludeDeactivatedUsers` with `State`.
4. Create a dedicated CONVI-6719 worktree only when implementation begins.
5. Implement in reviewable slices while keeping caller migration out of Phase 3.

## Timeline

- 2026-09-08 — Revalidated Linear scope, current parser behavior, post-savepoint commits, role-sensitive manual-QA behavior, caller inventory, and current auth RPC capabilities. Evidence: `sessions/2026-09-08/codex-convi-6719-current-behavior-validation.md`.
