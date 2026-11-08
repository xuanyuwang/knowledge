# CONVI-7852 protobuf membership roles PR

Created [cresta-proto #10550](https://github.com/cresta/cresta-proto/pull/10550), titled `CONVI-7852: add per-team roles to analytics user memberships`, with `semver:minor`. This is preparatory work for CONVI-7852, not a complete fix for the ticket.

## Change

Only `cresta/nonpublic/user/user.proto` changes: import the existing auth definition and add `repeated cresta.v1.auth.AuthProto.Role roles = 3` to `UserGroupMembership`. Comments define directly assigned Cresta roles and empty indirect memberships; inherited roles are not resolved. The user deferred custom roles. No handler, Director, generated-code or database changes are included.

Worktree: `/Users/xuanyu.wang/repos/cresta-proto-convi-7852-membership-roles`. Branch: `convi-7852-adventhealth-leaderboard-shows-dual-role-users-agent-manager`. Commit: `bd08570dcb20a781aca27149e5d3d58f6fba3689`. The source worktree is clean after pushing.

## Validation

Scoped Buf build, lint and breaking checks passed. Full `mage -v lint`, `mage -v apiLint`, dependency checks, import and HTTP-path lint, clang-format 22.1.0, `git diff --check`, Gazelle and `bazel build //cresta/nonpublic/user:all` passed. Gazelle produced no BUILD changes. A separate reviewer reported no findings.

Current main at `7d51703ec3` already fails protobuf compilation because `DIRECTOR_INSIGHTS_CUSTOM_EXPORT_PAGE` and `API_CONVERSATION_OPERA_SUMMARY` both use enum value 91 in `cresta/v1/auth/permission_feature.proto`. This was reproduced before editing. The branch starts at preceding revision `8507e6296cd2a1f06b4c92acaff3652a2c012d43`; the checks above passed there with this change. The PR documents that main's collision can block merged-main CI. CI completion is not yet verified.

## Remaining work

Implement auth query projection and response population separately, then carry memberships through Insights response and cache paths before replacing the extra Director lookup. Preserve all team contributions during aggregation and keep the approved Agent display preference separate. The existing Director worktree remains unchanged and uncommitted.

Git and gh authenticated themselves; no explicit AWS profile, Okta/Azure credential or SSH key was selected or inspected. No Linear comment or other external message was sent.
