# CONVI-7715: Oportun virtual group filter hides Performance Insights for a scoped manager

**Technical status:** Backend fix committed as `899da6f56e` and focused parser suite passed locally. Walter-dev UI validation passed for a non-root team-scoped manager on 2026-10-02; historical Oportun request and build remain uncaptured.  
**Official status:** In Progress (Linear, 2026-10-02).  
**Primary domain:** `analytics`; **subdomain:** `insights-user-filter`.  
**Ticket:** https://linear.app/cresta/issue/CONVI-7715/oportun-performance-insights-group-filter-returns-na-for-team-scoped
**PR:** https://github.com/cresta/go-servers/pull/33026 (opened 2026-09-29)

**Implementation worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7715`, branch `xw/convi-7715-dynamic-group-filter`, refreshed to `origin/main` (`19cd65cbb5`) on 2026-09-29. Fix committed as `899da6f56e`; worktree clean after commit.

## Finding

The selected `Coll test group AB test 1to59` is a dynamic group (`group_type=2`) in profile `us-west-2`, with 53 active direct members on 2026-09-24. All 53 are also members of at least one team on which Liliana has a direct `custom_manager` or `custom_manager_2nd` role. Liliana is not a member of the dynamic group. Oportun production config enables advanced data access control.

Director sends the selected group as `QAAttribute.groups` for Performance Insights. `RetrieveQAScoreStats` passes it to `ParseUserFilterForAnalytics`. For a group-only selection, `BuildResourceACLOptions` requests **only** managed groups filtered to the selected group ID, omitting managed users. `ResourceACL` derives manageable groups from TEAM hierarchy only, so the selected dynamic group cannot match. The ACL returns no group and only the caller user. The parser has already narrowed ground-truth users to the dynamic group's members, which excludes Liliana; intersection therefore becomes empty and `RetrieveQAScoreStats` returns an empty result. Root/broad access bypasses this narrowing; a team filter can match a managed TEAM group and expand its users. This explains the reported split without implying missing scorecard data.

The defect is at the authorization/filter intersection, not a lack of Liliana's team management over these 53 current members. A safe fix should resolve virtual-group members as a selected audience, then intersect those users with the caller's independently computed managed-user ACL. It must not grant access to all virtual-group members merely because the group is selectable. Preserve explicit-user plus group union semantics and team-filter behavior.

## Scope and limitations

- Read-only auth DB checks establish current group type, roster, manager-team overlap and active profile status. They do not reproduce Liliana's exact September 11 request, scorecard time window, or deployed image.
- The ticket reports Sergio Ceballos Melo as a second affected user. His direct manager team has **zero** current overlap with this virtual group's 53 members, so the same authorization expectation is not established for him by these data.
- The related CONVI-7347 concerned Ryan Helwig's empty Collections view. Its ticket has no causal fix details or matching group-filter reproduction; a shared cause is unproven.
- No product code, Linear issue, or production data was changed.

## Next actions

1. Review PR #33026 and run any broader verification required for rollout. The focused `common_user_filter_test` Bazel target passed locally and the Walter-dev scoped-manager UI scenario passed; deployed image identity has not been established.
2. Capture one failing historical Oportun request/build or validate the fix in a scoped Oportun session; compare the exact selected group ID, resolved member IDs, ACL user IDs and response.
3. Investigate Sergio separately against the intended team/virtual-group audience.

## Implementation and validation — 2026-09-29

At the end of Step 0, the parser resolves name-only selected groups through profile-scoped UserService metadata. It retains TEAM groups for the existing group/ACL path and converts DYNAMIC_GROUP memberships from `groundTruthUsers` into requested users before the ACL call. Dynamic members therefore enter the existing managed-user ACL intersection. A selected group that resolves to no eligible audience returns empty before Step 2; the no-filter `shouldUseAllAgents` expression is unchanged. Missing/stale groups also resolve to an empty selected audience. The new metadata lookup adds one `ListGroups` call when groups are selected.

The focused suite covers limited access, misleading request type, selected empty and stale groups, dynamic plus TEAM and explicit-user union, and direct membership. `bazel test //insights-server/internal/analyticsimpl:common_user_filter_test --test_output=errors` passed locally. `gofmt` and `git diff --check` passed. Commit: `899da6f56e`. This is local source/test evidence, not a deployed fix or a reproduction of the historical Oportun request.

## Walter-dev staging fixture — 2026-10-02

An existing `cresta/walter-dev` dynamic group `00` contains an active agent on `Advanced Access Control team` with two scored September 23 conversations under active template `xuanyu na score`. The team has an active `custom_manager_2nd` user outside the dynamic group (`test-manager@cresta.test`) whose global custom role list has no super-admin role. In the existing broad-access Chrome session, selecting the template, Last 30 days, and group `00` produced 100% and two filtered conversations; DevTools captured the name-only group filter and a response for the scored agent. This validates the fixture and broad-access path. The scoped-manager request and deployed fix are still unverified. See [session](../sessions/2026-10-02/codex-convi-7715-walter-dev-fixture.md).

After the Chrome account's roles were changed to manager and user admin, the read-only auth DB showed both assignments on the root TEAM. Chrome still sent older broad authorization claims. On the same template and date range, both the fixture TEAM and dynamic group `00` returned 100% and two scored conversations. This comparison confirms the fixture but still does not exercise the limited team-scoped manager path.

After removal from the root TEAM, the Chrome user's auth DB aggregate is manager only and its sole TEAM assignment is `xuanyu only`. That TEAM has no agent members or child groups, so it does not contain the scored agent in dynamic group `00`. The known positive fixture remains `Advanced Access Control team` plus group `00`; a non-root manager session scoped to that TEAM is still needed for the decisive comparison. See the [Walter-dev session](../sessions/2026-10-02/codex-convi-7715-walter-dev-fixture.md).

The user then assigned the Chrome account `MANAGER_2ND` on `Advanced Access Control team` and signed in again. A read-only auth DB check found that role and TEAM assignment without root TEAM membership. On the same scored template and Last 30 days / Weekly range, both the TEAM filter and dynamic group `00` returned 100% and two conversations; Sergei's leaderboard row showed two 100% scorecards. This validates the positive scoped-manager UI scenario on Walter-dev. It does not reconstruct the historical Oportun request or prove the deployed image identity. See the [Walter-dev session](../sessions/2026-10-02/codex-convi-7715-walter-dev-fixture.md).

## Evidence

- [Session investigation](../sessions/2026-09-24/codex-convi-7715-oportun-group-filter.md)
- [Scaffold review](../sessions/2026-09-24/codex-convi-7715-filter-scaffold-review.md)
- [Worktree and filter-semantics follow-up](../sessions/2026-09-24/codex-convi-7715-worktree-and-filter-semantics.md)
- [Implementation session](../sessions/2026-09-29/codex-convi-7715-implementation.md)
- [Parser refactor analysis](../sessions/2026-09-29/codex-parse-user-filter-refactor-analysis.md)
- [Walter-dev staging fixture](../sessions/2026-10-02/codex-convi-7715-walter-dev-fixture.md)
- Latest config `origin/master:configv3/prod/oportun/config.yaml`: `authConfig.enableAdvancedDataAccessControl: true`, `authDbConnArn` resolves `chat-prod/auth-prod/oportun`.
- Source: Director `getQAFilterByAttribute.ts`; go-servers `retrieve_qa_score_stats.go`, `common_user_filter.go`, `insights-server/internal/shared/common.go`, `shared/auth/acl_helper.go`.
