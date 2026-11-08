# CONVI-7852 — internal membership custom-role protobuf PR

Source repo: `/Users/xuanyu.wang/repos/cresta-proto`.
Branch/worktree: `xw/convi-7852-custom-membership-roles`, `/Users/xuanyu.wang/repos/cresta-proto-convi-7852-membership-roles`.
Reused clean worktree from merged #10550 on a new branch based on fetched origin/main `33f0f79da7`. User requested PR following the custom-role gap investigation. Interpret customer_roles in the request as custom_roles, consistent with the established public membership field and prior proposal; stated this choice to user before editing.

Add repeated string custom_roles=4 to cresta/nonpublic/user/user.proto UserGroupMembership. Values are full role resource names assigned directly to the user in this group; indirect memberships empty. No additional imports or generated source. Producer population, analytics pass-through, and FE integration remain separate work.

Scoped baseline buf build passed before editing. Follow repository PR template, lint/API lint, scoped compile/breaking checks, formatting, Gazelle and affected Bazel package build. No separate behavior tests for this additive schema field; compilation and generation validate type/tag compatibility.

## Outcome

Created https://github.com/cresta/cresta-proto/pull/10601, title `CONVI-7852: add custom roles to analytics user memberships`, label semver:minor, base main. Commit `720c300b2afbdb30ff8c9c2b2b3fca364875d4f5`. Live PR read-back confirms one changed file with four additions; source worktree clean. PR attached to this chat. No generated code committed.

Validation: scoped Buf baseline/build/lint and breaking check against origin/main passed; full mage -v lint and mage -v apiLint passed; full checkAllProtoDependencies, import and HTTP-path lint passed. Gazelle produced no BUILD diff. clang-format 22.1.0 dry-run and git diff --check passed. bazel build //cresta/nonpublic/user:all passed in 31.5s, including Go generation/compilation. Existing Buf DEFAULT deprecation and C++ platform warnings did not fail checks. Initial uvx formatter lookup used configured private index and failed authentication; reran with explicit public PyPI index and no config, which succeeded. No credentials inspected, selected or persisted for the lookup.

No producer, consumer, frontend, customer DB or deployment changes. Git/gh, Docker and package tools used existing self-authentication; no AWS/Okta/Azure profile or SSH key explicitly selected. CI not monitored beyond PR creation/read-back. Follow-up remains proto merge/release, then auth projection and analytics pass-through, then FE custom-role definition resolution.
