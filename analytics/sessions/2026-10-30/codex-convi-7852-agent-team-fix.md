# CONVI-7852 Agent-tab team preference implementation

Date: 2026-10-30 (America/Toronto).
Primary source repo for this session: `/Users/xuanyu.wang/repos/director`.
Worktree: `/Users/xuanyu.wang/repos/director-convi-7852`, branch `xw/convi-7852-agent-team`, based on freshly fetched origin/main `59ec62ac94`.

## Authorized behavior

User approved preferring direct teams where the user has the Agent role, with team resource ID sorting for deterministic ties. When there are no eligible Agent teams, preserve the existing first-team behavior. No separate manager-only hierarchy rule is requested.

## Approach

Keep this in Director rather than changing the reduced analytics protobuf contract or adding an all-group read. Analytics already retains each row's full groups array. Query existing public ListUsers only for distinct users with multiple candidate teams, including memberships and fetching all returned pages. Use membership roles to select the lowest-ID direct Agent TEAM from the response's existing candidate groups. Change only the singular display group on a derived response, preserving groups, metrics, filters, ACL, and the shared query cache. Feed the derived response to both Agent tables and CSV through existing page props. Keep the old label while metadata is unavailable or the read fails.

Public ListUsers is used for metadata enrichment only, not as an analytics population replacement. Source verification confirmed the public response includes membership roles and default unspecified state includes active and inactive profile users. Candidate intersection is required because public memberships may span profiles.

## Worktree and validation

Native worktree creation is tied to this chat's workspace-metadata checkout and cannot select the sibling Director repo, so used git worktree add for the explicit target repository. No existing source changes were present. Initially reused installed dependencies through symlinks. The installed web-client was 2.32.91 while the branch requires 2.32.116, causing unrelated Training Simulator TypeScript errors. Removed both symlinks and installed an independent dependency tree with the immutable lockfile; final checks use the required 2.32.116 client. Main-checkout dependencies were not modified.

- Baseline: existing Agent row utility suite passed, 4 tests.
- Added regression tests before behavior; with identity implementation, three failures showed wrong Manager-team selection, absent deterministic Agent-team preference, and missing targeted IDs; five fallback/edge cases passed.
- Implemented pure team selection and targeted, paginated membership lookup; wired only AgentLeaderboardPage conversation stats to both existing Agent table paths.


## Final source review

Five Director files changed: AgentLeaderboardPage plus agentTeams/useAgentLeaderboardTeams and their tests. The shared analytics transformer and backend are untouched. AgentLeaderboard assigns the displayed team from conversation stats; the by-metric hook builds its user row template from the same response. Existing CSV callbacks export those derived rows. QA-only rows retain existing behavior. Agent-preferred candidates must be direct TEAM memberships, carry AGENT, and appear in the analytics row. Resource-name order is resource-ID order within the same customer. Original groups, input rows and metrics are preserved.

Membership metadata is asynchronous: initial display and a failed or ACL-limited lookup retain the old team label. Rows whose metadata arrives on later pages update when that page is fetched. No production browser/API confirmation, deployed check, performance benchmark, commit, push or PR was performed.

## Verification

- Fresh independent dependencies: immutable Yarn install completed; existing peer-dependency warnings only.
- Entire Leaderboard unit folder: 7 test files, 27 tests passed, including 11 new selection/lookup tests.
- ESLint on all five changed files: passed, zero warnings.
- Biome on all five changed files: passed.
- Production knip files/dependencies: passed.
- git diff --check: passed.
- Director-app TypeScript build (`tsc6 -b`, equivalent to package yarn tsc): passed, exit 0 after preserving query-state narrowing.

Dependency setup used GitHub CLI self-authentication for an initial registry fetch (403) and then the existing GitHub package-registry token from local shell settings, passed only to the child installation process without printing or writing the secret. No AWS, Okta, Azure or SSH credential was explicitly selected, inspected or used. No customer DB queries or production changes in this implementation session.


Checks were run from `/Users/xuanyu.wang/repos/director-convi-7852/packages/director-app`, using Node 24.22.1 from the bundled Codex runtime. Reproducible package commands with normal Yarn setup: `yarn tsc`, `NODE_OPTIONS=--no-experimental-webstorage yarn test src/features/insights/leaderboard`, targeted ESLint on the five changed files, and `knip --production --files --dependencies`. Biome and git diff --check ran from the worktree root. The webstorage option is needed for Node's experimental localStorage global in the existing test environment; SDK sourcemap warnings are pre-existing.

The first TypeScript pass with correct dependencies caught the spread replacement widening React Query's discriminated union. Fixed the page wiring to retain the original pending/loading-error result and replace data only on the remaining query states. No assertion or suppression added. Final TypeScript rerun follows.

Final handoff: implementation complete locally; 27 tests, app TypeScript build, targeted ESLint, Biome, production knip and diff whitespace checks pass. No commit/push/PR; authenticated fixed-build customer verification remains pending.
