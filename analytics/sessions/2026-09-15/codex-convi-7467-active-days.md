# CONVI-7467 Oportun Active Days investigation

- **Date:** 2026-09-15
- **Source repo:** `/Users/xuanyu.wang/repos/director`
- **Branch/worktree context:** local `main`, read-only investigation; no implementation branch yet
- **Ticket:** [CONVI-7467](https://linear.app/cresta/issue/CONVI-7467/oportun-collections-supervisor-santiago-barreiro-missing-active-days)

## Inputs reviewed

- Linear issue description, relations, customer need, and all three comments.
- `knowledge` Analytics domain, Active Days subdomain, and AI operating model.
- Current indexed Director source plus the local checkout's leaderboard metric visibility and metric-data construction paths.

## Ticket facts

- Affected user: Santiago Barreiro Iniguez (`santiago.barreiro@oportun.com.vcc`).
- Affected use cases: collections voice and collections SPL voice.
- Same-role working peer: Alondra Isamar Puga.
- Support reports Manager, Manager Secondary, and Insights Admin roles, all carrying Leaderboard Manager Access.
- Support reports populated Active Days in the relevant hierarchy, including Santiago = 6 and multiple MX Collections Supervisor agents = 19-20.
- Santiago's directly owned team reportedly contains only manager/supervisor-role users; Alondra's contains front-line agents.

## Code trace

1. `useGetVisibleMetricsForLeaderboard` maps Active Days to `FA.CONVERSATIONS.LIVE.BASIC` and returns it when the authenticated user can access that feature.
2. `useVisibleColumnsForLeaderboards` adds the ordinary `activeDays` column from that visible-metric result when conversation-duration buckets are not selected.
3. The leaderboard-by-metric dropdown is constructed differently. `useFilterSelectableMetricsLogic` intersects visible metrics with `tableDataPerMetricMap.keys()`.
4. `useLeaderboardMetricDataForAgents` builds its canonical user template only from `conversationStats.data.resultGroups`.
5. It iterates Agent Stats groups, discards any Active Days group whose user is absent from that conversation-derived template, and only inserts `METRIC.ACTIVE_DAYS` if at least one matched row remains.

## Live tenant reproduction

- In Oportun Collections SPL, the Role Permissions matrix shows Manager, Manager 2nd, and Insights Admin all have both Leaderboard Manager Access and Conversations Live Basic Access. The ticket's documented permission and Director's current feature gate therefore do not explain the user split.
- With Conversation Duration at `All minutes`, Active Agents and Active Days cards, the ordinary Active Days column, and Active Days in the metric dropdown are present.
- Changing Conversation Duration to `1 - 60+` removes both cards, removes the ordinary Active Days column, removes Active Days from the metric dropdown, and changes the selected metric to Convo volume.
- Returning to `All minutes` restores all of those surfaces.
- The browser was left on the Oportun Agent Leaderboard with `All minutes` selected and the duration popover closed.

This is an exact behavioral match for the ticket's missing dropdown and columns. The ticket screenshots cannot disprove it: all were captured from a reporter/admin session showing `All minutes`, not from Santiago's authenticated browser state.

## Customer data control

- Santiago user ID: `2118b19a05222659`; use case: `voice-collections-spl`.
- Alondra user ID: `509fd61cb96be5a6`; use case: `voice-collections`.
- Santiago has 11 `conversation_source = 0` SPL conversations over 8 distinct days from 2026-07-02 through 2026-08-10.
- Therefore neither absence of activity nor the regular-source exclusion explains why the UI surface is missing.

## Persisted-state and backend explanation

- `useVisibleColumnsForLeaderboards` suppresses Active Days and AHT columns when duration buckets are selected.
- `useFilterSelectableMetricsLogic` suppresses Active Days, Active Agents, and AHT from the metric picker under the same condition.
- `Statistics` hides the Active Days and Active Agents cards under that condition.
- `useLeaderboardsFilters` stores `conversationDurationBuckets` in the `leaderboard-page` local-storage filter state. This permits a same-role, same-tenant peer comparison to diverge by browser/profile state.
- The constraint was introduced intentionally in Director commit `7e13269db10` / PR #5663. The product discussion at `https://crestalabs.slack.com/archives/C05L7FBAGRF/p1729208584854039` records concern that duration filtering could yield fewer or misleading Active Days; no later reversal was found.
- The backend cannot currently promise a correctly duration-filtered Active Days value. `parseClickhouseFilter` applies duration to `conversation_d`, while the relevant `conversation_with_labels_d` path lacks `conversation_duration_secs`; Agent Stats combines populations with a `FULL OUTER JOIN`, so active-assistance rows can survive the filter.

## Conclusion and proof boundary

The leading root cause is Santiago's persisted non-default Conversation Duration filter. It is the only verified condition that explains all reported missing surfaces while permissions and data remain valid. Customer-specific proof still requires Santiago to show the duration label or reset it to `All minutes` and confirm restoration. There is no supported customer-user impersonation mechanism; the employee tenant login helper does not impersonate Santiago.

## Proper fix

1. Immediate remediation: Santiago resets Conversation Duration to `All minutes` and verifies the Active Days card, table column, and selectable metric return.
2. Product UX: make the incompatibility explicit when a duration range is selected. Do not merely unhide Active Days, because its backend population is not consistently duration-filtered.
3. Only after customer confirmation and any agreed UX validation should the troubleshooting skill be created.

## Support confirmation request draft

> Could Santiago open Agent Leaderboard and capture the current Conversation Duration label? If it is anything other than **All minutes**, please change it to **All minutes** and confirm whether Active Days returns in the summary card, table column, and metric dropdown. We reproduced the exact reported symptom with a non-default duration range; this filter is saved per browser/profile, so another user with the same permissions can still see Active Days. A before/after screenshot would close the remaining customer-specific proof gap.

## Skill follow-on

After a validated fix, add an Active Days troubleshooting skill to `/Users/xuanyu.wang/repos/coaching-qm-skills`. It should encode symptom classification (metric absent vs N/A vs zero vs incorrect count), capability checks, request/population checks, source/label/heartbeat checks, and the proven fix/validation sequence from this case.
