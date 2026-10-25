# CONVI-7874 compared with the earlier Oportun fix

Date: 2026-10-23 (America/Toronto).
Primary source repo: /Users/xuanyu.wang/repos/go-servers.
Checkout context: main, local 9ed3337b252fe698dcba4d311aaeb6804e3db191 aca4c1377468f4c439c8e2428ea5640bc318f6bc; source read from fetched PR commit, without checkout or product edits.
Reviewed PR #34439 base: af832d5d38930857de84227f6a58aa5c29d819e3; head: 9ed3337b252fe698dcba4d311aaeb6804e3db191.
Earlier PR #33026: head 899da6f56ef787546c8581822b8c9ee5a171b50b, merged 2026-10-01 as 445e953c74f671d04222600a8cdf750b559cb3bc.

## Conclusion

The user supplied CE-8102 with the Oportun Performance Insights title. Existing project records identify that same incident/title as CONVI-7715 and its implemented fix as PR #33026. The CE/CONVI ticket alias was not independently checked in Linear this session. Current GitHub metadata and both diffs were read directly.

Both fixes use the same principle: resolve authoritative TEAM versus DYNAMIC_GROUP types; dynamic groups select member users, whose access is checked against the caller's allowed users. Neither changes the frontend tabs. The new PR extends that principle to a separate backend filter path rather than replacing the old PI behavior.

## Differences

| Aspect | Earlier #33026 | New #34439 |
|---|---|---|
| Customer report | Oportun PI N/A for a scoped manager | Sunbit Closed Conversations zero for non-admin, admin sees about 1.7K (PR description) |
| Entry point | RetrieveQAScoreStats → ParseUserFilterForAnalytics | RetrieveClosedConversations → ApplyResourceACLToAttributeFilter; shared with message RPCs and chart ACL helper |
| Request | QAAttribute.groups, name-only groups | Attribute.groups plus separate Attribute.dynamic_groups |
| Resolution | Expand dynamic memberships from groundTruthUsers before ACL; keep teams on group path | Keep resolved dynamic groups after team ACL intersection, expand through MoveFiltersToUserFilter, intersect with ACL users |
| Access configuration | PI ACL branch checks Advanced Data Access Control | Conversation helper checks Advanced OR Conversation Access Control; new regression covers Conversation on, Advanced off |
| Empty selection | Already protected in PI parser | Adds the protection to conversation helper even for message RPCs passing returnEmptyResponseWhenNoUsersAfterMove=false |
| Root/admin caller | Earlier fix normalizes mixed groups before ACL | New merge of pre-expanded dynamic_groups members also runs for root, preserving their selected filter |
| Mixed selection | Earlier parser already handles dynamic + team/user union | New conversation path retains dynamic group when an inaccessible team is selected alongside it |
| Reuse | Private resolveSelectedGroups | Moves implementation into shared.SplitGroupsByType, called by both paths; minor error wrapping/empty-input handling |

The older fix was not bypassed or reverted. Closed Conversations does not call that parser, so its separate team-only group intersection remained broken. The new PR changes no frontend files and does not change shared/auth/acl_helper.go's team-only group universe.

## Example

Manager can see A and B. Dynamic group G contains A and C. Earlier fix makes the PI audience {A}. Before the new fix, Closed Conversations intersects G with manageable TEAM IDs and returns empty before expanding members. The new path retains G, expands {A,C}, and intersects allowed {A,B}, resulting in {A}. For root with dynamic_groups=G, the new merge preserves {A,C} rather than losing the filter.

## Evidence and limits

Read both PR diffs, current GitHub status, exact new-head implementations and direct/shared callers. New tests cover non-root in/out-of-scope groups, split dynamic_groups for non-root/root, and empty-group Message Stats without ES query. Test additions were inspected; no Go tests or live customer requests were run. PR #34439 is open at the reviewed head. This is a comparison, not a full merge/security approval. ENABLE_DYNAMIC_GROUPS_ACL is still required for pre-expansion of the separate field; the PR's all-release claim was not independently verified. GitHub reads used gh/git with their existing authentication; no AWS/Okta/Azure/SSH credential was explicitly selected or inspected.

## Follow-up

Validate Sunbit-shaped configuration in staging as requested by PR rollout notes; keep future shared-filter migration distinct from this reuse of just the metadata lookup helper.

## Sources

- [Earlier fix: PR #33026](https://github.com/cresta/go-servers/pull/33026)
- [New fix: PR #34439](https://github.com/cresta/go-servers/pull/34439)
- [PI parser normalization](https://github.com/cresta/go-servers/blob/9ed3337b252fe698dcba4d311aaeb6804e3db191/insights-server/internal/analyticsimpl/common_user_filter.go#L230-L247)
- [Closed Conversations caller](https://github.com/cresta/go-servers/blob/9ed3337b252fe698dcba4d311aaeb6804e3db191/insights-server/internal/analyticsimpl/retrieve_closed_conversations.go#L74-L86)
- [Shared helper: type split and ACL](https://github.com/cresta/go-servers/blob/9ed3337b252fe698dcba4d311aaeb6804e3db191/insights-server/internal/shared/common.go#L1220-L1248)
- [Empty audience and root dynamic_groups](https://github.com/cresta/go-servers/blob/9ed3337b252fe698dcba4d311aaeb6804e3db191/insights-server/internal/shared/common.go#L1282-L1315)
