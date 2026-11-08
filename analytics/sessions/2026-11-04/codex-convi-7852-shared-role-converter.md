# CONVI-7852 — reuse the shared role converter

Source repo: `/Users/xuanyu.wang/repos/go-servers`.
Primary branch/worktree: `xw/convi-7852-auth-membership-roles` in `/Users/xuanyu.wang/repos/go-servers-convi-7852-auth-roles`.
Related consumer branch/worktree: `xw/convi-7852-analytics-membership-roles` in `/Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles`.

User approved exporting and reusing the existing `auth/shared/model/user.go:stringRoleIDToRoles`, removing `analyticsMembershipRoles`. Existing implementations are identical: parse decimal strings with signed int32 bounds, skip invalid values, preserve unknown numeric enum values and input order. No behavior changes intended. Both worktrees clean; remote PR heads match local auth `cf06a739bae770bb0cfcd79c0ffa594b0bbd5875` and consumer `25086d9489cd4316c52d12d8640ff5908b2bc460`, both PRs open.

Run the existing full auth user-service and shared-model targets before and after the refactor; preserve the existing tests. Export the existing helper, update its public conversion caller and analytics caller, remove duplicate helper and unused import, run Gazelle, commit auth change, rebase consumer onto updated producer, and push both with explicit leases.

## Outcome

Exported the existing implementation as `model.StringRoleIDsToRoles`, switched its existing public user conversion caller and `ListUsersForAnalytics` to it, removed the duplicate analytics function and unused strconv import. Exactly two Go files changed (5 insertions, 16 deletions). Existing public user-service private helpers are outside this cleanup and untouched. Converter body is unchanged; malformed/out-of-range skipping, unknown numeric enum handling, ordering, and empty-slice behavior stay identical.

Same test targets before/after: `//auth/internal/service/nonpublic/user:user_test` and `//auth/shared/model:model_test`, using local Postgres. Baseline: auth cached pass (16.9s), model pass (4.3s). After refactor: auth fresh pass (15.9s), model cached pass (4.3s). Gazelle passed without BUILD changes, gofmt and diff checks passed. No new tests or behavior changes.

Auth commit `76223191bb3a6c3093ba68f3e213fc3a2321a6f3` pushed to [#35092](https://github.com/cresta/go-servers/pull/35092). Consumer rebased to `c30fca8d40a4c60d967dbdb40611b9d55c74cbc0` in [#35094](https://github.com/cresta/go-servers/pull/35094); range-diff confirms identical consumer patch. Both refs pushed atomically using explicit leases; auth was a fast-forward and analytics rewritten only for the changed parent. Auth PR description updated to document shared converter and both test targets. CI after push not monitored. Earlier full conversation-suite local ClickHouse memory limitation remains unchanged and documented; this cleanup does not change analytics code. Director untouched.

Git/gh used existing authentication; no AWS, Okta, Azure, or SSH credential was explicitly selected. No production/customer operations or messages.
