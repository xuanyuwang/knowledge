# CONVI-7785 shared behavior ownership review

- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree/branch: `/Users/xuanyu.wang/repos/director-convi-7785`, `xw/convi-7785-exclude-simulator-rules`
- PR/comment: [director#23461](https://github.com/cresta/director/pull/23461#discussion_r4219504259)
- Reviewed PR head: `3a852b97abe2bec84550e51dca52dd38336bf0a1`

Flatplate asked whether the `trainingSimulatorOnlyBehaviors` and `nonSimulatorBehaviors` sets are redundant. In the Director loop, the `return` at line 56 exits only the callback for one policy/behavior pair. It does not itself prevent the same behavior name appearing under another policy.

Current go-servers `main` provides the actual normal-write invariant. `CreatePolicy` and `UpdatePolicy` call `validatePolicy`, which calls `validateBehaviorUsage` (`apiserver/internal/policy/validator.go:242,391-434`). The lookup rejects another policy in the same customer/profile whose `behavior_ids` overlap. It has no status predicate, so archived ownership also blocks a second policy. Tests cover both create and update conflicts (`action_create_policy_moments_test.go:1055-1098`, `action_update_policy_test.go:484-536`). Other inspected service paths, including duplicate, requirement sync, and policy grouping, call CreatePolicy or UpdatePolicy.

Conclusion: the two sets and shared-ownership test protect a state rejected by normal policy writes. Recommend removing `nonSimulatorBehaviors`, changing the filter to a single Simulator-owned set membership check, and replacing the shared-ownership test with a supported-state test or deleting it. Scope the claim to normal API writes: the table stores `behavior_ids` arrays without a cross-policy uniqueness constraint; historical direct writes or races were not ruled out. If support for such rows is an explicit requirement, retain the two-set logic and document that requirement.

This review made no Director or GitHub changes. `gh` and `git` authenticated themselves; no selected AWS, Okta, Azure, or SSH credential was used.
