# CONVI-7715 Walter-dev staging fixture — 2026-10-02

**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Branch/worktree context:** `xw/convi-7715-dynamic-group-filter` at `/Users/xuanyu.wang/repos/go-servers-convi-7715` (read-only investigation; no source edits)

## Scope and sources

- Staging customer `cresta`, profile `walter-dev`, cluster `voice-staging`; current config path `origin/master:configv3/staging/cresta/walter-dev/config.yaml`.
- Read-only `auth-staging/cresta` PostgreSQL, `voice-staging/walter-dev` PostgreSQL, and `clickhouse-conversations.voice-staging.internal.cresta.ai/cresta_walter_dev`.
- The `voice-staging_dev` and `chat-staging_dev` AWS SSO profile entries were inspected before use. DB access used the connection skills in read-only mode. No credentials were printed or recorded.
- Chrome's native accessibility interface exposed the user's already-open Walter-dev Performance Insights page and its open DevTools Network panel. Browser-extension control was unavailable.

## Existing fixture

- Advanced data access control is enabled in the staging `cresta` config.
- Dynamic group `00` (`019ed61f-c996-7d65-8c69-c1c931dd2aea`) is active, is type 2, and has 17 direct members, 15 active. The proposed scoped caller `test-manager@cresta.test` (`751886b9a4f585e3`) is active, has a `walter-dev` profile config, has only `custom_manager_2nd` as a global custom role, has that role on TEAM `Advanced Access Control team` (`44c60ef5-a3e6-47c0-bf01-e5a8e0ca0db1`), and is outside the dynamic group.
- Group member Sergei Radchenko (`7d884aaa024ef269`) is active and a direct agent member of that TEAM. His own broad roles do not establish the caller's permissions.
- Active template `xuanyu na score` (`019d87ad-c39b-7047-a6ca-c7fc87a903dd`) has no audience restriction in the current template row. Two September 23 scorecards for this agent and template are on distinct conversations, each has score 100, `is_dev_user=false`, `is_voice_mail=false`, and use case `walter-dev`.
- The two conversation IDs are `01a0cec5-94d9-7589-8647-bafd5226a241` and `01a0ceaa-6c86-760c-ba1f-72efb1b6777d`.

## Live browser check

On the existing Chrome session, select template `xuanyu na score`, Last 30 days / Daily, and Groups > `00`. The Performance page displays 100% and two filtered conversations on September 23. DevTools shows a successful `qaScoreStats:retrieve` request to `/v1/customers/cresta/profiles/walter-dev/qaScoreStats:retrieve`. Its payload includes `groups:[{name:"customers/cresta/groups/019ed61f-c996-7d65-8c69-c1c931dd2aea"}]`, the selected template resource, `2026-09-03T04:00:00.000Z` through `2026-10-03T03:59:59.999Z`, daily frequency, and AGENT/TIME_RANGE grouping. The response contains the scored agent.

This establishes an existing scored dynamic-group fixture and a healthy broad-access browser baseline. The current Chrome account has broad roles. It does not validate the deployed group-filter behavior for the scoped manager. To finish that check, authenticate as `test-manager@cresta.test` through an authorized session and compare the same template/date window with dynamic group `00` versus TEAM `Advanced Access Control team`; inspect each response and correlate its parser/ClickHouse logs.

## Role-change follow-up

The user changed Chrome account `d634c90a8748bcb2` to manager and user admin only. A read-only auth DB check found `custom_role_ids={custom_manager,custom_user_admin}`; both assignments are on the root TEAM. This user has no manager assignment on `Advanced Access Control team` and is not a member of dynamic group `00`.

After reloading the existing Chrome page, the same `xuanyu na score` template and Last 30 days / Daily range returned 100%, two September 23 filtered conversations, and Sergei Radchenko for **both** `Advanced Access Control team` and dynamic group `00`. DevTools showed successful `qaScoreStats:retrieve` calls with a name-only resource for each selection; the group call used `customers/cresta/groups/019ed61f-c996-7d65-8c69-c1c931dd2aea`. The current browser session still sent older, broader authorization claims after the role change. These page results remain a broad-access fixture check, not a scoped-manager validation. A fresh authenticated session for the secondary manager on the fixture TEAM is needed for that validation.

The user noted that UserService role changes can take a few minutes to propagate. A later Chrome reload still returned the two group-filtered conversations and still sent the older role claims. The Mac then locked, preventing another browser check. Resume with a fresh request after unlock and verify its authorization context before interpreting the result as scoped access.

After the Mac was unlocked, a further reload more than 20 minutes later again showed 100% and two filtered September 23 conversations for group `00`. The page initially remained in a loading state before completing. This remains a fixture check; the account's root TEAM assignments still make it unsuitable for the scoped-manager validation. The previously active Chrome tab was restored after inspection.

## Scoped-role follow-up

After the user removed the root TEAM membership, a read-only `chat-staging/auth-staging/cresta` auth DB check found that `d634c90a8748bcb2` has only `user_roles={2}` and `custom_role_ids={custom_manager}`. Its sole direct TEAM membership is `xuanyu only` (`019ea91a-3244-72c7-982d-f1cdefd9f941`), carrying `role_id={2}` and `role_ids={custom_manager}`. This TEAM has one direct member (the caller), no agent members, and no child groups. In contrast, the existing `Advanced Access Control team` fixture (`44c60ef5-a3e6-47c0-bf01-e5a8e0ca0db1`) has four direct members, including two with the agent role; scored group-`00` member Sergei is on that TEAM.

The Chrome Performance page currently shows the unfiltered `All` template, Last 30 days / Weekly, 0% and zero conversation volume. No fresh request with both the scored template and group `00` was captured after the membership change. The current `xuanyu only` scope has no scored agent in the known fixture, so an empty group result would not distinguish correct ACL restriction from the CONVI-7715 defect. To validate the fix with this browser account, give it a non-root manager assignment on `Advanced Access Control team` (and keep root manager-plus assignments absent), then compare that TEAM against dynamic group `00` with template `xuanyu na score` and the same date range. Alternatively, authenticate as the existing `test-manager@cresta.test` fixture account. No membership or source data was changed in this investigation.

## Scoped-manager browser validation

The user assigned browser account `d634c90a8748bcb2` as `MANAGER_2ND` on `Advanced Access Control team` and signed out and back in. A read-only auth DB check confirmed `user_roles={14}`, `custom_role_ids={custom_manager_2nd}`, that direct TEAM assignment, and no root TEAM membership.

With template `xuanyu na score`, Last 30 days / Weekly, All minutes, and Exclude voicemail Yes, the Performance page returned **100% and two conversations** when filtered to `Advanced Access Control team`. Selecting dynamic group `00` also returned **100% and two conversations**. The agent leaderboard identified Sergei Radchenko on Advanced Access Control team with two scorecards at 100%. This is a positive Walter-dev UI validation for a non-root team-scoped manager using the scored dynamic-group fixture.

The Chrome page went blank after a subsequent DevTools reload, so the exact fresh group-request trace header was not preserved. The browser UI result and auth DB role check are the independent evidence for this validation. The historical Oportun request and build remain uncaptured; this Walter-dev check does not establish its production outcome.
