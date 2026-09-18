# CONVI-6719 current-behavior validation

Date: 2026-09-08
Source repo: `/Users/xuanyu.wang/repos/go-servers`
Source context: local `main` at `95068db42b`; inspected `origin/main` at `782e72ba518f2ff817a5bca121aa5619f0b035e2` (2026-09-08)
Related repo: `/Users/xuanyu.wang/repos/cresta-proto`
Mode: investigation only; no product-code changes

## Question

Before starting CONVI-6719 Phase 3, verify that the saved behavioral model still matches current code, especially whether a newer role filter was added to `ParseUserFilterForAnalytics`.

## Sources reviewed

- Linear CONVI-6719 and comments (no comments)
- Linear CONVI-7300, CONVI-7301, CONVI-7343, and CONVI-7585
- `go-servers` current `origin/main` implementation and history for:
  - `insights-server/internal/analyticsimpl/common_user_filter.go`
  - `common_user_filter_test.go`
  - `retrieve_manual_qa_stats.go`
  - all `retrieve_*` call sites
  - `shared/user-filter/{types,options,user_filter}.go`
  - auth `ListUsersForAnalytics` implementation
- `cresta-proto` current `ListUsersForAnalyticsRequest`
- legacy `knowledge/user-filter-consolidation` plan, behavioral standard, Phase 1 review, and later logs

## Findings

### Current Phase 3 state

- Phase 2 merged on 2026-04-25 (`1c6f666a`, PR #26451).
- The merged interface method is `Parser.Parse(ctx, opts)`, not `ParseV2` as the Linear description still says.
- `shared/user-filter/parse.go` is absent on current main.
- No caller uses the new `Parser` interface yet.

### Role-filter clarification

The remembered role change is commit `1c2c84e9` / PR #28200 (CONVI-6930), but it is adjacent to, not inside, `ParseUserFilterForAnalytics`.

`getUsersFromUserAndGroupNames(..., roles)` now applies the optional role list only to the public `ListUsers` request used for group expansion. Explicitly supplied user resource names are inserted afterward and remain preserved. The agent-audience wrapper passes `[AGENT]`; scorecard reviewer queries pass `nil` so reviewer groups can include non-agents.

`ParseUserFilterForAnalytics` itself still accepts only `listAgentOnly`. In auth, `AgentOnly=true` uses an exact-role-array-style predicate: at least one role and no role other than AGENT. That differs from public `ListUsers` `Roles=[AGENT]`, which means the user has any requested role.

### Additional drift since Phase 2 savepoint

- `4fc30176` / PR #28530: scorecard reviewer audience now reuses `ParseUserFilterForAnalytics` with `listAgentOnly=false`, no peer stats, and inactive users included.
- `23d7af64` / PR #29407: removed unused cache and `shouldMoveFiltersToUserFilter` parameters from the analytics parser and callers.
- `20243e10` / PR #30197: reduced allocation/CPU constants by using resource-name builders, reusing converted final users, and reflection-free sorts. It did not change the full-profile enumeration behavior.
- `a2f2905b` / PR #28217: sorts every user's mapped team names alphabetically.
- The analytics parser now has 18 direct `retrieve_*` callers, up from the saved 12, plus the internal scorecard reviewer audience helper. Several saved “old pattern” callers have migrated.
- `ListUsersForAnalyticsRequest.user_ids` is implemented in auth as of CONVI-7585 and can narrow a request while retaining internal auth IDs and profile-scoped memberships.
- `ListUsersForAnalyticsRequest.user_types` exists in the proto, but go-servers does not currently apply it. CONVI-7343 added and threaded it, then was reverted the next day; CONVI-7343 and its parent performance work were canceled.

### Stale durable claims

- The behavioral standard still says `ParseUserFilterForAnalytics` does not expand child teams. Current code has used `shared.ListGroups` for child expansion since CONVI-6260, and the Phase 1 review already verified B-GH-4 passing.
- The Linear Phase 3 ticket still says `ParseV2` and treats the analytics suite as sufficient for a unified parser.
- The Phase 1 “0 gaps” conclusion is correct for retargeting analytics behavior, but not for proving all options in `ParseOptions`: the suite does not establish coaching `Roles`, `GroupRoles`, `UserTypes`, or `State` behavior.

## Compatibility boundary for implementation

The safe Phase 3 baseline includes:

1. Base-population intersection before final output.
2. ACL disabled/root/limited states and UNION of ACL users plus expanded ACL groups.
3. UNION of explicitly selected users and group members.
4. Direct-membership selection semantics plus complete direct/all membership output maps in the new API.
5. Active/inactive and exact agent-only analytics behavior.
6. Child-team expansion, TEAM-versus-DYNAMIC handling, profile scoping, root/default post-processing, deterministic ordering, and `ShouldQueryAllUsers` semantics.
7. `listAgentOnly=false` audiences that intentionally include non-agent users.
8. Coaching and manual-QA role semantics, including the distinction between role-filtered group expansion and preserved explicit users.

The implementation must choose an explicit strategy for two population APIs. `ListUsersForAnalytics` is the canonical source for analytics identity/membership and supports `AgentOnly`, inactivity, group IDs, profile IDs, and user IDs. Public `ListUsers` supports `Roles`, `GroupRoles`, `UserTypes`, and `State`, but has materially different identity and membership behavior. Treating these option sets as aliases would change results.

## Recommended Phase 3 adjustment

- Correct the public method name to `Parse`.
- Require the implementation to honor `Roles`, `GroupRoles`, `UserTypes`, and `State`; `Roles=[]` means no role filter. These are part of the unified contract, not deferred migration-only placeholders.
- Split implementation and tests by population contract:
  - analytics mode: `ListUsersForAnalytics`, `ListAgentOnly`, `ExcludeDeactivatedUsers`;
  - coaching/legacy mode: preserve `Roles`, `GroupRoles`, `UserTypes`, and `State` semantics explicitly.
- Prefer rejecting conflicting mode options over implicit precedence.
- Add role/coaching tests before the core port, including mixed-role users, role-filtered group members, explicit user preservation, non-agent reviewer groups, user types, and active state.
- Keep the canceled all-user short circuit and user-type allowlist out of the compatibility baseline. Evaluate `user_ids` narrowing later as an optimization with invariant-focused tests.

## Validation

- Read-only comparison against current `origin/main`: complete.
- Existing regression command: `bazel test //insights-server/internal/analyticsimpl:common_user_filter_test //shared/user-filter:user-filter_test`.
- Final command result: both targets passed (2/2) on the local checkout; elapsed 385.915 seconds. This validates the checked-out reference suites, while the source-history comparison separately established that the inspected files do not differ from current `origin/main`.

## Credentials used

- Existing Linear connector authorization was used for read-only issue retrieval.
- No local SSH key, Git credential, AWS profile, or other credential file was read or used.
