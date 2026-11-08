# CONVI-7852 Chrome reproduction

- Source repo: /Users/xuanyu.wang/repos/director
- Source worktree/branch context: /Users/xuanyu.wang/repos/director-convi-7852, xw/convi-7852-agent-team (no source operations in this session)
- Date: 2026-11-06, America/Toronto
- User requested reproducing the case and setting filters in Chrome from AdventHealth Performance Insights.

Opened the supplied Performance URL, then used the visible Insights → Leaderboard navigation to the Agent tab, where the reported team-label case occurs. Selected user Sarah Young through Target audience → Users search. Current use case: AdventHealth - Voice - Care. Other visible filters: Template All; Oct 6–Nov 6; All minutes; Exclude voicemail Yes; Calculation Scorecard based. No team filter was selected, preserving the ability to observe team label selection.

Live deployed UI reproduces the reported label: Sarah Young shows Christina Gusby in both Agent tables. Main row shows 12.8 conversations/day, 19 active days, 251 scorecards, 83% performance, 4m 8s AHT. Prior investigation's expected direct Agent team is Brittni Howard; current role memberships were not independently re-queried in this browser session. This validates the current deployed symptom, not the draft PR fix or backend deployment.

Page left open in Chrome with filters applied: https://advent-health-us-east-1.cresta.com/director/insights/leaderboard/agents . Screenshot: /Users/xuanyu.wang/.codex/visualizations/2026/11/05/01a1adce-f68c-77c1-b55b-662ef5d627c7/adventhealth-convi-7852-reproduction.jpg . Used the existing authenticated browser session; no credentials read/selected or source/customer data modified. Only page navigation and filter selection.
