# CONVI-7852: AdventHealth Leaderboard team selection

Investigated October 29, 2026. [Linear](https://linear.app/cresta/issue/CONVI-7852). Read-only source and production auth-data investigation; no product changes.

## Conclusion

The current source explains Sarah Young's and Allison Nesbitt's reported wrong Team values. The backend returns every direct TEAM membership, sorted by group resource name (the group ID determines order within a customer). Director reduces that list to its first element. Membership roles are absent from the analytics user contract, so neither stage can prefer the Agent/home team using this data. Consolidating Manager 2nd to a single parent membership cannot fix this when that parent's ID sorts first.

This mechanism is confirmed in source and reproduced with live membership values through the actual Director transformers. The production page/API response and deployed image versions were not captured; the labels below are source-derived predictions, not direct UI observations.

## Evidence and source trace

Source checkouts: go-servers `35c364ff698e86e4d8eef77f4504eb85dc76c5cc`; Director `34ad89e76d0050eaaf9ec6e8cf1b5148933ca84e`. Fetched origin/main in both repositories. Remote heads were go-servers `4950a8d8e90ca77e2342a3d68c6d8e1b1ddeccd9` and Director `8f706463bbd47cc4732129cfef5e8235cfccfe05`. Inspected remote diffs: the relevant team-selection and transformer logic is unchanged; go-servers has unrelated group-type resolution extraction. All local line references below refer to the inspected checkouts.

1. `director/packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx:74` requests conversation stats grouped by Agent and Team.
2. `go-servers/auth/internal/service/nonpublic/user/action_list_users_for_analytics.go:210` reads membership rows from `auth.user_group_memberships_all`. Its smart-select projection and returned `UserGroupMembership` include group ID/name/type and directness, but omit role arrays.
3. `cresta-proto/cresta/nonpublic/user/user.proto:23` defines analytics `UserGroupMembership` as group plus `is_direct_member`; there is no per-membership role or home-team designation. Public `cresta/v1/user/group.proto:124` does expose roles and custom roles, so the source data exists outside this reduced contract.
4. `go-servers/insights-server/internal/analyticsimpl/common_user_filter.go:558` collects all direct TEAM memberships for Agent grouping without role priority; lines 599–604 sort their resource names. The sorting was introduced by `a2f2905b5c4`, PR #28217, May 27, 2026, to make labels stable across response ordering. That change adds determinism, not a home-team rule.
5. `retrieve_conversation_stats.go:107` first queries per-user stats; `:142` attaches all mapped groups to the same per-user result. It does not select a primary team.
6. `director/packages/director-api/src/services/cresta-api/insights/transformers.ts:794` sets singular `group` from `attribute.groups[0]` while also retaining the full `groups` array. There is no root or role priority here.
7. `AgentLeaderboard.tsx:162` copies that singular group's display name into the Team column. `:384` exports the same rows to CSV.

The historical [December 2025 Slack thread](https://crestalabs.slack.com/archives/C05FGPN6VQQ/p1765379076182839) describes response-time mapping through ListUsersForAnalytics and says users can belong to multiple teams. Its indexed text was available through Glean; structured retrieval reported TRANSFORM_ERROR. The current code is more precise than the shorthand “non-root team”: it chooses the first sorted direct TEAM, and the Agent-grouping mapping does not explicitly exclude root/default IDs. The six examples have no direct root/default membership, so this distinction does not explain their cases.

## Production membership findings

Resolved from latest `config` origin/master: `configv3/prod/advent-health/config.yaml`; AWS account alias `us-east-1-prod`, auth cluster `auth-us-east-1-prod`, database/customer ID `advent-health`, profile `us-east-1`. The ticket's “adventhealth” spelling is a use-case label rather than the customer ID. Used the customer-auth-DB skill and `cresta-cli connstring -i --read-only`; connection data remained in process memory. Set a 15-second statement timeout and explicitly enabled session default transaction read-only before the membership and count queries; connection was closed afterward.

| User | Current source predicts | Current Agent memberships | Interpretation |
| --- | --- | --- | --- |
| Sarah Young | Christina Gusby | Brittni Howard only | Confirmed Agent-versus-Manager-2nd mismatch mechanism. Christina's ID `01a0fce6-f365-7a82-860a-5e3cdcc90bd2` sorts before Brittni's `01a0fce6-f365-7ac6-9446-600b750bac65`. |
| Allison Nesbitt | Silvia Martin | Miranda Adcock only | Same mismatch. Silvia's ID `01a0fce6-f365-7aa3-955f-2cfaf3d52767` sorts before Miranda's `01a0fce6-f365-7ab8-b55a-f08959334572`. |
| Ana Lopez Cordova | Danielle Shuler | Danielle Shuler only | The current mapping predicts her Agent team. Her historical wrong label is not reproduced by today's membership snapshot. |
| Gary Nunez | Erika Gonzalez | Erika Gonzalez, Jeilene Leon, Kayla Light | All three direct teams have Agent plus Manager 2nd. Role preference alone cannot identify one home team. |
| Stephanie Montalvo | Erika Gonzalez | Erika Gonzalez, Jeilene Leon, Kayla Light | Same multi-Agent ambiguity as Gary. |
| Anita Soltis | Region: GLR CxC | None | Eight direct teams today: Manager on Anita Soltis, Manager 2nd elsewhere. Expected Angie Luckey-Succes is the parent of her managed team, not her Manager-role team. |

Hierarchy queries confirm Christina Gusby is the parent of Brittni Howard, Miranda Adcock, and Silvia Martin; Angie Luckey-Succes is the parent of Anita Soltis; Region: GLR CxC is the parent of Angie Luckey-Succes. All six users have empty legacy `team_id` and `manager_id`; the public User proto also deprecates the old single-team fields. Those fields cannot resolve these six examples.

Active-profile membership population, excluding DEV_USER=3 and EXTERNAL_PARTY_USER=6 and matching analytics profile/managed-group rules:

- 960 users have at least one direct Agent team.
- 43 have exactly one Agent team and additional direct teams.
- Of those 43, 33 have a different team first under the current ID sort.
- 25 have multiple direct Agent teams and need a separate choice rule.

These are membership populations, not counts of users with qualifying conversations, affected visible rows, or customer-observed failures. They do not prove historical region coverage. [Reproducible SQL](../sessions/2026-10-29/convi-7852-membership-checks.sql). An initial exploratory count used the wrong UserType exclusions; the figures above come from the corrected query using enum values verified in source.

## Effects beyond the Team label

- **Agent conversation metrics:** per-user stats are queried before group attachment. The first group selection does not change conversation count/AHT in this response path. Rows are keyed by user, so multiple group labels do not create multiple Agent rows here. This does not prove every metric is accurate under all filters.
- **CSV:** exports the same row Team string; the mismatch propagates when Team is included.
- **Team filters:** membership/ACL resolution happens before response enrichment, and the Agent mapping can attach all direct teams even when a particular team was selected. A Team label can therefore differ from the selected team without proving a filter leak. No live filter/ACL reproduction was performed.
- **Team Leaderboard:** `retrieve_conversation_stats.go:179` rolls each user's stats into every mapped group. Changing shared memberships to only a preferred team would also change group totals and semantics. A display-selection fix must not silently rewrite those memberships.
- **Manager Leaderboard:** a different path in `ManagerLeaderboardPage.tsx:197` builds manager-to-team mapping from explicit MANAGER memberships. It does not provide the expected parent leader for Anita by itself. The ticket's manager-only expectation needs an explicit business rule and tab clarification.
- **Agents-only filter:** the UI forwards `filtersState.listAgentOnly`; ListUsersForAnalytics implements this as users whose overall proto roles contain only AGENT. All six have additional roles, so the exact filter setting must be recorded during live reproduction. This is separate from choosing an Agent-role home team.

## Verification

[Replay script](../sessions/2026-10-29/convi-7852-team-label-replay.cjs) extracts and transpiles the actual Director `transformAttributes` and `transformConversationStatsResponse` functions, using identity stubs for user/group model conversion and modeling the backend ID sort from source. Both Sarah and Allison select the non-Agent team; reversing group order changes the selected label. One response row and a synthetic count of 17 remain unchanged. The count is illustrative, not customer data. Executed successfully with Node; no product test files or source were edited.

## Recommended next work

1. Capture one authenticated production Agent Leaderboard response and screen for Sarah with the exact filters/date range, plus the running frontend/backend versions. A background browser visit redirected to the AdventHealth sign-in page; no authenticated session was available, no credentials were entered, and the temporary tab was closed. This leaves a concrete gap between source/data reproduction and observed deployment behavior.
2. Define Agent/home-team display separately from membership and access. For exactly one Agent team, a role-aware preference can resolve Sarah and Allison, but analytics must receive per-membership role information or an explicitly resolved reporting-team value. A frontend-only reorder cannot infer roles from the current reduced response.
3. Decide how to select among multiple Agent teams (Gary/Stephanie), how manager-only users should resolve to their leader (Anita), and what happens with zero valid candidates or multiple parents. Avoid inventing a universal hierarchy-depth heuristic.
4. Once agreed, add regression cases for single Agent plus Manager 2nd parent/siblings, multiple Agent teams, manager-only parent leader, root/default, no candidate, selected-team filters, and CSV. Keep Agent metric counts, Team rollups, and access filtering covered separately.

No product fix, org configuration mutation, Linear update, or external message was made. AWS authentication was handled by cresta-cli; no AWS profile, Okta credential, Azure credential, or SSH key was explicitly selected or inspected.


## Implementation follow-up — October 30

The user resolved the display rule: prefer direct Agent-role teams in the Agent tab, choose the lowest resource ID when several qualify, and otherwise preserve the existing first team. This supersedes the reporting-team decisions left open above. The Director-only fix uses targeted public ListUsers membership metadata intersected with existing analytics groups; it does not require changing the reduced analytics protobuf. Worktree: `/Users/xuanyu.wang/repos/director-convi-7852`, branch `xw/convi-7852-agent-team`. Both Agent tables and their CSV paths use the derived label. See the [implementation record](../sessions/2026-10-30/codex-convi-7852-agent-team-fix.md) for tests and remaining verification. The preceding investigation remains a record of the October 29 evidence, not a deployment claim.
