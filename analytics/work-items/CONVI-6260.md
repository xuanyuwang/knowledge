# CONVI-6260: Descendant teams missing from Team Leaderboard

**Record state:** Historical fix and review record; current PR/deployment status not rechecked.
**Primary domain:** analytics
**Primary subdomain:** insights-user-filter
**Official ticket:** [CONVI-6260](https://linear.app/cresta/issue/CONVI-6260)
**Last substantive source update:** 2026-02-17
**Consolidated:** 2026-10-25

## Objective and Impact

Restore separate child-team rows when Hilton selects a parent team in Analytics. Role: investigated the group mapping, implemented a fix, and addressed the recorded review findings.

## Source Context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`.
- Historical worktree: `/Users/xuanyu.wang/repos/go-servers-convi-6260`; branch: `convi-6260-hilton-performance-assistance-insights-team-leaderboard-not` (not revalidated).
- [PR #25733](https://github.com/cresta/go-servers/pull/25733).
- [Investigation](../sessions/2026-10-25/convi-6260-legacy-evidence/investigation.md), [review resolutions](../sessions/2026-10-25/convi-6260-legacy-evidence/pr-25733-review.md), [February 13 log](../sessions/2026-10-25/convi-6260-legacy-evidence/log/2026-02-13.md), [February 17 log](../sessions/2026-10-25/convi-6260-legacy-evidence/log/2026-02-17.md).

## Findings and Decisions

`groupsToAggregate` determines the rows shown in a Team Leaderboard. The new parser's `buildUserGroupMappings` fetched explicitly selected groups without expanding their children. The later membership gate rejected descendant teams even if users belonged to them. The recorded fix restored `shared.ListGroups` child-group expansion.

Review found a second requirement: only TEAM names should restrict the team aggregation list. Dynamic groups select a user population but must not become team-row names. The saved review records TEAM filtering in `59999495a5`, plus restoration of the cross-profile customer comment in `1efbf2ee26`. This preserves the distinction between population filtering and team grouping.

The investigation contains contradictory “flag OFF/not active” sentences alongside its enabled-in-production summary. The later README and daily logs identify the new path as enabled outside Schwab at the time. Preserve the original contradictions as historical context; do not infer current flag state. `RetrieveAssistanceStats` used the old path in the saved trace, so that fix alone did not explain every Assistance Insights symptom.

## Validation and Next Actions

- The February 13 log records 32 passing `TestParseUserFilter` tests; February 17 records review fixes. These are historical results, not tests rerun during consolidation.
- No new merge, rollout, or customer confirmation claim is made.
- If reopened, check current group expansion, TEAM/DYNAMIC_GROUP behavior, ACL/profile handling, and each affected API path.
- Durable reference: [Insights User Filter](../subdomains/insights-user-filter/README.md); UI context: [Leaderboard](../subdomains/leaderboard/README.md).

## Timeline

- 2026-02-13 — Root cause and parent-team reproduction recorded; fix published as PR #25733.
- 2026-02-17 — Review fixes and test/mock context documented.
- 2026-07-15 — Legacy README gained domain navigation.
- 2026-10-25 — Evidence consolidated; see [migration session](../sessions/2026-10-25/codex-five-legacy-project-consolidation.md).
