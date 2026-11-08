# CONVI-7852 local Chrome verification

- Source repo: /Users/xuanyu.wang/repos/director
- Worktree: /Users/xuanyu.wang/repos/director-convi-7852
- Branch: xw/convi-7852-agent-team
- HEAD: 0ec809a7ae; worktree clean, fix wiring and selector confirmed in source
- Local URL: http://localhost:3100/director/insights/leaderboard/agents
- Date: 2026-11-06, America/Toronto

User requested live verification of the fix on localhost in Chrome. UI identifies branch xw/convi-7852-agent-team and connection to advent-health-us-east-1.cresta.com. Applied the same Sarah Young filter as production with Voice Care, Template All, Oct 6–Nov 6, All minutes, Exclude voicemail Yes, Scorecard based. Both local Agent tables still show Christina Gusby. Metrics match the prior production observation: 12.8 conversations/day, 19 active days, 251 scorecards, 83%, 4m 8s AHT. Expected direct Agent team from prior membership investigation is Brittni Howard.

Inspected the actual non-AgentAssist useRetrieveConversationStats query through the visible Tanstack query devtools Data Explorer. Request is Sarah's user resource 2f8a738e43909145, no group filter, Agent+Group grouping, daily, same Voice Care usecase. Cached data contains one result row, groupedByAttributes.groups has two entries, and groupedByAttributes.user.groupMemberships is []. Total conversation count is 244, average 12.8421049118042, AHT 248 seconds. No query-cache data edited or simulated. The UI clipboard copy yielded no text, so data was inspected by expanding the explorer instead.

Conclusion: live corrected-label verification is blocked by empty membership metadata in the loaded data. The selector has no eligible candidate and follows the approved first-team fallback. This does not establish whether backend rollout, normal response caching, or another upstream path caused the empty list; raw response/deployment was not independently inspected. No replacement user fetch, cache override, mocking, or source fix performed.

Closed query devtools and left the filtered local tab open. Screenshot: /Users/xuanyu.wang/.codex/visualizations/2026/11/05/01a1adce-f68c-77c1-b55b-662ef5d627c7/adventhealth-convi-7852-local-verification.jpg . Only navigation, filters and read-only query inspection. Existing browser authentication; no AWS/Okta/Azure/SSH credential or profile selected/read. Next: establish that the analytics endpoint is returning populated memberships, then repeat the same local-browser check.
