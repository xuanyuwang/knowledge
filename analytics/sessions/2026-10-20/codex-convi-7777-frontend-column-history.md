# CONVI-7777 frontend column history

- Date: 2026-10-20 (America/Toronto).
- Primary domain/subdomain: analytics / assistance-insights.
- Source repo: `/Users/xuanyu.wang/repos/director`, main checkout, branch `main`.
- Request: locate the prior column-addition commits before parallel frontend work on CONVI-7777.

## Verified findings

- Commit `1c5299b2810625de1dc00e2ca9002e4e11e9ecd8`, `[CONVI-3323] Add new columns to Assistance Used leaderboards`, merged in [Director #8535](https://github.com/cresta/director/pull/8535) on 2024-08-05, was authored by Kurt Choi. It introduced both `# of convos (per day)` and `Convos powered by AA %`, plus Active Days, across agent/team Assistance Used leaderboards.
- The six-file change covers page-level conversation/agent stats loading, leaderboard props, both agent/team data and column hooks, visible-column defaults, and row types. This is the directly relevant implementation precedent for CONVI-7777.
- Current agent definitions are in `packages/director-app/src/components/insights/assistance/assistance-insights-statistics/assistance-used-page/assistance-used-leaderboard/hooks/useAssistanceUsedByAgentLeaderboard.tsx`, lines 415-454. Corresponding team definitions are in `useAssistanceUsedByTeamLeaderboard.tsx` around lines 366-412.
- Follow-up commit `fb335d6b30`, CONVI-3358 / [#8576](https://github.com/cresta/director/pull/8576), added the CSV column support on 2024-08-07, also authored by Kurt Choi.
- Xuanyu's closest matching column-addition work is `da2050af5d` (and parallel-history equivalent `8b1b8cc769`), `feat(leaderboard): add agent submitted scorecard count column`, 2026-06-02; the merged change is `6ec3409229`, CONVI-6968 / [#19346](https://github.com/cresta/director/pull/19346), merged 2026-06-09.
- Evidence: local `git log --all` author/path/pickaxe searches, `git blame`, commit diffs, and live `gh pr view` metadata for #8535 and #19346. No explicitly selected credential/profile was used; gh authenticated itself.

## State and follow-up

This turn located implementation precedents. No frontend product code was changed. Next frontend work can trace the dedicated Assistance Usage API through the same agent/team table integration points; API readiness and final frontend scope need current verification before implementation.
