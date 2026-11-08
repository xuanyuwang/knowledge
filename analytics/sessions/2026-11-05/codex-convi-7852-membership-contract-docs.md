# CONVI-7852 membership contract review

- Primary source repo for this session: /Users/xuanyu.wang/repos/cresta-proto
- Worktree: /Users/xuanyu.wang/repos/cresta-proto-convi-7852-membership-roles
- Branch: xw/convi-7852-membership-docs
- Base: origin/main a95de1dd2d
- Review: https://github.com/cresta/go-servers/pull/35094#discussion_r4485910411
- Analytics head verified: 79a43980e782045c7ef98bb694d32f54bddf1bcc

## Validation

The review is valid as a documentation mismatch. cresta/v1/user/user.proto says User.group_memberships are DIRECT only. Existing auth ListUsers queries user_group_memberships_all and model conversion preserves m.IsIndirect; analytics inline conversion likewise sets IsIndirect from IsDirectMember. A doc-only correction is sufficient and preserves approved all-membership response semantics.

The proposed docs say the list includes indirect memberships and directs callers to each entry's is_indirect flag. No fields, wire values, producer behavior, or analytics code change. This User is absent from the filtered public/proto tree; public/proto output stays unchanged.

Director current main was verified through GitHub contents API: user/transformers.ts blob 527cc5c2fc3dd564ed686d8eb81432e18ab38dde, getApiUserTeamDisplayName checks !group.isIndirect and TEAM; transformApiGroupMembershipToModel preserves isIndirect. The local CONVI-7852 prototype agentTeams.ts checks !membership.isIndirect, TEAM, AGENT and row groups. Its eight existing tests pass, including ignoring indirect memberships. The prototype is uncommitted work; inline analytics consumption is still a follow-up, not a deployed guarantee or a claim about every Director consumer. No Director edits.

## Checks

Full mage lint and apiLint passed. Affected //cresta/v1/user:all Bazel build passed (7.4s). clang-format dry-run passed. Scoped buf breaking against origin/main passed. Eight local Director selector tests passed (9.8s). Logs /tmp/convi-7852-membership-docs-{lint,apilint,build,breaking}.log and /tmp/convi-7852-director-indirect-check.log.

The format wrapper uses mapfile unavailable in default macOS Bash, so formatting was verified directly with the existing clang-format binary; no tool changes. The clean proto worktree was reused from closed custom-role PR #10601, preserving that historical branch at 720c300b2a. No change to custom-role schema.

## Credentials

GitHub CLI/git and Docker authenticated themselves. No AWS/Okta/Azure/SSH credential or profile explicitly selected or read.

## Publication

Created https://github.com/cresta/cresta-proto/pull/10620 at a05b252c6f on xw/convi-7852-membership-docs, one comment-only file change. Attached the PR to this chat. Gazelle passed without BUILD changes; source worktree clean. New PR CI is pending, no merge performed.

Posted validation/fix/frontend distinction in the original review thread: https://github.com/cresta/go-servers/pull/35094#discussion_r4486112954 . Resolved thread PRRT_kwDOGivnF860Q-4X; no go-servers changes were needed.
