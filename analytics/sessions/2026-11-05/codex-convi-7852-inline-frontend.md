# CONVI-7852 inline Agent leaderboard integration

- Source repo: /Users/xuanyu.wang/repos/director
- Worktree: /Users/xuanyu.wang/repos/director-convi-7852
- Branch: xw/convi-7852-agent-team
- Refreshed base: 764bb8bf15 (origin/main)
- Commit: 9e42ac1f6e7cd331129f2ddf3075ea3a1abec1bb
- PR: https://github.com/cresta/director/pull/24670 (draft; no merge)

## Contract and implementation

User authorized implementing the approved direct TEAM legacy AGENT preference only on Leaderboard → Agent. Choose the lowest resource ID among existing analytics row groups; otherwise retain the first-team fallback. No hierarchy inference or custom-role permission inference. Cached metadata's normal delay is acceptable.

Verified go-servers #35092 merged at 2026-11-05T14:08:49Z and #35094 at 2026-11-05T22:21:55Z, head 79a43980e782045c7ef98bb694d32f54bddf1bcc. Proto #10550 is merged, #10620 docs-only remained open at initial check, and custom-role #10601 is closed. Merges are not deployment evidence.

Safely stashed tracked/untracked prototype, fetched main, fast-forwarded from 59ec62ac94 to 764bb8bf15, and restored the stash without conflict. No other source edits existed. Immutable Yarn install synchronized main's pinned web-client 2.33.35; no package or lockfile change. Generated User.groupMemberships and GroupMembership.roles/groupType/isIndirect already exist. Director-api User/GroupMembership models preserve these fields; transformConversationStatsResponse → transformAttributes → transformUsers → transformApiUserToModel → transformApiGroupMembershipToModel already preserves inline metadata, so no production API converter changes were needed.

preferAgentTeams now reads each row's inline user memberships instead of a separate user map. Direct TEAM AGENT membership names constrain candidates within existing groups, deterministic lexical resource order breaks ties, and only groupedByAttributes.group changes in a derived response. Original response/groups/metrics remain untouched. AgentLeaderboardPage memoizes that response for the main and by-metric Agent tables and their existing exports. Other pages/shared query cache retain the old data. Removed getMultiTeamUserIds, useAgentLeaderboardTeams and its pagination/fetch tests. Existing QA-only rows retain their policy.

## Validation

- Red: adjusted selector tests to consume inline memberships; dual-role and tie selection failed under the fetch-based prototype (2 failed / 9 passed). Green: 12 selector tests pass after inline implementation, including a real converter-to-selector test.
- 28 leaderboard tests across six files pass, including existing Agent, Team, Manager, ALO and QA-only data behavior.
- 24 Insights API tests across three files pass, including two new metadata preservation/old-producer regressions. Fixtures use fixed group timestamps because the existing converter defaults absent group createTime to now; UTC/timezone Dayjs plugins are initialized in API tests.
- App `yarn tsc` and testkit `yarn tsc` pass.
- Changed-file ESLint and Biome pass; root `yarn check:deps`, API/app production Knip, staged diff checks pass.
- Commit hooks: i18n extraction, lint, i18next lint, formatting and ticket-prefix check pass.
- Independent read-only review found no actionable issues and verified main/by-metric wiring and shared cache isolation.
- Extended existing Agent smoke fixture with a lower-ID Manager2nd team plus direct Agent team, asserted main/by-metric Support Team and unchanged 7.5 conversations/day. Playwright execution cannot reach the test: local setup throws because `.auth/auth-token.json` is missing. Standard `.env` examples were copied via Makefile; no token copied or created. No browser/customer validation or videos.
- PR CI queued at creation; no success claimed for remote CI or deployment.

Local logs: /tmp/convi-7852-selector-red.log, /tmp/convi-7852-selector-green.log, /tmp/convi-7852-leaderboard-tests.log, /tmp/convi-7852-api-tests.log, /tmp/convi-7852-tsc.log, /tmp/convi-7852-testkit-tsc.log, /tmp/convi-7852-{api,app,testkit}-lint.log, /tmp/convi-7852-{api,app}-knip.log, /tmp/convi-7852-deps.log, /tmp/convi-7852-smoke.log, /tmp/convi-7852-commit.log.

## Follow-up and credentials

Review PR/CI, provide a valid test login to run the extended smoke, and verify backend rollout/customer labels and metric parity before treating the fix as deployed. No merge requested or performed. Git/gh authenticated themselves; Yarn used existing registry authentication. No AWS/Okta/Azure/SSH credential/profile explicitly selected or read. No customer database access or production data mutation in this session.

## Follow-up: leave testkit unchanged

User requested avoiding testkit changes when possible. Restored the Agent smoke spec and fixture exactly to base 764bb8bf15, keeping all selector and converter unit coverage. PR description updated to reflect unchanged testkit and pending browser validation. This follow-up only removes testkit edits; production code and the 52 passing focused unit tests are unchanged.

Follow-up commit `0ec809a7ae` pushed to #24670; all commit hooks pass, source worktree clean, and `git diff 764bb8bf15...HEAD -- packages/director-testkit` is empty.
